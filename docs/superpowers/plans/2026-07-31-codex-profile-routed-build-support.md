<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0169 — Codex support for profile-routed Docket builds](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0169-codex-profile-routed-build-support.md)**
<!-- docket:backlink:end -->

# Codex Support for Profile-Routed Docket Builds — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship one complete, validated twelve-agent Codex block in `agents/harness-defaults.yml`, arm the shipped-harness completeness gate for it, and flip every guard and document that deliberately encodes "docket ships nothing for Codex".

**Architecture:** No new mechanism. The Codex TOML emitter, the field-level resolver, the sidecar validator, and Codex's native named-agent dispatch already exist and already work — the only missing input is the data. Adding a `codex:` block plus the `codex` token in `HD_SHIPPED_HARNESSES` makes all twelve `.codex/agents/docket-*.toml` wrappers come out pinned with zero emitter changes (verified end-to-end during planning). Everything else in this plan is consequence management: guards whose premise was the absence must be re-pointed at the post-change truth rather than deleted, and documentation that promised "until change 0169" must become the present tense.

**Tech Stack:** Bash 3.2-compatible shell (`sync-agents.sh`, `scripts/lib/harness-defaults.sh`), `awk`/`sed` text processing, the repo's hand-rolled `tests/test_*.sh` assert harness, Codex CLI 0.146.0.

## Global Constraints

- **Bash 3.2 / BSD userland.** No `declare -A`, no `mapfile`, no GNU-only `sed -i` form (use `sed -i.bak` or `sed -i ''` consistently with the file you are editing). **`/usr/bin/grep` is the portability target** — PATH `grep` on this machine is ugrep and accepts intervals BSD grep rejects. Re-check any new regex under `/usr/bin/grep`.
- **`set -o pipefail` is in force** in `sync-agents.sh` and the test files. Never `producer | grep -q` / `| head` — capture into a variable, then `grep <<<"$var"`. Single-line selection uses `${var%%$'\n'*}`.
- **`grep` for a pattern leading with `--` must declare it** (`grep -qF -- "$pat"`); inside a negated assert a bare leading `--` becomes a permanently green vacuous guard.
- **ADR-0015 opaque passthrough.** Model IDs and effort tokens are never validated against a vendor allowlist. Structural validation only — no runtime model allowlist, no automatic fallback for an unavailable slug.
- **ADR-0064 provenance split holds and is tested, not redesigned.** Only a model/effort that won from a **user** configuration layer may become a delegated runner `--model`/`--effort` flag. A shipped sidecar value never may.
- **No harness-neutral `default:` block** in the sidecar; **`runner:` is forbidden** there.
- **Bare scalars only** in the sidecar — unquoted, space-free — or `hd_validate` rejects them.
- **Exact shipped Codex values** (the settled design; re-verified against the installed catalog at reconcile):

  | Agent | Model | Effort |
  |---|---|---|
  | `adr` | `gpt-5.6-terra` | `xhigh` |
  | `auto-groom` | `gpt-5.6-sol` | `low` |
  | `auto-groom-critic` | `gpt-5.6-sol` | `medium` |
  | `brainstorm-consultant` | `gpt-5.6-sol` | `medium` |
  | `build-economy` | `gpt-5.6-luna` | `xhigh` |
  | `build-standard` | `gpt-5.6-terra` | `high` |
  | `build-premium` | `gpt-5.6-sol` | `medium` |
  | `finalize-change` | `gpt-5.6-terra` | `high` |
  | `implement-next` | `gpt-5.6-sol` | `medium` |
  | `integration-repair` | `gpt-5.6-sol` | `high` |
  | `rebase-resolver` | `gpt-5.6-sol` | `high` |
  | `status` | `gpt-5.6-luna` | `xhigh` |

- **Claude and Cursor generated output must not change.** This change adds a harness; it does not retune the other two.
- **Guards are code** (AGENTS.md): every new or re-pointed assert must be mutation-tested — break the thing it guards, watch it redden — and the mutation must be confirmed to have landed.
- **Derive guard populations from `HD_SHIPPED_HARNESSES`**, never from a literal `claude cursor codex` list. A hand-written harness list is the exact thing that let a stale claim survive on the Cursor side (see Task 3's note).
- **A retiring guard must hand off, not vanish.** Several guards in this repo are written as `if <premise-still-true>; then …assert…; fi`, so landing the Codex block silently switches them off. Every such site gets an `else` arm asserting the post-change truth. Deleting or silently retiring one is a plan violation.
- **Cross-references anchor on a symbol name or a verbatim-quoted clause, never a line number** (AGENTS.md). This binds maintained source; point-in-time records keep their original wording.
- **Commit after every task.** Run the **whole** suite at the final gate, never only the enumerated tests.

---

## Verified during planning (do not re-derive; reuse the evidence)

Three mechanics were prototyped against the real tree before this plan was written. Each is reproduced as working code in its task.

1. **The sidecar block alone is sufficient.** With the `codex:` block appended and `HD_SHIPPED_HARNESSES="claude cursor codex"`, `hd_validate` passes and a sandbox run of `sync-agents.sh` emits all twelve `.codex/agents/docket-*.toml` carrying exactly the table above as `model` / `model_reasoning_effort`. **No change to `sync-agents.sh`'s emitter or resolver is required.**
2. **The uncovered-pair warning survives** via a repo-copy fixture that reconstructs a known-but-unshipped harness (strip `codex` from the copy's `HD_SHIPPED_HARNESSES` *and* delete its sidecar block). The warning fires verbatim: `WARN codex/docket-status: no harness-specific model — generated unpinned; harness 'codex' will apply its own default. Set agents.codex.status.model to pin it.`
3. **A fully derived mirror terminator works.** Deriving each harness's `.docket.example.yml` slice terminator as `build-premium:.*<ERE-escaped sidecar build-premium model>` isolates all three blocks correctly (claude 13 lines, cursor 13, codex 14) and all 36 model/effort comparisons pass.

---

## File Structure

**Modified**
- `agents/harness-defaults.yml` — gains the complete twelve-agent `codex:` block; header comment's "the claude and cursor blocks are both COMPLETE" rule restated over the shipped set.
- `scripts/lib/harness-defaults.sh` — `HD_SHIPPED_HARNESSES` gains `codex`; the comment that explains codex's deliberate absence is retired.
- `tests/test_harness_defaults.sh` — codex value asserts; harness-set assert; sparse-read assert narrowed to a genuinely unshipped token; completeness mutations derived per shipped harness.
- `tests/test_sync_agents_codex.sh` — two TOML absence asserts become sidecar-derived value asserts; all-twelve coverage; the AGENTS.md premise guard gains its `else` arm.
- `tests/test_sync_agents.sh` — the uncovered-pair warning fixture is rebuilt on a repo copy; a sharpened provenance assert proves a shipped **Codex** default never becomes a Claude-to-Codex runner flag.
- `sync-agents.sh` — `assemble_agents_md_dispatch`'s HEAD prose stops claiming Codex ships nothing; the two block comments that explain that premise are updated.
- `.docket.example.yml` — the `codex:` block is promoted to a singly commented exact mirror with the settled build rows; surrounding prose describes three shipped harnesses.
- `tests/test_docket_example_yml.sh` — comment-level assert flips; mirror population and terminators derived from `HD_SHIPPED_HARNESSES`; the resolver round-trip enables codex and gains TOML evidence.
- `README.md`, `docs/codex/setup.md`, `skills/docket-build/SKILL.md` — maintained prose updated to three shipped harnesses and harness-neutral profile workers.
- `docs/results/2026-07-31-codex-profile-routed-build-support-results.md` — created in Task 6.

**Deliberately NOT modified**
- Any emitter, resolver, or controller logic. The change is data plus consequence management.
- `agents/docket-*.md` wrapper sources, the `claude:` / `cursor:` sidecar blocks, `skills/docket-build-task/SKILL.md`'s contract.
- Point-in-time records: archived changes, Accepted ADR bodies, prior specs, plans, and results keep their original wording.

---

### Task 1: Ship the Codex block and re-point every guard whose premise was its absence

This task is deliberately atomic. Landing the block flips four assertions from true to false at once, so the data and the guards that read it must move in one commit or the suite is red mid-task.

**Files:**
- Modify: `agents/harness-defaults.yml`
- Modify: `scripts/lib/harness-defaults.sh`
- Test: `tests/test_harness_defaults.sh`
- Test: `tests/test_sync_agents_codex.sh`
- Test: `tests/test_sync_agents.sh`

**Interfaces:**
- Consumes: nothing from earlier tasks.
- Produces: `hd_field "$HD" codex <agent> model|effort` resolves for all twelve agents; `HD_SHIPPED_HARNESSES` is `"claude cursor codex"`; `hd_harnesses` returns `claude codex cursor` (sorted). Later tasks derive their populations from `HD_SHIPPED_HARNESSES`.

- [ ] **Step 1: Write the failing value asserts for the Codex block**

In `tests/test_harness_defaults.sh`, replace the harness-set assert. Find:

```bash
assert "harnesses are claude+cursor only" \
  '[ "$(hd_harnesses "$HD" | tr "\n" " ")" = "claude cursor " ]'
```

Replace with (`hd_harnesses` sorts, so `codex` lands between `claude` and `cursor`):

```bash
assert "harnesses are exactly the three shipped ones" \
  '[ "$(hd_harnesses "$HD" | tr "\n" " ")" = "claude codex cursor " ]'
```

Then find and DELETE these three lines (the block-absence premise is what this change removes):

```bash
assert "no codex block yet (change 0169 owns it)" '[ -z "$(hd_agents "$HD" codex)" ]'
# A pair unlisted because its HARNESS ships nothing still resolves empty — the sparse-by-harness
# property survives cursor becoming complete.
assert "unlisted harness pair resolves empty" '[ -z "$(hd_field "$HD" codex status model)" ]'
```

Insert in their place — a full value loop plus a **narrowed** sparse-read assert. The sparse-by-harness read is a real mechanism that still exists, so it is narrowed to a token with genuinely no block rather than deleted:

```bash
# ---- the Codex block: complete, with per-agent efforts ----------------------
# Unlike cursor (whose IDs encode their variant, so every effort is `auto`), Codex takes a real
# reasoning-effort token per agent, so both fields are asserted per row.
for triple in \
  "adr gpt-5.6-terra xhigh" \
  "auto-groom gpt-5.6-sol low" \
  "auto-groom-critic gpt-5.6-sol medium" \
  "brainstorm-consultant gpt-5.6-sol medium" \
  "build-economy gpt-5.6-luna xhigh" \
  "build-standard gpt-5.6-terra high" \
  "build-premium gpt-5.6-sol medium" \
  "finalize-change gpt-5.6-terra high" \
  "implement-next gpt-5.6-sol medium" \
  "integration-repair gpt-5.6-sol high" \
  "rebase-resolver gpt-5.6-sol high" \
  "status gpt-5.6-luna xhigh" ; do
  set -- $triple
  assert "codex/$1 = $2/$3" \
    '[ "$(hd_field "$HD" codex "'"$1"'" model)/$(hd_field "$HD" codex "'"$1"'" effort)" = "'"$2"'/'"$3"'" ]'
done
# The three build profiles are the settled ladder for this change, asserted separately from the
# loop above so a reader sees the claim the change is actually making.
assert "codex build ladder = luna/xhigh, terra/high, sol/medium" \
  '[ "$(hd_field "$HD" codex build-economy model)/$(hd_field "$HD" codex build-economy effort)" = "gpt-5.6-luna/xhigh" ] &&
   [ "$(hd_field "$HD" codex build-standard model)/$(hd_field "$HD" codex build-standard effort)" = "gpt-5.6-terra/high" ] &&
   [ "$(hd_field "$HD" codex build-premium model)/$(hd_field "$HD" codex build-premium effort)" = "gpt-5.6-sol/medium" ]'
# Sparse-by-harness is still a live property of the reader — it just no longer has a shipped
# harness to demonstrate it on. Narrowed (not deleted) to a token that genuinely holds no block:
# what this guards is that hd_field returns EMPTY for an absent harness rather than falling through
# to another block's row.
assert "a harness with no block resolves empty (sparse-by-harness read)" \
  '[ -z "$(hd_field "$HD" windsurf status model)" ] && [ -z "$(hd_agents "$HD" windsurf)" ]'
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `bash tests/test_harness_defaults.sh`
Expected: FAIL — thirteen new codex asserts fail (`hd_field` returns empty) and the harness-set assert fails (`claude cursor ` != `claude codex cursor `).

- [ ] **Step 3: Add the Codex block to the sidecar**

Append to `agents/harness-defaults.yml`, after the `cursor:` block. Keep the file's flow-map style and bare scalars:

```yaml

  # Codex's full wrapper set (change 0169). Codex takes a real reasoning-effort token per agent,
  # so unlike the cursor block every row carries a distinct model/effort PAIR rather than `auto`.
  #
  # The profile names describe the capability/cost role of a complete pair, NOT a cross-model
  # ordinal: reasoning tokens are model-specific settings, so `xhigh` on Luna is not "stronger
  # than" `medium` on Sol. The build ladder is therefore Luna/xhigh for positively established
  # low-risk work, Terra/high for ordinary work, and Sol/medium for named risk or the one allowed
  # escalation.
  codex:
    adr:                   { model: gpt-5.6-terra, effort: xhigh }
    auto-groom:            { model: gpt-5.6-sol, effort: low }
    auto-groom-critic:     { model: gpt-5.6-sol, effort: medium }
    brainstorm-consultant: { model: gpt-5.6-sol, effort: medium }
    build-economy:         { model: gpt-5.6-luna, effort: xhigh }
    build-standard:        { model: gpt-5.6-terra, effort: high }
    build-premium:         { model: gpt-5.6-sol, effort: medium }
    finalize-change:       { model: gpt-5.6-terra, effort: high }
    implement-next:        { model: gpt-5.6-sol, effort: medium }
    integration-repair:    { model: gpt-5.6-sol, effort: high }
    rebase-resolver:       { model: gpt-5.6-sol, effort: high }
    status:                { model: gpt-5.6-luna, effort: xhigh }
```

Also update the file's header rule so it no longer names only two harnesses. Find:

```
#   - the claude and cursor blocks are both COMPLETE (each key set equals agents/docket-*.md).
```

Replace with:

```
#   - every harness listed in HD_SHIPPED_HARNESSES carries a COMPLETE block (its key set equals
#     agents/docket-*.md) — claude, cursor, and codex today.
```

- [ ] **Step 4: Arm the completeness gate**

In `scripts/lib/harness-defaults.sh`, find:

```bash
# Harnesses docket actually SHIPS defaults for. Each must carry a COMPLETE block (every
# agents/docket-*.md). A known-but-unshipped harness (codex today, until change 0169) may hold no
# block at all — but the moment it holds one, listing it here is what makes partial coverage an
# error rather than a silent half-pinned harness.
HD_SHIPPED_HARNESSES="claude cursor"
```

Replace with:

```bash
# Harnesses docket actually SHIPS defaults for. Each must carry a COMPLETE block (every
# agents/docket-*.md). A known-but-unshipped harness — one listed in HD_KNOWN_HARNESSES but absent
# here — may hold no block at all; the moment it holds one, listing it here is what makes partial
# coverage an error rather than a silent half-pinned harness. All three known harnesses ship
# complete blocks today.
HD_SHIPPED_HARNESSES="claude cursor codex"
```

- [ ] **Step 5: Run the harness-defaults test to verify it passes**

Run: `bash tests/test_harness_defaults.sh`
Expected: PASS.

- [ ] **Step 6: Derive the completeness mutations per shipped harness**

The existing mutation legs are hand-written per harness with value-specific `sed` patterns (`claude-haiku`, `cursor-grok`), which does not extend and re-enumerates what `HD_SHIPPED_HARNESSES` already knows. Replace them with a derived loop. In `tests/test_harness_defaults.sh`, find the two blocks starting at the comment `# Completeness is now enforced for BOTH shipped harnesses` and ending with the `cursor_gap_diag` assert, and replace the whole span with:

```bash
# Completeness is enforced for EVERY shipped harness, so the mutation is derived from
# HD_SHIPPED_HARNESSES rather than written once per harness with a value-specific pattern — adding
# a fourth shipped harness arms this loop for free. Deleting the row from ONE block only is the
# point: a bare `/^    status:/d` would delete it from all three at once and go green whichever leg
# actually fired, unable to tell a working per-harness rule from one that was never written.
del_entry(){ # $1=harness $2=agent -> writes $T/hd.yml
  awk -v h="$1" -v a="$2" '
    { nc=$0; sub(/#.*/,"",nc) }
    nc ~ "^  "h"[[:space:]]*:[[:space:]]*$" { inb=1; print; next }
    inb && nc ~ /^  [A-Za-z0-9._-]+[[:space:]]*:/ { inb=0 }
    inb && nc ~ "^    "a"[[:space:]]*:" { next }
    { print }
  ' "$HD" > "$T/hd.yml"
}
for h in $HD_SHIPPED_HARNESSES; do
  del_entry "$h" status
  # Non-vacuity: prove the mutation actually landed on THIS block and left the others intact.
  # Without this, a del_entry that silently matched nothing would leave every assert below green.
  assert "mutation landed: $h lost exactly one entry" \
    '[ "$(hd_agents "$T/hd.yml" "'"$h"'" | grep -c .)" = "$(( $(hd_agents "$HD" "'"$h"'" | grep -c .) - 1 ))" ]'
  assert "reject: missing a $h entry" '! hd_validate "$T/hd.yml" "$SRC" 2>/dev/null'
  gap_diag="$(hd_validate "$T/hd.yml" "$SRC" 2>&1 || true)"
  assert "reject: the $h gap is reported against $h" \
    'grep -q "'"$h"' block is incomplete — no entry for .status." <<<"$gap_diag"'
done
```

- [ ] **Step 7: Run it and verify the derived mutations pass**

Run: `bash tests/test_harness_defaults.sh`
Expected: PASS, with three `mutation landed`, three `reject: missing`, and three `reject: … reported against` lines — one per shipped harness.

- [ ] **Step 8: Flip the Codex TOML absence asserts to sidecar-derived value asserts**

In `tests/test_sync_agents_codex.sh`, find the comment block ending in the two absence asserts:

```bash
assert "codex TOML: no model key — nothing shipped for codex, so honestly unpinned" \
  '! toml_has_key "$T" model'
assert "codex TOML: no model_reasoning_effort key either" \
  '! toml_has_key "$T" model_reasoning_effort'
```

Replace that comment and both asserts with (the values are read from the sidecar, never re-typed — a literal here would be a second copy of the shipped table that could drift):

```bash
# Change 0169 ships a complete codex block, so the honest output is a PINNED wrapper. These read
# the expected values from the sidecar rather than restating them: a literal here would be a second
# copy of the shipped table, free to drift from the one the generator actually reads.
#
# These are the two asserts change 0168 inverted to absence; they are inverted BACK rather than
# deleted, so their original job — catching a cross-harness leak, a Claude ID landing in a Codex
# wrapper — stays live. The leak now shows up as a value mismatch instead of as an unexpected key.
# shellcheck source=/dev/null
. "$REPO/scripts/lib/harness-defaults.sh"
HD="$REPO/agents/harness-defaults.yml"
assert "codex TOML: model is the shipped codex pin" \
  '[ -n "$(hd_field "$HD" codex status model)" ] &&
   [ "$(toml_get "$T" model)" = "$(hd_field "$HD" codex status model)" ]'
assert "codex TOML: model_reasoning_effort is the shipped codex effort" \
  '[ -n "$(hd_field "$HD" codex status effort)" ] &&
   [ "$(toml_get "$T" model_reasoning_effort)" = "$(hd_field "$HD" codex status effort)" ]'
# And no Codex wrapper carries a Claude-namespace model ID — the cross-harness leak, stated as the
# property rather than as one agent's value.
assert "codex TOML: model is not a claude-namespace ID" \
  '! grep -qE "^model[[:space:]]*=[[:space:]]*\"claude-" "$T"'
# Whole-set coverage: every one of the twelve generated wrappers matches its sidecar row. Population
# derived from the sidecar, so a thirteenth agent arms this loop automatically.
n_codex_checked=0
while IFS= read -r a; do
  [ -n "$a" ] || continue
  n_codex_checked=$((n_codex_checked+1))
  assert "codex TOML docket-$a: model + effort match the sidecar" \
    '[ "$(toml_get "$SBX/.codex/agents/docket-'"$a"'.toml" model)" = "$(hd_field "$HD" codex "'"$a"'" model)" ] &&
     [ "$(toml_get "$SBX/.codex/agents/docket-'"$a"'.toml" model_reasoning_effort)" = "$(hd_field "$HD" codex "'"$a"'" effort)" ]'
done < <(hd_agents "$HD" codex)
assert "codex TOML: every shipped codex entry was checked (floor 12; got $n_codex_checked)" \
  '[ "$n_codex_checked" -ge 12 ]'
```

- [ ] **Step 9: Rebuild the uncovered-pair warning fixture in `tests/test_sync_agents.sh`**

Once Codex ships a complete block, no *known* harness can produce an uncovered pair, so the existing fixture asserts a condition that can no longer arise. What the block **guards** — that an uncovered pair generates unpinned and says so — is still a real, reachable rule (it is what a fourth, unshipped harness would hit), so the fixture is rebuilt rather than deleted, on the repo-copy pattern this file already uses elsewhere.

Find the comment beginning `# The unpinned leg is driven by CODEX, not cursor:` and replace it and the fixture through the `rm -rf "$SBX" "$HROOT168W" "$HROOT168D"` line's first sandbox with:

```bash
# The unpinned leg can no longer be driven by any SHIPPED harness — claude, cursor, and codex all
# carry complete blocks since change 0169. What the rule guards is still reachable (it is what a
# newly-added, not-yet-mapped harness hits), so the fixture reconstructs that state in a throwaway
# copy of the repo rather than asserting a condition the shipped tree can no longer reach: drop
# codex from the copy's shipped list AND delete its block, which is exactly "known but unshipped".
make_sandbox
HROOT168W="$(mktemp -d)"; mkdir -p "$HROOT168W/.claude"
SCRW="$(mktemp -d)"; cp -R "$REPO/agents" "$REPO/cursor-rules" "$REPO/scripts" "$REPO/sync-agents.sh" "$SCRW/"
sed -i.bak 's/^HD_SHIPPED_HARNESSES="\(.*\) codex"$/HD_SHIPPED_HARNESSES="\1"/' "$SCRW/scripts/lib/harness-defaults.sh"
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
  '[ "$(hd_agents "$SCRW/agents/harness-defaults.yml" cursor | grep -c .)" = "12" ]'
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
rm -rf "$SCRW"
# Complement, on the REAL tree: because codex now ships complete, a shipped harness draws no
# unpinned warning at all. This is the property change 0169 actually adds, and it is what would
# redden if the codex block were dropped or left partial.
make_sandbox
HROOT169S="$(mktemp -d)"; mkdir -p "$HROOT169S/.claude"
printf 'agent_harnesses: [claude, cursor, codex]\n' > "$SBX/.docket.yml"
w169="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT169S" bash "$SYNC" 2>&1 >/dev/null)"
assert "0169: a complete codex block silences the whole harness" \
  '! grep -qF "WARN codex/" <<<"$w169"'
rm -rf "$SBX" "$HROOT169S"
```

Keep the `agents.default` amendment fixture (`HROOT168D`) that follows unchanged; adjust only the trailing `rm -rf` list so every temp dir this step created is removed exactly once.

- [ ] **Step 10: Sharpen the runner-provenance guard for the Codex direction**

Before change 0169 the "a shipped native default never becomes a runner flag" assert could pass **vacuously in the Codex direction**, because the Codex sidecar held nothing that could have leaked. Now it holds twelve entries, so the property is worth stating sharply. In `tests/test_sync_agents.sh`, immediately after the assert `"0168: runner-only shim bakes NO --effort flag"`, add:

```bash
# 0169: the Codex sidecar now supplies a model for this very agent, so the negative asserts above
# stopped being vacuous in the direction that matters — a `runner: codex` shim could now bake a
# real Codex ID if provenance were ignored. Pin the fixture's premise so a future emptying of the
# codex block cannot quietly re-vacuum them.
assert "0169: the codex sidecar really does supply a model for this agent (the guard above is not vacuous)" \
  '[ -n "$(hd_field "$HD" codex status model)" ]'
assert "0169: and the shipped CODEX default did not leak into the runner flags either" \
  '! grep -qF -- "$(hd_field "$HD" codex status model)" "$S"'
```

- [ ] **Step 11: Run all three affected tests**

Run: `bash tests/test_harness_defaults.sh && bash tests/test_sync_agents_codex.sh && bash tests/test_sync_agents.sh`
Expected: PASS for all three.

- [ ] **Step 12: Mutation-test the new guards**

Run each mutation, confirm the named test reddens, then restore. Confirm each mutation actually landed before trusting its result.

```bash
# (a) remove one codex entry -> completeness + value asserts redden
cp agents/harness-defaults.yml /tmp/hd.bak
sed -i.bak '/^    build-premium:.*gpt-5\.6-sol/d' agents/harness-defaults.yml
grep -c 'build-premium' agents/harness-defaults.yml   # must drop by 1 vs the backup
bash tests/test_harness_defaults.sh; echo "expect FAIL: $?"
cp /tmp/hd.bak agents/harness-defaults.yml

# (b) phantom codex entry -> validator rejects
cp agents/harness-defaults.yml /tmp/hd.bak
printf '    phantom-not-a-wrapper: { model: gpt-5.6-sol, effort: low }\n' >> agents/harness-defaults.yml
bash tests/test_harness_defaults.sh; echo "expect FAIL: $?"
cp /tmp/hd.bak agents/harness-defaults.yml

# (c) wrong codex value -> the TOML value asserts redden (proves they read the generator, not a literal)
cp agents/harness-defaults.yml /tmp/hd.bak
sed -i.bak 's/^    status:                { model: gpt-5\.6-luna, effort: xhigh }/    status:                { model: gpt-5.6-sol, effort: low }/' agents/harness-defaults.yml
grep -E '^    status:.*gpt' agents/harness-defaults.yml   # must show the mutated row
bash tests/test_sync_agents_codex.sh; echo "expect FAIL: $?"
cp /tmp/hd.bak agents/harness-defaults.yml

# (d) de-ship codex -> the "complete block silences the harness" complement reddens
cp scripts/lib/harness-defaults.sh /tmp/lib.bak
sed -i.bak 's/^HD_SHIPPED_HARNESSES="claude cursor codex"$/HD_SHIPPED_HARNESSES="claude cursor"/' scripts/lib/harness-defaults.sh
bash tests/test_sync_agents.sh; echo "expect FAIL: $?"
cp /tmp/lib.bak scripts/lib/harness-defaults.sh

# (e) restore green
bash tests/test_harness_defaults.sh && bash tests/test_sync_agents_codex.sh && bash tests/test_sync_agents.sh
echo "expect PASS: $?"
rm -f agents/*.bak scripts/lib/*.bak /tmp/hd.bak /tmp/lib.bak
```

- [ ] **Step 13: Commit**

```bash
git add agents/harness-defaults.yml scripts/lib/harness-defaults.sh \
        tests/test_harness_defaults.sh tests/test_sync_agents_codex.sh tests/test_sync_agents.sh
git commit -m "feat(0169): ship a complete codex default block and arm its completeness gate"
```

---

### Task 2: The committed `AGENTS.md` dispatch block tells the truth, and keeps telling it

The dispatch block is **committed into consumer repos** and checked by `sync-agents.sh --check`, so a false claim in it ships. Its current head says docket ships no validated Codex model IDs — false the moment Task 1 lands.

The guard over it is written `if [ "$n_codex_shipped" = "0" ]; then …; fi`, so Task 1 switches it off silently and leaves the new claim unguarded. **This is not hypothetical:** the Cursor twin of this guard (`tests/test_cursor_dispatch_rule.sh`, gated on `n_cursor_pinned -lt n_src`) retired itself when change 0168 completed the Cursor block, and `cursor-rules/dispatch.head.md` still carries the now-false claim that Cursor ships IDs "for the three build-profile workers only". That is a live defect on `main` and is recorded in Task 6 for capture as separate follow-up work — this task's job is to make sure the Codex side does not repeat it.

**Files:**
- Modify: `sync-agents.sh`
- Test: `tests/test_sync_agents_codex.sh`

**Interfaces:**
- Consumes: `hd_agents "$HD" codex` is non-empty (Task 1).
- Produces: no new symbols.

- [ ] **Step 1: Write the failing guard — give the retiring `if` an `else` arm**

In `tests/test_sync_agents_codex.sh`, find the block that opens with the comment `# 0168 whole-branch review, IMPORTANT 4.` and ends at the closing `fi` of `if [ "$n_codex_shipped" = "0" ]; then`. Replace the whole `if` (keeping the sourcing of the library and `n_codex_shipped` above it) with:

```bash
if [ "$n_codex_shipped" = "0" ]; then
  assert "agentsmd: makes no blanket 'ships pinned agent definitions' claim while the sidecar ships no codex pins" \
    '! grep -qiE "ships model/effort-pinned agent definitions" "$A"'
  assert "agentsmd: says an unconfigured codex agent runs UNPINNED" 'grep -qi "unpinned" "$A"'
  assert "agentsmd: still requires the dispatch regardless of the pin" \
    'grep -qi "either way" "$A"'
else
  # Change 0169 shipped the codex block, so the premise above is false and that arm no longer runs.
  # A guard that merely switches off leaves its NEW truth unguarded — which is exactly how the
  # cursor dispatch head kept a stale "ships IDs for the three build profiles only" claim after
  # change 0168 completed the cursor block. So the else arm asserts the post-0169 claim just as
  # hard: the block must no longer call an unconfigured Codex agent unpinned, and must still
  # require the dispatch for a reason that survives the pin.
  assert "agentsmd: no longer claims an unconfigured codex agent runs unpinned" \
    '! grep -qi "unpinned" "$A"'
  assert "agentsmd: no longer promises validated IDs are still to come" \
    '! grep -qiE "ships no validated|no validated codex|change 0169" "$A"'
  assert "agentsmd: states the dispatch is required for reasons beyond the pin" \
    'grep -qi "either way" "$A"'
  assert "agentsmd: still carries NO model id (machine-neutral even now that pins exist)" \
    '! grep -qE "claude-|gpt-|model_reasoning_effort|model[[:space:]]*=" "$A"'
fi
# Population floor: without this, an emptied codex block would take the else arm out of service and
# BOTH arms would be satisfied by whichever branch happened to run. Anchored on the source glob so a
# thirteenth wrapper does not redden it.
n_src_codex=0
for f in "$REPO"/agents/docket-*.md; do [ -e "$f" ] || continue; n_src_codex=$((n_src_codex+1)); done
assert "agentsmd: the pinned-premise branch is the live one (codex ships $n_codex_shipped of $n_src_codex)" \
  '[ "$n_codex_shipped" = "$n_src_codex" ] && [ "$n_src_codex" -ge 12 ]'
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `bash tests/test_sync_agents_codex.sh`
Expected: FAIL — the `else` arm now runs and the generated `AGENTS.md` still says "unpinned" and "Docket ships no validated Codex model IDs today".

- [ ] **Step 3: Rewrite the dispatch block's head prose**

In `sync-agents.sh`, find inside `assemble_agents_md_dispatch`'s heredoc:

```
preload, and whatever model and reasoning effort your config layers pin for it. Docket ships no
validated Codex model IDs today, so an unconfigured Codex agent runs **unpinned**, at Codex's own
default — dispatch to the agent either way. Pass the request through unchanged, including any
change or ADR id.
```

Replace with:

```
preload, and whatever model and reasoning effort your config layers pin for it. Docket ships a
validated Codex model and reasoning effort for every one of these agents, so they are pinned out of
the box; your config layers override either field per agent. Dispatch to the agent either way — the
pin is not the only reason, since the agent also carries the skill's dispatch contract and preload.
Pass the request through unchanged, including any change or ADR id.
```

- [ ] **Step 4: Update the two comments that explain the old premise**

Above `assemble_agents_md_dispatch`, find:

```
# The head deliberately does NOT claim these definitions are pinned. This block is COMMITTED into
# consumer repos and checked by `--check`, so a false claim here ships. Since change 0168 the
# default store is agents/harness-defaults.yml, which carries no codex entries: every generated
# Codex wrapper is unpinned until the user configures one, or until change 0169 lands validated
# IDs. The dispatch is required either way — the agent carries the skill's contract and preload,
# not just a model — so the rationale is stated in terms that stay true when a pin IS configured.
# Guarded, against the sidecar rather than a literal, in tests/test_sync_agents_codex.sh.
```

Replace with:

```
# This block is COMMITTED into consumer repos and checked by `--check`, so a false claim here ships
# rather than merely displaying. The default store is agents/harness-defaults.yml, whose codex block
# is complete since change 0169, so the head states the pinned truth. The dispatch is required
# either way — the agent carries the skill's contract and preload, not just a model — so the
# rationale stays true however the pin resolves. Guarded, against the sidecar rather than a literal,
# in tests/test_sync_agents_codex.sh, which asserts the claim in BOTH directions so completing or
# emptying the codex block cannot leave this prose unchecked.
```

Then find the line near the Codex TOML emitter reading `# wrapper is honestly UNPINNED and Codex applies its own default.` and leave it as-is **only if** its surrounding sentence is conditional (it describes what happens for a pair with no entry, which remains true for an unmapped harness). Read the full sentence first; if it asserts unconditionally that Codex wrappers are unpinned, reword it to the conditional form: an unmapped pair is unpinned; the shipped codex block covers all twelve today.

- [ ] **Step 5: Run to verify it passes**

Run: `bash tests/test_sync_agents_codex.sh`
Expected: PASS.

- [ ] **Step 6: Verify the generated block by eye and confirm `--check` currency**

```bash
SBX=$(mktemp -d); HR=$(mktemp -d); mkdir -p "$HR/.claude"
printf 'agent_harnesses: [claude, codex]\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HR" bash "$PWD/sync-agents.sh" >/dev/null 2>&1 )
sed -n '/docket:dispatch:start/,/docket:dispatch:end/p' "$SBX/AGENTS.md" | head -20
rm -rf "$SBX" "$HR"
```

Expected: the head reads as the pinned truth, and no model ID appears anywhere in the block.

- [ ] **Step 7: Mutation-test the new `else` arm**

```bash
# Re-introduce the stale word in the generated head; the else arm must redden.
cp sync-agents.sh /tmp/sync.bak
sed -i.bak 's/so they are pinned out of/so they are unpinned out of/' sync-agents.sh
grep -c 'unpinned out of' sync-agents.sh    # must be 1 — the mutation landed
bash tests/test_sync_agents_codex.sh; echo "expect FAIL: $?"
cp /tmp/sync.bak sync-agents.sh
bash tests/test_sync_agents_codex.sh; echo "expect PASS: $?"
rm -f sync-agents.sh.bak /tmp/sync.bak
```

- [ ] **Step 8: Commit**

```bash
git add sync-agents.sh tests/test_sync_agents_codex.sh
git commit -m "feat(0169): the committed AGENTS.md dispatch block states the pinned truth, guarded both ways"
```

---

### Task 3: Promote `.docket.example.yml`'s Codex block to a singly commented exact mirror

**Files:**
- Modify: `.docket.example.yml`
- Test: `tests/test_docket_example_yml.sh`

**Interfaces:**
- Consumes: the complete `codex:` sidecar block (Task 1).
- Produces: an example whose three harness blocks all sit at the same single-comment level, mirroring the sidecar value-for-value, and which resolves through the real generator into Claude, Cursor, **and** Codex wrappers.

The mirror guard's harness population and each block's slice terminator both become **derived from `HD_SHIPPED_HARNESSES` and the sidecar**, not from a literal `claude cursor codex` list. This was prototyped during planning: the derived terminator `build-premium:.*<ERE-escaped sidecar build-premium model>` isolates all three blocks correctly and all 36 comparisons pass.

- [ ] **Step 1: Write the failing asserts — comment level, derived population, derived terminators**

In `tests/test_docket_example_yml.sh`, find and replace the comment-level assert. Find:

```bash
assert "codex example is doubly commented (docket ships no codex defaults)" \
  'grep -Eq "^#[[:space:]]+#[[:space:]]*codex:[[:space:]]*$" "$EX"'
```

Replace with:

```bash
# Change 0169 shipped the codex block, so codex joins claude and cursor as a MIRROR and sits at the
# same single-comment level. The doubly-commented level is asserted ABSENT rather than the assert
# being deleted: an accidental second '#' would silently demote a shipped mirror back to an
# illustration, which is the exact regression this pair of asserts exists to catch.
assert "codex example is singly commented, like claude and cursor (all three mirror the sidecar)" \
  'grep -Eq "^#[[:space:]]+codex:[[:space:]]*$" "$EX" && ! grep -Eq "^#[[:space:]]+#[[:space:]]*codex:[[:space:]]*$" "$EX"'
assert "no doubly-commented harness block survives under agents:" \
  '! sed -n "/^# agents:$/,/^runners:$/p" "$EX" | grep -Eq "^#[[:space:]]+#[[:space:]]*[a-z]+:[[:space:]]*$"'
```

Then replace the two hand-listed mirror loops. Find the block that begins:

```bash
# Terminators are the two build-premium rows, which differ by VALUE (`claude-opus-5,` vs
# `claude-opus-5-high`) — the only place the two blocks' text diverges enough to anchor on.
claude_slice="$(ex_slice claude 'build-premium:.*claude-opus-5,')"
cursor_slice="$(ex_slice cursor 'build-premium:.*claude-opus-5-high')"
```

…and everything through the end of the second `for h in claude cursor` loop (the one ending with the `every shipped $h entry was checked (floor 12; …)` assert). Replace the whole span with:

```bash
# Population AND terminator are both derived — from HD_SHIPPED_HARNESSES and from the sidecar's own
# build-premium row. A literal `claude cursor codex` list here would be a fourth restatement of what
# the shipped set already knows, and it is precisely a hand-maintained harness list that let a stale
# claim survive elsewhere in this repo. Adding a fourth shipped harness arms these loops for free.
#
# Each block's terminator is its own build-premium MODEL, which is what makes the three ranges
# independent: every agent key appears in all three blocks, so a key-only anchor would resolve every
# lookup to whichever block came first in the file.
ere_escape(){ sed -E 's/[][\.^$*+?(){}|]/\\&/g' <<<"$1"; }
for h in $HD_SHIPPED_HARNESSES; do
  bp_model="$(hd_field "$HD" "$h" build-premium model)"
  assert "$h mirror: the sidecar supplies a build-premium model to anchor the slice on" '[ -n "$bp_model" ]'
  slice="$(ex_slice "$h" "build-premium:.*$(ere_escape "$bp_model")")"
  # Terminator guard: an unclosed sed range silently runs to EOF, pulling in neighbouring blocks and
  # surrounding prose, while every assert below stays green on the over-wide slice. Pinning the
  # slice's FIRST and LAST lines catches both over-run and under-run. First/last taken by parameter
  # expansion, not `printf | head -n1`: under this file's `set -o pipefail` a producer feeding an
  # early-exiting consumer takes SIGPIPE and turns the assert into an intermittent 141.
  first="${slice%%$'\n'*}"; first="${first#"${first%%[![:space:]]*}"}"
  last="${slice##*$'\n'}"
  assert "$h mirror: the $h slice was isolated and terminates at its build-premium anchor" \
    '[ -n "$slice" ] && [ "$first" = "'"$h"':" ] && grep -q "build-premium:" <<<"$last"'
  mirrored=0
  while IFS= read -r a; do
    [ -n "$a" ] || continue
    mirrored=$((mirrored+1))
    assert "$h/$a: wrapper exists" '[ -f "$REPO/agents/docket-'"$a"'.md" ]'
    assert "$h/$a: model mirrors the shipped sidecar" \
      '[ -n "$(ex_slice_field "$slice" "'"$a"'" model)" ] &&
       [ "$(ex_slice_field "$slice" "'"$a"'" model)" = "$(hd_field "$HD" '"$h"' "'"$a"'" model)" ]'
    assert "$h/$a: effort mirrors the shipped sidecar" \
      '[ -n "$(ex_slice_field "$slice" "'"$a"'" effort)" ] &&
       [ "$(ex_slice_field "$slice" "'"$a"'" effort)" = "$(hd_field "$HD" '"$h"' "'"$a"'" effort)" ]'
  done < <(hd_agents "$HD" "$h")
  assert "$h mirror: every shipped $h entry was checked (floor 12; got $mirrored)" \
    '[ "$mirrored" -ge 12 ]'
done
# Floor on the POPULATION itself, not only on each block's row count: an emptied HD_SHIPPED_HARNESSES
# would make the whole loop above run zero times with every assert trivially satisfied.
n_shipped="$(printf '%s\n' $HD_SHIPPED_HARNESSES | grep -c .)"
assert "mirror: at least three harnesses were mirrored (got $n_shipped)" '[ "$n_shipped" -ge 3 ]'
```

- [ ] **Step 2: Run it to confirm it fails**

Run: `bash tests/test_docket_example_yml.sh`
Expected: FAIL — the codex block is still doubly commented, so its slice does not isolate and its rows do not mirror the sidecar (the three build rows also still hold the illustrative Sol values).

- [ ] **Step 3: Promote the Codex block in `.docket.example.yml`**

Make four edits inside the commented `agents:` excerpt.

(a) Update the framing paragraph above the block. Find:

```
# The `claude:` and `cursor:` blocks below MIRROR docket's shipped built-in defaults — the values
# in agents/harness-defaults.yml — shown so the otherwise-invisible defaults are visible and
# tunable. Deleting any line falls back to the SAME shipped default. The sidecar is the single
# source of truth; this mirror never leads, and tests/test_docket_example_yml.sh enforces the
# equality. If a shipped default changes, update this mirror to match. The `codex:` block is
# NOT a mirror — docket ships no codex defaults yet (change 0169), so it is carried one comment
# level deeper as unvalidated illustration.
```

Replace with:

```
# The `claude:`, `codex:`, and `cursor:` blocks below all MIRROR docket's shipped built-in defaults
# — the values in agents/harness-defaults.yml — shown so the otherwise-invisible defaults are
# visible and tunable. Deleting any line falls back to the SAME shipped default. The sidecar is the
# single source of truth; this mirror never leads, and tests/test_docket_example_yml.sh enforces the
# equality. If a shipped default changes, update this mirror to match. All three shipped harnesses
# are complete — every one of the twelve agents carries a model and an effort under each.
```

(b) Replace the codex block's own header comment. Find:

```
#   # To enable the codex block: verify the example IDs against Codex's current models, strip BOTH comment levels, and add `codex` to `agent_harnesses` above. The IDs here are UNVALIDATED examples.
```

Replace with:

```
#   # The codex block — SHIPPED defaults (change 0169). Codex takes a real reasoning-effort token
#   # per agent, so every row carries a distinct model/effort pair rather than cursor's `auto`.
#   # Add `codex` to `agent_harnesses` above to generate the wrappers.
```

(c) Strip the second comment layer from the twelve codex rows and the `codex:` header, so they sit at the same level as `claude:` / `cursor:` — `#   codex:` and `#     <agent>: { … }`.

(d) Delete the intra-block build-profile comment (`#   #   # build profiles — … unvalidated examples (change 0169 lands validated mappings)`) and set the three build rows to the shipped values. The resulting codex block must read:

```
#   codex:
#     status:                { model: gpt-5.6-luna, effort: xhigh }
#     adr:                   { model: gpt-5.6-terra, effort: xhigh }
#     brainstorm-consultant: { model: gpt-5.6-sol, effort: medium }
#     auto-groom:            { model: gpt-5.6-sol, effort: low }
#     auto-groom-critic:     { model: gpt-5.6-sol, effort: medium }
#     implement-next:        { model: gpt-5.6-sol, effort: medium }
#     rebase-resolver:       { model: gpt-5.6-sol, effort: high }
#     integration-repair:    { model: gpt-5.6-sol, effort: high }
#     finalize-change:       { model: gpt-5.6-terra, effort: high }
#     build-economy:         { model: gpt-5.6-luna, effort: xhigh }
#     build-standard:        { model: gpt-5.6-terra, effort: high }
#     build-premium:         { model: gpt-5.6-sol, effort: medium }
```

The whole `agents:` surface stays **singly** commented — the key is presence-sensitive and must not ship active.

- [ ] **Step 4: Run to verify the mirror passes**

Run: `bash tests/test_docket_example_yml.sh`
Expected: PASS. If the codex slice assert fails, the second-layer strip missed a row — check for a stray `#   #`.

- [ ] **Step 5: Update the resolver round-trip to cover Codex**

The round-trip's two-stage strip exists only because codex and cursor used to sit a level deeper than claude. All three are now at the same level, so stage 1 alone uncomments everything and stage 2 must go — leaving it in place would corrupt the now-singly-commented rows.

Find the comment paragraph beginning `# Within the slice, codex:/cursor: are DOUBLY commented` and the `stage1` / `stage2` assignments, and replace them with:

```bash
# Since change 0169 all three harness blocks sit at the SAME single-comment level, so one strip
# uncomments agents:, its three harness blocks, and all thirty-six rows. (Before 0169 codex and
# cursor sat a level deeper and needed a second, block-scoped strip; that stage is gone with the
# asymmetry it existed for.)
stage2="$(printf '%s\n' "$agents_block" | sed -E 's/^#[[:space:]]?//')"
```

Extend the harness list and assert Codex evidence. Find:

```bash
harnesses_line="$(printf '%s' "$harnesses_line" | sed -E 's/\[claude\]/[claude, cursor]/')"
```

Replace with:

```bash
harnesses_line="$(printf '%s' "$harnesses_line" | sed -E 's/\[claude\]/[claude, cursor, codex]/')"
```

Add `"$SB/.codex/agents"` to the `mkdir -p` line in the same section. Then, after the existing cursor round-trip assert, add:

```bash
# Codex evidence (change 0169): the example's codex rows must survive the REAL generator into real
# Codex TOML, which is what proves they are executable YAML rather than text that merely happens to
# match the sidecar reader. Read from the generated wrapper, compared against the sidecar.
CT="$SB/.codex/agents/docket-status.toml"
assert "round-trip: a codex wrapper was generated" '[ -f "$CT" ]'
assert "round-trip: codex status model came from the example block" \
  '[ -n "$(hd_field "$HD" codex status model)" ] &&
   [ "$(sed -nE "s/^model[[:space:]]*=[[:space:]]*\"(.*)\"[[:space:]]*$/\1/p" "$CT")" = "$(hd_field "$HD" codex status model)" ]'
assert "round-trip: codex status effort came from the example block" \
  '[ "$(sed -nE "s/^model_reasoning_effort[[:space:]]*=[[:space:]]*\"(.*)\"[[:space:]]*$/\1/p" "$CT")" = "$(hd_field "$HD" codex status effort)" ]'
assert "round-trip: the codex build profiles resolve to their shipped ladder" \
  '[ "$(sed -nE "s/^model[[:space:]]*=[[:space:]]*\"(.*)\"[[:space:]]*$/\1/p" "$SB/.codex/agents/docket-build-economy.toml")" = "gpt-5.6-luna" ] &&
   [ "$(sed -nE "s/^model[[:space:]]*=[[:space:]]*\"(.*)\"[[:space:]]*$/\1/p" "$SB/.codex/agents/docket-build-standard.toml")" = "gpt-5.6-terra" ] &&
   [ "$(sed -nE "s/^model[[:space:]]*=[[:space:]]*\"(.*)\"[[:space:]]*$/\1/p" "$SB/.codex/agents/docket-build-premium.toml")" = "gpt-5.6-sol" ]'
```

Note: the existing slice end-anchor (`finalize-change:.*cursor-grok-4\.5-high-fast`) still terminates the whole `agents:` excerpt correctly because the cursor block remains last in the file; the existing terminator guard above it stays and will catch it if that ever changes.

- [ ] **Step 6: Run to verify the round-trip passes**

Run: `bash tests/test_docket_example_yml.sh`
Expected: PASS, including the four new round-trip asserts.

- [ ] **Step 7: Mutation-test the example guards**

```bash
cp .docket.example.yml /tmp/ex.bak

# (a) change one example value -> the mirror assert reddens
sed -i.bak 's/^#     build-standard:         { model: gpt-5\.6-terra, effort: high }/#     build-standard:         { model: gpt-5.6-sol, effort: high }/' .docket.example.yml
grep -c 'build-standard:.*gpt-5.6-sol' .docket.example.yml   # must be 1 — mutation landed
bash tests/test_docket_example_yml.sh; echo "expect FAIL: $?"
cp /tmp/ex.bak .docket.example.yml

# (b) restore the second comment layer on codex -> the comment-level assert reddens
sed -i.bak -E '/^#   codex:/,/^#     build-premium:/ s/^#(   )/#  #/' .docket.example.yml
grep -cE '^#  #' .docket.example.yml   # must be >0 — mutation landed
bash tests/test_docket_example_yml.sh; echo "expect FAIL: $?"
cp /tmp/ex.bak .docket.example.yml

bash tests/test_docket_example_yml.sh; echo "expect PASS: $?"
rm -f .docket.example.yml.bak /tmp/ex.bak
```

If mutation (b)'s `sed` does not land (check the `grep -c`), apply the second layer by hand to two rows instead — the requirement is that the mutation demonstrably occurred, not that this particular `sed` works.

- [ ] **Step 8: Commit**

```bash
git add .docket.example.yml tests/test_docket_example_yml.sh
git commit -m "feat(0169): promote the codex example block to a singly commented exact mirror"
```

---

### Task 4: Maintained documentation describes three shipped harnesses

The spec requires the documentation set to be **derived from a whole-repository grep**, not hand-listed. Point-in-time records (archived changes, results, prior specs and plans, Accepted ADR bodies) are explicitly excluded — rewriting them falsifies history.

**Files:**
- Modify: `README.md`
- Modify: `docs/codex/setup.md`
- Modify: `skills/docket-build/SKILL.md`
- Test: `tests/test_skill_size_budgets.sh` (verify only — budgets must still pass)

**Interfaces:**
- Consumes: everything from Tasks 1–3.
- Produces: no new symbols.

- [ ] **Step 1: Derive the site list**

```bash
grep -rniE "unpinned|ships no|no codex|until change 0169|Claude profile agent" \
  --include="*.md" --include="*.sh" --include="*.yml" . \
  | grep -viE "^\./docs/(changes|results)/" \
  | grep -viE "^\./docs/superpowers/(plans|specs)/"
```

Sort every hit into **prose to update** vs **already-true conditional prose**. A sentence saying "an unmapped pair generates unpinned" stays — that rule is unchanged and still reachable. Only claims that Codex *specifically* ships nothing, or that a mapping is still to come, are false now.

- [ ] **Step 2: Update the README's build-role paragraph**

Find the paragraph beginning `**`docket-build` ships validated model IDs for Claude Code and Cursor.**` and replace it with:

```
**`docket-build` ships validated model IDs for Claude Code, Cursor, and Codex.** Every shipped default lives in [`agents/harness-defaults.yml`](agents/harness-defaults.yml), indexed by harness, and all three are complete — twelve agents each, the three build profiles among them — so any of the three harnesses gets profile-routed builds with no configuration at all. Codex takes a real reasoning-effort token per agent, so its rows carry a model/effort pair where Cursor's IDs encode their variant and use `auto`. A harness docket does not yet map generates **unpinned**, letting that harness apply its own default rather than inherit an ID that means nothing there. To retune any pair, set the model yourself in a config layer — `.docket.example.yml` mirrors all three shipped blocks value for value.
```

- [ ] **Step 3: Make the profile-worker prose harness-neutral**

The workers are named agents resolved by whichever harness is hosting; calling them "Claude profile agents" is now wrong on two of three harnesses.

In `skills/docket-build/SKILL.md`, in the frontmatter `description:`, change `named economy/standard/premium Claude profile agent` to `named economy/standard/premium profile agent`.

In `README.md`, find `Each task is routed to one of three named Claude profile agents` and change it to `Each task is routed to one of three named profile agents`.

In `README.md`, the opt-in paragraph naming `install.sh` and "Claude Code registers both only at process start" describes a Claude-specific setup step. Generalize its registration clause to name the hosting harness rather than Claude specifically, keeping the concrete `install.sh` remedy — every one of the three harnesses registers agent definitions at process start, which is why the restart instruction exists for all of them.

- [ ] **Step 4: Update `docs/codex/setup.md`**

Two places. First, the bullet that currently reads that docket ships **no Codex defaults** "until change 0169 lands validated IDs — so all twelve `.toml` wrappers are generated **unpinned**". Rewrite it to state that `agents/harness-defaults.yml` ships a complete twelve-agent `codex:` block, so every wrapper is generated pinned, and that any field can be overridden per agent from any config layer. Keep the surrounding facts that are unchanged: the wrappers are machine-local, gitignored, regenerated per machine, never committed (ADR-0020).

Second, in `## Verifying it works`, item 1 currently says an unconfigured agent correctly has no `model` line "until change 0169 ships Codex defaults". Rewrite it so the expected state is the shipped pin, and give the reader the concrete check:

```
1. `.codex/agents/docket-*.toml` exist and each carries the `model` / `model_reasoning_effort`
   docket ships for that agent — compare against the `codex:` block in
   `agents/harness-defaults.yml`, which is the single source of truth. A field you set in a config
   layer wins over the shipped value; a missing line means neither docket nor your config supplied
   one, and Codex will apply its own default.
```

Also fix the sentence in the same file stating that because docket "ships no Codex defaults yet, that config is the **only** source of a Codex pin" — the config layer is now an override, not the only source.

- [ ] **Step 5: Verify prose changes did not break their guards**

Per the restatement rule, grep the suite for the prose you changed rather than for the source it restates:

```bash
bash tests/test_readme_skill_catalog.sh
bash tests/test_readme_finalize_docs.sh
bash tests/test_skill_size_budgets.sh
bash tests/test_comment_anchor_style.sh
bash tests/test_cursor_dispatch_rule.sh
bash tests/test_composition_wiring.sh
```

Expected: PASS for all. `test_skill_size_budgets.sh` is the one most likely to redden — the `docket-build/SKILL.md` description edit is a net reduction, so it should not, but if a budget is exceeded, slim the prose rather than raising the number (raising it is an explicit, commented, in-diff decision, not a reflex).

- [ ] **Step 6: Confirm no stale promise survives**

```bash
grep -rniE "until change 0169|change 0169 lands|no codex defaults|ships no validated" \
  --include="*.md" --include="*.sh" --include="*.yml" . \
  | grep -viE "^\./docs/(changes|results)/" \
  | grep -viE "^\./docs/superpowers/(plans|specs)/"
```

Expected: no output. Any remaining hit is either a point-in-time record (leave it) or a miss (fix it).

- [ ] **Step 7: Commit**

```bash
git add README.md docs/codex/setup.md skills/docket-build/SKILL.md
git commit -m "docs(0169): three shipped harnesses, harness-neutral profile workers"
```

---

### Task 5: Whole-suite gate and the mutation-evidence sweep

**Files:**
- No production changes expected. Any fix this task surfaces is committed here.

- [ ] **Step 1: Run the whole suite**

There is no aggregate runner in this repo; each file is `bash tests/test_*.sh`. Run all of them and collect failures:

```bash
fails=""
for t in tests/test_*.sh; do
  out="$(bash "$t" 2>&1)" || true
  if ! printf '%s\n' "$out" | tail -n1 | grep -q '^PASS$'; then
    fails="$fails $t"
    printf '\n===== %s =====\n%s\n' "$t" "$(printf '%s\n' "$out" | grep -E '^NOT OK|^FAIL' | head -20)"
  fi
done
echo "FAILING: ${fails:-none}"
```

Expected: `FAILING: none`. Investigate every failure; do not narrow a guard to make it green without establishing what it guards.

- [ ] **Step 2: Re-check the new regexes under BSD grep**

PATH `grep` is ugrep and is more permissive than the portability target. Re-run the three tests carrying new patterns with `/usr/bin/grep` first on PATH:

```bash
PATH=/usr/bin:/bin:/usr/sbin:/sbin bash tests/test_harness_defaults.sh
PATH=/usr/bin:/bin:/usr/sbin:/sbin bash tests/test_sync_agents_codex.sh
PATH=/usr/bin:/bin:/usr/sbin:/sbin bash tests/test_docket_example_yml.sh
PATH=/usr/bin:/bin:/usr/sbin:/sbin bash tests/test_grep_portability.sh
```

Expected: PASS. A pattern that only works under ugrep is a real portability bug, not a test-environment quirk.

- [ ] **Step 3: Confirm the full mutation matrix**

Every mutation below must redden the named guard and only then be reverted. Confirm each mutation landed (a `grep -c` before/after) before trusting its result — a `sed` that silently matched nothing produces a false "the guard held" reading.

| # | Mutation | Must redden |
|---|---|---|
| 1 | Remove one `codex:` entry from the sidecar | `test_harness_defaults.sh` completeness + the codex value loop |
| 2 | Add a phantom `codex:` entry naming no wrapper | `test_harness_defaults.sh` validator rejection |
| 3 | Change one `.docket.example.yml` codex value | `test_docket_example_yml.sh` mirror equality |
| 4 | Restore the second comment layer on the codex example block | `test_docket_example_yml.sh` comment-level assert |
| 5 | Defeat the provenance predicate (bake a sidecar value into a runner flag) | `test_sync_agents.sh` runner-provenance asserts |
| 6 | Replace a generated-value assert with an absence assert | that assert fails against the now-pinned TOML |
| 7 | Drop `codex` from `HD_SHIPPED_HARNESSES` | `test_sync_agents.sh` "complete codex block silences the harness"; `test_docket_example_yml.sh` population floor |
| 8 | Re-introduce "unpinned" in the generated `AGENTS.md` head | `test_sync_agents_codex.sh` else-arm |

For (5), the predicate lives in `emit_wrapper`: change `[ "${RES_MODEL_FROM_USER:-0}" = "1" ] && flag_model="$2"` to assign unconditionally, run `bash tests/test_sync_agents.sh`, confirm the runner-provenance asserts redden, and revert.

- [ ] **Step 4: Confirm the working tree is clean of mutation debris**

```bash
git status --porcelain
find . -name "*.bak" -not -path "./.git/*"
```

Expected: no unexpected modifications and no `.bak` files.

- [ ] **Step 5: Commit any fixes**

```bash
git add -A
git commit -m "test(0169): whole-suite gate and mutation evidence"
```

If nothing needed fixing, skip the commit rather than creating an empty one.

---

### Task 6: Certification evidence and the results artifact

**Tier 2 is NOT executable inside this build, and this task records that honestly rather than quietly.** The determination was made against the installed CLI during planning, not assumed:

- `codex exec --help` exposes **no agent-selection flag**. A named profile dispatch can only be induced by a real, non-deterministic multi-task build inside a Codex session — it cannot be invoked directly.
- `codex debug` offers `models`, `app-server`, and `prompt-input` only. There is **no agent-registry command**, so there is no way to observe which model a named agent resolved to short of the session reporting it.
- The evidence the spec demands per profile — the controller's routing line, the observed named agent/model indicator, the structured worker outcome, focused verification, and the task commit — is session-observation evidence.

So this change follows the change 0168 precedent: Tier 1 ships autonomously and complete; Tier 2 becomes an explicit maintainer checklist with a recorded waiver. **The support claim is not certified until all three named dispatches are observed.** Do not write it up as certified, and do not let a green Tier 1 be presented as certification.

What *is* mechanically checkable is recorded automatically below.

**Files:**
- Create: `docs/results/2026-07-31-codex-profile-routed-build-support-results.md`

- [ ] **Step 1: Collect the automated certification evidence**

```bash
codex --version
codex debug models 2>/dev/null | python3 -c "
import json,sys
d=json.load(sys.stdin)
found={m['slug']:[e['effort'] for e in m.get('supported_reasoning_levels',[])] for m in d['models']}
for s in ('gpt-5.6-luna','gpt-5.6-terra','gpt-5.6-sol'):
    print(s, found.get(s,'MISSING'))
"
SBX=$(mktemp -d); HR=$(mktemp -d); mkdir -p "$HR/.claude"
printf 'agent_harnesses: [claude, codex]\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HR" bash "$PWD/sync-agents.sh" >/dev/null 2>&1 )
for f in "$SBX"/.codex/agents/docket-*.toml; do
  printf '%s: %s\n' "$(basename "$f" .toml)" \
    "$(grep -E '^(model|model_reasoning_effort)[[:space:]]*=' "$f" | tr '\n' ' ')"
done
rm -rf "$SBX" "$HR"
```

Paste the real output into the results file — this covers two of the spec's required recorded items (the installed catalog/version, and the generated TOML values for all twelve wrappers).

- [ ] **Step 2: Write the results file**

Create `docs/results/2026-07-31-codex-profile-routed-build-support-results.md` from `results-template.md`. It must contain, at minimum:

- A prominent **"Codex certification pending"** note at the top stating that Tier 2 is unproven and why an autonomous build could not execute it (no `--agent` flag on `codex exec`; no agent-registry debug command), and that the change should not be presented as certified until the checklist passes.
- The automated evidence from Step 1: Codex CLI version, the catalog's reported efforts for the three slugs, and the generated `model` / `model_reasoning_effort` for all twelve wrappers.
- A **`## Verify (human)`** checklist to run in a real Codex session after `bash sync-agents.sh` **and a restart** (Codex registers agent definitions at process start):

  - [ ] An explicit `economy` task dispatches to a named agent observed on `gpt-5.6-luna` / `xhigh`
  - [ ] An explicit `standard` task dispatches to a named agent observed on `gpt-5.6-terra` / `high`
  - [ ] An explicit `premium` task dispatches to a named agent observed on `gpt-5.6-sol` / `medium`

  Record for each: the controller's routing line, the observed named agent and model indicator, the structured worker outcome, the focused verification, and the task commit. A merely completed child is not success evidence — the controller must observe the worker's structured outcome.

- The **recorded waiver**, in the spec's own terms: automatic classification and the `NEEDS_ESCALATION` → single-retry path are deliberately **not** repeated live. They are defined in the harness-neutral `docket-build-task` contract and `docket-build`'s loop, loaded identically by all three profile workers; what is harness-specific is *dispatch*. Both paths have prior Claude evidence and hermetic coverage. The waiver is recorded explicitly so it can be reopened if Codex behavior later diverges.
- A **re-probe instruction**: query the catalog again immediately before certifying, and record the version then — the design forbids substituting a model if a slug has become unavailable; that is a stop-and-surface, not an in-implementation choice.

- [ ] **Step 3: Record the follow-up findings**

In the results file's follow-ups section, record both discoveries from this change's planning. Neither is in scope here — the first is Cursor's, which this change's *Out of scope* assigns to change 0168's lineage, and the second is a house-rule violation in a file this change does not otherwise touch:

1. **`cursor-rules/dispatch.head.md` carries a stale claim.** It still says docket "ships validated Cursor model IDs for the three build-profile workers only — … Every other wrapper is generated **unpinned**". Change 0168 completed the Cursor block (all twelve pinned), which made that false *and* silently retired the guard over it: `tests/test_cursor_dispatch_rule.sh` gates its head asserts on `if [ "$n_cursor_pinned" -lt "$n_src" ]`, and `12 < 12` is false. This head is catted verbatim into every consumer repo's `.cursor/rules/docket-dispatch.mdc`, so the false claim ships. The fix is the same shape as Task 2's: correct the prose and give the guard an `else` arm. Verified live during this change's planning.
2. **A line-number cross-reference in maintained source.** `scripts/lib/harness-defaults.sh`'s completeness comment says the reverse direction "is already enforced per-entry above (line 131)". AGENTS.md requires a symbol name or a verbatim-quoted clause; the prose "line N" form is unenforceable by `tests/test_comment_anchor_style.sh` and rots exactly where code moves most.

- [ ] **Step 4: Commit**

```bash
git add docs/results/2026-07-31-codex-profile-routed-build-support-results.md
git commit -m "docs(0169): results — Codex certification pending, automated evidence recorded"
```

---

## Self-review

**Spec coverage.** Each of the spec's ten Tier-1 properties maps to a task: (1) validates with Codex shipped → Task 1 Steps 3–5; (2) set equality both directions → Task 1's existing forward/reverse loops now cover codex via the shipped-harness completeness gate, plus Step 6's derived mutations; (3) generated TOML carries exact values → Task 1 Step 8; (4) build workers resolve to Luna/xhigh, Terra/high, Sol/medium → Task 1 Step 1's ladder assert and Task 3 Step 5's round-trip; (5) the two absence assertions become value assertions, not deleted → Task 1 Step 8, stated explicitly in its comment; (6) user layers still override field-by-field → covered by the existing `agents.default` / machine-local fixtures in `test_sync_agents.sh`, left intact and re-verified by the whole-suite gate; (7) foreign `agents.default` winner still warns and the artifact carries the named value → the `HROOT168D` fixture, explicitly preserved in Task 1 Step 9; (8) a shipped native default never becomes a runner flag → Task 1 Step 10; (9) the singly commented example mirrors every row and resolves through the real generator → Task 3; (10) existing controller tests stay green → Task 5's whole-suite gate. The spec's mutation-evidence list is Task 5 Step 3, item for item. Tier 2 is Task 6, settled explicitly rather than deferred.

**Placeholder scan.** No "TBD", no "add error handling", no "similar to Task N". Task 4 Steps 3–4 describe two prose rewrites by intent plus the exact sentences to find, rather than quoting full replacement paragraphs, because their surrounding text must be read in place to keep the paragraph coherent — the find-anchors and the required post-state are given exactly, and Step 6 is a mechanical check that no stale promise survived.

**Type consistency.** `hd_field "$HD" <harness> <agent> model|effort` and `hd_agents "$HD" <harness>` are used with the same signatures throughout. `HD` is bound to `$REPO/agents/harness-defaults.yml` in every test file that uses it (Task 1 Step 8 adds the binding to `test_sync_agents_codex.sh`, where it did not previously exist at that point in the file). `ex_slice` / `ex_slice_field` keep their existing two- and three-argument shapes; `ere_escape` and `del_entry` are new and defined at first use. `HD_SHIPPED_HARNESSES` is a space-separated string in every consumer, iterated unquoted.

**Ordering.** Task 1 is deliberately the largest because four assertions flip from true to false the instant the sidecar block lands; splitting them would leave the suite red between commits. Tasks 2–4 are additive and independently reviewable. Task 5 gates. Task 6 records.
