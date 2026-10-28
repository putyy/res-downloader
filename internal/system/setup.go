package system

import (
	"os"
	"path/filepath"
	"res-downloader/internal/config"
	"res-downloader/internal/logging"
)

type Environment struct {
	UserDir   string
	AppName   string
	PublicCrt []byte
}

type Setup struct {
	app                        Environment
	logger                     *logging.Logger
	config                     *config.Config
	CertFile                   string
	Password                   string
	LegacyCertificateMigration CertificateMigrationRecord
}

func NewSetup(app Environment, config *config.Config, logger *logging.Logger) *Setup {
	setup := &Setup{
		app:      app,
		logger:   logger,
		config:   config,
		CertFile: filepath.Join(app.UserDir, "mitm-ca.crt"),
	}
	setup.LegacyCertificateMigration = runLegacyCertificateMigration(setup)
	return setup
}

func (s *Setup) initCert() ([]byte, error) {
	content, err := os.ReadFile(s.CertFile)
	if err == nil {
		return content, nil
	}
	if os.IsNotExist(err) {
		err = os.WriteFile(s.CertFile, s.app.PublicCrt, 0644)
		if err != nil {
			return nil, err
		}
		return s.app.PublicCrt, nil
	} else {
		return nil, err
	}
}

// WithPassword returns an operation-scoped copy. The original Setup never
// retains administrator credentials and no password is written to disk.
func (s *Setup) WithPassword(password string) *Setup {
	temporary := *s
	temporary.Password = password
	return &temporary
}

func (s *Setup) SetProxy() error {
	return s.setProxy()
}

func (s *Setup) UnsetProxy() error {
	return s.unsetProxy()
}

func (s *Setup) InstallCertificate() (string, error) {
	return s.installCert()
}

func (s *Setup) IsCertificateInstalled(fingerprint string) (bool, error) {
	return s.isCertificateInstalled(fingerprint)
}

func (s *Setup) UninstallCertificate(fingerprint string) (string, error) {
	return s.uninstallCertificate(fingerprint)
}

func (s *Setup) RemoveLegacyCertificate(fingerprint string) (string, string, error) {
	return s.removeLegacyCertificate(fingerprint)
}
