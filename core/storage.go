package core

import (
	"os"
	"path"
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
	if !FileExist(l.fileName) {
		err := os.WriteFile(l.fileName, l.def, 0777)
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
	if err := os.WriteFile(l.fileName, data, 0777); err != nil {
		return err
	}
	return nil
}
