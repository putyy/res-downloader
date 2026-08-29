package plugin

import (
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	pluginCorrelationTTL  = 30 * time.Minute
	maxPluginCorrelations = 20000
)

type pluginCorrelationRegistration struct {
	GroupKey string   `json:"groupKey"`
	TrackID  string   `json:"trackId"`
	Role     string   `json:"role"`
	Aliases  []string `json:"aliases"`
}

type pluginCorrelationRef struct {
	GroupKey string `json:"groupKey"`
	TrackID  string `json:"trackId"`
	Role     string `json:"role"`
}

type pluginCorrelationEntry struct {
	refs      []pluginCorrelationRef
	expiresAt time.Time
	touchedAt time.Time
}

type pluginCorrelationStore struct {
	mu      sync.Mutex
	entries map[string]pluginCorrelationEntry
}

func newPluginCorrelationStore() *pluginCorrelationStore {
	return &pluginCorrelationStore{entries: make(map[string]pluginCorrelationEntry)}
}

func (s *pluginCorrelationStore) register(pluginID string, registration pluginCorrelationRegistration) {
	if pluginID == "" || registration.GroupKey == "" || registration.TrackID == "" {
		return
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked(now)
	ref := pluginCorrelationRef{GroupKey: registration.GroupKey, TrackID: registration.TrackID, Role: registration.Role}
	for _, alias := range registration.Aliases {
		key := correlationKey(pluginID, alias)
		if key == "" {
			continue
		}
		entry := s.entries[key]
		found := false
		for _, existing := range entry.refs {
			if existing == ref {
				found = true
				break
			}
		}
		if !found {
			entry.refs = append(entry.refs, ref)
		}
		entry.expiresAt = now.Add(pluginCorrelationTTL)
		entry.touchedAt = now
		s.entries[key] = entry
	}
	if len(s.entries) > maxPluginCorrelations {
		s.removeOldestLocked(len(s.entries) - maxPluginCorrelations)
	}
}

func (s *pluginCorrelationStore) find(pluginID, alias string) []pluginCorrelationRef {
	key := correlationKey(pluginID, alias)
	if key == "" {
		return nil
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, exists := s.entries[key]
	if !exists || now.After(entry.expiresAt) {
		delete(s.entries, key)
		return nil
	}
	entry.touchedAt = now
	entry.expiresAt = now.Add(pluginCorrelationTTL)
	s.entries[key] = entry
	return append([]pluginCorrelationRef(nil), entry.refs...)
}

func (s *pluginCorrelationStore) clearPlugin(pluginID string) {
	prefix := pluginID + "\x00"
	s.mu.Lock()
	defer s.mu.Unlock()
	for key := range s.entries {
		if strings.HasPrefix(key, prefix) {
			delete(s.entries, key)
		}
	}
}

func (s *pluginCorrelationStore) pruneLocked(now time.Time) {
	for key, entry := range s.entries {
		if now.After(entry.expiresAt) {
			delete(s.entries, key)
		}
	}
}

func (s *pluginCorrelationStore) removeOldestLocked(count int) {
	for count > 0 {
		var oldestKey string
		var oldest time.Time
		for key, entry := range s.entries {
			if oldestKey == "" || entry.touchedAt.Before(oldest) {
				oldestKey = key
				oldest = entry.touchedAt
			}
		}
		if oldestKey == "" {
			return
		}
		delete(s.entries, oldestKey)
		count--
	}
}

func correlationKey(pluginID, alias string) string {
	alias = strings.TrimSpace(alias)
	if alias == "" {
		return ""
	}
	if parsed, err := url.Parse(alias); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		parsed.Scheme = strings.ToLower(parsed.Scheme)
		parsed.Host = strings.ToLower(parsed.Host)
		parsed.Fragment = ""
		alias = parsed.String()
	}
	return pluginID + "\x00" + alias
}
