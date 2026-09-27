package updates

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"res-downloader/internal/metadata"
)

func configureSources(t *testing.T) {
	t.Helper()
	domestic, overseas := metadata.Domestic, metadata.Overseas
	metadata.Domestic = "https://example.com/update-test/"
	metadata.Overseas = "https://mirror.example.com/update-test"
	t.Cleanup(func() { metadata.Domestic, metadata.Overseas = domestic, overseas })
}

func TestConfiguredSourceAssets(t *testing.T) {
	configureSources(t)
	for _, test := range []struct {
		address string
		want    bool
	}{
		{"https://example.com/update-test/new.exe", true},
		{"https://mirror.example.com/update-test/packages/new.exe", true},
		{"https://example.com/update-testing/new.exe", false},
		{"https://example.com/update-test/../new.exe", false},
		{"https://example.com/update-test/different.exe", false},
		{"https://unconfigured.example.com/update-test/new.exe", false},
		{"http://example.com/update-test/new.exe", false},
		{"https://github.com/putyy/res-downloader/releases/download/v4.0.1/new.exe", true},
		{"https://github.com/putyy/res-downloader/releases/download/v4.0.0/new.exe", false},
		{"https://github.com/another/repo/releases/download/v4.0.1/new.exe", false},
	} {
		manifest := Manifest{SchemaVersion: 1, Version: "4.0.1", Tag: "v4.0.1", Assets: []Asset{{Name: "new.exe", URL: test.address, Size: 123}}}
		raw, err := json.Marshal(manifest)
		if err != nil {
			t.Fatal(err)
		}
		_, err = Decode(raw)
		if (err == nil) != test.want {
			t.Errorf("%s: error=%v", test.address, err)
		}
	}
}

func TestPackageRedirectBoundaries(t *testing.T) {
	configureSources(t)
	const github = "https://github.com/putyy/res-downloader/releases/download/v4.0.1/new.exe"
	const custom = "https://example.com/update-test/new.exe"
	for _, test := range []struct {
		from, to string
		want     bool
	}{
		{custom, "https://example.com/update-test/assets/new.exe", true},
		{custom, "https://example.com/update-test/new.exe?download=1", true},
		{custom, "https://example.com/other/new.exe", false},
		{custom, "https://mirror.example.com/update-test/new.exe", false},
		{custom, github, false},
		{custom, "https://release-assets.githubusercontent.com/package", false},
		{custom, "http://example.com/update-test/new.exe", false},
		{custom, "https://user@example.com/update-test/new.exe", false},
		{github, "https://release-assets.githubusercontent.com/package?signature=value", true},
		{github, "https://github.com/putyy/res-downloader/releases/download/v4.0.1/new.exe", true},
		{github, custom, false},
		{github, "https://github.com.evil.test/package", false},
		{github, "https://github.com:8443/package", false},
		{github, "http://release-assets.githubusercontent.com/package", false},
	} {
		from, err := url.Parse(test.from)
		if err != nil {
			t.Fatal(err)
		}
		to, err := url.Parse(test.to)
		if err != nil {
			t.Fatal(err)
		}
		err = checkDownloadRedirect(&http.Request{URL: to}, []*http.Request{{URL: from}})
		if (err == nil) != test.want {
			t.Errorf("%s -> %s: %v", test.from, test.to, err)
		}
	}
	original, _ := url.Parse(custom)
	via := make([]*http.Request, 10)
	for i := range via {
		via[i] = &http.Request{URL: original}
	}
	if checkDownloadRedirect(&http.Request{URL: original}, via) == nil {
		t.Error("accepted too many redirects")
	}
}
