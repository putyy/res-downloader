//go:build linux

package core

import (
	"os"
	"os/exec"
)

func (s *SystemSetup) setProxy() error {
	commands := [][]string{
		{"gsettings", "set", "org.gnome.system.proxy", "mode", "manual"},
		{"gsettings", "set", "org.gnome.system.proxy.http", "host", "127.0.0.1"},
		{"gsettings", "set", "org.gnome.system.proxy.http", "port", globalConfig.Port},
		{"gsettings", "set", "org.gnome.system.proxy.https", "host", "127.0.0.1"},
		{"gsettings", "set", "org.gnome.system.proxy.https", "port", globalConfig.Port},
	}

	for _, cmd := range commands {
		if err := exec.Command(cmd[0], cmd[1:]...).Run(); err != nil {
			return err
		}
	}
	return nil
}

func (s *SystemSetup) unsetProxy() error {
	cmd := []string{"gsettings", "set", "org.gnome.system.proxy", "mode", "none"}
	return exec.Command(cmd[0], cmd[1:]...).Run()
}

func (s *SystemSetup) installCert() (string, error) {
	certData, err := s.initCert()
	if err != nil {
		return "", err
	}
	destFile := "/usr/share/ca-certificates/trust-source/" + appOnce.AppName + ".crt"

	err = os.WriteFile(destFile, certData, 0644)
	if err != nil {
		return "", err
	}

	cmd := exec.Command("sudo", "update-ca-trust")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), err
	}
	return "", nil
}
