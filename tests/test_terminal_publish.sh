#!/usr/bin/env bash
# tests/test_terminal_publish.sh — arg-validation guards for terminal-publish.sh. The --id/--adr
# integer guard fires at parse time, before any git work, so these need no repo. (change 0032)
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$REPO/scripts/terminal-publish.sh"
DOCKET_BASH_PATH=""
for runtime_candidate in "$(command -v bash)" /opt/homebrew/bin/bash /usr/local/bin/bash; do
  [ -x "$runtime_candidate" ] || continue
  [ "$(LC_ALL=C "$runtime_candidate" --version 2>/dev/null | sed -n 's/^GNU bash, version \([0-9][0-9]*\)\..*/\1/p')" -ge 4 ] 2>/dev/null || continue
  DOCKET_BASH_PATH="$runtime_candidate"; break
done
: "${DOCKET_BASH_PATH:?tests require an absolute GNU Bash 4+ runtime}"
export DOCKET_BASH_PATH
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

# Every fixture below lives under ONE root that is removed on exit. Each fixture used to mint its
# own `mktemp -d` and leak it (six per run). tp_repo runs inside a process substitution — a
# SUBSHELL — so a "register the dir in an array" scheme could not work: the append would be lost.
# Minting under $TP_ROOT survives the subshell because the parenthood is in the path, not in state.
TP_ROOT="$(mktemp -d)"
trap 'rm -rf "$TP_ROOT"' EXIT

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
  root="$(mktemp -d "$TP_ROOT/repo.XXXXXX")"; origin="$root/origin.git"; work="$root/work"
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
TWM="$TP_ROOT/mainwt"
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
read -r TW2 _ < <(tp_repo)
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
read -r TW4 _ < <(tp_repo)
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
CW="$TP_ROOT/cw"
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
# ...and the report must carry GIT'S OWN diagnostic. Muting `pull --rebase` collapsed conflict,
# dirty tree, already-rebasing and network failure into one opaque line, which is four different
# operator actions behind one message.
assert "the rebase report carries git's own diagnostic, not just ours" \
  'grep -qiE "conflict|could not apply|error:" <<<"$out"'

# (e2) the abort must be CONDITIONAL. `.docket` is shared with concurrent autonomous loops, so
# `pull --rebase` can fail *because someone else's* rebase is already in flight — and an
# unconditional `rebase --abort` then destroys THAT operation's state, not ours. Driven through the
# advertised GIT mock seam: the stand-in fails the marker-removal push and, in the same breath,
# wedges the metadata worktree into a rebase this script did not start (the race, made
# deterministic). The pre-existing rebase must survive.
read -r TW8 TO8 < <(tp_repo)
CW8="$TP_ROOT/cw8"
git_quiet clone "$TO8" "$CW8"
git -C "$CW8" config user.email t@t; git -C "$CW8" config user.name t
git_quiet -C "$CW8" checkout docket
sed -i.bak 's/pending human approval/CONCURRENT EDIT on the same lines/' "$CW8/$CHANGE_REL"
rm -f "$CW8/$CHANGE_REL.bak"
git -C "$CW8" add -A; git_quiet -C "$CW8" commit -m "concurrent metadata write"; git_quiet -C "$CW8" push origin docket

GITWRAP="$TP_ROOT/git-wrap.sh"
cat > "$GITWRAP" <<'WRAP'
#!/usr/bin/env bash
# passthrough git, except: the first `git -C <mw> push …` fails AND leaves <mw> mid-rebase, as if a
# concurrent docket loop had started one between the HEAD guard and the pull. Matched on the
# METADATA worktree specifically — the publish's own `git -C <pub> push` must run untouched, or the
# ordering mutation (which reorders the two) would drag this fixture into its signature.
if [ "${1:-}" = "-C" ] && [ "${2:-}" = "$WEDGE_MW" ] && [ "${3:-}" = "push" ] && [ ! -e "$WEDGE_FLAG" ]; then
  : > "$WEDGE_FLAG"
  git -C "$WEDGE_MW" rebase origin/docket >/dev/null 2>&1
  exit 1
fi
exec git "$@"
WRAP
chmod +x "$GITWRAP"
out="$( cd "$TW8" && GIT="$GITWRAP" WEDGE_MW="$TW8" WEDGE_FLAG="$TP_ROOT/wedged" \
        bash "$SCRIPT" "${bad_args[@]}" --metadata-worktree "$TW8" --enabled true 2>&1 )"; prerc=$?
TW8_GITDIR="$(cd "$TW8" && git rev-parse --absolute-git-dir)"
assert "a pre-existing rebase still fails the publish"      '[ "$prerc" -ne 0 ]'
assert "the pre-existing-rebase failure is reported"        'grep -q "rebase failed" <<<"$out"'
assert "a rebase this script did NOT start is left alone" \
  '[ -d "$TW8_GITDIR/rebase-merge" ] || [ -d "$TW8_GITDIR/rebase-apply" ]'

# (f) the HEAD-on-$META_BRANCH guard. The removal is committed and pushed with `HEAD:$META_BRANCH`,
# i.e. it publishes whatever the metadata worktree has CHECKED OUT — not the branch named by
# --metadata-branch. On a DETACHED worktree that pushes an unrelated line of history onto the
# metadata branch and then publishes from it. Previously the guard was reached twice per run but
# never driven into its `die`, so deleting it changed no test: it was decoration. This fixture is
# what makes it code.
read -r TW5 _ < <(tp_repo)
git_quiet -C "$TW5" checkout --detach
out="$( cd "$TW5" && bash "$SCRIPT" "${bad_args[@]}" --metadata-worktree "$TW5" \
        --enabled true 2>&1 )"; detrc=$?
assert "a DETACHED metadata worktree does NOT publish-and-exit-0"  '[ "$detrc" -ne 0 ]'
assert "the detached-worktree diagnostic says 'detached HEAD'"     'grep -q "detached HEAD" <<<"$out"'
assert "the detached-worktree run never reports a publish"         '! grep -q "record(s)" <<<"$out"'
# Keyed on origin/DOCKET, not origin/main: deleting the guard lets the removal be committed on the
# detached HEAD and pushed with `HEAD:docket`, so the marker vanishes from the metadata tip — the
# specific damage this guard prevents. Keying it on origin/main instead would ALSO redden under the
# ordering mutation (M1), blurring that mutation's deliberately single-assert signature.
git_quiet -C "$TW5" fetch origin docket
det_meta="$(git -C "$TW5" show "origin/docket:$CHANGE_REL" 2>/dev/null)"
assert "the detached-worktree run left the marker intact on origin/docket" \
  'grep -qxF -- "$MARKER" <<<"$det_meta"'

# (f2) `symbolic-ref` is EMPTY for two distinct faults: a detached HEAD, and "not a git worktree at
# all". A mis-resolved --metadata-worktree that merely happens to hold the change file was reported
# as a detached HEAD, sending the reader after a rebase that never existed. The two must read
# differently. $PLAIN sits under $TP_ROOT (mktemp territory, outside any repo) so `rev-parse
# --git-dir` genuinely fails there.
read -r TW6 _ < <(tp_repo)
PLAIN="$TP_ROOT/plain-not-a-worktree"
mkdir -p "$PLAIN/docs/changes/archive"
tp_change_file > "$PLAIN/$CHANGE_REL"
out="$( cd "$TW6" && bash "$SCRIPT" "${bad_args[@]}" --metadata-worktree "$PLAIN" \
        --enabled true 2>&1 )"; plainrc=$?
assert "a non-repo --metadata-worktree does NOT publish-and-exit-0" '[ "$plainrc" -ne 0 ]'
assert "a non-repo --metadata-worktree is NOT misreported as detached" \
  '! grep -q "detached HEAD" <<<"$out"'
assert "the non-repo diagnostic says it is not a git worktree" \
  'grep -q "not a git worktree" <<<"$out"'

# (g) the fail-closed postcondition. It is what backs the claim "there is no path on which a
# marker-carrying record reaches the integration branch with exit 0" — and until now that claim was
# asserted, never proven: nothing made the removal silently not-happen. A stand-in
# mark-publish-deferred.sh does. It reports SUCCESS and touches the file (so the `git commit -- <path>`
# below still has something to record, which a pure no-op would not) while LEAVING the marker in
# place. $0-relative dispatch means the stand-in has to live in a copied scripts dir, not on PATH.
read -r TW7 _ < <(tp_repo)
FAKE_SCRIPTS="$TP_ROOT/fake-scripts"
cp -R "$REPO/scripts" "$FAKE_SCRIPTS"
cat > "$FAKE_SCRIPTS/mark-publish-deferred.sh" <<'STUB'
#!/usr/bin/env bash
# stand-in: reports success WITHOUT removing the marker (a silently-failed removal)
f=""
while [ $# -gt 0 ]; do case "$1" in --change-file) f="$2"; shift ;; esac; shift; done
[ -n "$f" ] && printf '\n<!-- stand-in touched this file but removed nothing -->\n' >> "$f"
exit 0
STUB
chmod +x "$FAKE_SCRIPTS/mark-publish-deferred.sh"
out="$( cd "$TW7" && bash "$FAKE_SCRIPTS/terminal-publish.sh" "${bad_args[@]}" \
        --metadata-worktree "$TW7" --enabled true 2>&1 )"; postrc=$?
assert "a silently-failed removal does NOT publish-and-exit-0"   '[ "$postrc" -ne 0 ]'
assert "the surviving-marker diagnostic names the metadata tip"  'grep -q "survives on origin/docket" <<<"$out"'
assert "the surviving-marker run never reports a publish"        '! grep -q "record(s)" <<<"$out"'
# The direct proof of the advertised guarantee — "no path on which a marker-carrying record reaches
# the integration branch with exit 0". Keyed on the stand-in's SENTINEL rather than on the marker or
# the spec: origin/main was seeded with a marker-carrying record at baseline (so a marker assert
# here is vacuous), and a spec assert would also redden under the ordering mutation (M1), which must
# keep its single-assert signature. The sentinel appears on the metadata tip only once the
# silently-failed removal has run, so it lands on main only if the postcondition let it through.
git_quiet -C "$TW7" fetch origin main
post_int="$(git -C "$TW7" show "origin/main:$CHANGE_REL" 2>/dev/null)"
assert "the silently-unremoved record never reached origin/main" \
  '! grep -q "stand-in touched" <<<"$post_int"'

# (0136) terminal-publish re-stamps the change's plan/results back-links inside the SINGLE publish
# commit. A dedicated hermetic fixture: a change (id 70) archived on docket carrying plan:/results:,
# with those two files already present on the integration branch (main) — the post-PR-merge state.
# The renderer that terminal-publish invokes resolves metadata_branch/changes_dir via a DOCKET_CONFIG
# stub, inherited by the renderer subprocess, so the fixture stays offline
# (metadata-branch-invisible-to-suite). The archived change is checked out into `pub` (it is in the
# copy-set), so the renderer reads its id/title and derives the archive/ relpath from there.
RS_CONFIG="$TP_ROOT/rs-config.sh"
cat > "$RS_CONFIG" <<'RSC'
#!/usr/bin/env bash
echo "METADATA_BRANCH=docket"
echo "CHANGES_DIR=docs/changes"
RSC
chmod +x "$RS_CONFIG"

RS_CHANGE_REL=docs/changes/archive/2026-07-15-0070-restamp.md
RS_PLAN_REL=docs/superpowers/plans/2026-07-15-restamp.md
RS_RESULTS_REL=docs/results/2026-07-15-restamp-results.md

rs_change(){
  cat <<'CF'
---
id: 70
slug: restamp
title: A change whose plan and results get back-linked
status: done
priority: medium
spec:
plan: docs/superpowers/plans/2026-07-15-restamp.md
results: docs/results/2026-07-15-restamp-results.md
adrs: []
---

## Why

Body.
CF
}

# rs_repo: prints "<work>" — a bare origin holding main (with the plan+results files + the archived
# record) and docket (the archived record carrying plan:/results:).
rs_repo(){
  local root origin work
  root="$(mktemp -d "$TP_ROOT/rs.XXXXXX")"; origin="$root/origin.git"; work="$root/work"
  git_quiet init --bare "$origin"
  git_quiet clone "$origin" "$work"
  git -C "$work" config user.email t@t; git -C "$work" config user.name t
  git_quiet -C "$work" checkout -b main
  mkdir -p "$work/docs/adrs" "$work/docs/changes/archive" "$work/docs/superpowers/plans" "$work/docs/results"
  echo "# adr index" > "$work/docs/adrs/README.md"
  printf '# Plan\n\nplan body.\n' > "$work/$RS_PLAN_REL"
  printf '# Results\n\nresults body.\n' > "$work/$RS_RESULTS_REL"
  rs_change > "$work/$RS_CHANGE_REL"
  git -C "$work" add -A; git_quiet -C "$work" commit -m "main baseline (plan+results present)"; git_quiet -C "$work" push -u origin main
  git_quiet -C "$work" checkout --orphan docket
  git_quiet -C "$work" rm -rf .
  mkdir -p "$work/docs/changes/archive" "$work/docs/adrs"
  rs_change > "$work/$RS_CHANGE_REL"
  git -C "$work" add -A; git_quiet -C "$work" commit -m "docket baseline"; git_quiet -C "$work" push -u origin docket
  printf '%s\n' "$work"
}

# (0136-a) an ENABLED publish stamps BOTH files, in ONE new commit on the integration branch.
RSW="$(rs_repo)"
rs_args=(--id 70 --outcome done --integration-branch main --metadata-branch docket
         --changes-dir docs/changes --adrs-dir docs/adrs --metadata-worktree "$RSW")
git_quiet -C "$RSW" fetch origin main
rs_before="$(git -C "$RSW" rev-list --count origin/main)"
( cd "$RSW" && DOCKET_CONFIG="$RS_CONFIG" bash "$SCRIPT" "${rs_args[@]}" --enabled true >/dev/null 2>&1 ); rsrc=$?
assert "restamp: enabled publish exits zero" '[ "$rsrc" -eq 0 ]'
git_quiet -C "$RSW" fetch origin main
rs_plan="$(git -C "$RSW" show "origin/main:$RS_PLAN_REL" 2>/dev/null)"
rs_results="$(git -C "$RSW" show "origin/main:$RS_RESULTS_REL" 2>/dev/null)"
assert "restamp: plan carries the back-link block on origin/main"    'grep -qF "docket:backlink:start" <<<"$rs_plan"'
assert "restamp: results carries the back-link block on origin/main" 'grep -qF "docket:backlink:start" <<<"$rs_results"'
assert "restamp: the block points at the archived change path"       'grep -qF "docs/changes/archive/2026-07-15-0070-restamp.md" <<<"$rs_plan"'
rs_after="$(git -C "$RSW" rev-list --count origin/main)"
assert "restamp: rides the single publish commit (no extra commit)"  '[ "$((rs_after - rs_before))" -eq 1 ]'

# (0136-b) a SUPPRESSED publish (--enabled false) leaves the plan/results untouched — the knob guard
# exits before any re-stamp can run.
RSW2="$(rs_repo)"
rs_args2=(--id 70 --outcome done --integration-branch main --metadata-branch docket
          --changes-dir docs/changes --adrs-dir docs/adrs --metadata-worktree "$RSW2")
( cd "$RSW2" && DOCKET_CONFIG="$RS_CONFIG" bash "$SCRIPT" "${rs_args2[@]}" --enabled false >/dev/null 2>&1 )
git_quiet -C "$RSW2" fetch origin main
rs2_plan="$(git -C "$RSW2" show "origin/main:$RS_PLAN_REL" 2>/dev/null)"
assert "restamp: --enabled false leaves plan untouched (no block)" '! grep -qF "docket:backlink:start" <<<"$rs2_plan"'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
