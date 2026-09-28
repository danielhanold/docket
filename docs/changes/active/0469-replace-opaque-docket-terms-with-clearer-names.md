---
id: 469
slug: 'replace-opaque-docket-terms-with-clearer-names'
title: 'Replace opaque docket terms with clearer names'
status: 'proposed'
priority: 'medium'
type: 'refactor'
created: '2026-09-28'
updated: '2026-09-28'
depends_on: []
stacked_on:
related: [402, 468]
discovered_from: []
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

A review of `docs/reference/glossary.md` (2026-09-28) found a set of docket terms that don't clash with anything but are hard for a newcomer (human or agent) to understand without reading the entry: "admission slot", "owner generation", "unmet conjuncts", "presence-encoded section", "Step-0 preamble", "CREATE_ORPHAN", dispatch tiers "A / B / C", "dummy mode", "disposition", and others. Each has a plainer name that says what the thing is or does. Renaming them lowers the reading cost of the docs, run logs, and verdict lines. "needs-brainstorm" also conflicts with the project's own vocabulary, which uses "groom" for this step and reserves "brainstorm" for net-new changes.

## What changes

Rename the opaque terms (proposed names are hypotheses to settle at grooming):

| Current | Suggested | Why |
|---|---|---|
| gate key / dispatch context | **run ticket** / **claim token** | What each is for: you hand in the ticket at verdict time; the token goes to `change.claim` |
| "unmet **conjuncts**" (`gate-retry-once`) | **unmet conditions** | Logic-textbook jargon |
| **Admission slot** | **worktree lock** | That's exactly what it is |
| `unresolved-execution` | `previous-run-unconfirmed` | Says what's wrong: nothing proved the last run shut down |
| **Owner generation** (`--owner-gen`) | **owner number** (`--owner-seq`) | It's a counter |
| **Identity mismatch / identity drift** | **head changed** / **checkout changed** | Names the thing that actually moved |
| **Liveness transition** | **moved to background** | Plain description |
| **Native supervisor / gate execution** | **run supervisor** / **launch mode** | "Native" says nothing to a newcomer |
| **Presence-encoded section** | **marker section** | Its presence is the flag |
| **Step-0 preamble** | **skill startup check** | Describes what it does |
| **Bootstrap verdicts** `STOP_MIGRATE` / `CREATE_ORPHAN` | `NEEDS_MIGRATION` / `CREATE_METADATA_BRANCH` | "Orphan" is git plumbing jargon |
| **needs-brainstorm** (readiness) | **needs-grooming** | The project uses "groom" for this step and reserves "brainstorm" for net-new changes |
| **Abstain** (auto-groom) | **hand back** | The stub goes back to the human queue |
| **Dispatch tiers A / B / C** + carve-out | **run-inline / skip / halt** + **never-inline** | The letters carry no meaning; name them by behaviour |
| **Dummy mode** / persona | **plain-language mode** / **reader profile** | "Dummy" is mildly pejorative and hides what it does |
| **Coordination key / scope tag** | **shared setting** / **where-settable tag** | Describes the rule |
| **Inert / deferred setting** | **unused / not-yet-supported setting** | Plain meaning |
| **Disposition** (everywhere) | **outcome** | Gate drives already use `.outcome`; keep "result" for the envelope's top level |
| **Closed vocabulary** | **allowed values** | Plain meaning |
| **Digest / digest-only read** | **status summary** | A Bash-era leftover; today it's just `docket status` |
| **Compare-and-swap** (prose) / **contended** | **conflict-checked write** / **lost a race** | Keep `contended` as the token; use the plain phrase in human-facing text |
| **Pay per relevance** | **read on demand** | Plain meaning |
| **Sync integration** | **fast-forward main** | Says what it does |
| **Metadata branch** | **backlog branch** | It holds the backlog, specs and ADRs; "metadata" is vague |
| **Identity repair** (`change repair-identity`) | **relink branch/PR** (`change relink`) | Says what it fixes |
| **Continuation** id | **continue token** | Pairs with `gate-continue`; avoids "resume", which `--resume` already uses |
| **Reconcile** | **still-valid check** (optional) | Low priority; reconcile is known in git circles |

Where a renamed term is also a wire token (operation id, CLI flag, config key, closed-vocabulary token, board cell), keep the old spelling as an accepted alias for a deprecation window so skills, agents, and scripts that match on it keep working. Update the glossary, guide, concept pages, skills, agent wrappers, and CLAUDE.md/AGENTS.md prose consistently.

## Out of scope

- The colliding-term renames and the obsolete-term retirement; those are tracked in the companion change.
- Terms that are already standard or plain English (harness, claim, stub, trivial, spec, plan, handoff, takeover, slice, fix loop, war story, sweep, stacked change, preflight, `gate-retry-once`, `gate-done`, `gate-stop`).
- Rewriting frozen build records, archived changes, specs, or Accepted ADRs.
- Removing the old wire-token spellings; alias removal is a later change after the deprecation window.
