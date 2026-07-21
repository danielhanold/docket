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

# Every fixture is minted UNDER one root, removed by an EXIT trap: the per-fixture `mktemp -d`
# calls used to leak a directory apiece into $TMPDIR on every run.
MPD_ROOT="$(mktemp -d)"
trap 'rm -rf "$MPD_ROOT"' EXIT

# mkfile — writes a minimal archived change file, prints its path.
mkfile(){
  local d; d="$(mktemp -d "$MPD_ROOT/fixture.XXXXXX")"
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
before_copy="$(mktemp "$MPD_ROOT/before.XXXXXX")"
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

# --- a file with NO TRAILING NEWLINE is a genuine no-op on remove (change 0083 review, minor 6) ---
# The old gate stripped the marker into a temp file and `cmp -s`'d it against the input. awk always
# terminates its last output line, so for a file that does not end in `\n` the stripped copy
# differed by exactly that byte: `cmp` said "changed", the no-op path was skipped, and the file was
# rewritten WITH a newline appended — falsifying this script's documented "byte-untouched, not
# merely line-equivalent" claim. The precondition grep decides it now, before any temp file exists.
nonl_dir="$(mktemp -d "$MPD_ROOT/nonl.XXXXXX")"
nonl="$nonl_dir/nonl.md"
printf -- '---\nid: 9\n---\n\n## Why\n\nBecause.' > "$nonl"   # deliberately NO trailing newline
cp "$nonl" "$nonl_dir/before"
assert "fixture precondition: the no-trailing-newline file really lacks one" \
  '[ "$(tail -c 1 "$nonl")" != "" ]'
out="$(bash "$SCRIPT" --mode remove --change-file "$nonl" 2>&1)"; rc=$?
assert "remove on a markerless file with NO trailing newline exits zero" '[ "$rc" -eq 0 ]'
assert "remove on a markerless file with NO trailing newline appends nothing (BYTE-identical)" \
  'cmp -s "$nonl_dir/before" "$nonl"'

# ...and the precondition must not have broken the real removal on such a file.
nonl2="$nonl_dir/nonl2.md"
printf -- '---\nid: 9\n---\n\n## Why\n\nBecause.\n\n## Publish deferred\n\nstill blocked.' > "$nonl2"
bash "$SCRIPT" --mode remove --change-file "$nonl2" >/dev/null 2>&1
assert "remove still strips the marker from a file with no trailing newline" \
  '! grep -qxF -- "$MARKER" "$nonl2"'
assert "remove keeps that file's body"      'grep -qxF -- "## Why" "$nonl2"'
assert "remove keeps that file's frontmatter" 'grep -qxF -- "id: 9" "$nonl2"'

# --- a failed body read must never yield a TRUNCATED record (change 0083 review, finding 2) -------
# `{ cat "$tmp.2"; printf …; } > "$tmp.3" || die` read only the LAST command's status, so a failed
# `cat` (ENOSPC/EIO) left $tmp.3 holding the marker section ALONE, `die` never fired, and the `mv`
# replaced the archived record with it — the whole body destroyed, exit 0, by the sole writer of a
# durable record. `cat` is invoked unqualified, so a PATH stub reproduces the I/O failure exactly.
stub_fail="$MPD_ROOT/stub-cat-fail"; mkdir -p "$stub_fail"
printf '#!/usr/bin/env bash\nexit 1\n' > "$stub_fail/cat"; chmod +x "$stub_fail/cat"
f7="$(mkfile)"; cp "$f7" "$MPD_ROOT/before7"
err="$(PATH="$stub_fail:$PATH" bash "$SCRIPT" --mode add --change-file "$f7" --reason blocked \
        --date 2026-07-08 --integration-branch main --id 43 2>&1)"; rc=$?
assert "a failed body read exits NON-ZERO (the group's status no longer hides it)" '[ "$rc" -ne 0 ]'
assert "a failed body read leaves the record BYTE-untouched"       'cmp -s "$MPD_ROOT/before7" "$f7"'
assert "a failed body read never produces a marker-only record"    'grep -qxF -- "## Why killed" "$f7"'

# A read that TRUNCATES but still reports success slips past every `|| die`; the size postcondition
# is what catches it. Needs a body far larger than the ~500-byte marker section — with the tiny
# fixture the marker section alone outweighs the loss and the check legitimately cannot see it.
stub_trunc="$MPD_ROOT/stub-cat-trunc"; mkdir -p "$stub_trunc"
printf '#!/usr/bin/env bash\nhead -n 1 -- "$1"\nexit 0\n' > "$stub_trunc/cat"; chmod +x "$stub_trunc/cat"
f8="$(mkfile)"
for _ in $(seq 1 120); do printf 'Filler line that makes the body substantially larger than the marker section.\n' >> "$f8"; done
cp "$f8" "$MPD_ROOT/before8"
err="$(PATH="$stub_trunc:$PATH" bash "$SCRIPT" --mode add --change-file "$f8" --reason blocked \
        --date 2026-07-08 --integration-branch main --id 43 2>&1)"; rc=$?
assert "a silently-truncating body read exits NON-ZERO (size postcondition)" '[ "$rc" -ne 0 ]'
assert "a silently-truncating body read leaves the record BYTE-untouched" 'cmp -s "$MPD_ROOT/before8" "$f8"'
assert "the size postcondition says what it refused"  'grep -qiE -- "postcondition|smaller" <<<"$err"'

# --- --id and --integration-branch are untrusted too (change 0083 review, finding 3) --------------
# Unvalidated, they were worse than --detail: a newline injects lines INTO the marker section, and
# a column-0 `## ` heading among them TERMINATES the section for the strip pass — so `--mode remove`
# stops at the injected heading and strands the tail in the record permanently, while
# terminal-publish.sh (which only checks the marker heading is gone) publishes it with exit 0.
f9="$(mkfile)"; cp "$f9" "$MPD_ROOT/before9"
err="$(bash "$SCRIPT" --mode add --change-file "$f9" --reason blocked --date 2026-07-08 \
        --id "$(printf '43\n\n## Fake heading\n\ninjected')" 2>&1)"; rc=$?
assert "a multi-line --id is REJECTED"                     '[ "$rc" -ne 0 ]'
assert "a rejected --id names the offending flag"          'grep -qiE -- "\-\-id|integer" <<<"$err"'
assert "a rejected --id leaves the file BYTE-untouched"    'cmp -s "$MPD_ROOT/before9" "$f9"'
assert "a rejected --id injects no column-0 heading"       '! grep -qxF -- "## Fake heading" "$f9"'

err="$(bash "$SCRIPT" --mode add --change-file "$f9" --reason blocked --date 2026-07-08 --id abc 2>&1)"; rc=$?
assert "a non-numeric --id is REJECTED"                    '[ "$rc" -ne 0 ]'
assert "the non-numeric --id rejection touches nothing"    'cmp -s "$MPD_ROOT/before9" "$f9"'

# Its OWN fixture, deliberately: sharing $f9 made a mutation that disables the --id guard corrupt
# the file and redden these asserts as collateral, blurring which guard each mutation proves.
f10="$(mkfile)"; cp "$f10" "$MPD_ROOT/before10"
err="$(bash "$SCRIPT" --mode add --change-file "$f10" --reason blocked --date 2026-07-08 --id 43 \
        --integration-branch "$(printf 'main\n\n## Fake heading\n\ninjected')" 2>&1)"; rc=$?
assert "a multi-line --integration-branch is REJECTED"              '[ "$rc" -ne 0 ]'
assert "the rejection names control characters"                     'grep -qiE -- "control|integration-branch" <<<"$err"'
assert "a rejected --integration-branch leaves the file BYTE-untouched" 'cmp -s "$MPD_ROOT/before10" "$f10"'
assert "a rejected --integration-branch injects no column-0 heading" '! grep -qxF -- "## Fake heading" "$f10"'

# The valid shapes still pass — the guards reject, they do not block ordinary use.
f11="$(mkfile)"
bash "$SCRIPT" --mode add --change-file "$f11" --reason blocked --date 2026-07-08 --id 43 \
     --integration-branch feature/int-2 >/dev/null 2>&1; rc=$?
assert "a numeric --id and an ordinary branch name are still ACCEPTED" '[ "$rc" -eq 0 ]'
assert "the accepted run wrote the marker" 'grep -qxF -- "$MARKER" "$f11"'

# --- arg validation ------------------------------------------------------------------------------
err="$(bash "$SCRIPT" --mode add --change-file /nonexistent/nope.md --reason deferred 2>&1)"; rc=$?
assert "missing change file exits non-zero" '[ "$rc" -ne 0 ]'

err="$(bash "$SCRIPT" --mode sideways --change-file "$f" 2>&1)"; rc=$?
assert "invalid --mode exits non-zero"      '[ "$rc" -ne 0 ]'

err="$(bash "$SCRIPT" --mode add --change-file "$f" --reason sideways 2>&1)"; rc=$?
assert "invalid --reason exits non-zero"    '[ "$rc" -ne 0 ]'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
