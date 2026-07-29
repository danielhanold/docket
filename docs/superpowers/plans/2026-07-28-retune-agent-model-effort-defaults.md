<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0164 — Retune agent model/effort defaults for all three supported harnesses](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-07-29-0164-retune-agent-model-effort-defaults-for-all-three-harnesses.md)**
<!-- docket:backlink:end -->

# Retune Agent Model/Effort Defaults Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move docket's shipped per-skill agent model/effort defaults to the change-0164 values across all three supported harnesses, updating every mirror and every string assertion that pins the old values.

**Architecture:** Values only — no logic, no resolution, no generation behavior changes. The nine `agents/docket-*.md` wrapper files are the single source of truth for the built-in `claude` tier (ADR-0039); `.docket.example.yml`'s commented `agents.claude` block mirrors them value-for-value and is machine-checked by `tests/test_docket_example_yml.sh` §(4). Because that mirror is enforced, the wrappers and the example block must move in the same task or the suite is red between them. Illustrative literals in README/agent-layer/sync-agents comments are cosmetic and move last.

**Tech Stack:** Bash 4+ shell scripts, markdown frontmatter, YAML config, a hand-rolled `assert`-based test suite under `tests/` (no runner — each file is executed directly with `bash`).

## Global Constraints

- **Values only.** No change to how the agent layer resolves, generates, or dispatches wrappers.
- **The wrappers LEAD, the example MIRRORS.** Per ADR-0039, edit `agents/docket-*.md` first and make `.docket.example.yml` match — never the reverse.
- **Never weaken an assertion.** Every test that pins a value by string moves to the new value. Do not relax a `=` to a wildcard, do not delete an assert, do not replace a literal with a variable read from the file under test.
- **Bash portability.** Shell edits and verification commands run under `/opt/homebrew/bin/bash`. Do not rely on the interactive shell: `grep` on this machine is `ugrep` and accepts patterns `/usr/bin/grep` rejects. Verify with `command grep` or `git grep`.
- **Verify mutations landed.** Every bulk substitution in this plan is followed by a `grep -c` count check. A `sed` that silently matches nothing yields a green run with nothing changed.
- **Exact target values** (the authority for every task):

```yaml
agents:
  claude:
    status:                { model: claude-haiku-4-5-20251001, effort: medium }
    adr:                   { model: claude-opus-5,             effort: low }
    brainstorm-consultant: { model: claude-opus-5,             effort: medium }
    auto-groom:            { model: claude-opus-5,             effort: low }
    auto-groom-critic:     { model: claude-opus-5,             effort: medium }
    implement-next:        { model: claude-opus-5,             effort: medium }
    rebase-resolver:       { model: claude-opus-5,             effort: medium }
    integration-repair:    { model: claude-opus-5,             effort: medium }
    finalize-change:       { model: claude-opus-5,             effort: low }
  codex:
    status:                { model: gpt-5.6-luna,  effort: xhigh }
    adr:                   { model: gpt-5.6-terra, effort: xhigh }
    brainstorm-consultant: { model: gpt-5.6-sol,   effort: medium }
    auto-groom:            { model: gpt-5.6-sol,   effort: low }
    auto-groom-critic:     { model: gpt-5.6-sol,   effort: medium }
    implement-next:        { model: gpt-5.6-sol,   effort: medium }
    rebase-resolver:       { model: gpt-5.6-sol,   effort: high }
    integration-repair:    { model: gpt-5.6-sol,   effort: high }
    finalize-change:       { model: gpt-5.6-terra, effort: high }
  cursor:
    status:                { model: cursor-grok-4.5-low-fast,  effort: auto }
    adr:                   { model: cursor-grok-4.5-high,      effort: auto }
    brainstorm-consultant: { model: cursor-grok-4.5-high,      effort: auto }
    auto-groom:            { model: cursor-grok-4.5-medium,    effort: auto }
    auto-groom-critic:     { model: cursor-grok-4.5-high,      effort: auto }
    implement-next:        { model: cursor-grok-4.5-high,      effort: auto }
    rebase-resolver:       { model: cursor-grok-4.5-high,      effort: auto }
    integration-repair:    { model: cursor-grok-4.5-high,      effort: auto }
    finalize-change:       { model: cursor-grok-4.5-high-fast, effort: auto }
```

**Established at reconcile — do not re-litigate:**

- `status` is **already** at its target (`claude-haiku-4-5-20251001` / `medium`). Its wrapper and every assertion pinning it stay byte-untouched.
- The `codex:` example block is **already** byte-equivalent to the target. No value edit. Task 3 verifies this rather than assuming it.
- `tests/test_sync_agents_codex.sh` and `tests/test_sync_agents_cursor.sh` pin only the **claude `status`** id (as the built-in a non-claude harness inherits). Since `status` does not move, **both files need no edit** — despite the change body listing them as sites.
- The `claude-sonnet-5` advisory in `docket-new-change` / `docket-groom-next` (asserted at `tests/test_sync_agents.sh:494-495`) is **out of scope** — tracked as change 0166. Leave it.

---

## File Structure

**Modified:**

| File | Responsibility here |
|---|---|
| `agents/docket-{adr,auto-groom,auto-groom-critic,brainstorm-consultant,finalize-change,implement-next,integration-repair,rebase-resolver}.md` | The eight moving wrappers' `model:`/`effort:` frontmatter — the built-in claude tier |
| `agents/docket-status.md` | **UNCHANGED** — already at target |
| `.docket.example.yml` | The commented `agents.claude` mirror (lines ~288-296) and the doubly-commented `cursor:` block (lines ~310-318) |
| `tests/test_sync_agents.sh` | Built-in value assertions for the claude tier |
| `tests/test_docket_example_yml.sh` | The round-trip slice's sed range anchor (line ~881) and the cursor `status` round-trip assertion (line ~907) |
| `README.md` | Two illustrative `agents:` example blocks (lines ~397, ~425) |
| `skills/docket-convention/references/agent-layer.md` | The illustrative `default:` block (lines ~31-32) |
| `sync-agents.sh` | The effort-rendering comment table (lines ~435-436) |

**Created:** nothing.

---

### Task 1: Retune the claude tier — wrappers, the example mirror, and their assertions

The wrappers, the `.docket.example.yml` mirror, and `tests/test_sync_agents.sh` are one atomic unit: `tests/test_docket_example_yml.sh` §(4) asserts the example matches each wrapper value-for-value, so splitting them leaves the suite red at the task boundary.

**Files:**
- Modify: `agents/docket-adr.md` (frontmatter `model:`/`effort:`)
- Modify: `agents/docket-auto-groom.md`
- Modify: `agents/docket-auto-groom-critic.md`
- Modify: `agents/docket-brainstorm-consultant.md`
- Modify: `agents/docket-finalize-change.md`
- Modify: `agents/docket-implement-next.md`
- Modify: `agents/docket-integration-repair.md`
- Modify: `agents/docket-rebase-resolver.md`
- Modify: `.docket.example.yml:288-296` (the commented `claude:` block)
- Test: `tests/test_sync_agents.sh`
- Test: `tests/test_docket_example_yml.sh` (no edit this task — it must go green on its own)

**Interfaces:**
- Consumes: nothing.
- Produces: the eight retuned wrapper files, which Task 2's round-trip and Task 3's illustrative literals both describe. The exact per-agent values are in Global Constraints; later tasks read them from there, not from these files.

- [ ] **Step 1: Write the failing assertions in `tests/test_sync_agents.sh`**

Apply these edits. Each is an exact string replacement; the left side appears once.

At the built-in block (around lines 51-60), replace:

```bash
assert "implement-next built-in = claude-opus-4-8/xhigh" \
  '[ "$(fm "$AGENTS/docket-implement-next.md" model)/$(fm "$AGENTS/docket-implement-next.md" effort)" = "claude-opus-4-8/xhigh" ]'
assert "auto-groom built-in = claude-opus-4-8/xhigh" \
  '[ "$(fm "$AGENTS/docket-auto-groom.md" model)/$(fm "$AGENTS/docket-auto-groom.md" effort)" = "claude-opus-4-8/xhigh" ]'
assert "finalize-change built-in = claude-sonnet-5/medium" \
  '[ "$(fm "$AGENTS/docket-finalize-change.md" model)/$(fm "$AGENTS/docket-finalize-change.md" effort)" = "claude-sonnet-5/medium" ]'
```

with:

```bash
assert "implement-next built-in = claude-opus-5/medium" \
  '[ "$(fm "$AGENTS/docket-implement-next.md" model)/$(fm "$AGENTS/docket-implement-next.md" effort)" = "claude-opus-5/medium" ]'
assert "auto-groom built-in = claude-opus-5/low" \
  '[ "$(fm "$AGENTS/docket-auto-groom.md" model)/$(fm "$AGENTS/docket-auto-groom.md" effort)" = "claude-opus-5/low" ]'
assert "finalize-change built-in = claude-opus-5/low" \
  '[ "$(fm "$AGENTS/docket-finalize-change.md" model)/$(fm "$AGENTS/docket-finalize-change.md" effort)" = "claude-opus-5/low" ]'
```

Leave the `status built-in = claude-haiku-4-5-20251001/medium` assert exactly as it is.

Replace:

```bash
assert "adr built-in = claude-sonnet-5/medium" \
  '[ "$(fm "$AGENTS/docket-adr.md" model)/$(fm "$AGENTS/docket-adr.md" effort)" = "claude-sonnet-5/medium" ]'
```

with:

```bash
assert "adr built-in = claude-opus-5/low" \
  '[ "$(fm "$AGENTS/docket-adr.md" model)/$(fm "$AGENTS/docket-adr.md" effort)" = "claude-opus-5/low" ]'
```

Around line 107, replace:

```bash
assert "auto keeps the built-in model" '[ "$(fm "$SBX/.claude/agents/docket-implement-next.md" model)" = "claude-opus-4-8" ]'
assert "unlisted skill keeps built-in model+effort" '[ "$(fm "$SBX/.claude/agents/docket-adr.md" model)/$(fm "$SBX/.claude/agents/docket-adr.md" effort)" = "claude-sonnet-5/medium" ]'
```

with:

```bash
assert "auto keeps the built-in model" '[ "$(fm "$SBX/.claude/agents/docket-implement-next.md" model)" = "claude-opus-5" ]'
assert "unlisted skill keeps built-in model+effort" '[ "$(fm "$SBX/.claude/agents/docket-adr.md" model)/$(fm "$SBX/.claude/agents/docket-adr.md" effort)" = "claude-opus-5/low" ]'
```

Around line 131, replace:

```bash
assert "0048: unlisted implement-next carries built-in model (claude-opus-4-8)" '[ "$(fm "$SBX/.claude/agents/docket-implement-next.md" model)" = "claude-opus-4-8" ]'
```

with:

```bash
assert "0048: unlisted implement-next carries built-in model (claude-opus-5)" '[ "$(fm "$SBX/.claude/agents/docket-implement-next.md" model)" = "claude-opus-5" ]'
```

Around lines 149-150, replace:

```bash
assert "0048: UNLISTED agent generated at built-in default (implement-next=claude-opus-4-8/xhigh)" \
  '[ "$(fm "$SBX/.claude/agents/docket-implement-next.md" model)/$(fm "$SBX/.claude/agents/docket-implement-next.md" effort)" = "claude-opus-4-8/xhigh" ]'
```

with:

```bash
assert "0048: UNLISTED agent generated at built-in default (implement-next=claude-opus-5/medium)" \
  '[ "$(fm "$SBX/.claude/agents/docket-implement-next.md" model)/$(fm "$SBX/.claude/agents/docket-implement-next.md" effort)" = "claude-opus-5/medium" ]'
```

Around lines 326-327, replace:

```bash
assert "critic: model is claude-opus-4-8" '[ "$(fm "$CRITIC" model)" = "claude-opus-4-8" ]'
assert "critic: effort is xhigh" '[ "$(fm "$CRITIC" effort)" = "xhigh" ]'
```

with:

```bash
assert "critic: model is claude-opus-5" '[ "$(fm "$CRITIC" model)" = "claude-opus-5" ]'
assert "critic: effort is medium" '[ "$(fm "$CRITIC" effort)" = "medium" ]'
```

Around lines 353-361, replace the comment line and the two loop asserts:

```bash
# they inject ONLY docket-convention, pin opus/xhigh, and carry abort-and-report.
```

with:

```bash
# they inject ONLY docket-convention, pin opus/medium, and carry abort-and-report.
```

and

```bash
  assert "$nw: model is claude-opus-4-8" '[ "$(fm "$f" model)" = "claude-opus-4-8" ]'
  assert "$nw: effort is xhigh" '[ "$(fm "$f" effort)" = "xhigh" ]'
```

with:

```bash
  assert "$nw: model is claude-opus-5" '[ "$(fm "$f" model)" = "claude-opus-5" ]'
  assert "$nw: effort is medium" '[ "$(fm "$f" effort)" = "medium" ]'
```

Around lines 375-376, replace:

```bash
assert "consultant: model is claude-opus-4-8" '[ "$(fm "$CONSULT" model)" = "claude-opus-4-8" ]'
assert "consultant: effort is xhigh" '[ "$(fm "$CONSULT" effort)" = "xhigh" ]'
```

with:

```bash
assert "consultant: model is claude-opus-5" '[ "$(fm "$CONSULT" model)" = "claude-opus-5" ]'
assert "consultant: effort is medium" '[ "$(fm "$CONSULT" effort)" = "medium" ]'
```

**Do NOT touch** these, which are deliberately unrelated to the built-in tier:
- the `status built-in`, line-449, line-670, and line-750 asserts (all pin `claude-haiku-4-5-20251001`, which does not move);
- lines 494-495 (`claude-sonnet-5` advisory — change 0166);
- lines 648 and 658 (`claude-opus-4-8` appears there as an arbitrary **fixture override value** exercising resolution, not as a built-in; changing it would test nothing new).

- [ ] **Step 2: Run the test to verify it fails**

```bash
cd /Users/homer/dev/docket/.worktrees/retune-agent-model-effort-defaults-for-all-three-harnesses
/opt/homebrew/bin/bash tests/test_sync_agents.sh
```

Expected: FAIL. The failing asserts name the eight moving agents (`implement-next`, `auto-groom`, `finalize-change`, `adr`, `critic`, `rebase-resolver`, `integration-repair`, `consultant`) — the wrappers still carry the old values. Confirm `status` asserts are **not** among the failures; if one is, an unintended edit landed — revert it.

- [ ] **Step 3: Retune the eight wrapper files**

Edit each file's frontmatter `model:` and `effort:` lines. These are the only two lines that change in each file; leave `name:`, `description:`, `skills:`, and the body untouched.

| File | `model:` | `effort:` |
|---|---|---|
| `agents/docket-adr.md` | `claude-opus-5` | `low` |
| `agents/docket-auto-groom.md` | `claude-opus-5` | `low` |
| `agents/docket-auto-groom-critic.md` | `claude-opus-5` | `medium` |
| `agents/docket-brainstorm-consultant.md` | `claude-opus-5` | `medium` |
| `agents/docket-finalize-change.md` | `claude-opus-5` | `low` |
| `agents/docket-implement-next.md` | `claude-opus-5` | `medium` |
| `agents/docket-integration-repair.md` | `claude-opus-5` | `medium` |
| `agents/docket-rebase-resolver.md` | `claude-opus-5` | `medium` |

`agents/docket-status.md` is **not** in this table and must not be edited.

Verify the edits landed and that `status` survived untouched:

```bash
cd /Users/homer/dev/docket/.worktrees/retune-agent-model-effort-defaults-for-all-three-harnesses
for f in agents/docket-*.md; do
  printf '%-40s %s / %s\n' "$f" \
    "$(sed -n 's/^model:[[:space:]]*//p' "$f" | head -n1)" \
    "$(sed -n 's/^effort:[[:space:]]*//p' "$f" | head -n1)"
done
```

Expected exactly:

```
agents/docket-adr.md                     claude-opus-5 / low
agents/docket-auto-groom-critic.md       claude-opus-5 / medium
agents/docket-auto-groom.md              claude-opus-5 / low
agents/docket-brainstorm-consultant.md   claude-opus-5 / medium
agents/docket-finalize-change.md         claude-opus-5 / low
agents/docket-implement-next.md          claude-opus-5 / medium
agents/docket-integration-repair.md      claude-opus-5 / medium
agents/docket-rebase-resolver.md         claude-opus-5 / medium
agents/docket-status.md                  claude-haiku-4-5-20251001 / medium
```

Also confirm the old ids are gone from `agents/`:

```bash
cd /Users/homer/dev/docket/.worktrees/retune-agent-model-effort-defaults-for-all-three-harnesses
command grep -rc 'claude-opus-4-8\|claude-sonnet-5' agents/ || echo "0 matches (expected)"
```

Expected: no matches.

- [ ] **Step 4: Run `tests/test_sync_agents.sh` to verify it passes**

```bash
cd /Users/homer/dev/docket/.worktrees/retune-agent-model-effort-defaults-for-all-three-harnesses
/opt/homebrew/bin/bash tests/test_sync_agents.sh
```

Expected: PASS.

- [ ] **Step 5: Confirm the example mirror is now RED**

```bash
cd /Users/homer/dev/docket/.worktrees/retune-agent-model-effort-defaults-for-all-three-harnesses
/opt/homebrew/bin/bash tests/test_docket_example_yml.sh
```

Expected: FAIL, in §(4) MIRROR EQUALITY — eight `model mirrors wrapper` / `effort mirrors wrapper` pairs. This is the machine-enforced mirror doing its job; it is why the next step is in this task and not a later one. If this file passes here, the mirror guard is not actually running — stop and investigate before proceeding.

- [ ] **Step 6: Update the `.docket.example.yml` claude mirror**

Replace the nine commented lines (around 288-296):

```yaml
#     status:                { model: claude-haiku-4-5-20251001, effort: medium }
#     adr:                   { model: claude-sonnet-5,           effort: medium }
#     brainstorm-consultant: { model: claude-opus-4-8,           effort: xhigh }
#     auto-groom:            { model: claude-opus-4-8,           effort: xhigh }
#     auto-groom-critic:     { model: claude-opus-4-8,           effort: xhigh }
#     implement-next:        { model: claude-opus-4-8,           effort: xhigh }
#     rebase-resolver:       { model: claude-opus-4-8,           effort: xhigh }
#     integration-repair:    { model: claude-opus-4-8,           effort: xhigh }
#     finalize-change:       { model: claude-sonnet-5,           effort: medium }
```

with:

```yaml
#     status:                { model: claude-haiku-4-5-20251001, effort: medium }
#     adr:                   { model: claude-opus-5,             effort: low }
#     brainstorm-consultant: { model: claude-opus-5,             effort: medium }
#     auto-groom:            { model: claude-opus-5,             effort: low }
#     auto-groom-critic:     { model: claude-opus-5,             effort: medium }
#     implement-next:        { model: claude-opus-5,             effort: medium }
#     rebase-resolver:       { model: claude-opus-5,             effort: medium }
#     integration-repair:    { model: claude-opus-5,             effort: medium }
#     finalize-change:       { model: claude-opus-5,             effort: low }
```

Keep the `#     ` prefix and the four-space YAML indent under it exactly — `ex_field()` matches `^    <agent>:[[:space:]]` after stripping one `# ` layer, so a changed indent silently breaks the mirror guard rather than failing it loudly. Column alignment of the `effort:` values is cosmetic but keep it tidy.

- [ ] **Step 7: Run both test files to verify they pass**

```bash
cd /Users/homer/dev/docket/.worktrees/retune-agent-model-effort-defaults-for-all-three-harnesses
/opt/homebrew/bin/bash tests/test_sync_agents.sh && /opt/homebrew/bin/bash tests/test_docket_example_yml.sh
```

Expected: both PASS.

- [ ] **Step 8: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/retune-agent-model-effort-defaults-for-all-three-harnesses
git add agents/ .docket.example.yml tests/test_sync_agents.sh
git commit -m "chore(0164): retune the built-in claude agent tier to claude-opus-5"
```

---

### Task 2: Retune the cursor example block and its two test anchors

**Files:**
- Modify: `.docket.example.yml:310-318` (the doubly-commented `cursor:` block)
- Test: `tests/test_docket_example_yml.sh:881` (the round-trip slice's sed range end anchor)
- Test: `tests/test_docket_example_yml.sh:907` (the cursor `status` round-trip assertion)

**Interfaces:**
- Consumes: nothing from Task 1 (a different block of the same file).
- Produces: the `cursor-grok-4.5-*` namespace in the example, which Task 3's verification sweep expects to find.

**Why the sed anchor is a hidden second site.** `tests/test_docket_example_yml.sh` §(5) isolates the commented `agents:` region with a `sed` range whose **end address is the cursor `finalize-change` model literal**:

```bash
agents_block="$(sed -n '/^# agents:$/,/finalize-change:.*grok-4\.5-fast-high/p' "$EX")"
```

Changing that value to `cursor-grok-4.5-high-fast` makes the end address never match, so `sed` runs to EOF and the round-trip feeds the resolver hundreds of lines of unrelated prose. The failure is loud but its cause is not — update the anchor in the same edit as the value.

- [ ] **Step 1: Write the failing assertions in `tests/test_docket_example_yml.sh`**

Replace (line ~881):

```bash
agents_block="$(sed -n '/^# agents:$/,/finalize-change:.*grok-4\.5-fast-high/p' "$EX")"
```

with:

```bash
agents_block="$(sed -n '/^# agents:$/,/finalize-change:.*cursor-grok-4\.5-high-fast/p' "$EX")"
```

Replace (line ~907):

```bash
  '[ "$(fm "$SB/.cursor/agents/docket-status.md" model)" = "grok-4.5-fast-medium" ]'
```

with:

```bash
  '[ "$(fm "$SB/.cursor/agents/docket-status.md" model)" = "cursor-grok-4.5-low-fast" ]'
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
cd /Users/homer/dev/docket/.worktrees/retune-agent-model-effort-defaults-for-all-three-harnesses
/opt/homebrew/bin/bash tests/test_docket_example_yml.sh
```

Expected: FAIL — at minimum `round-trip: cursor status model came from the example block`, because the example still says `grok-4.5-fast-medium`. Other §(5) round-trip asserts may also fail now that the slice anchor no longer matches the file; that is expected and resolves in Step 4.

- [ ] **Step 3: Update the `.docket.example.yml` cursor block**

Replace the nine doubly-commented lines (around 310-318):

```yaml
#   #   status:                { model: grok-4.5-fast-medium, effort: auto }
#   #   adr:                   { model: grok-4.5-xhigh, effort: auto }
#   #   brainstorm-consultant: { model: grok-4.5-xhigh, effort: auto }
#   #   auto-groom:            { model: grok-4.5-high, effort: auto }
#   #   auto-groom-critic:     { model: grok-4.5-xhigh, effort: auto }
#   #   implement-next:        { model: grok-4.5-xhigh, effort: auto }
#   #   rebase-resolver:       { model: grok-4.5-xhigh, effort: auto }
#   #   integration-repair:    { model: grok-4.5-xhigh, effort: auto }
#   #   finalize-change:       { model: grok-4.5-fast-high, effort: auto }
```

with:

```yaml
#   #   status:                { model: cursor-grok-4.5-low-fast, effort: auto }
#   #   adr:                   { model: cursor-grok-4.5-high, effort: auto }
#   #   brainstorm-consultant: { model: cursor-grok-4.5-high, effort: auto }
#   #   auto-groom:            { model: cursor-grok-4.5-medium, effort: auto }
#   #   auto-groom-critic:     { model: cursor-grok-4.5-high, effort: auto }
#   #   implement-next:        { model: cursor-grok-4.5-high, effort: auto }
#   #   rebase-resolver:       { model: cursor-grok-4.5-high, effort: auto }
#   #   integration-repair:    { model: cursor-grok-4.5-high, effort: auto }
#   #   finalize-change:       { model: cursor-grok-4.5-high-fast, effort: auto }
```

The `#   #   ` prefix (the doubly-commented form) must be preserved exactly on all nine lines — §(5)'s stage-1/stage-2 uncommenting depends on it, and §(3) asserts the `cursor:` header is still commented.

Confirm the substitution landed and that no bare `grok-4.5-` survives in the file:

```bash
cd /Users/homer/dev/docket/.worktrees/retune-agent-model-effort-defaults-for-all-three-harnesses
command grep -c 'cursor-grok-4\.5-' .docket.example.yml
command grep -n '[^-]grok-4\.5-' .docket.example.yml || echo "no bare grok ids (expected)"
```

Expected: `9`, then `no bare grok ids (expected)`.

- [ ] **Step 4: Run the test to verify it passes**

```bash
cd /Users/homer/dev/docket/.worktrees/retune-agent-model-effort-defaults-for-all-three-harnesses
/opt/homebrew/bin/bash tests/test_docket_example_yml.sh
```

Expected: PASS — including all of §(5), which proves the retuned cursor block still resolves through the real `sync-agents.sh` into a generated Cursor wrapper.

- [ ] **Step 5: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/retune-agent-model-effort-defaults-for-all-three-harnesses
git add .docket.example.yml tests/test_docket_example_yml.sh
git commit -m "chore(0164): move the cursor example block to the cursor-grok-4.5-* namespace"
```

---

### Task 3: Retune the illustrative literals and verify the whole surface

The remaining `claude-opus-4-8` occurrences are teaching examples, not values any code reads. No test pins them, so this task's gate is the full suite plus an explicit inventory sweep.

**Files:**
- Modify: `README.md:397` and `README.md:425`
- Modify: `skills/docket-convention/references/agent-layer.md:31-32`
- Modify: `sync-agents.sh:435-436`

**Interfaces:**
- Consumes: the retuned values from Tasks 1-2 (Global Constraints is the authority).
- Produces: nothing later tasks depend on.

- [ ] **Step 1: Update the two README example blocks**

Both `README.md:397` (inside the `~/.config/docket/config.yml` example) and `README.md:425` (inside the `.docket.local.yml` example) carry the identical line:

```yaml
    implement-next: { model: claude-opus-4-8, effort: xhigh }
```

Replace **both** with:

```yaml
    implement-next: { model: claude-opus-5, effort: medium }
```

- [ ] **Step 2: Update the agent-layer reference example**

In `skills/docket-convention/references/agent-layer.md`, replace:

```yaml
    implement-next: { model: claude-opus-4-8, effort: xhigh }
    status:         { model: claude-haiku-4-5-20251001 }
```

with:

```yaml
    implement-next: { model: claude-opus-5, effort: medium }
    status:         { model: claude-haiku-4-5-20251001 }
```

The `status:` line is already correct — it stays, unchanged, and is deliberately shown without an `effort:` key to illustrate the omitted-vs-`auto` distinction the surrounding prose teaches. Do not add an effort to it.

Leave the `cursor:` and `claude:` sub-blocks in that same example alone: their `gpt-5.1` / `gpt-5.5-medium-fast` / `gpt-5.1-codex` ids illustrate per-harness override and runner delegation, not the built-in tier.

- [ ] **Step 3: Update the sync-agents comment table**

In `sync-agents.sh`, replace:

```bash
#   claude-opus-4-8  xhigh            model: claude-opus-4-8[effort=xhigh]
#   claude-opus-4-8  unset|auto       model: claude-opus-4-8
```

with:

```bash
#   claude-opus-5    medium           model: claude-opus-5[effort=medium]
#   claude-opus-5    unset|auto       model: claude-opus-5
```

Preserve the surrounding column alignment of that comment table.

- [ ] **Step 4: Verify no stale literal survives on any live surface**

```bash
cd /Users/homer/dev/docket/.worktrees/retune-agent-model-effort-defaults-for-all-three-harnesses
command grep -rn 'claude-opus-4-8' \
  --include='*.md' --include='*.sh' --include='*.yml' . \
  | command grep -vE '^\./docs/(changes|adrs|results|superpowers)/' \
  | command grep -v '^\./tests/test_sync_agents\.sh'
```

Expected: **no output**. Two exclusions are deliberate and correct:
- `docs/changes/`, `docs/adrs/`, `docs/results/`, and `docs/superpowers/` hold archived changes, plans, ADRs, and specs — historical records of what the defaults *were*. They keep their literals. This does NOT cover all of `docs/` — live user-facing docs such as `docs/codex/setup.md` and `docs/cursor/*` are in scope for the sweep and must come back clean.
- `tests/test_sync_agents.sh` retains `claude-opus-4-8` at lines ~648 and ~658, where it is an arbitrary fixture override value, not a built-in (see Task 1, Step 1).

If either exclusion produces a surprise hit, investigate before proceeding — a live surface hiding behind a broad filter is exactly the failure mode this sweep exists to catch. Note that the shell's `grep` is `ugrep` and strips leading `./` from paths; the `command grep` calls above are required for the `^\./` filters to match at all.

Now confirm the codex block genuinely needed no edit, rather than assuming it:

```bash
cd /Users/homer/dev/docket/.worktrees/retune-agent-model-effort-defaults-for-all-three-harnesses
command grep -n 'gpt-5\.6-' .docket.example.yml
```

Expected: exactly nine lines, matching the `codex:` block in Global Constraints value-for-value (`luna`/`xhigh`, `terra`/`xhigh`, `sol`/`medium`, `sol`/`low`, `sol`/`medium`, `sol`/`medium`, `sol`/`high`, `sol`/`high`, `terra`/`high`). If any differs, fix it here.

- [ ] **Step 5: Run `sync-agents.sh --check`**

```bash
cd /Users/homer/dev/docket/.worktrees/retune-agent-model-effort-defaults-for-all-three-harnesses
/opt/homebrew/bin/bash sync-agents.sh --check; echo "EXIT=$?"
```

Expected: `EXIT=0`. This is the change's own stated verification bar.

- [ ] **Step 6: Run the full test suite**

```bash
cd /Users/homer/dev/docket/.worktrees/retune-agent-model-effort-defaults-for-all-three-harnesses
fail=0
for t in tests/test_*.sh; do
  if ! /opt/homebrew/bin/bash "$t" >/tmp/0164-suite.log 2>&1; then
    echo "RED: $t"; tail -20 /tmp/0164-suite.log; fail=1
  fi
done
echo "SUITE_FAIL=$fail"
```

Expected: `SUITE_FAIL=0` with no `RED:` lines. `tests/test_sync_agents_codex.sh` and `tests/test_sync_agents_cursor.sh` must be green **without having been edited** — they pin only the claude `status` id, which did not move. If either is red, the `status` wrapper was modified by mistake; revert that edit rather than adjusting the assertion.

- [ ] **Step 7: Commit**

```bash
cd /Users/homer/dev/docket/.worktrees/retune-agent-model-effort-defaults-for-all-three-harnesses
git add README.md skills/docket-convention/references/agent-layer.md sync-agents.sh
git commit -m "docs(0164): retune illustrative model literals to match the new built-ins"
```

---

## Self-Review

**Spec coverage** — every site in the change body maps to a task:

| Change-body site | Task |
|---|---|
| The nine `agents/docket-*.md` wrappers | 1 (eight move; `status` verified unchanged) |
| `.docket.example.yml` claude block | 1 |
| `.docket.example.yml` codex block | 3 Step 4 (verified already at target — no edit) |
| `.docket.example.yml` cursor block | 2 |
| `skills/docket-convention/references/agent-layer.md` | 3 |
| `tests/test_sync_agents.sh` | 1 |
| `tests/test_docket_example_yml.sh` | 1 (mirror, no edit) + 2 (anchors) |
| `tests/test_sync_agents_codex.sh` | 3 Step 6 (verified green unedited — `status` did not move) |
| `tests/test_sync_agents_cursor.sh` | 3 Step 6 (same) |
| `README.md`, `sync-agents.sh` (added at reconcile) | 3 |
| Verification: `sync-agents.sh --check` + touched tests green | 3 Steps 5-6 |

**Placeholder scan:** no TBDs; every edit shows both the exact before and after text; every verification step names the command and its expected output.

**Consistency:** `claude-opus-5` and the per-agent efforts are stated once in Global Constraints and referenced identically in Tasks 1 and 3. `cursor-grok-4.5-high-fast` appears in Task 2 in both the example value and the sed anchor — the two that must agree.

**One gap worth naming:** nothing in the repo validates that `claude-opus-5`, `gpt-5.6-*`, or `cursor-grok-4.5-*` are real model ids in their harnesses. That is deliberate — model ids pass through verbatim (ADR-0015) and the codex/cursor blocks carry an explicit "UNVALIDATED examples" banner. The change's Out of scope section makes the same call. Record it in the results file rather than trying to close it here.
