package main

import "testing"

func TestReleaseAssetClassification(t *testing.T) {
	for _, test := range []struct{ name, platform, arch, variant string }{
		{"res-downloader_4.0.0_win_amd64.exe", "windows", "amd64", "installer"},
		{"res-downloader_4.0.0_win_arm64.exe", "windows", "arm64", "installer"},
		{"res-downloader_4.0.0_win_amd64_fixed_webview2.exe", "windows", "amd64", "fixed-webview2"},
		{"res-downloader_4.0.0_mac.dmg", "darwin", "universal", "dmg"},
		{"res-downloader_4.0.0_linux_amd64.AppImage", "linux", "amd64", "appimage"},
		{"res-downloader_4.0.0_linux_arm64.deb", "linux", "arm64", "deb"},
		{"res-downloader_4.0.0_linux_arm64", "linux", "arm64", "binary"},
		{"res-downloader_4.1.0_win_amd64.exe", "", "", ""},
	} {
		p, a, v := classifyAsset(test.name, "4.0.0")
		if p != test.platform || a != test.arch || v != test.variant {
			t.Errorf("%s classified as %s/%s/%s", test.name, p, a, v)
		}
	}
}
