---
id: 258
slug: guard-the-config-suite-s-enumerated-claims-export-order-and
title: 'Guard the config-suite''s enumerated claims: export order and rung pairs'
status: in-progress
priority: medium
type: chore
created: 2026-08-07
updated: 2026-08-09
depends_on: []
related: [251]
discovered_from: [123, 125]
adrs: []
spec: docs/superpowers/specs/2026-08-07-guard-the-config-suite-s-enumerated-claims-export-order-and-design.md
plan: docs/superpowers/plans/2026-08-08-guard-the-config-suite-s-enumerated-claims-export-order-and.md
results:
trivial: false
auto_groomable: true
branch: feat/guard-the-config-suite-s-enumerated-claims-export-order-and
claimed_at: 2026-08-09T15:41:13Z
pr:
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-07-guard-the-config-suite-s-enumerated-claims-export-order-and-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-07-guard-the-config-suite-s-enumerated-claims-export-order-and-design.md) |
| Plan | [2026-08-08-guard-the-config-suite-s-enumerated-claims-export-order-and.md](https://github.com/danielhanold/docket/blob/feat/guard-the-config-suite-s-enumerated-claims-export-order-and/docs/superpowers/plans/2026-08-08-guard-the-config-suite-s-enumerated-claims-export-order-and.md) |
<!-- docket:artifacts:end -->

## Why

Consolidates #0123 and #0125 (2026-08-07 triage): the same meta-question — an enumerated claim in the config-suite's contract surface is prose-only; guard the claim or delete it — asked about two adjacent claims. One posture ruling, two applications.

Verified 2026-08-07:

- **Export-list order unguarded (#0123).** The fenced export list at `scripts/docket-config.md:344-374` (32/33 entries) is pinned only by per-key *presence* greps (`test_docket_config.sh:1248,1429,1581,1954`) and two runtime-only adjacency clusters (`:1643-1650`, `:1943`). No test reads the fence block and compares its **sequence** to `--export --format plain` output — a doc-side reorder stays green while R7 pins runtime order for a few pairs.
- **Rung-pair completeness prose-only (#0125).** Section S pins all six ordered rung pairs (s4–s9), but the "six pairs" claim lives only in the section header comment; no derivation from the resolver's layer set exists, so a fourth config layer silently leaves six cells unpinned. The blocker named in the stub has resolved: 0114 landed ADR-0054 ("convert, do not close"), so source-shape anchors are a live option.

Posture ruled at grooming (2026-08-07, autonomous, critic-gated): **guard both claims** — neither is re-specified away. Design detail lives in the spec.

## What changes

- **Leg 1:** a whole-sequence equality guard — the doc's fenced export list (extracted at the `### Emit` heading anchor) vs the resolver's `--export` key emission, both formats (`plain` = fence; `shell` = fence minus `REPO_ROOT`), plus a derived check of the doc's "33/34 lines" prose numerals. Existing R7/AUTO_* adjacency asserts stay.
- **Leg 2:** the rung-pair completeness claim becomes computed — expected ordered pairs derived from `config_scalar_get`'s layer-dispatch arms (n·(n−1)); pinned pairs declared by per-fixture `RUNG_PAIR:` markers collected across the `test_docket_config*.sh` family glob; verdict is set equality. The header comment's prose enumeration stops being the claim of record.
- Both guards mutation-proved (reorder, removal, count-stable rename, and a simulated fourth layer must redden) and written corpus-indifferent to #0251's split (no `BASH_SOURCE` whole-file scans).

## Out of scope

- Adding config layers or keys; changing emission order.
- The population-floor/sharding rework of the same file — owned by #0251; coordinate at build time (same test file; whichever lands second rebases, per 0251's spec assumptions 7/9).

## Reconcile log

### 2026-08-08 — reconcile at claim (no scope change)

Re-verified every premise of the spec against `origin/main` @ `487bfdc5`; all hold, so the
design is carried forward unchanged.

- **Leg 1 doc side.** `scripts/docket-config.md` still opens `### Emit` with "printed as
  `KEY=value` lines to stdout in this order", followed by one fenced block of **34** entries
  (`DOCKET_MODE` … `BOOTSTRAP`), `REPO_ROOT` annotated `(plain format only — see below)`, and the
  prose sentence "33 lines in `shell` format; 34 in `plain` format". Counts match the spec's
  33/34; no doc edit has landed since grooming.
- **Leg 1 runtime side.** A live `docket.sh preflight` emits the fence's sequence exactly,
  `REPO_ROOT` in position 6. The claim is true today, which is what makes it worth pinning.
- **Leg 2 resolver side.** `config_scalar_get` (`scripts/docket-config.sh`) still dispatches
  exactly three layer arms — `committed` / `global` / `local`, each calling
  `config_scalar_from_lines … "${CONFIG_LINES_<LAYER>[@]}"` — plus the `*)` die arm. n = 3,
  n·(n−1) = 6, matching section (S4-S9)'s six fixtures.
- **Leg 2 pinned side.** `tests/test_docket_config.sh` section `(S4-S9)` (changes 0106 + 0112)
  carries fixtures s4–s9 and states the six-pair enumeration only in its header comment, as the
  spec describes. The six pairs are unchanged: s4 local→committed, s5 committed→global,
  s6 global→committed, s7 committed→local, s8 global→local, s9 local→global.
- **ADR-0054** ("Cross-references … anchor on symbols or quoted clauses, never line numbers") is
  `Accepted`, so the leg-2 source-shape anchor remains licensed.
- **#0251 coupling.** Still `proposed` / build-ready — the `test_docket_config*.sh` split has NOT
  landed, so the family glob currently resolves to the single file. The corpus-indifference
  constraint (no `BASH_SOURCE` whole-file scans; iterate the glob) is honored regardless, per
  spec assumption 8; whichever change lands second rebases. No `depends_on` added.
- **Auto-capture (site A).** Nothing minted: the pass surfaced no work outside this change's
  scope that clears the six admission gates.

### 2026-08-08 — resumed at Step 5 (no scope change)

`origin/main` is still `487bfdc5`, unchanged since the pass above, so the reconcile is carried
forward as-is under the resume-safety guard. The previous run's `## Run halted` section is
removed by this commit — git history keeps it — because the run is live again.

### 2026-08-09 — resumed at Step 5 (no scope change)

`origin/main` has advanced from `487bfdc5` to `05fbb224` (changes 0242 and 0269 landed), so the
resume-safety guard's re-reconcile fired. Every premise re-verified against the new tip and all
hold — none of the three files this change reasons about moved:

- `scripts/docket-config.md`, `scripts/docket-config.sh` and `tests/test_docket_config.sh` are
  byte-identical between `487bfdc5` and `05fbb224`. The `### Emit` fence is still 34 entries with
  `REPO_ROOT` annotated plain-only and the "33 lines in `shell` format; 34 in `plain`" sentence
  intact; `config_scalar_get` still dispatches exactly the three layer arms.
- `tests/runtime-budgets.tsv` did change in 0242, but the `tests/test_docket_config.sh` row is
  unchanged at `55  parallel`, so Task 3's budget check is unaffected.
- #0251 has still not landed, so the `test_docket_config*.sh` family glob still resolves to one
  file; the corpus-indifference constraint is honored regardless.
- The previous run's `## Run halted` section is removed by this commit — git history keeps it —
  because the run is live again.
- **Auto-capture (site A).** Nothing minted. The dispatch-duration ceiling that halted the last
  two runs is already tracked as #0271, so filing it again would be a duplicate.

## Run halted

**2026-08-09** — the build is COMPLETE and GREEN; the run halted inside Step 6's fix loop because
the fix-worker dispatch could not complete. Nothing about the change's design or its code is in
question.

### What is already done and verified

- Tasks 1 and 2 are committed on `feat/guard-the-config-suite-s-enumerated-claims-export-order-and`
  (`42629ef7` leg 1, `46433f5f` leg 2). Worktree clean at `46433f5f`.
- Task 3, the full-suite gate, RAN AND PASSED on this run: `scripts/run-tests.sh` →
  **93/93 files, 7069 asserts, 0 failures, exit 0, wall 172s**. Build evidence is green at
  `head_sha: 46433f5f59e8aeacf92b149aacd163b04d477a55`, which equals branch HEAD.
- `tests/test_docket_config.sh` did **not** trip the advisory OVER BUDGET report (136s against a
  55s ceiling with a 2.5x slack factor = 137.5s threshold). The six files that did trip it
  (`test_board_checks`, `test_harness_defaults`, `test_harness_defaults_validator`,
  `test_sync_agents`, `test_sync_agents_codex`, `test_sync_agents_runners`) are all outside this
  branch's diff, which touches only `tests/test_docket_config.sh` plus the plan file.
- `tests/test_runner_opencode.sh` and `tests/test_runner_cursor.sh` both PASSED (1s each). The
  suspected `DOCKET_REPO_ROOT` / `DOCKET_RUNNER_CFG_PERMISSIONS` environment leak did not occur in
  this run's launch environment — neither variable was set.
- Review ran: rung `docket-review-standard` (no build record survived the earlier interrupted runs,
  so the default sink applied; whole-branch diff is 717 changed lines, under the 1500-line bump
  threshold). It returned **4 findings: 0 blocker, 2 important, 2 minor** — recorded below so the
  next run does not have to re-review.

### What stopped the run

The first fix task was dispatched foreground to `docket-build-standard`. That wrapper delegates to
the `opencode` runner through a single foreground `docket.sh runner-dispatch` Bash call, which ran
to the Bash tool's maximum timeout of 600000 ms and was killed (SIGTERM, exit 143). No stdout was
relayed and no child final message was produced.

The worker verified git state rather than trusting prose and reported **BLOCKED**: HEAD still
`46433f5f`, `git status --short` clean, no `tests/.focus-0258.sh` left behind. **The child produced
nothing — no commit and no partial working-tree state — so the worktree needs no cleanup.**

This is the known 600000 ms foreground dispatch-duration ceiling already tracked as **#0271**
(`implemented`, PR #188). It is not a defect in 0258 and no duplicate stub was minted for it.

Because a dispatch was resolved and ATTEMPTED and the attempt FAILED, dispatch capability is
established as unavailable per the convention's *Dispatch-capability resolution*. The fix role is
**Tier C, authorized-or-halt**, and `skills.build` resolves to `docket-build` — not the explicit
`auto` that would authorize fixing inline — so the posture is abort-and-report. Recording all four
findings and opening the PR anyway is explicitly NOT the fallback (`fix-loop.md`: "that fails the
loop open silently").

### What a human must decide

Pick one and re-run `docket-implement-next 258`:

1. **Raise or bypass the dispatch ceiling** — land #0271, or give `runner-dispatch` a backgrounded
   durable-log path with a blocking monitor keyed on exit code, so a fix worker can outlive 600 s.
2. **Make the child cheap enough to finish inside 600 s** — a faster model or lower effort for the
   `docket-build-standard` / `docket-build-economy` profiles on this repo.
3. **Set `skills.build: auto`** for this repo, which authorizes the fix loop to run inline and
   removes the dispatch dependency entirely.
4. **Accept the branch as-is** — deliberately, with the four findings unfixed — by opening the PR by
   hand. The build is green and there are no blockers, but this bypasses the fix loop rather than
   satisfying it, so it is a human's call and not the agent's.

### Review findings carried forward (do not re-review)

**F1 — important.** `tests/test_docket_config.sh`, leg 2. The `# RUNG_PAIR:` marker guard has an
existence floor (`"0258 L2 control: the family glob yielded a non-empty pinned pair population"`)
but no attachment or coverage floor: nothing binds a marker to a fixture body, so when a fourth
layer grows the expected set from 6 pairs to 12, pasting six more marker lines re-greens the guard
with zero new masking coverage. The reworded `(S4-S9)` header also overstates it — it says the
guard reddens "until six new **fixtures** exist" when it reddens until six new **markers** exist.
Fix: an awk attachment assert over the family glob (every marker followed within a small window by
a `mkrepo`), a count-equality assert so a duplicated unbacked marker reddens, and a corrected
header comment. Routed `standard`. **This is the fix task that was dispatched and never landed.**

**F2 — important.** Runtime headroom. `tests/test_docket_config.sh` lands at 136s against a 137.5s
advisory threshold, so leg 1's `l1` fixture consumes essentially all remaining slack; the next
assert added to this file, or a busier host, trips OVER BUDGET for a reason unrelated to whoever
adds it. Disposition: **deferred**, remedy owned by #0251's shard/re-baseline — the branch is
correctly constrained not to touch `tests/runtime-budgets.tsv`. The reviewer's alternate remedy
(drop the `l1` fixture's `git commit`/`git push`) was checked and is **invalid**:
`scripts/docket-config.sh` reads the committed config via `git show "origin/HEAD:.docket.yml"`, so
the push is load-bearing. Carry the 136s measurement into the PR body and results file.

**F3 — minor.** The `l1_sentence` prose-numeral check greps a hard-wrapped markdown phrase against
the raw whole document. Two issues: a pure re-flow of that paragraph would redden an assert about a
claim that never changed (`phrase-grep-over-wrapped-prose`, a four-time recurrence in this repo),
and scanning the whole file means the assert pins that the numerals exist somewhere, not that *this*
sentence carries them. Fix: flatten whitespace before matching and restrict the haystack to the
`### Emit` section slice the extractor already anchors on. Routed `standard`.

**F4 — minor.** The family glob `"$REPO"/tests/test_docket_config*.sh` admits any similarly-named
scratch or backup file in `tests/` — the hazard the plan's own amendment had to warn about for its
throwaway proof harness. A stray file doubles the pinned set and reddens set equality with a
diagnostic pointing at the markers instead of at the stray file. Fix: add the glob's matched file
list to the assert's failure output, or restrict collection to files the suite runner schedules.
Routed `economy`.

The reviewer found no `pipefail` or early-exiting-consumer violations, no `${BASH_SOURCE[0]}`
whole-file scan, no BSD-awk violations, and confirmed the R7 and AUTO_* adjacency asserts are
intact. Leg 1's core design (independent populations, whole-sequence equality, anti-vacuity
controls) and leg 2's computed expected side both check out.

### Unvalidated — requires a fresh session

The `docket-build-standard` wrapper at `.claude/agents/docket-build-standard.md` was hand-edited to
fix its caller-args passthrough (the `--` payload). **That fix is NOT validated by this run and no
verdict on it should be read into this record.** Claude Code caches agent definitions at process
start and this session predates the edit, so the dispatch above necessarily used the stale cached
definition. A `ps -eww` argv sampler was armed across the whole dispatch window and captured **zero**
samples of this run's dispatch — its only match was PID 27068, a 12-hour-old orphan
`runner-dispatch.sh --observe` belonging to an unrelated worktree. So the run produced **no argv
observation at all**, in either direction: nothing here is evidence that the passthrough fix works,
and nothing here is evidence that it fails. Re-probe from a session started after the edit.

### Environment note (unrelated to this change)

`DOCKET_SCRIPTS_DIR` currently exports
`/Users/homer/dev/docket/.worktrees/runner-delegation-has-no-execution-posture-for-a-child-that/scripts`
— a FEATURE worktree of another in-flight change, not the primary tree's `scripts/`. Preflight
resolved `REPO_ROOT=/Users/homer/dev/docket` correctly and every facade call in this run behaved,
so nothing was harmed, but docket helpers are being run from an unmerged branch's copy. Worth
repointing at `/Users/homer/dev/docket/scripts`.
