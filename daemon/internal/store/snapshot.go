package store

import (
	"encoding/json"
	"time"
)

type Snapshot struct {
	ID        string    `json:"id"`
	Workspace string    `json:"workspace"`
	Files     []string  `json:"files"`
	CreatedAt time.Time `json:"created_at"`
}

func (k *KV) SaveSnapshot(s Snapshot) error {
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now()
	}
	b, _ := json.Marshal(s)
	return k.Put(BucketSnapshots, s.ID, b)
}

func (k *KV) LoadSnapshot(id string) (*Snapshot, error) {
	b, err := k.Get(BucketSnapshots, id)
	if err != nil {
		return nil, err
	}
	var s Snapshot
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	return &s, nil
}
