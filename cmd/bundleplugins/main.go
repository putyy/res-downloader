// bundleplugins prepares runtime-only plugins before Wails embeds them.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"res-downloader/internal/plugin"

	"gopkg.in/yaml.v3"
)

type bundledPlugin struct {
	ID         string `json:"id"`
	Repository string `json:"repository"`
}

var directoryName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

// manifestName is the project manifest shared with the runtime and the release
// pipeline. It declares the bundled plugins and the application version.
const manifestName = "app.json"

func main() {
	download := flag.Bool("download", false, "use latest remote sources instead of local plugins (release preparation)")
	flag.Parse()
	root, err := projectRoot()
	if err == nil {
		err = prepare(root, *download)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "prepare bundled plugins:", err)
		os.Exit(1)
	}
}

// The build tasks invoke this from the project root; manual invocations may
// start in any subdirectory.
func projectRoot() (string, error) {
	directory, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(directory, manifestName)); err == nil {
			return directory, nil
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", fmt.Errorf("run inside the res-downloader source tree")
		}
		directory = parent
	}
}

// checkBuildVersion keeps the Wails build metadata in sync with the manifest.
// build/config.yml drives the Windows version resource, the macOS Info.plist
// and the installer defines, so a stale value would ship a mislabelled build.
func checkBuildVersion(root, productVersion string) error {
	if productVersion == "" {
		return nil
	}
	raw, err := os.ReadFile(filepath.Join(root, "build", "config.yml"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var config struct {
		Info struct {
			Version string `yaml:"version"`
		} `yaml:"info"`
	}
	if err := yaml.Unmarshal(raw, &config); err != nil {
		return fmt.Errorf("parse build/config.yml: %w", err)
	}
	if config.Info.Version != productVersion {
		return fmt.Errorf("build/config.yml info.version %q must match %s info.productVersion %q", config.Info.Version, manifestName, productVersion)
	}
	return nil
}

func repositoryDirectory(repository string) (string, error) {
	u, err := url.Parse(repository)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("repository must be an HTTPS URL without credentials, query or fragment")
	}
	name := strings.TrimSuffix(path.Base(u.Path), ".git")
	if !directoryName.MatchString(name) {
		return "", fmt.Errorf("invalid repository directory %q", name)
	}
	return name, nil
}

func prepare(root string, download bool) error {
	raw, err := os.ReadFile(filepath.Join(root, manifestName))
	if err != nil {
		return err
	}
	var config struct {
		Plugins []bundledPlugin `json:"plugins"`
		Info    struct {
			ProductVersion string `json:"productVersion"`
		} `json:"info"`
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		return err
	}
	if config.Plugins == nil {
		return fmt.Errorf("%s requires a plugins array", manifestName)
	}
	if err := checkBuildVersion(root, config.Info.ProductVersion); err != nil {
		return err
	}
	ids, names := map[string]bool{}, map[string]bool{}
	for _, item := range config.Plugins {
		name, err := repositoryDirectory(item.Repository)
		if err != nil {
			return fmt.Errorf("%s: %w", item.ID, err)
		}
		if !strings.HasPrefix(item.ID, "official.") || ids[item.ID] || names[name] {
			return fmt.Errorf("bundled plugins require unique official IDs and repository directory names: %q", item.ID)
		}
		ids[item.ID], names[name] = true, true
	}
	target := filepath.Join(root, "internal", "plugin", "bundled")
	stage, err := os.MkdirTemp(filepath.Dir(target), ".bundle-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	// Preserve the tracked placeholder that makes the first preparation compilable.
	readme, err := os.ReadFile(filepath.Join(target, "README.md"))
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(stage, "README.md"), readme, 0644); err != nil {
		return err
	}
	for _, item := range config.Plugins {
		name, _ := repositoryDirectory(item.Repository)
		source := filepath.Join(root, "plugins", name)
		if download {
			// Download separately so release preparation never modifies local work.
			scratch, err := os.MkdirTemp("", "resd-bundled-*")
			if err != nil {
				return err
			}
			defer os.RemoveAll(scratch)
			source = filepath.Join(scratch, name)
			clone := exec.Command("git", "clone", "--depth=1", "--", item.Repository, source)
			clone.Stdout, clone.Stderr = os.Stdout, os.Stderr
			if err := clone.Run(); err != nil {
				return fmt.Errorf("download %s: %w", item.ID, err)
			}
			commit, err := exec.Command("git", "-C", source, "rev-parse", "HEAD").Output()
			if err != nil {
				return err
			}
			fmt.Printf("%s commit %s\n", item.ID, strings.TrimSpace(string(commit)))
		} else if _, err := os.Stat(source); err != nil {
			return fmt.Errorf("missing local plugin %s; clone %s into plugins/%s: %w", item.ID, item.Repository, name, err)
		}
		if err := copyRuntimePlugin(source, filepath.Join(stage, name), item.ID); err != nil {
			return fmt.Errorf("%s: %w", item.ID, err)
		}
	}
	// Replace only generated files, after every plugin has passed static validation.
	if err := os.RemoveAll(target); err != nil {
		return err
	}
	return os.Rename(stage, target)
}

func copyRuntimePlugin(source, target, expectedID string) error {
	manifest, err := plugin.ValidatePluginDirectory(source)
	if err != nil {
		return err
	}
	if manifest.ID != expectedID {
		return fmt.Errorf("manifest ID %q does not match configured ID %q", manifest.ID, expectedID)
	}
	files := map[string]bool{"plugin.json": true}
	if manifest.Entry != "" {
		files[manifest.Entry] = true
	}
	for _, script := range manifest.PageScripts {
		files[script.Entry] = true
	}
	for _, processor := range manifest.Processors {
		files[processor.Entry] = true
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := strings.ToUpper(entry.Name())
		if !entry.IsDir() && (strings.HasPrefix(name, "LICENSE") || strings.HasPrefix(name, "COPYING") || strings.HasPrefix(name, "NOTICE")) {
			files[entry.Name()] = true
		}
	}
	paths := make([]string, 0, len(files))
	for name := range files {
		paths = append(paths, name)
	}
	sort.Strings(paths)
	for _, name := range paths {
		if !fs.ValidPath(name) || strings.Contains(name, `\`) {
			return fmt.Errorf("invalid runtime file %q", name)
		}
		file := source
		for _, component := range strings.Split(name, "/") {
			if strings.HasPrefix(component, ".") || strings.HasPrefix(component, "_") {
				return fmt.Errorf("runtime file %q would be excluded by Go embed", name)
			}
			file = filepath.Join(file, component)
			info, err := os.Lstat(file)
			if err != nil {
				return err
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("runtime file %q contains a symbolic link", name)
			}
		}
		content, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(destination), 0750); err != nil {
			return err
		}
		if err := os.WriteFile(destination, content, 0644); err != nil {
			return err
		}
	}
	_, err = plugin.ValidatePluginDirectory(target)
	if err == nil {
		fmt.Printf("prepared %s v%s\n", manifest.ID, manifest.Version)
	}
	return err
}
