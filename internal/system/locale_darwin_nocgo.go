//go:build darwin && !cgo

package system

func preferredLanguage() string {
	return ""
}
