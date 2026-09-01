package app

import (
	"context"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/reposetup"
)

// This file supplies the two concrete reposetup.TestTree read seams the setup
// planners take: init reads the primary worktree on disk, migrate reads the
// pinned source commit's git tree. Keeping the tree behind the interface lets
// PlanInit/PlanMigration stay pure while the app owns the real file access.

// osTree implements reposetup.TestTree over a worktree directory on disk (init's
// tree: the primary worktree at the pinned clean state the rest of init reads).
// Every path is repo-root-relative with `/` separators.
type osTree struct {
	root string
}

// newOSTree returns a TestTree rooted at the given worktree directory.
func newOSTree(root string) reposetup.TestTree { return osTree{root: root} }

// Exists reports whether the repo-relative path exists in the worktree. A
// not-exist result is (false, nil); any other stat fault is a probe error.
func (t osTree) Exists(p string) (bool, error) {
	_, err := os.Stat(filepath.Join(t.root, filepath.FromSlash(p)))
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// ReadFile returns the repo-relative file's bytes, or fs.ErrNotExist when absent
// (os.ReadFile already returns an fs.ErrNotExist-wrapping error for a missing
// file, so the reposetup absence check reads it correctly).
func (t osTree) ReadFile(p string) ([]byte, error) {
	return os.ReadFile(filepath.Join(t.root, filepath.FromSlash(p)))
}

// Glob matches a path.Match-style, repo-root-relative pattern against the
// worktree and returns repo-relative slash paths. filepath.Glob shares
// path.Match's semantics (a `*` never crosses a separator), so the two trees
// agree on what a detector pattern matches.
func (t osTree) Glob(pattern string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(t.root, filepath.FromSlash(pattern)))
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		rel, rerr := filepath.Rel(t.root, m)
		if rerr != nil {
			return nil, rerr
		}
		out = append(out, filepath.ToSlash(rel))
	}
	return out, nil
}

// objectSourceTree implements reposetup.TestTree over a pinned git ObjectSource
// (migrate's tree: the pinned source revision the whole migration decides on, so
// discovery reads exactly the copy the preview and the commit act on). The whole
// tree listing is fetched once and cached for Glob.
type objectSourceTree struct {
	ctx    context.Context
	src    gitcli.ObjectSource
	leaves []string // repo-relative slash paths of every blob leaf; nil until loaded
	loaded bool
}

// newObjectSourceTree returns a TestTree reading the pinned commit src.
func newObjectSourceTree(ctx context.Context, src gitcli.ObjectSource) *objectSourceTree {
	return &objectSourceTree{ctx: ctx, src: src}
}

// Exists reports whether a blob exists at the repo-relative path in the pinned
// tree. A probe fault surfaces as an error, never a false absence.
func (t *objectSourceTree) Exists(p string) (bool, error) {
	results, err := t.src.ReadBlobs(t.ctx, []gitcli.RepoPath{gitcli.RepoPath(p)})
	if err != nil {
		return false, err
	}
	return len(results) == 1 && results[0].Found, nil
}

// ReadFile returns the blob bytes at the repo-relative path, or fs.ErrNotExist
// when the pinned tree has no such blob.
func (t *objectSourceTree) ReadFile(p string) ([]byte, error) {
	results, err := t.src.ReadBlobs(t.ctx, []gitcli.RepoPath{gitcli.RepoPath(p)})
	if err != nil {
		return nil, err
	}
	if len(results) != 1 || !results[0].Found {
		return nil, fs.ErrNotExist
	}
	return results[0].Blob.Bytes, nil
}

// Glob matches a path.Match-style pattern against every blob leaf in the pinned
// tree. The full listing is loaded once and cached.
func (t *objectSourceTree) Glob(pattern string) ([]string, error) {
	if !t.loaded {
		entries, err := t.src.ListTree(t.ctx, nil)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			t.leaves = append(t.leaves, string(e.Path))
		}
		t.loaded = true
	}
	var out []string
	for _, leaf := range t.leaves {
		ok, err := path.Match(pattern, leaf)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, leaf)
		}
	}
	return out, nil
}
