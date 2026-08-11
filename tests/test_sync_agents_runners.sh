#!/usr/bin/env bash
# tests/test_sync_agents_runners.sh — runner shims, atomic generation, pin injection (shard of test_sync_agents.sh,
# change 0227). Run: bash tests/test_sync_agents_runners.sh
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
  '[ "$(find "$SBX/.claude/agents" -name "docket-*.md" | wc -l | tr -d " ")" = "16" ]'
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
# check_project_level turns the advisory assert below NOT OK (16 advisory lines are emitted).
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
  '[ "$(find "$D2ROOT/.claude/agents" -name "docket-*.md" | wc -l | tr -d " ")" = "16" ]'
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

# ---- change 0168: emit() inserts exactly one model:/effort: line -------------
# emit() is strip-then-insert now: it drops any model:/effort: the SOURCE still carries and injects
# the resolved pair before the closing fence. The failure mode that rewrite introduces is a
# DUPLICATED key (insert without strip), which no earlier assert could see because substitution
# cannot duplicate. Counted inside the FIRST frontmatter block — a whole-file count would also see
# body prose (AGENTS.md: anchor a frontmatter read to the first ---…--- block) — plus a whole-file
# count, so an insertion that lands outside the block is caught too.
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
# Claude model ID to a Codex child. Only a USER-configured value may cross that boundary. What the
# wrapper's own frontmatter carries is a separate question, settled by the change-0269 note below.
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
# change 0269 replaces 0168's two "the shim frontmatter carries the resolved native pin
# (bookkeeping)" asserts — the effort half and the model half of one claim whose premise was false.
# The shim runs in the PARENT harness, so its frontmatter is the parent-side relay's own pin: the
# runners.<name> knobs, here unset and therefore at their defaults. The resolved native values
# belong to the CHILD and reach it only through the baked flags asserted above.
assert "0269: an unconfigured runner shim's frontmatter effort is the shim default, not the resolved native effort" \
  '[ "$(fm "$S" effort)" = "low" ] && [ "$(fm "$S" effort)" != "$(hd_field "$HD" claude status effort)" ]'
assert "0269: an unconfigured runner shim's frontmatter model is the shim default, not the child's resolved model" \
  '[ "$(fm "$S" model)" = "inherit" ] && [ "$(fm "$S" model)" != "user-picked-id" ]'

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
  '[ "$(sed -n "s/^model:[[:space:]]*//p" "$SBX/.cursor/agents/docket-status.md" | sed -n 1p)" = "claude-opus-4-8" ]'
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

exit $fail
