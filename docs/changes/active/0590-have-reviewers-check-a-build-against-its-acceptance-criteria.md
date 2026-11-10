---
id: 590
slug: 'have-reviewers-check-a-build-against-its-acceptance-criteria'
title: 'Have reviewers check a build against its acceptance criteria when the change has them'
status: 'proposed'
priority: 'medium'
type: 'feat'
deliverable: 'pr'
created: '2026-11-10'
updated: '2026-11-10'
depends_on: []
stacked_on:
related: [581]
discovered_from: [582]
adrs: []
spec:
plan:
results:
auto_groomable:
hold:
branch_prefix:
branch:
pr:
ticket_provider:
ticket_id:
ticket_url:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

Change 582 gave changes an optional `## Acceptance criteria` section, and left reviewers checking a build against it out of scope. Today the criteria are written down but nobody reads them at review: implement-next's review payload carries the change's title, `## Why`, and `## What changes`, not its acceptance criteria. A build can open a PR that misses a criterion the human wrote, and the first person to notice is the human.

## What changes

- When a change has an `## Acceptance criteria` section, implement-next passes it to the reviewer along with the rest of the change's scope.
- The `docket-review` skill checks the branch against each criterion and reports any criterion the branch does not meet as a normal review finding, using the existing severities and return schema. That finding goes through the existing review-fix flow like any other.
- A change without the section is reviewed exactly as today.

Left to grooming: what severity an unmet criterion gets; whether results-artifact mode checks the results against the criteria too; and whether the reviewer reports criteria it could not judge from the diff alone.

## Acceptance criteria

A change with acceptance criteria gets a review payload that includes them; a branch that misses a criterion gets a review finding naming it; a change without the section gets the same payload and review as before.

## Out of scope

Making acceptance criteria a build gate or a new halt: an unmet criterion is a review finding, never a block of its own. Requiring changes to have acceptance criteria. Checking criteria anywhere other than review, such as at finalize.
