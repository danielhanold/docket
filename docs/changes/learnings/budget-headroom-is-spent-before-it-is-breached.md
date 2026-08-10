---
slug: budget-headroom-is-spent-before-it-is-breached
hook: "A wall-clock budget row sitting AT its ceiling is already spent — the queued changes against that same file, not the current green run, decide whether it breaches."
topics: [testing, budgets, planning]
changes: [270]
created: 2026-08-10
updated: 2026-08-10
promotion_state: retained
promoted_to:
---

## Apply
A per-file runtime budget is a threshold, so it reports as binary: under, or over. That framing
hides the state that actually matters — **how much headroom is left** — and it hides it precisely
when the headroom reaches zero, because a file measuring exactly at its ceiling still reports green.
The run that finally breaches it will be some *later* change, which then pays a diagnostic cost it
did not cause and gets handed a budget decision it has no context for.

Two things make this worse than a normal threshold:

- **The measurement is contended, not absolute.** A suite that runs its files in parallel measures
  each file under whatever contention the rest of the run happens to produce, so the number moves
  between runs on the same code. A file at parity with its ceiling is therefore not "just barely
  passing" — it is passing or failing by luck of scheduling, and the runner's slack factor is the
  only thing absorbing it.
- **Headroom is consumed by changes that are not yours.** Test files are shared surfaces. Several
  queued changes can each add a few cases to one file, each individually cheap, with no textual
  conflict and nothing for a rebase to see. This is the runtime-cost sibling of
  [[concurrent-edits-compose-at-rebase]]'s registry hazard: the coupling is through a shared
  *number*, not a shared line.

So the rule is to treat **parity as the finding**, not the breach. At close-out, when a change moved
a budgeted file's measurement, read the row's remaining margin and ask which open changes are queued
against that same file. If the margin is gone and more work is queued, re-budget or shard **before**
that work lands — the decision is cheapest for the change that can still see why the number moved,
and most expensive for the unrelated change that trips it. Report the margin in the results file as
a number, never as "did not trip the budget check": the second phrasing is true and tells the next
reader nothing.

## War story
- 2026-08-10 (#270, PR #193) — a change that added exactly **one** test section (a real linked
  worktree plus a `build-*` dispatch) pushed `tests/test_runner_dispatch.sh` to **10s under parallel
  contention against its own 10s ceiling** in `tests/runtime-budgets.tsv` — 6.0s serial, up from
  5.79s. It did not trip the budget check; the runner's slack factor absorbed it, and the gate was
  green. But the file became the closest in the suite to its own ceiling, and **two further changes
  (0208 and 0277) were already queued adding cases to that same file** — neither of which would have
  any reason to know the margin had been spent, or that a single added section had spent it. The
  close-out flagged it as a human decision ("raise that row before the next change adds to this
  file") rather than leaving it for whichever change happened to breach first. Note the shape of the
  near-miss: the *serial* number moved only 0.2s, so serial measurement would have shown comfortable
  margin — the contended number is the one the ceiling is actually compared against.
