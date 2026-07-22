#!/usr/bin/env bash
# tests/test_docket_frontmatter.sh — unit tests for the shared frontmatter/dependency helper
# (change 0022). Sources the library directly and asserts the accessors, resolve_deps arrays,
# and the readiness classifier. Run: bash tests/test_docket_frontmatter.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LIB="$REPO/scripts/lib/docket-frontmatter.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

assert "library exists" '[ -f "$LIB" ]'
# shellcheck source=/dev/null
source "$LIB"

# --- int_field: integer-only accessor ---
ifd="$(mktemp -d)"
printf -- '---\nid: 7\n---\n'    > "$ifd/ok.md"
printf -- '---\nid: 007\n---\n'  > "$ifd/pad.md"
printf -- '---\nid: 0\n---\n'    > "$ifd/zero.md"
printf -- '---\nid: abc\n---\n'  > "$ifd/abc.md"
printf -- '---\nid: 1.5\n---\n'  > "$ifd/dot.md"
printf -- '---\nid: 7x\n---\n'   > "$ifd/trail.md"
printf -- '---\nid: -3\n---\n'   > "$ifd/neg.md"
printf -- '---\nslug: x\n---\n'  > "$ifd/none.md"
assert "int_field accepts 7"        '[ "$(int_field "$ifd/ok.md" id)" = "7" ]'
assert "int_field accepts 007"      '[ "$(int_field "$ifd/pad.md" id)" = "007" ]'
assert "int_field accepts 0"        '[ "$(int_field "$ifd/zero.md" id)" = "0" ]'
assert "int_field rejects abc"      '[ -z "$(int_field "$ifd/abc.md" id)" ]'
assert "int_field rejects 1.5"      '[ -z "$(int_field "$ifd/dot.md" id)" ]'
assert "int_field rejects 7x"       '[ -z "$(int_field "$ifd/trail.md" id)" ]'
assert "int_field rejects -3"       '[ -z "$(int_field "$ifd/neg.md" id)" ]'
assert "int_field empty when unset" '[ -z "$(int_field "$ifd/none.md" id)" ]'
rm -rf "$ifd"

tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/active" "$tmp/archive"

# 10 done (satisfies a dep); 8 implemented (needs your merge); 3 proposed (not yet built)
cat > "$tmp/archive/2026-06-15-0010-juliet.md" <<'EOF'
---
id: 10
slug: juliet
title: Juliet feature
status: done
priority: medium
depends_on: []
EOF
cat > "$tmp/active/0008-hotel.md" <<'EOF'
---
id: 8
slug: hotel
title: Hotel feature
status: implemented
priority: high
depends_on: []
EOF
cat > "$tmp/active/0003-charlie.md" <<'EOF'
---
id: 3
slug: charlie
title: Charlie feature
status: proposed
priority: medium
depends_on: []
spec:
EOF
# 2: build-ready, dep on a done change (satisfied) + has spec
cat > "$tmp/active/0002-bravo.md" <<'EOF'
---
id: 2
slug: bravo
title: Bravo feature
status: proposed
priority: medium
depends_on: [10]
spec: docs/superpowers/specs/2026-06-10-bravo.md
EOF
# 5: waiting / not yet built (dep 3 is proposed)
cat > "$tmp/active/0005-echo.md" <<'EOF'
---
id: 5
slug: echo
title: Echo feature
status: proposed
priority: medium
depends_on: [3]
spec: docs/superpowers/specs/2026-06-10-echo.md
EOF
# 6: waiting / needs your merge (dep 8 is implemented)
cat > "$tmp/active/0006-foxtrot.md" <<'EOF'
---
id: 6
slug: foxtrot
title: Foxtrot feature
status: proposed
priority: medium
depends_on: [8]
spec: docs/superpowers/specs/2026-06-10-foxtrot.md
EOF
# 4: needs-brainstorm has the auto-groom-blocked body section
cat > "$tmp/active/0004-delta.md" <<'EOF'
---
id: 4
slug: delta
title: Delta feature
status: proposed
priority: low
depends_on: []
spec:
---

## Auto-groom blocked

2026-06-12 — abstained: needs a human call on scope.
EOF
# 14: proposed, no spec, and it only *talks about* the marker in prose — must NOT be blocked.
cat > "$tmp/active/0014-november.md" <<'EOF'
---
id: 14
slug: november
title: November feature
status: proposed
priority: low
depends_on: []
spec:
---

## Design

- A stub the groomer abstains on gets a dated `## Auto-groom blocked` body section
  (see change 0014) so the abstention is self-describing at the change.
EOF
# 13: implemented and genuinely carrying the finalize-blocked section (the true positive).
cat > "$tmp/active/0013-mike.md" <<'EOF'
---
id: 13
slug: mike
title: Mike feature
status: implemented
priority: high
depends_on: []
pr: https://github.com/o/r/pull/151
---

## Finalize blocked

2026-07-18 — ambiguous rebase conflict; resolve by hand and re-run.
EOF
# 15: implemented, and it only *talks about* the marker in prose — must NOT be blocked.
cat > "$tmp/active/0015-papa.md" <<'EOF'
---
id: 15
slug: papa
title: Papa feature
status: implemented
priority: low
depends_on: []
pr: https://github.com/o/r/pull/153
---

## Design

- A gate failure is marked with a dated `## Finalize blocked` section mirroring the
  proven `## Auto-groom blocked` marker — presence-encoded, cleared by hand.
EOF

# --- accessors ---
assert "field reads a scalar" '[ "$(field "$tmp/active/0008-hotel.md" status)" = "implemented" ]'
assert "field trims trailing space" '[ "$(field "$tmp/active/0008-hotel.md" priority)" = "high" ]'
assert "list_field expands a flow list" '[ "$(list_field "$tmp/active/0002-bravo.md" depends_on)" = "10" ]'
assert "list_field empty for []" '[ -z "$(list_field "$tmp/active/0008-hotel.md" depends_on)" ]'
assert "has_section finds a body line" 'has_section "$tmp/active/0004-delta.md" "## Auto-groom blocked"'
assert "has_section absent returns nonzero" '! has_section "$tmp/active/0003-charlie.md" "## Auto-groom blocked"'
# has_section is a WHOLE-LINE match. These markers are presence-encoded state, and change files
# routinely mention them inline in prose; an unanchored substring match (`grep -qF`) turned every
# such mention into a false "blocked" cell on the board. Both marker strings, both directions.
assert "has_section ignores an inline prose mention (auto-groom)" \
  '! has_section "$tmp/active/0014-november.md" "## Auto-groom blocked"'
assert "has_section ignores an inline prose mention (finalize)" \
  '! has_section "$tmp/active/0015-papa.md" "## Finalize blocked"'
assert "has_section still matches the real section it was pointed at" \
  'has_section "$tmp/active/0004-delta.md" "## Auto-groom blocked"'

# --- resolve_deps ---
resolve_deps "$tmp"
assert "STATUS_OF records own status" '[ "${STATUS_OF[10]}" = "done" ]'
assert "dep on done is clear" '[ "${DEP_STATE[2]}" = "clear" ] && [ -z "${DEP_REASON[2]}" ] && [ -z "${DEP_ON[2]}" ]'
assert "dep on proposed is waiting / not yet built" \
  '[ "${DEP_STATE[5]}" = "waiting" ] && [ "${DEP_REASON[5]}" = "not yet built" ] && [ "${DEP_ON[5]}" = "3" ]'
assert "dep on implemented is waiting / needs your merge" \
  '[ "${DEP_STATE[6]}" = "waiting" ] && [ "${DEP_REASON[6]}" = "needs your merge" ] && [ "${DEP_ON[6]}" = "8" ]'
assert "no deps is clear" '[ "${DEP_STATE[8]}" = "clear" ]'

# --- readiness ---
assert "readiness build-ready (spec + satisfied dep)" '[ "$(readiness "$tmp/active/0002-bravo.md")" = "build-ready" ]'
assert "readiness needs-brainstorm (no spec, not trivial)" '[ "$(readiness "$tmp/active/0003-charlie.md")" = "needs-brainstorm" ]'
assert "readiness auto-groom-blocked (no spec + blocked section)" '[ "$(readiness "$tmp/active/0004-delta.md")" = "auto-groom-blocked" ]'
assert "readiness waiting takes precedence over missing spec" '[ "$(readiness "$tmp/active/0005-echo.md")" = "waiting" ]'
assert "readiness waiting (needs-your-merge dep) returns waiting" '[ "$(readiness "$tmp/active/0006-foxtrot.md")" = "waiting" ]'
assert "readiness needs-brainstorm when the marker is only a prose mention" \
  '[ "$(readiness "$tmp/active/0014-november.md")" = "needs-brainstorm" ]'

# --- finalize_blocked (change 0087) ---
assert "finalize_blocked true for a real section" 'finalize_blocked "$tmp/active/0013-mike.md"'
assert "finalize_blocked false for a prose mention" '! finalize_blocked "$tmp/active/0015-papa.md"'
assert "finalize_blocked false when the section is absent" '! finalize_blocked "$tmp/active/0008-hotel.md"'

# --- iso_to_epoch: portable UTC ISO-8601 -> epoch ---
# Derive the oracle from the host's own date (GNU or BSD) so the test is host-portable —
# compare iso_to_epoch against that, never against a hardcoded epoch constant.
known="2026-07-17T12:00:00Z"
oracle="$(TZ=UTC date -u -d "$known" +%s 2>/dev/null || date -u -j -f '%Y-%m-%dT%H:%M:%SZ' "$known" +%s 2>/dev/null)"
got="$(iso_to_epoch "$known")"
assert "iso_to_epoch parses a UTC ISO-8601 timestamp" '[ -n "$got" ] && [ "$got" = "$oracle" ]'
assert "iso_to_epoch returns nonzero + empty on garbage" '! iso_to_epoch "not-a-timestamp" >/dev/null 2>&1'
assert "iso_to_epoch returns empty string on garbage" '[ -z "$(iso_to_epoch "not-a-timestamp" 2>/dev/null)" ]'

# --- shared board vocabularies (change 0116) ---
assert "DOCKET_PRIORITIES is rank-ordered critical > high > medium > low" \
  '[ "${DOCKET_PRIORITIES[*]:-}" = "critical high medium low" ]'
assert "DOCKET_PRIORITIES has exactly four members" '[ "${#DOCKET_PRIORITIES[@]}" = 4 ]' 2>/dev/null
assert "active-status helper is defined" 'declare -F docket_status_is_active >/dev/null'
assert "terminal-status helper is defined" 'declare -F docket_status_is_terminal >/dev/null'
assert "priority-membership helper is defined" 'declare -F docket_priority_is_member >/dev/null'
assert "priority-rank helper is defined" 'declare -F docket_priority_rank >/dev/null'
assert "DOCKET_PRIORITY_DEFAULT is a declared priority" \
  'docket_priority_is_member "${DOCKET_PRIORITY_DEFAULT:-}"'
assert "active helper accepts proposed" 'docket_status_is_active proposed'
assert "active helper rejects terminal done" '! docket_status_is_active done'
assert "active helper rejects empty" '! docket_status_is_active ""'
assert "terminal helper accepts killed" 'docket_status_is_terminal killed'
assert "terminal helper rejects active implemented" '! docket_status_is_terminal implemented'
assert "terminal helper rejects empty" '! docket_status_is_terminal ""'
assert "priority membership accepts high" 'docket_priority_is_member high'
assert "priority membership rejects empty" '! docket_priority_is_member ""'
assert "priority membership rejects unknown" '! docket_priority_is_member urgent'
assert "priority rank derives critical as zero" '[ "$(docket_priority_rank critical)" = 0 ]'
assert "priority rank derives low as three" '[ "$(docket_priority_rank low)" = 3 ]'
assert "priority rank defaults empty to medium's index" '[ "$(docket_priority_rank "")" = 2 ]'
assert "priority rank defaults unknown to medium's index" '[ "$(docket_priority_rank urgent)" = 2 ]'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
