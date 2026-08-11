#!/usr/bin/env bash
# tests/test_sync_agents_validator.sh — the change-0173 generation validator (shard of test_sync_agents.sh,
# change 0227). Run: bash tests/test_sync_agents_validator.sh
# shellcheck source=lib/sync_agents_common.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/sync_agents_common.sh"

# ---- 0173: the validator — unconsumable values fail generation, loudly, before any write ----
# Posture is deliberately asymmetric with runner-dispatch.sh: here a human is reading output and a
# wrong pin PERSISTS in a generated file, so generation aborts. Partial generation carrying a
# known-bad pin is precisely the harm this change exists to prevent.
#
# Every grep below reads a herestring, never `printf … | grep -q`: the suite runs under
# `set -o pipefail`, and an early-exiting consumer SIGPIPEs its producer into an intermittent 141
# (AGENTS.md, "Shell"). Same asserts, no race.
SQ173="'"   # a literal single quote, so the diagnostic's `'model'` quoting can be asserted verbatim

# -- a space-bearing value: non-zero exit, named diagnostic, and NO wrapper written --
make_sandbox
HROOT173V="$(mktemp -d)"; mkdir -p "$HROOT173V/.claude"
printf 'agents:\n  default:\n    status: { model: two words, effort: high }\n' > "$SBX/.docket.yml"
v_err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT173V" bash "$SYNC" 2>&1 >/dev/null )"; v_rc=$?
assert "0173 validator: space-bearing value exits non-zero" '[ "$v_rc" != "0" ]'
assert "0173 validator: diagnostic names the harness/agent" '/usr/bin/grep -qF "default/status" <<<"$v_err"'
assert "0173 validator: diagnostic names the key"           '/usr/bin/grep -qF "${SQ173}model${SQ173}" <<<"$v_err"'
assert "0173 validator: diagnostic quotes the RAW value"    '/usr/bin/grep -qF "two words" <<<"$v_err"'
assert "0173 validator: diagnostic names what was CONSUMED" '/usr/bin/grep -qF "consumes only" <<<"$v_err"'
assert "0173 validator: says not a bare scalar"             '/usr/bin/grep -qF "is not a bare scalar" <<<"$v_err"'
assert "0173 validator: diagnostic names the layer file"    '/usr/bin/grep -qF ".docket.yml" <<<"$v_err"'
# The whole point of validating BEFORE the write: no half-regenerated agent dir.
assert "0173 validator: NO wrapper file was written" '[ ! -e "$SBX/.claude/agents/docket-status.md" ]'
assert "0173 validator: no agents dir created at all" '[ ! -d "$SBX/.claude/agents" ]'
rm -rf "$SBX" "$HROOT173V"

# -- a quoted value: same posture. `"claude-opus-5"` has consumed == raw, so the raw/consumed
#    comparison alone CANNOT see it — this assert is what pins the explicit quote leg. --
make_sandbox
HROOT173Q="$(mktemp -d)"; mkdir -p "$HROOT173Q/.claude"
printf 'agents:\n  default:\n    status: { model: "claude-opus-5", effort: high }\n' > "$SBX/.docket.yml"
q_err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT173Q" bash "$SYNC" 2>&1 >/dev/null )"; q_rc=$?
assert "0173 validator: quoted value exits non-zero" '[ "$q_rc" != "0" ]'
assert "0173 validator: quoted diagnostic names the remedy" '/usr/bin/grep -qF "unquoted" <<<"$q_err"'
assert "0173 validator: quoted value writes no wrapper" '[ ! -e "$SBX/.claude/agents/docket-status.md" ]'
rm -rf "$SBX" "$HROOT173Q"

# -- a SINGLE-quoted value is caught the same way (the remedy says "unquoted", not "double-quoted") --
make_sandbox
HROOT173S="$(mktemp -d)"; mkdir -p "$HROOT173S/.claude"
printf "agents:\n  default:\n    status: { model: 'claude-opus-5', effort: high }\n" > "$SBX/.docket.yml"
s_err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT173S" bash "$SYNC" 2>&1 >/dev/null )"; s_rc=$?
assert "0173 validator: single-quoted value exits non-zero" '[ "$s_rc" != "0" ]'
assert "0173 validator: single-quoted value writes no wrapper" '[ ! -e "$SBX/.claude/agents/docket-status.md" ]'
rm -rf "$SBX" "$HROOT173S"

# -- 0255: a `#` INSIDE the flow map. harness_agent_line strips comments on BOTH bash paths before
#    either reader runs, so `{ model: c#5 }` truncates to `c` and every value-comparison leg agrees
#    it is fine — the silent truncation this validator family exists to close, in the one corner the
#    value legs structurally cannot see. Generation must abort before any wrapper is written. --
make_sandbox
HROOT255H="$(mktemp -d)"; mkdir -p "$HROOT255H/.claude"
printf 'agents:\n  default:\n    status: { model: c#5, effort: high }\n' > "$SBX/.docket.yml"
h_err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT255H" bash "$SYNC" 2>&1 >/dev/null )"; h_rc=$?
assert "0255 validator: '#' inside the flow map exits non-zero" '[ "$h_rc" != "0" ]'
assert "0255 validator: '#' diagnostic names the harness/agent" '/usr/bin/grep -qF "default/status" <<<"$h_err"'
assert "0255 validator: '#' diagnostic names the flow map" '/usr/bin/grep -qF "inside the flow map" <<<"$h_err"'
assert "0255 validator: '#' diagnostic names the layer file" '/usr/bin/grep -qF ".docket.yml" <<<"$h_err"'
# A diagnostic that blames the wrong cause is the defect the split messages here exist to prevent:
# "write it unquoted" is not the remedy for a truncating `#`.
assert "0255 validator: '#' diagnostic does not blame quoting" \
  '! /usr/bin/grep -qF "unquoted and space-free" <<<"$h_err"'
assert "0255 validator: '#' value writes no wrapper" '[ ! -e "$SBX/.claude/agents/docket-status.md" ]'
rm -rf "$SBX" "$HROOT255H"

# -- the carve-outs. Over-rejecting either of these would hard-abort generation on the documented,
#    legitimate comment styles used throughout .docket.example.yml and agent-layer.md's own example. --
make_sandbox
HROOT255T="$(mktemp -d)"; mkdir -p "$HROOT255T/.claude"
printf 'agents:\n  default:\n    status: { model: claude-opus-5, effort: high }   # trailing note\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT255T" bash "$SYNC" >/dev/null 2>&1 ); t_rc=$?
assert "0255 validator: a trailing comment AFTER the brace still generates (rc=0)" '[ "$t_rc" = "0" ]'
assert "0255 validator: and the trailing-comment wrapper IS written" \
  '[ "$(fm_anchored "$SBX/.claude/agents/docket-status.md" model)" = "claude-opus-5" ]'
rm -rf "$SBX" "$HROOT255T"

# A commented-out map is field-absent post-strip, which is LEGAL in user config (every field is
# optional) — and it is the natural workaround for this very gate, so it must not fire.
make_sandbox
HROOT255C="$(mktemp -d)"; mkdir -p "$HROOT255C/.claude"
printf 'agents:\n  default:\n    status: # { model: c#5, effort: high }\n    adr: { model: claude-opus-5, effort: low }\n' > "$SBX/.docket.yml"
c_err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT255C" bash "$SYNC" 2>&1 >/dev/null )"; c_rc=$?
assert "0255 validator: a commented-out map still generates (rc=0)" '[ "$c_rc" = "0" ]'
assert "0255 validator: and it fires no flow-map complaint" \
  '! /usr/bin/grep -qF "inside the flow map" <<<"$c_err"'
assert "0255 validator: and the sibling entry still resolves" \
  '[ "$(fm_anchored "$SBX/.claude/agents/docket-adr.md" model)" = "claude-opus-5" ]'
rm -rf "$SBX" "$HROOT255C"

# -- a genuinely MISSING value is a DIFFERENT diagnostic. Without this distinction a clip that
#    lands empty makes the error blame ABSENCE for what is really a quoting problem. --
make_sandbox
HROOT173M="$(mktemp -d)"; mkdir -p "$HROOT173M/.claude"
printf 'agents:\n  default:\n    status: { model: , effort: high }\n' > "$SBX/.docket.yml"
m_err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT173M" bash "$SYNC" 2>&1 >/dev/null )"; m_rc=$?
assert "0173 validator: empty value exits non-zero" '[ "$m_rc" != "0" ]'
assert "0173 validator: empty value uses the MISSING diagnostic" '/usr/bin/grep -qF "has no value" <<<"$m_err"'
assert "0173 validator: empty value does NOT claim not-a-bare-scalar" \
  '! /usr/bin/grep -qF "is not a bare scalar" <<<"$m_err"'
rm -rf "$SBX" "$HROOT173M"

# -- every offender is reported, not just the first (collect-then-fail) --
make_sandbox
HROOT173A="$(mktemp -d)"; mkdir -p "$HROOT173A/.claude"
printf 'agents:\n  default:\n    status: { model: two words }\n    adr: { model: three more words }\n' > "$SBX/.docket.yml"
a_err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT173A" bash "$SYNC" 2>&1 >/dev/null )"
assert "0173 validator: reports the first offender"  '/usr/bin/grep -qF "default/status" <<<"$a_err"'
assert "0173 validator: reports the second offender too" '/usr/bin/grep -qF "default/adr" <<<"$a_err"'
rm -rf "$SBX" "$HROOT173A"

# -- every LAYER is walked, not just the committed one (local + global each reach the gate) --
make_sandbox
HROOT173L="$(mktemp -d)"; mkdir -p "$HROOT173L/.claude"
printf 'agents:\n  default:\n    status: { model: local words }\n' > "$SBX/.docket.local.yml"
l_err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT173L" bash "$SYNC" 2>&1 >/dev/null )"; l_rc=$?
assert "0173 validator: machine-local layer is validated too" '[ "$l_rc" != "0" ]'
assert "0173 validator: local-layer diagnostic names .docket.local.yml" \
  '/usr/bin/grep -qF ".docket.local.yml" <<<"$l_err"'
rm -rf "$SBX" "$HROOT173L"

make_sandbox
mkdir -p "$SBX/.config/docket"
printf 'agents:\n  default:\n    status: { model: global words }\n' > "$SBX/.config/docket/config.yml"
g_err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" 2>&1 >/dev/null )"; g_rc=$?
assert "0173 validator: global layer is validated too" '[ "$g_rc" != "0" ]'
assert "0173 validator: global-layer diagnostic names config.yml" \
  '/usr/bin/grep -qF "config.yml" <<<"$g_err"'
assert "0173 validator: global offender writes no user-level wrapper" \
  '[ ! -e "$SBX/.claude/agents/docket-status.md" ]'
rm -rf "$SBX"

# -- the non-model keys are gated too --
make_sandbox
HROOT173F="$(mktemp -d)"; mkdir -p "$HROOT173F/.claude"
printf 'agents:\n  default:\n    status: { model: sonnet, effort: "high" }\n' > "$SBX/.docket.yml"
f_err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT173F" bash "$SYNC" 2>&1 >/dev/null )"; f_rc=$?
assert "0173 validator: a quoted effort is an offender too" '[ "$f_rc" != "0" ]'
assert "0173 validator: effort offender names the effort key" \
  '/usr/bin/grep -qF "${SQ173}effort${SQ173}" <<<"$f_err"'
rm -rf "$SBX" "$HROOT173F"

# -- --check validates too: CI must not pass against config a real run would refuse --
# NOTE on the rc assert: --check ALSO exits 1 in this fixture for unrelated drift (no wrappers
# generated, no .gitignore block), so `k_rc != 0` is green even before this task — probed. The two
# message asserts below are the load-bearing ones; they are what actually go red without the gate.
make_sandbox
HROOT173K="$(mktemp -d)"; mkdir -p "$HROOT173K/.claude"
printf 'agents:\n  default:\n    status: { model: two words }\n' > "$SBX/.docket.yml"
k_out="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT173K" bash "$SYNC" --check 2>&1 )"; k_rc=$?
assert "0173 validator: --check fails on an unconsumable value" '[ "$k_rc" != "0" ]'
assert "0173 validator: --check names the offending value, not just generic drift" \
  '/usr/bin/grep -qF "is not a bare scalar" <<<"$k_out"'
assert "0173 validator: --check refuses via the user-config gate" \
  '/usr/bin/grep -qF "user agent config has unconsumable values" <<<"$k_out"'
rm -rf "$SBX" "$HROOT173K"

# -- a CLEAN provider-prefixed config passes the validator (it must not over-reject) --
make_sandbox
HROOT173P="$(mktemp -d)"; mkdir -p "$HROOT173P/.claude"
printf 'agents:\n  default:\n    status: { model: anthropic/claude-opus-5, effort: high }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT173P" bash "$SYNC" >/dev/null 2>&1 ); p_rc=$?
assert "0173 validator: clean provider-prefixed config still generates (rc=0)" '[ "$p_rc" = "0" ]'
assert "0173 validator: and the wrapper IS written" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
rm -rf "$SBX" "$HROOT173P"

# -- over-rejection floor: a REALISTIC multi-harness config, aligned-column style, with an entry
#    that omits model, one that omits effort, a runner:, a trailing comment, and a tab-indented
#    layer. All of it is legal today and must stay legal. --
make_sandbox
mkdir -p "$SBX/.cursor" "$SBX/.config/docket"
printf 'agents:\n  default:\n    adr: { model: claude-opus-5, effort: low }\n' > "$SBX/.config/docket/config.yml"
printf 'agents:\n\tdefault:\n\t\tauto-groom: { model: tab-m }\n' > "$SBX/.docket.local.yml"
{
  printf 'agent_harnesses: [claude, cursor]\n'
  printf 'agents:\n'
  printf '  default:\n'
  printf '    status:         { model: claude-haiku-4-5-20251001, effort: medium }   # aligned + commented\n'
  printf '    implement-next: { effort: auto }\n'
  printf '    finalize-change: { model: claude-opus-5 }\n'
  printf '  cursor:\n'
  printf '    status:         { model: cursor-grok-4.5-low-fast,  effort: auto }\n'
  printf '  claude:\n'
  printf '    integration-repair: { model: gpt-5.1-codex, effort: high, runner: codex }\n'
} > "$SBX/.docket.yml"
r_err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" 2>&1 >/dev/null )"; r_rc=$?
assert "0173 validator: a realistic multi-harness config is NOT rejected (rc=0)" '[ "$r_rc" = "0" ]'
assert "0173 validator: and it emits no bare-scalar complaint" \
  '! /usr/bin/grep -qF "is not a bare scalar" <<<"$r_err"'
assert "0173 validator: and it emits no has-no-value complaint" \
  '! /usr/bin/grep -qF "has no value" <<<"$r_err"'
assert "0173 validator: the realistic config still generated its wrapper" \
  '[ -f "$SBX/.claude/agents/docket-status.md" ]'
rm -rf "$SBX"

# -- an absent agents: block is not an error (the overwhelmingly common case) --
make_sandbox
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 ); n_rc=$?
assert "0173 validator: no config at all still generates (rc=0)" '[ "$n_rc" = "0" ]'
rm -rf "$SBX"

# -- the pre-0046 FLAT shape is warned+dropped elsewhere; the gate must not resurrect it as a hard
#    error, or a repo carrying already-ignored legacy config would stop generating entirely. --
make_sandbox
HROOT173G="$(mktemp -d)"; mkdir -p "$HROOT173G/.claude"
printf 'agents:\n  status: { model: two words, effort: high }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT173G" bash "$SYNC" >/dev/null 2>&1 ); lg_rc=$?
assert "0173 validator: legacy flat shape is not promoted to a fatal error" '[ "$lg_rc" = "0" ]'
rm -rf "$SBX" "$HROOT173G"

# -- the SAME carve-out reasoning, applied evenly (change 0173 review). Two more shapes are already
#    warned-and-dropped by sync-agents.sh, so the gate must not hard-fail on them either:
#    (a) an agents.<harness> block for a harness outside agent_harnesses ("ignored (dead config)"),
#    (b) an agent key overriding no built-in ("ignored (typo?)").
#    A repo carrying either with a quoted value could otherwise generate NOTHING at all. --
make_sandbox
HROOT173X="$(mktemp -d)"; mkdir -p "$HROOT173X/.claude"
printf 'agent_harnesses: [claude]\nagents:\n  codex:\n    status: { model: "gpt-5.1-codex" }\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT173X" bash "$SYNC" >/dev/null 2>&1 ); dead_rc=$?
assert "0173 validator: dead-config harness block does not block generation" '[ "$dead_rc" = "0" ]'
assert "0173 validator: and the live wrappers ARE written" \
  '[ "$(fm_anchored "$SBX/.claude/agents/docket-status.md" model)" = "sonnet" ]'
rm -rf "$SBX" "$HROOT173X"

make_sandbox
HROOT173Y="$(mktemp -d)"; mkdir -p "$HROOT173Y/.claude"
printf 'agents:\n  default:\n    nonexistent-agent: { model: "quoted-value" }\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT173Y" bash "$SYNC" >/dev/null 2>&1 ); typo_rc=$?
assert "0173 validator: typo'd agent key does not block generation" '[ "$typo_rc" = "0" ]'
assert "0173 validator: and the real agent still resolves" \
  '[ "$(fm_anchored "$SBX/.claude/agents/docket-status.md" model)" = "sonnet" ]'
rm -rf "$SBX" "$HROOT173Y"

# The carve-out must NOT become a hole: a LIVE harness block is still validated. Without this,
# skipping dead config could silently disarm the gate for the config that actually generates.
make_sandbox
HROOT173Z="$(mktemp -d)"; mkdir -p "$HROOT173Z/.claude"
printf 'agent_harnesses: [claude]\nagents:\n  claude:\n    status: { model: "quoted-value" }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT173Z" bash "$SYNC" >/dev/null 2>&1 ); live_rc=$?
assert "0173 validator: a LIVE harness block is still a hard failure" '[ "$live_rc" != "0" ]'
assert "0173 validator: and it wrote no wrapper" '[ ! -e "$SBX/.claude/agents/docket-status.md" ]'
rm -rf "$SBX" "$HROOT173Z"

# ---- 0208 (whole-branch review): the build-family CONSISTENCY BOND ----------------------
# validate_agent_scopes accepted any well-formed value, so a `docket-build-*` source declaring
# `worktree-scope: metadata` — a copy-paste from a metadata source, or a hand edit — lost gate 1,
# gate 3b and the shim's `--worktree` slot with nothing red, while runner-dispatch.sh's
# empty-payload refusal (`case "$AGENT" in build-*)`) kept treating it as a build worker. Two
# readings of ONE family that can disagree silently, in the direction that ships un-gated.
#
# These asserts exercise validate_agent_scopes DIRECTLY, against a fixture sources dir: the
# function takes the dir as its argument, and sync-agents.sh runs nothing on source
# (`if [ "${BASH_SOURCE[0]}" = "${0}" ]`), so no whole-tree copy and no generation pass is needed
# — the end-to-end wiring of this same validator into a real run is already pinned by
# tests/test_sync_agents_runners.sh ("0208(b): a missing worktree-scope fails generation").
# The source runs in a command substitution so sync-agents.sh's `set -euo pipefail` cannot leak
# into this file's own shell (the pattern tests/test_sync_agents_codex.sh already uses).
SCOPESRC="$(mktemp -d "${TMPDIR:-/tmp}/docket-0208-scopes.XXXXXX")"
cp "$REPO"/agents/docket-*.md "$SCOPESRC/"
scope_out="$( . "$REPO/sync-agents.sh" >/dev/null 2>&1; set +e +u; validate_agent_scopes "$SCOPESRC" 2>&1 )"; scope_rc=$?
assert "0208 bond: the SHIPPED source set validates clean (floor — the mutations below need it)" \
  '[ "$scope_rc" = "0" ]'
assert "0208 bond: floor — the fixture really holds the build family" \
  '[ -f "$SCOPESRC/docket-build-standard.md" ]'

# The mutation: one build profile flipped to a value that is WELL-FORMED but wrong for its family.
sed 's/^worktree-scope:.*/worktree-scope: metadata/' "$SCOPESRC/docket-build-standard.md" > "$SCOPESRC/.flip" \
  && mv -f "$SCOPESRC/.flip" "$SCOPESRC/docket-build-standard.md"
assert "0208 bond: fixture sanity — the copy really declares metadata now" \
  'grep -qx "worktree-scope: metadata" "$SCOPESRC/docket-build-standard.md"'
bond_out="$( . "$REPO/sync-agents.sh" >/dev/null 2>&1; set +e +u; validate_agent_scopes "$SCOPESRC" 2>&1 )"; bond_rc=$?
assert "0208 bond: a build-* source declaring metadata FAILS validation" '[ "$bond_rc" != "0" ]'
assert "0208 bond: the refusal names the agent and the value it must declare" \
  'grep -qF "build-standard" <<<"$bond_out" && grep -qF "feature" <<<"$bond_out"'
rm -rf "$SCOPESRC"

# The complement, and the reason this is a BOND and not a new floor: the declaration is still what
# rules for every agent the facade does not read by name. A NON-build source flipped to `metadata`
# is a policy change, not a contradiction, and must still validate — otherwise the bond has quietly
# become "the tests decide each agent's scope", which is what change 0208 exists to stop.
SCOPESRC2="$(mktemp -d "${TMPDIR:-/tmp}/docket-0208-scopes2.XXXXXX")"
cp "$REPO"/agents/docket-*.md "$SCOPESRC2/"
sed 's/^worktree-scope:.*/worktree-scope: metadata/' "$SCOPESRC2/docket-review-lean.md" > "$SCOPESRC2/.flip" \
  && mv -f "$SCOPESRC2/.flip" "$SCOPESRC2/docket-review-lean.md"
assert "0208 bond: fixture sanity — review-lean now declares metadata" \
  'grep -qx "worktree-scope: metadata" "$SCOPESRC2/docket-review-lean.md"'
nb_out="$( . "$REPO/sync-agents.sh" >/dev/null 2>&1; set +e +u; validate_agent_scopes "$SCOPESRC2" 2>&1 )"; nb_rc=$?
assert "0208 bond: a NON-build source may still declare either value" '[ "$nb_rc" = "0" ]'
rm -rf "$SCOPESRC2"

# ---- 0208 (whole-branch review): the shipped population's scopes are PINNED per agent ----
# The bond above binds one family of four; the other twelve sources were pinned by nothing but two spot
# checks that exist as fixture sanity elsewhere. A scope is a per-agent FACT — there is no shape to
# key on, so this is deliberately a table, and an agent absent from it fails rather than passing
# unclassified: adding a docket agent must be an explicit decision about where it runs. A REMOVED
# agent trips the count floor at the end.
expected_scope(){  # $1 = short name -> the scope this agent must declare, empty if unclassified
  case "$1" in
    build-economy|build-standard|build-premium|build-max) printf feature ;;
    rebase-resolver|integration-repair)                   printf feature ;;
    review-lean|review-standard|review-deep)              printf feature ;;
    adr|auto-groom|auto-groom-critic|brainstorm-consultant) printf metadata ;;
    finalize-change|implement-next|status)                 printf metadata ;;
  esac
}
n_pinned=0
for scope_src in "$REPO"/agents/docket-*.md; do
  [ -e "$scope_src" ] || continue
  sn="$(basename "$scope_src")"; sn="${sn#docket-}"; sn="${sn%.md}"
  want="$(expected_scope "$sn")"
  assert "0208 table: '$sn' is classified in the expected-scope table" '[ -n "$want" ]'
  [ -n "$want" ] || continue
  got="$(sed -n '/^worktree-scope:/{s/^worktree-scope:[[:space:]]*//;p;q;}' "$scope_src")"
  assert "0208 table: '$sn' declares worktree-scope: $want" '[ "$got" = "$want" ]'
  n_pinned=$((n_pinned+1))
done
# Floor: a vanished agents/ dir, or a source dropped without updating the table, leaves every
# assert above vacuously green.
assert "0208 table: the whole shipped population was pinned (>= 16, saw $n_pinned)" \
  '[ "$n_pinned" -ge 16 ]'

exit $fail
