package model

// ResourceView is the catalog representation returned to the desktop UI.
// ResourceCandidate remains the single resource model; Children only adds the
// hierarchy needed for presentation and import/export.
type ResourceView struct {
	ResourceCandidate
	Children []ResourceView `json:"children,omitempty"`
}
