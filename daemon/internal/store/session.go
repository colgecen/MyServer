package store

import (
	"encoding/json"
	"time"

	bolt "go.etcd.io/bbolt"
)

type ChatSession struct {
	ID          string    `json:"id"`
	Workspace   string    `json:"workspace"`
	Messages    []string  `json:"messages"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (k *KV) SaveSession(s ChatSession) error {
	s.UpdatedAt = time.Now()
	if s.CreatedAt.IsZero() {
		s.CreatedAt = s.UpdatedAt
	}
	b, _ := json.Marshal(s)
	return k.Put(BucketSessions, s.ID, b)
}

func (k *KV) LoadSession(id string) (*ChatSession, error) {
	b, err := k.Get(BucketSessions, id)
	if err != nil {
		return nil, err
	}
	var s ChatSession
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (k *KV) ListSessions() ([]ChatSession, error) {
	var out []ChatSession
	// simple scan via View
	err := k.db.View(func(tx *bolt.Tx) error {
		cur := tx.Bucket([]byte(BucketSessions)).Cursor()
		for k, v := cur.First(); k != nil; k, v = cur.Next() {
			var s ChatSession
			if json.Unmarshal(v, &s) == nil {
				out = append(out, s)
			}
		}
		return nil
	})
	return out, err
}
