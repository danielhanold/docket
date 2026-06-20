# Verb-aware ADR status-consistency check — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `adr-checks.sh`'s `adr-status-inconsistent` arm (b) verb-aware so a right-id/wrong-verb back-pointer (e.g. `Superseded by ADR-X` where it should be `Reversed by ADR-X`) is caught instead of passing silently.

**Architecture:** Add a tolerant `status_verb` helper (sibling of the existing `status_target`), then split arm (b)'s currently-merged `${SUPS[$id]} ${REVS[$id]}` loop into a `supersedes` pass and a `reverses` pass — each knows its edge verb and flags when the target's back-pointer status carries the wrong id **or** the wrong verb. One `check-id` (`adr-status-inconsistent`), warn-only, no taxonomy change.

**Tech Stack:** Bash (`set -uo pipefail`), the repo's hand-rolled assert harness (`tests/test_adr_checks.sh`), `mkadr`/`has_finding` helpers. Offline, no network.

## Global Constraints

- Warn-only: a finding never changes exit code except under the pre-existing `--strict`. Do not add new exit-code semantics.
- Tolerant matching: reuse `status_target` for the id and the new `status_verb` for the verb — never compare the full status string (padding variance `ADR-2` vs `ADR-0002` would false-positive).
- Keep the single `check-id` `adr-status-inconsistent`; do NOT mint a new check-id.
- Behaviour for cases arm (b) already handled must not change: a correct flip stays silent; a missing/wrong-id back-pointer still fires.
- Spec: `docs/superpowers/specs/2026-06-20-adr-status-check-verb-match-design.md` (on the `docket` branch / `.docket/`).

---

## File Structure

- **Modify** `scripts/adr-checks.sh`: add `status_verb()` after `status_target()` (~line 55); replace the arm (b) `for` loop (~lines 80–87) with two verb-aware passes.
- **Modify** `tests/test_adr_checks.sh`: add two verb-mismatch fixtures + one reverses-correct control after the existing arm (b) block (~line 84).

This is a single coherent deliverable (verb-aware arm b) with one TDD cycle — one task.

---

### Task 1: Verb-aware arm (b)

**Files:**
- Modify: `scripts/adr-checks.sh` (add `status_verb`; split arm (b) loop)
- Test: `tests/test_adr_checks.sh` (add 3 assertions)

**Interfaces:**
- Consumes: existing `status_target STATUS -> id|""`, `pad`, `emit CHECK ID MSG`, the `SUPS`/`REVS`/`STATUS`/`EXISTS` assoc arrays, and the per-id loop over `$SORTED_IDS`.
- Produces: `status_verb STATUS -> "supersedes" | "reverses" | ""`; arm (b) now emits `adr-status-inconsistent` on a right-id/wrong-verb back-pointer.

- [ ] **Step 1: Write the failing tests**

In `tests/test_adr_checks.sh`, immediately after the existing arm (b) block (the line `rm -rf "$d" "$d2"` at ~line 84), add:

```bash
# ===== adr-status-inconsistent arm (b) verb-mismatch: right id, WRONG verb =====
# ADR-2 supersedes [1] but ADR-1's status uses the REVERSED verb -> must flag ADR-1.
d="$(mktemp -d)"
mkadr "$d" 1 "Reversed by ADR-0002" "[]" "[]" "[]"
mkadr "$d" 2 Accepted "[1]" "[]" "[]"
out="$(bash "$SCRIPT" --adrs-dir "$d" 2>/dev/null)"
assert "status-inconsistent (b) flagged on supersedes-edge w/ reversed-verb target" 'has_finding "$out" adr-status-inconsistent 1'
rm -rf "$d"
# ADR-2 reverses [1] but ADR-1's status uses the SUPERSEDED verb -> must flag ADR-1.
d="$(mktemp -d)"
mkadr "$d" 1 "Superseded by ADR-0002" "[]" "[]" "[]"
mkadr "$d" 2 Accepted "[]" "[1]" "[]"
out="$(bash "$SCRIPT" --adrs-dir "$d" 2>/dev/null)"
assert "status-inconsistent (b) flagged on reverses-edge w/ superseded-verb target" 'has_finding "$out" adr-status-inconsistent 1'
rm -rf "$d"
# control: reverses edge with the CORRECT verb on target -> silent.
d="$(mktemp -d)"
mkadr "$d" 1 "Reversed by ADR-0002" "[]" "[]" "[]"
mkadr "$d" 2 Accepted "[]" "[1]" "[]"
out="$(bash "$SCRIPT" --adrs-dir "$d" 2>/dev/null)"
assert "status-inconsistent (b) silent on reverses edge correctly flipped" '! has_finding "$out" adr-status-inconsistent 1'
rm -rf "$d"
```

- [ ] **Step 2: Run the suite to verify the two new wrong-verb assertions FAIL**

Run: `bash tests/test_adr_checks.sh`
Expected: the two `NOT OK - status-inconsistent (b) flagged on …-verb target` lines appear and the script ends `FAIL` (exit 1). The reverses-correct control already passes under the old code (old arm (b) is verb-blind, so it is silent there too) — that is the no-regression guard.

- [ ] **Step 3: Add the `status_verb` helper**

In `scripts/adr-checks.sh`, immediately after the closing `}` of `status_target()` (~line 55), add:

```bash
# status_verb STATUS -> "supersedes" | "reverses" | "" — the edge a back-pointer status implies
status_verb(){
  case "$1" in
    "Superseded by ADR-"*) printf 'supersedes' ;;
    "Reversed by ADR-"*)   printf 'reverses' ;;
    *) printf '' ;;
  esac
}
```

- [ ] **Step 4: Split arm (b) into verb-aware passes**

In `scripts/adr-checks.sh`, replace the entire arm (b) block:

```bash
  # --- adr-status-inconsistent arm (b): this ADR supersedes/reverses a target NOT flipped back ---
  for ref in ${SUPS[$id]} ${REVS[$id]}; do
    [ -n "${EXISTS[$ref]:-}" ] || continue              # dangling already flagged above
    back="$(status_target "${STATUS[$ref]}")"
    if [ "$back" != "$id" ]; then
      emit adr-status-inconsistent "$ref" "ADR-$(pad "$id") supersedes/reverses it but its status is '${STATUS[$ref]}'"
    fi
  done
```

with:

```bash
  # --- adr-status-inconsistent arm (b): supersedes/reverses target NOT flipped back (verb-aware) ---
  # a supersedes edge requires the target status 'Superseded by ADR-X'; a reverses edge requires
  # 'Reversed by ADR-X'. Right id but wrong verb is a finding, not a silent pass.
  for ref in ${SUPS[$id]}; do
    [ -n "${EXISTS[$ref]:-}" ] || continue              # dangling already flagged above
    back="$(status_target "${STATUS[$ref]}")"
    verb="$(status_verb "${STATUS[$ref]}")"
    if [ "$back" != "$id" ] || [ "$verb" != supersedes ]; then
      emit adr-status-inconsistent "$ref" "ADR-$(pad "$id") supersedes it but its status is '${STATUS[$ref]}' (expected 'Superseded by ADR-$(pad "$id")')"
    fi
  done
  for ref in ${REVS[$id]}; do
    [ -n "${EXISTS[$ref]:-}" ] || continue              # dangling already flagged above
    back="$(status_target "${STATUS[$ref]}")"
    verb="$(status_verb "${STATUS[$ref]}")"
    if [ "$back" != "$id" ] || [ "$verb" != reverses ]; then
      emit adr-status-inconsistent "$ref" "ADR-$(pad "$id") reverses it but its status is '${STATUS[$ref]}' (expected 'Reversed by ADR-$(pad "$id")')"
    fi
  done
```

- [ ] **Step 5: Run the full suite to verify all pass**

Run: `bash tests/test_adr_checks.sh`
Expected: every line `ok - …` and a final `PASS` (exit 0). Confirm specifically that the two new wrong-verb assertions, the reverses-correct control, AND the pre-existing arm-(b) flagged/silent assertions (lines 77, 83) all pass.

- [ ] **Step 6: Run the whole shell test suite (no collateral regressions)**

Run: `for t in tests/test_*.sh; do echo "== $t =="; bash "$t" | tail -1; done`
Expected: every test file ends `PASS`. (Confirms the `adr-checks.sh` edit didn't disturb any sibling.)

- [ ] **Step 7: Commit**

```bash
git add scripts/adr-checks.sh tests/test_adr_checks.sh
git commit -m "feat(0031): verb-aware adr-status-inconsistent arm (b)

Split the merged supersedes/reverses loop into per-verb passes and add a
tolerant status_verb helper, so a right-id/wrong-verb back-pointer is flagged
instead of passing silently. Same check-id, warn-only. Tests cover both edge
verbs' mismatch + a reverses-correct control."
```

---

## Self-Review

**1. Spec coverage:** `status_verb` helper ✓ (Step 3); split arm (b), verb+id check, enriched message naming the expected back-pointer ✓ (Step 4); tolerant matching (id via `status_target`, verb via `status_verb`) ✓; one check-id ✓; warn-only ✓ (no exit-code edits); tests — supersedes-wrong-verb ✓, reverses-wrong-verb ✓, reverses-correct control ✓, existing controls preserved ✓ (Step 5/6). All spec items mapped.

**2. Placeholder scan:** No TBD/TODO/"handle edge cases"; every code step shows full code and exact commands. Clean.

**3. Type consistency:** `status_verb` returns exactly `supersedes`/`reverses`/`""`, matched verbatim in the `[ "$verb" != supersedes ]` / `[ "$verb" != reverses ]` guards; `status_target` reused unchanged; `emit`/`pad`/array names match the existing script. Consistent.
