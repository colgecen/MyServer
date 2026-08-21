// Package indexer provides language detection from file extension + content.
package indexer

import (
	"bytes"
	"path/filepath"
)

// LanguageMap holds extension -> language mappings.
var LanguageMap = map[string]string{
	// Go
	".go": "go",
	// Python
	".py": "python",
	// Rust
	".rs": "rust",
	// C/C++
	".c":    "c",
	".h":    "c",
	".cpp":  "cpp",
	".cc":   "cpp",
	".cxx":  "cpp",
	".hpp":  "cpp",
	".hxx":  "cpp",
	// Java
	".java": "java",
	// JavaScript/TypeScript
	".js":   "javascript",
	".jsx":  "javascript",
	".ts":   "typescript",
	".tsx":  "typescript",
	".mjs":  "javascript",
	".cjs":  "javascript",
	// Shell
	".sh":   "bash",
	".bash": "bash",
	".zsh":  "zsh",
	".fish": "fish",
	// Web
	".html":    "html",
	".htm":     "html",
	".css":     "css",
	".scss":    "scss",
	".sass":    "sass",
	".less":    "less",
	".vue":     "vue",
	".svelte":  "svelte",
	".astro":   "astro",
	// JSON/YAML/TOML
	".json":  "json",
	".jsonc": "json",
	".yaml":  "yaml",
	".yml":   "yaml",
	".toml":  "toml",
	// Markdown
	".md":   "markdown",
	".mdx":  "markdown",
	// Docker
	"Dockerfile":     "dockerfile",
	".dockerignore": "dockerfile",
	// Config
	".ini":         "ini",
	".cfg":         "ini",
	".conf":        "ini",
	".properties":  "properties",
	// SQL
	".sql": "sql",
	// Ruby
	".rb": "ruby",
	// PHP
	".php": "php",
	// Swift
	".swift": "swift",
	// Kotlin
	".kt":  "kotlin",
	".kts": "kotlin",
	// Scala
	".scala": "scala",
	// Lua
	".lua": "lua",
	// Perl
	".pl": "perl",
	".pm": "perl",
	// R
	".r": "r",
	// Dart
	".dart": "dart",
	// Elixir
	".ex":  "elixir",
	".exs": "elixir",
	// Haskell
	".hs": "haskell",
	// Terraform
	".tf": "terraform",
	// Protobuf
	".proto": "protobuf",
	// GraphQL
	".graphql": "graphql",
	".gql":     "graphql",
}

// ShebangLanguages maps shebang patterns to languages.
var ShebangLanguages = map[string]string{
	"python":  "python",
	"python3": "python",
	"node":    "javascript",
	"bash":    "bash",
	"sh":      "bash",
	"zsh":     "zsh",
	"fish":    "fish",
	"ruby":    "ruby",
	"perl":    "perl",
	"lua":     "lua",
	"php":     "php",
	"nodejs":  "javascript",
	"deno":    "typescript",
	"bun":     "typescript",
}

// DetectLanguage returns the language for a file path and optional content.
// Extension is checked first; if unknown, shebang is inspected.
func DetectLanguage(path string, content []byte) string {
	ext := filepath.Ext(path)
	if lang, ok := LanguageMap[ext]; ok {
		return lang
	}
	// check special filenames (Dockerfile, Makefile, etc.)
	base := filepath.Base(path)
	if lang, ok := LanguageMap[base]; ok {
		return lang
	}
	// shebang
	if len(content) > 2 && content[0] == '#' && content[1] == '!' {
		line := content[:bytes.IndexByte(content, '\n')]
		if len(line) == 0 {
			line = content
		}
		// strip leading #!
		line = bytes.TrimSpace(line[2:])
		parts := bytes.Fields(line)
		if len(parts) >= 1 {
			// /bin/bash -> bash
			// /usr/bin/env python3 -> env (first), python3 (second)
			candidates := []string{}
			if len(parts) >= 2 && filepath.Base(string(parts[0])) == "env" {
				candidates = append(candidates, string(parts[1]))
			}
			candidates = append(candidates, filepath.Base(string(parts[0])))
			for _, c := range candidates {
				if lang, ok := ShebangLanguages[c]; ok {
					return lang
				}
			}
		}
	}
	return "unknown"
}