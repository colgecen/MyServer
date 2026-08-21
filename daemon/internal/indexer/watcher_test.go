package indexer_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/anomalyco/myserver/daemon/internal/indexer"
)

func waitForEvents(t *testing.T, ch <-chan indexer.WatchEvent, want int, timeout time.Duration) []indexer.WatchEvent {
	t.Helper()
	var evs []indexer.WatchEvent
	deadline := time.After(timeout)
	for len(evs) < want {
		select {
		case ev := <-ch:
			evs = append(evs, ev)
		case <-deadline:
			return evs
		}
	}
	return evs
}

func TestFileWatcher_DebouncesAndFilters(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "node_modules"), 0o755); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var events []indexer.WatchEvent
	fw, err := indexer.NewFileWatcher(root, func(ev indexer.WatchEvent) {
		mu.Lock()
		events = append(events, ev)
		mu.Unlock()
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer fw.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := fw.Start(ctx); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond) // let watches settle

	// burst of writes to one file -> should coalesce into 1 event
	target := filepath.Join(root, "app.go")
	for i := 0; i < 5; i++ {
		if err := os.WriteFile(target, []byte("package main"), 0o644); err != nil {
			t.Fatal(err)
		}
		time.Sleep(20 * time.Millisecond)
	}

	evs := waitForEvents(t, eventChan(&mu, &events), 1, 3*time.Second)
	found := false
	for _, ev := range evs {
		if filepath.Base(ev.Path) == "app.go" {
			found = true
		}
		if filepath.Base(filepath.Dir(ev.Path)) == "node_modules" {
			t.Fatalf("node_modules event leaked: %+v", ev)
		}
	}
	if !found {
		t.Fatalf("no app.go event among %d", len(events))
	}
}

// eventChan returns a receive-only view for the helper.
func eventChan(mu *sync.Mutex, events *[]indexer.WatchEvent) <-chan indexer.WatchEvent {
	ch := make(chan indexer.WatchEvent)
	go func() {
		for {
			time.Sleep(30 * time.Millisecond)
			mu.Lock()
			if len(*events) > 0 {
				ev := (*events)[0]
				*events = (*events)[1:]
				mu.Unlock()
				ch <- ev
				continue
			}
			mu.Unlock()
		}
	}()
	return ch
}