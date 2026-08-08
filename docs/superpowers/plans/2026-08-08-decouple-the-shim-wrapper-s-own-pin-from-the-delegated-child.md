<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0269 — Decouple the shim wrapper's own pin from the delegated child's](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0269-decouple-the-shim-wrapper-s-own-pin-from-the-delegated-child.md)**
<!-- docket:backlink:end -->

# Decouple the Shim Wrapper's Own Pin from the Delegated Child's — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make a runner-delegation shim's frontmatter `model:`/`effort:` describe the **shim agent itself** (which runs in the parent harness, Claude Code) instead of the delegated child, so every delegated wrapper stops being pinned to a model Claude Code cannot resolve.

**Architecture:** Two new per-runner config keys — `runners.<name>.shim_model` (default `inherit`) and `runners.<name>.shim_effort` (default `low`) — are resolved by a small layered reader added to `sync-agents.sh` and passed to `emit_shim` **in place of** the child's model and effort. `emit_shim`'s arity is unchanged; its `$2`/`$3` are repurposed. The child's values continue to ride the baked `--model` / `--effort` dispatch arguments untouched. A generation-time gate rejects a non-bare-scalar value for either key before any wrapper is written.

**Tech Stack:** Bash (floor 3.2), awk/sed/grep (BSD-compatible), the docket test harness (`tests/lib/sync_agents_common.sh`, `scripts/run-tests.sh`).

## Global Constraints

- **Bash floor is 3.2.** No associative arrays, no `declare -n` namerefs, no `${var^^}` in new code paths that must run on the floor. `sync-agents.sh` runs `set -euo pipefail` — a failing command as the last element of an `&&` list aborts the run.
- **Never `producer | early-exiting-consumer`** (`grep -q`, `head`, `head -n1`) under `set -o pipefail`. Capture into a variable first, then `grep <<<"$var"`.
- **awk indent classes are `[^[:space:]]`, never `[^ ]`.**
- **`grep` for a pattern leading with `--`** must declare it: `grep -qF -- "<pat>"`.
- **A cross-reference in maintained source anchors on a symbol name or a verbatim-quoted clause — never a line number.**
- **A guard is code: mutation-test it.** Strip the thing it guards, watch it redden, restore. Confirm each mutation actually landed with a `grep -c` count before and after.
- **Run the whole suite at the build gate**, via the resolved `finalize.test_command` — `scripts/run-tests.sh` — never only the tests this plan names.
- `sync-agents.sh` lives at the **repo root**, not in `scripts/`.

## Not in this branch

**The superseding ADR is not written here.** `docket-implement-next` dispatches the `docket-adr` subagent at Step 6, and that agent writes the ADR on the `docket` metadata branch and appends its number to the change's `adrs:` field. Do **not** create a file under `docs/adrs/` in this worktree, and do not edit `docs/adrs/0038-runner-shim-wrapper-single-dispatch-chokepoint.md` — an Accepted ADR's Decision is never edited, and its `status:` flip is the ADR agent's write, on the metadata branch.

## File Structure

| File | Responsibility | Task |
|---|---|---|
| `sync-agents.sh` (repo root) | Modify: add `runner_key` layered reader; resolve the two knobs in `emit_wrapper`; repurpose `emit_shim`'s `$2`/`$3`; restate `emit_wrapper`'s calling-contract header; add `validate_runner_shim_values` and wire it into both gate sites | 1, 2 |
| `tests/test_sync_agents_runners.sh` | Modify: invert the assert that encodes the false premise; add the default / configured / baked-flag / regression asserts; add the validation asserts | 1, 2 |
| `.docket.example.yml` | Modify: document both keys under the `runners.codex:` block with a scope tag | 3 |
| `tests/test_docket_example_yml.sh` | Modify: two new `classify_key` arms, `expected_key_count` 45 → 47, two completeness asserts | 3 |
| `README.md` | Modify: the *Runner delegation* rules list gains the shim-pin rule | 3 |
| `scripts/runners/codex.md`, `scripts/runners/cursor.md`, `scripts/runners/opencode.md` | Modify: each contract's shim/frontmatter description | 3 |
| `skills/docket-convention/references/agent-layer.md` | Modify: the shim + "Model and effort on a delegated agent" prose | 3 |

---

### Task 1: Resolve and emit the shim's own pin

The behavior fix. After this task a delegated wrapper's frontmatter carries `model: inherit` / `effort: low` by default, the child's IDs still ride `--model` / `--effort`, and the assert that encoded the false premise is inverted into the regression guard.

**Files:**
- Modify: `sync-agents.sh` — add `runner_key` immediately below `section_body`; edit `emit_wrapper`'s header comment and body; edit `emit_shim`'s header comment and parameter comment
- Test: `tests/test_sync_agents_runners.sh`

**Interfaces:**
- Consumes: `section_body <key>` (reads stdin, prints the dedented body under `<key>:`), `LOCAL_CFG`, `DOCKET_YML`, `GLOBAL_CFG` — all already defined in `sync-agents.sh`.
- Produces: `runner_key <runner> <key>` — prints the resolved bare scalar for `runners.<runner>.<key>` from the highest-precedence layer that carries the key, or the empty string when no layer does. Used again by Task 2.

- [ ] **Step 1: Write the failing tests**

Open `tests/test_sync_agents_runners.sh`. Find this existing line (it is the assert that encodes the defect):

```bash
assert "0079: shim keeps frontmatter model (bookkeeping)" '[ "$(fm "$G" model)" = "gpt-5.1-codex" ]'
```

**Replace that single line** with the block below. Do not keep the original alongside the new asserts — the old assert and the regression assert are the same claim with opposite signs, so keeping both makes the suite unsatisfiable.

```bash
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
```

Now add the configured-knob case. Append this block immediately after the `0079: --check flags a de-shimmed wrapper as drift` assert (the end of the first shim fixture group), so it gets its own fresh sandbox:

```bash
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `bash tests/test_sync_agents_runners.sh 2>&1 | grep -c "^NOT OK"`

Expected: a non-zero count. Confirm the *reasons* by reading the failures — `bash tests/test_sync_agents_runners.sh 2>&1 | grep "^NOT OK"` — the frontmatter model should currently be `gpt-5.1-codex` for every new assert, which is exactly the defect.

- [ ] **Step 3: Add the layered reader**

In `sync-agents.sh`, immediately **after** the closing `}` of `section_body` and **before** the `if [ "${BASH_VERSINFO[0]}" -ge 4 ]; then` block, insert:

```bash
# --- runners.<name>.<key>: per-key across layers (change 0269) ----------------
# The shim wrapper runs in the PARENT harness and does one thing: a foreground `docket.sh
# runner-dispatch` call plus a stdout relay. Its frontmatter pin therefore governs the parent-side
# agent and must be resolvable by the parent — the child's pin is the baked `--model` argument and
# only that. These two knobs are what the frontmatter carries.
#
# Layering is repo-local > repo-committed > global, PER KEY: the first layer that carries the key
# wins, and a layer supplying only one of the two leaves the other to resolve further down (or to
# its default). Same rule runner-dispatch.sh already applies to the rest of the block.
#
# Composed from section_body rather than re-implementing the dedenting walk. This makes
# sync-agents.sh the SECOND independent consumer of the `runners:` block — runner-dispatch.sh's
# `runner_block`/`yaml_section` pair is the first — which is a knowing deferral, not an oversight:
# unifying the two parsers is change #0256's scope, and it should absorb both readers when it lands.
# The value class here is the BLOCK-mapping one (rest of the line, comment stripped, trimmed), not
# the `{…}` flow-map class the agents entries use; a block mapping has one key per line.
runner_key() {  # $1=runner  $2=key  -> the value from the highest-precedence layer carrying it, else ''
  local f blk line v
  for f in "$LOCAL_CFG" "$DOCKET_YML" "$GLOBAL_CFG"; do
    [ -f "$f" ] || continue
    blk="$(section_body runners < "$f" | section_body "$1")"
    [ -n "$blk" ] || continue
    line="$(awk -v k="$2" '
      { nc=$0; sub(/#.*/, "", nc) }
      nc ~ ("^[[:space:]]*" k "[[:space:]]*:") { print nc; exit }
    ' <<<"$blk")"
    [ -n "$line" ] || continue
    v="$(sed -E -e 's/^[[:space:]]*[A-Za-z0-9._-]+[[:space:]]*:[[:space:]]*//' -e 's/[[:space:]]+$//' <<<"$line")"
    printf '%s' "$v"
    return 0
  done
  return 0
}
```

- [ ] **Step 4: Resolve the knobs and pass them to `emit_shim`**

In `sync-agents.sh`, inside `emit_wrapper`, find the final line of the function body:

```bash
  emit_shim "$1" "$2" "$3" "$runner" "$6" "$flag_model" "$flag_effort"
```

Replace it with:

```bash
  # change 0269: $2/$3 are the CHILD's resolved pin and reach the shim only as the baked flags
  # above. The shim's own frontmatter gets the parent-side knobs, defaulting to `inherit`/`low`.
  # `inherit` is deliberate: emit() passes it through VERBATIM on the claude harness (Claude Code
  # documents it as "run on the parent conversation's model", a real value distinct from omitting
  # the key), so every currently-broken wrapper is repaired by regeneration alone with no config
  # edit. The knob is a cost optimization on top, never a prerequisite.
  local shim_model shim_effort
  shim_model="$(runner_key "$runner" shim_model)"
  [ -n "$shim_model" ] || shim_model="inherit"
  shim_effort="$(runner_key "$runner" shim_effort)"
  [ -n "$shim_effort" ] || shim_effort="low"
  emit_shim "$1" "$shim_model" "$shim_effort" "$runner" "$6" "$flag_model" "$flag_effort"
```

- [ ] **Step 5: Restate `emit_wrapper`'s calling-contract header**

The header above `emit_wrapper` is a second documented statement of the false premise. Find this paragraph:

```
# CALLING CONTRACT (change 0220): $2 MUST be the RES_MODEL that resolve_agent_layers just resolved
# for this exact (harness, agent) pair, and $3 the matching RES_EFFORT. $2 is used TWICE and for
# two different things — as emit_shim's frontmatter pin, and (provenance-filtered through
# RES_MODEL_FROM_USER) as the baked --model flag — so a caller that passes a post-processed model
# would split the wrapper against itself. This is why the provenance filter here is a second
# spelling of user_flag_model's rather than a call to it: rerouting only the flag would leave the
# frontmatter pin on $2 and emit a wrapper whose two halves disagree, silently. The assertion at the
# top of the body is what makes the contract enforced rather than merely conventional.
```

Replace it with:

```
# CALLING CONTRACT (change 0220, amended by change 0269): $2 MUST be the RES_MODEL that
# resolve_agent_layers just resolved for this exact (harness, agent) pair, and $3 the matching
# RES_EFFORT. On the native path they are the wrapper's pin directly; on the delegated path they
# reach the child ONLY as the baked --model/--effort flags, provenance-filtered through
# RES_MODEL_FROM_USER. Either way a caller that passes a post-processed model sends the wrong
# identity to the harness that runs the work, which is what the assertion at the top of the body
# exists to prevent.
#
# Change 0269 removed $2's SECOND use: the delegated shim's frontmatter pin now comes from
# `runners.<name>.shim_model`, not from $2. Both halves of a delegated wrapper are still resolved
# here, but they now answer two different questions — "what can the PARENT harness run this relay
# on" (the frontmatter, via runner_key) and "what should the CHILD run the work on" (the baked
# flag, via $2). That is why the provenance filter stays a second spelling of user_flag_model's
# rather than a call to it: the two values are no longer the same value wearing two hats, and the
# filter belongs to the flag alone.
```

- [ ] **Step 6: Restate `emit_shim`'s header and parameter comment**

Find `emit_shim`'s header and signature comment:

```bash
# The shim: native frontmatter carrying the FULLY RESOLVED pin (bookkeeping for the claude parent —
# the effective pin for the delegated work is the baked --model argument), body = one foreground
# facade call + relay + verify rules. The baked flags come from $6/$7, which carry USER-configured
# values only (change 0168); an empty one bakes NO flag, so the child harness applies its own
# default rather than inheriting a default that was only ever meant for this harness.
emit_shim(){  # $1=src $2=model $3=effort $4=runner $5=agent-name $6=flag-model $7=flag-effort  (stdout)
```

Replace both with:

```bash
# The shim: native frontmatter carrying the SHIM'S OWN pin (change 0269 — this agent runs in the
# claude parent and does one foreground facade call plus a stdout relay, so its pin must name
# something the PARENT can resolve; the pin for the delegated work is the baked --model argument),
# body = one foreground facade call + relay + verify rules. The baked flags come from $6/$7, which
# carry USER-configured values only (change 0168); an empty one bakes NO flag, so the child harness
# applies its own default rather than inheriting a default that was only ever meant for this
# harness. This function stays a pure emitter — its caller resolves both pins and hands them down.
emit_shim(){  # $1=src $2=shim-model $3=shim-effort $4=runner $5=agent-name $6=flag-model $7=flag-effort  (stdout)
```

The body of `emit_shim` is **unchanged** — `emit "$1" "$2" "$3" | awk …` now receives the shim's own values because the caller supplies them.

- [ ] **Step 7: Run the tests to verify they pass**

Run: `bash tests/test_sync_agents_runners.sh 2>&1 | grep "^NOT OK"`
Expected: no output (every assert green).

Then run the neighboring shards, which also generate wrappers:

Run: `bash tests/test_sync_agents.sh 2>&1 | grep "^NOT OK"; bash tests/test_sync_agents_validator.sh 2>&1 | grep "^NOT OK"`
Expected: no output from either. If a `model:`/`effort:` assert in another shard reddens against a *delegated* fixture, it is asserting the old premise — update it the same way as Step 1's, and say so in the commit message.

- [ ] **Step 8: Mutation-test the regression assert**

Prove the new guard bites. Temporarily reintroduce the defect and confirm red:

```bash
grep -c 'emit_shim "$1" "$shim_model" "$shim_effort"' sync-agents.sh   # expect 1
sed -i.bak 's/emit_shim "\$1" "\$shim_model" "\$shim_effort"/emit_shim "$1" "$2" "$3"/' sync-agents.sh
grep -c 'emit_shim "$1" "$2" "$3"' sync-agents.sh                       # expect 1 — the mutation LANDED
bash tests/test_sync_agents_runners.sh 2>&1 | grep -c "^NOT OK"         # expect > 0
mv -f sync-agents.sh.bak sync-agents.sh
grep -c 'emit_shim "$1" "$shim_model" "$shim_effort"' sync-agents.sh    # expect 1 — restored
bash tests/test_sync_agents_runners.sh 2>&1 | grep -c "^NOT OK"         # expect 0
```

A green run against the landed mutation is a defect in the guard, not a pass — fix the guard before continuing.

- [ ] **Step 9: Commit**

```bash
git add sync-agents.sh tests/test_sync_agents_runners.sh
git commit -m "fix(0269): a shim's frontmatter pin is the parent-side agent's, not the child's"
```

---

### Task 2: Reject a non-bare-scalar `shim_model` / `shim_effort` at generation time

`runner_key`'s value class consumes the rest of the line. A quoted value would ride into the emitted frontmatter with its quotes, and a present-but-empty key reads as "unset" — both silently produce a wrong pin in a file that persists. `sync-agents.sh` is loud at generation time by design (the asymmetry with `runner-dispatch.sh`'s tolerant mid-dispatch posture is deliberate), so these are refusals before the first wrapper is written.

**Files:**
- Modify: `sync-agents.sh` — add `validate_runner_shim_values` next to `validate_user_agent_values`; call it at both gate sites
- Test: `tests/test_sync_agents_runners.sh`

**Interfaces:**
- Consumes: `section_body`, `REGISTERED_RUNNERS`, `log`, `LOCAL_CFG`/`DOCKET_YML`/`GLOBAL_CFG`. It deliberately does **not** call `runner_key`: that function returns the *resolved* value from the winning layer, while this gate must report **every** offender in **every** layer, including ones precedence would mask. Reusing it would silently skip a bad value in a lower layer that a good higher-layer value happens to shadow — and that bad value becomes live the moment the higher layer is edited.
- Produces: `validate_runner_shim_values` — returns 0 when every layer's `shim_model`/`shim_effort` is a bare scalar, 1 after logging every offender.

- [ ] **Step 1: Write the failing tests**

Append to `tests/test_sync_agents_runners.sh`, after Task 1's blocks:

```bash
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

# --check reports the same failure without writing anything (parity with the other two gates).
mkgitrepo
mkdir -p "$SBX/.claude"
printf 'agents:\n  claude:\n    status: { model: gpt-5.1-codex, runner: codex }\nrunners:\n  codex:\n    shim_model: "quoted"\n' > "$SBX/.docket.yml"
err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check 2>&1 >/dev/null )"; rc=$?
assert "0269: --check also refuses a bad shim value" '[ "$rc" != "0" ]'
assert "0269: --check wrote no wrapper" '[ ! -f "$SBX/.claude/agents/docket-status.md" ]'

# A VALID bare scalar is accepted — the positive control, without which every assert above is
# consistent with a gate that refuses everything.
mkgitrepo
mkdir -p "$SBX/.claude"
printf 'agents:\n  claude:\n    status: { model: gpt-5.1-codex, runner: codex }\nrunners:\n  codex:\n    shim_model: claude-haiku-4-5-20251001\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 ); rc=$?
assert "0269: a bare-scalar shim_model generates cleanly" '[ "$rc" = "0" ]'
assert "0269: the accepted run DID write the wrapper" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `bash tests/test_sync_agents_runners.sh 2>&1 | grep "^NOT OK"`
Expected: the new nonzero-exit and diagnostic asserts fail (generation currently succeeds and emits a quoted pin). The positive-control asserts at the end should already pass.

- [ ] **Step 3: Add the validator**

In `sync-agents.sh`, insert immediately **after** the closing `}` of `validate_user_agent_values` and **before** the `report_runner_error_once` header comment:

```bash
# --- runners.<name> shim-pin value validation (change 0269) -------------------
# runner_key's value class consumes the rest of the line, so a quoted value would ride into the
# emitted frontmatter WITH its quotes and a present-but-empty key would read as "unset" — either
# way a wrong pin persisted in a generated file. Same posture and same two-leg diagnostic split as
# validate_user_agent_values: collect every offender across every layer, report them all, and fail
# BEFORE any wrapper is written.
#
# Only REGISTERED runners are walked. An unregistered `runners:` sub-block is inert config that
# generates nothing, and hard-failing a repo over it would punish a cosmetic typo in a block no
# pass reads — the same reasoning that exempts the pre-0046 flat agents shape.
validate_runner_shim_values() {
  local rc=0 f r k blk line raw trimmed
  for f in "$LOCAL_CFG" "$DOCKET_YML" "$GLOBAL_CFG"; do
    [ -f "$f" ] || continue
    for r in $REGISTERED_RUNNERS; do
      blk="$(section_body runners < "$f" | section_body "$r")"
      [ -n "$blk" ] || continue
      for k in shim_model shim_effort; do
        line="$(awk -v key="$k" '
          { nc=$0; sub(/#.*/, "", nc) }
          nc ~ ("^[[:space:]]*" key "[[:space:]]*:") { print nc; exit }
        ' <<<"$blk")"
        # Key absent from this block is normal — both knobs are optional in every layer.
        [ -n "$line" ] || continue
        raw="$(sed -E -e 's/^[[:space:]]*[A-Za-z0-9._-]+[[:space:]]*:[[:space:]]*//' -e 's/[[:space:]]+$//' <<<"$line")"
        if [ -z "$raw" ]; then
          log "runners.$r.$k is present but has no value ($f)"
          rc=1
          continue
        fi
        trimmed="$raw"
        case "$trimmed" in *[[:space:]]*)
          log "runners.$r.$k value '$raw' is not a bare scalar — write shim_model/shim_effort values unquoted and space-free ($f)"
          rc=1
          continue
          ;;
        esac
        case "$trimmed" in '"'*|"'"*)
          # The quote leg catches what the whitespace leg structurally CANNOT see: a quoted but
          # space-free value has no embedded space, so the quotes would ride into the emitted pin
          # verbatim while the diagnostic's own remedy text tells the user to write them unquoted.
          log "runners.$r.$k value '$raw' is not a bare scalar — write shim_model/shim_effort values unquoted and space-free ($f)"
          rc=1
          ;;
        esac
      done
    done
  done
  return $rc
}
```

- [ ] **Step 4: Wire it into both gate sites**

In `sync-agents.sh`, in the `--check` branch, find:

```bash
    if ! validate_runner_config; then
      log "check: runner configuration is invalid — a real run would refuse to write wrappers."
      exit 1
    fi
```

Insert **immediately above** it:

```bash
    if ! validate_runner_shim_values; then
      log "check: runner shim-pin configuration is invalid — a real run would refuse to write wrappers."
      exit 1
    fi
```

Then, on the real-run path, find:

```bash
  if ! validate_runner_config; then
    log "ERROR runner configuration is invalid — no wrappers were written."
    exit 1
  fi
```

Insert **immediately above** it:

```bash
  # Same placement bound as Gate 3 directly below: ABOVE user_level_pass, because the first
  # `mkdir -p` or emit_wrapper redirection past this point is already a partial generation.
  if ! validate_runner_shim_values; then
    log "ERROR runner shim-pin configuration is invalid — no wrappers were written."
    exit 1
  fi
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `bash tests/test_sync_agents_runners.sh 2>&1 | grep "^NOT OK"`
Expected: no output.

- [ ] **Step 6: Mutation-test the gate**

```bash
grep -c 'validate_runner_shim_values' sync-agents.sh    # expect 3 (definition + two call sites)
sed -i.bak 's/^  if ! validate_runner_shim_values; then/  if false; then/' sync-agents.sh
grep -c 'if false; then' sync-agents.sh                  # expect 1 — the mutation LANDED
bash tests/test_sync_agents_runners.sh 2>&1 | grep -c "^NOT OK"   # expect > 0
mv -f sync-agents.sh.bak sync-agents.sh
grep -c 'validate_runner_shim_values' sync-agents.sh     # expect 3 — restored
bash tests/test_sync_agents_runners.sh 2>&1 | grep -c "^NOT OK"   # expect 0
```

- [ ] **Step 7: Commit**

```bash
git add sync-agents.sh tests/test_sync_agents_runners.sh
git commit -m "fix(0269): refuse a non-bare-scalar shim_model/shim_effort before any wrapper is written"
```

---

### Task 3: Ship the knob end-to-end — config example, guards, README, contracts, reference

**Correction to the spec's documentation list.** It names "`.docket.example.yml` and
`config.yml.example` — the two new keys". There is no `config.yml.example` in this repo. The global
config template `scripts/ensure-global-config.sh` seeds is a stub whose only guidance is a pointer:
"See .docket.example.yml in the docket repo for every key, default, and allowed layer." So
`.docket.example.yml` is the single documentation surface for both keys, and
`ensure-global-config.sh` needs no edit.

A new config knob is not done when it merely works. This task adds the two keys to `.docket.example.yml` with a scope tag, satisfies that file's guard suite (which enforces a classification arm and an exact key count for every documented key), and corrects every prose statement of the false premise. Write the config comment for a **user deciding whether to set the key** — not for a reviewer of this change; if its first sentence cannot be understood without having read change 0269, rewrite it.

**Files:**
- Modify: `.docket.example.yml`
- Modify: `tests/test_docket_example_yml.sh`
- Modify: `README.md`
- Modify: `scripts/runners/codex.md`, `scripts/runners/cursor.md`, `scripts/runners/opencode.md`
- Modify: `skills/docket-convention/references/agent-layer.md`

**Interfaces:**
- Consumes: the `runners.codex.shim_model` / `runners.codex.shim_effort` keys implemented in Tasks 1–2; `sync-agents.sh` is their consumer anchor (it is the file that reads them).
- Produces: no code interface — documentation and guard state only.

- [ ] **Step 1: Write the failing guard asserts**

In `tests/test_docket_example_yml.sh`, find the completeness asserts block containing `completeness: runners.opencode.permissions present` and add immediately after it:

```bash
assert "completeness: runners.codex.shim_model present" \
  'grep -Eq "^[[:space:]]*shim_model:" "$EX"'
assert "completeness: runners.codex.shim_effort present" \
  'grep -Eq "^[[:space:]]*shim_effort:" "$EX"'
```

Then find `expected_key_count=45` and change it to:

```bash
# change 0269 took it from 45 to 47 (runners.codex.shim_model and runners.codex.shim_effort).
expected_key_count=47
```

Then find the `classify_key` arms for the runner knobs:

```bash
    runners.codex.sandbox) echo 'elsewhere:scripts/runners/codex.sh' ;;
```

Add above that line:

```bash
    # Read by the GENERATOR, not by an adapter: these govern the shim wrapper's own frontmatter
    # pin (the parent-side relay agent), which is decided at generation time — change 0269.
    runners.codex.shim_model)  echo 'elsewhere:sync-agents.sh' ;;
    runners.codex.shim_effort) echo 'elsewhere:sync-agents.sh' ;;
```

- [ ] **Step 2: Run the guard suite to verify it fails**

Run: `bash tests/test_docket_example_yml.sh 2>&1 | grep "^NOT OK"`
Expected: failures naming the missing keys and the key-count mismatch (the example file has not been edited yet).

- [ ] **Step 3: Document the keys in `.docket.example.yml`**

In `.docket.example.yml`, replace the `runners:` block:

```yaml
runners:
  codex:
    sandbox: workspace-write   # workspace-write (default) | danger-full-access
    network: true              # default true — git push and gh need it
  opencode:
    permissions: ask           # ask (default — REFUSES to delegate) | auto-approve
```

with:

```yaml
runners:
  codex:
    sandbox: workspace-write   # workspace-write (default) | danger-full-access
    network: true              # default true — git push and gh need it
    # shim_model / shim_effort — what the small relay agent in YOUR harness runs on. Delegation
    # generates a two-part wrapper: a relay that runs here in Claude Code and makes one call out to
    # the child, and the child that does the actual work on the `model:` you set under `agents:`.
    # These two keys pin the RELAY; they never touch the child. Leave them alone and the relay runs
    # on whatever model your session is using (`inherit`), which is always correct. Set shim_model
    # to a cheap model if you would rather not spend session-model tokens on a call that only waits
    # for output and passes it back. Available under every runner, not just codex.
    shim_model: inherit        # default inherit — any model ID YOUR harness can run
    shim_effort: low           # default low
  opencode:
    permissions: ask           # ask (default — REFUSES to delegate) | auto-approve
```

Both keys sit under the existing `runners:` block, which already carries its `# scope: any layer` tag directly above the `runners:` header — nested leaves inherit it, so no new scope tag is required. Verify that with Step 4 rather than assuming it.

- [ ] **Step 4: Run the guard suite to verify it passes**

Run: `bash tests/test_docket_example_yml.sh 2>&1 | grep "^NOT OK"`
Expected: no output. If a scope-tag assert reddens, add a `# scope: any layer (.docket.yml, .docket.local.yml, or global config.yml)` comment line directly above each new key.

- [ ] **Step 5: Correct the prose in README**

In `README.md`, in the *Runner delegation* rules list, find the bullet beginning:

```
- **A delegated agent must carry an explicit `model:` in your config.**
```

Insert a new bullet immediately **above** it:

```markdown
- **The wrapper's own `model:`/`effort:` are not the child's.** A generated shim is two agents in
  one file: a relay that Claude Code runs, and the child run it dispatches. The frontmatter pins the
  relay and must therefore name a model *Claude Code* can resolve — it defaults to `inherit` (the
  parent conversation's model) and is retuned per runner with `runners.<name>.shim_model` /
  `shim_effort`. The child's identity travels in the baked `--model` / `--effort` arguments, from
  the `model:` you set under `agents:`. Setting `shim_model` to a cheap model is a pure cost
  optimization: the relay only blocks on the child and passes its output back.
```

Then, in the same section, find the sentence:

```
`model:` is passed to the child verbatim (ADR-0015); `effort:` maps to
Codex's `model_reasoning_effort` (docket's `max` → codex `xhigh`).
```

Replace with:

```
`model:` is passed to the child verbatim (ADR-0015); `effort:` maps to
Codex's `model_reasoning_effort` (docket's `max` → codex `xhigh`). The wrapper's own frontmatter
pin is a separate value — see the shim-pin rule below.
```

- [ ] **Step 6: Correct the three runner contracts**

In each of `scripts/runners/codex.md`, `scripts/runners/cursor.md`, and `scripts/runners/opencode.md`, find the sentence describing what the generated shim carries (in `codex.md` and `cursor.md` this is the bullet containing "so a generated shim never omits"; in `opencode.md` locate the equivalent shim description near the top). Append this sentence to that bullet in all three files, verbatim:

```
The shim's own frontmatter `model:`/`effort:` pin the PARENT-side relay agent, not this child — they come from `runners.<name>.shim_model` / `shim_effort` (defaults `inherit` / `low`) and must name something the parent harness can resolve.
```

If a file's shim description does not exist as a bullet, add the sentence as its own paragraph directly under that contract's first mention of the shim.

- [ ] **Step 7: Correct the agent-layer reference**

In `skills/docket-convention/references/agent-layer.md`, find the closing sentence of the *Model and effort on a delegated agent* paragraph:

```
With no
flag baked the child uses its own default for the chosen model; the parent's effort stays in the
wrapper frontmatter and never reaches the child.
```

Replace with:

```
With no
flag baked the child uses its own default for the chosen model.

**The shim's own pin is a third value (change 0269).** A shim wrapper is executed by the PARENT
harness — its whole body is one foreground `docket.sh runner-dispatch` call plus a stdout relay — so
its frontmatter `model:`/`effort:` govern that relay and must be resolvable by the parent, never by
the child. They come from `runners.<name>.shim_model` and `runners.<name>.shim_effort`, resolved
per-key across the same layers as the rest of the block, defaulting to `inherit` and `low`. The
child's pin is the baked `--model` / `--effort` argument and only that. Pinning a shim to the
child's model tells the parent harness to run the relay on a model it cannot resolve, which kills
the run before the dispatch script is ever reached.
```

- [ ] **Step 8: Run the full suite**

Run: `scripts/run-tests.sh`
Expected: green. Read any trailing `OVER BUDGET:` line and act on it — it does not fail the run, so nothing else will catch it.

If a test in another file greps prose you just rewrote, **relocate the assert** to the artifact that owns the content rather than restoring the deleted wording — re-adding text to keep a grep green reinstates the duplication.

- [ ] **Step 9: Commit**

```bash
git add .docket.example.yml tests/test_docket_example_yml.sh README.md scripts/runners/codex.md scripts/runners/cursor.md scripts/runners/opencode.md skills/docket-convention/references/agent-layer.md
git commit -m "docs(0269): ship shim_model/shim_effort end-to-end and retire the bookkeeping premise"
```

---

## Verification at the build gate

- [ ] `scripts/run-tests.sh` is green across the whole suite (not only the three files this plan names).
- [ ] `bash sync-agents.sh --check` in this repo exits 0, or reports only drift that predates this branch.
- [ ] A real regeneration produces a delegated wrapper whose frontmatter `model:` is `inherit` and whose dispatch line still bakes the configured child model — confirm by reading one generated file, not by trusting the asserts.
