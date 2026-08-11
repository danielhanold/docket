# Resume the docket drain (paused 2026-08-11 for a session restart)

State at pause: **no open claims, nothing in flight.** 8 changes merged and archived across two loops.
Loop 1 (0286, 0270, 0277, 0208) is fully complete. Loop 2 is 4/10 done.

## After restarting, run this

```
/loop drain the remaining 6 changes one per iteration in this order: 0247, 0118, 0268, 0221, 0251, 0252.

Per iteration:
1. Refresh state (preflight), then pick the FIRST id in the order above that is not yet done/blocked.
2. If the picked id is already in-progress from a crashed iteration: resume docket-implement-next with the EXPLICIT id, first establishing what is already built from the branch's actual commits (read the manifest's branch:/pr: fields and git log — never trust an unticked plan or a subagent's completion prose).
3. Otherwise dispatch docket-implement-next <id> FOREGROUND. On return verify git state: preflight, then `"${DOCKET_SCRIPTS_DIR:?}"/docket.sh verify-run <id>`, keying on the report line (run-complete / run-unclaimed / run-halted / run-incomplete), never the exit code. Never re-dispatch a halt. Re-dispatch an incomplete at most once.
4. Then dispatch docket-finalize-change <id> to merge, archive, refresh the board. On a denied merge or failed gate: leave the Finalize-blocked marker, retry at most once, move on.
5. End when all 6 are done or blocked, with a per-id summary.

Read /Users/homer/dev/docket/.docket/RESUME-DRAIN.md first — it carries the reconcile context every dispatch needs.
```

## Order rationale (already analyzed — do not re-derive)

Remaining shared-file clusters:
- `scripts/docket-status.sh` — 0247, 0118, 0268
- `scripts/run-tests.sh` — 0268, 0221, 0251, 0252
- `tests/runtime-budgets.tsv` — 0221, 0251
- **0252 is the widest** (tests/lib hardening: `fixture_lib.sh`, `sync_agents_common.sh`, ~8 test files, 4 scripts) → last

0247 → 0118 → 0268 keeps the docket-status cluster adjacent, with 0268 bridging into the
test-infra cluster; 0221 → 0251 are adjacent on budgets; 0252 absorbs every rebase.

## Merged this session — reconcile context for every remaining dispatch

Specs for the remaining 6 all predate these, so each dispatch must reconcile against them:

| Change | Merge | What it established |
|---|---|---|
| 0286 | `fc482699` | Correct caller poll-loop shape, taught in `scripts/gate-run.md` § *The caller's loop*: capture-then-match with `\|\| true`, `case` arms prefix-matched on the WHOLE printed line, only `state=running*` retried, fail-closed unknown arm, bounded by `GATE_OBSERVATION_BUDGET`. **Reuse it; never invent one.** |
| 0270 | `d0197ee5` | Runner-config locality fence; `runner-dispatch.sh` left byte-identical |
| 0277 | `6cc79e8b` | **ADR-0082** — delegated briefs travel via `--brief-file`, not argv; adapters REFUSE brief-file-plus-trailing-argv. Raised `test_runner_dispatch.sh` row 10s→20s |
| 0208 | `07de6e55` | **ADR-0083** — worktree membership gate keyed on a declared `worktree-scope:` frontmatter fact; flag-parse guards; gate 3b conditioned on `ANCHOR_FALLBACK != 1` |
| 0275 | `cdbcfa4c` | **ADR-0084** — re-dispatch permission gated on mechanical attribution capability, not launch shape; adds "unattributed mode" (verify and report, never re-dispatch) |
| 0281 | `08a96566` | **ADR-0085** — critic verdict travels on ONE channel, the foreground return; name-addressed message-back banned both directions |
| 0260 | `fe486696` | **ADR-0086** — in-context gating dispatch carved OUT of the 0137 A/B/C tier taxonomy; push-denial named in `gate-failure.md` |
| 0284 | `ab4277e1` | **ADR-0087** (a liveness probe's non-zero answer is not evidence of death; only a failed `kill -0` is) and **ADR-0088** (a halt's exit code is a property of run state, not discovery path — halts exit `3`). New `scripts/lib/docket-liveness.sh` |

## Standing rules to pass into every dispatch

- **Derive any site list from a whole-repo grep, NEVER hand-list.** 0208 found a fourth population of dispatch prose at `cursor-rules/dispatch/`; 0281 found FIVE populations where its spec assumed one.
- **Deletion and inversion are different mutation probes** — a comparison operator needs both (0275: deleting a conjunct reddened a guard while inverting the same comparison left it green).
- **Confirm a probe actually changed bytes before trusting its green.** Against hard-wrapped prose a single-line `perl` pattern silently no-ops, and a mutation that never applied is indistinguishable from a guard that failed to catch (0281).
- **A mutation probe checks exact removed/added LINE COUNTS, never a token count** (0284 — a probe passed its token landing check while silently deleting 128 lines).
- **PLAN-SUPPLIED TEST/PROBE CODE IS THE DRAIN'S DOMINANT FAILURE MODE.** Defective in 0286 (×2), 0281, 0260 (×3), 0284 (×7 distinct places). Run it; expect the landing check itself to be wrong. Tracked systemically as **#0292**.
- **DO NOT run `scripts/run-tests.sh --timings <test path>`** against a real test file — it truncates it to zero bytes (**#0290**, unfixed).
- Any file pushed past its `tests/runtime-budgets.tsv` row must be re-measured and the row RAISED with a measured number.
- `tests/test_sync_agents_runners.sh` at ~190s vs a 60s ceiling is PRE-EXISTING, tracked as **#0280** — leave it alone unless the change IS 0251.
- Current margins: `test_runner_dispatch.sh` row 20s (0208 measured 16s); observe shard 30s (0284 measured 22.5–23.2s, ~7s margin that **0252** inherits).

## Stubs minted during the drain — NOT part of the 10, do not build in the loop

- **#0290** (raised to high) — `run-tests.sh --timings <test path>` truncates the named test file to zero bytes. Destructive.
- **#0291** — load `gate-failure.md` before the dispatch verb at both finalize gate steps. The declined review finding 6; the reviewer called it *"the one delivery question the whole design turns on."* **Needs a human brainstorm.**
- **#0292** (high) — shared, tested mutation-probe harness; take the landing check out of each plan author's care.
- **#0293** (high) — `test_gate_run_stop.sh`'s TERM-escalation fixture deadline sits at exact parity (10.0s vs 10.0s) with the production budget it waits on. Latent flake, reproduces on clean `origin/main`; it can redden the suite for any remaining change.

## Open human to-dos

1. ~~Run `sync-agents.sh`~~ (done during 0275) — **the restart itself completes this**; wrappers load at process start.
2. Validate 0277's regenerated shim: one live delegated dispatch from the fresh session, confirming `$DDIR/brief` holds the task text.
3. #0291 needs grooming — it will not move autonomously.
4. Consider flipping #0293 to `high`.
5. Two deliberate calls made under the autonomous-drain instruction, both reversible only by a new ADR + follow-up change:
   - **0281** — a transient return-channel fault now permanently disarms a healthy stub (recovery: re-arm `auto_groomable: true`, delete its `## Auto-groom blocked` section).
   - **0284 / ADR-0088** — halts exit `3`, deviating from a spec row the build found internally inconsistent.
6. For whoever grooms **#283** (slim-agents-md): the run-gate block is now the largest single always-loaded contributor, grown 25→48 lines across three separately-argued bound raises — a slim must argue against each individually.
