<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0173 — field_of() silently truncates a model ID containing / or :](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-07-31-0173-field-of-silently-truncates-a-model-id-containing-or.md)**
<!-- docket:backlink:end -->

# field_of() value-class truncation — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stop `sync-agents.sh` and `scripts/runner-dispatch.sh` from silently truncating a config value that contains `/` or `:` — widen each reader's value class to fit the YAML shape it actually parses, and make an unconsumable value in `sync-agents.sh` fail generation loudly before any wrapper is written.

**Architecture:** Two readers, deliberately asymmetric. `sync-agents.sh`'s `field_of()` parses a **flow map** (`agent: {model: x, effort: y}`), so its value class becomes "everything up to the flow-map delimiters" and it gains a `field_of_raw` companion plus a validator that aborts generation. `scripts/runner-dispatch.sh` parses a **block mapping** (one key per line), so its value class becomes "rest of line, comment stripped, trimmed", and its posture stays tolerant (skip, never die) because it runs mid-handoff on a live dispatch path.

**Tech Stack:** Bash 4+, POSIX ERE via `sed -nE`, awk. No new dependencies.

## Global Constraints

Copied from the spec and the repo's standing rules. Every task's requirements implicitly include this section.

- **ADR-0015 — model IDs are opaque passthrough values.** No vendor allowlist, no availability lookup, no normalization of the value's *content*. The fix is about the *class of characters the reader can carry*, never about judging the value.
- **Value class for the flow-map reader is `[^,}[:space:]]+`** — "everything up to the flow-map delimiters, not a character allowlist". This is byte-for-byte the class `hd_field` in `scripts/lib/harness-defaults.sh` already uses.
- **Raw class for the flow-map reader is `[^,}]*`**, trailing whitespace trimmed — what a YAML parser would see.
- **Key-side classes are NOT touched.** `sync-agents.sh` lines 336/338/351/361 and `runner-dispatch.sh`'s key extraction keep `[A-Za-z0-9._-]+`. YAML keys are legitimately narrow.
- **`runner-dispatch.sh:33`'s runner-name validation is NOT touched** (`*[!A-Za-z0-9._-]*|*..*`) — a filename-safety guard, correctly narrow.
- **`scripts/lib/harness-defaults.sh` is NOT touched** — fixed by change 0168.
- **Shell portability:** BSD/macOS `sed` and `grep`. Verify greps with `/usr/bin/grep`, never the interactive shell's `grep` (it is ugrep and accepts intervals BSD grep rejects). No GNU-only flags, no `grep -P`.
- **`set -o pipefail` is in force in `sync-agents.sh`.** Never end a pipeline in an early-exiting consumer (`head -n1` after a producer) in new code; use `${var%%$'\n'*}` or `sed '{p;q;}'`.
- **Existing behavior preserved:** `runner-dispatch.sh`'s per-key precedence claim (the key is claimed for its layer *before* its value is parsed, so a malformed high-precedence value still masks lower layers) must survive unchanged.

---

## File Structure

| File | Responsibility | Change |
|---|---|---|
| `sync-agents.sh` | The generator. Holds `field_of()` (flow-map reader) and the pre-write validation gate. | Modify: widen `field_of`, add `field_of_raw`, add `validate_user_agent_values`, wire it into both entry points. |
| `scripts/runner-dispatch.sh` | The runner delegation facade. Reads `runners.<name>.*` into `DOCKET_RUNNER_CFG_*`. | Modify: widen the block-mapping value read (1 statement + comment). |
| `tests/test_sync_agents.sh` | Generator test suite. | Modify: append two sections (reader coverage, validator coverage). |
| `tests/test_runner_dispatch.sh` | Facade + adapter test suite. | Modify: append one section (config value class). |
| `scripts/runner-dispatch.md` | The facade's authoritative contract. | Modify: document the value class + comment/trim rule. |

**Verified facts this plan is built on** (probed against `origin/main` @ `9d41fa6b`, not assumed):

- `field_of "…{model: anthropic/claude-opus-5…}" model` currently returns **`anthropic`**. Confirmed live.
- `field_of "…{model: openrouter:vendor/model…}" model` currently returns **`openrouter`**. Confirmed live.
- `runner-dispatch.sh`'s value read on `workdir: /Users/x/some/path` currently returns the **empty string** (the value starts with `/`, outside the class, so the regex does not match at all) — the key is then `continue`d and dropped entirely, *after* having been claimed in `seen_keys`. On `endpoint: https://example.test/v1` it returns **`https`**.
- `sync-agents.sh` lives at the **repo root**, not under `scripts/`.
- Layer variables: `LOCAL_CFG="$REPO/.docket.local.yml"` (line 70), `DOCKET_YML="$REPO/.docket.yml"` (line 69), `GLOBAL_CFG` (line 67). All three are read with `under_agents=1`.
- `per_repo_opted_in()` returns true when **either** `.docket.local.yml` **or** `.docket.yml` carries a top-level `agents:` or `agent_harnesses:` key — so a local-layer-only fixture does trigger per-repo generation.
- `log()` already prefixes every message with `sync-agents: `.
- `fm_anchored()` is defined at `tests/test_sync_agents.sh:1525`; new sections appended after it may use it.

### Design decision this plan makes explicit — the quote leg

The spec asks the validator to fail a **quoted** value and a **space-bearing** value. A strict mirror of the twin delivers only the second. Probed against the real `hd_field`/`hd_field_raw`:

| entry | consumed | raw | `consumed != raw` fires? |
|---|---|---|---|
| `{model: "quoted-model"}` | `"quoted-model"` | `"quoted-model"` | **no** |
| `{model: two words}` | `two` | `two words` | yes |
| `{model: anthropic/claude-opus-5}` | `anthropic/claude-opus-5` | same | no (correct — it round-trips) |

A quoted-but-space-free value has `consumed == raw`, so the `!=` leg never fires and the quotes ride into the emitted pin verbatim. The remedy text the spec dictates — *"write model/effort values unquoted and space-free"* — would therefore name a rule the validator cannot enforce.

**This plan adds an explicit quote leg** (a raw value whose first character is `"` or `'` is not a bare scalar) alongside the `!=` leg. This is an addition to the mirrored design, not a departure from it: the `!=` leg is kept byte-for-byte, and the quote leg only extends it to the case the `!=` comparison structurally cannot see. It is the one non-obvious decision in this change and should be recorded as an ADR at review.

The same gap exists in `hd_validate` in `scripts/lib/harness-defaults.sh`, which this change does **not** touch (out of scope per the spec). Task 4 records it as a follow-up rather than leaving it an unrecorded observation.

---

### Task 1: Widen `field_of` and add `field_of_raw`

**Files:**
- Modify: `sync-agents.sh:259-264` (the `field_of` block)
- Test: `tests/test_sync_agents.sh` (append a new section at end of file, before `exit $fail`)

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: `field_of <line> <field>` — unchanged signature, widened class, prints the consumed value or nothing. `field_of_raw <line> <field>` — same signature, prints the raw YAML-visible value (trailing whitespace trimmed) or nothing. Task 2's validator consumes both.

- [ ] **Step 1: Write the failing tests**

Append to `tests/test_sync_agents.sh`, immediately before the final `exit $fail` line:

```bash
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
  '! grep -q "model:.*}" "$SBX/.claude/agents/docket-status.md"'
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
assert "0173: merge — harness block wins for cursor, whole" \
  '[ "${$(fm_anchored "$SBX/.cursor/agents/docket-status.md" model)%%[*}" = "openrouter:vendor/model" ]'
assert "0173: merge — claude falls to agents.default, whole" \
  '[ "$(fm_anchored "$SBX/.claude/agents/docket-status.md" model)" = "anthropic/claude-opus-5" ]'
rm -rf "$SBX" "$HROOT173E"
```

> **Note for the implementer — one supplied assert is knowingly wrong.** The cursor merge assert
> above uses `${$(…)%%[*}`, which is **not valid bash** (parameter expansion does not take a command
> substitution as its parameter). It is written that way here to be caught, not copied: the Cursor
> wrapper encodes effort *inside* the model value as `model[effort=…]`, so the raw value needs the
> `[…]` suffix stripped before comparison. Replace it with a two-line form:
>
> ```bash
> cur_m="$(fm_anchored "$SBX/.cursor/agents/docket-status.md" model)"; cur_m="${cur_m%%[*}"
> assert "0173: merge — harness block wins for cursor, whole" '[ "$cur_m" = "openrouter:vendor/model" ]'
> ```
>
> Confirm the effort suffix is actually present or absent in your fixture before choosing whether to
> strip; this config sets no effort for cursor, so the value may be bare. Assert what the file
> really contains.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `bash tests/test_sync_agents.sh 2>&1 | /usr/bin/grep '0173'`

Expected: the three layer asserts and the merge asserts print `NOT OK` — with values truncated to `anthropic`, `openai`, `openrouter`. The plain-`sonnet` non-regression assert should already pass. If a `0173` assert passes before the fix, the fixture is not exercising the reader — stop and fix the fixture, do not proceed.

- [ ] **Step 3: Write the implementation**

Replace `sync-agents.sh` lines 259-264 (the whole `field_of` block, comment included) with:

```bash
# field_of() — the flow-map value reader (change 0173).
#
# The value class is "everything up to the flow-map delimiters" — NOT a character allowlist.
# ADR-0015 makes model IDs opaque passthrough with no vendor allowlist, and provider-prefixed IDs
# (`anthropic/claude-opus-5`, `openrouter:vendor/model`) are ordinary. The pre-0173 class
# ([A-Za-z0-9._-]+) did not REJECT such an ID — which would at least be honest — it TRUNCATED it to
# a first segment that still looks well-formed, and the generator baked that wrong pin into the
# wrapper with no warning. This is the same class, and the same fix, as hd_field in
# scripts/lib/harness-defaults.sh (change 0168); the two readers deliberately match.
# Anything this class cannot express is caught by validate_user_agent_values, not silently clipped.
field_of() {  # $1=line  $2=field
  local out
  out="$(printf '%s' "$1" | sed -nE "s/.*[{,[:space:]]${2}[[:space:]]*:[[:space:]]*([^,}[:space:]]+).*/\1/p")"
  head -n1 <<<"$out"
}

# field_of_raw() — the RAW field text: everything between the colon and the next flow-map delimiter
# (`,` or `}`), trailing whitespace trimmed. This is what a YAML parser would see; field_of is what
# DOCKET's reader consumes. validate_user_agent_values rejects any entry where the two disagree, so
# a value the reader cannot consume whole fails loudly instead of shipping as a truncated prefix.
# The `_raw` tier follows the existing pair convention — docket-frontmatter.sh has field/field_raw
# (ADR-0058), harness-defaults.sh has hd_field/hd_field_raw — though the split here is
# reader-capability, not quote-style.
field_of_raw() {  # $1=line  $2=field
  local out
  out="$(printf '%s' "$1" | sed -nE "s/.*[{,[:space:]]${2}[[:space:]]*:[[:space:]]*([^,}]*).*/\1/p")"
  out="$(head -n1 <<<"$out")"
  printf '%s' "$(sed -E 's/[[:space:]]+$//' <<<"$out")"
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `bash tests/test_sync_agents.sh 2>&1 | /usr/bin/grep -c 'NOT OK'`
Expected: `0`

Then run the whole file to confirm no pre-existing assert regressed:
Run: `bash tests/test_sync_agents.sh > /tmp/0173-t1.log 2>&1; echo "rc=$?"; /usr/bin/grep -c '^ok' /tmp/0173-t1.log`
Expected: `rc=0`, and the `ok` count is at least the pre-change count.

- [ ] **Step 5: Mutation-check the new asserts**

The asserts must detect the defect, not merely confirm the fix. Revert the class in a scratch copy and prove they go red — and confirm the mutation actually landed with a count, since a substitution that silently fails to match yields a green run with nothing mutated:

```bash
cp sync-agents.sh /tmp/0173-mut.sh
before=$(/usr/bin/grep -c '\[^,}\[:space:\]\]+' /tmp/0173-mut.sh)
sed -i '' 's/(\[^,}\[:space:\]\]+)/([A-Za-z0-9._-]+)/' /tmp/0173-mut.sh
after=$(/usr/bin/grep -c '\[^,}\[:space:\]\]+' /tmp/0173-mut.sh)
echo "mutation landed: $before -> $after"
```

Expected: the count drops (the mutation landed). Run the suite against the mutated copy and confirm the `0173` layer asserts go `NOT OK`. Discard `/tmp/0173-mut.sh` afterwards — never commit it.

- [ ] **Step 6: Commit**

```bash
git add sync-agents.sh tests/test_sync_agents.sh
git commit -m "fix(0173): widen field_of value class to the flow-map delimiters

A provider-prefixed model ID (anthropic/claude-opus-5) was silently truncated
to its first segment and baked into the wrapper as a wrong pin. Matches
hd_field in scripts/lib/harness-defaults.sh (change 0168). Adds field_of_raw
for the validator that follows."
```

---

### Task 2: Fail generation loudly on an unconsumable value

**Files:**
- Modify: `sync-agents.sh` (add `validate_user_agent_values` after the config helpers, ~line 362; wire into both entry points at ~995-1012)
- Test: `tests/test_sync_agents.sh` (append after Task 1's section)

**Interfaces:**
- Consumes: `field_of` and `field_of_raw` from Task 1; the existing `agents_block_harnesses`, `agent_keys`, `harness_agent_line` helpers; `log`.
- Produces: `validate_user_agent_values` — takes no arguments, reads `$LOCAL_CFG`, `$DOCKET_YML`, `$GLOBAL_CFG`, prints one diagnostic per offender on stderr, returns 0 when clean and 1 when any offender was found. Nothing later depends on it.

- [ ] **Step 1: Write the failing tests**

Append to `tests/test_sync_agents.sh` after Task 1's section:

```bash
# ---- 0173: the validator — unconsumable values fail generation, loudly, before any write ----
# Posture is deliberately asymmetric with runner-dispatch.sh: here a human is reading output and a
# wrong pin PERSISTS in a generated file, so generation aborts. Partial generation carrying a
# known-bad pin is precisely the harm this change exists to prevent.

# -- a space-bearing value: non-zero exit, named diagnostic, and NO wrapper written --
make_sandbox
HROOT173V="$(mktemp -d)"; mkdir -p "$HROOT173V/.claude"
printf 'agents:\n  default:\n    status: { model: two words, effort: high }\n' > "$SBX/.docket.yml"
v_err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT173V" bash "$SYNC" 2>&1 >/dev/null )"; v_rc=$?
assert "0173 validator: space-bearing value exits non-zero" '[ "$v_rc" != "0" ]'
assert "0173 validator: diagnostic names the harness/agent" 'printf "%s" "$v_err" | /usr/bin/grep -q "default/status"'
assert "0173 validator: diagnostic names the key"           'printf "%s" "$v_err" | /usr/bin/grep -q "'"'"'model'"'"'"'
assert "0173 validator: diagnostic quotes the RAW value"    'printf "%s" "$v_err" | /usr/bin/grep -qF "two words"'
assert "0173 validator: diagnostic names what was CONSUMED" 'printf "%s" "$v_err" | /usr/bin/grep -qF "consumes only"'
assert "0173 validator: says not a bare scalar"             'printf "%s" "$v_err" | /usr/bin/grep -qF "is not a bare scalar"'
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
assert "0173 validator: quoted diagnostic names the remedy" 'printf "%s" "$q_err" | /usr/bin/grep -qF "unquoted"'
assert "0173 validator: quoted value writes no wrapper" '[ ! -e "$SBX/.claude/agents/docket-status.md" ]'
rm -rf "$SBX" "$HROOT173Q"

# -- a genuinely MISSING value is a DIFFERENT diagnostic. Without this distinction a clip that
#    lands empty makes the error blame ABSENCE for what is really a quoting problem. --
make_sandbox
HROOT173M="$(mktemp -d)"; mkdir -p "$HROOT173M/.claude"
printf 'agents:\n  default:\n    status: { model: , effort: high }\n' > "$SBX/.docket.yml"
m_err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT173M" bash "$SYNC" 2>&1 >/dev/null )"; m_rc=$?
assert "0173 validator: empty value exits non-zero" '[ "$m_rc" != "0" ]'
assert "0173 validator: empty value uses the MISSING diagnostic" 'printf "%s" "$m_err" | /usr/bin/grep -qF "has no value"'
assert "0173 validator: empty value does NOT claim not-a-bare-scalar" \
  '! printf "%s" "$m_err" | /usr/bin/grep -qF "is not a bare scalar"'
rm -rf "$SBX" "$HROOT173M"

# -- every offender is reported, not just the first (collect-then-fail) --
make_sandbox
HROOT173A="$(mktemp -d)"; mkdir -p "$HROOT173A/.claude"
printf 'agents:\n  default:\n    status: { model: two words }\n    adr: { model: three more words }\n' > "$SBX/.docket.yml"
a_err="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT173A" bash "$SYNC" 2>&1 >/dev/null )"
assert "0173 validator: reports the first offender"  'printf "%s" "$a_err" | /usr/bin/grep -q "default/status"'
assert "0173 validator: reports the second offender too" 'printf "%s" "$a_err" | /usr/bin/grep -q "default/adr"'
rm -rf "$SBX" "$HROOT173A"

# -- --check validates too: CI must not pass against config a real run would refuse --
make_sandbox
HROOT173K="$(mktemp -d)"; mkdir -p "$HROOT173K/.claude"
printf 'agents:\n  default:\n    status: { model: two words }\n' > "$SBX/.docket.yml"
k_out="$( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT173K" bash "$SYNC" --check 2>&1 )"; k_rc=$?
assert "0173 validator: --check fails on an unconsumable value" '[ "$k_rc" != "0" ]'
rm -rf "$SBX" "$HROOT173K"

# -- a CLEAN provider-prefixed config passes the validator (it must not over-reject) --
make_sandbox
HROOT173P="$(mktemp -d)"; mkdir -p "$HROOT173P/.claude"
printf 'agents:\n  default:\n    status: { model: anthropic/claude-opus-5, effort: high }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT173P" bash "$SYNC" >/dev/null 2>&1 ); p_rc=$?
assert "0173 validator: clean provider-prefixed config still generates (rc=0)" '[ "$p_rc" = "0" ]'
assert "0173 validator: and the wrapper IS written" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
rm -rf "$SBX" "$HROOT173P"

# -- an absent agents: block is not an error (the overwhelmingly common case) --
make_sandbox
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null 2>&1 ); n_rc=$?
assert "0173 validator: no config at all still generates (rc=0)" '[ "$n_rc" = "0" ]'
rm -rf "$SBX"
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `bash tests/test_sync_agents.sh 2>&1 | /usr/bin/grep '0173 validator'`

Expected: the failure-posture asserts print `NOT OK` (generation currently succeeds and writes a wrapper pinned to the truncated `two`). The two "clean config still generates" asserts should already pass.

- [ ] **Step 3: Write the implementation**

Insert into `sync-agents.sh` immediately after `agents_block_harnesses()` (i.e. after line 362, before the `# --- emit a resolved wrapper to stdout ---` banner):

```bash
# --- user-config value validation (change 0173) ------------------------------
# field_of consumes bare scalars only. A value it cannot consume WHOLE — a quoted scalar, an
# embedded space — would otherwise be clipped to a prefix that still looks well-formed and baked
# into a wrapper as a wrong pin. Collect every offender across every layer, report them all, and
# fail BEFORE any wrapper is written: partial generation carrying a known-bad pin is exactly the
# harm this exists to prevent.
#
# Posture note: this is deliberately asymmetric with scripts/runner-dispatch.sh, which stays
# tolerant. Generation time has a human reading output and leaves a wrong pin persisted in a file;
# runner-dispatch runs mid-handoff on a live dispatch path, where dying would convert a cosmetic
# config typo into a failed dispatch.
#
# Only the harness-first shape is walked. The pre-0046 flat shape is warned about and DROPPED by
# warn_legacy_shape/legacy_agent_keys, so validating it would reject config that is already ignored.
validate_user_agent_values() {
  local rc=0 f h a k line raw consumed
  for f in "$LOCAL_CFG" "$DOCKET_YML" "$GLOBAL_CFG"; do
    [ -f "$f" ] || continue
    while IFS= read -r h; do
      [ -n "$h" ] || continue
      while IFS= read -r a; do
        [ -n "$a" ] || continue
        line="$(harness_agent_line "$f" "$h" "$a" 1)"
        [ -n "$line" ] || continue
        for k in model effort runner; do
          # Key absent from this entry is normal — every field is optional in user config.
          printf '%s' "$line" | grep -Eq "[{,[:space:]]${k}[[:space:]]*:" || continue
          raw="$(field_of_raw "$line" "$k")"
          consumed="$(field_of "$line" "$k")"
          if [ -z "$raw" ]; then
            # Present-but-empty. A DIFFERENT diagnostic from the one below on purpose: without the
            # split, a clip that lands empty blames ABSENCE for what is really a quoting problem.
            log "$h/$a '$k' is present but has no value ($f)"; rc=1
          elif [ "$raw" != "$consumed" ] || case "$raw" in '"'*|"'"*) true;; *) false;; esac; then
            # Two legs. The != leg catches anything the value class cannot express (an embedded
            # space). The quote leg catches what != structurally CANNOT see: a quoted but
            # space-free value has consumed == raw, so the quotes would ride into the emitted pin
            # verbatim while the diagnostic's own remedy text tells the user to write them unquoted.
            log "$h/$a '$k' value '$raw' is not a bare scalar — the reader consumes only '$consumed'; write model/effort values unquoted and space-free ($f)"
            rc=1
          fi
        done
      done < <(agent_keys "$f" 1)
    done < <(agents_block_harnesses "$f")
  done
  return $rc
}
```

Then wire it into **both** entry points in `sync-agents.sh`. In the `--check` branch, immediately after the existing `hd_validate` gate (after line 1003's `fi`):

```bash
    if ! validate_user_agent_values; then
      log "check: user agent config has unconsumable values — a real run would refuse to write wrappers."
      exit 1
    fi
```

And in the main path, immediately after the existing `hd_validate` gate (after line 1012's `fi`):

```bash
  # Same gate for USER config (change 0173): validate before writing any wrapper.
  if ! validate_user_agent_values; then
    log "ERROR user agent config has unconsumable values — no wrappers were written."
    exit 1
  fi
```

> Placement matters: both must sit **before** `migrate_legacy_global` / `user_level_pass`, so no
> `mkdir -p` and no `emit_wrapper` redirection has run. The `[ ! -d .claude/agents ]` assert in the
> tests is what pins this.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `bash tests/test_sync_agents.sh > /tmp/0173-t2.log 2>&1; echo "rc=$?"; /usr/bin/grep -c 'NOT OK' /tmp/0173-t2.log`
Expected: `rc=0` and a `NOT OK` count of `0`.

- [ ] **Step 5: Verify the real repo's own config still generates**

The repo's own `.docket.yml` and any local/global layer must pass the new gate — a validator that rejects docket's own config is a broken validator, and this is the one fixture the test suite cannot supply.

Run: `cd "$(git rev-parse --show-toplevel)" && bash sync-agents.sh --check; echo "rc=$?"`
Expected: the same rc as before this task (run it on a stashed tree first if unsure). Any *new* `is not a bare scalar` or `has no value` line naming a real docket agent is a defect in this task, not in the config.

- [ ] **Step 6: Commit**

```bash
git add sync-agents.sh tests/test_sync_agents.sh
git commit -m "fix(0173): fail generation on a user-config value field_of cannot consume

Collects every offender across all three layers and both config shapes,
reports them on stderr, and exits non-zero BEFORE any wrapper is written.
Distinguishes an unconsumable value from a missing one, and catches a quoted
but space-free value that the raw/consumed comparison structurally cannot see."
```

---

### Task 3: Widen `runner-dispatch.sh`'s block-mapping value class

**Files:**
- Modify: `scripts/runner-dispatch.sh:75` (the `v=` assignment) and its surrounding comment
- Modify: `scripts/runner-dispatch.md` (document the value class)
- Test: `tests/test_runner_dispatch.sh` (append before `exit $fail`)

**Interfaces:**
- Consumes: nothing from earlier tasks — this reader is independent and shares no code with `sync-agents.sh`.
- Produces: no new function. `DOCKET_RUNNER_CFG_<KEY>` exports now carry the whole value.

- [ ] **Step 1: Write the failing tests**

Append to `tests/test_runner_dispatch.sh`, immediately before the final `exit $fail`:

```bash
# ---- 0173: runners.<name> value class — block mapping, tolerant posture -------------
# The facade exports runners.<name>.* as DOCKET_RUNNER_CFG_*. These values are free-form and more
# likely to be paths or URLs than model IDs, so the class is "rest of line", not the flow-map class
# sync-agents.sh uses. Asserts read the exported value directly rather than through the codex
# adapter, so they pin the READER and not the adapter's flag mapping.
make_fixture
probe(){  # $1 = yaml value text -> prints the resulting DOCKET_RUNNER_CFG_PROBEKEY
  printf 'runners:\n  codex:\n    probekey: %s\n' "$1" > "$SBX/.docket.yml"
  cat > "$BIN/../probe.sh" <<'PROBE'
#!/usr/bin/env bash
printf '%s' "${DOCKET_RUNNER_CFG_PROBEKEY-<unset>}"
PROBE
  chmod +x "$SBX/probe.sh"
  ( cd "$SBX" && PATH="$BIN:$PATH" RUNNERS_DIR="$SBX/runners" bash "$FACADE" --runner probe --agent status 2>/dev/null )
}
mkdir -p "$SBX/runners"
cat > "$SBX/runners/probe.sh" <<'PROBE'
#!/usr/bin/env bash
printf '%s' "${DOCKET_RUNNER_CFG_PROBEKEY-<unset>}"
PROBE
chmod +x "$SBX/runners/probe.sh"

assert "0173 rd: slash-bearing value arrives intact" \
  '[ "$(probe "/Users/x/some/path")" = "/Users/x/some/path" ]'
assert "0173 rd: colon-bearing URL value arrives intact" \
  '[ "$(probe "https://example.test/v1")" = "https://example.test/v1" ]'
assert "0173 rd: trailing comment stripped, whitespace trimmed" \
  '[ "$(probe "workspace-write   # why we chose it")" = "workspace-write" ]'
assert "0173 rd: a plain value is unchanged (non-regression)" \
  '[ "$(probe "danger-full-access")" = "danger-full-access" ]'
rm -rf "$SBX"

# -- tolerant posture: an unparseable value skips WITHOUT dying, and still masks lower layers --
make_fixture
mkdir -p "$SBX/runners"
cat > "$SBX/runners/probe.sh" <<'PROBE'
#!/usr/bin/env bash
printf '%s' "${DOCKET_RUNNER_CFG_SANDBOX-<unset>}"
PROBE
chmod +x "$SBX/runners/probe.sh"
# High-precedence layer claims `sandbox` with an EMPTY value; the committed layer sets a real one.
printf 'runners:\n  codex:\n    sandbox:\n' > "$SBX/.docket.local.yml"
printf 'runners:\n  codex:\n    sandbox: danger-full-access\n' > "$SBX/.docket.yml"
tol_out="$( cd "$SBX" && PATH="$BIN:$PATH" RUNNERS_DIR="$SBX/runners" bash "$FACADE" --runner probe --agent status 2>/dev/null )"; tol_rc=$?
assert "0173 rd: malformed high-precedence value does not kill the dispatch" '[ "$tol_rc" = "0" ]'
assert "0173 rd: and it still MASKS the lower layer (per-key precedence preserved)" \
  '[ "$tol_out" = "<unset>" ]'
rm -rf "$SBX"
```

> **Note for the implementer — the `probe()` helper above is over-built and partly wrong.** It
> writes a probe script to two different paths (`"$BIN/../probe.sh"` and `"$SBX/runners/probe.sh"`)
> and only the second is used. Delete the `cat > "$BIN/../probe.sh"` block and the `chmod +x
> "$SBX/probe.sh"` line from inside `probe()`; the adapter written once before the asserts is
> sufficient. Also confirm the facade actually reaches a custom `RUNNERS_DIR` adapter in this
> fixture — run one probe by hand and look at its output before trusting any of the four asserts.
> If the `RUNNERS_DIR` seam does not compose with `$FACADE` the way this assumes, fall back to
> asserting through the existing `codex` adapter with `sandbox:`/`network:` keys, as the
> pre-existing "facade: runners.<name> config resolution across layers" section already does.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `bash tests/test_runner_dispatch.sh 2>&1 | /usr/bin/grep '0173 rd'`

Expected: the slash, URL, and comment asserts print `NOT OK`. Specifically the slash case yields the empty string today (the value starts with `/`, so the regex does not match at all), and the URL case yields `https`.

- [ ] **Step 3: Write the implementation**

In `scripts/runner-dispatch.sh`, replace the `v=` line (line 75) and add the explanatory comment:

```bash
    # Value class (change 0173): rest of the line, trailing `# comment` stripped, whitespace
    # trimmed. This is a BLOCK mapping (one key per line), not sync-agents.sh's flow map, so the
    # flow-map class would be wrong here — it would admit a slash-bearing path only by luck. The
    # pre-0173 class ([A-Za-z0-9._-]+) truncated `https://host/v1` to `https` and dropped a
    # leading-slash path entirely. Comment detection requires leading whitespace, per YAML, so a
    # value containing `#` (a URL fragment) survives.
    # Posture stays TOLERANT — an unparseable value `continue`s rather than dying. This path runs
    # mid-handoff to a child process, where dying converts a cosmetic config typo into a failed
    # dispatch. sync-agents.sh is loud instead; the asymmetry is deliberate (change 0173).
    v="$(sed -nE 's/^[[:space:]]*[A-Za-z0-9._-]+[[:space:]]*:[[:space:]]*(.*)$/\1/p' <<<"$line")"
    v="${v%%$'\n'*}"
    v="$(sed -E -e 's/[[:space:]]+#.*$//' -e 's/[[:space:]]+$//' <<<"$v")"
    [ -n "$v" ] || continue
```

> The `seen_keys` claim above this block is **not** moved — the key is still claimed for its layer
> before the value is parsed, so a malformed high-precedence value still masks lower layers.
> Precedence is per-key, not per-value.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `bash tests/test_runner_dispatch.sh > /tmp/0173-t3.log 2>&1; echo "rc=$?"; /usr/bin/grep -c 'NOT OK' /tmp/0173-t3.log`
Expected: `rc=0`, `NOT OK` count `0`. The pre-existing "facade: runners.<name> config resolution across layers" asserts must still pass — they are the non-regression proof for this reader.

- [ ] **Step 5: Update the contract**

In `scripts/runner-dispatch.md`, find the section describing `runners.<name>` config resolution and add the value-class rule to it:

```markdown
A value is read as the **rest of the line** after `<key>:`, with a whitespace-preceded `#` comment
stripped and surrounding whitespace trimmed — so paths and URLs (`/Users/x/p`,
`https://host/v1`) survive intact. A value that parses to nothing is **skipped, not fatal**: this
runs on a live dispatch path, so a cosmetic config typo must not convert into a failed dispatch.
The key is still claimed for its layer before its value is parsed, so a malformed
high-precedence value masks the same key in lower layers (precedence is per-key, not per-value).
```

Place it adjacent to the existing precedence prose rather than as a new trailing section, so the two rules read together.

- [ ] **Step 6: Commit**

```bash
git add scripts/runner-dispatch.sh scripts/runner-dispatch.md tests/test_runner_dispatch.sh
git commit -m "fix(0173): widen runners.<name> value class to the rest of the line

A path or URL value was truncated (https://host/v1 -> https) or dropped
entirely (a leading-slash path matched nothing). Block-mapping class with a
comment strip and trim; posture stays tolerant because this is a live
dispatch path. Per-key precedence claim preserved."
```

---

### Task 4: Full-suite gate and follow-up capture

**Files:**
- No production files. This task runs the suite and records what the build surfaced.

**Interfaces:**
- Consumes: everything from Tasks 1-3.
- Produces: a green suite, and a named follow-up for the twin gap.

- [ ] **Step 1: Run the full suite in one foreground pass**

Run the repo's whole test suite in a **single foreground command** (it takes roughly ten minutes; do not background it):

```bash
cd "$(git rev-parse --show-toplevel)" && bash tests/run-all.sh
```

If `tests/run-all.sh` does not exist, discover the entry point (`ls tests/`) and run every
`tests/test_*.sh` in one loop under an explicit `bash`, never the interactive shell:

```bash
bash -c 'cd "$(git rev-parse --show-toplevel)"; rc=0; for t in tests/test_*.sh; do
  echo "== $t"; bash "$t" >/dev/null 2>&1 || { echo "FAIL $t"; rc=1; }; done; exit $rc'
```

Expected: exit 0. A red test outside `test_sync_agents.sh` / `test_runner_dispatch.sh` is a real
regression from this change until proven otherwise — re-run it on the unmodified base before
concluding it was already broken.

- [ ] **Step 2: Run the change's own thesis over its own additions**

This change exists because a narrow character class silently truncates a value. New code added by
such a change is the likeliest place for the same class to reappear. Grep the branch's additions:

```bash
git diff origin/main... -- '*.sh' | /usr/bin/grep '^+' | /usr/bin/grep -n 'A-Za-z0-9'
```

Every surviving occurrence must be a **key-side** read or the runner-name safety guard. Any
value-side occurrence introduced by this branch is the defect reappearing — fix it before review.

- [ ] **Step 3: Record the untouched twin as a follow-up**

`hd_validate` in `scripts/lib/harness-defaults.sh` has the same quote gap this change closed in
`sync-agents.sh`: a quoted but space-free value (`{model: "claude-opus-5"}`) has `consumed == raw`,
so its bare-scalar leg does not fire and the quotes ride into the shipped default. The spec places
that file out of scope, so do **not** fix it here — surface it in the run report as follow-up work
so it is tracked rather than left an unrecorded observation.

- [ ] **Step 4: Commit any straggling fixes**

```bash
git add -A
git commit -m "test(0173): full-suite gate"
```

Skip this commit if the tree is clean — Tasks 1-3 should have committed everything.

---

## Self-Review

**Spec coverage:**

| Spec requirement | Task |
|---|---|
| `field_of` value class widened to `[^,}[:space:]]+` | 1 |
| `field_of_raw` companion, `[^,}]*`, trailing whitespace trimmed | 1 |
| Naming follows the `field`/`field_raw` pair convention | 1 |
| Validator fails loudly, before anything is written | 2 |
| Collects every offender across all layers, reports all | 2 |
| Diagnostic distinguishes unconsumable from missing | 2 |
| Diagnostic message shape mirrors `hd_validate`'s | 2 (plus the layer filename, additively) |
| `runner-dispatch.sh` block-mapping class, comment strip, trim | 3 |
| `runner-dispatch.sh` posture stays tolerant | 3 |
| Per-key precedence claim preserved | 3 |
| Coverage: three user layers independently | 1 |
| Coverage: `agents.default` vs `agents.<harness>` merge | 1 |
| Coverage: slash, colon, both, plain non-regression | 1 |
| Coverage: quoted + space-bearing fail, no wrapper written | 2 |
| Coverage: missing value ≠ unconsumable value | 2 |
| Coverage: runner-dispatch slash, colon, comment, tolerant skip | 3 |
| Not touched: key-side classes, runner-name guard, `harness-defaults.sh` | Global Constraints + Task 4 Step 2 |

No spec requirement is unassigned.

**Deliberate additions beyond a strict reading of the spec**, both flagged inline where they occur:

1. **The quote leg** in Task 2 — the spec's own quoted-value coverage is unsatisfiable without it (probed, table in *File Structure*). This is the change's one ADR-worthy decision.
2. **The layer filename in the diagnostic** — with three layers, "which file" is the first thing a user needs. Additive to the specced shape, so shape asserts still hold.

**Placeholder scan:** every code step carries real code; no "add error handling", no "similar to Task N", no TBD.

**Type consistency:** `field_of` / `field_of_raw` keep one signature (`$1=line $2=field`) across Tasks 1 and 2. `validate_user_agent_values` takes no arguments and returns 0/1 in both call sites. `RES_MODEL_FROM_HARNESS` is only read in a test, never redefined.

**Two supplied test snippets are knowingly defective and marked as such** (the cursor merge assert in Task 1, the `probe()` helper in Task 3). Plan-supplied test code is unverified code, not an oracle — every assert here must be shown to *fail before* and *pass after*, and the two marked ones must be repaired against real output rather than copied.
