package app

import (
	"errors"

	"res-downloader/internal/httpapi"

	"github.com/wailsapp/wails/v3/pkg/application"
)

var errDesktopUnavailable = errors.New("native desktop integration is unavailable")

// quit asks the native application to terminate. Shutdown cleanup runs from the
// application shutdown hook.
func (a *App) quit() {
	if a == nil || a.desktop == nil {
		return
	}
	a.desktop.Quit()
}

func (a *App) errorDialog(message string) {
	if a == nil || a.desktop == nil {
		return
	}
	dialog := a.desktop.Dialog.Error()
	dialog.SetTitle("Error").SetMessage(message)
	dialog.AddButton("OK")
	dialog.Show()
}

func (a *App) openFileDialog(title string, filters []httpapi.FileFilter) (string, error) {
	if a == nil || a.desktop == nil {
		return "", errDesktopUnavailable
	}
	options := &application.OpenFileDialogOptions{Title: title, CanChooseFiles: true}
	for _, filter := range filters {
		options.Filters = append(options.Filters, application.FileFilter{
			DisplayName: filter.DisplayName,
			Pattern:     filter.Pattern,
		})
	}
	return a.desktop.Dialog.OpenFileWithOptions(options).PromptForSingleSelection()
}

func (a *App) openDirectoryDialog(title string) (string, error) {
	if a == nil || a.desktop == nil {
		return "", errDesktopUnavailable
	}
	return a.desktop.Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{
		Title:                title,
		CanChooseDirectories: true,
		CanCreateDirectories: true,
	}).PromptForSingleSelection()
}
