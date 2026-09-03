---
id: 321
slug: render-artifact-backlink-sh-eats-artifact-tails-on-marker-sh
title: 'render-artifact-backlink.sh eats artifact tails on marker-shaped literals — anchor whole-line, validate balance, refuse'
status: 'killed'
priority: medium
type: fix
created: 2026-08-13
updated: '2026-09-03'
depends_on: []
stacked_on:
related: []
discovered_from: [306]
adrs: []
spec:
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
<!-- docket:artifacts:end -->

## Why

**Trigger** — during change 0306's build, `render-artifact-backlink.sh` truncated the committed implementation plan from Task 8 to EOF: its `grep -qF` + awk `index()` substring match fired on a marker-shaped example literal inside the plan's authored body, treated it as the real backlink block, and consumed everything after it. The branch survives only via a string-concatenation workaround in that one plan file; the 0306 review flagged the absent fix and record as an important finding.
**Opportunity** — anchor the renderer's marker match to a whole-line comparison (column-zero exact marker lines, the same grammar the Go document layer now enforces) and validate marker order/balance before rewriting, refusing and leaving the file untouched on an unclosed or ambiguous range — the always-in-context managed-block rule applied to the one writer that currently violates it.
**Independent value** — every spec, plan, or results artifact that quotes an annotated `docket:backlink:start` line is one close-out re-render away from silent tail loss; the renderer runs on every artifact at stamp time and again at terminal-publish, with 0306 reverted or not.
**Boundary** — `scripts/render-artifact-backlink.sh` (match anchoring, balance validation, refuse-on-invalid) plus a regression test with a marker-shaped-literal fixture; deliberately leaves alone the Go document layer, other renderers, and any migration of call sites.
**Reason for deferral** — change 0306 is scoped "no Bash production change"; fixing shipped bash tooling on its branch would expand that scope, and the defect predates the branch.

## Why killed

Backlog review 2026-09-02 (Bash→Go migration): already fixed in Go — internal/document's marker scan is fence-aware, anchors column-zero whole lines, and refuses on unbalanced markers (malformed-markers).
