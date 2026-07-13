# Harden the BOARD.md write guard — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the proxy guards that police `render-board.sh`'s stdout with one that prohibits the *write* itself, so no-space, `>>`, `>|`, variable-target, and continuation-line redirects all die identically — and mutation-test every one of them.

**Architecture:** Three guards, derived from a single invariant — *`render-board.sh`'s stdout reaches a file through `board-refresh.sh` and nothing else.* **Guard 1** (new, `tests/test_render_board.sh`) is a repo-wide write sentinel: a bash function that, for any script, joins backslash-continuations → drops whole-line comments → erases fd dups → tokenizes per invocation → fails on any surviving `>`. It never matches the write *target*, so it cannot be evaded by renaming it. It ships as a **function** precisely so the mutation battery calls the *same code* the real scan calls — a battery that tests a copy is decoration. **Guard 2** keeps `REDIRECT_RE` byte-identical but re-scopes it to `skills/*/SKILL.md` prose, where its narrow whitespace-bounded shape is correct, and re-derives its design comment. **Guard 3** (`tests/test_docket_status.sh`) keeps the `--format digest` flag check and inherits the continuation-joining fix.

**Tech Stack:** Bash (`set -uo pipefail`), `awk` (continuation joining — portable across BSD/GNU, unlike `sed`'s `N`/`\n` handling), `grep -oE`, `find`. No production code changes; no new dependencies.

## Global Constraints

- **Test-only change.** `render-board.sh`, `board-refresh.sh`, and `docket-status.sh` are all correct today and MUST NOT be modified. Copied verbatim from the spec: *"No production code changes... This change is about the suite's ability to notice if a future one is not."*
- **`REDIRECT_RE` is kept UNWIDENED and byte-identical**: `render-board\.sh.{0,200}[[:space:]]>[[:space:]].{0,200}/BOARD\.md`. Its positive control (`HISTORICAL_REDIRECT`) stays too. Only its *scope* (skills-only) and its *comment* change.
- **Call-site lists are DERIVED, never hand-maintained** (ledger #64). Guard 1 enumerates scripts with `find "$REPO/scripts" -name '*.sh' -type f` — which covers `scripts/lib/*.sh`, invisible to a flat `scripts/*.sh` glob.
- **`board-refresh.sh` is the ONLY allowlisted writer**, matched by basename.
- **Every guard is mutation-tested** (ledger #64: *a guard is code — mutation-test it before trusting it, or it is decoration*). Each evasion must turn the guard RED; the fd-dup control must keep it GREEN.
- The false-positive control is the codebase's real invocation, copied verbatim from `scripts/docket-status.sh:172`:
  `out="$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$cd_dir" --format digest 2>&2)"`
- Existing harness conventions in both test files are preserved: `set -uo pipefail`, `REPO=`, `fail=0`, `assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }`, and the `$tmp` mktemp dir with its `trap`.

---

### Task 1: Guard 1 — the repo-wide write sentinel + its mutation battery

**Files:**
- Modify: `tests/test_render_board.sh` — insert the joiner, the guard function, the battery, and the real scan immediately BEFORE the existing `REDIRECT_RE` block (currently at line ~249, the comment starting `# --- negative sentinel:`). Task 2 rewrites that block; this task does not touch it.
- Test: `tests/test_render_board.sh` (the battery *is* the test — the guard function is the code under test).

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces:
  - `join_continuations <file>` → stdout: the file with backslash-continuations folded into logical lines. Task 3 defines an identical twin in `tests/test_docket_status.sh`.
  - `render_board_write_free <file>` → exit **0** = clean (no file-directed redirect on any `render-board.sh` invocation), exit **1** = violation (offending invocations echoed to stdout).

- [ ] **Step 1: Write the failing test — the mutation battery + the real scan**

Insert this block into `tests/test_render_board.sh` immediately before the line `# --- negative sentinel: no skill body may redirect render-board.sh stdout straight into`.

The battery comes first *in the file* as well as first in time: it is the reason the guard is trustworthy, and a reader must meet it before the scan that relies on it.

```bash
# --- Guard 1: the repo-wide render-board.sh WRITE sentinel (change 0070) ----------------------
# THE INVARIANT, stated once: render-board.sh's stdout reaches a file through board-refresh.sh
# and NOTHING else.
#
# Every guard that came before encoded a PROXY for that sentence — "a --format digest flag is
# present" (tests/test_docket_status.sh), "a ` > ` appears near the string BOARD.md" (REDIRECT_RE,
# below) — and every known evasion is an exploit of the gap between the proxy and the invariant.
# This guard prohibits the WRITE instead of recognizing the write TARGET, so it never matches a
# filename and cannot be evaded by renaming one:
#     >"$d/BOARD.md"   (no space — the IDIOMATIC shell form, which REDIRECT_RE cannot see)
#     >> / >|          (append and clobber-force)
#     > "$mw/$rel"     (VARIABLE target — the literal string "BOARD.md" appears nowhere near the
#                       redirect, so NO regex keyed on /BOARD\.md can reach it AT ANY WIDTH; this
#                       is what forecloses "just widen REDIRECT_RE" as a complete answer)
# all die identically. It does not ask WHERE you are writing; it asserts you are not writing.
#
# The call-site list is DERIVED FROM A find(1) SWEEP, never hand-maintained (ledger #64: never
# hand-list the call sites of an operation you are gating). The sweep covers scripts/lib/*.sh,
# which a flat scripts/*.sh glob would silently miss. board-refresh.sh — the one gated writer
# (change 0059: render to temp -> chmod -> rename) — is the ONLY allowlisted script.
#
# PIPELINE ORDER IS LOAD-BEARING:
#   1. JOIN backslash-continuations into logical lines. The tokenizer is line-oriented, so a
#      redirect parked on a continuation line hands it a first-line token with no `>` — a clean
#      pass. (REDIRECT_RE survived that shape only because it flattens the file with `tr` first;
#      that flattening was never incidental. Guard 1 subsumes the scan it replaces ONLY because
#      it joins.)
#   2. DROP whole-line comments — prose that merely NAMES the script is not an invocation.
#      render-adr-index.sh, render-change-links.sh, and render-board.sh itself mention it in
#      comments ONLY, so this step is what keeps them out of the scan, not a nicety.
#   3. ERASE fd dups (>&2, 2>&1) BEFORE tokenizing. Subtle and mandatory: the tokenizer splits on
#      ; & | — and the `&` of `2>&2` would CUT the token mid-redirect, leaving a dangling `>` that
#      fires on the codebase's own CORRECT invocation. Erase first and the fd dup is gone before
#      it can be mistaken for a write.
#   4. TOKENIZE PER INVOCATION, not per line: a logical line carrying a clean call beside a rogue
#      one must not be whitewashed by the clean one (ledger #64).
#   5. ANY surviving `>` in a token is a file-directed redirect => violation. This rejects even a
#      stderr-to-file form (2>/dev/null) — deliberately conservative: the correct way to route
#      this renderer's stderr is the fd dup already in use (2>&2), and a guard that permits SOME
#      writes is a guard whose next author must relitigate which.
#
# KNOWN, ACCEPTED GAP: a pipe into a writer (`render-board.sh ... | tee f`) is not caught — the
# tokenizer ends the invocation at the `|`. The answer to that class is the filesystem-effect test
# the design DEFERRED (run the orchestrator against a fixture, assert BOARD.md's bytes): it is
# syntax-independent but path-dependent, and it earns its cost when a write path exists that a
# source scan cannot reach. Today none does.
#
# The battery below is the substance of this guard (ledger #64: a guard is code — mutation-test it
# before trusting it, or it is decoration). Every evasion above is injected into a fixture and MUST
# turn the guard RED; the fd-dup control — the codebase's real invocation — MUST keep it GREEN.

# Folds backslash-continuations into logical lines. awk, not sed: BSD sed does not portably treat
# `\n` in an s/// LHS as a newline, so the classic `:a; /\\$/N; s/\\\n//; ta` is a GNU-ism here.
# TWIN: an identical function lives in tests/test_docket_status.sh (Guard 3 tokenizes the same
# unit and inherits the same fix) — keep the two in step.
join_continuations(){
  awk '{ while (sub(/\\$/, "")) { if ((getline nxt) > 0) { $0 = $0 nxt } else { break } } print }' "$1"
}

# 0 = clean; 1 = at least one render-board.sh invocation carries a file-directed redirect.
render_board_write_free(){
  local f="$1" violation=0 inv
  while IFS= read -r inv; do
    [ -n "$inv" ] || continue
    echo "  (render-board.sh invocation writes to a file in ${f##*/}: $inv)"
    violation=1
  done < <(
    join_continuations "$f" \
      | grep -v '^[[:space:]]*#' \
      | sed 's/[0-9]*>&[0-9-]*//g' \
      | grep -oE '[^;&|]*/render-board\.sh[^;&|]*' \
      | grep '>' || true
  )
  return "$violation"
}

# --- the mutation battery: each row is a fixture the guard must judge correctly ---
mut="$tmp/mut"; mkdir -p "$mut"

# (1) spaced redirect — the ONLY shape REDIRECT_RE could already see. Establishes the baseline.
printf '%s\n' '#!/usr/bin/env bash' \
  '"$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" > "$d/BOARD.md"' > "$mut/spaced.sh"
assert "guard1 flags a spaced redirect into BOARD.md" \
  '! render_board_write_free "$mut/spaced.sh"'

# (2) NO SPACE — the idiomatic shell redirect, invisible to REDIRECT_RE's [[:space:]]>[[:space:]].
printf '%s\n' '#!/usr/bin/env bash' \
  '"$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" >"$d/BOARD.md"' > "$mut/nospace.sh"
assert "guard1 flags a no-space redirect into BOARD.md" \
  '! render_board_write_free "$mut/nospace.sh"'

# (3) append. Not special-cased: the board is a full regeneration, never an accumulation — but
#     this dies for the same reason everything else does, not because append is uniquely wrong.
printf '%s\n' '#!/usr/bin/env bash' \
  '"$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" >> "$d/BOARD.md"' > "$mut/append.sh"
assert "guard1 flags an append (>>) redirect into BOARD.md" \
  '! render_board_write_free "$mut/append.sh"'

# (4) clobber-force.
printf '%s\n' '#!/usr/bin/env bash' \
  '"$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" >| "$d/BOARD.md"' > "$mut/clobber.sh"
assert "guard1 flags a clobber-force (>|) redirect into BOARD.md" \
  '! render_board_write_free "$mut/clobber.sh"'

# (5) VARIABLE TARGET — the row that forecloses widening. docket-status.sh really does hold the
#     board path in a variable (`local rel="$CHANGES_DIR/BOARD.md"`), so a rogue write can carry
#     the literal "BOARD.md" nowhere near the redirect. Unreachable by any BOARD.md-keyed regex.
printf '%s\n' '#!/usr/bin/env bash' \
  'rel="$CHANGES_DIR/BOARD.md"' \
  '"$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" > "$mw/$rel"' > "$mut/vartarget.sh"
assert "guard1 flags a redirect to a VARIABLE target (no BOARD.md near the >)" \
  '! render_board_write_free "$mut/vartarget.sh"'

# (6) CONTINUATION LINE — the redirect sits on the second physical line. A line-oriented tokenizer
#     sees a first-line token with no `>` and passes it clean. This is why joining precedes
#     tokenizing, and why dropping the old flattened scan is only safe once it does.
printf '%s\n' '#!/usr/bin/env bash' \
  '"$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" \' \
  '  > "$f"' > "$mut/continuation.sh"
assert "guard1 flags a redirect parked on a continuation line" \
  '! render_board_write_free "$mut/continuation.sh"'

# (6b) CONTINUATION + NO SPACE — the union shape the design named as still-live under BOTH old
#      guards (evades the line-oriented tokenizer AND REDIRECT_RE's whitespace requirement).
printf '%s\n' '#!/usr/bin/env bash' \
  '"$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" \' \
  '  >"$mw/$rel"' > "$mut/continuation-nospace.sh"
assert "guard1 flags a no-space redirect on a continuation line (the union evasion)" \
  '! render_board_write_free "$mut/continuation-nospace.sh"'

# (7) FALSE-POSITIVE CONTROL — the codebase's REAL invocation, copied verbatim from
#     scripts/docket-status.sh, fd dup and all. A guard that fires on this is a guard someone
#     disables; this row carries as much weight as every RED row above it.
printf '%s\n' '#!/usr/bin/env bash' \
  'if ! out="$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$cd_dir" --format digest 2>&2)"; then' \
  '  return 1' \
  'fi' > "$mut/fddup.sh"
assert "guard1 stays GREEN on the real 2>&2 --format digest invocation (false-positive control)" \
  'render_board_write_free "$mut/fddup.sh"'

# (8) FALSE-POSITIVE CONTROL — a 2>&1 fd dup beside a COMMENT that spells out the old redirect.
#     Comment-stripping is load-bearing: three scripts name render-board.sh in comments only.
printf '%s\n' '#!/usr/bin/env bash' \
  '# historical (pre-0059): render-board.sh --changes-dir "$d" > "$d/BOARD.md"' \
  'out="$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" 2>&1)"' > "$mut/comment.sh"
assert "guard1 stays GREEN on an fd-dup call beside a comment naming the old redirect" \
  'render_board_write_free "$mut/comment.sh"'

# --- the real scan: every script under scripts/ EXCEPT the one allowlisted writer ---
guard1_violation=0
scanned=0
while IFS= read -r s; do
  [ "${s##*/}" = "board-refresh.sh" ] && continue
  scanned=$((scanned + 1))
  render_board_write_free "$s" || guard1_violation=1
done < <(find "$REPO/scripts" -name '*.sh' -type f | sort)
assert "no script under scripts/ (except board-refresh.sh) writes render-board.sh's stdout to a file" \
  '[ "$guard1_violation" -eq 0 ]'

# Anti-vacuity: a scan over zero files passes for the wrong reason. Assert the sweep actually saw
# the tree, and that the allowlisted writer it skips really exists (a rename must not silently
# turn the allowlist into a no-op that hides the real writer).
assert "the write scan is not vacuous (it swept the scripts tree)" '[ "$scanned" -ge 10 ]'
assert "the allowlisted writer scripts/board-refresh.sh exists" \
  '[ -f "$REPO/scripts/board-refresh.sh" ]'
```

- [ ] **Step 2: Run the test to verify the battery fails**

Run: `cd /Users/homer/dev/docket/.worktrees/redirect-regex-board-write-guard && bash tests/test_render_board.sh 2>&1 | grep -E "guard1|write scan|allowlisted|^(PASS|FAIL)"`

Expected: **FAIL**. Because `render_board_write_free` and `join_continuations` are pasted in Step 1 together with the battery, the honest red state to observe here is the one that matters — so run Step 2 **with the two function definitions temporarily commented out** to prove the battery is not self-satisfying:

```bash
# Temporarily neuter the guard to prove the battery detects a broken guard (mutation of the GUARD
# itself, not of the call sites). Replace the body of render_board_write_free with `return 0`:
#   render_board_write_free(){ return 0; }
```

With the stub returning 0 (always "clean"), expected output: the six `! render_board_write_free` rows print `NOT OK - guard1 flags ...`, the two control rows print `ok - guard1 stays GREEN ...`, and the script ends `FAIL`. That is the proof the battery has teeth: a guard that never fires is caught. Restore the real function body before Step 3.

- [ ] **Step 3: Run the test to verify it passes with the real guard**

Run: `cd /Users/homer/dev/docket/.worktrees/redirect-regex-board-write-guard && bash tests/test_render_board.sh 2>&1 | grep -E "guard1|write scan|allowlisted|^(PASS|FAIL)"`

Expected — all ten rows `ok -`, and the file ends `PASS`:

```
ok - guard1 flags a spaced redirect into BOARD.md
ok - guard1 flags a no-space redirect into BOARD.md
ok - guard1 flags an append (>>) redirect into BOARD.md
ok - guard1 flags a clobber-force (>|) redirect into BOARD.md
ok - guard1 flags a redirect to a VARIABLE target (no BOARD.md near the >)
ok - guard1 flags a redirect parked on a continuation line
ok - guard1 flags a no-space redirect on a continuation line (the union evasion)
ok - guard1 stays GREEN on the real 2>&2 --format digest invocation (false-positive control)
ok - guard1 stays GREEN on an fd-dup call beside a comment naming the old redirect
ok - no script under scripts/ (except board-refresh.sh) writes render-board.sh's stdout to a file
ok - the write scan is not vacuous (it swept the scripts tree)
ok - the allowlisted writer scripts/board-refresh.sh exists
PASS
```

If the false-positive control (row 7) is RED, the fd-dup erasure in Step 3 of the pipeline is missing or misordered — the `&` of `2>&2` is cutting the token and leaving a dangling `>`. Fix the pipeline; never relax the assertion.

- [ ] **Step 4: Prove the guard against the LIVE tree, not just fixtures**

The battery proves the function works on fixtures. This proves it is wired to reality: inject the evasion into the real `scripts/docket-status.sh`, confirm the suite goes RED, then revert.

```bash
cd /Users/homer/dev/docket/.worktrees/redirect-regex-board-write-guard
# The evasion that today's guards CANNOT see: no-space redirect to a variable target.
perl -0pi -e 's{(--format digest 2>&2\)\)")}{$1 >"\$mw/\$rel"}' scripts/docket-status.sh
grep -n 'render-board.sh' scripts/docket-status.sh | tail -1     # confirm the mutation landed
bash tests/test_render_board.sh 2>&1 | grep -E "writes render-board|^(PASS|FAIL)"
```

Expected: `NOT OK - no script under scripts/ (except board-refresh.sh) writes render-board.sh's stdout to a file` and `FAIL`.

Then revert the production file — it must not be modified by this change:

```bash
git checkout -- scripts/docket-status.sh
git status --porcelain scripts/          # expected: EMPTY
bash tests/test_render_board.sh 2>&1 | tail -1   # expected: PASS
```

- [ ] **Step 5: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/redirect-regex-board-write-guard
git add tests/test_render_board.sh
git commit -m "test(0070): repo-wide render-board.sh write sentinel + mutation battery

Guard 1: prohibit the WRITE instead of recognizing the target. Joins
continuations, drops comments, erases fd dups, tokenizes per invocation,
fails on any surviving '>'. Derives its call-site list from a find(1)
sweep of scripts/ (covers scripts/lib/), allowlisting only board-refresh.sh.

Mutation-tested: no-space, >>, >|, variable-target, continuation, and
continuation+no-space all turn it red; the real 2>&2 --format digest
invocation and a comment naming the old redirect keep it green."
```

---

### Task 2: Guard 2 — re-scope `REDIRECT_RE` to prose, re-derive its comment, drop 0069's scan

**Files:**
- Modify: `tests/test_render_board.sh:249-302` — the `# --- negative sentinel:` comment block, and the `scripts/docket-status.sh` scan at its end (which Guard 1 now subsumes).

**Interfaces:**
- Consumes: `render_board_write_free` from Task 1 exists in the same file and already covers `scripts/docket-status.sh` — that is the precondition that makes deleting the scan below safe.
- Produces: nothing new; `REDIRECT_RE` and `HISTORICAL_REDIRECT` keep their exact names and values.

- [ ] **Step 1: Replace the comment block and the two scans**

Delete everything from `# --- negative sentinel: no skill body may redirect render-board.sh stdout straight into` through the end of the `scripts/docket-status.sh` scan (the assertion `"scripts/docket-status.sh never redirects render-board.sh stdout into BOARD.md"`), and put this in its place. `REDIRECT_RE`, `HISTORICAL_REDIRECT`, the positive control, and the `skills/*/SKILL.md` scan are **byte-identical to what is there today** — only the comment and the scope change.

```bash
# --- Guard 2: REDIRECT_RE — the PROSE sentinel (re-scoped by change 0070) --------------------
# No skill BODY may show the pre-0059 anti-pattern `render-board.sh ... > .../BOARD.md`. This
# guard's target is documentation, and its narrow shape is CORRECT for documentation:
#   render-board\.sh.{0,200}[[:space:]]>[[:space:]].{0,200}/BOARD\.md
#   - `.{0,200}` bounded any-char gaps (NOT `[^>]*`): the historical redirect's destination is a
#     bracket placeholder `<metadata working tree>/<changes_dir>/BOARD.md`, whose `>` characters
#     and internal spaces a `[^>]*` class could never cross — so `[^>]*` was BLIND to the exact
#     reintroduction shape this sentinel exists to catch. `.` crosses placeholder `>`s freely.
#   - `[[:space:]]>[[:space:]]` whitespace-bounded operator: in PROSE a real ` > ` redirect has a
#     space on both sides, whereas a placeholder's closing bracket is `tree>` / `dir>/` (letter
#     before `>`, or no space after) — so every `<...>` placeholder is structurally excluded.
#   - `/BOARD\.md` (slash required, not bare `BOARD.md`): rejects a flattened markdown blockquote
#     (`\n> ` -> ` > `) that lands a bare "BOARD.md" prose word inside the window. Blockquotes
#     genuinely appear in docket-status and docket-implement-next: a LIVE false-positive class.
#
# WHY THIS REGEX IS AIMED AT PROSE AND NOTHING ELSE (change 0070):
# This regex defends PROSE. Shell scripts are guarded by the repo-wide WRITE sentinel above
# (`render_board_write_free`), which can be far wider — it prohibits the write outright rather
# than recognizing the target — precisely BECAUSE prose hazards (bracket placeholders, flattened
# blockquotes) cannot occur in a script. Read the asymmetry the other way and it is the whole
# lesson of this change: the shapes that force THIS regex to be narrow (spaces around `>`) are the
# very shapes a bash script never has, and the shapes a bash script DOES have (`>"$f"`, `>>`, `>|`,
# a variable target) are ones this regex can never see AT ANY WIDTH. Change 0069 aimed it at
# scripts/docket-status.sh, where the hazard profile is inverted; change 0070 aimed that script at
# the write sentinel instead and returned this regex to the job it is shaped for.
# DO NOT widen this regex to cover shell, and DO NOT delete it as redundant: one guard, one hole.
#
# THE SAME REGEX serves the positive control and the scan, so weakening it (e.g. back to `[^>]*`)
# trips the positive control loudly rather than silently hollowing out the scan.
REDIRECT_RE='render-board\.sh.{0,200}[[:space:]]>[[:space:]].{0,200}/BOARD\.md'

# Positive control ("test the test"): the historical bracket-placeholder redirect that WAS in
# this codebase pre-0059 MUST still be flagged by the guard. If a future edit weakens REDIRECT_RE
# so it can no longer cross placeholder brackets, this assertion fails — not the silent scan.
HISTORICAL_REDIRECT='"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/render-board.sh --changes-dir <metadata working tree>/<changes_dir> > <metadata working tree>/<changes_dir>/BOARD.md'
assert "guard regex flags the historical bracket-placeholder redirect (positive control)" \
  'printf "%s" "$HISTORICAL_REDIRECT" | tr "\n" " " | grep -Eq "$REDIRECT_RE"'

# Negative-control (change 0070): a flattened markdown blockquote must NOT trip it. This is the
# false-positive class that keeps the regex narrow — assert it, don't just assert it in a comment.
BLOCKQUOTE_PROSE='The renderer prints to stdout. > Never hand-edit BOARD.md — it is generated.'
assert "guard regex does NOT flag a flattened markdown blockquote (false-positive control)" \
  '! printf "%s" "$BLOCKQUOTE_PROSE" | tr "\n" " " | grep -Eq "$REDIRECT_RE"'

# Negative scan: no CURRENT skill body redirects render-board.sh stdout into BOARD.md.
redirect_found=0
for f in "$REPO"/skills/*/SKILL.md; do
  if tr '\n' ' ' < "$f" | grep -Eq "$REDIRECT_RE"; then
    echo "  (direct render-board.sh -> BOARD.md redirect found in: $f)"
    redirect_found=1
  fi
done
assert "no skills/*/SKILL.md redirects render-board.sh stdout directly into BOARD.md" \
  '[ "$redirect_found" -eq 0 ]'

# Anti-vacuity: the skills glob must be non-empty, or the scan above passes for the wrong reason.
assert "the skills/*/SKILL.md scan is not vacuous" \
  '[ "$(ls "$REPO"/skills/*/SKILL.md | wc -l)" -ge 5 ]'

# NOTE (change 0070): 0069's second scan — the same REDIRECT_RE aimed at scripts/docket-status.sh
# — is GONE, replaced by the write sentinel above. It was a prose-tuned regex pointed at a bash
# script, blind to every idiomatic shell redirect form; the sentinel catches all of them, and
# catches them in EVERY script rather than the one that happened to be named.
```

- [ ] **Step 2: Run the test to verify it passes**

Run: `cd /Users/homer/dev/docket/.worktrees/redirect-regex-board-write-guard && bash tests/test_render_board.sh 2>&1 | grep -E "guard regex|skills/|vacuous|^(PASS|FAIL)"`

Expected:
```
ok - guard regex flags the historical bracket-placeholder redirect (positive control)
ok - guard regex does NOT flag a flattened markdown blockquote (false-positive control)
ok - no skills/*/SKILL.md redirects render-board.sh stdout directly into BOARD.md
ok - the skills/*/SKILL.md scan is not vacuous
PASS
```

And the deleted scan must be gone — this must print nothing:

Run: `grep -n "scripts/docket-status.sh never redirects" tests/test_render_board.sh`
Expected: no output (exit 1).

- [ ] **Step 3: Verify the regex is byte-identical to `origin/main` (it must NOT have been widened)**

Run:
```bash
cd /Users/homer/dev/docket/.worktrees/redirect-regex-board-write-guard
diff <(git show origin/main:tests/test_render_board.sh | grep '^REDIRECT_RE=') \
     <(grep '^REDIRECT_RE=' tests/test_render_board.sh) && echo "REDIRECT_RE unchanged"
```
Expected: `REDIRECT_RE unchanged`. A diff here means the constraint was violated — restore the original bytes.

- [ ] **Step 4: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/redirect-regex-board-write-guard
git add tests/test_render_board.sh
git commit -m "test(0070): re-scope REDIRECT_RE to prose; drop 0069's script scan

The regex is unchanged byte-for-byte and stays narrow — its shape is
correct for documentation, where bracket placeholders and flattened
blockquotes are live false-positive classes. It now scans only
skills/*/SKILL.md; scripts/docket-status.sh is covered by the write
sentinel, which can be far wider precisely because prose hazards cannot
occur in a script. Adds the blockquote false-positive control and an
anti-vacuity check the old scan lacked."
```

---

### Task 3: Guard 3 — continuation-joining for the `--format digest` flag check

**Files:**
- Modify: `tests/test_docket_status.sh:12-33` — the inline-board wiring sentinel: add `join_continuations` and pipe the script through it before the comment filter and tokenizer.

**Interfaces:**
- Consumes: the `join_continuations` contract defined in Task 1 (identical twin, defined locally — this file is a standalone script and shares no library with `tests/test_render_board.sh`, matching how every test file in this repo defines its own `assert`).
- Produces: nothing consumed downstream.

- [ ] **Step 1: Write the failing test — a continuation-line mutation of the flag check**

The existing tokenizer reads one physical line at a time, so a legitimate invocation whose `--format digest` flag sits on a continuation line is a **false positive** (loud, not silent) — and a rogue one hiding behind a continuation is a false negative. Both die with the join.

Append this block immediately after the existing `assert "every render-board.sh invocation in docket-status is the read-only --format digest"` assertion in `tests/test_docket_status.sh` (this is the test; Step 3 adds the function it calls and rewires the scan):

```bash
# --- change 0070: the flag check tokenizes LOGICAL lines, not physical ones -------------------
# The tokenizer below reads one PHYSICAL line at a time, so an invocation split across a
# backslash-continuation is torn in half: the first-line token carries the call WITHOUT its
# --format digest flag (false positive — loud), and any redirect parked on the continuation is
# invisible (false negative — silent). Mutation-test both directions on fixtures, using the SAME
# function the real scan uses, so the fix cannot be true here and broken there.
# TWIN: join_continuations is defined identically in tests/test_render_board.sh (Guard 1). Keep
# the two in step — a broken tokenizer sitting beside a fixed one is how the next author
# cargo-cults the broken one (ledger #64).
digest_tokens(){
  join_continuations "$1" \
    | grep -v '^[[:space:]]*#' \
    | grep -oE '[^;&|]*/render-board\.sh[^;&|]*' || true
}

# A legitimate call whose flag sits on the continuation line: exactly ONE logical invocation, and
# it IS the digest projection. The join is what lets the tokenizer see that.
ct="$tmp/continuation-call.sh"
printf '%s\n' '#!/usr/bin/env bash' \
  'out="$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$cd_dir" \' \
  '  --format digest 2>&2)"' > "$ct"
ct_ungated=0
while IFS= read -r inv; do
  [ -n "$inv" ] || continue
  case "$inv" in
    *"--format digest"*) : ;;
    *) ct_ungated=1 ;;
  esac
done < <(digest_tokens "$ct")
assert "flag check sees a --format digest flag parked on a continuation line (no false positive)" \
  '[ "$ct_ungated" -eq 0 ]'

# The same shape WITHOUT the flag must still be caught — the join must not launder a rogue call.
rt="$tmp/continuation-rogue.sh"
printf '%s\n' '#!/usr/bin/env bash' \
  'out="$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$cd_dir" \' \
  '  2>&2)"' > "$rt"
rt_ungated=0
while IFS= read -r inv; do
  [ -n "$inv" ] || continue
  case "$inv" in
    *"--format digest"*) : ;;
    *) rt_ungated=1 ;;
  esac
done < <(digest_tokens "$rt")
assert "flag check still catches an ungated call split across a continuation line" \
  '[ "$rt_ungated" -eq 1 ]'
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd /Users/homer/dev/docket/.worktrees/redirect-regex-board-write-guard && bash tests/test_docket_status.sh 2>&1 | grep -E "continuation|^(PASS|FAIL)"`

Expected: FAIL — `bash: join_continuations: command not found` from `digest_tokens`, so both new assertions report `NOT OK`.

- [ ] **Step 3: Add `join_continuations` and rewire the real scan through it**

In `tests/test_docket_status.sh`, add the function directly above the existing sentinel block (before the `# --- inline-board wiring sentinel` comment):

```bash
# Folds backslash-continuations into logical lines, so the tokenizer below sees INVOCATIONS rather
# than physical lines. Identical twin in tests/test_render_board.sh (Guard 1) — keep in step.
# awk, not sed: BSD sed does not portably treat `\n` in an s/// LHS as a newline.
join_continuations(){
  awk '{ while (sub(/\\$/, "")) { if ((getline nxt) > 0) { $0 = $0 nxt } else { break } } print }' "$1"
}
```

Then change the real scan's input from the line-oriented pipeline to the joined one. Replace:

```bash
done < <(grep -v '^[[:space:]]*#' "$SCRIPT" | grep -oE '[^;&|]*/render-board\.sh[^;&|]*' || true)
```

with:

```bash
done < <(digest_tokens "$SCRIPT")
```

and move the `digest_tokens` definition from Step 1's block up beside `join_continuations`, so the real scan and the mutation fixtures run the SAME tokenizer (a battery that tests a copy is decoration). Extend the sentinel's existing comment with one line:

```bash
# Change 0070: tokenizes LOGICAL lines (continuations joined first) — a flag or a redirect parked
# on a continuation line is otherwise torn away from the call it belongs to.
```

- [ ] **Step 4: Run to verify it passes**

Run: `cd /Users/homer/dev/docket/.worktrees/redirect-regex-board-write-guard && bash tests/test_docket_status.sh 2>&1 | grep -E "continuation|read-only --format digest|routes the inline|^(PASS|FAIL)"`

Expected:
```
ok - docket-status routes the inline board render through board-refresh.sh
ok - every render-board.sh invocation in docket-status is the read-only --format digest
ok - flag check sees a --format digest flag parked on a continuation line (no false positive)
ok - flag check still catches an ungated call split across a continuation line
PASS
```

- [ ] **Step 5: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/redirect-regex-board-write-guard
git add tests/test_docket_status.sh
git commit -m "test(0070): join continuations before tokenizing the digest flag check

The flag sentinel read one physical line at a time, so an invocation
split across a backslash-continuation was torn in half — the flag (false
positive, loud) or a redirect (false negative, silent) fell off the token.
Joins logical lines first, through the same digest_tokens() the mutation
fixtures use, so the fix cannot be true in the battery and broken in the
scan."
```

---

### Task 4: Whole-suite verification against the base

**Files:**
- Modify: none (verification only).

**Interfaces:**
- Consumes: Tasks 1–3, all committed.
- Produces: the evidence the PR rests on.

- [ ] **Step 1: Confirm no production file was touched**

The change is test-only — this is a Global Constraint, and it is cheap to prove.

Run:
```bash
cd /Users/homer/dev/docket/.worktrees/redirect-regex-board-write-guard
git diff --name-only origin/main...HEAD
```
Expected — exactly these, and nothing under `scripts/`:
```
docs/superpowers/plans/2026-07-13-redirect-regex-board-write-guard-plan.md
tests/test_docket_status.sh
tests/test_render_board.sh
```

- [ ] **Step 2: Run the full suite in ONE foreground call**

Per ledger #34/#66: a RED suite is a hypothesis, not a verdict — and per the docket loop's own rule, the suite runs in the FOREGROUND (a backgrounded ~10-minute suite stalls the loop).

Run (single Bash call, `timeout 600000`):
```bash
cd /Users/homer/dev/docket/.worktrees/redirect-regex-board-write-guard
for t in tests/*.sh; do
  printf '=== %s: ' "$t"
  if out="$(bash "$t" 2>&1)"; then printf 'PASS\n'; else printf 'FAIL\n'; printf '%s\n' "$out" | tail -20; fi
done
```
Expected: every test PASS.

- [ ] **Step 3: If anything is RED, differential it against the unmodified base BEFORE calling it a regression**

An ambient `DOCKET_SCRIPTS_DIR` in the dev shell, or an environment fact (`origin/HEAD`, umask, timeouts), has twice produced a RED suite that was not a regression (ledger #34/#66).

```bash
cd /Users/homer/dev/docket        # the primary tree, on main
for t in tests/<the-failing-test>.sh; do env -u DOCKET_SCRIPTS_DIR bash "$t" >/dev/null 2>&1 \
  && echo "base PASS: $t" || echo "base FAIL: $t"; done
```
If the same test fails on unmodified `origin/main`, it is environment-bound, not a regression — record the differential in the results file. If it passes on base and fails on the branch, it IS a regression: fix it before the PR.

- [ ] **Step 4: Commit (only if Step 3 required a fix)**

```bash
cd /Users/homer/dev/docket/.worktrees/redirect-regex-board-write-guard
git add -A tests/
git commit -m "test(0070): fix <the regression Step 3 surfaced>"
```

---

## Self-Review

**1. Spec coverage.**

| Spec requirement | Task |
|---|---|
| Guard 1: repo-wide write sentinel, `scripts/*.sh` minus `board-refresh.sh`, glob-derived | Task 1 (widened to a `find` sweep so `scripts/lib/*.sh` is covered) |
| Guard 1: join continuations → strip comments → tokenize per invocation | Task 1, Step 1 (plus fd-dup erasure, which the spec's "fd dups allowed" requires and the `&`-splitting tokenizer makes mandatory) |
| Guard 1: no file-directed redirect; fd dups allowed | Task 1, Step 1, pipeline stage 5 |
| Guard 2: `REDIRECT_RE` kept unwidened, re-scoped to `skills/*/SKILL.md` | Task 2 (byte-identity asserted in Step 3) |
| Guard 2: comment re-derived, gaining the prose-vs-shell asymmetry sentence | Task 2, Step 1 |
| Guard 3: flag check stays, inherits continuation-joining | Task 3 |
| Mutation battery: 6 evasions RED, `2>&2` GREEN | Task 1, Step 1 (rows 1–6b RED, rows 7–8 GREEN) + Step 4 (live-tree mutation) |
| Test-only; no production changes | Global Constraints; asserted in Task 4, Step 1 |

**2. Placeholder scan.** No TBDs; every step carries the literal bytes to paste and the exact command with its expected output.

**3. Type consistency.** `join_continuations <file>` and `render_board_write_free <file>` are named and used identically in Tasks 1 and 3; `digest_tokens <file>` is defined once in Task 3 and used by both its fixtures and its real scan. The fd-dup control string is copied verbatim from `scripts/docket-status.sh:172` in both the battery (Task 1) and the reconcile record.

**Deviations from the spec, and why** — to disclose in the results file:
1. **`find "$REPO/scripts" -name '*.sh'` instead of a flat `scripts/*.sh` glob.** The spec says "iterate `scripts/*.sh`"; the repo has `scripts/lib/*.sh`, which that glob does not reach. Ledger #64 is explicit that a gated operation's call-site list must be derived, not hand-shaped — a flat glob is a hand-shaped list wearing a glob's clothes.
2. **Erasing fd dups BEFORE tokenizing** — not in the spec's three-step pipeline, but forced by it: the tokenizer splits on `; & |`, and the `&` in `2>&2` cuts the token mid-redirect, leaving a dangling `>` that would fire on the codebase's own correct invocation (the spec's designated GREEN control). Without this step the false-positive control cannot pass.
3. **Guard 1 rejects `2>/dev/null`** (a stderr-to-file redirect). Strictly implied by "no file-directed redirect", but worth stating: routing this renderer's stderr is done with the fd dup already in use.
4. **A known, accepted gap: `| tee`.** The tokenizer ends an invocation at `|`, so a pipe into a writer is not caught. Documented in the guard's comment; the deferred filesystem-effect test is the answer to that class, with the same trigger the spec already gave it.
