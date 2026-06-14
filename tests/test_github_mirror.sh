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
# No --project given -> Projects skipped cleanly, issues still emitted, exit 0.
bash "$SCRIPT" --dry-run --changes-dir "$tmp" --repo o/r >/dev/null 2>&1
assert "exits 0 when no project is configured (Projects skipped, issues still mirrored)" \
  '[ "$?" -eq 0 ]'
projout="$(bash "$SCRIPT" --dry-run --changes-dir "$tmp" --repo o/r --project o/7 2>&1)"
assert "with --project, constructs a GraphQL Projects call" \
  'echo "$projout" | grep -qE "api graphql"'

# === C. REAL invocation through the mock gh (idempotent, best-effort) ========
GH_LOG="$tmp/gh.log" GH="$mock/gh" bash "$SCRIPT" --changes-dir "$tmp" --repo o/r \
  --metadata-branch docket --changes-path docs/changes >/dev/null 2>&1
rc=$?
assert "real run via mock gh exits 0 (best-effort never aborts)" '[ "'$rc'" -eq 0 ]'
assert "mock gh actually received an issue create call" \
  'grep -qE "MOCKGH issue create" "$tmp/gh.log"'
assert "mock gh received the close-completed call for the done change" \
  'grep -qE "MOCKGH issue close 88 .*--reason completed" "$tmp/gh.log"'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
