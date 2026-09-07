<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0383 — Remove or plumb the dead metadata-fetch diagnostic append in augmentCheckFacts](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-07-0383-remove-dead-metadata-fetch-diagnostic-append-augmentcheckfacts.md)**
<!-- docket:backlink:end -->
# Remove Dead Metadata-Fetch Diagnostic Append Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Delete the one dead `sc.diagnostics = append(...)` statement in `augmentCheckFacts` while preserving every other behavior of the failed- and successful-fetch paths.

**Architecture:** `augmentCheckFacts` (internal/app/repository_check.go) receives `setupContext` by value and returns nothing, so the `metadata-fetch` diagnostic it appends on a failed `FetchBranch` is never read by anyone: the sole consumer of `setupContext.diagnostics` is `prepareNotices` in `internal/app/repository_prepare.go`, which reads the preparation flow's own context, never this value copy. The fix is a single-statement deletion — no plumbing, no signature change, no new output.

**Tech Stack:** Go (`internal/app`), suite via `go run ./cmd/docket development test`.

**Spec:** `docs/superpowers/specs/2026-09-07-remove-dead-metadata-fetch-diagnostic-append-augmentcheckfacts-design.md` (on the `docket` metadata branch)

## Global Constraints

- Behavior-preserving cleanup only: no new diagnostic, log, finding, notice, result value, or exit-code behavior (spec, "Scope and implementation").
- Preserve verbatim: the `FetchBranch` call and its control flow; the `f.MetadataRoot = reposetup.RootUnknown` assignment on fetch failure; the successful-fetch path (fetched-tip assignment and `verifyMetadataOwnership` call); all subsequent checks; all existing function signatures and shared diagnostic types.
- No new test mirroring the deletion, and no source-text guard asserting the statement is gone (spec: "Review the narrow diff directly; do not add a source-text guard merely to assert that one statement was deleted").
- Retain existing `TestIntegrationRepoOwnershipCheckFailedFetchIsUnknown` coverage in `internal/app/repoownership_integration_test.go` and the ownership/classifier tests — do not edit any test file.
- The build-gate suite command is whatever `build.test_command` resolves to (today `go run ./cmd/docket development test`); run the whole suite at the gate, never only spec-named tests. Budget clause lines keep their standing meanings per `tests/README.md`.

## Reconcile evidence (verified at plan time — re-check in Task 1 Step 1)

Liveness was re-proven against this branch's tree (learnings: verify-the-claim, dormant-code-live-mid-branch):

- `augmentCheckFacts(ctx context.Context, git *gitcli.Client, f *reposetup.Facts, sc setupContext)` — `sc` by value, no return; called once, by value, from `RunRepositoryCheck`.
- Within `internal/app/repository_check.go`, `sc.diagnostics` is written exactly once (the append under `if ferr != nil`) and read nowhere.
- The only reader of `setupContext.diagnostics` anywhere in `internal/app` is `prepareNotices` (`internal/app/repository_prepare.go`), fed by the producers in `internal/app/repository_facts.go` — a different context instance than the check's value copy. Change 0403's configuration-error path did not make this append live.

If Task 1's re-check contradicts any of this (the append has a reader, or the function now takes `sc` by pointer / returns diagnostics), STOP: the spec requires revising the design before deleting — return BLOCKED rather than proceeding.

---

### Task 1: Delete the dead append in augmentCheckFacts

**Files:**
- Modify: `internal/app/repository_check.go` (the `ferr != nil` branch inside `augmentCheckFacts`)

**Interfaces:**
- Consumes: nothing from other tasks (sole task).
- Produces: nothing new — `augmentCheckFacts`'s signature and all public check output are unchanged.

- [ ] **Step 1: Re-verify the append is still dead in the current tree**

Run:

```bash
cd /Users/homer/dev/docket/.worktrees/remove-dead-metadata-fetch-diagnostic-append-augmentcheckfacts
grep -n "sc.diagnostics" internal/app/repository_check.go
grep -n "func augmentCheckFacts" internal/app/repository_check.go
grep -rn "\.diagnostics" internal/app --include='*.go' | grep -v _test.go | grep -v repository_facts.go | grep -v repository_check.go
```

Expected: the first grep matches exactly one line — the `metadata-fetch` append; the second shows `sc setupContext` passed by value with no diagnostics return; the third's only `setupContext.diagnostics` reader is the `range sc.diagnostics` loop in `prepareNotices` (`internal/app/repository_prepare.go`). Any other reader of the check-path copy means the append is live — STOP and return BLOCKED per the reconcile-evidence note above.

- [ ] **Step 2: Delete the append statement**

In `internal/app/repository_check.go`, inside `augmentCheckFacts`, change the fetch-failure branch from:

```go
		rev, ferr := git.FetchBranch(ctx, sc.repo, setupRemote(), metaRef)
		if ferr != nil {
			// A fetch error is unknown even if an older object happens to be available
			// locally: never fall back to the ls-remote tip and never prove ownership
			// from a stale object. Unknown, never a false shape.
			f.MetadataRoot = reposetup.RootUnknown
			sc.diagnostics = append(sc.diagnostics, setupDiag{Probe: "metadata-fetch", Err: ferr})
		} else {
```

to:

```go
		rev, ferr := git.FetchBranch(ctx, sc.repo, setupRemote(), metaRef)
		if ferr != nil {
			// A fetch error is unknown even if an older object happens to be available
			// locally: never fall back to the ls-remote tip and never prove ownership
			// from a stale object. Unknown, never a false shape.
			f.MetadataRoot = reposetup.RootUnknown
		} else {
```

Exactly one line is removed. The comment, the `RootUnknown` assignment, and the `else` (successful-fetch) branch are untouched. Do not rename `ferr`, restructure the branch, or touch anything else in the file.

- [ ] **Step 3: Verify the diff is exactly one deleted line and the package compiles**

Run:

```bash
cd /Users/homer/dev/docket/.worktrees/remove-dead-metadata-fetch-diagnostic-append-augmentcheckfacts
git diff --stat
git diff
go build ./...
go vet ./internal/app/
```

Expected: `git diff --stat` shows only `internal/app/repository_check.go | 1 -`; the diff body removes only the `sc.diagnostics = append(...)` line; build and vet succeed. (If `ferr` became unused, the build would fail — it stays used by the `if`; `rev` stays used by the else branch.)

- [ ] **Step 4: Run the retained failed-fetch integration test and the ownership tests, cache-defeated**

Run:

```bash
cd /Users/homer/dev/docket/.worktrees/remove-dead-metadata-fetch-diagnostic-append-augmentcheckfacts
go test ./internal/app/ -run 'TestIntegrationRepoOwnershipCheckFailedFetchIsUnknown' -count=1 -v
go test ./internal/app/ -run 'Ownership' -count=1
```

Expected: PASS. These assert the unknown-ownership fact and rejection of foreign ownership on a failed fetch — the behavior this change must preserve. Note per the spec: the failed-fetch fixture starts from a zero-valued root, so a green run is regression evidence, not a mutation-tested proof of the explicit `RootUnknown` assignment — do not add such a proof; it is out of scope.

- [ ] **Step 5: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/remove-dead-metadata-fetch-diagnostic-append-augmentcheckfacts
git add internal/app/repository_check.go
git commit -m "fix(0383): remove dead metadata-fetch diagnostic append in augmentCheckFacts

augmentCheckFacts receives setupContext by value and returns no
diagnostics; the appended metadata-fetch entry was never read by
RunRepositoryCheck or any other consumer. The failed-fetch behavior
(MetadataRoot = RootUnknown, no stale-object ownership proof) is
carried by the retained facts assignment and is unchanged."
```

---

## Build gate (owned by docket-build, after the task)

Run the full suite through the configured `build.test_command` (today `go run ./cmd/docket development test`), entered from the feature worktree. Expected: green. Handle any `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` line as a screening finding and any `SERIAL CONFIRMED OVER BUDGET:` line as an authoritative breach per `tests/README.md` — nothing else surfaces them. This one-line deletion adds no test wall-clock, so no budget movement is expected.

## Out of scope (from the change file — do not do these)

- New user-visible diagnostics, findings, notices, result values, or exit-code behavior.
- Broader changes to the check facts pipeline or shared diagnostic handling.
- Changes to the ownership verifier, stale-object safety, or repository preparation.
