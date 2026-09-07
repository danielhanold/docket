---
id: 354
slug: 'halt-report-authoring-writes-a-duplicate-run-halted-heading'
title: 'Halt-report authoring writes a duplicate Run halted heading, wedging docket change resume-halted'
status: 'in-progress'
priority: 'high'
type: 'fix'
created: '2026-08-26'
updated: '2026-09-07'
depends_on: []
stacked_on:
related: [343, 368]
discovered_from: [351]
adrs: []
spec: 'docs/superpowers/specs/2026-09-07-halt-report-authoring-writes-a-duplicate-run-halted-heading-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/halt-report-authoring-writes-a-duplicate-run-halted-heading'
pr:
blocked_by:
reconciled: false
claimed_at: '2026-09-07T14:52:31Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-07-halt-report-authoring-writes-a-duplicate-run-halted-heading-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-07-halt-report-authoring-writes-a-duplicate-run-halted-heading-design.md) |
<!-- docket:artifacts:end -->

## Why

A halt report must remain inside the single `## Run halted` section that recovery removes. Today the halt operation supplies that heading and a dated subheading, but accepts caller-authored report bodies containing additional H2 headings. A repeated halt heading makes `change.resume-halted` refuse the edit as ambiguous; other H2 headings split the report and can leave content behind after resume.

This was observed on change 0351 on 2026-08-26 and repaired manually. The current write boundary still permits the malformed report shape, and implement-next's authoring guidance does not clearly separate the operation-owned wrapper from the caller-owned body. Fix both so a successful halt write produces a report that sanctioned recovery can remove completely.

## What changes

- Make the halt operation the sole owner of the halt heading and date wrapper; clarify the report-body contract in the skill and request documentation.
- Reject report content that would create another structural H2 or leave an unterminated code fence, using the section editor's existing fence-aware rules. Return an actionable input diagnostic before any metadata effects; do not silently rewrite authored evidence.
- Preserve valid prose, subsections, and heading examples inside closed code fences.
- Verify malformed-input refusal, repeated halt replacement, and a complete halt-to-resume cycle that removes the report while preserving surrounding content and recovery safeguards.

## Out of scope

- Automatic repair of already-corrupted historical records.
- Pre-allocation workspace recovery, tracked separately in change 0368.
- Changes to dispatch attribution, claim semantics, or the halt/resume lifecycle and its acknowledgement, version, and workspace safeguards.
- Replacing the Markdown parser or tightening every general section-edit caller.
