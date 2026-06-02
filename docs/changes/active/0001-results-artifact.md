---
id: 1
slug: results-artifact
title: Change results artifact — linked, optional close-out file
status: proposed
priority: medium
created: 2026-06-02
updated: 2026-06-02
depends_on: []
related: []
adrs: []
spec: docs/superpowers/specs/2026-06-02-results-artifact-design.md
plan:
results:
trivial: false
branch:
pr:
blocked_by:
reconciled: false
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
