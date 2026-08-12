#!/usr/bin/env bash
# tests/test_runtime_budgets.sh — regrowth guard (change 0227): every tests/test_*.sh carries a
# wall-clock budget row, no row exceeds the 60s ceiling, the serial pin-list is budgeted with its
# own counter, the ceilings sum to a pinned total, and the merge gate still reports breaches.
#
# WHY SEPARATE COUNTERS. A count-based guard whose remedy is "make the number agree" teaches the
# evasion it exists to catch (repo learning: guard-remedy-must-not-teach-the-evasion). Four paths
# grant a file relief from the discipline, and each is asserted on its own, so that taking any one
# of them leaves the completeness assertion green and reddens only its own counter:
#
#   A  a ceiling ABOVE the hard 60s limit                       — assertion (3)
#   B  a `serial` pin, removing the file from the parallel phase — assertion (4)
#   C  a ceiling raised anywhere BELOW the limit — the quiet one: a row moved 35 -> 60 breaks no
#      ceiling and pins nothing serial, yet buys 62s MORE measured slack under the 5/2 factor.
#      The table's TOTAL is pinned instead, so any raise moves it                — assertion (5)
#   D  the wiring site: disarming the budget report at the merge gate, which leaves every
#      assertion about the table green and the table itself decoration            — assertion (7)
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

TBL="$REPO/tests/runtime-budgets.tsv"
CEILING=60          # the hard ceiling; no row may exceed it
EXPECTED_SERIAL=0   # files pinned serial by the change-0227 audit. RAISING THIS IS A FINDING:
                    # a serial pin removes a file from the parallel phase, so it must be justified
                    # in the same diff with the shared state that forced it.
EXPECTED_TOTAL=1795 # the sum of every ceiling, seeded with the table from the measured serial run.
                    # 1785 -> 1795 (change 0298): tests/test_board_checks_stack.sh, a NEW test file
                    # — the same legitimate mover. It is a SIBLING SHARD of
                    # tests/test_board_checks.sh, which sits at 55s, and it carries the two stacked
                    # health checks plus the pin that keeps `stack-invalid` and
                    # `stack-parent-killed` separate ids with separate remedies. Readings
                    # 1.93/1.92/1.92s standalone serial: the floor, 10. Its rationale is in the tsv
                    # header beside the row.
                    # Recomputed from the table itself, never hand-adjusted:
                    #   awk -F'\t' '!/^#/ && NF>=2 {s+=$2} END{print s}' tests/runtime-budgets.tsv
                    # 1760 -> 1770 (change 0298): a NEW test file bringing its own row — the first
                    # of the two legitimate movers the table header names.
                    # tests/test_docket_status_stack.sh is a SIBLING SHARD of
                    # tests/test_docket_status.sh carrying the sweep's stacked legs: a child whose PR
                    # merged into its stack parent's branch flips to `stacked-merged` in place, while
                    # a stacked child merged into the integration branch still closes out to `done`.
                    # It is a sibling rather than an extension because the tsv header states
                    # tests/test_docket_status.sh, at the 60s ceiling, has no next raise. Readings
                    # 1.19/1.27/1.18s standalone serial, so the sizing rule puts it at the floor, 10.
                    # Recomputed from the table itself, never hand-adjusted:
                    #   awk -F'\t' '!/^#/ && NF>=2 {s+=$2} END{print s}' tests/runtime-budgets.tsv
                    # 1750 -> 1760 (change 0298): tests/test_docket_stack.sh, a NEW test file for the
                    # stacked-changes library and its stack-base.sh CLI — the same legitimate mover.
                    # Readings 0.23/0.22/0.22s standalone serial: the floor, 10. Its rationale is in
                    # the tsv header beside the row; this ledger line was omitted when the row landed
                    # and is written back here so the chain below reads continuously.
                    # Recomputed from the table itself, never hand-adjusted:
                    #   awk -F'\t' '!/^#/ && NF>=2 {s+=$2} END{print s}' tests/runtime-budgets.tsv
                    # 1740 -> 1750 (change 0221, review finding 4): tests/test_run_tests.sh 20 -> 30
                    # — a file that GOT SLOWER, which the header does not list as a legitimate mover,
                    # so it is defended by measurement and by having first removed the part of the
                    # cost that was avoidable. 0221 gates run-tests.sh on the source-hygiene
                    # preflight, and this file invokes the runner ~35 times. Rule (a)'s tests-tree
                    # sweep forked one awk PER TESTS-TREE FILE per invocation, so the file paid
                    # O(#test files x #invocations) and that term grew with the suite; batching the
                    # sweep into one awk process (verdict-preserving — byte-identical checker output
                    # over the live tree and every fixture) took it from 27.24 back to 21.03, against
                    # a 16.16 base. Only then was the row re-seeded, from 21.03: next multiple of 5
                    # is 25, plus the 5s margin -> 30. The readings and the method are in the tsv
                    # header beside the row's own note.
                    # Recomputed from the table itself, never hand-adjusted:
                    #   awk -F'\t' '!/^#/ && NF>=2 {s+=$2} END{print s}' tests/runtime-budgets.tsv
                    # 1730 -> 1740 (change 0221): a NEW test file bringing its own row — the first
                    # of the two legitimate cases the table header names, not a file that got
                    # slower. tests/test_assert_hygiene.sh is the regression test for
                    # scripts/check-test-source-hygiene.sh, driving 18 committed fixtures in both
                    # directions. It measures 1.26/1.25/1.26s standalone (three readings,
                    # /usr/bin/time -p, quiet machine), so the sizing rule — round up to the next
                    # multiple of 5, add a 5s margin, minimum 10 — puts it at the floor, 10. Its
                    # cost is two checker invocations over the fixture set plus three single-file
                    # ones, and does not grow with the suite.
                    # Recomputed from the table itself, never hand-adjusted:
                    #   awk -F'\t' '!/^#/ && NF>=2 {s+=$2} END{print s}' tests/runtime-budgets.tsv
                    # 1720 -> 1730: change 0118's review finding 2 raised tests/test_docket_status.sh
                    # 50 -> 60 (its rationale, and the readings behind it, are in the tsv header).
                    # Recomputed from the table itself, never hand-adjusted:
                    #   awk -F'\t' '!/^#/ && NF>=2 {s+=$2} END{print s}' tests/runtime-budgets.tsv
                    # 1715 -> 1720 (change 0118): tests/test_docket_status.sh 45 -> 50. Not "the
                    # file got slower" — the file GREW: 0118 adds two sweep runs and three
                    # fault-injection runs to the one file that owns sweep_execute coverage, and
                    # the row was already at parity before it (45.15s standalone serial against 45).
                    # Re-seeded by COMPUTING the sum, never by hand-adding:
                    #   awk -F'\t' '!/^#/ && NF>=2 {s+=$2} END{print s}' tests/runtime-budgets.tsv
                    # The new row is set from the WORST of three post-change standalone serial
                    # readings (44.55s of 44.55/43.76/44.22) per the table header's rule; the
                    # sharding-vs-raise argument lives with the row, in that header.
                    # 1705 -> 1715 (change 0247): a NEW test file bringing its own row — the first
                    # of the two cases the table header names as a legitimate move of the total.
                    # tests/test_shared_worktree_commit_scope.sh is the shared-metadata-worktree
                    # commit-scope guard; it measures 1.50/1.43/1.43s standalone, so the sizing rule
                    # (round up to the next multiple of 5, add a 5s margin, minimum 10) puts it at
                    # the floor, 10. It is a static scan of scripts/**/*.sh with no git fixtures, so
                    # its cost is one awk per file and does not grow with the suite.
                    # 1695 -> 1705 (change 0247): a row that was never sized on the WHOLE file —
                    # the same case as change 0277's entry below, NOT a file that got slower.
                    # tests/test_docket_status.sh 35 -> 45. Under the sizing rule (round up to the
                    # next multiple of 5, add a 5s margin) a 35s row asserts a file costing 30s or
                    # less; at the merge-base commit this file already measured 34.18/34.12/34.30s
                    # SERIALLY, so the row was a size behind before this change touched it. On the
                    # branch — which adds Half 2's two shared-worktree fixtures, 488 asserts to 508
                    # — it measures 36.59/36.78/35.96s across three standalone serial runs on a
                    # quiet machine. The sizing input is the WORST of those readings, 36.78s, which
                    # the rule puts at 45. The added block's own cost is ~1.5s, measured directly
                    # with epoch stamps around its four fixture runs: the rest of the gap was
                    # already there.
                    # This is not a number raised to absorb a breach. The file has never been
                    # reported OVER BUDGET — the runner's breach test is `measured > ceiling * 5/2`,
                    # and the full parallel suite for this change (106 files, 8665 asserts, wall
                    # 192s) reads it at 66s against a 87.5s threshold. What is raised is the CLAIM,
                    # which was already false by ~0.8s at the merge-base and is false by ~1.8s now.
                    # 1680 -> 1690 (change 0284): the new-test-file case —
                    # tests/test_docket_liveness.sh, a hermetic unit test of the shared liveness
                    # predicate. Measured 0.06/0.05/0.05s standalone; the sizing rule's 10s floor.
                    # 1690 -> 1695 (change 0284): the re-shaping case —
                    # tests/test_runner_dispatch_observe.sh 25 -> 30. The liveness leg's arms spawn
                    # real process groups, kill them and wait on their reaping, which is wall-clock
                    # the file did not previously carry. Measured 22.26/22.27/22.25s standalone;
                    # the sizing rule (round up to a multiple of 5, add 5) gives 30, leaving 7.73s
                    # of margin under the row and 30s under the hard ceiling.
                    # 1670 -> 1680 (change 0281): the new-test-file case —
                    # tests/test_critic_return_channel.sh, a prose-grep sentinel at the 10s floor.
                    # 1660 -> 1670 (change 0277): a row that was never sized on the whole file —
                    # the 0242 case recorded further below, NOT a file that got slower.
                    # tests/test_runner_dispatch.sh 10 -> 20. Under the sizing rule (next multiple
                    # of 5 plus a 5s margin, min 10s) a 10s row asserts a file costing 5s or less;
                    # this one measures 7/8/7s SERIALLY at the merge-base ea9dc0bf, so the row was
                    # already a size behind before this change touched the file. On the branch —
                    # which appends the brief-file cases to it, 136 asserts to 174 — it measures
                    # 8/8/9/9/9/12s across six standalone serial runs, three of them interleaved
                    # with the merge-base runs above on the same machine. The sizing input is the
                    # WORST of those standalone serial readings, 12s — none of them is discounted,
                    # since they are all quiet-machine standalone runs — which the rule puts at 20.
                    # (Sized at 15 first, off the 9s cluster; corrected here on review, which is
                    # also what leaves change 0208 the headroom named below.)
                    # This is not a number raised to absorb a breach: the file has
                    # never been reported OVER BUDGET, and a row is a SERIAL claim, so the 13s it
                    # shows in this change's full parallel run (104 files, wall 223s, nine
                    # unrelated files breaching — a loaded machine) is contention that the runner's
                    # own 5/2 factor absorbs. What is raised is the CLAIM, which at 10 sat one
                    # second from parity with a 9s file and so left change 0208 — queued to add
                    # further cases to this same file — no headroom to spend
                    # (LEARNINGS `budget-headroom-is-spent-before-it-is-breached`).
                    # 1660 STANDS (change 0282 review, finding 9): the two gate-run rows below were
                    # cut from a `-j 1` reading while the budget is enforced during the PARALLEL
                    # phase, and the safety argument they cited ("two full-suite runs, never OVER
                    # BUDGET") predated the tightening, so it bounded nothing about the new numbers.
                    # The evidence was therefore re-taken where the check actually runs: one full
                    # parallel suite (scripts/run-tests.sh --timings, 104 files, 7961 asserts,
                    # wall 201s) measures tests/test_gate_run.sh at 18s against its 20s row and
                    # tests/test_gate_run_stop.sh at 34s against its 35s row — the launch shard
                    # having grown the four-spelling unusable-pgid loop and the mid-flight-abort
                    # fixture this same review added. That run was a LOADED one, not a quiet
                    # machine: three unrelated files breached, tests/test_sync_agents_runners at
                    # 201s against a 60s ceiling — so these two readings are a pessimistic sample.
                    # The rows are KEPT rather than re-sized upward, for the reason the
                    # harness-gaps entry below already states: a row is a SERIAL claim, and
                    # parallel-phase contention is what the runner's own comparison factor
                    # (SLACK_NUM/SLACK_DEN = 5/2) absorbs. Under that factor these rows report OVER
                    # BUDGET only past 50s and 87s, so the hazard the entry below names — "a ceiling
                    # set just above a load-sensitive reading manufactures intermittent findings" —
                    # is not the state either of them is in, and the total does not move.
                    # 1655 -> 1660 (change 0282 review, launch-failure identity blocker): the
                    # RE-CUT case on a single row — tests/test_gate_run.sh 15 -> 20. The review fix
                    # gates `--launch`'s failure-path signal on identity, and the only witness that
                    # can tell a proven kill from an unprovable one is a live BYSTANDER GROUP that
                    # must still be there afterwards — so the fixture is a real detached launch, a
                    # real foreign group, a one-second `ps -o lstart=` separation (that clock has
                    # whole-second resolution, so the separation IS the mismatch), and the failure
                    # path's own fixed 2s leak probe running to its end because the bystander
                    # survives. None of that can be mocked without mocking away the property. The
                    # file measures 14s standalone, which the sizing rule — next multiple of 5 plus
                    # a 5s margin — puts at 20. Not moved into the other shard instead: the --stop
                    # shard measures 30s against 35 and this is a --launch fixture.
                    # 1605 -> 1655 (change 0282): the new-test-file case, twice —
                    # tests/test_gate_run.sh and tests/test_gate_run_stop.sh each bring their own
                    # row. They are the second family in the table (after the runner-dispatch detach
                    # shards) whose cost is WALL CLOCK THAT CANNOT BE MOCKED: scripts/gate-run.sh
                    # launches real detached children and the shards wait on real terminal records
                    # and real rendezvous barriers, because "the launcher returned before the child
                    # finished", "a signal death is not a red suite" and "a stop held across
                    # completion keeps the verdict" are only observable in wall-clock time. Split
                    # along the verb seam — launch/observe in the first, --stop and its five
                    # interleaving fixtures in the second — because one file carrying both would
                    # exceed the hard 60s ceiling and blur two review surfaces. The tasks that
                    # shipped them seeded 40 and 60 deliberately generously, ahead of this change's
                    # own full-suite run, on the theory that a barrier-driven file's parallel-phase
                    # cost is what a standalone run understates. Two full-suite runs later neither
                    # file has ever been reported OVER BUDGET, so the placeholders are retired here
                    # for the sizing rule's own number: measured serially at 9s and 30s
                    # (scripts/run-tests.sh -j 1 --timings), which the rule — next multiple of 5
                    # plus a 5s margin, min 10s — puts at 15 and 35, and the sum FALLS by 50. The
                    # margin is real headroom rather than a shave: the runner breaches at 2.5x, so
                    # 35 tolerates 87s against a 30s file whose largest single component is a fixed
                    # 10s TERM grace window that load cannot inflate at all. A ceiling set just
                    # above a load-sensitive reading manufactures intermittent findings, which is
                    # strictly worse than a loose one — hence the rule's margin, not a tighter cut.
                    # 1585 -> 1595 (change 0276): the new-test-file case named below —
                    # tests/test_dummy_mode.sh brings its own row. It is a pure prose scan (two
                    # file reads, an awk heading slice, and a handful of greps over collapsed
                    # haystacks), measured standalone at 1.3/1.3/1.4s across three consecutive
                    # runs, so the sizing rule — next multiple of 5 plus a 5s margin, min 10s —
                    # floors it at the 10s minimum.
                    # 1595 -> 1605 (change 0276 integration repair): tests/test_pipe_shapes.sh
                    # brings its own row — one grep -E pass over every tracked shell file plus
                    # eleven tmpdir fixtures, measured standalone at 0.13s, floored at the 10s
                    # minimum by the same sizing rule.
                    # 1595 -> 1585 (change 0271 review, finding 13): the SHARD-RE-CUT case, and the
                    # first re-cut that LOWERS the total. tests/test_runner_dispatch_detach.sh sat
                    # on the hard 60s ceiling with zero headroom — the row below explains why it was
                    # seeded there — while measuring ~28s, and its cost is FIXED SLEEP, so every arm
                    # the review added moved it toward a breach with no number left to raise. The
                    # table's own remedy applies: shard, never a bigger number. Cut along the file's
                    # three natural seams into tests/test_runner_dispatch_detach.sh (launch),
                    # tests/test_runner_dispatch_observe.sh (observation and budget) and
                    # tests/test_runner_dispatch_build_gate.sh (build verdict + implement-next run
                    # gate + posture docs), with the shared prologue in
                    # tests/lib/runner_dispatch_detach_common.sh. A MOVE, not a rewrite: the 184
                    # asserts split 36/88/60 with none lost. Measured standalone across three
                    # consecutive runs — 6/6/6s, 19/19/19s and 5/4/4s — which the sizing rule (next
                    # multiple of 5 plus a 5s margin, min 10s) puts at 15, 25 and 10. The sum FALLS
                    # by 10 because the 60 was never a measurement of the file: it was the ceiling
                    # itself, taken as a budget.
                    # 1510 -> 1535 (change 0242 review, finding 7): the SHARD-RE-CUT case, not a
                    # raise. tests/test_sync_agents_codex.sh carried one 55s row over two
                    # independent surfaces — the per-repo .codex/agents/*.toml wrappers and the
                    # committed AGENTS.md dispatch block — and this change added a second `--check`
                    # leg to the dispatch half. Measured SERIALLY at 56/57s (branch) against 53/53s
                    # (merge-base 487bfdc5, same machine, interleaved), i.e. already past its own
                    # row and inside the hard 60s ceiling's noise band, so the table's remedy
                    # applies: shard, never a bigger number. Cut at the "AGENTS.md dispatch block"
                    # banner into tests/test_sync_agents_codex.sh (wrappers) and
                    # tests/test_sync_agents_codex_dispatch.sh (dispatch block); the 74 asserts
                    # split 44/30 with none lost. Re-measured standalone across three consecutive
                    # serial runs: 21/19/22s and 41/38/41s. The sizing rule (next multiple of 5
                    # plus a 5s margin, min 10s) puts those at 30 and 50. The sum grows by 25
                    # because the +5 margin is now paid twice AND because the single 55s row had
                    # never been sized on the whole file.
                    # +20 (change 0242 review): the new-test-file case again —
                    # tests/test_sync_agents_surface_containment.sh brings its own row. The
                    # containment guard (docket writes and strips its dispatch block only inside
                    # the checkout it was run in) could not go into
                    # tests/test_sync_agents_claude_surface.sh: that file already measures ~40s
                    # against its 45s row, and the combined file measured 56.8s — past the 60s hard
                    # ceiling, so the table's own remedy is a sibling shard, not a bigger number.
                    # The new file runs sync-agents.sh five times (three generations and two
                    # --check re-reads) plus two source-and-call probes of resolve_physical_path,
                    # and measures 11.8/14.1/11.2s across three consecutive standalone runs; the
                    # sizing rule (next multiple of 5 plus a 5s margin, min 10s) puts that at 20.
                    # +60 (change 0271): the new-test-file case —
                    # tests/test_runner_dispatch_detach.sh brings its own row. It is the one file in
                    # the table whose cost is DELIBERATE SLEEP rather than work: it launches
                    # detached children that sleep for a caller-controlled duration and then waits
                    # for their sentinels, because "the launcher returned before the child finished"
                    # and "the child outlived the call" are only observable in wall-clock time. Its
                    # Task-3 half measures 5/5/6s across three consecutive standalone runs, but the
                    # sizing rule is applied to the file this change ships, not to a half of it:
                    # tasks 4 and 5 append the observation arms to this same file, including a
                    # 6s-child observe loop, a 120s-child budget-exhaustion arm cut short by the
                    # kill, and a post-kill settle. The row is the hard 60s ceiling because that is
                    # what the whole file costs; a file that later outgrows it gets the table's
                    # standing remedy — shard it, never a bigger number.
                    # 1475 -> 1490 (change 0242): still the SAME new-test-file case as the line
                    # below, re-seeded on the finished file rather than on a mid-change snapshot.
                    # tests/test_sync_agents_claude_surface.sh was sized at 30 when it covered the
                    # surface WRITE only; the change's next task appended its --check half — four
                    # more sandboxes and six more sync-agents.sh invocations — and the completed
                    # file measures 39.7/40.0/39.8s across three consecutive standalone runs. The
                    # table's own sizing rule (next multiple of 5 plus a 5s margin, min 10s) puts
                    # that at 45. Not a file that got slower: a row that was never sized on the
                    # whole file. This is the last task that adds to it.
                    # +30 (change 0242): the new test file tests/test_sync_agents_claude_surface.sh
                    # brought its own row, measured standalone at 25s.
                    # 1435 -> 1445 (change 0242): the new-test-file case named below —
                    # tests/test_sync_agents_run_gate.sh brings its own row. It invokes
                    # sync-agents.sh ONCE (a single [cursor, codex] generation); everything else it
                    # asserts is file reads. Measured standalone at 2-3s across three consecutive
                    # runs, so the table's own sizing rule — next multiple of 5 plus a 5s margin,
                    # min 10s — floors it at the 10s minimum.
                    # 1415 -> 1435 (change 0245): the new-test-file case named below —
                    # tests/test_sync_agents_harness_gaps.sh brings its own row; it invokes
                    # sync-agents.sh EIGHT times — the RK and RC generations, their two --check
                    # re-runs, and one generation per run_cell cell (global-noopt / global-opted
                    # each driving a full user-level pass over [claude, cursor]). Measured
                    # SERIALLY at 14s (scripts/run-tests.sh -j 1 --timings, three consecutive
                    # runs, all 14s) and sized to 20s. A row is a serial claim: the ~33s this
                    # file shows in a parallel full-suite run is runner contention, which the
                    # runner's own comparison factor already absorbs, not the file's cost.
                    # 1405 -> 1415 (change 0244): the new-test-file case named below —
                    # tests/test_frontmatter_read_shapes.sh brings its own row, floored to the
                    # table's 10s minimum.
                    # 1365 -> 1405 (change 0255): the new-test-file case named below —
                    # tests/test_harness_defaults_flow_map.sh brings its own row, measured
                    # standalone at 9.8s and sized to 40s to cover the `#`-leg probes task 2
                    # appends to the same file.
                    # 1355 -> 1365 (change 0254): the new-test-file case named below —
                    # tests/test_bsd_tool_defaults.sh brings its own row, floored to the table's
                    # 10s minimum.
                    # 1345 -> 1355 (change 0237): the new-test-file case named below —
                    # tests/test_verify_run.sh brings its own row, measured serially at 1.4s,
                    # floored to the table's 10s minimum.
                    # 1325 -> 1335 (change 0223): the new-test-file case named below —
                    # tests/test_gate_execution_posture.sh brings its own row, measured serially at
                    # 1s (scripts/run-tests.sh -j 1), floored to the table's 10s minimum.
                    # 1335 -> 1345 (change 0190): the same case again —
                    # tests/test_skip_allowlist_invisibility.sh brings its own row, measured at
                    # 0.4s standalone, floored to the table's 10s minimum.
                    # It is the whole suite's budget, and it moves only for a reason that is stated
                    # in the diff that moves it: a new test file brings its own row, and re-cutting
                    # a sharded file's rows redistributes cost that has to be re-measured.

assert "budget table exists" '[ -f "$TBL" ]'

rows="$(grep -vE '^[[:space:]]*(#|$)' "$TBL")"

# (1) well-formedness — three tab fields, integer seconds, known mode
bad=0
while IFS=$'\t' read -r p secs mode; do
  [ -n "$p" ] || continue
  case "$secs" in ''|*[!0-9]*) echo "  malformed seconds: $p [$secs]" >&2; bad=1 ;; esac
  case "$mode" in parallel|serial) ;; *) echo "  malformed mode: $p [$mode]" >&2; bad=1 ;; esac
  [ "$(awk -F'\t' -v k="$p" '$1==k{n++} END{print n+0}' <<<"$rows")" = 1 ] || { echo "  duplicate row: $p" >&2; bad=1; }
done <<<"$rows"
assert "every row is <path><TAB><integer seconds><TAB><parallel|serial>, no duplicates" '[ "$bad" = 0 ]'

# (2) completeness, BOTH directions — a new test file with no row fails here
listed="$(awk -F'\t' '{print $1}' <<<"$rows" | LC_ALL=C sort)"
actual="$(cd "$REPO" && find tests -maxdepth 1 -name 'test_*.sh' | LC_ALL=C sort)"
assert "every tests/test_*.sh has a budget row, and every row has a live file" \
  '[ "$listed" = "$actual" ] || { diff <(echo "$listed") <(echo "$actual") >&2; false; }'

# (3) RELIEF COUNTER A — rows above the hard ceiling. Independent of (2): laundering a single
# file's ceiling upward leaves (2) green and reddens only this.
over="$(awk -F'\t' -v c="$CEILING" '$2 > c {print $1}' <<<"$rows")"
assert "no budget row exceeds the ${CEILING}s ceiling" \
  '[ -z "$over" ] || { echo "  over ceiling: $over" >&2; echo "  Shard the file or move its new assertions into a shard with room. Raising the ceiling is not the remedy." >&2; false; }'

# (4) RELIEF COUNTER B — files pinned serial, budgeted exactly. Also independent of (2).
serial_n="$(awk -F'\t' '$3 == "serial" {n++} END{print n+0}' <<<"$rows")"
assert "exactly $EXPECTED_SERIAL files are pinned serial" \
  '[ "$serial_n" = "$EXPECTED_SERIAL" ] || { echo "  serial rows: $(awk -F"\t" "\$3==\"serial\"{print \$1}" <<<"$rows" | tr "\n" " ")" >&2; echo "  A serial pin removes a file from the parallel phase. Name the shared state that forces it, in this diff." >&2; false; }'

# (5) RELIEF COUNTER C — the table's TOTAL. Counters A and B only see the two loud reliefs; a row
# raised from 35 to 60 trips neither, and completeness never looks at column 2 at all. The sum sees
# every raise, however small, and reddens on it alone.
total="$(awk -F'\t' '{s += $2} END {print s+0}' <<<"$rows")"
assert "the table's budgeted total is $EXPECTED_TOTAL seconds" \
  '[ "$total" = "$EXPECTED_TOTAL" ] || { echo "  budgeted total: ${total}s across $(wc -l <<<"$rows" | tr -d " ") rows, pinned at ${EXPECTED_TOTAL}s" >&2; echo "  A ceiling describes what a file already costs, so it moves when the file is re-shaped, not when it gets slower: shard the file, or move its new assertions into a shard with room." >&2; echo "  The total legitimately moves in two cases, and both belong in this diff: a new test file brings its own row, and re-cutting a sharded file redistributes its cost. Re-seed from a measured serial run (scripts/run-tests.sh -j 1 --timings PATH) and say which case it was." >&2; false; }'

# (6) the runner actually READS this table by default — otherwise the whole table is decoration
assert "run-tests.sh defaults to tests/runtime-budgets.tsv" \
  'grep -q "tests/runtime-budgets.tsv" "$REPO/scripts/run-tests.sh"'

# (7) RELIEF PATH D — THE WIRING SITE. (6) proves the runner reads the table when it runs; it says
# nothing about the command the merge gate actually runs. Since a breach became advisory, that gate
# no longer turns red on one — the `OVER BUDGET:` REPORT is the whole remaining channel (see
# scripts/run-tests.md, "Why a breach is advisory by default"). So what has to hold is that the
# configured command still emits that report: `--no-budget-check` measures no breach and prints
# none, and would leave every assertion above green over a table nothing reads.
#
# Resolution mirrors scripts/docket-config.sh's precedence — repo-local `.docket.local.yml` over
# committed `.docket.yml`. The machine-global layer is deliberately NOT consulted: a test that
# reads the developer's $HOME is the shape the change-0227 parallel-safety audit forbids.
test_command_of(){   # block-scoped awk, the tests/test_finalize_gate.sh idiom
  awk '
    /^finalize:[[:space:]]*$/{f=1;next}
    f && /^[^[:space:]#]/{f=0}
    f && /^[[:space:]]+test_command[[:space:]]*:/{
      line=$0; sub(/#.*/,"",line); sub(/.*test_command[[:space:]]*:[[:space:]]*/,"",line);
      sub(/[[:space:]]+$/,"",line); print line; exit
    }' "$1" 2>/dev/null
}
TESTCMD="$(test_command_of "$REPO/.docket.local.yml")"
[ -n "$TESTCMD" ] || TESTCMD="$(test_command_of "$REPO/.docket.yml")"

assert "the resolved finalize.test_command runs the budget-reporting runner" \
  'case "$TESTCMD" in *run-tests.sh*) true ;; *) echo "  resolved finalize.test_command: ${TESTCMD:-<unset>}" >&2; echo "  The merge gate is the one run whose output is always read, so it is where a budget breach gets seen. A command that does not run scripts/run-tests.sh reports no budgets at all; point the gate back at the runner, or record in this diff what carries the report instead." >&2; false ;; esac'

assert "the resolved finalize.test_command does not suppress the budget report" \
  'case "$TESTCMD" in *--no-budget-check*) echo "  resolved finalize.test_command: $TESTCMD" >&2; echo "  --no-budget-check measures no breach and prints none, which retires this whole table at the one run everybody reads. If a file is breaching, shard it or move its new assertions into a shard with room; --no-budget-check belongs in measurement runs, where enforcing a ceiling against the number being measured is circular." >&2; false ;; *) true ;; esac'

# (8) tests/README.md exists and tells the reader where new tests go
assert "tests/README.md exists"          '[ -f "$REPO/tests/README.md" ]'
assert "tests/README.md says where new tests go" \
  'grep -qi "where new tests go" "$REPO/tests/README.md"'

exit $fail
