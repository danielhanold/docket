<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0364 — Advance the primary in place on migrate via a gitcli clean-fast-forward primitive](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-02-0364-migrate-primary-clean-fast-forward.md)**
<!-- docket:backlink:end -->
# Migrate Primary Clean Fast-Forward Implementation Plan

> **For agentic workers:** REQUIRED BUILD SKILL: Use `docket-build` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Advance an unchanged, clean primary worktree to the verified migrated integration commit in place, while preserving the existing operator remedy for every unsafe or failed local finish.

**Architecture:** Add one `internal/gitcli` worktree operation that validates an exact target OID, refuses a dirty checkout, distinguishes an already-current checkout, and delegates the actual branch move to `git merge --ff-only`. Thread the re-read integration OID through migrate's best-effort local finish; suppress the primary-sync pending item only when the primitive succeeds, while retaining the moved-primary reconcile branch and the exact current fast-forward remedy on every refusal.

**Tech Stack:** Go 1.26, `internal/gitcli`, real-Git integration tests behind the `integration` build tag, Bash shard runners, and the Go-native Docket test runner.

**Spec:** `docs/superpowers/specs/2026-08-28-migrate-primary-clean-fast-forward-design.md`

## Global Constraints

- The primitive must run `git -C <worktree> merge --ff-only <target>` semantics; never use `reset --hard`, auto-stash, auto-commit, or otherwise discard local work.
- A malformed target is `KindInvalidRequest`; a dirty or non-fast-forward checkout is a typed refusal and remains unmodified.
- The target passed by migrate is the exact integration commit re-read after publication, not the metadata tip or a newly resolved local proxy.
- The moved-primary branch remains a `git pull --rebase` remedy and is never offered to the fast-forward primitive.
- Any local advance refusal is non-fatal because the remote migration is already durable; preserve today's fast-forward remedy text exactly.
- No caller other than repository migration changes behavior.
- Extend the existing integration shards; do not add or enlarge a runtime-budget row.

---

## File Structure

- Create `internal/gitcli/fastforward.go` for the exported clean-fast-forward operation and its dedicated `Operation` label.
- Create `internal/gitcli/fastforward_integration_test.go` for real-repository success, no-op, malformed-target, dirty-tree, and divergent-history behavior.
- Modify `internal/app/repository_migrate.go` to thread the verified integration OID into local finish, attempt the primitive only on the not-moved path, conditionally emit the remedy, and omit empty human pending text.
- Modify `internal/app/repomigration_integration_test.go` to prove the healthy happy path and pin dirty/moved degradation without weakening the durable-remote assertions.

### Task 1: Add the clean-fast-forward gitcli primitive

**Files:**

- Create: `internal/gitcli/fastforward.go`
- Create: `internal/gitcli/fastforward_integration_test.go`

**Interfaces:**

- Consumes: `(*Client).worktreeHead(ctx context.Context, op Operation, worktreeDir string) (ObjectID, *Failure)`, `(*Client).run`, `validateObjectID`, `newFailure`, and `stderrExcerpt` from `internal/gitcli`.
- Produces: `func (c *Client) FastForwardWorktree(ctx context.Context, worktree string, target ObjectID) (advanced bool, err error)` and `fastForwardWorktreeOp Operation = "worktree-fast-forward"` for Task 2.

- [ ] **Step 1: Write the real-Git integration test before the API exists**

Create `internal/gitcli/fastforward_integration_test.go` with the integration build tag and one `TestIntegrationRepoFastForwardWorktree` test containing isolated subtests. Use `newMainModeRepos`, `newRealClient`, `writerCommit`, `gitOut`, and `writeWorktreeFile`; every mutation assertion must compare both `HEAD` and worktree bytes.

```go
//go:build integration

package gitcli

import (
    "context"
    "os"
    "path/filepath"
    "testing"
)

func TestIntegrationRepoFastForwardWorktree(t *testing.T) {
    ctx := context.Background()

    t.Run("clean advance and already current", func(t *testing.T) {
        r := newMainModeRepos(t)
        c := newRealClient(t)
        target := r.writerCommit(t, "main", map[string]string{"remote-only.txt": "target\n"})
        gitOut(t, r.Invocation, "fetch", "-q", "origin", "main")

        advanced, err := c.FastForwardWorktree(ctx, r.Invocation, target)
        if err != nil || !advanced {
            t.Fatalf("FastForwardWorktree advance = %v, %v; want true, nil", advanced, err)
        }
        if got := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD")); got != target {
            t.Fatalf("HEAD = %s, want %s", got, target)
        }

        advanced, err = c.FastForwardWorktree(ctx, r.Invocation, target)
        if err != nil || advanced {
            t.Fatalf("FastForwardWorktree already-current = %v, %v; want false, nil", advanced, err)
        }
    })

    t.Run("divergent history is typed and untouched", func(t *testing.T) {
        r := newMainModeRepos(t)
        c := newRealClient(t)
        target := r.writerCommit(t, "main", map[string]string{"remote-only.txt": "target\n"})
        gitOut(t, r.Invocation, "fetch", "-q", "origin", "main")
        writeWorktreeFile(t, r.Invocation, "local-only.txt", "local\n")
        gitOut(t, r.Invocation, "add", "--", "local-only.txt")
        gitOut(t, r.Invocation, "commit", "-q", "-m", "diverge locally")
        before := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))

        advanced, err := c.FastForwardWorktree(ctx, r.Invocation, target)
        f, ok := AsFailure(err)
        if advanced || !ok || f.Operation != fastForwardWorktreeOp || f.Kind != KindCommandFailed {
            t.Fatalf("divergent result = %v, %#v; want typed command-failed", advanced, err)
        }
        if got := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD")); got != before {
            t.Fatalf("refused fast-forward moved HEAD from %s to %s", before, got)
        }
    })

    t.Run("any dirty path is refused and preserved", func(t *testing.T) {
        r := newMainModeRepos(t)
        c := newRealClient(t)
        target := r.writerCommit(t, "main", map[string]string{"remote-only.txt": "target\n"})
        gitOut(t, r.Invocation, "fetch", "-q", "origin", "main")
        dirtyPath := filepath.Join(r.Invocation, "README.md")
        beforeHead := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))
        writeWorktreeFile(t, r.Invocation, "README.md", "uncommitted bytes\n")

        advanced, err := c.FastForwardWorktree(ctx, r.Invocation, target)
        f, ok := AsFailure(err)
        if advanced || !ok || f.Operation != fastForwardWorktreeOp || f.Kind != KindInvalidRepository {
            t.Fatalf("dirty result = %v, %#v; want typed invalid-repository", advanced, err)
        }
        if got := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD")); got != beforeHead {
            t.Fatalf("dirty refusal moved HEAD from %s to %s", beforeHead, got)
        }
        got, readErr := os.ReadFile(dirtyPath)
        if readErr != nil || string(got) != "uncommitted bytes\n" {
            t.Fatalf("dirty bytes after refusal = %q, %v", got, readErr)
        }
    })

    t.Run("malformed target is invalid request", func(t *testing.T) {
        r := newMainModeRepos(t)
        c := newRealClient(t)
        before := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD"))
        advanced, err := c.FastForwardWorktree(ctx, r.Invocation, ObjectID("not-an-oid"))
        f, ok := AsFailure(err)
        if advanced || !ok || f.Operation != fastForwardWorktreeOp || f.Kind != KindInvalidRequest {
            t.Fatalf("malformed result = %v, %#v; want typed invalid-request", advanced, err)
        }
        if got := ObjectID(gitOut(t, r.Invocation, "rev-parse", "HEAD")); got != before {
            t.Fatalf("malformed target moved HEAD from %s to %s", before, got)
        }
    })
}
```

- [ ] **Step 2: Run the gitcli repository shard and verify the new test fails to compile**

Run: `scripts/run-tests.sh --verbose tests/test_go_integration_gitcli_repo.sh`

Expected: FAIL while compiling `internal/gitcli`, with `c.FastForwardWorktree undefined` (and `fastForwardWorktreeOp undefined`).

- [ ] **Step 3: Implement validation, cleanliness refusal, no-op detection, and ff-only execution**

Create `internal/gitcli/fastforward.go`. Validate before executing Git, require the same absolute worktree spelling as the lifecycle operations, use porcelain status including untracked files so a non-overlapping dirty file cannot slip through Git's permissive merge behavior, read `HEAD` through the existing typed helper, and use the dedicated operation on every new failure.

```go
package gitcli

import (
    "context"
    "path/filepath"
)

const fastForwardWorktreeOp Operation = "worktree-fast-forward"

// FastForwardWorktree advances the branch checked out at worktree to target
// only when the checkout is clean and Git can perform a fast-forward. It returns
// false, nil when HEAD already equals target.
func (c *Client) FastForwardWorktree(ctx context.Context, worktree string, target ObjectID) (bool, error) {
    if !filepath.IsAbs(worktree) {
        return false, newFailure(fastForwardWorktreeOp, KindInvalidRequest, "worktree path must be absolute", nil)
    }
    if err := validateObjectID(target); err != nil {
        return false, newFailure(fastForwardWorktreeOp, KindInvalidRequest, "invalid target id", err)
    }

    status, f := c.run(ctx, runRequest{
        op: fastForwardWorktreeOp,
        dir: worktree,
        args: []string{"status", "--porcelain=v1", "-z", "--untracked-files=all"},
    })
    if f != nil {
        return false, f
    }
    if status.exitCode != 0 {
        return false, newFailure(fastForwardWorktreeOp, KindCommandFailed,
            "worktree status failed: "+stderrExcerpt(status.stderr), nil).withExitCode(status.exitCode)
    }
    if len(status.stdout) != 0 {
        return false, newFailure(fastForwardWorktreeOp, KindInvalidRepository, "worktree has uncommitted changes", nil)
    }

    head, f := c.worktreeHead(ctx, fastForwardWorktreeOp, worktree)
    if f != nil {
        return false, f
    }
    if head == target {
        return false, nil
    }

    merged, f := c.run(ctx, runRequest{
        op: fastForwardWorktreeOp,
        dir: worktree,
        args: []string{"merge", "--ff-only", string(target)},
    })
    if f != nil {
        return false, f
    }
    if merged.exitCode != 0 {
        return false, newFailure(fastForwardWorktreeOp, KindCommandFailed,
            "merge --ff-only failed: "+stderrExcerpt(merged.stderr), nil).withExitCode(merged.exitCode)
    }
    return true, nil
}
```

Do not add an `IsAncestor` pre-check: `merge --ff-only` is the mutation-boundary authority, and the package's existing `KindCommandFailed` convention is sufficient for both non-fast-forward and Git-side refusal.

- [ ] **Step 4: Run the gitcli shard and verify every primitive scenario passes**

Run: `scripts/run-tests.sh --verbose tests/test_go_integration_gitcli_repo.sh`

Expected: PASS; `TestIntegrationRepoFastForwardWorktree` is selected by the existing `^TestIntegrationRepo` shard and all four subtests pass.

- [ ] **Step 5: Commit the primitive as an independently reviewable unit**

```bash
git add internal/gitcli/fastforward.go internal/gitcli/fastforward_integration_test.go
git commit -m "feat: add clean worktree fast-forward primitive"
```

### Task 2: Use the verified integration tip during migrate local finish

**Files:**

- Modify: `internal/app/repository_migrate.go`
- Modify: `internal/app/repomigration_integration_test.go`

**Interfaces:**

- Consumes: `(*gitcli.Client).FastForwardWorktree(ctx, primary, integrationTip) (bool, error)` from Task 1, `pruneCommit` from the fresh migration path, and `sc.sourceRevision` as the already re-read remote integration tip on resume-local.
- Produces: `migrateLocalFinish(..., metadataTip, sourceOID, integrationTip gitcli.ObjectID) []string`; `migratePrimarySyncRemedy(..., sourceOID, integrationTip gitcli.ObjectID) string`, where `""` means local primary synchronization completed and any non-empty return is an operator remedy.

- [ ] **Step 1: Add migration integration assertions that distinguish success from fallback**

In `internal/app/repomigration_integration_test.go`, add a happy-path test that captures the invocation clone's pre-migration `HEAD`, runs an authorized migration, and asserts all of the following together:

```go
func TestIntegrationRepoMigrationPrimaryFastForwardsHealthy(t *testing.T) {
    r := newInitRepo(t, legacyDocketYML, cleanLegacyFiles())
    before := runGit(t, r.invocation, "rev-parse", "HEAD")

    res := r.runMigrate(t, MigrateOptions{Authorized: true})
    if res.Result != ResultApplied {
        t.Fatalf("migrate = %q (%s), want applied", res.Result, res.HumanText())
    }
    want := r.originTip(t, "main")
    if want == before {
        t.Fatal("test setup did not publish a descendant integration commit")
    }
    if got := runGit(t, r.invocation, "rev-parse", "HEAD"); got != want {
        t.Errorf("primary HEAD = %s, want migrated integration %s", got, want)
    }
    if res.RepositoryState != string(reposetup.StateHealthy) {
        t.Errorf("RepositoryState = %q, want healthy", res.RepositoryState)
    }
    if len(res.PendingLocal) != 0 {
        t.Errorf("PendingLocal = %v, want no primary-sync remedy", res.PendingLocal)
    }
    if strings.Contains(res.HumanText(), "pending local sync:") {
        t.Errorf("healthy human result carries an empty pending line: %q", res.HumanText())
    }
}
```

Add a dirty-after-publication test using `setupHooks.beforeLocalFinish`. Dirty a tracked file without committing it, then assert the migration remains `applied`, reports `needs-review`, leaves `HEAD` at the captured source, preserves the dirty bytes, and contains exactly the existing fast-forward remedy payload:

```go
primary, err := filepath.EvalSymlinks(r.invocation)
if err != nil {
    t.Fatal(err)
}
wantRemedy := fmt.Sprintf("fast-forward your primary worktree to the migrated integration branch: `git -C %s merge --ff-only origin/main`", primary)
if len(res.PendingLocal) != 1 || res.PendingLocal[0] != wantRemedy {
    t.Fatalf("PendingLocal = %v, want exact fast-forward remedy %q", res.PendingLocal, wantRemedy)
}
```

Strengthen `TestIntegrationRepoMigrationLocalMovedAfterPublish` to assert `RepositoryState == needs-review`, the local-only commit remains `HEAD`, and its pending list contains the exact existing pull-rebase remedy:

```go
primary, err := filepath.EvalSymlinks(r.invocation)
if err != nil {
    t.Fatal(err)
}
wantMoved := fmt.Sprintf("your local main has moved past the migrated tip; reconcile it: `git -C %s pull --rebase origin main`", primary)
```

Retain that test's existing assertions that both durable remote tips survive and a retry performs no remote write.

- [ ] **Step 2: Run the migration shard and verify the clean happy path fails**

Run: `scripts/run-tests.sh --verbose tests/test_go_integration_app_repomigration.sh`

Expected: FAIL in `TestIntegrationRepoMigrationPrimaryFastForwardsHealthy`; the primary remains at its source commit, `PendingLocal` contains the manual fast-forward remedy, and `RepositoryState` is `needs-review`.

- [ ] **Step 3: Thread the integration target through local finish and conditionally append the remedy**

In `migrateExecute`, pass the exact `pruneCommit` that was compared against the re-read `FetchBranch` result:

```go
pendingLocal := migrateLocalFinish(ctx, git, facts, sc, docketRef, metadataTip, sourceOID, pruneCommit)
```

In `migrateResumeLocal`, the gathered `sc.sourceRevision` is already the authoritative remote integration tip, so pass it as both the current phase's source comparison and advance target; this preserves the current moved/reconcile behavior for a primary still behind while allowing an already-current clean primary to return no remedy:

```go
integrationTip := gitcli.ObjectID(sc.sourceRevision)
pendingLocal := migrateLocalFinish(ctx, git, facts, sc, docketRef, metadataTip, integrationTip, integrationTip)
```

Change `migrateLocalFinish` to accept `integrationTip`, call `migratePrimarySyncRemedy`, and append only a non-empty remedy:

```go
if remedy := migratePrimarySyncRemedy(ctx, git, sc, sourceOID, integrationTip); remedy != "" {
    pending = append(pending, remedy)
}
```

Update `migratePrimarySyncRemedy` without changing its current moved detection or either remedy string. The moved path returns before any mutation; the not-moved path attempts the primitive against the supplied verified commit and swallows every error into today's exact remedy:

```go
if moved {
    return fmt.Sprintf("your local %s has moved past the migrated tip; reconcile it: `git -C %s pull --rebase %s %s`", branch, primary, remote, branch)
}
if _, err := git.FastForwardWorktree(ctx, primary, integrationTip); err == nil {
    return ""
}
return fmt.Sprintf("fast-forward your primary worktree to the migrated integration branch: `git -C %s merge --ff-only %s/%s`", primary, remote, branch)
```

Rewrite the comments to state that this helper returns an empty string only after clean local synchronization and that failures remain `pending_local`, never migration errors. Do not log or embed the primitive failure in the remedy: fallback text must remain byte-for-byte stable.

- [ ] **Step 4: Omit empty pending text from healthy human results**

Change `migrateApplied` so the base human result is always present, but the pending line is added only when `pendingLocal` is non-empty:

```go
out.human = fmt.Sprintf("repository migrated: metadata %s, integration %s", metadataTip, integrationTip)
if len(pendingLocal) > 0 {
    out.human += "\npending local sync: " + strings.Join(pendingLocal, "; ")
}
```

This keeps JSON `pending_local` omitted via its existing `omitempty`, aligns human output with the same state predicate, and leaves unrelated pending items untouched.

- [ ] **Step 5: Run focused default and integration coverage**

Run: `go test ./internal/gitcli ./internal/app`

Expected: PASS for the default-tag unit packages.

Run: `scripts/run-tests.sh --verbose tests/test_go_integration_gitcli_repo.sh tests/test_go_integration_app_repomigration.sh`

Expected: PASS; the clean migration is healthy and advances in place, while dirty and moved scenarios preserve local state, exact remedies, and durable remote tips.

- [ ] **Step 6: Run the authoritative whole-suite build gate**

Run: `go run ./cmd/docket development test`

Expected: exit 0 with every test file passing. Read the output for `BUDGET WATCH:`, `PARALLEL-SENSITIVE:`, and `SERIAL CONFIRMED OVER BUDGET:` lines; any authoritative serial breach must be resolved rather than dismissed because the command exited 0.

- [ ] **Step 7: Commit the migrate wiring and regression coverage**

```bash
git add internal/app/repository_migrate.go internal/app/repomigration_integration_test.go
git commit -m "feat: fast-forward primary after repository migration"
```

## Build Handoff

Use `docket-build` to execute Tasks 1 and 2 in order, preserving each task's red-green test cycle and commit boundary. Stop and report rather than widening scope if the verified integration OID cannot be threaded without changing callers outside repository migration.
