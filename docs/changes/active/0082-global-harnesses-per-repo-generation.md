---
id: 82
slug: global-harnesses-per-repo-generation
title: Global agent_harnesses doesn't reach per-repo generation — silent no-op
status: proposed
priority: low
created: 2026-07-15
updated: 2026-07-15
depends_on: []
related: [77, 78, 51]
adrs: [36, 19, 20]
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
type: fix
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| ADRs | [ADR-0036](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0036-codex-agents-md-dispatch-block-committed-machine-neutral.md), [ADR-0019](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0019-global-config-fence-classification.md), [ADR-0020](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0020-generated-agent-artifacts-machine-local.md) |
<!-- docket:artifacts:end -->

## Why

Setting `agent_harnesses: [claude, codex]` in the **global** config
(`~/.config/docket/config.yml`) and then running `sync-agents.sh` in a repo generates
**nothing** per-repo — no `.codex/agents/*.toml`, no committed `AGENTS.md` dispatch block —
and prints no explanation. The user must re-declare `agent_harnesses` in the repo's
`.docket.yml` (or `.docket.local.yml`) to trigger per-repo generation. Discovered testing
change 77 (codex harness) against `~/dev/dotfiles-backup`.

This is currently *by design*: global `agent_harnesses` scopes the **user-level** pass only
(`~/.claude/agents`, `~/.codex/agents`); the **per-repo** pass is gated by
`per_repo_opted_in()` and driven by `resolve_agent_harnesses()`, both of which read only the
repo's own `.docket.yml`/`.docket.local.yml`. Change 0050 scoped it that way deliberately:
the committed `AGENTS.md` block (ADR-0036) must be deterministic from the repo's *own*
committed config, or a collaborator/CI without the same global config fails
`sync-agents.sh --check`. The constraint is real for team repos — but for a solo dev the
silent no-op is a confusing dead end, and re-declaring per-repo is friction.

## What changes

To be settled at brainstorm. Candidate directions (not mutually exclusive), captured so the
design conversation starts informed:

1. **Kill the silent no-op (ship regardless).** When global `agent_harnesses` is set but the
   current repo isn't opted in, emit one advisory pointing at the fix (add `agent_harnesses:`
   to `.docket.local.yml` or `.docket.yml`). Turns a dead end into a next step. Low risk.
2. **Split what global can drive by commit-status.** Let global `agent_harnesses` extend to
   the **machine-local** per-repo artifacts (`.codex/agents/*.toml`, Cursor rules — gitignored,
   so no cross-machine `--check` divergence) while the **committed** `AGENTS.md` stays gated on
   repo config. Caveat: without `AGENTS.md`, codex loads the pinned agents but won't auto-dispatch.
3. **Explicit global opt-in for full per-repo behavior.** A flag (e.g. `apply_per_repo: true`)
   that accepts the cross-machine tradeoff by choice, keeping the safe default intact. Heavier;
   re-opens the 0050 determinism argument — only if #2 proves insufficient.

## Out of scope

- Changing the ADR-0036 decision that `AGENTS.md` is committed and machine-neutral.
- User-level `~/.codex/AGENTS.md` dispatch (deferred to change 78's live-codex validation).

## Open questions

- Is #1 sufficient on its own, or is the ergonomic ask really #2/#3?
- For #2: is "agents present but not auto-dispatched" enough value for codex, or misleading?
- Does the coordination-key fence (ADR-0019) already answer whether a global-driven per-repo
  target is permissible at all?

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
