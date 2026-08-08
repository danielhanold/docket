#!/usr/bin/env bash
# tests/test_sync_agents_surface_containment.sh — docket writes and strips its dispatch block ONLY
# inside the checkout it was run in (change 0242 review).
# run: bash tests/test_sync_agents_surface_containment.sh
#
# A dispatch surface is an ordinary file, and a user may symlink AGENTS.md or CLAUDE.md anywhere: a
# shared instructions file in a sibling checkout, a ~/dotfiles target absent on this machine, a
# volume that is not mounted. Two ways that escaped the checkout before this guard existed —
#   (1) a resolver hop that could not `cd` assigned the failed substitution's EMPTY output into the
#       accumulator before breaking, so the surface resolved to `/<basename>`, a path at the
#       FILESYSTEM ROOT that the write pass then acted on;
#   (2) neither pass tested the resolved path for containment at all, so a CLAUDE.md pointing at a
#       shared file outside the repo was written to by one repo's sync and stripped by another's.
# Split out of tests/test_sync_agents_claude_surface.sh, which is already at its budget ceiling
# (tests/README.md, "extend a sibling shard or start a new one").
set -uo pipefail
unset XDG_CONFIG_HOME
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SYNC="$REPO/sync-agents.sh"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

# Sandboxes mirror the sibling shard: minted under a path whose last component is a SYMLINK
# ($BASE/via -> phys), so the repo's logical spelling and its `pwd -P` canonicalisation differ. That
# is what makes containment discriminating here — a predicate that compared the resolved path
# against the LOGICAL $PWD would reject the repo's own surfaces, and one that compared unresolved
# spellings would accept a link pointing straight out of the tree.
new_sandbox(){
  BASE="$(mktemp -d "${TMPDIR:-/tmp}/surfcontain.XXXXXX")"
  mkdir -p "$BASE/phys"
  ( cd "$BASE" && ln -s phys via )
  SBX="$BASE/via/repo"
  mkdir -p "$SBX"
  git -C "$SBX" init --quiet
  git -C "$SBX" config user.email t@t.test
  git -C "$SBX" config user.name Test
  PHYSSBX="$(cd "$SBX" && pwd -P)"
  PHYSBASE="$(cd "$BASE" && pwd -P)"
}
# The sync, with its diagnostics captured instead of discarded (stdout still dropped).
run_sync_log(){ ( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" 2>&1 >/dev/null ); }
run_check(){ ( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check >/dev/null 2>&1 ); }
# Count managed-block openings in $1. A missing file, or a present file with no block, is 0 —
# spelled without `|| echo 0` so the two cases cannot print a doubled count (grep -c prints its 0).
blocks_in(){ [ -e "$1" ] || { printf '0\n'; return 0; }; grep -c "docket:dispatch:start" "$1"; }

# --- unit: an unresolvable hop falls back to the LAST RESOLVABLE path --------------------------
# Exercised on resolve_physical_path itself (sourcing sync-agents.sh runs no main — its body is
# guarded on BASH_SOURCE) because the defect is in the resolver rather than in any one caller: a
# repo has one CLAUDE.md, so an end-to-end fixture can only ever show one of the two hop branches,
# and the relative and absolute branches are separate lines that were separately wrong.
resolve_via_sync(){  # $1 = cwd (repo root) ; $2 = path to resolve
  ( cd "$1" && . "$SYNC" >/dev/null 2>&1; resolve_physical_path "$2" )
}
RSCR="$(mktemp -d "${TMPDIR:-/tmp}/surfcontain-res.XXXXXX")"; mkdir -p "$RSCR/repo"
# Neither `docs/` nor `$RSCR/gone/` exists, so both links dangle on a MISSING DIRECTORY — the
# condition that makes the hop's `cd` fail. Nothing outside the scratch dir is assumed.
( cd "$RSCR/repo" && ln -s docs/AGENTS.md CLAUDE.md && ln -s "$RSCR/gone/AGENTS.md" ABS.md )
RPHYS="$(cd "$RSCR/repo" && pwd -P)"
assert "resolver: a dangling RELATIVE link into a missing dir keeps the last resolvable path" \
  '[ "$(resolve_via_sync "$RSCR/repo" "$RSCR/repo/CLAUDE.md")" = "$RPHYS/CLAUDE.md" ]'
assert "resolver: a dangling ABSOLUTE link into a missing dir keeps the last resolvable path" \
  '[ "$(resolve_via_sync "$RSCR/repo" "$RSCR/repo/ABS.md")" = "$RPHYS/ABS.md" ]'

# --- end-to-end: a DANGLING surface link is reachable, and must not escape the sandbox ---------
# Reachable because claude_surface_target only creates a surface that is neither -e nor -L, and a
# dangling link IS -L: it is passed through untouched and its unresolvable path flows into the write
# pass. The invariant is asserted over the run's own diagnostics, which is what names the escape
# (`//CLAUDE.md`) when it happens — the filesystem-root assert below cannot, on a host whose root
# is read-only.
escaping_targets(){ grep -E "dispatch block (in|from) /" <<<"$1" | grep -v -F -- "$PHYSSBX/" || true; }
new_sandbox
printf 'agent_harnesses: [claude, codex]\n' > "$SBX/.docket.yml"
printf '# Repo instructions\n' > "$SBX/AGENTS.md"
( cd "$SBX" && ln -s docs/AGENTS.md CLAUDE.md )   # docs/ does not exist: the link dangles
LOG="$(run_sync_log)"
assert "dangling surface: every reported target stays inside the sandbox" '[ -z "$(escaping_targets "$LOG")" ]'
# The portable half of the same claim, and the one with teeth on a Linux/CI box running as root,
# where the escaped path WOULD be created. On a macOS read-only root the OS denies that write, so
# this assert stays green under mutation there and the log assert above is the discriminating one.
assert "dangling surface: nothing was created at the filesystem root" '[ ! -e /CLAUDE.md ] && [ ! -e /AGENTS.md ]'
assert "dangling surface: the real AGENTS.md still got its block" '[ "$(blocks_in "$SBX/AGENTS.md")" = "1" ]'
assert "dangling surface: the broken link is left exactly as it was" \
  '[ -L "$SBX/CLAUDE.md" ] && [ "$(readlink "$SBX/CLAUDE.md")" = "docs/AGENTS.md" ]'
# ...and the run must not CLAIM it wrote that file. The dangling link resolves to a path inside the
# repo, so containment passes it through to the write, where the redirect fails on the missing
# target — `ensure_managed_block` used to return `wrote` regardless of the redirect's fate, so the
# sync printed "wrote/updated … — COMMIT THIS" for bytes that were never written and sent the user
# to commit a file that does not exist (change 0242 review, finding 8). This fixture is the one that
# already reaches an always-failing write, so the claim is asserted on ITS log, with no extra sync.
# Bound to the path, not asserted as two independent greps: a run that wrote correctly elsewhere
# also emits a "wrote/updated" line.
NAMED="$(grep -F -- "$PHYSSBX/CLAUDE.md" <<<"$LOG")"
assert "dangling surface: nothing claims to have written the unwritable target" \
  '! grep -q "wrote/updated" <<<"$NAMED"'
assert "dangling surface: one diagnostic names the target and reports the write FAILED" \
  '[ -n "$NAMED" ] && grep -q "could not write" <<<"$NAMED"'

# --- end-to-end: a surface pointing OUT of the checkout is neither written nor stripped --------
# The markers are read out of sync-agents.sh rather than transcribed: a fixture seeding a block that
# the strip pass does not recognise would pass no matter what that pass does.
DSTART="$(sed -n "s/^DISPATCH_START='\(.*\)'\$/\1/p" "$SYNC")"
DEND="$(sed -n "s/^DISPATCH_END='\(.*\)'\$/\1/p" "$SYNC")"
assert "the dispatch markers were read out of sync-agents.sh" '[ -n "$DSTART" ] && [ -n "$DEND" ]'

new_sandbox
OUT="$BASE/shared-instructions.md"
OUTPHYS="$PHYSBASE/shared-instructions.md"
# The outside file already carries a STALE docket block — the shape a second checkout's sync leaves
# behind. It is what a write pass would refresh and a strip pass would delete, so both refusals have
# something to fail to do.
{ printf '# shared, owned by another checkout\n'
  printf '%s\n' "$DSTART"; printf 'STALE BODY\n'; printf '%s\n' "$DEND"; } > "$OUT"
OUT_BEFORE="$(cksum < "$OUT")"
printf 'agent_harnesses: [claude, codex]\n' > "$SBX/.docket.yml"
printf '# Repo instructions\n' > "$SBX/AGENTS.md"
( cd "$SBX" && ln -s "$OUT" CLAUDE.md )
LOG="$(run_sync_log)"
assert "out-of-repo write: the outside file is byte-identical"      '[ "$(cksum < "$OUT")" = "$OUT_BEFORE" ]'
assert "out-of-repo write: its stale block was NOT refreshed"       'grep -qxF "STALE BODY" "$OUT"'
# The diagnostic must name the path AND say why on ONE line: a run that merely reports "wrote ... in
# <that path>" also names it, so two independent greps over the whole log would pass on the very
# behaviour this fixture exists to reject. Two steps rather than a pipe — a `grep -q` consumer takes
# SIGPIPE under pipefail (AGENTS.md, shell).
NAMED="$(grep -F -- "$OUTPHYS" <<<"$LOG")"
assert "out-of-repo write: one diagnostic names the RESOLVED path and says why it was skipped" \
  'grep -q "outside this repository" <<<"$NAMED"'
assert "out-of-repo write: the in-repo surface still got its block" '[ "$(blocks_in "$SBX/AGENTS.md")" = "1" ]'
# --check is the read-only twin of that pass, so it must refuse the same file: reporting the stale
# block out there would demand a write `bash sync-agents.sh` will never perform — a red CI leg with
# no green path out (learnings: correspondence-guard-runs-one-way).
run_check; rc=$?
assert "out-of-repo write: --check does not demand a write the sync refuses to make" '[ "$rc" = "0" ]'

# Strip half, same sandbox: de-list claude, so CLAUDE.md stops being a target and the strip pass
# reaches it. Stripping through the link deletes a live block out of the other checkout's file.
printf 'agent_harnesses: [codex]\n' > "$SBX/.docket.yml"
LOG="$(run_sync_log)"
assert "out-of-repo strip: the outside file is STILL byte-identical" '[ "$(cksum < "$OUT")" = "$OUT_BEFORE" ]'
assert "out-of-repo strip: its block was not stripped"               '[ "$(blocks_in "$OUT")" = "1" ]'
NAMED="$(grep -F -- "$OUTPHYS" <<<"$LOG")"
assert "out-of-repo strip: one diagnostic names the RESOLVED path and says why it was skipped" \
  'grep -q "outside this repository" <<<"$NAMED"'
# The check's STRIP half is a distinct leg from its write half, and only a de-listed surface reaches
# it — with claude still targeted the path lands in the check's `seen` set and this leg never looks
# at it. Same failure mode as the write half: a stray block out there is not docket's to remove.
run_check; rc=$?
assert "out-of-repo strip: --check does not demand a strip the sync refuses to make" '[ "$rc" = "0" ]'

# --- a surface docket SEEDED, emptied by the strip: advised, never deleted -----------------------
# Change 0242 review, finding 9. On a claude-only repo with neither surface present,
# claude_surface_target creates a REAL CLAUDE.md whose only content is the managed block. De-listing
# claude then strips that block and leaves a one-byte file docket created and nobody owns. Docket
# must not delete it — at strip time nothing records who created the file, and the identical shape
# is reachable for a file the USER created and emptied — so the contract is a named advisory.
# Housed in this shard rather than in tests/test_sync_agents_claude_surface.sh, which owns the
# seeding cases but sits at 41s against a 45s ceiling (tests/runtime-budgets.tsv; tests/README.md,
# "extend a sibling shard or start a new one"); this shard already captures the sync's diagnostics.
new_sandbox
printf 'agent_harnesses: [claude]\n' > "$SBX/.docket.yml"
run_sync_log >/dev/null
assert "seeded surface: a claude-only repo got a REAL CLAUDE.md carrying the block" \
  '[ -f "$SBX/CLAUDE.md" ] && [ ! -L "$SBX/CLAUDE.md" ] && [ "$(blocks_in "$SBX/CLAUDE.md")" = "1" ]'
printf 'agent_harnesses: [cursor]\n' > "$SBX/.docket.yml"
# Negative control, stripped by the SAME run: a second surface carrying user prose around its block.
# An advisory keyed on "the strip ran" rather than on "nothing is left" would fire for this one too,
# telling the user a file full of their own text is empty. Seeded now, so only the strip pass sees
# it, and with the markers read out of sync-agents.sh above rather than transcribed.
{ printf '# keep me\n'; printf '%s\n' "$DSTART"; printf 'STALE\n'; printf '%s\n' "$DEND"; } > "$SBX/AGENTS.md"
LOG="$(run_sync_log)"
assert "seeded surface, de-listed: the block is stripped"        '[ "$(blocks_in "$SBX/CLAUDE.md")" = "0" ]'
assert "seeded surface, de-listed: the file is NOT deleted"      '[ -f "$SBX/CLAUDE.md" ]'
# The precondition for the advisory being about anything: strip really did leave it contentless.
assert "seeded surface, de-listed: what remains is empty"        '[ -z "$(tr -d "[:space:]" < "$SBX/CLAUDE.md")" ]'
# One line must name the file AND say docket is leaving it — the removal line already names the same
# path, so two independent greps over the log would pass with the advisory deleted.
NAMED="$(grep -F -- "$PHYSSBX/CLAUDE.md" <<<"$LOG")"
assert "seeded surface, de-listed: one diagnostic names the now-empty file" \
  '[ -n "$NAMED" ] && grep -q "now EMPTY" <<<"$NAMED"'
assert "seeded surface, de-listed: that diagnostic says docket will not delete it" \
  'grep -q "delete it by hand" <<<"$NAMED"'
# The negative control seeded above: its block went, its prose stayed, and no advisory named it.
# Two steps, never a pipe into `grep -q` (AGENTS.md, shell: SIGPIPE under pipefail).
AGM_NAMED="$(grep -F -- "$PHYSSBX/AGENTS.md" <<<"$LOG" || true)"
assert "negative control: the prose-carrying surface was stripped too" \
  '[ "$(blocks_in "$SBX/AGENTS.md")" = "0" ] && grep -qxF "# keep me" "$SBX/AGENTS.md"'
assert "negative control: it was named by the run, so the advisory assert is not vacuous" \
  '[ -n "$AGM_NAMED" ]'
assert "negative control: no advisory is emitted for a file that was NOT emptied" \
  '! grep -q "now EMPTY" <<<"$AGM_NAMED"'

echo; [ "$fail" = 0 ] && echo "ALL PASS" || echo "FAILURES"; exit "$fail"
