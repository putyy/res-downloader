package core

import (
	"os"
	"path"
	"res-downloader/core/shared"
)

type Storage struct {
	fileName string
	def      []byte
}

func NewStorage(filename string, def []byte) *Storage {
	return &Storage{
		fileName: path.Join(appOnce.UserDir, filename),
		def:      def,
	}
}

func (l *Storage) Load() ([]byte, error) {
	if !shared.FileExist(l.fileName) {
		err := os.WriteFile(l.fileName, l.def, 0644)
		if err != nil {
			return nil, err
		}
		return l.def, nil
	}
	d, err := os.ReadFile(l.fileName)
	if err != nil {
		return nil, err
	}
	return d, err
}

func (l *Storage) Store(data []byte) error {
	if err := os.WriteFile(l.fileName, data, 0644); err != nil {
		return err
	}
	return nil
}
