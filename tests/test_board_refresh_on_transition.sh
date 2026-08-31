#!/usr/bin/env bash
# tests/test_board_refresh_on_transition.sh — verifies change 0004:
# BOARD.md is refreshed on every status transition, not only at Step 0.
# Extended by change 0059 (Task 3): every status-writing Board-pass caller must name the
# gated `board-refresh.sh` entry point at its Board site AND state the diff-only commit rule
# (only if BOARD.md changed), not just delegate to "docket-status's Board pass" prose.
# Run: bash tests/test_board_refresh_on_transition.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }
# A. The board-refresh invariant lives in the convention (single-sourced in docket-convention).
assert "board-refresh invariant present in the convention" \
  'grep -q "Board refresh on status writes" skills/docket-convention/SKILL.md'

# B. docket-implement-next (change 0315 + 0377 Task 10): the claim/reconcile/mark-implemented
# transitions render the inline board ATOMICALLY inside their own `docket change` transaction — no
# separate Board pass — and Task 10 migrated the reconcile-kill path onto `docket change kill`, which
# ALSO renders the board atomically in its own transaction, so NO best-effort Board pass survives
# anywhere in the skill. The board-refresh-on-transition guarantee is preserved, relocated onto the
# transaction render (learnings: restatement-accumulates-its-own-guards; assert-detects-removal-not-
# replacement), never dropped.
assert "implement-next names the atomic board render section (no separate pass)" \
  'grep -q "Atomic board rendering" skills/docket-implement-next/SKILL.md'
# Mutation twin: strip the atomic inline-board render from the transitions and this reddens.
assert "implement-next renders the inline board atomically inside its change transactions" \
  'grep -qiE "inline board[^.]{0,60}(atomic|metadata commit)|(atomic|metadata commit)[^.]{0,60}inline board" skills/docket-implement-next/SKILL.md'
# Task 10: the reconcile-kill path renders the board atomically via `docket change kill` (floor on the
# new Go verb so deleting it reddens); the retired best-effort facade pass must be GONE.
assert "implement-next renders the reconcile-kill board atomically via docket change kill" \
  'grep -qF "docket change kill" skills/docket-implement-next/SKILL.md'
assert "implement-next no longer runs a separate best-effort Board pass" \
  '! grep -q "run the Board pass (best-effort" skills/docket-implement-next/SKILL.md'

# C. docket-new-change proposed-kill (change 0377 Task 11): the kill is a `docket change kill`
# transaction that renders the inline board ATOMICALLY inside its own metadata commit — so no
# separate must-land Board pass survives. Assert the atomic-kill board render (floor on the Go verb),
# and the mutation twin — that the retired must-land facade pass is GONE.
assert "new-change proposed-kill renders the board atomically via docket change kill" \
  'grep -qF "docket change kill" skills/docket-new-change/SKILL.md'
assert "new-change no longer runs a separate must-land Board pass" \
  '! grep -q "must-land Board pass" skills/docket-new-change/SKILL.md'

# D. terminal-publish stays board-agnostic — the kill gap is fixed at the SITES, not here.
assert "terminal-publish keeps the 'BOARD.md is never published' guarantee" \
  'grep -qF "is **never** published" skills/docket-finalize-change/SKILL.md'

# --- change 0059 Task 3, NARROWED by change 0071 -----------------------------------------------
# 0059 asserted that every Board-pass caller named `board-refresh.sh` and hand-stated the
# diff-only commit rule. 0071 collapses all 8 call sites into ONE facade call
# (`docket.sh docket-status --board-only`): the orchestrator now owns the render, the diff-only
# decision, the commit, and the push, and NO surfaces value crosses the skill/script boundary.
# The prose clauses 0059 anchored on are therefore gone BY DESIGN.
#
# This guard is NARROWED, never deleted (ADR-0031: deleting a sentinel is how the guarded hole
# reopens). The property that is still load-bearing: every status-writing skill routes its board
# write through the deterministic gated pipeline at its Board site — never a hand-render, never a
# raw redirect, never a bare "docket-status will get to it eventually" delegation.
#
# The diff-only rule 0059 asserted in PROSE is now asserted where it actually executes:
# tests/test_docket_status.sh ("board_pass second (clean) run reports clean") proves the
# orchestrator does not commit an unchanged board.
RETIRED_BOARD_PASS_CALL="docket.sh docket-status --board-only"

# E. The convention still names board-refresh.sh as the gated inline writer (a NOUN mention —
# permitted by ADR-0030, and load-bearing: it is what documents the single write choke point).
assert "convention names board-refresh.sh (the gated inline writer)" \
  'grep -q "board-refresh.sh" skills/docket-convention/SKILL.md'

# E2. change 0377: the convention no longer defines a board-pass facade call — board refresh is
# ABSORBED into every board-authoritative typed mutation (the same absorption docket-finalize-change
# already carries, asserted below). The 4 still-Bash status-writing skills keep the facade call until
# Tasks 10-11 (asserted unchanged in the CALLERS loop). Guard re-pointed at the surviving substance:
# the atomic-render invariant, and the exceptional-drift repair escape hatch that replaced the pass.
assert "convention states the board renders atomically inside the typed mutation (no separate pass)" \
  'grep -qiE "atomically inside its own metadata commit|no separate Board pass" skills/docket-convention/SKILL.md'
assert "convention names the exceptional-drift repair path (check + authorized migrate)" \
  'grep -qF "docket repository check" skills/docket-convention/SKILL.md && grep -qF "docket repository migrate" skills/docket-convention/SKILL.md'

# docket-finalize-change is NOT in this facade-caller loop as of 0316 — see the dedicated
# absorption assertion after the loop. docket-implement-next left the loop as of 0377 Task 10 (its
# reconcile-kill board pass is absorbed into `docket change kill`, block B). Change 0377 Task 11
# then migrated the LAST three status-writing skills (docket-new-change, docket-groom-next,
# docket-auto-groom) onto the Go-v1 change transactions (`docket change create`/`groom`/`defer`/
# `kill`), each rendering the inline board ATOMICALLY in its own metadata commit — so NO facade
# Board pass survives in ANY skill. The facade-caller loop is therefore inverted: assert the retired
# pass is GONE from each, floored on the atomic-board invariant so it cannot go vacuously green
# (restatement-accumulates-its-own-guards; assert-detects-removal-not-replacement).
MIGRATED=(
  skills/docket-new-change/SKILL.md
  skills/docket-groom-next/SKILL.md
  skills/docket-auto-groom/SKILL.md
)

for f in "${MIGRATED[@]}"; do
  name="$(basename "$(dirname "$f")")"
  assert "$name no longer routes a Board site through the retired facade pass" \
    "! grep -qF \"\$RETIRED_BOARD_PASS_CALL\" \"$f\""
  # Mutation twin: strip the atomic-board invariant and this reddens.
  assert "$name states the board renders with no separate pass (atomic floor)" \
    "grep -qF 'no separate Board pass' \"$f\""
  # The retired shapes must be GONE: a skill that still spells a surfaces value is a skill that
  # can still send an unresolved one.
  assert "$name no longer spells a surfaces value at its Board site" \
    "! grep -qE '\-\-surfaces|BOARD_SURFACES' \"$f\""
done

# RETIRED (0316, category (a)): docket-finalize-change no longer routes a Board site through the
# `docket.sh docket-status --board-only` facade — board refresh is absorbed into every mutating Go
# transaction ("Every mutating Go transaction re-renders `BOARD.md` in the same commit as the record
# it reflects, so the board needs no separate pass"). Authority #2 (the ~12 board-render sites under
# internal/app; the skill no longer calls the facade) + Authority #3 (the skill states the board
# needs no separate pass). Guard re-pointed at the absorption and the never-published invariant
# (restored in 8c74c1c8).
FIN_BR="skills/docket-finalize-change/SKILL.md"
fin_br_flat="$(tr -s '[:space:]' ' ' < "$FIN_BR")"
assert "docket-finalize-change: board refresh is absorbed into every mutating Go transaction" \
  'grep -qiE "[Ee]very mutating .{0,4}Go .{0,4}transaction re-renders .BOARD.md|board needs no separate pass" <<<"$fin_br_flat"'
assert "docket-finalize-change keeps the 'BOARD.md is never published' invariant" \
  'grep -qiE "never.{0,4}published to the integration branch|BOARD.md.{0,20}never.{0,20}published" <<<"$fin_br_flat"'
assert "docket-finalize-change no longer spells a surfaces value at its Board site" \
  '! grep -qE "\-\-surfaces|BOARD_SURFACES" "$FIN_BR"'

exit $fail
