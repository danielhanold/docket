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

# has_finding OUTPUT CHECK-ID CHANGE-ID — exit 0 iff OUTPUT has a "<check>\t<id>\t…" line.
# Builds a literal-TAB ERE pattern via printf (portable: avoids grep -P, which BSD grep lacks).
has_finding(){ printf '%s' "$1" | grep -qE "$(printf '^%s\t%s\t' "$2" "$3")"; }

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
# NOTE: this is checked via IFS field-extraction, not has_finding — has_finding builds an
# unescaped ERE from its args, and "?" is an ERE quantifier metacharacter (matches its preceding
# atom 0-or-1 times), so `has_finding "$wout" malformed-id "?"` is vacuously true for ANY
# malformed-id line regardless of the actual column value (verified: it stays green even when the
# implementation emits the raw frontmatter value instead of the "?" fallback). Read the finding
# back exactly the way docket-status.sh does instead, matching the TAB-injection case above.
read -r work4 _ <<<"$(new_repo)"
printf -- '---\nid: nope\nslug: weird\ntitle: Weird\nstatus: proposed\npriority: low\ndepends_on: []\n---\n' > "$work4/docs/changes/active/no-leading-id.md"
wout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$work4/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
IFS=$'\t' read -r w_check w_id w_msg <<<"$(printf '%s' "$wout" | grep '^malformed-id')"
assert "an id-less filename falls back to '?' in the change-id column" '[ "$w_id" = "?" ]'

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

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
