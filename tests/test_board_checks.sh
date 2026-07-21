#!/usr/bin/env bash
# tests/test_board_checks.sh — verifies change 0023: scripts/board-checks.sh, the mechanical
# docket-status health checks (broken-spec, broken-plan-results, dep-cycle, stale-in-progress,
# merge-gate-stall). Hermetic: a temp repo with a local *bare* origin carrying docket + main and
# a few feature branches; no gh, no network. Run: bash tests/test_board_checks.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$REPO/scripts/board-checks.sh"
SKILL="$REPO/skills/docket-status/SKILL.md"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }
git_quiet(){ git "$@" >/dev/null 2>&1; }

# has_finding OUTPUT CHECK-ID CHANGE-ID — exit 0 iff OUTPUT has a line beginning with the
# LITERAL "<check-id><TAB><change-id><TAB>" prefix.
#
# Matches literally, not as a regex. An earlier version built an ERE via
# `grep -qE "$(printf '^%s\t%s\t' "$2" "$3")"`, which let any ERE metacharacter in CHANGE-ID be
# reinterpreted as a pattern — most treacherously "?" (matches its preceding atom 0-or-1 times),
# which collapses `^check\t?\t` to effectively `^check\t`, i.e. TRUE for any change-id at all. And
# CHANGE-ID can legitimately BE the literal "?": it's the change-id column's fallback value when a
# filename yields no derivable id (see padded_id_from_file in scripts/board-checks.sh). A test
# author calling `has_finding "$out" some-check "?"` would silently get a vacuous, permanently-green
# assert with no signal it had happened. Matching via `case`/glob with the prefix double-quoted
# sidesteps this entirely: quoting a variable inside a case pattern makes its contents literal, so
# no argument value — "?", "*", "[", etc. — is ever reinterpreted as a pattern metacharacter.
#
# Also avoids piping a producer into an early-exiting consumer (this file runs under
# `set -uo pipefail`, so `printf ... | grep -q` is a real hazard, not just style): OUTPUT is
# consumed from a here-string, not a pipe, so an early `return` on match never races printf.
has_finding(){
  local out="$1" prefix line
  prefix="$2"$'\t'"$3"$'\t'
  while IFS= read -r line; do
    case "$line" in
      "$prefix"*) return 0 ;;
    esac
  done <<<"$out"
  return 1
}

# A fixed reference "now"; tests age commits relative to it and pass NOW=$NOW_EPOCH to the script
# so staleness never depends on wall-clock. (2026-06-15T13:20:00Z-ish; the exact value is irrelevant.)
NOW_EPOCH=1750000000

# new_repo: prints "<work> <origin>" — a fresh clone with a bare origin holding docket + main.
#   docket: docs/changes/active|archive + docs/superpowers/specs (committed specs).
#   main:   docs/superpowers/plans + docs/results (committed build artifacts).
# Callers add change files under $work/docs/changes/{active,archive}/ on the docket checkout,
# create feature branches as needed, then invoke the script against $work/docs/changes.
new_repo(){
  local root work origin
  root="$(mktemp -d)"; origin="$root/origin.git"; work="$root/work"
  git_quiet init --bare "$origin"
  git_quiet clone "$origin" "$work"
  git -C "$work" config user.email t@t; git -C "$work" config user.name t
  # --- main branch: build artifacts that 'done' changes link to ---
  git -C "$work" checkout -b main >/dev/null 2>&1
  mkdir -p "$work/docs/superpowers/plans" "$work/docs/results"
  echo "# plan"    > "$work/docs/superpowers/plans/2026-06-01-present.md"
  echo "# results" > "$work/docs/results/2026-06-01-present-results.md"
  git -C "$work" add -A; git_quiet -C "$work" commit -m "main artifacts"
  git_quiet -C "$work" push -u origin main
  # --- docket branch: orphan metadata ---
  git -C "$work" checkout --orphan docket >/dev/null 2>&1
  git -C "$work" rm -rf . >/dev/null 2>&1 || true
  mkdir -p "$work/docs/changes/active" "$work/docs/changes/archive" "$work/docs/superpowers/specs"
  echo "# present spec" > "$work/docs/superpowers/specs/2026-06-01-present.md"
  git -C "$work" add -A; git_quiet -C "$work" commit -m "docket metadata baseline"
  git_quiet -C "$work" push -u origin docket
  # leave the work clone parked on docket (the metadata working tree)
  printf '%s %s\n' "$work" "$origin"
}

# commit_present_spec_change: a helper used across tasks — writes a change file into active/.
# (Inline cat in each task is fine too; this keeps fixtures short.)

assert "script exists and is executable" '[ -x "$SCRIPT" ]'

# ============================ broken-spec ============================
# A change citing a spec absent on the metadata branch ⇒ one broken-spec finding.
# A change citing a present spec ⇒ silent. A trivial change with no spec ⇒ silent (carve-out).
read -r W _ < <(new_repo)
cat > "$W/docs/changes/active/0001-good.md" <<'EOF'
---
id: 1
slug: good
title: Good
status: proposed
priority: medium
depends_on: []
spec: docs/superpowers/specs/2026-06-01-present.md
trivial: false
EOF
cat > "$W/docs/changes/active/0002-missing.md" <<'EOF'
---
id: 2
slug: missing
title: Missing spec
status: proposed
priority: medium
depends_on: []
spec: docs/superpowers/specs/2026-06-01-ABSENT.md
trivial: false
EOF
cat > "$W/docs/changes/active/0003-trivial.md" <<'EOF'
---
id: 3
slug: trivial
title: Trivial, unresolvable spec
status: proposed
priority: medium
depends_on: []
spec: docs/superpowers/specs/2026-06-01-ABSENT.md
trivial: true
EOF
out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$W/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "broken-spec fires for a missing spec path (id 2)" 'has_finding "$out" broken-spec 2'
assert "broken-spec silent for a present spec (id 1)" '! has_finding "$out" broken-spec 1'
assert "broken-spec silent for a trivial change even with an unresolvable spec (id 3, carve-out)" '! has_finding "$out" broken-spec 3'

# ============================ clean tree + exit codes ============================
# A repo whose only change cites a present spec ⇒ no output, exit 0; --strict still exit 0.
read -r C _ < <(new_repo)
cat > "$C/docs/changes/active/0001-good.md" <<'EOF'
---
id: 1
slug: good
title: Good
status: proposed
priority: medium
depends_on: []
spec: docs/superpowers/specs/2026-06-01-present.md
trivial: false
EOF
clean="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$C/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "clean tree ⇒ empty stdout" '[ -z "$clean" ]'
NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$C/docs/changes" --metadata-branch docket --integration-branch main >/dev/null 2>&1; rc=$?
assert "clean tree ⇒ exit 0" '[ "$rc" = 0 ]'
NOW=$NOW_EPOCH bash "$SCRIPT" --strict --changes-dir "$C/docs/changes" --metadata-branch docket --integration-branch main >/dev/null 2>&1; rc=$?
assert "clean tree ⇒ --strict exit 0" '[ "$rc" = 0 ]'
# --strict on a finding ⇒ exit 1
NOW=$NOW_EPOCH bash "$SCRIPT" --strict --changes-dir "$W/docs/changes" --metadata-branch docket --integration-branch main >/dev/null 2>&1; rc=$?
assert "finding present ⇒ --strict exit 1" '[ "$rc" = 1 ]'
# without --strict, a finding still exits 0 (findings go to stdout; caller surfaces them)
NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$W/docs/changes" --metadata-branch docket --integration-branch main >/dev/null 2>&1; rc=$?
assert "finding present without --strict ⇒ exit 0" '[ "$rc" = 0 ]'

# ============================ usage errors ============================
bash "$SCRIPT" --metadata-branch docket --integration-branch main >/dev/null 2>&1; rc=$?
assert "missing --changes-dir ⇒ exit 2" '[ "$rc" = 2 ]'

# --lease-ttl-hours input validation (change 0089, Task 5 review carry-over): a non-numeric or
# negative value must `die` cleanly rather than crash the staleness arithmetic unbound. Mirrors
# reclaim-claims.sh's own `case "$TTL_HOURS" in ''|*[!0-9]*) die ...` guard. Fired UNCONDITIONALLY
# (before the change walk), so a clean repo with no in-progress change still rejects a bad value —
# the crash it prevents (`$(( abc * 3600 ))`) would otherwise only surface on repos that happen to
# carry an in-progress change, i.e. exactly when it is least expected.
bash "$SCRIPT" --changes-dir "$C/docs/changes" --metadata-branch docket --integration-branch main --lease-ttl-hours abc >/dev/null 2>&1; rc=$?
assert "non-numeric --lease-ttl-hours ⇒ exit 2" '[ "$rc" = 2 ]'
bash "$SCRIPT" --changes-dir "$C/docs/changes" --metadata-branch docket --integration-branch main --lease-ttl-hours -5 >/dev/null 2>&1; rc=$?
assert "negative --lease-ttl-hours ⇒ exit 2" '[ "$rc" = 2 ]'
bad_ttl_err="$(bash "$SCRIPT" --changes-dir "$C/docs/changes" --metadata-branch docket --integration-branch main --lease-ttl-hours abc 2>&1 >/dev/null)"
assert "invalid --lease-ttl-hours names the offending value on stderr" \
  'printf "%s" "$bad_ttl_err" | grep -qiF "lease-ttl-hours"'
# A valid integer still passes (no regression): the clean repo stays exit 0.
NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$C/docs/changes" --metadata-branch docket --integration-branch main --lease-ttl-hours 72 >/dev/null 2>&1; rc=$?
assert "valid integer --lease-ttl-hours ⇒ exit 0 (no regression)" '[ "$rc" = 0 ]'

# ============================ broken-plan-results ============================
# A 'done' change whose results: path is absent on the integration branch ⇒ one finding.
# The SAME missing field on an 'implemented' change ⇒ silent (carve-out). Present links ⇒ silent.
read -r P _ < <(new_repo)
cat > "$P/docs/changes/archive/2026-06-02-0010-donegood.md" <<'EOF'
---
id: 10
slug: donegood
title: Done, links present
status: done
priority: medium
depends_on: []
plan: docs/superpowers/plans/2026-06-01-present.md
results: docs/results/2026-06-01-present-results.md
EOF
cat > "$P/docs/changes/archive/2026-06-02-0011-donerot.md" <<'EOF'
---
id: 11
slug: donerot
title: Done, results link rotted
status: done
priority: medium
depends_on: []
plan: docs/superpowers/plans/2026-06-01-present.md
results: docs/results/2026-06-01-ABSENT-results.md
EOF
cat > "$P/docs/changes/active/0012-implmissing.md" <<'EOF'
---
id: 12
slug: implmissing
title: Implemented, plan not on integration yet
status: implemented
priority: medium
depends_on: []
plan: docs/superpowers/plans/2026-06-01-ABSENT.md
results:
EOF
pout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$P/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "broken-plan-results fires for a done change with a rotted results link (id 11)" \
  'has_finding "$pout" broken-plan-results 11'
assert "broken-plan-results silent for a done change with present links (id 10)" \
  '! has_finding "$pout" broken-plan-results 10'
assert "broken-plan-results silent for an implemented change with an absent plan (id 12, carve-out)" \
  '! has_finding "$pout" broken-plan-results 12'

# ============================ dep-cycle ============================
# A→B→A ⇒ a finding for EACH node (1 and 2). A self-loop C→C ⇒ a finding for C.
# A clean DAG (D→E, no back edge) ⇒ silent. A dangling depends_on (F→99 missing) ⇒ silent.
read -r G _ < <(new_repo)
mk(){ # mk ID SLUG DEPS  — minimal proposed change with a present spec (so broken-spec stays quiet)
  cat > "$G/docs/changes/active/$(printf '%04d' "$1")-$2.md" <<EOF
---
id: $1
slug: $2
title: $2
status: proposed
priority: medium
depends_on: [$3]
spec: docs/superpowers/specs/2026-06-01-present.md
trivial: false
EOF
}
mk 1 a 2
mk 2 b 1
mk 3 c 3
mk 4 d 5
mk 5 e ""
mk 6 f 99
gout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$G/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "dep-cycle fires for both nodes of A→B→A (id 1)" 'has_finding "$gout" dep-cycle 1'
assert "dep-cycle fires for both nodes of A→B→A (id 2)" 'has_finding "$gout" dep-cycle 2'
assert "dep-cycle fires for a self-loop (id 3)" 'has_finding "$gout" dep-cycle 3'
assert "dep-cycle silent for a DAG node (id 4)" '! has_finding "$gout" dep-cycle 4'
assert "dep-cycle silent for a DAG leaf (id 5)" '! has_finding "$gout" dep-cycle 5'
assert "dep-cycle silent for a dangling depends_on (id 6 → missing 99)" '! has_finding "$gout" dep-cycle 6'

# ============================ stale-in-progress ============================
# in-progress + branch with last commit 4 days old ⇒ finding. branch with a commit today ⇒ silent.
# in-progress + branch: set but branch absent ⇒ silent (carve-out).
# Change 0089 widens the signal: claimed_at + --lease-ttl-hours (default 72) also flags a change,
# catching the crashed-BEFORE-branch case the branch-age signal misses. At most one finding per change.
read -r S _ < <(new_repo)
STALE_EPOCH=$(( NOW_EPOCH - 4*86400 ))
FRESH_EPOCH=$(( NOW_EPOCH - 3600 ))
# iso EPOCH -> UTC ISO-8601 second-precision (BSD date first, then GNU) — builds claimed_at relative to NOW.
iso(){ date -u -r "$1" +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -d "@$1" +%Y-%m-%dT%H:%M:%SZ; }
LEASE_EXPIRED="$(iso $(( NOW_EPOCH - 100*3600 )))"   # 100h old  > default 72h TTL  => expired
LEASE_FRESH="$(iso   $(( NOW_EPOCH -   1*3600 )))"   #   1h old  < default 72h TTL  => fresh
# feat/stale — aged commit
git -C "$S" checkout -b feat/stale >/dev/null 2>&1
echo x > "$S/x"; git -C "$S" add x
GIT_AUTHOR_DATE="@$STALE_EPOCH +0000" GIT_COMMITTER_DATE="@$STALE_EPOCH +0000" git_quiet -C "$S" commit -m "aged"
# feat/fresh — commit "now"
git -C "$S" checkout -b feat/fresh docket >/dev/null 2>&1
echo y > "$S/y"; git -C "$S" add y
GIT_AUTHOR_DATE="@$FRESH_EPOCH +0000" GIT_COMMITTER_DATE="@$FRESH_EPOCH +0000" git_quiet -C "$S" commit -m "fresh"
git -C "$S" checkout docket >/dev/null 2>&1
cat > "$S/docs/changes/active/0020-stale.md" <<'EOF'
---
id: 20
slug: stale
title: Stale claim
status: in-progress
priority: medium
depends_on: []
branch: feat/stale
EOF
cat > "$S/docs/changes/active/0021-fresh.md" <<'EOF'
---
id: 21
slug: fresh
title: Fresh claim
status: in-progress
priority: medium
depends_on: []
branch: feat/fresh
EOF
cat > "$S/docs/changes/active/0022-justclaimed.md" <<'EOF'
---
id: 22
slug: justclaimed
title: Just claimed, no branch yet
status: in-progress
priority: medium
depends_on: []
branch: feat/justclaimed
EOF
# id 23: expired lease (100h), NO branch ref resolves ⇒ reclaimable (the crashed-BEFORE-branch case).
cat > "$S/docs/changes/active/0023-expirednobranch.md" <<EOF
---
id: 23
slug: expirednobranch
title: Expired lease, no branch
status: in-progress
priority: medium
depends_on: []
branch: feat/expirednobranch
claimed_at: $LEASE_EXPIRED
EOF
# id 24: expired lease (100h), branch ref EXISTS (feat/fresh, recent commit ⇒ not idle) ⇒ flagged,
# but NOT reclaimable (a live implementer may hold it — needs human review).
cat > "$S/docs/changes/active/0024-expiredwithbranch.md" <<EOF
---
id: 24
slug: expiredwithbranch
title: Expired lease, branch exists
status: in-progress
priority: medium
depends_on: []
branch: feat/fresh
claimed_at: $LEASE_EXPIRED
EOF
# id 25: fresh lease (1h), no branch ⇒ silent (no expiry, no idle branch).
cat > "$S/docs/changes/active/0025-freshnobranch.md" <<EOF
---
id: 25
slug: freshnobranch
title: Fresh lease, no branch
status: in-progress
priority: medium
depends_on: []
branch: feat/freshnobranch
claimed_at: $LEASE_FRESH
EOF
# id 26: branch idle >3d (feat/stale) AND lease expired ⇒ exactly ONE finding (the preserved
# branch-idle message wins priority over the expired-with-branch message).
cat > "$S/docs/changes/active/0026-idleandexpired.md" <<EOF
---
id: 26
slug: idleandexpired
title: Branch idle AND lease expired
status: in-progress
priority: medium
depends_on: []
branch: feat/stale
claimed_at: $LEASE_EXPIRED
EOF
sout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$S/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "stale-in-progress fires for a branch idle >3 days (id 20)" \
  'has_finding "$sout" stale-in-progress 20'
assert "id 20 branch-idle message text is unchanged (regression)" \
  'printf "%s" "$sout" | grep -E "$(printf "^stale-in-progress\t20\t")" | grep -qF "branch feat/stale idle >3 days"'
assert "stale-in-progress silent for a branch with a recent commit (id 21)" \
  '! has_finding "$sout" stale-in-progress 21'
assert "stale-in-progress silent when branch: set but branch absent (id 22, carve-out)" \
  '! has_finding "$sout" stale-in-progress 22'
assert "stale-in-progress fires for expired lease + no branch (id 23)" \
  'has_finding "$sout" stale-in-progress 23'
assert "id 23 finding carries the [reclaimable] marker" \
  'printf "%s" "$sout" | grep -E "$(printf "^stale-in-progress\t23\t")" | grep -qF "[reclaimable]"'
assert "id 23 message reports age in hours (100h)" \
  'printf "%s" "$sout" | grep -E "$(printf "^stale-in-progress\t23\t")" | grep -qF "100h ago"'
assert "stale-in-progress fires for expired lease + branch ref exists (id 24)" \
  'has_finding "$sout" stale-in-progress 24'
assert "id 24 finding does NOT carry [reclaimable] (branch exists ⇒ needs review, not auto-reclaimable)" \
  '! (printf "%s" "$sout" | grep -E "$(printf "^stale-in-progress\t24\t")" | grep -qF "[reclaimable]")'
assert "id 24 message names the branch and says not auto-reclaimable" \
  'printf "%s" "$sout" | grep -E "$(printf "^stale-in-progress\t24\t")" | grep -qF "branch feat/fresh" \
   && printf "%s" "$sout" | grep -E "$(printf "^stale-in-progress\t24\t")" | grep -qF "not auto-reclaimable"'
assert "stale-in-progress silent for a fresh lease with no branch (id 25)" \
  '! has_finding "$sout" stale-in-progress 25'
assert "stale-in-progress emits exactly one finding when both branch-idle and lease-expired apply (id 26)" \
  '[ "$(printf "%s" "$sout" | grep -cE "$(printf "^stale-in-progress\t26\t")")" = 1 ]'
assert "id 26 finding is the branch-idle message, not the reclaimable/expired one (priority: branch-idle wins)" \
  'printf "%s" "$sout" | grep -E "$(printf "^stale-in-progress\t26\t")" | grep -qF "idle >3 days" \
   && ! (printf "%s" "$sout" | grep -E "$(printf "^stale-in-progress\t26\t")" | grep -qF "[reclaimable]")'
# --lease-ttl-hours override: id 25's 1h-old lease is silent under the default 72h TTL (asserted
# above) but IS flagged under an explicit --lease-ttl-hours 0 — proves the flag is actually wired.
touts="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$S/docs/changes" --metadata-branch docket --integration-branch main --lease-ttl-hours 0 2>/dev/null)"
assert "--lease-ttl-hours overrides the default: a 1h-old lease is flagged under TTL=0 (id 25)" \
  'has_finding "$touts" stale-in-progress 25'

# ============================ merge-gate-stall ============================
# A build-ready change depends_on a change at 'implemented' ⇒ finding naming that dep.
# A build-ready change depends_on a change still 'proposed' (not yet built) ⇒ NOT a merge-gate-stall.
read -r M _ < <(new_repo)
cat > "$M/docs/changes/active/0030-impl.md" <<'EOF'
---
id: 30
slug: impl
title: Implemented dep
status: implemented
priority: medium
depends_on: []
pr: https://github.com/o/r/pull/9
EOF
cat > "$M/docs/changes/active/0031-waiter.md" <<'EOF'
---
id: 31
slug: waiter
title: Build-ready, waiting on a merge
status: proposed
priority: medium
depends_on: [30]
spec: docs/superpowers/specs/2026-06-01-present.md
trivial: false
EOF
cat > "$M/docs/changes/active/0032-unbuilt.md" <<'EOF'
---
id: 32
slug: unbuilt
title: Unbuilt dep
status: proposed
priority: medium
depends_on: []
spec: docs/superpowers/specs/2026-06-01-present.md
trivial: false
EOF
cat > "$M/docs/changes/active/0033-waiter2.md" <<'EOF'
---
id: 33
slug: waiter2
title: Waiting on a not-yet-built dep
status: proposed
priority: medium
depends_on: [32]
spec: docs/superpowers/specs/2026-06-01-present.md
trivial: false
EOF
mout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$M/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "merge-gate-stall fires for a build-ready change waiting on an implemented dep (id 31)" \
  'has_finding "$mout" merge-gate-stall 31'
assert "merge-gate-stall names the blocking dep #30" \
  'printf "%s" "$mout" | grep -E "$(printf "^merge-gate-stall\t31\t")" | grep -qF "#30"'
assert "merge-gate-stall silent for a change waiting on a not-yet-built dep (id 33)" \
  '! has_finding "$mout" merge-gate-stall 33'

# ============ malformed-id + findings-channel sanitization (change 0104, spec part 3) ============
# The change-id column is the field docket-status.sh splits on
# (`IFS=$'\t' read -r check_id change_id message`). It must NEVER carry a raw frontmatter value.
# Pre-0104 the malformed-id emit put `$raw` there verbatim, so a TAB in `id:` shifted the message
# into the wrong field — the guard's own channel injectable by the input class it exists to catch.
read -r work origin <<<"$(new_repo)"
printf -- '---\nid: abc\nslug: bad\ntitle: Bad\nstatus: proposed\npriority: low\ndepends_on: []\n---\n' > "$work/docs/changes/active/0001-bad.md"
out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$work/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
# The check still fires — but keyed on the FILENAME-derived id, not the raw value. (This assert
# replaces the pre-0104 `has_finding "$out" malformed-id abc`: what the block GUARDS is "a
# non-integer id is flagged", and that is preserved; only the column the raw value lands in moved.)
assert "malformed-id fires on a non-integer id, keyed on the filename-derived id" \
  'has_finding "$out" malformed-id 0001'
assert "malformed-id no longer keys the change-id column on the raw frontmatter value" \
  '! has_finding "$out" malformed-id abc'
assert "the raw value survives in the MESSAGE column (diagnosis is not lost)" \
  'printf "%s" "$out" | grep -qF "non-integer id '"'"'abc'"'"'"'

# TAB injection: an interior TAB in id: must not shift the message into the change-id field.
read -r work2 _ <<<"$(new_repo)"
printf -- '---\nid: 4\tEVIL\nslug: tabby\ntitle: Tabby\nstatus: proposed\npriority: low\ndepends_on: []\n---\n' > "$work2/docs/changes/active/0002-tabby.md"
tout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$work2/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
# Read the finding back exactly the way docket-status.sh does; all three columns must survive.
IFS=$'\t' read -r t_check t_id t_msg <<<"$(printf '%s' "$tout" | grep '^malformed-id')"
assert "TAB-in-id: check_id column survives the caller's IFS split" '[ "$t_check" = "malformed-id" ]'
assert "TAB-in-id: change-id column is the filename id, not a fragment of the raw value" '[ "$t_id" = "0002" ]'
assert "TAB-in-id: the message column is non-empty (not shifted into the id field)" '[ -n "$t_msg" ]'
assert "TAB-in-id: the embedded TAB is escaped to a visible \\t, not passed through raw" \
  'printf "%s" "$t_msg" | grep -qF "4\\tEVIL"'

# An archive filename (<date>-<id>-<slug>.md) still yields its id.
read -r work3 _ <<<"$(new_repo)"
printf -- '---\nid: xyz\nslug: arch\ntitle: Arch\nstatus: done\npriority: low\ndepends_on: []\n---\n' > "$work3/docs/changes/archive/2026-06-16-0012-arch.md"
aout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$work3/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "archive filenames yield their padded id for the change-id column" 'has_finding "$aout" malformed-id 0012'

# A filename with no derivable id falls back to `?` rather than emitting an empty column.
# has_finding now matches the change-id column literally (see its definition above), so passing
# the literal "?" here is safe and no longer needs the IFS-extraction workaround this assert used
# to require: "?" cannot be reinterpreted as a pattern, and this is genuinely discriminating —
# it fails if the implementation ever emits anything other than the "?" fallback.
read -r work4 _ <<<"$(new_repo)"
printf -- '---\nid: nope\nslug: weird\ntitle: Weird\nstatus: proposed\npriority: low\ndepends_on: []\n---\n' > "$work4/docs/changes/active/no-leading-id.md"
wout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$work4/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "an id-less filename falls back to '?' in the change-id column" 'has_finding "$wout" malformed-id "?"'

# ============================ field-domain (change 0104, spec part 1) ============================
# A value that is well-formed TEXT but outside its field's DOMAIN. Validated by shape/membership,
# never by enumerating bad strings — the spelling you enumerate is never the one that arrives.
read -r F _ < <(new_repo)
mk_fd(){ # mk_fd FILE-BASENAME ID SLUG TITLE STATUS PRIORITY
  printf -- '---\nid: %s\nslug: %s\ntitle: %s\nstatus: %s\npriority: %s\ndepends_on: []\n---\n' \
    "$2" "$3" "$4" "$5" "$6" > "$F/docs/changes/active/$1"
}
mk_fd 0040-clean.md    40 clean    "Clean change"  proposed            medium
mk_fd 0041-poison.md   41 poison   "Poisoned"      "proposed  # awaiting X" medium
mk_fd 0042-badslug.md  42 "bad slug" "Bad slug"    proposed            medium
mk_fd 0043-badprio.md  43 badprio  "Bad priority"  proposed            urgent
mk_fd 0044-pipe.md     44 pipe     "T5 | injected | row" proposed      medium
mk_fd 0045-emptyprio.md 45 emptyprio "Empty priority" proposed         ""
printf -- '---\nid: 46\nslug: nostatus\ntitle: No status\nstatus:\npriority: medium\ndepends_on: []\n---\n' \
  > "$F/docs/changes/active/0046-nostatus.md"
fout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$F/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"

assert "field-domain silent for a wholly clean change (id 40)"      '! has_finding "$fout" field-domain 40'
assert "field-domain fires for a status carrying an inline comment (id 41)" 'has_finding "$fout" field-domain 41'
assert "field-domain fires for a slug with a space (id 42)"          'has_finding "$fout" field-domain 42'
assert "field-domain fires for an unrecognized priority (id 43)"     'has_finding "$fout" field-domain 43'
assert "field-domain fires for a title containing a pipe (id 44)"    'has_finding "$fout" field-domain 44'
# The documented default: an EMPTY priority is LEGAL (convention says medium; the sort implements
# it). This assert is what keeps the domain check from becoming over-eager.
assert "field-domain SILENT for an empty priority (id 45, documented default)" \
  '! has_finding "$fout" field-domain 45'
assert "field-domain fires for an EMPTY status (id 46, no documented default)" \
  'has_finding "$fout" field-domain 46'

# Messages name the field and quote the offending value, so a reader can act without opening the file.
assert "the status finding names the field and the offending value" \
  'printf "%s" "$fout" | grep -qF "status '"'"'proposed  # awaiting X'"'"'"'
assert "the title finding names the pipe as the board-row hazard" \
  'printf "%s" "$fout" | grep -E "$(printf "^field-domain\t44\t")" | grep -qF "title"'

# Shape, not spelling: a slug with a TAB and a slug with an uppercase letter both fire, though
# neither is an enumerated bad value.
mk_fd 0047-tabslug.md 47 "$(printf 'tab\tslug')" "Tab slug" proposed medium
mk_fd 0048-upper.md   48 UpperSlug "Upper slug"   proposed medium
sout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$F/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "field-domain fires for a slug containing a TAB (shape check, id 47)"  'has_finding "$sout" field-domain 47'
assert "field-domain fires for an uppercase slug (shape check, id 48)"        'has_finding "$sout" field-domain 48'
assert "a TAB inside a slug value cannot shift the findings line's columns (id 47)" \
  'printf "%s" "$sout" | grep -E "$(printf "^field-domain\t47\t")" | grep -qF "\\t"'

# Warn-only posture is preserved: findings present, exit still 0 without --strict.
NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$F/docs/changes" --metadata-branch docket --integration-branch main >/dev/null 2>&1
assert "field-domain findings do not change the default exit status (warn-only)" '[ "$?" = 0 ]'

# The archive is walked too — a terminal status is in the vocabulary and must stay silent.
read -r G _ < <(new_repo)
printf -- '---\nid: 60\nslug: archived\ntitle: Archived\nstatus: done\npriority: medium\ndepends_on: []\n---\n' \
  > "$G/docs/changes/archive/2026-06-16-0060-archived.md"
gout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$G/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "field-domain silent for a terminal status in archive/ (id 60)" '! has_finding "$gout" field-domain 60'

# ======================= board-row-dropped (change 0104, spec part 2) =======================
# The invariant: an ACTIVE file counted in render-board.sh's `total` but rendered in no section.
# The trigger is COMPUTED (renders_row mirrors the renderer's bucketing), not enumerated per drop
# cause — case (f) below is the case no enumerated check can see. SUPPRESSED only by a finding that
# genuinely explains the DISAPPEARANCE (malformed-id, or field-domain on `status`); a bad
# slug/priority/title must NOT suppress, which case (g) pins.
read -r D _ < <(new_repo)
# (a) the live un-suppressed trigger: NO id: field at all. malformed-id needs a non-empty raw
#     value, so nothing explains this drop.
printf -- '---\nslug: noid\ntitle: No id\nstatus: proposed\npriority: medium\ndepends_on: []\n---\n' \
  > "$D/docs/changes/active/0070-noid.md"
dout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$D/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "board-row-dropped fires for an active file with no id: field (0070)" \
  'has_finding "$dout" board-row-dropped 0070'

# (b) suppression by field-domain: a poisoned status yields EXACTLY ONE finding for that id.
read -r E _ < <(new_repo)
printf -- '---\nid: 71\nslug: poison\ntitle: Poisoned\nstatus: proposed  # awaiting X\npriority: medium\ndepends_on: []\n---\n' \
  > "$E/docs/changes/active/0071-poison.md"
eout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$E/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
n71="$(printf '%s' "$eout" | grep -c .)"
# A REAL suppression decision, not a self-cancelling pair: the DROPPED entry for 71 is written by the
# computed predicate (renders_row — an unrecognized status is outside DOCKET_STATUSES_ACTIVE), while
# EXPLAINED is marked by the field-domain `status` arm. They are populated at independent sites, so
# deleting the arm's `EXPLAINED[...]=1` reddens this assert with a second (board-row-dropped) finding.
assert "a poisoned status yields exactly ONE finding, not two (suppression works)" '[ "$n71" = 1 ]'
assert "and that one finding is field-domain, not board-row-dropped" 'has_finding "$eout" field-domain 71'
assert "board-row-dropped is suppressed when field-domain explains the drop" \
  '! has_finding "$eout" board-row-dropped 71'

# (c) suppression by malformed-id.
read -r H _ < <(new_repo)
printf -- '---\nid: abc\nslug: badid\ntitle: Bad id\nstatus: proposed\npriority: medium\ndepends_on: []\n---\n' \
  > "$H/docs/changes/active/0072-badid.md"
hout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$H/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "board-row-dropped is suppressed when malformed-id explains the drop (0072)" \
  '! has_finding "$hout" board-row-dropped 0072'
assert "malformed-id still fires for that file (0072)" 'has_finding "$hout" malformed-id 0072'

# (d) archive/ is NOT subject to the invariant — the archive table renders from its own pass.
read -r I _ < <(new_repo)
printf -- '---\nslug: archnoid\ntitle: Arch no id\nstatus: done\npriority: medium\ndepends_on: []\n---\n' \
  > "$I/docs/changes/archive/2026-06-16-0073-archnoid.md"
iout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$I/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "board-row-dropped does not fire for an archive/ file (0073)" \
  '! has_finding "$iout" board-row-dropped 0073'

# (e) a wholly clean tree stays silent — the backstop must not fire on healthy repos.
read -r J _ < <(new_repo)
printf -- '---\nid: 74\nslug: fine\ntitle: Fine\nstatus: proposed\npriority: medium\ndepends_on: []\n---\n' \
  > "$J/docs/changes/active/0074-fine.md"
jout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$J/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "clean active tree emits no board-row-dropped finding" '! printf "%s" "$jout" | grep -q "^board-row-dropped"'

# (f) THE COMPUTED-PREDICATE CASE: an active/ file carrying a TERMINAL status (`done`). Every
# ENUMERATED check is correctly silent — `done` is in DOCKET_STATUSES so field-domain passes it, and
# the id is a well-formed integer so malformed-id passes it — yet render-board.sh counts the file in
# `total` (:86) and calls print_section only for the five ACTIVE statuses (:265-269), so the row is
# rendered nowhere and the board's count line disagrees with its tables. Only an invariant computed
# from DOCKET_STATUSES_ACTIVE sees this; a predicate written against DOCKET_STATUSES cannot.
# Reachable in practice: docket-status's `sweep-failed <id> archive <reason>` is exactly this state
# (status flipped to done, archive move failed).
read -r K _ < <(new_repo)
printf -- '---\nid: 75\nslug: fine\ntitle: Fine\nstatus: proposed\npriority: medium\ndepends_on: []\n---\n' \
  > "$K/docs/changes/active/0075-fine.md"
printf -- '---\nid: 76\nslug: stuck\ntitle: Stuck in active\nstatus: done\npriority: medium\ndepends_on: []\n---\n' \
  > "$K/docs/changes/active/0076-stuck.md"
kout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$K/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "board-row-dropped fires for an active/ file with a TERMINAL status (76)" \
  'has_finding "$kout" board-row-dropped 76'
# NOT suppressed — and the reason matters: there is no field-domain finding to suppress it with.
# `done` is a legal status; it is merely legal in the wrong directory.
assert "the terminal-in-active drop is NOT explained by field-domain (done is a legal status)" \
  '! has_finding "$kout" field-domain 76'
assert "the terminal-in-active drop is NOT explained by malformed-id (76 is a valid id)" \
  '! has_finding "$kout" malformed-id 76'
assert "the healthy sibling (75) draws no board-row-dropped finding" \
  '! has_finding "$kout" board-row-dropped 75'
# The same terminal status in archive/ — where it belongs — stays silent (the archive renders from
# its own pass). Keeps the predicate from degenerating into "done is always wrong".
read -r L _ < <(new_repo)
printf -- '---\nid: 77\nslug: archived\ntitle: Archived\nstatus: done\npriority: medium\ndepends_on: []\n---\n' \
  > "$L/docs/changes/archive/2026-06-16-0077-archived.md"
lout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$L/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "the same terminal status in archive/ draws NO board-row-dropped finding (77)" \
  '! has_finding "$lout" board-row-dropped 77'

# (g) FALSE-SUPPRESSION GUARD: a violation that does NOT explain a drop must not silence the
# backstop. This file both drops (terminal status in active/) and carries a piped title — a piped
# title INJECTS columns into a row that is still emitted, so it explains nothing about the row's
# disappearance. Marking EXPLAINED from the slug/priority/title arms reddens this pair.
read -r M _ < <(new_repo)
printf -- '---\nid: 78\nslug: both\ntitle: Dropped | and | piped\nstatus: done\npriority: medium\ndepends_on: []\n---\n' \
  > "$M/docs/changes/active/0078-both.md"
mout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$M/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "a piped title fires field-domain (78)" 'has_finding "$mout" field-domain 78'
assert "a piped title does NOT suppress board-row-dropped on a row that really dropped (78)" \
  'has_finding "$mout" board-row-dropped 78'
# Same shape for the other two non-explaining arms, so no single arm can regress unnoticed.
read -r N _ < <(new_repo)
printf -- '---\nid: 79\nslug: Bad Slug\ntitle: Bad slug and dropped\nstatus: killed\npriority: urgent\ndepends_on: []\n---\n' \
  > "$N/docs/changes/active/0079-badslug.md"
nout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$N/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "a bad slug + bad priority do NOT suppress board-row-dropped on a dropped row (79)" \
  'has_finding "$nout" board-row-dropped 79'
assert "field-domain still reports the slug/priority violations alongside it (79)" \
  'has_finding "$nout" field-domain 79'

# ============================ merged-orphan / unknown-commit-ref ============================
# Cross-reference change ids in integration-branch (main) commit *subjects* against active/archive.
# All fixtures use --allow-empty commits (subjects only). Each negative is discriminating: it pairs
# a real change file with a real commit, so the excluded grammar (bare #, body text) or the
# active/archive carve-out is what keeps the finding from firing.
read -r O _ < <(new_repo)
# --- craft integration-branch (main) history: subjects only, via empty commits ---
git -C "$O" checkout main >/dev/null 2>&1
git_quiet -C "$O" commit --allow-empty -m "docket(0050): merged-orphan via conventional scope"
git_quiet -C "$O" commit --allow-empty -m "feat: add a thing (change 0054)"      # trailing form
git_quiet -C "$O" commit --allow-empty -m "Fix a thing #51"                       # bare # only (excluded)
git_quiet -C "$O" commit --allow-empty -m "unrelated subject" -m "body mentions (change 52)"  # body only (excluded)
git_quiet -C "$O" commit --allow-empty -m "docket(0053): terminal record — done" # id 53 is archived
git_quiet -C "$O" commit --allow-empty -m "docket(0099): mystery id with no file" # unknown ref
git -C "$O" checkout docket >/dev/null 2>&1
# --- change files: 50/51/52/54 active, 53 archived (done), 99 absent ---
for pair in 50:orphan 51:barehash 52:bodyonly 54:trailing; do
  id="${pair%%:*}"; slug="${pair##*:}"
  cat > "$O/docs/changes/active/00$id-$slug.md" <<EOF
---
id: $id
slug: $slug
title: $slug
status: in-progress
priority: medium
depends_on: []
---
EOF
done
cat > "$O/docs/changes/archive/2026-07-01-0053-published.md" <<'EOF'
---
id: 53
slug: published
title: Terminal, published
status: done
priority: medium
depends_on: []
---
EOF
oout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$O/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
# merged-orphan: active id referenced by a merged subject (both grammar forms)
assert "merged-orphan fires for an active id in a conventional scope docket(0050) (id 50)" \
  'has_finding "$oout" merged-orphan 50'
assert "merged-orphan fires for an active id in the trailing (change 0054) form (id 54)" \
  'has_finding "$oout" merged-orphan 54'
# negatives — each discriminating (the id HAS an active file; only the excluded grammar keeps it quiet)
assert "merged-orphan silent for a bare #51 reference (grammar excludes bare #, id 51)" \
  '! has_finding "$oout" merged-orphan 51'
assert "merged-orphan silent for a body-only (change 52) mention (subjects only, id 52)" \
  '! has_finding "$oout" merged-orphan 52'
assert "merged-orphan silent for a docket(0053) subject of an ARCHIVED change (carve-out, id 53)" \
  '! has_finding "$oout" merged-orphan 53'
# unknown-commit-ref: referenced id with no change file at all
assert "unknown-commit-ref fires for docket(0099) with no change file (id 99)" \
  'has_finding "$oout" unknown-commit-ref 99'
# unknown-commit-ref must NOT fire for ids that DO have a file (active or archived)
assert "unknown-commit-ref silent for a known active id (id 50)" \
  '! has_finding "$oout" unknown-commit-ref 50'
assert "unknown-commit-ref silent for a known archived id (id 53)" \
  '! has_finding "$oout" unknown-commit-ref 53'
# evidence: the merged-orphan message names the evidence commit subject
assert "merged-orphan names the evidence commit for id 50" \
  'printf "%s" "$oout" | grep -E "$(printf "^merged-orphan\t50\t")" | grep -qF "docket(0050)"'

# ============================ stale-finalize-blocked ============================
# An 'implemented' change carrying `## Finalize blocked` whose change-file's last commit is older
# than the hardcoded 72h horizon ⇒ one stale-finalize-blocked finding. A recent last commit ⇒
# silent. No marker ⇒ silent. A non-implemented status carrying a stray marker ⇒ silent (the
# status==implemented gate). Marker age is the change file's git commit timestamp (the
# GIT_AUTHOR/COMMITTER_DATE seams below), never a model-authored in-body date. Hermetic: NOW pinned.
read -r FB _ < <(new_repo)
FB_STALE_EPOCH=$(( NOW_EPOCH - 100*3600 ))   # 100h old  > 72h horizon => stale
FB_FRESH_EPOCH=$(( NOW_EPOCH -   1*3600 ))   #   1h old  < 72h horizon => fresh
# id 40: implemented + marker, file committed 100h ago ⇒ fires.
cat > "$FB/docs/changes/active/0040-staleblocked.md" <<'EOF'
---
id: 40
slug: staleblocked
title: Implemented, finalize-blocked, stale marker
status: implemented
priority: medium
depends_on: []
pr: https://github.com/o/r/pull/40
---

## Finalize blocked

### 2026-01-01 — gate failure
Rebase onto main hit a conflict; a human must resolve.
EOF
# id 41: implemented + marker, file committed 1h ago ⇒ silent.
cat > "$FB/docs/changes/active/0041-freshblocked.md" <<'EOF'
---
id: 41
slug: freshblocked
title: Implemented, finalize-blocked, fresh marker
status: implemented
priority: medium
depends_on: []
pr: https://github.com/o/r/pull/41
---

## Finalize blocked

### 2026-07-19 — gate failure
Rebase onto main hit a conflict; just marked.
EOF
# id 42: implemented, NO marker ⇒ silent (even though committed 100h ago).
cat > "$FB/docs/changes/active/0042-nomarker.md" <<'EOF'
---
id: 42
slug: nomarker
title: Implemented, no finalize-blocked marker
status: implemented
priority: medium
depends_on: []
pr: https://github.com/o/r/pull/42
---

## Why
Nothing blocked here.
EOF
# id 43: in-progress carrying a STRAY marker, file committed 100h ago ⇒ silent (status gate).
cat > "$FB/docs/changes/active/0043-wrongstatus.md" <<'EOF'
---
id: 43
slug: wrongstatus
title: In-progress with a stray finalize-blocked marker
status: in-progress
priority: medium
depends_on: []
branch: feat/wrongstatus
---

## Finalize blocked

### 2026-01-01 — stray
Should not fire — status is not implemented.
EOF
# Commit the stale-dated fixtures (40/42/43) at 100h-old, then the fresh fixture (41) at 1h-old.
# `git log -1 --format=%ct -- <file>` resolves each file's own last-touching commit, so the two
# commits' dates attach per-file regardless of global commit ordering.
git -C "$FB" add docs/changes/active/0040-staleblocked.md \
                 docs/changes/active/0042-nomarker.md \
                 docs/changes/active/0043-wrongstatus.md
GIT_AUTHOR_DATE="@$FB_STALE_EPOCH +0000" GIT_COMMITTER_DATE="@$FB_STALE_EPOCH +0000" \
  git_quiet -C "$FB" commit -m "fb: stale fixtures"
git -C "$FB" add docs/changes/active/0041-freshblocked.md
GIT_AUTHOR_DATE="@$FB_FRESH_EPOCH +0000" GIT_COMMITTER_DATE="@$FB_FRESH_EPOCH +0000" \
  git_quiet -C "$FB" commit -m "fb: fresh fixture"
fbout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$FB/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "stale-finalize-blocked fires for an implemented change with a stale marker (id 40)" \
  'has_finding "$fbout" stale-finalize-blocked 40'
assert "stale-finalize-blocked message reports the marker age in hours (100h)" \
  'printf "%s" "$fbout" | grep -E "$(printf "^stale-finalize-blocked\t40\t")" | grep -qF "100h"'
assert "stale-finalize-blocked message names re-run finalize with the id (id 40)" \
  'printf "%s" "$fbout" | grep -E "$(printf "^stale-finalize-blocked\t40\t")" | grep -qF "finalize 40"'
assert "stale-finalize-blocked silent for a recent marker (id 41)" \
  '! has_finding "$fbout" stale-finalize-blocked 41'
assert "stale-finalize-blocked silent for an implemented change without the marker (id 42)" \
  '! has_finding "$fbout" stale-finalize-blocked 42'
assert "stale-finalize-blocked silent for a non-implemented change carrying a stray marker (id 43, status gate)" \
  '! has_finding "$fbout" stale-finalize-blocked 43'

# ============================ docket-status wiring sentinels (SKILL is code on main) ============================
assert "docket-status Health checks invoke board-checks (via the docket.sh facade)" \
  'grep -qF "docket.sh board-checks" "$SKILL"'
# The five mechanical checks are now delegated — their old standalone bullets are gone as bullets,
# but the SKILL still names them so a reader knows what the script covers. Assert the surviving
# model-driven signals, each anchored to a phrase it owns: the blocked_by re-examination
# (judgment) and the github mirror-reachability visibility flag. Change 0024 retired the inline
# board/source-drift check (deterministic render + the unconditional Board-pass re-render make it
# vacuous); its removed tripwire lives in tests/test_board_refresh_on_transition.sh.
assert "docket-status keeps blocked_by re-examination model-driven" \
  'grep -qiF "blocked_by:" "$SKILL"'
assert "docket-status keeps the github mirror-reachability visibility flag (survives 0024 inline-drift retirement)" \
  'grep -qiF "mirror reachability" "$SKILL" || grep -qiF "mirror-reachability" "$SKILL"'
assert "docket-status keeps the do-not-auto-fix stance" \
  'grep -qiF "do not auto-fix" "$SKILL"'
# Mutation guard: the board-checks invocation passes the changes-dir + both branch refs.
assert "the board-checks invocation passes --changes-dir" 'grep -qF -- "--changes-dir" "$SKILL"'
assert "the board-checks invocation passes --metadata-branch and --integration-branch" \
  'grep -qF -- "--metadata-branch" "$SKILL" && grep -qF -- "--integration-branch" "$SKILL"'

# ============================ publish-deferred ============================
# A change carrying the `## Publish deferred` marker emits exactly one publish-deferred finding —
# in archive/ (where the marker is actually written) and in active/ (harmlessly, per spec §3.3).
# NO status gate and NO directory gate: the marker's PRESENCE is the state, so a marker anywhere
# in the change set is a pending deferral. A change without the marker is silent, and an inline
# PROSE MENTION of the marker name is not state (has_section's whole-line rule).
read -r PD _ < <(new_repo)
# id 50: ARCHIVED + marker ⇒ fires (the real shape — the marker is written on the archived file).
cat > "$PD/docs/changes/archive/2026-07-08-0050-deferredkill.md" <<'EOF'
---
id: 50
slug: deferredkill
title: Killed proposal whose publish was deferred
status: killed
priority: medium
depends_on: []
---

## Why killed

Obsolete.

## Publish deferred

### 2026-07-08 — terminal-publish to `main` not completed

**deferred** — pending human approval

The record is on the metadata branch only.
EOF
# id 51: ARCHIVED, no marker ⇒ silent.
cat > "$PD/docs/changes/archive/2026-07-08-0051-cleankill.md" <<'EOF'
---
id: 51
slug: cleankill
title: Killed proposal published cleanly
status: killed
priority: medium
depends_on: []
---

## Why killed

Obsolete.
EOF
# id 52: ACTIVE + marker ⇒ fires too (no directory gate).
cat > "$PD/docs/changes/active/0052-activemarker.md" <<'EOF'
---
id: 52
slug: activemarker
title: Active change carrying the marker
status: proposed
priority: medium
depends_on: []
trivial: true
---

## Publish deferred

### 2026-07-08 — terminal-publish to `main` not completed

**blocked** — direct push to protected main
EOF
# id 53: a PROSE MENTION of the marker name, not a section ⇒ silent.
cat > "$PD/docs/changes/active/0053-prosemention.md" <<'EOF'
---
id: 53
slug: prosemention
title: Change whose body merely mentions the marker
status: proposed
priority: medium
depends_on: []
trivial: true
---

## What changes

Append a dated `## Publish deferred` section when the publish is deferred.
EOF
git -C "$PD" add docs/changes; git_quiet -C "$PD" commit -m "pd fixtures"
pdout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$PD/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"

assert "publish-deferred fires for an ARCHIVED change carrying the marker (id 50)" \
  'has_finding "$pdout" publish-deferred 50'
# Isolate the id-50 line into a variable FIRST, then match it. Never `producer | grep -q` under
# `set -o pipefail` (AGENTS.md): grep exits early, the producer takes SIGPIPE, and the 141 shows
# up as an intermittent failure. `grep -c`/`grep` without -q are safe producers to pipe from.
pd50="$(grep -E "$(printf '^publish-deferred\t50\t')" <<<"$pdout")"
assert "publish-deferred message names the integration branch" \
  'grep -qF -- "main" <<<"$pd50"'
assert "publish-deferred message says the record is on the metadata branch only" \
  'grep -qF -- "docket" <<<"$pd50"'
assert "publish-deferred silent for an archived change with no marker (id 51)" \
  '! has_finding "$pdout" publish-deferred 51'
assert "publish-deferred fires for an ACTIVE change carrying the marker (id 52, no directory gate)" \
  'has_finding "$pdout" publish-deferred 52'
assert "publish-deferred silent for a PROSE MENTION of the marker (id 53)" \
  '! has_finding "$pdout" publish-deferred 53'
# Exactly one finding per marked change — not one per line of the section.
assert "publish-deferred emits exactly ONE finding for id 50" \
  '[ "$(grep -cE "$(printf "^publish-deferred\t50\t")" <<<"$pdout")" -eq 1 ]'
# The marker must NOT suppress board-row-dropped, and must not itself drop a row: id 52 is a
# legal active change, so it renders and no board-row-dropped fires for it.
assert "a marked ACTIVE change does not trip board-row-dropped (id 52)" \
  '! has_finding "$pdout" board-row-dropped 52'
# warn-only: findings alone never change the exit status
assert "board-checks still exits 0 with publish-deferred findings (warn-only)" \
  'NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$PD/docs/changes" --metadata-branch docket --integration-branch main >/dev/null 2>&1'

# --- registration: the check-id is documented everywhere it must be (correspondence guard) ------
# Derived by grep from the emitting script, never hand-listed: every check-id board-checks.sh can
# EMIT must appear in the script's own header set, in board-checks.md, and in docket-status.md's
# closed enumeration. Anchored on the emitting code so a new check-id added without registering
# reddens here (change 0104's three-mirror drift; tracked structurally as change 0111).
#
# The derivation keys on the call's SYNTACTIC SHAPE (`emit <id> "`), never on line position. An
# earlier version anchored `^[[:space:]]*emit`, requiring `emit` to be the first token on its
# line; it silently missed every `cond || emit ...` call (board-checks.sh:197 and :206 — the
# broken-spec / broken-plan-results idiom), so the guard was decoration for 2 of the 12 real
# check-ids, and for any future check-id written with that idiom. `emit <id> "` doesn't care what
# precedes it on the line, and does NOT match the English "emit a table row" prose comment on :94
# — a real call is always `emit` + identifier + a quoted change-id argument; prose never quotes
# like that.
BCSH="$REPO/scripts/board-checks.sh"; BCMD="$REPO/scripts/board-checks.md"; DSMD="$REPO/scripts/docket-status.md"
emitted="$(grep -oE 'emit [a-z][a-z-]*[[:space:]]+"' "$BCSH" | awk '{print $2}' | sort -u)"

# Non-vacuity, CROSS-CHECKED rather than a hand-picked floor: a magic number like the old `-ge 8`
# sits below the true count by construction, so it can never catch an under-derivation — it didn't
# catch this file's own bug (10 cleared an `-ge 8` floor while the real count was 12). Instead
# derive an INDEPENDENT count from the script's own header comment (`check-id ∈ {...}`, see
# board-checks.sh:11-13 — the set spans three comment lines, so the extraction joins them before
# parsing) and assert the two counts agree. An under-derivation now disagrees with the header
# instead of merely clearing a floor both the buggy and correct counts satisfy.
header_ids="$(sed -n '/check-id ∈ {/,/}/p' "$BCSH" | sed -E 's/^#[[:space:]]*//' | tr '\n' ' ' \
  | sed -E 's/.*\{([^}]*)\}.*/\1/' | tr ',' '\n' | sed -E 's/^[[:space:]]+//; s/[[:space:]]+$//' \
  | grep -v '^$' | sort -u)"
emitted_count="$(grep -c . <<<"$emitted")"
header_count="$(grep -c . <<<"$header_ids")"
assert "the header's own check-id enumeration is non-empty (the cross-check itself is not vacuous)" \
  '[ "$header_count" -ge 1 ]'
assert "the emitted check-id derivation is non-empty (a broken regex must not vacuously satisfy the set compare)" \
  '[ "$emitted_count" -ge 1 ]'
# SET equality, not count equality (change 0083 review, minor 5). Counts are blind to a RENAME:
# misspelling `publish-deferred` in the header alone kept both sides at 12 and the suite printed
# PASS. `comm -3` prints the lines unique to either side, so any disagreement — under-derivation,
# over-derivation, or a one-for-one rename — leaves output and reddens. Both sides are already
# `sort -u`'d, which comm requires. Matches tests/test_docket_facade.sh:148's exact-set idiom for
# this same class of guard. The `|| { … >&2; false; }` tail reports WHICH ids disagree.
assert "emitted check-id SET == the header's own check-id ∈ {...} enumeration (a rename disagrees; a count compare would not)" \
  '[ -z "$(comm -3 <(printf "%s\n" "$emitted") <(printf "%s\n" "$header_ids"))" ] \
   || { comm -3 <(printf "%s\n" "$emitted") <(printf "%s\n" "$header_ids") >&2; false; }'
assert "publish-deferred is among the emitted check-ids" \
  'grep -qxF -- "publish-deferred" <<<"$emitted"'
# The `$BCSH` arm this loop used to carry was TAUTOLOGICAL — `$emitted` is derived BY grepping
# `$BCSH`, so grepping `$BCSH` for what it just yielded can never fail. The header enumeration is
# the real board-checks.sh surface, and the set compare above now covers it. Dropped, not moved.
#
# WORD-BOUNDARY, not substring (change 0083 review, minor 7). `grep -qF -- "$c"` passed a future
# id that is a substring of an existing one (e.g. `dep-cycle` vs a hypothetical `dep-cycle-hard`)
# on the STRONGER id's registration. No current id is a substring of another, so this is latent —
# fixed while the guard is open rather than left as a trap. Ids are `[a-z-]+`, so a boundary is
# simply "no adjacent id character"; backticks, spaces and punctuation in the docs all qualify.
reg_ok=1
for c in $emitted; do
  grep -qE -- "(^|[^a-z-])$c([^a-z-]|$)" "$BCMD" || { echo "check-id $c missing from board-checks.md" >&2; reg_ok=0; }
  grep -qE -- "(^|[^a-z-])$c([^a-z-]|$)" "$DSMD" || { echo "check-id $c missing from docket-status.md" >&2; reg_ok=0; }
done
assert "every EMITTED check-id is registered in both documentation surfaces (whole-word)" '[ "$reg_ok" -eq 1 ]'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
