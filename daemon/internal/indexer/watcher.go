// Package indexer — watcher.go
//
// Live index updates via fsnotify: watches the workspace root recursively,
// debounces bursts of events (editors love to fire 5 events per save), and
// re-chunks only the affected files. Events outside the ignore rules are
// dropped before they reach the callback.
package indexer

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	debounceDelay  = 300 * time.Millisecond
	watchEventCap  = 4096 // max pending paths before forced flush
)

// WatchEvent reports an incremental change.
type WatchEvent struct {
	Path   string // absolute
	Action string // "modified" | "created" | "removed" | "renamed"
}

// FileWatcher emits debounced, filtered change events for a workspace.
type FileWatcher struct {
	root    string
	w       *fsnotify.Watcher
	mu      sync.Mutex
	pending map[string]string // path -> action

	onChange func(WatchEvent)
	onError  func(error)

	stopOnce sync.Once
	stopped  chan struct{}
}

// NewFileWatcher creates a watcher for root. onChange is invoked with debounced
// events; onError receives non-fatal watch errors.
func NewFileWatcher(root string, onChange func(WatchEvent), onError func(error)) (*FileWatcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if onChange == nil {
		onChange = func(WatchEvent) {}
	}
	if onError == nil {
		onError = func(error) {}
	}
	fw := &FileWatcher{
		root:     root,
		w:        w,
		pending:  make(map[string]string),
		onChange: onChange,
		onError:  onError,
		stopped:  make(chan struct{}),
	}
	return fw, nil
}

// Start adds recursive watches and begins pumping events. Blocks no caller;
// internal goroutines stop when ctx is cancelled or Close is called.
func (fw *FileWatcher) Start(ctx context.Context) error {
	if err := fw.addRecursive(fw.root); err != nil {
		fw.Close()
		return err
	}

	go fw.loop(ctx)
	go fw.flushLoop(ctx)
	return nil
}

// Close releases all watches. Safe to call multiple times.
func (fw *FileWatcher) Close() {
	fw.stopOnce.Do(func() {
		close(fw.stopped)
		_ = fw.w.Close()
	})
}

func (fw *FileWatcher) addRecursive(root string) error {
	return filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // unreadable entries are not fatal
		}
		if !d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(fw.root, p)
		if rel != "." && fw.skipDir(rel) {
			return filepath.SkipDir
		}
		return fw.w.Add(p)
	})
}

func (fw *FileWatcher) skipDir(rel string) bool {
	for _, ex := range DefaultExcludes {
		if strings.HasPrefix(rel+string(filepath.Separator), ex+string(filepath.Separator)) || rel == ex {
			return true
		}
	}
	return false
}

func (fw *FileWatcher) loop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-fw.stopped:
			return
		case ev, ok := <-fw.w.Events:
			if !ok {
				return
			}
			fw.handle(ev)
		case err, ok := <-fw.w.Errors:
			if !ok {
				return
			}
			if err != nil {
				log.Printf("indexer watcher error: %v", err)
				fw.onError(err)
			}
		}
	}
}

func (fw *FileWatcher) handle(ev fsnotify.Event) {
	rel, err := filepath.Rel(fw.root, ev.Name)
	if err != nil || rel == "." {
		return
	}

	// new directory: start watching its children too
	if ev.Has(fsnotify.Create) {
		if info, err := os.Stat(ev.Name); err == nil && info.IsDir() && !fw.skipDir(rel) {
			_ = fw.addRecursive(ev.Name)
			return
		}
	}

	if fw.skipPath(rel) {
		return
	}

	action := "modified"
	switch {
	case ev.Has(fsnotify.Create):
		action = "created"
	case ev.Has(fsnotify.Remove):
		action = "removed"
	case ev.Has(fsnotify.Rename):
		action = "renamed"
	case ev.Has(fsnotify.Write):
		action = "modified"
	default:
		return
	}

	fw.mu.Lock()
	if len(fw.pending) >= watchEventCap {
		fw.pending = make(map[string]string) // burst guard: drop stale batch
	}
	fw.pending[ev.Name] = action
	fw.mu.Unlock()
}

func (fw *FileWatcher) skipPath(rel string) bool {
	if strings.HasPrefix(filepath.Base(rel), ".") {
		return true
	}
	for _, ex := range DefaultExcludes {
		base := filepath.Base(rel)
		if matched, _ := filepath.Match(ex, base); matched {
			return true
		}
	}
	return false
}

// flushLoop coalesces pending events after each quiet period.
func (fw *FileWatcher) flushLoop(ctx context.Context) {
	ticker := time.NewTicker(debounceDelay)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-fw.stopped:
			return
		case <-ticker.C:
			fw.mu.Lock()
			if len(fw.pending) == 0 {
				fw.mu.Unlock()
				continue
			}
			batch := fw.pending
			fw.pending = make(map[string]string, len(batch))
			fw.mu.Unlock()

			for path, action := range batch {
				fw.onChange(WatchEvent{Path: path, Action: action})
			}
		}
	}
}