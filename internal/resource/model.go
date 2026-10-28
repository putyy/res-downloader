package resource

import (
	shared "res-downloader/internal/model"
	"res-downloader/internal/plugin"
	"time"
)

func validateCandidate(candidate *shared.ResourceCandidate) error {
	return plugin.ValidateCandidate(candidate)
}
func normalizeResourceModel(candidate *shared.ResourceCandidate, now time.Time) {
	shared.NormalizeResourceCandidate(candidate, now)
}
func primaryResourceTrack(tracks []shared.ResourceTrack) *shared.ResourceTrack {
	return shared.PrimaryResourceTrack(tracks)
}
func resourceNeedsRecaptureForHeaders(candidate shared.ResourceCandidate) bool {
	return shared.ResourceNeedsRecaptureForHeaders(candidate)
}
