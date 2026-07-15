# First-run setup — starter config + install scaffolding + README restructure — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make docket's first-run setup an explicit sequence and turn the invisible default configuration into a concrete, editable artifact.

**Architecture:** Three deliverables. (1) A committed `config.yml.example` at the repo root — a copy-me template for the global `~/.config/docket/config.yml` that ships `agent_harnesses: [claude]` active, an `agents.claude` block mirroring docket's nine shipped built-in defaults, and commented-out `codex`/`cursor` example blocks. (2) A new idempotent primitive `scripts/ensure-global-config.sh` (with a co-located `.md` contract) that `install.sh` runs — before `sync-agents.sh` — to drop the starter into `${XDG_CONFIG_HOME:-$HOME/.config}/docket/config.yml` on first run only, never overwriting. (3) A README Install restructure from a single "one-line install" into a numbered setup sequence naming the config step.

**Tech Stack:** POSIX/bash shell scripts (`set -euo pipefail`), YAML (parsed by docket's hand-rolled bash parser in `sync-agents.sh`/`docket-config.sh` — NOT `yq`), Markdown docs, standalone bash test scripts under `tests/`.

## Global Constraints

- **This is a docs + shell-scaffolding change.** No changes to config *resolution* (`docket-config.sh`, four-layer precedence, coordination-key fence), no new config keys, no change to `sync-agents.sh` generation behavior.
- **The nine claude defaults are a MIRROR, not a source (ADR-0039).** `agents/docket-*.md` wrapper frontmatter is the single source of truth; `config.yml.example`'s `agents.claude` block must equal it value-for-value. The nine verified values (from `origin/main` wrapper frontmatter, 2026-07-15):
  - `status` → `docket-status.md`: `claude-haiku-4-5-20251001` / `medium`
  - `adr` → `docket-adr.md`: `claude-sonnet-5` / `medium`
  - `brainstorm-consultant` → `docket-brainstorm-consultant.md`: `claude-opus-4-8` / `xhigh`
  - `auto-groom` → `docket-auto-groom.md`: `claude-opus-4-8` / `xhigh`
  - `auto-groom-critic` → `docket-auto-groom-critic.md`: `claude-opus-4-8` / `xhigh`
  - `implement-next` → `docket-implement-next.md`: `claude-opus-4-8` / `xhigh`
  - `rebase-resolver` → `docket-rebase-resolver.md`: `claude-opus-4-8` / `xhigh`
  - `integration-repair` → `docket-integration-repair.md`: `claude-opus-4-8` / `xhigh`
  - `finalize-change` → `docket-finalize-change.md`: `claude-sonnet-5` / `medium`
- **Path agreement with `sync-agents.sh`.** `ensure-global-config.sh` MUST write to the exact path `sync-agents.sh` reads as the global config: `${XDG_CONFIG_HOME:-$HARNESS_ROOT/.config}/docket/config.yml` where `HARNESS_ROOT="${DOCKET_HARNESS_ROOT:-$HOME}"`. Honoring the `DOCKET_HARNESS_ROOT` test seam is required (install.sh passes it through to all sub-scripts) and is what makes the ordering meaningful (sync-agents reads the just-written file).
- **YAML format for the `agents` block.** Nested `agents:` → `<harness>:` → `<agent>: { model: X, effort: Y }`. Agent entries are 4-space-indented inline flow-maps; the harness key is 2-space-indented; `agents:` is column 0. This is the exact shape docket's parser (`section_body` + `harness_agent_line` + `field_of`) accepts — do not change indentation.
- **Commented harness blocks stay warning-free.** A harness block present in `agents:` but not listed in `agent_harnesses` would be noise; keep `codex`/`cursor` fully commented so a fresh copy is clean. Comment lines are stripped by the parser before it reads values.
- **Every `scripts/<name>.sh` has a co-located `scripts/<name>.md`** (enforced by `tests/test_script_contracts_coverage.sh`).
- **Tests are standalone bash scripts** run via `bash tests/test_<name>.sh`, exit 0 = pass. Hermetic: `unset XDG_CONFIG_HOME` and sandbox `HOME`/`DOCKET_HARNESS_ROOT` to `mktemp -d`. There is no GitHub Actions CI — the local suite is the gate.
- **Guards are code (LEARNINGS):** every new assert must be mutation-tested — strip the guarded value, watch it go red. Anchor on the unique full spelling, not a keyword set. A dir-creating step under `set -euo pipefail` must not abort the run; regression tests assert exit 0 AND that later work still happens.

---

### Task 1: `config.yml.example` (committed starter) + its guard test

**Files:**
- Create: `config.yml.example` (repo root)
- Create/Test: `tests/test_config_example.sh`

**Interfaces:**
- Produces: the committed starter file `config.yml.example` at the repo root, read by Task 2's `ensure-global-config.sh` (source) and by `sync-agents.sh` (once scaffolded into the global config path).

- [ ] **Step 1: Write the failing guard test** `tests/test_config_example.sh`

```bash
#!/usr/bin/env bash
# tests/test_config_example.sh — run: bash tests/test_config_example.sh
# Guards config.yml.example: it exists, ships agent_harnesses [claude] active, its
# agents.claude block MIRRORS the nine shipped agents/docket-*.md defaults value-for-value
# (ADR-0039 coupling), its codex/cursor blocks ship COMMENTED (warning-free fresh copy),
# and the file resolves cleanly through the REAL resolver (sync-agents.sh) — verbatim
# (no harness warnings) and with cursor uncommented + enabled (the example IDs resolve).
set -uo pipefail
unset XDG_CONFIG_HOME
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CFGEX="$REPO/config.yml.example"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }
_tmpdirs=(); trap 'rm -rf "${_tmpdirs[@]}"' EXIT

# frontmatter scalar from a wrapper file (col-0 model:/effort:)
fm(){ sed -n "s/^$2:[[:space:]]*//p" "$1" | head -n1 | sed 's/[[:space:]]*$//'; }
# model/effort from config.yml.example's 4-space-indented claude agent line — uses the
# SAME regex sync-agents.sh's field_of() uses, so the test can't accept a shape the
# resolver rejects.
cfg_field(){ # $1=agent  $2=field(model|effort)
  local line
  line="$(grep -E "^    $1:[[:space:]]" "$CFGEX" | head -n1)"
  printf '%s' "$line" | sed -nE "s/.*[{,[:space:]]$2[[:space:]]*:[[:space:]]*([A-Za-z0-9._-]+).*/\1/p" | head -n1
}

assert "config.yml.example exists at repo root" '[ -f "$CFGEX" ]'
assert "agent_harnesses ships [claude] active" \
  'grep -Eq "^agent_harnesses:[[:space:]]*\[[[:space:]]*claude[[:space:]]*\]" "$CFGEX"'

# Defaults-match: the nine agents.claude values equal the shipped wrapper frontmatter.
# agent-key -> wrapper basename is identity with a docket- prefix.
for a in status adr brainstorm-consultant auto-groom auto-groom-critic \
         implement-next rebase-resolver integration-repair finalize-change; do
  w="$REPO/agents/docket-$a.md"
  assert "$a: wrapper exists" '[ -f "$w" ]'
  assert "$a: model mirrors wrapper" '[ -n "$(cfg_field "$a" model)" ] && [ "$(cfg_field "$a" model)" = "$(fm "$w" model)" ]'
  assert "$a: effort mirrors wrapper" '[ -n "$(cfg_field "$a" effort)" ] && [ "$(cfg_field "$a" effort)" = "$(fm "$w" effort)" ]'
done

# codex/cursor ship COMMENTED: no ACTIVE (uncommented) codex:/cursor: header line exists.
assert "codex block ships commented (no active codex: header)" '! grep -Eq "^[[:space:]]*codex:[[:space:]]*$" "$CFGEX"'
assert "cursor block ships commented (no active cursor: header)" '! grep -Eq "^[[:space:]]*cursor:[[:space:]]*$" "$CFGEX"'
# ...but the commented examples are present (so a user can find + enable them).
assert "commented codex example present" 'grep -Eq "^[[:space:]]*#[[:space:]]*codex:" "$CFGEX"'
assert "commented cursor example present" 'grep -Eq "^[[:space:]]*#[[:space:]]*cursor:" "$CFGEX"'

# Real-resolver check A: the file as-shipped generates a claude wrapper with NO harness
# warnings (the commented codex/cursor blocks are invisible to the parser).
SB="$(mktemp -d)"; _tmpdirs+=("$SB"); mkdir -p "$SB/.claude/agents" "$SB/.config/docket"
cp "$CFGEX" "$SB/.config/docket/config.yml"
err="$(cd "$SB" && HOME="$SB" DOCKET_HARNESS_ROOT="$SB" bash "$REPO/sync-agents.sh" 2>&1 >/dev/null)"; rc=$?
assert "sync-agents resolves config.yml.example verbatim (exit 0)" '[ "$rc" = "0" ]'
assert "verbatim: a claude wrapper was generated" '[ -f "$SB/.claude/agents/docket-status.md" ]'
assert "verbatim: no unknown-harness-token warning" '! printf "%s" "$err" | grep -qiE "unknown agent_harnesses token"'
assert "verbatim: generated claude status model mirrors built-in" \
  '[ "$(fm "$SB/.claude/agents/docket-status.md" model)" = "$(fm "$REPO/agents/docket-status.md" model)" ]'

# Real-resolver check B: uncomment the cursor block (it is the LAST block in the file) and
# enable cursor — the example IDs must resolve into a cursor wrapper. Proves the commented
# block is valid YAML that resolves once uncommented.
SB2="$(mktemp -d)"; _tmpdirs+=("$SB2"); mkdir -p "$SB2/.claude/agents" "$SB2/.cursor/agents" "$SB2/.config/docket"
awk '
  /^[[:space:]]*# cursor:/ { c=1 }
  c && /^  # ?/ { sub(/^  # ?/,"  ") }
  { print }
' "$CFGEX" | sed -E 's/^agent_harnesses:.*/agent_harnesses: [claude, cursor]/' \
  > "$SB2/.config/docket/config.yml"
err2="$(cd "$SB2" && HOME="$SB2" DOCKET_HARNESS_ROOT="$SB2" bash "$REPO/sync-agents.sh" 2>&1 >/dev/null)"; rc2=$?
assert "sync-agents resolves cursor-uncommented config (exit 0)" '[ "$rc2" = "0" ]'
assert "uncommented: a cursor wrapper was generated" '[ -f "$SB2/.cursor/agents/docket-status.md" ]'
assert "uncommented: cursor status model came from the example block" \
  '[ "$(fm "$SB2/.cursor/agents/docket-status.md" model)" = "grok-4.5-fast-medium" ]'

exit $fail
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_config_example.sh`
Expected: FAIL — `config.yml.example exists at repo root` is NOT OK (file absent), and everything downstream fails.

- [ ] **Step 3: Create `config.yml.example`**

```yaml
# ~/.config/docket/config.yml — docket's global (per-machine) configuration.
#
# This starter was scaffolded from docket's config.yml.example on your first `install.sh`.
# It accepts the FULL .docket.yml schema; every key resolves per-field, four layers deep:
# repo-local (.docket.local.yml) > repo-committed (.docket.yml) > this global file > built-in.
# Only the two harness/model keys are shown here — see README -> Configuration for every
# other key, and for the coordination-key fence (repo-only keys that are ignored here).
#
# Claude-only users can leave this file untouched: the values below already match docket's
# shipped defaults, so an unedited file behaves exactly as if it were absent.

# Which agent harnesses get generated wrapper files. To enable `cursor` or `codex`, add it
# here AND uncomment that harness's block under `agents:` below, then re-run `install.sh`.
# The two are orthogonal: this list decides which harness dirs get files; the block sets
# the model/effort those files carry.
agent_harnesses: [claude]

# Per-skill model + effort for each docket subagent, keyed by harness.
#
# The `claude:` block below MIRRORS docket's shipped built-in defaults (the values in each
# agents/docket-*.md wrapper), shown so the otherwise-invisible defaults are visible and
# tunable. Deleting any line falls back to the SAME built-in. Per ADR-0039 the wrappers are
# the single source of truth; if a shipped default changes, update this mirror to match.
agents:
  claude:
    status:                { model: claude-haiku-4-5-20251001, effort: medium }
    adr:                   { model: claude-sonnet-5,           effort: medium }
    brainstorm-consultant: { model: claude-opus-4-8,           effort: xhigh }
    auto-groom:            { model: claude-opus-4-8,           effort: xhigh }
    auto-groom-critic:     { model: claude-opus-4-8,           effort: xhigh }
    implement-next:        { model: claude-opus-4-8,           effort: xhigh }
    rebase-resolver:       { model: claude-opus-4-8,           effort: xhigh }
    integration-repair:    { model: claude-opus-4-8,           effort: xhigh }
    finalize-change:       { model: claude-sonnet-5,           effort: medium }

  # To enable a block below: verify the example IDs against your harness's current models, uncomment the block, and add the harness to `agent_harnesses` above. The IDs here are UNVALIDATED examples.
  # codex:
  #   status:                { model: gpt-5.6-luna, effort: xhigh }
  #   adr:                   { model: gpt-5.6-terra, effort: xhigh }
  #   brainstorm-consultant: { model: gpt-5.6-sol, effort: medium }
  #   auto-groom:            { model: gpt-5.6-sol, effort: low }
  #   auto-groom-critic:     { model: gpt-5.6-sol, effort: medium }
  #   implement-next:        { model: gpt-5.6-sol, effort: medium }
  #   rebase-resolver:       { model: gpt-5.6-sol, effort: high }
  #   integration-repair:    { model: gpt-5.6-sol, effort: high }
  #   finalize-change:       { model: gpt-5.6-terra, effort: high }
  # cursor:
  #   status:                { model: grok-4.5-fast-medium, effort: auto }
  #   adr:                   { model: grok-4.5-xhigh, effort: auto }
  #   brainstorm-consultant: { model: grok-4.5-xhigh, effort: auto }
  #   auto-groom:            { model: grok-4.5-high, effort: auto }
  #   auto-groom-critic:     { model: grok-4.5-xhigh, effort: auto }
  #   implement-next:        { model: grok-4.5-xhigh, effort: auto }
  #   rebase-resolver:       { model: grok-4.5-xhigh, effort: auto }
  #   integration-repair:    { model: grok-4.5-xhigh, effort: auto }
  #   finalize-change:       { model: grok-4.5-fast-high, effort: auto }
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_config_example.sh`
Expected: PASS — all `ok - …` lines, exit 0.

- [ ] **Step 5: Mutation-test the defaults-match + resolver guards (LEARNINGS mandate)**

Prove each guard can redden against the real tree, then revert:
```bash
# (a) break a mirror value -> the "mirrors wrapper" assert must go red
sed -i.bak 's/claude-haiku-4-5-20251001/claude-haiku-WRONG/' config.yml.example
bash tests/test_config_example.sh | grep -q "NOT OK - status: model mirrors wrapper" && echo "MUTANT-A caught"
mv config.yml.example.bak config.yml.example
# (b) break the cursor example model -> the uncommented-resolves assert must go red
sed -i.bak 's/grok-4.5-fast-medium/grok-WRONG/' config.yml.example
bash tests/test_config_example.sh | grep -q "NOT OK - uncommented: cursor status model came from the example block" && echo "MUTANT-B caught"
mv config.yml.example.bak config.yml.example
```
Expected: both `MUTANT-… caught` printed; after reverts, `bash tests/test_config_example.sh` is green again. (Do not commit the `.bak` files.)

- [ ] **Step 6: Commit**

```bash
git add config.yml.example tests/test_config_example.sh
git commit -m "feat(0081): committed config.yml.example starter + mirror/resolver guard test"
```

---

### Task 2: `scripts/ensure-global-config.sh` primitive + `.md` contract + unit test

**Files:**
- Create: `scripts/ensure-global-config.sh`
- Create: `scripts/ensure-global-config.md`
- Create/Test: `tests/test_ensure_global_config.sh`

**Interfaces:**
- Consumes: `config.yml.example` (from Task 1, at repo root).
- Produces: the primitive `scripts/ensure-global-config.sh`, invoked by `install.sh` in Task 3. Behavior contract: writes `${XDG_CONFIG_HOME:-$HARNESS_ROOT/.config}/docket/config.yml` (`HARNESS_ROOT="${DOCKET_HARNESS_ROOT:-$HOME}"`) from `config.yml.example` iff absent; leaves any existing file untouched; exits 0 in both cases; honors `DOCKET_HARNESS_ROOT`/`XDG_CONFIG_HOME`.

- [ ] **Step 1: Write the failing unit test** `tests/test_ensure_global_config.sh`

```bash
#!/usr/bin/env bash
# tests/test_ensure_global_config.sh — run: bash tests/test_ensure_global_config.sh
# Unit-tests the ensure-global-config.sh primitive: fresh (writes byte-identical copy +
# logs "wrote"), existing (untouched + logs "left untouched"), idempotent, exit 0 both.
set -uo pipefail
unset XDG_CONFIG_HOME
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$REPO/scripts/ensure-global-config.sh"
CFGEX="$REPO/config.yml.example"
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }
_tmpdirs=(); trap 'rm -rf "${_tmpdirs[@]}"' EXIT

# Fresh: empty sandbox home, no existing config.
SB="$(mktemp -d)"; _tmpdirs+=("$SB")
DEST="$SB/.config/docket/config.yml"
out="$(HOME="$SB" DOCKET_HARNESS_ROOT="$SB" bash "$SCRIPT" 2>&1)"; rc=$?
assert "fresh: exits 0" '[ "$rc" = "0" ]'
assert "fresh: creates the global config" '[ -f "$DEST" ]'
assert "fresh: copy is byte-identical to config.yml.example" 'cmp -s "$CFGEX" "$DEST"'
assert "fresh: logs a wrote line naming the dest" 'printf "%s" "$out" | grep -qF "wrote $DEST"'

# Existing: pre-seed a distinct sentinel; the script must NOT touch it.
SB2="$(mktemp -d)"; _tmpdirs+=("$SB2")
DEST2="$SB2/.config/docket/config.yml"
mkdir -p "$(dirname "$DEST2")"; printf 'sentinel: do-not-overwrite\n' > "$DEST2"
out2="$(HOME="$SB2" DOCKET_HARNESS_ROOT="$SB2" bash "$SCRIPT" 2>&1)"; rc2=$?
assert "existing: exits 0" '[ "$rc2" = "0" ]'
assert "existing: file is left untouched" '[ "$(cat "$DEST2")" = "sentinel: do-not-overwrite" ]'
assert "existing: logs a left-untouched line" 'printf "%s" "$out2" | grep -qF "left untouched"'

# Idempotent: a second fresh-run over the just-written file leaves it untouched (now existing).
out3="$(HOME="$SB" DOCKET_HARNESS_ROOT="$SB" bash "$SCRIPT" 2>&1)"; rc3=$?
assert "idempotent: second run exits 0" '[ "$rc3" = "0" ]'
assert "idempotent: second run reports left untouched" 'printf "%s" "$out3" | grep -qF "left untouched"'

# XDG_CONFIG_HOME wins when set.
SB3="$(mktemp -d)"; _tmpdirs+=("$SB3"); XDGDIR="$(mktemp -d)"; _tmpdirs+=("$XDGDIR")
out4="$(HOME="$SB3" DOCKET_HARNESS_ROOT="$SB3" XDG_CONFIG_HOME="$XDGDIR" bash "$SCRIPT" 2>&1)"; rc4=$?
assert "xdg: honors XDG_CONFIG_HOME" '[ -f "$XDGDIR/docket/config.yml" ] && [ "$rc4" = "0" ]'

exit $fail
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `bash tests/test_ensure_global_config.sh`
Expected: FAIL — script missing (`bash: .../ensure-global-config.sh: No such file`), every assert NOT OK.

- [ ] **Step 3: Create `scripts/ensure-global-config.sh`**

```bash
#!/usr/bin/env bash
# ensure-global-config.sh — scaffold the global docket config on first run.
#
# Copies the committed config.yml.example (repo root) to the user's global docket config
# at ${XDG_CONFIG_HOME:-$HOME/.config}/docket/config.yml — but ONLY if that file does not
# already exist. Never overwrites, never merges, never edits an existing file. Idempotent:
# safe to re-run any number of times. Run by install.sh BEFORE sync-agents.sh so the first
# generator pass reads the just-written global config.
#
# Test seam: DOCKET_HARNESS_ROOT overrides $HOME for the config root (matching sync-agents.sh),
# and it is only consulted when XDG_CONFIG_HOME is unset (a set XDG_CONFIG_HOME wins).
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SRC="$REPO_ROOT/config.yml.example"

HARNESS_ROOT="${DOCKET_HARNESS_ROOT:-$HOME}"
DEST_DIR="${XDG_CONFIG_HOME:-$HARNESS_ROOT/.config}/docket"
DEST="$DEST_DIR/config.yml"

if [ ! -f "$SRC" ]; then
  echo "docket: ensure-global-config: source $SRC not found — skipping" >&2
  exit 0
fi

if [ -e "$DEST" ]; then
  echo "docket: $DEST already exists — left untouched"
  exit 0
fi

mkdir -p "$DEST_DIR"
cp "$SRC" "$DEST"
echo "docket: wrote $DEST from config.yml.example (edit to enable harnesses / tune models)"
exit 0
```

- [ ] **Step 4: Create the contract `scripts/ensure-global-config.md`**

```markdown
# ensure-global-config.sh

## Purpose

Scaffold the global docket config on first run: drop the committed `config.yml.example`
into place as the user's global `~/.config/docket/config.yml`, so the otherwise-invisible
per-skill defaults are discoverable and the file exists for editing — without ever
clobbering a config the user has already written.

## Usage

```
bash scripts/ensure-global-config.sh
```

Run by `install.sh` as a primitive, before `sync-agents.sh`. Standalone-safe.

Environment:
- `XDG_CONFIG_HOME` — when set, the config root (wins over `HOME`/`DOCKET_HARNESS_ROOT`).
- `DOCKET_HARNESS_ROOT` — test seam overriding `$HOME` for the config root; consulted only
  when `XDG_CONFIG_HOME` is unset. Matches `sync-agents.sh`'s resolution so both agree on
  the path.

## Behavior

- Destination: `${XDG_CONFIG_HOME:-${DOCKET_HARNESS_ROOT:-$HOME}/.config}/docket/config.yml`.
- If the destination does NOT exist: create the parent dir as needed, copy
  `config.yml.example` (from the repo root) to it, and log
  `docket: wrote <dest> from config.yml.example (edit to enable harnesses / tune models)`.
- If the destination already exists: do nothing to it, log
  `docket: <dest> already exists — left untouched`.
- If `config.yml.example` is missing: log a skip to stderr and exit 0 (never fatal).
- Never overwrites, merges, or edits an existing file.

## Exit codes

- `0` — always (wrote, left-untouched, or source-missing skip). Idempotent.

## Invariants

- An existing global config is never modified.
- The written copy is byte-identical to `config.yml.example`.
- The destination path equals the path `sync-agents.sh` reads as the global config.
```

- [ ] **Step 5: Run the unit test + the contract-coverage test to verify they pass**

Run: `bash tests/test_ensure_global_config.sh && bash tests/test_script_contracts_coverage.sh`
Expected: both PASS (exit 0). The coverage test confirms `ensure-global-config.sh` has its `.md`.

- [ ] **Step 6: Mutation-test the never-overwrite guard**

```bash
# Break the existence guard so it overwrites -> the "left untouched" assert must go red.
cp scripts/ensure-global-config.sh /tmp/egc.bak
sed -i.bak 's/if \[ -e "\$DEST" \]; then/if false; then/' scripts/ensure-global-config.sh
bash tests/test_ensure_global_config.sh | grep -q "NOT OK - existing: file is left untouched" && echo "MUTANT caught"
cp /tmp/egc.bak scripts/ensure-global-config.sh; rm -f scripts/ensure-global-config.sh.bak
```
Expected: `MUTANT caught`; after revert, `bash tests/test_ensure_global_config.sh` is green.

- [ ] **Step 7: Commit**

```bash
git add scripts/ensure-global-config.sh scripts/ensure-global-config.md tests/test_ensure_global_config.sh
git commit -m "feat(0081): ensure-global-config.sh primitive + contract + unit test"
```

---

### Task 3: Wire `ensure-global-config.sh` into `install.sh` + extend `test_install.sh`

**Files:**
- Modify: `install.sh` (add the fourth primitive before `sync-agents.sh`; update the header comment's "runs N primitives in order" list)
- Modify: `tests/test_install.sh`

**Interfaces:**
- Consumes: `scripts/ensure-global-config.sh` (Task 2).
- Produces: an `install.sh` that scaffolds the global config before generating wrappers, so the first `sync-agents.sh` pass reads it.

- [ ] **Step 1: Extend `tests/test_install.sh` with the failing assertions**

Add, immediately after the existing `install.sh ran sync-agents.sh …` assert (before the idempotency block), assertions that install scaffolds the global config and leaves an edited one untouched:

```bash
# install.sh scaffolds the global config (ensure-global-config.sh), before sync-agents reads it.
assert "install.sh scaffolded the global config" '[ -f "$tmp/.config/docket/config.yml" ]'
assert "install.sh global config is the committed starter" 'cmp -s "$REPO/config.yml.example" "$tmp/.config/docket/config.yml"'
```

And after the existing idempotency block, prove a user-edited global config survives a re-run:

```bash
# A user-edited global config is NOT overwritten by a re-run.
printf '# user edit\nagent_harnesses: [claude]\n' > "$tmp/.config/docket/config.yml"
out3="$(cd "$tmp" && HOME="$tmp" DOCKET_HARNESS_ROOT="$tmp" DOCKET_TARGET_SHELL=zsh bash "$REPO/install.sh" 2>&1)"; rc3=$?
assert "install.sh re-run exits 0 with an edited global config" '[ "$rc3" = "0" ]'
assert "install.sh re-run left the user-edited global config untouched" \
  'grep -qF "# user edit" "$tmp/.config/docket/config.yml"'
```

- [ ] **Step 2: Run the test to verify the new assertions fail**

Run: `bash tests/test_install.sh`
Expected: FAIL — `install.sh scaffolded the global config` is NOT OK (install doesn't run the primitive yet).

- [ ] **Step 3: Modify `install.sh`**

Update the header comment's primitive list from "three" to "four" and insert the new step. Replace the header block that reads:

```bash
# Runs the three install primitives in order:
#   1. link-skills.sh  — symlink the skills into each present harness's skill dir (live; edit-once)
#   2. sync-agents.sh  — generate the model/effort-pinned agent wrappers into each present harness
#                        (generated copies; re-run after editing a config layer)
#   3. ensure-docket-env.sh — export DOCKET_SCRIPTS_DIR so the skills can reach scripts/ from any
#                             consuming repo (re-run back-fills already-migrated clones)
```

with:

```bash
# Runs the four install primitives in order:
#   1. link-skills.sh  — symlink the skills into each present harness's skill dir (live; edit-once)
#   2. ensure-global-config.sh — scaffold ~/.config/docket/config.yml from config.yml.example on
#                                first run (non-destructive), so the defaults are discoverable and
#                                the generator (step 3) reads it
#   3. sync-agents.sh  — generate the model/effort-pinned agent wrappers into each present harness
#                        (generated copies; re-run after editing a config layer)
#   4. ensure-docket-env.sh — export DOCKET_SCRIPTS_DIR so the skills can reach scripts/ from any
#                             consuming repo (re-run back-fills already-migrated clones)
```

Then insert the invocation between the `link-skills.sh` block and the `sync-agents.sh` block. After:

```bash
echo "==> link-skills.sh (install skills)"
bash "$SCRIPT_DIR/link-skills.sh"
```

insert:

```bash
echo "==> ensure-global-config.sh (scaffold global config)"
bash "$SCRIPT_DIR/scripts/ensure-global-config.sh"
```

(The `sync-agents.sh` and `ensure-docket-env.sh` blocks are unchanged. `DOCKET_HARNESS_ROOT` is already inherited by every sub-script, so no wiring change is needed there.)

- [ ] **Step 4: Run the test to verify it passes**

Run: `bash tests/test_install.sh`
Expected: PASS — all `ok - …`, including the new scaffold + untouched-on-re-run assertions, exit 0.

- [ ] **Step 5: Commit**

```bash
git add install.sh tests/test_install.sh
git commit -m "feat(0081): install.sh scaffolds the global config before sync-agents"
```

---

### Task 4: README Install restructure

**Files:**
- Modify: `README.md` (the `## Install` section)

**Interfaces:**
- Consumes: the behavior established in Tasks 1-3 (the new primitive, the starter file).
- Produces: a numbered setup sequence naming the config step. Docs only — no code.

- [ ] **Step 1: Add a structural guard to `tests/test_config_example.sh`**

Append before the final `exit $fail` in `tests/test_config_example.sh` a minimal, uniquely-anchored check that the README wires the new setup step (catches an accidental omission of the doc change; anchored on the exact new heading, not a keyword set):

```bash
# README wires the new setup step (Deliverable 3).
README="$REPO/README.md"
assert "README has the step-2 global-config heading" 'grep -qF "### 2. Set up your global config" "$README"'
assert "README step-2 names config.yml.example" 'grep -qF "config.yml.example" "$README"'
```

- [ ] **Step 2: Run to verify it fails**

Run: `bash tests/test_config_example.sh`
Expected: FAIL — `README has the step-2 global-config heading` is NOT OK (README not restructured yet).

- [ ] **Step 3: Restructure the README `## Install` section**

Keep `### Prerequisites` unchanged. Rename the `### The one-line install` heading to `### 1. Install docket on your machine`. In its primitives list, insert an `ensure-global-config.sh` bullet (between the `link-skills.sh` and `sync-agents.sh` bullets), and update the sentence "runs three primitives in order" to "runs four primitives in order". Concretely:

Change the heading line:
```markdown
### The one-line install
```
to:
```markdown
### 1. Install docket on your machine
```

Change the sentence:
```markdown
That is the whole install. `install.sh` runs three primitives in order and is idempotent — re-run it any time (after adding a harness, or after editing `~/.config/docket/config.yml`):
```
to:
```markdown
That is the whole install. `install.sh` runs four primitives in order and is idempotent — re-run it any time (after adding a harness, or after editing `~/.config/docket/config.yml`):
```

Insert this bullet immediately after the `link-skills.sh` bullet and before the `sync-agents.sh` bullet:
```markdown
- **`ensure-global-config.sh`** drops a starter `~/.config/docket/config.yml` into place from the committed `config.yml.example` the first time you install — non-destructively (an existing config is left untouched). This is where docket's per-skill model defaults become visible and editable (see step 2). It runs before `sync-agents.sh` so the generator reads the just-written config.
```

Then, immediately after the `### 1. …` subsection's closing parenthetical line:
```markdown
(You can still run any primitive on its own — `install.sh` just saves you from remembering all three.)
```
update "all three" to "all four":
```markdown
(You can still run any primitive on its own — `install.sh` just saves you from remembering all four.)
```
and add a new subsection **before** the "change data" tail paragraph:

```markdown
### 2. Set up your global config

`install.sh` writes a starter `~/.config/docket/config.yml` from `config.yml.example` the first time it runs (and leaves an existing one untouched). That starter is where docket's otherwise-invisible defaults become visible and editable:

- It shows docket's built-in **per-skill model and effort** for every subagent — the `agents.claude` block mirrors the shipped defaults, so you can see and tune them in one place instead of reading nine wrapper files.
- **Claude-only users can skip editing entirely** — the defaults already apply, so an unedited file behaves exactly as no file at all.
- **To enable another harness (Cursor, Codex):** add it to `agent_harnesses` **and** uncomment that harness's block under `agents:`, then re-run `install.sh` so `sync-agents.sh` regenerates the wrappers.

See [Configuration](#configuration--docketyml-global-config-and-machine-local-overrides) for the full schema and every other key.
```

The existing "change data — `docs/changes/` … `migrate-to-docket.sh`" paragraph stays as the tail of the section, unchanged.

- [ ] **Step 4: Run to verify it passes**

Run: `bash tests/test_config_example.sh`
Expected: PASS — the two README asserts are now `ok`.

- [ ] **Step 5: Commit**

```bash
git add README.md tests/test_config_example.sh
git commit -m "docs(0081): README Install restructure — numbered setup sequence + global-config step"
```

---

## Whole-suite verification (after all tasks)

- [ ] Run the full suite and confirm green:

```bash
for t in tests/test_*.sh; do echo "== $t =="; bash "$t" || echo "FAILED: $t"; done
```
Expected: no `FAILED:` lines. Pay special attention to `test_config_example.sh`, `test_ensure_global_config.sh`, `test_install.sh`, `test_script_contracts_coverage.sh`, `test_sync_agents.sh`, and `test_docket_config.sh` (the config surfaces this change touches).

## Self-review notes (author)

- **Spec coverage:** Deliverable 1 → Task 1 (file + mirror/resolver guard); Deliverable 2 → Task 2 (script + contract + unit test); Deliverable 2 wiring → Task 3 (install.sh + test_install); Deliverable 3 → Task 4 (README + heading guard). Every spec Testing case is covered: fresh/existing/idempotency → Task 2; YAML-validity/resolution → Task 1 checks A+B (via the real resolver, not yq); defaults-match → Task 1.
- **ADR-0039 coupling** is enforced by Task 1's per-agent mirror assert (mutation-tested in Step 5). No automated drift guard beyond the build-time equality check (out of scope, per spec).
- **Path agreement:** `ensure-global-config.sh` and `sync-agents.sh` both resolve `${XDG_CONFIG_HOME:-${DOCKET_HARNESS_ROOT:-$HOME}/.config}/docket/config.yml` — verified consistent; Task 3's test_install assert proves the umbrella writes where the generator reads.
- **Type/name consistency:** agent keys (`status`, `adr`, …) map identically to `docket-<key>.md` wrappers across Tasks 1-2; the log strings `wrote <dest>` / `left untouched` are used verbatim by both the script (Task 2) and its tests.
