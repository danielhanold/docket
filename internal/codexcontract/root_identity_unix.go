//go:build darwin || linux

package codexcontract

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
)

// ObserveRootIdentity reads the feature root without following a final symlink
// and records the canonical per-worktree Git directory.
func ObserveRootIdentity(root, gitDir string) (RootIdentity, error) {
	canonRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return RootIdentity{}, fmt.Errorf("resolve feature root: %w", err)
	}
	if canonRoot != root {
		return RootIdentity{}, fmt.Errorf("feature root is not canonical")
	}
	canonGitDir, err := filepath.EvalSymlinks(gitDir)
	if err != nil {
		return RootIdentity{}, fmt.Errorf("resolve feature git dir: %w", err)
	}
	info, err := os.Lstat(root)
	if err != nil {
		return RootIdentity{}, fmt.Errorf("stat feature root: %w", err)
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return RootIdentity{}, fmt.Errorf("stat feature root: unavailable platform identity")
	}
	return RootIdentity{Platform: runtime.GOOS, Device: uint64(st.Dev), Inode: uint64(st.Ino), GitDir: canonGitDir}, nil
}
