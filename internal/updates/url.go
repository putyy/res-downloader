package updates

import (
	"errors"
	"net/http"
	"net/url"
	"path"
	"strings"

	"res-downloader/internal/metadata"
)

func validAssetURL(raw, tag, name string) bool {
	u, err := url.Parse(raw)
	if err != nil || !metadata.ValidHTTPSURL(u) {
		return false
	}
	expected := "/putyy/res-downloader/releases/download/" + tag + "/" + name
	if u.Host == "github.com" && u.Path == expected && u.RawQuery == "" {
		return true
	}
	return path.Base(u.Path) == name && (metadata.WithinSource(u, metadata.Domestic) || metadata.WithinSource(u, metadata.Overseas))
}

func checkDownloadRedirect(req *http.Request, via []*http.Request) error {
	if len(via) == 0 || len(via) >= 10 || !metadata.ValidHTTPSURL(req.URL) {
		return errors.New("untrusted update download redirect")
	}
	original := via[0].URL
	// A source-hosted package may redirect only within that same source directory.
	for _, source := range []string{metadata.Domestic, metadata.Overseas} {
		if metadata.WithinSource(original, source) && metadata.WithinSource(req.URL, source) {
			return nil
		}
	}
	// GitHub release downloads use signed githubusercontent.com redirects.
	host := strings.ToLower(req.URL.Hostname())
	if original.Host == "github.com" && strings.HasPrefix(original.Path, "/putyy/res-downloader/releases/download/") &&
		(req.URL.Port() == "" || req.URL.Port() == "443") && (host == "github.com" || strings.HasSuffix(host, ".githubusercontent.com")) {
		return nil
	}
	return errors.New("untrusted update download redirect")
}
