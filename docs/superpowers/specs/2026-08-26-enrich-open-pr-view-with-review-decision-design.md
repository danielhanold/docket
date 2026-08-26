<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0348 — Enrich the exact-PR view with reviewDecision so open-PR snapshots populate Approved](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-26-0348-enrich-open-pr-view-with-review-decision.md)**
<!-- docket:backlink:end -->

# Exact-PR approval observation — design (change 0348)

## Problem

Change 0347 moved finalize's open-PR probe from branch discovery to
`githubcli.ViewPullRequest`, an exact read of the PR number recorded in the change manifest. That
adapter currently requests the standard PR fields but not GitHub's `reviewDecision`. Its normalized
`PullRequest` therefore carries no approval fact, and `githubFinalizeProber` necessarily leaves
`PRFacts.Approved` false for every open PR.

This is conservative, but structurally wrong: a repository with
`finalize.require_pr_approval: true` cannot distinguish an approved PR from an unapproved one on the
automatic finalize path. Change 0347 deliberately preserved identity checking despite that gap;
this change fills the missing approval observation without reopening exact-PR identity or changing
approval policy.

## Decision

Enrich only the exact-number view with GitHub's nullable `reviewDecision` field and expose the
result as `PullRequest.Approved`.

The mapping is strict:

| GitHub `reviewDecision` | `Approved` |
|---|---|
| `APPROVED` | `true` |
| `REVIEW_REQUIRED` | `false` |
| `CHANGES_REQUESTED` | `false` |
| `null` | `false` |

An unknown non-null enum is invalid external state and fails the read. It is never silently folded
into either approval outcome. This follows the adapter's existing enum posture: new or malformed
GitHub vocabulary is unobservable until Docket understands it, and unobservable state never
authorizes a merge.

`Approved` means an affirmative GitHub review decision, not "GitHub would currently permit a
merge." In particular, `null` does not become true merely because a repository has no required
review rule. Docket's `finalize.require_pr_approval` policy decides whether the boolean matters; an
explicitly named finalize run remains the existing human authorization override.

## Adapter shape

Add a view-specific field set consisting of the existing standard PR fields plus
`reviewDecision`, and use it only in `ViewPullRequest`. The shared list/create/edit field set stays
unchanged. Those other operations publish or locate PRs and do not need approval state; widening
all of them would enlarge the contract and alter snapshots unrelated to this fix.

Extend the JSON projection with a nullable review-decision field and extend the normalized
`PullRequest` with `Approved bool`. The common decoder applies the strict mapping when the field is
present. A missing field and JSON `null` both yield false, which preserves the standard-field
callers while giving the exact view the intended null behavior. A present unknown value returns a
typed invalid-state failure and never a partially populated PR.

Keep `Approved` outside `computeVersion`. That version is the compare-and-swap token used by
Docket's PR-writing paths, whose standard list/create/edit snapshots deliberately do not request
review state. Including approval would make the same approved PR produce incompatible version
tokens depending on whether it came from an exact view or a standard write-path read. Review state
is read-only gate evidence: finalize reloads it directly before effects rather than authorizing a
review mutation through this token. Update the version documentation to name this boundary.

## Finalize propagation

`githubFinalizeProber.ProbePR` copies `pr.Approved` into `domain.PRFacts.Approved` for a clean open
or closed exact view. The existing merged reprobe remains unchanged; merged recovery no longer
needs approval to authorize the already-completed effect.

No domain classification or approval-gate predicate changes. The existing behavior remains:

- automatic finalize with `finalize.require_pr_approval: true` skips any exact PR whose observed
  decision is not `APPROVED`;
- identity failures continue to outrank approval failures;
- an explicit id supplies the existing attended human authorization and overrides only the
  approval-required/finalize-blocked skips;
- probe or decode failures remain unknown and authorize no effect.

Remove stale source and test comments claiming that production can never populate approval. Keep
the identity-before-approval regression case, but describe its `Approved: false` input as an
intentional unapproved fixture rather than an adapter limitation.

## Verification

Adapter tests must prove:

- the exact-number `gh pr view` invocation requests `reviewDecision`;
- `APPROVED` maps to true;
- `REVIEW_REQUIRED`, `CHANGES_REQUESTED`, and null each map to false;
- an unknown non-null decision returns a typed invalid-state failure and the zero PR;
- standard list/create/edit decoding remains valid when the field is absent;
- changing review state alone does not change the write-CAS version token.

App and domain tests must prove:

- `githubFinalizeProber` propagates true and false approval observations into `PRFacts`;
- an approved open exact PR can pass the existing approval conjunct;
- known non-approved/null decisions still produce the existing `approval-required` outcome when
  policy requires approval;
- an identity mismatch still outranks approval for an intentionally unapproved PR.

Mutation-test the population seam: removing `Approved: pr.Approved` from the prober must redden a
test. Run the full suite through the configured build gate, not only the focused Go packages.

## Out of scope

- Changing exact-PR-number identity, PR-reference parsing, or branch/head reconciliation governed
  by ADR-0097 and change 0347.
- Exposing the raw review-decision enum through the domain or command protocol.
- Querying branch-protection rules or redefining "approved" as "merge currently permitted."
- Changing `finalize.require_pr_approval`, explicit-id authorization, mergeability, or finalize
  selection policy.
- Adding approval facts to list/create/edit PR operations or allowing Docket to author reviews.
- Adding a new ADR; this is a localized adapter completion under existing identity and consent
  policy.

## Settled decisions

The human approved the strict mapping: only `APPROVED` is true; `REVIEW_REQUIRED`,
`CHANGES_REQUESTED`, and null are false. Unknown non-null vocabulary fails closed. The exact view
alone gains the field, finalize consumes the resulting boolean through its existing policy, and
review state does not join the PR write-CAS token.

## Open questions

None remain.
