---
id: 154
slug: audit-skill-bodies-for-the-stale-restatement-class-change-01
title: 'Remove stale Bash instructions and duplicated runtime contracts from Docket skills'
status: 'implemented'
priority: medium
type: docs
created: 2026-07-28
updated: '2026-09-09'
depends_on: []
related: [111, 144, 157, 159, 257, 363, 370, 372, 377, 394, 399]
discovered_from: [145]
adrs: [3, 12, 54, 99, 104, 109]
spec: 'docs/superpowers/specs/2026-09-08-audit-skill-bodies-for-the-stale-restatement-class-change-01-design.md'
plan: 'docs/superpowers/plans/2026-09-08-audit-skill-bodies-for-the-stale-restatement-class-change-01.md'
results: 'docs/results/2026-09-08-audit-skill-bodies-for-the-stale-restatement-class-change-01-results.md'
trivial: false
auto_groomable: true
branch: 'docs/audit-skill-bodies-for-the-stale-restatement-class-change-01'
pr: 'https://github.com/danielhanold/docket/pull/294'
blocked_by:
reconciled: true
claimed_at: '2026-09-09T02:15:38Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-08-audit-skill-bodies-for-the-stale-restatement-class-change-01-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-08-audit-skill-bodies-for-the-stale-restatement-class-change-01-design.md) |
| Plan | [2026-09-08-audit-skill-bodies-for-the-stale-restatement-class-change-01.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-09-08-audit-skill-bodies-for-the-stale-restatement-class-change-01.md) |
| Results | [2026-09-08-audit-skill-bodies-for-the-stale-restatement-class-change-01-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-09-08-audit-skill-bodies-for-the-stale-restatement-class-change-01-results.md) |
| ADRs | [ADR-0003](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0003-convention-reference-loading.md), [ADR-0012](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0012-docket-status-script-vs-model-boundary.md), [ADR-0054](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0054-cross-reference-anchor-style.md), [ADR-0099](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0099-one-metadata-topology-for-go-v1.md), [ADR-0104](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0104-the-capability-catalog-is-the-authoritative-executable-cli-s.md), [ADR-0109](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0109-docket-schema-is-a-separate-reflected-payload-schema-surface.md) |
<!-- docket:artifacts:end -->

## Why

Agents currently receive contradictory instructions from Docket's skills. The Go runtime uses typed operations and structured reports, but parts of the skills still describe deleted Bash scripts, the former line-oriented status report, an active GitHub board mirror, and retired configuration behavior. These copies can misdirect an agent even when the implementation's tests pass.

Change 0145 exposed the underlying problem: copying a runtime-owned list or explanation into a skill creates another place that can drift. The 2026-09-08 review confirmed that the wider cleanup remains necessary, while the August design's Bash targets and guard exemptions are obsolete. This regroom replaces that design with an audit of today's installed skill instructions and their current Go owners.

## What changes

One documentation PR audits all maintained Markdown under `skills/`, including references and templates, and removes stale runtime instructions and unnecessary copies of runtime contracts.

Prefer deletion plus a usable reference, then compression to the judgment the caller owns; retain an enumeration only when it is necessary and protected against a current owner. Use the capability/schema channels for runtime discovery, and verify configuration claims against the Go resolver and shipped reference. Preserve the authority checks, completion checks, and distinct failure behavior that agents need.

The linked replacement spec commits the current seed repairs: status's obsolete report grammar and sweep narrative; missing Bash references; the disabled-board contradiction; active GitHub-mirror/write-back claims; the convention's copied configuration rules, obsolete main-mode advice, and stale counts. The convention's config sketch is included, with one overlapping item from deferred 0257 explicitly accounted for.

The build records the full inventory, updates affected Go guards without weakening their surviving invariants, regenerates installed assets, verifies reference usability, and runs the full configured suite. Existing metadata status, type, and priority remain appropriate: proposed, docs, medium.

## Out of scope

Runtime, CLI/schema/config behavior, defaults, permissions, and feature support; restoring Bash, GitHub mirroring, terminal publication, or main-mode; a generic prose-lint framework; unrelated documentation rewrites; and modernizing historical specs, archived records, merged build artifacts, or Accepted ADR bodies.

Only the configuration-sketch item shared with deferred 0257 is covered. Its remaining rationale/guidance work stays deferred. No implementation plan or code is produced by this regroom.

## Reconcile log

### 2026-09-08

2026-09-08: Reconciled against HEAD d7363492 on main after claim. The spec was regroomed earlier today (2026-09-08) with a fresh necessity review, so it already reflects current reality: the Go migration owns the runtime, the Bash/board-mirror/main-mode surfaces are retired, and the discovery channels are capability + schema + config operations. Confirmed the in-scope population is the 29 git-tracked Markdown files under skills/ (discovered from the checkout, not hard-coded). Predecessors 0363/0370/0372/0377/0394/0399 are done, so depends_on stays empty and no stacked base is needed; related/discovered_from/adrs relations remain accurate. No scope or relation adjustment required. Seed defects are verified at build time against their named current owners (internal/app/status_result.go, status_human.go, planning.go, derived_views.go, internal/config/schema.go, capability.go, .docket.example.yml) rather than reapplying the obsolete August edits; deferred 0257's single sketch-comment item is folded in per the spec, with its remaining rationale work left deferred.

## Finalize blocked

### 2026-09-09 — attempt 20260909T164355Z-77849d583f3f

<!-- attempt:20260909T164355Z-77849d583f3f -->

- Reason: resolver-capability-missing
- Head: dc84a95dfb6dd26ccad8ff6ae2b3ab0bdad6192c
- PR: #294
- Comment: https://github.com/danielhanold/docket/pull/294#issuecomment-5605464417

Remedy: Update or reinstall Docket so its validated capability catalog provides finalize.resolver-reserve, then retry finalize by explicitly naming change 154. Keep the PR open; do not resolve or merge this conflict manually.
