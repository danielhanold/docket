#!/usr/bin/env bash
# tests/test_sync_agents_runners_gates.sh — the runner-config generation gates and atomicity half
# of the runner suite (sharded out of tests/test_sync_agents_runners.sh at change 0324, when the
# 17-agent matrix pushed the single file to ~96s against the hard 60s ceiling and the table's remedy
# is a shard, never a bigger number). Covers the build-* --worktree slot (changes 0206/0208), the
# reserved / unregistered-runner rules, the required-model rule (change 0205), atomic all-or-nothing
# wrapper generation (changes 0207/0220), and the no-runner native-output fence. The shim rendering
# + shim-pin config stayed in tests/test_sync_agents_runners.sh; the resolved-pin injection is in
# tests/test_sync_agents_runners_pins.sh.
# Run: bash tests/test_sync_agents_runners_gates.sh
# shellcheck source=lib/sync_agents_common.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/sync_agents_common.sh"

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
# 0271: the launch-then-observe rewrite makes it two invocations of the same one seam. The
# property this assert defends is unchanged — every dispatch in a build-* shim goes through the
# facade and nothing else — so the count moves with the shim rather than the assert being dropped.
assert "0206: exactly two dispatch invocations in the build-* shim" \
  '[ "$(grep -cF "docket.sh runner-dispatch" "$B")" = "2" ]'

# ---- 0208: the --worktree slot keys on the DECLARED scope, not on a name shape --------
# The 0206 asserts above are still the mirror correspondence; this widens the population they run
# over. `review-lean` is feature-scoped and matches no `build-*` name shape, so it is the leg that
# distinguishes a scope-keyed gate from the old case statement — under the old rule its shim
# carries no slot, which is exactly the silent main-tree anchor 0206 exists to eliminate.
rm -rf "$SBX"
mkgitrepo
mkdir -p "$SBX/.claude"
printf 'agents:\n  claude:\n    review-lean: { model: test-model-x, runner: codex }\n    adr: { model: test-model-y, runner: codex }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
R="$SBX/.claude/agents/docket-review-lean.md"
A="$SBX/.claude/agents/docket-adr.md"
assert "0208(b): fixture sanity — review-lean generated a real shim" \
  'grep -qF "docket.sh runner-dispatch" "$R"'
assert "0208(b): fixture sanity — adr generated a real shim" \
  'grep -qF "docket.sh runner-dispatch" "$A"'
assert "0208(b): a feature-scoped NON-build shim bakes --worktree" \
  'grep -qF -- "--worktree" "$R"'
assert "0208(b): the feature-scoped rule text is generic, not build-specific" \
  '! grep -qiF "this is a BUILD worker" "$R"'
assert "0208(b): a metadata-scoped shim still carries no --worktree" \
  '! grep -qF -- "--worktree" "$A"'
rm -rf "$SBX"

# ---- 0208: a source with no worktree-scope FAILS generation loudly --------------------
# The generation gate is where absence is PREVENTABLE — a future feature-scoped agent must not be
# able to ship undeclared. AGENTS_SRC in sync-agents.sh is hardcoded ($SCRIPT_DIR/agents, no seam),
# so the fixture copies the generator's own inputs and strips the key from one source in the COPY —
# the mutation-fixture pattern tests/test_docket_status.sh already uses — rather than adding a
# generator seam that exists only for this test.
#
# THE COPY IS THE GENERATOR'S INPUT SET, established by inspection of sync-agents.sh's $SCRIPT_DIR
# reads, not by copying the repo: `scripts/lib/` (the sourced libs), `agents/` (the sources plus
# harness-defaults.yml) and `cursor-rules/`. It reads nothing under `skills/` — it only NAMES skills
# in generated prose — and nothing in `scripts/` outside `lib/`. A wrong input set does not pass
# quietly here: the asserts below key on the refusal's TEXT, so a run that died for a missing input
# reddens rather than counting as the refusal under test.
mkgitrepo
mkdir -p "$SBX/.claude"
COPY="$SBX/docketcopy"
mkdir -p "$COPY/scripts"
cp "$REPO/sync-agents.sh" "$COPY/"
cp -R "$REPO/agents" "$COPY/agents"
cp -R "$REPO/scripts/lib" "$COPY/scripts/lib"
[ -d "$REPO/cursor-rules" ] && cp -R "$REPO/cursor-rules" "$COPY/cursor-rules"
# Strip the key from TWO sources, in the two shapes absence actually takes. `sed -i` is not portable
# to BSD without an argument, so rewrite through a temp file beside the destination.
#   review-lean  — plain absence.
#   review-deep  — absence WITH a column-0 `worktree-scope:` line in the BODY. That is the
#                  anchoring leg: the shared reader (scripts/lib/docket-agent-scope.sh) scans only
#                  the first ---…--- block, so this source is ABSENT and the run must still refuse.
#                  A bare column-0 match reads the body prose as a declaration, `feature` validates,
#                  and this agent silently disappears from the refusal — which is why the assert
#                  below names BOTH agents rather than just the first.
sed '/^worktree-scope:/d' "$COPY/agents/docket-review-lean.md" > "$COPY/agents/.tmp" \
  && mv -f "$COPY/agents/.tmp" "$COPY/agents/docket-review-lean.md"
sed '/^worktree-scope:/d' "$COPY/agents/docket-review-deep.md" > "$COPY/agents/.tmp" \
  && mv -f "$COPY/agents/.tmp" "$COPY/agents/docket-review-deep.md"
printf '\nworktree-scope: feature\n' >> "$COPY/agents/docket-review-deep.md"
assert "0208(b): fixture sanity — the key really was stripped from the copy" \
  '! grep -q "^worktree-scope:" "$COPY/agents/docket-review-lean.md"'
fmdecl="$(awk '/^---[[:space:]]*$/{n++} n==1 && /^worktree-scope:/{c++} END{print c+0}' \
  "$COPY/agents/docket-review-deep.md")"
assert "0208(b): fixture sanity — the decoy source declares nothing in FRONTMATTER and everything in BODY" \
  '[ "$fmdecl" = "0" ] && grep -qx "worktree-scope: feature" "$COPY/agents/docket-review-deep.md"'
out="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$COPY/sync-agents.sh" 2>&1 )"; rc=$?
assert "0208(b): a missing worktree-scope fails generation" '[ "$rc" != "0" ]'
assert "0208(b): the refusal names the key and the agent" \
  'grep -qF "worktree-scope" <<<"$out" && grep -qF "review-lean" <<<"$out"'
assert "0208(b): body prose is not a declaration — the anchored read refuses the decoy source too" \
  'grep -qF "review-deep" <<<"$out"'
assert "0208(b): and no wrappers were written" '[ ! -e "$SBX/.claude/agents/docket-adr.md" ]'
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
  # `! -e`, not `! -s`: change 0207 gave this rule the fail-before-write property it lacked. The
  # gate runs above the first emit_wrapper redirection, so the offending agent's file is never
  # created rather than created-and-truncated.
  assert "0205/$rnr: no wrapper was written for the offending agent" \
    '[ ! -e "$SBX/.claude/agents/docket-status.md" ]'
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

# ---- change 0207: wrapper generation is ATOMIC ----------------------------------------------
# A bad runner: config is detected before the FIRST wrapper write. Previously emit_wrapper failed
# inline, mid-loop, with its stdout already redirected into the target — so the offending agent was
# left zero-length and every agent later in glob order was never regenerated. The invariant now:
# a run either regenerates every wrapper or changes nothing on disk (nginx -t semantics).

# (1) FRESH tree + bad runner => NO wrapper files exist at all, for ANY agent.
mkgitrepo
mkdir -p "$SBX/.claude"
printf 'agents:\n  claude:\n    status: { runner: codex }\n' > "$SBX/.docket.yml"
err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" 2>&1 >/dev/null )"; rc=$?
assert "0207: a bad runner config fails the run nonzero" '[ "$rc" != "0" ]'
# `! -e`, not `! -s`: change 0207 makes this a fail-BEFORE-write property. The offending agent's
# file is never created, so the zero-length-wrapper case the 0205 comment described is gone.
assert "0207: fresh tree — the offending agent has no wrapper at all" \
  '[ ! -e "$SBX/.claude/agents/docket-status.md" ]'
# The whole point: OTHER agents are not written either. Under the old mid-loop abort, every agent
# ahead of docket-status in glob order was already on disk by the time it failed.
assert "0207: fresh tree — NO wrapper was written for any agent" \
  '[ "$(find "$SBX/.claude/agents" -name "docket-*.md" 2>/dev/null | wc -l | tr -d " ")" = "0" ]'
assert "0207: the summary names the whole-run consequence" \
  'grep -qiE "no wrappers were written" <<<"$err"'
rm -rf "$SBX"

# (2) PRE-EXISTING wrappers + bad runner => every wrapper BYTE-IDENTICAL to before the run.
# This is the invariant the change exists to create and had no test before it.
mkgitrepo
mkdir -p "$SBX/.claude"
printf 'agents:\n  claude:\n    status: { runner: codex, model: some/model-id }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
assert "0207: (fixture) the good config generated wrappers to preserve" \
  '[ -s "$SBX/.claude/agents/docket-status.md" ]'
before="$(mktemp -d)"; cp -R "$SBX/.claude/agents/." "$before/"
# Now break it: drop the model: from the SAME entry.
printf 'agents:\n  claude:\n    status: { runner: codex }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 ); rc=$?
assert "0207: the run over pre-existing wrappers still fails nonzero" '[ "$rc" != "0" ]'
assert "0207: every pre-existing wrapper survives byte-untouched" \
  'diff -r "$before" "$SBX/.claude/agents" >/dev/null'
rm -rf "$SBX" "$before"

# (3) MULTIPLE offenders across different agents => all named in ONE run.
mkgitrepo
mkdir -p "$SBX/.claude"
printf 'agents:\n  claude:\n    status: { runner: codex }\n    adr: { runner: gemini-cli }\n' > "$SBX/.docket.yml"
err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" 2>&1 >/dev/null )"; rc=$?
assert "0207: multiple offenders fail the run nonzero" '[ "$rc" != "0" ]'
assert "0207: the first offender is named" 'grep -qF "docket-status" <<<"$err"'
assert "0207: the SECOND offender is named in the same run" 'grep -qF "docket-adr" <<<"$err"'
# Accumulating, not short-circuiting: the unregistered one must report its OWN rule, not be
# swallowed by whichever offender the walk happened to reach first.
#
# Extract the OFFENDING AGENT'S OWN LINE first (change 0220). Matching the runner name against the
# whole output could not tell the two rules apart — the name appears in both diagnostics, so
# swapping the if-blocks in runner_config_error left this green. A whole-output negative assert is
# not available either: docket-status in this same fixture is legitimately model-less, so the
# required-model wording IS in $err against a correct implementation.
#
# Capture each offender's own lines FIRST, then grep the capture — never `grep … | head -n1`: an
# early-exiting consumer under `set -o pipefail` (this file's `set -uo pipefail`) can SIGPIPE the
# producer and turn 141 into an intermittent failure. Same spelling as the 0220/D6 block below.
d4_adr_lines="$(grep -F "docket-adr" <<<"$err")"
assert "0220/D4: (fixture) the unregistered offender produced a diagnostic line" '[ -n "$d4_adr_lines" ]'
assert "0220/D4: the unregistered offender's OWN line reports the REGISTRATION rule" \
  'grep -qF "is not a registered runner" <<<"$d4_adr_lines"'
assert "0220/D4: and that same line does NOT report the required-model rule" \
  '! grep -qF "requires an explicit model" <<<"$d4_adr_lines"'
# The companion direction: the registered-but-model-less agent reports the OTHER rule on its line.
d4_status_lines="$(grep -F "docket-status" <<<"$err")"
assert "0220/D4: the model-less offender's own line reports the required-model rule" \
  '[ -n "$d4_status_lines" ] && grep -qF "requires an explicit model" <<<"$d4_status_lines"'
rm -rf "$SBX"

# (4) --check reports the failure and exits nonzero (docket's `nginx -t`).
mkgitrepo
mkdir -p "$SBX/.claude"
printf 'agents:\n  claude:\n    status: { runner: codex }\n' > "$SBX/.docket.yml"
err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check 2>&1 >/dev/null )"; rc=$?
assert "0207: --check fails on a bad runner config" '[ "$rc" != "0" ]'
assert "0207: --check says a real run would refuse to write wrappers" \
  'grep -qiE "would refuse to write wrappers" <<<"$err"'
# (This assert was vacuous as written — leg (c) redirects into a mktemp -d, so --check never wrote
# into .claude/agents even pre-0207. The property it was reaching for is pinned by the 0220/D1
# fixture below, which proves --check reaches its own return rather than exiting mid-leg.)
# Positive clause first: absence alone cannot tell "the gate exited before the legs" apart from
# "the run died before reaching either log site". The gate's own line proves it was the gate.
assert "0207: --check exits before check_project_level runs its legs" \
  'grep -qF "check: runner configuration is invalid" <<<"$err" && ! grep -qF "nothing else to check" <<<"$err" && ! grep -qF "advisory" <<<"$err"'
rm -rf "$SBX"

# NON-VACUITY COMPANION for the whole 0207 block: the same shape with a VALID runner config must
# generate the full set. Without this, every assert above stays green if sync-agents.sh broke for
# an unrelated reason and wrote nothing at all.
mkgitrepo
mkdir -p "$SBX/.claude"
printf 'agents:\n  claude:\n    status: { runner: codex, model: some/model-id }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 ); rc=$?
assert "0207: a VALID runner config still generates (the guards above are not vacuous)" '[ "$rc" = "0" ]'
assert "0207: and the full built-in set lands" \
  '[ "$(find "$SBX/.claude/agents" -name "docket-*.md" | wc -l | tr -d " ")" = "17" ]'
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

# ---- change 0220 / D1: the gate and leg (c) share ONE predicate -------------------------------
# The gap this closes: with a GLOBAL agent_harnesses: list that omits claude, $USER_TARGETS has no
# claude, so the gate's user-level leg never sees agents.claude.*; in a repo that is NOT opted in
# the gate's project-level leg `continue`s; but leg (c) iterated $HARNESSES (default claude) and
# called emit_wrapper, which died on its can't-happen assertion — raw ERROR + exit 1, skipping the
# remaining --check legs and leaking leg (c)'s mktemp -d.
#
# The assertion shape is INVERTED from the obvious one on purpose. In this fixture the gate itself
# is legitimately silent (runner_config_error returns 0 on the user leg because USER_TARGETS has no
# claude; the project leg continues), and on the --check path a FAILING gate exit 1s before
# check_project_level runs at all — so "the gate now catches it" and "the remaining legs still run
# after a gate failure" both describe unreachable paths. What is provable is that leg (c) no longer
# runs at all in a repo with no per-repo wrappers: no runner ERROR, no false advisory, and rc = 0.
#
# rc = 0 is the load-bearing assert: emit_wrapper's failure path is a hard `exit 1`, so rc can only
# be 0 if check_project_level reached its own `return $rc`. The .gitignore docket block is
# pre-written so leg (a) passes and rc = 0 is meaningful rather than vacuously non-zero.
mkgitrepo
mkdir -p "$SBX/.config/docket"
# global layer: harness list WITHOUT claude, plus a bad (model-less) claude runner
printf 'agent_harnesses: [codex]\nagents:\n  claude:\n    status: { runner: codex }\n' \
  > "$SBX/.config/docket/config.yml"
# NO .docket.yml and NO .docket.local.yml => per_repo_opted_in is false.
# gitignore_block_wanted is still TRUE (the block below), which is exactly the weaker predicate.
( . "$REPO/scripts/lib/docket-gitignore-block.sh" && emit_docket_gitignore_block ) > "$SBX/.gitignore"
d1_err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check 2>&1 >/dev/null )"; d1_rc=$?
assert "0220/D1: --check completes rather than aborting inside leg (c) (rc=0)" \
  '[ "$d1_rc" = "0" ]'
assert "0220/D1: no raw runner ERROR escapes from leg (c)" \
  '! grep -qF "requires an explicit model" <<<"$d1_err"'
# The false advisory D1 removes as a side effect: project_level_pass writes nothing in a
# non-opted-in repo, so leg (c) reporting "not generated on this machine" was always wrong.
# This assert only discriminates here by accident of iteration order: leg (c) walks docket-*.md
# alphabetically, and the bad runner in this fixture is pinned to `status`, so docket-adr et al.
# reach the advisory before docket-status trips emit_wrapper's can't-happen exit. Move the bad
# runner onto the first agent and the advisory would never print, leaving this line green under the
# mutation it exists to catch. Kept (it is cheap and states leg (c)'s full silent shape), but the
# D1b fixture below is the guard that does not depend on which agent aborts first.
assert "0220/D1: no false leg-(c) advisory for un-generated per-repo wrappers" \
  '! grep -qF "not generated on this machine" <<<"$d1_err"'
# Non-vacuity: the fixture really did put the weaker predicate in the TRUE state, so the assert
# above is about the shared predicate and not about check_project_level having returned early.
assert "0220/D1: (fixture) gitignore_block_wanted was true — leg (a) ran and passed" \
  '! grep -qF "nothing else to check" <<<"$d1_err"'
rm -rf "$SBX"

# ---- change 0220 / D1b: the same shape with a VALID runner — the DISCRIMINATING advisory guard --
# One variable changes from the D1 fixture above: the global claude runner carries its required
# `model:`, so nothing in leg (c) can abort. Pre-fix, leg (c) therefore ran to completion and
# printed "not generated on this machine" once per built-in agent — the false advisory D1 removes
# ("Consequence — a second bug fixed as a side effect" in the design spec), since a non-opted-in
# repo's wrappers are not merely missing, they are never generated at all. Post-fix the shared
# `project_wrappers_generated` early return fires first and the advisory is absent.
#
# Mutation-verified: deleting `if ! project_wrappers_generated; then return $rc; fi` from
# check_project_level turns the advisory assert below NOT OK (17 advisory lines are emitted).
mkgitrepo
mkdir -p "$SBX/.config/docket"
printf 'agent_harnesses: [codex]\nagents:\n  claude:\n    status: { runner: codex, model: some/model-id }\n' \
  > "$SBX/.config/docket/config.yml"
# As in D1: no .docket.yml and no .docket.local.yml => per_repo_opted_in false, while the
# pre-written block keeps the weaker gitignore_block_wanted predicate true.
( . "$REPO/scripts/lib/docket-gitignore-block.sh" && emit_docket_gitignore_block ) > "$SBX/.gitignore"
d1b_err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check 2>&1 >/dev/null )"; d1b_rc=$?
assert "0220/D1b: --check completes (rc=0)" '[ "$d1b_rc" = "0" ]'
assert "0220/D1b: (fixture) gitignore_block_wanted was true — leg (a) ran and passed" \
  '! grep -qF "nothing else to check" <<<"$d1b_err"'
# Non-vacuity for the negative below: this runner config is VALID, so leg (c) had no abort of its
# own to hide behind — if it ran, it ran to the advisory.
assert "0220/D1b: (fixture) the runner config is valid — no runner diagnostic at all" \
  '! grep -qF "requires an explicit model" <<<"$d1b_err" && ! grep -qF "is not a registered runner" <<<"$d1b_err"'
assert "0220/D1b: no false leg-(c) advisory for un-generated per-repo wrappers" \
  '! grep -qF "not generated on this machine" <<<"$d1b_err"'
rm -rf "$SBX"

# ---- change 0220 / D1c: prune_orphans' per-repo legs use the SAME named boundary ----------------
# D1's thesis is that the per-repo wrapper-lifecycle boundary is spelled once and NAMED. Legs (1a)
# and (2) delete exactly the files project_level_pass writes (`$REPO/.<harness>/agents/docket-*` and
# the per-repo dispatch rule), so they gate on the same concept as the writer and must move with it.
# Keyed on shape — the raw predicate appearing ANYWHERE in the function body is the failure — rather
# than on a hand-listed set of call sites, so a fourth per-repo leg cannot slip past the assert.
prune_body="$(awk '/^prune_orphans\(\)/{f=1} f{print} f&&/^}$/{exit}' "$REPO/sync-agents.sh")"
assert "0220/D1c: (fixture) the prune_orphans body was extracted whole" \
  '[ -n "$prune_body" ] && grep -qF "de-listed per-repo harness" <<<"$prune_body"'
assert "0220/D1c: leg (1a) gates on the named predicate" \
  'within "$REPO/sync-agents.sh" "(1a) per-repo removed-builtin dirs" "project_wrappers_generated" 200'
assert "0220/D1c: leg (2) gates on the named predicate" \
  'within "$REPO/sync-agents.sh" "(2) de-listed per-repo harness" "project_wrappers_generated" 400'
assert "0220/D1c: and prune_orphans calls the raw per_repo_opted_in nowhere" \
  '! grep -qF "per_repo_opted_in" <<<"$prune_body"'

# ---- change 0220 / D2: the gate's USER-LEVEL leg, exercised through the GLOBAL layer -----------
# Every other runner: fixture writes .docket.yml (the project layer), so the whole
# `for harness in $USER_TARGETS` block — inline in validate_runner_config then, in the gates' shared
# for_each_candidate_triple walk now — was mutation-survivable: delete it and nothing reddened.
# (Change 0269's review gave the shim-pin gate the same walk, so this leg now protects both.)
# This is the leg that protects ~/.claude/agents — the widest blast radius of
# the original change-0079/0205 bug. The repo here has NO .docket.yml and NO .docket.local.yml, so
# per_repo_opted_in is false and the project-level leg `continue`s: only the user-level leg can
# catch this, and rc != 0 is therefore attributable to it alone.
D2REPO="$(mktemp -d)"; D2ROOT="$(mktemp -d)"
mkdir -p "$D2ROOT/.claude" "$D2ROOT/.config/docket"
printf 'agents:\n  claude:\n    status: { runner: codex }\n' > "$D2ROOT/.config/docket/config.yml"
d2_err="$( cd "$D2REPO" && DOCKET_HARNESS_ROOT="$D2ROOT" bash "$SYNC" 2>&1 >/dev/null )"; d2_rc=$?
assert "0220/D2: a bad runner in the GLOBAL layer fails the real run nonzero" '[ "$d2_rc" != "0" ]'
assert "0220/D2: the user-level diagnostic names the agent" 'grep -qF "docket-status" <<<"$d2_err"'
assert "0220/D2: and names the required-model rule" \
  'grep -qF "requires an explicit model" <<<"$d2_err"'
# The protected behavior, stated as behavior: ~/.claude/agents is never generated from bad config.
assert "0220/D2: NO user-level wrapper was written for any agent" \
  '[ "$(find "$D2ROOT/.claude/agents" -name "docket-*.md" 2>/dev/null | wc -l | tr -d " ")" = "0" ]'
# --check must reach the same verdict (this is the path where compute_user_targets has not run and
# USER_TARGETS/USER_HARNESSES_SET are unset under set -u).
d2c_err="$( cd "$D2REPO" && DOCKET_HARNESS_ROOT="$D2ROOT" bash "$SYNC" --check 2>&1 >/dev/null )"; d2c_rc=$?
assert "0220/D2: --check fails on the same global-layer config" '[ "$d2c_rc" != "0" ]'
assert "0220/D2: --check says a real run would refuse to write wrappers" \
  'grep -qiE "would refuse to write wrappers" <<<"$d2c_err"'
# Non-vacuity companion: the SAME shape with a model present must generate the full set, so the
# asserts above cannot be satisfied by sync-agents.sh failing for an unrelated reason.
rm -rf "$D2ROOT/.claude"; mkdir -p "$D2ROOT/.claude"
printf 'agents:\n  claude:\n    status: { runner: codex, model: some/model-id }\n' \
  > "$D2ROOT/.config/docket/config.yml"
( cd "$D2REPO" && DOCKET_HARNESS_ROOT="$D2ROOT" bash "$SYNC" >/dev/null 2>&1 ); d2v_rc=$?
assert "0220/D2: a VALID global runner config still generates (not vacuous)" '[ "$d2v_rc" = "0" ]'
assert "0220/D2: and the full built-in set lands user-level" \
  '[ "$(find "$D2ROOT/.claude/agents" -name "docket-*.md" | wc -l | tr -d " ")" = "17" ]'
rm -rf "$D2REPO" "$D2ROOT"

# ---- change 0220 / D3: emit_wrapper's $2 == $RES_MODEL calling contract ------------------------
# emit_wrapper keeps its OWN copy of the provenance filter (RES_MODEL_FROM_USER over positional $2)
# rather than calling user_flag_model. The two agree only because all three call sites pass
# $RES_MODEL immediately after resolve_agent_layers. Nothing documented or enforced that, so a
# future call site passing a post-processed model would silently reintroduce the mid-loop abort
# 0207 exists to prevent. The contract is now on the header AND asserted; this fixture pins the
# assertion by calling emit_wrapper directly with a mismatched $2.
d3_out="$(
  set +e
  # shellcheck source=/dev/null
  DOCKET_HARNESS_ROOT="$(mktemp -d)" bash -c '
    set -uo pipefail
    . "$1" 2>/dev/null || true
    RES_MODEL="the-resolved-one"; RES_EFFORT=""; RES_MODEL_FROM_USER=1; RES_EFFORT_FROM_USER=0
    emit_wrapper "$2" "a-DIFFERENT-model" "" "" "claude" "status"
  ' _ "$REPO/sync-agents.sh" "$REPO/agents/docket-status.md" 2>&1
  printf 'RC=%s' "$?"
)"
assert "0220/D3: emit_wrapper aborts when \$2 is not the resolved RES_MODEL" \
  '! grep -qF "RC=0" <<<"$d3_out"'
assert "0220/D3: and the abort names the contract" \
  'grep -qiE "RES_MODEL" <<<"$d3_out"'
# Non-vacuity: the SAME call with $2 == $RES_MODEL must succeed, so the assert above is about the
# mismatch and not about emit_wrapper failing to run at all in this harness.
d3_ok="$(
  set +e
  DOCKET_HARNESS_ROOT="$(mktemp -d)" bash -c '
    set -uo pipefail
    . "$1" 2>/dev/null || true
    RES_MODEL="the-resolved-one"; RES_EFFORT=""; RES_MODEL_FROM_USER=1; RES_EFFORT_FROM_USER=0
    emit_wrapper "$2" "the-resolved-one" "" "" "claude" "status"
  ' _ "$REPO/sync-agents.sh" "$REPO/agents/docket-status.md" >/dev/null 2>&1
  printf 'RC=%s' "$?"
)"
assert "0220/D3: (non-vacuity) a matching \$2 still emits successfully" '[ "$d3_ok" = "RC=0" ]'
# And the contract is stated where a caller reads it, not only enforced.
# Anchored on a verbatim clause unique to the comment block: `within` indexes FORWARD from its
# second argument, so anchoring on `emit_wrapper(){` searches only the body and would be satisfied
# by the body's own assertion even with the whole header deleted.
assert "0220/D3: emit_wrapper's header states the \$2 contract" \
  'grep -qF "CALLING CONTRACT (change 0220, amended by change 0269)" "$REPO/sync-agents.sh"'

# ---- change 0220 / D6: each distinct diagnostic is reported exactly once ------------------------
# A bad runner: in the GLOBAL layer is visible to BOTH gate legs — the user-level leg resolves over
# $GLOBAL_CFG, the project-level leg over local ⊕ committed ⊕ global — so it was logged twice,
# verbatim identically, against a README that promises every offender "in one pass".
#
# This fixture is ALSO the over-dedupe guard, which is the failure mode the dedup code introduces.
# The two legs here yield DISTINCT diagnostics: .docket.yml sets an unregistered runner on status
# (project leg only — the global layer has no status entry), while the global config sets a
# registered-but-model-less runner on adr (visible to BOTH legs, hence the duplicate). Deduping on
# the rendered string must collapse adr's two identical copies while leaving status's different
# diagnostic untouched.
mkgitrepo
mkdir -p "$SBX/.claude" "$SBX/.config/docket"
printf 'agents:\n  claude:\n    status: { runner: gemini-cli }\n' > "$SBX/.docket.yml"
printf 'agents:\n  claude:\n    adr: { runner: codex }\n' > "$SBX/.config/docket/config.yml"
d6_err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" 2>&1 >/dev/null )"; d6_rc=$?
assert "0220/D6: (fixture) the run still fails nonzero" '[ "$d6_rc" != "0" ]'
# The dedup itself: the doubly-visible global offender is logged ONCE, not once per leg.
assert "0220/D6: a diagnostic visible to both legs is reported exactly once" \
  '[ "$(grep -cF "docket-adr" <<<"$d6_err")" = "1" ]'
# The OVER-dedupe guard: a genuinely different offender must survive the filter.
assert "0220/D6: a distinct offender is NOT suppressed by the dedup" \
  '[ "$(grep -cF "docket-status" <<<"$d6_err")" -ge 1 ]'
# Capture each offender's own lines FIRST, then grep the capture: a `grep … | grep -q` pipeline
# under `set -o pipefail` (this file's `set -uo pipefail`) can take SIGPIPE and turn 141 into an
# intermittent failure.
d6_status_lines="$(grep -F "docket-status" <<<"$d6_err")"
d6_adr_lines="$(grep -F "docket-adr" <<<"$d6_err")"
assert "0220/D6: and it keeps its own rule" \
  'grep -qF "is not a registered runner" <<<"$d6_status_lines"'
assert "0220/D6: while the deduped one keeps its own, different rule" \
  'grep -qF "requires an explicit model" <<<"$d6_adr_lines"'
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
exit $fail
