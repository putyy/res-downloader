//go:build !linux

package system

const certificateMigrationVersion = 1

func (s *Setup) certificateInstallationComplete(fingerprint string) (bool, error) {
	return s.isCertificateInstalled(fingerprint)
}
