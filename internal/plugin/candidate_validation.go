package plugin

import (
	"encoding/json"
	"errors"
	"fmt"
	shared "res-downloader/internal/model"
	"strings"
	"time"
)

func ValidateCandidate(candidate *shared.ResourceCandidate) error {
	if len(candidate.GroupKey) > 512 || len(candidate.ParentGroupKey) > 512 {
		return errors.New("resource group key exceeds 512 bytes")
	}
	if candidate.ParentGroupKey != "" && candidate.ParentGroupKey == candidate.GroupKey {
		return errors.New("resource cannot be its own parent")
	}
	if len(candidate.Tracks) == 0 && candidate.GroupKey == "" {
		return errors.New("resource requires groupKey when no track URL is available")
	}
	if candidate.Kind == "" {
		return errors.New("resource kind is required")
	}
	shared.NormalizeResourceCandidate(candidate, time.Now())
	if !shared.ValidPrimaryResourceType(candidate.PrimaryType) {
		return fmt.Errorf("resource primaryType %q is invalid", candidate.PrimaryType)
	}
	for _, trait := range candidate.Traits {
		if !shared.ValidResourceTrait(trait) {
			return fmt.Errorf("resource trait %q is invalid", trait)
		}
	}
	if candidate.DedupeKey == "" {
		if candidate.GroupKey != "" {
			candidate.DedupeKey = shared.Md5(candidate.Source.PluginID + "\x00" + candidate.GroupKey)
		} else if primary := shared.PrimaryResourceTrack(candidate.Tracks); primary != nil {
			candidate.DedupeKey = shared.Md5(primary.URL)
		}
	}
	if candidate.DedupeKey == "" {
		return errors.New("resource requires groupKey when its primary track has no URL")
	}
	if candidate.Metadata == nil {
		candidate.Metadata = map[string]interface{}{}
	}
	trackIDs := make(map[string]struct{}, len(candidate.Tracks))
	for index := range candidate.Tracks {
		track := &candidate.Tracks[index]
		if track.ID == "" {
			return errors.New("resource track id is required")
		}
		if _, exists := trackIDs[track.ID]; exists {
			return fmt.Errorf("resource has duplicate track id %q", track.ID)
		}
		trackIDs[track.ID] = struct{}{}
		if track.Role == "" {
			track.Role = "primary"
		}
		if track.Executor == "capture-file" {
			if strings.TrimSpace(track.CaptureKey) == "" || len(track.CaptureKey) > maxPluginCaptureKeySize || strings.IndexByte(track.CaptureKey, 0) >= 0 {
				return fmt.Errorf("resource track %q requires captureKey", track.ID)
			}
		} else if track.CaptureKey != "" {
			return fmt.Errorf("resource track %q uses captureKey without capture-file executor", track.ID)
		}
		if track.URL != "" {
			if err := shared.ValidateRemoteURL(track.URL); err != nil {
				return fmt.Errorf("resource track %q has invalid URL", track.ID)
			}
		}
		if !validExtension(track.Extension) {
			return fmt.Errorf("resource track %q has an invalid extension", track.ID)
		}
		for _, processor := range track.Processors {
			if err := validateDownloadProcessor(processor); err != nil {
				return fmt.Errorf("resource track %q: %w", track.ID, err)
			}
		}
	}
	for _, capability := range candidate.Capabilities {
		if !validIdentifier(capability) {
			return fmt.Errorf("resource capability %q is invalid", capability)
		}
	}
	if candidate.Preview != nil {
		if !validIdentifier(candidate.Preview.Renderer) {
			return errors.New("resource preview renderer is invalid")
		}
		if candidate.Preview.TrackID != "" {
			if _, exists := trackIDs[candidate.Preview.TrackID]; !exists {
				return fmt.Errorf("resource preview references unknown track %q", candidate.Preview.TrackID)
			}
		}
	}
	shared.NormalizeCandidateState(candidate)
	if raw, err := json.Marshal(candidate); err != nil || len(raw) > 1024*1024 {
		return errors.New("resource metadata exceeds the 1 MiB limit")
	}
	return nil
}

func validateCandidate(candidate *shared.ResourceCandidate) error {
	return ValidateCandidate(candidate)
}
