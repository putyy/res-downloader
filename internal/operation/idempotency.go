package operation

import (
	"encoding/json"
	bolt "go.etcd.io/bbolt"
	m "res-downloader/internal/model"
	"time"
)

// Keys have a separate lifecycle: deleting history must not replay side effects.
// A tombstone exposes the original ID/state with resultStatus=cleaned until expiry.
type keyRecord struct {
	Hash      string               `json:"hash"`
	Expires   int64                `json:"expires"`
	BatchID   string               `json:"batchId,omitempty"`
	Execution m.OperationExecution `json:"execution"`
}

func keySnapshot(v m.OperationExecution) m.OperationExecution {
	v.Result = nil
	v.InputSummary = nil
	v.Progress = nil
	v.ResultProjection = false
	v.ResultStatus = "cleaned"
	v.ArtifactIDs = []string{}
	return v
}
func (s *Service) keyUpdates(records []*record) map[string]keyRecord {
	out := map[string]keyRecord{}
	now := time.Now().UnixMilli()
	for _, r := range records {
		if r.KeyExpires <= now {
			continue
		}
		if r.Key != "" && (s.keys[r.Key].Expires <= now || s.keys[r.Key].Execution.ExecutionID == r.View.ExecutionID) {
			out[r.Key] = keyRecord{Hash: r.Hash, Expires: r.KeyExpires, Execution: keySnapshot(r.View)}
		}
		if r.BatchKey != "" && (s.keys[r.BatchKey].Expires <= now || s.keys[r.BatchKey].BatchID == r.View.BatchID) {
			out[r.BatchKey] = keyRecord{Hash: r.BatchHash, Expires: r.KeyExpires, BatchID: r.View.BatchID}
		}
	}
	return out
}
func writeKeys(tx *bolt.Tx, updates map[string]keyRecord) error {
	b := tx.Bucket([]byte("idempotency"))
	for key, value := range updates {
		raw, err := json.Marshal(value)
		if err != nil {
			return err
		}
		if err = b.Put([]byte(key), raw); err != nil {
			return err
		}
	}
	return nil
}
func (s *Service) pruneKeys() error {
	expired := []string{}
	for key, value := range s.keys {
		if value.Expires <= time.Now().UnixMilli() {
			expired = append(expired, key)
		}
	}
	if len(expired) == 0 {
		return nil
	}
	err := s.db.Update(func(tx *bolt.Tx) error {
		for _, key := range expired {
			if err := tx.Bucket([]byte("idempotency")).Delete([]byte(key)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		s.fault = err
		return s.available()
	}
	for _, key := range expired {
		delete(s.keys, key)
	}
	return nil
}

func validateItemBudget(value interface{}, limit int) error {
	switch value := value.(type) {
	case []interface{}:
		if len(value) > MaxItems {
			return problem("output_too_large", "array exceeds the 100-item host bound")
		}
		for _, v := range value {
			if err := validateItemBudget(v, limit); err != nil {
				return err
			}
		}
	case map[string]interface{}:
		for key, v := range value {
			if key == "items" {
				if items, ok := v.([]interface{}); ok && len(items) > limit {
					return problem("output_too_large", "items exceed the requested page budget")
				}
			}
			if err := validateItemBudget(v, limit); err != nil {
				return err
			}
		}
	}
	return nil
}
