#!/usr/bin/env bash
# tests/test_gate_driver_boundary_mutation.sh — change 0342. Mutation proofs for the gate-driver
# architectural boundary guard (tests/test_gate_driver_boundary.sh).
#
# A guard is code: it is decoration until a mutation that defeats the thing it guards is shown to
# redden it (CLAUDE.md, ADR: "strip the thing it guards, watch it redden"). This file drives the
# REAL guard — via its `--scan-only <DIR>` mode — over crafted fixture trees, one shape at a time,
# and proves each detector reddens on the violation and stays green on the faithful control (the
# control IS the revert: the same shape without the violation must pass). Every fixture lives in its
# own tmpdir; nothing here touches the repo tree.
#
# Coverage maps to the guard's four detectors:
#   A — a raw `docket gate launch` inside a fenced recipe of a workflow skill  (+ inline-prose control)
#   B — a direct `.GateLaunch(` call in an orchestration Go path              (+ cli-layer control)
#   C — a fenced sleep/poll loop over `docket gate observe`                    (+ no-loop control)
#   D — an authored WAITING contract with its handoff requirement removed      (+ intact control)
set -uo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
GUARD="$REPO/tests/test_gate_driver_boundary.sh"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

assert "the guard under test exists and is executable-shaped" '[ -f "$GUARD" ]'

# scan <root> -> prints violations (category-tagged), exit 1 if any. Captured, never piped to a
# short-circuiting consumer (pipefail).
scan(){ bash "$GUARD" --scan-only "$1"; }

mkroot(){ mktemp -d "${TMPDIR:-/tmp}/gate-boundary-mut.XXXXXX"; }
tb='```'   # a fenced-code delimiter, kept out of this file's own prose as a literal

# --- (A) fenced raw-verb recipe in a workflow skill -----------------------------------------------
rootA="$(mkroot)"; mkdir -p "$rootA/skills/probe-skill"
{
  printf '# Probe skill\n\nRun the gate:\n\n%sbash\n' "$tb"
  printf 'docket gate launch --root /tmp/r --cwd /tmp/wt -- scripts/run-tests.sh\n'
  printf 'docket gate observe /tmp/r/run-1\n'
  printf '%s\n' "$tb"
} > "$rootA/skills/probe-skill/SKILL.md"
outA="$(scan "$rootA")"; rcA=$?
assert "A: a fenced raw-verb recipe in a skill is REJECTED" '[ "$rcA" -ne 0 ]'
assert "A: the rejection is attributed to detector A" 'grep -qE "^A"$'"'"'\t'"'"' <<<"$outA"'

# Control / revert: the SAME verb named only in inline prose is PERMITTED.
rootAc="$(mkroot)"; mkdir -p "$rootAc/skills/probe-skill"
printf '# Probe skill\n\nThe raw `docket gate launch` and `docket gate observe` verbs are primitives the driver composes; a workflow caller never runs them.\n' \
  > "$rootAc/skills/probe-skill/SKILL.md"
scan "$rootAc" >/dev/null; rcAc=$?
assert "A(revert): an inline-prose primitive mention is PERMITTED (green)" '[ "$rcAc" -eq 0 ]'

# --- (B) direct GateLaunch call in an orchestration Go path ---------------------------------------
rootB="$(mkroot)"; mkdir -p "$rootB/internal/app"
cat > "$rootB/internal/app/finalize_probe.go" <<'GO'
package app

func (s *Service) reGate(root, cwd string, argv []string) GateResult {
	return s.GateLaunch(root, cwd, argv)
}
GO
outB="$(scan "$rootB")"; rcB=$?
assert "B: a direct GateLaunch call in internal/app orchestration is REJECTED" '[ "$rcB" -ne 0 ]'
assert "B: the rejection is attributed to detector B" 'grep -qE "^B"$'"'"'\t'"'"' <<<"$outB"'

# Control / revert: the identical call inside the raw CLI adapter layer is PERMITTED.
rootBc="$(mkroot)"; mkdir -p "$rootBc/internal/cli"
cat > "$rootBc/internal/cli/probe.go" <<'GO'
package cli

func runLaunch(app *App, root, cwd string, argv []string) {
	setResult(app.GateLaunch(root, cwd, argv))
}
GO
scan "$rootBc" >/dev/null; rcBc=$?
assert "B(revert): the same call in internal/cli (raw CLI impl) is PERMITTED (green)" '[ "$rcBc" -eq 0 ]'
# and the gate seam DEFINITION file family is permitted too
rootBd="$(mkroot)"; mkdir -p "$rootBd/internal/app"
cat > "$rootBd/internal/app/gate_supervisor.go" <<'GO'
package app

func (s *Service) reGate(root, cwd string, argv []string) GateResult {
	return s.GateLaunch(root, cwd, argv)
}
GO
scan "$rootBd" >/dev/null; rcBd=$?
assert "B(revert): a call in the internal/app/gate*.go primitive family is PERMITTED (green)" '[ "$rcBd" -eq 0 ]'

# --- (C) fenced sleep/poll loop over raw observe --------------------------------------------------
rootC="$(mkroot)"; mkdir -p "$rootC/skills/probe-skill"
{
  printf '# Probe skill\n\nPoll until terminal:\n\n%sbash\n' "$tb"
  printf 'while :; do\n  state=$(docket gate observe "$run" --json | jq -r .state)\n  [ "$state" = passed ] && break\n  sleep 5\ndone\n'
  printf '%s\n' "$tb"
} > "$rootC/skills/probe-skill/SKILL.md"
outC="$(scan "$rootC")"; rcC=$?
assert "C: a fenced sleep/poll loop over raw observe is REJECTED" '[ "$rcC" -ne 0 ]'
assert "C: the rejection is attributed to detector C" 'grep -qE "^C"$'"'"'\t'"'"' <<<"$outC"'

# Control / revert: a skill that drives the gate through the driver verbs (no raw observe, no loop).
rootCc="$(mkroot)"; mkdir -p "$rootCc/skills/probe-skill"
{
  printf '# Probe skill\n\nAdvance one slice:\n\n%sbash\n' "$tb"
  printf 'docket gate drive advance --id "$drive" --owner "$gen" --json\n'
  printf '%s\n' "$tb"
} > "$rootCc/skills/probe-skill/SKILL.md"
scan "$rootCc" >/dev/null; rcCc=$?
assert "C(revert): a driver-verb (docket gate drive advance) recipe is PERMITTED (green)" '[ "$rcCc" -eq 0 ]'

# --- (D) authored WAITING contract with the handoff requirement removed ---------------------------
# Start from the REAL authored task contract, so the mutation is exactly "remove the handoff
# requirement from an authored workflow contract" and the control is the shipped file passing.
SRC="$REPO/skills/docket-build-task/SKILL.md"
assert "D: the authored task contract exists to mutate" '[ -f "$SRC" ]'

# control / revert first: the intact contract passes.
rootDc="$(mkroot)"; mkdir -p "$rootDc/skills/docket-build-task"
cp "$SRC" "$rootDc/skills/docket-build-task/SKILL.md"
scan "$rootDc" >/dev/null; rcDc=$?
assert "D(revert): the intact authored WAITING contract is PERMITTED (green)" '[ "$rcDc" -eq 0 ]'

# mutation: strip every line naming the handoff — removes both the token naming and the
# "bare wait is not a valid return" clause, while the WAITING outcome vocabulary survives.
rootD="$(mkroot)"; mkdir -p "$rootD/skills/docket-build-task"
grep -viE 'handoff' "$SRC" > "$rootD/skills/docket-build-task/SKILL.md"
# sanity: the WAITING outcome vocabulary must still be present, or the mutation is vacuous.
assert "D: the mutated contract still declares the WAITING outcome (mutation is non-vacuous)" \
  'grep -qE "WAITING" "$rootD/skills/docket-build-task/SKILL.md" && grep -qiE "COMPLETE|BLOCKED|NEEDS_ESCALATION" "$rootD/skills/docket-build-task/SKILL.md"'
outD="$(scan "$rootD")"; rcD=$?
assert "D: a WAITING contract with the handoff requirement removed is REJECTED" '[ "$rcD" -ne 0 ]'
assert "D: the rejection is attributed to detector D" 'grep -qE "^D"$'"'"'\t'"'"' <<<"$outD"'

# cleanup (best-effort; tmpdirs are self-contained)
rm -rf "$rootA" "$rootAc" "$rootB" "$rootBc" "$rootBd" "$rootC" "$rootCc" "$rootD" "$rootDc" 2>/dev/null || true

exit "$fail"
