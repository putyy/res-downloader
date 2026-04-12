package core

import (
	"context"
	"embed"
	"fmt"
	"github.com/vrischmann/userdir"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"time"
)

type App struct {
	ctx         context.Context
	assets      embed.FS
	AppName     string `json:"AppName"`
	Version     string `json:"Version"`
	Description string `json:"Description"`
	Copyright   string `json:"Copyright"`
	UserDir     string `json:"-"`
	PublicCrt   []byte `json:"-"`
	PrivateKey  []byte `json:"-"`
	IsProxy     bool   `json:"IsProxy"`
	IsReset     bool   `json:"-"`
}

var (
	appOnce        *App
	globalConfig   *Config
	globalLogger   *Logger
	resourceOnce   *Resource
	systemOnce     *SystemSetup
	proxyOnce      *Proxy
	httpServerOnce *HttpServer
	ruleOnce       *RuleSet
)

func initAppBase(wjs string) *App {
	if appOnce != nil {
		return appOnce
	}

	matches := regexp.MustCompile(`"productVersion":\s*"([\d.]+)"`).FindStringSubmatch(wjs)
	version := "1.0.1"
	if len(matches) > 0 {
		version = matches[1]
	}

	appOnce = &App{
		AppName:     "res-downloader",
		Version:     version,
		Description: "res-downloader是一款集网络资源嗅探 + 高速下载功能于一体的软件，高颜值、高性能和多样化，提供个人用户下载自己上传到各大平台的网络资源功能！",
		Copyright:   "Copyright © 2023~" + strconv.Itoa(time.Now().Year()),
		IsReset:     false,
	}
	appOnce.UserDir = filepath.Join(userdir.GetConfigHome(), appOnce.AppName)
	if err := os.MkdirAll(appOnce.UserDir, 0750); err != nil {
		fmt.Println("Mkdir UserDir err: ", err.Error())
	}

	return appOnce
}

func GetApp(assets embed.FS, wjs string) *App {
	app := initAppBase(wjs)
	app.assets = assets

	initLogger()
	initSystem()
	if err := app.syncLocalCertificate(true); err != nil {
		globalLogger.Esg(err, "init local certificate failed")
		panic(err)
	}

	initConfig()
	initProxy()
	initResource()
	initHttpServer()
	initRule()

	return app
}

func GetCliApp(wjs string) *App {
	app := initAppBase(wjs)
	initLogger()
	initSystem()
	if err := app.syncLocalCertificate(false); err != nil && !os.IsNotExist(err) {
		globalLogger.Esg(err, "load local certificate for cli failed")
	}
	return app
}

func (a *App) syncLocalCertificate(createIfMissing bool) error {
	var (
		cert []byte
		key  []byte
		err  error
	)

	if createIfMissing {
		cert, key, err = systemOnce.EnsureLocalCA()
	} else {
		cert, key, err = systemOnce.LoadLocalCA()
	}
	if err != nil {
		return err
	}

	a.PublicCrt = cert
	a.PrivateKey = key
	return nil
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	go httpServerOnce.run()
}

func (a *App) OnExit() {
	if err := a.UnsetSystemProxy(); err != nil {
		globalLogger.Esg(err, "unset system proxy on exit failed")
	}
	if err := a.RemoveTrustedCert(); err != nil {
		globalLogger.Esg(err, "remove trusted cert on exit failed")
	}
	globalLogger.Close()
	if appOnce.IsReset {
		err := a.ResetApp()
		fmt.Println("err:", err)
	}
}

func (a *App) installCert() (string, error) {
	out, err := systemOnce.installCert()
	if err != nil {
		globalLogger.Esg(err, out)
	}
	return out, err
}

func (a *App) RemoveTrustedCert() error {
	err := systemOnce.removeCert()
	if err != nil && os.IsNotExist(err) {
		return nil
	}
	return err
}

func (a *App) OpenSystemProxy() error {
	if a.IsProxy {
		return nil
	}
	err := systemOnce.setProxy()
	if err == nil {
		a.IsProxy = true
		return nil
	}
	return err
}

func (a *App) UnsetSystemProxy() error {
	if !a.IsProxy {
		return nil
	}
	err := systemOnce.unsetProxy()
	if err == nil {
		a.IsProxy = false
		return nil
	}
	return err
}

func (a *App) isInstall() bool {
	installed, err := systemOnce.isCertInstalled()
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		globalLogger.Esg(err, "check trusted cert failed")
		return false
	}
	return installed
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

	if err := systemOnce.unsetProxy(); err != nil {
		globalLogger.Esg(err, "unset proxy during reset failed")
	}
	if err := a.RemoveTrustedCert(); err != nil {
		globalLogger.Esg(err, "remove cert during reset failed")
	}

	_ = os.Remove(systemOnce.CacheFile)
	_ = os.Remove(filepath.Join(appOnce.UserDir, "config.json"))
	_ = os.Remove(systemOnce.CertFile)
	_ = os.Remove(systemOnce.KeyFile)

	cmd := exec.Command(exePath)
	cmd.Start()
	return nil
}

func CleanupSystemState(wjs string) error {
	app := GetCliApp(wjs)
	defer globalLogger.Close()

	var firstErr error
	if err := systemOnce.unsetProxy(); err != nil {
		globalLogger.Esg(err, "cleanup unset proxy failed")
		firstErr = err
	}
	if err := app.RemoveTrustedCert(); err != nil {
		globalLogger.Esg(err, "cleanup remove cert failed")
		if firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
