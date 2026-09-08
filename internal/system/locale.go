package system

import "strings"

// DefaultLocale maps the user's first preferred UI language to a supported locale.
// An unsupported or unavailable language falls back to English.
func DefaultLocale() string {
	return localeForLanguage(preferredLanguage())
}

func localeForLanguage(language string) string {
	language = strings.ToLower(strings.TrimSpace(language))
	language = strings.SplitN(language, ".", 2)[0]
	language = strings.SplitN(language, "@", 2)[0]
	language = strings.ReplaceAll(language, "_", "-")
	if language == "zh" || strings.HasPrefix(language, "zh-") {
		return "zh"
	}
	return "en"
}
