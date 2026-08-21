// Package indexer — context.go
//
// Context window management: token budgeting, top-K chunk retrieval with
// vector similarity (lexical fallback), and sliding-window packing so the
// assembled context always fits the LLM's context limit.
package indexer

import (
	"sort"
	"strings"
)

const (
	charsPerToken      = 4    // rough heuristic for code+prose
	defaultBudgetTok   = 4096 // tokens reserved for injected context
	minReservedTokens  = 512  // never spend more than budget-reserve on prompt
)

// IndexedChunk is a chunk enriched with its embedding vector.
type IndexedChunk struct {
	Chunk
	Embedding []float32 `json:"embedding,omitempty"`
}

// ContextStore holds the searchable index for one workspace.
type ContextStore struct {
	chunks []IndexedChunk
}

// NewContextStore creates an empty store.
func NewContextStore() *ContextStore { return &ContextStore{} }

// Add appends chunks (deduped per file path + line range).
func (s *ContextStore) Add(chunks ...IndexedChunk) {
	for _, c := range chunks {
		if s.exists(c.FilePath, c.StartLine, c.EndLine) {
			continue
		}
		s.chunks = append(s.chunks, c)
	}
}

// ReplaceFile drops all chunks of a file then adds the new ones (watcher path).
func (s *ContextStore) ReplaceFile(path string, chunks ...IndexedChunk) {
	kept := s.chunks[:0]
	for _, c := range s.chunks {
		if c.FilePath != path {
			kept = append(kept, c)
		}
	}
	s.chunks = kept
	s.Add(chunks...)
}

// RemoveFile drops all chunks belonging to a deleted file.
func (s *ContextStore) RemoveFile(path string) {
	kept := s.chunks[:0]
	for _, c := range s.chunks {
		if c.FilePath != path {
			kept = append(kept, c)
		}
	}
	s.chunks = kept
}

// Len returns the number of indexed chunks.
func (s *ContextStore) Len() int { return len(s.chunks) }

func (s *ContextStore) exists(path string, start, end int) bool {
	for _, c := range s.chunks {
		if c.FilePath == path && c.StartLine == start && c.EndLine == end {
			return true
		}
	}
	return false
}

// Scored pairs a chunk with its relevance score.
type Scored struct {
	Chunk IndexedChunk
	Score float64
}

// Retrieve selects the most relevant chunks for the query within the token
// budget. Uses cosine similarity when both query and chunk have embeddings,
// falling back to lexical overlap otherwise. Results are ordered by score
// descending; ties break toward earlier file paths for determinism.
func (s *ContextStore) Retrieve(queryEmbedding []float32, queryText string, budgetTokens int) []Scored {
	if budgetTokens <= 0 {
		budgetTokens = defaultBudgetTok
	}

	scored := make([]Scored, 0, len(s.chunks))
	qTerms := termSet(queryText)
	for _, c := range s.chunks {
		score := 0.0
		switch {
		case len(queryEmbedding) > 0 && len(c.Embedding) == len(queryEmbedding):
			if sim, err := CosineSimilarity(queryEmbedding, c.Embedding); err == nil {
				score = sim
			}
		case len(qTerms) > 0:
			score = lexicalOverlap(qTerms, c.Content)
		default:
			continue // no signal at all
		}
		if score > 0 {
			scored = append(scored, Scored{Chunk: c, Score: score})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].Score != scored[j].Score {
			return scored[i].Score > scored[j].Score
		}
		return scored[i].Chunk.FilePath < scored[j].Chunk.FilePath
	})

	return packByBudget(scored, budgetTokens)
}

// packByBudget greedily fills the budget from highest-scored chunks.
// Oversized single chunks are truncated to whatever budget remains.
func packByBudget(scored []Scored, budgetTokens int) []Scored {
	var out []Scored
	remaining := budgetTokens
	for _, sc := range scored {
		if remaining <= 0 {
			break
		}
		toks := EstimateTokens(sc.Chunk.Content)
		if toks <= remaining {
			out = append(out, sc)
			remaining -= toks
			continue
		}
		// truncate chunk content to fit remaining budget
		trunc := truncateToTokens(sc.Chunk.Content, remaining-truncationNoticeTokens)
		if trunc != "" {
			sc.Chunk.Content = trunc + truncationNotice
			sc.Chunk.EndLine = sc.Chunk.StartLine + strings.Count(trunc, "\n")
			out = append(out, sc)
		}
		remaining = 0
	}
	return out
}

const truncationNotice = "\n[...chunk truncated to fit context budget...]"

var truncationNoticeTokens = EstimateTokens(truncationNotice)

// EstimateTokens approximates token count (~4 chars/token).
func EstimateTokens(text string) int {
	return (len(text) + charsPerToken - 1) / charsPerToken
}

// truncateToTokens cuts text at an approximate token boundary (word edge).
func truncateToTokens(text string, maxTokens int) string {
	if maxTokens <= 0 {
		return ""
	}
	maxChars := maxTokens * charsPerToken
	if len(text) <= maxChars {
		return text
	}
	cut := text[:maxChars]
	// back off to last whitespace to avoid splitting words/tokens mid-way
	if idx := strings.LastIndexAny(cut, " \t\n"); idx > maxChars/2 {
		cut = cut[:idx]
	}
	return cut
}

// BuildContext renders retrieved chunks into a single markdown block ready
// for LLM injection, respecting the reserved budget for the user prompt.
func BuildContext(scored []Scored, totalBudget, promptTokens int) string {
	reserved := promptTokens + minReservedTokens
	if reserved >= totalBudget {
		reserved = totalBudget * 3 / 4
	}
	budget := totalBudget - reserved
	if budget <= 0 {
		return ""
	}

	var sb strings.Builder
	used := 0
	for _, sc := range scored {
		hdr := header(sc.Chunk)
		toks := EstimateTokens(hdr + "\n" + sc.Chunk.Content)
		if used+toks > budget {
			break
		}
		sb.WriteString(hdr)
		sb.WriteString("\n")
		sb.WriteString(sc.Chunk.Content)
		sb.WriteString("\n\n")
		used += toks
	}
	return sb.String()
}

func header(c IndexedChunk) string {
	loc := c.FilePath
	if c.StartLine > 0 {
		loc += ":" + itoa(c.StartLine) + "-" + itoa(c.EndLine)
	}
	if c.Symbol != "" {
		loc += " (" + c.Symbol + ")"
	}
	return "// [" + c.Language + "] " + loc
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// termSet lowercases and splits into word terms (lexical fallback).
func termSet(text string) map[string]struct{} {
	fields := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_')
	})
	set := make(map[string]struct{}, len(fields))
	for _, f := range fields {
		if len(f) >= 2 { // skip 1-char noise
			set[f] = struct{}{}
		}
	}
	return set
}

// lexicalOverlap scores |queryTerms ∩ docTerms| / |queryTerms|.
func lexicalOverlap(qTerms map[string]struct{}, doc string) float64 {
	dTerms := termSet(doc)
	hits := 0
	for t := range qTerms {
		if _, ok := dTerms[t]; ok {
			hits++
		}
	}
	if hits == 0 {
		return 0
	}
	return float64(hits) / float64(len(qTerms))
}