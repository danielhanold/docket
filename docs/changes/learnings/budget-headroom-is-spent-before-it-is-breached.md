---
slug: budget-headroom-is-spent-before-it-is-breached
hook: "A wall-clock budget row sitting AT its ceiling is already spent — the queued changes against that same file, not the current green run, decide whether it breaches."
topics: [testing, budgets, planning]
changes: [270, 277, 247, 324, 309]
created: 2026-08-10
updated: 2026-08-16
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
- 2026-08-11 (#277, PR #194 — merged) — **The prediction closing, from the queued side.** 0277 was
  one of the two changes this finding named as queued against `tests/test_runner_dispatch.sh`, and it
  added ~50 cases to the file that had entered the change at 9s serial against a 10s ceiling. Because
  the margin had been recorded as a *number* at 0270's close-out rather than as "did not trip the
  budget check," the queued change could see that it was spending headroom it did not have and
  re-budgeted deliberately instead of discovering it as a red gate: the row was re-measured and
  raised to **20s**, with `EXPECTED_TOTAL` re-seeded to 1670. The first attempt raised it to 15s and
  was corrected at review to apply the table's own "next multiple of 5 plus a 5s margin" rule to the
  **worst standalone serial reading**, not the run-of-the-day reading. Change 0208, still queued
  against the same file, now inherits real margin. What this adds: re-budgeting is cheapest for the
  *next* change into the file, not only for the one that spent the margin — but only if the margin
  was written down where that change will read it. The reporting rule in *Apply* is the load-bearing
  half of this finding, not a stylistic preference.
- 2026-08-11 (#247, PR #200 — merged) — **Two margins spent on the `docket-status` surface, and the
  two changes that will breach them are already named.** This branch left
  `tests/test_docket_status.sh` with roughly **3s of margin against its 45s row**, and
  `skills/docket-status/SKILL.md` with **22 words** of headroom against its size budget. Both numbers
  are recorded here, not merely as "did not trip the budget check," because **#0118 and #0268 are
  both queued against `scripts/docket-status.sh`** and will each add to that surface. Whichever lands
  first is the one that trips it, and it will arrive with no context for why the margin was gone.
  The remedy is already known and does not need re-deriving: apply **change 0137's rounding rule**
  (next multiple of 5 plus a 5s margin, applied to the worst *standalone serial* reading, never the
  run-of-the-day contended one) and carry **change 0201's in-diff argument** for the word budget.
  Note the second surface: this finding was written about wall-clock rows, but a **word budget on a
  SKILL.md behaves identically** — a threshold that reports binary, consumed by changes that are not
  yours, on a file several queued changes all edit. The rule generalizes to any budgeted shared
  surface, not just the runtime table.
- 2026-08-15 (#324, PR #209 — merged) — **When the ceiling itself cannot be raised, parity forces a
  shard.** `tests/test_sync_agents_runners.sh` was already sitting AT the hard 60s ceiling on
  `origin/main` — headroom fully spent before this change existed. Registering the seventeenth
  shipped agent added the increment that tipped its measured wall over, and the usual remedy was
  unavailable: 60s is the maximum the runtime table's own rules permit for a row, and
  `EXPECTED_TOTAL` is pinned, so "re-budget" had no move left. The resolution was to **shard the
  file into three siblings** (`…_runners.sh` / `…_runners_gates.sh` / `…_runners_pins.sh`) with every
  assertion preserved and `EXPECTED_TOTAL` re-seeded 1845→1905. The lesson this adds to the family:
  the earlier entries treat re-budgeting as the escape hatch parity buys you time to take, but a row
  at the table's *hard* ceiling has no re-budget — the only remaining moves are sharding or deleting
  coverage, and sharding is a whole task's worth of work landed by whichever unrelated change happens
  to be holding the increment. That is the strongest argument yet for treating parity as the finding:
  at the hard ceiling, the cost of discovering it late is not a number edit, it is a refactor.
- 2026-08-16 (#309, PR #211 — merged) — **The hard-ceiling shard, again, one change after the last
  one.** `tests/test_go_race.sh` carried the same 60s hard ceiling that forced 0324's three-way
  split the day before, and `internal/gitcli` was already sitting near it before 0309 existed. The
  transaction package's real-git fixtures (each test registering and pruning actual detached
  worktrees) supplied the increment, and the remedy was 0324's verbatim: shard, because the row had
  no re-budget left. A sibling `tests/test_go_race_transaction.sh` took `./internal/repository/
  transaction/`, `test_go_race.sh` excluded that one package from a `go list`-derived set, and
  `EXPECTED_TOTAL` was re-seeded 1965→2035. What this entry adds is **the partition guard**: the
  split is held by an assertion that the two files' package sets union to exactly `go list ./...`,
  so a package added later cannot silently fall through the gap between the shards. That is the
  piece the earlier sharding entries did not have, and it is the difference between a shard and a
  coverage hole — an exclusion expressed as `grep -v` is a subtraction with no one checking that
  anything caught it. Note also the recurrence rate: two independent hard-ceiling shards in two
  days, on two different suites, each landed by whichever unrelated change happened to hold the
  increment. When that starts repeating, the finding is no longer "watch the margin" but "the
  ceiling policy is producing refactors as its failure mode."
