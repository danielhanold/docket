#!/usr/bin/env bash
# tests/test_docket_metadata_branch.sh — verifies docket-mode (the metadata-branch change, 0002).
# Run: bash tests/test_docket_metadata_branch.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }
SKILLS=(docket-new-change docket-status docket-implement-next docket-finalize-change docket-adr docket-groom-next)

# A. metadata_branch default flipped to docket, in the convention (single-sourced in docket-convention).
assert "metadata_branch default is docket in the convention" \
  'grep -Eq "^metadata_branch: docket" skills/docket-convention/SKILL.md'

# B. integration_branch vocabulary propagated into every skill (the knob itself is guarded in docket-convention below).
# RE-KEYED (0316, category (c)): the assertion guards VOCABULARY propagation, not the raw config
# knob (which the convention guards below). The Go-sequencer docket-finalize-change reads the base
# through `docket context finalize`/preflight and states the abstraction as the spaced prose
# "integration branch" rather than the underscored config literal. Key on the vocabulary in either
# separator so the propagation guard survives that surface change without weakening for the skills
# that still name the literal.
for s in "${SKILLS[@]}"; do
  assert "integration_branch vocabulary present in $s" \
    'grep -qiE "integration.branch" "skills/'"$s"'/SKILL.md"'
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

# G. Terminal-publish is DEFERRED for the finalize Go sequencer (0316, Out of scope).
# RETIRED (0316, category (a)): the finalize skill used to single-source the terminal-publish
# procedure (copy from `origin/docket`, the Accepted-ADR copy-site gate, the main-mode skip). 0316's
# *Out of scope* defers "terminal publishing", so the Go-sequencer finalize carries none of it — the
# still-bash sweep (docket-status, asserted in section I below) owns terminal publishing until a
# later change. Authority #1 (Out of scope: terminal publishing). Inverted guards proving finalize
# carries no terminal-publish procedure, with a non-vacuity anchor so an empty/renamed file cannot
# pass.
assert "finalize SKILL is the Go sequencer (non-vacuity anchor)" \
  'grep -qF "docket finalize" skills/docket-finalize-change/SKILL.md'
assert "finalize carries no deferred terminal-publish procedure" \
  '! grep -qi "terminal publish\|terminal-publish" skills/docket-finalize-change/SKILL.md'
assert "finalize does not copy from origin/docket by hand (Go closeout owns backlinks)" \
  '! grep -q "checkout origin/docket" skills/docket-finalize-change/SKILL.md'

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
# K2. RETIRED (0316, category (a)): terminal publishing is deferred (Out of scope), so there is no
# publish site in the finalize sequencer to carry a main-mode skip clause. Finalize's own main-mode
# closeout behavior is Go-owned (the `docket` mode backlink leg in step 9). Inverted guard proving
# the deferred publish-site skip clause is absent, anchored non-vacuously by section G above.
assert "finalize carries no deferred main-mode terminal-publish skip clause" \
  '! grep -q "Skipped entirely in \`main\`-mode" skills/docket-finalize-change/SKILL.md'
# K3. Proposed-kill (docket-new-change) delegates to the close-out reference, which carries the
#     main-mode archive-move degradation clause (moved out of the skill by the kill-path rewire).
assert "close-out ref documents main-mode archive-move degradation (proposed-kill)" \
  'grep -q "archive commit is itself the terminal record" skills/docket-convention/references/terminal-close-out.md'
assert "docket-new-change points proposed-kill at the close-out reference" \
  'grep -qF "terminal-close-out.md" skills/docket-new-change/SKILL.md'
# K4. Reconcile-kill (docket-implement-next) delegates to the close-out reference, which carries the
#     main-mode archive-move degradation clause (moved out of the skill by the kill-path rewire).
assert "close-out ref documents main-mode archive-move degradation (reconcile-kill)" \
  'grep -q "archive commit is itself the terminal record" skills/docket-convention/references/terminal-close-out.md'
assert "docket-implement-next points reconcile-kill at the close-out reference" \
  'grep -qF "terminal-close-out.md" skills/docket-implement-next/SKILL.md'

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
  'grep -qE -e "--yes\b|\b-y\b" migrate-to-docket.sh'
assert "migrate prompts for confirmation (reads /dev/tty)" \
  'grep -q "/dev/tty" migrate-to-docket.sh'

# --- change 0083: the `## Publish deferred` marker is registered in the convention -------------
CONV="$REPO/skills/docket-convention/SKILL.md"
TCO="$REPO/skills/docket-convention/references/terminal-close-out.md"
conv_lines="$(grep -F -- "## Publish deferred" "$CONV")"
tco_lines="$(grep -iE "publish deferred|marker" "$TCO")"
assert "convention's body-section list documents ## Publish deferred" \
  '[ -n "$conv_lines" ]'
assert "convention names the marker's REMOVAL on a successful publish" \
  'grep -qiE "remov|clear" <<<"$conv_lines"'
assert "close-out step 3 documents the write-marker-on-defer rule" \
  'grep -qF -- "## Publish deferred" "$TCO"'
assert "close-out states the marker is NEVER written under suppression" \
  'grep -qiE "never|not written|suppress" <<<"$tco_lines"'

exit $fail
