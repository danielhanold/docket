<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0228 — finalize's auto-detect suite loop has no failure accumulator, so a mid-suite red merges](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0228-finalize-s-auto-detect-suite-loop-has-no-failure-accumulator.md)**
<!-- docket:backlink:end -->

# finalize's auto-detect suite loop failure accumulator — results

Change: #0228 · Branch: feat/finalize-s-auto-detect-suite-loop-has-no-failure-accumulator · PR: <url> · Plan: docs/superpowers/plans/2026-08-07-finalize-auto-detect-suite-failure-accumulator.md · ADRs: none

## Verify (human)

- [ ] Nothing. The fragment is executed by `tests/test_configured_bash_finalize.sh` against real fixture repos, and every property this change claims was mutation-proven (below). There is no manual check an automated test cannot reach here.

## Findings

**The fix's own guard initially reproduced the defect class it fixes.** The defect was an *uninspected exit status*. The first cut of the guard asserted only the failure direction (`-ne 0`) and threw away the status of its one all-green run — so a regression making the block **always** non-zero would have left all 6123 asserts green while turning every green suite into a red gate at both consumers. Caught at review as the important finding and fixed in `ff2bd939`; the mutation (`suite_status=1` as the initializer) reddens the new success-direction assert and nothing else. This is the `fix-reintroduces-its-own-defect-class` learning firing exactly as written, and it is worth a harvest note.

**One assert shipped in the plan was green-by-construction.** The plan's Task 2 specified `assert "empty suite runs zero tests" '[ ! -s "$empty_execution_log" ]'` over a fixture directory created empty with the log truncated immediately before the run — nothing could ever write to it, so no reachable mutation could redden it. It read as coverage while carrying none. Deleted in `0764d125` and replaced by a mechanism pin (below). Worth recording because the plan itself predicted the assert would "stay `ok` under this mutation" and shipped it anyway — the `plan-supplied-test-code-is-unverified` learning: plan-supplied test code is unverified code, and "stays green under the mutation" was the tell.

**Outcome pinned, mechanism unpinned.** `empty_status -ne 0` was satisfied by *any* failure inside the subshell — a failed `cd`, a broken fragment, a missing `DOCKET_BASH_PATH` — not specifically by the literal glob reaching the runtime and failing. The fixture was already collecting the positive evidence (`empty_runtime_log`) purely for isolation and never asserting on it. Now asserted: `[ "$(cat "$empty_runtime_log")" = "tests/test_*.sh" ]`.

**A text guard was traded for a behavioral one.** The `! grep -qi -- "nullglob"` assert would have false-reddened against a maintainer who *correctly* hardened the fragment with `shopt -u nullglob`, and could only ever enumerate spellings. It was deleted rather than re-spelled: with the mechanism pinned behaviorally, any suppression of the literal glob — `shopt -s nullglob`, `[ -e "$test" ] || continue`, a `compgen`/array rewrite — empties the runtime log and reddens the assert regardless of how it is written. Deleting also sidestepped the ugrep-vs-`/usr/bin/grep` regex-portability trap, since no new regex was introduced. The reasoning is recorded in a block comment above the fixture so the next maintainer does not re-add the text grep.

**No ADR.** The spec's assumption 8 settled this in advance: the `exit "$suite_status"` form would have created a new cross-skill "execute this fragment as its own shell invocation" contract and been ADR-shaped, but it was reversed at grooming in favor of the trailing `[ … -eq 0 ]`, leaving a bug fix inside an existing contract. Nothing during the build created a new non-obvious decision. **If a future implementer reinstates the `exit` form, an ADR (or a deferral to change 0224) is required.**

## Follow-ups

**Plan deviations, both deliberate and both narrower than the plan text.**

1. The plan's fixture comment cross-referenced "line 77". `AGENTS.md` ("Comments and cross-references") forbids line-number anchors in maintained source, so the committed comment names the explicit-command case instead. Repository instructions outrank plan text.
2. The plan's Task 2 shipped two asserts that review then removed (the green-by-construction one and the text-shape `nullglob` grep), replacing them with one mechanism pin. Net assert count across the file is unchanged at 6123 for the suite.

**Not a follow-up, recorded so the next reader does not re-investigate:** `scripts/run-tests.sh` prints an `OVER BUDGET` advisory for ten pre-existing files (`test_sync_agents*`, board, harness). It is pre-existing, fires on every run in this repo, is explicitly advisory (the run still exits 0), and is unrelated to this change — this branch adds no new test file and roughly one second to `test_configured_bash_finalize`. Change 0229 already tracks the slack factor's hardware-dependent basis.

**Coupling to watch at merge.** Changes 0223 and 0190 are `in-progress` with live branches touching `skills/docket-build/SKILL.md` and `skills/docket-finalize-change/SKILL.md` respectively, both *outside* this marker block — rebase compositions, not conflicts. Change 0224 (`proposed`) states the green/red-is-the-exit-code rule normatively; 0228 is the code half and must not land a contradictory edit to that contract.
