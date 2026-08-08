<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0259 — Harden render-board: sanitize feeder values and settle the failure contract](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0259-harden-render-board-sanitize-feeder-values-and-settle-the-fa.md)**
<!-- docket:backlink:end -->

# Harden render-board: sanitize feeder values and settle the failure contract — results

Change: #0259 · Branch: feat/harden-render-board-sanitize-feeder-values-and-settle-the-fa · PR: (see change `pr:`) · Plan: docs/superpowers/plans/2026-08-07-harden-render-board-sanitize-feeder-values-and-settle-the-fa.md · ADRs: none

## Verify (human)

- [ ] **Accept the availability trade-off, now that it is wider than the spec described.** Spec Assumption 1 accepted that one malformed change file freezes `BOARD.md` and halts autonomous selection until a human fixes the named file. As shipped that is true of **five** rejection classes (M1, M2, M3, M5 upfront; M4 in the archive feeder pass), and — after the blocker fix — it is true on the **digest** path as well as the markdown one. Before this change the digest path exited 0 regardless. Confirm the loop-halting behavior is what you want on a repo whose `.docket` worktree is shared between concurrent agents.
- [ ] **Confirm M5's blast radius is acceptable.** M5 rejects any change file whose *path* contains a TAB or CR. It was added at review time to close a gap the spec never named (see Findings 4). It is a new malformed class, not a refinement of an existing one, and it fires before M1–M4.
- [ ] **Sanity-check the board renders unchanged against the live backlog.** The perf fix (`f7d44db6`) was accepted on a byte-identical diff of both projections over the 261-file live backlog plus a wall-clock A/B, not on a suite assert — a behavior-preserving optimization has no oracle in the tests. Numbers recorded under Findings 8.

## Findings

Whole-branch review ran at the **deep** rung (highest build profile routed was `premium`; the 1015-line diff was under the 1500-line bump threshold). It returned **8 findings: 1 blocker, 3 important, 4 minor**. All are dispositioned in the PR body's table; the ones with lasting consequences are recorded here.

**1 (blocker, fixed `27c85f0a`) — M4 never ran under `--format digest`.** The read-back guard sat inside the archive rendering block, below the digest block's early exit. Markdown returned 3 on an M4-malformed fixture; the digest returned **0 with empty stderr and a wrong `backlog done` tally**. That split the contract across the two enumerated consumers over identical input: `board-refresh.sh` froze the board while `docket-status.sh`'s `digest_only_pass` handed `docket-implement-next` a queue built from a corrupt archive. Three places already asserted the opposite and were false. This is the concrete instance of the `exit-code-encodes-a-non-failure` learning's core claim — a documented exit-code table constrains nobody; only the wiring does.

**2 (important, fixed by the same commit) — an M4-rejected file was still counted.** M4 fired after both tally loops and never populated `BAD`, so the rejected file stayed in `ARC_COUNT`, the `<summary>` header, and the digest rollup — a fixture rendered `Archive — done (2)` above exactly one row. `board-checks.sh`'s `board-row-dropped` backstop does **not** catch this class: `renders_row` models an archive file with a usable id and a terminal status as rendered. The review also found a **fourth** `ARCFILES` consumer with no gate at all (the mermaid `DONE_IDS` loop), contradicting the in-code claim that `BAD` was honored "at all three consumers".

**3 (important, fixed by the same commit) — the diagnostic emitted the file path raw.** `mark_malformed` sanitized the offending *value* but not the *path*, so M4's own case — a TAB in a filename — wrote a literal TAB to stderr and from there into a docket-status report. The assert that should have caught it was scoped to a fixture whose malformation was an interior-TAB *status*, which structurally cannot produce a raw TAB in a path.

**4 (important, fixed `b1615773`) — the ACTIVE-side feeder had the identical exposure, undisclosed.** `SECTION` uses the same `id<TAB>file` join and the same two-variable split, with four consumers and no guard: a TAB in an active filename rendered a raw TAB into `BOARD.md` at **exit 0**. The "future control-character path" M4 was written to guard against was already reachable. Closed with **M5**, a fourth upfront class rejecting any TAB/CR-bearing path, chosen over a per-consumer guard because guarding one of four sites leaves three open plus any fifth added later.

**5 and 6 (minor, resolved by the blocker fix).** The in-script comment said "Two conjuncts" while describing three, and the `[ ! -e "$f" ]` conjunct would have turned a benign concurrent file move — routine in this repo's shared `.docket` worktree — into exit 3 with a diagnostic blaming tuple corruption. The blocker fix replaced the conjunct list with a genuine round trip (join, split back with the consumer's own `read`, require every field unchanged), which removed both.

**7 (minor, recorded — not fixed).** The review asked for a fixture exercising M4's shifted-tuple conjuncts. After the M5 fix that became **structurally impossible**: M5 rejects any TAB-bearing path upfront, and M3 subsumes an interior-TAB status, so M4's conjuncts are now unreachable by construction rather than merely untested. They are retained as defence in depth and labelled as such in both the code and `scripts/render-board.md`. The SECTION-loop `BAD` gate is likewise unobservable — it mutation-tested to zero failures. **Residual risk:** three guard clauses have no assert that can go red, so a future regression could defeat them silently. This is disclosure, not coverage.

**8 (minor, fixed `f7d44db6`) — the validation pass re-read and discarded every field.** Measured at ~16% slower on the live backlog. Caching the validated id/status made it **~31% faster than the pre-fix branch and faster than `origin/main`** (10.97/10.98/11.06s → 7.58/7.63/7.63s), with both projections byte-identical over the 261-file live backlog.

**No ADR was recorded.** The one genuinely architectural decision — a renderer that fails the whole run on one malformed file, halting autonomous selection — was settled at design time in the spec's Assumption 1, not during implementation. The implementation-time decisions (round trip over a conjunct list; M5 upfront over a per-consumer guard) are tactical and documented in `scripts/render-board.md`.

## Plan deviations

- **The plan's M4 mechanism was wrong.** `IFS=$'\t' read -r date id st f` assigns the *unsplit remainder* to the final field, so a TAB in a filename lands wholly inside `f` without shifting anything. The plan's two conjuncts left the test red when applied verbatim. Corrected during Task 3, then superseded entirely by the blocker fix's round trip.
- **The plan placed M4 at the render-time consumer.** That placement is what put it below the digest's early exit — the blocker. Validation now runs in a feeder pass above both emission paths.
- **M5 is not in the plan or the spec.** It was added at review time to close finding 4.
- The plan's two filename-plus-line-number comment anchors were rejected by `tests/test_comment_anchor_style.sh` and re-anchored on verbatim-quoted clauses.

## Follow-ups

None minted. The `sanitize()` duplication between `render-board.sh` and `board-checks.sh` remains deliberate (spec Assumption 5, `board-checks.sh` explicitly out of scope) — it is a three-line dedupe, below the bar for its own change. Nothing else surfaced clearing auto-capture's six admission gates; every review finding was about this branch's own diff, which is never mintable.
