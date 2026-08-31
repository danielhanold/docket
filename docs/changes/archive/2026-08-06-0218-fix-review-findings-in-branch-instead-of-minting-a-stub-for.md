---
id: 218
slug: fix-review-findings-in-branch-instead-of-minting-a-stub-for
title: Fix review findings in-branch instead of minting a stub for every one
status: done
priority: high
type: feat
created: 2026-08-05
updated: 2026-08-06
depends_on: []
related: [197, 200, 220]
discovered_from: [202]
adrs: [66, 70]
spec: docs/superpowers/specs/2026-08-06-fix-review-findings-in-branch-design.md
plan: docs/superpowers/plans/2026-08-06-fix-review-findings-in-branch.md
results: docs/results/2026-08-06-fix-review-findings-in-branch-instead-of-minting-a-stub-for-results.md
trivial: false
auto_groomable:
branch: feat/fix-review-findings-in-branch-instead-of-minting-a-stub-for
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/162
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-06-fix-review-findings-in-branch-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-06-fix-review-findings-in-branch-design.md) |
| Plan | [2026-08-06-fix-review-findings-in-branch.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-06-fix-review-findings-in-branch.md) |
| Results | [2026-08-06-fix-review-findings-in-branch-instead-of-minting-a-stub-for-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-08-06-fix-review-findings-in-branch-instead-of-minting-a-stub-for-results.md) |
| ADRs | [ADR-0066](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0066-docket-owns-the-review-role-suite-runs-in-the-build-gate.md), [ADR-0070](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0070-fix-loop-profile-envelope-blocker-floor-and-max-ceiling.md) |
<!-- docket:artifacts:end -->

## Why

**54 of 56 `proposed` changes carry `discovered_from`.** Only two entered the backlog from outside
the loop. docket is generating its own work faster than it drains it, and the mechanism is
structural, not incidental: **no station on the assembly line is permitted to fix a review
finding.**

- `docket-build` — findings do not exist yet.
- `docket-review` — its contract is explicitly *"never fixes, dispatches, or runs the test suite."*
- `docket-implement-next` — takes the review output, writes it into the results file under
  "for merge-time judgment," and stops at the merge gate. There is no loop back.
- `docket-finalize-change` — the only repair authority in the gate is `docket-integration-repair`,
  scoped to making a **red suite green after rebase** and gated behind sign-off. Nothing authorizes
  a correctness or quality fix.

So a non-blocking finding has exactly one exit: a stub. The pipeline routes 100% of them to the
backlog, where they compete for selection against real feature work and mostly lose.

The cost is visible in the ledger. Change 0113 merged with five unfixed findings; change 0202
existed solely to clear them, and produced five more (two important, three minor) which became
0215, 0216, and 0217. One full change consumed — plan, build, review, gate, close-out — for a net
of zero findings retired. Three of those five were one-line edits: a dead `|| continue`, a stale
comment, a wrong grep pattern in a plan file.

The failure is worst for `minor` findings, where the fix is reliably cheaper than the stub that
records it. A stub costs a title, a body, an id, a groom, a spec, a plan, a branch, a PR, and a
close-out. The dead line of code costs one deletion.

## What changes

- **A bounded fix loop inside `docket-implement-next` Step 6** — an extended phase, not a new role
  skill — repairing findings on the open branch after review returns and before the PR opens. The
  human's gate does not move: every auto-authored fix arrives inside the diff they already read.
- **Two orthogonal axes:** the fix's *character* picks the profile via docket-build's task-routing
  rubric (extracted to a shared `docket-build/references/task-routing.md`, stub + pointer left
  behind); the finding's *severity* picks only the failure posture. No fix task ever dispatches
  `max` — the rubric's premium/max boundary doubles as the in-branch size ceiling; blockers halt on
  a max-character fix (today's ladder endpoint), non-blockers get a PR-body record.
- **Per-finding tasks and commits** for blockers and importants (replacing the single synthetic
  all-blockers task); minors batch per shared routed profile with one enumerating commit. All fixes
  run the `docket-build-task` contract with build's one-bounded-escalation rule, truncated at
  premium.
- **Revert-and-record suite gate:** one full-suite re-run after fixes; red → revert the non-blocker
  fix commits and re-run once — green proceeds with those findings recorded unfixed, still-red
  halts. Bounded at two suite runs; no re-review round.
- **`review.min_fix_severity` knob** (`minor` default | `important` | `blocker`), resolved as
  `REVIEW_MIN_FIX_SEVERITY`; `blocker` is the pre-0218 compat escape hatch. Global-able; not a
  coordination key.
- **Auto-capture narrows:** a finding about this branch's own diff is never mintable; the
  materiality bar gains "work fixable by a small in-branch edit fails the bar." PR-body findings
  become a disposition table (fixed / deferred / reverted / recorded); `## Verify (human)` shrinks
  to genuinely manual checks.

## Out of scope

- Changing `docket-review`'s read-only contract (ADR-0066). The reviewer stays a reviewer.
- Retroactively clearing the existing self-generated backlog. That is its own triage pass, and this
  change is about closing the tap.
- The merge gate's `docket-integration-repair` path, which is correct as-is for its own purpose.

## Reconcile log

### 2026-08-06

Reconciled against `origin/main` at `12cf78e6` and the current docket branch. **Scope unchanged —
every premise the spec was groomed on still holds:**

- `skills/docket-implement-next/SKILL.md` Step 6 still carries the exact rule this change replaces
  ("An `important` or `minor` finding is recorded in the PR body … never auto-fixed") and the single
  synthetic all-blockers task on the `standard → premium → halt` ladder.
- `skills/docket-build/SKILL.md` still carries the `## Routing` section verbatim at lines 48–90,
  with the `max`/`premium` organizing principle the fix loop's ceiling rule leans on. The extraction
  target `skills/docket-build/references/task-routing.md` does not exist yet, and `docket-build/`
  currently holds only `SKILL.md` — the `references/` directory is new.
- No `review:` block and no `REVIEW_MIN_FIX_SEVERITY` exist anywhere in `scripts/` or `skills/`.
  `finalize.gate` (`scripts/docket-config.sh:424`) is the working precedent for a global-able,
  non-fenced block-scoped scalar, including its explicit "deliberately NOT coordination-fenced"
  comment at line 434 — that comment block is the place the new knob's classification note belongs.
- `skills/docket-convention/references/auto-capture.md` §Materiality bar (lines 16–20) is still the
  three-way routing paragraph the new "fixable by a small in-branch edit" clause extends.
- ADR-0066 is `Accepted` and unchanged; the reviewer's read-only contract stays out of scope.
- The motivating pattern is not stale, it grew: 0197, 0200, **and 0220** are all still sitting in
  `active/` as unbuilt "clear the unfixed review findings from …" changes, and 0220 is currently
  second in the build-ready queue behind this change.

**One new coupling, non-blocking.** Change 0190 (`close-the-build-evidence-value-gap`) is
`in-progress` on `feat/close-the-build-evidence-value-gap-a-post-gate-results-commi` and reworks how
the build-evidence record's `head_sha` staleness is treated after post-gate commits. This change's
suite gate refreshes that same record after fix commits land. 0190 is unmerged, so this build
proceeds against the evidence contract as it exists on `origin/main` today; whichever change merges
second absorbs the reconciliation. Recorded rather than made a `depends_on` — neither change's
correctness needs the other, and adding the edge would deadlock two in-flight branches.

No obsolescence, no invalidation, no scope adjustment. Proceeding to plan.
