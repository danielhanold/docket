#!/usr/bin/env bash
# tests/test_docket_status.sh — verifies change 0058: the docket-status orchestrator.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$REPO/scripts/docket-status.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

assert "script exists and is executable" '[ -x "$SCRIPT" ]'
assert "--help exits 0 and prints usage" '"$SCRIPT" --help 2>&1 | grep -qi "usage"'

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

# Bootstrap gate: stub docket-config.sh --export via CONFIG_EXPORT_CMD (a hermetic fixture
# script emitting the eval-able KEY=value block), and assert the gate's exit code + remedy text.
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT

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
  cat > "$tmp/fixture-board.sh" <<EOF
#!/usr/bin/env bash
printf '%s\n' \
  'BOOTSTRAP=PROCEED' \
  'METADATA_BRANCH=main' \
  'INTEGRATION_BRANCH=main' \
  'DOCKET_MODE=main' \
  'METADATA_WORKTREE=.docket' \
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

write_board_fixture ""
(cd "$tmp/board-case/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-board.sh" "$SCRIPT" --board-only >"$tmp/board-run3.txt" 2>"$tmp/board-run3-err.txt")
rc=$?
assert "board_pass empty-surfaces run exits zero" '[ $rc -eq 0 ]'
# Change 0069: silence is not evidence. A board-off pass must SAY the board is off — an empty
# stdout is indistinguishable from "the script silently did nothing", which is the exact
# confusion that made an agent hunt for a BOARD.md its config forbids.
assert "board_pass empty-surfaces emits a positive 'board off' line" \
  'grep -qxF "board off" "$tmp/board-run3.txt"'
assert "board_pass empty-surfaces emits no inline board line" \
  '! grep -q "board inline" "$tmp/board-run3.txt"'

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
if [ "\$1" = pull ] && [ ! -f "\$raced" ]; then
  git "\$@"; rc=\$?
  touch "\$raced"
  git -C "$tmp/conflict-case/work2" push -q origin main
  exit \$rc
fi
exec git "\$@"
EOF
chmod +x "$tmp/git-race.sh"

write_board_fixture inline
(cd "$tmp/conflict-case/work" && CONFIG_EXPORT_CMD="bash $tmp/fixture-board.sh" GIT="$tmp/git-race.sh" "$SCRIPT" --board-only >"$tmp/conflict-run.txt" 2>"$tmp/conflict-run-err.txt")
rc=$?
assert "conflict run exits zero" '[ $rc -eq 0 ]'
assert "conflict run reports inline changed pushed or push-failed" 'grep -Eq "board inline changed (pushed|push-failed)" "$tmp/conflict-run.txt"'
assert "conflict run: BOARD.md non-empty after run" '[ -s "$tmp/conflict-case/work/docs/changes/BOARD.md" ]'
if grep -q "board inline changed pushed" "$tmp/conflict-run.txt"; then
  assert "conflict run pushed: local BOARD.md carries both merged changes" \
    'grep -q "beta" "$tmp/conflict-case/work/docs/changes/BOARD.md" && grep -q "Alpha feature v2" "$tmp/conflict-case/work/docs/changes/BOARD.md"'
  assert "conflict run pushed: remote BOARD.md matches local" \
    'git -C "$tmp/conflict-case/work" show origin/main:docs/changes/BOARD.md 2>/dev/null | cmp -s - "$tmp/conflict-case/work/docs/changes/BOARD.md"'
fi

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
printf '20\tclean-thing\t20\t2026-07-08\n21\tbroken-render\t21\t2026-07-09\n22\talready-done\t22\t2026-07-05\n23\tcleanup-broken\t23\t2026-07-10\n' > "$sweep_input"

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
  'BOARD_SURFACES=' \
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

# Case B: TERMINAL_PUBLISH entirely UNSET by the config mock (not merely "true") — reproduces the
# exact hazard the fix guards against: sweep_execute_one runs under `set -u`, so a bare
# $TERMINAL_PUBLISH would abort the sweep with an unbound-variable error under a stale/mocked
# config export that doesn't emit the key. "${TERMINAL_PUBLISH:-true}" must default to enabled
# instead, matching pre-0064 behavior.
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
  'BOARD_SURFACES='
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
assert "0064 gate(TERMINAL_PUBLISH unset): defaults to enabled — archived record DOES reach the integration branch" \
  'git -C "$gate_dir2/work" ls-tree -r --name-only origin/main | grep -q "docs/changes/archive/2026-07-11-0060-gate-thing.md"'

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
# BOARD_SURFACES="" to skip the board pass entirely (already covered above).
write_full_fixture(){
  # $1 board_surfaces (usually empty)
  cat > "$tmp/fixture-full.sh" <<EOF
#!/usr/bin/env bash
printf '%s\n' \
  'BOOTSTRAP=PROCEED' \
  'METADATA_BRANCH=main' \
  'INTEGRATION_BRANCH=main' \
  'DOCKET_MODE=main' \
  'METADATA_WORKTREE=.docket' \
  'CHANGES_DIR=docs/changes' \
  'ADRS_DIR=docs/adrs' \
  'RESULTS_DIR=docs/results' \
  'BOARD_SURFACES=$1'
EOF
}
write_full_fixture ""

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

write_full_fixture ""
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

write_full_fixture ""
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
assert "SKILL invokes docket-status.sh" 'grep -qF "/docket-status.sh" "$SKILL"'
assert "SKILL no longer inlines the sweep loop enumeration" \
  '! grep -qF "For each \`implemented\` change:" "$SKILL"'

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
#   1. UNGATED: with BOARD_SURFACES="" the full pass still emits the digest and `pass ok`.
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

write_full_fixture ""
(cd "$fp_dir/work" && \
  CONFIG_EXPORT_CMD="bash $tmp/fixture-full.sh" GH="$tmp/gh-fullpass.sh" \
  SCRIPTS_DIR="$tmp/mock-real" \
  "$SCRIPT" --repo x/y >"$tmp/fullpass-out.txt" 2>"$tmp/fullpass-err.txt")
rc=$?
assert "full pass (real renderer) exits zero" '[ $rc -eq 0 ]'
assert "full pass swept the merged change" 'grep -qxF "swept 60 2026-07-11" "$tmp/fullpass-out.txt"'
# (1) ungated: the digest and `pass ok` reach the FULL path, board off and all.
assert "full pass emits 'board off' (board_surfaces empty)" \
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

exit $fail
