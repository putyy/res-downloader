package main

import (
	"context"
	"embed"
	"fmt"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"log"
	"os"
	"path/filepath"
	application "res-downloader/internal/app"
	"res-downloader/internal/plugin"
	"runtime"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var icon []byte

//go:embed wails.json
var wailsJson string

func main() {
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
	err := wails.Run(&options.App{
		Title:                    app.AppName,
		Width:                    1280,
		MinWidth:                 960,
		Height:                   800,
		MinHeight:                600,
		Frameless:                !isMac,
		Menu:                     appMenu,
		EnableDefaultContextMenu: true,
		AssetServer: &assetserver.Options{
			Assets:     assets,
			Middleware: appRuntime.HTTP.Middleware,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup: func(ctx context.Context) {
			logo := `
	 _ __    ___   ___            __| |   ___   __      __  _ __   | |   ___     __ _     __| |   ___   _ __
	| '__|  / _ \ / __|  _____   / _· |  / _ \  \ \ /\ / / | '_ \  | |  / _ \   / _· |   / _· |  / _ \ | ·__|
	| |    |  __/ \__ \ |_____| | (_| | | (_) |  \ V  V /  | | | | | | | (_) | | (_| |  | (_| | |  __/ | |
	|_|     \___| |___/          \__,_|  \___/    \_/\_/   |_| |_| |_|  \___/   \__ ,_|  \__,_|  \___| |_|`

			log.Println(logo)
			fmt.Println("version:", app.Version)
			app.Startup(ctx)
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
		},
		Linux: &linux.Options{
			ProgramName:         app.AppName,
			Icon:                icon,
			WebviewGpuPolicy:    linux.WebviewGpuPolicyOnDemand,
			WindowIsTranslucent: true,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

func bundledWebView2Path() string {
	if runtime.GOOS != "windows" {
		return ""
	}

	executablePath, err := os.Executable()
	if err != nil {
		return ""
	}

	runtimePath := filepath.Join(filepath.Dir(executablePath), "WebView2Runtime")
	if _, err := os.Stat(filepath.Join(runtimePath, "msedgewebview2.exe")); err != nil {
		return ""
	}

	return runtimePath
}
