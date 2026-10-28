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

<<<<<<< Updated upstream
	application "res-downloader/internal/app"
=======
	app "res-downloader/internal/app"
>>>>>>> Stashed changes
	"res-downloader/internal/automation"
	"res-downloader/internal/config"
	"res-downloader/internal/plugin"
	"res-downloader/internal/system"
	"res-downloader/internal/updates"

<<<<<<< Updated upstream
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
=======
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
>>>>>>> Stashed changes
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
<<<<<<< Updated upstream
		_ = json.Unmarshal([]byte(wailsJson), &metadata)
=======
		_ = json.Unmarshal([]byte(appManifest), &metadata)
>>>>>>> Stashed changes
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
<<<<<<< Updated upstream
	appRuntime, runtimeErr := application.NewRuntime(assets, wailsJson)
	if runtimeErr != nil {
		log.Fatal(runtimeErr)
	}
	app := appRuntime.App
	bind := application.NewBind(appRuntime)
=======
	appRuntime, runtimeErr := app.NewRuntime(assets, appManifest)
	if runtimeErr != nil {
		log.Fatal(runtimeErr)
	}
	state := appRuntime.App
	bind := app.NewBind(appRuntime)
>>>>>>> Stashed changes
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

<<<<<<< Updated upstream
	// Create application with options
	windowWidth, windowHeight := appRuntime.Config.WindowSize()
	err := wails.Run(&options.App{
		Title:                    app.AppName,
		Width:                    windowWidth,
		MinWidth:                 config.MinWindowWidth,
		Height:                   windowHeight,
		MinHeight:                config.MinWindowHeight,
		Frameless:                !isMac,
		StartHidden:              true,
		Menu:                     appMenu,
		EnableDefaultContextMenu: true,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               "res-downloader",
			OnSecondInstanceLaunch: func(options.SecondInstanceData) { app.ActivateWindow() },
		},
		AssetServer: &assetserver.Options{
			Assets:     assets,
			Middleware: appRuntime.Middleware,
		},
		BackgroundColour: &options.RGBA{R: 243, G: 245, B: 244, A: 255},
		OnStartup: func(ctx context.Context) {
			app.PrepareStartupWindow(ctx)
			wailsruntime.EventsOn(ctx, "window:resized", func(...interface{}) {
				app.SaveWindowSize(ctx)
			})
			logo := `
=======
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
>>>>>>> Stashed changes
	 _ __    ___   ___            __| |   ___   __      __  _ __   | |   ___     __ _     __| |   ___   _ __
	| '__|  / _ \ / __|  _____   / _· |  / _ \  \ \ /\ / / | '_ \  | |  / _ \   / _· |   / _· |  / _ \ | ·__|
	| |    |  __/ \__ \ |_____| | (_| | | (_) |  \ V  V /  | | | | | | | (_) | | (_| |  | (_| | |  __/ | |
	|_|     \___| |___/          \__,_|  \___/    \_/\_/   |_| |_| |_|  \___/   \__ ,_|  \__,_|  \___| |_|`

<<<<<<< Updated upstream
			log.Println(logo)
			fmt.Println("version:", app.Version)
		},
		OnDomReady: func(ctx context.Context) {
			// Wails checks the Linux instance lock after scheduling OnStartup.
			// DOM readiness runs after frontend setup on all supported platforms.
			app.PrepareStartupWindow(ctx)
			go app.Startup(ctx)
		},
		OnBeforeClose: func(ctx context.Context) bool {
			app.SaveWindowSize(ctx)
			app.BeginShutdown()
			return false
		},
		OnShutdown: func(ctx context.Context) {
			app.OnExit()
		},
		Bind: []interface{}{
			bind,
		},
		Mac: &mac.Options{
			TitleBar: mac.TitleBarHiddenInset(),
			About: &mac.AboutInfo{
				Title:   fmt.Sprintf("%s %s", app.AppName, app.Version),
				Message: app.Description + app.Copyright,
				Icon:    icon,
			},
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
		Windows: &windows.Options{
			WebviewIsTransparent:              false,
			WindowIsTranslucent:               false,
			DisableFramelessWindowDecorations: false,
			WebviewBrowserPath:                bundledWebView2Path(),
			Messages:                          windowsRuntimeMessages(),
		},
		Linux: &linux.Options{
			ProgramName:         app.AppName,
			Icon:                icon,
			WebviewGpuPolicy:    linux.WebviewGpuPolicyOnDemand,
			WindowIsTranslucent: true,
		},
	})
=======
	log.Println(logo)
	fmt.Println("version:", state.Version)

	// Run the application. This blocks until the application has been exited.
	err := desktopApp.Run()
>>>>>>> Stashed changes

	if err != nil {
		appRuntime.Logger.Esg(err, "run application")
	}
<<<<<<< Updated upstream
	// Also covers failures before Wails reaches OnShutdown.
	app.OnExit()
}

func windowsRuntimeMessages() *windows.Messages {
	messages := windows.DefaultMessages()
	recovery := "If installation fails, manually install the WebView2 Runtime matching the system architecture."
	if runtime.GOARCH == "amd64" {
		recovery = "If installation fails, use the fixed_webview2 installer."
	}
	messages.Error = "Error"
	messages.MissingRequirements = "Missing Requirements"
	messages.Webview2NotInstalled = "WebView2 Runtime was not found"
	messages.PressOKToInstall = ""
	messages.InstallationRequired = fmt.Sprintf("WebView2 Runtime is required. Press OK to install it. %s", recovery)
	messages.UpdateRequired = "WebView2 Runtime must be updated before the application can start."
	messages.FailedToInstall = fmt.Sprintf("WebView2 Runtime installation failed. Check your network connection. %s", recovery)
	messages.ContactAdmin = fmt.Sprintf("WebView2 Runtime could not be installed. Contact your administrator. %s", recovery)
	messages.InvalidFixedWebview2 = "The bundled WebView2 Runtime is incomplete, inaccessible, or quarantined by security software. Reinstall the application and check your security software."
	messages.WebView2ProcessCrash = "The WebView2 process crashed. Restart the application. If the problem persists, open the log directory and include the application log in your report."
	return messages
=======
	// Also covers failures before Wails reaches the shutdown hook.
	state.OnExit()
>>>>>>> Stashed changes
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
