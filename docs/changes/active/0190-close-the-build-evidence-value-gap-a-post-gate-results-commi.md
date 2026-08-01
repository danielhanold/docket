---
id: 190
slug: close-the-build-evidence-value-gap-a-post-gate-results-commi
title: Close the build-evidence value gap: a post-gate results commit always defeats finalize's suite skip
status: proposed
priority: medium
type: feat
created: 2026-08-01
updated: 2026-08-01
depends_on: []
related: []
discovered_from: [170]
adrs: []
spec:
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

Decide whether the skip predicate should admit a narrow **docs-only extension**: the evidence
`head_sha` is an *ancestor* of the branch HEAD, and every intervening commit touches only paths
that cannot affect the suite (`<results_dir>/`, `docs/superpowers/plans/`, and nothing else).

The design work is the hard part, not the implementation:

- Ancestry plus a path allowlist is strictly weaker than SHA equality. Enumerate what an attacker
  or a mistake could smuggle through a path filter, and whether a docs path can ever affect a suite
  that greps documentation — in this repo it demonstrably can, since several guards assert over
  `README.md` and `skills/**/*.md`, so the allowlist must be justified per-repo rather than assumed.
- Consider the cheaper alternative first: have Step 7 **re-mint** the evidence after the last
  post-gate commit instead of relaxing the consumer. That keeps exact SHA equality — the property
  that makes the predicate auditable — and moves the work to the producer, which already knows when
  it has finished committing. This may make the whole extension unnecessary.
- Whichever path is chosen, the "fails toward running" posture and the loud one-line skip log must
  survive unchanged.

## Out of scope

- Weakening any other condition of the skip predicate.
- Changing where the evidence block lives (the PR body; settled by ADR-0066).
