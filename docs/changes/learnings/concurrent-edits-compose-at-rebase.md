---
slug: concurrent-edits-compose-at-rebase
hook: "When two open changes touch one function, keep each additive and funnel through a shared chokepoint; at rebase reconcile by INTENT — compose, don't choose."
topics: [git, rebase, concurrency]
changes: [79, 89, 113, 223, 308]
created: 2026-07-16
updated: 2026-08-15
promotion_state: retained
promoted_to:
---

## Apply
When two open changes touch one function, keep each additive and funnel through a single
shared chokepoint; at rebase reconcile by INTENT (compose, don't choose), then run BOTH changes'
twin test suites green before trusting the merge — `bash -n` + no-conflict-markers is not enough.

When the conflicted hunk is a COUNT (an asserted line count, an inventory size, a prose "N ops"),
neither side is likely to be correct: one branch added items, the other removed them, and the
merged truth is a third number. Derive it by counting the merged artifact itself, never by taking
a side or by arithmetic on the two claims. Then hunt the count's SEMANTIC TWINS — the same fact
restated in files that auto-merged cleanly, where git's line-based merge saw no conflict at all.
A clean auto-merge is not evidence of a correct merge for any fact stated in more than one place.

The two branches need not touch a single common file. When main introduces a REGISTRY with a
completeness invariant over some set — every test file has a budget row, every check id appears on
three surfaces, every script has a contract — any open branch that ADDS A MEMBER to that set is
already red, with zero textual overlap and nothing for a conflict resolver to see. Ask at rebase
what sets your branch adds a member to, and whether the base started policing one of them.

## War story
- 2026-07-16 (#79, PR #86) — Two in-flight changes (0077 codex-TOML emission, 0079 runner shim)
  edited the same `sync-agents.sh` emitter; the second to merge hit a REAL semantic rebase conflict,
  not a textual one — both features had to COMPOSE (route the shim's native paths through the
  harness-aware `emit_for_harness` chokepoint) rather than pick a side. It stayed resolvable only
  because 0079's edits were additive (new functions + call-site swaps, per its own results note).
- 2026-07-18 (#89, PR #99) — Change 0095 retired the `auto_approve` subsystem on main while 0089
  added two reclaim knobs; the rebase produced six hunks that were almost all COUNTERS moving in
  opposite directions (a `KEY=value` line-count assertion 22→21 vs 22→24, resolving to 23; a
  facade op list 13 vs 15, resolving to 14). Taking either side verbatim would have routed a facade
  op to a script main had deleted. The counts were settled by counting the merged artifacts, not by
  arithmetic. The trap was in `scripts/docket-config.md`, which auto-merged CLEANLY and so silently
  took the PR's now-wrong numbers — the same fact as the conflicted test assertion, invisible to a
  line-based merge.
- 2026-08-03 (#113, PR #154 — merged) — **Both changes predicted this conflict in writing, and the
  prediction is what made it cheap.** 0201 (skill compression) ratcheted the size budgets in
  `tests/test_skill_size_budgets.sh` DOWN while 0113 added prose to
  `skills/docket-implement-next/SKILL.md` and raised that file's row UP; each change's reconcile log
  and PR body named the other and said, in advance, *re-measure the merged file rather than taking
  either side*. 0201 merged first; 0113 rebased across it. The conflicted hunk was a **count**,
  exactly as this finding describes: 0113's `4050` was measured against the pre-slim file, 0201's
  `3700` was measured before 0113's prose existed, and the merged truth was a third number — 3728
  actual words → `3800` after the file's own rounding rule. The line budget needed no raise at all
  (139 still fits 0201's ratcheted 145), so one side's conflicted number was simply *deleted*
  rather than reconciled: composing does not mean both edits survive, it means the merged artifact
  decides.
  **The semantic twin fired again, in a file with no conflict markers anywhere.** 0113's
  `docs/results/…-results.md` restated the same fact in prose — "raised it 3950 → 4050" and "only
  37 words of budget margin left" — in a Follow-ups section git auto-merged clean, because the
  results file existed only on 0113's branch and nothing on main touched it. A file the other
  branch never opened cannot conflict, so *nothing* in the rebase could have surfaced it; it landed
  on main stating two numbers that were both wrong the moment the rebase resolved. Caught at
  finalize by re-reading the merged results file against the resolution and appended as a
  post-merge note. Add to the twin hunt: **the count's restatements are not only in files that
  auto-merged — they are in files the conflict could not reach at all**, and a build's own results
  or plan prose is the most likely place, since it was written before the rebase and is never
  re-read after it.
- 2026-08-07 (#223, PR #166 — merged) — **The disjoint-file case: a textually clean rebase, red on
  arrival.** 0223 added `tests/test_gate_execution_posture.sh`; change 0227 had independently landed
  `tests/runtime-budgets.tsv` on main, whose guard asserts a two-directional correspondence between
  the table's key column and `find tests -maxdepth 1 -name 'test_*.sh'`. Neither branch opened a
  file the other touched, so all 12 commits rebased clean and the resolver had nothing to resolve —
  the failure surfaced only at the gate's post-rebase suite run, as `> tests/test_gate_execution_posture.sh`
  in a one-line diff. The fix was two lines (the row, plus the `EXPECTED_TOTAL` sum the table pins
  precisely so a raise cannot be quiet), so the cost was not the repair but the full 87-file gate
  cycle spent discovering it. **The generalization worth carrying: a completeness registry converts
  every concurrent branch that adds a member into a semantic conflict, and the conflict is invisible
  to git by construction** — the registry's whole point is to live in one file that no member's
  author edits. The counterpart to the previous entry's lesson: there the twin was a fact restated
  in a file the conflict could not reach; here it is a fact about a file that does not exist yet on
  the other side.
- 2026-08-15 (#308, PR #210 — merged) — **The registry's own pinned total, raised by both sides
  from the same ancestor.** 0308 added `tests/test_go_race.sh` and raised `EXPECTED_TOTAL` in
  `tests/test_runtime_budgets.sh` 1825 → 1885 (+60 for the new row); meanwhile 0324 landed on main
  and raised the same constant 1825 → 1905 (a shard re-cut of `test_sync_agents_runners.sh` plus two
  new plan-writer guard files). `tests/runtime-budgets.tsv` auto-merged cleanly — the rows are
  additive and disjoint — so only the pinned total conflicted, and **neither side's number was the
  answer**: 1885 and 1905 both describe a table that no longer exists. The merged truth, 1965, came
  from summing the reconciled table itself
  (`awk -F'\t' '!/^#/ && NF>=2 {s+=$2} END{print s}' tests/runtime-budgets.tsv`), never from
  arithmetic on the two claims. This is the previous entry's registry lesson one turn further in:
  there a concurrent branch adding a member went red against a registry it never opened; here both
  branches added members and the registry's *invariant constant* is the single line the merge can
  see. **The twin fired once more, and once more in prose git could not reach**: a `#`-comment at
  `tests/runtime-budgets.tsv:152` still narrates `EXPECTED_TOTAL moves 1825 -> 1885`, and the
  change's results file restates the same superseded pair. Neither can fail the assert (both sit
  outside the awk sum), which is exactly why nothing catches them — the count's restatements
  survive a correct resolution as stale narrative.
