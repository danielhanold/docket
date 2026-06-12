#!/usr/bin/env bash
# tests/test_docket_metadata_branch.sh — verifies docket-mode (the metadata-branch change, 0002).
# Run: bash tests/test_docket_metadata_branch.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }
SKILLS=(docket-new-change docket-status docket-implement-next docket-finalize-change docket-adr docket-groom-next)

# A. metadata_branch default flipped to docket, in the convention (single-sourced in docket-convention).
assert "metadata_branch default is docket in the convention" \
  'grep -Eq "^metadata_branch: docket" skills/docket-convention/SKILL.md'

# B. integration_branch vocabulary propagated into every skill (the knob itself is guarded in docket-convention below).
for s in "${SKILLS[@]}"; do
  assert "integration_branch knob present in $s" \
    'grep -q "integration_branch" "skills/'"$s"'/SKILL.md"'
done
assert "integration_branch knob present in the convention" \
  'grep -q "integration_branch" skills/docket-convention/SKILL.md'

# C. The "metadata working tree" abstraction appears in every skill.
for s in "${SKILLS[@]}"; do
  assert "metadata working tree wording in $s" \
    'grep -qi "metadata working tree" "skills/'"$s"'/SKILL.md"'
done

# D. Branch-model: feature branch cut from the integration branch (not hard-coded main).
assert "branch-model generalized to integration_branch" \
  'grep -q "origin/<integration_branch>" "skills/docket-new-change/SKILL.md"'

# E. Bootstrap guard (refuse-to-migrate) present in the convention (single-sourced in docket-convention).
assert "bootstrap guard present in convention" \
  'grep -qiE "half-migrated|bootstrap guard|migrate-to-docket" "skills/docket-convention/SKILL.md"'

# F. The v1 docket caveat is REMOVED from docket-implement-next.
assert "v1 docket caveat removed from implement-next" \
  '! grep -qi "v1 rough edge" skills/docket-implement-next/SKILL.md'

# G. Terminal-publish: single-sourced in finalize; copies from origin/docket; Accepted gate.
assert "terminal-publish procedure in finalize" \
  'grep -qi "terminal publish\|terminal-publish" skills/docket-finalize-change/SKILL.md'
assert "publish copies from origin/docket (not a branch merge)" \
  'grep -q "checkout origin/docket" skills/docket-finalize-change/SKILL.md'
assert "Accepted gate on ADR publish (copy-site gate, not the ADR schema)" \
  'grep -q "whose ADR is \`Accepted\`" skills/docket-finalize-change/SKILL.md'

# H. Kill-publish wired in BOTH kill origins (producer + implementer), not just finalize.
assert "proposed-kill wired in docket-new-change" \
  'grep -qi "kill" skills/docket-new-change/SKILL.md && grep -qi "terminal.publish\|terminal-publish" skills/docket-new-change/SKILL.md'
assert "reconcile-kill wired in docket-implement-next" \
  'grep -qi "kill" skills/docket-implement-next/SKILL.md && grep -qi "terminal.publish\|terminal-publish" skills/docket-implement-next/SKILL.md'

# I. docket-status: sweep invokes terminal-publish.
assert "status sweep invokes terminal-publish" \
  'grep -qi "terminal.publish\|terminal-publish" skills/docket-status/SKILL.md'

# J. docket-adr: Accepted ADRs publish.
assert "adr skill references terminal-publish / publish" \
  'grep -qi "terminal.publish\|terminal-publish\|publish" skills/docket-adr/SKILL.md'

# K. main-mode backward-compat: the degradation is documented at each docket-mode
#    mechanic site (spec §7.6/§12). These assertions FAIL if a degradation clause is
#    deleted — unlike a bare "main-mode" grep, which any unrelated mention satisfies.
# K1. Convention documents the pinned main-mode opt-out (non-vacuous: the exact opt-out prose).
assert "main-mode opt-out documented in convention" \
  'grep -q "pinning \`metadata_branch: main\`" "skills/docket-convention/SKILL.md"'
# K2. Terminal-publish is explicitly skipped in main-mode (degradation at the publish site).
assert "terminal-publish skipped entirely in main-mode" \
  'grep -q "Skipped entirely in \`main\`-mode" skills/docket-finalize-change/SKILL.md'
# K3. Proposed-kill (docket-new-change) carries its main-mode archive-move degradation clause.
assert "proposed-kill degrades to a direct archive move in main-mode" \
  'grep -q "no \`docket\` branch / no terminal-publish): do the archive move" skills/docket-new-change/SKILL.md'
# K4. Reconcile-kill (docket-implement-next) carries its main-mode archive-move degradation clause.
assert "reconcile-kill degrades to a direct archive move in main-mode" \
  'grep -q "no \`docket\` branch / no terminal-publish): do the archive move" skills/docket-implement-next/SKILL.md'

# L. .gitignore ignores the metadata worktree + feature worktrees.
assert ".gitignore ignores .docket/" 'grep -qE "^\.docket/?" .gitignore'
assert ".gitignore ignores .worktrees/" 'grep -qE "^\.worktrees/?" .gitignore'

# M. migrate-to-docket.sh exists, executable, creates orphan + prunes.
assert "migrate-to-docket.sh exists" '[ -f migrate-to-docket.sh ]'
assert "migrate-to-docket.sh is executable" '[ -x migrate-to-docket.sh ]'
assert "migration creates an orphan docket branch" \
  'grep -q "checkout --orphan docket\|worktree add --orphan" migrate-to-docket.sh'
assert "migration prunes the live surface" \
  'grep -qi "active\|BOARD.md" migrate-to-docket.sh'

# N. README documents docket-mode + integration_branch + artifact locations.
assert "README documents metadata_branch: docket default" 'grep -q "metadata_branch: docket" README.md'
assert "README documents integration_branch" 'grep -q "integration_branch" README.md'
assert "README has docket-mode / artifact-location content" \
  'grep -qiE "docket-mode|artifact|lives on" README.md'

# O. Existing conventions preserved (no regression of the 0001 results work).
assert "results: field still present (no regression)" 'grep -q "^results:" skills/docket-convention/SKILL.md'

# P. migrate-to-docket.sh targets $PWD's repo (not its own SCRIPT_DIR) + has a --yes bypass (change 0003).
assert "migrate resolves target via git rev-parse --show-toplevel" \
  'grep -q "rev-parse --show-toplevel" migrate-to-docket.sh'
assert "migrate no longer cd's to SCRIPT_DIR" \
  '! grep -q "cd \"\$SCRIPT_DIR\"" migrate-to-docket.sh'
assert "migrate has a --yes/-y confirmation bypass" \
  'grep -qE "\-\-yes\b|\b-y\b" migrate-to-docket.sh'
assert "migrate prompts for confirmation (reads /dev/tty)" \
  'grep -q "/dev/tty" migrate-to-docket.sh'

exit $fail
