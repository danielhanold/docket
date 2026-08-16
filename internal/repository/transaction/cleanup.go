package transaction

// This file owns two local-resource operations. The first half is per-candidate
// cleanup: tearing down exactly the candidate a run allocated, and nothing else.
// The second half — PruneAbandoned — is the recovery sweep: it reclaims candidates
// left behind by crashed runs, but only after proving, per candidate, the spec's
// complete six-point ownership proof. Neither half ever broadens to a global sweep
// and neither ever runs `git worktree prune`, resets a branch, or deletes a ref.
//
// Cleanup order matters. The registered detached worktree must be deregistered
// through Git (worktree remove) before its directory is deleted, or a bare
// RemoveAll would orphan the worktree registration in the common dir. The live
// lock is released before the candidate directory is removed. After a successful
// push the applied result is never relabelled failed: a cleanup that cannot
// finish only adds a "cleanup-pending: <id>" warning naming the state a later
// PruneAbandoned can reclaim.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/danielhanold/docket/internal/gitcli"
)

// cleanupCandidate tears down exactly the candidate c: it deregisters c's detached
// worktree (only when Git still has it registered), releases the live lock, and
// removes the candidate directory through a root anchored at the transactions
// root. It returns any cleanup-pending warnings — never an error and never a
// relabel — so a caller keeps the disposition it already computed.
func (e *Engine) cleanupCandidate(ctx context.Context, repo gitcli.Repository, c *candidate) []string {
	var warnings []string
	canRemoveDir := true

	// Deregister the worktree through Git only if it is still registered. A worktree
	// that never got added (an allocate-then-fail path) must not be reported as a
	// failed removal, and its directory can be deleted directly.
	if e.worktreeRegistered(ctx, repo, c.worktree) {
		if err := e.client.RemoveWorktree(ctx, repo, c.worktree); err != nil {
			warnings = appendCleanupPending(warnings, c.id)
			// Leave the directory in place: deleting it now would orphan Git's
			// still-live worktree registration, which we may never prune globally.
			canRemoveDir = false
		}
	}

	if c.live != nil {
		_ = c.live.release()
	}

	if canRemoveDir {
		if err := e.removeCandidateRoot(repo, c); err != nil {
			warnings = appendCleanupPending(warnings, c.id)
		}
	}
	return warnings
}

// worktreeRegistered reports whether Git currently registers a worktree at
// worktreePath, comparing canonical (Abs + every-symlink-hop) paths so a
// /tmp -> /private/tmp indirection never hides a match. A list failure is treated
// as "not registered" — cleanup then falls back to a direct directory removal
// rather than attempting a remove that would fail anyway.
func (e *Engine) worktreeRegistered(ctx context.Context, repo gitcli.Repository, worktreePath string) bool {
	infos, err := e.client.ListWorktrees(ctx, repo)
	if err != nil {
		return false
	}
	target := canonicalPath(worktreePath)
	for _, info := range infos {
		if canonicalPath(info.Path) == target {
			return true
		}
	}
	return false
}

// removeCandidateRoot deletes the candidate's directory subtree through an os.Root
// anchored at the transactions root, so the removal can never escape that owned
// tree even if the candidate id were somehow adversarial.
func (e *Engine) removeCandidateRoot(repo gitcli.Repository, c *candidate) error {
	root, err := os.OpenRoot(transactionsRoot(repo))
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	return root.RemoveAll(c.id)
}

// canonicalPath resolves p to an absolute, every-symlink-hop-canonicalized path
// for identity comparison. When the target no longer exists (already removed) it
// falls back to the absolute form, which is still a stable comparison key.
func canonicalPath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	return abs
}

// appendCleanupPending appends a "cleanup-pending: <id>" warning unless one for id
// is already present, so a two-stage cleanup failure names the id once.
func appendCleanupPending(warnings []string, id string) []string {
	msg := "cleanup-pending: " + id
	for _, w := range warnings {
		if w == msg {
			return warnings
		}
	}
	return append(warnings, msg)
}

// ---------------------------------------------------------------------------
// Recovery: PruneAbandoned and the six-point ownership proof.
// ---------------------------------------------------------------------------

// The recovery verdicts. Every evaluated candidate resolves to exactly one.
// "pruned" is the ONLY verdict that removes bytes; every other verdict leaves the
// candidate byte-untouched. They are strings so a caller (and a test) reads a
// stable vocabulary without importing an enum.
const (
	verdictPruned        = "pruned"         // passed all six checks and was removed
	verdictLive          = "live"           // live.lock is held — an owner is active
	verdictForeign       = "foreign"        // belongs to another repo / not ours by identity
	verdictMalformed     = "malformed"      // our-shaped name, unusable manifest/paths
	verdictCleanupFailed = "cleanup-failed" // proven ours+abandoned, but removal failed
)

// PruneEntry is one candidate's recovery verdict. ID is the directory basename
// exactly as found (it may be malformed or foreign). Pushed is meaningful only for
// a "pruned" entry: it reports whether the candidate's own commit was already
// reachable from the target ref (residue of an applied push) versus never pushed.
// Detail is a bounded, redaction-safe diagnostic — never object bytes, a receipt,
// a remote URL, or unbounded subprocess output.
type PruneEntry struct {
	ID      string
	Verdict string
	Pushed  bool
	Detail  string
}

// PruneReport is the complete, deterministic result of one sweep: every candidate
// the inventory saw, in ascending ID order, each with its verdict.
type PruneReport struct{ Entries []PruneEntry }

// PruneAbandoned inventories the transactions root under the short registry lock,
// then evaluates each candidate against the spec's six ownership checks and deletes
// ONLY candidates passing ALL SIX — validating the complete proof before the first
// destructive step, so a late-failing check can never leave a half-authorized
// deletion. The six checks, in order:
//
//  1. directory name and permissions have the owned shape;
//  2. manifest.json is a supported schema and every identity/path field is canonical;
//  3. manifest repository identity equals the CURRENT canonical common directory;
//  4. the candidate and its worktree resolve beneath the owned root (every symlink
//     hop canonicalized);
//  5. Git registration is absent or names exactly this worktree;
//  6. the candidate live.lock can be acquired non-blocking.
//
// A held live.lock is live regardless of the manifest's timestamp or PID — no age
// threshold overrides it and PID liveness is never consulted. Everything that fails
// any check is reported and left byte-untouched. Recovery never resets a branch,
// deletes a commit/ref, removes a feature worktree, enters the legacy .docket/
// worktree, or invokes global `git worktree prune`; if Git cannot remove stale
// administrative state by exact identity the candidate is retained with a
// diagnostic rather than deleting internal Git directories by guessed pathname.
func (e *Engine) PruneAbandoned(ctx context.Context, repo gitcli.Repository) (PruneReport, error) {
	root := transactionsRoot(repo)

	// Inventory under the registry lock. Allocation creates a candidate directory,
	// acquires its live lock, and publishes its manifest all while holding this same
	// lock, so a listing taken under it can never observe a half-published directory
	// — every name we collect already has a complete, live-locked owner or is fully
	// abandoned. The lock is dropped before any evaluation or removal: recovery,
	// like allocation, never holds the registry lock across mutation work.
	var names []string
	if err := withRegistryLock(root, func() error {
		entries, err := os.ReadDir(root)
		if err != nil {
			return err
		}
		for _, ent := range entries {
			// registry.lock is the mutex file itself, not a candidate.
			if ent.Name() == registryLockName {
				continue
			}
			names = append(names, ent.Name())
		}
		return nil
	}); err != nil {
		return PruneReport{}, err
	}

	sort.Strings(names)
	report := PruneReport{}
	for _, name := range names {
		report.Entries = append(report.Entries, e.evaluateCandidate(ctx, repo, root, name))
	}
	return report, nil
}

// evaluateCandidate runs the full six-point ownership proof on one inventoried
// directory and, only when all six pass, prunes it. It performs NO destructive or
// state-touching action until every check has passed: the pure filesystem/manifest
// inspection (checks 1–5) precedes the live-lock acquire (check 6), which precedes
// the first removal. Any failure returns a byte-untouched verdict.
func (e *Engine) evaluateCandidate(ctx context.Context, repo gitcli.Repository, root, name string) PruneEntry {
	entry := PruneEntry{ID: name}
	candRoot := filepath.Join(root, name)

	// Check 1: owned directory shape and permissions. A non-owned name, a symlink
	// standing in for the directory, a non-directory, or loosened permissions all
	// disqualify the entry before its manifest is even read.
	if !isOwnedTransactionID(name) {
		return entry.reject(verdictForeign, "directory name is not an owned transaction id")
	}
	li, err := os.Lstat(candRoot)
	if err != nil {
		// Vanished between inventory and evaluation (a concurrent prune/cleanup).
		return entry.reject(verdictMalformed, "candidate directory is no longer present")
	}
	if li.Mode()&os.ModeSymlink != 0 {
		return entry.reject(verdictForeign, "candidate root is a symlink")
	}
	if !li.IsDir() {
		return entry.reject(verdictForeign, "candidate root is not a directory")
	}
	if li.Mode().Perm() != txnDirMode {
		return entry.reject(verdictMalformed, "candidate root has unexpected permissions")
	}

	// Check 2: supported manifest with canonical identity/path fields. Absence,
	// truncation, an unsupported schema, a non-fixed worktree name, or a
	// non-canonical common dir are all malformed — absence never certifies ownership.
	m, ok := readManifestFile2(candRoot)
	if !ok {
		return entry.reject(verdictMalformed, "manifest is absent or unreadable")
	}
	if m.Schema != manifestSchemaVersion {
		return entry.reject(verdictMalformed, "manifest schema is unsupported")
	}
	if m.WorktreeRel != worktreeDirName {
		return entry.reject(verdictMalformed, "manifest worktree_rel is not the canonical name")
	}
	if !isCanonicalAbs(m.CommonDir) {
		return entry.reject(verdictMalformed, "manifest common_dir is not canonical")
	}

	// Check 3: repository identity. A manifest that names a different repository's
	// common directory is foreign — it belongs to another repo's transactions tree.
	if m.CommonDir != repo.CommonDir {
		return entry.reject(verdictForeign, "manifest common_dir names a different repository")
	}

	// Check 4: containment. Canonicalizing every symlink hop, the candidate root and
	// its worktree must both resolve beneath the owned transactions root. A symlinked
	// ancestor that redirects either path outside the owned tree disqualifies it.
	ownedRoot := canonicalPath(root)
	if !resolvesBeneath(candRoot, ownedRoot) {
		return entry.reject(verdictForeign, "candidate does not resolve beneath the owned root")
	}
	worktreePath := filepath.Join(candRoot, m.WorktreeRel)
	if !resolvesBeneath(worktreePath, canonicalPath(candRoot)) {
		return entry.reject(verdictMalformed, "worktree does not resolve beneath the candidate root")
	}

	// Check 5: Git registration is absent or names exactly this worktree. A
	// registration pointing at an unexpected path inside the candidate is ambiguous
	// ownership — refuse rather than guess. A list failure is itself a reason not to
	// delete: without the registration picture we cannot prove the worktree is
	// unregistered or exactly ours.
	infos, err := e.client.ListWorktrees(ctx, repo)
	if err != nil {
		return entry.reject(verdictCleanupFailed, "cannot list worktrees to prove registration")
	}
	regStatus, regHead := classifyRegistration(infos, canonicalPath(candRoot), canonicalPath(worktreePath))
	if regStatus == regAmbiguous {
		return entry.reject(verdictForeign, "ambiguous git worktree registration")
	}

	// Check 6: the live lock is acquirable non-blocking. A held lock means an owner
	// is active — the candidate is live regardless of any timestamp or PID recorded
	// in its manifest. This is the ONLY liveness signal. Acquiring the lock is not a
	// destructive act; it is held across the removal and released afterward.
	lock, err := acquireLock(filepath.Join(candRoot, liveLockName), false)
	if err != nil {
		if errors.Is(err, errLockHeld) {
			return entry.reject(verdictLive, "live lock is held by an active owner")
		}
		return entry.reject(verdictCleanupFailed, "cannot acquire live lock")
	}
	defer func() { _ = lock.release() }()

	// All six checks passed: this candidate is provably ours and abandoned. Before
	// the first destructive step, inspect the target ref and the candidate's own
	// commit so the report distinguishes an already-pushed residue from never-pushed
	// residue. This read is non-destructive and precedes every removal.
	entry.Pushed = e.candidateReachable(ctx, repo, m.TargetRef, regHead, regStatus == regExact)

	// First destructive step. Deregister the exact registered worktree through Git
	// before deleting the directory, so a RemoveAll can never orphan a live
	// registration. A removal failure retains the candidate with a diagnostic rather
	// than deleting Git-internal state by guessed pathname.
	if regStatus == regExact {
		if err := e.client.RemoveWorktree(ctx, repo, worktreePath); err != nil {
			return entry.reject(verdictCleanupFailed, "worktree remove failed")
		}
	}
	if err := e.removeCandidateRoot(repo, &candidate{id: name}); err != nil {
		return entry.reject(verdictCleanupFailed, "candidate directory removal failed")
	}
	entry.Verdict = verdictPruned
	return entry
}

// reject stamps a byte-untouched verdict and bounded detail and returns the entry
// by value, so an evaluation reads as `return entry.reject(...)`.
func (p PruneEntry) reject(verdict, detail string) PruneEntry {
	p.Verdict = verdict
	p.Detail = detail
	return p
}

// candidateReachable reports whether commit is already reachable from the target
// ref (an applied push's residue). It is meaningful only when the worktree is
// registered so its detached HEAD is known; an absent registration, an
// unresolvable target ref, or an ancestry probe error all read as "not reachable"
// — the conservative report, never a reason to skip deletion.
func (e *Engine) candidateReachable(ctx context.Context, repo gitcli.Repository, target gitcli.RefName, commit gitcli.ObjectID, haveCommit bool) bool {
	if !haveCommit {
		return false
	}
	tip, err := e.client.ResolveRef(ctx, repo, target)
	if err != nil {
		return false
	}
	reachable, err := e.client.IsAncestor(ctx, repo, commit, tip)
	if err != nil {
		return false
	}
	return reachable
}

// registration is the trichotomy check 5 resolves a candidate's Git registration
// to: absent (no registration touches the candidate tree), exact (exactly the
// candidate's own worktree is registered), or ambiguous (a registration points at
// an unexpected path inside the candidate tree).
type registration int

const (
	regAbsent registration = iota
	regExact
	regAmbiguous
)

// classifyRegistration inspects every Git worktree registration against the
// canonical candidate root and worktree path. A registration whose canonical path
// equals the worktree is the candidate's own; any registration whose canonical
// path is the candidate root or lies beneath it but is NOT the worktree is
// ambiguous ownership. When the exact worktree is registered its detached HEAD is
// returned so the caller can report reachability. Every path is compared after
// canonicalizing every symlink hop, so a /tmp -> /private/tmp indirection cannot
// hide or fake a match.
func classifyRegistration(infos []gitcli.WorktreeInfo, candRootCanon, worktreeCanon string) (registration, gitcli.ObjectID) {
	status := regAbsent
	var head gitcli.ObjectID
	for _, info := range infos {
		p := canonicalPath(info.Path)
		switch {
		case p == worktreeCanon:
			if status != regAmbiguous {
				status = regExact
				head = info.Head
			}
		case p == candRootCanon || strings.HasPrefix(p, candRootCanon+string(filepath.Separator)):
			// A registration inside our owned candidate tree but at a path other than
			// the expected worktree: we cannot prove sole ownership. Ambiguous wins.
			status = regAmbiguous
		}
	}
	return status, head
}

// isOwnedTransactionID reports whether name has the exact shape of a minted
// transaction id: 32 lowercase hex characters. It keys on shape, never on a
// list of known ids, so a foreign-named directory is rejected regardless of
// spelling.
func isOwnedTransactionID(name string) bool {
	if len(name) != 32 {
		return false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// isCanonicalAbs reports whether p is an absolute path already equal to its own
// every-symlink-hop canonicalization — the shape allocation writes into a manifest.
// A relative, dotted, or symlink-bearing spelling is not canonical.
func isCanonicalAbs(p string) bool {
	return p != "" && filepath.IsAbs(p) && canonicalPath(p) == p
}

// resolvesBeneath reports whether path, after canonicalizing every symlink hop,
// equals rootCanon or lies strictly beneath it. rootCanon must already be
// canonical. A path that escapes the root (via `..` or a symlinked ancestor)
// resolves outside and returns false.
func resolvesBeneath(path, rootCanon string) bool {
	p := canonicalPath(path)
	if p == rootCanon {
		return true
	}
	return strings.HasPrefix(p, rootCanon+string(filepath.Separator))
}

// readManifestFile2 reads and decodes a candidate's manifest directly from
// candRoot without a *candidate. It reports ok=false for an absent, unreadable, or
// undecodable manifest — the malformed cases recovery must refuse.
func readManifestFile2(candRoot string) (manifest, bool) {
	c := &candidate{root: candRoot}
	m, err := c.readManifest()
	if err != nil {
		return manifest{}, false
	}
	return m, true
}
