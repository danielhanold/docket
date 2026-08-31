---
id: 137
slug: forked-claude-code-skills-assume-absent-task-dispatch
title: "Claude Code dispatch-capability detection: name-based probing silently drops SDD build and review discipline"
status: done
priority: critical
type: fix
created: 2026-07-24
updated: 2026-07-27
depends_on: []
related: [16, 17, 49, 61, 113, 135]
discovered_from: [136]
adrs: [8, 17, 24, 26, 59]
spec: docs/superpowers/specs/2026-07-25-dispatch-capability-detection-design.md
plan: docs/superpowers/plans/2026-07-25-dispatch-capability-detection.md
results: docs/results/2026-07-25-dispatch-capability-detection-results.md
trivial: false
auto_groomable:
branch: feat/forked-claude-code-skills-assume-absent-task-dispatch
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/126
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-25-dispatch-capability-detection-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-25-dispatch-capability-detection-design.md) |
| Plan | [2026-07-25-dispatch-capability-detection.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-07-25-dispatch-capability-detection.md) |
| Results | [2026-07-25-dispatch-capability-detection-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-07-25-dispatch-capability-detection-results.md) |
| ADRs | [ADR-0008](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0008-agent-layer-generated-subagents.md), [ADR-0017](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0017-cursor-dispatch-rule-full-agent-set.md), [ADR-0024](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0024-claude-context-fork-skill-dispatch.md), [ADR-0026](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0026-fork-dispatch-opacity-two-invocation-paths.md), [ADR-0059](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0059-dispatch-capability-resolved-not-inferred-from-tool-name.md) |
<!-- docket:artifacts:end -->

## Why

A live `docket-implement-next 136` run reported that its runtime had **no subagent-dispatch
(`Task`) tool**, so `superpowers:subagent-driven-development` could not dispatch fresh per-task
implementers and `superpowers:requesting-code-review` could not dispatch a reviewer. Both roles
fell back to their inline `auto` fallbacks. PR #124 was sound and the degradation was disclosed in
the results file and PR body — the honest-degradation posture worked — but the SDD isolation and
independent review docket's wrapper advertises did not run. The same thing happened on change
[[127]] (2026-07-22).

**Grooming established that the premise is wrong.** A probe on 2026-07-25 found there is no tool
named `Task` in current Claude Code at all — the dispatch tool is named `Agent` — and that dispatch,
nesting, and `Skill` all work in a dispatched subagent. Subagent tool sets are also **partially
deferred** behind a search surface, so absence is easy to observe without ever having resolved
anything. `AskUserQuestion` really is absent, so ADR-[[0024]]'s fork-exclusion principle stands.

The defect is a **false-negative capability probe**, not a harness capability gap. Docket's own
prose primes an agent to look for a tool named `Task`; the Skill-layer *missing-skill rule* then
fires correctly on a false premise, dropping the discipline while every artifact still looks
complete — the `skill-fallback-degrades-discipline` learning (change [[66]]) biting again, with a
new trigger. That it is variance rather than a wall is confirmed by SDD dispatching successfully on
2026-07-14: two bad runs, not always-on.

Only **two** live prose sites are actually wrong (`agent-layer.md:131`, `README.md:620`) plus the
immutable ADR-[[0024]]:16. Four other `Task` mentions are Cursor-scoped and correct — a blanket
rename would introduce new errors.

## What changes

Make docket's dispatch-capability detection honest, and define what an autonomous run does when
dispatch is genuinely unavailable. Design in the spec; at scope altitude:

- Add a **capability-resolution rule** to docket-convention: resolve a dispatch mechanism —
  including searching deferred tool surfaces — and if inconclusive, attempt one trivial dispatch.
  Only a failed attempt or a policy denial establishes unavailability; the absence of a
  specifically-named tool never does. Stated by capability, never by tool name; an observed name
  may appear in the diagnostic only.
- Adopt a **tiered unavailability posture**: deterministic composition (`docket-status`,
  `docket-adr`) runs inline as a first-class equivalent path, since its contract is git state;
  the adversarial `docket-auto-groom-critic` instead triggers auto-groom's existing **abstain**;
  and `build`/`review` are **authorized-or-halt** — an explicitly configured `auto` is the human's
  authorization for inline, anything else halts via abort-and-report.
- Correct the two wrong prose sites, leaving the Cursor-scoped mentions alone.
- Record a **new, harness-neutral ADR** (capability-gate-not-name plus the tiered posture) that
  change [[135]] can cite, plus a dated `## Update` on ADR-[[0024]] extending its fork-exclusion
  reasoning from the human channel to the dispatch channel.
- Prove reachability: a structural test anchored on the consuming skill sections, a negative guard
  that no docket prose gates on a literal tool name, and a **build-time live spike** over both
  ADR-[[0026]] invocation paths whose findings are recorded verbatim in the results file.
- Add the new ADR's id to change [[135]]'s `adrs:` so it cites the shared decision. (The prose
  amendment recording that 135's failure is skill-delivery, not dispatch, already landed at
  grooming on 2026-07-25 — see the reconcile log.)

## Out of scope

- The **Cursor** instance of the symptom — owned by change [[135]]. This change does **not** fix
  Cursor: its fix is delivered as docket-convention prose, through the one channel Cursor is broken
  on. The two share the ADR, not the mechanism, and stay `related:` rather than `depends_on:`.
- Changing the Superpowers SDD / TDD / code-review skills themselves.
- Reworking the agent-layer wrapper generation (`sync-agents.sh`) beyond what this fix requires.
- Retrofitting the already-open PR #124 from the change-136 run.
- Renaming the four Cursor-scoped `Task` mentions, which are correct.
- Verifying Cursor's Task nesting limit — that belongs to change [[135]].

## Open questions

- Does a real `context: fork` child have dispatch, and does it differ from the agent-dispatch path?
  **Answered at build time by the spec's live spike**, which is a *gating* task: if forks
  categorically lack dispatch, Tier C would halt every forked build, so the change stops and reports
  back to the human rather than shipping a posture that bricks `/docket-implement-next`.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

- **2026-07-25 (build claim)** — Claimed for implementation. Design verified against current `main`
  + `docket`; it **stands unchanged** and no scope was adjusted downward beyond one completed item.
  Details in the spec's *Reconcile addendum*. In brief: (1) every line number the spec cites is
  still byte-accurate — both wrong sites, the immutable ADR-0024:16, and all four Cursor-scoped
  mentions; no re-survey needed. (2) **New build constraint** — `tests/test_skill_size_budgets.sh`
  leaves `docket-convention/SKILL.md` **two words** of headroom (347/354 lines, 5848/5850 words) and
  `docket-implement-next/SKILL.md` **eight**, so the fix must consciously raise those budget rows
  in the same diff (the guard's own sanctioned escape hatch; precedent 0127/0102). (3) **New design
  obligation** — Tier C must be drawn *against* the existing missing-skill rule, not layered on it:
  skill not invocable ⇒ degrade+warn, skill invocable but dispatch unresolvable ⇒ authorized-or-halt.
  (4) The negative guard's scope must exclude `archive/` and `docs/results/` as well as the four
  Cursor mentions — those immutable records use "Task" both as the old tool name and as plan-task
  labels. (5) **Scope item already done** — 135's stub amendment landed at grooming on 2026-07-25,
  so it drops to just adding the new ADR id to 135's `adrs:`. Context checks: #0136 is `done`
  (PR #124 merged 2026-07-24), #0135 and #0113 remain `proposed` and gate nothing. This run is
  **deliberately non-forked** — driven inline at `claude-opus-5` at the human's request to preserve
  session model/effort — which itself makes the run a live observation of the dispatch surface the
  change is about.
