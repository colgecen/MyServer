// Package workspace validates file paths against the user's currently selected
// workspace root and blocks path-traversal / symlink-escape attempts.
//
// A Workspace is bound to a single absolute root. All operations on the
// workspace must be expressed as relative paths that resolve *inside* the
// root; anything else is rejected.
package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrNoRoot          = errors.New("workspace: no root configured")
	ErrOutsideRoot     = errors.New("workspace: path escapes workspace root")
	ErrRootInvalid     = errors.New("workspace: root path invalid")
	ErrSymlinkEscape   = errors.New("workspace: symlink escapes workspace root")
	ErrRestrictedPath  = errors.New("workspace: path is restricted by policy")
)

// Workspace is the sandbox for a single selected folder.
type Workspace struct {
	root       string
	restricted []string // absolute path prefixes that are always blocked
}

// New builds a Workspace rooted at root. The root must exist and be a
// directory. Relative paths supplied by callers will be resolved against root.
func New(root string) (*Workspace, error) {
	if strings.TrimSpace(root) == "" {
		return nil, ErrNoRoot
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRootInvalid, err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRootInvalid, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%w: not a directory", ErrRootInvalid)
	}
	return &Workspace{
		root: abs,
		restricted: []string{
			"/etc", "/usr", "/bin", "/sbin", "/var", "/boot", "/root",
		},
	}, nil
}

// Root returns the absolute workspace root.
func (w *Workspace) Root() string { return w.root }

// Resolve maps a caller-supplied path (absolute or relative) to an absolute
// path guaranteed to be inside the workspace root. The returned path is the
// evaluated path (symlinks followed); ErrOutsideRoot / ErrSymlinkEscape are
// returned on policy violations.
func (w *Workspace) Resolve(p string) (string, error) {
	if w == nil {
		return "", ErrNoRoot
	}
	if p == "" {
		return "", errors.New("workspace: empty path")
	}
	var abs string
	if filepath.IsAbs(p) {
		abs = filepath.Clean(p)
	} else {
		abs = filepath.Clean(filepath.Join(w.root, p))
	}

	// Lstat first so we don't follow symlinks for the existence check itself.
	if info, err := os.Lstat(abs); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := filepath.EvalSymlinks(abs)
			if err != nil {
				return "", fmt.Errorf("workspace: resolve symlink: %w", err)
			}
			if !pathHasPrefix(target, w.root) {
				return "", ErrSymlinkEscape
			}
			abs = target
		}
	}

	if !pathHasPrefix(abs, w.root) {
		return "", ErrOutsideRoot
	}
	if w.isRestricted(abs) {
		return "", ErrRestrictedPath
	}
	return abs, nil
}

// IsRestricted returns true if the absolute path falls under any restricted
// policy prefix.
func (w *Workspace) isRestricted(abs string) bool {
	for _, prefix := range w.restricted {
		if pathHasPrefix(abs, prefix) {
			return true
		}
	}
	return false
}

// ValidateCommandPath performs lightweight argument-time inspection: rejects
// any command-line argument that references a restricted system path or
// escapes the workspace root. Returns nil if all args look safe.
//
// This is best-effort and supplements (does not replace) the regex blacklist.
func (w *Workspace) ValidateCommandPath(arg string) error {
	if w == nil {
		return nil
	}
	// strip shell quoting crud
	cleaned := strings.Trim(arg, `"'`)
	if !filepath.IsAbs(cleaned) {
		return nil
	}
	if w.isRestricted(cleaned) {
		return ErrRestrictedPath
	}
	if !pathHasPrefix(cleaned, w.root) {
		return ErrOutsideRoot
	}
	return nil
}

// pathHasPrefix is a stricter alternative to strings.HasPrefix that requires
// the prefix to end on a path-separator boundary (or match exactly).
func pathHasPrefix(p, prefix string) bool {
	if p == prefix {
		return true
	}
	prefix = strings.TrimRight(prefix, string(filepath.Separator)) + string(filepath.Separator)
	return strings.HasPrefix(p, prefix)
}
