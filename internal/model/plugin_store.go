package model

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
	ID             string              `json:"id,omitempty"`
	Name           string              `json:"name"`
	Description    string              `json:"description,omitempty"`
	Repository     string              `json:"repository"`
	RepositoryURL  string              `json:"repositoryUrl"`
	Homepage       string              `json:"homepage,omitempty"`
	Owner          string              `json:"owner"`
	OwnerAvatarURL string              `json:"ownerAvatarUrl,omitempty"`
	Stars          int                 `json:"stars,omitempty"`
	Forks          int                 `json:"forks,omitempty"`
	License        string              `json:"license,omitempty"`
	UpdatedAt      string              `json:"updatedAt,omitempty"`
	Source         string              `json:"source"`
	Status         string              `json:"status"`
	StatusMessage  string              `json:"statusMessage,omitempty"`
	Manifest       *PluginManifest     `json:"manifest,omitempty"`
	Release        *PluginStoreRelease `json:"release,omitempty"`
}

type PluginStoreRelease struct {
	Version        string `json:"version"`
	Tag            string `json:"tag"`
	PublishedAt    string `json:"publishedAt,omitempty"`
	NotesURL       string `json:"notesUrl,omitempty"`
	ArchiveURL     string `json:"archiveUrl"`
	AcceleratedURL string `json:"acceleratedUrl,omitempty"`
}

const (
	PluginStoreSchemaVersion = 1
	PluginStoreTopic         = "res-downloader-ext"
	PluginStoreAvailable     = "available"
	PluginStoreUnavailable   = "unavailable"
	PluginSourceBuiltin      = "builtin"
	PluginSourceOfficial     = "official"
	PluginSourceCommunity    = "community"
)
