//go:build dev

package app

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	internalproxy "res-downloader/internal/proxy"
)

// This diagnostic is wired only into Wails development builds. Site matching
// stays here; the proxy dumper has no knowledge of individual plugins.
func configureDevelopmentResponseDump(proxy *Proxy, logger *Logger) {
	hosts, err := developmentResponseDumpHosts(os.Getenv("RESD_DUMP_HOSTS"))
	if err != nil {
		logger.Esg(err, "invalid RESD_DUMP_HOSTS; development response dump disabled")
		return
	}
	if len(hosts) == 0 {
		return
	}
	directory := strings.TrimSpace(os.Getenv("RESD_DUMP_DIR"))
	if directory == "" {
		directory = filepath.Join("test", "response-dump")
	}
	directory, err = filepath.Abs(directory)
	if err != nil {
		logger.Esg(err, "resolve development response dump directory")
		return
	}
	proxy.ConfigureResponseDump(internalproxy.ResponseDumpConfig{
		Directory: directory,
		Match: func(request *http.Request) bool {
			if request == nil || request.URL == nil {
				return false
			}
			host := strings.TrimSuffix(strings.ToLower(request.URL.Hostname()), ".")
			for _, pattern := range hosts {
				if strings.HasPrefix(pattern, "*.") {
					if strings.HasSuffix(host, pattern[1:]) {
						return true
					}
				} else if host == pattern {
					return true
				}
			}
			return false
		},
		ReportError: func(err error) {
			logger.Esg(err, "save development response dump")
		},
	})
	logger.Info().Str("directory", directory).Strs("hosts", hosts).Msg("development HTML/JS response dump enabled")
}

// Accept exact DNS hosts and a leading wildcard for subdomains. Reject URLs,
// ports and other glob syntax so a typo cannot silently broaden the scope.
func developmentResponseDumpHosts(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	label := `[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?`
	domain := regexp.MustCompile(`^` + label + `(?:\.` + label + `)*$`)
	var hosts []string
	seen := make(map[string]bool)
	for _, item := range strings.Split(value, ",") {
		pattern := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(item)), ".")
		host := strings.TrimPrefix(pattern, "*.")
		if len(host) > 253 || !domain.MatchString(host) {
			return nil, fmt.Errorf("expected comma-separated domains or *.domain patterns, got %q", item)
		}
		if !seen[pattern] {
			hosts = append(hosts, pattern)
			seen[pattern] = true
		}
	}
	return hosts, nil
}
