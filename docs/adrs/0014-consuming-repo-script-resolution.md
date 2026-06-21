---
id: 14
slug: consuming-repo-script-resolution
title: Consuming-repo script resolution via `DOCKET_SCRIPTS_DIR`
status: Accepted
date: 2026-06-21
supersedes: []
reverses: []
relates_to: [12]
change: 34
---

## Context

Every docket skill invokes its deterministic helper scripts (`docket-config.sh`, `render-board.sh`, `archive-change.sh`, `terminal-publish.sh`, `render-adr-index.sh`, `github-mirror.sh`, `board-checks.sh`, etc.) by a bare CWD-relative path `scripts/<name>.sh`. Those scripts exist only in the docket source clone. A skill runs with its CWD set to the *consuming* repo, where `scripts/` is that project's own directory — so every deterministic primitive was unreachable, and skills silently degraded to hand-worked operations, losing determinism, the fail-closed config guard, idempotency, and the scripts' validation. Observed live in markhaus change #43. The skills reach consuming repos via symlink (`link-skills.sh`), but nothing made `scripts/` reachable the same way.

## Decision

Introduce `DOCKET_SCRIPTS_DIR` — the absolute path to the docket clone's `scripts/` directory — and have every skill resolve helpers as `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/<name>.sh`. The binding rules:

- **Env var, NOT vendoring.** Rejected copying or symlinking the scripts into the consuming repo: a copy drifts (scripts are developed in lockstep with the live-symlinked skills), a symlink does not survive a fresh clone. The env var points at the same live clone the skills are symlinked from, so scripts and skills stay version-matched with zero drift.
- **Injection: shell-profile `export` is the PRIMARY path; user-level `~/.claude/settings.json` `env` is reinforcement.** The Bash tool re-initializes each shell from the user profile on every call, so a profile `export` is re-sourced by the main session AND by dispatched subagent shells — via per-call re-sourcing, NOT OS process-tree inheritance (verified unreliable for subagents). `install.sh` writes both bindings (it holds the clone's absolute path); re-running it back-fills already-migrated clones. Shell floor: zsh/bash `export`, fish `set -gx`, POSIX `export` fallback.
- **Fail loud via `:?`.** A missing or incomplete install surfaces the `run docket/install.sh` remedy on stderr at the first helper call and aborts a bare invocation outright, so the executing agent stops and fixes the install instead of silently degrading. Nuance: at the Step-0 `eval "$(...)"` site the `:?` fires inside the command substitution, so the outer eval does not exit non-zero, but the remedy still reaches stderr — which is what the agent sees.
- **`DOCKET_` namespacing constraint.** Every env var docket introduces is `DOCKET_`-namespaced (joins `DOCKET_MODE`, `DOCKET_INTEGRATION_BRANCH`, `DOCKET_HARNESS_ROOT`) to avoid collisions in the user's shared shell.
- **Settings reinforcement is Claude-Code-only.** The harness-agnostic profile `export` is the actual guarantee; the per-harness settings write is belt-and-suspenders for the Claude Code execution context only.
- **Prose-vs-runnable convention + drift-guard.** Skill prose names a script by basename (`render-board.sh`); a runnable invocation uses the resolved form. A test-suite static audit (`tests/test_consuming_repo_scripts.sh`) guards against any bare `scripts/<name>.sh` reference regressing into a skill body. No GitHub Actions CI exists; the test suite is the de-facto gate.

## Consequences

**Enables:** Every deterministic primitive works from any consuming repo with zero per-repo install and a loud failure when the install is missing. Re-running `install.sh` back-fills already-migrated clones.

**Costs:** A machine-level shell-profile write owned by `install.sh`; the settings-`env` reinforcement is Claude-Code-specific; the eval-site fail-loud is loud-but-not-fail-stop (mitigated by the agent-as-executor seeing the stderr remedy on every subagent shell initialization).

**Relates to ADR-0012** (the docket-status script-vs-model boundary): the scripts are the deterministic layer the model defers to, and this change restores that layer's reachability from consuming repos.
