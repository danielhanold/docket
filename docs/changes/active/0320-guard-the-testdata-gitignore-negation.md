---
id: 320
slug: guard-the-testdata-gitignore-negation
title: 'Guard the testdata gitignore negation'
status: proposed
priority: medium
type: chore
created: 2026-08-13
updated: 2026-08-13
depends_on: []
stacked_on:
related: []
discovered_from: [305]
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

**Trigger** — change 0305's review (important finding #3) found that the repo-wide `.docket.local.yml`
ignore was silently swallowing the repository-local config fixtures under `testdata/repositories/`,
which existed only because of a one-time `git add -f`. The durable fix landed as a nested
`testdata/repositories/.gitignore` negation kept outside the managed docket gitignore block. Its
effectiveness was confirmed by two **manual** `git check-ignore` probes and by nothing else.

**Opportunity** — a committed guard that reproves the negation holds: assert that a newly-created
file matching the swallowed pattern under `testdata/repositories/` is reported un-ignored by
`git check-ignore`, and that the deciding rule is the nested negation rather than an incidental
user-global excludesfile. Today no test fails if the ignore layout changes, if the managed block is
rewritten so it hoists past the negation, or if the nested file is deleted.

**Independent value** — the property is about the repository's fixture-tracking discipline, not
about 0305's Go code. It holds with 0305 reverted and protects every fixture tree added afterward:
the failure mode is a *new* sibling fixture silently never being added, which no existing test can
see because the tree it reads still looks complete.

**Boundary** — one guard test over the ignore behavior of `testdata/repositories/`, mutation-proven
(delete the nested negation, watch it redden). It does not restructure the managed gitignore block,
does not add a general-purpose ignore linter for the rest of the repo, and does not touch the
fixtures themselves.

**Reason for deferral** — 0305 is merged; its branch is gone. The residual was recorded in its
results file as knowingly unprobed rather than undetectable, which is exactly the shape this repo's
own `residual-is-for-undetectable-not-unprobed` finding says should become follow-up work.
