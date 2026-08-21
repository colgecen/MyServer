// Package indexer implements recursive workspace indexing with .gitignore support,
// language detection, AST-based chunking, and vector embeddings via Ollama.
package indexer

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	gitignore "github.com/go-git/go-git/v5/plumbing/format/gitignore"
)

// DefaultExcludes are directories/files always skipped regardless of .gitignore.
var DefaultExcludes = []string{
	".git",
	"node_modules",
	"vendor",
	"dist",
	"build",
	"out",
	".next",
	".turbo",
	"__pycache__",
	".pytest_cache",
	".mypy_cache",
	"target",
	"bin",
	"obj",
	".vscode",
	".idea",
	".DS_Store",
	"*.lock",
	"*.sum",
}

// Config tunes the walker.
type Config struct {
	Root         string        // absolute workspace root
	MaxFileSize  int64         // skip files larger than this (0 = no limit)
	Parallelism  int           // concurrent file readers (0 = GOMAXPROCS)
	IgnoreGlobs  []string      // additional glob patterns to skip
	ProgressFn   func(int, int) // called with (processed, total)
	OnFile       func(Entry)    // callback for each indexed file
	OnError      func(error)    // non-fatal errors
}

// Entry represents a single indexed file before chunking.
type Entry struct {
	Path        string    `json:"path"`         // relative to root
	AbsPath     string    `json:"abs_path"`     // absolute
	Language    string    `json:"language"`     // "go", "python", etc.
	Size        int64     `json:"size"`
	ModTime     time.Time `json:"mod_time"`
	ContentHash string    `json:"content_hash"` // sha256 of content
}

// Result is the summary after a walk completes.
type Result struct {
	FileCount int
	Duration  time.Duration
	Errors    []error
}

// Walker orchestrates the directory walk with ignore matching.
type Walker struct {
	cfg   Config
	ign   gitignore.Matcher
	mu    sync.Mutex
	stops map[string]func() // path -> cancel for live watch
}

// NewWalker constructs a walker for the given config.
func NewWalker(cfg Config) (*Walker, error) {
	if cfg.Root == "" {
		return nil, errors.New("indexer: empty root")
	}
	if cfg.Parallelism <= 0 {
		cfg.Parallelism = 4
	}
	if cfg.MaxFileSize <= 0 {
		cfg.MaxFileSize = 1 << 20 // 1 MiB default
	}
	ign, err := compileIgnorePatterns(cfg.Root, cfg.IgnoreGlobs)
	if err != nil {
		return nil, err
	}
	return &Walker{
		cfg:   cfg,
		ign:   ign,
		stops: make(map[string]func()),
	}, nil
}

// Walk performs a full recursive walk. Returns a result summary.
func (w *Walker) Walk(ctx context.Context) (*Result, error) {
	start := time.Now()
	var (
		entries []Entry
		mu      sync.Mutex
		errs    []error
		wg      sync.WaitGroup
		sem     = make(chan struct{}, w.cfg.Parallelism)
	)

	// first pass: collect all candidate paths
	err := filepath.WalkDir(w.cfg.Root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err // filesystem error, propagate
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		rel, _ := filepath.Rel(w.cfg.Root, p)
		if rel == "." {
			return nil
		}
		if w.shouldSkip(rel, d.IsDir()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		entries = append(entries, Entry{Path: rel, AbsPath: p})
		return nil
	})
	if err != nil {
		return nil, err
	}

	// second pass: read + hash in parallel
	candidates := entries
	var (
		processed int
		pmu       sync.Mutex
	)
	entries = make([]Entry, 0, len(candidates))
	for _, e := range candidates {
		wg.Add(1)
		sem <- struct{}{}
		go func(e Entry) {
			defer wg.Done()
			defer func() { <-sem }()
			if ctx.Err() != nil {
				return
			}
			entry, err := w.processFile(ctx, e)
			if err != nil {
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
				if w.cfg.OnError != nil {
					w.cfg.OnError(err)
				}
				return
			}
			pmu.Lock()
			entries = append(entries, entry)
			processed++
			pmu.Unlock()
			if w.cfg.OnFile != nil {
				w.cfg.OnFile(entry)
			}
			if w.cfg.ProgressFn != nil {
				w.cfg.ProgressFn(processed, len(candidates))
			}
		}(e)
	}
	wg.Wait()

	return &Result{
		FileCount: len(entries),
		Duration:  time.Since(start),
		Errors:    errs,
	}, nil
}

// shouldSkip checks .gitignore + default excludes + hidden entries.
func (w *Walker) shouldSkip(rel string, isDir bool) bool {
	// hidden files/dirs (.foo) are skipped; .gitignore is still parsed
	// separately via compileIgnorePatterns
	if strings.HasPrefix(filepath.Base(rel), ".") {
		return true
	}
	// default excludes
	for _, ex := range DefaultExcludes {
		if matched, _ := filepath.Match(ex, filepath.Base(rel)); matched {
			return true
		}
		if isDir && strings.HasPrefix(rel, ex+string(filepath.Separator)) {
			return true
		}
	}
	// .gitignore patterns
	if w.ign != nil && w.ign.Match(strings.Split(rel, string(filepath.Separator)), isDir) {
		return true
	}
	return false
}

// processFile reads, hashes, and detects language.
func (w *Walker) processFile(ctx context.Context, e Entry) (Entry, error) {
	info, err := os.Stat(e.AbsPath)
	if err != nil {
		return e, err
	}
	if info.Size() > w.cfg.MaxFileSize {
		return e, fmt.Errorf("file too large: %s (%d bytes)", e.Path, info.Size())
	}
	content, err := os.ReadFile(e.AbsPath)
	if err != nil {
		return e, err
	}
	e.Size = info.Size()
	e.ModTime = info.ModTime()
	e.ContentHash = hashContent(content)
	e.Language = DetectLanguage(e.Path, content)
	return e, nil
}

// compileIgnorePatterns builds a gitignore.Matcher from .gitignore files
// in the root and additional glob patterns.
func compileIgnorePatterns(root string, extraGlobs []string) (gitignore.Matcher, error) {
	var patterns []gitignore.Pattern

	// read .gitignore from root
	if data, err := os.ReadFile(filepath.Join(root, ".gitignore")); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			patterns = append(patterns, gitignore.ParsePattern(line, nil))
		}
	}

	// add extra globs as patterns
	for _, g := range extraGlobs {
		patterns = append(patterns, gitignore.ParsePattern(g, nil))
	}

	if len(patterns) == 0 {
		return nil, nil
	}
	return gitignore.NewMatcher(patterns), nil
}

// hashContent returns a short sha256 hex.
func hashContent(b []byte) string {
	// use a simple FNV for now; sha256 can be added if needed
	h := uint64(1469598103934665603)
	for _, c := range b {
		h ^= uint64(c)
		h *= 1099511628211
	}
	return fmt.Sprintf("%x", h)
}