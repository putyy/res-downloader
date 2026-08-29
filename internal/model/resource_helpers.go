package model

import (
	"errors"
	"net/url"
	"strings"
	"time"
	"unicode"
)

const ResourceSchemaVersion = 1

func ValidateRemoteURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return errors.New("invalid remote URL")
	}
	return nil
}

func PrimaryResourceTrack(tracks []ResourceTrack) *ResourceTrack {
	for index := range tracks {
		if tracks[index].Role == "primary" || tracks[index].Role == "video" {
			return &tracks[index]
		}
	}
	if len(tracks) > 0 {
		return &tracks[0]
	}
	return nil
}

func NormalizeCandidateState(candidate *ResourceCandidate) {
	available := make(map[string]bool, len(candidate.Tracks))
	for _, track := range candidate.Tracks {
		if track.URL != "" || (track.Executor == "capture-file" && track.CaptureKey != "") {
			available[track.Role] = true
		}
	}
	candidate.State = ResourceStateReady
	for _, role := range candidate.RequiredTracks {
		if !available[role] {
			candidate.State = ResourceStatePartial
			break
		}
	}
}

func NormalizeResourceCandidate(candidate *ResourceCandidate, now time.Time) {
	// Stream kinds remain opaque and plugin-extensible, while the two built-in
	// audiovisual stream kinds have stable main-type and trait semantics.
	if candidate.Kind == "stream.live" {
		candidate.PrimaryType = ResourceTypeVideo
		candidate.Traits = appendUnique(candidate.Traits, ResourceTraitStreaming, ResourceTraitLive)
	} else if candidate.Kind == "stream.hls" {
		candidate.PrimaryType = ResourceTypeVideo
		candidate.Traits = appendUnique(candidate.Traits, ResourceTraitSegmented, ResourceTraitStreaming)
	}
	if candidate.PrimaryType == "" {
		candidate.PrimaryType = primaryTypeFromKind(candidate.Kind)
	}
	if !ValidPrimaryResourceType(candidate.PrimaryType) {
		candidate.PrimaryType = ResourceTypeOther
	}
	if candidate.Kind == ResourceKindCollection {
		candidate.Traits = appendUnique(candidate.Traits, ResourceTraitHasChildren)
	}
	if len(candidate.Tracks) > 1 {
		candidate.Traits = appendUnique(candidate.Traits, ResourceTraitMultiTrack)
	}
	if len(candidate.RequiredTracks) > 1 {
		candidate.Traits = appendUnique(candidate.Traits, ResourceTraitMergeRequired)
	}
	if contains(candidate.Capabilities, ResourceCapabilityDownload) {
		candidate.Traits = appendUnique(candidate.Traits, ResourceTraitDownloadable)
	}
	if contains(candidate.Capabilities, ResourceCapabilityPreview) {
		candidate.Traits = appendUnique(candidate.Traits, ResourceTraitPreviewable)
	}
	for _, track := range candidate.Tracks {
		if track.Executor == "hls" || track.Executor == "ffmpeg-hls" {
			candidate.Traits = appendUnique(candidate.Traits, ResourceTraitSegmented, ResourceTraitStreaming)
		}
		if track.Executor == "ffmpeg-hls" {
			candidate.Traits = appendUnique(candidate.Traits, ResourceTraitLive)
		}
		if len(track.Processors) > 0 {
			candidate.Traits = appendUnique(candidate.Traits, ResourceTraitEncrypted)
		}
	}
	if candidate.Technical.MIME == "" {
		if track := PrimaryResourceTrack(candidate.Tracks); track != nil {
			candidate.Technical.MIME, candidate.Technical.Codecs = track.MIME, track.Codecs
			candidate.Technical.Container = strings.TrimPrefix(track.Extension, ".")
		}
	}
	nowMillis := now.UnixMilli()
	if candidate.Lifecycle.SchemaVersion <= 0 {
		candidate.Lifecycle.SchemaVersion = ResourceSchemaVersion
	}
	if candidate.Lifecycle.DiscoveredAt <= 0 {
		candidate.Lifecycle.DiscoveredAt = nowMillis
	}
	candidate.Lifecycle.UpdatedAt = nowMillis
	if candidate.Lifecycle.Availability == "" {
		candidate.Lifecycle.Availability = ResourceAvailabilityAvailable
	}
	if candidate.Lifecycle.ExpiresAt > 0 && candidate.Lifecycle.ExpiresAt <= nowMillis && candidate.Lifecycle.Availability == ResourceAvailabilityAvailable {
		candidate.Lifecycle.Availability = ResourceAvailabilityNeedsRefresh
	}
}

func ValidPrimaryResourceType(value string) bool {
	switch value {
	case ResourceTypeVideo, ResourceTypeAudio, ResourceTypeImage, ResourceTypeDocument, ResourceTypeArchive, ResourceTypeCollection, ResourceTypeOther:
		return true
	default:
		return false
	}
}

func ValidResourceTrait(value string) bool {
	if validResourceIdentifier(value) {
		return true
	}
	parts := strings.Split(value, ":")
	return len(parts) == 2 && validResourceIdentifier(parts[0]) && validResourceIdentifier(parts[1])
}

func ResourceNeedsRecaptureForHeaders(candidate ResourceCandidate) bool {
	for _, track := range candidate.Tracks {
		for _, name := range track.NonPersistentHeaders {
			found := false
			for header := range track.Headers {
				if strings.EqualFold(header, name) {
					found = true
					break
				}
			}
			if !found {
				return true
			}
		}
	}
	return false
}

func primaryTypeFromKind(kind string) string {
	leaf := kind
	if index := strings.LastIndex(kind, "."); index >= 0 {
		leaf = kind[index+1:]
	}
	switch strings.ToLower(leaf) {
	case "video":
		return ResourceTypeVideo
	case "audio":
		return ResourceTypeAudio
	case "image", "photo", "picture":
		return ResourceTypeImage
	case "document", "pdf", "text":
		return ResourceTypeDocument
	case "archive", "zip", "rar", "7z":
		return ResourceTypeArchive
	case "collection", "gallery":
		return ResourceTypeCollection
	default:
		return ResourceTypeOther
	}
}

func appendUnique(values []string, additions ...string) []string {
	for _, addition := range additions {
		if !contains(values, addition) {
			values = append(values, addition)
		}
	}
	return values
}
func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
func validResourceIdentifier(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, char := range value {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) && char != '.' && char != '-' && char != '_' {
			return false
		}
	}
	return true
}
