# stack-base.sh — the effective base branch for one change

## Purpose

Answers the one question every branch-cutting and rebasing step in docket needs: **what branch is
this change built on?** A change that declares `stacked_on: <parent id>` is built on its parent's
unmerged feature branch; every other change is built on the integration branch.

Consumed by `docket-implement-next` (which branch to cut from, and which base the PR opens against)
and by `docket-finalize-change` (which branch to rebase onto before merging). Both invoke it
**unconditionally** — a non-stacked change resolves to the integration branch at exit 0, so no
caller needs to know in advance whether a change is stacked.

Pure read. No writes, no commits, no network. The resolver itself lives in
`scripts/lib/docket-stack.sh` as `stack_effective_base`; this script is the CLI over it.

## Usage

```
stack-base.sh \
  --changes-dir DIR \
  --id N \
  --integration-branch BR \
  [--remote R]
```

- `--changes-dir` — the docket changes directory: the parent of `active/` and `archive/`
  (required). Both are searched, `active/` first.
- `--id` — the change id, padded or bare (required). `0298` and `298` are the same change; the id is
  canonicalized with `10#` at the argument boundary.
- `--integration-branch` — the repo's integration branch, `main` or `develop` (required). It is a
  parameter, never a literal: it is the answer for an unstacked change and the terminus of every
  upward walk.
- `--remote` — the remote whose refs decide whether a parent's branch has actually been pushed
  (default `origin`).

**Mock seam:** `GIT="${GIT:-git}"` — tests substitute a stub.

The whole flag set is validated before any work runs, so a caller fixing one usage error is not sent
back for the next one a call later.

## Behavior

The resolution walks the `stacked_on` chain upward from the change, applying spec §3's four rules:

1. **A live parent whose branch is pushed is the base.** The parent's `branch:` is printed.
2. **A parent that has already merged resolves upward.** A `done` parent, or a `stacked-merged`
   parent whose branch is gone, contributes no branch of its own, so the answer is whatever *its*
   base resolves to — recursively, until the walk reaches an unstacked ancestor and lands on the
   integration branch.
3. **A `killed` parent stops the walk** at exit 3.
4. **Anything else is an invalid resolution** at exit 4: a missing parent, a cycle, or a parent
   whose `branch:` has no ref on the remote.

Exactly one line is printed on stdout on exit 0 — the branch name, nothing else — so a caller can
use `$(stack-base.sh …)` directly. Exits 2, 3 and 4 print a diagnostic on **stderr** and nothing on
stdout.

## Exit codes

| Code | Meaning | What it obliges the caller to do |
|------|---------|----------------------------------|
| 0 | Resolved. The base branch is on stdout. | Cut from, open the PR against, or rebase onto that branch. |
| 2 | Usage error: a missing or unknown flag, a non-numeric `--id`, or a `--changes-dir` that does not exist. | Fix the invocation. |
| 3 | The chain reaches a **killed** parent. | Stop and surface it to a human. Rescoping a change off a killed parent is a **scoping decision**, never an automatic fallback to the integration branch — the child's diff was written against work that was abandoned, and silently rebasing it onto the integration branch produces a branch nobody designed. |
| 4 | **Invalid resolution**: missing parent, cycle, or a parent branch with no remote ref. | Treat as a **data repair**: fix `stacked_on:`/`branch:`, or push the parent's branch. Never fall back to the integration branch — see the invariant below. |

The two failure codes are distinct because the remedies are: 3 is a human scoping decision, 4 is a
data repair. `scripts/board-checks.sh` reports them as the separate `stack-parent-killed` and
`stack-invalid` health checks for the same reason.

## Invariants

- **Rule 1 requires the remote ref to exist, not merely a populated `branch:`.** `branch:` is
  stamped into the manifest at **claim** time, but the branch is not pushed until the PR step. So an
  `in-progress` parent routinely carries a valid-looking `branch:` with nothing behind it. Basing a
  child on that name would silently produce a branch cut from the integration branch while every
  surface reports it as stacked — so the unpushed case is exit 4, a sequencing problem a human
  resolves, and never a quiet fallback.
- **Never falls back to the integration branch on a failure.** Exits 3 and 4 print nothing on
  stdout. A fallback would turn "this stack is broken" into "this stack is fine", which is exactly
  the failure the exit codes exist to prevent.
- **The walk terminates.** `stack_chain` is run first and refuses a cycle or a missing parent with
  exit 4 before any recursion begins, so the upward walk cannot loop.
- **`stacked_on:` and `branch:` are read with the anchored `fm_field`; `status:` with `field`.**
  The first two are optional keys, and an unanchored read of an absent key falls through to body
  prose — ordinary content in a repo whose subject matter *is* these field names. `status:` is
  guaranteed by the change template. This is the selection rule in
  `scripts/lib/docket-frontmatter.sh`, censused by `tests/test_frontmatter_read_shapes.sh`.
- **Ids are canonicalized with `10#` at every boundary.** Docket displays zero-padded 4-digit ids,
  and bash reads a leading `0` as an octal prefix: `0237` would silently become 159 and `0008` would
  not parse at all.
- **Read-only.** The only external call is `git -C <changes dir> show-ref --verify --quiet`, through
  the `GIT` seam. It is addressed at `--changes-dir`'s repo, never the caller's cwd: this CLI is
  invoked from wherever its dispatcher left the shell, and a lookup against the wrong repo finds no
  ref — which rule 1 cannot tell apart from "the parent's branch was never pushed", so it would
  refuse a perfectly valid base at exit `4`.
