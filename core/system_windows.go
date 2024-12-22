//go:build windows

package core

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"unsafe"
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

func (s *SystemSetup) installCert() (string, error) {
	certData, err := s.initCert()
	if err != nil {
		return "", errors.New("installCert1:" + err.Error())
	}

	block, _ := pem.Decode(certData)
	if block == nil {
		return "", errors.New("Failed to parse certificate PEM" + err.Error())
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", errors.New("installCert3:" + err.Error())
	}

	rootStorePtr, err := windows.UTF16PtrFromString("ROOT")
	if err != nil {
		return "", errors.New("installCert4:" + err.Error())
	}

	store, err := windows.CertOpenStore(windows.CERT_STORE_PROV_SYSTEM, 0, 0, windows.CERT_SYSTEM_STORE_LOCAL_MACHINE, uintptr(unsafe.Pointer(rootStorePtr)))
	if err != nil {
		return "", errors.New("installCert5:" + err.Error())
	}
	defer windows.CertCloseStore(store, 0)

	certContext, err := windows.CertCreateCertificateContext(windows.X509_ASN_ENCODING|windows.PKCS_7_ASN_ENCODING, &cert.Raw[0], uint32(len(cert.Raw)))
	if err != nil {
		return "", errors.New("installCert6:" + err.Error())
	}
	defer windows.CertFreeCertificateContext(certContext)
	err = windows.CertAddCertificateContextToStore(store, certContext, windows.CERT_STORE_ADD_REPLACE_EXISTING, nil)
	if err != nil {
		return "", errors.New("installCert7:" + err.Error())
	}
	return "", nil
}
