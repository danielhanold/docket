---
id: 259
slug: harden-render-board-sanitize-feeder-values-and-settle-the-fa
title: 'Harden render-board: sanitize feeder values and settle the failure contract'
status: proposed
priority: medium
type: fix
created: 2026-08-07
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [155, 156]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

Consolidates #0155 and #0156 (2026-08-07 triage): both carved out of 0143 as its follow-ups, both in `scripts/render-board.sh` — value hygiene in the sort feeders, and the renderer's failure contract.

Verified 2026-08-07:

- **Interior TABs shift the archive feeder (#0155) — with one premise correction.** The archive feeder still interpolates a raw frontmatter value into its TAB join: `render-board.sh:400-404` (`st="$(field "$f" status)"` → `printf '%s\t%s\t%s\t%s\n'`), read back at `:388` with `IFS=$'\t'` — a TAB inside the value splits into extra fields and shifts everything right (the mirror of 0143's empty-field left-shift). `ARC_COUNT["$st"]` (`:154`) also takes the raw value as an array subscript. **The live carrier is `status:` — not `title:` as #0155 claimed; `title` is read at print time (`:315,:399`), never fed through a TAB join. `created` is shape-validated (`:249-252`).** The active-side feeder is safe (id+path only). The sanitize precedent exists: `board-checks.sh:142` (change 0104).
- **Exit-0-on-corruption (#0156).** `render-board.sh` has no malformed-input failure path — the only non-zero exits are CLI-argument errors; the digest path ends `exit 0` (`:247`), markdown falls off the end. A silent renderer failure can empty the `--format digest` `ready` line and starve the autonomous build loop. Downstream mitigation exists but is partial: `board-refresh.sh:110-123` gates on exit code + non-emptiness (catches an empty render, not a corrupt-but-non-empty one); `docket-status.sh:407` trusts the digest call's exit status. 0143's demonstrated corruptions are fixed (guards at `:402`, `:139-140`); the contract gap is what remains.

## What changes

- Sanitize (or reject) control characters in feeder-interpolated frontmatter values, `board-checks.sh:142`-style, at the archive feeder and the `ARC_COUNT` subscript; regression fixture with an interior-TAB `status:`.
- Settle the failure contract: the renderer detects malformed input and exits non-zero with a diagnostic (default posture — callers already gate on exit code / report lines: `board-refresh.sh` and `docket-status.sh` need no new branching, only honest signals). Enumerate what "malformed" means (unparseable frontmatter, impossible status, feeder field-count mismatch) and pin each with a fixture.

## Out of scope

- Structural fixes (replacing the TAB-join protocol entirely).
- New board surfaces or digest fields.

## Open questions

- Whether warn-and-skip-the-row is ever preferable to exit-nonzero for a single bad change file (availability vs correctness at the row level) — the one genuine posture fork; an abstain back to the human queue is acceptable.
