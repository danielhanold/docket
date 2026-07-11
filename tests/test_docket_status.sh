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

exit $fail
