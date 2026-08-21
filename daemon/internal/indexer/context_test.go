package indexer_test

import (
	"strings"
	"testing"

	"github.com/anomalyco/myserver/daemon/internal/indexer"
)

func mkChunk(path, lang, content, symbol string) indexer.IndexedChunk {
	lines := strings.Split(content, "\n")
	return indexer.IndexedChunk{
		Chunk: indexer.Chunk{
			FilePath: path, StartLine: 1, EndLine: len(lines),
			Content: content, Symbol: symbol, Language: lang,
		},
	}
}

func TestContextStore_RetrieveLexical(t *testing.T) {
	store := indexer.NewContextStore()
	store.Add(
		mkChunk("a.go", "go", "func processPayment(amount int) error {\n\treturn nil\n}", "processPayment"),
		mkChunk("b.go", "go", "func renderHTML(w io.Writer, tpl string) {}", "renderHTML"),
		mkChunk("c.py", "python", "def parse_csv(path):\n    pass", "parse_csv"),
	)

	results := store.Retrieve(nil, "how does processPayment work?", 4000)
	if len(results) == 0 {
		t.Fatal("no results")
	}
	if results[0].Chunk.Symbol != "processPayment" {
		t.Fatalf("top hit = %q want processPayment", results[0].Chunk.Symbol)
	}
}

func TestContextStore_RetrieveVector(t *testing.T) {
	store := indexer.NewContextStore()

	relevant := mkChunk("vec.go", "go", "database connection pooling logic", "")
	relevant.Embedding = []float32{1, 0, 0}
	irrelevant := mkChunk("other.go", "go", "ui styling helpers", "")
	irrelevant.Embedding = []float32{0, 1, 0}
	store.Add(relevant, irrelevant)

	results := store.Retrieve([]float32{1, 0, 0}, "", 4000)
	if len(results) != 1 {
		t.Fatalf("len=%d want 1", len(results))
	}
	if results[0].Chunk.FilePath != "vec.go" {
		t.Fatalf("got %s want vec.go", results[0].Chunk.FilePath)
	}
}

func TestRetrieve_RespectsTokenBudget(t *testing.T) {
	store := indexer.NewContextStore()
	big := strings.Repeat("token ", 5000) // ~6250 tokens
	for i := 0; i < 5; i++ {
		store.Add(mkChunk("big.go", "go", big, ""))
	}
	results := store.Retrieve(nil, "token", 1000)
	total := 0
	for _, r := range results {
		total += indexer.EstimateTokens(r.Chunk.Content)
	}
	if total > 1000 {
		t.Fatalf("budget blown: %d > 1000", total)
	}
	if len(results) == 0 {
		t.Fatal("expected at least a truncated chunk")
	}
}

func TestBuildContext_FitsBudget(t *testing.T) {
	store := indexer.NewContextStore()
	content := strings.Repeat("xx ", 266) // ~200 tokens, term "xx" is >= 2 chars
	var chunks []indexer.IndexedChunk
	for i := 0; i < 20; i++ {
		chunks = append(chunks, mkChunk("f.go", "go", content, ""))
	}
	store.Add(chunks...)

	scored := store.Retrieve(nil, "xx", 8192)
	if len(scored) == 0 {
		t.Fatal("no scored chunks")
	}
	rendered := indexer.BuildContext(scored, 8192, 1000)
	if rendered == "" {
		t.Fatal("empty context")
	}
	tokens := indexer.EstimateTokens(rendered)
	if tokens > 8192-1000 {
		t.Fatalf("context too large: %d", tokens)
	}
}

func TestReplaceAndRemoveFile(t *testing.T) {
	store := indexer.NewContextStore()
	c1 := mkChunk("a.go", "go", "old content one", "")
	c2 := mkChunk("a.go", "go", "old content two", "")
	c2.StartLine = c1.EndLine + 1
	store.Add(c1, c2)
	if store.Len() != 2 {
		t.Fatalf("len=%d", store.Len())
	}
	store.ReplaceFile("a.go", mkChunk("a.go", "go", "new content", ""))
	if store.Len() != 1 {
		t.Fatalf("after replace len=%d want 1", store.Len())
	}
	store.RemoveFile("a.go")
	if store.Len() != 0 {
		t.Fatalf("after remove len=%d want 0", store.Len())
	}
}

func TestEstimateTokens(t *testing.T) {
	if got := indexer.EstimateTokens(""); got != 0 {
		t.Fatalf("empty=%d", got)
	}
	if got := indexer.EstimateTokens("abcd"); got != 1 { // 4 chars -> 1 token
		t.Fatalf("4chars=%d want 1", got)
	}
	if got := indexer.EstimateTokens("abcde"); got != 2 { // ceil(5/4)=2
		t.Fatalf("5chars=%d want 2", got)
	}
}