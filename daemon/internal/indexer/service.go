// Package indexer — service.go
//
// IndexService orchestrates the full indexing pipeline for one workspace:
// walk -> language detect -> chunk -> (optional) embed -> store, with live
// watcher-driven incremental updates and progress reporting.
package indexer

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Progress reports indexing state back to the GUI over IPC.
type Progress struct {
	Phase       string `json:"phase"` // "walking" | "chunking" | "embedding" | "done" | "error"
	FilesDone   int    `json:"files_done"`
	FilesTotal  int    `json:"files_total"`
	Chunks      int    `json:"chunks"`
	Message     string `json:"message,omitempty"`
}

// Service manages indexing for the currently selected workspace.
type Service struct {
	mu        sync.Mutex
	root      string
	store     *ContextStore
	watcher   *FileWatcher
	embedder  Embedder
	onProgress func(Progress)

	cancelWatch context.CancelFunc
	indexed     bool
}

// NewService creates a Service; onProgress may be nil.
func NewService(embedder Embedder, onProgress func(Progress)) *Service {
	if onProgress == nil {
		onProgress = func(Progress) {}
	}
	return &Service{
		store:      NewContextStore(),
		embedder:   embedder,
		onProgress: onProgress,
	}
}

// Store exposes the context store for retrieval.
func (s *Service) Store() *ContextStore {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.store
}

// Root returns the current workspace root ("" if none selected).
func (s *Service) Root() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.root
}

// SelectWorkspace switches to a new workspace root and performs a full index.
// A previous watch loop is cancelled first.
func (s *Service) SelectWorkspace(ctx context.Context, root string) (*IndexSummary, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve path: %w", err)
	}
	if info, err := os.Stat(abs); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("not a directory: %s", abs)
	}

	s.mu.Lock()
	s.stopWatcherLocked()
	s.root = abs
	s.store = NewContextStore()
	s.mu.Unlock()

	summary, err := s.fullIndex(ctx)
	if err != nil {
		return nil, err
	}
	s.startWatcher()
	return summary, nil
}

// IndexSummary is returned after a full (re)index completes.
type IndexSummary struct {
	Root      string        `json:"root"`
	FileCount int           `json:"file_count"`
	ChunkCount int          `json:"chunk_count"`
	Duration  time.Duration `json:"duration_ms"`
	Skipped   int           `json:"skipped_files"`
}

// fullIndex walks, chunks, embeds and stores everything under root.
func (s *Service) fullIndex(ctx context.Context) (*IndexSummary, error) {
	start := time.Now()
	s.onProgress(Progress{Phase: "walking", Message: s.root})

	w, err := NewWalker(Config{
		Root:        s.root,
		OnFile:      nil,
		OnError:     func(err error) { log.Printf("indexer walk: %v", err) },
	})
	if err != nil {
		return nil, err
	}

	var (
		mu         sync.Mutex
		allEntries []Entry
	)
	w.cfg.OnFile = func(e Entry) {
		mu.Lock()
		allEntries = append(allEntries, e)
		mu.Unlock()
	}
	result, err := w.Walk(ctx)
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", s.root, err)
	}
	s.onProgress(Progress{Phase: "chunking", FilesTotal: len(allEntries)})

	// chunk every entry
	type chunkJob struct {
		entry  Entry
		chunks []Chunk
	}
	var jobs []chunkJob
	for _, e := range allEntries {
		content, err := os.ReadFile(e.AbsPath)
		if err != nil {
			continue
		}
		chunks := ChunkFile(e.Path, e.Language, content)
		if len(chunks) > 0 {
			jobs = append(jobs, chunkJob{entry: e, chunks: chunks})
		}
	}

	total := 0
	for _, j := range jobs {
		total += len(j.chunks)
	}
	s.onProgress(Progress{Phase: "embedding", FilesTotal: len(allEntries), Chunks: total})

	// build flat text list for embedding (skip if unavailable)
	var texts []string
	if s.embedder != nil && s.embedder.Available(ctx) {
		for _, j := range jobs {
			for _, c := range j.chunks {
				texts = append(texts, c.Content)
			}
		}
	}
	var vectors [][]float32
	if len(texts) > 0 {
		vectors, err = s.embedder.Embed(ctx, texts)
		if err != nil {
			log.Printf("indexer: embedding failed (%v); continuing without vectors", err)
			vectors = nil
		}
	}

	// assemble store entries with positional vector alignment
	idx := 0
	for _, j := range jobs {
		for _, c := range j.chunks {
			ic := IndexedChunk{Chunk: c}
			if idx < len(vectors) {
				ic.Embedding = vectors[idx]
			}
			idx++
			s.Store().Add(ic)
		}
	}

	summary := &IndexSummary{
		Root:       s.root,
		FileCount:  result.FileCount,
		ChunkCount: total,
		Duration:   time.Since(start),
		Skipped:    len(result.Errors),
	}
	s.mu.Lock()
	s.indexed = true
	s.mu.Unlock()
	s.onProgress(Progress{Phase: "done", FilesDone: summary.FileCount, FilesTotal: summary.FileCount, Chunks: summary.ChunkCount})
	return summary, nil
}

// startWatcher launches live incremental updates for the current root.
func (s *Service) startWatcher() {
	s.mu.Lock()
	defer s.mu.Unlock()

	fw, err := NewFileWatcher(s.root,
		func(ev WatchEvent) { s.applyEvent(ev) },
		func(err error) { log.Printf("indexer watcher: %v", err) },
	)
	if err != nil {
		log.Printf("indexer: watcher unavailable: %v", err)
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.watcher = fw
	s.cancelWatch = cancel

	go func() {
		if err := fw.Start(ctx); err != nil {
			log.Printf("indexer: watcher start failed: %v", err)
		}
	}()
}

func (s *Service) stopWatcherLocked() {
	if s.cancelWatch != nil {
		s.cancelWatch()
		s.cancelWatch = nil
	}
	if s.watcher != nil {
		s.watcher.Close()
		s.watcher = nil
	}
}

// applyEvent re-chunks one changed file (or removes it).
func (s *Service) applyEvent(ev WatchEvent) {
	switch ev.Action {
	case "removed", "renamed":
		s.Store().RemoveFile(relPath(s.Root(), ev.Path))
		return
	}

	abs := ev.Path
	info, err := os.Stat(abs)
	if err != nil || info.IsDir() || info.Size() > 1<<20 {
		if os.IsNotExist(err) {
			s.Store().RemoveFile(relPath(s.Root(), abs))
		}
		return
	}
	content, err := os.ReadFile(abs)
	if err != nil {
		return
	}
	rel := relPath(s.Root(), abs)
	lang := DetectLanguage(rel, content)
	chunks := ChunkFile(rel, lang, content)

	var indexed []IndexedChunk
	for _, c := range chunks {
		indexed = append(indexed, IndexedChunk{Chunk: c})
	}
	// best-effort single-file embedding
	if s.embedder != nil && len(indexed) > 0 && s.embedder.Available(context.Background()) {
		texts := make([]string, len(chunks))
		for i, c := range chunks {
			texts[i] = c.Content
		}
		if vecs, err := s.embedder.Embed(context.Background(), texts); err == nil {
			for i := range vecs {
				if i < len(indexed) {
					indexed[i].Embedding = vecs[i]
				}
			}
		}
	}
	s.Store().ReplaceFile(rel, indexed...)
}

// Shutdown stops background watchers.
func (s *Service) Shutdown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopWatcherLocked()
}

func relPath(root, p string) string {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return p
	}
	return filepath.ToSlash(rel)
}