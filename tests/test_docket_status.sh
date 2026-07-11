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

exit $fail
