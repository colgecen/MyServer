package workspace_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/anomalyco/myserver/daemon/internal/workspace"
)

func TestResolve_RejectsTraversal(t *testing.T) {
	root := t.TempDir()
	ws, err := workspace.New(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ws.Resolve("../etc/passwd"); err == nil {
		t.Fatal("expected ErrOutsideRoot")
	}
	if _, err := ws.Resolve("/etc/passwd"); err == nil {
		t.Fatal("expected ErrOutsideRoot for absolute outside")
	}
}

func TestResolve_AcceptsInside(t *testing.T) {
	root := t.TempDir()
	ws, err := workspace.New(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ws.Resolve("a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(got) != filepath.Clean(filepath.Join(root, "a.txt")) {
		t.Fatalf("got=%s want=%s", got, filepath.Join(root, "a.txt"))
	}
}

func TestResolve_RejectsRestrictedPrefix(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-only paths")
	}
	root := t.TempDir()
	ws, err := workspace.New(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ws.Resolve("/etc/hosts"); err == nil {
		t.Fatal("expected ErrRestrictedPath or ErrOutsideRoot for /etc")
	}
}

func TestResolve_SymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX symlinks only")
	}
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "leak")); err != nil {
		t.Fatal(err)
	}
	ws, _ := workspace.New(root)
	if _, err := ws.Resolve("leak"); err == nil {
		t.Fatal("expected symlink escape error")
	}
}

func TestNew_RejectsEmptyRoot(t *testing.T) {
	if _, err := workspace.New(""); err == nil {
		t.Fatal("expected error for empty root")
	}
}
