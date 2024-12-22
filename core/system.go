package core

import (
	"os"
	"path/filepath"
)

type SystemSetup struct {
	CertFile string
}

func initSystem() *SystemSetup {
	if systemOnce == nil {
		systemOnce = &SystemSetup{
			CertFile: filepath.Join(appOnce.UserDir, "cert.crt"),
		}
	}
	return systemOnce
}

func (s *SystemSetup) initCert() ([]byte, error) {
	content, err := os.ReadFile(s.CertFile)
	if err == nil {
		return content, nil
	}
	if os.IsNotExist(err) {
		err = os.WriteFile(s.CertFile, appOnce.PublicCrt, 0777)
		if err != nil {
			return nil, err
		}
		return appOnce.PublicCrt, nil
	} else {
		return nil, err
	}
}
