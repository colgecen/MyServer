package guardrail

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	bolt "go.etcd.io/bbolt"
)

var auditBucket = []byte("audit")

// AuditLogEntry is persisted for every command decision.
type AuditLogEntry struct {
	ID             string    `json:"id"`
	Timestamp      time.Time `json:"timestamp"`
	Command        string    `json:"command"`
	Classification string    `json:"classification"` // READ_ONLY | WORKSPACE_WRITE | FULL_SYSTEM_EXEC
	Denied         bool      `json:"denied"`
	ApprovedBy     string    `json:"approved_by"` // "auto" | "user" | "denied"
	Status         string    `json:"status"`      // executed | denied | dry_run
	User           string    `json:"user"`
	ExitCode       *int      `json:"exit_code,omitempty"`
	DurationMS     int64     `json:"duration_ms"`
}

// AuditStore is a bbolt-backed append/query log.
type AuditStore struct {
	db   *bolt.DB
	mu   sync.Mutex
	path string
}

// OpenAudit opens (creating if needed) the audit database at path.
func OpenAudit(path string) (*AuditStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("audit dir: %w", err)
	}
	db, err := bolt.Open(path, 0o600, &bolt.Options{Timeout: 2 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("open audit db: %w", err)
	}
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(auditBucket)
		return err
	})
	if err != nil {
		db.Close()
		return nil, err
	}
	user := os.Getenv("USER")
	if user == "" {
		user = "unknown"
	}
	return &AuditStore{db: db, path: path}, nil
}

// Append persists one entry.
func (a *AuditStore) Append(entry AuditLogEntry) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return a.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(auditBucket)
		return b.Put([]byte(entry.ID), data)
	})
}

// Recent returns up to limit newest entries.
func (a *AuditStore) Recent(limit int) ([]AuditLogEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	var out []AuditLogEntry
	err := a.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(auditBucket).Cursor()
		for k, v := c.Last(); k != nil && len(out) < limit; k, v = c.Prev() {
			var e AuditLogEntry
			if json.Unmarshal(v, &e) == nil {
				out = append(out, e)
			}
		}
		return nil
	})
	return out, err
}

// Close releases the db file lock.
func (a *AuditStore) Close() error { return a.db.Close() }

func newAuditID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("audit-%d", time.Now().UnixNano())
	}
	return "audit-" + hex.EncodeToString(b[:])
}