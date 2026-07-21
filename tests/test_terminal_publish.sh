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

# main-mode gets a metadata worktree actually checked out on `main`, because that is what main-mode
# IS (metadata branch == integration branch == the tree you are standing in). Handing it the
# docket-checked-out $TW instead would be an incoherent fixture, and the HEAD-on-metadata-branch
# guard would then stop the run for that reason — masking the suppression mutation (mode guard
# relocated below the removal block), which must keep reddening the assert below.
TWM="$(mktemp -d)/mainwt"
git_quiet clone "$TO" "$TWM"
git -C "$TWM" config user.email t@t; git -C "$TWM" config user.name t
git_quiet -C "$TWM" checkout main
( cd "$TWM" && bash "$SCRIPT" --id 60 --outcome killed --integration-branch main --metadata-branch main \
    --changes-dir docs/changes --adrs-dir docs/adrs --metadata-worktree "$TWM" --enabled true >/dev/null 2>&1 )
assert "main-mode publish leaves the marker in place" \
  'grep -qxF -- "$MARKER" "$TWM/docs/changes/archive/2026-07-08-0060-sample.md"'

# baseline for the landing assert below: origin/main carries the archived record already (marker
# and all — see tp_repo), but NOT the spec. So "the spec is on main" is a property ONLY the publish
# can establish, unlike "the archived record is on main", which is true before anything runs.
git_quiet -C "$TW" fetch origin main
spec_baseline="$(git -C "$TW" show origin/main:docs/superpowers/specs/2026-07-08-sample.md 2>/dev/null)"
assert "fixture baseline: origin/main carries no spec yet" '[ -z "$spec_baseline" ]'

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
spec_landed="$(git -C "$TW" show origin/main:docs/superpowers/specs/2026-07-08-sample.md 2>/dev/null)"
# Deliberately keyed on the SPEC, not on the archived record's content: main was seeded with that
# record at baseline, so `[ -n "$int_body" ]` was vacuously true before any publish ran. Keying the
# landing assert on the spec (absent at baseline) also keeps the ordering mutation's signature
# single-assert — moving the removal block after the copy-set build must redden the marker assert
# below and nothing else.
assert "the publish delivered the copy-set to origin/main" '[ -n "$spec_landed" ]'
assert "the published record carries NO stale marker"      '! grep -qxF -- "$MARKER" <<<"$int_body"'

# (c) idempotent re-run: a second publish on an already-clean record is a no-op that still exits 0.
( cd "$TW" && bash "$SCRIPT" "${tp_args[@]}" --enabled true >/dev/null 2>&1 ); tprc2=$?
assert "re-publish with no marker exits zero (idempotent)" '[ "$tprc2" -eq 0 ]'

# (d) REGRESSION — the removal gate must read the REMOTE copy, not the local working-tree file.
# Gating on `[ -f "$mark_file" ]` alone meant a metadata worktree that was missing, behind, or
# resolved to the wrong path skipped the removal SILENTLY: the script published the still-marked
# record onto the integration branch and exited 0. That is the whole gap this change closes, so it
# gets a test whose central claim is "NOT exit 0".
CHANGE_REL=docs/changes/archive/2026-07-08-0060-sample.md
read -r TW2 TO2 < <(tp_repo)
bad_args=(--id 60 --outcome killed --integration-branch main --metadata-branch docket
          --changes-dir docs/changes --adrs-dir docs/adrs)

# (d1) --metadata-worktree points at a path that does not exist
out="$( cd "$TW2" && bash "$SCRIPT" "${bad_args[@]}" --metadata-worktree "$TW2/no-such-tree" \
        --enabled true 2>&1 )"; badrc=$?
assert "a nonexistent --metadata-worktree does NOT publish-and-exit-0" '[ "$badrc" -ne 0 ]'
assert "the nonexistent-worktree run never reports a publish" '! grep -q "record(s)" <<<"$out"'
assert "the nonexistent-worktree diagnostic names the marker" 'grep -qF -- "Publish deferred" <<<"$out"'
git_quiet -C "$TW2" fetch origin docket
d1_meta="$(git -C "$TW2" show origin/docket:$CHANGE_REL 2>/dev/null)"
assert "the refused run left the marker intact on origin/docket" 'grep -qxF -- "$MARKER" <<<"$d1_meta"'

# (d2) the worktree resolves, but its file has already lost the marker while origin still carries
# it (a diverged/behind tree, or an unpushed removal) — also an error, never a silent skip.
# Its OWN fixture: (d1) shares nothing with it, and a mutation that lets (d1) get as far as
# provisioning `pub-60` would otherwise leave that worktree registered and (d2) would die on the
# leftover instead of on the guard under test.
read -r TW4 TO4 < <(tp_repo)
"$REPO/scripts/mark-publish-deferred.sh" --mode remove --change-file "$TW4/$CHANGE_REL" >/dev/null 2>&1
out="$( cd "$TW4" && bash "$SCRIPT" "${bad_args[@]}" --metadata-worktree "$TW4" \
        --enabled true 2>&1 )"; badrc2=$?
assert "an out-of-sync metadata worktree does NOT publish-and-exit-0" '[ "$badrc2" -ne 0 ]'
assert "the out-of-sync run never reports a publish"          '! grep -q "record(s)" <<<"$out"'
assert "the out-of-sync diagnostic names the metadata branch" 'grep -q "origin/docket" <<<"$out"'

# (e) REGRESSION — a rebase failure must not WEDGE the shared metadata worktree.
# The CAS retry loop dies when `pull --rebase` fails. $META_WORKTREE is the real, shared `.docket`
# every later docket operation runs in (unlike `pub`, which is thrown away), so dying mid-rebase
# left it detached with conflicted paths until a human ran `git rebase --abort`.
read -r TW3 TO3 < <(tp_repo)
# a concurrent writer diverges the very lines the marker removal deletes => guaranteed conflict
CW="$(mktemp -d)/cw"
git_quiet clone "$TO3" "$CW"
git -C "$CW" config user.email t@t; git -C "$CW" config user.name t
git_quiet -C "$CW" checkout docket
sed -i.bak 's/pending human approval/CONCURRENT EDIT on the same lines/' "$CW/$CHANGE_REL"
rm -f "$CW/$CHANGE_REL.bak"
git -C "$CW" add -A; git_quiet -C "$CW" commit -m "concurrent metadata write"; git_quiet -C "$CW" push origin docket

out="$( cd "$TW3" && bash "$SCRIPT" --id 60 --outcome killed --integration-branch main \
        --metadata-branch docket --changes-dir docs/changes --adrs-dir docs/adrs \
        --metadata-worktree "$TW3" --enabled true 2>&1 )"; rbrc=$?
assert "a conflicting concurrent writer fails the publish" '[ "$rbrc" -ne 0 ]'
assert "the rebase failure is what gets reported"          'grep -q "rebase failed" <<<"$out"'
TW3_GITDIR="$(cd "$TW3" && git rev-parse --absolute-git-dir)"
tw3_head="$(git -C "$TW3" symbolic-ref --quiet --short HEAD 2>/dev/null)"
tw3_unmerged="$(git -C "$TW3" ls-files --unmerged)"
assert "the shared metadata worktree is NOT left mid-rebase" \
  '[ ! -d "$TW3_GITDIR/rebase-merge" ] && [ ! -d "$TW3_GITDIR/rebase-apply" ]'
assert "the shared metadata worktree HEAD is NOT left detached" '[ "$tw3_head" = "docket" ]'
assert "the shared metadata worktree has NO conflicted paths"  '[ -z "$tw3_unmerged" ]'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
