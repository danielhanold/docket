<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0192 — opencode support for profile-routed Docket builds](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-02-0192-opencode-profile-routed-build-support.md)**
<!-- docket:backlink:end -->

# opencode profile-routed build support — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Register `opencode` as docket's fourth first-class agent harness — known token, shipped sixteen-agent default block, a native emitter writing `.opencode/agents/docket-*.md`, and AGENTS.md dispatch — so a repo listing `opencode` in `agent_harnesses:` gets the complete profile-routed build ladder generated and pinned.

**Architecture:** Every insertion point already exists; change 0169 (Codex) walked the identical path. Four registration constants gain a token, one new emitter joins the `emit_for_harness` registry, the shared `AGENTS.md` managed block becomes harness-neutral so Codex and opencode share it, and the sidecar gains a complete `opencode:` block. No controller change: `docket-build` keeps dispatching `docket-build-economy` / `-standard` / `-premium` / `-max` by name and opencode resolves the name to the generated definition.

**Tech Stack:** POSIX-ish Bash (portability floor: BSD/macOS `/bin/sh` + `/usr/bin/grep`, `/usr/bin/sed`, `awk`), the repo's own 74-file `tests/test_*.sh` suite, opencode 1.18.11, OpenRouter model IDs.

## Global Constraints

- **Every model ID and effort token is an opaque bare scalar** (ADR-0015). Docket keeps no vendor allowlist. Values must be unquoted and space-free or `hd_validate` rejects them.
- **The sidecar is the single source of truth** for shipped pins. `.docket.example.yml` mirrors it and never leads.
- **`HD_SHIPPED_HARNESSES` membership means COMPLETE**: the block's key set must equal the `agents/docket-*.md` source set in both directions. Sparseness is per-harness, never per-field, never per-agent (learning: `commented-default-is-no-default`).
- **Never hand-list the sites of a literal** — derive a guard's population from `HD_SHIPPED_HARNESSES` or from the consuming code, never from an enumerated list (repo `AGENTS.md` rules; learning: `correspondence-guard-runs-one-way`).
- **Key a guard on syntactic shape, never an enumerated list of spellings** (repo `AGENTS.md`).
- **Mutation evidence is required** for every new guard: make the guard redden by breaking the thing it guards, and prove the mutation actually landed (learning: `assert-detects-removal-not-replacement`, `guards-are-code`).
- **Portability:** `/usr/bin/grep` is the oracle, not the PATH `grep` (which is ugrep and accepts extensions BSD grep rejects). `sed -i` needs the `.bak` form. No `grep -P`.
- **Run the whole suite at the build gate**, not just the touched files.
- **Do not change** `REGISTERED_RUNNERS`, the `docket-build` controller's routing, the shared task-worker contract, or the Claude/Cursor/Codex mappings.

## Verified environment facts (settled at reconcile — do NOT re-litigate)

These were probed against a real opencode **1.18.11** install on 2026-08-02 with `opencode debug agent <name>`, which prints the fully resolved agent config. Treat them as established; the learning `harness-behavior-is-mode-and-version-scoped` says record the version with the fact, so the version is named everywhere it is used.

Given `.opencode/agents/probe-one.md`:

```markdown
---
description: probe agent for docket effort passthrough
mode: subagent
model: openrouter/deepseek/deepseek-v4-flash-0731
reasoningEffort: high
---

You are a probe.
```

`opencode debug agent probe-one` returned (permissions elided):

```json
{
  "name": "probe-one",
  "mode": "subagent",
  "options": { "reasoningEffort": "high" },
  "native": false,
  "model": { "providerID": "openrouter", "modelID": "deepseek/deepseek-v4-flash-0731" },
  "prompt": "You are a probe.",
  "description": "probe agent for docket effort passthrough"
}
```

1. **The effort passthrough works and its spelling is `reasoningEffort`.** An unrecognized frontmatter key lands in `options` and is forwarded to the provider as a model option. This closes the spec's one open question; the emitter hardcodes `reasoningEffort`.
2. **The double-prefixed ID parses correctly**: `openrouter/deepseek/deepseek-v4-flash-0731` splits into `providerID: openrouter` + `modelID: deepseek/deepseek-v4-flash-0731`.
3. **`mode: subagent` is honored**, and the markdown body becomes the agent's `prompt`.
4. **`.opencode/agents/` (plural) is read.** (`.opencode/agent/` singular is also read; plural is chosen because it matches docket's uniform `.<token>/agents/` plumbing and therefore needs no special-casing anywhere.)
5. **The extension is `.md`**, so `harness_ext` needs **no** new branch — its `*)` default already returns `md`.
6. **Docket skills are already reachable from opencode with no change**: `opencode debug skill` resolves skills out of `~/.agents/skills/`, which `link-skills.sh` already populates via its existing `$HARNESS_ROOT/.agents/skills` entry. **Do not edit `link-skills.sh`.**
7. **All three selected OpenRouter model IDs exist verbatim** in the installed catalog (`opencode models`, 360 entries): `openrouter/deepseek/deepseek-v4-flash-0731`, `openrouter/moonshotai/kimi-k3`, `openrouter/openai/gpt-5.6-luna`.

**External-truth caveat (learning: `external-truth-needs-a-human-checkpoint`):** no in-repo test can be the oracle for a vendor model ID — every mirror assert compares the generated output against the sidecar that generated it, so both sides move together and the assert is green whether the ID is right or a typo. The catalog check above is the certification; it belongs in the results file as a named human verification item, not as a new assert. Do not write a test that "validates" a model ID.

## File Structure

| File | Responsibility | Action |
|---|---|---|
| `scripts/lib/docket-gitignore-block.sh` | canonical harness roster (`DOCKET_GI_HARNESS_TOKENS`), which also feeds `VALID_HARNESS_TOKENS` and the managed `.gitignore` block | Modify L9 |
| `scripts/lib/harness-defaults.sh` | `HD_KNOWN_HARNESSES`, `HD_SHIPPED_HARNESSES`, `hd_validate` | Modify L18–27 |
| `agents/harness-defaults.yml` | the shipped sixteen-row `opencode:` block | Modify header L15–16, append block |
| `sync-agents.sh` | `emit_opencode_md`, the emitter registry, `AGENTS_MD_DISPATCH_HARNESSES`, the harness-neutral dispatch prose | Modify |
| `.docket.example.yml` | the singly commented mirror block | Modify |
| `.gitignore` | regenerated managed block | Regenerate |
| `docs/opencode/setup.md` | new per-harness setup doc | Create |
| `README.md`, `skills/docket-convention/references/agent-layer.md` | maintained harness enumerations | Modify |
| `tests/test_sync_agents_opencode.sh` | the opencode emitter contract | Create |
| `tests/test_harness_defaults.sh`, `tests/test_sync_agents.sh`, `tests/test_docket_example_yml.sh`, `tests/test_docket_review.sh`, `tests/test_cursor_contract_docs.sh`, `tests/test_docket_build.sh`, `tests/test_docket_gitignore_block.sh`, `tests/test_dispatch_capability.sh` | existing guards: arm for opencode, and re-derive the literal populations | Modify |

**Do NOT touch:** `link-skills.sh` (fact 6), `REGISTERED_RUNNERS`, `scripts/runners/*`, the `docket-build` controller, the task-worker contract.

---

### Task 1: Register the harness and ship its complete sidecar block

Registration and the block must land together: adding `opencode` to `HD_SHIPPED_HARNESSES` makes `hd_validate` demand a complete block, so a token without a block fails generation atomically. Adding it to `DOCKET_GI_HARNESS_TOKENS` also regenerates the managed `.gitignore` block, which must be committed in the same task or `sync-agents.sh --check` reddens.

At the end of this task opencode is registered and shipped but still has **no named emitter**, so `emit_for_harness`'s `*)` branch would emit a Claude-shaped wrapper. That intermediate state is buildable and the suite is green; Task 2 closes it immediately (learning: `intermediate-task-state-buildable`).

**Files:**
- Modify: `scripts/lib/docket-gitignore-block.sh:9`
- Modify: `scripts/lib/harness-defaults.sh:18-27`
- Modify: `agents/harness-defaults.yml:15-16` and append a block after L103
- Modify: `.gitignore` (regenerated, not hand-edited)
- Test: `tests/test_harness_defaults.sh`, `tests/test_sync_agents.sh:1570-1603`, `tests/test_docket_gitignore_block.sh`

**Interfaces:**
- Consumes: nothing.
- Produces: the token `opencode` in `DOCKET_GI_HARNESS_TOKENS`, `HD_KNOWN_HARNESSES`, `HD_SHIPPED_HARNESSES`; a complete `agents.opencode` block in `agents/harness-defaults.yml` whose sixteen keys are `adr`, `auto-groom`, `auto-groom-critic`, `brainstorm-consultant`, `build-economy`, `build-standard`, `build-premium`, `build-max`, `finalize-change`, `implement-next`, `integration-repair`, `rebase-resolver`, `review-lean`, `review-standard`, `review-deep`, `status`.

- [ ] **Step 1: Update the harness-roster assert to include opencode (the failing test)**

`hd_harnesses` sorts, so the expected string is alphabetical. In `tests/test_harness_defaults.sh` replace L15–16:

```bash
assert "harnesses are exactly the four shipped ones" \
  '[ "$(hd_harnesses "$HD" | tr "\n" " ")" = "claude codex cursor opencode " ]'
```

- [ ] **Step 2: Run it to verify it fails**

Run: `"$DOCKET_BASH_PATH" tests/test_harness_defaults.sh 2>&1 | grep -c "^NOT OK"`
Expected: at least 1 `NOT OK` — the sidecar still holds only three harnesses.

- [ ] **Step 3: Add the token to the canonical roster**

In `scripts/lib/docket-gitignore-block.sh`, L9:

```bash
DOCKET_GI_HARNESS_TOKENS="claude codex cursor opencode agents kiro windsurf"
```

Place `opencode` immediately after `cursor` so the three docket-shipped harnesses stay grouped ahead of the merely-ignored tokens. No second literal line is needed in `emit_docket_gitignore_block` — the per-token loop already emits `.opencode/agents/docket-*.md`, and opencode's extension is `.md` (fact 5).

- [ ] **Step 4: Add the token to the sidecar registration constants**

In `scripts/lib/harness-defaults.sh`, L18–27, replace both constants and reword the roster comment:

```bash
# Known harness tokens — what may APPEAR in the sidecar at all. Adding one here is not enough to
# ship defaults for it; the emitter must also know how to write the harness's wrapper.
HD_KNOWN_HARNESSES="claude cursor codex opencode"

# Harnesses that must carry a COMPLETE block (key set equal to the wrapper sources in
# agents/docket-*.md). A known-but-unshipped harness — one listed in HD_KNOWN_HARNESSES but absent
# here — may hold no block at all; sparseness is a property of WHICH harnesses appear, never of how
# much of one appears. All four known harnesses ship complete blocks today.
HD_SHIPPED_HARNESSES="claude cursor codex opencode"
```

- [ ] **Step 5: Reword the sidecar header's roster line**

In `agents/harness-defaults.yml`, L15–16:

```yaml
#   - every harness listed in HD_SHIPPED_HARNESSES carries a COMPLETE block (its key set equals
#     agents/docket-*.md) — claude, cursor, codex, and opencode today.
```

- [ ] **Step 6: Append the complete opencode block**

Append to the end of `agents/harness-defaults.yml` (after the `codex:` block's last row, L103). Keep the column-26 value alignment and the codex block's key order:

```yaml

  # opencode's full wrapper set (change 0192). opencode has no first-class effort token; it passes
  # unrecognized agent-frontmatter keys through to the provider as model options, so docket emits
  # the effort as `reasoningEffort:` and it arrives as a real per-agent reasoning effort. Verified
  # against opencode 1.18.11 via `opencode debug agent`, which reports it under `options`.
  #
  # Models are reached through OpenRouter, so IDs are double-prefixed (`openrouter/<vendor>/<id>`);
  # the bare-scalar rule tolerates the slashes. The table is three models, chosen for very cheap
  # high intelligence: DeepSeek V4 Flash carries every volume row (sweep, grooming, orchestration,
  # the economy and standard build rungs, the lean review rung), Kimi K3 carries the judgment rows
  # — design prose, ADRs, merge-intent reconstruction, red-suite repair — and the ladder top,
  # appearing at two efforts exactly as Sol does in the codex block, and GPT-5.6 Luna is one
  # deliberate diversity row so the adversarial critic does not share the drafter's blind spots.
  #
  # As in every other block the PAIR is the role, not the model: `high` on Flash is not "stronger
  # than" `medium` on Kimi. review-deep equals build-max so the cap rung never reviews below the
  # strength the riskiest build work was built with. Efforts cap at `high` — reasoning-effort
  # vocabularies are model-specific and xhigh-class tokens are not assumed portable here.
  opencode:
    adr:                   { model: openrouter/moonshotai/kimi-k3, effort: medium }
    auto-groom:            { model: openrouter/deepseek/deepseek-v4-flash-0731, effort: medium }
    auto-groom-critic:     { model: openrouter/openai/gpt-5.6-luna, effort: high }
    brainstorm-consultant: { model: openrouter/moonshotai/kimi-k3, effort: medium }
    build-economy:         { model: openrouter/deepseek/deepseek-v4-flash-0731, effort: medium }
    build-standard:        { model: openrouter/deepseek/deepseek-v4-flash-0731, effort: high }
    build-premium:         { model: openrouter/moonshotai/kimi-k3, effort: medium }
    build-max:             { model: openrouter/moonshotai/kimi-k3, effort: high }
    finalize-change:       { model: openrouter/deepseek/deepseek-v4-flash-0731, effort: high }
    implement-next:        { model: openrouter/deepseek/deepseek-v4-flash-0731, effort: high }
    integration-repair:    { model: openrouter/moonshotai/kimi-k3, effort: high }
    rebase-resolver:       { model: openrouter/moonshotai/kimi-k3, effort: high }
    review-lean:           { model: openrouter/deepseek/deepseek-v4-flash-0731, effort: high }
    review-standard:       { model: openrouter/moonshotai/kimi-k3, effort: medium }
    review-deep:           { model: openrouter/moonshotai/kimi-k3, effort: high }
    status:                { model: openrouter/deepseek/deepseek-v4-flash-0731, effort: low }
```

- [ ] **Step 7: Add verbatim-value asserts for the opencode block**

The claude/cursor/codex blocks each have a verbatim pair loop in `tests/test_harness_defaults.sh` (L19–36, L40–57, L59–78). Add the fourth, after the codex loop, matching their existing shape:

```bash
# ---- every shipped opencode value, verbatim ---------------------------------
for pair in \
  "adr openrouter/moonshotai/kimi-k3 medium" \
  "auto-groom openrouter/deepseek/deepseek-v4-flash-0731 medium" \
  "auto-groom-critic openrouter/openai/gpt-5.6-luna high" \
  "brainstorm-consultant openrouter/moonshotai/kimi-k3 medium" \
  "build-economy openrouter/deepseek/deepseek-v4-flash-0731 medium" \
  "build-standard openrouter/deepseek/deepseek-v4-flash-0731 high" \
  "build-premium openrouter/moonshotai/kimi-k3 medium" \
  "build-max openrouter/moonshotai/kimi-k3 high" \
  "finalize-change openrouter/deepseek/deepseek-v4-flash-0731 high" \
  "implement-next openrouter/deepseek/deepseek-v4-flash-0731 high" \
  "integration-repair openrouter/moonshotai/kimi-k3 high" \
  "rebase-resolver openrouter/moonshotai/kimi-k3 high" \
  "review-lean openrouter/deepseek/deepseek-v4-flash-0731 high" \
  "review-standard openrouter/moonshotai/kimi-k3 medium" \
  "review-deep openrouter/moonshotai/kimi-k3 high" \
  "status openrouter/deepseek/deepseek-v4-flash-0731 low" \
; do
  set -- $pair
  assert "opencode/$1 model is $2"  '[ "$(hd_field "$HD" opencode '"$1"' model)" = "'"$2"'" ]'
  assert "opencode/$1 effort is $3" '[ "$(hd_field "$HD" opencode '"$1"' effort)" = "'"$3"'" ]'
done

# The build ladder, stated as the pairs it actually is. Flash carries the two volume rungs at
# different efforts and Kimi carries the two judgment rungs at different efforts — the pair is the
# role, not the model.
assert "opencode build ladder: economy is Flash/medium" \
  '[ "$(hd_field "$HD" opencode build-economy model)" = "openrouter/deepseek/deepseek-v4-flash-0731" ] && [ "$(hd_field "$HD" opencode build-economy effort)" = "medium" ]'
assert "opencode build ladder: standard is Flash/high" \
  '[ "$(hd_field "$HD" opencode build-standard model)" = "openrouter/deepseek/deepseek-v4-flash-0731" ] && [ "$(hd_field "$HD" opencode build-standard effort)" = "high" ]'
assert "opencode build ladder: premium is Kimi/medium" \
  '[ "$(hd_field "$HD" opencode build-premium model)" = "openrouter/moonshotai/kimi-k3" ] && [ "$(hd_field "$HD" opencode build-premium effort)" = "medium" ]'
assert "opencode build ladder: max is Kimi/high" \
  '[ "$(hd_field "$HD" opencode build-max model)" = "openrouter/moonshotai/kimi-k3" ] && [ "$(hd_field "$HD" opencode build-max effort)" = "high" ]'

# The slash-bearing ID is the first double-prefixed value any block ships. Pin that the bare-scalar
# reader returns it WHOLE rather than clipping at the slash — hd_field's value class is
# [^,}[:space:]]+, so this is the assert that would catch a future narrowing to an allowlist.
assert "opencode: a double-prefixed ID survives the bare-scalar read intact" \
  '[ "$(hd_field "$HD" opencode status model)" = "$(hd_field_raw "$HD" opencode status model)" ] && case "$(hd_field "$HD" opencode status model)" in */*/*) true;; *) false;; esac'
```

- [ ] **Step 8: Fix the `HD_SHIPPED_HARNESSES` strip fixture in test_sync_agents.sh**

`tests/test_sync_agents.sh:1581` strips the last token with a `codex`-anchored pattern that no longer matches once `opencode` is appended. Re-anchor it on the token it actually means to remove rather than on final position:

```bash
sed -i.bak 's/^HD_SHIPPED_HARNESSES="\(.*\)codex\(.*\)"$/HD_SHIPPED_HARNESSES="\1\2"/' "$fx/scripts/lib/harness-defaults.sh"
```

Then normalize any doubled or trailing space the substitution can leave:

```bash
sed -i.bak2 's/HD_SHIPPED_HARNESSES="\(.*\)"/HD_SHIPPED_HARNESSES="\1"/; s/  */ /g; s/ "$/"/' "$fx/scripts/lib/harness-defaults.sh"
```

The fixture's own sanity asserts (L1587–1592) verify the strip landed; leave them, they are the mutation evidence that this edit is correct.

- [ ] **Step 9: Add the gitignore-pattern assert**

In `tests/test_docket_gitignore_block.sh`, beside the existing `.codex/...` asserts (L23–24):

```bash
assert "block ignores generated opencode agent definitions" \
  'printf "%s\n" "$BLK" | grep -F -x -q -- ".opencode/agents/docket-*.md"'
```

- [ ] **Step 10: Regenerate .gitignore**

Run: `bash sync-agents.sh` from the repo root, then confirm the managed block gained the line:

Run: `/usr/bin/grep -F -x -- '.opencode/agents/docket-*.md' .gitignore`
Expected: the line prints. Do not hand-edit `.gitignore`.

- [ ] **Step 11: Verify the validator accepts the new block and mutation-test completeness**

Run: `"$DOCKET_BASH_PATH" tests/test_harness_defaults.sh`
Expected: all `ok -`, no `NOT OK`.

Then prove the completeness guard actually covers opencode — delete one row and confirm it reddens:

```bash
cp agents/harness-defaults.yml /tmp/hd.bak
sed -i.bak '/^    status:.*deepseek-v4-flash-0731, effort: low }$/d' agents/harness-defaults.yml
bash -c 'source scripts/lib/harness-defaults.sh; hd_validate agents/harness-defaults.yml agents' ; echo "rc=$?"
cp /tmp/hd.bak agents/harness-defaults.yml
```

Expected: a diagnostic naming `opencode block is incomplete — no entry for 'status'` and `rc=1`. Confirm the file is restored before continuing.

- [ ] **Step 12: Run the three touched test files**

Run: `for t in tests/test_harness_defaults.sh tests/test_sync_agents.sh tests/test_docket_gitignore_block.sh; do "$DOCKET_BASH_PATH" "$t" | grep "^NOT OK"; done`
Expected: no output.

- [ ] **Step 13: Commit**

```bash
git add scripts/lib/docket-gitignore-block.sh scripts/lib/harness-defaults.sh agents/harness-defaults.yml .gitignore tests/test_harness_defaults.sh tests/test_sync_agents.sh tests/test_docket_gitignore_block.sh
git commit -m "feat(0192): register opencode as a shipped harness with a complete sidecar block"
```

---

### Task 2: The opencode emitter

opencode must get its **own named emitter**. Falling through `emit_for_harness`'s `*)` branch is exactly how the Cursor defect (change 0135) shipped: the token inherits Claude's frontmatter and docket reports pins the harness never reads.

**Files:**
- Modify: `sync-agents.sh` (emitter registry ~L652-664; new function after `emit_cursor_md`, ~L771)
- Modify: `skills/docket-convention/references/agent-layer.md:107-116` (the wrapper-shape table)
- Test: `tests/test_sync_agents_opencode.sh` (create), `tests/test_cursor_contract_docs.sh`

**Interfaces:**
- Consumes: Task 1's registration and sidecar block.
- Produces: `emit_opencode_md <src-md> <model> <effort>` writing an opencode agent definition on stdout; an `opencode` arm in `emit_for_harness`; a `| opencode |` row in the agent-layer table.

- [ ] **Step 1: Write the failing emitter contract test**

Create `tests/test_sync_agents_opencode.sh`, modeled on `tests/test_sync_agents_codex.sh`. Header and fixture:

```bash
#!/usr/bin/env bash
# tests/test_sync_agents_opencode.sh — the opencode emitter contract (change 0192).
# opencode agent definitions are markdown with YAML frontmatter in .opencode/agents/.
# Verified against opencode 1.18.11: `mode: subagent` is honored, an unrecognized frontmatter key
# is forwarded to the provider under `options`, and a double-prefixed OpenRouter model ID parses
# into providerID + modelID. See docs/opencode/setup.md.
set -u
fail=0
assert(){ if eval "$2"; then echo "ok - $1"; else echo "NOT OK - $1"; fail=1; fi; }

REPO="$(cd "$(dirname "$0")/.." && pwd)"
HD="$REPO/agents/harness-defaults.yml"
# shellcheck source=/dev/null
. "$REPO/scripts/lib/harness-defaults.sh"

WORK="$(mktemp -d "${TMPDIR:-/tmp}/docket-oc-XXXXXX")"
trap 'rm -rf "$WORK"' EXIT

mk_opencode_repo(){  # $1=dest
  mkdir -p "$1"
  git -C "$1" init -q 2>/dev/null || true
  printf 'agent_harnesses: [opencode]\n' > "$1/.docket.yml"
}

R="$WORK/repo"
mk_opencode_repo "$R"
( cd "$R" && DOCKET_HARNESS_ROOT="$WORK/home" bash "$REPO/sync-agents.sh" >"$WORK/gen.log" 2>&1 ) || true

D="$R/.opencode/agents"
assert "opencode wrappers are generated as .md" '[ "$(ls "$D"/docket-*.md 2>/dev/null | wc -l | tr -d " ")" = "16" ]'
```

Then the shape asserts, driven off the sidecar rather than literals:

```bash
# Every generated definition carries the sidecar's exact resolved pair. Population is derived from
# the sidecar, with a floor so a broken read cannot pass with an empty loop.
n=0
while IFS= read -r a; do
  [ -n "$a" ] || continue
  f="$D/docket-$a.md"
  m="$(hd_field "$HD" opencode "$a" model)"
  e="$(hd_field "$HD" opencode "$a" effort)"
  assert "opencode/$a: definition exists"    '[ -f "$f" ]'
  assert "opencode/$a: mode is subagent"     'grep -qx "mode: subagent" "$f"'
  assert "opencode/$a: model is $m"          'grep -qx "model: '"$m"'" "$f"'
  assert "opencode/$a: effort is $e"         'grep -qx "reasoningEffort: '"$e"'" "$f"'
  assert "opencode/$a: has a description"    'grep -q "^description: ." "$f"'
  assert "opencode/$a: carries no claude-shaped effort key" '! grep -qx "effort: '"$e"'" "$f"'
  n=$((n+1))
done < <(hd_agents "$HD" opencode)
assert "opencode: the per-agent loop covered the whole block" '[ "$n" -ge 16 ]'
```

The `carries no claude-shaped effort key` assert is the one that would have caught the 0135 defect: a Claude-shaped fallback emits `effort:`, which opencode does not read.

- [ ] **Step 2: Run it to verify it fails**

Run: `"$DOCKET_BASH_PATH" tests/test_sync_agents_opencode.sh 2>&1 | grep -c "^NOT OK"`
Expected: many `NOT OK` — no emitter exists, so wrappers are Claude-shaped (`effort:` present, `mode:` absent).

- [ ] **Step 3: Write the emitter**

Insert into `sync-agents.sh` immediately after `emit_cursor_md` ends (~L771), before the `emit_wrapper` comment block:

```bash
# Transform a built-in markdown wrapper into an opencode agent definition on stdout (change 0192).
# opencode agents are markdown with YAML frontmatter under .opencode/agents/; the FILENAME is the
# agent identifier, so no `name:` field is emitted. Field mapping (ADR-0015 verbatim passthrough):
#   frontmatter description: -> description
#   (constant)               -> mode: subagent      every docket agent is dispatched, never primary
#   resolved model           -> model               (omit if empty/inherit)
#   resolved effort          -> reasoningEffort     (omit if empty/auto)
#   skills: preload + body   -> a body preamble + the body verbatim
#
# opencode has NO first-class reasoning-effort field. It forwards unrecognized agent-frontmatter
# keys to the provider as model options, so `reasoningEffort` is a real per-agent effort rather
# than a decorative key: verified against opencode 1.18.11, where `opencode debug agent <name>`
# reports it as `options.reasoningEffort`. That is also why effort is dropped when no model
# resolves — a provider option with no provider selected has nothing to reach.
#
# Docket keeps NO allowlist of opencode model IDs or effort tokens (ADR-0015). IDs reached through
# OpenRouter are double-prefixed (`openrouter/<vendor>/<model>`); opencode splits that into a
# providerID and a modelID itself, so docket passes the whole string through untouched.
emit_opencode_md(){  # $1=src md  $2=model  $3=effort   (both FINAL resolved values)
  local src="$1" mo="$2" eo="$3"
  local desc model effort skills_csv body
  desc="$(agent_description "$src")"
  # change 0168: FINAL resolved values (shipped sidecar ⊕ user layers). The source frontmatter is
  # no longer a default store, so an unresolved field means the wrapper is honestly UNPINNED and
  # opencode applies its own default.
  model="$mo"
  effort="$eo"
  # Normalize the two "no pin" sentinels to empty, so the emit logic below has one shape to test.
  # `inherit` is a real Claude Code frontmatter value with no opencode equivalent, so it normalizes
  # here exactly as it does in emit_cursor_md/emit_codex_toml rather than passing through.
  [ "$model" = "inherit" ] && model=""
  [ "$effort" = "auto" ] && effort=""
  skills_csv="$(sed -n '/^skills:/{s/^skills:[[:space:]]*//;p;q;}' "$src" | sed -e 's/^\[//' -e 's/\][[:space:]]*$//' -e 's/[[:space:]]*$//')"
  # body = everything after the frontmatter closing --- , leading blank lines trimmed.
  body="$(awk '/^---[[:space:]]*$/ && d<2 {d++; next} d>=2 {print}' "$src" | awk 'NF{p=1} p{print}')"
  printf -- '---\n'
  printf 'description: %s\n' "$desc"
  printf 'mode: subagent\n'
  if [ -n "$model" ]; then
    printf 'model: %s\n' "$model"
    [ -n "$effort" ] && printf 'reasoningEffort: %s\n' "$effort"
  elif [ -n "$effort" ]; then
    log "WARN opencode/docket-$(short_name "$src"): effort '$effort' dropped — opencode carries effort as a provider model option, and no model is resolved (either none is configured or it is the 'inherit' sentinel). Set an explicit model to pin effort on opencode."
  fi
  printf -- '---\n\n'
  if [ -n "$skills_csv" ]; then
    printf 'Before acting, load these docket skills from your opencode skills directory: %s.\n\n' "$skills_csv"
  fi
  printf '%s\n' "$body"
}
```

- [ ] **Step 4: Wire it into the registry**

In `sync-agents.sh`, `emit_for_harness` (~L652-664), add the arm above the `claude` arm:

```bash
emit_for_harness(){  # $1=src md  $2=harness  $3=model  $4=effort
  case "$2" in
    codex)    emit_codex_toml "$1" "$3" "$4";;
    cursor)   emit_cursor_md  "$1" "$3" "$4";;
    opencode) emit_opencode_md "$1" "$3" "$4";;
    claude)   emit            "$1" "$3" "$4";;
    # The generic Claude-shaped wrapper. A harness reaching this branch has NO verified contract
    # mapping — its wrapper is a best guess, not a supported shape. Adding a harness token here
    # without a named emitter is how the Cursor defect (change 0135) shipped: the token inherited
    # Claude's frontmatter, and docket reported pins the harness never read. Give a new harness its
    # own emitter, or accept that its wrapper is unverified.
    *)        emit            "$1" "$3" "$4";;
  esac
}
```

`harness_ext` needs **no** change: its `*)` default already returns `md` (fact 5).

- [ ] **Step 5: Add the agent-layer table row**

`tests/test_cursor_contract_docs.sh` derives its reverse direction from the emitter case-branch shape, so a named emitter with no doc row reddens. In `skills/docket-convention/references/agent-layer.md`, add a row to the per-harness wrapper-shape table (L107–116), matching the existing columns:

```markdown
| opencode | `.opencode/agents/docket-<name>.md` | `model:` (`openrouter/<vendor>/<id>`) | `reasoningEffort:` (a provider model option, not a first-class field) | body preamble |
```

- [ ] **Step 6: Add the forward doc-row assert**

In `tests/test_cursor_contract_docs.sh`, replace the literal population at L17–19 with one derived from the shipped set, so the next harness arms it for free:

```bash
# One row per harness with a named emitter. Population derived from HD_SHIPPED_HARNESSES rather
# than a literal list, so a newly shipped harness cannot land without a doc row
# (correspondence-guard-runs-one-way: anchor on the consuming code, never an allowlist).
. "$REPO/scripts/lib/harness-defaults.sh"
n_rows=0
for h in $HD_SHIPPED_HARNESSES; do
  assert "agent-layer: table has a $h row" 'grep -qE "^\| *'"$h"' *\|" "$AL"'
  n_rows=$((n_rows+1))
done
assert "agent-layer: the row loop covered every shipped harness" '[ "$n_rows" -ge 4 ]'
```

- [ ] **Step 7: Add the row-scoped opencode encoding assert**

Beside the existing cursor row-scoped assert, add opencode's, so the `reasoningEffort` claim is pinned to the opencode row rather than satisfied by the word appearing elsewhere in the file:

```bash
assert "agent-layer: opencode row shows the reasoningEffort passthrough" \
  'grep -qE "^\| *opencode *\|.*reasoningEffort" "$AL"'
```

- [ ] **Step 8: Run the tests to verify they pass**

Run: `"$DOCKET_BASH_PATH" tests/test_sync_agents_opencode.sh; "$DOCKET_BASH_PATH" tests/test_cursor_contract_docs.sh`
Expected: all `ok -`, no `NOT OK`.

- [ ] **Step 9: Mutation-test the emitter guard**

Prove the contract test actually detects a Claude-shaped fallback. Temporarily remove the `opencode)` arm from `emit_for_harness`, re-run, restore:

```bash
cp sync-agents.sh /tmp/sa.bak
sed -i.bak '/^    opencode) emit_opencode_md /d' sync-agents.sh
"$DOCKET_BASH_PATH" tests/test_sync_agents_opencode.sh 2>&1 | grep -c "^NOT OK"
cp /tmp/sa.bak sync-agents.sh
```

Expected: a non-zero `NOT OK` count (the `mode: subagent` and `reasoningEffort:` asserts fail, and `carries no claude-shaped effort key` fails). Confirm `sync-agents.sh` is restored and the test is green again before continuing.

- [ ] **Step 10: Verify against the real opencode installation**

This is the fact-2/fact-3 re-probe at the point the emitter exists. Generate into a scratch repo and ask opencode to resolve one definition:

```bash
T="$(mktemp -d)"; mkdir -p "$T"; cd "$T"; git init -q
printf 'agent_harnesses: [opencode]\n' > .docket.yml
bash /Users/homer/dev/docket/sync-agents.sh
opencode debug agent docket-build-economy 2>&1 | python3 -c "import json,sys; d=json.load(sys.stdin); d.pop('permission',None); print(json.dumps({k:d.get(k) for k in ('name','mode','model','options','description')}, indent=2))"
```

Expected: `mode` is `subagent`, `model` is `{"providerID": "openrouter", "modelID": "deepseek/deepseek-v4-flash-0731"}`, `options` is `{"reasoningEffort": "medium"}`. Record this output verbatim — it is the Tier-2 certification evidence for the economy rung and belongs in the results file. If it does not match, STOP and surface it: that is design drift, never a silent substitution.

- [ ] **Step 11: Commit**

```bash
git add sync-agents.sh skills/docket-convention/references/agent-layer.md tests/test_sync_agents_opencode.sh tests/test_cursor_contract_docs.sh
git commit -m "feat(0192): native opencode emitter writing .opencode/agents definitions"
```

---

### Task 3: Share the AGENTS.md dispatch block between Codex and opencode

opencode reads the same committed project-root `AGENTS.md` Codex reads. Joining `AGENTS_MD_DISPATCH_HARNESSES` makes the existing block serve both — but the block's prose currently names `.codex/agents/docket-*.toml` and "a validated Codex model", and three log/diagnostic strings say "codex". Once the block is shared those become false, which matters because this block is **committed into consumer repos** (ADR-0036: committed, machine-neutral).

The generalization is prose-only: the write-vs-strip logic, the `--check` staleness leg, and the machine-neutrality invariant carry over unchanged. A repo targeting either harness gets the block; a repo targeting both gets it once.

**Files:**
- Modify: `sync-agents.sh:245-253` (`AGENTS_MD_DISPATCH_HARNESSES` + comment), `:859-894` (`assemble_agents_md_dispatch`), `:896-913` (`sync_codex_agents_md_dispatch` log strings), `:1100` (`--check` message)
- Test: `tests/test_sync_agents_codex.sh`, `tests/test_sync_agents_opencode.sh`

**Interfaces:**
- Consumes: Task 2's emitter (the block is written by the same per-repo pass).
- Produces: `AGENTS_MD_DISPATCH_HARNESSES="codex opencode"`; harness-neutral block prose.

- [ ] **Step 1: Write the failing tests**

Append to `tests/test_sync_agents_opencode.sh`:

```bash
# --- the shared AGENTS.md dispatch block (change 0192) -----------------------
# opencode reads the same committed project-root AGENTS.md that Codex reads, so the single managed
# block serves both harnesses. It is COMMITTED into consumer repos, so a false claim here ships.
A="$R/AGENTS.md"
assert "opencode-only repo gets the AGENTS.md dispatch block" '[ -f "$A" ] && grep -q "docket:dispatch:start" "$A"'
assert "block lists every wrapper source" \
  '[ "$(grep -c "^- \*\*docket-" "$A")" = "16" ]'
# Machine-neutrality (ADR-0036): the committed block must carry no model IDs.
assert "block carries no model IDs" \
  '! grep -qE "openrouter/|gpt-5|claude-opus|kimi-k3|deepseek" "$A"'
# Harness-neutral prose: with the block shared, naming ONE harness's artifact path is a false claim
# in the other harness's repo. Anchor on shape — the block must not hardcode either harness's
# generated-file path in its head prose.
assert "block prose is harness-neutral about the generated path" \
  '! grep -qE "\.codex/agents/docket-\*\.toml|\.opencode/agents/docket-\*\.md" "$A"'
assert "block prose names the hosting harness generically" \
  'grep -qi "hosting harness" "$A"'

# De-list: dropping the only AGENTS.md-dispatch harness strips the block.
R2="$WORK/repo2"; mk_opencode_repo "$R2"
( cd "$R2" && DOCKET_HARNESS_ROOT="$WORK/home2" bash "$REPO/sync-agents.sh" >/dev/null 2>&1 ) || true
assert "block present before de-list" 'grep -q "docket:dispatch:start" "$R2/AGENTS.md"'
printf 'agent_harnesses: [claude]\n' > "$R2/.docket.yml"
( cd "$R2" && DOCKET_HARNESS_ROOT="$WORK/home2" bash "$REPO/sync-agents.sh" >/dev/null 2>&1 ) || true
assert "block stripped when no AGENTS.md-dispatch harness is targeted" \
  '! grep -q "docket:dispatch:start" "$R2/AGENTS.md" 2>/dev/null'
```

- [ ] **Step 2: Run to verify they fail**

Run: `"$DOCKET_BASH_PATH" tests/test_sync_agents_opencode.sh 2>&1 | grep "^NOT OK"`
Expected: the block asserts fail — `opencode` is not in `AGENTS_MD_DISPATCH_HARNESSES`, so no `AGENTS.md` is written at all.

- [ ] **Step 3: Add opencode to the dispatch harnesses**

In `sync-agents.sh`, L245–246:

```bash
# Codex and opencode both read a committed project-root AGENTS.md; they share the single managed
# dispatch block (changes 0077, 0192). A repo targeting either gets it; targeting both gets it once.
AGENTS_MD_DISPATCH_HARNESSES="codex opencode"
```

- [ ] **Step 4: Generalize the block prose**

In `sync-agents.sh`, replace the `HEAD` heredoc in `assemble_agents_md_dispatch` (L871–882) with harness-neutral prose. Do not name either harness's artifact path — the block is shared:

```bash
  cat <<'HEAD'
## Docket agents — dispatch, don't run inline

Docket generates an agent definition per docket skill in your harness's own agents directory. When
you are asked to run one of the docket skills below, run the matching **agent** instead of executing
the skill inline at the session model: the agent carries that skill's dispatch contract, its skill
preload, and whatever model and reasoning effort your config layers pin for it. Docket ships a
validated model and reasoning effort for every one of these agents on every harness it supports, so
they are pinned out of the box; your config layers override either field per agent. Dispatch through
the hosting harness's native named-agent dispatch either way — the pin is not the only reason, since
the agent also carries the skill's dispatch contract and preload. Pass the request through
unchanged, including any change or ADR id.
HEAD
```

- [ ] **Step 5: Update the function comment**

Replace `sync-agents.sh` L862–868 with:

```bash
# This block is COMMITTED into consumer repos and checked by `--check`, so a false claim here ships
# rather than merely displaying. It is SHARED by every harness in AGENTS_MD_DISPATCH_HARNESSES
# (codex and opencode), so its prose names no harness's artifact path and no harness's model
# vocabulary — a claim true for Codex only would be false in an opencode repo. The default store is
# agents/harness-defaults.yml, whose blocks are complete for every shipped harness, so the head
# states the pinned truth. The dispatch is required either way — the agent carries the skill's
# contract and preload, not just a model. Guarded, against the sidecar rather than a literal, in
# tests/test_sync_agents_codex.sh and tests/test_sync_agents_opencode.sh.
```

- [ ] **Step 6: Rename the sync function and fix its log strings**

The function is no longer codex-specific. Rename `sync_codex_agents_md_dispatch` to `sync_agents_md_dispatch` and update its comment and de-list log string. In `sync-agents.sh` L896–913:

```bash
# Write the AGENTS.md dispatch block when an AGENTS.md-dispatch harness (codex, opencode) is a
# targeted per-repo harness; strip it when the last one is de-listed (within an opted-in repo).
# Logs a one-time commit notice on write/remove.
sync_agents_md_dispatch(){
```

and the de-list arm (L909):

```bash
      removed) log "removed the docket dispatch block from $f (no AGENTS.md-dispatch harness targeted) — COMMIT THIS.";;
```

Update the sole call site at L1065 to `sync_agents_md_dispatch`.

- [ ] **Step 7: Fix the `--check` diagnostic**

`sync-agents.sh` L1100 names codex specifically; the condition is now "no AGENTS.md-dispatch harness":

```bash
      log "check: AGENTS.md carries a docket dispatch block but no AGENTS.md-dispatch harness (codex, opencode) is in agent_harnesses — run: bash sync-agents.sh and commit AGENTS.md"
```

- [ ] **Step 8: Verify no stale call site or string survives**

Derive the sites from a whole-repo grep rather than trusting the list above:

Run: `/usr/bin/grep -rn "sync_codex_agents_md_dispatch" . --include='*.sh' --include='*.md' | /usr/bin/grep -v '^./.docket/'`
Expected: no output.

Run: `/usr/bin/grep -n "codex de-listed\|but codex is not in agent_harnesses" sync-agents.sh`
Expected: no output.

- [ ] **Step 9: Run both harness contract tests**

Run: `"$DOCKET_BASH_PATH" tests/test_sync_agents_opencode.sh; "$DOCKET_BASH_PATH" tests/test_sync_agents_codex.sh`
Expected: all `ok -`. `test_sync_agents_codex.sh` asserts the block prose in both directions against the sidecar; if its head-prose asserts pin the old Codex-specific wording, update those asserts to the harness-neutral claim — they are guarding that the prose matches reality, and reality changed.

- [ ] **Step 10: Commit**

```bash
git add sync-agents.sh tests/test_sync_agents_opencode.sh tests/test_sync_agents_codex.sh
git commit -m "feat(0192): share the AGENTS.md dispatch block between codex and opencode"
```

---

### Task 4: Mirror the block in .docket.example.yml and arm the mirror guards

**Files:**
- Modify: `.docket.example.yml` (prose ~L301-306; a new commented block after the cursor block)
- Test: `tests/test_docket_example_yml.sh:826-1005`

**Interfaces:**
- Consumes: Task 1's sidecar block (the mirror must equal it row for row).
- Produces: a singly commented `#   opencode:` block whose sixteen rows equal the sidecar's.

- [ ] **Step 1: Write the failing asserts**

In `tests/test_docket_example_yml.sh`, bump the mirror-guard population floor (L933–936) from 3 to 4 — the loop already derives from `HD_SHIPPED_HARNESSES` and arms for free, but the floor is what stops a broken read passing with an empty loop:

```bash
assert "the mirror guard covered every shipped harness" '[ "$n_shipped" -ge 4 ]'
```

And beside the existing singly-commented-level asserts (L848–858), add opencode's pair:

```bash
assert "no ACTIVE opencode: header under agents:" \
  '! grep -qE "^[[:space:]]*opencode:[[:space:]]*$" "$EX"'
assert "the opencode mirror block is singly commented" \
  'grep -qE "^#   opencode:[[:space:]]*$" "$EX"'
```

- [ ] **Step 2: Run to verify they fail**

Run: `"$DOCKET_BASH_PATH" tests/test_docket_example_yml.sh 2>&1 | grep "^NOT OK"`
Expected: the floor assert and the singly-commented assert fail.

- [ ] **Step 3: Update the mirror prose**

In `.docket.example.yml`, L301–306:

```
# The `claude:`, `codex:`, `cursor:`, and `opencode:` blocks below all MIRROR docket's shipped
# built-in defaults — the values in agents/harness-defaults.yml — shown so the otherwise-invisible
# defaults are visible and tunable. Deleting any line falls back to the SAME shipped default. The
# sidecar is the single source of truth; this mirror never leads, and tests/test_docket_example_yml.sh
# enforces the equality. If a shipped default changes, update this mirror to match. All four shipped
# harnesses are complete — every one of the sixteen agents carries a model and an effort under each.
```

- [ ] **Step 4: Append the mirror block**

Add **after** the cursor block (keeping opencode last, so the round-trip slice anchored on cursor's `finalize-change` line at L948 is unaffected). Use the example file's row order — `status` first, `build-max` last, since `build-max` is the mirror guard's slice terminator:

```
#
#   # The opencode block — SHIPPED defaults (change 0192). opencode has no first-class effort
#   # field; it forwards unrecognized agent-frontmatter keys to the provider as model options, so
#   # docket emits effort as `reasoningEffort:` and it arrives as a real per-agent effort. Models
#   # are reached through OpenRouter, so IDs are double-prefixed. Add `opencode` to
#   # `agent_harnesses` above to generate the wrappers.
#   opencode:
#     status:                { model: openrouter/deepseek/deepseek-v4-flash-0731, effort: low }
#     adr:                   { model: openrouter/moonshotai/kimi-k3, effort: medium }
#     brainstorm-consultant: { model: openrouter/moonshotai/kimi-k3, effort: medium }
#     auto-groom:            { model: openrouter/deepseek/deepseek-v4-flash-0731, effort: medium }
#     auto-groom-critic:     { model: openrouter/openai/gpt-5.6-luna, effort: high }
#     implement-next:        { model: openrouter/deepseek/deepseek-v4-flash-0731, effort: high }
#     rebase-resolver:       { model: openrouter/moonshotai/kimi-k3, effort: high }
#     integration-repair:    { model: openrouter/moonshotai/kimi-k3, effort: high }
#     finalize-change:       { model: openrouter/deepseek/deepseek-v4-flash-0731, effort: high }
#     review-lean:           { model: openrouter/deepseek/deepseek-v4-flash-0731, effort: high }
#     review-standard:       { model: openrouter/moonshotai/kimi-k3, effort: medium }
#     review-deep:           { model: openrouter/moonshotai/kimi-k3, effort: high }
#     build-economy:         { model: openrouter/deepseek/deepseek-v4-flash-0731, effort: medium }
#     build-standard:        { model: openrouter/deepseek/deepseek-v4-flash-0731, effort: high }
#     build-premium:         { model: openrouter/moonshotai/kimi-k3, effort: medium }
#     build-max:             { model: openrouter/moonshotai/kimi-k3, effort: high }
```

- [ ] **Step 5: Extend the resolver round-trip with opencode evidence**

The round-trip proves the example resolves through the real generator. At `tests/test_docket_example_yml.sh` L964 the harnesses line is rewritten and L967 pre-creates the harness dirs; add opencode to both:

```bash
harnesses_line='agent_harnesses: [claude, cursor, codex, opencode]'
```
```bash
mkdir -p "$sandbox/.claude" "$sandbox/.cursor" "$sandbox/.codex" "$sandbox/.opencode"
```

Then, beside the codex TOML evidence (L995–1005), add the opencode evidence — that the example's own values reach a generated opencode definition:

```bash
OCF="$sandbox/.opencode/agents/docket-build-economy.md"
assert "round-trip: the example resolves into an opencode definition" '[ -f "$OCF" ]'
assert "round-trip: opencode definition carries the example's model" \
  'grep -qx "model: openrouter/deepseek/deepseek-v4-flash-0731" "$OCF"'
assert "round-trip: opencode definition carries the example's effort as reasoningEffort" \
  'grep -qx "reasoningEffort: medium" "$OCF"'
```

- [ ] **Step 6: Run to verify they pass**

Run: `"$DOCKET_BASH_PATH" tests/test_docket_example_yml.sh`
Expected: all `ok -`, no `NOT OK`.

- [ ] **Step 7: Mutation-test the mirror guard**

Prove the guard sees the new block. Change one example value away from the sidecar, re-run, restore:

```bash
cp .docket.example.yml /tmp/ex.bak
sed -i.bak 's|^#     build-max:             { model: openrouter/moonshotai/kimi-k3, effort: high }|#     build-max:             { model: openrouter/moonshotai/kimi-k3, effort: low }|' .docket.example.yml
"$DOCKET_BASH_PATH" tests/test_docket_example_yml.sh 2>&1 | grep -c "^NOT OK"
cp /tmp/ex.bak .docket.example.yml
```

Expected: a non-zero `NOT OK` count naming the opencode `build-max` effort mismatch. Confirm restore and green before continuing.

- [ ] **Step 8: Commit**

```bash
git add .docket.example.yml tests/test_docket_example_yml.sh
git commit -m "feat(0192): mirror the opencode block in .docket.example.yml with round-trip evidence"
```

---

### Task 5: Re-derive the remaining literal harness populations

Three guards still hard-code `claude cursor codex` and will not arm for opencode by themselves. Per the repo's own `AGENTS.md` ("never hand-list the sites of a literal — derive them") the fix is to derive the population, not to append a fourth token, so the next harness arms for free.

**Files:**
- Modify: `tests/test_docket_review.sh:68,79`
- Modify: `tests/test_sync_agents.sh:1674` (cross-harness leak guard)
- Modify: `tests/test_docket_build.sh:385-447` (ladder invariants)

**Interfaces:**
- Consumes: Tasks 1–2.
- Produces: guards whose population is `$HD_SHIPPED_HARNESSES`.

- [ ] **Step 1: Derive the review-rung population**

In `tests/test_docket_review.sh`, replace both literal loops (L68 and L79) with the shipped set, and add a floor so a failed source cannot pass with an empty loop:

```bash
  for h in $HD_SHIPPED_HARNESSES; do
    assert "harness-defaults: $h supplies a pair for review-$rung" \
      'grep -qE "^ *review-'"$rung"': *\{ *model: *[^ ,}]+, *effort: *[^ ,}]+ *\}" "$HD"'
  done
```

```bash
n_h=0
for h in $HD_SHIPPED_HARNESSES; do
  assert "$h: the review-deep pin equals the build-max pin" \
    'blk="$(awk "/^  '"$h"':/{f=1;next} /^  [a-z]+:/{f=0} f" "$HD")";
     d="$(printf "%s" "$blk" | sed -n "s/^ *review-deep: *//p")";
     m="$(printf "%s" "$blk" | sed -n "s/^ *build-max: *//p")";
     [ -n "$d" ] && [ "$d" = "$m" ]'
  n_h=$((n_h+1))
done
assert "the cap-rung invariant was checked on every shipped harness" '[ "$n_h" -ge 4 ]'
```

Ensure `scripts/lib/harness-defaults.sh` is sourced in this file before the loops; add `. "$REPO/scripts/lib/harness-defaults.sh"` near the top if it is not already there.

- [ ] **Step 2: Verify the awk block-boundary still works for opencode**

The invariant assert slices a block with `/^  [a-z]+:/{f=0}`. `opencode` is lowercase-only, so it matches — but confirm rather than assume:

Run: `awk '/^  opencode:/{f=1;next} /^  [a-z]+:/{f=0} f' agents/harness-defaults.yml | /usr/bin/grep -c 'model:'`
Expected: `16`.

- [ ] **Step 3: Derive the cross-harness leak population**

In `tests/test_sync_agents.sh` L1674, the guard asserts no other harness's model IDs leak into a given harness's output. Replace the literal `for h in cursor codex` with a derivation that excludes the harness under test:

```bash
other_models="$(for h in $HD_SHIPPED_HARNESSES; do
  [ "$h" = "claude" ] && continue
  hd_agents "$HD" "$h" | while IFS= read -r a; do
    [ -n "$a" ] || continue
    hd_field "$HD" "$h" "$a" model
  done
done | sort -u)"
```

Add the reverse direction too — no Claude model ID may appear in an opencode definition — beside the existing `.codex/agents/docket-*.toml` scan (L1687–1694):

```bash
if [ -d "$R/.opencode/agents" ]; then
  leaked=0
  for f in "$R"/.opencode/agents/docket-*.md; do
    [ -e "$f" ] || continue
    grep -qE "^model: (claude-|gpt-5|cursor-)" "$f" && leaked=1
  done
  assert "no claude/codex/cursor model ID leaks into an opencode definition" '[ "$leaked" = "0" ]'
fi
```

- [ ] **Step 4: Derive the ladder-invariant population in test_docket_build.sh**

At L438–447 the `cursor_models` and `codex_pairs` loops are literal. Replace them with one loop over the shipped set that asserts the ladder is complete on every harness:

```bash
# The four-rung ladder must be complete on every shipped harness — a rung missing on one harness is
# a build that silently falls back to that harness's own default mid-ladder. Population derived
# from HD_SHIPPED_HARNESSES so a new harness arms this for free.
n_lad=0
for h in $HD_SHIPPED_HARNESSES; do
  for rung in economy standard premium max; do
    assert "$h: build-$rung carries a complete pair" \
      '[ -n "$(hd_field "$HD" '"$h"' build-'"$rung"' model)" ] && [ -n "$(hd_field "$HD" '"$h"' build-'"$rung"' effort)" ]'
  done
  n_lad=$((n_lad+1))
done
assert "the ladder invariant was checked on every shipped harness" '[ "$n_lad" -ge 4 ]'
```

Keep the existing claude-specific verbatim asserts (L393–408) — those pin actual values, which is a different obligation from completeness.

- [ ] **Step 5: Run the three files**

Run: `for t in tests/test_docket_review.sh tests/test_sync_agents.sh tests/test_docket_build.sh; do echo "== $t"; "$DOCKET_BASH_PATH" "$t" | grep "^NOT OK"; done`
Expected: no `NOT OK` lines.

- [ ] **Step 6: Mutation-test the derived population**

Prove the derived loops really cover opencode — break the opencode `review-deep` pin and confirm the cap-rung assert reddens:

```bash
cp agents/harness-defaults.yml /tmp/hd2.bak
sed -i.bak 's|^    review-deep:           { model: openrouter/moonshotai/kimi-k3, effort: high }|    review-deep:           { model: openrouter/moonshotai/kimi-k3, effort: low }|' agents/harness-defaults.yml
"$DOCKET_BASH_PATH" tests/test_docket_review.sh 2>&1 | grep "^NOT OK"
cp /tmp/hd2.bak agents/harness-defaults.yml
```

Expected: a `NOT OK - opencode: the review-deep pin equals the build-max pin` line. Note the sed matches the **opencode** row specifically because only that block uses the kimi-k3 ID; confirm the restore leaves the suite green.

- [ ] **Step 7: Commit**

```bash
git add tests/test_docket_review.sh tests/test_sync_agents.sh tests/test_docket_build.sh
git commit -m "test(0192): derive harness populations from HD_SHIPPED_HARNESSES"
```

---

### Task 6: Documentation

The maintained documentation set is derived from a whole-repository grep, not from the list below — point-in-time records (archived changes, Accepted ADRs, prior specs/plans/results) keep their original wording and must NOT be rewritten.

**Files:**
- Create: `docs/opencode/setup.md`
- Modify: `README.md` (L10, L145, L147, L582, L588-593, L613, L724)
- Modify: `skills/docket-convention/references/agent-layer.md` (L16, L59-71 prose; the table row landed in Task 2)
- Modify: `tests/test_dispatch_capability.sh:244-247` (scope comment)

**Interfaces:**
- Consumes: Tasks 1–4.
- Produces: `docs/opencode/setup.md`.

- [ ] **Step 1: Derive the real update set**

Do not trust the line numbers above. Run the grep first and work from its output:

Run: `/usr/bin/grep -rn "Cursor, and Codex\|cursor, and codex\|All three shipped\|all three are complete\|three shipped harnesses" . --include='*.md' | /usr/bin/grep -v '^./.docket/' | /usr/bin/grep -v '/archive/\|/adrs/\|/plans/\|/specs/\|/results/'`
Expected: a short list of maintained surfaces. Reconcile it against the list above; if the grep finds a file not listed, update it too and note it.

- [ ] **Step 2: Write docs/opencode/setup.md**

Model it on `docs/codex/setup.md`'s structure: title; intro naming the two artifacts; `## Two scopes — and the opt-in you need`; `## Pinning models and effort`; `## Verifying it works`; `## Restart after (re)generating`. Content requirements:

- The two artifacts are `.opencode/agents/docket-*.md` (the sixteen generated definitions, gitignored) and the committed `AGENTS.md` dispatch block **shared with Codex**.
- Opt-in is `agent_harnesses: [opencode]` (or any list including it) in `.docket.yml` / `.docket.local.yml` / the global config.
- Models are reached through OpenRouter; authenticate with `opencode providers` (alias `opencode auth`). IDs are double-prefixed, e.g. `openrouter/deepseek/deepseek-v4-flash-0731`.
- Model-ID discovery command: `opencode models openrouter` (the Codex doc has the equivalent fence — mirror its shape).
- Effort is carried as `reasoningEffort:`, a provider **model option**, not a first-class opencode field. State that it therefore only applies when a model is resolved, and that the vocabulary is model-specific — `high` is the ceiling docket ships.
- Verification: `opencode debug agent docket-build-economy` prints the resolved config; show the expected `mode` / `model` / `options` shape from fact 1.
- Restart: opencode loads config once at start and does not hot-reload, so quit and restart after regenerating.
- Do **not** repeat the stale "complete thirteen-agent block" phrasing from `docs/codex/setup.md:7`; opencode's block is sixteen agents. (That Codex line is a pre-existing defect in a maintained doc — fix it in passing and say so in the commit body.)

- [ ] **Step 3: Update the README enumerations**

Working from Step 1's grep output: the harness roster becomes "Claude Code, Cursor, Codex, and opencode"; "All three of the example's commented harness blocks" becomes four and names `agents.opencode`; the *Finding model IDs* table (L588–593) gains a row `| opencode | opencode models openrouter |`; the per-repo pass section (L613) links `docs/opencode/setup.md` beside `docs/codex/setup.md`; L724's "all three are complete — sixteen agents each" becomes four. At L582 (`model: inherit` — "Cursor and Codex have no equivalent") add opencode to the no-equivalent list, since `emit_opencode_md` normalizes the sentinel.

- [ ] **Step 4: Update the agent-layer reference prose**

L16's built-in row ("harness-indexed; claude, cursor, and codex each complete") gains opencode; the shipped-layer/validator prose (L59–71) reflects four shipped harnesses. The wrapper-shape table row already landed in Task 2.

- [ ] **Step 5: Update the dispatch-capability scope comment**

`tests/test_dispatch_capability.sh:244-247` lists `docs/codex/setup.md` and `docs/cursor/*.md` as knowingly-omitted maintained docs. Add `docs/opencode/setup.md` to that list so the new doc's omission is deliberate and recorded rather than an accidental gap.

- [ ] **Step 6: Verify no maintained surface still claims three**

Run: `/usr/bin/grep -rn "All three shipped\|all three are complete\|Cursor, and Codex are first-class" . --include='*.md' | /usr/bin/grep -v '^./.docket/' | /usr/bin/grep -v '/archive/\|/adrs/\|/plans/\|/specs/\|/results/'`
Expected: no output.

- [ ] **Step 7: Run the docs-touching tests**

Run: `for t in tests/test_cursor_contract_docs.sh tests/test_dispatch_capability.sh tests/test_docket_example_yml.sh; do echo "== $t"; "$DOCKET_BASH_PATH" "$t" | grep "^NOT OK"; done`
Expected: no `NOT OK` lines.

- [ ] **Step 8: Commit**

```bash
git add docs/opencode/setup.md README.md skills/docket-convention/references/agent-layer.md tests/test_dispatch_capability.sh docs/codex/setup.md
git commit -m "docs(0192): opencode setup guide and maintained harness enumerations"
```

---

### Task 7: Whole-suite gate and live certification evidence

**Files:**
- No source changes expected; this task is the gate.

**Interfaces:**
- Consumes: Tasks 1–6.
- Produces: a green whole-suite run and the Tier-2 certification evidence for the results file.

- [ ] **Step 1: Run the whole suite**

Run exactly the canonical command (the fenced block in `skills/docket-finalize-change/SKILL.md`), in the foreground, in one call:

```bash
fail=0; for test in tests/test_*.sh; do "$DOCKET_BASH_PATH" "$test" | grep "^NOT OK" && fail=1; done; echo "SUITE_FAIL=$fail"
```

Expected: `SUITE_FAIL=0` and no `NOT OK` lines. Fix any failure before continuing; do not proceed on a red suite.

- [ ] **Step 2: Re-check portability under the real BSD tools**

The PATH `grep` is ugrep and accepts patterns `/usr/bin/grep` rejects, so a portability bug can pass locally. Check the files this change touched:

Run: `for f in sync-agents.sh scripts/lib/harness-defaults.sh scripts/lib/docket-gitignore-block.sh tests/test_sync_agents_opencode.sh; do /usr/bin/grep -nE 'grep -P|\{[0-9]+,[0-9]{3,}\}|sed -i +[^.]' "$f"; done`
Expected: no output.

- [ ] **Step 3: Confirm generation is clean and idempotent on this repo**

Run: `bash sync-agents.sh && bash sync-agents.sh --check; echo "CHECK_RC=$?"`
Expected: `CHECK_RC=0` and no staleness warnings. This repo does not opt into opencode, so no `.opencode/` directory should appear:

Run: `ls .opencode 2>&1`
Expected: `No such file or directory`.

- [ ] **Step 4: Collect the Tier-2 live certification evidence**

Certify the three named build dispatches against the real installation. For each profile, generate into a scratch repo and record `opencode debug agent`'s resolved config:

```bash
T="$(mktemp -d)"; cd "$T"; git init -q
printf 'agent_harnesses: [opencode]\n' > .docket.yml
bash /Users/homer/dev/docket/sync-agents.sh
opencode --version
for a in docket-build-economy docket-build-standard docket-build-premium; do
  echo "== $a"
  opencode debug agent "$a" 2>&1 | python3 -c "import json,sys; d=json.load(sys.stdin); print(json.dumps({k:d.get(k) for k in ('name','mode','model','options')}, indent=2))"
done
```

Expected: economy on Flash/`medium`, standard on Flash/`high`, premium on Kimi/`medium`, each with `mode: subagent` and the model split into `providerID: openrouter` + the vendor-qualified `modelID`. Record the output verbatim — it is the certification the spec's Tier 2 requires. A mismatch STOPS the build and surfaces design drift; never substitute a model.

- [ ] **Step 5: Record the waivers**

The spec deliberately does not certify live: the max rung, the three review rungs, automatic classification, and the single-escalation path. Their harness-neutral tests plus prior Claude/Codex evidence stand in. Note each waiver explicitly for the results file so it can be reopened if opencode behavior later diverges.

- [ ] **Step 6: Commit any gate fixes**

```bash
git add -A
git commit -m "test(0192): whole-suite gate and opencode live certification"
```

(Skip if the working tree is clean — a clean tree at the gate is the expected outcome.)

---

## Self-Review

**Spec coverage.** Every section of `docs/superpowers/specs/2026-08-02-opencode-profile-routed-build-support-design.md` maps to a task: *Shipped opencode defaults* → Task 1; *Harness registration and native generation* → Tasks 1–2; *Dispatch* → Task 3; *Example and documentation surfaces* → Tasks 4 and 6; *Failure behavior* → the catalog verification in the Verified-environment section plus Task 2 Step 10 and Task 7 Step 4, all of which stop rather than substitute; *Verification Tier 1* items 1–7 → Task 1 Step 11, Task 2 Steps 1/9, Task 2 Step 3 (sentinel normalization), Task 5 (override/leak), Task 3 (AGENTS.md write/strip/stale/neutral), Task 4 (mirror + round-trip), Task 7 Step 1 (whole suite); *Verification Tier 2* → Task 7 Steps 4–5.

Two spec items resolved rather than carried: the effort-passthrough spelling (settled as `reasoningEffort` against opencode 1.18.11) and the model-ID catalog check (all three verified present). One item added that the spec omitted: the `.gitignore` entry, which turned out to be free — the managed block generates it from `DOCKET_GI_HARNESS_TOKENS`, so Task 1 Step 3 covers it with no separate edit.

One spec assumption corrected: `link-skills.sh` needs no change, because opencode already resolves skills from `~/.agents/skills/`, which that script already populates.

**ADR.** Whether the AGENTS.md-block generalization (Task 3) warrants a new ADR or an `## Update` note on ADR-0036 is decided at build time by `docket-adr`'s own rules — it is a real change to a committed, machine-neutral artifact's scope. The sidecar extension itself is the planned consumer path of ADR-0064 and needs no new ADR.

**Placeholder scan.** No TBD/TODO; every code step carries the actual text. The one deliberately non-literal step is Task 6 Step 1, which derives its own file set from a grep — that is the repo's own `AGENTS.md` rule, not a placeholder.

**Type consistency.** `emit_opencode_md` takes `(src, model, effort)` — the same arity and meaning as `emit_codex_toml` / `emit_cursor_md` — and is called that way from `emit_for_harness`. `sync_codex_agents_md_dispatch` → `sync_agents_md_dispatch` is renamed at its definition (Task 3 Step 6), its single call site (L1065), and verified by grep (Task 3 Step 8). The sixteen agent short names are identical across the sidecar block, the example mirror, and the emitter tests.
