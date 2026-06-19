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
read -r S _ < <(new_repo)
STALE_EPOCH=$(( NOW_EPOCH - 4*86400 ))
FRESH_EPOCH=$(( NOW_EPOCH - 3600 ))
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
sout="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$S/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "stale-in-progress fires for a branch idle >3 days (id 20)" \
  'has_finding "$sout" stale-in-progress 20'
assert "stale-in-progress silent for a branch with a recent commit (id 21)" \
  '! has_finding "$sout" stale-in-progress 21'
assert "stale-in-progress silent when branch: set but branch absent (id 22, carve-out)" \
  '! has_finding "$sout" stale-in-progress 22'

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

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
