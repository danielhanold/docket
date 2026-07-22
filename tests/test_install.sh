#!/usr/bin/env bash
# tests/test_install.sh — run: bash tests/test_install.sh
set -uo pipefail
unset XDG_CONFIG_HOME   # hermetic: sync-agents.sh reads ${XDG_CONFIG_HOME:-$HARNESS_ROOT/.config}; a set XDG would leak (and since 0050, MIGRATE) the dev's real global config
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
# A present harness root. link-skills.sh parent-checks <root>/.claude and creates skills/ itself;
# sync-agents.sh parent-checks <root>/.claude and creates agents/ itself.
mkdir -p "$tmp/.claude/skills"

# Run the umbrella installer against the sandbox harness, from a repo-less cwd (so sync-agents.sh's
# per-repo pass is a no-op — we are only exercising the user-level install both primitives perform).
out="$(cd "$tmp" && HOME="$tmp" DOCKET_HARNESS_ROOT="$tmp" DOCKET_TARGET_SHELL=zsh /bin/bash "$REPO/install.sh" 2>&1)"; rc=$?
assert "install.sh exits 0" '[ "$rc" = "0" ]'
assert "install.sh ran link-skills.sh (skill symlinked)" '[ -L "$tmp/.claude/skills/docket-status" ]'
assert "install.sh ran sync-agents.sh (agent generated)" '[ -f "$tmp/.claude/agents/docket-status.md" ]'
assert "install.sh injected DOCKET_SCRIPTS_DIR into the shell profile" \
  'grep -qF "export DOCKET_SCRIPTS_DIR=\"$REPO/scripts\"" "$tmp/.zshenv"'
assert "install.sh injected DOCKET_BASH_PATH into the shell profile" \
  'grep -qE "^export DOCKET_BASH_PATH=\"/.*\"$" "$tmp/.zshenv"'
assert "install.sh injected env.DOCKET_SCRIPTS_DIR into settings.json" \
  'jq -e --arg v "$REPO/scripts" ".env.DOCKET_SCRIPTS_DIR == \$v" "$tmp/.claude/settings.json" >/dev/null'
assert "install.sh injected env.DOCKET_BASH_PATH into settings.json" \
  'jq -e '\''(.env.DOCKET_BASH_PATH | type == "string" and startswith("/"))'\'' "$tmp/.claude/settings.json" >/dev/null'

# --- the skill and the scripts it invokes must resolve to ONE clone (change 0094) -------------
# docket-implement-next Step 1 runs `"$DOCKET_SCRIPTS_DIR"/docket.sh docket-status --digest-only`.
# The prose naming that flag and the script implementing it ship in the same repo, but are reached
# at runtime through two INDEPENDENT install channels: an absolute symlink into <harness>/skills/,
# and the DOCKET_SCRIPTS_DIR env var. If those ever resolved to different clones, a rewired skill
# could invoke a --digest-only the resolved scripts/ does not implement — a version-skew window in
# which Step 1 hard-errors (non-zero exit) on every run and the whole drain loop halts. Change
# 0094 argued this window cannot open, but verified it by INSPECTION only; these asserts mechanize
# it. The `-L` assert above proves only that SOME link exists, never where it points, so it cannot
# catch skew on its own.
impl_link="$tmp/.claude/skills/docket-implement-next"
assert "install.sh symlinked docket-implement-next" '[ -L "$impl_link" ]'
impl_target="$(readlink "$impl_link")"   # link-skills.sh writes ABSOLUTE targets, so no -f needed
assert "the docket-implement-next symlink points into THIS clone" \
  '[ "$impl_target" = "$REPO/skills/docket-implement-next" ]'
# Resolve a repo root from each channel INDEPENDENTLY, then require the two to agree. Deriving
# both from $REPO would assume the very thing under test, so read the scripts dir back out of the
# installed settings.json — the value a live harness actually consumes.
skill_root="$(cd "$(dirname "$impl_target")/.." && pwd -P)"
scripts_dir="$(jq -r '.env.DOCKET_SCRIPTS_DIR' "$tmp/.claude/settings.json")"
scripts_root="$(cd "$scripts_dir/.." && pwd -P)"
assert "the installed skill and DOCKET_SCRIPTS_DIR resolve to the SAME clone (no mixed-state window)" \
  '[ "$skill_root" = "$scripts_root" ]'
# Payoff: walk BOTH install channels end to end and confirm the flag the installed skill NAMES is
# implemented by the scripts/ that DOCKET_SCRIPTS_DIR actually RESOLVES to. In-repo coupling is
# already pinned by tests/test_skill_facade_wiring.sh; what this adds is that the coupling survives
# installation — the reachability claim itself, rather than a proxy for it.
assert "the installed skill names the --digest-only invocation" \
  'grep -q -- "docket-status --digest-only" "$impl_link/SKILL.md"'
assert "the scripts/ reached via DOCKET_SCRIPTS_DIR implements --digest-only" \
  'grep -q -- "--digest-only) DIGEST_ONLY=1" "$scripts_dir/docket-status.sh"'

# install.sh scaffolds the global config (ensure-global-config.sh), before sync-agents reads it.
assert "install.sh scaffolded the global config" '[ -f "$tmp/.config/docket/config.yml" ]'
# The scaffold pins no policy default: its only active value is the machine-local runtime, and it
# still points at .docket.example.yml. The unit test covers the exhaustive shape.
assert "install.sh global config contains the managed runtime block" \
  'grep -qF "# >>> docket (runtime.bash) >>>" "$tmp/.config/docket/config.yml" && grep -qE "^[[:space:]]+bash: /" "$tmp/.config/docket/config.yml"'
assert "install.sh global config points at .docket.example.yml" \
  'grep -qF ".docket.example.yml" "$tmp/.config/docket/config.yml"'

# Idempotent: a second run still succeeds.
out2="$(cd "$tmp" && HOME="$tmp" DOCKET_HARNESS_ROOT="$tmp" DOCKET_TARGET_SHELL=zsh /bin/bash "$REPO/install.sh" 2>&1)"; rc2=$?
assert "install.sh idempotent (second run exits 0)" '[ "$rc2" = "0" ]'

# A user-edited global config is NOT overwritten by a re-run.
printf '# user edit\nagent_harnesses: [claude]\n' > "$tmp/.config/docket/config.yml"
out3="$(cd "$tmp" && HOME="$tmp" DOCKET_HARNESS_ROOT="$tmp" DOCKET_TARGET_SHELL=zsh /bin/bash "$REPO/install.sh" 2>&1)"; rc3=$?
assert "install.sh re-run exits 0 with an edited global config" '[ "$rc3" = "0" ]'
assert "install.sh re-run left the user-edited global config untouched" \
  'grep -qF "# user edit" "$tmp/.config/docket/config.yml"'

# A quoted explicit runtime with an inline comment must pass through install's config read with the
# same normalization as the resolver/ensure primitive, then reach profile/settings successfully.
quoted_tmp="$(mktemp -d)"
mkdir -p "$quoted_tmp/.claude/skills" "$quoted_tmp/.config/docket"
installed_runtime="$(jq -r '.env.DOCKET_BASH_PATH' "$tmp/.claude/settings.json")"
printf 'runtime:\n  bash: "%s" # hand-authored explicit runtime\n' "$installed_runtime" > "$quoted_tmp/.config/docket/config.yml"
quoted_out="$(cd "$quoted_tmp" && HOME="$quoted_tmp" DOCKET_HARNESS_ROOT="$quoted_tmp" DOCKET_TARGET_SHELL=zsh /bin/bash "$REPO/install.sh" 2>&1)"; quoted_rc=$?
assert "install.sh accepts a quoted explicit runtime with an inline comment" '[ "$quoted_rc" -eq 0 ]'
assert "install.sh normalizes the quoted runtime before profile binding" \
  'grep -qF "export DOCKET_BASH_PATH=\"$installed_runtime\"" "$quoted_tmp/.zshenv"'
assert "install.sh normalizes the quoted runtime before settings binding" \
  'jq -e --arg v "$installed_runtime" ".env.DOCKET_BASH_PATH == \$v" "$quoted_tmp/.claude/settings.json" >/dev/null'
rm -rf "$quoted_tmp"

exit $fail
