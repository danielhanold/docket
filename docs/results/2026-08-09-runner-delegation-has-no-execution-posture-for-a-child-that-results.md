<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0271 — Runner delegation has no execution posture for a child that outlives its foreground call](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-09-0271-runner-delegation-has-no-execution-posture-for-a-child-that.md)**
<!-- docket:backlink:end -->

# Runner delegation — detached execution posture — results

Change 0271. Branch `feat/runner-delegation-has-no-execution-posture-for-a-child-that`.
Suite green at `37829f5e`: 100 files, 7480 asserts, 0 failures.

## Verify (human)

1. **Re-probe the four adapter launch shapes — the one thing this change could not measure.**
   `skills/docket-build/references/delegation-execution.md` ships **all four harness rows as
   `unverified`**, deliberately. `gate-execution.md`'s verdicts were measured for a *gate* launch (a
   test command) and are explicitly version- and scope-scoped, so they do not transfer to an
   *adapter* launch. Re-probing needs each child CLI installed and authenticated, which an
   autonomous run cannot do. **This change therefore lands the mechanism with no evidence that any
   child CLI tolerates being started detached.** The probe recipe is in that file's
   `## Probe recipe`; it changes one variable per run and says what counts as inconclusive versus
   `incompatible`. Until it is run, treat delegation on a real runner as unproven.
   One caution from writing the recipe: isolate the launching shell into its own process group
   before sending it a group-directed signal, or the probe kills the harness running it.

2. **Watch one real delegated run end-to-end.** The mechanism is proved hermetically (fake adapter),
   not against a live child. Worth confirming the relay in particular: the child's stdout now
   reaches the caller only because `--observe` copies `stdout.log` to its own stdout on terminal
   paths. A silent `build-*` worker is the symptom if that path regresses.

## Findings

**Two review blockers, both regressions this branch introduced, both fixed in-branch.**

- **The relay was severed** (`b8672268`). `--launch` redirects the adapter's streams into the
  dispatch dir and every `--observe` diagnostic goes to stderr, so nothing ever wrote the child's
  captured stdout to stdout. Every in-context-report agent — `build-*` returning
  COMPLETE/NEEDS_ESCALATION/BLOCKED, `review-*` returning findings — would have returned **nothing**
  to its caller, while git-state agents (status, adr) kept working. The relay is now sentinel-gated:
  it fires only where the child is finished and its output complete, never on the still-running
  path (which the shim polls repeatedly) and never on the own-group refusal (where the child is
  still writing).
- **Change 0237's run gate went dead** (`fdebb73d`). Rewriting the shim to always `--launch` left
  `GATE=0; [ "$AGENT" = "implement-next" ] && GATE=1` sitting after both verbs' `exit` — unreachable
  for every delegated run — while `runner-dispatch.md` still claimed "`implement-next` — unchanged".
  A delegated run that halted, or stopped before its PR, would have exited 0 at the adapter and
  observed as `complete`. That is exactly the prose-level failure 0237 was built to eliminate.
  `--observe` now carries the attribution snapshot (captured at launch, both reads from fresh
  origin) and synthesizes `3` on `run-halted`. **No auto re-dispatch from an observation** — that is
  a decision, not an omission: re-launching a detached child from an observation races the run being
  observed.

**Three importants, fixed.** The build verdict's `branch` conjunct was structurally vacuous on its
only consumer — it compared the anchor's branch to itself — so a child ending on the wrong branch or
a detached HEAD reported `task-committed`; the branch is now recorded *at launch* (`c023852d`),
and `tip` became a descendancy check rather than an inequality. The budget kill signalled a pgid
read from a file with no identity check, so a **recycled pgid** got TERM then KILL; it now verifies
`child_pid` **and** `child_lstart` first (`17d61fb0`). And the observation loop had states it could
never leave — three facade paths returned `4` forever, making the shim's "the facade stops on its
own" false — now bounded on both sides (`617914ff`).

**Eleven minors, all fixed** (`72f256bb`, `9d9c293b`, `37829f5e`), including a check-then-act race
where a sentinel landing between the "no sentinel" read and the kill was masked permanently,
reporting completed work as lost.

**Three guards the plan specified were vacuous, and the workers caught them.** The `set -m` mutation
assert passed with `set -m` removed (it read a fallback field rather than the live process); the
budget arm's "adapter never completed its work" could not fail in its window; and the `build-*`
observe-only assert measured nothing. Each was rewritten to bind on the mechanism and re-proved. The
plan's `HD_SHIPPED_HARNESSES` extractor also pointed at the wrong file and would have returned empty.
Worth noting for its own sake: **plan-supplied test code is unverified code, not an oracle.**

**A portability trap.** PATH `grep` here is ugrep, which accepts `\b`; stock BSD `/usr/bin/grep`
does not. The plan's exit-4 assert used `\b4\b` and would have passed locally while rotting for
anyone on stock grep. New regexes were re-checked under `/usr/bin/grep`.

## Notable deviations

- **Detachment is a new process GROUP, not a new session.** `setsid` is absent on macOS and docket
  takes no `perl` dependency, so `set -m` job control is the mechanism. Measured 2026-08-09, one run
  with two arms and one variable: the `set -m` child survived a group-directed TERM; the non-`set -m`
  child did not. That satisfies capability 1's stronger reading; it does not obtain a controlling-
  terminal detach, and ADR-0080 says so rather than overclaiming.
- **Exit `4` is a new caller-visible non-failure code.** Its only consumer is the generated shim,
  changed in the same change to loop on it — the discipline `exit-code-encodes-a-non-failure`
  demands. `--observe` never returns `3` from the sentinel path alone; `3` is the halt verdict.
- **`tests/test_runner_dispatch_detach.sh` was sharded** into launch / observe / build-gate (its row
  had reached the table's hard ceiling of 60 with no headroom). 184 asserts before, 36+88+60 = 184
  after — a move, verified by diffing the extracted bodies.
- One accepted residual, stated in `runner-dispatch.md`: a group whose leader died while its children
  live is now left un-signalled, trading a visible orphan for never killing an unrelated group.

## Follow-ups

- **Change 0274** (auto-captured): `runner-dispatch.sh`'s five value-taking flags use `shift 2`, and
  bash's `shift` fails rather than truncating, so a flag in final position with no value **hangs** —
  `timeout 3 bash scripts/runner-dispatch.sh --runner` returns 124. Pre-existing, independent of this
  change, and it renders five validation branches unreachable.
- **Suite budget breaches are contention artifacts, not this branch's.** The full run reported 9
  files over budget; two runs of the same branch reported different sets (6 then 9) at similar
  absolute times, which is the signature change 0229 exists to remove. This branch's own three new
  files land at 7s / 24s / 8s against their rows. `test_docket_config.sh` grew ~53 lines here and is
  in the breach set; the rest are untouched by this change.
- The `killed`-marker path still cannot distinguish a never-terminal dispatch from one abandoned
  mid-flight; retention keeps such a dispatch forever by design (7-day prune skips anything without a
  terminal marker).
