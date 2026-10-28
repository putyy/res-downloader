package system

import "testing"

func TestLocaleForLanguage(t *testing.T) {
	for _, test := range []struct{ language, want string }{
		{"zh", "zh"}, {"zh-CN", "zh"}, {"zh-Hant-TW", "zh"},
		{"zh_HK.UTF-8", "zh"}, {" ZH_cn@variant ", "zh"},
		{"en-US", "en"}, {"ja-JP", "en"}, {"fr", "en"},
		{"", "en"}, {"C", "en"}, {"POSIX", "en"}, {"zhinvalid", "en"},
	} {
		t.Run(test.language, func(t *testing.T) {
			if got := localeForLanguage(test.language); got != test.want {
				t.Fatalf("localeForLanguage(%q) = %q, want %q", test.language, got, test.want)
			}
		})
	}
}
