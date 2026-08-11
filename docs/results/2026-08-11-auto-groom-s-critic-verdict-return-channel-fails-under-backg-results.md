<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0281 — Auto-groom's critic verdict return channel fails under background dispatch](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0281-auto-groom-s-critic-verdict-return-channel-fails-under-backg.md)**
<!-- docket:backlink:end -->

# Auto-groom's critic verdict return channel — results

Change: #0281 · Branch: feat/auto-groom-s-critic-verdict-return-channel-fails-under-backg · PR: (see change `pr:`) · Plan: docs/superpowers/plans/2026-08-11-auto-groom-s-critic-verdict-return-channel.md · ADRs: 0085

## Verify (human)

- [ ] **Judge the disarm trade-off in ADR-0085's sub-decision.** The no-verdict route now takes the *full* Abstain exit, `auto_groomable: false` included — so a transient plumbing fault permanently disarms a stub whose draft was fine, until you re-arm it. The build chose this because the alternative breaks the drain's provable-termination invariant (a stub left armed is re-selected at unchanged rank and spins on an unattended run). The reasoning is sound, but the *cost* is a product judgment only you can accept: after a return-channel outage you will find one or more stubs sitting at `auto_groomable: false` with a `## Auto-groom blocked` section, needing a flag flip each.
- [ ] **Confirm the fix against the real failure, if you can reproduce it.** Everything here is prose-contract and guard work; no automated test can prove a live critic now returns its verdict on the dispatch return. The next real `docket-auto-groom` run over an armed stub is the first genuine exercise.

## Findings

**ADR-0085 — the critic's verdict travels on exactly one channel.** The foreground dispatch return, banning name-addressed message-back in both directions, with a bounded no-verdict posture (one collect, one re-dispatch, then Tier B abstain). Full decision trail, rejected alternatives, and the termination-invariant argument are in the ADR.

**Review returned 6 findings (0 blocker, 2 important, 4 minor); all 6 fixed in-branch.** Two are worth reading beyond their commits:

- *Finding 1* — the guard's original delivery assert was satisfiable by a **conditional** restatement in a later paragraph, so deleting the critic's entire unconditional delivery contract left all five critic-side asserts green. The change's actual thesis clauses on the critic side — exclusivity ("the only channel your verdict travels on") and "your dispatcher is blocking on it" — were bound by nothing. Fixed by slicing the assert to the paragraph carrying the bolded lead and adding asserts on both thesis clauses.
- *Finding 6* — "one fresh **foreground** re-dispatch" said nothing about what makes the second dispatch block when the first did not. On a harness that backgrounds by default — the exact failure this change is named for — the retry leg would simply repeat the first, leaving the collect attempt as the only real recovery. The posture now says the re-dispatch is issued through whatever mechanism makes the parent block on the return, and that where none does, the leg is skipped straight to Tier B rather than spent.

**Two defects in this change's own plan, worked around rather than edited** (the plan is a frozen build record):

- Task 1 Step 5's Probe 1 strips markdown emphasis only, so it leaves the guard green *legitimately* — it is not a valid mutation probe. The worker substituted two real deletion probes and recorded the substitution. Anyone re-running the plan's literal probe will see a passing probe that proves nothing.
- The plan's `scripts/run-tests.sh -j 1 --timings <test path>` is malformed: `--timings` takes an **output** path, so the named test file becomes the timings sink and is truncated to zero bytes. It happened once and the file was reconstructed byte-identically. Correct form is `--timings <out.tsv> <test path>`.

**A mutation-probe trap specific to hard-wrapped prose.** A single-line `perl` mutation pattern silently no-ops against a clause wrapped across two lines, and the resulting green reads exactly like a surviving hole in the guard. Caught by checking `git diff --stat` on the probe rather than trusting the test verdict. Any probe against wrapped prose must confirm the mutation actually applied before the verdict is read. (Candidate for the learnings harvest at close-out.)

**Three size/runtime ceilings moved, each argued in-diff.** `skills/docket-auto-groom/SKILL.md` 66/1300 → 70/1500 (across two raises, compression attempted first each time), `skills/docket-convention/SKILL.md` 6400 → 6450 words, and `EXPECTED_TOTAL` 1670 → 1680 for the new guard's row. The `references/`-home argument against minting `skills/docket-auto-groom/references/critic-dispatch.md` rests on the change-0137 ground: a rule that fires at the moment of action cannot live in a file read ahead of it.

**Not this branch's:** `tests/test_sync_agents_runners.sh` runs ~190s against a 60s ceiling. Pre-existing, tracked as change #0280, deliberately untouched.

## Follow-ups

- **#0289 — Bind budget-ledger entries to the numbers they narrate** (auto-captured this run). Review findings 3 and 4 shared one root cause: a hand-maintained ledger comment drifted from the code it narrates, and nothing enforces the correspondence. Both `EXPECTED_TOTAL`'s history block and the `BUDGETS` argument block exist so a quiet raise stays auditable, and both were caught only by a line-by-line human-equivalent read.
- **Four populations of critic-dispatch prose were surveyed and deliberately not edited** — `cursor-rules/dispatch/docket-auto-groom-critic.md`, `AGENTS.md`'s generated `docket:dispatch` block, `agents/harness-defaults.yml`, and `README.md` / `docs/codex/`. Each addresses the *parent* and already states foreground dispatch; the delivery clause added here binds the *critic*, which loads only its own agent source plus `docket-convention`. Recorded in the change's `## Out of scope` so the exclusion reads as a decision rather than an oversight.
- **Change 0260** is queued immediately after this one and also edits `skills/docket-convention/SKILL.md`. This branch's diff to that file is confined to the `**Composition (change 0017).**` paragraph — one line changed, verified mechanically — so 0260 has a clean rebase base.
