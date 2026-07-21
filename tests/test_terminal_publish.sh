#!/usr/bin/env bash
# tests/test_terminal_publish.sh — arg-validation guards for terminal-publish.sh. The --id/--adr
# integer guard fires at parse time, before any git work, so these need no repo. (change 0032)
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$REPO/scripts/terminal-publish.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

assert "script exists and is executable" '[ -x "$SCRIPT" ]'

err="$(bash "$SCRIPT" --id abc 2>&1)"; rc=$?
assert "--id abc exits non-zero"        '[ "$rc" -ne 0 ]'
assert "--id abc diagnostic names id"   'printf "%s" "$err" | grep -qiE "id"'

err="$(bash "$SCRIPT" --adr 1.5 2>&1)"; rc=$?
assert "--adr 1.5 exits non-zero"       '[ "$rc" -ne 0 ]'

# a valid integer id passes the int-guard (it dies later on a DIFFERENT, missing-arg error)
err="$(bash "$SCRIPT" --id 5 2>&1)"; rc=$?
assert "--id 5 passes the int guard"    '[ "$rc" -ne 0 ] && ! printf "%s" "$err" | grep -qi "non-integer"'

# --- change 0084: the --enabled contract ------------------------------------------------------
# Publish is opt-in. An OMITTED --enabled is a caller bug rather than a decision, so it no-ops
# LOUDLY; an explicit `--enabled false` is a decision, so it stays silent. Exit 0 on both paths:
# callers trust the exit code and a missing flag must never abort a close-out — the WARNING, not a
# non-zero exit, is what keeps a skipped publish from hiding (the #0043 silent-gap failure mode).
# The arg/mode/knob guards all run before any git work, so these need no repo fixture.
pub_args=(--id 5 --outcome done --integration-branch main --metadata-branch docket
          --changes-dir docs/changes --adrs-dir docs/adrs)

err="$(bash "$SCRIPT" "${pub_args[@]}" 2>&1)"; rc=$?
assert "omitted --enabled exits zero (never aborts a close-out)" '[ "$rc" -eq 0 ]'
assert "omitted --enabled warns on stderr"                       'printf "%s" "$err" | grep -q "WARNING"'
assert "omitted --enabled says NOTHING was published"            'printf "%s" "$err" | grep -qi "nothing was published"'
assert "omitted --enabled names the fix (--enabled true)"        'printf "%s" "$err" | grep -q -- "--enabled true"'

err="$(bash "$SCRIPT" "${pub_args[@]}" --enabled false 2>&1)"; rc=$?
assert "explicit --enabled false exits zero"                     '[ "$rc" -eq 0 ]'
assert "explicit --enabled false is SILENT (no WARNING)"          '! printf "%s" "$err" | grep -q "WARNING"'
assert "explicit --enabled false logs the suppression"           'printf "%s" "$err" | grep -q "terminal_publish: false"'

err="$(bash "$SCRIPT" "${pub_args[@]}" --enabled maybe 2>&1)"; rc=$?
assert "invalid --enabled exits non-zero"                        '[ "$rc" -ne 0 ]'
assert "invalid --enabled diagnostic names the value"            'printf "%s" "$err" | grep -q "maybe"'

# an explicit EMPTY value stays fail-closed — it must not be mistaken for an omitted flag
err="$(bash "$SCRIPT" "${pub_args[@]}" --enabled "" 2>&1)"; rc=$?
assert "empty --enabled exits non-zero (not treated as omitted)" '[ "$rc" -ne 0 ]'
assert "empty --enabled does NOT warn"                           '! printf "%s" "$err" | grep -q "WARNING"'

# --- change 0083: remove the `## Publish deferred` marker on a successful publish ---------------
# Needs a real repo (the arg-guard tests above do not): the removal writes and pushes on the
# metadata branch, which is exactly the state a hermetic fixture must construct rather than mock
# (metadata-branch-invisible-to-suite). Local bare origin, two branches, no network, no gh.
MARKER='## Publish deferred'
git_quiet(){ git "$@" >/dev/null 2>&1; }

# tp_repo: prints "<work> <origin>" — bare origin holding main + docket; docket carries an
# archived change file (id 60) that CARRIES the marker, plus its spec.
#
# main carries a marker-CARRYING copy of that same archived file too. That mirrors reality (in
# main-mode the archive lives on the integration branch) and it is what makes two asserts real
# rather than vacuous: the `origin/main` marker assert now requires the publish to OVERWRITE a
# marker that was already there, and the main-mode suppression mutation can actually reach the
# removal block instead of dying earlier on a missing archive.
tp_repo(){
  local root work origin
  root="$(mktemp -d)"; origin="$root/origin.git"; work="$root/work"
  git_quiet init --bare "$origin"
  git_quiet clone "$origin" "$work"
  git -C "$work" config user.email t@t; git -C "$work" config user.name t
  git_quiet -C "$work" checkout -b main
  mkdir -p "$work/docs/adrs" "$work/docs/changes/archive"
  echo "# adr index" > "$work/docs/adrs/README.md"
  tp_change_file > "$work/docs/changes/archive/2026-07-08-0060-sample.md"
  git -C "$work" add -A; git_quiet -C "$work" commit -m "main baseline"; git_quiet -C "$work" push -u origin main
  git_quiet -C "$work" checkout --orphan docket
  git_quiet -C "$work" rm -rf .
  mkdir -p "$work/docs/changes/archive" "$work/docs/superpowers/specs" "$work/docs/adrs"
  echo "# spec" > "$work/docs/superpowers/specs/2026-07-08-sample.md"
  tp_change_file > "$work/docs/changes/archive/2026-07-08-0060-sample.md"
  git -C "$work" add -A; git_quiet -C "$work" commit -m "docket baseline"; git_quiet -C "$work" push -u origin docket
  printf '%s %s\n' "$work" "$origin"
}

tp_change_file(){
  cat <<'CF'
---
id: 60
slug: sample
title: Archived change whose publish was deferred
status: killed
priority: medium
spec: docs/superpowers/specs/2026-07-08-sample.md
adrs: []
---

## Why killed

Obsolete.

## Publish deferred

### 2026-07-08 — terminal-publish to `main` not completed

**deferred** — pending human approval
CF
}

read -r TW TO < <(tp_repo)
tp_args=(--id 60 --outcome killed --integration-branch main --metadata-branch docket
         --changes-dir docs/changes --adrs-dir docs/adrs --metadata-worktree "$TW")

# (a) suppression carve-outs write and remove NOTHING — the marker must survive untouched.
( cd "$TW" && bash "$SCRIPT" "${tp_args[@]}" --enabled false >/dev/null 2>&1 )
assert "publish suppressed (--enabled false) leaves the marker in place" \
  'grep -qxF -- "$MARKER" "$TW/docs/changes/archive/2026-07-08-0060-sample.md"'

( cd "$TW" && bash "$SCRIPT" --id 60 --outcome killed --integration-branch main --metadata-branch main \
    --changes-dir docs/changes --adrs-dir docs/adrs --metadata-worktree "$TW" --enabled true >/dev/null 2>&1 )
assert "main-mode publish leaves the marker in place" \
  'grep -qxF -- "$MARKER" "$TW/docs/changes/archive/2026-07-08-0060-sample.md"'

# (b) a real, enabled publish removes the marker on the METADATA branch and pushes it...
( cd "$TW" && bash "$SCRIPT" "${tp_args[@]}" --enabled true >/dev/null 2>&1 ); tprc=$?
assert "enabled publish exits zero" '[ "$tprc" -eq 0 ]'
git_quiet -C "$TW" fetch origin docket
meta_body="$(git -C "$TW" show origin/docket:docs/changes/archive/2026-07-08-0060-sample.md 2>/dev/null)"
assert "successful publish removed the marker on origin/docket" \
  '! grep -qxF -- "$MARKER" <<<"$meta_body"'
assert "marker removal preserved the rest of the archived record" \
  'grep -qxF -- "## Why killed" <<<"$meta_body"'

# ...and the copy that landed on the INTEGRATION branch is marker-free too (the ordering property:
# the removal must precede the copy-set build, or main receives a stale "not completed" marker).
git_quiet -C "$TW" fetch origin main
int_body="$(git -C "$TW" show origin/main:docs/changes/archive/2026-07-08-0060-sample.md 2>/dev/null)"
assert "the published record landed on origin/main"        '[ -n "$int_body" ]'
assert "the published record carries NO stale marker"      '! grep -qxF -- "$MARKER" <<<"$int_body"'

# (c) idempotent re-run: a second publish on an already-clean record is a no-op that still exits 0.
( cd "$TW" && bash "$SCRIPT" "${tp_args[@]}" --enabled true >/dev/null 2>&1 ); tprc2=$?
assert "re-publish with no marker exits zero (idempotent)" '[ "$tprc2" -eq 0 ]'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
