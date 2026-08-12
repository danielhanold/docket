# Drain record — closed 2026-08-12

**This drain is finished. Nothing here is a resume instruction any more.**
Kept as the record of what landed and what it left behind.

## Outcomes

Loop 1 — all 4 merged:

| id | PR | Merge | Notes |
|---|---|---|---|
| 0286 | #192 | `fc482699` | taught the caller poll-loop shape in `scripts/gate-run.md` |
| 0270 | #193 | `d0197ee5` | left `runner-dispatch.sh` byte-identical |
| 0277 | #194 | `6cc79e8b` | **ADR-0082** |
| 0208 | #195 | `07de6e55` | **ADR-0083**, ADR-0068 updated |

Loop 2 — 7 merged, 1 killed, 2 withdrawn by the human:

| id | PR | Merge | Notes |
|---|---|---|---|
| 0275 | #196 | `cdbcfa4c` | **ADR-0084** |
| 0281 | #197 | `08a96566` | **ADR-0085** |
| 0260 | #198 | `fe486696` | **ADR-0086** |
| 0284 | #199 | `ab4277e1` | **ADR-0087**, **ADR-0088** |
| 0247 | #200 | `74895565` | **ADR-0089** |
| 0118 | #201 | `94d70b52` | **ADR-0090** |
| 0268 | — | — | **killed at reconcile** — already fixed on main by #0276 (`3b93574d`) |
| 0221 | #202 | `753e1ede` | **ADR-0091** |
| 0251 | — | — | **removed from the loop by the human; never built.** Still `proposed`. |
| 0252 | — | — | **removed from the loop by the human; never built.** Still `proposed`. |

## Left open — needs a human

1. **Two policy calls from 0221**, neither resolved by the merge:
   - The hygiene gate is a pre-flight inside `scripts/run-tests.sh`, so `bash tests/test_foo.sh` bypasses it. Acceptable, or should it reach standalone runs?
   - ADR-0091 makes the backtick ban **total**, outlawing *intentional* legacy backtick substitution in `tests/`, not just the hazard. That is a repo-wide style mandate — the narrower alternative was considered and rejected.
2. **0247** — main-mode preflight now *aborts* when the primary worktree is not on `INTEGRATION_BRANCH` rather than rebasing a topic branch. Docket is docket-mode, so the suite exercises the mechanics but not the judgement.
3. **#0291** needs grooming — the declined review finding 6, called *"the one delivery question the whole design turns on."*
4. **0277's shim** was never runtime-validated: one live delegated dispatch confirming `$DDIR/brief` holds the task text.
5. Two calls made under the autonomous-drain instruction, reversible only by a new ADR + follow-up change:
   - **0281** — a transient return-channel fault permanently disarms a healthy stub (recovery: re-arm `auto_groomable: true`, delete its `## Auto-groom blocked` section).
   - **0284 / ADR-0088** — halts exit `3`, deviating from a spec row found internally inconsistent.

## Stubs minted during the drain

| id | Priority | What |
|---|---|---|
| #0290 | high | `run-tests.sh --timings <test path>` truncates the named file to zero bytes — destructive |
| #0291 | medium | load `gate-failure.md` before the dispatch verb at both finalize gate steps — needs brainstorm |
| #0292 | high | shared, tested mutation-probe harness |
| #0293 | high | `test_gate_run_stop.sh` fixture deadline at exact parity (10.0s vs 10.0s) with `stop_run`'s TERM budget |
| #0295 | high | `render-change-links.sh` documented offline-safe but fetches and dies on failure |
| #0296 | medium | shard `tests/test_docket_status.sh` — its row is at the table's hard 60s ceiling |
| #0297 | medium | relax 0212's `SITES` backtick ban now the hygiene gate enforces it repo-wide |

## Budget state at close

- `tests/test_docket_status.sh` — **at the hard 60s ceiling, no next raise.** #0296 shards it; #0154 also targets it.
- `tests/test_sync_agents_runners.sh` — ~193s against 60s, the largest breach in the table. Pre-existing, #0280.
- `tests/test_run_tests.sh` 20 → 30s and new row `test_assert_hygiene.sh 10` (0221, measured).
- `test_runner_dispatch.sh` 20s row (0208 measured 16s); observe shard 30s (0284 measured 22.5–23.2s).

## The drain's dominant failure mode

Plan-supplied test and probe code shipped defective in **eight** changes: 0286, 0281, 0260, 0284, 0247, 0118, 0221. Every instance was caught by *running* it; none shipped. Worst cases:

- 0247 — a fixture cleanup whose `rm -rf` resolved against the test's own cwd, because `git -C … rev-parse --git-dir` prints a **relative** path. Destructive, not merely wrong.
- 0118 — a `mkgitfail` wrapper whose scan broke on `-C`'s *value*, which would have made **eleven** fault-injection asserts vacuous while the suite stayed green.
- 0221 — a lexer reading backslash-newline as an escape rather than a splice, inert on **55% of the suite**, hidden because every red fixture wrote its hazard on one physical line and the only continuation fixture was green.

The rules that came out of it, all now in the learnings ledger: a fault-injection wrapper or landing check is an **instrument, not an assert** — probe the probe; a fixture matrix must cross each **rule** with each **line shape**; a mutation probe checks exact removed/added **line counts**, never a token count; deletion and inversion are different probes. **#0292** is the systemic fix, and 0221 supplied the argument no earlier instance had: a guard on the oracle cannot be validated by the oracle.
