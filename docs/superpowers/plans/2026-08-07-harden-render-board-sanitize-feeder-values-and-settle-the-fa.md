<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0259 — Harden render-board: sanitize feeder values and settle the failure contract](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-08-0259-harden-render-board-sanitize-feeder-values-and-settle-the-fa.md)**
<!-- docket:backlink:end -->

# Harden render-board: sanitize feeder values and settle the failure contract — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give `scripts/render-board.sh` a closed, loud failure contract — reject malformed change files by vocabulary before their values reach a TAB join or an array subscript, diagnose each one on stderr, and exit 3 so every caller stops trusting a corrupt render.

**Architecture:** One upfront validation pass over `AFILES` + `ARCFILES` classifies each file against a closed enumeration (M1 unusable id, M2 empty status, M3 status outside `DOCKET_STATUSES`), records a per-file diagnostic, and marks the file excluded. The existing section/tally loops keep their guards and gain a membership check so an excluded file never reaches `SECTION[...]`, `ARC_COUNT[...]`, or the archive sort feeder. A belt-and-suspenders read-back check (M4) at the archive consumer catches any future control-character path into the TAB join that M1–M3 cannot see. Both emission paths (markdown fall-off-the-end, digest `exit 0`) become `exit 3` when the malformed count is non-zero. Membership over the full seven-name vocabulary is single-sourced through a new `docket_status_is_member` helper in `scripts/lib/docket-frontmatter.sh`, beside its existing `_active` / `_terminal` siblings.

**Tech Stack:** Bash 4+ (`scripts/lib/docket-frontmatter.sh` shared library), the repo's hand-rolled `assert`-based test harness (`tests/test_*.sh`), `scripts/run-tests.sh` as the full-suite runner.

## Global Constraints

- **Bash 4+, BSD-first.** macOS BSD `sed`/`grep`/`mktemp` are the baseline. BSD `sed` does not interpret `\t` in a pattern — use pure bash parameter expansion for the `sanitize()` escapes, exactly as `scripts/board-checks.sh:142` does.
- **In tests, always `/usr/bin/grep`, never PATH `grep`.** PATH `grep` in this environment is ugrep 7.5.0 and accepts patterns BSD grep rejects, so a portability bug passes locally under PATH `grep`. Every new assert that greps follows the existing `/usr/bin/grep -qF` / `-qE` house style already used in the change-0143 block.
- **Do not add a third `for st in "${DOCKET_STATUSES[@]}"` line to `scripts/render-board.sh`.** `tests/test_render_board.sh:1890-1893` pins `n_all = 2` by a literal `grep -cF` of that exact string, and `:1896` pins `n_literal = 0` for `^[[:space:]]*for st in [a-z]`. The validation pass iterates FILES (`for f in …`), and membership is tested with the `docket_status_is_member` helper — never a new status-array iteration and never a hand-written status list.
- **`render-board.sh` stays a pure renderer.** stdout carries content only, stderr carries diagnostics only; no git writes, no network, no writing/truncating `BOARD.md`. Determinism is contractual: same change files → identical bytes.
- **Exit-code vocabulary is closed:** `0` clean, `2` CLI-argument error (unchanged, already asserted by existing tests), `3` malformed change file(s) detected. Never reuse `1` or `2`.
- **`scripts/board-checks.sh` is not edited by this change** — its `sanitize()` is duplicated into `render-board.sh`, deliberately not hoisted (spec Assumption 5).
- **Sanitization is for diagnostics only.** Never launder a value and then interpolate it into a feeder; M3 rejects before interpolation (spec Assumption 4).
- **Placement is not malformedness.** A vocabulary-valid status in the "wrong" directory (`done` in `active/`, `proposed` in `archive/`) is legitimate mid-sweep state and stays `board-checks.sh`'s `board-row-dropped` territory (spec Assumption 3).
- **Full suite at every gate.** `bash scripts/run-tests.sh` — never only the test files a task enumerates.

---

## File Structure

| File | Change | Responsibility |
|---|---|---|
| `scripts/lib/docket-frontmatter.sh` | Modify (add ~2 lines beside `docket_status_is_terminal`, ~line 381) | Add `docket_status_is_member` — membership over the full seven-name `DOCKET_STATUSES`, so no consumer restates the vocabulary. |
| `tests/test_docket_frontmatter.sh` | Modify (extend the change-0116 shared-vocabulary block, ~line 222) | Pin the new helper's definition and its accept/reject behavior. |
| `scripts/render-board.sh` | Modify | `sanitize()`, the upfront validation pass, per-file stderr diagnostics, the `MALFORMED` counter, membership guards on the three consumers, M4 read-back check, and the exit-3 plumbing on both emission paths. |
| `tests/test_render_board.sh` | Modify | New interior-TAB and impossible-status regression fixtures; contract flips on the pinned exit-0 asserts; new clean-path exit-0/empty-stderr pins; M4 pin. |
| `tests/test_board_refresh.sh` | Modify | One caller-integration assert: a real malformed changes dir makes `board-refresh.sh` exit non-zero and leave `BOARD.md` byte-identical. |
| `scripts/render-board.md` | Modify (`## Exit codes` at line 148; `## Behavior` at line 37) | Document exit 3, the closed M1–M4 enumeration, the diagnostic line shape, and the accepted availability cost. |

---

## Task 1: `docket_status_is_member` — full-vocabulary membership helper

**Files:**
- Modify: `scripts/lib/docket-frontmatter.sh:380-381` (immediately after `docket_status_is_terminal`)
- Test: `tests/test_docket_frontmatter.sh:222-233` (the change-0116 shared-vocabulary block)

**Interfaces:**
- Consumes: `DOCKET_STATUSES` (line 351), `_docket_array_has` (line 373) — both already defined above the insertion point.
- Produces: `docket_status_is_member STATUS` — exit 0 iff `STATUS` is one of the seven lifecycle statuses; exit 1 for anything else, **including the empty string** (`_docket_array_has` returns 1 on an empty needle). No stdout. Task 2 is its only production consumer.

- [ ] **Step 1: Write the failing tests**

In `tests/test_docket_frontmatter.sh`, immediately after the existing line
`assert "terminal helper rejects empty" '! docket_status_is_terminal ""'`, add:

```bash
assert "member-status helper is defined" 'declare -F docket_status_is_member >/dev/null'
assert "member helper accepts an active status" 'docket_status_is_member proposed'
assert "member helper accepts a terminal status" 'docket_status_is_member done'
assert "member helper rejects a status outside the vocabulary" '! docket_status_is_member bogus'
assert "member helper rejects empty" '! docket_status_is_member ""'
# An interior TAB can never match a closed-vocabulary name — this is the rejection that IS the
# sanitization for `status:` in render-board.sh (change 0259, spec assumption 4).
assert "member helper rejects a vocabulary name carrying an interior TAB" \
  '! docket_status_is_member "$(printf "done\tx")"'
assert "member helper accepts every DOCKET_STATUSES entry" \
  'for _m_s in "${DOCKET_STATUSES[@]}"; do docket_status_is_member "$_m_s" || exit 1; done'
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_docket_frontmatter.sh`
Expected: FAIL — `NOT OK - member-status helper is defined` (and the behavioral asserts below it), because `docket_status_is_member` does not exist yet.

- [ ] **Step 3: Write the minimal implementation**

In `scripts/lib/docket-frontmatter.sh`, immediately after the existing
`docket_status_is_terminal(){ _docket_array_has "$1" "${DOCKET_STATUSES_TERMINAL[@]}"; }` line, insert:

```bash
# Membership over the FULL seven-name vocabulary — the union its two siblings partition. Distinct
# from both: `_active` and `_terminal` each answer "which half", and a consumer that only needs
# "is this a status at all" would otherwise have to call both or restate the list. render-board.sh's
# malformed-file validation (change 0259) is that consumer: a status outside this vocabulary can
# never be a legal array subscript or a legal TAB-join field, so rejecting by vocabulary IS the
# sanitization — a value carrying an interior TAB or CR cannot match any of the seven names.
docket_status_is_member(){ _docket_array_has "$1" "${DOCKET_STATUSES[@]}"; }
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `bash tests/test_docket_frontmatter.sh`
Expected: PASS — every new assert reports `ok - …`, final line `PASS`.

- [ ] **Step 5: Run the full suite**

Run: `bash scripts/run-tests.sh`
Expected: green. (A new function in the shared lib is additive; nothing should move. If a lib-inventory or sentinel assert elsewhere reddens, that guard is the one to read and update — do not weaken the helper.)

- [ ] **Step 6: Commit**

```bash
git add scripts/lib/docket-frontmatter.sh tests/test_docket_frontmatter.sh
git commit -m "feat(0259): add docket_status_is_member for full-vocabulary status membership"
```

---

## Task 2: the failure contract in `render-board.sh` — validate, diagnose, exit 3

This task changes the renderer's contract and, in the same commit, updates every existing assert whose premise that contract change deletes. Splitting those apart would leave a red intermediate state, which is not a shippable task boundary.

**Files:**
- Modify: `scripts/render-board.sh` — insert `sanitize()` and the validation pass after the `resolve_deps "$CHANGES_DIR"` call (line 133) and after `ARCFILES` is populated (line 150); guard the three consumers at lines 138-142, 154, and 399-404; add exit-3 plumbing at line 247 and at end-of-file (line 415).
- Modify: `tests/test_render_board.sh` — new fixtures; contract flips at lines 227-229, 1448-1451, and 2155-2168.

**Interfaces:**
- Consumes: `docket_status_is_member` from Task 1; `field`, `int_field` from `scripts/lib/docket-frontmatter.sh` (already sourced at line 57).
- Produces:
  - `sanitize VALUE` → stdout, TAB→`\t` and CR→`\r` as visible two-character escapes. Diagnostics only.
  - `MALFORMED` — integer counter, incremented once per malformed FILE.
  - `BAD` — associative array keyed by file path; a key's presence means "excluded from every projection". Task 3 reads it.
  - Exit 3 from both the digest and the markdown path when `MALFORMED > 0`.

- [ ] **Step 1: Write the failing tests — the two new regression fixtures**

In `tests/test_render_board.sh`, immediately **before** the final
`if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi` line, append:

```bash
# --- change 0259: interior-TAB status must not shift the archive feeder; malformed => exit 3 ---
# The archive sort feeder joins `date<TAB>id<TAB>status<TAB>file` and the consumer splits it with
# `IFS=$'\t' read -r date id st f`. A TAB *inside* the status value adds a field and shifts every
# later field RIGHT — the mirror of 0143's empty-field left-shift, which `[ -n "$st" ]` cannot see.
# Rejection is by VOCABULARY: `done\tx` is not one of the seven statuses, so it never reaches the
# join, the ARC_COUNT subscript, or the SECTION subscript (spec assumption 4).
c259="$(mktemp -d)"
mkdir -p "$c259/active" "$c259/archive"
printf -- '---\nid: 1\nslug: tabbed\ntitle: Tabbed Status\nstatus: done\tx\ncreated: 2026-08-01\n---\n' \
  > "$c259/archive/2026-08-01-0001-tabbed.md"
printf -- '---\nid: 2\nslug: sibling\ntitle: Well Formed\nstatus: done\ncreated: 2026-08-02\n---\n' \
  > "$c259/archive/2026-08-02-0002-sibling.md"
printf -- '---\nid: 3\nslug: bogus-status\ntitle: Impossible Status\nstatus: bogus\npriority: medium\ncreated: 2026-08-03\n---\n' \
  > "$c259/active/0003-bogus-status.md"
printf -- '---\nid: 4\nslug: good\ntitle: Good Active\nstatus: proposed\npriority: medium\ncreated: 2026-08-04\nspec: docs/x.md\n---\n' \
  > "$c259/active/0004-good.md"

c259_md="$(bash "$SCRIPT" --changes-dir "$c259" 2>/dev/null)"; c259_md_rc=$?
c259_err="$(bash "$SCRIPT" --changes-dir "$c259" 2>&1 >/dev/null)"
c259_digest="$(bash "$SCRIPT" --changes-dir "$c259" --format digest 2>/dev/null)"; c259_dg_rc=$?

assert "0259: markdown render exits 3 when a change file is malformed" '[ "$c259_md_rc" -eq 3 ]'
assert "0259: digest render exits 3 when a change file is malformed" '[ "$c259_dg_rc" -eq 3 ]'
# No field-shifted archive row (0143's corrupt-row ERE; the YYYY-MM collapse key is not 4 digits).
assert "0259: no field-shifted archive row from the interior-TAB status" \
  '! /usr/bin/grep -qE "^\| \[[0-9]{4}\]\(archive/\) \|" <<<"$c259_md"'
assert "0259: the interior-TAB row is skipped entirely" \
  '! /usr/bin/grep -qF -- "Tabbed Status" <<<"$c259_md"'
assert "0259: the well-formed archive sibling still renders" \
  '/usr/bin/grep -qF -- "| [0002](archive/2026-08-02-0002-sibling.md) | Well Formed | 2026-08-02 |" <<<"$c259_md"'
assert "0259: the impossible-status active row is skipped" \
  '! /usr/bin/grep -qF -- "Impossible Status" <<<"$c259_md"'
assert "0259: the well-formed active row still renders" \
  '/usr/bin/grep -qF -- "Good Active" <<<"$c259_md"'
# Diagnostics: one line per malformed FILE, naming the path and the reason, with the TAB rendered
# as the visible two-character escape so the diagnostic itself cannot smuggle a control character.
assert "0259: stderr names the interior-TAB file with the TAB escaped" \
  '/usr/bin/grep -qF -- "render-board: malformed change file: $c259/archive/2026-08-01-0001-tabbed.md: status '"'"'done\\tx'"'"' is not one of the seven lifecycle statuses" <<<"$c259_err"'
assert "0259: stderr names the impossible-status file" \
  '/usr/bin/grep -qF -- "render-board: malformed change file: $c259/active/0003-bogus-status.md: status '"'"'bogus'"'"' is not one of the seven lifecycle statuses" <<<"$c259_err"'
assert "0259: stderr carries exactly one diagnostic per malformed file" \
  '[ "$(/usr/bin/grep -c "malformed change file:" <<<"$c259_err")" = 2 ]'
assert "0259: no raw TAB survives into the diagnostic stream" \
  '! /usr/bin/grep -q "$(printf "\t")" <<<"$c259_err"'
# The digest stays COMPLETE modulo skipped rows — the ready line is still emitted, and the
# well-formed proposed change is still selectable content-wise. Exit 3 is what gates the caller;
# the stdout projection is not degraded beyond the skipped rows.
assert "0259: the digest still emits a ready line naming the well-formed change" \
  '/usr/bin/grep -qxF "ready 4" <<<"$c259_digest"'
assert "0259: the digest does not count the impossible status in any backlog rollup" \
  '! /usr/bin/grep -q "^backlog bogus" <<<"$c259_digest"'
assert "0259: the malformed archive file is excluded from the done tally" \
  '/usr/bin/grep -qxF "backlog done 1" <<<"$c259_digest"'
rm -rf "$c259"

# --- change 0259: the CLEAN path stays exit 0 with empty stderr ---
# Written fresh, not claimed as existing: the golden byte-compare above discards stderr and never
# captures an exit code, so nothing previously pinned either half. Without this, "exit 3 on
# malformed" could be satisfied by a renderer that exits 3 on EVERYTHING.
clean_rc_out="$(bash "$SCRIPT" --changes-dir "$tmp" --repo o/r 2>"$tmp/clean-stderr.txt")"; clean_rc=$?
assert "0259: a clean changes dir renders markdown with exit 0" '[ "$clean_rc" -eq 0 ]'
assert "0259: a clean markdown render writes nothing to stderr" '[ ! -s "$tmp/clean-stderr.txt" ]'
assert "0259: the clean render is still non-empty" '[ -n "$clean_rc_out" ]'
bash "$SCRIPT" --changes-dir "$tmp" --repo o/r --format digest >/dev/null 2>"$tmp/clean-digest-stderr.txt"; clean_dg_rc=$?
assert "0259: a clean changes dir renders the digest with exit 0" '[ "$clean_dg_rc" -eq 0 ]'
assert "0259: a clean digest render writes nothing to stderr" '[ ! -s "$tmp/clean-digest-stderr.txt" ]'
rm -f "$tmp/clean-stderr.txt" "$tmp/clean-digest-stderr.txt"
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_render_board.sh`
Expected: FAIL — `NOT OK - 0259: markdown render exits 3 when a change file is malformed` (renderer currently exits 0), plus the diagnostic and tally asserts. The clean-path asserts may already pass; that is fine — they are the guard against over-firing, not the driver.

- [ ] **Step 3: Add `sanitize()` and the validation pass**

In `scripts/render-board.sh`, insert the following **after** the `resolve_deps "$CHANGES_DIR"` line (currently line 133) and **before** the `# Collect active files by status` comment:

```bash
# sanitize VALUE — render TAB and CR as the visible two-character escapes \t and \r. Duplicated
# from board-checks.sh:142 rather than hoisted to the shared lib: hoisting would edit
# board-checks.sh, which change 0259 holds explicitly out of scope for a three-line dedupe.
# Pure bash parameter expansion — BSD sed does not interpret \t in a pattern, so a sed form would
# be silently wrong. Used ONLY in diagnostics: a malformed value is REJECTED before it can reach a
# feeder, never laundered into one.
sanitize(){ local v="$1"; v="${v//$'\t'/\\t}"; v="${v//$'\r'/\\r}"; printf '%s' "$v"; }

# --- malformed-input validation (change 0259) -------------------------------------------------
# One upfront pass over every change file, BEFORE the section/tally loops read anything. The
# renderer's contract is "render complete output modulo skipped rows, then fail loud": a malformed
# file is excluded from every projection, diagnosed on stderr, and makes the whole run exit 3.
#
# "Malformed" is a CLOSED enumeration, deliberately narrow:
#   M1 unusable id     — int_field yields nothing (absent, empty, or non-integer).
#   M2 empty status    — field yields nothing.
#   M3 status outside DOCKET_STATUSES — which SUBSUMES any status carrying an interior TAB or CR,
#      because a control-char value can never match one of the seven closed names. Rejection by
#      vocabulary IS the sanitization for `status:`, applied before the value ever reaches the
#      archive TAB join or an ARC_COUNT/SECTION array subscript.
# Deliberately NOT malformed: a vocabulary-valid status in the "wrong" directory (`done` in
# active/ is legitimate mid-sweep state — failing on it would make docket-status's own sweep window
# a renderer failure); an unknown or absent type: (change 0127 — a type problem must never affect a
# row); a malformed created: (change 0094's 9999-99-99 sentinel already owns it).
declare -A BAD       # file path -> 1 when excluded from every projection
MALFORMED=0
mark_malformed(){    # mark_malformed FILE REASON
  BAD["$1"]=1
  MALFORMED=$(( MALFORMED + 1 ))
  printf 'render-board: malformed change file: %s: %s\n' "$1" "$2" >&2
}
```

Then, **after** the `mapfile -t ARCFILES …` line (currently line 150) — so both file lists exist —
insert the pass itself:

```bash
# Both directories, one pass. Iterates FILES, never the status array: render-board.sh's
# full-vocabulary iteration count is pinned at 2 by tests/test_render_board.sh, and membership here
# goes through docket_status_is_member so the vocabulary keeps exactly one source.
for f in ${AFILES[@]+"${AFILES[@]}"} ${ARCFILES[@]+"${ARCFILES[@]}"}; do
  v_id="$(int_field "$f" id)"
  if [ -z "$v_id" ]; then
    mark_malformed "$f" "unusable id (absent, empty, or non-integer)"
    continue
  fi
  v_st="$(field "$f" status)"
  if [ -z "$v_st" ]; then
    mark_malformed "$f" "empty status"
    continue
  fi
  if ! docket_status_is_member "$v_st"; then
    mark_malformed "$f" "status '$(sanitize "$v_st")' is not one of the seven lifecycle statuses"
    continue
  fi
done
```

Note the ordering requirement: `ARCFILES` is populated at line 150, which is **below** the
`SECTION` loop at lines 138-142. Move the `mapfile -t ARCFILES` line and its `total=` companions
up so they sit immediately after the `mapfile -t AFILES` line, then place the validation pass
directly beneath them and above the `SECTION` loop. The `total=${#AFILES[@]}` /
`total=$(( total + ${#ARCFILES[@]} ))` arithmetic is unchanged and must stay adjacent to each
other; only their position moves. `total` deliberately still counts malformed files — an
unaccounted-for file is exactly what `board-checks.sh`'s `board-row-dropped` reports, and change
0115 owns that.

- [ ] **Step 4: Guard the three consumers on `BAD`**

Replace the `SECTION` loop (currently lines 138-142) with:

```bash
for f in "${AFILES[@]}"; do
  [ -z "${BAD[$f]:-}" ] || continue
  id="$(int_field "$f" id)"; [ -n "$id" ] || continue
  st="$(field "$f" status)"; [ -n "$st" ] || continue
  SECTION["$st"]+="$id"$'\t'"$f"$'\n'
done
```

Replace the `ARC_COUNT` loop (currently line 154) with:

```bash
# The BAD gate is what narrows the 0143-era "header counts what the table drops" mismatch: an
# unusable-id archive file is now excluded from the tally as well as from the table, so the
# remaining mismatch can only come from PLACEMENT divergence — board-row-dropped's territory.
for f in "${ARCFILES[@]}"; do
  [ -z "${BAD[$f]:-}" ] || continue
  st="$(field "$f" status)"; [ -n "$st" ] || continue
  ARC_COUNT["$st"]=$(( ${ARC_COUNT[$st]:-0} + 1 ))
done
```

Replace the archive sort feeder's producer subshell (currently lines 399-404, the
`for f in "${ARCFILES[@]}"; do base=… done | sort …` block) with:

```bash
    for f in "${ARCFILES[@]}"; do
      [ -z "${BAD[$f]:-}" ] || continue
      base="$(basename "$f")"; d="${base:0:10}"; id="$(int_field "$f" id)"; st="$(field "$f" status)"
      [ -n "$id" ] && [ -n "$st" ] || continue
      printf '%s\t%s\t%s\t%s\n' "$d" "$id" "$st" "$f"
    done | sort -t$'\t' -k1,1r -k2,2nr
```

- [ ] **Step 5: Add the exit-3 plumbing on both emission paths**

In the digest block, replace the terminating `exit 0` (currently line 247) with:

```bash
  [ "$MALFORMED" -eq 0 ] || exit 3
  exit 0
```

At the very end of the file (after the markdown path's final `fi`), append:

```bash

# Render-complete, then fail loud (change 0259). stdout above is the full projection modulo the
# skipped rows; a non-zero malformed count makes the RUN non-zero so every caller gates honestly.
# Callers need no new branching, and this was verified against each of them rather than assumed:
#   board-refresh.sh    — captures the render into a temp file and, on any non-zero rc, leaves
#                         BOARD.md byte-identical and propagates the code (board-refresh.sh:108-112).
#   docket-status.sh    — backlog_pass tests `if ! out="$(… 2>&2)"`, a BARE non-zero check, so it
#                         reports "backlog digest failed; continuing without it" and emits no
#                         digest lines; digest_only_pass then fails closed on that empty capture.
# The consequence is deliberate and is the accepted cost of this change (spec assumption 1): ONE
# malformed file anywhere freezes BOARD.md and halts autonomous selection until a human fixes the
# named file. That is the trade — corruption-adjacent state stops the loop loudly rather than
# letting it run beside an unrenderable backlog. Exit 3, not 1 or 2: 2 is the established
# CLI-argument-error code and 1 is indistinguishable from an unexpected crash.
[ "$MALFORMED" -eq 0 ] || exit 3
exit 0
```

- [ ] **Step 6: Run the new tests to verify they pass**

Run: `bash tests/test_render_board.sh`
Expected: the new `0259:` asserts all report `ok`. The three pinned blocks from Step 7 still report `NOT OK` — that is the expected next step, not a regression.

- [ ] **Step 7: Flip the invalidated contract asserts**

Three existing blocks pin the OLD contract. Each is updated in place, keeping every content-level
assertion verbatim; only the exit-code and stderr-shape claims move. Read what each block GUARDS
before touching it — none of them is deleted.

**(a) `tests/test_render_board.sh:1448` — the malformed-id block.** Replace the single exit assert
and add a diagnostic assert, keeping both row-skip asserts unchanged:

```bash
mout="$("$SCRIPT" --changes-dir "$tmp" 2>/tmp/render-board-stderr.$$)"; mrc=$?
# Contract flip (change 0259): a malformed-id file is still SKIPPED row-wise — that assertion is
# unchanged — but the run no longer exits 0. Exiting 0 here was the silent-corruption channel 0259
# exists to close.
assert "render-board exits 3 with a malformed-id file present" '[ "$mrc" -eq 3 ]'
assert "render-board skips malformed active row (title absent)"  '! printf "%s" "$mout" | grep -q "Bad Active"'
assert "render-board skips malformed archive row (title absent)" '! printf "%s" "$mout" | grep -q "Bad Archive"'
assert "render-board diagnoses both malformed-id files on stderr" \
  '[ "$(/usr/bin/grep -c "malformed change file:" /tmp/render-board-stderr.$$)" = 2 ]'
rm -f "$tmp/active/0099-bad.md" "$tmp/archive/2026-06-01-0098-badarc.md" /tmp/render-board-stderr.$$
```

**(b) `tests/test_render_board.sh:2155-2168` — the change-0143 block.** The fixture carries an
empty-`id` archive file (`status: done`) and empty-`status` files, so all three renders now exit 3.
Replace the stderr-clean assert and the two tally asserts; leave the corrupt-row, well-formed-row,
`backlog proposed 1`, and `ready 8` asserts byte-identical:

```bash
c143_md="$(bash "$SCRIPT" --changes-dir "$c143" 2>/dev/null)"; c143_md_rc=$?
c143_err="$(bash "$SCRIPT" --changes-dir "$c143" 2>&1 >/dev/null)"
c143_digest="$(bash "$SCRIPT" --changes-dir "$c143" --format digest 2>/dev/null)"

assert "0143: no corrupt archive row with an empty basename" \
  '! /usr/bin/grep -qE "^\| \[[0-9]{4}\]\(archive/\) \|" <<<"$c143_md"'
assert "0143: the well-formed archive row still renders" \
  '/usr/bin/grep -qF -- "| [0006](archive/2026-07-03-0006-ok.md) | Fine | 2026-07-03 |" <<<"$c143_md"'
# Contract flip (change 0259): the fixture's files ARE malformed (empty id, empty status), so the
# render is now expected to exit 3 and to say why. What the original assert guarded — no subscript
# abort, no printf/sed noise — is PRESERVED by narrowing rather than deleting: stderr must contain
# the 0259 diagnostics and NOTHING else.
assert "0143: render exits 3 on the malformed fixture" '[ "$c143_md_rc" -eq 3 ]'
assert "0143: render stderr carries only 0259 diagnostics (no subscript abort, no printf/sed noise)" \
  '[ -z "$(/usr/bin/grep -v "^render-board: malformed change file: " <<<"$c143_err" | /usr/bin/grep -v "^$")" ]'
# CONTRACT FLIP, deliberate and loud. The original pair pinned "Archive — done (2)" / "backlog
# done 2" precisely so no silent fix to the id-guard-less ARC_COUNT loop could land — change 0259
# IS that fix, landing loudly. The empty-ID archive file (status: done) is now excluded by M1, so
# the tally drops to 1. The empty-STATUS files were already uncounted by the pre-existing guards
# and move no tally. The residual header/table mismatch this pair once described now arises only
# from PLACEMENT divergence, which stays board-row-dropped's report (change 0115).
assert "0259 flip: the archive header tally excludes the unusable-id file" \
  '/usr/bin/grep -qF -- "Archive — done (1)" <<<"$c143_md"'
assert "0259 flip: digest counts only the usable-id done file" \
  '/usr/bin/grep -qxF "backlog done 1" <<<"$c143_digest"'
assert "0143: digest still reaches the active change behind the empty-status file" \
  '/usr/bin/grep -qxF "backlog proposed 1" <<<"$c143_digest"'
assert "0143: the ready queue line is not emptied by the tally abort" \
  '/usr/bin/grep -qxF "ready 8" <<<"$c143_digest"'
rm -rf "$c143"
```

**(c) `tests/test_render_board.sh:227-229` — the golden byte-compare.** No assert here is
invalidated (the golden fixture is clean), and Step 1 already added the fresh exit-0/empty-stderr
pins for it. Leave this block unchanged.

- [ ] **Step 8: Run the file's tests to verify they pass**

Run: `bash tests/test_render_board.sh`
Expected: PASS — final line `PASS`, no `NOT OK` lines.

- [ ] **Step 9: Run the full suite**

Run: `bash scripts/run-tests.sh`
Expected: green. `tests/test_board_checks.sh` and `tests/test_docket_status.sh` both build fixtures
that exercise `render-board.sh`; if either reddens, read whether its fixture is deliberately
malformed. A fixture that is malformed by design gets the same treatment as (b) above — flip the
exit expectation, preserve the content assertions. A fixture that is malformed by ACCIDENT is a
fixture bug this change has just surfaced: fix the fixture.

- [ ] **Step 10: Commit**

```bash
git add scripts/render-board.sh tests/test_render_board.sh
git commit -m "fix(0259): reject malformed change files by vocabulary and exit 3"
```

---

## Task 3: M4 — the archive feeder read-back check

**Files:**
- Modify: `scripts/render-board.sh` — the archive consumer loop (`while IFS=$'\t' read -r date id st f`, currently line 388)
- Test: `tests/test_render_board.sh` (append before the final PASS/FAIL line)

**Interfaces:**
- Consumes: `docket_status_is_member` (Task 1), `mark_malformed` / `MALFORMED` (Task 2).
- Produces: no new names. Strengthens the existing loop's guard so a tuple that arrives shifted is counted malformed and skipped rather than rendered.

**Why this exists even though M1–M3 already run:** M1–M3 validate the values the producer READS. M4
validates the tuple the consumer RECEIVES. They are different failure surfaces — a TAB in a
*filename* (not a frontmatter value) reaches the join without passing through any frontmatter read,
and no upfront value check can see it. This is belt-and-suspenders against a future control-character
path, not a duplicate of M3.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_render_board.sh`, before the final PASS/FAIL line:

```bash
# --- change 0259 (M4): a shifted feeder tuple is caught at the CONSUMER, not only at the producer ---
# M1-M3 validate the values the producer reads from frontmatter. M4 validates the tuple the
# consumer receives, catching a control-character path that never passes through a frontmatter read
# at all — here, a TAB in the FILENAME. Without M4 the split misassigns fields and the loop renders
# a row built from the wrong ones.
m4="$(mktemp -d)"
mkdir -p "$m4/active" "$m4/archive"
printf -- '---\nid: 1\nslug: ok\ntitle: Fine Row\nstatus: done\ncreated: 2026-08-01\n---\n' \
  > "$m4/archive/2026-08-01-0001-ok.md"
printf -- '---\nid: 2\nslug: tabname\ntitle: Tab Name\nstatus: done\ncreated: 2026-08-02\n---\n' \
  > "$m4/archive/$(printf '2026-08-02-0002-tab\tname.md')"
m4_md="$(bash "$SCRIPT" --changes-dir "$m4" 2>/dev/null)"; m4_rc=$?
m4_err="$(bash "$SCRIPT" --changes-dir "$m4" 2>&1 >/dev/null)"
assert "0259 M4: the well-formed archive row still renders alongside the shifted tuple" \
  '/usr/bin/grep -qF -- "| [0001](archive/2026-08-01-0001-ok.md) | Fine Row | 2026-08-01 |" <<<"$m4_md"'
assert "0259 M4: a shifted feeder tuple produces no rendered row" \
  '! /usr/bin/grep -qF -- "Tab Name" <<<"$m4_md"'
assert "0259 M4: a shifted feeder tuple is diagnosed on stderr" \
  '/usr/bin/grep -qF -- "archive feeder tuple" <<<"$m4_err"'
assert "0259 M4: a shifted feeder tuple makes the run exit 3" '[ "$m4_rc" -eq 3 ]'
rm -rf "$m4"
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_render_board.sh`
Expected: FAIL — `NOT OK - 0259 M4: a shifted feeder tuple is diagnosed on stderr` and
`NOT OK - 0259 M4: a shifted feeder tuple makes the run exit 3`.

- [ ] **Step 3: Add the read-back check**

In `scripts/render-board.sh`, in the archive consumer loop, replace the loop's opening guard
(`[ -n "$id" ] || continue`) with:

```bash
  while IFS=$'\t' read -r date id st f; do
    [ -n "$id" ] || continue
    # M4 (change 0259) — validate the tuple as RECEIVED, not only the values as read. M1-M3 cannot
    # see a control character that never passed through a frontmatter read (a TAB in a FILENAME
    # reaches the join directly), and a shifted split silently rebinds every later field. Two
    # conjuncts, each catching a different shift: a status that is not in the closed vocabulary
    # means the tuple slid, and a path that does not resolve on disk means `f` is a fragment.
    if ! docket_status_is_member "$st" || [ ! -e "$f" ]; then
      mark_malformed "${f:-<unresolvable>}" "archive feeder tuple did not read back cleanly (status '$(sanitize "$st")')"
      continue
    fi
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_render_board.sh`
Expected: PASS — final line `PASS`.

- [ ] **Step 5: Run the full suite**

Run: `bash scripts/run-tests.sh`
Expected: green.

- [ ] **Step 6: Commit**

```bash
git add scripts/render-board.sh tests/test_render_board.sh
git commit -m "fix(0259): validate the archive feeder tuple at the consumer (M4)"
```

---

## Task 4: caller-integration pin and the contract document

**Files:**
- Modify: `tests/test_board_refresh.sh` (append a new numbered case after the existing empty-render case, ~line 240)
- Modify: `scripts/render-board.md:148-154` (`## Exit codes`) and `:37` (`## Behavior`)

**Interfaces:**
- Consumes: the exit-3 contract from Tasks 2-3.
- Produces: nothing new in code — this task makes the caller wiring an asserted fact rather than a
  claim in a comment, and states the contract where a reader will look for it.

**Why the integration assert is not redundant with test #9:** test #9 injects a *mock* renderer that
exits 7 through the `RENDER_BOARD` seam. It proves board-refresh honors a non-zero code; it cannot
prove the REAL renderer produces one. The learning that governs this change — a new exit code is a
contract change for every existing caller, and a documented exit-code table constrains nobody —
is discharged only by asserting the real wiring end to end.

- [ ] **Step 1: Write the failing test**

In `tests/test_board_refresh.sh`, after the existing empty-render case (the block ending with the
`"empty render: pre-existing BOARD.md untouched (byte-identical)"` assert), append:

```bash
# --- change 0259: the REAL renderer's exit 3 reaches board-refresh, and BOARD.md survives it ---
# Test #9 above proves board-refresh honors a non-zero code from an INJECTED stub. This proves the
# real render-board.sh produces one on real malformed input, and that the two are wired together —
# the end-to-end claim a documented exit-code table cannot make on its own.
mkdir -p "$tmp/active"
cat > "$tmp/active/0099-malformed.md" <<'EOF'
---
id: 99
slug: malformed
title: Malformed Status
status: not-a-status
priority: medium
depends_on: []
---
EOF
board_before="$(cat "$tmp/BOARD.md")"
"$DOCKET_BASH_PATH" "$SCRIPT" --changes-dir "$tmp" --surfaces inline >"$work/m-out.txt" 2>"$work/m-err.txt"
rc_malformed=$?
assert "0259: board-refresh exits non-zero when the real renderer reports malformed input" \
  '[ "$rc_malformed" -ne 0 ]'
assert "0259: board-refresh propagates the renderer's exit 3 verbatim" '[ "$rc_malformed" -eq 3 ]'
assert "0259: BOARD.md is left byte-identical after a malformed render" \
  '[ "$(cat "$tmp/BOARD.md")" = "$board_before" ]'
assert "0259: the renderer's diagnostic names the offending file on board-refresh's stderr" \
  '/usr/bin/grep -qF -- "malformed change file: $tmp/active/0099-malformed.md" "$work/m-err.txt"'
rm -f "$tmp/active/0099-malformed.md"
```

If `$tmp/BOARD.md` does not exist at this point in the file (it is created by earlier cases), add
`printf 'sentinel\n' > "$tmp/BOARD.md"` immediately before `board_before=…` so the
byte-identical assert has a known prior state to compare against.

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_board_refresh.sh`
Expected: without Tasks 2-3 this would fail; with them applied it should already PASS. Run it to
confirm which — if it passes immediately, that is the wiring being confirmed, and the assert stays
as the regression pin. If it fails, the wiring is the bug: fix it before moving on.

- [ ] **Step 3: Update the contract document**

In `scripts/render-board.md`, replace the `## Exit codes` table (lines 150-153) with:

```markdown
| Code | Meaning |
|---|---|
| 0 | The requested projection (the markdown board, or the digest under `--format digest`) was written to stdout successfully, and every change file was well-formed. |
| 2 | Missing or invalid argument (`--changes-dir` absent or not a directory; unknown flag; unknown `--format` value). |
| 3 | One or more change files were malformed (change 0259). stdout still carries the complete projection **modulo the skipped rows**; stderr carries one `render-board: malformed change file: <path>: <reason>` line per offending file. |
```

Then, at the end of the `## Behavior` section, add:

```markdown
### Malformed input — render complete, then fail loud (change 0259)

A single upfront pass validates every change file in `active/` and `archive/` before any projection
is built. A file is **malformed** under a closed enumeration:

| | Condition |
|---|---|
| M1 | Unusable `id:` — absent, empty, or non-integer. |
| M2 | Empty `status:`. |
| M3 | `status:` outside the seven-name `DOCKET_STATUSES` vocabulary. This **subsumes** any status carrying an interior TAB or CR: a control-character value can never match a closed vocabulary name, so rejection-by-vocabulary *is* the sanitization — no such value ever reaches the archive TAB join or an `ARC_COUNT`/`SECTION` array subscript. |
| M4 | The archive sort feeder's tuple did not read back cleanly at the consumer — a defence against a control-character path that never passes through a frontmatter read at all (a TAB in a *filename*), which M1–M3 cannot see. |

A malformed file is excluded from every projection — the section table, the per-status tallies, and
the digest — and diagnosed once on stderr. It is still counted in the header's total-changes number:
an unaccounted-for file is exactly the state `board-checks.sh`'s `board-row-dropped` exists to
report (change 0115).

**Deliberately not malformed:** a vocabulary-valid status in the "wrong" directory (`done` in
`active/`, `proposed` in `archive/`) — legitimate mid-sweep state, and failing on it would turn
`docket-status`'s own sweep window into a renderer failure; an unknown or absent `type:` (change
0127); a malformed `created:` (change 0094's `9999-99-99` sentinel already owns it).

**Callers need no new branching, and the accepted cost is real.** `board-refresh.sh` already leaves
`BOARD.md` byte-identical and propagates any non-zero code; `docket-status.sh`'s `backlog_pass`
tests a bare non-zero and emits no digest lines, after which `digest_only_pass` fails closed on the
empty capture. The consequence is that **one malformed file anywhere freezes `BOARD.md` and halts
autonomous selection** until a human fixes the named file — accepted deliberately, because
corruption-adjacent state should stop the loop loudly rather than let it run beside an unrenderable
backlog.
```

- [ ] **Step 4: Run the full suite**

Run: `bash scripts/run-tests.sh`
Expected: green. Several test files assert on `scripts/*.md` contract structure; if a doc-shape
guard reddens, read what it pins before editing the prose back.

- [ ] **Step 5: Commit**

```bash
git add tests/test_board_refresh.sh scripts/render-board.md
git commit -m "docs(0259): document the exit-3 failure contract and pin the caller wiring"
```

---

## Self-Review

**1. Spec coverage.**

| Spec requirement | Task |
|---|---|
| Failure contract: render-complete, then fail loud | Task 2 Step 5 |
| Per-file stderr diagnostic, sanitized | Task 2 Steps 3, 5 |
| Exit 3 on both markdown and digest paths | Task 2 Step 5 |
| Exit-code vocabulary in the header comment / contract | Task 2 Step 5, Task 4 Step 3 |
| M1 unusable id | Task 2 Step 3 |
| M2 empty status | Task 2 Step 3 |
| M3 status outside `DOCKET_STATUSES` (subsumes interior TAB/CR) | Task 1 + Task 2 Step 3 |
| M4 feeder read-back mismatch | Task 3 |
| `sanitize()` copied from `board-checks.sh:142`, diagnostics only | Task 2 Step 3 |
| Tally subscripts see closed-vocabulary keys only | Task 2 Step 4 |
| Interior-TAB-status regression fixture | Task 2 Step 1 |
| Impossible-status fixture | Task 2 Step 1 |
| Contract flips on existing exit-0 pins, incl. the `ARC_COUNT` tally override | Task 2 Step 7 |
| New clean-path exit-0 / empty-stderr pins | Task 2 Step 1 |
| Caller-integration assert in `test_board_refresh.sh` | Task 4 Step 1 |
| `board-checks.sh` untouched | Global Constraints (no task modifies it) |

No gaps.

**2. Placeholder scan.** No TBD, no "add error handling", no "similar to Task N". Every code step
carries the literal text to insert; every test step carries the literal assert and the exact
command plus expected outcome.

**3. Type consistency.** `docket_status_is_member` is defined in Task 1 and called under that exact
name in Task 2 Step 3 and Task 3 Step 3. `mark_malformed FILE REASON`, `MALFORMED`, `BAD`, and
`sanitize VALUE` are defined in Task 2 Step 3 and used under those exact names in Task 2 Steps 4-5
and Task 3 Step 3. The diagnostic string emitted by `mark_malformed` — `render-board: malformed
change file: <path>: <reason>` — matches the substrings asserted in Task 2 Step 1, Task 2 Step 7(a),
Task 3 Step 1, and Task 4 Step 1, and matches the shape documented in Task 4 Step 3.

**One sequencing hazard worth naming for the implementer:** Task 2 Step 3 requires moving the
`mapfile -t ARCFILES` line (and its `total=` companions) above the `SECTION` loop so the validation
pass can see both file lists. That is a real reordering of live code, not a cosmetic move — verify
`total` still counts `active` + `archive` and that the golden byte-compare still passes before
proceeding to Step 4.
