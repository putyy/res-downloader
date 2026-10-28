package resource

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	shared "res-downloader/internal/model"
)

func (r *Resource) executeProcessorsAtOffset(path string, processors []shared.DownloadStep, initialOffset uint64, _ bool) error {
	if len(processors) == 0 {
		return nil
	}
	workingPath, err := copyToProcessingFile(path)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(workingPath) }()

	for _, processor := range processors {
		switch processor.Type {
		case "xor-prefix":
			key, _ := processor.Options["key"].(string)
			if key == "" {
				return errors.New("xor-prefix processor requires key")
			}
			decodedKey, err := base64.StdEncoding.DecodeString(key)
			if err != nil {
				return fmt.Errorf("decode xor-prefix key: %w", err)
			}
			if err := xorFilePrefixAtOffset(workingPath, decodedKey, initialOffset); err != nil {
				return err
			}
		case "plugin-wasm":
			nextPath, err := r.plugins.ProcessWASM(context.Background(), processor, workingPath, initialOffset)
			if err != nil {
				return err
			}
			if err := os.Remove(workingPath); err != nil {
				_ = os.Remove(nextPath)
				return err
			}
			workingPath = nextPath
		default:
			return fmt.Errorf("unsupported processor %q", processor.Type)
		}
	}
	if err := replaceProcessedDownload(workingPath, path); err != nil {
		return fmt.Errorf("replace processed download: %w", err)
	}
	workingPath = ""
	return nil
}

func (r *Resource) ProcessDownload(path string, processors []shared.DownloadStep, initialOffset uint64, reportProgress bool) error {
	return r.executeProcessorsAtOffset(path, processors, initialOffset, reportProgress)
}

func replaceProcessedDownload(processedPath, destinationPath string) error {
	if _, err := os.Stat(destinationPath); err != nil {
		if os.IsNotExist(err) {
			return os.Rename(processedPath, destinationPath)
		}
		return err
	}

	// Both paths are expected to share a filesystem. Retain the destination
	// until the replacement has been committed so processing stays transactional.
	backup, err := os.CreateTemp(filepath.Dir(destinationPath), ".res-downloader-original-*")
	if err != nil {
		return err
	}
	backupPath := backup.Name()
	if err := backup.Close(); err != nil {
		_ = os.Remove(backupPath)
		return err
	}
	defer os.Remove(backupPath)
	if err := os.Remove(backupPath); err != nil {
		return err
	}
	if err := os.Rename(destinationPath, backupPath); err != nil {
		return err
	}
	if err := os.Rename(processedPath, destinationPath); err != nil {
		if restoreErr := os.Rename(backupPath, destinationPath); restoreErr != nil {
			return fmt.Errorf("install processed file: %v; restore original: %w", err, restoreErr)
		}
		return err
	}
	return nil
}

func copyToProcessingFile(sourcePath string) (string, error) {
	source, err := os.Open(sourcePath)
	if err != nil {
		return "", err
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil {
		return "", err
	}
	target, err := os.CreateTemp(filepath.Dir(sourcePath), ".res-downloader-processing-*")
	if err != nil {
		return "", err
	}
	targetPath := target.Name()
	succeeded := false
	defer func() {
		_ = target.Close()
		if !succeeded {
			_ = os.Remove(targetPath)
		}
	}()
	if err := target.Chmod(info.Mode().Perm()); err != nil {
		return "", err
	}
	if _, err := io.Copy(target, source); err != nil {
		return "", err
	}
	if err := target.Sync(); err != nil {
		return "", err
	}
	if err := target.Close(); err != nil {
		return "", err
	}
	succeeded = true
	return targetPath, nil
}
