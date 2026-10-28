package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	app "res-downloader/internal/app"
	"res-downloader/internal/automation"
	"res-downloader/internal/config"
	"res-downloader/internal/plugin"
	"res-downloader/internal/system"
	"res-downloader/internal/updates"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var icon []byte

//go:embed app.json
var appManifest string

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--wait-for-parent" {
		if len(os.Args) != 3 {
			log.Fatal("invalid restart arguments")
		}
		pid, err := strconv.Atoi(os.Args[2])
		if err != nil {
			log.Fatal("invalid restart process ID")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		err = system.WaitForParentExit(ctx, pid)
		cancel()
		if err != nil {
			log.Fatal(err)
		}
		os.Args = os.Args[:1]
	}
	if len(os.Args) == 3 && os.Args[1] == "--apply-update" {
		if err := updates.RunHelper(os.Args[2]); err != nil {
			log.Print(err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && (os.Args[1] == "cli" || os.Args[1] == "mcp") {
		system.PrepareCommandConsole()
		var metadata struct {
			Info struct {
				Version string `json:"productVersion"`
			} `json:"info"`
		}
		_ = json.Unmarshal([]byte(appManifest), &metadata)
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		code := automation.Run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr, metadata.Info.Version)
		cancel()
		os.Exit(code)
	}
	if len(os.Args) > 1 && os.Args[1] == "plugin" {
		if err := plugin.RunPluginCLI(os.Args[2:], os.Stdout); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, "plugin command failed:", err)
			os.Exit(1)
		}
		return
	}
	// Create an instance of the app structure
	appRuntime, runtimeErr := app.NewRuntime(assets, appManifest)
	if runtimeErr != nil {
		log.Fatal(runtimeErr)
	}
	state := appRuntime.App
	bind := app.NewBind(appRuntime)
	isMac := runtime.GOOS == "darwin"
	windowWidth, windowHeight := appRuntime.Config.WindowSize()

	desktopApp := application.New(application.Options{
		Name:        state.AppName,
		Description: state.Description,
		Icon:        icon,
		Services: []application.Service{
			application.NewService(bind),
		},
		Assets: application.AssetOptions{
			Handler:    application.AssetFileServerFS(assets),
			Middleware: appRuntime.Middleware,
		},
		// Wails acquires this lock while creating the application, before any
		// window or service exists, so a duplicate launch never opens databases
		// or mutates plugins.
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID:               "res-downloader",
			OnSecondInstanceLaunch: func(application.SecondInstanceData) { state.ActivateWindow() },
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		Windows: application.WindowsOptions{
			WebviewBrowserPath: bundledWebView2Path(),
		},
		Linux: application.LinuxOptions{
			ProgramName: state.AppName,
		},
		OnShutdown: func() {
			state.OnExit()
		},
	})

	window := desktopApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:             "main",
		Title:            state.AppName,
		Width:            windowWidth,
		Height:           windowHeight,
		MinWidth:         config.MinWindowWidth,
		MinHeight:        config.MinWindowHeight,
		Frameless:        !isMac,
		Hidden:           true,
		BackgroundColour: application.NewRGB(243, 245, 244),
		URL:              "/",
		Mac: application.MacWindow{
			Backdrop: application.MacBackdropNormal,
			TitleBar: application.MacTitleBarHiddenInset,
		},
		Linux: application.LinuxWindow{
			Icon:                icon,
			WindowIsTranslucent: true,
			WebviewGpuPolicy:    application.WebviewGpuPolicyOnDemand,
		},
	})

	appRuntime.AttachDesktop(desktopApp, window)

	// menu
	if isMac {
		appMenu := application.NewMenu()
		appMenu.AddRole(application.AppMenu)
		appMenu.AddRole(application.EditMenu)
		appMenu.AddRole(application.WindowMenu)
		desktopApp.Menu.Set(appMenu)
	}

	// Register the reveal listener before the frontend can emit it.
	desktopApp.Event.On("window:ready", func(*application.CustomEvent) {
		state.RevealStartupWindow()
	})
	desktopApp.Event.On("window:resized", func(*application.CustomEvent) {
		state.SaveWindowSize()
	})

	window.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) {
		state.SaveWindowSize()
		state.BeginShutdown()
	})

	window.OnWindowEvent(events.Common.WindowRuntimeReady, func(*application.WindowEvent) {
		// Runtime readiness runs after the frontend has loaded, which is the
		// earliest point where the API session token can be consumed.
		state.PrepareStartupWindow()
		go state.Startup(appRuntime.Context())
	})

	logo := `
	 _ __    ___   ___            __| |   ___   __      __  _ __   | |   ___     __ _     __| |   ___   _ __
	| '__|  / _ \ / __|  _____   / _· |  / _ \  \ \ /\ / / | '_ \  | |  / _ \   / _· |   / _· |  / _ \ | ·__|
	| |    |  __/ \__ \ |_____| | (_| | | (_) |  \ V  V /  | | | | | | | (_) | | (_| |  | (_| | |  __/ | |
	|_|     \___| |___/          \__,_|  \___/    \_/\_/   |_| |_| |_|  \___/   \__ ,_|  \__,_|  \___| |_|`

	log.Println(logo)
	fmt.Println("version:", state.Version)

	// Run the application. This blocks until the application has been exited.
	err := desktopApp.Run()

	if err != nil {
		appRuntime.Logger.Esg(err, "run application")
	}
	// Also covers failures before Wails reaches the shutdown hook.
	state.OnExit()
}

func bundledWebView2Path() string {
	if runtime.GOOS != "windows" {
		return ""
	}

	executablePath, err := os.Executable()
	if err != nil {
		return ""
	}
	return bundledWebView2PathForExecutable(executablePath)
}

func bundledWebView2PathForExecutable(executablePath string) string {
	// Standard installers opt into Evergreen even if a legacy fixed payload
	// cannot safely be removed. Fixed installers remove this marker again.
	if _, err := os.Stat(filepath.Join(filepath.Dir(executablePath), ".res-downloader-system-webview2")); err == nil {
		return ""
	}

	runtimePath := filepath.Join(filepath.Dir(executablePath), "WebView2Runtime")
	if _, err := os.Stat(filepath.Join(runtimePath, "msedgewebview2.exe")); err != nil {
		return ""
	}

	return runtimePath
}
