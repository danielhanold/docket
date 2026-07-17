#!/usr/bin/env bash
# tests/test_reclaim_claims.sh — verifies change 0089: scripts/reclaim-claims.sh, the deterministic
# claim-lease reclaim sweep. Reclaims a crashed in-progress change back to build-ready `proposed`
# ONLY in the provably-safe case: an EXPIRED claim lease AND no feature branch ref (the
# crashed-before-push blind spot). Hermetic: a temp repo with a local *bare* origin parked on the
# docket branch so the CAS push actually lands; only NOW is mocked (the branch-ref probe runs
# against real refs — feat/b is created, feat/a/c/f are left absent). Run: bash tests/test_reclaim_claims.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$REPO/scripts/reclaim-claims.sh"
# shellcheck source=/dev/null
. "$REPO/scripts/lib/docket-frontmatter.sh"   # field / iso_to_epoch for the assertions
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }
git_quiet(){ git "$@" >/dev/null 2>&1; }

# A fixed reference "now" (staleness must never depend on wall-clock); passed as NOW=$NOW_EPOCH.
NOW_EPOCH=1750000000
TTL=72
# iso EPOCH -> UTC ISO-8601 second-precision (BSD date first, then GNU) — builds claimed_at relative to NOW.
iso(){ date -u -r "$1" +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -d "@$1" +%Y-%m-%dT%H:%M:%SZ; }
EXPIRED="$(iso $(( NOW_EPOCH - 100*3600 )))"   # 100h old  > 72h TTL  => expired
FRESH="$(iso   $(( NOW_EPOCH -   1*3600 )))"   #   1h old  < 72h TTL  => fresh

# new_repo: prints the work-clone path — a fresh clone parked on the orphan docket branch with a
# local bare origin, so the script's change-file-only CAS `git push origin docket` succeeds.
new_repo(){
  local root origin work
  root="$(mktemp -d)"; origin="$root/origin.git"; work="$root/work"
  git_quiet init --bare "$origin"
  git_quiet clone "$origin" "$work"
  git -C "$work" config user.email t@t; git -C "$work" config user.name t
  git -C "$work" checkout --orphan docket >/dev/null 2>&1
  git -C "$work" rm -rf . >/dev/null 2>&1 || true
  mkdir -p "$work/docs/changes/active" "$work/docs/changes/archive"
  echo baseline > "$work/docs/changes/.keep"
  git -C "$work" add -A; git_quiet -C "$work" commit -m "docket baseline"
  git_quiet -C "$work" push -u origin docket
  printf '%s\n' "$work"
}

# mkchange WORK FILE STATUS BRANCH CLAIMED — write an active change file. CLAIMED="" omits claimed_at.
mkchange(){
  local work="$1" file="$2" status="$3" branch="$4" claimed="$5" id slug
  slug="${file%.md}"; slug="${slug#*-}"; id="$(( 10#${file%%-*} ))"
  {
    printf -- '---\n'
    printf 'id: %s\n'         "$id"
    printf 'slug: %s\n'       "$slug"
    printf 'title: %s\n'      "Change $slug"
    printf 'status: %s\n'     "$status"
    printf 'priority: medium\n'
    printf 'depends_on: []\n'
    printf 'branch: %s\n'     "$branch"
    [ -n "$claimed" ] && printf 'claimed_at: %s\n' "$claimed"
    printf 'reconciled: true\n'
    printf 'updated: 2026-07-13\n'
    printf -- '---\n\n'
    printf '# Change %s\n\n' "$slug"
    printf '## Artifacts\n\n- spec: (none)\n'
  } > "$work/docs/changes/active/$file"
}

assert "script exists and is executable" '[ -x "$SCRIPT" ]'

# ======================= the reclaim sweep: CASES A–F =======================
W="$(new_repo)"
mkchange "$W" 0001-a.md in-progress feat/a "$EXPIRED"   # A: expired, no branch ref     => reclaimed
mkchange "$W" 0002-b.md in-progress feat/b "$EXPIRED"   # B: expired, branch REF EXISTS  => untouched (orphan guard)
mkchange "$W" 0003-c.md in-progress feat/c "$FRESH"     # C: fresh lease                 => untouched
mkchange "$W" 0004-d.md in-progress feat/d ""           # D: NO claimed_at               => untouched (no evidence)
mkchange "$W" 0005-e.md proposed    ""      ""          # E: already proposed            => ignored
mkchange "$W" 0006-f.md in-progress feat/f "$EXPIRED"   # F: expired, no branch, sorts AFTER the skips => reclaimed
git_quiet -C "$W" branch feat/b docket                  # ONLY feat/b resolves to a ref
git -C "$W" add -A; git_quiet -C "$W" commit -m "fixtures"; git_quiet -C "$W" push origin docket

out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$W/docs/changes" --lease-ttl-hours "$TTL" 2>/dev/null)"; rc=$?

# --- CASE A: expired + no branch => reclaimed to build-ready proposed ---
assert "A expired + no branch is reclaimed to proposed" \
  '[ "$(field "$W/docs/changes/active/0001-a.md" status)" = proposed ]'
assert "A reclaim clears branch"      '[ -z "$(field "$W/docs/changes/active/0001-a.md" branch)" ]'
assert "A reclaim clears claimed_at"  '[ -z "$(field "$W/docs/changes/active/0001-a.md" claimed_at)" ]'
assert "A reclaim resets reconciled to false" \
  '[ "$(field "$W/docs/changes/active/0001-a.md" reconciled)" = false ]'
assert "A reclaim sets updated to UTC today" \
  '[ "$(field "$W/docs/changes/active/0001-a.md" updated)" = "$(date -u +%Y-%m-%d)" ]'
assert "A reclaim appends a Reclaim log section" \
  'grep -qF "## Reclaim log" "$W/docs/changes/active/0001-a.md"'
assert "A Reclaim log body binds age + TTL args correctly" \
  'grep -qF "~100h" "$W/docs/changes/active/0001-a.md" && grep -qF "TTL 72h" "$W/docs/changes/active/0001-a.md"'
assert "A reclaim preserves the Artifacts block (not regenerated)" \
  'grep -qF "## Artifacts" "$W/docs/changes/active/0001-a.md"'
assert "A reclaim reports the change on stdout" 'printf "%s" "$out" | grep -qE "^reclaimed 1 a \(lease 100h, no branch\)$"'
# CAS actually landed on origin (not just the working tree).
assert "A reclaim CAS-pushed to origin/docket" \
  'git -C "$W" show origin/docket:docs/changes/active/0001-a.md | grep -qxF "status: proposed"'

# --- CASE B: expired but a branch ref EXISTS => NOT reclaimed (orphan/collision guard) ---
assert "B expired + branch ref present is left in-progress" \
  '[ "$(field "$W/docs/changes/active/0002-b.md" status)" = in-progress ]'
assert "B is not reported"  '! printf "%s" "$out" | grep -qE "^reclaimed 2 "'

# --- CASE C: fresh lease => NOT reclaimed ---
assert "C fresh lease is left in-progress" \
  '[ "$(field "$W/docs/changes/active/0003-c.md" status)" = in-progress ]'

# --- CASE D: no claimed_at => NEVER reclaimed (no positive evidence of expiry) ---
assert "D no claimed_at is never reclaimed" \
  '[ "$(field "$W/docs/changes/active/0004-d.md" status)" = in-progress ]'

# --- CASE E: a proposed change is untouched ---
assert "E a proposed change is ignored" \
  '[ "$(field "$W/docs/changes/active/0005-e.md" status)" = proposed ]'

# --- CASE F: errexit hygiene — a reclaimable change sorting AFTER the B/C/D/E skips still processes ---
assert "F reclaimed after earlier skips (|| continue keeps the loop alive)" \
  '[ "$(field "$W/docs/changes/active/0006-f.md" status)" = proposed ]'
assert "F reclaim reported on stdout" 'printf "%s" "$out" | grep -qE "^reclaimed 6 f \(lease 100h, no branch\)$"'

# --- clean-sweep exit code + idempotency ---
assert "clean sweep exits 0" '[ "$rc" = 0 ]'
out2="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$W/docs/changes" --lease-ttl-hours "$TTL" 2>/dev/null)"; rc2=$?
assert "second sweep is idempotent (nothing eligible, empty stdout, exit 0)" \
  '[ -z "$out2" ] && [ "$rc2" = 0 ]'

# ======================= usage errors =======================
bash "$SCRIPT" --lease-ttl-hours 72 >/dev/null 2>&1; rc=$?
assert "missing --changes-dir is a hard error (exit != 0)" '[ "$rc" != 0 ]'
bash "$SCRIPT" --changes-dir "$W/docs/changes" >/dev/null 2>&1; rc=$?
assert "missing --lease-ttl-hours is a hard error (exit != 0)" '[ "$rc" != 0 ]'
bash "$SCRIPT" --changes-dir "$W/docs/changes" --lease-ttl-hours abc >/dev/null 2>&1; rc=$?
assert "non-numeric --lease-ttl-hours is a hard error (exit != 0)" '[ "$rc" != 0 ]'
bash "$SCRIPT" --changes-dir "$W/docs/changes/NOPE" --lease-ttl-hours 72 >/dev/null 2>&1; rc=$?
assert "non-existent --changes-dir is a hard error (exit != 0)" '[ "$rc" != 0 ]'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
