---
id: 218
slug: fix-review-findings-in-branch-instead-of-minting-a-stub-for
title: Fix review findings in-branch instead of minting a stub for every one
status: proposed
priority: medium
type: feat
created: 2026-08-05
updated: 2026-08-05
depends_on: []
related: []
discovered_from: [202]
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

- **A fix loop in `docket-implement-next`, after review returns and before the PR opens.** Findings
  are repaired where the build context is still hot and the branch is still open. The human's gate
  does not move: the PR is still the review, and every auto-authored fix arrives inside the diff
  they already read.
- **Severity routing**, replacing today's uniform "record and defer":
  - `blocker` → fix (already the case).
  - `important` → fix in-branch by default; stub only on an explicit, recorded deferral.
  - `minor` → fix in-branch; never stub.
- **Raise the auto-capture materiality bar** so cosmetic findings cannot become changes at all. The
  bar is "would a human file this as a `docket-new-change`?" — a dead line of code fails it, and
  0217 should not have been mintable.
- Whatever bound the fix loop needs (attempt cap, size ceiling, suite re-run) so it cannot become an
  unbounded second build phase.

## Out of scope

- Changing `docket-review`'s read-only contract. The reviewer should stay a reviewer; the fixer is
  a separate role, dispatched with the review's findings as input.
- Retroactively clearing the existing self-generated backlog. That is its own triage pass, and this
  change is about closing the tap.
- The merge gate's `docket-integration-repair` path, which is correct as-is for its own purpose.

## Open questions

- Does the fixer run as a new role skill (`skills.fix`) or as a bounded phase inside
  `docket-implement-next`? A role skill is more composable; a phase is less machinery.
- Does a fix loop that reddens the suite abort the run, or hand back to build?
- Should the severity policy be configurable (`review.fix_severity: minor|important|blocker`), or
  fixed? A knob invites a repo to turn the discipline off, which is how this hole opened.
- Does the review re-run after fixes land, and if so, how is a second round of findings bounded?
- Where does the `## Verify (human)` checklist fit once findings are fixed rather than recorded?
