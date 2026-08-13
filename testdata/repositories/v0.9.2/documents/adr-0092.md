---
id: 92
slug: a-stacked-changes-base-is-its-parents-merge-destination
title: "A stacked change's effective base is its parent's merge destination"
status: Accepted
date: 2026-08-12
supersedes: []
reverses: []
relates_to: []
change: 298
---

## Context

`stack_effective_base` (`scripts/lib/docket-stack.sh`) answers "which branch should this stacked
change be cut from, and is its base built yet?" That one answer drives build-readiness, the branch
cut, the PR base, and the finalize gate — so a wrong answer silently produces a feature branch that
is missing its own parent's work, the exact failure stacking exists to prevent.

The change's accepted spec (`docs/superpowers/specs/2026-08-11-stacked-changes-design.md`, §3
rule 2) lumps a `done` parent together with a `stacked-merged` one: both "recursively resolve the
parent's effective base". Its justification is the clause "a merged parent's commits survive in its
base". That reasoning holds for one of the two statuses and fails for the other, because docket has
**two different merge destinations**:

- `stacked-merged` — the change merged into its **parent's** branch. Its commits really are inside
  that branch, so resolving upward reaches a branch that contains them.
- `done` — the change's PR merged into the **integration branch**. Docket's governing lifecycle
  invariant is that `done` means the code is reachable from the integration branch, and spec §2 is
  explicit that a change merged only into its parent becomes `stacked-merged`, never `done`.

So for a `done` parent the commits live on the integration branch. A still-open grandparent branch
was cut *before* that merge and does not contain them; recursing upward returns a base missing the
parent's work. The case is reachable whenever an intermediate change is retargeted and merged
straight to the integration branch while its own parent is still open.

## Decision

In `stack_effective_base`, `done` and `stacked-merged` parents take **different** arms:

- a **`done`** parent resolves to the **integration branch, terminally** — no upward recursion;
- a **`stacked-merged`** parent recursively resolves that parent's own effective base.

The general rule both arms express: **resolve to wherever the parent's commits actually landed —
its merge destination — and docket has two of them.**

This deviates from the change's own accepted spec, which is a point-in-time record and was
deliberately left unedited; this ADR is the reconciling document.

## Consequences

- A child stacked on a `done` parent now gets a base that actually contains its parent's code.
- Second-order fix: a `done` parent whose *own* parent is `killed` previously exited 3 and dropped
  the child out of the ready queue via the `stack-parent-killed` health check. It now resolves to
  the integration branch — correct, because the `done` parent's merge already made the killed
  ancestor irrelevant to the child.
- The resolver's exit 3 (killed ancestor) is narrowed: reachable only via the immediate parent being
  killed, or via the `stacked-merged`-with-no-remote-branch fallback recursing into a killed
  ancestor. A kill above a `done` link is unreachable. Diagnostics in `scripts/board-checks.sh` and
  `scripts/stack-base.sh` were reworded to match.
- Cost: the spec and the change's merged plan record the superseded rule. Both are frozen
  point-in-time records kept as written, so a future reader meets the old rule first and needs this
  ADR to reconcile it.
- Verified before the behavior change (the reasoning was checked against the spec) and after, by
  mutation-testing both directions in `tests/test_docket_stack.sh`: restoring the old recursive arm
  reddens the three new legs, and deleting the `done)` arm entirely reddens those three plus the
  pre-existing "rule 2: a done parent resolves to the integration branch" assert — confirming that
  assert is not vacuous under the new code. Full suite green (112 files, 9259 asserts).
