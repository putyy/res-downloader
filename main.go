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

	application "res-downloader/internal/app"
	"res-downloader/internal/automation"
	"res-downloader/internal/config"
	"res-downloader/internal/plugin"
	"res-downloader/internal/system"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var icon []byte

//go:embed wails.json
var wailsJson string

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "cli" || os.Args[1] == "mcp") {
		system.PrepareCommandConsole()
		var metadata struct {
			Info struct {
				Version string `json:"productVersion"`
			} `json:"info"`
		}
		_ = json.Unmarshal([]byte(wailsJson), &metadata)
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
	appRuntime, runtimeErr := application.NewRuntime(assets, wailsJson)
	if runtimeErr != nil {
		log.Fatal(runtimeErr)
	}
	app := appRuntime.App
	bind := application.NewBind(appRuntime)
	isMac := runtime.GOOS == "darwin"
	// menu
	appMenu := menu.NewMenu()
	if isMac {
		appMenu.Append(menu.AppMenu())
		appMenu.Append(menu.EditMenu())
		appMenu.Append(menu.WindowMenu())
	}

	// Create application with options
	windowWidth, windowHeight := appRuntime.Config.WindowSize()
	err := wails.Run(&options.App{
		Title:                    app.AppName,
		Width:                    windowWidth,
		MinWidth:                 config.MinWindowWidth,
		Height:                   windowHeight,
		MinHeight:                config.MinWindowHeight,
		Frameless:                !isMac,
		Menu:                     appMenu,
		EnableDefaultContextMenu: true,
		AssetServer: &assetserver.Options{
			Assets:     assets,
			Middleware: appRuntime.HTTP.Middleware,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup: func(ctx context.Context) {
			wailsruntime.EventsOn(ctx, "window:resized", func(...interface{}) {
				app.SaveWindowSize(ctx)
			})
			logo := `
	 _ __    ___   ___            __| |   ___   __      __  _ __   | |   ___     __ _     __| |   ___   _ __
	| '__|  / _ \ / __|  _____   / _· |  / _ \  \ \ /\ / / | '_ \  | |  / _ \   / _· |   / _· |  / _ \ | ·__|
	| |    |  __/ \__ \ |_____| | (_| | | (_) |  \ V  V /  | | | | | | | (_) | | (_| |  | (_| | |  __/ | |
	|_|     \___| |___/          \__,_|  \___/    \_/\_/   |_| |_| |_|  \___/   \__ ,_|  \__,_|  \___| |_|`

			log.Println(logo)
			fmt.Println("version:", app.Version)
			app.Startup(ctx)
		},
		OnBeforeClose: func(ctx context.Context) bool {
			app.SaveWindowSize(ctx)
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

	if err != nil {
		appRuntime.Logger.Esg(err, "run application")
	}
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
