# Cursor dispatch-rule generation + always-full-set agents — design

**Date:** 2026-07-08
**Change:** 0048
**Depends on:** 0046 (harness-first per-agent model resolution — at the merge gate)
**Related:** 0045 (multi-harness agent generation), 0016 (agent layer)
**ADRs:** 0015, 0016 (extended)

## Context

docket's **agent layer** (ADR-0008/0016) pins each autonomous skill to a model/effort via a
generated **subagent wrapper** (`agents/docket-*.md`). `sync-agents.sh` generates these into two
layers: **user-level** (built-in ⊕ global config → every present harness's `~/.<H>/agents/`) and
**per-repo** (built-in ⊕ `.docket.yml` `agents:` → committed `<repo>/.<H>/agents/`). Change 0045
fanned per-repo generation out across an explicit `agent_harnesses` list; change 0046 made the
`agents:` block **harness-first** so each harness can carry its own model IDs, resolved field by
field (`agents.<harness>.<agent>` → `agents.default.<agent>` → the agent's shipped built-in
default). ADR-0015 established that model IDs are **direct, harness-neutral, passed through
verbatim** — the property that lets docket drive non-Claude harnesses such as Cursor.

Two problems remain when running docket through **Cursor**.

### Problem 1 — the Cursor skill-dispatch quirk

When a skill is invoked **directly** in Cursor's agent chat, Cursor does **not** dispatch to the
skill's bound subagent — it runs the skill inline at whatever model is currently selected. This
defeats the entire point of the model/effort-pinned wrappers: the autonomous docket skills that are
supposed to run at a fixed model instead run at an arbitrary one.

The proven workaround is a Cursor **rule file** (`.cursor/rules/*.mdc`) with `alwaysApply: true`
that intercepts the request and forces the parent to dispatch to the matching `subagent_type` via
the Task tool. These rules are currently **hand-authored** per Cursor repo. They must be
**generated** wherever docket generates Cursor agents — at both the user-level (`~/.cursor/rules/`)
and per-repo (`<repo>/.cursor/rules/`) layers.

### Problem 2 — per-repo generation is listed-only, so the dispatch targets may not exist

A generated dispatch rule names `subagent_type: docket-<name>` for each docket agent. For Cursor to
**resolve** that dispatch, a matching agent file must exist in the same layer/harness
(`<repo>/.cursor/agents/docket-<name>.md` or `~/.cursor/agents/docket-<name>.md`); otherwise the
dispatch silently falls back to inline — the very quirk we are fixing.

But `sync-agents.sh`'s two passes are **asymmetric**:

- `user_level_pass` iterates the **full** built-in set (`for src in agents/docket-*.md`).
- `project_level_pass` (as of 0046) iterates only the agents **listed** as keys in the
  `.docket.yml` `agents:` block (`agent_keys`).

So a repo that lists a subset of agents gets a subset of committed per-repo agent files, while a
dispatch rule naming the full set would have unresolved targets. This conflates "listed in config"
with "gets generated" — but the `agents:` block was only ever meant to carry **model/effort
overrides**, not to decide **which agents exist**. Which agents exist is fixed by docket's design:
the agents **compose** (implement-next dispatches status/adr; finalize dispatches
rebase-resolver/integration-repair; auto-groom dispatches its critic), so a harness needs all of
them regardless of which ones have a custom model.

## Goals

1. Generate a Cursor dispatch rule (`docket-dispatch.mdc`) alongside Cursor agents, at both the
   user-level and per-repo layers, from an authored source that stays in sync with the agent set.
2. Make agent generation **always write the full built-in agent set** into every targeted harness,
   so dispatch targets resolve by construction — turning Problem 1's "verify the targets exist"
   into a design invariant.
3. Make the whole generator **idempotent under removal**: when a built-in agent is removed, or a
   harness is de-listed from `agent_harnesses`, the orphaned docket-owned files are deleted.

## Non-goals

- A rules mechanism for non-Cursor harnesses (only Cursor exhibits the quirk).
- Validating model IDs against a harness's roster (docket stays passthrough per ADR-0015).
- A single dense dispatch **table** layout (rejected in favor of per-agent subsections).
- Cleaning up pre-existing **hand-authored** per-agent `.mdc` rule files a user created before this
  feature (the consolidated rule replaces them going forward; migration of stale hand files is the
  operator's concern).
- Folding this into 0046 (it stays "harness-first resolution"; this is a distinct change layered on
  it).

## Design

Three coherent pieces, all in `sync-agents.sh` (root-level script; no `scripts/` contract).

### Piece 1 — always-full-set generation

Flip `project_level_pass` to iterate the **full built-in set** (`agents/docket-*.md`), mirroring
`user_level_pass`, instead of iterating `agent_keys`. For each built-in agent × each harness in
`HARNESSES`, resolve the override with the existing `resolve_agent` (returns the configured
`model`/`effort` for `agents.<harness>.<agent>`, else `agents.default.<agent>`, else empty → the
wrapper's shipped built-in default via `emit`). `warn_fallback_model` continues to warn when a
**non-Claude** harness falls through to the built-in default (likely-wrong ID).

This unifies both passes on one iteration source; they now diverge only in:

| | override source | write target |
|---|---|---|
| user-level | `~/.config/docket/agents.yaml` (top-level, `under_block=0`) | `~/.<H>/agents/` for every present harness |
| per-repo | `.docket.yml` `agents:` block (`under_block=1`) | `<repo>/.<H>/agents/` for each `H ∈ agent_harnesses` |

The `agents:` block becomes **override-only**: an entry that names no real built-in stays a typo
warning; the `agents.<harness>` dead-config warning (harness not in `agent_harnesses`) is retained.
No config entry is ever required for an agent to be generated.

**Consequence:** a repo that previously committed a subset of per-repo agent files will, on the
next `sync-agents.sh` run, gain the remaining agents at their default models (plus the dispatch
rule). This is the intended reproducibility outcome — every clone, every harness, the full set.

### Piece 2 — the Cursor dispatch rule

**Source of truth** — a new top-level `cursor-rules/` directory, parallel to `agents/`:

- `cursor-rules/dispatch.head.md` — the **static preamble**: the `.mdc` frontmatter
  (`description`, `alwaysApply: true`), the `## Docket agents — dispatch only` intro, and the shared
  `### Required dispatch pattern` section (the 3-step "do not run inline; launch Task with matching
  `subagent_type`, `run_in_background: false`; relay the result").
- `cursor-rules/dispatch/docket-<name>.md` — **one fragment per agent**, a self-contained
  `## docket-<name> — dispatch only` subsection: the agent's trigger phrases (examples), what the
  dispatch prompt must include, what the parent must NOT do, a wraps-no-skill / composition note
  where relevant, and an example Task call. These are the substance of the operator's existing
  hand-authored per-agent rules, minus their individual frontmatter, reused as body fragments.

**Assembly** — a Cursor-only branch in the generator produces the rule by concatenation:

```
docket-dispatch.mdc = dispatch.head.md
                    + for each built-in agent (agents/docket-<name>.md), in glob order:
                        fragment cursor-rules/dispatch/docket-<name>.md   # if present
                        else a minimal block derived from the agent's `description` + generic example  (+ WARN)
```

- Iterating the **built-in agent set** is what makes "only agents that actually exist appear"
  automatic. Because Piece 1 generates that same full set into the harness, the rule's targets and
  the generated agent files are **equal by construction**.
- **Ordering is glob/alphabetical** (the same order `agents/docket-*.md` expands), for
  deterministic, drift-stable byte output.
- A fragment with **no matching built-in agent** is silently skipped (dormant fragment for an agent
  docket no longer ships).
- A built-in agent with **no fragment** gets a minimal auto-block (name + its `description` as the
  "when" + a generic `Task(subagent_type: "docket-<name>", …)` example) and a `log` warning nudging
  the author to write a proper fragment — a newly-added agent is never silently un-dispatched.

**Where it is written** — Cursor-only, keyed on the `cursor` harness token (a small
`HARNESS_HAS_DISPATCH_RULES={cursor}` notion, not a general per-harness plugin system):

- **User-level:** when `~/.cursor/` is present (the same root-presence test the agent pass uses),
  write the assembled rule to `~/.cursor/rules/docket-dispatch.mdc`.
- **Per-repo:** when `cursor ∈ agent_harnesses`, write it to `<repo>/.cursor/rules/docket-dispatch.mdc`
  (committed, clone-identical).

Because the rule is a **single file re-assembled whole on every run**, it is inherently
prune-safe: removing an agent (its wrapper + fragment) simply drops that subsection on the next
run — there is no per-agent `.mdc` to delete.

### Piece 3 — prune orphaned docket-owned files

After both emit passes, a `prune` step deletes docket-owned files whose source no longer exists.
Strictly scoped to `docket-*` filenames the generator owns — it never touches a non-docket file.

- **Removed built-in agent:** for each targeted harness's `agents/` dir (per-repo
  `<repo>/.<H>/agents/`; user-level `~/.<H>/agents/` for present harnesses), for each
  `docket-<name>.md`, if `agents/docket-<name>.md` (built-in) does not exist → `rm` it.
- **De-listed harness (per-repo only):** for any harness `H` **not** in `HARNESSES` that still has
  `<repo>/.<H>/agents/docket-*.md` or `<repo>/.<H>/rules/docket-dispatch.mdc` → `rm` those
  docket-owned files. `rmdir` `agents/` / `rules/` (and `.<H>/`) only if empty afterward, so a
  shared harness dir with the operator's own files is preserved.
- **Cursor de-listed:** the above removes `<repo>/.cursor/rules/docket-dispatch.mdc` and the
  `<repo>/.cursor/agents/docket-*.md` set.

Deletion is a **working-tree `rm`** — `sync-agents.sh` does no git (consistent with how it writes
files today); the surrounding skill/CI stages and commits the deletion.

**`--check` mode:** the prune step **reports** an orphan as drift (`rc=1`, log the path) but does
**not** delete — matching how `check_project_level` reports content drift without writing.

## Drift check

`check_project_level` extends to cover, per targeted harness:

1. Every built-in agent's committed per-repo file (already covered once Piece 1 makes it full-set).
2. The committed `<repo>/.cursor/rules/docket-dispatch.mdc` — re-assemble head+fragments and byte-diff.
3. Orphans (Piece 3 report path).

The rule's bytes are harness-independent (there is only the `cursor` harness for rules), so the
check re-assembles once and diffs the committed copy.

## Idempotency guarantees

- Re-running `sync-agents.sh` with unchanged inputs is a no-op (overwrite with identical bytes,
  nothing to prune).
- Adding a built-in agent → next run generates its wrapper into every targeted harness and adds its
  subsection to the rule (fragment, or minimal auto-block + warning).
- Removing a built-in agent → next run stops generating it, prunes its orphaned wrapper files, and
  drops its rule subsection.
- De-listing a harness from `agent_harnesses` → next run prunes that harness's docket-owned per-repo
  files (agents + dispatch rule).
- Removing an agent from the `agents:` block (but leaving the built-in) → the agent is **still**
  generated at its fallback/default model; only its model changes. Not a deletion (correct).

## Testing (`tests/test_sync_agents.sh`)

Add cases (using the `DOCKET_HARNESS_ROOT` test seam):

1. Per-repo now generates the **full** built-in set for a listed harness even when the `agents:`
   block lists only a subset; unlisted agents carry the built-in default model.
2. The assembled `docket-dispatch.mdc` contains a subsection for **every** generated agent and none
   for a non-existent one; content matches head+fragments; ordering is deterministic.
3. A built-in agent with no fragment produces the minimal auto-block + a warning.
4. Removing a built-in agent prunes its per-repo and user-level `docket-<name>.md` and drops its
   rule subsection.
5. De-listing `cursor` from `agent_harnesses` prunes `<repo>/.cursor/rules/docket-dispatch.mdc` and
   the `<repo>/.cursor/agents/docket-*.md` set, while leaving a co-located non-docket file intact.
6. Cursor-only: no dispatch rule is written for `claude`/`codex`/other harness dirs.
7. `--check` reports drift for a hand-edited committed rule and for an orphaned agent file, without
   deleting.
8. User-level: the rule is written to `~/.cursor/rules/` when `~/.cursor/` is present and skipped
   when absent.

## Documentation & ADR

- **`docket-convention`** "Agent layer": note that per-repo generation writes the full built-in set
  (config is override-only), and that the `cursor` harness additionally gets a generated
  `docket-dispatch.mdc` dispatch rule.
- **This repo's `.docket.yml`** commented example + README pointer: reflect override-only semantics.
- **ADR:** this refines (does not reverse) ADR-0015/0016 — the always-full-set invariant and the
  Cursor dispatch-rule companion. New ADR vs a dated `## Update` note decided at the build's ADR
  step.

## Out of scope

- Non-Cursor harness rule mechanisms.
- Model-ID validation.
- The single-table rule layout.
- Migrating pre-existing hand-authored per-agent `.mdc` files.
