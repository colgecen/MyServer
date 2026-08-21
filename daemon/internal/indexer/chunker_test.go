package indexer_test

import (
	"strings"
	"testing"

	"github.com/anomalyco/myserver/daemon/internal/indexer"
)

const goSample = `package main

import "fmt"

type Config struct {
	Name string
}

func (c *Config) Greet() string {
	return "hello " + c.Name
}

func helper(n int) int {
	total := 0
	for i := 0; i < n; i++ {
		total += i
	}
	return total
}

func main() {
	c := Config{Name: "x"}
	fmt.Println(c.Greet(), helper(10))
}
`

func TestChunkFile_GoAST(t *testing.T) {
	chunks := indexer.ChunkFile("main.go", "go", []byte(goSample))
	if len(chunks) == 0 {
		t.Fatal("no chunks produced")
	}

	symbols := map[string]bool{}
	for _, ch := range chunks {
		symbols[ch.Symbol] = true
	}
	for _, want := range []string{"imports", "Config", "Config.Greet", "helper", "main"} {
		if !symbols[want] {
			t.Fatalf("missing symbol %q in %v", want, symbols)
		}
	}
	// verify content lines match declared ranges
	for _, ch := range chunks {
		lines := strings.Split(ch.Content, "\n")
		if got := ch.EndLine - ch.StartLine + 1; got != len(lines) {
			t.Fatalf("chunk %s: line span=%d content lines=%d", ch.Symbol, got, len(lines))
		}
	}
}

func TestChunkFile_PythonDeclBoundaries(t *testing.T) {
	src := `import os

class Alpha:
    def run(self):
        return 1

def beta(x):
    return x * 2

async def gamma():
    pass
`
	chunks := indexer.ChunkFile("app.py", "python", []byte(src))
	if len(chunks) < 2 {
		t.Fatalf("expected >=2 chunks, got %d: %+v", len(chunks), chunks)
	}
	found := map[string]bool{}
	for _, ch := range chunks {
		found[ch.Symbol] = true
	}
	for _, want := range []string{"Alpha", "beta"} {
		if !found[want] {
			t.Fatalf("missing %q in %v", want, found)
		}
	}
}

func TestChunkFile_FallbackWindow(t *testing.T) {
	// unknown language -> window chunking
	var sb strings.Builder
	for i := 0; i < 150; i++ {
		sb.WriteString("line\n")
	}
	chunks := indexer.ChunkFile("data.xyz", "unknown", []byte(sb.String()))
	if len(chunks) < 2 {
		t.Fatalf("expected multiple windows, got %d", len(chunks))
	}
	if chunks[0].StartLine != 1 || chunks[0].EndLine < 50 {
		t.Fatalf("unexpected first chunk range: %+v", chunks[0])
	}
}

func TestChunkFile_Empty(t *testing.T) {
	if chunks := indexer.ChunkFile("empty.go", "go", nil); len(chunks) != 0 {
		t.Fatalf("expected no chunks, got %d", len(chunks))
	}
}