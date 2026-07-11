#!/usr/bin/env bash
# tests/test_docket_status.sh — verifies change 0058: the docket-status orchestrator.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$REPO/scripts/docket-status.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

assert "script exists and is executable" '[ -x "$SCRIPT" ]'
assert "--help exits 0 and prints usage" '"$SCRIPT" --help 2>&1 | grep -qi "usage"'

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
assert "board_pass empty-surfaces emits no board line" '! grep -qw "board" "$tmp/board-run3.txt"'

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
{"data":{"p10":{"number":101,"mergedAt":"2026-07-05T18:22:31Z","state":"MERGED"},"p11":{"number":102,"mergedAt":null,"state":"OPEN"}}}
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

exit $fail
