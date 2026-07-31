<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0168 — Cursor support for profile-routed Docket builds](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0168-cursor-profile-routed-build-support.md)**
<!-- docket:backlink:end -->

# Cursor Support for Profile-Routed Docket Builds — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move every shipped agent model/effort default out of the twelve `agents/docket-*.md` wrapper sources into one harness-indexed sidecar, and ship three Cursor-native build-profile defaults from it.

**Architecture:** A new shipped-data file `agents/harness-defaults.yml` becomes the lowest resolution layer (below machine-local, committed, and global user config). `sync-agents.sh` gains a shipped-layer lookup plus provenance tracking, and its three emitters stop reading `model:`/`effort:` off the source frontmatter — they consume a single resolved pair instead. Only then are the fields deleted from the twelve sources. Tasks 1–4 are deliberately output-preserving: because the sidecar's Claude values are byte-identical to today's source frontmatter, every generated Claude wrapper must come out unchanged, which is the plan's main safety property.

**Tech Stack:** Bash 3.2-compatible shell (`sync-agents.sh`, `scripts/lib/`), `awk`/`sed` text processing, the repo's hand-rolled `tests/test_*.sh` assert harness.

## Global Constraints

- **Bash 3.2 / BSD userland.** No `declare -A`, no `mapfile`, no GNU-only `sed -i` form. `/usr/bin/grep` is the portability target — PATH `grep` is ugrep and accepts intervals BSD grep rejects. Re-check any new regex under `/usr/bin/grep`.
- **`set -o pipefail` is in force.** Never `producer | grep -q` / `| head` — capture into a variable, then `grep <<<"$var"`.
- **ADR-0015 opaque passthrough.** Model IDs and effort tokens are never validated against a vendor allowlist. Structural validation only.
- **ADR-0060.** Each per-harness emitter is a pure target-contract translation. No emitter learns which concrete models docket recommends.
- **No harness-neutral `default:` block** is permitted in the sidecar. Every entry nests under a concrete harness.
- **`runner` is forbidden in the sidecar.** Delegation is user policy, never a shipped default.
- **Claude generated output must not change.** Every one of the twelve Claude wrappers keeps its exact resolved model and effort.
- **Exact shipped values** — Claude: `adr` `claude-opus-5`/`low`; `auto-groom` `claude-opus-5`/`low`; `auto-groom-critic` `claude-opus-5`/`medium`; `brainstorm-consultant` `claude-opus-5`/`medium`; `build-economy` `claude-opus-5`/`low`; `build-standard` `claude-opus-5`/`medium`; `build-premium` `claude-opus-5`/`high`; `finalize-change` `claude-opus-5`/`low`; `implement-next` `claude-opus-5`/`medium`; `integration-repair` `claude-opus-5`/`medium`; `rebase-resolver` `claude-opus-5`/`medium`; `status` `claude-haiku-4-5-20251001`/`medium`. Cursor: `build-economy` `cursor-grok-4.5-medium`/`auto`; `build-standard` `cursor-grok-4.5-high`/`auto`; `build-premium` `claude-opus-5-high`/`auto`.
- **Guards are code** (AGENTS.md): every new assert must be mutation-tested — break the thing it guards, watch it redden — and the mutation must be confirmed to have landed (`grep -c` before/after).
- **Commit after every task.** Run the whole suite at the final gate, never only the enumerated tests.

---

## File Structure

**Created**
- `agents/harness-defaults.yml` — the shipped default sidecar. Program data, not user config.
- `scripts/lib/harness-defaults.sh` — reader + structural validator, sourced by `sync-agents.sh`. Lives in `scripts/lib/` beside `docket-gitignore-block.sh`, the established home for sourced helpers.
- `tests/test_harness_defaults.sh` — validator unit tests + set-equality guards in both directions.

**Modified**
- `sync-agents.sh` — shipped layer in resolution, provenance tracking, three emitters consume resolved values, `emit()` inserts instead of substitutes, `warn_fallback_model()` re-worded, sidecar validated before any write.
- `agents/docket-*.md` (12 files) — `model:`/`effort:` frontmatter removed.
- `tests/test_sync_agents.sh` — default asserts re-pointed at the sidecar; negative guard added.
- `tests/test_sync_agents_cursor.sh` — byte-identity assert rewritten; Cursor build-profile asserts added.
- `tests/test_sync_agents_codex.sh` — byte-identity assert rewritten.
- `tests/test_docket_example_yml.sh` — mirror-equality loop re-pointed at the sidecar.
- `tests/test_docket_build.sh` — profile asserts re-pointed at the sidecar.
- `tests/test_composition_wiring.sh` — stale source-of-truth comment corrected.
- `.docket.example.yml` — mirror re-pointed; Cursor block distinguishes shipped from illustrative.
- `README.md` — count-free `## Skills` catalog; falsified shipped-model paragraph rewritten.
- `skills/docket-convention/references/agent-layer.md`, `skills/docket-convention/SKILL.md`, `skills/docket-build/SKILL.md`, `docs/cursor/validation.md`, `docs/codex/setup.md`, `scripts/runners/*.md`, `scripts/runner-dispatch.md` — prose re-pointed.

---

### Task 1: The shipped sidecar and its validator

**Files:**
- Create: `agents/harness-defaults.yml`
- Create: `scripts/lib/harness-defaults.sh`
- Test: `tests/test_harness_defaults.sh`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces, all sourced from `scripts/lib/harness-defaults.sh`:
  - `hd_field <file> <harness> <agent> <field>` → prints the value, or empty. `<field>` is `model` or `effort`.
  - `hd_agents <file> <harness>` → prints the agent short-names under `<harness>`, one per line, sorted.
  - `hd_harnesses <file>` → prints the harness keys, one per line, sorted.
  - `hd_validate <file> <sources-dir>` → exit 0 valid; exit 1 with one or more `harness-defaults: …` diagnostics on stderr.

- [ ] **Step 1: Write the failing test**

Create `tests/test_harness_defaults.sh`:

```bash
#!/usr/bin/env bash
# tests/test_harness_defaults.sh — run: bash tests/test_harness_defaults.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

. "$REPO/scripts/lib/harness-defaults.sh"
HD="$REPO/agents/harness-defaults.yml"
SRC="$REPO/agents"

# ---- the shipped file itself ------------------------------------------------
assert "sidecar exists"            '[ -f "$HD" ]'
assert "sidecar validates"         'hd_validate "$HD" "$SRC"'
assert "harnesses are claude+cursor only" \
  '[ "$(hd_harnesses "$HD" | tr "\n" " ")" = "claude cursor " ]'

# ---- every shipped Claude value, verbatim -----------------------------------
for pair in \
  "adr claude-opus-5 low" \
  "auto-groom claude-opus-5 low" \
  "auto-groom-critic claude-opus-5 medium" \
  "brainstorm-consultant claude-opus-5 medium" \
  "build-economy claude-opus-5 low" \
  "build-standard claude-opus-5 medium" \
  "build-premium claude-opus-5 high" \
  "finalize-change claude-opus-5 low" \
  "implement-next claude-opus-5 medium" \
  "integration-repair claude-opus-5 medium" \
  "rebase-resolver claude-opus-5 medium" \
  "status claude-haiku-4-5-20251001 medium" ; do
  set -- $pair
  assert "claude/$1 = $2/$3" \
    '[ "$(hd_field "$HD" claude "'"$1"'" model)/$(hd_field "$HD" claude "'"$1"'" effort)" = "'"$2"'/'"$3"'" ]'
done

# ---- the three Cursor build profiles ----------------------------------------
assert "cursor/build-economy = cursor-grok-4.5-medium/auto" \
  '[ "$(hd_field "$HD" cursor build-economy model)/$(hd_field "$HD" cursor build-economy effort)" = "cursor-grok-4.5-medium/auto" ]'
assert "cursor/build-standard = cursor-grok-4.5-high/auto" \
  '[ "$(hd_field "$HD" cursor build-standard model)/$(hd_field "$HD" cursor build-standard effort)" = "cursor-grok-4.5-high/auto" ]'
assert "cursor/build-premium = claude-opus-5-high/auto" \
  '[ "$(hd_field "$HD" cursor build-premium model)/$(hd_field "$HD" cursor build-premium effort)" = "claude-opus-5-high/auto" ]'
assert "cursor block is exactly the three build workers" \
  '[ "$(hd_agents "$HD" cursor | tr "\n" " ")" = "build-economy build-standard build-premium " ]'
assert "no codex block yet (change 0169 owns it)" '[ -z "$(hd_agents "$HD" codex)" ]'
assert "unlisted pair resolves empty" '[ -z "$(hd_field "$HD" cursor status model)" ]'

# ---- set correspondence, BOTH directions ------------------------------------
# forward: every claude entry names a real source wrapper
while IFS= read -r a; do
  [ -n "$a" ] || continue
  assert "claude/$a has a source wrapper" '[ -f "$SRC/docket-'"$a"'.md" ]'
done < <(hd_agents "$HD" claude)
# reverse: every source wrapper has a claude entry (anchored on the real glob, not a list)
for f in "$SRC"/docket-*.md; do
  n="$(basename "$f" .md)"; n="${n#docket-}"
  assert "source $n has a claude entry" '[ -n "$(hd_field "$HD" claude "'"$n"'" model)" ]'
done
# reverse: every cursor entry is a build worker
while IFS= read -r a; do
  [ -n "$a" ] || continue
  assert "cursor/$a is a build worker" '[ -f "$SRC/docket-'"$a"'.md" ] && case "'"$a"'" in build-*) true;; *) false;; esac'
done < <(hd_agents "$HD" cursor)

# ---- validator rejects each malformed shape ---------------------------------
T="$(mktemp -d)"
mut(){ cp "$HD" "$T/hd.yml"; }

mut; sed -i.bak '/^    status:/d' "$T/hd.yml"
assert "reject: missing a claude entry" '! hd_validate "$T/hd.yml" "$SRC" 2>/dev/null'

mut; printf '    phantom:               { model: x, effort: low }\n' >> "$T/hd.yml"
assert "reject: phantom agent key" '! hd_validate "$T/hd.yml" "$SRC" 2>/dev/null'

mut; printf '\n  default:\n    adr:                   { model: x, effort: low }\n' >> "$T/hd.yml"
assert "reject: harness-neutral default block" '! hd_validate "$T/hd.yml" "$SRC" 2>/dev/null'

mut; printf '\n  bogus:\n    adr:                   { model: x, effort: low }\n' >> "$T/hd.yml"
assert "reject: unknown harness key" '! hd_validate "$T/hd.yml" "$SRC" 2>/dev/null'

mut; sed -i.bak 's|^    adr:.*|    adr:                   { model: claude-opus-5 }|' "$T/hd.yml"
assert "reject: entry missing effort" '! hd_validate "$T/hd.yml" "$SRC" 2>/dev/null'

mut; sed -i.bak 's|^    adr:.*|    adr:                   { model: claude-opus-5, effort: low, runner: codex }|' "$T/hd.yml"
assert "reject: runner is forbidden" '! hd_validate "$T/hd.yml" "$SRC" 2>/dev/null'

mut; sed -i.bak 's|^    build-economy:.*cursor-grok-4.5-medium.*|    build-economy:         { model: , effort: auto }|' "$T/hd.yml"
assert "reject: empty field value" '! hd_validate "$T/hd.yml" "$SRC" 2>/dev/null'

assert "reject: missing file" '! hd_validate "$T/nope.yml" "$SRC" 2>/dev/null'
rm -rf "$T"

[ "$fail" = 0 ] && echo "PASS" || echo "FAIL"
exit "$fail"
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_harness_defaults.sh`
Expected: FAIL — `scripts/lib/harness-defaults.sh: No such file or directory`.

- [ ] **Step 3: Create the sidecar**

Create `agents/harness-defaults.yml` exactly:

```yaml
# agents/harness-defaults.yml — docket's SHIPPED per-harness agent model/effort defaults.
#
# This is program data, NOT a user configuration file. It is the lowest layer of the agent
# resolution order; every user layer (machine-local .docket.local.yml, committed .docket.yml,
# global config.yml) overrides it FIELD BY FIELD. See ADR-0016 and the agent-layer reference.
#
# Rules this file must satisfy (enforced by scripts/lib/harness-defaults.sh, run before
# sync-agents.sh writes any wrapper):
#   - every entry nests under a CONCRETE harness; a harness-neutral `default:` block is forbidden,
#     because that is exactly the cross-harness model leakage this file exists to remove;
#   - keys are wrapper SHORT names (build-economy, not docket-build-economy);
#   - a listed entry supplies BOTH model and effort — the table is sparse by harness and agent,
#     never by field;
#   - `runner:` is forbidden — delegation is user policy, never a shipped default;
#   - the claude block is COMPLETE (its key set equals agents/docket-*.md);
#   - the cursor block is COMPLETE for the build profiles (its key set equals agents/docket-build-*.md).
#
# Model IDs and effort tokens are opaque passthrough values (ADR-0015). Docket keeps no vendor
# allowlist; an unrecognized string is never rejected here.
agents:
  claude:
    adr:                   { model: claude-opus-5, effort: low }
    auto-groom:            { model: claude-opus-5, effort: low }
    auto-groom-critic:     { model: claude-opus-5, effort: medium }
    brainstorm-consultant: { model: claude-opus-5, effort: medium }
    build-economy:         { model: claude-opus-5, effort: low }
    build-standard:        { model: claude-opus-5, effort: medium }
    build-premium:         { model: claude-opus-5, effort: high }
    finalize-change:       { model: claude-opus-5, effort: low }
    implement-next:        { model: claude-opus-5, effort: medium }
    integration-repair:    { model: claude-opus-5, effort: medium }
    rebase-resolver:       { model: claude-opus-5, effort: medium }
    status:                { model: claude-haiku-4-5-20251001, effort: medium }

  # Cursor's three build-profile workers. Each ID is a COMPLETE Cursor built-in whose variant is
  # already encoded, so effort is `auto`: the Cursor emitter writes the ID verbatim rather than
  # appending a second, conflicting `[effort=…]` suffix.
  cursor:
    build-economy:         { model: cursor-grok-4.5-medium, effort: auto }
    build-standard:        { model: cursor-grok-4.5-high, effort: auto }
    build-premium:         { model: claude-opus-5-high, effort: auto }
```

- [ ] **Step 4: Create the reader/validator library**

Create `scripts/lib/harness-defaults.sh`:

```bash
#!/usr/bin/env bash
# scripts/lib/harness-defaults.sh — reader + structural validator for agents/harness-defaults.yml,
# docket's SHIPPED per-harness agent model/effort default layer (change 0168).
#
# Sourced by sync-agents.sh. Validation runs BEFORE any wrapper is written, so a malformed sidecar
# can never leave a half-regenerated agent directory.
#
# The file's shape is fixed and shallow (agents: -> <harness>: -> <agent>: { model: , effort: }),
# so these readers parse it directly rather than pulling in a YAML dependency. They deliberately do
# NOT reuse sync-agents.sh's section_body/field_of: those read USER config, whose shape is looser,
# and coupling the shipped-data reader to them would let a user-config change silently reshape
# program data.

# Known harness tokens. Adding one here is not enough to ship defaults for it — the emitter and the
# set-equality rules in hd_validate decide what a complete block means.
HD_KNOWN_HARNESSES="claude cursor codex"

# Print the body lines under `  <harness>:` (four-space-indented entries), comments stripped.
_hd_block(){ # $1=file $2=harness
  [ -f "$1" ] || return 0
  awk -v h="$2" '
    { nc=$0; sub(/#.*/,"",nc) }
    nc ~ "^  "h"[[:space:]]*:[[:space:]]*$" { inb=1; next }
    inb && nc ~ /^  [A-Za-z0-9._-]+[[:space:]]*:/ { inb=0 }
    inb && nc ~ /^    [A-Za-z0-9._-]+[[:space:]]*:/ { print nc }
  ' "$1"
}

# Print the harness keys (two-space-indented, bare) present under agents:.
hd_harnesses(){ # $1=file
  [ -f "$1" ] || return 0
  awk '
    { nc=$0; sub(/#.*/,"",nc) }
    nc ~ /^  [A-Za-z0-9._-]+[[:space:]]*:[[:space:]]*$/ {
      k=nc; sub(/^  /,"",k); sub(/[[:space:]]*:.*/,"",k); if (k!="") print k
    }' "$1" | sort -u
}

# Print the agent short-names under <harness>, in FILE ORDER. Callers that need a set comparison
# sort explicitly; file order is what makes the "exactly the three build workers, in ladder order"
# assertion readable.
hd_agents(){ # $1=file $2=harness
  _hd_block "$1" "$2" | sed -e 's/^    //' -e 's/[[:space:]]*:.*//'
}

# Print the value of <field> for (harness, agent), or nothing.
hd_field(){ # $1=file $2=harness $3=agent $4=model|effort
  local line
  line="$(_hd_block "$1" "$2" | grep -E "^    $3[[:space:]]*:" || true)"
  line="$(head -n1 <<<"$line")"
  [ -n "$line" ] || return 0
  printf '%s' "$line" | sed -nE "s/.*[{,[:space:]]$4[[:space:]]*:[[:space:]]*([A-Za-z0-9._-]+).*/\1/p" | head -n1
}

# Validate the sidecar against <sources-dir> (agents/). Exit 1 with diagnostics on stderr.
hd_validate(){ # $1=file $2=sources-dir
  local f="$1" src="$2" rc=0 h a line k v n
  if [ ! -f "$f" ] || [ ! -r "$f" ]; then
    echo "harness-defaults: missing or unreadable: $f" >&2; return 1
  fi
  if ! grep -qE '^agents:[[:space:]]*$' "$f"; then
    echo "harness-defaults: no top-level 'agents:' block" >&2; rc=1
  fi
  for h in $(hd_harnesses "$f"); do
    if [ "$h" = "default" ]; then
      echo "harness-defaults: a harness-neutral 'default:' block is forbidden — every entry must name a concrete harness" >&2; rc=1; continue
    fi
    case " $HD_KNOWN_HARNESSES " in *" $h "*) : ;; *)
      echo "harness-defaults: unknown harness '$h' (known: $HD_KNOWN_HARNESSES)" >&2; rc=1; continue ;;
    esac
    # duplicate harness block
    if [ "$(hd_harnesses "$f" | grep -cx "$h")" -gt 1 ]; then
      echo "harness-defaults: duplicate harness block '$h'" >&2; rc=1
    fi
    while IFS= read -r line; do
      [ -n "$line" ] || continue
      a="$(printf '%s' "$line" | sed -e 's/^    //' -e 's/[[:space:]]*:.*//')"
      [ -f "$src/docket-$a.md" ] || {
        echo "harness-defaults: $h/$a names no wrapper source ($src/docket-$a.md)" >&2; rc=1; }
      # exactly the allowed fields
      for k in $(printf '%s' "$line" | sed -nE 's/.*\{(.*)\}.*/\1/p' | tr ',' '\n' | sed -nE 's/^[[:space:]]*([A-Za-z0-9._-]+)[[:space:]]*:.*/\1/p'); do
        case "$k" in
          model|effort) : ;;
          runner) echo "harness-defaults: $h/$a sets 'runner' — delegation is user policy, never a shipped default" >&2; rc=1 ;;
          *) echo "harness-defaults: $h/$a has unknown field '$k' (allowed: model, effort)" >&2; rc=1 ;;
        esac
      done
      for k in model effort; do
        v="$(hd_field "$f" "$h" "$a" "$k")"
        [ -n "$v" ] || { echo "harness-defaults: $h/$a is missing a non-empty '$k'" >&2; rc=1; }
      done
      # duplicate agent entry within the block
      if [ "$(hd_agents "$f" "$h" | grep -cx "$a")" -gt 1 ]; then
        echo "harness-defaults: duplicate entry '$a' under '$h'" >&2; rc=1
      fi
    done < <(_hd_block "$f" "$h")
  done
  # completeness: claude == every source wrapper, both directions
  for n in "$src"/docket-*.md; do
    [ -e "$n" ] || continue
    a="$(basename "$n" .md)"; a="${a#docket-}"
    [ -n "$(hd_field "$f" claude "$a" model)" ] || {
      echo "harness-defaults: claude block is incomplete — no entry for '$a'" >&2; rc=1; }
  done
  # completeness: cursor == every build worker, both directions
  for n in "$src"/docket-build-*.md; do
    [ -e "$n" ] || continue
    a="$(basename "$n" .md)"; a="${a#docket-}"
    [ -n "$(hd_field "$f" cursor "$a" model)" ] || {
      echo "harness-defaults: cursor block is incomplete — no entry for build profile '$a'" >&2; rc=1; }
  done
  for a in $(hd_agents "$f" cursor); do
    case "$a" in build-*) : ;; *)
      echo "harness-defaults: cursor/$a is not a build profile — change 0168 ships cursor defaults for the build workers only" >&2; rc=1 ;;
    esac
  done
  return $rc
}
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `bash tests/test_harness_defaults.sh`
Expected: PASS.

- [ ] **Step 6: Mutation-prove the two set-equality directions**

Both must be confirmed to *land* before being believed (`grep -c` before and after).

```bash
cp agents/harness-defaults.yml /tmp/hd.bak
# forward: delete a real Claude entry -> the reverse loop reddens
grep -c '^    status:' agents/harness-defaults.yml            # expect 1
sed -i.bak '/^    status:/d' agents/harness-defaults.yml
grep -c '^    status:' agents/harness-defaults.yml            # expect 0 — mutation landed
bash tests/test_harness_defaults.sh; echo "expect FAIL: $?"
cp /tmp/hd.bak agents/harness-defaults.yml
# reverse: add a phantom entry -> the forward loop reddens
printf '    phantom:               { model: x, effort: low }\n' >> agents/harness-defaults.yml
bash tests/test_harness_defaults.sh; echo "expect FAIL: $?"
cp /tmp/hd.bak agents/harness-defaults.yml
rm -f agents/harness-defaults.yml.bak
bash tests/test_harness_defaults.sh; echo "expect PASS: $?"
```

- [ ] **Step 7: Commit**

```bash
git add agents/harness-defaults.yml scripts/lib/harness-defaults.sh tests/test_harness_defaults.sh
git commit -m "feat(0168): ship the harness-indexed agent default sidecar + validator"
```

---

### Task 2: Resolve the shipped layer, with provenance

**Files:**
- Modify: `sync-agents.sh` (source the library; validate at startup; extend `resolve_agent_layers`)
- Test: `tests/test_sync_agents.sh`

**Interfaces:**
- Consumes: `hd_field`, `hd_validate` from Task 1.
- Produces, for Tasks 3 and 5: `resolve_agent_layers <harness> <agent> <files…>` continues to set `RES_MODEL`, `RES_EFFORT`, `RES_RUNNER`, `RES_MODEL_FROM_HARNESS`, and now additionally sets:
  - `RES_MODEL_FROM_USER` — `1` iff `RES_MODEL` came from a user layer (local/committed/global), `0` if it came from the shipped sidecar or is empty.
  - `RES_EFFORT_FROM_USER` — same for effort.
  After the call, `RES_MODEL`/`RES_EFFORT` are the FINAL values including the shipped floor. An empty value means "no pin — omit the field."

- [ ] **Step 1: Write the failing test**

Append to `tests/test_sync_agents.sh`, after the existing four-layer resolution asserts:

```bash
# ---- change 0168: the shipped sidecar is the lowest layer -------------------
mk_repo_cfg "$(cat <<'YML'
agent_harnesses: [claude]
agents:
  claude:
    adr: { effort: high }
YML
)"
run_sync
A="$SBX/repo/.claude/agents/docket-adr.md"
assert "0168: unconfigured agent takes the shipped claude model" \
  '[ "$(fm "$SBX/repo/.claude/agents/docket-status.md" model)" = "claude-haiku-4-5-20251001" ]'
assert "0168: a user effort override beats the shipped effort" '[ "$(fm "$A" effort)" = "high" ]'
assert "0168: the un-overridden field still comes from the sidecar" \
  '[ "$(fm "$A" model)" = "claude-opus-5" ]'
```

(`mk_repo_cfg` / `run_sync` are the file's existing sandbox helpers — reuse them verbatim rather than adding new ones.)

- [ ] **Step 2: Run to verify it fails**

Run: `bash tests/test_sync_agents.sh`
Expected: the three new asserts are already green *by accident* — the values still come from the source frontmatter. This is the [[assert-detects-removal-not-replacement]] trap: these asserts pin the outcome, not the mechanism. The mechanism is proven in Step 5 by pointing the resolver at a temporary sidecar with different values. Note this in the commit body; do not treat the accidental green as verification.

- [ ] **Step 3: Source the library and validate before any write**

In `sync-agents.sh`, beside the existing `docket-gitignore-block.sh` source line (~line 52):

```bash
# shellcheck source=/dev/null
. "$SCRIPT_DIR/scripts/lib/harness-defaults.sh"
HARNESS_DEFAULTS="$SCRIPT_DIR/agents/harness-defaults.yml"
```

Then, immediately before the first generation pass runs (before `user_level_pass`), add the fail-before-write gate:

```bash
# The sidecar is required program data (change 0168). Validate BEFORE writing any wrapper, so a
# malformed file cannot leave a half-regenerated agent directory behind.
if ! hd_validate "$HARNESS_DEFAULTS" "$AGENTS_SRC"; then
  log "ERROR agents/harness-defaults.yml is missing or invalid — no wrappers were written."
  exit 1
fi
```

- [ ] **Step 4: Add the shipped floor and provenance to the resolver**

Replace the body of `resolve_agent_layers` (keeping its comment header, with the "the built-in floor is handled by emit()" sentence corrected to name the sidecar):

```bash
resolve_agent_layers() {  # $1=harness  $2=agent  $3..=layer files (precedence order)
  local harness="$1" agent="$2" f hline dline hm he dm de hr dr
  shift 2
  RES_MODEL=""; RES_EFFORT=""; RES_RUNNER=""; RES_MODEL_FROM_HARNESS=0
  RES_MODEL_FROM_USER=0; RES_EFFORT_FROM_USER=0
  for f in "$@"; do
    hline="$(harness_agent_line "$f" "$harness" "$agent" 1)"
    dline="$(harness_agent_line "$f" default "$agent" 1)"
    hm="$(field_of "$hline" model)";  he="$(field_of "$hline" effort)"
    dm="$(field_of "$dline" model)";  de="$(field_of "$dline" effort)"
    hr="$(field_of "$hline" runner)"; dr="$(field_of "$dline" runner)"
    if [ -z "$RES_MODEL" ]; then
      if   [ -n "$hm" ]; then RES_MODEL="$hm"; RES_MODEL_FROM_HARNESS=1; RES_MODEL_FROM_USER=1
      elif [ -n "$dm" ]; then RES_MODEL="$dm"; RES_MODEL_FROM_USER=1; fi
    fi
    if [ -z "$RES_EFFORT" ]; then
      if   [ -n "$he" ]; then RES_EFFORT="$he"; RES_EFFORT_FROM_USER=1
      elif [ -n "$de" ]; then RES_EFFORT="$de"; RES_EFFORT_FROM_USER=1; fi
    fi
    if [ -z "$RES_RUNNER" ]; then
      if   [ -n "$hr" ]; then RES_RUNNER="$hr"
      elif [ -n "$dr" ]; then RES_RUNNER="$dr"; fi
    fi
  done
  # Shipped floor (change 0168): the sidecar is harness-indexed, so it can only supply a value for
  # the harness being generated. It never sets RES_*_FROM_USER — that split is what keeps a shipped
  # native default out of a delegated child-runner's flags (see emit_wrapper).
  [ -z "$RES_MODEL" ]  && RES_MODEL="$(hd_field "$HARNESS_DEFAULTS" "$harness" "$agent" model)"
  [ -z "$RES_EFFORT" ] && RES_EFFORT="$(hd_field "$HARNESS_DEFAULTS" "$harness" "$agent" effort)"
  return 0
}
```

Note `RES_MODEL_FROM_HARNESS` keeps its existing meaning — *a user layer's harness-specific line supplied it* — which is what `warn_fallback_model` needs. The sidecar deliberately does not set it; Task 5 re-words that warning around the new reality.

- [ ] **Step 5: Prove the mechanism, not just the outcome**

Point the resolver at a mutated sidecar and confirm the generated wrapper follows it:

```bash
cp agents/harness-defaults.yml /tmp/hd.bak
grep -c 'claude-haiku-4-5-20251001' agents/harness-defaults.yml   # expect 1
sed -i.bak 's/claude-haiku-4-5-20251001/claude-sentinel-9/' agents/harness-defaults.yml
grep -c 'claude-sentinel-9' agents/harness-defaults.yml            # expect 1 — mutation landed
# regenerate into a scratch harness root and read the status wrapper back
T="$(mktemp -d)"; mkdir -p "$T/repo"; printf 'agent_harnesses: [claude]\n' > "$T/repo/.docket.yml"
( cd "$T/repo" && DOCKET_HARNESS_ROOT="$T" bash "$OLDPWD/sync-agents.sh" >/dev/null 2>&1 )
grep '^model:' "$T/repo/.claude/agents/docket-status.md"           # expect claude-sentinel-9
cp /tmp/hd.bak agents/harness-defaults.yml; rm -f agents/harness-defaults.yml.bak; rm -rf "$T"
```

Expected: `model: claude-sentinel-9`. If it still prints the real ID, the resolver is not reaching the sidecar and Task 2 is not done.

- [ ] **Step 6: Run the suite legs touched so far**

Run: `bash tests/test_harness_defaults.sh && bash tests/test_sync_agents.sh`
Expected: both PASS. Generated output is unchanged because the sidecar mirrors the sources exactly.

- [ ] **Step 7: Commit**

```bash
git add sync-agents.sh tests/test_sync_agents.sh
git commit -m "feat(0168): resolve the shipped sidecar as the lowest agent layer, with provenance"
```

---

### Task 3: Emitters consume resolved values instead of reading the source

**Files:**
- Modify: `sync-agents.sh` — `emit()`, `emit_cursor_md()`, `emit_codex_toml()`
- Test: `tests/test_sync_agents_cursor.sh`, `tests/test_sync_agents_codex.sh`

**Interfaces:**
- Consumes: `RES_MODEL`/`RES_EFFORT` from Task 2 — now FINAL values, not overrides.
- Produces: all three emitters treat their `$2`/`$3` as the final resolved pair. Empty or `inherit` model ⇒ omit the model field. Empty or `auto` effort ⇒ omit the effort field. `emit()` becomes insertion-based and is idempotent whether or not the source still carries the lines — which is what lets Task 4 be a pure deletion.

- [ ] **Step 1: Write the failing test**

The critical property is that `emit()` must not emit two `model:` lines while the sources still carry theirs. Append to `tests/test_sync_agents.sh`:

```bash
# ---- change 0168: emit() inserts exactly one model:/effort: line ------------
mk_repo_cfg 'agent_harnesses: [claude]'
run_sync
for w in docket-status docket-implement-next docket-build-premium; do
  G="$SBX/repo/.claude/agents/$w.md"
  assert "0168: $w emits exactly one model: line" \
    '[ "$(grep -c "^model:" "$G")" = "1" ]'
  assert "0168: $w emits exactly one effort: line" \
    '[ "$(grep -c "^effort:" "$G")" = "1" ]'
done
```

- [ ] **Step 2: Run to verify it fails**

Run: `bash tests/test_sync_agents.sh`
Expected: PASS today (substitution cannot duplicate). It becomes the regression guard the moment Step 3 switches to insertion — run it again after Step 3 and confirm it is still green; a `2` there means the strip half of the rewrite is missing.

- [ ] **Step 3: Rewrite `emit()` as strip-then-insert**

Replace `emit()` and its comment:

```bash
# --- emit a resolved wrapper to stdout ---------------------------------------
# Model/effort are the FINAL resolved values (change 0168): the shipped sidecar, not the source
# frontmatter, is the default store. This strips any model:/effort: line the source still carries
# and inserts the resolved pair, so it is idempotent across the source-cleanup in the same change
# and can never emit a duplicated key. An empty/`inherit` model or an empty/`auto` effort omits its
# field entirely — the harness then applies its own default.
emit() {  # $1=src file  $2=model  $3=effort
  local m="$2" e="$3"
  [ "$m" = "inherit" ] && m=""
  [ "$e" = "auto" ] && e=""
  awk -v model="$m" -v effort="$e" '
    /^---[[:space:]]*$/ {
      d++
      if (d==1) { print; infm=1; next }
      if (d==2 && infm) {                      # closing fence: insert before it
        if (model!="")  print "model: " model
        if (effort!="") print "effort: " effort
        infm=0; print; next
      }
      print; next
    }
    infm && $0 ~ /^model[[:space:]]*:/  { next }   # drop any source-carried pin
    infm && $0 ~ /^effort[[:space:]]*:/ { next }
    { print }
  ' "$1"
}
```

This moves `model:`/`effort:` to the end of the frontmatter block. That is a real byte change to the generated Claude wrappers (field order), which the byte-identity asserts in Task 6 already have to be rewritten for. Field order carries no meaning in YAML frontmatter and no consumer depends on it.

- [ ] **Step 4: Strip the source reads from the Cursor and Codex emitters**

In `emit_cursor_md()`, delete the two `bi_*` lines and the `${mo:-$bi_*}` fallbacks; replace with direct assignment:

```bash
  local name desc model effort skills_csv body
  name="$(sed -n '/^name:/{s/^name:[[:space:]]*//;p;q;}' "$src")"
  [ -n "$name" ] || name="docket-$(short_name "$src")"
  desc="$(agent_description "$src")"
  model="$mo"      # change 0168: FINAL resolved value; the source no longer carries a default
  effort="$eo"
```

Keep the existing `inherit`/`auto` normalization and every emit branch below it unchanged. Update the function's comment: `effective model + effort` becomes `resolved model + effort (shipped sidecar ⊕ user layers)`, and drop `(override||built-in)`.

Apply the identical edit to `emit_codex_toml()`: delete `bi_model`/`bi_effort` and set `model="$mo"`, `effort="$eo"`. Its emit guards (`!= "inherit"`, `!= "auto"`) already handle the omit cases.

- [ ] **Step 5: Verify generated output is unchanged apart from field order**

```bash
T="$(mktemp -d)"; mkdir -p "$T/a/repo" "$T/b/repo"
printf 'agent_harnesses: [claude, cursor, codex]\n' | tee "$T/a/repo/.docket.yml" > "$T/b/repo/.docket.yml"
git stash                                          # pre-change tree
( cd "$T/a/repo" && DOCKET_HARNESS_ROOT="$T/a" bash "$OLDPWD/sync-agents.sh" >/dev/null 2>&1 )
git stash pop                                      # post-change tree
( cd "$T/b/repo" && DOCKET_HARNESS_ROOT="$T/b" bash "$OLDPWD/sync-agents.sh" >/dev/null 2>&1 )
# Cursor + Codex wrappers must be byte-identical; Claude differs only in frontmatter field ORDER.
diff -r "$T/a/repo/.cursor" "$T/b/repo/.cursor" && echo "cursor identical"
diff -r "$T/a/repo/.codex"  "$T/b/repo/.codex"  && echo "codex identical"
for f in "$T/a/repo/.claude/agents/"*.md; do
  n="$(basename "$f")"
  diff <(sort "$f") <(sort "$T/b/repo/.claude/agents/$n") || echo "REAL DIFF in $n"
done
rm -rf "$T"
```

Expected: `cursor identical`, `codex identical`, and no `REAL DIFF` line. The Cursor wrappers here still carry Claude IDs for all twelve agents — that is correct at this point, because the Cursor sidecar block only covers the three build workers and the Cursor build wrappers now pick up their own IDs. Confirm exactly that: `grep '^model:' "$T/b/repo/.cursor/agents/docket-build-standard.md"` should already read `cursor-grok-4.5-high`.

- [ ] **Step 6: Run the touched suites**

Run: `bash tests/test_sync_agents.sh; bash tests/test_sync_agents_cursor.sh; bash tests/test_sync_agents_codex.sh`
Expected: `test_sync_agents_cursor.sh` and `test_sync_agents_codex.sh` now FAIL on their byte-identity and bracket-encoded-effort asserts. That failure is expected and is repaired in Task 6 — record which asserts failed, verbatim, in the commit body so Task 6 has the list.

- [ ] **Step 7: Commit**

```bash
git add sync-agents.sh tests/test_sync_agents.sh
git commit -m "refactor(0168): emitters consume resolved model/effort, not source frontmatter"
```

---

### Task 4: Delete the defaults from the twelve wrapper sources

**Files:**
- Modify: `agents/docket-adr.md`, `docket-auto-groom.md`, `docket-auto-groom-critic.md`, `docket-brainstorm-consultant.md`, `docket-build-economy.md`, `docket-build-premium.md`, `docket-build-standard.md`, `docket-finalize-change.md`, `docket-implement-next.md`, `docket-integration-repair.md`, `docket-rebase-resolver.md`, `docket-status.md`
- Test: `tests/test_harness_defaults.sh`

**Interfaces:**
- Consumes: Task 3's insertion-based emitters (without them this task breaks every generated pin).
- Produces: sources are behavior-only templates carrying `name`, `description`, optional `skills:`, and the body.

- [ ] **Step 1: Write the failing negative guard**

This is the [[assert-detects-removal-not-replacement]] shape — assert the removed state is *absent*, anchored on the frontmatter block, not a whole-file grep. Append to `tests/test_harness_defaults.sh` before the final `[ "$fail" = 0 ]` line:

```bash
# ---- the sources are behavior-only templates --------------------------------
# Anchored to the first frontmatter block: these files' BODIES legitimately discuss model/effort.
fm_has(){ # $1=file $2=key -> 0 if the key appears in the first --- block
  awk -v k="$2" '
    /^---[[:space:]]*$/ { d++; if (d>=2) exit 1; next }
    d==1 && $0 ~ "^"k"[[:space:]]*:" { exit 0 }
    END { exit 1 }' "$1"
}
for f in "$SRC"/docket-*.md; do
  n="$(basename "$f")"
  assert "$n: no model: in frontmatter (sidecar owns it)"  '! fm_has "'"$f"'" model'
  assert "$n: no effort: in frontmatter (sidecar owns it)" '! fm_has "'"$f"'" effort'
  assert "$n: still declares name:"                        'fm_has "'"$f"'" name'
done
```

- [ ] **Step 2: Run to verify it fails**

Run: `bash tests/test_harness_defaults.sh`
Expected: FAIL — 24 asserts red, one `model:` and one `effort:` per source.

- [ ] **Step 3: Delete the two lines from each source**

Each file's frontmatter is `---` / `name:` / `description:` / `model:` / `effort:` / optional `skills:` / `---`. Remove only lines 4–5 of the frontmatter block:

```bash
for f in agents/docket-*.md; do
  before="$(grep -c '^\(model\|effort\):' "$f")"
  [ "$before" = "2" ] || { echo "UNEXPECTED shape in $f ($before)"; continue; }
  sed -i.bak '/^model:/d; /^effort:/d' "$f" && rm -f "$f.bak"
  after="$(grep -c '^\(model\|effort\):' "$f")"
  echo "$f: $before -> $after"     # every line must read '2 -> 0'
done
```

Confirm all twelve print `2 -> 0`. A file printing anything else did not have the expected shape and must be inspected by hand — do not proceed past a mismatch.

- [ ] **Step 4: Run to verify it passes**

Run: `bash tests/test_harness_defaults.sh`
Expected: PASS.

- [ ] **Step 5: Confirm generated pins survived the deletion**

The whole risk of this task is a silently unpinned wrapper.

```bash
T="$(mktemp -d)"; mkdir -p "$T/repo"
printf 'agent_harnesses: [claude, cursor, codex]\n' > "$T/repo/.docket.yml"
( cd "$T/repo" && DOCKET_HARNESS_ROOT="$T" bash "$OLDPWD/sync-agents.sh" >/dev/null 2>&1 )
grep -h '^model:'  "$T/repo/.claude/agents/"*.md | sort | uniq -c   # 11 claude-opus-5, 1 haiku
grep -h '^effort:' "$T/repo/.claude/agents/"*.md | sort | uniq -c   # 4 low, 7 medium, 1 high
grep '^model:' "$T/repo/.cursor/agents/docket-build-economy.md"     # cursor-grok-4.5-medium
grep '^model:' "$T/repo/.cursor/agents/docket-build-standard.md"    # cursor-grok-4.5-high
grep '^model:' "$T/repo/.cursor/agents/docket-build-premium.md"     # claude-opus-5-high
grep -c '^model:' "$T/repo/.cursor/agents/docket-status.md"         # 0 — honestly unpinned
grep -c '^model' "$T/repo/.codex/agents/docket-status.toml"         # 0 — honestly unpinned
rm -rf "$T"
```

Expected: twelve Claude wrappers pinned exactly as before; the three Cursor build wrappers carrying their own IDs with **no** `[effort=…]` suffix; every other Cursor and every Codex wrapper carrying no model at all rather than a Claude ID.

- [ ] **Step 6: Commit**

```bash
git add agents/
git commit -m "refactor(0168): wrapper sources become behavior-only templates"
```

---

### Task 5: The runner-delegation provenance boundary

**Files:**
- Modify: `sync-agents.sh` — `emit_wrapper()`, `emit_shim()`, `warn_fallback_model()`
- Test: `tests/test_sync_agents.sh`

**Interfaces:**
- Consumes: `RES_MODEL_FROM_USER` / `RES_EFFORT_FROM_USER` from Task 2.
- Produces: `emit_shim` receives *user-layer-only* model/effort for its baked `--model`/`--effort` flags, while its native frontmatter still carries the fully resolved pair.

- [ ] **Step 1: Write the failing test**

Append to `tests/test_sync_agents.sh`:

```bash
# ---- change 0168: a shipped default never becomes a child-runner flag -------
mk_repo_cfg "$(cat <<'YML'
agent_harnesses: [claude]
agents:
  claude:
    status: { runner: codex }
YML
)"
run_sync
S="$SBX/repo/.claude/agents/docket-status.md"
assert "0168: runner-only shim bakes NO --model flag" '! grep -q -- "--model" "$S"'
assert "0168: runner-only shim bakes NO --effort flag" '! grep -q -- "--effort" "$S"'
assert "0168: runner-only shim still names the runner" 'grep -q -- "--runner codex" "$S"'
assert "0168: runner-only shim frontmatter still carries the native pin" \
  '[ "$(fm "$S" model)" = "claude-haiku-4-5-20251001" ]'

mk_repo_cfg "$(cat <<'YML'
agent_harnesses: [claude]
agents:
  claude:
    status: { runner: codex, model: gpt-5.5, effort: high }
YML
)"
run_sync
S="$SBX/repo/.claude/agents/docket-status.md"
assert "0168: an explicit override still passes through to the child" \
  'grep -q -- "--model gpt-5.5" "$S" && grep -q -- "--effort high" "$S"'
```

- [ ] **Step 2: Run to verify it fails**

Run: `bash tests/test_sync_agents.sh`
Expected: FAIL — the first two asserts. Today `RES_MODEL` is non-empty for `status` (the sidecar supplies `claude-haiku-4-5-20251001`), so the shim bakes `--model claude-haiku-4-5-20251001` and sends a Claude ID to a Codex child. This is the concrete defect the provenance split exists to prevent.

- [ ] **Step 3: Pass user-layer-only values to the shim**

In `emit_wrapper()`, change the shim call so the flags come from user-layer values while the frontmatter keeps the resolved pair. Add two locals and extend `emit_shim`'s signature:

```bash
emit_wrapper(){  # $1=src $2=model $3=effort $4=runner $5=harness $6=agent-name  (stdout)
  local runner="$4"
  if [ -z "$runner" ]; then emit_for_harness "$1" "$5" "$2" "$3"; return 0; fi
  if [ "$5" != "claude" ]; then
    log "WARN $5/docket-$6: runner: $runner is reserved for the claude parent — ignored (native dispatch)"
    emit_for_harness "$1" "$5" "$2" "$3"; return 0
  fi
  if ! is_registered_runner "$runner"; then
    log "ERROR docket-$6: runner '$runner' is not a registered runner (registered: $REGISTERED_RUNNERS)"
    exit 1
  fi
  # change 0168: ONLY a user-configured value may become a child-runner flag. A shipped native
  # default configures this Claude wrapper; it is not evidence that the same ID means anything to
  # a Codex or Cursor child. A runner-only override therefore lets the child pick its own default.
  local flag_model="" flag_effort=""
  [ "${RES_MODEL_FROM_USER:-0}" = "1" ]  && flag_model="$2"
  [ "${RES_EFFORT_FROM_USER:-0}" = "1" ] && flag_effort="$3"
  emit_shim "$1" "$2" "$3" "$runner" "$6" "$flag_model" "$flag_effort"
}
```

And in `emit_shim`, build the flags from the new arguments rather than from `$2`/`$3`:

```bash
emit_shim(){  # $1=src $2=model $3=effort $4=runner $5=agent-name $6=flag-model $7=flag-effort
  emit "$1" "$2" "$3" | awk '/^---[[:space:]]*$/{d++; print; next} d<2{print}'
  local flags="--runner $4 --agent $5"
  [ -n "${6:-}" ] && flags="$flags --model $6"
  [ -n "${7:-}" ] && [ "${7:-}" != "auto" ] && flags="$flags --effort $7"
```

Leave the heredoc body unchanged. Update the function's comment: the native frontmatter carries the resolved pin, while the baked flags carry only user-configured values.

- [ ] **Step 4: Re-word the fallback warning**

`warn_fallback_model()`'s premise changed: a non-Claude harness with no sidecar entry now resolves to *no model*, not to a leaked Claude ID.

```bash
# Non-fatal footgun warning: a NON-claude harness wrapper with no harness-specific value — neither a
# shipped agents/harness-defaults.yml entry nor an agents.<harness> override — is generated UNPINNED
# (change 0168) or, if only agents.default supplied a model, carries an ID that may be meaningless to
# that harness (ADR-0015: some harnesses silently run their house default on an unknown model).
# Never an error; sync still succeeds. Scoped to non-claude — the claude sidecar values ARE Claude IDs.
warn_fallback_model(){  # $1=harness $2=agent ; consumes RES_MODEL_FROM_HARNESS / RES_MODEL
  [ "$1" = "claude" ] && return 0
  [ "$RES_MODEL_FROM_HARNESS" = "1" ] && return 0
  if [ -n "$(hd_field "$HARNESS_DEFAULTS" "$1" "$2" model)" ]; then return 0; fi
  if [ -z "$RES_MODEL" ]; then
    log "WARN $1/docket-$2: no harness-specific model — generated unpinned; harness '$1' will apply its own default. Set agents.$1.$2.model to pin it."
  else
    log "WARN $1/docket-$2: model '$RES_MODEL' came from agents.default; may not be a valid model ID for harness '$1'."
  fi
}
```

Scope note: this keeps the warning honest under the new resolution order and stops there. Making the unmapped-harness gap *loud* — an error, a summary, a `--check` leg — is change 0142's job; do not build it here.

- [ ] **Step 5: Run to verify it passes**

Run: `bash tests/test_sync_agents.sh`
Expected: PASS, including the explicit-override passthrough assert.

- [ ] **Step 6: Mutation-prove the provenance split**

```bash
grep -c 'RES_MODEL_FROM_USER:-0' sync-agents.sh     # expect 1
sed -i.bak 's/\[ "${RES_MODEL_FROM_USER:-0}" = "1" \]  \&\& flag_model="$2"/flag_model="$2"/' sync-agents.sh
grep -c 'RES_MODEL_FROM_USER:-0' sync-agents.sh     # expect 0 — mutation landed
bash tests/test_sync_agents.sh; echo "expect FAIL: $?"
mv sync-agents.sh.bak sync-agents.sh
bash tests/test_sync_agents.sh; echo "expect PASS: $?"
```

- [ ] **Step 7: Commit**

```bash
git add sync-agents.sh tests/test_sync_agents.sh
git commit -m "fix(0168): keep shipped native defaults out of child-runner flags"
```

---

### Task 6: Repair and re-point the tests whose premise the split deleted

**Files:**
- Modify: `tests/test_sync_agents.sh`, `tests/test_sync_agents_cursor.sh`, `tests/test_sync_agents_codex.sh`, `tests/test_docket_example_yml.sh`, `tests/test_docket_build.sh`, `tests/test_composition_wiring.sh`

**Interfaces:**
- Consumes: everything from Tasks 1–5.
- Produces: a green suite whose assertions name the sidecar as the default store.

Apply [[test-premise-deleted-not-regated]] throughout: ask what each block *guards*. A block guarding the emitter split (a real mechanism) is **kept and inverted**; a block guarding only "the source frontmatter holds the default" is **re-pointed at the sidecar**, never re-gated to green.

Also note the [[frontmatter-anchored-read]] hazard: `fm()` in these files is a first-match-**anywhere** read. Now that `model:` is absent from the sources, `fm "$AGENTS/docket-status.md" model` scans past the frontmatter into the body and can return prose. Every remaining `fm()` call against an `agents/` **source** for `model`/`effort` must be deleted or re-pointed — not merely expected to return empty.

- [ ] **Step 1: Re-point the source-default asserts in `tests/test_sync_agents.sh`**

Delete the two per-agent loop asserts that read model/effort off the source (`"$w: model is a known alias or full id"`, `"$w: effort in allowed set"`) and the five `built-in = …` asserts below the loop. Replace them with sidecar-anchored equivalents:

```bash
# Shipped defaults live in agents/harness-defaults.yml (change 0168), not the wrapper sources.
. "$REPO/scripts/lib/harness-defaults.sh"
HD="$REPO/agents/harness-defaults.yml"
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
```

Do the same for the four other source-reading sites — the critic (`~:326`), the rebase-resolver/integration-repair loop (`~:360`), and the consultant (`~:375`): re-point each `fm "$AGENTS/…" model|effort` at `hd_field "$HD" claude <short-name> model|effort`.

The generated-wrapper asserts (`auto keeps the built-in model`, the `0048` fallthrough asserts at `~:131/:149/:449/:670/:750`) read **generated** files and their expected values are unchanged — leave them alone, but update the word "built-in" in their labels to "shipped" so the vocabulary matches the new source of truth.

Finally, correct the doc-guard comment at `~:636` which asserts in prose that "built-in defaults live only in `agents/docket-*.md`" — re-point it at `agents/harness-defaults.yml`. The assert itself (README must not hardcode a model literal) still holds.

- [ ] **Step 2: Rewrite the two byte-identity asserts**

These guard a real mechanism — *the emitter split does not corrupt the Claude side* — so invert rather than delete. In `tests/test_sync_agents_cursor.sh` replace:

```bash
assert "cursor split: claude wrapper still byte-identical to its source" \
  'diff -q "$REPO/agents/docket-status.md" "$SBX/.claude/agents/docket-status.md" >/dev/null'
```

with a body-and-metadata equivalence check plus an explicit pin check, since byte identity is now structurally impossible (the source no longer carries the pin, the generator injects it):

```bash
# The claude wrapper is no longer byte-identical to its source: the source is a behavior-only
# template and the generator injects the pin from agents/harness-defaults.yml (change 0168).
# What must still hold is that the emitter split changed NOTHING except that injection.
body_of(){ awk '/^---[[:space:]]*$/ && d<2 {d++; next} d>=2 {print}' "$1"; }
assert "cursor split: claude wrapper body is verbatim from its source" \
  'diff -q <(body_of "$REPO/agents/docket-status.md") <(body_of "$SBX/.claude/agents/docket-status.md") >/dev/null'
assert "cursor split: claude wrapper name/description/skills come from the source" \
  '[ "$(fm "$SBX/.claude/agents/docket-status.md" name)" = "docket-status" ] &&
   [ "$(fm "$SBX/.claude/agents/docket-status.md" description)" = "$(fm "$REPO/agents/docket-status.md" description)" ] &&
   [ "$(fm "$SBX/.claude/agents/docket-status.md" skills)" = "$(fm "$REPO/agents/docket-status.md" skills)" ]'
assert "cursor split: claude wrapper carries the SHIPPED pin, injected not copied" \
  '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "claude-haiku-4-5-20251001" ] &&
   ! grep -q "^model:" "$REPO/agents/docket-status.md"'
```

The final assert is the important one: it pins both halves — the generated file *has* the pin and the source *does not* — so restoring a source default reddens it.

Apply the same three-assert replacement to `tests/test_sync_agents_codex.sh` at its `claude side still .md, byte-identical to source` assert, keeping its existing TOML asserts (`model = claude-haiku-4-5-20251001`, `model_reasoning_effort = medium`) unchanged — those read generated output and their values are unchanged.

- [ ] **Step 3: Fix the Cursor bracket-encoding expectation and add the build-profile asserts**

`tests/test_sync_agents_cursor.sh:47` expects `docket-status`'s Cursor wrapper to carry `claude-haiku-4-5-20251001[effort=medium]`. Under the sidecar, `status` has no Cursor entry, so that wrapper is now honestly unpinned. Re-point the bracket-encoding assert at a case that still exercises encoding — a user-configured Cursor override — and add the three shipped build asserts:

```bash
assert "cursor: unmapped agent is honestly unpinned (no claude ID leak)" \
  '! has_fm_key "$C" model'
assert "cursor: build-economy carries its shipped Cursor ID, no effort suffix" \
  '[ "$(fm "$SBX/.cursor/agents/docket-build-economy.md" model)" = "cursor-grok-4.5-medium" ]'
assert "cursor: build-standard carries its shipped Cursor ID, no effort suffix" \
  '[ "$(fm "$SBX/.cursor/agents/docket-build-standard.md" model)" = "cursor-grok-4.5-high" ]'
assert "cursor: build-premium carries its shipped Cursor ID, no effort suffix" \
  '[ "$(fm "$SBX/.cursor/agents/docket-build-premium.md" model)" = "claude-opus-5-high" ]'
for p in economy standard premium; do
  assert "cursor: build-$p emits no standalone effort: key" \
    '! has_fm_key "$SBX/.cursor/agents/docket-build-'"$p"'.md" effort'
  assert "cursor: build-$p model has no appended [effort=…] suffix" \
    '! grep -q "\[effort=" "$SBX/.cursor/agents/docket-build-'"$p"'.md"'
done
```

The bracket-encoding path itself is still covered by the file's existing override test (`~:63–65`), which supplies an explicit model *and* effort — verify that assert is present and green before deleting the old `:47` expectation; if it is not, add one rather than losing the coverage.

- [ ] **Step 4: Re-point the example mirror-equality loop**

In `tests/test_docket_example_yml.sh` (~:848–867), the loop compares `fm()` over `$REPO/agents/docket-$a.md` against the commented example block. Replace the `fm()` side with `hd_field`:

```bash
. "$REPO/scripts/lib/harness-defaults.sh"
HD="$REPO/agents/harness-defaults.yml"
for a in $EXAMPLE_AGENTS; do
  assert "$a: model mirrors the shipped sidecar" \
    '[ "$(ex_field claude "'"$a"'" model)" = "$(hd_field "$HD" claude "'"$a"'" model)" ]'
  assert "$a: effort mirrors the shipped sidecar" \
    '[ "$(ex_field claude "'"$a"'" effort)" = "$(hd_field "$HD" claude "'"$a"'" effort)" ]'
done
```

Apply the same substitution at the round-trip assert (~:915, "claude status model mirrors the built-in").

- [ ] **Step 5: Re-point the build-profile asserts**

In `tests/test_docket_build.sh` (~:357–380), the three `profile $name: effort is low|medium|high`, `profile $name: model is set`, `three DISTINCT efforts`, and `three profiles share one model` asserts all read source frontmatter. Re-point them at `hd_field "$HD" claude build-<profile> …`. Keep the invariants themselves intact — three distinct efforts, one shared model — but scope the shared-model invariant to the **claude** block explicitly, since the Cursor block deliberately uses three different models:

```bash
assert "claude build profiles share one model" \
  '[ "$(for p in economy standard premium; do hd_field "$HD" claude build-$p model; done | sort -u | wc -l | tr -d " ")" = "1" ]'
assert "cursor build profiles use three DISTINCT models" \
  '[ "$(for p in economy standard premium; do hd_field "$HD" cursor build-$p model; done | sort -u | wc -l | tr -d " ")" = "3" ]'
```

- [ ] **Step 6: Correct the stale comment in `tests/test_composition_wiring.sh`**

At `~:51`, the comment claims model/effort lives in "the wrapper frontmatter + layered config (the single source of truth, ADR-0008)". Re-point it at `agents/harness-defaults.yml` ⊕ the layered config. The negative guards below it (dispatch prose pins no literal tier) are unaffected.

- [ ] **Step 7: Run every touched suite**

Run:

```bash
for t in test_harness_defaults test_sync_agents test_sync_agents_cursor test_sync_agents_codex \
         test_docket_example_yml test_docket_build test_composition_wiring; do
  echo "== $t"; bash "tests/$t.sh" | tail -3
done
```

Expected: PASS on all seven.

- [ ] **Step 8: Commit**

```bash
git add tests/
git commit -m "test(0168): re-point default assertions at the sidecar; rewrite the byte-identity guards"
```

---

### Task 7: `.docket.example.yml` — re-point the mirror, distinguish shipped from illustrative

**Files:**
- Modify: `.docket.example.yml`
- Test: `tests/test_docket_example_yml.sh`

**Interfaces:**
- Consumes: Task 6's re-pointed mirror loop.
- Produces: an example file whose prose names `agents/harness-defaults.yml` as the shipped default store, and whose `cursor:` block separates the three validated build defaults from unvalidated illustrations.

- [ ] **Step 1: Update the resolution-order prose**

At `~:275`, replace the chain `agents.<harness>.<agent> -> agents.default.<agent> -> shipped built-in (agents/docket-*.md)` with `… -> shipped built-in (agents/harness-defaults.yml) -> unpinned (the harness applies its own default)`. At `~:13`, the layer legend line `4. built-in      docket's defaults — the values in this file` becomes `4. built-in      docket's shipped defaults — agents/harness-defaults.yml (mirrored below)`.

- [ ] **Step 2: Re-point the mirror-rule comment**

At `~:297–300`, the block asserting "The `claude:` block below MIRRORS docket's shipped built-in defaults (the values in each `agents/docket-*.md` wrapper) … Per ADR-0039 the wrappers are the single source of truth" must name the sidecar and the superseding ADR instead:

```
  # The `claude:` block below MIRRORS docket's shipped built-in defaults — the values in
  # agents/harness-defaults.yml — shown so the otherwise-invisible defaults are visible and
  # tunable. Deleting any line falls back to the SAME shipped default. The sidecar is the single
  # source of truth; this mirror never leads, and tests/test_docket_example_yml.sh enforces the
  # equality. If a shipped default changes, update this mirror to match.
```

Leave the twelve mirrored rows' values untouched — they are still correct.

- [ ] **Step 3: Separate shipped from illustrative in the `cursor:` block**

The Cursor block currently carries commented `build-*` rows that were unvalidated examples. Mark the three shipped ones and keep the rest visibly illustrative:

```
  # cursor: docket SHIPS validated defaults for the three build-profile workers only
  # (change 0168) — the rows below mirror agents/harness-defaults.yml and are what you get
  # with no configuration at all:
  #   build-economy:  { model: cursor-grok-4.5-medium }
  #   build-standard: { model: cursor-grok-4.5-high }
  #   build-premium:  { model: claude-opus-5-high }
  # Cursor's other nine agents ship UNPINNED: Cursor applies its own default model unless you
  # set one here. Any cursor row for a non-build agent is an unvalidated illustration.
```

- [ ] **Step 4: Run the example suite**

Run: `bash tests/test_docket_example_yml.sh`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add .docket.example.yml
git commit -m "docs(0168): example config mirrors the sidecar; mark shipped vs illustrative cursor rows"
```

---

### Task 8: README skill catalog + the falsified shipped-model claim + reference docs

**Files:**
- Modify: `README.md`, `skills/docket-convention/references/agent-layer.md`, `skills/docket-convention/SKILL.md`, `skills/docket-build/SKILL.md`, `docs/cursor/validation.md`, `docs/codex/setup.md`, `scripts/runners/cursor.md`, `scripts/runners/codex.md`, `scripts/runner-dispatch.md`
- Test: `tests/test_readme_skill_catalog.sh` (create)

**Interfaces:**
- Consumes: nothing from earlier tasks (documentation only).
- Produces: a count-free `## Skills` catalog guarded bidirectionally against `skills/*/SKILL.md`.

- [ ] **Step 1: Write the failing bidirectional catalog guard**

Create `tests/test_readme_skill_catalog.sh`:

```bash
#!/usr/bin/env bash
# tests/test_readme_skill_catalog.sh — run: bash tests/test_readme_skill_catalog.sh
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

README="$REPO/README.md"

# The heading carries NO count (change 0168): a counted heading needs an edit per new skill and
# teaches the next author to bump the number instead of adding the row ([[guard-remedy-must-not-teach-the-evasion]]).
assert "catalog heading is count-free '## Skills'" 'grep -qx "## Skills" "$README"'
assert "no counted skills heading survives" \
  '! grep -qiE "^## The [a-z]+ skills" "$README"'
assert "no stale anchor link to a counted heading" \
  '! grep -qF "#the-eight-skills" "$README"'

# The rows named in the catalog section.
section="$(awk '/^## Skills[[:space:]]*$/{f=1;next} f&&/^## /{exit} f{print}' "$README")"
listed="$(printf '%s\n' "$section" | sed -nE 's/^\|[[:space:]]*`?(docket-[a-z-]+)`?[[:space:]]*\|.*/\1/p' | sort -u)"
live="$(cd "$REPO/skills" && for d in */SKILL.md; do echo "${d%/SKILL.md}"; done | sort -u)"

# forward: every live skill package is documented
while IFS= read -r s; do
  [ -n "$s" ] || continue
  assert "skills/$s is listed in the README catalog" 'grep -qx "'"$s"'" <<<"$listed"'
done <<<"$live"
# reverse: every listed row names a real skill package (anchored on the dirs, not an allowlist)
while IFS= read -r s; do
  [ -n "$s" ] || continue
  assert "catalog row $s names a real skills/ package" '[ -f "$REPO/skills/'"$s"'/SKILL.md" ]'
done <<<"$listed"

[ "$fail" = 0 ] && echo "PASS" || echo "FAIL"
exit "$fail"
```

- [ ] **Step 2: Run to verify it fails**

Run: `bash tests/test_readme_skill_catalog.sh`
Expected: FAIL — heading is `## The eight skills`, the anchor still exists, and `docket-brainstorm`, `docket-build`, `docket-build-task` are absent from the table.

- [ ] **Step 3: Rewrite the catalog**

In `README.md`: change the heading at `:635` to `## Skills`; change the TOC entry at `:25` to `- [Skills](#skills)`; rewrite the lead sentence at `:637` to drop the count (e.g. "The operating loop — create, groom, implement, finalize, report, decide — plus the shared contract every skill loads, and the pluggable role skills docket ships."). Add three rows to the table for `docket-brainstorm`, `docket-build`, and `docket-build-task`, then delete the reconciling footnote at `:697` entirely — its whole content was the arithmetic the count-free heading removes. Check for any other in-repo link to `#the-eight-skills` before finishing (`grep -rn 'the-eight-skills' --exclude-dir=.git .`).

- [ ] **Step 4: Rewrite the falsified shipped-model paragraph**

`README.md:720` claims "the only model IDs docket actually ships are the Claude ones under `agents.claude`" and promises that holds "until changes 0168 and 0169 land validated mappings". Rewrite to the post-change truth: docket ships validated Claude defaults for all twelve agents and validated Cursor defaults for the three build-profile workers, all from `agents/harness-defaults.yml`; Codex remains user-configured until change 0169; Cursor's other nine agents ship unpinned. Also correct `:145` ("the example's commented `agents.claude` block mirrors the shipped defaults … instead of opening twelve wrapper files") to name the sidecar as the one place to read them, and `:611`.

- [ ] **Step 5: Re-point the agent-layer reference and convention**

In `skills/docket-convention/references/agent-layer.md`: the layer table row at `:16` becomes `| Built-in | `agents/harness-defaults.yml` shipped in docket (harness-indexed; claude complete, cursor build-profiles only) | — |`; the resolution chain at `:40` gains the sidecar in place of `agents/docket-*.md`; `:47` and `:55`'s fallback prose are updated to say an unmapped pair ships unpinned rather than inheriting a foreign ID; `:66`'s "defaults to `model: inherit`" is corrected to "omits the field". Add one sentence naming the sidecar's rules (concrete harnesses only, both fields per entry, no `runner`).

In `skills/docket-convention/SKILL.md:102–106`, the sentence "A wrapper is a thin generated file: it pins `model` + `effort` … no entry in any layer ⇒ `model: inherit`, no `effort`" is corrected: values resolve from the layered config over `agents/harness-defaults.yml`, and a pair with no entry anywhere is generated unpinned. Keep the twelve-wrapper counts — they are unchanged and separately guarded by `tests/test_finalize_gate.sh:140`.

- [ ] **Step 6: Re-point the remaining prose sites**

- `skills/docket-build/SKILL.md` — the layer story should name the sidecar; the existing "never restate literal model IDs" rule stays.
- `docs/cursor/validation.md` — the wrapper-inspection checklist must now expect the three build wrappers pinned to their Cursor IDs and the other nine unpinned; add the Cursor IDE certification checklist items from the spec (explicit economy/standard/premium routing, one auto-classified task, one bounded escalation).
- `docs/codex/setup.md` — note that Codex wrappers ship unpinned until change 0169.
- `scripts/runners/cursor.md`, `scripts/runners/codex.md`, `scripts/runner-dispatch.md` — their "the built-in agent … its wrapper source `agents/docket-<name>.md` supplies the skills list and body" statements stay true (they read name/description/skills/body, never model), but add one clause noting model/effort now arrive as flags resolved from the sidecar plus user layers, and that a shipped default is never forwarded.

Derive the full site list from a whole-repo grep rather than this list alone (AGENTS.md: never hand-list the sites), excluding archived changes, specs, plans, results, and Accepted ADRs, which keep their point-in-time statements:

```bash
grep -rn 'agents/docket-\*\.md' --exclude-dir=.git --exclude-dir=.worktrees --exclude-dir=.docket . \
  | grep -v -e '^\./docs/adrs/' -e '^\./docs/superpowers/' -e '^\./docs/results/'
```

- [ ] **Step 7: Run the doc suites**

Run: `bash tests/test_readme_skill_catalog.sh && bash tests/test_cursor_contract_docs.sh && bash tests/test_docket_example_yml.sh && bash tests/test_finalize_gate.sh`
Expected: PASS.

- [ ] **Step 8: Mutation-prove the catalog guard in both directions**

```bash
cp README.md /tmp/rm.bak
grep -c '| `docket-build-task`' README.md            # expect 1
sed -i.bak '/| `docket-build-task`/d' README.md
grep -c '| `docket-build-task`' README.md            # expect 0 — mutation landed
bash tests/test_readme_skill_catalog.sh; echo "expect FAIL (forward): $?"
cp /tmp/rm.bak README.md
# reverse: a row naming no package
sed -i.bak 's/| `docket-build-task`/| `docket-phantom`/' README.md
bash tests/test_readme_skill_catalog.sh; echo "expect FAIL (reverse): $?"
cp /tmp/rm.bak README.md; rm -f README.md.bak
bash tests/test_readme_skill_catalog.sh; echo "expect PASS: $?"
```

- [ ] **Step 9: Commit**

```bash
git add README.md skills/ docs/ scripts/ tests/test_readme_skill_catalog.sh
git commit -m "docs(0168): count-free skill catalog, sidecar-accurate agent-layer prose"
```

---

### Task 9: Full-suite gate

**Files:**
- Modify: none expected. Any file the suite reddens.

- [ ] **Step 1: Run the entire suite**

Run, in ONE foreground call with the maximum timeout:

```bash
for t in tests/test_*.sh; do
  out="$(bash "$t" 2>&1)" || true
  printf '%-52s %s\n' "$(basename "$t")" "$(printf '%s\n' "$out" | grep -cE '^NOT OK') NOT-OK"
done
```

Expected: `0 NOT-OK` for every test file. Any non-zero count is a real regression — root-cause it, never re-gate it to green.

- [ ] **Step 2: Confirm the portability constraint**

Any regex added in this change must be re-checked under BSD grep, since PATH `grep` is ugrep and is more permissive:

```bash
grep -rn 'harness-defaults' tests/ scripts/lib/ sync-agents.sh | head -20
bash tests/test_grep_portability.sh
```

Expected: PASS.

- [ ] **Step 3: Confirm a fresh generation is clean**

```bash
bash sync-agents.sh --check 2>&1 | tail -20
```

Expected: no CI-meaningful failure. Advisory drift lines about this machine's own `.claude/agents` are expected and acceptable — the maintainer's `.docket.local.yml` carries its own overrides.

- [ ] **Step 4: Commit any repairs**

```bash
git add -A
git commit -m "fix(0168): full-suite repairs"
```

---

## Notes for the reviewer and the close-out

**The ADR is not a plan task.** The change records one new ADR — *shipped agent defaults live in a sparse, harness-indexed sidecar; behavioral wrapper templates carry no cross-harness model floor* — authored on the `docket` branch by the implementer's ADR step, not on this feature branch. It **supersedes ADR-0048**, restating its three `.docket.example.yml` invariants with the mirror target re-pointed at `agents/harness-defaults.yml`; ADR-0048's `status:` flips to `Superseded by ADR-NN`. Both ADR ids must appear in the change's `adrs:` so terminal-publish carries them onto the integration branch atomically ([[adr-update-delivery]]) — a standalone publish would land one and dangle the other. ADR-0039 is already `Superseded by ADR-48` and is not touched.

**Cursor IDE certification is Tier 2 and human-only.** The five IDE checks in the spec (explicit economy/standard/premium routing, one auto-classified task, one bounded escalation) cannot be run by an autonomous build. Until a human completes them, the results file and PR body must say **Cursor IDE certification pending**, and that pending state blocks the merge. `cursor-agent` is not an accepted substitute.

**What this change does not do.** It does not make the unmapped-harness gap loud (change 0142), ship Codex defaults (change 0169), change the build controller's routing rubric or escalation graph (ADR-0063), or touch the `docket-build-task` worker contract.
