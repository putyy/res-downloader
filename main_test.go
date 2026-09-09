package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWebView2InstallerModeSwitch(t *testing.T) {
	directory := t.TempDir()
	executable := filepath.Join(directory, "res-downloader.exe")
	runtimeDirectory := filepath.Join(directory, "WebView2Runtime")
	marker := filepath.Join(directory, ".res-downloader-system-webview2")
	if got := bundledWebView2PathForExecutable(executable); got != "" {
		t.Fatalf("installation without a fixed payload selected %q", got)
	}
	if err := os.Mkdir(runtimeDirectory, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runtimeDirectory, "msedgewebview2.exe"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if got := bundledWebView2PathForExecutable(executable); got != runtimeDirectory {
		t.Fatalf("legacy/fixed installation selected %q", got)
	}
	if err := os.WriteFile(marker, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if got := bundledWebView2PathForExecutable(executable); got != "" {
		t.Fatalf("standard upgrade selected leftover fixed payload %q", got)
	}
	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	if got := bundledWebView2PathForExecutable(executable); got != runtimeDirectory {
		t.Fatalf("switching back to fixed selected %q", got)
	}
}
