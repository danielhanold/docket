---
id: 479
slug: 'ad-hoc-go-test-tags-integration-internal-app-hits-go-test-s'
title: 'Refuse an unfiltered integration-tagged run of internal/app before go test''s 10-minute timeout'
status: 'in-progress'
priority: 'low'
type: 'chore'
created: '2026-09-30'
updated: '2026-10-01'
depends_on: []
stacked_on:
related: [333, 434, 465, 466]
discovered_from: [472]
adrs: [108]
spec: 'docs/superpowers/specs/2026-10-01-ad-hoc-go-test-tags-integration-internal-app-hits-go-test-s-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'chore/ad-hoc-go-test-tags-integration-internal-app-hits-go-test-s'
pr:
blocked_by:
reconciled: false
claimed_at: '2026-10-01T12:01:31Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-01-ad-hoc-go-test-tags-integration-internal-app-hits-go-test-s-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-10-01-ad-hoc-go-test-tags-integration-internal-app-hits-go-test-s-design.md) |
| ADRs | [ADR-0108](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0108-bound-total-go-test-load-at-the-runner-and-isolate-real-proc.md) |
<!-- docket:artifacts:end -->

## Why

`go test -tags integration ./internal/app/` with no `-run` filter runs the whole integration corpus of `internal/app` in one process. That takes about 19 minutes, longer than go test's default 10-minute per-package timeout, so the run dies at 10 minutes with a goroutine-dump panic. Change 0472's build lost a verification step this way, and the command keeps appearing in plans: five so far, three of them dated 2026-09-28 to 2026-09-30.

The trace at grooming found that neither the suite runner nor the gate drive is affected. The suite runs this corpus only through prefix-filtered shard runners, each well under a minute. Only hand-typed and plan-prescribed runs hit the limit, and nothing tells the caller the supported forms until the 10 minutes are gone.

## What changes

`internal/app`'s integration-tagged test binary refuses an unfiltered run at go test's default timeout, right after compile, and prints the supported forms: run a shard runner, filter with `-run '^<Prefix>'`, or pass `-timeout 30m` for a deliberate whole run. The check lives in `internal/testsupport` next to the existing no-real-git guard and follows its build-tag split; only `internal/app` calls it.

`tests/README.md` gains a short section on running integration-tagged Go tests by hand. A cheap end-to-end test proves the refusal and turns red if the guard or its call is removed.

## Out of scope

Timeout changes to the suite, the shard runners, or the race gate (0465, ADR-0108); other packages' integration corpora; AGENTS.md and the learnings ledger; editing merged plans; weakening or skipping integration tests; and the closeout budget breach (0475).
