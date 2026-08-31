// Package repoguard provides a shared, fail-closed walk of the repository's
// MAINTAINED source surface, plus the categorical exclusions that keep immutable
// history and frozen fixture corpora out of every repo-wide guard.
//
// It is the Go home for the repo-wide shape guards that used to live as Bash
// tests under tests/ (change 0370, Gate 2). Every guard in this package scans
// the population MaintainedFiles / ExecutableSurface returns, so the exclusion
// policy is written ONCE, here, and cannot drift between guards.
//
// # Categorical exclusions (by location/ownership, never a per-file allowlist)
//
// The exclusions are structural — a directory is in or out by WHERE it lives and
// WHO owns it, never by an enumerated filename (ADR-0050, enumerated-floor). The
// excluded corpora are immutable history and frozen fixture DATA that a repo-wide
// scan must tolerate, not police:
//
//   - any directory named "testdata": Go reserves this name for non-compiled
//     fixture DATA. This single rule covers internal/repository/testdata (the
//     frozen v0.9.x recorded corpus), the top-level testdata/ release-versioned
//     repository corpora (testdata/repositories/v0.9.2..v0.9.6, PROVENANCE.md),
//     internal/install/testdata/legacy (adopted pre-migration input), and every
//     package's local fixture tree.
//   - internal/install/legacydata: frozen recorded pre-migration dispatch blocks
//     docket ADOPTS — legacy input DATA, not a maintained caller. Not named
//     "testdata", so it needs its own categorical entry.
//   - docs: immutable point-in-time history (changes, specs, plans, results, and
//     Accepted ADRs) — the convention forbids rewriting it, so a guard cannot
//     demand a repair there (frozen-fixture-corpus-trips-repo-wide-scans).
//   - tests/fixtures: crafted fixtures for the shell suite, not maintained source.
//   - .git, .worktrees: version-control internals and sibling checkouts.
//
// # Fail-closed
//
// A walk that cannot read a directory returns an error rather than a short,
// silently-incomplete list — a partial traversal is not absence evidence
// (probe-error-is-not-clean-absence). Consumers that scan file CONTENT are
// expected to fail closed on a read error the same way (an unreadable file in the
// scanned surface is an error, not a clean miss).
//
// # Population is the filesystem, not the git index
//
// MaintainedFiles walks the on-disk tree under root, so it also sees files that
// are present but not yet tracked by git. That is deliberate: the guards run at
// the build gate over a checkout, and a facade line planted in an untracked file
// is still a live dependency of that checkout. The tradeoff is that a genuinely
// transient scratch file in a dirty worktree is in-population until removed.
package repoguard

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Root walks up from the current working directory to the module root (the
// nearest ancestor holding go.mod) and returns it. Every guard in this package
// resolves the repo root through Root so the scan population is identical no
// matter which package directory `go test` runs from.
func Root() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("repoguard: getwd: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("repoguard: no go.mod found walking up from working directory")
		}
		dir = parent
	}
}

// isExcludedDir reports whether a directory (rel is its slash path relative to
// the walk root, name its base) is a categorically excluded corpus. See the
// package doc for the rationale of each rule.
func isExcludedDir(rel, name string) bool {
	// Go's reserved fixture directory, at any depth.
	if name == "testdata" {
		return true
	}
	// Version-control internals / sibling checkouts, at any depth.
	if name == ".git" || name == ".worktrees" {
		return true
	}
	// Exact-location corpora.
	switch filepath.ToSlash(rel) {
	case "docs", "tests/fixtures", "internal/install/legacydata":
		return true
	}
	return false
}

// MaintainedFiles walks root and returns every maintained file path, relative to
// root and slash-separated, sorted. The categorical exclusions above are applied
// by pruning excluded directories from the walk (SkipDir), never by filtering
// their files out afterward.
//
// A symlink whose target resolves to a regular file (the repo's CLAUDE.md ->
// AGENTS.md alias is the live example) is included; a broken or unresolvable
// symlink is an error (fail-closed). An unreadable directory is an error.
func MaintainedFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// Fail closed: an unreadable directory (or any walk error) aborts the
			// traversal rather than yielding a silently-truncated population.
			return fmt.Errorf("repoguard: walk %s: %w", path, walkErr)
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return fmt.Errorf("repoguard: relativize %s: %w", path, rerr)
		}
		if d.IsDir() {
			if rel != "." && isExcludedDir(rel, d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		if d.Type().IsRegular() {
			files = append(files, relSlash)
			return nil
		}
		if d.Type()&fs.ModeSymlink != 0 {
			info, serr := os.Stat(path) // follows the link
			if serr != nil {
				return fmt.Errorf("repoguard: unresolvable symlink %s: %w", relSlash, serr)
			}
			if info.Mode().IsRegular() {
				files = append(files, relSlash)
			}
			return nil
		}
		// Devices, sockets, named pipes: not maintained source; skip silently.
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

// ExecutableSurface filters MaintainedFiles to files whose bytes an agent or a
// shell executes, and whose content therefore counts as a live runnable
// dependency rather than descriptive prose (agent-executed-markdown-is-code):
//
//   - *.sh and *.bash shell scripts;
//   - any file carrying a Unix executable bit;
//   - maintained .md command surfaces — the script contracts under scripts/ and
//     every markdown under skills/ (SKILL.md files and their references), which a
//     harness runs verbatim.
//
// LIMITATION, stated where the population is defined (byte-pattern-guard rules and
// agent-executed-markdown-is-code): the always-loaded rule files AGENTS.md /
// CLAUDE.md and the agent-definition trees agents/ and cursor-rules/ are ALSO an
// agent-executed surface, but they are NOT folded in here — this function's
// contract is the narrower scripts/+skills/ command surface. The absence guard
// (change 0370, Gate 6) scans those always-loaded files explicitly rather than
// through ExecutableSurface; a guard that needs them reads them by name.
func ExecutableSurface(root string) ([]string, error) {
	all, err := MaintainedFiles(root)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, rel := range all {
		if isExecutableSurface(root, rel) {
			out = append(out, rel)
		}
	}
	return out, nil
}

// isExecutableSurface classifies one maintained file (rel is slash-relative to
// root). See ExecutableSurface for the categories and the stated limitation.
func isExecutableSurface(root, rel string) bool {
	base := filepath.Base(rel)
	if strings.HasSuffix(base, ".sh") || strings.HasSuffix(base, ".bash") {
		return true
	}
	// Executable-bit files (Lstat so a symlink is judged as a symlink, not its
	// target — a symlink is never itself an executable-surface entry).
	if info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(rel))); err == nil {
		if info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0 {
			return true
		}
	}
	// Maintained .md command surfaces: script contracts and skill markdown.
	if strings.HasSuffix(base, ".md") {
		if rel == "scripts" || strings.HasPrefix(rel, "scripts/") ||
			strings.HasPrefix(rel, "skills/") {
			return true
		}
	}
	return false
}
