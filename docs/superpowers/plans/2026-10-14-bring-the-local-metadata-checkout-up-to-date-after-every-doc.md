<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0549 — Bring the local metadata checkout up to date after every docket write](../../changes/archive/2026-10-15-0549-bring-the-local-metadata-checkout-up-to-date-after-every-doc.md)**
<!-- docket:backlink:end -->

# Bring the local metadata checkout up to date after every docket write — Implementation Plan

> **For agentic workers:** this plan is executed by `docket-build` task by task (each task routed to a build tier under the `docket-build-task` contract), with a single whole-suite gate at the end. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When a docket write returns, the clone's metadata checkout on disk (shared `.docket/`, private `checkouts/<clone-id>/`) already shows the commit that write pushed, whenever that can be done safely.

**Architecture:** The transaction engine gains one optional `AppliedHook`, run once after `Execute` settles on an applied or already-applied outcome. An app-layer, best-effort, silent `followMetadataCheckout` resolves the layout, checks that the metadata worktree is attached on the metadata branch and clean, and reuses the existing in-place compare-and-swap fast-forward (`gitcli.Client.FastForwardCheckedOutBranch`). The CLI wires the hook unconditionally in `newPlanningDepsOver`, the one place the engine is assembled, and `executeRepositoryRepair`, which pushes outside the engine, calls the same function after its verified push.

**Tech Stack:** Go (module `github.com/danielhanold/docket`), real-Git integration tests behind `//go:build integration`, shard runners under `tests/`.

**Spec:** `docs/superpowers/specs/2026-10-14-bring-the-local-metadata-checkout-up-to-date-after-every-doc-design.md` (on the `docket` metadata branch; read it alongside this plan).

## Global Constraints

- Best-effort and silent: the follow step never changes a write's result, disposition, output, findings, or exit code, and reports nothing. Every error inside it is ignored.
- Never create or attach a metadata checkout from the follow step. Attaching stays `repository prepare`'s job.
- Never move the metadata branch backwards or across a divergence. Only `FastForwardCheckedOutBranch` moves it (compare-and-swap from the observed local tip; the target must descend from it).
- A checkout with uncommitted edits, an untracked file, an unfinished Git operation, or commits of its own is left untouched. The next `prepare` reports it as today.
- Same behavior in shared and private repositories. No new command, flag, config key, or output line.
- No new blocking gate of any kind.
- Engine stays git-only and layout-agnostic: `internal/repository/transaction` must not import `internal/app` or `internal/layout`.
- Cross-references in maintained source anchor on a symbol name or a verbatim-quoted clause, never a line number (`TestCommentAnchorStyle`).
- Docs describe current behavior only: no change numbers, PR numbers, or "now"/"new" phrasing in `docs/guide/`.
- Every new `tests/test_*.sh` file needs a row in `tests/runtime-budgets.tsv` and the `# docket-suite: go` declaration in its first 10 lines.
- Run every focused Go test with `-count=1` (a cached PASS proves nothing about a mutated tree).
- Mutation-test every new guard: strip what it guards, watch the test go red, restore from a backup copy (never `git checkout --` over uncommitted work).
- The whole suite runs once at the build gate via `build.test_command` (resolve it from config; for this repo it is the Go runner, `go run ./cmd/docket development test`). Read the budget report even when green.

## Spec reconciliations (read before Task 2)

1. **Clean check before the fast-forward.** The spec's design step 5 relies on `FastForwardCheckedOutBranch`'s `read-tree -u -m` refusing over local changes. But a two-tree `read-tree -u -m` only refuses a change *in the way*. A non-conflicting uncommitted edit, or a non-conflicting untracked file, is carried along while the branch moves. That breaks the spec's own table row ("uncommitted edits, untracked file in the way, or unfinished Git operation | untouched") and its "Left alone" test. So `followMetadataCheckout` runs the existing `worktreeCleanState` probe (the one `repository prepare`/`check` use) and moves only a clean checkout. That is also exactly prepare's own rule: it fast-forwards only a clean, strictly-behind checkout.
2. **Attachment check by registration.** The spec's step 3 ("the metadata worktree exists and has that branch checked out") is implemented with `ListWorktrees`: an entry whose cleaned path equals `Layout.MetadataWorktree`, whose branch is the metadata ref, and which is not prunable. It mirrors `prepareAugment`'s holder probe. Running `git -C <dir>` on a directory that is not itself a worktree would otherwise resolve to the enclosing primary worktree.
3. **Where the wiring guard lives.** The behavioral integration tests live in `internal/app`, where the shared and private fixtures already exist. The mutation check on `newPlanningDepsOver`'s wiring is a real-Git test in `internal/cli`. That package has no no-git guard, and `root_test.go`'s `newStatusFixtureRepo` already runs real Git untagged. The test invokes the wired hook and asserts the checkout moved, so deleting the wiring or wiring a no-op both turn it red.

## Review Focus

1. A non-conflicting uncommitted edit (or untracked file) in the checkout: the branch and files must stay exactly as they were. Pinned in Task 2 (`...LeavesUncommittedEditAlone`, `...LeavesUntrackedFileAlone`) and Task 3 (`...TypedWriteLeavesDirtyCheckoutAlone`).
2. A write run from a feature worktree (`.worktrees/<slug>`), the normal implement-next path: the layout must resolve from the shared common dir and move the clone's checkout. Pinned in Task 2 (`...FromLinkedWorktree`) and Task 3 (the CLI test's `feature` case).
3. An already-applied replay or a concurrent writer handing back a commit older than the local tip: the branch must never move backwards. Pinned in Task 2 (`...LeavesCheckoutAlreadyPastAlone`).
4. Private mode: the checkout lives under the XDG data home, not in the repository. Pinned in Task 3 (`...TypedWritePrivate`).
5. A different branch, or a detached HEAD, in the metadata worktree, or no worktree at all: nothing is attached or switched. Pinned in Task 2 (`...LeavesDetachedCheckoutAlone`, `...NeverAttachesAbsentCheckout`).

---

### Task 1: Engine — optional `AppliedHook`, run once after an applied outcome

Suggested tier: standard.

**Files:**
- Modify: `internal/repository/transaction/engine.go` (the `Engine` struct and its type doc; `Execute`)
- Test: `internal/repository/transaction/engine_integration_test.go` (append; prefix `TestIntegrationTxnApply`, shard `tests/test_go_integration_transaction_apply.sh`)

**Interfaces:**
- Consumes: nothing new.
- Produces: `Engine.AppliedHook func(ctx context.Context, req Request, res Result)`. It is a field set after `NewEngine` and before the first `Execute`. It runs once per `Execute`, only when `res.Disposition` is `DispositionApplied` or `DispositionAlreadyApplied` and `res.AppliedCommit != ""`. It has no return value and cannot change the result.

- [ ] **Step 1: Write the failing tests**

Append to `internal/repository/transaction/engine_integration_test.go`:

```go
// hookRecorder records every AppliedHook invocation. It is used from one
// goroutine per test (Execute calls the hook synchronously), so it needs no lock.
type hookRecorder struct {
	calls []hookCall
}

type hookCall struct {
	req Request
	res Result
}

func (h *hookRecorder) hook(_ context.Context, req Request, res Result) {
	h.calls = append(h.calls, hookCall{req: req, res: res})
}

// TestIntegrationTxnApplyEngineAppliedHookRunsOnceOnApplied proves the hook runs
// exactly once on an applied outcome, with the request and the final result.
func TestIntegrationTxnApplyEngineAppliedHookRunsOnceOnApplied(t *testing.T) {
	for _, topo := range topologies() {
		t.Run(topo.name, func(t *testing.T) {
			r := topo.build(t)
			client, repo := r.discover(t)
			eng := newEngine(t, client)
			rec := &hookRecorder{}
			eng.AppliedHook = rec.hook

			res := mustExecute(t, eng, r, repo, nil, createOp(thirdChangePath, thirdChange()))
			if res.Disposition != DispositionApplied {
				t.Fatalf("disposition = %q, want applied", res.Disposition)
			}
			if len(rec.calls) != 1 {
				t.Fatalf("hook calls = %d, want 1", len(rec.calls))
			}
			got := rec.calls[0]
			if got.res.Disposition != DispositionApplied || got.res.AppliedCommit != res.AppliedCommit {
				t.Errorf("hook result = (%q, %q), want (applied, %q)", got.res.Disposition, got.res.AppliedCommit, res.AppliedCommit)
			}
			if got.res.AppliedCommit != r.originTip(t) {
				t.Errorf("hook applied commit = %q, want origin tip %q", got.res.AppliedCommit, r.originTip(t))
			}
			if got.req.TargetRef != r.Target || got.req.Repository.CommonDir != repo.CommonDir {
				t.Errorf("hook request = (%q, %q), want (%q, %q)", got.req.TargetRef, got.req.Repository.CommonDir, r.Target, repo.CommonDir)
			}
		})
	}
}

// TestIntegrationTxnApplyEngineAppliedHookRunsOncePerExecuteAfterRetry proves the
// hook runs once per Execute, after the attempt loop settles — not per attempt.
func TestIntegrationTxnApplyEngineAppliedHookRunsOncePerExecuteAfterRetry(t *testing.T) {
	r := newMainModeRepos(t)
	client, repo := r.discover(t)
	eng := newEngine(t, client)
	rec := &hookRecorder{}
	eng.AppliedHook = rec.hook

	op := createOp(thirdChangePath, thirdChange())
	op.beforePlan = func(call int) {
		if call == 1 {
			r.advanceOrigin(t, "docs/changes/active/0005-writer-change.md",
				corpusChange(5, "writer-change", "proposed"))
		}
	}
	res := mustExecute(t, eng, r, repo, nil, op)
	if res.Disposition != DispositionApplied || res.Attempts != 2 {
		t.Fatalf("disposition/attempts = %q/%d, want applied/2", res.Disposition, res.Attempts)
	}
	if len(rec.calls) != 1 {
		t.Fatalf("hook calls = %d, want exactly 1 across two attempts", len(rec.calls))
	}
	if rec.calls[0].res.AppliedCommit != res.AppliedCommit {
		t.Errorf("hook applied commit = %q, want %q", rec.calls[0].res.AppliedCommit, res.AppliedCommit)
	}
}

// TestIntegrationTxnApplyEngineAppliedHookRunsOnAlreadyApplied proves an
// idempotent replay runs the hook with the ORIGINAL applied commit.
func TestIntegrationTxnApplyEngineAppliedHookRunsOnAlreadyApplied(t *testing.T) {
	r := newMainModeRepos(t)
	client, repo := r.discover(t)
	eng := newEngine(t, client)
	rec := &hookRecorder{}
	eng.AppliedHook = rec.hook

	first, err := eng.Execute(context.Background(), Request{
		Repository: repo, Remote: "origin", TargetRef: r.Target,
		Idempotency: keyReq(), Loader: testLoader{}, Operation: createOp(thirdChangePath, thirdChange()),
	})
	if err != nil || first.Disposition != DispositionApplied {
		t.Fatalf("first Execute: disposition %q err %v", first.Disposition, err)
	}
	replay, err := eng.Execute(context.Background(), Request{
		Repository: repo, Remote: "origin", TargetRef: r.Target,
		Idempotency: keyReq(), Loader: testLoader{}, Operation: createOp(thirdChangePath, thirdChange()),
	})
	if err != nil || replay.Disposition != DispositionAlreadyApplied {
		t.Fatalf("replay Execute: disposition %q err %v", replay.Disposition, err)
	}
	if len(rec.calls) != 2 {
		t.Fatalf("hook calls = %d, want 2 (apply + replay)", len(rec.calls))
	}
	got := rec.calls[1].res
	if got.Disposition != DispositionAlreadyApplied || got.AppliedCommit != first.AppliedCommit {
		t.Errorf("replay hook result = (%q, %q), want (already-applied, %q)", got.Disposition, got.AppliedCommit, first.AppliedCommit)
	}
}

// TestIntegrationTxnApplyEngineAppliedHookNeverRunsOtherwise proves the hook never
// runs on refused, contended, no-op, failed, or call-shape outcomes.
func TestIntegrationTxnApplyEngineAppliedHookNeverRunsOtherwise(t *testing.T) {
	firstPath := "docs/changes/active/0001-first-change.md"
	cases := []struct {
		name string
		req  func(r *testRepos, repo gitcli.Repository) Request
		want Disposition
	}{
		{"refused", func(r *testRepos, repo gitcli.Repository) Request {
			return Request{Repository: repo, Remote: "origin", TargetRef: r.Target, Loader: testLoader{},
				Operation: &scriptedOp{refuse: true, findings: []domain.Finding{
					{Code: "op-refused", Severity: domain.SeverityError, Entity: domain.EntityRef{Kind: domain.EntityRepo}},
				}}}
		}, DispositionRefused},
		{"contended", func(r *testRepos, repo gitcli.Repository) Request {
			return Request{Repository: repo, Remote: "origin", TargetRef: r.Target, Loader: testLoader{},
				Expected: []EntityExpectation{{Path: gitcli.RepoPath(firstPath),
					Revision: ExpectedRevision{Kind: RevisionBlob, ObjectID: "0000000000000000000000000000000000000000"}}},
				Operation: createOp(thirdChangePath, thirdChange())}
		}, DispositionContended},
		{"no-op", func(r *testRepos, repo gitcli.Repository) Request {
			return Request{Repository: repo, Remote: "origin", TargetRef: r.Target, Loader: testLoader{},
				Operation: &scriptedOp{}}
		}, DispositionNoOp},
		{"failed", func(r *testRepos, repo gitcli.Repository) Request {
			return Request{Repository: repo, Remote: "origin", TargetRef: "refs/heads/nope", Loader: testLoader{},
				Operation: createOp(thirdChangePath, thirdChange())}
		}, DispositionFailed},
		{"call-shape", func(r *testRepos, repo gitcli.Repository) Request {
			return Request{Repository: repo, Remote: "origin", TargetRef: "refs/tags/v1", Loader: testLoader{},
				Operation: createOp(thirdChangePath, thirdChange())}
		}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newMainModeRepos(t)
			client, repo := r.discover(t)
			eng := newEngine(t, client)
			rec := &hookRecorder{}
			eng.AppliedHook = rec.hook

			res, _ := eng.Execute(context.Background(), tc.req(r, repo))
			if res.Disposition != tc.want {
				t.Fatalf("disposition = %q, want %q", res.Disposition, tc.want)
			}
			if len(rec.calls) != 0 {
				t.Errorf("hook ran %d times on a %s outcome, want 0", len(rec.calls), tc.name)
			}
		})
	}
}
```

Check the type name of the disposition constants before running (`grep -n "DispositionApplied" internal/repository/transaction/result.go`). If the type is not `Disposition`, use the real name in the `want` field. For the call-shape row, `Execute` returns `base` with an empty disposition. Confirm that in `Execute`'s call-shape validation block. If it differs, assert on what the code actually returns, not on a guess.

- [ ] **Step 2: Run them to verify they fail**

Run: `go test -tags integration -count=1 -run '^TestIntegrationTxnApplyEngineAppliedHook' ./internal/repository/transaction/`
Expected: compile failure, `eng.AppliedHook undefined`.

- [ ] **Step 3: Implement**

In `internal/repository/transaction/engine.go`, add the field to `Engine` after `AdmissionHook`:

```go
	// AppliedHook is the optional after-apply observer. See the type doc. Set it
	// after NewEngine and before any Execute; the engine never mutates it.
	AppliedHook func(ctx context.Context, req Request, res Result)
```

Extend the `Engine` type doc comment with a paragraph after the `AdmissionHook` paragraph:

```go
// AppliedHook, when set, runs once per Execute after the attempt loop settles,
// only when the final disposition is applied or already-applied and the result
// carries an applied commit. It receives the caller's request and the final
// result, cannot change either (it has no return), and never runs on a refused,
// contended, no-op, interrupted, failed, or call-shape outcome. Like
// AdmissionHook it is OPTIONAL (nil keeps the standalone contract exactly), must
// be set once at wiring time before the first Execute, and must be safe for
// concurrent invocation. The engine stays layout-agnostic: what the hook does
// with the request's repository and target ref is the wiring layer's business.
```

Rename the current `Execute` method to `execute` (same body, same doc moved to the wrapper), and add the wrapper above it:

```go
// Execute runs the spec's exact attempt sequence and returns a typed Result. A
// non-nil Go error accompanies only a programmer/call-shape failure or an
// external/interruption outcome (where it is a typed *Failure keyed on stage and
// kind); a domain refusal, contention, no-op, or applied outcome returns a nil
// error. Engine and Client are safe for concurrent goroutines. When AppliedHook
// is set it runs once, after the outcome is final, on applied and
// already-applied outcomes only.
func (e *Engine) Execute(ctx context.Context, req Request) (Result, error) {
	res, err := e.execute(ctx, req)
	if e.AppliedHook != nil && res.AppliedCommit != "" &&
		(res.Disposition == DispositionApplied || res.Disposition == DispositionAlreadyApplied) {
		e.AppliedHook(ctx, req, res)
	}
	return res, err
}

// execute is Execute's attempt sequence, without the after-apply hook.
func (e *Engine) execute(ctx context.Context, req Request) (Result, error) {
	// ... the former Execute body, unchanged ...
}
```

Grep for any in-package caller or comment that names `Execute` as the place a specific step happens (`grep -n "Execute" internal/repository/transaction/*.go`). Leave them alone: `Execute` is still the public entry point.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -tags integration -count=1 -run '^TestIntegrationTxnApplyEngine' ./internal/repository/transaction/`
Expected: PASS (new tests plus every existing `TestIntegrationTxnApplyEngine*` test, which all run with a nil hook and prove nil changes nothing).

Also run the default-tag package tests: `go test -count=1 ./internal/repository/transaction/`. Expected: PASS.

- [ ] **Step 5: Mutation-test**

Back up `engine.go` (`cp internal/repository/transaction/engine.go "$(mktemp -d "${TMPDIR:-/tmp}/mut.XXXXXX")/engine.go"`), then one mutation at a time:
- Drop the disposition filter (call the hook whenever it is non-nil). Expected: `...NeverRunsOtherwise` goes RED.
- Move the hook call into `runAttempt` before its return on every attempt. Expected: `...RunsOncePerExecuteAfterRetry` goes RED, or the refused/contended cases go RED.
- Restore from the backup copy and re-run to green.

- [ ] **Step 6: Commit**

```bash
git add internal/repository/transaction/engine.go internal/repository/transaction/engine_integration_test.go
git commit -m "feat(transaction): add an optional after-apply hook to the engine"
```

---

### Task 2: App — `followMetadataCheckout`, the best-effort in-place follow

Suggested tier: premium (it moves a branch and working tree that a human may be reading or editing; a wrong guard silently carries or clobbers their state).

**Files:**
- Create: `internal/app/metadata_follow.go`
- Create: `internal/app/metadata_follow_integration_test.go` (prefix `TestIntegrationMetadataFollow`)
- Create: `tests/test_go_integration_app_metadatafollow.sh`
- Modify: `tests/runtime-budgets.tsv` (one new row)

**Interfaces:**
- Consumes: `resolveLayout(ctx, remoteURLReader, gitcli.Repository) (layout.Layout, error)` and `metadataRef(layout.Layout) gitcli.RefName` (`internal/app/layout_resolve.go`); `worktreeCleanState(ctx, *gitcli.Client, string) (reposetup.Presence, bool, bool)` (`internal/app/repository_check.go`); `(*gitcli.Client).ListWorktrees`, `ResolveRef`, and `FastForwardCheckedOutBranch(ctx, worktreeDir string, branch RefName, expectedTip, target ObjectID) error`.
- Produces:
  - `func followMetadataCheckout(ctx context.Context, client *gitcli.Client, repo gitcli.Repository, targetRef gitcli.RefName, pushed gitcli.ObjectID)`, no return value.
  - `func MetadataCheckoutFollower(client *gitcli.Client) func(context.Context, transaction.Request, transaction.Result)`, the hook closure Task 3 wires into the engine.
  - Test helpers (in the new test file, used by Task 3): `discoverFollowRepo(t *testing.T, dir string) gitcli.Repository`, `newFollowRepo(t *testing.T) (r *initRepo, oldTip, newTip string)`, `followAt(t *testing.T, dir, ref, pushed string)`.

- [ ] **Step 1: Write the shard runner and budget row**

Create `tests/test_go_integration_app_metadatafollow.sh` (mode 0755):

```bash
#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_app_metadatafollow.sh — Go integration shard: a docket
# write moves the clone's metadata checkout (shared .docket, private
# checkouts/<clone-id>) forward in place to the commit it just pushed when that is
# safe — clean, attached on the metadata branch, strictly behind — and leaves every
# other checkout untouched, behind the `integration` build tag, prefix
# ^TestIntegrationMetadataFollow. A new shard because no existing app prefix is a
# name-prefix of TestIntegrationMetadataFollow. Declarations only — execution and
# inspection live in tests/lib/go-integration-shard.sh; the completeness contract is
# tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/app"
SHARD_PREFIX="TestIntegrationMetadataFollow"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
```

The `assert` line must be byte-identical to the tree's canonical one (a repoguard source-hygiene rule allowlists it byte for byte). Copy it from `tests/test_go_integration_app_repoinplaceff.sh` rather than retyping it.

Add to `tests/runtime-budgets.tsv`, next to the other `test_go_integration_app_*` rows, a provisional row:

```
tests/test_go_integration_app_metadatafollow.sh	30	parallel
```

After Task 3 adds its tests to this shard, measure it solo (`time bash tests/test_go_integration_app_metadatafollow.sh`) and re-seed the ceiling with the file's own rule: the measured serial time, rounded up to the next multiple of 5, plus a 5s margin, minimum 10.

- [ ] **Step 2: Write the failing tests**

Create `internal/app/metadata_follow_integration_test.go`:

```go
//go:build integration

package app

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/testsupport"
)

// --- metadata checkout follow (TestIntegrationMetadataFollow shard) ----------------
//
// followMetadataCheckout moves the clone's metadata checkout in place to a commit a
// docket write just pushed, when it is clean, attached on the metadata branch, and
// strictly behind; everything else is left exactly as it was. Each direct scenario
// starts from a check-healthy repository whose remote docket branch is one commit
// ahead of .docket, with that commit's objects fetched into the clone (a real write
// creates its commit locally, so its objects are always present).

// discoverFollowRepo discovers the repository at dir with a fresh client.
func discoverFollowRepo(t *testing.T, dir string) gitcli.Repository {
	t.Helper()
	repo, err := newGitClient(t).Discover(context.Background(), gitcli.DiscoverOptions{InvocationPath: dir})
	if err != nil {
		t.Fatalf("Discover %s: %v", dir, err)
	}
	return repo
}

// newFollowRepo is newBehindHealthyRepo with the advanced commit fetched into the
// invocation clone. The fetch moves only refs/remotes/origin/docket, never the
// local docket branch or .docket.
func newFollowRepo(t *testing.T) (r *initRepo, oldTip, newTip string) {
	t.Helper()
	r, oldTip, newTip = newBehindHealthyRepo(t)
	runGit(t, r.invocation, "fetch", "-q", "origin", "docket")
	if head := r.dotDocketHead(t); head != oldTip {
		t.Fatalf("fixture: .docket HEAD = %s after fetch, want %s", head, oldTip)
	}
	return r, oldTip, newTip
}

// followAt runs followMetadataCheckout for the repository discovered at dir.
func followAt(t *testing.T, dir, ref, pushed string) {
	t.Helper()
	followMetadataCheckout(context.Background(), newGitClient(t), discoverFollowRepo(t, dir),
		gitcli.RefName(ref), gitcli.ObjectID(pushed))
}

func localDocketTip(t *testing.T, r *initRepo) string {
	t.Helper()
	return runGit(t, r.invocation, "rev-parse", "refs/heads/docket")
}

func TestIntegrationMetadataFollowFastForwardsCleanBehindCheckout(t *testing.T) {
	r, _, newTip := newFollowRepo(t)
	followAt(t, r.invocation, "refs/heads/docket", newTip)
	if head := r.dotDocketHead(t); head != newTip {
		t.Fatalf(".docket HEAD = %s, want the pushed commit %s", head, newTip)
	}
	if tip := localDocketTip(t, r); tip != newTip {
		t.Fatalf("local docket branch = %s, want %s", tip, newTip)
	}
	b, err := os.ReadFile(filepath.Join(r.invocation, ".docket", "notes", "advance.txt"))
	if err != nil || string(b) != "advanced\n" {
		t.Fatalf("pushed file on disk = %q, %v; want the pushed content", b, err)
	}
	requireCheckHealthy(t, r)
}

// TestIntegrationMetadataFollowFromLinkedWorktree runs the follow from a linked
// feature worktree, as a write from .worktrees/<slug> does: the metadata checkout is
// per clone, resolved from the shared common dir.
func TestIntegrationMetadataFollowFromLinkedWorktree(t *testing.T) {
	r, _, newTip := newFollowRepo(t)
	feat := filepath.Join(testsupport.TempDir(t), "feat")
	runGit(t, r.invocation, "worktree", "add", "-q", "-b", "feat", feat, "main")
	followAt(t, feat, "refs/heads/docket", newTip)
	if head := r.dotDocketHead(t); head != newTip {
		t.Fatalf(".docket HEAD = %s, want %s (follow from a linked worktree)", head, newTip)
	}
}

func TestIntegrationMetadataFollowIgnoresOtherTargetRef(t *testing.T) {
	r, oldTip, newTip := newFollowRepo(t)
	followAt(t, r.invocation, "refs/heads/main", newTip)
	if head := r.dotDocketHead(t); head != oldTip {
		t.Fatalf(".docket HEAD = %s, want it untouched at %s for a non-metadata target", head, oldTip)
	}
}

// TestIntegrationMetadataFollowLeavesUncommittedEditAlone edits a tracked file the
// pushed commit does NOT touch. read-tree -u -m alone would carry that edit across
// and move the branch; the clean check must leave everything as it was.
func TestIntegrationMetadataFollowLeavesUncommittedEditAlone(t *testing.T) {
	r, oldTip, newTip := newFollowRepo(t)
	dot := filepath.Join(r.invocation, ".docket")
	tracked := strings.Fields(runGit(t, dot, "ls-files"))
	if len(tracked) == 0 {
		t.Fatal("fixture: .docket tracks no files")
	}
	edited := filepath.Join(dot, tracked[0])
	orig, err := os.ReadFile(edited)
	if err != nil {
		t.Fatal(err)
	}
	want := string(orig) + "\nlocal edit\n"
	if err := os.WriteFile(edited, []byte(want), 0o644); err != nil {
		t.Fatal(err)
	}
	followAt(t, r.invocation, "refs/heads/docket", newTip)
	if head := r.dotDocketHead(t); head != oldTip {
		t.Fatalf(".docket HEAD = %s, want it untouched at %s", head, oldTip)
	}
	if tip := localDocketTip(t, r); tip != oldTip {
		t.Fatalf("local docket branch = %s, want %s", tip, oldTip)
	}
	if b, _ := os.ReadFile(edited); string(b) != want {
		t.Fatalf("local edit lost: %q", b)
	}
	if _, err := os.Stat(filepath.Join(dot, "notes", "advance.txt")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("pushed file appeared in a dirty checkout (err=%v)", err)
	}
}

func TestIntegrationMetadataFollowLeavesUntrackedFileAlone(t *testing.T) {
	r, oldTip, newTip := newFollowRepo(t)
	dot := filepath.Join(r.invocation, ".docket")
	scratch := filepath.Join(dot, "notes", "scratch.txt")
	writeRepoFile(t, dot, "notes/scratch.txt", "mine\n")
	followAt(t, r.invocation, "refs/heads/docket", newTip)
	if head := r.dotDocketHead(t); head != oldTip {
		t.Fatalf(".docket HEAD = %s, want it untouched at %s", head, oldTip)
	}
	if b, _ := os.ReadFile(scratch); string(b) != "mine\n" {
		t.Fatalf("untracked file changed: %q", b)
	}
}

// TestIntegrationMetadataFollowLeavesLocalCommitAlone: the local branch carries its
// own commit, so the pushed commit does not descend from it (diverged).
func TestIntegrationMetadataFollowLeavesLocalCommitAlone(t *testing.T) {
	r, _, newTip := newFollowRepo(t)
	local := r.commitInDocket(t, "notes/local.txt", "local\n", "local commit")
	followAt(t, r.invocation, "refs/heads/docket", newTip)
	if head := r.dotDocketHead(t); head != local {
		t.Fatalf(".docket HEAD = %s, want it untouched at the local commit %s", head, local)
	}
}

// TestIntegrationMetadataFollowLeavesCheckoutAlreadyPastAlone hands the follow a
// commit OLDER than the local tip (an already-applied replay, or a concurrent write
// that already moved the checkout): it must never move the branch backwards.
func TestIntegrationMetadataFollowLeavesCheckoutAlreadyPastAlone(t *testing.T) {
	r, oldTip, newTip := newFollowRepo(t)
	followAt(t, r.invocation, "refs/heads/docket", newTip)
	if head := r.dotDocketHead(t); head != newTip {
		t.Fatalf("setup: .docket HEAD = %s, want %s", head, newTip)
	}
	followAt(t, r.invocation, "refs/heads/docket", oldTip)
	if head := r.dotDocketHead(t); head != newTip {
		t.Fatalf(".docket HEAD = %s, want it to stay at %s (never backwards)", head, newTip)
	}
}

func TestIntegrationMetadataFollowLeavesDetachedCheckoutAlone(t *testing.T) {
	r, oldTip, newTip := newFollowRepo(t)
	dot := filepath.Join(r.invocation, ".docket")
	runGit(t, dot, "checkout", "-q", "--detach")
	followAt(t, r.invocation, "refs/heads/docket", newTip)
	if head := r.dotDocketHead(t); head != oldTip {
		t.Fatalf("detached .docket HEAD = %s, want %s", head, oldTip)
	}
	if tip := localDocketTip(t, r); tip != oldTip {
		t.Fatalf("local docket branch = %s, want %s (no checkout holds it)", tip, oldTip)
	}
}

func TestIntegrationMetadataFollowNeverAttachesAbsentCheckout(t *testing.T) {
	r, oldTip, newTip := newFollowRepo(t)
	removeDocketWorktree(t, r)
	followAt(t, r.invocation, "refs/heads/docket", newTip)
	if _, err := os.Stat(filepath.Join(r.invocation, ".docket")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf(".docket was (re)created by the follow (err=%v)", err)
	}
	if tip := localDocketTip(t, r); tip != oldTip {
		t.Fatalf("local docket branch = %s, want %s", tip, oldTip)
	}
}
```

Before running, read `removeDocketWorktree` in `internal/app/repoinplaceff_integration_test.go` and confirm it leaves the local `docket` branch in place. If it deletes the branch, replace the last assertion with one that `refs/heads/docket` is still absent (`git rev-parse --verify --quiet` fails). Also confirm `writeRepoFile` creates parent directories (it is used for nested paths elsewhere, for example `notes/advance.txt` in `advanceRemoteDocket`).

- [ ] **Step 3: Run them to verify they fail**

Run: `go test -tags integration -count=1 -run '^TestIntegrationMetadataFollow' ./internal/app/`
Expected: compile failure, `undefined: followMetadataCheckout`.

- [ ] **Step 4: Implement**

Create `internal/app/metadata_follow.go`:

```go
package app

import (
	"context"
	"path/filepath"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/reposetup"
	"github.com/danielhanold/docket/internal/repository/transaction"
)

// This file keeps the clone's metadata checkout (shared .docket, private
// checkouts/<clone-id>) current after a docket write. A write commits in a
// throwaway transaction worktree and pushes straight to the metadata remote; this
// step then moves the local metadata branch and its checkout forward IN PLACE to
// the pushed commit, by the same compare-and-swap fast-forward `repository
// prepare` uses (gitcli's FastForwardCheckedOutBranch). It is best-effort and
// silent: it never attaches or creates a checkout, never moves one that is dirty,
// detached, ahead, diverged, or already past the pushed commit, and swallows every
// error — the next prepare sees the same state it sees today and reports it.

// MetadataCheckoutFollower returns the transaction engine's AppliedHook closure:
// after an applied or already-applied metadata write, follow the clone's metadata
// checkout to the applied commit. The engine invokes it only with a non-empty
// applied commit; it is safe for concurrent invocation (all state is per call).
func MetadataCheckoutFollower(client *gitcli.Client) func(context.Context, transaction.Request, transaction.Result) {
	return func(ctx context.Context, req transaction.Request, res transaction.Result) {
		followMetadataCheckout(ctx, client, req.Repository, req.TargetRef, res.AppliedCommit)
	}
}

// followMetadataCheckout fast-forwards repo's metadata checkout to pushed when
// targetRef is the layout's metadata ref and the checkout is attached on that
// branch, clean, and strictly behind pushed. Anything else — a resolution or probe
// error included — leaves the checkout exactly as it was. The pushed commit was
// created in this repository, so no fetch is needed.
func followMetadataCheckout(ctx context.Context, client *gitcli.Client, repo gitcli.Repository, targetRef gitcli.RefName, pushed gitcli.ObjectID) {
	if client == nil || pushed == "" {
		return
	}
	lay, err := resolveLayout(ctx, client, repo)
	if err != nil {
		return
	}
	metaRef := metadataRef(lay)
	if targetRef != metaRef {
		return
	}
	worktree := lay.MetadataWorktree
	if !metadataCheckoutAttached(ctx, client, repo, worktree, metaRef) {
		return
	}
	// read-tree -u -m refuses only a change IN THE WAY and carries any other local
	// edit across, so only a clean checkout moves — prepare's own rule.
	if clean, _, _ := worktreeCleanState(ctx, client, worktree); clean != reposetup.PresencePresent {
		return
	}
	localTip, err := client.ResolveRef(ctx, repo, metaRef)
	if err != nil || localTip == pushed {
		return
	}
	// Refuses (nothing changed) unless pushed descends from localTip and the branch
	// is still at localTip, so it never moves backwards or across a divergence.
	_ = client.FastForwardCheckedOutBranch(ctx, worktree, metaRef, localTip, pushed)
}

// metadataCheckoutAttached reports whether a live worktree registration sits at
// worktree with metaRef checked out. Any probe error reads as not attached, which
// only ever leaves the checkout alone.
func metadataCheckoutAttached(ctx context.Context, client *gitcli.Client, repo gitcli.Repository, worktree string, metaRef gitcli.RefName) bool {
	wts, err := client.ListWorktrees(ctx, repo)
	if err != nil {
		return false
	}
	for _, wt := range wts {
		if wt.Branch == metaRef && !wt.Prunable && filepath.Clean(wt.Path) == filepath.Clean(worktree) {
			return true
		}
	}
	return false
}
```

If `internal/app` gets an import cycle on `transaction` (it should not: `runtracker_fence.go` already imports it), stop and report rather than restructure.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test -tags integration -count=1 -run '^TestIntegrationMetadataFollow' ./internal/app/`
Expected: PASS, all nine tests.

If `...FastForwardsCleanBehindCheckout` or `...FromLinkedWorktree` fails with HEAD unchanged, the likely cause is a path-spelling mismatch between `Layout.MetadataWorktree` and the registered worktree path (macOS `/var` vs `/private/var`). Print both and compare. Fix by comparing canonical paths (`filepath.EvalSymlinks` on both, falling back to `filepath.Clean` on error) inside `metadataCheckoutAttached`. Do not weaken the test.

Then run the shard runner: `bash tests/test_go_integration_app_metadatafollow.sh`. Expected: all `ok`.

- [ ] **Step 6: Mutation-test**

Back up `metadata_follow.go` to a `mktemp -d "${TMPDIR:-/tmp}/mut.XXXXXX"` directory, then one at a time:
- Delete the `worktreeCleanState` check. Expected: `...LeavesUncommittedEditAlone` goes RED. If it stays green, the edit landed in a file the pushed commit also touches. Investigate the fixture rather than accept a vacuous guard (learning: residual-is-for-undetectable-not-unprobed).
- Delete the `targetRef != metaRef` check. Expected: `...IgnoresOtherTargetRef` goes RED.
- Make `metadataCheckoutAttached` always return true. Expected: `...LeavesDetachedCheckoutAlone` or `...NeverAttachesAbsentCheckout` goes RED, or report precisely which one stays green and why (FastForwardCheckedOutBranch's own symbolic-ref check may also catch the detached case; that overlap is acceptable only if at least one of the two reddens).
- Restore from the backup copy and re-run to green.

- [ ] **Step 7: Commit**

```bash
git add internal/app/metadata_follow.go internal/app/metadata_follow_integration_test.go tests/test_go_integration_app_metadatafollow.sh tests/runtime-budgets.tsv
git commit -m "feat(app): follow the metadata checkout to a pushed metadata commit"
```

---

### Task 3: Wire the hook where the engine is built; prove typed writes move the checkout

Suggested tier: standard.

**Files:**
- Modify: `internal/cli/change.go` (`newPlanningDepsOver`)
- Create: `internal/cli/planning_deps_test.go` (untagged; runs in the default `internal/cli` test binary)
- Modify: `internal/app/metadata_follow_integration_test.go` (append the typed-write scenarios)
- Modify: `tests/runtime-budgets.tsv` (re-seed the `metadatafollow` row after measuring)

**Interfaces:**
- Consumes: `app.MetadataCheckoutFollower(client *gitcli.Client) func(context.Context, transaction.Request, transaction.Result)` (Task 2); `transaction.Engine.AppliedHook` (Task 1); test helpers from Task 2 (`newFollowRepo` is not needed here; `discoverFollowRepo` is).
- Produces: every engine built by `newPlanningDeps` / `newPlanningDepsOver` (CLI typed writes, maintenance sweep closeouts, finalize) carries the follower.

- [ ] **Step 1: Write the failing CLI wiring test**

Create `internal/cli/planning_deps_test.go`:

```go
package cli

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/app"
	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/repository/transaction"
	"github.com/danielhanold/docket/internal/testsupport"
)

// followGitOut runs git -C dir args and returns trimmed stdout, failing the test
// on any error. It uses testsupport.GitEnv like statusGit.
func followGitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), testsupport.GitEnv(t)...)
	var out, errBuf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errBuf
	if err := cmd.Run(); err != nil {
		t.Fatalf("git -C %s %s: %v: %s", dir, strings.Join(args, " "), err, errBuf.String())
	}
	return strings.TrimSpace(out.String())
}

// newFollowFixture builds a shared-layout clone whose attached .docket worktree is
// one commit behind origin's docket branch, with that commit's objects fetched
// into the clone. It returns the clone and the newer docket commit.
func newFollowFixture(t *testing.T) (invocation, newTip string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found on PATH")
	}
	t.Setenv("XDG_CONFIG_HOME", testsupport.TempDir(t))
	root := testsupport.TempDir(t)
	origin := filepath.Join(root, "origin.git")
	writer := filepath.Join(root, "writer")
	invocation = filepath.Join(root, "invocation")

	statusGit(t, root, "init", "--bare", "-b", "main", origin)
	statusGit(t, root, "init", "-b", "main", writer)
	statusGit(t, writer, "config", "user.name", "t")
	statusGit(t, writer, "config", "user.email", "t@t")
	statusGit(t, writer, "config", "commit.gpgsign", "false")
	statusWriteFile(t, writer, "README.md", "readme\n")
	statusGit(t, writer, "add", "-A")
	statusGit(t, writer, "commit", "-q", "-m", "main content")
	statusGit(t, writer, "remote", "add", "origin", origin)
	statusGit(t, writer, "push", "-q", "-u", "origin", "main")
	statusGit(t, writer, "checkout", "-q", "--orphan", "docket")
	statusGit(t, writer, "rm", "-rf", "-q", ".")
	statusWriteFile(t, writer, "docs/changes/BOARD.md", "# Board\n")
	statusGit(t, writer, "add", "-A")
	statusGit(t, writer, "commit", "-q", "-m", "docket: initialize metadata branch")
	statusGit(t, writer, "push", "-q", "-u", "origin", "docket")

	statusGit(t, root, "clone", "-q", origin, invocation)
	statusGit(t, invocation, "worktree", "add", "-q", "-b", "docket", filepath.Join(invocation, ".docket"), "origin/docket")

	statusWriteFile(t, writer, "docs/changes/BOARD.md", "# Board\n\nupdated\n")
	statusGit(t, writer, "commit", "-q", "-am", "docket: board refresh")
	statusGit(t, writer, "push", "-q", "origin", "docket")
	statusGit(t, invocation, "fetch", "-q", "origin", "docket")
	return invocation, followGitOut(t, invocation, "rev-parse", "refs/remotes/origin/docket")
}

// TestPlanningDepsWireMetadataCheckoutFollower pins the wiring in
// newPlanningDepsOver: with and without a feature repoDir, the assembled engine's
// AppliedHook moves the clone's .docket checkout to the applied commit. Deleting
// the wiring (nil hook) or wiring a no-op both fail it.
func TestPlanningDepsWireMetadataCheckoutFollower(t *testing.T) {
	for _, tc := range []struct {
		name        string
		withRepoDir bool
	}{{"standalone", false}, {"feature", true}} {
		t.Run(tc.name, func(t *testing.T) {
			invocation, newTip := newFollowFixture(t)
			client, err := gitcli.NewClient()
			if err != nil {
				t.Fatal(err)
			}
			var deps app.PlanningDeps
			if tc.withRepoDir {
				deps, err = newPlanningDepsOver(client, invocation)
			} else {
				deps, err = newPlanningDepsOver(client)
			}
			if err != nil {
				t.Fatal(err)
			}
			engine, ok := deps.Engine.(*transaction.Engine)
			if !ok {
				t.Fatalf("deps.Engine is %T, want *transaction.Engine", deps.Engine)
			}
			if engine.AppliedHook == nil {
				t.Fatal("newPlanningDepsOver left the engine's AppliedHook unset")
			}
			repo, err := client.Discover(context.Background(), gitcli.DiscoverOptions{InvocationPath: invocation})
			if err != nil {
				t.Fatal(err)
			}
			engine.AppliedHook(context.Background(),
				transaction.Request{Repository: repo, Remote: "origin", TargetRef: "refs/heads/docket"},
				transaction.Result{Disposition: transaction.DispositionApplied, AppliedCommit: gitcli.ObjectID(newTip)})
			dot := filepath.Join(invocation, ".docket")
			if head := followGitOut(t, dot, "rev-parse", "HEAD"); head != newTip {
				t.Fatalf(".docket HEAD = %s, want the applied commit %s", head, newTip)
			}
			if b, err := os.ReadFile(filepath.Join(dot, "docs", "changes", "BOARD.md")); err != nil || string(b) != "# Board\n\nupdated\n" {
				t.Fatalf("BOARD.md on disk = %q, %v; want the applied board", b, err)
			}
		})
	}
}
```

If `statusGit(... "rm", "-rf", "-q", ".")` rejects `-q` in that position, drop `-q` (`newStatusFixtureRepo` uses `rm -rf .`).

- [ ] **Step 2: Run it to verify it fails**

Run: `go test -count=1 -run '^TestPlanningDepsWireMetadataCheckoutFollower$' ./internal/cli/`
Expected: FAIL, "newPlanningDepsOver left the engine's AppliedHook unset".

- [ ] **Step 3: Wire the hook**

In `internal/cli/change.go`, `newPlanningDepsOver`, after the `AdmissionHook` block and before the `return`:

```go
	// Follow the clone's metadata checkout after every applied metadata write, at
	// the same single assembly point, so every Engine.Execute caller is covered —
	// unconditionally, unlike the fence: a maintenance or standalone build (a sweep
	// closeout included) moves the checkout too. Best-effort and silent; see
	// app.MetadataCheckoutFollower.
	engine.AppliedHook = app.MetadataCheckoutFollower(client)
```

- [ ] **Step 4: Run it to verify it passes**

Run: `go test -count=1 -run '^TestPlanningDepsWireMetadataCheckoutFollower$' ./internal/cli/`
Expected: PASS (both subtests).

Then run the whole default-tag CLI package: `go test -count=1 ./internal/cli/`. Expected: PASS. A failure here most likely means an existing CLI test drives a real typed write and asserted the checkout stayed behind. Read it and update it to the new current behavior only if it is asserting the old lag; otherwise report it.

- [ ] **Step 5: Mutation-test the wiring**

Back up `internal/cli/change.go`. Then:
- Delete the `engine.AppliedHook = ...` line. Expected: RED ("left the engine's AppliedHook unset").
- Replace it with `engine.AppliedHook = func(context.Context, transaction.Request, transaction.Result) {}`. Expected: RED (".docket HEAD = ..."). Add the `context` import only for the duration of the mutation.
- Restore from the backup copy and re-run to green.

- [ ] **Step 6: Write the typed-write integration tests**

Append to `internal/app/metadata_follow_integration_test.go` (add `"github.com/danielhanold/docket/internal/repository/transaction"` to its imports):

```go
// --- typed writes through an engine wired like newPlanningDepsOver ----------------

// followingPlanningDepsFor is planningDepsFor with the engine's AppliedHook wired to
// MetadataCheckoutFollower, as internal/cli's newPlanningDepsOver wires it.
func followingPlanningDepsFor(t *testing.T, dir string) realNode {
	t.Helper()
	client := newGitClient(t)
	engine, err := transaction.NewEngine(client, testClock())
	if err != nil {
		t.Fatalf("transaction.NewEngine: %v", err)
	}
	engine.AppliedHook = MetadataCheckoutFollower(client)
	return realNode{dir: dir, deps: PlanningDeps{
		Client: client, Engine: engine, Reader: NewGitStatusReader(client), Clock: testClock(),
	}}
}

const followBoardPath = "docs/changes/BOARD.md"

func TestIntegrationMetadataFollowTypedWriteShared(t *testing.T) {
	r := newHealthyRepo(t)
	dot := filepath.Join(r.invocation, ".docket")
	boardBefore, err := os.ReadFile(filepath.Join(dot, followBoardPath))
	if err != nil {
		t.Fatal(err)
	}
	node := followingPlanningDepsFor(t, r.invocation)
	created := ChangeCreate(context.Background(), node.deps, node.dir, validChangeCreateRequest())
	if created.Result != ResultApplied {
		t.Fatalf("change create = %q (%s, findings %v)", created.Result, created.HumanText(), created.Findings)
	}
	if head := r.dotDocketHead(t); head != created.Revision {
		t.Fatalf(".docket HEAD = %s, want the pushed commit %s", head, created.Revision)
	}
	onDisk, err := os.ReadFile(filepath.Join(dot, followBoardPath))
	if err != nil {
		t.Fatal(err)
	}
	onBranch, ok := originFile(t, r.origin, "docket", followBoardPath)
	if !ok {
		t.Fatalf("origin docket has no %s", followBoardPath)
	}
	if strings.TrimSpace(string(onDisk)) != strings.TrimSpace(onBranch) {
		t.Fatalf("BOARD.md on disk differs from the branch:\n--- disk\n%s\n--- branch\n%s", onDisk, onBranch)
	}
	if string(onDisk) == string(boardBefore) {
		t.Fatal("BOARD.md on disk is unchanged; the create did not reach the checkout")
	}
	requireCheckHealthy(t, r)
}

func TestIntegrationMetadataFollowTypedWriteLeavesDirtyCheckoutAlone(t *testing.T) {
	r := newHealthyRepo(t)
	dot := filepath.Join(r.invocation, ".docket")
	before := r.dotDocketHead(t)
	tracked := strings.Fields(runGit(t, dot, "ls-files"))
	if len(tracked) == 0 {
		t.Fatal("fixture: .docket tracks no files")
	}
	// Edit a tracked file the create does not write (the board and the new record
	// are the create's paths), so the edit is never in the way of read-tree.
	var edited string
	for _, p := range tracked {
		if p != followBoardPath && !strings.HasPrefix(p, "docs/changes/active/") {
			edited = filepath.Join(dot, p)
			break
		}
	}
	if edited == "" {
		t.Fatalf("fixture: no tracked file outside the create's paths in %v", tracked)
	}
	orig, err := os.ReadFile(edited)
	if err != nil {
		t.Fatal(err)
	}
	want := string(orig) + "\nlocal edit\n"
	if err := os.WriteFile(edited, []byte(want), 0o644); err != nil {
		t.Fatal(err)
	}
	node := followingPlanningDepsFor(t, r.invocation)
	created := ChangeCreate(context.Background(), node.deps, node.dir, validChangeCreateRequest())
	if created.Result != ResultApplied {
		t.Fatalf("change create = %q (%s), want applied despite the dirty checkout", created.Result, created.HumanText())
	}
	if head := r.dotDocketHead(t); head != before {
		t.Fatalf(".docket HEAD = %s, want it untouched at %s", head, before)
	}
	if b, _ := os.ReadFile(edited); string(b) != want {
		t.Fatalf("local edit lost: %q", b)
	}
}

func TestIntegrationMetadataFollowTypedWritePrivate(t *testing.T) {
	r, _ := newPrivateInitRepo(t, nil)
	if res := r.runInitWith(t, InitOptions{Private: true}); res.Result != ResultApplied {
		t.Fatalf("init --private = %q (%s), want applied", res.Result, res.HumanText())
	}
	lay := resolvedPrivateLayout(t, r.invocation)
	before := runGit(t, lay.MetadataWorktree, "rev-parse", "HEAD")

	node := followingPlanningDepsFor(t, r.invocation)
	created := ChangeCreate(context.Background(), node.deps, node.dir, validChangeCreateRequest())
	if created.Result != ResultApplied {
		t.Fatalf("change create = %q (%s, findings %v)", created.Result, created.HumanText(), created.Findings)
	}
	head := runGit(t, lay.MetadataWorktree, "rev-parse", "HEAD")
	if head == before || head != created.Revision {
		t.Fatalf("private checkout HEAD = %s (was %s), want the pushed commit %s", head, before, created.Revision)
	}
	onDisk, err := os.ReadFile(filepath.Join(lay.MetadataWorktree, followBoardPath))
	if err != nil {
		t.Fatal(err)
	}
	onBranch, ok := originFile(t, lay.DefaultBareRemote, privateFixtureBranch, followBoardPath)
	if !ok {
		t.Fatalf("dckt store has no %s", followBoardPath)
	}
	if strings.TrimSpace(string(onDisk)) != strings.TrimSpace(onBranch) {
		t.Fatalf("private BOARD.md on disk differs from the branch:\n--- disk\n%s\n--- branch\n%s", onDisk, onBranch)
	}
}
```

Fixture notes for the builder:
- If `ChangeCreate` on `newHealthyRepo` or on the private repo refuses for a reason unrelated to this change (for example a missing test policy), mirror `newPrivateLeakWorkspaceFor`'s setup (`runConfigureTestsWith` with `passingGateScript`) rather than loosening any assertion.
- If `originFile` returns content without the trailing newline the file on disk has, the `TrimSpace` comparison already absorbs it. Do not compare raw bytes against a trimmed oracle.
- If the shared fixture's `.docket` tracks only paths under `docs/changes/` that the create writes, pick another tracked path the create provably does not write (for example the ADR index or `docs/changes/README.md`). Confirm by listing the create's commit paths with `git diff-tree --no-commit-id --name-only -r <created.Revision>`.

- [ ] **Step 7: Run them**

Run: `go test -tags integration -count=1 -run '^TestIntegrationMetadataFollow' ./internal/app/`
Expected: PASS (Task 2's nine plus these three).

Mutation: in `followingPlanningDepsFor`, comment out the `engine.AppliedHook =` line. Expected: `...TypedWriteShared` and `...TypedWritePrivate` go RED. Restore.

- [ ] **Step 8: Re-seed the shard budget**

Run `time bash tests/test_go_integration_app_metadatafollow.sh` on a quiet machine. Set the row's ceiling in `tests/runtime-budgets.tsv` to the measured seconds, rounded up to the next multiple of 5, plus 5 (minimum 10). Keep `parallel`.

- [ ] **Step 9: Commit**

```bash
git add internal/cli/change.go internal/cli/planning_deps_test.go internal/app/metadata_follow_integration_test.go tests/runtime-budgets.tsv
git commit -m "feat(cli): follow the metadata checkout after every applied metadata write"
```

---

### Task 4: `repository repair` follows the checkout to the repair commit

Suggested tier: economy.

**Files:**
- Modify: `internal/app/repository_repair.go` (`executeRepositoryRepair`)
- Test: `internal/app/repository_repair_integration_test.go` (append; prefix `TestIntegrationRepoRepair`, shard `tests/test_go_integration_app_reporepair.sh`)

**Interfaces:**
- Consumes: `followMetadataCheckout(ctx, client *gitcli.Client, repo gitcli.Repository, targetRef gitcli.RefName, pushed gitcli.ObjectID)` (Task 2).
- Produces: nothing new.

- [ ] **Step 1: Write the failing test**

Append to `internal/app/repository_repair_integration_test.go` (add `"os"` to its imports):

```go
// TestIntegrationRepoRepairFollowsMetadataCheckout proves an applied repair leaves
// the clean .docket checkout at the repair commit, with the recomputed board on
// disk — repair pushes outside the transaction engine, so it follows itself.
func TestIntegrationRepoRepairFollowsMetadataCheckout(t *testing.T) {
	r := newHealthyRepo(t)
	r.publishHealthyDrift(t)

	res := r.runRepair(t, RepairOptions{Authorized: true})
	if res.Result != ResultApplied {
		t.Fatalf("repair = %q (%s), want applied", res.Result, res.HumanText())
	}
	if head := r.dotDocketHead(t); head != res.MetadataTip {
		t.Fatalf(".docket HEAD = %s, want the repair commit %s", head, res.MetadataTip)
	}
	board, err := os.ReadFile(filepath.Join(r.invocation, ".docket", "docs", "changes", "BOARD.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(board), "hand-written stale board") {
		t.Fatalf("BOARD.md on disk still carries the stale board:\n%s", board)
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test -tags integration -count=1 -run '^TestIntegrationRepoRepairFollowsMetadataCheckout$' ./internal/app/`
Expected: FAIL, ".docket HEAD = <drift tip>, want the repair commit ...".

- [ ] **Step 3: Implement**

In `internal/app/repository_repair.go`, `executeRepositoryRepair`, between the `if rev.Commit != commit { ... }` block and `return repairApplied(...)`:

```go
	// Repair pushes outside the transaction engine, so the engine's AppliedHook
	// never sees it: follow the clone's metadata checkout to the verified repair
	// commit here. Best-effort and silent, exactly as after an engine write.
	followMetadataCheckout(ctx, git, sc.repo, docketRef, commit)
```

- [ ] **Step 4: Run the repair shard**

Run: `bash tests/test_go_integration_app_reporepair.sh`
Expected: all `ok`. If an existing repair test asserted `.docket` stayed behind after an applied repair, it was asserting the old lag. Update only that assertion to the new current behavior and say so in the commit body.

- [ ] **Step 5: Mutation-test**

Back up `repository_repair.go`, delete the `followMetadataCheckout` call, and re-run the new test. Expected: RED. Restore from the backup copy.

- [ ] **Step 6: Commit**

```bash
git add internal/app/repository_repair.go internal/app/repository_repair_integration_test.go
git commit -m "feat(app): follow the metadata checkout after an applied repository repair"
```

---

### Task 5: Docs — describe the follow in the metadata-location guide

Suggested tier: economy.

**Files:**
- Modify: `docs/guide/where-the-metadata-lives.md` (section *The `.docket/` metadata worktree*)

**Interfaces:**
- Consumes: the behavior shipped by Tasks 1–4.
- Produces: nothing.

- [ ] **Step 1: Edit the guide**

In *The `.docket/` metadata worktree*, replace the first paragraph's closing sentences:

```markdown
check (`docket repository prepare`) attaches the persistent worktree at `.docket/` and brings it up
to date with the remote before any read. **Your main working
tree never switches branches.**
```

with:

```markdown
check (`docket repository prepare`) attaches the persistent worktree at `.docket/` and brings it up
to date with the remote before any read. Every docket write also moves the worktree forward to the
commit it just pushed whenever it safely can, so the files there, the board included, are current
as soon as the write returns. A worktree with local edits or commits of its own is left alone for
the next startup check to report. `docket open board` brings the worktree up to date and opens the
board from it. **Your main working tree never switches branches.**
```

Re-wrap to the file's existing line width (about 100 columns). Do not add change numbers or "now".

- [ ] **Step 2: Sweep for contradicting prose**

Grep the shipped docs for claims that only `prepare` refreshes the metadata checkout, and correct any that the new behavior falsifies:

```bash
grep -rn -e "only .*prepare" -e "next workflow" -e "stays behind" -e "until .*prepare" docs/ README.md skills/ 2>/dev/null
```

Fix only statements this change makes false. Leave point-in-time records (specs, archived changes, Accepted ADRs) untouched. If a skill body under `skills/` changes, run `go generate ./internal/assets/` and commit the regenerated assets with it.

- [ ] **Step 3: Run the doc guards**

Run: `go test -count=1 ./internal/repoguard/`
Expected: PASS. Then find any test that reads this guide (`grep -rln "where-the-metadata-lives" internal tests`) and run it. If a prose guard pins the old sentence verbatim, update the guard to bind the new claim (phrase bound to what it asserts), and mutation-test it.

- [ ] **Step 4: Commit**

```bash
git add docs/guide/where-the-metadata-lives.md
git commit -m "docs(guide): note that docket writes keep the metadata worktree current"
```

---

## Build-gate notes (for `docket-build`'s single full-suite gate)

- Run the whole suite through `build.test_command`. Expect possible fallout in end-to-end shards that drive the real binary (`tests/test_go_finalize_e2e.sh` and the app workflow and closeout shards that go through the CLI). After a CLI typed write the clone's checkout is no longer behind, so a later `prepare` may report `no-op` where it used to report an applied fast-forward. Such an assertion was pinning the old lag. Update it to the new current behavior, and name each one in the results file.
- Read the budget report. A `BUDGET WATCH:` line on `tests/test_go_integration_app_metadatafollow.sh` or `tests/test_go_integration_transaction_apply.sh` is a screening finding to re-measure quietly from the feature worktree before acting on it.
- The installed `docket` binary that runs this build's own metadata writes predates this change, so this repository's `.docket` will keep lagging during the build. That is expected and is not evidence against the change. The hermetic shared and private tests in Tasks 2–4 are the proof. Record this in the results file so a reviewer does not read the lag as a failure.
