---
slug: metadata-branch-invisible-to-suite
hook: "Repo tests can only see the integration branch — verify metadata-branch artifacts at build time and record it in the results file."
topics: [testing, metadata-branch, docket]
changes: [6]
created: 2026-06-12
updated: 2026-06-12
promotion_state: retained
promoted_to:
---

## Apply
When specifying tests for metadata-branch artifacts, verify them at build time and record in
the results file instead — repo tests can only see the integration branch.

## War story
- 2026-06-12 (#6, PR #8) — The spec asked for a test asserting a metadata-branch file exists,
  but the suite runs against the integration-branch checkout where that file never lives.
