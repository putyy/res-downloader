//go:build !darwin && !windows

package system

import "testing"

func TestLanguageFromEnvironment(t *testing.T) {
	for _, test := range []struct {
		name string
		env  map[string]string
		want string
	}{
		{"language priority", map[string]string{"LANGUAGE": "zh_TW:en", "LANG": "en_US.UTF-8"}, "zh_TW"},
		{"first preference unsupported", map[string]string{"LANGUAGE": "ja:zh:en"}, "ja"},
		{"lc all", map[string]string{"LC_ALL": "en_US.UTF-8", "LC_MESSAGES": "zh_CN", "LANG": "zh_CN"}, "en_US.UTF-8"},
		{"messages", map[string]string{"LC_MESSAGES": "zh_CN", "LANG": "en_US.UTF-8"}, "zh_CN"},
		{"lang", map[string]string{"LANG": "zh_CN.UTF-8"}, "zh_CN.UTF-8"},
		{"c locale", map[string]string{"LC_ALL": "C", "LANGUAGE": "zh", "LANG": "zh_CN"}, "C"},
		{"posix locale", map[string]string{"LANG": "POSIX", "LANGUAGE": "zh"}, "POSIX"},
		{"missing", nil, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := languageFromEnvironment(func(key string) string { return test.env[key] })
			if got != test.want {
				t.Fatalf("languageFromEnvironment() = %q, want %q", got, test.want)
			}
		})
	}
}
