---
id: 171
slug: settle-a-reflow-tolerant-house-pattern-for-prose-anchored-gu
title: Settle a reflow-tolerant house pattern for prose-anchored guards
status: killed
priority: medium
type: refactor
created: 2026-07-30
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [167]
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

Change 0167's review rounds repeatedly found guards over skill prose that are **line-scoped** — patterns
like `grep -qE "foo[^.]{0,80}bar"` cannot cross a newline, so a purely cosmetic reflow of the guarded
sentence false-reddens them while changing nothing about the rule. Reviewers flagged this on
`controller: performs no final review of its own`, on its `^This build performs …` sibling, and on the
three dispatch-rule asserts added late in the change; each time it was correctly deferred as a
whole-family problem rather than patched one assert at a time.

The failure mode is the mirror of the one the same change spent three fix rounds closing: a
presence-anywhere grep is too loose (it survives deletion of what it guards), and a line-scoped prose
literal is too tight (it dies on a rewrap that preserves the rule). Both are symptoms of anchoring on
prose layout instead of on a stable syntactic feature.

## What changes

Audit the prose-anchored guards across `tests/` — starting with `tests/test_docket_build.sh`, which
carries the densest population — and settle a house pattern that is simultaneously reflow-tolerant and
deletion-sensitive. Candidate approaches worth comparing before choosing: normalizing whitespace out of
the haystack before matching (so a reflow is invisible), anchoring on a heading/bullet/marker structure
rather than a sentence, or splitting a single assert into a structural anchor plus a shorter within-line
literal. Whatever is chosen should be written down once and reused, not re-derived per assert.

## Out of scope

- Loosening any guard's bite. The completion bar stays mutation-testing in both directions: deleting the
  rule must redden, and a legitimate reword that preserves the rule must not.
- The `producer | grep -q` / `| head` shell-form normalization — that is its own separate cleanup.

## Why killed

Consolidated into #0253 at the 2026-08-07 backlog triage: the reflow-tolerant house pattern and #0233's stacked-gap ban are one idiom ruling; the triplicated flatten() helper is the shared starting point.
