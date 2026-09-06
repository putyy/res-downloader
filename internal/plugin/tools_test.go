package plugin

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPackPluginDirectoryExcludesDevelopmentFilesAndDirectories(t *testing.T) {
	directory := t.TempDir()
	manifest := `{"id":"test.pack","name":"Pack","version":"1.0.0","apiVersion":1,"runtime":"javascript","entry":"main.js","permissions":{"domains":["example.com"],"capabilities":[]},"match":[]}`
	for name, content := range map[string]string{
		"plugin.json":           manifest,
		"main.js":               `function onObservation() { return {decision: "continue"} }`,
		".git/config":           "secret",
		".idea/workspace.xml":   "local workspace",
		".vscode/settings.json": "{}",
		".gitignore":            "dist/",
		".DS_Store":             "finder metadata",
		"README.md":             "development documentation",
		"LICENSE":               "license text",
		"dist/old.zip":          "old",
		"tests/main.test.js":    `throw new Error("development only")`,
		"fixtures/video.json":   `{}`,
	} {
		fileName := filepath.Join(directory, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(fileName), 0750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fileName, []byte(content), 0640); err != nil {
			t.Fatal(err)
		}
	}
	output := filepath.Join(directory, "dist", "plugin.zip")
	if err := packPluginDirectory(directory, output); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.OpenReader(output)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	seen := make(map[string]bool)
	for _, entry := range archive.File {
		seen[entry.Name] = true
		if strings.HasPrefix(entry.Name, ".git/") || strings.HasPrefix(entry.Name, ".idea/") || strings.HasPrefix(entry.Name, ".vscode/") || strings.HasPrefix(entry.Name, "dist/") || strings.HasPrefix(entry.Name, "tests/") {
			t.Fatalf("pack included generated or repository metadata: %q", entry.Name)
		}
	}
	for _, excluded := range []string{".gitignore", ".DS_Store", "README.md", "LICENSE"} {
		if seen[excluded] {
			t.Fatalf("pack included excluded file %q", excluded)
		}
	}
	if !seen["plugin.json"] || !seen["main.js"] || !seen["fixtures/video.json"] {
		t.Fatalf("missing plugin files: %#v", seen)
	}
}

func TestPackPluginDirectoryAllowsOfficialID(t *testing.T) {
	directory := t.TempDir()
	manifest := `{"id":"official.douyin","name":"Douyin","version":"1.0.0","apiVersion":1,"runtime":"javascript","entry":"main.js","permissions":{"domains":["*.douyin.com"],"capabilities":[]},"match":[]}`
	if err := os.WriteFile(filepath.Join(directory, "plugin.json"), []byte(manifest), 0640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "main.js"), []byte(`function onObservation() { return {decision: "continue"} }`), 0640); err != nil {
		t.Fatal(err)
	}
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relativeDirectory, err := filepath.Rel(workingDirectory, directory)
	if err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(relativeDirectory, "dist", "plugin.zip")
	if err := packPluginDirectory(relativeDirectory, output); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.OpenReader(output)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
}

func TestRunPluginCLIPackUsesDefaultDistOutput(t *testing.T) {
	directory := t.TempDir()
	manifest := `{"id":"test.default-pack","name":"Pack","version":"1.0.0","apiVersion":1,"runtime":"javascript","entry":"main.js","permissions":{"domains":["example.com"],"capabilities":[]},"match":[]}`
	if err := os.WriteFile(filepath.Join(directory, "plugin.json"), []byte(manifest), 0640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "main.js"), []byte(`function onObservation() { return {decision: "continue"} }`), 0640); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := RunPluginCLI([]string{"pack", directory}, &output); err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join(directory, "dist", "plugin.zip")
	if strings.TrimSpace(output.String()) != expected {
		t.Fatalf("pack output = %q, expected %q", strings.TrimSpace(output.String()), expected)
	}
	archive, err := zip.OpenReader(expected)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
}

func TestRunPluginCLICreateInteractiveUsesDefaults(t *testing.T) {
	pluginsDirectory := t.TempDir()
	input := strings.NewReader(pluginsDirectory + "\n\n\n")
	var output bytes.Buffer
	if err := RunPluginCLIWithInput([]string{"create"}, input, &output); err != nil {
		t.Fatal(err)
	}

	directory := filepath.Join(pluginsDirectory, "com.example.my-plugin")
	manifestRaw, err := os.ReadFile(filepath.Join(directory, "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	manifest := string(manifestRaw)
	if !strings.Contains(manifest, `"id": "com.example.my-plugin"`) || !strings.Contains(manifest, `"name": "com.example.my-plugin"`) {
		t.Fatalf("manifest does not contain default identity: %s", manifest)
	}
	if !strings.Contains(output.String(), "Plugins directory [./plugins]") || !strings.Contains(output.String(), "created plugin com.example.my-plugin") {
		t.Fatalf("unexpected create output: %q", output.String())
	}
	assertPluginScaffoldDevelopmentFiles(t, directory)
}

func TestRunPluginCLICreateAcceptsDisplayNameArgument(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "video-plugin")
	var output bytes.Buffer
	if err := RunPluginCLIWithInput([]string{"create", directory, "com.example.video", "Example Video"}, strings.NewReader(""), &output); err != nil {
		t.Fatal(err)
	}

	manifestRaw, err := os.ReadFile(filepath.Join(directory, "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(manifestRaw), `"name": "Example Video"`) {
		t.Fatalf("manifest does not contain display name: %s", manifestRaw)
	}
	assertPluginScaffoldDevelopmentFiles(t, directory)
}

func TestRunPluginCLICreateEOFDoesNotWriteScaffold(t *testing.T) {
	for _, suffix := range []string{"", "\n", "\ncom.example.video\n", "\ncom.example.video\nUnfinished name"} {
		parent := t.TempDir()
		directory := filepath.Join(parent, "plugins")
		var output bytes.Buffer
		err := RunPluginCLIWithInput([]string{"create"}, strings.NewReader(directory+suffix), &output)
		if !errors.Is(err, io.EOF) {
			t.Fatalf("suffix=%q: expected EOF cancellation, got %v", suffix, err)
		}
		if _, err := os.Stat(directory); !os.IsNotExist(err) {
			t.Fatalf("creation wrote files after EOF: %v", err)
		}
	}
	if err := RunPluginCLIWithInput([]string{"create"}, strings.NewReader(""), io.Discard); !errors.Is(err, io.EOF) {
		t.Fatalf("expected empty input to cancel, got %v", err)
	}
}

func assertPluginScaffoldDevelopmentFiles(t *testing.T, directory string) {
	t.Helper()
	gitignore, err := os.ReadFile(filepath.Join(directory, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if string(gitignore) != ".idea\n.vscode\n" {
		t.Fatalf("unexpected .gitignore: %q", gitignore)
	}
	readme, err := os.ReadFile(filepath.Join(directory, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(readme), "## Development") {
		t.Fatalf("README is missing development guidance: %s", readme)
	}
	if _, err := os.Stat(filepath.Join(directory, "fixtures")); err != nil {
		t.Fatal(err)
	}
}

func TestSyncBundledPluginKeepsSourceDirectoryName(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "resd-plugin-wechat")
	bundledRoot := filepath.Join(root, "bundled")
	for name, content := range map[string]string{
		"plugin.json":         `{"id":"official.wechat","name":"WeChat","version":"1.0.0","apiVersion":1,"runtime":"javascript","entry":"main.js","permissions":{"domains":["*.weixin.qq.com"],"capabilities":[]},"match":[]}`,
		"main.js":             `function onObservation() { return {decision: "continue"} }`,
		"fixtures/video.json": `{}`,
		"tests/main.test.js":  `throw new Error("development only")`,
		"dist/plugin.zip":     `generated`,
		".git/config":         `private`,
		".gitignore":          `dist/`,
	} {
		path := filepath.Join(source, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0640); err != nil {
			t.Fatal(err)
		}
	}
	oldDirectory := filepath.Join(bundledRoot, "official.wechat")
	if err := os.MkdirAll(oldDirectory, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldDirectory, "plugin.json"), []byte(`{"id":"official.wechat","name":"Old","version":"0.9.0","apiVersion":1,"runtime":"javascript","entry":"main.js","permissions":{"domains":["*.weixin.qq.com"],"capabilities":[]},"match":[]}`), 0640); err != nil {
		t.Fatal(err)
	}

	manifest, target, err := syncBundledPluginToRoot(source, bundledRoot)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.ID != "official.wechat" || filepath.Base(target) != "resd-plugin-wechat" {
		t.Fatalf("unexpected synchronized plugin: manifest=%#v target=%q", manifest, target)
	}
	if _, err := os.Stat(oldDirectory); !os.IsNotExist(err) {
		t.Fatalf("old manifest-ID directory still exists: %v", err)
	}
	for _, excluded := range []string{".git", ".gitignore", "dist", "tests"} {
		if _, err := os.Stat(filepath.Join(target, excluded)); !os.IsNotExist(err) {
			t.Fatalf("excluded directory %q exists: %v", excluded, err)
		}
	}
	if _, err := os.Stat(filepath.Join(target, "fixtures", "video.json")); err != nil {
		t.Fatalf("fixture was not synchronized: %v", err)
	}
}
