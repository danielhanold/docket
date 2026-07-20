# Frontmatter Field-Domain Guard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Detect frontmatter values that are well-formed text but outside their field's domain — which today silently delete a change from every board surface, including the `ready` queue that drives autonomous selection — and surface them through the existing warn-only findings channel.

**Architecture:** Four parts, built bottom-up. Part 4 first (single-source the status vocabulary into `lib/docket-frontmatter.sh`) because the new `field-domain` check validates `status` against that array. Then part 3 (sanitize the findings channel) because it changes the shape of the change-id column every later finding uses. Then part 1 (`field-domain`) and part 2 (`board-row-dropped`, a suppressed backstop). Registration for each new check-id folds into the task that introduces it; a final task repairs two *pre-existing* registration drifts the reconcile found and runs the whole suite.

**Tech Stack:** Bash 3.2-compatible shell (`scripts/board-checks.sh`, `scripts/render-board.sh`, `scripts/lib/docket-frontmatter.sh`); hermetic shell test harness (`tests/test_board_checks.sh`, `tests/test_render_board.sh`) using `assert "<name>" '<expr>'` and temp-repo fixtures.

## Global Constraints

- **Warn-only, everywhere.** `board-checks.sh` exits 0 regardless of findings unless `--strict` is passed. `render-board.sh` must never exit non-zero — it sits on the must-land Board pass.
- **Git-only, offline.** No `gh`, no network, in both scripts and all fixtures.
- **Bash 3.2 / BSD-portable.** No `grep -P`. No `sed 's/\t/…/'` (BSD `sed` does not interpret `\t`) — use bash parameter expansion or a literal `$'\t'`.
- **`set -uo pipefail` is already in force** in both scripts. Never pipe a producer into an early-exiting consumer (`grep -q`, `head`) — capture into a variable first.
- **Untrusted input.** Every frontmatter value is untrusted input to these scripts. Validate by **shape** (character-class / membership), never by enumerating bad strings.
- **A guard is code.** Every assert added by this plan must be mutation-tested — strip the thing it guards, watch it redden. A mutation that leaves an assert green is a defect until proven otherwise.
- **Byte-identical board output.** Part 4 is a pure refactor: `tests/test_render_board.sh`'s golden byte-compare and idempotence asserts must pass **unchanged**. If the golden needs re-blessing, the substitution was wrong.
- **Whole-suite gate.** The build is not done until the whole suite is green, not only the two test files this plan touches:
  ```bash
  fail=0; for t in tests/test_*.sh; do bash "$t" >"/tmp/$(basename "$t").out" 2>&1 || { echo "FAIL: $t"; fail=1; }; done; echo "suite fail=$fail"
  ```

## File Structure

| File | Responsibility | Change |
|---|---|---|
| `scripts/lib/docket-frontmatter.sh` | Shared frontmatter accessors + dependency resolution | **Modify** — add the `DOCKET_STATUSES*` arrays (the single source of the status vocabulary) |
| `scripts/render-board.sh` | Deterministic board + digest renderer | **Modify** — replace four hand-written status lists with the shared arrays |
| `scripts/board-checks.sh` | The mechanical warn-only health checker | **Modify** — sanitize `emit`, add `field-domain` and `board-row-dropped`, update the header enumeration |
| `scripts/board-checks.md` | `board-checks.sh`'s contract | **Modify** — document both new checks; correct the `malformed-id` change-id-column sentence |
| `scripts/docket-status.md` | `docket-status.sh`'s contract | **Modify** — the closed `check <check-id>` enumeration |
| `tests/test_board_checks.sh` | Hermetic tests for the checker | **Modify** — fixtures for both new checks + the sanitization hole |
| `tests/test_render_board.sh` | Golden/idempotence tests for the renderer | **Modify** — vocabulary-correspondence + residual case-statement guards |

**Verified line numbers** (against `origin/main` at `2748ed9`; re-confirm before editing, they shift as tasks land):
`board-checks.sh:11-12` header enumeration · `:60` `emit()` · `:71-160` the FILES walk · `:74` the `malformed-id` emit ·
`render-board.sh:63-66` `emoji_for` · `:123` and `:193` (full seven) · `:137` and `:290` (active five) · `:205-208` `label_for_title` ·
`docket-status.md:344` the check-id row.

---

### Task 1: Single-source the status vocabulary (spec part 4)

Move the seven-name status list out of `render-board.sh`'s four hand-written iterations into `lib/docket-frontmatter.sh`, authored as the convention's two semantic groups. This is a **pure refactor** — the board's bytes must not move. It lands first because Task 3's `status` domain check reads the same array.

**Files:**
- Modify: `scripts/lib/docket-frontmatter.sh` (append after the accessors)
- Modify: `scripts/render-board.sh:123`, `:137`, `:193`, `:290`
- Test: `tests/test_render_board.sh`

**Interfaces:**
- Produces: three shell arrays, available to any script that sources `lib/docket-frontmatter.sh` —
  `DOCKET_STATUSES_ACTIVE=(in-progress proposed blocked deferred implemented)`,
  `DOCKET_STATUSES_TERMINAL=(done killed)`,
  `DOCKET_STATUSES=("${DOCKET_STATUSES_ACTIVE[@]}" "${DOCKET_STATUSES_TERMINAL[@]}")`.
  Task 3 consumes `DOCKET_STATUSES` for the `status` domain; Task 4 consumes it for the drop invariant.

- [ ] **Step 1: Write the failing tests**

Append to `tests/test_render_board.sh`, immediately before the final `if [ "$fail" = 0 ]` block:

```bash
# ============ status vocabulary is single-sourced (change 0104, spec part 4) ============
# The seven-name list used to be written out at four sites in this script, in two shapes. It now
# lives in lib/docket-frontmatter.sh. Two guard families:
#   (a) PRODUCER-anchored: the four iteration sites actually reference the arrays, and no
#       hand-written status list survives anywhere in the renderer.
#   (b) MIRROR correspondence: emoji_for / label_for_title are a PARALLEL representation of the
#       same vocabulary that the array cannot unify (a case statement is the right shape for a
#       mapping). They fail by printing NOTHING for an unknown status — the same silent-emptiness
#       class this change exists to kill. Set EQUALITY pins both directions at once: a name added
#       to the array without a case arm reddens, AND a phantom case arm for a retired status
#       reddens. Anything weaker is one-directional. See learnings: correspondence-guard-runs-one-way.
LIB="$REPO/scripts/lib/docket-frontmatter.sh"
# shellcheck source=/dev/null
source "$LIB"

# (a) producer-anchored substitution completeness
n_all="$(grep -cF 'for st in "${DOCKET_STATUSES[@]}"' "$SCRIPT")"
n_active="$(grep -cF 'for st in "${DOCKET_STATUSES_ACTIVE[@]}"' "$SCRIPT")"
n_literal="$(grep -cE '^[[:space:]]*for st in [a-z]' "$SCRIPT")"
assert "render-board.sh iterates DOCKET_STATUSES at both full-vocabulary sites" '[ "$n_all" = 2 ]'
assert "render-board.sh iterates DOCKET_STATUSES_ACTIVE at both active-only sites" '[ "$n_active" = 2 ]'
assert "no hand-written status list survives in render-board.sh" '[ "$n_literal" = 0 ]'

# (b) the arrays themselves: composition and order (the golden compares bytes, this names the rule)
assert "DOCKET_STATUSES is ACTIVE ++ TERMINAL, in that order" \
  '[ "${DOCKET_STATUSES[*]}" = "${DOCKET_STATUSES_ACTIVE[*]} ${DOCKET_STATUSES_TERMINAL[*]}" ]'
assert "DOCKET_STATUSES holds the convention's seven lifecycle statuses" '[ "${#DOCKET_STATUSES[@]}" = 7 ]'
assert "DOCKET_STATUSES_ACTIVE holds the five non-terminal statuses" '[ "${#DOCKET_STATUSES_ACTIVE[@]}" = 5 ]'

# case_labels FUNC — the case arms of a one-line-header case statement in render-board.sh, sorted.
# The `exit` on esac bounds the range to this function's own body.
case_labels(){
  awk -v fn="$1" '
    $0 ~ "^" fn "\\(\\)\\{ case" { inb = 1 }
    inb { print }
    inb && /esac/ { exit }
  ' "$SCRIPT" | grep -oE '[a-z][a-z-]*\)' | tr -d ')' | sort -u
}
emoji_labels="$(case_labels emoji_for)"
title_labels="$(case_labels label_for_title)"
# Non-vacuity: a tokenizer that parses nothing passes everything. Assert the COUNT it found before
# trusting the comparison below (learnings: guards-are-code).
assert "case_labels extracted all 7 emoji_for arms (tokenizer sees the function)" \
  '[ "$(printf "%s\n" "$emoji_labels" | grep -c .)" = 7 ]'
assert "case_labels extracted all 5 label_for_title arms (tokenizer sees the function)" \
  '[ "$(printf "%s\n" "$title_labels" | grep -c .)" = 5 ]'
exp_all="$(printf '%s\n' "${DOCKET_STATUSES[@]}" | sort -u)"
exp_active="$(printf '%s\n' "${DOCKET_STATUSES_ACTIVE[@]}" | sort -u)"
assert "emoji_for's case arms are EXACTLY DOCKET_STATUSES (both directions)" \
  '[ "$emoji_labels" = "$exp_all" ]'
assert "label_for_title's case arms are EXACTLY DOCKET_STATUSES_ACTIVE (both directions)" \
  '[ "$title_labels" = "$exp_active" ]'
```

- [ ] **Step 2: Run the tests to verify they fail**

```bash
cd /Users/homer/dev/docket/.worktrees/guard-frontmatter-field-domain-violations-that-silently-drop
bash tests/test_render_board.sh 2>&1 | grep -E 'NOT OK|^FAIL|^PASS'
```

Expected: `NOT OK` for the three producer-anchored asserts (`n_all`/`n_active` are 0, `n_literal` is 4) and for the three array asserts (the arrays do not exist yet, so `${#DOCKET_STATUSES[@]}` is 0 under `set -u`… if sourcing aborts the test run instead, that is also a valid failure — the arrays are genuinely absent). The two `case_labels` count asserts should already pass; the two set-equality asserts fail because `exp_all`/`exp_active` are empty.

- [ ] **Step 3: Add the arrays to the shared lib**

Append to `scripts/lib/docket-frontmatter.sh`, after the `readiness`/`finalize_blocked` function definitions (end of file):

```bash
# --- status vocabulary (change 0104) ----------------------------------------------------------
# The seven lifecycle statuses, authored as the convention's two semantic groups: `active/` holds
# every non-terminal status, `archive/` holds the two terminal outcomes. DOCKET_STATUSES is the
# concatenation, in the renderer's display order — the order IS the contract (BOARD.md's section
# order and the digest's `backlog` rollup order both come from iterating it), so never reorder
# these without re-blessing tests/test_render_board.sh's golden.
#
# Single source for render-board.sh's section iteration AND board-checks.sh's `status` field-domain
# check. Duplicating the list makes the checker and the renderer drift in two directions and only
# one of them is detectable: a status added to the renderer but not the checker makes field-domain
# fire a FALSE finding on every file carrying it (and suppresses the board-row-dropped backstop,
# which would otherwise be the thing that noticed), while the reverse direction is caught.
DOCKET_STATUSES_ACTIVE=(in-progress proposed blocked deferred implemented)
DOCKET_STATUSES_TERMINAL=(done killed)
DOCKET_STATUSES=("${DOCKET_STATUSES_ACTIVE[@]}" "${DOCKET_STATUSES_TERMINAL[@]}")
```

- [ ] **Step 4: Substitute the four iteration sites in `render-board.sh`**

`render-board.sh` already sources the lib at `:45`, so the arrays are in scope at every site.

At `:123` (the digest's `backlog` rollup) and `:193` (the markdown count line) replace:

```bash
for st in in-progress proposed blocked deferred implemented done killed; do
```

with:

```bash
for st in "${DOCKET_STATUSES[@]}"; do
```

At `:137` (the digest's `change` lines) and `:290` (the mermaid graph's active nodes) replace:

```bash
    for st in in-progress proposed blocked deferred implemented; do
```

with:

```bash
    for st in "${DOCKET_STATUSES_ACTIVE[@]}"; do
```

Preserve each site's existing indentation exactly — `:123` and `:193` are at two-space and zero-space indent respectively, `:137` and `:290` are inside process substitutions at four-space and two-space indent. Change nothing else on those lines.

- [ ] **Step 5: Run the tests to verify they pass — including the golden**

```bash
bash tests/test_render_board.sh 2>&1 | grep -E 'NOT OK|^FAIL|^PASS'
```

Expected: `PASS`, with **zero** `NOT OK`. In particular `rendered output matches the golden byte-for-byte` and `render is idempotent` must still pass — they are the proof the refactor moved no bytes. If the golden fails, the substitution changed an iteration order: fix the arrays, never the golden.

- [ ] **Step 6: Mutation-test both directions of the mirror guard**

Prove each new assert is load-bearing. Run each mutation, confirm the named assert reddens, then revert.

```bash
# Direction 1 — a vocabulary name with no case arm must redden.
cp scripts/lib/docket-frontmatter.sh /tmp/lib.bak
sed -i.tmp 's/^DOCKET_STATUSES_TERMINAL=(done killed)$/DOCKET_STATUSES_TERMINAL=(done killed retired)/' scripts/lib/docket-frontmatter.sh
bash tests/test_render_board.sh 2>&1 | grep -E 'NOT OK'   # expect: emoji_for set-equality NOT OK
cp /tmp/lib.bak scripts/lib/docket-frontmatter.sh; rm -f scripts/lib/docket-frontmatter.sh.tmp

# Direction 2 — a phantom case arm for a status not in the vocabulary must redden.
cp scripts/render-board.sh /tmp/rb.bak
sed -i.tmp "s/  deferred) printf '⚪';;/  retired) printf '🧊';; deferred) printf '⚪';;/" scripts/render-board.sh
bash tests/test_render_board.sh 2>&1 | grep -E 'NOT OK'   # expect: emoji_for set-equality + arm-count NOT OK
cp /tmp/rb.bak scripts/render-board.sh; rm -f scripts/render-board.sh.tmp

# Direction 3 — a reverted substitution must redden the producer asserts.
cp scripts/render-board.sh /tmp/rb.bak
sed -i.tmp '193s/.*/for st in in-progress proposed blocked deferred implemented done killed; do/' scripts/render-board.sh
bash tests/test_render_board.sh 2>&1 | grep -E 'NOT OK'   # expect: n_all + n_literal NOT OK
cp /tmp/rb.bak scripts/render-board.sh; rm -f scripts/render-board.sh.tmp

bash tests/test_render_board.sh 2>&1 | tail -1            # expect: PASS (fully reverted)
```

- [ ] **Step 7: Commit**

```bash
git add scripts/lib/docket-frontmatter.sh scripts/render-board.sh tests/test_render_board.sh
git commit -m "refactor(0104): single-source the status vocabulary into the shared lib

The seven-name list was written out at four sites in render-board.sh in two
shapes. It now lives in lib/docket-frontmatter.sh as DOCKET_STATUSES_ACTIVE /
DOCKET_STATUSES_TERMINAL / DOCKET_STATUSES, so the upcoming field-domain check
and the renderer cannot drift apart. Pure refactor: the golden byte-compare and
idempotence asserts pass unchanged.

Pins the residual parallel representation (emoji_for / label_for_title) by set
equality against the arrays, in both directions."
```

---

### Task 2: Sanitize the findings channel (spec part 3)

`emit malformed-id "$raw" …` puts an **untrusted frontmatter value in the TAB-separated change-id column**, and `docket-status.sh:627` reads findings back with `IFS=$'\t' read -r check_id change_id message`. An interior TAB in `id:` shifts the message into the wrong field. `field()` truncates at the first newline and strips trailing whitespace, but an interior TAB survives it. The guard's own reporting channel is injectable by the exact input class this change exists to catch.

**Files:**
- Modify: `scripts/board-checks.sh:58-60` (add helpers), `:71-76` (the walk head)
- Test: `tests/test_board_checks.sh`

**Interfaces:**
- Consumes: nothing from Task 1.
- Produces: `sanitize VALUE` (escapes TAB→`\t`, CR→`\r`), `padded_id_from_file FILE` (the 4-digit id from the basename, `?` when the filename yields none), and the per-file `cid` convention — the change-id column's value for every finding about that file. Tasks 3 and 4 emit with `$cid`.

**Design note — a deliberate deviation from the spec's letter, recorded for the ADR step.**
The spec says the change-id column "uses the filename-derived padded id, falling back to `?`", stated as a blanket rule. Applying it to *every* check would change `broken-spec` / `dep-cycle` / `stale-in-progress` / … from `2` to `0002`, silently breaking the report format and ~15 existing asserts — a behavior change the spec never argues for. The stated **rationale** is that the column must never carry a value that can shift a field. An `int_field`-validated id is `^[0-9]+$` and provably cannot. So:

- `emit` sanitizes **both** columns unconditionally (defense in depth, and it covers the message column's quoted values too).
- The filename-derived padded id is used **exactly where a raw frontmatter value would otherwise appear** — the `malformed-id` site, and Task 4's drop finding for a file with no usable id.
- Every check that has a validated `$id` keeps emitting `$id`.

This satisfies the spec's rationale without the unstated format break. Flag it at the ADR step in Step 6 of Task 5.

- [ ] **Step 1: Write the failing tests**

Replace the existing `malformed-id` section of `tests/test_board_checks.sh` (currently at `:413-417`) with:

```bash
# ============ malformed-id + findings-channel sanitization (change 0104, spec part 3) ============
# The change-id column is the field docket-status.sh splits on
# (`IFS=$'\t' read -r check_id change_id message`). It must NEVER carry a raw frontmatter value.
# Pre-0104 the malformed-id emit put `$raw` there verbatim, so a TAB in `id:` shifted the message
# into the wrong field — the guard's own channel injectable by the input class it exists to catch.
read -r work origin <<<"$(new_repo)"
printf -- '---\nid: abc\nslug: bad\ntitle: Bad\nstatus: proposed\npriority: low\ndepends_on: []\n---\n' > "$work/docs/changes/active/0001-bad.md"
out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$work/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
# The check still fires — but keyed on the FILENAME-derived id, not the raw value. (This assert
# replaces the pre-0104 `has_finding "$out" malformed-id abc`: what the block GUARDS is "a
# non-integer id is flagged", and that is preserved; only the column the raw value lands in moved.)
assert "malformed-id fires on a non-integer id, keyed on the filename-derived id" \
  'has_finding "$out" malformed-id 0001'
assert "malformed-id no longer keys the change-id column on the raw frontmatter value" \
  '! has_finding "$out" malformed-id abc'
assert "the raw value survives in the MESSAGE column (diagnosis is not lost)" \
  'printf "%s" "$out" | grep -qF "non-integer id '"'"'abc'"'"'"'

# TAB injection: an interior TAB in id: must not shift the message into the change-id field.
read -r work2 _ <<<"$(new_repo)"
printf -- '---\nid: 4\tEVIL\nslug: tabby\ntitle: Tabby\nstatus: proposed\npriority: low\ndepends_on: []\n---\n' > "$work2/docs/changes/active/0002-tabby.md"
tout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$work2/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
# Read the finding back exactly the way docket-status.sh does; all three columns must survive.
IFS=$'\t' read -r t_check t_id t_msg <<<"$(printf '%s' "$tout" | grep '^malformed-id')"
assert "TAB-in-id: check_id column survives the caller's IFS split" '[ "$t_check" = "malformed-id" ]'
assert "TAB-in-id: change-id column is the filename id, not a fragment of the raw value" '[ "$t_id" = "0002" ]'
assert "TAB-in-id: the message column is non-empty (not shifted into the id field)" '[ -n "$t_msg" ]'
assert "TAB-in-id: the embedded TAB is escaped to a visible \\t, not passed through raw" \
  'printf "%s" "$t_msg" | grep -qF "4\\tEVIL"'

# An archive filename (<date>-<id>-<slug>.md) still yields its id.
read -r work3 _ <<<"$(new_repo)"
printf -- '---\nid: xyz\nslug: arch\ntitle: Arch\nstatus: done\npriority: low\ndepends_on: []\n---\n' > "$work3/docs/changes/archive/2026-06-16-0012-arch.md"
aout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$work3/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "archive filenames yield their padded id for the change-id column" 'has_finding "$aout" malformed-id 0012'

# A filename with no derivable id falls back to `?` rather than emitting an empty column.
read -r work4 _ <<<"$(new_repo)"
printf -- '---\nid: nope\nslug: weird\ntitle: Weird\nstatus: proposed\npriority: low\ndepends_on: []\n---\n' > "$work4/docs/changes/active/no-leading-id.md"
wout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$work4/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "an id-less filename falls back to '?' in the change-id column" 'has_finding "$wout" malformed-id "?"'
```

- [ ] **Step 2: Run the tests to verify they fail**

```bash
bash tests/test_board_checks.sh 2>&1 | grep -E 'NOT OK|^PASS|^FAIL'
```

Expected: `NOT OK` for the filename-keyed asserts (the column still holds `abc`), for the TAB-split asserts (`t_id` is `4`, the message is shifted), and for the archive/fallback asserts.

- [ ] **Step 3: Add the helpers and rewire the walk head**

In `scripts/board-checks.sh`, replace the `emit` definition block (currently `:58-60`):

```bash
declare -A ID_ACTIVE ID_EXISTS                # id -> 1; populated in the FILES walk below
FINDINGS=""                            # accumulate "<check>\t<id>\t<msg>\n"; sorted + printed at the end
emit(){ FINDINGS+="$1"$'\t'"$2"$'\t'"$3"$'\n'; }
```

with:

```bash
declare -A ID_ACTIVE ID_EXISTS                # id -> 1; populated in the FILES walk below
FINDINGS=""                            # accumulate "<check>\t<id>\t<msg>\n"; sorted + printed at the end

# sanitize VALUE — render TAB and CR as the visible two-character escapes \t and \r (change 0104).
# Findings are TAB-separated and the caller splits them with `IFS=$'\t' read -r check_id change_id
# message` (docket-status.sh:627), so an interior TAB in ANY embedded value shifts every later
# field. field() truncates at the first newline and strips trailing whitespace, but an interior TAB
# survives it — these values are untrusted frontmatter, not program constants. Pure bash parameter
# expansion: BSD sed does not interpret \t in a pattern, so a sed form would be silently wrong.
sanitize(){ local v="$1"; v="${v//$'\t'/\\t}"; v="${v//$'\r'/\\r}"; printf '%s' "$v"; }

emit(){ FINDINGS+="$1"$'\t'"$(sanitize "$2")"$'\t'"$(sanitize "$3")"$'\n'; }

# padded_id_from_file FILE — the zero-padded id encoded in the BASENAME (`0104-slug.md`, or
# `2026-07-20-0104-slug.md` in archive/), or `?` when the filename yields none. Used for the
# change-id column whenever the frontmatter id is unusable: that column is what the caller splits
# on, so it must never carry a raw frontmatter value. A validated int_field id is ^[0-9]+$ and
# cannot shift a field, so checks that have one keep emitting it verbatim (unpadded, as before).
padded_id_from_file(){
  local b; b="$(basename "$1")"
  b="${b#[0-9][0-9][0-9][0-9]-[0-9][0-9]-[0-9][0-9]-}"   # strip an archive/ date prefix, if any
  case "$b" in
    [0-9][0-9][0-9][0-9]-*) printf '%s' "${b%%-*}" ;;
    *) printf '?' ;;
  esac
}
```

Then in the FILES walk, replace the head (currently `:71-76`):

```bash
for f in "${FILES[@]}"; do
  raw="$(field "$f" id)"; id="$(int_field "$f" id)"
  if [ -z "$id" ]; then
    [ -n "$raw" ] && emit malformed-id "$raw" "non-integer id '$raw' in $(basename "$f")"
    continue
  fi
```

with:

```bash
for f in "${FILES[@]}"; do
  raw="$(field "$f" id)"; id="$(int_field "$f" id)"
  pid="$(padded_id_from_file "$f")"
  # cid — the change-id column for every finding about this file: the validated integer id when
  # there is one, else the filename-derived padded id. NEVER the raw frontmatter value.
  cid="${id:-$pid}"
  if [ -z "$id" ]; then
    [ -n "$raw" ] && emit malformed-id "$cid" "non-integer id '$raw' in $(basename "$f")"
    continue
  fi
```

- [ ] **Step 4: Run the tests to verify they pass**

```bash
bash tests/test_board_checks.sh 2>&1 | grep -E 'NOT OK|^PASS|^FAIL'
```

Expected: `PASS`. Every other check's asserts (`broken-spec 2`, `dep-cycle 1`, `stale-in-progress 20`, …) must still pass — they emit `$id`, which is unchanged.

- [ ] **Step 5: Mutation-test the sanitizer**

```bash
cp scripts/board-checks.sh /tmp/bc.bak
# Strip the sanitizer: emit passes values through raw again.
sed -i.tmp 's/^emit(){ FINDINGS+="\$1"\$.\\t."\$(sanitize "\$2")"\$.\\t."\$(sanitize "\$3")"\$.\\n."; }$/emit(){ FINDINGS+="$1"$'"'"'\\t'"'"'"$2"$'"'"'\\t'"'"'"$3"$'"'"'\\n'"'"'; }/' scripts/board-checks.sh
grep -n '^emit()' scripts/board-checks.sh          # confirm the mutation actually applied
bash tests/test_board_checks.sh 2>&1 | grep -E 'NOT OK'   # expect: the TAB-escape assert reddens
cp /tmp/bc.bak scripts/board-checks.sh; rm -f scripts/board-checks.sh.tmp

# Strip the cid indirection: malformed-id keys on the raw value again.
cp scripts/board-checks.sh /tmp/bc.bak
sed -i.tmp 's/emit malformed-id "\$cid"/emit malformed-id "$raw"/' scripts/board-checks.sh
bash tests/test_board_checks.sh 2>&1 | grep -E 'NOT OK'   # expect: the filename-keyed + TAB-split asserts redden
cp /tmp/bc.bak scripts/board-checks.sh; rm -f scripts/board-checks.sh.tmp

bash tests/test_board_checks.sh 2>&1 | tail -1            # expect: PASS
```

If a `sed` mutation is awkward to express, apply it by hand with an editor instead — the requirement is that the mutation lands and the named assert reddens, not that `sed` expresses it.

- [ ] **Step 6: Commit**

```bash
git add scripts/board-checks.sh tests/test_board_checks.sh
git commit -m "fix(0104): sanitize the findings channel against field-shifting values

emit() now escapes TAB and CR in both embedded columns, and the change-id column
uses the filename-derived padded id wherever the frontmatter id is unusable —
never the raw value. docket-status.sh reads findings back with IFS=\$'\\t' read,
so an interior TAB in id: shifted the message into the wrong field: the guard's
own reporting channel was injectable by the input class it exists to catch.

Checks with an int_field-validated id keep emitting it verbatim (^[0-9]+\$ cannot
shift a field), so the report format is unchanged for every existing check."
```

---

### Task 3: The `field-domain` check (spec part 1)

**Files:**
- Modify: `scripts/board-checks.sh` (inside the FILES walk, after `status=` is read), `:11-12` (header enumeration)
- Modify: `scripts/board-checks.md`
- Test: `tests/test_board_checks.sh`

**Interfaces:**
- Consumes: `DOCKET_STATUSES` (Task 1), `emit` / `cid` (Task 2).
- Produces: `status_ok` (1 when the file's `status:` is in the vocabulary, else 0) — Task 4 reads it for the drop invariant.

Domains, chosen by what the renderers actually consume:

| field | domain | today's silent failure |
|---|---|---|
| `status` | ∈ `DOCKET_STATUSES`; **empty also fails** | row dropped from every board surface |
| `slug` | `^[a-z0-9-]+$` (`slugify`'s own alphabet, `mint-stub.sh:88-91`); empty fails | leaks raw into the digest's space-joined `change` line |
| `priority` | ∈ `low|medium|high|critical`; **empty is LEGAL** | sorts as `medium` in `ready`, renders raw in the Priority cell |
| `title` | contains no `|` | injects columns into the `BOARD.md` table row |

`priority`'s empty case is legal because the convention documents `medium` as the default and `render-board.sh`'s sort already implements it. `status` and `slug` have no documented default. `id` is deliberately **not** covered — `malformed-id` already detects it and a second overlapping check would double-report.

- [ ] **Step 1: Write the failing tests**

Append to `tests/test_board_checks.sh`, after the Task 2 block:

```bash
# ============================ field-domain (change 0104, spec part 1) ============================
# A value that is well-formed TEXT but outside its field's DOMAIN. Validated by shape/membership,
# never by enumerating bad strings — the spelling you enumerate is never the one that arrives.
read -r F _ < <(new_repo)
mk_fd(){ # mk_fd FILE-BASENAME ID SLUG TITLE STATUS PRIORITY
  printf -- '---\nid: %s\nslug: %s\ntitle: %s\nstatus: %s\npriority: %s\ndepends_on: []\n---\n' \
    "$2" "$3" "$4" "$5" "$6" > "$F/docs/changes/active/$1"
}
mk_fd 0040-clean.md    40 clean    "Clean change"  proposed            medium
mk_fd 0041-poison.md   41 poison   "Poisoned"      "proposed  # awaiting X" medium
mk_fd 0042-badslug.md  42 "bad slug" "Bad slug"    proposed            medium
mk_fd 0043-badprio.md  43 badprio  "Bad priority"  proposed            urgent
mk_fd 0044-pipe.md     44 pipe     "T5 | injected | row" proposed      medium
mk_fd 0045-emptyprio.md 45 emptyprio "Empty priority" proposed         ""
printf -- '---\nid: 46\nslug: nostatus\ntitle: No status\nstatus:\npriority: medium\ndepends_on: []\n---\n' \
  > "$F/docs/changes/active/0046-nostatus.md"
fout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$F/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"

assert "field-domain silent for a wholly clean change (id 40)"      '! has_finding "$fout" field-domain 40'
assert "field-domain fires for a status carrying an inline comment (id 41)" 'has_finding "$fout" field-domain 41'
assert "field-domain fires for a slug with a space (id 42)"          'has_finding "$fout" field-domain 42'
assert "field-domain fires for an unrecognized priority (id 43)"     'has_finding "$fout" field-domain 43'
assert "field-domain fires for a title containing a pipe (id 44)"    'has_finding "$fout" field-domain 44'
# The documented default: an EMPTY priority is LEGAL (convention says medium; the sort implements
# it). This assert is what keeps the domain check from becoming over-eager.
assert "field-domain SILENT for an empty priority (id 45, documented default)" \
  '! has_finding "$fout" field-domain 45'
assert "field-domain fires for an EMPTY status (id 46, no documented default)" \
  'has_finding "$fout" field-domain 46'

# Messages name the field and quote the offending value, so a reader can act without opening the file.
assert "the status finding names the field and the offending value" \
  'printf "%s" "$fout" | grep -qF "status '"'"'proposed  # awaiting X'"'"'"'
assert "the title finding names the pipe as the board-row hazard" \
  'printf "%s" "$fout" | grep -E "^field-domain\t44\t" | grep -qF "title"'

# Shape, not spelling: a slug with a TAB and a slug with an uppercase letter both fire, though
# neither is an enumerated bad value.
mk_fd 0047-tabslug.md 47 "$(printf 'tab\tslug')" "Tab slug" proposed medium
mk_fd 0048-upper.md   48 UpperSlug "Upper slug"   proposed medium
sout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$F/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "field-domain fires for a slug containing a TAB (shape check, id 47)"  'has_finding "$sout" field-domain 47'
assert "field-domain fires for an uppercase slug (shape check, id 48)"        'has_finding "$sout" field-domain 48'
assert "a TAB inside a slug value cannot shift the findings line's columns (id 47)" \
  'printf "%s" "$sout" | grep -E "^field-domain\t47\t" | grep -qF "\\t"'

# Warn-only posture is preserved: findings present, exit still 0 without --strict.
NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$F/docs/changes" --metadata-branch docket --integration-branch main >/dev/null 2>&1
assert "field-domain findings do not change the default exit status (warn-only)" '[ "$?" = 0 ]'

# The archive is walked too — a terminal status is in the vocabulary and must stay silent.
read -r G _ < <(new_repo)
printf -- '---\nid: 60\nslug: archived\ntitle: Archived\nstatus: done\npriority: medium\ndepends_on: []\n---\n' \
  > "$G/docs/changes/archive/2026-06-16-0060-archived.md"
gout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$G/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "field-domain silent for a terminal status in archive/ (id 60)" '! has_finding "$gout" field-domain 60'
```

- [ ] **Step 2: Run the tests to verify they fail**

```bash
bash tests/test_board_checks.sh 2>&1 | grep -E 'NOT OK|^PASS|^FAIL'
```

Expected: every `field-domain fires…` assert is `NOT OK` (the check does not exist); the `silent` asserts pass vacuously. That asymmetry is expected and is why the mutation step below matters.

- [ ] **Step 3: Implement the check**

In `scripts/board-checks.sh`, inside the FILES walk, replace:

```bash
  status="$(field "$f" status)"
  spec="$(field "$f" spec)"; trivial="$(field "$f" trivial)"
```

with:

```bash
  status="$(field "$f" status)"
  spec="$(field "$f" spec)"; trivial="$(field "$f" trivial)"

  # --- field-domain: a value that is well-formed TEXT but outside its field's DOMAIN (change 0104).
  # These four fields are what the board renderers consume. A value outside the domain does not
  # error — it silently drops the row from every surface (status, slug) or injects columns into it
  # (title), and since change 0094 the digest's `ready` line is the machine-parsed selection channel
  # for docket-implement-next, so a stray inline comment can remove a change from the autonomous
  # build queue while the board still reports a healthy count. One finding per violated field.
  # Every domain is a SHAPE or a MEMBERSHIP test — never an enumeration of bad values.
  # `id` is deliberately absent: malformed-id already covers it (no double-reporting).
  fd_slug="$(field "$f" slug)"; fd_priority="$(field "$f" priority)"; fd_title="$(field "$f" title)"

  status_ok=0
  for fd_s in "${DOCKET_STATUSES[@]}"; do
    if [ "$status" = "$fd_s" ]; then status_ok=1; break; fi
  done
  if [ "$status_ok" != 1 ]; then
    emit field-domain "$cid" "status '$status' is not one of: ${DOCKET_STATUSES[*]}"
  fi

  # slugify's own alphabet (mint-stub.sh:88-91). Empty fails — slug has no documented default.
  case "$fd_slug" in
    ''|*[!a-z0-9-]*) emit field-domain "$cid" "slug '$fd_slug' is not ^[a-z0-9-]+\$" ;;
  esac

  # Empty priority is LEGAL: the convention documents `medium` as the default and render-board.sh's
  # sort already implements it. Flagging it here would make the guard the noise source.
  case "$fd_priority" in
    ''|low|medium|high|critical) ;;
    *) emit field-domain "$cid" "priority '$fd_priority' is not one of: low medium high critical (empty = medium)" ;;
  esac

  case "$fd_title" in
    *'|'*) emit field-domain "$cid" "title contains '|', which injects columns into the board row: $fd_title" ;;
  esac
```

Then update the header enumeration at `:11-12` to add the new id (and the long-missing `malformed-id`):

```bash
#     check-id ∈ {broken-spec, broken-plan-results, dep-cycle, field-domain, stale-in-progress,
#                 merge-gate-stall, stale-finalize-blocked, merged-orphan, unknown-commit-ref,
#                 malformed-id}
```

- [ ] **Step 4: Run the tests to verify they pass**

```bash
bash tests/test_board_checks.sh 2>&1 | grep -E 'NOT OK|^PASS|^FAIL'
```

Expected: `PASS`.

- [ ] **Step 5: Mutation-test each domain independently**

Each domain arm must be individually load-bearing — a single assert that reddens for all four proves nothing about three of them.

```bash
cp scripts/board-checks.sh /tmp/bc.bak
for arm in status slug priority title; do
  cp /tmp/bc.bak scripts/board-checks.sh
  case "$arm" in
    status)   sed -i.tmp 's/^    emit field-domain "\$cid" "status /    : "/' scripts/board-checks.sh ;;
    slug)     sed -i.tmp "s|''|\\*\\[!a-z0-9-\\]\\*) emit field-domain|''|*[!a-z0-9-]*) : |" scripts/board-checks.sh ;;
    priority) sed -i.tmp 's/    \*) emit field-domain "\$cid" "priority /    *) : "/' scripts/board-checks.sh ;;
    title)    sed -i.tmp "s|    \\*'|'\\*) emit field-domain|    *'|'*) : |" scripts/board-checks.sh ;;
  esac
  echo "--- mutation: $arm ---"
  bash tests/test_board_checks.sh 2>&1 | grep -E 'NOT OK'
done
cp /tmp/bc.bak scripts/board-checks.sh; rm -f scripts/board-checks.sh.tmp
bash tests/test_board_checks.sh 2>&1 | tail -1   # expect: PASS
```

Expected: each mutation reddens **only** its own domain's assert. If a mutation reddens nothing, that domain's assert is decoration — fix the assert, not the mutation. The `sed` expressions above are fiddly with shell quoting; applying each mutation by hand in an editor is equally valid and often faster.

Also confirm the empty-priority negative is discriminating rather than coincidental: temporarily add `''` to the *bad* side of the priority case (`''|*)` → emit) and confirm the id-45 assert reddens. Revert.

- [ ] **Step 6: Document the check in the contract**

In `scripts/board-checks.md`, add after the `dep-cycle` section:

```markdown
**`field-domain`** — A frontmatter value that is well-formed *text* but outside its field's
*domain*. These are the four fields the board renderers consume; a value outside the domain does
not error, it silently drops the change's row from every board surface (`status`, `slug`) or
injects columns into it (`title`). One finding per violated field, per change.

| Field | Domain | Empty | Failure mode without the check |
|---|---|---|---|
| `status` | one of the seven lifecycle statuses (`DOCKET_STATUSES` in `lib/docket-frontmatter.sh`) | **fails** | The row is bucketed under an unrecognized key and never emitted, while the file is still counted in the board's total — the count line and the tables disagree. The change also vanishes from the digest's `ready` queue. |
| `slug` | `^[a-z0-9-]+$` — `slugify`'s own alphabet | **fails** | Leaks raw into the digest's space-joined `change` line. |
| `priority` | one of `low`, `medium`, `high`, `critical` | **legal** (`medium`) | Sorts as `medium` in the `ready` queue while rendering raw in the Priority cell. |
| `title` | contains no `|` | legal | Injects extra columns into the `BOARD.md` table row. |

`id` is deliberately **not** covered here — `malformed-id` already detects a non-integer id, and a
second overlapping check would double-report the same file. Every domain is a shape or membership
test; none enumerates bad values.
```

- [ ] **Step 7: Commit**

```bash
git add scripts/board-checks.sh scripts/board-checks.md tests/test_board_checks.sh
git commit -m "feat(0104): add the field-domain check

A frontmatter value that is well-formed text but outside its field's domain
silently deletes the change from every board surface — including the digest's
ready queue, the machine-parsed selection channel for docket-implement-next.
Covers status, slug, priority and title, the four fields the renderers consume.
Empty priority stays legal (the convention's documented medium default); empty
status and slug do not. Warn-only; id stays with malformed-id.

Registers the new id in the header block, which had also been missing
malformed-id since that check landed."
```

---

### Task 4: The `board-row-dropped` backstop (spec part 2)

An active file counted in the board's `total` but rendered in no section is itself a detectable invariant violation. It is emitted **only when no `field-domain` or `malformed-id` finding already explains that id** — suppression is what makes it mean exactly one thing: *a row vanished and nothing enumerated explains why.*

The un-suppressed trigger that exists **today**: a change file with **no `id:` field at all**. `raw` is empty, so `malformed-id` (which requires a non-empty raw) never fires; `render-board.sh:76` skips it from `SECTION` while `:86` still counts it in `total`. Nothing explains the drop. That is a real defect and the backstop's proof of non-vacuity.

**Files:**
- Modify: `scripts/board-checks.sh` (walk head, the `field-domain` block, and a new post-walk loop), `:11-12`
- Modify: `scripts/board-checks.md`
- Test: `tests/test_board_checks.sh`

**Interfaces:**
- Consumes: `cid` (Task 2), `status_ok` (Task 3).
- Produces: nothing consumed downstream.

- [ ] **Step 1: Write the failing tests**

Append to `tests/test_board_checks.sh`, after the Task 3 block:

```bash
# ======================= board-row-dropped (change 0104, spec part 2) =======================
# The invariant: an ACTIVE file counted in render-board.sh's `total` but rendered in no section.
# SUPPRESSED when field-domain or malformed-id already explains that id — a backstop that fires
# alongside every domain finding trains the reader to ignore it.
read -r D _ < <(new_repo)
# (a) the live un-suppressed trigger: NO id: field at all. malformed-id needs a non-empty raw
#     value, so nothing explains this drop.
printf -- '---\nslug: noid\ntitle: No id\nstatus: proposed\npriority: medium\ndepends_on: []\n---\n' \
  > "$D/docs/changes/active/0070-noid.md"
dout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$D/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "board-row-dropped fires for an active file with no id: field (0070)" \
  'has_finding "$dout" board-row-dropped 0070'

# (b) suppression by field-domain: a poisoned status yields EXACTLY ONE finding for that id.
read -r E _ < <(new_repo)
printf -- '---\nid: 71\nslug: poison\ntitle: Poisoned\nstatus: proposed  # awaiting X\npriority: medium\ndepends_on: []\n---\n' \
  > "$E/docs/changes/active/0071-poison.md"
eout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$E/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
n71="$(printf '%s' "$eout" | grep -c .)"
assert "a poisoned status yields exactly ONE finding, not two (suppression works)" '[ "$n71" = 1 ]'
assert "and that one finding is field-domain, not board-row-dropped" 'has_finding "$eout" field-domain 71'
assert "board-row-dropped is suppressed when field-domain explains the drop" \
  '! has_finding "$eout" board-row-dropped 71'

# (c) suppression by malformed-id.
read -r H _ < <(new_repo)
printf -- '---\nid: abc\nslug: badid\ntitle: Bad id\nstatus: proposed\npriority: medium\ndepends_on: []\n---\n' \
  > "$H/docs/changes/active/0072-badid.md"
hout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$H/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "board-row-dropped is suppressed when malformed-id explains the drop (0072)" \
  '! has_finding "$hout" board-row-dropped 0072'
assert "malformed-id still fires for that file (0072)" 'has_finding "$hout" malformed-id 0072'

# (d) archive/ is NOT subject to the invariant — the archive table renders from its own pass.
read -r I _ < <(new_repo)
printf -- '---\nslug: archnoid\ntitle: Arch no id\nstatus: done\npriority: medium\ndepends_on: []\n---\n' \
  > "$I/docs/changes/archive/2026-06-16-0073-archnoid.md"
iout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$I/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "board-row-dropped does not fire for an archive/ file (0073)" \
  '! has_finding "$iout" board-row-dropped 0073'

# (e) a wholly clean tree stays silent — the backstop must not fire on healthy repos.
read -r J _ < <(new_repo)
printf -- '---\nid: 74\nslug: fine\ntitle: Fine\nstatus: proposed\npriority: medium\ndepends_on: []\n---\n' \
  > "$J/docs/changes/active/0074-fine.md"
jout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$J/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "clean active tree emits no board-row-dropped finding" '! printf "%s" "$jout" | grep -q "^board-row-dropped"'
```

- [ ] **Step 2: Run the tests to verify they fail**

```bash
bash tests/test_board_checks.sh 2>&1 | grep -E 'NOT OK|^PASS|^FAIL'
```

Expected: `NOT OK` for (a) — the check does not exist. (b)'s exactly-one assert should already pass (only `field-domain` fires today), which is precisely why Step 5's mutation is required to prove it is not vacuous.

- [ ] **Step 3: Implement the invariant and its suppression**

In `scripts/board-checks.sh`, extend the `declare -A` line near `FINDINGS`:

```bash
declare -A ID_ACTIVE ID_EXISTS                # id -> 1; populated in the FILES walk below
declare -A EXPLAINED DROPPED                  # change-id -> 1; drive board-row-dropped (change 0104)
```

In the walk head, record the drop and the explanation for the malformed/absent-id case:

```bash
for f in "${FILES[@]}"; do
  raw="$(field "$f" id)"; id="$(int_field "$f" id)"
  pid="$(padded_id_from_file "$f")"
  cid="${id:-$pid}"
  fd_active=0; case "$f" in */active/*) fd_active=1 ;; esac
  if [ -z "$id" ]; then
    if [ -n "$raw" ]; then
      emit malformed-id "$cid" "non-integer id '$raw' in $(basename "$f")"
      EXPLAINED["$cid"]=1
    fi
    # No usable id ⇒ render-board.sh:76 skips the row while :86 still counts the file.
    [ "$fd_active" = 1 ] && DROPPED["$cid"]=1
    continue
  fi
```

In the `field-domain` block, mark the id explained on every emit and record the status-driven drop. Replace the `status_ok` emit and add the drop line after the four domain arms:

```bash
  if [ "$status_ok" != 1 ]; then
    emit field-domain "$cid" "status '$status' is not one of: ${DOCKET_STATUSES[*]}"
    EXPLAINED["$cid"]=1
    # An unrecognized status buckets into a SECTION key no renderer iterates.
    [ "$fd_active" = 1 ] && DROPPED["$cid"]=1
  fi
```

and append `EXPLAINED["$cid"]=1` to the other three arms, e.g.:

```bash
  case "$fd_slug" in
    ''|*[!a-z0-9-]*) emit field-domain "$cid" "slug '$fd_slug' is not ^[a-z0-9-]+\$"; EXPLAINED["$cid"]=1 ;;
  esac
```

(Do the same for the `priority` and `title` arms. Marking every `field-domain` emit — not only the status one — is what the spec's suppression rule says: *no `field-domain` or `malformed-id` finding exists for that same change id*.)

Then add the post-walk emission, immediately after the FILES `done` and before the `dep-cycle` block:

```bash
# --- board-row-dropped: an ACTIVE file counted in the board's total but rendered in no section ---
# render-board.sh counts every active *.md in `total` (:86) but buckets rows on the raw `status:`
# read (:78) and skips files whose id is not an integer (:76) — so such a file is counted and never
# emitted, and the board's count line disagrees with its tables. SUPPRESSED when a field-domain or
# malformed-id finding already explains that id: a backstop that fires alongside every domain
# finding trains the reader to ignore it. Suppressed, it says exactly one thing — a row vanished and
# nothing enumerated explains why. Its live trigger today is a file with NO `id:` field at all
# (malformed-id needs a non-empty raw value); with the domain checks in place, its remaining trigger
# is a future renderer-added drop path, which is what a backstop is for.
for drop_id in "${!DROPPED[@]}"; do
  [ -n "${EXPLAINED[$drop_id]:-}" ] && continue
  emit board-row-dropped "$drop_id" "counted in the board total but rendered in no section, and no field-domain or malformed-id finding explains it"
done
```

Finally add the id to the header enumeration at `:11-12`:

```bash
#     check-id ∈ {board-row-dropped, broken-spec, broken-plan-results, dep-cycle, field-domain,
#                 stale-in-progress, merge-gate-stall, stale-finalize-blocked, merged-orphan,
#                 unknown-commit-ref, malformed-id}
```

- [ ] **Step 4: Run the tests to verify they pass**

```bash
bash tests/test_board_checks.sh 2>&1 | grep -E 'NOT OK|^PASS|^FAIL'
```

Expected: `PASS`.

- [ ] **Step 5: Mutation-test the suppression in both directions**

The suppression asserts pass trivially if the backstop never computes anything. Prove otherwise.

```bash
cp scripts/board-checks.sh /tmp/bc.bak

# (i) Remove the suppression lookup: the poisoned-status fixture must now emit TWO findings,
#     reddening the "exactly ONE finding" assert. This proves the invariant genuinely computes
#     rather than being a dead branch that suppression only appears to gate.
sed -i.tmp 's/^  \[ -n "\${EXPLAINED\[\$drop_id\]:-}" \] && continue$/  :/' scripts/board-checks.sh
bash tests/test_board_checks.sh 2>&1 | grep -E 'NOT OK'   # expect: exactly-ONE + suppression asserts redden
cp /tmp/bc.bak scripts/board-checks.sh; rm -f scripts/board-checks.sh.tmp

# (ii) Remove the DROPPED write on the no-id path: assert (a) must redden.
sed -i.tmp 's/^    \[ "\$fd_active" = 1 \] && DROPPED\["\$cid"\]=1$/    :/' scripts/board-checks.sh
bash tests/test_board_checks.sh 2>&1 | grep -E 'NOT OK'   # expect: the 0070 assert reddens
cp /tmp/bc.bak scripts/board-checks.sh; rm -f scripts/board-checks.sh.tmp

# (iii) Drop the active-only guard: the archive fixture (d) must redden.
sed -i.tmp 's/fd_active=0; case "\$f" in \*\/active\/\*) fd_active=1 ;; esac/fd_active=1/' scripts/board-checks.sh
bash tests/test_board_checks.sh 2>&1 | grep -E 'NOT OK'   # expect: the 0073 archive assert reddens
cp /tmp/bc.bak scripts/board-checks.sh; rm -f scripts/board-checks.sh.tmp

bash tests/test_board_checks.sh 2>&1 | tail -1            # expect: PASS
```

- [ ] **Step 6: Document the check in the contract**

In `scripts/board-checks.md`, add after the `field-domain` section:

```markdown
**`board-row-dropped`** — Backstop for the count-vs-rows invariant. An `active/` change file that
`render-board.sh` counts in the board's total but renders in no section: its `id:` is not a
well-formed integer, or its `status:` is outside `DOCKET_STATUSES`. Emitted **only when no
`field-domain` or `malformed-id` finding already exists for that change id** — a backstop that fired
alongside every domain finding would train the reader to ignore it. Suppressed, it means exactly one
thing: *a row vanished and nothing enumerated explains why.*

Its live trigger is a change file with **no `id:` field at all** — `malformed-id` requires a
non-empty (if non-integer) value, so nothing else reports it. Beyond that, its remaining trigger is
a future renderer-added drop path. `archive/` files are exempt: the archive table renders from its
own pass and is not subject to this invariant.
```

Also correct the pre-existing `malformed-id` paragraph, which still describes the old column:

```markdown
**`malformed-id`** — Guard/carve-out, not counted among the named checks above. A change file
whose `id:` field is non-empty but non-integer emits a `malformed-id` finding. The change-id column
carries the **filename-derived** padded id (`?` when the filename yields none) — never the raw
frontmatter value, which is untrusted input and would shift the caller's TAB-separated fields; the
raw value appears in the message instead. The file is then skipped for all other checks.
```

And add to the **Invariants** section:

```markdown
- **The findings channel is not injectable.** `emit` escapes TAB and CR to visible `\t` / `\r` in
  both embedded columns, and the change-id column never carries a raw frontmatter value. The caller
  splits findings with `IFS=$'\t' read -r check_id change_id message`, so an un-escaped TAB in an
  untrusted value would shift every later field.
```

- [ ] **Step 7: Commit**

```bash
git add scripts/board-checks.sh scripts/board-checks.md tests/test_board_checks.sh
git commit -m "feat(0104): add the suppressed board-row-dropped backstop

An active file counted in the board's total but rendered in no section, emitted
only when no field-domain or malformed-id finding already explains that id. The
suppression is what gives it meaning: a row vanished and nothing enumerated
explains why. Its live trigger today is a change file with no id: field at all,
which malformed-id structurally cannot report.

Also corrects board-checks.md's malformed-id paragraph, which still described
the pre-sanitization change-id column."
```

---

### Task 5: Repair the pre-existing registration drift, then gate on the whole suite

The reconcile found **both** check-id enumerations already stale, in opposite directions, each undetected since the change that introduced the gap. Tasks 3 and 4 repaired `board-checks.sh`'s header as a by-product; `scripts/docket-status.md`'s closed enumeration still omits `stale-finalize-blocked` (change 0098 never registered it) as well as both new ids.

Shipping a guard against silent drift alongside a drifted enumeration of its own check-ids would be self-refuting.

**Files:**
- Modify: `scripts/docket-status.md:344`
- Test: the whole suite

**Interfaces:**
- Consumes: the final check-id set from Tasks 3 and 4.

- [ ] **Step 1: Update the closed check-id enumeration**

In `scripts/docket-status.md`, replace line 344's enumeration:

```markdown
| `check <check-id> <change-id> <message>` | One `board-checks.sh` finding, passed through with the `check` prefix. `<check-id>` ∈ {broken-spec, broken-plan-results, dep-cycle, stale-in-progress, merge-gate-stall, merged-orphan, unknown-commit-ref, malformed-id}. |
```

with:

```markdown
| `check <check-id> <change-id> <message>` | One `board-checks.sh` finding, passed through with the `check` prefix. `<check-id>` ∈ {board-row-dropped, broken-spec, broken-plan-results, dep-cycle, field-domain, stale-in-progress, stale-finalize-blocked, merge-gate-stall, merged-orphan, unknown-commit-ref, malformed-id}. |
```

- [ ] **Step 2: Verify the two enumerations now agree with the emitting code**

Derive the emitted set from the producer rather than reading either list:

```bash
emitted="$(grep -oE '^[[:space:]]*emit [a-z-]+' scripts/board-checks.sh | awk '{print $2}' | sort -u)"
printf 'emitted:\n%s\n' "$emitted"
for c in $emitted; do
  grep -qF "$c" scripts/docket-status.md || echo "MISSING from docket-status.md: $c"
  grep -qF "$c" scripts/board-checks.md  || echo "MISSING from board-checks.md: $c"
  sed -n '11,14p' scripts/board-checks.sh | grep -qF "$c" || echo "MISSING from the header block: $c"
done
```

Expected: the `emitted` list is the eleven ids, and **no** `MISSING` line. (This is a one-off build-time verification, not a committed guard — the standing guard is tracked as change 0111.)

- [ ] **Step 3: Run the whole suite**

Never gate on only the two test files this plan touched.

```bash
cd /Users/homer/dev/docket/.worktrees/guard-frontmatter-field-domain-violations-that-silently-drop
fail=0; for t in tests/test_*.sh; do bash "$t" >"/tmp/$(basename "$t").out" 2>&1 || { echo "FAIL: $t"; fail=1; }; done; echo "suite fail=$fail"
```

Expected: `suite fail=0`. If any unrelated test fails, read `/tmp/<test>.out` — the likely culprits are `test_script_contracts_coverage.sh` (contract/script correspondence) and `test_docket_status.sh` (report-line vocabulary), both of which read the files this plan edits.

- [ ] **Step 4: Verify the real repo renders unchanged**

The hermetic suite sees only its fixtures. Check the actual board is unmoved by the Task 1 refactor and that the checks fire sanely against real data (learnings: `metadata-branch-invisible-to-suite`).

```bash
bash scripts/render-board.sh --changes-dir /Users/homer/dev/docket/.docket/docs/changes > /tmp/board-new.md 2>/dev/null
diff -u /Users/homer/dev/docket/.docket/docs/changes/BOARD.md /tmp/board-new.md && echo "BOARD unchanged"
bash scripts/board-checks.sh --changes-dir /Users/homer/dev/docket/.docket/docs/changes \
  --metadata-branch docket --integration-branch origin/main
echo "exit=$?"
```

Expected: `BOARD unchanged` (or only differences attributable to changes that landed since the board was last rendered — inspect, do not assume), zero `field-domain` / `board-row-dropped` findings against the live backlog, and `exit=0`. A finding here is a real defect in a real change file — report it, do not silence the check.

- [ ] **Step 5: Prove the new checks fire against real data**

A green run over a clean repo does not prove the path is live. Mutate a throwaway copy.

```bash
cp -R /Users/homer/dev/docket/.docket/docs/changes /tmp/changes-probe
sed -i.tmp 's/^status: proposed$/status: proposed  # probe/' /tmp/changes-probe/active/0083-*.md
bash scripts/board-checks.sh --changes-dir /tmp/changes-probe \
  --metadata-branch docket --integration-branch origin/main | grep -E 'field-domain|board-row-dropped'
rm -rf /tmp/changes-probe
```

Expected: exactly one `field-domain` finding for id 83, and **no** `board-row-dropped` for it (suppressed). Record this result for the results file.

- [ ] **Step 6: Commit**

```bash
git add scripts/docket-status.md
git commit -m "docs(0104): register the new check-ids and repair pre-existing drift

docket-status.md's closed check-id enumeration gains field-domain and
board-row-dropped, and also stale-finalize-blocked, which change 0098 shipped
without ever registering. Both enumerations were stale in opposite directions;
a drift guard shipping beside a drifted copy of its own check-ids would be
self-refuting. The standing correspondence guard is change 0111."
```

---

## Notes for the implementer

- **Line numbers shift as tasks land.** Every `:NNN` in this plan was verified against `origin/main` at `2748ed9`. Re-locate by content (the quoted code), not by line number, from Task 2 onward.
- **The plan's test code is unverified code.** It was written against an implementation that did not exist. Before debugging the implementation against a red assert, check the assert's own field indices, quoting, and expected strings against the real output format — an assert that cannot pass under *any* correct implementation reads as a real regression and burns a cycle. Shell quoting inside `assert '<expr>'` (which is `eval`'d) is the most likely defect: prefer computing a value into a variable on the line above and asserting on the variable.
- **Fixtures must discriminate.** The `field-domain` negatives (clean change, empty priority, archived terminal status) are only meaningful if the corresponding positive fires in the same run. Where a mutation reddens nothing, the fixture — not the assert — is usually what is hiding the hole.
- **Do not re-bless the golden.** If `tests/test_render_board.sh`'s byte-compare fails after Task 1, the substitution changed an iteration order. Fix the arrays.

## Self-Review

**1. Spec coverage.** Part 1 `field-domain` → Task 3 (all four domains, empty-priority carve-out, `id` exclusion). Part 2 `board-row-dropped` → Task 4 (invariant + suppression by both explaining checks). Part 3 sanitize → Task 2 (`emit` escaping + change-id column), with the blanket-padding deviation argued and flagged for an ADR. Part 4 single-source → Task 1 (lib arrays, four substitutions, order preserved). Spec *Registration points* → Tasks 3, 4 (header, `board-checks.md`) and Task 5 (`docket-status.md`, plus the pre-existing `stale-finalize-blocked` gap the reconcile added). Spec *Testing* list: one fixture per domain ✓ (T3S1); empty-priority no-finding ✓ (T3S1); suppression exactly-one ✓ (T4S1); TAB-in-`id` through the caller's `read` ✓ (T2S1); golden + idempotence unchanged ✓ (T1S5); buckets == lib array ✓ (T1S1, set equality both directions); `emoji_for`/`label_for_title` totality ✓ (T1S1, same set-equality asserts — stronger than "non-empty", since it also catches a phantom arm); mutation-check per guard ✓ (T1S6, T2S5, T3S5, T4S5). Spec *Autonomy posture* (reports, never gates) → no skill-file edits, per the reconcile log's answer to the spec's deferred question; the warn-only exit-status assert in T3S1 pins the script half.

**2. Placeholder scan.** No TBD/TODO. Every code step carries the literal code. Every mutation step names the expected reddening assert. The one "apply by hand if `sed` is awkward" escape is a note about mechanism, not about content — the mutation itself is fully specified.

**3. Type consistency.** `cid` is introduced in Task 2 and consumed by Tasks 3 and 4 under that exact name. `status_ok` is set in Task 3 and read in Task 4. `fd_active` is introduced in Task 4's walk-head rewrite and used in the same task; Task 3's code does not reference it. `EXPLAINED` / `DROPPED` are declared and used in Task 4 only. `padded_id_from_file` / `sanitize` keep one spelling throughout. `DOCKET_STATUSES` / `DOCKET_STATUSES_ACTIVE` / `DOCKET_STATUSES_TERMINAL` match the spec's names exactly.

**Known ordering hazard:** Task 4 rewrites the walk head that Task 2 wrote (adding `fd_active` and the `DROPPED`/`EXPLAINED` writes). That intermediate state — Task 2's head without Task 4's additions — is itself buildable and its tests pass, so the two tasks remain independently reviewable.
