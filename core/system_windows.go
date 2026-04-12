//go:build windows

package core

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func (s *SystemSetup) setProxy() error {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()

	err = key.SetStringValue("ProxyServer", "127.0.0.1:"+globalConfig.Port)
	if err != nil {
		return err
	}

	err = key.SetDWordValue("ProxyEnable", 1)
	if err != nil {
		return err
	}
	return nil
}

func (s *SystemSetup) unsetProxy() error {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	err = key.SetDWordValue("ProxyEnable", 0)
	if err != nil {
		return err
	}
	return nil
}

func (s *SystemSetup) loadCertificateContext() (*x509.Certificate, error) {
	certData, err := os.ReadFile(s.CertFile)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(certData)
	if block == nil {
		return nil, errors.New("failed to parse certificate PEM")
	}

	return x509.ParseCertificate(block.Bytes)
}

func (s *SystemSetup) openRootStore() (windows.Handle, error) {
	rootStorePtr, err := windows.UTF16PtrFromString("ROOT")
	if err != nil {
		return 0, err
	}

	return windows.CertOpenStore(windows.CERT_STORE_PROV_SYSTEM, 0, 0, windows.CERT_SYSTEM_STORE_LOCAL_MACHINE, uintptr(unsafe.Pointer(rootStorePtr)))
}

func (s *SystemSetup) installCert() (string, error) {
	cert, err := s.loadCertificateContext()
	if err != nil {
		return "", errors.New("install cert: " + err.Error())
	}

	store, err := s.openRootStore()
	if err != nil {
		return "", errors.New("open root store: " + err.Error())
	}
	defer windows.CertCloseStore(store, 0)

	certContext, err := windows.CertCreateCertificateContext(windows.X509_ASN_ENCODING|windows.PKCS_7_ASN_ENCODING, &cert.Raw[0], uint32(len(cert.Raw)))
	if err != nil {
		return "", errors.New("create certificate context: " + err.Error())
	}
	defer windows.CertFreeCertificateContext(certContext)

	err = windows.CertAddCertificateContextToStore(store, certContext, windows.CERT_STORE_ADD_REPLACE_EXISTING, nil)
	if err != nil {
		return "", errors.New("add certificate to store: " + err.Error())
	}
	return "", nil
}

func (s *SystemSetup) isCertInstalled() (bool, error) {
	targetCert, err := s.loadCertificateContext()
	if err != nil {
		return false, err
	}

	store, err := s.openRootStore()
	if err != nil {
		return false, err
	}
	defer windows.CertCloseStore(store, 0)

	var current *windows.CertContext
	for {
		next, err := windows.CertEnumCertificatesInStore(store, current)
		if next == nil {
			if current != nil {
				windows.CertFreeCertificateContext(current)
			}
			if err != nil {
				return false, nil
			}
			return false, nil
		}

		encoded := unsafe.Slice(next.EncodedCert, next.Length)
		if bytes.Equal(encoded, targetCert.Raw) {
			windows.CertFreeCertificateContext(next)
			return true, nil
		}

		current = next
	}
}

func (s *SystemSetup) removeCert() error {
	targetCert, err := s.loadCertificateContext()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	store, err := s.openRootStore()
	if err != nil {
		return err
	}
	defer windows.CertCloseStore(store, 0)

	var current *windows.CertContext
	for {
		next, err := windows.CertEnumCertificatesInStore(store, current)
		if next == nil {
			if current != nil {
				windows.CertFreeCertificateContext(current)
			}
			if err != nil {
				return nil
			}
			return nil
		}

		encoded := unsafe.Slice(next.EncodedCert, next.Length)
		if bytes.Equal(encoded, targetCert.Raw) {
			return windows.CertDeleteCertificateFromStore(next)
		}

		current = next
	}
}
