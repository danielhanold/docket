---
id: 308
slug: git-adapter-and-authoritative-object-source
title: 'Git adapter and authoritative object source'
status: implemented
priority: critical
type: feat
created: 2026-08-12
updated: 2026-08-15
depends_on: [304]
stacked_on:
related: []
discovered_from: [303]
adrs: [1, 34]
spec: docs/superpowers/specs/2026-08-13-git-adapter-and-authoritative-object-source-design.md
plan: docs/superpowers/plans/2026-08-15-git-adapter-and-authoritative-object-source.md
results: docs/results/2026-08-15-git-adapter-and-authoritative-object-source-results.md
trivial: false
auto_groomable:
branch: feat/git-adapter-and-authoritative-object-source
claimed_at:
pr: 210
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-13-git-adapter-and-authoritative-object-source-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-13-git-adapter-and-authoritative-object-source-design.md) |
| Plan | [2026-08-15-git-adapter-and-authoritative-object-source.md](https://github.com/danielhanold/docket/blob/feat/git-adapter-and-authoritative-object-source/docs/superpowers/plans/2026-08-15-git-adapter-and-authoritative-object-source.md) |
| Results | [2026-08-15-git-adapter-and-authoritative-object-source-results.md](https://github.com/danielhanold/docket/blob/feat/git-adapter-and-authoritative-object-source/docs/results/2026-08-15-git-adapter-and-authoritative-object-source-results.md) |
| PR | 210 |
| ADRs | [ADR-0001](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0001-docket-metadata-branch-model.md), [ADR-0034](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0034-repo-root-anchored-to-main-worktree.md) |
<!-- docket:artifacts:end -->

## Why

Authoritative reads and Git identities must be available without modifying the user's checkout or
letting Git command details leak into domain code.

## What changes

Add a typed `internal/gitcli` adapter that resolves the primary repository from any linked
worktree, executes Git directly under a controlled non-interactive environment, discovers remote
refs, fetches an exact authoritative branch revision, and exposes an immutable revision-bound
object source. Single and batch reads return exact blob bytes, Git modes, and opaque blob IDs while
leaving the invocation checkout untouched. Real temporary-repository tests cover both metadata
topologies and hostile path, environment, failure, and concurrency cases.

## Out of scope

Configuration resolution or metadata-mode selection; Markdown parsing or patching; domain and
repository snapshot assembly; metadata transaction worktrees, commits, leases, pushes, or retries;
status and health presentation; planning mutations; feature workspaces; GitHub and pull requests;
process supervision; agent workflows; finalize and recovery; installation, release, self-hosting,
and cutover behavior owned by changes 0305–0307 and 0309–0318.

## Design decisions

The approved focused design is in the linked spec. Discovery canonicalizes the primary worktree and
Git common directory without treating a linked checkout as the repository root. A targeted fetch
updates only Git's object/ref state, resolves one exact commit, and opens a source that never moves;
a later refresh creates a new source. Tree listings and multi-blob reads use NUL-safe, batch-oriented
plumbing, return blob IDs as entity versions, and never infer configuration or interpret documents.

Git execution is private to the adapter: consumers receive typed operations rather than an
arbitrary command runner. Commands use argument arrays, preserve ordinary authentication support,
remove repository/config redirection from the inherited environment, disable prompting, separate
data from diagnostics, and enforce cancellation and bounded timeouts. Failures have stable kinds
without parsing human stderr. A missing optional path is read data, while an unavailable executable,
repository, remote, ref, or malformed plumbing response is a typed failure.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

- 2026-08-15 — Reconciled against origin/main. Changes 0304–0307 have all merged: `internal/app`,
  `internal/config`, `internal/document`, `internal/domain`, `internal/repository` exist; no
  `internal/gitcli` yet, so this change's scope is intact and unduplicated. Updated one stale spec
  sentence that still called 0307's domain work "pending". ADR-0092/0093 (from 0307) concern
  stacked-base semantics and repository-reference severity — no bearing on this adapter. No scope
  change; no follow-up work surfaced (auto-capture: nothing minted).

## Run halted

**2026-08-15T15:15:19Z** — Autonomous `docket-implement-next` (resumed run) halted during the
Step 6 review fix loop. A human must decide how to resolve the in-flight blocker fix before the run
can continue to a PR.

**What stopped the run.** The review returned 10 findings (1 blocker, 4 important, 5 minor). The
first fix worker — dispatched at `docket-build-premium` to fix the blocker (review #1: caller repo
paths passed to git as pathspecs, not literals, so a leading `:` silently mis-scopes reads),
folding in co-located minors #6 (`GIT_EXEC_PATH`/pathspec-family scrub) and #7 (mutation-green
dedup guard) — **returned a malformed outcome** (no `COMPLETE`/`NEEDS_ESCALATION`/`BLOCKED` schema,
no committed SHA) and **left uncommitted edits in the feature worktree**. Per the docket-build and
fix-loop contracts a malformed return halts; per the convention I must not adopt/commit another
agent's uncommitted files, nor dispatch a replacement worker onto the shared worktree — both are
the human's call.

**Feature-branch git state at halt.**
- Branch `feat/git-adapter-and-authoritative-object-source`, HEAD `593a386c` (unchanged from before
  the fix loop).
- Committed and green: plan + Tasks 1–8 (`64e2e1a0`…`80d58c51`) plus the integration repair
  `593a386c` (grep-portability datum). The full suite (`scripts/run-tests.sh`) was **green at
  `593a386c`** (114/114) — the build-evidence baseline.
- **Uncommitted** working-tree edits (the halted worker's in-flight blocker fix, NOT committed):
  `internal/gitcli/exec.go`, `internal/gitcli/exec_test.go`, `internal/gitcli/harness_test.go`,
  `internal/gitcli/readblobs_test.go`. Inspection shows they appear to implement the intended fix
  (`GIT_LITERAL_PATHSPECS=1` appended to the sanitized controls, `GIT_EXEC_PATH` added to the
  scrub set, strengthened sanitize test, colon-path fixture) but are unverified and uncommitted.

**What a human must decide.**
1. Inspect the uncommitted edits in `.worktrees/git-adapter-and-authoritative-object-source`. Either
   (a) verify them (`go test -count=1 ./internal/gitcli/`, `go vet`, `gofmt`, full
   `scripts/run-tests.sh`) and commit as the blocker fix, or (b) discard them and re-dispatch the
   blocker fix.
2. Then complete the remaining review fixes still owed on this branch: importants #2
   (remote URL reaching `Failure.Detail` — spec forbids; secret-non-disclosure test missing), #3
   (`Failure.ExitCode` declared but never populated), #4 (`cmd.WaitDelay` unset → a network
   timeout/cancel can hang on pipe-holding grandchildren); minors #8 (`ReadBlobs` `ls-tree` missing
   `--full-tree`), #9 (fetch-failure classification probe pays a second full network timeout), #10
   (no concurrency test / no `-race` on the Go gate — `-race` on the shared `test_go_toolchain.sh`
   is a repo-wide change, treat as a merge-time note not an in-branch must-fix).
3. Finding #5 (plan Task 9 produced no commit / budget row not re-measured) is already
   substantively satisfied: Task 9 measured the Go suite serial at **15s** (row `test_go_toolchain.sh`
   is 20s — correct, unchanged) and all four required mutation probes were confirmed RED. This
   evidence belongs in a results file, not a code fix.
4. Re-run the suite gate and open the PR (Step 7), which this halt did not reach.

The change stays `in-progress` with the claim lease refreshed; the worktree is left exactly as it
stands for inspection. Resume by naming change id 308 with the state above.
