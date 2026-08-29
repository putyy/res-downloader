//go:build darwin

package system

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

func (s *Setup) runCommand(args []string) ([]byte, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("no command provided")
	}

	var cmd *exec.Cmd
	if s.Password != "" {
		cmd = exec.Command("sudo", append([]string{"-S"}, args...)...)
		cmd.Stdin = bytes.NewReader([]byte(s.Password + "\n"))
	} else {
		cmd = exec.Command(args[0], args[1:]...)
	}

	output, err := cmd.CombinedOutput()
	return output, err
}

func (s *Setup) getNetworkServices() ([]string, error) {
	output, err := s.runCommand([]string{"networksetup", "-listallnetworkservices"})
	if err != nil {
		return nil, fmt.Errorf("failed to execute command: %v", err)
	}

	services := strings.Split(string(output), "\n")
	var activeServices []string
	for _, service := range services {
		service = strings.TrimSpace(service)
		if service == "" || strings.Contains(service, "*") || strings.Contains(service, "Serial Port") {
			continue
		}

		infoOutput, err := s.runCommand([]string{"networksetup", "-getinfo", service})
		if err != nil {
			fmt.Printf("failed to get info for service %s: %v\n", service, err)
			continue
		}

		if strings.Contains(string(infoOutput), "IP address:") {
			activeServices = append(activeServices, service)
		}
	}

	if len(activeServices) == 0 {
		return nil, fmt.Errorf("no active network services found")
	}

	return activeServices, nil
}

func (s *Setup) setProxy() error {
	port := s.config.Snapshot().Port
	services, err := s.getNetworkServices()
	if err != nil {
		return err
	}

	isSuccess := false
	var errs strings.Builder
	for _, serviceName := range services {
		commands := [][]string{
			{"networksetup", "-setwebproxy", serviceName, "127.0.0.1", port},
			{"networksetup", "-setsecurewebproxy", serviceName, "127.0.0.1", port},
		}
		for _, cmd := range commands {
			if output, err := s.runCommand(cmd); err != nil {
				errs.WriteString(fmt.Sprintf("cmd: %v\noutput: %s\nerr: %s\n", cmd, output, err))
			} else {
				isSuccess = true
			}
		}
	}

	if isSuccess {
		return nil
	}

	return fmt.Errorf("failed to set proxy for any active network service, errs:%s", errs.String())
}

func (s *Setup) unsetProxy() error {
	services, err := s.getNetworkServices()
	if err != nil {
		return err
	}

	isSuccess := false
	var errs strings.Builder
	for _, serviceName := range services {
		commands := [][]string{
			{"networksetup", "-setwebproxystate", serviceName, "off"},
			{"networksetup", "-setsecurewebproxystate", serviceName, "off"},
		}
		for _, cmd := range commands {
			if output, err := s.runCommand(cmd); err != nil {
				errs.WriteString(fmt.Sprintf("cmd: %v\noutput: %s\nerr: %s\n", cmd, output, err))
			} else {
				isSuccess = true
			}
		}
	}

	if isSuccess {
		return nil
	}

	return fmt.Errorf("failed to unset proxy for any active network service, errs:%s", errs.String())
}

func (s *Setup) installCert() (string, error) {
	_, err := s.initCert()
	if err != nil {
		return "", err
	}
	output, err := s.runCommand([]string{"security", "add-trusted-cert", "-d", "-r", "trustRoot", "-k", "/Library/Keychains/System.keychain", s.CertFile})
	if err != nil {
		return string(output), err
	}
	return "", nil
}

func (s *Setup) isCertificateInstalled(fingerprint string) (bool, error) {
	keychain := "/Library/Keychains/System.keychain"
	output, err := exec.Command("security", "find-certificate", "-a", "-Z", keychain).CombinedOutput()
	if err != nil {
		return false, commandOutputError("inspect system keychain", output, err)
	}
	return certificateListContainsFingerprint(output, fingerprint), nil
}

func (s *Setup) uninstallCertificate(fingerprint string) (string, error) {
	installed, err := s.isCertificateInstalled(fingerprint)
	if err != nil || !installed {
		return "", err
	}
	if s.Password == "" {
		return "", fmt.Errorf("administrator authorization is required to remove the current device certificate")
	}
	keychain := "/Library/Keychains/System.keychain"
	output, err := s.runCommand([]string{"security", "delete-certificate", "-Z", fingerprint, keychain})
	if err != nil {
		return string(output), commandOutputError("remove current device certificate", output, err)
	}
	return string(output), nil
}

func (s *Setup) removeLegacyCertificate(fingerprint string) (string, string, error) {
	keychain := "/Library/Keychains/System.keychain"
	output, err := s.runCommand([]string{"security", "find-certificate", "-a", "-Z", keychain})
	if err != nil {
		return "failed", "", commandOutputError("inspect system keychain", output, err)
	}
	if !certificateListContainsFingerprint(output, fingerprint) {
		return "notFound", "legacy desktop certificate was not installed; remove it manually from any phones that trusted it", nil
	}

	// The system keychain is writable only with administrator privileges. At
	// startup there may be no cached password yet, so do not run a command that
	// can only fail with errSecWrPerm (-61, surfaced by the process as exit 195).
	if s.Password == "" {
		return "authorizationRequired", "administrator authorization is required to remove the legacy certificate", nil
	}

	// Do not pass -t here. It removes the invoking user's trust settings in
	// addition to the certificate, which is both unnecessary for a certificate
	// stored in System.keychain and can fail when security is invoked via sudo.
	deleted, deleteErr := s.runCommand([]string{"security", "delete-certificate", "-Z", fingerprint, keychain})

	// Always verify the result. This also makes the migration idempotent if the
	// command removed the item but returned a secondary keychain error.
	remaining, inspectErr := s.runCommand([]string{"security", "find-certificate", "-a", "-Z", keychain})
	if inspectErr == nil && !certificateListContainsFingerprint(remaining, fingerprint) {
		return "removed", "legacy desktop certificate removed; certificates installed on phones must be removed manually", nil
	}
	if deleteErr != nil {
		return "needsManualCleanup", "", commandOutputError("delete legacy certificate", deleted, deleteErr)
	}
	if inspectErr != nil {
		return "needsManualCleanup", "", commandOutputError("verify legacy certificate cleanup", remaining, inspectErr)
	}
	return "needsManualCleanup", "", fmt.Errorf("legacy certificate is still present after the delete command")
}

func certificateListContainsFingerprint(output []byte, fingerprint string) bool {
	normalize := func(value string) string {
		value = strings.ToUpper(value)
		return strings.NewReplacer(":", "", " ", "", "\r", "", "\n", "", "\t", "").Replace(value)
	}
	return strings.Contains(normalize(string(output)), normalize(fingerprint))
}

func commandOutputError(action string, output []byte, err error) error {
	detail := strings.TrimSpace(string(output))
	if detail == "" {
		return fmt.Errorf("%s: %w", action, err)
	}
	return fmt.Errorf("%s: %w: %s", action, err, detail)
}
