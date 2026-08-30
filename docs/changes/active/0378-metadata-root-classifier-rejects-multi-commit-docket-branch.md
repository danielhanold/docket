---
id: 378
slug: 'metadata-root-classifier-rejects-multi-commit-docket-branch'
title: 'Shared metadata-root classifier misreads any multi-commit docket branch as foreign'
status: proposed
priority: high
type: fix
created: '2026-08-30'
updated: '2026-08-30'
depends_on: []
stacked_on:
related: [370, 371, 372, 377]
discovered_from: [377]
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

The repository-setup services share one predicate to decide whether a repo's metadata branch is
"our own" (an orphan seed we published) versus an unrelated foreign branch that must not be adopted.
That predicate — the classifier case reading `len(roots) == 1 && roots[0] == metaTip` in
`internal/app/repository_check.go`, mirrored as `len(roots) != 1 || roots[0] != commit` in
`repository_init.go` and reachable through the seed-verification path in `repository_migrate.go` —
is only true for a **single-commit** branch. It treats "the branch has exactly one root commit AND
that root *is the branch tip*" as the orphan proof, which conflates the seed root with the tip.

Any healthy docket branch grows past one commit. This live repository's `docket` branch is a
multi-thousand-commit chain with a single orphan seed root, so the predicate classifies it
`metadata-root-foreign` and the setup services refuse it. Confirmed directly against `main`: the
installed `docket repository check --json` already emits `metadata-root-foreign` on this repo.

This is a **pre-existing `main` defect**, independent of any one change. It was latent because no
maintained path refused on the classification until change [[0377]] (`repository prepare`, meant to
replace `docket.sh preflight` on real repos) became the first operation to hard-depend on it — that
run halted here. `RootCommits`' own contract states the correct orphan proof is "exactly one root,
**and that root carries the docket seed receipt / tree**" — not "that root equals the tip."

## What changes

Fix the shared metadata-root classifier so it proves orphan-ness by the **seed root's identity
(its tree / published seed receipt)**, not by root-equals-tip, and apply the fix consistently across
every service that reuses the predicate — `repository_check.go`, `repository_init.go`,
`repository_migrate.go`, and the `reposetup` `RootParentless` contract they lean on. A healthy
multi-commit docket branch with a valid seed root must classify as *ours*; a genuinely foreign
branch must still be refused. This is the fix that unblocks 377's `repository prepare` on real repos.

## Out of scope

- Change 377's own migration work (the native CLI verbs). This stub is the shared-predicate fix 377
  depends on; whether 377 absorbs it or is rebased onto it is a grooming decision, not settled here.
- Any redesign of docket's metadata-branch topology or seed model — the fix must preserve the
  existing orphan-seed model, only correcting how orphan-ness is *detected*.
- Weakening foreign-branch detection. The predicate is a security boundary (the foreign-branch
  adoption gate); a bare `len(roots) == 1` relaxation is explicitly not the intended fix.

## Open questions

- Exact orphan proof: seed **tree** equality, published **seed receipt** presence, or both — and
  which the `RootParentless` contract should assert.
- Whether the four call sites should collapse to a single shared classifier helper rather than
  each re-spelling the predicate (they drift as four copies today).
- Relationship to 377: fold the fix into 377's scope, or keep it as this standalone predecessor
  that 377 `depends_on`.

## Reconcile log
