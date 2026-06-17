---
id: 10
slug: finalize-merge-gate-split-agents
title: Finalize merge gate — split conflict-resolution from semantic-repair at the rebase-completion boundary
status: Accepted
date: 2026-06-17
supersedes: []
reverses: []
relates_to: [8, 9]
change: 15
---

## Context

`docket-finalize-change` merges an approved PR by trusting the PR's **own** CI —
which was green on the PR **head**. `gh pr merge` only blocks *textual* conflicts.
So a PR that is **behind base** can pass its own CI and still produce a
logically-broken integration branch once merged — a **semantic conflict** git
auto-merges cleanly (e.g. base renamed a symbol the PR still calls, or changed a
contract the PR relied on). Nothing re-validates the **merged result** before it
lands; finalize's only test step today was a parenthetical *optional*. Finalize's
merge step is the **only place docket itself performs a merge** (the `docket-status`
bulk sweep only archives PRs a human already merged), so it is the single chokepoint
where docket can interpose a gate.

Two distinct kinds of judgment arise once we bring the feature branch up to base:
reconciling **textual** conflicts *during* the rebase, and repairing **semantic**
breakage that surfaces only *after* the merged tree exists and the tests run red.
Conflating them in one agent muddies the contract (when does it run tests? whose
fault is a red suite?) and risks an under-powered or mis-scoped repair.

## Decision

Add a **rebase-onto-base + re-run-tests gate** to finalize's merge step, configured
by a new `.docket.yml` `finalize.gate` knob (`local` default · `ci` · `both` · `off`).
The gate rebases `feat/<slug>` onto `origin/<integration_branch>` and re-validates the
integrated result *before* `gh pr merge`; `gate: off` preserves prior behavior exactly.

The judgment work is split across **two dedicated wrappers**, divided at **the rebase
completing**:

1. **`docket-rebase-resolver` (①)** reconciles textual conflicts *during* the rebase
   and **never runs tests** — its job ends when the rebase lands.
2. **`docket-integration-repair` (②)** owns **every** red-test outcome *after* the
   rebase lands — whether the cause is base drift or a bad ① resolution — bounded to
   **≤2 attempts**.

The rebase-completion boundary is the rule: before it is ①'s territory (conflicts),
after it is ②'s (semantics). ② owning *all* red outcomes regardless of cause keeps
the boundary clean — there is no shared "who repairs this" ambiguity.

Both wrappers **wrap no skill**: they load only `docket-convention`, reusing the
[[0009]] no-skill-wrapper pattern (their behavior rides the dispatch prompt from the
finalize gate, which stays the single source). Both are pinned at the judgment tier.

The gate's auto-authored repairs are governed by a **sign-off rule**: **interactive**
finalize prompts before merge; **autonomous** finalize force-pushes the ②-authored
repair and then **aborts-and-reports** — it never merges an unseen, machine-authored
repair. This reuses change 0017's named-subagent-dispatch pattern and extends
[[0008]]'s abort-and-report rule (recorded as a dated Update on [[0008]], not a
reversal).

## Consequences

- **Two new generated wrappers** (`docket-rebase-resolver`, `docket-integration-repair`)
  — bringing the total to **eight**. The Agent-layer count language stays exact: **five
  *skills* get a wrapper** ([[0008]]); these two — like the [[0009]] critic — wrap **no
  skill**, so they do not change the five-skills count.
- **The rebase-completion boundary** is the durable seam: ① = conflicts (no tests),
  ② = all red outcomes (with tests), bounded ≤2 attempts.
- **The gate is finalize-only** — the `docket-status` sweep never merges, so a pre-merge
  gate has nothing to act on there; GitHub-button merges bypass the gate by nature
  (outside docket's control). A one-line note records this in the sweep docs.
- **`gate: off` preserves prior behavior** — the gate is opt-out, defaulting to `local`.
- **Repair authorship is decoupled from repair approval**: finalize may now *author* a
  repair, but the sign-off rule guarantees a machine-authored repair never merges unseen
  (interactive prompts; autonomous force-pushes + aborts-and-reports for human review and
  a re-run). See the dated Update on [[0008]].
- **Cost:** the gate asserts only its *mechanics* (config parse, mode dispatch, abort
  paths) in tests; resolution/repair *correctness* is judgment, governed by the agents'
  pinned tier — not test-asserted.
