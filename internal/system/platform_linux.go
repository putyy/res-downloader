//go:build linux

package system

import (
	"bytes"
	"crypto/sha1"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func (s *Setup) removeLegacyCertificate(fingerprint string) (string, string, error) {
	paths := []string{
		"/usr/local/share/ca-certificates/res-downloader.crt",
		"/etc/ca-certificates/trust-source/anchors/res-downloader.crt",
		"/usr/share/ca-certificates/trust-source/res-downloader.crt",
		"/usr/share/ca-certificates/res-downloader/res-downloader.crt",
	}
	removed := false
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		block, _ := pem.Decode(raw)
		if block == nil {
			continue
		}
		certificate, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}
		digest := sha1.Sum(certificate.Raw)
		if !strings.EqualFold(hex.EncodeToString(digest[:]), fingerprint) {
			continue
		}
		if s.Password == "" {
			return "authorizationRequired", "administrator authorization is required to remove the legacy certificate", nil
		}
		if output, err := s.runCommand([]string{"rm", "-f", path}, true); err != nil {
			return "needsManualCleanup", string(output), err
		}
		removed = true
	}
	if !removed {
		return "notFound", "legacy desktop certificate was not installed; remove it manually from any phones that trusted it", nil
	}
	distro, err := s.getLinuxDistro()
	if err != nil {
		return "needsManualCleanup", "", fmt.Errorf("detect distro after legacy certificate removal: %w", err)
	}
	updateCmd := []string{"update-ca-certificates"}
	if distro == "arch" {
		updateCmd = []string{"update-ca-trust", "extract"}
	}
	if distro == "deepin" {
		entryPattern := "^" + s.app.AppName + "/" + s.app.AppName + "\\.crt$"
		if output, err := s.runCommand([]string{"sed", "-i", "\\|" + entryPattern + "|d", "/etc/ca-certificates.conf"}, true); err != nil {
			return "needsManualCleanup", string(output), fmt.Errorf("unregister legacy desktop certificate: %w", err)
		}
	}
	if output, err := s.runCommand(updateCmd, true); err != nil {
		return "needsManualCleanup", string(output), fmt.Errorf("refresh system trust after legacy certificate removal: %w", err)
	}
	for _, path := range paths {
		matches, err := certificateFileMatchesSHA1(path, fingerprint)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "needsManualCleanup", "", fmt.Errorf("verify legacy certificate file %s: %w", path, err)
		}
		if matches {
			return "needsManualCleanup", "", fmt.Errorf("legacy certificate file %s is still present after cleanup", path)
		}
	}
	return "removed", "legacy desktop certificate removed; certificates installed on phones must be removed manually", nil
}

func (s *Setup) isCertificateInstalled(fingerprint string) (bool, error) {
	paths := []string{
		"/usr/local/share/ca-certificates/res-downloader.crt",
		"/etc/ca-certificates/trust-source/anchors/res-downloader.crt",
		"/usr/share/ca-certificates/trust-source/res-downloader.crt",
		"/usr/share/ca-certificates/res-downloader/res-downloader.crt",
	}
	for _, path := range paths {
		matches, err := certificateFileMatchesSHA1(path, fingerprint)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return false, err
		}
		if matches {
			return true, nil
		}
	}
	return false, nil
}

func (s *Setup) uninstallCertificate(fingerprint string) (string, error) {
	paths := []string{
		"/usr/local/share/ca-certificates/res-downloader.crt",
		"/etc/ca-certificates/trust-source/anchors/res-downloader.crt",
		"/usr/share/ca-certificates/trust-source/res-downloader.crt",
		"/usr/share/ca-certificates/res-downloader/res-downloader.crt",
	}
	var output strings.Builder
	removed := false
	for _, path := range paths {
		matches, err := certificateFileMatchesSHA1(path, fingerprint)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return output.String(), err
		}
		if !matches {
			continue
		}
		result, err := s.runCommand([]string{"rm", "-f", path}, true)
		output.Write(result)
		if err != nil {
			return output.String(), fmt.Errorf("remove current device certificate: %w", err)
		}
		removed = true
	}
	if !removed {
		return output.String(), nil
	}

	distro, err := s.getLinuxDistro()
	if err != nil {
		return output.String(), fmt.Errorf("detect distro after certificate removal: %w", err)
	}
	updateCmd := []string{"update-ca-certificates"}
	if distro == "arch" {
		updateCmd = []string{"update-ca-trust", "extract"}
	}
	if distro == "deepin" {
		entryPattern := "^" + s.app.AppName + "/" + s.app.AppName + "\\.crt$"
		result, err := s.runCommand([]string{"sed", "-i", "\\|" + entryPattern + "|d", "/etc/ca-certificates.conf"}, true)
		output.Write(result)
		if err != nil {
			return output.String(), fmt.Errorf("unregister current device certificate: %w", err)
		}
	}
	result, err := s.runCommand(updateCmd, true)
	output.Write(result)
	if err != nil {
		return output.String(), fmt.Errorf("update system certificate trust after removal: %w", err)
	}
	return output.String(), nil
}

func (s *Setup) getLinuxDistro() (string, error) {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "ID=") {
			return strings.Trim(strings.TrimPrefix(line, "ID="), "\""), nil
		}
	}
	return "", fmt.Errorf("could not determine linux distribution")
}

func (s *Setup) runCommand(args []string, sudo bool) ([]byte, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("no command provided")
	}

	var cmd *exec.Cmd
	if s.Password != "" && sudo {
		cmd = exec.Command("sudo", append([]string{"-S"}, args...)...)
		cmd.Stdin = bytes.NewReader([]byte(s.Password + "\n"))
	} else {
		cmd = exec.Command(args[0], args[1:]...)
	}

	output, err := cmd.CombinedOutput()
	return output, err
}

func (s *Setup) setProxy() error {
	port := s.config.Snapshot().Port
	commands := [][]string{
		{"gsettings", "set", "org.gnome.system.proxy", "mode", "manual"},
		{"gsettings", "set", "org.gnome.system.proxy.http", "host", "127.0.0.1"},
		{"gsettings", "set", "org.gnome.system.proxy.http", "port", port},
		{"gsettings", "set", "org.gnome.system.proxy.https", "host", "127.0.0.1"},
		{"gsettings", "set", "org.gnome.system.proxy.https", "port", port},
	}

	isSuccess := false
	var errs strings.Builder

	for _, cmd := range commands {
		if output, err := s.runCommand(cmd, false); err != nil {
			errs.WriteString(fmt.Sprintf("cmd: %v\noutput: %s\nerr: %s\n", cmd, output, err))
		} else {
			isSuccess = true
		}
	}

	if isSuccess {
		return nil
	}

	return fmt.Errorf("failed to set proxy:\n%s", errs.String())
}

func (s *Setup) unsetProxy() error {
	cmd := []string{"gsettings", "set", "org.gnome.system.proxy", "mode", "none"}
	output, err := s.runCommand(cmd, false)
	if err != nil {
		return fmt.Errorf("failed to unset proxy: %s\noutput: %s", err.Error(), string(output))
	}
	return nil
}

func (s *Setup) installCert() (string, error) {
	_, err := s.initCert()
	if err != nil {
		return "", err
	}

	distro, err := s.getLinuxDistro()
	if err != nil {
		return "", fmt.Errorf("detect distro failed: %w", err)
	}

	certName := s.app.AppName + ".crt"
	var certPath string
	updateCmd := []string{"update-ca-certificates"}

	switch distro {
	case "deepin":
		certDir := "/usr/share/ca-certificates/" + s.app.AppName
		certPath = certDir + "/" + certName
		if output, err := s.runCommand([]string{"mkdir", "-p", certDir}, true); err != nil {
			return string(output), fmt.Errorf("create certificate directory: %w", err)
		}
	case "arch":
		certPath = "/etc/ca-certificates/trust-source/anchors/" + certName
		updateCmd = []string{"update-ca-trust", "extract"}
	default:
		certPath = "/usr/local/share/ca-certificates/" + certName
	}

	if output, err := s.runCommand([]string{"cp", "-f", s.CertFile, certPath}, true); err != nil {
		return string(output), fmt.Errorf("copy certificate: %w", err)
	}

	if distro == "deepin" {
		confPath := "/etc/ca-certificates.conf"
		confEntry := s.app.AppName + "/" + certName
		checkCmd := []string{"grep", "-qxF", confEntry, confPath}
		if _, err := s.runCommand(checkCmd, true); err != nil {
			echoCmd := []string{"bash", "-c", fmt.Sprintf("echo '%s' >> %s", confEntry, confPath)}
			if output, err := s.runCommand(echoCmd, true); err != nil {
				return string(output), fmt.Errorf("register certificate: %w", err)
			}
		}
	}

	if output, err := s.runCommand(updateCmd, true); err != nil {
		return string(output), fmt.Errorf("update system certificate trust: %w", err)
	}
	return "", nil
}
