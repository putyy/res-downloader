package system

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	legacyCertificateSHA1       = "F0A43AE8E1884CE134A995271F5CD7B0BAF87080"
	certificateMigrationVersion = 1
)

type CertificateMigrationRecord struct {
	Version   int    `json:"version"`
	Status    string `json:"status"`
	Message   string `json:"message,omitempty"`
	CheckedAt int64  `json:"checkedAt"`
}

type DesktopCertificateStatus struct {
	Installed       bool   `json:"installed"`
	FingerprintSHA1 string `json:"fingerprintSha1,omitempty"`
	CheckedAt       int64  `json:"checkedAt"`
	Error           string `json:"error,omitempty"`
}

type CertificateStatus struct {
	FingerprintSHA256 string                     `json:"fingerprintSha256,omitempty"`
	Desktop           DesktopCertificateStatus   `json:"desktop"`
	Migration         CertificateMigrationRecord `json:"migration"`
	NeedsPhoneCleanup bool                       `json:"needsPhoneCleanup"`
	Error             string                     `json:"error,omitempty"`
}

func CurrentCertificateStatus(certificate []byte, certificateError string, setup *Setup) CertificateStatus {
	status := CertificateStatus{NeedsPhoneCleanup: true, Error: certificateError}
	status.Desktop.FingerprintSHA1 = CurrentCertificateSHA1(certificate)
	status.Desktop.CheckedAt = time.Now().UnixMilli()
	if setup != nil {
		status.Migration = setup.LegacyCertificateMigration
		if status.Desktop.FingerprintSHA1 != "" {
			installed, err := setup.isCertificateInstalled(status.Desktop.FingerprintSHA1)
			status.Desktop.Installed = installed
			if err != nil {
				status.Desktop.Error = err.Error()
			}
		}
	}
	block, _ := pem.Decode(certificate)
	if block != nil {
		digest := sha256.Sum256(block.Bytes)
		status.FingerprintSHA256 = strings.ToUpper(hex.EncodeToString(digest[:]))
	}
	return status
}

func CurrentCertificateSHA1(certificate []byte) string {
	block, _ := pem.Decode(certificate)
	if block == nil {
		return ""
	}
	digest := sha1.Sum(block.Bytes)
	return strings.ToUpper(hex.EncodeToString(digest[:]))
}

func certificateFileMatchesSHA1(path, fingerprint string) (bool, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return false, fmt.Errorf("certificate %s is not valid PEM", path)
	}
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false, fmt.Errorf("parse certificate %s: %w", path, err)
	}
	digest := sha1.Sum(certificate.Raw)
	return strings.EqualFold(hex.EncodeToString(digest[:]), fingerprint), nil
}

func RetryLegacyCertificateMigration(system *Setup, password string) CertificateMigrationRecord {
	setup := system
	if password != "" {
		setup = system.WithPassword(password)
	}
	status, message, err := setup.removeLegacyCertificate(legacyCertificateSHA1)
	status, message = certificateMigrationResult(status, message, err)
	record := CertificateMigrationRecord{Version: certificateMigrationVersion, Status: status, Message: message, CheckedAt: time.Now().UnixMilli()}
	system.LegacyCertificateMigration = record
	marker := filepath.Join(system.app.UserDir, "certificate-migration-v1.json")
	if raw, marshalErr := json.MarshalIndent(record, "", "  "); marshalErr == nil {
		_ = writePrivateFile(marker, raw)
	}
	return record
}

func certificateMigrationResult(status, message string, err error) (string, string) {
	if err == nil {
		return status, message
	}
	if status == "" {
		status = "failed"
	}
	if message == "" {
		message = err.Error()
	} else {
		message += ": " + err.Error()
	}
	return status, message
}

func InitializeCertificateAuthority(userDir string) ([]byte, []byte, error) {
	certPath := filepath.Join(userDir, "mitm-ca.crt")
	keyPath := filepath.Join(userDir, "mitm-ca.key")
	certPEM, certErr := os.ReadFile(certPath)
	keyPEM, keyErr := os.ReadFile(keyPath)
	if certErr == nil && keyErr == nil {
		if err := validateCertificateAuthority(certPEM, keyPEM); err != nil {
			return nil, nil, fmt.Errorf("stored device CA is invalid: %w", err)
		}
		return certPEM, keyPEM, nil
	}
	if !errors.Is(certErr, os.ErrNotExist) || !errors.Is(keyErr, os.ErrNotExist) {
		return nil, nil, fmt.Errorf("read device CA: certificate=%v key=%v", certErr, keyErr)
	}
	certPEM, keyPEM, err := generateCertificateAuthority()
	if err != nil {
		return nil, nil, err
	}
	if err := os.WriteFile(certPath, certPEM, 0644); err != nil {
		return nil, nil, err
	}
	if err := writePrivateFile(keyPath, keyPEM); err != nil {
		_ = os.Remove(certPath)
		return nil, nil, err
	}
	return certPEM, keyPEM, nil
}

func generateCertificateAuthority() ([]byte, []byte, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 3072)
	if err != nil {
		return nil, nil, err
	}
	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return nil, nil, err
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "res-downloader Device CA", Organization: []string{"res-downloader"}},
		NotBefore:    now.Add(-5 * time.Minute), NotAfter: now.AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true, IsCA: true, MaxPathLen: 0,
		SubjectKeyId: randomSubjectKeyID(),
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, nil, err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), nil
}

func randomSubjectKeyID() []byte {
	value := make([]byte, 20)
	if _, err := rand.Read(value); err != nil {
		return nil
	}
	return value
}

func validateCertificateAuthority(certPEM, keyPEM []byte) error {
	if _, err := tls.X509KeyPair(certPEM, keyPEM); err != nil {
		return err
	}
	block, _ := pem.Decode(certPEM)
	if block == nil {
		return errors.New("certificate PEM is invalid")
	}
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}
	if !certificate.IsCA || time.Now().After(certificate.NotAfter) {
		return errors.New("certificate is not a valid, unexpired CA")
	}
	return nil
}

func runLegacyCertificateMigration(setup *Setup) CertificateMigrationRecord {
	marker := filepath.Join(setup.app.UserDir, "certificate-migration-v1.json")
	if raw, err := os.ReadFile(marker); err == nil {
		var saved CertificateMigrationRecord
		if json.Unmarshal(raw, &saved) == nil && saved.Version == certificateMigrationVersion {
			return saved
		}
	}
	status, message, err := setup.removeLegacyCertificate(legacyCertificateSHA1)
	status, message = certificateMigrationResult(status, message, err)
	record := CertificateMigrationRecord{Version: certificateMigrationVersion, Status: status, Message: message, CheckedAt: time.Now().UnixMilli()}
	if raw, marshalErr := json.MarshalIndent(record, "", "  "); marshalErr == nil {
		if writeErr := writePrivateFile(marker, raw); writeErr != nil {
			setup.logger.Esg(writeErr, "write certificate migration marker")
		}
	}
	return record
}
