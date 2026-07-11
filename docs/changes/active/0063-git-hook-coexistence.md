---
id: 63
slug: git-hook-coexistence
title: Coexist with git-hook frameworks — docket bookkeeping commits skip hooks
status: in-progress
priority: high
created: 2026-07-11
updated: 2026-07-11
depends_on: []
related: []
adrs: [1]
spec: docs/superpowers/specs/2026-07-11-git-hook-coexistence-design.md
plan:
results:
trivial: false
auto_groomable:
branch: feat/git-hook-coexistence
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-11-git-hook-coexistence-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-11-git-hook-coexistence-design.md) |
| ADRs | [ADR-0001](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0001-docket-metadata-branch-model.md) |
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
