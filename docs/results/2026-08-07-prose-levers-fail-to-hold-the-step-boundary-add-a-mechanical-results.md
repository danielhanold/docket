<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0237 — Prose levers fail to hold the step boundary — give the disposition contract a consumer](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0237-prose-levers-fail-to-hold-the-step-boundary-add-a-mechanical.md)**
<!-- docket:backlink:end -->

# Prose levers fail to hold the step boundary — give the disposition contract a consumer — results

Change: #0237 · Branch: feat/prose-levers-fail-to-hold-the-step-boundary-add-a-mechanical · PR: <url> · Plan: docs/superpowers/plans/2026-08-07-prose-levers-fail-to-hold-the-step-boundary-add-a-mechanical.md · ADRs: 69, 75

## Verify (human)

- [ ] **Signal semantics after removing `exec`.** The suite cannot reach this. Removing `exec` from
      `runner-dispatch.sh` makes the adapter a child rather than a replacement image, which changes
      what a signal to the facade does. Measured at build time, both deliveries, pre- and
      post-change:

      | build | delivery | facade exit | adapter trap | note |
      |---|---|---|---|---|
      | pre-0237 `exec` | group INT | 130 | yes | |
      | post-0237 | group INT | 130 | yes | **unchanged — the interactive Ctrl-C path** |
      | pre-0237 `exec` | pid INT | 130 | yes | deferred behind `sleep` |
      | post-0237 | pid INT | **0** | no | was 130 |
      | post-0237 | group TERM | 143 | yes | |
      | post-0237 | pid TERM | 143 | no | **adapter orphaned, still running** |

      Group-directed delivery — what a terminal Ctrl-C actually does — is unchanged, which is why
      no forwarding trap was added. Pid-directed delivery changed: the facade now absorbs the
      signal instead of *being* the adapter. Nothing in docket signals the facade by pid today, but
      a `timeout`-style supervisor would. Confirm you accept that trade, or ask for forwarding
      traps as a follow-up.

- [ ] **The run gate is unreachable from a Claude run, by design.** Claude Code dispatches
      subagents itself, so `runner-dispatch.sh` is not on that path — this gate covers `codex`,
      `cursor`, `opencode`, and every future adapter. All six observed incidents were Claude runs
      and would each still have gone uncaught at the moment they happened. The spec accepts this as
      the honest cost of the cross-harness constraint; change 0242 is the Claude half. Worth a
      conscious nod at the merge gate rather than a surprise later.

## Findings

- **A plan-supplied assert was vacuous, and the plan was mine.** The "facade no longer execs" guard
  the plan handed the worker could never fail: `\$` inside an `eval`'d double-quoted string
  collapses to a bare `$`, which grep then read as an end-anchor. It passed against the *unchanged*
  file. Replaced with a shape-anchored static guard plus a runtime `$PPID`-vs-`$$` check, both
  red-before/green-after. This is the `plan-supplied-test-code-is-unverified` learning landing on
  docket's own plan — worth a war-story line on that finding at harvest.

- **The reviewer's stacked-gap hang did not reproduce on this machine.** The deep review measured a
  two-bounded-gap ERE running >24s on non-matching input; the fixing worker measured the same
  pattern at 0.051–0.082s under PATH `grep` (ugrep 7.5.0) and 0.007s under `/usr/bin/grep`, across
  here-string, file, and unflattened variants. Both local engines appear DFA-based rather than
  backtracking. **The rewrite was kept regardless** — the stacked gap is a real latent hazard under
  a backtracking engine and the single-gap form is strictly safer — but the "hangs the suite today"
  characterization is unconfirmed here, and the discrepancy itself is worth understanding before
  anyone trusts a timing-based claim from either side.

- **A `claimed_at` window cannot close the concurrent-claim case**, contrary to the review's
  suggested fix. `docket-implement-next` re-stamps `claimed_at` at every phase boundary (the
  heartbeat rider), so a concurrently-running loop's claim carries a stamp inside our window and is
  indistinguishable by clock alone. Closed by **cardinality** instead: an implement-next run claims
  at most one change, so two or more surviving candidates means none is attributable and the gate
  stands down. Recorded in **ADR-0075** with its accepted residual (a `drained` run concurrent with
  exactly one other claim still misattributes).

- **A halt was silently discarded at the seam.** The gate read `run-halted` from git — the hard part
  — then returned the adapter's exit code, telling a driver to *continue* on the one disposition the
  contract defines as *stop + surface*. Now exit **3**, distinct from `1` (two-strikes abort) and
  from the verbatim adapter code on no-action paths. Also recorded in **ADR-0075**. The
  post-re-dispatch `run-halted` leg turned out to be entirely unhandled before the fix.

- **`## Run halted` was specified with a dated heading and read by a whole-line match** — so a
  producer following the instruction literally would have written a section `verify-run` could never
  see, making the whole `run-halted` verdict unreachable. The identical trap for `## Finalize
  blocked` is already documented in `docket-finalize-change/references/gate-failure.md`; this change
  now follows that wording (bare heading, date in the body) at all four producer surfaces.

- **A zero-padded id parsed as octal.** `verify-run 0237` silently reported on change 0159 and exited
  0, because docket displays padded ids everywhere while bash `printf %d` reads a leading `0` as
  octal. Canonicalized with `10#`, matching the existing precedent in `board-checks.sh` and
  `adr-checks.sh`.

## Follow-ups

- **Change 0261** (auto-captured this run) — give `## Run halted` a board surface and a health
  check. It is the only member of its family with no derived view: `## Finalize blocked` and
  `## Auto-groom blocked` both render needs-you cells. Deferred because 0237's spec explicitly rules
  board rendering out of scope and deliberately leaves `board-checks.sh` untouched.

- **Change 0242** (pre-existing, `depends_on: [237]`) — the Claude Code `Stop`/`SubagentStop` hook.
  Now a small wiring job onto an oracle that exists.

- **Two `producer | early-exiting-consumer` pipelines remain** in `tests/test_verify_run.sh`
  (`flat "$IMPL" | grep -q …`), the AGENTS.md SIGPIPE-under-`pipefail` shape. Pre-existing to the
  finding that touched that block; two others were removed in passing. Not yet observed flaking at
  141, so left alone rather than widened into an unrelated sweep.

- **Budget pins moved, all with in-diff justification:** `runtime-budgets.tsv` gained a
  `tests/test_verify_run.sh` row and `EXPECTED_TOTAL` went 1335 → 1345 (the "a new test file brings
  its own row" case); skill-size budgets rose for the two SKILL.md files this change edits.
  `docket-implement-next/SKILL.md` now sits at 4489/4500 words — 11 words of headroom, so the next
  edit to it will need a real decision about extraction rather than another raise.
