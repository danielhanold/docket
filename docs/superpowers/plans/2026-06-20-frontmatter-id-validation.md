# Validate numeric `id` across the frontmatter script family — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Harden the docket script family against a malformed (non-integer) `id:` frontmatter field — one shared `int_field` helper, adopted by role: renderers + the shared scan skip a bad row, validators flag it with a first-class warn-only `malformed-id` finding, `terminal-publish.sh` fails closed on a non-integer CLI `--id`/`--adr`.

**Architecture:** A pure `int_field FILE KEY` accessor (sibling of `field`) returns the value only when it matches `^[0-9]+$`, else empty — so every existing `[ -n "$id" ] || continue` guard now also skips a malformed id with zero new control flow. Validators additionally compare raw-vs-integer and `emit malformed-id`. The publisher validates its CLI arg at parse time.

**Tech Stack:** Bash (`set -uo pipefail`); the repo's hand-rolled `assert` harnesses (`tests/test_*.sh`), each sourcing the lib or invoking a script and grepping output. Offline.

## Global Constraints

- `int_field` is **pure** — no side effects on source, no diagnostics (the lib header guarantees this). Empty-string return is the only signal.
- Tolerant integer test `^[0-9]+$` (non-negative). Reject ``/`abc`/`1.5`/`7x`/`-3`/`+3`/`1e3`/trailing-space; accept `7`/`007`/`0`.
- Renderers + the shared `resolve_deps`/`readiness` scan: **skip** a malformed row (existing `|| continue`), no finding — one bad file must never blank the board/index.
- Validators (`board-checks.sh`, `adr-checks.sh`): **first-class warn-only `malformed-id` finding**, same `emit` shape, sorted with the rest; escalates to exit 1 only under the pre-existing `--strict`. No other exit-code semantics.
- `terminal-publish.sh`: `--id`/`--adr` are CLI args (not frontmatter) → **fail closed** (`die`) on a non-integer, matching its existing `die`-on-bad-arg style.
- Scope: `id:` only. Do NOT validate other numeric fields (`depends_on`, `adrs:`, `change:`).
- Harden **every** id-read site (reconcile pinned the full inventory — see each task).

---

## File Structure

- `scripts/lib/docket-frontmatter.sh` — add `int_field`; adopt at `resolve_deps` (L43, L48) + `readiness` (L71).
- `scripts/render-board.sh` — adopt at L52 (SECTION), L164 (done-id list), L182 (archive builder).
- `scripts/render-adr-index.sh` — adopt at L38 (scan).
- `scripts/board-checks.sh` — `malformed-id` finding at the L50 scan; `int_field` at L92 (cycle `cid`).
- `scripts/adr-checks.sh` — `malformed-id` finding at the L35 scan (before the L42 `MAXID` arithmetic).
- `scripts/terminal-publish.sh` — fail-closed `--id`/`--adr` guard after the mutual-exclusivity checks (~L46).
- Tests: extend `test_docket_frontmatter.sh`, `test_render_board.sh`, `test_render_adr_index.sh`, `test_board_checks.sh`, `test_adr_checks.sh`; create `test_terminal_publish.sh`.

(Line numbers are the reconcile-pinned `origin/main@ad799b1` positions; confirm with `grep -nE 'field .*\bid\b'` before editing each file.)

---

### Task 1: `int_field` helper + lib unit test

**Files:**
- Modify: `scripts/lib/docket-frontmatter.sh` (add `int_field` after `list_field`)
- Test: `tests/test_docket_frontmatter.sh`

**Interfaces:**
- Consumes: existing `field FILE KEY`.
- Produces: `int_field FILE KEY` → the value iff `^[0-9]+$`, else `""`.

- [ ] **Step 1: Write the failing test**

In `tests/test_docket_frontmatter.sh`, after `source "$LIB"` (~line 14), add a block (uses its own tiny fixtures so it is order-independent):

```bash
# --- int_field: integer-only accessor ---
ifd="$(mktemp -d)"
printf -- '---\nid: 7\n---\n'    > "$ifd/ok.md"
printf -- '---\nid: 007\n---\n'  > "$ifd/pad.md"
printf -- '---\nid: 0\n---\n'    > "$ifd/zero.md"
printf -- '---\nid: abc\n---\n'  > "$ifd/abc.md"
printf -- '---\nid: 1.5\n---\n'  > "$ifd/dot.md"
printf -- '---\nid: 7x\n---\n'   > "$ifd/trail.md"
printf -- '---\nid: -3\n---\n'   > "$ifd/neg.md"
printf -- '---\nslug: x\n---\n'  > "$ifd/none.md"
assert "int_field accepts 7"        '[ "$(int_field "$ifd/ok.md" id)" = "7" ]'
assert "int_field accepts 007"      '[ "$(int_field "$ifd/pad.md" id)" = "007" ]'
assert "int_field accepts 0"        '[ "$(int_field "$ifd/zero.md" id)" = "0" ]'
assert "int_field rejects abc"      '[ -z "$(int_field "$ifd/abc.md" id)" ]'
assert "int_field rejects 1.5"      '[ -z "$(int_field "$ifd/dot.md" id)" ]'
assert "int_field rejects 7x"       '[ -z "$(int_field "$ifd/trail.md" id)" ]'
assert "int_field rejects -3"       '[ -z "$(int_field "$ifd/neg.md" id)" ]'
assert "int_field empty when unset" '[ -z "$(int_field "$ifd/none.md" id)" ]'
rm -rf "$ifd"
```

- [ ] **Step 2: Run to verify it fails**

Run: `bash tests/test_docket_frontmatter.sh`
Expected: the `int_field …` assertions print `NOT OK` (function not defined → command substitution empty, so the "accepts" asserts fail), script ends `FAIL`.

- [ ] **Step 3: Implement `int_field`**

In `scripts/lib/docket-frontmatter.sh`, immediately after the `list_field(){ … }` function (before `has_section`), add:

```bash
# int_field FILE KEY — like field(), but returns the value ONLY when it is a well-formed
# non-negative integer (^[0-9]+$); empty string otherwise. Pure; no side effects on source.
int_field(){
  local v; v="$(field "$1" "$2")"
  case "$v" in (''|*[!0-9]*) printf '' ;; (*) printf '%s' "$v" ;; esac
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `bash tests/test_docket_frontmatter.sh`
Expected: all `int_field …` assertions `ok`, final `PASS`.

- [ ] **Step 5: Commit**

```bash
git add scripts/lib/docket-frontmatter.sh tests/test_docket_frontmatter.sh
git commit -m "feat(0032): int_field — integer-only frontmatter accessor

Pure ^[0-9]+$ accessor (sibling of field); empty on non-integer. Foundation for
the by-role id-validation adoption across the script family.

Claude-Session: https://claude.ai/code/session_011F2ADM8tyKirJc6Ku6nq35"
```

---

### Task 2: Renderers + shared scan skip a malformed id

**Files:**
- Modify: `scripts/lib/docket-frontmatter.sh` (resolve_deps L43, L48; readiness L71)
- Modify: `scripts/render-board.sh` (L52, L164, L182)
- Modify: `scripts/render-adr-index.sh` (L38)
- Test: `tests/test_render_board.sh`, `tests/test_render_adr_index.sh`

**Interfaces:**
- Consumes: `int_field` (Task 1).
- Produces: no signature change — these scans now skip a malformed-id file instead of forming a junk key / feeding `pad`.

- [ ] **Step 1: Write the failing tests**

In `tests/test_render_board.sh`, after the existing fixture tree is written but before/around the first render assertion, add a malformed file in BOTH active and archive and assert it is skipped (find the existing `out="$("$SCRIPT" --changes-dir "$tmp")"` render; add this as a separate focused render against a copy so it doesn't disturb the golden compare — simplest: add it after the golden assertions, rendering the same `$tmp` after injecting the bad files):

```bash
# --- malformed id is skipped (active + archive), renderer still succeeds ---
printf -- '---\nid: abc\nslug: bad\ntitle: Bad Active\nstatus: proposed\npriority: low\ndepends_on: []\n---\n' > "$tmp/active/0099-bad.md"
printf -- '---\nid: nope\nslug: badarc\ntitle: Bad Archive\nstatus: done\npriority: low\ndepends_on: []\n---\n' > "$tmp/archive/2026-06-01-0098-badarc.md"
mout="$("$SCRIPT" --changes-dir "$tmp" 2>/tmp/render-board-stderr.$$)"; mrc=$?
assert "render-board exits 0 with a malformed-id file present" '[ "$mrc" -eq 0 ]'
assert "render-board skips malformed active row (title absent)"  '! printf "%s" "$mout" | grep -q "Bad Active"'
assert "render-board skips malformed archive row (title absent)" '! printf "%s" "$mout" | grep -q "Bad Archive"'
rm -f "$tmp/active/0099-bad.md" "$tmp/archive/2026-06-01-0098-badarc.md" /tmp/render-board-stderr.$$
```

In `tests/test_render_adr_index.sh`, after the golden assertions, add:

```bash
# --- malformed ADR id is skipped, renderer still succeeds ---
printf -- '---\nid: xyz\nslug: bad\ntitle: Bad ADR\nstatus: Accepted\ndate: 2026-06-01\nsupersedes: []\nreverses: []\nrelates_to: []\nchange:\n---\n## Decision\nx.\n' > "$tmp/0099-bad.md"
aout="$("$SCRIPT" --adrs-dir "$tmp" 2>/dev/null)"; arc=$?
assert "render-adr-index exits 0 with malformed-id ADR present" '[ "$arc" -eq 0 ]'
assert "render-adr-index skips malformed ADR (title absent)" '! printf "%s" "$aout" | grep -q "Bad ADR"'
rm -f "$tmp/0099-bad.md"
```

- [ ] **Step 2: Run to verify they fail**

Run: `bash tests/test_render_board.sh; bash tests/test_render_adr_index.sh`
Expected: at least the "skips malformed … row" assertions print `NOT OK` (current code forms a junk key and the malformed title leaks into the rendered table / index; or `pad` errors). Each file ends `FAIL`.

- [ ] **Step 3: Adopt `int_field` in the lib's two scans**

In `scripts/lib/docket-frontmatter.sh`, in `resolve_deps`, change BOTH id reads (the `# pass 1` and `# pass 2` loops):

```bash
    id="$(field "$f" id)"; [ -n "$id" ] || continue
```
to:
```bash
    id="$(int_field "$f" id)"; [ -n "$id" ] || continue
```

And in `readiness()`, change:
```bash
  id="$(field "$f" id)"
```
to:
```bash
  id="$(int_field "$f" id)"
```

- [ ] **Step 4: Adopt `int_field` in `render-board.sh` (all three sites)**

- L52 (SECTION builder): `id="$(field "$f" id)"; [ -n "$id" ] || continue` → `id="$(int_field "$f" id)"; [ -n "$id" ] || continue`
- L164 (done-id list): the `… && field "$f" id; done | sort -n)` → `… && int_field "$f" id; done | sort -n)`
- L182 (archive builder): `base="$(basename "$f")"; d="${base:0:10}"; id="$(field "$f" id)"` → `… id="$(int_field "$f" id)"`

(The downstream `[ -n "$id" ] || continue` at L165/L177 then skips a malformed row.)

- [ ] **Step 5: Adopt `int_field` in `render-adr-index.sh`**

- L38: `id="$(field "$f" id)"; [ -n "$id" ] || continue` → `id="$(int_field "$f" id)"; [ -n "$id" ] || continue`

- [ ] **Step 6: Run to verify the renderer tests pass**

Run: `bash tests/test_render_board.sh; bash tests/test_render_adr_index.sh`
Expected: both end `PASS` — the golden idempotence assertions still pass (valid fixtures unchanged) AND the new skip assertions pass.

- [ ] **Step 7: Commit**

```bash
git add scripts/lib/docket-frontmatter.sh scripts/render-board.sh scripts/render-adr-index.sh tests/test_render_board.sh tests/test_render_adr_index.sh
git commit -m "feat(0032): renderers + shared scan skip a malformed id

resolve_deps (2 passes) + readiness + render-board (3 sites) + render-adr-index
adopt int_field; a non-integer id row is skipped instead of forming a junk key /
feeding pad. No finding (renderers are not the health surface).

Claude-Session: https://claude.ai/code/session_011F2ADM8tyKirJc6Ku6nq35"
```

---

### Task 3: Validators emit a first-class `malformed-id` finding

**Files:**
- Modify: `scripts/board-checks.sh` (L50 scan emits; L92 cycle `cid` uses int_field)
- Modify: `scripts/adr-checks.sh` (L35 scan emits, before the L42 MAXID arithmetic)
- Test: `tests/test_board_checks.sh`, `tests/test_adr_checks.sh`

**Interfaces:**
- Consumes: `int_field` (Task 1), existing `field`, `emit CHECK ID MSG`.
- Produces: a new warn-only finding id `malformed-id` (id column = the raw non-integer value).

- [ ] **Step 1: Write the failing tests**

In `tests/test_adr_checks.sh`, after the existing `mkadr`-based blocks, add (write the malformed file directly — `mkadr` cannot pad a non-integer):

```bash
# ===== malformed-id: a non-integer id is flagged, valid neighbours unaffected =====
d="$(mktemp -d)"
mkadr "$d" 1 Accepted "[]" "[]" "[]"
mkadr "$d" 2 Accepted "[]" "[]" "[]"
printf -- '---\nid: abc\nslug: bad\ntitle: Bad\nstatus: Accepted\ndate: 2026-06-01\nsupersedes: []\nreverses: []\nrelates_to: []\nchange:\n---\n## Decision\nx.\n' > "$d/9001-bad.md"
out="$(bash "$SCRIPT" --adrs-dir "$d" 2>/dev/null)"; rc=$?
assert "adr malformed-id flagged on non-integer id 'abc'" 'has_finding "$out" malformed-id abc'
assert "adr malformed-id: no numbering-gap false positive (valid 1,2 only)" '! has_finding "$out" adr-numbering-gap 1'
assert "adr malformed-id: script still exits 0 (warn-only)" '[ "$rc" -eq 0 ]'
rm -rf "$d"
# control: clean ledger emits no malformed-id
d="$(mktemp -d)"
mkadr "$d" 1 Accepted "[]" "[]" "[]"
out="$(bash "$SCRIPT" --adrs-dir "$d" 2>/dev/null)"
assert "adr malformed-id silent on clean ledger" '! has_finding "$out" malformed-id 1'
rm -rf "$d"
```

In `tests/test_board_checks.sh`, add a malformed change file to a repo and assert the finding (use the harness's `new_repo`; write the bad file on the docket checkout under `docs/changes/active`):

```bash
# ===== malformed-id: a non-integer change id is flagged =====
read -r work origin <<<"$(new_repo)"
printf -- '---\nid: abc\nslug: bad\ntitle: Bad\nstatus: proposed\npriority: low\ndepends_on: []\n---\n' > "$work/docs/changes/active/0001-bad.md"
out="$(METADATA_BRANCH=docket INTEGRATION_BRANCH=main bash "$SCRIPT" --changes-dir "$work/docs/changes" 2>/dev/null)"
assert "board malformed-id flagged on non-integer change id 'abc'" 'has_finding "$out" malformed-id abc'
```

(Match the exact invocation form the other `board-checks` assertions in this file already use — copy their env/flags; the line above shows the finding assertion, adapt the invocation to the file's established pattern.)

- [ ] **Step 2: Run to verify they fail**

Run: `bash tests/test_adr_checks.sh; bash tests/test_board_checks.sh`
Expected: the `malformed-id flagged …` assertions print `NOT OK` (no such finding today; `adr-checks` may also emit a stderr `integer expression expected` from the L42 `[ "$id" -gt … ]` on `abc`). Each ends `FAIL`.

- [ ] **Step 3: Emit `malformed-id` in `adr-checks.sh`**

In `scripts/adr-checks.sh`, replace the scan-loop head:

```bash
for f in "${FILES[@]}"; do
  id="$(field "$f" id)"; [ -n "$id" ] || continue
  EXISTS["$id"]=1
```
with:
```bash
for f in "${FILES[@]}"; do
  raw="$(field "$f" id)"; id="$(int_field "$f" id)"
  if [ -z "$id" ]; then
    [ -n "$raw" ] && emit malformed-id "$raw" "non-integer id '$raw' in $(basename "$f")"
    continue
  fi
  EXISTS["$id"]=1
```

Note `emit`/`FINDINGS` are defined just below the scan loop today; move the `emit(){ … }` definition and `FINDINGS=""` to ABOVE the scan loop (right after the `declare -A EXISTS …` line) so `emit` is defined when the scan calls it. (Pure relocation — no behavior change for the other arms.)

- [ ] **Step 4: Emit `malformed-id` in `board-checks.sh`; harden the cycle scan**

In `scripts/board-checks.sh`, replace the main scan-loop head (L50):

```bash
for f in "${FILES[@]}"; do
  id="$(field "$f" id)"; [ -n "$id" ] || continue
  status="$(field "$f" status)"
```
with:
```bash
for f in "${FILES[@]}"; do
  raw="$(field "$f" id)"; id="$(int_field "$f" id)"
  if [ -z "$id" ]; then
    [ -n "$raw" ] && emit malformed-id "$raw" "non-integer id '$raw' in $(basename "$f")"
    continue
  fi
  status="$(field "$f" status)"
```

(`emit`/`FINDINGS` are already defined above this loop in board-checks — verify.) And in the dep-cycle loop (L92):

```bash
  cid="$(field "$f" id)"; [ -n "$cid" ] || continue
```
to:
```bash
  cid="$(int_field "$f" id)"; [ -n "$cid" ] || continue
```

- [ ] **Step 5: Run to verify they pass**

Run: `bash tests/test_adr_checks.sh; bash tests/test_board_checks.sh`
Expected: both end `PASS`; the malformed-id assertions pass, the clean-ledger control stays silent, and `adr-checks` no longer emits the `integer expression expected` stderr (the malformed file is skipped before L42).

- [ ] **Step 6: Commit**

```bash
git add scripts/adr-checks.sh scripts/board-checks.sh tests/test_adr_checks.sh tests/test_board_checks.sh
git commit -m "feat(0032): validators flag a malformed id (first-class warn-only finding)

board-checks + adr-checks compare raw vs int_field and emit malformed-id instead
of silently skipping; the at-risk adr-checks MAXID arithmetic now only sees ints.
board-checks cycle scan adopts int_field too.

Claude-Session: https://claude.ai/code/session_011F2ADM8tyKirJc6Ku6nq35"
```

---

### Task 4: `terminal-publish.sh` fail-closed `--id`/`--adr` guard

**Files:**
- Modify: `scripts/terminal-publish.sh` (guard after the mutual-exclusivity checks ~L46)
- Test: `tests/test_terminal_publish.sh` (new — arg-validation only, no git)

**Interfaces:**
- Consumes: existing `die()`.
- Produces: a non-integer `--id`/`--adr` exits non-zero with a diagnostic before any git work.

- [ ] **Step 1: Write the failing test (new file)**

Create `tests/test_terminal_publish.sh`:

```bash
#!/usr/bin/env bash
# tests/test_terminal_publish.sh — arg-validation guards for terminal-publish.sh. The --id/--adr
# integer guard fires at parse time, before any git work, so these need no repo. (change 0032)
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$REPO/scripts/terminal-publish.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

assert "script exists and is executable" '[ -x "$SCRIPT" ]'

err="$(bash "$SCRIPT" --id abc 2>&1)"; rc=$?
assert "--id abc exits non-zero"        '[ "$rc" -ne 0 ]'
assert "--id abc diagnostic names id"   'printf "%s" "$err" | grep -qiE "id"'

err="$(bash "$SCRIPT" --adr 1.5 2>&1)"; rc=$?
assert "--adr 1.5 exits non-zero"       '[ "$rc" -ne 0 ]'

# a valid integer id passes the int-guard (it dies later on a DIFFERENT, missing-arg error)
err="$(bash "$SCRIPT" --id 5 2>&1)"; rc=$?
assert "--id 5 passes the int guard"    '[ "$rc" -ne 0 ] && ! printf "%s" "$err" | grep -qi "non-integer"'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
```

Make it executable: `chmod +x tests/test_terminal_publish.sh`

- [ ] **Step 2: Run to verify it fails**

Run: `bash tests/test_terminal_publish.sh`
Expected: `--id abc exits non-zero` may pass coincidentally (dies on missing `--outcome`/branches), but `--id 5 passes the int guard` and the `non-integer` diagnostic assertions reveal there is no int guard yet → at least one `NOT OK`, ends `FAIL`. (If all happen to pass, the guard message assertion still pins the new behavior.)

- [ ] **Step 3: Add the guard**

In `scripts/terminal-publish.sh`, immediately AFTER the two mutual-exclusivity checks:

```bash
if [ -n "$ID" ] && [ -n "$ADR" ]; then die "--id and --adr are mutually exclusive"; fi
if [ -z "$ID" ] && [ -z "$ADR" ]; then die "exactly one of --id / --adr is required"; fi
```
add:
```bash
# fail closed on a non-integer id (CLI arg, never frontmatter) — a publish must hard-stop, not skip
case "$ID"  in (''|*[!0-9]*) [ -z "$ID" ]  || die "non-integer --id: '$ID'"  ;; esac
case "$ADR" in (''|*[!0-9]*) [ -z "$ADR" ] || die "non-integer --adr: '$ADR'" ;; esac
```

(The `[ -z … ] ||` lets the empty case fall through — emptiness is already handled by the "exactly one" check above; only a set-but-non-integer value dies here.)

- [ ] **Step 4: Run to verify it passes**

Run: `bash tests/test_terminal_publish.sh`
Expected: ends `PASS` — `--id abc`/`--adr 1.5` die with the int-guard message; `--id 5` passes the guard and dies later on a different (missing-arg) error.

- [ ] **Step 5: Commit**

```bash
git add scripts/terminal-publish.sh tests/test_terminal_publish.sh
git commit -m "feat(0032): terminal-publish fails closed on a non-integer --id/--adr

The publisher's id is a CLI arg, not frontmatter; validate it at parse time with
die() (matching the script's style) so a bad id hard-stops before any git work.

Claude-Session: https://claude.ai/code/session_011F2ADM8tyKirJc6Ku6nq35"
```

---

## Self-Review

**1. Spec coverage:** `int_field` helper ✓ (T1); renderers + resolve_deps + readiness skip ✓ (T2, all reconcile-pinned sites incl. lib readiness L71); validators first-class `malformed-id` finding + cycle scan ✓ (T3); terminal-publish fail-closed CLI guard ✓ (T4); tolerant matching ✓ (no full-string compare); warn-only ✓; scope `id:` only ✓.

**2. Placeholder scan:** every step has full code + exact commands; the one "adapt to the file's established invocation pattern" note (board-checks test) points at a concrete existing pattern in the same file, not a TBD. No "handle edge cases."

**3. Type consistency:** `int_field` returns value-or-`""`, consumed identically everywhere via the existing `[ -n "$id" ] || continue`; the validators' `raw`/`id` pair and `emit malformed-id "$raw" …` shape match the existing `emit CHECK ID MSG` signature; `die` reused in terminal-publish. The `malformed-id` check-id string is identical across both validators and both tests.
