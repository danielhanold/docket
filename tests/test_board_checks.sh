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
title: Trivial, no spec
status: proposed
priority: medium
depends_on: []
spec:
trivial: true
EOF
out="$(NOW=$NOW_EPOCH bash "$SCRIPT" --changes-dir "$W/docs/changes" --metadata-branch docket --integration-branch main 2>/dev/null)"
assert "broken-spec fires for a missing spec path (id 2)" 'has_finding "$out" broken-spec 2'
assert "broken-spec silent for a present spec (id 1)" '! has_finding "$out" broken-spec 1'
assert "broken-spec silent for a trivial change with no spec (id 3, carve-out)" '! has_finding "$out" broken-spec 3'

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

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
