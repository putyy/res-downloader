package download

import (
	"encoding/json"
	"errors"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	shared "res-downloader/internal/model"

	bolt "go.etcd.io/bbolt"
)

var bucketName = []byte("tasks")

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

func (s *Store) Upsert(task shared.DownloadTaskRecord) error {
	if s == nil || s.db == nil {
		return errors.New("download task database is not open")
	}
	persisted, err := persistentTask(task)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(persisted)
	if err != nil {
		return err
	}
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketName).Put([]byte(task.ID), raw)
	})
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

func persistentTask(task shared.DownloadTaskRecord) (shared.DownloadTaskRecord, error) {
	persisted, err := shared.CloneJSON(task)
	if err != nil {
		return shared.DownloadTaskRecord{}, err
	}
	persisted.Resource, err = shared.PersistentResourceCandidate(task.Resource)
	if err != nil {
		return shared.DownloadTaskRecord{}, err
	}
	scrubPlanHeaders(&persisted.Plan, task.Resource.Tracks)
	for index := range task.Items {
		persisted.Items[index].Resource, err = shared.PersistentResourceCandidate(task.Items[index].Resource)
		if err != nil {
			return shared.DownloadTaskRecord{}, err
		}
		scrubPlanHeaders(&persisted.Items[index].Plan, task.Items[index].Resource.Tracks)
	}
	return persisted, nil
}

func scrubPlanHeaders(plan *shared.DownloadPlan, tracks []shared.ResourceTrack) {
	for _, track := range tracks {
		if len(track.NonPersistentHeaders) == 0 {
			continue
		}
		for inputIndex := range plan.Inputs {
			if plan.Inputs[inputIndex].ID != track.ID {
				continue
			}
			for header := range plan.Inputs[inputIndex].Headers {
				for _, excluded := range track.NonPersistentHeaders {
					if strings.EqualFold(header, excluded) {
						delete(plan.Inputs[inputIndex].Headers, header)
					}
				}
			}
		}
	}
}

func (s *Store) List() ([]shared.DownloadTaskRecord, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	tasks := make([]shared.DownloadTaskRecord, 0)
	err := s.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket(bucketName).ForEach(func(_, value []byte) error {
			var task shared.DownloadTaskRecord
			if err := json.Unmarshal(value, &task); err != nil {
				return err
			}
			tasks = append(tasks, task)
			return nil
		})
	})
	sort.Slice(tasks, func(i, j int) bool {
		if tasks[i].CreatedAt == tasks[j].CreatedAt {
			return tasks[i].ID < tasks[j].ID
		}
		return tasks[i].CreatedAt > tasks[j].CreatedAt
	})
	return tasks, err
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	s.closeOnce.Do(func() { s.closeErr = s.db.Close() })
	return s.closeErr
}
