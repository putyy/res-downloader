package operation

import (
	"encoding/json"
	bolt "go.etcd.io/bbolt"
	m "res-downloader/internal/model"
	"sort"
	"time"
)

func (s *Service) load() error {
	return s.db.Update(func(tx *bolt.Tx) error {
		if raw := tx.Bucket([]byte("settings")).Get([]byte("policy")); raw != nil {
			if err := json.Unmarshal(raw, &s.settings); err != nil {
				return err
			}
		}
		if err := tx.Bucket([]byte("idempotency")).ForEach(func(k, v []byte) error {
			var key keyRecord
			if err := json.Unmarshal(v, &key); err != nil {
				return err
			}
			s.keys[string(k)] = key
			return nil
		}); err != nil {
			return err
		}
		b := tx.Bucket([]byte("executions"))
		changed := []*record{}
		if err := b.ForEach(func(k, v []byte) error {
			var r record
			if err := json.Unmarshal(v, &r); err != nil {
				return err
			}
			r.StoredResult = r.View.Result
			r.LeaseHeld = false
			if !Terminal(r.View.State) {
				r.View.State = "interrupted"
				r.View.ErrorCode = "host_restarted"
				r.View.UpdatedAt = time.Now().UnixMilli()
				if r.View.AcceptedAt > 0 {
					r.View.Certainty = "unknown"
				}
				changed = append(changed, &r)
			}
			s.records[string(k)] = &r
			return nil
		}); err != nil {
			return err
		}
		updates := s.keyUpdates(changed)
		if err := writeKeys(tx, updates); err != nil {
			return err
		}
		for key, value := range updates {
			s.keys[key] = value
		}
		for _, r := range changed {
			raw, err := json.Marshal(persistent(r))
			if err != nil {
				return err
			}
			if err = b.Put([]byte(r.View.ExecutionID), raw); err != nil {
				return err
			}
		}
		return tx.Bucket([]byte("artifacts")).ForEach(func(k, v []byte) error {
			var a storedArtifact
			if err := json.Unmarshal(v, &a); err != nil {
				return err
			}
			a.Value.CaptureKey = a.CaptureKey
			s.artifacts[string(k)] = a.Value
			return nil
		})
	})
}

type storedArtifact struct {
	Value      m.OperationArtifact `json:"value"`
	CaptureKey string              `json:"captureKey,omitempty"`
}

func persistent(r *record) *record {
	c := *r
	c.View = r.View
	c.Request = m.OperationRequest{}
	c.View.Progress = nil
	c.View.Result = r.StoredResult
	c.StoredResult = nil
	if !r.Definition.PersistResult {
		c.View.ArtifactIDs = nil
	}
	c.View.ResultProjection = len(r.StoredResult) > 0
	if len(c.View.Result) == 0 && c.View.ResultStatus == "available" {
		c.View.ResultStatus = "never_persisted"
	}
	return &c
}
func (s *Service) save(records []*record) error {
	if err := s.available(); err != nil {
		return err
	}
	updates := s.keyUpdates(records)
	err := s.db.Update(func(tx *bolt.Tx) error {
		if err := writeKeys(tx, updates); err != nil {
			return err
		}
		b := tx.Bucket([]byte("executions"))
		for _, r := range records {
			raw, e := json.Marshal(persistent(r))
			if e != nil {
				return e
			}
			if e = b.Put([]byte(r.View.ExecutionID), raw); e != nil {
				return e
			}
			if !r.Definition.PersistResult {
				continue
			}
			for _, artifactID := range r.View.ArtifactIDs {
				a := s.artifacts[artifactID]
				raw, e = json.Marshal(storedArtifact{a, a.CaptureKey})
				if e != nil {
					return e
				}
				if e = tx.Bucket([]byte("artifacts")).Put([]byte(artifactID), raw); e != nil {
					return e
				}
			}
		}
		return nil
	})
	if err != nil {
		s.fault = err
		return s.available()
	}
	for key, value := range updates {
		s.keys[key] = value
	}
	s.nextCleanup = 0
	return nil
}
func (s *Service) cleanLocked(history, results bool) error {
	now := time.Now().UnixMilli()
	if !history && !results && now < s.nextCleanup {
		return nil
	}
	if err := s.pruneKeys(); err != nil {
		return err
	}
	// Reads share a scan until a write or the next retention deadline. Never
	// delay result/artifact expiry or make expired idempotency records visible.
	nextCleanup := now + time.Minute.Milliseconds()
	for _, key := range s.keys {
		nextCleanup = min(nextCleanup, key.Expires)
	}
	historyAge := int64(s.settings.HistoryDays) * 24 * time.Hour.Milliseconds()
	resultAge := int64(s.settings.ResultHours) * time.Hour.Milliseconds()
	ordered := make([]*record, 0, len(s.records))
	for _, r := range s.records {
		ordered = append(ordered, r)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].View.CreatedAt > ordered[j].View.CreatedAt })
	changed := map[string]*record{}
	removed := []string{}
	removedArtifacts := map[string]bool{}
	resultCount, resultBytes, historyBytes := 0, 0, 0
	// Only replace value fields/slices on private copies; payloads need no JSON
	// round trip. Publish changed records only after the transaction commits.
	for i, original := range ordered {
		if original.Finalizing || original.LeaseHeld {
			continue
		}
		r := *original
		v := &r.View
		raw, _ := json.Marshal(persistent(&r))
		historyBytes += len(raw)
		if Terminal(v.State) && (history || now-v.CreatedAt > historyAge || i >= MaxHistory || historyBytes > MaxHistoryBytes) {
			removed = append(removed, v.ExecutionID)
			for _, a := range v.ArtifactIDs {
				removedArtifacts[a] = true
			}
			continue
		}
		if !Terminal(v.State) {
			continue
		}
		nextCleanup = min(nextCleanup, v.CreatedAt+historyAge+1)
		expired := v.ResultExpiresAt > 0 && (now >= v.ResultExpiresAt || now-v.UpdatedAt > resultAge)
		evicted := false
		if v.ResultStatus == "available" || len(v.ArtifactIDs) > 0 {
			resultCount++
			resultBytes += len(v.Result)
			evicted = resultCount > MaxResults || resultBytes > MaxResultTotalBytes
		}
		if (results || expired || evicted) && (v.ResultStatus == "available" || len(v.ArtifactIDs) > 0) {
			v.Result = nil
			r.StoredResult = nil
			v.ResultStatus = "cleaned"
			if expired {
				v.ResultStatus = "expired"
			}
			for _, a := range v.ArtifactIDs {
				removedArtifacts[a] = true
			}
			v.ArtifactIDs = []string{}
			changed[v.ExecutionID] = &r
		} else if (v.ResultStatus == "available" || len(v.ArtifactIDs) > 0) && v.ResultExpiresAt > 0 {
			nextCleanup = min(nextCleanup, v.ResultExpiresAt, v.UpdatedAt+resultAge+1)
		}
	}
	if len(changed) == 0 && len(removed) == 0 {
		s.nextCleanup = nextCleanup
		return nil
	}
	updates := map[string]keyRecord{}
	for _, r := range changed {
		for key, value := range s.keyUpdates([]*record{r}) {
			updates[key] = value
		}
	}
	for _, executionID := range removed {
		r := *s.records[executionID]
		r.View = keySnapshot(r.View)
		r.View.ArtifactIDs = []string{}
		for key, value := range s.keyUpdates([]*record{&r}) {
			updates[key] = value
		}
	}
	err := s.db.Update(func(tx *bolt.Tx) error {
		if err := writeKeys(tx, updates); err != nil {
			return err
		}
		b := tx.Bucket([]byte("executions"))
		for executionID, r := range changed {
			raw, err := json.Marshal(persistent(r))
			if err != nil {
				return err
			}
			if err = b.Put([]byte(executionID), raw); err != nil {
				return err
			}
		}
		for _, executionID := range removed {
			if err := b.Delete([]byte(executionID)); err != nil {
				return err
			}
		}
		for artifactID := range removedArtifacts {
			if err := tx.Bucket([]byte("artifacts")).Delete([]byte(artifactID)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		s.fault = err
		return s.available()
	}
	for key, value := range updates {
		s.keys[key] = value
	}
	for executionID, r := range changed {
		s.records[executionID] = r
	}
	for _, executionID := range removed {
		delete(s.records, executionID)
	}
	for artifactID := range removedArtifacts {
		delete(s.artifacts, artifactID)
	}
	s.nextCleanup = nextCleanup
	return nil
}

type cursor struct {
	Query, Session, Revision, Plugin, Operation, Value string
	Expires                                            int64
}

func (s *Service) createCursor(r *record, value string) (string, error) {
	if s.cursors == nil {
		s.cursors = map[string]cursor{}
	}
	for key, c := range s.cursors {
		if c.Expires < time.Now().UnixMilli() {
			delete(s.cursors, key)
		}
	}
	if len(s.cursors) >= MaxResults {
		return "", problem("cursor_limit", "cursor capacity exceeded")
	}
	key, e := id()
	if e != nil {
		return "", e
	}
	s.cursors[key] = cursor{hash(r.Request.Input), r.View.PageSessionID, r.View.Revision, r.View.PluginID, r.View.OperationID, value, time.Now().Add(CursorTTL).UnixMilli()}
	return key, nil
}
func (s *Service) resolveCursor(req m.OperationRequest, p m.OperationSession) (string, error) {
	c, ok := s.cursors[req.Cursor]
	if !ok || c.Expires < time.Now().UnixMilli() || c.Query != hash(req.Input) || c.Session != p.PageSessionID || c.Revision != p.Revision || c.Plugin != req.PluginID || c.Operation != req.OperationID {
		return "", problem("cursor_expired", "cursor is expired or does not match the query and page")
	}
	return c.Value, nil
}
