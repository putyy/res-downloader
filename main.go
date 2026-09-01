package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	application "res-downloader/internal/app"
	"res-downloader/internal/plugin"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
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
	recoveryZH := "如果安装失败，请手动安装与系统架构匹配的 WebView2 Runtime。"
	recoveryEN := "If installation fails, manually install the WebView2 Runtime matching the system architecture."
	if runtime.GOARCH == "amd64" {
		recoveryZH = "如果安装失败，请使用 fixed_webview2 安装包。"
		recoveryEN = "If installation fails, use the fixed_webview2 installer."
	}
	messages.Error = "错误 / Error"
	messages.MissingRequirements = "缺少运行环境 / Missing Requirements"
	messages.Webview2NotInstalled = "未检测到 WebView2 Runtime / WebView2 Runtime was not found"
	messages.PressOKToInstall = ""
	messages.InstallationRequired = fmt.Sprintf("运行此应用需要安装 WebView2 Runtime。点击确定后将尝试安装。%s\n\nWebView2 Runtime is required. Press OK to install it. %s", recoveryZH, recoveryEN)
	messages.UpdateRequired = "WebView2 Runtime 版本过低，需要更新后才能启动。\n\nWebView2 Runtime must be updated before the application can start."
	messages.FailedToInstall = fmt.Sprintf("WebView2 Runtime 安装失败，请检查网络。%s\n\nWebView2 Runtime installation failed. Check the network. %s", recoveryZH, recoveryEN)
	messages.ContactAdmin = fmt.Sprintf("当前系统无法安装 WebView2 Runtime，请联系系统管理员。%s\n\nWebView2 Runtime could not be installed. Contact your administrator. %s", recoveryZH, recoveryEN)
	messages.InvalidFixedWebview2 = "安装包内的 WebView2 Runtime 不完整、无权限访问或已被安全软件隔离，请重新安装并检查安全软件。\n\nThe bundled WebView2 Runtime is incomplete, inaccessible, or quarantined by security software."
	messages.WebView2ProcessCrash = "WebView2 渲染进程已崩溃，需要重新启动应用。如果问题反复出现，请打开日志目录并反馈。\n\nThe WebView2 process crashed. Restart the application and provide the application log if this continues."
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

	runtimePath := filepath.Join(filepath.Dir(executablePath), "WebView2Runtime")
	if _, err := os.Stat(filepath.Join(runtimePath, "msedgewebview2.exe")); err != nil {
		return ""
	}

	return runtimePath
}
