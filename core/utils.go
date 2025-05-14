package core

import (
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func DialogErr(message string) {
	_, _ = runtime.MessageDialog(appOnce.ctx, runtime.MessageDialogOptions{
		Type:          runtime.ErrorDialog,
		Title:         "Error",
		Message:       message,
		DefaultButton: "Cancel",
	})
}
