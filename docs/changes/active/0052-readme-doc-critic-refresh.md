---
id: 52
slug: readme-doc-critic-refresh
title: README doc-critic refresh — accuracy, structure, newcomer clarity (post-0051)
status: in-progress
priority: medium
created: 2026-07-09
updated: 2026-07-10
depends_on: [51]
related: [45, 46, 47, 50, 51]
adrs: [15, 19]
spec: docs/superpowers/specs/2026-07-09-readme-doc-critic-refresh-design.md
plan:
results:
trivial: false
auto_groomable:
branch: feat/readme-doc-critic-refresh
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-09-readme-doc-critic-refresh-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-09-readme-doc-critic-refresh-design.md) |
| ADRs | [ADR-0015](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0015-harness-portable-agent-config.md), [ADR-0019](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0019-global-config-fence-classification.md) |
<!-- docket:artifacts:end -->

## Why

The README has grown by accretion — each change bolted its story onto the existing text — and
it now fails a technical-documentation review on three fronts. **Accuracy:** change 0051
(unmerged, in-progress) replaces the committed-wrapper agent-generation model with gitignored
all-local generation and adds the `.docket.local.yml` fourth config layer, invalidating the
agent story in four places (Install, the `.docket.yml` block, Global config, the entire
"Tuning an agent's model & effort" section); smaller claims like `finalize.require_pr_approval`
disagree between README and convention. **Structure:** the differentiator (the reconcile pitch)
is buried after install and config; the 70-word lead assumes docket's own vocabulary; no TOC.
**Newcomer clarity:** no prerequisites, jargon before definition, and no daily-use walkthrough —
the README never shows what you actually type after installing.

## What changes

Per the linked spec: a critique-driven editorial rewrite of `README.md`, executed after 0051
merges (the spec's critique is written against today's text plus 0051's known deltas; the
build's reconcile + accuracy audit re-validates against the merged result).

- **Accuracy audit** — extract every testable claim (commands, paths, config keys, behaviors)
  into a checklist; verify each against the merged codebase, script contracts, and
  docket-convention; fix or cut. The checklist ships in the results doc as the change's
  verification surface.
- **Restructure** to the spec's target outline (*what → why → try it → configure → internals →
  reference*): rewritten plain-language lead, TOC, "How it works", "Why docket" with the
  reconcile pitch promoted, Install with prerequisites, a new **Quickstart: the daily loop**
  section, one consolidated four-layer Configuration section introducing `.docket.local.yml`,
  docket-mode internals, the post-0051 agent-tuning story, the eight-skills table, Status.
- **Prose pass** — humanizer-standard cleanup of the rewritten text.

Single PR touching `README.md` only.

## Out of scope

- Any length cap — concision was explicitly deselected as a goal; depth stays where clarity
  needs it.
- Changes to `docket-convention`, skill bodies, script contracts, or any doc other than the
  repo-root `README.md` (per-directory READMEs untouched).
- New features or behavior changes of any kind.

## Open questions

- Does 0051's own targeted README rewrite (in its scope, unbuilt at proposal time) land text
  the audit keeps, or does the outline absorb it wholesale? Resolve at reconcile.
- `finalize.require_pr_approval`: README documents it, convention's schema omits it — which
  side is stale? Resolve during the accuracy audit against the scripts.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
