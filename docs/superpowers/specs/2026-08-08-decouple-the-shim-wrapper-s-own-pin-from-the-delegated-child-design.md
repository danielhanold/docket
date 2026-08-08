<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0269 — Decouple the shim wrapper's own pin from the delegated child's](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0269-decouple-the-shim-wrapper-s-own-pin-from-the-delegated-child.md)**
<!-- docket:backlink:end -->

# Decouple the shim wrapper's own pin from the delegated child's

## Problem

`sync-agents.sh`'s `emit_shim` builds a runner-delegation shim's frontmatter by calling
`emit "$1" "$2" "$3"` with the **resolved child** model and effort, then replaces the body with the
delegation instructions:

```bash
emit_shim(){  # $1=src $2=model $3=effort $4=runner $5=agent-name $6=flag-model $7=flag-effort
  emit "$1" "$2" "$3" | awk '/^---[[:space:]]*$/{d++; print; next} d<2{print}'
```

The shim agent runs **in the parent harness** — Claude Code. Its entire job is one foreground
`docket.sh runner-dispatch` call plus a stdout relay; the delegated work happens in the child
process, reached through the baked `--model` argument. Pinning the shim's own frontmatter to the
child's model tells Claude Code to run the relay on a model Claude Code cannot resolve. The run dies
with a bare harness error before `runner-dispatch.sh` is ever invoked, so the failure does not even
name the runner.

Observed on 2026-08-08 during a `docket-implement-next 258` run: every `docket-build-*` dispatch
failed, with the wrappers pinned to `openrouter/deepseek/deepseek-v4-flash-0731`.

### Why it is guaranteed, not incidental

ADR-0067 requires a runner-bearing agent to carry a **user-configured** model and explicitly rejects
`inherit` as a bypass. So a delegated claude agent always has a child model ID resolved and
available to be copied into the wrong slot. There is no configuration that avoids the defect —
every runner-delegated claude wrapper is born broken.

### The false premise

ADR-0038 (Accepted, change 0079) decided this deliberately:

> generates the SAME wrapper file with the SAME frontmatter (name, description, and the `model:`
> line kept for bookkeeping — the effective pin is the baked argument)

and lists byte-stability against the native wrapper shape among its accepted consequences. The
frontmatter `model:` is not bookkeeping. Claude Code reads it as the live pin for the shim agent.
The premise holds only for a reader that never executes the wrapper.

## Scope

`emit_shim` is reachable only when `harness = claude`: `runner_config_error` returns 0 early for any
other harness, and `runner:` under a non-claude harness is warned-and-ignored. Codex and cursor
generation are untouched by this change.

## Design

### Two new per-runner keys

| Key | Default when unset | Emitted as |
|---|---|---|
| `runners.<name>.shim_model` | `inherit` | the shim's frontmatter `model:` |
| `runners.<name>.shim_effort` | `low` | the shim's frontmatter `effort:` |

Per-runner rather than per-agent: the value answers "what can this machine's parent harness reach
cheaply," which does not vary by which skill is being delegated.

### Emission

`emit_wrapper` resolves both values alongside the config it already resolves and passes them to
`emit_shim` **in place of** the child model and effort. `emit_shim`'s `$2`/`$3` are repurposed, not
extended:

```bash
emit_shim(){  # $1=src $2=shim-model $3=shim-effort $4=runner $5=agent-name $6=flag-model $7=flag-effort
```

Arity is unchanged, `emit_shim` stays a pure emitter with no config access of its own, and the
child's values continue to ride `--model $6` / `--effort $7` untouched.

`emit` already supports the needed output. Its own comment documents that `model: inherit` passes
through **verbatim** on the claude harness, because Claude Code treats `inherit` as a real value
meaning "run on the parent conversation's model" — a different runtime outcome from omitting the
key. No change to `emit` is required.

### Why `inherit` as the default

Every currently-broken wrapper is repaired by regeneration alone, with no config edit. The knob
becomes a cost optimization rather than a prerequisite: setting
`shim_model: claude-haiku-4-5-20251001` stops the relay from burning session-model tokens, but
leaving it unset is correct and working.

### Relationship to ADR-0067

ADR-0067 governs **config input** for the child model and is untouched. `shim_model` governs
**generator output** for the parent-side pin. `inherit` is explicitly allowed here precisely because
there is no guard to bypass — the user-configured child model is still mandatory and still lands in
`--model`.

### Config layering and the fence

`runners.*` carries no coordination-key fence in `docket-config.sh`, and `runner-dispatch.sh` already
layers the block repo-local > repo-committed > global, per-key. Both new keys inherit that
unchanged. Machine-scoped placement is correct here: which models the parent harness can reach is a
per-machine fact, not shared coordination state.

Both values take the same bare-scalar validation as `model:`/`effort:` — an unquoted, space-free
scalar; a quoted or spaced value is a generation-time diagnostic.

### The reader

`sync-agents.sh` has no access to the `runners:` block today. It carries only a static registry of
names:

```bash
REGISTERED_RUNNERS="codex cursor opencode"
```

The layered parse already exists as `runner_block()` in `runner-dispatch.sh`. This change adds the
smallest reader in `sync-agents.sh` that resolves `runners.<name>.shim_model` and
`runners.<name>.shim_effort` across the layers, and **does not** attempt to unify the two parsers.

That is a deliberate deferral, and it has a cost: it makes `sync-agents.sh` the second independent
consumer of the `runners:` block, which is exactly the duplication change 0256 exists to resolve.
Folding 0256 in would roughly double this change. 0256 is linked in `related:` and should absorb
both readers when it lands.

## ADR

ADR-0038 is `Accepted`, and an Accepted ADR's Decision is never edited. This change records a **new**
ADR with `supersedes: [38]`, stating that a shim's frontmatter pin governs the parent-side shim
agent and must therefore be resolvable by the **parent** harness; the child's pin is the baked
dispatch argument and only that. It should also retire ADR-0038's byte-stability-with-the-native-
wrapper consequence, which is the rationale that produced the defect.

## Testing

`tests/test_sync_agents.sh` gains:

1. A delegated wrapper's frontmatter carries `model: inherit` and `effort: low` when neither knob is
   set.
2. It carries the configured values when `shim_model` / `shim_effort` are set.
3. The child's model and effort still land in the baked `--model` / `--effort` arguments.
4. **The regression assert:** a shim's frontmatter `model:` is never equal to the value baked into
   `--model`. This is the check whose absence let the defect ship.
5. Bare-scalar validation rejects a quoted or spaced `shim_model`.

## Documentation

- `scripts/runners/*.md` — the shim frontmatter description in each runner contract.
- `.docket.example.yml` and `config.yml.example` — the two new keys, with the defaults.
- `README.md` Configuration section.
- `docket-convention`'s agent-layer reference, where the shim shape is described.

## Out of scope

- **The `runners.opencode.permissions` locality defect.** `.docket.local.yml` is gitignored, so a
  freshly created feature worktree has no copy. A build worker is dispatched with
  `--worktree <feature worktree>` and the adapter resolves config from `DOCKET_REPO_ROOT` = that
  worktree, so a `permissions: auto-approve` grant written in another worktree resolves back to the
  default `ask` and the adapter refuses. Real, independently reproducible, and its own change.
- **Config-reader consolidation** — change 0256.
- Any change to ADR-0067's requirement that a runner-bearing agent carry a user-configured model.
- Codex and cursor wrapper generation.
