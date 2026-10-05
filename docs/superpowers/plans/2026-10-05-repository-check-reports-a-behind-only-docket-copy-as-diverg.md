<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0523 — Treat a behind-only .docket copy as healthy and make prepare fast-forward it in place](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-05-0523-repository-check-reports-a-behind-only-docket-copy-as-diverg.md)**
<!-- docket:backlink:end -->
# Behind-Only .docket Is Healthy, and Prepare Fast-Forwards In Place — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. In this repository the plan is executed by `docket-build`.

**Goal:** `docket repository check` treats a clean, behind-only local `.docket` copy as healthy and keys every local-copy finding on the real ancestry relationship; `docket repository prepare` fast-forwards `.docket` in place (nothing removed, compare-and-swap, no hooks, refuse rather than discard) and its attach path checks freshness; the primary-checkout finding is split by relationship; the Bash upgrade guide drops its extra `prepare` step.

**Architecture:** One pure relationship type (`reposetup.SyncRelation`) carried in `reposetup.Facts`, computed by ONE app function (`syncRelationship`, moved out of `repository_prepare.go`) that both the check augmentation and the prepare augmentation call. The classifier and health findings key on that fact, never on tip equality. Two new `gitcli` primitives — `FastForwardCheckedOutBranch` (update-ref CAS, then `read-tree -u -m`, rolled back on refusal, hooks forced off) and `AdvanceBranchChecked` (update-ref CAS on an unattached branch) — replace prepare's remove/delete/re-add sequence and give prepare's attach path a fresh-attach action. The ancestry probe ignores replace refs and grafts.

**Tech Stack:** Go (`internal/reposetup`, `internal/app`, `internal/gitcli`, `internal/bashupgrade`), real-git integration tests behind the `integration` build tag, POSIX shard runners under `tests/`.

**Spec:** `docs/superpowers/specs/2026-10-05-repository-check-reports-a-behind-only-docket-copy-as-diverg-design.md` (on the `docket` branch; read it from `/Users/homer/dev/docket/.docket/docs/superpowers/specs/2026-10-05-repository-check-reports-a-behind-only-docket-copy-as-diverg-design.md`).

## Global Constraints

- A clean, behind-only local `.docket` copy is **healthy**: `check` exits 0 with **no finding** for it (no warning — a healthy state with any finding exits 1 via `CheckExit`).
- Fix `check`, not the writers: typed operations keep pushing from detached worktrees; nothing here makes them touch `.docket`.
- **One relationship computation.** `check`, `configure-tests`, and `prepare` share one function. It may be renamed and moved; it must never be copied.
- The primary checkout's health condition `CondPrimaryAtRemoteTip` is unchanged (HEAD must equal the tip); only the finding is split by relationship.
- `prepare` fast-forward properties (spec §3): nothing removed; CAS on the router's observed tip; no repository hooks run (forced empty hooks dir per command); refuse rather than discard; interruption never destroys state; idempotent.
- `ensureMetadataWorktree`'s contract for `init` and `migrate` does not change. `prepare` gets its own fresh-attach action.
- Do not add ancestry probes to `gatherRepoFacts` / `primaryAtTipPresence` (the shared base gather every operational command runs). Capturing the already-read primary HEAD there is allowed; computing a relationship there is not.
- Out of scope, unchanged: `init`'s refusal text, `repair`'s output, `migrate`/`init` attach behavior, self-healing a missing hooks-off config, `transaction.Engine.candidateReachable`, other `check` findings.
- Every printed remedy must be valid in the exact state that produced it and never destructive (learning `printed-remedy-state-validity`; `assertNoDestructiveCommand` in `internal/reposetup/health_test.go`).
- Docs describe current behavior only: no change or PR numbers in `docs/release/upgrading-from-bash.md`.
- Cross-references in maintained source anchor on symbol names or quoted clauses, never line numbers (AGENTS.md, `TestCommentAnchorStyle`).
- Every mutation probe and re-verification run uses `go test -count=1` (learning `cached-runner-serves-a-mutated-tree`), and every mutation is restored from a backup copy, never `git checkout --` (learning `mutation-restore-needs-a-backup-copy`).
- Integration tests run only through their shard prefixes: `go test -tags integration -count=1 -run '^<Prefix>' <pkg>`. Never run `./internal/app/` integration tests without `-run`.

## Review Focus

1. **An ignored file sitting at a path the target newly tracks.** This follows Git's normal checkout rule, as spec §3 property 4 allows ("Ignored files survive, except at a path the target itself tracks, which is Git's normal checkout rule"): ignored files are expendable, so modern Git's `read-tree -u -m` (verified on git 2.55.0, like `merge --ff-only` and `checkout`) overwrites the file with the target's version and the fast-forward succeeds. Docket adds no stricter refusal (decided 2026-10-05 after the Task 2 halt). An ignored file at any path the target does not track survives, and an untracked, non-ignored file in the way still refuses. Pinned in Task 2, subtest "ignored file at a newly tracked path follows Git's checkout rule".
2. **Stale index stat info** (a tracked file touched but unchanged, e.g. by an editor or `touch`). A person expects the fast-forward to still work. Pinned in Task 2, subtest "stale stat info on a changed path still fast-forwards" (the `update-index --refresh` step is load-bearing).
3. **An interrupted fast-forward** (branch moved, tree not yet updated). A person expects `check` to say dirty and `prepare` to refuse, never to report healthy and never to need `git worktree prune`. Pinned in Task 7, `TestIntegrationRepoInPlaceFFInterruptedReadsDirty`.
4. **A replace ref or graft that fakes ancestry.** A person expects local-only commits never to be fast-forwarded over. Pinned in Task 1 (both replace refs and the graft file).
5. **`.docket` on a detached HEAD or a different branch at execution time.** A person expects the primitive to refuse, never to move some other ref. Pinned in Task 2, subtest "HEAD not on the branch refuses".

## File Structure

- `internal/gitcli/push.go` (modify): `IsAncestor` delegates to a new shared `isAncestorIn`; new `IsAncestorIgnoringReplacements`.
- `internal/gitcli/exec.go` (modify): `runRequest.env` doc comment admits the graft-disabling override.
- `internal/gitcli/hooksoff.go` (modify): extract `emptyHooksDir` from `DisableWorktreeHooks`.
- `internal/gitcli/fastforward_inplace.go` (create): `FastForwardCheckedOutBranch`.
- `internal/gitcli/refadvance.go` (create): `AdvanceBranchChecked`.
- `internal/gitcli/ancestry_integration_test.go`, `fastforward_inplace_integration_test.go`, `refadvance_integration_test.go` (create): real-git tests, prefix `TestIntegrationRepo` (existing `tests/test_go_integration_gitcli_repo.sh` shard).
- `internal/reposetup/probe.go` (modify): `SyncRelation`, `Facts.LocalMetadataSync`, `Facts.PrimaryTipRelation`, `WorktreeFact.UnfinishedOperation`, `Synchronized` comment.
- `internal/reposetup/classify.go`, `health.go`, `healthconditions.go` (modify): relationship-keyed reasons and findings.
- `internal/reposetup/classify_test.go`, `health_test.go` (modify).
- `internal/app/repository_sync_relation.go` (create): `syncRelationship`, `applyLocalMetadataSync`.
- `internal/app/repository_check.go` (modify): `augmentCheckFacts`, `worktreeCleanState`; delete `synchronizedPresence`.
- `internal/app/repository_prepare.go` (modify): router on `f.LocalMetadataSync`, `observedTip`, in-place fast-forward, fresh attach; delete `prepareSync`, `prepareSyncRelationship`, `prepareFastForwardWorktree`.
- `internal/app/repository_facts.go` (modify): `setupContext.primaryHead`.
- `internal/app/repository_prepare_test.go`, `repository_check_test.go` (modify).
- `internal/app/reposynccheck_integration_test.go` (create): prefix `TestIntegrationRepoSyncCheck`.
- `internal/app/repoinplaceff_integration_test.go` (create): prefix `TestIntegrationRepoInPlaceFF`.
- `internal/app/repoprepare_integration_test.go` (modify): header comment only.
- `tests/test_go_integration_app_reposynccheck.sh`, `tests/test_go_integration_app_repoinplaceff.sh` (create) + rows in `tests/runtime-budgets.tsv`.
- `docs/release/upgrading-from-bash.md`, `internal/bashupgrade/registry_test.go` (modify).

Task ordering note (learning `intermediate-task-state-buildable`): Task 4 changes the pure classifier only. Between Task 4 and Task 5 the `internal/bashupgrade` guide test (`tests/test_go_integration_bashupgrade.sh`) is expected red, because it still asserts the old false-conflict codes; Task 5 owns the guide and its test. Every other package stays green at every task boundary.

---

### Task 1: Ancestry probe that ignores replace refs and grafts

**Files:**
- Modify: `internal/gitcli/push.go` (`IsAncestor`)
- Modify: `internal/gitcli/exec.go` (`runRequest.env` doc comment)
- Create: `internal/gitcli/ancestry_integration_test.go`

**Interfaces:**
- Produces: `func (c *Client) IsAncestorIgnoringReplacements(ctx context.Context, repo Repository, ancestor, descendant ObjectID) (bool, error)` and the unexported `func (c *Client) isAncestorIn(ctx context.Context, dir string, ancestor, descendant ObjectID, ignoreReplacements bool) (bool, error)` that Tasks 2 and 3 call with a worktree dir.

- [ ] **Step 1: Write the failing test**

Create `internal/gitcli/ancestry_integration_test.go`:

```go
//go:build integration

package gitcli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestIntegrationRepoIsAncestorIgnoringReplacements proves the relationship probe
// ignores both history-rewrite mechanisms: a replace ref and the legacy graft file
// each make an unrelated orphan commit look like a descendant to plain IsAncestor,
// and IsAncestorIgnoringReplacements must still answer false.
func TestIntegrationRepoIsAncestorIgnoringReplacements(t *testing.T) {
	ctx := context.Background()
	newOrphan := func(t *testing.T, r *testRepos) (base, orphan ObjectID) {
		t.Helper()
		base = ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))
		tree := gitOut(t, r.Invocation, "rev-parse", "HEAD^{tree}")
		orphan = ObjectID(strings.TrimSpace(gitOut(t, r.Invocation,
			"-c", "user.name=t", "-c", "user.email=t@example.com",
			"commit-tree", tree, "-m", "orphan")))
		return base, orphan
	}

	t.Run("replace ref", func(t *testing.T) {
		r := newMainModeRepos(t)
		c := newRealClient(t)
		base, orphan := newOrphan(t, r)
		gitOut(t, r.Invocation, "replace", "--graft", string(orphan), string(base))
		repo := Repository{PrimaryWorktree: r.Invocation}

		plain, err := c.IsAncestor(ctx, repo, base, orphan)
		if err != nil || !plain {
			t.Fatalf("premise: plain IsAncestor under a replace graft = %v, %v; want true, nil", plain, err)
		}
		got, err := c.IsAncestorIgnoringReplacements(ctx, repo, base, orphan)
		if err != nil || got {
			t.Fatalf("IsAncestorIgnoringReplacements = %v, %v; want false, nil (replace refs ignored)", got, err)
		}
	})

	t.Run("graft file", func(t *testing.T) {
		r := newMainModeRepos(t)
		c := newRealClient(t)
		base, orphan := newOrphan(t, r)
		grafts := filepath.Join(r.Invocation, ".git", "info", "grafts")
		if err := os.MkdirAll(filepath.Dir(grafts), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(grafts, []byte(string(orphan)+" "+string(base)+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		repo := Repository{PrimaryWorktree: r.Invocation}

		plain, err := c.IsAncestor(ctx, repo, base, orphan)
		if err != nil || !plain {
			t.Fatalf("premise: plain IsAncestor under a graft file = %v, %v; want true, nil", plain, err)
		}
		got, err := c.IsAncestorIgnoringReplacements(ctx, repo, base, orphan)
		if err != nil || got {
			t.Fatalf("IsAncestorIgnoringReplacements = %v, %v; want false, nil (graft file ignored)", got, err)
		}
	})

	t.Run("real ancestry still true", func(t *testing.T) {
		r := newMainModeRepos(t)
		c := newRealClient(t)
		base := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))
		writeWorktreeFile(t, r.Invocation, "next.txt", "next\n")
		gitOut(t, r.Invocation, "add", "--", "next.txt")
		gitOut(t, r.Invocation, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "next")
		next := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))
		got, err := c.IsAncestorIgnoringReplacements(ctx, Repository{PrimaryWorktree: r.Invocation}, base, next)
		if err != nil || !got {
			t.Fatalf("real ancestry = %v, %v; want true, nil", got, err)
		}
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -tags integration -count=1 -run '^TestIntegrationRepoIsAncestorIgnoringReplacements$' ./internal/gitcli/`
Expected: FAIL to compile — `c.IsAncestorIgnoringReplacements undefined`.

- [ ] **Step 3: Implement**

In `internal/gitcli/push.go`, replace the body of `IsAncestor` and add the two functions below it:

```go
func (c *Client) IsAncestor(ctx context.Context, repo Repository, ancestor, descendant ObjectID) (bool, error) {
	return c.isAncestorIn(ctx, repo.PrimaryWorktree, ancestor, descendant, false)
}

// IsAncestorIgnoringReplacements is IsAncestor with Git's history rewrites switched
// off: replace refs are ignored (`--no-replace-objects`) and so is the legacy graft
// file (GIT_GRAFT_FILE set to the empty string, which Git opens as no file). A
// relationship probe that decides whether a fast-forward is safe uses it, so a
// replace ref or graft can never fake "behind" and strand local-only commits.
// Other callers keep IsAncestor's behavior.
func (c *Client) IsAncestorIgnoringReplacements(ctx context.Context, repo Repository, ancestor, descendant ObjectID) (bool, error) {
	return c.isAncestorIn(ctx, repo.PrimaryWorktree, ancestor, descendant, true)
}

// isAncestorIn runs `git merge-base --is-ancestor` in dir: exit 0 is true, exit 1
// is false, and any other exit is a typed command-failed *Failure.
func (c *Client) isAncestorIn(ctx context.Context, dir string, ancestor, descendant ObjectID, ignoreReplacements bool) (bool, error) {
	if err := validateObjectID(ancestor); err != nil {
		return false, newFailure(isAncestorOp, KindInvalidRequest, "invalid ancestor id", err)
	}
	if err := validateObjectID(descendant); err != nil {
		return false, newFailure(isAncestorOp, KindInvalidRequest, "invalid descendant id", err)
	}
	args := []string{"merge-base", "--is-ancestor", string(ancestor), string(descendant)}
	var env []string
	if ignoreReplacements {
		args = append([]string{"--no-replace-objects"}, args...)
		env = []string{"GIT_GRAFT_FILE="}
	}
	res, f := c.run(ctx, runRequest{op: isAncestorOp, dir: dir, args: args, env: env})
	if f != nil {
		return false, f
	}
	switch res.exitCode {
	case 0:
		return true, nil
	case 1:
		return false, nil
	default:
		return false, newFailure(isAncestorOp, KindCommandFailed, "merge-base --is-ancestor failed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
	}
}
```

In `internal/gitcli/exec.go`, extend the `env` field comment on `runRequest`: after "used for the engine-clock commit dates (GIT_AUTHOR_DATE / GIT_COMMITTER_DATE)" add "and for the ancestry probe's graft-disabling GIT_GRAFT_FILE= (IsAncestorIgnoringReplacements)". Keep the "never carries repository redirection, config injection, or credentials" clause — an empty graft file disables a rewrite, it redirects nothing.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -tags integration -count=1 -run '^TestIntegrationRepoIsAncestorIgnoringReplacements$' ./internal/gitcli/ && go test -count=1 ./internal/gitcli/`
Expected: PASS.

- [ ] **Step 5: Mutation-check the key**

Copy `internal/gitcli/push.go` to `${TMPDIR:-/tmp}/push.go.bak`. Delete the `env = []string{"GIT_GRAFT_FILE="}` line; re-run the Step 4 integration command: the `graft file` subtest must FAIL. Restore with `cp "${TMPDIR:-/tmp}/push.go.bak" internal/gitcli/push.go`. Repeat removing only the `--no-replace-objects` prepend: the `replace ref` subtest must FAIL. Restore and confirm `git diff --stat internal/gitcli/push.go` shows only your Step 3 change.

- [ ] **Step 6: Commit**

```bash
git add internal/gitcli/push.go internal/gitcli/exec.go internal/gitcli/ancestry_integration_test.go
git commit -m "feat(gitcli): ancestry probe that ignores replace refs and grafts"
```

---

### Task 2: In-place fast-forward of a checked-out branch

**Files:**
- Modify: `internal/gitcli/hooksoff.go` (extract `emptyHooksDir` from `DisableWorktreeHooks`)
- Create: `internal/gitcli/fastforward_inplace.go`
- Create: `internal/gitcli/fastforward_inplace_integration_test.go`

**Interfaces:**
- Consumes: `isAncestorIn` (Task 1).
- Produces:
  - `func (c *Client) emptyHooksDir(ctx context.Context, op Operation, worktreeDir string) (string, *Failure)` — resolves and creates `<git-common-dir>/docket/empty-hooks`, returns its absolute path.
  - `func (c *Client) FastForwardCheckedOutBranch(ctx context.Context, worktreeDir string, branch RefName, expectedTip, target ObjectID) error` — Task 7 calls it with the `.docket` path, `refs/heads/docket`, the router's observed tip, and the pinned remote tip.

Mechanism (decide it here, not later): (1) refuse unless `symbolic-ref HEAD` in the worktree is exactly `branch`; (2) refuse unless `target` descends from `expectedTip` (replace refs and grafts ignored); (3) compare-and-swap the branch ref `expectedTip → target` with `update-ref <branch> <target> <expectedTip>` — if the branch moved, nothing changes; (4) `update-index -q --refresh` (exit 0 or 1 accepted); (5) `read-tree -u -m <expectedTip> <target>`, which updates index and files from the old tree to the new one and refuses on local changes or untracked (non-ignored) files in the way — an ignored file at a path the target tracks is overwritten, Git's normal checkout rule (Review Focus 1); (6) on any read-tree refusal, compare-and-swap the branch back `target → expectedTip`. Every command carries `-c core.hooksPath=<empty-hooks>`, which also silences `reference-transaction` and `post-index-change`. An interruption between (3) and (5) leaves HEAD at target with the old index: `git status` reports staged changes, so it reads dirty — never clean, never a missing worktree.

- [ ] **Step 1: Write the failing tests**

Create `internal/gitcli/fastforward_inplace_integration_test.go`:

```go
//go:build integration

package gitcli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/danielhanold/docket/internal/testsupport"
)

const ffIdent = "user.email=t@example.com"

// newCheckedOutBranchFixture builds an invocation clone whose branch "meta" sits at
// base and is checked out in a linked worktree wt, plus one later commit on main
// (target) that descends from base, rewrites README.md, and adds target.txt.
func newCheckedOutBranchFixture(t *testing.T) (r *testRepos, wt string, base, target ObjectID) {
	t.Helper()
	r = newMainModeRepos(t)
	base = ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))
	wt = filepath.Join(testsupport.TempDir(t), "meta-wt")
	gitOut(t, r.Invocation, "worktree", "add", "-q", "-b", "meta", wt, string(base))
	writeWorktreeFile(t, r.Invocation, "README.md", "readme v2\n")
	writeWorktreeFile(t, r.Invocation, "target.txt", "target\n")
	gitOut(t, r.Invocation, "add", "--", "README.md", "target.txt")
	gitOut(t, r.Invocation, "-c", "user.name=t", "-c", ffIdent, "commit", "-q", "-m", "target")
	target = ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))
	return r, wt, base, target
}

func metaTip(t *testing.T, r *testRepos) ObjectID {
	t.Helper()
	return ObjectID(gitOut(t, r.Invocation, "rev-parse", "refs/heads/meta"))
}

// installHooks writes the named hooks into a fresh dir; each appends its own name to
// sentinel and exits with code. The dir is returned for core.hooksPath.
func installHooks(t *testing.T, sentinel string, code int, names ...string) string {
	t.Helper()
	dir := filepath.Join(testsupport.TempDir(t), "hooks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		body := "#!/bin/sh\necho " + n + " >> '" + sentinel + "'\nexit " + strconv.Itoa(code) + "\n"
		if err := os.WriteFile(filepath.Join(dir, n), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestIntegrationRepoFastForwardCheckedOutBranch(t *testing.T) {
	ctx := context.Background()

	t.Run("advances branch, index, and files in place", func(t *testing.T) {
		r, wt, base, target := newCheckedOutBranchFixture(t)
		c := newRealClient(t)
		before, err := os.Stat(wt)
		if err != nil {
			t.Fatal(err)
		}
		if err := c.FastForwardCheckedOutBranch(ctx, wt, "refs/heads/meta", base, target); err != nil {
			t.Fatalf("FastForwardCheckedOutBranch: %v", err)
		}
		if got := metaTip(t, r); got != target {
			t.Fatalf("meta = %s, want %s", got, target)
		}
		if got := ObjectID(gitOut(t, wt, "rev-parse", "HEAD")); got != target {
			t.Fatalf("worktree HEAD = %s, want %s", got, target)
		}
		if s := gitOut(t, wt, "status", "--porcelain", "--untracked-files=all"); s != "" {
			t.Fatalf("worktree not clean after fast-forward:\n%s", s)
		}
		if b, _ := os.ReadFile(filepath.Join(wt, "target.txt")); string(b) != "target\n" {
			t.Fatalf("target.txt = %q, want the target content", b)
		}
		after, err := os.Stat(wt)
		if err != nil || !os.SameFile(before, after) {
			t.Fatalf("worktree directory was replaced (err=%v)", err)
		}
		if log := gitOut(t, r.Invocation, "reflog", "show", "--format=%gs", "refs/heads/meta"); !strings.Contains(log, "docket: fast-forward") {
			t.Fatalf("branch reflog lacks the fast-forward entry:\n%s", log)
		}
	})

	t.Run("ignored file at an untouched path survives", func(t *testing.T) {
		r, wt, base, target := newCheckedOutBranchFixture(t)
		c := newRealClient(t)
		appendExclude(t, r.Invocation, "*.local")
		writeWorktreeFile(t, wt, "notes.local", "keep me\n")
		if err := c.FastForwardCheckedOutBranch(ctx, wt, "refs/heads/meta", base, target); err != nil {
			t.Fatalf("FastForwardCheckedOutBranch: %v", err)
		}
		if b, err := os.ReadFile(filepath.Join(wt, "notes.local")); err != nil || string(b) != "keep me\n" {
			t.Fatalf("ignored file lost: %q, %v", b, err)
		}
	})

	// Git's normal checkout rule: an ignored file at a path the target tracks is
	// expendable, so it is replaced by the target's version (spec §3 property 4).
	t.Run("ignored file at a newly tracked path follows Git's checkout rule", func(t *testing.T) {
		r, wt, base, target := newCheckedOutBranchFixture(t)
		c := newRealClient(t)
		appendExclude(t, r.Invocation, "target.txt")
		writeWorktreeFile(t, wt, "target.txt", "mine\n")
		if err := c.FastForwardCheckedOutBranch(ctx, wt, "refs/heads/meta", base, target); err != nil {
			t.Fatalf("FastForwardCheckedOutBranch: %v", err)
		}
		if got := metaTip(t, r); got != target {
			t.Fatalf("meta = %s, want %s", got, target)
		}
		if b, _ := os.ReadFile(filepath.Join(wt, "target.txt")); string(b) == "mine\n" {
			t.Fatalf("target.txt still holds the ignored content; want the target's tracked version")
		}
	})

	t.Run("untracked file in the way refuses and rolls back", func(t *testing.T) {
		r, wt, base, target := newCheckedOutBranchFixture(t)
		c := newRealClient(t)
		writeWorktreeFile(t, wt, "target.txt", "mine\n")
		if err := c.FastForwardCheckedOutBranch(ctx, wt, "refs/heads/meta", base, target); err == nil {
			t.Fatal("fast-forward over an untracked file succeeded; want refusal")
		}
		if got := metaTip(t, r); got != base {
			t.Fatalf("meta = %s after refusal, want rolled back to %s", got, base)
		}
		if got := ObjectID(gitOut(t, wt, "rev-parse", "HEAD")); got != base {
			t.Fatalf("worktree HEAD = %s after refusal, want %s", got, base)
		}
		if b, _ := os.ReadFile(filepath.Join(wt, "target.txt")); string(b) != "mine\n" {
			t.Fatalf("untracked file changed: %q", b)
		}
	})

	t.Run("stale stat info on a changed path still fast-forwards", func(t *testing.T) {
		r, wt, base, target := newCheckedOutBranchFixture(t)
		c := newRealClient(t)
		future := time.Now().Add(2 * time.Hour)
		if err := os.Chtimes(filepath.Join(wt, "README.md"), future, future); err != nil {
			t.Fatal(err)
		}
		if err := c.FastForwardCheckedOutBranch(ctx, wt, "refs/heads/meta", base, target); err != nil {
			t.Fatalf("FastForwardCheckedOutBranch with stale stat: %v", err)
		}
		if got := metaTip(t, r); got != target {
			t.Fatalf("meta = %s, want %s", got, target)
		}
	})

	t.Run("locked worktree fast-forwards and stays locked", func(t *testing.T) {
		r, wt, base, target := newCheckedOutBranchFixture(t)
		c := newRealClient(t)
		gitOut(t, r.Invocation, "worktree", "lock", wt)
		if err := c.FastForwardCheckedOutBranch(ctx, wt, "refs/heads/meta", base, target); err != nil {
			t.Fatalf("FastForwardCheckedOutBranch on a locked worktree: %v", err)
		}
		if list := gitOut(t, r.Invocation, "worktree", "list", "--porcelain"); !strings.Contains(list, "\nlocked") {
			t.Fatalf("lock lost:\n%s", list)
		}
	})

	t.Run("repository hooks never run", func(t *testing.T) {
		r, wt, base, target := newCheckedOutBranchFixture(t)
		c := newRealClient(t)
		sentinel := filepath.Join(testsupport.TempDir(t), "hook-ran")
		hooks := installHooks(t, sentinel, 1, "reference-transaction", "post-index-change", "post-checkout", "post-merge")
		gitOut(t, r.Invocation, "config", "core.hooksPath", hooks)
		if err := c.FastForwardCheckedOutBranch(ctx, wt, "refs/heads/meta", base, target); err != nil {
			t.Fatalf("FastForwardCheckedOutBranch with failing repo hooks: %v", err)
		}
		if b, err := os.ReadFile(sentinel); err == nil {
			t.Fatalf("a repository hook ran: %q", b)
		}
	})

	t.Run("stale expected tip refuses and changes nothing", func(t *testing.T) {
		r, wt, base, target := newCheckedOutBranchFixture(t)
		c := newRealClient(t)
		writeWorktreeFile(t, wt, "local-only.txt", "local\n")
		gitOut(t, wt, "add", "--", "local-only.txt")
		gitOut(t, wt, "-c", "user.name=t", "-c", ffIdent, "commit", "-q", "-m", "local-only")
		moved := metaTip(t, r)
		if err := c.FastForwardCheckedOutBranch(ctx, wt, "refs/heads/meta", base, target); err == nil {
			t.Fatal("fast-forward from a stale expected tip succeeded; want refusal")
		}
		if got := metaTip(t, r); got != moved {
			t.Fatalf("meta = %s, want the local commit %s kept", got, moved)
		}
		if _, err := os.Stat(filepath.Join(wt, "local-only.txt")); err != nil {
			t.Fatalf("local commit's file gone: %v", err)
		}
	})

	t.Run("target not descending refuses", func(t *testing.T) {
		r, wt, base, _ := newCheckedOutBranchFixture(t)
		c := newRealClient(t)
		tree := gitOut(t, r.Invocation, "rev-parse", "HEAD^{tree}")
		orphan := ObjectID(gitOut(t, r.Invocation, "-c", "user.name=t", "-c", ffIdent, "commit-tree", tree, "-m", "orphan"))
		if err := c.FastForwardCheckedOutBranch(ctx, wt, "refs/heads/meta", base, orphan); err == nil {
			t.Fatal("fast-forward to a non-descendant succeeded; want refusal")
		}
		if got := metaTip(t, r); got != base {
			t.Fatalf("meta = %s, want untouched %s", got, base)
		}
	})

	t.Run("HEAD not on the branch refuses", func(t *testing.T) {
		r, wt, base, target := newCheckedOutBranchFixture(t)
		c := newRealClient(t)
		gitOut(t, wt, "checkout", "-q", "--detach")
		if err := c.FastForwardCheckedOutBranch(ctx, wt, "refs/heads/meta", base, target); err == nil {
			t.Fatal("fast-forward with a detached HEAD succeeded; want refusal")
		}
		if got := metaTip(t, r); got != base {
			t.Fatalf("meta = %s, want untouched %s", got, base)
		}
	})
}

// appendExclude adds a pattern to the repository's shared info/exclude.
func appendExclude(t *testing.T, repoDir, pattern string) {
	t.Helper()
	p := filepath.Join(repoDir, ".git", "info", "exclude")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(pattern + "\n"); err != nil {
		t.Fatal(err)
	}
}
```

Add `"strconv"` to that file's imports (used by `installHooks`).

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -tags integration -count=1 -run '^TestIntegrationRepoFastForwardCheckedOutBranch$' ./internal/gitcli/`
Expected: FAIL to compile — `c.FastForwardCheckedOutBranch undefined`.

- [ ] **Step 3: Extract `emptyHooksDir`**

In `internal/gitcli/hooksoff.go`, move the block of `DisableWorktreeHooks` from "Resolve the common git dir relative to the worktree" through `os.MkdirAll(empty, 0o755)` into:

```go
// emptyHooksDir resolves the worktree's common git dir and returns docket's
// absolute, empty hooks directory under it (<common>/docket/empty-hooks), creating
// it when absent. It is the one place that directory is named: DisableWorktreeHooks
// points a worktree's per-worktree core.hooksPath at it, and the in-place
// fast-forward primitives force it per command with `-c core.hooksPath=`.
func (c *Client) emptyHooksDir(ctx context.Context, op Operation, worktreeDir string) (string, *Failure) {
	commonRes, f := c.run(ctx, runRequest{op: op, dir: worktreeDir, args: []string{"rev-parse", "--git-common-dir"}})
	if f != nil {
		return "", f
	}
	if commonRes.exitCode != 0 {
		return "", newFailure(op, KindCommandFailed, "cannot resolve git common dir: "+stderrExcerpt(commonRes.stderr), nil).withExitCode(commonRes.exitCode)
	}
	commonLines := stdoutLines(commonRes.stdout)
	if len(commonLines) != 1 {
		return "", newFailure(op, KindInvalidOutput, "unexpected git-common-dir output", nil)
	}
	common, err := resolveGitPath(commonLines[0], worktreeDir)
	if err != nil {
		return "", newFailure(op, KindInvalidOutput, "cannot canonicalize git common dir", err)
	}
	empty := filepath.Join(common, "docket", "empty-hooks")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		return "", newFailure(op, KindCommandFailed, "cannot create empty hooks dir", err)
	}
	return empty, nil
}
```

and in `DisableWorktreeHooks` replace the moved block with:

```go
	empty, f := c.emptyHooksDir(ctx, disableHooksOp, worktreeDir)
	if f != nil {
		return f
	}
```

Run `go test -count=1 ./internal/gitcli/ && go test -tags integration -count=1 -run '^TestIntegrationRepo' ./internal/gitcli/` — the existing hooks-off tests must stay green (pure refactor).

- [ ] **Step 4: Implement `FastForwardCheckedOutBranch`**

Create `internal/gitcli/fastforward_inplace.go`:

```go
package gitcli

import (
	"context"
	"path/filepath"
	"strings"
)

const fastForwardCheckedOutOp Operation = "fast-forward-checked-out-branch"

// fastForwardReflogMessage names the in-place fast-forward in the branch reflog.
const fastForwardReflogMessage = "docket: fast-forward"

// FastForwardCheckedOutBranch advances branch — which must be the branch checked out
// at worktreeDir — from expectedTip to target IN PLACE. Nothing is removed: the
// worktree directory, its admin dir (registration, per-worktree config, lock), the
// branch, and its reflog all survive. The branch moves by compare-and-swap from
// expectedTip, so a commit made since the caller observed expectedTip makes it
// refuse with nothing changed. target must descend from expectedTip, judged with
// replace refs and grafts ignored. The tree moves with `read-tree -u -m`, which
// refuses rather than overwrite a local change or an untracked file in the way; on
// that refusal the branch is swapped back. An ignored file at a path target tracks
// is overwritten, Git's normal checkout rule. Every command forces docket's
// empty hooks dir, so no repository hook runs even when the worktree's own hooks-off
// setting is missing. An interruption after the ref swap leaves HEAD at target with
// the old index, which `git status` reports as staged changes (dirty), never clean.
func (c *Client) FastForwardCheckedOutBranch(ctx context.Context, worktreeDir string, branch RefName, expectedTip, target ObjectID) error {
	op := fastForwardCheckedOutOp
	if !filepath.IsAbs(worktreeDir) {
		return newFailure(op, KindInvalidRequest, "worktree path must be absolute", nil)
	}
	if err := validateRefName(branch); err != nil {
		return newFailure(op, KindInvalidRequest, "invalid branch ref", err)
	}
	if _, ok := branchShortName(branch); !ok {
		return newFailure(op, KindInvalidRequest, "branch must be fully qualified refs/heads/<name>", nil)
	}
	if err := validateObjectID(expectedTip); err != nil {
		return newFailure(op, KindInvalidRequest, "invalid expected tip id", err)
	}
	if err := validateObjectID(target); err != nil {
		return newFailure(op, KindInvalidRequest, "invalid target id", err)
	}

	sym, f := c.run(ctx, runRequest{op: op, dir: worktreeDir, args: []string{"symbolic-ref", "--quiet", "HEAD"}})
	if f != nil {
		return f
	}
	if sym.exitCode != 0 || RefName(strings.TrimSpace(string(sym.stdout))) != branch {
		return newFailure(op, KindInvalidRepository, "the worktree does not have the branch checked out", nil)
	}

	descends, err := c.isAncestorIn(ctx, worktreeDir, expectedTip, target, true)
	if err != nil {
		return err
	}
	if !descends {
		return newFailure(op, KindInvalidRepository, "the target does not descend from the expected tip", nil)
	}

	hooks, f := c.emptyHooksDir(ctx, op, worktreeDir)
	if f != nil {
		return f
	}
	noHooks := "core.hooksPath=" + hooks

	cas, f := c.run(ctx, runRequest{op: op, dir: worktreeDir,
		args: []string{"-c", noHooks, "update-ref", "-m", fastForwardReflogMessage, string(branch), string(target), string(expectedTip)}})
	if f != nil {
		return f
	}
	if cas.exitCode != 0 {
		return newFailure(op, KindInvalidRepository, "the branch is no longer at the expected tip; nothing changed: "+stderrExcerpt(cas.stderr), nil).withExitCode(cas.exitCode)
	}

	refresh, f := c.run(ctx, runRequest{op: op, dir: worktreeDir, args: []string{"-c", noHooks, "update-index", "-q", "--refresh"}})
	if f == nil && refresh.exitCode > 1 {
		f = newFailure(op, KindCommandFailed, "update-index --refresh failed: "+stderrExcerpt(refresh.stderr), nil).withExitCode(refresh.exitCode)
	}
	if f == nil {
		rt, rf := c.run(ctx, runRequest{op: op, dir: worktreeDir,
			args: []string{"-c", noHooks, "read-tree", "-u", "-m", string(expectedTip), string(target)}})
		switch {
		case rf != nil:
			f = rf
		case rt.exitCode != 0:
			f = newFailure(op, KindInvalidRepository, "the working tree update was refused: "+stderrExcerpt(rt.stderr), nil).withExitCode(rt.exitCode)
		default:
			return nil
		}
	}

	// The tree update did not complete: swap the branch back so nothing changed.
	back, bf := c.run(ctx, runRequest{op: op, dir: worktreeDir,
		args: []string{"-c", noHooks, "update-ref", "-m", fastForwardReflogMessage + " (rolled back)", string(branch), string(expectedTip), string(target)}})
	if bf != nil {
		return newFailure(op, KindCommandFailed, f.Error()+"; rolling the branch back also failed: "+bf.Error(), nil)
	}
	if back.exitCode != 0 {
		return newFailure(op, KindCommandFailed, f.Error()+"; rolling the branch back also failed: "+stderrExcerpt(back.stderr), nil).withExitCode(back.exitCode)
	}
	return f
}
```

If `*Failure` has no `Error()` method with that exact spelling, use the existing `stderrExcerpt`/message accessor the package already uses for composing failure text (read `internal/gitcli/types.go` for `Failure`'s fields). Keep the rule: a failed rollback reports BOTH the original refusal and the rollback failure.

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test -tags integration -count=1 -run '^TestIntegrationRepoFastForwardCheckedOutBranch$' ./internal/gitcli/ && go test -count=1 ./internal/gitcli/`
Expected: PASS.

- [ ] **Step 6: Mutation-check the keys**

With a backup copy of `fastforward_inplace.go` in `${TMPDIR:-/tmp}`, apply each mutation, re-run the Step 5 integration command with `-count=1`, observe the named subtest go red, and restore from the backup:
1. Drop `string(expectedTip)` from the first `update-ref` (no CAS) → "stale expected tip refuses and changes nothing" red.
2. Drop both `"-c", noHooks,` pairs on `update-ref` and `read-tree` → "repository hooks never run" red.
3. Delete the `update-index --refresh` run (keep `f == nil`) → "stale stat info on a changed path still fast-forwards" red.
4. Replace the rollback with `return f` → "untracked file in the way refuses and rolls back" red.

- [ ] **Step 7: Commit**

```bash
git add internal/gitcli/hooksoff.go internal/gitcli/fastforward_inplace.go internal/gitcli/fastforward_inplace_integration_test.go
git commit -m "feat(gitcli): in-place compare-and-swap fast-forward of a checked-out branch"
```

---

### Task 3: Compare-and-swap advance of an unattached branch

**Files:**
- Create: `internal/gitcli/refadvance.go`
- Create: `internal/gitcli/refadvance_integration_test.go`

**Interfaces:**
- Consumes: `isAncestorIn` (Task 1), `emptyHooksDir` (Task 2), `ListWorktrees`.
- Produces: `func (c *Client) AdvanceBranchChecked(ctx context.Context, repo Repository, branch RefName, expectedTip, target ObjectID) error` — Task 8 calls it before attaching `.docket`. `expectedTip == target` is a CAS-verified no-op.

- [ ] **Step 1: Write the failing tests**

Create `internal/gitcli/refadvance_integration_test.go`:

```go
//go:build integration

package gitcli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// newUnattachedBranchFixture: branch "meta" at base (checked out nowhere) and a
// later commit on main (target) descending from base.
func newUnattachedBranchFixture(t *testing.T) (r *testRepos, base, target ObjectID) {
	t.Helper()
	r = newMainModeRepos(t)
	base = ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))
	gitOut(t, r.Invocation, "branch", "meta", string(base))
	writeWorktreeFile(t, r.Invocation, "target.txt", "target\n")
	gitOut(t, r.Invocation, "add", "--", "target.txt")
	gitOut(t, r.Invocation, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "target")
	target = ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))
	return r, base, target
}

func TestIntegrationRepoAdvanceBranchChecked(t *testing.T) {
	ctx := context.Background()

	t.Run("advances an unattached branch", func(t *testing.T) {
		r, base, target := newUnattachedBranchFixture(t)
		c := newRealClient(t)
		if err := c.AdvanceBranchChecked(ctx, Repository{PrimaryWorktree: r.Invocation}, "refs/heads/meta", base, target); err != nil {
			t.Fatalf("AdvanceBranchChecked: %v", err)
		}
		if got := metaTip(t, r); got != target {
			t.Fatalf("meta = %s, want %s", got, target)
		}
	})

	t.Run("equal tips are a verified no-op", func(t *testing.T) {
		r, base, _ := newUnattachedBranchFixture(t)
		c := newRealClient(t)
		if err := c.AdvanceBranchChecked(ctx, Repository{PrimaryWorktree: r.Invocation}, "refs/heads/meta", base, base); err != nil {
			t.Fatalf("AdvanceBranchChecked equal tips: %v", err)
		}
		if got := metaTip(t, r); got != base {
			t.Fatalf("meta = %s, want %s", got, base)
		}
	})

	t.Run("stale expected tip refuses", func(t *testing.T) {
		r, base, target := newUnattachedBranchFixture(t)
		c := newRealClient(t)
		if err := c.AdvanceBranchChecked(ctx, Repository{PrimaryWorktree: r.Invocation}, "refs/heads/meta", target, target); err == nil {
			t.Fatal("advance from a stale expected tip succeeded; want refusal")
		}
		if got := metaTip(t, r); got != base {
			t.Fatalf("meta = %s, want untouched %s", got, base)
		}
	})

	t.Run("branch checked out in a worktree refuses", func(t *testing.T) {
		r, base, target := newUnattachedBranchFixture(t)
		c := newRealClient(t)
		wt := filepath.Join(testsupport.TempDir(t), "holder")
		gitOut(t, r.Invocation, "worktree", "add", "-q", wt, "meta")
		if err := c.AdvanceBranchChecked(ctx, Repository{PrimaryWorktree: r.Invocation}, "refs/heads/meta", base, target); err == nil {
			t.Fatal("advance of a checked-out branch succeeded; want refusal")
		}
		if got := metaTip(t, r); got != base {
			t.Fatalf("meta = %s, want untouched %s", got, base)
		}
	})

	t.Run("target not descending refuses", func(t *testing.T) {
		r, base, _ := newUnattachedBranchFixture(t)
		c := newRealClient(t)
		tree := gitOut(t, r.Invocation, "rev-parse", "HEAD^{tree}")
		orphan := ObjectID(gitOut(t, r.Invocation, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit-tree", tree, "-m", "orphan"))
		if err := c.AdvanceBranchChecked(ctx, Repository{PrimaryWorktree: r.Invocation}, "refs/heads/meta", base, orphan); err == nil {
			t.Fatal("advance to a non-descendant succeeded; want refusal")
		}
	})

	t.Run("repository hooks never run", func(t *testing.T) {
		r, base, target := newUnattachedBranchFixture(t)
		c := newRealClient(t)
		sentinel := filepath.Join(testsupport.TempDir(t), "hook-ran")
		hooks := installHooks(t, sentinel, 1, "reference-transaction")
		gitOut(t, r.Invocation, "config", "core.hooksPath", hooks)
		if err := c.AdvanceBranchChecked(ctx, Repository{PrimaryWorktree: r.Invocation}, "refs/heads/meta", base, target); err != nil {
			t.Fatalf("AdvanceBranchChecked with a failing reference-transaction hook: %v", err)
		}
		if b, err := os.ReadFile(sentinel); err == nil {
			t.Fatalf("a repository hook ran: %q", b)
		}
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -tags integration -count=1 -run '^TestIntegrationRepoAdvanceBranchChecked$' ./internal/gitcli/`
Expected: FAIL to compile — `c.AdvanceBranchChecked undefined`.

- [ ] **Step 3: Implement**

Create `internal/gitcli/refadvance.go`:

```go
package gitcli

import "context"

const advanceBranchOp Operation = "advance-branch"

// AdvanceBranchChecked moves a branch that no worktree has checked out from
// expectedTip to target by compare-and-swap (`update-ref <branch> <target>
// <expectedTip>`), with repository hooks forced off. target must descend from
// expectedTip, judged with replace refs and grafts ignored. A branch some worktree
// holds is refused (moving it would strand that worktree's index), and a branch no
// longer at expectedTip is refused with nothing changed. expectedTip == target is a
// CAS-verified no-op. branch must be a fully qualified refs/heads/<name>.
func (c *Client) AdvanceBranchChecked(ctx context.Context, repo Repository, branch RefName, expectedTip, target ObjectID) error {
	op := advanceBranchOp
	if err := validateRefName(branch); err != nil {
		return newFailure(op, KindInvalidRequest, "invalid branch ref", err)
	}
	if _, ok := branchShortName(branch); !ok {
		return newFailure(op, KindInvalidRequest, "branch must be fully qualified refs/heads/<name>", nil)
	}
	if err := validateObjectID(expectedTip); err != nil {
		return newFailure(op, KindInvalidRequest, "invalid expected tip id", err)
	}
	if err := validateObjectID(target); err != nil {
		return newFailure(op, KindInvalidRequest, "invalid target id", err)
	}
	descends, err := c.isAncestorIn(ctx, repo.PrimaryWorktree, expectedTip, target, true)
	if err != nil {
		return err
	}
	if !descends {
		return newFailure(op, KindInvalidRepository, "the target does not descend from the expected tip", nil)
	}
	infos, err := c.ListWorktrees(ctx, repo)
	if err != nil {
		return err
	}
	for _, wi := range infos {
		if wi.Branch == branch {
			return newFailure(op, KindInvalidRepository, "the branch is checked out in a worktree", nil)
		}
	}
	hooks, f := c.emptyHooksDir(ctx, op, repo.PrimaryWorktree)
	if f != nil {
		return f
	}
	res, f := c.run(ctx, runRequest{op: op, dir: repo.PrimaryWorktree,
		args: []string{"-c", "core.hooksPath=" + hooks, "update-ref", "-m", fastForwardReflogMessage, string(branch), string(target), string(expectedTip)}})
	if f != nil {
		return f
	}
	if res.exitCode != 0 {
		return newFailure(op, KindInvalidRepository, "the branch is no longer at the expected tip; nothing changed: "+stderrExcerpt(res.stderr), nil).withExitCode(res.exitCode)
	}
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -tags integration -count=1 -run '^TestIntegrationRepoAdvanceBranchChecked$' ./internal/gitcli/`
Expected: PASS.

- [ ] **Step 5: Mutation-check** (backup copy, `-count=1`, restore from backup): drop `string(expectedTip)` from `update-ref` → "stale expected tip refuses" red; delete the worktree-holder loop → "branch checked out in a worktree refuses" red.

- [ ] **Step 6: Commit**

```bash
git add internal/gitcli/refadvance.go internal/gitcli/refadvance_integration_test.go
git commit -m "feat(gitcli): compare-and-swap advance of an unattached branch"
```

---

### Task 4: Relationship-keyed classifier and local-copy health findings (pure)

**Files:**
- Modify: `internal/reposetup/probe.go`, `classify.go`, `health.go`, `healthconditions.go`
- Test: `internal/reposetup/classify_test.go`, `health_test.go`

**Interfaces:**
- Produces (Tasks 5–9 rely on these exact names):
  - `type SyncRelation int` with `SyncUnknown` (zero), `SyncCurrent`, `SyncBehind`, `SyncAhead`, `SyncDiverged`; `func (r SyncRelation) Synchronized() Presence`.
  - `Facts.LocalMetadataSync SyncRelation`.
  - `WorktreeFact.UnfinishedOperation bool`.
  - Reason token and finding code `local-metadata-ahead`; finding code `local-metadata-sync-unverified`.
  - Exact texts (Task 5's parity test compares them with `prepare`'s):
    - ahead message `The local docket branch is ahead of the remote docket branch (it carries commits the remote does not).`, remedy `Reconcile the local docket branch with the remote manually with a human before any repository operation.`
    - dirty message (files) `The .docket metadata worktree has uncommitted or untracked changes.`; dirty message (unfinished operation) `The .docket metadata worktree has an unfinished Git operation (a merge, cherry-pick, revert, rebase, am, or bisect).`

- [ ] **Step 1: Write the failing tests**

In `internal/reposetup/classify_test.go`:
- In `healthyFacts()`, add `LocalMetadataSync: SyncCurrent,` after `LocalMetadata`.
- Replace the case `"ahead metadata worktree -> conflict metadata-worktree-dirty"` (it encodes the old false conflict) with the rows of the new test below, and change the `"diverged local metadata branch..."` case's mutation to `f.LocalMetadata = BranchFact{Presence: PresencePresent, Tip: "other9"}; f.LocalMetadataSync = SyncDiverged; f.DocketWorktree.Synchronized = PresenceAbsent`.
- Add:

```go
// TestClassifyLocalMetadataRelationship pins every row of the spec's local-copy
// table by exact reason set: the relationship, never tip equality, decides.
func TestClassifyLocalMetadataRelationship(t *testing.T) {
	rows := []struct {
		name        string
		mutate      func(*Facts)
		wantState   State
		wantReasons []string
	}{
		{"current clean", func(f *Facts) {}, StateHealthy, nil},
		{"behind clean", func(f *Facts) {
			f.LocalMetadata.Tip = "meta-old"
			f.LocalMetadataSync = SyncBehind
			f.DocketWorktree.Synchronized = PresencePresent
		}, StateHealthy, nil},
		{"behind dirty", func(f *Facts) {
			f.LocalMetadata.Tip = "meta-old"
			f.LocalMetadataSync = SyncBehind
			f.DocketWorktree.Clean = PresenceAbsent
		}, StateConflict, []string{"metadata-worktree-dirty"}},
		{"ahead clean", func(f *Facts) {
			f.LocalMetadata.Tip = "meta-ahead"
			f.LocalMetadataSync = SyncAhead
			f.DocketWorktree.Synchronized = PresenceAbsent
		}, StateConflict, []string{"local-metadata-ahead"}},
		{"ahead dirty", func(f *Facts) {
			f.LocalMetadata.Tip = "meta-ahead"
			f.LocalMetadataSync = SyncAhead
			f.DocketWorktree.Synchronized = PresenceAbsent
			f.DocketWorktree.Clean = PresenceAbsent
		}, StateConflict, []string{"metadata-worktree-dirty", "local-metadata-ahead"}},
		{"diverged clean", func(f *Facts) {
			f.LocalMetadata.Tip = "meta-div"
			f.LocalMetadataSync = SyncDiverged
			f.DocketWorktree.Synchronized = PresenceAbsent
		}, StateConflict, []string{"local-metadata-diverged"}},
		{"unknown relation", func(f *Facts) {
			f.LocalMetadata.Tip = "meta-x"
			f.LocalMetadataSync = SyncUnknown
			f.DocketWorktree.Synchronized = PresenceUnknown
		}, StateConflict, []string{"postconditions-unmet"}},
	}
	for _, row := range rows {
		f := healthyFacts()
		row.mutate(&f)
		got := Classify(f)
		if got.State != row.wantState || !reflect.DeepEqual(got.Reasons, row.wantReasons) {
			t.Errorf("%s: Classify = %+v, want state %q reasons %v", row.name, got, row.wantState, row.wantReasons)
		}
	}
}

func TestSyncRelationSynchronized(t *testing.T) {
	want := map[SyncRelation]Presence{
		SyncUnknown: PresenceUnknown, SyncCurrent: PresencePresent, SyncBehind: PresencePresent,
		SyncAhead: PresenceAbsent, SyncDiverged: PresenceAbsent,
	}
	for r, p := range want {
		if got := r.Synchronized(); got != p {
			t.Errorf("SyncRelation(%d).Synchronized() = %v, want %v", r, got, p)
		}
	}
}
```

(Import `reflect` if `classify_test.go` does not already.)

In `internal/reposetup/health_test.go`:
- Add `"local-metadata-ahead"` to the reason lists in `TestHealthRemedyConflictNeverDestructive` and `TestHealthInitCommandAppearsOnlyInFresh`.
- Replace `TestHealthDirtyMessageNamesObservedAlternatives` (it asserts the deleted "not synchronized" variant) with:

```go
// TestHealthDirtyMessageNamesTheCause: the dirty finding names an unfinished Git
// operation when that is the cause, and never mentions synchronization.
func TestHealthDirtyMessageNamesTheCause(t *testing.T) {
	files := healthyFacts()
	files.DocketWorktree.Clean = PresenceAbsent
	op := healthyFacts()
	op.DocketWorktree.Clean = PresenceAbsent
	op.DocketWorktree.UnfinishedOperation = true
	for name, tc := range map[string]struct {
		f    Facts
		want string
	}{
		"files":     {files, "The .docket metadata worktree has uncommitted or untracked changes."},
		"operation": {op, "The .docket metadata worktree has an unfinished Git operation (a merge, cherry-pick, revert, rebase, am, or bisect)."},
	} {
		var msg string
		for _, fn := range EvaluateHealth(Classify(tc.f), tc.f, nil) {
			if fn.Code == "metadata-worktree-dirty" {
				msg = fn.Message
			}
		}
		if msg != tc.want {
			t.Errorf("%s: dirty message = %q, want %q", name, msg, tc.want)
		}
		if strings.Contains(msg, "synchroniz") {
			t.Errorf("%s: dirty message still mentions synchronization: %q", name, msg)
		}
	}
}

// TestHealthLocalMetadataRelationshipRows pins findings and exit per table row.
func TestHealthLocalMetadataRelationshipRows(t *testing.T) {
	rows := []struct {
		name      string
		mutate    func(*Facts)
		wantCodes []string
		wantExit  int
	}{
		{"behind clean is healthy with no finding", func(f *Facts) {
			f.LocalMetadata.Tip = "meta-old"
			f.LocalMetadataSync = SyncBehind
		}, nil, 0},
		{"behind dirty", func(f *Facts) {
			f.LocalMetadataSync = SyncBehind
			f.DocketWorktree.Clean = PresenceAbsent
		}, []string{"metadata-worktree-dirty"}, 1},
		{"ahead", func(f *Facts) {
			f.LocalMetadataSync = SyncAhead
			f.DocketWorktree.Synchronized = PresenceAbsent
		}, []string{"local-metadata-ahead"}, 1},
		{"ahead dirty", func(f *Facts) {
			f.LocalMetadataSync = SyncAhead
			f.DocketWorktree.Synchronized = PresenceAbsent
			f.DocketWorktree.Clean = PresenceAbsent
		}, []string{"metadata-worktree-dirty", "local-metadata-ahead"}, 1},
		{"diverged", func(f *Facts) {
			f.LocalMetadataSync = SyncDiverged
			f.DocketWorktree.Synchronized = PresenceAbsent
		}, []string{"local-metadata-diverged"}, 1},
		{"unknown with both tips known warns unverified", func(f *Facts) {
			f.LocalMetadata.Tip = "meta-x"
			f.LocalMetadataSync = SyncUnknown
			f.DocketWorktree.Synchronized = PresenceUnknown
		}, []string{"postconditions-unmet", "local-metadata-sync-unverified"}, 1},
	}
	for _, row := range rows {
		f := healthyFacts()
		row.mutate(&f)
		c := Classify(f)
		got := EvaluateHealth(c, f, nil)
		if codes := findingCodes(got); !reflect.DeepEqual(codes, row.wantCodes) {
			t.Errorf("%s: codes = %v, want %v", row.name, codes, row.wantCodes)
		}
		if exit := CheckExit(c, got); exit != row.wantExit {
			t.Errorf("%s: exit = %d, want %d", row.name, exit, row.wantExit)
		}
	}
	// A missing local tip is explained by local-metadata-unverified, never by the
	// sync warning.
	f := healthyFacts()
	f.LocalMetadata = BranchFact{Presence: PresenceUnknown}
	f.LocalMetadataSync = SyncUnknown
	f.DocketWorktree.Synchronized = PresenceUnknown
	if hasCode(EvaluateHealth(Classify(f), f, nil), "local-metadata-sync-unverified") {
		t.Error("sync-unverified fired although the local tip is missing")
	}
}
```

`findingCodes` returns `nil` for an empty slice? Check its definition in `health_test.go`; if it returns `[]string{}`, compare with `len(codes) == 0` for the nil-want row instead of `reflect.DeepEqual`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -count=1 ./internal/reposetup/`
Expected: FAIL to compile — `SyncCurrent`, `LocalMetadataSync`, `UnfinishedOperation` undefined.

- [ ] **Step 3: Implement**

`internal/reposetup/probe.go` — add after `BranchFact`:

```go
// SyncRelation is the ancestry relationship of a local tip to the remote tip it
// tracks. The zero value, SyncUnknown, is safe: an unproven relationship is never
// read as current or behind.
type SyncRelation int

const (
	SyncUnknown  SyncRelation = iota // a tip is missing or an ancestry probe failed
	SyncCurrent                      // local == remote
	SyncBehind                       // local is a strict ancestor of remote
	SyncAhead                        // remote is a strict ancestor of local
	SyncDiverged                     // neither is an ancestor of the other
)

// Synchronized maps a relationship to the docket-worktree-synchronized fact:
// proven current or behind — the local copy holds nothing the remote lacks — is
// Present; proven ahead or diverged is Absent; an unproven relationship is Unknown.
func (r SyncRelation) Synchronized() Presence {
	switch r {
	case SyncCurrent, SyncBehind:
		return PresencePresent
	case SyncAhead, SyncDiverged:
		return PresenceAbsent
	default:
		return PresenceUnknown
	}
}
```

In `WorktreeFact`, change the `Synchronized` comment to `// proven current or behind the remote metadata tip: the local copy holds nothing the remote lacks` and add after `Clean`:

```go
	UnfinishedOperation bool // Clean is Absent because a merge, cherry-pick, revert, rebase, am, or bisect is unfinished
```

In `Facts`, add after `LocalMetadata`:

```go
	LocalMetadataSync     SyncRelation // local docket tip vs remote docket tip; set only by the check/prepare augmentation
```

`internal/reposetup/classify.go` — replace the two conflict blocks for `metadata-worktree-dirty` and `local-metadata-diverged` with:

```go
	if f.RemoteMetadata.Presence == PresencePresent && f.DocketWorktree.Presence == PresencePresent &&
		f.DocketWorktree.Clean == PresenceAbsent {
		conflict = append(conflict, "metadata-worktree-dirty")
	}
	if f.RemoteMetadata.Presence == PresencePresent && f.LocalMetadataSync == SyncAhead {
		conflict = append(conflict, "local-metadata-ahead")
	}
	if f.RemoteMetadata.Presence == PresencePresent && f.LocalMetadataSync == SyncDiverged {
		conflict = append(conflict, "local-metadata-diverged")
	}
```

`internal/reposetup/health.go`:
- `categoryOf`: add `"local-metadata-ahead"` to the `catLocalWorktree` case beside `"local-metadata-diverged"`.
- `findingFor` `"metadata-worktree-dirty"`: replace the `msg` switch with

```go
		msg := "The .docket metadata worktree has uncommitted or untracked changes."
		if f.DocketWorktree.UnfinishedOperation {
			msg = "The .docket metadata worktree has an unfinished Git operation (a merge, cherry-pick, revert, rebase, am, or bisect)."
		}
```

  (Code, severity, ref, and remedy unchanged.)
- `findingFor`: add before `"local-metadata-diverged"`:

```go
	case "local-metadata-ahead":
		return Finding{
			Code:     "local-metadata-ahead",
			Severity: SeverityError,
			Message:  "The local docket branch is ahead of the remote docket branch (it carries commits the remote does not).",
			Remedy:   "Reconcile the local docket branch with the remote manually with a human before any repository operation.",
		}
```

- `reasonExplains`: `"metadata-worktree-dirty": {CondWorktreeClean},` and add `"local-metadata-ahead": {CondWorktreeSynchronized},` (diverged stays `{CondWorktreeSynchronized}`).
- `conditionFinding` `CondWorktreeSynchronized`: replace its body with

```go
	case CondWorktreeSynchronized:
		if !worktreeInspectable(f) {
			return nil
		}
		// Ahead and diverged are explained by their reasons. Unknown with both tips
		// known means the ancestry probe failed: unverified, never guessed behind. A
		// missing tip is explained by the local-metadata findings.
		if f.DocketWorktree.Synchronized != PresenceUnknown || f.LocalMetadata.Tip == "" || f.RemoteMetadata.Tip == "" {
			return nil
		}
		return &Finding{
			Code:     "local-metadata-sync-unverified",
			Severity: SeverityWarning,
			Ref:      ".docket",
			Message:  "Could not determine how the local docket branch relates to the remote docket branch (unverified, not proven diverged).",
			Remedy:   "Re-run `docket repository check` once local Git reads succeed.",
		}
```

- Update the `CondWorktreeClean` comment in `conditionFinding` ("Absent is explained by the metadata-worktree-dirty reason") — still true; leave it.

`internal/reposetup/healthconditions.go`: add a trailing comment on the constant: `CondWorktreeSynchronized  HealthCondition = "docket-worktree-synchronized" // proven current or behind: the local copy holds nothing the remote lacks; Unknown never satisfies it`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -count=1 ./internal/reposetup/ && go build ./...`
Expected: PASS and a clean build. (`internal/app` still compiles: it never read the deleted message variants.)

- [ ] **Step 5: Mutation-check** (backup `classify.go`, `-count=1`, restore from backup): change the ahead guard to append `"local-metadata-diverged"` → `TestClassifyLocalMetadataRelationship` "ahead clean" red; restore `|| f.DocketWorktree.Synchronized == PresenceAbsent` in the dirty guard → "ahead clean" red (dirty companion).

- [ ] **Step 6: Commit**

```bash
git add internal/reposetup/probe.go internal/reposetup/classify.go internal/reposetup/health.go internal/reposetup/healthconditions.go internal/reposetup/classify_test.go internal/reposetup/health_test.go
git commit -m "fix(reposetup): key local-copy health on the sync relationship, not tip equality"
```

---

### Task 5: Check and prepare share one relationship computation; upgrade guide follows

**Files:**
- Create: `internal/app/repository_sync_relation.go`
- Modify: `internal/app/repository_check.go` (`augmentCheckFacts`; delete `synchronizedPresence`)
- Modify: `internal/app/repository_prepare.go` (delete `prepareSync`, its constants, and `prepareSyncRelationship`; `prepareAugment`, `prepareRoute`, `RunRepositoryPrepare`)
- Modify: `internal/app/repository_prepare_test.go`, `internal/app/repository_check_test.go`
- Create: `internal/app/reposynccheck_integration_test.go`
- Create: `tests/test_go_integration_app_reposynccheck.sh`; modify `tests/runtime-budgets.tsv`
- Modify: `docs/release/upgrading-from-bash.md`, `internal/bashupgrade/registry_test.go`

**Interfaces:**
- Consumes: `IsAncestorIgnoringReplacements` (Task 1); `reposetup.SyncRelation`, `Facts.LocalMetadataSync` (Task 4).
- Produces:
  - `func syncRelationship(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, localTip, remoteTip string) reposetup.SyncRelation` — the ONE computation (Task 9 reuses it for the primary checkout).
  - `func applyLocalMetadataSync(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, f *reposetup.Facts)`.
  - `func prepareAugment(ctx context.Context, git *gitcli.Client, f *reposetup.Facts, sc setupContext)` (no return value in this task).
  - `func prepareRoute(f reposetup.Facts) prepareVerdict` — reads `f.LocalMetadataSync`.

- [ ] **Step 1: Write the failing tests**

(a) In `internal/app/repository_prepare_test.go`: add `LocalMetadataSync: reposetup.SyncCurrent,` to `preparableFacts()`. Then apply one mechanical rule to every call site: `prepareRoute(X, prepareSyncY)` becomes "set `X.LocalMetadataSync = reposetup.SyncY` on a local facts value, then `prepareRoute(X)`" (`prepareSyncUnknown`→`reposetup.SyncUnknown`, `prepareSyncCurrent`→`SyncCurrent`, `prepareSyncBehind`→`SyncBehind`, `prepareSyncAhead`→`SyncAhead`, `prepareSyncDiverged`→`SyncDiverged`). Where the call is `prepareRoute(preparableFacts(), …)`, introduce `f := preparableFacts()` first. This preserves the row each test pins (e.g. the local-unknown test still routes with `SyncUnknown`). Find every site with `grep -n "prepareRoute(" internal/app/*_test.go` — do not hand-list them.

(b) Add a parity test to `internal/app/repository_check_test.go`:

```go
// TestLocalMetadataRefusalsMatchPrepare: check's ahead/diverged findings and
// prepare's refusals for the same state carry the same code, message, and remedy,
// so the two commands never describe one state two ways.
func TestLocalMetadataRefusalsMatchPrepare(t *testing.T) {
	for _, tc := range []struct {
		rel    reposetup.SyncRelation
		reason string
	}{
		{reposetup.SyncAhead, "local-metadata-ahead"},
		{reposetup.SyncDiverged, "local-metadata-diverged"},
	} {
		f := preparableFacts()
		f.LocalMetadataSync = tc.rel
		f.DocketWorktree.Synchronized = reposetup.PresenceAbsent
		v := prepareRoute(f)
		if v.finding == nil {
			t.Fatalf("%s: prepare produced no finding", tc.reason)
		}
		got := reposetup.EvaluateHealth(reposetup.Classification{State: reposetup.StateConflict, Reasons: []string{tc.reason}}, reposetup.Facts{}, nil)
		if len(got) != 1 {
			t.Fatalf("%s: check findings = %+v", tc.reason, got)
		}
		if got[0].Code != v.finding.Code || got[0].Message != v.finding.Message || got[0].Remedy != v.finding.Remedy {
			t.Errorf("%s: check %+v != prepare %+v", tc.reason, got[0], *v.finding)
		}
	}
}
```

(c) Create `internal/app/reposynccheck_integration_test.go`:

```go
//go:build integration

package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/reposetup"
)

// --- local-copy relationship scenarios (TestIntegrationRepoSyncCheck shard) --------
//
// Every typed metadata write pushes from a detached worktree and leaves the local
// .docket copy clean and behind. These prove check keys on the real ancestry
// relationship: behind is healthy with no finding; ahead, diverged, and dirty each
// report exactly their own finding.

func checkCodes(res RepositoryCheckResult) map[string]bool {
	got := map[string]bool{}
	for _, f := range res.Findings {
		got[f.Code] = true
	}
	return got
}

func TestIntegrationRepoSyncCheckBehindIsHealthy(t *testing.T) {
	r := newHealthyRepo(t)
	oldTip := r.dotDocketHead(t)
	newTip := r.advanceRemoteDocket(t, "notes/advance.txt", "advanced\n", "advance docket")

	res := r.runCheck(t)
	if res.RepositoryState != string(reposetup.StateHealthy) || len(res.Findings) != 0 || res.CheckExitCode() != 0 {
		t.Fatalf("behind-only .docket: state=%q findings=%+v exit=%d, want healthy, none, 0",
			res.RepositoryState, res.Findings, res.CheckExitCode())
	}
	// Positive evidence the copy really was behind (check is read-only).
	if res.Revisions["local-metadata"] != oldTip || res.Revisions["remote-metadata"] != newTip {
		t.Fatalf("revisions = %v, want local %s behind remote %s", res.Revisions, oldTip, newTip)
	}
	if head := r.dotDocketHead(t); head != oldTip {
		t.Fatalf("check moved .docket from %s to %s", oldTip, head)
	}

	ct := r.runConfigureTests(t)
	if ct.Result == ResultInvalidState {
		t.Fatalf("configure-tests refused a behind-only repository: %s", ct.HumanText())
	}
}

func TestIntegrationRepoSyncCheckAheadReportsAheadOnly(t *testing.T) {
	r := newHealthyRepo(t)
	r.commitInDocket(t, "ahead.txt", "ahead\n", "local-only docket commit")
	res := r.runCheck(t)
	got := checkCodes(res)
	if !got["local-metadata-ahead"] || got["local-metadata-diverged"] || got["metadata-worktree-dirty"] || res.CheckExitCode() != 1 {
		t.Fatalf("ahead: findings=%+v exit=%d, want local-metadata-ahead only, exit 1", res.Findings, res.CheckExitCode())
	}
}

func TestIntegrationRepoSyncCheckDivergedReportsDivergedOnly(t *testing.T) {
	r := newHealthyRepo(t)
	r.advanceRemoteDocket(t, "notes/remote.txt", "remote\n", "remote side")
	r.commitInDocket(t, "local.txt", "local\n", "local side")
	res := r.runCheck(t)
	got := checkCodes(res)
	if !got["local-metadata-diverged"] || got["local-metadata-ahead"] || got["metadata-worktree-dirty"] {
		t.Fatalf("diverged: findings=%+v, want local-metadata-diverged only", res.Findings)
	}
}

func TestIntegrationRepoSyncCheckDirtyBehindReportsDirtyOnly(t *testing.T) {
	r := newHealthyRepo(t)
	r.advanceRemoteDocket(t, "notes/advance.txt", "advanced\n", "advance docket")
	if err := os.WriteFile(filepath.Join(r.invocation, ".docket", "scratch.txt"), []byte("dirt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res := r.runCheck(t)
	got := checkCodes(res)
	if !got["metadata-worktree-dirty"] || got["local-metadata-diverged"] || got["local-metadata-ahead"] {
		t.Fatalf("dirty+behind: findings=%+v, want metadata-worktree-dirty only", res.Findings)
	}
}
```

(`newHealthyRepo`, `runCheck`, `runConfigureTests`, `advanceRemoteDocket`, `commitInDocket`, and `dotDocketHead` already exist in the `integration`-tagged files of package `app`.)

(d) Create `tests/test_go_integration_app_reposynccheck.sh` by copying `tests/test_go_integration_app_repoprepare.sh` and changing only: the header comment (describe the local-copy relationship scenarios for `repository check`, prefix `^TestIntegrationRepoSyncCheck`, no change numbers needed beyond what the sibling headers use) and `SHARD_PREFIX="TestIntegrationRepoSyncCheck"`. Keep `# docket-suite: go` on line 2. Add to `tests/runtime-budgets.tsv`, in sorted position after `tests/test_go_integration_app_reposetup_race.sh`'s neighbors (keep the file's existing order convention): `tests/test_go_integration_app_reposynccheck.sh	30	parallel` (tab-separated).

(e) Upgrade guide test — in `internal/bashupgrade/registry_test.go` replace `runRepairApply` and `runRepoConfirm` with:

```go
// runRepairApply runs the block and checks the guide's claims: the repair is pushed
// to the docket branch, the output names `docket repository prepare`, which the guide
// calls optional, and the local .docket copy is left behind the remote (so the
// healthy check that follows proves a behind-only copy is healthy).
func runRepairApply(t *testing.T, c *upgradeCase, st *runState, body string) {
	t.Helper()
	before := gitRev(t, c, "docket")
	r := runBlock(t, c, st, body)
	after := gitRev(t, c, "docket")
	if after == before {
		t.Fatalf("guide says repair pushes to the docket branch; origin/docket did not move")
	}
	mustContain(t, "repair output", r.Stdout+r.Stderr, "docket repository prepare")
	mustContain(t, "guide", st.Guide, repairPrepareOptionalProse)
	local := strings.TrimSpace(c.mustGit(t, filepath.Join(st.Cwd, ".docket"), "rev-parse", "HEAD"))
	if local == after {
		t.Fatalf("the local .docket copy is already at the repaired tip; the guide's claim that prepare is optional is untested")
	}
}

// runRepoConfirm checks the guide's claim that `docket repository check` alone
// reports healthy right after the repair, with no `prepare` step in between.
func runRepoConfirm(t *testing.T, c *upgradeCase, st *runState, body string) {
	t.Helper()
	if strings.TrimSpace(body) != "docket repository check" {
		t.Fatalf("repo-confirm block must be exactly `docket repository check`:\n%s", body)
	}
	r := runBlock(t, c, st, body)
	mustContain(t, "repository check output", r.Stdout+r.Stderr, "repository check: no-op (healthy)")
}
```

and add the prose constant near `releaseVerifiedProse`:

```go
// repairPrepareOptionalProse is the guide's statement that the prepare step repair
// mentions is optional.
const repairPrepareOptionalProse = "That step is optional: the next docket command brings the folder up to date by itself."
```

(Import `path/filepath` if the file does not already. If `c.mustGit` requires a directory inside the clone that `st.Cwd` is not, use the clone root field the helper already uses — read `upgradeCase` first.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -count=1 ./internal/app/ -run 'TestRepositoryPrepare|TestLocalMetadataRefusalsMatchPrepare'`
Expected: FAIL to compile — `prepareRoute` called with one argument.

- [ ] **Step 3: Implement**

Create `internal/app/repository_sync_relation.go`:

```go
package app

import (
	"context"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/reposetup"
)

// syncRelationship is the ONE ancestry computation behind `repository check`,
// `repository configure-tests`, and `repository prepare`: how a local tip relates to
// the remote tip it tracks. An empty tip or a probe error is the safe SyncUnknown —
// an unproven relationship is never read as current or behind, so it never
// fast-forwards and never reads healthy. The probe ignores replace refs and grafts
// (IsAncestorIgnoringReplacements), so neither can fake "behind" and strand
// local-only commits.
func syncRelationship(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, localTip, remoteTip string) reposetup.SyncRelation {
	if localTip == "" || remoteTip == "" {
		return reposetup.SyncUnknown
	}
	if localTip == remoteTip {
		return reposetup.SyncCurrent
	}
	local, remote := gitcli.ObjectID(localTip), gitcli.ObjectID(remoteTip)
	localBehind, err := git.IsAncestorIgnoringReplacements(ctx, repo, local, remote)
	if err != nil {
		return reposetup.SyncUnknown
	}
	remoteBehind, err := git.IsAncestorIgnoringReplacements(ctx, repo, remote, local)
	if err != nil {
		return reposetup.SyncUnknown
	}
	switch {
	case localBehind:
		return reposetup.SyncBehind
	case remoteBehind:
		return reposetup.SyncAhead
	default:
		return reposetup.SyncDiverged
	}
}

// applyLocalMetadataSync records how the local docket branch relates to the remote
// docket tip, and derives the .docket worktree's synchronized fact from it. Both the
// check augmentation (augmentCheckFacts) and the prepare augmentation (prepareAugment)
// call it, so their facts never disagree. It reads LocalMetadata and RemoteMetadata,
// which the caller has already resolved.
func applyLocalMetadataSync(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, f *reposetup.Facts) {
	if f.LocalMetadata.Presence == reposetup.PresencePresent && f.RemoteMetadata.Presence == reposetup.PresencePresent {
		f.LocalMetadataSync = syncRelationship(ctx, git, repo, f.LocalMetadata.Tip, f.RemoteMetadata.Tip)
	}
	if f.DocketWorktree.Presence == reposetup.PresencePresent {
		f.DocketWorktree.Synchronized = f.LocalMetadataSync.Synchronized()
	}
}
```

`internal/app/repository_check.go`:
- In `augmentCheckFacts`, delete `f.DocketWorktree.Synchronized = synchronizedPresence(f.LocalMetadata, f.RemoteMetadata)` from the worktree block and add, right after that block, `applyLocalMetadataSync(ctx, git, sc.repo, f)`. In the metadata-root comment, replace "the synchronizedPresence comparison's remote side" with "the remote side of syncRelationship".
- Change the worktree block's comment to "clean and hooks disabled; its synchronized fact comes from applyLocalMetadataSync".
- Delete `synchronizedPresence`.

`internal/app/repository_prepare.go`:
- Delete the `prepareSync` type, its five constants, and `prepareSyncRelationship`.
- `prepareAugment`: drop the return value; delete the `Synchronized` line; end with `applyLocalMetadataSync(ctx, git, sc.repo, f)`. Delete the now-unused `metaTip` remote-side usage only if the compiler reports it unused (it still feeds the fetch; keep the fetch logic as is). Update its doc comment: it fills the facts, including `LocalMetadataSync`, and returns nothing.
- `prepareRoute(f reposetup.Facts) prepareVerdict`: replace `switch sync {` with `switch f.LocalMetadataSync {` and the cases `prepareSyncCurrent` → `reposetup.SyncCurrent`, etc. Update its doc comment ("plus the computed sync relationship" → "including the local/remote sync relationship in f.LocalMetadataSync").
- `RunRepositoryPrepare`: replace the `sync := prepareSyncUnknown … sync = prepareAugment(...)` lines with

```go
	if facts.RemoteMetadata.Presence == reposetup.PresencePresent {
		prepareAugment(ctx, d.Git, &facts, sc)
	}

	verdict := prepareRoute(facts)
```

`docs/release/upgrading-from-bash.md` — replace the paragraph after the `repair-apply` block and the `repo-confirm` block with:

```markdown
It then tells you to run `docket repository prepare` to bring your local `.docket` folder up to
date. That step is optional: the next docket command brings the folder up to date by itself.
Check the repository:

<!-- upgrade-step: repo-confirm -->
```sh
docket repository check
```

`docket repository check` now reports `healthy`.
```

(The sentence in the middle must match `repairPrepareOptionalProse` byte for byte, including the line break position only if `mustContain` compares raw text — it does, so keep "That step is optional: the next docket command brings the folder up to date by itself." on ONE line, re-wrapping the surrounding text instead.)

- [ ] **Step 4: Run tests to verify they pass**

Run, in order:
1. `go build ./... && go vet ./internal/app/ ./internal/reposetup/`
2. `go test -count=1 ./internal/app/ -run 'TestRepositoryPrepare|TestLocalMetadataRefusalsMatchPrepare|TestRepositoryCheck'`
3. `bash tests/test_go_integration_app_reposynccheck.sh`
4. `bash tests/test_go_integration_app_repoprepare.sh && bash tests/test_go_integration_app_repocheck.sh`
5. `bash tests/test_go_integration_bashupgrade.sh`
6. `bash tests/test_go_integration_contract.sh` (proves the new shard is wired and every tagged test maps to exactly one runner)
Expected: all PASS. Time step 3 (`time bash …`); if it exceeds 20s solo, record it for Task 10's budget review.

- [ ] **Step 5: Commit**

```bash
git add internal/app/repository_sync_relation.go internal/app/repository_check.go internal/app/repository_prepare.go internal/app/repository_prepare_test.go internal/app/repository_check_test.go internal/app/reposynccheck_integration_test.go tests/test_go_integration_app_reposynccheck.sh tests/runtime-budgets.tsv docs/release/upgrading-from-bash.md internal/bashupgrade/registry_test.go
git commit -m "fix(check): a clean behind-only .docket copy is healthy; one sync computation for check and prepare"
```

---

### Task 6: "Clean" excludes an unfinished Git operation

**Files:**
- Modify: `internal/app/repository_check.go` (`worktreeCleanPresence` → `worktreeCleanState`)
- Modify: `internal/app/repository_prepare.go` (`prepareAugment`, dirty refusal message)
- Modify: `internal/app/repository_check_test.go` (extend the parity test)
- Modify: `internal/app/reposynccheck_integration_test.go`

**Interfaces:**
- Consumes: `gitcli.Client.WorktreeCheckoutState` (existing; `CheckoutState.OperationInProgress` covers MERGE_HEAD, CHERRY_PICK_HEAD, REVERT_HEAD, BISECT_LOG, rebase-merge, rebase-apply — `am` uses rebase-apply); `WorktreeFact.UnfinishedOperation` (Task 4).
- Produces: `func worktreeCleanState(ctx context.Context, git *gitcli.Client, worktreeDir string) (reposetup.Presence, bool)` — the one clean probe for check and prepare; the bool is "an unfinished operation made it not clean".

- [ ] **Step 1: Write the failing tests**

Append to `internal/app/reposynccheck_integration_test.go`:

```go
// TestIntegrationRepoSyncCheckUnfinishedMergeIsDirty: a merge whose index equals HEAD
// (so `git status` lists nothing) is still not clean. Check reports it as dirty and
// names the operation; prepare refuses and keeps MERGE_HEAD.
func TestIntegrationRepoSyncCheckUnfinishedMergeIsDirty(t *testing.T) {
	r := newHealthyRepo(t)
	r.advanceRemoteDocket(t, "notes/advance.txt", "advanced\n", "advance docket")
	dot := filepath.Join(r.invocation, ".docket")
	runGit(t, dot, "fetch", "-q", "origin", "docket")
	runGit(t, dot, "merge", "--no-ff", "--no-commit", "-s", "ours", "FETCH_HEAD")
	if s := runGit(t, dot, "status", "--porcelain", "--untracked-files=all"); s != "" {
		t.Fatalf("premise: the unfinished merge must leave status empty, got:\n%s", s)
	}
	headBefore := r.dotDocketHead(t)
	mergeHead := runGit(t, dot, "rev-parse", "MERGE_HEAD")

	res := r.runCheck(t)
	var msg string
	for _, f := range res.Findings {
		if f.Code == "metadata-worktree-dirty" {
			msg = f.Message
		}
	}
	if !strings.Contains(msg, "unfinished Git operation") {
		t.Fatalf("check findings %+v, want metadata-worktree-dirty naming the unfinished operation", res.Findings)
	}

	pr := runPrepareAt(t, r.invocation)
	if pr.Disposition != PrepareDispositionRefused || !prepareFinding(pr, "metadata-worktree-dirty") {
		t.Fatalf("prepare = %q %+v, want refused metadata-worktree-dirty", pr.Disposition, pr.Findings)
	}
	if got := runGit(t, dot, "rev-parse", "MERGE_HEAD"); got != mergeHead {
		t.Fatalf("MERGE_HEAD = %q after prepare, want kept %q", got, mergeHead)
	}
	if got := r.dotDocketHead(t); got != headBefore {
		t.Fatalf(".docket HEAD moved from %s to %s", headBefore, got)
	}
}
```

(Add `"strings"` to the file's imports.)

Extend `TestLocalMetadataRefusalsMatchPrepare` with a dirty-operation row: `f := preparableFacts(); f.DocketWorktree.Clean = reposetup.PresenceAbsent; f.DocketWorktree.UnfinishedOperation = true; v := prepareRoute(f)`; compare `v.finding.Message` with the `metadata-worktree-dirty` message `reposetup.EvaluateHealth` returns for `reposetup.Facts{DocketWorktree: reposetup.WorktreeFact{UnfinishedOperation: true}}` under reason `metadata-worktree-dirty`, and the same for `UnfinishedOperation: false`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -count=1 ./internal/app/ -run TestLocalMetadataRefusalsMatchPrepare && go test -tags integration -count=1 -run '^TestIntegrationRepoSyncCheckUnfinishedMergeIsDirty$' ./internal/app/`
Expected: the parity row FAILS (prepare's message has no operation variant); the integration test FAILS (check reports healthy-or-other, prepare applies and discards MERGE_HEAD).

- [ ] **Step 3: Implement**

In `internal/app/repository_check.go`, replace `worktreeCleanPresence` with:

```go
// worktreeCleanState is the ONE clean probe for the .docket worktree (check and
// prepare both call it). Clean means no uncommitted or untracked change AND no
// unfinished Git operation (merge, cherry-pick, revert, rebase, am, bisect): a merge
// whose index equals HEAD lists nothing in `git status`, yet a fast-forward would
// silently drop its MERGE_HEAD. The bool reports that an unfinished operation is
// what made it not clean. Any probe error is the safe Unknown.
func worktreeCleanState(ctx context.Context, git *gitcli.Client, worktreeDir string) (reposetup.Presence, bool) {
	st, err := git.WorktreeCheckoutState(ctx, worktreeDir)
	if err != nil {
		return reposetup.PresenceUnknown, false
	}
	changes, err := git.ChangedPaths(ctx, worktreeDir)
	if err != nil {
		return reposetup.PresenceUnknown, false
	}
	if st.OperationInProgress {
		return reposetup.PresenceAbsent, true
	}
	if len(changes) == 0 {
		return reposetup.PresencePresent, false
	}
	return reposetup.PresenceAbsent, false
}
```

and in `augmentCheckFacts`: `f.DocketWorktree.Clean, f.DocketWorktree.UnfinishedOperation = worktreeCleanState(ctx, git, worktreeDir)`.

In `internal/app/repository_prepare.go` `prepareAugment`: the same assignment with `filepath.Join(sc.repo.PrimaryWorktree, docketWorktreeName)`. In `prepareRoute`'s `case reposetup.PresenceAbsent:` of the clean switch, build the message:

```go
		msg := "The .docket metadata worktree has uncommitted or untracked changes."
		if f.DocketWorktree.UnfinishedOperation {
			msg = "The .docket metadata worktree has an unfinished Git operation (a merge, cherry-pick, revert, rebase, am, or bisect)."
		}
```

and use `Message: msg` (remedy unchanged).

A rebase or bisect in `.docket` usually leaves HEAD detached, which `docketWorktreeFact` already reports as foreign; no special case is added for that — `check` may now also report `metadata-worktree-dirty` beside `docket-dir-foreign` there, which is true.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -count=1 ./internal/app/ -run 'TestLocalMetadataRefusalsMatchPrepare|TestRepositoryPrepare' && bash tests/test_go_integration_app_reposynccheck.sh && bash tests/test_go_integration_app_repoprepare.sh && bash tests/test_go_integration_app_repocheck.sh`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/repository_check.go internal/app/repository_prepare.go internal/app/repository_check_test.go internal/app/reposynccheck_integration_test.go
git commit -m "fix(check): an unfinished Git operation in .docket is not clean"
```

---

### Task 7: Prepare fast-forwards `.docket` in place

**Files:**
- Modify: `internal/app/repository_prepare.go` (`prepareVerdict.observedTip`, `prepareApplyVerdict`, `prepareExecute`, `prepareEffectFailure`; delete `prepareFastForwardWorktree`; refresh doc comments that describe the remove/re-add)
- Modify: `internal/app/repository_prepare_test.go`
- Modify: `internal/app/repoprepare_integration_test.go` (header comment only)
- Create: `internal/app/repoinplaceff_integration_test.go`
- Create: `tests/test_go_integration_app_repoinplaceff.sh`; modify `tests/runtime-budgets.tsv`

**Interfaces:**
- Consumes: `FastForwardCheckedOutBranch` (Task 2).
- Produces: `prepareVerdict.observedTip string`; `func prepareApplyVerdict(action prepareAction, targetRev, observedTip string) prepareVerdict` (Task 8 passes the observed tip on the attach rows).

- [ ] **Step 1: Write the failing tests**

(a) In `TestRepositoryPrepareCleanBehindFastForwards` (`repository_prepare_test.go`), add after the target assert:

```go
	if v.observedTip != "m1" {
		t.Errorf("observedTip = %q, want the router's local tip m1 (the compare-and-swap source)", v.observedTip)
	}
```

(b) Create `internal/app/repoinplaceff_integration_test.go`:

```go
//go:build integration

package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/reposetup"
	"github.com/danielhanold/docket/internal/testsupport"
)

// --- in-place fast-forward scenarios (TestIntegrationRepoInPlaceFF shard) ----------
//
// prepare fast-forwards a clean, behind .docket inside the existing worktree:
// nothing removed, compare-and-swap on the router's observed tip, no repository
// hook, refuse rather than discard. Each scenario starts from a check-healthy
// repository whose remote docket branch is one commit ahead of .docket.

func newBehindHealthyRepo(t *testing.T) (r *initRepo, oldTip, newTip string) {
	t.Helper()
	r = newHealthyRepo(t)
	oldTip = r.dotDocketHead(t)
	newTip = r.advanceRemoteDocket(t, "notes/advance.txt", "advanced\n", "advance docket")
	return r, oldTip, newTip
}

func requireFastForwarded(t *testing.T, r *initRepo, newTip string) {
	t.Helper()
	res := runPrepareAt(t, r.invocation)
	if res.Disposition != PrepareDispositionApplied {
		t.Fatalf("prepare = %q (%s), want applied", res.Disposition, res.HumanText())
	}
	if head := r.dotDocketHead(t); head != newTip {
		t.Fatalf(".docket HEAD = %s, want %s", head, newTip)
	}
}

func requireCheckHealthy(t *testing.T, r *initRepo) {
	t.Helper()
	res := r.runCheck(t)
	if res.RepositoryState != string(reposetup.StateHealthy) || res.CheckExitCode() != 0 {
		t.Fatalf("check after prepare: state=%q findings=%+v exit=%d", res.RepositoryState, res.Findings, res.CheckExitCode())
	}
}

func writeHook(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestIntegrationRepoInPlaceFFLockedWorktree(t *testing.T) {
	r, _, newTip := newBehindHealthyRepo(t)
	dot := filepath.Join(r.invocation, ".docket")
	runGit(t, r.invocation, "worktree", "lock", dot)
	requireFastForwarded(t, r, newTip)
	if list := runGit(t, r.invocation, "worktree", "list", "--porcelain"); !strings.Contains(list, "\nlocked") {
		t.Fatalf(".docket lock lost:\n%s", list)
	}
}

// TestIntegrationRepoInPlaceFFRepositoryHooksNeverRun installs failing, file-writing
// post-checkout and post-merge hooks through the repository's shared hooks path.
// A remove/re-add (or a porcelain merge) would run them.
func TestIntegrationRepoInPlaceFFRepositoryHooksNeverRun(t *testing.T) {
	r, _, newTip := newBehindHealthyRepo(t)
	hooks := filepath.Join(testsupport.TempDir(t), "shared-hooks")
	sentinel := filepath.Join(testsupport.TempDir(t), "hook-ran")
	for _, h := range []string{"post-checkout", "post-merge"} {
		writeHook(t, hooks, h, "echo "+h+" >> '"+sentinel+"'\nexit 1\n")
	}
	runGit(t, r.invocation, "config", "core.hooksPath", hooks)
	requireFastForwarded(t, r, newTip)
	if b, err := os.ReadFile(sentinel); err == nil {
		t.Fatalf("a repository hook ran: %q", b)
	}
	requireCheckHealthy(t, r)
}

func TestIntegrationRepoInPlaceFFIgnoredFileAndDirectoryIdentity(t *testing.T) {
	r, _, newTip := newBehindHealthyRepo(t)
	dot := filepath.Join(r.invocation, ".docket")
	exclude := filepath.Join(r.invocation, ".git", "info", "exclude")
	f, err := os.OpenFile(exclude, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("*.local\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	sentinel := filepath.Join(dot, "sentinel.local")
	if err := os.WriteFile(sentinel, []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(dot)
	if err != nil {
		t.Fatal(err)
	}
	requireFastForwarded(t, r, newTip)
	after, err := os.Stat(dot)
	if err != nil || !os.SameFile(before, after) {
		t.Fatalf(".docket directory was removed and re-created (err=%v)", err)
	}
	if b, err := os.ReadFile(sentinel); err != nil || string(b) != "keep\n" {
		t.Fatalf("ignored file lost: %q, %v", b, err)
	}
	requireCheckHealthy(t, r)
}

// TestIntegrationRepoInPlaceFFStaleObservedTipRefuses hands the execute step the
// tip the router saw, after a commit landed in .docket: it must refuse and keep it.
func TestIntegrationRepoInPlaceFFStaleObservedTipRefuses(t *testing.T) {
	r, oldTip, newTip := newBehindHealthyRepo(t)
	runGit(t, r.invocation, "fetch", "-q", "origin", "docket")
	runGit(t, r.invocation, "cat-file", "-e", newTip+"^{commit}") // premise: target object is local
	client := newGitClient(t)
	_, sc, err := GatherSetupFacts(context.Background(), SetupDeps{Git: client, RepoDir: r.invocation}, true)
	if err != nil {
		t.Fatal(err)
	}
	moved := r.commitInDocket(t, "late.txt", "late\n", "commit after routing")

	v := prepareVerdict{disposition: PrepareDispositionApplied, action: prepareActionFastForward, targetRev: newTip, observedTip: oldTip}
	if err := prepareExecute(context.Background(), client, sc, v); err == nil {
		t.Fatal("fast-forward from a stale observed tip succeeded; want refusal")
	}
	if got := runGit(t, r.invocation, "rev-parse", "refs/heads/docket"); got != moved {
		t.Fatalf("local docket = %s, want the late commit %s kept", got, moved)
	}
	if b := mustReadFile(t, filepath.Join(r.invocation, ".docket", "late.txt")); string(b) != "late\n" {
		t.Fatalf("late.txt = %q", b)
	}
}

// TestIntegrationRepoInPlaceFFHooksOffMissing removes the per-worktree hooks-off
// setting; the forced empty hooks dir must still keep the shared hooks silent inside
// .docket. Git runs a hook from the worktree root, so each hook records $PWD, and the
// primary's own fetches (which do run reference-transaction) are told apart from the
// fast-forward. post-index-change is deliberately NOT installed: prepare's own
// `git status` clean probe inside .docket may write the index and would fire it
// before the fast-forward starts, which is not what this test measures.
func TestIntegrationRepoInPlaceFFHooksOffMissing(t *testing.T) {
	r, _, newTip := newBehindHealthyRepo(t)
	dot := filepath.Join(r.invocation, ".docket")
	hooks := filepath.Join(testsupport.TempDir(t), "shared-hooks")
	sentinel := filepath.Join(testsupport.TempDir(t), "hook-ran")
	for _, h := range []string{"reference-transaction", "post-checkout", "post-merge"} {
		writeHook(t, hooks, h, "echo \""+h+" $PWD\" >> '"+sentinel+"'\nexit 0\n")
	}
	runGit(t, r.invocation, "config", "core.hooksPath", hooks)
	runGit(t, dot, "config", "--worktree", "--unset", "core.hooksPath")

	dotReal, err := filepath.EvalSymlinks(dot)
	if err != nil {
		t.Fatal(err)
	}
	inDocket := func(line string) bool {
		return strings.HasSuffix(line, " "+dot) || strings.HasSuffix(line, " "+dotReal)
	}

	// Premise: with hooks-off missing, a ref update inside .docket does run the hook.
	runGit(t, dot, "update-ref", "refs/docket-test/premise", "HEAD")
	runGit(t, dot, "update-ref", "-d", "refs/docket-test/premise")
	premise, _ := os.ReadFile(sentinel)
	ran := false
	for _, line := range strings.Split(strings.TrimSpace(string(premise)), "\n") {
		ran = ran || inDocket(line)
	}
	if !ran {
		t.Fatalf("premise: the shared hook did not run inside .docket:\n%s", premise)
	}
	if err := os.Remove(sentinel); err != nil {
		t.Fatal(err)
	}

	requireFastForwarded(t, r, newTip)
	b, _ := os.ReadFile(sentinel)
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if inDocket(line) {
			t.Fatalf("a repository hook ran inside .docket during prepare: %q", line)
		}
	}
}

func TestIntegrationRepoInPlaceFFIdempotent(t *testing.T) {
	r, _, newTip := newBehindHealthyRepo(t)
	requireFastForwarded(t, r, newTip)
	second := runPrepareAt(t, r.invocation)
	if second.Disposition != PrepareDispositionNoOp {
		t.Fatalf("second prepare = %q (%s), want no-op", second.Disposition, second.HumanText())
	}
	hooksPath := runGit(t, filepath.Join(r.invocation, ".docket"), "config", "--worktree", "core.hooksPath")
	if hooksPath == "" {
		t.Fatal("per-worktree hooks-off setting was removed")
	}
	requireCheckHealthy(t, r)
}

// TestIntegrationRepoInPlaceFFInterruptedReadsDirty simulates an interruption after
// the branch moved but before the tree did: it must read dirty, never clean, and
// prepare must refuse without removing anything.
func TestIntegrationRepoInPlaceFFInterruptedReadsDirty(t *testing.T) {
	r, oldTip, newTip := newBehindHealthyRepo(t)
	dot := filepath.Join(r.invocation, ".docket")
	runGit(t, r.invocation, "fetch", "-q", "origin", "docket")
	runGit(t, dot, "update-ref", "refs/heads/docket", newTip, oldTip)

	res := r.runCheck(t)
	if !checkCodes(res)["metadata-worktree-dirty"] {
		t.Fatalf("interrupted fast-forward: findings %+v, want metadata-worktree-dirty", res.Findings)
	}
	pr := runPrepareAt(t, r.invocation)
	if pr.Disposition != PrepareDispositionRefused {
		t.Fatalf("prepare = %q, want refused", pr.Disposition)
	}
	if _, err := os.Stat(dot); err != nil {
		t.Fatalf(".docket removed: %v", err)
	}
}
```

(c) Create `tests/test_go_integration_app_repoinplaceff.sh` from the `repoprepare` runner with `SHARD_PREFIX="TestIntegrationRepoInPlaceFF"` and a header describing the in-place fast-forward and attach-freshness scenarios. Add `tests/test_go_integration_app_repoinplaceff.sh	30	parallel` to `tests/runtime-budgets.tsv` (sorted position).

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -count=1 ./internal/app/ -run TestRepositoryPrepareCleanBehindFastForwards`
Expected: FAIL to compile — `v.observedTip undefined`.

- [ ] **Step 3: Implement**

In `internal/app/repository_prepare.go`:
- Add to `prepareVerdict`: `observedTip string // the local docket tip the router decided on; every branch move is a compare-and-swap from it ("" when no local branch exists)`.
- Change `prepareApplyVerdict` to `func prepareApplyVerdict(action prepareAction, targetRev, observedTip string) prepareVerdict` setting `observedTip`. Update callers: the attach row passes `""` for now (Task 8 replaces it); the behind row passes `f.LocalMetadata.Tip`.
- In `prepareExecute`, replace the fast-forward case with:

```go
	case prepareActionFastForward:
		return git.FastForwardCheckedOutBranch(ctx, worktreePath, metaRef, gitcli.ObjectID(verdict.observedTip), target)
```

  (Hooks-off is not re-applied: nothing removed it.)
- Delete `prepareFastForwardWorktree`.
- In `prepareEffectFailure`, for the fast-forward stage append to the human text: `" Nothing was removed. If `git -C .docket status` shows changes, finish or reset that update inside .docket with plain Git, then re-run `docket repository prepare`."` (one constant, `prepareFastForwardRecovery`).
- Refresh doc comments that describe the old sequence: the file header ("attaches or fast-forwards it idempotently" stays; add "in place"), `prepareExecute`'s comment, and `RunRepositoryPrepare`'s comment. Grep `remove\|re-add\|re-adding` in the file and fix every hit that describes prepare's fast-forward.

In `internal/app/repoprepare_integration_test.go`, reword the header sentences that call the fast-forward "composed from the worktree primitives" / "remove/delete/re-add" to say it is an in-place compare-and-swap fast-forward whose properties are proven in the `TestIntegrationRepoInPlaceFF` shard.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go build ./... && go test -count=1 ./internal/app/ -run TestRepositoryPrepare && bash tests/test_go_integration_app_repoinplaceff.sh && bash tests/test_go_integration_app_repoprepare.sh && bash tests/test_go_integration_contract.sh`
Expected: PASS. Time the new shard solo; note it for Task 10.

- [ ] **Step 5: Commit**

```bash
git add internal/app/repository_prepare.go internal/app/repository_prepare_test.go internal/app/repoprepare_integration_test.go internal/app/repoinplaceff_integration_test.go tests/test_go_integration_app_repoinplaceff.sh tests/runtime-budgets.tsv
git commit -m "fix(prepare): fast-forward .docket in place with a compare-and-swap and no hooks"
```

---

### Task 8: Prepare's attach path checks freshness

**Files:**
- Modify: `internal/app/repository_prepare.go` (`prepareAugment` returns the holder probe; `prepareRoute(f, heldElsewhere)`; new `prepareAttachRoute`, `prepareAttachFresh`, `prepareAheadFinding`, `prepareDivergedFinding`; `prepareExecute` attach case)
- Modify: `internal/app/repository_prepare_test.go`, `internal/app/repository_check_test.go` (call sites)
- Modify: `internal/app/repoinplaceff_integration_test.go`

**Interfaces:**
- Consumes: `AdvanceBranchChecked` (Task 3); `gitcli.Client.AddBranchWorktree`, `AttachBranchWorktree`, `ListWorktrees`.
- Produces:
  - `func prepareAugment(ctx context.Context, git *gitcli.Client, f *reposetup.Facts, sc setupContext) reposetup.Presence` — returns whether another worktree holds `refs/heads/docket`, probed only when `.docket` is absent and the local branch is present (otherwise `PresenceUnknown`, never consulted).
  - `func prepareRoute(f reposetup.Facts, heldElsewhere reposetup.Presence) prepareVerdict`.
  - `ensureMetadataWorktree` is unchanged and no longer called by prepare.

Routing for an absent `.docket` (spec §5):

| `LocalMetadata` / `LocalMetadataSync` | verdict |
|---|---|
| Absent | apply attach, observed `""` (create at target) |
| Unknown | local-unknown refusal |
| Present, Current | apply attach, observed = local tip |
| Present, Behind, held Absent | apply attach, observed = local tip |
| Present, Behind, held Present | refuse `docket-worktree-ambiguous-registration` |
| Present, Behind, held Unknown | local-unknown refusal |
| Present, Ahead | refuse `local-metadata-ahead` |
| Present, Diverged | refuse `local-metadata-diverged` |
| Present, Unknown | local-unknown refusal |

- [ ] **Step 1: Write the failing tests**

(a) Apply the mechanical call-site rule: every `prepareRoute(f)` becomes `prepareRoute(f, reposetup.PresenceAbsent)` (find them with grep, do not hand-list). Then add to `repository_prepare_test.go`:

```go
// TestRepositoryPrepareAbsentWorktreeRoutesOnFreshness pins every row of the
// absent-.docket table: prepare never attaches a stale, ahead, or diverged branch.
func TestRepositoryPrepareAbsentWorktreeRoutesOnFreshness(t *testing.T) {
	absent := func(local reposetup.BranchFact, rel reposetup.SyncRelation) reposetup.Facts {
		f := preparableFacts()
		f.DocketWorktree = reposetup.WorktreeFact{Presence: reposetup.PresenceAbsent}
		f.LocalMetadata = local
		f.LocalMetadataSync = rel
		return f
	}
	present := func(tip string) reposetup.BranchFact {
		return reposetup.BranchFact{Presence: reposetup.PresencePresent, Tip: tip}
	}
	rows := []struct {
		name         string
		f            reposetup.Facts
		held         reposetup.Presence
		wantDisp     string
		wantAction   prepareAction
		wantObserved string
		wantCode     string
	}{
		{"no local branch creates", absent(reposetup.BranchFact{Presence: reposetup.PresenceAbsent}, reposetup.SyncUnknown), reposetup.PresenceUnknown, PrepareDispositionApplied, prepareActionAttach, "", ""},
		{"local unknown refuses", absent(reposetup.BranchFact{}, reposetup.SyncUnknown), reposetup.PresenceUnknown, PrepareDispositionRefused, prepareActionNone, "", "prepare-local-state-unknown"},
		{"current attaches", absent(present("m9"), reposetup.SyncCurrent), reposetup.PresenceUnknown, PrepareDispositionApplied, prepareActionAttach, "m9", ""},
		{"behind free advances then attaches", absent(present("m1"), reposetup.SyncBehind), reposetup.PresenceAbsent, PrepareDispositionApplied, prepareActionAttach, "m1", ""},
		{"behind held elsewhere refuses", absent(present("m1"), reposetup.SyncBehind), reposetup.PresencePresent, PrepareDispositionRefused, prepareActionNone, "", "docket-worktree-ambiguous-registration"},
		{"behind holder unknown refuses", absent(present("m1"), reposetup.SyncBehind), reposetup.PresenceUnknown, PrepareDispositionRefused, prepareActionNone, "", "prepare-local-state-unknown"},
		{"ahead refuses", absent(present("mA"), reposetup.SyncAhead), reposetup.PresenceAbsent, PrepareDispositionRefused, prepareActionNone, "", "local-metadata-ahead"},
		{"diverged refuses", absent(present("mD"), reposetup.SyncDiverged), reposetup.PresenceAbsent, PrepareDispositionRefused, prepareActionNone, "", "local-metadata-diverged"},
		{"relation unknown refuses", absent(present("mX"), reposetup.SyncUnknown), reposetup.PresenceAbsent, PrepareDispositionRefused, prepareActionNone, "", "prepare-local-state-unknown"},
	}
	for _, row := range rows {
		v := prepareRoute(row.f, row.held)
		if v.disposition != row.wantDisp || v.action != row.wantAction || v.observedTip != row.wantObserved {
			t.Errorf("%s: verdict %q/%v observed %q, want %q/%v observed %q", row.name, v.disposition, v.action, v.observedTip, row.wantDisp, row.wantAction, row.wantObserved)
		}
		if row.wantCode != "" && (v.finding == nil || v.finding.Code != row.wantCode) {
			t.Errorf("%s: finding %+v, want code %s", row.name, v.finding, row.wantCode)
		}
		if row.wantDisp == PrepareDispositionApplied && v.targetRev != "m9" {
			t.Errorf("%s: targetRev = %q, want m9", row.name, v.targetRev)
		}
	}
}
```

Also update `TestRepositoryPrepareHealthyMissingWorktreeAttaches` to assert `v.observedTip == ""`.

(b) Append to `internal/app/repoinplaceff_integration_test.go`:

```go
// removeDocketWorktree deregisters a clean .docket, leaving the local branch.
func removeDocketWorktree(t *testing.T, r *initRepo) {
	t.Helper()
	runGit(t, r.invocation, "worktree", "remove", filepath.Join(r.invocation, ".docket"))
}

func TestIntegrationRepoInPlaceFFAttachBehindAdvancesBranch(t *testing.T) {
	r, _, newTip := newBehindHealthyRepo(t)
	removeDocketWorktree(t, r)
	res := runPrepareAt(t, r.invocation)
	if res.Disposition != PrepareDispositionApplied {
		t.Fatalf("prepare = %q (%s), want applied", res.Disposition, res.HumanText())
	}
	if head := r.dotDocketHead(t); head != newTip {
		t.Fatalf(".docket attached at %s, want the remote tip %s (never the stale local tip)", head, newTip)
	}
	if local := runGit(t, r.invocation, "rev-parse", "refs/heads/docket"); local != newTip {
		t.Fatalf("local docket = %s, want %s", local, newTip)
	}
	requireCheckHealthy(t, r)
}

func TestIntegrationRepoInPlaceFFAttachAheadRefuses(t *testing.T) {
	r := newHealthyRepo(t)
	aheadTip := r.commitInDocket(t, "ahead.txt", "ahead\n", "local-only docket commit")
	removeDocketWorktree(t, r)
	res := runPrepareAt(t, r.invocation)
	if res.Disposition != PrepareDispositionRefused || !prepareFinding(res, "local-metadata-ahead") {
		t.Fatalf("prepare = %q %+v, want refused local-metadata-ahead", res.Disposition, res.Findings)
	}
	if local := runGit(t, r.invocation, "rev-parse", "refs/heads/docket"); local != aheadTip {
		t.Fatalf("local docket = %s, want untouched %s", local, aheadTip)
	}
	if _, err := os.Stat(filepath.Join(r.invocation, ".docket")); !os.IsNotExist(err) {
		t.Fatalf(".docket attached on a refusal (err=%v)", err)
	}
}

func TestIntegrationRepoInPlaceFFAttachDivergedRefuses(t *testing.T) {
	r := newHealthyRepo(t)
	r.advanceRemoteDocket(t, "notes/remote.txt", "remote\n", "remote side")
	localTip := r.commitInDocket(t, "local.txt", "local\n", "local side")
	removeDocketWorktree(t, r)
	res := runPrepareAt(t, r.invocation)
	if res.Disposition != PrepareDispositionRefused || !prepareFinding(res, "local-metadata-diverged") {
		t.Fatalf("prepare = %q %+v, want refused local-metadata-diverged", res.Disposition, res.Findings)
	}
	if local := runGit(t, r.invocation, "rev-parse", "refs/heads/docket"); local != localTip {
		t.Fatalf("local docket = %s, want untouched %s", local, localTip)
	}
	if _, err := os.Stat(filepath.Join(r.invocation, ".docket")); !os.IsNotExist(err) {
		t.Fatalf(".docket attached on a refusal (err=%v)", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -count=1 ./internal/app/ -run TestRepositoryPrepare`
Expected: FAIL to compile — `prepareRoute` takes one argument.

- [ ] **Step 3: Implement**

In `internal/app/repository_prepare.go`:

```go
// prepareAheadFinding and prepareDivergedFinding are prepare's refusals for a local
// docket branch carrying commits the remote lacks; the worktree row and the attach
// row share them. Their texts match check's findings (TestLocalMetadataRefusalsMatchPrepare).
func prepareAheadFinding() reposetup.Finding {
	return reposetup.Finding{
		Code:     string(FCLocalMetadataAhead),
		Severity: reposetup.SeverityError,
		Message:  "The local docket branch is ahead of the remote docket branch (it carries commits the remote does not).",
		Remedy:   "Reconcile the local docket branch with the remote manually with a human before any repository operation.",
	}
}

func prepareDivergedFinding() reposetup.Finding {
	return reposetup.Finding{
		Code:     string(FCLocalMetadataDiverged),
		Severity: reposetup.SeverityError,
		Message:  "The local docket branch has diverged from the remote docket branch.",
		Remedy:   "Reconcile the local and remote docket branches manually with a human before any repository operation.",
	}
}

// prepareAttachRoute decides the absent-.docket row by the local docket branch's
// relationship to the pinned remote tip, so prepare never attaches a stale, ahead,
// or diverged branch as-is and then reports healthy. A behind branch moves to the
// target first (a compare-and-swap from the observed tip); a behind branch another
// worktree holds is a forced, unsupported setup and refuses with nothing touched.
func prepareAttachRoute(f reposetup.Facts, heldElsewhere reposetup.Presence) prepareVerdict {
	target := f.RemoteMetadata.Tip
	switch f.LocalMetadata.Presence {
	case reposetup.PresenceAbsent:
		return prepareApplyVerdict(prepareActionAttach, target, "")
	case reposetup.PresenceUnknown:
		return prepareLocalUnknownVerdict()
	}
	switch f.LocalMetadataSync {
	case reposetup.SyncCurrent:
		return prepareApplyVerdict(prepareActionAttach, target, f.LocalMetadata.Tip)
	case reposetup.SyncBehind:
		switch heldElsewhere {
		case reposetup.PresenceAbsent:
			return prepareApplyVerdict(prepareActionAttach, target, f.LocalMetadata.Tip)
		case reposetup.PresencePresent:
			return prepareRefuseVerdict(reposetup.StateConflict, reposetup.Finding{
				Code:     string(FCDocketWorktreeAmbiguousRegistration),
				Severity: reposetup.SeverityError,
				Ref:      docketWorktreeName,
				Message:  "The local docket branch is checked out in another worktree, so it cannot be moved to the remote tip and attached at .docket.",
				Remedy:   "Inspect the worktree holding the docket branch and resolve it manually with a human, then run `docket repository check`.",
			})
		default:
			return prepareLocalUnknownVerdict()
		}
	case reposetup.SyncAhead:
		return prepareRefuseVerdict(reposetup.StateConflict, prepareAheadFinding())
	case reposetup.SyncDiverged:
		return prepareRefuseVerdict(reposetup.StateConflict, prepareDivergedFinding())
	default:
		return prepareLocalUnknownVerdict()
	}
}
```

- `prepareRoute(f reposetup.Facts, heldElsewhere reposetup.Presence)`: the `case reposetup.PresenceAbsent:` of the worktree switch becomes `return prepareAttachRoute(f, heldElsewhere)`; the worktree row's ahead/diverged cases use `prepareAheadFinding()` / `prepareDivergedFinding()` (no duplicated literals). Document `heldElsewhere` in the doc comment.
- `prepareAugment` returns `reposetup.Presence`: after `applyLocalMetadataSync`, add

```go
	if f.DocketWorktree.Presence != reposetup.PresenceAbsent || f.LocalMetadata.Presence != reposetup.PresencePresent {
		return reposetup.PresenceUnknown // not consulted outside the absent-.docket attach row
	}
	wts, err := git.ListWorktrees(ctx, sc.repo)
	if err != nil {
		return reposetup.PresenceUnknown
	}
	for _, wt := range wts {
		if wt.Branch == metaRef {
			return reposetup.PresencePresent
		}
	}
	return reposetup.PresenceAbsent
```

- `RunRepositoryPrepare`: `held := reposetup.PresenceUnknown; if … { held = prepareAugment(ctx, d.Git, &facts, sc) }; verdict := prepareRoute(facts, held)`.
- `prepareExecute` attach case:

```go
	case prepareActionAttach:
		if err := prepareAttachFresh(ctx, git, sc.repo, worktreePath, metaRef, verdict); err != nil {
			return err
		}
		return git.DisableWorktreeHooks(ctx, worktreePath)
```

```go
// prepareAttachFresh is prepare's own attach; ensureMetadataWorktree stays init's and
// migrate's, with its contract unchanged. With no local docket branch it creates one
// at the target (git refuses if a branch appeared since routing). With one, it moves
// that branch from the observed tip to the target by compare-and-swap (a no-op that
// still verifies the tip when it is already current), then attaches it.
func prepareAttachFresh(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, worktreePath string, metaRef gitcli.RefName, v prepareVerdict) error {
	target := gitcli.ObjectID(v.targetRev)
	if v.observedTip == "" {
		return git.AddBranchWorktree(ctx, repo, worktreePath, metaRef, target)
	}
	if err := git.AdvanceBranchChecked(ctx, repo, metaRef, gitcli.ObjectID(v.observedTip), target); err != nil {
		return err
	}
	return git.AttachBranchWorktree(ctx, repo, worktreePath, metaRef)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go build ./... && go test -count=1 ./internal/app/ -run 'TestRepositoryPrepare|TestLocalMetadataRefusalsMatchPrepare' && bash tests/test_go_integration_app_repoinplaceff.sh && bash tests/test_go_integration_app_repoprepare.sh && bash tests/test_go_integration_app_reposetup.sh && bash tests/test_go_integration_app_repomigration.sh`
Expected: PASS (`reposetup`/`repomigration` prove init's and migrate's attach behavior is unchanged).

- [ ] **Step 5: Commit**

```bash
git add internal/app/repository_prepare.go internal/app/repository_prepare_test.go internal/app/repository_check_test.go internal/app/repoinplaceff_integration_test.go
git commit -m "fix(prepare): attach .docket only at the remote tip; refuse ahead or diverged branches"
```

---

### Task 9: Split the primary-checkout finding by relationship

**Files:**
- Modify: `internal/reposetup/probe.go` (`Facts.PrimaryTipRelation`), `internal/reposetup/health.go` (`conditionFinding` `CondPrimaryAtRemoteTip`)
- Modify: `internal/reposetup/health_test.go`
- Modify: `internal/app/repository_facts.go` (`setupContext.primaryHead`, set in `primaryAtTipPresence`)
- Modify: `internal/app/repository_check.go` (`augmentCheckFacts`)
- Modify: `internal/app/reposynccheck_integration_test.go`

**Interfaces:**
- Consumes: `syncRelationship` (Task 5), `reposetup.SyncRelation` (Task 4).
- Produces: `Facts.PrimaryTipRelation SyncRelation`; finding codes `primary-ahead-of-remote-tip`, `primary-diverged-from-remote-tip`.

- [ ] **Step 1: Write the failing tests**

`internal/reposetup/health_test.go`:

```go
// TestHealthPrimaryTipFindingByRelationship: the primary-not-at-tip finding is chosen
// by relationship; every case stays non-healthy and every remedy works in its state.
func TestHealthPrimaryTipFindingByRelationship(t *testing.T) {
	rows := []struct {
		rel        SyncRelation
		wantCode   string
		remedyHas  string
		remedyLack string
	}{
		{SyncBehind, "primary-behind-remote-tip", "sync-integration", ""},
		{SyncAhead, "primary-ahead-of-remote-tip", "Push the local commits", "sync-integration"},
		{SyncDiverged, "primary-diverged-from-remote-tip", "rebase or merge", "sync-integration"},
		{SyncUnknown, "primary-tip-unverified", "", "sync-integration"},
	}
	for _, row := range rows {
		f := healthyFacts()
		f.PrimaryAtRemoteTip = PresenceAbsent
		f.PrimaryTipRelation = row.rel
		c := Classify(f)
		if c.State == StateHealthy {
			t.Fatalf("%s: a primary off the tip classified healthy", row.wantCode)
		}
		got := EvaluateHealth(c, f, nil)
		var fnd *Finding
		for i := range got {
			if strings.HasPrefix(got[i].Code, "primary-") {
				fnd = &got[i]
			}
		}
		if fnd == nil || fnd.Code != row.wantCode {
			t.Fatalf("relation %d: findings %v, want %s", row.rel, findingCodes(got), row.wantCode)
		}
		if row.remedyHas != "" && !strings.Contains(fnd.Remedy, row.remedyHas) {
			t.Errorf("%s remedy %q lacks %q", row.wantCode, fnd.Remedy, row.remedyHas)
		}
		if row.remedyLack != "" && strings.Contains(fnd.Remedy, row.remedyLack) {
			t.Errorf("%s remedy %q must not name %q (invalid in this state)", row.wantCode, fnd.Remedy, row.remedyLack)
		}
		assertNoDestructiveCommand(t, fnd.Remedy)
		if CheckExit(c, got) != 1 {
			t.Errorf("%s: exit != 1", row.wantCode)
		}
	}
}
```

Append to `internal/app/reposynccheck_integration_test.go`:

```go
func TestIntegrationRepoSyncCheckPrimaryAhead(t *testing.T) {
	r := newHealthyRepo(t)
	writeRepoFile(t, r.invocation, "local-only.txt", "unpushed\n")
	runGit(t, r.invocation, "add", "--", "local-only.txt")
	runGit(t, r.invocation, "commit", "-q", "-m", "unpushed")
	got := checkCodes(r.runCheck(t))
	if !got["primary-ahead-of-remote-tip"] || got["primary-behind-remote-tip"] {
		t.Fatalf("primary with an unpushed commit: findings %v, want primary-ahead-of-remote-tip only", got)
	}
}

func TestIntegrationRepoSyncCheckPrimaryBehind(t *testing.T) {
	r := newHealthyRepo(t)
	runGit(t, r.writer, "fetch", "-q", "origin", "main")
	runGit(t, r.writer, "checkout", "-q", "-B", "main", "origin/main")
	writeRepoFile(t, r.writer, "remote-only.txt", "remote\n")
	runGit(t, r.writer, "add", "--", "remote-only.txt")
	runGit(t, r.writer, "commit", "-q", "-m", "remote advance")
	runGit(t, r.writer, "push", "-q", "origin", "main")
	got := checkCodes(r.runCheck(t))
	if !got["primary-behind-remote-tip"] || got["primary-ahead-of-remote-tip"] {
		t.Fatalf("primary behind the remote: findings %v, want primary-behind-remote-tip only", got)
	}
}
```

(The writer clone may need `gitIdentity(t, r.writer)`; `newInitRepo` already sets it.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test -count=1 ./internal/reposetup/ -run TestHealthPrimaryTipFindingByRelationship`
Expected: FAIL to compile — `PrimaryTipRelation` undefined.

- [ ] **Step 3: Implement**

`internal/reposetup/probe.go` — add after `PrimaryAtRemoteTip` in `Facts`:

```go
	PrimaryTipRelation    SyncRelation // primary HEAD vs pinned integration tip; set only by check's augmentation, only when PrimaryAtRemoteTip is Absent
```

`internal/reposetup/health.go` — replace the `CondPrimaryAtRemoteTip` case:

```go
	case CondPrimaryAtRemoteTip:
		if !integrationResolved(f) {
			return nil
		}
		if f.PrimaryAtRemoteTip == PresenceAbsent {
			switch f.PrimaryTipRelation {
			case SyncBehind:
				return &Finding{
					Code:     "primary-behind-remote-tip",
					Severity: SeverityError,
					Message:  "The primary worktree's HEAD is behind the pinned remote integration tip.",
					Remedy:   "Fetch and fast-forward the integration branch (e.g. `docket repository sync-integration`), preserving local work, then re-run `docket repository check`.",
				}
			case SyncAhead:
				return &Finding{
					Code:     "primary-ahead-of-remote-tip",
					Severity: SeverityError,
					Message:  "The primary worktree's HEAD is ahead of the pinned remote integration tip (it carries commits the remote does not).",
					Remedy:   "Push the local commits (or move them to a branch) yourself, preserving them, then re-run `docket repository check`.",
				}
			case SyncDiverged:
				return &Finding{
					Code:     "primary-diverged-from-remote-tip",
					Severity: SeverityError,
					Message:  "The primary worktree's HEAD has diverged from the pinned remote integration tip.",
					Remedy:   "Reconcile the primary checkout with the remote integration branch yourself (rebase or merge), preserving local work, then re-run `docket repository check`.",
				}
			}
		}
		// Unproven position, or HEAD off the tip with an unproven relationship.
		return &Finding{
			Code:     "primary-tip-unverified",
			Severity: SeverityWarning,
			Message:  "The primary worktree's position against the remote integration tip could not be resolved (unverified).",
			Remedy:   "Re-run `docket repository check` once the local HEAD read succeeds.",
		}
```

(`conditionCategory`'s default already places `primary-*` codes in the local-worktree bucket.)

`internal/app/repository_facts.go`: add `primaryHead string // primary worktree HEAD read by primaryAtTipPresence, else ""` to `setupContext`; in `primaryAtTipPresence`, set `sc.primaryHead = string(wt.Head)` when the primary is found (before comparing). No ancestry probe here.

`internal/app/repository_check.go` `augmentCheckFacts`, after `f.PrimaryOnIntegration = …`:

```go
	// The primary checkout's relationship to the pinned integration tip, computed
	// only where it is reported and only when HEAD is not the tip, through the same
	// syncRelationship the local docket copy uses.
	if f.PrimaryAtRemoteTip == reposetup.PresenceAbsent && sc.sourceRevision != "" {
		f.PrimaryTipRelation = syncRelationship(ctx, git, sc.repo, sc.primaryHead, sc.sourceRevision)
	}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test -count=1 ./internal/reposetup/ ./internal/app/ -run 'TestHealth|TestClassify|TestRepositoryCheck' && bash tests/test_go_integration_app_reposynccheck.sh && bash tests/test_go_integration_app_repocheck.sh`
Expected: PASS. Then run a whole-repo grep for `primary-behind-remote-tip` and confirm every non-history hit (outside `docs/superpowers/plans/`, `docs/results/`) is consistent with the split.

- [ ] **Step 5: Commit**

```bash
git add internal/reposetup/probe.go internal/reposetup/health.go internal/reposetup/health_test.go internal/app/repository_facts.go internal/app/repository_check.go internal/app/reposynccheck_integration_test.go
git commit -m "fix(check): name a primary checkout ahead of or diverged from the remote tip honestly"
```

---

### Task 10: Mutation matrix, self-audit, and budget review

**Files:** none changed unless a mutation stays green (then fix the guard in the owning file and commit it).

- [ ] **Step 1: Mutation matrix (spec §8).** For each row: copy the file to `${TMPDIR:-/tmp}/<name>.bak`, apply the mutation, run the named test with `-count=1`, confirm it FAILS, restore with `cp -f` from the backup, confirm `git status --porcelain` is empty.

| Mutation | File | Must redden |
|---|---|---|
| Put tip equality back: in `syncRelationship`, `return reposetup.SyncDiverged` right after the `localTip == remoteTip` check | `internal/app/repository_sync_relation.go` | `go test -tags integration -count=1 -run '^TestIntegrationRepoSyncCheckBehindIsHealthy$' ./internal/app/` |
| Collapse ahead into diverged: `case remoteBehind: return reposetup.SyncDiverged` | same | `-run '^TestIntegrationRepoSyncCheckAheadReportsAheadOnly$'` |
| Drop the primary split: return the `primary-behind-remote-tip` finding for every Absent case | `internal/reposetup/health.go` | `-run '^TestIntegrationRepoSyncCheckPrimaryAhead$'` and `go test -count=1 -run TestHealthPrimaryTipFindingByRelationship ./internal/reposetup/` |
| Restore remove/re-add: in `prepareExecute`'s fast-forward case, call `git.RemoveWorktreeClean`, `git.DeleteLocalBranchChecked(…, observedTip)`, `git.AddBranchWorktree(…, target)`, `git.DisableWorktreeHooks` | `internal/app/repository_prepare.go` | `-run '^TestIntegrationRepoInPlaceFF(LockedWorktree|RepositoryHooksNeverRun|IgnoredFileAndDirectoryIdentity)$'` (all three) |
| Drop the compare-and-swap: remove `string(expectedTip)` from the first `update-ref` in `FastForwardCheckedOutBranch` | `internal/gitcli/fastforward_inplace.go` | `-run '^TestIntegrationRepoInPlaceFFStaleObservedTipRefuses$' ./internal/app/` |
| Drop the forced hooks path: remove the `"-c", noHooks,` pairs | same | `-run '^TestIntegrationRepoInPlaceFFHooksOffMissing$' ./internal/app/` |
| Drop attach routing: `prepareAttachRoute` returns `prepareApplyVerdict(prepareActionAttach, target, "")` for a present branch and `prepareAttachFresh` calls `ensureMetadataWorktree` | `internal/app/repository_prepare.go` | `-run '^TestIntegrationRepoInPlaceFFAttachBehindAdvancesBranch$'` |
| Drop the unfinished-operation check: ignore `st.OperationInProgress` in `worktreeCleanState` | `internal/app/repository_check.go` | `-run '^TestIntegrationRepoSyncCheckUnfinishedMergeIsDirty$'` |

A mutation that leaves its test green is a defect in the test: fix the test (not the mutation) and re-run the row.

- [ ] **Step 2: Self-audit the change's own additions** (learning `fix-reintroduces-its-own-defect-class`). Grep the branch diff (`git diff origin/main...HEAD -- '*.go'`) for any NEW tip-equality comparison used as a sync predicate (`.Tip ==`, `.Tip !=`, `== remoteTip`) outside `syncRelationship`'s own current check, and for any new `IsAncestor(` call that decides a fast-forward without `IgnoringReplacements`. Each hit is either justified in a comment or fixed.

- [ ] **Step 3: Budget review.** Run each touched shard solo and record its wall time: `time bash tests/test_go_integration_app_reposynccheck.sh`, `…_repoinplaceff.sh`, `…_repoprepare.sh`, `…_repocheck.sh`, `…_gitcli_repo.sh`. If a new shard runs over 20s solo, set its `runtime-budgets.tsv` ceiling to the measured time × 1.5 rounded up to 5 (never above 30); if over 30s, split its scenarios into a second prefix shard instead of raising the ceiling (tests/README.md: never grow past the budget and bump the number). Commit any budget change with its measured numbers in the message.

- [ ] **Step 4: Whole-suite gate.** The build role runs `build.test_command` (`go run ./cmd/docket development test`) once at the end. Read its budget report even when green: act on any `SERIAL CONFIRMED OVER BUDGET:` line for the shards above.
