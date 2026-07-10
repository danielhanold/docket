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
spec: docs/superpowers/specs/2026-07-10-slim-remaining-skills-design.md
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
| Spec | [2026-07-10-slim-remaining-skills-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-10-slim-remaining-skills-design.md) |
<!-- docket:artifacts:end -->

## Why

Change #0053's categorization: `docket-implement-next` (~137 lines / ~2,900 words) restates the
`render-change-links.sh` regeneration litany four times and carries the Step-0/mode boilerplate;
the four small skills (`docket-new-change`, `docket-groom-next`, `docket-adr`,
`docket-auto-groom`) are lean but still carry the full Step-0 preamble and (new-change) a
duplicated kill sequence. Medium potential, medium risk — the implementer's repetition is partly
deliberate reinforcement for an autonomous agent.

## What changes

- `docket-implement-next` (~137 → ~95–100 L): Step-0 → the convention's *Step-0 preamble*
  pointer (the one-line config eval stays verbatim in the body); reconcile-kill rewired to
  `references/terminal-close-out.md` (caller posture only stays skill-side); a named
  **field-write rule** stated once in *Branch & metadata discipline* replaces the ×3
  `render-change-links.sh` litany — steps 4/6/7 become one-line pointers. Selection, claim CAS,
  SHA-compare push confirm, reconcile section, and *Best-effort board refresh* stay.
- `docket-new-change` (70 → ~55 L): Step-0 compression; proposed-kill → close-out reference,
  keeping the must-land Board pass posture and the nothing-to-clean-up note.
- `docket-groom-next` (77 → ~65 L), `docket-adr` (88 → ~78 L), `docket-auto-groom` (64 → ~58 L):
  Step-0 preamble compression; narration cuts.
- One named behavior delta: both kill paths adopt the reference's full sequence, gaining its
  step-2 Artifacts re-render (benign no-op for kills); the reference gains a one-line
  no-diff-is-success clarifier so an empty re-render never trips the skip-publish guard.
- Sentinels follow content (#0053 precedent): K3/K4 and the kill-litany asserts re-point to the
  reference; everything else stays anchored in place.

## Out of scope

- Any semantics change to selection, claim CAS, reconcile, build, review, or kill outcomes
  (beyond the named re-render harmonization).
- `docket-convention`, `docket-status` (#0053) and `docket-finalize-change` (#0054) — except the
  single clarifier line in `references/terminal-close-out.md`.
- Scripts, script contracts, frontmatter `description:` lines, agent wrappers.

## Open questions

## Reconcile log
