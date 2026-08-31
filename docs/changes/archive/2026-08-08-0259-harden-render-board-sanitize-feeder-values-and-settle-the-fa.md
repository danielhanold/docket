---
id: 259
slug: harden-render-board-sanitize-feeder-values-and-settle-the-fa
title: 'Harden render-board: sanitize feeder values and settle the failure contract'
status: done
priority: medium
type: fix
created: 2026-08-07
updated: 2026-08-08
depends_on: []
related: [244]
discovered_from: [155, 156]
adrs: []
spec: docs/superpowers/specs/2026-08-07-harden-render-board-sanitize-feeder-values-and-settle-the-fa-design.md
plan: docs/superpowers/plans/2026-08-07-harden-render-board-sanitize-feeder-values-and-settle-the-fa.md
results: docs/results/2026-08-07-harden-render-board-sanitize-feeder-values-and-settle-the-fa-results.md
trivial: false
auto_groomable: true
branch: feat/harden-render-board-sanitize-feeder-values-and-settle-the-fa
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/177
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-07-harden-render-board-sanitize-feeder-values-and-settle-the-fa-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-07-harden-render-board-sanitize-feeder-values-and-settle-the-fa-design.md) |
| Plan | [2026-08-07-harden-render-board-sanitize-feeder-values-and-settle-the-fa.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-07-harden-render-board-sanitize-feeder-values-and-settle-the-fa.md) |
| Results | [2026-08-07-harden-render-board-sanitize-feeder-values-and-settle-the-fa-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-08-07-harden-render-board-sanitize-feeder-values-and-settle-the-fa-results.md) |
<!-- docket:artifacts:end -->

## Why

Consolidates #0155 and #0156 (2026-08-07 triage): both carved out of 0143 as its follow-ups, both in `scripts/render-board.sh` — value hygiene in the sort feeders, and the renderer's failure contract.

Verified 2026-08-07:

- **Interior TABs shift the archive feeder (#0155) — with one premise correction.** The archive feeder still interpolates a raw frontmatter value into its TAB join: `render-board.sh:400-404` (`st="$(field "$f" status)"` → `printf '%s\t%s\t%s\t%s\n'`), read back at `:388` with `IFS=$'\t'` — a TAB inside the value splits into extra fields and shifts everything right (the mirror of 0143's empty-field left-shift). `ARC_COUNT["$st"]` (`:154`) also takes the raw value as an array subscript. **The live carrier is `status:` — not `title:` as #0155 claimed; `title` is read at print time (`:315,:399`), never fed through a TAB join. `created` is shape-validated (`:249-252`).** The active-side feeder is safe (id+path only). The sanitize precedent exists: `board-checks.sh:142` (change 0104).
- **Exit-0-on-corruption (#0156).** `render-board.sh` has no malformed-input failure path — the only non-zero exits are CLI-argument errors; the digest path ends `exit 0` (`:247`), markdown falls off the end. A silent renderer failure can empty the `--format digest` `ready` line and starve the autonomous build loop. Downstream mitigation exists but is partial: `board-refresh.sh:110-123` gates on exit code + non-emptiness (catches an empty render, not a corrupt-but-non-empty one); `docket-status.sh:407` trusts the digest call's exit status. 0143's demonstrated corruptions are fixed (guards at `:402`, `:139-140`); the contract gap is what remains.

## What changes

Settled design (auto-groomed 2026-08-07; detail and audit trail in the spec's `## Assumptions`):

- **Failure contract — render-complete, then fail loud.** Row-level skips stay (stdout complete modulo skipped rows, both formats), but every malformed file emits a sanitized stderr diagnostic (`render-board: malformed change file: <path>: <reason>`) and a non-zero count makes the run exit 3 (0 clean, 2 CLI-argument errors unchanged). Callers need no edits — `board-refresh.sh` already discards on non-zero, `docket-status.sh`'s `digest_only_pass` fails closed — so corruption becomes stale-with-a-named-cause instead of corrupt-and-committed.
- **"Malformed" is a closed enumeration**, checked in one upfront pass: unusable id (M1), empty status (M2), status outside `DOCKET_STATUSES` (M3 — which subsumes interior TAB/CR in `status:`, so rejection-by-vocabulary is the sanitization and no control character reaches the TAB join or the `ARC_COUNT`/`SECTION` subscripts), plus a belt-and-suspenders read-back arity check at the archive consumer (M4). Placement-wrong-but-vocabulary-valid status (e.g. mid-sweep `done` in active/) is deliberately NOT malformed — that stays `board-row-dropped`'s territory.
- **Tests:** interior-TAB-status regression fixture (per #0155), impossible-status fixture, contract flips on the existing exit-0 pins (including a loud, deliberate override of the pinned no-id-guard `ARC_COUNT` tally asserts), and new clean-path exit-0/empty-stderr pins.

## Out of scope

- Structural fixes (replacing the TAB-join protocol entirely).
- New board surfaces or digest fields.
- Any edit to `board-checks.sh` (its `sanitize()` is duplicated, not hoisted); slug/title content hygiene in digest lines (spec-recorded narrowing).

## Open questions

None — resolved at design time; see the spec's `## Assumptions`. The former posture fork (warn-and-skip vs exit-nonzero) is settled as both: skip the row for stdout completeness, exit 3 so callers gate honestly; the availability cost (one bad file halts board refresh and autonomous selection, loudly) is accepted in Assumption 1. `related: [244]` records a file-collision coupling (0244 migrates frontmatter read call sites in `render-board.sh`); no ordering constraint.

## Reconcile log

### 2026-08-07 — reconciled at claim (implement-next)

Every premise in the change body and the spec re-verified against `origin/main` at `483c5dad`. **No scope change; the design stands as groomed.**

- **Feeder hazard still live.** `scripts/render-board.sh` (415 lines) still interpolates the raw `status:` value into the archive sort feeder's TAB join and reads it back under `IFS=$'\t' read -r date id st f`; the guard on that line is still emptiness-only (`[ -n "$id" ] && [ -n "$st" ] || continue`). `ARC_COUNT["$st"]` and `SECTION["$st"]` still take the raw value as an associative-array subscript, guarded only for emptiness.
- **Exit-0-on-corruption still live.** The digest block still ends with an unconditional `exit 0`; markdown still falls off the end. Non-zero exits remain CLI-argument errors only.
- **Line numbers drifted, targets did not.** The spec's `:400-405` / `:388` / `:154` / `:141` / `:247` citations are approximate against the current file; the code shapes they name are all present and unchanged in substance. Implementation should locate by code shape, not by the spec's line numbers.
- **Test pins confirmed present** in `tests/test_render_board.sh` (2170 lines): the exit-0-on-malformed-id assert, the 0143 block's `Archive — done (2)` / `backlog done 2` tally asserts (pinned deliberately so no silent fix lands), and the golden byte-compare render that discards stderr and never captures an exit code. The spec's contract-flip plan applies verbatim.
- **Coupling unchanged.** `related: [244]` — change 0244 is still `proposed` and unbuilt, so no frontmatter-read-helper migration has landed in `render-board.sh`; no ordering constraint, whichever lands second reconciles mechanically. `depends_on:` stays empty; all other cited changes (0143, 0155, 0156, 0157, 0104, 0115, 0127, 0094) remain terminal.
- **Auto-capture:** enabled this run; the pass surfaced no discovery clearing the six admission gates. Nothing minted.
