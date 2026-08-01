---
id: 190
slug: close-the-build-evidence-value-gap-a-post-gate-results-commi
title: "Close the build-evidence value gap: a post-gate results commit always defeats finalize's suite skip"
status: proposed
priority: medium
type: feat
created: 2026-08-01
updated: 2026-08-01
depends_on: []
related: []
discovered_from: [170]
adrs: [66]
spec: docs/superpowers/specs/2026-08-01-close-the-build-evidence-value-gap-a-post-gate-results-commi-design.md
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-01-close-the-build-evidence-value-gap-a-post-gate-results-commi-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-01-close-the-build-evidence-value-gap-a-post-gate-results-commi-design.md) |
| ADRs | [ADR-0066](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0066-docket-owns-the-review-role-suite-runs-in-the-build-gate.md) |
<!-- docket:artifacts:end -->

## Why

Change 0170's build-evidence chain lets `docket-finalize-change` skip its post-rebase suite run
only when the rebase was a no-op AND the PR body's evidence block is green AND its `head_sha`
equals the branch HEAD being merged. That third condition is exact SHA equality, which is what
makes the predicate safe.

But `docket-implement-next` Step 6.5 commits the `results:` file **on the feature branch** after
the build gate has already minted the evidence. Any such post-gate commit moves HEAD, so the
`head_sha` no longer matches and the skip never fires. The whole-branch review measured the
frequency against this repo's own history: roughly 73% of archived changes carry a results file.
So the headline benefit — one full-suite run on the clean path — is inert on the majority path.

This is **not a safety bug**: the predicate fails toward running, which is the correct posture,
and 0170 documents the caveat honestly in both Step 7 and the README rather than hiding it. It is
a value gap, deliberately left open rather than closed in haste.

## What changes

Extend `docket-finalize-change`'s post-rebase suite-skip predicate (from change 0170) with a
narrow **docs-only ancestor** disjunct: the skip fires when the rebase is a no-op AND the PR body's
evidence block is green AND (`head_sha` equals the branch HEAD, as 0170 ships — **or** `head_sha` is
a strict ancestor of HEAD and every path changed in `head_sha..HEAD` lies under the repo's
`<results_dir>/`, the tree Step 6.5 commits post-gate). Anything else — a missing/malformed block,
a non-ancestor SHA, any changed path outside the allowlist — runs the suite exactly as today; the
"fails toward running" posture and the loud one-line skip log survive unchanged.

The consumer-side extension is chosen over the two alternatives because re-testing to earn a skip
never reduces the run count: a producer-side **re-mint** of the evidence at Step 7 is net-neutral
on the common path and costs an extra run when the base moved (gate + re-mint + finalize), and a
producer-issued **attestation** field is strictly weaker than finalize deriving the delta from git
state at skip time. Safety of the allowlist is **per-repo, verified, and guarded**: this change
adds a live guard test asserting no suite component reads `<results_dir>/` as content (this repo's
suite is hermetic to it by construction), with a build-time degrade-off rule — if the verification
cannot be established at build reconcile, the extension ships off (0170's equality-only predicate).

Design detail, the smuggling-vector enumeration, the guard contract, and the ripple list live in
the linked spec. The skip's trust boundary change is recorded as a dated Update note on ADR-0066
(this change's `adrs:`).

## Out of scope

- Weakening any other condition of the skip predicate.
- Changing where the evidence block lives (the PR body; settled by ADR-0066).
