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
# NB: must assert on the "**Re-arm:**" marker itself, not just "terminal-publish" — that substring
# is already satisfied by the `### <date> — terminal-publish to ...` sub-heading above, so a grep
# for it alone would stay green even if the whole **Re-arm:** paragraph were deleted.
assert "add names the re-arm command"         'grep -qF -- "**Re-arm:**" "$f"'
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
# NB: `## Why killed` PRECEDES the marker here — mkfile's fixture always appends the marker LAST,
# so this only exercises a section that comes BEFORE the marker, never a genuine trailing one.
assert "re-mark still preserves the preceding section"  'grep -qxF -- "## Why killed" "$f"'

# --- remove ------------------------------------------------------------------------------------
out="$(bash "$SCRIPT" --mode remove --change-file "$f" 2>&1)"; rc=$?
body="$(cat "$f")"
assert "remove exits zero"                          '[ "$rc" -eq 0 ]'
assert "remove strips the marker heading"           '! grep -qxF -- "$MARKER" "$f"'
assert "remove strips the marker body"              '! grep -qF -- "direct push to protected main" "$f"'
# NB: both of these are sections that PRECEDE the marker (mkfile always appends the marker LAST) —
# renamed from "remove PRESERVES the following section" / "remove preserves the preceding section",
# which mis-described the fixture: neither ever exercised a section genuinely AFTER the marker.
assert "remove preserves the immediately-preceding section" 'grep -qxF -- "## Why killed" "$f"'
assert "remove preserves an earlier preceding section"      'grep -qxF -- "## Why" "$f"'
assert "remove preserves frontmatter"               'grep -qxF -- "id: 43" "$f"'

# --- a genuine TRAILING section (marker in the MIDDLE of the file) must survive removal ----------
# mkfile + `--mode add` always appends the marker LAST, so every assert above only ever covered a
# section that precedes the marker. This is the actual scenario the section-terminator guard (the
# next column-0 `## ` heading ends the section) exists for: append a real `## ` section AFTER the
# marker directly to the file (never through the script, which cannot produce this layout), then
# confirm `--mode remove` strips the marker while leaving the trailing section intact.
f6="$(mkfile)"
bash "$SCRIPT" --mode add --change-file "$f6" --reason deferred --detail "d" \
     --date 2026-07-08 --integration-branch main --id 43 >/dev/null 2>&1
printf '\n## Something Else\n\nTrailing content that must survive.\n' >> "$f6"
bash "$SCRIPT" --mode remove --change-file "$f6" >/dev/null 2>&1
assert "remove with marker in the MIDDLE strips the marker"        '! grep -qxF -- "$MARKER" "$f6"'
# The marker's OWN `### <date> …` sub-heading must not leak either — this is the actual guard a
# broadened terminator (e.g. `/^#/` instead of `/^## /`) would break: it would treat that
# sub-heading as the section end, printing it and everything after (including the marker's own
# reason line) verbatim instead of skipping through to the real trailing section.
assert "remove with marker in the MIDDLE strips the marker's dated sub-heading" \
       '! grep -q "terminal-publish to .main. not completed" "$f6"'
assert "remove with marker in the MIDDLE strips the marker's reason line" \
       '! grep -qF -- "**deferred** — d" "$f6"'
assert "remove with marker in the MIDDLE preserves the trailing section heading" \
       'grep -qxF -- "## Something Else" "$f6"'
assert "remove with marker in the MIDDLE preserves the trailing section body" \
       'grep -qF -- "Trailing content that must survive." "$f6"'

# remove on a file with NO marker must be a TRUE no-op: byte-identical, not just line-identical.
# `$(cat "$f")` (command substitution) strips trailing newlines, so comparing that way would stay
# green even if `remove` silently rewrote the file and dropped trailing blank lines — exactly the
# CRITICAL regression this guards: give the file trailing blank lines first, then compare raw
# bytes with `cmp`, never through a variable.
printf '\n\n' >> "$f"
before_copy="$(mktemp)"
cp "$f" "$before_copy"
out="$(bash "$SCRIPT" --mode remove --change-file "$f" 2>&1)"; rc=$?
assert "remove with no marker exits zero" '[ "$rc" -eq 0 ]'
assert "remove with no marker leaves the file BYTE-IDENTICAL (trailing blank lines untouched)" \
       'cmp -s "$before_copy" "$f"'
rm -f "$before_copy"

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
