//go:build windows

package system

import (
	"crypto/sha1"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

const shellExecuteMaskNoCloseProcess = 0x00000040

var shellExecuteExW = windows.NewLazySystemDLL("shell32.dll").NewProc("ShellExecuteExW")

type shellExecuteInfo struct {
	Size       uint32
	Mask       uint32
	Window     windows.Handle
	Verb       *uint16
	File       *uint16
	Parameters *uint16
	Directory  *uint16
	Show       int32
	Instance   windows.Handle
	IDList     uintptr
	Class      *uint16
	ClassKey   windows.Handle
	HotKey     uint32
	Icon       windows.Handle
	Process    windows.Handle
}

func (s *Setup) removeLegacyCertificate(fingerprint string) (string, string, error) {
	installed, err := s.isCertificateInstalled(fingerprint)
	if err != nil {
		return "failed", "", fmt.Errorf("inspect legacy desktop certificate: %w", err)
	}
	if !installed {
		return "notFound", "legacy desktop certificate was not installed; remove it manually from any phones that trusted it", nil
	}
	if err := runElevatedSystemTool("certutil.exe", "-delstore", "Root", fingerprint); err != nil {
		return "needsManualCleanup", "", fmt.Errorf("remove legacy desktop certificate: %w", err)
	}
	remaining, err := s.isCertificateInstalled(fingerprint)
	if err != nil {
		return "needsManualCleanup", "", fmt.Errorf("verify legacy desktop certificate cleanup: %w", err)
	}
	if remaining {
		return "needsManualCleanup", "", errors.New("legacy desktop certificate is still present after the elevated delete command")
	}
	return "removed", "legacy desktop certificate removed; certificates installed on phones must be removed manually", nil
}

func (s *Setup) isCertificateInstalled(fingerprint string) (bool, error) {
	rootStorePtr, err := windows.UTF16PtrFromString("ROOT")
	if err != nil {
		return false, err
	}
	store, err := windows.CertOpenStore(windows.CERT_STORE_PROV_SYSTEM, 0, 0, windows.CERT_SYSTEM_STORE_LOCAL_MACHINE, uintptr(unsafe.Pointer(rootStorePtr)))
	if err != nil {
		return false, err
	}
	defer windows.CertCloseStore(store, 0)

	var previous *windows.CertContext
	for {
		context, enumErr := windows.CertEnumCertificatesInStore(store, previous)
		if context == nil {
			// CertEnumCertificatesInStore reports the end of the store as an
			// error. Opening the store above already established that it is
			// readable, so reaching the end simply means no match.
			_ = enumErr
			return false, nil
		}
		raw := unsafe.Slice(context.EncodedCert, context.Length)
		digest := sha1.Sum(raw)
		if strings.EqualFold(hex.EncodeToString(digest[:]), fingerprint) {
			_ = windows.CertFreeCertificateContext(context)
			return true, nil
		}
		previous = context
	}
}

func (s *Setup) uninstallCertificate(fingerprint string) (string, error) {
	installed, err := s.isCertificateInstalled(fingerprint)
	if err != nil || !installed {
		return "", err
	}
	if err := runElevatedSystemTool("certutil.exe", "-delstore", "Root", fingerprint); err != nil {
		return "", fmt.Errorf("remove current device certificate: %w", err)
	}
	return "", nil
}

func (s *Setup) setProxy() error {
	port := s.config.Snapshot().Port
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()

	err = key.SetStringValue("ProxyServer", "127.0.0.1:"+port)
	if err != nil {
		return err
	}

	err = key.SetDWordValue("ProxyEnable", 1)
	if err != nil {
		return err
	}
	return nil
}

func (s *Setup) unsetProxy() error {
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

func (s *Setup) installCert() (string, error) {
	certData, err := s.initCert()
	if err != nil {
		return "", errors.New("installCert1:" + err.Error())
	}

	block, _ := pem.Decode(certData)
	if block == nil {
		return "", errors.New("parse current device certificate: invalid PEM")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("parse current device certificate: %w", err)
	}

	if len(cert.Raw) == 0 {
		return "", errors.New("installCert4: certificate is empty")
	}
	if err := runElevatedSystemTool("certutil.exe", "-addstore", "-f", "Root", s.CertFile); err != nil {
		return "", fmt.Errorf("install current device certificate: %w", err)
	}
	return "", nil
}

func runElevatedSystemTool(program string, args ...string) error {
	systemDirectory, err := windows.GetSystemDirectory()
	if err != nil {
		return fmt.Errorf("locate Windows system directory: %w", err)
	}
	return runElevated(filepath.Join(systemDirectory, program), args...)
}

func runElevated(program string, args ...string) error {
	verb, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}
	file, err := windows.UTF16PtrFromString(program)
	if err != nil {
		return err
	}
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, syscall.EscapeArg(arg))
	}
	parameters, err := windows.UTF16PtrFromString(strings.Join(quoted, " "))
	if err != nil {
		return err
	}
	info := shellExecuteInfo{
		Mask:       shellExecuteMaskNoCloseProcess,
		Verb:       verb,
		File:       file,
		Parameters: parameters,
		Show:       windows.SW_HIDE,
	}
	info.Size = uint32(unsafe.Sizeof(info))
	result, _, callErr := shellExecuteExW.Call(uintptr(unsafe.Pointer(&info)))
	if result == 0 {
		if callErr != nil && callErr != syscall.Errno(0) {
			return callErr
		}
		return errors.New("administrator authorization was not granted")
	}
	if info.Process == 0 {
		return errors.New("elevated certificate process was not started")
	}
	defer windows.CloseHandle(info.Process)
	waitResult, err := windows.WaitForSingleObject(info.Process, windows.INFINITE)
	if err != nil {
		return err
	}
	if waitResult != windows.WAIT_OBJECT_0 {
		return fmt.Errorf("wait for elevated certificate process: status %d", waitResult)
	}
	var exitCode uint32
	if err := windows.GetExitCodeProcess(info.Process, &exitCode); err != nil {
		return err
	}
	if exitCode != 0 {
		return fmt.Errorf("elevated certificate process exited with code %d", exitCode)
	}
	return nil
}
