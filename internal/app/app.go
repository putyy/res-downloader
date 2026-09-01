package app

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	desktopsystem "res-downloader/internal/system"
	"strconv"
	"strings"
	"time"

	"res-downloader/internal/events"

	"github.com/vrischmann/userdir"
)

type App struct {
	ctx              context.Context
	assets           embed.FS
	runtime          *Runtime
	events           *events.Emitter
	AppName          string `json:"AppName"`
	Version          string `json:"Version"`
	Description      string `json:"Description"`
	Copyright        string `json:"Copyright"`
	UserDir          string `json:"-"`
	PublicCrt        []byte `json:"-"`
	PrivateKey       []byte `json:"-"`
	CertificateError string `json:"CertificateError,omitempty"`
	IsProxy          bool   `json:"IsProxy"`
	IsReset          bool   `json:"-"`
}

func newApp(assets embed.FS, wjs string) (*App, error) {
	matches := regexp.MustCompile(`"productVersion":\s*"([\d.]+)"`).FindStringSubmatch(wjs)
	version := "1.0.1"
	if len(matches) > 0 {
		version = matches[1]
	}

	app := &App{
		assets: assets, AppName: "res-downloader", Version: version,
		Description: "res-downloader是一款集网络资源嗅探 + 高速下载功能于一体的软件，高颜值、高性能和多样化，提供个人用户下载自己上传到各大平台的网络资源功能！",
		Copyright:   "Copyright © 2023~" + strconv.Itoa(time.Now().Year()),
	}
	app.UserDir = filepath.Join(userdir.GetConfigHome(), app.AppName)
	if err := os.MkdirAll(app.UserDir, 0750); err != nil {
		return nil, fmt.Errorf("create user directory: %w", err)
	}
	if err := removeLegacyLocalFiles(app.UserDir); err != nil {
		return nil, fmt.Errorf("remove legacy local files: %w", err)
	}
	return app, nil
}

func removeLegacyLocalFiles(userDir string) error {
	var cleanupErrors []error
	for _, name := range []string{"pass.cache", "install.lock", "cert.crt"} {
		if err := os.Remove(filepath.Join(userDir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("remove %s: %w", name, err))
		}
	}
	return errors.Join(cleanupErrors...)
}

func (a *App) emitEvent(eventType string, data interface{}) {
	if a == nil || a.ctx == nil || a.events == nil {
		return
	}
	_ = a.events.Emit(eventType, data)
}

func (a *App) Startup(ctx context.Context) {
	if err := a.runtime.Start(ctx); err != nil {
		a.runtime.Logger.Esg(err, "start application runtime")
		a.dialogErr(err.Error())
	}
}

func (a *App) OnExit() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := a.runtime.Close(ctx); err != nil {
		fmt.Println("err:", err)
	}
}

func (a *App) installCertWithPassword(password string) (string, error) {
	return a.installCertWith(a.runtime.System.WithPassword(password))
}

func (a *App) installCertWith(setup *SystemSetup) (string, error) {
	if a.CertificateError != "" {
		return "", fmt.Errorf("certificate authority is unavailable: %s", a.CertificateError)
	}
	out, err := setup.InstallCertificate()
	if err != nil {
		a.runtime.Logger.Esg(err, out)
		return out, err
	}
	installed, err := setup.IsCertificateInstalled(desktopsystem.CurrentCertificateSHA1(a.PublicCrt))
	if err != nil {
		return out, fmt.Errorf("verify installed certificate: %w", err)
	}
	if !installed {
		return out, fmt.Errorf("the current device certificate was not found in the system trust store after installation")
	}
	return out, nil
}

func (a *App) uninstallCertWithPassword(password string) (string, error) {
	if a.CertificateError != "" {
		return "", fmt.Errorf("certificate authority is unavailable: %s", a.CertificateError)
	}
	setup := a.runtime.System.WithPassword(password)
	if a.IsProxy {
		if err := setup.UnsetProxy(); err != nil {
			return "", fmt.Errorf("disable system proxy before certificate removal: %w", err)
		}
		a.IsProxy = false
	}
	out, err := setup.UninstallCertificate(desktopsystem.CurrentCertificateSHA1(a.PublicCrt))
	if err != nil {
		a.runtime.Logger.Esg(err, out)
		return out, err
	}
	installed, err := setup.IsCertificateInstalled(desktopsystem.CurrentCertificateSHA1(a.PublicCrt))
	if err != nil {
		return out, fmt.Errorf("verify certificate removal: %w", err)
	}
	if installed {
		return out, fmt.Errorf("the current device certificate is still present in the system trust store")
	}
	return out, nil
}

func (a *App) OpenSystemProxy(password string) error {
	if a.IsProxy {
		return nil
	}
	if err := a.runtime.System.WithPassword(password).SetProxy(); err != nil {
		return err
	}
	a.IsProxy = true
	return nil
}

func (a *App) UnsetSystemProxy(password string) error {
	if !a.IsProxy {
		return nil
	}
	if err := a.runtime.System.WithPassword(password).UnsetProxy(); err != nil {
		return err
	}
	a.IsProxy = false
	return nil
}

func (a *App) PrepareReset(password string) error {
	if a == nil || a.runtime == nil || a.runtime.System == nil {
		return errors.New("certificate cleanup is unavailable")
	}
	setup := a.runtime.System.WithPassword(password)
	if a.IsProxy {
		if err := setup.UnsetProxy(); err != nil {
			return fmt.Errorf("disable system proxy before reset: %w", err)
		}
		a.IsProxy = false
	}
	migration := desktopsystem.RetryLegacyCertificateMigration(a.runtime.System, password)
	if migration.Status != "removed" && migration.Status != "notFound" {
		message := strings.TrimSpace(migration.Message)
		if message == "" {
			message = migration.Status
		}
		return fmt.Errorf("remove legacy shared certificate before reset: %s", message)
	}

	certificate := a.PublicCrt
	if len(certificate) == 0 {
		stored, err := os.ReadFile(a.runtime.System.CertFile)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("read current device certificate before reset: %w", err)
		}
		certificate = stored
	}
	if len(certificate) > 0 {
		fingerprint := desktopsystem.CurrentCertificateSHA1(certificate)
		if fingerprint == "" {
			return errors.New("the current device certificate is invalid; remove it manually before resetting the application")
		}
		installed, err := a.runtime.System.IsCertificateInstalled(fingerprint)
		if err != nil {
			return fmt.Errorf("check current device certificate before reset: %w", err)
		}
		if installed {
			output, err := setup.UninstallCertificate(fingerprint)
			if err != nil {
				if detail := strings.TrimSpace(output); detail != "" {
					return fmt.Errorf("remove current device certificate before reset: %w: %s", err, detail)
				}
				return fmt.Errorf("remove current device certificate before reset: %w", err)
			}
			remaining, err := a.runtime.System.IsCertificateInstalled(fingerprint)
			if err != nil {
				return fmt.Errorf("verify current device certificate cleanup: %w", err)
			}
			if remaining {
				return errors.New("the current device certificate is still present after cleanup")
			}
		}
	}

	a.IsReset = true
	return nil
}

func (a *App) ResetApp() error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	exePath, err = filepath.Abs(exePath)
	if err != nil {
		return err
	}
	var cleanupErrors []error
	for _, name := range []string{
		"install.lock", "pass.cache", "config.json", "cert.crt",
		"mitm-ca.crt", "mitm-ca.key", "resources.db", "tasks.db",
		"certificate-migration-v1.json", "plugin-store-cache.json",
	} {
		if err := os.Remove(filepath.Join(a.UserDir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("remove %s: %w", name, err))
		}
	}
	for _, name := range []string{"capture-cache"} {
		if err := os.RemoveAll(filepath.Join(a.UserDir, name)); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("remove %s: %w", name, err))
		}
	}
	if err := errors.Join(cleanupErrors...); err != nil {
		return err
	}
	cmd := exec.Command(exePath)
	return cmd.Start()
}
