//go:build darwin

package core

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
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

func (s *SystemSetup) getNetworkServices() ([]string, error) {
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

func (s *SystemSetup) setProxy() error {
	services, err := s.getNetworkServices()
	if err != nil {
		return err
	}

	isSuccess := false
	var errs strings.Builder
	for _, serviceName := range services {
		commands := [][]string{
			{"networksetup", "-setwebproxy", serviceName, "127.0.0.1", globalConfig.Port},
			{"networksetup", "-setsecurewebproxy", serviceName, "127.0.0.1", globalConfig.Port},
			// macOS 15 上仅写 server/port 可能仍保持 Enabled: No；
			// 显式开启状态，避免应用 IsProxy 与真实系统代理不一致。
			{"networksetup", "-setwebproxystate", serviceName, "on"},
			{"networksetup", "-setsecurewebproxystate", serviceName, "on"},
		}
		serviceSuccess := true
		for _, cmd := range commands {
			if output, err := s.runCommand(cmd); err != nil {
				serviceSuccess = false
				errs.WriteString(fmt.Sprintf("cmd: %v\noutput: %s\nerr: %s\n", cmd, output, err))
			}
		}
		isSuccess = isSuccess || serviceSuccess
	}

	if isSuccess {
		return nil
	}

	return fmt.Errorf("failed to set proxy for any active network service, errs:%s", errs.String())
}

func (s *SystemSetup) unsetProxy() error {
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
		serviceSuccess := true
		for _, cmd := range commands {
			if output, err := s.runCommand(cmd); err != nil {
				serviceSuccess = false
				errs.WriteString(fmt.Sprintf("cmd: %v\noutput: %s\nerr: %s\n", cmd, output, err))
			}
		}
		isSuccess = isSuccess || serviceSuccess
	}

	if isSuccess {
		return nil
	}

	return fmt.Errorf("failed to unset proxy for any active network service, errs:%s", errs.String())
}

func (s *SystemSetup) installCert() (string, error) {
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
