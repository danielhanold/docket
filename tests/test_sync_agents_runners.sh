#!/usr/bin/env bash
# tests/test_sync_agents_runners.sh — runner delegation shims and their own shim-pin config
# (change 0079 runner shims + change 0269 runners.<name>.shim_model / shim_effort gates). Sharded
# out of the whole-file runner suite at change 0324: once the plan-writer agent grew the matrix to
# 17 agents the single file measured ~96s serially against the table's hard 60s ceiling, and the
# remedy for a file over its ceiling is a shard, never a bigger number (tests/runtime-budgets.tsv).
# The generation gates + atomicity moved to tests/test_sync_agents_runners_gates.sh and the
# resolved-pin injection to tests/test_sync_agents_runners_pins.sh. Add a shim-rendering or
# shim-pin-config assertion here; a generation-gate/atomicity one to _gates; a pin-injection one to
# _pins.
# Run: bash tests/test_sync_agents_runners.sh
# shellcheck source=lib/sync_agents_common.sh
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/sync_agents_common.sh"

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
# change 0269: the shim's frontmatter pin governs the PARENT-side shim agent (Claude Code runs the
# relay), so it must be resolvable by the parent — never the child's model, which Claude Code cannot
# resolve. This replaces 0079's "shim keeps frontmatter model (bookkeeping)" assert, whose premise
# was false: Claude Code reads the line as the live pin, so every delegated wrapper was born broken.
assert "0269: shim frontmatter model defaults to inherit" '[ "$(fm "$G" model)" = "inherit" ]'
assert "0269: shim frontmatter effort defaults to low" '[ "$(fm "$G" effort)" = "low" ]'
# THE REGRESSION ASSERT — the check whose absence let the defect ship. Derived from the two values
# rather than hardcoded, so it keeps biting when the fixture's model ID changes.
_fm_model="$(fm "$G" model)"
_baked_model="$(sed -n 's/.*--model \([^ ]*\).*/\1/p' "$G" | sed -n 1p)"
assert "0269: fixture sanity — a model IS baked into the dispatch line" '[ -n "$_baked_model" ]'
assert "0269: shim frontmatter model is NEVER the value baked into --model" \
  '[ "$_fm_model" != "$_baked_model" ]'
assert "0079: shim body invokes docket.sh runner-dispatch" 'grep -qF "docket.sh runner-dispatch" "$G"'
assert "0079: shim body pins --runner codex" 'grep -qF -- "--runner codex" "$G"'
assert "0079: shim body pins --agent status" 'grep -qF -- "--agent status" "$G"'
assert "0079: shim body bakes the resolved model" 'grep -qF -- "--model gpt-5.1-codex" "$G"'
assert "0079: shim body bakes the resolved effort" 'grep -qF -- "--effort high" "$G"'
# change 0271: the shim no longer bounds the whole delegated run inside one foreground call —
# that ceiling (600000 ms, the Bash tool's maximum and not a tunable) is the defect. The
# instruction is now launch-then-observe. This assert REPLACES 0079's "one foreground call"
# guard: the old one is not restored, because keeping it green would reinstate the defect.
assert "0271: shim bakes the --launch verb" 'grep -qF -- "--launch" "$G"'
assert "0271: shim bakes the --observe verb" 'grep -qF -- "--observe" "$G"'
# DETECTS THE REMOVAL, not the addition (LEARNINGS: assert-detects-removal-not-replacement).
assert "0271: shim no longer forbids backgrounding the dispatch" \
  '! grep -qi "never background it" "$G"'
assert "0271: shim no longer demands ONE foreground call" \
  '! grep -qi "one foreground" "$G"'
assert "0271: shim no longer bakes the 600000 ceiling" '! grep -qF "600000" "$G"'
# Exit 4 is NOT a failure, and this shim is its ONLY consumer — so the wrapper must name it
# explicitly rather than inheriting the bare-non-zero rule
# (LEARNINGS: exit-code-encodes-a-non-failure). Spelled with an explicit non-digit boundary
# rather than `\b`: `\b` is a GNU/ugrep extension that BSD `grep -E` does not honor, and PATH
# `grep` here is ugrep, so a `\b` spelling would pass locally and rot on a stock macOS grep.
# Keyed on the PAIRING, not on a bare digit: an isolated `4` appears in frontmatter and in baked
# `--model` values, so a digit-only match stays green after the whole `4 — still running` bullet
# is deleted, which is the one thing this guard exists to catch. Flattened to one line so the
# code and its meaning must co-occur regardless of wrapping.
assert "0271: shim names exit 4 as the observe-again code" \
  'grep -qE "(^|[^0-9])4[^0-9][^.]*still running" <<<"$(tr "\n" " " < "$G")"'
assert "0271: shim still forbids the inline fallback" 'grep -qiE "never.*inline" "$G"'
# Shape, not spelling: the clause may name what is not retried between the verb and the adverb.
assert "0271: shim still forbids a silent retry" 'grep -qiE "never retry[^.]*silently" "$G"'
assert "0079: shim replaced the native body" '! grep -qF "Execute docket-status to refresh docket state" "$G"'
# 0271: two INVOCATIONS (launch + observe), still exactly ONE dispatch SEAM — both are the same
# facade. ADR-0038's chokepoint property is about the seam, not the call count.
assert "0271: exactly two dispatch invocations in the shim" \
  '[ "$(grep -cF "docket.sh runner-dispatch" "$G")" = "2" ]'
# The caller's task text is the child's ONLY input — it inherits no conversation. Rendered as an
# optional trailing `[-- <caller args>]`, the model omitted it on live dispatches and the child
# improvised from the worktree, so the run LOOKED successful. These asserts defend the two
# properties that made the omission possible: the slot must be UNBRACKETED (not readable as
# optional) and it must sit ON the launch line (not in trailing prose that can be skimmed past).
# Captured into a variable before searching — `grep … | grep -q` would SIGPIPE the producer
# under pipefail. `grep -qE --` is mandatory throughout: every pattern here leads with `--`, and
# a bare leading `--` is parsed as an option (exit 2), which inside the negations below would be
# permanently, vacuously green.
# DETECTS THE REMOVAL (LEARNINGS: assert-detects-removal-not-replacement): the bracketed spelling
# is the defect itself, so it must not reappear anywhere in the shim, on the launch line or below.
assert "0271: shim no longer renders the task slot as optional brackets" \
  '! grep -qF -- "[--" "$G"'
# change 0277: the task brief no longer travels as shell argv. The shim teaches ONE path — write
# the brief with a quoted-delimiter heredoc and launch with --brief-file in the SAME Bash call —
# because two taught paths let the model pick the lossy one. Any slot the model must still fill
# stays UNBRACKETED for 0271's reason: a bracketed rendering reads as optional and was in fact
# dropped on live dispatches.
assert "0277: shim teaches an explicitly templated mktemp for the brief" \
  'grep -qF -- "TMPDIR:-/tmp" "$G"'
assert "0277: shim teaches a QUOTED-delimiter heredoc (every character literal)" \
  'grep -qF -- "<<'\''DOCKET_BRIEF_EOF'\''" "$G"'
assert "0277: shim closes the heredoc" 'grep -qxF -- "DOCKET_BRIEF_EOF" "$G"'
launch_line="$(grep -F -- "--launch" "$G")"
# The brief path is no longer a slot at all: the launch rides in the same call as the write, so
# the argument is a live expansion. DETECTS THE REMOVAL of that property — an argument carrying an
# angle-bracket placeholder is the shipped defect returning.
assert "0277: the launch line's --brief-file argument is a live value, not a slot to fill" \
  '! grep -qE -- "--brief-file[[:space:]]+[^[:space:]]*<" <<<"$launch_line"'
# THE RECIPE MUST ACTUALLY YIELD A USABLE PATH (0277 whole-branch review, blocker). Every assert
# around this one pins the SHAPE of the launch line; none of them pins that a model FOLLOWING the
# recipe hands the facade a readable brief. The first shipped form did not, and every assert stayed
# green over it: it taught "two foreground Bash calls", harness Bash calls share no shell state, and
# mktemp's suffix is random — so at the launch the model had never seen the path, `$BRIEF` expanded
# empty in a fresh shell, and the facade died on `--brief-file requires a path`. That is a total
# failure of the SOLE taught channel, for every runner and every delegated agent.
#
# Guarded by EXECUTING the emitted recipe against a stub facade rather than by matching prose: a
# reworded shim cannot fool it, and a future split back into two calls cannot survive it. Run
# against the STATUS shim on purpose — a build shim's launch line still carries the caller-supplied
# `<feature worktree>` slot, which is not a shell-executable token and never could be.
_recipe_dir="$(mktemp -d "${TMPDIR:-/tmp}/docket-0277-recipe.XXXXXX")"
mkdir -p "$_recipe_dir/scripts"
# A brief with the characters that broke the argv channel: newlines, single quotes, a dollar sign.
printf '%s\n' "brief line one" "line two with 'single quotes' and a \$dollar" > "$_recipe_dir/expected"
# Slice the taught recipe out of the shim: from the mktemp assignment through the launch line, with
# the heredoc BODY (whatever the caller-text slot is worded as) swapped for the known brief. Keyed
# on the heredoc's own delimiter, read off the `<<'…'` line, so no spelling is hardcoded.
awk -v q="'" -v subf="$_recipe_dir/expected" '
  !seen { if (/mktemp/) { seen=1; print } ; next }
  !her && index($0, "<<" q) {
    her=1; delim=$0; sub(/^.*<</, "", delim); gsub(q, "", delim); print
    while ((getline l < subf) > 0) print l
    next
  }
  her { if ($0 == delim) { her=0; print } ; next }
  { print }
  index($0, "--launch") { exit }
' "$G" > "$_recipe_dir/recipe.sh"
# The stub facade: records its argv and nothing else, so the assert below reads exactly what a real
# dispatch would have been handed.
printf '%s\n' '#!/usr/bin/env bash' 'printf "%s\n" "$@" > "${DOCKET_0277_ARGV:?}"' \
  > "$_recipe_dir/scripts/docket.sh"
chmod +x "$_recipe_dir/scripts/docket.sh"
# `bash -e` so a recipe that is not one runnable script stops at the first bad line instead of
# limping on to a launch that only looks reached.
DOCKET_0277_ARGV="$_recipe_dir/argv" DOCKET_SCRIPTS_DIR="$_recipe_dir/scripts" \
  bash -e "$_recipe_dir/recipe.sh" >/dev/null 2>&1 </dev/null
_recipe_rc=$?
assert "0277: the emitted STEP 1 recipe runs as ONE script, start to finish" '[ "$_recipe_rc" = "0" ]'
assert "0277: the recipe reached the dispatch facade" '[ -s "$_recipe_dir/argv" ]'
_bf=""
if [ -s "$_recipe_dir/argv" ]; then
  _prev=""
  while IFS= read -r _a; do
    if [ "$_prev" = "--brief-file" ]; then _bf="$_a"; break; fi
    _prev="$_a"
  done < "$_recipe_dir/argv"
fi
assert "0277: the recipe handed --brief-file a non-empty path" '[ -n "$_bf" ]'
assert "0277: that path is a readable file (the property the shape asserts cannot see)" \
  '[ -f "$_bf" ] && [ -r "$_bf" ]'
assert "0277: that file holds the caller's brief verbatim, quotes and newlines included" \
  'diff -q "$_recipe_dir/expected" "$_bf" >/dev/null 2>&1'
# The brief itself lands in TMPDIR, not in the sandbox — the recipe's own mktemp put it there.
rm -rf "$_recipe_dir"
if [ -n "$_bf" ]; then rm -f "$_bf"; fi
# The executable sentinel above proves the recipe RUNS as one script; this one proves the shim
# still TELLS the model to send it as one call. Shape, not spelling: between the assignment and the
# launch there must be no line at all outside the heredoc — a prose paragraph there is precisely
# the "1a. … 1b. …" two-call split that shipped broken.
_between="$(awk -v q="'" '
  !seen { if (/mktemp/) seen=1; next }
  index($0, "--launch") { exit }
  !her && index($0, "<<" q) { her=1; delim=$0; sub(/^.*<</, "", delim); gsub(q, "", delim); next }
  her { if ($0 == delim) her=0; next }
  { print }
' "$G")"
assert "0277: nothing separates the brief write from the launch — they are ONE taught call" \
  '[ -z "$(grep -vE "^[[:space:]]*$" <<<"$_between")" ]'
# DETECTS THE REMOVAL (LEARNINGS: assert-detects-removal-not-replacement): the argv payload slot
# and its quoting instructions are the defect, so neither may survive anywhere in the shim.
assert "0277: shim no longer renders a trailing single-quoted argv task slot" \
  '! grep -qE -- "-- '\''<[^>]+>'\''" "$G"'
assert "0277: shim no longer teaches the ONE-single-quoted-argument workaround" \
  '! grep -qiE "one single-quoted argument|as ONE .*argument" "$G"'
# The escape-and-reopen glyph run the deleted paragraph taught, assembled rather than written
# literally: spelled inline it needs four levels of quoting and the plan's own spelling came out
# as a DOUBLED backslash, which the generator never emits — a pattern that cannot match is a
# permanently green guard. Built here so it is the exact four characters the old shim contained.
_sq="'"; _gym="$_sq\\$_sq$_sq"
assert "0277: fixture sanity — the gymnastics pattern is the 4-char escape-reopen run" \
  '[ "${#_gym}" = "4" ]'
assert "0277: shim no longer teaches quote-escape gymnastics" '! grep -qF -- "$_gym" "$G"'
# MIRROR correspondence, not a subset: observe takes a KEY, never the brief. Without this, a
# future edit that pastes the payload onto both lines passes every assert above while telling the
# model to re-send a multi-KB brief on every poll.
observe_line="$(grep -F -- "--observe" "$G")"
assert "0277: observe line carries no brief slot" \
  '! grep -qF -- "--brief-file" <<<"$observe_line"'
# Getting the payload wrong is SILENT — the child improvises rather than erroring, which is why
# "it worked last time" is not evidence. Shape-tolerant alternation: the clause may name the
# consequence either way round.
assert "0271: shim names the omission failure as silent" \
  'grep -qiE "fails silently|does not error|looks successful" "$G"'
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

# ---- change 0269: runners.<name>.shim_model / shim_effort govern the shim's OWN pin -------------
mkgitrepo
mkdir -p "$SBX/.claude"
printf 'agents:\n  claude:\n    status: { model: gpt-5.1-codex, effort: high, runner: codex }\nrunners:\n  codex:\n    shim_model: claude-haiku-4-5-20251001\n    shim_effort: medium\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
G="$SBX/.claude/agents/docket-status.md"
assert "0269: configured shim_model lands in the shim frontmatter" \
  '[ "$(fm "$G" model)" = "claude-haiku-4-5-20251001" ]'
assert "0269: configured shim_effort lands in the shim frontmatter" \
  '[ "$(fm "$G" effort)" = "medium" ]'
# The child's values are untouched by the knobs — they still ride the baked dispatch arguments.
assert "0269: the child model still rides --model" 'grep -qF -- "--model gpt-5.1-codex" "$G"'
assert "0269: the child effort still rides --effort" 'grep -qF -- "--effort high" "$G"'
assert "0269: the shim pin is not baked as the child model" '! grep -qF -- "--model claude-haiku-4-5-20251001" "$G"'

# Machine-local layer wins per key, and an unset key still defaults (per-key precedence, not
# per-block): .docket.local.yml supplies only shim_model, so shim_effort must still resolve to low.
mkgitrepo
mkdir -p "$SBX/.claude"
printf 'agents:\n  claude:\n    status: { model: gpt-5.1-codex, effort: high, runner: codex }\nrunners:\n  codex:\n    shim_model: from-committed\n    shim_effort: high\n' > "$SBX/.docket.yml"
printf 'runners:\n  codex:\n    shim_model: from-local\n' > "$SBX/.docket.local.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
G="$SBX/.claude/agents/docket-status.md"
assert "0269: .docket.local.yml wins for shim_model" '[ "$(fm "$G" model)" = "from-local" ]'
assert "0269: precedence is PER KEY — committed shim_effort still applies" \
  '[ "$(fm "$G" effort)" = "high" ]'

# ---- change 0269: shim_model / shim_effort take the bare-scalar rule --------------------------
# Generation-time refusal, matching sync-agents.sh's loud posture for user config (the tolerant
# posture belongs to runner-dispatch.sh, which runs mid-handoff on a live dispatch).
mkgitrepo
mkdir -p "$SBX/.claude"
printf 'agents:\n  claude:\n    status: { model: gpt-5.1-codex, runner: codex }\nrunners:\n  codex:\n    shim_model: "claude-haiku-4-5-20251001"\n' > "$SBX/.docket.yml"
err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" 2>&1 >/dev/null )"; rc=$?
assert "0269: a QUOTED shim_model fails generation nonzero" '[ "$rc" != "0" ]'
assert "0269: the quoted-shim_model diagnostic names the key" 'grep -qF "shim_model" <<<"$err"'
assert "0269: the quoted-shim_model diagnostic names the runner" 'grep -qF "codex" <<<"$err"'
assert "0269: a refused run writes NO wrapper" '[ ! -f "$SBX/.claude/agents/docket-status.md" ]'

mkgitrepo
mkdir -p "$SBX/.claude"
printf 'agents:\n  claude:\n    status: { model: gpt-5.1-codex, runner: codex }\nrunners:\n  codex:\n    shim_effort: very low\n' > "$SBX/.docket.yml"
err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" 2>&1 >/dev/null )"; rc=$?
assert "0269: a SPACED shim_effort fails generation nonzero" '[ "$rc" != "0" ]'
assert "0269: the spaced-shim_effort diagnostic names the key" 'grep -qF "shim_effort" <<<"$err"'

mkgitrepo
mkdir -p "$SBX/.claude"
printf 'agents:\n  claude:\n    status: { model: gpt-5.1-codex, runner: codex }\nrunners:\n  codex:\n    shim_model:\n' > "$SBX/.docket.yml"
err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" 2>&1 >/dev/null )"; rc=$?
# A DIFFERENT diagnostic from the not-a-bare-scalar one, on purpose: without the split, a
# present-but-empty key blames absence for what the user reads as "I set it".
assert "0269: a present-but-empty shim_model fails generation nonzero" '[ "$rc" != "0" ]'
assert "0269: the empty-shim_model diagnostic says present but has no value" \
  'grep -qiF "present but has no value" <<<"$err"'

# --check reports the same failure (parity with the other two gates).
#
# NON-VACUITY (whole-branch review of 0269): the obvious fixture — a bare mkgitrepo carrying only
# the bad value — proves nothing. A fresh repo has no .gitignore, so check_project_level's leg (a)
# already sets rc=1 and `--check` exits nonzero with this gate deleted. Two constructions fix that:
#
#   1. Bring the fixture to a --check-CLEAN baseline first (a real run writes the .gitignore block
#      and the wrappers), and assert that baseline exits 0. Every other --check leg is then known
#      satisfied, so a later nonzero is attributable.
#   2. Choose a bad value whose ABSENCE would emit the SAME BYTES. A present-but-empty shim_model
#      on the runner the agent really delegates to (codex) resolves through runner_key to '' either
#      way, so emit_wrapper falls back to `inherit` and the wrapper is byte-identical to the clean
#      baseline — leg (c)'s drift loop has nothing to report, and only this gate can turn --check
#      red.
#
#      Two fixtures that look equivalent and are not. A QUOTED value on codex CHANGES the emitted
#      pin, so leg (c) goes red on its own and restores the vacuity through the back door. A bad
#      value on a registered-but-UNREFERENCED runner (cursor) was this block's first construction,
#      and it relied on the gate walking every registered runner regardless of use — the unscoped
#      walk that the 0269 whole-branch review found and the scoping block further down now pins
#      as WRONG. Scoped, cursor's block is inert config the gate is right to ignore, so that
#      fixture would have gone green for a brand-new wrong reason.
#
# The rc assert is then paired with a grep for the gate's OWN diagnostic — the one string no other
# --check failure emits — and with a negative assert that no drift was reported, so neither an
# unrelated red nor a deleted gate can be mistaken for a pass. The old "--check wrote no wrapper"
# assert is gone rather than repaired: `--check` writes no wrapper on ANY path, gate or no gate, so
# it was true unconditionally.
mkgitrepo
mkdir -p "$SBX/.claude"
printf 'agents:\n  claude:\n    status: { model: gpt-5.1-codex, runner: codex }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 )
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check >/dev/null 2>&1 ); rc=$?
assert "0269: fixture sanity — --check is CLEAN before the bad shim value is introduced" '[ "$rc" = "0" ]'
printf 'agents:\n  claude:\n    status: { model: gpt-5.1-codex, runner: codex }\nrunners:\n  codex:\n    shim_model:\n' > "$SBX/.docket.yml"
err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check 2>&1 >/dev/null )"; rc=$?
assert "0269: --check also refuses a bad shim value" '[ "$rc" != "0" ]'
assert "0269: --check fails on the SHIM gate specifically, not an unrelated leg" \
  'grep -qF "check: runner shim-pin configuration is invalid" <<<"$err"'
assert "0269: the bad value moved no emitted byte — leg (c) reported no drift of its own" \
  '! grep -qF "drift in" <<<"$err"'

# A VALID bare scalar is accepted — the positive control, without which every assert above is
# consistent with a gate that refuses everything.
mkgitrepo
mkdir -p "$SBX/.claude"
printf 'agents:\n  claude:\n    status: { model: gpt-5.1-codex, runner: codex }\nrunners:\n  codex:\n    shim_model: claude-haiku-4-5-20251001\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 ); rc=$?
assert "0269: a bare-scalar shim_model generates cleanly" '[ "$rc" = "0" ]'
assert "0269: the accepted run DID write the wrapper" '[ -f "$SBX/.claude/agents/docket-status.md" ]'

# ---- change 0269 (whole-branch review): the shim gate is SCOPED to the runners this run CONSUMES
# The finding: the gate walked REGISTERED_RUNNERS x every layer unconditionally, so one typo'd
# shim_model in ~/.config/docket/config.yml — for a runner no agent references — hard-failed
# `sync-agents.sh` and `--check` in EVERY repo on the machine, including repos with no `runners:`
# usage at all. It now walks the runners for_each_candidate_triple actually resolves, which is the
# same population validate_runner_config enumerates (LEARNINGS:
# guard-keyed-on-presence-not-provenance — key on what got USED, not on what a layer merely HOLDS).
#
# Every assert below comes in a MATCHED PAIR, because the fix has two ways to be wrong: too wide
# (the original finding) and too narrow (a gate that quietly stopped firing). Each silent case is
# paired with a near-identical config that DOES refuse, so a fail-open regression cannot hide in
# the silence.

# (1) Registered but referenced by NO agent: inert config, must not refuse the run.
mkgitrepo
mkdir -p "$SBX/.claude"
printf 'agents:\n  claude:\n    status: { model: gpt-5.1-codex, runner: codex }\nrunners:\n  cursor:\n    shim_model: "quoted"\n' > "$SBX/.docket.yml"
err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" 2>&1 >/dev/null )"; rc=$?
assert "0269: a bad shim value on an UNREFERENCED registered runner does not refuse the run" \
  '[ "$rc" = "0" ]'
assert "0269: ... and the gate says nothing about that runner" \
  '! grep -qF "runners.cursor.shim_model" <<<"$err"'
# NON-VACUITY: without this, a generation that died before the gate would pass the two asserts above.
assert "0269: fixture sanity — the run really did generate the codex shim" \
  'grep -qF -- "--runner codex" "$SBX/.claude/agents/docket-status.md"'
# The paired half: the SAME bad value, same file, moved onto the runner the agent DOES delegate to.
printf 'agents:\n  claude:\n    status: { model: gpt-5.1-codex, runner: codex }\nrunners:\n  codex:\n    shim_model: "quoted"\n' > "$SBX/.docket.yml"
err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" 2>&1 >/dev/null )"; rc=$?
assert "0269: the same bad value on the REFERENCED runner still refuses the run" '[ "$rc" != "0" ]'
assert "0269: ... naming that runner in the diagnostic" 'grep -qF "runners.codex.shim_model" <<<"$err"'

# (2) THE MACHINE-WIDE CASE from the finding: the typo lives in the user's global config and the
#     repo delegates nothing at all. This pair is also what separates a whole-predicate copy from
#     one that only copied validate_runner_config's PROJECT-level leg (LEARNINGS:
#     duplicated-gate-copies-the-whole-predicate): nothing here opts the repo in, so only the
#     USER-LEVEL leg of for_each_candidate_triple can ever see the delegating triple below.
mkgitrepo
mkdir -p "$SBX/.claude" "$SBX/.config/docket"
printf 'runners:\n  cursor:\n    shim_model: "quoted"\n' > "$SBX/.config/docket/config.yml"
err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" 2>&1 >/dev/null )"; rc=$?
assert "0269: a global shim typo does not refuse a repo that delegates nothing" '[ "$rc" = "0" ]'
assert "0269: fixture sanity — that run generated wrappers" \
  '[ -f "$SBX/.claude/agents/docket-status.md" ]'
assert "0269: fixture sanity — and not one of them delegates" \
  '! grep -qF "docket.sh runner-dispatch" "$SBX/.claude/agents/docket-status.md"'
# The paired half: the same global file now delegates ONE agent to that same runner.
printf 'agents:\n  claude:\n    status: { model: gpt-5.1-codex, runner: cursor }\nrunners:\n  cursor:\n    shim_model: "quoted"\n' > "$SBX/.config/docket/config.yml"
err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" 2>&1 >/dev/null )"; rc=$?
assert "0269: the USER-LEVEL leg alone still catches the same bad value" '[ "$rc" != "0" ]'
assert "0269: ... naming the runner that user-level entry delegates to" \
  'grep -qF "runners.cursor.shim_model" <<<"$err"'

# STRUCTURAL TRIPWIRES, not behavioral asserts. Both gates must keep judging ONE shared walk; a
# future edit that re-forks either into its own copy of the population is exactly the
# duplicated-gate-copies-the-whole-predicate failure, and it would stay green under every fixture
# above until the day the two copies disagree. The gate's body is sliced out and searched for the
# unscoped population it used to walk — the fixtures above cannot see the difference between a gate
# that never regressed and one that regressed and was re-scoped by a second, divergent predicate.
shim_body="$(awk '/^validate_runner_shim_values\(\)/{inb=1} inb{print} inb && /^\}/{exit}' "$SYNC")"
assert "0269: fixture sanity — the shim gate's body was really extracted" \
  '[ "$(grep -c . <<<"$shim_body")" -gt 10 ]'
assert "0269: the shim gate does not walk the whole runner registry" \
  '! grep -qF "REGISTERED_RUNNERS" <<<"$shim_body"'
assert "0269: the shim gate walks the resolved candidate set instead" \
  'grep -qF "CANDIDATE_RUNNERS" <<<"$shim_body"'
assert "0269: validate_runner_config judges the shared candidate-triple walk" \
  'grep -qF "for_each_candidate_triple check_triple_runner_config" "$SYNC"'

# ---- change 0269 (whole-branch review, finding 5): the bare-scalar rule is an ALLOWLIST ---------
# The predicate used to name three REJECTED shapes — empty, whitespace-bearing, leading quote — and
# passed everything else through into the emitted `model:` pin VERBATIM, while its own diagnostic
# promised a bare scalar. Every value below is a YAML shape that is not a bare scalar and that none
# of those three legs can see: two block-scalar indicators, two flow collections, an alias, an
# anchor, and a value quoted only on the RIGHT. Inverting the predicate into an allowlist is what
# makes the rule the diagnostic's own remedy text rather than a list of the shapes someone thought
# of (LEARNINGS: byte-pattern-guard-matches-a-spelling — a blocklist covers spellings, never the
# property). Each must refuse the run, name the offender, and leave no wrapper behind.
for bad in '>' '|' '[a]' '{m:x}' '*ref' '&anchor' 'haiku"'; do
  mkgitrepo
  mkdir -p "$SBX/.claude"
  printf 'agents:\n  claude:\n    status: { model: gpt-5.1-codex, runner: codex }\nrunners:\n  codex:\n    shim_model: %s\n' "$bad" > "$SBX/.docket.yml"
  err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" 2>&1 >/dev/null )"; rc=$?
  assert "0269/F5: a non-bare-scalar shim_model '$bad' refuses the run" '[ "$rc" != "0" ]'
  assert "0269/F5: ... naming runners.codex.shim_model ('$bad')" \
    'grep -qF "runners.codex.shim_model" <<<"$err"'
  assert "0269/F5: ... and writing no wrapper ('$bad')" \
    '[ ! -f "$SBX/.claude/agents/docket-status.md" ]'
  rm -rf "$SBX"
done

# THE PAIRED HALF, and it is not decorative: inverting a blocklist into an allowlist trades a
# fail-open for a fail-CLOSED the moment the class is drawn too narrowly. The values below are the
# ones real configs carry — the two documented defaults (.docket.example.yml ships `inherit` and
# `low`), a full Claude model ID, and the provider-prefixed forms change 0173 made first-class for
# the child pin, which carry `/` and `:`. Each must generate AND land verbatim in the frontmatter;
# "the run succeeded" alone would not catch a value that was silently truncated on the way through.
for good in inherit auto low claude-haiku-4-5-20251001 anthropic/claude-opus-5 openrouter:vendor/model; do
  mkgitrepo
  mkdir -p "$SBX/.claude"
  printf 'agents:\n  claude:\n    status: { model: gpt-5.1-codex, runner: codex }\nrunners:\n  codex:\n    shim_model: %s\n' "$good" > "$SBX/.docket.yml"
  ( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 ); rc=$?
  assert "0269/F5: a legitimate shim_model '$good' still generates" '[ "$rc" = "0" ]'
  assert "0269/F5: ... and lands verbatim in the shim frontmatter ('$good')" \
    '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "'"$good"'" ]'
  rm -rf "$SBX"
done

# ---- change 0269 (whole-branch review, finding 6): a FLOW-STYLE runner block is REFUSED ---------
# section_body recognizes only a bare `<runner>:` header, so `codex: { shim_model: haiku }` yielded
# no block, no key, no gate hit and a silent fallback to inherit/low. That style is not exotic: the
# `agents:` entries configured directly above it in the same file are written exactly that way.
# Configured-but-never-applied with zero feedback is the defect class this change exists to remove,
# so the gate refuses it rather than reading it — parsing flow style would be a second, divergent
# reader of the same block (LEARNINGS: duplicated-gate-copies-the-whole-predicate).
mkgitrepo
mkdir -p "$SBX/.claude"
printf 'agents:\n  claude:\n    status: { model: gpt-5.1-codex, runner: codex }\nrunners:\n  codex: { shim_model: claude-haiku-4-5-20251001 }\n' > "$SBX/.docket.yml"
err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" 2>&1 >/dev/null )"; rc=$?
assert "0269/F6: a flow-style runners.codex block refuses the run" '[ "$rc" != "0" ]'
assert "0269/F6: the diagnostic names the runner" 'grep -qF "runners.codex" <<<"$err"'
assert "0269/F6: the diagnostic says a block mapping is required" 'grep -qiF "block mapping" <<<"$err"'
assert "0269/F6: a refused run writes NO wrapper" '[ ! -f "$SBX/.claude/agents/docket-status.md" ]'
# NON-VACUITY: the SAME pin, same fixture, written as a block mapping must generate and land — so
# the refusal above is attributable to the style and not to anything else in the fixture.
printf 'agents:\n  claude:\n    status: { model: gpt-5.1-codex, runner: codex }\nrunners:\n  codex:\n    shim_model: claude-haiku-4-5-20251001\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 ); rc=$?
assert "0269/F6: the same pin in BLOCK form still generates" '[ "$rc" = "0" ]'
assert "0269/F6: ... and lands in the shim frontmatter" \
  '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "claude-haiku-4-5-20251001" ]'
rm -rf "$SBX"

# SCOPED exactly like every other leg of this gate — the flow-style refusal stays inside the
# candidate-runner scoping the fix above established, or one flow-style block in a machine's global
# config would refuse `sync-agents.sh` in every repo that never delegates to that runner.
mkgitrepo
mkdir -p "$SBX/.claude"
printf 'agents:\n  claude:\n    status: { model: gpt-5.1-codex, runner: codex }\nrunners:\n  cursor: { shim_model: haiku }\n' > "$SBX/.docket.yml"
err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" 2>&1 >/dev/null )"; rc=$?
assert "0269/F6: a flow-style block on an UNREFERENCED runner does not refuse the run" '[ "$rc" = "0" ]'
assert "0269/F6: ... and the gate says nothing about that runner" \
  '! grep -qF "runners.cursor" <<<"$err"'
assert "0269/F6: fixture sanity — the run really did generate the codex shim" \
  'grep -qF -- "--runner codex" "$SBX/.claude/agents/docket-status.md"'
rm -rf "$SBX"
exit $fail
