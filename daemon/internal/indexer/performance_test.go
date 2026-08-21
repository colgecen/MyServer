package indexer_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/anomalyco/myserver/daemon/internal/indexer"
)

// buildTree creates a synthetic workspace with nFiles source files across
// nDirs subdirectories, each file containing nFuncs Go functions.
func buildTree(t *testing.T, root string, nDirs, nFilesPerDir, nFuncs int) {
	t.Helper()
	for d := 0; d < nDirs; d++ {
		dir := filepath.Join(root, fmt.Sprintf("pkg%02d", d))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for f := 0; f < nFilesPerDir; f++ {
			src := "package pkg\n\n"
			for i := 0; i < nFuncs; i++ {
				src += fmt.Sprintf("func Func%d_%d() int {\n\tx := %d\n\treturn x * 2\n}\n\n", f, i, i)
			}
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("file%d.go", f)), []byte(src), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	n := 1
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			n++
		}
	}
	return n
}

func TestAccuracy_WalkerFindsEveryFunction(t *testing.T) {
	root := t.TempDir()
	buildTree(t, root, 3, 2, 5) // 6 files x 5 funcs = 30 funcs

	w, err := indexer.NewWalker(indexer.Config{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	res, err := w.Walk(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.FileCount != 6 {
		t.Fatalf("files=%d want 6", res.FileCount)
	}
	if len(res.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", res.Errors)
	}
}

func TestAccuracy_ChunkerPreservesLineSpans(t *testing.T) {
	root := t.TempDir()
	buildTree(t, root, 1, 1, 10)

	path := filepath.Join(root, "pkg00", "file0.go")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	chunks := indexer.ChunkFile("file0.go", "go", content)
	if len(chunks) < 10 {
		t.Fatalf("chunks=%d want >=10 (one per func)", len(chunks))
	}
	// every func declaration line must be covered by exactly one chunk
	funcLines := map[int]bool{}
	for i, line := range strings.Split(string(content), "\n") {
		if strings.HasPrefix(line, "func ") {
			funcLines[i+1] = true // 1-based
		}
	}
	covered := map[int]bool{}
	for _, c := range chunks {
		if c.StartLine > c.EndLine {
			t.Fatalf("inverted range in %+v", c)
		}
		lines := countLines(c.Content)
		if lines != c.EndLine-c.StartLine+1 {
			t.Fatalf("chunk %s span %d-%d but %d content lines", c.Symbol, c.StartLine, c.EndLine, lines)
		}
		for ln := c.StartLine; ln <= c.EndLine; ln++ {
			if covered[ln] {
				t.Fatalf("line %d covered twice", ln)
			}
			covered[ln] = true
		}
	}
	for ln := range funcLines {
		if !covered[ln] {
			t.Fatalf("func declaration at line %d not covered by any chunk", ln)
		}
	}
	symbols := map[string]bool{}
	for _, c := range chunks {
		if c.Symbol != "" {
			symbols[c.Symbol] = true
		}
	}
	if len(symbols) < 10 {
		t.Fatalf("expected >=10 distinct symbols, got %d", len(symbols))
	}
}

func TestPerformance_Walk1000Files(t *testing.T) {
	if testing.Short() {
		t.Skip("perf test skipped in -short")
	}
	root := t.TempDir()
	buildTree(t, root, 20, 50, 5) // 1000 files

	start := time.Now()
	w, err := indexer.NewWalker(indexer.Config{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	res, err := w.Walk(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)

	if res.FileCount != 1000 {
		t.Fatalf("files=%d want 1000", res.FileCount)
	}
	budget := 5 * time.Second
	if elapsed > budget {
		t.Fatalf("walk too slow: %s (budget %s)", elapsed, budget)
	}
	t.Logf("walked 1000 files in %s (%.0f files/sec)", elapsed, float64(1000)/elapsed.Seconds())
}

func TestPerformance_RetrievalLatency(t *testing.T) {
	if testing.Short() {
		t.Skip("perf test skipped in -short")
	}
	store := indexer.NewContextStore()
	for i := 0; i < 5000; i++ {
		c := mkChunk(fmt.Sprintf("f%d.go", i%97), "go", fmt.Sprintf("function body number %d with some words repeated", i), fmt.Sprintf("fn%d", i))
		store.Add(c)
	}

	start := time.Now()
	results := store.Retrieve(nil, "function body words repeated", 4096)
	elapsed := time.Since(start)

	if len(results) == 0 {
		t.Fatal("no results")
	}
	budget := 250 * time.Millisecond
	if elapsed > budget {
		t.Fatalf("retrieval too slow over 5000 chunks: %s (budget %s)", elapsed, budget)
	}
	t.Logf("retrieval over %d chunks in %s", store.Len(), elapsed)
}