package core

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type SystemSetup struct {
	CertFile  string
	KeyFile   string
	CacheFile string
	Password  string
	aesCipher *AESCipher
}

func initSystem() *SystemSetup {
	if systemOnce == nil {
		systemOnce = &SystemSetup{
			aesCipher: NewAESCipher("resd48w2d7er95627d447c490a8f02ff"),
			CertFile:  filepath.Join(appOnce.UserDir, "ca.crt"),
			KeyFile:   filepath.Join(appOnce.UserDir, "ca.key"),
			CacheFile: filepath.Join(appOnce.UserDir, "pass.cache"),
		}
		systemOnce.checkPasswordFile()
	}
	return systemOnce
}

func GetSystemCertFile() string {
	if systemOnce == nil {
		return ""
	}
	return systemOnce.CertFile
}

func (s *SystemSetup) EnsureLocalCA() ([]byte, []byte, error) {
	cert, key, err := s.LoadLocalCA()
	if err == nil {
		return cert, key, nil
	}
	if !os.IsNotExist(err) {
		fmt.Println("Reload local CA failed, regenerating:", err.Error())
	}
	return s.generateLocalCA()
}

func (s *SystemSetup) LoadLocalCA() ([]byte, []byte, error) {
	certPEM, err := os.ReadFile(s.CertFile)
	if err != nil {
		return nil, nil, err
	}
	keyPEM, err := os.ReadFile(s.KeyFile)
	if err != nil {
		return nil, nil, err
	}

	if _, err := tls.X509KeyPair(certPEM, keyPEM); err != nil {
		return nil, nil, err
	}

	cert, err := s.loadCertificate()
	if err != nil {
		return nil, nil, err
	}
	if !cert.IsCA {
		return nil, nil, fmt.Errorf("local certificate is not a CA")
	}

	return certPEM, keyPEM, nil
}

func (s *SystemSetup) loadCertificate() (*x509.Certificate, error) {
	certPEM, err := os.ReadFile(s.CertFile)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return nil, fmt.Errorf("failed to decode certificate pem")
	}

	return x509.ParseCertificate(block.Bytes)
}

func (s *SystemSetup) generateLocalCA() ([]byte, []byte, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, err
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"res-downloader"},
			CommonName:   "res-downloader Local CA",
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, nil, err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	keyBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, nil, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes})

	if err := os.WriteFile(s.CertFile, certPEM, 0644); err != nil {
		return nil, nil, err
	}
	if err := os.WriteFile(s.KeyFile, keyPEM, 0600); err != nil {
		return nil, nil, err
	}

	return certPEM, keyPEM, nil
}

func (s *SystemSetup) certFingerprintSHA1() (string, error) {
	cert, err := s.loadCertificate()
	if err != nil {
		return "", err
	}

	sum := sha1.Sum(cert.Raw)
	return strings.ToUpper(hex.EncodeToString(sum[:])), nil
}

func (s *SystemSetup) SetPassword(password string, isCache bool) {
	s.Password = password
	if isCache {
		encrypted, err := s.aesCipher.Encrypt(password)
		if err == nil {
			err1 := os.WriteFile(s.CacheFile, []byte(encrypted), 0750)
			if err1 != nil {
				fmt.Println("Failed to write password: ", err1.Error())
			}
		} else {
			fmt.Println("Failed to Encrypt password: ", err.Error())
		}
	}
}

func (s *SystemSetup) checkPasswordFile() {
	fileInfo, err := os.Stat(s.CacheFile)
	if err != nil {
		return
	}

	lastModified := fileInfo.ModTime()
	oneMonthAgo := time.Now().AddDate(0, -1, 0)
	if lastModified.Before(oneMonthAgo) {
		os.Remove(s.CacheFile)
		return
	}

	content, err := os.ReadFile(s.CacheFile)
	if err != nil {
		return
	}

	password, err := s.aesCipher.Decrypt(string(content))
	if err != nil {
		return
	}
	s.Password = password
}
