package app

func (a *App) dialogErr(message string) {
	if a.desktop != nil {
		a.desktop.Dialog.Error().SetTitle("Error").SetMessage(message).Show()
	}
}
