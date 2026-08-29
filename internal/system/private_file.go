package system

import "os"

func writePrivateFile(fileName string, data []byte) error {
	if err := os.WriteFile(fileName, data, 0600); err != nil {
		return err
	}
	return os.Chmod(fileName, 0600)
}
