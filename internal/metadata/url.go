package metadata

import (
	"net/url"
	"path"
	"strings"
)

// WithinSource accepts only HTTPS URLs inside a configured origin and directory.
// Compare decoded paths and reject ambiguous traversal/escaping before prefix matching.
func WithinSource(target *url.URL, source string) bool {
	base, err := url.Parse(normalizeSource(source))
	if err != nil || !ValidHTTPSURL(base) || !ValidHTTPSURL(target) || base.RawQuery != "" || base.ForceQuery {
		return false
	}
	port := func(u *url.URL) string {
		if u.Port() == "" {
			return "443"
		}
		return u.Port()
	}
	if !strings.EqualFold(base.Hostname(), target.Hostname()) || port(base) != port(target) {
		return false
	}
	cleanPath := func(u *url.URL) (string, bool) {
		value := u.Path
		if value == "" {
			value = "/"
		}
		if strings.ContainsAny(value, "%\\\x00\r\n") {
			return "", false
		}
		trimmed := strings.TrimSuffix(value, "/")
		if value != "/" && path.Clean(value) != trimmed {
			return "", false
		}
		// Encoded separators can be interpreted differently by reverse proxies.
		escaped := strings.ToLower(u.EscapedPath())
		if strings.Contains(escaped, "%2f") || strings.Contains(escaped, "%5c") {
			return "", false
		}
		return trimmed, true
	}
	basePath, baseOK := cleanPath(base)
	targetPath, targetOK := cleanPath(target)
	return baseOK && targetOK && strings.HasPrefix(targetPath, basePath+"/")
}

func ValidHTTPSURL(u *url.URL) bool {
	return u != nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil && u.Opaque == "" && u.Fragment == ""
}
