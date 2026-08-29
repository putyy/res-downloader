package resource

import (
	"errors"
	"io"
	"os"
)

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func xorFilePrefix(fileName string, key []byte) error {
	return xorFilePrefixAtOffset(fileName, key, 0)
}

func xorFilePrefixAtOffset(fileName string, key []byte, initialOffset uint64) error {
	if len(key) == 0 {
		return errors.New("xor prefix key must not be empty")
	}
	if initialOffset >= uint64(len(key)) {
		return nil
	}
	key = key[initialOffset:]
	file, err := os.OpenFile(fileName, os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	byteCount := len(key)
	fileBytes := make([]byte, byteCount)
	n, err := file.Read(fileBytes)
	if err != nil && err != io.EOF {
		return err
	}

	if n < byteCount {
		byteCount = n
	}

	xorResult := make([]byte, byteCount)
	for i := 0; i < byteCount; i++ {
		xorResult[i] = key[i] ^ fileBytes[i]
	}
	_, err = file.Seek(0, 0)
	if err != nil {
		return err
	}

	_, err = file.Write(xorResult)
	if err != nil {
		return err
	}
	return nil
}
