---
id: 261
slug: give-run-halted-a-board-surface-and-a-health-check-like-its
title: 'Give ''## Run halted'' a board surface and a health check, like its two family members'
status: 'in-progress'
priority: high
type: feat
created: 2026-08-07
updated: '2026-09-04'
depends_on: []
related: [222]
discovered_from: [237]
adrs: []
spec: docs/superpowers/specs/2026-08-09-give-run-halted-a-board-surface-and-a-health-check-like-its-design.md
plan: 'docs/superpowers/plans/2026-09-03-give-run-halted-a-board-surface-and-a-health-check-like-its.md'
results:
trivial: false
auto_groomable: true
branch: 'feat/give-run-halted-a-board-surface-and-a-health-check-like-its'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-04T01:17:11Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-09-give-run-halted-a-board-surface-and-a-health-check-like-its-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-09-give-run-halted-a-board-surface-and-a-health-check-like-its-design.md) |
| Plan | [2026-09-03-give-run-halted-a-board-surface-and-a-health-check-like-its.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-09-03-give-run-halted-a-board-surface-and-a-health-check-like-its.md) |
<!-- docket:artifacts:end -->

## Why

**Trigger** — surfaced by the whole-branch review of change 0237, which introduced the
`## Run halted` presence-encoded body section and its `verify-run` reader.

**Opportunity** — `## Run halted` is the only member of its family with no derived-view surface.
Both named siblings have one: `## Finalize blocked` renders `finalize blocked — needs you` and is
guarded by the `stale-finalize-blocked` health check; `## Auto-groom blocked` renders
`auto-groom blocked — needs you`. `## Run halted` renders nothing and no check reads it, so a run
that deliberately stopped needing a human is visible only to whoever reads the dispatch seam's
stderr, or later and indirectly through `aborted-run`'s floor-gated backstop.

**Independent value** — with 0237 reverted the section would not exist, so this is strictly
downstream of it; but with 0237 merged the surface is valuable on its own and on any schedule. A
human scanning the board is the intended consumer of every other needs-you cell, and the halt is
the one disposition the contract defines as *stop + surface*.

**Boundary** — a board cell for an `in-progress` change carrying `## Run halted`, plus a
`board-checks.sh` check-id for it (with the `BOARD_CHECK_IDS` array, `board-checks.md`,
`docket-status.md`, and the `--help` enumeration all updated together, as that array's contract
requires). It deliberately leaves alone: `verify-run`'s verdict vocabulary, the dispatch seam's
exit contract, and every existing `aborted-run` leg and floor.

**Reason for deferral** — change 0237's spec explicitly rules it out: *"Board rendering of the
section is out of scope — `aborted-run` already surfaces the change, and a new board cell is scope
this change does not need."* Building it on 0237's branch would reverse a settled scope decision
and pull `board-checks.sh` — which that spec deliberately leaves untouched — into the diff.

## What changes

Re-targeted to the Docket **Go** tree (the Bash scripts the original spec named — `render-board.sh`, `board-checks.sh`, `lib/docket-frontmatter.sh`, the `BOARD_CHECK_IDS` array — are all deleted). Against current reality the deliverable narrows to the **board surface**, which carries the change's primary value (a human scanning the board is the intended consumer of the `stop + surface` disposition):

- **Board cell** — the In progress section table in `internal/render/board.go` gains a trailing `Readiness` column that renders `run halted — needs you` for an `in-progress` change carrying the bare `## Run halted` section (via the existing `domain.Change.HasRunHalted()` predicate), and an empty cell otherwise. This mirrors the Blocked table's `finalize blocked — needs you` cell wording (family idiom: lowercase marker name + `— needs you`). The header (`boardSectionTableHeader`), the row (`boardSectionRow`), and the package-doc column-layout comment move together; the board golden (`internal/render/testdata/board/board.golden`) plus a run-halted corpus fixture are regenerated in the same commit.

**Already satisfied in Go (no work):**
- The **shared predicate** the spec asked for already exists as a first-class method: `HasRunHalted()` (`internal/domain/entities.go`), decoded from the whole-line `## Run halted` heading in `internal/repository/decode.go` (`markerRunHalted`) — the same `-x`-equivalent whole-line match 0237's bare-heading contract requires. No new helper is needed.
- The spec's **`digest_readiness()` in-progress arm** has no Go home: Go readiness (`internal/domain/readiness.go` `EvaluateReadiness`) is a **proposed-only** concept with no in-progress arm and no separate digest projection to keep in sync. The board cell is the single Go surface, so there is no digest/board divergence to guard.

## Out of scope

Unchanged exclusions from the stub's boundary: `run.verify`'s verdict vocabulary, the dispatch seam's exit contract, every existing `aborted-run` leg and floor, and the marker's write/remove lifecycle (0237 owns it).

**Newly out of scope after Go reconcile — the health check.** The spec's `stale-run-halted` health check was designed to *mirror* `stale-finalize-blocked` in `board-checks.sh`. That entire advisory board-health subsystem was **never ported to Go**: there is no `stale-finalize-blocked`, no `aborted-run`, no `stale-claim` check, no `BOARD_CHECK_IDS` array, and no `FINALIZE_BLOCKED_STALE_SECS`-family horizon constant anywhere in `internal/`. Building `stale-run-halted` would therefore not be a one-check addition mirroring a sibling — it would be net-new check-id infrastructure (a home in the maintenance sweep or `repository.check`, a horizon-constant family, a finding-code, and a surfacing contract), which is separate design work beyond this change. It is reported as follow-up work for deliberate capture. The board cell alone delivers the disposition's board visibility, which is the change's stated primary value.

## Open questions

Resolved at reconcile (2026-09-04): the Bash→Go re-targeting is settled. The board cell lands in `internal/render/board.go`; the predicate already exists (`HasRunHalted()`); the digest arm and the health-check/`BOARD_CHECK_IDS` subsystem are dropped as no-longer-applicable to the Go tree (see Out of scope and the Reconcile log). No open questions remain for the scoped board-cell deliverable.

## Reconcile log

### 2026-09-04

2026-09-04 — Reconciled against the current Docket Go tree (spec was authored 2026-08-09 against the now-deleted Bash implementation). Findings: (1) the shared `run_halted()` predicate the spec calls for already exists in Go as `domain.Change.HasRunHalted()`, decoded via the whole-line `markerRunHalted` heading in internal/repository/decode.go — no new helper needed. (2) The `digest_readiness()` in-progress arm has no Go equivalent: Go readiness (internal/domain/readiness.go) is proposed-only with no in-progress digest projection, so there is no board/digest divergence to keep in sync; the board cell is the sole surface. (3) The `stale-run-halted` health check and the four-surface `BOARD_CHECK_IDS` pin reference an advisory board-health subsystem (stale-finalize-blocked, aborted-run, stale-claim, horizon constants) that was NEVER ported to Go — confirmed by whole-repo grep (only archived docs mention it). Building it would be net-new infrastructure, not a sibling mirror, so it is moved out of scope and reported as follow-up. Scope decision: deliver the board surface only — a trailing Readiness column on the In progress table rendering `run halted — needs you` in internal/render/board.go, mirroring the Blocked table's `finalize blocked — needs you` cell — which carries the change's stated primary value (board visibility of the stop+surface disposition). The design is scope-adjusted, not fundamentally invalidated: section 1 of the spec (the board cell) is intact and buildable; sections 2-4 are either already satisfied in Go or reference deleted Bash infrastructure.
