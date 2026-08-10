<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0286 — Caller-authored gate-run --observe poll loops strip the state= prefix and never terminate](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0286-gate-run-observe-poll-loops-strip-state-prefix.md)**
<!-- docket:backlink:end -->

# Caller-authored `gate-run --observe` poll loops strip the `state=` prefix and never terminate — results

Change: #0286 · Branch: feat/gate-run-observe-poll-loops-strip-state-prefix · PR: <url> · Plan: docs/superpowers/plans/2026-08-10-gate-run-observe-poll-loops-strip-state-prefix.md · ADRs: none

## Verify (human)

No manual checks. The whole deliverable is machine-verified: the canonical loop is **extracted from
`scripts/gate-run.md` and executed** by `tests/test_gate_run.sh` against a stubbed `--observe`, so
every claim the contract makes about it is a passing assert rather than prose. Nothing here needs a
human at a terminal.

## Findings

**The defect class is caller-side, and the helper was never wrong.** `--observe` printed
`state=passed` correctly the whole time; the live loop that burned its budget had re-tokenized the
line with `awk '{print $1}'` and matched bare `passed`. Worth stating plainly because it decides
where a fix belongs: the taught surface, not the helper. A `--wait` verb would have reversed
`gate-run.md`'s stated invariant *"The helper never polls for the caller"* — rejected at groom as a
contract-level decision, and this branch honors that. The invariant line is byte-untouched.

**Two defects in the plan's own supplied test code, both caught by running it as code.** The
recurring lesson (`plan-supplied-test-code-is-unverified`), hit twice in one plan:

1. The supplied `loop_sec` slicer used a `/^#/` terminator, which closes on the **fence's own first
   comment line** — three lines into the section. The slice was ~3 lines, so its own `>= 20`
   non-vacuity anchor could never pass against *any* correct implementation. Replaced with a
   fence-aware slicer that closes only on an out-of-fence markdown heading. This is
   `section-slice-needs-a-named-terminator` in a new dress: not a heading-shaped line inside a fence,
   but a *comment*-shaped one.
2. The supplied fixture shadowed only `sleep`, which would have made mutation key (b) spin for the
   fixture's real 5-minute budget — twice — against a plan that claimed "milliseconds". The harness
   now shadows `sleep` **and** `date` to simulate wall clock, so the fence still runs byte-unmodified
   and the mutation reddens instantly.

**A guard that was vacuous against its own named mutation, disclosed by the build and then fixed at
review.** `grep -qF -- "|| true" <<<"$loop_fence"` stayed green with `|| true` deleted from the
executable line, because the fence's *explanatory comment* also quotes the literal. The build
recorded it as an accepted residual; review disagreed, and the repair (match comment-stripped lines)
was proven by A/B against the mutated file — old expression green, new expression red. The general
shape: **when a guard reads a document that discusses the thing it guards, prose and invocation are
byte-identical**, and the assert must strip one from the other.

**An unguarded precondition that mimics a legal configuration.** The loop read
`GATE_OBSERVATION_BUDGET` bare while guarding its sibling with `${DOCKET_SCRIPTS_DIR:?…}`. Bash
arithmetic turns an unset name into `0`, and a `0` budget is *legal* — it buys exactly one
observation. So a missing export was indistinguishable from a valid config, and would have halted a
healthy build 30 minutes early. Fixed with `${GATE_OBSERVATION_BUDGET:?…}` plus a fixture whose
harness deliberately drops `-u` from `set -euo pipefail`: under `set -u` bash aborts on the unset
name by itself, which would have left the new assert green with the `:?` guard deleted — a vacuous
assert hiding inside a stricter shell.

**A test helper that could hang the suite instead of reddening.** `run_loop`'s only exit was the
fence's own deadline check, so a mutation removing that check would hot-spin — and
`scripts/run-tests.sh` reports `OVER BUDGET` advisorily but never kills a job. The stub now hard-stops
past 200 observations and emits `state=LOOPCAP`, which the fence's fail-closed `*)` arm disposes as
`unavailable`: a runaway resolves to a comparable value (`unavailable|201`) rather than hanging. A
guard whose failure mode is a hang is not a guard — the same class as
`stacked-gap-regex-hangs-instead-of-failing`.

**A slice coupling that fails safe but opaquely.** The sentence added to
`skills/docket-build/SKILL.md` must never begin a line, because `para()` in
`tests/test_gate_execution_posture.sh` closes its slice at a column-0 `**`. A routine reflow would
redden three sentinels against a file where the sentence is plainly present. The constraint is now
recorded in the `(12a-ii)` comment beside the asserts that depend on it.

## Follow-ups

- **`OVER BUDGET: test_sync_agents_runners`** — the full-suite gate reported it on both runs (186s
  against a 60s ceiling). Advisory only; the file is untouched by this branch. Already tracked as
  change **#0280** (*Shard or re-budget the test files the suite runner reports OVER BUDGET*), so it
  was **not** minted as a duplicate.
- **No `--wait` verb**, deliberately. Spec assumption 1 leaves the door open for a later
  human-driven, ADR-level decision; nothing in this branch forecloses it.
- **`runner-dispatch --observe` carries the same defect class** and is deliberately untouched here —
  that surface is being reworked by changes **#0277** and **#0284** with its own vocabulary, and
  folding it in would collide.
