#!/usr/bin/env bash
# tests/test_docket_status.sh — verifies change 0058: the docket-status orchestrator.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$REPO/scripts/docket-status.sh"
DOCKET_BASH_PATH=""
for runtime_candidate in "$(command -v bash)" /opt/homebrew/bin/bash /usr/local/bin/bash; do
  [ -x "$runtime_candidate" ] || continue
  [ "$(LC_ALL=C "$runtime_candidate" --version 2>/dev/null | sed -n 's/^GNU bash, version \([0-9][0-9]*\)\..*/\1/p')" -ge 4 ] 2>/dev/null || continue
  DOCKET_BASH_PATH="$runtime_candidate"; break
done
: "${DOCKET_BASH_PATH:?tests require an absolute GNU Bash 4+ runtime}"
export DOCKET_BASH_PATH
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

assert "script exists and is executable" '[ -x "$SCRIPT" ]'
assert "--help exits 0 and prints usage" '"$SCRIPT" --help 2>&1 | grep -qi "usage"'

# Scratch dir shared by every fixture in this file: the continuation-joining battery below, the
# bootstrap-gate fixtures via write_fixture() (further down), and everything after it.
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT

# Folds backslash-continuations into logical lines, so the tokenizer below sees INVOCATIONS rather
# than physical-line fragments. TWIN of tests/test_render_board.sh's join_continuations (Guard 1) —
# defined identically here rather than shared, because this file is a standalone script with no
# library to share (matching how every test file in this repo defines its own `assert`). Keep the
# two in step: a broken tokenizer sitting beside a fixed one is how the next author cargo-cults the
# broken one (ledger #64).
# awk, not sed: BSD sed does not portably treat `\n` in an s/// LHS as a newline.
join_continuations(){
  awk '{ while (sub(/\\$/, "")) { if ((getline nxt) > 0) { $0 = $0 nxt } else { break } } print }' "$1"
}

# digest_tokens — render-board.sh invocation tokens, tokenized from LOGICAL lines rather than
# physical ones. Shared by the real scan below (over "$SCRIPT") AND the mutation fixtures further
# down, so the fix cannot be true in the battery and broken in the scan.
# PIPELINE ORDER IS LOAD-BEARING, mirroring tests/test_render_board.sh's normalize_source: comments
# OUT FIRST — drop whole-line ones, then strip trailing ones — and ONLY THEN join continuations.
# The obvious order (join, then strip) is EXPLOITABLE. Bash comments are PHYSICAL-LINE scoped: a
# trailing backslash does NOT continue a comment onto the next line. So in
#     # regenerate the board \
#     "$SCRIPTS_DIR"/render-board.sh --changes-dir "$d" 2>&2
# the second line REALLY EXECUTES as a live, ungated invocation (proven in test_render_board.sh's
# Guard 1 against a stub renderer). Joining first folds that live line INTO the comment, and the
# comment-drop then deletes both — laundering the ungated call into silence. Stripping comments
# first leaves the live invocation standing alone, where the tokenizer below still catches it (the
# "comment ending in backslash, followed by a live call" row in the battery pins this ordering).
digest_tokens(){
  join_continuations <(
    grep -v '^[[:space:]]*#' "$1" | sed 's/[[:space:]]#.*$//'
  ) | grep -oE '[^;&|]*/render-board\.sh[^;&|]*' || true
}

# --- inline-board wiring sentinel (change 0059, narrowed by change 0069, tokenizer fixed 0070) ---
# 0059's rule: the inline BOARD.md *write* has exactly ONE gated path — board-refresh.sh — so the
# orchestrator must never render-and-write the board itself. 0069 adds a READ-ONLY consumer of the
# same renderer (`--format digest`, piped straight to the report, no file touched), so the guard
# can no longer be "never mention render-board.sh." It is narrowed to what it actually protects:
# every render-board.sh invocation in this script must be the read-only digest projection.
# Tokenized PER INVOCATION (not per line): a line carrying a gated and an ungated call side by
# side must not be whitewashed by the gated one. Comment lines are stripped first — prose that
# merely names the script is not an invocation.
# Change 0070: tokenizes LOGICAL lines (continuations joined first, via digest_tokens above) — a
# flag or a redirect parked on a continuation line is otherwise torn away from the call it belongs
# to. See digest_tokens' comment for why comments must be stripped BEFORE the join, not after.
assert "docket-status routes the inline board render through board-refresh.sh" \
  'grep -qF "/board-refresh.sh" "$SCRIPT"'

ungated_render=0
digest_scan_count=0
while IFS= read -r inv; do
  [ -n "$inv" ] || continue
  digest_scan_count=$((digest_scan_count + 1))
  case "$inv" in
    *"--format digest"*) : ;;
    *) ungated_render=1; echo "  (ungated render-board.sh invocation: $inv)" ;;
  esac
done < <(digest_tokens "$SCRIPT")
assert "every render-board.sh invocation in docket-status is the read-only --format digest" \
  '[ "$ungated_render" -eq 0 ]'
# Anti-vacuity (Task 1's convention, tests/test_render_board.sh): a scan over zero invocations
# passes for the wrong reason — deleting the renderer call from docket-status.sh entirely would
# leave the assertion above green with nothing to check. Assert the scan actually saw one.
assert "the digest-flag scan is not vacuous (it saw at least one render-board.sh invocation)" \
  '[ "$digest_scan_count" -ge 1 ]'

# --- change 0070: the flag check tokenizes LOGICAL lines, not physical ones -------------------
# What this sentinel guards, and ONLY this: whether each render-board.sh invocation in
# docket-status.sh carries --format digest (the read-only projection). It has NO redirect/write
# visibility — digest_tokens() never greps for a `>`, before this fix or after it, and no fixture
# below constructs one. Whether a call WRITES a file is REDIRECT_RE's and the write sentinel's job
# (Guard 1, tests/test_render_board.sh) — that guard's own comment states the two catch different
# holes and neither subsumes the other; nothing here changes that boundary, and nothing here
# licenses deleting either guard as redundant with this one.
#
# The tokenizer above used to read one PHYSICAL line at a time, so an invocation whose --format
# digest flag sat on a backslash-continuation was torn in half: the first-line token carried the
# call WITHOUT the flag it actually has — a loud FALSE POSITIVE (a legitimately gated call flagged
# as ungated; the "flag check sees a --format digest flag parked on a continuation line" row below
# proves the join fixes it). A call genuinely missing the flag was already caught either way — the
# flag's absence from whichever physical line matches is what the check keys on, split or not — so
# the "flag check still catches an ungated call split across a continuation line" row below is a
# no-regression proof, not evidence of a prior miss. The third row locks a narrower, real hazard
# specific to a JOIN-based tokenizer: digest_tokens() strips comments BEFORE joining continuations,
# and getting that order backwards would fold a live invocation into a preceding
# backslash-terminated comment, deleting both when the comment is dropped (verified empirically:
# swapping the order launders that exact fixture to zero tokens). The "ordering" row below pins
# that the shipped order does not do that.
digest_mut="$tmp/mut-digest"; mkdir -p "$digest_mut"

# A legitimate call whose flag sits on the continuation line: exactly ONE logical invocation, and
# it IS the digest projection. The join is what lets the tokenizer see that (no false positive).
digest_ct="$digest_mut/continuation-call.sh"
printf '%s\n' '#!/usr/bin/env bash' \
  'out="$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$cd_dir" \' \
  '  --format digest 2>&2)"' > "$digest_ct"
digest_ct_ungated=0
while IFS= read -r inv; do
  [ -n "$inv" ] || continue
  case "$inv" in
    *"--format digest"*) : ;;
    *) digest_ct_ungated=1 ;;
  esac
done < <(digest_tokens "$digest_ct")
assert "flag check sees a --format digest flag parked on a continuation line (no false positive)" \
  '[ "$digest_ct_ungated" -eq 0 ]'

# The same shape WITHOUT the flag must still be caught — the join must not launder a rogue call.
digest_rt="$digest_mut/continuation-rogue.sh"
printf '%s\n' '#!/usr/bin/env bash' \
  'out="$("$SCRIPTS_DIR"/render-board.sh --changes-dir "$cd_dir" \' \
  '  2>&2)"' > "$digest_rt"
digest_rt_ungated=0
while IFS= read -r inv; do
  [ -n "$inv" ] || continue
  case "$inv" in
    *"--format digest"*) : ;;
    *) digest_rt_ungated=1 ;;
  esac
done < <(digest_tokens "$digest_rt")
assert "flag check still catches an ungated call split across a continuation line" \
  '[ "$digest_rt_ungated" -eq 1 ]'

# THE ORDERING PROOF: a comment line ending in a backslash, followed by a LIVE ungated invocation.
# Bash comments do not continue across a trailing backslash, so the second line really executes —
# this is the exact laundering shape a join-before-strip order would hide (see digest_tokens'
# comment above). Comments must be stripped BEFORE continuations are joined: get the order wrong
# and this row goes silently green, because the live invocation gets folded into the comment and
# deleted along with it.
digest_lc="$digest_mut/comment-then-live.sh"
printf '%s\n' '#!/usr/bin/env bash' \
  '# regenerate the board \' \
  '"$SCRIPTS_DIR"/render-board.sh --changes-dir "$cd_dir" 2>&2' > "$digest_lc"
digest_lc_ungated=0
while IFS= read -r inv; do
  [ -n "$inv" ] || continue
  case "$inv" in
    *"--format digest"*) : ;;
    *) digest_lc_ungated=1 ;;
  esac
done < <(digest_tokens "$digest_lc")
assert "flag check still catches a live invocation following a comment that ends in backslash (ordering)" \
  '[ "$digest_lc_ungated" -eq 1 ]'

# Bootstrap gate: stub docket-config.sh --export via CONFIG_EXPORT_CMD (a hermetic fixture
# script emitting the eval-able KEY=value block), and assert the gate's exit code + remedy text.
write_fixture(){
  cat > "$tmp/fixture-export.sh" <<EOF
#!/usr/bin/env bash
printf '%s\n' \
  'BOOTSTRAP=$1' \
  'METADATA_BRANCH=docket' \
  'INTEGRATION_BRANCH=main' \
  'DOCKET_MODE=docket' \
  'METADATA_WORKTREE=.docket' \
  'CHANGES_DIR=docs/changes' \
  'ADRS_DIR=docs/adrs' \
  'RESULTS_DIR=docs/results' \
  'BOARD_SURFACES=inline'
EOF
}

# Hermetic GIT stub for the bootstrap-gate tests: these don't exercise sync behavior, so
# route git through a no-op stub and run inside a scratch dir — never the real docket repo.
cat > "$tmp/stub-git.sh" <<'EOF'
#!/usr/bin/env bash
echo "stub-git: $*" >&2
exit 0
EOF
chmod +x "$tmp/stub-git.sh"
mkdir -p "$tmp/scratch"

write_fixture STOP_MIGRATE
(cd "$tmp/scratch" && CONFIG_EXPORT_CMD="bash $tmp/fixture-export.sh" GIT="$tmp/stub-git.sh" "$SCRIPT" --board-only >/dev/null 2>"$tmp/err.txt")
rc=$?
assert "STOP_MIGRATE exits non-zero" '[ $rc -ne 0 ]'
assert "STOP_MIGRATE prints migrate remedy" 'grep -qi "migrate" "$tmp/err.txt"'

write_fixture CREATE_ORPHAN
(cd "$tmp/scratch" && CONFIG_EXPORT_CMD="bash $tmp/fixture-export.sh" GIT="$tmp/stub-git.sh" "$SCRIPT" --board-only >/dev/null 2>"$tmp/err.txt")
rc=$?
assert "CREATE_ORPHAN exits non-zero" '[ $rc -ne 0 ]'

write_fixture PROCEED
(cd "$tmp/scratch" && CONFIG_EXPORT_CMD="bash $tmp/fixture-export.sh" GIT="$tmp/stub-git.sh" "$SCRIPT" --board-only >/dev/null 2>"$tmp/err.txt")
rc=$?
assert "PROCEED exits zero" '[ $rc -eq 0 ]'

# ensure_and_sync_worktree: hermetic fixture repos (no network, throwaway origin bare repo).
git_repo_setup(){
  local root="$1"
  git init -q -b main "$root/seed" \
    && git -C "$root/seed" -c user.email=t@t -c user.name=t commit -q --allow-empty -m init \
    && git -C "$root/seed" -c user.email=t@t -c user.name=t branch docket \
    && git clone -q --bare "$root/seed" "$root/origin.git"
}

write_sync_fixture(){
  # $1 mode, $2 metadata_branch, $3 metadata_worktree
  cat > "$tmp/fixture-sync.sh" <<EOF
#!/usr/bin/env bash
printf '%s\n' \
  'BOOTSTRAP=PROCEED' \
  'METADATA_BRANCH=$2' \
  'INTEGRATION_BRANCH=main' \
  'DOCKET_MODE=$1' \
  'METADATA_WORKTREE=$3' \
  'CHANGES_DIR=docs/changes' \
  'ADRS_DIR=docs/adrs' \
  'RESULTS_DIR=docs/results' \
  'BOARD_SURFACES=inline'
EOF
}

# main-mode: sync degrades to a no-op-safe `git pull --rebase` on the primary tree.
git_repo_setup "$tmp/main-case"
git clone -q "$tmp/main-case/origin.git" "$tmp/main-case/work" 2>/dev/null
write_sync_fixture main docket .docket
(cd "$tmp/main-case/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-sync.sh" "$SCRIPT" --board-only >/dev/null 2>"$tmp/sync-main-err.txt")
rc=$?
assert "main-mode sync exits zero" '[ $rc -eq 0 ]'

# docket-mode: a missing metadata worktree is created, then synced; exits zero.
git_repo_setup "$tmp/docket-case"
git clone -q "$tmp/docket-case/origin.git" "$tmp/docket-case/work" 2>/dev/null
write_sync_fixture docket docket .docket
assert "metadata worktree absent before run" '[ ! -d "$tmp/docket-case/work/.docket" ]'
(cd "$tmp/docket-case/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-sync.sh" "$SCRIPT" --board-only >/dev/null 2>"$tmp/sync-docket-err.txt")
rc=$?
assert "docket-mode sync exits zero" '[ $rc -eq 0 ]'
assert "docket-mode sync created metadata worktree" '[ -d "$tmp/docket-case/work/.docket" ]'


# board_pass: hermetic changes fixture rendered inline, committed + pushed to a bare remote.
write_board_fixture(){
  # $1 = board_surfaces value
  # METADATA_WORKTREE=. — main-mode's REAL export (docket-config.sh: `main) DOCKET_MODE=main;
  # METADATA_WORKTREE=. ;;`). It used to read `.docket` here, which docket-config.sh never emits in
  # main mode; the fixture got away with it only because docket-status.sh hard-coded `mw="."` in
  # main mode and never read the key. Change 0075 routes every mw through docket_metadata_worktree,
  # which honors the value it is given — so an unfaithful fixture would now test a config that
  # cannot exist.
  cat > "$tmp/fixture-board.sh" <<EOF
#!/usr/bin/env bash
printf '%s\n' \
  'BOOTSTRAP=PROCEED' \
  'METADATA_BRANCH=main' \
  'INTEGRATION_BRANCH=main' \
  'DOCKET_MODE=main' \
  'METADATA_WORKTREE=.' \
  'CHANGES_DIR=docs/changes' \
  'ADRS_DIR=docs/adrs' \
  'RESULTS_DIR=docs/results' \
  'BOARD_SURFACES=$1'
EOF
}

seed_changes_fixture(){
  local root="$1"
  mkdir -p "$root/docs/changes/active" "$root/docs/changes/archive"
  cat > "$root/docs/changes/active/0001-alpha.md" <<'EOF'
---
id: 1
slug: alpha
title: Alpha feature
status: in-progress
priority: high
depends_on: []
spec: docs/superpowers/specs/2026-06-10-alpha.md
branch: feat/alpha
EOF
}

git_repo_setup "$tmp/board-case"
git clone -q "$tmp/board-case/origin.git" "$tmp/board-case/work" 2>/dev/null
seed_changes_fixture "$tmp/board-case/work"
git -C "$tmp/board-case/work" -c user.email=t@t -c user.name=t add docs/changes
git -C "$tmp/board-case/work" -c user.email=t@t -c user.name=t commit -q -m "seed changes fixture"
git -C "$tmp/board-case/work" push -q origin main

write_board_fixture inline
(cd "$tmp/board-case/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-board.sh" "$SCRIPT" --board-only >"$tmp/board-run1.txt" 2>"$tmp/board-run1-err.txt")
rc=$?
assert "board_pass first run exits zero" '[ $rc -eq 0 ]'
assert "board_pass first run reports changed" 'grep -qw "changed" "$tmp/board-run1.txt"'
assert "board_pass first run reports pushed" 'grep -qw "pushed" "$tmp/board-run1.txt"'
assert "board_pass first run reports inline surface" 'grep -qw "inline" "$tmp/board-run1.txt"'

(cd "$tmp/board-case/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-board.sh" "$SCRIPT" --board-only >"$tmp/board-run2.txt" 2>"$tmp/board-run2-err.txt")
rc=$?
assert "board_pass second (clean) run exits zero" '[ $rc -eq 0 ]'
assert "board_pass second run reports clean" 'grep -qw "clean" "$tmp/board-run2.txt"'
assert "board_pass second run reports board line" 'grep -qw "board" "$tmp/board-run2.txt"'
assert "board_pass second run reports inline surface" 'grep -qw "inline" "$tmp/board-run2.txt"'

# Change 0071: an EMPTY BOARD_SURFACES is no longer "board off" — it is an unresolved-config
# wiring bug, and the orchestrator that every skill's Board pass now routes through must fail
# LOUDLY rather than silently skip the board. This is the reference implementation of the
# polarity reversal: the guard that used to read "no surfaces => disabled" is now an assertion
# that the resolver produced a value at all.
write_board_fixture ""
(cd "$tmp/board-case/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-board.sh" "$SCRIPT" --board-only >"$tmp/board-run3.txt" 2>"$tmp/board-run3-err.txt")
rc=$?
assert "board_pass empty-surfaces run exits 2 (fatal wiring bug)" '[ $rc -eq 2 ]'
assert "board_pass empty-surfaces names the unresolved config on stderr" \
  'grep -qF "BOARD_SURFACES" "$tmp/board-run3-err.txt"'
assert "board_pass empty-surfaces NEVER reports 'board off'" \
  '! grep -qxF "board off" "$tmp/board-run3.txt"'
assert "board_pass empty-surfaces emits no inline board line" \
  '! grep -q "board inline" "$tmp/board-run3.txt"'

# --- the deliberate off-state (`none`) keeps TODAY's byte-identical `board off` report ----------
write_board_fixture none
(cd "$tmp/board-case/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-board.sh" "$SCRIPT" --board-only >"$tmp/board-run3n.txt" 2>"$tmp/board-run3n-err.txt")
rc=$?
assert "board_pass none run exits zero" '[ $rc -eq 0 ]'
assert "board_pass none emits a positive 'board off' line" \
  'grep -qxF "board off" "$tmp/board-run3n.txt"'
assert "board_pass none emits no inline board line" \
  '! grep -q "board inline" "$tmp/board-run3n.txt"'

# --- `none` is EXCLUSIVE: combined with any other surface it must fail LOUDLY (exit 2), never ---
# silently pick a winner. The contradiction is a property of the token SET, not of order, so
# cover both orderings. The space must be backslash-escaped (matching docket-config.sh's real
# %q-quoted export, e.g. `inline\ none`) so it survives write_board_fixture's own eval intact as
# ONE value — an unescaped space would eval as `BOARD_SURFACES=inline` env-prefixing a `none`
# command, masking the guard under test with the (already-covered) empty-surfaces guard instead.
write_board_fixture "inline\ none"
(cd "$tmp/board-case/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-board.sh" "$SCRIPT" --board-only >"$tmp/board-run3x.txt" 2>"$tmp/board-run3x-err.txt")
rc=$?
assert "board_pass 'inline none' exits 2 (none is exclusive)" '[ $rc -eq 2 ]'
assert "board_pass 'inline none' names the exclusivity conflict on stderr" \
  'grep -qF "exclusive" "$tmp/board-run3x-err.txt"'
assert "board_pass 'inline none' NEVER reports 'board off'" \
  '! grep -qxF "board off" "$tmp/board-run3x.txt"'
assert "board_pass 'inline none' emits no inline board line" \
  '! grep -q "board inline" "$tmp/board-run3x.txt"'

write_board_fixture "none\ inline"
(cd "$tmp/board-case/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-board.sh" "$SCRIPT" --board-only >"$tmp/board-run3y.txt" 2>"$tmp/board-run3y-err.txt")
rc=$?
assert "board_pass 'none inline' exits 2 (none is exclusive)" '[ $rc -eq 2 ]'
assert "board_pass 'none inline' names the exclusivity conflict on stderr" \
  'grep -qF "exclusive" "$tmp/board-run3y-err.txt"'
assert "board_pass 'none inline' NEVER reports 'board off'" \
  '! grep -qxF "board off" "$tmp/board-run3y.txt"'
assert "board_pass 'none inline' emits no inline board line" \
  '! grep -q "board inline" "$tmp/board-run3y.txt"'

# --- change 0071 review, finding 6 (defence-in-depth): a WHITESPACE-ONLY BOARD_SURFACES must be
# treated identically to a truly-empty one — it passes the `-z` check but tokenizes to zero words.
# The space must be backslash-escaped (matching docket-config.sh's real %q-quoted export) so it
# survives write_board_fixture's own eval intact as ONE value, the same trick "inline\ none" above
# uses — an unescaped space would eval as a bare `BOARD_SURFACES=` env-prefixing a no-op command.
write_board_fixture "\ "
(cd "$tmp/board-case/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-board.sh" "$SCRIPT" --board-only >"$tmp/board-run3w.txt" 2>"$tmp/board-run3w-err.txt")
rc=$?
assert "board_pass whitespace-only surfaces exits 2 (treated identically to empty)" '[ $rc -eq 2 ]'
assert "board_pass whitespace-only surfaces names the unresolved config on stderr" \
  'grep -qF "BOARD_SURFACES" "$tmp/board-run3w-err.txt"'
assert "board_pass whitespace-only surfaces NEVER reports 'board off'" \
  '! grep -qxF "board off" "$tmp/board-run3w.txt"'
assert "board_pass whitespace-only surfaces emits no inline board line" \
  '! grep -q "board inline" "$tmp/board-run3w.txt"'

# --- change 0071 review, finding 1: the report channel must be TOTAL, not just the success shapes.
# Two exit-0 paths through board_pass used to emit NO `board …` line at all (stderr-only warnings):
# an unrecognized surface token, and an inline render failure. Both now carry a positive stdout
# line — the exact hole a must-land caller (keying on the report line, never the exit code) would
# otherwise fall through silently.

# 1a: a typo'd/unknown token ALONE — must still exit 0 (a typo never aborts a build) but the report
# is no longer silent: a positive `board <tok> unknown` line carries the outcome on stdout.
write_board_fixture "inlne"
(cd "$tmp/board-case/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-board.sh" "$SCRIPT" --board-only >"$tmp/board-unknown.txt" 2>"$tmp/board-unknown-err.txt")
rc=$?
assert "board_pass unknown-token-alone run exits zero (a typo never aborts)" '[ $rc -eq 0 ]'
assert "board_pass unknown-token-alone emits a positive 'board inlne unknown' line" \
  'grep -qxF "board inlne unknown" "$tmp/board-unknown.txt"'
assert "board_pass unknown-token-alone still warns on stderr" \
  'grep -qF "unknown board surface" "$tmp/board-unknown-err.txt"'
assert "board_pass unknown-token-alone still closes with pass ok" \
  'grep -qxF "pass ok" "$tmp/board-unknown.txt"'

# 1b: an unknown token ALONGSIDE a working surface — the typo's line must not be swallowed by, nor
# swallow, the working surface's own line. Both must reach the report. The space must be
# backslash-escaped (matching docket-config.sh's real %q-quoted export, same as "inline\ none"
# above) so it survives write_board_fixture's own eval intact as ONE value — an unescaped space
# would eval as `BOARD_SURFACES=inline` env-prefixing an `inlne` command (which fails to execute,
# never reaching the shell's variable table at all), silently testing nothing.
write_board_fixture "inline\ inlne"
(cd "$tmp/board-case/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-board.sh" "$SCRIPT" --board-only >"$tmp/board-unknown2.txt" 2>"$tmp/board-unknown2-err.txt")
rc=$?
assert "board_pass unknown+inline run exits zero" '[ $rc -eq 0 ]'
assert "board_pass unknown+inline still emits the inline surface's own line" \
  'grep -q "board inline" "$tmp/board-unknown2.txt"'
assert "board_pass unknown+inline ALSO emits the unknown-token line (neither swallows the other)" \
  'grep -qxF "board inlne unknown" "$tmp/board-unknown2.txt"'

# 1c: an inline RENDER FAILURE — best-effort (exit 0, prior BOARD.md kept), but the failure is now
# positive evidence on stdout (`board inline failed`), not just a stderr diagnostic a caller keying
# on the report line would never see. Force the failure via board-refresh.sh's own RENDER_BOARD
# mock seam (env-inherited by the child process docket-status.sh execs).
cat > "$tmp/failing-render-board.sh" <<'EOF'
#!/usr/bin/env bash
printf 'PARTIAL RENDER GARBAGE — must never reach BOARD.md\n'
exit 7
EOF
chmod +x "$tmp/failing-render-board.sh"
write_board_fixture inline
(cd "$tmp/board-case/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-board.sh" RENDER_BOARD="$tmp/failing-render-board.sh" "$SCRIPT" --board-only >"$tmp/board-renderfail.txt" 2>"$tmp/board-renderfail-err.txt")
rc=$?
assert "board_pass inline-render-failure run exits zero (best-effort)" '[ $rc -eq 0 ]'
assert "board_pass inline-render-failure emits a positive 'board inline failed' line" \
  'grep -qxF "board inline failed" "$tmp/board-renderfail.txt"'
assert "board_pass inline-render-failure never reports a success shape instead" \
  '! grep -Eq "board inline (clean|changed)" "$tmp/board-renderfail.txt"'
assert "board_pass inline-render-failure still logs the stderr diagnostic" \
  'grep -qF "board render failed" "$tmp/board-renderfail-err.txt"'
assert "board_pass inline-render-failure still closes with pass ok" \
  'grep -qxF "pass ok" "$tmp/board-renderfail.txt"'

# board_pass rebase-conflict-regenerate branch: force a push rejection whose only conflicting
# path is BOARD.md, so the orchestrator must pull --rebase, hit a BOARD.md-only conflict,
# regenerate via render-board.sh, and continue — never leaving BOARD.md empty/truncated.
# A GIT wrapper races a competing push (from a second clone) in right after the orchestrator's
# initial worktree sync but before its own push, so the sync itself sees no conflict and the
# race is deterministic (no real network timing).
git_repo_setup "$tmp/conflict-case"
git clone -q "$tmp/conflict-case/origin.git" "$tmp/conflict-case/work" 2>/dev/null
seed_changes_fixture "$tmp/conflict-case/work"
git -C "$tmp/conflict-case/work" -c user.email=t@t -c user.name=t add docs/changes
git -C "$tmp/conflict-case/work" -c user.email=t@t -c user.name=t commit -q -m "seed changes fixture"
git -C "$tmp/conflict-case/work" push -q origin main

git clone -q "$tmp/conflict-case/origin.git" "$tmp/conflict-case/work2" 2>/dev/null
cat > "$tmp/conflict-case/work2/docs/changes/active/0002-beta.md" <<'EOF'
---
id: 2
slug: beta
title: Beta feature
status: proposed
priority: medium
depends_on: []
spec: docs/superpowers/specs/2026-06-11-beta.md
branch: feat/beta
EOF
"$REPO/scripts/render-board.sh" --changes-dir "$tmp/conflict-case/work2/docs/changes" --repo x/y \
  > "$tmp/conflict-case/work2/docs/changes/BOARD.md"
git -C "$tmp/conflict-case/work2" -c user.email=t@t -c user.name=t add docs/changes
git -C "$tmp/conflict-case/work2" -c user.email=t@t -c user.name=t commit -q -m "add beta + board"
# NOTE: work2's competing commit is pushed by the GIT race wrapper below, after $work's initial
# sync, so the orchestrator's own push (not its startup sync) is the one that gets rejected.

sed -i.bak 's/Alpha feature/Alpha feature v2/' "$tmp/conflict-case/work/docs/changes/active/0001-alpha.md"
rm -f "$tmp/conflict-case/work/docs/changes/active/0001-alpha.md.bak"
git -C "$tmp/conflict-case/work" -c user.email=t@t -c user.name=t add docs/changes
git -C "$tmp/conflict-case/work" -c user.email=t@t -c user.name=t commit -q -m "alpha v2 (local, unpushed)"

cat > "$tmp/git-race.sh" <<EOF
#!/usr/bin/env bash
# Wraps real git; races work2's push in once, right after \$work's startup sync pull, so
# the orchestrator's own board_pass push collides deterministically without real timing.
raced="$tmp/conflict-case/.raced"
# change 0075: main-mode preflight now anchors its sync with \`git -C <root> pull --rebase\`
# (D2 parity — the sync must target the MAIN worktree, not the caller's CWD), so the pull
# subcommand may sit at \$3 behind a leading -C DIR rather than always at \$1.
sub="\$1"
[ "\$sub" = "-C" ] && sub="\$3"
if [ "\$sub" = pull ] && [ ! -f "\$raced" ]; then
  git "\$@"; rc=\$?
  touch "\$raced"
  git -C "$tmp/conflict-case/work2" push -q origin main
  exit \$rc
fi
exec git "\$@"
EOF
chmod +x "$tmp/git-race.sh"

write_board_fixture inline
(cd "$tmp/conflict-case/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-board.sh" \
  GIT="$tmp/git-race.sh" GIT_EDITOR=true \
  "$SCRIPT" --board-only >"$tmp/conflict-run.txt" 2>"$tmp/conflict-run-err.txt")
rc=$?
assert "conflict run exits zero" '[ $rc -eq 0 ]'
assert "conflict run reports inline changed pushed or push-failed" 'grep -Eq "board inline changed (pushed|push-failed)" "$tmp/conflict-run.txt"'
assert "conflict run: BOARD.md non-empty after run" '[ -s "$tmp/conflict-case/work/docs/changes/BOARD.md" ]'
# The outcome of THIS fixture is deterministic, not merely "one of two acceptable shapes": the
# race is built so the only conflict is on BOARD.md, the regen callback always resolves it, and
# the retry loop's push then always succeeds. Change 0067 review, finding 2 — the two strong
# asserts below used to sit inside `if grep -q "board inline changed pushed" ...; then ... fi`,
# which self-disables the moment the branch degrades: break the regen callback so the outcome
# drops to push-failed, and the guard goes false, so the two strong asserts below simply never
# run — the suite stays GREEN with the asserts silently vanished rather than failing (proven: 234
# ok -> 232 ok, 0 NOT OK). Asserting the expected outcome directly and UNCONDITIONALLY (no `if`)
# converts a broken regen callback into a hard NOT OK right here, and the two strong asserts below
# now always execute rather than being gated on the very thing they exist to catch a regression in.
assert "conflict run reports the deterministic pushed outcome (not vacuously satisfied by push-failed)" \
  'grep -qxF "board inline changed pushed" "$tmp/conflict-run.txt"'
assert "conflict run pushed: local BOARD.md carries both merged changes" \
  'grep -q "beta" "$tmp/conflict-case/work/docs/changes/BOARD.md" && grep -q "Alpha feature v2" "$tmp/conflict-case/work/docs/changes/BOARD.md"'
assert "conflict run pushed: remote BOARD.md matches local" \
  'git -C "$tmp/conflict-case/work" show origin/main:docs/changes/BOARD.md 2>/dev/null | cmp -s - "$tmp/conflict-case/work/docs/changes/BOARD.md"'

# board_pass_inline: a clean working tree is NOT sufficient evidence the board landed on the
# remote (change 0071 review, finding 3). Seed a repo whose BOARD.md already matches what a fresh
# render would produce (so the post-render diff is clean) but whose local metadata branch carries
# that exact commit UNPUSHED — origin never saw it, simulating a prior run that committed the
# board locally then failed to push. board_pass_inline must not mistake "nothing to commit" for
# "nothing to push": it must attempt the push and report a changed outcome, never `board inline
# clean`.
git_repo_setup "$tmp/unpushed-case"
git clone -q "$tmp/unpushed-case/origin.git" "$tmp/unpushed-case/work" 2>/dev/null
seed_changes_fixture "$tmp/unpushed-case/work"
git -C "$tmp/unpushed-case/work" -c user.email=t@t -c user.name=t add docs/changes
git -C "$tmp/unpushed-case/work" -c user.email=t@t -c user.name=t commit -q -m "seed changes fixture"
git -C "$tmp/unpushed-case/work" push -q origin main

# Render through the same gated primitive board_pass_inline itself uses, then commit WITHOUT
# pushing — this is the exact local state a failed push leaves behind.
"$REPO/scripts/board-refresh.sh" --changes-dir "$tmp/unpushed-case/work/docs/changes" --surfaces inline
git -C "$tmp/unpushed-case/work" -c user.email=t@t -c user.name=t add docs/changes/BOARD.md
git -C "$tmp/unpushed-case/work" -c user.email=t@t -c user.name=t commit -q -m "docket: board refresh"

assert "unpushed fixture: local branch really is ahead of origin on BOARD.md before the run" \
  '[ "$(git -C "$tmp/unpushed-case/work" rev-list --count origin/main..HEAD -- docs/changes/BOARD.md)" -gt 0 ]'

write_board_fixture inline
(cd "$tmp/unpushed-case/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-board.sh" "$SCRIPT" --board-only >"$tmp/unpushed-run.txt" 2>"$tmp/unpushed-run-err.txt")
rc=$?
assert "unpushed board commit: run exits zero" '[ $rc -eq 0 ]'
assert "unpushed board commit: reports changed pushed or push-failed (not vacuous: positive shape match)" \
  'grep -Eq "board inline changed (pushed|push-failed)" "$tmp/unpushed-run.txt"'
assert "unpushed board commit: never reports clean" \
  '! grep -qxF "board inline clean" "$tmp/unpushed-run.txt"'
if grep -q "board inline changed pushed" "$tmp/unpushed-run.txt"; then
  assert "unpushed board commit pushed: remote now carries the previously-unpushed commit" \
    'git -C "$tmp/unpushed-case/work" show origin/main:docs/changes/BOARD.md 2>/dev/null | cmp -s - "$tmp/unpushed-case/work/docs/changes/BOARD.md"'
fi

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
# Give board_pass something to render+commit+push: mutate a change so BOARD.md changes. Commit
# the mutation itself first (matching the conflict-case fixture's own pattern above) — main-mode
# preflight runs `git pull --rebase` on this same working tree before board_pass ever executes,
# and that fails outright on an uncommitted change, never reaching the retry loop under test.
sed -i.bak 's/Alpha feature/Alpha feature v3/' "$tmp/mustland-case/work/docs/changes/active/0001-alpha.md"
rm -f "$tmp/mustland-case/work/docs/changes/active/0001-alpha.md.bak"
git -C "$tmp/mustland-case/work" -c user.email=t@t -c user.name=t add docs/changes
git -C "$tmp/mustland-case/work" -c user.email=t@t -c user.name=t commit -q -m "alpha v3 (local, unpushed)"
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

# detect_merged: batched sweep detection (task 4). Source the script (guarded so it doesn't
# auto-run main), seed a hermetic changes tree with two `implemented` changes — one whose GH
# mock reports a merged PR, one open — and a GH stub serving canned graphql JSON.
detect_dir="$tmp/detect-case"
mkdir -p "$detect_dir/docs/changes/active"
cat > "$detect_dir/docs/changes/active/0010-merged-thing.md" <<'EOF'
---
id: 10
slug: merged-thing
title: Merged thing
status: implemented
priority: high
depends_on: []
branch: feat/merged-thing
pr: 101
EOF
cat > "$detect_dir/docs/changes/active/0011-open-thing.md" <<'EOF'
---
id: 11
slug: open-thing
title: Open thing
status: implemented
priority: high
depends_on: []
branch: feat/open-thing
pr: 102
EOF

cat > "$tmp/gh-detect-ok.sh" <<'EOF'
#!/usr/bin/env bash
if [ "$1" = repo ] && [ "$2" = view ]; then
  echo "x/y"
  exit 0
fi
if [ "$1" = api ] && [ "$2" = graphql ]; then
  cat <<'JSON'
{"data":{"p10":{"pullRequest":{"number":101,"mergedAt":"2026-07-05T18:22:31Z","state":"MERGED"}},"p11":{"pullRequest":{"number":102,"mergedAt":null,"state":"OPEN"}}}}
JSON
  exit 0
fi
echo "gh-detect-ok: unexpected args: $*" >&2
exit 1
EOF
chmod +x "$tmp/gh-detect-ok.sh"

detect_out="$( cd "$detect_dir" && \
  DOCKET_MODE=main CHANGES_DIR=docs/changes GH="$tmp/gh-detect-ok.sh" \
  bash -c '. "'"$SCRIPT"'"; detect_merged' )"
expected_line="$(printf '10\tmerged-thing\t101\t2026-07-05')"
assert "detect_merged prints exactly the merged change" \
  'printf "%s\n" "$detect_out" | grep -qF "$expected_line"'
assert "detect_merged does not print the open change" \
  '! printf "%s\n" "$detect_out" | grep -q "open-thing"'
assert "detect_merged output has exactly one candidate line" \
  '[ "$(printf "%s\n" "$detect_out" | grep -c "$(printf "\t")")" -eq 1 ]'

cat > "$tmp/gh-detect-fail.sh" <<'EOF'
#!/usr/bin/env bash
echo "gh-detect-fail: boom" >&2
exit 1
EOF
chmod +x "$tmp/gh-detect-fail.sh"

detect_fail_out="$( cd "$detect_dir" && \
  DOCKET_MODE=main CHANGES_DIR=docs/changes GH="$tmp/gh-detect-fail.sh" \
  bash -c '. "'"$SCRIPT"'"; detect_merged' )"
detect_fail_rc=$?
assert "detect_merged with failing GH reports sweep-skipped" \
  'printf "%s\n" "$detect_fail_out" | grep -q "^sweep-skipped"'
assert "detect_merged with failing GH returns success (best-effort)" '[ $detect_fail_rc -eq 0 ]'

# I1 regression: detect_merged's "sweep-skipped <reason>" line must survive the
# `detect_merged | sweep_execute` pipe composition (sweep_execute must not silently
# swallow it as a bogus TSV close-out record), and no git/close-out action must fire.
pipe_out="$( cd "$detect_dir" && \
  DOCKET_MODE=main CHANGES_DIR=docs/changes GH="$tmp/gh-detect-fail.sh" GIT="$tmp/git-should-not-run.sh" \
  SCRIPTS_DIR="$tmp/scripts-should-not-run" \
  bash -c '. "'"$SCRIPT"'"; detect_merged | sweep_execute' )"
assert "detect_merged | sweep_execute: sweep-skipped reaches stdout through the pipe" \
  'printf "%s\n" "$pipe_out" | grep -q "^sweep-skipped"'
assert "detect_merged | sweep_execute: no bogus close-out output for the skip line" \
  '! printf "%s\n" "$pipe_out" | grep -Eq "^(swept|harvest|sweep-failed) "'

# sweep_execute: chained close-out (task 5). Mock the four shared scripts via the SCRIPTS_DIR
# seam so the loop is hermetic — no network, no real docket-config.sh, no real close-out logic.
sweep_dir="$tmp/sweep-case"
git_repo_setup "$sweep_dir"
git clone -q "$sweep_dir/origin.git" "$sweep_dir/work" 2>/dev/null
mkdir -p "$sweep_dir/work/docs/changes/active" "$sweep_dir/work/docs/changes/archive" "$sweep_dir/work/docs/adrs"

seed_sweep_change(){
  # $1 id, $2 slug, $3 status
  cat > "$sweep_dir/work/docs/changes/active/$(printf '%04d' "$1")-$2.md" <<EOF
---
id: $1
slug: $2
title: $2 change
status: $3
priority: high
depends_on: []
branch: feat/$2
pr: $1
---

Body.
EOF
}
seed_sweep_change 20 clean-thing implemented
seed_sweep_change 21 broken-render implemented
seed_sweep_change 23 cleanup-broken implemented
seed_sweep_change 24 publish-broken implemented
seed_sweep_change 25 publish-mark-broken implemented
# The sweep commits on this clone itself (the artifacts refresh, and change 0083's deferral mark),
# without passing `-c user.*`. Configure an identity locally so the fixture does not silently
# depend on the developer's global git config.
git -C "$sweep_dir/work" config user.email t@t
git -C "$sweep_dir/work" config user.name t
git -C "$sweep_dir/work" -c user.email=t@t -c user.name=t add docs/changes
git -C "$sweep_dir/work" -c user.email=t@t -c user.name=t commit -q -m "seed sweep changes"
git -C "$sweep_dir/work" push -q origin main

mkdir -p "$tmp/mock-scripts"
sweep_log="$tmp/sweep-calls.log"
: > "$sweep_log"

cat > "$tmp/mock-scripts/archive-change.sh" <<'EOF'
#!/usr/bin/env bash
echo "archive-change $*" >> "$SWEEP_LOG"
changes_dir="" id="" date=""
while [ $# -gt 0 ]; do
  case "$1" in
    --changes-dir) changes_dir="$2"; shift ;;
    --id) id="$2"; shift ;;
    --date) date="$2"; shift ;;
  esac
  shift
done
pad="$(printf '%04d' "$id")"
active="$(find "$changes_dir/active" -maxdepth 1 -name "${pad}-*.md" | head -n1)"
[ -n "$active" ] || exit 1
base="$(basename "$active")"
slug="${base#"${pad}"-}"; slug="${slug%.md}"
mkdir -p "$changes_dir/archive"
dest="$changes_dir/archive/${date}-${pad}-${slug}.md"
root="$(git -C "$changes_dir" rev-parse --show-toplevel)"
git -C "$root" mv "$active" "$dest"
sed -i.bak "s/^status:.*/status: done/" "$dest" && rm -f "$dest.bak"
git -C "$root" add -- "$dest" 2>/dev/null
git -C "$root" -c user.email=t@t -c user.name=t commit -q -m "mock archive" >/dev/null 2>&1
git -C "$root" push -q origin main >/dev/null 2>&1
exit 0
EOF

cat > "$tmp/mock-scripts/render-change-links.sh" <<'EOF'
#!/usr/bin/env bash
echo "render-change-links $*" >> "$SWEEP_LOG"
case "$*" in *broken-render*) exit 1 ;; esac
exit 0
EOF

cat > "$tmp/mock-scripts/terminal-publish.sh" <<'EOF'
#!/usr/bin/env bash
echo "terminal-publish $*" >> "$SWEEP_LOG"
# change 0083: ids 24 and 25 drive the publish-FAILURE branch — the one the sweep must now mark
# on the archived change file before emitting its `sweep-failed … terminal-publish` line.
case "$*" in *"--id 24 "*|*"--id 25 "*) exit 1 ;; esac
exit 0
EOF

# change 0083: the sweep's own deferral mark. The mock ACTUALLY appends a marker heading, so the
# commit+push leg after it is exercised for real rather than short-circuiting on an empty diff.
cat > "$tmp/mock-scripts/mark-publish-deferred.sh" <<'EOF'
#!/usr/bin/env bash
echo "mark-publish-deferred $*" >> "$SWEEP_LOG"
args="$*"
cf=""
while [ $# -gt 0 ]; do
  case "$1" in --change-file) cf="$2"; shift ;; esac
  shift
done
# id 25 models a mark that FAILS and writes nothing: the sweep must be wholly indifferent to it.
case "$args" in *"--id 25"*) exit 1 ;; esac
printf '\n## Publish deferred\n\nmocked marker\n' >> "$cf"
exit 0
EOF

cat > "$tmp/mock-scripts/cleanup-feature-branch.sh" <<'EOF'
#!/usr/bin/env bash
echo "cleanup-feature-branch $*" >> "$SWEEP_LOG"
case "$*" in *cleanup-broken*) exit 1 ;; esac
exit 0
EOF
chmod +x "$tmp/mock-scripts/"*.sh

sweep_input="$tmp/sweep-input.tsv"
# 24 and 25 (the change-0083 publish-failure cases) sit BEFORE 23, so `23` is processed after them
# and a loop-continuation assert keyed on it is real rather than order-trivial.
printf '20\tclean-thing\t20\t2026-07-08\n21\tbroken-render\t21\t2026-07-09\n22\talready-done\t22\t2026-07-05\n24\tpublish-broken\t24\t2026-07-11\n25\tpublish-mark-broken\t25\t2026-07-12\n23\tcleanup-broken\t23\t2026-07-10\n' > "$sweep_input"

# NOTE: docket-status.sh's own top-level flag parser consumes "$@" at source time, so no
# positional args can be passed through `bash -c '. script; ...' _ <args>` here — feed the
# canned merged-change list via a file instead.
sweep_out="$( cd "$sweep_dir/work" && \
  DOCKET_MODE=main CHANGES_DIR=docs/changes ADRS_DIR=docs/adrs \
  INTEGRATION_BRANCH=main METADATA_BRANCH=main \
  SCRIPTS_DIR="$tmp/mock-scripts" SWEEP_LOG="$sweep_log" SWEEP_INPUT="$sweep_input" \
  bash -c '. "'"$SCRIPT"'"; sweep_execute < "$SWEEP_INPUT"' )"

assert "sweep_execute: clean change emits swept" \
  'printf "%s\n" "$sweep_out" | grep -qE "^swept 20 2026-07-08$"'
assert "sweep_execute: clean change emits harvest with archived path" \
  'printf "%s\n" "$sweep_out" | grep -qE "^harvest 20 .*2026-07-08-0020-clean-thing\.md$"'
assert "sweep_execute: clean change calls all four stubs" \
  'grep -q -- "--id 20 " "$sweep_log" && grep -q "clean-thing" "$sweep_log" \
   && grep -q "^terminal-publish" "$sweep_log" && grep -q "^cleanup-feature-branch" "$sweep_log"'
# change 0083: the sweep must hand terminal-publish.sh an EXPLICIT --metadata-worktree. Without it
# the publish resolves the metadata tree from the main-worktree anchor, which is right only by
# coincidence — and the flag was silently dropped once already. Asserted on the recorded invocation
# (the mock logs "$*"), so the wiring cannot regress unnoticed. Mirrors test_closeout.sh's
# every-call-site-supplies---enabled check, one layer down: this one is behavioral.
assert "sweep_execute: terminal-publish invocation supplies --metadata-worktree" \
  'tp_call="$(grep -m1 "^terminal-publish .*--id 20 " "$sweep_log")"; \
   [ -n "$tp_call" ] && grep -q -- "--metadata-worktree /" <<<"$tp_call"'
assert "sweep_execute: broken-render change emits sweep-failed render-change-links" \
  'printf "%s\n" "$sweep_out" | grep -qE "^sweep-failed 21 render-change-links "'
assert "sweep_execute: broken-render change does NOT call terminal-publish" \
  '! grep -q "terminal-publish.*--id 21 " "$sweep_log"'
assert "sweep_execute: broken-render change does not emit swept" \
  '! printf "%s\n" "$sweep_out" | grep -qE "^swept 21 "'
assert "sweep_execute: already-done (missing active file) is a silent no-op" \
  '! printf "%s\n" "$sweep_out" | grep -qE " 22 "'
assert "sweep_execute: archive-change called before render-change-links (order)" \
  'archive_line=$(grep -n "^archive-change" "$sweep_log" | grep " --id 20 " | head -n1 | cut -d: -f1); \
   render_line=$(grep -n "^render-change-links" "$sweep_log" | grep "clean-thing" | head -n1 | cut -d: -f1); \
   [ -n "$archive_line" ] && [ -n "$render_line" ] && [ "$archive_line" -lt "$render_line" ]'
assert "sweep_execute: render-change-links called before terminal-publish (order, change 20)" \
  'render_line=$(grep -n "^render-change-links" "$sweep_log" | grep "clean-thing" | head -n1 | cut -d: -f1); \
   publish_line=$(grep -n "^terminal-publish" "$sweep_log" | grep -- "--id 20 " | head -n1 | cut -d: -f1); \
   [ -n "$render_line" ] && [ -n "$publish_line" ] && [ "$render_line" -lt "$publish_line" ]'
assert "sweep_execute: terminal-publish called before cleanup-feature-branch (order, change 20)" \
  'publish_line=$(grep -n "^terminal-publish" "$sweep_log" | grep -- "--id 20 " | head -n1 | cut -d: -f1); \
   cleanup_line=$(grep -n "^cleanup-feature-branch" "$sweep_log" | grep -- "--slug clean-thing" | head -n1 | cut -d: -f1); \
   [ -n "$publish_line" ] && [ -n "$cleanup_line" ] && [ "$publish_line" -lt "$cleanup_line" ]'
assert "sweep_execute: cleanup failure emits sweep-failed cleanup" \
  'printf "%s\n" "$sweep_out" | grep -qE "^sweep-failed 23 cleanup "'
assert "sweep_execute: cleanup failure still emits swept (terminal transition already durable)" \
  'printf "%s\n" "$sweep_out" | grep -qE "^swept 23 2026-07-10$"'
assert "sweep_execute: cleanup failure still emits harvest" \
  'printf "%s\n" "$sweep_out" | grep -qE "^harvest 23 .*2026-07-10-0023-cleanup-broken\.md$"'
assert "sweep_execute: cleanup failure emits sweep-failed before swept/harvest (order)" \
  'failed_line=$(printf "%s\n" "$sweep_out" | grep -n "^sweep-failed 23 cleanup " | head -n1 | cut -d: -f1); \
   swept_line=$(printf "%s\n" "$sweep_out" | grep -n "^swept 23 " | head -n1 | cut -d: -f1); \
   harvest_line=$(printf "%s\n" "$sweep_out" | grep -n "^harvest 23 " | head -n1 | cut -d: -f1); \
   [ -n "$failed_line" ] && [ -n "$swept_line" ] && [ -n "$harvest_line" ] \
   && [ "$failed_line" -lt "$swept_line" ] && [ "$swept_line" -lt "$harvest_line" ]'
assert "sweep_execute: cleanup failure does not block clean-thing (loop continues)" \
  'printf "%s\n" "$sweep_out" | grep -qE "^swept 20 2026-07-08$"'

# --- change 0083: a failed terminal-publish MARKS ITSELF on the archived change file -------------
# This is the highest-volume automated path on which a publish does not complete. Before change
# 0083 it emitted one report line and nothing else: the change was archived-but-unpublished, no
# later sweep resumed it (the sweep only scans active/), and board-checks' `publish-deferred` check
# read a marker that nothing had written. Now the sweep marks the archived file itself, and commits
# it — an UNCOMMITTED marker would dirty the shared metadata worktree and fail the next pass's
# `pull --rebase` for every change, which is strictly worse than the gap it records.
assert "0083: a failed terminal-publish still emits sweep-failed terminal-publish script-error" \
  'grep -qE "^sweep-failed 24 terminal-publish script-error$" <<<"$sweep_out"'
assert "0083: the failed publish invokes mark-publish-deferred on the ARCHIVED change file" \
  'mk="$(grep -m1 "^mark-publish-deferred .*publish-broken" "$sweep_log")"; \
   [ -n "$mk" ] && grep -q -- "--change-file .*archive/2026-07-11-0024-publish-broken.md" <<<"$mk"'
assert "0083: the mark is --mode add --reason blocked" \
  'mk="$(grep -m1 "^mark-publish-deferred .*publish-broken" "$sweep_log")"; \
   [ -n "$mk" ] && grep -q -- "--mode add" <<<"$mk" && grep -q -- "--reason blocked" <<<"$mk"'
assert "0083: the mark carries the change id and the integration branch" \
  'mk="$(grep -m1 "^mark-publish-deferred .*publish-broken" "$sweep_log")"; \
   [ -n "$mk" ] && grep -q -- "--id 24" <<<"$mk" && grep -q -- "--integration-branch main" <<<"$mk"'
assert "0083: the mark runs AFTER terminal-publish failed, not before it ran" \
  'pub_line=$(grep -n "^terminal-publish .*--id 24 " "$sweep_log" | head -n1 | cut -d: -f1); \
   mark_line=$(grep -n "^mark-publish-deferred .*publish-broken" "$sweep_log" | head -n1 | cut -d: -f1); \
   [ -n "$pub_line" ] && [ -n "$mark_line" ] && [ "$pub_line" -lt "$mark_line" ]'
assert "0083: a SUCCESSFUL publish is never marked" \
  '! grep -q "^mark-publish-deferred .*clean-thing" "$sweep_log"'
assert "0083: a change that never reached the publish step is never marked" \
  '! grep -q "^mark-publish-deferred .*broken-render" "$sweep_log"'
assert "0083: the publish failure still abandons the rest of the close-out (no cleanup)" \
  '! grep -q "^cleanup-feature-branch .*--slug publish-broken" "$sweep_log"'
assert "0083: the publish failure emits neither swept nor harvest" \
  '! grep -qE "^(swept|harvest) 24 " <<<"$sweep_out"'
# The mark must be COMMITTED, not left in the working tree.
assert "0083: the marker reached the metadata branch as a commit" \
  'archived_body="$(git -C "$sweep_dir/work" show "HEAD:docs/changes/archive/2026-07-11-0024-publish-broken.md")"; \
   grep -qxF -- "## Publish deferred" <<<"$archived_body"'
assert "0083: the sweep left the shared metadata worktree CLEAN" \
  '[ -z "$(git -C "$sweep_dir/work" status --porcelain)" ]'

# A mark that FAILS must be invisible: identical report lines, identical control flow. `-c` on the
# whole output, not a presence grep — an extra or duplicated line would pass a presence check.
assert "0083: a FAILED mark still emits exactly the one sweep-failed line for that change" \
  '[ "$(printf "%s\n" "$sweep_out" | grep -c "^sweep-failed 25 ")" -eq 1 ]'
assert "0083: a FAILED mark leaves the reason unchanged (terminal-publish script-error)" \
  'grep -qE "^sweep-failed 25 terminal-publish script-error$" <<<"$sweep_out"'
assert "0083: a FAILED mark does not add swept/harvest" \
  '! grep -qE "^(swept|harvest) 25 " <<<"$sweep_out"'
assert "0083: a FAILED mark does not resume the close-out (still no cleanup)" \
  '! grep -q "^cleanup-feature-branch .*--slug publish-mark-broken" "$sweep_log"'
# Keyed on 23, which the input orders AFTER 24/25 — so this is genuine loop continuation past both
# publish failures, not a change that had already been processed before them.
assert "0083: a FAILED mark does not stop the loop (the change processed AFTER it still sweeps)" \
  'grep -qE "^swept 23 2026-07-10$" <<<"$sweep_out"'

# --- change 0064 (Finding 1): TERMINAL_PUBLISH gates the REAL sweep's terminal-publish.sh call ---
# A behavioral test (not just wiring): drives docket-status.sh's actual merge-sweep pipeline in a
# hermetic docket-mode fixture (separate docket/main branches on a bare origin) with the REAL
# terminal-publish.sh and REAL cleanup-feature-branch.sh in play (archive-change.sh and
# render-change-links.sh are mocked, matching this file's existing sweep_execute convention, since
# their own behavior is already covered above — this section is about the --enabled wiring and the
# knob's suppress-but-don't-abort contract). GH/graphql is mocked (no network).
gate_setup(){
  # $1 = root dir. Seeds a bare origin with docket+main, a real `implemented` change on docket
  # (id 60, slug gate-thing, pr 60), and a real feat/gate-thing branch+worktree on the primary
  # checkout so cleanup-feature-branch.sh has genuine work to do.
  local root="$1"
  git_repo_setup "$root"
  git clone -q "$root/origin.git" "$root/seed-docket" 2>/dev/null
  git -C "$root/seed-docket" checkout docket >/dev/null 2>&1
  mkdir -p "$root/seed-docket/docs/changes/active" "$root/seed-docket/docs/changes/archive" "$root/seed-docket/docs/adrs"
  cat > "$root/seed-docket/docs/changes/active/0060-gate-thing.md" <<'EOF'
---
id: 60
slug: gate-thing
title: Gate thing
status: implemented
priority: high
depends_on: []
branch: feat/gate-thing
pr: 60
---

Body.
EOF
  git -C "$root/seed-docket" add docs
  git -C "$root/seed-docket" -c user.email=t@t -c user.name=t commit -q -m "seed gate change"
  git -C "$root/seed-docket" push -q origin docket
  git clone -q "$root/origin.git" "$root/work" 2>/dev/null
  git -C "$root/work" worktree add "$root/work/.worktrees/gate-thing" -b feat/gate-thing main >/dev/null 2>&1
  git -C "$root/work" push -q origin feat/gate-thing
}

mkdir -p "$tmp/mock-gate"
# NOTE: unlike the sweep_execute mock above (mw="." — changes-dir IS the worktree root, so
# cwd-relative paths and worktree-relative paths coincide), this fixture runs in DOCKET_MODE=docket
# (mw=".docket", a linked worktree). `git -C "$root" mv <cwd-relative-path>` would resolve that
# path against $root, not the invoking cwd, so the paths must be converted to be relative to the
# worktree root first (mirrors archive-change.sh's own REL_ABS/REL computation).
cat > "$tmp/mock-gate/archive-change.sh" <<'EOF'
#!/usr/bin/env bash
changes_dir="" id="" date=""
while [ $# -gt 0 ]; do
  case "$1" in
    --changes-dir) changes_dir="$2"; shift ;;
    --id) id="$2"; shift ;;
    --date) date="$2"; shift ;;
  esac
  shift
done
pad="$(printf '%04d' "$id")"
active="$(find "$changes_dir/active" -maxdepth 1 -name "${pad}-*.md" | head -n1)"
[ -n "$active" ] || exit 1
base="$(basename "$active")"
slug="${base#"${pad}"-}"; slug="${slug%.md}"
mkdir -p "$changes_dir/archive"
root="$(git -C "$changes_dir" rev-parse --show-toplevel)"
rel_abs="$(cd "$changes_dir" && pwd -P)"
rel="${rel_abs#"$root"/}"
active_rel="$rel/active/$base"
dest_rel="$rel/archive/${date}-${pad}-${slug}.md"
git -C "$root" mv "$active_rel" "$dest_rel"
dest="$changes_dir/archive/${date}-${pad}-${slug}.md"
sed -i.bak "s/^status:.*/status: done/" "$dest" && rm -f "$dest.bak"
git -C "$root" add -- "$dest_rel" 2>/dev/null
git -C "$root" -c user.email=t@t -c user.name=t commit -q -m "mock archive" >/dev/null 2>&1
branch="$(git -C "$root" rev-parse --abbrev-ref HEAD)"
git -C "$root" push -q origin "$branch" >/dev/null 2>&1
exit 0
EOF
cat > "$tmp/mock-gate/render-change-links.sh" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
# terminal-publish.sh and cleanup-feature-branch.sh are the REAL scripts (exec'd by absolute
# path so their own $(dirname "$0") resolution — e.g. terminal-publish.sh sourcing
# lib/docket-frontmatter.sh — still finds their real co-located files).
cat > "$tmp/mock-gate/terminal-publish.sh" <<EOF
#!/usr/bin/env bash
exec "$REPO/scripts/terminal-publish.sh" "\$@"
EOF
cat > "$tmp/mock-gate/cleanup-feature-branch.sh" <<EOF
#!/usr/bin/env bash
exec "$REPO/scripts/cleanup-feature-branch.sh" "\$@"
EOF
cat > "$tmp/mock-gate/board-checks.sh" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
cat > "$tmp/mock-gate/sync-integration-branch.sh" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
chmod +x "$tmp/mock-gate/"*.sh

cat > "$tmp/gh-gate.sh" <<'EOF'
#!/usr/bin/env bash
if [ "$1" = api ] && [ "$2" = graphql ]; then
  cat <<'JSON'
{"data":{"p60":{"pullRequest":{"number":60,"mergedAt":"2026-07-11T12:00:00Z","state":"MERGED"}}}}
JSON
  exit 0
fi
echo "gh-gate: unexpected args: $*" >&2
exit 1
EOF
chmod +x "$tmp/gh-gate.sh"

# Case A: terminal_publish: false — the archived record must NOT reach the integration branch,
# but the rest of the close-out (archive on docket, cleanup) still completes: a suppressed publish
# is success, not a reason to abort the sweep.
cat > "$tmp/fixture-gate-disabled.sh" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' \
  'BOOTSTRAP=PROCEED' \
  'METADATA_BRANCH=docket' \
  'INTEGRATION_BRANCH=main' \
  'DOCKET_MODE=docket' \
  'METADATA_WORKTREE=.docket' \
  'CHANGES_DIR=docs/changes' \
  'ADRS_DIR=docs/adrs' \
  'RESULTS_DIR=docs/results' \
  'BOARD_SURFACES=none' \
  'TERMINAL_PUBLISH=false'
EOF

gate_dir="$tmp/gate-disabled-case"
gate_setup "$gate_dir"
(cd "$gate_dir/work" && \
  CONFIG_EXPORT_CMD="bash $tmp/fixture-gate-disabled.sh" GH="$tmp/gh-gate.sh" \
  SCRIPTS_DIR="$tmp/mock-gate" \
  "$SCRIPT" --repo x/y >"$tmp/gate-disabled-out.txt" 2>"$tmp/gate-disabled-err.txt")
rc=$?
assert "0064 gate(disabled): sweep exits zero" '[ $rc -eq 0 ]'
assert "0064 gate(disabled): sweep emits swept (archive still ran)" \
  'grep -qE "^swept 60 2026-07-11$" "$tmp/gate-disabled-out.txt"'
assert "0064 gate(disabled): no sweep-failed lines (suppressed publish is not a failure)" \
  '! grep -q "sweep-failed 60" "$tmp/gate-disabled-out.txt"'
git -C "$gate_dir/work" fetch origin main >/dev/null 2>&1
assert "0064 gate(disabled): archived record NOT published to the integration branch" \
  '! git -C "$gate_dir/work" ls-tree -r --name-only origin/main | grep -q "docs/changes/archive/2026-07-11-0060-gate-thing.md"'
git -C "$gate_dir/work" fetch origin docket >/dev/null 2>&1
assert "0064 gate(disabled): the archive itself still landed on the metadata branch" \
  'git -C "$gate_dir/work" ls-tree -r --name-only origin/docket | grep -q "docs/changes/archive/2026-07-11-0060-gate-thing.md"'
assert "0064 gate(disabled): terminal-publish logged the suppression" \
  'grep -q "terminal_publish: false" "$tmp/gate-disabled-err.txt"'
assert "0064 gate(disabled): the sweep still cleaned up the feature worktree" \
  '[ ! -e "$gate_dir/work/.worktrees/gate-thing" ]'
assert "0064 gate(disabled): the sweep still deleted the remote feature branch" \
  '! git -C "$gate_dir/work" ls-remote --exit-code origin feat/gate-thing >/dev/null 2>&1'

# Case B: TERMINAL_PUBLISH entirely UNSET by the config mock (not merely "false") — reproduces the
# exact hazard the fix guards against: sweep_execute_one runs under `set -u`, so a bare
# $TERMINAL_PUBLISH would abort the sweep with an unbound-variable error under a stale/mocked
# config export that doesn't emit the key. "${TERMINAL_PUBLISH:-false}" must keep guarding that
# crash (the `:-` expansion is the guard) while defaulting to DISABLED — change 0084: a repo that
# never set the key must never get a direct machine commit on its integration branch.
cat > "$tmp/fixture-gate-unset.sh" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' \
  'BOOTSTRAP=PROCEED' \
  'METADATA_BRANCH=docket' \
  'INTEGRATION_BRANCH=main' \
  'DOCKET_MODE=docket' \
  'METADATA_WORKTREE=.docket' \
  'CHANGES_DIR=docs/changes' \
  'ADRS_DIR=docs/adrs' \
  'RESULTS_DIR=docs/results' \
  'BOARD_SURFACES=none'
EOF

gate_dir2="$tmp/gate-enabled-case"
gate_setup "$gate_dir2"
(cd "$gate_dir2/work" && \
  CONFIG_EXPORT_CMD="bash $tmp/fixture-gate-unset.sh" GH="$tmp/gh-gate.sh" \
  SCRIPTS_DIR="$tmp/mock-gate" \
  "$SCRIPT" --repo x/y >"$tmp/gate-enabled-out.txt" 2>"$tmp/gate-enabled-err.txt")
rc=$?
assert "0064 gate(TERMINAL_PUBLISH unset): sweep exits zero (no unbound-variable crash)" '[ $rc -eq 0 ]'
assert "0064 gate(TERMINAL_PUBLISH unset): sweep emits swept" \
  'grep -qE "^swept 60 2026-07-11$" "$tmp/gate-enabled-out.txt"'
git -C "$gate_dir2/work" fetch origin main >/dev/null 2>&1
assert "0084 gate(TERMINAL_PUBLISH unset): defaults to DISABLED — archived record does NOT reach the integration branch" \
  '! git -C "$gate_dir2/work" ls-tree -r --name-only origin/main | grep -q "docs/changes/archive/2026-07-11-0060-gate-thing.md"'

# health_checks: prefixes board-checks.sh's TSV findings as "check <id> <change-id> <message>".
# Mock board-checks.sh via SCRIPTS_DIR — this is a pure formatting/plumbing test, not a
# re-test of board-checks.sh's own check logic.
health_dir="$tmp/health-case"
mkdir -p "$health_dir/docs/changes/active" "$tmp/mock-health"
cat > "$tmp/mock-health/board-checks.sh" <<'EOF'
#!/usr/bin/env bash
echo "board-checks $*" >> "$HEALTH_LOG"
printf 'broken-spec\t12\tspec path missing on docket\n'
EOF
chmod +x "$tmp/mock-health/board-checks.sh"
health_log="$tmp/health-calls.log"; : > "$health_log"

health_out="$( cd "$health_dir" && \
  DOCKET_MODE=main CHANGES_DIR=docs/changes INTEGRATION_BRANCH=main METADATA_BRANCH=main \
  SCRIPTS_DIR="$tmp/mock-health" HEALTH_LOG="$health_log" \
  bash -c '. "'"$SCRIPT"'"; health_checks' )"
assert "health_checks: prefixes board-checks finding as 'check <id> <change-id> <message>'" \
  'printf "%s\n" "$health_out" | grep -qF "check broken-spec 12 spec path missing on docket"'
assert "health_checks: invokes board-checks.sh with expected flags" \
  'grep -Eq -- "--changes-dir \./?docs/changes" "$health_log" && grep -q -- "--metadata-branch main" "$health_log" \
   && grep -q -- "--integration-branch origin/main" "$health_log"'

# health_checks: clean tree (no findings) prints nothing.
mkdir -p "$tmp/mock-health-clean"
cat > "$tmp/mock-health-clean/board-checks.sh" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
chmod +x "$tmp/mock-health-clean/board-checks.sh"
health_clean_out="$( cd "$health_dir" && \
  DOCKET_MODE=main CHANGES_DIR=docs/changes INTEGRATION_BRANCH=main METADATA_BRANCH=main \
  SCRIPTS_DIR="$tmp/mock-health-clean" \
  bash -c '. "'"$SCRIPT"'"; health_checks' )"
assert "health_checks: clean board-checks output emits nothing" '[ -z "$health_clean_out" ]'

# emit_judgment: one "judgment blocked <id> <blocked_by text>" per blocked active change.
judg_dir="$tmp/judgment-case"
mkdir -p "$judg_dir/docs/changes/active"
cat > "$judg_dir/docs/changes/active/0012-waiting-thing.md" <<'EOF'
---
id: 12
slug: waiting-thing
title: Waiting thing
status: blocked
priority: high
depends_on: []
blocked_by: needs decision from platform team on auth flow
EOF
cat > "$judg_dir/docs/changes/active/0013-not-blocked.md" <<'EOF'
---
id: 13
slug: not-blocked
title: Not blocked
status: proposed
priority: high
depends_on: []
EOF

judg_out="$( cd "$judg_dir" && \
  DOCKET_MODE=main CHANGES_DIR=docs/changes \
  bash -c '. "'"$SCRIPT"'"; emit_judgment' )"
assert "emit_judgment: blocked change emits judgment line with id and blocked_by text" \
  'printf "%s\n" "$judg_out" | grep -qF "judgment blocked 12 needs decision from platform team on auth flow"'
assert "emit_judgment: non-blocked change emits nothing" \
  '! printf "%s\n" "$judg_out" | grep -q " 13 "'

# Full-run wiring: main() runs health_checks/emit_judgment always, and gates integration_sync
# on swept_count > 0. Mock every shared script via SCRIPTS_DIR so the run is hermetic; use
# BOARD_SURFACES=none to skip the board pass entirely (already covered above).
write_full_fixture(){
  # $1 board_surfaces (usually empty)
  # METADATA_WORKTREE=. — main-mode's REAL export; see write_board_fixture above.
  cat > "$tmp/fixture-full.sh" <<EOF
#!/usr/bin/env bash
printf '%s\n' \
  'BOOTSTRAP=PROCEED' \
  'METADATA_BRANCH=main' \
  'INTEGRATION_BRANCH=main' \
  'DOCKET_MODE=main' \
  'METADATA_WORKTREE=.' \
  'CHANGES_DIR=docs/changes' \
  'ADRS_DIR=docs/adrs' \
  'RESULTS_DIR=docs/results' \
  'BOARD_SURFACES=$1'
EOF
}
write_full_fixture none

mkdir -p "$tmp/mock-full"
full_log="$tmp/full-calls.log"
cat > "$tmp/mock-full/board-checks.sh" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
cat > "$tmp/mock-full/archive-change.sh" <<'EOF'
#!/usr/bin/env bash
changes_dir="" id="" date=""
while [ $# -gt 0 ]; do
  case "$1" in
    --changes-dir) changes_dir="$2"; shift ;;
    --id) id="$2"; shift ;;
    --date) date="$2"; shift ;;
  esac
  shift
done
pad="$(printf '%04d' "$id")"
active="$(find "$changes_dir/active" -maxdepth 1 -name "${pad}-*.md" | head -n1)"
[ -n "$active" ] || exit 1
base="$(basename "$active")"
slug="${base#"${pad}"-}"; slug="${slug%.md}"
mkdir -p "$changes_dir/archive"
dest="$changes_dir/archive/${date}-${pad}-${slug}.md"
root="$(git -C "$changes_dir" rev-parse --show-toplevel)"
git -C "$root" mv "$active" "$dest"
sed -i.bak "s/^status:.*/status: done/" "$dest" && rm -f "$dest.bak"
git -C "$root" add -- "$dest" 2>/dev/null
git -C "$root" -c user.email=t@t -c user.name=t commit -q -m "mock archive" >/dev/null 2>&1
git -C "$root" push -q origin main >/dev/null 2>&1
exit 0
EOF
cat > "$tmp/mock-full/render-change-links.sh" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
cat > "$tmp/mock-full/terminal-publish.sh" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
cat > "$tmp/mock-full/cleanup-feature-branch.sh" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
cat > "$tmp/mock-full/sync-integration-branch.sh" <<'EOF'
#!/usr/bin/env bash
echo "sync-integration-branch $*" >> "$FULL_LOG"
touch "$SYNC_MARKER"
exit 0
EOF
chmod +x "$tmp/mock-full/"*.sh

cat > "$tmp/gh-full-merged.sh" <<'EOF'
#!/usr/bin/env bash
if [ "$1" = api ] && [ "$2" = graphql ]; then
  cat <<'JSON'
{"data":{"p30":{"pullRequest":{"number":30,"mergedAt":"2026-07-08T12:00:00Z","state":"MERGED"}}}}
JSON
  exit 0
fi
echo "gh-full-merged: unexpected args: $*" >&2
exit 1
EOF
chmod +x "$tmp/gh-full-merged.sh"

cat > "$tmp/gh-full-none.sh" <<'EOF'
#!/usr/bin/env bash
if [ "$1" = repo ] && [ "$2" = view ]; then
  echo "x/y"; exit 0
fi
echo "gh-full-none: unexpected args: $*" >&2
exit 1
EOF
chmod +x "$tmp/gh-full-none.sh"

# Case 1: one merged change present ⇒ sweep occurs ⇒ integration_sync IS invoked.
git_repo_setup "$tmp/full-merged-case"
git clone -q "$tmp/full-merged-case/origin.git" "$tmp/full-merged-case/work" 2>/dev/null
mkdir -p "$tmp/full-merged-case/work/docs/changes/active" "$tmp/full-merged-case/work/docs/adrs"
cat > "$tmp/full-merged-case/work/docs/changes/active/0030-merged-full.md" <<'EOF'
---
id: 30
slug: merged-full
title: Merged full
status: implemented
priority: high
depends_on: []
branch: feat/merged-full
pr: 30
EOF
git -C "$tmp/full-merged-case/work" -c user.email=t@t -c user.name=t add docs/changes
git -C "$tmp/full-merged-case/work" -c user.email=t@t -c user.name=t commit -q -m "seed"
git -C "$tmp/full-merged-case/work" push -q origin main

sync_marker_yes="$tmp/sync-marker-yes"
rm -f "$sync_marker_yes"
(cd "$tmp/full-merged-case/work" && \
  CONFIG_EXPORT_CMD="bash $tmp/fixture-full.sh" GH="$tmp/gh-full-merged.sh" \
  SCRIPTS_DIR="$tmp/mock-full" FULL_LOG="$full_log" SYNC_MARKER="$sync_marker_yes" \
  "$SCRIPT" --repo x/y >"$tmp/full-merged-out.txt" 2>"$tmp/full-merged-err.txt")
rc=$?
assert "full run (merged case) exits zero" '[ $rc -eq 0 ]'
assert "full run (merged case) emits swept line" \
  'grep -qE "^swept 30 2026-07-08$" "$tmp/full-merged-out.txt"'
assert "full run (merged case) invokes integration_sync (marker touched)" \
  '[ -f "$sync_marker_yes" ]'

# Case 2: no merged changes ⇒ no sweep ⇒ integration_sync is NOT invoked.
git_repo_setup "$tmp/full-none-case"
git clone -q "$tmp/full-none-case/origin.git" "$tmp/full-none-case/work" 2>/dev/null
mkdir -p "$tmp/full-none-case/work/docs/changes/active" "$tmp/full-none-case/work/docs/adrs"
git -C "$tmp/full-none-case/work" -c user.email=t@t -c user.name=t commit -q --allow-empty -m "seed2" 2>/dev/null || true
git -C "$tmp/full-none-case/work" push -q origin main 2>/dev/null || true

sync_marker_no="$tmp/sync-marker-no"
rm -f "$sync_marker_no"
(cd "$tmp/full-none-case/work" && \
  CONFIG_EXPORT_CMD="bash $tmp/fixture-full.sh" GH="$tmp/gh-full-none.sh" \
  SCRIPTS_DIR="$tmp/mock-full" FULL_LOG="$full_log" SYNC_MARKER="$sync_marker_no" \
  "$SCRIPT" >"$tmp/full-none-out.txt" 2>"$tmp/full-none-err.txt")
rc=$?
assert "full run (no merges) exits zero" '[ $rc -eq 0 ]'
assert "full run (no merges) does not invoke integration_sync (no marker)" \
  '[ ! -f "$sync_marker_no" ]'

# --board-only: exits after board_pass with no check/swept/judgment lines and no sync call.
sync_marker_bo="$tmp/sync-marker-boardonly"
rm -f "$sync_marker_bo"
(cd "$tmp/full-none-case/work" && \
  CONFIG_EXPORT_CMD="bash $tmp/fixture-full.sh" GH="$tmp/gh-full-none.sh" \
  SCRIPTS_DIR="$tmp/mock-full" SYNC_MARKER="$sync_marker_bo" \
  "$SCRIPT" --board-only >"$tmp/full-boardonly-out.txt" 2>"$tmp/full-boardonly-err.txt")
rc=$?
assert "--board-only exits zero" '[ $rc -eq 0 ]'
assert "--board-only emits no check/swept/judgment/sweep-skipped lines" \
  '! grep -Eq "^(check|swept|judgment|sweep-skipped|sweep-failed|harvest) " "$tmp/full-boardonly-out.txt"'
assert "--board-only does not invoke integration_sync" '[ ! -f "$sync_marker_bo" ]'

# --board-only fast mode (task 7): LOCK that the early exit sits immediately after board_pass,
# even when the fixture WOULD sweep in a full run (a merged `implemented` change present).
# Mock every sweep/checks/sync sub-script with a marker-touching stub and assert none fire.
bo_dir="$tmp/board-only-lock-case"
git_repo_setup "$bo_dir"
git clone -q "$bo_dir/origin.git" "$bo_dir/work" 2>/dev/null
mkdir -p "$bo_dir/work/docs/changes/active" "$bo_dir/work/docs/adrs"
cat > "$bo_dir/work/docs/changes/active/0040-mergeable-thing.md" <<'EOF'
---
id: 40
slug: mergeable-thing
title: Mergeable thing
status: implemented
priority: high
depends_on: []
branch: feat/mergeable-thing
pr: 40
EOF
git -C "$bo_dir/work" -c user.email=t@t -c user.name=t add docs/changes
git -C "$bo_dir/work" -c user.email=t@t -c user.name=t commit -q -m "seed board-only-lock fixture"
git -C "$bo_dir/work" push -q origin main

mkdir -p "$tmp/mock-bo"
bo_marker_checks="$tmp/mock-bo/.marker-board-checks"
bo_marker_sync="$tmp/mock-bo/.marker-sync-integration"
bo_marker_archive="$tmp/mock-bo/.marker-archive"
bo_marker_cleanup="$tmp/mock-bo/.marker-cleanup"
rm -f "$bo_marker_checks" "$bo_marker_sync" "$bo_marker_archive" "$bo_marker_cleanup"

cat > "$tmp/mock-bo/board-checks.sh" <<EOF
#!/usr/bin/env bash
touch "$bo_marker_checks"
exit 0
EOF
cat > "$tmp/mock-bo/sync-integration-branch.sh" <<EOF
#!/usr/bin/env bash
touch "$bo_marker_sync"
exit 0
EOF
cat > "$tmp/mock-bo/archive-change.sh" <<EOF
#!/usr/bin/env bash
touch "$bo_marker_archive"
exit 0
EOF
cat > "$tmp/mock-bo/cleanup-feature-branch.sh" <<EOF
#!/usr/bin/env bash
touch "$bo_marker_cleanup"
exit 0
EOF
chmod +x "$tmp/mock-bo/"*.sh

cat > "$tmp/gh-bo.sh" <<'EOF'
#!/usr/bin/env bash
echo "gh-bo: should never be invoked in --board-only: $*" >&2
exit 1
EOF
chmod +x "$tmp/gh-bo.sh"

write_board_fixture inline
(cd "$bo_dir/work" && \
  CONFIG_EXPORT_CMD="bash $tmp/fixture-board.sh" GH="$tmp/gh-bo.sh" \
  SCRIPTS_DIR="$tmp/mock-bo" \
  "$SCRIPT" --board-only >"$tmp/bo-out.txt" 2>"$tmp/bo-err.txt")
rc=$?
assert "board-only-lock: exits zero" '[ $rc -eq 0 ]'
assert "board-only-lock: emits board inline line" 'grep -qw "board" "$tmp/bo-out.txt" && grep -qw "inline" "$tmp/bo-out.txt"'
assert "board-only-lock: no swept/harvest/check/judgment/sweep-failed/sweep-skipped lines" \
  '! grep -Eq "^(swept|harvest|check|judgment|sweep-failed|sweep-skipped) " "$tmp/bo-out.txt"'
assert "board-only-lock: board-checks.sh never invoked" '[ ! -f "$bo_marker_checks" ]'
assert "board-only-lock: sync-integration-branch.sh never invoked" '[ ! -f "$bo_marker_sync" ]'
assert "board-only-lock: archive-change.sh never invoked" '[ ! -f "$bo_marker_archive" ]'
assert "board-only-lock: cleanup-feature-branch.sh never invoked" '[ ! -f "$bo_marker_cleanup" ]'

# determinism / idempotence: a full orchestrator pass over a fixture, then a second full pass
# over the now-unchanged change files. Board output must be byte-identical across runs, the
# second run must be a board no-op ("board inline clean", no re-commit), and re-running
# detect_merged/sweep_execute over an already-`done` change must not re-emit "swept".
det_dir="$tmp/det-case"
git_repo_setup "$det_dir"
git clone -q "$det_dir/origin.git" "$det_dir/work" 2>/dev/null
mkdir -p "$det_dir/work/docs/changes/active" "$det_dir/work/docs/adrs"
cat > "$det_dir/work/docs/changes/active/0050-det-thing.md" <<'EOF'
---
id: 50
slug: det-thing
title: Det thing
status: implemented
priority: high
depends_on: []
branch: feat/det-thing
pr: 50
EOF
git -C "$det_dir/work" -c user.email=t@t -c user.name=t add docs/changes
git -C "$det_dir/work" -c user.email=t@t -c user.name=t commit -q -m "seed determinism fixture"
git -C "$det_dir/work" push -q origin main

mkdir -p "$tmp/mock-det"
cat > "$tmp/mock-det/archive-change.sh" <<'EOF'
#!/usr/bin/env bash
changes_dir="" id="" date=""
while [ $# -gt 0 ]; do
  case "$1" in
    --changes-dir) changes_dir="$2"; shift ;;
    --id) id="$2"; shift ;;
    --date) date="$2"; shift ;;
  esac
  shift
done
pad="$(printf '%04d' "$id")"
active="$(find "$changes_dir/active" -maxdepth 1 -name "${pad}-*.md" | head -n1)"
[ -n "$active" ] || exit 1
base="$(basename "$active")"
slug="${base#"${pad}"-}"; slug="${slug%.md}"
mkdir -p "$changes_dir/archive"
dest="$changes_dir/archive/${date}-${pad}-${slug}.md"
root="$(git -C "$changes_dir" rev-parse --show-toplevel)"
git -C "$root" mv "$active" "$dest"
sed -i.bak "s/^status:.*/status: done/" "$dest" && rm -f "$dest.bak"
git -C "$root" add -- "$dest" 2>/dev/null
git -C "$root" -c user.email=t@t -c user.name=t commit -q -m "mock archive" >/dev/null 2>&1
git -C "$root" push -q origin main >/dev/null 2>&1
exit 0
EOF
cat > "$tmp/mock-det/render-change-links.sh" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
cat > "$tmp/mock-det/terminal-publish.sh" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
cat > "$tmp/mock-det/cleanup-feature-branch.sh" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
cat > "$tmp/mock-det/board-checks.sh" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
cat > "$tmp/mock-det/sync-integration-branch.sh" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
chmod +x "$tmp/mock-det/"*.sh

cat > "$tmp/gh-det.sh" <<'EOF'
#!/usr/bin/env bash
if [ "$1" = repo ] && [ "$2" = view ]; then
  echo "x/y"; exit 0
fi
if [ "$1" = api ] && [ "$2" = graphql ]; then
  cat <<'JSON'
{"data":{"p50":{"pullRequest":{"number":50,"mergedAt":"2026-07-08T12:00:00Z","state":"MERGED"}}}}
JSON
  exit 0
fi
echo "gh-det: unexpected args: $*" >&2
exit 1
EOF
chmod +x "$tmp/gh-det.sh"

write_full_fixture none
(cd "$det_dir/work" && \
  CONFIG_EXPORT_CMD="bash $tmp/fixture-full.sh" GH="$tmp/gh-det.sh" \
  SCRIPTS_DIR="$tmp/mock-det" \
  "$SCRIPT" >"$tmp/det-run1.txt" 2>"$tmp/det-run1-err.txt")
rc=$?
assert "determinism run1 exits zero" '[ $rc -eq 0 ]'
assert "determinism run1 emits swept" 'grep -qE "^swept 50 2026-07-08$" "$tmp/det-run1.txt"'

# Mock archive-change.sh mutates status to done so the second run's active file already
# reflects sweep having happened, but since the real archive script is mocked as a no-op above,
# the fixture's `implemented` file stays put — so instead lock board determinism via two
# board-only passes, plus idempotence of detect_merged/sweep_execute over an already-`done` change.
write_board_fixture inline
(cd "$det_dir/work" && \
  CONFIG_EXPORT_CMD="bash $tmp/fixture-board.sh" \
  "$SCRIPT" --board-only >"$tmp/det-board1.txt" 2>"$tmp/det-board1-err.txt")
rc=$?
assert "determinism board pass 1 exits zero" '[ $rc -eq 0 ]'
cp "$det_dir/work/docs/changes/BOARD.md" "$tmp/det-board-snapshot1.md"

(cd "$det_dir/work" && \
  CONFIG_EXPORT_CMD="bash $tmp/fixture-board.sh" \
  "$SCRIPT" --board-only >"$tmp/det-board2.txt" 2>"$tmp/det-board2-err.txt")
rc=$?
assert "determinism board pass 2 exits zero" '[ $rc -eq 0 ]'
assert "determinism: second board pass is a no-op (board inline clean)" \
  'grep -qF "board inline clean" "$tmp/det-board2.txt"'
assert "determinism: BOARD.md byte-identical across the two board-only runs" \
  'cmp -s "$tmp/det-board-snapshot1.md" "$det_dir/work/docs/changes/BOARD.md"'

# Idempotence: re-run detect_merged | sweep_execute over a change already at `done` (as it
# would be after a real sweep) — must not re-emit "swept".
done_dir="$tmp/done-case"
git_repo_setup "$tmp/done-seed"
git clone -q "$tmp/done-seed/origin.git" "$done_dir" 2>/dev/null
mkdir -p "$done_dir/docs/changes/active" "$done_dir/docs/changes/archive"
cat > "$done_dir/docs/changes/active/0051-already-done.md" <<'EOF'
---
id: 51
slug: already-done
title: Already done
status: done
priority: high
depends_on: []
branch: feat/already-done
pr: 51
EOF

idem_out="$( cd "$done_dir" && \
  DOCKET_MODE=main CHANGES_DIR=docs/changes GH="$tmp/gh-det.sh" \
  bash -c '. "'"$SCRIPT"'"; detect_merged' )"
assert "idempotence: detect_merged skips an already-done change (implemented-only filter)" \
  '! printf "%s\n" "$idem_out" | grep -q "already-done"'

sweep_idem_input="$tmp/sweep-idem-input.tsv"
printf '51\talready-done\t51\t2026-07-08\n' > "$sweep_idem_input"
sweep_idem_out="$( cd "$done_dir" && \
  DOCKET_MODE=main CHANGES_DIR=docs/changes ADRS_DIR=docs/adrs \
  INTEGRATION_BRANCH=main METADATA_BRANCH=main \
  SCRIPTS_DIR="$tmp/mock-det" \
  bash -c '. "'"$SCRIPT"'"; sweep_execute < "'"$sweep_idem_input"'"' )"
assert "idempotence: sweep_execute over an already-done change emits no swept line" \
  '! printf "%s\n" "$sweep_idem_out" | grep -qE "^swept 51 "'

# main-mode degradation: DOCKET_MODE=main, no .docket worktree anywhere — board renders
# against the primary tree (mw="."), and integration_sync is a genuine no-op appropriate to
# main-mode (still invoked as a best-effort call, but touches nothing beyond it). Run exits 0
# and never creates/uses a .docket metadata worktree.
mm_dir="$tmp/mainmode-case"
git_repo_setup "$mm_dir"
git clone -q "$mm_dir/origin.git" "$mm_dir/work" 2>/dev/null
seed_changes_fixture "$mm_dir/work"
git -C "$mm_dir/work" -c user.email=t@t -c user.name=t add docs/changes
git -C "$mm_dir/work" -c user.email=t@t -c user.name=t commit -q -m "seed main-mode fixture"
git -C "$mm_dir/work" push -q origin main

mkdir -p "$tmp/mock-mm"
mm_sync_marker="$tmp/mock-mm/.marker-sync"
cat > "$tmp/mock-mm/sync-integration-branch.sh" <<EOF
#!/usr/bin/env bash
touch "$mm_sync_marker"
exit 0
EOF
chmod +x "$tmp/mock-mm/sync-integration-branch.sh"

write_board_fixture inline
assert "main-mode: .docket worktree absent before run" '[ ! -d "$mm_dir/work/.docket" ]'
(cd "$mm_dir/work" && \
  CONFIG_EXPORT_CMD="bash $tmp/fixture-board.sh" SCRIPTS_DIR="$tmp/mock-mm" \
  "$SCRIPT" --board-only >"$tmp/mm-out.txt" 2>"$tmp/mm-err.txt")
rc=$?
assert "main-mode run exits zero" '[ $rc -eq 0 ]'
assert "main-mode: board renders against primary tree (BOARD.md written at repo root)" \
  '[ -s "$mm_dir/work/docs/changes/BOARD.md" ]'
assert "main-mode: no .docket metadata worktree created" '[ ! -d "$mm_dir/work/.docket" ]'
assert "main-mode: board reports changed pushed" 'grep -qw "changed" "$tmp/mm-out.txt" && grep -qw "pushed" "$tmp/mm-out.txt"'

# main-mode: integration_sync is invoked only when a sweep happened; with --board-only it's
# skipped entirely, and no .docket worktree is created regardless of sweep activity. Confirm a
# full (non --board-only) main-mode run with no merges also never creates .docket.
mm2_dir="$tmp/mainmode-full-case"
git_repo_setup "$mm2_dir"
git clone -q "$mm2_dir/origin.git" "$mm2_dir/work" 2>/dev/null
mkdir -p "$mm2_dir/work/docs/changes/active" "$mm2_dir/work/docs/adrs"
git -C "$mm2_dir/work" -c user.email=t@t -c user.name=t commit -q --allow-empty -m "seed mm2" 2>/dev/null || true
git -C "$mm2_dir/work" push -q origin main 2>/dev/null || true

write_full_fixture none
mkdir -p "$tmp/mock-mm2"
cat > "$tmp/mock-mm2/board-checks.sh" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
mm2_sync_marker="$tmp/mock-mm2/.marker-sync"
cat > "$tmp/mock-mm2/sync-integration-branch.sh" <<EOF
#!/usr/bin/env bash
touch "$mm2_sync_marker"
exit 0
EOF
chmod +x "$tmp/mock-mm2/"*.sh
(cd "$mm2_dir/work" && \
  CONFIG_EXPORT_CMD="bash $tmp/fixture-full.sh" GH="$tmp/gh-full-none.sh" \
  SCRIPTS_DIR="$tmp/mock-mm2" \
  "$SCRIPT" >"$tmp/mm2-out.txt" 2>"$tmp/mm2-err.txt")
rc=$?
assert "main-mode full run (no merges) exits zero" '[ $rc -eq 0 ]'
assert "main-mode full run: integration_sync not invoked (no sweep)" '[ ! -f "$mm2_sync_marker" ]'
assert "main-mode full run: no .docket metadata worktree created" '[ ! -d "$mm2_dir/work/.docket" ]'

# skill-body wiring: the docket-status SKILL invokes the orchestrator script and no longer
# inlines the full per-change sweep loop prose it now delegates to docket-status.sh.
SKILL="$REPO/skills/docket-status/SKILL.md"
assert "SKILL invokes the orchestrator (docket.sh docket-status)" 'grep -qF "docket.sh docket-status" "$SKILL"'
assert "SKILL no longer inlines the sweep loop enumeration" \
  '! grep -qF "For each \`implemented\` change:" "$SKILL"'

# --- change 0069: the report is self-evidencing and board-independent ---
# A board-off repo (`board_surfaces: []`, resolved by docket-config.sh to `BOARD_SURFACES=none` —
# change 0071) must still get a complete, positive report: `board off`, the backlog digest, and
# `pass ok` — and must still perform ZERO git writes and leave no BOARD.md.
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

write_board_fixture none
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
write_board_fixture none
(cd "$tmp/boardoff-case/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-board.sh" SCRIPTS_DIR="$tmp/stub-scripts" "$SCRIPT" --board-only >"$tmp/degrade-out.txt" 2>"$tmp/degrade-err.txt")
rc=$?
assert "failing digest still exits 0 (best-effort)" '[ $rc -eq 0 ]'
assert "failing digest emits no digest lines" '! grep -qE "^(backlog|change) " "$tmp/degrade-out.txt"'
assert "failing digest still emits 'board off'" 'grep -qxF "board off" "$tmp/degrade-out.txt"'
assert "failing digest still closes with 'pass ok'" 'grep -qxF "pass ok" "$tmp/degrade-out.txt"'
# Anchored on the diagnostic's own text: stderr is NEVER empty here (git pull --rebase noise and
# the failing stub's own output land there), so `[ -s ... ]` would pass even with the diagnostic
# deleted — a green assert for the wrong reason.
assert "failing digest logs its diagnostic to stderr" \
  'grep -qF "backlog digest failed" "$tmp/degrade-err.txt"'

# --- change 0069: the digest on a FULL (non --board-only) pass — ungated, and POST-SWEEP ---
# Every other full-pass fixture in this file points SCRIPTS_DIR at a mock dir that carries NO
# render-board.sh, so the digest silently takes its best-effort failure branch there and the full
# path's digest + `pass ok` were entirely unproven (deleting either left the suite green). This
# fixture carries the REAL render-board.sh — plus its lib/, which it sources relative to its own
# location — so a full pass genuinely renders the digest.
#
# It locks two things at once:
#   1. UNGATED: with BOARD_SURFACES=none the full pass still emits the digest and `pass ok`.
#   2. POST-SWEEP: backlog_pass runs AFTER the sweep, so a change swept during this very pass is
#      reported as `done` — never as the `implemented` it was when the pass began. This is the
#      report's self-consistency: the digest is the sole backlog channel, so a pre-sweep snapshot
#      would have the same report say "swept 60" and "change 60 implemented" with no correction.
mkdir -p "$tmp/mock-real/lib"
cp "$REPO/scripts/render-board.sh" "$tmp/mock-real/render-board.sh"
cp "$REPO"/scripts/lib/*.sh "$tmp/mock-real/lib/"
cat > "$tmp/mock-real/board-checks.sh" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
cat > "$tmp/mock-real/archive-change.sh" <<'EOF'
#!/usr/bin/env bash
changes_dir="" id="" date=""
while [ $# -gt 0 ]; do
  case "$1" in
    --changes-dir) changes_dir="$2"; shift ;;
    --id) id="$2"; shift ;;
    --date) date="$2"; shift ;;
  esac
  shift
done
pad="$(printf '%04d' "$id")"
active="$(find "$changes_dir/active" -maxdepth 1 -name "${pad}-*.md" | head -n1)"
[ -n "$active" ] || exit 1
base="$(basename "$active")"
slug="${base#"${pad}"-}"; slug="${slug%.md}"
mkdir -p "$changes_dir/archive"
dest="$changes_dir/archive/${date}-${pad}-${slug}.md"
root="$(git -C "$changes_dir" rev-parse --show-toplevel)"
git -C "$root" mv "$active" "$dest"
sed -i.bak "s/^status:.*/status: done/" "$dest" && rm -f "$dest.bak"
git -C "$root" add -- "$dest" 2>/dev/null
git -C "$root" -c user.email=t@t -c user.name=t commit -q -m "mock archive" >/dev/null 2>&1
git -C "$root" push -q origin main >/dev/null 2>&1
exit 0
EOF
for s in render-change-links terminal-publish cleanup-feature-branch sync-integration-branch; do
  cat > "$tmp/mock-real/$s.sh" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
done
chmod +x "$tmp/mock-real/"*.sh

cat > "$tmp/gh-fullpass.sh" <<'EOF'
#!/usr/bin/env bash
if [ "$1" = api ] && [ "$2" = graphql ]; then
  cat <<'JSON'
{"data":{"p60":{"pullRequest":{"number":60,"mergedAt":"2026-07-11T09:00:00Z","state":"MERGED"}}}}
JSON
  exit 0
fi
echo "gh-fullpass: unexpected args: $*" >&2
exit 1
EOF
chmod +x "$tmp/gh-fullpass.sh"

fp_dir="$tmp/fullpass-digest-case"
git_repo_setup "$fp_dir"
git clone -q "$fp_dir/origin.git" "$fp_dir/work" 2>/dev/null
mkdir -p "$fp_dir/work/docs/changes/active" "$fp_dir/work/docs/changes/archive" "$fp_dir/work/docs/adrs"
# 0060 — implemented with a merged PR: this is the change the pass sweeps to done.
cat > "$fp_dir/work/docs/changes/active/0060-gate-thing.md" <<'EOF'
---
id: 60
slug: gate-thing
title: Gate thing
status: implemented
priority: high
depends_on: []
branch: feat/gate-thing
pr: 60
EOF
# 0061 + 0062 — survive the sweep, so the post-sweep digest has real rows (>=2: plurality).
cat > "$fp_dir/work/docs/changes/active/0061-alfa.md" <<'EOF'
---
id: 61
slug: alfa
title: Alfa feature
status: proposed
priority: medium
depends_on: []
spec: docs/superpowers/specs/2026-07-01-alfa.md
EOF
cat > "$fp_dir/work/docs/changes/active/0062-bravo-two.md" <<'EOF'
---
id: 62
slug: bravo-two
title: Bravo two
status: in-progress
priority: low
depends_on: []
branch: feat/bravo-two
EOF
git -C "$fp_dir/work" -c user.email=t@t -c user.name=t add docs/changes
git -C "$fp_dir/work" -c user.email=t@t -c user.name=t commit -q -m "seed full-pass digest fixture"
git -C "$fp_dir/work" push -q origin main

write_full_fixture none
(cd "$fp_dir/work" && \
  CONFIG_EXPORT_CMD="bash $tmp/fixture-full.sh" GH="$tmp/gh-fullpass.sh" \
  SCRIPTS_DIR="$tmp/mock-real" \
  "$SCRIPT" --repo x/y >"$tmp/fullpass-out.txt" 2>"$tmp/fullpass-err.txt")
rc=$?
assert "full pass (real renderer) exits zero" '[ $rc -eq 0 ]'
assert "full pass swept the merged change" 'grep -qxF "swept 60 2026-07-11" "$tmp/fullpass-out.txt"'
# (1) ungated: the digest and `pass ok` reach the FULL path, board off and all.
assert "full pass emits 'board off' (board_surfaces none)" \
  'grep -qxF "board off" "$tmp/fullpass-out.txt"'
assert "full pass emits the backlog digest (UNGATED — not just --board-only)" \
  'grep -qxF "change 61 proposed build-ready alfa" "$tmp/fullpass-out.txt" && grep -qxF "change 62 in-progress - bravo-two" "$tmp/fullpass-out.txt"'
assert "full pass digest has >=2 change rows" \
  '[ "$(grep -cE "^change [0-9]+ " "$tmp/fullpass-out.txt")" -ge 2 ]'
assert "full pass closes with 'pass ok' as its LAST line" \
  '[ "$(tail -n1 "$tmp/fullpass-out.txt")" = "pass ok" ]'
# (2) post-sweep: the swept change is `done` in the digest, and is NOT reported as implemented.
assert "full pass digest is POST-sweep: the swept change is counted done" \
  'grep -qxF "backlog done 1" "$tmp/fullpass-out.txt"'
assert "full pass digest never reports the swept change as implemented" \
  '! grep -qE "^change 60 implemented " "$tmp/fullpass-out.txt"'
assert "full pass digest gives the swept (now archived) change no change line at all" \
  '! grep -qE "^change 60 " "$tmp/fullpass-out.txt"'
assert "full pass digest has no implemented rollup left" \
  '! grep -qE "^backlog implemented " "$tmp/fullpass-out.txt"'
# (3) report order: board -> sweep -> checks/judgment -> digest -> pass ok.
fp_swept_ln="$(grep -n "^swept 60 " "$tmp/fullpass-out.txt" | head -n1 | cut -d: -f1)"
fp_digest_ln="$(grep -n "^backlog " "$tmp/fullpass-out.txt" | head -n1 | cut -d: -f1)"
assert "full pass emits the digest AFTER the sweep lines" \
  '[ -n "$fp_swept_ln" ] && [ -n "$fp_digest_ln" ] && [ "$fp_digest_ln" -gt "$fp_swept_ln" ]'

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

# One assert = one clause. A bare "board off" / "digest" grep is NOT a sentinel: both words occur
# several times across this SKILL, so the assert stays green while the clause it exists to guard is
# deleted (or inverted back to "read from BOARD.md" — the exact posture 0069 abolishes). Each is
# therefore anchored on the unique phrase ITS clause owns, and pinned to exactly ONE occurrence so a
# future duplication cannot silently re-open the same hole. Held in variables (not inlined) because
# the assert body is eval'd — a literal backtick inside the double-quoted grep pattern would be
# command substitution.
skill_boardoff_clause='the repo sets `board_surfaces: []` and there is deliberately **no board**'
skill_digest_clause='read from the digest lines — never from the board file'
assert "SKILL names the board-off report line (Read-the-report bullet, exactly once)" \
  '[ "$(grep -cF -- "$skill_boardoff_clause" "$SKILL_MD")" -eq 1 ]'
assert "SKILL summarizes from the digest, not the board file (Final summary, exactly once)" \
  '[ "$(grep -cF -- "$skill_digest_clause" "$SKILL_MD")" -eq 1 ]'

# The Overview is the first thing the dispatched subagent reads: it must name the backlog digest as
# a job/channel of the pass, not just the board/sweep/checks.
skill_overview="$(sed -n '/^## Overview$/,/^## /p' "$SKILL_MD")"
assert "SKILL Overview names the backlog digest as a job of the pass" \
  'printf "%s" "$skill_overview" | grep -qF "backlog digest"'

# The orchestrator contract documents every new line shape.
assert "status contract documents board off"  'grep -qF "board off" "$STATUS_CONTRACT"'
assert "status contract documents pass ok"    'grep -qF "pass ok" "$STATUS_CONTRACT"'
assert "status contract documents the backlog rollup line" \
  'grep -qF "backlog <status> <count>" "$STATUS_CONTRACT"'
assert "status contract documents the change digest line" \
  'grep -qF "change <id> <status> <readiness> <slug>" "$STATUS_CONTRACT"'
assert "status contract states the backlog pass is ungated" \
  'grep -qiF "ungated" "$STATUS_CONTRACT"'
# Change 0071 review, finding 1: the two new report lines that close the "no line at all" hole.
assert "status contract documents the inline-render-failure line" \
  'grep -qF "board inline failed" "$STATUS_CONTRACT"'
assert "status contract documents the unknown-surface-token line" \
  'grep -qF "board <token> unknown" "$STATUS_CONTRACT"'
# Minor 4 (0094 whole-branch review): the contract-coverage guard above enumerates every OTHER
# report-line shape but was never extended for this branch's own new one — the `ready` queue line.
assert "status contract documents the ready queue line" \
  'grep -qF "ready [<id> …]" "$STATUS_CONTRACT"'

# The renderer contract documents the new flag.
assert "render-board contract documents --format" 'grep -qF -- "--format" "$BOARD_CONTRACT"'
assert "render-board contract documents the digest projection" \
  'grep -qF "digest" "$BOARD_CONTRACT"'

# --- (0068) docket-status shares the preflight impl; no private sync copy -----
assert "docket-status sources the shared preflight lib" \
  'grep -q "lib/docket-preflight.sh" "$SCRIPT"'
assert "docket-status calls docket_preflight" \
  'grep -q "docket_preflight" "$SCRIPT"'
assert "docket-status no longer defines a private ensure_and_sync_worktree" \
  '! grep -qE "^ensure_and_sync_worktree\(\)" "$SCRIPT"'

# --- change 0075 §5: the artifacts-refresh block (sweep_execute_one) ---------------------------
# This block was DEAD pre-0075: its pathspec ($archived) carried the same RELATIVE $mw that
# `git -C "$mw"` is already rooted at, so under `git -C .docket` the pathspec `.docket/docs/...`
# matched NOTHING, the refreshed `## Artifacts` block was never committed, and the whole
# commit/push/`return 0` limb never executed. Anchoring $mw brings it alive for the first time, so
# it is tested here with the REAL render-change-links.sh — a no-op mock of that tool routes the
# test straight through the degrade branch and proves nothing (LEARNINGS) — against a change file
# whose `## Artifacts` block is genuinely STALE, so the file really is dirty:
#   (i)  the refreshed ## Artifacts block is actually COMMITTED on the metadata branch
#   (ii) a failure inside the block does NOT abandon terminal-publish or cleanup
#
# All eight sweep-path scripts are the REAL ones (exec'd by absolute path so their own
# $(dirname "$0") lib resolution still finds their real files). DOCKET_CONFIG points render-change-
# links.sh at the same fixture config, so it never shells out to the real docket-config.sh.
mkdir -p "$tmp/mock-a5"
for s in archive-change.sh render-change-links.sh terminal-publish.sh cleanup-feature-branch.sh \
         board-refresh.sh render-board.sh board-checks.sh sync-integration-branch.sh; do
  printf '#!/usr/bin/env bash\nexec %q "$@"\n' "$REPO/scripts/$s" > "$tmp/mock-a5/$s"
  chmod +x "$tmp/mock-a5/$s"
done

cat > "$tmp/fixture-a5.sh" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' \
  'BOOTSTRAP=PROCEED' \
  'METADATA_BRANCH=docket' \
  'INTEGRATION_BRANCH=main' \
  'DOCKET_MODE=docket' \
  'METADATA_WORKTREE=.docket' \
  'CHANGES_DIR=docs/changes' \
  'ADRS_DIR=docs/adrs' \
  'RESULTS_DIR=docs/results' \
  'BOARD_SURFACES=none' \
  'TERMINAL_PUBLISH=true'
EOF
chmod +x "$tmp/fixture-a5.sh"

# Re-seed gate_setup's change 0060 ON THE METADATA BRANCH (via a side clone — gate_setup does NOT
# create the .docket worktree; preflight does, later, inside the run under test) with:
#   * a genuinely STALE `## Artifacts` block, so the REAL render-change-links.sh has something to
#     rewrite and the block's `status --porcelain` reports a genuinely dirty file; and
#   * an `updated:` key, which the REAL archive-change.sh's fail-closed postcondition requires
#     (its portable-sed set_field only rewrites keys that already exist).
a5_seed_stale(){
  local root="$1" c
  git clone -q "$root/origin.git" "$root/stale-seed" 2>/dev/null
  git -C "$root/stale-seed" checkout -q docket
  c="$root/stale-seed/docs/changes/active/0060-gate-thing.md"
  cat > "$c" <<'EOF'
---
id: 60
slug: gate-thing
title: Gate thing
status: implemented
priority: high
depends_on: []
branch: feat/gate-thing
pr: 60
updated: 2026-07-01
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| STALE | stale-placeholder |
<!-- docket:artifacts:end -->

Body.
EOF
  git -C "$root/stale-seed" add -A
  git -C "$root/stale-seed" -c user.email=t@t -c user.name=t commit -q -m "stale artifacts block"
  git -C "$root/stale-seed" push -q origin docket
}

a5="$tmp/a5-case"
gate_setup "$a5"
a5_seed_stale "$a5"

(cd "$a5/work" && \
  CONFIG_EXPORT_CMD="bash $tmp/fixture-a5.sh" DOCKET_CONFIG="$tmp/fixture-a5.sh" \
  GH="$tmp/gh-gate.sh" SCRIPTS_DIR="$tmp/mock-a5" \
  "$SCRIPT" --repo x/y >"$tmp/a5-out.txt" 2>"$tmp/a5-err.txt")
rc=$?
a5_archived="docs/changes/archive/2026-07-11-0060-gate-thing.md"
assert "0075 §5: the sweep exits zero" '[ $rc -eq 0 ]'
assert "0075 §5: the change is swept" 'grep -qE "^swept 60 " "$tmp/a5-out.txt"'
assert "0075 §5: no sweep-failed line on the happy path" \
  '! grep -q "^sweep-failed 60 " "$tmp/a5-out.txt"'
git -C "$a5/work" fetch origin docket >/dev/null 2>&1
assert "0075 §5: the refreshed ## Artifacts block is COMMITTED on the metadata branch (the block was DEAD pre-0075)" \
  '! git -C "$a5/work" show "origin/docket:$a5_archived" | grep -q "stale-placeholder"'
assert "0075 §5: the committed block is the REAL renderer's output (the PR row), not just a deletion" \
  'git -C "$a5/work" show "origin/docket:$a5_archived" | grep -qF "| PR | 60 |"'
assert "0075 §5: the close-out still completed — terminal-publish landed the record on main" \
  'git -C "$a5/work" fetch origin main >/dev/null 2>&1; git -C "$a5/work" ls-tree -r --name-only origin/main | grep -q "$a5_archived"'
assert "0075 §5: the close-out still completed — cleanup removed the feature worktree" \
  '[ ! -e "$a5/work/.worktrees/gate-thing" ]'

# (ii) THE LANDMINE: make the artifacts-refresh PUSH fail, and prove the close-out still finishes.
# The fault is injected as a `pre-receive` hook on the bare origin that rejects ONLY the
# artifacts-refresh commit on refs/heads/docket. It has to be that surgical: archive-change.sh's
# own push to docket, terminal-publish.sh's push to main and cleanup-feature-branch.sh's remote
# branch delete must all still SUCCEED, or the "did not abandon the close-out" asserts below would
# pass (or fail) for the wrong reason. (A blanket broken push URL — the obvious recipe — is worse
# than useless here: linked worktrees SHARE the repo config, so it would also break archive-change,
# terminal-publish and cleanup, and archive-change's `until push; do pull --rebase; done` cas_push
# would spin forever.)
a5b="$tmp/a5b-case"
gate_setup "$a5b"
a5_seed_stale "$a5b"
cat > "$a5b/origin.git/hooks/pre-receive" <<'EOF'
#!/usr/bin/env bash
# Checks the whole PUSHED RANGE ($old..$new), not just the tip, so a fixture edit that
# stacks another commit on top of "refresh artifacts links" can't silently slip it past this
# hook (0075 review finding 3). Handles the all-zeros $old branch-creation case by treating the
# range as every ancestor of $new.
zero=0000000000000000000000000000000000000000
while read -r old new ref; do
  case "$ref" in
    refs/heads/docket)
      [ "$new" = "$zero" ] && continue
      range="$new"
      [ "$old" != "$zero" ] && range="$old..$new"
      if git log $range --format=%s | grep -q "refresh artifacts links"; then
        echo "pre-receive: rejecting the artifacts-refresh push (0075 §5 test fixture)" >&2
        exit 1
      fi ;;
  esac
done
exit 0
EOF
chmod +x "$a5b/origin.git/hooks/pre-receive"

(cd "$a5b/work" && \
  CONFIG_EXPORT_CMD="bash $tmp/fixture-a5.sh" DOCKET_CONFIG="$tmp/fixture-a5.sh" \
  GH="$tmp/gh-gate.sh" SCRIPTS_DIR="$tmp/mock-a5" \
  "$SCRIPT" --repo x/y >"$tmp/a5b-out.txt" 2>"$tmp/a5b-err.txt")
rc=$?
assert "0075 §5(landmine): the sweep still exits zero when the artifacts push fails" '[ $rc -eq 0 ]'
assert "0075 §5(landmine): the push failure is REPORTED on the report channel" \
  'grep -qE "^sweep-failed 60 render-change-links push-failed$" "$tmp/a5b-out.txt"'
assert "0075 §5(landmine): the push really was rejected — the stale block survives on the metadata branch (cosmetic)" \
  'git -C "$a5b/work" fetch origin docket >/dev/null 2>&1; git -C "$a5b/work" show "origin/docket:$a5_archived" | grep -q "stale-placeholder"'
assert "0075 §5(landmine): the failure does NOT abandon terminal-publish — the record still landed on main" \
  'git -C "$a5b/work" fetch origin main >/dev/null 2>&1; git -C "$a5b/work" ls-tree -r --name-only origin/main | grep -q "$a5_archived"'
assert "0075 §5(landmine): the failure does NOT abandon cleanup — the feature worktree is gone" \
  '[ ! -e "$a5b/work/.worktrees/gate-thing" ]'
assert "0075 §5(landmine): the failure does NOT abandon cleanup — the remote feat branch is gone" \
  '! git -C "$a5b/work" ls-remote --exit-code origin feat/gate-thing >/dev/null 2>&1'
assert "0075 §5(landmine): the sweep still reports the change as swept" \
  'grep -qE "^swept 60 " "$tmp/a5b-out.txt"'

# --- change 0067: the learnings index self-heal + two needs-you advisories ---------------------
# Fixture realism (this project's #1 test-defect class, per a prior finding on this very file — a
# docket-status fixture once pointed SCRIPTS_DIR at a mock dir with no render-board.sh at all,
# giving that change's headline claims ZERO real coverage): SCRIPTS_DIR here carries the REAL
# render-learnings-index.sh AND its lib/, never a bare stub. The wrapper below traces every
# invocation (via LEARN_TRACE, when set) and then DELEGATES to the real, co-located renderer — so
# the disabled-path tests can prove non-invocation while every enabled-path test still exercises
# genuine renderer behavior, not a best-effort degrade branch.
mkdir -p "$tmp/mock-learn/lib"
cp "$REPO/scripts/render-learnings-index.sh" "$tmp/mock-learn/render-learnings-index.sh.real"
cp "$REPO"/scripts/lib/*.sh "$tmp/mock-learn/lib/"
chmod +x "$tmp/mock-learn/render-learnings-index.sh.real"
cat > "$tmp/mock-learn/render-learnings-index.sh" <<'EOF'
#!/usr/bin/env bash
[ -n "${LEARN_TRACE:-}" ] && printf 'render-learnings-index %s\n' "$*" >> "$LEARN_TRACE"
exec "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/render-learnings-index.sh.real" "$@"
EOF
chmod +x "$tmp/mock-learn/render-learnings-index.sh"
cat > "$tmp/mock-learn/board-checks.sh" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
chmod +x "$tmp/mock-learn/board-checks.sh"

# write_learn_fixture ENABLED CAP — main-mode (METADATA_WORKTREE=.), board off (these tests are
# about the learnings pass, not the board), so BOARD_SURFACES=none keeps every fixture minimal.
write_learn_fixture(){
  cat > "$tmp/fixture-learn.sh" <<EOF
#!/usr/bin/env bash
printf '%s\n' \
  'BOOTSTRAP=PROCEED' \
  'METADATA_BRANCH=main' \
  'INTEGRATION_BRANCH=main' \
  'DOCKET_MODE=main' \
  'METADATA_WORKTREE=.' \
  'CHANGES_DIR=docs/changes' \
  'ADRS_DIR=docs/adrs' \
  'RESULTS_DIR=docs/results' \
  'BOARD_SURFACES=none' \
  'LEARNINGS_ENABLED=$1' \
  'LEARNINGS_CAP=$2'
EOF
}

# mkfinding_learn ROOT SLUG STATE — a minimal, real finding file (Task 1's frontmatter shape,
# tests/test_render_learnings_index.sh's mkfinding).
mkfinding_learn(){
  local root="$1" slug="$2" state="$3"
  mkdir -p "$root/docs/changes/learnings"
  cat > "$root/docs/changes/learnings/$slug.md" <<EOF
---
slug: $slug
hook: "Finding for $slug."
topics: [testing]
changes: [1]
created: 2026-06-17
updated: 2026-07-16
promotion_state: $state
promoted_to:
---

## Apply
The rule for $slug.

## War story
- 2026-07-14 (#1, PR #1) — something happened.
EOF
}

# (a) enabled: a stale index is re-rendered, committed, AND reaches origin — never keyed on a
# local proxy (the board pass's own hard-won lesson, change 0071 review finding 3: a clean
# working tree is not evidence a write landed on the remote).
learn_a_dir="$tmp/learn-stale-case"
git_repo_setup "$learn_a_dir"
git clone -q "$learn_a_dir/origin.git" "$learn_a_dir/work" 2>/dev/null
mkfinding_learn "$learn_a_dir/work" guards-are-code retained
printf 'STALE INDEX — must be re-rendered by the pass.\n' > "$learn_a_dir/work/docs/changes/learnings/README.md"
git -C "$learn_a_dir/work" -c user.email=t@t -c user.name=t add docs/changes
git -C "$learn_a_dir/work" -c user.email=t@t -c user.name=t commit -q -m "seed stale learnings index"
git -C "$learn_a_dir/work" push -q origin main

write_learn_fixture true 300
(cd "$learn_a_dir/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-learn.sh" SCRIPTS_DIR="$tmp/mock-learn" \
  "$SCRIPT" >"$tmp/learn-a-out.txt" 2>"$tmp/learn-a-err.txt")
rc=$?
out="$(cat "$tmp/learn-a-out.txt")"
assert "learnings(a): pass exits zero" '[ $rc -eq 0 ]'
assert "status re-renders a stale learnings index" \
  'printf "%s" "$out" | grep -qE "^learnings index (clean|changed)"'
# Strengthens the assert above (which alone is satisfiable by either half of an OR — deleting the
# "changed" branch would still pass it via "clean"): pin to the exact positive shape a genuinely
# stale index must produce.
assert "learnings(a): the stale index really changed (not a false-positive clean)" \
  'grep -qxF "learnings index changed pushed" "$tmp/learn-a-out.txt"'
assert "learnings(a): the render reached origin, not just the local tree (change 0071 finding-3 discipline)" \
  'git -C "$learn_a_dir/work" fetch origin main >/dev/null 2>&1 \
   && git -C "$learn_a_dir/work" show origin/main:docs/changes/learnings/README.md 2>/dev/null | grep -qF "guards-are-code"'
assert "under-cap emits no over-cap advisory" \
  '! printf "%s" "$out" | grep -qF "over-cap"'

# Idempotence: a second run over the now-fresh index reports clean, not changed — the commit path
# never fires twice for the same bytes.
(cd "$learn_a_dir/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-learn.sh" SCRIPTS_DIR="$tmp/mock-learn" \
  "$SCRIPT" >"$tmp/learn-a2-out.txt" 2>"$tmp/learn-a2-err.txt")
rc=$?
assert "learnings(a) second run: exits zero" '[ $rc -eq 0 ]'
assert "learnings(a) second run: reports clean (no re-commit of an unchanged index)" \
  'grep -qxF "learnings index clean" "$tmp/learn-a2-out.txt"'

# (a') unpushed precedent (mirrors board_pass_inline's own change-0071-finding-3 test verbatim):
# a repo whose learnings/README.md already matches a fresh render (clean diff) but whose local
# metadata branch carries that exact commit UNPUSHED — the state a prior failed push leaves
# behind. The pass must not mistake "nothing to commit" for "nothing to push".
learn_u_dir="$tmp/learn-unpushed-case"
git_repo_setup "$learn_u_dir"
git clone -q "$learn_u_dir/origin.git" "$learn_u_dir/work" 2>/dev/null
mkfinding_learn "$learn_u_dir/work" guards-are-code retained
git -C "$learn_u_dir/work" -c user.email=t@t -c user.name=t add docs/changes
git -C "$learn_u_dir/work" -c user.email=t@t -c user.name=t commit -q -m "seed unpushed fixture"
git -C "$learn_u_dir/work" push -q origin main

# Render through the REAL renderer directly (the exact bytes the pass would itself produce),
# commit, but do NOT push — this is the exact local state a failed push leaves behind.
"$REPO/scripts/render-learnings-index.sh" --learnings-dir "$learn_u_dir/work/docs/changes/learnings" \
  > "$learn_u_dir/work/docs/changes/learnings/README.md"
git -C "$learn_u_dir/work" -c user.email=t@t -c user.name=t add docs/changes/learnings/README.md
git -C "$learn_u_dir/work" -c user.email=t@t -c user.name=t commit -q -m "docket: learnings index refresh"

assert "learnings(unpushed): local branch really is ahead of origin on the index before the run" \
  '[ "$(git -C "$learn_u_dir/work" rev-list --count origin/main..HEAD -- docs/changes/learnings/README.md)" -gt 0 ]'

write_learn_fixture true 300
(cd "$learn_u_dir/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-learn.sh" SCRIPTS_DIR="$tmp/mock-learn" \
  "$SCRIPT" >"$tmp/learn-u-out.txt" 2>"$tmp/learn-u-err.txt")
rc=$?
assert "learnings(unpushed): run exits zero" '[ $rc -eq 0 ]'
assert "learnings(unpushed): reports changed pushed or push-failed (not vacuous: positive shape match)" \
  'grep -Eq "learnings index changed (pushed|push-failed)" "$tmp/learn-u-out.txt"'
assert "learnings(unpushed): never reports clean" \
  '! grep -qxF "learnings index clean" "$tmp/learn-u-out.txt"'
if grep -q "learnings index changed pushed" "$tmp/learn-u-out.txt"; then
  assert "learnings(unpushed) pushed: remote now carries the previously-unpushed commit" \
    'git -C "$learn_u_dir/work" show origin/main:docs/changes/learnings/README.md 2>/dev/null | cmp -s - "$learn_u_dir/work/docs/changes/learnings/README.md"'
fi

# (b) disabled: exactly one note, the renderer is NEVER invoked, and an existing finding file is
# left byte-untouched (the gate is a read/write gate, never a purge).
learn_b_dir="$tmp/learn-disabled-case"
git_repo_setup "$learn_b_dir"
git clone -q "$learn_b_dir/origin.git" "$learn_b_dir/work" 2>/dev/null
mkfinding_learn "$learn_b_dir/work" seeded retained
SEEDED_BYTES="$(cat "$learn_b_dir/work/docs/changes/learnings/seeded.md")"
git -C "$learn_b_dir/work" -c user.email=t@t -c user.name=t add docs/changes
git -C "$learn_b_dir/work" -c user.email=t@t -c user.name=t commit -q -m "seed disabled fixture"
git -C "$learn_b_dir/work" push -q origin main

write_learn_fixture false 300
learn_trace="$tmp/learn-trace-disabled.log"; rm -f "$learn_trace"
(cd "$learn_b_dir/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-learn.sh" SCRIPTS_DIR="$tmp/mock-learn" \
  LEARN_TRACE="$learn_trace" \
  "$SCRIPT" >"$tmp/learn-b-out.txt" 2>"$tmp/learn-b-err.txt")
rc=$?
out_disabled="$(cat "$tmp/learn-b-out.txt")"
trace_disabled="$(cat "$learn_trace" 2>/dev/null)"
LD="$learn_b_dir/work/docs/changes/learnings"
assert "learnings(b): pass exits zero" '[ $rc -eq 0 ]'
assert "disabled emits exactly one learnings-disabled note" \
  '[ "$(printf "%s" "$out_disabled" | grep -cF "learnings disabled")" = "1" ]'
assert "disabled never invokes the renderer" \
  '! printf "%s" "$trace_disabled" | grep -qF "render-learnings-index"'
assert "disabled leaves an existing finding file byte-untouched" \
  '[ "$(cat "$LD/seeded.md")" = "$SEEDED_BYTES" ]'
assert "learnings(b): disabled emits no advisories" \
  '! printf "%s" "$out_disabled" | grep -qE "over-cap|promotion-pending"'
assert "learnings(b): disabled never creates the index (no self-heal render at all)" \
  '[ ! -e "$LD/README.md" ]'

# (c) over-cap advisory (needs-you channel, ADR-0028's pattern applied to learnings)
learn_c_dir="$tmp/learn-overcap-case"
git_repo_setup "$learn_c_dir"
git clone -q "$learn_c_dir/origin.git" "$learn_c_dir/work" 2>/dev/null
mkfinding_learn "$learn_c_dir/work" finding-one retained
mkfinding_learn "$learn_c_dir/work" finding-two retained
git -C "$learn_c_dir/work" -c user.email=t@t -c user.name=t add docs/changes
git -C "$learn_c_dir/work" -c user.email=t@t -c user.name=t commit -q -m "seed overcap fixture"
git -C "$learn_c_dir/work" push -q origin main

write_learn_fixture true 1
(cd "$learn_c_dir/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-learn.sh" SCRIPTS_DIR="$tmp/mock-learn" \
  "$SCRIPT" >"$tmp/learn-c-out.txt" 2>"$tmp/learn-c-err.txt")
rc=$?
out_overcap="$(cat "$tmp/learn-c-out.txt")"
assert "learnings(c): pass exits zero" '[ $rc -eq 0 ]'
assert "over-cap surfaces the needs-you advisory" \
  'printf "%s" "$out_overcap" | grep -qF "learnings over-cap — needs curation"'
assert "learnings(c): the over-cap line names the real counts (2 active, cap 1)" \
  'grep -qxF "learnings over-cap — needs curation (2 active, cap 1)" "$tmp/learn-c-out.txt"'

# (d) promotion-pending advisory
learn_d_dir="$tmp/learn-candidate-case"
git_repo_setup "$learn_d_dir"
git clone -q "$learn_d_dir/origin.git" "$learn_d_dir/work" 2>/dev/null
mkfinding_learn "$learn_d_dir/work" needs-promo candidate
git -C "$learn_d_dir/work" -c user.email=t@t -c user.name=t add docs/changes
git -C "$learn_d_dir/work" -c user.email=t@t -c user.name=t commit -q -m "seed candidate fixture"
git -C "$learn_d_dir/work" push -q origin main

write_learn_fixture true 300
(cd "$learn_d_dir/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-learn.sh" SCRIPTS_DIR="$tmp/mock-learn" \
  "$SCRIPT" >"$tmp/learn-d-out.txt" 2>"$tmp/learn-d-err.txt")
rc=$?
out_candidate="$(cat "$tmp/learn-d-out.txt")"
assert "learnings(d): pass exits zero" '[ $rc -eq 0 ]'
assert "a candidate finding surfaces promotion-pending 1" \
  'printf "%s" "$out_candidate" | grep -qF "learnings promotion-pending 1"'
assert "learnings(d): no over-cap advisory (well under cap)" \
  '! printf "%s" "$out_candidate" | grep -qF "over-cap"'

# (e) the cap counts ACTIVE findings — a promoted finding must not count. 3 promoted + 1 retained,
# cap 2: if promoted findings counted, active=4 > cap(2) would (wrongly) fire over-cap.
learn_e_dir="$tmp/learn-promoted-case"
git_repo_setup "$learn_e_dir"
git clone -q "$learn_e_dir/origin.git" "$learn_e_dir/work" 2>/dev/null
mkfinding_learn "$learn_e_dir/work" promoted-one promoted
mkfinding_learn "$learn_e_dir/work" promoted-two promoted
mkfinding_learn "$learn_e_dir/work" promoted-three promoted
mkfinding_learn "$learn_e_dir/work" still-retained retained
git -C "$learn_e_dir/work" -c user.email=t@t -c user.name=t add docs/changes
git -C "$learn_e_dir/work" -c user.email=t@t -c user.name=t commit -q -m "seed promoted-over fixture"
git -C "$learn_e_dir/work" push -q origin main

write_learn_fixture true 2
(cd "$learn_e_dir/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-learn.sh" SCRIPTS_DIR="$tmp/mock-learn" \
  "$SCRIPT" >"$tmp/learn-e-out.txt" 2>"$tmp/learn-e-err.txt")
rc=$?
out_promoted_over="$(cat "$tmp/learn-e-out.txt")"
assert "learnings(e): pass exits zero" '[ $rc -eq 0 ]'
assert "promoted findings do not count toward the cap" \
  '! printf "%s" "$out_promoted_over" | grep -qF "over-cap"'
assert "learnings(e): the fixture really seeded 3 promoted + 1 retained finding (anti-vacuity)" \
  '[ "$(find "$learn_e_dir/work/docs/changes/learnings" -maxdepth 1 -name "*.md" ! -name README.md | wc -l)" -eq 4 ]'

# (f) change 0067 review, finding 3: the two needs-you advisories are computed from the finding
# FILES, not from the render outcome — a broken renderer must not also mute the escalation
# channels precisely when something is already wrong. Force the render to fail (a deliberately
# broken render-learnings-index.sh, never the real renderer) while seeding REAL, valid finding
# files that would otherwise trip both advisories, and assert both still fire.
mkdir -p "$tmp/mock-learn-fail"
cat > "$tmp/mock-learn-fail/render-learnings-index.sh" <<'EOF'
#!/usr/bin/env bash
echo "render-learnings-index-fail: deliberately broken for the F3 regression test" >&2
exit 1
EOF
chmod +x "$tmp/mock-learn-fail/render-learnings-index.sh"
cat > "$tmp/mock-learn-fail/board-checks.sh" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
chmod +x "$tmp/mock-learn-fail/board-checks.sh"

learn_f_dir="$tmp/learn-renderfail-case"
git_repo_setup "$learn_f_dir"
git clone -q "$learn_f_dir/origin.git" "$learn_f_dir/work" 2>/dev/null
mkfinding_learn "$learn_f_dir/work" finding-one retained
mkfinding_learn "$learn_f_dir/work" finding-two retained
mkfinding_learn "$learn_f_dir/work" needs-promo candidate
git -C "$learn_f_dir/work" -c user.email=t@t -c user.name=t add docs/changes
git -C "$learn_f_dir/work" -c user.email=t@t -c user.name=t commit -q -m "seed renderfail fixture"
git -C "$learn_f_dir/work" push -q origin main

write_learn_fixture true 1
(cd "$learn_f_dir/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-learn.sh" SCRIPTS_DIR="$tmp/mock-learn-fail" \
  "$SCRIPT" >"$tmp/learn-f-out.txt" 2>"$tmp/learn-f-err.txt")
rc=$?
assert "learnings(f): pass exits zero (a broken render is best-effort, not fatal)" '[ $rc -eq 0 ]'
assert "learnings(f): reports the render failure" \
  'grep -qxF "learnings index failed" "$tmp/learn-f-out.txt"'
assert "learnings(f): F3 — over-cap advisory STILL fires despite the broken render" \
  'grep -qxF "learnings over-cap — needs curation (3 active, cap 1)" "$tmp/learn-f-out.txt"'
assert "learnings(f): F3 — promotion-pending advisory STILL fires despite the broken render" \
  'grep -qxF "learnings promotion-pending 1 — needs you" "$tmp/learn-f-out.txt"'
assert "learnings(f): the failed render wrote no README.md" \
  '[ ! -e "$learn_f_dir/work/docs/changes/learnings/README.md" ]'

# --- wiring: the learnings pass runs on the FULL path only, never under --board-only -----------
learn_wire_dir="$tmp/learn-wiring-case"
git_repo_setup "$learn_wire_dir"
git clone -q "$learn_wire_dir/origin.git" "$learn_wire_dir/work" 2>/dev/null
mkfinding_learn "$learn_wire_dir/work" guards-are-code retained
git -C "$learn_wire_dir/work" -c user.email=t@t -c user.name=t add docs/changes
git -C "$learn_wire_dir/work" -c user.email=t@t -c user.name=t commit -q -m "seed wiring fixture"
git -C "$learn_wire_dir/work" push -q origin main

write_learn_fixture true 300
(cd "$learn_wire_dir/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-learn.sh" SCRIPTS_DIR="$tmp/mock-learn" \
  "$SCRIPT" --board-only >"$tmp/learn-bo-out.txt" 2>"$tmp/learn-bo-err.txt")
rc=$?
assert "learnings wiring: --board-only exits zero" '[ $rc -eq 0 ]'
assert "learnings wiring: --board-only emits NO learnings line at all" \
  '! grep -q "^learnings" "$tmp/learn-bo-out.txt"'
assert "learnings wiring: --board-only never creates the index" \
  '[ ! -e "$learn_wire_dir/work/docs/changes/learnings/README.md" ]'

(cd "$learn_wire_dir/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-learn.sh" SCRIPTS_DIR="$tmp/mock-learn" \
  "$SCRIPT" >"$tmp/learn-full-out.txt" 2>"$tmp/learn-full-err.txt")
rc=$?
assert "learnings wiring: the full pass exits zero" '[ $rc -eq 0 ]'
assert "learnings wiring: the full pass DOES emit a learnings line" \
  'grep -q "^learnings" "$tmp/learn-full-out.txt"'

# ============================================================================
# Task 6 (change 0089): docket-status reclaim wiring — health_checks forwards the
# lease TTL to board-checks; on the FULL path only, reclaim_pass prints a
# state-valid remedy (reclaim.auto off) OR runs the mutating reclaim sweep
# (reclaim.auto on). Never on --board-only. The mutation is gated behind BOTH a
# [reclaimable] finding AND reclaim.auto=true.
# ============================================================================

# (i) health_checks forwards --lease-ttl-hours (the resolved RECLAIM_LEASE_TTL, not a hardcode).
mkdir -p "$tmp/mock-reclaim-ttl"
cat > "$tmp/mock-reclaim-ttl/board-checks.sh" <<'EOF'
#!/usr/bin/env bash
echo "board-checks $*" >> "$HEALTH_LOG"
exit 0
EOF
chmod +x "$tmp/mock-reclaim-ttl/board-checks.sh"
ttl_log="$tmp/reclaim-ttl-calls.log"; : > "$ttl_log"
( cd "$health_dir" && \
  DOCKET_MODE=main CHANGES_DIR=docs/changes INTEGRATION_BRANCH=main METADATA_BRANCH=main \
  RECLAIM_LEASE_TTL=48 SCRIPTS_DIR="$tmp/mock-reclaim-ttl" HEALTH_LOG="$ttl_log" \
  bash -c '. "'"$SCRIPT"'"; health_checks' >/dev/null )
assert "reclaim(ttl): health_checks forwards --lease-ttl-hours to board-checks" \
  'grep -q -- "--lease-ttl-hours" "$ttl_log"'
assert "reclaim(ttl): the forwarded value is the resolved RECLAIM_LEASE_TTL (48, not hardcoded)" \
  'grep -q -- "--lease-ttl-hours 48" "$ttl_log"'

# Shared fixture for the full-run reclaim cases: a main-mode repo with an in-progress change (so
# detect_merged finds no `implemented` change and never touches gh), board_surfaces=none, learnings
# off — minimal output, so the only source of the word "reclaim" is reclaim_pass itself.
reclaim_dir="$tmp/reclaim-wire-case"
git_repo_setup "$reclaim_dir"
git clone -q "$reclaim_dir/origin.git" "$reclaim_dir/work" 2>/dev/null
mkdir -p "$reclaim_dir/work/docs/changes/active" "$reclaim_dir/work/docs/adrs"
cat > "$reclaim_dir/work/docs/changes/active/0023-expirednobranch.md" <<'EOF'
---
id: 23
slug: expirednobranch
title: Expired lease no branch
status: in-progress
priority: medium
depends_on: []
claimed_at: 2026-01-01T00:00:00Z
EOF
git -C "$reclaim_dir/work" -c user.email=t@t -c user.name=t add docs/changes
git -C "$reclaim_dir/work" -c user.email=t@t -c user.name=t commit -q -m "seed reclaim-wire fixture"
git -C "$reclaim_dir/work" push -q origin main

mkdir -p "$tmp/mock-reclaim"
# board-checks: always emits TWO stale-in-progress findings carrying the [reclaimable] marker. The
# message deliberately does NOT contain "docket.sh reclaim-claims", so the remedy line's own
# "docket.sh reclaim-claims" is unambiguous evidence reclaim_pass printed it — not the finding text.
# Two findings also make the remedy's count meaningful (2, not a hardcoded 1).
cat > "$tmp/mock-reclaim/board-checks.sh" <<'EOF'
#!/usr/bin/env bash
printf 'stale-in-progress\t23\tclaim lease expired 100h ago; no feature branch [reclaimable]\n'
printf 'stale-in-progress\t27\tclaim lease expired 90h ago; no feature branch [reclaimable]\n'
exit 0
EOF
# reclaim-claims: records that it ran (marker + call log) and emits one report line for reclaim_pass
# to prefix. A stub — it does not actually mutate; this section tests the wiring/gating, not the
# reclaim sweep itself (that is tests/test_reclaim_claims.sh's job).
cat > "$tmp/mock-reclaim/reclaim-claims.sh" <<'EOF'
#!/usr/bin/env bash
echo "reclaim-claims $*" >> "$RECLAIM_LOG"
touch "$RECLAIM_MARKER"
printf 'reclaimed 23 expirednobranch (lease 100h, no branch)\n'
exit 0
EOF
# render-board stub so the ungated backlog pass stays quiet (no digest lines, no stderr noise).
cat > "$tmp/mock-reclaim/render-board.sh" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF
chmod +x "$tmp/mock-reclaim/"*.sh

cat > "$tmp/gh-reclaim.sh" <<'EOF'
#!/usr/bin/env bash
echo "gh-reclaim: should not be invoked (no implemented changes to detect): $*" >&2
exit 1
EOF
chmod +x "$tmp/gh-reclaim.sh"

# write_reclaim_fixture AUTO — main-mode config export carrying RECLAIM_AUTO / RECLAIM_LEASE_TTL.
write_reclaim_fixture(){
  cat > "$tmp/fixture-reclaim.sh" <<EOF
#!/usr/bin/env bash
printf '%s\n' \
  'BOOTSTRAP=PROCEED' \
  'METADATA_BRANCH=main' \
  'INTEGRATION_BRANCH=main' \
  'DOCKET_MODE=main' \
  'METADATA_WORKTREE=.' \
  'CHANGES_DIR=docs/changes' \
  'ADRS_DIR=docs/adrs' \
  'RESULTS_DIR=docs/results' \
  'BOARD_SURFACES=none' \
  'LEARNINGS_ENABLED=false' \
  'RECLAIM_LEASE_TTL=72' \
  'RECLAIM_AUTO=$1'
EOF
}

# (ii) reclaim.auto OFF + a [reclaimable] finding => state-valid remedy printed, NO mutation.
write_reclaim_fixture false
reclaim_marker_off="$tmp/.reclaim-marker-off"; rm -f "$reclaim_marker_off"
reclaim_log_off="$tmp/reclaim-off-calls.log"; : > "$reclaim_log_off"
(cd "$reclaim_dir/work" && \
  CONFIG_EXPORT_CMD="bash $tmp/fixture-reclaim.sh" GH="$tmp/gh-reclaim.sh" \
  SCRIPTS_DIR="$tmp/mock-reclaim" RECLAIM_MARKER="$reclaim_marker_off" RECLAIM_LOG="$reclaim_log_off" \
  "$SCRIPT" >"$tmp/reclaim-off-out.txt" 2>"$tmp/reclaim-off-err.txt")
rc=$?
assert "reclaim(auto off): full pass exits zero" '[ $rc -eq 0 ]'
assert "reclaim(auto off): the [reclaimable] findings reached the report" \
  'grep -qF "[reclaimable]" "$tmp/reclaim-off-out.txt"'
assert "reclaim(auto off): prints the state-valid remedy naming docket.sh reclaim-claims" \
  'printf "%s\n" "$(cat "$tmp/reclaim-off-out.txt")" | grep -qF "docket.sh reclaim-claims"'
assert "reclaim(auto off): the remedy names the reclaimable count (2)" \
  'grep -qF "reclaim: 2 expired-lease change(s) can self-heal" "$tmp/reclaim-off-out.txt"'
assert "reclaim(auto off): does NOT invoke reclaim-claims (no mutation)" \
  '[ ! -f "$reclaim_marker_off" ]'
assert "reclaim(auto off): reclaim-claims call log is empty (never executed)" \
  '[ ! -s "$reclaim_log_off" ]'

# (iii) reclaim.auto ON => reclaim-claims invoked (mutating), report surfaced, remedy NOT printed.
write_reclaim_fixture true
reclaim_marker_on="$tmp/.reclaim-marker-on"; rm -f "$reclaim_marker_on"
reclaim_log_on="$tmp/reclaim-on-calls.log"; : > "$reclaim_log_on"
(cd "$reclaim_dir/work" && \
  CONFIG_EXPORT_CMD="bash $tmp/fixture-reclaim.sh" GH="$tmp/gh-reclaim.sh" \
  SCRIPTS_DIR="$tmp/mock-reclaim" RECLAIM_MARKER="$reclaim_marker_on" RECLAIM_LOG="$reclaim_log_on" \
  "$SCRIPT" >"$tmp/reclaim-on-out.txt" 2>"$tmp/reclaim-on-err.txt")
rc=$?
assert "reclaim(auto on): full pass exits zero" '[ $rc -eq 0 ]'
assert "reclaim(auto on): invokes reclaim-claims (mutation ran)" \
  '[ -f "$reclaim_marker_on" ]'
assert "reclaim(auto on): forwards --lease-ttl-hours (72) to reclaim-claims" \
  'grep -q -- "--lease-ttl-hours 72" "$reclaim_log_on"'
assert "reclaim(auto on): passes the metadata changes-dir to reclaim-claims" \
  'grep -q -- "--changes-dir" "$reclaim_log_on" && grep -qF "docs/changes" "$reclaim_log_on"'
assert "reclaim(auto on): surfaces the reclaim report prefixed with 'reclaim '" \
  'grep -qF "reclaim reclaimed 23 expirednobranch" "$tmp/reclaim-on-out.txt"'
assert "reclaim(auto on): does NOT print the state-valid remedy (reclaim already ran)" \
  '! grep -qF "docket.sh reclaim-claims" "$tmp/reclaim-on-out.txt"'
assert "reclaim(auto on): prints no 'can self-heal' remedy line" \
  '! grep -qF "can self-heal" "$tmp/reclaim-on-out.txt"'

# (iv) --board-only triggers NEITHER the remedy NOR the mutation, even with reclaim.auto=true —
# board-checks never runs on this path, so there is no [reclaimable] finding to key on.
write_reclaim_fixture true
reclaim_marker_bo="$tmp/.reclaim-marker-bo"; rm -f "$reclaim_marker_bo"
reclaim_log_bo="$tmp/reclaim-bo-calls.log"; : > "$reclaim_log_bo"
(cd "$reclaim_dir/work" && \
  CONFIG_EXPORT_CMD="bash $tmp/fixture-reclaim.sh" GH="$tmp/gh-reclaim.sh" \
  SCRIPTS_DIR="$tmp/mock-reclaim" RECLAIM_MARKER="$reclaim_marker_bo" RECLAIM_LOG="$reclaim_log_bo" \
  "$SCRIPT" --board-only >"$tmp/reclaim-bo-out.txt" 2>"$tmp/reclaim-bo-err.txt")
rc=$?
assert "reclaim(--board-only): exits zero" '[ $rc -eq 0 ]'
assert "reclaim(--board-only): no [reclaimable] finding (board-checks never runs)" \
  '! grep -qF "[reclaimable]" "$tmp/reclaim-bo-out.txt"'
assert "reclaim(--board-only): emits no reclaim line at all (neither remedy nor report)" \
  '! grep -qF "reclaim" "$tmp/reclaim-bo-out.txt"'
assert "reclaim(--board-only): never invokes reclaim-claims even with reclaim.auto=true" \
  '[ ! -f "$reclaim_marker_bo" ]'

# (v) THE MARKER GATE, independent of reclaim.auto: a finding WITHOUT the [reclaimable] marker (e.g.
# an expired lease WITH a live branch — needs-review, not auto-reclaimable) must trigger NEITHER the
# mutation NOR a remedy, even under reclaim.auto=true. Proves the write is gated on the [reclaimable]
# marker itself, not merely on the auto knob — a gate-removal regression fires here.
mkdir -p "$tmp/mock-reclaim-noflag"
cat > "$tmp/mock-reclaim-noflag/board-checks.sh" <<'EOF'
#!/usr/bin/env bash
printf 'stale-in-progress\t24\tclaim lease expired 100h ago; branch feat/x exists — needs your review (not auto-reclaimable)\n'
exit 0
EOF
cp "$tmp/mock-reclaim/reclaim-claims.sh" "$tmp/mock-reclaim-noflag/reclaim-claims.sh"
cp "$tmp/mock-reclaim/render-board.sh" "$tmp/mock-reclaim-noflag/render-board.sh"
chmod +x "$tmp/mock-reclaim-noflag/"*.sh

write_reclaim_fixture true
reclaim_marker_nf="$tmp/.reclaim-marker-noflag"; rm -f "$reclaim_marker_nf"
reclaim_log_nf="$tmp/reclaim-noflag-calls.log"; : > "$reclaim_log_nf"
(cd "$reclaim_dir/work" && \
  CONFIG_EXPORT_CMD="bash $tmp/fixture-reclaim.sh" GH="$tmp/gh-reclaim.sh" \
  SCRIPTS_DIR="$tmp/mock-reclaim-noflag" RECLAIM_MARKER="$reclaim_marker_nf" RECLAIM_LOG="$reclaim_log_nf" \
  "$SCRIPT" >"$tmp/reclaim-nf-out.txt" 2>"$tmp/reclaim-nf-err.txt")
rc=$?
assert "reclaim(no-marker): full pass exits zero" '[ $rc -eq 0 ]'
assert "reclaim(no-marker): the non-reclaimable finding still reaches the report" \
  'grep -qF "not auto-reclaimable" "$tmp/reclaim-nf-out.txt"'
assert "reclaim(no-marker): reclaim.auto=true but NO [reclaimable] marker ⇒ reclaim-claims NOT invoked" \
  '[ ! -f "$reclaim_marker_nf" ]'
assert "reclaim(no-marker): no remedy line printed (nothing to self-heal)" \
  '! grep -qF "can self-heal" "$tmp/reclaim-nf-out.txt"'
assert "reclaim(no-marker): no reclaim-report line printed" \
  '! grep -qF "docket.sh reclaim-claims" "$tmp/reclaim-nf-out.txt"'

# (vi) FORGED MARKER (change 0104 review). [reclaimable] is a machine contract between board-checks
# and this MUTATING gate, but findings echo untrusted frontmatter verbatim by design — a
# `field-domain` message quotes free-form `title` prose. So a change file titled
# `Sneaky | thing [reclaimable]` puts the marker into the findings blob without board-checks ever
# having decided anything is reclaimable. An unscoped substring search of the blob accepts it; the
# gate must anchor on the check-id COLUMN (`^check stale-in-progress `) instead.
#
# (vi-a) forged marker ALONE, under reclaim.auto=true: neither the mutation nor a remedy.
mkdir -p "$tmp/mock-reclaim-forged"
cat > "$tmp/mock-reclaim-forged/board-checks.sh" <<'EOF'
#!/usr/bin/env bash
printf 'field-domain\t44\ttitle contains '"'"'|'"'"', which injects columns into the board row: Sneaky | thing [reclaimable]\n'
exit 0
EOF
cp "$tmp/mock-reclaim/reclaim-claims.sh" "$tmp/mock-reclaim-forged/reclaim-claims.sh"
cp "$tmp/mock-reclaim/render-board.sh"   "$tmp/mock-reclaim-forged/render-board.sh"
chmod +x "$tmp/mock-reclaim-forged/"*.sh

write_reclaim_fixture true
reclaim_marker_fg="$tmp/.reclaim-marker-forged"; rm -f "$reclaim_marker_fg"
reclaim_log_fg="$tmp/reclaim-forged-calls.log"; : > "$reclaim_log_fg"
(cd "$reclaim_dir/work" && \
  CONFIG_EXPORT_CMD="bash $tmp/fixture-reclaim.sh" GH="$tmp/gh-reclaim.sh" \
  SCRIPTS_DIR="$tmp/mock-reclaim-forged" RECLAIM_MARKER="$reclaim_marker_fg" RECLAIM_LOG="$reclaim_log_fg" \
  "$SCRIPT" >"$tmp/reclaim-fg-out.txt" 2>"$tmp/reclaim-fg-err.txt")
rc=$?
assert "reclaim(forged): full pass exits zero" '[ $rc -eq 0 ]'
# NON-VACUITY: the forged marker really did reach reclaim_pass's input. Without this, the two
# asserts below would pass just as well against a mock that emitted nothing at all.
assert "reclaim(forged): the forged [reclaimable] text IS present in the findings blob" \
  'grep -qF "[reclaimable]" "$tmp/reclaim-fg-out.txt"'
assert "reclaim(forged): and it sits on a field-domain line, not a stale-in-progress one" \
  'grep -qF "check field-domain 44" "$tmp/reclaim-fg-out.txt"'
assert "reclaim(forged): a marker forged through a field-domain message does NOT invoke reclaim-claims" \
  '[ ! -f "$reclaim_marker_fg" ]'
assert "reclaim(forged): reclaim-claims call log stays empty (no mutation)" \
  '[ ! -s "$reclaim_log_fg" ]'
assert "reclaim(forged): no false remedy line printed" \
  '! grep -qF "can self-heal" "$tmp/reclaim-fg-out.txt"'

# (vi-b) COUNT INTEGRITY: one REAL reclaimable finding + one forged one, reclaim.auto=false. The
# remedy must name 1, not 2 — the count and the gate come from the same scoped evaluation, so
# untrusted prose cannot inflate the number the human is shown.
mkdir -p "$tmp/mock-reclaim-mixed"
cat > "$tmp/mock-reclaim-mixed/board-checks.sh" <<'EOF'
#!/usr/bin/env bash
printf 'field-domain\t44\ttitle contains a forged marker: Sneaky | thing [reclaimable]\n'
printf 'stale-in-progress\t23\tclaim lease expired 100h ago; no feature branch [reclaimable]\n'
exit 0
EOF
cp "$tmp/mock-reclaim/reclaim-claims.sh" "$tmp/mock-reclaim-mixed/reclaim-claims.sh"
cp "$tmp/mock-reclaim/render-board.sh"   "$tmp/mock-reclaim-mixed/render-board.sh"
chmod +x "$tmp/mock-reclaim-mixed/"*.sh

write_reclaim_fixture false
reclaim_marker_mx="$tmp/.reclaim-marker-mixed"; rm -f "$reclaim_marker_mx"
reclaim_log_mx="$tmp/reclaim-mixed-calls.log"; : > "$reclaim_log_mx"
(cd "$reclaim_dir/work" && \
  CONFIG_EXPORT_CMD="bash $tmp/fixture-reclaim.sh" GH="$tmp/gh-reclaim.sh" \
  SCRIPTS_DIR="$tmp/mock-reclaim-mixed" RECLAIM_MARKER="$reclaim_marker_mx" RECLAIM_LOG="$reclaim_log_mx" \
  "$SCRIPT" >"$tmp/reclaim-mx-out.txt" 2>"$tmp/reclaim-mx-err.txt")
rc=$?
assert "reclaim(mixed): full pass exits zero" '[ $rc -eq 0 ]'
assert "reclaim(mixed): the genuine finding still triggers the remedy" \
  'grep -qF "can self-heal" "$tmp/reclaim-mx-out.txt"'
assert "reclaim(mixed): the forged marker does NOT inflate the count (1, not 2)" \
  'grep -qF "reclaim: 1 expired-lease change(s) can self-heal" "$tmp/reclaim-mx-out.txt"'
assert "reclaim(mixed): the forged line reached the report (non-vacuity)" \
  'grep -qF "check field-domain 44" "$tmp/reclaim-mx-out.txt"'

# ── change 0094: --digest-only, the write-free selection read ────────────────────────────────
# The digest is how docket-implement-next Step 1 acquires its ordered candidate set. It is a READ:
# it must not sync, commit, push, render the board, or move HEAD. --board-only was not reusable —
# it commits and pushes BOARD.md, and a selection read must not be a write.

dg="$tmp/digest-only"; mkdir -p "$dg/work/.docket/docs/changes/active" "$dg/work/.docket/docs/changes/archive"
cat > "$dg/work/.docket/docs/changes/active/0050-tango.md" <<'EOF'
---
id: 50
slug: tango
title: tango
status: proposed
priority: high
created: 2026-02-01
updated: 2026-02-01
depends_on: []
spec: docs/superpowers/specs/t.md
trivial: false
---

## Why
x
EOF
cat > "$dg/work/.docket/docs/changes/active/0051-uniform.md" <<'EOF'
---
id: 51
slug: uniform
title: uniform
status: proposed
priority: critical
created: 2026-03-01
updated: 2026-03-01
depends_on: []
spec: docs/superpowers/specs/u.md
trivial: false
---

## Why
x
EOF

cat > "$tmp/fixture-digest.sh" <<'EOF'
#!/usr/bin/env bash
# METADATA_WORKTREE is RELATIVE (`.docket`) — matching what the REAL docket-config.sh --export
# actually emits on this path. FORMAT=shell is the default (--export alone never selects `plain`),
# and docket-config.sh's absolutization block is INSIDE `if [ "$FORMAT" = plain ]` — its own
# comment says "relative for shell ...; absolute for plain". So production --digest-only always
# receives a RELATIVE METADATA_WORKTREE, and docket_metadata_worktree's anchor helper shells out
# to `git worktree list --porcelain` (a READ-ONLY call) to resolve it — exactly as it does on
# every other docket-status.sh path. That read-only anchoring call is expected and fine; what the
# write-free contract actually forbids is MUTATION (fetch/pull/push/commit/worktree add/checkout)
# and moving HEAD — see the git-call-log assert below, which checks for that property directly
# rather than "no git call at all" (which no real invocation of this path ever satisfies: even
# `docket-config.sh --export` itself runs an unconditional `git fetch` + `git remote set-head`).
printf '%s\n' \
  'BOOTSTRAP=PROCEED' \
  'METADATA_BRANCH=docket' \
  'INTEGRATION_BRANCH=main' \
  'DOCKET_MODE=docket' \
  'METADATA_WORKTREE=.docket' \
  'CHANGES_DIR=docs/changes' \
  'ADRS_DIR=docs/adrs' \
  'RESULTS_DIR=docs/results' \
  'BOARD_SURFACES=inline'
EOF

# A git stub that RECORDS every invocation, so "it never synced" is proven by evidence rather than
# by the absence of a visible symptom.
cat > "$tmp/spy-git.sh" <<EOF
#!/usr/bin/env bash
printf '%s\n' "\$*" >> "$tmp/git-calls.txt"
exit 0
EOF
chmod +x "$tmp/spy-git.sh"
: > "$tmp/git-calls.txt"

# Minor 5 (0094 whole-branch review): pre-write BOARD.md with KNOWN bytes before the run. The old
# assert `[ ! -e BOARD.md ]` was already true before the run even started (this fixture never
# creates one) — it proved "not created", not the spec's "byte-unchanged". Writing a sentinel file
# first and cmp'ing it after is the only way to actually exercise the "left untouched" claim.
printf 'SENTINEL — pre-existing BOARD.md bytes; --digest-only must never touch this file\n' \
  > "$dg/work/.docket/docs/changes/BOARD.md"
cp "$dg/work/.docket/docs/changes/BOARD.md" "$tmp/dg-board-before.md"

dg_out="$tmp/digest-only-out.txt"
(cd "$dg/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-digest.sh" GIT="$tmp/spy-git.sh" \
  "$SCRIPT" --digest-only >"$dg_out" 2>"$tmp/digest-only-err.txt")
dgrc=$?

assert "--digest-only exits 0" '[ "$dgrc" -eq 0 ]'
assert "--digest-only emits the ready line"    'grep -qE "^ready( [0-9]+)*$" "$dg_out"'
assert "--digest-only emits change lines"      'grep -q "^change 50 proposed build-ready tango" "$dg_out"'
assert "--digest-only emits backlog rollups"   'grep -q "^backlog proposed 2" "$dg_out"'
assert "--digest-only ready order is critical-first (51 before 50)" \
  '[ "$(sed -n "s/^ready //p" "$dg_out")" = "51 50" ]'

# The load-bearing half: it is a READ. A read-only `worktree list` call (anchoring the relative
# METADATA_WORKTREE, same as every other docket-status.sh path) is expected and fine — what the
# write-free contract forbids is a call that MUTATES the working tree or moves refs/HEAD (pull,
# push, commit, worktree add, checkout), not every git invocation whatsoever. Review fix 4 (0094
# whole-branch review): the assert below used to also list "fetch" among the forbidden calls, which
# overstates what this path actually guarantees — the REAL docket-config.sh --export (bypassed here
# by CONFIG_EXPORT_CMD's stub) runs an unconditional, read-only `git fetch --quiet origin` +
# `git remote set-head origin -a` on every invocation, production included. That is fine: neither
# call moves HEAD or touches the working tree, so the design intent (no mutation) still holds —
# but a name claiming "no fetch" would describe a property this path's production invocation does
# not actually have. Renamed/scoped to the narrower, true claim.
assert "--digest-only emits NO board line" '! grep -q "^board " "$dg_out"'
assert "--digest-only leaves BOARD.md byte-unchanged (not merely 'not created')" \
  'cmp -s "$tmp/dg-board-before.md" "$dg/work/.docket/docs/changes/BOARD.md"'
assert "--digest-only makes no working-tree- or ref-mutating git call (no pull/push/commit/worktree add/checkout — fetch/set-head are permitted, read-only)" \
  '! grep -Eq "(^| )(pull|push|commit|checkout)( |$)" "$tmp/git-calls.txt" \
   && ! grep -qF "worktree add" "$tmp/git-calls.txt"'
assert "the git-call log is not vacuous (the read-only anchoring call was actually made)" \
  '[ -s "$tmp/git-calls.txt" ] && grep -qF "worktree list" "$tmp/git-calls.txt"'
assert "--digest-only emits no pass ok (it is not a pass)" '! grep -q "^pass ok" "$dg_out"'
assert "--digest-only stdout is non-empty" '[ -s "$dg_out" ]'

# Minor 5 continued: the spec also promises "working tree clean, HEAD unmoved" — properties the
# spy-git stub above cannot attest to (it isn't real git). Prove them with a REAL git repo: seed
# one change, commit + push a known-bytes BOARD.md, capture HEAD, run --digest-only against it with
# the real `git` (no GIT= override), and assert both invariants plus the same byte-unchanged check.
dgb="$tmp/digest-only-realgit"
git_repo_setup "$dgb/case"
git clone -q "$dgb/case/origin.git" "$dgb/work" 2>/dev/null
mkdir -p "$dgb/work/docs/changes/active" "$dgb/work/docs/changes/archive"
cat > "$dgb/work/docs/changes/active/0060-victor.md" <<'EOF'
---
id: 60
slug: victor
title: victor
status: proposed
priority: medium
created: 2026-04-01
updated: 2026-04-01
depends_on: []
spec: docs/superpowers/specs/v.md
trivial: false
---

## Why
x
EOF
printf 'SENTINEL — pre-existing BOARD.md bytes (real-git fixture); must survive untouched\n' \
  > "$dgb/work/docs/changes/BOARD.md"
git -C "$dgb/work" -c user.email=t@t -c user.name=t add docs/changes
git -C "$dgb/work" -c user.email=t@t -c user.name=t commit -q -m "seed digest-only real-git fixture"
git -C "$dgb/work" push -q origin main
cp "$dgb/work/docs/changes/BOARD.md" "$tmp/dgb-board-before.md"
dgb_head_before="$(git -C "$dgb/work" rev-parse HEAD)"

cat > "$tmp/fixture-digest-realgit.sh" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' \
  'BOOTSTRAP=PROCEED' \
  'METADATA_BRANCH=main' \
  'INTEGRATION_BRANCH=main' \
  'DOCKET_MODE=main' \
  'METADATA_WORKTREE=.' \
  'CHANGES_DIR=docs/changes' \
  'ADRS_DIR=docs/adrs' \
  'RESULTS_DIR=docs/results' \
  'BOARD_SURFACES=inline'
EOF

(cd "$dgb/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-digest-realgit.sh" \
  "$SCRIPT" --digest-only >"$tmp/dg-realgit-out.txt" 2>"$tmp/dg-realgit-err.txt")
dgbrc=$?
assert "--digest-only (real-git fixture) exits 0" '[ "$dgbrc" -eq 0 ]'
assert "--digest-only (real-git fixture) emits the ready line" \
  'grep -qF "change 60 proposed build-ready victor" "$tmp/dg-realgit-out.txt"'
assert "--digest-only (real-git fixture) leaves BOARD.md byte-unchanged" \
  'cmp -s "$tmp/dgb-board-before.md" "$dgb/work/docs/changes/BOARD.md"'
assert "--digest-only (real-git fixture) leaves the working tree clean" \
  '[ -z "$(git -C "$dgb/work" status --porcelain)" ]'
assert "--digest-only (real-git fixture) leaves HEAD unmoved" \
  '[ "$(git -C "$dgb/work" rev-parse HEAD)" = "$dgb_head_before" ]'

# ── review fix 3 (0094 whole-branch review): the happy-path battery never exercises real path
# anchoring from a SUBDIRECTORY ───────────────────────────────────────────────────────────────
# Every fixture above (the spy-git battery AND the real-git fixture just above) invokes
# docket-status.sh from the fixture's OWN work dir. With the spy-git stub, docket_main_worktree's
# `worktree list` call returns nothing (the stub isn't a real repo), so docket_anchor_path takes
# its soft "not a repo" fallback and METADATA_WORKTREE stays relative (`.docket`) — those asserts
# pass only because CWD happens to equal the relative base; the SAME fixture run from a
# subdirectory would resolve `.docket/docs/changes` against the wrong root and fail with "metadata
# worktree not found", rc=1. --digest-only is the one path that deliberately skips
# docket_preflight's own anchoring (see the header comment on digest_only_pass), so it is the path
# that most needs a REAL anchoring check. Reuse the real-git fixture above ($dgb, real `git`, no
# GIT= override, METADATA_WORKTREE=.) but invoke --digest-only from a directory nested several
# levels under the work tree — proving `git worktree list --porcelain` (docket_main_worktree's
# resolution call) finds the repo root regardless of CWD, not merely when CWD happens to equal the
# relative base.
mkdir -p "$dgb/work/nested/deeper/cwd-probe"
(cd "$dgb/work/nested/deeper/cwd-probe" && CONFIG_EXPORT_CMD="bash $tmp/fixture-digest-realgit.sh" \
  "$SCRIPT" --digest-only >"$tmp/dg-subdir-out.txt" 2>"$tmp/dg-subdir-err.txt")
dgsubrc=$?
assert "--digest-only resolves from a subdirectory of the work tree (real git anchoring)" \
  '[ "$dgsubrc" -eq 0 ]'
assert "--digest-only from a subdirectory still emits the ready line" \
  'grep -qF "change 60 proposed build-ready victor" "$tmp/dg-subdir-out.txt"'
assert "--digest-only from a subdirectory leaves BOARD.md byte-unchanged" \
  'cmp -s "$tmp/dgb-board-before.md" "$dgb/work/docs/changes/BOARD.md"'

# Mutual exclusion: the two flags are opposite postures (a read vs. a committing write).
(cd "$dg/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-digest.sh" GIT="$tmp/spy-git.sh" \
  "$SCRIPT" --digest-only --board-only >/dev/null 2>"$tmp/dg-both-err.txt")
bothrc=$?
assert "--digest-only with --board-only exits 2" '[ "$bothrc" -eq 2 ]'
assert "the mutual-exclusion error names both flags" \
  'grep -q -- "--digest-only" "$tmp/dg-both-err.txt" && grep -q -- "--board-only" "$tmp/dg-both-err.txt"'
# Order-independence: a flag pair that only rejects in one order is a half-closed gate.
# NB: capture the status into a NAMED variable. `assert ... '[ "$?" -eq 2 ]'` would evaluate `$?`
# inside assert's own eval, where it reports the previous command in THAT scope — the assert would
# be measuring itself and would pass for the wrong reason.
(cd "$dg/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-digest.sh" GIT="$tmp/spy-git.sh" \
  "$SCRIPT" --board-only --digest-only >/dev/null 2>/dev/null)
revrc=$?
assert "--board-only with --digest-only also exits 2 (order-independent)" '[ "$revrc" -eq 2 ]'

# ── review fix 4: --must-land is silently ignored under --digest-only ────────────────────────
# --must-land is documented as meaningful only "(with --board-only)" — it retries/maps the exit
# code of the board pass. main() short-circuits --digest-only BEFORE MUST_LAND is ever read, so
# without a gate, `--digest-only --must-land` exits 0 and prints the digest with no diagnostic at
# all — the flag is silently dropped. Extend the existing --digest-only/--board-only gate rather
# than inventing a separate mechanism (both orders, matching that gate's own discipline).
(cd "$dg/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-digest.sh" GIT="$tmp/spy-git.sh" \
  "$SCRIPT" --digest-only --must-land >/dev/null 2>"$tmp/dg-ml-err.txt")
dgmlrc=$?
assert "--digest-only with --must-land exits 2 (silently-ignored flag combo)" '[ "$dgmlrc" -eq 2 ]'
assert "the --digest-only/--must-land error names both flags" \
  'grep -q -- "--digest-only" "$tmp/dg-ml-err.txt" && grep -q -- "--must-land" "$tmp/dg-ml-err.txt"'
(cd "$dg/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-digest.sh" GIT="$tmp/spy-git.sh" \
  "$SCRIPT" --must-land --digest-only >/dev/null 2>/dev/null)
mldgrc=$?
assert "--must-land with --digest-only also exits 2 (order-independent)" '[ "$mldgrc" -eq 2 ]'

# ── review fix 5 (0094 whole-branch review): --repo/--project/--auto-create-project/
# --project-owner are silently dropped under --digest-only, same shape as --must-land above ──────
# --must-land got a hard exit-2 gate precisely because a silently-dropped flag is a defect shape;
# --repo, --project, --auto-create-project, and --project-owner are dropped by the SAME
# short-circuit (main() never reaches board_pass/github-mirror.sh under --digest-only) and nothing
# on the digest path consumes any of them. Extend the existing gate so each one exits 2 and names
# itself on stderr, rather than silently vanishing.
(cd "$dg/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-digest.sh" GIT="$tmp/spy-git.sh" \
  "$SCRIPT" --digest-only --repo owner/repo >/dev/null 2>"$tmp/dg-repo-err.txt")
dgrepork=$?
assert "--digest-only with --repo exits 2 (silently-ignored flag combo)" '[ "$dgrepork" -eq 2 ]'
assert "the --digest-only/--repo error names both flags" \
  'grep -q -- "--digest-only" "$tmp/dg-repo-err.txt" && grep -q -- "--repo" "$tmp/dg-repo-err.txt"'

(cd "$dg/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-digest.sh" GIT="$tmp/spy-git.sh" \
  "$SCRIPT" --digest-only --project owner/1 >/dev/null 2>"$tmp/dg-project-err.txt")
dgprojrc=$?
assert "--digest-only with --project exits 2 (silently-ignored flag combo)" '[ "$dgprojrc" -eq 2 ]'
assert "the --digest-only/--project error names both flags" \
  'grep -q -- "--digest-only" "$tmp/dg-project-err.txt" && grep -q -- "--project" "$tmp/dg-project-err.txt"'

(cd "$dg/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-digest.sh" GIT="$tmp/spy-git.sh" \
  "$SCRIPT" --digest-only --auto-create-project >/dev/null 2>"$tmp/dg-acp-err.txt")
dgacprc=$?
assert "--digest-only with --auto-create-project exits 2 (silently-ignored flag combo)" '[ "$dgacprc" -eq 2 ]'
assert "the --digest-only/--auto-create-project error names both flags" \
  'grep -q -- "--digest-only" "$tmp/dg-acp-err.txt" && grep -q -- "--auto-create-project" "$tmp/dg-acp-err.txt"'

(cd "$dg/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-digest.sh" GIT="$tmp/spy-git.sh" \
  "$SCRIPT" --digest-only --project-owner acme >/dev/null 2>"$tmp/dg-po-err.txt")
dgpork=$?
assert "--digest-only with --project-owner exits 2 (silently-ignored flag combo)" '[ "$dgpork" -eq 2 ]'
assert "the --digest-only/--project-owner error names both flags" \
  'grep -q -- "--digest-only" "$tmp/dg-po-err.txt" && grep -q -- "--project-owner" "$tmp/dg-po-err.txt"'

# Totality on an EMPTY backlog: stdout is still non-empty and the ready line is still there.
dge="$tmp/digest-empty"; mkdir -p "$dge/work/.docket/docs/changes/active" "$dge/work/.docket/docs/changes/archive"
dge_out="$tmp/digest-empty-out.txt"
(cd "$dge/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-digest.sh" GIT="$tmp/spy-git.sh" \
  "$SCRIPT" --digest-only >"$dge_out" 2>/dev/null)
assert "--digest-only on an empty backlog still emits a bare ready line" \
  '[ "$(cat "$dge_out")" = "ready" ]'

# Bootstrap stays fail-closed on this path too — a read must not silently report an empty backlog
# for a repo that was never migrated. METADATA_WORKTREE is relative here too (`.docket`),
# consistent with fixture-digest.sh above — both fixtures model the SAME real shell-format export
# (change 0068's absolutization applies only to `--format plain`, never to this file's `.sh`
# `--export` shape). This BOOTSTRAP verdict is checked before METADATA_WORKTREE is ever resolved,
# so the value never actually matters for this fixture — kept relative anyway so a reader
# skimming the two side by side sees one convention, not two.
cat > "$tmp/fixture-digest-stop.sh" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' \
  'BOOTSTRAP=STOP_MIGRATE' \
  'METADATA_BRANCH=docket' \
  'INTEGRATION_BRANCH=main' \
  'DOCKET_MODE=docket' \
  'METADATA_WORKTREE=.docket' \
  'CHANGES_DIR=docs/changes' \
  'ADRS_DIR=docs/adrs' \
  'RESULTS_DIR=docs/results' \
  'BOARD_SURFACES=inline'
EOF
(cd "$dg/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-digest-stop.sh" GIT="$tmp/spy-git.sh" \
  "$SCRIPT" --digest-only >"$tmp/dg-stop-out.txt" 2>"$tmp/dg-stop-err.txt")
stoprc=$?
assert "--digest-only is fail-closed on a non-PROCEED bootstrap verdict" '[ "$stoprc" -ne 0 ]'
assert "--digest-only emits no ready line when the bootstrap gate rejects" \
  '! grep -q "^ready" "$tmp/dg-stop-out.txt"'

# ── review fix 1: --digest-only is fail-OPEN when the metadata worktree is missing ───────────
# Reachable scenario: a fresh clone of an already-migrated repo. origin/docket exists, so
# BOOTSTRAP=PROCEED — but .docket/ is gitignored and no `worktree add` has ever run in THIS
# clone. digest_only_pass deliberately skips docket_preflight (the one thing that would create
# it), and backlog_pass is best-effort (a render-board.sh failure logs to stderr and `return 0`s).
# Today that means: exit 0, empty stdout, no `ready` line at all — a selector keying on `rc==0`
# gets success-plus-nothing, the exact two-cases-one-signal collapse the always-emitted `ready`
# line exists to prevent. A real (not stubbed) git repo, so docket_metadata_worktree's anchor
# helper resolves an ACTUAL absolute root the way it would in production.
dgnw="$tmp/digest-noworktree"
git_repo_setup "$dgnw/case"
git clone -q "$dgnw/case/origin.git" "$dgnw/work" 2>/dev/null
cat > "$tmp/fixture-digest-noworktree.sh" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' \
  'BOOTSTRAP=PROCEED' \
  'METADATA_BRANCH=docket' \
  'INTEGRATION_BRANCH=main' \
  'DOCKET_MODE=docket' \
  'METADATA_WORKTREE=.docket' \
  'CHANGES_DIR=docs/changes' \
  'ADRS_DIR=docs/adrs' \
  'RESULTS_DIR=docs/results' \
  'BOARD_SURFACES=inline'
EOF
assert "fresh-clone fixture: metadata worktree genuinely absent before the run" \
  '[ ! -d "$dgnw/work/.docket" ]'
(cd "$dgnw/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-digest-noworktree.sh" \
  "$SCRIPT" --digest-only >"$tmp/dg-nw-out.txt" 2>"$tmp/dg-nw-err.txt")
dgnwrc=$?
assert "--digest-only fails closed when the metadata worktree does not exist (fresh clone)" \
  '[ "$dgnwrc" -ne 0 ]'
assert "--digest-only emits no ready line when the metadata worktree is missing" \
  '! grep -q "^ready" "$tmp/dg-nw-out.txt"'
assert "--digest-only stdout is empty when the metadata worktree is missing (no digest emitted)" \
  '[ ! -s "$tmp/dg-nw-out.txt" ]'
assert "--digest-only names the resolved changes-dir path on stderr" \
  'grep -qF ".docket/docs/changes" "$tmp/dg-nw-err.txt"'
assert "--digest-only points the caller at docket.sh preflight" \
  'grep -qi "preflight" "$tmp/dg-nw-err.txt"'

# ── Important 1 (0094 whole-branch review): --digest-only was fail-OPEN on a render failure ────
# digest_only_pass's existence check guards ONLY the missing-changes-dir case; the call it then
# makes, backlog_pass, is best-effort BY DESIGN (a render failure logs to stderr and `return 0`s).
# So every OTHER render failure — a partially-installed or non-executable render-board.sh — still
# produced exit 0 with COMPLETELY EMPTY stdout on this path: no `ready` line, no diagnostic reaching
# the caller via the exit code, the exact "two-cases-one-signal collapse" digest_only_pass's own
# header comment claims to prevent. Reproduce with a stub SCRIPTS_DIR/render-board.sh that exits 1 —
# mirroring the --board-only stub-render fixture above (the "failing digest still exits 0" block).
dgrf="$tmp/digest-only-renderfail"
mkdir -p "$dgrf/work/.docket/docs/changes/active" "$dgrf/work/.docket/docs/changes/archive"
cat > "$dgrf/work/.docket/docs/changes/active/0070-whiskey.md" <<'EOF'
---
id: 70
slug: whiskey
title: whiskey
status: proposed
priority: medium
created: 2026-05-01
updated: 2026-05-01
depends_on: []
spec: docs/superpowers/specs/w.md
trivial: false
---

## Why
x
EOF
mkdir -p "$tmp/stub-scripts-digest-renderfail"
cat > "$tmp/stub-scripts-digest-renderfail/render-board.sh" <<'EOF'
#!/usr/bin/env bash
echo "stub render-board: boom" >&2
exit 1
EOF
chmod +x "$tmp/stub-scripts-digest-renderfail/render-board.sh"

(cd "$dgrf/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-digest.sh" GIT="$tmp/spy-git.sh" \
  SCRIPTS_DIR="$tmp/stub-scripts-digest-renderfail" \
  "$SCRIPT" --digest-only >"$tmp/dg-renderfail-out.txt" 2>"$tmp/dg-renderfail-err.txt")
dgrfrc=$?
assert "--digest-only fails closed when the digest render itself fails (Important 1)" \
  '[ "$dgrfrc" -ne 0 ]'
assert "--digest-only emits no ready line on a render failure" \
  '! grep -q "^ready" "$tmp/dg-renderfail-out.txt"'
assert "--digest-only stdout is empty on a render failure (no partial/garbled digest)" \
  '[ ! -s "$tmp/dg-renderfail-out.txt" ]'
assert "--digest-only emits a diagnostic on stderr for a render failure" \
  'grep -qi "digest" "$tmp/dg-renderfail-err.txt"'

# --help documents the flag (the skill's author has to be able to find it).
assert "--help mentions --digest-only" '"$SCRIPT" --help 2>&1 | grep -q -- "--digest-only"'

exit $fail
