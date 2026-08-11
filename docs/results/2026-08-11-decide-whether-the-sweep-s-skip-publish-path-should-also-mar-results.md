<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0118 — Decide whether the sweep's skip-publish path should also mark an unpublished terminal record](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0118-decide-whether-the-sweep-s-skip-publish-path-should-also-mar.md)**
<!-- docket:backlink:end -->

# Mark the sweep's skip-publish path — results

Change: #0118 · Branch: feat/decide-whether-the-sweep-s-skip-publish-path-should-also-mar · PR: (see change file `pr:`) · Plan: docs/superpowers/plans/2026-08-11-sweep-skip-publish-mark.md · ADRs: 90

## Verify (human)

- [ ] **The two legs now need different remediations — confirm the SKILL.md wording reads right to you.** A `terminal-publish` gap is fixed by publishing. A `skipped-publish` gap must be **re-rendered first**, because publishing strips the marker and ships the stale `## Artifacts` block the skip-publish guard exists to stop. No automated test can tell you whether the sentence an agent will actually read at triage time is clear enough to be followed correctly.
- [ ] **`tests/test_docket_status.sh` is now AT the runtime table's hard 60s ceiling** (`tests/runtime-budgets.tsv`). There is no next raise. #0296 was minted to shard it, and #0268 and #0154 are both queued against that same file — decide whether the shard should be pulled forward ahead of them.

## Findings

**The decision was already settled; only its consequences needed building.** The change's title reads as an open question, but the 2026-08-07 spec settled it and none of the nine changes merged since reopened it. The reconcile pass said so explicitly rather than re-litigating.

**The deciding property is permanence, not frequency — and that inverts the stub's own premise.** The stub assumed "the next sweep retries the re-render, so the window is one pass." It does not: once archived, a change leaves `active/` and the sweep scans `active/` only, so **no later pass ever resumes it**. The gap is permanent until a human acts. Recorded as **ADR-0090**, which extends ADR-0051 rather than superseding it: `## Publish deferred` now covers any **handled** post-archive failure that abandons an expected publish. "Handled" is load-bearing — a hard crash can write nothing by definition, so the rule must not claim coverage it cannot enforce, and that residual stays accepted.

**A shared helper's precondition is not automatically shared (review finding 1 — the most substantive).** The plan put the clean-path precondition inside the new `sweep_mark_publish_deferred` helper, so it silently gated the change-0083 leg too. On that leg the dirty path is frequently *this run's own wake* — including the exact window `scripts/terminal-publish.sh` documents as covered by "the driver's defer path re-marks" after its marker-removal `die`. Refusing to mark there re-opened a documented recovery window with nothing left to restore it. The fix scopes the clean-path check to the `skipped-publish` call site and leaves only the wedge probe shared. The general shape: when a precondition encodes an invariant that holds on one caller's path and not another's, hoisting it into the shared helper silently changes the other caller's contract, and no test written for either caller alone will notice.

**Change 0247 landed on this surface after the spec was written, and its rule had to be extended, not just respected.** 0247 taught the sweep's artifacts-refresh block to refuse committing into a mid-rebase/merge shared worktree. The mark blocks are further exposed commits into that same tree and 0247 did not reach them. The wedge probe was folded into the mark's precondition — not as scope creep but as a correctness requirement of the spec's own recovery step: inside a rebase, `HEAD` is the rebase's *detached* HEAD, so `checkout HEAD -- "$archived"` would corrupt the very file it exists to restore.

**Plan-supplied test and probe code was defective in four separate places, and every one was caught by running it.** This is the sixth consecutive change in this drain to hit the class (tracked systemically as #0292):
- a `printf` line-wrap that split a clause the plan's own Interfaces block promised would render intact, reddening a correct assert;
- `grep -c "…\$GIT…"` probe counts that return **0** under this repo's PATH `grep` (ugrep 7.5.0 treats `$` as an anchor mid-pattern). Because the plan `&&`-chained the count into the mutation, the mutation **never ran** and the probe was silently vacuous — a green that meant nothing;
- a `mkgitfail` git wrapper whose subcommand scan broke at the first non-flag word, which is `-C`'s *value*, so it never blocked anything. Proven by the plan's own standalone probe step before any assert was written; had it been trusted, all eleven fault-injection asserts would have been vacuous;
- mutation `before/after` counts stated as `1 then 0` where the repo actually contains two byte-identical lines, so an unanchored pattern would mutate the wrong site.

The pattern worth keeping: **three of the four were caught by a probe-the-probe step, not by the assert going red.** A vacuous guard and a working guard look identical from the outside.

**Deep review returned nine findings, zero blockers; all nine were fixed in-branch.** Two are worth naming because they are latent rather than active: a comment warning about `test_closeout.sh`'s call-site scanner was itself one reflow away from tripping that scanner, and the single-writer assert counted every *mention* of a script name rather than its *invocations*, so a future doc-pointer would have reddened an assert whose message sends the reader hunting a call site that does not exist.

**Budget headroom was spent before this change arrived, on three rows, not one.** The plan anticipated two; `skills/docket-convention/references/terminal-close-out.md` was also at parity (1469/1500 words, 174/180 lines) and was found only when a worker ran the guard. Measured base for `tests/test_docket_status.sh` was **41.75s** worst standalone serial against a 45s row — already inside a few seconds of parity before this change touched it. This change's six fixtures cost **≈ +3.2s**; the row went 45 → 50 → 60 across the run. Margins left as numbers, deliberately: **`tests/test_docket_status.sh` 60s row, ≈15s from the quiet worst reading and 8.6s from the worst reading ever seen on this machine; `skills/docket-status/SKILL.md` 2539/2600 words (61); `terminal-close-out.md` 192/200 lines and 1683/1750 words.**

## Follow-ups

- **#0295** (`fix`) — make `render-change-links.sh` genuinely offline-safe. It is documented as offline-safe but resolves config through `docket-config.sh --export`, which runs `git fetch` and dies on failure — so a network blip fires the very branch this change now marks, and pushing that marker needs the same network that just failed. The spec named this as out of scope and bounded the residual (the local commit self-heals); the stub removes the cause.
- **#0296** (`chore`) — shard `tests/test_docket_status.sh`. Its runtime row is now at the table's hard 60s ceiling, where the table's own header says the remedy is a shard, not a number. Coupled to #0268 and #0154, which target the same file.
- **Not minted, reported only:** the gate's first full run also flagged `tests/test_docket_config.sh` (147s/55s) and `tests/test_sync_agents.sh` (127s/50s) over budget under heavy parallel contention; neither reproduced on the second run and neither is touched by this branch. `tests/test_sync_agents_runners.sh` (~193s/60s) is the known pre-existing #0280. These are the same class as #0296 but in files this change never opened.

## Plan deviations

- The clean-path precondition moved from the shared helper to the `skipped-publish` call site (review finding 1) — a correction to the plan, not to the spec, which had it at the call site all along.
- The wedge probe was added to the mark path, which the spec did not name (it predates change 0247). Argued in the reconcile log as a correctness requirement of the spec's own recovery step.
- A third budget file (`terminal-close-out.md`) needed raising; the plan's budget task named only two.
- `skills/docket-status/SKILL.md`'s word budget was raised at **11 words of margin rather than after a breach** — the near-zero state the budgets table's own comment forbids. Recorded here because "did not trip the budget check" is exactly the phrasing the `budget-headroom-is-spent-before-it-is-breached` finding exists to stop.
