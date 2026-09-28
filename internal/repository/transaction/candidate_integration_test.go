//go:build integration

package transaction

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"syscall"
	"testing"
	"time"
)

// TestIntegrationTxnApplyAllocateCandidateStructureAndManifest proves allocation lays down the
// private candidate tree, mints a 32-hex id, publishes a fully populated
// manifest, and stays invisible to the primary checkout's status.
func TestIntegrationTxnApplyAllocateCandidateStructureAndManifest(t *testing.T) {
	client, repo := newTxnRepo(t)

	c, err := allocateCandidate(txnTestClock, repo, "origin", "refs/heads/main", fixedBase)
	if err != nil {
		t.Fatalf("allocateCandidate: %v", err)
	}
	defer func() { _ = c.live.release() }()

	// Directory shape: <transactionsRoot>/<id>/{worktree not yet, hooks empty}.
	wantRoot := filepath.Join(transactionsRoot(repo), c.id)
	if c.root != wantRoot {
		t.Errorf("c.root = %q, want %q", c.root, wantRoot)
	}
	if c.worktree != filepath.Join(wantRoot, "worktree") {
		t.Errorf("c.worktree = %q", c.worktree)
	}
	if c.hooks != filepath.Join(wantRoot, "hooks") {
		t.Errorf("c.hooks = %q", c.hooks)
	}
	if fi, err := os.Stat(c.root); err != nil || !fi.IsDir() {
		t.Fatalf("candidate root not a dir: %v", err)
	}
	entries, err := os.ReadDir(c.hooks)
	if err != nil {
		t.Fatalf("read hooks dir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("hooks dir not empty: %v", entries)
	}
	if _, err := os.Stat(filepath.Join(c.root, "manifest.json")); err != nil {
		t.Errorf("manifest.json missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(c.root, "live.lock")); err != nil {
		t.Errorf("live.lock missing: %v", err)
	}

	// ID shape.
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(c.id) {
		t.Errorf("id %q does not match ^[0-9a-f]{32}$", c.id)
	}

	// Two allocations differ.
	c2, err := allocateCandidate(txnTestClock, repo, "origin", "refs/heads/main", fixedBase)
	if err != nil {
		t.Fatalf("second allocateCandidate: %v", err)
	}
	defer func() { _ = c2.live.release() }()
	if c.id == c2.id {
		t.Errorf("two allocations produced the same id %q", c.id)
	}

	// Manifest round-trips with every field populated.
	m := readManifestFile(t, c.root)
	wantStamp := txnTestClock.Now().UTC().Format(time.RFC3339)
	want := manifest{
		Schema:        manifestSchemaVersion,
		TransactionID: c.id,
		CommonDir:     repo.CommonDir,
		Remote:        "origin",
		TargetRef:     "refs/heads/main",
		BaseCommit:    fixedBase,
		WorktreeRel:   "worktree",
		Phase:         phaseAllocating,
		CreatedUTC:    wantStamp,
		UpdatedUTC:    wantStamp,
		PID:           os.Getpid(),
	}
	if m != want {
		t.Errorf("manifest =\n%+v\nwant\n%+v", m, want)
	}

	// The transactions tree lives under the common dir, invisible to status.
	changes, err := client.ChangedPaths(context.Background(), repo.PrimaryWorktree)
	if err != nil {
		t.Fatalf("ChangedPaths: %v", err)
	}
	if len(changes) != 0 {
		t.Errorf("primary checkout dirty after allocation: %v", changes)
	}
}

// TestIntegrationTxnApplyCandidateModesUnderUmask proves the promised modes are enforced with an
// explicit chmod: under a permissive umask (0o022) an unchmodded lock/dir would
// land at 0644/0755, so passing under BOTH umasks can only mean the chmod ran.
func TestIntegrationTxnApplyCandidateModesUnderUmask(t *testing.T) {
	for _, um := range []int{0o077, 0o022} {
		t.Run(fmt.Sprintf("umask_%04o", um), func(t *testing.T) {
			old := syscall.Umask(um)
			defer syscall.Umask(old)

			_, repo := newTxnRepo(t)
			c, err := allocateCandidate(txnTestClock, repo, "origin", "refs/heads/main", fixedBase)
			if err != nil {
				t.Fatalf("allocateCandidate: %v", err)
			}
			defer func() { _ = c.live.release() }()

			assertPerm(t, c.root, 0o700)
			assertPerm(t, c.hooks, 0o700)
			assertPerm(t, filepath.Join(c.root, "manifest.json"), 0o600)
			assertPerm(t, filepath.Join(c.root, "live.lock"), 0o600)
		})
	}
}
