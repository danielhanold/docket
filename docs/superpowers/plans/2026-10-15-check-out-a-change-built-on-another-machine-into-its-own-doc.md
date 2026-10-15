<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0551 — Check out a change built on another machine into its own docket worktree](../../changes/archive/2026-10-15-0551-check-out-a-change-built-on-another-machine-into-its-own-doc.md)**
<!-- docket:backlink:end -->

# Check out a change built on another machine into its own docket worktree - Implementation Plan

> **For agentic workers:** this plan is executed by `docket-build` task by task (each task routed to a build tier under the `docket-build-task` contract), with a single whole-suite gate at the end. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `docket workspace checkout --id <id>` puts an `implemented` change into its own docket-owned worktree at `.worktrees/<slug>` on any machine, adopting only state proven identical to (or an ancestor of) the head its open PR and its remote branch agree on. Finalize runs it first, and `finalize rebase` reports a missing or foreign workspace as `workspace-not-ready` with the real reason.

**Architecture:** Three gitcli primitives (worktree `locked`, CAS fast-forward of an unchecked-out branch, merge-base) feed `workspace.Service.Checkout`, which mirrors `Prepare` (operation lock, three-outcome probes, decisions before mutations, allocating->ready manifest) but adopts a pushed branch and removes a foreign worktree only after fresh removal proofs and git's clean-only remove. A thin app operation proves the PR half of the head as `probeRebasePR` does. `FinalizeRebase` reports an unregistered workspace before running Git in it; the finalize skill and guide document the command.

**Tech Stack:** Go, real-Git integration tests behind `//go:build integration`, shard runners under `tests/`, Cobra, embedded skill assets (`go generate ./internal/assets/`).

**Spec:** `docs/superpowers/specs/2026-10-15-check-out-a-change-built-on-another-machine-into-its-own-doc-design.md` (the feature branch's first commit). Read it with this plan.

## Global Constraints

- Never force: no `worktree remove --force`, `-B`, reset, or `branch -f`. A branch moves only by CAS fast-forward; a foreign worktree goes only through `RemoveWorktreeClean`.
- A probe error is an error, never absence. A `blocked`/`contended` checkout changes nothing but the per-workspace lock dir and fetched remote-tracking refs. All decisions precede the first mutation; removal proofs are the last reads before the removal, read from the worktree itself.
- Checkout acts only on `implemented` changes; it never repairs an owned workspace, never changes `workspace prepare`, never removes a worktree that does not hold the change's branch.
- Docs describe current behavior only (no change/PR numbers) in `docs/guide/`, `docs/concepts/`, `docs/reference/`. Source cross-references anchor on symbols, never line numbers.
- New `tests/test_*.sh`: `# docket-suite: go` in the first 10 lines plus a `tests/runtime-budgets.tsv` row. New `*_integration_test.go`: `//go:build integration` on line 1, blank line 2; each test name matches exactly one shard prefix in its package.
- Focused tests use `-count=1`; integration tests run as `go test -tags integration -count=1 -run '^<Name>' <pkg>` (never drop `-run` on `./internal/app/`).
- Mutation-test every guard: strip it, watch the named test go red, restore from a backup copy (`cp f f.bak` ... `mv -f f.bak f`), never `git checkout --`.
- After editing `skills/**`, `go generate ./internal/assets/` and commit the bundle with it. Conventional commits ending `(change 0551)`.
- Build gate: the whole suite via `build.test_command` (`go run ./cmd/docket development test`); read the budget report even when green.

## Spec reconciliations (read before Task 2)

1. **`finalize.rebase` gates `RebaseState` on `insp.Registered`, not on `ready`/`dirty-owned`.** A rebase detaches the worktree (`git worktree list --porcelain` reports `detached`, probed on git 2.56), so `Inspect` calls an owned worktree mid-rebase `mismatch`; the spec's literal gate would turn `TestIntegrationFinalizeRebaseGateForeignStateBlocked/foreign-rebase-in-progress` into `workspace-not-ready`. Registration meets both spec goals.
2. **Owned-workspace refusals do not halt finalize.** After a completed unpublished owned rebase the local head is the rewritten head (`local-head-mismatch`); mid-conflict the owned worktree is detached and dirty (`workspace-not-ready`/`workspace-dirty`); `finalize.rebase` recovers both from its receipt today. The skill continues past exactly those three reasons; every other `blocked` halts. The service emits them only for a manifest this repository owns (a foreign one gets `foreign-manifest`). A candidate whose PR already merged skips the checkout.
3. **The skill step is a bold-led paragraph closing step 1**, so steps 2-12 keep their numbers and `step N` references.
4. **Checkout's resume arm is its own**: `prepareResume` would create a missing branch at the recorded base (a merge-base here), not the PR head. Prepare's ancestry rule is kept: the recorded base must be contained in the head, else `workspace-not-ready`.
5. **Extra refusals**, nothing touched: `remote-feature-absent`, `foreign-manifest` (blocked), `local-branch-moved` (contended, lost CAS); app-level `revision-head-pair`, `malformed-head` (invalid-input) and `pr-not-open`, `pr-base-mismatch`, `pr-probe-failed`, `repository-unresolved`, mirroring `probeRebasePR`.
6. **Removal candidates**: a worktree attached to the feature ref anywhere but the primary checkout, plus a *detached* worktree at the canonical path when nothing holds the branch (it always fails proof 2 - the spec's "detached" test). A worktree at the canonical path on another branch is `target-path-occupied`. The spec's "rebase in progress" proof test uses `git bisect start` (HEAD stays attached and clean); a real foreign rebase is Review Focus 1.
7. **Proof 7** ("no docket manifest naming a different owner") scans every manifest under the workspaces root for one recording the candidate's path, excluding the change's own slot.

## Review Focus

1. A foreign worktree mid-rebase outside `.worktrees/` is detached, so it holds the branch only in git's eyes (`'feat' is already used by worktree at ...`, probed): expect an error, the rebase untouched, an allocating manifest left; after the human aborts it, a rerun removes the worktree and attaches. Task 2 `ForeignRebaseElsewhereRecovers`.
2. A recorded branch that is not `feat/<slug>` (downstream change 21's shape) lands at `.worktrees/<slug>`. Task 2 `Attaches/recorded-branch-differs`.
3. Two checkouts racing: one `attached`, one `existing`. Task 2 `ConcurrentRunsSerialize`.
4. An owned workspace mid-rebase: `workspace-not-ready`, rebase untouched, so the skill routes on to `finalize.rebase`. Task 2 `Refusals/owned-mid-rebase`.
5. `--head` uppercase or abbreviated: `malformed-head` before any Git. Task 4 `RefusesBeforeAnyChange/malformed-head`.

## Learnings applied

probe-error-is-not-clean-absence, decide-and-act-on-the-same-copy, cas-re-read-fresh-origin, duplicated-gate-copies-the-whole-predicate, assert-pins-outcome-not-mechanism, groomed-root-cause-is-a-hypothesis, printed-remedy-state-validity, mutation-restore-needs-a-backup-copy.

## ADRs to record

Recorded by the implement-next parent through `docket-adr`; not a build task.

1. **Docket may adopt or replace unowned local state for a change only when that state is proven identical to, or an ancestor of, the change's PR-and-remote-agreed head, and never removes anything that is not clean.** `workspace prepare` keeps refusing branches it did not create (a fresh build has nothing to prove them against); `workspace checkout` adopts an `implemented` change's pushed branch because the open PR and the fetched remote together prove its head, and every action is checked against it. This stops a later change from reverting checkout to prepare's rule or extending adoption to unproven states (e.g. `in-progress` changes).

---

### Task 1: gitcli primitives

Suggested tier: standard.

**Files:** Modify `internal/gitcli/worktree.go`, `internal/gitcli/history.go`, `internal/gitcli/worktree_integration_test.go`, `internal/gitcli/history_integration_test.go`. Create `internal/gitcli/branchref.go`, `internal/gitcli/worktree_parse_test.go`, `internal/gitcli/branchref_integration_test.go`.

**Interfaces (produces):** `WorktreeInfo.Locked bool`; `func (c *Client) FastForwardBranchRef(ctx context.Context, repo Repository, branch RefName, expectedTip, target ObjectID) error`; `func (c *Client) MergeBase(ctx context.Context, repo Repository, a, b ObjectID) (ObjectID, bool, error)` (found=false only on merge-base's clean "no common ancestor").

- [ ] **Step 1: Failing tests.** Create `internal/gitcli/worktree_parse_test.go` (default build, pure):

```go
package gitcli

import "testing"

func TestParseWorktreeListZLocked(t *testing.T) {
	h := "1111111111111111111111111111111111111111"
	stream := "worktree /p\x00HEAD " + h + "\x00branch refs/heads/main\x00\x00" +
		"worktree /a\x00HEAD " + h + "\x00branch refs/heads/a\x00locked\x00\x00" +
		"worktree /b\x00HEAD " + h + "\x00detached\x00locked on a usb disk\x00\x00"
	infos, err := parseWorktreeListZ([]byte(stream))
	if err != nil || len(infos) != 3 {
		t.Fatalf("parse = %d stanzas, %v", len(infos), err)
	}
	if infos[0].Locked || !infos[1].Locked || !infos[2].Locked || !infos[2].Detached {
		t.Errorf("locked flags = %v %v %v", infos[0].Locked, infos[1].Locked, infos[2].Locked)
	}
}
```

Append `TestIntegrationRepoListWorktreesReportsLocked` to `worktree_integration_test.go`: `AddBranchWorktree` a `feat/locked-x` worktree under `testsupport.TempDir(t)`; its `ListWorktrees` entry is unlocked, locked after `git worktree lock --reason x <path>`, unlocked after `git worktree unlock <path>`.

Create `branchref_integration_test.go` with `TestIntegrationBranchTipFastForwardBranchRef`: a helper builds three commits A<-B<-C on main of `newMainModeRepos` and `git branch feat/ff A`. Subtests: **moves** (FF A->C, tip == C); **moved tip** (`update-ref refs/heads/feat/ff B`, FF A->C errors, tip stays B); **backward** (branch at C, FF C->A returns `KindInvalidRepository`, tip stays C); **checked out** (`git worktree add <tmp> feat/ff`, FF A->C returns `KindInvalidRepository`, tip stays A).

Append to `history_integration_test.go` `TestIntegrationHistoryMergeBase`: **fork point** (`historyRepo`, root commit, `checkout -b side` + commit, back to main + commit: `MergeBase(side, main)` == root, found) and **disjoint** (orphan branch as in `TestIntegrationHistorySharedAncestryDisjoint`: found=false, no error, empty id).

Run: `go test -count=1 -run '^TestParseWorktreeListZLocked$' ./internal/gitcli/` - FAIL (no `Locked`).

- [ ] **Step 2: Implement.** In `worktree.go` add to `WorktreeInfo` after `Prunable`:

```go
	// Locked is Git's "locked" annotation (`git worktree lock`, with or without
	// a reason): git refuses to remove or prune it until it is unlocked.
	Locked bool
```

In `parseWorktreeListZ` add `case "locked": cur.Locked = true` before `default`, and in its doc comment list `locked` among the recognized attributes.

Create `branchref.go` with `const fastForwardBranchRefOp Operation = "fast-forward-branch-ref"` and `FastForwardBranchRef`, doc-commented as "moves a local branch no worktree has checked out from expectedTip to target by compare-and-swap, so a branch that moved since the caller read it is refused unchanged; FastForwardCheckedOutBranch moves a checked-out branch". Body, in order: `validateRefName(branch)` and `branchShortName` (invalid-request), `validateObjectID` on both ids (invalid-request); `IsAncestorIgnoringReplacements(expectedTip, target)` false -> `KindInvalidRepository` "target does not descend from the expected tip" (error returned as is); `ListWorktrees` (error returned) with any `!Detached && Branch == branch` entry -> `KindInvalidRepository` "branch is checked out in a worktree"; then `c.run` in `repo.PrimaryWorktree` with `update-ref -m "docket: fast-forward" <branch> <target> <expectedTip>`; non-zero exit -> `KindCommandFailed` with `stderrExcerpt(res.stderr)` and `withExitCode`.

In `history.go` add `mergeBaseOp Operation = "merge-base"` to the label block and, after `HasSharedAncestry`, `MergeBase`: validate both ids (invalid-request); run `merge-base a b` in `repo.PrimaryWorktree`; exit 1 with empty trimmed stdout -> `("", false, nil)`; other non-zero -> command-failed with `stderrExcerpt`; otherwise exactly one `stdoutLines` line that passes `validateObjectID` (else invalid-output) -> `(id, true, nil)`. Doc comment: "found is false only on merge-base's documented clean 'no merge base' answer; every other failure is a typed error, never a missing base."

- [ ] **Step 3: Pass.** `go test -count=1 -run '^TestParseWorktreeListZLocked$' ./internal/gitcli/ && go test -tags integration -count=1 -run '^(TestIntegrationRepo|TestIntegrationBranchTipFastForwardBranchRef|TestIntegrationHistoryMergeBase)' ./internal/gitcli/` - PASS.

- [ ] **Step 4: Mutate.** Drop `string(expectedTip)` from the update-ref args: the moved-tip subtest goes red. Delete the `ListWorktrees` loop: the checked-out subtest goes red. Restore each.

- [ ] **Step 5: Commit** `feat(gitcli): worktree locked flag, CAS branch fast-forward, merge-base read (change 0551)`.

---

### Task 2: `workspace.Service.Checkout`

Suggested tier: premium (it creates and moves a branch and deletes a folder docket did not make).

**Files:** Create `internal/workspace/checkout.go`, `internal/workspace/checkout_integration_test.go`, `tests/test_go_integration_workspace_checkout.sh`. Modify `internal/workspace/prepare.go`, `internal/workspace/result.go`, `tests/runtime-budgets.tsv`.

**Interfaces:**
- Consumes (Task 1): `FastForwardBranchRef`, `MergeBase`, `WorktreeInfo.Locked`; existing `WorktreeCheckoutState`, `ChangedPaths`, `RemoveWorktreeClean`, `AddBranchWorktree`, `AttachBranchWorktree`, `IsAncestor`.
- Produces (Task 4 uses): `CheckoutDisposition` values `CheckoutExisting|Attached|Blocked|Contended` = `existing|attached|blocked|contended`. `CheckoutReason<Name>` = token: HeadMismatch `head-mismatch`, RemoteFeatureAbsent `remote-feature-absent`, ForeignManifest `foreign-manifest`, WorkspaceDirty `workspace-dirty`, LocalHeadMismatch `local-head-mismatch`, WorkspaceNotReady `workspace-not-ready`, LocalBranchDiverged `local-branch-diverged`, BranchInPrimary `branch-in-primary-checkout`, ForeignWorktreeUnsafe `foreign-worktree-unsafe`, TargetPathOccupied `target-path-occupied`, LocalBranchMoved `local-branch-moved`. `Proof<Name>` = token: SeveralHolders `several-holders`, MetadataWorktree `metadata-worktree`, PathMissing `path-missing`, Locked `locked`, ManifestClaims `manifest-claims-path`, NotAttached `not-attached`, Operation `operation-in-progress`, HeadMismatch `head-mismatch`, Dirty `dirty`, GitRefused `git-refused`. `CheckoutRequest{Repository; Remote; Target; ExpectedHead gitcli.ObjectID; MetadataWorktree string}`; `CheckoutResult{Disposition; Reason, Message, FailedProof, Path string; FeatureRef, BaseRef gitcli.RefName; BaseCommit, HeadCommit gitcli.ObjectID; RemovedWorktree string; FastForwardedFrom gitcli.ObjectID}`; `func (s *Service) Checkout(ctx, req CheckoutRequest) (CheckoutResult, error)`.

- [ ] **Step 1: Refactor (no behavior change).** In `prepare.go` give `publishAllocating` and `reinspectAndFinalize` a leading `op string` parameter (`publishAllocating(op, root, dir string, m Manifest)`; `reinspectAndFinalize(ctx, op, repo, dir, man, intendedPath, target, baseCommit, disposition)`), use `op` for every `Failure.Op`/`mapGitFailure` inside them, pass `prepareOp` at the three existing calls. `result.go`'s `Failure.Op` comment gains `| "checkout"`. `go test -tags integration -count=1 -run '^TestIntegrationWorkspaceSetup' ./internal/workspace/` - PASS.

- [ ] **Step 2: Failing tests.** Create `internal/workspace/checkout_integration_test.go`, reusing the Prepare harness (`mainModeRepo`, `docketModeRepo`, `pushBranch`, `newService`, `freshTarget`, `wsPathOf`, `metaDirOf`, `writeStateManifest`, `snapshotAll`/`assertAllUnchanged`, `snapshotTree`/`assertUnchanged`, `writeFailingGit`, `symbolicHead`, `gitTry`, `readFileBytes`, `loadManifest`, `writeManifest`):

```go
//go:build integration

package workspace

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/gitcli"
)

const coFeature = "feat/" + prepSlug

func (r *wsRepos) writerCommit(t *testing.T, branch, file string) gitcli.ObjectID {
	t.Helper()
	gitOut(t, r.Writer, "checkout", "-q", branch)
	writeWorktreeFile(t, r.Writer, file, "writer "+file+"\n")
	gitOut(t, r.Writer, "add", "-A")
	gitOut(t, r.Writer, "commit", "-q", "-m", file)
	gitOut(t, r.Writer, "push", "-q", "origin", branch)
	h := gitcli.ObjectID(gitOut(t, r.Writer, "rev-parse", "HEAD"))
	gitOut(t, r.Writer, "checkout", "-q", "main")
	return h
}
func (r *wsRepos) mainTip(t *testing.T) gitcli.ObjectID {
	return gitcli.ObjectID(gitOut(t, r.Writer, "rev-parse", "main"))
}
func (r *wsRepos) localFeatureAt(t *testing.T, at gitcli.ObjectID) {
	gitOut(t, r.Primary, "fetch", "-q", "origin", coFeature)
	gitOut(t, r.Primary, "branch", coFeature, string(at))
}
func commitIn(t *testing.T, dir, file string) gitcli.ObjectID {
	writeWorktreeFile(t, dir, file, file+"\n")
	gitOut(t, dir, "add", "-A")
	gitOut(t, dir, "commit", "-q", "-m", file)
	return gitcli.ObjectID(gitOut(t, dir, "rev-parse", "HEAD"))
}
func (r *wsRepos) localCommit(t *testing.T) {
	tmp := filepath.Join(filepath.Dir(r.Primary), "tmp-wt")
	gitOut(t, r.Primary, "worktree", "add", "-q", tmp, coFeature)
	commitIn(t, tmp, "local-only.txt")
	gitOut(t, r.Primary, "worktree", "remove", tmp)
}
func (r *wsRepos) foreignAt(t *testing.T, path string) string {
	gitOut(t, r.Primary, "worktree", "add", "-q", path, coFeature)
	cp, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return cp
}
func (r *wsRepos) elsewhere() string { return filepath.Join(filepath.Dir(r.Primary), "foreign") }
func (r *wsRepos) conflictedRebase(t *testing.T, dir string) {
	gitOut(t, r.Writer, "checkout", "-q", "main")
	writeWorktreeFile(t, r.Writer, "on-feat-"+prepSlug+".txt", "conflicting base edit\n") // pushBranch's feature file
	gitOut(t, r.Writer, "add", "-A")
	gitOut(t, r.Writer, "commit", "-q", "-m", "conflict")
	gitOut(t, r.Writer, "push", "-q", "origin", "main")
	gitOut(t, dir, "fetch", "-q", "origin", "main")
	if _, err := gitTry(dir, "rebase", "origin/main"); err == nil {
		t.Fatal("fixture: the rebase did not stop at a conflict")
	}
}
func targetFor(t *testing.T, branch string) Target {
	tgt, err := NewTarget(7, prepSlug, freshTarget(t, 7).Base, branch)
	if err != nil {
		t.Fatal(err)
	}
	return tgt
}
func coReq(repo gitcli.Repository, tgt Target, head gitcli.ObjectID, meta string) CheckoutRequest {
	return CheckoutRequest{Repository: repo, Remote: "origin", Target: tgt, ExpectedHead: head, MetadataWorktree: meta}
}
func runCheckout(t *testing.T, svc *Service, repo gitcli.Repository, tgt Target, head gitcli.ObjectID, meta string) CheckoutResult {
	t.Helper()
	res, err := svc.Checkout(context.Background(), coReq(repo, tgt, head, meta))
	if err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	return res
}
func assertAttached(t *testing.T, svc *Service, repo gitcli.Repository, tgt Target, res CheckoutResult, want gitcli.ObjectID) {
	t.Helper()
	ws := wsPathOf(repo)
	if res.Disposition != CheckoutAttached || res.Path != ws || res.HeadCommit != want {
		t.Fatalf("= %q %s@%s (%s: %s); want attached %s@%s", res.Disposition, res.Path, res.HeadCommit, res.Reason, res.Message, ws, want)
	}
	if got := symbolicHead(t, ws); got != string(tgt.FeatureRef) {
		t.Errorf("HEAD = %q; want %q", got, tgt.FeatureRef)
	}
	m, present, err := loadManifest(metaDirOf(repo, tgt))
	if err != nil || !present || m.Phase != PhaseReady {
		t.Fatalf("manifest %v %q %v; want ready", present, m.Phase, err)
	}
	if _, err := gitTry(repo.PrimaryWorktree, "merge-base", "--is-ancestor", string(m.BaseCommit), string(want)); err != nil {
		t.Errorf("base %s not contained in %s", m.BaseCommit, want)
	}
	if insp, err := svc.Inspect(context.Background(), InspectRequest{Repository: repo, Target: tgt}); err != nil || insp.Kind != StateReady {
		t.Errorf("Inspect = %q, %v", insp.Kind, err)
	}
}
func adopt(t *testing.T, r *wsRepos, repo gitcli.Repository, tgt Target, head gitcli.ObjectID) string {
	svc, _ := r.newService(t)
	assertAttached(t, svc, repo, tgt, runCheckout(t, svc, repo, tgt, head, ""), head)
	return wsPathOf(repo)
}
func foreignWith(mutate func(t *testing.T, r *wsRepos, p string)) func(*testing.T, *wsRepos, gitcli.Repository, Target, gitcli.ObjectID) (gitcli.ObjectID, string) {
	return func(t *testing.T, r *wsRepos, _ gitcli.Repository, _ Target, head gitcli.ObjectID) (gitcli.ObjectID, string) {
		r.localFeatureAt(t, head)
		p := r.foreignAt(t, r.elsewhere())
		mutate(t, r, p)
		return head, p
	}
}

type attachRow struct {
	name, branch string
	docket       bool
	arrange func(t *testing.T, r *wsRepos, repo gitcli.Repository, tgt Target, head gitcli.ObjectID) (gitcli.ObjectID, string, gitcli.ObjectID)
}

func TestIntegrationWorkspaceCheckoutAttaches(t *testing.T) {
	for _, tc := range attachRows {
		t.Run(tc.name, func(t *testing.T) {
			r := mainModeRepo(t)
			if tc.docket {
				r = docketModeRepo(t)
			}
			branch := coFeature
			if tc.branch != "" {
				branch = tc.branch
			}
			head := r.pushBranch(t, branch, "main")
			svc, repo := r.newService(t)
			tgt := targetFor(t, branch)
			want, removed, ff := tc.arrange(t, r, repo, tgt, head)
			before := r.snapshotAll(t)
			res := runCheckout(t, svc, repo, tgt, want, "")
			assertAttached(t, svc, repo, tgt, res, want)
			if res.RemovedWorktree != removed || res.FastForwardedFrom != ff {
				t.Errorf("removed=%q ff=%q; want %q %q", res.RemovedWorktree, res.FastForwardedFrom, removed, ff)
			}
			if removed != "" && removed != wsPathOf(repo) {
				if _, err := os.Lstat(removed); !os.IsNotExist(err) {
					t.Errorf("foreign folder survived (%v)", err)
				}
			}
			r.assertAllUnchanged(t, before)
		})
	}
}

type refuseRow struct {
	name, branch, disp, reason, proof, mention string
	owned, meta                                bool
	arrange func(t *testing.T, r *wsRepos, repo gitcli.Repository, tgt Target, head gitcli.ObjectID) (gitcli.ObjectID, string)
}

// Every refusal leaves the local branch, the kept worktree, and the primary
// byte-identical and never advances a manifest to ready.
func TestIntegrationWorkspaceCheckoutRefusals(t *testing.T) {
	for _, tc := range refuseRows {
		t.Run(tc.name, func(t *testing.T) {
			r := mainModeRepo(t)
			head := r.pushBranch(t, coFeature, "main")
			svc, repo := r.newService(t)
			branch := coFeature
			if tc.branch != "" {
				branch = tc.branch
			}
			tgt := targetFor(t, branch)
			expected, keep := tc.arrange(t, r, repo, tgt, head)
			var snap map[string]string
			if keep != "" {
				snap = snapshotTree(t, keep)
			}
			tip, _ := gitTry(r.Primary, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
			before := r.snapshotAll(t)
			meta := ""
			if tc.meta {
				meta = keep
			}
			res := runCheckout(t, svc, repo, tgt, expected, meta)
			if string(res.Disposition) != tc.disp || res.Reason != tc.reason || res.FailedProof != tc.proof {
				t.Fatalf("= %q/%q proof %q (%s); want %s/%s proof %q", res.Disposition, res.Reason, res.FailedProof, res.Message, tc.disp, tc.reason, tc.proof)
			}
			if !strings.Contains(res.Message, tc.mention) {
				t.Errorf("message %q lacks %q", res.Message, tc.mention)
			}
			if keep != "" {
				assertUnchanged(t, snap, keep)
			}
			if after, _ := gitTry(r.Primary, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch); after != tip {
				t.Errorf("local branch moved %q -> %q", tip, after)
			}
			if m, ok, _ := loadManifest(metaDirOf(repo, tgt)); ok && m.Phase == PhaseReady && !tc.owned {
				t.Error("a refused checkout adopted the workspace")
			}
			r.assertAllUnchanged(t, before)
		})
	}
}
```

Define package-level `attachRows`/`refuseRows` literals, one entry per row below (`owned`: a ready manifest predates the call; `meta`: pass `keep` as `MetadataWorktree`; `keep`: a worktree that must stay byte-identical).

| attachRows `name` | `branch` / `docket` | arrange (returns want, removed, ff) |
|---|---|---|
| `remote-only` | - | `head, "", ""` |
| `remote-only-docket-topology` | `docket: true` | `head, "", ""` |
| `recorded-branch-differs` (Review Focus 2) | `feature/renamed-head` | `head, "", ""` |
| `local-at-head` | - | `localFeatureAt(head)`; `head, "", ""` |
| `local-behind` | - | `localFeatureAt(head)`; `writerCommit(coFeature, "two.txt"), "", head` |
| `resume-nothing-created` | - | `writeStateManifest(repo, tgt, mainTip, PhaseAllocating)`; `head, "", ""` |
| `resume-branch-created` | - | manifest as above; `localFeatureAt(head)`; `head, "", ""` |
| `resume-worktree-attached` | - | manifest; `localFeatureAt(head)`; `git worktree add -q <wsPathOf(repo)> coFeature`; `head, "", ""` |
| `foreign-elsewhere` | - | `localFeatureAt(head)`; `p := foreignAt(elsewhere())`; write ignored `p/.docket/build-output.txt`; `head, p, ""` |
| `foreign-at-target` | - | `localFeatureAt(head)`; `head, foreignAt(wsPathOf(repo)), ""` |

| refuseRows `name` | reason (disp `blocked` unless noted; "unsafe / P" = ForeignWorktreeUnsafe with proof P) | arrange (returns expected, keep) | `mention` / flags |
|---|---|---|---|
| `local-ahead` | LocalBranchDiverged | `localFeatureAt(head)`; `localCommit`; `head, ""` | `"git branch -D " + coFeature` |
| `local-diverged` | LocalBranchDiverged | `localFeatureAt(head)`; `localCommit`; `writerCommit(coFeature, "r.txt"), ""` | |
| `branch-in-primary` | BranchInPrimary | `localFeatureAt(head)`; `git -C primary checkout -q coFeature`; `head, ""` | `"switch"` |
| `head-mismatch` | HeadMismatch (contended) | `mainTip, ""` | |
| `remote-absent` | RemoteFeatureAbsent | `head, ""` | `branch: "feat/never-pushed"` |
| `target-folder` | TargetPathOccupied | write `wsPathOf(repo)/notes.txt`; `head, ""` | |
| `target-other-branch` | TargetPathOccupied | `git worktree add -q -b feat/else <wsPathOf> main`; `head, wsPathOf` | |
| `foreign-manifest` | ForeignManifest | write `{not json` at `metaDirOf(repo,tgt)/manifestFileName`; `head, ""` | |
| `owned-dirty` | WorkspaceDirty | `ws := adopt(...)`; write `ws/scratch.txt`; `head, ws` | `owned` |
| `owned-ahead` | LocalHeadMismatch | `ws := adopt`; `commitIn(ws, "more.txt")`; `head, ws` | `owned` |
| `owned-mid-rebase` (Review Focus 4) | WorkspaceNotReady | `ws := adopt`; `conflictedRebase(ws)`; `head, ws` | `owned` |
| `owned-cleaned` | WorkspaceNotReady | `writeStateManifest(..., mainTip, PhaseCleaned)`; `head, ""` | `owned` |
| `resume-base-not-contained` | WorkspaceNotReady | `moved := writerCommit("main", "moved.txt")`; `git -C primary fetch -q origin main`; `writeStateManifest(..., moved, PhaseAllocating)`; `head, ""` | `owned` |
| `proof-dirty-tracked` | unsafe / ProofDirty | `foreignWith`: overwrite `p/main.go` | |
| `proof-staged` | unsafe / ProofDirty | `foreignWith`: write `p/staged.txt`, `git -C p add staged.txt` | |
| `proof-untracked` | unsafe / ProofDirty | `foreignWith`: write `p/new.txt` | |
| `proof-operation` | unsafe / ProofOperation | `foreignWith`: `git -C p bisect start` | |
| `proof-head` | unsafe / ProofHeadMismatch | `foreignWith`: `commitIn(p, "unpushed.txt")` | |
| `proof-locked` | unsafe / ProofLocked | `foreignWith`: `git -C primary worktree lock p` | `"git worktree unlock"` |
| `proof-metadata` | unsafe / ProofMetadataWorktree | `foreignWith`: no-op | `meta` |
| `proof-manifest-claims` | unsafe / ProofManifestClaims | `foreignWith` no-op, then `writeManifest(metaDirOf(repo, other), Manifest{...valid, Path: p, Phase: PhaseReady})` for `other, _ := NewTarget(8, "other-change", tgt.Base, "feat/other-change")` (schema, id, common dir, ids, refs, base `mainTip`, RFC3339 times); `head, p` | |
| `proof-path-missing` | unsafe / ProofPathMissing | `foreignWith` no-op, then `os.RemoveAll(p)`; `head, ""` | `"git worktree prune"` |
| `proof-not-attached` | unsafe / ProofNotAttached | `git -C primary fetch -q origin coFeature`; `git worktree add -q --detach <wsPathOf> head`; `head, wsPathOf` | |

Then append `TestIntegrationWorkspaceCheckoutExistingIsNoOp` (push, `adopt`, snapshot the workspace and read the manifest bytes; a second `runCheckout` returns `existing` at the head; snapshot and manifest bytes unchanged) and `TestIntegrationWorkspaceCheckoutConcurrentRunsSerialize` (Review Focus 3: two services from two `r.newService` calls, two goroutines released together by a closed channel, each sending its disposition or `"error: ..."`; collect both with a 2-minute timeout; sorted they must be `attached`, `existing`), plus:

```go
// A failing probe is a *Failure from the expected stage; nothing is adopted or removed.
func TestIntegrationWorkspaceCheckoutProbeErrorsTouchNothing(t *testing.T) {
	for _, tc := range []struct {
		sub     string
		foreign bool
		stage   Stage
	}{{"fetch", false, "fetch"}, {"merge-base", false, "inventory"}, {"status", true, "probe"}, {"symbolic-ref", true, "probe"}} {
		t.Run(tc.sub, func(t *testing.T) {
			r := mainModeRepo(t)
			head := r.pushBranch(t, coFeature, "main")
			_, repo := r.newService(t) // real discovery: discovery itself runs `git worktree list`
			tgt := freshTarget(t, 7)
			var p string
			var snap map[string]string
			if tc.foreign {
				r.localFeatureAt(t, head)
				p = r.foreignAt(t, r.elsewhere())
				snap = snapshotTree(t, p)
			}
			c, err := gitcli.NewClient(gitcli.WithExecutable(writeFailingGit(t, tc.sub)))
			if err != nil {
				t.Fatal(err)
			}
			svc, _ := NewService(c)
			_, err = svc.Checkout(context.Background(), coReq(repo, tgt, head, ""))
			if f, ok := AsFailure(err); !ok || f.Stage != tc.stage {
				t.Fatalf("failing %s = %v; want a *Failure at stage %s", tc.sub, err, tc.stage)
			}
			if tc.foreign {
				assertUnchanged(t, snap, p)
			}
			if _, ok, _ := loadManifest(metaDirOf(repo, tgt)); ok {
				t.Error("a manifest was published")
			}
		})
	}
}

// Review Focus 1.
func TestIntegrationWorkspaceCheckoutForeignRebaseElsewhereRecovers(t *testing.T) {
	r := mainModeRepo(t)
	head := r.pushBranch(t, coFeature, "main")
	r.localFeatureAt(t, head)
	foreign := r.foreignAt(t, r.elsewhere())
	r.conflictedRebase(t, foreign)
	svc, repo := r.newService(t)
	tgt := freshTarget(t, 7)
	if _, err := svc.Checkout(context.Background(), coReq(repo, tgt, head, "")); err == nil {
		t.Fatal("Checkout over a foreign rebase = nil error; want git's refusal")
	}
	rd := gitOut(t, foreign, "rev-parse", "--git-path", "rebase-merge")
	if !filepath.IsAbs(rd) {
		rd = filepath.Join(foreign, rd)
	}
	if _, err := os.Stat(rd); err != nil {
		t.Fatalf("the foreign rebase was disturbed: %v", err)
	}
	if m, ok, err := loadManifest(metaDirOf(repo, tgt)); err != nil || !ok || m.Phase != PhaseAllocating {
		t.Fatalf("manifest %v %q %v; want an allocating breadcrumb", ok, m.Phase, err)
	}
	gitOut(t, foreign, "rebase", "--abort")
	res := runCheckout(t, svc, repo, tgt, head, "")
	assertAttached(t, svc, repo, tgt, res, head)
	if res.RemovedWorktree != foreign {
		t.Errorf("RemovedWorktree = %q; want %q", res.RemovedWorktree, foreign)
	}
}
```

If `gitcli.Discover` runs `status` or `symbolic-ref` itself, the `probe` rows fail at `validate`: pick a subcommand only the proofs run rather than loosen the stage assert.

Shard: copy `tests/test_go_integration_workspace_setup.sh` to `tests/test_go_integration_workspace_checkout.sh` (0755) with `SHARD_PREFIX="TestIntegrationWorkspaceCheckout"` and a header naming the workspace Checkout real-git tests; add `tests/test_go_integration_workspace_checkout.sh	40	parallel` after the `workspace_lifecycle` row of `tests/runtime-budgets.tsv`.

`go test -tags integration -count=1 -run '^TestIntegrationWorkspaceCheckout' ./internal/workspace/` - FAIL to compile.

- [ ] **Step 3: Implement `internal/workspace/checkout.go`.** Header comment: adopts an implemented change's pushed branch into `<primary>/.worktrees/<slug>` at the head the caller-proved PR head and the fetched remote agree on; unlike Prepare it adopts a branch at or behind that head, and removes a foreign worktree only when every `removalProof` holds, via git's clean-only remove; all else is a blocked value; probe errors are errors; decisions precede mutations; allocating->ready manifest as in Prepare. `const checkoutOp = "checkout"`, the **Interfaces** names, and `func (r CheckoutResult) refuse(d CheckoutDisposition, reason, msg string) CheckoutResult`.

`Checkout`:
1. `validateIdentity`, `validatePreparedTarget`; empty `Remote` or `!validObjectID(ExpectedHead)` -> Failure stage `validate`, invalid-input.
2. `dir := workspaceDir(CommonDir, FeatureRef)`; `intended := <PrimaryWorktree>/.worktrees/<Slug>` (also `out.Path`). Hold `acquireOperationLock(dir)` to return.
3. `FetchBranch(remote, FeatureRef)`: `KindRefUnavailable` -> blocked `remote-feature-absent`; other error -> `mapGitFailure(checkoutOp, "fetch", err)`; commit != head -> contended `head-mismatch` naming both heads.
4. `classifyManifest(dir)`: unknown -> Failure `inventory`; foreign, or valid but `!ownsManifest(m, repo, target, intended)` -> blocked `foreign-manifest`; owned ready -> existing arm; owned allocating -> attach arm, `resume = &m`; owned other -> blocked `workspace-not-ready`; absent -> attach arm, `resume = nil`.

Existing arm (read-only `Inspect`): ready at the head -> `existing` (Base/Head from the inspection); ready elsewhere -> `local-head-mismatch`; `dirty-owned` -> `workspace-dirty`; else `workspace-not-ready`. Messages name the path and say it was left as is.

Attach arm (all probes before the first mutation):
1. `ListWorktrees`. Holders: `!Detached && Branch == FeatureRef`. `atTarget`: the registration whose `canonicalOrLexical(Path) == intended`.
2. >1 holder -> `foreign-worktree-unsafe`, `several-holders`. A holder at `PrimaryWorktree` -> `branch-in-primary-checkout` (remedy `git -C <primary> switch <base branch>`). A holder at `intended` while resuming -> plan *registered*. Any other holder -> foreign candidate.
3. `atTarget` that is not that holder: a detached one with no holder -> foreign candidate; else `target-path-occupied` (remedy `git worktree remove <path>` or `git worktree prune`). No registration but `pathPresent(intended)` -> `target-path-occupied` ("not a worktree; move it aside"); stat error -> Failure.
4. `ResolveRef(FeatureRef)` (`KindRefUnavailable` = absent; other -> Failure `inventory`). Without a foreign candidate: *registered* needs tip == head else `local-head-mismatch`; absent -> *create*; tip == head -> *attach*; `IsAncestor(tip, head)` -> *fast-forward*; else `local-branch-diverged` naming tip and head with `git branch <short>-local <tip>` and `git branch -D <short>`. (A candidate's head is judged by proof 3.)
5. Base: resuming, `IsAncestor(resume.BaseCommit, head)` else `workspace-not-ready`; fresh, `FetchBranch(remote, BaseRef)` then `MergeBase(head, baseTip)` (not found -> Failure invalid-state). Set `out.BaseCommit`.
6. Foreign candidate: `removalProof`; failing -> `foreign-worktree-unsafe` with `FailedProof` and the remedy as message. Then `RemoveWorktreeClean(foreign.Path)`: `KindCommandFailed` -> `foreign-worktree-unsafe`, `git-refused`, git's detail; other -> Failure `remove`. Success: `RemovedWorktree` (canonical, computed first), plan *attach*.
7. Unless resuming, `publishAllocating(checkoutOp, workspacesRoot(CommonDir), dir, Manifest{...allocating, BaseCommit: base, Path: intended})` as Prepare builds it (resume reuses `*resume`). *create*: `AddBranchWorktree(intended, FeatureRef, head)`. *fast-forward*: `FastForwardBranchRef(FeatureRef, tip, head)` - on error re-read the ref: moved -> contended `local-branch-moved`, else Failure `allocate` - then `FastForwardedFrom = tip` and `AttachBranchWorktree`. *attach*: `AttachBranchWorktree`. *registered*: nothing. Worktree errors -> Failure `worktree`.
8. `reinspectAndFinalize(ctx, checkoutOp, ..., base, PrepareCreated)`; head != expected -> Failure `verify` invalid-state; else `attached`.

`removalProof(ctx, req, info gitcli.WorktreeInfo) (proof, remedy string, err error)` - first failure wins, each remedy valid in that exact state, any probe error returned (stage `probe`):

| # | Check | Token | Remedy |
|---|---|---|---|
| 1 | path == `canonicalOrLexical(MetadataWorktree)` (when set) | `metadata-worktree` | move it back to the metadata branch by hand |
| 2 | `info.Prunable` or `!pathPresent` | `path-missing` | `git worktree prune` |
| 3 | `info.Locked` | `locked` | if nothing needs it, `git worktree unlock <path>` |
| 4 | `manifestClaimsPath(CommonDir, workspaceID(FeatureRef), path)` | `manifest-claims-path` | another change's manifest records it; nothing changed |
| 5 | `WorktreeCheckoutState`: detached or `Branch != FeatureRef` | `not-attached` | if it holds nothing you need, `git worktree remove <path>` |
| 6 | ... `OperationInProgress` | `operation-in-progress` | finish or abort it there |
| 7 | ... `Head != ExpectedHead` | `head-mismatch` | push or move its commits, then `git worktree remove <path>` |
| 8 | `ChangedPaths` non-empty (ignored files are not work) | `dirty` | commit, stash, or discard them |

Every remedy message names the canonical path.

`manifestClaimsPath(commonDir, ownID, path string) (bool, error)`: `os.ReadDir(workspacesRoot(commonDir))` (not-exist -> false; other error -> Failure stage `probe`); for each subdirectory other than `ownID`, `classifyManifest`: unknown -> Failure stage `probe`; valid with `canonicalOrLexical(m.Path) == path` -> true; foreign (undecodable) names no path and is skipped.

`canonicalOrLexical(p string) string`: `canonicalizePath(p)` when it resolves; otherwise `filepath.Abs`, then `EvalSymlinks` of the parent joined with the base name (so a stale registration whose folder is gone still equals a canonical target path); last resort the cleaned absolute path.

- [ ] **Step 4: Pass.** `go test -tags integration -count=1 -run '^TestIntegrationWorkspace' ./internal/workspace/` - PASS (Setup/Lifecycle prove the refactor). If `Refusals/owned-mid-rebase` reports `workspace-dirty`, read `Inspect` first: either owned reason routes the same in the skill; change the expectation only if `Inspect` genuinely says `dirty-owned`, and say so in the commit.

- [ ] **Step 5: Mutate.** Back up `checkout.go`; for each guard make it `false`, run `go test -tags integration -count=1 -run '^TestIntegrationWorkspaceCheckout<Test>$/^<row>$' ./internal/workspace/`, confirm red, restore with `mv -f`; a green row is a defect (the per-proof `FailedProof` assert stops git's own `git-refused` from satisfying a proof row). Guards -> rows: remote head == expected -> `Refusals/head-mismatch`; divergence -> `local-ahead`, `local-diverged`; primary holder -> `branch-in-primary`; resume base contained -> `resume-base-not-contained`; create at the head (swap in the base) -> `Attaches/resume-nothing-created`; proofs 1-8 -> `proof-metadata`, `proof-path-missing`, `proof-locked`, `proof-manifest-claims`, `proof-not-attached`, `proof-operation`, `proof-head`, `proof-dirty-tracked`/`staged`/`untracked`.

- [ ] **Step 6: Commit** `feat(workspace): check out an implemented change's pushed branch into its own worktree (change 0551)`.

---

### Task 3: `finalize rebase` reports an unregistered workspace as `workspace-not-ready`

Suggested tier: standard.

**Files:** Modify `internal/app/finalize_rebase.go`, `internal/app/finalize_rebase_test.go`, `internal/app/finalize_rebase_integration_test.go`.

**Interfaces (produces, used by Tasks 4-5):** `func (f *rebaseFixture) forgetLocalWorkspace(t *testing.T)`.

- [ ] **Step 1: Fixture helper.** In `finalize_rebase_test.go` (add `"os"`), after `localHead`:

```go
// forgetLocalWorkspace makes the clone look like a machine that never built the
// change: owned worktree, local branch, and manifest dir gone; origin keeps the branch.
func (f *rebaseFixture) forgetLocalWorkspace(t *testing.T) {
	t.Helper()
	runGit(t, f.repo.invocation, "worktree", "remove", "--force", f.wp)
	runGit(t, f.repo.invocation, "branch", "-D", f.target.FeatureBranch())
	if err := os.RemoveAll(f.metaDir); err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Failing test.** Append to `finalize_rebase_integration_test.go` (add `"path/filepath"`):

```go
func TestIntegrationFinalizeRebaseGateWorkspaceNotReady(t *testing.T) {
	requireRealGit(t)
	for _, tc := range []struct {
		name, detail string
		foreign      bool
	}{{"absent", `"absent"`, false}, {"foreign", "no workspace manifest; local feature branch exists", true}} {
		t.Run(tc.name, func(t *testing.T) {
			f := setupRebaseFixture(t, planRepoModes()[0])
			f.forgetLocalWorkspace(t)
			if tc.foreign {
				b := f.target.FeatureBranch()
				runGit(t, f.repo.invocation, "fetch", "-q", "origin", "refs/heads/"+b+":refs/heads/"+b)
				runGit(t, f.repo.invocation, "worktree", "add", "-q", filepath.Join(f.repo.root, "gh-checkout"), b)
			}
			gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
			res := FinalizeRebase(context.Background(), f.finalizeDeps(gh, &fakeGate{}), f.repo.invocation,
				FinalizeRebaseRequest{ID: f.id, Revision: f.revision, Head: f.head})
			assertRebaseRefused(t, res, ResultBlocked, ReasonRebaseWorkspaceNotReady)
			for _, want := range []string{tc.detail, fmt.Sprintf("docket workspace checkout --id %d", f.id)} {
				if !strings.Contains(res.Message, want) {
					t.Errorf("message %q lacks %q", res.Message, want)
				}
			}
			f.receiptAbsent(t)
		})
	}
}
```

Run it: FAIL with `workspace-probe-failed`.

- [ ] **Step 3: Implement.** In `FinalizeRebase`, replace the fresh-attempt comment and the line before `state, err := deps.Planning.Client.RebaseState(ctx, rc.wsDir)` so it reads:

```go
	// Fresh attempt (no owned receipt). A workspace not registered at its
	// canonical path (absent, foreign, a cleaned tombstone) has no checkout Git
	// can probe: report it not ready, with the inspection's reason and remedy,
	// before any Git runs in a directory that may not exist. A registered one
	// then has a foreign in-progress rebase detected FIRST (a rebase detaches the
	// worktree, so Inspect alone calls it a mismatch), never demoted to a
	// dirty-tree or not-ready refusal. Both precede the receipt.
	if !rc.insp.Registered {
		if r := requireReadyWorkspace(op, rc, req.Head); r != nil {
			return *r
		}
	}
```

In `requireReadyWorkspace`'s `default:` arm use `workspaceNotReadyMessage(rc)` as the message, and add:

```go
// workspaceNotReadyMessage carries the inspection's own reason and, when no
// docket workspace holds the change here, the command that checks it out.
func workspaceNotReadyMessage(rc *rebaseContext) string {
	msg := fmt.Sprintf("the feature workspace is %q, not the clean registered feature state", rc.insp.Kind)
	if rc.insp.Detail != "" {
		msg += " (" + rc.insp.Detail + ")"
	}
	if rc.insp.Kind == workspace.StateAbsent || rc.insp.Kind == workspace.StateForeign {
		msg += fmt.Sprintf("; check the change out into its own worktree with `docket workspace checkout --id %d`", int(rc.change.ID()))
	}
	return msg
}
```

- [ ] **Step 4: Pass.** `go test -tags integration -count=1 -run '^TestIntegrationFinalizeRebaseGate(WorkspaceNotReady|ForeignStateBlocked)$' ./internal/app/` - PASS.

- [ ] **Step 5: Mutate.** Delete the `if !rc.insp.Registered` block: `WorkspaceNotReady` goes red. Replace its condition with `rc.insp.Kind != workspace.StateReady && rc.insp.Kind != workspace.StateDirty` (the spec's literal gate): `ForeignStateBlocked/foreign-rebase-in-progress` goes red. Restore each.

- [ ] **Step 6: Commit** `fix(app): finalize rebase reports an unregistered workspace as workspace-not-ready with the checkout remedy (change 0551)`.

---

### Task 4: The `workspace.checkout` operation and `docket workspace checkout`

Suggested tier: standard.

**Files:** Create `internal/app/workspace_checkout.go`, `internal/app/workspace_checkout_integration_test.go`, `tests/test_go_integration_app_checkout.sh`. Modify `internal/app/workspace_ops.go`, `internal/app/schema_registry.go`, `internal/cli/workspace.go`, `internal/cli/install.go`, `internal/cli/workspace_test.go`, `internal/cli/capability_production_test.go`, `docs/reference/cli.md`, `tests/runtime-budgets.tsv`.

**Interfaces (produces):** `OperationWorkspaceCheckout = "workspace.checkout"`; `WorkspaceCheckoutRequest{ID int; Revision, Head string}` (json `id`, `revision,omitempty`, `head,omitempty`); `WorkspacePRReader` (`DiscoverRepository`, `FindOpenPullRequestsByHead`, signatures as in `FinalizeGitHub`); `WorkspaceCheckoutService{Checkout}`; `WorkspaceCheckoutDeps{Service; GitHub}`; `func WorkspaceCheckout(ctx, deps PlanningDeps, cdeps WorkspaceCheckoutDeps, repoDir string, req WorkspaceCheckoutRequest) WorkspaceOpResult`; `ReasonWorkspace<Name>`: CheckoutPinIncomplete=`revision-head-pair`, MalformedHead=`malformed-head`, NotImplemented=`not-implemented`, RevisionDrift=`revision-drift`, CheckoutRepoUnresolved=`repository-unresolved`, CheckoutPRProbe=`pr-probe-failed`, CheckoutPRNotOpen=`pr-not-open`, CheckoutPRBaseMismatch=`pr-base-mismatch`; `FindingForeignWorktreeRemoved`, `FindingLocalBranchFastForwarded`.

- [ ] **Step 1: Failing tests.** Create `internal/app/workspace_checkout_integration_test.go` (imports: context, errors, path/filepath, strings, testing, githubcli, workspace):

```go
//go:build integration

package app

func (f *rebaseFixture) checkoutDeps(gh WorkspacePRReader) WorkspaceCheckoutDeps {
	return WorkspaceCheckoutDeps{Service: f.svc, GitHub: gh}
}

func TestIntegrationWorkspaceCheckoutAttachesBuiltElsewhere(t *testing.T) {
	requireRealGit(t)
	ctx := context.Background()
	f := setupRebaseFixture(t, planRepoModes()[0])
	f.forgetLocalWorkspace(t)
	b := f.target.FeatureBranch()
	runGit(t, f.repo.invocation, "fetch", "-q", "origin", "refs/heads/"+b+":refs/heads/"+b)
	foreign := filepath.Join(f.repo.root, "gh-checkout")
	runGit(t, f.repo.invocation, "worktree", "add", "-q", foreign, b)
	canon, err := filepath.EvalSymlinks(foreign)
	if err != nil {
		t.Fatal(err)
	}
	gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}

	res := WorkspaceCheckout(ctx, f.deps, f.checkoutDeps(gh), f.repo.invocation, WorkspaceCheckoutRequest{ID: f.id})
	if res.Result != ResultApplied || res.Disposition != "attached" || res.Path != f.wp || res.Head != f.head || res.Operation != OperationWorkspaceCheckout {
		t.Fatalf("= %q/%q %s@%s (%s: %s)", res.Result, res.Disposition, res.Path, res.Head, res.Reason, res.Message)
	}
	if len(res.Findings) != 1 || res.Findings[0].Code != FindingForeignWorktreeRemoved || res.Findings[0].Path != canon {
		t.Errorf("findings = %+v; want one %s naming %s", res.Findings, FindingForeignWorktreeRemoved, canon)
	}
	if insp := WorkspaceInspect(ctx, f.deps, WorkspaceDeps{Service: f.svc}, f.repo.invocation, WorkspaceIDRequest{ID: f.id}); insp.State != "ready" {
		t.Errorf("inspect = %q; want ready", insp.State)
	}
	again := WorkspaceCheckout(ctx, f.deps, f.checkoutDeps(gh), f.repo.invocation, WorkspaceCheckoutRequest{ID: f.id, Revision: f.revision, Head: f.head})
	if again.Result != ResultNoOp || again.Disposition != "existing" {
		t.Errorf("rerun = %q/%q; want no-op/existing", again.Result, again.Disposition)
	}
}

// Every refusal leaves the clone as a machine that never built the change.
func TestIntegrationWorkspaceCheckoutRefusesBeforeAnyChange(t *testing.T) {
	requireRealGit(t)
	ctx := context.Background()
	f := setupRebaseFixture(t, planRepoModes()[0])
	f.forgetLocalWorkspace(t)
	for _, tc := range checkoutRefusalRows(f) {
		t.Run(tc.name, func(t *testing.T) {
			res := WorkspaceCheckout(ctx, f.deps, f.checkoutDeps(tc.gh), f.repo.invocation, tc.req)
			if res.Result != tc.result || res.Disposition != tc.disp || res.Reason != tc.reason {
				t.Fatalf("= %q/%q/%q (%s); want %q/%q/%q", res.Result, res.Disposition, res.Reason, res.Message, tc.result, tc.disp, tc.reason)
			}
		})
	}
	if insp := WorkspaceInspect(ctx, f.deps, WorkspaceDeps{Service: f.svc}, f.repo.invocation, WorkspaceIDRequest{ID: f.id}); insp.State != "absent" {
		t.Errorf("after the refusals the workspace is %q; want absent", insp.State)
	}
}

func TestIntegrationWorkspaceCheckoutRefusesNotImplemented(t *testing.T) {
	requireRealGit(t)
	f := setupRebaseFixtureStatus(t, planRepoModes()[0], "in-progress")
	gh := &fakeRebaseGitHub{repo: retargetRepo(), prs: []githubcli.PullRequest{f.prForHead(f.head, "")}}
	res := WorkspaceCheckout(context.Background(), f.deps, f.checkoutDeps(gh), f.repo.invocation, WorkspaceCheckoutRequest{ID: f.id})
	if res.Result != ResultBlocked || res.Reason != ReasonWorkspaceNotImplemented || f.localHead() != f.head {
		t.Fatalf("= %q/%q, head %s; want blocked/%s with the workspace untouched", res.Result, res.Reason, f.localHead(), ReasonWorkspaceNotImplemented)
	}
}
```

`checkoutRefusalRows(f *rebaseFixture)` returns `[]struct{name string; gh *fakeRebaseGitHub; req WorkspaceCheckoutRequest; result Result; disp, reason string}`, one per row (`open` = `[]githubcli.PullRequest{f.prForHead(f.head, "")}`, `other` = `strings.Repeat("e", 40)`; `gh(prs...)` = `&fakeRebaseGitHub{repo: retargetRepo(), prs: prs}`):

| name | gh | req | result / disp / reason |
|---|---|---|---|
| `revision-without-head` | `gh(open...)` | `{ID, Revision: f.revision}` | invalid-input / "" / `revision-head-pair` |
| `malformed-head` (Review Focus 5) | `gh(open...)` | `{ID, Revision: f.revision, Head: strings.ToUpper(f.head)}` | invalid-input / "" / `malformed-head` |
| `revision-drift` | `gh(open...)` | `{ID, Revision: 40 zeros, Head: f.head}` | contended / `contended` / `revision-drift` |
| `head-flag-mismatch` | `gh(open...)` | `{ID, Revision: f.revision, Head: other}` | contended / `contended` / `head-mismatch` |
| `pr-head-not-remote` | `gh(f.prForHead(other, ""))` | `{ID}` | contended / `contended` / `workspace.CheckoutReasonHeadMismatch` |
| `no-open-pr` | `gh()` | `{ID}` | blocked / `blocked` / `pr-not-open` |
| `pr-base-mismatch` | `gh(pr)` with `pr.BaseBranch = "develop"` | `{ID}` | blocked / `blocked` / `pr-base-mismatch` |
| `pr-probe-error` | `{repo: retargetRepo(), findErr: errors.New("gh down")}` | `{ID}` | external-failed / `unknown` / `pr-probe-failed` |
| `repo-unresolved` | `{repoErr: errors.New("no remote")}` | `{ID}` | external-failed / `unknown` / `repository-unresolved` |

Shard: copy `tests/test_go_integration_app_rebase.sh` to `tests/test_go_integration_app_checkout.sh` (0755) with `SHARD_PREFIX="TestIntegrationWorkspaceCheckout"` and a header describing the workspace.checkout operation tests; add `tests/test_go_integration_app_checkout.sh	40	parallel` after the `app_change` row.

CLI tests: in `internal/cli/workspace_test.go` add `{[]string{"workspace", "checkout"}, []string{"id", "revision", "head", "repo-dir"}}` to `TestWorkspaceCommandsRegistered` (doc comment "four" -> "five"), `"workspace checkout"` to `TestWorkspaceCommandsAssetIndependent`, and `TestWorkspaceCheckoutRoutesFlags` modeled on `TestWorkspaceCommitSpecRoutesFlags` (`workspace checkout --id 7 --repo-dir <tmp> --json` yields one document containing `"operation":"workspace.checkout"`; bare `workspace checkout` exits 2 naming `id`). In `capability_production_test.go` add `"workspace.checkout": "--id <id> [--head <ref>] [--repo-dir <dir>] [--revision <revision>]",` after the `workspace.prepare` row.

Run: `go test -count=1 ./internal/cli/` and the app shard prefix - FAIL.

- [ ] **Step 2: Implement `internal/app/workspace_checkout.go`.** Header comment: the operation puts an implemented change into its own worktree at the head its open PR and remote agree on; context, revision, and effective base come from the shared workspace context; the PR half of the head is proved with `probeRebasePR`'s whole predicate; `workspace.Service.Checkout` proves the remote half under its lock and owns every Git effect; a service error is `unknown`. Declare the **Interfaces** names (`checkoutDispUnknown = "unknown"` unexported). `WorkspaceCheckout`, with a local `refuse(result, disposition, reason, message)` building `newWorkspaceResult(op, result, WorkspaceOpResult{ID, Disposition, Reason, Message})`:

1. `(Revision == "") != (Head == "")` -> invalid-input `revision-head-pair`; `Head != "" && !validFullObjectID(Head)` -> invalid-input `malformed-head`.
2. `loadWorkspaceContext(ctx, deps, repoDir, ID, op)` (return its refusal). Status not `domain.StatusImplemented` -> blocked/`blocked`/`not-implemented` ("change %04d is %q, not implemented; workspace checkout takes over only an implemented change's pushed branch"). `Revision != "" && wc.revision != Revision` -> contended/`contended`/`revision-drift`. `resolveWorkspaceTarget` (return its refusal).
3. `cdeps.GitHub.DiscoverRepository(ctx, repoDir)` error -> external-failed/`unknown`/`repository-unresolved`. `FindOpenPullRequestsByHead(repo, strings.TrimPrefix(FeatureRef, branchRefPrefix))` error -> external-failed/`unknown`/`pr-probe-failed`; `len != 1` -> blocked/`blocked`/`pr-not-open`; `pr.BaseBranch != wc.base.Branch` -> blocked/`blocked`/`pr-base-mismatch`; `Head != "" && Head != pr.HeadCommit` -> contended/`contended`/`ReasonWorkspaceHeadMismatch`. Messages name the values compared.
4. `cdeps.Service.Checkout(ctx, workspace.CheckoutRequest{Repository: wc.repo, Remote: originRemote, Target: target, ExpectedHead: gitcli.ObjectID(pr.HeadCommit), MetadataWorktree: wc.pin.Layout.MetadataWorktree})`. Error -> `mapWorkspaceFailure(op, ID, err)` with `Disposition = "unknown"`.
5. Map: attached -> applied, existing -> no-op, blocked -> blocked, contended -> contended, else internal-error; copy `Path`, refs, `BaseCommit`, `Head`, disposition, reason, message. `RemovedWorktree` adds a `StatusFinding{Code: "foreign-worktree-removed", Severity: string(domain.SeverityWarning), Path: <removed>, Message: "removed the clean worktree another tool made for this change's branch; ignored files in it, such as build output, went with it"}`; `FastForwardedFrom` adds `local-branch-fast-forwarded` ("fast-forwarded <ref> from <old> to the pull request head <new>").

In `workspace_ops.go`: name `workspace checkout` (in `workspace_checkout.go`) in the file header; in `WorkspaceOpResult.HumanText`'s `r.Reason != ""` branch append `" - " + r.Message` when the message is non-empty, so a human sees the remedy. In `schema_registry.go` add `{ID: "workspace.checkout", Request: nil, Result: WorkspaceOpResult{}}, // WorkspaceCheckout` before the `workspace.commit-spec` row (gofmt-aligned).

- [ ] **Step 3: CLI.** In `internal/cli/workspace.go` (import `githubcli`): group `Short` -> `"Prepare, inspect, and publish feature workspaces for in-progress changes; check out implemented ones"`; add a `checkout` command modeled on `prepare` (`Annotations: capability("workspace.checkout", EffectLocalWrite)`, flags `id` "change `id` to check out (required)", `revision` "exact record `revision` the caller read; given together with --head", `head` "expected feature head `ref` (the pull request head); given together with --revision", `repo-dir`; only `id` required) whose `RunE` reads the flags, calls `newWorkspaceCheckoutDeps(repoDir)` and `setResult(app.WorkspaceCheckout(c.Context(), deps, cdeps, repoDir, app.WorkspaceCheckoutRequest{ID: id, Revision: revision, Head: head}))`; register it in `AddCommand`. Add `newWorkspaceCheckoutDeps(repoDir string) (app.PlanningDeps, app.WorkspaceCheckoutDeps, error)` beside `newWorkspaceDeps`: `newPlanningDeps(repoDir)`, `workspace.NewService(deps.Client)`, `githubcli.NewClient()`, returning `app.WorkspaceCheckoutDeps{Service: svc, GitHub: gh}` (each error returned).

Add `"workspace checkout": true,` to `assetIndependent` in `install.go`. In `docs/reference/cli.md` the `docket workspace` bullet becomes: "prepare, commit the spec copy onto, inspect, and publish feature workspaces for in-progress changes, and check out an implemented change's pushed branch into its own worktree on this machine. Verbs: `docket workspace --help`."

- [ ] **Step 4: Pass.** `gofmt -l internal/` (empty), `go test -count=1 ./internal/cli/ ./internal/app/` (default build) and `go test -tags integration -count=1 -run '^TestIntegrationWorkspaceCheckout' ./internal/app/` - PASS; `go run ./cmd/docket schema --operation workspace.checkout --json` exits 0. If the catalog orders the optional flags differently, take the generated signature only after confirming it is exactly `--id <id>` plus optional `--head <ref>`, `--repo-dir <dir>`, `--revision <revision>`.

- [ ] **Step 5: Mutate** (backup, `-count=1`, restore): `pr.BaseBranch != wc.base.Branch` -> false reddens `RefusesBeforeAnyChange/pr-base-mismatch`; `req.Head != "" && req.Head != pr.HeadCommit` -> false reddens `head-flag-mismatch`; `req.Revision != "" && wc.revision != req.Revision` -> false reddens `revision-drift`.

- [ ] **Step 6: Commit** `feat(app): workspace.checkout operation and docket workspace checkout (change 0551)`.

---

### Task 5: End-to-end - checkout lets a `finalize.gate: off` merge land

Suggested tier: standard.

**Files:** Modify `internal/app/workspace_checkout_integration_test.go`.

- [ ] **Step 1: Test.**

```go
// On a machine that never built the change, a gate-off finalize.merge refuses
// with zero merge calls (no local head agrees with the PR and remote heads);
// after workspace.checkout the heads agree and the merge lands once.
func TestIntegrationWorkspaceCheckoutLetsGateOffMergeLand(t *testing.T) {
	requireRealGit(t)
	ctx := context.Background()
	f := setupMergeFixture(t, planRepoModes()[0])
	f.repo.writerAdvance(t, "main", map[string]string{
		".docket.yml": "integration_branch: main\nbuild:\n  test_command: 'go test ./...'\nfinalize:\n  gate: \"off\"\n  test_command: 'go test ./...'\n",
	})
	mergeCommit := f.mergeFeatureIntoBase(t)
	gh := f.carryFake(t, false) // no green evidence: only gate off satisfies the gate
	gh.mergeOutcome = githubcli.MergeMerged
	gh.mergeFacts = mergedFactsFor(f.head, "main", mergeCommit)
	f.forgetLocalWorkspace(t)

	if before := FinalizeMerge(ctx, f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false)); before.Result == ResultApplied || gh.mergeCalls != 0 {
		t.Fatalf("merge without a workspace = %q (%s), %d call(s); want a refusal with none", before.Result, before.Reason, gh.mergeCalls)
	}
	co := WorkspaceCheckout(ctx, f.deps, f.checkoutDeps(gh), f.repo.invocation, WorkspaceCheckoutRequest{ID: f.id, Revision: f.revision, Head: f.head})
	if co.Result != ResultApplied {
		t.Fatalf("checkout = %q/%q (%s: %s)", co.Result, co.Disposition, co.Reason, co.Message)
	}
	res := FinalizeMerge(ctx, f.mergeDeps(gh), f.repo.invocation, mergeReq(f, f.head, true, false))
	if res.Result != ResultApplied || res.Disposition != MergeDispMerged || gh.mergeCalls != 1 {
		t.Fatalf("merge after checkout = %q/%q (%s: %s), %d call(s)", res.Result, res.Disposition, res.Reason, res.Message, gh.mergeCalls)
	}
}
```

- [ ] **Step 2: Pass.** `go test -tags integration -count=1 -run '^TestIntegrationWorkspaceCheckoutLetsGateOffMergeLand$' ./internal/app/`. If the first merge refuses for a reason other than the heads condition, fix the fixture so only the missing workspace is wrong; never relax the assert.

- [ ] **Step 3: Mutate.** Comment out the `WorkspaceCheckout` call and its check: the final merge goes red. Restore.

- [ ] **Step 4: Commit** `test(app): checkout then a gate-off finalize merge lands with agreeing heads (change 0551)`.

---

### Task 6: The finalize skill checks the change out first

Suggested tier: standard.

**Files:** Modify `skills/docket-finalize-change/SKILL.md`, `internal/repoguard/budgets_test.go`, `internal/repoguard/prose_contracts_test.go`; regenerate `internal/assets/`.

- [ ] **Step 1: Failing contract.** Append to the prose-contract table in `prose_contracts_test.go`, after the `align_0502_one_layout` rows:

```go
	// 0551: finalize checks the change out before any step reads the workspace,
	// in every gate mode, and routes owned-workspace refusals on to finalize.rebase.
	{sentinel: "align_0551_finalize_checkout", file: "skills/docket-finalize-change/SKILL.md",
		present: []string{
			"the `workspace.checkout` operation with `--id <id> --revision <revision> --head <feature head>`",
			"in every gate mode",
			"`blocked` with `workspace-dirty`, `local-head-mismatch`, or `workspace-not-ready`",
			"any other `blocked` is `halted`",
		}},
```

`go test -count=1 -run Prose ./internal/repoguard/` - FAIL.

- [ ] **Step 2: Skill.** Under `### 1. Authoritative context`, after its paragraph, add a blank line and:

```markdown
**Local checkout.** When the selected candidate's PR is open, put the change into its own workspace on this machine before any step that reads it, in every gate mode (`finalize.merge` compares the local head even when `finalize.gate` is `off`): the `workspace.checkout` operation with `--id <id> --revision <revision> --head <feature head>`. It adopts the pushed branch only at the head the PR and the remote agree on, and removes a foreign worktree only when it is provably clean at that head. Route on `disposition`: `existing`/`attached` - continue, naming any `foreign-worktree-removed` finding's path in the final report; `blocked` with `workspace-dirty`, `local-head-mismatch`, or `workspace-not-ready` - the workspace is already docket's own (an owned rebase may be in flight), so continue and let `finalize.rebase` recover or refuse it; any other `blocked` is `halted`, relaying the message verbatim; `contended` - re-read `context.finalize` (the run is `contended`); `unknown` - `halted`. A candidate whose PR already merged has nothing to check out and skips this.
```

- [ ] **Step 3: Budget and assets.** `wc -lw skills/docket-finalize-change/SKILL.md` -> `<L>`/`<W>`; set the `docket-finalize-change/SKILL.md` row in `budgets_test.go` to `<L>, <W>` and prefix its comment with `0551: step 1 checks the change out into its own workspace first (241/6033 -> <L>/<W>); `. Run `go generate ./internal/assets/`.

- [ ] **Step 4: Pass.** `go test -count=1 ./internal/repoguard/ ./internal/assets/` - PASS.

- [ ] **Step 5: Mutate.** Change "any other `blocked` is `halted`" to "... continues": the contract goes red. Restore and regenerate.

- [ ] **Step 6: Commit** `feat(skills): finalize checks the change out into its own workspace first (change 0551)` (include `internal/assets/`).

---

### Task 7: Guide and concept docs

Suggested tier: economy.

**Files:** Modify `docs/guide/landing-changes.md`, `docs/concepts/finalize-sequencer.md`. Current behavior only; no change numbers.

- [ ] **Step 1: Guide.** In "What finalize does, in order", insert a new item 1 and renumber the rest 2-7: "checks the change out into its own worktree on the machine running finalize, when it is not already there (see [Working on a change built on another machine](#working-on-a-change-built-on-another-machine));". Before `## When finalize is blocked` add:

```markdown
## Working on a change built on another machine

Finalize needs the change's branch in a worktree docket made, at `.worktrees/<slug>` in your
clone. When the change was built on another machine, or its branch was checked out with
`gh pr checkout` or a hand-made `git worktree add`, run `docket workspace checkout --id <id>`.

It checks the change out at the head its pull request and pushed branch agree on: it reuses a
local branch at that head, fast-forwards one that is behind, and deletes a clean worktree another
tool made for the branch (ignored files such as build output go with it). It refuses anything it
cannot prove safe - local commits the pull request lacks, uncommitted changes, an unfinished
rebase, the branch checked out in your main checkout - and says what to run instead. It works for
an `implemented` change with an open pull request.

Finalize runs the same checkout before it rebases, so you rarely need it by hand. Avoid
`gh pr checkout` and `git worktree add` for these branches: docket cannot use the worktrees they
make until a checkout replaces them.
```

- [ ] **Step 2: Concept.** In `finalize-sequencer.md`'s diagram insert between `approved PR` and `finalize retarget-children` a `workspace checkout: the change's own worktree on this machine` box with the `(adopts the pushed branch only at the PR head)` note, connectors matching the diagram; add a first bullet under it: "Finalize first checks the change out into its own worktree on the machine running it, so a change built elsewhere lands the same way. It adopts only a local state proven identical to, or behind, the head the pull request and the pushed branch agree on, and removes a worktree another tool made only when it is clean at that head."

- [ ] **Step 3:** `go test -count=1 ./internal/repoguard/` - PASS.

- [ ] **Step 4: Commit** `docs(guide): working on a change built on another machine (change 0551)`.

---

### Build gate

- [ ] Run the whole suite from the feature worktree via `build.test_command` (`go run ./cmd/docket development test`); it must be green. Read the budget report: `BUDGET WATCH:`/`PARALLEL-SENSITIVE:` on the two new shards, `app_rebase`, or the gitcli shards is a finding to record; `SERIAL CONFIRMED OVER BUDGET:` is a breach to fix by splitting, never by raising the number.
