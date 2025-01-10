//go:build linux

package core

import (
	"fmt"
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
	is := false
	for _, cmd := range commands {
		if err := exec.Command(cmd[0], cmd[1:]...).Run(); err != nil {
			fmt.Println(err)
		} else {
			is = true
		}
	}
	if is {
		return nil
	}
	return fmt.Errorf("Failed to activate proxy")
}

func (s *SystemSetup) unsetProxy() error {
	cmd := []string{"gsettings", "set", "org.gnome.system.proxy", "mode", "none"}
	return exec.Command(cmd[0], cmd[1:]...).Run()
}

func (s *SystemSetup) installCert() (string, error) {
	_, err := s.initCert()
	if err != nil {
		return "", err
	}

	actions := [][]string{
		{"/usr/local/share/ca-certificates/", "update-ca-certificates"},
		{"/usr/share/ca-certificates/trust-source/anchors/", "update-ca-trust"},
		{"/usr/share/ca-certificates/trust-source/anchors/", "trust extract-compat"},
		{"/etc/pki/ca-trust/source/anchors/", "update-ca-trust"},
		{"/etc/ssl/ca-certificates/", "update-ca-certificates"},
	}

	is := false

	for _, action := range actions {
		dir := action[0]
		if err := exec.Command("sudo", "cp", "-f", s.CertFile, dir+appOnce.AppName+".crt").Run(); err != nil {
			fmt.Printf("Failed to copy to %s: %v\n", dir, err)
			continue
		}

		cmd := action[1]
		if err := exec.Command("sudo", cmd).Run(); err != nil {
			fmt.Printf("Failed to refresh certificates using %s: %v\n", cmd, err)
			continue
		}

		is = true
	}

	if !is {
		return "", fmt.Errorf("Certificate installation failed")
	}

	return "", nil
}
