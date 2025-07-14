//go:build linux

package core

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func (s *SystemSetup) getLinuxDistro() (string, error) {
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

func (s *SystemSetup) runCommand(args []string, sudo bool) ([]byte, error) {
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

func (s *SystemSetup) setProxy() error {
	commands := [][]string{
		{"gsettings", "set", "org.gnome.system.proxy", "mode", "manual"},
		{"gsettings", "set", "org.gnome.system.proxy.http", "host", "127.0.0.1"},
		{"gsettings", "set", "org.gnome.system.proxy.http", "port", globalConfig.Port},
		{"gsettings", "set", "org.gnome.system.proxy.https", "host", "127.0.0.1"},
		{"gsettings", "set", "org.gnome.system.proxy.https", "port", globalConfig.Port},
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

func (s *SystemSetup) unsetProxy() error {
	cmd := []string{"gsettings", "set", "org.gnome.system.proxy", "mode", "none"}
	output, err := s.runCommand(cmd, false)
	if err != nil {
		return fmt.Errorf("failed to unset proxy: %s\noutput: %s", err.Error(), string(output))
	}
	return nil
}

func (s *SystemSetup) installCert() (string, error) {
	_, err := s.initCert()
	if err != nil {
		return "", err
	}

	distro, err := s.getLinuxDistro()
	if err != nil {
		return "", fmt.Errorf("detect distro failed: %w", err)
	}

	certName := appOnce.AppName + ".crt"
	var certPath string
	var updateCmd = []string{"update-ca-certificates"}

	switch distro {
	case "deepin":
		certDir := "/usr/share/ca-certificates/" + appOnce.AppName
		certPath = certDir + "/" + certName
		s.runCommand([]string{"mkdir", "-p", certDir}, true)
	case "arch":
		certPath = "/usr/share/ca-certificates/trust-source/" + certName
		updateCmd = []string{"update-ca-trust"}
	default:
		certPath = "/usr/local/share/ca-certificates/" + certName
	}

	var outs, errs strings.Builder
	isSuccess := false

	if output, err := s.runCommand([]string{"cp", "-f", s.CertFile, certPath}, true); err != nil {
		errs.WriteString(fmt.Sprintf("copy cert failed: %s\n%s\n", err.Error(), output))
	} else {
		isSuccess = true
		outs.Write(output)
	}

	if distro == "deepin" {
		confPath := "/etc/ca-certificates.conf"
		checkCmd := []string{"grep", "-qxF", certName, confPath}
		if _, err := s.runCommand(checkCmd, true); err != nil {
			echoCmd := []string{"bash", "-c", fmt.Sprintf("echo '%s' >> %s", appOnce.AppName+"/"+certName, confPath)}
			if output, err := s.runCommand(echoCmd, true); err != nil {
				errs.WriteString(fmt.Sprintf("append conf failed: %s\n%s\n", err.Error(), output))
			} else {
				isSuccess = true
				outs.Write(output)
			}
		}
	}

	if output, err := s.runCommand(updateCmd, true); err != nil {
		errs.WriteString(fmt.Sprintf("update failed: %s\n%s\n", err.Error(), output))
	} else {
		isSuccess = true
		outs.Write(output)
	}

	if isSuccess {
		return "", nil
	}

	return outs.String(), fmt.Errorf("certificate installation failed:\n%s", errs.String())
}
