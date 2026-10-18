<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0565 — Refuse a stack-parent kill that would strand built children](../changes/archive/2026-10-18-0565-make-killed-parent-stacking-behavior-match-its-documented-po.md)**
<!-- docket:backlink:end -->

# Results — Refuse a stack-parent kill that would strand built children

**Human action:** None required to merge. One Optional walkthrough is below. One follow-up needs your triage: a narrow gap the review found, outside this change's scope.

## Outcome

`change.kill` now refuses to kill a change that still has `implemented` or `stacked-merged` stacked descendants reached through live links. A descendant below a `done` or `killed` link does not count. The refusal names each built descendant and gives a remedy that never names a command that would refuse. For an `in-progress` parent the remedy is to finish it, or pause it with `change.block` and later resume it with `change.unblock`. For a legacy `proposed` parent that carries a branch, the remedy is to leave it or defer it. The terminal output prints that message under the result line.

A kill never changes any descendant's status. The unused `KillStackParent` blocking code has been deleted. On the board, a proposed child whose stack ancestor was killed reads `stack parent #NNNN killed — re-parent, un-stack, or kill`, naming the killed ancestor. The stacked-changes reference, the convention, and the glossary now describe only this behavior.

## Human actions and testing

- **Optional: see the refusal in a scratch repo.** Prerequisites: an installed `docket` built from this branch, and a throwaway repository set up with `docket repository init`. Steps:
  1. Create change A and claim it.
  2. Create change B with `stacked_on: A`, and take it through to `implemented` (or set B to `implemented` by hand in that scratch repo).
  3. Run `docket change kill --id <A> --revision <rev>`.

  Expected: the output starts `change kill: invalid-state`, followed by a line beginning `change 000A has built stacked changes (#000B implemented)`. Both records are unchanged. Cleanup: delete the scratch repository.

## Verification performed

- Build: four plan tasks, each with RED-then-GREEN focused tests and mutation checks (domain walk arms, final-link pruning, the message overwrite, the remedy branch, and the board branch). The full suite (`go run ./cmd/docket development test`) passed after the build and again after the fix pass.
- Review: a standard-tier review returned 1 important and 3 minor findings. All three minors were fixed in-branch: the refusal detail now carries `reason`, `from`, and `action`; the glossary gives the proposed-parent remedy; and an end-to-end `ChangeKill` integration test, mutation-checked, proves no commit is made and both records stay byte-identical. The important finding is beyond this branch and is reported below.
- The suite printed BUDGET WATCH and PARALLEL-SENSITIVE lines for several integration scripts under -j11, in scripts this change does not touch. There was no SERIAL CONFIRMED OVER BUDGET line.
- Two prose size budgets were re-baselined to measured numbers: convention SKILL.md went 360/7317 -> 360/7331, and stacked-changes.md went 214/2149 -> 219/2173.
- No ADR was recorded. The decision was settled at grooming and is documented in the spec and the stacked-changes reference.

## Known issues and follow-ups

- **An in-flight stacked child can still become stranded (suspected, not reproduced).** A kill is allowed while a descendant is `in-progress`. If that child's build resolved its base before the parent was killed, it can still open its PR against the dead parent branch and move to `implemented`, because `change.mark-implemented` never re-checks the effective base. The child then has no typed exit, which is the same dead end this change closes for the common case. Impact: rare. It needs a kill to race a running child build, and it has never happened here. Workaround: don't kill a parent while a child build is running. Suggested next action: gate `change.mark-implemented` on a resolvable effective base (refusing `parent-killed`), or document the race as an accepted gap. Backlog verdict: No existing change fits (checked 23). Capture it as a new change, with `related: [565]`.
