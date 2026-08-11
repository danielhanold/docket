<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0284 — runner-dispatch --observe is sentinel-only: adopt 0282's identity-checked liveness probe](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0284-runner-dispatch-observe-is-sentinel-only-adopt-0282-s-identi.md)**
<!-- docket:backlink:end -->

# `runner-dispatch --observe`: an identity-checked liveness probe — results

Change: #0284 · Branch: `feat/runner-dispatch-observe-is-sentinel-only-adopt-0282-s-identi` · PR: see the change's `pr:` field · Plan: `docs/superpowers/plans/2026-08-11-runner-dispatch-observe-liveness-probe.md` · ADRs: 87, 88

## Verify (human)

- [ ] **The `3`-not-`1` halt code is the one judgment call worth your eye** (ADR-0088). A halt discovered through the new liveness probe exits `3`, deviating from the spec's §3 table, which said `1`. The spec row was internally inconsistent — it named `observe_implement_next` as the reader, and that function exits `3` — and `3` is what the parent-facing run gate keys on to never re-dispatch a halt. The whole-branch reviewer was asked to form an independent view and reached the same conclusion. Reversing it is a one-line change in `vanished_code` plus two asserts.
- [ ] **Confirm the orphan residual is still the posture you want.** A supervisor that died while processes it spawned keep running is now reported dead *and those orphans are not reaped* — the same residual the give-up path already accepted, but it now shapes a **verdict** one lifecycle phase earlier rather than only a kill decision. Reversing it means signalling a group that cannot be proven still ours, which is an ADR-level decision and deliberately out of scope here.

## Findings

**Two ADRs came out of the build.**

- **ADR-0087 — a liveness probe's non-zero answer is not evidence of death; only a failed `kill -0` is.** The predicate was written uniformly fail-closed, with a justification imported verbatim from `gate-run.sh`: *"a false `dead` costs one wasted observation."* True there — one bounded relaunch of an idempotent suite. **False for this consumer**, where a false `dead` writes a terminal marker, ends the caller's loop permanently, and — because git decides the code — can return **`0`** for a child that is still running, letting a driver draw the next change while the live run keeps writing. Reachable, not theoretical: `ps -o lstart=` is `TZ`/`LC_TIME`-dependent, so a `--launch` and an `--observe` under different environments make a healthy child unprovable. Fixed by splitting the answer into `gone` (positive evidence) and `unprovable` (everything else), routing only the first to a terminal disposition and the second through the pre-existing `note_unenforceable` counter that already bounds itself at three passes. The generalizable rule: **the cost asymmetry that justified failing closed does not travel with the code** — re-derive it for each consumer.
- **ADR-0088 — a halt's exit code is a property of the run's state, not of how the facade learned it.** See *Verify* above.

**The build gate went red once, and it was not this branch.** `tests/test_gate_run_stop.sh` failed under the full 106-file suite at a single assert while passing standalone, under 4× self-contention, and on a clean `origin/main` worktree under the same load. Root-caused by reading the two sides rather than by re-running until green: that leg's child does `trap "" TERM`, so `stop_run` must exhaust its **10.0s** TERM wait (`waited -lt 20` × `sleep 0.5`) before escalating to KILL and reaching the barrier — and `wait_for_file` defaults to **exactly 10.0s** (`ticks=100` × `sleep 0.1`). The fixture gives up at the same instant the production path is expected to act. Zero margin, pre-existing, and nothing in the repo relates the two numbers. Captured as **#0293**; not fixed here because this change's own safety net is `tests/test_gate_run_stop.sh` passing *unchanged*.

**Plan-supplied test and probe code was defective in seven distinct places**, caught by workers rather than shipped. Recorded because the pattern is now the dominant failure mode in this drain (tracked as #0292):

- A probe whose token landing check passed **while it silently deleted 128 lines** — a non-greedy `.*?` latched onto the wrong `esac`. Remedy adopted branch-wide: every probe now checks exact removed/added line counts via `git diff --numstat`, never a token count alone.
- Two fixtures that were **same-second `ps -o lstart=` coin flips** — they rewrote `pgid` to name another live group but left `child_lstart` to luck, so on the unlucky half they never reached the guard they were written to measure.
- A `kill`-override probe asserting an **empty** log, which is indistinguishable from a shim that never intercepts. Repaired with a positive control before the loop.
- A barrier-inertness arm whose child finished **before the call site was reached**, so it was green against any mutation.
- An idempotence assert that was **architecturally unsatisfiable** (it compared the terminal transition against the marker re-read).
- A relay assert whose fixture had an **empty `stdout.log`**, so it could not have measured the finding at all.
- One mutation during the fix loop came back **green and was a defect in the test**, not an untestable invariant — the assert keyed on a phrase the reason string already carried, leaving the headline itself unguarded.

**Two spec deviations beyond the ADRs**, both recorded in the change's reconcile log:

- `identity_of` and `identity_matches` **survive in `gate-run.sh` as thin delegations** rather than being deleted as spec §1 says. Two *source-shape* asserts in `tests/test_gate_run.sh` pin those spellings verbatim, and the spec's own testing rule makes that file passing unchanged the proof the refactor was behaviour-preserving. Deleting the symbols would have forced the very edit the safety net exists to detect. The predicate still has exactly one definition; only the spellings stayed.
- The dead path **inherits change 0208's two non-verdict legs** (`task-unverifiable worktree-removed`, `task-unverifiable launch-branch-missing`), which spec §3's table omitted. Reproducing the table literally would have regressed ADR-0083 onto the new leg.

**Budget margin, as a number.** `tests/test_runner_dispatch_observe.sh` was re-budgeted **25s → 30s** and measures **22.5–23.2s** across runs, leaving **~7s of margin** (worst observed sample 24.2s, i.e. 5.8s). Change **#0252** is queued against `scripts/runner-dispatch.sh` and inherits that margin. `tests/test_docket_liveness.sh` is new at the table's 10s floor, measuring 0.06s. `EXPECTED_TOTAL` moved 1680 → 1695 across two commits, each naming its case. The suite's only `OVER BUDGET:` line is `test_sync_agents_runners` at ~187s, which is pre-existing (#0280) and untouched.

## Follow-ups

- **#0293** (minted this run) — `test_gate_run_stop`'s TERM-escalation fixture deadline is at exact parity with `stop_run`'s own TERM budget. Give it real headroom, ideally *derived* from the production budget rather than restated so the two cannot drift apart again, and sweep `wait_for_file`'s other call sites for the same parity.
- **Not minted, recorded here instead** — `tests/test_gate_run.sh` hand-rolls its own `ps -o lstart=` rendering rather than reading it through the shared helper, which leaves that one assert locale-fragile under a non-English `LC_TIME`. It is why the identity read is pinned with `unset TZ` rather than `TZ=UTC`. This folds naturally into #0293's boundary (fixture robustness in the `test_gate_run*` family) rather than deserving its own stub.
- **Residual, deliberate** — a genuinely recycled pgid whose child's work *did* land in git now reports unavailable after three bounded passes instead of `0` on the first, because the bounded terminal path does not consult git. Fail-closed toward "we cannot say" rather than toward a wrong verdict (ADR-0087).
- **Residual, deliberate** — a `child-vanished` marker written by an older build carries no `disposition=` field and replays fail-closed as `1`, so a dispatch disposed across an upgrade boundary can transition `3` → `1`. The safe direction (ADR-0088).
- **Residual, noted** — `terminate_dispatch`'s own identity conjunct is now largely shadowed: a dispatch whose token mismatches is disposed at step 3 and never reaches it. It stays reachable only through a genuine race (identity changing between step 3 and the give-up), which is not testable.
