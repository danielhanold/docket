# Self-Evidencing, Board-Independent docket-status Report — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the `docket-status` report always state what it did (`board off`, a backlog digest, `pass ok`) so a board-off repo never again hands an agent exit-0-plus-empty-stdout and triggers a `BOARD.md` hunt.

**Architecture:** Three seams. (1) `render-board.sh` gains `--format digest` — a second, line-oriented projection of the dependency/readiness pass it *already* runs; the default markdown path stays byte-identical. (2) `docket-status.sh` emits a positive `board off` line, gains an **ungated** `backlog_pass()` that pipes the digest through (report output, **not** a board surface — no git, no writes), and always closes with `pass ok`. (3) The prose (`skills/docket-status/SKILL.md`, `agents/docket-status.md`, and the two script contracts) goes board-neutral, so the Step-0 dispatch prompt stops promising a board the repo has disabled.

**Tech Stack:** Bash 3.2+/4 (`set -uo pipefail`, no `set -e`), the shared `scripts/lib/docket-frontmatter.sh` helpers (`field`, `int_field`, `resolve_deps`, `readiness`), and the repo's hand-rolled `assert` test harness (`bash tests/test_*.sh`, prints `ok - …` / `NOT OK - …`, exits non-zero on failure). No CI — **the suite is the gate**.

## Global Constraints

- **The digest is report output, not a board surface.** `backlog_pass()` runs regardless of `board_surfaces` and performs **zero** git operations — no write, no commit, no push, nothing to `BOARD.md`. This is what lets `board_surfaces: []` keep meaning "no board is rendered or committed."
- **`board-refresh.sh` is untouched.** It remains the sole gated *writer* of `BOARD.md`. The split: **board-refresh gates the surface, render-board serves the report.**
- **Default `render-board.sh` output must stay byte-identical.** `--format markdown` is the default; the existing golden byte-compare in `tests/test_render_board.sh` is the regression guard.
- **Stdout is never empty on a successful pass**, under any configuration. `pass ok` is printed only on completion, so it stays a reliable completion signal — a hard error still exits non-zero and prints no `pass ok`.
- **Best-effort backlog pass.** A `render-board.sh --format digest` failure logs to stderr, emits no digest lines, and never aborts the pass; `pass ok` is still printed.
- **Readiness tokens** (digest): `build-ready`, `needs-brainstorm`, `auto-groom-blocked`, `waiting-on-<N>-unbuilt`, `waiting-on-<N>-needs-merge`, or `-` when readiness does not apply (any non-`proposed` status).
- **Sentinel discipline** (from `LEARNINGS.md`): anchor each assert to the unique phrase its target clause owns — never a keyword OR-set; one assert owns exactly one clause; **mutation-test every new/changed sentinel** (delete or invert the clause it guards; the assert must flip to `NOT OK`). A grep sentinel must tokenize at the unit it claims to guard (the *invocation*, not the *line*).
- **Run the WHOLE suite** at the end, never only the tests this plan enumerates.

## File Structure

| File | Responsibility | Change |
|---|---|---|
| `scripts/render-board.sh` | pure renderer; gains the digest projection | Modify |
| `scripts/render-board.md` | its contract; documents `--format` | Modify |
| `scripts/docket-status.sh` | orchestrator; `board off`, `backlog_pass`, `pass ok` | Modify |
| `scripts/docket-status.md` | its contract; new output lines + 8-step sequence | Modify |
| `skills/docket-status/SKILL.md` | board-neutral description; board-off branch; thin-report rule; never-probe-`BOARD.md` prohibition | Modify |
| `agents/docket-status.md` | board-neutral wrapper description + body | Modify |
| `tests/test_render_board.sh` | digest coverage + byte-identical regression guard | Modify |
| `tests/test_docket_status.sh` | board-off/board-on/`--board-only`/degradation coverage; **rewrites two now-inverted assertions** | Modify |

---

## Task 1: `render-board.sh --format digest`

**Files:**
- Modify: `scripts/render-board.sh` (arg parsing ~lines 13-25; new digest branch inserted after the `ARC_COUNT` loop, ~line 67, i.e. **before** `printf '# Backlog\n\n'`)
- Test: `tests/test_render_board.sh`

**Interfaces:**
- Consumes: `field`, `int_field`, `readiness`, `resolve_deps`, and the `DEP_REASON` / `DEP_ON` globals from `scripts/lib/docket-frontmatter.sh` (already sourced at line 28); the script-local `rows_sorted`, `count_of`, `SECTION`, `ARC_COUNT`, `AFILES`, `ARCFILES` (all defined above the insertion point).
- Produces: `render-board.sh --changes-dir DIR --format digest` → stdout lines `backlog <status> <count>` then `change <id> <status> <readiness> <slug>`. `docket-status.sh`'s `backlog_pass()` (Task 2) is the only consumer. Unknown `--format` value → exit 2.

**Why this placement:** the digest needs `SECTION`/`ARC_COUNT`/`count_of`, which are all computed by line 67, and it must not perturb a single byte of the markdown emission below it. Inserting the branch there and `exit 0`ing keeps the default path physically untouched.

- [ ] **Step 1: Write the failing tests**

Append to `tests/test_render_board.sh`, immediately **before** the final `if [ "$fail" = 0 ]` line. The `$tmp` fixture (built at the top of that file) already spans every readiness band — `0002` build-ready, `0003` needs-brainstorm, `0004` auto-groom-blocked, `0005` waiting on a `proposed` dep (`not yet built`), `0006` waiting on an `implemented` dep (`needs your merge`) — so reuse it rather than building a second one.

```bash
# --- change 0069: --format digest (the line-oriented backlog projection) ---
# The digest is a SECOND projection of the dependency/readiness pass render-board.sh already
# runs — same source of truth as the board's Readiness cell, machine-parseable instead of prose.
# It is report output, never a board surface: docket-status.sh pipes it through without writing.

# (a) regression guard: the DEFAULT (no --format) output is byte-identical to the golden.
#     This is the load-bearing guarantee of the whole change — the digest must not perturb the
#     markdown path by a single byte. ("$rendered" was produced from the golden compare above.)
defaulted="$tmp/out-default.md"
bash "$SCRIPT" --changes-dir "$tmp" --repo o/r > "$defaulted" 2>/dev/null
assert "default output is byte-identical to the golden after --format lands" \
  'diff -u "$golden" "$defaulted"'

# (b) an explicit --format markdown is byte-identical to the default.
explicit="$tmp/out-markdown.md"
bash "$SCRIPT" --changes-dir "$tmp" --repo o/r --format markdown > "$explicit" 2>/dev/null
assert "--format markdown is byte-identical to the default" 'diff -u "$defaulted" "$explicit"'

# (c) the digest's exact shape, byte-compared to a hand-authored golden. Rollups first (fixed
#     status order, non-zero only), then one `change` line per ACTIVE change, ascending id.
digest_golden="$tmp/digest-golden.txt"
cat > "$digest_golden" <<'EOF'
backlog in-progress 1
backlog proposed 5
backlog blocked 1
backlog deferred 1
backlog implemented 1
backlog done 2
backlog killed 1
change 1 in-progress - alpha
change 2 proposed build-ready bravo
change 3 proposed needs-brainstorm charlie
change 4 proposed auto-groom-blocked delta
change 5 proposed waiting-on-3-unbuilt echo
change 6 proposed waiting-on-8-needs-merge foxtrot
change 7 blocked - golf
change 8 implemented - hotel
change 9 deferred - india
EOF
digest_out="$tmp/digest-out.txt"
bash "$SCRIPT" --changes-dir "$tmp" --repo o/r --format digest > "$digest_out" 2>/dev/null
drc=$?
assert "--format digest exits 0" '[ "$drc" -eq 0 ]'
assert "--format digest matches the digest golden byte-for-byte" 'diff -u "$digest_golden" "$digest_out"'

# (d) each readiness band individually (named asserts so a break names the band it broke).
assert "digest: build-ready token"            'grep -qxF "change 2 proposed build-ready bravo" "$digest_out"'
assert "digest: needs-brainstorm token"       'grep -qxF "change 3 proposed needs-brainstorm charlie" "$digest_out"'
assert "digest: auto-groom-blocked token"     'grep -qxF "change 4 proposed auto-groom-blocked delta" "$digest_out"'
assert "digest: waiting-on-N-unbuilt token"   'grep -qxF "change 5 proposed waiting-on-3-unbuilt echo" "$digest_out"'
assert "digest: waiting-on-N-needs-merge token" 'grep -qxF "change 6 proposed waiting-on-8-needs-merge foxtrot" "$digest_out"'
assert "digest: readiness is - for a non-proposed change" 'grep -qxF "change 1 in-progress - alpha" "$digest_out"'

# (e) the digest carries NO markdown board (it is a projection, not the board).
assert "digest emits no board markdown" '! grep -qF "# Backlog" "$digest_out"'
assert "digest emits no mermaid graph"  '! grep -qF "mermaid" "$digest_out"'

# (f) archive rollups only — archived changes get no `change` line (the digest is the ACTIVE backlog).
assert "digest: archived changes get no change line" '! grep -qE "^change (10|11|12) " "$digest_out"'

# (g) an unknown --format value is an argument error (exit 2), like any other bad flag.
bash "$SCRIPT" --changes-dir "$tmp" --format bogus >/dev/null 2>"$tmp/fmt-err.txt"
frc=$?
assert "unknown --format exits 2" '[ "$frc" -eq 2 ]'
assert "unknown --format names the flag on stderr" 'grep -qi "format" "$tmp/fmt-err.txt"'

# (h) the digest performs no git writes and needs no worktree (offline, pure).
assert "digest run leaves the fixture dir git-free" '[ ! -d "$tmp/.git" ]'
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `bash tests/test_render_board.sh`
Expected: FAIL. The `--format` runs exit 2 (`render-board: unknown argument: --format`), so `--format digest exits 0` reports `NOT OK`, the digest golden diff reports `NOT OK`, and the whole script exits non-zero. Assertions (a) and (b): (a) should already pass (the default path is untouched — that is the point of the guard); (b) fails.

- [ ] **Step 3: Add `--format` parsing + validation**

In `scripts/render-board.sh`, extend the header comment and the arg loop.

Replace the usage comment block (lines 7-9):

```bash
# Usage: render-board.sh --changes-dir DIR [--repo OWNER/REPO] [--format markdown|digest]
#   --repo builds pr: hyperlinks; defaults to deriving OWNER/REPO from the origin remote of
#   --changes-dir. Mock seam: GIT="${GIT:-git}".
#   --format markdown (default) emits the BOARD.md markdown; --format digest emits the
#   line-oriented backlog digest (`backlog <status> <count>` + `change <id> <status> <readiness>
#   <slug>`) — a second projection of the same dependency/readiness pass, consumed by
#   docket-status.sh's report. The digest is REPORT OUTPUT, NOT a board surface: it is never
#   persisted, committed, or written to BOARD.md.
```

Replace the variable declarations + arg loop (lines 13-23):

```bash
CHANGES_DIR=""
REPO=""
FORMAT="markdown"
while [ $# -gt 0 ]; do
  case "$1" in
    --changes-dir) CHANGES_DIR="$2"; shift ;;
    --repo) REPO="$2"; shift ;;
    --format) FORMAT="$2"; shift ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) printf 'render-board: unknown argument: %s\n' "$1" >&2; exit 2 ;;
  esac
  shift
done
```

Add the format validation immediately after the existing `--changes-dir` validation (after line 25's `[ -d "$CHANGES_DIR" ] || …`):

```bash
case "$FORMAT" in
  markdown|digest) : ;;
  *) printf 'render-board: unknown --format value: %s (expected markdown|digest)\n' "$FORMAT" >&2; exit 2 ;;
esac
```

- [ ] **Step 4: Add the digest projection**

In `scripts/render-board.sh`, insert this block **immediately after** the `ARC_COUNT` loop (currently line 67, the `for f in "${ARCFILES[@]}"; do st="$(field "$f" status)"; ARC_COUNT…done` line) and **immediately before** `printf '# Backlog\n\n'`. Nothing below it may be touched.

```bash
# --- digest projection (change 0069) --------------------------------------------------------
# A second, line-oriented projection of the SAME dependency-resolution/readiness pass the board
# renders from — so readiness has exactly one owner (readiness(), in lib/docket-frontmatter.sh)
# and the digest can never disagree with the board's Readiness cell. Emitted for the report;
# never persisted. Exits before the markdown emission, which stays byte-identical.
digest_readiness(){ # digest_readiness FILE ID STATUS -> machine-parseable readiness token
  local f="$1" id="$2" st="$3" tok
  # Readiness is only meaningful for a `proposed` change (see readiness() in the shared lib);
  # every other status reports `-` rather than a token that would not mean anything.
  [ "$st" = proposed ] || { printf '%s' '-'; return; }
  tok="$(readiness "$f")"
  case "$tok" in
    waiting)
      # readiness() collapses both flavors to `waiting`; the flavor + blocking id live in the
      # resolve_deps globals, exactly as the board's readiness_cell reads them.
      case "${DEP_REASON[$id]:-}" in
        "needs your merge") printf 'waiting-on-%s-needs-merge' "${DEP_ON[$id]}" ;;
        *)                  printf 'waiting-on-%s-unbuilt' "${DEP_ON[$id]}" ;;
      esac ;;
    *) printf '%s' "$tok" ;;
  esac
}

if [ "$FORMAT" = digest ]; then
  for st in in-progress proposed blocked deferred implemented done killed; do
    case "$st" in
      done|killed) n=${ARC_COUNT[$st]:-0} ;;
      *) n="$(count_of "$st")" ;;
    esac
    [ "$n" -gt 0 ] || continue
    printf 'backlog %s %s\n' "$st" "$n"
  done
  while IFS=$'\t' read -r id f; do
    [ -n "$id" ] || continue
    st="$(field "$f" status)"
    printf 'change %s %s %s %s\n' \
      "$id" "$st" "$(digest_readiness "$f" "$id" "$st")" "$(field "$f" slug)"
  done < <(
    for st in in-progress proposed blocked deferred implemented; do
      rows_sorted "$st"
    done | sort -t$'\t' -k1,1n
  )
  exit 0
fi
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `bash tests/test_render_board.sh`
Expected: PASS — every assertion `ok - …`, final line `PASS`, exit 0. In particular `default output is byte-identical to the golden after --format lands` and `rendered output matches the golden byte-for-byte` must both be `ok`.

- [ ] **Step 6: Mutation-test the byte-identical guard (prove it is not decoration)**

The regression guard is the whole safety net for "the markdown path is untouched." Prove it bites:

```bash
cd /Users/homer/dev/docket/.worktrees/status-report-self-evidencing
# Perturb the markdown path by one byte, then confirm the golden compares go RED.
sed -i.bak "s/printf '# Backlog\\\\n\\\\n'/printf '# Backlogg\\\\n\\\\n'/" scripts/render-board.sh
bash tests/test_render_board.sh | grep -c "NOT OK"   # expect >= 1 (the golden diffs fail)
mv scripts/render-board.sh.bak scripts/render-board.sh
bash tests/test_render_board.sh | tail -1            # expect: PASS
```
Expected: the mutated run prints at least one `NOT OK`; the restored run prints `PASS`. If the mutated run stays green, the guard is vacuous — stop and fix it.

- [ ] **Step 7: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/status-report-self-evidencing
git add scripts/render-board.sh tests/test_render_board.sh
git commit -m "feat(render-board): add --format digest, a line-oriented backlog projection

A second projection of the dependency/readiness pass render-board.sh already
runs, so readiness keeps exactly one owner. Default markdown output stays
byte-identical (guarded by the existing golden compare). Change 0069."
```

---

## Task 2: `docket-status.sh` — `board off`, the ungated backlog pass, and `pass ok`

**Files:**
- Modify: `scripts/docket-status.sh` (`board_pass` ~line 60; new `backlog_pass` after it; `main` ~line 370)
- Test: `tests/test_docket_status.sh` (**rewrites two now-inverted assertions** — see Step 1)

**Interfaces:**
- Consumes: `render-board.sh --format digest` from Task 1, invoked as `"$SCRIPTS_DIR"/render-board.sh` — the script's **documented mock seam** for chained scripts (`SCRIPTS_DIR="${SCRIPTS_DIR:-$SELF_DIR}"`, line 19), which is what makes the degradation test in Step 1(e) possible.
- Produces: new stdout lines `board off`, `backlog <status> <count>`, `change <id> <status> <readiness> <slug>`, `pass ok`. All are **additive** — no existing line shape changes.

**Two existing assertions in `tests/test_docket_status.sh` are inverted by this change and MUST be rewritten, not deleted:**

1. **Line ~20** — `assert "docket-status never calls render-board.sh directly (gated via board-refresh.sh)" '! grep -qF "/render-board.sh" "$SCRIPT"'`. This was change 0059's guard against an **ungated `BOARD.md` write path**. `backlog_pass` now legitimately calls `render-board.sh` — read-only, no write. Narrow the guard to what it actually protects: *every* `render-board.sh` invocation in the orchestrator must be the read-only `--format digest` projection; the inline **write** still routes through `board-refresh.sh`. Deleting the sentinel outright would re-open the 0059 hole.
2. **Line ~174** — `assert "board_pass empty-surfaces emits no board line" '! grep -qw "board" "$tmp/board-run3.txt"'`. This asserts precisely the silence this change exists to abolish. It becomes: the empty-surfaces run emits `board off`.

- [ ] **Step 1: Write the failing tests**

**(a)** Replace the 0059 sentinel block near the top of `tests/test_docket_status.sh` (the two asserts under the `--- inline-board wiring sentinel (change 0059) ---` comment) with a per-invocation guard:

```bash
# --- inline-board wiring sentinel (change 0059, narrowed by change 0069) ---
# 0059's rule: the inline BOARD.md *write* has exactly ONE gated path — board-refresh.sh — so the
# orchestrator must never render-and-write the board itself. 0069 adds a READ-ONLY consumer of the
# same renderer (`--format digest`, piped straight to the report, no file touched), so the guard
# can no longer be "never mention render-board.sh." It is narrowed to what it actually protects:
# every render-board.sh invocation in this script must be the read-only digest projection.
# Tokenized PER INVOCATION (not per line): a line carrying a gated and an ungated call side by
# side must not be whitewashed by the gated one. Comment lines are stripped first — prose that
# merely names the script is not an invocation.
assert "docket-status routes the inline board render through board-refresh.sh" \
  'grep -qF "/board-refresh.sh" "$SCRIPT"'

ungated_render=0
while IFS= read -r inv; do
  [ -n "$inv" ] || continue
  case "$inv" in
    *"--format digest"*) : ;;
    *) ungated_render=1; echo "  (ungated render-board.sh invocation: $inv)" ;;
  esac
done < <(grep -v '^[[:space:]]*#' "$SCRIPT" | grep -oE '[^;&|]*/render-board\.sh[^;&|]*' || true)
assert "every render-board.sh invocation in docket-status is the read-only --format digest" \
  '[ "$ungated_render" -eq 0 ]'
```

**(b)** Replace the inverted empty-surfaces assertion (line ~174):

```bash
assert "board_pass empty-surfaces run exits zero" '[ $rc -eq 0 ]'
# Change 0069: silence is not evidence. A board-off pass must SAY the board is off — an empty
# stdout is indistinguishable from "the script silently did nothing", which is the exact
# confusion that made an agent hunt for a BOARD.md its config forbids.
assert "board_pass empty-surfaces emits a positive 'board off' line" \
  'grep -qxF "board off" "$tmp/board-run3.txt"'
assert "board_pass empty-surfaces emits no inline board line" \
  '! grep -q "board inline" "$tmp/board-run3.txt"'
```

**(c)** Append the new coverage before the file's final `if [ "$fail" = 0 ]` line — the board-off contract (never-empty stdout, zero git writes):

```bash
# --- change 0069: the report is self-evidencing and board-independent ---
# A board-off repo (board_surfaces: []) must still get a complete, positive report: `board off`,
# the backlog digest, and `pass ok` — and must still perform ZERO git writes and leave no BOARD.md.
git_repo_setup "$tmp/boardoff-case"
git clone -q "$tmp/boardoff-case/origin.git" "$tmp/boardoff-case/work" 2>/dev/null
seed_changes_fixture "$tmp/boardoff-case/work"
# A second change so the digest has plurality (>=2 rows) and a non-trivial rollup.
cat > "$tmp/boardoff-case/work/docs/changes/active/0002-bravo.md" <<'EOF'
---
id: 2
slug: bravo
title: Bravo feature
status: proposed
priority: medium
depends_on: []
spec: docs/superpowers/specs/2026-06-10-bravo.md
EOF
git -C "$tmp/boardoff-case/work" -c user.email=t@t -c user.name=t add docs/changes
git -C "$tmp/boardoff-case/work" -c user.email=t@t -c user.name=t commit -q -m "seed board-off fixture"
git -C "$tmp/boardoff-case/work" push -q origin main
boardoff_head="$(git -C "$tmp/boardoff-case/work" rev-parse HEAD)"

write_board_fixture ""
(cd "$tmp/boardoff-case/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-board.sh" "$SCRIPT" --board-only >"$tmp/boardoff-out.txt" 2>"$tmp/boardoff-err.txt")
rc=$?
assert "board-off --board-only exits zero" '[ $rc -eq 0 ]'
assert "board-off stdout is NEVER empty" '[ -s "$tmp/boardoff-out.txt" ]'
assert "board-off emits 'board off'" 'grep -qxF "board off" "$tmp/boardoff-out.txt"'
assert "board-off emits the backlog rollup" 'grep -qxF "backlog proposed 1" "$tmp/boardoff-out.txt"'
assert "board-off emits a change line per active change" \
  'grep -qxF "change 1 in-progress - alpha" "$tmp/boardoff-out.txt" && grep -qxF "change 2 proposed build-ready bravo" "$tmp/boardoff-out.txt"'
assert "board-off closes with 'pass ok'" 'grep -qxF "pass ok" "$tmp/boardoff-out.txt"'
# The 0059 gate must not regress: no BOARD.md, no commit, no dirty tree.
assert "board-off wrote no BOARD.md" '[ ! -e "$tmp/boardoff-case/work/docs/changes/BOARD.md" ]'
assert "board-off made no commit" \
  '[ "$(git -C "$tmp/boardoff-case/work" rev-parse HEAD)" = "$boardoff_head" ]'
assert "board-off left the worktree clean" \
  '[ -z "$(git -C "$tmp/boardoff-case/work" status --porcelain)" ]'

# --- change 0069: board-ON still renders AND also reports the digest + pass ok ---
write_board_fixture inline
(cd "$tmp/board-case/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-board.sh" "$SCRIPT" --board-only >"$tmp/boardon-digest.txt" 2>/dev/null)
rc=$?
assert "board-on --board-only exits zero" '[ $rc -eq 0 ]'
assert "board-on still emits an inline board line" 'grep -q "board inline" "$tmp/boardon-digest.txt"'
assert "board-on ALSO emits the backlog digest" \
  'grep -qxF "change 1 in-progress - alpha" "$tmp/boardon-digest.txt"'
assert "board-on closes with 'pass ok'" 'grep -qxF "pass ok" "$tmp/boardon-digest.txt"'
assert "board-on never emits 'board off'" '! grep -qxF "board off" "$tmp/boardon-digest.txt"'

# --- change 0069: --board-only reports the backlog in BOTH configs (it is the "just show me
# the backlog" path; in a board-off repo it used to do literally nothing) ---
assert "--board-only reports the backlog with the board OFF" \
  'grep -qE "^change 1 " "$tmp/boardoff-out.txt"'
assert "--board-only reports the backlog with the board ON" \
  'grep -qE "^change 1 " "$tmp/boardon-digest.txt"'

# --- change 0069: the backlog pass is BEST-EFFORT (a failing digest never aborts the pass) ---
# Point the SCRIPTS_DIR mock seam at a stub render-board.sh that always fails.
mkdir -p "$tmp/stub-scripts"
cat > "$tmp/stub-scripts/render-board.sh" <<'EOF'
#!/usr/bin/env bash
echo "stub render-board: boom" >&2
exit 1
EOF
chmod +x "$tmp/stub-scripts/render-board.sh"
write_board_fixture ""
(cd "$tmp/boardoff-case/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-board.sh" SCRIPTS_DIR="$tmp/stub-scripts" "$SCRIPT" --board-only >"$tmp/degrade-out.txt" 2>"$tmp/degrade-err.txt")
rc=$?
assert "failing digest still exits 0 (best-effort)" '[ $rc -eq 0 ]'
assert "failing digest emits no digest lines" '! grep -qE "^(backlog|change) " "$tmp/degrade-out.txt"'
assert "failing digest still emits 'board off'" 'grep -qxF "board off" "$tmp/degrade-out.txt"'
assert "failing digest still closes with 'pass ok'" 'grep -qxF "pass ok" "$tmp/degrade-out.txt"'
assert "failing digest logs a diagnostic to stderr" '[ -s "$tmp/degrade-err.txt" ]'
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `bash tests/test_docket_status.sh`
Expected: FAIL — `NOT OK` for `board_pass empty-surfaces emits a positive 'board off' line`, every new `board off` / `backlog` / `change` / `pass ok` assertion, and the best-effort degradation block. The narrowed sentinel `every render-board.sh invocation … --format digest` should be `ok` *vacuously* right now (there are zero invocations); Step 5 mutation-tests it.

- [ ] **Step 3: Emit `board off` and add the ungated `backlog_pass`**

In `scripts/docket-status.sh`, replace the guard line inside `board_pass` (line 62):

```bash
board_pass(){
  local surfaces="${BOARD_SURFACES:-}"
  # Change 0069: silence is not evidence. With no surfaces configured the board pass is a
  # deliberate no-op — SAY SO, so "exit 0 + empty stdout" can never again be read as "the script
  # silently did nothing" and send an agent hunting for a BOARD.md the config forbids.
  [ -n "$surfaces" ] || { echo "board off"; return 0; }
```

Then add `backlog_pass` immediately after `board_pass_github` ends (after line 145), before the `detect_merged` comment block:

```bash
# backlog_pass — the backlog digest (change 0069). UNGATED: it runs regardless of
# BOARD_SURFACES, because the digest is REPORT OUTPUT, NOT A BOARD SURFACE. It persists
# nothing, commits nothing, pushes nothing, and never touches BOARD.md — which is exactly what
# lets `board_surfaces: []` keep meaning "no board is rendered or committed" while backlog state
# still reaches the report. Delegates to render-board.sh (--format digest), so readiness keeps
# exactly one owner and this orchestrator does not reimplement resolution. Best-effort: a render
# failure logs to stderr, emits no digest lines, and never aborts the pass.
backlog_pass(){
  local mw
  if [ "${DOCKET_MODE:-}" = docket ]; then mw="${METADATA_WORKTREE:-.docket}"; else mw="."; fi
  local cd_dir="$mw/$CHANGES_DIR"
  local out
  if ! out="$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$cd_dir" --format digest 2>&2)"; then
    echo "docket-status: backlog digest failed; continuing without it" >&2
    return 0
  fi
  [ -n "$out" ] && printf '%s\n' "$out"
  return 0
}
```

- [ ] **Step 4: Wire it into `main` — before the `--board-only` exit — and always close with `pass ok`**

In `scripts/docket-status.sh`, replace the body of `main` from `ensure_and_sync_worktree` to the end (lines 379-395):

```bash
  ensure_and_sync_worktree
  board_pass
  # Change 0069: the backlog pass runs BEFORE the --board-only early exit. --board-only is the
  # "just show me the backlog" path; in a board-off repo it used to do literally nothing and
  # return nothing. It now reports the backlog in every configuration.
  backlog_pass
  if [ "$BOARD_ONLY" = 1 ]; then
    echo "pass ok"
    exit 0
  fi

  local swept_count=0 line
  while IFS= read -r line; do
    [ -n "$line" ] || continue
    printf '%s\n' "$line"
    case "$line" in
      "swept "*) swept_count=$((swept_count + 1)) ;;
    esac
  done < <(detect_merged | sweep_execute)

  health_checks
  emit_judgment
  [ "$swept_count" -gt 0 ] && integration_sync
  # Change 0069: stdout is NEVER empty on a completed pass. `pass ok` means "the orchestrator ran
  # to completion" — a hard error exits non-zero above and never reaches this line, so it stays a
  # reliable completion signal. A thin report is the success case, not a symptom.
  echo "pass ok"
  exit 0
}
```

Also update the script's header usage comment (line 3) so it describes the report honestly:

```bash
# Sequences the shared docket scripts in one process; emits one line-oriented report on stdout.
# The report is self-evidencing: it always states what it did (`board off` when the board is
# disabled, the backlog digest, `pass ok` on completion), so stdout is never empty (change 0069).
```

- [ ] **Step 5: Run the tests to verify they pass — and mutation-test the narrowed sentinel**

Run: `bash tests/test_docket_status.sh`
Expected: PASS — final line `PASS`, exit 0.

Then prove the narrowed 0059 sentinel is not vacuous (it must still catch an **ungated** render, which is the 0059 hole):

```bash
cd /Users/homer/dev/docket/.worktrees/status-report-self-evidencing
cp scripts/docket-status.sh /tmp/ds.bak
# Mutation A: reintroduce an ungated render-board.sh call (a raw BOARD.md write path).
printf '%s\n' 'ungated_probe(){ "$SCRIPTS_DIR"/render-board.sh --changes-dir "$1" > "$1/BOARD.md"; }' >> scripts/docket-status.sh
bash tests/test_docket_status.sh 2>&1 | grep -F "every render-board.sh invocation"   # expect: NOT OK
cp /tmp/ds.bak scripts/docket-status.sh
# Mutation B: strip the `board off` echo — the empty-surfaces assertion must go red.
sed -i.bak 's/|| { echo "board off"; return 0; }/|| return 0/' scripts/docket-status.sh
bash tests/test_docket_status.sh 2>&1 | grep -F "positive 'board off' line"           # expect: NOT OK
cp /tmp/ds.bak scripts/docket-status.sh
bash tests/test_docket_status.sh | tail -1                                            # expect: PASS
rm -f scripts/docket-status.sh.bak /tmp/ds.bak
```
Expected: mutation A prints `NOT OK - every render-board.sh invocation …`; mutation B prints `NOT OK - board_pass empty-surfaces emits a positive 'board off' line`; the restored run prints `PASS`. **A mutation that leaves an assert GREEN is a defect — fix the assert, do not rationalize it.**

- [ ] **Step 6: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/status-report-self-evidencing
git add scripts/docket-status.sh tests/test_docket_status.sh
git commit -m "feat(docket-status): self-evidencing report — board off, backlog digest, pass ok

board_pass now says 'board off' instead of returning silently; a new UNGATED
backlog_pass emits the digest (report output, not a board surface — no git,
no BOARD.md) in both modes, before the --board-only early exit; main always
closes with 'pass ok', so stdout is never empty. Narrows 0059's sentinel from
'never call render-board.sh' to 'every call is the read-only --format digest',
keeping the single gated BOARD.md write path intact. Change 0069."
```

---

## Task 3: Prose — stop instructing the agent to ignore the evidence

The script changes give the agent evidence; **this task is the one that actually stops the hunt.** Left alone, the prose still promises a board and still tells the skill to summarize backlog state it can only get from `BOARD.md`.

**Files:**
- Modify: `skills/docket-status/SKILL.md` (frontmatter `description`; Overview; Final summary; a new thin-report rule + never-probe prohibition)
- Modify: `agents/docket-status.md` (frontmatter `description`; wrapper body)
- Modify: `scripts/docket-status.md` (output-contract table; the sequence, now 8 steps)
- Modify: `scripts/render-board.md` (documents `--format`)
- Test: `tests/test_docket_status.sh` (doc sentinels)

**Interfaces:**
- Consumes: the line shapes produced in Tasks 1-2 (`board off`, `backlog …`, `change …`, `pass ok`).
- Produces: no code. The `description` fields are what `docket-implement-next`'s Step-0 dispatch prompt paraphrases — that propagation is why they must go board-neutral.

- [ ] **Step 1: Write the failing doc sentinels**

Append to `tests/test_docket_status.sh` before the final `if [ "$fail" = 0 ]` line. Each assert anchors to the **unique phrase its target clause owns** — never a keyword set — and each owns exactly one clause.

```bash
# --- change 0069: prose is board-neutral and tells the agent a thin report is success ---
SKILL_MD="$REPO/skills/docket-status/SKILL.md"
AGENT_MD="$REPO/agents/docket-status.md"
STATUS_CONTRACT="$REPO/scripts/docket-status.md"
BOARD_CONTRACT="$REPO/scripts/render-board.md"

# The SKILL description and the wrapper description/body are what docket-implement-next's Step-0
# dispatch prompt paraphrases — a board promise there reaches the subagent verbatim. They must not
# promise a board the repo may have disabled. (Scoped to the frontmatter description LINE and the
# wrapper body: the SKILL's own reference section may still discuss BOARD.md legitimately.)
skill_desc="$(grep -m1 '^description:' "$SKILL_MD")"
agent_desc="$(grep -m1 '^description:' "$AGENT_MD")"
agent_body="$(sed -n '/^---$/,/^---$/!p' "$AGENT_MD")"
assert "SKILL description does not promise BOARD.md" '! printf "%s" "$skill_desc" | grep -qF "BOARD.md"'
assert "agent wrapper description does not promise BOARD.md" '! printf "%s" "$agent_desc" | grep -qF "BOARD.md"'
assert "agent wrapper body does not promise to refresh the board" \
  '! printf "%s" "$agent_body" | grep -qiF "refresh the board"'

# The thin-report rule and the never-probe prohibition — the two clauses that actually stop the
# hunt. Anchored on the unique phrase each owns.
assert "SKILL states a thin report is the success case" \
  'grep -qiF "a thin report is the success case" "$SKILL_MD"'
assert "SKILL prohibits probing BOARD.md" \
  'grep -qiF "never probe" "$SKILL_MD"'
assert "SKILL names the board-off report line" 'grep -qF "board off" "$SKILL_MD"'
assert "SKILL summarizes from the digest, not the board file" 'grep -qiF "digest" "$SKILL_MD"'

# The orchestrator contract documents every new line shape.
assert "status contract documents board off"  'grep -qF "board off" "$STATUS_CONTRACT"'
assert "status contract documents pass ok"    'grep -qF "pass ok" "$STATUS_CONTRACT"'
assert "status contract documents the backlog rollup line" \
  'grep -qF "backlog <status> <count>" "$STATUS_CONTRACT"'
assert "status contract documents the change digest line" \
  'grep -qF "change <id> <status> <readiness> <slug>" "$STATUS_CONTRACT"'
assert "status contract states the backlog pass is ungated" \
  'grep -qiF "ungated" "$STATUS_CONTRACT"'

# The renderer contract documents the new flag.
assert "render-board contract documents --format" 'grep -qF "--format" "$BOARD_CONTRACT"'
assert "render-board contract documents the digest projection" \
  'grep -qF "digest" "$BOARD_CONTRACT"'
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `bash tests/test_docket_status.sh`
Expected: FAIL — `NOT OK` for `SKILL description does not promise BOARD.md`, `agent wrapper description does not promise BOARD.md`, `agent wrapper body does not promise to refresh the board`, the thin-report/never-probe asserts, and every contract assert.

- [ ] **Step 3: Make `skills/docket-status/SKILL.md` board-neutral**

Replace the frontmatter `description` (line 3):

```yaml
description: Use when you want to see or refresh the docket backlog — what is proposed, in progress, blocked, implemented, or done — by refreshing docket state, sweeping merged changes to done, and running health checks for stale claims, broken spec/plan/results links, and dependency stalls.
```

Replace the Overview paragraph (line 12):

```markdown
`docket-status` gives you a queryable, up-to-date view of the backlog and keeps it clean. It has three jobs: refresh docket state (rendering each enabled board surface — `BOARD.md` when `inline` is enabled, nothing at all when `board_surfaces` is empty), sweep any `implemented` change whose PR merged into the archive, and run health checks that flag stale claims, broken links, and dependency stalls. The change files are the source of truth; any board is always generated output, never edited by hand. Since change 0058 all of this is sequenced by the deterministic orchestrator (contract: `scripts/docket-status.md`) — this skill's job is to invoke it, trust its exit code, surface its report, and apply the handful of judgment calls the script deliberately leaves in-model.
```

Insert this new section immediately after the *Run the orchestrator* section (after line 44, before *Judgment follow-ups*):

```markdown
## Read the report — it is the only channel you need

The report is **self-evidencing**: it always states what it did, so you never have to go looking for corroboration.

- **`board off`** — the repo sets `board_surfaces: []` and there is deliberately **no board**. This is a configuration, not a failure. Do not look for `BOARD.md`; it must not exist.
- **`backlog <status> <count>` + `change <id> <status> <readiness> <slug>`** — the backlog digest, emitted in **every** configuration. **This is your backlog-state channel.** Write the summary from these lines.
- **`pass ok`** — the orchestrator ran to completion. It is always the last line of a successful pass.

Two rules follow, and they are not optional:

- **A thin report is the success case, not a symptom.** An empty sweep, no health findings, and `board off` together mean a healthy, board-less repo. The pass is complete. Do **not** re-run the orchestrator, trace it, or investigate — there is nothing to find.
- **Never probe `BOARD.md`.** With the board off it must not exist; with the board on, summarize from the digest lines rather than opening the file. Reading, rendering, or hand-writing `BOARD.md` is never part of this skill's job — `board-refresh.sh` is its only writer.
```

Replace the *Final summary* section (line 57):

```markdown
## Final summary

Close with a short human-facing summary: backlog state (counts/highlights, read from the digest lines — never from the board file), what was swept to done (if anything), and any health-check findings or judgment flags raised above. When the `inline` board is enabled, point the user at `BOARD.md` (or the GitHub mirror, if enabled) for the full picture rather than reproducing it inline. When the report says `board off`, there is no board to point at — the digest-derived summary **is** the deliverable, and that is the intended, complete outcome.
```

- [ ] **Step 4: Make `agents/docket-status.md` board-neutral**

Replace its `description` (line 3) and body line (line 8):

```yaml
description: Use when you want to see or refresh the docket backlog — what is proposed, in progress, blocked, implemented, or done — by refreshing docket state, sweeping merged changes to done, and running health checks for stale claims, broken spec/plan/results links, and dependency stalls.
```

```markdown
Execute docket-status to refresh docket state and run the sweep + health checks. Follow the skill exactly. A thin report is the success case — do not go looking for artifacts the repo's configuration disables.
```

- [ ] **Step 5: Update `scripts/docket-status.md` (the orchestrator contract)**

Three edits.

**(i)** In *Behavior*, change the opening line `The pass runs as a fixed 7-step sequence:` to `The pass runs as a fixed 8-step sequence:`, and **renumber steps 4-7 to 5-8** (Batched sweep detection → 5, Sweep execution → 6, Health checks → 7, Integration sync → 8).

**(ii)** In step **3 (Board pass)**, fix the stale opening clause and the stale `inline` bullet — the current text still describes rendering into a `BOARD.md.tmp` file, which change **0059** superseded (the write decision moved into `board-refresh.sh`, and a suite sentinel already forbids this script from calling `render-board.sh` for anything but the read-only digest). Replace the step-3 preamble and its `inline` bullet with:

```markdown
**3. Board pass**, once per surface token in the space-separated `BOARD_SURFACES` config value.
**No surfaces configured emits a positive `board off` line** (change 0069) — never silence: an
empty stdout is indistinguishable from a script that did nothing, and that ambiguity is what sent
an agent hunting for a `BOARD.md` the configuration forbids.
- **inline** — Renders and writes the board through `board-refresh.sh` (change 0059), which owns
  the surface gate and the atomic, truncation-safe replace of `BOARD.md`; this script never calls
  `render-board.sh` to produce the board. A failed render leaves the existing `BOARD.md`
  untouched, logs to stderr, and is treated as success for sequencing purposes (best-effort). If
  `BOARD.md` is unchanged, nothing is committed (`board inline clean`). Otherwise it is `git
  add`ed and committed with message `docket: board refresh`, then pushed with up to 5 retry
  attempts: on push failure it rebase-pulls; if the rebase conflicts only on `BOARD.md`, it
  regenerates through the same gated helper (never a raw redirect) and continues the rebase; a
  rebase conflict on anything else, or a failed regeneration mid-rebase, aborts the rebase and
  stops retrying.
```

Then insert a new **step 4** between the board pass and the `--board-only` exit note:

```markdown
**4. Backlog pass — UNGATED.** Runs `render-board.sh --format digest` and passes its lines
through (`backlog <status> <count>` rollups, then one `change <id> <status> <readiness> <slug>`
line per active change). It runs **regardless of `board_surfaces`**, and **before** the
`--board-only` exit, because **the digest is report output, not a board surface**: it persists
nothing, commits nothing, pushes nothing, and never touches `BOARD.md`. That boundary is exactly
what lets `board_surfaces: []` keep meaning "no board is rendered or committed" while backlog
state still reaches the report — and it makes `--board-only` (the "just show me the backlog" path)
useful in a board-off repo, where it previously did nothing at all. Best-effort: a digest failure
logs to stderr, emits no digest lines, and never aborts the pass. Resolution is **not**
reimplemented here — `render-board.sh` stays the single owner of readiness.

If `--board-only` was passed, the process prints `pass ok` and exits 0 here — no sweep, health
checks, judgment, or integration sync.
```

(Delete the old standalone "If `--board-only` was passed, the process exits 0 here…" paragraph that followed step 3, since it is now folded in above.)

**(iii)** In the *Output contract* table, add these rows (keep `board off` adjacent to the other `board` rows; put `pass ok` last):

```markdown
| `board off` | `BOARD_SURFACES` is empty — the board is deliberately disabled (`board_surfaces: []`); no surface was rendered and nothing was committed. Positive evidence of a deliberate skip, never silence. |
| `backlog <status> <count>` | One rollup per non-zero status across the active + archived change files (from the ungated backlog pass). |
| `change <id> <status> <readiness> <slug>` | One line per **active** change. `<readiness>` is `build-ready`, `needs-brainstorm`, `auto-groom-blocked`, `waiting-on-<N>-unbuilt`, `waiting-on-<N>-needs-merge`, or `-` when readiness does not apply (any non-`proposed` status). |
| `pass ok` | The orchestrator ran to completion. Always the last line of a successful pass; **stdout is never empty**. A hard error exits non-zero and never prints it, so it is a reliable completion signal. |
```

And in *Exit codes*, extend the `0` entry:

```markdown
- `0` — the pass completed (and printed `pass ok` as its last line). Findings, `sweep-failed`,
  `sweep-skipped`, `board *-failed`, `board off`, and `judgment` lines on stdout are all normal,
  expected pass outcomes, not errors — **a thin report is the success case.**
```

- [ ] **Step 6: Update `scripts/render-board.md` (the renderer contract)**

In *Usage*, replace the synopsis and add the flag row:

```markdown
```
render-board.sh --changes-dir DIR [--repo OWNER/REPO] [--format markdown|digest]
```

| Flag | Required | Description |
|---|---|---|
| `--changes-dir DIR` | yes | Path to the changes directory (`active/` and `archive/` are children of this dir). |
| `--repo OWNER/REPO` | no | Used to build `pr:` hyperlinks in the **Implemented** column. Defaults to deriving `OWNER/REPO` from the `origin` remote of `--changes-dir` (best-effort, offline). Absent or non-GitHub remote: PR numbers render as bare `#N`. |
| `--format markdown\|digest` | no | Output projection. `markdown` (default) emits the board. `digest` emits the line-oriented backlog digest (change 0069). Any other value is an argument error (exit 2). |
```

Add this section immediately after the *Archive section* paragraph in *Behavior*:

```markdown
**Digest projection (`--format digest`).** A second projection of the **same**
dependency-resolution/readiness pass the board renders from — so `readiness()` keeps exactly one
owner and the digest can never disagree with the board's Readiness cell. Emits, in order: one
`backlog <status> <count>` line per non-zero status (fixed order: in-progress, proposed, blocked,
deferred, implemented, done, killed; `done`/`killed` counted from `archive/`), then one
`change <id> <status> <readiness> <slug>` line per **active** change, ascending by id. `<readiness>`
is `build-ready`, `needs-brainstorm`, `auto-groom-blocked`, `waiting-on-<N>-unbuilt`,
`waiting-on-<N>-needs-merge`, or `-` for any non-`proposed` status (where readiness does not apply).
No markdown, no mermaid graph, no archive table.

The digest is **report output, not a board surface**: `docket-status.sh` pipes it straight to its
report and never persists it. It is therefore emitted regardless of `board_surfaces` — which is
what lets `board_surfaces: []` keep meaning "no board is rendered or committed" while backlog state
still reaches the report. `board-refresh.sh` remains the sole gated writer of `BOARD.md`:
**board-refresh gates the surface, render-board serves the report.**
```

In *Exit codes*, update code `2`:

```markdown
| 2 | Missing or invalid argument (`--changes-dir` absent or not a directory; unknown flag; unknown `--format` value). |
```

In *Invariants*, add:

```markdown
- **Default output is byte-identical.** `--format` defaults to `markdown`; the digest is purely
  additive. The golden byte-compare in `tests/test_render_board.sh` is the regression guard.
```

- [ ] **Step 7: Run the tests to verify they pass**

Run: `bash tests/test_docket_status.sh && bash tests/test_render_board.sh`
Expected: PASS from both, exit 0.

- [ ] **Step 8: Mutation-test the doc sentinels (they are the easiest to ship vacuous)**

```bash
cd /Users/homer/dev/docket/.worktrees/status-report-self-evidencing
# Each must be satisfied by exactly ONE location — a double-guarded assert stays green when its
# real target is deleted. Verify the occurrence count before trusting the grep.
grep -c "a thin report is the success case" skills/docket-status/SKILL.md   # expect: 1
grep -c "never probe" skills/docket-status/SKILL.md                          # expect: 1
grep -c "BOARD.md" agents/docket-status.md                                   # expect: 0

# Mutation: strip the thin-report rule; the assert must flip to NOT OK.
cp skills/docket-status/SKILL.md /tmp/skill.bak
sed -i.bak '/a thin report is the success case/d' skills/docket-status/SKILL.md
bash tests/test_docket_status.sh 2>&1 | grep -F "thin report is the success case"  # expect: NOT OK
cp /tmp/skill.bak skills/docket-status/SKILL.md
rm -f skills/docket-status/SKILL.md.bak /tmp/skill.bak
bash tests/test_docket_status.sh | tail -1                                          # expect: PASS
```
Expected: the counts are exactly as annotated (a count > 1 means the assert is double-guarded — split it or re-anchor to a phrase the clause uniquely owns); the mutated run prints `NOT OK`; the restored run prints `PASS`.

- [ ] **Step 9: Run the WHOLE suite**

Not just the two tests this plan touched — an out-of-goal regression is exactly what the other tests exist to catch (`LEARNINGS.md`, #52/#54). Run every test in **one foreground call**:

```bash
cd /Users/homer/dev/docket/.worktrees/status-report-self-evidencing
fail=0
for t in tests/test_*.sh; do
  out="$(bash "$t" 2>&1)"; rc=$?
  if [ $rc -ne 0 ]; then fail=1; echo "=== FAIL: $t"; printf '%s\n' "$out" | grep -F "NOT OK"; else echo "ok   $t"; fi
done
echo "SUITE_EXIT=$fail"
```
Expected: `SUITE_EXIT=0`.

If anything is red: a RED suite is a **hypothesis, not a verdict** (`LEARNINGS.md`). Before calling it a regression, re-run the identical suite against unmodified `origin/main` in a scratch worktree and byte-compare the failing set — environment-bound failures (`origin/HEAD` unresolvable, umask-dependent modes, an exported `DOCKET_SCRIPTS_DIR` in the shell) are known here. Record any differential in the results file.

- [ ] **Step 10: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/status-report-self-evidencing
git add skills/docket-status/SKILL.md agents/docket-status.md scripts/docket-status.md scripts/render-board.md tests/test_docket_status.sh
git commit -m "docs(docket-status): board-neutral prose; a thin report is the success case

The SKILL/wrapper descriptions no longer promise a BOARD.md (that description
is what docket-implement-next's Step-0 dispatch prompt paraphrases, so the
promise propagated verbatim into the subagent). Adds the board-off branch, the
thin-report rule, and a prohibition on probing BOARD.md; documents board off /
backlog / change / pass ok in the orchestrator contract and --format digest in
the renderer contract. Also corrects the contract's step-3 prose, stale since
0059 moved the BOARD.md write into board-refresh.sh. Change 0069."
```

---

## Self-Review

**1. Spec coverage.**

| Spec requirement | Task |
|---|---|
| `render-board.sh --format digest`; markdown default byte-identical; unknown format → exit 2 | 1 (Steps 3-4; guarded Steps 1a/1g, mutation-tested 6) |
| Digest shape: `backlog <status> <count>` + `change <id> <status> <readiness> <slug>` | 1 (Step 4; golden-compared 1c) |
| Readiness tokens incl. both waiting flavors and `-` | 1 (`digest_readiness`; asserted per band, 1d) |
| `board_pass()` emits `board off` | 2 (Step 3; asserted + mutation-tested) |
| Ungated `backlog_pass()`, no git ops, both modes, before `--board-only` exit | 2 (Steps 3-4; asserted incl. no-commit/no-BOARD.md/clean-tree) |
| `main()` always closes with `pass ok`; stdout never empty | 2 (Step 4; asserted on both paths) |
| Backlog pass best-effort (failure → no lines, still `pass ok`, still exit 0) | 2 (Step 1e degradation block via the `SCRIPTS_DIR` seam) |
| `--board-only` reports the backlog in both configs | 2 (Step 1c/1d asserts) |
| `board-refresh.sh` untouched; single gated `BOARD.md` write path preserved | 2 (narrowed sentinel, mutation A) |
| SKILL board-off branch, thin-report rule, never-probe prohibition | 3 (Step 3) |
| SKILL `description` + `agents/docket-status.md` board-neutral | 3 (Steps 3-4) |
| `scripts/docket-status.md` documents the new lines + the backlog pass | 3 (Step 5) |
| `scripts/render-board.md` documents `--format` | 3 (Step 6) |

No gaps.

**2. Placeholder scan.** None: every step carries the literal bash/markdown to apply, the exact command to run, and its expected output.

**3. Type consistency.** `digest_readiness(FILE, ID, STATUS)` is defined and called only in Task 1. `backlog_pass()` (Task 2) invokes `"$SCRIPTS_DIR"/render-board.sh --changes-dir <dir> --format digest` — matching Task 1's flag name and values exactly (`markdown|digest`). The four line shapes (`board off`, `backlog …`, `change …`, `pass ok`) are spelled identically in the script, the tests, and both contracts.

**Two collisions this plan resolves explicitly** (either would otherwise hard-fail the suite the moment Task 2 lands):
- `tests/test_docket_status.sh` line ~20's 0059 sentinel `! grep -qF "/render-board.sh"` — **narrowed**, not deleted (Task 2, Step 1a; mutation A proves it still catches an ungated write path).
- `tests/test_docket_status.sh` line ~174's `board_pass empty-surfaces emits no board line` — **inverted by design** to assert `board off` (Task 2, Step 1b).
