---
id: 55
slug: slim-remaining-skills
title: Slim docket-implement-next + propagate Step-0 preamble to the small skills
status: proposed
priority: medium
created: 2026-07-10
updated: 2026-07-10
depends_on: [53]
related: [53, 54]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

Change #0053's categorization: `docket-implement-next` (~137 lines / ~2,900 words) restates the
`render-change-links.sh` regeneration litany four times and carries the Step-0/mode boilerplate;
the four small skills (`docket-new-change`, `docket-groom-next`, `docket-adr`,
`docket-auto-groom`) are lean but still carry the full Step-0 preamble and (new-change) a
duplicated kill sequence. Medium potential, medium risk — the implementer's repetition is partly
deliberate reinforcement for an autonomous agent.

## What changes

- `docket-implement-next`: state the Artifacts-block regeneration rule once (convention-side),
  dedupe conservatively; rewire the reconcile-kill to
  `references/terminal-close-out.md`; compress the Step-0/mode boilerplate. The branch/metadata
  discipline section stays.
- `docket-new-change`: proposed-kill sub-path → close-out reference; Step-0 compression.
- `docket-groom-next`, `docket-adr`, `docket-auto-groom`: Step-0 preamble compression only.
- Cut provenance narration per #0053's decision 2 across all five.

## Out of scope

- Any behavior change to selection, claim CAS, reconcile, build, review, or kill semantics.
- `docket-convention`, `docket-status` (#0053) and `docket-finalize-change` (#0054).

## Open questions

- How much of implement-next's repetition is load-bearing reinforcement vs. safe to dedupe —
  decide per-instance at brainstorm/plan time.

## Reconcile log
