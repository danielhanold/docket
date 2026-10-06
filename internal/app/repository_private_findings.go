package app

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/layout"
	"github.com/danielhanold/docket/internal/reposetup"
)

// This file holds the two check findings visibility adds: a repository-level
// config file whose `visibility` disagrees with the repository's actual mode
// (the setting never moves a repository), and a private store checkout whose
// clone moved or no longer exists.

// The finding codes.
const (
	FindingVisibilityMismatch = "visibility-mismatch"
	FindingOrphanedCheckout   = "orphaned-checkout"
)

// visibilityMismatchFinding reports a repository-level config file
// (.docket.yml, .docket.local.yml, or .git/dckt/config.yml) that explicitly
// sets a visibility other than the mode the repository is set up in. A global
// setting or the built-in default never mismatches: they only choose the mode
// a fresh repository is set up in.
func visibilityMismatchFinding(lay layout.Layout, eff config.Effective) *reposetup.Finding {
	v := eff.Visibility
	if !v.Explicit {
		return nil
	}
	if v.Provenance.Layer != config.LayerRepository && v.Provenance.Layer != config.LayerRepositoryLocal {
		return nil
	}
	if layout.Mode(v.Value) == lay.Mode {
		return nil
	}
	src := v.Provenance.Source
	return &reposetup.Finding{
		Code:     FindingVisibilityMismatch,
		Severity: reposetup.SeverityWarning,
		Ref:      src,
		Message: fmt.Sprintf("%s sets visibility: %s, but this repository is set up %s; the setting does not move an existing repository.",
			src, v.Value, lay.Mode),
		Remedy: fmt.Sprintf("Change visibility in %s to %s, or switch the repository with `docket repository set-visibility %s`.",
			src, lay.Mode, v.Value),
	}
}

// scanOrphanedCheckouts lists the private store's checkouts (other than this
// clone's own) whose .git file names a gitdir that no longer exists: the clone
// that owned them moved or was deleted. A shared layout, or a store with no
// checkouts folder, has none. A checkout whose .git file cannot be read or
// parsed is skipped — it is not proven ours. Any other probe error is
// returned, never read as "no orphans".
func scanOrphanedCheckouts(lay layout.Layout) ([]string, error) {
	if lay.Mode != layout.Private || lay.CheckoutsDir == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(lay.CheckoutsDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("listing %s: %w", lay.CheckoutsDir, err)
	}
	own := filepath.Clean(lay.MetadataWorktree)
	var orphans []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(lay.CheckoutsDir, e.Name())
		if filepath.Clean(dir) == own {
			continue
		}
		orphan, err := checkoutOrphaned(dir)
		if err != nil {
			return nil, err
		}
		if orphan {
			orphans = append(orphans, dir)
		}
	}
	sort.Strings(orphans)
	return orphans, nil
}

// checkoutOrphaned reports whether the checkout at dir names a gitdir that no
// longer exists. An unreadable or unparsable .git file is not an orphan.
func checkoutOrphaned(dir string) (bool, error) {
	gitdir, ok := checkoutGitdir(dir)
	if !ok {
		return false, nil
	}
	_, err := os.Stat(gitdir)
	switch {
	case err == nil:
		return false, nil
	case errors.Is(err, fs.ErrNotExist):
		return true, nil
	default:
		return false, fmt.Errorf("probing the gitdir of %s: %w", dir, err)
	}
}

// staleOwnCheckout reports whether this clone's own private checkout path holds
// a stale checkout: one whose .git file names a gitdir that no longer exists.
// Deleting a private clone and re-cloning it at the same path leaves exactly
// that (the clone id depends only on the path, and the store survives). A
// stale checkout is no live worktree of anything, so it reads as absent and is
// replaced on attach. An unreadable .git file or a probe error is not proven
// stale. A shared layout never has one.
func staleOwnCheckout(lay layout.Layout) bool {
	if lay.Mode != layout.Private || lay.MetadataWorktree == "" {
		return false
	}
	stale, err := checkoutOrphaned(lay.MetadataWorktree)
	return err == nil && stale
}

// removeStaleOwnCheckout removes this clone's own private checkout when it is
// proven stale now, re-verified on the copy it acts on (learning
// decide-and-act-on-the-same-copy): a live checkout, an absent one, or one whose
// .git file cannot be read is left alone. Its data lives in the metadata remote.
func removeStaleOwnCheckout(lay layout.Layout) error {
	if lay.Mode != layout.Private || lay.MetadataWorktree == "" {
		return nil
	}
	stale, err := checkoutOrphaned(lay.MetadataWorktree)
	if err != nil || !stale {
		return err
	}
	if err := os.RemoveAll(lay.MetadataWorktree); err != nil {
		return fmt.Errorf("removing the stale checkout %s: %w", lay.MetadataWorktree, err)
	}
	return nil
}

// checkoutGitdir parses the gitdir a checkout's .git file names, resolving a
// relative path against the checkout. ok is false when the file cannot be
// read or names no gitdir.
func checkoutGitdir(dir string) (string, bool) {
	raw, err := os.ReadFile(filepath.Join(dir, ".git"))
	if err != nil {
		return "", false
	}
	gitdir, ok := parseGitdirLine(raw)
	if !ok {
		return "", false
	}
	if !filepath.IsAbs(gitdir) {
		gitdir = filepath.Join(dir, gitdir)
	}
	return filepath.Clean(gitdir), true
}

// parseGitdirLine returns the path on the first "gitdir:" line.
func parseGitdirLine(raw []byte) (string, bool) {
	for _, line := range strings.Split(string(raw), "\n") {
		if rest, ok := strings.CutPrefix(line, "gitdir:"); ok {
			p := strings.TrimSpace(rest)
			return p, p != ""
		}
	}
	return "", false
}

// orphanedCheckoutFinding is the warning one orphaned checkout yields.
func orphanedCheckoutFinding(path string) reposetup.Finding {
	repairable := true
	return reposetup.Finding{
		Code:       FindingOrphanedCheckout,
		Severity:   reposetup.SeverityWarning,
		Ref:        path,
		Message:    "A metadata checkout whose clone moved or no longer exists.",
		Remedy:     "Run `docket repository repair` to remove it; its data lives in the metadata remote.",
		Repairable: &repairable,
	}
}

// orphanScanUnverifiedFinding is the warning a failed orphan scan yields in
// place of the per-orphan findings: unverified, never "none".
func orphanScanUnverifiedFinding(err error) reposetup.Finding {
	return reposetup.Finding{
		Code:     FindingOrphanedCheckout + "-unverified",
		Severity: reposetup.SeverityWarning,
		Message:  "The metadata store's checkouts could not be scanned for orphans (unverified, not proven absent): " + err.Error(),
		Remedy:   "Restore read access to the metadata store's checkouts folder, then re-run `docket repository check`.",
	}
}

// privateCheckFindings are the visibility findings check appends after
// classification. Neither changes the classified state.
func privateCheckFindings(sc setupContext) []reposetup.Finding {
	var out []reposetup.Finding
	if f := visibilityMismatchFinding(sc.layout, sc.cfg); f != nil {
		out = append(out, *f)
	}
	orphans, err := scanOrphanedCheckouts(sc.layout)
	if err != nil {
		return append(out, orphanScanUnverifiedFinding(err))
	}
	for _, p := range orphans {
		out = append(out, orphanedCheckoutFinding(p))
	}
	return out
}
