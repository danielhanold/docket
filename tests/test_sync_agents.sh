#!/usr/bin/env bash
# tests/test_sync_agents.sh — run: bash tests/test_sync_agents.sh
set -uo pipefail
unset XDG_CONFIG_HOME   # hermetic: the script reads ${XDG_CONFIG_HOME:-$HARNESS_ROOT/.config}; pin global to the sandbox
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# True when any occurrence of $2 is followed by $3 within $4 characters in $1.
# Use Perl rather than grep -Pz: BSD grep lacks -P and caps interval bounds at 255.
within(){
  perl -0e '
    my ($file, $first, $second, $limit) = @ARGV;
    open my $fh, "<", $file or exit 1;
    local $/;
    my $text = <$fh>;
    my $offset = 0;
    while (($offset = index($text, $first, $offset)) >= 0) {
      exit 0 if index(substr($text, $offset, length($first) + $limit), $second) >= 0;
      ++$offset;
    }
    exit 1;
  ' "$1" "$2" "$3" "$4"
}

# Extract a single-line frontmatter scalar value from a markdown file.
fm(){ sed -n "s/^$2:[[:space:]]*//p" "$1" | head -n1 | sed 's/[[:space:]]*$//'; }

# Body = everything after the frontmatter's closing fence. Change 0168 made emit() strip-and-insert
# the resolved model/effort, so a generated wrapper is no longer byte-identical to its source; what
# must still hold is that the BODY comes through verbatim and only the pin is injected.
body_of(){ awk '/^---[[:space:]]*$/ && d<2 {d++; next} d>=2 {print}' "$1"; }

# The shipped default store (change 0168). Asserts about what an UNCONFIGURED wrapper is pinned to
# read it from here rather than from the wrapper source's frontmatter.
# shellcheck source=/dev/null
. "$REPO/scripts/lib/harness-defaults.sh"
HD="$REPO/agents/harness-defaults.yml"

# ---- Task 1: built-in wrapper source files ---------------------------------
AGENTS="$REPO/agents"
AUTONOMOUS="docket-implement-next docket-auto-groom docket-finalize-change docket-status docket-adr"

assert "agents/ source dir exists" '[ -d "$AGENTS" ]'
assert "exactly 16 built-in wrappers" '[ "$(find "$AGENTS" -maxdepth 1 -name "docket-*.md" | wc -l | tr -d " ")" = "16" ]'

for w in $AUTONOMOUS; do
  f="$AGENTS/$w.md"
  assert "$w: file exists" '[ -f "$f" ]'
  assert "$w: name matches file" '[ "$(fm "$f" name)" = "$w" ]'
  assert "$w: has a description" '[ -n "$(fm "$f" description)" ]'
  assert "$w: description matches the skill (single source)" \
    '[ "$(fm "$f" description)" = "$(fm "$REPO/skills/$w/SKILL.md" description)" ]'
  assert "$w: skills: injects the skill itself" 'grep -Eq "^skills:.*\b'"$w"'\b" "$f"'
  assert "$w: skills: injects docket-convention" 'grep -Eq "^skills:.*docket-convention" "$f"'
  assert "$w: body carries abort-and-report directive" 'grep -qi "abort-and-report" "$f"'
done

# Shipped model/effort match the §4 default table. Change 0168 moved these OUT of the wrapper
# sources and into agents/harness-defaults.yml, so they are read with hd_field, not fm(). Reading
# them off the source with fm() would be worse than stale: fm() is a first-match-ANYWHERE read, so
# with no `model:` line left in the frontmatter it scans on into the body and can return prose.
for w in $AUTONOMOUS; do
  n="${w#docket-}"
  assert "$w: shipped model is a known alias or full id" \
    '[[ "$(hd_field "$HD" claude "'"$n"'" model)" =~ ^(opus|sonnet|haiku|fable|claude-[a-z0-9]+(-[a-z0-9]+)*)$ ]]'
  assert "$w: shipped effort in allowed set" \
    '[[ "$(hd_field "$HD" claude "'"$n"'" effort)" =~ ^(low|medium|high|xhigh|max)$ ]]'
done
assert "implement-next shipped = claude-opus-5/medium" \
  '[ "$(hd_field "$HD" claude implement-next model)/$(hd_field "$HD" claude implement-next effort)" = "claude-opus-5/medium" ]'
assert "auto-groom shipped = claude-opus-5/low" \
  '[ "$(hd_field "$HD" claude auto-groom model)/$(hd_field "$HD" claude auto-groom effort)" = "claude-opus-5/low" ]'
assert "finalize-change shipped = claude-opus-5/low" \
  '[ "$(hd_field "$HD" claude finalize-change model)/$(hd_field "$HD" claude finalize-change effort)" = "claude-opus-5/low" ]'
assert "status shipped = claude-haiku-4-5-20251001/medium" \
  '[ "$(hd_field "$HD" claude status model)/$(hd_field "$HD" claude status effort)" = "claude-haiku-4-5-20251001/medium" ]'
assert "adr shipped = claude-opus-5/low" \
  '[ "$(hd_field "$HD" claude adr model)/$(hd_field "$HD" claude adr effort)" = "claude-opus-5/low" ]'

# Advisory/interactive skills must NOT have a wrapper file.
assert "no wrapper for new-change (advisory)" '[ ! -f "$AGENTS/docket-new-change.md" ]'
assert "no wrapper for groom-next (advisory)" '[ ! -f "$AGENTS/docket-groom-next.md" ]'

# ---- Task 2: sync-agents.sh generator --------------------------------------
SYNC="$REPO/sync-agents.sh"
assert "sync-agents.sh exists and is executable-by-bash" '[ -f "$SYNC" ]'

# -- cached reader equivalence: flow-map and block-shaped field boundaries --
reader_out="$({
  . "$SYNC"
  set +e  # sync-agents.sh enables errexit for direct invocation; this test intentionally does not.
  printf '%s\t%s\n' inline "$(field_of 'x: {model: a.b_c-d, effort: high}' model)"
  printf '%s\t%s\n' block "$(field_of '  model: slash/vendor:id' model)"
  printf '%s\t%s\n' prefix "$(field_of 'x: {model_alias: wrong, model: right}' model)"
  printf '%s\t%s\n' repeated "$(field_of 'x: {model: first, model: last}' model)"
  printf '%s\t%s\n' missing "$(field_of 'x: {effort: high}' model)"
  printf '%s\t%s\n' raw "$(field_of_raw 'x: {model: two words   , effort: high}' model)"
} )"
assert "0175 readers: consumed/raw edge cases preserve fixed semantics" \
  '[ "$reader_out" = "$(printf "inline\\ta.b_c-d\\nblock\\tslash/vendor:id\\nprefix\\tright\\nrepeated\\tlast\\nmissing\\t\\nraw\\ttwo words")" ]'

# Helper: a fresh fake harness root + repo for an isolated generator run.
make_sandbox(){ SBX="$(mktemp -d)"; mkdir -p "$SBX/.claude" "$SBX/.agents"; }   # .cursor/.codex/.kiro/.windsurf absent on purpose

# Count the parser-heavy external commands used by one real generation pass. The shims deliberately
# exec the pre-PATH absolute tools so this measures the generator rather than replacing its behavior.
parser_subprocess_count(){  # $1=generator path; sets FORK_COUNT + FORK_RC
  local generator="$1" shim_dir fork_log harness_root tool real_tool
  local real_sed real_head real_awk real_grep
  real_sed="$(command -v sed)"
  real_head="$(command -v head)"
  real_awk="$(command -v awk)"
  real_grep="$(command -v grep)"
  shim_dir="$(mktemp -d)"
  fork_log="$(mktemp)"
  harness_root="$(mktemp -d)"
  mkdir -p "$harness_root/.claude"
  : > "$fork_log"
  for tool in sed head awk grep; do
    case "$tool" in
      sed) real_tool="$real_sed" ;;
      head) real_tool="$real_head" ;;
      awk) real_tool="$real_awk" ;;
      grep) real_tool="$real_grep" ;;
    esac
    printf '%s\n' '#!/usr/bin/env bash' \
      "printf '%s\\n' '$tool' >> \"\${DOCKET_175_FORK_LOG:?}\"" \
      "exec $real_tool \"\$@\"" > "$shim_dir/$tool"
    chmod +x "$shim_dir/$tool"
  done
  make_sandbox
  PATH="$shim_dir:$PATH" DOCKET_175_FORK_LOG="$fork_log" DOCKET_HARNESS_ROOT="$harness_root" \
    bash "$generator" >/dev/null 2>&1
  FORK_RC=$?
  FORK_COUNT="$(wc -l < "$fork_log" | tr -d '[:space:]')"
  rm -rf "$SBX" "$shim_dir" "$fork_log" "$harness_root"
}

# The optimization's standing performance oracle: retain real generator behavior while bounding
# only its historic dominant parser commands. The nonzero floor makes a broken shim setup red too.
parser_subprocess_count "$SYNC"
assert "0175 parser subprocess guard: real generation completes successfully" '[ "$FORK_RC" = "0" ]'
assert "0175 parser subprocess guard: shims observed real generator calls" '[ "$FORK_COUNT" -gt 0 ]'
assert "0175 parser subprocess guard: dominant parser commands stay below 400" '[ "$FORK_COUNT" -lt 400 ]'

# -- command-line contract: help/errors must return before any generation side effect --
make_sandbox
HROOT175A="$(mktemp -d)"; mkdir -p "$HROOT175A/.claude"
help_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT175A" bash "$SYNC" --help 2>&1)"; help_rc=$?
assert "0175 args: --help succeeds" '[ "$help_rc" = "0" ]'
assert "0175 args: --help prints inventory-safe usage" '/usr/bin/grep -qF "Usage: sync-agents.sh [--check]" <<<"$help_out"'
assert "0175 args: --help writes no wrapper" '[ ! -e "$HROOT175A/.claude/agents/docket-status.md" ]'
bad_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT175A" bash "$SYNC" --bogus 2>&1)"; bad_rc=$?
assert "0175 args: unknown flag fails with rc=2" '[ "$bad_rc" = "2" ]'
assert "0175 args: unknown flag names the argument" '/usr/bin/grep -qF "unknown argument: --bogus" <<<"$bad_out"'
assert "0175 args: unknown flag writes no wrapper" '[ ! -e "$HROOT175A/.claude/agents/docket-status.md" ]'
rm -rf "$SBX" "$HROOT175A"

# -- optimized sidecar validation must preserve the raw top-header rule from hd_validate --
SCR175V="$(mktemp -d)"
cp -R "$REPO/agents" "$REPO/cursor-rules" "$REPO/scripts" "$REPO/sync-agents.sh" "$SCR175V/"
awk '{ if (!done && $0 == "agents:") { print "agents: # not a bare header"; done=1 } else print }' \
  "$SCR175V/agents/harness-defaults.yml" > "$SCR175V/agents/harness-defaults.yml.tmp"
mv "$SCR175V/agents/harness-defaults.yml.tmp" "$SCR175V/agents/harness-defaults.yml"
make_sandbox
HROOT175V="$(mktemp -d)"; mkdir -p "$HROOT175V/.claude"
v175_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT175V" bash "$SCR175V/sync-agents.sh" 2>&1 >/dev/null)"; v175_rc=$?
assert "0175 validator parity: commented top header is rejected" '[ "$v175_rc" != "0" ]'
assert "0175 validator parity: commented top header names the missing bare block" \
  '/usr/bin/grep -qF "no top-level '\''agents:'\'' block" <<<"$v175_err"'
assert "0175 validator parity: rejection happens before wrapper writes" \
  '[ ! -e "$HROOT175V/.claude/agents/docket-status.md" ]'
rm -rf "$SBX" "$HROOT175V" "$SCR175V"

# git-repo fixture: sandbox repo with identity + one commit (for ls-files-based legs).
# Defined here (rather than at first historical use, further down) so the change-0057
# widened-trigger tests — which need a real docket branch — can use it too.
mkgitrepo(){
  SBX="$(mktemp -d)"
  git -C "$SBX" init --quiet
  git -C "$SBX" config user.email t@t.test
  git -C "$SBX" config user.name Test
}

# -- user-level install: built-in wrappers, verbatim, into present harnesses --
make_sandbox
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null )
assert "writes into present .claude/agents" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
assert "writes into present .agents/agents" '[ -f "$SBX/.agents/agents/docket-status.md" ]'
assert "all 16 wrappers land in .claude/agents" '[ "$(find "$SBX/.claude/agents" -name "docket-*.md" | wc -l | tr -d " ")" = "16" ]'
assert "does NOT create an absent harness (.cursor)" '[ ! -d "$SBX/.cursor/agents" ]'
# Change 0168 replaced the byte-identity assert here: the generator now INJECTS the pin from
# agents/harness-defaults.yml instead of copying the source's frontmatter, so byte identity is
# structurally impossible. The mechanism this guarded — an unconfigured run reproduces the source
# faithfully and adds nothing of its own — is asserted directly instead.
assert "no override => body verbatim from the built-in source" \
  'diff -q <(body_of "$REPO/agents/docket-status.md") <(body_of "$SBX/.claude/agents/docket-status.md") >/dev/null'
assert "no override => name/description/skills come from the source" \
  '[ "$(fm "$SBX/.claude/agents/docket-status.md" name)" = "docket-status" ] &&
   [ "$(fm "$SBX/.claude/agents/docket-status.md" description)" = "$(fm "$REPO/agents/docket-status.md" description)" ] &&
   [ "$(fm "$SBX/.claude/agents/docket-status.md" skills)" = "$(fm "$REPO/agents/docket-status.md" skills)" ]'
assert "no override => the emitted pin is the SHIPPED sidecar value" \
  '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "$(hd_field "$HD" claude status model)" ] &&
   [ "$(fm "$SBX/.claude/agents/docket-status.md" effort)" = "$(hd_field "$HD" claude status effort)" ]'

# -- idempotency: second run is byte-identical ----
before="$(cat "$SBX/.claude/agents/docket-implement-next.md")"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null )
after="$(cat "$SBX/.claude/agents/docket-implement-next.md")"
assert "second run idempotent (byte-identical)" '[ "$before" = "$after" ]'
rm -rf "$SBX"

# -- global layer (harness-first, change 0050): config.yml agents: default: block overrides model/effort --
make_sandbox
mkdir -p "$SBX/.config/docket"
printf 'agents:\n  default:\n    status: { model: haiku, effort: low }\n    implement-next: { effort: auto }\n' > "$SBX/.config/docket/config.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null )
assert "global default sets model" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "haiku" ]'
assert "global default sets effort" '[ "$(fm "$SBX/.claude/agents/docket-status.md" effort)" = "low" ]'
assert "effort: auto drops the effort line" '! grep -q "^effort:" "$SBX/.claude/agents/docket-implement-next.md"'
assert "auto keeps the shipped model" '[ "$(fm "$SBX/.claude/agents/docket-implement-next.md" model)" = "claude-opus-5" ]'
assert "unlisted skill keeps shipped model+effort" '[ "$(fm "$SBX/.claude/agents/docket-adr.md" model)/$(fm "$SBX/.claude/agents/docket-adr.md" effort)" = "claude-opus-5/low" ]'
rm -rf "$SBX"

# -- global: a per-harness block overrides default for THAT harness only (user-level) --
make_sandbox                                        # .claude and .cursor both present so both get user-level files
mkdir -p "$SBX/.cursor" "$SBX/.config/docket"
printf 'agents:\n  default:\n    status: { model: haiku }\n  cursor:\n    status: { model: gpt-5.5-medium-fast }\n' > "$SBX/.config/docket/config.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null )
# 0135: the Cursor wrapper encodes effort INSIDE the model value. This config sets only a model,
# and since change 0168 there is nothing left for the effort to fall through TO on this harness:
# agents/harness-defaults.yml ships no cursor entry for `status`, and the Claude wrapper source is
# no longer a default store. A bare model is the correct output — the `[effort=medium]` this used to
# expect was docket-status's CLAUDE built-in leaking onto a harness that never saw a Claude pin.
# (Bracket encoding itself is still covered below by the agents.default effort fixture, and in
# tests/test_sync_agents_cursor.sh by the explicit model+effort override.)
assert "global cursor block wins for cursor" '[ "$(fm "$SBX/.cursor/agents/docket-status.md" model)" = "gpt-5.5-medium-fast" ]'
assert "global cursor block leaks no claude effort into the model value" \
  '! grep -q "\[effort=" "$SBX/.cursor/agents/docket-status.md"'
assert "global claude falls to default" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "haiku" ]'
rm -rf "$SBX"

# -- per-repo layer (harness-first): .docket.yml agents.default: => project-level files (machine-local since 0051) --
make_sandbox                                       # SBX = the repo
HROOT="$(mktemp -d)"; mkdir -p "$HROOT/.claude"    # separate user-level harness root
printf 'agents:\n  default:\n    status: { model: sonnet, effort: high }\n    new-change: { model: opus }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT" bash "$SYNC" >/dev/null )
assert "per-repo default writes project-level file" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
assert "per-repo default applies model" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "sonnet" ]'
assert "per-repo default applies effort" '[ "$(fm "$SBX/.claude/agents/docket-status.md" effort)" = "high" ]'
assert "0048: unlisted skill NOW generated at shipped default (implement-next)" '[ -f "$SBX/.claude/agents/docket-implement-next.md" ]'
assert "0048: unlisted implement-next carries shipped model (claude-opus-5)" '[ "$(fm "$SBX/.claude/agents/docket-implement-next.md" model)" = "claude-opus-5" ]'
assert "advisory skill in agents: produces NO file (new-change)" '[ ! -f "$SBX/.claude/agents/docket-new-change.md" ]'
rm -rf "$SBX" "$HROOT"

# ============================================================================
# Change 0048 — always-full-set per-repo generation (Piece 1)
# ============================================================================

# Per-repo now generates the FULL built-in set for a listed harness even when the
# agents: block lists only a subset; unlisted agents carry the built-in default model.
make_sandbox                                       # SBX = the repo
HROOT48A="$(mktemp -d)"; mkdir -p "$HROOT48A/.claude"
printf 'agents:\n  default:\n    status: { model: sonnet, effort: high }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48A" bash "$SYNC" >/dev/null )
assert "0048: full set — all 16 built-ins land in project-level .claude/agents" \
  '[ "$(find "$SBX/.claude/agents" -name "docket-*.md" | wc -l | tr -d " ")" = "16" ]'
assert "0048: listed agent carries its override (status=sonnet)" \
  '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "sonnet" ]'
assert "0048: UNLISTED agent generated at shipped default (implement-next=claude-opus-5/medium)" \
  '[ "$(fm "$SBX/.claude/agents/docket-implement-next.md" model)/$(fm "$SBX/.claude/agents/docket-implement-next.md" effort)" = "claude-opus-5/medium" ]'
rm -rf "$SBX" "$HROOT48A"

# 0048 Piece 2 — the Cursor dispatch rule is generated per-repo when cursor is listed.
make_sandbox
HROOT48R="$(mktemp -d)"; mkdir -p "$HROOT48R/.claude"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48R" bash "$SYNC" >/dev/null )
RULE="$SBX/.cursor/rules/docket-dispatch.mdc"
assert "0048 rule: per-repo docket-dispatch.mdc written for cursor" '[ -f "$RULE" ]'
assert "0048 rule: carries alwaysApply: true frontmatter" 'grep -q "^alwaysApply: true" "$RULE"'
# The rule is a GENERATED artifact whose head is cursor-rules/dispatch.head.md catted verbatim.
# tests/test_cursor_dispatch_rule.sh guards the head's CONTENT; nothing guarded that the generated
# file still carries it, so a head edit could be asserted true at the source and shipped mangled
# (0168 whole-branch review, IMPORTANT 3). Byte-compare the generated prefix against the source.
assert "0048 rule: generated file opens with dispatch.head.md byte-for-byte" \
  'diff -q <(head -n "$(wc -l < "$REPO/cursor-rules/dispatch.head.md")" "$RULE") "$REPO/cursor-rules/dispatch.head.md" >/dev/null'
assert "0048 rule: has the required dispatch pattern heading" 'grep -q "## Required dispatch pattern" "$RULE"'
assert "0048 rule: has a subsection for every built-in agent (16)" \
  '[ "$(grep -cE "^## docket-.* — dispatch only" "$RULE")" = "16" ]'
assert "0048 rule: names docket-implement-next as a subsection" 'grep -q "^## docket-implement-next — dispatch only" "$RULE"'
assert "0048 rule: names docket-status as a subsection" 'grep -q "^## docket-status — dispatch only" "$RULE"'
assert "0048 rule: no subsection for a non-existent agent" '! grep -q "docket-nonexistent" "$RULE"'
assert "0048 rule: deterministic order — adr before status" \
  '[ "$(grep -n "^## docket-adr — dispatch only" "$RULE" | cut -d: -f1)" -lt "$(grep -n "^## docket-status — dispatch only" "$RULE" | cut -d: -f1)" ]'
rm -rf "$SBX" "$HROOT48R"

# 0048 Piece 2 — cursor NOT listed => no per-repo rule (claude/other harness gets none).
make_sandbox
HROOT48N="$(mktemp -d)"; mkdir -p "$HROOT48N/.claude"
printf 'agent_harnesses: [claude]\nagents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48N" bash "$SYNC" >/dev/null )
assert "0048 rule: no dispatch rule for a claude-only repo" '[ ! -e "$SBX/.cursor/rules/docket-dispatch.mdc" ]'
assert "0048 rule: no rules dir under .claude" '[ ! -e "$SBX/.claude/rules/docket-dispatch.mdc" ]'
rm -rf "$SBX" "$HROOT48N"

# 0048 Piece 2 — user-level: rule written to ~/.cursor/rules when ~/.cursor present, skipped when absent.
make_sandbox                                  # make_sandbox creates .claude + .agents; .cursor ABSENT
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null )
assert "0048 rule: user-level rule SKIPPED when ~/.cursor absent" '[ ! -e "$SBX/.cursor/rules/docket-dispatch.mdc" ]'
mkdir -p "$SBX/.cursor"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null )
assert "0048 rule: user-level rule WRITTEN when ~/.cursor present" '[ -f "$SBX/.cursor/rules/docket-dispatch.mdc" ]'
rm -rf "$SBX"

# 0048 Piece 2 — a built-in agent lacking a fragment gets a minimal auto-block + a warning.
# Simulate by pointing the generator at a scratch clone whose fragment we remove.
make_sandbox
HROOT48F="$(mktemp -d)"; mkdir -p "$HROOT48F/.claude"
printf 'agent_harnesses: [cursor]\nagents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
# Remove one fragment in a throwaway copy of the repo scripts so the auto-block path fires.
SCRATCH="$(mktemp -d)"; cp -R "$REPO/agents" "$REPO/cursor-rules" "$REPO/scripts" "$REPO/sync-agents.sh" "$SCRATCH/"
rm -f "$SCRATCH/cursor-rules/dispatch/docket-status.md"
gen_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48F" bash "$SCRATCH/sync-agents.sh" 2>&1 >/dev/null)"
RULE="$SBX/.cursor/rules/docket-dispatch.mdc"
assert "0048 auto-block: warns about the missing fragment" 'printf "%s" "$gen_err" | grep -qi "no dispatch fragment for docket-status"'
assert "0048 auto-block: still emits a docket-status subsection" 'grep -q "^## docket-status — dispatch only" "$RULE"'
# 0135: the auto-block instructs by CAPABILITY, not by a tool name (ADR-0059 §2) — it must still
# name the agent it dispatches to, so this pins the dispatch sentence, not the old `subagent_type:`.
assert "0048 auto-block: subsection dispatches to the named subagent by capability" \
  'grep -q "Dispatch to the subagent .docket-status. using this mode.s subagent-launch mechanism" "$RULE"'
rm -rf "$SBX" "$HROOT48F" "$SCRATCH"

# 0048 Piece 2 --check — a committed dispatch rule that drifts fails --check.
make_sandbox
HROOT48C="$(mktemp -d)"; mkdir -p "$HROOT48C/.claude"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48C" bash "$SYNC" >/dev/null )
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48C" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0048 rule-check: passes for an in-sync committed rule (rc=0)" '[ "$chk_rc" = "0" ]'
# Hand-edit the committed rule -> advisory (leg c; content staleness never fails CI).
printf '\n<!-- tampered -->\n' >> "$SBX/.cursor/rules/docket-dispatch.mdc"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48C" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0048 rule-check: advisory-flags a hand-edited rule (rc=0)" '[ "$chk_rc" = "0" ]'
assert "0048 rule-check: names the dispatch rule in the advisory report" \
  'printf "%s" "$chk_out" | grep -q "advisory" && printf "%s" "$chk_out" | grep -q "docket-dispatch.mdc"'
# Delete the committed rule -> advisory (missing local file).
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48C" bash "$SYNC" >/dev/null )   # regenerate clean
rm -f "$SBX/.cursor/rules/docket-dispatch.mdc"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48C" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0048 rule-check: advisory-flags a missing committed rule (rc=0)" '[ "$chk_rc" = "0" ]'
assert "0048 rule-check: missing-rule advisory names it" \
  'printf "%s" "$chk_out" | grep -q "advisory" && printf "%s" "$chk_out" | grep -q "docket-dispatch.mdc"'
rm -rf "$SBX" "$HROOT48C"

# 0048 Piece 3 — removing a built-in agent prunes its generated files (both layers) + rule subsection.
make_sandbox
HROOT48P="$(mktemp -d)"; mkdir -p "$HROOT48P/.cursor"   # present user-level cursor root
printf 'agent_harnesses: [cursor]\nagents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
# Scratch clone we can mutate (remove a built-in agent + its fragment).
SCRATCH="$(mktemp -d)"; cp -R "$REPO/agents" "$REPO/cursor-rules" "$REPO/scripts" "$REPO/sync-agents.sh" "$SCRATCH/"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48P" bash "$SCRATCH/sync-agents.sh" >/dev/null )
assert "0048 prune: adr generated before removal (per-repo)" '[ -f "$SBX/.cursor/agents/docket-adr.md" ]'
assert "0048 prune: adr generated before removal (user-level)" '[ -f "$HROOT48P/.cursor/agents/docket-adr.md" ]'
# Remove the built-in agent + its fragment, regenerate: the orphan must be pruned.
rm -f "$SCRATCH/agents/docket-adr.md" "$SCRATCH/cursor-rules/dispatch/docket-adr.md"
# change 0168: the sidecar's claude block is set-EQUAL to agents/docket-*.md in both
# directions, so retiring a built-in also retires its shipped default entry. Leaving it
# behind is a genuine sidecar defect and hd_validate refuses the whole run before any
# wrapper is written — which would make this leg fail for the wrong reason.
sed -i.bak '/^    adr:/d' "$SCRATCH/agents/harness-defaults.yml"; rm -f "$SCRATCH/agents/harness-defaults.yml.bak"
assert "0048 prune fixture: sidecar adr entry removed with the wrapper" \
  '[ "$(grep -c "^    adr:" "$SCRATCH/agents/harness-defaults.yml")" = "0" ]'
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48P" bash "$SCRATCH/sync-agents.sh" >/dev/null )
assert "0048 prune: removed built-in pruned from per-repo .cursor/agents" '[ ! -e "$SBX/.cursor/agents/docket-adr.md" ]'
assert "0048 prune: removed built-in pruned from user-level .cursor/agents" '[ ! -e "$HROOT48P/.cursor/agents/docket-adr.md" ]'
assert "0048 prune: rule subsection for removed agent dropped" '! grep -q "^## docket-adr — dispatch only" "$SBX/.cursor/rules/docket-dispatch.mdc"'
assert "0048 prune: a surviving agent remains" '[ -f "$SBX/.cursor/agents/docket-status.md" ]'
rm -rf "$SBX" "$HROOT48P" "$SCRATCH"

# 0048 Piece 3 — de-listing cursor prunes its per-repo docket files + rule, keeps a co-located non-docket file.
make_sandbox
HROOT48D="$(mktemp -d)"; mkdir -p "$HROOT48D/.claude"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48D" bash "$SYNC" >/dev/null )
: > "$SBX/.cursor/agents/my-own-agent.md"          # operator's own co-located file
assert "0048 delist: cursor agents present before de-list" '[ -f "$SBX/.cursor/agents/docket-status.md" ]'
assert "0048 delist: cursor rule present before de-list" '[ -f "$SBX/.cursor/rules/docket-dispatch.mdc" ]'
# De-list cursor.
printf 'agent_harnesses: [claude]\nagents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48D" bash "$SYNC" >/dev/null )
assert "0048 delist: cursor docket agents pruned" '[ ! -e "$SBX/.cursor/agents/docket-status.md" ]'
assert "0048 delist: cursor dispatch rule pruned" '[ ! -e "$SBX/.cursor/rules/docket-dispatch.mdc" ]'
assert "0048 delist: operator's co-located non-docket file preserved" '[ -f "$SBX/.cursor/agents/my-own-agent.md" ]'
assert "0048 delist: claude still generated" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
rm -rf "$SBX" "$HROOT48D"

# 0048 Piece 3 --check — an orphaned local file is reported as advisory, NOT deleted
# (change 0051: orphaned per-repo files are untracked local artifacts now, not CI-fatal).
make_sandbox
HROOT48O="$(mktemp -d)"; mkdir -p "$HROOT48O/.claude"
printf 'agent_harnesses: [claude]\nagents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48O" bash "$SYNC" >/dev/null )
: > "$SBX/.claude/agents/docket-bogus.md"           # an orphan: no built-in docket-bogus
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48O" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0048 orphan-check: advisory-flags the orphan (rc=0)" '[ "$chk_rc" = "0" ]'
assert "0048 orphan-check: names the orphaned file" 'printf "%s" "$chk_out" | grep -q "advisory" && printf "%s" "$chk_out" | grep -q "docket-bogus.md"'
assert "0048 orphan-check: --check does NOT delete the orphan" '[ -f "$SBX/.claude/agents/docket-bogus.md" ]'
rm -rf "$SBX" "$HROOT48O"

# (a)+(b) harness override wins; field-level merge — model from cursor, effort inherited from default.
make_sandbox
HROOTM="$(mktemp -d)"; mkdir -p "$HROOTM/.claude"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: sonnet, effort: high }\n  cursor:\n    status: { model: gpt-5.5-medium-fast }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTM" bash "$SYNC" >/dev/null )
assert "0046 (a): cursor model from cursor block" '[ "$(fm "$SBX/.cursor/agents/docket-status.md" model)" = "gpt-5.5-medium-fast[effort=high]" ]'
# 0135 retired the standalone `effort:` key from Cursor wrappers, but the MECHANISM this guards —
# field-level merge, where effort falls through to default: while model comes from the cursor: block
# — is still live. Narrowed to read the surviving carrier of that value.
assert "0046 (b): cursor effort inherited from default (now inside the model value)" \
  '[ "$(fm "$SBX/.cursor/agents/docket-status.md" model)" = "gpt-5.5-medium-fast[effort=high]" ]'
assert "0046 (a): claude model falls to default" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "sonnet" ]'
assert "0046 (a): claude effort from default" '[ "$(fm "$SBX/.claude/agents/docket-status.md" effort)" = "high" ]'
# (c) arbitrary non-Claude id passes through verbatim; the two harness files now DIFFER (was byte-identical pre-0046).
assert "0046 (c): non-Claude id verbatim in .cursor" '[ "$(fm "$SBX/.cursor/agents/docket-status.md" model)" = "gpt-5.5-medium-fast[effort=high]" ]'
# NOTE (0135): this next assert is now trivially true — every Cursor wrapper differs from its Claude
# counterpart by shape alone. Its discriminating power moved to "0135 (d): default-only => harness
# files DIFFER". Kept because it is cheap.
assert "0046: harness files differ when overridden" '! diff -q "$SBX/.claude/agents/docket-status.md" "$SBX/.cursor/agents/docket-status.md" >/dev/null'
rm -rf "$SBX" "$HROOTM"

# (d) default-only (no harness block) reaches EVERY listed harness. 0135 inverted the byte-identity
# half of this: a Cursor wrapper is no longer Claude-shaped, so the two files must now DIFFER.
make_sandbox
HROOTD0="$(mktemp -d)"; mkdir -p "$HROOTD0/.claude"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: sonnet, effort: high }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTD0" bash "$SYNC" >/dev/null )
# 0135 inverted this: a Cursor wrapper is NO LONGER Claude-shaped, so default-only config must
# produce DIFFERENT files. The surviving property is that the default: block reaches both.
assert "0135 (d): default-only => harness files DIFFER (cursor has its own shape)" \
  '! diff -q "$SBX/.claude/agents/docket-status.md" "$SBX/.cursor/agents/docket-status.md" >/dev/null'
assert "0046 (d): default-only applies model to claude" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "sonnet" ]'
assert "0135 (d): default-only applies model+effort to cursor" '[ "$(fm "$SBX/.cursor/agents/docket-status.md" model)" = "sonnet[effort=high]" ]'
rm -rf "$SBX" "$HROOTD0"

# 0046: tab-indented .docket.yml agents: block resolves (ind() must count tabs as indentation, not drop the block)
make_sandbox
HROOTT="$(mktemp -d)"; mkdir -p "$HROOTT/.claude"
printf 'agents:\n\tdefault:\n\t\tstatus: { model: sonnet, effort: high }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTT" bash "$SYNC" >/dev/null )
assert "0046: tab-indented agents: block is not silently dropped" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
assert "0046: tab-indented default: resolves model" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "sonnet" ]'
rm -rf "$SBX" "$HROOTT"

# ---- Task 1b: the docket-auto-groom-critic wrapper (wraps NO skill) ---------
CRITIC="$AGENTS/docket-auto-groom-critic.md"
assert "critic wrapper exists" '[ -f "$CRITIC" ]'
assert "critic: name matches file" '[ "$(fm "$CRITIC" name)" = "docket-auto-groom-critic" ]'
assert "critic: has a description" '[ -n "$(fm "$CRITIC" description)" ]'
assert "critic: shipped model is claude-opus-5" '[ "$(hd_field "$HD" claude auto-groom-critic model)" = "claude-opus-5" ]'
assert "critic: shipped effort is medium" '[ "$(hd_field "$HD" claude auto-groom-critic effort)" = "medium" ]'
assert "critic: skills injects docket-convention" 'grep -Eq "^skills:.*docket-convention" "$CRITIC"'
# Isolation: the skills: line must NOT pull in the designer skill (would re-inject its bias).
# Scope the check to the skills: line — the name: line legitimately contains "docket-auto-groom".
crit_skills_line="$(grep -E "^skills:" "$CRITIC" || true)"
assert "critic: skills EXCLUDES the docket-auto-groom designer skill" '! grep -q "docket-auto-groom" <<<"$crit_skills_line"'
assert "critic: body carries abort-and-report directive" 'grep -qi "abort-and-report" "$CRITIC"'

# Per-repo override of the critic key (auto-groom-critic) resolves to this wrapper source,
# proving the precedence path + --check drift gate cover the critic.
make_sandbox                                        # SBX = the repo
HROOT2="$(mktemp -d)"; mkdir -p "$HROOT2/.claude"   # separate user-level harness root
printf 'agents:\n  default:\n    auto-groom-critic: { model: sonnet, effort: high }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT2" bash "$SYNC" >/dev/null )
assert "per-repo critic override writes project-level file" '[ -f "$SBX/.claude/agents/docket-auto-groom-critic.md" ]'
assert "per-repo critic override applies model" '[ "$(fm "$SBX/.claude/agents/docket-auto-groom-critic.md" model)" = "sonnet" ]'
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT2" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "--check passes for in-sync critic (rc=0)" '[ "$chk_rc" = "0" ]'
sed -i.bak 's/^model: sonnet/model: haiku/' "$SBX/.claude/agents/docket-auto-groom-critic.md"; rm -f "$SBX/.claude/agents/docket-auto-groom-critic.md.bak"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT2" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "--check advisory-flags critic drift (rc=0)" '[ "$chk_rc" = "0" ]'
assert "--check advisory-flags critic drift (names file)" \
  'printf "%s" "$chk_out" | grep -q "advisory" && printf "%s" "$chk_out" | grep -q "docket-auto-groom-critic.md"'
rm -rf "$SBX" "$HROOT2"

# ---- Task 1c: the two finalize-gate wrappers (wrap NO skill) ----------------
# docket-rebase-resolver (①) and docket-integration-repair (②): like the critic,
# they inject ONLY docket-convention, pin opus/medium, and carry abort-and-report.
for nw in docket-rebase-resolver docket-integration-repair; do
  f="$AGENTS/$nw.md"
  assert "$nw: wrapper exists" '[ -f "$f" ]'
  assert "$nw: name matches file" '[ "$(fm "$f" name)" = "$nw" ]'
  assert "$nw: has a description" '[ -n "$(fm "$f" description)" ]'
  assert "$nw: shipped model is claude-opus-5" '[ "$(hd_field "$HD" claude "'"${nw#docket-}"'" model)" = "claude-opus-5" ]'
  assert "$nw: shipped effort is medium" '[ "$(hd_field "$HD" claude "'"${nw#docket-}"'" effort)" = "medium" ]'
  assert "$nw: skills injects docket-convention" 'grep -Eq "^skills:.*docket-convention" "$f"'
  # Isolation: the skills: line wraps NO docket skill (only the convention).
  nw_skills_line="$(grep -E "^skills:" "$f" || true)"
  assert "$nw: skills EXCLUDES any wrapped docket skill" \
    '! grep -Eq "docket-(finalize-change|implement-next|auto-groom|status|adr|groom-next|new-change)" <<<"$nw_skills_line"'
  assert "$nw: body carries abort-and-report directive" 'grep -qi "abort-and-report" "$f"'
done

# ---- the brainstorm consultant wrapper (wraps NO skill AND injects NO convention) ----
CONSULT="$AGENTS/docket-brainstorm-consultant.md"
assert "consultant: wrapper exists" '[ -f "$CONSULT" ]'
assert "consultant: name matches file" '[ "$(fm "$CONSULT" name)" = "docket-brainstorm-consultant" ]'
assert "consultant: has a description" '[ -n "$(fm "$CONSULT" description)" ]'
assert "consultant: shipped model is claude-opus-5" '[ "$(hd_field "$HD" claude brainstorm-consultant model)" = "claude-opus-5" ]'
assert "consultant: shipped effort is medium" '[ "$(hd_field "$HD" claude brainstorm-consultant effort)" = "medium" ]'
# Deliberate ADR-0009 deviation: injects NEITHER a wrapped skill NOR docket-convention.
assert "consultant: injects NO docket-convention" '! grep -Eq "^skills:.*docket-convention" "$CONSULT"'
assert "consultant: injects NO wrapped docket skill" '! grep -Eq "^skills:.*docket-(finalize-change|implement-next|auto-groom|status|adr|groom-next|new-change|brainstorm)\b" "$CONSULT"'
assert "consultant: body names the spec deliverable + assumptions requirement" 'grep -qi "spec" "$CONSULT" && grep -qi "assumption" "$CONSULT"'

# Per-repo override of a new key (rebase-resolver) resolves to its wrapper source,
# proving the precedence path + --check drift gate cover the new wrappers.
make_sandbox                                        # SBX = the repo
HROOT3="$(mktemp -d)"; mkdir -p "$HROOT3/.claude"   # separate user-level harness root
printf 'agents:\n  default:\n    rebase-resolver: { model: sonnet, effort: high }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT3" bash "$SYNC" >/dev/null )
assert "per-repo rebase-resolver override writes project-level file" '[ -f "$SBX/.claude/agents/docket-rebase-resolver.md" ]'
assert "per-repo rebase-resolver override applies model" '[ "$(fm "$SBX/.claude/agents/docket-rebase-resolver.md" model)" = "sonnet" ]'
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT3" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "--check passes for in-sync rebase-resolver (rc=0)" '[ "$chk_rc" = "0" ]'
sed -i.bak 's/^model: sonnet/model: haiku/' "$SBX/.claude/agents/docket-rebase-resolver.md"; rm -f "$SBX/.claude/agents/docket-rebase-resolver.md.bak"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT3" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "--check advisory-flags rebase-resolver drift (rc=0)" '[ "$chk_rc" = "0" ]'
assert "--check advisory-flags rebase-resolver drift (names file)" \
  'printf "%s" "$chk_out" | grep -q "advisory" && printf "%s" "$chk_out" | grep -q "docket-rebase-resolver.md"'
rm -rf "$SBX" "$HROOT3"

# ---- Task 3: --check drift gate --------------------------------------------
make_sandbox
printf 'agents:\n  default:\n    status: { model: sonnet, effort: high }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null )   # generate committed project file
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "--check passes when committed agents match config (rc=0)" '[ "$chk_rc" = "0" ]'

# Out-of-band edit to a local project-level file -> advisory (leg c), never CI-fatal.
sed -i.bak 's/^model: sonnet/model: haiku/' "$SBX/.claude/agents/docket-status.md"; rm -f "$SBX/.claude/agents/docket-status.md.bak"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "--check advisory-flags drift (rc=0)" '[ "$chk_rc" = "0" ]'
assert "--check reports an advisory" 'printf "%s" "$chk_out" | grep -q "advisory"'

# Local file removed after having been generated once (block already written) ->
# advisory only (leg c; missing local file is never CI-fatal).
make_sandbox
printf 'agents:\n  default:\n    status: { model: sonnet, effort: high }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null )   # generate + write the gitignore block
rm -f "$SBX/.claude/agents/docket-status.md"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "--check advisory-flags a missing local file (rc=0)" '[ "$chk_rc" = "0" ]'
assert "--check reports the missing-local-file advisory" 'printf "%s" "$chk_out" | grep -q "advisory"'

# leg (a): opted-in repo whose .gitignore block was never written (sync never ran) -> rc!=0.
make_sandbox
printf 'agents:\n  default:\n    status: { model: sonnet, effort: high }\n' > "$SBX/.docket.yml"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "--check leg-a: missing gitignore block fails (rc!=0)" '[ "$chk_rc" != "0" ]'
assert "--check leg-a: names the gitignore block" 'printf "%s" "$chk_out" | grep -qi "gitignore"'

# 0048 opt-in: a .docket.yml present for change-tracking only (no agents: / no agent_harnesses) does
# NOT opt into per-repo generation — nothing is written and --check stays a no-op (backward-compat).
make_sandbox                                          # SBX = the repo
HROOTTO="$(mktemp -d)"; mkdir -p "$HROOTTO/.claude"   # separate user-level root
printf 'metadata_branch: docket\n' > "$SBX/.docket.yml"      # tracking-only: no opt-in keys
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTTO" bash "$SYNC" >/dev/null )
assert "0048 opt-in: tracking-only repo writes NO project-level wrappers" '[ ! -e "$SBX/.claude/agents/docket-status.md" ]'
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTTO" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0048 opt-in: tracking-only repo --check is a no-op (rc=0)" '[ "$chk_rc" = "0" ]'
rm -rf "$SBX" "$HROOTTO"

# 0048 opt-in: agent_harnesses alone (NO agents: block) opts in — the real Cursor-repo case:
# full built-in set + dispatch rule generated for the listed harnesses, at built-in defaults.
make_sandbox
HROOTAH="$(mktemp -d)"; mkdir -p "$HROOTAH/.claude"
printf 'agent_harnesses: [claude, cursor]\n' > "$SBX/.docket.yml"   # no agents: block at all
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTAH" bash "$SYNC" >/dev/null )
assert "0048 opt-in: agent_harnesses-only generates full set for cursor" '[ "$(find "$SBX/.cursor/agents" -name "docket-*.md" | wc -l | tr -d " ")" = "16" ]'
assert "0048 opt-in: agent_harnesses-only generates full set for claude" '[ "$(find "$SBX/.claude/agents" -name "docket-*.md" | wc -l | tr -d " ")" = "16" ]'
assert "0048 opt-in: agent_harnesses-only generates the cursor dispatch rule" '[ -f "$SBX/.cursor/rules/docket-dispatch.mdc" ]'
assert "0048 opt-in: agent_harnesses-only wrappers carry shipped default (no overrides)" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "claude-haiku-4-5-20251001" ]'
rm -rf "$SBX" "$HROOTAH"

# 0048: a repo with NO .docket.yml at all has nothing to check -> passes.
make_sandbox
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0048: --check passes when no .docket.yml (rc=0)" '[ "$chk_rc" = "0" ]'
rm -rf "$SBX"

# ---- Task 5: docket-convention documents the agent layer -------------------
CONV="$REPO/skills/docket-convention/SKILL.md"
AGL="$REPO/skills/docket-convention/references/agent-layer.md"
assert "agent-layer reference exists" '[ -f "$AGL" ]'
assert "convention points at the agent-layer reference (blocking)" 'grep -qF "references/agent-layer.md" "$CONV"'
assert "convention documents the agents: config block" 'grep -q "agents:" "$CONV"'
assert "convention names the generator sync-agents.sh" 'grep -q "sync-agents.sh" "$CONV"'
assert "convention states the precedence" 'grep -qi "repo-local > repo-committed > global > built-in" "$CONV"'
assert "agent-layer ref states auto => omit effort" 'grep -qi "auto" "$AGL" && grep -qi "omit" "$AGL"'
assert "convention states abort-and-report for autonomous subagents" 'grep -qi "abort-and-report" "$CONV"'
assert "convention points at composition (0017)" 'grep -q "0017" "$CONV"'
# Non-vacuous guard: the agent section must be a distinct heading, not an incidental word.
assert "convention has an agent-layer section heading" 'grep -qiE "^#+ .*(agent layer|model/effort|subagent)" "$CONV"'

# 0046: convention documents the harness-first agents: shape (default: + harness keys, field-level fallback).
assert "0046 doc: agent-layer ref names the reserved default: key" 'within "$AGL" "agents:" "default:" 400'
assert "0046 doc: agent-layer ref shows a per-harness key example (cursor)" 'within "$AGL" "agents:" "cursor:" 600'
assert "0046 doc: agent-layer ref states field-level fallback H -> default -> built-in" 'grep -qiE "harness.*default.*built-in|<harness>.*default.*built-in" "$AGL"'
# Change 0168 reworded this line: the shipped layer is harness-indexed, so a non-claude harness no
# longer falls back to a claude ID — it warns and ships unpinned. Anchored on the verbatim clause.
assert "0046 doc: agent-layer ref notes non-Claude fallback warning" \
  'grep -qiE "non-.?claude.? harness with no harness-specific model gets a non-fatal warning" "$AGL"'

# 0048 doc: convention states per-repo generates the full built-in set (config override-only)
# and that cursor gets a generated docket-dispatch.mdc rule.
assert "0048 doc: agent-layer ref says per-repo writes the full built-in set" 'grep -qiE "full (built-in )?(agent )?set" "$AGL"'
assert "0048 doc: agent-layer ref says the agents: block is override-only" 'grep -qi "override-only" "$AGL"'
assert "0048 doc: agent-layer ref names the cursor dispatch rule" 'grep -q "docket-dispatch.mdc" "$AGL"'

# ---- Task 6: advisory recommendation in the interactive skills -------------
NEWC="$REPO/skills/docket-new-change/SKILL.md"
GROOM="$REPO/skills/docket-groom-next/SKILL.md"
assert "new-change carries an advisory recommendation" 'grep -qi "[Rr]ecommended model" "$NEWC"'
assert "new-change recommends sonnet" 'grep -qi "sonnet" "$NEWC"'
assert "groom-next carries an advisory recommendation" 'grep -qi "[Rr]ecommended model" "$GROOM"'
assert "groom-next recommends sonnet/high" 'grep -qiE "sonnet[^A-Za-z]+high|high[^A-Za-z]+sonnet" "$GROOM"'
# Non-vacuous: it must be advisory, not a hard requirement (we cannot force the session model).
assert "new-change frames it as advisory" 'grep -qi "advisory" "$NEWC"'
# Explicit pin (change 0042): the advisory must name the full model ID, not the bare alias.
assert "new-change advisory pins claude-sonnet-5" 'grep -q "claude-sonnet-5" "$NEWC"'
assert "groom-next advisory pins claude-sonnet-5" 'grep -q "claude-sonnet-5" "$GROOM"'

# ============================================================================
# Change 0045 — multi-harness project-level generation (agent_harnesses)
# ============================================================================

# (a) DEFAULT (no agent_harnesses key) => [claude]: project-level writes
#     .claude/agents ONLY (byte-identical to pre-0045 behavior). Separate HROOT
#     so <repo>/.claude/agents is purely project-level output.
make_sandbox                                          # SBX = the repo
HROOTA="$(mktemp -d)"; mkdir -p "$HROOTA/.claude"     # separate user-level root
printf 'agents:\n  default:\n    status: { model: sonnet, effort: high }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTA" bash "$SYNC" >/dev/null )
assert "0045 default: writes project-level .claude/agents" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
assert "0045 default: does NOT write .cursor/agents" '[ ! -e "$SBX/.cursor/agents/docket-status.md" ]'
assert "0045 default: per-repo model applied" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "sonnet" ]'
rm -rf "$SBX" "$HROOTA"

# (b) agent_harnesses: [claude, cursor] => BOTH dirs generated; cursor gets its own model
#     override so the files DIFFER (0046: no longer byte-identical when overridden).
make_sandbox
HROOTB="$(mktemp -d)"; mkdir -p "$HROOTB/.claude"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: sonnet }\n  cursor:\n    status: { model: gpt-5.5-medium-fast }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTB" bash "$SYNC" >/dev/null )
assert "0045 fanout: .claude/agents generated" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
assert "0045 fanout: .cursor/agents generated" '[ -f "$SBX/.cursor/agents/docket-status.md" ]'
assert "0046 fanout: claude carries default model" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "sonnet" ]'
# 0135 + 0168: bracket-encoded when an effort resolves. This fixture pins no effort, and the cursor
# harness has no shipped `status` entry in agents/harness-defaults.yml, so the model is emitted bare
# rather than picking up docket-status's Claude built-in effort (see "global cursor block wins").
assert "0046 fanout: cursor carries its override model" '[ "$(fm "$SBX/.cursor/agents/docket-status.md" model)" = "gpt-5.5-medium-fast" ]'
# NOTE (0135): trivially true now (see the note on "0046: harness files differ when overridden").
assert "0046 fanout: harness files differ when cursor overrides" '! diff -q "$SBX/.claude/agents/docket-status.md" "$SBX/.cursor/agents/docket-status.md" >/dev/null'
rm -rf "$SBX" "$HROOTB"

# (b') agent_harnesses: [cursor] ONLY => cursor generated, claude NOT (no forced-claude).
make_sandbox
HROOTC="$(mktemp -d)"; mkdir -p "$HROOTC/.claude"
printf 'agent_harnesses: [cursor]\nagents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTC" bash "$SYNC" >/dev/null )
assert "0045 cursor-only: .cursor/agents generated" '[ -f "$SBX/.cursor/agents/docket-status.md" ]'
assert "0045 cursor-only: .claude/agents NOT generated" '[ ! -e "$SBX/.claude/agents/docket-status.md" ]'
assert "0048: [cursor]-only leaves the pre-existing user .claude dir intact" '[ -d "$SBX/.claude" ]'
rm -rf "$SBX" "$HROOTC"

# (d) unknown harness token => warned + dropped, NOT fatal; known harness still generated.
make_sandbox
HROOTD="$(mktemp -d)"; mkdir -p "$HROOTD/.claude"
printf 'agent_harnesses: [claude, bogus]\nagents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
gen_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTD" bash "$SYNC" 2>&1 >/dev/null)"; gen_rc=$?
assert "0045 unknown-token: generation not fatal (rc=0)" '[ "$gen_rc" = "0" ]'
assert "0045 unknown-token: warns about the token" 'printf "%s" "$gen_err" | grep -qi "unknown agent_harnesses token"'
assert "0045 unknown-token: names the bad token" 'printf "%s" "$gen_err" | grep -q "bogus"'
assert "0045 unknown-token: known harness still generated" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
assert "0045 unknown-token: bad-token dir NOT created" '[ ! -e "$SBX/.bogus/agents" ]'
rm -rf "$SBX" "$HROOTD"

# (e) explicit empty list agent_harnesses: [] => resolves to no targets: no project
#     files generated (mirrors board_surfaces: []). Locks the empty-set code path.
make_sandbox
HROOTE0="$(mktemp -d)"; mkdir -p "$HROOTE0/.claude"
printf 'agent_harnesses: []\nagents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTE0" bash "$SYNC" >/dev/null )
assert "0045 empty-list: no .claude project file" '[ ! -e "$SBX/.claude/agents/docket-status.md" ]'
assert "0045 empty-list: no .cursor project file" '[ ! -e "$SBX/.cursor/agents/docket-status.md" ]'
assert "0048: empty-list leaves the pre-existing user .claude dir intact" '[ -d "$SBX/.claude" ]'
rm -rf "$SBX" "$HROOTE0"

# --check must span every listed harness: drift in a .cursor/agents file fails CI.
make_sandbox
HROOTF="$(mktemp -d)"; mkdir -p "$HROOTF/.claude"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: sonnet, effort: high }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTF" bash "$SYNC" >/dev/null )   # generate both harness files
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTF" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0045 check: passes when both harness files in sync (rc=0)" '[ "$chk_rc" = "0" ]'
# Drift the CURSOR file only -> advisory (leg c), never CI-fatal.
sed -i.bak 's/^model: sonnet/model: haiku/' "$SBX/.cursor/agents/docket-status.md"; rm -f "$SBX/.cursor/agents/docket-status.md.bak"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTF" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0045 check: advisory-flags .cursor/agents drift (rc=0)" '[ "$chk_rc" = "0" ]'
assert "0045 check: advisory report names the cursor harness" 'printf "%s" "$chk_out" | grep -q "advisory" && printf "%s" "$chk_out" | grep -q "cursor"'
# A listed-harness file never generated locally -> advisory (missing local file).
rm -f "$SBX/.cursor/agents/docket-status.md"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTF" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0045 check: advisory-flags missing cursor file (rc=0)" '[ "$chk_rc" = "0" ]'
assert "0045 check: missing-file advisory names cursor" 'printf "%s" "$chk_out" | grep -q "advisory" && printf "%s" "$chk_out" | grep -q "cursor"'
rm -rf "$SBX" "$HROOTF"

# Convention documents agent_harnesses + the direct-model-ID (harness-neutral) contract.
CONV="$REPO/skills/docket-convention/SKILL.md"
AGL="$REPO/skills/docket-convention/references/agent-layer.md"
assert "0045 doc: convention names agent_harnesses" 'grep -q "agent_harnesses" "$CONV"'
assert "0045 doc: convention states default [claude]" 'grep -qE "agent_harnesses.*\[claude\]|default.*\[claude\]" "$CONV"'
assert "0045 doc: agent-layer ref states harness-neutral direct model IDs" 'grep -qiE "harness-neutral|direct model id" "$AGL"'
assert "0045 doc: agent-layer ref notes passthrough enables non-Claude harnesses" 'grep -qi "passthrough" "$AGL"'
assert "0045 doc: agent-layer ref points at ADR-0015 near agent_harnesses" 'within "$AGL" "agent_harnesses" "ADR-0015" 500 || within "$AGL" "ADR-0015" "agent_harnesses" 500'

# (f) a glob-metachar token must NOT expand against the cwd (set -f guard). A decoy
#     file present in the repo must never leak into the warnings.
make_sandbox
HROOTG="$(mktemp -d)"; mkdir -p "$HROOTG/.claude"
: > "$SBX/DECOYFILE"                                  # a filename the glob would match
printf 'agent_harnesses: [claude, *]\nagents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
gen_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTG" bash "$SYNC" 2>&1 >/dev/null)"; gen_rc=$?
assert "0045 glob-token: generation not fatal (rc=0)" '[ "$gen_rc" = "0" ]'
assert "0045 glob-token: cwd decoy file did NOT leak into warnings" '! printf "%s" "$gen_err" | grep -q "DECOYFILE"'
assert "0045 glob-token: known harness still generated" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
rm -rf "$SBX" "$HROOTG"

# (g) agent_harnesses is a top-level (column-0) key: an indented decoy under another
#     block must NOT be read; the real top-level key wins.
make_sandbox
HROOTH="$(mktemp -d)"; mkdir -p "$HROOTH/.claude"
printf 'decoy:\n  agent_harnesses: [cursor]\nagent_harnesses: [claude]\nagents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTH" bash "$SYNC" >/dev/null )
assert "0045 anchor: top-level agent_harnesses honored (.claude generated)" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
assert "0045 anchor: indented decoy ignored (.cursor NOT generated)" '[ ! -e "$SBX/.cursor/agents/docket-status.md" ]'
rm -rf "$SBX" "$HROOTH"

# ---- README discoverability of the agent model/effort refresh workflow (change 0047) ----
# The facts already exist buried in the Install prose, so a whole-README grep would pass
# vacuously. Extract the NEW dedicated section (heading -> next `## `) and assert within it,
# so each sentinel is RED before the section exists and non-vacuous after.
READMEF="$REPO/README.md"
sec="$(awk '/^##[[:space:]].*[Aa]gent.*([Mm]odel|[Ee]ffort)/{f=1;print;next} f&&/^##[[:space:]]/{f=0} f{print}' "$READMEF")"

assert "0047: README has a discoverable agent model/effort section" '[ -n "$sec" ]'
assert "0047 §agent-cfg: names the global layer ~/.config/docket/config.yml" \
  'grep -qF "~/.config/docket/config.yml" <<<"$sec"'
assert "0047 §agent-cfg: names the per-repo .docket.yml agents: layer" \
  'grep -qF "\`agents:\` block in a repo" <<<"$sec"'
assert "0047 §agent-cfg: gives the refresh command (bash sync-agents.sh)" \
  'grep -qE "bash sync-agents\.sh" <<<"$sec"'
assert "0047 §agent-cfg: names the user-level target (every present harness)" \
  'grep -qiE "present.*harness" <<<"$sec"'
assert "0047 §agent-cfg: names the project-level target (agent_harnesses)" \
  'grep -qF "agent_harnesses" <<<"$sec"'
assert "0047 §agent-cfg: documents the --check drift gate" \
  'grep -qF "sync-agents.sh --check" <<<"$sec"'
assert "0047 §agent-cfg: references docket-convention Agent layer for the shape (not restated)" \
  'grep -qF "docket-convention" <<<"$sec" && grep -qi "agent layer" <<<"$sec"'
assert "0047 §agent-cfg: documents effort: auto drops the pinned effort line" \
  'grep -qF "effort: auto" <<<"$sec" && grep -qF "drops the effort line" <<<"$sec"'
# Non-restatement guard: the section must NOT hardcode a per-skill model/effort literal
# (those are config-overridable; the shipped defaults live only in agents/harness-defaults.yml
# since change 0168 — the wrapper sources are behavior-only templates). LEARNINGS #17.
assert "0047 §agent-cfg: does NOT hardcode a model/effort literal (references the source instead)" \
  '! grep -qiE "\b(opus|sonnet|haiku|fable)\b.*\b(xhigh|high|medium|low)\b|model:[[:space:]]*(opus|sonnet|haiku|claude-)" <<<"$sec"'

# ============================================================================
# Change 0046 — per-harness values: diagnostics
# ============================================================================

# (h) Non-Claude fallback warning: a cursor file whose model fell through to agents.default warns;
#     suppressed for claude, and suppressed when cursor supplies its own model.
# Change 0168 re-worded the diagnostic: the source frontmatter is no longer a default store, so
# the fallthrough can only come from agents.default (or from nothing at all, which is a distinct
# "generated unpinned" warning). The property guarded is unchanged — this fixture pins a Claude
# ID under agents.default, which outranks the sidecar, so the cursor wrapper really is emitted with
# a foreign ID and cursor must be told. (Cursor now ships a complete sidecar block; what makes the
# warning correct here is that agents.default WON the field, not that the pair is uncovered.)
make_sandbox
HROOTW="$(mktemp -d)"; mkdir -p "$HROOTW/.claude"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: claude-opus-4-8 }\n' > "$SBX/.docket.yml"
gen_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTW" bash "$SYNC" 2>&1 >/dev/null)"; gen_rc=$?
assert "0046 (h): generation not fatal (rc=0)" '[ "$gen_rc" = "0" ]'
assert "0046 (h): warns cursor model came from agents.default" 'grep -qi "cursor/docket-status" <<<"$gen_err" && grep -qF "came from agents.default" <<<"$gen_err"'
assert "0046 (h): does NOT warn for the claude harness" '! printf "%s" "$gen_err" | grep -qiE "claude/docket-status|WARN claude"'
rm -rf "$SBX" "$HROOTW"

# (h') warning suppressed when the cursor block supplies the model.
make_sandbox
HROOTW2="$(mktemp -d)"; mkdir -p "$HROOTW2/.claude"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: claude-opus-4-8 }\n  cursor:\n    status: { model: gpt-5.5-medium-fast }\n' > "$SBX/.docket.yml"
gen_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTW2" bash "$SYNC" 2>&1 >/dev/null)"; gen_rc=$?
assert "0046 (h'): no fallback warning when cursor supplies model" '! grep -qiE "cursor/docket-status: (no harness-specific model|model .* came from agents\.default)" <<<"$gen_err"'
rm -rf "$SBX" "$HROOTW2"

# (f) Legacy bare-agent-key block (pre-0046 flat shape) => warned + ignored; --check flags it as drift.
make_sandbox
HROOTL="$(mktemp -d)"; mkdir -p "$HROOTL/.claude"
printf 'agents:\n  status: { model: sonnet, effort: high }\n' > "$SBX/.docket.yml"   # bare agent key, no default:/harness
gen_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTL" bash "$SYNC" 2>&1 >/dev/null)"; gen_rc=$?
assert "0046 (f): legacy shape not fatal (rc=0)" '[ "$gen_rc" = "0" ]'
assert "0046 (f): warns about the legacy bare agent key" 'printf "%s" "$gen_err" | grep -qi "legacy" && printf "%s" "$gen_err" | grep -q "status"'
assert "0046 (f): legacy status NOT applied (no project file / shipped only)" '[ ! -f "$SBX/.claude/agents/docket-status.md" ] || [ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "claude-haiku-4-5-20251001" ]'
# Pre-run a normal sync so the .gitignore block exists (leg a green) and the legacy
# committed-config-shape leg is isolated (still rc!=0 — CI-meaningful, not advisory).
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTL" bash "$SYNC" >/dev/null 2>&1 )
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTL" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0046 (g'): --check flags the legacy shape (rc!=0)" '[ "$chk_rc" != "0" ]'
assert "0046 (g'): --check names the legacy shape" 'printf "%s" "$chk_out" | grep -qi "legacy"'
rm -rf "$SBX" "$HROOTL"

# (e) Dead-config harness (a block in agents: not present in agent_harnesses) => warned + dropped.
make_sandbox
HROOTX="$(mktemp -d)"; mkdir -p "$HROOTX/.claude"
printf 'agent_harnesses: [claude]\nagents:\n  default:\n    status: { model: sonnet }\n  cursor:\n    status: { model: gpt-5.5-medium-fast }\n' > "$SBX/.docket.yml"
gen_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTX" bash "$SYNC" 2>&1 >/dev/null)"; gen_rc=$?
assert "0046 (e): dead-config not fatal (rc=0)" '[ "$gen_rc" = "0" ]'
assert "0046 (e): warns cursor block is not in agent_harnesses" 'printf "%s" "$gen_err" | grep -qi "cursor" && printf "%s" "$gen_err" | grep -qi "agent_harnesses"'
assert "0046 (e): cursor file NOT generated (dropped)" '[ ! -e "$SBX/.cursor/agents/docket-status.md" ]'
assert "0046 (e): claude still generated from default" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "sonnet" ]'
rm -rf "$SBX" "$HROOTX"

# ============================================================================
# Change 0050 — agents.yaml -> config.yml auto-migration (owned by sync-agents.sh)
# ============================================================================

# Happy path: agents.yaml (old top-level harness-first map) is rewritten under agents:
# in config.yml, the original renamed .migrated, the run logs loudly, values apply.
make_sandbox
mkdir -p "$SBX/.config/docket"
printf 'default:\n  status: { model: haiku, effort: low }\n' > "$SBX/.config/docket/agents.yaml"
mig_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" 2>&1 >/dev/null)"
assert "0050 mig: config.yml gains an agents: block" 'grep -qE "^agents[[:space:]]*:" "$SBX/.config/docket/config.yml"'
assert "0050 mig: old file renamed to .migrated" '[ -f "$SBX/.config/docket/agents.yaml.migrated" ] && [ ! -e "$SBX/.config/docket/agents.yaml" ]'
assert "0050 mig: logs the migration loudly" 'printf "%s" "$mig_err" | grep -qi "migrat"'
assert "0050 mig: migrated values applied to wrappers" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "haiku" ]'
# Idempotency: a second run leaves config.yml byte-identical (no duplicate agents: block).
cfg_before="$(cat "$SBX/.config/docket/config.yml")"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
cfg_after="$(cat "$SBX/.config/docket/config.yml")"
assert "0050 mig: second run no-ops on config.yml" '[ "$cfg_before" = "$cfg_after" ]'
assert "0050 mig: exactly one agents: block" '[ "$(grep -cE "^agents[[:space:]]*:" "$SBX/.config/docket/config.yml")" = "1" ]'
rm -rf "$SBX"

# Migration preserves pre-existing non-agents config.yml content.
make_sandbox
mkdir -p "$SBX/.config/docket"
printf 'auto_groom: true\n' > "$SBX/.config/docket/config.yml"
printf 'default:\n  status: { model: haiku }\n' > "$SBX/.config/docket/agents.yaml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
assert "0050 mig: pre-existing config.yml keys preserved" 'grep -q "^auto_groom: true" "$SBX/.config/docket/config.yml"'
assert "0050 mig: agents: appended alongside" 'grep -qE "^agents[[:space:]]*:" "$SBX/.config/docket/config.yml"'
assert "0050 mig: values from the appended block apply" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "haiku" ]'
rm -rf "$SBX"

# Migration into a config.yml whose last line lacks a trailing newline must not glue keys.
make_sandbox
mkdir -p "$SBX/.config/docket"
printf 'auto_groom: true' > "$SBX/.config/docket/config.yml"     # NO trailing newline
printf 'default:\n  status: { model: haiku }\n' > "$SBX/.config/docket/agents.yaml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
assert "0050 mig: no-trailing-newline config.yml not glued" 'grep -q "^auto_groom: true$" "$SBX/.config/docket/config.yml" && grep -qE "^agents[[:space:]]*:" "$SBX/.config/docket/config.yml"'
assert "0050 mig: no-trailing-newline values still apply" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "haiku" ]'
rm -rf "$SBX"

# Stale twin: config.yml already has agents: AND a live agents.yaml is present ->
# warn stale, do NOT read it, do NOT rename it (only the migration renames).
make_sandbox
mkdir -p "$SBX/.config/docket"
printf 'agents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.config/docket/config.yml"
printf 'default:\n  status: { model: haiku }\n' > "$SBX/.config/docket/agents.yaml"
stale_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" 2>&1 >/dev/null)"
assert "0050 stale: warns agents.yaml is stale/unread" 'printf "%s" "$stale_err" | grep -qi "stale"'
assert "0050 stale: config.yml value wins" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "sonnet" ]'
assert "0050 stale: agents.yaml left in place" '[ -f "$SBX/.config/docket/agents.yaml" ]'
rm -rf "$SBX"

# No dual-read: a lone agents.yaml.migrated (post-migration state) is never read.
make_sandbox
mkdir -p "$SBX/.config/docket"
printf 'default:\n  status: { model: haiku }\n' > "$SBX/.config/docket/agents.yaml.migrated"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
assert "0050 no-dual-read: .migrated is not read (shipped model)" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "claude-haiku-4-5-20251001" ]'
rm -rf "$SBX"

# ============================================================================
# Change 0050 — global agent_harnesses scopes the USER-LEVEL pass only
# ============================================================================

# Extends + narrows: the global list overrides presence-on-disk detection.
make_sandbox                                   # creates .claude + .agents; .cursor ABSENT
mkdir -p "$SBX/.config/docket"
printf 'agent_harnesses: [claude, cursor]\n' > "$SBX/.config/docket/config.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
assert "0050 gah: listed ABSENT harness extended (cursor created+written)" '[ -f "$SBX/.cursor/agents/docket-status.md" ]'
assert "0050 gah: listed present harness written (claude)" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
assert "0050 gah: present-but-UNLISTED harness narrowed (.agents untouched)" '[ ! -e "$SBX/.agents/agents/docket-status.md" ]'
assert "0050 gah: user-level cursor dispatch rule written when cursor listed" '[ -f "$SBX/.cursor/rules/docket-dispatch.mdc" ]'
rm -rf "$SBX"

# Global [] => the user-level pass writes nothing (explicit empty list, not "unset"),
# and existing user-level docket wrappers are pruned (every known harness is de-listed).
make_sandbox
mkdir -p "$SBX/.config/docket" "$SBX/.claude/agents"
: > "$SBX/.claude/agents/docket-status.md"          # stale wrapper from an earlier run
printf 'agent_harnesses: []\n' > "$SBX/.config/docket/config.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
assert "0050 gah []: no user-level files written despite present .claude" '[ ! -e "$SBX/.claude/agents/docket-status.md" ]'
assert "0050 gah []: harness root preserved after prune" '[ -d "$SBX/.claude" ]'
rm -rf "$SBX"

# Unset global key => presence-on-disk detection unchanged (regression pin).
make_sandbox
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
assert "0050 gah unset: presence detection still writes .claude" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
assert "0050 gah unset: absent harness still skipped" '[ ! -d "$SBX/.cursor/agents" ]'
rm -rf "$SBX"

# Unknown token in the GLOBAL list: warned + dropped, not fatal.
make_sandbox
mkdir -p "$SBX/.config/docket"
printf 'agent_harnesses: [claude, bogus]\n' > "$SBX/.config/docket/config.yml"
gah_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" 2>&1 >/dev/null)"; gah_rc=$?
assert "0050 gah unknown: not fatal (rc=0)" '[ "$gah_rc" = "0" ]'
assert "0050 gah unknown: warns and names the token" 'printf "%s" "$gah_err" | grep -qi "unknown agent_harnesses token" && printf "%s" "$gah_err" | grep -q "bogus"'
assert "0050 gah unknown: known harness still written" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
rm -rf "$SBX"

# Scope split: the global key never opts a repo into per-repo generation, and the
# per-repo committed pass is governed SOLELY by the repo's own agent_harnesses.
REPO50="$(mktemp -d)"; HROOT50="$(mktemp -d)"
mkdir -p "$HROOT50/.claude" "$HROOT50/.config/docket"
printf 'metadata_branch: docket\n' > "$REPO50/.docket.yml"          # tracking-only repo
printf 'agent_harnesses: [claude]\n' > "$HROOT50/.config/docket/config.yml"
( cd "$REPO50" && DOCKET_HARNESS_ROOT="$HROOT50" bash "$SYNC" >/dev/null 2>&1 )
assert "0050 gah scope: global key does NOT opt repo into per-repo generation" '[ ! -e "$REPO50/.claude/agents/docket-status.md" ]'
assert "0050 gah scope: user-level still written" '[ -f "$HROOT50/.claude/agents/docket-status.md" ]'
rm -rf "$REPO50" "$HROOT50"

REPO51="$(mktemp -d)"; HROOT51="$(mktemp -d)"
mkdir -p "$HROOT51/.claude" "$HROOT51/.config/docket"
printf 'agent_harnesses: [claude]\n' > "$REPO51/.docket.yml"        # repo opts in: claude only
printf 'agent_harnesses: [cursor]\n' > "$HROOT51/.config/docket/config.yml"
( cd "$REPO51" && DOCKET_HARNESS_ROOT="$HROOT51" bash "$SYNC" >/dev/null 2>&1 )
assert "0050 gah scope: per-repo pass follows the REPO list (claude written)" '[ -f "$REPO51/.claude/agents/docket-status.md" ]'
assert "0050 gah scope: per-repo pass ignores the global list (no repo .cursor)" '[ ! -e "$REPO51/.cursor/agents/docket-status.md" ]'
assert "0050 gah scope: global [cursor] scopes user-level (cursor written)" '[ -f "$HROOT51/.cursor/agents/docket-status.md" ]'
assert "0050 gah scope: user-level claude NOT written (narrowed by global list)" '[ ! -e "$HROOT51/.claude/agents/docket-status.md" ]'
rm -rf "$REPO51" "$HROOT51"

# Narrowing the global list on a later run prunes the de-listed harness's USER-LEVEL
# docket-owned files (mirrors the per-repo de-list rule); user content + the root survive.
make_sandbox
mkdir -p "$SBX/.config/docket" "$SBX/.cursor"
printf 'agent_harnesses: [claude, cursor]\n' > "$SBX/.config/docket/config.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
assert "0050 gah prune: cursor user files present before narrowing" '[ -f "$SBX/.cursor/agents/docket-status.md" ]'
: > "$SBX/.cursor/agents/my-own-agent.md"
printf 'agent_harnesses: [claude]\n' > "$SBX/.config/docket/config.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
assert "0050 gah prune: de-listed cursor docket agents pruned" '[ ! -e "$SBX/.cursor/agents/docket-status.md" ]'
assert "0050 gah prune: de-listed cursor dispatch rule pruned" '[ ! -e "$SBX/.cursor/rules/docket-dispatch.mdc" ]'
assert "0050 gah prune: user's own co-located file preserved" '[ -f "$SBX/.cursor/agents/my-own-agent.md" ]'
assert "0050 gah prune: harness root dir preserved" '[ -d "$SBX/.cursor" ]'
assert "0050 gah prune: listed claude still written" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
rm -rf "$SBX"

# ---- Change 0050 — README "Global config" section + convention three-layer story ----
# Extract the new dedicated README section (heading -> next `## `), assert within it.
gsec="$(awk '/^##[[:space:]].*[Gg]lobal config/{f=1;print;next} f&&/^##[[:space:]]/{f=0} f{print}' "$READMEF")"
assert "0050 doc: README has a Global config section" '[ -n "$gsec" ]'
assert "0050 doc: §global names the canonical path" 'grep -qF "~/.config/docket/config.yml" <<<"$gsec"'
assert "0050 doc: §global states the same-schema rule" 'grep -qiE "same schema as .?\.docket\.yml" <<<"$gsec"'
assert "0050 doc: §global states per-key precedence" 'grep -qi "repo-local > repo-committed > global > built-in" <<<"$gsec"'
assert "0050 doc: §global states coordination keys are per-repo-only" 'grep -qi "per-repo-only" <<<"$gsec"'
assert "0050 doc: §global names the agents.yaml migration" 'grep -qF "agents.yaml.migrated" <<<"$gsec"'
assert "0050 doc: §global scopes agent_harnesses to the user-level pass" 'grep -qiE "user-level pass" <<<"$gsec"'
# Tuning section gains the both-passes clarification (LEARNINGS #49 — surface end-to-end).
sec="$(awk '/^##[[:space:]].*[Aa]gent.*([Mm]odel|[Ee]ffort)/{f=1;print;next} f&&/^##[[:space:]]/{f=0} f{print}' "$READMEF")"
assert "0050 doc: tuning section states sync-agents writes BOTH layers" 'grep -qiE "both" <<<"$sec" && grep -qiE "project (level )?win|project-over-user|project wins" <<<"$sec"'
# Convention: Configuration documents the three-layer story + the fence.
CONV="$REPO/skills/docket-convention/SKILL.md"
AGL="$REPO/skills/docket-convention/references/agent-layer.md"
assert "0050 doc: convention names config.yml" 'grep -qF "config.yml" "$CONV"'
assert "0050 doc: convention states the coordination-key fence" 'grep -qi "fence" "$CONV" && grep -qi "per-repo-only" "$CONV"'
assert "0050 doc: agent-layer ref Agent layer global row points at config.yml agents: block" \
  'grep -qE "^\| Global \|.*config\.yml" "$AGL"'

# ---- Change 0051 doc sentinels ----
assert "0051 doc: README documents .docket.local.yml" 'grep -qF ".docket.local.yml" "$READMEF"'
assert "0051 doc: README states generated agents are machine-local, never committed" \
  'grep -qiE "machine-local" "$READMEF" && grep -qiE "never committed" "$READMEF"'
assert "0057 doc: README documents the managed docket .gitignore block" 'grep -qF "# docket:start" "$READMEF" || grep -qE "managed .docket. block" "$READMEF"'
assert "0057 doc: README no longer names the legacy docket:generated block" '! grep -qF "docket:generated" "$READMEF"'
assert "0051 doc: README documents the migration (git rm --cached / one commit)" 'grep -qiE "migrat" "$READMEF" && grep -qF -e "--cached" "$READMEF"'
assert "0051 doc: convention documents .docket.local.yml" 'grep -qF ".docket.local.yml" "$CONV"'
assert "0051 doc: agent-layer ref states all-local generation (gitignored, never committed)" 'grep -qiE "gitignored, never committed|machine-local, never committed" "$AGL"'
assert "0057 doc: agent-layer ref documents the managed docket block (new marker)" 'grep -qF "# docket:start" "$AGL" || grep -qi "managed docket .gitignore block" "$AGL"'
assert "0057 doc: agent-layer ref documents the check via the managed block" 'grep -qi "advisory" "$AGL"'
assert "0057 doc: agent-layer ref no longer names docket:generated" '! grep -qF "docket:generated" "$AGL"'
# Change 0101: the agents: documentation moved from this repo's .docket.yml (now values-only)
# to .docket.example.yml, the canonical reference. Both asserts follow it.
assert "0051 doc: example agents comment states machine-local generation" 'grep -qi "machine-local" "$REPO/.docket.example.yml"'
assert "0051 doc: example drops the stale agents.yaml global reference" '! grep -q "agents.yaml" "$REPO/.docket.example.yml"'

# ============================================================================
# Change 0051 — four-layer per-field agents: resolution; all-local generation.
# Precedence: local.agents.H.X -> local.default.X -> committed.H.X -> committed.default.X
#             -> global.H.X -> global.default.X -> built-in. THE 0050 BUG FIX:
# a global agents: block now REACHES per-repo generated files (no committed shadow).
# ============================================================================

# (4L-a) THE FIX — opted-in repo + global agents: + no repo/local override
# => the generated project-level file carries the GLOBAL model (was: built-in + SHADOWED warning).
make_sandbox
HROOT51A="$(mktemp -d)"; mkdir -p "$HROOT51A/.claude" "$HROOT51A/.config/docket"
printf 'agents:\n  default:\n    status: { model: global-model-x }\n' > "$HROOT51A/.config/docket/config.yml"
printf 'agent_harnesses: [claude]\n' > "$SBX/.docket.yml"
sw_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT51A" bash "$SYNC" 2>&1 >/dev/null)"
assert "0051 4L: global agents value reaches the per-repo generated file" \
  '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "global-model-x" ]'
assert "0051 4L: the 0050 SHADOWED stopgap warning is gone" '! printf "%s" "$sw_err" | grep -q "SHADOWED"'
rm -rf "$SBX" "$HROOT51A"

# (4L-b) full chain: local beats committed beats global; per-FIELD independence
# (model from local, effort from committed) and harness-over-default within a layer.
make_sandbox
HROOT51B="$(mktemp -d)"; mkdir -p "$HROOT51B/.claude" "$HROOT51B/.config/docket"
printf 'agents:\n  default:\n    status: { model: global-m, effort: low }\n' > "$HROOT51B/.config/docket/config.yml"
printf 'agents:\n  default:\n    status: { model: committed-m, effort: high }\n' > "$SBX/.docket.yml"
printf 'agents:\n  default:\n    status: { model: local-m }\n' > "$SBX/.docket.local.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT51B" bash "$SYNC" >/dev/null 2>&1 )
assert "0051 4L: local model beats committed+global"        '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "local-m" ]'
assert "0051 4L: effort unset locally falls to committed"   '[ "$(fm "$SBX/.claude/agents/docket-status.md" effort)" = "high" ]'
# harness key in a LOWER layer still loses to default in a HIGHER layer for that field:
printf 'agents:\n  claude:\n    status: { model: committed-claude-m }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT51B" bash "$SYNC" >/dev/null 2>&1 )
assert "0051 4L: local default beats committed harness key" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "local-m" ]'
rm -rf "$SBX" "$HROOT51B"

# (4L-c) opt-in via the LOCAL file alone — a machine opts a tracking-only repo in
# without touching committed config; local agent_harnesses governs the target list.
make_sandbox
HROOT51C="$(mktemp -d)"; mkdir -p "$HROOT51C/.claude"
printf 'metadata_branch: docket\n' > "$SBX/.docket.yml"           # tracking-only committed file
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: local-m }\n' > "$SBX/.docket.local.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT51C" bash "$SYNC" >/dev/null 2>&1 )
assert "0051 opt-in: local file alone opts in (claude generated)"  '[ -f "$SBX/.claude/agents/docket-status.md" ]'
assert "0051 opt-in: local agent_harnesses honored (cursor too)"   '[ -f "$SBX/.cursor/agents/docket-status.md" ]'
assert "0051 opt-in: cursor dispatch rule generated"               '[ -f "$SBX/.cursor/rules/docket-dispatch.mdc" ]'
assert "0051 opt-in: local model applied"                          '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "local-m" ]'
rm -rf "$SBX" "$HROOT51C"

# (4L-d) local agent_harnesses BEATS committed (key-level precedence, not a merge).
make_sandbox
HROOT51D="$(mktemp -d)"; mkdir -p "$HROOT51D/.claude"
printf 'agent_harnesses: [claude, cursor]\n' > "$SBX/.docket.yml"
printf 'agent_harnesses: [claude]\n' > "$SBX/.docket.local.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT51D" bash "$SYNC" >/dev/null 2>&1 )
assert "0051 gah: local list wins (claude generated)"     '[ -f "$SBX/.claude/agents/docket-status.md" ]'
assert "0051 gah: committed cursor overridden away"       '[ ! -e "$SBX/.cursor/agents/docket-status.md" ]'
rm -rf "$SBX" "$HROOT51D"

# (4L-e) tracking-only repo with NEITHER file opted in: still zero files (regression pin).
make_sandbox
HROOT51E="$(mktemp -d)"; mkdir -p "$HROOT51E/.claude"
printf 'metadata_branch: docket\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT51E" bash "$SYNC" >/dev/null 2>&1 )
assert "0051 opt-in: neither file => zero project files" '[ ! -e "$SBX/.claude/agents/docket-status.md" ]'
rm -rf "$SBX" "$HROOT51E"

# (4L-f) malformed .docket.local.yml (a directory): warn + skip, run still succeeds,
# committed layer still honored.
make_sandbox
HROOT51F="$(mktemp -d)"; mkdir -p "$HROOT51F/.claude"
printf 'agents:\n  default:\n    status: { model: committed-m }\n' > "$SBX/.docket.yml"
mkdir "$SBX/.docket.local.yml"
mf_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT51F" bash "$SYNC" 2>&1 >/dev/null)"; mf_rc=$?
assert "0051 malformed local: not fatal (rc=0)"        '[ "$mf_rc" = "0" ]'
assert "0051 malformed local: warns and names the file" 'printf "%s" "$mf_err" | grep -qi "docket.local.yml"'
assert "0051 malformed local: committed layer still applies" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "committed-m" ]'
rm -rf "$SBX" "$HROOT51F"

# (4L-g) tab-indented local YAML resolves (LEARNINGS #46 — indent classes must be [^[:space:]]).
make_sandbox
HROOT51G="$(mktemp -d)"; mkdir -p "$HROOT51G/.claude"
printf 'agent_harnesses: [claude]\n' > "$SBX/.docket.yml"
printf 'agents:\n\tdefault:\n\t\tstatus: { model: tab-m }\n' > "$SBX/.docket.local.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT51G" bash "$SYNC" >/dev/null 2>&1 )
assert "0051 4L: tab-indented local YAML resolves" '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "tab-m" ]'
rm -rf "$SBX" "$HROOT51G"

# (rider) prune_orphans empty-scan_dirs guard: bash 3.2 + set -u with NO harness roots
# on disk AND no opt-in must not crash ("${scan_dirs[@]}" on an empty array).
SBXR="$(mktemp -d)"                                   # deliberately NO .claude/.agents dirs
rid_rc=0
( cd "$SBXR" && DOCKET_HARNESS_ROOT="$SBXR" /bin/bash "$SYNC" >/dev/null 2>&1 ) || rid_rc=$?
assert "0051 rider: empty scan_dirs run succeeds under /bin/bash (rc=0)" '[ "$rid_rc" = "0" ]'
rm -rf "$SBXR"

# ---- change 0168: the shipped sidecar is the lowest layer -------------------
# Outcome asserts only: they pin WHAT resolves, not WHERE it came from. While the
# sources still carry model:/effort: these are green either way — the mechanism
# (the resolver actually reading agents/harness-defaults.yml) is proved separately
# by pointing the resolver at a sentinel sidecar, and permanently by Task 4's
# deletion of the source frontmatter.
make_sandbox
HROOT168="$(mktemp -d)"; mkdir -p "$HROOT168/.claude"
cat > "$SBX/.docket.yml" <<'YML'
agent_harnesses: [claude]
agents:
  claude:
    adr: { effort: high }
YML
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT168" bash "$SYNC" >/dev/null 2>&1 )
A="$SBX/.claude/agents/docket-adr.md"
assert "0168: unconfigured agent takes the shipped claude model" \
  '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "claude-haiku-4-5-20251001" ]'
assert "0168: a user effort override beats the shipped effort" '[ "$(fm "$A" effort)" = "high" ]'
assert "0168: the un-overridden field still comes from the sidecar" \
  '[ "$(fm "$A" model)" = "claude-opus-5" ]'
rm -rf "$SBX" "$HROOT168"

# 0168 fail-before-write gate: an invalid sidecar aborts the run with a named diagnostic
# and leaves ZERO wrappers behind — never a half-regenerated agent directory.
make_sandbox
HROOT168B="$(mktemp -d)"; mkdir -p "$HROOT168B/.claude"
SCR168="$(mktemp -d)"; cp -R "$REPO/agents" "$REPO/cursor-rules" "$REPO/scripts" "$REPO/sync-agents.sh" "$SCR168/"
printf 'agent_harnesses: [claude]\n' > "$SBX/.docket.yml"
printf '    phantom-not-a-wrapper: { model: x, effort: low }\n' >> "$SCR168/agents/harness-defaults.yml"
hd_rc=0
hd_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT168B" bash "$SCR168/sync-agents.sh" 2>&1 >/dev/null)" || hd_rc=$?
assert "0168 gate: invalid sidecar fails the run (rc!=0)"  '[ "$hd_rc" != "0" ]'
assert "0168 gate: the diagnostic names harness-defaults"  'printf "%s" "$hd_err" | grep -q "harness-defaults"'
assert "0168 gate: no per-repo wrapper was written"        '[ "$(find "$SBX" -name "docket-*.md" -path "*/agents/*" | wc -l | tr -d " ")" = "0" ]'
assert "0168 gate: no user-level wrapper was written"      '[ "$(find "$HROOT168B" -name "docket-*.md" | wc -l | tr -d " ")" = "0" ]'
rm -rf "$SBX" "$HROOT168B" "$SCR168"

# ============================================================================
# Change 0051/0057 — managed .gitignore block (# docket:start/end; mechanics now
# live in scripts/lib/docket-gitignore-block.sh, sourced by sync-agents.sh)
# ============================================================================

# (gi-a) opted-in repo: block created (file didn't exist), loud "commit" notice,
# patterns strictly docket-scoped, emitted from the harness table (all 6 tokens).
make_sandbox
HROOTGA="$(mktemp -d)"; mkdir -p "$HROOTGA/.claude"
printf 'agent_harnesses: [claude]\n' > "$SBX/.docket.yml"
gi_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTGA" bash "$SYNC" 2>&1 >/dev/null)"
GI="$SBX/.gitignore"
assert "0051 gi: .gitignore created with the managed block" 'grep -q "^# docket:start" "$GI" && grep -q "^# docket:end$" "$GI"'
assert "0051 gi: block ignores .docket.local.yml"            'grep -q "^\.docket\.local\.yml$" "$GI"'
assert "0051 gi: block ignores claude agents pattern"        'grep -q "^\.claude/agents/docket-\*\.md$" "$GI"'
assert "0051 gi: block ignores cursor agents pattern"        'grep -q "^\.cursor/agents/docket-\*\.md$" "$GI"'
assert "0051 gi: block ignores the cursor dispatch rule"     'grep -q "^\.cursor/rules/docket-dispatch\.mdc$" "$GI"'
assert "0051 gi: loud commit-this notice"                    'printf "%s" "$gi_err" | grep -qi "commit"'
assert "0051 gi: every block line is docket-scoped (starts with . or #)" \
  '! awk "/# docket:start/,/# docket:end/" "$GI" | grep -qvE "^(#|\.)"'

# (gi-b) idempotent: second run leaves .gitignore byte-identical and prints no notice.
gi_before="$(cat "$GI")"
gi_err2="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTGA" bash "$SYNC" 2>&1 >/dev/null)"
assert "0051 gi: second run byte-identical"    '[ "$gi_before" = "$(cat "$GI")" ]'
assert "0051 gi: second run no UPDATED notice" '! printf "%s" "$gi_err2" | grep -q "managed block"'

# (gi-c) hand-edit inside the block repaired; content OUTSIDE the markers preserved.
printf 'my-own-ignore/\n%s\n' "$(cat "$GI")" > "$GI"          # user content above the block
sed -i.bak '/docket-dispatch/d' "$GI"; rm -f "$GI.bak"        # vandalize the block
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTGA" bash "$SYNC" >/dev/null 2>&1 )
assert "0051 gi: hand-edited block repaired"   'grep -q "docket-dispatch" "$GI"'
assert "0051 gi: user content preserved"       'grep -q "^my-own-ignore/$" "$GI"'
assert "0051 gi: exactly one block after repair" '[ "$(grep -c "^# docket:start" "$GI")" = "1" ]'
rm -rf "$SBX" "$HROOTGA"

# (gi-d) tracking-only repo WITH a .docket.local.yml that has NO opt-in keys: the block
# is still written (the local file itself must never be committable); zero agent files.
make_sandbox
HROOTGD="$(mktemp -d)"; mkdir -p "$HROOTGD/.claude"
printf 'metadata_branch: docket\n' > "$SBX/.docket.yml"
printf 'finalize:\n  gate: off\n' > "$SBX/.docket.local.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTGD" bash "$SYNC" >/dev/null 2>&1 )
assert "0051 gi: local-file-present repo gets the block"  'grep -q "^# docket:start" "$SBX/.gitignore"'
assert "0051 gi: but still generates zero agent files"    '[ ! -e "$SBX/.claude/agents/docket-status.md" ]'
rm -rf "$SBX" "$HROOTGD"

# (gi-e) repo with NEITHER signal: .gitignore never touched/created (LEARNINGS #48 posture).
make_sandbox
HROOTGE="$(mktemp -d)"; mkdir -p "$HROOTGE/.claude"
printf 'metadata_branch: docket\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTGE" bash "$SYNC" >/dev/null 2>&1 )
assert "0051 gi: no-signal repo gets NO .gitignore" '[ ! -e "$SBX/.gitignore" ]'
rm -rf "$SBX" "$HROOTGE"

# (gi-core) the block now carries the three core docket-owned entries (change 0057).
make_sandbox
HROOTGC="$(mktemp -d)"; mkdir -p "$HROOTGC/.claude"
printf 'agent_harnesses: [claude]\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTGC" bash "$SYNC" >/dev/null 2>&1 )
GI="$SBX/.gitignore"
assert "0057 gi: block carries .docket/"              'grep -qxF ".docket/" "$GI"'
assert "0057 gi: block carries .worktrees/"           'grep -qxF ".worktrees/" "$GI"'
assert "0057 gi: block carries settings.local.json"   'grep -qxF ".claude/settings.local.json" "$GI"'
assert "0057 gi: new start marker, no legacy marker"  'grep -qxF "# docket:start (managed by docket — do not hand-edit)" "$GI" && ! grep -qF "docket:generated" "$GI"'
rm -rf "$SBX" "$HROOTGC"

# (gi-widen+) widened trigger POSITIVE: a tracking-only repo (NOT opted in, no local file) that
# HAS a local docket branch heals the block (the bootstrap guard's DOCKET probe).
mkgitrepo
HROOTGW="$(mktemp -d)"; mkdir -p "$HROOTGW/.claude"
printf 'metadata_branch: docket\n' > "$SBX/.docket.yml"        # tracking-only, not opted in
git -C "$SBX" add -A; git -C "$SBX" commit --quiet -m init
git -C "$SBX" branch docket                                    # DOCKET signal present
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTGW" bash "$SYNC" >/dev/null 2>&1 )
assert "0057 gi: docket-branch repo heals the block"  'grep -qxF "# docket:start (managed by docket — do not hand-edit)" "$SBX/.gitignore"'
assert "0057 gi: but still generates zero agent files" '[ ! -e "$SBX/.claude/agents/docket-status.md" ]'
rm -rf "$SBX" "$HROOTGW"

# (gi-widen-) widened trigger NEGATIVE (the 0048 regression): a repo with NO docket signal
# (no opt-in, no .docket.local.yml, no docket branch, no existing block) is untouched.
mkgitrepo
HROOTGN="$(mktemp -d)"; mkdir -p "$HROOTGN/.claude"
printf 'metadata_branch: docket\n' > "$SBX/.docket.yml"
git -C "$SBX" add -A; git -C "$SBX" commit --quiet -m init
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTGN" bash "$SYNC" >/dev/null 2>&1 )
assert "0057 gi: no-signal repo gets NO .gitignore" '[ ! -e "$SBX/.gitignore" ]'
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTGN" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0057 gi: no-signal repo --check stays a no-op (rc=0)" '[ "$chk_rc" = "0" ]'
rm -rf "$SBX" "$HROOTGN"

# (gi-heal-present) heal-if-present: a repo carrying only a legacy block (no other signal) is
# UPGRADED to the new block.
mkgitrepo
HROOTGH="$(mktemp -d)"; mkdir -p "$HROOTGH/.claude"
printf 'metadata_branch: docket\n' > "$SBX/.docket.yml"
printf '# docket:generated:start (managed by sync-agents.sh — do not hand-edit)\n.docket.local.yml\n# docket:generated:end\n' > "$SBX/.gitignore"
git -C "$SBX" add -A; git -C "$SBX" commit --quiet -m init
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTGH" bash "$SYNC" >/dev/null 2>&1 )
assert "0057 gi: legacy-only repo upgraded to new block" 'grep -qxF "# docket:start (managed by docket — do not hand-edit)" "$SBX/.gitignore" && ! grep -qF "docket:generated" "$SBX/.gitignore"'
rm -rf "$SBX" "$HROOTGH"

# (gi-f) UNTERMINATED block (start marker, no end): refuse to rewrite, warn, preserve
# every byte — user content after the dangling marker must survive.
make_sandbox
HROOTGF="$(mktemp -d)"; mkdir -p "$HROOTGF/.claude"
printf 'agent_harnesses: [claude]\n' > "$SBX/.docket.yml"
printf '# docket:start (managed by docket — do not hand-edit)\n.docket.local.yml\nnode_modules/\n' > "$SBX/.gitignore"
gi_before="$(cat "$SBX/.gitignore")"
gf_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTGF" bash "$SYNC" 2>&1 >/dev/null)"; gf_rc=$?
assert "0051 gi-f: unterminated block run still succeeds (rc=0)" '[ "$gf_rc" = "0" ]'
assert "0051 gi-f: warns the block is corrupt/unterminated" 'printf "%s" "$gf_err" | grep -qi "untermin\|corrupt"'
assert "0051 gi-f: file left byte-identical (user content preserved)" '[ "$gi_before" = "$(cat "$SBX/.gitignore")" ]'
rm -rf "$SBX" "$HROOTGF"

# ============================================================================
# Change 0051 — migration (0048-era tracked wrappers) + --check three legs
# ============================================================================

# (mkgitrepo defined earlier, alongside make_sandbox, so the 0057 widened-trigger tests above
# can use it too.)

# (mig-a) 0048-era repo: tracked wrappers + rule -> deleted from the worktree, block
# written, local set regenerated, single migration commit printed. Idempotent.
mkgitrepo
HROOTM="$(mktemp -d)"; mkdir -p "$HROOTM/.claude"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
mkdir -p "$SBX/.claude/agents" "$SBX/.cursor/agents" "$SBX/.cursor/rules"
printf 'stale 0048 wrapper\n' > "$SBX/.claude/agents/docket-status.md"
printf 'stale 0048 wrapper\n' > "$SBX/.cursor/agents/docket-status.md"
printf 'stale 0048 rule\n'    > "$SBX/.cursor/rules/docket-dispatch.mdc"
git -C "$SBX" add -A; git -C "$SBX" commit --quiet -m "0048-era state"
mig_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTM" bash "$SYNC" 2>&1 >/dev/null)"; mig_rc=$?
assert "0051 mig: run succeeds (rc=0)"                     '[ "$mig_rc" = "0" ]'
assert "0051 mig: announces the migration"                 'printf "%s" "$mig_err" | grep -qi "migrat"'
assert "0051 mig: prints git rm --cached instructions"     'printf "%s" "$mig_err" | grep -q -e "git rm" '
assert "0051 mig: gitignore block written"                 'grep -q "^# docket:start" "$SBX/.gitignore"'
assert "0051 mig: local files regenerated (fresh content)" 'grep -q "^model: sonnet" "$SBX/.claude/agents/docket-status.md"'
assert "0051 mig: full local set regenerated"              '[ "$(find "$SBX/.claude/agents" -name "docket-*.md" | wc -l | tr -d " ")" = "16" ]'
# perform the printed migration commit; second run must NOT re-announce
( cd "$SBX" && git rm -r -q --cached '.claude/agents/docket-*.md' '.cursor/agents/docket-*.md' '.cursor/rules/docket-dispatch.mdc' && git add .gitignore && git commit -q -m "docket: agent files go machine-local" )
mig_err2="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTM" bash "$SYNC" 2>&1 >/dev/null)"
assert "0051 mig: idempotent — post-commit run is silent about migration" '! printf "%s" "$mig_err2" | grep -qi "migrat"'
# and --check is fully green now (all three legs)
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTM" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0051 mig: post-migration --check green (rc=0)" '[ "$chk_rc" = "0" ]'
rm -rf "$SBX" "$HROOTM"

# (mig-b) stale tracked wrappers in a repo with NO current opt-in and no .gitignore:
# the printed remedy must be runnable AS PRINTED (no git add .gitignore clause).
mkgitrepo
HROOTMB="$(mktemp -d)"; mkdir -p "$HROOTMB/.claude"
printf 'metadata_branch: docket\n' > "$SBX/.docket.yml"        # tracking-only: NOT opted in
mkdir -p "$SBX/.claude/agents"
printf 'stale 0048 wrapper\n' > "$SBX/.claude/agents/docket-status.md"
git -C "$SBX" add -A; git -C "$SBX" commit --quiet -m "0048-era stale state"
migb_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTMB" bash "$SYNC" 2>&1 >/dev/null)"; migb_rc=$?
assert "0051 mig-b: run succeeds (rc=0)"                      '[ "$migb_rc" = "0" ]'
assert "0051 mig-b: remedy omits git add .gitignore"          'printf "%s" "$migb_err" | grep -e "git rm" | grep -v -q "git add .gitignore"'
assert "0051 mig-b: no .gitignore was created (not wanted)"   '[ ! -e "$SBX/.gitignore" ]'
# the printed remedy must actually run: extract and eval it, then leg (b) goes green.
remedy="$(printf '%s\n' "$migb_err" | sed -n 's/^sync-agents:[[:space:]]*\(git rm .*\)$/\1/p' | head -n1)"
assert "0051 mig-b: a runnable remedy line was printed"       '[ -n "$remedy" ]'
( cd "$SBX" && eval "$remedy" ) >/dev/null 2>&1
migb_chk="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTMB" bash "$SYNC" --check 2>&1)"; migb_chk_rc=$?
assert "0051 mig-b: after running the printed remedy, --check leg (b) green (rc=0)" '[ "$migb_chk_rc" = "0" ]'
rm -rf "$SBX" "$HROOTMB"

# (chk-a) leg (a): opted-in repo, block missing (sync never ran) -> rc!=0 naming the block.
make_sandbox
HROOTCA="$(mktemp -d)"; mkdir -p "$HROOTCA/.claude"
printf 'agents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTCA" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0051 chk-a: missing block fails --check (rc!=0)" '[ "$chk_rc" != "0" ]'
assert "0051 chk-a: names the gitignore block"           'printf "%s" "$chk_out" | grep -qi "gitignore"'
# stale block (hand-pruned pattern) also fails:
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTCA" bash "$SYNC" >/dev/null 2>&1 )
sed -i.bak '/docket-dispatch/d' "$SBX/.gitignore"; rm -f "$SBX/.gitignore.bak"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTCA" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0051 chk-a: stale block fails --check (rc!=0)"   '[ "$chk_rc" != "0" ]'
rm -rf "$SBX" "$HROOTCA"

# (chk-b) leg (b): tracked generated file -> rc!=0 with the migration remedy.
mkgitrepo
HROOTCB="$(mktemp -d)"; mkdir -p "$HROOTCB/.claude"
printf 'agents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTCB" bash "$SYNC" >/dev/null 2>&1 )   # block + local files
git -C "$SBX" add -A -f; git -C "$SBX" commit --quiet -m "wrongly track everything"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTCB" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0051 chk-b: tracked generated file fails --check (rc!=0)" '[ "$chk_rc" != "0" ]'
assert "0051 chk-b: names a tracked path"                          'printf "%s" "$chk_out" | grep -q "docket-status.md"'
rm -rf "$SBX" "$HROOTCB"

# (chk-c) leg (c): content staleness is ADVISORY — rc stays 0, output says advisory.
make_sandbox
HROOTCC="$(mktemp -d)"; mkdir -p "$HROOTCC/.claude"
printf 'agents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTCC" bash "$SYNC" >/dev/null 2>&1 )
sed -i.bak 's/^model: sonnet/model: haiku/' "$SBX/.claude/agents/docket-status.md"; rm -f "$SBX/.claude/agents/docket-status.md.bak"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTCC" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0051 chk-c: content drift is advisory (rc=0)"  '[ "$chk_rc" = "0" ]'
assert "0051 chk-c: advisory names the drifted file"   'printf "%s" "$chk_out" | grep -q "advisory" && printf "%s" "$chk_out" | grep -q "docket-status.md"'
rm -rf "$SBX" "$HROOTCC"

# (chk-d) fresh clone of a MIGRATED repo: committed .docket.yml (opted-in) + committed
# block, NO generated files -> --check fully green (leg c vacuous on CI).
mkgitrepo
HROOTCD="$(mktemp -d)"; mkdir -p "$HROOTCD/.claude"
printf 'agents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTCD" bash "$SYNC" >/dev/null 2>&1 )     # writes block + files
find "$SBX" -name 'docket-*.md' -path '*/agents/*' -delete                       # simulate the fresh clone
git -C "$SBX" add .docket.yml .gitignore; git -C "$SBX" commit --quiet -m "migrated repo"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTCD" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0051 chk-d: fresh migrated clone --check green (rc=0)" '[ "$chk_rc" = "0" ]'
rm -rf "$SBX" "$HROOTCD"

# ---- change 0079: runner delegation shims -----------------------------------------
# registry <-> adapters parity, BOTH directions (consuming-surface guard, LEARNINGS (d))
REGISTRY_LINE="$(grep -E '^REGISTERED_RUNNERS=' "$SYNC")"
REGISTRY_LINE="$(head -n1 <<<"$REGISTRY_LINE")"
assert "0079: sync-agents declares REGISTERED_RUNNERS" '[ -n "$REGISTRY_LINE" ]'
runners_from_registry="$(sed -E 's/^REGISTERED_RUNNERS="([^"]*)".*/\1/' <<<"$REGISTRY_LINE")"
n_registry_tokens=0
for r in $runners_from_registry; do
  assert "0079: registry token '$r' has an adapter script" '[ -f "$REPO/scripts/runners/'"$r"'.sh" ]'
  n_registry_tokens=$((n_registry_tokens+1))
done
# NON-VACUITY (change 0205): the registry->adapter loop above derives its population from a sed
# extraction of the REGISTERED_RUNNERS line. An extractor that returned the empty string would run
# that loop zero times and leave every assert inside it unexecuted, which reads exactly like parity
# holding. Pin the population instead of trusting the silence.
assert "0205: the registry->adapter loop actually enumerated the registry (got $n_registry_tokens)" \
  '[ "$n_registry_tokens" -ge 3 ]'
for a in "$REPO"/scripts/runners/*.sh; do
  [ -e "$a" ] || continue
  tok="$(basename "$a" .sh)"
  assert "0079: adapter '$tok' is in REGISTERED_RUNNERS" 'case " $runners_from_registry " in *" '"$tok"' "*) true;; *) false;; esac'
done

# shim generation: agents.claude.<agent>.runner: codex swaps the BODY for the shim
mkgitrepo
mkdir -p "$SBX/.claude"
printf 'agents:\n  claude:\n    status: { model: gpt-5.1-codex, effort: high, runner: codex }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
G="$SBX/.claude/agents/docket-status.md"
assert "0079: shim keeps frontmatter model (bookkeeping)" '[ "$(fm "$G" model)" = "gpt-5.1-codex" ]'
assert "0079: shim body invokes docket.sh runner-dispatch" 'grep -qF "docket.sh runner-dispatch" "$G"'
assert "0079: shim body pins --runner codex" 'grep -qF -- "--runner codex" "$G"'
assert "0079: shim body pins --agent status" 'grep -qF -- "--agent status" "$G"'
assert "0079: shim body bakes the resolved model" 'grep -qF -- "--model gpt-5.1-codex" "$G"'
assert "0079: shim body bakes the resolved effort" 'grep -qF -- "--effort high" "$G"'
assert "0079: shim body demands ONE foreground call" 'grep -qi "one foreground" "$G"'
assert "0079: shim body forbids the inline fallback" 'grep -qiE "never.*inline" "$G"'
assert "0079: shim replaced the native body" '! grep -qF "Execute docket-status to refresh docket state" "$G"'
assert "0079: exactly one dispatch invocation in the shim" '[ "$(grep -cF "docket.sh runner-dispatch" "$G")" = "1" ]'
# unlisted agent stays native in the same repo
assert "0079: agent without runner: stays native" 'grep -qF "abort-and-report" "$SBX/.claude/agents/docket-adr.md" && ! grep -qF "runner-dispatch" "$SBX/.claude/agents/docket-adr.md"'
# effort auto + runner => no --effort flag in the shim
printf 'agents:\n  claude:\n    status: { model: gpt-5.1-codex, effort: auto, runner: codex }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
assert "0079: effort auto omits --effort from the shim" '! grep -qF -- "--effort" "$G"'
# --check leg (c): a de-shimmed wrapper is advisory drift (proves leg c shares emit_wrapper).
# The de-shim uses the EXACT bytes bare emit would produce (same model, no runner) — junk
# bytes would drift under either emission path and let a leg-(c)-bypasses-shim mutant survive.
printf 'agents:\n  claude:\n    status: { model: gpt-5.1-codex }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
cp "$G" "$SBX/native-status.md"
printf 'agents:\n  claude:\n    status: { model: gpt-5.1-codex, runner: codex }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
assert "0079: fixture sanity — shim and native differ" '! diff -q "$G" "$SBX/native-status.md" >/dev/null'
cp "$SBX/native-status.md" "$G"
chk="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check 2>&1 )"
assert "0079: --check flags a de-shimmed wrapper as drift" 'grep -qF "drift in .claude/agents/docket-status.md" <<<"$chk"'
rm -rf "$SBX"

# ---- change 0206: build-* shims bake --worktree as a required slot -------------------
# BIDIRECTIONAL by construction (LEARNINGS: correspondence-guard-runs-one-way). This is a MIRROR
# correspondence, not a subset: build-* shims must carry the flag AND non-build shims must not, so
# a future change that widens the flag to every shim reddens just as a change that drops it does.
# Placed AFTER the leg-(c) block's `rm -rf "$SBX"`: this block mints its own fixture, and the
# leg-(c) asserts above still read the previous fixture's $G.
mkgitrepo
mkdir -p "$SBX/.claude"
printf 'agents:\n  claude:\n    build-economy: { model: test-model-x, runner: codex }\n    status: { model: test-model-y, runner: codex }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
B="$SBX/.claude/agents/docket-build-economy.md"
S="$SBX/.claude/agents/docket-status.md"

# NON-VACUITY FLOOR: every assert below reads these two files, so prove BOTH are real shims first.
# Without this, a generation failure leaves absent files and the negative asserts pass by default —
# which reads exactly like the property holding.
assert "0206: fixture sanity — build-economy generated a real shim" \
  'grep -qF "docket.sh runner-dispatch" "$B"'
assert "0206: fixture sanity — status generated a real shim" \
  'grep -qF "docket.sh runner-dispatch" "$S"'

# Direction 1: a build-* shim CARRIES the slot. The dispatch line is captured into a variable
# before it is searched — a `grep … | grep -q` pipeline would SIGPIPE the producer under pipefail.
dispatch_line="$(grep -F "docket.sh runner-dispatch" "$B")"
assert "0206: build-* shim bakes --worktree into the dispatch line" \
  'grep -qF -- "--worktree" "$B"'
assert "0206: build-* shim keeps the slot on the runner-dispatch line itself" \
  'grep -qF -- "--worktree" <<<"$dispatch_line"'
assert "0206: build-* shim tells the caller to abort rather than guess a path" \
  'grep -qiF "abort-and-report" "$B"'

# Direction 2: a non-build shim does NOT. (grep -qF -- is mandatory: a bare leading `--` is parsed
# as an option, exit 2, which inside this negation would be permanently, vacuously green.)
assert "0206: non-build shim carries no --worktree" '! grep -qF -- "--worktree" "$S"'
assert "0206: exactly one dispatch invocation in the build-* shim" \
  '[ "$(grep -cF "docket.sh runner-dispatch" "$B")" = "1" ]'
rm -rf "$SBX"

# runner under a NON-claude harness key: warned-and-ignored (reserved), file stays native
mkgitrepo
mkdir -p "$SBX/.claude" "$SBX/.cursor"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  cursor:\n    status: { model: gpt-5.5-medium-fast, runner: codex }\n' > "$SBX/.docket.yml"
warn="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" 2>&1 >/dev/null )"
assert "0079: non-claude runner warns (reserved)" 'grep -qiE "runner.*reserved" <<<"$warn"'
assert "0079: non-claude wrapper stays native" '! grep -qF "runner-dispatch" "$SBX/.cursor/agents/docket-status.md"'
rm -rf "$SBX"

# unregistered runner under claude: loud generation-time ERROR (nonzero)
mkgitrepo
mkdir -p "$SBX/.claude"
printf 'agents:\n  claude:\n    status: { runner: gemini-cli }\n' > "$SBX/.docket.yml"
err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" 2>&1 >/dev/null )"; rc=$?
assert "0079: unregistered runner fails generation nonzero" '[ "$rc" != "0" ]'
assert "0079: unregistered-runner error names it" 'grep -qF "gemini-cli" <<<"$err"'
rm -rf "$SBX"

# ---- change 0205: a runner: with no USER-configured model is a generation-time ERROR ----
# Runner-wide, not opencode-only. Under change 0168 a shipped agents/harness-defaults.yml value is
# never forwarded to a child harness, so a model-less delegation silently ran on the CHILD's own
# default — of unknown identity and, on a pay-per-token backend like OpenRouter, unknown cost. The
# failure surfaced on the bill, not in the run. Raised at generation time, where the config was
# just written, rather than mid-dispatch. Asserted on ALL THREE runners because this is a framework
# rule, not an adapter behavior: a per-adapter implementation would be the defect it replaces.
for rnr in codex cursor opencode; do
  mkgitrepo
  mkdir -p "$SBX/.claude"
  printf 'agents:\n  claude:\n    status: { runner: %s }\n' "$rnr" > "$SBX/.docket.yml"
  err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" 2>&1 >/dev/null )"; rc=$?
  assert "0205/$rnr: model-less runner fails generation nonzero" '[ "$rc" != "0" ]'
  assert "0205/$rnr: the diagnostic names the agent"  'grep -qF "docket-status" <<<"$err"'
  assert "0205/$rnr: the diagnostic names the runner" 'grep -qF "'"$rnr"'" <<<"$err"'
  assert "0205/$rnr: the diagnostic says a model is required" 'grep -qiE "model" <<<"$err"'
  # The error must not leave a USABLE shim behind. Note this is deliberately `! -s` (absent OR
  # empty), NOT `! -e`: emit_wrapper's call sites redirect into the target path, so the shell
  # creates and truncates the file BEFORE the function body runs and exits. The offending agent is
  # therefore left with a zero-length wrapper, which is inert — the harness has nothing to
  # dispatch — and is overwritten on the next successful run. Asserting `! -e` here would be
  # asserting a fail-before-write property this rule does not have.
  assert "0205/$rnr: no usable wrapper was written for the offending agent" \
    '[ ! -s "$SBX/.claude/agents/docket-status.md" ]'
  # NON-VACUITY COMPANION: the same fixture WITH a user model must succeed and emit a real shim.
  # Without this, every assert above stays green if sync-agents.sh broke for an unrelated reason.
  printf 'agents:\n  claude:\n    status: { runner: %s, model: some/model-id }\n' "$rnr" > "$SBX/.docket.yml"
  ( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 ); rc2=$?
  assert "0205/$rnr: the SAME fixture with a user model succeeds (the guard above is not vacuous)" '[ "$rc2" = "0" ]'
  assert "0205/$rnr: and emits a real shim baking that model" \
    'grep -qF -- "--model some/model-id" "$SBX/.claude/agents/docket-status.md"'
  rm -rf "$SBX"
done

# `model: inherit` is docket's own NO-PIN sentinel — every adapter normalizes it to "no flag", so
# accepting it here would leave a one-word bypass around the rule: the child would run on its own
# default exactly as if no model had been written at all.
mkgitrepo
mkdir -p "$SBX/.claude"
printf 'agents:\n  claude:\n    status: { runner: codex, model: inherit }\n' > "$SBX/.docket.yml"
err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" 2>&1 >/dev/null )"; rc=$?
assert "0205: model: inherit does not satisfy the required-model rule" '[ "$rc" != "0" ]'
assert "0205: the inherit diagnostic still names the agent" 'grep -qF "docket-status" <<<"$err"'
rm -rf "$SBX"

# ORDERING FENCE: the registration check must still fire FIRST. This fixture is model-less AND
# unregistered; if the model check ran first, the diagnostic would stop naming the runner and the
# change-0079 unregistered-runner assert would break for the wrong reason.
mkgitrepo
mkdir -p "$SBX/.claude"
printf 'agents:\n  claude:\n    status: { runner: gemini-cli }\n' > "$SBX/.docket.yml"
err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" 2>&1 >/dev/null )"
assert "0205: an unregistered AND model-less runner still reports the REGISTRATION failure first" \
  'grep -qF "gemini-cli" <<<"$err" && grep -qiE "not a registered runner" <<<"$err"'
rm -rf "$SBX"

# A non-claude harness carrying runner: is warned-and-ignored (reserved) and emits NATIVE — the
# required-model rule must not fire there, because no delegation happens.
mkgitrepo
mkdir -p "$SBX/.claude" "$SBX/.cursor"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  cursor:\n    status: { runner: codex }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 ); rc=$?
assert "0205: a reserved (non-claude) runner does NOT trip the required-model rule" '[ "$rc" = "0" ]'
assert "0205: and its wrapper is still native" '! grep -qF "runner-dispatch" "$SBX/.cursor/agents/docket-status.md"'
rm -rf "$SBX"

# no runner config anywhere: native (un-shimmed) output (regression fence)
make_sandbox
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
# Change 0168: byte identity with the source is structurally impossible (the pin is injected, not
# copied). What this fence guards is that a repo with NO runner config anywhere gets the NATIVE
# wrapper — source body verbatim, no delegation shim — carrying the shipped pin.
assert "0079: no-runner repo output stays native (source body verbatim, no shim)" \
  'diff -q <(body_of "$REPO/agents/docket-status.md") <(body_of "$SBX/.claude/agents/docket-status.md") >/dev/null &&
   ! grep -qF "runner-dispatch" "$SBX/.claude/agents/docket-status.md"'
assert "0079: no-runner repo output carries the shipped pin" \
  '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "$(hd_field "$HD" claude status model)" ]'
rm -rf "$SBX"

# ---- change 0168: emit() inserts exactly one model:/effort: line -------------
# emit() is strip-then-insert now: it drops any model:/effort: the SOURCE still carries and injects
# the resolved pair before the closing fence. The failure mode that rewrite introduces is a
# DUPLICATED key (insert without strip), which no earlier assert could see because substitution
# cannot duplicate. Counted inside the FIRST frontmatter block — a whole-file count would also see
# body prose (AGENTS.md: anchor a frontmatter read to the first ---…--- block) — plus a whole-file
# count, so an insertion that lands outside the block is caught too.
fm_key_count(){  # $1=file $2=key -> occurrences of `^<key>:` inside the first --- block
  awk -v k="$2" '/^---[[:space:]]*$/{ d++; if (d>=2) exit; next }
                 d==1 && $0 ~ "^"k"[[:space:]]*:" { n++ }
                 END { print n+0 }' "$1"
}
mkgitrepo
mkdir -p "$SBX/.claude"
printf 'agent_harnesses: [claude]\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
for w in docket-status docket-implement-next docket-build-max; do
  G="$SBX/.claude/agents/$w.md"
  assert "0168: $w emits exactly one model: line in the frontmatter" \
    '[ "$(fm_key_count "$G" model)" = "1" ]'
  assert "0168: $w emits exactly one effort: line in the frontmatter" \
    '[ "$(fm_key_count "$G" effort)" = "1" ]'
  assert "0168: $w emits exactly one model: line in the whole file" \
    '[ "$(grep -c "^model:" "$G")" = "1" ]'
  assert "0168: $w emits exactly one effort: line in the whole file" \
    '[ "$(grep -c "^effort:" "$G")" = "1" ]'
done
rm -rf "$SBX"

# ---- `model: inherit` is a CLAUDE VALUE, not a cross-harness sentinel -------
# 0168 whole-branch review, IMPORTANT 2. `inherit` is a documented Claude Code frontmatter value
# meaning "run this subagent on the parent conversation's model"; Claude Code reads it and acts on
# it. It is NOT a docket sentinel there. Cursor and Codex have no such value, so their emitters
# (both pre-0168) normalize it to "emit no pin" — the harness then applies its own default.
# Change 0168's rewritten emit() briefly folded the Cursor sentinel into the SHARED emitter, which
# silently turned `model: inherit` into NO model: line on Claude — a different runtime meaning
# (parent's model vs. Claude Code's own subagent default) on the one harness the change promised
# to leave byte-for-byte alone. These asserts pin the split: verbatim on claude, dropped elsewhere.
make_sandbox
mkdir -p "$SBX/.cursor" "$SBX/.codex"
HROOTINH="$(mktemp -d)"; mkdir -p "$HROOTINH/.claude"
cat > "$SBX/.docket.yml" <<'YML'
agent_harnesses: [claude, cursor, codex]
agents:
  default:
    status: { model: inherit, effort: medium }
YML
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOTINH" bash "$SYNC" >/dev/null 2>&1 )
assert "inherit: claude emits it VERBATIM (Claude Code's 'use the parent's model')" \
  '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "inherit" ]'
assert "inherit: claude still emits the configured effort alongside it" \
  '[ "$(fm "$SBX/.claude/agents/docket-status.md" effort)" = "medium" ]'
assert "inherit: cursor emits NO model: line (no such Cursor value)" \
  '! grep -q "^model:" "$SBX/.cursor/agents/docket-status.md"'
assert "inherit: codex emits NO model = line (no such Codex value)" \
  '! grep -q "^model = " "$SBX/.codex/agents/docket-status.toml"'
rm -rf "$SBX" "$HROOTINH"

# ---- change 0168: a shipped default never becomes a child-runner flag -------
# The provenance boundary. `runner:` delegates this agent to a DIFFERENT harness's CLI, so the
# baked --model/--effort flags are read by that child, not by Claude. A shipped
# agents/harness-defaults.yml value is a CLAUDE default; baking it into a Codex dispatch sends a
# Claude model ID to a Codex child. Only a USER-configured value may cross that boundary — the
# resolved pair still pins the wrapper's own native frontmatter, which is bookkeeping for the
# Claude parent and never reaches the child.
#
# CHANGE 0205 NARROWED THIS TEST'S REACH, and the narrowing is a strengthening, not lost coverage.
# The fixture used to configure a bare `runner: codex` with no model; that is now a generation-time
# ERROR (the required-model rule), which makes the MODEL half of the provenance leak structurally
# impossible rather than merely guarded — a shipped model can no longer be the resolved-and-baked
# value under a runner, because a user model is mandatory. What remains reachable, and is therefore
# what this block now pins, is the EFFORT half: effort stays optional, so a user who configures only
# a model must still not have the shipped effort forwarded to the child.
mkgitrepo
mkdir -p "$SBX/.claude"
HROOT168F="$(mktemp -d)"; mkdir -p "$HROOT168F/.claude"
cat > "$SBX/.docket.yml" <<'YML'
agent_harnesses: [claude]
agents:
  claude:
    status: { runner: codex, model: user-picked-id }
YML
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT168F" bash "$SYNC" >/dev/null 2>&1 )
S="$SBX/.claude/agents/docket-status.md"
# Fixture sanity FIRST: without a real shim the negative asserts below are vacuous.
assert "0168: a user-model runner config emits a shim" 'grep -qF "docket.sh runner-dispatch" "$S"'
assert "0168: the shim names the runner" 'grep -qF -- "--runner codex" "$S"'
assert "0168: the USER model is baked (the provenance rule's positive half)" \
  'grep -qF -- "--model user-picked-id" "$S"'
assert "0168: no user effort configured => NO --effort flag baked" '! grep -qF -- "--effort" "$S"'
# 0169 (re-pointed by 0205): the sidecar supplies an EFFORT for this very agent on both the parent
# and the child harness, so the negative assert above is non-vacuous in the direction that matters
# — a shipped effort could be baked into the flags if provenance were ignored. Pin the fixture's
# premise so a future emptying of either block cannot quietly re-vacuum it.
assert "0169: the claude sidecar really does supply an effort for this agent (the guard above is not vacuous)" \
  '[ -n "$(hd_field "$HD" claude status effort)" ]'
assert "0169: the codex sidecar really does supply an effort for this agent" \
  '[ -n "$(hd_field "$HD" codex status effort)" ]'
assert "0169: and neither shipped EFFORT leaked into the runner flags" \
  '! grep -qF -- "--effort $(hd_field "$HD" claude status effort)" "$S" && ! grep -qF -- "--effort $(hd_field "$HD" codex status effort)" "$S"'
assert "0169: nor did the shipped CODEX model" \
  '! grep -qF -- "$(hd_field "$HD" codex status model)" "$S"'
assert "0168: runner shim frontmatter still carries the resolved native effort (bookkeeping)" \
  '[ "$(fm "$S" effort)" = "$(hd_field "$HD" claude status effort)" ]'
# The MODEL half of that same bookkeeping claim. The 0205 migration re-pointed the effort assert but
# dropped this one, leaving "the shim's frontmatter carries the resolved pin" only half pinned. Here
# the resolved model IS the user value (the required-model rule guarantees one), so this asserts the
# frontmatter tracks the resolution rather than being left empty or stale by the shim path.
assert "0168: runner shim frontmatter still carries the resolved native model (bookkeeping)" \
  '[ "$(fm "$S" model)" = "user-picked-id" ]'

# A user-configured pair is policy, not a shipped guess: it still passes through to the child.
cat > "$SBX/.docket.yml" <<'YML'
agent_harnesses: [claude]
agents:
  claude:
    status: { runner: codex, model: gpt-5.5, effort: high }
YML
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT168F" bash "$SYNC" >/dev/null 2>&1 )
assert "0168: an explicit override still passes through to the child" \
  'grep -qF -- "--model gpt-5.5" "$S" && grep -qF -- "--effort high" "$S"'

# The two fields split independently: a user model with no user effort bakes --model only,
# even though the sidecar supplies an effort for this agent.
cat > "$SBX/.docket.yml" <<'YML'
agent_harnesses: [claude]
agents:
  claude:
    status: { runner: codex, model: gpt-5.5 }
YML
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT168F" bash "$SYNC" >/dev/null 2>&1 )
assert "0168: user model alone bakes --model but not the shipped --effort" \
  'grep -qF -- "--model gpt-5.5" "$S" && ! grep -qF -- "--effort" "$S"'
rm -rf "$SBX" "$HROOT168F"

# The fallback warning's premise moved with the default store. A non-claude harness/agent pair the
# sidecar SUPPLIES is a deliberate shipped default, not a leak, so it must stay silent; a pair it
# does not cover generates unpinned rather than inheriting a foreign ID, and says so.
#
# The unpinned leg can no longer be driven by any SHIPPED harness — claude, cursor, and codex all
# carry complete blocks since change 0169. What the rule guards is still reachable (it is what a
# newly-added, not-yet-mapped harness hits), so the fixture reconstructs that state in a throwaway
# copy of the repo rather than asserting a condition the shipped tree can no longer reach: drop
# codex from the copy's shipped list AND delete its block, which is exactly "known but unshipped".
make_sandbox
HROOT168W="$(mktemp -d)"; mkdir -p "$HROOT168W/.claude"
SCRW="$(mktemp -d)"; cp -R "$REPO/agents" "$REPO/cursor-rules" "$REPO/scripts" "$REPO/sync-agents.sh" "$SCRW/"
# Anchored on the token being removed, not on its position in the list: `codex` stopped being the
# final token when change 0192 appended `opencode`, and a position-anchored pattern would silently
# stop matching (the fixture sanity asserts just below are what catch that).
sed -i.bak 's/^HD_SHIPPED_HARNESSES="\(.*\)codex\(.*\)"$/HD_SHIPPED_HARNESSES="\1\2"/' "$SCRW/scripts/lib/harness-defaults.sh"
# Normalize the doubled/trailing space the substitution above can leave. Address-scoped to the
# assignment line: an unscoped `s/  */ /g` would also collapse the two literal spaces inside
# _hd_block's `"^  "h` indent regexes and silently break the whole reader in the copy.
sed -i.bak2 '/^HD_SHIPPED_HARNESSES=/{ s/  */ /g; s/= /=/; s/" /"/; s/ "$/"/; }' "$SCRW/scripts/lib/harness-defaults.sh"
awk '/^  codex:[[:space:]]*$/{skip=1; next}
     skip && /^  [A-Za-z0-9._-]+[[:space:]]*:[[:space:]]*$/{skip=0}
     !skip' "$SCRW/agents/harness-defaults.yml" > "$SCRW/hd.tmp" && mv "$SCRW/hd.tmp" "$SCRW/agents/harness-defaults.yml"
# Fixture sanity FIRST: if either strip silently missed, every assert below is vacuous — the copy
# would still ship codex and simply never warn.
assert "0169 fixture: the copy no longer lists codex as shipped" \
  '! grep -qE "^HD_SHIPPED_HARNESSES=.*codex" "$SCRW/scripts/lib/harness-defaults.sh"'
assert "0169 fixture: the copy has no codex block" \
  '[ -z "$(hd_agents "$SCRW/agents/harness-defaults.yml" codex)" ]'
assert "0169 fixture: the copy still ships a complete cursor block (only codex was stripped)" \
  '[ "$(hd_agents "$SCRW/agents/harness-defaults.yml" cursor | grep -c .)" = "16" ]'
printf 'agent_harnesses: [claude, cursor, codex]\n' > "$SBX/.docket.yml"
w168="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT168W" bash "$SCRW/sync-agents.sh" 2>&1 >/dev/null)"
assert "0168: a cursor agent the sidecar supplies draws no warning" \
  '! grep -qF "cursor/docket-build-standard" <<<"$w168"'
assert "0168: a complete cursor block silences the whole harness" \
  '! grep -qF "WARN cursor/" <<<"$w168"'
assert "0168: an agent with no sidecar entry warns that it is generated unpinned" \
  'grep -qF "codex/docket-status: no harness-specific model" <<<"$w168"'
assert "0168: the unpinned warning names the key that would fix it" \
  'grep -qF "agents.codex.status.model" <<<"$w168"'
rm -rf "$SCRW" "$SBX" "$HROOT168W"
# Complement, on the REAL tree: because codex now ships complete, a shipped harness draws no
# unpinned warning at all.
#
# The negative assert CANNOT carry this on its own, and the pair is what makes the property real.
# A dropped or partial codex block makes `hd_validate` abort generation before any wrapper is
# written, so no `WARN codex/` line is ever emitted and the pure-negative assert stays green; it
# would also stay green on any unrelated `sync-agents.sh` failure. The positive companion supplies
# the missing half — the run really succeeded, a codex wrapper really exists, and it really carries
# the value the sidecar ships — so between them: generation reached completion AND it produced a
# pinned wrapper AND it did so silently. Mutation-proved by deleting the codex `status` row: the
# companion reddens (the abort writes no wrapper) while the negative alone does not.
make_sandbox
HROOT169S="$(mktemp -d)"; mkdir -p "$HROOT169S/.claude"
printf 'agent_harnesses: [claude, cursor, codex]\n' > "$SBX/.docket.yml"
w169="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT169S" bash "$SYNC" 2>&1 >/dev/null)"; rc169=$?
assert "0169: a complete codex block silences the whole harness" \
  '! grep -qF "WARN codex/" <<<"$w169"'
assert "0169: and generation actually SUCCEEDED and pinned the codex wrapper (the silence is not an abort)" \
  '[ "$rc169" = "0" ] &&
   [ -f "$SBX/.codex/agents/docket-status.toml" ] &&
   [ -n "$(hd_field "$HD" codex status model)" ] &&
   [ "$(sed -nE "s/^model[[:space:]]*=[[:space:]]*\"(.*)\"[[:space:]]*$/\1/p" "$SBX/.codex/agents/docket-status.toml")" = "$(hd_field "$HD" codex status model)" ]'
rm -rf "$SBX" "$HROOT169S"
# Amendment guard: a user `agents.default` outranks the sidecar, so the wrapper carries the FOREIGN
# id — the warning must fire even though the pair IS covered. Testing entry-existence instead of
# value-provenance silenced this exact case.
make_sandbox
HROOT168D="$(mktemp -d)"; mkdir -p "$HROOT168D/.claude"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: claude-opus-4-8 }\n' > "$SBX/.docket.yml"
w168d="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT168D" bash "$SYNC" 2>&1 >/dev/null)"
assert "0168: agents.default overriding a COVERED cursor pair still warns" \
  'grep -qF "cursor/docket-status: model '"'"'claude-opus-4-8'"'"' came from agents.default" <<<"$w168d"'
assert "0168: and the wrapper really does carry the foreign id (the warning is not a false alarm)" \
  '[ "$(sed -n "s/^model:[[:space:]]*//p" "$SBX/.cursor/agents/docket-status.md" | head -n1)" = "claude-opus-4-8" ]'
rm -rf "$SBX" "$HROOT168D"

# ---- change 0168's two headline properties, asserted on a BARE opt-in --------
# 0168 whole-branch review, Recommendation 2. Everything above proves a mechanism; this proves the
# OUTCOME a repo actually gets from `agent_harnesses:` and nothing else — no agents: block, no
# overrides in any layer. Two properties, stated the way the change states them:
#   (a) Claude keeps a complete pin — every generated claude wrapper carries BOTH model and effort;
#   (b) no Claude-only model ID leaks into ANY other shipped harness's wrapper.
# (b) is the defect class change 0135 shipped and 0168 was written to make structurally impossible,
# and it is the one a future harness token added without its own sidecar entry would re-open — so
# the population below is derived from $HD_SHIPPED_HARNESSES, never hand-listed: a newly shipped
# harness is opted in, generated, and leak-scanned here for free (repo AGENTS.md).
make_sandbox
hlist=""
for h in $HD_SHIPPED_HARNESSES; do
  hlist="${hlist:+$hlist, }$h"
  [ "$h" = "claude" ] || mkdir -p "$SBX/.$h"
done
HROOT168R="$(mktemp -d)"; mkdir -p "$HROOT168R/.claude"
printf 'agent_harnesses: [%s]\n' "$hlist" > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT168R" bash "$SYNC" >/dev/null 2>&1 )
# Frontmatter-ANCHORED read. fm() above is first-match-anywhere, so on a wrapper with no pin it
# scans into the body and can return prose — a false green for an assert whose whole point is
# "the pin is present" (AGENTS.md: anchor a frontmatter read to the first ---…--- block).
fm_anchored(){  # $1=file $2=key
  awk -v k="$2" '/^---[[:space:]]*$/ { d++; if (d>=2) exit; next }
                 d==1 && $0 ~ "^"k"[[:space:]]*:" { sub("^"k"[[:space:]]*:[[:space:]]*",""); sub(/[[:space:]]+$/,""); print; exit }' "$1"
}
n_r2=0
for f in "$SBX"/.claude/agents/docket-*.md; do
  [ -e "$f" ] || continue
  n_r2=$((n_r2+1)); b="$(basename "$f")"
  assert "0168 R2: claude/$b carries a non-empty model"  '[ -n "$(fm_anchored "'"$f"'" model)" ]'
  assert "0168 R2: claude/$b carries a non-empty effort" '[ -n "$(fm_anchored "'"$f"'" effort)" ]'
done
assert "0168 R2: the full claude set generated (floor 16; got $n_r2) — the loop above is not vacuous" \
  '[ "$n_r2" -ge 16 ]'

# Claude-ONLY model IDs: every model the sidecar's claude block names, minus every model any other
# harness block names. Derived from the sidecar, so a cursor entry that legitimately reuses a
# Claude ID (claude-opus-5-high today) is excluded rather than hand-waived.
claude_models="$(for a in $(hd_agents "$HD" claude); do hd_field "$HD" claude "$a" model; printf '\n'; done | sort -u | grep -v '^$')"
other_models="$(for h in $HD_SHIPPED_HARNESSES; do
  [ "$h" = "claude" ] && continue
  for a in $(hd_agents "$HD" "$h"); do hd_field "$HD" "$h" "$a" model; printf '\n'; done
done | sort -u | grep -v '^$')"
claude_only="$(comm -23 <(printf '%s\n' "$claude_models") <(printf '%s\n' "$other_models"))"
assert "0168 R2: the claude-only model set is non-empty (floor — otherwise the leak asserts are vacuous)" \
  '[ -n "$claude_only" ]'
# Cursor encodes effort INSIDE the model value, so compare on the bare ID with any [effort=…]
# suffix stripped; a substring match would false-positive on cursor's own claude-opus-5-high.
# The scan is keyed on the FILE's shape (TOML `model = "…"` vs frontmatter `model: …`), not on a
# list of harness names, so every non-claude harness docket ships is covered by construction.
leaks=""
n_scan=0
for h in $HD_SHIPPED_HARNESSES; do
  [ "$h" = "claude" ] && continue
  for f in "$SBX/.$h"/agents/docket-*; do
    [ -e "$f" ] || continue
    case "$f" in
      # `{p;q;}` rather than `| head -n1`: an early-exiting consumer would SIGPIPE sed under pipefail.
      *.toml) v="$(sed -n -E '/^model[[:space:]]*=/{s/^model[[:space:]]*=[[:space:]]*"(.*)"[[:space:]]*$/\1/p;q;}' "$f")" ;;
      *)      v="$(fm_anchored "$f" model)"; v="${v%%\[*}" ;;
    esac
    [ -n "$v" ] || continue
    n_scan=$((n_scan+1))
    grep -qxF "$v" <<<"$claude_only" && leaks="$leaks $h:$(basename "$f")=$v"
  done
done
# Floor: 16 wrappers on each non-claude shipped harness. Without it, a harness whose directory was
# never generated would make the leak assert below pass vacuously.
assert "0168 R2: the leak scan read a full wrapper set per non-claude harness (got $n_scan)" \
  '[ "$n_scan" -ge 48 ]'
assert "0168 R2: no non-claude wrapper carries a model that lives ONLY in the sidecar's claude block (leaks:${leaks:- none})" \
  '[ -z "$leaks" ]'
rm -rf "$SBX" "$HROOT168R"

# ============================================================================
# Change 0173 — field_of() value class: provider-prefixed model IDs round-trip
# ============================================================================
# The truncation is SILENT: a wrapper is still written and still parses, it just
# carries `anthropic` where the user wrote `anthropic/claude-opus-5`. Every assert
# here is therefore value-level — "generation succeeded" and "the wrapper exists"
# both pass against the bug.

# -- layer 1 of 3: global config.yml --
make_sandbox
mkdir -p "$SBX/.config/docket"
printf 'agents:\n  default:\n    status: { model: anthropic/claude-opus-5, effort: low }\n' > "$SBX/.config/docket/config.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null )
assert "0173: global layer — slash-bearing model survives whole" \
  '[ "$(fm_anchored "$SBX/.claude/agents/docket-status.md" model)" = "anthropic/claude-opus-5" ]'
assert "0173: global layer — effort alongside it is unaffected" \
  '[ "$(fm_anchored "$SBX/.claude/agents/docket-status.md" effort)" = "low" ]'
rm -rf "$SBX"

# -- layer 2 of 3: repo-committed .docket.yml --
make_sandbox
HROOT173B="$(mktemp -d)"; mkdir -p "$HROOT173B/.claude"
printf 'agents:\n  default:\n    status: { model: openai:gpt-5.6-sol, effort: high }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT173B" bash "$SYNC" >/dev/null )
assert "0173: committed layer — colon-bearing model survives whole" \
  '[ "$(fm_anchored "$SBX/.claude/agents/docket-status.md" model)" = "openai:gpt-5.6-sol" ]'
rm -rf "$SBX" "$HROOT173B"

# -- layer 3 of 3: machine-local .docket.local.yml --
make_sandbox
HROOT173C="$(mktemp -d)"; mkdir -p "$HROOT173C/.claude"
printf 'agents:\n  default:\n    status: { model: openrouter:vendor/model, effort: high }\n' > "$SBX/.docket.local.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT173C" bash "$SYNC" >/dev/null )
assert "0173: local layer — colon AND slash together survive whole" \
  '[ "$(fm_anchored "$SBX/.claude/agents/docket-status.md" model)" = "openrouter:vendor/model" ]'
rm -rf "$SBX" "$HROOT173C"

# -- non-regression: a plain unprefixed id is untouched by the widening --
make_sandbox
HROOT173D="$(mktemp -d)"; mkdir -p "$HROOT173D/.claude"
printf 'agents:\n  default:\n    status: { model: sonnet, effort: high }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT173D" bash "$SYNC" >/dev/null )
assert "0173: plain unprefixed model still resolves exactly (non-regression)" \
  '[ "$(fm_anchored "$SBX/.claude/agents/docket-status.md" model)" = "sonnet" ]'
assert "0173: closing brace is not swallowed into the value" \
  '! /usr/bin/grep -q "model:.*}" "$SBX/.claude/agents/docket-status.md"'
rm -rf "$SBX" "$HROOT173D"

# -- the agents.default vs agents.<harness> merge, with provenance --
# A harness-specific line and a default line, both provider-prefixed. The harness line must win
# for its own harness, the default must reach the other, and RES_MODEL_FROM_HARNESS (which drives
# warn_fallback_model) must be unaffected by the widening.
make_sandbox
mkdir -p "$SBX/.cursor"
HROOT173E="$(mktemp -d)"; mkdir -p "$HROOT173E/.claude"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: anthropic/claude-opus-5 }\n  cursor:\n    status: { model: openrouter:vendor/model }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT173E" bash "$SYNC" >/dev/null )
# Cursor encodes effort INSIDE the model value as `<id>[effort=<e>]`, but only when an effort
# actually resolves. Probed against the real generated file for THIS fixture (no cursor effort is
# configured and none resolves): the emitted value is bare, with no `[effort=…]` suffix. So compare
# on the whole value — an unstripped comparison also catches a suffix appearing where none belongs.
cur_m="$(fm_anchored "$SBX/.cursor/agents/docket-status.md" model)"
assert "0173: merge — harness block wins for cursor, whole" '[ "$cur_m" = "openrouter:vendor/model" ]'
assert "0173: merge — claude falls to agents.default, whole" \
  '[ "$(fm_anchored "$SBX/.claude/agents/docket-status.md" model)" = "anthropic/claude-opus-5" ]'
rm -rf "$SBX" "$HROOT173E"

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

exit $fail
