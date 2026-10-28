package model

import "strings"

// PluginStoreIndex is the portable JSON contract generated from GitHub and
// consumed by the desktop client. The index contains metadata and immutable
// package references only; plugin code is still loaded from a verified ZIP.
type PluginStoreIndex struct {
	SchemaVersion int                `json:"schemaVersion"`
	GeneratedAt   string             `json:"generatedAt"`
	Topic         string             `json:"topic"`
	Extensions    []PluginStoreEntry `json:"extensions"`
}

type PluginStoreEntry struct {
	ID            string                  `json:"id,omitempty"`
	Name          string                  `json:"name"`
	Description   string                  `json:"description,omitempty"`
	Repository    string                  `json:"repository"`
	Stars         int                     `json:"stars,omitempty"`
	License       string                  `json:"license,omitempty"`
	UpdatedAt     string                  `json:"updatedAt,omitempty"`
	Source        string                  `json:"source"`
	Status        string                  `json:"status"`
	StatusMessage string                  `json:"statusMessage,omitempty"`
	Author        string                  `json:"author,omitempty"`
	Locales       map[string]PluginLocale `json:"locales,omitempty"`
	Capabilities  []string                `json:"capabilities,omitempty"`
	Release       *PluginStoreRelease     `json:"release,omitempty"`
}

type PluginStoreRelease struct {
	Tag         string `json:"tag"`
	PublishedAt string `json:"publishedAt,omitempty"`
	Accelerated bool   `json:"accelerated,omitempty"`
}

// Version returns the release tag without its leading "v", so "v1.0.3" and
// "1.0.3" compare equal against manifest versions.
func (release PluginStoreRelease) Version() string {
	return strings.TrimPrefix(release.Tag, "v")
}

const (
	PluginStoreSchemaVersion = 1
	PluginStoreMaxIndexSize  = 4 * 1024 * 1024
	PluginStoreTopic         = "res-downloader-ext"
	PluginStoreAvailable     = "available"
	PluginStoreUnavailable   = "unavailable"
	PluginSourceBuiltin      = "builtin"
	PluginSourceOfficial     = "official"
	PluginSourceCommunity    = "community"
)
