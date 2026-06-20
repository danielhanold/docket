# Script docket-adr's deterministic passes — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Lift `docket-adr`'s three remaining model-prose deterministic passes — ADR index render, ADR ledger validation, and ADR-only terminal-publish — into tested shell, bringing `docket-adr` to parity with the 0022/0023/0025 scripting sweep.

**Architecture:** Two new offline scripts that mirror existing siblings (`render-adr-index.sh` ↔ `render-board.sh`, `adr-checks.sh` ↔ `board-checks.sh`), both sourcing `scripts/lib/docket-frontmatter.sh`; plus an `--adr <NN>` mode added to the existing `terminal-publish.sh` so it becomes the single executor of both publish shapes. Then the two skill bodies (`docket-adr`, `docket-finalize-change`) are rewired to invoke the scripts instead of hand-rendering / hand-running git. No ADR or board *semantics* change — faithful re-implementation only.

**Tech Stack:** Bash (≥4, per the existing scripts — `mapfile`, `declare -A`), `git`, the shared `scripts/lib/docket-frontmatter.sh` accessors (`field`, `list_field`). Hermetic shell tests in `tests/` (bare-origin clones; no `gh`, no network), run individually as `bash tests/test_NAME.sh`.

## Global Constraints

- **Faithful re-implementation, not redesign.** Index grouping, row format, the three checks, and the publish mechanics are reproduced exactly from the current `docket-adr` / `docket-finalize-change` prose. Do NOT add new ADR/board semantics.
- **Offline + deterministic.** The two new scripts touch only the filesystem — no `gh`, no `git`, no network. Same ADR files ⇒ byte-identical output.
- **No git writes in the renderer/validator.** They emit to stdout; the caller redirects + commits (the `render-board.sh` discipline).
- **The renderer emits the raw frontmatter `title:` verbatim.** It does NOT reproduce hand-added markdown. The current committed index row for ADR-0001 shows ``orphan `docket` branch`` (backticks) while its frontmatter `title:` has none — the deterministic renderer will emit the un-backticked title. Regenerating the real index therefore normalizes that hand-drift away; this is the intended self-healing (spec §1), not a bug. The **golden fixture authored in the test is the byte-for-byte contract**, NOT the current real `README.md`.
- **ADR references are 4-digit zero-padded** everywhere they appear (`ADR-0001`, filenames), matching the committed index. **Change back-references are bare** (`← change #2`).
- **`adrs:` / `supersedes:` / `reverses:` / `relates_to:` are flat list frontmatter** — `list_field` already covers them (no `yq`).
- **No `producer | grep -q` under `pipefail`** (LEARNINGS #11/#16): capture into a var, then grep the var.
- **Anchor any frontmatter-field write to the first `---…---` block** (LEARNINGS #25) — N/A here since the new scripts never write fields, but keep in mind for terminal-publish.
- **Golden fixtures use real-shaped values + plurality** (LEARNINGS #22): full status strings, `change:` back-links, ≥2 entries in every list an annotation renders, ≥2 rows per non-empty group; and the renderer/validator are **smoke-tested against the real ADR ledger before merge** (Task 5).
- **Test files print `ok - ` / `NOT OK - ` per assertion and `exit "$fail"`**, matching the existing `tests/*.sh` harness shape.

---

### Task 1: `render-adr-index.sh` — the ADR index renderer

**Files:**
- Create: `scripts/render-adr-index.sh`
- Test: `tests/test_render_adr_index.sh`

**Interfaces:**
- Consumes: `scripts/lib/docket-frontmatter.sh` (`field FILE KEY`, `list_field FILE KEY`).
- Produces: CLI `render-adr-index.sh --adrs-dir DIR` → emits the index markdown to **stdout**. Exit 2 on a usage error (missing/invalid `--adrs-dir`).

**Output contract (locked by the golden fixture in this task):**

```
# Architecture Decision Records

Immutable, numbered record of *why*. ADRs are never archived or rewritten; once `Accepted`, only the `status:` line changes (on supersession/reversal). This index is generated — do not hand-edit.

## Active

- [ADR-NNNN](<file>.md) — <title> (<status>)<annotations>
…

## Superseded / Reversed

_None._        ← or rows

## Deprecated

_None._        ← or rows
```

- Three groups, **always in this order**: Active, Superseded / Reversed, Deprecated. Each row sorted by **ascending numeric id**. An empty group renders the literal `_None._`.
- **Grouping by `status:`** — `Accepted` (and any `Proposed`/`Draft`/unknown) → **Active**; `Superseded by …` / `Reversed by …` → **Superseded / Reversed**; `Deprecated` → **Deprecated**.
- **Row:** `- [ADR-NNNN](<file>.md) — <title> (<status>)` then annotations in fixed order, each space-prefixed:
  - ` ← change #N` when `change:` is set (bare N);
  - ` → supersedes ADR-NNNN[, ADR-NNNN]` when `supersedes:` non-empty;
  - ` → reverses ADR-NNNN[, ADR-NNNN]` when `reverses:` non-empty;
  - ` · relates to ADR-NNNN[, ADR-NNNN]` when `relates_to:` non-empty.
- **No generated-at timestamp** (consistent with `render-board.sh`). File ends with a single trailing newline after the last group's content (no trailing blank line).

- [ ] **Step 1: Write the failing test** (`tests/test_render_adr_index.sh`)

```bash
#!/usr/bin/env bash
# tests/test_render_adr_index.sh — verifies change 0030: deterministic ADR index rendering.
# A fixture adrs/ tree spanning Active (with change:/relates_to:/→supersedes/→reverses), a
# Superseded, a Reversed, and a Deprecated entry is rendered and byte-compared to a hand-authored
# golden; a second render must be byte-identical (idempotence); an empty-ledger case renders all
# three groups as _None._. Also asserts docket-adr wiring. Run: bash tests/test_render_adr_index.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$REPO/scripts/render-adr-index.sh"
SKILL="$REPO/skills/docket-adr/SKILL.md"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

assert "script exists and is executable" '[ -x "$SCRIPT" ]'

tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT

# --- Active: two Accepted ADRs (plurality), one with change: + relates_to (plural), one with
#     → supersedes (plural) AND → reverses to exercise every annotation + the ", " joiner. ---
cat > "$tmp/0001-alpha.md" <<'EOF'
---
id: 1
slug: alpha
title: Alpha decision
status: Accepted
date: 2026-06-01
supersedes: []
reverses: []
relates_to: []
change: 2
---
## Decision
A.
EOF
cat > "$tmp/0002-bravo.md" <<'EOF'
---
id: 2
slug: bravo
title: Bravo decision
status: Accepted
date: 2026-06-02
supersedes: []
reverses: []
relates_to: [1, 5]
change: 3
---
## Decision
B.
EOF
cat > "$tmp/0006-foxtrot.md" <<'EOF'
---
id: 6
slug: foxtrot
title: Foxtrot decision
status: Accepted
date: 2026-06-06
supersedes: [4, 5]
reverses: [3]
relates_to: []
change:
---
## Decision
F.
EOF
# --- Superseded / Reversed: one of each (plurality in the group). ---
cat > "$tmp/0004-delta.md" <<'EOF'
---
id: 4
slug: delta
title: Delta decision
status: Superseded by ADR-0006
date: 2026-06-04
supersedes: []
reverses: []
relates_to: []
change: 4
---
## Decision
D.
EOF
cat > "$tmp/0005-echo.md" <<'EOF'
---
id: 5
slug: echo
title: Echo decision
status: Superseded by ADR-0006
date: 2026-06-05
supersedes: []
reverses: []
relates_to: []
change:
---
## Decision
E.
EOF
cat > "$tmp/0003-charlie.md" <<'EOF'
---
id: 3
slug: charlie
title: Charlie decision
status: Reversed by ADR-0006
date: 2026-06-03
supersedes: []
reverses: []
relates_to: []
change:
---
## Decision
C.
EOF
# --- Deprecated. ---
cat > "$tmp/0007-golf.md" <<'EOF'
---
id: 7
slug: golf
title: Golf decision
status: Deprecated
date: 2026-06-07
supersedes: []
reverses: []
relates_to: []
change:
---
## Decision
G.
EOF
# A README.md must be ignored (never indexed, never a row).
echo "# old index" > "$tmp/README.md"

# Hand-authored golden — the byte-for-byte contract for the renderer.
golden="$tmp/golden.md"
cat > "$golden" <<'EOF'
# Architecture Decision Records

Immutable, numbered record of *why*. ADRs are never archived or rewritten; once `Accepted`, only the `status:` line changes (on supersession/reversal). This index is generated — do not hand-edit.

## Active

- [ADR-0001](0001-alpha.md) — Alpha decision (Accepted) ← change #2
- [ADR-0002](0002-bravo.md) — Bravo decision (Accepted) ← change #3 · relates to ADR-0001, ADR-0005
- [ADR-0006](0006-foxtrot.md) — Foxtrot decision (Accepted) → supersedes ADR-0004, ADR-0005 → reverses ADR-0003

## Superseded / Reversed

- [ADR-0003](0003-charlie.md) — Charlie decision (Reversed by ADR-0006)
- [ADR-0004](0004-delta.md) — Delta decision (Superseded by ADR-0006)
- [ADR-0005](0005-echo.md) — Echo decision (Superseded by ADR-0006)

## Deprecated

- [ADR-0007](0007-golf.md) — Golf decision (Deprecated)
EOF

rendered="$tmp/out.md"
bash "$SCRIPT" --adrs-dir "$tmp" > "$rendered" 2>/dev/null
assert "rendered output matches the golden byte-for-byte" 'diff -u "$golden" "$rendered"'

rendered2="$tmp/out2.md"
bash "$SCRIPT" --adrs-dir "$tmp" > "$rendered2" 2>/dev/null
assert "render is idempotent (re-run is byte-identical)" 'diff -u "$rendered" "$rendered2"'

assert "README.md is never emitted as a row" '! grep -q "old index" "$rendered"'

# --- empty-ledger: all three groups render _None._ ---
empty="$(mktemp -d)"
emptyout="$(bash "$SCRIPT" --adrs-dir "$empty" 2>/dev/null)"
assert "empty ledger: Active group renders _None._" \
  'printf "%s\n" "$emptyout" | awk "/## Active/{f=1;next} /## /{f=0} f&&/_None._/{ok=1} END{exit !ok}"'
assert "empty ledger: three _None._ lines (one per group)" \
  '[ "$(printf "%s\n" "$emptyout" | grep -c "^_None\._$")" -eq 3 ]'
rm -rf "$empty"

# --- usage errors ---
bash "$SCRIPT" >/dev/null 2>&1; assert "missing --adrs-dir exits 2" '[ "$?" -eq 2 ]'
bash "$SCRIPT" --adrs-dir "$tmp/nope" >/dev/null 2>&1; assert "absent dir exits 2" '[ "$?" -eq 2 ]'

# --- docket-adr wiring sentinels (the SKILL is code on the integration branch) ---
assert "docket-adr Index/validate invokes render-adr-index.sh" 'grep -qF "scripts/render-adr-index.sh" "$SKILL"'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_render_adr_index.sh`
Expected: FAIL — `NOT OK - script exists and is executable` (script absent), and the wiring sentinel fails (SKILL not yet edited — that line goes green in Task 4; it is fine for it to be red now).

- [ ] **Step 3: Write `scripts/render-adr-index.sh`**

```bash
#!/usr/bin/env bash
# scripts/render-adr-index.sh — deterministic, idempotent renderer for the ADR index
# (<adrs_dir>/README.md), change 0030. The exact analog of render-board.sh (0022): reads the ADR
# files and emits the index to STDOUT byte-for-byte per docket-adr's *Index / validate* structure.
# No git writes (the caller redirects + commits), offline (no gh, no git, no network). Same ADR
# files => identical bytes. Reuses lib/docket-frontmatter.sh.
#
# Usage: render-adr-index.sh --adrs-dir DIR
set -uo pipefail

ADRS_DIR=""
while [ $# -gt 0 ]; do
  case "$1" in
    --adrs-dir) ADRS_DIR="$2"; shift ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) printf 'render-adr-index: unknown argument: %s\n' "$1" >&2; exit 2 ;;
  esac
  shift
done
[ -n "$ADRS_DIR" ] || { printf 'render-adr-index: missing --adrs-dir\n' >&2; exit 2; }
[ -d "$ADRS_DIR" ] || { printf 'render-adr-index: adrs dir not found: %s\n' "$ADRS_DIR" >&2; exit 2; }

# shellcheck source=/dev/null
source "$(dirname "${BASH_SOURCE[0]}")/lib/docket-frontmatter.sh"

pad(){ printf '%04d' "$1"; }                       # bare id -> 4-digit
adr_list(){ # "1 5" -> "ADR-0001, ADR-0005"
  local out="" x
  for x in $1; do [ -n "$out" ] && out+=", "; out+="ADR-$(pad "$x")"; done
  printf '%s' "$out"
}

# --- single scan: collect every ADR (excluding README.md) into parallel maps + group buckets ---
declare -A T_FILE T_TITLE T_STATUS T_CHANGE T_SUPS T_REVS T_REL
ACTIVE_IDS=""; SUPREV_IDS=""; DEPR_IDS=""
mapfile -t FILES < <(find "$ADRS_DIR" -maxdepth 1 -name '*.md' ! -name 'README.md' 2>/dev/null | sort)
for f in "${FILES[@]}"; do
  id="$(field "$f" id)"; [ -n "$id" ] || continue
  T_FILE["$id"]="$(basename "$f")"
  T_TITLE["$id"]="$(field "$f" title)"
  T_STATUS["$id"]="$(field "$f" status)"
  T_CHANGE["$id"]="$(field "$f" change)"
  T_SUPS["$id"]="$(list_field "$f" supersedes)"
  T_REVS["$id"]="$(list_field "$f" reverses)"
  T_REL["$id"]="$(list_field "$f" relates_to)"
  case "${T_STATUS[$id]}" in
    "Superseded by"*|"Reversed by"*) SUPREV_IDS+="$id"$'\n' ;;
    Deprecated)                      DEPR_IDS+="$id"$'\n' ;;
    *)                               ACTIVE_IDS+="$id"$'\n' ;;   # Accepted/Proposed/draft/unknown
  esac
done

row(){ # row ID
  local id="$1" line ann=""
  line="- [ADR-$(pad "$id")](${T_FILE[$id]}) — ${T_TITLE[$id]} (${T_STATUS[$id]})"
  [ -n "${T_CHANGE[$id]}" ]  && ann+=" ← change #${T_CHANGE[$id]}"
  [ -n "${T_SUPS[$id]}" ]    && ann+=" → supersedes $(adr_list "${T_SUPS[$id]}")"
  [ -n "${T_REVS[$id]}" ]    && ann+=" → reverses $(adr_list "${T_REVS[$id]}")"
  [ -n "${T_REL[$id]}" ]     && ann+=" · relates to $(adr_list "${T_REL[$id]}")"
  printf '%s%s\n' "$line" "$ann"
}

emit_group(){ # emit_group HEADER IDSTR
  printf '\n## %s\n\n' "$1"
  local sorted id
  sorted="$(printf '%s' "$2" | sed '/^$/d' | sort -n)"
  if [ -z "$sorted" ]; then printf '_None._\n'; return; fi
  while IFS= read -r id; do [ -n "$id" ] && row "$id"; done <<<"$sorted"
}

printf '# Architecture Decision Records\n\n'
printf 'Immutable, numbered record of *why*. ADRs are never archived or rewritten; once `Accepted`, only the `status:` line changes (on supersession/reversal). This index is generated — do not hand-edit.\n'
emit_group "Active" "$ACTIVE_IDS"
emit_group "Superseded / Reversed" "$SUPREV_IDS"
emit_group "Deprecated" "$DEPR_IDS"
```

- [ ] **Step 4: Make it executable**

Run: `chmod +x scripts/render-adr-index.sh`

- [ ] **Step 5: Run the test to verify it passes** (except the Task-4 wiring sentinel)

Run: `bash tests/test_render_adr_index.sh`
Expected: every assertion `ok -` EXCEPT `docket-adr Index/validate invokes render-adr-index.sh` (red until Task 4). If the golden diff fails, fix the **renderer** to match the golden — the golden is the contract.

- [ ] **Step 6: Commit**

```bash
git add scripts/render-adr-index.sh tests/test_render_adr_index.sh
git commit -m "feat(0030): render-adr-index.sh — deterministic ADR index renderer + golden test"
```

---

### Task 2: `adr-checks.sh` — the ADR ledger validator

**Files:**
- Create: `scripts/adr-checks.sh`
- Test: `tests/test_adr_checks.sh`

**Interfaces:**
- Consumes: `scripts/lib/docket-frontmatter.sh` (`field`, `list_field`).
- Produces: CLI `adr-checks.sh --adrs-dir DIR [--strict]` → one finding per line on stdout, **TAB-separated `<check-id>\t<adr-id>\t<message>`**, sorted by `(check-id, adr-id)`. Clean ledger ⇒ no output, exit 0. `--strict` ⇒ exit 1 if any finding. Exit 2 on usage error.
- `check-id ∈ {adr-numbering-gap, adr-dangling-link, adr-status-inconsistent}`.

**Check semantics (reproduced exactly from `docket-adr`'s validation prose):**
- **`adr-numbering-gap`** — an id missing from `1..max` (one finding per missing id, keyed on the missing id).
- **`adr-dangling-link`** — a `supersedes:` / `reverses:` / `relates_to:` value referencing an id with no file (keyed on the *referencing* ADR; one finding per dangling ref).
- **`adr-status-inconsistent`** —
  - **arm (a):** an ADR whose `status:` is `Superseded by ADR-NN` / `Reversed by ADR-NN` but no ADR NN exists (keyed on the badly-statused ADR).
  - **arm (b):** an ADR that `supersedes:` / `reverses:` a target whose `status:` was **not** flipped to point back at it (keyed on the *target* — the un-flipped ADR — message names the referencing ADR). A dangling target (no file) is skipped here (already covered by `adr-dangling-link`).

- [ ] **Step 1: Write the failing test** (`tests/test_adr_checks.sh`)

```bash
#!/usr/bin/env bash
# tests/test_adr_checks.sh — verifies change 0030: scripts/adr-checks.sh, the ADR-ledger analog of
# board-checks.sh (numbering gaps, dangling links, status inconsistencies). Offline (no gh, no
# network); warn-only. Run: bash tests/test_adr_checks.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$REPO/scripts/adr-checks.sh"
SKILL="$REPO/skills/docket-adr/SKILL.md"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }
# has_finding OUTPUT CHECK-ID ADR-ID — literal-TAB ERE (portable; no grep -P).
has_finding(){ printf '%s' "$1" | grep -qE "$(printf '^%s\t%s\t' "$2" "$3")"; }

mkadr(){ # mkadr DIR ID STATUS SUPERSEDES REVERSES RELATES  (lists like "[]" or "[4]")
  cat > "$1/$(printf '%04d' "$2")-a$2.md" <<EOF
---
id: $2
slug: a$2
title: Decision $2
status: $3
date: 2026-06-01
supersedes: $4
reverses: $5
relates_to: $6
change:
---
## Decision
x.
EOF
}

assert "script exists and is executable" '[ -x "$SCRIPT" ]'

# ===== clean ledger: 1,2,3 all Accepted, 3 supersedes-free => no output, exit 0 =====
d="$(mktemp -d)"
mkadr "$d" 1 Accepted "[]" "[]" "[]"
mkadr "$d" 2 Accepted "[]" "[]" "[1]"
mkadr "$d" 3 Accepted "[]" "[]" "[]"
echo "# index" > "$d/README.md"
out="$(bash "$SCRIPT" --adrs-dir "$d" 2>/dev/null)"; rc=$?
assert "clean ledger: no output" '[ -z "$out" ]'
assert "clean ledger: exit 0" '[ "$rc" -eq 0 ]'
bash "$SCRIPT" --adrs-dir "$d" --strict >/dev/null 2>&1
assert "clean ledger: --strict exits 0" '[ "$?" -eq 0 ]'
rm -rf "$d"

# ===== adr-numbering-gap: 1 and 3 present, 2 missing =====
d="$(mktemp -d)"
mkadr "$d" 1 Accepted "[]" "[]" "[]"
mkadr "$d" 3 Accepted "[]" "[]" "[]"
out="$(bash "$SCRIPT" --adrs-dir "$d" 2>/dev/null)"
assert "numbering-gap flagged on missing id 2" 'has_finding "$out" adr-numbering-gap 2'
assert "numbering-gap NOT flagged on present id 1" '! has_finding "$out" adr-numbering-gap 1'
rm -rf "$d"

# ===== adr-dangling-link: ADR-2 relates_to [9] which has no file =====
d="$(mktemp -d)"
mkadr "$d" 1 Accepted "[]" "[]" "[]"
mkadr "$d" 2 Accepted "[]" "[]" "[9]"
out="$(bash "$SCRIPT" --adrs-dir "$d" 2>/dev/null)"
assert "dangling-link flagged on ADR-2 (relates_to 9 absent)" 'has_finding "$out" adr-dangling-link 2'
rm -rf "$d"

# ===== adr-status-inconsistent arm (a): status 'Superseded by ADR-0099', no ADR-99 =====
d="$(mktemp -d)"
mkadr "$d" 1 Accepted "[]" "[]" "[]"
mkadr "$d" 2 "Superseded by ADR-0099" "[]" "[]" "[]"
out="$(bash "$SCRIPT" --adrs-dir "$d" 2>/dev/null)"
assert "status-inconsistent (a) flagged on ADR-2 (target 99 absent)" 'has_finding "$out" adr-status-inconsistent 2'
rm -rf "$d"

# ===== adr-status-inconsistent arm (b): ADR-2 supersedes [1] but ADR-1 status still Accepted =====
d="$(mktemp -d)"
mkadr "$d" 1 Accepted "[]" "[]" "[]"
mkadr "$d" 2 Accepted "[1]" "[]" "[]"
out="$(bash "$SCRIPT" --adrs-dir "$d" 2>/dev/null)"
assert "status-inconsistent (b) flagged on un-flipped target ADR-1" 'has_finding "$out" adr-status-inconsistent 1'
# control: when ADR-1 IS flipped, arm (b) is silent.
d2="$(mktemp -d)"
mkadr "$d2" 1 "Superseded by ADR-0002" "[]" "[]" "[]"
mkadr "$d2" 2 Accepted "[1]" "[]" "[]"
out2="$(bash "$SCRIPT" --adrs-dir "$d2" 2>/dev/null)"
assert "status-inconsistent (b) silent when target correctly flipped" '! has_finding "$out2" adr-status-inconsistent 1'
rm -rf "$d" "$d2"

# ===== --strict exits 1 on any finding =====
d="$(mktemp -d)"
mkadr "$d" 1 Accepted "[]" "[]" "[]"
mkadr "$d" 3 Accepted "[]" "[]" "[]"   # gap at 2
bash "$SCRIPT" --adrs-dir "$d" --strict >/dev/null 2>&1
assert "--strict exits 1 when findings exist" '[ "$?" -eq 1 ]'
rm -rf "$d"

# ===== usage =====
bash "$SCRIPT" >/dev/null 2>&1; assert "missing --adrs-dir exits 2" '[ "$?" -eq 2 ]'

# ===== docket-adr wiring sentinel =====
assert "docket-adr Index/validate invokes adr-checks.sh" 'grep -qF "scripts/adr-checks.sh" "$SKILL"'

if [ "$fail" = 0 ]; then echo "PASS"; else echo "FAIL"; fi
exit "$fail"
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_adr_checks.sh`
Expected: FAIL — `NOT OK - script exists and is executable` (plus the Task-4 wiring sentinel, red until Task 4).

- [ ] **Step 3: Write `scripts/adr-checks.sh`**

```bash
#!/usr/bin/env bash
# scripts/adr-checks.sh — the ADR-ledger analog of board-checks.sh (change 0030). Sources the shared
# frontmatter helper (0022) and walks the ADR files, emitting one finding per line on stdout. Offline
# (no gh, no network) and warn-only (never auto-fixes); the caller (docket-adr) surfaces the lines.
#
# Usage: adr-checks.sh --adrs-dir DIR [--strict]
#   Findings: TAB-separated  <check-id>\t<adr-id>\t<message>  on stdout, sorted by (check-id, adr-id).
#     check-id ∈ {adr-numbering-gap, adr-dangling-link, adr-status-inconsistent}
#   Clean ledger => no output, exit 0. --strict => exit 1 if any finding (a future CI gate).
set -uo pipefail

ADRS_DIR=""; STRICT=0
while [ $# -gt 0 ]; do
  case "$1" in
    --adrs-dir) ADRS_DIR="$2"; shift ;;
    --strict) STRICT=1 ;;
    -h|--help) grep '^#' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) printf 'adr-checks: unknown argument: %s\n' "$1" >&2; exit 2 ;;
  esac
  shift
done
[ -n "$ADRS_DIR" ] || { printf 'adr-checks: missing --adrs-dir\n' >&2; exit 2; }
[ -d "$ADRS_DIR" ] || { printf 'adr-checks: adrs dir not found: %s\n' "$ADRS_DIR" >&2; exit 2; }

# shellcheck source=/dev/null
source "$(dirname "${BASH_SOURCE[0]}")/lib/docket-frontmatter.sh"

pad(){ printf '%04d' "$1"; }

# --- single scan: existence + status + cross-ref lists, keyed by integer id ---
declare -A EXISTS STATUS SUPS REVS REL
IDS=""; MAXID=0
mapfile -t FILES < <(find "$ADRS_DIR" -maxdepth 1 -name '*.md' ! -name 'README.md' 2>/dev/null | sort)
for f in "${FILES[@]}"; do
  id="$(field "$f" id)"; [ -n "$id" ] || continue
  EXISTS["$id"]=1
  STATUS["$id"]="$(field "$f" status)"
  SUPS["$id"]="$(list_field "$f" supersedes)"
  REVS["$id"]="$(list_field "$f" reverses)"
  REL["$id"]="$(list_field "$f" relates_to)"
  IDS+="$id"$'\n'
  [ "$id" -gt "$MAXID" ] && MAXID="$id"
done

FINDINGS=""
emit(){ FINDINGS+="$1"$'\t'"$2"$'\t'"$3"$'\n'; }

# status_target STATUS -> bare integer id from "Superseded by ADR-0006" / "Reversed by ADR-0006" ("" otherwise)
status_target(){
  case "$1" in
    "Superseded by ADR-"*|"Reversed by ADR-"*)
      local t="${1##*ADR-}"; t="${t%% *}"; printf '%d' "$((10#$t))" ;;
    *) printf '' ;;
  esac
}

# --- adr-numbering-gap: every id missing from 1..MAXID ---
n=1
while [ "$n" -le "$MAXID" ]; do
  [ -z "${EXISTS[$n]:-}" ] && emit adr-numbering-gap "$n" "no ADR file for id $n (gap in 1..$MAXID)"
  n=$(( n + 1 ))
done

# iterate ids in ascending numeric order for deterministic per-adr findings
SORTED_IDS="$(printf '%s' "$IDS" | sed '/^$/d' | sort -n)"
while IFS= read -r id; do
  [ -n "$id" ] || continue

  # --- adr-dangling-link: any cross-ref to an id with no file ---
  for ref in ${SUPS[$id]} ${REVS[$id]} ${REL[$id]}; do
    [ -z "${EXISTS[$ref]:-}" ] && emit adr-dangling-link "$id" "references ADR-$(pad "$ref") which has no file"
  done

  # --- adr-status-inconsistent arm (a): status says Superseded/Reversed by a non-existent ADR ---
  tgt="$(status_target "${STATUS[$id]}")"
  if [ -n "$tgt" ] && [ -z "${EXISTS[$tgt]:-}" ]; then
    emit adr-status-inconsistent "$id" "status '${STATUS[$id]}' but no ADR-$(pad "$tgt") exists"
  fi

  # --- adr-status-inconsistent arm (b): this ADR supersedes/reverses a target NOT flipped back ---
  for ref in ${SUPS[$id]} ${REVS[$id]}; do
    [ -n "${EXISTS[$ref]:-}" ] || continue              # dangling already flagged above
    back="$(status_target "${STATUS[$ref]}")"
    if [ "$back" != "$id" ]; then
      emit adr-status-inconsistent "$ref" "ADR-$(pad "$id") supersedes/reverses it but its status is '${STATUS[$ref]}'"
    fi
  done
done <<<"$SORTED_IDS"

# --- emit sorted by (check-id asc, adr-id numeric asc) ---
if [ -n "$FINDINGS" ]; then
  printf '%s' "$FINDINGS" | sort -t"$(printf '\t')" -k1,1 -k2,2n
fi

if [ "$STRICT" = 1 ] && [ -n "$FINDINGS" ]; then exit 1; fi
exit 0
```

- [ ] **Step 4: Make it executable**

Run: `chmod +x scripts/adr-checks.sh`

- [ ] **Step 5: Run the test to verify it passes** (except the Task-4 wiring sentinel)

Run: `bash tests/test_adr_checks.sh`
Expected: every assertion `ok -` EXCEPT `docket-adr Index/validate invokes adr-checks.sh` (red until Task 4).

- [ ] **Step 6: Commit**

```bash
git add scripts/adr-checks.sh tests/test_adr_checks.sh
git commit -m "feat(0030): adr-checks.sh — ADR ledger validator (gaps, dangling links, status) + tests"
```

---

### Task 3: `terminal-publish.sh` — add an `--adr <NN>` mode

**Files:**
- Modify: `scripts/terminal-publish.sh`
- Test: `tests/test_closeout.sh` (extend; keep the existing `--id` tests as the regression gate)

**Interfaces:**
- Produces: `terminal-publish.sh --adr <NN> --integration-branch B --metadata-branch M --changes-dir REL --adrs-dir REL [--message MSG] [--remote R]` — copies the single ADR file `<adrs_dir>/<NNNN>-*.md` from `origin/<metadata-branch>` onto the integration branch. `--id` and `--adr` are **mutually exclusive; exactly one is required**. `--outcome` is required only with `--id` (ADR mode has no outcome). Reuses the existing provision → copy → CAS-push → self-verify → teardown machinery and the main-mode no-op guard unchanged.
- Token: `T = <id>` (id mode, branch `pub-<id>`) or `T = adr-<NNNN>` (ADR mode, branch `pub-adr-<NNNN>`).
- ADR mode: copy-set = the single resolved ADR file; **step-1 archive skipped**; **no `Accepted` gate** (the caller decides, including a status-line flip). Default message `docket(adr-<NNNN>): publish ADR-<NNNN>`.

The change generalizes the script's `pub-$ID` references to `pub-$T` and branches the copy-set construction. Apply these exact edits:

- [ ] **Step 1: Add the failing tests to `tests/test_closeout.sh`**

Insert this block immediately **after** the existing main-mode no-op block (after the line `assert "publish: main-mode created no pub worktree" '! git -C "$W" worktree list | grep -q "pub-7"'`) and before the `cleanup-feature-branch.sh` section. `new_repo` already seeds `docs/adrs/0003-accepted.md` (Accepted) and `0005-proposed.md` (Proposed) on `docket`.

```bash
# --- terminal-publish.sh --adr: standalone Accepted ADR publishes to the integration branch ---
read -r W _ < <(new_repo)
( cd "$W" && "$PUBLISH" --adr 3 --integration-branch main --metadata-branch docket \
    --changes-dir docs/changes --adrs-dir docs/adrs ) >/dev/null 2>&1
rc=$?; git -C "$W" fetch origin main >/dev/null 2>&1
ls_main(){ git -C "$W" ls-tree -r --name-only origin/main; }
assert "publish --adr: exits 0" "[ $rc -eq 0 ]"
assert "publish --adr: ADR-0003 file landed on integration branch" 'ls_main | grep -q "docs/adrs/0003-accepted.md"'
assert "publish --adr: no change file published (archive skipped)" '! ls_main | grep -q "docs/changes/"'
assert "publish --adr: pub-adr-3 worktree torn down" '! git -C "$W" worktree list | grep -q "pub-adr-3"'

# --- terminal-publish.sh --adr: NO Accepted gate (a non-Accepted ADR still publishes) ---
read -r W _ < <(new_repo)
( cd "$W" && "$PUBLISH" --adr 5 --integration-branch main --metadata-branch docket \
    --changes-dir docs/changes --adrs-dir docs/adrs ) >/dev/null 2>&1
git -C "$W" fetch origin main >/dev/null 2>&1
assert "publish --adr: Proposed ADR-0005 still published (no gate in adr mode)" \
  'git -C "$W" ls-tree -r --name-only origin/main | grep -q "docs/adrs/0005-proposed.md"'

# --- terminal-publish.sh --adr: idempotent re-run is a no-op ---
read -r W _ < <(new_repo)
( cd "$W" && "$PUBLISH" --adr 3 --integration-branch main --metadata-branch docket --changes-dir docs/changes --adrs-dir docs/adrs ) >/dev/null 2>&1
git -C "$W" fetch origin main >/dev/null 2>&1; before="$(git -C "$W" rev-parse origin/main)"
( cd "$W" && "$PUBLISH" --adr 3 --integration-branch main --metadata-branch docket --changes-dir docs/changes --adrs-dir docs/adrs ) >/dev/null 2>&1
rc=$?; git -C "$W" fetch origin main >/dev/null 2>&1; after="$(git -C "$W" rev-parse origin/main)"
assert "publish --adr: re-run exits 0" "[ $rc -eq 0 ]"
assert "publish --adr: re-run is a no-op (no new integration commit)" '[ "$before" = "$after" ]'

# --- terminal-publish.sh --adr: main-mode no-op ---
read -r W _ < <(new_repo)
( cd "$W" && "$PUBLISH" --adr 3 --integration-branch main --metadata-branch main --changes-dir docs/changes --adrs-dir docs/adrs ) >/dev/null 2>&1
assert "publish --adr: main-mode exits 0 (no-op)" "[ $? -eq 0 ]"
assert "publish --adr: main-mode created no pub-adr worktree" '! git -C "$W" worktree list | grep -q "pub-adr-3"'

# --- terminal-publish.sh: --id and --adr are mutually exclusive; exactly one required ---
read -r W _ < <(new_repo)
( cd "$W" && "$PUBLISH" --id 7 --adr 3 --outcome done --integration-branch main --metadata-branch docket --changes-dir docs/changes --adrs-dir docs/adrs ) >/dev/null 2>&1
assert "publish: --id + --adr together is rejected (non-zero)" '[ "$?" -ne 0 ]'
( cd "$W" && "$PUBLISH" --integration-branch main --metadata-branch docket --changes-dir docs/changes --adrs-dir docs/adrs ) >/dev/null 2>&1
assert "publish: neither --id nor --adr is rejected (non-zero)" '[ "$?" -ne 0 ]'
```

Also extend the finalize wiring sentinels block (near the bottom, the `wiring(finalize):` asserts) with:

```bash
assert "wiring(finalize): ADR-only publish names terminal-publish.sh --adr" 'grep -qE "terminal-publish\.sh --adr" "$FINALIZE"'
assert "wiring(finalize): no leftover by-hand pub-adr git block" '! grep -qE "git worktree add -B .?pub-adr" "$FINALIZE"'
```

- [ ] **Step 2: Run the test to verify the new cases fail**

Run: `bash tests/test_closeout.sh`
Expected: the existing `--id` assertions stay `ok -`; the new `--adr` assertions are `NOT OK` (script rejects `--adr` as an unknown argument / `missing --id`), and the new finalize wiring sentinels fail (red until Task 4).

- [ ] **Step 3: Add `--adr` to the arg parser and validation**

In `scripts/terminal-publish.sh`, change the variable init line (currently):

```bash
ID="" OUTCOME="" INT_BRANCH="" META_BRANCH="" CHANGES_DIR="" ADRS_DIR="" MESSAGE="" REMOTE="origin"
```

to add `ADR=""`:

```bash
ID="" ADR="" OUTCOME="" INT_BRANCH="" META_BRANCH="" CHANGES_DIR="" ADRS_DIR="" MESSAGE="" REMOTE="origin"
```

Add an `--adr` case to the arg loop, right after the `--id` case:

```bash
    --id) ID="$2"; shift ;;
    --adr) ADR="$2"; shift ;;
```

Replace the current validation block (lines that read `[ -n "$ID" ] || die "missing --id"` and the `--outcome` case) with mutually-exclusive validation:

```bash
# exactly one of --id / --adr
if [ -n "$ID" ] && [ -n "$ADR" ]; then die "--id and --adr are mutually exclusive"; fi
if [ -z "$ID" ] && [ -z "$ADR" ]; then die "exactly one of --id / --adr is required"; fi
# --outcome is required (and validated) only in change (--id) mode
if [ -n "$ID" ]; then
  case "$OUTCOME" in done|killed) ;; *) die "missing/invalid --outcome" ;; esac
fi
[ -n "$INT_BRANCH" ] && [ -n "$META_BRANCH" ] || die "missing --integration-branch/--metadata-branch"
[ -n "$CHANGES_DIR" ] && [ -n "$ADRS_DIR" ]   || die "missing --changes-dir/--adrs-dir"
```

- [ ] **Step 4: Branch the token + message + copy-set construction**

Replace the block that currently begins at `pad="$(printf '%04d' "$ID")"` and runs through the copy-set construction (down to, and including, the Accepted-gate `for aid in $adr_ids; do … done` loop) with a mode branch. The generic provision/copy/CAS/self-verify section below it stays as-is **except** that every `pub-$ID` / `pub-$ID` reference and the `-B "pub-$ID"` / `branch -D "pub-$ID"` / postcondition `grep -q "pub-$ID"` are retargeted to `pub-$T` (Step 5).

New block (replaces lines ~50–86):

```bash
# --- fetch the authoritative metadata remote tip ---
$GIT fetch "$REMOTE" "$META_BRANCH" >/dev/null 2>&1 || die "fetch $REMOTE/$META_BRANCH failed"
metaref="$REMOTE/$META_BRANCH"

tmpd="$(mktemp -d)"
trap 'rm -rf "$tmpd"' EXIT

if [ -n "$ADR" ]; then
  # ----- ADR-only publish: copy-set = the single ADR file; step-1 archive skipped; no Accepted gate -----
  apad="$(printf '%04d' "$ADR")"
  T="adr-$apad"
  [ -n "$MESSAGE" ] || MESSAGE="docket(adr-$apad): publish ADR-$apad"
  adr_tree="$($GIT ls-tree -r --name-only "$metaref" -- "$ADRS_DIR")"
  apath="$(printf '%s\n' "$adr_tree" | grep -E "/$apad-[^/]*\.md$")"
  apath="${apath%%$'\n'*}"
  [ -n "$apath" ] || die "no ADR file for id $ADR on $metaref"
  copyset=("$apath")
else
  # ----- change publish: token = the id; build copy-set from the archived change manifest -----
  pad="$(printf '%04d' "$ID")"
  T="$ID"
  [ -n "$MESSAGE" ] || MESSAGE="docket($pad): publish terminal record ($OUTCOME)"
  tree="$($GIT ls-tree -r --name-only "$metaref" -- "$CHANGES_DIR/archive")"
  change_path="$(printf '%s\n' "$tree" | grep -E "/[0-9]{4}-[0-9]{2}-[0-9]{2}-$pad-[^/]*\.md$")"
  change_path="${change_path%%$'\n'*}"
  [ -n "$change_path" ] || die "no archived change file for id $ID on $metaref"
  $GIT show "$metaref:$change_path" > "$tmpd/change.md" || die "cannot read $change_path"
  spec_path="$(field "$tmpd/change.md" spec)"
  adr_ids="$(list_field "$tmpd/change.md" adrs)"
  copyset=("$change_path")
  [ -n "$spec_path" ] && copyset+=("$spec_path")
  # Accepted gate: include an ADR only if its status: is Accepted on the metadata branch
  adr_tree="$($GIT ls-tree -r --name-only "$metaref" -- "$ADRS_DIR")"
  for aid in $adr_ids; do
    apad="$(printf '%04d' "$aid")"
    apath="$(printf '%s\n' "$adr_tree" | grep -E "/$apad-[^/]*\.md$")"
    apath="${apath%%$'\n'*}"
    [ -n "$apath" ] || { log "adr $aid: file not found on $metaref; skipping"; continue; }
    $GIT show "$metaref:$apath" > "$tmpd/adr.md" || { log "adr $aid: unreadable; skipping"; continue; }
    if [ "$(field "$tmpd/adr.md" status)" = "Accepted" ]; then
      copyset+=("$apath")
    else
      log "adr $aid: not Accepted; skipped by gate"
    fi
  done
fi
```

> Note: the `tmpd` + `trap` move up from their old position (they were created mid-script after the copy-set); the old `trap 'rm -rf "$tmpd"' EXIT` and the `mktemp -d` for `tmpd` lower down must be **removed** so they are not created twice. The mode guard (`if [ "$META_BRANCH" = "$INT_BRANCH" ]; then … exit 0; fi`) must remain **above** this block, before any worktree provisioning, so main-mode stays a clean no-op for both modes.

- [ ] **Step 5: Retarget the throwaway branch token from `$ID` to `$T`**

In the provision + teardown + push + postcondition sections, replace every `pub-$ID` with `pub-$T`. The affected lines:

```bash
$GIT worktree add -B "pub-$T" "$pub" "$REMOTE/$INT_BRANCH" >/dev/null 2>&1 \
  || die "could not provision pub-$T worktree"
```

```bash
teardown(){
  $GIT -C "$pub" checkout --detach >/dev/null 2>&1
  $GIT worktree remove --force "$pub" >/dev/null 2>&1
  $GIT branch -D "pub-$T" >/dev/null 2>&1 || true
  rm -rf "$(dirname "$pub")" "$tmpd"
}
```

```bash
printf '%s\n' "$wt_list" | grep -q "pub-$T" && die "postcondition: pub-$T worktree survived"
log "published ${#copyset[@]} record(s) for $T onto $INT_BRANCH"
```

(The CAS push loop, the `diff --cached` guard, and the fail-closed self-verify over `"${copyset[@]}"` need **no** change — they already operate on `$pub` and `copyset`.)

- [ ] **Step 6: Run the full close-out suite (regression + new cases)**

Run: `bash tests/test_closeout.sh`
Expected: ALL existing `--id` assertions still `ok -` (the regression gate), and every new `--adr` / mutual-exclusion assertion `ok -`, EXCEPT the two new `wiring(finalize):` sentinels (red until Task 4).

- [ ] **Step 7: Commit**

```bash
git add scripts/terminal-publish.sh tests/test_closeout.sh
git commit -m "feat(0030): terminal-publish.sh --adr mode — single executor of both publish shapes + tests"
```

---

### Task 4: Wire the skills — retire the replaced prose

**Files:**
- Modify: `skills/docket-adr/SKILL.md`
- Modify: `skills/docket-finalize-change/SKILL.md`

This task makes the wiring sentinels added in Tasks 1–3 go green. No new test file — the sentinels live in `test_render_adr_index.sh`, `test_adr_checks.sh`, and `test_closeout.sh`.

**Constraints (keep, do not drop):** the separate-index-commit discipline; the regenerate-don't-3-way-merge rule (now literally "re-run the script"); the human-readable description of *what* the three checks cover; the `## Terminal publish (docket-mode)` heading and the documented generic mechanics (kept as the contract the script implements, parallel to how the change-publish mechanics stay documented).

- [ ] **Step 1: Rewire `docket-adr`'s *Index / validate* section**

In `skills/docket-adr/SKILL.md`, replace the *Index / validate* section body (the hand-render prose with the fenced row examples, and the "Validate the ledger and flag:" bullet list) with script invocations that keep the human-readable description. Replacement:

````markdown
### Index / validate

(Re)render `<adrs_dir>/README.md` by invoking the deterministic generator — never hand-render it:

```
scripts/render-adr-index.sh --adrs-dir <metadata tree>/<adrs_dir> > <metadata tree>/<adrs_dir>/README.md
```

In `docket`-mode the metadata tree is `.docket/`, so: `scripts/render-adr-index.sh --adrs-dir .docket/<adrs_dir> > .docket/<adrs_dir>/README.md`. It emits the index grouped into **Active**, **Superseded / Reversed**, and **Deprecated**, each row sorted by ascending id, with the `← change #N` / `→ supersedes ADR-NN` / `→ reverses ADR-NN` / `· relates to ADR-NN` annotations — offline and deterministic (same ADR files ⇒ byte-identical). Commit the regenerated index as a **separate commit** (like `BOARD.md`, so concurrent ADR creates never conflict on the shared index) and push `origin/docket`. On a git conflict on the index, **re-run the script** rather than hand-merging (the regenerate-don't-3-way-merge rule).

Validate the ledger by invoking the checker and surfacing each finding line:

```
scripts/adr-checks.sh --adrs-dir <metadata tree>/<adrs_dir>
```

It is warn-only (one TAB-separated `<check-id>\t<adr-id>\t<message>` per line; `--strict` exits 1 for a future CI gate) and covers:
- **`adr-numbering-gap`** — an id missing from the `1..max` sequence.
- **`adr-dangling-link`** — a `supersedes:` / `reverses:` / `relates_to:` value referencing an id with no corresponding file.
- **`adr-status-inconsistent`** — an ADR whose `status:` says `Superseded by ADR-NN` / `Reversed by ADR-NN` but no such ADR exists, or an ADR that `supersedes:` / `reverses:` another without the old ADR's `status:` flipped to point back.
````

- [ ] **Step 2: Rewire `docket-adr`'s two ADR-only-publish references**

In `skills/docket-adr/SKILL.md`, the *Create* (step 6) "Publish on acceptance" and *Supersede / reverse* / *Update note* sections refer to "this skill's own ADR-only terminal-publish invocation". Update those references so they name the script. In the *How an ADR reaches the integration branch* section's **Standalone ADR** and **Status change** bullets, replace the prose "runs the procedure's **ADR-only** entry (token `T = adr-<NN>`, copy-set = that single ADR file, step 1 archive is skipped)" with a concrete call:

```
scripts/terminal-publish.sh --adr <NN> --integration-branch <integration_branch> --metadata-branch <metadata_branch> --changes-dir <changes_dir> --adrs-dir <adrs_dir>
```

Keep the surrounding explanation of *why* (standalone ADR would otherwise be stranded on `docket`; a status change must re-publish because the producing change is long `done`). Note that ADR mode applies no `Accepted` gate — the caller publishes the ADR's current bytes (including a just-flipped `status:` line), which is exactly what the supersede/reverse and deprecate paths need.

- [ ] **Step 3: Rewire `docket-finalize-change`'s ADR-only publish path**

In `skills/docket-finalize-change/SKILL.md`, in *Terminal publish (docket-mode)*:
- Update the **ADR-only publish** bullet (currently: "This path is **not** handled by `terminal-publish.sh` (which is keyed on `--id`); it follows the generic provision → copy → CAS-push → teardown mechanics … run over its single-ADR copy-set") to say it **is** now handled by `scripts/terminal-publish.sh --adr <NN>`, the single executor of both publish shapes.
- Update the *The change-publish path* trailer sentence ("`terminal-publish.sh` is the **change-publish executor** of the generic mechanics below; the ADR-only variant performs those same steps by hand …") so it no longer says the ADR-only variant is by-hand — both shapes are executed by `terminal-publish.sh` (`--id` / `--adr`).
- Update *The mechanics* heading note ("the steps the **ADR-only publish** … follows by hand") to frame the section as the documented contract that `terminal-publish.sh` implements for **both** `--id` and `--adr`, no longer a by-hand runbook for the ADR path.

Do NOT delete the `## Terminal publish (docket-mode)` heading or the generic-mechanics prose — they remain the documented contract (and the `test_closeout.sh` sentinel `Terminal publish section heading preserved` must stay green). Ensure no by-hand `git worktree add -B pub-adr…` block remains (the new sentinel `no leftover by-hand pub-adr git block`).

- [ ] **Step 4: Run all three affected test files to confirm every sentinel is green**

```bash
bash tests/test_render_adr_index.sh
bash tests/test_adr_checks.sh
bash tests/test_closeout.sh
```

Expected: all three end `PASS` / exit 0 — including the wiring sentinels that were red in Tasks 1–3:
- `docket-adr Index/validate invokes render-adr-index.sh`
- `docket-adr Index/validate invokes adr-checks.sh`
- `wiring(finalize): ADR-only publish names terminal-publish.sh --adr`
- `wiring(finalize): no leftover by-hand pub-adr git block`
- and the pre-existing `wiring(finalize):` sentinels (`Terminal publish section heading preserved`, `Accepted gate still documented`, `ADR-only publish path preserved`, `no leftover raw archive bash`).

- [ ] **Step 5: Verify no other test regressed**

Run the full suite once (each file individually; there is no aggregate runner):

```bash
for t in tests/test_*.sh; do echo "== $t =="; bash "$t" >/tmp/t.out 2>&1; tail -1 /tmp/t.out; grep -c "NOT OK" /tmp/t.out | sed 's/^/NOT OK count: /'; done
```

Expected: every file's last line is `PASS` (or exits 0) and `NOT OK count: 0`. Investigate any non-zero before continuing.

- [ ] **Step 6: Commit**

```bash
git add skills/docket-adr/SKILL.md skills/docket-finalize-change/SKILL.md
git commit -m "feat(0030): wire docket-adr + docket-finalize-change to the new scripts; retire hand-run prose"
```

---

### Task 5: Smoke-test the renderer + validator against the real ADR ledger

**Files:** none (build-time verification; output captured for the results file, no commit). Per LEARNINGS #22, a green golden fixture is necessary but not sufficient — run the scripts against real data before merge.

The real ADR ledger lives on the `docket` branch in the `.docket/` metadata worktree at the repo root: `/Users/homer/dev/docket/.docket/docs/adrs/`.

- [ ] **Step 1: Smoke-run the renderer against the real ledger**

```bash
bash scripts/render-adr-index.sh --adrs-dir /Users/homer/dev/docket/.docket/docs/adrs
```

Expected: a well-formed index listing all twelve current ADRs (0001–0012) in **Active**, with `_None._` for Superseded / Reversed and Deprecated; exit 0. Confirm the `← change #N` and `· relates to ADR-NNNN` annotations render (e.g. ADR-0002 → `← change #2 · relates to ADR-0001`).

- [ ] **Step 2: Diff against the committed `docket` index to confirm the only delta is the intended drift-heal**

```bash
diff <(bash scripts/render-adr-index.sh --adrs-dir /Users/homer/dev/docket/.docket/docs/adrs) \
     /Users/homer/dev/docket/.docket/docs/adrs/README.md
```

Expected: the **only** difference is the ADR-0001 row losing its hand-added backticks around `docket` (the renderer emits the raw frontmatter title) — the intended self-healing per spec §1. Any *other* difference is a renderer bug to fix before merge. Record the diff in the results file.

- [ ] **Step 3: Smoke-run the validator against the real ledger**

```bash
bash scripts/adr-checks.sh --adrs-dir /Users/homer/dev/docket/.docket/docs/adrs; echo "exit=$?"
```

Expected: **no output, exit 0** (the real ledger 0001–0012 is contiguous, fully linked, with no supersessions yet). If any finding appears, investigate — it is either a real ledger inconsistency worth surfacing or a validator false-positive to fix.

- [ ] **Step 4: Record the smoke-test results**

Capture the Step 1–3 output (and the Step 2 diff) for the results file authored at close-out (`docket-implement-next` step 6.5). No commit in this task.

---

## Notes for the implementer

- **The new ADR is NOT authored in this plan.** Per the change spec §6 and the `docket-implement-next` procedure, the ADR recording that ADR-0012's script-vs-model boundary now extends to the `docket-adr` surface is authored at **step 6** by the `docket-adr` subagent dispatch (it assigns the number, commits on `origin/docket`, publishes onto the integration branch via the new `terminal-publish.sh --adr`, and returns the number to append to the change's `adrs:`). Do not create it as a build task.
- **Bash version:** the existing scripts assume bash ≥4 (`mapfile`, `declare -A`). Both new scripts and the empty-string-accumulation idiom (rather than empty-array expansion) keep them robust; run tests under the same bash the other `tests/*.sh` use.
- **Determinism discipline:** never introduce a timestamp, `$RANDOM`, or locale-sensitive sort into the renderer/validator — `sort -n` on integer ids only.
