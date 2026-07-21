#!/usr/bin/env bash
# tests/test_mark_publish_deferred.sh — verifies scripts/mark-publish-deferred.sh (change 0083):
# the sole writer of the `## Publish deferred` marker. Pure file editor — no git, no network, so
# these need only a temp file. Run: bash tests/test_mark_publish_deferred.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$REPO/scripts/mark-publish-deferred.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

MARKER='## Publish deferred'

# mkfile — writes a minimal archived change file, prints its path.
mkfile(){
  local d; d="$(mktemp -d)"
  cat > "$d/2026-07-08-0043-sample.md" <<'EOF'
---
id: 43
slug: sample
title: A killed proposal
status: killed
priority: medium
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
<!-- docket:artifacts:end -->

## Why

Because.

## Why killed

Obsolete.
EOF
  printf '%s' "$d/2026-07-08-0043-sample.md"
}

assert "script exists and is executable" '[ -x "$SCRIPT" ]'

# --- add ---------------------------------------------------------------------------------------
f="$(mkfile)"
out="$(bash "$SCRIPT" --mode add --change-file "$f" --reason deferred \
        --detail "pending human approval" --date 2026-07-08 --integration-branch main --id 43 2>&1)"; rc=$?
body="$(cat "$f")"
# NOTE on assert style: never `printf … | grep -q …` under `set -o pipefail` (AGENTS.md) — the
# producer takes SIGPIPE when grep exits early and the 141 surfaces as an intermittent failure.
# Match against a here-string, or grep the file directly.
assert "add exits zero"                       '[ "$rc" -eq 0 ]'
assert "add writes the exact marker heading"  'grep -qxF -- "$MARKER" "$f"'
assert "add writes a dated sub-heading"       'grep -qF -- "### 2026-07-08" "$f"'
# NB: no backticks inside an assert expression — assert runs `eval "$2"`, which would treat them
# as command substitution. Match the backtick positions with `.` instead.
assert "add names the integration branch"     'grep -q "terminal-publish to .main. not completed" "$f"'
assert "add carries the reason prefix"        'grep -qF -- "**deferred**" "$f"'
assert "add carries the free-text detail"     'grep -qF -- "pending human approval" "$f"'
assert "add names the re-arm command"         'grep -qF -- "terminal-publish" "$f"'
assert "add preserves pre-existing body"      'grep -qxF -- "## Why killed" "$f"'
assert "add preserves frontmatter"            'grep -qxF -- "id: 43" "$f"'

# --- add is REPLACE, not APPEND (presence-encoded-state war story (a)) ---------------------------
out="$(bash "$SCRIPT" --mode add --change-file "$f" --reason blocked \
        --detail "direct push to protected main" --date 2026-07-09 --integration-branch main --id 43 2>&1)"; rc=$?
body="$(cat "$f")"
n="$(grep -cxF -- "$MARKER" "$f")"
assert "re-mark exits zero"                             '[ "$rc" -eq 0 ]'
assert "re-mark leaves EXACTLY ONE marker heading"      '[ "$n" -eq 1 ]'
assert "re-mark replaced the old reason"                '! grep -qF -- "pending human approval" "$f"'
assert "re-mark carries the new reason"                 'grep -qF -- "direct push to protected main" "$f"'
assert "re-mark still preserves the trailing section"   'grep -qxF -- "## Why killed" "$f"'

# --- remove ------------------------------------------------------------------------------------
out="$(bash "$SCRIPT" --mode remove --change-file "$f" 2>&1)"; rc=$?
body="$(cat "$f")"
assert "remove exits zero"                          '[ "$rc" -eq 0 ]'
assert "remove strips the marker heading"           '! grep -qxF -- "$MARKER" "$f"'
assert "remove strips the marker body"              '! grep -qF -- "direct push to protected main" "$f"'
assert "remove PRESERVES the following section"     'grep -qxF -- "## Why killed" "$f"'
assert "remove preserves the preceding section"     'grep -qxF -- "## Why" "$f"'
assert "remove preserves frontmatter"               'grep -qxF -- "id: 43" "$f"'

# remove on a file with NO marker is an idempotent no-op
before="$(cat "$f")"
out="$(bash "$SCRIPT" --mode remove --change-file "$f" 2>&1)"; rc=$?
assert "remove with no marker exits zero"      '[ "$rc" -eq 0 ]'
assert "remove with no marker changes nothing" '[ "$before" = "$(cat "$f")" ]'

# --- marker LAST in the file: removal must not eat the file, nor leave a dangling tail ----------
f2="$(mkfile)"
bash "$SCRIPT" --mode add --change-file "$f2" --reason deferred --detail "d" \
     --date 2026-07-08 --integration-branch main --id 43 >/dev/null 2>&1
bash "$SCRIPT" --mode remove --change-file "$f2" >/dev/null 2>&1
assert "marker-last removal keeps the final pre-existing section" 'grep -qxF -- "## Why killed" "$f2"'
assert "marker-last removal leaves no marker"                     '! grep -qxF -- "$MARKER" "$f2"'

# --- PROSE MENTION must not be treated as state (has_section's -x rule, applied to the writer) ---
f3="$(mkfile)"
printf '\nA sentence mentioning `%s` in prose.\n' "$MARKER" >> "$f3"
before="$(cat "$f3")"
bash "$SCRIPT" --mode remove --change-file "$f3" >/dev/null 2>&1
assert "remove ignores an inline prose MENTION of the marker" '[ "$before" = "$(cat "$f3")" ]'

# --- untrusted --detail (model-authored-values-are-untrusted-input) ------------------------------
f4="$(mkfile)"
err="$(bash "$SCRIPT" --mode add --change-file "$f4" --reason deferred \
        --detail "$(printf 'line1\nstatus: done')" --date 2026-07-08 --integration-branch main --id 43 2>&1)"; rc=$?
assert "multi-line --detail is REJECTED"        '[ "$rc" -ne 0 ]'
assert "multi-line --detail names the problem"  'grep -qiE "control|newline|single line" <<<"$err"'
assert "rejected --detail leaves the file untouched" '! grep -qxF -- "$MARKER" "$f4"'

# an ampersand is ordinary English and must survive verbatim (the sed-replacement trap)
f5="$(mkfile)"
bash "$SCRIPT" --mode add --change-file "$f5" --reason deferred --detail "approval & sign-off pending" \
     --date 2026-07-08 --integration-branch main --id 43 >/dev/null 2>&1
assert "an '&' in --detail survives verbatim" 'grep -qF -- "approval & sign-off pending" "$f5"'

# --- arg validation ------------------------------------------------------------------------------
err="$(bash "$SCRIPT" --mode add --change-file /nonexistent/nope.md --reason deferred 2>&1)"; rc=$?
assert "missing change file exits non-zero" '[ "$rc" -ne 0 ]'

err="$(bash "$SCRIPT" --mode sideways --change-file "$f" 2>&1)"; rc=$?
assert "invalid --mode exits non-zero"      '[ "$rc" -ne 0 ]'

err="$(bash "$SCRIPT" --mode add --change-file "$f" --reason sideways 2>&1)"; rc=$?
assert "invalid --reason exits non-zero"    '[ "$rc" -ne 0 ]'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
