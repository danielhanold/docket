# Second-Round Skill Slimming Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Re-slim the docket skill bodies that regrew since the 0053–0055 round back to at-or-under their post-slim targets, add a `--must-land` board-pass flag that pulls the duplicated caller retry litany into `docket-status.sh`, and add a size-budget test that fails the suite on future regrowth.

**Architecture:** One mechanical shift (the `--must-land` flag, fully TDD-tested at the script boundary) plus a behavior-neutral editorial slim of the convention, the nine operating skills, and the two references — every cut is narration, a duplicate restated elsewhere, or content moved to a one-level-deep reference behind a loud blocking pointer. The largest single move is the Learnings-ledger deep mechanics from the convention into a new `references/learnings.md`. A new regrowth-guard test asserts per-file line/word budgets over `skills/**/*.md`.

**Tech Stack:** Bash (POSIX-portable, BSD+GNU), markdown skill files, the repo's self-contained shell test harness (`ok -` / `NOT OK -` asserts, `exit $fail`).

## Global Constraints

- **Behavior-neutrality outranks the number.** On every editorial task the size target is a *direction, not a gate* (learnings finding `size-target-is-direction`, #55). Once a whole-branch read shows the residual lines are load-bearing/test-anchored, accept the size and stop trimming. The size-budget test's numbers are set from the *actual* post-slim sizes + ~10%, not the other way round.
- **The one allowed semantic change is `--must-land`.** Every other change is behavior-neutral restructure: each deleted sentence must be (a) narration, (b) restated inline elsewhere, (c) moved to a reference, or (d) covered by a script contract. (Spec §Decisions.2, §6.2.)
- **Sentinels are re-anchored, never re-gated to green** (findings `foundational-test-discipline`, `test-premise-deleted-not-regated`). A sentinel grepping prose that moves is re-pointed to the new home preserving its INTENT; a heading that stays must stay byte-stable.
- **Extraction leaves a stub + pointer under the original heading** (finding `skill-extraction-and-stub-pointer`, #20), and the MOVE is verified by reading the sibling against the base section. Invoking a skill loads only its `SKILL.md`; sibling reference files are NOT auto-loaded, so every consumer that needs the moved mechanics keeps a pointer.
- **`--must-land` becomes the SOLE channel for the board-pass retry** (finding `sole-channel`, #69/71): prove the report channel is TOTAL (every path emits exactly one classified outcome + a mapped exit code — "no line" is never success), and enumerate the retry contract by its RETRYABLE set (`board inline changed push-failed` ONLY), never its terminal set.
- **Metadata-branch files are invisible to the suite** (finding `metadata-branch-invisible-to-suite`, #6). `skills/**` is on the integration branch, so the budget test can read it; the learnings *finding files* live on `docket` and are NOT part of this change.
- **Shell rules (always-in-context, AGENTS.md):** `set -uo pipefail`; never a producer piped into an early-exiting consumer under pipefail (capture into a var first); BSD+GNU-portable `sed -E`/`awk`; `grep -F --` for fixed-string patterns that start with `-`.
- **Facade discipline:** any helper invocation in skill prose uses the byte-exact `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh <op>` form; descriptive NOUN mentions of a script are permitted. `docket.sh docket-status --board-only` must stay present in exactly the 7 files it lives in today.
- **Feature branch adds only plan + results + code.** Never touch docket metadata (change files, BOARD.md, ADRs) from this branch. The skill files, scripts, tests, and references ARE the code here.

---

## Baseline sizes (measured on `origin/main`, 2026-07-16)

| File | Now | Target (direction) |
|---|---|---|
| `skills/docket-convention/SKILL.md` | 329 L / 5453 w | ≤ ~200 L / ≤ ~2600 w |
| `skills/docket-finalize-change/SKILL.md` | 157 L / 2821 w | ≤ ~140 L / ≤ ~2200 w |
| `skills/docket-status/SKILL.md` | 114 L / 2434 w | ~100 L / ≤ ~1700 w |
| `skills/docket-implement-next/SKILL.md` | 108 L / 2491 w | ~100 L / ≤ ~2100 w |
| `skills/docket-adr/SKILL.md` | 90 L / 1402 w | ~80 L |
| `skills/docket-groom-next/SKILL.md` | 75 L / 1527 w | ~65 L |
| `skills/docket-new-change/SKILL.md` | 59 L / 1549 w | ~55 L / ≤ ~1100 w |
| `skills/docket-auto-groom/SKILL.md` | 64 L / 1256 w | light narration pass |
| `skills/docket-brainstorm/SKILL.md` | 78 L / 653 w | light narration pass |
| `skills/docket-convention/references/agent-layer.md` | 165 L / 1833 w | ≤ ~150 L, TOC if >100 L |
| `skills/docket-convention/references/terminal-close-out.md` | 135 L / 1155 w | ≤ ~150 L, TOC if >100 L |
| `skills/docket-convention/references/learnings.md` | (new) | ≤ ~150 L |

Re-measure at build time with: `for f in <files>; do git show origin/main:"$f" | wc -lw; done` (or `wc -lw <file>` in the worktree).

---

## Task 1: `--must-land` board-pass flag in `docket-status.sh` + script tests + contract

**Files:**
- Modify: `scripts/docket-status.sh` (arg parser ~line 27–39; add two functions near `board_pass`; `main()` ~line 587–598)
- Modify: `scripts/docket-status.md` (Usage flags table; a new Behavior sub-step; Exit codes)
- Test: `tests/test_docket_status.sh` (append a new `--must-land` section after the existing board-pass fixtures, ~line 550)

**Interfaces:**
- Consumes: existing `board_pass` (emits the `board …` report lines and may `exit 2` fail-closed), `docket_metadata_worktree`, the `GIT` mock seam.
- Produces: `--must-land` flag; `MUST_LAND` var (default 0); `board_classify BOARD_OUT` → prints `success`|`retryable`|`failed`; `board_pass_must_land` → returns 0 on success, 1 on failure/exhaustion, propagates `board_pass`'s `exit 2`. Report-line vocabulary is UNCHANGED. Flagless behavior is byte-identical to today.

**Contract of the flag (from spec §1):**
- Bounded retry on `board inline changed push-failed` ONLY: re-sync the metadata worktree (`git -C "$mw" pull --rebase`) and re-render, **3 attempts total**.
- Exit 0 ⇔ every emitted `board …` line is a terminal SUCCESS line: `board inline changed pushed`, `board inline clean`, `board off`, `board github ok`. Any other terminal line, or retry exhaustion, prints its report line(s) and exits non-zero.
- `board_pass`'s fail-closed `exit 2` (empty/whitespace/`none`-combined `BOARD_SURFACES`) propagates unchanged.
- Absence of any `board …` line is treated as `failed`, never success (sole-channel totality).

- [ ] **Step 1: Write the failing tests.** Append to `tests/test_docket_status.sh` (after the existing `board-case`/`unpushed-case` fixtures, before the `detect_merged` section). Add a dedicated hermetic fixture so earlier board tests don't interfere:

```bash
# ============================================================================
# --must-land (change 0085): the board-pass retry loop + exit-code mapping move
# into the script. Vocabulary unchanged; flagless behavior byte-identical.
# ============================================================================

# Fresh hermetic fixture: a clone with an unpushed board change so a push is attempted.
git_repo_setup "$tmp/mustland-case"
git clone -q "$tmp/mustland-case/origin.git" "$tmp/mustland-case/work" 2>/dev/null
seed_changes_fixture "$tmp/mustland-case/work"
git -C "$tmp/mustland-case/work" -c user.email=t@t -c user.name=t add docs/changes
git -C "$tmp/mustland-case/work" -c user.email=t@t -c user.name=t commit -q -m "seed changes fixture"
git -C "$tmp/mustland-case/work" push -q origin main

# A: must-land success — a normal render pushes; exit 0, pass ok present.
write_board_fixture inline
(cd "$tmp/mustland-case/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-board.sh" "$SCRIPT" --board-only --must-land >"$tmp/ml-ok.txt" 2>"$tmp/ml-ok-err.txt")
rc=$?
assert "must-land success exits zero" '[ $rc -eq 0 ]'
assert "must-land success reports a terminal-success board line" \
  'grep -Eq "board inline (changed pushed|clean)" "$tmp/ml-ok.txt"'
assert "must-land success still closes with pass ok" 'grep -qxF "pass ok" "$tmp/ml-ok.txt"'

# B: must-land board-off (none) — a deliberate off-state is success; exit 0.
write_board_fixture none
(cd "$tmp/mustland-case/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-board.sh" "$SCRIPT" --board-only --must-land >"$tmp/ml-off.txt" 2>"$tmp/ml-off-err.txt")
rc=$?
assert "must-land board-off exits zero (deliberate off-state is success)" '[ $rc -eq 0 ]'
assert "must-land board-off emits a positive board off line" 'grep -qxF "board off" "$tmp/ml-off.txt"'

# C: must-land fail-closed (empty surfaces) — exit 2 PROPAGATES unchanged.
write_board_fixture ""
(cd "$tmp/mustland-case/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-board.sh" "$SCRIPT" --board-only --must-land >"$tmp/ml-empty.txt" 2>"$tmp/ml-empty-err.txt")
rc=$?
assert "must-land empty-surfaces exits 2 (fail-closed propagates)" '[ $rc -eq 2 ]'
assert "must-land empty-surfaces names the unresolved config on stderr" \
  'grep -qF "BOARD_SURFACES" "$tmp/ml-empty-err.txt"'

# D: must-land unknown token — a non-retryable failure line; exit non-zero, NO retry.
write_board_fixture "inlne"
(cd "$tmp/mustland-case/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-board.sh" "$SCRIPT" --board-only --must-land >"$tmp/ml-unknown.txt" 2>"$tmp/ml-unknown-err.txt")
rc=$?
assert "must-land unknown-token exits non-zero (unknown is a failure, not success)" '[ $rc -ne 0 ]'
assert "must-land unknown-token emits the unknown line exactly once (no retry on a non-retryable line)" \
  '[ "$(grep -cxF "board inlne unknown" "$tmp/ml-unknown.txt")" -eq 1 ]'
assert "must-land unknown-token never prints pass ok" '! grep -qxF "pass ok" "$tmp/ml-unknown.txt"'

# E: must-land persistent push-failure — retries EXACTLY 3× then exits non-zero.
# GIT mock: real git for everything except `push`, which always fails. `push` is always the
# 3rd token (git -C "$mw" push); pull --rebase (the re-sync) still succeeds against the bare origin.
cat > "$tmp/git-nopush.sh" <<'EOF'
#!/usr/bin/env bash
sub="$1"; [ "$sub" = "-C" ] && sub="$3"
if [ "$sub" = push ]; then echo "git-nopush: push rejected" >&2; exit 1; fi
exec git "$@"
EOF
chmod +x "$tmp/git-nopush.sh"
# Give board_pass something to render+commit+push: mutate a change so BOARD.md changes.
sed -i.bak 's/Alpha feature/Alpha feature v3/' "$tmp/mustland-case/work/docs/changes/active/0001-alpha.md"
rm -f "$tmp/mustland-case/work/docs/changes/active/0001-alpha.md.bak"
write_board_fixture inline
(cd "$tmp/mustland-case/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-board.sh" GIT="$tmp/git-nopush.sh" "$SCRIPT" --board-only --must-land >"$tmp/ml-pf.txt" 2>"$tmp/ml-pf-err.txt")
rc=$?
assert "must-land persistent push-failure exits non-zero (retry exhausted)" '[ $rc -ne 0 ]'
assert "must-land persistent push-failure retries exactly 3 times (push-failed line ×3)" \
  '[ "$(grep -cxF "board inline changed push-failed" "$tmp/ml-pf.txt")" -eq 3 ]'
assert "must-land persistent push-failure never prints pass ok" '! grep -qxF "pass ok" "$tmp/ml-pf.txt"'

# F: FLAGLESS NEUTRALITY — the same push-failure WITHOUT --must-land is best-effort: exit 0,
# the push-failed line appears exactly once (no retry), pass ok present. Proves flagless is
# byte-identical to pre-0085.
(cd "$tmp/mustland-case/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-board.sh" GIT="$tmp/git-nopush.sh" "$SCRIPT" --board-only >"$tmp/ml-flagless.txt" 2>"$tmp/ml-flagless-err.txt")
rc=$?
assert "flagless push-failure exits zero (best-effort, unchanged)" '[ $rc -eq 0 ]'
assert "flagless push-failure emits the push-failed line exactly once (no retry)" \
  '[ "$(grep -cxF "board inline changed push-failed" "$tmp/ml-flagless.txt")" -eq 1 ]'
assert "flagless push-failure still closes with pass ok" 'grep -qxF "pass ok" "$tmp/ml-flagless.txt"'
```

- [ ] **Step 2: Run the tests to verify they fail.**

Run: `bash tests/test_docket_status.sh 2>&1 | grep "NOT OK"`
Expected: the new `must-land …` asserts fail (the flag is unrecognized → `docket-status: unknown argument: --must-land`, exit 2), while every pre-existing assert still passes.

- [ ] **Step 3: Add the flag to the arg parser.** In `scripts/docket-status.sh`, add the default and the case arm:

```bash
# with the other defaults (~line 27):
BOARD_ONLY=0 MUST_LAND=0 REPO_FLAG="" PROJECT_FLAG="" AUTO_CREATE_PROJECT=0 PROJECT_OWNER=""
```
```bash
# in the while/case parser (~line 31), next to --board-only:
    --must-land) MUST_LAND=1 ;;
```

Update the usage header comment block (lines 7–13) to list `[--must-land]` and one line:
```
#   --must-land            (with --board-only) retry a push-failed board write in-script and
#                          map the outcome to the exit code (0 = board landed); see docket-status.md
```

- [ ] **Step 4: Add `board_classify` and `board_pass_must_land`.** Insert immediately AFTER the `board_pass()` function definition (after line 97):

```bash
# board_classify BOARD_OUT — reduces captured board-pass stdout to one verdict (change 0085):
#   failed    — any non-retryable board failure line, or NO board line at all (sole-channel:
#               "no line" is never success)
#   retryable — at least one `board inline changed push-failed` and no non-retryable failure
#   success   — every `board …` line is a terminal success line
# Precedence: failed > retryable > success. Non-`board ` lines (minted …, digest) are ignored.
board_classify(){
  local out="$1" line has_retryable=0 has_failed=0 has_board=0
  while IFS= read -r line; do
    case "$line" in
      "board "*) has_board=1 ;;
      *) continue ;;
    esac
    case "$line" in
      "board inline changed pushed"|"board inline clean"|"board off"|"board github ok") ;;
      "board inline changed push-failed") has_retryable=1 ;;
      *) has_failed=1 ;;   # board inline failed | board github failed | board <tok> unknown | anything else
    esac
  done <<<"$out"
  if [ "$has_board" -eq 0 ] || [ "$has_failed" -eq 1 ]; then echo failed
  elif [ "$has_retryable" -eq 1 ]; then echo retryable
  else echo success; fi
}

# board_pass_must_land — the --must-land wrapper (change 0085). Runs board_pass; on the SOLE
# retryable outcome (`board inline changed push-failed`) re-syncs the metadata worktree and
# re-renders, up to 3 attempts total. Returns 0 iff every emitted `board …` line is a terminal
# success line; prints the report line(s) each attempt and returns non-zero on any other terminal
# line or on retry exhaustion. board_pass's fail-closed `exit 2` (unresolved config) is captured
# via the command substitution's exit status and propagated verbatim. Flagless callers never reach
# this — main() invokes board_pass directly, byte for byte as before.
board_pass_must_land(){
  local mw board_out rc attempt=0 verdict
  mw="$(docket_metadata_worktree)"
  while :; do
    attempt=$((attempt + 1))
    board_out="$(board_pass)"; rc=$?
    [ -n "$board_out" ] && printf '%s\n' "$board_out"
    [ "$rc" -ne 0 ] && exit "$rc"   # board_pass hard-failed (fail-closed) — propagate verbatim
    verdict="$(board_classify "$board_out")"
    case "$verdict" in
      success) return 0 ;;
      failed)  return 1 ;;
      retryable)
        [ "$attempt" -ge 3 ] && return 1   # exhausted — the push-failed line is already printed
        "$GIT" -C "$mw" pull --rebase >&2 2>&1 || true
        ;;
    esac
  done
}
```

- [ ] **Step 5: Route `main()` through the wrapper when `--must-land`.** Replace the top of `main()` (the bare `board_pass` on line 589) with:

```bash
main(){
  docket_preflight "$SCRIPTS_DIR" || exit 1
  if [ "$MUST_LAND" = 1 ]; then
    board_pass_must_land || exit 1
  else
    board_pass
  fi
  if [ "$BOARD_ONLY" = 1 ]; then
```

(The rest of `main()` is unchanged. Flagless: `MUST_LAND=0` → `board_pass` is called exactly as before, so every pre-existing test stays byte-identical. Must-land success falls through to the same `--board-only` block, `backlog_pass` → `pass ok` → exit 0. Must-land failure exits 1 before `pass ok`.)

- [ ] **Step 6: Run the tests to verify they pass.**

Run: `bash tests/test_docket_status.sh 2>&1 | tail -2`
Expected: `PASS`-equivalent — the last line is the fail count check; assert `grep -c "NOT OK"` is 0. Confirm both the new `must-land …` asserts AND every pre-existing assert are `ok -`.

- [ ] **Step 7: Update the contract `scripts/docket-status.md`.**
  - Usage synopsis: add `[--must-land]` alongside `[--board-only]`.
  - Flags table: add a row for `--must-land` — "With `--board-only`: run the board pass with an in-script bounded retry (3 attempts) on the sole retryable outcome `board inline changed push-failed`, re-syncing the metadata worktree between attempts, and map the result to the exit code. Exit 0 iff every board line is a terminal success (`board inline changed pushed`/`clean`, `board off`, `board github ok`); any other terminal line or retry exhaustion exits non-zero. Report-line vocabulary and flagless behavior are unchanged; `board_pass`'s fail-closed exit 2 propagates."
  - Behavior: add a short paragraph under step 3 (Board pass) noting `--must-land` wraps it.
  - Exit codes: add "non-zero — under `--must-land`, the board pass ended on a non-success terminal line or exhausted its 3 retries."

- [ ] **Step 8: Verify the contract-coverage test still passes** (a flag is not a new script, so no new `.md` is needed):

Run: `bash tests/test_script_contracts_coverage.sh 2>&1 | grep -c "NOT OK"`
Expected: `0`.

- [ ] **Step 9: Commit.**

```bash
git add scripts/docket-status.sh scripts/docket-status.md tests/test_docket_status.sh
git commit -m "feat(docket): add --must-land board-pass flag (retry+exit-code in docket-status.sh)"
```

---

## Task 2: Adopt `--must-land` at caller sites + compress the convention's board-refresh paragraph

**Files:**
- Modify: `skills/docket-convention/SKILL.md` (the "Board refresh on status writes" paragraph)
- Modify: `skills/docket-new-change/SKILL.md` (3 board-pass sites — the must-land ones)
- Modify: `skills/docket-groom-next/SKILL.md`
- Modify: `skills/docket-auto-groom/SKILL.md`
- Modify: `skills/docket-finalize-change/SKILL.md`
- Modify: `skills/docket-convention/references/terminal-close-out.md`

**Interfaces:**
- Consumes: the Task 1 `--must-land` flag.
- Produces: each must-land board-pass caller invokes `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh docket-status --board-only --must-land` and collapses its ~10-line report-line litany to one line plus posture: "non-zero exit → STOP and surface (abort-and-report)." Best-effort callers (all of `docket-implement-next`'s sites) keep the flagless call and log-and-continue — UNCHANGED.

**Invariants to preserve (verified by existing tests):**
- `test_skill_facade_wiring.sh`: `docket.sh docket-status --board-only` must remain present in exactly 7 files (auto-groom, finalize, groom-next, implement-next, new-change, convention SKILL.md, terminal-close-out.md). The `--must-land` suffix keeps the substring match, so the 7-count holds. Do NOT add the call to an 8th file or remove it from any.
- `test_board_refresh_on_transition.sh`: convention keeps `Board refresh on status writes`, `board-refresh.sh` (noun), `docket.sh docket-status --board-only`, `never on the exit code`; new-change keeps `must-land Board pass`; implement-next keeps `Best-effort board refresh` and ≥3 `run the Board pass (best-effort` clauses; finalize keeps `is **never** published`; every caller has no `--surfaces`/`BOARD_SURFACES`.
- `--must-land` is not `--surfaces`, so the "no surfaces value" asserts stay green.

- [ ] **Step 1: Record the baseline.** `bash tests/test_board_refresh_on_transition.sh; bash tests/test_skill_facade_wiring.sh` — both must be green before starting (they are on `origin/main`).

- [ ] **Step 2: Compress the convention's "Board refresh on status writes" paragraph** (spec §1, ~25 lines → ~8). Keep, verbatim in meaning: the single facade call, the "no surfaces value is ever passed by a skill" rule, the two posture sentences (must-land STOP/abort-and-report vs best-effort log-and-continue), `never on the exit code`, `must never trail the change files`, the `board-refresh.sh` noun mention. Add ONE sentence: "A must-land caller passes `--must-land`; the bounded retry and the exit-code mapping live in the script contract (`scripts/docket-status.md`)." MOVE the per-report-line enumeration (the closed-channel `push-failed`-is-the-only-retryable-line litany) OUT — it now lives in the script contract. Do NOT delete `must never trail the change files` (a `test_convention_extraction.sh` sentinel).

- [ ] **Step 3: Collapse each must-land caller site** to the facade call + one-line posture. In `docket-new-change` (×3 sites — keep the phrase `must-land Board pass`), `docket-groom-next`, `docket-auto-groom`, `docket-finalize-change`, and `terminal-close-out.md`: replace the inline retry-litany with:

> Run the must-land Board pass: `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh docket-status --board-only --must-land` — a non-zero exit means the board did not land; STOP and surface it (abort-and-report).

Leave `docket-implement-next`'s three board sites UNCHANGED (they are best-effort; `--must-land` does not apply — the spec's "implement-next where must-land" qualifier is vacuous for it).

- [ ] **Step 4: Run the board sentinels.**

Run: `bash tests/test_board_refresh_on_transition.sh 2>&1 | grep -c "NOT OK"` → expect `0`
Run: `bash tests/test_skill_facade_wiring.sh 2>&1 | grep -c "NOT OK"` → expect `0`
Then confirm the 7-count directly:
Run: `grep -rlF 'docket.sh docket-status --board-only' skills/ | wc -l` → expect `7`

- [ ] **Step 5: Behavior-neutrality read.** Re-read each collapsed site and confirm the NET caller behavior is unchanged: must-land sites still abort-and-report on failure; best-effort sites still log-and-continue. The retry moved into the script (Task 1), it was not removed.

- [ ] **Step 6: Commit.**

```bash
git add skills/docket-convention/SKILL.md skills/docket-new-change/SKILL.md \
  skills/docket-groom-next/SKILL.md skills/docket-auto-groom/SKILL.md \
  skills/docket-finalize-change/SKILL.md skills/docket-convention/references/terminal-close-out.md
git commit -m "refactor(docket): adopt --must-land at board-pass callers; compress convention board paragraph"
```

---

## Task 3: `docket-convention` re-slim — extract the Learnings ledger + tighten narration

**Files:**
- Create: `skills/docket-convention/references/learnings.md`
- Modify: `skills/docket-convention/SKILL.md`

**Interfaces:**
- Produces: `references/learnings.md` holding the Learnings-ledger DEEP mechanics; the convention's `### Learnings ledger` section reduced to a stub + loud blocking pointer.

**The extraction — what MOVES vs what STAYS (this is the crux):**

The convention's `### Learnings ledger` section must KEEP inline every phrase `test_learnings_ledger.sh` asserts against `$CONV`, so NO convention assert in that test needs re-anchoring. Enumerate and KEEP inline (one compressed line each is enough):
- the heading `### Learnings ledger`
- `<changes_dir>/learnings/` (names the findings dir)
- `is a **derived view**` (the index is a derived view)
- `will the agent know to search for this?` (the tiering criterion)
- `counts **active findings**` (the cap rule)
- `a no-op **read/write gate, never a` (the off-switch)  — write it so the sentence reads `... a no-op **read/write gate, never a purge**`
- `retained | candidate | promoted` (the promotion_state enum)
- `remains as a pointer stub` (the LEARNINGS.md pointer)
- `build-loop memory` (identity sentinel)
- the `**Readers:**` line naming `docket-implement-next`, `docket-groom-next`, `docket-auto-groom`
- the two-step read contract (index always; finding files on relevance)

MOVE to `references/learnings.md` (the heavy, off-common-path detail):
- the full finding-file frontmatter YAML block (~15 lines)
- the "Structure — index + detail" expansion, the harvest create-vs-extend-vs-never-merge rules
- the "Promotion — the shrink valve" long paragraph (landing in AGENTS.md, `promoted_to:`, dedup receipt)
- the "Capacity" consolidation detail and the "Off switch" byte-untouched detail

Add a loud blocking pointer under the heading: `Full mechanics — finding-file frontmatter, the harvest (create/extend), promotion, capacity, and the off-switch — are in [references/learnings.md](references/learnings.md); read it before harvesting, promoting, or curating findings.`

- [ ] **Step 1: Record baselines.** `wc -lw skills/docket-convention/SKILL.md`; `bash tests/test_learnings_ledger.sh 2>&1 | grep -c "NOT OK"` (expect 0); `bash tests/test_convention_extraction.sh 2>&1 | grep -c "NOT OK"` (expect 0); `bash tests/test_skill_facade_wiring.sh 2>&1 | grep -c "NOT OK"` (expect 0).

- [ ] **Step 2: Create `references/learnings.md`.** Start with a TOC (the file will exceed 100 L only if needed; keep ≤ ~150 L). Move the deep-mechanics prose listed above out of the convention into this file, verbatim where possible (the MOVE is a copy, not a rewrite — finding #20). Keep the convention as the single source for the read contract; the reference owns write/harvest/promotion/cap mechanics.

- [ ] **Step 3: Reduce the convention's `### Learnings ledger` section** to the inline stub (the KEEP list above) + the blocking pointer. Verify by eye that every KEEP phrase is present.

- [ ] **Step 4: Tighten the rest of the convention** (spec §2), preserving all `test_convention_extraction.sh` anchors and `test_skill_facade_wiring.sh` Layer-2 anchors:
  - **Bootstrap guard:** keep the 2×2 table, the `DOCKET`/`LIVE` probe definitions, `live planning surface`, `half-migrated`, and the verdict actions; cut surrounding prose to pointers at `scripts/docket-config.md`.
  - **Agent layer:** keep the load-bearing composition rules (foreground = actively block, never background-and-yield; a bare `completed` is not proof; never adopt a child's uncommitted files) tightened; push expansion detail to `references/agent-layer.md` (already pointed to). Keep the unique anchors `push-retry CAS loops alike`, `as its own Bash call`, `read the printed \`KEY=value\` block` EXACTLY ONCE each.
  - **Branch model / terminal-publish / hooks:** cut change-number archaeology to bare `(ADR-NNNN)` / `(change NNNN)` pointers. KEEP `only flow of metadata onto the code line`, `never gitignored`, `must never trail the change files` (sentinels).
  - **Skill layer:** keep the roles table; tighten bullets.
  - Byte-stable headings that MUST remain: `### Configuration`, `### Directory layout`, `### Change manifest`, `### ADR file`, `### Lifecycle`, `### Build-readiness`, `### Bootstrap guard`, `### Branch model`, `### Learnings ledger`, plus `## Convention (load first — blocking)` and the manifest/lifecycle sentinels (`proposed ──claim──▶`, `satisfied when it reaches`, `immutable once Accepted`, `zero-padded to 4 digits`, `PM-altitude proposal`).

- [ ] **Step 5: Run the anchor gate (the four sentinel suites).**

```bash
for t in test_convention_extraction test_learnings_ledger test_board_refresh_on_transition test_skill_facade_wiring; do
  echo "== $t =="; bash "tests/$t.sh" 2>&1 | grep "NOT OK" || echo "clean"
done
```
Expected: every suite `clean`. If any `NOT OK` names a MOVED phrase, do NOT re-gate it green blindly — decide per `test-premise-deleted-not-regated`: if the phrase legitimately moved to `references/learnings.md`, re-point that single assert's target from `$CONV` to the reference and keep its intent; if the phrase should have stayed inline (a KEEP-list item), restore it inline. Record any re-anchor in the commit message.

- [ ] **Step 6: Measure + behavior-neutrality read.** `wc -lw skills/docket-convention/SKILL.md references/learnings.md`. Read the diff top-to-bottom: every deleted convention sentence is narration, restated inline, moved to `references/learnings.md`, or covered by `scripts/docket-config.md`/`references/agent-layer.md`. Accept the landing size even if >200 L, provided the residual is load-bearing (finding #55).

- [ ] **Step 7: Commit.**

```bash
git add skills/docket-convention/SKILL.md skills/docket-convention/references/learnings.md
# include the re-anchored test only if Step 5 required it:
# git add tests/test_learnings_ledger.sh
git commit -m "refactor(docket): re-slim docket-convention; extract Learnings ledger to references/learnings.md"
```

---

## Task 4: Operating-skill re-slim (all nine)

**Files (each modified):** `skills/docket-status/SKILL.md`, `skills/docket-implement-next/SKILL.md`, `skills/docket-finalize-change/SKILL.md`, `skills/docket-adr/SKILL.md`, `skills/docket-groom-next/SKILL.md`, `skills/docket-new-change/SKILL.md`, `skills/docket-auto-groom/SKILL.md`, `skills/docket-brainstorm/SKILL.md` (docket-convention was Task 3; the board-pass callers' board paragraphs were Task 2).

**Levers (spec §3):**
- Re-compress each skill's Step-0 section to the ~3-line convention citation (the pattern the convention's *Step-0 preamble* already defines): "Run the convention's Step-0 preamble (load convention, run `docket.sh preflight` as its own Bash call, read the printed block, act on the verdict)." + the one line naming where this skill's writes land. Do NOT restate the preamble mechanics.
- Provenance cuts: change-number narration → bare `(change NNNN)` / `(ADR-NNNN)` pointers where the *why* is not load-bearing.
- **The small-model constraint holds for `docket-status`:** keep every step an explicit numbered imperative — cuts remove duplication/narration, never step explicitness.
- **`docket-finalize-change`:** the gate flow, the sign-off rule, and the abort-and-report sets survive verbatim in MEANING. Keep `is **never** published` and `Harvest learnings`, `already contains this change`, `learnings disabled — harvest skipped`, `docket.sh render-learnings-index` (test_learnings_ledger.sh sentinels).
- **`docket-status`:** keep `learnings disabled`, `render-learnings-index`, `over-cap`, `promotion-pending`, `Harvest learnings`, `docket-finalize-change` (test_learnings_ledger.sh).
- **`docket-implement-next`:** keep `Best-effort board refresh` + ≥3 `run the Board pass (best-effort` clauses; keep 2× `learnings/README.md` and 2× `learnings.enabled` (test asserts `-ge 2`).
- **`docket-groom-next` / `docket-auto-groom` / `docket-brainstorm`:** keep their `learnings/README.md` / `learnings.enabled` / `learnings (findings|index)` reads (test_learnings_ledger.sh (c)/(c')).
- Every operating skill keeps `## Convention (load first — blocking)` and names `docket-convention`, and must NOT contain any `test_convention_extraction.sh` convention sentinel.

- [ ] **Step 1: Baseline the suite.** Run every test once and record the pass line:
```bash
for f in tests/*.sh; do bash "$f" >/dev/null 2>&1 || echo "PRE-FAIL: $f"; done
```
Expected: no `PRE-FAIL` lines (clean tree on the branch tip after Tasks 1–3).

- [ ] **Step 2: Slim each skill** per the levers, one file at a time. After EACH file, run the sentinel suites that touch it (`test_convention_extraction`, `test_learnings_ledger`, `test_board_refresh_on_transition`, `test_skill_facade_wiring`) plus any skill-specific test (`test_finalize_gate`, `test_auto_groom`, `test_groom_recap`, `test_consultant_brainstorm`, `test_composition_wiring`, `test_skill_fork_dispatch`). Keep going only while green.

- [ ] **Step 3: Full-suite gate.**
```bash
notok=0; for f in tests/*.sh; do bash "$f" 2>&1 | grep -q "NOT OK" && { echo "NOT OK in $f"; notok=1; }; done; echo "notok=$notok"
```
Expected: `notok=0`.

- [ ] **Step 4: Measure.** `for f in skills/docket-*/SKILL.md; do printf '%s ' "$f"; wc -lw < "$f"; done` — record actuals for Task 6's budgets.

- [ ] **Step 5: Behavior-neutrality read** across all eight diffs (finding #55 + `foundational-test-discipline`: pair the sentinel greps with a read-for-meaning). Every deletion is narration/duplication/moved/contract-covered.

- [ ] **Step 6: Commit.**
```bash
git add skills/docket-status/SKILL.md skills/docket-implement-next/SKILL.md \
  skills/docket-finalize-change/SKILL.md skills/docket-adr/SKILL.md \
  skills/docket-groom-next/SKILL.md skills/docket-new-change/SKILL.md \
  skills/docket-auto-groom/SKILL.md skills/docket-brainstorm/SKILL.md
git commit -m "refactor(docket): re-slim the nine operating skills to post-slim targets"
```

---

## Task 5: References trim (`agent-layer.md`, `terminal-close-out.md`)

**Files:** `skills/docket-convention/references/agent-layer.md` (165 L), `skills/docket-convention/references/terminal-close-out.md` (135 L). (`github-board-mirror.md` is out of scope — already right-sized.)

- [ ] **Step 1:** Narration pass on each: cut change-number archaeology to `(ADR-NNNN)`/`(change NNNN)` pointers, keep every load-bearing rule. Each stays one level deep, ≤ ~150 L, with a leading TOC if it remains > 100 L.
- [ ] **Step 2:** `terminal-close-out.md` keeps its `docket.sh docket-status --board-only --must-land` call (added in Task 2) — do not drop it (the 7-file count).
- [ ] **Step 3: Gate.** `bash tests/test_closeout.sh; bash tests/test_skill_facade_wiring.sh; bash tests/test_composition_wiring.sh` → 0 `NOT OK` each. `grep -rlF 'docket.sh docket-status --board-only' skills/ | wc -l` → `7`.
- [ ] **Step 4:** `wc -lw` both files (record for Task 6). Behavior-neutrality read.
- [ ] **Step 5: Commit.**
```bash
git add skills/docket-convention/references/agent-layer.md skills/docket-convention/references/terminal-close-out.md
git commit -m "refactor(docket): trim agent-layer and terminal-close-out references"
```

---

## Task 6: Size-budget regrowth test

**Files:**
- Create: `tests/test_skill_size_budgets.sh`

**Design (spec §5):** glob every `skills/**/*.md` (auto-discovery — finding #12), hold a per-file budget table of max lines / max words set to post-slim actuals + ~10%, fail on exceed, AND fail if a globbed file has no budget entry (a new skill file can't go un-budgeted) or a table entry names a missing file. Prove non-vacuous (finding `foundational-test-discipline` / `guards-are-code`).

- [ ] **Step 1: Capture post-slim actuals.**
```bash
find skills -name '*.md' | sort | while read -r f; do printf '%s ' "$f"; wc -lw < "$f"; done
```
Record `L`/`W` for every file (SKILL.md ×9, references ×3 incl. new `learnings.md`, `github-board-mirror.md`, and every `*-template.md`).

- [ ] **Step 2: Write the test.** Budgets = `ceil(actual * 1.1)` per file. Use an ordinary indexed pair encoding (portable; no assoc-array dependency):

```bash
#!/usr/bin/env bash
# tests/test_skill_size_budgets.sh — regrowth guard (change 0085): every skills/**/*.md stays
# within a per-file line/word budget (~10% above the 0085 post-slim actuals). A future change that
# bloats a skill must slim elsewhere or consciously RAISE the budget in this table (an in-diff edit).
# Budgets are a DIRECTION made durable, not the slim's goal (learnings: size-target-is-direction).
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# BUDGETS: one row per tracked file — "<relpath> <maxLines> <maxWords>". Set from 0085 post-slim
# actuals + ~10%. To raise a budget, edit the number here in the same diff that grows the file.
BUDGETS="
skills/docket-convention/SKILL.md                        <L> <W>
skills/docket-convention/references/learnings.md         <L> <W>
skills/docket-convention/references/agent-layer.md       <L> <W>
skills/docket-convention/references/terminal-close-out.md <L> <W>
skills/docket-convention/github-board-mirror.md          <L> <W>
skills/docket-adr/SKILL.md                               <L> <W>
skills/docket-auto-groom/SKILL.md                        <L> <W>
skills/docket-brainstorm/SKILL.md                        <L> <W>
skills/docket-finalize-change/SKILL.md                   <L> <W>
skills/docket-groom-next/SKILL.md                        <L> <W>
skills/docket-implement-next/SKILL.md                    <L> <W>
skills/docket-new-change/SKILL.md                        <L> <W>
skills/docket-status/SKILL.md                            <L> <W>
<...every *-template.md the glob finds, each with its actual+10% budget...>
"

# Every tracked file is within budget.
budgeted=""
while read -r rel maxL maxW; do
  [ -n "$rel" ] || continue
  budgeted="$budgeted $rel"
  f="$REPO/$rel"
  assert "budgeted file exists: $rel" '[ -f "$f" ]'
  [ -f "$f" ] || continue
  L=$(wc -l < "$f" | tr -d ' '); W=$(wc -w < "$f" | tr -d ' ')
  assert "$rel within line budget ($L <= $maxL)" '[ "$L" -le "$maxL" ]'
  assert "$rel within word budget ($W <= $maxW)" '[ "$W" -le "$maxW" ]'
done <<EOF
$BUDGETS
EOF

# Completeness (auto-discovery guard, finding #12): every skills/**/*.md has a budget row, so a
# newly-added skill file can never go silently un-budgeted.
missing=""
while IFS= read -r f; do
  rel="${f#"$REPO"/}"
  printf '%s' "$budgeted" | grep -qF -- " $rel" || missing="$missing $rel"
done < <(find "$REPO/skills" -name '*.md' | sort)
assert "every skills/**/*.md has a budget row (unbudgeted:[$missing])" '[ -z "$missing" ]'

# Non-vacuity / mutation proof: the guard actually bites. A synthetic file 1 line over a 1-line
# budget must be caught by the same comparison.
probe="$(mktemp)"; printf 'a\nb\n' > "$probe"
pL=$(wc -l < "$probe" | tr -d ' ')
assert "the line-budget comparison is non-vacuous (2 > 1 is caught)" '[ ! "$pL" -le 1 ]'
rm -f "$probe"

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
```

Fill every `<L>`/`<W>` and the template rows from Step 1's actuals × 1.1 (round up).

- [ ] **Step 3: Run — expect PASS.** `bash tests/test_skill_size_budgets.sh 2>&1 | tail -1` → `PASS`.

- [ ] **Step 4: Mutation-prove the guard bites** (finding `guards-are-code`). Temporarily lower the convention's budget by 1 line, run, confirm a `NOT OK - skills/docket-convention/SKILL.md within line budget`, then restore. Also temporarily `touch skills/docket-adr/EXTRA.md`, run, confirm the completeness assert flags `unbudgeted:[…EXTRA.md]`, then `rm` it.

- [ ] **Step 5: Commit.**
```bash
git add tests/test_skill_size_budgets.sh
git commit -m "test(docket): add skill size-budget regrowth guard"
```

---

## Task 7: Whole-branch verification

**Files:** none (verification only; fixes land back in the relevant task's files if something breaks).

- [ ] **Step 1: Anchor grep-gate (spec §6.1).** No file under `skills/`, `agents/`, `scripts/`, or `tests/` references a section heading that no longer exists. Build the check from the surviving `##`/`###` headings:
```bash
# List headings that USED to exist on origin/main but no longer exist on the branch, then grep for
# stale references to them across the repo.
comm -23 \
  <(git show origin/main:skills/docket-convention/SKILL.md | grep -oE '^#{2,3} .*' | sort -u) \
  <(grep -hoE '^#{2,3} .*' skills/docket-convention/SKILL.md | sort -u) \
  > /tmp/removed-headings.txt
# For each removed heading, ensure nothing still cross-references it by name.
while IFS= read -r h; do
  name="${h#\#* }"
  grep -rIn -- "$name" skills/ agents/ scripts/ tests/ 2>/dev/null | grep -v 'origin/main' && echo "STALE REF: $name"
done < /tmp/removed-headings.txt
```
Expected: no `STALE REF` lines. Repeat the removed-heading probe for any other file whose headings changed (in practice only the convention and the two references are candidates). Kept headings must be byte-stable (already asserted by `test_convention_extraction.sh`).

- [ ] **Step 2: Full suite green.**
```bash
notok=0; for f in tests/*.sh; do
  if bash "$f" 2>&1 | grep -q "NOT OK"; then echo "NOT OK in $f"; notok=1; fi
done; echo "notok=$notok"
```
Expected: `notok=0`. This is the whole-branch review's mechanical half; the read-for-meaning half is Steps 4–5 of Tasks 3/4/5.

- [ ] **Step 3: `docket-status` smoke run.** Confirm the refactored orchestrator still runs end-to-end against a hermetic fixture (the existing `test_docket_status.sh` board-pass + full-pass fixtures cover this; a real-repo run is not possible from the feature worktree since metadata lives on `docket`). Re-run `bash tests/test_docket_status.sh` and confirm both `--board-only` and `--must-land` paths are exercised (they are, from Task 1).

- [ ] **Step 4: Re-read the learnings index at review** (`docs/changes/learnings/README.md` on the metadata branch — done at plan time; re-confirm the findings that bore on this change were honored: `size-target-is-direction`, `skill-extraction-and-stub-pointer`, `sole-channel`, `foundational-test-discipline`, `test-premise-deleted-not-regated`, `metadata-branch-invisible-to-suite`, `check-plumbing-auto-discovery`).

- [ ] **Step 5: Final size report** for the results/PR body:
```bash
for f in skills/docket-convention/SKILL.md skills/docket-convention/references/learnings.md \
  skills/docket-*/SKILL.md skills/docket-convention/references/agent-layer.md \
  skills/docket-convention/references/terminal-close-out.md; do
  printf '%s ' "$f"; wc -lw < "$f"
done
```

- [ ] **Step 6:** No commit of its own unless Step 1/2 surfaced a fix (which lands amended into the owning task's commit or a small follow-up commit).

---

## Self-Review

**1. Spec coverage:**
- §1 board-pass `--must-land` → Task 1 (flag+tests+contract) + Task 2 (caller adoption + convention paragraph). ✓
- §2 convention re-slim + learnings extraction → Task 3. ✓
- §3 all nine operating skills re-slimmed → Task 3 (convention) + Task 2 (board-caller paragraphs) + Task 4 (the other eight). ✓
- §4 references trim → Task 5. ✓
- §5 size-budget test → Task 6. ✓
- §6 verification (anchor grep-gate, behavior-neutrality, sentinel re-anchoring, smoke run, size asserts) → Task 7 + the per-task gates. ✓

**2. Placeholder scan:** the only intentional fill-ins are the `<L>`/`<W>` budget numbers in Task 6, which are DEFINED to be computed from post-slim actuals at build time (Task 6 Step 1) — the spec's sole open question. No other placeholders.

**3. Type consistency:** `board_classify` / `board_pass_must_land` / `MUST_LAND` names are used identically in Tasks 1–2 and the tests. The facade call string `docket.sh docket-status --board-only --must-land` is spelled identically in the callers (Task 2), the reference (Task 5), and the 7-file count assertions.
