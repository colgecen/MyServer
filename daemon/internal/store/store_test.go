package store_test

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/anomalyco/myserver/daemon/internal/store"
)

func TestKV_CrashRecoveryAndConcurrency(t *testing.T) {
	dir := t.TempDir()
	kv, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	// concurrent writes
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			_ = kv.Put(store.BucketCache, string(rune('a'+n)), []byte("v"))
		}(i)
	}
	wg.Wait()
	if _, err := kv.Get(store.BucketCache, "a"); err != nil {
		t.Fatalf("get after concurrent: %v", err)
	}
	kv.Close()
	// reopen => recovery
	kv2, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer kv2.Close()
	if _, err := kv2.Get(store.BucketCache, "a"); err != nil {
		t.Fatalf("recovery failed: %v", err)
	}
	// session roundtrip
	s := store.ChatSession{ID: "s1", Workspace: "/tmp/ws", Messages: []string{"hi"}}
	if err := kv2.SaveSession(s); err != nil {
		t.Fatal(err)
	}
	if _, err := kv2.LoadSession("s1"); err != nil {
		t.Fatal(err)
	}
}
