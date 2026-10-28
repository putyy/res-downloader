package updates

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strconv"
	"strings"
)

type Manifest struct {
	SchemaVersion int     `json:"schemaVersion"`
	Version       string  `json:"version"`
	Tag           string  `json:"tag"`
	PublishedAt   string  `json:"publishedAt"`
	GeneratedAt   string  `json:"generatedAt"`
	ReleaseURL    string  `json:"releaseUrl"`
	Notes         string  `json:"notes"`
	Assets        []Asset `json:"assets"`
}
type Asset struct {
	Platform string `json:"platform"`
	Arch     string `json:"arch"`
	Variant  string `json:"variant"`
	Name     string `json:"name"`
	URL      string `json:"url"`
	Size     int64  `json:"size"`
	SHA256   string `json:"sha256,omitempty"`
}

var stableVersion = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

var ErrInvalidManifest = errors.New("invalid update manifest")

func Stable(version string) bool { return stableVersion.MatchString(version) }
func Newer(next, current string) bool {
	next = strings.TrimPrefix(next, "v")
	current = strings.TrimPrefix(current, "v")
	base := strings.SplitN(current, "-", 2)[0]
	if !Stable(next) || !Stable(base) {
		return false
	}
	a, b := strings.Split(next, "."), strings.Split(base, ".")
	for i := range a {
		x, e1 := strconv.ParseUint(a[i], 10, 64)
		y, e2 := strconv.ParseUint(b[i], 10, 64)
		if e1 != nil || e2 != nil {
			return false
		}
		if x != y {
			return x > y
		}
	}
	return current != base
}
func Decode(raw []byte) (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return m, fmt.Errorf("%w: malformed JSON: %v", ErrInvalidManifest, err)
	}
	if !Stable(m.Version) {
		return m, fmt.Errorf("%w: version must be a stable release", ErrInvalidManifest)
	}
	if m.SchemaVersion != 1 {
		return m, fmt.Errorf("%w: schemaVersion must be 1", ErrInvalidManifest)
	}
	if strings.TrimPrefix(m.Tag, "v") != m.Version {
		return m, fmt.Errorf("%w: tag must match version", ErrInvalidManifest)
	}
	if len(m.Assets) == 0 {
		return m, fmt.Errorf("%w: assets must not be empty", ErrInvalidManifest)
	}
	for _, a := range m.Assets {
		if a.Name == "" || a.Name == "." || a.Name == ".." || path.Base(a.Name) != a.Name || strings.ContainsAny(a.Name, "\\\x00\r\n") || a.Size <= 0 || a.Size > 2<<30 || !validAssetURL(a.URL, m.Tag, a.Name) {
			return m, fmt.Errorf("%w: invalid asset URL, name or size", ErrInvalidManifest)
		}
		if a.SHA256 != "" {
			digest, err := hex.DecodeString(a.SHA256)
			if err != nil || len(digest) != 32 {
				return m, fmt.Errorf("%w: invalid SHA256", ErrInvalidManifest)
			}
		}
	}
	return m, nil
}
