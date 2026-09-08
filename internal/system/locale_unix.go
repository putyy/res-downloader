//go:build !darwin && !windows

package system

import (
	"os"
	"strings"
)

func preferredLanguage() string {
	return languageFromEnvironment(os.Getenv)
}

func languageFromEnvironment(getenv func(string) string) string {
	var locale string
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if value := strings.TrimSpace(getenv(key)); value != "" {
			locale = value
			break
		}
	}
	// gettext ignores LANGUAGE when message localization is explicitly disabled.
	if locale == "C" || locale == "POSIX" {
		return locale
	}
	for _, language := range strings.Split(getenv("LANGUAGE"), ":") {
		if language = strings.TrimSpace(language); language != "" {
			return language
		}
	}
	return locale
}
