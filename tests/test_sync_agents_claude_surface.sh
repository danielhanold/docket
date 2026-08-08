#!/usr/bin/env bash
# tests/test_sync_agents_claude_surface.sh — docket creates and maintains the Claude parent-facing
# instruction surface, one managed block per distinct PHYSICAL file (change 0242).
# run: bash tests/test_sync_agents_claude_surface.sh
set -uo pipefail
unset XDG_CONFIG_HOME
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SYNC="$REPO/sync-agents.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# Every sandbox is minted under a path whose LAST COMPONENT IS A SYMLINK ($BASE/via -> phys). The
# repo therefore lives at a logical path that `pwd -P` canonicalises differently, which is what
# makes the absolute-symlink combo below discriminating: a resolver that canonicalises only the
# starting directory and then trusts a symlink's own absolute target agrees with a correct one on
# every fixture whose path happens to be physical already (learnings:
# shared-resource-keeps-first-owner-assumptions — a fixture that cannot tell two implementations
# apart is not coverage).
new_sandbox(){
  BASE="$(mktemp -d "${TMPDIR:-/tmp}/claudesurface.XXXXXX")"
  mkdir -p "$BASE/phys"
  ( cd "$BASE" && ln -s phys via )
  SBX="$BASE/via/repo"
  mkdir -p "$SBX"
  git -C "$SBX" init --quiet
  git -C "$SBX" config user.email t@t.test
  git -C "$SBX" config user.name Test
}
run_sync(){ ( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 ); }

# $1 = agent_harnesses body; $2 = pre-existing surface combo
mk_repo(){
  new_sandbox
  printf 'agent_harnesses: %s\n' "$1" > "$SBX/.docket.yml"
  case "$2" in
    agents)    printf '# Repo instructions\n'  > "$SBX/AGENTS.md" ;;
    claude)    printf '# Repo instructions\n'  > "$SBX/CLAUDE.md" ;;
    both)      printf '# A\n' > "$SBX/AGENTS.md"; printf '# C\n' > "$SBX/CLAUDE.md" ;;
    symlinked) printf '# Repo instructions\n'  > "$SBX/AGENTS.md"
               ( cd "$SBX" && ln -s AGENTS.md CLAUDE.md ) ;;
    abslinked) printf '# Repo instructions\n'  > "$SBX/AGENTS.md"
               ( cd "$SBX" && ln -s "$SBX/AGENTS.md" CLAUDE.md ) ;;
    chained)   printf '# Repo instructions\n'  > "$SBX/AGENTS.md"
               ( cd "$SBX" && ln -s AGENTS.md MID.md && ln -s MID.md CLAUDE.md ) ;;
    none)      : ;;
  esac
  run_sync
}
# Count managed-block openings in $1. A missing file, or a present file with no block, is 0 —
# spelled without `|| echo 0` so the two cases cannot print a doubled count (grep -c prints its 0).
blocks_in(){ [ -e "$1" ] || { printf '0\n'; return 0; }; grep -c "docket:dispatch:start" "$1"; }

# --- claude + AGENTS.md only: the symlink is created, ONE physical file, ONE block ---
mk_repo "[claude, codex]" agents
assert "AGENTS-only: CLAUDE.md is created"            '[ -e "$SBX/CLAUDE.md" ]'
assert "AGENTS-only: CLAUDE.md is a symlink"          '[ -L "$SBX/CLAUDE.md" ]'
assert "AGENTS-only: symlink is RELATIVE to AGENTS.md" '[ "$(readlink "$SBX/CLAUDE.md")" = "AGENTS.md" ]'
assert "AGENTS-only: exactly ONE dispatch block"      '[ "$(blocks_in "$SBX/AGENTS.md")" = "1" ]'
assert "AGENTS-only: the block carries the run gate"  'grep -q "verify-run --in-progress-ids" "$SBX/AGENTS.md"'
assert "AGENTS-only: pre-existing content survives"   'grep -qxF "# Repo instructions" "$SBX/AGENTS.md"'

# --- virgin repo, BOTH surfaces wanted: still ONE physical file ---
# The discriminating case for resolution ORDER: AGENTS.md does not exist when the Claude target is
# resolved — it is about to be created by this same run. A surface resolver that asks "does
# AGENTS.md exist NOW" seeds a second real file here and every future run maintains two copies of
# the same block.
mk_repo "[claude, codex]" none
assert "virgin both: CLAUDE.md is a symlink, not a second real file" '[ -L "$SBX/CLAUDE.md" ]'
assert "virgin both: AGENTS.md was created"                '[ -f "$SBX/AGENTS.md" ] && [ ! -L "$SBX/AGENTS.md" ]'
assert "virgin both: exactly ONE dispatch block"           '[ "$(blocks_in "$SBX/AGENTS.md")" = "1" ]'
assert "virgin both: the link resolves (gate readable through it)" \
  'grep -q "verify-run --in-progress-ids" "$SBX/CLAUDE.md"'

# --- claude alone (no AGENTS.md-dispatch harness): a real CLAUDE.md is seeded ---
mk_repo "[claude]" none
assert "claude-only/neither: CLAUDE.md is created"       '[ -f "$SBX/CLAUDE.md" ]'
assert "claude-only/neither: it is a REAL file, no link" '[ ! -L "$SBX/CLAUDE.md" ]'
assert "claude-only/neither: it carries the gate"        'grep -q "verify-run --in-progress-ids" "$SBX/CLAUDE.md"'
assert "claude-only/neither: no AGENTS.md is created"    '[ ! -e "$SBX/AGENTS.md" ]'

# --- an existing CLAUDE.md is written INTO, never replaced ---
mk_repo "[claude]" claude
assert "existing CLAUDE.md: still a real file"     '[ -f "$SBX/CLAUDE.md" ] && [ ! -L "$SBX/CLAUDE.md" ]'
assert "existing CLAUDE.md: pre-existing content survives" 'grep -q "Repo instructions" "$SBX/CLAUDE.md"'
assert "existing CLAUDE.md: gains exactly one block" '[ "$(blocks_in "$SBX/CLAUDE.md")" = "1" ]'

# --- already symlinked: physical-path dedupe writes the block exactly ONCE ---
mk_repo "[claude, codex]" symlinked
assert "symlinked: AGENTS.md has exactly one block" '[ "$(blocks_in "$SBX/AGENTS.md")" = "1" ]'
# CLAUDE.md IS AGENTS.md here; the second target must be recognised as the same physical file.
assert "symlinked: reading through the link shows one block" '[ "$(blocks_in "$SBX/CLAUDE.md")" = "1" ]'
assert "symlinked: the link was not replaced by a file" '[ -L "$SBX/CLAUDE.md" ]'
assert "symlinked: user content survived" 'grep -qxF "# Repo instructions" "$SBX/AGENTS.md"'

# --- ABSOLUTE symlink through a symlinked directory component: same physical file ---
mk_repo "[claude, codex]" abslinked
assert "abs symlink: AGENTS.md has exactly one block" '[ "$(blocks_in "$SBX/AGENTS.md")" = "1" ]'
assert "abs symlink: the link was not replaced by a file" '[ -L "$SBX/CLAUDE.md" ]'
assert "abs symlink: user content survived" 'grep -qxF "# Repo instructions" "$SBX/AGENTS.md"'

# --- a symlink CHAIN resolves to the file at the end of it ---
mk_repo "[claude, codex]" chained
assert "chain: AGENTS.md has exactly one block" '[ "$(blocks_in "$SBX/AGENTS.md")" = "1" ]'
assert "chain: both hops survive as links" '[ -L "$SBX/CLAUDE.md" ] && [ -L "$SBX/MID.md" ]'
assert "chain: user content survived" 'grep -qxF "# Repo instructions" "$SBX/AGENTS.md"'

# --- two DISTINCT physical files each get their own block ---
mk_repo "[claude, codex]" both
assert "distinct files: AGENTS.md has a block" '[ "$(blocks_in "$SBX/AGENTS.md")" = "1" ]'
assert "distinct files: CLAUDE.md has a block" '[ "$(blocks_in "$SBX/CLAUDE.md")" = "1" ]'
assert "distinct files: CLAUDE.md content survived" 'grep -q "^# C$" "$SBX/CLAUDE.md"'
assert "distinct files: AGENTS.md content survived" 'grep -q "^# A$" "$SBX/AGENTS.md"'

# --- claude NOT targeted: no Claude surface is created or touched ---
mk_repo "[codex]" agents
assert "no claude: CLAUDE.md is not created" '[ ! -e "$SBX/CLAUDE.md" ]'
assert "no claude: AGENTS.md still gets its block" '[ "$(blocks_in "$SBX/AGENTS.md")" = "1" ]'

# --- a user's own CLAUDE.md -> AGENTS.md link is NOT a second file to strip through ---
# The dangerous shape: claude is NOT targeted, so CLAUDE.md is not a surface — but it is an ALIAS
# for one that IS. Stripping "CLAUDE.md" here writes straight through the link and deletes the
# live codex block from AGENTS.md. This assert is what makes physical-path resolution load-bearing
# rather than decorative.
mk_repo "[codex]" symlinked
assert "codex + user link: the live block was not stripped through the alias" \
  '[ "$(blocks_in "$SBX/AGENTS.md")" = "1" ]'
assert "codex + user link: the user's link is left alone" '[ -L "$SBX/CLAUDE.md" ]'

# --- no surface harness at all: every surface loses its block, user bytes kept ---
mk_repo "[claude, codex]" agents
printf '# keep me\n' >> "$SBX/AGENTS.md"
printf 'agent_harnesses: [cursor]\n' > "$SBX/.docket.yml"
run_sync
assert "de-list all: AGENTS.md block removed" '[ "$(blocks_in "$SBX/AGENTS.md")" = "0" ]'
assert "de-list all: user content kept"       'grep -qxF "# keep me" "$SBX/AGENTS.md"'
assert "de-list all: the CLAUDE.md link is not deleted" '[ -L "$SBX/CLAUDE.md" ]'

# --- a symlink CYCLE is bounded: the sync terminates instead of spinning forever ---
# `[ -e ]` is false on a cycle but `[ -L ]` is true, so the surface is neither replaced nor walked
# past the hop bound. Asserted by watching the process, not by reading the code: an unbounded walk
# is a hang, and a hang is the one failure a plain `assert` can never report.
new_sandbox
printf 'agent_harnesses: [claude, codex]\n' > "$SBX/.docket.yml"
printf '# Repo instructions\n' > "$SBX/AGENTS.md"
( cd "$SBX" && ln -s LOOP.md CLAUDE.md && ln -s CLAUDE.md LOOP.md )
# Kill the whole descendant tree, not just the wrapper subshell: the spinning process is the
# grandchild `bash sync-agents.sh`, and killing only its parent would leave a runaway orphan
# burning a core for the rest of the suite.
kill_tree(){ local p="$1" c; for c in $(pgrep -P "$p" 2>/dev/null); do kill_tree "$c"; done; kill -9 "$p" 2>/dev/null; }
run_sync & syncpid=$!
n=0; hung=1
while [ "$n" -lt 40 ]; do
  if kill -0 "$syncpid" 2>/dev/null; then sleep 0.5; n=$((n+1)); else hung=0; break; fi
done
[ "$hung" = "1" ] && kill_tree "$syncpid"
wait "$syncpid" 2>/dev/null
assert "cycle: the sync terminates (the link walk is bounded)" '[ "$hung" = "0" ]'
assert "cycle: AGENTS.md still got its block" '[ "$(blocks_in "$SBX/AGENTS.md")" = "1" ]'

# --- --check iterates the SAME surface set the write pass does (change 0242) ------------------
# The correspondence property, asserted in both directions: --check must fail on exactly the
# surfaces `bash sync-agents.sh` would go on to change, and pass once it has changed them. A check
# that walked only AGENTS.md would certify a repo whose Claude surface had drifted (learnings:
# correspondence-guard-runs-one-way).
run_check(){ ( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check >/dev/null 2>&1 ); }
# Corrupt a managed block's BODY, leaving both markers intact and correctly ordered — this must
# exercise the staleness path, not the malformed-marker path, which refuses rather than reports.
stale_block(){  # $1 = a real (non-symlink) file carrying the block
  awk '{ print } /docket:dispatch:start/ { print "STALE" }' "$1" > "$1.staletmp" \
    && mv -f "$1.staletmp" "$1"
}

mk_repo "[claude]" none
run_check; rc=$?
assert "check: a freshly synced claude-only tree passes" '[ "$rc" = "0" ]'

stale_block "$SBX/CLAUDE.md"
run_check; rc=$?
assert "check: a stale Claude-surface block fails the check" '[ "$rc" = "1" ]'
run_sync; run_check; rc=$?
assert "check: re-running the sync clears the Claude-surface staleness" '[ "$rc" = "0" ]'

# The other direction of the same correspondence: a DISTINCT surface whose harness was de-listed.
# The write pass strips it (its physical path is absent from the write pass's `seen` set), so the
# check must report it — even though another dispatch harness is still targeted, which is the leg
# a "no harness at all" else-branch would never reach.
mk_repo "[claude, codex]" both
printf 'agent_harnesses: [codex]\n' > "$SBX/.docket.yml"
run_check; rc=$?
assert "check: a de-listed DISTINCT surface's stray block fails the check" '[ "$rc" = "1" ]'
run_sync; run_check; rc=$?
assert "check: the strip pass clears exactly what the check reported" '[ "$rc" = "0" ]'
assert "de-listed distinct: its block is gone" '[ "$(blocks_in "$SBX/CLAUDE.md")" = "0" ]'
assert "de-listed distinct: the live surface kept its block" '[ "$(blocks_in "$SBX/AGENTS.md")" = "1" ]'

# --check is READ-ONLY: it must never call the creating resolver. Synced first, then the surface is
# deleted, so the ONLY thing left for the check to trip on is the missing Claude surface itself —
# an unsynced sandbox would fail on the .gitignore leg and prove nothing about this one.
mk_repo "[claude]" none
rm -f "$SBX/CLAUDE.md"
run_check; rc=$?
assert "check: a missing Claude surface fails the check" '[ "$rc" = "1" ]'
assert "check: ...and was NOT created by the check" '[ ! -e "$SBX/CLAUDE.md" ] && [ ! -L "$SBX/CLAUDE.md" ]'

# The shim is retired: one name owns the surface write.
assert "no sync_agents_md_dispatch caller or definition survives" \
  '[ "$(grep -c "sync_agents_md_dispatch" "$SYNC")" = "0" ]'

echo; [ "$fail" = 0 ] && echo "ALL PASS" || echo "FAILURES"; exit "$fail"
