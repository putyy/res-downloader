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

func NewStorage(root, filename string, def []byte) *Storage {
	return &Storage{
		fileName: filepath.Join(root, filename),
		def:      def,
	}
}

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
