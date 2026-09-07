<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0388 — Reimplement post-merge fast-forward integration-branch sync as a native Go verb](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0388-reimplement-post-merge-fast-forward-integration-branch-sync.md)**
<!-- docket:backlink:end -->
# Native post-merge integration-branch sync Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add the native `repository.sync-integration` operation — a safe, explicit fast-forward of the consuming repository's primary checkout to the freshly fetched integration tip — and run it once at the end of the finalize workflow and once at the end of either maintenance-sweep scope.

**Architecture:** One app-layer service (`internal/app/repository_sync.go`) over the existing `loadOperationalContext` loader (discovery, config resolution, legacy refusal, fetch-and-pin of the integration branch) and the existing `gitcli.Client.FastForwardWorktree` primitive, with an injectable seam so orchestration is unit-testable without a repository. The public CLI verb and the maintenance sweep both call that same service; finalize reaches it through the capability catalog from skill prose. A new `gitcli` checkout-state probe supplies the branch/detached/unfinished-operation facts the safety ladder needs.

**Tech Stack:** Go (cobra CLI, `internal/gitcli` adapter, protocol-v1 envelopes, reflected schema surface), hermetic Go integration tests with local bare origins, Bash suite shards under `tests/`.

**Spec:** `docs/superpowers/specs/2026-09-07-reimplement-post-merge-fast-forward-integration-branch-sync-design.md` (on the metadata branch; synchronized copy at `.docket/docs/superpowers/specs/…` in the primary tree). Read it before executing — every task below argues from it.

## Global Constraints

- Never switch branches, stash, commit user edits, reset, rebase, force a ref, delete resources, or retry with another Git strategy. `merge --ff-only` against a pinned object ID is the only mutation.
- A probe has three outcomes — present, cleanly absent, unknown — and unknown never authorizes the mutating branch (learnings: probe-error-is-not-clean-absence). A failed fetch or unresolved target never falls back to stale remote-tracking data.
- Pass the pinned object ID to `FastForwardWorktree`, never a movable ref name (learnings: decide-and-act-on-the-same-copy).
- No new config knob, change-id argument, branch/remote override, or force flag. Effects: `local-write`.
- Closed disposition vocabulary: `advanced`, `already-current`, `skipped`, `refused`, `failed`. Human messages are explanatory, never decision inputs.
- Every guard added must be mutation-tested: strip the guarded condition, watch the test redden, restore (defeat the Go test cache with `-count=1`).
- Maintained skills name the semantic operation `repository.sync-integration` and resolve argv from the capability catalog — no restated argv.
- Existing Envelope result mapping: advance → `applied`; already-current and deliberate safety skips → `no-op` (successful exit); invalid input/config → the existing non-success envelopes; unobservable/failed Git work → `external-failed`/`internal-error` semantics via `classifyStatusError`.
- Run tests via the source-entered suite: `go test ./internal/<pkg>/ -run <name> -count=1` per task, `go run ./cmd/docket development test` at the build gate.

---

### Task 1: `gitcli` checkout-state probe

**Files:**
- Create: `internal/gitcli/checkoutstate.go`
- Test: `internal/gitcli/checkoutstate_integration_test.go` (build tag `integration`, test prefix `TestIntegrationRepo…` so it rides the existing `tests/test_go_integration_gitcli_repo.sh` shard — confirm that shard's `SHARD_PREFIX` before naming)

**Interfaces:**
- Produces: `type CheckoutState struct { Branch RefName; Detached bool; Head ObjectID; OperationInProgress bool }` and `func (c *Client) WorktreeCheckoutState(ctx context.Context, worktreeDir string) (CheckoutState, error)`. `Branch` is the full symbolic ref (`refs/heads/<name>`) when attached, empty when detached. Any probe failure is a typed `*Failure` error — never a zero-value success.

The safety ladder (Task 3) needs three facts about the primary checkout that no existing exported method gives together: the symbolic branch (or detached), the HEAD object id, and whether an unfinished Git operation (merge, rebase, cherry-pick, revert, bisect) is in progress. `worktreeHead` and `rebaseInProgress` exist but are unexported and narrower.

- [ ] **Step 1: Write the failing integration test**

In `checkoutstate_integration_test.go` (line 1 must be `//go:build integration`, line 2 blank — the contract in `tests/test_go_integration_contract.sh` enforces this), using the package's existing fixture helpers (`newMainModeRepos`, `newRealClient`, `gitOut`, `writeWorktreeFile` — see `fastforward_integration_test.go` for the idiom):

```go
func TestIntegrationRepoWorktreeCheckoutState(t *testing.T) {
	ctx := context.Background()

	t.Run("attached clean branch", func(t *testing.T) {
		r := newMainModeRepos(t)
		c := newRealClient(t)
		st, err := c.WorktreeCheckoutState(ctx, r.Invocation)
		if err != nil {
			t.Fatalf("WorktreeCheckoutState: %v", err)
		}
		if st.Detached || st.Branch != "refs/heads/main" || st.OperationInProgress {
			t.Fatalf("state = %+v, want attached refs/heads/main with no operation", st)
		}
		if got := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD")); got != st.Head {
			t.Fatalf("Head = %s, want %s", st.Head, got)
		}
	})

	t.Run("detached HEAD", func(t *testing.T) {
		r := newMainModeRepos(t)
		c := newRealClient(t)
		gitOut(t, r.Invocation, "checkout", "-q", "--detach", "HEAD")
		st, err := c.WorktreeCheckoutState(ctx, r.Invocation)
		if err != nil {
			t.Fatalf("WorktreeCheckoutState: %v", err)
		}
		if !st.Detached || st.Branch != "" {
			t.Fatalf("state = %+v, want detached with empty branch", st)
		}
	})

	t.Run("merge in progress", func(t *testing.T) {
		r := newMainModeRepos(t)
		c := newRealClient(t)
		// Build two divergent commits and start a real conflicted merge.
		writeWorktreeFile(t, r.Invocation, "conflict.txt", "local\n")
		gitOut(t, r.Invocation, "add", "--", "conflict.txt")
		gitOut(t, r.Invocation, "commit", "-q", "-m", "local side")
		gitOut(t, r.Invocation, "checkout", "-q", "-b", "other", "HEAD~1")
		writeWorktreeFile(t, r.Invocation, "conflict.txt", "other\n")
		gitOut(t, r.Invocation, "add", "--", "conflict.txt")
		gitOut(t, r.Invocation, "commit", "-q", "-m", "other side")
		gitOut(t, r.Invocation, "checkout", "-q", "main")
		_ = gitMaybe(t, r.Invocation, "merge", "other") // conflicts; ignore exit
		st, err := c.WorktreeCheckoutState(ctx, r.Invocation)
		if err != nil {
			t.Fatalf("WorktreeCheckoutState: %v", err)
		}
		if !st.OperationInProgress {
			t.Fatalf("state = %+v, want OperationInProgress", st)
		}
	})

	t.Run("probe failure is an error, not a zero state", func(t *testing.T) {
		c := newRealClient(t)
		if _, err := c.WorktreeCheckoutState(ctx, t.TempDir()); err == nil {
			t.Fatal("want error for a non-repository directory")
		}
	})
}
```

If the fixture set has no `gitMaybe` (a git run whose non-zero exit is tolerated), add a tiny local helper in this test file that shells the command and ignores the exit code. Do not weaken `gitOut`.

- [ ] **Step 2: Run to verify it fails**

Run: `go test -tags integration ./internal/gitcli/ -run TestIntegrationRepoWorktreeCheckoutState -count=1`
Expected: FAIL — `WorktreeCheckoutState` undefined.

- [ ] **Step 3: Implement `WorktreeCheckoutState`**

In `checkoutstate.go`, following the `FastForwardWorktree` shape (absolute-path check, `c.run` with a named `Operation` const, typed failures):

```go
package gitcli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

const worktreeCheckoutStateOp Operation = "worktree-checkout-state"

// CheckoutState is one observation of a worktree's checkout: the symbolic
// branch (full ref) or detached, the HEAD commit, and whether an unfinished
// Git operation (merge, rebase, cherry-pick, revert, bisect) is in progress.
type CheckoutState struct {
	Branch              RefName // full ref, e.g. refs/heads/main; empty when Detached
	Detached            bool
	Head                ObjectID
	OperationInProgress bool
}

// WorktreeCheckoutState probes worktreeDir. Every failure is a typed error —
// a caller must never read a zero CheckoutState as an observed fact
// (learnings: probe-error-is-not-clean-absence).
func (c *Client) WorktreeCheckoutState(ctx context.Context, worktreeDir string) (CheckoutState, error) {
	var st CheckoutState
	if !filepath.IsAbs(worktreeDir) {
		return st, newFailure(worktreeCheckoutStateOp, KindInvalidRequest, "worktree path must be absolute", nil)
	}

	sym, f := c.run(ctx, runRequest{
		op:   worktreeCheckoutStateOp,
		dir:  worktreeDir,
		args: []string{"symbolic-ref", "--quiet", "HEAD"},
	})
	if f != nil {
		return st, f
	}
	switch sym.exitCode {
	case 0:
		st.Branch = RefName(strings.TrimSpace(string(sym.stdout)))
	case 1:
		st.Detached = true // --quiet: exit 1 IS the clean detached answer
	default:
		return CheckoutState{}, newFailure(worktreeCheckoutStateOp, KindCommandFailed,
			"symbolic-ref HEAD failed: "+stderrExcerpt(sym.stderr), nil).withExitCode(sym.exitCode)
	}

	head, hf := c.worktreeHead(ctx, worktreeCheckoutStateOp, worktreeDir)
	if hf != nil {
		return CheckoutState{}, hf
	}
	st.Head = head

	// Unfinished-operation markers, resolved through git so linked worktrees
	// and split git-dirs are honored. Any marker's presence is in-progress.
	markers := []string{"MERGE_HEAD", "CHERRY_PICK_HEAD", "REVERT_HEAD", "BISECT_LOG", "rebase-merge", "rebase-apply"}
	paths, pf := c.run(ctx, runRequest{
		op:   worktreeCheckoutStateOp,
		dir:  worktreeDir,
		args: append([]string{"rev-parse", "--git-path"}, markers...),
	})
	if pf != nil {
		return CheckoutState{}, pf
	}
	if paths.exitCode != 0 {
		return CheckoutState{}, newFailure(worktreeCheckoutStateOp, KindCommandFailed,
			"rev-parse --git-path failed: "+stderrExcerpt(paths.stderr), nil).withExitCode(paths.exitCode)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(paths.stdout)), "\n") {
		p := strings.TrimSpace(line)
		if p == "" {
			continue
		}
		if !filepath.IsAbs(p) {
			p = filepath.Join(worktreeDir, p)
		}
		if _, err := os.Stat(p); err == nil {
			st.OperationInProgress = true
			break
		}
	}
	return st, nil
}
```

Adjust field/helper spellings to the package's actual `runResult` members (`exitCode`, `stdout`, `stderr` — confirmed in `fastforward.go`). `worktreeHead` lives in `rebase.go` and is reusable as-is.

- [ ] **Step 4: Run to verify it passes**

Run: `go test -tags integration ./internal/gitcli/ -run TestIntegrationRepoWorktreeCheckoutState -count=1`
Expected: PASS.

- [ ] **Step 5: Mutation-test the detached mapping**

Temporarily change `case 1:` to also return an error; the detached subtest must redden. Restore. Temporarily make the marker loop skip `MERGE_HEAD`; the merge-in-progress subtest must redden. Restore. Re-run with `-count=1` after each mutation.

- [ ] **Step 6: Commit**

```bash
git add internal/gitcli/checkoutstate.go internal/gitcli/checkoutstate_integration_test.go
git commit -m "feat(0388): gitcli worktree checkout-state probe"
```

---

### Task 2: Sync result document, vocabularies, and schema surface

**Files:**
- Create: `internal/app/repository_sync.go` (types + constants only in this task; the service body arrives in Task 3)
- Modify: `internal/app/schema_registry.go` (one binding row), `internal/app/schema_vocab.go` (one disposition family)
- Test: `internal/app/repository_sync_test.go` (result-shape tests); existing `schema_*_test.go` guards must stay green

**Interfaces:**
- Produces:
  - `const OperationRepositorySyncIntegration = "repository.sync-integration"`
  - Disposition consts: `SyncDispAdvanced = "advanced"`, `SyncDispAlreadyCurrent = "already-current"`, `SyncDispSkipped = "skipped"`, `SyncDispRefused = "refused"`, `SyncDispFailed = "failed"` (a prefixed const group, so `TestVocabularyConstCompleteness` can hold the emitted vocabulary in correspondence with it — follow that test's existing derivation pattern).
  - Reason consts (skips): `ReasonSyncDirtyWorktree = "dirty-worktree"`, `ReasonSyncDetachedHead = "detached-head"`, `ReasonSyncOtherBranch = "other-branch"`, `ReasonSyncOperationInProgress = "operation-in-progress"`, `ReasonSyncLocalAhead = "local-ahead"`, `ReasonSyncDiverged = "diverged"`, `ReasonSyncCheckoutChanged = "checkout-changed"`. Reason consts (failures): `ReasonSyncStateProbeFailed = "state-probe-failed"`, `ReasonSyncAncestryProbeFailed = "ancestry-probe-failed"`, `ReasonSyncUpdateFailed = "update-failed"`, `ReasonSyncPostcheckUnverified = "postcheck-unverified"`. Discovery/configuration/fetch failures reuse the loader's existing `classifyStatusError` reasons rather than minting parallel spellings.
  - `type SyncOutcome struct` — the Docket-owned structured outcome shared verbatim by the standalone document and the maintenance embed:

```go
// SyncOutcome is the structured integration-sync outcome: disposition, reason,
// the primary checkout it decided about, and the object ids it knew. It is
// embedded by RepositorySyncResult and carried additively by MaintenanceResult;
// human messages are explanatory, never decision inputs.
type SyncOutcome struct {
	Disposition       string `json:"disposition" docket:"enum=sync_dispositions"`
	Reason            string `json:"reason,omitempty"`
	Message           string `json:"message,omitempty"`
	PrimaryPath       string `json:"primary_path,omitempty"`
	IntegrationBranch string `json:"integration_branch,omitempty"`
	BeforeOID         string `json:"before_oid,omitempty"`
	TargetOID         string `json:"target_oid,omitempty"`
	AfterOID          string `json:"after_oid,omitempty"`
}

// RepositorySyncResult is the protocol-v1 document `repository sync-integration`
// returns.
type RepositorySyncResult struct {
	Envelope
	SyncOutcome
}
```

  - `HumanText()` on `RepositorySyncResult`: one line, e.g. `repository.sync-integration: advanced <branch> <before-short>..<after-short> at <primary_path>` for an advance, `repository.sync-integration: skipped (dirty-worktree) — <message>` otherwise. The dirty-worktree message must explicitly say non-ignored untracked files can block sync and tell the user to inspect and resolve the dirt before retrying.
  - `newRepositorySyncResult(result Result, out RepositorySyncResult) RepositorySyncResult` stamping `NewEnvelope(OperationRepositorySyncIntegration, result)` (mirror `newMaintenanceResult`).

- [ ] **Step 1: Write the failing shape tests**

In `repository_sync_test.go`:

```go
func TestRepositorySyncResultShape(t *testing.T) {
	r := newRepositorySyncResult(ResultApplied, RepositorySyncResult{SyncOutcome: SyncOutcome{
		Disposition: SyncDispAdvanced, IntegrationBranch: "main",
		PrimaryPath: "/repo", BeforeOID: "a", TargetOID: "b", AfterOID: "b",
	}})
	if r.Operation != OperationRepositorySyncIntegration || r.Result != ResultApplied {
		t.Fatalf("envelope = %s/%s", r.Operation, r.Result)
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"disposition":"advanced"`, `"integration_branch":"main"`, `"before_oid":"a"`, `"target_oid":"b"`, `"after_oid":"b"`} {
		if !strings.Contains(string(b), key) {
			t.Fatalf("marshal missing %s in %s", key, b)
		}
	}
}

func TestRepositorySyncDirtyMessageNamesUntrackedFiles(t *testing.T) {
	msg := syncDirtyMessage() // the one constructor every dirty-skip path uses
	for _, phrase := range []string{"untracked", "inspect"} {
		if !strings.Contains(msg, phrase) {
			t.Fatalf("dirty message %q must mention %q", msg, phrase)
		}
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/app/ -run 'TestRepositorySync' -count=1`
Expected: FAIL — types undefined.

- [ ] **Step 3: Implement types, consts, `syncDirtyMessage`, `HumanText`, and the constructor**

As specified in Interfaces above. `syncDirtyMessage()` returns the fixed human string, e.g. `"primary checkout has uncommitted changes (non-ignored untracked files also block sync); inspect and resolve the dirt, then rerun"`.

- [ ] **Step 4: Register the schema surface**

- `schema_registry.go`: add `{ID: "repository.sync-integration", Request: nil, Result: RepositorySyncResult{}}, // RunRepositorySyncIntegration` in sorted position (the registry test enforces sorted-unique).
- `schema_vocab.go`: add `v["sync_dispositions"] = Vocabulary{Members: []string{SyncDispAdvanced, SyncDispAlreadyCurrent, SyncDispFailed, SyncDispRefused, SyncDispSkipped}}` following the existing families, and extend whatever const-group correspondence `TestVocabularyConstCompleteness` requires (read that test and follow its derivation — do not hand-duplicate the list if it derives from the const group).

Note: the registry entry references `RunRepositorySyncIntegration`, which does not exist until Task 4. If the registry's describability test executes the symbol comment only as prose this is fine; the Go compiler needs only the types, which exist now. If any guard demands the CLI command too, defer the registry row to Task 5 and say so in the commit message.

- [ ] **Step 5: Run the schema guards**

Run: `go test ./internal/app/ -run 'TestRepositorySync|TestOperationBindings|TestVocabulary|TestSchema' -count=1`
Expected: PASS (fix ordering/derivation complaints per the guards' own remedy messages).

- [ ] **Step 6: Commit**

```bash
git add internal/app/repository_sync.go internal/app/repository_sync_test.go internal/app/schema_registry.go internal/app/schema_vocab.go
git commit -m "feat(0388): sync-integration result document, dispositions, schema surface"
```

---

### Task 3: The sync service and its safety ladder (unit-tested over seams)

**Files:**
- Modify: `internal/app/repository_sync.go`
- Test: `internal/app/repository_sync_test.go`

**Interfaces:**
- Consumes: `SyncOutcome`, disposition/reason consts (Task 2); `gitcli.CheckoutState` (Task 1).
- Produces:

```go
// syncSeams is the injection seam repositorySyncIntegration runs over.
// Production (Task 4) wires it from loadOperationalContext and *gitcli.Client;
// unit tests inject fakes so the whole decision ladder is proved without a
// repository.
type syncSeams struct {
	// load performs the one ordered read: discovery, config resolution,
	// legacy refusal, and the fetch-and-pin of the integration branch.
	load func(ctx context.Context) (syncContext, error)
	// state observes the primary checkout (branch/detached/head/in-progress).
	state func(ctx context.Context, worktreeDir string) (gitcli.CheckoutState, error)
	// dirty reports tracked or non-ignored-untracked dirt in the primary
	// (ignored files are not dirt).
	dirty func(ctx context.Context, worktreeDir string) (bool, error)
	// isAncestor: exit-1 false is a clean negative; an error is a probe failure.
	isAncestor func(ctx context.Context, ancestor, descendant string) (bool, error)
	// fastForward applies merge --ff-only to the pinned target.
	fastForward func(ctx context.Context, worktreeDir, target string) (bool, error)
}

// syncContext is the loader's slice the ladder needs.
type syncContext struct {
	primaryWorktree     string
	integrationBranch   string // short name, e.g. main
	integrationRevision string // pinned, freshly fetched object id
}

func repositorySyncIntegration(ctx context.Context, seams syncSeams) RepositorySyncResult
```

The ladder (spec §Safe advancement), in order — each numbered check maps to one block and one test:

1. `load` error → `classifyStatusError(ctx, err)` supplies the envelope Result and reason; disposition is `SyncDispRefused` when the Result is `ResultInvalidInput`/`ResultUnsupportedConfig`/`ResultInvalidState`, else `SyncDispFailed`. Never an absent/defaulted repository.
2. `state` on the primary: error → `failed`/`state-probe-failed` (envelope `external-failed`). Detached → `skipped`/`detached-head`. Attached to any branch other than `refs/heads/<integrationBranch>` → `skipped`/`other-branch`. `OperationInProgress` → `skipped`/`operation-in-progress`. Then `dirty`: error → `failed`/`state-probe-failed`; true → `skipped`/`dirty-worktree` with `syncDirtyMessage()`. Skips are envelope `no-op`.
3. The pinned target came from `load` (the loader already fetched fresh — a stale remote-tracking fallback is structurally impossible because no other source is consulted).
4. `head == target` → `already-current` (`no-op`). Else `isAncestor(head, target)`: error → `failed`/`ancestry-probe-failed`. False → probe the direction: `isAncestor(target, head)`; true → `skipped`/`local-ahead`, false → `skipped`/`diverged`, error → `failed`/`ancestry-probe-failed`.
5. Recheck: call `state` again immediately before the advance; error → `failed`/`state-probe-failed` (nothing mutated); a changed branch, detachment, new in-progress operation, or moved head versus step 2's observation → `skipped`/`checkout-changed`, leave the tree alone. Then `fastForward(primary, target)` — it rechecks cleanliness and enforces FF-only itself; error → `failed`/`update-failed` (envelope `external-failed`; never retry, reset, or fall back).
6. Post-check: `state` once more; error, or head != target, or branch changed → `failed`/`postcheck-unverified`, reporting the known before/target OIDs and NOT claiming the tree untouched (After empty on an unobservable head). Verified → `advanced` (`applied`) with Before/Target/After set.

Every outcome carries `PrimaryPath` and `IntegrationBranch` when known, and the OIDs known at that point.

- [ ] **Step 1: Write the failing ladder tests**

Table-driven over fake seams. Cover, at minimum, one test per row:

| name | fakes | want disposition/reason | want Envelope |
|---|---|---|---|
| load discovery error | `load` returns wrapped `ErrStatusExternal` | failed / (classifier's reason) | external-failed |
| load invalid config | `load` returns `ErrStatusInvalidInput`-classified error | refused | invalid-input |
| state probe error | `state` errors | failed / state-probe-failed | external-failed |
| detached | `Detached: true` | skipped / detached-head | no-op |
| other branch | Branch `refs/heads/feature-x` | skipped / other-branch | no-op |
| merge in progress | `OperationInProgress: true` | skipped / operation-in-progress | no-op |
| dirty | `dirty` true | skipped / dirty-worktree, message contains "untracked" | no-op |
| dirty probe error | `dirty` errors | failed / state-probe-failed | external-failed |
| already current | head == target | already-current | no-op |
| local ahead | head!=target, isAncestor(head,target)=false, isAncestor(target,head)=true | skipped / local-ahead | no-op |
| diverged | both ancestry probes false | skipped / diverged | no-op |
| ancestry error | first isAncestor errors | failed / ancestry-probe-failed | external-failed |
| checkout changed at recheck | second `state` call returns a different branch | skipped / checkout-changed; `fastForward` NOT called (assert via call counter) | no-op |
| head moved at recheck | second `state` returns a different Head | skipped / checkout-changed; no fastForward call | no-op |
| update failed | `fastForward` errors | failed / update-failed; exactly one fastForward call | external-failed |
| post-check unverified | third `state` errors (or head != target) | failed / postcheck-unverified; After empty | external-failed |
| clean advance | ancestor, fastForward true, post state at target | advanced; Before/Target/After all set | applied |

The fakes record call counts and the sequence of `state` calls (return different values per call via a slice). Assert the negative-ancestry and error-ancestry rows produce *different* reasons — that is the spec's "distinguish a negative ancestry result from a probe error" clause, and the mutation test below proves the assert bites.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/app/ -run TestRepositorySyncLadder -count=1`
Expected: FAIL — `repositorySyncIntegration` undefined.

- [ ] **Step 3: Implement the ladder exactly as specified above**

- [ ] **Step 4: Run to verify pass**

Run: `go test ./internal/app/ -run TestRepositorySync -count=1`
Expected: PASS.

- [ ] **Step 5: Mutation-test the load-bearing guards**

Each mutation below must redden at least one test; restore after each and re-run with `-count=1`:
1. Delete the recheck (step 5's second `state` call) → the checkout-changed rows redden.
2. Collapse the ancestry error branch into the false branch → the ancestry-error row reddens (it would report local-ahead/diverged instead of failed).
3. Make the post-check failure return `advanced` → the post-check row reddens.
4. Make the dirty probe error return "not dirty" → the dirty-probe-error row reddens (probe-error-is-not-clean-absence).

- [ ] **Step 6: Commit**

```bash
git add internal/app/repository_sync.go internal/app/repository_sync_test.go
git commit -m "feat(0388): sync-integration safety ladder over injectable seams"
```

---

### Task 4: Production wiring and hermetic integration tests

**Files:**
- Modify: `internal/app/repository_sync.go` (add `RunRepositorySyncIntegration`)
- Create: `internal/app/repository_sync_integration_test.go` (build tag `integration`, prefix `TestIntegrationSync`)
- Create: `tests/test_go_integration_app_sync.sh`
- Modify: `tests/runtime-budgets.tsv` (one row for the new shard, budget mirroring `test_go_integration_app_sweep.sh`'s row)

**Interfaces:**
- Consumes: `syncSeams`, `repositorySyncIntegration` (Task 3); `loadOperationalContext` (`internal/app/operational_context.go`); `gitcli.Client` methods `WorktreeCheckoutState` (Task 1), `ChangedPaths` (status.go — porcelain, excludes ignored files), `IsAncestor`, `FastForwardWorktree`.
- Produces: `func RunRepositorySyncIntegration(ctx context.Context, d SetupDeps) OperationResult` — the entry point the CLI (Task 5) and maintenance (Task 6) call.

Production seam wiring:

```go
// RunRepositorySyncIntegration wires the ladder over the live loader and Git
// adapter. loadOperationalContext supplies discovery, pinned-blob config
// resolution, the legacy-topology refusal, and the freshly fetched pinned
// integration revision — the sync never calls the mutating repository.prepare
// workflow and never reads stale remote-tracking state.
func RunRepositorySyncIntegration(ctx context.Context, d SetupDeps) OperationResult {
	return repositorySyncIntegration(ctx, syncSeams{
		load: func(ctx context.Context) (syncContext, error) {
			oc, err := loadOperationalContext(ctx, d.Git, d.RepoDir)
			if err != nil {
				return syncContext{}, err
			}
			return syncContext{
				primaryWorktree:     oc.repo.PrimaryWorktree,
				integrationBranch:   oc.integrationBranch,
				integrationRevision: oc.integrationRevision,
			}, nil
		},
		state: d.Git.WorktreeCheckoutState,
		dirty: func(ctx context.Context, dir string) (bool, error) {
			changes, err := d.Git.ChangedPaths(ctx, dir)
			if err != nil {
				return false, err
			}
			return len(changes) > 0, nil
		},
		isAncestor: func(ctx context.Context, ancestor, descendant string) (bool, error) {
			repo, err := d.Git.Discover(ctx, gitcli.DiscoverOptions{InvocationPath: d.RepoDir})
			if err != nil {
				return false, err
			}
			return d.Git.IsAncestor(ctx, repo, gitcli.ObjectID(ancestor), gitcli.ObjectID(descendant))
		},
		fastForward: func(ctx context.Context, dir, target string) (bool, error) {
			return d.Git.FastForwardWorktree(ctx, dir, gitcli.ObjectID(target))
		},
	})
}
```

Refinement while implementing: avoid the second `Discover` in `isAncestor` by capturing `oc.repo` from `load` into the closure environment (bind the seams inside `RunRepositorySyncIntegration` after a single load, or have `syncContext` carry the `gitcli.Repository`). Prefer carrying the repository in `syncContext` — one discovery, decided-and-acted-on the same copy. Verify `ChangedPaths` uses `--porcelain` without `--ignored` (it must not count ignored files as dirt — read `internal/gitcli/status.go` and assert this in a test rather than assuming).

- [ ] **Step 1: Write the failing hermetic integration tests**

`TestIntegrationSyncIntegrationBranch` in `repository_sync_integration_test.go`. Build fixtures with the package's existing docket-topology test helpers (read `setup_testtree.go` and an existing `TestIntegrationSweep…` test first and reuse its fixture constructor — a local bare origin with a `docket` metadata branch, an integration branch, and a primary clone; do not invent a parallel fixture). Subtests:

1. **advance + idempotent repeat**: advance the bare origin's integration branch past the primary's HEAD (commit via a second writer clone, push), run `RunRepositorySyncIntegration` → `advanced`, primary HEAD equals the pushed tip; run again → `already-current`.
2. **configured integration branch differs from default**: fixture whose `.docket.yml` sets `integration_branch` to a non-default branch; only that branch's tip is synced.
3. **invocation from the metadata worktree and from a feature worktree**: run with `RepoDir` set to `<primary>/.docket` and to a `.worktrees/<slug>` path; the PRIMARY advances (assert by path in the result and on-disk HEAD), the invocation worktree does not move.
4. **skips preserve state**: parameterized over dirty tracked file, non-ignored untracked file, ignored-only file (expect NOT dirty — expect `advanced` or `already-current`), detached HEAD, checked-out other branch, in-progress merge, local-ahead (commit on the primary), diverged. For each skip: assert disposition/reason, and assert branch, HEAD, index, and the offending file are byte-identical afterward.
5. **failed fetch with a stale remote-tracking tip**: prime `refs/remotes/origin/<branch>` in the primary, then break the origin remote (point origin's URL at a nonexistent path via `git remote set-url`); run → `failed` with the loader's fetch classification, primary untouched — the stale remote-tracking tip is never used, and the envelope is NOT success (the CLI must not disguise a failed fetch as synchronization).
6. **legacy topology refusal**: a fixture without the docket metadata topology → `refused`/`failed` per the classifier's existing typed contract (assert it is the classifier's reason, not a sync-minted one).

- [ ] **Step 2: Run to verify failure**

Run: `go test -tags integration ./internal/app/ -run TestIntegrationSync -count=1`
Expected: FAIL — `RunRepositorySyncIntegration` undefined.

- [ ] **Step 3: Implement `RunRepositorySyncIntegration`** as above (with the single-discovery refinement).

- [ ] **Step 4: Run to verify pass**

Run: `go test -tags integration ./internal/app/ -run TestIntegrationSync -count=1`
Expected: PASS.

- [ ] **Step 5: Add the suite shard**

`tests/test_go_integration_app_sync.sh`, byte-parallel to `tests/test_go_integration_app_sweep.sh`:

```bash
#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_app_sync.sh — Go integration shard (change 0388):
# the real-process repository.sync-integration tests driven through the
# production entry point app.RunRepositorySyncIntegration, behind the
# `integration` build tag, prefix ^TestIntegrationSync. Declarations only —
# execution and inspection live in tests/lib/go-integration-shard.sh; the
# completeness contract is tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/app"
SHARD_PREFIX="TestIntegrationSync"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
```

Make it executable (`chmod +x`). Add its `runtime-budgets.tsv` row (copy the sweep shard's budget; the file's own header documents the row format). Prefix-disjointness note: `TestIntegrationSync` and `TestIntegrationSweep` — neither is a string prefix of the other, so contract check (4) stays satisfied; verify by running the contract.

- [ ] **Step 6: Run the contract**

Run: `bash tests/test_go_integration_contract.sh`
Expected: all `ok` lines — the new shard is discovered, selects at least one test, and every `TestIntegrationSync…` test matches exactly one runner.

- [ ] **Step 7: Commit**

```bash
git add internal/app/repository_sync.go internal/app/repository_sync_integration_test.go tests/test_go_integration_app_sync.sh tests/runtime-budgets.tsv
git commit -m "feat(0388): production sync-integration wiring + hermetic integration shard"
```

---

### Task 5: CLI verb and capability registration

**Files:**
- Modify: `internal/cli/repository.go`
- Test: `internal/cli/repository_test.go` (stub-runner dispatch test, following the existing `repositoryCheckRunner` stub pattern); `internal/cli/capability_production_test.go` and `internal/cli/capabilities_command_test.go` guards must pass

**Interfaces:**
- Consumes: `app.RunRepositorySyncIntegration` (Task 4).
- Produces: `docket repository sync-integration [--repo-dir <dir>] [--json]`, capability id `repository.sync-integration`, effects `[local-write]`.

- [ ] **Step 1: Write the failing CLI test**

Follow the file's existing stubbed-runner tests: stub a new package var `repositorySyncIntegrationRunner`, invoke `docket repository sync-integration --repo-dir <tmp>`, assert the runner received the resolved `RepoDir` and that the presenter printed the stub result. Also assert the annotation: build the root command and check the subcommand's `Annotations` carry id `repository.sync-integration` with exactly `local-write` (read how `capability_test.go` asserts annotations and mirror it).

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/cli/ -run 'Repository.*Sync|Sync.*Repository' -count=1`
Expected: FAIL.

- [ ] **Step 3: Implement**

In `repository.go`:

```go
repositorySyncIntegrationRunner = func(ctx context.Context, d app.SetupDeps) app.OperationResult {
	return app.RunRepositorySyncIntegration(ctx, d)
}
```

and in `newRepositoryCommand`:

```go
syncIntegrationCmd := repositorySubcommand("sync-integration",
	"Fast-forward the primary checkout to the freshly fetched integration tip when it is safe (explicit skips otherwise)",
	func(c *cobra.Command, deps app.SetupDeps) {
		setResult(repositorySyncIntegrationRunner(c.Context(), deps))
	},
	// local-write: fetches objects/remote-tracking state and may fast-forward
	// the primary checkout; it never pushes and changes no planning metadata.
	EffectLocalWrite)
```

added to the `AddCommand` list. No new flags beyond the helper's `--repo-dir` and the global `--json`.

- [ ] **Step 4: Run the CLI + catalog guards**

Run: `go test ./internal/cli/ -count=1`
Expected: PASS. If a catalog/production correspondence guard reddens with its own remedy (a derived expectation to refresh), follow the guard's remedy message; also read `internal/cli/install.go`'s command map (the `"repository check": true` allowlist near its top) to decide whether sync-integration belongs in it — it gates which commands some install/bootstrap path admits; add it only if the map's own header comment says every repository verb belongs, otherwise leave it out and note why in the commit body.

- [ ] **Step 5: Also re-run the schema fidelity guards** (`go test ./internal/app/ -run 'TestSchema|TestOperationBindings' -count=1`) — the Task 2 registry row now has its live command; both surfaces must agree.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/repository.go internal/cli/repository_test.go
git commit -m "feat(0388): docket repository sync-integration CLI verb (local-write)"
```

---

### Task 6: Maintenance-sweep embedding

**Files:**
- Modify: `internal/app/maintenance.go`, `internal/app/schema_registry.go` only if a guard demands re-description (MaintenanceResult's binding row already exists)
- Test: `internal/app/maintenance_test.go`

**Interfaces:**
- Consumes: `SyncOutcome`, `SyncDisp*` consts (Task 2), `RunRepositorySyncIntegration` (Task 4).
- Produces: `MaintenanceResult.IntegrationSync *SyncOutcome` (JSON `integration_sync,omitempty`); a new `sweepOps.syncIntegration func(ctx context.Context) *SyncOutcome` seam; production wiring in `MaintenanceSweep`.

Rules (spec §Maintenance and status):
- After the item loop (and the assess entries), for BOTH scopes — including a successfully initialized sweep with zero items — invoke the seam exactly once. Every whole-sweep refusal path (invalid scope, pin failure, deferred capability, snapshot refusal) returns BEFORE the sync; a cancelled context also skips it (check `ctx.Err() != nil` before invoking, and preserve cancellation — no detached work).
- The outcome rides `IntegrationSync`, never an `entries` row, and never inflates the applied count derived from entries. But a sync `advanced` upgrades an otherwise `no-op` envelope to `applied`; a sync failure NEVER downgrades or overwrites the entry-derived result or the refusal semantics.
- `HumanText` appends `; integration sync: <disposition>[ (<reason>)]` when `IntegrationSync != nil`.
- Production wiring in `MaintenanceSweep`:

```go
syncIntegration: func(ctx context.Context) *SyncOutcome {
	res := RunRepositorySyncIntegration(ctx, SetupDeps{Git: deps.Planning.Client, RepoDir: repoDir})
	if sr, ok := res.(RepositorySyncResult); ok {
		out := sr.SyncOutcome
		return &out
	}
	return &SyncOutcome{Disposition: SyncDispFailed, Reason: "internal-error", Message: "sync returned an unexpected document"}
},
```

(`RunRepositorySyncIntegration` returns `OperationResult`; if Task 4 declared its concrete return as `RepositorySyncResult`, drop the type assertion — prefer the concrete return type there so this branch is impossible.) A nil `deps.Planning.Client` returns a `failed` outcome, never a panic. Bounded, repository-level Git work only — nothing here scales with archive size, and no per-item invocation. `maintenance preflight` composes `MaintenanceSweep` (implementation scope) and therefore inherits the embed with no extra call — verify, don't re-wire.

- [ ] **Step 1: Write the failing orchestration tests** (extend the existing `maintenanceSweep` unit tests with a counting fake seam):

1. full scope, empty worklist → seam called exactly once; `IntegrationSync` populated; envelope `no-op` when the fake returns `skipped`.
2. implementation scope, multiple items with one per-item failure → seam still called exactly once, entries/dispositions unchanged from today's expectations.
3. fake returns `advanced` on an otherwise no-op sweep → envelope `applied`; applied ENTRY count unchanged (assert the count is computed from entries only).
4. fake returns `failed` on a sweep with applied entries → envelope stays `applied`; `IntegrationSync.Disposition == "failed"` reported separately.
5. whole-sweep refusal (invalid scope; pin error) → seam called zero times.
6. cancelled context before the suffix → seam called zero times.
7. nil seam (an old orchestration test) → no panic, `IntegrationSync` nil — existing tests stay green.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/app/ -run TestMaintenance -count=1` — new tests FAIL, existing PASS.

- [ ] **Step 3: Implement** the field, seam, invocation point (immediately before the final `stamp(newMaintenanceResult(...))` return, threading the outcome into the result), the applied upgrade, and the `HumanText` suffix.

- [ ] **Step 4: Run to verify pass** (`-count=1`). Also run `go test ./internal/app/ -run 'TestSchema|TestOperationBindings|TestVocabulary' -count=1` — the enlarged `MaintenanceResult` must still describe cleanly (the `IntegrationSync` field's `enum=sync_dispositions` tag rides in via `SyncOutcome`).

- [ ] **Step 5: Mutation-test**: (a) remove the refusal-path early return so sync runs on a pin failure → test 5 reddens; (b) make a sync failure overwrite the envelope with `external-failed` → test 4 reddens; (c) call the seam per-item → tests 1–2 redden (call count > 1). Restore each.

- [ ] **Step 6: Commit**

```bash
git add internal/app/maintenance.go internal/app/maintenance_test.go
git commit -m "feat(0388): maintenance sweep runs the integration sync once per scope"
```

---

### Task 7: Workflow prose, docs, and the embedded bundle

**Files:**
- Modify: `skills/docket-convention/SKILL.md`, `skills/docket-finalize-change/SKILL.md`, `skills/docket-status/SKILL.md`, `README.md` (only if it states the manual-sync posture — grep first), `internal/assets/embedded/*` (regenerated)
- Create: `tests/test_sync_integration_wiring.sh` (+ its `tests/runtime-budgets.tsv` row)

**Interfaces:**
- Consumes: the shipped `repository.sync-integration` operation (Tasks 4–5).

First, derive the sites (AGENTS.md rule — never hand-list): run `grep -rn "post-merge\|fast-forward\|integration sync\|sync-integration" --include="*.md" skills README.md docs` from the repo root, and sort hits into (a) maintained executable/skill prose to change, (b) point-in-time records (archived changes, specs, ADRs, results) to leave untouched, (c) generated copies (`internal/assets/embedded/tree/…`) that regenerate. Record the sorted list in the commit message.

Known required edits (verify against the grep, don't stop at them):

1. **docket-convention** — the terminal-close-out paragraph currently ends: "The native closeout runs no post-merge integration-branch sync; keeping a `docket`-mode integration checkout fast-forwarded after a merge is a human step, not part of the automated flow." Replace with prose stating: the closeout itself still embeds no sync; the finalize workflow's end-of-run step and both maintenance-sweep scopes run the `repository.sync-integration` operation once, which fast-forwards a clean primary checkout already on the configured integration branch to the freshly fetched tip and reports explicit skips otherwise. Check `skills/docket-convention/references/terminal-close-out.md` for the same claim and update it in the same voice.
2. **docket-finalize-change** — add one end-of-run step AFTER the batch's closeout and cleanup attempts (and after already-merged recovery / pending-retained cleanup): run the `repository.sync-integration` operation once with `--repo-dir <primary checkout path from the Step-0 prepare context> --json` — a path that survives feature-worktree removal — surface its outcome separately, and state the posture: best-effort; a sync refusal/failure is reported but never undoes or replaces completed merge/closeout/cleanup results, never writes a finalize-blocked marker, and never suppresses unrelated work; if the batch halted after earlier verified merges, still run the suffix when repository context is valid and execution is not cancelled, preserving the original halt verdict; never through failed bootstrap or unknown repository identity; a stacked child never makes its parent branch the target — the operation always resolves the configured integration branch itself. Do NOT add sync to the per-change merge/closeout steps.
3. **docket-status** — the sweep bullet already says the full-scope sweep includes "integration sync"; make the prose accurate for both scopes and add the boundary sentence: status itself stays read-only and relies on maintenance's embedded sync when it requests a sweep — it must not invoke a duplicate `repository.sync-integration`.
4. **README.md** — update only if the grep finds a manual-sync or workflow claim; otherwise leave it.

- [ ] **Step 1: Write the failing prose sentinel** `tests/test_sync_integration_wiring.sh` — house style (`set -uo pipefail`, the tree's byte-canonical `assert` helper, `# docket-suite:` header line matching what sibling shell tests carry). Sentinels are sampling, so anchor each grep to its claim (learnings: prose-guard-binds-phrase-to-claim, specified-but-unreachable), collapsing whitespace first (`tr '\n' ' ' | tr -s ' '` into a variable, then `grep <<<"$var"` — never `producer | grep -q`):

```bash
# (1) assert-detects-removal: the manual-step sentence is GONE from the convention
! grep -F "is a human step, not part of the automated flow" skills/docket-convention/SKILL.md
# (2) the finalize skill names the semantic operation in an end-of-run step
flat_finalize contains "repository.sync-integration"
# (3) producer-side reachability: finalize binds the op to the post-cleanup suffix —
#     require "sync" and "cleanup" within one collapsed-prose window (bounded gap, single interval)
# (4) the status skill carries the no-duplicate-sync boundary: "must not invoke a duplicate"
#     (or the exact phrase written in Task 7 edit 3) adjacent to "sync-integration" or "read-only"
# (5) both maintenance scopes: the convention/status prose names full AND implementation scope
#     adjacent to the sync claim
# (6) generated-copy freshness is NOT asserted here — TestEmbeddedMatchesAuthored owns it
```

Write each as a real `assert "…" '…'` line against the collapsed variables; keep every ERE to at most ONE bounded gap (learnings: stacked-gap-regex-hangs-instead-of-failing), and remember the negated grep rule: a pattern that could lead with `--` must use `grep -F --`.

- [ ] **Step 2: Run it — Expected: FAIL** (edits not yet made): `bash tests/test_sync_integration_wiring.sh`

- [ ] **Step 3: Make the prose edits** (1)–(4) above.

- [ ] **Step 4: Regenerate the embedded bundle**

Run: `go generate ./internal/assets` (it runs `go run ./cmd/genassets -repo .`), then `go test ./internal/assets/ -run TestEmbeddedMatchesAuthored -count=1`
Expected: PASS — the drift guard proves the bundle matches the authored skills.

- [ ] **Step 5: Run the sentinel — Expected: PASS.** Then mutation-test it: revert edit (1) only (restore the old sentence) → sentinel reddens; restore. Delete the finalize sync step → sentinel reddens; restore. Re-regenerate the bundle if any mutation touched skills (the drift guard otherwise reddens at the gate).

- [ ] **Step 6: Add the budgets row** for `test_sync_integration_wiring.sh` (copy a small prose-test's budget).

- [ ] **Step 7: Commit**

```bash
git add skills/ README.md internal/assets/embedded tests/test_sync_integration_wiring.sh tests/runtime-budgets.tsv
git commit -m "docs(0388): wire sync-integration into finalize/status/convention prose + sentinel"
```

---

### Task 8: Whole-suite gate and final sweep

**Files:** none new — verification only (plus any fixes it forces).

- [ ] **Step 1: Re-derive invocation sites** one last time: `grep -rn "sync-integration" --include="*.go" --include="*.md" --include="*.sh" .` (from the repo root, excluding `.git`). Confirm: exactly one CLI registration, one schema-registry row, one maintenance seam wiring, the skill/doc prose sites from Task 7, the tests — and nothing in archived changes/specs/ADRs was rewritten.

- [ ] **Step 2: Run the full configured suite** from the source checkout: `go run ./cmd/docket development test`
Expected: `SUITE` summary green. Treat `BUDGET WATCH:`/`PARALLEL-SENSITIVE:` lines as screening findings; a `SERIAL CONFIRMED OVER BUDGET:` line is authoritative — confirm serially and act (see `tests/README.md`) before claiming the gate.

- [ ] **Step 3: Spec acceptance checklist** (spec §Validation and acceptance) — walk each bullet and name the test that covers it; any uncovered bullet is a missing test to add now, not a note. In particular re-verify: only the correct primary advances from any invocation worktree (Task 4 subtest 3); unknown never authorized a write (Task 3 mutation 4, Task 4 subtest 5); no per-history amplification and no duplicate status-skill sync call (Task 6 tests 1–2, Task 7 sentinel 4).

- [ ] **Step 4: Commit any residue and stop** — the build gate, review, and PR belong to the driving workflow, not this plan.

---

## Self-review notes (already applied)

- Spec's "recheck branch identity and unfinished-operation state immediately before the advance" → Task 3 ladder step 5 + mutation 1. "Let FastForwardWorktree recheck cleanliness" → deliberate: the service does not re-probe dirt at step 5; the primitive owns that recheck (confirmed in `internal/gitcli/fastforward.go`).
- Spec's "post-check failure … does not roll back or claim the tree was untouched" → Task 3 ladder step 6 (After left empty only when unobservable; no reset ever).
- Spec's "finalize ordering / once-per-batch / halt preservation" lives in skill prose (finalize is a skill-driven workflow, not a Go composition), so its coverage is the Task 7 sentinel plus the reviewer's read — there is no Go finalize orchestrator to unit-test for it. Maintenance, which IS Go, gets the real orchestration tests (Task 6).
- The `integration_sync` field is additive and `omitempty`; old consumers of `MaintenanceResult` see no change until the field is populated.
