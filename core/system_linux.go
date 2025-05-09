//go:build linux

package core

import (
	"bytes"
	"fmt"
	"os/exec"
)

func (s *SystemSetup) runCommand(args []string) ([]byte, error) {
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

func (s *SystemSetup) setProxy() error {
	commands := [][]string{
		{"gsettings", "set", "org.gnome.system.proxy", "mode", "manual"},
		{"gsettings", "set", "org.gnome.system.proxy.http", "host", "127.0.0.1"},
		{"gsettings", "set", "org.gnome.system.proxy.http", "port", globalConfig.Port},
		{"gsettings", "set", "org.gnome.system.proxy.https", "host", "127.0.0.1"},
		{"gsettings", "set", "org.gnome.system.proxy.https", "port", globalConfig.Port},
	}
	is := false
	errs := ""
	for _, cmd := range commands {
		if output, err := s.runCommand(cmd); err != nil {
			errs = errs + "output:" + string(output) + " err:" + err.Error() + "\n"
			fmt.Println(err)
		} else {
			is = true
		}
	}
	if is {
		return nil
	}

	return fmt.Errorf("failed to set proxy for any active network service, errs:%s", errs)
}

func (s *SystemSetup) unsetProxy() error {
	cmd := []string{"gsettings", "set", "org.gnome.system.proxy", "mode", "none"}
	output, err := s.runCommand(cmd)
	return fmt.Errorf("failed to unset proxy for any active network service, errs output:" + string(output) + " err:" + err.Error())
}

func (s *SystemSetup) installCert() (string, error) {
	_, err := s.initCert()
	if err != nil {
		return "", err
	}

	actions := [][]string{
		{"cp", "-f", s.CertFile, "/usr/local/share/ca-certificates/" + appOnce.AppName + ".crt"},
		{"update-ca-certificates"},
		{"cp", "-f", s.CertFile, "/usr/share/ca-certificates/trust-source/anchors/" + appOnce.AppName + ".crt"},
		{"update-ca-trust"},
		{"trust", "extract-compat"},
		{"cp", "-f", s.CertFile, "/etc/pki/ca-trust/source/anchors/" + appOnce.AppName + ".crt"},
		{"update-ca-trust"},
		{"cp", "-f", s.CertFile, "/etc/ssl/ca-certificates/" + appOnce.AppName + ".crt"},
		{"update-ca-certificates"},
	}

	is := false
	outs := ""
	errs := ""
	for _, action := range actions {
		if output, err1 := s.runCommand(action); err1 != nil {
			outs += string(output) + "\n"
			errs += err1.Error() + "\n"
			fmt.Printf("Failed to execute %v: %v\n", action, err1)
			continue
		}

		is = true
	}

	if is {
		return "", nil
	}

	return outs, fmt.Errorf("Certificate installation failed, errs:%s", errs)
}
