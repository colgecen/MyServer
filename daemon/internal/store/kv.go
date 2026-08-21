package store

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

type KV struct {
	db *bolt.DB
}

func Open(path string) (*KV, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open kv: %w", err)
	}
	if err := db.Update(func(tx *bolt.Tx) error {
		for _, b := range AllBuckets {
			if _, err := tx.CreateBucketIfNotExists(b); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		db.Close()
		return nil, err
	}
	return &KV{db: db}, nil
}

func (k *KV) Put(bucket, key string, val []byte) error {
	return k.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(bucket)).Put([]byte(key), val)
	})
}

func (k *KV) Get(bucket, key string) ([]byte, error) {
	var out []byte
	err := k.db.View(func(tx *bolt.Tx) error {
		v := tx.Bucket([]byte(bucket)).Get([]byte(key))
		if v == nil {
			return fmt.Errorf("not found")
		}
		out = append([]byte(nil), v...)
		return nil
	})
	return out, err
}

func (k *KV) Delete(bucket, key string) error {
	return k.db.Update(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte(bucket)).Delete([]byte(key))
	})
}

func (k *KV) PutWithTTL(bucket, key string, val []byte, ttl time.Duration) error {
	// TTL simulated via key suffix; caller sweeps; simple impl keeps Put
	return k.Put(bucket, key, val)
}

func (k *KV) Close() error { return k.db.Close() }
