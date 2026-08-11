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
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }
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
# NEW_REPO_TEMPLATE: root holding the once-built baseline (tpl/) plus every copied
# fixture. One mktemp -d for the root instead of one per call.
NEW_REPO_TEMPLATE=""
_new_repo_build_template(){
  NEW_REPO_TEMPLATE="$(mktemp -d)"
  local work="$NEW_REPO_TEMPLATE/tpl/work" origin="$NEW_REPO_TEMPLATE/tpl/origin.git"
  mkdir -p "$NEW_REPO_TEMPLATE/tpl"
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
  # leave the template parked on docket (the metadata working tree)
}
new_repo(){
  local root work origin
  root="$(mktemp -d "$NEW_REPO_TEMPLATE/fXXXXXX")"
  origin="$root/origin.git"; work="$root/work"
  cp -R "$NEW_REPO_TEMPLATE/tpl/origin.git" "$origin"
  cp -R "$NEW_REPO_TEMPLATE/tpl/work" "$work"
  # The copy inherits the TEMPLATE's remote.origin.url; repoint it at its own bare
  # origin, or every fixture would push into (and read back from) the template.
  git -C "$work" remote set-url origin "$origin"
  printf '%s %s\n' "$work" "$origin"
}
# Built eagerly, at file scope, on purpose: every caller consumes new_repo as
# `read -r W O < <(new_repo)`, which runs it in a SUBSHELL. A lazy
# `[ -n "$NEW_REPO_TEMPLATE" ] || _new_repo_build_template` inside new_repo would
# assign NEW_REPO_TEMPLATE only in that subshell, so the parent would still see it
# empty and rebuild the template on every single call -- correct, silently, and with
# no speedup at all. Per-call roots come from mktemp for the same reason: a shared
# counter incremented in a subshell never advances.
_new_repo_build_template
# Safe to install here: this file has no other EXIT trap, so nothing is being replaced.
trap 'rm -rf "$NEW_REPO_TEMPLATE"' EXIT

# --- fixture independence (change 0174) --------------------------------------
# new_repo now copies a once-built template. Fixtures must not share an origin.
read -r indep_a_w indep_a_o < <(new_repo)
read -r indep_b_w indep_b_o < <(new_repo)
indep_tpl_before="$(git -C "$NEW_REPO_TEMPLATE/tpl/origin.git" rev-parse refs/heads/docket)"
indep_b_before="$(git -C "$indep_b_o" rev-parse refs/heads/docket)"
echo mutated > "$indep_a_w/MUTATION.md"
git -C "$indep_a_w" add MUTATION.md
git_quiet -C "$indep_a_w" commit -m mutate
git_quiet -C "$indep_a_w" push origin docket
assert "0174 independence: the mutated fixture's own origin DID advance (mutation was real)" \
  '[ "$(git -C "$indep_a_o" rev-parse refs/heads/docket)" != "$indep_tpl_before" ]'
assert "0174 independence: a sibling fixture's origin is untouched" \
  '[ "$(git -C "$indep_b_o" rev-parse refs/heads/docket)" = "$indep_b_before" ]'
assert "0174 independence: the template's origin is untouched" \
  '[ "$(git -C "$NEW_REPO_TEMPLATE/tpl/origin.git" rev-parse refs/heads/docket)" = "$indep_tpl_before" ]'
assert "0174 independence: a sibling worktree never sees the mutation" \
  '[ ! -e "$indep_b_w/MUTATION.md" ]'
assert "0174 independence: each fixture points at its OWN origin" \
  '[ "$(git -C "$indep_a_w" config remote.origin.url)" = "$indep_a_o" ]'
assert "0174 fixture parity: the copy is still parked on docket with main present" \
  '[ "$(git -C "$indep_b_w" rev-parse --abbrev-ref HEAD)" = docket ] && [ -n "$(git -C "$indep_b_o" rev-parse --verify -q refs/heads/main)" ]'

# Template integrity: the independence block above proves the property HERE; this snapshot
# plus the re-assertion just before the final exit extends it over every fixture call in
# the file, so a future test that dirties the shared template cannot go unnoticed.
tplint_refs="$(git -C "$NEW_REPO_TEMPLATE/tpl/origin.git" for-each-ref --format='%(refname) %(objectname)' | LC_ALL=C sort)"
tplint_head="$(git -C "$NEW_REPO_TEMPLATE/tpl/work" rev-parse HEAD)"
tplint_branch="$(git -C "$NEW_REPO_TEMPLATE/tpl/work" rev-parse --abbrev-ref HEAD)"

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
  'grep <<<"$bad_ttl_err" -qiF "lease-ttl-hours"'
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
  'grep <<<"$sout" -E "$(printf "^stale-in-progress\t20\t")" | grep >/dev/null -F "branch feat/stale idle >3 days"'
assert "stale-in-progress silent for a branch with a recent commit (id 21)" \
  '! has_finding "$sout" stale-in-progress 21'
assert "stale-in-progress silent when branch: set but branch absent (id 22, carve-out)" \
  '! has_finding "$sout" stale-in-progress 22'
assert "stale-in-progress fires for expired lease + no branch (id 23)" \
  'has_finding "$sout" stale-in-progress 23'
assert "id 23 finding carries the [reclaimable] marker" \
  'grep <<<"$sout" -E "$(printf "^stale-in-progress\t23\t")" | grep >/dev/null -F "[reclaimable]"'
assert "id 23 message reports age in hours (100h)" \
  'grep <<<"$sout" -E "$(printf "^stale-in-progress\t23\t")" | grep >/dev/null -F "100h ago"'
assert "stale-in-progress fires for expired lease + branch ref exists (id 24)" \
  'has_finding "$sout" stale-in-progress 24'
assert "id 24 finding does NOT carry [reclaimable] (branch exists ⇒ needs review, not auto-reclaimable)" \
  '! (grep <<<"$sout" -E "$(printf "^stale-in-progress\t24\t")" | grep >/dev/null -F "[reclaimable]")'
assert "id 24 message names the branch and says not auto-reclaimable" \
  'grep <<<"$sout" -E "$(printf "^stale-in-progress\t24\t")" | grep >/dev/null -F "branch feat/fresh" \
   && grep <<<"$sout" -E "$(printf "^stale-in-progress\t24\t")" | grep >/dev/null -F "not auto-reclaimable"'
assert "stale-in-progress silent for a fresh lease with no branch (id 25)" \
  '! has_finding "$sout" stale-in-progress 25'
assert "stale-in-progress emits exactly one finding when both branch-idle and lease-expired apply (id 26)" \
  '[ "$(grep <<<"$sout" -cE "$(printf "^stale-in-progress\t26\t")")" = 1 ]'
assert "id 26 finding is the branch-idle message, not the reclaimable/expired one (priority: branch-idle wins)" \
  'grep <<<"$sout" -E "$(printf "^stale-in-progress\t26\t")" | grep >/dev/null -F "idle >3 days" \
   && ! (grep <<<"$sout" -E "$(printf "^stale-in-progress\t26\t")" | grep >/dev/null -F "[reclaimable]")'
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
  'grep <<<"$mout" -E "$(printf "^merge-gate-stall\t31\t")" | grep >/dev/null -F "#30"'
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
  'grep <<<"$out" -qF "non-integer id '"'"'abc'"'"'"'

# TAB injection: an interior TAB in id: must not shift the message into the change-id field.
read -r work2 _ <<<"$(new_repo)"
printf -- '---\nid: 4\tEVIL\nslug: tabby\ntitle: Tabby\nstatus: proposed\npriority: low\ndepends_on: []\n---\n' > "$work2/docs/changes/active/0002-tabby.md"
tout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$work2/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
# Read the finding back exactly the way docket-status.sh does; all three columns must survive.
IFS=$'\t' read -r t_check t_id t_msg <<<"$(grep <<<"$tout" '^malformed-id')"
assert "TAB-in-id: check_id column survives the caller's IFS split" '[ "$t_check" = "malformed-id" ]'
assert "TAB-in-id: change-id column is the filename id, not a fragment of the raw value" '[ "$t_id" = "0002" ]'
assert "TAB-in-id: the message column is non-empty (not shifted into the id field)" '[ -n "$t_msg" ]'
assert "TAB-in-id: the embedded TAB is escaped to a visible \\t, not passed through raw" \
  'grep <<<"$t_msg" -qF "4\\tEVIL"'

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
assert "invalid-priority finding lists the shared rank order and default" \
  'grep -qF -- "priority '\''urgent'\'' is not one of: critical high medium low (empty = medium)" <<<"$fout"'
assert "priority validation calls the shared membership helper" \
  'grep -qF -- '\''docket_priority_is_member "$fd_priority"'\'' "$SCRIPT"'
assert "field-domain fires for a title containing a pipe (id 44)"    'has_finding "$fout" field-domain 44'
# The documented default: an EMPTY priority is LEGAL (convention says medium; the sort implements
# it). This assert is what keeps the domain check from becoming over-eager.
assert "field-domain SILENT for an empty priority (id 45, documented default)" \
  '! has_finding "$fout" field-domain 45'
assert "field-domain fires for an EMPTY status (id 46, no documented default)" \
  'has_finding "$fout" field-domain 46'

# Messages name the field and quote the offending value, so a reader can act without opening the file.
assert "the status finding names the field and the offending value" \
  'grep <<<"$fout" -qF "status '"'"'proposed  # awaiting X'"'"'"'
assert "the title finding names the pipe as the board-row hazard" \
  'grep <<<"$fout" -E "$(printf "^field-domain\t44\t")" | grep >/dev/null -F "title"'

# Shape, not spelling: a slug with a TAB and a slug with an uppercase letter both fire, though
# neither is an enumerated bad value.
mk_fd 0047-tabslug.md 47 "$(printf 'tab\tslug')" "Tab slug" proposed medium
mk_fd 0048-upper.md   48 UpperSlug "Upper slug"   proposed medium
sout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$F/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "field-domain fires for a slug containing a TAB (shape check, id 47)"  'has_finding "$sout" field-domain 47'
assert "field-domain fires for an uppercase slug (shape check, id 48)"        'has_finding "$sout" field-domain 48'
assert "a TAB inside a slug value cannot shift the findings line's columns (id 47)" \
  'grep <<<"$sout" -E "$(printf "^field-domain\t47\t")" | grep >/dev/null -F "\\t"'

# Warn-only posture is preserved: findings present, exit still 0 without --strict.
NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$F/docs/changes" --metadata-branch docket --integration-branch main >/dev/null 2>&1
assert "field-domain findings do not change the default exit status (warn-only)" '[ "$?" = 0 ]'

# The archive is walked too — a terminal status is in the vocabulary and must stay silent.
read -r G _ < <(new_repo)
printf -- '---\nid: 60\nslug: archived\ntitle: Archived\nstatus: done\npriority: medium\ndepends_on: []\n---\n' \
  > "$G/docs/changes/archive/2026-06-16-0060-archived.md"
gout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$G/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "field-domain silent for a terminal status in archive/ (id 60)" '! has_finding "$gout" field-domain 60'

# ----------------------- field-domain: `type` (change 0127) -----------------------
# `type` is rendered into every active board row, so it needs the same column-injection guard
# `title` has: a `|` in it silently widens that row of BOARD.md. The DOMAIN half deliberately
# checks SHAPE (^[a-z][a-z0-9-]*$), never membership in the configured taxonomy — render-board
# renders a type this machine does not configure on purpose ("configuration governs CREATION,
# never the readability of shared history"), so a membership check here would report another
# machine's legitimate type as a finding and turn the guard into the noise source. Empty is
# LEGAL: it renders as `untyped`, the state the migration exists to drain, and flagging it would
# fire on every un-backfilled change.
read -r T _ < <(new_repo)
mk_ft(){ # mk_ft FILE-BASENAME ID SLUG [TYPE] — TYPE omitted/empty ⇒ NO frontmatter `type:` line
  local tl=""
  [ -n "${4-}" ] && tl="type: $4"$'\n'
  printf -- '---\nid: %s\nslug: %s\ntitle: Typed change\nstatus: proposed\npriority: medium\ndepends_on: []\n%s---\n' \
    "$2" "$3" "$tl" > "$T/docs/changes/active/$1"
}
mk_ft 0050-pipetype.md  50 pipetype  "feat | fix"
mk_ft 0051-uppertype.md 51 uppertype "Feat"
mk_ft 0052-goodtype.md  52 goodtype  "feat"
mk_ft 0053-notype.md    53 notype
mk_ft 0054-spiketype.md 54 spiketype "spike"
# No frontmatter `type:` at all, but the BODY opens a line with `type:` carrying a pipe. The read
# is ANCHORED to the first ---...--- block (fm_field), so this must stay silent; an unanchored
# field() would fall through and return the prose as the type.
mk_ft 0055-bodytype.md  55 bodytype
cat >> "$T/docs/changes/active/0055-bodytype.md" <<'EOF'

## Notes
type: something | weird
EOF
tout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$T/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"

assert "field-domain fires for a type containing a pipe (id 50)" 'has_finding "$tout" field-domain 50'
assert "the type finding names the pipe as the board-row column-injection hazard (id 50)" \
  'grep <<<"$tout" -E "$(printf "^field-domain\t50\t")" | grep >/dev/null -F "type contains '\''|'\'', which injects columns into the board row"'
assert "field-domain fires for a malformed type shape (id 51, uppercase)" 'has_finding "$tout" field-domain 51'
assert "the malformed-type finding quotes the value and names the shape (id 51)" \
  'grep <<<"$tout" -E "$(printf "^field-domain\t51\t")" | grep >/dev/null -F "type '\''Feat'\'' is not ^[a-z][a-z0-9-]*\$"'
assert "field-domain SILENT for a well-formed type (id 52)" '! has_finding "$tout" field-domain 52'
# The assert that stops the guard becoming a noise source: an ABSENT type: is legal (renders as
# `untyped`), so every un-backfilled change during the migration must stay quiet.
assert "field-domain SILENT for an ABSENT type (id 53, empty = untyped)" \
  '! has_finding "$tout" field-domain 53'
# SHAPE, not membership: `spike` is well-formed but not in DOCKET_CHANGE_TYPES_DEFAULT. Tightening
# this guard into a taxonomy-membership check turns this assert red — which is the point.
assert "field-domain SILENT for a well-formed type OUTSIDE the configured taxonomy (id 54, spike)" \
  '! has_finding "$tout" field-domain 54'
# The anchored read: body prose beginning `type:` is not a frontmatter value.
assert "field-domain SILENT for a piped 'type:' line in the BODY of an untyped change (id 55, anchored read)" \
  '! has_finding "$tout" field-domain 55'

# ============================ scalar-form (change 0191) ============================
# The well-formedness leg of the house yaml-scalar rule (AGENTS.md + ADR-0065): an UNQUOTED
# frontmatter scalar that carries ': ' (colon-space) or is exactly a YAML 1.1 bare boolean keyword
# (on/off/yes/no/true/false, case-insensitive) is read ambiguously by any YAML consumer. Covers the
# only two free-text string scalars docket reads that are not already shape/domain-gated — title
# (field_raw) and the optional blocked_by (the anchored, comment-strip-free fm_field_verbatim; see
# mutation 3b for why the comment-stripping fm_field_raw cannot serve here). A scalar that OPENS with "
# or ' is well-formed by definition and never inspected (the 0190 quoted-title shape is the ACCEPT
# case); the natively-boolean fields (trivial, auto_groomable, reconciled) hold a bare true/false
# BY DESIGN and are not scanned. One finding per violated leg per field; warn-only (never EXPLAINED,
# never board-row-dropped). Each fixture is its own new_repo with ONE change file, so the finding
# set per id is unambiguous.
mk_sf(){ # mk_sf REPO-DIR ID SLUG [FRONTMATTER-LINES...] — a well-formed active change file whose
  local mrepo="$1" mid="$2" mslug="$3"; shift 3
  { printf -- '---\nid: %s\nslug: %s\nstatus: proposed\npriority: medium\ndepends_on: []\n' "$mid" "$mslug"
    printf '%s\n' "$@"
    printf -- '---\n'
  } > "$mrepo/docs/changes/active/$(printf '%04d' "$mid")-$mslug.md"
}

# --- RED: must fire a scalar-form finding ---
# Unquoted colon-space title (the 0190 regression shape, unquoted).
read -r S90 _ < <(new_repo)
mk_sf "$S90" 90 colonspace-title 'title: a: b'
s90out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$S90/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "scalar-form fires once for an unquoted colon-space title (id 90)" \
  '[ "$(grep -cE "$(printf "^scalar-form\t90\t")" <<<"$s90out")" = 1 ]'
SF_COLON_SHAPE="': '"
s90line="$(grep -E "$(printf "^scalar-form\t90\t")" <<<"$s90out")"
assert "the colon-space finding names the title field (id 90)" \
  'grep -qF -- "title: unquoted scalar contains" <<<"$s90line"'
assert "the colon-space finding names the ': ' shape literally (id 90)" \
  'grep -qF "$SF_COLON_SHAPE" <<<"$s90line"'

# Unquoted bare-boolean title — lower-case (yes) and case-insensitive (TRUE).
read -r S91 _ < <(new_repo)
mk_sf "$S91" 91 bool-title 'title: yes'
s91out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$S91/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "scalar-form fires for an unquoted bare-boolean title (id 91, boolean leg)" \
  'has_finding "$s91out" scalar-form 91'
s91line="$(grep -E "$(printf "^scalar-form\t91\t")" <<<"$s91out")"
assert "the boolean finding names the boolean shape and quotes the value (id 91)" \
  'grep -qF -- "unquoted bare YAML boolean (yes)" <<<"$s91line"'
read -r S85 _ < <(new_repo)
mk_sf "$S85" 85 uppercase-bool-title 'title: TRUE'
s85out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$S85/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "scalar-form is case-insensitive: an UNQUOTED UPPERCASE boolean title fires (id 85)" \
  'has_finding "$s85out" scalar-form 85'
s85line="$(grep -E "$(printf "^scalar-form\t85\t")" <<<"$s85out")"
assert "the uppercase-boolean finding quotes the TRUE value (id 85)" \
  'grep -qF -- "unquoted bare YAML boolean (TRUE)" <<<"$s85line"'

# Trailing colon — the leg whose absence let change 0173 sit unreported (change 0235).
read -r S86 _ < <(new_repo)
mk_sf "$S86" 86 trailing-colon-title 'title: a model ID containing / or :'
s86out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$S86/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "scalar-form fires for a title ENDING in a colon (id 86, trailing-colon leg)" \
  'has_finding "$s86out" scalar-form 86'
s86line="$(grep -E "$(printf "^scalar-form\t86\t")" <<<"$s86out")"
assert "the trailing-colon finding names the title field and the shape (id 86)" \
  'grep -qF -- "title: unquoted scalar ends with" <<<"$s86line"'

# ' #' opens a YAML comment: it TRUNCATES silently rather than aborting, so it is the quieter defect.
read -r S84 _ < <(new_repo)
mk_sf "$S84" 84 hash-title 'title: clear finding #3 from review'
s84out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$S84/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "scalar-form fires for a title containing ' #' (id 84, comment-introducer leg)" \
  'has_finding "$s84out" scalar-form 84'
s84line="$(grep -E "$(printf "^scalar-form\t84\t")" <<<"$s84out")"
assert "the comment-introducer finding names the title field and the comment shape (id 84)" \
  'grep -qF -- "title: unquoted scalar contains whitespace followed by '"'"'#'"'"', a YAML comment introducer" <<<"$s84line"'

# Any WHITESPACE before the '#' opens the comment, not a literal space alone. This is the
# hand-authored case the check exists for: mint-stub's control-character gate keeps a tab off the
# WRITE path, but nothing gates a file a human typed — and fm_field_raw's own strip is
# `[[:space:]]+#`, so the reader would truncate here whether or not the detector spoke up.
read -r S73 _ < <(new_repo)
mk_sf "$S73" 73 tab-hash-title "$(printf 'title: a stalled run\t#3 in the queue')"
s73out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$S73/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "scalar-form fires for a title whose '#' is preceded by a TAB (id 73, comment-introducer leg)" \
  'has_finding "$s73out" scalar-form 73'
s73line="$(grep -E "$(printf "^scalar-form\t73\t")" <<<"$s73out")"
assert "the TAB-preceded finding lands on the comment-introducer leg, not another (id 73)" \
  'grep -qF -- "a YAML comment introducer that silently truncates it" <<<"$s73line"'

# A leading YAML indicator character.
read -r S83 _ < <(new_repo)
mk_sf "$S83" 83 indicator-title 'title: [WIP] rework the runner'
s83out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$S83/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "scalar-form fires for a title opening with a YAML indicator (id 83, indicator leg)" \
  'has_finding "$s83out" scalar-form 83'
s83line="$(grep -E "$(printf "^scalar-form\t83\t")" <<<"$s83out")"
assert "the indicator finding names the title field and the indicator shape (id 83)" \
  'grep -qF -- "title: unquoted scalar opens with a YAML indicator character" <<<"$s83line"'

# A CLOSED flow collection — the shape a flow-collection exemption would have waved through. There
# is no such exemption: the legs judge whether a value is well-formed as a bare SCALAR, and `[234]`
# is not one, so end-to-end this is a RED fixture on the indicator leg. A field docket MEANS as a
# sequence (depends_on, adrs) is simply never routed through this check.
read -r S74 _ < <(new_repo)
mk_sf "$S74" 74 flow-collection-title 'title: [234]'
s74out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$S74/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "scalar-form fires for a title that is a closed flow collection (id 74, indicator leg — no exemption)" \
  'has_finding "$s74out" scalar-form 74'
s74line="$(grep -E "$(printf "^scalar-form\t74\t")" <<<"$s74out")"
assert "the flow-collection finding lands on the indicator leg (id 74)" \
  'grep -qF -- "title: unquoted scalar opens with a YAML indicator character" <<<"$s74line"'

# --- GREEN near-misses for the new legs: each is well-formed bare YAML and must stay SILENT ---
read -r S82 _ < <(new_repo)
mk_sf "$S82" 82 hash-nospace-title 'title: issue#3 reopened by the reporter'
s82out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$S82/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "scalar-form SILENT for a '#' not preceded by whitespace (id 82)" \
  '! has_finding "$s82out" scalar-form 82'

read -r S81 _ < <(new_repo)
mk_sf "$S81" 81 interior-dash-title 'title: a well-formed title with an interior dash'
s81out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$S81/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "scalar-form SILENT for an interior dash (id 81)" \
  '! has_finding "$s81out" scalar-form 81'

read -r S80 _ < <(new_repo)
mk_sf "$S80" 80 colon-nospace-title 'title: the a:b ratio holds'
s80out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$S80/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "scalar-form SILENT for a colon with no following space (id 80)" \
  '! has_finding "$s80out" scalar-form 80'

# The SINGLE-quoted shape mint-stub now always writes must be accepted by the checker it will meet.
read -r S79 _ < <(new_repo)
mk_sf "$S79" 79 minted-shape-title "title: 'The manifest''s elsewhere: check proves a word occurrence'"
s79out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$S79/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "scalar-form SILENT for the exact shape mint-stub now writes (id 79, skip leg)" \
  '! has_finding "$s79out" scalar-form 79'

# Unquoted colon-space blocked_by.
read -r S92 _ < <(new_repo)
mk_sf "$S92" 92 colonspace-blocked 'blocked_by: a: b'
s92out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$S92/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "scalar-form fires for an unquoted colon-space blocked_by (id 92)" \
  'has_finding "$s92out" scalar-form 92'
s92line="$(grep -E "$(printf "^scalar-form\t92\t")" <<<"$s92out")"
assert "the blocked_by colon-space finding names blocked_by (id 92)" \
  'grep -qF -- "blocked_by: unquoted scalar contains" <<<"$s92line"'

# Unquoted bare-boolean blocked_by.
read -r S93 _ < <(new_repo)
mk_sf "$S93" 93 bool-blocked 'blocked_by: off'
s93out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$S93/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "scalar-form fires for an unquoted bare-boolean blocked_by (id 93)" \
  'has_finding "$s93out" scalar-form 93'
s93line="$(grep -E "$(printf "^scalar-form\t93\t")" <<<"$s93out")"
assert "the blocked_by boolean finding quotes the value (id 93)" \
  'grep -qF -- "unquoted bare YAML boolean (off)" <<<"$s93line"'

# Trailing colon in blocked_by — the third new leg mirrored onto the SECOND field. scalar_form_check
# is invoked once per field, so a leg pinned on `title` alone proves only that the predicate has it,
# never that both call sites reach it (the ' #' leg was reachable on title and structurally dead on
# blocked_by for exactly that reason — see mutation 3b).
read -r S88 _ < <(new_repo)
mk_sf "$S88" 88 trailing-colon-blocked 'blocked_by: waiting on the model ID containing / or :'
s88out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$S88/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "scalar-form fires for a blocked_by ENDING in a colon (id 88, trailing-colon leg)" \
  'has_finding "$s88out" scalar-form 88'
s88line="$(grep -E "$(printf "^scalar-form\t88\t")" <<<"$s88out")"
assert "the blocked_by trailing-colon finding names blocked_by and the shape (id 88)" \
  'grep -qF -- "blocked_by: unquoted scalar ends with" <<<"$s88line"'

# ' #' in blocked_by — the real shape on the metadata branch (`blocked_by: PR #69 is stale …`).
# The comment-introducer leg is pinned in BOTH fields, not just title: a reader that strips the
# inline comment BEFORE the predicate sees it (fm_field_raw does, deliberately, for its own
# consumers) hands the check a truncated remnant and the leg can never fire on blocked_by. The
# blocked_by read therefore goes through the comment-strip-free anchored accessor.
read -r S78 _ < <(new_repo)
mk_sf "$S78" 78 hash-blocked 'blocked_by: PR #69 is stale, predating the facade rework'
s78out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$S78/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "scalar-form fires for a blocked_by containing ' #' (id 78, comment-introducer leg)" \
  'has_finding "$s78out" scalar-form 78'
s78line="$(grep -E "$(printf "^scalar-form\t78\t")" <<<"$s78out")"
assert "the blocked_by comment-introducer finding names blocked_by (id 78)" \
  'grep -qF -- "blocked_by: unquoted scalar contains" <<<"$s78line"'

# A LEADING '#' in blocked_by — reachable only because the blocked_by read is now comment-strip-free
# (id 78's leg). The comment opens at character one, so the WHOLE value parses to null: the maximal
# form of the truncation, and the quietest. It carries no ': ' and no ' #', so it must reach the
# indicator leg rather than any earlier one.
read -r S75 _ < <(new_repo)
mk_sf "$S75" 75 leading-hash-blocked 'blocked_by: #235 follow-up work'
s75out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$S75/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "scalar-form fires for a blocked_by opening with '#' (id 75, indicator leg)" \
  'has_finding "$s75out" scalar-form 75'
s75line="$(grep -E "$(printf "^scalar-form\t75\t")" <<<"$s75out")"
assert "the leading-hash blocked_by finding names the indicator shape (id 75)" \
  'grep -qF -- "blocked_by: unquoted scalar opens with a YAML indicator character" <<<"$s75line"'

# GREEN near-miss on the same field: a '#' NOT preceded by whitespace is part of the value, and the
# quoted form of the ' #' shape is well-formed — neither may fire now that the comment survives the
# read.
read -r S77 _ < <(new_repo)
mk_sf "$S77" 77 hash-nospace-blocked 'blocked_by: PR#69 is stale'
s77out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$S77/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "scalar-form SILENT for a blocked_by '#' not preceded by whitespace (id 77)" \
  '[ -z "$s77out" ]'
read -r S76 _ < <(new_repo)
mk_sf "$S76" 76 quoted-hash-blocked "blocked_by: 'PR #69 is stale'"
s76out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$S76/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "scalar-form SILENT for a single-quoted ' #' blocked_by (id 76, skip leg)" \
  '[ -z "$s76out" ]'

# --- GREEN: must NOT fire a scalar-form finding (and nothing else on these clean fixtures) ---
# Quoted colon-space TITLE — double-quoted and single-quoted (the 0190 ACCEPT shape; the quote leg
# keeps it green). Exact empty output: no scalar-form and no other check fires.
read -r S94 _ < <(new_repo)
mk_sf "$S94" 94 quoted-colonspace 'title: "a: b"'
s94out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$S94/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "scalar-form SILENT for a DOUBLE-quoted colon-space title (id 94, the 0190 accept shape)" \
  '[ -z "$s94out" ]'
read -r S87 _ < <(new_repo)
mk_sf "$S87" 87 singlequoted-colonspace "title: 'a: b'"
s87out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$S87/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "scalar-form SILENT for a SINGLE-quoted colon-space title (id 87, quote leg)" \
  '[ -z "$s87out" ]'

# Quoted blocked_by with a colon-space.
read -r S95 _ < <(new_repo)
mk_sf "$S95" 95 quoted-blocked 'blocked_by: "waiting: on review"'
s95out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$S95/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "scalar-form SILENT for a quoted colon-space blocked_by (id 95)" \
  '[ -z "$s95out" ]'

# Clean bare title (no colon-space, no bare boolean).
read -r S96 _ < <(new_repo)
mk_sf "$S96" 96 clean-title 'title: A clean bare title'
s96out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$S96/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "scalar-form SILENT for a clean bare title (id 96)" \
  '[ -z "$s96out" ]'

# Present, well-formed blocked_by.
read -r S97 _ < <(new_repo)
mk_sf "$S97" 97 clean-blocked 'blocked_by: waiting on review'
s97out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$S97/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "scalar-form SILENT for a present, well-formed blocked_by (id 97)" \
  '[ -z "$s97out" ]'

# THE LOAD-BEARING fixture: frontmatter OMITS blocked_by but the BODY opens a blocked_by: line.
# fm_field_raw is ANCHORED to the first ---...--- block, so it returns empty and the check stays
# green; an UNANCHORED field_raw would fall through to the body prose and misfire (mutation 3).
read -r S98 _ < <(new_repo)
mk_sf "$S98" 98 absent-blocked-by 'title: Omits blocked_by'
cat >> "$S98/docs/changes/active/0098-absent-blocked-by.md" <<'EOF'

## Notes
blocked_by: waiting: on review
EOF
s98out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$S98/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "scalar-form SILENT for a change that OMITS blocked_by while its BODY opens a blocked_by: line (id 98, anchored read)" \
  '[ -z "$s98out" ]'

# Natively-boolean field untouched: trivial: true is CORRECT well-formed YAML for a boolean field.
read -r S99 _ < <(new_repo)
mk_sf "$S99" 99 trivial-boolean 'trivial: true'
s99out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$S99/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "scalar-form SILENT for a natively-boolean field (trivial: true, id 99)" \
  '[ -z "$s99out" ]'

# Quoted bare-boolean title.
read -r S89 _ < <(new_repo)
mk_sf "$S89" 89 quoted-bool-title 'title: "yes"'
s89out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$S89/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "scalar-form SILENT for a quoted bare-boolean title (title: \"yes\", id 89)" \
  '[ -z "$s89out" ]'

# --- mutation tests (guards-are-code): throwaway copies of board-checks.sh, never committed ---
# A guard is code: each leg below is mutated in a COPY of the script and watched change the
# fixtures' outcome. The copy lives in a temp scripts/ tree so its `source` of
# scripts/lib/docket-frontmatter.sh resolves. Every mutation is run against a FRESH
# pristine copy (never a cumulative chain of earlier mutations on one file) and CONFIRMED LANDED
# (grep -c before/after) before its red/green result is believed.
read -r MUT _ < <(new_repo)
mk_sf "$MUT" 85  uppercase-bool-title   'title: TRUE'
mk_sf "$MUT" 90  colonspace-title       'title: a: b'
mk_sf "$MUT" 91  bool-title             'title: yes'
mk_sf "$MUT" 92  colonspace-blocked     'blocked_by: a: b'
mk_sf "$MUT" 93  bool-blocked           'blocked_by: off'
mk_sf "$MUT" 94  quoted-colonspace      'title: "a: b"'
mk_sf "$MUT" 98  absent-blocked-by      'title: Omits blocked_by'
mk_sf "$MUT" 78  hash-blocked           'blocked_by: PR #69 is stale, predating the facade rework'
mk_sf "$MUT" 86  trailing-colon-title   'title: a model ID containing / or :'
cat >> "$MUT/docs/changes/active/0098-absent-blocked-by.md" <<'EOF'

## Notes
blocked_by: waiting: on review
EOF
mcopy=""
mreseed(){
  [ -n "$mcopy" ] && rm -rf "$mcopy"; mcopy="$(mktemp -d)"
  mkdir -p "$mcopy/scripts/lib"
  cp "$SCRIPT" "$mcopy/scripts/board-checks.sh"
  cp "$REPO/scripts/lib/docket-frontmatter.sh" "$mcopy/scripts/lib/"
  MUTSCRIPT="$mcopy/scripts/board-checks.sh"
}
mrun(){ NOW=$NOW_EPOCH bash "$MUTSCRIPT" --changes-dir "$MUT/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null; }

# Mutation 1 — strip the colon-space leg: 90/92 go GREEN; the boolean leg SURVIVES (91/93).
mreseed
m1_before="$(grep -cF -- "unquoted scalar contains ': '" "$MUTSCRIPT")"
awk "!/unquoted scalar contains ': '/" "$MUTSCRIPT" > "$MUTSCRIPT.trim"; mv "$MUTSCRIPT.trim" "$MUTSCRIPT"
m1_after="$(grep -cF -- "unquoted scalar contains ': '" "$MUTSCRIPT")"
m1out="$(mrun)"
assert "mutation 1 landed: the colon-space emit arm is gone (count 1 -> 0)" \
  '[ "$m1_before" = 1 ] && [ "$m1_after" = 0 ]'
assert "mutation 1 (strip colon-space leg): colon-space title 90 goes GREEN" \
  '! has_finding "$m1out" scalar-form 90'
assert "mutation 1 (strip colon-space leg): colon-space blocked_by 92 goes GREEN" \
  '! has_finding "$m1out" scalar-form 92'
assert "mutation 1 (strip colon-space leg): boolean title 91 still fires (leg survives)" \
  'has_finding "$m1out" scalar-form 91'
assert "mutation 1 (strip colon-space leg): boolean blocked_by 93 still fires (leg survives)" \
  'has_finding "$m1out" scalar-form 93'

# Mutation 1b — strip the trailing-colon leg (change 0235's newest, and the one whose absence let
# change 0173 sit unreported): fixture 86 goes GREEN while the colon-space leg SURVIVES on 90/92.
# Mutation 1 above cannot stand in for it — the two are separate emit arms reading separate predicate
# tokens, and a leg no mutation has ever reddened is decoration.
mreseed
m1b_before="$(grep -cF -- "unquoted scalar ends with" "$MUTSCRIPT")"
awk '!/unquoted scalar ends with/' "$MUTSCRIPT" > "$MUTSCRIPT.trim"; mv "$MUTSCRIPT.trim" "$MUTSCRIPT"
m1b_after="$(grep -cF -- "unquoted scalar ends with" "$MUTSCRIPT")"
m1bout="$(mrun)"
assert "mutation 1b landed: the trailing-colon emit arm is gone (count 1 -> 0)" \
  '[ "$m1b_before" = 1 ] && [ "$m1b_after" = 0 ]'
assert "mutation 1b (strip trailing-colon leg): trailing-colon title 86 goes GREEN" \
  '! has_finding "$m1bout" scalar-form 86'
assert "mutation 1b (strip trailing-colon leg): colon-space title 90 still fires (leg survives)" \
  'has_finding "$m1bout" scalar-form 90'
assert "mutation 1b (strip trailing-colon leg): colon-space blocked_by 92 still fires (leg survives)" \
  'has_finding "$m1bout" scalar-form 92'

# Mutation 2 — strip the quote/empty skip leg: the QUOTED colon-space title 94 REDDENS (the wrong
# direction, proving the quote leg is load-bearing); an EMPTY raw value (98, absent blocked_by)
# still stays green because an empty value is neither colon-space nor a boolean.
mreseed
m2_before="$(grep -cF -- "skip leg:" "$MUTSCRIPT")"
awk "!/skip leg:/" "$MUTSCRIPT" > "$MUTSCRIPT.trim"; mv "$MUTSCRIPT.trim" "$MUTSCRIPT"
m2_after="$(grep -cF -- "skip leg:" "$MUTSCRIPT")"
m2out="$(mrun)"
assert "mutation 2 landed: the quote/empty skip arm is gone (count 1 -> 0)" \
  '[ "$m2_before" = 1 ] && [ "$m2_after" = 0 ]'
assert "mutation 2 (strip quote skip): the QUOTED colon-space title 94 REDDENS (the wrong direction)" \
  'has_finding "$m2out" scalar-form 94'
assert "mutation 2 (strip quote skip): the absent blocked_by 98 stays GREEN (empty is not a shape)" \
  '! has_finding "$m2out" scalar-form 98'

# Mutation 3 — replace fm_field_verbatim with field_raw for blocked_by: the absent-blocked_by +
# body-prose fixture 98 MISFires (the unanchored read takes the body prose), proving the anchoring.
mreseed
m3_before="$(grep -cF 'fm_field_verbatim "$f" blocked_by' "$MUTSCRIPT")"
awk '{ if ($0 ~ /sf_blocked_by=/) sub(/fm_field_verbatim/, "field_raw"); print }' "$MUTSCRIPT" > "$MUTSCRIPT.trim"; mv "$MUTSCRIPT.trim" "$MUTSCRIPT"
m3_after="$(grep -cF 'fm_field_verbatim "$f" blocked_by' "$MUTSCRIPT")"
m3out="$(mrun)"
assert "mutation 3 landed: the blocked_by read is unanchored (fm_field_verbatim count 1 -> 0)" \
  '[ "$m3_before" = 1 ] && [ "$m3_after" = 0 ]'
assert "mutation 3 (replace fm_field_verbatim with field_raw): absent-blocked_by fixture 98 MISFires — proves the anchoring" \
  'has_finding "$m3out" scalar-form 98'
assert "mutation 3: the colon-space blocked_by 92 still fires under the unanchored read" \
  'has_finding "$m3out" scalar-form 92'

# Mutation 3b — read blocked_by through fm_field_raw (the comment-STRIPPING anchored twin) instead:
# the ' #' fixture 78 goes GREEN because the reader truncated the value to `PR` before the predicate
# saw it, while the colon-space fixture 92 stays red. This is the exact defect the verbatim accessor
# exists to close: the comment-introducer leg is structurally unreachable through a stripping reader,
# and only a fixture on THIS field can see it (change 0235).
mreseed
m3b_before="$(grep -cF 'fm_field_verbatim "$f" blocked_by' "$MUTSCRIPT")"
awk '{ if ($0 ~ /sf_blocked_by=/) sub(/fm_field_verbatim/, "fm_field_raw"); print }' "$MUTSCRIPT" > "$MUTSCRIPT.trim"; mv "$MUTSCRIPT.trim" "$MUTSCRIPT"
m3b_after="$(grep -cF 'fm_field_verbatim "$f" blocked_by' "$MUTSCRIPT")"
m3bout="$(mrun)"
assert "mutation 3b landed: the blocked_by read strips inline comments (fm_field_verbatim count 1 -> 0)" \
  '[ "$m3b_before" = 1 ] && [ "$m3b_after" = 0 ]'
assert "mutation 3b (read blocked_by through the comment-stripping fm_field_raw): the ' #' fixture 78 goes GREEN" \
  '! has_finding "$m3bout" scalar-form 78'
assert "mutation 3b: the colon-space blocked_by 92 still fires (only the comment leg is blinded)" \
  'has_finding "$m3bout" scalar-form 92'

# Mutation 4 — drop the whole scalar-form probe: every red fixture goes GREEN. TWO regions, not one
# (change 0200). The definition now sits at TOP LEVEL, so the old marker-to-marker range delete
# (`# --- scalar-form:` .. `# --- broken-spec:`) would also swallow the FILES mapfile, the walk's
# own `for` line, and every check between, leaving an orphaned `done`. That copy is syntactically
# dead — and mrun discards stderr, so the landed assert and every "goes GREEN" assert below would
# pass without the script having run at all — observed, not theorised: with the old awk against the
# hoisted script this whole file still reported a fully green run. Region 1 is the hoisted definition,
# bounded by its own start marker and the NAMED end marker; region 2 is the four call-site lines
# inside the walk, matched individually. The `bash -n` assert is what makes a future regression of
# this exact shape impossible to miss — it is the assert this arm has never had.
mreseed
m4_before="$(grep -c 'scalar_form_check' "$MUTSCRIPT")"
m4_for_before="$(grep -c '^for f in ' "$MUTSCRIPT")"
awk '
  /^# --- scalar-form:/               { insf=1; next }
  /^# --- end scalar-form helper ---/ { insf=0; next }
  insf                                { next }
  /^  sf_title=/                      { next }
  /^  sf_blocked_by=/                 { next }
  /^  scalar_form_check title /       { next }
  /^  scalar_form_check blocked_by /  { next }
  { print }
' "$MUTSCRIPT" > "$MUTSCRIPT.trim"; mv "$MUTSCRIPT.trim" "$MUTSCRIPT"
m4_after="$(grep -c 'scalar_form_check' "$MUTSCRIPT")"
m4_for_after="$(grep -c '^for f in ' "$MUTSCRIPT")"
m4out="$(mrun)"
assert "mutation 4 landed: the scalar-form probe is gone from BOTH regions (scalar_form_check count 3 -> 0)" \
  '[ "$m4_before" = 3 ] && [ "$m4_after" = 0 ]'
assert "mutation 4 landed: the mutated copy is STILL VALID BASH — a range delete that spanned the walk would not be" \
  'bash -n "$MUTSCRIPT"'
# The walk's `for f in "${FILES[@]}"` opening is the first of the script's two top-level `for f in`
# lines and is exactly what a marker-to-marker range spanning the walk would eat. Pinned as
# before-and-after rather than a bare literal so the assert survives a third walk being added.
assert "mutation 4 landed: the walk survived the delete (both top-level for-loop openings are still there)" \
  '[ "$m4_for_before" = 2 ] && [ "$m4_for_after" = 2 ]'
assert "mutation 4 (drop whole probe block): colon-space title 90 goes GREEN" \
  '! has_finding "$m4out" scalar-form 90'
assert "mutation 4 (drop whole probe block): boolean title 91 goes GREEN" \
  '! has_finding "$m4out" scalar-form 91'
assert "mutation 4 (drop whole probe block): colon-space blocked_by 92 goes GREEN" \
  '! has_finding "$m4out" scalar-form 92'
assert "mutation 4 (drop whole probe block): boolean blocked_by 93 goes GREEN" \
  '! has_finding "$m4out" scalar-form 93'
assert "mutation 4 (drop whole probe block): uppercase boolean title 85 goes GREEN" \
  '! has_finding "$m4out" scalar-form 85'
assert "mutation 4 (drop whole probe block): trailing-colon title 86 goes GREEN" \
  '! has_finding "$m4out" scalar-form 86'
# NON-VACUITY for the six GREEN asserts above. It deliberately is NOT "$m4out is non-empty": the MUT
# fixture set is built so that scalar-form is the ONLY check that fires, so an empty $m4out is the
# CORRECT result here and could never discriminate. What discriminates is that the mutant RAN — mrun
# sends stderr to /dev/null, which is exactly how a syntactically dead copy fakes six green asserts,
# so this re-runs it capturing stderr INSTEAD of stdout and demands both a silent stderr and a
# normal exit. A range delete spanning the walk leaves an orphaned `done`: bash prints "syntax error
# near unexpected token \`done'" and exits 2, and both halves of this assert go red.
m4_err="$(NOW=$NOW_EPOCH bash "$MUTSCRIPT" --changes-dir "$MUT/docs/changes" --metadata-branch docket --integration-branch main 2>&1 >/dev/null)"
m4_rc=$?
assert "mutation 4: the mutated copy still RUNS — no stderr, normal exit (mrun's 2>/dev/null hides both)" \
  '[ -z "$m4_err" ] && [ "$m4_rc" = 0 ]'
rm -rf "$mcopy"

# ============================ aborted-run, leg A (change 0113) ============================
# An autonomous run that narrated success but dropped its bookkeeping write. Leg A is the
# TIME-FREE half: the feature branch carries an artifact file that is absent from the integration
# branch while the matching manifest field is EMPTY. This is the exact INVERSE of
# broken-plan-results (field set, file missing on the integration branch) — same two fields, same
# two trees, opposite direction; together they close a square that was half-open.
#
# Every optional field this check reads (plan, results, branch, claimed_at) goes through the
# ANCHORED fm_field: an unanchored read falls through the closing --- into body prose, and in this
# repo a change file whose body discusses `plan:` is normal content (ADR-0057). Fixture ar5 is what
# pins that; mutation 3 in Task 3 is what proves the pin can fail.
#
# Advisory only: warn-only, never EXPLAINED, never board-row-dropped, never mutates a file.
AR_PLAN_NEW="docs/superpowers/plans/2026-08-03-aborted.md"
AR_RESULTS_NEW="docs/results/2026-08-03-aborted-results.md"

# ar_branch REPO BRANCH PATH — cut BRANCH from main in REPO and commit PATH on it, so PATH exists
# on BRANCH and NOT on main. Leaves the repo parked back on docket (the metadata working tree).
ar_branch(){
  local arb_repo="$1" arb_br="$2" arb_path="$3"
  git -C "$arb_repo" checkout -b "$arb_br" main >/dev/null 2>&1
  mkdir -p "$arb_repo/$(dirname "$arb_path")"
  printf '# artifact\n' > "$arb_repo/$arb_path"
  git -C "$arb_repo" add "$arb_path"
  git_quiet -C "$arb_repo" commit -m "artifact on $arb_br"
  git -C "$arb_repo" checkout docket >/dev/null 2>&1
}

# ar_branch_at REPO BRANCH BASE AGE_SECS [PATH] — cut BRANCH from BASE in REPO and, when PATH is
# given, commit PATH on it with BOTH author and committer dates set to AGE_SECS before NOW_EPOCH.
# Leaves the repo parked back on docket, like ar_branch.
#
# A SIBLING of ar_branch, not a widening of it (change 0211): ar_branch is called by every existing
# ARM fixture, and giving it date control would change what those fixtures measure. Byte-identical
# is not the same as unaffected — leg C changes what they COULD emit, which is pinned separately.
#
# The dates are load-bearing, not decoration. ar_branch's commits carry real wall-clock dates while
# NOW_EPOCH is 1750000000 (2025-06), so `NOW - ts` is hugely NEGATIVE for them and they are silent
# for leg C's idle floor only by accident. A leg-C fixture must never inherit that accident: its
# age has to be the thing under test.
ar_branch_at(){
  local aba_repo="$1" aba_br="$2" aba_base="$3" aba_age="$4" aba_path="${5:-}" aba_when
  aba_when="@$(( NOW_EPOCH - aba_age ))"
  git -C "$aba_repo" checkout -b "$aba_br" "$aba_base" >/dev/null 2>&1
  if [ -n "$aba_path" ]; then
    mkdir -p "$aba_repo/$(dirname "$aba_path")"
    printf '# artifact\n' > "$aba_repo/$aba_path"
    git -C "$aba_repo" add "$aba_path"
    GIT_AUTHOR_DATE="$aba_when" GIT_COMMITTER_DATE="$aba_when" \
      git -C "$aba_repo" commit -m "commit on $aba_br" >/dev/null 2>&1
  fi
  git -C "$aba_repo" checkout docket >/dev/null 2>&1
}

# ar_push REPO BRANCH — publish BRANCH to the fixture's own bare origin, which is what creates
# refs/remotes/origin/<BRANCH> — the ref leg C's message branch probes. Separate from ar_branch_at
# because "was it pushed" is exactly the axis the two leg-C messages split on.
ar_push(){ git -C "$1" push -q origin "$2" >/dev/null 2>&1; }

# --- RED: the branch carries a plan the manifest does not record ---
read -r AR1 _ < <(new_repo)
ar_branch "$AR1" feat/ar1 "$AR_PLAN_NEW"
cat > "$AR1/docs/changes/active/0201-plan-unrecorded.md" <<'EOF'
---
id: 201
slug: plan-unrecorded
title: Plan committed, plan field never written
status: in-progress
priority: medium
depends_on: []
branch: feat/ar1
plan:
results:
---
EOF
ar1out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR1/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run leg A fires for a committed plan with an empty plan: (id 201)" \
  'has_finding "$ar1out" aborted-run 201'
ar1line="$(grep -E "$(printf "^aborted-run\t201\t")" <<<"$ar1out")"
assert "the leg-A plan finding names the plan: field and the offending path (id 201)" \
  'grep -qF -- "plan: is unset" <<<"$ar1line" && grep -qF -- "$AR_PLAN_NEW" <<<"$ar1line"'
assert "aborted-run fires exactly ONCE for id 201 (leg B must stay silent on a fresh claim)" \
  '[ "$(grep -cE "$(printf "^aborted-run\t201\t")" <<<"$ar1out")" = 1 ]'
# Leg C (change 0211) shares this check-id, and 201 satisfies three of its four conjuncts: feat/ar1
# is ahead of main and pr: is absent. It stays silent only because ar_branch dates its commit with
# the real wall clock while NOW is NOW_EPOCH, making the idle floor's delta NEGATIVE. Name the
# intent, so re-dating this fixture reddens here — with a message that says why — instead of
# silently turning the count above into 2.
assert "leg C stays out of the id-201 count: no leg-C message on 201" \
  '! grep -qF "pr: is unset" <<<"$ar1line"'

# --- RED: the branch carries a results file the manifest does not record ---
read -r AR2 _ < <(new_repo)
ar_branch "$AR2" feat/ar2 "$AR_RESULTS_NEW"
cat > "$AR2/docs/changes/active/0202-results-unrecorded.md" <<'EOF'
---
id: 202
slug: results-unrecorded
title: Results committed, results field never written
status: in-progress
priority: medium
depends_on: []
branch: feat/ar2
plan: docs/superpowers/plans/2026-06-01-present.md
results:
---
EOF
ar2out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR2/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run leg A fires for a committed results file with an empty results: (id 202)" \
  'has_finding "$ar2out" aborted-run 202'
ar2line="$(grep -E "$(printf "^aborted-run\t202\t")" <<<"$ar2out")"
assert "the leg-A results finding names the results: field and the offending path (id 202)" \
  'grep -qF -- "results: is unset" <<<"$ar2line" && grep -qF -- "$AR_RESULTS_NEW" <<<"$ar2line"'

# --- GREEN: the healthy in-flight build. Branch carries a plan AND the field records it. ---
read -r AR3 _ < <(new_repo)
ar_branch "$AR3" feat/ar3 "$AR_PLAN_NEW"
cat > "$AR3/docs/changes/active/0203-healthy.md" <<EOF
---
id: 203
slug: healthy
title: Healthy in-flight build
status: in-progress
priority: medium
depends_on: []
branch: feat/ar3
plan: $AR_PLAN_NEW
results:
claimed_at: $(iso $(( NOW_EPOCH - 3600 )))
---
EOF
ar3out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR3/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run SILENT for a healthy in-flight build: plan committed AND plan: set (id 203)" \
  '! has_finding "$ar3out" aborted-run 203'

# --- GREEN: an in-progress change with a branch that carries NO new artifact at all. Leg A is
# time-free and must not fire merely because a claim exists; leg B (Task 2) is what covers this
# shape, and its claim here is fresh.
read -r AR4 _ < <(new_repo)
git -C "$AR4" checkout -b feat/ar4 main >/dev/null 2>&1
git -C "$AR4" checkout docket >/dev/null 2>&1
cat > "$AR4/docs/changes/active/0204-nothing-yet.md" <<EOF
---
id: 204
slug: nothing-yet
title: Claimed, branch cut, nothing built yet
status: in-progress
priority: medium
depends_on: []
branch: feat/ar4
plan:
results:
claimed_at: $(iso $(( NOW_EPOCH - 3600 )))
---
EOF
ar4out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR4/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run SILENT for a fresh claim whose branch carries no new artifact (id 204)" \
  '! has_finding "$ar4out" aborted-run 204'

# --- THE LOAD-BEARING anchoring fixture: frontmatter OMITS plan: entirely while the BODY opens a
# plan: line. fm_field is anchored to the first ---...--- block, so plan: reads EMPTY and the check
# FIRES (the artifact is genuinely unrecorded). An UNANCHORED field() would read the body prose as
# a set plan: and go silently green — the false-negative this whole change exists to prevent.
read -r AR5 _ < <(new_repo)
ar_branch "$AR5" feat/ar5 "$AR_PLAN_NEW"
cat > "$AR5/docs/changes/active/0205-body-prose-plan.md" <<'EOF'
---
id: 205
slug: body-prose-plan
title: Omits plan in frontmatter, discusses it in the body
status: in-progress
priority: medium
depends_on: []
branch: feat/ar5
---

## Notes
plan: docs/superpowers/plans/2026-06-01-present.md
EOF
ar5out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR5/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run leg A FIRES when plan: is absent from frontmatter and only body prose mentions it (id 205, anchored read)" \
  'has_finding "$ar5out" aborted-run 205'

# --- GREEN: status gate. The identical incoherence on a NON-in-progress change is silent.
read -r AR6 _ < <(new_repo)
ar_branch "$AR6" feat/ar6 "$AR_PLAN_NEW"
cat > "$AR6/docs/changes/active/0206-proposed.md" <<'EOF'
---
id: 206
slug: proposed-incoherent
title: Same incoherence but not in-progress
status: proposed
priority: medium
depends_on: []
branch: feat/ar6
plan:
results:
---
EOF
ar6out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR6/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run SILENT on a 'proposed' change with the same incoherence (id 206, status gate)" \
  '! has_finding "$ar6out" aborted-run 206'

# --- GREEN: an artifact that is on the branch AND on the integration branch is already merged
# work, not an unrecorded artifact. The template's main carries
# docs/superpowers/plans/2026-06-01-present.md; a branch that merely inherits it must not fire.
read -r AR7 _ < <(new_repo)
git -C "$AR7" checkout -b feat/ar7 main >/dev/null 2>&1
echo unrelated > "$AR7/unrelated.txt"; git -C "$AR7" add unrelated.txt
git_quiet -C "$AR7" commit -m "unrelated code commit"
git -C "$AR7" checkout docket >/dev/null 2>&1
cat > "$AR7/docs/changes/active/0207-inherited.md" <<EOF
---
id: 207
slug: inherited-artifact
title: Branch inherits main's plan file only
status: in-progress
priority: medium
depends_on: []
branch: feat/ar7
plan:
results:
claimed_at: $(iso $(( NOW_EPOCH - 3600 )))
---
EOF
ar7out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR7/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run SILENT when the branch's only plan file is INHERITED from the integration branch (id 207)" \
  '! has_finding "$ar7out" aborted-run 207'

# --- The --results-dir flag: a repo whose results live somewhere else is honored.
read -r AR8 _ < <(new_repo)
ar_branch "$AR8" feat/ar8 "docs/custom-results/2026-08-03-x-results.md"
cat > "$AR8/docs/changes/active/0208-custom-results.md" <<'EOF'
---
id: 208
slug: custom-results
title: Custom results dir
status: in-progress
priority: medium
depends_on: []
branch: feat/ar8
plan: docs/superpowers/plans/2026-06-01-present.md
results:
---
EOF
ar8_default="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR8/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run SILENT for a custom results dir when --results-dir is NOT passed (default docs/results)" \
  '! has_finding "$ar8_default" aborted-run 208'
ar8_custom="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR8/docs/changes" --metadata-branch docket --integration-branch main --results-dir docs/custom-results 2>/dev/null)"
assert "aborted-run FIRES for a custom results dir when --results-dir names it (id 208)" \
  'has_finding "$ar8_custom" aborted-run 208'

# ---------------- aborted-run, leg B: run-scale stale claim (12h, hardcoded) ----------------
# The abort that leaves NOTHING in git — the originating incident, where the plan was written but
# never committed, so leg A has no artifact to see. Deliberately a separate check-id from
# stale-in-progress rather than a retuned one: that check's 72h lease / 3-day branch-idle horizons
# are human-scale abandonment signals with a different remedy and a machine contract
# (the trailing [reclaimable] marker) that docket-status already keys on.
AR_STALE_CLAIM="$(iso $(( NOW_EPOCH - 13*3600 )))"   # 13h  > 12h  => fires
AR_FRESH_CLAIM="$(iso $(( NOW_EPOCH - 11*3600 )))"   # 11h  < 12h  => silent
AR_LEASE_STALE_CLAIM="$(iso $(( NOW_EPOCH - 100*3600 )))"  # 100h > 72h lease AND > 12h => BOTH checks fire

# ---------------- branch_only_artifact: C-quoted paths (change 0202, finding 4) ----------------
# These are leg-A fixtures, placed here rather than beside the other leg-A ones because they consume
# `AR_FRESH_CLAIM`, which is defined just above: under this file's `set -u`, a heredoc referencing it
# any earlier aborts the run outright.
# `git ls-tree --name-only` C-quotes any path with a quote, a backslash, a control character, or
# (under the default core.quotePath=true) a non-ASCII byte. git_has would then look up the literal
# quoted string, fail, and report an INHERITED artifact as branch-only — a false positive. The fix
# reads the listing NUL-delimited (-z), which suppresses quoting entirely.
# core.quotePath is set explicitly per-repo: it is git's default, but a developer's global config
# may turn it off, which would make the mutation below silently unreproducible.
AR_PLAN_UTF8="docs/superpowers/plans/2026-06-01-café-plan.md"

# ARQ1 — SANITY: the non-ASCII plan is branch-only. Leg A fires. Proves the NUL plumbing reads a
# real path; does NOT discriminate the mutation (a branch-only path fails git_has either way).
read -r ARQ1 _ < <(new_repo)
git -C "$ARQ1" config core.quotePath true
ar_branch "$ARQ1" feat/arq1 "$AR_PLAN_UTF8"
cat > "$ARQ1/docs/changes/active/0230-utf8-branchonly.md" <<EOF
---
id: 230
slug: utf8-branchonly
title: Non-ASCII plan committed on the branch only
status: in-progress
priority: medium
depends_on: []
branch: feat/arq1
plan:
results:
claimed_at: $AR_FRESH_CLAIM
---
EOF
arq1out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$ARQ1/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "0202: leg A fires for a branch-only plan with a non-ASCII path (id 230, NUL plumbing reads it)" \
  'has_finding "$arq1out" aborted-run 230'
# The reported path must be the REAL path, not a C-quoted rendering of it.
arq1line="$(grep -E "$(printf "^aborted-run\t230\t")" <<<"$arq1out")"
assert "0202: the leg-A finding reports the unquoted non-ASCII path (id 230)" \
  'grep -qF "$AR_PLAN_UTF8" <<<"$arq1line"'

# ARQ2 — INHERITED (the discriminating fixture): the non-ASCII plan is on main, so the branch
# INHERITS it and it is NOT branch-only. Fixed script: SILENT. Mutated: FIRES (the false positive).
# The "only" is load-bearing — branch_only_artifact returns the FIRST non-inherited path it finds,
# so any stray branch-only plan in this repo would mask the assert.
read -r ARQ2 _ < <(new_repo)
git -C "$ARQ2" config core.quotePath true
git -C "$ARQ2" checkout main >/dev/null 2>&1
mkdir -p "$ARQ2/$(dirname "$AR_PLAN_UTF8")"
printf '# artifact\n' > "$ARQ2/$AR_PLAN_UTF8"
git -C "$ARQ2" add "$AR_PLAN_UTF8"; git_quiet -C "$ARQ2" commit -m "non-ASCII plan on main"
git -C "$ARQ2" branch feat/arq2 main
git -C "$ARQ2" checkout docket >/dev/null 2>&1
cat > "$ARQ2/docs/changes/active/0231-utf8-inherited.md" <<EOF
---
id: 231
slug: utf8-inherited
title: Non-ASCII plan inherited from the integration branch
status: in-progress
priority: medium
depends_on: []
branch: feat/arq2
plan:
results:
claimed_at: $AR_FRESH_CLAIM
---
EOF
arq2out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$ARQ2/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "0202: leg A SILENT for an INHERITED non-ASCII plan (id 231, no C-quoting false positive)" \
  '! has_finding "$arq2out" aborted-run 231'
# Non-vacuity: the fixture must actually have a plan file on its branch, or the silence above
# would be the trivial empty-listing silence rather than the inherited-path silence.
assert "0202: fixture 231's branch really does carry the non-ASCII plan (assert is not vacuous)" \
  'git -C "$ARQ2" cat-file -e "feat/arq2:$AR_PLAN_UTF8"'
assert "0202: fixture 231's non-ASCII plan is also on main (that is what makes it inherited)" \
  'git -C "$ARQ2" cat-file -e "main:$AR_PLAN_UTF8"'

# ARQ3 — a branch-only plan whose PATH embeds an interior newline (change 0200). Since change 0202
# leg A reads the listing NUL-delimited, so a git path arrives RAW and $ar_hit carries the LF all
# the way into emit. sanitize must escape it: otherwise one finding becomes TWO TSV records, and
# the caller (docket-status.sh's health_checks, `IFS=$'\t' read -r check_id change_id message`)
# reads the orphaned tail as a finding of its own — and the trailing `sort` reorders it anywhere.
AR_PLAN_LF="$(printf 'docs/superpowers/plans/2026-06-01-multi\nline-plan.md')"
AR_LF_ESCAPED='multi\nline-plan.md'
read -r ARQ3 _ < <(new_repo)
ar_branch "$ARQ3" feat/arq3 "$AR_PLAN_LF"
cat > "$ARQ3/docs/changes/active/0249-lf-path-branchonly.md" <<EOF
---
id: 249
slug: lf-path-branchonly
title: Plan path with an embedded newline committed on the branch only
status: in-progress
priority: medium
depends_on: []
branch: feat/arq3
plan:
results:
claimed_at: $AR_FRESH_CLAIM
---
EOF
arq3out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$ARQ3/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "0200: leg A fires for a branch-only plan whose path embeds a newline (id 249)" \
  'has_finding "$arq3out" aborted-run 249'
# Non-vacuity, through the same tree the check reads: without this, the discriminating assert below
# could be measuring "the fixture never had an LF-named plan" rather than "the LF was escaped".
assert "0200: fixture 249's branch really carries the LF-named plan (assert is not vacuous)" \
  'git -C "$ARQ3" cat-file -e "feat/arq3:$AR_PLAN_LF"'
# THE DISCRIMINATOR — and note what it deliberately is NOT. A "exactly one line matches the
# aborted-run<TAB>249<TAB> prefix" count passes in BOTH directions: unfixed, that prefix line still
# exists, it just ends at "…multi" with "line-plan.md) but plan: is unset …" orphaned on the next
# record. What only the fixed script can produce is the two-character escape \n WITH the post-LF
# tail on the SAME line.
arq3line="$(grep -E "$(printf "^aborted-run\t249\t")" <<<"$arq3out")"
assert "0200: the leg-A finding stays ONE TSV record — the interior LF is escaped to a visible \\n" \
  'grep -qF "$AR_LF_ESCAPED" <<<"$arq3line"'

read -r AR10 _ < <(new_repo)
cat > "$AR10/docs/changes/active/0210-stale-claim.md" <<EOF
---
id: 210
slug: stale-claim
title: Claim older than the run-scale window
status: in-progress
priority: medium
depends_on: []
branch:
plan:
results:
claimed_at: $AR_STALE_CLAIM
---
EOF
ar10out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR10/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run leg B fires for a claim older than 12h (id 210)" \
  'has_finding "$ar10out" aborted-run 210'
ar10line="$(grep -E "$(printf "^aborted-run\t210\t")" <<<"$ar10out")"
assert "the leg-B finding reports the claim age in hours (id 210)" \
  'grep -qE "13h" <<<"$ar10line"'
# stale-in-progress must stay SILENT here: 13h is far inside its 72h lease TTL. This is the whole
# point of the separate check-id — the two predicates must not have become one.
assert "stale-in-progress SILENT on the same 13h claim (id 210, the two horizons stay distinct)" \
  '! has_finding "$ar10out" stale-in-progress 210'

read -r AR11 _ < <(new_repo)
cat > "$AR11/docs/changes/active/0211-fresh-claim.md" <<EOF
---
id: 211
slug: fresh-claim
title: Claim inside the run-scale window
status: in-progress
priority: medium
depends_on: []
branch:
plan:
results:
claimed_at: $AR_FRESH_CLAIM
---
EOF
ar11out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR11/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run leg B SILENT for a claim 11h old (id 211, just inside the window)" \
  '! has_finding "$ar11out" aborted-run 211'

# No claimed_at at all => no positive evidence => silent (never treated as infinitely old).
read -r AR12 _ < <(new_repo)
cat > "$AR12/docs/changes/active/0212-no-claim.md" <<'EOF'
---
id: 212
slug: no-claim
title: In-progress with no claimed_at
status: in-progress
priority: medium
depends_on: []
branch:
plan:
results:
claimed_at:
---
EOF
ar12out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR12/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run leg B SILENT when claimed_at is absent (id 212, no positive evidence)" \
  '! has_finding "$ar12out" aborted-run 212'

# An unparseable claimed_at is also no positive evidence — never an exception, never "expired".
read -r AR13 _ < <(new_repo)
cat > "$AR13/docs/changes/active/0213-bad-claim.md" <<'EOF'
---
id: 213
slug: bad-claim
title: In-progress with a malformed claimed_at
status: in-progress
priority: medium
depends_on: []
branch:
plan:
results:
claimed_at: not-a-timestamp
---
EOF
ar13out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR13/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run leg B SILENT for an unparseable claimed_at (id 213)" \
  '! has_finding "$ar13out" aborted-run 213'

# BOTH legs on one change: an unrecorded plan AND a stale claim => TWO findings, not one.
# The legs are independent evidence, and collapsing them would hide whichever fired second.
read -r AR14 _ < <(new_repo)
ar_branch "$AR14" feat/ar14 "$AR_PLAN_NEW"
cat > "$AR14/docs/changes/active/0214-both-legs.md" <<EOF
---
id: 214
slug: both-legs
title: Unrecorded plan and a stale claim
status: in-progress
priority: medium
depends_on: []
branch: feat/ar14
plan:
results:
claimed_at: $AR_STALE_CLAIM
---
EOF
ar14out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR14/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "both aborted-run legs fire independently on one change (id 214, exactly 2 findings)" \
  '[ "$(grep -cE "$(printf "^aborted-run\t214\t")" <<<"$ar14out")" = 2 ]'
# Same reasoning as the id-201 count above: leg C (change 0211) shares this check-id and 214 meets
# three of its conjuncts, so the "exactly 2" count means "leg A plus leg B" only for as long as
# ar_branch's wall-clock dating keeps the idle floor's delta negative. Pin which two legs they are.
ar14line="$(grep -E "^aborted-run"$'\t'"214"$'\t' <<<"$ar14out")"
assert "leg C stays out of the id-214 count: the two findings are legs A and B, not leg C" \
  '! grep -qF "pr: is unset" <<<"$ar14line"'

# Status gate again, on leg B this time: a 'proposed' change with an ancient claimed_at is silent.
read -r AR15 _ < <(new_repo)
cat > "$AR15/docs/changes/active/0215-proposed-stale.md" <<EOF
---
id: 215
slug: proposed-stale
title: Proposed with an old claimed_at
status: proposed
priority: medium
depends_on: []
branch:
plan:
results:
claimed_at: $AR_STALE_CLAIM
---
EOF
ar15out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR15/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run leg B SILENT on a 'proposed' change with an old claimed_at (id 215, status gate)" \
  '! has_finding "$ar15out" aborted-run 215'

# ---------------- aborted-run leg C: built but not delivered (change 0211) ----------------
# The signature legs A and B are both blind to: the build finished, the delivery did not. Fixture
# ids start at 232 — 220-226 are the ARM mutation repo and 230-231 are the ARQ non-ASCII fixtures.

# --- RED: commits on an UNPUSHED branch, quiet 3h, pr: unset -> leg C, "never pushed" message ---
read -r AR16 _ < <(new_repo)
ar_branch_at "$AR16" feat/ar16 main $(( 3*3600 )) "$AR_PLAN_NEW"
# Sanity: the branch really is unpushed. Without this the "never pushed" assert below could pass
# because the push silently failed rather than because the message branch is right.
assert "leg C fixture 232 precondition: feat/ar16 has NO remote-tracking ref" \
  '! git -C "$AR16" show-ref --verify --quiet refs/remotes/origin/feat/ar16'
cat > "$AR16/docs/changes/active/0232-built-unpushed.md" <<EOF
---
id: 232
slug: built-unpushed
title: Build finished, branch never pushed
status: in-progress
priority: medium
depends_on: []
branch: feat/ar16
plan: docs/superpowers/plans/2026-06-01-present.md
results:
pr:
claimed_at: $AR_FRESH_CLAIM
---
EOF
ar16out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR16/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run leg C FIRES on an unpushed branch ahead of main, quiet 3h, pr: unset (id 232)" \
  'has_finding "$ar16out" aborted-run 232'
assert "leg C names the NEVER-PUSHED remedy on 232" \
  'grep -qF "branch never pushed and pr: is unset" <<<"$ar16out"'
# The count excludes BOTH bases, so the message names both — and singularizes the noun at 1.
assert "leg C reports the commit count and the bases it was measured against on 232" \
  'grep -qF "1 commit on feat/ar16 ahead of main and origin/main" <<<"$ar16out"'
assert "leg C does not render \"1 commits\" on 232" \
  '! grep -qF "1 commits" <<<"$ar16out"'
assert "leg C reports the branch idle age in hours on 232" \
  'grep -qF "(last commit 3h ago)" <<<"$ar16out"'
assert "leg C is SILENT for leg A on 232 — plan: is recorded, so the legs did not double-count" \
  '! grep -qF "but plan: is unset" <<<"$ar16out"'
# The REGISTER, pinned as deliberately as the evidence. Leg C's predicate fires on healthy runs by
# construction (the idle floor's known residual), so the message may not assert the abort as fact;
# and its remedy may not be a state change, because "push it / open the PR" acted on against a run
# that is merely between commits races the running agent on its own branch.
assert "leg C HEDGES on 232 and prescribes verification, not a push" \
  'grep -qF "a run may have stopped before it pushed; verify it is not still building" <<<"$ar16out" &&
   ! grep -qF "push and open it" <<<"$ar16out"'

# --- RED: the same branch PUSHED, pr: still unset -> leg C, push/PR-seam message ---
read -r AR17 _ < <(new_repo)
ar_branch_at "$AR17" feat/ar17 main $(( 3*3600 )) "$AR_PLAN_NEW"
ar_push "$AR17" feat/ar17
assert "leg C fixture 233 precondition: feat/ar17 HAS a remote-tracking ref (the push landed)" \
  'git -C "$AR17" show-ref --verify --quiet refs/remotes/origin/feat/ar17'
cat > "$AR17/docs/changes/active/0233-built-pushed.md" <<EOF
---
id: 233
slug: built-pushed
title: Branch pushed, PR never recorded
status: in-progress
priority: medium
depends_on: []
branch: feat/ar17
plan: docs/superpowers/plans/2026-06-01-present.md
results:
pr:
claimed_at: $AR_FRESH_CLAIM
---
EOF
ar17out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR17/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run leg C FIRES on a PUSHED branch with pr: unset (id 233)" \
  'has_finding "$ar17out" aborted-run 233'
assert "leg C names the PUSH/PR-SEAM remedy on 233" \
  'grep -qF "feat/ar17 is pushed but pr: is unset" <<<"$ar17out"'
assert "leg C does NOT claim 233 was never pushed — the two messages are exclusive" \
  '! grep -qF "branch never pushed" <<<"$ar17out"'
assert "leg C HEDGES on 233 and prescribes verification, not opening the PR" \
  'grep -qF "a run may have stopped between its push and its PR record; verify the PR exists" <<<"$ar17out" &&
   ! grep -qF "open the PR or record it" <<<"$ar17out"'
# Leg B emits "a run may have stopped mid-step". Leg C hedges with its OWN seam instead, and that
# separation is what keeps a message-shape assert (mutation K's, below) able to discriminate: were
# leg C to reuse "mid-step", an assert keyed on it could be satisfied by a leg-B finding.
assert "leg C's hedge is not leg B's — \"mid-step\" stays leg-B-exclusive on both leg-C arms" \
  '! grep -qF "mid-step" <<<"$ar16out" && ! grep -qF "mid-step" <<<"$ar17out"'

# --- GREEN: the LIVE-RUN WINDOW. Identical to 232 except the branch is 30m old, not 3h. This is
# the fixture that proves the idle floor is real: board-checks.sh runs on every Board pass,
# INCLUDING the passes inside the very run being built, so without a floor leg C would fire on
# every healthy build for the whole build span.
read -r AR18 _ < <(new_repo)
ar_branch_at "$AR18" feat/ar18 main $(( 1800 )) "$AR_PLAN_NEW"
cat > "$AR18/docs/changes/active/0234-live-run.md" <<EOF
---
id: 234
slug: live-run
title: Build commits 30 minutes old, run still live
status: in-progress
priority: medium
depends_on: []
branch: feat/ar18
plan: docs/superpowers/plans/2026-06-01-present.md
results:
pr:
claimed_at: $AR_FRESH_CLAIM
---
EOF
ar18out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR18/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run leg C SILENT inside the live-run window — branch quiet only 30m (id 234)" \
  '! has_finding "$ar18out" aborted-run 234'

# --- GREEN: DELIVERED. Pushed, quiet 3h, ahead — every other conjunct true — but pr: is SET, so
# the free frontmatter read short-circuits the leg before a single git call.
read -r AR19 _ < <(new_repo)
ar_branch_at "$AR19" feat/ar19 main $(( 3*3600 )) "$AR_PLAN_NEW"
ar_push "$AR19" feat/ar19
cat > "$AR19/docs/changes/active/0235-delivered.md" <<EOF
---
id: 235
slug: delivered
title: Pushed and the PR is recorded
status: in-progress
priority: medium
depends_on: []
branch: feat/ar19
plan: docs/superpowers/plans/2026-06-01-present.md
results:
pr: https://github.com/o/r/pull/7
claimed_at: $AR_FRESH_CLAIM
---
EOF
ar19out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR19/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
# Leg D (change 0219) fires on 235 BY DESIGN — `pr:` recorded while `status:` is still in-progress
# is its entire subject — so plain aborted-run silence is no longer the right oracle for LEG C's
# silence here. This keys on `pr: is unset`, the clause board-checks.md pins as leg-C-exclusive, and
# the companion assert keeps it non-vacuous by pinning that leg D is what did speak.
ar19_235="$(grep -E "$(printf "^aborted-run\t235\t")" <<<"$ar19out")"
assert "aborted-run leg C SILENT when pr: is recorded — the delivered state (id 235)" \
  '! grep -qF -- "pr: is unset" <<<"$ar19_235"'
assert "the only aborted-run finding on 235 is leg D's, not leg C's (id 235)" \
  'grep -qF -- "pr: records https://github.com/o/r/pull/7" <<<"$ar19_235"'

# --- GREEN: NOTHING BUILT. The branch exists and is old, but carries ZERO commits of its own.
# This is the 0109 signature — a run that stopped with nothing built — which is leg B's territory,
# not leg C's. Leg C claims "built but not delivered"; with nothing built it must stay silent.
read -r AR20 _ < <(new_repo)
ar_branch_at "$AR20" feat/ar20 main $(( 3*3600 ))
cat > "$AR20/docs/changes/active/0236-nothing-built.md" <<EOF
---
id: 236
slug: nothing-built
title: Branch cut, nothing committed on it
status: in-progress
priority: medium
depends_on: []
branch: feat/ar20
plan:
results:
pr:
claimed_at: $AR_FRESH_CLAIM
---
EOF
ar20out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR20/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run leg C SILENT on a branch with zero commits ahead — the 0109 signature (id 236)" \
  '! has_finding "$ar20out" aborted-run 236'

# --- GREEN: a STALE LOCAL integration ref. Advance the fixture's own bare origin (and therefore
# refs/remotes/origin/main) WITHOUT moving local main, then cut the change's branch from
# origin/main with no commits of its own. `push origin tmp:main` does both in one call — no fetch
# needed, since new_repo's template already carries both refs.
#
# The advancing commits MUST be dated relative to NOW_EPOCH: the change's branch tip IS one of
# them, so with real wall-clock dates the idle floor would be false and this fixture could never
# discriminate — it would be green for the wrong reason, and mutation I could never fire.
#
# Under the correct BOTH-bases predicate this branch is ahead of nothing: SILENT. Under a
# local-ref-only predicate it inherits every commit origin/main gained, looks arbitrarily far
# ahead with an arbitrarily old tip, and fires.
#
# The advancing commit touches a NEUTRAL path — neither RESULTS_DIR_REL nor PLANS_DIR_REL. Leg A's
# `branch_only_artifact "$ar_ref" "$RESULTS_DIR_REL"` probe reads the SAME stale local main as its
# base, so a results-shaped advancing file would ride onto feat/ar21 and fire LEG A here, making
# the leg-C-silence assert below (and Task 3's mutation-K assert on the ARM twin) vacuous.
read -r AR21 _ < <(new_repo)
ar_branch_at "$AR21" tmp-advance main $(( 3*3600 )) "docs/notes/2026-06-02-advance.md"
git -C "$AR21" push -q origin tmp-advance:main >/dev/null 2>&1
assert "leg C fixture 237 precondition: origin/main is AHEAD of local main (the stale local ref)" \
  '[ "$(git -C "$AR21" rev-parse refs/remotes/origin/main)" != "$(git -C "$AR21" rev-parse refs/heads/main)" ]'
ar_branch_at "$AR21" feat/ar21 refs/remotes/origin/main 0
cat > "$AR21/docs/changes/active/0237-stale-local-base.md" <<EOF
---
id: 237
slug: stale-local-base
title: Branch cut from origin/main while local main lags
status: in-progress
priority: medium
depends_on: []
branch: feat/ar21
plan:
results:
pr:
claimed_at: $AR_FRESH_CLAIM
---
EOF
ar21out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR21/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run leg C SILENT when the branch is ahead of a STALE LOCAL main but not of origin/main (id 237)" \
  '! has_finding "$ar21out" aborted-run 237'

# --- RED x2: leg A and leg C on ONE change, proving the legs stayed independent. The branch
# carries an unrecorded PLAN (leg A) and is quiet, ahead, unpushed with pr: unset (leg C). Same
# shape as the existing "BOTH legs" fixture for A+B; both messages are self-contained, so
# docket-status printing two lines with different remedies for one change needs no caller change.
read -r AR22 _ < <(new_repo)
ar_branch_at "$AR22" feat/ar22 main $(( 3*3600 )) "$AR_PLAN_NEW"
cat > "$AR22/docs/changes/active/0238-both-a-and-c.md" <<EOF
---
id: 238
slug: both-a-and-c
title: Unrecorded plan on a quiet unpushed branch
status: in-progress
priority: medium
depends_on: []
branch: feat/ar22
plan:
results:
pr:
claimed_at: $AR_FRESH_CLAIM
---
EOF
ar22out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR22/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run fires on 238 (legs A and C together)" 'has_finding "$ar22out" aborted-run 238'
assert "legs A and C are INDEPENDENT: leg A's unrecorded-plan message is present on 238" \
  'grep -qF "but plan: is unset" <<<"$ar22out"'
assert "legs A and C are INDEPENDENT: leg C's never-pushed message is ALSO present on 238" \
  'grep -qF "branch never pushed and pr: is unset" <<<"$ar22out"'
# Computed OUTSIDE the assert: the assert argument is eval'd, and burying a TAB-bearing pattern in
# it means escaping out of and back into single quotes for every field separator.
ar22n="$(grep -cE "^aborted-run"$'\t'"238"$'\t' <<<"$ar22out")"
assert "238 emits exactly TWO aborted-run lines — one per leg, not one merged or three" \
  '[ "$ar22n" = 2 ]'

# --- RED: the PLURAL arm of the count noun. Every other firing leg-C fixture is exactly one commit
# ahead, so without this the `commits` branch is unreachable and a mutation pinning the noun to the
# singular would stay green. Two commits, both dated relative to NOW_EPOCH — the tip's age is what
# the idle floor reads, and a real wall-clock date would make it negative (see ar_branch_at).
read -r AR23 _ < <(new_repo)
ar_branch_at "$AR23" feat/ar23 main $(( 3*3600 )) "$AR_PLAN_NEW"
git -C "$AR23" checkout feat/ar23 >/dev/null 2>&1
# A NEUTRAL path (neither PLANS_DIR_REL nor RESULTS_DIR_REL), so the extra commit cannot make leg A
# fire here and turn the count assert below into a two-leg accident.
mkdir -p "$AR23/docs/notes"
printf '# second\n' > "$AR23/docs/notes/2026-06-03-second.md"
git -C "$AR23" add docs/notes/2026-06-03-second.md
GIT_AUTHOR_DATE="@$(( NOW_EPOCH - 3*3600 ))" GIT_COMMITTER_DATE="@$(( NOW_EPOCH - 3*3600 ))" \
  git -C "$AR23" commit -m "second commit on feat/ar23" >/dev/null 2>&1
git -C "$AR23" checkout docket >/dev/null 2>&1
assert "leg C fixture 239 precondition: feat/ar23 is TWO commits ahead of both bases" \
  '[ "$(git -C "$AR23" rev-list --count feat/ar23 --not refs/heads/main refs/remotes/origin/main)" = 2 ]'
cat > "$AR23/docs/changes/active/0239-two-commits.md" <<EOF
---
id: 239
slug: two-commits
title: Two commits built, nothing delivered
status: in-progress
priority: medium
depends_on: []
branch: feat/ar23
plan: docs/superpowers/plans/2026-06-01-present.md
results:
pr:
claimed_at: $AR_FRESH_CLAIM
---
EOF
ar23out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR23/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "leg C pluralizes the count noun above one commit (id 239)" \
  'grep -qF "2 commits on feat/ar23 ahead of main and origin/main" <<<"$ar23out"'

# --- GREEN: NO BASE RESOLVES. Run against an integration branch that exists as neither
# refs/heads/<b> nor refs/remotes/origin/<b>, so ar_bases comes out EMPTY and leg C's count gate
# short-circuits the whole predicate. board-checks.md's claim for this case is "No base resolving
# at all is silence, never 'ahead of nothing'" — this is the fixture that makes it assertable.
#
# Silence ALONE is weak evidence, because an aborted script is also silent. So the repo carries a
# SECOND change, 248, that sorts AFTER 247 in the walk (`find … | sort`, by filename) and produces
# an UNRELATED finding: broken-spec, which resolves against the METADATA branch and so is entirely
# untouched by the bogus integration branch. Asserting 248's finding is PRESENT is what
# distinguishes "leg C declined correctly" from "the walk died before reaching 248".
#
# plan: and results: are SET on 247 deliberately. With a bogus integration branch every git_has
# against it fails, so branch_only_artifact would report the template's INHERITED plan file as
# branch-only and fire LEG A here — masking exactly what this fixture measures. The set fields
# cannot themselves fire broken-plan-results, which is gated on `status: done`.
read -r AR24 _ < <(new_repo)
ar_branch_at "$AR24" feat/ar24 main $(( 3*3600 )) "docs/notes/2026-06-04-nobase.md"
assert "leg C fixture 247 precondition: 'nosuchbranch' resolves as NEITHER base (ar_bases is empty)" \
  '! git -C "$AR24" show-ref --verify --quiet refs/heads/nosuchbranch &&
   ! git -C "$AR24" show-ref --verify --quiet refs/remotes/origin/nosuchbranch'
# Non-vacuity: every OTHER leg-C conjunct must hold, or 247 would be silent for some other reason
# and mutation M would prove nothing.
assert "leg C fixture 247 precondition: feat/ar24 is genuinely built and its tip clears the idle floor" \
  '[ -n "$(git -C "$AR24" rev-list -n 1 feat/ar24 --not refs/heads/main refs/remotes/origin/main)" ] &&
   [ "$(( NOW_EPOCH - $(git -C "$AR24" log -1 --format=%ct feat/ar24) ))" -gt 7200 ]'
cat > "$AR24/docs/changes/active/0247-no-base.md" <<EOF
---
id: 247
slug: no-base
title: Built branch, integration branch resolves to nothing
status: in-progress
priority: medium
depends_on: []
branch: feat/ar24
plan: docs/superpowers/plans/2026-06-01-present.md
results: docs/results/2026-06-01-present-results.md
pr:
claimed_at: $AR_FRESH_CLAIM
---
EOF
cat > "$AR24/docs/changes/active/0248-later-finding.md" <<'EOF'
---
id: 248
slug: later-finding
title: Sorts after 247 and carries an unrelated defect
status: proposed
priority: medium
depends_on: []
spec: docs/superpowers/specs/2026-06-01-absent.md
---
EOF
ar24err="$(mktemp)"
ar24out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR24/docs/changes" --metadata-branch docket --integration-branch nosuchbranch 2>"$ar24err")"
assert "aborted-run leg C SILENT when NO base resolves (id 247) — silence, never 'ahead of nothing'" \
  '! has_finding "$ar24out" aborted-run 247'
assert "the walk SURVIVED the declining leg: the later change 248 still reports broken-spec" \
  'has_finding "$ar24out" broken-spec 248'
assert "the no-base run writes NOTHING to stderr — the count gate short-circuits before the expansion" \
  '[ ! -s "$ar24err" ]'
rm -f "$ar24err"

# ---------------- aborted-run, leg D: pr: recorded, status: never advanced (change 0219) ----------------
# The Step 7 seam. docket-implement-next writes `status: implemented` and `pr:` in ONE field-write,
# and no script under scripts/ writes pr: — so a manifest carrying pr: while still in-progress is an
# anomaly BY CONSTRUCTION. Time-free for that reason, exactly like leg A: there is no healthy window
# to wait out, so an idle floor would only delay a finding that is already certain.
#
# Leg D is the pr:-NON-empty arm of the same hoisted read whose pr:-empty arm is leg C, so the two
# are mutually exclusive by construction and one fixture can never produce both.

# --- RED: pr: set while status: is still in-progress ---
read -r AR_D1 _ < <(new_repo)
cat > "$AR_D1/docs/changes/active/0260-pr-recorded-status-stale.md" <<'EOF'
---
id: 260
slug: pr-recorded-status-stale
title: PR recorded, status never advanced
status: in-progress
priority: medium
depends_on: []
branch: feat/ar-d1
plan: docs/superpowers/plans/2026-06-01-present.md
results:
pr: 314
---
EOF
ard1out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR_D1/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run leg D fires when pr: is set but status: is still in-progress (id 260)" \
  'has_finding "$ard1out" aborted-run 260'
ard1line="$(grep -E "$(printf "^aborted-run\t260\t")" <<<"$ard1out")"
assert "the leg-D finding names the recorded PR and the missing status write (id 260)" \
  'grep -qF -- "pr: records 314" <<<"$ard1line" && grep -qF -- "status: is still in-progress" <<<"$ard1line"'
assert "the leg-D remedy is a VERIFICATION, and names the status it should reach (id 260)" \
  'grep -qF -- "verify the PR and set status: implemented" <<<"$ard1line"'
# Leg D must not borrow leg C's or leg B's exclusive clause: board-checks.md pins `pr: is unset` as
# leg-C-exclusive and `mid-step` as leg-B-exclusive so a message-shape assert can tell legs apart.
assert "leg D does not reuse leg C's exclusive 'pr: is unset' clause (id 260)" \
  '! grep -qF -- "pr: is unset" <<<"$ard1line"'
assert "leg D does not reuse leg B's exclusive 'mid-step' phrasing (id 260)" \
  '! grep -qF -- "mid-step" <<<"$ard1line"'
assert "aborted-run fires exactly ONCE for id 260 (leg B stays silent on an absent claimed_at)" \
  '[ "$(grep -cE "$(printf "^aborted-run\t260\t")" <<<"$ard1out")" = 1 ]'

# --- GREEN: pr: set AND status: implemented — the delivered change, the whole point of the gate ---
read -r AR_D2 _ < <(new_repo)
cat > "$AR_D2/docs/changes/active/0261-pr-recorded-delivered.md" <<'EOF'
---
id: 261
slug: pr-recorded-delivered
title: PR recorded and status advanced
status: implemented
priority: medium
depends_on: []
branch: feat/ar-d2
plan: docs/superpowers/plans/2026-06-01-present.md
results:
pr: 315
---
EOF
ard2out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR_D2/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run SILENT when pr: is set and status: is implemented (id 261, status gate)" \
  '! has_finding "$ard2out" aborted-run 261'

# --- GREEN: pr: empty — leg C's domain, and leg D must not poach it ---
read -r AR_D3 _ < <(new_repo)
cat > "$AR_D3/docs/changes/active/0262-pr-empty.md" <<'EOF'
---
id: 262
slug: pr-empty
title: In-progress with no PR recorded and no branch
status: in-progress
priority: medium
depends_on: []
branch:
plan: docs/superpowers/plans/2026-06-01-present.md
results:
pr:
---
EOF
ard3out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR_D3/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run SILENT for an in-progress change with an empty pr: and no branch (id 262)" \
  '! has_finding "$ard3out" aborted-run 262'

# --- RED: the ANCHORED read. Frontmatter OMITS pr: entirely while the body opens a `pr:` line.
# An unanchored read returns the body prose and fires leg D on a change that has no PR at all —
# the ADR-0057 shape, here producing a FALSE POSITIVE (the mirror of leg A's false negative).
# The natural fixture (a file that HAS pr:) passes under both implementations, so this
# absent-key one is the only fixture that discriminates. Paired with mutation N below: the fixture
# is inert without a mutation that reaches it, and the mutation is inert without this fixture.
read -r AR_D4 _ < <(new_repo)
cat > "$AR_D4/docs/changes/active/0263-pr-prose-only.md" <<'EOF'
---
id: 263
slug: pr-prose-only
title: Body prose mentions pr but frontmatter omits it
status: in-progress
priority: medium
depends_on: []
branch:
plan: docs/superpowers/plans/2026-06-01-present.md
results:
---

## Notes
pr: 999 was never opened for this change
EOF
ard4out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AR_D4/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "aborted-run leg D SILENT when only body prose mentions pr: (id 263, anchored read)" \
  '! has_finding "$ard4out" aborted-run 263'

# ---------------- aborted-run mutation tests (guards-are-code) ----------------
# Each predicate is broken in a throwaway COPY of board-checks.sh and watched change the fixtures'
# outcome. Every mutation runs against a FRESH pristine copy (never a cumulative chain) and is
# CONFIRMED LANDED with a grep -c before/after before its result is believed.
read -r ARM _ < <(new_repo)
ar_branch "$ARM" feat/arm-plan    "$AR_PLAN_NEW"
git -C "$ARM" checkout -b feat/arm-results main >/dev/null 2>&1
mkdir -p "$ARM/docs/results"
printf '# artifact\n' > "$ARM/$AR_RESULTS_NEW"
git -C "$ARM" add "$AR_RESULTS_NEW"; git_quiet -C "$ARM" commit -m "results on feat/arm-results"
git -C "$ARM" checkout docket >/dev/null 2>&1
# 220: unrecorded plan, FRESH claim  -> leg A (plan) only
cat > "$ARM/docs/changes/active/0220-mplan.md" <<EOF
---
id: 220
slug: mplan
title: Unrecorded plan
status: in-progress
priority: medium
depends_on: []
branch: feat/arm-plan
plan:
results:
claimed_at: $AR_FRESH_CLAIM
---
EOF
# 221: unrecorded results, FRESH claim -> leg A (results) only
cat > "$ARM/docs/changes/active/0221-mresults.md" <<EOF
---
id: 221
slug: mresults
title: Unrecorded results
status: in-progress
priority: medium
depends_on: []
branch: feat/arm-results
plan: docs/superpowers/plans/2026-06-01-present.md
results:
claimed_at: $AR_FRESH_CLAIM
---
EOF
# 222: STALE claim, no branch -> leg B only
cat > "$ARM/docs/changes/active/0222-mclaim.md" <<EOF
---
id: 222
slug: mclaim
title: Stale claim only
status: in-progress
priority: medium
depends_on: []
branch:
plan:
results:
claimed_at: $AR_STALE_CLAIM
---
EOF
# 223: plan absent from FRONTMATTER, present in BODY prose, unrecorded plan on the branch, fresh
# claim -> leg A fires under the ANCHORED read and goes silent under an unanchored one.
cat > "$ARM/docs/changes/active/0223-manchor.md" <<EOF
---
id: 223
slug: manchor
title: Body prose mentions plan
status: in-progress
priority: medium
depends_on: []
branch: feat/arm-plan
results:
claimed_at: $AR_FRESH_CLAIM
---

## Notes
plan: docs/superpowers/plans/2026-06-01-present.md
EOF
# 224: results absent from FRONTMATTER, present in BODY prose, unrecorded results on the branch,
# fresh claim -> leg A's RESULTS arm fires under the ANCHORED read and goes silent under an
# unanchored one. The exact mirror of 223, which pins the same property for the plan arm.
# plan: is SET so the plan arm cannot contribute the finding and mask what this fixture measures.
cat > "$ARM/docs/changes/active/0224-manchor-results.md" <<EOF
---
id: 224
slug: manchor-results
title: Body prose mentions results
status: in-progress
priority: medium
depends_on: []
branch: feat/arm-results
plan: docs/superpowers/plans/2026-06-01-present.md
claimed_at: $AR_FRESH_CLAIM
---

## Notes
results: docs/results/2026-06-01-present-results.md
EOF
# 225: healthy fields, but the branch DOES carry a branch-only plan. Baseline: SILENT (plan: is
# set, so leg A's -z guard correctly declines). Under mutation A (-z -> -n) it MISFIRES. This is
# the fixture mutation A's "both directions" claim needs — 221 cannot serve, because its branch
# (feat/arm-results) carries no plan file, leaving the misfire conjunct unreachable.
cat > "$ARM/docs/changes/active/0225-mhealthy.md" <<EOF
---
id: 225
slug: mhealthy
title: Recorded plan and results, plan file present on the branch
status: in-progress
priority: medium
depends_on: []
branch: feat/arm-plan
plan: docs/superpowers/plans/2026-06-01-present.md
results: docs/results/2026-06-01-present-results.md
claimed_at: $AR_FRESH_CLAIM
---
EOF
# 226: no branch, claim 100h old — past BOTH the 12h aborted-run window and the 72h stale-in-progress
# lease, so BOTH checks fire at baseline. That is what makes mutation E's "stale-in-progress must
# stay unaffected" claim assertable: dropping the aborted-run block must remove exactly one of them.
cat > "$ARM/docs/changes/active/0226-mboth-checks.md" <<EOF
---
id: 226
slug: mboth-checks
title: Claim past both the run window and the lease
status: in-progress
priority: medium
depends_on: []
branch:
plan:
results:
claimed_at: $AR_LEASE_STALE_CLAIM
---
EOF

# --- leg C fixtures in the shared mutation repo (change 0211) ---------------------------------
# Two OLD-dated commits are laid down on the integration bases FIRST, because two of the leg-C
# mutation fixtures need a branch whose TIP IS OLD while its ahead-count is ZERO — a shape that
# cannot be built from the template's own main. new_repo's main tip carries a real wall-clock date
# (2026-08) while NOW_EPOCH is 1750000000 (2025-06), so `NOW - tip` is NEGATIVE for any branch cut
# from it with nothing committed: such a branch fails leg C's IDLE FLOOR and would be silent no
# matter what the ahead-of-bases predicate did. Fixtures 244 and 245 would then be green for the
# wrong reason and mutations H and K could never fire — the exact vacuity the landed-asserts exist
# to catch.
#
#   B1 — dated 3h before NOW_EPOCH, fast-forwarded onto BOTH local main and origin/main.
#        Fixture 244's branch is cut here: old tip, ahead of nothing.
#   B2 — dated 3h before NOW_EPOCH, pushed to origin/main ONLY, so the LOCAL integration ref lags.
#        Fixture 245's branch is cut here: old tip, ahead of LOCAL main but not of origin/main.
#
# Both advancing commits touch a NEUTRAL path — neither PLANS_DIR_REL nor RESULTS_DIR_REL. This is
# not cosmetic. Leg A's `branch_only_artifact "$ar_ref" "$RESULTS_DIR_REL"` resolves "branch-only"
# against the SAME stale local main, so a results-shaped or plan-shaped advancing file would ride
# onto feat/arm-c-frombase and fire LEG A on 245 — which would break the id-scoped
# "leg C SILENT on 245" baseline assert below AND make mutation K's `has_finding … 245` VACUOUSLY
# true, passing even with the both-bases predicate deleted.
#
# Safe for every other check in this repo: leg A compares file LISTS under the plans/results dirs
# (a docs/notes/ file appears in neither), broken-plan-results only asks whether the recorded
# plan/results paths still resolve on local main (they do — B1 only adds), and merged-orphan /
# unknown-commit-ref key on commit SUBJECTS matching a numeric conventional-commit scope or a
# "(change N)" tag, which "commit on tmp-base"/"commit on tmp-advance" match neither of.
ar_branch_at "$ARM" tmp-base main $(( 3*3600 )) "docs/notes/2026-06-02-arm-base.md"
git -C "$ARM" branch -f main tmp-base >/dev/null 2>&1
git -C "$ARM" push -q origin tmp-base:main >/dev/null 2>&1
ar_branch_at "$ARM" tmp-advance tmp-base $(( 3*3600 )) "docs/notes/2026-06-02-arm-advance.md"
git -C "$ARM" push -q origin tmp-advance:main >/dev/null 2>&1
assert "ARM precondition: origin/main is AHEAD of local main (fixture 245's stale local ref)" \
  '[ "$(git -C "$ARM" rev-parse refs/remotes/origin/main)" != "$(git -C "$ARM" rev-parse refs/heads/main)" ]'
# The idle floor must be SATISFIED by a branch sitting on either base, or 244/245 are silent for the
# wrong reason and mutations H and K prove nothing.
assert "ARM precondition: local main's tip is older than the leg-C idle floor (2h before NOW_EPOCH)" \
  '[ "$(( NOW_EPOCH - $(git -C "$ARM" log -1 --format=%ct refs/heads/main) ))" -gt 7200 ]'
assert "ARM precondition: origin/main's tip is older than the leg-C idle floor (2h before NOW_EPOCH)" \
  '[ "$(( NOW_EPOCH - $(git -C "$ARM" log -1 --format=%ct refs/remotes/origin/main) ))" -gt 7200 ]'

ar_branch_at "$ARM" feat/arm-c-unpushed main $(( 3*3600 )) "$AR_PLAN_NEW"
ar_branch_at "$ARM" feat/arm-c-pushed   main $(( 3*3600 )) "$AR_PLAN_NEW"
ar_push "$ARM" feat/arm-c-pushed
ar_branch_at "$ARM" feat/arm-c-live     main $(( 1800 ))   "$AR_PLAN_NEW"
ar_branch_at "$ARM" feat/arm-c-empty    main 0
ar_branch_at "$ARM" feat/arm-c-frombase refs/remotes/origin/main 0
ar_branch_at "$ARM" feat/arm-c-prose    main $(( 3*3600 )) "$AR_PLAN_NEW"
# Non-vacuity for mutations H and K: each fixture's baseline SILENCE must come from the predicate
# the mutation removes, not from some other conjunct that happens to decline first.
assert "ARM precondition: feat/arm-c-empty is ahead of NEITHER base (mutation H's silence is the ahead test)" \
  '[ -z "$(git -C "$ARM" rev-list -n 1 feat/arm-c-empty --not refs/heads/main refs/remotes/origin/main)" ]'
assert "ARM precondition: feat/arm-c-frombase is ahead of LOCAL main (mutation K has something to find)" \
  '[ -n "$(git -C "$ARM" rev-list -n 1 feat/arm-c-frombase --not refs/heads/main)" ]'
assert "ARM precondition: feat/arm-c-frombase is ahead of NEITHER base together (baseline silence is the remote base)" \
  '[ -z "$(git -C "$ARM" rev-list -n 1 feat/arm-c-frombase --not refs/heads/main refs/remotes/origin/main)" ]'

# 240: unpushed, quiet 3h, ahead, pr: unset -> leg C fires, "never pushed"
cat > "$ARM/docs/changes/active/0240-mc-unpushed.md" <<EOF
---
id: 240
slug: mc-unpushed
title: Built on an unpushed branch
status: in-progress
priority: medium
depends_on: []
branch: feat/arm-c-unpushed
plan: docs/superpowers/plans/2026-06-01-present.md
results:
pr:
claimed_at: $AR_FRESH_CLAIM
---
EOF
# 241: identical but only 30m quiet -> SILENT. Mutation G's fixture (the idle floor).
cat > "$ARM/docs/changes/active/0241-mc-live.md" <<EOF
---
id: 241
slug: mc-live
title: Build commits 30 minutes old
status: in-progress
priority: medium
depends_on: []
branch: feat/arm-c-live
plan: docs/superpowers/plans/2026-06-01-present.md
results:
pr:
claimed_at: $AR_FRESH_CLAIM
---
EOF
# 242: PUSHED, pr: unset -> leg C fires, push/PR seam. Mutation J's fixture (the message branch).
cat > "$ARM/docs/changes/active/0242-mc-pushed.md" <<EOF
---
id: 242
slug: mc-pushed
title: Pushed with no PR recorded
status: in-progress
priority: medium
depends_on: []
branch: feat/arm-c-pushed
plan: docs/superpowers/plans/2026-06-01-present.md
results:
pr:
claimed_at: $AR_FRESH_CLAIM
---
EOF
# 243: pushed AND pr: set -> SILENT. Mutation I's fixture (the pr: gate).
cat > "$ARM/docs/changes/active/0243-mc-delivered.md" <<EOF
---
id: 243
slug: mc-delivered
title: Pushed and delivered
status: in-progress
priority: medium
depends_on: []
branch: feat/arm-c-pushed
plan: docs/superpowers/plans/2026-06-01-present.md
results:
pr: https://github.com/o/r/pull/9
claimed_at: $AR_FRESH_CLAIM
---
EOF
# 244: branch with ZERO commits ahead -> SILENT. Mutation H's fixture (the ahead-of-bases test).
# Cut from B1, which sits on BOTH bases: nothing built, yet the tip is old enough to clear the idle
# floor, so the ONLY conjunct declining here is the ahead-of-bases test.
cat > "$ARM/docs/changes/active/0244-mc-empty.md" <<EOF
---
id: 244
slug: mc-empty
title: Branch cut, nothing built
status: in-progress
priority: medium
depends_on: []
branch: feat/arm-c-empty
plan:
results:
pr:
claimed_at: $AR_FRESH_CLAIM
---
EOF
# 245: cut from origin/main while LOCAL main lags -> SILENT. Mutation K's fixture (both bases).
cat > "$ARM/docs/changes/active/0245-mc-frombase.md" <<EOF
---
id: 245
slug: mc-frombase
title: Cut from origin/main, local main stale
status: in-progress
priority: medium
depends_on: []
branch: feat/arm-c-frombase
plan:
results:
pr:
claimed_at: $AR_FRESH_CLAIM
---
EOF
# 246: pr: absent from FRONTMATTER, present in BODY prose -> leg C fires under the ANCHORED read
# and goes silent under an unanchored one. Mutation L's fixture (ADR-0057), the exact shape 223/224
# pin for plan: and results:.
cat > "$ARM/docs/changes/active/0246-mc-prose.md" <<EOF
---
id: 246
slug: mc-prose
title: Body prose mentions pr
status: in-progress
priority: medium
depends_on: []
branch: feat/arm-c-prose
plan: docs/superpowers/plans/2026-06-01-present.md
results:
claimed_at: $AR_FRESH_CLAIM
---

## Notes
pr: https://github.com/o/r/pull/11
EOF

armcopy=""
armreseed(){
  [ -n "$armcopy" ] && rm -rf "$armcopy"; armcopy="$(mktemp -d)"
  mkdir -p "$armcopy/scripts/lib"
  cp "$SCRIPT" "$armcopy/scripts/board-checks.sh"
  cp "$REPO/scripts/lib/docket-frontmatter.sh" "$armcopy/scripts/lib/"
  ARMSCRIPT="$armcopy/scripts/board-checks.sh"
}
armrun_at(){ NOW=$NOW_EPOCH bash "$ARMSCRIPT" --changes-dir "$1/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null; }
armrun(){ armrun_at "$ARM"; }

# armrun_ib REPO INTEGRATION-BRANCH — run the mutated copy against REPO with a CALLER-CHOSEN
# integration branch, setting armib_out / armib_err / armib_status. A SIBLING of armrun_at, not a
# widening of it: armrun_at hardcodes `--integration-branch main` and DISCARDS stderr, and
# mutations A-L are all measured through it — changing its shape would change what they measure.
# Mutation M needs both axes: an integration branch that resolves to no ref at all, and stderr,
# which under a bash that errors on empty-array expansion is the ONLY channel the defect reaches.
#
# Sets globals rather than printing, and must therefore be CALLED DIRECTLY — `x="$(armrun_ib …)"`
# would run it in a subshell and the parent would see none of the three.
armib_out=""; armib_err=""; armib_status=0
armrun_ib(){
  local aib_errf; aib_errf="$(mktemp)"
  armib_out="$(NOW=$NOW_EPOCH bash "$ARMSCRIPT" --changes-dir "$1/docs/changes" \
    --metadata-branch docket --integration-branch "$2" 2>"$aib_errf")"
  armib_status=$?
  armib_err="$(cat "$aib_errf")"; rm -f "$aib_errf"
}

# Baseline: the un-mutated copy fires the expected findings, pinned one by one below. Deliberately
# no count here (change 0200) — the per-fixture asserts beneath ARE the guard, and the number this
# line used to carry had already drifted past what follows it with nothing to redden.
armreseed
arm0out="$(armrun)"
assert "mutation baseline: unmutated copy fires leg A on 220 (plan)" 'has_finding "$arm0out" aborted-run 220'
assert "mutation baseline: unmutated copy fires leg A on 221 (results)" 'has_finding "$arm0out" aborted-run 221'
assert "mutation baseline: unmutated copy fires leg B on 222 (stale claim)" 'has_finding "$arm0out" aborted-run 222'
assert "mutation baseline: unmutated copy fires leg A on 223 (anchored read)" 'has_finding "$arm0out" aborted-run 223'
assert "mutation baseline: unmutated copy fires leg A on 224 (anchored results read)" \
  'has_finding "$arm0out" aborted-run 224'
assert "mutation baseline: unmutated copy is SILENT on 225 (healthy fields, plan file on the branch)" \
  '! has_finding "$arm0out" aborted-run 225'
assert "mutation baseline: unmutated copy fires leg B on 226 (100h claim)" \
  'has_finding "$arm0out" aborted-run 226'
assert "mutation baseline: unmutated copy ALSO fires stale-in-progress on 226 (both checks, one change)" \
  'has_finding "$arm0out" stale-in-progress 226'
assert "mutation baseline: leg C fires on 240 (unpushed, quiet, ahead)" 'has_finding "$arm0out" aborted-run 240'
assert "mutation baseline: leg C fires on 242 (pushed, no PR)" 'has_finding "$arm0out" aborted-run 242'
assert "mutation baseline: leg C fires on 246 (pr: only in body prose)" 'has_finding "$arm0out" aborted-run 246'
assert "mutation baseline: leg C SILENT on 241 (live-run window)"   '! has_finding "$arm0out" aborted-run 241'
# 243 carries a recorded pr: on an in-progress change, so leg D (change 0219) speaks on it at
# baseline. Leg C's silence is measured through its exclusive `pr: is unset` clause instead.
arm0_243="$(grep -E "$(printf "^aborted-run\t243\t")" <<<"$arm0out")"
assert "mutation baseline: leg C SILENT on 243 (pr: recorded)" \
  '! grep -qF -- "pr: is unset" <<<"$arm0_243"'
assert "mutation baseline: leg D SPEAKS on 243 (pr: records …) — the aborted-run line exists, so leg C's silence above is not vacuous" \
  'grep -qF -- "pr: records" <<<"$arm0_243"'
assert "mutation baseline: leg C SILENT on 244 (nothing built)"     '! has_finding "$arm0out" aborted-run 244'
assert "mutation baseline: leg C SILENT on 245 (stale local main)"  '! has_finding "$arm0out" aborted-run 245'

# The EXISTING leg-A/B fixtures (220, 221, 223, 225) all have a branch that is ahead of main with
# pr: absent — three of leg C's four conjuncts. They stay leg-C-silent ONLY because ar_branch dates
# its commits with the real wall clock (2026-08) while NOW_EPOCH is 1750000000 (2025-06), making
# `NOW - ts` NEGATIVE and the idle floor false. That is an ACCIDENT of the harness, not an intent,
# and it is exactly what leg C's arrival makes load-bearing: the single-finding asserts above are
# otherwise guarded by nothing but the sign of that delta. Pin the intent explicitly, so that
# re-dating those fixtures later reddens HERE with a message that says why, instead of silently
# changing what mutations A-F measure.
# Computed outside the asserts, for the same reason as 238's count above.
# The absence asserts match on `pr: is unset` — the substring COMMON to both of leg C's messages,
# and emitted by neither leg A nor leg B (pinned on 201 and 214 above, whose twins of these asserts
# use the same string). Matching the never-pushed message alone would lean on an unasserted property
# of ar_branch (that it never pushes its branches): the day a fixture here gains a push, leg C could
# start firing the PUSHED message on it and these asserts would stay green.
arm0_legc="$(grep -cF "pr: is unset" <<<"$arm0out")"
arm0_220="$(grep -E "^aborted-run"$'\t'"220"$'\t' <<<"$arm0out")"
arm0_221="$(grep -E "^aborted-run"$'\t'"221"$'\t' <<<"$arm0out")"
arm0_223="$(grep -E "^aborted-run"$'\t'"223"$'\t' <<<"$arm0out")"
assert "leg C does not reach the existing leg-A fixtures: no leg-C message on 220" \
  '! grep -qF "pr: is unset" <<<"$arm0_220"'
assert "leg C does not reach the existing leg-A fixtures: no leg-C message on 221" \
  '! grep -qF "pr: is unset" <<<"$arm0_221"'
assert "leg C does not reach the existing leg-A fixtures: no leg-C message on 223" \
  '! grep -qF "pr: is unset" <<<"$arm0_223"'
# Non-vacuity companion for the three absence asserts above (they would all pass if leg C emitted
# nothing at all): the SAME string, through the SAME extractor, must be present somewhere in this
# very output — on the fixtures that are supposed to have it.
assert "a leg-C message IS present in this run — the three absence asserts are not vacuous" \
  '[ "$arm0_legc" -ge 1 ]'
assert "leg C SILENT on 225 (healthy fields) — the fixture stays a pure leg-A mutation target" \
  '! has_finding "$arm0out" aborted-run 225'

# Mutation A — invert leg A's plan emptiness test (-z becomes -n): the unrecorded-plan fixture 220
# goes GREEN and the healthy-field fixture 225 starts misfiring. Both directions. 225, not 221:
# 221's branch (feat/arm-results) carries no plan file, so the misfire conjunct is unreachable
# there — the guard would prove only half of what its comment claims (change 0202, finding 3).
armreseed
armA_before="$(grep -cF 'if [ -z "$(fm_field "$f" plan)" ]' "$ARMSCRIPT")"
awk '{ if ($0 ~ /fm_field "\$f" plan/) sub(/-z /, "-n "); print }' "$ARMSCRIPT" > "$ARMSCRIPT.t"; mv "$ARMSCRIPT.t" "$ARMSCRIPT"
armA_after="$(grep -cF 'if [ -z "$(fm_field "$f" plan)" ]' "$ARMSCRIPT")"
armAout="$(armrun)"
assert "mutation A landed: leg A's plan emptiness test is inverted (count 1 -> 0)" \
  '[ "$armA_before" = 1 ] && [ "$armA_after" = 0 ]'
assert "mutation A (invert plan emptiness): the unrecorded-plan fixture 220 goes GREEN" \
  '! has_finding "$armAout" aborted-run 220'
assert "mutation A (invert plan emptiness): the healthy fixture 225 MISFIRES — the other direction" \
  'has_finding "$armAout" aborted-run 225'
assert "mutation A: the stale-claim fixture 222 still fires (leg B is independent)" \
  'has_finding "$armAout" aborted-run 222'

# Mutation B — strip leg A's results emit arm: 221 goes GREEN, the plan arm survives on 220.
# The whole `if … then / emit / fi` arm goes, not just the emit line: deleting the emit alone would
# leave an empty `then` body, which is a bash SYNTAX ERROR — the script would die before any check
# ran and every fixture would go green for the wrong reason, including the surviving-arm assert.
armreseed
armB_before="$(grep -cF 'but results: is unset' "$ARMSCRIPT")"
awk '/\[ -z "\$\(fm_field "\$f" results\)" \]/{inres=1}
     inres{ if ($0 ~ /^[[:space:]]*fi$/) inres=0; next }
     {print}' "$ARMSCRIPT" > "$ARMSCRIPT.t"; mv "$ARMSCRIPT.t" "$ARMSCRIPT"
armB_after="$(grep -cF 'but results: is unset' "$ARMSCRIPT")"
armBout="$(armrun)"
assert "mutation B landed: leg A's results emit arm is gone (count 1 -> 0)" \
  '[ "$armB_before" = 1 ] && [ "$armB_after" = 0 ]'
assert "mutation B landed: the mutated copy is still valid bash (an empty then-body would be a syntax error)" \
  'bash -n "$ARMSCRIPT"'
assert "mutation B (strip results arm): the unrecorded-results fixture 221 goes GREEN" \
  '! has_finding "$armBout" aborted-run 221'
assert "mutation B: the unrecorded-plan fixture 220 still fires (arm survives)" \
  'has_finding "$armBout" aborted-run 220'

# Mutation C — widen leg B's window from 12h to 1000h: the stale-claim fixture 222 goes GREEN,
# proving the finding is produced by the THRESHOLD and not by the mere presence of claimed_at.
armreseed
armC_before="$(grep -cF 'ABORTED_RUN_STALE_SECS=$(( 12 * 3600 ))' "$ARMSCRIPT")"
awk '{ sub(/ABORTED_RUN_STALE_SECS=\$\(\( 12 \* 3600 \)\)/, "ABORTED_RUN_STALE_SECS=$(( 1000 * 3600 ))"); print }' "$ARMSCRIPT" > "$ARMSCRIPT.t"; mv "$ARMSCRIPT.t" "$ARMSCRIPT"
armC_after="$(grep -cF 'ABORTED_RUN_STALE_SECS=$(( 12 * 3600 ))' "$ARMSCRIPT")"
armCout="$(armrun)"
assert "mutation C landed: leg B's window widened to 1000h (12h literal count 1 -> 0)" \
  '[ "$armC_before" = 1 ] && [ "$armC_after" = 0 ]'
assert "mutation C (widen leg B window): the 13h stale-claim fixture 222 goes GREEN" \
  '! has_finding "$armCout" aborted-run 222'
assert "mutation C: the unrecorded-plan fixture 220 still fires (leg A is independent)" \
  'has_finding "$armCout" aborted-run 220'

# Mutation D — unanchor the plan read (fm_field -> field): the body-prose fixture 223 goes GREEN,
# because the unanchored read takes `plan: …` from the body as a set field and certifies the abort.
# This is the FALSE-NEGATIVE direction, and it is the reason every read here is anchored.
armreseed
armD_before="$(grep -cF 'fm_field "$f" plan' "$ARMSCRIPT")"
awk '{ if ($0 ~ /fm_field "\$f" plan/) sub(/fm_field/, "field"); print }' "$ARMSCRIPT" > "$ARMSCRIPT.t"; mv "$ARMSCRIPT.t" "$ARMSCRIPT"
armD_after="$(grep -cF 'fm_field "$f" plan' "$ARMSCRIPT")"
armDout="$(armrun)"
assert "mutation D landed: the plan read is unanchored (fm_field count 1 -> 0)" \
  '[ "$armD_before" = 1 ] && [ "$armD_after" = 0 ]'
assert "mutation D (unanchor the plan read): the body-prose fixture 223 goes GREEN — proves the anchoring" \
  '! has_finding "$armDout" aborted-run 223'
assert "mutation D: fixture 220, which has no body plan: line, still fires" \
  'has_finding "$armDout" aborted-run 220'

# Mutation D2 — unanchor the RESULTS read (fm_field -> field), the mirror of D. The body-prose
# fixture 224 goes GREEN (proving the anchoring is what makes it fire), while 221 — which has no
# body results: line — still fires, proving the arm itself survived the mutation.
armreseed
armD2_before="$(grep -cF 'fm_field "$f" results' "$ARMSCRIPT")"
awk '{ sub(/fm_field "\$f" results/, "field \"$f\" results"); print }' "$ARMSCRIPT" > "$ARMSCRIPT.t"
mv "$ARMSCRIPT.t" "$ARMSCRIPT"
armD2_after="$(grep -cF 'fm_field "$f" results' "$ARMSCRIPT")"
armD2out="$(armrun)"
assert "mutation D2 landed: the results read is unanchored (fm_field count 1 -> 0)" \
  '[ "$armD2_before" = 1 ] && [ "$armD2_after" = 0 ]'
assert "mutation D2 landed: the mutated copy is still valid bash" 'bash -n "$ARMSCRIPT"'
assert "mutation D2 (unanchor the results read): the body-prose fixture 224 goes GREEN — proves the anchoring" \
  '! has_finding "$armD2out" aborted-run 224'
assert "mutation D2: fixture 221, which has no body results: line, still fires" \
  'has_finding "$armD2out" aborted-run 221'

# Mutation E — drop the whole aborted-run block: every red fixture goes GREEN, and stale-in-progress
# must stay unaffected (the two checks are genuinely separate code). Fixture 226 is what makes the
# second half assertable: its 100h claim fires BOTH checks at baseline, so dropping this block must
# remove the aborted-run finding and leave the stale-in-progress one standing (change 0202).
armreseed
armE_before="$(grep -c 'aborted-run' "$ARMSCRIPT")"
awk '/# --- aborted-run:/{inar=1} inar && /# --- merge-gate-stall:/{inar=0} !inar' "$ARMSCRIPT" > "$ARMSCRIPT.t"; mv "$ARMSCRIPT.t" "$ARMSCRIPT"
armE_after="$(grep -c 'aborted-run' "$ARMSCRIPT")"
armEout="$(armrun)"
assert "mutation E landed: the aborted-run block is gone (aborted-run occurrences dropped)" \
  '[ "$armE_before" -ge 3 ] && [ "$armE_after" -lt "$armE_before" ]'
assert "mutation E landed: the mutated copy is still valid bash (the whole if-block came out balanced)" \
  'bash -n "$ARMSCRIPT"'
assert "mutation E (drop whole block): fixture 220 goes GREEN" '! has_finding "$armEout" aborted-run 220'
assert "mutation E (drop whole block): fixture 221 goes GREEN" '! has_finding "$armEout" aborted-run 221'
assert "mutation E (drop whole block): fixture 222 goes GREEN" '! has_finding "$armEout" aborted-run 222'
assert "mutation E (drop whole block): fixture 223 goes GREEN" '! has_finding "$armEout" aborted-run 223'
assert "mutation E (drop whole block): fixture 226's aborted-run finding goes GREEN" \
  '! has_finding "$armEout" aborted-run 226'
assert "mutation E: stale-in-progress on 226 SURVIVES — the two checks are separate code" \
  'has_finding "$armEout" stale-in-progress 226'
rm -rf "$armcopy"

# Mutation F — restore the C-quoting bug in branch_only_artifact (change 0202). BOTH halves must
# revert together. Reverting -z ALONE is not a usable mutation: `read -d ''` would hit EOF on
# newline-delimited input, the loop body would never run, the function would return 1 for every
# input, and both fixtures would go green for entirely the wrong reason. So the read form reverts
# with it. The here-string capture is NOT restored and does not need to be — the C-quoting is
# produced by ls-tree, not by how the output is consumed, so these two edits reproduce the defect
# exactly. Runs against ARQ2 (inherited non-ASCII plan), the only fixture that discriminates.
armreseed
armF_z_before="$(grep -cF 'ls-tree -r -z --name-only' "$ARMSCRIPT")"
armF_d_before="$(grep -cF "read -r -d ''" "$ARMSCRIPT")"
sed -e 's/ls-tree -r -z --name-only/ls-tree -r --name-only/' \
    -e "s/while IFS= read -r -d '' boa_p; do/while IFS= read -r boa_p; do/" \
    "$ARMSCRIPT" > "$ARMSCRIPT.t"; mv "$ARMSCRIPT.t" "$ARMSCRIPT"
armF_z_after="$(grep -cF 'ls-tree -r -z --name-only' "$ARMSCRIPT")"
armF_d_after="$(grep -cF "read -r -d ''" "$ARMSCRIPT")"
assert "mutation F landed: -z is gone from the ls-tree listing (count 1 -> 0)" \
  '[ "$armF_z_before" = 1 ] && [ "$armF_z_after" = 0 ]'
assert "mutation F landed: the NUL read form is gone (count 1 -> 0)" \
  '[ "$armF_d_before" = 1 ] && [ "$armF_d_after" = 0 ]'
assert "mutation F landed: the mutated copy is still valid bash" 'bash -n "$ARMSCRIPT"'
armFout="$(armrun_at "$ARQ2")"
assert "mutation F (restore C-quoting): the INHERITED non-ASCII fixture 231 MISFIRES — the false positive" \
  'has_finding "$armFout" aborted-run 231'
armFsan="$(armrun_at "$ARQ1")"
assert "mutation F: the branch-only fixture 230 still fires (the arm itself survives the mutation)" \
  'has_finding "$armFsan" aborted-run 230'
rm -rf "$armcopy"

# Mutation O — rewrite branch_only_artifact's consumption into the FORBIDDEN capture shape (change
# 0200). Letters A-N are taken; O is the next free one. This arm is the enforcement the comment
# above the function ("The NUL listing CANNOT be captured into a variable first") never had.
#
# Everything that LOOKS like a correctness signal survives the rewrite, which is the entire trap:
# -z stays, `read -r -d ''` stays, and `bash -n` still passes. But `$(…)` strips NUL bytes, so the
# here-string carries one NUL-free blob, `read -d ''` hits EOF on the first iteration, the loop body
# never runs, and the function returns 1 for EVERY input. Leg A would go permanently, silently
# false-negative with a fully green suite.
#
# The GREEN assert is not vacuous: fixture 230's baseline firing is pinned in the ARQ1 block above.
# The two "still present" asserts matter as much as the landed ones — they prove this arm reproduces
# the CAPTURE defect specifically and has not accidentally degenerated into mutation F.
armreseed
# THE CAPTURE LINE IS DERIVED, NEVER RESTATED. This arm used to hand-write a second copy of
# board-checks.sh's real listing command into its awk program. Two costs, both paid here (change
# 0200): a duplicate drifts the moment the real command is edited — the mutant would then capture
# a stale command while every assert stayed green — and, because the duplicate had to SPELL
# `ls-tree … -- <dir>`, it read to tests/test_skip_allowlist_invisibility.sh's limb 2 as a genuine
# unbounded tree walk in this file. That extractor is textual and cannot tell a described walk
# from a performed one, and the right answer to a guard that cannot distinguish them is to stop
# writing the thing that looks like one, not to declare an exemption for prose.
#
# So the transform LIFTS the command out of the `done < <(…)` feed that is already in the script
# and re-emits it in capture position: pass 1 (NR == FNR) reads the feed, pass 2 rewrites. The
# awk anchors on `^  done < <(` — the indent matters, board-checks.sh has a second, column-0
# process-substituted feed for an unrelated `log` walk — and armO_ps_before pins that exactly one
# line matches, which is the precondition the derivation rests on.
armO_ps_before="$(grep -c '^  done < <(' "$ARMSCRIPT" || true)"
armO_srcline="$(grep '^  done < <(' "$ARMSCRIPT" || true)"
# The prefix goes through a VARIABLE, and the trailing paren is quoted: an unbalanced `<(` or `)`
# written inline inside a ${…} operator is scanned by bash before it is a pattern, and the file
# stops parsing at the unmatched one.
armO_feed='  done < <('
armO_cmd="${armO_srcline#"$armO_feed"}"; armO_cmd="${armO_cmd%")"}"
awk '
  NR == FNR {
    if ($0 ~ /^  done < <\(/) {
      boa_cmd = $0; sub(/^  done < <\(/, "", boa_cmd); sub(/\)[[:space:]]*$/, "", boa_cmd)
    }
    next
  }
  $0 ~ /^  while IFS= read -r -d .. boa_p; do$/ {
    print "  boa_list=\"$(" boa_cmd ")\""
    print; next
  }
  $0 ~ /^  done < <\(/ { print "  done <<<\"$boa_list\""; next }
  { print }
' "$ARMSCRIPT" "$ARMSCRIPT" > "$ARMSCRIPT.t"; mv "$ARMSCRIPT.t" "$ARMSCRIPT"
armO_ps_after="$(grep -c '^  done < <(' "$ARMSCRIPT" || true)"
armO_hs="$(grep -cF 'done <<<"$boa_list"' "$ARMSCRIPT")"
# The capture line must be the feed's command VERBATIM, in capture position — not merely "a
# capture of something". $armO_want is built from the pre-mutation line read above, so a drift in
# board-checks.sh moves both sides together and this assert keeps meaning the same thing.
armO_capline="$(grep -F 'boa_list="$(' "$ARMSCRIPT" || true)"
armO_want="  boa_list=\"\$($armO_cmd)\""
# 1, not 2: the rewrite REPLACES the `done < <(…)` line, so the only surviving occurrence of the
# `-z --name-only` text is the capture line the awk emits. Measured on a hand-built mutant, not
# reasoned about. The exact count is load-bearing — under mutation F this same grep reads 0, which
# is what makes this assert discriminate the capture defect from the -z revert.
armO_z="$(grep -cF 'ls-tree -r -z --name-only' "$ARMSCRIPT")"
armO_d="$(grep -cF "read -r -d ''" "$ARMSCRIPT")"
assert "mutation O landed: the process-substituted listing is gone (count 1 -> 0)" \
  '[ "$armO_ps_before" = 1 ] && [ "$armO_ps_after" = 0 ]'
assert "mutation O landed: the forbidden here-string consumption is in place" '[ "$armO_hs" = 1 ]'
assert "mutation O landed: the listing is captured into a variable first, VERBATIM from the feed it replaced" \
  '[ -n "$armO_cmd" ] && [ "$armO_capline" = "$armO_want" ]'
assert "mutation O landed: the mutated copy is still valid bash — the broken shape is SYNTACTICALLY FINE" \
  'bash -n "$ARMSCRIPT"'
assert "mutation O is the CAPTURE defect, not mutation F: -z survives the rewrite" \
  '[ "$armO_z" = 1 ]'
assert "mutation O is the CAPTURE defect, not mutation F: the NUL read form survives the rewrite" \
  '[ "$armO_d" = 1 ]'
armOout="$(armrun_at "$ARQ1")"
assert "mutation O (capture the -z listing): the branch-only fixture 230 goes GREEN — NULs stripped, the loop never runs" \
  '! has_finding "$armOout" aborted-run 230'
# NON-VACUITY for the GREEN assert above — the guard mutation 4 carries, adapted. For the ARQ1
# fixture under mutation O the CORRECT $armOout is entirely EMPTY, so `! has_finding` passes
# identically whether the capture defect was reproduced or the mutant never ran at all; armrun_at
# sends stderr to /dev/null, which is exactly how a copy that aborts at runtime fakes a green
# assert. So re-run capturing stderr INSTEAD of stdout. It CANNOT demand silence the way mutation
# 4's does: capturing a NUL-bearing listing is the whole point of this mutant, and bash announces
# it ("ignored null byte in input") on every such substitution — a silence assert here would be
# red on the correct mutant. What it pins instead is a normal exit AND the absence of any
# syntax/abort diagnostic, which is what a dead copy emits and a working one never does.
armO_err="$(NOW=$NOW_EPOCH bash "$ARMSCRIPT" --changes-dir "$ARQ1/docs/changes" \
  --metadata-branch docket --integration-branch main 2>&1 >/dev/null)"
armO_rc=$?
assert "mutation O: the mutated copy still RUNS — normal exit, no abort diagnostic (armrun_at's 2>/dev/null hides both)" \
  '[ "$armO_rc" = 0 ] && ! grep -qE "syntax error|unexpected token|command not found|unbound variable|No such file or directory" <<<"$armO_err"'
rm -rf "$armcopy"

# ---------------- leg C mutations (change 0211) ----------------
# Mutation G — neutralize leg C's idle floor (the > comparison becomes a tautology): the live-run
# fixture 241 starts firing. NOTE the blast radius: without the floor, every ARM branch fixture
# whose branch is ahead with pr: absent becomes a leg-C candidate, which is why the baseline block
# above pins those fixtures explicitly rather than trusting the date delta.
armreseed
armG_before="$(grep -cF '"$(( NOW - ar_tip ))" -gt "$ABORTED_RUN_IDLE_SECS"' "$ARMSCRIPT")"
sed 's/"$(( NOW - ar_tip ))" -gt "$ABORTED_RUN_IDLE_SECS"/-n "$ar_tip"/' "$ARMSCRIPT" > "$ARMSCRIPT.t"
mv "$ARMSCRIPT.t" "$ARMSCRIPT"
armG_after="$(grep -cF '"$(( NOW - ar_tip ))" -gt "$ABORTED_RUN_IDLE_SECS"' "$ARMSCRIPT")"
assert "mutation G landed: the idle-floor comparison is gone (count 1 -> 0)" \
  '[ "$armG_before" = 1 ] && [ "$armG_after" = 0 ]'
assert "mutation G landed: the mutated copy is still valid bash" 'bash -n "$ARMSCRIPT"'
armGout="$(armrun)"
assert "mutation G (drop the idle floor): the live-run fixture 241 starts firing — the floor is real" \
  'has_finding "$armGout" aborted-run 241'
assert "mutation G: the quiet fixture 240 still fires (the leg itself survives)" \
  'has_finding "$armGout" aborted-run 240'
rm -rf "$armcopy"

# Mutation H — make the ahead-of-bases probe unconditionally true: the nothing-built fixture 244
# starts firing, i.e. leg C would claim "built but not delivered" about a branch with no build.
armreseed
armH_before="$(grep -cF 'rev-list -n 1 "$ar_ref" --not' "$ARMSCRIPT")"
sed 's|\[ -n "$("$GIT" -C "$CHANGES_DIR" rev-list -n 1 "$ar_ref" --not "${ar_bases\[@\]}" 2>/dev/null)" \]|true|' \
  "$ARMSCRIPT" > "$ARMSCRIPT.t"; mv "$ARMSCRIPT.t" "$ARMSCRIPT"
armH_after="$(grep -cF 'rev-list -n 1 "$ar_ref" --not' "$ARMSCRIPT")"
assert "mutation H landed: the ahead-of-bases probe is gone (count 1 -> 0)" \
  '[ "$armH_before" = 1 ] && [ "$armH_after" = 0 ]'
assert "mutation H landed: the mutated copy is still valid bash" 'bash -n "$ARMSCRIPT"'
armHout="$(armrun)"
assert "mutation H (drop the ahead test): the nothing-built fixture 244 starts firing" \
  'has_finding "$armHout" aborted-run 244'
assert "mutation H: the genuinely-built fixture 240 still fires" 'has_finding "$armHout" aborted-run 240'
rm -rf "$armcopy"

# Mutation I — drop leg C's pr:-empty gate: the delivered fixture 243 starts firing.
armreseed
armI_before="$(grep -cF 'if [ -z "$ar_pr" ] && [ -n "$ar_ref" ]; then' "$ARMSCRIPT")"
sed 's|if \[ -z "$ar_pr" \] && \[ -n "$ar_ref" \]; then|if [ -n "$ar_ref" ]; then|' \
  "$ARMSCRIPT" > "$ARMSCRIPT.t"; mv "$ARMSCRIPT.t" "$ARMSCRIPT"
armI_after="$(grep -cF 'if [ -z "$ar_pr" ] && [ -n "$ar_ref" ]; then' "$ARMSCRIPT")"
assert "mutation I landed: leg C's pr:-empty gate is gone (count 1 -> 0)" \
  '[ "$armI_before" = 1 ] && [ "$armI_after" = 0 ]'
assert "mutation I landed: the mutated copy is still valid bash" 'bash -n "$ARMSCRIPT"'
armIout="$(armrun)"
# Keyed on leg C's exclusive clause, not on has_finding: leg D already speaks on 243 at baseline
# (change 0219), so a bare aborted-run test here would pass whether or not the mutation landed.
armI_243="$(grep -E "$(printf "^aborted-run\t243\t")" <<<"$armIout")"
assert "mutation I (drop the pr: gate): the delivered fixture 243 starts firing" \
  'grep -qF -- "pr: is unset" <<<"$armI_243"'
rm -rf "$armcopy"

# Mutation J — invert the message-selecting remote-ref probe: the two firing fixtures SWAP
# messages. This is the mutation that proves the branch is a real discriminator and not a coin
# flip that happens to be right for one of them.
armreseed
armJ_before="$(grep -cF 'show-ref --verify --quiet "refs/remotes/origin/$ar_branch"' "$ARMSCRIPT")"
sed 's|if "$GIT" -C "$CHANGES_DIR" show-ref --verify --quiet "refs/remotes/origin/$ar_branch"; then|if ! "$GIT" -C "$CHANGES_DIR" show-ref --verify --quiet "refs/remotes/origin/$ar_branch"; then|' \
  "$ARMSCRIPT" > "$ARMSCRIPT.t"; mv "$ARMSCRIPT.t" "$ARMSCRIPT"
armJ_after="$(grep -cF 'if ! "$GIT" -C "$CHANGES_DIR" show-ref --verify --quiet "refs/remotes/origin/$ar_branch"; then' "$ARMSCRIPT")"
assert "mutation J landed: the remote-ref probe is negated (count 0 -> 1)" \
  '[ "$armJ_before" = 1 ] && [ "$armJ_after" = 1 ]'
assert "mutation J landed: the mutated copy is still valid bash" 'bash -n "$ARMSCRIPT"'
armJout="$(armrun)"
armJ240="$(grep -E "^aborted-run"$'\t'"240"$'\t' <<<"$armJout")"
armJ242="$(grep -E "^aborted-run"$'\t'"242"$'\t' <<<"$armJout")"
assert "mutation J (swap the probe): the UNPUSHED fixture 240 now gets the pushed message" \
  'grep -qF "is pushed but pr: is unset" <<<"$armJ240"'
assert "mutation J (swap the probe): the PUSHED fixture 242 now gets the never-pushed message" \
  'grep -qF "branch never pushed and pr: is unset" <<<"$armJ242"'
rm -rf "$armcopy"

# Mutation K — drop the remote-tracking base from ar_bases (the single-base predicate an earlier
# draft used): fixture 245, cut from origin/main while local main lags, starts firing. This is the
# false positive the both-bases design exists to prevent — and note the idle floor does NOT catch
# it, because the inherited commits are genuinely old.
armreseed
armK_before="$(grep -cF 'for ar_b in "refs/heads/$INTEGRATION_BRANCH" "refs/remotes/origin/$INTEGRATION_BRANCH"; do' "$ARMSCRIPT")"
sed 's|for ar_b in "refs/heads/$INTEGRATION_BRANCH" "refs/remotes/origin/$INTEGRATION_BRANCH"; do|for ar_b in "refs/heads/$INTEGRATION_BRANCH"; do|' \
  "$ARMSCRIPT" > "$ARMSCRIPT.t"; mv "$ARMSCRIPT.t" "$ARMSCRIPT"
armK_after="$(grep -cF 'for ar_b in "refs/heads/$INTEGRATION_BRANCH" "refs/remotes/origin/$INTEGRATION_BRANCH"; do' "$ARMSCRIPT")"
assert "mutation K landed: the remote-tracking base is gone from ar_bases (count 1 -> 0)" \
  '[ "$armK_before" = 1 ] && [ "$armK_after" = 0 ]'
assert "mutation K landed: the mutated copy is still valid bash" 'bash -n "$ARMSCRIPT"'
armKout="$(armrun)"
assert "mutation K (local base only): the stale-local-main fixture 245 starts firing" \
  'has_finding "$armKout" aborted-run 245'
# 245 is silent at baseline and speaks ONLY here, and the message it speaks is leg C's — not leg A
# riding the same stale local base off the advancing commit (which is why that commit touches a
# neutral path, not docs/results). Without this, the assert above would pass on a leg-A finding.
armK245="$(grep -E "^aborted-run"$'\t'"245"$'\t' <<<"$armKout")"
assert "mutation K: 245's new finding is LEG C's, not leg A riding the same stale base" \
  'grep -qF "ahead of main, branch never pushed and pr: is unset" <<<"$armK245"'
rm -rf "$armcopy"

# Mutation L — unanchor leg C's pr: read (fm_field -> field): the body-prose fixture 246 goes
# GREEN, because the unanchored read falls through the closing --- and returns the prose line as
# if it were a recorded PR. ADR-0057, the same property 223 and 224 pin for plan: and results:.
# A FALSE NEGATIVE is the dangerous direction here: it makes the check certify the exact abort it
# exists to catch.
armreseed
# Since change 0219 the pr: read is HOISTED and shared with leg D, so the mutation target is that
# one hoisted line — and its damage now runs in both directions at once: leg C goes silent on 246
# (measured here) while leg D MISFIRES on a change with no PR at all (measured by mutation N).
armL_before="$(grep -cF 'ar_pr="$(fm_field "$f" pr)"' "$ARMSCRIPT")"
sed 's|ar_pr="$(fm_field "$f" pr)"|ar_pr="$(field "$f" pr)"|' "$ARMSCRIPT" > "$ARMSCRIPT.t"
mv "$ARMSCRIPT.t" "$ARMSCRIPT"
armL_after="$(grep -cF 'ar_pr="$(fm_field "$f" pr)"' "$ARMSCRIPT")"
assert "mutation L landed: leg C's pr: read is unanchored (count 1 -> 0)" \
  '[ "$armL_before" = 1 ] && [ "$armL_after" = 0 ]'
assert "mutation L landed: the mutated copy is still valid bash" 'bash -n "$ARMSCRIPT"'
armLout="$(armrun)"
armL_246="$(grep -E "$(printf "^aborted-run\t246\t")" <<<"$armLout")"
assert "mutation L (unanchor the pr: read): the body-prose fixture 246 goes GREEN — proves the anchoring" \
  '! grep -qF -- "pr: is unset" <<<"$armL_246"'
assert "mutation L: leg D misfires on 246 with the body pr: value — the aborted-run line exists, so the silence above is not vacuous" \
  'grep -qF -- "pr: records" <<<"$armL_246"'
assert "mutation L: fixture 240, which has no body pr: line, still fires" \
  'has_finding "$armLout" aborted-run 240'
rm -rf "$armcopy"

# Mutation M — delete leg C's empty-ar_bases count gate, leaving `"${ar_bases[@]}"` expanded with
# nothing to expand. Run against AR24, whose integration branch resolves as NEITHER base.
#
# The gate's removal has TWO different observable consequences and WHICH one you get is decided by
# the bash running the script, so the arm asserted here is chosen by probing that very bash:
#   - bash >= 4.4 expands an empty array under `set -u` without complaint. `rev-list -n 1 "$ar_ref"
#     --not` then excludes NO bases at all, lists the branch's whole history, and leg C FIRES with
#     an EMPTY base label — literally "ahead of , branch never pushed", the "ahead of nothing"
#     reading board-checks.md forbids.
#   - bash before 4.4 raises `ar_bases[@]: unbound variable`. Measured on /bin/bash 3.2, not
#     assumed: that error kills only the COMMAND-SUBSTITUTION SUBSHELL. The substitution comes out
#     empty, leg C declines, the walk continues to 248, and the script still exits 0 — so the
#     damage is a diagnostic leaking onto stderr for every no-base change, which the baseline
#     assert on "$ar24err" pins as absent. There is NO stdout-observable difference from baseline
#     there, which is exactly why this runner keeps stderr.
#     The window where this arm actually RUNS is bash 4.0-4.3: board-checks.sh needs `mapfile` and
#     `declare -g`, so bash 3.2 cannot execute it at all. The arm is kept because without it this
#     mutation would go spuriously RED on a 4.0-4.3 host rather than measuring anything.
# Both arms therefore assert a real defect; neither can assert a truncated walk or a non-zero exit,
# because the script does not die in either world (pinned below, so a future bash that DOES die
# reddens here rather than silently changing what this mutation measures).
armreseed
armM_before="$(grep -cF 'if [ "${#ar_bases[@]}" -gt 0 ] && \' "$ARMSCRIPT")"
sed 's|if \[ "${#ar_bases\[@\]}" -gt 0 \] && \\$|if \\|' "$ARMSCRIPT" > "$ARMSCRIPT.t"
mv "$ARMSCRIPT.t" "$ARMSCRIPT"
armM_after="$(grep -cF 'if [ "${#ar_bases[@]}" -gt 0 ] && \' "$ARMSCRIPT")"
assert "mutation M landed: leg C's empty-ar_bases count gate is gone (count 1 -> 0)" \
  '[ "$armM_before" = 1 ] && [ "$armM_after" = 0 ]'
assert "mutation M landed: the mutated copy is still valid bash" 'bash -n "$ARMSCRIPT"'
# Probe the SAME interpreter armrun_ib invokes (`bash` off PATH), not this test's own.
armM_lax=0
bash -uc 'armprobe=(); : "${armprobe[@]}"' 2>/dev/null && armM_lax=1
armrun_ib "$AR24" nosuchbranch
if [ "$armM_lax" = 1 ]; then
  assert "mutation M (drop the count gate; this bash expands an empty array): leg C MISFIRES on 247" \
    'has_finding "$armib_out" aborted-run 247'
  assert "mutation M: the misfire IS the forbidden reading — an empty base label, 'ahead of nothing'" \
    'grep -qF "ahead of , branch never pushed and pr: is unset" <<<"$armib_out"'
else
  assert "mutation M (drop the count gate; this bash errors on an empty array): stderr names ar_bases" \
    'grep -qF "ar_bases[@]: unbound variable" <<<"$armib_err"'
  assert "mutation M: on this bash the diagnostic is the whole damage — stdout still declines 247" \
    '! has_finding "$armib_out" aborted-run 247'
fi
assert "mutation M: the run does NOT abort — exit 0, so the evidence above is the only evidence there is" \
  '[ "$armib_status" = 0 ]'
assert "mutation M: the walk is NOT truncated — 248's later finding survives in both worlds" \
  'has_finding "$armib_out" broken-spec 248'
rm -rf "$armcopy"

# Mutation N (change 0219) — unanchor leg D's pr: read. Fixture 263 omits pr: from frontmatter while
# its body opens a `pr:` line, so an unanchored read returns the prose and leg D MISFIRES on a change
# that has no PR at all. This is the ADR-0057 shape in its false-POSITIVE direction; the fixture and
# the mutation only discriminate as a pair.
mreseed
mn_before="$(grep -cF -- 'ar_pr="$(fm_field "$f" pr)"' "$MUTSCRIPT")"
perl -pi -e 's/ar_pr="\$\(fm_field "\$f" pr\)"/ar_pr="\$(field "\$f" pr)"/' "$MUTSCRIPT"
mn_after="$(grep -cF -- 'ar_pr="$(fm_field "$f" pr)"' "$MUTSCRIPT")"
mnout="$(NOW=$NOW_EPOCH bash "$MUTSCRIPT" --changes-dir "$AR_D4/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "mutation N landed: leg D's pr: read is unanchored (fm_field count 1 -> 0)" \
  '[ "$mn_before" = 1 ] && [ "$mn_after" = 0 ]'
assert "mutation N landed: the mutated copy is still valid bash" 'bash -n "$MUTSCRIPT"'
assert "mutation N (unanchor leg D's pr: read): body prose 263 MISFIRES — proves the anchoring" \
  'has_finding "$mnout" aborted-run 263'
assert "mutation N: the misfire echoes the BODY value, not a frontmatter one" \
  'grep -qF -- "pr: records 999" <<<"$mnout"'


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
n71="$(grep <<<"$eout" -c .)"
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

# (d) CASE B — an archive/ file with NO id: field at all, at a terminal status. The archive summary
#     count keys on the raw status, so this file IS counted there, while no row IDENTIFYING it ever
#     renders. Nothing enumerated explains it: malformed-id needs a non-empty raw id value, and
#     `done` is a legal status so field-domain passes it. Only the computed invariant sees it.
#     (Before change 0115 this block asserted the OPPOSITE, under the premise that archive/ was
#     exempt from the invariant. That premise is what 0115 deletes.)
read -r I _ < <(new_repo)
printf -- '---\nslug: archnoid\ntitle: Arch no id\nstatus: done\npriority: medium\ndepends_on: []\n---\n' \
  > "$I/docs/changes/archive/2026-06-16-0073-archnoid.md"
iout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$I/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "board-row-dropped fires for an archive/ file with no id: field (0073)" \
  'has_finding "$iout" board-row-dropped 0073'
assert "malformed-id does NOT fire for it (there is no raw id value to report)" \
  '! has_finding "$iout" malformed-id 0073'
assert "field-domain does NOT explain it (done is a legal status)" \
  '! has_finding "$iout" field-domain 0073'

# (e) a wholly clean tree stays silent — the backstop must not fire on healthy repos.
read -r J _ < <(new_repo)
printf -- '---\nid: 74\nslug: fine\ntitle: Fine\nstatus: proposed\npriority: medium\ndepends_on: []\n---\n' \
  > "$J/docs/changes/active/0074-fine.md"
jout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$J/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "clean active tree emits no board-row-dropped finding" '! grep <<<"$jout" -q "^board-row-dropped"'

# (f) THE COMPUTED-PREDICATE CASE: an active/ file carrying a TERMINAL status (`done`). Every
# ENUMERATED check is correctly silent — `done` is in DOCKET_STATUSES so field-domain passes it, and
# the id is a well-formed integer so malformed-id passes it — yet render-board.sh counts the file in
# `total` (`total=${#AFILES[@]}`) and calls print_section only for the five ACTIVE statuses, so the
# row is
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

# ============ archive-side board-row-dropped (change 0115) ============
# The invariant is SINGULAR — one `total`, one set of tables — so it is widened, not split: no new
# check-id. renders_row now takes the directory and reads the status set the renderer actually
# iterates for it (DOCKET_STATUSES_ACTIVE vs DOCKET_STATUSES_TERMINAL, via the shared
# docket_status_is_* helpers), above a hoisted "id must be usable" clause.

# (T1) CASE A, block open: a non-terminal status in archive/, beside a healthy done sibling. The
# archive block opens (the sibling is terminal) so the misfiled row DOES print — but under a
# <summary> count that excludes it, because that count reads terminal statuses only. Count and
# tables disagree, which is the whole invariant.
read -r AA _ < <(new_repo)
printf -- '---\nid: 80\nslug: misfiled\ntitle: Misfiled\nstatus: implemented\npriority: medium\ndepends_on: []\n---\n' \
  > "$AA/docs/changes/archive/2026-06-16-0080-misfiled.md"
printf -- '---\nid: 81\nslug: good\ntitle: Good\nstatus: done\npriority: medium\ndepends_on: []\n---\n' \
  > "$AA/docs/changes/archive/2026-06-16-0081-good.md"
aaout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AA/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "board-row-dropped fires for a NON-TERMINAL status in archive/ (80)" \
  'has_finding "$aaout" board-row-dropped 80'
assert "the healthy done sibling in archive/ draws no finding (81)" \
  '! has_finding "$aaout" board-row-dropped 81'
assert "the archive misfile is NOT explained by field-domain (implemented is a legal status)" \
  '! has_finding "$aaout" field-domain 80'

# (T2) CASE A, block closed: the same misfiled file with NO terminal sibling. The entire archive
# block is gated on the terminal counts, so it never opens and the row appears NOWHERE at all.
# Distinct from T1 in the rendered outcome; identical in the accounting failure.
read -r AB _ < <(new_repo)
printf -- '---\nid: 82\nslug: alone\ntitle: Alone\nstatus: implemented\npriority: medium\ndepends_on: []\n---\n' \
  > "$AB/docs/changes/archive/2026-06-16-0082-alone.md"
about="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AB/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "board-row-dropped fires when the archive block never opens at all (82)" \
  'has_finding "$about" board-row-dropped 82'

# (T3) FALSE-POSITIVE GUARD for the ARCHIVE_RECENT window. 16 well-formed done files: the renderer
# shows 15 verbatim and REDIRECTS the 16th into the per-month "Older done (collapsed)" digest.
# Collapse is a redirect, not a discard — the file is still in the summary count and still
# represented in the digest — so the predicate, which is written against ACCOUNTING rather than
# against verbatim row emission, must be blind to it. A predicate "tightened" toward row emission
# would fire on every done file past the 16th; this assert is what stops that from creeping back in.
# Asserted on the SPECIFIC id the window pushes out (not "no findings at all", which would pass
# vacuously): sort is date-desc, so the oldest date (2026-06-01, id 101) is the one that collapses.
# Verified against the running renderer: 15 verbatim rows + 1 collapsed.
read -r AC _ < <(new_repo)
for i in $(seq 1 16); do
  acd="$(printf '2026-06-%02d' "$i")"; acid=$(( 100 + i ))
  printf -- '---\nid: %s\nslug: c%s\ntitle: C%s\nstatus: done\npriority: medium\ndepends_on: []\n---\n' \
    "$acid" "$acid" "$acid" > "$AC/docs/changes/archive/$acd-0$acid-c$acid.md"
done
acout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AC/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "the collapsed done file draws NO board-row-dropped finding (101 — ARCHIVE_RECENT redirects, never discards)" \
  '! has_finding "$acout" board-row-dropped 101'
assert "nor does the newest verbatim done row (116)" \
  '! has_finding "$acout" board-row-dropped 116'

# (T4) the other member of DOCKET_STATUSES_TERMINAL: a killed archive file is healthy and silent.
# Its value is covering `killed`, not anything about collapse (killed never collapses).
read -r AD _ < <(new_repo)
printf -- '---\nid: 84\nslug: abandoned\ntitle: Abandoned\nstatus: killed\npriority: medium\ndepends_on: []\n---\n' \
  > "$AD/docs/changes/archive/2026-06-16-0084-abandoned.md"
adout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AD/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "a killed archive file draws NO board-row-dropped finding (84)" \
  '! has_finding "$adout" board-row-dropped 84'

# (T6) suppression by malformed-id on the archive side. A non-integer id is a genuine archive drop
# cause, so the enumerated finding accounts for it and the backstop stays quiet.
read -r AE _ < <(new_repo)
printf -- '---\nid: nope\nslug: badarch\ntitle: Bad arch id\nstatus: done\npriority: medium\ndepends_on: []\n---\n' \
  > "$AE/docs/changes/archive/2026-06-16-0085-badarch.md"
aeout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AE/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "malformed-id fires for a non-integer id in archive/ (0085)" \
  'has_finding "$aeout" malformed-id 0085'
assert "board-row-dropped is suppressed when malformed-id explains the archive drop (0085)" \
  '! has_finding "$aeout" board-row-dropped 0085'

# (T7) suppression by the field-domain `status` arm on the archive side. A status outside the
# seven-name vocabulary is outside DOCKET_STATUSES_TERMINAL too, so it explains the archive drop.
# EXACTLY ONE finding, for the same reason case (b) asserts it on the active side: DROPPED is
# written by the computed predicate and EXPLAINED by the field-domain status arm, at independent
# sites — so deleting that arm's EXPLAINED marker reddens this with a second finding.
read -r AF _ < <(new_repo)
printf -- '---\nid: 86\nslug: weird\ntitle: Weird status\nstatus: finished\npriority: medium\ndepends_on: []\n---\n' \
  > "$AF/docs/changes/archive/2026-06-16-0086-weird.md"
afout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AF/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
n86="$(printf '%s' "$afout" | /usr/bin/grep -c .)"
assert "an out-of-vocabulary archive status yields exactly ONE finding (86)" '[ "$n86" = 1 ]'
assert "and that one finding is field-domain, not board-row-dropped (86)" \
  'has_finding "$afout" field-domain 86'
assert "board-row-dropped is suppressed when field-domain explains the archive drop (86)" \
  '! has_finding "$afout" board-row-dropped 86'

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
  'grep <<<"$oout" -E "$(printf "^merged-orphan\t50\t")" | grep >/dev/null -F "docket(0050)"'

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
  'grep <<<"$fbout" -E "$(printf "^stale-finalize-blocked\t40\t")" | grep >/dev/null -F "100h"'
assert "stale-finalize-blocked message names re-run finalize with the id (id 40)" \
  'grep <<<"$fbout" -E "$(printf "^stale-finalize-blocked\t40\t")" | grep >/dev/null -F "finalize 40"'
assert "stale-finalize-blocked silent for a recent marker (id 41)" \
  '! has_finding "$fbout" stale-finalize-blocked 41'
assert "stale-finalize-blocked silent for an implemented change without the marker (id 42)" \
  '! has_finding "$fbout" stale-finalize-blocked 42'
assert "stale-finalize-blocked silent for a non-implemented change carrying a stray marker (id 43, status gate)" \
  '! has_finding "$fbout" stale-finalize-blocked 43'

# ============================ docket-status wiring sentinels (SKILL is code on main) ============================
# The SKILL no longer names the check-ids at all (change 0145 — see the section-scoped guard at
# the end of this file, which now forbids it). What remains here are the surviving model-driven
# signals, each anchored to a phrase it owns: the blocked_by re-examination (judgment) and the
# github mirror-reachability visibility flag. Change 0024 retired the inline board/source-drift
# check (deterministic render + the unconditional Board-pass re-render make it vacuous); its
# removed tripwire lives in tests/test_board_refresh_on_transition.sh.
assert "docket-status keeps blocked_by re-examination model-driven" \
  'grep -qiF "blocked_by:" "$SKILL"'
assert "docket-status keeps the github mirror-reachability visibility flag (survives 0024 inline-drift retirement)" \
  'grep -qiF "mirror reachability" "$SKILL" || grep -qiF "mirror-reachability" "$SKILL"'
assert "docket-status keeps the do-not-auto-fix stance" \
  'grep -qiF "do not auto-fix" "$SKILL"'

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

# ============================ adr-unpublished (missing arm) ============================
# The due rule (spec §4.2) decides whether an ADR absent from the integration branch is a gap.
# Every negative row is asserted, not just the positive: a check that fires on everything absent
# is the naive formulation the ADR-0023/ADR-0060 data points exist to rule out.
read -r AU AU_ORIGIN <<<"$(new_repo)"
mkdir -p "$AU/docs/adrs"

# --- change files the due rule resolves `change:` against (on the docket checkout) ---
cat > "$AU/docs/changes/archive/2026-07-01-0060-done-change.md" <<'EOF'
---
id: 60
slug: done-change
title: A change that reached done
status: done
priority: medium
depends_on: []
trivial: true
---
EOF
cat > "$AU/docs/changes/archive/2026-07-02-0061-killed-change.md" <<'EOF'
---
id: 61
slug: killed-change
title: A change that was killed
status: killed
priority: medium
depends_on: []
trivial: true
---
EOF
cat > "$AU/docs/changes/active/0062-implemented-change.md" <<'EOF'
---
id: 62
slug: implemented-change
title: A change still at the merge gate
status: implemented
priority: medium
depends_on: []
trivial: true
---
EOF

# --- ADRs. Only adr_pub is committed to BOTH branches; the rest live on docket only. ---
# 10: standalone (no change:), Accepted        -> DUE now, absent      => finding
# 11: change: 60 (done)                        -> DUE, absent          => finding
# 12: change: 61 (killed)                      -> DUE, absent          => finding
# 13: change: 62 (implemented)                 -> NOT due (ADR-0060 shape) => silent
# 14: standalone, status: Superseded by ADR-10 -> NOT Accepted, absent  => silent
# 15: change: 99 (no such change file)         -> unresolvable          => silent
# 16 (adr_pub): standalone, Accepted, PUBLISHED (byte-identical on main too) -> the i_blob-present
#     arm must fire (status-blind, due forever) but this check's `missing` arm must stay silent —
#     the whole point of the "present on integration branch => continue" clause under test.
write_adr(){ # write_adr NUM STATUS CHANGE
  local num="$1" st="$2" ch="$3"
  { printf -- '---\nid: %s\nslug: adr-%s\ntitle: ADR %s\nstatus: %s\ndate: 2026-07-01\n' \
      "$((10#$num))" "$((10#$num))" "$((10#$num))" "$st"
    printf 'supersedes: []\nreverses: []\nrelates_to: []\n'
    [ -n "$ch" ] && printf 'change: %s\n' "$ch"
    printf -- '---\n\n## Context\n\nc\n\n## Decision\n\nd\n\n## Consequences\n\nq\n'
  } > "$AU/docs/adrs/${num}-adr-$((10#$num)).md"
}
write_adr 0010 Accepted ""
write_adr 0011 Accepted 60
write_adr 0012 Accepted 61
write_adr 0013 Accepted 62
write_adr 0014 "Superseded by ADR-10" ""
write_adr 0015 Accepted 99
write_adr 0016 Accepted ""
echo "# index" > "$AU/docs/adrs/README.md"
git -C "$AU" add -A; git_quiet -C "$AU" commit -m "adr fixtures"; git_quiet -C "$AU" push origin docket

# --- publish ADR-0016 onto main with IDENTICAL bytes (the way terminal-publish.sh does): checkout
# main, pull the exact blob from docket, commit, push, return to docket (new_repo's parked branch).
git -C "$AU" checkout main >/dev/null 2>&1
mkdir -p "$AU/docs/adrs"
git -C "$AU" checkout docket -- docs/adrs/0016-adr-16.md
git -C "$AU" add docs/adrs/0016-adr-16.md
git_quiet -C "$AU" commit -m "publish adr 16 onto main"
git_quiet -C "$AU" push origin main
git -C "$AU" checkout docket >/dev/null 2>&1

auout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AU/docs/changes" \
  --adrs-dir "$AU/docs/adrs" --terminal-publish \
  --metadata-branch docket --integration-branch main 2>/dev/null)"

assert "adr-unpublished fires for a STANDALONE Accepted ADR absent from the integration branch (ADR-0010)" \
  'has_finding "$auout" adr-unpublished "?"'
assert "adr-unpublished fires for a change-tied ADR whose change is DONE (ADR-0011, cid 60)" \
  'has_finding "$auout" adr-unpublished 60'
assert "adr-unpublished fires for a change-tied ADR whose change is KILLED (ADR-0012, cid 61)" \
  'has_finding "$auout" adr-unpublished 61'
assert "adr-unpublished SILENT for a change-tied ADR whose change is IMPLEMENTED (ADR-0013 — the live ADR-0060 shape)" \
  '! has_finding "$auout" adr-unpublished 62'
assert "adr-unpublished SILENT for a non-Accepted ADR absent from the integration branch (ADR-0014)" \
  '[ "$(grep -c -- "ADR-0014" <<<"$auout")" -eq 0 ]'
assert "adr-unpublished SILENT for an ADR whose change: resolves to no change file (ADR-0015)" \
  '[ "$(grep -c -- "ADR-0015" <<<"$auout")" -eq 0 ]'
assert "adr-unpublished names the ADR number in the message column (ADR-0010)" \
  '[ "$(grep -c -- "ADR-0010" <<<"$auout")" -eq 1 ]'
assert "adr-unpublished skips README.md (never reported as an ADR)" \
  '[ "$(grep -ci -- "README" <<<"$auout")" -eq 0 ]'
assert "adr-unpublished SILENT for a standalone Accepted ADR already published byte-identical on the integration branch (ADR-0016, adr_pub — the i_blob-present arm)" \
  '[ "$(grep -c -- "ADR-0016" <<<"$auout")" -eq 0 ]'
assert "exactly one adr-unpublished finding carries the '?' change-id (ADR-0010 only — adr_pub must not also fire)" \
  '[ "$(grep -cF -- "$(printf "adr-unpublished\t?\t")" <<<"$auout")" -eq 1 ]'
assert "adr-unpublished emits exactly the three due findings and nothing else" \
  '[ "$(grep -c "^adr-unpublished" <<<"$auout")" -eq 3 ]'
# M3(c): the change-id column carries only the change id (or '?'), not the ADR number — verify the
# ADR number itself is NAMED in the message for the change-tied findings too, not just the
# standalone one already checked above (ADR-0010).
assert "adr-unpublished names ADR-0011 in its message, not only the change-id column (cid 60)" \
  'grep -qF -- "ADR-0011" <<<"$auout"'
assert "adr-unpublished names ADR-0012 in its message, not only the change-id column (cid 61)" \
  'grep -qF -- "ADR-0012" <<<"$auout"'
# ADR-0049: the change-id column carries a shape-validated value only. `?` is the existing
# fallback for "no usable id" (padded_id_from_file), reused here rather than widening the column
# to admit an ADR reference — the ADR number rides the message column, which is the last field of
# the caller's `read` and therefore harmless.
# Fixed-string tab prefix, not `\?` in an ERE — see the st21 comment below for why.
au10="$(grep -F -- "$(printf 'adr-unpublished\t?\t')" <<<"$auout")"
assert "the standalone ADR's change-id column is the validated '?' fallback, not an ADR reference" \
  '[ -n "$au10" ]'
assert "adr-unpublished message names the integration branch" 'grep -qF -- "main" <<<"$au10"'

# ============================ adr-unpublished (stale arm) ============================
# An ADR present on BOTH branches whose bytes differ — the un-re-published status flip. A marker
# structurally cannot catch this: nothing failed at publish time, the file simply moved on.
# Fixture shape matters (green-suite-untested-branch): ADR-0020 is published and IDENTICAL, so the
# arm must distinguish drift from mere presence rather than firing on every published ADR.
write_adr 0020 Accepted ""
write_adr 0021 Accepted ""
git -C "$AU" add -A; git_quiet -C "$AU" commit -m "adrs 20,21 on docket"; git_quiet -C "$AU" push origin docket
# Publish both onto main verbatim, then drift ONLY 0021 on docket (a status flip).
git_quiet -C "$AU" checkout main
git_quiet -C "$AU" checkout docket -- docs/adrs/0020-adr-20.md docs/adrs/0021-adr-21.md
git -C "$AU" add -A; git_quiet -C "$AU" commit -m "publish adrs 20,21"; git_quiet -C "$AU" push origin main
git_quiet -C "$AU" checkout docket
sed -i.bak 's/^status: Accepted/status: Superseded by ADR-20/' "$AU/docs/adrs/0021-adr-21.md"
rm -f "$AU/docs/adrs/0021-adr-21.md.bak"
git -C "$AU" add -A; git_quiet -C "$AU" commit -m "flip adr 21 status"; git_quiet -C "$AU" push origin docket

stout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AU/docs/changes" \
  --adrs-dir "$AU/docs/adrs" --terminal-publish \
  --metadata-branch docket --integration-branch main 2>/dev/null)"

# Fixed-string tab prefix, not `\?` in an ERE (POSIX leaves `\?` in ERE undefined; the failure
# mode is silent — the pattern degrades to matching EVERY adr-unpublished line, making a `-n`
# assert vacuous). Computed once, up front, and reused below instead of re-deriving it — a
# `producer | grep -qF` chain would trip this file's own `set -uo pipefail` (see has_finding).
st21="$(grep -F -- "$(printf 'adr-unpublished\t?\t')" <<<"$stout" | grep -- "ADR-0021")"
assert "adr-unpublished fires the STALE arm for a published ADR whose bytes drifted (ADR-0021)" \
  '[ "$(grep -c -- "ADR-0021" <<<"$stout")" -eq 1 ]'
assert "the stale finding is an adr-unpublished line (one check-id, two messages)" \
  '[ -n "$st21" ]'
assert "adr-unpublished SILENT for a published ADR whose bytes MATCH (ADR-0020)" \
  '[ "$(grep -c -- "ADR-0020" <<<"$stout")" -eq 0 ]'
# Status-blindness: ADR-0021 is no longer Accepted, yet it is still due because it is already
# published. An Accepted-only gate on this arm would silence exactly the case it exists to catch.
assert "the stale message is DISTINCT from the missing message (says differs/re-publish, not absent)" \
  '! grep -qF -- "absent" <<<"$st21"'
assert "the stale message names both branches" \
  'grep -qF -- "docket" <<<"$st21" && grep -qF -- "main" <<<"$st21"'

# --- the SCRIPT-SIDE gate leg: no --terminal-publish => the check is entirely silent ---
augateout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AU/docs/changes" \
  --adrs-dir "$AU/docs/adrs" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "gate: without --terminal-publish the check emits NOTHING (spec §4.4)" \
  '[ "$(grep -c "^adr-unpublished" <<<"$augateout")" -eq 0 ]'
# ...and the whole check is opt-in on --adrs-dir too, so every pre-existing caller is unaffected.
aunodir="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AU/docs/changes" --terminal-publish \
  --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "gate: without --adrs-dir the check emits NOTHING" \
  '[ "$(grep -c "^adr-unpublished" <<<"$aunodir")" -eq 0 ]'
assert "board-checks still exits 0 with adr-unpublished findings (warn-only)" \
  'NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AU/docs/changes" --adrs-dir "$AU/docs/adrs" \
     --terminal-publish --metadata-branch docket --integration-branch main >/dev/null 2>&1'
assert "a missing --adrs-dir path is rejected up front (exit 2), never silently skipped" \
  '! bash "$SCRIPT" --changes-dir "$AU/docs/changes" --adrs-dir "$AU/docs/adrs-nope" \
     --terminal-publish --metadata-branch docket --integration-branch main >/dev/null 2>&1'
# An --adrs-dir that EXISTS but sits outside any git worktree must also be rejected up front (exit
# 2), not silently swallowed into an empty prefix — that would make every ADR ref lookup miss and
# the check would misreport EVERY ADR as unpublished (a false-positive storm, not a silent skip,
# but just as much a "never a silent failure mode" violation as the missing-dir case above).
AU_NOGIT="$(mktemp -d)"
assert "an --adrs-dir outside any git worktree is rejected up front (exit 2)" \
  '! bash "$SCRIPT" --changes-dir "$AU/docs/changes" --adrs-dir "$AU_NOGIT" \
     --terminal-publish --metadata-branch docket --integration-branch main >/dev/null 2>&1'

# ============================ adr-unpublished: additional coverage (review round) =============
# Three separate fixtures, isolated in their own repo: (a) i_blob present / m_blob absent — an ADR
# published on the integration branch that was never committed on the metadata branch at all (the
# stale arm's own "nothing to compare against" comment describes this shape, but nothing exercised
# it); (b) a change-tied ADR whose change IS terminal but whose status is NOT Accepted and which is
# absent from the integration branch — the due matrix's sixth cell; (c) an ADR sitting only in the
# working tree, never committed anywhere (I2) — a publish-gap finding here would print a remedy
# (`terminal-publish.sh --adr N`) that reads its copy-set from the metadata branch and fails.
read -r AU2 AU2_ORIGIN <<<"$(new_repo)"
mkdir -p "$AU2/docs/adrs"
cat > "$AU2/docs/changes/archive/2026-07-01-0070-done-change.md" <<'EOF'
---
id: 70
slug: done-change-2
title: A change that reached done
status: done
priority: medium
depends_on: []
trivial: true
EOF

# (b) change-tied (terminal change 70), status Proposed (NOT Accepted), absent from integration.
cat > "$AU2/docs/adrs/0030-adr-30.md" <<'EOF'
---
id: 30
slug: adr-30
title: ADR 30
status: Proposed
date: 2026-07-01
supersedes: []
reverses: []
relates_to: []
change: 70
---

## Context

c

## Decision

d

## Consequences

q
EOF
echo "# index" > "$AU2/docs/adrs/README.md"
git -C "$AU2" add docs/changes docs/adrs/0030-adr-30.md docs/adrs/README.md
git_quiet -C "$AU2" commit -m "au2 fixtures (change 70, adr 30)"
git_quiet -C "$AU2" push origin docket

# (a) standalone Accepted, committed onto main ONLY — never committed onto docket at all.
git_quiet -C "$AU2" checkout main
mkdir -p "$AU2/docs/adrs"
cat > "$AU2/docs/adrs/0031-adr-31.md" <<'EOF'
---
id: 31
slug: adr-31
title: ADR 31
status: Accepted
date: 2026-07-01
supersedes: []
reverses: []
relates_to: []
---

## Context

c

## Decision

d

## Consequences

q
EOF
git -C "$AU2" add docs/adrs/0031-adr-31.md
git_quiet -C "$AU2" commit -m "adr 31 on main only"
git_quiet -C "$AU2" push origin main
git_quiet -C "$AU2" checkout docket

# (c) I2: standalone Accepted, written to the working tree but never `git add`-ed anywhere.
cat > "$AU2/docs/adrs/0032-adr-32.md" <<'EOF'
---
id: 32
slug: adr-32
title: ADR 32
status: Accepted
date: 2026-07-01
supersedes: []
reverses: []
relates_to: []
---

## Context

c

## Decision

d

## Consequences

q
EOF

au2out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AU2/docs/changes" \
  --adrs-dir "$AU2/docs/adrs" --terminal-publish \
  --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "adr-unpublished SILENT: change-tied change IS terminal but ADR status is NOT Accepted, absent from integration (due matrix's sixth cell, ADR-0030)" \
  '[ "$(grep -c -- "ADR-0030" <<<"$au2out")" -eq 0 ]'
assert "adr-unpublished SILENT: ADR present on the integration branch only, never committed on the metadata branch (i_blob present / m_blob absent, ADR-0031)" \
  '[ "$(grep -c -- "ADR-0031" <<<"$au2out")" -eq 0 ]'
assert "adr-unpublished SILENT: ADR present in the working tree but never committed (I2, ADR-0032)" \
  '[ "$(grep -c -- "ADR-0032" <<<"$au2out")" -eq 0 ]'

# ============================ I1: an --adrs-dir that EXISTS but is EMPTY must not crash ========
# The normal state of a repo that opted into the check before writing its first ADR. Before the
# fix, `mapfile -t ADR_FILES < ...` (empty) then `for af in "${ADR_FILES[@]}"` threw an unbound-
# variable error under `set -u` on bash 4.0-4.3 (this repo's floor per ensure-docket-env.sh),
# aborting the script before the FINDINGS print and losing every OTHER check's output too.
read -r AE _ < <(new_repo)
mkdir -p "$AE/docs/adrs"   # exists, zero *.md files
cat > "$AE/docs/changes/active/0080-missing.md" <<'EOF'
---
id: 80
slug: missing-spec-2
title: Missing spec (I1 fixture)
status: proposed
priority: medium
depends_on: []
spec: docs/superpowers/specs/2026-06-01-ABSENT.md
trivial: false
EOF
aeout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$AE/docs/changes" \
  --adrs-dir "$AE/docs/adrs" --terminal-publish \
  --metadata-branch docket --integration-branch main 2>/dev/null)"; aerc=$?
assert "I1: an empty (but existing) --adrs-dir does not crash board-checks.sh (bash 4.0-4.3 unbound-var shape)" \
  '[ "$aerc" -eq 0 ]'
assert "I1: other checks still fire when --adrs-dir is empty (broken-spec, id 80)" \
  'has_finding "$aeout" broken-spec 80'

# --- registration: the check-id is documented everywhere it must be (correspondence guard) ------
# Derived by grep from the emitting script, never hand-listed: every check-id board-checks.sh can
# EMIT must appear in the script's own header set, in board-checks.md, and in docket-status.md's
# closed enumeration. Anchored on the emitting code so a new check-id added without registering
# reddens here (change 0104's three-mirror drift; tracked structurally as change 0111).
#
# The derivation keys on the call's SYNTACTIC SHAPE (`emit <id> "`), never on line position. An
# earlier version anchored `^[[:space:]]*emit`, requiring `emit` to be the first token on its
# line; it silently missed every `cond || emit ...` call (the `broken-spec` and field-loop sites in
# board-checks.sh — the
# broken-spec / broken-plan-results idiom), so the guard was decoration for 2 of the 12 real
# check-ids, and for any future check-id written with that idiom. `emit <id> "` doesn't care what
# precedes it on the line, and does NOT match the English "emit a table row" prose comment in the
# `renders_row` header
# — a real call is always `emit` + identifier + a quoted change-id argument; prose never quotes
# like that.
BCSH="$REPO/scripts/board-checks.sh"; BCMD="$REPO/scripts/board-checks.md"; DSMD="$REPO/scripts/docket-status.md"
emitted="$(grep -oE 'emit [a-z][a-z-]*[[:space:]]+"' "$BCSH" | awk '{print $2}' | sort -u)"

# Non-vacuity, CROSS-CHECKED rather than a hand-picked floor: a magic number like the old `-ge 8`
# sits below the true count by construction, so it can never catch an under-derivation — it didn't
# catch this file's own bug (10 cleared an `-ge 8` floor while the real count was 12). Instead
# derive an INDEPENDENT count from the script's own header comment (`check-id ∈ {...}`, see
# board-checks.sh's `check-id ∈ {…}` header enumeration — the set spans three comment lines, so
# the extraction joins them before
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
# `sort -u`'d, which comm requires. Matches the exact-set idiom in tests/test_docket_facade.sh's
# "docket.sh op set == docket.md documented op set" assert, for
# this same class of guard. The `|| { … >&2; false; }` tail reports WHICH ids disagree.
assert "emitted check-id SET == the header's own check-id ∈ {...} enumeration (a rename disagrees; a count compare would not)" \
  '[ -z "$(comm -3 <(printf "%s\n" "$emitted") <(printf "%s\n" "$header_ids"))" ] \
   || { comm -3 <(printf "%s\n" "$emitted") <(printf "%s\n" "$header_ids") >&2; false; }'
assert "publish-deferred is among the emitted check-ids" \
  'grep -qxF -- "publish-deferred" <<<"$emitted"'
# --- the two DOCUMENTATION surfaces, pinned in BOTH directions (change 0111) -------------------
# 0104 shipped this as a one-way membership loop: every EMITTED id must appear in each document.
# (Its `$BCSH` arm was dropped as tautological — `$emitted` is derived BY grepping `$BCSH` — and
# the header set-compare above is board-checks.sh's real surface.) That direction alone cannot see
# a PHANTOM: a check-id retired from the code but left behind in either document, or a typo'd
# extra entry, passed green. Both documents assert their enumeration is CLOSED — docket-status.md
# with `∈ {...}`, board-checks.md with its `### Check enumeration` heading — and a closed set that
# can silently over-claim is exactly the failure `correspondence-guard-runs-one-way` names. So each
# document is compared as a SET now, which pins both directions at once.
#
# Each extractor anchors on that document's own structural shape, never a hand-kept list:
#   board-checks.md  — per-check section heads, `**`<id>`**` at line start
#   docket-status.md — the single `check <check-id> ...` report-line row's `{...}` span
doc_ids="$(grep -oE '^\*\*`[a-z-]+`\*\*' "$BCMD" | sed -E 's/\*\*//g; s/`//g' | sort -u)"
ds_row_count="$(grep -cE '^\| `check <check-id>' "$DSMD")"
ds_ids="$(grep -E '^\| `check <check-id>' "$DSMD" \
  | sed -E 's/.*∈ \{([^}]*)\}.*/\1/' | tr ',' '\n' \
  | sed -E 's/^[[:space:]]+//; s/[[:space:]]+$//' | grep -v '^$' | sort -u)"

# Anchor integrity BEFORE the set compares: `grep -E ... | sed` yields an EMPTY set just as
# happily when the row was retitled as when the enumeration was emptied, and an empty set would
# make the compare fail loudly here but could pass vacuously in a future refactor of this block.
# Pinning the row count at exactly 1 distinguishes "the doc changed shape" from "the doc drifted".
assert "docket-status.md has exactly ONE 'check <check-id>' report-line row for the extractor to anchor on" \
  '[ "$ds_row_count" = 1 ]'
assert "the board-checks.md check-id extraction is non-empty (a retitled section head must redden, not pass vacuously)" \
  '[ "$(grep -c . <<<"$doc_ids")" -ge 1 ]'
assert "the docket-status.md check-id extraction is non-empty (a reflowed table row must redden, not pass vacuously)" \
  '[ "$(grep -c . <<<"$ds_ids")" -ge 1 ]'

# The description quotes the doc's own section-head spelling, apostrophes and backticks and all, so
# it is carried in a quoted-delimiter heredoc — inert text, no backtick inside double quotes (0221).
bc_doc_desc="$(cat <<'BCDESC'
emitted check-id SET == scripts/board-checks.md's per-check sections (add or remove a '**`<id>`**' section there)
BCDESC
)"
assert "$bc_doc_desc" \
  '[ -z "$(comm -3 <(printf "%s\n" "$emitted") <(printf "%s\n" "$doc_ids"))" ] \
   || { echo "board-checks.md drift (left=emitted only, right=documented only):" >&2; \
        comm -3 <(printf "%s\n" "$emitted") <(printf "%s\n" "$doc_ids") >&2; false; }'
assert "emitted check-id SET == scripts/docket-status.md's 'check <check-id>' enumeration (edit that row's {...} set)" \
  '[ -z "$(comm -3 <(printf "%s\n" "$emitted") <(printf "%s\n" "$ds_ids"))" ] \
   || { echo "docket-status.md drift (left=emitted only, right=documented only):" >&2; \
        comm -3 <(printf "%s\n" "$emitted") <(printf "%s\n" "$ds_ids") >&2; false; }'

# --- S0: the DECLARED vocabulary, sourced as a real runtime array (change 0111) -----------------
# board-checks.sh is NOT sourceable (it parses argv and runs the whole walk on source), so the
# vocabulary is declared in the lib that board-checks.sh already sources near the top of the file.
# That lets this
# guard read the REAL array rather than parsing source text for it — the same mechanism
# tests/test_render_board.sh uses for DOCKET_STATUSES (its `source "$LIB"` of
# lib/docket-frontmatter.sh), and it deletes a whole class of
# tokenizer fragility instead of relocating it.
LIB="$REPO/scripts/lib/docket-frontmatter.sh"
# shellcheck source=/dev/null
source "$LIB"

# 15 since change 0113 added aborted-run (14 at 0191's scalar-form; 13 at 0117's adr-unpublished).
# This literal is the ONE hand-edit the derived set-compares below do not absorb — bump it with
# every new id.
assert "BOARD_CHECK_IDS holds the 15 check-ids board-checks.sh emits" \
  '[ "${#BOARD_CHECK_IDS[@]}" = 15 ]'
assert "BOARD_CHECK_IDS SET == the set board-checks.sh actually emits (edit scripts/lib/docket-frontmatter.sh)" \
  '[ -z "$(comm -3 <(printf "%s\n" "${BOARD_CHECK_IDS[*]}" | tr " " "\n" | sort -u) <(printf "%s\n" "$emitted"))" ] \
   || { comm -3 <(printf "%s\n" "${BOARD_CHECK_IDS[*]}" | tr " " "\n" | sort -u) <(printf "%s\n" "$emitted") >&2; false; }'

# --- extractor integrity: every emit site uses a LITERAL check-id (change 0111) ----------------
# Everything above derives the emitted set with `emit <id> "` — a literal check-id followed by the
# quoted change-id argument. A site written `emit "$var" ...` matches none of it and is therefore
# invisible to every assert in this section, WITHOUT reddening any of them: the distinct-id set is
# unchanged whenever the dynamic site's id is also emitted somewhere else. Verified, not assumed —
# mutating board-checks.sh's slug-alphabet `emit field-domain` call to `emit "$dyn"` holds the set
# at 12 and
# drops the call-site count from 17 to 16. So the count is the only thing that can see it.
#
# Comments are stripped first so the header's prose (`emit a table row`, in the `renders_row`
# header) is out of scope; it
# would not match the literal shape anyway, but stripping makes the two counts comparable over the
# same text. `emit(){` is excluded for free by requiring the space after `emit`.
#
# Both counters are LINE-oriented, and `grep -vE '^[[:space:]]*#'` strips only FULL-LINE comments.
# Two legitimate shapes can therefore make the two counts disagree: an `emit` call split across a
# backslash line continuation, and an inline trailing `# ...` comment following `emit` on a code
# line. Either would redden this assert — never pass silently — so this is a known fail-loud gap,
# not a logic bug; rewrite the offending line rather than "fixing" the regex.
bcsh_code="$(grep -vE '^[[:space:]]*#' "$BCSH")"
emit_sites="$(grep -oE '\bemit [^;|&)]*' <<<"$bcsh_code" | grep -c .)"
emit_literal_sites="$(grep -oE '\bemit [a-z][a-z-]*[[:space:]]+"' <<<"$bcsh_code" | grep -c .)"

assert "board-checks.sh has emit call sites for the lint to inspect (17 at the 0111 baseline)" \
  '[ "$emit_sites" -ge 1 ]'
assert "every board-checks.sh emit call site names a LITERAL check-id (an 'emit \$var' site is invisible to this whole guard)" \
  '[ "$emit_sites" = "$emit_literal_sites" ] \
   || { echo "emit sites: $emit_sites, literal-id sites: $emit_literal_sites — a dynamic check-id would escape the set compares above" >&2; false; }'


# ============ SKILL.md must not restate the check-id vocabulary (change 0145) ============
# Change 0145 DELETED the count word, the five-item check-id list, and the hand-run invocation
# block from skills/docket-status/SKILL.md's `### Health checks` section. The 0111 correspondence
# guard above pins the check-id set across four surfaces, and SKILL.md was never one of them — so
# every new check-id drifted there silently while this suite stayed green. Rather than add a fifth
# pinned surface (which taxes every future check-id with another edit), the restatement was
# removed; this guard is what keeps it removed.
#
# SCOPED TO THE SECTION, NOT THE FILE. The `### Merge sweep` section legitimately names
# `publish-deferred` while explaining what that mark drives — it is the file's only check-id
# occurrence outside the section below, and a file-wide ban would redden honest prose.
#
# NAMED LIMITATION: section scoping means this stops the restatement returning *in this section
# only*. An editor who re-adds the list under a **new** heading escapes it — the non-vacuity
# anchor catches a *rename of* `### Health checks`, not a *new* section elsewhere.
#
# The extractor terminates on the next `^(#|##|###) ` heading OR EOF. The EOF arm is the LIVE
# path, not a fallback: `### Health checks` is currently the file's LAST section, so an extractor
# written as "lines between two heading matches" would yield the empty set and the negative assert
# would pass vacuously forever. The non-vacuity anchor below is what catches that — and catches a
# rename of the heading, which would otherwise silently disable this whole guard.
hc_section="$(awk '/^### Health checks[[:space:]]*$/{inhc=1;next} inhc && /^(#|##|###) /{exit} inhc' "$SKILL")"

assert "SKILL.md's '### Health checks' section is extractable and non-empty (a heading rename must redden, not pass vacuously)" \
  '[ -n "${hc_section//[[:space:]]/}" ]'
assert "SKILL.md's '### Health checks' section points at the authoritative enumeration (scripts/board-checks.md)" \
  'grep -qF "scripts/board-checks.md" <<<"$hc_section"'

# Word-boundary match, deliberately NOT backtick-anchored. A backtick-only matcher would miss a
# list re-added in bare form (`- broken-spec — ...`) AND would pass a mutation check written by
# copying the old backticked list — passing its own test while leaving the hole open. Every
# emitted id is a hyphenated compound that cannot occur in ordinary prose, so word-boundary
# matching costs no false positives. Consumed from a here-string, never a pipe: this file runs
# under `set -uo pipefail`, where piping into an early-exiting `grep -q` is a real hazard.
hc_restated="$(while IFS= read -r cid; do
                 [ -n "$cid" ] || continue
                 grep -qw -- "$cid" <<<"$hc_section" && printf "%s\n" "$cid"
               done <<<"$emitted")"
assert "no check-id is restated in SKILL.md's '### Health checks' section (point at scripts/board-checks.md, never a list)" \
  '[ -z "$hc_restated" ] || { echo "restated check-ids: $(echo $hc_restated)" >&2; false; }'

assert "0174 template integrity: the shared template is unmutated after the full run" \
  '[ "$(git -C "$NEW_REPO_TEMPLATE/tpl/origin.git" for-each-ref --format="%(refname) %(objectname)" | LC_ALL=C sort)" = "$tplint_refs" ] &&
   [ "$(git -C "$NEW_REPO_TEMPLATE/tpl/work" rev-parse HEAD)" = "$tplint_head" ] &&
   [ "$(git -C "$NEW_REPO_TEMPLATE/tpl/work" rev-parse --abbrev-ref HEAD)" = "$tplint_branch" ]'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
