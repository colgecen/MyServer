package indexer_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/anomalyco/myserver/daemon/internal/indexer"
)

func TestWalker_SkipsGitignore(t *testing.T) {
	root := t.TempDir()
	// create .gitignore
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("*.log\nignored/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// create files
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "debug.log"), []byte("log"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "ignored"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ignored", "secret.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	w, err := indexer.NewWalker(indexer.Config{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	res, err := w.Walk(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.FileCount != 1 {
		t.Fatalf("expected 1 file, got %d", res.FileCount)
	}
}

func TestWalker_SkipsDefaultExcludes(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "node_modules", "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "node_modules", "pkg", "index.js"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git", "config"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	w, err := indexer.NewWalker(indexer.Config{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	res, err := w.Walk(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if res.FileCount != 1 {
		t.Fatalf("expected 1 file, got %d", res.FileCount)
	}
}

func TestWalker_LanguageDetection(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"main.go":      "go",
		"script.py":    "python",
		"lib.rs":       "rust",
		"app.js":       "javascript",
		"types.ts":     "typescript",
		"style.css":    "css",
		"Dockerfile":   "dockerfile",
		"README.md":    "markdown",
		"config.yaml":  "yaml",
		"data.json":    "json",
		"build.sh":     "bash",
		"Makefile":     "unknown", // no entry
		"unknown.xyz":  "unknown",
	}
	for name := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	w, err := indexer.NewWalker(indexer.Config{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	_, err = w.Walk(context.Background())
	if err != nil {
		t.Fatal(err)
	}
}

func TestDetectLanguage_ByExtension(t *testing.T) {
	cases := []struct {
		name, lang string
	}{
		{"foo.go", "go"},
		{"bar.py", "python"},
		{"baz.rs", "rust"},
		{"app.js", "javascript"},
		{"types.ts", "typescript"},
		{"style.css", "css"},
		{"Dockerfile", "dockerfile"},
		{"README.md", "markdown"},
		{"config.yaml", "yaml"},
		{"data.json", "json"},
		{"script.sh", "bash"},
		{"unknown.xyz", "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := indexer.DetectLanguage(tc.name, nil); got != tc.lang {
				t.Fatalf("DetectLanguage(%q) = %q, want %q", tc.name, got, tc.lang)
			}
		})
	}
}

func TestDetectLanguage_Shebang(t *testing.T) {
	cases := []struct {
		content string
		lang    string
	}{
		{"#!/usr/bin/env python3\nprint('hi')", "python"},
		{"#!/bin/bash\necho hi", "bash"},
		{"#!/usr/bin/env node\nconsole.log('hi')", "javascript"},
		{"#!/usr/bin/env zsh\necho hi", "zsh"},
		{"#!/usr/bin/env fish\necho hi", "fish"},
		{"#!/usr/bin/env ruby\nputs 'hi'", "ruby"},
		{"#!/usr/bin/env perl\nprint 'hi'", "perl"},
		{"#!/usr/bin/env lua\nprint('hi')", "lua"},
		{"#!/usr/bin/env php\n<?php echo 'hi';", "php"},
		{"#!/usr/bin/env deno\nconsole.log('hi')", "typescript"},
		{"#!/usr/bin/env bun\nconsole.log('hi')", "typescript"},
	}
	for i, tc := range cases {
		t.Run(string(rune('a'+i)), func(t *testing.T) {
			if got := indexer.DetectLanguage("script", []byte(tc.content)); got != tc.lang {
				t.Fatalf("DetectLanguage(shebang) = %q, want %q", got, tc.lang)
			}
		})
	}
}