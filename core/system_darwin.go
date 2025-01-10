//go:build darwin

package core

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

func (s *SystemSetup) getNetworkServices() ([]string, error) {
	cmd := exec.Command("networksetup", "-listallnetworkservices")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute command: %v", err)
	}

	services := strings.Split(string(output), "\n")

	var validServices []string
	for _, service := range services {
		service = strings.TrimSpace(service)
		if service != "" && !strings.Contains(service, "*") && !strings.Contains(service, "Serial Port") {
			validServices = append(validServices, service)
		}
	}

	return validServices, nil
}

func (s *SystemSetup) setProxy() error {
	services, err := s.getNetworkServices()
	if err != nil {
		return err
	}
	if len(services) == 0 {
		return fmt.Errorf("find to Network failed")
	}

	is := false
	for _, serviceName := range services {
		if err := exec.Command("networksetup", "-setwebproxy", serviceName, "127.0.0.1", globalConfig.Port).Run(); err != nil {
			fmt.Println(err)
		} else {
			is = true
		}
		if err := exec.Command("networksetup", "-setsecurewebproxy", serviceName, "127.0.0.1", globalConfig.Port).Run(); err != nil {
			fmt.Println(err)
		} else {
			is = true
		}
	}

	if is {
		return nil
	}

	return fmt.Errorf("find to Network failed")
}

func (s *SystemSetup) unsetProxy() error {
	services, err := s.getNetworkServices()
	if err != nil {
		return err
	}
	if len(services) == 0 {
		return fmt.Errorf("find to Network failed")
	}

	is := false
	for _, serviceName := range services {
		if err := exec.Command("networksetup", "-setwebproxystate", serviceName, "off").Run(); err != nil {
			fmt.Println(err)
		} else {
			is = true
		}
		if err := exec.Command("networksetup", "-setsecurewebproxystate", serviceName, "off").Run(); err != nil {
			fmt.Println(err)
		} else {
			is = true
		}
	}

	if is {
		return nil
	}

	return fmt.Errorf("find to Network failed")
}

func (s *SystemSetup) installCert() (string, error) {
	_, err := s.initCert()
	if err != nil {
		return "", err
	}

	getPasswordCmd := exec.Command("osascript", "-e", `tell app "System Events" to display dialog "请输入你的电脑密码，用于安装证书文件:" default answer "" with hidden answer`, "-e", `text returned of result`)
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
