# Cursor dispatch-rule generation + always-full-set agents — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `sync-agents.sh` always generate the full built-in agent set per targeted harness, generate a Cursor `docket-dispatch.mdc` rule alongside the Cursor agents, and prune orphaned docket-owned files — so a Cursor repo's dispatch targets resolve by construction.

**Architecture:** Three coherent pieces, all in the root-level `sync-agents.sh` (no `scripts/` contract), layered on change 0046's harness-first resolution. (1) `project_level_pass` and `check_project_level` iterate the **full built-in agent set** (`agents/docket-*.md`) instead of the config-listed subset; the `agents:` block becomes override-only. (2) A new authored `cursor-rules/` source dir (`dispatch.head.md` + one `dispatch/docket-<name>.md` fragment per agent) is assembled into `docket-dispatch.mdc`, written at the user-level (`~/.cursor/rules/`) and per-repo (`<repo>/.cursor/rules/`) layers, and joins the `--check` drift gate. (3) A prune step deletes orphaned docket-owned files (a built-in agent docket no longer ships; a harness de-listed from `agent_harnesses`), scoped strictly to `docket-*` names; `--check` reports orphans without deleting.

**Tech Stack:** Bash (`set -euo pipefail`), awk/sed text processing, the project's plain-bash test scripts under `tests/` (run as `bash tests/test_sync_agents.sh`; no CI runner, no aggregate harness).

## Global Constraints

- **All generator logic lives in the root-level `sync-agents.sh`.** No `scripts/<name>.sh` + `scripts/<name>.md` contract for this script (it predates that convention; the spec keeps it root-level).
- **`sync-agents.sh` does NO git.** It only reads/writes/`rm`s working-tree files; the surrounding skill/CI stages and commits. Deletion is a working-tree `rm`.
- **Model IDs pass through verbatim** — harness-neutral, never validated against a roster (ADR-0015). Do not add model-ID validation.
- **Prune is strictly scoped to `docket-*` filenames** the generator owns. It must NEVER touch a non-docket file, and must NEVER `rmdir` a directory it did not itself empty by removing a docket file (protects a user's pre-existing empty `.claude/` etc.).
- **Test seam:** `DOCKET_HARNESS_ROOT` overrides `$HOME` for harness dirs and the global-config root (the latter only when `XDG_CONFIG_HOME` is unset). Every test uses a fresh `mktemp -d` sandbox; never touch the real `$HOME` or the real repo.
- **Determinism:** the dispatch rule is assembled by iterating `agents/docket-*.md` in glob order; generation and `--check` re-assemble identically, so the committed bytes are self-consistent.
- **Cursor is the only harness with a dispatch rule** (`HARNESS_HAS_DISPATCH_RULES` = just `cursor`); do not add a rule mechanism for other harnesses.
- **Preserve every currently-green assertion** in `tests/test_sync_agents.sh` except the two this plan explicitly rewrites (line ~101 and lines ~220–224). Preserve the convention (`skills/docket-convention/SKILL.md`) and README grep-assertions in the same test file (do not remove existing phrases; the README section must not hardcode a per-skill model/effort literal — LEARNINGS #17).

---

## File Structure

- `sync-agents.sh` (modify) — the generator. New/changed functions: `project_level_pass` (full-set), `check_project_level` (full-set + rule + orphan report), `agent_description` (new helper), `assemble_dispatch_rule` (new), `dispatch_rule_passes` / write points (new), `prune_orphans` (new). Bottom driver wires prune into the normal run.
- `cursor-rules/dispatch.head.md` (create) — static `.mdc` preamble (frontmatter + intro + required dispatch pattern).
- `cursor-rules/dispatch/docket-<name>.md` (create ×8) — one per built-in agent: a `## docket-<name> — dispatch only` subsection.
- `tests/test_sync_agents.sh` (modify) — rewrite 2 stale assertions; add a `Change 0048` test block.
- `skills/docket-convention/SKILL.md` (modify) — Agent layer: note full-set per-repo generation (config override-only) + the Cursor dispatch rule.
- `.docket.yml` (modify) — commented `agents:` example note: override-only semantics.
- `README.md` (modify) — the agent config section: note override-only + the Cursor dispatch rule.

---

## Task 1: Always-full-set per-repo generation (Piece 1)

**Files:**
- Modify: `sync-agents.sh` — `project_level_pass` (~229-259), `check_project_level` (~261-291)
- Test: `tests/test_sync_agents.sh` — rewrite line ~101 and lines ~220-224; add new cases

**Interfaces:**
- Consumes (unchanged, already in the file): `resolve_agent FILE HARNESS AGENT UNDER` → sets `RES_MODEL`/`RES_EFFORT`/`RES_MODEL_FROM_HARNESS`; `short_name PATH` → bare agent name (`implement-next`); `agent_keys FILE UNDER` → configured agent keys; `legacy_agent_keys`, `agents_block_harnesses`, `warn_legacy_shape`, `warn_fallback_model`, `emit`, `resolve_agent_harnesses` (sets `HARNESSES`).
- Produces (for later tasks): `project_level_pass` writes the **full** built-in set into `<repo>/.<H>/agents/` for each `H ∈ HARNESSES`; `check_project_level` verifies the full set per harness. Task 2 appends the dispatch-rule write to `project_level_pass`/user pass; Task 3 appends the rule check to `check_project_level`; Task 4 appends prune.

- [ ] **Step 1: Write the failing test — per-repo generates the full built-in set even when `agents:` lists a subset**

Add after the existing per-repo block (after the current line ~103 `rm -rf "$SBX" "$HROOT"`), a new block:

```bash
# ============================================================================
# Change 0048 — always-full-set per-repo generation (Piece 1)
# ============================================================================

# Per-repo now generates the FULL built-in set for a listed harness even when the
# agents: block lists only a subset; unlisted agents carry the built-in default model.
make_sandbox                                       # SBX = the repo
HROOT48A="$(mktemp -d)"; mkdir -p "$HROOT48A/.claude"
printf 'agents:\n  default:\n    status: { model: sonnet, effort: high }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48A" bash "$SYNC" >/dev/null )
assert "0048: full set — all 8 built-ins land in project-level .claude/agents" \
  '[ "$(find "$SBX/.claude/agents" -name "docket-*.md" | wc -l | tr -d " ")" = "8" ]'
assert "0048: listed agent carries its override (status=sonnet)" \
  '[ "$(fm "$SBX/.claude/agents/docket-status.md" model)" = "sonnet" ]'
assert "0048: UNLISTED agent generated at built-in default (implement-next=claude-opus-4-8/xhigh)" \
  '[ "$(fm "$SBX/.claude/agents/docket-implement-next.md" model)/$(fm "$SBX/.claude/agents/docket-implement-next.md" effort)" = "claude-opus-4-8/xhigh" ]'
rm -rf "$SBX" "$HROOT48A"
```

- [ ] **Step 2: Run it to verify it fails**

Run: `bash tests/test_sync_agents.sh 2>&1 | grep -E "0048: (full set|UNLISTED)"`
Expected: `NOT OK - 0048: full set — all 8 built-ins land...` and `NOT OK - 0048: UNLISTED agent generated...` (current code only writes the listed `status`).

- [ ] **Step 3: Flip `project_level_pass` to iterate the full built-in set**

Replace the body of `project_level_pass` (from `names="$(agent_keys "$DOCKET_YML" 1)"` through the `done <<EOF ... EOF`) so it iterates `agents/docket-*.md` like `user_level_pass`, keeping the dead-config warning and adding a typo-guard. The full replacement function:

```bash
project_level_pass() {  # built-in ⊕ per-repo -> <repo>/.<H>/agents for each H in HARNESSES (committed)
  [ -f "$DOCKET_YML" ] || return 0
  local src name harness dir cfg_h cfgname
  warn_legacy_shape "$DOCKET_YML" 1
  # Warn on any agents.<harness> block whose harness is NOT in agent_harnesses (dead config).
  while IFS= read -r cfg_h; do
    [ -n "$cfg_h" ] || continue
    [ "$cfg_h" = "default" ] && continue
    case " $HARNESSES " in *" $cfg_h "*) : ;; *) log "WARN agents.$cfg_h: block is not in agent_harnesses — ignored (dead config)." ;; esac
  done < <(agents_block_harnesses "$DOCKET_YML")
  # Typo guard: an agents: entry that overrides no real built-in is a no-op — warn (do not fail).
  while IFS= read -r cfgname; do
    [ -n "$cfgname" ] || continue
    [ -f "$AGENTS_SRC/docket-$cfgname.md" ] || log "WARN agents: '$cfgname' overrides no built-in agent (no agents/docket-$cfgname.md) — ignored (typo? advisory/interactive skills have no wrapper)."
  done < <(agent_keys "$DOCKET_YML" 1)
  # Always generate the FULL built-in set (config is override-only) into each listed harness.
  for src in "$AGENTS_SRC"/docket-*.md; do
    [ -e "$src" ] || continue
    name="$(short_name "$src")"
    for harness in $HARNESSES; do
      resolve_agent "$DOCKET_YML" "$harness" "$name" 1
      warn_fallback_model "$harness" "$name"
      dir="$REPO/.$harness/agents"
      mkdir -p "$dir"
      emit "$src" "$RES_MODEL" "$RES_EFFORT" > "$dir/docket-$name.md"
    done
  done
}
```

- [ ] **Step 4: Update `check_project_level` to iterate the full built-in set**

Replace the `names`-based iteration in `check_project_level`. Remove the `[ -n "$names" ] || { log "no agents: block — nothing to check"; return $rc; }` early-out and the `names="$(agent_keys ...)"` line; iterate the full built-in set instead. The full replacement function (dispatch-rule and orphan checks are appended in Tasks 3 and 4 — leave the clearly-marked insertion comment):

```bash
check_project_level() {  # diff committed <repo>/.<H>/agents files against freshly-resolved config (per harness)
  local rc=0 src name got tmp d harness
  [ -f "$DOCKET_YML" ] || { log "no .docket.yml in $REPO — nothing to check"; return 0; }
  local legacy; legacy="$(legacy_agent_keys "$DOCKET_YML" 1)"
  if [ -n "$legacy" ]; then
    log "drift: legacy bare-agent-key agents: shape ($(printf '%s' "$legacy" | tr '\n' ' ')) — reshape to agents.default.<agent> (run: bash sync-agents.sh)"
    rc=1
  fi
  tmp="$(mktemp -d)"
  for src in "$AGENTS_SRC"/docket-*.md; do
    [ -e "$src" ] || continue
    name="$(short_name "$src")"
    for harness in $HARNESSES; do
      resolve_agent "$DOCKET_YML" "$harness" "$name" 1
      emit "$src" "$RES_MODEL" "$RES_EFFORT" > "$tmp/docket-$name.md"
      got="$REPO/.$harness/agents/docket-$name.md"
      if [ ! -f "$got" ]; then
        log "drift: missing $got (run: bash sync-agents.sh)"; rc=1; continue
      fi
      d="$(diff -u "$got" "$tmp/docket-$name.md" || true)"
      if [ -n "$d" ]; then log "drift in .$harness/agents/docket-$name.md:"; printf '%s\n' "$d" >&2; rc=1; fi
    done
  done
  rm -rf "$tmp"
  # docket:0048 dispatch-rule check inserted here by Task 3
  # docket:0048 orphan report inserted here by Task 4
  return $rc
}
```

- [ ] **Step 5: Rewrite the two stale existing assertions**

(a) Line ~101 asserted the old listed-only behavior. In the existing per-repo block (the one that writes `printf 'agents:\n  default:\n    status: { model: sonnet, effort: high }\n    new-change: { model: opus }\n'`), replace:

```bash
assert "no project-level file for unlisted skill (implement-next)" '[ ! -f "$SBX/.claude/agents/docket-implement-next.md" ]'
```

with:

```bash
assert "0048: unlisted skill NOW generated at built-in default (implement-next)" '[ -f "$SBX/.claude/agents/docket-implement-next.md" ]'
assert "0048: unlisted implement-next carries built-in model (claude-opus-4-8)" '[ "$(fm "$SBX/.claude/agents/docket-implement-next.md" model)" = "claude-opus-4-8" ]'
```

The neighboring `assert "advisory skill in agents: produces NO file (new-change)" ...` stays as-is — `new-change` has no built-in wrapper, so it is still never generated.

(b) Lines ~220-224 asserted "no `agents:` block → nothing to check → rc=0", which is no longer true (the full set is now expected whenever `.docket.yml` is present). Replace the block:

```bash
# A repo with no agents: block has nothing to check -> passes.
rm -f "$SBX/.docket.yml"; : > "$SBX/.docket.yml"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "--check passes when no agents: block (rc=0)" '[ "$chk_rc" = "0" ]'
rm -rf "$SBX"
```

with (0048: an empty `agents:` block still requires the full committed set; generate-then-check passes; a truly absent `.docket.yml` still has nothing to check):

```bash
# 0048: an empty agents: block still expects the FULL committed set. Generate, then --check passes.
rm -f "$SBX/.docket.yml"; : > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null )
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0048: --check passes with empty agents: block once full set is committed (rc=0)" '[ "$chk_rc" = "0" ]'
rm -rf "$SBX"

# 0048: a repo with NO .docket.yml at all has nothing to check -> passes.
make_sandbox
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0048: --check passes when no .docket.yml (rc=0)" '[ "$chk_rc" = "0" ]'
rm -rf "$SBX"
```

- [ ] **Step 6: Run the full sync-agents test file to verify green**

Run: `bash tests/test_sync_agents.sh; echo "EXIT=$?"`
Expected: every line `ok - ...`, `EXIT=0`. In particular the new `0048:` assertions pass and no previously-green assertion regressed.

- [ ] **Step 7: Commit**

```bash
git add sync-agents.sh tests/test_sync_agents.sh
git commit -m "feat(0048): always-full-set per-repo agent generation (Piece 1)

project_level_pass and check_project_level now iterate the full built-in
agent set; the agents: block is override-only (typo-guarded). Unlisted
agents generate at their built-in default model."
```

---

## Task 2: Cursor dispatch rule — source + assembly + write (Piece 2)

**Files:**
- Create: `cursor-rules/dispatch.head.md`
- Create: `cursor-rules/dispatch/docket-adr.md`, `docket-auto-groom.md`, `docket-auto-groom-critic.md`, `docket-finalize-change.md`, `docket-implement-next.md`, `docket-integration-repair.md`, `docket-rebase-resolver.md`, `docket-status.md`
- Modify: `sync-agents.sh` — add `CURSOR_RULES_SRC`, `agent_description`, `assemble_dispatch_rule`, `write_dispatch_rule_user`, and per-repo rule write inside `project_level_pass`
- Test: `tests/test_sync_agents.sh` — add dispatch-rule generation cases

**Interfaces:**
- Consumes: `short_name`, `HARNESSES`, `HARNESS_AGENT_DIRS`, `log`, `AGENTS_SRC`, `REPO`, `HARNESS_ROOT`.
- Produces: `assemble_dispatch_rule` (stdout: head + per-agent subsections) — reused by Task 3's `--check`; `<repo>/.cursor/rules/docket-dispatch.mdc` (per-repo, when `cursor ∈ HARNESSES`) and `~/.cursor/rules/docket-dispatch.mdc` (user-level, when `~/.cursor/` present).

- [ ] **Step 1: Create the static preamble `cursor-rules/dispatch.head.md`**

```markdown
---
description: Docket agents must be dispatched, never run inline. Cursor runs a directly-invoked skill at the current model, which defeats docket's model/effort pins — so force a Task dispatch to the matching subagent_type.
alwaysApply: true
---

# Docket agents — dispatch only

Docket ships model/effort-pinned subagent wrappers in `.cursor/agents/docket-*.md`. When you are
asked to run one of the docket agents listed below, Cursor would otherwise run the skill **inline at
the currently-selected model**, which defeats the pin. Always dispatch to the matching subagent
instead.

## Required dispatch pattern

For every docket agent named below:

1. Do **NOT** run the skill inline in this chat.
2. Launch a **Task** with `subagent_type: "docket-<name>"` and `run_in_background: false`
   (foreground — wait for it). Pass the user's request through in the prompt, including any change /
   ADR id or argument they gave.
3. Relay the subagent's result back; do not re-do its work in the parent chat.
```

- [ ] **Step 2: Create the 8 per-agent fragments**

Create `cursor-rules/dispatch/docket-implement-next.md`:

```markdown
## docket-implement-next — dispatch only

Trigger when asked to implement the next build-ready change, drain the backlog, or build a specific
change id end-to-end (e.g. "implement the next change", "build change 48", "drain the docket backlog").

Dispatch prompt must include the explicit change id if the user named one (otherwise let the agent
select), and that it runs autonomously to an open PR and stops at the human merge gate.

Do NOT run the build inline, merge the PR, or re-brainstorm the design (the agent reconciles but never
re-brainstorms).

    Task(subagent_type: "docket-implement-next", run_in_background: false,
         prompt: "Implement change 48 end-to-end to an open PR; stop at the merge gate.")
```

Create `cursor-rules/dispatch/docket-auto-groom.md`:

```markdown
## docket-auto-groom — dispatch only

Trigger when asked to drain the auto-groomable needs-brainstorm queue with no human (e.g. "auto-groom
the backlog", "design the auto-groomable stubs").

Dispatch prompt must include any explicit stub id, and that kill/defer are never autonomous (the agent
abstains back to the human queue instead).

Do NOT run the grooming inline or make kill/defer decisions in the parent.

    Task(subagent_type: "docket-auto-groom", run_in_background: false,
         prompt: "Drain the auto-groomable needs-brainstorm queue.")
```

Create `cursor-rules/dispatch/docket-auto-groom-critic.md`:

```markdown
## docket-auto-groom-critic — dispatch only

This agent wraps no skill and is normally dispatched **by** docket-auto-groom as its adversarial gate,
not invoked directly. If you are asked to adversarially review an auto-groom draft spec or trivial
verdict, dispatch it rather than reviewing inline.

Dispatch prompt must include the draft spec / trivial verdict under review and the dispatching skill's
verdict protocol.

Do NOT let it improve the draft — it only attacks and returns exactly one verdict.

    Task(subagent_type: "docket-auto-groom-critic", run_in_background: false,
         prompt: "Adversarially review this draft spec and return one verdict per the protocol.")
```

Create `cursor-rules/dispatch/docket-finalize-change.md`:

```markdown
## docket-finalize-change — dispatch only

Trigger when asked to close out a change whose PR is approved or merged (e.g. "finalize change 48",
"close out the merged PR").

Dispatch prompt must include the change id, and that it merges (if approved) through the rebase-retest
gate, archives, cleans up the branch/worktree, and refreshes the board.

Do NOT merge or archive inline; let the agent run its gate (it may itself dispatch the rebase-resolver
or integration-repair subagents).

    Task(subagent_type: "docket-finalize-change", run_in_background: false,
         prompt: "Finalize change 48: merge through the gate, archive, clean up, refresh the board.")
```

Create `cursor-rules/dispatch/docket-adr.md`:

```markdown
## docket-adr — dispatch only

Trigger when asked to record, supersede, reverse, or index an architecture decision (e.g. "record an
ADR for this decision", "supersede ADR-0015", "regenerate the ADR index").

Dispatch prompt must include the decision (context / decision / consequences) or the index operation;
the agent assigns the number and updates the index.

Do NOT hand-write the ADR file or pick the number in the parent.

    Task(subagent_type: "docket-adr", run_in_background: false,
         prompt: "Record an ADR for <decision>: context, decision, consequences.")
```

Create `cursor-rules/dispatch/docket-integration-repair.md`:

```markdown
## docket-integration-repair — dispatch only

This agent wraps no skill and is normally dispatched **by** docket-finalize-change when the rebased
suite is red, not invoked directly. If asked to make a suite pass after a finalize rebase, dispatch it.

Dispatch prompt must include the red-test output and the base it was rebased onto; the agent writes a
minimal fix in at most two attempts and never weakens tests.

Do NOT weaken or delete tests in the parent to force green.

    Task(subagent_type: "docket-integration-repair", run_in_background: false,
         prompt: "The rebased suite is red — root-cause and write a minimal fix.")
```

Create `cursor-rules/dispatch/docket-rebase-resolver.md`:

```markdown
## docket-rebase-resolver — dispatch only

This agent wraps no skill and is normally dispatched **by** docket-finalize-change when its
rebase-onto-base gate hits a conflict, not invoked directly. If asked to resolve rebase conflicts for a
finalize, dispatch it.

Dispatch prompt must include the conflicted rebase state; the agent reconciles each hunk by merge
intent and continues the rebase to completion (it never runs tests).

Do NOT resolve the conflicts inline in the parent.

    Task(subagent_type: "docket-rebase-resolver", run_in_background: false,
         prompt: "Resolve the rebase conflicts by merge intent and continue the rebase.")
```

Create `cursor-rules/dispatch/docket-status.md`:

```markdown
## docket-status — dispatch only

Trigger when asked to see or refresh the docket backlog / board (e.g. "show the docket board",
"refresh the board", "run the docket health checks", "sweep merged changes").

Dispatch prompt must include which pass is wanted (board regen, merge sweep, or health checks) if the
user specified.

Do NOT regenerate the board or run the sweep inline.

    Task(subagent_type: "docket-status", run_in_background: false,
         prompt: "Refresh the docket board: regenerate BOARD.md, sweep, run health checks.")
```

- [ ] **Step 3: Write the failing test — assembled rule has a subsection per generated agent and none for a non-existent one**

Add a new block (place it after Task 1's 0048 block):

```bash
# 0048 Piece 2 — the Cursor dispatch rule is generated per-repo when cursor is listed.
make_sandbox
HROOT48R="$(mktemp -d)"; mkdir -p "$HROOT48R/.claude"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48R" bash "$SYNC" >/dev/null )
RULE="$SBX/.cursor/rules/docket-dispatch.mdc"
assert "0048 rule: per-repo docket-dispatch.mdc written for cursor" '[ -f "$RULE" ]'
assert "0048 rule: carries alwaysApply: true frontmatter" 'grep -q "^alwaysApply: true" "$RULE"'
assert "0048 rule: has the required dispatch pattern heading" 'grep -q "## Required dispatch pattern" "$RULE"'
assert "0048 rule: has a subsection for every built-in agent (8)" \
  '[ "$(grep -cE "^## docket-.* — dispatch only" "$RULE")" = "8" ]'
assert "0048 rule: names docket-implement-next as a subsection" 'grep -q "^## docket-implement-next — dispatch only" "$RULE"'
assert "0048 rule: names docket-status as a subsection" 'grep -q "^## docket-status — dispatch only" "$RULE"'
assert "0048 rule: no subsection for a non-existent agent" '! grep -q "docket-nonexistent" "$RULE"'
assert "0048 rule: deterministic order — adr before status" \
  '[ "$(grep -n "^## docket-adr — dispatch only" "$RULE" | cut -d: -f1)" -lt "$(grep -n "^## docket-status — dispatch only" "$RULE" | cut -d: -f1)" ]'
rm -rf "$SBX" "$HROOT48R"

# 0048 Piece 2 — cursor NOT listed => no per-repo rule (claude/other harness gets none).
make_sandbox
HROOT48N="$(mktemp -d)"; mkdir -p "$HROOT48N/.claude"
printf 'agent_harnesses: [claude]\nagents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48N" bash "$SYNC" >/dev/null )
assert "0048 rule: no dispatch rule for a claude-only repo" '[ ! -e "$SBX/.cursor/rules/docket-dispatch.mdc" ]'
assert "0048 rule: no rules dir under .claude" '[ ! -e "$SBX/.claude/rules/docket-dispatch.mdc" ]'
rm -rf "$SBX" "$HROOT48N"

# 0048 Piece 2 — user-level: rule written to ~/.cursor/rules when ~/.cursor present, skipped when absent.
make_sandbox                                  # make_sandbox creates .claude + .agents; .cursor ABSENT
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null )
assert "0048 rule: user-level rule SKIPPED when ~/.cursor absent" '[ ! -e "$SBX/.cursor/rules/docket-dispatch.mdc" ]'
mkdir -p "$SBX/.cursor"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$SBX" bash "$SYNC" >/dev/null )
assert "0048 rule: user-level rule WRITTEN when ~/.cursor present" '[ -f "$SBX/.cursor/rules/docket-dispatch.mdc" ]'
rm -rf "$SBX"
```

- [ ] **Step 4: Run it to verify it fails**

Run: `bash tests/test_sync_agents.sh 2>&1 | grep "0048 rule:"`
Expected: the `per-repo docket-dispatch.mdc written for cursor` and related assertions are `NOT OK` (no assembly code yet).

- [ ] **Step 5: Add the source-dir constant, description helper, and assembler to `sync-agents.sh`**

After the `AGENTS_SRC="$SCRIPT_DIR/agents"` line (~26), add:

```bash
CURSOR_RULES_SRC="$SCRIPT_DIR/cursor-rules"
```

Add near the other config helpers (e.g. after `short_name`, ~91):

```bash
# Extract the single-line `description:` frontmatter value from a wrapper source file.
agent_description(){ sed -n 's/^description:[[:space:]]*//p' "$1" | head -n1; }

# Harnesses that get a generated Cursor-style dispatch rule (only cursor exhibits the inline quirk).
HARNESS_HAS_DISPATCH_RULES="cursor"
harness_has_dispatch_rule(){ case " $HARNESS_HAS_DISPATCH_RULES " in *" $1 "*) return 0;; *) return 1;; esac; }
```

Add the assembler (place it after `emit`, ~188). It emits head + one subsection per built-in agent, in glob order; a built-in with no fragment gets a minimal auto-block + a warning:

```bash
# Assemble the Cursor dispatch rule to stdout: static head + one subsection per built-in agent
# (glob order). A built-in agent with a fragment uses it verbatim; one without gets a minimal
# auto-block derived from its description + a warning (a new agent is never silently un-dispatched).
assemble_dispatch_rule() {
  cat "$CURSOR_RULES_SRC/dispatch.head.md"
  local src name frag desc
  for src in "$AGENTS_SRC"/docket-*.md; do
    [ -e "$src" ] || continue
    name="$(short_name "$src")"
    frag="$CURSOR_RULES_SRC/dispatch/docket-$name.md"
    printf '\n'
    if [ -f "$frag" ]; then
      cat "$frag"
    else
      desc="$(agent_description "$src")"
      printf '## docket-%s — dispatch only\n\n' "$name"
      printf '%s\n\n' "$desc"
      printf 'When this applies, do NOT run the skill inline. Launch a Task with `subagent_type: "docket-%s"`, `run_in_background: false`, and relay its result.\n' "$name"
      log "WARN no dispatch fragment for docket-$name — emitted a minimal auto-block; add cursor-rules/dispatch/docket-$name.md"
    fi
  done
}

# Write the dispatch rule into a harness root's rules/ dir (<root>/.<harness>/rules/docket-dispatch.mdc).
write_dispatch_rule() {  # $1 = <root>/.<harness> base path
  mkdir -p "$1/rules"
  assemble_dispatch_rule > "$1/rules/docket-dispatch.mdc"
}
```

- [ ] **Step 6: Write the per-repo rule inside `project_level_pass`**

At the end of `project_level_pass` (after the agent-emit `for` loop, before the closing brace), add:

```bash
  # Cursor-only dispatch rule, per-repo (committed) when cursor is a targeted harness.
  local h
  for h in $HARNESSES; do
    harness_has_dispatch_rule "$h" || continue
    write_dispatch_rule "$REPO/.$h"
  done
```

- [ ] **Step 7: Write the user-level rule for present dispatch-rule harnesses**

At the end of `user_level_pass` (after the agent-emit loop, before the closing brace), add:

```bash
  # Cursor-only dispatch rule, user-level, for each present dispatch-rule harness root.
  local drh
  for drh in $HARNESS_HAS_DISPATCH_RULES; do
    [ -d "$HARNESS_ROOT/.$drh" ] || continue
    write_dispatch_rule "$HARNESS_ROOT/.$drh"
  done
```

- [ ] **Step 8: Write the failing test — a built-in agent with no fragment gets the minimal auto-block + warning**

Add:

```bash
# 0048 Piece 2 — a built-in agent lacking a fragment gets a minimal auto-block + a warning.
# Simulate by pointing the generator at a scratch clone whose fragment we remove.
make_sandbox
HROOT48F="$(mktemp -d)"; mkdir -p "$HROOT48F/.claude"
printf 'agent_harnesses: [cursor]\nagents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
# Remove one fragment in a throwaway copy of the repo scripts so the auto-block path fires.
SCRATCH="$(mktemp -d)"; cp -R "$REPO/agents" "$REPO/cursor-rules" "$REPO/sync-agents.sh" "$SCRATCH/"
rm -f "$SCRATCH/cursor-rules/dispatch/docket-status.md"
gen_err="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48F" bash "$SCRATCH/sync-agents.sh" 2>&1 >/dev/null)"
RULE="$SBX/.cursor/rules/docket-dispatch.mdc"
assert "0048 auto-block: warns about the missing fragment" 'printf "%s" "$gen_err" | grep -qi "no dispatch fragment for docket-status"'
assert "0048 auto-block: still emits a docket-status subsection" 'grep -q "^## docket-status — dispatch only" "$RULE"'
assert "0048 auto-block: subsection includes a Task subagent_type" 'grep -q "subagent_type: \"docket-status\"" "$RULE"'
rm -rf "$SBX" "$HROOT48F" "$SCRATCH"
```

- [ ] **Step 9: Run the full test file to verify green**

Run: `bash tests/test_sync_agents.sh; echo "EXIT=$?"`
Expected: all `ok - ...`, `EXIT=0` (all `0048 rule:` and `0048 auto-block:` assertions pass; nothing regressed).

- [ ] **Step 10: Commit**

```bash
git add cursor-rules sync-agents.sh tests/test_sync_agents.sh
git commit -m "feat(0048): generate the Cursor docket-dispatch.mdc rule (Piece 2)

New cursor-rules/ source (head + per-agent fragments); assemble_dispatch_rule
concatenates head + a subsection per built-in agent (glob order), with a
minimal auto-block + warning for a fragment-less agent. Written per-repo when
cursor is listed and user-level when ~/.cursor is present."
```

---

## Task 3: Dispatch-rule `--check` drift gate (Piece 2, check side)

**Files:**
- Modify: `sync-agents.sh` — `check_project_level` (insert the rule check at the `docket:0048 dispatch-rule check` marker)
- Test: `tests/test_sync_agents.sh` — add rule-drift cases

**Interfaces:**
- Consumes: `assemble_dispatch_rule` (Task 2), `HARNESSES`, `harness_has_dispatch_rule`, `REPO`, `log`.
- Produces: `check_project_level` returns `rc=1` when a committed `<repo>/.cursor/rules/docket-dispatch.mdc` is missing or drifts from a fresh re-assembly.

- [ ] **Step 1: Write the failing test — a hand-edited committed rule is drift**

Add:

```bash
# 0048 Piece 2 --check — a committed dispatch rule that drifts fails --check.
make_sandbox
HROOT48C="$(mktemp -d)"; mkdir -p "$HROOT48C/.claude"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48C" bash "$SYNC" >/dev/null )
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48C" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0048 rule-check: passes for an in-sync committed rule (rc=0)" '[ "$chk_rc" = "0" ]'
# Hand-edit the committed rule -> drift.
printf '\n<!-- tampered -->\n' >> "$SBX/.cursor/rules/docket-dispatch.mdc"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48C" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0048 rule-check: flags a hand-edited rule (rc!=0)" '[ "$chk_rc" != "0" ]'
assert "0048 rule-check: names the dispatch rule in the drift report" 'printf "%s" "$chk_out" | grep -q "docket-dispatch.mdc"'
# Delete the committed rule -> missing-file drift.
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48C" bash "$SYNC" >/dev/null )   # regenerate clean
rm -f "$SBX/.cursor/rules/docket-dispatch.mdc"
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48C" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0048 rule-check: flags a missing committed rule (rc!=0)" '[ "$chk_rc" != "0" ]'
rm -rf "$SBX" "$HROOT48C"
```

- [ ] **Step 2: Run it to verify it fails**

Run: `bash tests/test_sync_agents.sh 2>&1 | grep "0048 rule-check:"`
Expected: `flags a hand-edited rule` and `flags a missing committed rule` are `NOT OK` (check doesn't yet look at the rule).

- [ ] **Step 3: Insert the rule check into `check_project_level`**

Replace the `# docket:0048 dispatch-rule check inserted here by Task 3` marker line with:

```bash
  # Dispatch-rule drift: re-assemble and byte-diff the committed per-repo rule for each listed
  # dispatch-rule harness (cursor). The rule bytes are harness-independent, so assemble once.
  local h rule_got rule_tmp rd
  rule_tmp="$(mktemp)"
  assemble_dispatch_rule > "$rule_tmp"
  for h in $HARNESSES; do
    harness_has_dispatch_rule "$h" || continue
    rule_got="$REPO/.$h/rules/docket-dispatch.mdc"
    if [ ! -f "$rule_got" ]; then
      log "drift: missing $rule_got (run: bash sync-agents.sh)"; rc=1; continue
    fi
    rd="$(diff -u "$rule_got" "$rule_tmp" || true)"
    if [ -n "$rd" ]; then log "drift in .$h/rules/docket-dispatch.mdc:"; printf '%s\n' "$rd" >&2; rc=1; fi
  done
  rm -f "$rule_tmp"
```

- [ ] **Step 4: Run the full test file to verify green**

Run: `bash tests/test_sync_agents.sh; echo "EXIT=$?"`
Expected: all `ok - ...`, `EXIT=0`.

- [ ] **Step 5: Commit**

```bash
git add sync-agents.sh tests/test_sync_agents.sh
git commit -m "feat(0048): --check drift gate for the Cursor dispatch rule (Piece 2)

check_project_level re-assembles the rule and byte-diffs the committed
<repo>/.cursor/rules/docket-dispatch.mdc; missing or drifted -> rc=1."
```

---

## Task 4: Prune orphaned docket-owned files (Piece 3)

**Files:**
- Modify: `sync-agents.sh` — add `prune_orphans`; wire it into the normal run and the `--check` path (via `check_project_level`'s orphan marker)
- Test: `tests/test_sync_agents.sh` — add prune cases

**Interfaces:**
- Consumes: `short_name`, `HARNESSES`, `VALID_HARNESS_TOKENS`, `HARNESS_AGENT_DIRS`, `AGENTS_SRC`, `REPO`, `HARNESS_ROOT`, `CHECK`, `log`.
- Produces: on a normal run, orphaned docket-owned files are `rm`'d (a built-in docket no longer ships; a per-repo harness de-listed from `agent_harnesses`); empty docket-emptied dirs are `rmdir`'d. On `--check`, orphans are reported as drift (rc=1) without deletion.

- [ ] **Step 1: Write the failing test — removing a built-in agent prunes its files and drops its rule subsection**

Add:

```bash
# 0048 Piece 3 — removing a built-in agent prunes its generated files (both layers) + rule subsection.
make_sandbox
HROOT48P="$(mktemp -d)"; mkdir -p "$HROOT48P/.cursor"   # present user-level cursor root
printf 'agent_harnesses: [cursor]\nagents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
# Scratch clone we can mutate (remove a built-in agent + its fragment).
SCRATCH="$(mktemp -d)"; cp -R "$REPO/agents" "$REPO/cursor-rules" "$REPO/sync-agents.sh" "$SCRATCH/"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48P" bash "$SCRATCH/sync-agents.sh" >/dev/null )
assert "0048 prune: adr generated before removal (per-repo)" '[ -f "$SBX/.cursor/agents/docket-adr.md" ]'
assert "0048 prune: adr generated before removal (user-level)" '[ -f "$HROOT48P/.cursor/agents/docket-adr.md" ]'
# Remove the built-in agent + its fragment, regenerate: the orphan must be pruned.
rm -f "$SCRATCH/agents/docket-adr.md" "$SCRATCH/cursor-rules/dispatch/docket-adr.md"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48P" bash "$SCRATCH/sync-agents.sh" >/dev/null )
assert "0048 prune: removed built-in pruned from per-repo .cursor/agents" '[ ! -e "$SBX/.cursor/agents/docket-adr.md" ]'
assert "0048 prune: removed built-in pruned from user-level .cursor/agents" '[ ! -e "$HROOT48P/.cursor/agents/docket-adr.md" ]'
assert "0048 prune: rule subsection for removed agent dropped" '! grep -q "^## docket-adr — dispatch only" "$SBX/.cursor/rules/docket-dispatch.mdc"'
assert "0048 prune: a surviving agent remains" '[ -f "$SBX/.cursor/agents/docket-status.md" ]'
rm -rf "$SBX" "$HROOT48P" "$SCRATCH"

# 0048 Piece 3 — de-listing cursor prunes its per-repo docket files + rule, keeps a co-located non-docket file.
make_sandbox
HROOT48D="$(mktemp -d)"; mkdir -p "$HROOT48D/.claude"
printf 'agent_harnesses: [claude, cursor]\nagents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48D" bash "$SYNC" >/dev/null )
: > "$SBX/.cursor/agents/my-own-agent.md"          # operator's own co-located file
assert "0048 delist: cursor agents present before de-list" '[ -f "$SBX/.cursor/agents/docket-status.md" ]'
assert "0048 delist: cursor rule present before de-list" '[ -f "$SBX/.cursor/rules/docket-dispatch.mdc" ]'
# De-list cursor.
printf 'agent_harnesses: [claude]\nagents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48D" bash "$SYNC" >/dev/null )
assert "0048 delist: cursor docket agents pruned" '[ ! -e "$SBX/.cursor/agents/docket-status.md" ]'
assert "0048 delist: cursor dispatch rule pruned" '[ ! -e "$SBX/.cursor/rules/docket-dispatch.mdc" ]'
assert "0048 delist: operator's co-located non-docket file preserved" '[ -f "$SBX/.cursor/agents/my-own-agent.md" ]'
assert "0048 delist: claude still generated" '[ -f "$SBX/.claude/agents/docket-status.md" ]'
rm -rf "$SBX" "$HROOT48D"

# 0048 Piece 3 --check — an orphaned committed file is reported as drift, NOT deleted.
make_sandbox
HROOT48O="$(mktemp -d)"; mkdir -p "$HROOT48O/.claude"
printf 'agent_harnesses: [claude]\nagents:\n  default:\n    status: { model: sonnet }\n' > "$SBX/.docket.yml"
( cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48O" bash "$SYNC" >/dev/null )
: > "$SBX/.claude/agents/docket-bogus.md"           # an orphan: no built-in docket-bogus
chk_out="$(cd "$SBX" && DOCKET_HARNESS_ROOT="$HROOT48O" bash "$SYNC" --check 2>&1)"; chk_rc=$?
assert "0048 orphan-check: reports the orphan as drift (rc!=0)" '[ "$chk_rc" != "0" ]'
assert "0048 orphan-check: names the orphaned file" 'printf "%s" "$chk_out" | grep -q "docket-bogus.md"'
assert "0048 orphan-check: --check does NOT delete the orphan" '[ -f "$SBX/.claude/agents/docket-bogus.md" ]'
rm -rf "$SBX" "$HROOT48O"
```

- [ ] **Step 2: Run it to verify it fails**

Run: `bash tests/test_sync_agents.sh 2>&1 | grep -E "0048 (prune|delist|orphan-check):"`
Expected: prune/delist/orphan-check assertions are `NOT OK` (no prune yet).

- [ ] **Step 3: Add `prune_orphans` to `sync-agents.sh`**

Add after `check_project_level` (~291). It handles both a removed built-in (in every targeted agents dir) and a de-listed per-repo harness; honours `CHECK` (report vs delete):

```bash
# Handle one orphaned docket-owned file: report it as drift under --check, else rm it.
handle_orphan() {  # $1 = path ; sets ORPHAN_DRIFT=1 under --check
  if [ "$CHECK" = "1" ]; then
    log "drift: orphaned docket-owned file $1 (run: bash sync-agents.sh)"
    ORPHAN_DRIFT=1
  else
    rm -f "$1"
  fi
}

# rmdir a dir ONLY if docket emptied it this run (never a pre-existing empty/user dir). Delete mode only.
rmdir_if_docket_emptied() {  # $1 = dir
  [ "$CHECK" = "1" ] && return 0
  [ -d "$1" ] || return 0
  rmdir "$1" 2>/dev/null || true
}

# Prune orphaned docket-owned files. Scope:
#   scope=all      normal run — per-repo (HARNESSES) + user-level (present harnesses) removed-builtins,
#                  plus per-repo de-listed-harness cleanup.
#   scope=per-repo --check — per-repo only, report-only.
prune_orphans() {  # $1 = scope (all|per-repo)
  local scope="$1" dir f name tok pruned
  # (1) Removed built-in agent: any docket-<name>.md whose built-in source is gone.
  local -a scan_dirs=()
  for tok in $HARNESSES; do scan_dirs+=("$REPO/.$tok/agents"); done
  if [ "$scope" = "all" ]; then
    for dir in "${HARNESS_AGENT_DIRS[@]}"; do
      [ -d "$(dirname "$dir")" ] && scan_dirs+=("$dir")
    done
  fi
  for dir in "${scan_dirs[@]}"; do
    [ -d "$dir" ] || continue
    for f in "$dir"/docket-*.md; do
      [ -e "$f" ] || continue
      name="$(short_name "$f")"
      [ -f "$AGENTS_SRC/docket-$name.md" ] || handle_orphan "$f"
    done
  done
  # (2) De-listed per-repo harness: a known harness NOT in HARNESSES with docket-owned per-repo files.
  for tok in $VALID_HARNESS_TOKENS; do
    case " $HARNESSES " in *" $tok "*) continue;; esac      # still listed -> not de-listed
    pruned=0
    for f in "$REPO/.$tok/agents"/docket-*.md; do
      [ -e "$f" ] || continue
      handle_orphan "$f"; pruned=1
    done
    if [ -e "$REPO/.$tok/rules/docket-dispatch.mdc" ]; then
      handle_orphan "$REPO/.$tok/rules/docket-dispatch.mdc"; pruned=1
    fi
    # Only rmdir dirs docket just emptied (pruned=1) — never a user's pre-existing dir.
    if [ "$pruned" = "1" ]; then
      rmdir_if_docket_emptied "$REPO/.$tok/agents"
      rmdir_if_docket_emptied "$REPO/.$tok/rules"
      rmdir_if_docket_emptied "$REPO/.$tok"
    fi
  done
}
```

- [ ] **Step 4: Wire prune into the normal run**

At the bottom of the file, change:

```bash
user_level_pass
project_level_pass
log "done"
```

to:

```bash
user_level_pass
project_level_pass
prune_orphans all
log "done"
```

- [ ] **Step 5: Wire the orphan report into `--check`**

Replace the `# docket:0048 orphan report inserted here by Task 4` marker in `check_project_level` with:

```bash
  # Orphan report (per-repo only, report-only): a committed docket-owned file with no source.
  ORPHAN_DRIFT=0
  prune_orphans per-repo
  [ "$ORPHAN_DRIFT" = "1" ] && rc=1
```

Also declare `ORPHAN_DRIFT` initialization is local-safe: it is a global set by `handle_orphan`; the line above resets it before the scan. (No `local` needed; it is intentionally global so `handle_orphan` can set it.)

- [ ] **Step 6: Run the full test file to verify green**

Run: `bash tests/test_sync_agents.sh; echo "EXIT=$?"`
Expected: all `ok - ...`, `EXIT=0`. Re-confirm the earlier `0048 rule:`, `0048 rule-check:`, and every pre-existing `0045`/`0046` assertion still pass (prune must not disturb them — e.g. the `[cursor]`-only and `[]`-empty cases must not `rmdir` a user's pre-existing `.claude`).

- [ ] **Step 7: Commit**

```bash
git add sync-agents.sh tests/test_sync_agents.sh
git commit -m "feat(0048): prune orphaned docket-owned files (Piece 3)

prune_orphans removes a docket-<name>.md whose built-in is gone (both layers)
and, per-repo, the files of a harness de-listed from agent_harnesses (agents +
dispatch rule); rmdir only dirs it emptied. --check reports orphans without
deleting."
```

---

## Task 5: Documentation — convention, `.docket.yml`, README

**Files:**
- Modify: `skills/docket-convention/SKILL.md` — Agent layer section
- Modify: `.docket.yml` — commented `agents:` note
- Modify: `README.md` — agent config section
- Test: `tests/test_sync_agents.sh` — a small doc assertion for the always-full-set + dispatch-rule facts

**Interfaces:**
- Consumes: nothing (prose only).
- Produces: convention/README prose the existing doc grep-assertions (0016/0045/0046/0047) must continue to satisfy, plus new 0048 facts.

- [ ] **Step 1: Write the failing doc test**

Add near the other convention doc assertions (after the 0046 doc block, ~241):

```bash
# 0048 doc: convention states per-repo generates the full built-in set (config override-only)
# and that cursor gets a generated docket-dispatch.mdc rule.
assert "0048 doc: convention says per-repo writes the full built-in set" 'grep -qiE "full (built-in )?(agent )?set" "$CONV"'
assert "0048 doc: convention says the agents: block is override-only" 'grep -qi "override-only" "$CONV"'
assert "0048 doc: convention names the cursor dispatch rule" 'grep -q "docket-dispatch.mdc" "$CONV"'
```

- [ ] **Step 2: Run it to verify it fails**

Run: `bash tests/test_sync_agents.sh 2>&1 | grep "0048 doc:"`
Expected: the three `0048 doc:` assertions are `NOT OK`.

- [ ] **Step 3: Edit `skills/docket-convention/SKILL.md` Agent layer**

In the "Agent layer" section, in the paragraph describing per-repo generation and the `agents:` block, add sentences (preserve all existing phrases the 0016/0045/0046 assertions grep for — `per-repo > global > built-in`, `default/built-in`, `agent_harnesses`, `[claude]`, `passthrough`, `ADR-0015`, the `default:`/`cursor:` example, etc.). Add, adjacent to the per-repo generation prose:

> The **per-repo pass writes the full built-in agent set** for every harness in `agent_harnesses` — the `agents:` block is **override-only** (it tunes a model/effort; it never decides *which* agents exist, since the agents compose and a harness needs all of them). An `agents:` entry naming no built-in is a typo warning. Additionally, the `cursor` harness gets a generated **`docket-dispatch.mdc`** rule (`~/.cursor/rules/` user-level; `<repo>/.cursor/rules/` per-repo) that forces a Task dispatch to the matching `subagent_type` — Cursor otherwise runs a directly-invoked skill inline at the current model, defeating the pin. Because the per-repo pass generates that same full set into the harness, the rule's dispatch targets resolve by construction. `sync-agents.sh` prunes orphaned `docket-*` files (a removed built-in; a de-listed harness) and `sync-agents.sh --check` spans the committed agents and the dispatch rule.

- [ ] **Step 4: Edit `.docket.yml` commented example**

In the commented `agents:` block prose in `.docket.yml`, add one line noting override-only + the full-set behavior (keep it a comment; do not add an active block):

```yaml
# The per-repo pass writes the FULL built-in agent set for every harness in agent_harnesses; this
# block is OVERRIDE-ONLY (tunes model/effort, never decides which agents exist). A `cursor` harness
# additionally gets a generated .cursor/rules/docket-dispatch.mdc dispatch rule. sync-agents.sh
# prunes orphaned docket-* files; --check spans the agents and the rule.
```

- [ ] **Step 5: Edit `README.md` agent config section**

In the README's agent model/effort section (the one the 0047 assertions target — heading matches `^##[[:space:]].*[Aa]gent.*([Mm]odel|[Ee]ffort)`), add a sentence. **Do NOT hardcode any per-skill model/effort literal** (0047 non-restatement guard, LEARNINGS #17):

> The per-repo layer writes the **full built-in agent set** for every harness in `agent_harnesses` (the `agents:` block only *overrides* model/effort — it never decides which agents exist). A repo listing `cursor` also gets a generated `.cursor/rules/docket-dispatch.mdc` that forces Cursor to dispatch docket agents instead of running them inline. `sync-agents.sh --check` covers both the generated agents and the dispatch rule.

- [ ] **Step 6: Run the full test file + spot-check the 0047 README guard**

Run: `bash tests/test_sync_agents.sh; echo "EXIT=$?"`
Expected: all `ok - ...`, `EXIT=0` — the three `0048 doc:` pass, and every `0047 §agent-cfg:` assertion (including `does NOT hardcode a model/effort literal`) still passes.

- [ ] **Step 7: Commit**

```bash
git add skills/docket-convention/SKILL.md .docket.yml README.md tests/test_sync_agents.sh
git commit -m "docs(0048): document always-full-set per-repo gen + the Cursor dispatch rule

docket-convention Agent layer, .docket.yml commented example, and the README
agent config section note override-only semantics and docket-dispatch.mdc."
```

---

## Self-Review

**1. Spec coverage:**
- Piece 1 (always-full-set) → Task 1 (`project_level_pass` + `check_project_level` iterate built-ins; typo-guard; override-only). Spec test case #1 → Task 1 Step 1.
- Piece 2 (dispatch rule) → Task 2 (source dir, `assemble_dispatch_rule`, both write layers, minimal auto-block + warning) + Task 3 (`--check` for the rule). Spec test cases #2, #3, #6, #8 → Task 2 Steps 3/8; #7 (rule half) → Task 3.
- Piece 3 (prune) → Task 4 (removed built-in both layers; de-listed harness per-repo; `rmdir` only docket-emptied dirs; `--check` reports without deleting). Spec test cases #4, #5, #7 (orphan half) → Task 4.
- Docs/ADR → Task 5 (convention, `.docket.yml`, README). The ADR itself (new ADR vs `## Update` on ADR-0015/0016) is recorded at the build's ADR step by the implementer, not in this plan.

**2. Placeholder scan:** No `TBD`/`handle appropriately`/"write tests for the above" — every code and test step carries full content.

**3. Type consistency:** Function/var names used across tasks are consistent: `assemble_dispatch_rule` (Task 2, consumed by Task 3), `write_dispatch_rule`, `harness_has_dispatch_rule`, `HARNESS_HAS_DISPATCH_RULES`, `CURSOR_RULES_SRC`, `agent_description`, `prune_orphans <scope>`, `handle_orphan`, `ORPHAN_DRIFT`, `rmdir_if_docket_emptied`. The `check_project_level` insertion markers (`docket:0048 dispatch-rule check` in Task 1, filled in Task 3; `docket:0048 orphan report` in Task 1, filled in Task 4) match. Glob order for the rule and for `--check` re-assembly is identical (both `"$AGENTS_SRC"/docket-*.md`).

**Buildable intermediate states (LEARNINGS #45):** each task ends green. Task 1 leaves the two check markers as inert comments (valid bash). Task 2 generates the rule but `--check` doesn't yet verify it (fine). Task 3 adds the check. Task 4 adds prune last so its `rmdir`-safety is reviewed against the already-green `[cursor]`-only / `[]`-empty cases. Shell caution (LEARNINGS #46): the authored `assemble_dispatch_rule`, `prune_orphans`, and the `check_project_level` rewrite are the highest-risk hunks — the per-task review must exercise the de-list `rmdir` guard, the SIGPIPE-safety of `assemble_dispatch_rule` piped into `diff`, and that prune never touches a non-`docket-*` file.
