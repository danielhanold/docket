# docket-status report is self-evidencing and board-independent — results

Change: #69 · Branch: `feat/status-report-self-evidencing` · PR: (see change file) · Plan: `docs/superpowers/plans/2026-07-13-status-report-self-evidencing.md` · ADRs: 28

## Verify (human)

- [ ] **Read the spec deviation below (Findings #1) and confirm it is what you want.** It is the one place the shipped behavior departs from the letter of the spec. Everything else follows the spec exactly.
- [ ] Optional live check in a board-off repo (the one that motivated this change): run `docket-status.sh` where `board_surfaces: []` and confirm the report now reads `board off` → digest lines → `pass ok` rather than printing nothing.

## Findings

**1. Spec deviation (deliberate, and the reason for it) — the digest projects POST-sweep state on a full pass.**

The spec says `backlog_pass()` is *"placed **before** the `--board-only` early exit"* — a single call. Built that way, it shipped a report that contradicts itself in the exact scenario the change exists to serve (`docket-implement-next`'s Step-0 merge sweep):

```
board off
backlog implemented 1
change 60 implemented - gate-thing     <- stale: captured before the sweep
swept 60 2026-07-11                    <- ...which then swept it to done
pass ok
```

Because this change makes the digest the **sole** backlog channel (the skill is now explicitly forbidden from opening `BOARD.md`), that staleness has no corrective path: the skill would summarize "#60 implemented, awaiting merge" for a change the same report says it just closed.

Shipped instead: `backlog_pass` is called **once per path** — before the early exit on `--board-only` (which performs no sweep, so nothing to be stale about, and this is the spec's stated reason for the placement), and **after** the sweep on a full pass. Both of the spec's normative requirements still hold — it runs in both modes, and `--board-only` reports the backlog in every config — while the full-pass report is now truthful at end of pass. The final reviewer independently judged the deviation defensible: the spec's placement sentence describes a *mechanism* for a single-call implementation and never contemplated the digest's interaction with the sweep. Documented at three sites (code comment, `scripts/docket-status.md`, the SKILL) and locked by tests that go red if the call moves back.

The **plan file** (`docs/superpowers/plans/…`, lines ~435/442) still carries the original pre-sweep wording — it is a historical build artifact, not a live contract; `scripts/docket-status.md` is the authoritative one and is correct.

**2. Every full-pass test was silently exercising the best-effort failure branch.** The suite's non-`--board-only` fixtures all point `SCRIPTS_DIR` at a mock dir that contains no `render-board.sh`, so the digest failed and degraded on every full-pass test. Consequence: the change's two headline claims — "the backlog pass is **ungated**" and "`main()` **always** closes with `pass ok`" — had **zero** real coverage; deleting the full-path `pass ok`, or gating the pass on `--board-only`, both left the suite green. Fixed by giving one full-pass fixture the real `render-board.sh` + its `lib/`. This is the LEARNINGS "green tests ≠ the hard branch was exercised" family, hit again — the mock was shaped to the *code path* rather than the real tool.

**3. Two doc sentinels shipped double-guarded** (`grep -qF "digest"` → 3 occurrences; `grep -qF "board off"` → 4). The reviewer rewrote the Final summary to literally say *"read from `BOARD.md`"* — the exact posture this change abolishes — and the assert stayed green. Re-anchored to the unique phrase each clause owns (`grep -c` == 1, mutation-confirmed). Same family as the LEARNINGS anchoring entry; the fix is the one that ledger prescribes.

**4. Change 0059's sentinel had to be narrowed, not kept.** It asserted `docket-status.sh` never mentions `/render-board.sh` at all — which `backlog_pass` necessarily violates. It now asserts every invocation is the read-only `--format digest` (tokenized **per invocation**, not per line, so a gated and an ungated call side by side cannot whitewash each other), plus a **second, independent** scan asserting the orchestrator never redirects the renderer's stdout into `BOARD.md`. Two guards because they catch different holes: mutation proved the flag-tokenizer alone misses `render-board.sh --format digest > BOARD.md`.

**5. ADR-0028 — "a report channel is not a board surface."** The load-bearing decision: the digest is report output (persists nothing, no git ops), which is *why* it can run ungated while `board_surfaces: []` keeps meaning "no board is rendered or committed." Split: **`board-refresh.sh` gates the surface; `render-board.sh` serves the report.** The test a future contributor should apply to any new channel: *does it persist anything?* If not, it is a report and is ungated; if so, it is a surface and must go through the gate.

**6. Pre-existing contract drift, corrected in passing (disclosed at reconcile).** `scripts/docket-status.md`'s step-3 prose still described the inline board pass as rendering into a `BOARD.md.tmp` file — stale since 0059 moved that write into `board-refresh.sh`. Corrected rather than left knowingly false beside new prose. Likewise `skills/docket-status/SKILL.md`'s `### Board` reference paragraph, which still claimed `render-board.sh` writes and commits `BOARD.md`.

## Follow-ups

- **`REDIRECT_RE` is a leaky matcher (Minor, inherited — not introduced here).** `tests/test_render_board.sh:270` requires whitespace on *both* sides of `>`, so it catches `… > "$1/BOARD.md"` but **misses** `>"$1/BOARD.md"` (no space) and `>>`. It is byte-identical to `origin/main`, so this has always applied to the cross-skill `skills/*/SKILL.md` scan; this change merely also points it at `scripts/docket-status.sh`, where it is now the **only** guard on that write (the flag-tokenizer misses it — mutation-proven). Tightening to `[[:space:]]>>?[[:space:]]*` would close it at both scan sites. Not fixed here deliberately: the regex carries a 15-line design comment explaining exactly which false-positive classes (bracket placeholders, markdown blockquotes) its current shape excludes, and re-deriving that analysis is its own small change rather than a late scope-add on this one. **Recommend a follow-up stub.**
- Consider whether `board_pass`'s inline surface should also move after the sweep for the same truthfulness reason the digest did (a board-on repo's `BOARD.md` has been a pre-sweep snapshot since 0058). Out of scope here — the board is refreshed again by the next pass, and unlike the digest it is not the sole channel — but it is the same latent staleness.
