#!/usr/bin/env bash
# tests/test_github_mirror.sh — verifies change 0011: the deterministic GitHub board-mirror
# script. Exercises COMMAND CONSTRUCTION via --dry-run + a mock `gh` (no live GitHub — the
# suite only ever sees the integration-branch checkout; live behavior is verified at build time
# and recorded in the results file, per LEARNINGS).
# Run: bash tests/test_github_mirror.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$REPO/scripts/github-mirror.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# --- fixture: a temp changes_dir with active/ + archive/ holding representative changes ---
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/active" "$tmp/archive"

cat > "$tmp/active/0011-github-issues-board-mirror.md" <<'EOF'
---
id: 11
slug: github-issues-board-mirror
title: GitHub board mirror — selectable surfaces
status: in-progress
priority: medium
depends_on: []
related: [4, 10]
adrs: []
spec: docs/superpowers/specs/2026-06-14-github-issues-board-mirror-design.md
plan: docs/superpowers/plans/2026-06-14-github-issues-board-mirror.md
issue:
EOF

cat > "$tmp/active/0009-existing.md" <<'EOF'
---
id: 9
slug: existing
title: Already mirrored change
status: proposed
priority: high
depends_on: []
adrs: []
spec:
issue: 142
EOF

cat > "$tmp/archive/2026-06-12-0006-donezo.md" <<'EOF'
---
id: 6
slug: donezo
title: A finished change
status: done
priority: medium
depends_on: []
adrs: []
issue: 88
EOF

cat > "$tmp/archive/2026-06-12-0005-killed.md" <<'EOF'
---
id: 5
slug: killed
title: An abandoned change
status: killed
priority: low
depends_on: []
adrs: []
issue: 77
EOF

cat > "$tmp/active/0013-target.md" <<'EOF'
---
id: 13
slug: target
title: Target change (implemented)
status: implemented
priority: medium
depends_on: []
adrs: []
issue: 200
EOF

cat > "$tmp/active/0012-waiter.md" <<'EOF'
---
id: 12
slug: waiter
title: Waiter change (proposed, depends on implemented)
status: proposed
priority: medium
depends_on: [13]
adrs: []
spec:
issue: 201
EOF

# --- mock gh: records argv and fakes `issue create` returning a URL ---
mock="$tmp/bin"; mkdir -p "$mock"
cat > "$mock/gh" <<'EOF'
#!/usr/bin/env bash
echo "MOCKGH $*" >> "$GH_LOG"
if [ "$1" = "issue" ] && [ "$2" = "create" ]; then
  echo "https://github.com/o/r/issues/4242"
fi
exit 0
EOF
chmod +x "$mock/gh"

assert "script exists and is executable" '[ -x "$SCRIPT" ]'

# === A. DRY-RUN command construction =========================================
out="$(bash "$SCRIPT" --dry-run --changes-dir "$tmp" --repo o/r --metadata-branch docket \
        --changes-path docs/changes 2>&1)"

assert "creates an issue for a change with empty issue:" \
  'echo "$out" | grep -qE "issue create"'
assert "edits (not creates) a change that already has issue: 142" \
  'echo "$out" | grep -qE "issue edit 142"'
assert "does NOT create a second issue for the already-mirrored change (only one create)" \
  '[ "$(echo "$out" | grep -cE "issue create")" -eq 1 ]'

# status -> close reason mapping
assert "done change closes as completed" \
  'echo "$out" | grep -qE "issue close 88 .*--reason completed"'
assert "killed change closes as not planned" \
  'echo "$out" | grep -qE "issue close 77 .*--reason (not.planned|\"not planned\")"'
assert "active (in-progress) change is not closed" \
  '! echo "$out" | grep -qE "issue (close|reopen) .*github-issues-board-mirror"'

# labels — docket: namespace only
assert "emits docket:status/ label" 'echo "$out" | grep -qF "docket:status/in-progress"'
assert "emits docket:priority/ label" 'echo "$out" | grep -qF "docket:priority/medium"'
assert "emits a needs-brainstorm readiness label for the proposed no-spec change" \
  'echo "$out" | grep -qF "docket:readiness/needs-brainstorm"'
assert "proposed change waiting on an implemented dep emits the needs-your-merge waiting label" \
  'echo "$out" | grep -qF "docket:waiting/needs-your-merge"'
assert "every mirror label is docket:-namespaced" \
  '! echo "$out" | grep -oE -- "--(add-label|label) [^ ]+" | grep -vqE "docket:|--(add-label|label)$"'

# body content
assert "issue body carries the one-way banner" \
  'echo "$out" | grep -qiF "edits and comments here are not read"'
assert "issue body links to the change file on the metadata branch" \
  'echo "$out" | grep -qF "blob/docket/docs/changes/active/0011-github-issues-board-mirror.md"'
assert "issue body links to the spec when set" \
  'echo "$out" | grep -qF "2026-06-14-github-issues-board-mirror-design.md"'

# one-way invariant: the sync never uses Closes #N
assert "script output never contains a Closes-#N auto-close directive" \
  '! echo "$out" | grep -qiE "closes #"'

# mint emission so the caller can persist issue:
assert "emits a machine-readable mint line for newly created issues" \
  'echo "$out" | grep -qE "issue-minted 11 "'

# === B. PROJECTS degradation =================================================
# No --project and no --auto-create -> Projects skipped cleanly, issues still emitted, exit 0.
bash "$SCRIPT" --dry-run --changes-dir "$tmp" --repo o/r >/dev/null 2>&1; rcB=$?
assert "exits 0 when no project is configured (Projects skipped, issues still mirrored)" \
  '[ "'$rcB'" -eq 0 ]'
noproj="$(bash "$SCRIPT" --dry-run --changes-dir "$tmp" --repo o/r 2>&1)"
assert "no project + no --auto-create-project: never mints a board" \
  '! echo "$noproj" | grep -qE "project create"'
assert "no project + no --auto-create-project: logs the skip notice" \
  'echo "$noproj" | grep -qiF "no project configured"'

# With an existing --project: link items, never create a board.
projout="$(bash "$SCRIPT" --dry-run --changes-dir "$tmp" --repo o/r --project o/7 2>&1)"
assert "with --project, adds mirrored issues to the board (item-add)" \
  'echo "$projout" | grep -qE "project item-add 7 --owner o"'
assert "with --project, links an existing issue by its URL" \
  'echo "$projout" | grep -qF "https://github.com/o/r/issues/142"'
assert "with --project, sets the item Status field (item-edit)" \
  'echo "$projout" | grep -qE "project item-edit .*--single-select-option-id"'
assert "with --project, never mints a board (only --auto-create-project does)" \
  '! echo "$projout" | grep -qE "project create"'

# === D. PROJECTS auto-create =================================================
# --auto-create-project with no --project: mint a private board, seed the field, emit project-minted.
autoout="$(bash "$SCRIPT" --dry-run --changes-dir "$tmp" --repo o/r --auto-create-project 2>&1)"
assert "auto-create mints a board under the repo owner" \
  'echo "$autoout" | grep -qE "project create --owner o"'
assert "auto-create seeds a SINGLE_SELECT Status field with the five active statuses" \
  'echo "$autoout" | grep -qE "project field-create .*--data-type SINGLE_SELECT" && echo "$autoout" | grep -qF "proposed,in-progress,blocked,deferred,implemented"'
assert "auto-create emits a machine-readable project-minted line for write-back" \
  'echo "$autoout" | grep -qE "project-minted o( |$)"'
assert "auto-create then links the mirrored issues as items" \
  'echo "$autoout" | grep -qE "project item-add DRYNUM --owner o"'
# Capture first (don't pipe the live script into grep -q: grep exits on first match, the still-
# writing script takes SIGPIPE, and pipefail would turn that 141 into a flaky failure).
ownerout="$(bash "$SCRIPT" --dry-run --changes-dir "$tmp" --repo o/r --auto-create-project --project-owner acme 2>&1)"
assert "auto-create respects --project-owner override" \
  'echo "$ownerout" | grep -qE "project create --owner acme"'
assert "terminal (done/killed) changes get no Status column value" \
  '! echo "$autoout" | grep -qE "issues/(88|77)\""'

# === E. WRONG-TREE GUARD =====================================================
# An empty active/ next to a populated archive/ = the pruned integration-branch checkout signature.
wrong="$(mktemp -d)"; mkdir -p "$wrong/active" "$wrong/archive"
cp "$tmp/archive/2026-06-12-0006-donezo.md" "$wrong/archive/"
guardout="$(bash "$SCRIPT" --dry-run --changes-dir "$wrong" --repo o/r 2>&1)"
assert "warns when active/ is empty but archive/ is populated (wrong-tree footgun)" \
  'echo "$guardout" | grep -qiE "integration-branch checkout|active.* is empty"'
assert "guard still mirrors the archived changes (best-effort, never aborts)" \
  'echo "$guardout" | grep -qE "issue edit 88"'
rm -rf "$wrong"
# A healthy tree (active/ populated) must NOT warn.
assert "no wrong-tree warning when active/ is populated" \
  '! echo "$out" | grep -qiF "integration-branch checkout"'

# === C. REAL invocation through the mock gh (idempotent, best-effort) ========
GH_LOG="$tmp/gh.log" GH="$mock/gh" bash "$SCRIPT" --changes-dir "$tmp" --repo o/r \
  --metadata-branch docket --changes-path docs/changes >/dev/null 2>&1
rc=$?
assert "real run via mock gh exits 0 (best-effort never aborts)" '[ "'$rc'" -eq 0 ]'
assert "mock gh actually received an issue create call" \
  'grep -qE "MOCKGH issue create" "$tmp/gh.log"'
assert "mock gh received the close-completed call for the done change" \
  'grep -qE "MOCKGH issue close 88 .*--reason completed" "$tmp/gh.log"'

# A change that is ALREADY terminal on its first sync (no issue: yet) must mint AND close in the
# same pass — not be left open until a later run. The mock returns issue 4242 on create.
fresh="$(mktemp -d)"; mkdir -p "$fresh/active" "$fresh/archive"
cat > "$fresh/archive/2026-06-12-0003-fresh-done.md" <<'EOF'
---
id: 3
slug: fresh-done
title: A done change never mirrored before
status: done
priority: medium
depends_on: []
issue:
EOF
GH_LOG="$fresh/gh.log" GH="$mock/gh" bash "$SCRIPT" --changes-dir "$fresh" --repo o/r >/dev/null 2>&1
assert "a freshly-minted done change is created AND closed-completed in the same pass" \
  'grep -qE "MOCKGH issue create" "$fresh/gh.log" && grep -qE "MOCKGH issue close 4242 .*--reason completed" "$fresh/gh.log"'
rm -rf "$fresh"

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
