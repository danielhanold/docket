<!-- results-template.md — close-out artifact for a change. -->
# Extend the board-row-dropped invariant to archive files — results
Change: #0115 · Branch: feat/extend-the-board-row-dropped-invariant-to-archive-files · PR: (set on open) · Plan: .superpowers/sdd/2026-07-27-archive-side-row-dropped-invariant/plan.md · ADRs: none

## Verify (human)

- [ ] Read the rewritten `board-row-dropped` section of `scripts/board-checks.md` — confirm the
  two-sided predicate description matches your mental model of `render-board.sh`'s active/archive
  iteration, and that the `ARCHIVE_RECENT` collapse note reads as intentional scope, not a gap.
- [ ] Skim the mutation matrix below — it's the evidence that each predicate clause (id gate,
  directory dispatch, population-site guard, both suppression sites) is independently load-bearing.

## Findings

- **Mutation matrix (Task 3), observed results.** Each mutation applied alone to
  `scripts/board-checks.sh`, run against the full suite, then reverted; `tests/test_board_checks.sh`
  itself was never modified.

  | # | Mutation | Predicted reddening | Observed reddening | Verdict |
  |---|---|---|---|---|
  | M1 | `renders_row`'s `archive` arm returns 0 unconditionally | T1 (80), T2 (82); T5/case-(d) (0073) stays green | Exactly T1 (80) + T2 (82) reddened; 0073 stayed green | MATCH |
  | M2 | Delete the hoisted `[ -n "$rr_id" ] \|\| return 1` line | T5 (0073) and active no-id case (a) (0070); T1/T2 stay green | Exactly 0070 + 0073 reddened; 80/82 stayed green | MATCH |
  | M3 | Restore the active-only population guard | T1, T2, T5 together (80, 82, 0073); active cases (a)/(f)/(g) stay green | All three archive cases reddened together; all active cases stayed green | MATCH |
  | M4 | Delete `EXPLAINED["$cid"]=1` in the `malformed-id` block | T6 (0085) gains a second finding | Both 0085 (archive) **and** 0072 (active) suppression asserts reddened | MATCH once corrected — see below |
  | M5 | Delete `EXPLAINED["$cid"]=1` in the `field-domain` status arm | T7 (86) plus the pre-existing active-side `n71` pair | Exactly the predicted 4 asserts reddened; sibling "finding is field-domain, not board-row-dropped" asserts stayed green on both sides | MATCH |

  **M1's T5-stays-green cell is deliberate evidence, not a formality**: 0073 (a no-`id:` archive
  file) is killed by the hoisted id-gate clause *before* the `case "$rr_dir"` directory switch ever
  runs, so forcing the `archive` arm to `return 0` cannot rescue it. That is exactly how the id gate
  and the directory dispatch are shown to be independent clauses rather than one fused condition.

  **M4 correction — not a code defect, an omission in the plan's table.** The plan predicted M4 would
  redden only the archive-side malformed-id suppression assert (T6, 0085). The observed run also
  reddened the pre-existing active-side assert (0072, case (c)). This is not a mismatch to explain
  away: `EXPLAINED["$cid"]=1` in the `malformed-id` block is a **single shared site** that runs for
  any file with a non-integer raw id, regardless of directory — so both suppression asserts that
  depend on it necessarily redden together. It is the identical collateral shape the plan *did*
  correctly predict for M5 (where the active-side `n71` pair was called out alongside T7). The
  plan's M4 row simply omitted the symmetric collateral it should have listed. Recording it here
  makes the mutation evidence stronger than the plan predicted, not weaker: it confirms
  `malformed-id` suppression is genuinely directory-agnostic by construction, not by two separate
  (and possibly divergent) code paths.

- **Reconcile delta: T9 dropped, not deferred.** By the time this change was built, change 0116 had
  already landed. The spec's "mirror by convention, not by construction" caveat about
  `DOCKET_STATUSES_ACTIVE`/`DOCKET_STATUSES_TERMINAL` staying in sync with `render-board.sh`'s own
  iteration was written against a world where that correspondence had to be asserted by a test (T9).
  0116 made it structural instead — `render-board.sh` now reads `DOCKET_STATUSES_TERMINAL` itself,
  so the arrays cannot drift out of correspondence without breaking the renderer directly. T9 was
  therefore dropped outright rather than carried forward as a deferred follow-up; there is nothing
  left for it to assert that isn't already guaranteed by the shared constant.

- **Renderer bug found while verifying case B — out of scope, follow-up only.** `render-board.sh`'s
  archive sort feeder joins each row's fields with a literal TAB and reads them back with
  `IFS=$'\t'`. Because TAB is one of the IFS-whitespace characters (not a single-character custom
  delimiter), an **empty id field collapses** on re-split and shifts every later field one position
  left. The renderer's own `[ -n "$id" ] || continue` guard runs *after* that shift, so it never
  actually sees an empty id — it sees the next field over — and a corrupt row like
  `| [0000](archive/) |  | <date> |` renders instead of being skipped. This does not change the
  predicate's correctness: a row that identifies nothing is not "the file's row" as far as the
  invariant is concerned, which is exactly why the archive-side message text says "no row
  identifying it" rather than claiming a row does or doesn't exist. The **active side is
  unaffected** — its id guard runs before any TAB-join, so no field-shift is possible there. Left
  alone per this change's scope (no edits to `render-board.sh`); tracked as a separate follow-up.

- **Real-archive verification.** The hermetic fixture suite in `tests/test_board_checks.sh` never
  exercises real repo history, so the widened check was also run directly against this repo's actual
  metadata worktree:
  ```
  bash scripts/board-checks.sh --changes-dir /Users/homer/dev/docket/.docket/docs/changes \
    --metadata-branch docket --integration-branch main | /usr/bin/grep '^board-row-dropped' || echo "clean"
  ```
  Result: `clean`. All 111 real `archive/` files carry a terminal status and a valid integer id, so
  the new archive-side arm produces zero findings against production data — consistent with the
  invariant having held by convention all along, just previously unchecked.

- **One shipped assertion inverted, not re-exempted.** `tests/test_board_checks.sh` case (d)
  previously asserted that `board-row-dropped` did **not** fire for an `archive/` file with no
  `id:` field — i.e., that `archive/` was exempt from the invariant. That exemption is precisely the
  premise this change deletes: extending the predicate to `archive/` makes a no-id archive file a
  live trigger, the same shape as the existing no-id active-file case. Re-exempting `archive/` or
  deleting the case outright would have thrown away the one pre-existing test that most directly
  contradicted the old (now-removed) assumption, so the block was inverted in place: it now asserts
  the check **fires** for that fixture (0073), and that `malformed-id`/`field-domain` do not explain
  it away.

## Follow-ups

- Fix `render-board.sh`'s archive sort feeder so an empty id field cannot collapse under
  `IFS=$'\t'` re-splitting (e.g. use a delimiter guaranteed absent from every field, or guard the
  split count) — the corrupt `| [0000](archive/) |  | <date> |` row described above. Deliberately
  out of scope for this change; `board-checks.sh`/`.md` and the test suite were the only files
  touched.
