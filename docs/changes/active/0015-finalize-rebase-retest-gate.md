---
id: 15
slug: finalize-rebase-retest-gate
title: finalize — rebase onto base + re-run tests before merge
status: implemented
priority: medium
created: 2026-06-15
updated: 2026-06-17
depends_on: []
related: [16, 17]
adrs: [8, 10]
spec: docs/superpowers/specs/2026-06-17-finalize-rebase-retest-gate-design.md
plan: docs/superpowers/plans/2026-06-17-finalize-rebase-retest-gate.md
results: docs/results/2026-06-17-finalize-rebase-retest-gate-results.md
trivial: false
auto_groomable:
branch: feat/finalize-rebase-retest-gate
pr: https://github.com/danielhanold/docket/pull/32
blocked_by:
reconciled: true
---

## Why

`docket-finalize-change` merges an approved PR by trusting the PR's own CI —
green on the PR **head**. `gh pr merge --merge` blocks only *textual* conflicts.
So a PR that is **behind base** can pass its own CI yet produce a logically-broken
integration branch once merged — a **semantic** conflict git auto-merges cleanly
(e.g. base renamed a symbol the PR still calls). Nothing re-validates the *merged*
result before it lands. Finalize's only test step today is a parenthetical
*optional*, so the effective gate is "the PR head was green when a human approved
it."

## What changes

Add a **rebase-onto-base + re-run-tests gate** to finalize's merge step — the only
place docket itself performs a merge. Before the merge lands: rebase the feature
branch onto `origin/<integration_branch>`, validate the integrated result against
the repo's suite, and merge only if green.

- **Config** (`.docket.yml`): `finalize.gate` = `local` (default) · `ci` · `both` ·
  `off`. The test command is auto-detected (optional `test_command` override); `off`
  restores today's behavior.
- **Two pinned `opus/xhigh` subagents**, split at rebase-completion: a
  **`docket-rebase-resolver`** reconciles rebase conflicts *during* the rebase, and a
  **`docket-integration-repair`** owns red-test outcomes *after* it (root-cause +
  minimal fix, ≤2 attempts). Both abort-and-report when ambiguous or stuck.
- **Auto-authored repairs never merge unseen**: interactive finalize prompts with
  the diff before merging; autonomous finalize pushes the repair and aborts-and-reports.

Full design — the gate flow, the two-agent boundary, the sign-off rule, and the
abort-and-report set — is in the linked spec.

## Out of scope

- The merge *mode* (merge / squash / rebase-merge) — stays the team's `gh` flag.
- The `docket-status` sweep — it only archives already-merged PRs, so a pre-merge
  gate has nothing to act on there; the gate is finalize-only.
- Asserting the two agents' resolution/repair *quality* — governed by their
  `opus/xhigh` tier, not the test suite.

## Reconcile log

### 2026-06-17 — reconciled at claim (no scope change)

Spec was authored **today** (2026-06-17) via `docket-groom-next`, *after* change 0017
merged (PR #31, archived 2026-06-17), so it was already drafted against the current
world. Verified the design against current code on `origin/main` — all in sync,
nothing dropped, adjusted, or folded in. Specifics confirmed:

- **All 8 touch points present and accurate:** `skills/docket-finalize-change/SKILL.md`
  step 1 is the merge step the gate guards; `.docket.yml` has no `finalize:` block yet
  (to add); `skills/docket-convention/SKILL.md` carries the wrapper-count prose at the
  Agent-layer + Composition sections; `skills/docket-status/SKILL.md` has the
  `## Merge sweep` section for the one-line finalize-only note; `agents/` has 6 wrappers
  today (the auto-discovery glob means the two new ones need no `sync-agents.sh` edit);
  `tests/test_sync_agents.sh` has the two `= "6"` count asserts (lines 17, 61) +
  the Task-1b critic block to mirror.
- **ADR landscape clarified.** Spec's header calls change 0017's decision
  "composition wiring — ADR-0009", but on disk **ADR-0009 is "auto-groom-critic
  isolation"**; the composition wiring itself is the dated **Update to ADR-0008**
  (2026-06-16). Both are immutable and must NOT be edited. The new ADR(s) this change
  produces should `relates_to: [8, 9]` and reuse 0017's named-subagent-dispatch +
  git-state-contract pattern — exactly as the spec's design body (§5) intends.
- **Stale-count trap (LEARNINGS #5/#14).** Adding two no-skill wrappers takes the
  total **6 → 8** (5 skill-wrappers + critic + these two). The "five *skills* get a
  wrapper" language stays exact (these wrap no skill, like the critic). Plan includes
  a repo-wide grep for stale count words beyond the two known spots.
- **No obsolescence, no fundamental invalidation** — proceeding to plan + build.
