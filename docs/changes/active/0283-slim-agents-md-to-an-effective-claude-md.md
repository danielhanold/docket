---
id: 283
slug: slim-agents-md-to-an-effective-claude-md
title: 'Slim AGENTS.md to an effective, lean always-in-context file'
status: proposed
priority: medium
type: docs
created: 2026-08-09
updated: '2026-09-07'
depends_on: []
related: [154, 257, 263]
discovered_from: []
adrs: [41, 54, 71, 104, 111]
spec: 'docs/superpowers/specs/2026-09-07-slim-agents-md-to-an-effective-claude-md-design.md'
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
| Spec | [2026-09-07-slim-agents-md-to-an-effective-claude-md-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-07-slim-agents-md-to-an-effective-claude-md-design.md) |
| ADRs | [ADR-0041](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0041-learnings-findings-directory-and-promotion-valve.md), [ADR-0054](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0054-cross-reference-anchor-style.md), [ADR-0071](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0071-writer-guarantees-yaml-validity-by-construction.md), [ADR-0104](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0104-the-capability-catalog-is-the-authoritative-executable-cli-s.md), [ADR-0111](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0111-run-gate-attribution-binds-a-dispatch-to-its-successful-clai.md) |
<!-- docket:artifacts:end -->

## Why

AGENTS.md is loaded into every session, but its authored prefix mixes operational safeguards with incident narratives and references to deleted Bash tooling. The 2026-09-07 review measured 129 lines and 1,409 words. Repository guards cover bounded source populations and cannot replace instructions that guide ad hoc agent commands before a test runs.

The user requested a fresh design and chose to keep operational safeguards inline while shortening explanations and removing obsolete references. This replaces the August design's guard-backed rule-pruning policy.

## What changes

Replace the old design with the linked 2026-09-07 spec. One documentation PR compresses the unmanaged AGENTS.md prefix while preserving every existing obligation, actionable exception, and gate outcome.

- Keep all five shell safeguards, frontmatter/marker rules, guard discipline, whole-suite and budget-report obligations, cross-reference policy, historical-record exception, and post-merge reinstall rule inline.
- Remove obsolete harvest, mint-stub.sh, and Bash guard references; point to current owners where a pointer helps.
- Preserve the complete managed dispatch/run-gate block byte-for-byte and keep CLAUDE.md as its existing symlink alias.
- The concrete draft reduces unmanaged text from 998 to 499 words (approximately 1,409 to 910 for the whole file). These are draft measurements, not a quota; preserving meaning wins.
- Verify every original obligation against the final wording, run existing relevant guards and the configured full build gate, and update existing test anchors only where necessary.

No deferred guard work is a prerequisite. Change 0154's skill audit and 0366's attended release remain independent.

## Out of scope

Removing or relocating operational safeguards; editing the managed dispatch block or its generator; new guard machinery or word-budget systems; implementing deferred 0263 or absorbing 0257's additional frontmatter guidance; changing skill workflows, promotion policy, learning records, Accepted ADRs, frozen fixtures, or runtime behavior. Grooming stops at the spec; implementation is a later run.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
