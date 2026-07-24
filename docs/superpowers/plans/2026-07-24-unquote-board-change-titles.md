<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0138 — Board generator wraps each change title in literal double quotes](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0138-unquote-board-change-titles.md)**
<!-- docket:backlink:end -->

# Unquote board change titles Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the shared frontmatter readers return a title's logical value so the board (and every other title consumer) renders it bare instead of wrapped in the literal YAML double quotes a comma/apostrophe title carries.

**Architecture:** Add one pure-bash unwrap helper to `scripts/lib/docket-frontmatter.sh` that strips a single matched surrounding quote pair, and call it from both `field()` and its anchored twin `fm_field()`. This is a purely read-side, single-source fix: the board, the GitHub mirror, the ADR index, board-checks, and the artifact backlinks all read titles through these two functions, so all render bare from one change. No title storage/write path changes.

**Tech Stack:** POSIX-ish Bash (`/opt/homebrew/bin/bash` per `DOCKET_BASH_PATH`; lib is `#!/usr/bin/env bash`), `awk`/`sed` for frontmatter parsing, the repo's hand-rolled `assert` shell test harness (`tests/test_*.sh`).

## Global Constraints

- **Read-side only.** Do NOT change how titles are written/stored — YAML quoting at write time is valid and stays (spec "Out of scope").
- **Strip rule (exact, from spec §Decision):** strip only when the (trailing-whitespace-trimmed) value is ≥2 chars AND its first and last characters are the **same** quote char, either `"` or `'`; strip exactly **one** layer; leave interior bytes byte-for-byte. This is **not** YAML unescaping — `\"`/`\\` are left untouched (deferred, spec Assumption 3).
- **Single shared helper.** One unwrap definition the two readers call — never a duplicated snippet (avoids the un-fixed-twin hazard, learnings `escape-ere-metacharacters-in-key`).
- **Preserve `field()`'s trailing-newline output contract** (`printf '%s\n'`, `docket-frontmatter.sh:35-37`): callers that pipe `field` directly (the mermaid done-id list) rely on the separator. The unwrap must re-emit that newline, never strip it via a `$(...)` round-trip that isn't followed by `printf '%s\n'`.
- **Pure bash, no forks in the helper.** Use parameter expansion only — no external tool — so it is pipefail-safe and GNU/BSD-portable (learnings, AGENTS.md `pipefail`, `shell-portability`).
- **Frontmatter edits stay anchored** to the first `---…---` block (AGENTS.md `frontmatter-edit-anchor`) — not relevant to code here, but the fixtures added to tests must keep valid single-block frontmatter.

---

### Task 1: Shared matched-quote unwrap helper, wired into `field()` and `fm_field()`

Introduce `_docket_unwrap_quotes` and call it from both readers. Both callers live in one task on purpose: the two differ in their output-terminator posture (`field()` keeps a trailing newline for piped consumers; `fm_field()` is always `$()`-captured), and keeping them side-by-side is how the reviewer catches the shared helper flattening that variance (learnings `consolidation-flattens-caller-variance`).

**Files:**
- Modify: `scripts/lib/docket-frontmatter.sh:31-70` (add the helper above `field()`; rewire `field()` and `fm_field()`; update both header contract comments)
- Test: `tests/test_docket_frontmatter.sh` (append a new assertion block)

**Interfaces:**
- Produces: `_docket_unwrap_quotes VALUE` — prints the logical scalar (a single matched surrounding `"`/`'` pair stripped) to stdout with **no** trailing newline. Pure; no side effects.
- Produces (unchanged signatures, new behavior): `field FILE KEY` still prints the value + one trailing `\n`, now unwrapped. `fm_field FILE KEY` still prints the first-block value (empty when absent), now unwrapped, as a single line.
- Consumes: nothing new.

- [ ] **Step 1: Write the failing reader-level tests**

Append this block to `tests/test_docket_frontmatter.sh` immediately **before** the final `if [ "$fail" = 0 ]; then echo "PASS"; ...` line:

```bash
# --- matched-quote unwrap: readers return the LOGICAL scalar (change 0138) ---
qd="$(mktemp -d)"
printf -- '---\ntitle: "Comma, title"\n---\n'   > "$qd/dq.md"
printf -- "---\ntitle: 'Comma, title'\n---\n"   > "$qd/sq.md"
printf -- '---\ntitle: Bare title\n---\n'       > "$qd/bare.md"
printf -- '---\ntitle: Say "hi" now\n---\n'     > "$qd/interior.md"
printf -- '---\ntitle: "unterminated\n---\n'    > "$qd/untl.md"
printf -- '---\ntitle: foo"\n---\n'             > "$qd/trailq.md"
printf -- '---\ntitle: "\n---\n'                > "$qd/onechar.md"
printf -- '---\ntitle: ""\n---\n'               > "$qd/empty2.md"

# field(): strip a matched surrounding pair, leave everything else byte-for-byte
assert "field strips a double-quoted value"       '[ "$(field "$qd/dq.md" title)" = "Comma, title" ]'
assert "field strips a single-quoted value"       '[ "$(field "$qd/sq.md" title)" = "Comma, title" ]'
assert "field leaves a bare value unchanged"      '[ "$(field "$qd/bare.md" title)" = "Bare title" ]'
assert "field leaves an interior quote untouched" '[ "$(field "$qd/interior.md" title)" = "Say \"hi\" now" ]'
assert "field leaves an unterminated open quote"  '[ "$(field "$qd/untl.md" title)" = "\"unterminated" ]'
assert "field leaves a trailing-only quote"       '[ "$(field "$qd/trailq.md" title)" = "foo\"" ]'
assert "field leaves a lone single quote char"    '[ "$(field "$qd/onechar.md" title)" = "\"" ]'
assert "field reduces an empty quoted value"      '[ -z "$(field "$qd/empty2.md" title)" ]'
# field() MUST keep its single trailing newline (piped-consumer contract, e.g. mermaid done-id list)
assert "field emits exactly one trailing newline" '[ "$(field "$qd/dq.md" title | wc -l | tr -d " ")" = "1" ]'

# fm_field(): mirror-image cases through the anchored twin (shares the helper)
assert "fm_field strips a double-quoted value"       '[ "$(fm_field "$qd/dq.md" title)" = "Comma, title" ]'
assert "fm_field strips a single-quoted value"       '[ "$(fm_field "$qd/sq.md" title)" = "Comma, title" ]'
assert "fm_field leaves a bare value unchanged"      '[ "$(fm_field "$qd/bare.md" title)" = "Bare title" ]'
assert "fm_field leaves an interior quote untouched" '[ "$(fm_field "$qd/interior.md" title)" = "Say \"hi\" now" ]'
assert "fm_field leaves an unterminated open quote"  '[ "$(fm_field "$qd/untl.md" title)" = "\"unterminated" ]'
assert "fm_field empty when the key is absent"       '[ -z "$(fm_field "$qd/dq.md" nonesuch)" ]'
rm -rf "$qd"
```

- [ ] **Step 2: Run the tests to verify the new block fails**

Run: `bash tests/test_docket_frontmatter.sh`
Expected: FAIL — e.g. `NOT OK - field strips a double-quoted value` (today `field` returns `"Comma, title"` with the quotes), final line `FAIL`, exit 1. The pre-existing assertions still print `ok - …`.

- [ ] **Step 3: Add the shared unwrap helper**

In `scripts/lib/docket-frontmatter.sh`, insert this function directly **above** the `field()` definition (i.e. just after the `# --- frontmatter accessors …` comment on line 31, before `field(){`):

```bash
# _docket_unwrap_quotes VALUE -> logical scalar on stdout, with NO trailing newline.
# Strips a SINGLE matched pair of surrounding quotes (both " or both ') when VALUE is at least two
# characters and its first and last characters are the same quote char. Interior bytes are left
# byte-for-byte: this is NOT YAML unescaping (\" and \\ are untouched — see change 0138, spec
# Assumption 3). Pure bash parameter expansion — no subshell, no fork, no external tool — so it is
# pipefail-safe and portable across GNU/BSD hosts. field() and fm_field() are its only callers.
_docket_unwrap_quotes(){
  local v="$1" q
  if [ "${#v}" -ge 2 ]; then
    q="${v:0:1}"
    if { [ "$q" = '"' ] || [ "$q" = "'" ]; } && [ "${v: -1}" = "$q" ]; then
      v="${v:1:${#v}-2}"
    fi
  fi
  printf '%s' "$v"
}
```

- [ ] **Step 4: Rewire `field()` to unwrap the value**

Replace the body of `field()` (`docket-frontmatter.sh:32-37`) so it strips trailing whitespace into a variable, unwraps, then re-emits with the trailing newline preserved:

```bash
field(){
  local raw; raw="$(sed -n "s/^$2:[[:space:]]*//p" "$1")"
  raw="${raw%%$'\n'*}"                              # keep only the first matching line — no pipe
  raw="${raw%"${raw##*[![:space:]]}"}"             # strip trailing whitespace
  printf '%s\n' "$(_docket_unwrap_quotes "$raw")"  # return the LOGICAL scalar (a matched surrounding
}                                                  # quote pair stripped); the trailing \n preserves the
                                                   # piped-consumer contract (e.g. the mermaid done-id list)
```

Note: `$(_docket_unwrap_quotes "$raw")` strips no meaningful bytes (the helper emits no trailing newline), and the outer `printf '%s\n'` re-adds the one newline — so `field()`'s output contract is preserved.

- [ ] **Step 5: Rewire `fm_field()` to unwrap the value**

Replace `fm_field()` (`docket-frontmatter.sh:56-70`) so it captures the awk result and passes it through the shared helper. Keep the awk body byte-for-byte; only the capture + trailing call are new:

```bash
fm_field(){ # fm_field FILE KEY -> logical scalar on stdout (empty when absent from the first block)
  local raw
  raw="$(awk -v key="$2" '
    BEGIN { n = 0 }
    /^---[[:space:]]*$/ { n++; if (n >= 2) exit; next }
    n == 1 {
      if ($0 ~ ("^" key ":")) {
        sub(/[[:space:]]+#.*$/, "")
        sub("^" key ":[[:space:]]*", "")
        sub(/[[:space:]]+$/, "")
        print
        exit
      }
    }
  ' "$1")"
  _docket_unwrap_quotes "$raw"
}
```

Note: every current `fm_field` consumer captures via `$(...)` (`render-artifact-backlink.sh:78`, `render-board.sh:85/106`, `github-mirror.sh:158/187`, `board-checks.sh:170`, `backfill-change-types.sh:94/100/117/135`), so emitting the value with no trailing newline is invisible to callers and matches the spec's "keep its shape" intent (a single line). The absent case still emits 0 bytes (empty `raw` → helper prints nothing).

- [ ] **Step 6: Update both header contract comments**

Update the `field`/`fm_field` descriptions in the header block (`docket-frontmatter.sh:8-11`) to state the readers return the logical scalar. Change the two lines to:

```bash
#   field FILE KEY        — first matching scalar for KEY anywhere in the file, trimmed and with a
#                           single matched pair of surrounding quotes stripped (logical value).
#   fm_field FILE KEY     — like field(), but ONLY inside the first ---...--- block. Use this for
#                           any key that may be ABSENT from frontmatter (e.g. type:), where field()
#                           would fall through and return body prose. Same quote-stripping as field().
```

- [ ] **Step 7: Run the tests to verify they pass**

Run: `bash tests/test_docket_frontmatter.sh`
Expected: PASS — every new `ok - field …` / `ok - fm_field …` line prints, all pre-existing assertions still `ok`, final line `PASS`, exit 0.

- [ ] **Step 8: Commit**

```bash
git add scripts/lib/docket-frontmatter.sh tests/test_docket_frontmatter.sh
git commit -m "fix(0138): strip matched surrounding quotes in field()/fm_field()"
```

---

### Task 2: Board-render regression guard + contract doc note

Prove end-to-end that a YAML-quoted title reaches `BOARD.md` bare, and record the read-side contract in the board script's doc so the behavior is documented, not incidental.

**Files:**
- Test: `tests/test_render_board.sh` (append a standalone regression block)
- Modify: `scripts/render-board.md` (one-line note on the Row structure section, ~line 55-60)

**Interfaces:**
- Consumes: `field FILE title` from Task 1 (the now-unwrapping reader `render-board.sh:303/399` calls).
- Produces: nothing consumed by later tasks.

- [ ] **Step 1: Write the failing board regression test**

Append this block to `tests/test_render_board.sh` immediately **before** the final `if [ "$fail" = 0 ]; then echo "PASS"; ...` line. It uses the `$(...)` capture form (matching the existing captures at lines 249/323) so it does not trip the render-board write sentinel:

```bash
# --- change 0138: a YAML-quoted title renders BARE in the board (regression guard) ---
qtmp="$(mktemp -d)"; mkdir -p "$qtmp/active" "$qtmp/archive"
cat > "$qtmp/active/0020-tango.md" <<'EOF'
---
id: 20
slug: tango
title: "Tango, with a comma"
status: proposed
priority: medium
type: fix
depends_on: []
spec: docs/superpowers/specs/2026-07-24-tango.md
EOF
qout="$(bash "$SCRIPT" --changes-dir "$qtmp" --repo o/r 2>/dev/null)"
assert "quoted title renders without its surrounding quotes" \
  'printf "%s" "$qout" | grep -qF "| Tango, with a comma |"'
assert "quoted title does not render the literal double quotes" \
  '! printf "%s" "$qout" | grep -qF "\"Tango, with a comma\""'
rm -rf "$qtmp"
```

- [ ] **Step 2: Run the board test to verify the new block fails on an unfixed reader**

Because Task 1 is already committed, this block will PASS now. To confirm it is a real guard (that it would have caught the bug), temporarily verify against the pre-fix behavior:

Run: `git stash -- scripts/lib/docket-frontmatter.sh 2>/dev/null; bash tests/test_render_board.sh; git stash pop 2>/dev/null || true`

If nothing is stashed (Task 1 already committed), instead confirm the guard's teeth by hand once:
Run: `printf '%s' "$(printf -- '---\ntitle: "Tango, with a comma"\nstatus: proposed\n' > /tmp/t.md; sed -n "s/^title:[[:space:]]*//p" /tmp/t.md)'`
Expected pre-fix value: `"Tango, with a comma"` (with quotes) — demonstrating the row WOULD have carried the quotes. Delete `/tmp/t.md` after.

(This step documents the regression intent; the block itself is green post-Task-1.)

- [ ] **Step 3: Run the board test to verify it passes**

Run: `bash tests/test_render_board.sh`
Expected: PASS — `ok - quoted title renders without its surrounding quotes`, `ok - quoted title does not render the literal double quotes`, the byte-golden and idempotence assertions still `ok`, final line `PASS`, exit 0.

- [ ] **Step 4: Add the contract note to `scripts/render-board.md`**

In `scripts/render-board.md`, add a sentence to the Row/structure section (near the `| in-progress | ... Title ... |` table, ~line 55-60) recording that titles render bare:

```markdown
**Titles render bare.** The `Title` column shows the change's *logical* title. Titles are read
through the shared `field()`/`fm_field()` readers (`scripts/lib/docket-frontmatter.sh`), which strip
a single matched pair of surrounding YAML quotes, so a title that was double-quoted at write time
(because it contains a comma or apostrophe) renders without the quotes (change 0138).
```

- [ ] **Step 5: Commit**

```bash
git add tests/test_render_board.sh scripts/render-board.md
git commit -m "test(0138): board regression guard for bare quoted titles + doc note"
```

---

## Self-Review

**1. Spec coverage:**
- Spec §Decision "fix the shared reader, strip one matched pair, both `field()` and `fm_field()` via one helper, preserve `field()`'s trailing newline" → Task 1 (helper + both readers + newline assertion).
- Spec §What changes bullet "`scripts/lib/docket-frontmatter.sh` add unwrap + update header contract" → Task 1 Steps 3-6.
- Spec §What changes bullet "`scripts/render-board.md` note titles render bare" → Task 2 Step 4. (No frontmatter-lib contract `.md` exists — verified — so none is created; YAGNI.)
- Spec §What changes Tests bullet "`tests/test_docket_frontmatter.sh` — double/single-quoted → bare; unquoted → unchanged; interior quote → unchanged; mismatched/unterminated → unchanged; empty/single-char → unchanged; mirror cases for `fm_field()`" → Task 1 Step 1 (all cases present for both readers).
- Spec §What changes Tests bullet "`tests/test_render_board.sh` — quoted title renders without surrounding quotes (regression guard)" → Task 2 Step 1.
- Spec Assumption 3 (no YAML unescaping) → helper leaves interior bytes untouched; `interior`/`unterminated` cases assert it.
- Spec §Decision `field()` trailing-newline contract → Task 1 Step 1 "field emits exactly one trailing newline" + Step 4 re-emit.

**2. Placeholder scan:** No TBD/TODO/"add error handling"/"write tests for the above" — every code and test step carries exact content.

**3. Type consistency:** `_docket_unwrap_quotes` is defined in Task 1 Step 3 and called in Steps 4-5 with the same name and single-arg signature; the tests call `field`/`fm_field` with their existing `FILE KEY` signatures; `$SCRIPT`/`assert` are the harness names already defined at the top of each test file. Consistent.
