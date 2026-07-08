# Retire the inline board/source-drift health check — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Retire the now-vacuous `inline` board/source-drift health check from `docket-status`, keep the `github` mirror-reachability visibility flag, and update the two tests that lock the retired check in place.

**Architecture:** A prose + test change only — no script behavior changes. In `skills/docket-status/SKILL.md` the single combined "Board/source drift" health-check bullet is split: the `inline` half is retired (with a terse note explaining why it is vacuous), the `github` mirror-reachability half survives as its own bullet. Two sentinel tests that asserted the old combined check are updated: `tests/test_board_checks.sh` swaps its keep-the-inline-check assertion for a positive anchor on the surviving `github` flag; `tests/test_board_refresh_on_transition.sh` drops assertion E (the retired tripwire).

**Tech Stack:** Bash sentinel tests (`grep`-based assertions), Markdown skill docs. No production code, no scripts touched.

## Global Constraints

- **No new ADR; no convention edit.** `adrs:` stays `[]`. Retiring a warn-only, now-vacuous check follows from change 0022's determinism and reverses no Accepted ADR (ADR-0012's script-vs-model boundary is untouched). (spec §4, §A3)
- **Keep the `github` half** — the mirror-reachability visibility flag must survive; do not silently fold it away. (spec §A4)
- **No absence sentinel for "board/source drift".** The retirement note legitimately still mentions the phrase, so a `! grep`/absence assertion would false-fail. Anchor the surviving contract with a POSITIVE assertion instead. (spec §4.2; LEARNINGS 2026-06-21 #36)
- **Do NOT edit** immutable historical records: `docs/adrs/0012-*`, and any `docs/changes/archive/*`, `docs/superpowers/specs/*`, `docs/superpowers/plans/*` mentioning the phrase — they are point-in-time build records. (spec §6)
- **Do NOT edit** `skills/docket-convention/SKILL.md` — its board-refresh-on-status-writes invariant (the real defense) stays; it enumerates no such check. (spec §6)
- **Scope:** only `skills/docket-status/SKILL.md`, `tests/test_board_checks.sh`, `tests/test_board_refresh_on_transition.sh`. (spec §6 touch-points — a floor, re-grepped at build; the reconcile pass confirmed exactly these three live consumers.)

---

### Task 1: Retire the inline drift check (skill prose + both sentinel tests)

This is one cohesive semantic change: the `SKILL.md` edit removes the `Board/source drift` (capital-B, space) phrase that assertion E greps for, so the skill edit and the assertion-E removal must land together to keep the suite green. The `test_board_checks.sh` assertion swap is TDD-first (new assertion is RED against current prose, then the skill edit turns it GREEN).

**Files:**
- Modify: `tests/test_board_checks.sh:322-328` (comment + the keep-the-inline-check assertion)
- Modify: `skills/docket-status/SKILL.md:185` (the combined "Board/source drift" bullet)
- Modify: `tests/test_board_refresh_on_transition.sh:28-30` (remove assertion E + its comment)

**Interfaces:**
- Consumes: nothing from earlier tasks (single-task plan).
- Produces: a `docket-status` SKILL whose Health-checks section carries a `github` mirror-reachability bullet containing the fixed substring **`mirror reachability`** (the anchor the new test greps), and no `Board/source drift` (capital-B, space) occurrence.

---

- [ ] **Step 1: Write the failing test — swap the keep-the-inline-check assertion for the surviving-`github`-flag assertion**

In `tests/test_board_checks.sh`, replace the comment block + assertion at lines 322–328. Current text:

```bash
# The five mechanical checks are now delegated — their old standalone bullets are gone as bullets,
# but the SKILL still names them so a reader knows what the script covers. Assert the two
# judgment/0024 checks remain explicitly model-driven, each anchored to a phrase it owns.
assert "docket-status keeps blocked_by re-examination model-driven" \
  'grep -qiF "blocked_by:" "$SKILL"'
assert "docket-status keeps the inline board/source drift check (owned by change 0024)" \
  'grep -qiF "board/source drift" "$SKILL" || grep -qiF "board/source-drift" "$SKILL"'
```

Replace with (keep the `blocked_by` assertion; swap only the drift one; update the comment to match reality):

```bash
# The five mechanical checks are now delegated — their old standalone bullets are gone as bullets,
# but the SKILL still names them so a reader knows what the script covers. Assert the surviving
# model-driven signals, each anchored to a phrase it owns: the blocked_by re-examination
# (judgment) and the github mirror-reachability visibility flag. Change 0024 retired the inline
# board/source-drift check (deterministic render + the unconditional Board-pass re-render make it
# vacuous); its removed tripwire lives in tests/test_board_refresh_on_transition.sh.
assert "docket-status keeps blocked_by re-examination model-driven" \
  'grep -qiF "blocked_by:" "$SKILL"'
assert "docket-status keeps the github mirror-reachability visibility flag (survives 0024 inline-drift retirement)" \
  'grep -qiF "mirror reachability" "$SKILL" || grep -qiF "mirror-reachability" "$SKILL"'
```

- [ ] **Step 2: Run the test to verify the new assertion FAILS (RED)**

Run: `cd /Users/homer/dev/docket/.worktrees/retire-board-source-drift-check && bash tests/test_board_checks.sh`
Expected: overall `FAIL`, with the line `NOT OK - docket-status keeps the github mirror-reachability visibility flag (survives 0024 inline-drift retirement)` — because current `SKILL.md` has no `mirror reachability` phrase. (The `blocked_by` assertion still passes.) This proves the new assertion is non-vacuous: it is currently unmet.

- [ ] **Step 3: Edit the skill — split the combined bullet into a `github`-only reachability bullet + inline-retirement note**

In `skills/docket-status/SKILL.md`, replace the entire line-185 bullet. Current text:

```markdown
- **Board/source drift** — runs **per enabled surface** (skipped entirely when `board_surfaces: []`). For `inline`: render the board in-memory from the change files (reusing the shared dependency-resolution pass) and compare it to the committed `BOARD.md`; if any change's rendered status or placement disagrees, **warn** naming the change(s) (a writer skipped the board-refresh invariant). For `github`: warn on a change carrying an `issue:` whose mirror is unreachable (the upsert is best-effort and self-heals; this is only a visibility flag). Like the other checks it only warns — it never auto-fixes; the Board pass in this same `docket-status` run re-renders the enabled surfaces and heals the drift regardless. A best-effort refresh is allowed to lose a race. (Retiring/downgrading this `inline` drift check once rendering is deterministic is change **0024**.)
```

Replace with:

```markdown
- **`github` mirror reachability** — runs only when `board_surfaces` includes `github` (skipped otherwise): warn on a change carrying an `issue:` whose mirror is unreachable (the upsert is best-effort and self-heals; this is only a visibility flag). Like the other checks it only warns — it never auto-fixes, and a best-effort refresh is allowed to lose a race. *(The paired `inline` board/source-drift check was **retired by change 0024**. It became vacuous once change 0022's `render-board.sh` made `inline` rendering deterministic: `docket-status`'s Board pass unconditionally re-renders `BOARD.md` **before** this Health-checks pass, healing any staleness first, so a drift check placed here cannot observe the condition it existed to flag. The convention's board-refresh-on-status-writes invariant is the real defense; the board is a self-healing derived view.)*
```

Note: the new text uses lowercase-hyphen `board/source-drift`, so no `Board/source drift` (capital-B, space) remains — this is what turns assertion E red and motivates Step 6. It contains the fixed substring `mirror reachability` (the Step-1 anchor) exactly once, in the surviving `github` clause. It does not touch the `do not auto-fix` phrase on line 165 (asserted separately at `test_board_checks.sh:329`).

- [ ] **Step 4: Run the test to verify GREEN**

Run: `cd /Users/homer/dev/docket/.worktrees/retire-board-source-drift-check && bash tests/test_board_checks.sh`
Expected: overall `PASS`. In particular `ok - docket-status keeps the github mirror-reachability visibility flag (survives 0024 inline-drift retirement)` and `ok - docket-status keeps the do-not-auto-fix stance` (line-165 phrase untouched).

- [ ] **Step 5: Mutation-check the new assertion (prove non-vacuity)**

Temporarily delete the `mirror reachability` heading token from the skill and confirm the new assertion flips to NOT OK, then restore. Run:

```bash
cd /Users/homer/dev/docket/.worktrees/retire-board-source-drift-check
cp skills/docket-status/SKILL.md /tmp/skill.bak
# neutralize only the anchor token
perl -0pi -e 's/mirror reachability/mirror XXXXX/g' skills/docket-status/SKILL.md
bash tests/test_board_checks.sh | grep -i "mirror-reachability visibility flag"   # expect: NOT OK - ...
cp /tmp/skill.bak skills/docket-status/SKILL.md                                    # restore verbatim
diff -q /tmp/skill.bak skills/docket-status/SKILL.md && echo RESTORED-CLEAN
```
Expected: the grep shows `NOT OK - docket-status keeps the github mirror-reachability visibility flag ...` (mutation caught), then `RESTORED-CLEAN`. Re-run `bash tests/test_board_checks.sh` → `PASS` to confirm the restore.

- [ ] **Step 6: Remove the now-broken assertion E from `test_board_refresh_on_transition.sh`**

In `tests/test_board_refresh_on_transition.sh`, delete these three lines (28–30) and the blank line separating E from D if it leaves a double blank:

```bash
# E. docket-status gains the board/source drift tripwire (a warning).
assert "docket-status has board/source drift health check" \
  'grep -q "Board/source drift" skills/docket-status/SKILL.md'
```

Assertions A–D (the board-refresh *invariant* in the convention + the per-site refreshes — the surviving contract of change 0004) stay untouched. The file's `exit $fail` line stays last.

- [ ] **Step 7: Run `test_board_refresh_on_transition.sh` to verify GREEN**

Run: `cd /Users/homer/dev/docket/.worktrees/retire-board-source-drift-check && bash tests/test_board_refresh_on_transition.sh; echo "exit=$?"`
Expected: `exit=0`, four `ok - ...` lines (A–D), and NO line mentioning `board/source drift health check`.

- [ ] **Step 8: Build-time regrep — confirm no live consumer of the phrase was missed**

Run: `cd /Users/homer/dev/docket/.worktrees/retire-board-source-drift-check && grep -rniE 'board/source[- ]drift' skills tests scripts`
Expected: matches ONLY inside `skills/docket-status/SKILL.md` (the retirement note) and the updated `tests/test_board_checks.sh` comment. NO match in any other live file, and NO `Board/source drift` (capital-B, space) anywhere. (spec §6 build-time regrep; LEARNINGS 2026-07-08 #42 / 2026-06-20 #32 — enumerated touch-points are a floor.)

- [ ] **Step 9: Run the full test suite**

Run: `cd /Users/homer/dev/docket/.worktrees/retire-board-source-drift-check && for t in tests/test_*.sh; do echo "== $t =="; bash "$t" >/tmp/o 2>&1; echo "exit=$?"; grep -iE 'NOT OK|^FAIL' /tmp/o || echo "clean"; done`
Expected: every test `exit=0` / `clean` (the retirement touches only docket-status prose + two test files; `render-board.sh`/`board-checks.sh` behavior is unchanged, so their tests are unaffected). If a test runner script exists (e.g. `tests/run_all.sh`), run that too.

- [ ] **Step 10: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/retire-board-source-drift-check
git add skills/docket-status/SKILL.md tests/test_board_checks.sh tests/test_board_refresh_on_transition.sh docs/superpowers/plans/2026-07-08-retire-board-source-drift-check.md
git commit -m "feat(0024): retire inline board/source-drift check; keep github reachability flag"
```
