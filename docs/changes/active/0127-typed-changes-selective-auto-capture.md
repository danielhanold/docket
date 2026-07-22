---
id: 127
slug: typed-changes-selective-auto-capture
title: Typed changes — configurable taxonomy, selective auto-capture, and backlog filters
status: in-progress
priority: high
type: feat
created: 2026-07-22
updated: 2026-07-22
depends_on: []
related: [90, 91, 94, 124]
discovered_from: []
adrs: [12, 19, 45, 52]
spec: docs/superpowers/specs/2026-07-22-typed-changes-selective-auto-capture-design.md
plan:
results:
trivial: false
auto_groomable:
branch: feat/typed-changes-selective-auto-capture
claimed_at: 2026-07-22T20:28:56Z
pr:
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-22-typed-changes-selective-auto-capture-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-22-typed-changes-selective-auto-capture-design.md) |
| ADRs | [ADR-0012](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0012-docket-status-script-vs-model-boundary.md), [ADR-0019](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0019-global-config-fence-classification.md), [ADR-0045](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0045-auto-capture-is-best-effort.md), [ADR-0052](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0052-config-key-resolution-boundary.md) |
<!-- docket:artifacts:end -->

## Why

Auto-capture has preserved follow-up work, but it has also filled the needs-brainstorm queue with
narrow fixes, guards, documentation work, and chores that compete with product features. At design
time, 16 of 23 proposed changes carried discovery provenance. Docket cannot apply a better policy
because every backlog record is currently an undifferentiated “change.”

An explicit type gives the backlog a useful vocabulary and lets repositories choose which kinds of
discovered work are durable enough to auto-capture. The same field also makes backlog review
practical without changing Docket's lifecycle or replacing its portable Markdown board.

## What changes

- Add a configurable `type:` field with default types `chore`, `docs`, `feat`, `fix`, `refactor`,
  and `perf`; every new-change and mint path classifies new work.
- Replace scalar `auto_capture` with an intentionally breaking nested configuration containing
  `enabled` and a `types` selector that defaults explicitly to `all` or accepts a
  whole-list-replacing allowlist. All new settings resolve through built-in, user/global,
  repo-committed, and repo-local layers.
- Gate best-effort auto-capture by the discovered work's type and report excluded candidates.
- Add Type to active board tables and add report-only `docket-status --type` and `--priority`
  filters. Filters never narrow lifecycle work or canonical board generation.
- Provide a human-approved, deterministic one-time categorization pass for active changes. Migrate
  this repository's active backlog; never edit archived changes.

## Out of scope

- Interactive filtering inside Markdown, GitHub Projects, plugins, or GitHub mirror changes.
- Reclassifying archived records.
- Changing lifecycle states, readiness, selection order, or auto-capture's materiality bar and
  best-effort failure posture.

## Open questions

None. The configuration shape, layer semantics, migration posture, and filter boundaries are
settled in the linked spec.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-07-22 — reconcile before build

Design holds; scope unchanged. Verified against current `origin/main` and the metadata branch.

- **Assumptions confirmed.** No `type:`/`change_types` machinery exists anywhere in `scripts/` —
  the change builds on a clean slate. `AUTO_CAPTURE` is still the scalar resolved at
  `scripts/docket-config.sh` (single `lcl`/`yaml_get`/`gbl` chain, boolean-validated, emitted once),
  so the breaking scalar→map rewrite has exactly the blast radius the spec assumed.
- **Related work re-checked.** #0090 (discovery provenance), #0091 (auto-capture), and #0094
  (selection-order digest) are all `done` and merged, so `discovered_from:`, the mint path, and the
  `ready` digest line are live and this change extends rather than anticipates them. #0124
  (backlog triage) is still `proposed`/needs-brainstorm — complementary, not overlapping: this
  change supplies the type vocabulary #0124's triage pass would consume. No scope dropped.
- **New constraint folded in (post-dates the spec).** Change #0116 landed after this spec was
  written and single-sourced the board vocabularies into `scripts/lib/docket-frontmatter.sh`
  (`DOCKET_STATUSES*`, `DOCKET_PRIORITIES`, `DOCKET_PRIORITY_DEFAULT`, plus membership/rank
  helpers), and **ADR-0055** now requires an exhaustive vocabulary mapping to be pinned by
  array-backed set equality with a cardinality assert and mutation tests in *both* directions.
  The new change-type vocabulary and the `--type`/`--priority` filter validation must therefore be
  array-pinned against a single authoritative array and must not hand-enumerate tokens at each
  use site. The built-in taxonomy gets one authoritative declaration; the resolver's configurable
  effective list layers on top of it.
- **Existing-guard obligations.** ADR-0052 + `tests/test_docket_example_yml.sh` carry a
  classification manifest requiring every key documented in `.docket.example.yml` to resolve
  through the resolver and appear in its export block, so `change_types`,
  `auto_capture.enabled`, and `auto_capture.types` each need a manifest arm and the retired
  `AUTO_CAPTURE` arm must be removed in the same pass. ADR-0053 puts every README yaml fence in
  scope by default, so the README config examples are auto-guarded once edited.
- **Migration-set correction.** This change file already carries `type: feat` (written at
  authoring time), so #0127 is **not** in the untyped migration set. The spec's "including #0127"
  reads as "every active change ends up typed", not "assign #0127 a type during the backfill" —
  the helper's conflict-refusal rule makes re-assigning an already-typed record an error, and the
  backfill's input set is the untyped active changes only.
- **Test gate.** The suite is the whole `tests/*.sh` set run at the build gate (AGENTS.md), not
  only the spec's named cases; `tests/test_docket_status.sh` needs `GIT_EDITOR=true` in a
  non-interactive run, per the #0116 close-out.
