<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0494 — A publish killed mid-flight wedges its run's cancel and closeout](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-03-0494-a-publish-killed-mid-flight-wedges-its-run-s-cancel-and-clos.md)**
<!-- docket:backlink:end -->
# A Publish Killed Mid-Flight Wedges Its Run's Cancel and Closeout: Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. In this repository the plan is executed by `docket-build`. It routes each task to the suggested tier worker and runs one full-suite gate at the end.

**Goal:** A `pr.publish` / `workspace.publish` journal entry blocks cancel, resume, and the success closeout only while its publisher may still be running. A publisher that is provably gone is reported as the informational finding `mutation-abandoned:<op>` and never blocks.

**Architecture:** The publisher takes a per-entry kernel `flock` (`<run-key-dir>/publish-<token>.lock`) before its `admitted` entry is written, and closes it only after the outcome is written. The entry journals the token. One classifier (`classifyAdmittedMutation`) replaces the three inline `Status != completed` loops in `reconcileRunTeardown`, `verifyTerminalRunQuiescence`, and `accountCompletionMutations`. `settleUncertainPublications` (the two write paths: cancel and the keyed closeout) first rewrites a dead publisher's `admitted` entry to `uncertain`, so 0444's verified-identical-retry match can still settle it as `mutation-settled`.

**Tech Stack:** Go 1.26 and the standard library (`syscall.Flock`). Tests are Go tests; real-git tests sit behind the `integration` build tag and are sharded by test-name prefix.

**Spec:** `docs/superpowers/specs/2026-10-03-a-publish-killed-mid-flight-wedges-its-run-s-cancel-and-clos-design.md` on the `docket` branch. Its synchronized copy is at `/Users/homer/dev/docket/.docket/docs/superpowers/specs/2026-10-03-a-publish-killed-mid-flight-wedges-its-run-s-cancel-and-clos-design.md`.

**Out of this plan (coordinator-owned):** The spec's Decision record is the new ADR "The publish journal blocks only on a publisher that may still be running", plus dated Update notes on ADR-0124 and ADR-0118. The coordinator records them on the `docket` branch through `docket-adr`. No task here writes or edits an ADR.

## Global Constraints

- `internal/process` stays standard-library-only (`internal/process/boundary_test.go` guards this).
- Lock files are never deleted. A holder never calls `LOCK_UN` and releases by close only. Every acquire and probe tries and never waits (ADR-0132).
- The lock path is exactly `<run-key-dir>/publish-<token>.lock`. `<run-key-dir>` is what `runKeyDir(repoDir, runKey, op)` returns. `<token>` is minted by `runToken()`: 32 lowercase hex characters.
- `AdmittedMutation.LockToken` uses the json tag `lock_token,omitempty`. It is additive, with **no** `runSchemaVersion` bump. An absent value means "no lock".
- The new finding is spelled exactly `mutation-abandoned:<op>`. `mutation-pending:<op>` and `mutation-settled:<op>` keep their spellings.
- No new refusal, no new block, no new signal, and no new Git or GitHub call in cancel, closeout, or resume. Every unprovable case stays the existing blocking `mutation-pending:<op>`.
- A lock failure at admission never refuses the publish. The entry is then journaled without `LockToken`, exactly as today.
- No lock for metadata transactions (`MutationAdmissionHook`, `pub == nil`), and none for an unfenced publish with no owning run.
- No protocol schema descriptor changes: `AdmittedMutation` is run-record state.
- Comment cross-references anchor on a symbol name or a verbatim-quoted clause, never a line number (ADR-0054).
- Default-build (untagged) tests in `internal/app` must never run real git: the no-git guard enforces it. Real-git tests go behind `//go:build integration`. They must be named with an existing shard prefix (`TestIntegrationRunCancel`, `TestIntegrationRunCompletion`, `TestIntegrationRunFence`), or no shard runs them.
- Every focused or mutation run uses `-count=1`, so the Go test cache cannot serve a stale PASS. A mutation is restored from a backup copy (`cp f f.bak` … `mv -f f.bak f`), never `git checkout --`, which would destroy uncommitted work.
- Tests that override a package-level seam (`publishLockMintToken`, `publishLockAcquire`, `publishLockProbe`) must not call `t.Parallel()`, and must restore the seam through `t.Cleanup`. Use `overrideSeam`.

## Review Focus

These are the inputs and conditions the spec implies but does not enumerate. Each one is pinned by a test in the task that owns the code.

1. **A corrupt or hand-edited `lock_token` in `run.json`** (for example `../../x`, uppercase, the wrong length). It must block as `mutation-pending` and must never be joined into a probe path. Pinned in Task 2 (`TestClassifyAdmittedMutationMatrix`, "malformed token" rows, which also assert the probe seam is never called) and Task 5 ("malformed token" case).
2. **Something other than a lock file at the lock path** (a directory, or an unreadable node). The real probe answers unknown, and the entry blocks. Pinned in Task 1 (`TestProbeLockTypedAnswers`, directory case), Task 2 ("lock path is a directory" row), and Task 5 ("lock path is a directory" case).
3. **The run-key directory cannot be resolved while classifying** (`runKeyDir` errors). Every token-bearing `admitted` entry must block, never read as free. Pinned in Task 2 ("empty run dir" row).
4. **A cancel that wins while a live publisher is mid-call, and the publisher then finishes.** The first cancel stays pending. The publisher's own callback writes `completed` and releases. The repeat cancel reports `cancelled` with no `mutation-abandoned`. Pinned in Task 5 (`TestIntegrationRunCancelPublisherFinishingAfterFenceIsNotAbandoned`).
5. **Repeating cancel or resume after an abandonment.** The entry is not rewritten twice. A repeat cancel on the terminal run reports `already-cancelled` and still carries the informational finding. Resume admits exactly one replacement. Pinned in Task 5 (rewritten `TestIntegrationRunCancelUncertainWithoutRetryIsAbandoned`, and `TestIntegrationRunCancelKilledPublisherIsAbandoned`).

---

## File Structure

- `internal/process/lock.go` (modify): add the exported typed probe `LockProbe` / `ProbeLock`, and make `probeFlock` delegate to it.
- `internal/process/lock_test.go` (modify): `TestProbeLockTypedAnswers`.
- `internal/app/runtracker_run_record.go` (modify): `AdmittedMutation.LockToken`.
- `internal/app/runtracker_publish_lock.go` (create): the publish-lock primitives (token validation, path, seams, acquire) and the single journal classifier (`classifyAdmittedMutation`, `accountAdmittedMutations`, `publisherGone`, `runJournalDir`).
- `internal/app/runtracker_publish_lock_test.go` (create, untagged): classifier and acquire unit tests, plus the shared `overrideSeam` helper.
- `internal/app/runtracker_publish_lock_holder_test.go` (create, untagged): the re-exec child role that holds a publish lock until SIGKILLed.
- `internal/app/gate_test.go` (modify): route the holder role in `TestMain`.
- `internal/app/runtracker_publish_lock_helpers_integration_test.go` (create, `integration`): shared seeders for real lock files and journals.
- `internal/app/runtracker_publish_lock_process_integration_test.go` (create, `integration`): the real SIGKILL test.
- `internal/app/runtracker_fence.go` (modify): lock-before-admit and release-after-outcome in `admitWorkflowMutation`.
- `internal/app/pr_publish_test.go`, `internal/app/workspace_ops_test.go` (modify): adapter hooks `onEnsure` / `onPublish` on the fakes.
- `internal/app/runtracker_fence_publish_lock_integration_test.go` (create, `integration`): admission tests.
- `internal/app/runtracker_publication.go` (modify): the admitted→uncertain step in `settleUncertainPublications`.
- `internal/app/runtracker_cancel.go` (modify): classifier in `reconcileRunTeardown` step (7) and in `verifyTerminalRunQuiescence`.
- `internal/app/runtracker_cancel_publish_lock_integration_test.go` (create, `integration`): cancel and resume tests.
- `internal/app/runtracker_complete.go` (modify): classifier in `accountCompletionMutations`.
- `internal/app/runtracker_complete_publish_lock_integration_test.go` (create, `integration`): closeout and keyed-verdict tests.
- Rewritten 0444 tests: `internal/app/runtracker_cancel_integration_test.go`, `runtracker_complete_integration_test.go`, `runtracker_fence_integration_test.go`, and `runtracker_publication_integration_test.go`.
- `docs/reference/glossary.md` and `docs/concepts/run-tracker.md` (modify).

Task order: 1 → 2 → 3 → 4 → 5 → 6 → 7. Every task leaves the tree buildable and its shards green.

---

### Task 1: Typed lock probe in `internal/process`

**Suggested tier:** economy (fully specified, pattern-following).

**Files:**
- Modify: `internal/process/lock.go` (add after `TryExclusiveLock`; replace the body of `probeFlock`)
- Test: `internal/process/lock_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces: `type LockProbe int` with the constants `LockProbeUnknown` (the zero value), `LockProbeHeld`, `LockProbeFree`, and `LockProbeMissing`, plus `func ProbeLock(path string) LockProbe`. Task 2 assigns `process.ProbeLock` to a seam.

- [ ] **Step 1: Write the failing test.** Append to `internal/process/lock_test.go`, and add the imports `errors`, `io/fs`, and `os`:

```go
// TestProbeLockTypedAnswers (change 0494): ProbeLock is the exported, typed,
// never-creating, never-waiting probe the publish journal classifier uses.
func TestProbeLockTypedAnswers(t *testing.T) {
	if LockProbe(0) != LockProbeUnknown {
		t.Fatal("the zero LockProbe must be LockProbeUnknown, so a default is never free")
	}
	dir := testsupport.TempDir(t)
	path := filepath.Join(dir, "publish-x.lock")

	if got := ProbeLock(path); got != LockProbeMissing {
		t.Fatalf("missing file probed %v, want LockProbeMissing", got)
	}
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("ProbeLock must never create the lock file, stat err = %v", err)
	}

	f, busy, err := TryExclusiveLock(path)
	if err != nil || busy {
		t.Fatalf("TryExclusiveLock: busy=%v err=%v", busy, err)
	}
	if got := ProbeLock(path); got != LockProbeHeld {
		t.Fatalf("held lock probed %v, want LockProbeHeld", got)
	}
	f.Close() // the kernel releases on close
	if got := ProbeLock(path); got != LockProbeFree {
		t.Fatalf("released lock probed %v, want LockProbeFree", got)
	}
	// The probe leaves no hold of its own behind.
	g, busy, err := TryExclusiveLock(path)
	if err != nil || busy {
		t.Fatalf("lock not free after a Free probe: busy=%v err=%v", busy, err)
	}
	g.Close()

	// A directory at the lock path cannot be proven free, held, or missing.
	d := filepath.Join(dir, "is-a-dir.lock")
	if err := os.Mkdir(d, 0o700); err != nil {
		t.Fatal(err)
	}
	if got := ProbeLock(d); got != LockProbeUnknown {
		t.Fatalf("directory probed %v, want LockProbeUnknown", got)
	}
}
```

- [ ] **Step 2: Run it and confirm it fails.**
Run: `go test -count=1 -run '^TestProbeLockTypedAnswers$' ./internal/process/`
Expected: a build failure (`undefined: LockProbe`).

- [ ] **Step 3: Implement.** Add the following to `internal/process/lock.go` after `TryExclusiveLock`:

```go
// LockProbe is ProbeLock's typed answer (change 0494). The zero value is
// LockProbeUnknown, so an unset or defaulted answer is never read as free.
type LockProbe int

const (
	// LockProbeUnknown: the probe could not decide (an open or flock error other
	// than not-exist / would-block). Never read as free and never as missing.
	LockProbeUnknown LockProbe = iota
	// LockProbeHeld: another open file description holds the lock right now.
	LockProbeHeld
	// LockProbeFree: the lock file exists and nobody holds it.
	LockProbeFree
	// LockProbeMissing: no file exists at the path.
	LockProbeMissing
)

// ProbeLock reports, without creating the file and without waiting, whether the
// advisory lock at path is held. It opens WITHOUT O_CREATE and tries
// LOCK_EX|LOCK_NB on a fresh descriptor. Acquiring proves no holder; the probe
// then releases by closing (never LOCK_UN), leaving the lock exactly as it found
// it. EWOULDBLOCK proves a live holder. A missing file is LockProbeMissing, and
// any other error is LockProbeUnknown.
func ProbeLock(path string) LockProbe {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return LockProbeMissing
		}
		return LockProbeUnknown
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return LockProbeHeld
		}
		return LockProbeUnknown
	}
	return LockProbeFree
}
```

Then replace the body of `probeFlock` so it delegates to the typed probe. Keep its existing doc comment, but change "it immediately unlocks and closes" to "it immediately closes":

```go
func probeFlock(path string) (held bool, answer probeAnswer) {
	switch ProbeLock(path) {
	case LockProbeHeld:
		return true, probeLive
	case LockProbeFree, LockProbeMissing:
		return false, probeAbsent
	default:
		return false, probeUnknown
	}
}
```

- [ ] **Step 4: Run the package and confirm it passes.**
Run: `go test -count=1 ./internal/process/`
Expected: PASS, including `TestFlockLifecycle`, `TestProbeLockTypedAnswers`, and the boundary test.

- [ ] **Step 5: Mutation check.** Run `cp internal/process/lock.go /tmp/lock.go.bak`. In `ProbeLock`, change `return LockProbeMissing` to `return LockProbeFree`, and run Step 2's command. Expected: FAIL ("missing file probed"). Restore with `mv -f /tmp/lock.go.bak internal/process/lock.go`, then re-run Step 4 and confirm it passes.

- [ ] **Step 6: Commit.**

```bash
git add internal/process/lock.go internal/process/lock_test.go
git commit -m "feat(0494): typed never-creating lock probe in internal/process"
```

---

### Task 2: Publish-lock primitives and the single journal classifier

**Suggested tier:** standard.

**Files:**
- Modify: `internal/app/runtracker_run_record.go` (`AdmittedMutation` struct)
- Create: `internal/app/runtracker_publish_lock.go`
- Test: `internal/app/runtracker_publish_lock_test.go` (untagged; temp dirs only, no git)

**Interfaces:**
- Consumes: `process.ProbeLock`, `process.LockProbe*`, `process.TryExclusiveLock`, `runToken() (string, error)`, `runKeyDir(repoDir, key, op string) (string, error)`, and the constants `mutationStatusAdmitted`, `mutationStatusUncertain` (`runtracker_fence.go`) and `mutationStatusCompleted` (`runtracker_cancel.go`).
- Produces (Tasks 3–6 rely on these exact names):
  - field `AdmittedMutation.LockToken string` with the json tag `lock_token,omitempty`
  - `const findingMutationPending = "mutation-pending:"`, `const findingMutationAbandoned = "mutation-abandoned:"`
  - `var publishLockMintToken func() (string, error)`, `var publishLockAcquire func(string) (*os.File, bool, error)`, `var publishLockProbe func(string) process.LockProbe`
  - `func validPublishLockToken(tok string) bool`
  - `func publishLockPath(dir, token string) (string, bool)`
  - `func acquirePublishLock(dir string) (token string, f *os.File)`: returns `("", nil)` on any failure
  - `func publisherGone(dir string, m AdmittedMutation) bool`
  - `func classifyAdmittedMutation(dir string, m AdmittedMutation) (blocks bool, finding string)`
  - `func accountAdmittedMutations(dir string, muts []AdmittedMutation) (blocked bool, findings []string)`
  - `func runJournalDir(repoDir, runKey string) string`: returns `""` when unresolvable
  - test helper `func overrideSeam[T any](t *testing.T, target *T, v T)` (in the untagged test file)

- [ ] **Step 1: Write the failing tests.** Create `internal/app/runtracker_publish_lock_test.go`:

```go
package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/process"
	"github.com/danielhanold/docket/internal/testsupport"
)

// overrideSeam swaps a package-level seam for the test and restores it on cleanup.
// Tests that use it must not call t.Parallel.
func overrideSeam[T any](t *testing.T, target *T, v T) {
	t.Helper()
	old := *target
	*target = v
	t.Cleanup(func() { *target = old })
}

// lockedTempFile makes a real publish lock for token in dir. hold keeps it held by
// this process until cleanup; !hold leaves the file present and free.
func lockedTempFile(t *testing.T, dir, token string, hold bool) {
	t.Helper()
	path, ok := publishLockPath(dir, token)
	if !ok {
		t.Fatalf("publishLockPath rejected %q", token)
	}
	f, busy, err := process.TryExclusiveLock(path)
	if err != nil || busy {
		t.Fatalf("TryExclusiveLock: busy=%v err=%v", busy, err)
	}
	if !hold {
		f.Close()
		return
	}
	t.Cleanup(func() { f.Close() })
}

func TestValidPublishLockToken(t *testing.T) {
	good := strings.Repeat("0123456789abcdef", 2)
	if !validPublishLockToken(good) {
		t.Fatalf("%q must be valid", good)
	}
	for _, bad := range []string{"", good[:31], good + "0", strings.ToUpper(good),
		"../../../../../../../../tmp/xx", strings.Repeat("g", 32), good[:30] + "/x"} {
		if validPublishLockToken(bad) {
			t.Fatalf("%q must be invalid", bad)
		}
	}
	if p, ok := publishLockPath("/d", good); !ok || p != filepath.Join("/d", "publish-"+good+".lock") {
		t.Fatalf("publishLockPath = %q %v", p, ok)
	}
	if _, ok := publishLockPath("", good); ok {
		t.Fatal("an empty run dir must yield no path")
	}
	if _, ok := publishLockPath("/d", "../x"); ok {
		t.Fatal("a malformed token must yield no path")
	}
}

// TestClassifyAdmittedMutationMatrix is spec section 2's table, plus Review Focus 1–3.
func TestClassifyAdmittedMutationMatrix(t *testing.T) {
	const op = OperationWorkspacePublish
	pending, abandoned := "mutation-pending:"+op, "mutation-abandoned:"+op
	tokFree := strings.Repeat("a1", 16)
	tokHeld := strings.Repeat("b2", 16)
	tokMissing := strings.Repeat("c3", 16)
	tokDir := strings.Repeat("d4", 16)
	dir := testsupport.TempDir(t)
	lockedTempFile(t, dir, tokFree, false)
	lockedTempFile(t, dir, tokHeld, true)
	dirPath, _ := publishLockPath(dir, tokDir)
	if err := os.Mkdir(dirPath, 0o700); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name       string
		dir        string
		m          AdmittedMutation
		wantBlocks bool
		wantFind   string
	}{
		{"completed", dir, AdmittedMutation{OpKey: op, Status: mutationStatusCompleted}, false, ""},
		{"completed with a held lock", dir, AdmittedMutation{OpKey: op, Status: mutationStatusCompleted, LockToken: tokHeld}, false, ""},
		{"uncertain", dir, AdmittedMutation{OpKey: op, Status: mutationStatusUncertain}, false, abandoned},
		{"uncertain with a held lock", dir, AdmittedMutation{OpKey: op, Status: mutationStatusUncertain, LockToken: tokHeld}, false, abandoned},
		{"admitted, lock free", dir, AdmittedMutation{OpKey: op, Status: mutationStatusAdmitted, LockToken: tokFree}, false, abandoned},
		{"admitted, lock held", dir, AdmittedMutation{OpKey: op, Status: mutationStatusAdmitted, LockToken: tokHeld}, true, pending},
		{"admitted, lock file missing", dir, AdmittedMutation{OpKey: op, Status: mutationStatusAdmitted, LockToken: tokMissing}, true, pending},
		{"admitted, lock path is a directory", dir, AdmittedMutation{OpKey: op, Status: mutationStatusAdmitted, LockToken: tokDir}, true, pending},
		{"admitted, no token", dir, AdmittedMutation{OpKey: op, Status: mutationStatusAdmitted}, true, pending},
		{"admitted, malformed traversal token", dir, AdmittedMutation{OpKey: op, Status: mutationStatusAdmitted, LockToken: "../../../../../../../../tmp/xx"}, true, pending},
		{"admitted, uppercase token", dir, AdmittedMutation{OpKey: op, Status: mutationStatusAdmitted, LockToken: strings.ToUpper(tokFree)}, true, pending},
		{"admitted, empty run dir", "", AdmittedMutation{OpKey: op, Status: mutationStatusAdmitted, LockToken: tokFree}, true, pending},
		{"unknown status, lock free", dir, AdmittedMutation{OpKey: op, Status: "bogus", LockToken: tokFree}, true, pending},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			blocks, find := classifyAdmittedMutation(tc.dir, tc.m)
			if blocks != tc.wantBlocks || find != tc.wantFind {
				t.Fatalf("classify = (%v, %q), want (%v, %q)", blocks, find, tc.wantBlocks, tc.wantFind)
			}
		})
	}
}

// TestClassifyAdmittedMutationProbeErrorBlocks: a probe error (seam) blocks, and a
// malformed token never reaches the probe at all.
func TestClassifyAdmittedMutationProbeErrorBlocks(t *testing.T) {
	var probed []string
	overrideSeam(t, &publishLockProbe, func(p string) process.LockProbe {
		probed = append(probed, p)
		return process.LockProbeUnknown
	})
	dir := testsupport.TempDir(t)
	tok := strings.Repeat("e5", 16)
	lockedTempFile(t, dir, tok, false)
	m := AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusAdmitted, LockToken: tok}
	if blocks, find := classifyAdmittedMutation(dir, m); !blocks || find != "mutation-pending:"+OperationPRPublish {
		t.Fatalf("probe error classified (%v, %q), want blocking mutation-pending", blocks, find)
	}
	probed = nil
	bad := AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusAdmitted, LockToken: "../x"}
	if blocks, _ := classifyAdmittedMutation(dir, bad); !blocks {
		t.Fatal("malformed token must block")
	}
	if len(probed) != 0 {
		t.Fatalf("a malformed token reached the probe: %v", probed)
	}
}

func TestAccountAdmittedMutationsAggregates(t *testing.T) {
	dir := testsupport.TempDir(t)
	tok := strings.Repeat("f6", 16)
	lockedTempFile(t, dir, tok, false)
	muts := []AdmittedMutation{
		{OpKey: OperationPRPublish, Status: mutationStatusCompleted},
		{OpKey: OperationWorkspacePublish, Status: mutationStatusAdmitted, LockToken: tok},
		{OpKey: OperationPRPublish, Status: mutationStatusUncertain},
	}
	blocked, findings := accountAdmittedMutations(dir, muts)
	want := []string{"mutation-abandoned:" + OperationWorkspacePublish, "mutation-abandoned:" + OperationPRPublish}
	if blocked || strings.Join(findings, ",") != strings.Join(want, ",") {
		t.Fatalf("account = (%v, %v), want (false, %v)", blocked, findings, want)
	}
	blocked, findings = accountAdmittedMutations(dir, append(muts, AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusAdmitted}))
	if !blocked || findings[len(findings)-1] != "mutation-pending:"+OperationPRPublish {
		t.Fatalf("a lockless admitted entry must block: (%v, %v)", blocked, findings)
	}
}

func TestAcquirePublishLock(t *testing.T) {
	dir := testsupport.TempDir(t)
	tok, f := acquirePublishLock(dir)
	if f == nil || !validPublishLockToken(tok) {
		t.Fatalf("acquire = (%q, %v), want a valid token and a held file", tok, f)
	}
	path, _ := publishLockPath(dir, tok)
	if got := process.ProbeLock(path); got != process.LockProbeHeld {
		t.Fatalf("acquired lock probed %v, want held", got)
	}
	f.Close()
	if got := process.ProbeLock(path); got != process.LockProbeFree {
		t.Fatalf("closed lock probed %v, want free (and the file still present)", got)
	}

	t.Run("mint failure", func(t *testing.T) {
		overrideSeam(t, &publishLockMintToken, func() (string, error) { return "", errors.New("no entropy") })
		if tok, f := acquirePublishLock(dir); tok != "" || f != nil {
			t.Fatalf("mint failure = (%q, %v), want (\"\", nil)", tok, f)
		}
	})
	t.Run("acquire error", func(t *testing.T) {
		overrideSeam(t, &publishLockAcquire, func(string) (*os.File, bool, error) { return nil, false, errors.New("EIO") })
		if tok, f := acquirePublishLock(dir); tok != "" || f != nil {
			t.Fatalf("acquire error = (%q, %v), want (\"\", nil)", tok, f)
		}
	})
	t.Run("busy on a fresh token", func(t *testing.T) {
		overrideSeam(t, &publishLockAcquire, func(string) (*os.File, bool, error) { return nil, true, nil })
		if tok, f := acquirePublishLock(dir); tok != "" || f != nil {
			t.Fatalf("busy = (%q, %v), want (\"\", nil)", tok, f)
		}
	})
	t.Run("empty dir", func(t *testing.T) {
		if tok, f := acquirePublishLock(""); tok != "" || f != nil {
			t.Fatalf("empty dir = (%q, %v), want (\"\", nil)", tok, f)
		}
	})
}
```

- [ ] **Step 2: Run them and confirm they fail.**
Run: `go test -count=1 -run '^(TestValidPublishLockToken|TestClassifyAdmittedMutation|TestAccountAdmittedMutations|TestAcquirePublishLock)' ./internal/app/`
Expected: a build failure (`undefined: classifyAdmittedMutation`, `unknown field LockToken`).

- [ ] **Step 3a: Add the field.** In `internal/app/runtracker_run_record.go`, extend `AdmittedMutation`:

```go
type AdmittedMutation struct {
	OpKey       string               `json:"op_key"`
	Status      string               `json:"status"` // admitted|completed|uncertain
	Publication *MutationPublication `json:"publication,omitempty"`
	Verified    bool                 `json:"verified,omitempty"`
	// LockToken names the publisher's per-entry kernel lock,
	// <run-key-dir>/publish-<token>.lock (change 0494). The publisher holds it from
	// before this entry is written until after its outcome is written, so a free
	// lock on an admitted entry proves the publisher is gone. Empty means no lock:
	// an entry written before change 0494, a metadata transaction, or a publish
	// whose lock could not be taken. Such an entry keeps blocking while admitted.
	LockToken string `json:"lock_token,omitempty"`
}
```

- [ ] **Step 3b: Create `internal/app/runtracker_publish_lock.go`.**

```go
// Publisher liveness for the run mutation journal (change 0494). A publication
// (pr.publish / workspace.publish) holds a per-entry kernel lock from BEFORE its
// admitted entry is written until AFTER its outcome is written (admitWorkflowMutation).
// The kernel frees the lock when the publisher exits or dies, so an admitted entry
// whose lock reads free has no publisher left to write its outcome.
// classifyAdmittedMutation is the ONE rule every journal reader applies: run.cancel
// teardown (reconcileRunTeardown), terminal repair and resume
// (verifyTerminalRunQuiescence), and the success closeout (accountCompletionMutations).
// An entry blocks only while its publisher may still be running. A provably gone
// publisher, or an uncertain entry (written only by the publisher's own callback, so
// its publisher has returned), is the informational mutation-abandoned:<op>. Every
// unprovable case keeps the existing blocking mutation-pending:<op>. The lock rules
// are ADR-0132's: release by close only (never LOCK_UN), never delete a lock file,
// and only try a lock, never wait on it.
package app

import (
	"os"
	"path/filepath"

	"github.com/danielhanold/docket/internal/process"
)

// The journal finding prefixes; the operation key follows the colon.
const (
	findingMutationPending   = "mutation-pending:"
	findingMutationAbandoned = "mutation-abandoned:"
)

const (
	publishLockPrefix = "publish-"
	publishLockSuffix = ".lock"
)

// Test seams. Production values never change.
var (
	publishLockMintToken = runToken
	publishLockAcquire   = process.TryExclusiveLock
	publishLockProbe     = process.ProbeLock
)

// validPublishLockToken reports whether tok has runToken's exact shape: 32
// lowercase hex characters. A journal token of any other shape (a corrupt or
// hand-edited record) is never joined into a path.
func validPublishLockToken(tok string) bool {
	if len(tok) != 32 {
		return false
	}
	for i := 0; i < len(tok); i++ {
		c := tok[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// publishLockPath is the lock file for token in the run-key directory dir. It
// refuses (ok=false) an empty dir or a malformed token.
func publishLockPath(dir, token string) (string, bool) {
	if dir == "" || !validPublishLockToken(token) {
		return "", false
	}
	return filepath.Join(dir, publishLockPrefix+token+publishLockSuffix), true
}

// acquirePublishLock mints a fresh token and takes its lock in dir without waiting.
// Any failure (no token, no dir, an I/O error, or a busy answer on a fresh token)
// returns ("", nil). The caller then journals the entry without a token and
// publishes anyway: a lock failure never refuses a publish.
func acquirePublishLock(dir string) (string, *os.File) {
	tok, err := publishLockMintToken()
	if err != nil {
		return "", nil
	}
	path, ok := publishLockPath(dir, tok)
	if !ok {
		return "", nil
	}
	f, busy, err := publishLockAcquire(path)
	if err != nil || busy || f == nil {
		if f != nil {
			_ = f.Close()
		}
		return "", nil
	}
	return tok, f
}

// publisherGone reports whether m is an admitted entry whose publisher provably
// exited without writing an outcome: a well-formed LockToken whose lock file
// exists and reads free. A held lock, a missing file, a probe error, an empty dir,
// or no or a malformed token is NOT proof (false).
func publisherGone(dir string, m AdmittedMutation) bool {
	if m.Status != mutationStatusAdmitted {
		return false
	}
	path, ok := publishLockPath(dir, m.LockToken)
	if !ok {
		return false
	}
	return publishLockProbe(path) == process.LockProbeFree
}

// classifyAdmittedMutation is the single journal rule (spec section 2). completed:
// accounted, no finding. uncertain, or admitted with a provably gone publisher:
// accounted, mutation-abandoned:<op>. Anything else (a held lock, a missing lock
// file, a probe error, no token, or an unknown status): blocks, mutation-pending:<op>.
// It never returns an error a caller could turn into a refusal.
func classifyAdmittedMutation(dir string, m AdmittedMutation) (blocks bool, finding string) {
	switch {
	case m.Status == mutationStatusCompleted:
		return false, ""
	case m.Status == mutationStatusUncertain:
		return false, findingMutationAbandoned + m.OpKey
	case publisherGone(dir, m):
		return false, findingMutationAbandoned + m.OpKey
	default:
		return true, findingMutationPending + m.OpKey
	}
}

// accountAdmittedMutations applies classifyAdmittedMutation to every entry in order
// and reports whether any blocks, plus the findings in journal order.
func accountAdmittedMutations(dir string, muts []AdmittedMutation) (blocked bool, findings []string) {
	for _, m := range muts {
		b, f := classifyAdmittedMutation(dir, m)
		if f != "" {
			findings = append(findings, f)
		}
		if b {
			blocked = true
		}
	}
	return blocked, findings
}

// runJournalDir resolves the run-key directory the publish locks of runKey live in.
// An unresolvable directory is "", under which every token-bearing admitted entry
// stays blocking (publishLockPath refuses an empty dir).
func runJournalDir(repoDir, runKey string) string {
	dir, err := runKeyDir(repoDir, runKey, "journal-classify")
	if err != nil {
		return ""
	}
	return dir
}
```

- [ ] **Step 4: Run them and confirm they pass.** Run Step 2's command, then `go vet ./internal/app/`.
Expected: PASS, with vet clean.

- [ ] **Step 5: Mutation checks.** Each one must turn a named test red. Back up first with `cp internal/app/runtracker_publish_lock.go /tmp/rpl.go.bak`, and restore after each with `mv -f /tmp/rpl.go.bak internal/app/runtracker_publish_lock.go` (back up again before the next):
  1. In `publisherGone`, change `== process.LockProbeFree` to `!= process.LockProbeMissing`. This treats held as free. → `TestClassifyAdmittedMutationMatrix/admitted,_lock_held` FAILS.
  2. Change it to `== process.LockProbeFree || publishLockProbe(path) == process.LockProbeMissing`. This treats missing as free. → `.../admitted,_lock_file_missing` FAILS.
  3. In `publishLockPath`, delete `|| !validPublishLockToken(token)`. → `TestValidPublishLockToken` and `TestClassifyAdmittedMutationProbeErrorBlocks` FAIL.
  Restore, then re-run Step 4 and confirm it passes.

- [ ] **Step 6: Commit.**

```bash
git add internal/app/runtracker_run_record.go internal/app/runtracker_publish_lock.go internal/app/runtracker_publish_lock_test.go
git commit -m "feat(0494): publish-lock primitives and the single journal classifier"
```

---

### Task 3: Real-process proof that a SIGKILLed publisher's entry reads abandoned

**Suggested tier:** standard.

**Files:**
- Create: `internal/app/runtracker_publish_lock_holder_test.go` (untagged)
- Modify: `internal/app/gate_test.go` (`TestMain`)
- Create: `internal/app/runtracker_publish_lock_process_integration_test.go` (`//go:build integration`)

**Interfaces:**
- Consumes: `publishLockPath`, `classifyAdmittedMutation`, `accountAdmittedMutations`, `process.TryExclusiveLock` (Task 2).
- Produces: the env constants `publishLockHolderEnv` and `publishLockReadyEnv`, and `func runPublishLockHolder(lockPath, readyPath string) int`.

- [ ] **Step 1: Write the child role.** Create `internal/app/runtracker_publish_lock_holder_test.go`:

```go
package app

import (
	"os"
	"time"

	"github.com/danielhanold/docket/internal/process"
)

// The publish-lock holder re-exec role (change 0494). TestMain routes it before any
// test-suite setup. The child takes the lock at the holder env path, writes
// "ready" to the ready env path, and blocks until killed. It never closes the
// lock: only process death may free it.
const (
	publishLockHolderEnv = "DOCKET_APP_TEST_PUBLISH_LOCK_HOLDER"
	publishLockReadyEnv  = "DOCKET_APP_TEST_PUBLISH_LOCK_READY"
)

func runPublishLockHolder(lockPath, readyPath string) int {
	f, busy, err := process.TryExclusiveLock(lockPath)
	if err != nil || busy {
		return 3
	}
	if err := os.WriteFile(readyPath, []byte("ready"), 0o600); err != nil {
		return 4
	}
	for {
		time.Sleep(time.Hour)
		_ = f // keep the descriptor (and so the lock) reachable until death
	}
}
```

- [ ] **Step 2: Route the role.** In `internal/app/gate_test.go` `TestMain`, insert this immediately after the `GuardianRequested()` block and before `testsupport.RefuseUnfilteredIntegrationRun`:

```go
	// Change 0494: the publish-lock real-process test re-execs THIS binary as a
	// child that holds a publish lock until it is SIGKILLed.
	if lockPath := os.Getenv(publishLockHolderEnv); lockPath != "" {
		os.Exit(runPublishLockHolder(lockPath, os.Getenv(publishLockReadyEnv)))
	}
```

- [ ] **Step 3: Write the test.** Create `internal/app/runtracker_publish_lock_process_integration_test.go`:

```go
//go:build integration

package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/testsupport"
)

// TestIntegrationRunCancelPublishLockFreedWhenPublisherIsSIGKILLed (change 0494,
// spec test 8): a real child process holds a publish lock (a live publisher), so the
// classifier blocks with mutation-pending. After SIGKILL, with no handler possible,
// the kernel frees the lock, the classifier reports mutation-abandoned, and the lock
// file still exists.
func TestIntegrationRunCancelPublishLockFreedWhenPublisherIsSIGKILLed(t *testing.T) {
	dir := testsupport.TempDir(t)
	token := strings.Repeat("9f", 16)
	lockPath, ok := publishLockPath(dir, token)
	if !ok {
		t.Fatal("publishLockPath rejected a valid token")
	}
	ready := filepath.Join(dir, "ready")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^$")
	cmd.Env = append(os.Environ(), publishLockHolderEnv+"="+lockPath, publishLockReadyEnv+"="+ready)
	if err := cmd.Start(); err != nil {
		t.Fatalf("start holder: %v", err)
	}
	reaped := false
	t.Cleanup(func() {
		if !reaped {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	deadline := time.Now().Add(15 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("holder never reported ready")
		}
		time.Sleep(10 * time.Millisecond)
	}

	m := AdmittedMutation{OpKey: OperationWorkspacePublish, Status: mutationStatusAdmitted, LockToken: token}
	if blocks, f := classifyAdmittedMutation(dir, m); !blocks || f != "mutation-pending:"+OperationWorkspacePublish {
		t.Fatalf("live publisher classified (%v, %q), want blocking mutation-pending", blocks, f)
	}

	if err := cmd.Process.Signal(syscall.SIGKILL); err != nil {
		t.Fatalf("SIGKILL: %v", err)
	}
	_ = cmd.Wait() // reaped: the kernel has released the child's lock
	reaped = true

	if blocks, f := classifyAdmittedMutation(dir, m); blocks || f != "mutation-abandoned:"+OperationWorkspacePublish {
		t.Fatalf("killed publisher classified (%v, %q), want non-blocking mutation-abandoned", blocks, f)
	}
	if blocked, fs := accountAdmittedMutations(dir, []AdmittedMutation{m}); blocked || len(fs) != 1 {
		t.Fatalf("accountAdmittedMutations = (%v, %v), want one non-blocking finding", blocked, fs)
	}
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("the lock file must survive its holder's death: %v", err)
	}
}
```

- [ ] **Step 4: Run it and confirm it passes.**
Run: `go test -tags integration -count=1 -run '^TestIntegrationRunCancelPublishLockFreedWhenPublisherIsSIGKILLed$' ./internal/app/`
Expected: PASS. Then confirm that TestMain routing did not break the default build: `go test -count=1 -run '^TestClassifyAdmittedMutationMatrix$' ./internal/app/` → PASS.

- [ ] **Step 5: Mutation check.** Back up `runtracker_publish_lock_holder_test.go`, then make the child close the lock before writing ready: insert `f.Close()` right after the busy check. Run Step 4's command. Expected: FAIL ("live publisher classified … want blocking"). Restore the file from the backup and re-run.

- [ ] **Step 6: Commit.**

```bash
git add internal/app/runtracker_publish_lock_holder_test.go internal/app/gate_test.go internal/app/runtracker_publish_lock_process_integration_test.go
git commit -m "test(0494): a SIGKILLed publisher's lock frees and its entry reads abandoned"
```

---

### Task 4: The publisher holds its lock across the remote call (admission)

**Suggested tier:** premium (lock ordering and concurrency; a mistake silently re-opens the wedge or creates a false "abandoned").

**Files:**
- Modify: `internal/app/runtracker_fence.go` (`admitWorkflowMutation`, the `mutationJournalDone` doc, and the status-constant doc)
- Modify: `internal/app/pr_publish_test.go` (`fakeGitHub`), `internal/app/workspace_ops_test.go` (`fakeWorkspaceService`)
- Modify: `internal/app/runtracker_publication_integration_test.go` (keep discarded in-flight callbacks alive; see Step 6)
- Create: `internal/app/runtracker_publish_lock_helpers_integration_test.go` (`//go:build integration`)
- Create: `internal/app/runtracker_fence_publish_lock_integration_test.go` (`//go:build integration`)

**Interfaces:**
- Consumes: `acquirePublishLock`, `publishLockPath`, `publisherGone`, `publishLockAcquire`, `publishLockMintToken`, `overrideSeam`, `runKeyDir`, `acquireRunLock(dir string) (*os.File, error)`, `settleUncertainPublications`, `mintFenceRun`, `newWorkingRepo`, `prRepo`, `prReader`, `prHead`, `prEvidenceBytes`, `prMatchPR`, `readyService`, `workspaceDepsFor`, `fakeReader`, `mainPin`, and `inProgressChangeBlob` (existing test helpers).
- Produces:
  - `admitWorkflowMutation` keeps its exact signature `func(repoDir, op string, pub *MutationPublication) (mutationJournalDone, error)`. The `repoguard` admission guard keys on it.
  - fake hooks: `fakeGitHub.onEnsure func()` and `fakeWorkspaceService.onPublish func()`, each called first inside `EnsurePullRequest` / `PublishHead`.
  - integration helpers (Tasks 5–6 use them): `seedPublishLock(t, repo, key string, hold bool) (token string, release func())`, `seedJournal(t, repo, key string, muts ...AdmittedMutation)`, `journalEntry(t, repo, key string, i int) AdmittedMutation`, `publishLockPathFor(t, repo, key, token string) string`, `lockTestWSDesc() MutationPublication`, and `lockTestPRDesc() MutationPublication`.

- [ ] **Step 1: Add the fake hooks.** In `internal/app/pr_publish_test.go`, add the field `onEnsure func()` to `fakeGitHub`, and make `EnsurePullRequest` start with `if f.onEnsure != nil { f.onEnsure() }`. In `internal/app/workspace_ops_test.go`, add `onPublish func()` to `fakeWorkspaceService`, and make `PublishHead` start with `if f.onPublish != nil { f.onPublish() }`.

- [ ] **Step 2: Create the shared integration helpers** in `internal/app/runtracker_publish_lock_helpers_integration_test.go`:

```go
//go:build integration

package app

import (
	"strings"
	"sync"
	"testing"

	"github.com/danielhanold/docket/internal/process"
)

// seedPublishLock creates a real publish lock file in key's run-key directory and
// returns its token. hold=true keeps it held by this test process (a live
// publisher) until release runs; release is also registered with t.Cleanup.
// hold=false leaves the file present and free: what a publisher that died
// mid-flight leaves behind.
func seedPublishLock(t *testing.T, repo, key string, hold bool) (string, func()) {
	t.Helper()
	dir, err := runKeyDir(repo, key, "test-seed-publish-lock")
	if err != nil {
		t.Fatalf("runKeyDir: %v", err)
	}
	token, err := runToken()
	if err != nil {
		t.Fatalf("runToken: %v", err)
	}
	path, ok := publishLockPath(dir, token)
	if !ok {
		t.Fatalf("publishLockPath rejected minted token %q", token)
	}
	f, busy, err := process.TryExclusiveLock(path)
	if err != nil || busy {
		t.Fatalf("TryExclusiveLock: busy=%v err=%v", busy, err)
	}
	if !hold {
		_ = f.Close()
		return token, func() {}
	}
	var once sync.Once
	release := func() { once.Do(func() { _ = f.Close() }) }
	t.Cleanup(release)
	return token, release
}

// seedJournal replaces key's admitted-mutation journal with muts.
func seedJournal(t *testing.T, repo, key string, muts ...AdmittedMutation) {
	t.Helper()
	if err := runRecordCAS(repo, key, func(r *RunRecord) error {
		r.AdmittedMutations = append([]AdmittedMutation(nil), muts...)
		return nil
	}); err != nil {
		t.Fatalf("seed journal: %v", err)
	}
}

// journalEntry loads key's run record and returns journal entry i.
func journalEntry(t *testing.T, repo, key string, i int) AdmittedMutation {
	t.Helper()
	ep, _, err := LoadRunRecord(repo, key)
	if err != nil {
		t.Fatalf("LoadRunRecord: %v", err)
	}
	if i >= len(ep.AdmittedMutations) {
		t.Fatalf("journal has %d entries, want index %d", len(ep.AdmittedMutations), i)
	}
	return ep.AdmittedMutations[i]
}

// publishLockPathFor is the lock path token names under key's run-key directory.
func publishLockPathFor(t *testing.T, repo, key, token string) string {
	t.Helper()
	dir, err := runKeyDir(repo, key, "test-publish-lock-path")
	if err != nil {
		t.Fatalf("runKeyDir: %v", err)
	}
	path, ok := publishLockPath(dir, token)
	if !ok {
		t.Fatalf("publishLockPath rejected %q", token)
	}
	return path
}

func lockTestWSDesc() MutationPublication {
	return MutationPublication{RepoDir: "/repo/.git", Remote: "origin",
		HeadRef: "refs/heads/fix/w", HeadCommit: strings.Repeat("a", 40)}
}

func lockTestPRDesc() MutationPublication {
	return MutationPublication{RepoHost: "github.com", RepoOwner: "o", RepoName: "r",
		HeadRef: "fix/w", HeadCommit: strings.Repeat("a", 40), BaseBranch: "main",
		TitleDigest: publicationDigest("pr-title", "t"), BodyDigest: publicationDigest("pr-body", "b")}
}
```

- [ ] **Step 3: Write the failing admission tests.** Create `internal/app/runtracker_fence_publish_lock_integration_test.go`:

```go
//go:build integration

package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/githubcli"
	"github.com/danielhanold/docket/internal/process"
	"github.com/danielhanold/docket/internal/workspace"
)

// lockStateDuringCall records, from inside the adapter call, the journal entry's
// status, its token, and its lock probe.
type lockStateDuringCall struct {
	status, token string
	probe         process.LockProbe
	seen          bool
}

func captureLockState(t *testing.T, repo, key string, out *lockStateDuringCall) func() {
	return func() {
		ep, _, err := LoadRunRecord(repo, key)
		if err != nil || len(ep.AdmittedMutations) == 0 {
			t.Errorf("load inside adapter: %v (entries %d)", err, len(ep.AdmittedMutations))
			return
		}
		m := ep.AdmittedMutations[len(ep.AdmittedMutations)-1]
		out.status, out.token, out.seen = m.Status, m.LockToken, true
		if dir, derr := runKeyDir(repo, key, "test"); derr == nil {
			if p, ok := publishLockPath(dir, m.LockToken); ok {
				out.probe = process.ProbeLock(p)
			}
		}
	}
}

func assertLockAcrossCall(t *testing.T, repo, key string, during lockStateDuringCall) {
	t.Helper()
	if !during.seen || during.status != mutationStatusAdmitted || !validPublishLockToken(during.token) {
		t.Fatalf("during the remote call: %+v, want an admitted entry with a valid lock_token", during)
	}
	if during.probe != process.LockProbeHeld {
		t.Fatalf("lock probed %v during the remote call, want held", during.probe)
	}
	m := journalEntry(t, repo, key, 0)
	if m.Status != mutationStatusCompleted || m.LockToken != during.token {
		t.Fatalf("after the call: %+v, want completed with the same token", m)
	}
	path := publishLockPathFor(t, repo, key, during.token)
	if got := process.ProbeLock(path); got != process.LockProbeFree {
		t.Fatalf("lock probed %v after the callback, want free", got)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("lock file must never be deleted: %v", err)
	}
}

// TestIntegrationRunFencePRPublishHoldsPublishLockAcrossRemoteCall (spec test 7).
func TestIntegrationRunFencePRPublishHoldsPublishLockAcrossRemoteCall(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	key := mintFenceRun(t, repoDir, repoDir, RunActive)
	var during lockStateDuringCall
	gh := &fakeGitHub{repo: prRepo(), ensureRes: githubcli.EnsureResult{Disposition: githubcli.EnsureCreated, PR: prMatchPR("verified")}}
	gh.onEnsure = captureLockState(t, repoDir, key, &during)
	res := PRPublish(context.Background(), workspaceDepsFor(t, prReader(t)), WorkspaceDeps{Service: readyService(prHead)},
		GitHubDeps{Service: gh}, repoDir, PRPublishRequest{ID: 7, Head: prHead, Title: "t", Body: "b\n", EvidenceRecord: prEvidenceBytes(t, prHead)})
	if res.Result != ResultApplied {
		t.Fatalf("result = %q (reason %q), want applied", res.Result, res.Reason)
	}
	assertLockAcrossCall(t, repoDir, key, during)
}

// TestIntegrationRunFenceWorkspacePublishHoldsPublishLockAcrossRemoteCall (spec test 7).
func TestIntegrationRunFenceWorkspacePublishHoldsPublishLockAcrossRemoteCall(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	key := mintFenceRun(t, repoDir, repoDir, RunActive)
	const head = "abcdef0000000000000000000000000000000000"
	var during lockStateDuringCall
	svc := &fakeWorkspaceService{
		inspection: workspace.Inspection{Kind: workspace.StateReady, HeadCommit: gitcli.ObjectID(head)},
		publishRes: workspace.PublishResult{Disposition: workspace.PublishPublished, Head: gitcli.ObjectID(head)},
	}
	svc.onPublish = captureLockState(t, repoDir, key, &during)
	reader := &fakeReader{pin: mainPin(t), corpus: []StatusBlob{inProgressChangeBlob(7, "widget", "v7", "")}}
	res := WorkspacePublish(context.Background(), workspaceDepsFor(t, reader), WorkspaceDeps{Service: svc},
		repoDir, WorkspacePublishRequest{ID: 7, Head: head})
	if res.Result != ResultApplied {
		t.Fatalf("result = %q (reason %q), want applied", res.Result, res.Reason)
	}
	assertLockAcrossCall(t, repoDir, key, during)
}

// TestIntegrationRunFencePublishLockFailureStillPublishes (spec test 7): a lock that
// cannot be taken never refuses the publish. The entry is journaled with no token
// and resolves exactly as before change 0494.
func TestIntegrationRunFencePublishLockFailureStillPublishes(t *testing.T) {
	const head = "abcdef0000000000000000000000000000000000"
	cases := map[string]func(t *testing.T){
		"acquire error": func(t *testing.T) {
			overrideSeam(t, &publishLockAcquire, func(string) (*os.File, bool, error) { return nil, false, errors.New("EIO") })
		},
		"busy on a fresh token": func(t *testing.T) {
			overrideSeam(t, &publishLockAcquire, func(string) (*os.File, bool, error) { return nil, true, nil })
		},
		"token mint failure": func(t *testing.T) {
			overrideSeam(t, &publishLockMintToken, func() (string, error) { return "", errors.New("no entropy") })
		},
	}
	for name, arm := range cases {
		t.Run(name, func(t *testing.T) {
			arm(t)
			repoDir := newWorkingRepo(t, nil).invocation
			key := mintFenceRun(t, repoDir, repoDir, RunActive)
			svc := &fakeWorkspaceService{
				inspection: workspace.Inspection{Kind: workspace.StateReady, HeadCommit: gitcli.ObjectID(head)},
				publishRes: workspace.PublishResult{Disposition: workspace.PublishPublished, Head: gitcli.ObjectID(head)},
			}
			reader := &fakeReader{pin: mainPin(t), corpus: []StatusBlob{inProgressChangeBlob(7, "widget", "v7", "")}}
			res := WorkspacePublish(context.Background(), workspaceDepsFor(t, reader), WorkspaceDeps{Service: svc},
				repoDir, WorkspacePublishRequest{ID: 7, Head: head})
			if res.Result != ResultApplied || len(svc.publishCalls) != 1 {
				t.Fatalf("result = %q, publish calls = %d; a lock failure must never refuse the publish", res.Result, len(svc.publishCalls))
			}
			if m := journalEntry(t, repoDir, key, 0); m.LockToken != "" || m.Status != mutationStatusCompleted {
				t.Fatalf("entry = %+v, want completed with no lock_token", m)
			}
		})
	}
}

// TestIntegrationRunFenceRefusedPublishReleasesItsLock (spec test 7): a fence
// refusal from the admission CAS closes the lock taken before it. No entry is
// journaled, the lock reads free, and the file is never deleted.
func TestIntegrationRunFenceRefusedPublishReleasesItsLock(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	key := mintFenceRun(t, repoDir, repoDir, RunCancelling)
	desc := lockTestWSDesc()
	done, err := admitWorkflowMutation(repoDir, OperationWorkspacePublish, &desc)
	if !errors.Is(err, ErrRunCancelled) || done != nil {
		t.Fatalf("admit = (%v, %v), want (nil, ErrRunCancelled)", done != nil, err)
	}
	dir, derr := runKeyDir(repoDir, key, "test")
	if derr != nil {
		t.Fatal(derr)
	}
	locks, _ := filepath.Glob(filepath.Join(dir, "publish-*.lock"))
	if len(locks) != 1 {
		t.Fatalf("publish locks = %v, want exactly the one taken before the CAS", locks)
	}
	if got := process.ProbeLock(locks[0]); got != process.LockProbeFree {
		t.Fatalf("refused publish left its lock %v, want free", got)
	}
	f, busy, terr := process.TryExclusiveLock(locks[0])
	if terr != nil || busy {
		t.Fatalf("TryExclusiveLock after refusal: busy=%v err=%v", busy, terr)
	}
	f.Close()
	if ep, _, _ := LoadRunRecord(repoDir, key); len(ep.AdmittedMutations) != 0 {
		t.Fatalf("a refused admission journaled %+v", ep.AdmittedMutations)
	}
}

// TestIntegrationRunFenceOutcomeWriteLandsBeforePublishLockRelease (spec tests 7 and 9):
// while the outcome write waits on run.lock, the publish lock stays held, and a
// settler's probe reads it busy. A settler running under a live publisher's lock
// leaves the entry admitted. Neither ordering deadlocks.
func TestIntegrationRunFenceOutcomeWriteLandsBeforePublishLockRelease(t *testing.T) {
	repoDir := newWorkingRepo(t, nil).invocation
	key := mintFenceRun(t, repoDir, repoDir, RunActive)
	dir, err := runKeyDir(repoDir, key, "test")
	if err != nil {
		t.Fatal(err)
	}
	desc := lockTestWSDesc()

	// (a) A settler under a live publisher's lock: it probes busy, leaves the entry
	// admitted, and releases run.lock; then the publisher's outcome write completes.
	done1, err := admitWorkflowMutation(repoDir, OperationWorkspacePublish, &desc)
	if err != nil {
		t.Fatal(err)
	}
	finishedA := make(chan struct{})
	go func() {
		defer close(finishedA)
		settled, findings := settleUncertainPublications(repoDir, key)
		if len(settled) != 0 || len(findings) != 0 {
			t.Errorf("settler under a live publisher: settled=%v findings=%v, want none", settled, findings)
		}
	}()
	select {
	case <-finishedA:
	case <-time.After(10 * time.Second):
		t.Fatal("settler deadlocked against a live publisher's lock")
	}
	if m := journalEntry(t, repoDir, key, 0); m.Status != mutationStatusAdmitted {
		t.Fatalf("settler rewrote a live publisher's entry to %q", m.Status)
	}
	done1(mutationStatusCompleted, true)
	if m := journalEntry(t, repoDir, key, 0); m.Status != mutationStatusCompleted || !m.Verified {
		t.Fatalf("publisher outcome = %+v, want completed+verified", m)
	}

	// (b) Ordering: hold run.lock (a settler mid-CAS); the publisher's callback blocks
	// on it for its outcome write, and its publish lock must stay held throughout.
	done2, err := admitWorkflowMutation(repoDir, OperationWorkspacePublish, &desc)
	if err != nil {
		t.Fatal(err)
	}
	entry := journalEntry(t, repoDir, key, 1)
	path, ok := publishLockPath(dir, entry.LockToken)
	if !ok {
		t.Fatalf("entry carries no valid lock_token: %+v", entry)
	}
	runLock, err := acquireRunLock(dir)
	if err != nil {
		t.Fatal(err)
	}
	finishedB := make(chan struct{})
	go func() {
		defer close(finishedB)
		done2(mutationStatusCompleted, true)
	}()
	for i := 0; i < 50; i++ { // ~500ms: a release-before-write would surface here
		if got := process.ProbeLock(path); got != process.LockProbeHeld {
			runLock.Close()
			t.Fatalf("publish lock %v while the outcome write is still blocked: released before the outcome was written", got)
		}
		if publisherGone(dir, entry) {
			runLock.Close()
			t.Fatal("a settler under run.lock read the live publisher as gone")
		}
		time.Sleep(10 * time.Millisecond)
	}
	runLock.Close()
	select {
	case <-finishedB:
	case <-time.After(10 * time.Second):
		t.Fatal("outcome write deadlocked after run.lock was released")
	}
	if m := journalEntry(t, repoDir, key, 1); m.Status != mutationStatusCompleted {
		t.Fatalf("outcome = %q, want completed", m.Status)
	}
	if got := process.ProbeLock(path); got != process.LockProbeFree {
		t.Fatalf("lock %v after the callback returned, want free", got)
	}
}
```

- [ ] **Step 4: Run them and confirm they fail.**
Run: `go test -tags integration -count=1 -run '^TestIntegrationRunFence(PRPublishHoldsPublishLock|WorkspacePublishHoldsPublishLock|PublishLockFailureStillPublishes|RefusedPublishReleasesItsLock|OutcomeWriteLandsBeforePublishLockRelease)' ./internal/app/`
Expected: FAIL. During the call there is no valid `lock_token`, and the refusal test finds zero locks.

- [ ] **Step 5: Implement in `admitWorkflowMutation`** (`internal/app/runtracker_fence.go`). After the `if !found { ... }` block and before the CAS, insert:

```go
	// Publisher liveness (change 0494). A publication takes a per-entry kernel lock
	// BEFORE its admitted entry becomes visible and releases it only AFTER its
	// outcome is written (the done callback below), so admitted-with-a-free-lock
	// proves the publisher exited without recording an outcome
	// (classifyAdmittedMutation). A lock that cannot be taken never refuses the
	// publish: the entry is journaled without a token and behaves exactly as before.
	// Metadata transactions (pub == nil) take no lock.
	var (
		lockToken string
		lockFile  *os.File
	)
	if pub != nil {
		if dir, derr := runKeyDir(repoDir, runKey, "publish-lock"); derr == nil {
			lockToken, lockFile = acquirePublishLock(dir)
		}
	}
	releaseLock := func() {
		if lockFile != nil {
			_ = lockFile.Close() // release by close only, never LOCK_UN (ADR-0132)
			lockFile = nil
		}
	}
```

In the `case RunActive:` append, add `LockToken: lockToken,` to the `AdmittedMutation` literal. Change the refusal return to:

```go
	if cerr != nil {
		releaseLock() // a fence refusal leaves no lock held
		return nil, cerr
	}
```

Make the done callback release last, on every path:

```go
	done := func(status string, verified bool) {
		// Outcome FIRST, release LAST (change 0494): the publish lock is closed only
		// after the outcome write returns, even when that write fails, so a reader
		// never sees admitted-with-a-free-lock for a publisher that is still writing.
		defer releaseLock()
		_ = runRecordCAS(repoDir, runKey, func(rec *RunRecord) error {
			// (existing body unchanged)
		})
	}
```

Update the docs in the same file:
- The `mutationJournalDone` doc comment gains: "For a publication it also releases the publisher's per-entry lock, after the outcome write and on every path (change 0494)."
- The `admitWorkflowMutation` "Owning run ACTIVE" bullet gains: "a publication (pub != nil) first takes its per-entry publish lock and journals its token (change 0494)."
- The status-constant comment block above `mutationStatusAdmitted` replaces "but only `completed` lets a cancellation reach `cancelled`" with: "`completed` is accounted; `uncertain`, or `admitted` whose publisher provably exited (a free publish lock, change 0494), is accounted as the informational mutation-abandoned; any other entry keeps a cancellation pending (classifyAdmittedMutation)."

- [ ] **Step 6: Keep in-flight test callbacks alive.** A discarded `done` closure lets the GC finalize its `*os.File`, which closes the lock. That would make a still-in-flight test admission read as a dead publisher once Task 5's settle step lands, and the race test would fail intermittently. Find every test admission with a non-nil publication whose `done` is discarded or never called:
Run: `grep -n "admitWorkflowMutation(" internal/app/*_test.go | grep -v ", nil)"`
For each hit whose callback is `_` or is never invoked, collect it in a slice, and keep it alive until the point where the test needs the publisher alive. Known hits are the unrelated admission and the four racers in `TestRaceIntegrationAppConcurrencySettlementNeverDowngradesUnderRacingCallback` (`internal/app/runtracker_publication_integration_test.go`). There, declare `var inflight []mutationJournalDone` (guard it with the existing `mu` for the racer goroutines), append each discarded callback, and add `runtime.KeepAlive(inflight)` after the round's final assertions, inside the loop. Add the import `runtime`. Do not call those callbacks: the test asserts they stay admitted.

- [ ] **Step 7: Run and confirm everything passes.**
Run Step 4's command → PASS. Then run the shards this change touches:
`bash tests/test_go_integration_app_runfence.sh && bash tests/test_go_integration_app_runcompletion.sh && bash tests/test_go_integration_app_runcancel.sh && bash tests/test_go_integration_app_concurrency.sh`
Expected: all `ok`. Existing production-path tests now journal tokens, and their locks are released, so no blocking behavior changes.

- [ ] **Step 8: Mutation check (spec test 10, fourth bullet).** Back up `internal/app/runtracker_fence.go`. In the done callback, replace `defer releaseLock()` with a plain `releaseLock()` call as the callback's first statement, so the lock closes before the outcome write. Run `go test -tags integration -count=1 -run '^TestIntegrationRunFenceOutcomeWriteLandsBeforePublishLockRelease$' ./internal/app/`. Expected: FAIL ("released before the outcome was written"). Restore from the backup and re-run → PASS.

- [ ] **Step 9: Commit.**

```bash
git add internal/app/runtracker_fence.go internal/app/pr_publish_test.go internal/app/workspace_ops_test.go \
  internal/app/runtracker_publish_lock_helpers_integration_test.go internal/app/runtracker_fence_publish_lock_integration_test.go \
  internal/app/runtracker_publication_integration_test.go
git commit -m "feat(0494): a publication holds its per-entry lock from admission until its outcome is written"
```

---

### Task 5: Cancel, terminal repair, and resume use the classifier; settlement records abandonment

**Suggested tier:** premium (it reverses 0444's blocking premise across several tests; the risk is a wrong unblock).

**Files:**
- Modify: `internal/app/runtracker_publication.go` (`settleUncertainPublications`)
- Modify: `internal/app/runtracker_cancel.go` (`reconcileRunTeardown` step (7), `verifyTerminalRunQuiescence`, and their doc comments)
- Create: `internal/app/runtracker_cancel_publish_lock_integration_test.go` (`//go:build integration`)
- Rewrite (do not delete):
  - `TestIntegrationRunCancelStaysPendingWithoutCompletedIdenticalRetry` (`runtracker_cancel_integration_test.go`)
  - `TestIntegrationRunCompletionReadOnlyPathsNeverSettle` (`runtracker_complete_integration_test.go`)
  - the helper `assertUnverifiedRetryLeavesOriginalPending` and its two callers (`runtracker_fence_integration_test.go`)
  - `TestIntegrationRunCompletionSettlementInterruptionConverges` part (b) (`runtracker_publication_integration_test.go`)

**Interfaces:**
- Consumes: `accountAdmittedMutations`, `runJournalDir`, `publisherGone`, `publishLockProbe`, `overrideSeam` (Task 2); `seedPublishLock`, `seedJournal`, `journalEntry`, `publishLockPathFor`, `lockTestWSDesc` (Task 4); and the existing `newCancelFixture`, `fakeCancelStopper`, `okLaunchReconciler`, `countFinding`, `hasFinding`, `loadRunState`, `validateResumeQuiescence`, `SupersedeCancelledRun`, `admitWorkflowMutation`.
- Produces: no new names. `settleUncertainPublications(repoDir, runKey string) (settled, findings []string)` keeps its signature.

- [ ] **Step 1: Write the new failing cancel tests.** Create `internal/app/runtracker_cancel_publish_lock_integration_test.go`:

```go
//go:build integration

package app

import (
	"os"
	"testing"

	"github.com/danielhanold/docket/internal/process"
)

func lockCancelSeams(fx cancelFixture) cancelSeams {
	return cancelSeams{store: fx.store, stopper: &fakeCancelStopper{proven: map[string]bool{fx.runDir: true}}, launches: okLaunchReconciler()}
}

// TestIntegrationRunCancelKilledPublisherIsAbandoned (spec test 1, Review Focus 5).
func TestIntegrationRunCancelKilledPublisherIsAbandoned(t *testing.T) {
	fx := newCancelFixture(t)
	desc := lockTestWSDesc()
	token, _ := seedPublishLock(t, fx.repo, fx.key, false)
	seedJournal(t, fx.repo, fx.key, AdmittedMutation{OpKey: OperationWorkspacePublish, Status: mutationStatusAdmitted, Publication: &desc, LockToken: token})

	res := runCancel(lockCancelSeams(fx), fx.repo, fx.key, "human stop")
	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("disposition = %q (findings %v), want cancelled", res.Disposition, res.Findings)
	}
	if countFinding(res.Findings, "mutation-abandoned:"+OperationWorkspacePublish) != 1 || hasFinding(res.Findings, "mutation-pending") {
		t.Fatalf("findings = %v, want exactly one mutation-abandoned and no mutation-pending", res.Findings)
	}
	if m := journalEntry(t, fx.repo, fx.key, 0); m.Status != mutationStatusUncertain || m.Verified || m.LockToken != token {
		t.Fatalf("stored entry = %+v, want uncertain, unverified, same token", m)
	}
	if _, err := os.Stat(publishLockPathFor(t, fx.repo, fx.key, token)); err != nil {
		t.Fatalf("lock file must never be deleted: %v", err)
	}
	// Repeat: terminal, quiescent, and no second rewrite.
	again := runCancel(lockCancelSeams(fx), fx.repo, fx.key, "human stop")
	if again.Disposition != CancelDispositionAlreadyCancelled || countFinding(again.Findings, "mutation-abandoned:"+OperationWorkspacePublish) != 1 {
		t.Fatalf("repeat = %q %v, want already-cancelled with the informational finding", again.Disposition, again.Findings)
	}
	ep, _, err := LoadRunRecord(fx.repo, fx.key)
	if err != nil {
		t.Fatal(err)
	}
	if ok, detail := validateResumeQuiescence(cancelSeams{store: fx.store, launches: okLaunchReconciler()}, fx.repo, ep); !ok {
		t.Fatalf("resume quiescence = %q, want quiescent", detail)
	}
	if err := SupersedeCancelledRun(fx.repo, fx.key, "replacement-key"); err != nil {
		t.Fatalf("SupersedeCancelledRun: %v (resume must admit exactly one replacement)", err)
	}
}

// TestIntegrationRunCancelLivePublisherStaysPending (spec test 3, cancel half).
func TestIntegrationRunCancelLivePublisherStaysPending(t *testing.T) {
	fx := newCancelFixture(t)
	desc := lockTestWSDesc()
	token, release := seedPublishLock(t, fx.repo, fx.key, true)
	seedJournal(t, fx.repo, fx.key, AdmittedMutation{OpKey: OperationWorkspacePublish, Status: mutationStatusAdmitted, Publication: &desc, LockToken: token})

	first := runCancel(lockCancelSeams(fx), fx.repo, fx.key, "human stop")
	if first.Disposition != CancelDispositionPending || !hasFinding(first.Findings, "mutation-pending:"+OperationWorkspacePublish) ||
		hasFinding(first.Findings, "mutation-abandoned") {
		t.Fatalf("live publisher: %q %v, want cancellation-pending with mutation-pending only", first.Disposition, first.Findings)
	}
	if m := journalEntry(t, fx.repo, fx.key, 0); m.Status != mutationStatusAdmitted {
		t.Fatalf("settlement touched a live publisher's entry: %q", m.Status)
	}
	release()
	second := runCancel(lockCancelSeams(fx), fx.repo, fx.key, "human stop")
	if second.Disposition != CancelDispositionCancelled || !hasFinding(second.Findings, "mutation-abandoned:"+OperationWorkspacePublish) {
		t.Fatalf("after release: %q %v, want cancelled with mutation-abandoned", second.Disposition, second.Findings)
	}
}

// TestIntegrationRunCancelUnprovableJournalEntriesBlock (spec test 4, Review Focus 1–2):
// every entry docket cannot prove abandoned blocks exactly as before change 0494.
func TestIntegrationRunCancelUnprovableJournalEntriesBlock(t *testing.T) {
	cases := []struct {
		name string
		seed func(t *testing.T, fx cancelFixture) AdmittedMutation
	}{
		{"no lock token", func(t *testing.T, fx cancelFixture) AdmittedMutation {
			return AdmittedMutation{OpKey: OperationWorkspacePublish, Status: mutationStatusAdmitted}
		}},
		{"lock file missing", func(t *testing.T, fx cancelFixture) AdmittedMutation {
			tok, err := runToken()
			if err != nil {
				t.Fatal(err)
			}
			return AdmittedMutation{OpKey: OperationWorkspacePublish, Status: mutationStatusAdmitted, LockToken: tok}
		}},
		{"probe error", func(t *testing.T, fx cancelFixture) AdmittedMutation {
			tok, _ := seedPublishLock(t, fx.repo, fx.key, false)
			overrideSeam(t, &publishLockProbe, func(string) process.LockProbe { return process.LockProbeUnknown })
			return AdmittedMutation{OpKey: OperationWorkspacePublish, Status: mutationStatusAdmitted, LockToken: tok}
		}},
		{"lock path is a directory", func(t *testing.T, fx cancelFixture) AdmittedMutation {
			tok, err := runToken()
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(publishLockPathFor(t, fx.repo, fx.key, tok), 0o700); err != nil {
				t.Fatal(err)
			}
			return AdmittedMutation{OpKey: OperationWorkspacePublish, Status: mutationStatusAdmitted, LockToken: tok}
		}},
		{"malformed token", func(t *testing.T, fx cancelFixture) AdmittedMutation {
			return AdmittedMutation{OpKey: OperationWorkspacePublish, Status: mutationStatusAdmitted, LockToken: "../../../../../../../../tmp/xx"}
		}},
		{"unknown status with a free lock", func(t *testing.T, fx cancelFixture) AdmittedMutation {
			tok, _ := seedPublishLock(t, fx.repo, fx.key, false)
			return AdmittedMutation{OpKey: OperationWorkspacePublish, Status: "bogus", LockToken: tok}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fx := newCancelFixture(t)
			m := tc.seed(t, fx)
			seedJournal(t, fx.repo, fx.key, m)
			res := runCancel(lockCancelSeams(fx), fx.repo, fx.key, "human stop")
			if res.Disposition != CancelDispositionPending || !hasFinding(res.Findings, "mutation-pending:"+OperationWorkspacePublish) ||
				hasFinding(res.Findings, "mutation-abandoned") {
				t.Fatalf("%q %v, want cancellation-pending with mutation-pending only", res.Disposition, res.Findings)
			}
			if got := journalEntry(t, fx.repo, fx.key, 0); got.Status != m.Status {
				t.Fatalf("entry status = %q, want untouched %q", got.Status, m.Status)
			}
		})
	}
}

// TestIntegrationRunCancelKilledPublisherThenVerifiedRetrySettles (spec test 5, cancel).
func TestIntegrationRunCancelKilledPublisherThenVerifiedRetrySettles(t *testing.T) {
	fx := newCancelFixture(t)
	desc := lockTestWSDesc()
	token, _ := seedPublishLock(t, fx.repo, fx.key, false)
	seedJournal(t, fx.repo, fx.key,
		AdmittedMutation{OpKey: OperationWorkspacePublish, Status: mutationStatusAdmitted, Publication: &desc, LockToken: token},
		AdmittedMutation{OpKey: OperationWorkspacePublish, Status: mutationStatusCompleted, Verified: true, Publication: &desc})
	res := runCancel(lockCancelSeams(fx), fx.repo, fx.key, "human stop")
	if res.Disposition != CancelDispositionCancelled {
		t.Fatalf("disposition = %q (findings %v), want cancelled", res.Disposition, res.Findings)
	}
	if countFinding(res.Findings, "mutation-settled:"+OperationWorkspacePublish) != 1 || hasFinding(res.Findings, "mutation-abandoned") {
		t.Fatalf("findings = %v, want mutation-settled and no mutation-abandoned", res.Findings)
	}
	if m := journalEntry(t, fx.repo, fx.key, 0); m.Status != mutationStatusCompleted || m.Verified {
		t.Fatalf("original = %+v, want completed and still unverified", m)
	}
}

// TestIntegrationRunCancelPublisherFinishingAfterFenceIsNotAbandoned (Review Focus 4):
// cancel wins while a live publisher is mid-call, and the publisher's own callback
// then resolves it.
func TestIntegrationRunCancelPublisherFinishingAfterFenceIsNotAbandoned(t *testing.T) {
	fx := newCancelFixture(t)
	desc := lockTestWSDesc()
	done, err := admitWorkflowMutation(fx.worktree, OperationWorkspacePublish, &desc)
	if err != nil {
		t.Fatalf("admit: %v", err)
	}
	first := runCancel(lockCancelSeams(fx), fx.repo, fx.key, "human stop")
	if first.Disposition != CancelDispositionPending || !hasFinding(first.Findings, "mutation-pending:"+OperationWorkspacePublish) {
		t.Fatalf("mid-call cancel = %q %v, want pending on the live publisher", first.Disposition, first.Findings)
	}
	done(mutationStatusCompleted, true)
	second := runCancel(lockCancelSeams(fx), fx.repo, fx.key, "human stop")
	if second.Disposition != CancelDispositionCancelled || hasFinding(second.Findings, "mutation-abandoned") || hasFinding(second.Findings, "mutation-pending") {
		t.Fatalf("after the publisher finished: %q %v, want cancelled with no journal finding", second.Disposition, second.Findings)
	}
	if m := journalEntry(t, fx.repo, fx.key, 0); m.Status != mutationStatusCompleted || !m.Verified {
		t.Fatalf("entry = %+v, want the publisher's own completed+verified outcome", m)
	}
}
```

- [ ] **Step 2: Run them and confirm they fail.**
Run: `go test -tags integration -count=1 -run '^TestIntegrationRunCancel(KilledPublisher|LivePublisherStaysPending|UnprovableJournalEntriesBlock|PublisherFinishingAfterFence)' ./internal/app/`
Expected: FAIL. The killed-publisher cases report `cancellation-pending` with `mutation-pending`.

- [ ] **Step 3a: Implement the settle step** (`internal/app/runtracker_publication.go`, `settleUncertainPublications`). Resolve the directory once before the CAS, and record abandonment inside it before the 0444 match:

```go
func settleUncertainPublications(repoDir, runKey string) (settled, findings []string) {
	dir := runJournalDir(repoDir, runKey) // "" ⇒ no entry can be proven abandoned
	var tokens []string
	err := runRecordCAS(repoDir, runKey, func(rec *RunRecord) error {
		tokens = nil // the closure's view is the fresh locked record; never carry a stale pass
		// (change 0494) Record abandonment first: an admitted entry whose publish lock
		// reads free has no publisher left to write its outcome, so it becomes
		// uncertain ("outcome unknown", never verified). The probe only tries and never
		// waits, so holding run.lock here cannot deadlock a publisher that holds its
		// publish lock while it waits on run.lock for its own outcome write.
		marked := false
		for i := range rec.AdmittedMutations {
			if publisherGone(dir, rec.AdmittedMutations[i]) {
				rec.AdmittedMutations[i].Status = mutationStatusUncertain
				rec.AdmittedMutations[i].Verified = false
				marked = true
			}
		}
		idxs := settleablePublicationIndexes(*rec)
		if len(idxs) == 0 && !marked {
			return errRunFenceNoWrite
		}
		for _, i := range idxs {
			rec.AdmittedMutations[i].Status = mutationStatusCompleted
			tokens = append(tokens, "mutation-settled:"+rec.AdmittedMutations[i].OpKey)
		}
		return nil
	})
	// (rest unchanged)
}
```

Extend the function's doc comment. Its only transitions are now admitted→uncertain, for an entry whose publisher is provably gone (change 0494), and uncertain→completed, for a retry-matched original. The abandonment write is bookkeeping: the lock probe is the evidence, so a failed write (`mutation-settle-failed`) still lets the classifier read the free lock.

- [ ] **Step 3b: Implement the readers** (`internal/app/runtracker_cancel.go`). Replace the step (7) loop in `reconcileRunTeardown` with:

```go
	// (7) Reconcile admitted mutations from the re-enumerated (post-settlement)
	// journal through the single journal rule (classifyAdmittedMutation, change
	// 0494): only an entry whose publisher may still be running, or whose state
	// cannot be proven, keeps cancellation pending (mutation-pending:<op>). An
	// uncertain entry, or one whose publisher provably exited, is accounted with the
	// informational mutation-abandoned:<op>.
	mblocked, mfindings := accountAdmittedMutations(runJournalDir(repoDir, runKey), reEp.AdmittedMutations)
	findings = append(findings, mfindings...)
	if mblocked {
		accounted = false
	}
```

Replace the loop in `verifyTerminalRunQuiescence` with:

```go
	mblocked, mfindings := accountAdmittedMutations(runJournalDir(repoDir, ep.RunKey), ep.AdmittedMutations)
	findings = append(findings, mfindings...)
	if mblocked {
		quiescent = false
	}
```

Update the doc comments of `reconcileRunTeardown` and `verifyTerminalRunQuiescence`. Replace "an admitted-not-completed mutation" / "an admitted-not-completed entry keeps the run pending" with "a journal entry whose publisher may still be running (classifyAdmittedMutation)". Note that `reconcileRunTeardown`'s only run write is now `settleUncertainPublications`, which flips a dead publisher's admitted entry to uncertain and a retry-proven uncertain entry to completed. In `verifyTerminalRunQuiescence`, state that it never writes: a dead publisher's entry is reported abandoned from the probe alone.

- [ ] **Step 4: Rewrite the 0444 no-retry tests to the new rule** (keep every test; update each doc comment to say that change 0494 reversed the premise):
  - **`TestIntegrationRunCancelStaysPendingWithoutCompletedIdenticalRetry`**: rename it to `TestIntegrationRunCancelUncertainWithoutRetryIsAbandoned` (spec test 6, cancel half). The first cancel must report `cancelled`, with exactly one `mutation-abandoned:workspace.publish`, no `mutation-pending`, and the entry still `uncertain`. Replace the append-a-retry tail with: a repeat `runCancel` → `already-cancelled`, still carrying `mutation-abandoned:workspace.publish`, and the entry still `uncertain`; then `validateResumeQuiescence` is ok, and `SupersedeCancelledRun(fx.repo, fx.key, "replacement-key")` succeeds.
  - **`TestIntegrationRunCompletionReadOnlyPathsNeverSettle`**: keep the settleable pair, and append a third entry, `admitted` with a free lock: `tok, _ := seedPublishLock(t, fx.repo, fx.key, false)` before seeding, then `{OpKey: OperationWorkspacePublish, Status: mutationStatusAdmitted, Publication: &other, LockToken: tok}` with a distinct `other` descriptor. Now `verifyTerminalRunQuiescence` must be quiescent, with exactly two `mutation-abandoned:workspace.publish` findings and no `mutation-pending`. `validateResumeQuiescence` must be ok. The record must stay byte-identical, so the third entry is still `admitted`: read-only paths never write the abandonment.
  - **`assertUnverifiedRetryLeavesOriginalPending`**: rename it to `assertUnverifiedRetryLeavesOriginalUnsettled`, and update both callers (`TestIntegrationRunFenceProductionUnverifiedPRRetryNeverSettles` and `TestIntegrationRunFenceProductionUnverifiedWorkspaceRetryNeverSettles`). New assertions: disposition `cancelled`; `mutation-abandoned:<op>` present; no `mutation-settled`; no `mutation-pending`; original still `uncertain`. Its doc says that the unverified retry still settles nothing, and that the original no longer blocks (change 0494).
  - **`TestIntegrationRunCompletionSettlementInterruptionConverges` part (b)**: the disposition stays `cancellation-pending`, because the read-only key dir also fails the final cancelling→cancelled write. Assert `mutation-settle-failed`, `finalize-unpersisted`, and `mutation-abandoned:workspace.publish` are present, and `mutation-pending` and `mutation-settled` are absent. Parts (a) and (c) are unchanged. Update the test's doc comment: exclusion is no longer retained by the journal, and a failed settlement write never blocks once the publisher has returned.
  Then derive any remaining test that expects an `uncertain` entry to block, instead of trusting this list:
  `grep -n "mutationStatusUncertain" internal/app/*_test.go` and `grep -n "mutation-pending" internal/app/*_test.go`. Every hit whose journal seeds an `uncertain` entry and expects `mutation-pending`, `cancellation-pending`, `completion-unaccounted`, `refused`, or a failed resume must be rewritten to the new rule in the same way. Leave closeout tests in `runtracker_complete_integration_test.go` for Task 6. Entries seeded `admitted` with no `LockToken` still block and stay unchanged.

- [ ] **Step 5: Run and confirm everything passes.**
Run Step 2's command → PASS. Then:
`bash tests/test_go_integration_app_runcancel.sh && bash tests/test_go_integration_app_runfence.sh && bash tests/test_go_integration_app_runcompletion.sh && bash tests/test_go_integration_app_runstart.sh && bash tests/test_go_integration_app_concurrency.sh`
Expected: all `ok`. The closeout still blocks on `uncertain` through `accountCompletionMutations` until Task 6, and no test seeds `uncertain` for a closeout that expects success yet.

- [ ] **Step 6: Mutation checks (spec test 10).** Back up `internal/app/runtracker_publish_lock.go` and `internal/app/runtracker_publication.go`, and restore after each check:
  1. In `publisherGone`, change `== process.LockProbeFree` to `!= process.LockProbeMissing` (held read as free) → `TestIntegrationRunCancelLivePublisherStaysPending` FAILS.
  2. Change it to `== process.LockProbeFree || publishLockProbe(path) == process.LockProbeMissing` (missing read as free) → `TestIntegrationRunCancelUnprovableJournalEntriesBlock/lock_file_missing` FAILS.
  3. In `settleUncertainPublications`, delete the `marked` loop (drop the admitted→uncertain step) → `TestIntegrationRunCancelKilledPublisherThenVerifiedRetrySettles` FAILS: it reports `mutation-abandoned` instead of `mutation-settled`.
  Run each with `go test -tags integration -count=1 -run '^<TestName>$' ./internal/app/`. Restore both files, then re-run Step 2's command → PASS.

- [ ] **Step 7: Commit.**

```bash
git add internal/app/runtracker_publication.go internal/app/runtracker_cancel.go \
  internal/app/runtracker_cancel_publish_lock_integration_test.go internal/app/runtracker_cancel_integration_test.go \
  internal/app/runtracker_complete_integration_test.go internal/app/runtracker_fence_integration_test.go \
  internal/app/runtracker_publication_integration_test.go
git commit -m "feat(0494): cancel and resume block only on a publisher that may still be running"
```

---

### Task 6: The success closeout uses the classifier

**Suggested tier:** standard.

**Files:**
- Modify: `internal/app/runtracker_complete.go` (`accountCompletionMutations`, its two call sites, and the `accountCompletionMutations` doc)
- Create: `internal/app/runtracker_complete_publish_lock_integration_test.go` (`//go:build integration`)
- Rewrite (do not delete): `TestIntegrationRunCompletionCompleteSuccessfulRunStillBlocksWithoutRetry` and `TestIntegrationRunCompletionCompleteSuccessfulRunBlocksOnUnverifiedRetry` (`runtracker_complete_integration_test.go`)

**Interfaces:**
- Consumes: `accountAdmittedMutations` and `runJournalDir` (Task 2); `seedPublishLock`, `seedJournal`, `journalEntry`, `lockTestPRDesc`, `lockTestWSDesc` (Task 4); and the existing `newCompletionFixture`, `completionFixture.seams()`, `newVerdictCompletionFixture`, `RunVerdict`, `countFinding`, `hasFinding`, and `completeSuccessfulRun`.
- Produces: `func accountCompletionMutations(repoDir, runKey string, ep RunRecord) (bool, []string)`. The signature changes, and both call sites are updated.

- [ ] **Step 1: Write the failing closeout tests.** Create `internal/app/runtracker_complete_publish_lock_integration_test.go`:

```go
//go:build integration

package app

import (
	"context"
	"testing"
)

// TestIntegrationRunCompletionKilledPublisherIsAbandonedAtCloseout (spec test 2).
func TestIntegrationRunCompletionKilledPublisherIsAbandonedAtCloseout(t *testing.T) {
	fx := newCompletionFixture(t)
	desc := lockTestPRDesc()
	token, _ := seedPublishLock(t, fx.repo, fx.key, false)
	seedJournal(t, fx.repo, fx.key, AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusAdmitted, Publication: &desc, LockToken: token})
	ok, reason, findings := completeSuccessfulRun(fx.seams(), fx.repo, fx.key)
	if !ok {
		t.Fatalf("closeout blocked: reason=%q findings=%v", reason, findings)
	}
	if countFinding(findings, "mutation-abandoned:"+OperationPRPublish) != 1 || hasFinding(findings, "mutation-pending") {
		t.Fatalf("findings = %v, want exactly one mutation-abandoned:pr.publish (deduplicated) and no mutation-pending", findings)
	}
	if st := loadRunState(t, fx.repo, fx.key); st != RunCompleted {
		t.Fatalf("state = %q, want completed", st)
	}
	if m := journalEntry(t, fx.repo, fx.key, 0); m.Status != mutationStatusUncertain {
		t.Fatalf("keyed closeout left the entry %q, want uncertain (journal kept truthful)", m.Status)
	}
}

// TestIntegrationRunCompletionVerdictRunCompleteReportsAbandonedPublication (spec test 2,
// through the real keyed verdict).
func TestIntegrationRunCompletionVerdictRunCompleteReportsAbandonedPublication(t *testing.T) {
	fx := newVerdictCompletionFixture(t)
	desc := lockTestPRDesc()
	token, _ := seedPublishLock(t, fx.repo, fx.key, false)
	seedJournal(t, fx.repo, fx.key, AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusAdmitted, Publication: &desc, LockToken: token})
	res := RunVerdict(context.Background(), fx.deps, fx.wdeps, fx.gdeps, fx.repo, fx.key)
	if got, want := res.HumanText(), "run-done "+fx.key+" run-complete 3"; got != want {
		t.Fatalf("HumanText = %q, want %q (findings %v)", got, want, res.CompletionFindings)
	}
	if countFinding(res.CompletionFindings, "mutation-abandoned:"+OperationPRPublish) != 1 {
		t.Fatalf("CompletionFindings = %v, want mutation-abandoned:pr.publish", res.CompletionFindings)
	}
	ep, _, err := LoadRunRecord(fx.repo, fx.key)
	if err != nil || ep.State != RunCompleted {
		t.Fatalf("run state = %q (err %v), want completed", ep.State, err)
	}
}

// TestIntegrationRunCompletionLivePublisherBlocksCloseout (spec test 3, closeout half).
func TestIntegrationRunCompletionLivePublisherBlocksCloseout(t *testing.T) {
	fx := newCompletionFixture(t)
	desc := lockTestPRDesc()
	token, release := seedPublishLock(t, fx.repo, fx.key, true)
	seedJournal(t, fx.repo, fx.key, AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusAdmitted, Publication: &desc, LockToken: token})
	ok, reason, findings := completeSuccessfulRun(fx.seams(), fx.repo, fx.key)
	if ok || reason != "completion-unaccounted" || !hasFinding(findings, "mutation-pending:"+OperationPRPublish) {
		t.Fatalf("live publisher: ok=%v reason=%q findings=%v, want completion-unaccounted with mutation-pending", ok, reason, findings)
	}
	if st := loadRunState(t, fx.repo, fx.key); st != RunCompleting {
		t.Fatalf("state = %q, want completing (the success fence holds)", st)
	}
	release()
	ok, reason, findings = completeSuccessfulRun(fx.seams(), fx.repo, fx.key)
	if !ok || !hasFinding(findings, "mutation-abandoned:"+OperationPRPublish) {
		t.Fatalf("after release: ok=%v reason=%q findings=%v, want completed with mutation-abandoned", ok, reason, findings)
	}
}

// TestIntegrationRunCompletionUnprovableEntryBlocksCloseout (spec test 4, closeout).
func TestIntegrationRunCompletionUnprovableEntryBlocksCloseout(t *testing.T) {
	fx := newCompletionFixture(t)
	tok, err := runToken() // a token with no lock file
	if err != nil {
		t.Fatal(err)
	}
	seedJournal(t, fx.repo, fx.key, AdmittedMutation{OpKey: OperationPRPublish, Status: mutationStatusAdmitted, LockToken: tok})
	ok, reason, findings := completeSuccessfulRun(fx.seams(), fx.repo, fx.key)
	if ok || reason != "completion-unaccounted" || !hasFinding(findings, "mutation-pending:"+OperationPRPublish) || hasFinding(findings, "mutation-abandoned") {
		t.Fatalf("missing lock file: ok=%v reason=%q findings=%v, want blocked mutation-pending only", ok, reason, findings)
	}
}

// TestIntegrationRunCompletionKilledPublisherThenVerifiedRetrySettlesAtCloseout (spec test 5).
func TestIntegrationRunCompletionKilledPublisherThenVerifiedRetrySettlesAtCloseout(t *testing.T) {
	fx := newCompletionFixture(t)
	desc := lockTestWSDesc()
	token, _ := seedPublishLock(t, fx.repo, fx.key, false)
	seedJournal(t, fx.repo, fx.key,
		AdmittedMutation{OpKey: OperationWorkspacePublish, Status: mutationStatusAdmitted, Publication: &desc, LockToken: token},
		AdmittedMutation{OpKey: OperationWorkspacePublish, Status: mutationStatusCompleted, Verified: true, Publication: &desc})
	ok, reason, findings := completeSuccessfulRun(fx.seams(), fx.repo, fx.key)
	if !ok {
		t.Fatalf("closeout blocked: reason=%q findings=%v", reason, findings)
	}
	if countFinding(findings, "mutation-settled:"+OperationWorkspacePublish) != 1 || hasFinding(findings, "mutation-abandoned") {
		t.Fatalf("findings = %v, want mutation-settled and no mutation-abandoned", findings)
	}
	if m := journalEntry(t, fx.repo, fx.key, 0); m.Status != mutationStatusCompleted {
		t.Fatalf("original = %q, want completed", m.Status)
	}
}
```

- [ ] **Step 2: Run them and confirm they fail.**
Run: `go test -tags integration -count=1 -run '^TestIntegrationRunCompletion(KilledPublisher|VerdictRunCompleteReportsAbandoned|LivePublisherBlocksCloseout|UnprovableEntryBlocksCloseout)' ./internal/app/`
Expected: FAIL. The killed-publisher closeout is `completion-unaccounted`, because the step (1b) settle made the entry `uncertain`, and the old loop still blocks it.

- [ ] **Step 3: Implement** in `internal/app/runtracker_complete.go`:

```go
// accountCompletionMutations accounts the journal through the single journal rule
// (classifyAdmittedMutation, change 0494), mirroring cancellation. A publisher that
// may still be running, or an unprovable entry, blocks with mutation-pending:<op>.
// An uncertain entry, or one whose publisher provably exited, is accounted with the
// informational mutation-abandoned:<op>. RunVerify's live probes behind the verified
// run-complete, not the journal, are the evidence that a publication landed.
func accountCompletionMutations(repoDir, runKey string, ep RunRecord) (bool, []string) {
	return accountAdmittedMutations(runJournalDir(repoDir, runKey), ep.AdmittedMutations)
}
```

Update both call sites: in `accountCompletionObligations`, `accountCompletionMutations(repoDir, runKey, ep)`; in `completeSuccessfulRun` step (4), `accountCompletionMutations(repoDir, runKey, reEp)`. The existing `dedupeFindings` already collapses the step (3) and step (4) duplicates. In the step (3) comment, change "an uncompleted mutation" to "a mutation whose publisher may still be running", and add `mutation-abandoned` to the informational examples.

- [ ] **Step 4: Rewrite the two 0444 closeout tests** (keep both; update their doc comments to say change 0494 reversed the premise):
  - **`TestIntegrationRunCompletionCompleteSuccessfulRunStillBlocksWithoutRetry`**: rename it to `TestIntegrationRunCompletionCompleteSuccessfulRunAbandonsUncertainWithoutRetry` (spec test 6, closeout half). Assert `ok == true`, exactly one `mutation-abandoned:workspace.publish`, no `mutation-pending`, state `RunCompleted`, and the entry still `uncertain`.
  - **`TestIntegrationRunCompletionCompleteSuccessfulRunBlocksOnUnverifiedRetry`**: rename it to `TestIntegrationRunCompletionCompleteSuccessfulRunUnverifiedRetrySettlesNothing`. Assert `ok == true`, no `mutation-settled`, exactly one `mutation-abandoned:workspace.publish`, and the original still `uncertain`. The unverified retry is still no evidence.
  Then re-run the derivation grep from Task 5 Step 4 (`mutationStatusUncertain` / `mutation-pending` in `internal/app/*_test.go`). Rewrite any remaining closeout test that expects an `uncertain` entry to block. Tests seeding `admitted` with no `LockToken` (for example the `mutation-pending` table row and the dedupe test) stay as they are.

- [ ] **Step 5: Run and confirm everything passes.**
Run Step 2's command, plus `-run '^TestIntegrationRunCompletionKilledPublisherThenVerifiedRetrySettlesAtCloseout$'` → PASS. Then:
`bash tests/test_go_integration_app_runcompletion.sh && bash tests/test_go_integration_app_runverdict.sh && bash tests/test_go_integration_app_runcancel.sh && bash tests/test_go_integration_app_runfence.sh`
Expected: all `ok`.

- [ ] **Step 6: Mutation check.** Back up `internal/app/runtracker_complete.go`. Make `accountCompletionMutations` return `false, nil` (closeout ignores the journal) → `TestIntegrationRunCompletionLivePublisherBlocksCloseout` FAILS. Restore, then re-run → PASS.

- [ ] **Step 7: Commit.**

```bash
git add internal/app/runtracker_complete.go internal/app/runtracker_complete_publish_lock_integration_test.go internal/app/runtracker_complete_integration_test.go
git commit -m "feat(0494): the success closeout blocks only on a publisher that may still be running"
```

---

### Task 7: Comments that state the old rule, glossary, and run-tracker concept

**Suggested tier:** economy.

**Files:**
- Modify: comment sites in `internal/app/*.go`, derived by grep (non-test files only)
- Modify: `docs/reference/glossary.md` (insert before `### Cancel finding \`tree-survives\``)
- Modify: `docs/concepts/run-tracker.md` (insert after the paragraph ending "…a moment before the suite runner finishes stopping its targets.")

**Interfaces:**
- Consumes: the vocabulary from Tasks 2–6.
- Produces: no code.

- [ ] **Step 1: Derive the comment sites.** Do not hand-list them.
Run: `grep -n -e 'uncertain' -e 'admitted-not-completed' -e 'mutation-pending' -e 'FAIL CLOSED' -e 'unresolved external effect' internal/app/*.go | grep -v '_test.go'`
For each hit that still states the old rule ("uncertain keeps a cancellation pending", "any admitted-not-completed entry keeps it pending", "a pending or uncertain obligation blocks completion", "never reported cancelled while … an unresolved external effect remains"), rewrite it to the new rule: an entry blocks only while its publisher may still be running (`classifyAdmittedMutation`); an `uncertain` entry, or an `admitted` entry with a provably gone publisher, is the informational `mutation-abandoned:<op>`. The spec names these known sites:
  - `runtracker_fence.go` package header: "(it is never reported `cancelled` while an owned writer or unresolved external effect remains)" becomes "(it is never reported `cancelled` while an owned writer or a publisher that may still be running remains; change 0494)".
  - `runtracker_publication.go` package header and the `publicationRetryMatch` doc ("the ONLY settling evidence"): keep the claim for settlement to `completed`, and add one sentence saying that a dead publisher's admitted→uncertain rewrite (change 0494) is bookkeeping, not settlement, and never marks an entry completed.
  - `runtracker_cancel.go` package header ORDER paragraph: "reconcile AdmittedMutations (any admitted-not-completed entry keeps it pending)" becomes "reconcile AdmittedMutations (an entry whose publisher may still be running keeps it pending)". Also update the docs of `CancelDispositionCancelled` and `CancelDispositionPending`, and the `mutationStatusCompleted` comment: "Any other status (admitted, uncertain) is an admitted-not-completed entry that keeps a cancellation pending" becomes "completed is the only status accounted with no finding; classifyAdmittedMutation decides every other status (change 0494)".
  - `runtracker_complete.go` FAIL CLOSED paragraph: replace "A live, busy, pending, uncertain, or unreadable obligation blocks completion" with "A live, busy, pending, or unreadable obligation blocks completion; a publication journal entry blocks only while its publisher may still be running (change 0494), because RunVerify's live probes behind the verified run-complete are the evidence a publication landed". Also update the `completeSuccessfulRun` doc so that "findings may also carry informational mutation-settled:<op> tokens" reads "mutation-settled:<op> or mutation-abandoned:<op> tokens".
  Anchor every cross-reference on a symbol name, never a line number.

- [ ] **Step 2: Add the glossary entry.** Insert it immediately before `### Cancel finding \`tree-survives\`` in `docs/reference/glossary.md`:

```markdown
### Cancel finding `mutation-abandoned`

`mutation-abandoned:<op>` is an informational finding from `run.cancel`, the death guardian, a
repeat cancel, the resume check, and `run.verdict`'s success closeout. `<op>` is `pr.publish` or
`workspace.publish`. Docket stopped waiting on that publish because it never saw the outcome. Either
the publishing process died mid-flight (Ctrl-C, a crash, or SIGKILL), or the publish returned
without seeing GitHub's or the remote's answer, and no identical publish later confirmed it. Docket
knows the publisher is gone because a publish holds a lock file beside the run record for its
whole remote call, and the kernel frees that lock when the process exits.

Cancel still reports `cancelled`, resume still admits its replacement, and the closeout verdict is
unchanged. The pushed branch or opened PR may or may not exist. A resumed run's publish adopts
whatever landed, because both publishes converge on a retry. The finding is information, never a
blocker: no skill or reviewer escalates it into one.

`mutation-pending:<op>` is different. There, the publisher may still be running, or docket cannot
prove it stopped (an entry with no lock, a missing lock file, an unreadable lock). That finding
still keeps cancel at `cancellation-pending` and the closeout at `completion-unaccounted`. When a
later identical publish completes and docket verifies it, the entry is reported
`mutation-settled:<op>` instead.
```

- [ ] **Step 3: Add the run-tracker paragraph.** Insert it after the paragraph in `docs/concepts/run-tracker.md` that ends "…a moment before the suite runner finishes stopping its targets.":

```markdown
A publish killed mid-flight no longer wedges a run. `pr.publish` and
`workspace.publish` journal each remote call in the run record before they
make it, and hold a lock file beside the record until they have written the
outcome. If the process dies in between, the lock frees with it. Cancel,
resume, and the success closeout then read the entry as abandoned: cancel
reports `cancelled`, resume admits its replacement, and the closeout
completes, each with a `mutation-abandoned:<op>` finding (see the glossary).
While the lock is still held, or when docket cannot prove it free, the entry
blocks as `mutation-pending:<op>`, exactly as before.
```

- [ ] **Step 4: Verify.**
Run: `go build ./... && go vet ./internal/app/ && go test -count=1 -run 'TestCommentAnchorStyle' ./internal/repoguard/`
Expected: PASS. Then re-run Step 1's grep and confirm that no remaining non-test hit states the old blocking rule. Report any line you deliberately left, with the reason.

- [ ] **Step 5: Commit.**

```bash
git add internal/app/runtracker_fence.go internal/app/runtracker_publication.go internal/app/runtracker_cancel.go internal/app/runtracker_complete.go \
  docs/reference/glossary.md docs/concepts/run-tracker.md
git commit -m "docs(0494): glossary and run-tracker concept describe mutation-abandoned; comments state the new journal rule"
```

(Stage only the files Step 1 actually changed. If the grep found another `internal/app` file, add that path explicitly. Never use `git add -A`.)

---

## Build gate (run by docket-build after the last task, not a task)

The whole suite runs once through the configured `build.test_command` (`go run ./cmd/docket development test`), entered from source. Read the budget report even when green, and act on any `SERIAL CONFIRMED OVER BUDGET:` line. The cancel, completion, fence, and concurrency shards each gain a handful of tests.

## Self-Review

- **Spec coverage:**
  - §1 lock-before-admit, journal token, release-last, refusal releases, lock failure degrades, scope → Task 4.
  - §2 one classifier, typed never-creating probe, std-lib only → Tasks 1 and 2; wired into all three readers → Tasks 5 and 6.
  - §3 admitted→uncertain before the 0444 match on the two write paths, read-only paths never write, no deadlock → Tasks 5 and 4.
  - §4 vocabulary → Tasks 2 and 7.
  - Failure posture → Global Constraints, plus the unprovable-blocks tests in Tasks 2, 5, and 6.
  - Prose and generated sites → Task 7.
  - Spec tests 1–10 → test 1: T5; 2: T6; 3: T5 and T6; 4: T2, T5, T6; 5: T5 and T6; 6: T5 and T6 (rewrites); 7: T4; 8: T3; 9: T4; 10: mutation steps in T2, T4, T5, T6.
  - The Decision record (ADRs) is coordinator-owned, by instruction.
- **Placeholder scan:** none. Each grep-derived step names its exact grep and the rewrite rule.
- **Type consistency:**
  - `classifyAdmittedMutation(dir string, m AdmittedMutation) (bool, string)`, `accountAdmittedMutations(dir string, muts []AdmittedMutation) (bool, []string)`, `runJournalDir(repoDir, runKey string) string`, `publisherGone(dir string, m AdmittedMutation) bool`, `publishLockPath(dir, token string) (string, bool)`, and `acquirePublishLock(dir string) (string, *os.File)` are used identically in Tasks 2–6.
  - `seedPublishLock(t, repo, key string, hold bool) (string, func())` is used identically in Tasks 4–6.
  - `accountCompletionMutations(repoDir, runKey string, ep RunRecord)` is updated at both call sites in Task 6.
- **Review Focus:** all five items have pinning tests in their owning tasks.
