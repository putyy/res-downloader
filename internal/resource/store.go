package resource

import (
	"encoding/json"
	"errors"
	"os"
	"sort"
	"sync"
	"time"

	shared "res-downloader/internal/model"

	bolt "go.etcd.io/bbolt"
)

var bucketName = []byte("resources")

type Record struct {
	Candidate shared.ResourceCandidate `json:"candidate"`
	CreatedAt int64                    `json:"createdAt"`
	UpdatedAt int64                    `json:"updatedAt"`
}

type Store struct {
	db        *bolt.DB
	closeOnce sync.Once
	closeErr  error
}

func Open(fileName string) (*Store, error) {
	db, err := bolt.Open(fileName, 0600, &bolt.Options{Timeout: time.Second})
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(fileName, 0600); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucketName)
		return err
	}); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Upsert(candidate shared.ResourceCandidate) error {
	return s.UpsertMany([]shared.ResourceCandidate{candidate})
}

func (s *Store) UpsertMany(candidates []shared.ResourceCandidate) error {
	if s == nil || s.db == nil {
		return errors.New("resource database is not open")
	}
	if len(candidates) == 0 {
		return nil
	}
	now := time.Now().UnixMilli()
	return s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketName)
		for _, candidate := range candidates {
			persisted, err := shared.PersistentResourceCandidate(candidate)
			if err != nil {
				return err
			}
			createdAt := now
			if previous := bucket.Get([]byte(candidate.ID)); len(previous) > 0 {
				var record Record
				if json.Unmarshal(previous, &record) == nil && record.CreatedAt > 0 {
					createdAt = record.CreatedAt
				}
			}
			raw, err := json.Marshal(Record{Candidate: persisted, CreatedAt: createdAt, UpdatedAt: now})
			if err != nil {
				return err
			}
			if err := bucket.Put([]byte(candidate.ID), raw); err != nil {
				return err
			}
		}
		return nil
	})
}

func PersistentCandidate(candidate shared.ResourceCandidate) (shared.ResourceCandidate, error) {
	return shared.PersistentResourceCandidate(candidate)
}

func (s *Store) Delete(ids []string) error {
	if s == nil || s.db == nil || len(ids) == 0 {
		return nil
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(bucketName)
		for _, id := range ids {
			if err := bucket.Delete([]byte(id)); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) Clear() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		if err := tx.DeleteBucket(bucketName); err != nil && !errors.Is(err, bolt.ErrBucketNotFound) {
			return err
		}
		_, err := tx.CreateBucket(bucketName)
		return err
	})
}

func (s *Store) List() ([]Record, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	records := make([]Record, 0)
	err := s.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketName).ForEach(func(_, value []byte) error {
			var record Record
			if err := json.Unmarshal(value, &record); err != nil {
				return err
			}
			records = append(records, record)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	sort.SliceStable(records, func(i, j int) bool {
		return records[i].UpdatedAt > records[j].UpdatedAt
	})
	return records, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	s.closeOnce.Do(func() {
		s.closeErr = s.db.Close()
	})
	return s.closeErr
}
