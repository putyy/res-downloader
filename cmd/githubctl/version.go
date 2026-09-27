package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"res-downloader/internal/updates"
)

func runVersion(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("githubctl version", flag.ContinueOnError)
	flags.SetOutput(stderr)
	output := flags.String("output", "dist/version.json", "output version JSON path")
	if err := flags.Parse(args); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/repos/putyy/res-downloader/releases/latest", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "res-downloader-githubctl")
	if token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub: %s", response.Status)
	}
	var release struct {
		Tag         string `json:"tag_name"`
		URL         string `json:"html_url"`
		Body        string `json:"body"`
		PublishedAt string `json:"published_at"`
		Draft       bool   `json:"draft"`
		Prerelease  bool   `json:"prerelease"`
		Assets      []struct {
			Name   string `json:"name"`
			URL    string `json:"browser_download_url"`
			Size   int64  `json:"size"`
			Digest string `json:"digest"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(&release); err != nil {
		return err
	}
	version := strings.TrimPrefix(release.Tag, "v")
	if release.Draft || release.Prerelease || !updates.Stable(version) {
		return errors.New("latest release is not a stable version")
	}
	manifest := updates.Manifest{SchemaVersion: 1, Version: version, Tag: release.Tag, Notes: release.Body, ReleaseURL: release.URL, PublishedAt: release.PublishedAt, GeneratedAt: time.Now().UTC().Format(time.RFC3339), Assets: []updates.Asset{}}
	for _, asset := range release.Assets {
		platform, arch, variant := classifyAsset(asset.Name, version)
		if platform == "" {
			continue
		}
		digest := ""
		if strings.HasPrefix(asset.Digest, "sha256:") {
			digest = strings.TrimPrefix(asset.Digest, "sha256:")
		}
		manifest.Assets = append(manifest.Assets, updates.Asset{Platform: platform, Arch: arch, Variant: variant, Name: asset.Name, URL: asset.URL, Size: asset.Size, SHA256: digest})
	}
	if len(manifest.Assets) == 0 {
		return errors.New("stable release has no recognized installation assets")
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if _, err := updates.Decode(raw); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(*output), 0755); err != nil {
		return err
	}
	// Rename only a complete document so a server never serves a partially written JSON.
	temporary, err := os.CreateTemp(filepath.Dir(*output), ".version-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(temporary.Name())
	if err := temporary.Chmod(0644); err != nil {
		temporary.Close()
		return err
	}
	if _, err = temporary.Write(append(raw, '\n')); err != nil {
		temporary.Close()
		return err
	}
	if err = temporary.Close(); err != nil {
		return err
	}
	if err = os.Rename(temporary.Name(), *output); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "wrote %s: %s (%d assets)\n", *output, version, len(manifest.Assets))
	return nil
}
func classifyAsset(name, version string) (platform, arch, variant string) {
	prefix := "res-downloader_" + version + "_"
	if !strings.HasPrefix(name, prefix) {
		return
	}
	suffix := strings.TrimPrefix(name, prefix)
	if suffix == "mac.dmg" {
		return "darwin", "universal", "dmg"
	}
	for _, arch := range []string{"amd64", "arm64"} {
		switch suffix {
		case "win_" + arch + ".exe":
			return "windows", arch, "installer"
		case "win_" + arch + "_fixed_webview2.exe":
			return "windows", arch, "fixed-webview2"
		case "linux_" + arch:
			return "linux", arch, "binary"
		case "linux_" + arch + ".deb":
			return "linux", arch, "deb"
		case "linux_" + arch + ".AppImage":
			return "linux", arch, "appimage"
		}
	}
	return
}
