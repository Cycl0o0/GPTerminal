package execution

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Workspace defines the directory tree a command is allowed to touch. Paths
// resolving outside Root (after following symlinks) are workspace escapes
// (INSTRUCTIONS.md §5: symlink evasion must be caught).
type Workspace struct {
	Root string
}

// NewWorkspace builds a Workspace rooted at dir, with symlinks in the root
// itself resolved so later comparisons use canonical paths.
func NewWorkspace(dir string) (*Workspace, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("workspace root: %w", err)
	}
	real, err := canonical(abs)
	if err != nil {
		return nil, fmt.Errorf("resolve workspace root: %w", err)
	}
	return &Workspace{Root: real}, nil
}

// ResolveWithin resolves target (relative to cwd) and verifies the real path,
// after following every symlink, stays inside the workspace. It returns
// ErrWorkspaceEscape otherwise. Non-existent targets are validated by their
// nearest existing ancestor, so creating an escaping path is also blocked.
func (w *Workspace) ResolveWithin(cwd, target string) (string, error) {
	if !filepath.IsAbs(target) {
		target = filepath.Join(cwd, target)
	}
	real, err := canonical(target)
	if err != nil {
		return "", err
	}
	if !w.contains(real) {
		return "", fmt.Errorf("%w: %s resolves to %s, outside %s",
			ErrWorkspaceEscape, target, real, w.Root)
	}
	return real, nil
}

// Contains reports whether an already-resolved path is inside the workspace.
func (w *Workspace) contains(real string) bool {
	if real == w.Root {
		return true
	}
	return strings.HasPrefix(real, w.Root+string(os.PathSeparator))
}

// canonical resolves all symlinks in path. If path does not exist, it resolves
// the longest existing prefix and re-appends the remainder, so a not-yet-created
// file is judged by where it *would* live. This is what defeats symlink-escape
// tricks such as `ln -s /etc/passwd local && cat local`.
func canonical(path string) (string, error) {
	path = filepath.Clean(path)
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved, nil
	}

	// Walk up to the nearest existing ancestor.
	dir := path
	var rest []string
	for {
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root without finding an existing ancestor.
			return path, nil
		}
		rest = append([]string{filepath.Base(dir)}, rest...)
		dir = parent
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			return filepath.Join(append([]string{resolved}, rest...)...), nil
		}
	}
}
