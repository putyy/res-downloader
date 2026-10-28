package config

import (
	"os"
	"path/filepath"
	shared "res-downloader/internal/model"
)

type Storage struct {
	fileName string
	def      []byte
}

// NewStorage returns a file-backed store for filename under root. def is the
// content written when the file does not exist yet.
func NewStorage(root, filename string, def []byte) *Storage {
	return &Storage{
		fileName: filepath.Join(root, filename),
		def:      def,
	}
}

// Load reads the file, creating it from the default payload when missing, and
// restricts the file mode to 0600. It returns the raw bytes and any read,
// create or chmod error.
func (l *Storage) Load() ([]byte, error) {
	if !shared.FileExist(l.fileName) {
		err := l.Store(l.def)
		if err != nil {
			return nil, err
		}
		return l.def, nil
	}
	d, err := os.ReadFile(l.fileName)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(l.fileName, 0600); err != nil {
		return nil, err
	}
	return d, nil
}

// Store writes data atomically: the payload goes to a temporary file in the same
// directory, is synced, and is then renamed over the target. When the rename
// fails (for example because the target is locked on Windows) it retries once
// after moving the existing file aside, restoring the backup on failure.
func (l *Storage) Store(data []byte) error {
	directory := filepath.Dir(l.fileName)
	temporary, err := os.CreateTemp(directory, ".res-downloader-config-*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryName, l.fileName); err != nil {
		backupFile, backupErr := os.CreateTemp(directory, ".res-downloader-config-backup-*")
		if backupErr != nil {
			return err
		}
		backupName := backupFile.Name()
		_ = backupFile.Close()
		_ = os.Remove(backupName)
		if backupErr = os.Rename(l.fileName, backupName); backupErr != nil {
			return err
		}
		if replaceErr := os.Rename(temporaryName, l.fileName); replaceErr != nil {
			_ = os.Rename(backupName, l.fileName)
			return replaceErr
		}
		_ = os.Remove(backupName)
	}
	return os.Chmod(l.fileName, 0600)
}
