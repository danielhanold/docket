---
id: 358
slug: treat-empty-review-decision-as-no-decision
title: 'Treat empty-string reviewDecision as no-decision, not an invalid enum'
status: 'implemented'
priority: critical
type: fix
created: 2026-08-26
updated: '2026-08-27'
depends_on: []
stacked_on:
related: [347, 348, 356]
discovered_from: [348]
adrs: [97]
spec:
plan: 'docs/superpowers/plans/2026-08-26-treat-empty-review-decision-as-no-decision.md'
results:
trivial: true
auto_groomable:
branch: 'fix/treat-empty-review-decision-as-no-decision'
pr: 'https://github.com/danielhanold/docket/pull/242'
blocked_by:
reconciled: true
claimed_at: '2026-08-26T23:20:18Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Plan | [2026-08-26-treat-empty-review-decision-as-no-decision.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-26-treat-empty-review-decision-as-no-decision.md) |
| ADRs | [ADR-0097](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0097-pr-identity-is-verified-by-parsed-pr-number.md) |
<!-- docket:artifacts:end -->

## Why

Change 0348 enriched the exact-PR view to request `reviewDecision`, so open-PR
snapshots could populate `Approved`. But GitHub returns `reviewDecision: ""`
(an empty **string**, not JSON `null`) for a repo whose branch protection
requires a PR but **0 approvals** — exactly this repo's configuration.

`normalizeReviewDecision` (`internal/githubcli/pr.go`) only treats a `nil`
pointer as "no decision"; a non-nil pointer to `""` falls through to `default`
→ `errEnum("unrecognized pull-request reviewDecision enum")`. That error
propagates: `decodePullRequest` fails → `ViewPullRequest` errors → `ProbePR`
substitutes unknown facts → finalize reads `pr-unknown` and **halts**.

Because none of this repo's PRs carry a required-review decision, essentially
every PR now probes as `pr-unknown` and **`docket-finalize-change` is broken
repo-wide**. Discovered when finalizing change 356 (PR #241): the PR was
`OPEN`/`CLEAN` at the recorded head, yet finalize halted on `pr-unknown`.

## What changes

In `normalizeReviewDecision`, treat an empty-string `reviewDecision` identically
to `nil` — both mean "no required-review decision", never an invalid enum:

```go
if raw == nil || *raw == "" {
    return false, nil
}
```

Update the function's doc comment (which currently says only "null/absent are
false") to state that empty-string is also no-decision. Add a regression test
that an empty-string `reviewDecision` yields `Approved: false` with no error —
complementing the existing `TestViewPullRequestUnknownReviewDecisionFailsClosed`
(a genuinely unknown non-empty enum must still fail closed). Rebuild the binary
(`docket development install`) after merge so installed finalize matches source.

Once merged and rebuilt, re-run `docket-finalize-change 356`.

## Out of scope

- No change to how a real `APPROVED` / `CHANGES_REQUESTED` / `REVIEW_REQUIRED`
  decision is handled — only the empty/null equivalence.
- No change to `ProbePR`'s fail-closed posture for genuinely unknown enums.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-08-26

2026-08-26: Reconciled against origin/main. Verified the defect is still present: `normalizeReviewDecision` in `internal/githubcli/pr.go` returns `false,nil` only for a nil pointer, and a non-nil pointer to "" falls through to `default` → `errEnum`. Scope, ADR-0097 link, and related changes (347/348 done, 356 implemented) remain accurate. No scope change; proceeding as a trivial one-function fix plus regression test complementing the existing `TestViewPullRequestUnknownReviewDecisionFailsClosed`.
