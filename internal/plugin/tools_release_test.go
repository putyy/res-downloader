package plugin

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyPluginRelease(t *testing.T) {
	directory := t.TempDir()
	files := map[string]string{
		"plugin.json": `{"id":"official.example","name":"Example","version":"1.0.1","apiVersion":1,"runtime":"javascript","entry":"main.js","permissions":{"domains":["example.com"],"capabilities":[]},"match":[]}`,
		"main.js":     `function onObservation() { return {handled: false}; }`,
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(directory, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := packPluginDirectory(directory, filepath.Join(directory, "dist", "plugin.zip")); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := RunPluginCLI([]string{"verify-release", directory, "v1.0.1"}, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "official.example v1.0.1") {
		t.Fatalf("unexpected verification output: %q", output.String())
	}
	for _, tag := range []string{"1.0.1", "v01.0.1", "v1.0.1-rc.1", "v1.0.1+build", "v1.0.2"} {
		if _, err := verifyPluginRelease(directory, tag); err == nil {
			t.Errorf("accepted invalid or mismatched tag %q", tag)
		}
	}
	// A same-length edit must be detected by content, not only file size.
	source := strings.ReplaceAll(files["main.js"], "false", "null ")
	if err := os.WriteFile(filepath.Join(directory, "main.js"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := verifyPluginRelease(directory, "v1.0.1"); err == nil || !strings.Contains(err.Error(), "main.js") {
		t.Fatalf("stale package was not detected: %v", err)
	}
}

func TestComparePluginReleaseArchives(t *testing.T) {
	writeArchive := func(name string, method uint16, names []string, symlink bool) string {
		t.Helper()
		var buffer bytes.Buffer
		archive := zip.NewWriter(&buffer)
		for _, name := range names {
			header := &zip.FileHeader{Name: name, Method: method}
			if symlink {
				header.SetMode(os.ModeSymlink | 0600)
			}
			entry, err := archive.CreateHeader(header)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := entry.Write([]byte("same bytes")); err != nil {
				t.Fatal(err)
			}
		}
		if err := archive.Close(); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), name)
		if err := os.WriteFile(path, buffer.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	expected := writeArchive("expected.zip", zip.Deflate, []string{"plugin.json", "main.js"}, false)
	for _, tc := range []struct {
		name    string
		entries []string
		symlink bool
		valid   bool
	}{
		{"different compression and order", []string{"main.js", "plugin.json"}, false, true},
		{"duplicate", []string{"plugin.json", "plugin.json"}, false, false},
		{"missing", []string{"plugin.json"}, false, false},
		{"extra", []string{"plugin.json", "main.js", "extra.txt"}, false, false},
		{"traversal", []string{"plugin.json", "../main.js"}, false, false},
		{"symlink", []string{"plugin.json", "main.js"}, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actual := writeArchive("actual.zip", zip.Store, tc.entries, tc.symlink)
			err := comparePluginReleaseArchives(actual, expected)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v, error=%v", tc.valid, err)
			}
		})
	}
}
