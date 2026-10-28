package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareCopiesOnlyRuntimeFilesAndKeepsPreviousOutputOnValidationFailure(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "plugins", "resd-plugin-example")
	manifest := `{"id":"official.example","name":"Example","version":"1.0.0","apiVersion":1,
"runtime":"javascript","entry":"main.js",
"permissions":{"domains":["example.com"],"capabilities":["process-download","inject-page-script"]},
"pageScripts":[{"id":"page","entry":"page/inject.js","match":[{"host":"example.com"}]}],
"processors":{"decrypt":{"runtime":"wasm","entry":"processor.wasm","apiVersion":1}}}`
	files := map[string]string{
		"plugin.json": manifest,
		// Preparation compiles source; it must never execute plugin code.
		"main.js":            `throw new Error("must not execute during preparation")`,
		"page/inject.js":     `console.log("example")`,
		"processor.wasm":     "\x00asm\x01\x00\x00\x00\x00\x01\x00",
		"LICENSE":            "example license",
		"NOTICE.txt":         "third-party notices",
		"README.md":          "development docs",
		"README-EN.md":       "development docs",
		"tests/main.test.js": "test code",
		"fixtures/api.json":  "fixture",
		"decrypt.wat":        "wasm source",
		"dist/plugin.zip":    "release artifact",
		".git/config":        "repository metadata",
	}
	for name, content := range files {
		writeFile(t, filepath.Join(source, filepath.FromSlash(name)), content)
	}
	writeFile(t, filepath.Join(root, "app.json"), `{"name":"Host","preBuildHooks":{},"plugins":[{"id":"official.example","repository":"https://github.com/example/resd-plugin-example.git"}]}`)
	generated := filepath.Join(root, "internal", "plugin", "bundled")
	writeFile(t, filepath.Join(generated, "README.md"), "bootstrap")
	writeFile(t, filepath.Join(generated, "obsolete", "main.js"), "old plugin")
	if err := prepare(root, false); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(generated, "resd-plugin-example")
	for _, name := range []string{"plugin.json", "main.js", "page/inject.js", "processor.wasm", "LICENSE", "NOTICE.txt"} {
		content, err := os.ReadFile(filepath.Join(target, filepath.FromSlash(name)))
		if err != nil || string(content) != files[name] {
			t.Fatalf("runtime file %s differs: %v", name, err)
		}
	}
	for _, name := range []string{"README.md", "README-EN.md", "tests", "fixtures", "decrypt.wat", "dist", ".git"} {
		if _, err := os.Stat(filepath.Join(target, name)); !os.IsNotExist(err) {
			t.Fatalf("development file %s was included: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(generated, "obsolete")); !os.IsNotExist(err) {
		t.Fatalf("obsolete plugin survived regeneration: %v", err)
	}
	writeFile(t, filepath.Join(source, "plugin.json"), strings.Replace(manifest, "official.example", "official.other", 1))
	if err := prepare(root, false); err == nil {
		t.Fatal("mismatched plugin ID was accepted")
	}
	if content, err := os.ReadFile(filepath.Join(target, "plugin.json")); err != nil || string(content) != manifest {
		t.Fatalf("failed preparation replaced previous output: %v", err)
	}
}

func TestPrepareRejectsMissingSourceAndConflictingConfiguration(t *testing.T) {
	for _, config := range []string{
		`{"plugins":[{"id":"official.example","repository":"https://github.com/example/missing.git"}]}`,
		`{"plugins":[{"id":"com.example.plugin","repository":"https://github.com/example/plugin.git"}]}`,
		`{"plugins":[{"id":"official.example","repository":"https://github.com/a/plugin.git"},{"id":"official.other","repository":"https://github.com/b/plugin.git"}]}`,
	} {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "app.json"), config)
		writeFile(t, filepath.Join(root, "internal", "plugin", "bundled", "README.md"), "bootstrap")
		if err := prepare(root, false); err == nil {
			t.Fatalf("invalid preparation succeeded: %s", config)
		}
	}
}

func TestPrepareRejectsBuildConfigVersionMismatch(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, manifestName), `{"info":{"productVersion":"4.0.0"},"plugins":[]}`)
	writeFile(t, filepath.Join(root, "build", "config.yml"), "version: '3'"+"\n"+"info:"+"\n"+"  version: \"3.9.9\""+"\n")
	err := prepare(root, false)
	if err == nil || !strings.Contains(err.Error(), "must match") {
		t.Fatalf("version mismatch was accepted: %v", err)
	}
}

func TestPrepareAcceptsMatchingBuildConfigVersion(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, manifestName), `{"info":{"productVersion":"4.0.0"},"plugins":[]}`)
	writeFile(t, filepath.Join(root, "build", "config.yml"), "version: '3'"+"\n"+"info:"+"\n"+"  version: \"4.0.0\""+"\n")
	writeFile(t, filepath.Join(root, "internal", "plugin", "bundled", "README.md"), "bootstrap")
	if err := prepare(root, false); err != nil {
		t.Fatalf("matching version was rejected: %v", err)
	}
}

func writeFile(t *testing.T, name, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}
