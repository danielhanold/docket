---
id: 92
slug: orphan-detection-script
title: Orphan detection script — cross-reference change ids in merged commits against archive state
status: proposed
priority: medium
created: 2026-07-17
updated: 2026-07-17
depends_on: []
related: [23, 83]
adrs: [1, 12]
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
| Artifact | Link |
|---|---|
| ADRs | [ADR-0001](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0001-docket-metadata-branch-model.md), [ADR-0012](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0012-docket-status-script-vs-model-boundary.md) |
<!-- docket:artifacts:end -->

## Why

Synthesized from the beads (gastownhall/beads) competitive review (2026-07-17). Beads embeds the
issue id in commit messages ("Fix bug (bd-abc)") and ships `bd orphans` / `bd doctor` checks that
detect "issues referenced in commits but still open" — a pure cross-reference between the code
history and the tracker that catches every book-keeping failure mode in one sweep.

docket already has the raw material: commit subjects and PR titles carry change ids by convention
(`docket(0062): …`, `feat …(change 0085)`), archive filenames carry ids and dates, and — under
`terminal_publish: true` — the integration branch should hold a copy of every terminal record.
Nothing cross-references them. The cost is real: change #0043's terminal record silently never
reached `main` and sat undetected for eight days until found by hand; #0083 investigates that
specific gap, and this check *generalizes* it — any divergence between what the git history says
happened to a change and what the docket state says becomes detectable mechanically.

## What changes

- **A deterministic script** (constraint from capture: a script, not model prose — the ADR-0012
  script-vs-model boundary), pure git reads, no network:
  - Extract change ids referenced by commits merged on `origin/<integration_branch>`.
  - Cross-reference against docket state on `origin/<metadata_branch>`: merged-but-not-terminal
    (id in history, change still `active/`) — the classic orphan; archived-but-unpublished
    (terminal on `docket`, record missing on the integration branch when `terminal_publish:
    true` — the #0043 failure); referenced-but-nonexistent ids (typo'd or deleted).
  - Report one line per finding with the evidence commit; exit codes per the script-contract
    convention (`scripts/<name>.md`).
- Wired in as a `docket-status` health check via the `docket.sh` facade, alongside the existing
  stale-claim / broken-link / dependency-stall checks.

## Out of scope

- Auto-healing (publishing missing records, archiving orphans) — the check reports; humans or a
  later change decide remediation. #0083 owns the decision for the terminal-publish gap
  specifically.
- Enforcing a commit-message id convention going forward (detection works over whatever ids it
  can parse; tightening the convention is separate).

## Open questions

- Id-extraction patterns: which subject forms count (`docket(NNNN)`, `(change NNNN)`, `#NNNN`,
  branch names in merge subjects), and how to bound false positives on bare `#NNNN` (which also
  matches PR numbers)?
- History window: full history each run, or since a recorded high-water mark?
- Relationship to #0083: does that change's detection half collapse into this script (with 0083
  keeping only the root-cause investigation)?

## Reconcile log
