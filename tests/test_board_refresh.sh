#!/usr/bin/env bash
# tests/test_board_refresh.sh — verifies change 0059: scripts/board-refresh.sh is the sole gated,
# surface-aware writer of BOARD.md. It composes on top of render-board.sh (unchanged, stdout-only)
# and owns the write decision: `inline` present in the caller's resolved --surfaces tokens renders
# and atomically replaces BOARD.md; `inline` absent leaves the filesystem untouched — no create,
# no truncate, no delete. A missing --surfaces flag is a wiring bug (exit 2); an explicit empty
# value is valid ("no surfaces" -> no-op). Unknown tokens warn but never abort or block inline.
# Mirrors tests/test_render_board.sh's hermetic-fixture pattern. Run: bash tests/test_board_refresh.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$REPO/scripts/board-refresh.sh"
RENDER="$REPO/scripts/render-board.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

assert "script exists and is executable" '[ -x "$SCRIPT" ]'

# changes-dir fixture: real filesystem tree the helper reads/writes.
tmp="$(mktemp -d)"
# scratch: captured stdout/stderr + reference renders, kept OUT of the changes dir so leftover-
# temp-file assertions on $tmp are not polluted by test bookkeeping.
work="$(mktemp -d)"
trap 'rm -rf "$tmp" "$work"' EXIT
mkdir -p "$tmp/active" "$tmp/archive"

cat > "$tmp/active/0001-alpha.md" <<'EOF'
---
id: 1
slug: alpha
title: Alpha feature
status: proposed
priority: medium
depends_on: []
spec:
EOF

# reference render (no --repo) — the byte-exact target for every inline-enabled case
"$RENDER" --changes-dir "$tmp" > "$work/expected.md" 2>"$work/expected.err"

# reference render (with --repo) — proves --repo forwards through unchanged
"$RENDER" --changes-dir "$tmp" --repo o/r > "$work/expected-repo.md" 2>"$work/expected-repo.err"

count_files(){ find "$tmp" -maxdepth 1 -type f | wc -l | tr -d ' '; }

# --- 1: --surfaces "inline" renders and writes BOARD.md byte-identical to render-board.sh -----
rm -f "$tmp/BOARD.md"
"$SCRIPT" --changes-dir "$tmp" --surfaces "inline" >"$work/out1" 2>"$work/err1"; rc1=$?
assert "inline: exit 0" '[ "$rc1" -eq 0 ]'
assert "inline: BOARD.md created" '[ -f "$tmp/BOARD.md" ]'
assert "inline: BOARD.md byte-identical to render-board.sh stdout" 'diff -u "$work/expected.md" "$tmp/BOARD.md"'
assert "inline: announces the write on stdout" 'grep -qF "board-refresh: inline rendered" "$work/out1"'
assert "inline: no leftover temp files in changes dir" '[ "$(count_files)" -eq 1 ]'

# --- 1b: --repo forwards verbatim to render-board.sh ------------------------------------------
rm -f "$tmp/BOARD.md"
"$SCRIPT" --changes-dir "$tmp" --surfaces "inline" --repo o/r >"$work/out1b" 2>"$work/err1b"; rc1b=$?
assert "inline+repo: exit 0" '[ "$rc1b" -eq 0 ]'
assert "inline+repo: BOARD.md byte-identical to render-board.sh --repo output" \
  'diff -u "$work/expected-repo.md" "$tmp/BOARD.md"'

# --- 2: --surfaces "" (empty value) -> exit 2 (change 0071: the polarity reversal) -------------
# Was: a no-op exit 0, byte-identical to a deliberately disabled repo. That ambiguity is the whole
# bug — an agent whose $BOARD_SURFACES never resolved sent "" and got a SILENT stale board with a
# success exit code. Empty now means exactly one thing: nobody resolved this.
rm -f "$tmp/BOARD.md"
"$SCRIPT" --changes-dir "$tmp" --surfaces "" >"$work/out2" 2>"$work/err2"; rc2=$?
assert "empty surfaces: exit 2 (wiring bug, not a configuration)" '[ "$rc2" -eq 2 ]'
assert "empty surfaces: BOARD.md not created" '[ ! -e "$tmp/BOARD.md" ]'
assert "empty surfaces: names the unresolved-config cause on stderr" \
  'grep -qF "empty --surfaces value" "$work/err2"'
assert "empty surfaces: points at the positive off-token on stderr" \
  'grep -qF "none" "$work/err2"'
assert "empty surfaces: no leftover temp files in changes dir" '[ "$(count_files)" -eq 0 ]'

# --- 1c: --surfaces " " (whitespace-only) -> exit 2, same as truly empty (change 0071 review, ----
# finding 6, defence-in-depth): it passes the `-n "$SURFACES"` check but tokenizes to zero words,
# the same "nobody resolved this" hole with a byte of padding.
rm -f "$tmp/BOARD.md"
"$SCRIPT" --changes-dir "$tmp" --surfaces " " >"$work/out1c" 2>"$work/err1c"; rc1c=$?
assert "whitespace-only surfaces: exit 2 (treated identically to empty)" '[ "$rc1c" -eq 2 ]'
assert "whitespace-only surfaces: BOARD.md not created" '[ ! -e "$tmp/BOARD.md" ]'
assert "whitespace-only surfaces: names the unresolved-config cause on stderr" \
  'grep -qF "empty --surfaces value" "$work/err1c"'
assert "whitespace-only surfaces: no leftover temp files in changes dir" '[ "$(count_files)" -eq 0 ]'

# --- 2b: --surfaces "none" -> the deliberate off-state: no-op, exit 0 --------------------------
rm -f "$tmp/BOARD.md"
"$SCRIPT" --changes-dir "$tmp" --surfaces "none" >"$work/out2b" 2>"$work/err2b"; rc2b=$?
assert "none: exit 0" '[ "$rc2b" -eq 0 ]'
assert "none: BOARD.md not created" '[ ! -e "$tmp/BOARD.md" ]'
assert "none: announces the deliberate no-op on stdout" \
  'grep -qF "board-refresh: board disabled (none)" "$work/out2b"'
assert "none: no leftover temp files in changes dir" '[ "$(count_files)" -eq 0 ]'

# --- 2c: `none` is RESERVED and EXCLUSIVE — a contradiction exits 2, never picks a winner ------
rm -f "$tmp/BOARD.md"
"$SCRIPT" --changes-dir "$tmp" --surfaces "none inline" >"$work/out2c" 2>"$work/err2c"; rc2c=$?
assert "none+inline: exit 2 (contradiction)" '[ "$rc2c" -eq 2 ]'
assert "none+inline: BOARD.md not written" '[ ! -e "$tmp/BOARD.md" ]'
assert "none+inline: says none is exclusive on stderr" 'grep -qF "exclusive" "$work/err2c"'
# order-independent: the contradiction is a property of the token SET, not of token order
rm -f "$tmp/BOARD.md"
"$SCRIPT" --changes-dir "$tmp" --surfaces "inline none" >"$work/out2d" 2>"$work/err2d"; rc2d=$?
assert "inline+none (reversed order): exit 2" '[ "$rc2d" -eq 2 ]'
assert "inline+none (reversed order): BOARD.md not written" '[ ! -e "$tmp/BOARD.md" ]'

# --- 2e/2f: change 0071 review, finding 5 — `other_seen=1` is set by the `inline)`, `github)`,
# AND `*)` arms alike, so `none` combined with `github` or an unrecognized token must ALSO exit 2,
# not just `none`+`inline`. Nothing above exercised the `github)` or `*)` arms' `other_seen=1` —
# deleting either would silently turn these into a no-op exit 0 with the suite green.
rm -f "$tmp/BOARD.md"
"$SCRIPT" --changes-dir "$tmp" --surfaces "none github" >"$work/out2e" 2>"$work/err2e"; rc2e=$?
assert "none+github: exit 2 (none is exclusive)" '[ "$rc2e" -eq 2 ]'
assert "none+github: BOARD.md not written" '[ ! -e "$tmp/BOARD.md" ]'
assert "none+github: says none is exclusive on stderr" 'grep -qF "exclusive" "$work/err2e"'

rm -f "$tmp/BOARD.md"
"$SCRIPT" --changes-dir "$tmp" --surfaces "none bogus" >"$work/out2f" 2>"$work/err2f"; rc2f=$?
assert "none+bogus: exit 2 (none is exclusive)" '[ "$rc2f" -eq 2 ]'
assert "none+bogus: BOARD.md not written" '[ ! -e "$tmp/BOARD.md" ]'
assert "none+bogus: says none is exclusive on stderr" 'grep -qF "exclusive" "$work/err2f"'

# --- 3: --surfaces "github" (inline absent) -> BOARD.md not written ----------------------------
rm -f "$tmp/BOARD.md"
"$SCRIPT" --changes-dir "$tmp" --surfaces "github" >"$work/out3" 2>"$work/err3"; rc3=$?
assert "github-only: exit 0" '[ "$rc3" -eq 0 ]'
assert "github-only: BOARD.md not written" '[ ! -e "$tmp/BOARD.md" ]'
assert "github-only: no leftover temp files in changes dir" '[ "$(count_files)" -eq 0 ]'

# --- 4: --surfaces "inline github" -> BOARD.md written -----------------------------------------
rm -f "$tmp/BOARD.md"
"$SCRIPT" --changes-dir "$tmp" --surfaces "inline github" >"$work/out4" 2>"$work/err4"; rc4=$?
assert "inline+github: exit 0" '[ "$rc4" -eq 0 ]'
assert "inline+github: BOARD.md written" '[ -f "$tmp/BOARD.md" ]'
assert "inline+github: content matches render-board.sh" 'diff -u "$work/expected.md" "$tmp/BOARD.md"'

# --- 5: truncation-trap regression — a pre-existing BOARD.md survives a DISABLED run byte-for-byte
# Change 0059's entire point, carried over unchanged except for the encoding (`""` -> `none`).
# Non-negotiable: disabling the board must never create, truncate, or delete a prior BOARD.md.
rm -f "$tmp/BOARD.md"
printf '# Stale Board\n\nDo not touch.\n' > "$tmp/BOARD.md"
cp "$tmp/BOARD.md" "$work/known-board.md"
"$SCRIPT" --changes-dir "$tmp" --surfaces "none" >"$work/out5" 2>"$work/err5"; rc5=$?
assert "truncation trap (none): exit 0" '[ "$rc5" -eq 0 ]'
assert "truncation trap (none): pre-existing BOARD.md untouched (byte-identical)" \
  'diff -u "$work/known-board.md" "$tmp/BOARD.md"'

# --- 5b: the exit-2 paths must ALSO leave a pre-existing BOARD.md byte-identical ---------------
# A loud failure that truncates the board on its way out would trade a silent stale board for a
# loud destroyed one. Both rejection paths (empty value, exclusivity violation) are asserted.
"$SCRIPT" --changes-dir "$tmp" --surfaces "" >"$work/out5b" 2>"$work/err5b"; rc5b=$?
assert "empty surfaces: exit 2 leaves a pre-existing BOARD.md byte-identical" \
  '[ "$rc5b" -eq 2 ] && diff -u "$work/known-board.md" "$tmp/BOARD.md"'
"$SCRIPT" --changes-dir "$tmp" --surfaces "none inline" >"$work/out5c" 2>"$work/err5c"; rc5c=$?
assert "none+inline: exit 2 leaves a pre-existing BOARD.md byte-identical" \
  '[ "$rc5c" -eq 2 ] && diff -u "$work/known-board.md" "$tmp/BOARD.md"'
rm -f "$tmp/BOARD.md"

# --- 6: unknown token warns, does not enable inline, does not write ----------------------------
rm -f "$tmp/BOARD.md"
"$SCRIPT" --changes-dir "$tmp" --surfaces "bogus" >"$work/out6" 2>"$work/err6"; rc6=$?
assert "unknown token: exit 0" '[ "$rc6" -eq 0 ]'
assert "unknown token: BOARD.md not written" '[ ! -e "$tmp/BOARD.md" ]'
assert "unknown token: warns on stderr" 'grep -qiF "unknown" "$work/err6"'

# --- 6b: an unknown token alongside inline still renders (a typo must never abort the write) ---
rm -f "$tmp/BOARD.md"
"$SCRIPT" --changes-dir "$tmp" --surfaces "inline bogus" >"$work/out6b" 2>"$work/err6b"; rc6b=$?
assert "unknown token + inline: exit 0" '[ "$rc6b" -eq 0 ]'
assert "unknown token + inline: BOARD.md still written" '[ -f "$tmp/BOARD.md" ]'
assert "unknown token + inline: still warns on stderr" 'grep -qiF "unknown" "$work/err6b"'

# --- 7: missing --surfaces flag entirely -> exit 2 (wiring bug, distinct from an empty value) ---
rm -f "$tmp/BOARD.md"
"$SCRIPT" --changes-dir "$tmp" >"$work/out7" 2>"$work/err7"; rc7=$?
assert "missing --surfaces flag: exit 2" '[ "$rc7" -eq 2 ]'
assert "missing --surfaces flag: BOARD.md not written" '[ ! -e "$tmp/BOARD.md" ]'

# --- 8: missing/invalid --changes-dir -> exit 2 -------------------------------------------------
"$SCRIPT" --changes-dir "$tmp/does-not-exist" --surfaces "inline" >"$work/out8" 2>"$work/err8"; rc8=$?
assert "invalid --changes-dir: exit 2" '[ "$rc8" -eq 2 ]'

"$SCRIPT" --surfaces "inline" >"$work/out8b" 2>"$work/err8b"; rc8b=$?
assert "missing --changes-dir flag: exit 2" '[ "$rc8b" -eq 2 ]'

# --- 9: enabled-render FAILURE must propagate the renderer's real exit code and leave BOARD.md
# untouched. Uses the RENDER_BOARD mock seam to inject a stub that emits partial output then
# exits 7. Without the exit-code fix (the buggy `if ! … ; then rc=$?`), rc is always 0 here.
stub="$work/failing-render.sh"
cat > "$stub" <<'EOF'
#!/usr/bin/env bash
printf 'PARTIAL RENDER GARBAGE — must never reach BOARD.md\n'
exit 7
EOF
chmod +x "$stub"
rm -f "$tmp/BOARD.md"
printf '# Known Good Board\n\nPre-existing, must survive a renderer failure.\n' > "$tmp/BOARD.md"
cp "$tmp/BOARD.md" "$work/pre-fail-board.md"
RENDER_BOARD="$stub" "$SCRIPT" --changes-dir "$tmp" --surfaces "inline" >"$work/out9" 2>"$work/err9"; rc9=$?
assert "render failure: propagates the renderer's real exit code (7, not 0)" '[ "$rc9" -eq 7 ]'
assert "render failure: pre-existing BOARD.md untouched (byte-identical)" \
  'diff -u "$work/pre-fail-board.md" "$tmp/BOARD.md"'
assert "render failure: no leftover temp file in changes dir (only BOARD.md remains)" \
  '[ "$(count_files)" -eq 1 ]'
assert "render failure: reports the failure on stderr" 'grep -qF "render-board.sh failed" "$work/err9"'

# --- 10: a successful inline write leaves BOARD.md at 0644 (not the 0600 mktemp creates) --------
rm -f "$tmp/BOARD.md"
"$SCRIPT" --changes-dir "$tmp" --surfaces "inline" >"$work/out10" 2>"$work/err10"; rc10=$?
assert "perms: inline write exit 0" '[ "$rc10" -eq 0 ]'
mode="$(stat -f '%OLp' "$tmp/BOARD.md" 2>/dev/null || stat -c '%a' "$tmp/BOARD.md")"
assert "perms: BOARD.md is mode 644 after a successful inline write" '[ "$mode" = "644" ]'

# --- 11: enabled render that exits 0 with EMPTY output must NOT overwrite BOARD.md. render-board.sh
# always emits a `# Backlog` header on a clean run, so this cannot happen with the real renderer —
# but the guard must be self-contained (a future render-board regression / the mock seam could hit
# it). Belt-and-suspenders companion to test #9 (non-zero exit). Mirrors what a real renderer could
# do: exit 0, print nothing.
empty_stub="$work/empty-render.sh"
cat > "$empty_stub" <<'EOF'
#!/usr/bin/env bash
# Emits nothing, exits 0 — the exit-0-but-empty case.
exit 0
EOF
chmod +x "$empty_stub"
rm -f "$tmp/BOARD.md"
printf '# Known Good Board\n\nPre-existing, must survive an empty render.\n' > "$tmp/BOARD.md"
cp "$tmp/BOARD.md" "$work/pre-empty-board.md"
RENDER_BOARD="$empty_stub" "$SCRIPT" --changes-dir "$tmp" --surfaces "inline" >"$work/out11" 2>"$work/err11"; rc11=$?
assert "empty render: exits non-zero (1), not 0" '[ "$rc11" -eq 1 ]'
assert "empty render: pre-existing BOARD.md untouched (byte-identical)" \
  'diff -u "$work/pre-empty-board.md" "$tmp/BOARD.md"'
assert "empty render: no leftover temp file in changes dir (only BOARD.md remains)" \
  '[ "$(count_files)" -eq 1 ]'
assert "empty render: reports empty output on stderr" 'grep -qF "empty output" "$work/err11"'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
