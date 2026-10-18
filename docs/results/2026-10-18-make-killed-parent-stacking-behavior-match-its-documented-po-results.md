<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0565 — Refuse a stack-parent kill that would strand built children](../changes/active/0565-make-killed-parent-stacking-behavior-match-its-documented-po.md)**
<!-- docket:backlink:end -->

# Results — Refuse a stack-parent kill that would strand built children

**Human action:** Assessment pending; the review fix pass is still running.

## Outcome

`change.kill` now refuses to kill a change that still has `implemented` or `stacked-merged` stacked descendants reached through live links. A kill never changes any descendant's status. The unused `KillStackParent` blocking code is gone. On the board, a proposed child whose stack parent was killed reads `stack parent #NNNN killed — re-parent, un-stack, or kill`. The stacked-changes reference, the convention, and the glossary describe only this behavior.

## Verification performed

- Build: four plan tasks (standard, standard, economy, standard), each with RED-then-GREEN focused tests and mutation checks. The full suite (`go run ./cmd/docket development test`) passed at a9aa2c6c0.
- The suite printed BUDGET WATCH and PARALLEL-SENSITIVE lines for several integration scripts (rebase, recordops, repovisibility, finalize e2e, and others) under -j11. There was no SERIAL CONFIRMED OVER BUDGET line. These scripts are untouched by this change.
- Two size budgets were re-baselined to measured numbers: convention SKILL.md 360/7317 -> 360/7331 and stacked-changes.md 214/2149 -> 219/2173.

### Review findings (standard tier, pending fixes)

1. important: an `in-progress` stacked child whose build already resolved its base can still reach `implemented` after its parent is killed, because `MarkImplemented` never checks the effective base. Beyond this branch; to be reported.
2. minor: the new refusal's detail drops the `reason`/`from`/`action` keys other kill refusals carry.
3. minor: the glossary remedy names only finish or block, which is wrong for a legacy `proposed` parent.
4. minor: the app tests check the refusal at `Plan` level only, with no end-to-end `ChangeKill` byte-identity check.
