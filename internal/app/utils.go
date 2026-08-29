package app

import (
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) dialogErr(message string) {
	_, _ = runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
		Type:          runtime.ErrorDialog,
		Title:         "Error",
		Message:       message,
		DefaultButton: "Cancel",
	})
}
