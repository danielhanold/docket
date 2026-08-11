<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0277 — Delegated task briefs travel through shell argv, a lossy model-performed transformation](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0277-delegated-task-briefs-travel-through-shell-argv-a-lossy-mode.md)**
<!-- docket:backlink:end -->

# Delegated brief-file channel — results

Change: #0277 · Branch: feat/delegated-task-briefs-travel-through-shell-argv-a-lossy-mode · PR: (opened at Step 7) · Plan: docs/superpowers/plans/2026-08-10-delegated-brief-file-channel.md · ADRs: 0082

## Verify (human)

- [ ] **One live delegated dispatch from a FRESH session.** The harness loads agent definitions at
  process start, so nothing in this run — and nothing in the suite — validated the regenerated shim
  as the harness will actually read it. Every shim claim in this branch is evidence over the
  *generator* (`sync-agents.sh` writes the bytes we intend) plus a read of the generated file. After
  merging and re-running the agent sync, restart the harness, dispatch one delegated agent, and
  confirm `$DDIR/brief` in the dispatch dir holds your caller's task text. A same-session check
  proves nothing — this was proven on change 0271, and the shim blocker below is what it looks like
  when the generated recipe is wrong in a way no test can see.
- [ ] **One `build-*` delegation with no task text.** Confirm the facade refuses it loudly rather
  than starting a worker that improvises. This is the half of 0271's silent-omission failure mode
  that this change made mechanical, and its value is only observable on a real dispatch.

## Findings

**The shim's first recipe could not work — caught at review, not by the suite (blocker).** The
initial `emit_shim` taught a two-call recipe: call 1 writes the brief with `mktemp` plus a quoted
heredoc, call 2 launches with `--brief-file <the path you just wrote>`. Harness Bash calls share no
shell state and `mktemp` produces a random suffix, so after call 1 the model has never seen the
path: `$BRIEF` is unset in call 2's fresh shell and expands empty, and the facade dies — on the sole
taught channel, for every runner and every delegated agent. The shim sentinels did not catch it
because they asserted the **slot's shape** and never that the recipe **executes to a usable value**.
Fixed by emitting the write and the launch as ONE call with a live `--brief-file "$BRIEF"`, which
removes the brief path as a model-substituted slot entirely. Recorded as **ADR-0082**, whose testing
corollary is the durable part: a sentinel over a generated instruction must pin that the recipe
runs, not that a placeholder looks right.

**Two definitions of "empty payload" (important).** `[ -s "$BRIEF_FILE" ]` measures bytes while
`payload="$(cat …)"` strips trailing newlines, so a newline-only brief passed every gate and
produced no task context at all — the improvise defect this change exists to close, reintroduced
inside the change's own additions. The argv leg had the same gap (`-- ""` satisfied `[ $# -gt 0 ]`).
Both channels now refuse on a whitespace-only payload, at the facade and in all three adapters.

**A brace group's `|| die` guards only its last command (important).**
`{ cat "$BRIEF_PATH"; printf …; } > "$RETRY_BRIEF" || die` did not catch a failed `cat`, so the run
gate's bounded re-dispatch could have run on a brief holding only the retry context — with the
original task silently stripped — after the caller's temp file was reaped during a long delegated
run. Each half now carries its own guard.

**A failed spool left an unreclaimable dispatch dir (minor).** The spool's `die` was the first that
could fire after `docket_dispatch_mint` created `$DDIR`, and the prune never removes a dispatch with
no terminal file. The failure path now reclaims its own directory, and the comment claiming "no new
lifecycle is introduced" was corrected rather than left to mislead.

**Budget parity, handled deliberately.** `tests/test_runner_dispatch.sh` entered this change at 9s
serial against a 10s ceiling — zero headroom, spent by change 0270 rather than breached by it. This
change added ~50 cases to that file; the row was re-measured and raised to **20s** (via 15s, then
corrected at review to match the table's own "next multiple of 5 plus a 5s margin" rule applied to
the worst standalone serial reading), with `EXPECTED_TOTAL` re-seeded to 1670. The final suite run
does **not** name `test_runner_dispatch` in its `OVER BUDGET:` line. Change 0208 is queued against
that same file and now has real margin to spend.

## Follow-ups

- **Nothing minted.** The reviewer noted `tests/test_docket_config.sh` breaching its ceiling at
  ~167s; change **#0280** already names that file explicitly, so this was a dedup skip rather than a
  new stub.
- **Out of scope by design, and left alone:** the `--observe` poll-loop `state=` prefix-strip defect
  class (change **#0284**, fixed for `gate-run` by change 0286), and `tests/test_sync_agents_runners.sh`'s
  pre-existing budget overrun (change **#0280**) — it is still the only file on the suite's
  `OVER BUDGET:` line.
- **The trailing-argv payload channel remains reachable** at the facade and the adapters, and is no
  longer lossy, but it is no longer taught by the shim. It exists for hand invocations and for the
  adapter contracts' documented direct-invocation path; if a later change wants to retire it, the
  refusal and join tests are the places that pin its behavior.
