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

exit $fail
