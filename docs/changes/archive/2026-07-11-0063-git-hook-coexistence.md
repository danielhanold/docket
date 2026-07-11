---
id: 63
slug: git-hook-coexistence
title: Coexist with git-hook frameworks — docket bookkeeping commits skip hooks
status: done
priority: high
created: 2026-07-11
updated: 2026-07-11
depends_on: []
related: []
adrs: [1, 25]
spec: docs/superpowers/specs/2026-07-11-git-hook-coexistence-design.md
plan: docs/superpowers/plans/2026-07-11-git-hook-coexistence.md
results: docs/results/2026-07-11-git-hook-coexistence-results.md
trivial: false
auto_groomable:
branch: feat/git-hook-coexistence
pr: https://github.com/danielhanold/docket/pull/72
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-11-git-hook-coexistence-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-11-git-hook-coexistence-design.md) |
| Plan | [2026-07-11-git-hook-coexistence.md](https://github.com/danielhanold/docket/blob/main/docs/superpowers/plans/2026-07-11-git-hook-coexistence.md) |
| Results | [2026-07-11-git-hook-coexistence-results.md](https://github.com/danielhanold/docket/blob/main/docs/results/2026-07-11-git-hook-coexistence-results.md) |
| PR | [#72](https://github.com/danielhanold/docket/pull/72) |
| ADRs | [ADR-0001](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0001-docket-metadata-branch-model.md), [ADR-0025](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0025-docket-worktrees-disable-git-hooks.md) |
<!-- docket:artifacts:end -->

## Why

Docket is effectively unusable in a repo that uses a git-hook framework (pre-commit.com, husky, lefthook, …) with a `pre-commit` hook — a very common setup. Git hooks are **shared across all worktrees** via the common git dir, and docket **never skips hooks** on any commit (verified: no `--no-verify` / `core.hooksPath` / hook logic anywhere in the codebase). So every one of docket's many machine-generated bookkeeping commits into the `.docket/` worktree — which lives on the **orphan `docket` branch** — fires the repo's shared `pre-commit` shim, which hard-fails because that branch has no `.pre-commit-config.yaml` (`No .pre-commit-config.yaml file was found`, exit 1).

Observed in practice (Cursor + a pre-commit repo): a metadata commit was blocked, and the agent recovered only because it happened to catch the failure and improvise a per-commit env-var workaround. That is fragile and non-deterministic — a helper-script commit, or an autonomous run that doesn't catch it, simply hard-fails. Docket needs to coexist with hook frameworks **systematically**, not by improvisation.

## What changes

Disable git hooks on the **`.docket` metadata worktree**, once, at creation/ensure time: enable git's local `extensions.worktreeConfig` and set a **worktree-scoped `core.hooksPath`** to an empty, docket-owned directory. Every metadata commit — helper-script or agent-driven — then skips hooks **by construction** (nothing to forget), **framework-agnostically**. The main working tree and feature worktrees are untouched, so the team's code-quality hooks still run on real code.

Scope is **metadata bookkeeping only**: docket's own commits on `metadata_branch` (via `.docket`) skip hooks, and so does `terminal-publish`'s doc-publish commit onto the integration branch (a temp worktree, handled per-invocation with `-c core.hooksPath`). Feature-branch **code** commits keep running the team's hooks. The disable is applied at every `.docket` create/ensure site (`docket-status.sh`, `migrate-to-docket.sh`, `docket-config.sh --bootstrap`, the convention Step-0 preamble), idempotently — so it also **self-heals** existing installs — single-sourced through a small new helper (`scripts/disable-worktree-hooks.sh`). Harness-agnostic (Cursor / Claude Code / Codex alike).

Full mechanism, the `extensions.worktreeConfig` safety caveat, the create/ensure sites, and the hermetic hook test are in the linked spec.

## Out of scope

- Feature-branch **code** commits — keep running the team's hooks (deliberate: real code headed to a PR).
- Framework-specific handling (`PRE_COMMIT_ALLOW_NO_CONFIG`, `SKIP=…`, husky/lefthook config) — the mechanism-level disable needs none.
- A configurable per-repo hook policy (`.docket.yml` `hooks:` knob) — YAGNI.
- Any change to which commits docket makes, or to the branch model (ADR-0001) itself.

## Open questions

- Unsafe-`worktreeConfig` degrade path: if a pre-existing common-config `core.worktree`/`core.bare` blocks safe enablement, relocate-and-proceed per git's guidance vs. warn-and-degrade to per-invocation `-c core.hooksPath` on script sites only. Lean relocate-and-proceed; confirm at plan time.
- README placement: standalone "git-hook frameworks" subsection vs. a note in the migration / branch-model section. Cosmetic.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-07-11 — reconciled against origin/main

Verified the design still holds against current code — all core claims confirmed:

- **No hook-skipping exists anywhere** (`git grep -E 'no-verify|hooksPath|PRE_COMMIT_ALLOW_NO_CONFIG'` over `scripts/`, `skills/`, `*.sh` → nil). The systematic gap the change targets is real and unaddressed.
- All named commit/worktree sites still present: `scripts/docket-status.sh` `ensure_and_sync_worktree()` (worktree-add at L44-45), `migrate-to-docket.sh` (worktree-add paths L196/251/260), `scripts/terminal-publish.sh` publish commit (L147, `$GIT -C "$pub" commit`). `scripts/disable-worktree-hooks.sh` does not yet exist (new file, as planned).

Two **plan-level refinements** (scope unchanged, design not invalidated):

1. **Drop `docket-config.sh --bootstrap` as a hook-disable site.** `create_orphan()` (docket-config.sh L53-59) is **worktree-free** — it builds the orphan `docket` via `commit-tree` + push, creating **no `.docket` worktree**. A worktree-scoped `core.hooksPath` has nothing to attach to at bootstrap. The `.docket` worktree is created immediately afterward by the Step-0 preamble / `ensure_and_sync_worktree`, which is a helper site and self-heals — so bootstrap needs no helper call. The worktree-creation helper sites are therefore: `docket-status.sh` `ensure_and_sync_worktree()` and `migrate-to-docket.sh`.
2. **terminal-publish: prefer the worktree-scoped disable on the transient `pub-$T` worktree over a single per-invocation `-c`.** The publish path commits at L147 **and** replays via `rebase --continue` inside the push-retry loop (L150-156); a lone `-c core.hooksPath` on the L147 commit would not cover the rebase replay. Applying the helper (worktree-scoped `core.hooksPath`) to the `pub-$T` worktree right after `worktree add` (L119) covers every commit in it, reuses the single-sourced helper, and is torn down with the worktree. Confirm exact mechanism at plan time.

Open questions carried into planning unchanged: unsafe-`worktreeConfig` degrade path (lean relocate-and-proceed) and README placement (cosmetic).
