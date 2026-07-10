---
id: 53
slug: slim-convention-status-skills
title: Slim docket-convention + docket-status via progressive disclosure
status: proposed
priority: medium
created: 2026-07-10
updated: 2026-07-10
depends_on: [51]
related: [51, 54, 55]
adrs: [12]
spec: docs/superpowers/specs/2026-07-10-docket-skill-slimming-design.md
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
| Spec | [2026-07-10-docket-skill-slimming-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-10-docket-skill-slimming-design.md) |
| ADRs | [ADR-0012](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0012-docket-status-script-vs-model-boundary.md) |
<!-- docket:artifacts:end -->

## Why

The docket skill bodies have outgrown their token cost. `docket-convention` — loaded as blocking
Step 0 by every docket skill on every run — grows to ~380 lines / ~6,000 words (~8k+ tokens) once
change 0051 merges, exceeding the agentskills.io < 5,000-token recommendation ~1.7×, and far past
the "frequently-loaded skills should be small" guidance. `docket-status` (~185 lines) runs on
every implement cycle at a small pinned model. Research (2026-07-10, in the spec) identifies the
fixes: progressive disclosure into one-level-deep reference files, cutting change-history
narration that ADRs and git history already record, and deduplicating the close-out sequence
currently restated in four skills under "must not diverge" warnings.

## What changes

Behavior-neutral restructure of the two hottest skills — no contract semantics change:

- `docket-convention`: core contract stays inline (~190 lines / ~2,400 words); the Agent-layer
  deep-dive moves to `references/agent-layer.md`; a new `references/terminal-close-out.md`
  becomes the single source of the shared archive→re-render→publish→cleanup→board sequence with a
  per-caller failure-posture table. Provenance narration is cut (bare `(ADR-NNNN)` pointers
  remain where a why is load-bearing). A new "Step-0 preamble" section becomes the single source
  of the boilerplate every skill restates.
- `docket-status`: board Structure section + rendered example deleted (`render-board.sh` is the
  executable source); sweep steps rewired to the close-out reference; Step-0 preamble compressed;
  target ~100 lines / ~1,500 words. All steps stay explicit imperatives (small-model wrapper).
- Guardrails: kept section headings stay byte-stable + anchor grep-gate; references one level
  deep; TOC in any reference > 100 lines; post-refactor `docket-status` smoke run.

## Out of scope

- Any semantic change to convention, config resolution, lifecycle, or close-out behavior.
- Editing the other six skills — follow-ups #0054 (finalize) and #0055 (implement-next + small
  skills) carry those, gated on this change's reference files.
- New scripts, script-contract changes, or frontmatter `description:` rewrites.

## Open questions

- Whether the skill-layer roles section compresses enough inline or also warrants a reference
  file (decide at plan time against the ~190-line target).

## Reconcile log
