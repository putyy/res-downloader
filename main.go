package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	application "res-downloader/internal/app"
	"res-downloader/internal/automation"
	"res-downloader/internal/config"
	"res-downloader/internal/plugin"
	"res-downloader/internal/system"
	"res-downloader/internal/updates"

	wailsapp "github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var icon []byte

//go:embed build/config.yml
var buildConfig string

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
		version, err := application.ConfigVersion(buildConfig)
		if err != nil {
			log.Fatal(err)
		}
		if application.ReleaseVersion != "" {
			version = application.ReleaseVersion
		}
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		code := automation.Run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr, version)
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
	// Wails acquires its single-instance lock in New. Construct business state afterwards.
	desktopVersion, err := application.ConfigVersion(buildConfig)
	if err != nil {
		log.Fatal(err)
	}
	if application.ReleaseVersion != "" {
		desktopVersion = application.ReleaseVersion
	}
	var appRuntime *application.Runtime
	desktop := wailsapp.New(wailsapp.Options{
		Name:        "res-downloader",
		Description: fmt.Sprintf("res-downloader %s\nA resource downloader with network capture and fast downloads", desktopVersion),
		Icon:        icon,
		Assets: wailsapp.AssetOptions{
			Handler: wailsapp.BundledAssetFileServer(assets),
			Middleware: func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if appRuntime == nil {
						http.Error(w, "Application is starting", http.StatusServiceUnavailable)
						return
					}
					appRuntime.Middleware(next).ServeHTTP(w, r)
				})
			},
		},
		SingleInstance: &wailsapp.SingleInstanceOptions{
			UniqueID: "res-downloader",
			OnSecondInstanceLaunch: func(wailsapp.SecondInstanceData) {
				if appRuntime != nil {
					appRuntime.App.ActivateWindow()
				}
			},
		},
		OnShutdown: func() {
			if appRuntime != nil {
				appRuntime.App.SaveWindowSize()
				appRuntime.App.OnExit()
			}
		},
		Mac:     wailsapp.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
		Windows: wailsapp.WindowsOptions{WebviewBrowserPath: bundledWebView2Path()},
		Linux:   wailsapp.LinuxOptions{ProgramName: "res-downloader"},
	})
	var runtimeErr error
	appRuntime, runtimeErr = application.NewRuntime(assets, buildConfig)
	if runtimeErr != nil {
		log.Fatal(runtimeErr)
	}
	app := appRuntime.App
	bind := application.NewBind(appRuntime)
	isMac := runtime.GOOS == "darwin"
	desktop.RegisterService(wailsapp.NewService(bind))
	if isMac {
		appMenu := desktop.Menu.New()
		appMenu.AddRole(wailsapp.AppMenu).AddRole(wailsapp.EditMenu).AddRole(wailsapp.WindowMenu)
		desktop.Menu.SetApplicationMenu(appMenu)
	}
	windowWidth, windowHeight := appRuntime.Config.WindowSize()
	window := desktop.Window.NewWithOptions(wailsapp.WebviewWindowOptions{
		Title: app.AppName, Width: windowWidth, Height: windowHeight,
		MinWidth: config.MinWindowWidth, MinHeight: config.MinWindowHeight,
		Frameless: !isMac, Hidden: true,
		BackgroundColour: wailsapp.RGBA{Red: 243, Green: 245, Blue: 244, Alpha: 255},
		Mac:              wailsapp.MacWindow{TitleBar: wailsapp.MacTitleBarHiddenInset},
	})
	app.AttachDesktop(desktop, window)
	window.OnWindowEvent(events.Common.WindowDidResize, func(*wailsapp.WindowEvent) { app.ScheduleWindowSizeSave() })
	window.OnWindowEvent(events.Common.WindowClosing, func(*wailsapp.WindowEvent) {
		app.SaveWindowSize()
		app.BeginShutdown()
	})
	desktop.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*wailsapp.ApplicationEvent) {
		app.PrepareStartupWindow()
		go app.Startup(desktop.Context())
	})
	log.Printf("version: %s", app.Version)
	err = desktop.Run()

	if err != nil {
		appRuntime.Logger.Esg(err, "run application")
	}
	// Also covers failures before Wails reaches OnShutdown.
	app.OnExit()
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
