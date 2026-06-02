---
id: 1
slug: results-artifact
title: Change results artifact — linked, optional close-out file
status: in-progress
priority: medium
created: 2026-06-02
updated: 2026-06-02
depends_on: []
related: []
adrs: []
spec: docs/superpowers/specs/2026-06-02-results-artifact-design.md
plan: docs/superpowers/plans/2026-06-02-results-artifact.md
results:
trivial: false
branch: feat/results-artifact
pr:
blocked_by:
reconciled: true
---

## Why

Implementing a change sometimes produces a close-out document — what the human must hand-verify at the merge gate, what the build discovered, what follow-ups it spawned. docket has no concept of such a file, so when Markhaus change #0004 (onboarding) produced one it landed orphaned at `markhaus/docs/2026-05-31-onboarding-results.md`: linked to nothing, and loose in top-level `docs/` where such files accumulate (markhaus already has several from the pre-docket era). The original design only addressed this obliquely — "fold results into the body" (2026-05-30 spec, line 401) — which conflicts with the rule that a change body is a PM-altitude proposal, not a home for build evidence or QA checklists.

## What changes

Introduce the **results artifact**: an optional close-out file modelled as a true twin of the plan — a feature-branch build artifact linked from the change by a new `results:` frontmatter field.

- New `results:` manifest field (single path), slotted after `plan:` with a parallel comment.
- New `results_dir` knob in `.docket.yml` (default `docs/results`), matching how `changes_dir`/`adrs_dir` are exposed; the file lives at `docs/results/<YYYY-MM-DD>-<slug>-results.md`.
- **Optional, written only when warranted** — a concrete trigger (human verification steps, findings, or follow-ups). Otherwise skipped; the PR + green CI are the receipt. Keeps the document count low.
- Authored in the feature worktree (like the plan), merges with the PR; the `results:` field is set in the main tree on `metadata_branch`. Never archived; linked by path; resolves on `main` after merge (same as `plan:`).
- A lean three-section template (`Verify (human)` / `Findings` / `Follow-ups`), shipped as `results-template.md`.

Touch-points: the synced convention block (manifest + layout + `.docket.yml` + branch-model line), `docket-implement-next` (conditional close-out step), `docket-finalize-change` (post-merge append note), `docket-status` (tolerate unresolved `results:` on `implemented`), the 2026-05-30 design spec (reconcile line 401), README, and the new template. Full detail in the linked spec.

## Out of scope

- Retrofitting other repos — migrating Markhaus's existing orphaned `*-results.md` files into `docs/results/` is a separate markhaus-side cleanup.
- Directory-per-change restructuring (rejected) and folding-results-into-the-body (superseded).
- Multi-file `results:` lists — forward-compatible but not implemented.

## Open questions

None outstanding — naming, cardinality, home, and the optional/triggered model are resolved in the spec.

## Reconcile log

**2026-06-02:** Reconciled at claim time — spec and change were authored the same day and `origin/main` has not advanced since, so this is a currency check, not a rewrite. Verified every spec §8 touch-point anchor against the live files:
- `docket-status` broken-link health check (its `plan:`-on-`implemented` tolerance) → extend the same tolerance to `results:`.
- Design-spec **line 401** ("results folded into the body") is a *historical description of the one-time Markhaus migration*, not a go-forward intent — so the reconcile action is to **add** a go-forward note + a §3 locked decision, not to rewrite the historical line.
- `docket-finalize-change` per-change steps → add a short post-merge "append interactive outcomes / late findings to the results file on `main`" note.
- README config block (`changes_dir`/`adrs_dir`) → add `results_dir`; mention the results artifact where artifacts are enumerated.
- Added **`change-template.md`** (add the `results:` field) to scope explicitly, alongside the new `results-template.md`.

Scope otherwise unchanged; no work shipped elsewhere to drop. Build approach: encode the requirements as content/sync **assertions** (TDD-style for a docs+invariant change) — a real "convention blocks are in sync" check and a "`results:`/`results_dir` present across all five skills" check — then edit the canonical convention block, run `sync-convention.sh` to propagate, and add the template.
