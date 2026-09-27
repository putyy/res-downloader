// Package metadata selects a responsive official metadata mirror for this session.
package metadata

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"res-downloader/internal/system"
)

const domesticWebsite = "https://res.putyy.com"
const overseasWebsite = "https://putyy.github.io/res-downloader"

// Override either metadata base URL at build time with -ldflags -X.
// Set both to the same HTTPS directory to use a single metadata source.
var Domestic = domesticWebsite
var Overseas = overseasWebsite

var ErrNotFound = errors.New("metadata not found")

func normalizeSource(source string) string {
	return strings.TrimRight(strings.TrimSpace(source), "/")
}

var preference struct {
	sync.Mutex
	source string
}

func Preferred() string {
	preference.Lock()
	defer preference.Unlock()
	if preference.source != "" {
		return preference.source
	}
	if system.DefaultLocale() == "zh" {
		return normalizeSource(Domestic)
	}
	return normalizeSource(Overseas)
}

// Fetch starts the alternate mirror after two seconds (or immediately on failure).
// A response wins only after its caller's schema validation succeeds.
func Fetch(ctx context.Context, path string, validate func([]byte) error) ([]byte, string, error) {
	return fetch(ctx, path, validate, Read)
}

// FetchWithProxy keeps the same mirror validation and redirect restrictions,
// while letting the updater use a snapshot of the user's download proxy settings.
func FetchWithProxy(ctx context.Context, path string, validate func([]byte) error, proxy func(*http.Request) (*url.URL, error)) ([]byte, string, error) {
	return fetch(ctx, path, validate, func(ctx context.Context, address string, maximum int64) ([]byte, error) {
		return readWithProxy(ctx, address, maximum, proxy)
	})
}

func fetch(ctx context.Context, path string, validate func([]byte) error, read func(context.Context, string, int64) ([]byte, error)) ([]byte, string, error) {
	first := Preferred()
	second := normalizeSource(Domestic)
	if first == second {
		second = normalizeSource(Overseas)
	}
	count := 2
	if first == second {
		count = 1
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	type result struct {
		raw    []byte
		source string
		err    error
	}
	results := make(chan result, 2)
	fetch := func(source string) {
		raw, err := read(ctx, source+path, 4<<20)
		if err == nil {
			err = validate(raw)
		}
		results <- result{raw, source, err}
	}
	go fetch(first)
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	started, failures := count == 1, 0
	var errs []error
	for {
		select {
		case <-ctx.Done():
			return nil, "", errors.Join(append(errs, ctx.Err())...)
		case <-timer.C:
			if !started {
				started = true
				go fetch(second)
			}
		case value := <-results:
			if value.err == nil {
				preference.Lock()
				preference.source = value.source
				preference.Unlock()
				return value.raw, value.source, nil
			}
			errs = append(errs, fmt.Errorf("%s: %w", value.source, value.err))
			failures++
			if failures == count {
				return nil, "", errors.Join(errs...)
			}
			if !started {
				started = true
				go fetch(second)
			}
		}
	}
}

func Read(ctx context.Context, address string, maximum int64) ([]byte, error) {
	return readWithProxy(ctx, address, maximum, nil)
}

func readWithProxy(ctx context.Context, address string, maximum int64, proxy func(*http.Request) (*url.URL, error)) ([]byte, error) {
	parsed, err := url.Parse(address)
	if err != nil {
		return nil, err
	}
	source := normalizeSource(Domestic)
	if !WithinSource(parsed, source) {
		source = normalizeSource(Overseas)
	}
	if !WithinSource(parsed, source) {
		return nil, errors.New("invalid metadata source URL")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "res-downloader")
	// No desktop API token is attached. Callers must reject the capture proxy
	// when selecting a configured upstream; nil explicitly means a direct request.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = proxy
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 20 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 || !WithinSource(req.URL, source) {
			return errors.New("untrusted metadata redirect")
		}
		return nil
	}}
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%w: HTTP %s", ErrNotFound, response.Status)
	}
	if response.StatusCode != http.StatusOK {
		return nil, errors.New("metadata HTTP status: " + response.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, maximum+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > maximum {
		return nil, errors.New("metadata is too large")
	}
	return raw, nil
}

func InstallationURL(source, locale string) string {
	return DocumentationURL(source, locale, "installation")
}

// DocumentationURL accepts page identifiers so callers do not construct site URLs.
// An unknown page returns an empty URL.
func DocumentationURL(source, locale, page string) string {
	path := ""
	switch page {
	case "home":
	case "installation":
		path = "guide/installation.html"
	case "filename-template":
		path = "guide/settings.html#filename-template"
		if locale == "zh" {
			path = "guide/settings.html#文件命名模板"
		}
	default:
		return ""
	}
	source = normalizeSource(source)
	if source != normalizeSource(Domestic) && source != normalizeSource(Overseas) {
		source = Preferred()
	}
	// Metadata overrides may include a test directory; documentation stays on the official sites.
	website := overseasWebsite
	if system.DefaultLocale() == "zh" {
		website = domesticWebsite
	}
	if parsed, err := url.Parse(source); err == nil {
		switch strings.ToLower(parsed.Hostname()) {
		case "res.putyy.com":
			website = domesticWebsite
		case "putyy.github.io":
			website = overseasWebsite
		}
	}
	if locale != "zh" {
		return website + "/en/" + path
	}
	return website + "/" + path
}
