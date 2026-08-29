package plugin

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	shared "res-downloader/internal/model"
	"sort"
	"strings"
	"sync"
)

// Bundled plugins are regular external plugins materialised into the user
// plugin directory at startup. They use no private Go API and can be disabled,
// inspected and exercised with the same tooling as community plugins.
//
//go:embed bundled/*
var bundledPlugins embed.FS

func installBundledPlugins(pluginDirectory string) error {
	return installBundledPluginsExcept(pluginDirectory, nil)
}

func installBundledPluginsExcept(pluginDirectory string, removed map[string]bool) error {
	_, err := installBundledPluginsForManager(pluginDirectory, removed, nil)
	return err
}

// installBundledPluginsForManager materialises bundled plugins unless the user
// removed the logical plugin or installed a newer official store version.
// Equal versions prefer the application-bundled copy.
func installBundledPluginsForManager(pluginDirectory string, removed map[string]bool, sources map[string]string) ([]string, error) {
	replacedSources := make([]string, 0)
	descriptors, err := bundledPluginDescriptors()
	if err != nil {
		return nil, err
	}
	for _, descriptor := range descriptors {
		pluginID := descriptor.ID
		if removed[pluginID] {
			continue
		}
		if sources[pluginID] == shared.PluginSourceOfficial {
			newer, compareErr := installedOfficialIsNewer(pluginDirectory, pluginID)
			if compareErr == nil && newer {
				continue
			}
			replacedSources = append(replacedSources, pluginID)
		}
		targetRoot := filepath.Join(pluginDirectory, pluginID)
		if err := os.RemoveAll(targetRoot); err != nil {
			return nil, fmt.Errorf("replace bundled plugin %s: %w", pluginID, err)
		}
		sourceRoot := filepath.ToSlash(filepath.Join("bundled", descriptor.Directory))
		if err := fs.WalkDir(bundledPlugins, sourceRoot, func(sourcePath string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			relative := strings.TrimPrefix(sourcePath, sourceRoot)
			relative = strings.TrimPrefix(relative, "/")
			target := filepath.Join(targetRoot, filepath.FromSlash(relative))
			if entry.IsDir() {
				return os.MkdirAll(target, 0750)
			}
			if !entry.Type().IsRegular() {
				return fmt.Errorf("bundled plugin %s contains unsupported file %s", pluginID, relative)
			}
			content, readErr := bundledPlugins.ReadFile(sourcePath)
			if readErr != nil {
				return readErr
			}
			if writeErr := os.WriteFile(target, content, 0644); writeErr != nil {
				return fmt.Errorf("write bundled plugin %s: %w", relative, writeErr)
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}
	return replacedSources, nil
}

func installedOfficialIsNewer(pluginDirectory, id string) (bool, error) {
	installedRaw, err := os.ReadFile(filepath.Join(pluginDirectory, id, "plugin.json"))
	if err != nil {
		return false, err
	}
	descriptor, exists := bundledPluginDescriptorForID(id)
	if !exists {
		return false, errors.New("bundled plugin not found")
	}
	bundledRaw, err := bundledPlugins.ReadFile(filepath.ToSlash(filepath.Join("bundled", descriptor.Directory, "plugin.json")))
	if err != nil {
		return false, err
	}
	installed, err := ParseStoreManifestJSON(installedRaw, true)
	if err != nil {
		return false, err
	}
	bundled, err := ParseStoreManifestJSON(bundledRaw, true)
	if err != nil {
		return false, err
	}
	comparison, err := compareSemanticVersions(installed.Version, bundled.Version)
	return comparison > 0, err
}

func isTrustedBundledPluginDirectory(id, directory string) bool {
	descriptor, exists := bundledPluginDescriptorForID(id)
	if !exists {
		return false
	}
	expected, err := hashPluginFS(bundledPlugins, filepath.ToSlash(filepath.Join("bundled", descriptor.Directory)))
	if err != nil {
		return false
	}
	actual, err := hashPluginDirectory(directory)
	return err == nil && actual == expected
}

type bundledPluginDescriptor struct {
	ID        string
	Directory string
}

var bundledPluginCache struct {
	sync.Once
	descriptors []bundledPluginDescriptor
	byID        map[string]bundledPluginDescriptor
	err         error
}

func bundledPluginDescriptors() ([]bundledPluginDescriptor, error) {
	bundledPluginCache.Do(func() {
		bundledPluginCache.byID = make(map[string]bundledPluginDescriptor)
		entries, err := fs.ReadDir(bundledPlugins, "bundled")
		if err != nil {
			bundledPluginCache.err = err
			return
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			raw, readErr := bundledPlugins.ReadFile(filepath.ToSlash(filepath.Join("bundled", entry.Name(), "plugin.json")))
			if readErr != nil {
				bundledPluginCache.err = fmt.Errorf("read bundled plugin %s manifest: %w", entry.Name(), readErr)
				return
			}
			manifest, parseErr := ParseStoreManifestJSON(raw, true)
			if parseErr != nil {
				bundledPluginCache.err = fmt.Errorf("validate bundled plugin %s manifest: %w", entry.Name(), parseErr)
				return
			}
			if _, exists := bundledPluginCache.byID[manifest.ID]; exists {
				bundledPluginCache.err = fmt.Errorf("duplicate bundled plugin id %q", manifest.ID)
				return
			}
			descriptor := bundledPluginDescriptor{ID: manifest.ID, Directory: entry.Name()}
			bundledPluginCache.descriptors = append(bundledPluginCache.descriptors, descriptor)
			bundledPluginCache.byID[manifest.ID] = descriptor
		}
		sort.Slice(bundledPluginCache.descriptors, func(i, j int) bool {
			return bundledPluginCache.descriptors[i].ID < bundledPluginCache.descriptors[j].ID
		})
	})
	return bundledPluginCache.descriptors, bundledPluginCache.err
}

func bundledPluginDescriptorForID(id string) (bundledPluginDescriptor, bool) {
	_, err := bundledPluginDescriptors()
	if err != nil {
		return bundledPluginDescriptor{}, false
	}
	descriptor, exists := bundledPluginCache.byID[id]
	return descriptor, exists
}

func isBundledPluginID(id string) bool {
	_, exists := bundledPluginDescriptorForID(id)
	return exists
}
