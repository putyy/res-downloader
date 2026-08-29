package plugin

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	shared "res-downloader/internal/model"
	"strings"
)

func syncBundledPlugin(sourceDirectory string) (shared.PluginManifest, string, error) {
	bundledRoot, err := findProjectBundledRoot()
	if err != nil {
		return shared.PluginManifest{}, "", err
	}
	return syncBundledPluginToRoot(sourceDirectory, bundledRoot)
}

func syncBundledPluginToRoot(sourceDirectory, bundledRoot string) (shared.PluginManifest, string, error) {
	runtimePlugin, _, err := LoadOfficialPlugin(sourceDirectory)
	if err != nil {
		return shared.PluginManifest{}, "", err
	}
	manifest := runtimePlugin.Manifest()
	if reservedPluginIDPrefix(manifest.ID) != "official." {
		return shared.PluginManifest{}, "", errors.New("only official.* plugins can be bundled")
	}

	sourceAbsolute, err := filepath.Abs(sourceDirectory)
	if err != nil {
		return shared.PluginManifest{}, "", err
	}
	info, err := os.Stat(sourceAbsolute)
	if err != nil {
		return shared.PluginManifest{}, "", err
	}
	if !info.IsDir() {
		return shared.PluginManifest{}, "", errors.New("plugin source must be a directory")
	}
	sourceName := filepath.Base(sourceAbsolute)
	if sourceName == "." || sourceName == string(filepath.Separator) || strings.HasPrefix(sourceName, ".") {
		return shared.PluginManifest{}, "", errors.New("plugin source directory name is invalid")
	}

	bundledRootAbsolute, err := filepath.Abs(bundledRoot)
	if err != nil {
		return shared.PluginManifest{}, "", err
	}
	if err := os.MkdirAll(bundledRootAbsolute, 0750); err != nil {
		return shared.PluginManifest{}, "", err
	}
	target := filepath.Join(bundledRootAbsolute, sourceName)
	if sourceAbsolute == target || strings.HasPrefix(target+string(filepath.Separator), sourceAbsolute+string(filepath.Separator)) {
		return shared.PluginManifest{}, "", errors.New("bundled target must be outside the plugin source directory")
	}

	stage, err := os.MkdirTemp(bundledRootAbsolute, ".sync-"+sourceName+"-")
	if err != nil {
		return shared.PluginManifest{}, "", err
	}
	defer os.RemoveAll(stage)
	if err := copyBundledPluginSource(sourceAbsolute, stage); err != nil {
		return shared.PluginManifest{}, "", err
	}
	stagedRuntime, _, err := LoadOfficialPlugin(stage)
	if err != nil {
		return shared.PluginManifest{}, "", fmt.Errorf("validate synchronized plugin: %w", err)
	}
	if stagedRuntime.Manifest().ID != manifest.ID || stagedRuntime.Manifest().Version != manifest.Version {
		return shared.PluginManifest{}, "", errors.New("synchronized plugin manifest changed unexpectedly")
	}

	replaced, err := bundledDirectoriesForReplacement(bundledRootAbsolute, filepath.Base(stage), sourceName, manifest.ID)
	if err != nil {
		return shared.PluginManifest{}, "", err
	}
	backupRoot, err := os.MkdirTemp(bundledRootAbsolute, ".sync-backup-"+sourceName+"-")
	if err != nil {
		return shared.PluginManifest{}, "", err
	}
	defer os.RemoveAll(backupRoot)
	moved := make([]string, 0, len(replaced))
	for _, name := range replaced {
		if err := os.Rename(filepath.Join(bundledRootAbsolute, name), filepath.Join(backupRoot, name)); err != nil {
			restoreBundledDirectories(bundledRootAbsolute, backupRoot, moved)
			return shared.PluginManifest{}, "", fmt.Errorf("replace bundled plugin directory %s: %w", name, err)
		}
		moved = append(moved, name)
	}
	if err := os.Rename(stage, target); err != nil {
		restoreBundledDirectories(bundledRootAbsolute, backupRoot, moved)
		return shared.PluginManifest{}, "", fmt.Errorf("activate synchronized bundled plugin: %w", err)
	}
	return manifest, target, nil
}

func copyBundledPluginSource(sourceRoot, targetRoot string) error {
	return filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		first := strings.SplitN(filepath.ToSlash(relative), "/", 2)[0]
		if strings.HasPrefix(first, ".") || strings.HasPrefix(first, "_") {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() && excludedPluginDevelopmentDirectory(first) {
			return filepath.SkipDir
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("plugin source contains unsupported symbolic link %q", relative)
		}
		target := filepath.Join(targetRoot, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0750)
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("plugin source contains unsupported file %q", relative)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0750); err != nil {
			return err
		}
		return os.WriteFile(target, content, 0644)
	})
}

func bundledDirectoriesForReplacement(root, stageName, targetName, pluginID string) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	replaced := make([]string, 0, 2)
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == stageName || strings.HasPrefix(entry.Name(), ".sync-backup-") {
			continue
		}
		if entry.Name() == targetName {
			replaced = append(replaced, entry.Name())
			continue
		}
		raw, readErr := os.ReadFile(filepath.Join(root, entry.Name(), "plugin.json"))
		if readErr != nil {
			continue
		}
		manifest, parseErr := ParseStoreManifestJSON(raw, true)
		if parseErr == nil && manifest.ID == pluginID {
			replaced = append(replaced, entry.Name())
		}
	}
	return replaced, nil
}

func restoreBundledDirectories(root, backupRoot string, names []string) {
	for index := len(names) - 1; index >= 0; index-- {
		_ = os.Rename(filepath.Join(backupRoot, names[index]), filepath.Join(root, names[index]))
	}
}

func findProjectBundledRoot() (string, error) {
	directory, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		moduleFile := filepath.Join(directory, "go.mod")
		bundledRoot := filepath.Join(directory, "internal", "plugin", "bundled")
		if moduleInfo, moduleErr := os.Stat(moduleFile); moduleErr == nil && moduleInfo.Mode().IsRegular() {
			if bundledInfo, bundledErr := os.Stat(bundledRoot); bundledErr == nil && bundledInfo.IsDir() {
				return bundledRoot, nil
			}
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", errors.New("run sync-bundled inside the res-downloader source tree")
		}
		directory = parent
	}
}
