package indexer

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strings"
)

// Chunk is a semantic slice of a source file.
type Chunk struct {
	FilePath  string `json:"file_path"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Content   string `json:"content"`
	Symbol    string `json:"symbol,omitempty"` // function/class name if detected
	Language  string `json:"language"`
}

const (
	defaultChunkLines = 60 // ~500 tokens at ~8 tokens/line
	minChunkLines     = 10
	maxChunkLines     = 200
)

// ChunkFile splits content into semantic chunks. Uses the Go AST parser for
// .go files, declaration-boundary heuristics for other supported languages,
// and a sliding line window as fallback.
func ChunkFile(path, language string, content []byte) []Chunk {
	if len(strings.TrimSpace(string(content))) == 0 {
		return nil
	}
	lines := strings.Split(strings.TrimRight(string(content), "\n"), "\n")
	if len(lines) == 0 {
		return nil
	}

	switch language {
	case "go":
		if chunks := chunkGoAST(path, lines); len(chunks) > 0 {
			return chunks
		}
	case "python", "javascript", "typescript", "rust", "java", "c", "cpp", "ruby", "php", "kotlin", "swift":
		if chunks := chunkDeclBoundaries(path, language, lines); len(chunks) > 0 {
			return chunks
		}
	}
	return chunkByWindow(path, language, lines)
}

// chunkGoAST uses the real Go parser to split top-level declarations.
func chunkGoAST(path string, lines []string) []Chunk {
	fset := token.NewFileSet()
	src := strings.Join(lines, "\n")
	f, err := parser.ParseFile(fset, path, src, parser.SkipObjectResolution)
	if err != nil {
		return nil // fall back to window chunking
	}

	var chunks []Chunk
	emit := func(start, end int, symbol string) {
		if end < start {
			return
		}
		// clamp and pad small gaps to keep context
		start = clamp(start+1, 1, len(lines)) // ast lines are 1-based
		end = clamp(end+1, 1, len(lines))
		chunks = append(chunks, Chunk{
			FilePath:  path,
			StartLine: start,
			EndLine:   end,
			Content:   strings.Join(lines[start-1:end], "\n"),
			Symbol:    symbol,
			Language:  "go",
		})
	}

	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			sym := d.Name.Name
			if d.Recv != nil {
				sym = recvTypeName(d.Recv) + "." + sym
			}
			emit(fset.Position(d.Pos()).Line, fset.Position(d.End()).Line, sym)
		case *ast.GenDecl:
			// group imports together; split types/vars by spec for precision
			if d.Tok == token.IMPORT {
				emit(fset.Position(d.Pos()).Line, fset.Position(d.End()).Line, "imports")
				continue
			}
			for _, spec := range d.Specs {
				gs, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				emit(fset.Position(gs.Pos()).Line, fset.Position(gs.End()).Line, gs.Name.Name)
			}
		}
	}
	return mergeSmallChunks(chunks)
}

var declPatterns = map[string]*regexp.Regexp{
	"python":     regexp.MustCompile(`^(?:async\s+)?def\s+(\w+)|^class\s+(\w+)`),
	"javascript": regexp.MustCompile(`^(?:export\s+)?(?:async\s+)?function\s+(\w+)|^(?:export\s+)?(?:class|const|let)\s+(\w+)`),
	"typescript": regexp.MustCompile(`^(?:export\s+)?(?:async\s+)?function\s+(\w+)|^(?:export\s+)?(?:class|interface|type|enum)\s+(\w+)`),
	"rust":       regexp.MustCompile(`^\s*(?:pub\s+)?(?:async\s+)?fn\s+(\w+)|^\s*(?:pub\s+)?(?:struct|enum|trait|impl|mod)\s+(\w+)`),
	"java":       regexp.MustCompile(`^\s*(?:public|private|protected)?\s*(?:static\s+)?(?:class|interface|void|int|long|String|\w+)\s+(\w+)\s*[\({]`),
	"c":          regexp.MustCompile(`^\w[\w\s\*]*\s+\**(\w+)\s*\([^;]*$`),
	"cpp":        regexp.MustCompile(`^\s*(?:template<[^>]*>\s*)?[\w:<>~\*&\s]+\s+(\w+)::\w*\s*\(|^\s*(?:class|struct|namespace)\s+(\w+)`),
	"ruby":       regexp.MustCompile(`^\s*def\s+(\w+)|^\s*(?:class|module)\s+(\w+)`),
	"php":        regexp.MustCompile(`^\s*(?:public|private|protected)?\s*function\s+(\w+)|^\s*(?:abstract\s+)?class\s+(\w+)`),
	"kotlin":     regexp.MustCompile(`^\s*(?:private|public|internal)?\s*fun\s+(?:[\w.<>]+\.)?(\w+)|^\s*(?:class|object|interface)\s+(\w+)`),
	"swift":      regexp.MustCompile(`^\s*(?:public|private|internal)?\s*func\s+(\w+)|^\s*(?:class|struct|enum|extension|protocol)\s+(\w+)`),
}

// chunkDeclBoundaries splits on top-level declaration keywords using regex.
func chunkDeclBoundaries(path, language string, lines []string) []Chunk {
	re, ok := declPatterns[language]
	if !ok {
		return nil
	}

	var bounds []int // starting line indices of declarations
	names := map[int]string{}
	for i, line := range lines {
		if m := re.FindStringSubmatch(line); m != nil {
			bounds = append(bounds, i)
			for _, g := range m[1:] {
				if g != "" {
					names[i] = g
					break
				}
			}
		}
	}
	if len(bounds) == 0 {
		return nil
	}

	var chunks []Chunk
	for idx, start := range bounds {
		end := len(lines)
		if idx+1 < len(bounds) {
			end = bounds[idx+1]
		}
		if end-start > maxChunkLines {
			// split oversized decl into windows but keep symbol on first piece
			pieces := splitWindow(lines, start, end, defaultChunkLines)
			for pi, p := range pieces {
				sym := ""
				if pi == 0 {
					sym = names[start]
				}
				chunks = append(chunks, Chunk{
					FilePath: path, StartLine: p[0] + 1, EndLine: p[1],
					Content: strings.Join(lines[p[0]:p[1]], "\n"),
					Symbol:  sym, Language: language,
				})
			}
			continue
		}
		chunks = append(chunks, Chunk{
			FilePath: path, StartLine: start + 1, EndLine: end,
			Content: strings.Join(lines[start:end], "\n"),
			Symbol:  names[start], Language: language,
		})
	}
	return mergeSmallChunks(chunks)
}

// chunkByWindow is the universal fallback: fixed-size overlapping windows.
func chunkByWindow(path, language string, lines []string) []Chunk {
	var chunks []Chunk
	for _, r := range splitWindow(lines, 0, len(lines), defaultChunkLines) {
		chunks = append(chunks, Chunk{
			FilePath: path, StartLine: r[0] + 1, EndLine: r[1],
			Content: strings.Join(lines[r[0]:r[1]], "\n"),
			Language: language,
		})
	}
	return chunks
}

// splitWindow returns [start,end) half-open ranges of at most size lines.
func splitWindow(lines []string, start, end, size int) [][2]int {
	var out [][2]int
	for s := start; s < end; s += size {
		e := s + size
		if e > end {
			e = end
		}
		out = append(out, [2]int{s, e})
	}
	return out
}

// mergeSmallChunks glues tiny adjacent chunks (< minChunkLines) together so we
// don't spam the index with fragments. Named declarations are never merged
// away: a chunk with a Symbol only absorbs anonymous neighbours.
func mergeSmallChunks(chunks []Chunk) []Chunk {
	if len(chunks) < 2 {
		return chunks
	}
	var out []Chunk
	cur := chunks[0]
	for _, next := range chunks[1:] {
		adjacent := cur.EndLine-next.StartLine <= 1
		curTiny := len(strings.Split(cur.Content, "\n")) < minChunkLines && cur.Symbol == ""
		nextTiny := len(strings.Split(next.Content, "\n")) < minChunkLines && next.Symbol == ""

		switch {
		case adjacent && curTiny:
			// absorb current into next (next may be named)
			next.StartLine = cur.StartLine
			next.Content = cur.Content + "\n" + next.Content
			if cur.Symbol != "" && next.Symbol == "" {
				next.Symbol = cur.Symbol
			}
			cur = next
		case adjacent && nextTiny && cur.Symbol != "":
			cur.EndLine = next.EndLine
			cur.Content += "\n" + next.Content
		default:
			out = append(out, cur)
			cur = next
		}
	}
	out = append(out, cur)
	return out
}

func recvTypeName(recv *ast.FieldList) string {
	if recv == nil || len(recv.List) == 0 {
		return ""
	}
	switch t := recv.List[0].Type.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			return id.Name
		}
	}
	return fmt.Sprint("recv")
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}