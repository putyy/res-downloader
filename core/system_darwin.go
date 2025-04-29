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

	is := false
	errs := ""
	for _, serviceName := range services {
		cmds := [][]string{
			{"networksetup", "-setwebproxy", serviceName, "127.0.0.1", globalConfig.Port},
			{"networksetup", "-setsecurewebproxy", serviceName, "127.0.0.1", globalConfig.Port},
		}
		for _, args := range cmds {
			if output, err := s.runCommand(args); err != nil {
				errs = errs + "\n output：" + string(output) + "err:" + err.Error()
				fmt.Println("setProxy:", output, " err:", err.Error())
			} else {
				is = true
			}
		}
	}

	if is {
		return nil
	}

	return fmt.Errorf("failed to set proxy for any active network service, errs: %s", errs)
}

func (s *SystemSetup) unsetProxy() error {
	services, err := s.getNetworkServices()
	if err != nil {
		return err
	}

	is := false
	errs := ""
	for _, serviceName := range services {
		cmds := [][]string{
			{"networksetup", "-setwebproxystate", serviceName, "off"},
			{"networksetup", "-setsecurewebproxystate", serviceName, "off"},
		}
		for _, args := range cmds {
			if output, err := s.runCommand(args); err != nil {
				errs = errs + "\n output：" + string(output) + "err:" + err.Error()
				fmt.Println("unsetProxy:", output, " err:", err.Error())
			} else {
				is = true
			}
		}
	}

	if is {
		return nil
	}

	return fmt.Errorf("failed to unset proxy for any active network service, errs: %s", errs)
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

	password := strings.TrimSpace(string(passwordOutput))
	s.SetPassword(password)

	cmd := exec.Command("sudo", "-S", "security", "add-trusted-cert", "-d", "-r", "trustRoot", "-k", "/Library/Keychains/System.keychain", s.CertFile)
	cmd.Stdin = bytes.NewReader([]byte(password + "\n"))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), err
	}
	return "", nil
}
