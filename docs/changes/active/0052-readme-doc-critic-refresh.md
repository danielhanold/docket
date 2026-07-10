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
adrs: [15, 19, 20]
spec: docs/superpowers/specs/2026-07-09-readme-doc-critic-refresh-design.md
plan:
results:
trivial: false
auto_groomable:
branch: feat/readme-doc-critic-refresh
pr:
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-09-readme-doc-critic-refresh-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-09-readme-doc-critic-refresh-design.md) |
| ADRs | [ADR-0015](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0015-harness-portable-agent-config.md), [ADR-0019](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0019-global-config-fence-classification.md), [ADR-0020](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0020-generated-agent-artifacts-machine-local.md) |
<!-- docket:artifacts:end -->

## Why

The README has grown by accretion — each change bolted its story onto the existing text — and
it now fails a technical-documentation review on two remaining fronts (a third, accuracy, was
largely resolved when 0051 merged — see the Reconcile log). **Structure:** the differentiator
(the reconcile pitch) is buried after install and two config sections; the 70-word lead assumes
docket's own vocabulary; no TOC. **Newcomer clarity:** no prerequisites, jargon before
definition, and no daily-use walkthrough — the README never shows what you actually type after
installing. **Accuracy** remains as a full-text audit: 0051's own targeted rewrite landed the
post-0051 agent/config story (Install bullet, `.docket.yml` block, four-layer Global config +
`.docket.local.yml` sections, the rewritten "Tuning an agent's model & effort"), so the audit
verifies rather than rewrites those sections — the rest of the text still gets every testable
claim checked.

## What changes

Per the linked spec: a critique-driven editorial rewrite of `README.md` against the post-0051
merged text on `origin/main` (0051 merged 2026-07-10, PR #60; the spec critique was written
against the pre-0051 text plus 0051's known deltas — the reconcile addendum in the spec maps
which findings 0051 already resolved).

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

- ~~Does 0051's own targeted README rewrite land text the audit keeps?~~ **Resolved at
  reconcile (2026-07-10):** yes — 0051 landed current, keepable text for the agent/config
  story; the build critiques and refines it in place per the spec's build method, rather than
  re-deriving it.
- ~~`finalize.require_pr_approval`: README documents it, convention's schema omits it — which
  side is stale?~~ **Resolved at reconcile (2026-07-10):** the README is correct — the key is
  real, implemented by `docket-finalize-change` (change 0021, ADR-0011). The convention
  schema's omission is the stale side; fixing docket-convention is out of scope here
  (candidate follow-up stub).

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

- **2026-07-10** — Reconciled against post-0051 `origin/main` (README @ `d7f4a96`, 336 lines).
  (1) 0051 merged 2026-07-10 (PR #60) and its targeted README rewrite landed all four accuracy
  items from the spec's critique §1 — Install `sync-agents.sh` bullet, the `.docket.yml`
  block, the four-layer Global config + `.docket.local.yml` sections, and the fully rewritten
  "Tuning an agent's model & effort" — so the accuracy front narrows from "rewrite invalidated
  sections" to "audit + refine current text". Structure (§2) and newcomer (§3) critiques
  verified still fully valid (70-word lead, no TOC, reconcile pitch after Install + two config
  sections, no prerequisites, no daily-loop walkthrough). (2) Both open questions resolved
  (see above); `require_pr_approval` stays in the README. (3) ADR-0020 (generated agent
  artifacts are machine-local; supersedes ADR-0017) now underpins the agent story — added to
  `adrs:`; the build's audit should check the tuning section against ADR-0020, not ADR-0017.
  (4) Concurrent change 0053 (skill slimming) is in-progress on the `docket` branch — unmerged,
  touches skill bodies only; the audit baseline stays `origin/main` at build time, but the
  README's by-name pointers to docket-convention sections ("Agent layer", "Skill layer") should
  be re-verified during the audit in case 0053 merges first. Scope unchanged: single PR,
  `README.md` only.
