#!/usr/bin/env bash
# tests/test_config_example.sh — run: bash tests/test_config_example.sh
# Guards config.yml.example: it exists, ships agent_harnesses [claude] active, its
# agents.claude block MIRRORS the nine shipped agents/docket-*.md defaults value-for-value
# (ADR-0039 coupling), its codex/cursor blocks ship COMMENTED (warning-free fresh copy),
# and the file resolves cleanly through the REAL resolver (sync-agents.sh) — verbatim
# (no harness warnings) and with cursor uncommented + enabled (the example IDs resolve).
set -uo pipefail
unset XDG_CONFIG_HOME
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CFGEX="$REPO/config.yml.example"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }
_tmpdirs=(); trap 'rm -rf "${_tmpdirs[@]}"' EXIT

# frontmatter scalar from a wrapper file (col-0 model:/effort:)
fm(){ sed -n "s/^$2:[[:space:]]*//p" "$1" | head -n1 | sed 's/[[:space:]]*$//'; }
# model/effort from config.yml.example's 4-space-indented claude agent line — uses the
# SAME regex sync-agents.sh's field_of() uses, so the test can't accept a shape the
# resolver rejects.
cfg_field(){ # $1=agent  $2=field(model|effort)
  local line
  line="$(grep -E "^    $1:[[:space:]]" "$CFGEX" | head -n1)"
  printf '%s' "$line" | sed -nE "s/.*[{,[:space:]]$2[[:space:]]*:[[:space:]]*([A-Za-z0-9._-]+).*/\1/p" | head -n1
}

assert "config.yml.example exists at repo root" '[ -f "$CFGEX" ]'
assert "agent_harnesses ships [claude] active" \
  'grep -Eq "^agent_harnesses:[[:space:]]*\[[[:space:]]*claude[[:space:]]*\]" "$CFGEX"'

# Defaults-match: the nine agents.claude values equal the shipped wrapper frontmatter.
# agent-key -> wrapper basename is identity with a docket- prefix.
for a in status adr brainstorm-consultant auto-groom auto-groom-critic \
         implement-next rebase-resolver integration-repair finalize-change; do
  w="$REPO/agents/docket-$a.md"
  assert "$a: wrapper exists" '[ -f "$w" ]'
  assert "$a: model mirrors wrapper" '[ -n "$(cfg_field "$a" model)" ] && [ "$(cfg_field "$a" model)" = "$(fm "$w" model)" ]'
  assert "$a: effort mirrors wrapper" '[ -n "$(cfg_field "$a" effort)" ] && [ "$(cfg_field "$a" effort)" = "$(fm "$w" effort)" ]'
done

# codex/cursor ship COMMENTED: no ACTIVE (uncommented) codex:/cursor: header line exists.
assert "codex block ships commented (no active codex: header)" '! grep -Eq "^[[:space:]]*codex:[[:space:]]*$" "$CFGEX"'
assert "cursor block ships commented (no active cursor: header)" '! grep -Eq "^[[:space:]]*cursor:[[:space:]]*$" "$CFGEX"'
# ...but the commented examples are present (so a user can find + enable them).
assert "commented codex example present" 'grep -Eq "^[[:space:]]*#[[:space:]]*codex:" "$CFGEX"'
assert "commented cursor example present" 'grep -Eq "^[[:space:]]*#[[:space:]]*cursor:" "$CFGEX"'

# Real-resolver check A: the file as-shipped generates a claude wrapper with NO harness
# warnings (the commented codex/cursor blocks are invisible to the parser).
SB="$(mktemp -d)"; _tmpdirs+=("$SB"); mkdir -p "$SB/.claude/agents" "$SB/.config/docket"
cp "$CFGEX" "$SB/.config/docket/config.yml"
err="$(cd "$SB" && HOME="$SB" DOCKET_HARNESS_ROOT="$SB" bash "$REPO/sync-agents.sh" 2>&1 >/dev/null)"; rc=$?
assert "sync-agents resolves config.yml.example verbatim (exit 0)" '[ "$rc" = "0" ]'
assert "verbatim: a claude wrapper was generated" '[ -f "$SB/.claude/agents/docket-status.md" ]'
assert "verbatim: no unknown-harness-token warning" '! printf "%s" "$err" | grep -qiE "unknown agent_harnesses token"'
assert "verbatim: generated claude status model mirrors built-in" \
  '[ "$(fm "$SB/.claude/agents/docket-status.md" model)" = "$(fm "$REPO/agents/docket-status.md" model)" ]'

# Real-resolver check B: uncomment the cursor block (it is the LAST block in the file) and
# enable cursor — the example IDs must resolve into a cursor wrapper. Proves the commented
# block is valid YAML that resolves once uncommented.
SB2="$(mktemp -d)"; _tmpdirs+=("$SB2"); mkdir -p "$SB2/.claude/agents" "$SB2/.cursor/agents" "$SB2/.config/docket"
awk '
  /^[[:space:]]*# cursor:/ { c=1 }
  c && /^  # ?/ { sub(/^  # ?/,"  ") }
  { print }
' "$CFGEX" | sed -E 's/^agent_harnesses:.*/agent_harnesses: [claude, cursor]/' \
  > "$SB2/.config/docket/config.yml"
err2="$(cd "$SB2" && HOME="$SB2" DOCKET_HARNESS_ROOT="$SB2" bash "$REPO/sync-agents.sh" 2>&1 >/dev/null)"; rc2=$?
assert "sync-agents resolves cursor-uncommented config (exit 0)" '[ "$rc2" = "0" ]'
assert "uncommented: a cursor wrapper was generated" '[ -f "$SB2/.cursor/agents/docket-status.md" ]'
assert "uncommented: cursor status model came from the example block" \
  '[ "$(fm "$SB2/.cursor/agents/docket-status.md" model)" = "grok-4.5-fast-medium" ]'

# README wires the new setup step (Deliverable 3).
README="$REPO/README.md"
assert "README has the step-2 global-config heading" 'grep -qF "### 2. Set up your global config" "$README"'
assert "README step-2 names config.yml.example" 'grep -qF "config.yml.example" "$README"'

exit $fail
