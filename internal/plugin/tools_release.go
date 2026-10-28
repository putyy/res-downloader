package plugin

import (
	"archive/zip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	shared "res-downloader/internal/model"
)

// verifyPluginRelease compiles plugin code without executing it and compares
// the committed archive against a fresh package produced by the same packer.
func verifyPluginRelease(directory, tag string) (shared.PluginManifest, error) {
	version, prefixed := strings.CutPrefix(tag, "v")
	if _, err := parseSemanticVersion(version); err != nil || !prefixed || strings.ContainsAny(version, "-+") {
		return shared.PluginManifest{}, fmt.Errorf("release tag %q must be vMAJOR.MINOR.PATCH", tag)
	}
	if _, err := os.Stat(filepath.Join(directory, "plugin.json")); err != nil {
		return shared.PluginManifest{}, fmt.Errorf("release requires root plugin.json: %w", err)
	}
	manifest, err := ValidatePluginDirectory(directory)
	if err != nil {
		return shared.PluginManifest{}, err
	}
	if manifest.Version != version {
		return shared.PluginManifest{}, fmt.Errorf("tag %q does not match plugin version %q", tag, manifest.Version)
	}
	temporary, err := os.MkdirTemp("", "resd-plugin-release-*")
	if err != nil {
		return shared.PluginManifest{}, err
	}
	defer os.RemoveAll(temporary)
	expected := filepath.Join(temporary, "plugin.zip")
	if err := packPluginDirectory(directory, expected); err != nil {
		return shared.PluginManifest{}, err
	}
	if err := comparePluginReleaseArchives(filepath.Join(directory, "dist", "plugin.zip"), expected); err != nil {
		return shared.PluginManifest{}, fmt.Errorf("verify dist/plugin.zip (repack before tagging): %w", err)
	}
	return manifest, nil
}

func comparePluginReleaseArchives(actualPath, expectedPath string) error {
	actual, err := zip.OpenReader(actualPath)
	if err != nil {
		return err
	}
	defer actual.Close()
	expected, err := zip.OpenReader(expectedPath)
	if err != nil {
		return err
	}
	defer expected.Close()
	if err := validatePluginArchiveEntries(&actual.Reader); err != nil {
		return err
	}
	if len(actual.File) != len(expected.File) {
		return fmt.Errorf("ZIP file list differs from source")
	}
	files := make(map[string]*zip.File, len(expected.File))
	for _, entry := range expected.File {
		files[entry.Name] = entry
	}
	for _, entry := range actual.File {
		source, exists := files[entry.Name]
		if !exists || !entry.Mode().IsRegular() {
			return fmt.Errorf("unexpected, duplicate, or non-regular ZIP entry %q", entry.Name)
		}
		delete(files, entry.Name)
		if entry.UncompressedSize64 != source.UncompressedSize64 {
			return fmt.Errorf("ZIP content differs from source for %q", entry.Name)
		}
		actualDigest, err := pluginReleaseEntryDigest(entry)
		if err != nil {
			return err
		}
		expectedDigest, err := pluginReleaseEntryDigest(source)
		if err != nil {
			return err
		}
		if actualDigest != expectedDigest {
			return fmt.Errorf("ZIP content differs from source for %q", entry.Name)
		}
	}
	return nil
}

func pluginReleaseEntryDigest(entry *zip.File) ([sha256.Size]byte, error) {
	var digest [sha256.Size]byte
	stream, err := entry.Open()
	if err != nil {
		return digest, err
	}
	defer stream.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, stream); err != nil {
		return digest, fmt.Errorf("read ZIP entry %q: %w", entry.Name, err)
	}
	copy(digest[:], hash.Sum(nil))
	return digest, nil
}
