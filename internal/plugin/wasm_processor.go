package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	shared "res-downloader/internal/model"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

const (
	maxPluginWASMSize       = 8 * 1024 * 1024
	maxPluginWASMOptions    = 64 * 1024
	pluginWASMMemoryPages   = 1024 // 64 MiB
	pluginWASMChunkSize     = 256 * 1024
	pluginWASMOutputSlack   = 64 * 1024
	pluginWASMCallTimeout   = 2 * time.Second
	pluginWASMTotalTimeout  = 10 * time.Minute
	wasmProcessorType       = "plugin-wasm"
	wasmProcessorIDOption   = "processor"
	wasmProcessorOwnerKey   = "_resDownloaderPlugin"
	wasmProcessorDigestKey  = "_resDownloaderPluginDigest"
	wasmProcessorAPIVersion = shared.PluginProcessorAPIVersion
)

type wasmProcessorSpec struct {
	pluginID    string
	processorID string
	path        string
	options     map[string]interface{}
}

func (m *PluginManager) bindDownloadProcessors(pluginID string, processors []shared.DownloadStep) error {
	for index := range processors {
		if processors[index].Type != wasmProcessorType {
			continue
		}
		if pluginID == "" {
			return errors.New("plugin-wasm processor requires a resource plugin id")
		}
		processorID, _ := processors[index].Options[wasmProcessorIDOption].(string)
		if processorID == "" {
			return errors.New("plugin-wasm processor requires options.processor")
		}

		m.mu.RLock()
		status, exists := m.statuses[pluginID]
		m.mu.RUnlock()
		if !exists || status.Builtin {
			return fmt.Errorf("plugin-wasm owner %q is not an external plugin", pluginID)
		}
		if !status.Manifest.Permissions.Has("process-download") {
			return fmt.Errorf("plugin %q does not have process-download permission", pluginID)
		}
		if _, exists := status.Manifest.Processors[processorID]; !exists {
			return fmt.Errorf("plugin %q does not declare processor %q", pluginID, processorID)
		}

		options := make(map[string]interface{}, len(processors[index].Options)+1)
		for key, value := range processors[index].Options {
			options[key] = value
		}
		options[wasmProcessorOwnerKey] = pluginID
		m.mu.RLock()
		options[wasmProcessorDigestKey] = m.statuses[pluginID].Digest
		m.mu.RUnlock()
		processors[index].Options = options
	}
	return nil
}

func (m *PluginManager) resolveWASMProcessor(step shared.DownloadStep) (wasmProcessorSpec, error) {
	if step.Type != wasmProcessorType {
		return wasmProcessorSpec{}, fmt.Errorf("processor type %q is not WebAssembly", step.Type)
	}
	pluginID, _ := step.Options[wasmProcessorOwnerKey].(string)
	processorID, _ := step.Options[wasmProcessorIDOption].(string)
	expectedDigest, _ := step.Options[wasmProcessorDigestKey].(string)
	if pluginID == "" || processorID == "" {
		return wasmProcessorSpec{}, errors.New("plugin-wasm processor is missing its bound owner or processor id")
	}

	m.mu.RLock()
	status, exists := m.statuses[pluginID]
	m.mu.RUnlock()
	if exists && expectedDigest != "" && status.Digest != expectedDigest {
		backupDirectory := filepath.Join(m.pluginBackupRoot(), pluginID)
		if digest, err := hashPluginDirectory(backupDirectory); err == nil && digest == expectedDigest {
			runtimePlugin, manifestPath, loadErr := m.loadExternalPlugin(backupDirectory)
			if loadErr != nil && isBundledPluginID(pluginID) {
				runtimePlugin, manifestPath, loadErr = LoadBundledPlugin(backupDirectory)
				if loadErr != nil {
					runtimePlugin, manifestPath, loadErr = m.loadOfficialPlugin(backupDirectory)
				}
			}
			if loadErr == nil {
				status = shared.PluginStatus{Manifest: runtimePlugin.Manifest(), Path: manifestPath, Source: status.Source, Loaded: true, Digest: digest}
				exists = true
			}
		}
	}
	if !exists || status.Builtin || !status.Manifest.Permissions.Has("process-download") {
		return wasmProcessorSpec{}, fmt.Errorf("plugin-wasm owner %q is unavailable", pluginID)
	}
	definition, exists := status.Manifest.Processors[processorID]
	if !exists || definition.Runtime != "wasm" || definition.APIVersion != wasmProcessorAPIVersion {
		return wasmProcessorSpec{}, fmt.Errorf("plugin-wasm processor %q is unavailable", processorID)
	}
	directory := filepath.Dir(status.Path)
	path, err := securePluginFilePath(directory, definition.Entry)
	if err != nil {
		return wasmProcessorSpec{}, err
	}

	options := make(map[string]interface{}, len(step.Options))
	for key, value := range step.Options {
		if key != wasmProcessorOwnerKey && key != wasmProcessorIDOption && key != wasmProcessorDigestKey {
			options[key] = value
		}
	}
	return wasmProcessorSpec{pluginID: pluginID, processorID: processorID, path: path, options: options}, nil
}

// ProcessWASM resolves a processor against the owning plugin version and
// executes it without exposing plugin paths or WASM implementation details to
// the resource/download packages.
func (m *PluginManager) ProcessWASM(ctx context.Context, step shared.DownloadStep, sourcePath string, initialOffset uint64) (string, error) {
	spec, err := m.resolveWASMProcessor(step)
	if err != nil {
		return "", err
	}
	return runWASMProcessorAtOffset(ctx, spec, sourcePath, initialOffset)
}

func runWASMProcessor(ctx context.Context, spec wasmProcessorSpec, sourcePath string) (string, error) {
	return runWASMProcessorAtOffset(ctx, spec, sourcePath, 0)
}

func runWASMProcessorAtOffset(ctx context.Context, spec wasmProcessorSpec, sourcePath string, initialOffset uint64) (string, error) {
	wasmBytes, err := os.ReadFile(spec.path)
	if err != nil {
		return "", err
	}
	if len(wasmBytes) <= 8 || len(wasmBytes) > maxPluginWASMSize || string(wasmBytes[:4]) != "\x00asm" {
		return "", errors.New("invalid plugin WebAssembly binary")
	}
	optionBytes, err := json.Marshal(spec.options)
	if err != nil {
		return "", fmt.Errorf("marshal WASM processor options: %w", err)
	}
	if len(optionBytes) > maxPluginWASMOptions {
		return "", fmt.Errorf("WASM processor options exceed %d bytes", maxPluginWASMOptions)
	}

	ctx, cancel := context.WithTimeout(ctx, pluginWASMTotalTimeout)
	defer cancel()
	runtimeConfig := wazero.NewRuntimeConfigInterpreter().
		WithMemoryLimitPages(pluginWASMMemoryPages).
		WithCloseOnContextDone(true)
	runtime := wazero.NewRuntimeWithConfig(ctx, runtimeConfig)
	defer runtime.Close(ctx)
	compileCtx, cancelCompile := context.WithTimeout(ctx, pluginWASMCallTimeout)
	compiled, err := runtime.CompileModule(compileCtx, wasmBytes)
	cancelCompile()
	if err != nil {
		return "", fmt.Errorf("compile plugin WASM: %w", err)
	}
	defer compiled.Close(ctx)
	instantiateCtx, cancelInstantiate := context.WithTimeout(ctx, pluginWASMCallTimeout)
	module, err := runtime.InstantiateModule(instantiateCtx, compiled, wazero.NewModuleConfig().
		WithName(spec.pluginID+"."+spec.processorID).
		WithStartFunctions())
	cancelInstantiate()
	if err != nil {
		return "", fmt.Errorf("instantiate plugin WASM: %w", err)
	}
	defer module.Close(ctx)

	functions, err := loadWASMProcessorFunctions(ctx, module)
	if err != nil {
		return "", err
	}
	if err := initialiseWASMProcessor(ctx, module, functions, optionBytes); err != nil {
		return "", err
	}
	return transformFileWithWASM(ctx, module, functions, sourcePath, initialOffset)
}

type wasmProcessorFunctions struct {
	alloc     api.Function
	free      api.Function
	transform api.Function
	init      api.Function
}

func loadWASMProcessorFunctions(ctx context.Context, module api.Module) (wasmProcessorFunctions, error) {
	version := module.ExportedFunction("rd_abi_version")
	functions := wasmProcessorFunctions{
		alloc: module.ExportedFunction("rd_alloc"), free: module.ExportedFunction("rd_free"),
		init: module.ExportedFunction("rd_init"), transform: module.ExportedFunction("rd_transform"),
	}
	if version == nil || functions.alloc == nil || functions.init == nil || functions.transform == nil || module.Memory() == nil {
		return functions, errors.New("WASM processor must export memory, rd_abi_version, rd_alloc, rd_init and rd_transform")
	}
	results, err := callWASMFunction(ctx, version)
	if err != nil {
		return functions, fmt.Errorf("call rd_abi_version: %w", err)
	}
	if len(results) != 1 || uint32(results[0]) != wasmProcessorAPIVersion {
		return functions, fmt.Errorf("unsupported WASM processor ABI version %d", firstWASMResult(results))
	}
	return functions, nil
}

func initialiseWASMProcessor(ctx context.Context, module api.Module, functions wasmProcessorFunctions, options []byte) error {
	ptr, err := allocateWASMMemory(ctx, module, functions.alloc, uint32(len(options)))
	if err != nil {
		return err
	}
	if functions.free != nil {
		defer func() { _, _ = callWASMFunction(ctx, functions.free, uint64(ptr), uint64(len(options))) }()
	}
	if len(options) > 0 && !module.Memory().Write(ptr, options) {
		return errors.New("write WASM processor options outside guest memory")
	}
	results, err := callWASMFunction(ctx, functions.init, uint64(ptr), uint64(len(options)))
	if err != nil {
		return fmt.Errorf("call rd_init: %w", err)
	}
	if status, err := wasmStatus(results); err != nil || status != 0 {
		if err != nil {
			return fmt.Errorf("rd_init: %w", err)
		}
		return fmt.Errorf("rd_init returned status %d", status)
	}
	return nil
}

func transformFileWithWASM(ctx context.Context, module api.Module, functions wasmProcessorFunctions, sourcePath string, initialOffset uint64) (string, error) {
	source, err := os.Open(sourcePath)
	if err != nil {
		return "", err
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil {
		return "", err
	}
	target, err := os.CreateTemp(filepath.Dir(sourcePath), ".res-downloader-wasm-*")
	if err != nil {
		return "", err
	}
	targetPath := target.Name()
	succeeded := false
	defer func() {
		_ = target.Close()
		if !succeeded {
			_ = os.Remove(targetPath)
		}
	}()
	if err := target.Chmod(info.Mode().Perm()); err != nil {
		return "", err
	}

	capacity := uint32(pluginWASMChunkSize + pluginWASMOutputSlack)
	ptr, err := allocateWASMMemory(ctx, module, functions.alloc, capacity)
	if err != nil {
		return "", err
	}
	if functions.free != nil {
		defer func() { _, _ = callWASMFunction(ctx, functions.free, uint64(ptr), uint64(capacity)) }()
	}

	buffer := make([]byte, pluginWASMChunkSize)
	offset := initialOffset
	for {
		count, readErr := io.ReadFull(source, buffer)
		final := readErr == io.EOF || readErr == io.ErrUnexpectedEOF
		if readErr != nil && !final {
			return "", readErr
		}
		if count > 0 && !module.Memory().Write(ptr, buffer[:count]) {
			return "", errors.New("write input outside WASM guest memory")
		}
		finalFlag := uint64(0)
		if final {
			finalFlag = 1
		}
		results, err := callWASMFunction(ctx, functions.transform,
			uint64(ptr), uint64(count), uint64(capacity),
			uint64(uint32(offset)), uint64(uint32(offset>>32)), finalFlag)
		if err != nil {
			return "", fmt.Errorf("call rd_transform at offset %d: %w", offset, err)
		}
		outputLength, err := wasmStatus(results)
		if err != nil {
			return "", fmt.Errorf("rd_transform at offset %d: %w", offset, err)
		}
		if outputLength < 0 {
			return "", fmt.Errorf("rd_transform returned status %d at offset %d", outputLength, offset)
		}
		if uint64(outputLength) > uint64(capacity) {
			return "", fmt.Errorf("rd_transform output %d exceeds capacity %d", outputLength, capacity)
		}
		if outputLength > 0 {
			output, ok := module.Memory().Read(ptr, uint32(outputLength))
			if !ok {
				return "", errors.New("read output outside WASM guest memory")
			}
			if _, err := target.Write(output); err != nil {
				return "", err
			}
		}
		offset += uint64(count)
		if final {
			break
		}
	}
	if err := target.Sync(); err != nil {
		return "", err
	}
	if err := target.Close(); err != nil {
		return "", err
	}
	succeeded = true
	return targetPath, nil
}

func allocateWASMMemory(ctx context.Context, module api.Module, alloc api.Function, size uint32) (uint32, error) {
	results, err := callWASMFunction(ctx, alloc, uint64(size))
	if err != nil {
		return 0, fmt.Errorf("call rd_alloc: %w", err)
	}
	if len(results) != 1 || results[0] > math.MaxUint32 {
		return 0, errors.New("rd_alloc returned an invalid pointer")
	}
	ptr := uint32(results[0])
	if _, ok := module.Memory().Read(ptr, size); !ok {
		return 0, errors.New("rd_alloc returned memory outside the guest range")
	}
	return ptr, nil
}

func callWASMFunction(ctx context.Context, function api.Function, params ...uint64) ([]uint64, error) {
	callCtx, cancel := context.WithTimeout(ctx, pluginWASMCallTimeout)
	defer cancel()
	return function.Call(callCtx, params...)
}

func wasmStatus(results []uint64) (int32, error) {
	if len(results) != 1 {
		return 0, fmt.Errorf("expected one i32 result, got %d", len(results))
	}
	return int32(uint32(results[0])), nil
}

func firstWASMResult(results []uint64) uint64 {
	if len(results) == 0 {
		return 0
	}
	return results[0]
}
