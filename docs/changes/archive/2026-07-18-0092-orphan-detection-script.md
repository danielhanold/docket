---
id: 92
slug: orphan-detection-script
title: Orphan detection script — cross-reference change ids in merged commits against archive state
status: done
priority: medium
created: 2026-07-17
updated: 2026-07-18
depends_on: []
related: [23, 83]
adrs: [1, 12]
spec: docs/superpowers/specs/2026-07-17-orphan-detection-script-design.md
plan: docs/superpowers/plans/2026-07-17-orphan-detection-script.md
results: docs/results/2026-07-17-orphan-detection-script-results.md
trivial: false
auto_groomable: true
branch: feat/orphan-detection-script
pr: https://github.com/danielhanold/docket/pull/98
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-17-orphan-detection-script-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-17-orphan-detection-script-design.md) |
| Plan | [2026-07-17-orphan-detection-script.md](https://github.com/danielhanold/docket/blob/main/docs/superpowers/plans/2026-07-17-orphan-detection-script.md) |
| Results | [2026-07-17-orphan-detection-script-results.md](https://github.com/danielhanold/docket/blob/main/docs/results/2026-07-17-orphan-detection-script-results.md) |
| PR | [#98](https://github.com/danielhanold/docket/pull/98) |
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
Nothing cross-references the classic case — a change whose work merged but whose docket record was
never closed out. The cost is real: change #0043's terminal record silently never reached `main`
and sat undetected for eight days until found by hand. That *specific* gap (a terminal record
archived on `docket` but never published to the integration branch) is **#0083's** — it is a
consciously-deferred, human-gated case whose detect-vs-mark-vs-accept decision only a human can
make, and this change deliberately does not pre-empt it. What this change ships is the general
book-keeping-drift detector around it: any change id that the git history says was merged, but that
docket's active/archive state says is not closed out, becomes detectable mechanically.

## What changes

Settled at brainstorm — see [the spec](../../superpowers/specs/2026-07-17-orphan-detection-script-design.md).

- **A deterministic, git-only check** (constraint from capture: a script, not model prose — the
  ADR-0012 script-vs-model boundary; pure git reads, no network), added as **two new check-ids in
  the existing `scripts/board-checks.sh`** (not a new script) so it inherits the git-only invariant,
  the `--metadata-branch`/`--integration-branch` inputs, and — because `docket-status` already pipes
  whatever `board-checks.sh` emits as `check <id> …` — the `docket-status` wiring for free:
  - **`merged-orphan`** — a change id parsed from a merged commit subject on
    `origin/<integration_branch>` while the change is still non-terminal (`active/`, not archived) on
    the metadata branch. The classic orphan; complements the PR-status sweep with a git-history
    signal.
  - **`unknown-commit-ref`** — an id referenced by an integration-branch commit subject with no
    change file (typo'd or deleted).
  - Ids are parsed conservatively from two docket-convention subject forms only —
    `<type>(<id>):` and trailing `(change <id>)` — with bare `#NNNN` excluded to avoid PR-number
    false positives (Open Q1). Full history each run, stateless (Open Q2).
  - Each finding names the evidence commit; output and exit codes ride `board-checks.sh`'s existing
    TSV/`--strict` contract unchanged.

## Out of scope

- **The class-2 detector — archived-but-unpublished terminal records** (a change terminal on
  `docket` whose record is missing from the integration branch under `terminal_publish: true`, the
  #0043 case). This is **#0083's** to decide: its open, human-only question is exactly whether that
  class of consciously-deferred record should be *detected*, marked, or accepted, and building a
  detector here would answer it. Deferred, not built; it lands later as another `board-checks.sh`
  check-id beside these two if #0083 decides so — no rework.
- Auto-healing (publishing missing records, archiving orphans) — the check reports; humans or a
  later change decide remediation. `board-checks.sh` is warn-only by invariant.
- Enforcing a commit-message id convention going forward (detection works over whatever ids it
  can parse; tightening the convention is separate).
- A `--since` history window / persisted high-water mark, and widening the extraction grammar (bare
  `#NNNN`, merge-subject branch names) — additive follow-ups; the conservative floor ships first.

## Open questions

Resolved at brainstorm 2026-07-17 — see the spec's `## Assumptions`. Summary:

- **Id-extraction patterns** → the two docket-convention subject forms `<type>(<id>):` and trailing
  `(change <id>)`, subjects only; bare `#NNNN` and body mentions excluded to bound PR-number false
  positives.
- **History window** → full history each run, stateless (no persisted high-water mark); a `--since`
  window is an additive follow-up if history grows.
- **Relationship to #0083** → **no collapse.** This change ships the classic-orphan and
  dangling-ref detectors (classes 1 & 3); the terminal-publish-gap detector (class 2) stays #0083's
  undecided call. The two compose additively as sibling `board-checks.sh` check-ids.

## Reconcile log

### 2026-07-17 — reconcile (docket-implement-next, pre-plan)

Re-read the change + spec against `related` (#0023, #0083), cited ADRs (0001, 0012), and the
current integration-branch code. Design holds unchanged — no scope adjustment, no fold-in, no
drop. Confirmations:

- **#0083 still `proposed` / auto-groom-blocked (needs human).** The class-2 (terminal-publish
  gap, #0043 case) detection question remains a human's undecided call. The scope boundary stands:
  0092 ships classes 1 (`merged-orphan`) & 3 (`unknown-commit-ref`) only; class 2 stays #0083's.
- **`scripts/board-checks.sh` matches the spec's described shape** (origin/main @ 250ff7c): shared
  `FINDINGS` accumulator + `emit`, sources `lib/docket-frontmatter.sh`, walks `active/`+`archive/`,
  `GIT="${GIT:-git}"` mock seam, sorts findings by `(check-id asc, change-id numeric asc)`,
  `--strict` exits 1 on any finding. The two new checks slot into these rails purely additively.
- **`lib/docket-frontmatter.sh` supplies the reuse surface** — `int_field`/`field`/`list_field`
  and the `active/`+`archive/` file walk already tell the checks which ids exist and their terminal
  state; only one `git log --format=%s` pass over the integration ref is new.
- **Zero `docket-status.sh` edit, verified against running code.** `health_checks()` invokes
  `board-checks.sh … --integration-branch "origin/$INTEGRATION_BRANCH"` and pipes *whatever* TSV it
  emits as `check <check-id> <change-id> <message>`. New check-ids surface with no wiring change —
  the extraction `git log` reads `origin/main` subjects (real merged history), resolvable from the
  changes-dir repo via the shared object store.
- Doc/test touchpoints unchanged from the spec: `board-checks.md` (check enumeration + grammar),
  `docket-status.md` (extend the `check` vocabulary note), `tests/test_board_checks.sh` (hermetic
  cases through the existing `GIT` mock + bare-origin `new_repo` harness, incl. the load-bearing
  negatives: bare `#NNNN`, body-only mention, terminal-publish subject of an archived change).
