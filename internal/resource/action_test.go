package resource

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"res-downloader/internal/logging"
	shared "res-downloader/internal/model"
	"res-downloader/internal/plugin"
	"testing"
)

func TestPluginFileActionProcessesWithoutReplacingSource(t *testing.T) {
	resources := &Resource{resType: map[string]bool{"all": true}}
	userDirectory := t.TempDir()
	installFileActionTestPlugin(t, userDirectory)
	manager := plugin.NewManager(
		userDirectory,
		func() plugin.NetworkSettings { return plugin.NetworkSettings{} },
		nil,
		resources,
		logging.New(false, ""),
	)
	candidate := shared.ResourceCandidate{
		ID: "test-resource", Source: shared.ResourceSource{PluginID: "test.file-action"},
		Actions: []shared.ResourceAction{{
			ID:   "decrypt-local-file",
			Data: map[string]interface{}{"options": map[string]interface{}{"key": float64(90)}},
		}},
	}
	definition, processor, err := manager.ResolveFileAction(candidate, "decrypt-local-file")
	if err != nil {
		t.Fatal(err)
	}

	plaintext := make([]byte, 320*1024)
	for index := range plaintext {
		plaintext[index] = byte(index % 251)
	}
	plainPath := filepath.Join(t.TempDir(), "plain.mp4")
	if err := os.WriteFile(plainPath, plaintext, 0600); err != nil {
		t.Fatal(err)
	}
	encryptedPath, err := manager.ProcessWASM(context.Background(), processor, plainPath, 0)
	if err != nil {
		t.Fatal(err)
	}
	sourcePath := filepath.Join(t.TempDir(), "downloaded.mp4")
	if err := os.Rename(encryptedPath, sourcePath); err != nil {
		t.Fatal(err)
	}
	originalEncrypted, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}

	outputPath, err := (&Resource{plugins: manager}).runFileAction(definition, processor, sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	decrypted, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Fatal("file action output does not match plaintext")
	}
	stillEncrypted, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stillEncrypted, originalEncrypted) {
		t.Fatal("file action modified the selected source file")
	}
}

func installFileActionTestPlugin(t *testing.T, userDirectory string) {
	t.Helper()
	directory := filepath.Join(userDirectory, "plugins", "test.file-action")
	if err := os.MkdirAll(directory, 0750); err != nil {
		t.Fatal(err)
	}
	manifest := `{
  "id":"test.file-action","name":"File action test","version":"1.0.0","apiVersion":1,
  "runtime":"javascript","entry":"main.js",
  "permissions":{"domains":["example.com"],"capabilities":["process-download"]},
  "processors":{"xor":{"runtime":"wasm","entry":"decrypt.wasm","apiVersion":1}},
  "actions":{"decrypt-local-file":{"kind":"process-file","processor":"xor","inputExtensions":[".mp4"],"outputExtension":".mp4"}}
}`
	if err := os.WriteFile(filepath.Join(directory, "plugin.json"), []byte(manifest), 0640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "main.js"), []byte(`function onObservation() { return {decision: "continue"}; }`), 0640); err != nil {
		t.Fatal(err)
	}
	wasm, err := os.ReadFile(filepath.Join("..", "..", "examples", "plugins", "wasm-xor", "decrypt.wasm"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "decrypt.wasm"), wasm, 0640); err != nil {
		t.Fatal(err)
	}
}
