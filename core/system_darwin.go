//go:build darwin

package core

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"os/exec"
	"strings"
)

func (s *SystemSetup) getActiveInterface() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, inter := range interfaces {
		if inter.Flags&net.FlagUp != 0 && inter.Flags&net.FlagLoopback == 0 {
			return inter.Name, nil
		}
	}
	return "", fmt.Errorf("no active network interface found")
}

func (s *SystemSetup) getNetworkServiceName(interfaceName string) (string, error) {
	cmd := exec.Command("networksetup", "-listallhardwareports")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}

	output := out.String()
	lines := strings.Split(output, "\n")
	var serviceName string
	for _, line := range lines {
		if strings.Contains(line, "Hardware Port:") {
			serviceName = strings.TrimSpace(strings.Split(line, ":")[1])
		}
		if strings.Contains(line, "Device: "+interfaceName) {
			return serviceName, nil
		}
	}
	return "", fmt.Errorf("no matching network service found for interface %s", interfaceName)
}

func (s *SystemSetup) setProxy() error {
	interfaceName, err := s.getActiveInterface()
	if err != nil {
		return err
	}

	serviceName, err := s.getNetworkServiceName(interfaceName)
	if err != nil {
		return err
	}
	commands := [][]string{
		{"networksetup", "-setwebproxy", serviceName, "127.0.0.1", globalConfig.Port},
		{"networksetup", "-setsecurewebproxy", serviceName, "127.0.0.1", globalConfig.Port},
	}

	for _, cmd := range commands {
		if err := exec.Command(cmd[0], cmd[1:]...).Run(); err != nil {
			return err
		}
	}
	return nil
}

func (s *SystemSetup) unsetProxy() error {
	interfaceName, err := s.getActiveInterface()
	if err != nil {
		return err
	}

	serviceName, err := s.getNetworkServiceName(interfaceName)
	if err != nil {
		return err
	}
	commands := [][]string{
		{"networksetup", "-setwebproxystate", serviceName, "off"},
		{"networksetup", "-setsecurewebproxystate", serviceName, "off"},
	}

	for _, cmd := range commands {
		if err := exec.Command(cmd[0], cmd[1:]...).Run(); err != nil {
			log.Println("UnsetProxy failed:", err)
			return err
		}
	}

	return nil
}

func (s *SystemSetup) installCert() (string, error) {
	_, err := s.initCert()
	if err != nil {
		return "", err
	}

	getPasswordCmd := exec.Command("osascript", "-e", `tell app "System Events" to display dialog "请输入密码，用于安装证书:" default answer "" with hidden answer`, "-e", `text returned of result`)
	passwordOutput, err := getPasswordCmd.Output()
	if err != nil {
		return string(passwordOutput), err
	}

	password := bytes.TrimSpace(passwordOutput)
	cmd := exec.Command("sudo", "-S", "security", "add-trusted-cert", "-d", "-r", "trustRoot", "-k", "/Library/Keychains/System.keychain", s.CertFile)

	cmd.Stdin = bytes.NewReader(append(password, '\n'))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), err
	}
	return "", nil
}
