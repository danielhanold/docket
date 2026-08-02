# opencode setup — enabling docket's opencode harness

opencode is a first-class docket harness. `sync-agents.sh` generates two opencode artifacts:

- **`.opencode/agents/docket-*.md`** — the agent definitions: markdown with YAML frontmatter, one
  per docket agent. The **filename is the agent identifier**, so no `name:` field is written. These
  are **machine-local**: gitignored, regenerated per machine, never committed (they bake resolved
  model IDs — ADR-0020). `agents/harness-defaults.yml` ships a complete sixteen-agent `opencode:`
  block, so all sixteen definitions are generated **pinned** with no configuration at all. Any
  field is overridable per agent from any config layer — your value wins over the shipped one. The
  pins are opencode-native: a Claude model ID means nothing to opencode, so an ID is never lent
  across harnesses.
- **A `docket` dispatch block in `AGENTS.md`** — a marker-bounded block in the repo-root
  `AGENTS.md` that tells the hosting harness to delegate a directly-invoked docket skill to its
  matching generated agent — pinned or not, since the agent also carries the skill's dispatch
  contract and preload. **This block is SHARED with Codex**: opencode reads the same committed
  project-root `AGENTS.md`, so one managed block serves both. A repo targeting either harness gets
  it; a repo targeting both gets it exactly once. It is **committed and machine-neutral**: it
  carries only agent names and delegation prose, never a model ID or effort value, so it is
  clone-identical across machines (ADR-0036).

Docket's skills themselves need no extra step: opencode resolves skills out of `~/.agents/skills/`,
which docket's `link-skills.sh` already populates on install.

## Two scopes — and the opt-in you need

`sync-agents.sh` writes in two passes, and opencode lives in **both**:

| Pass | Writes | Governed by |
|---|---|---|
| **User-level** | `~/.opencode/agents/docket-*.md` | **global** `agent_harnesses:` in `~/.config/docket/config.yml` |
| **Per-repo** | `<repo>/.opencode/agents/docket-*.md` **and** the `AGENTS.md` dispatch block | the **repo's own** `agent_harnesses:` in `.docket.yml` or `.docket.local.yml` |

**The gotcha: a global `agent_harnesses` does NOT generate per-repo opencode artifacts.** Setting
`agent_harnesses: [claude, opencode]` in `~/.config/docket/config.yml` writes
`~/.opencode/agents/…` but produces **nothing** inside a repo — no `.opencode/agents/*.md`, no
`AGENTS.md` block — and `sync-agents.sh` prints no explanation. To get the per-repo artifacts, the
**repo** must opt in:

```yaml
# in <repo>/.docket.yml  — commits the choice for the whole team
agent_harnesses: [claude, opencode]
```

```yaml
# or in <repo>/.docket.local.yml  — this machine only, gitignored, never leaves your clone
agent_harnesses: [claude, opencode]
```

Either file opts the repo in; the first of local-then-committed that declares the key wins the list
outright. Re-run `sync-agents.sh` (or `install.sh`) after editing any config layer.

**Why it works this way.** The `AGENTS.md` dispatch block is *committed*. If a global setting on
your machine generated that committed block, a collaborator (or CI) without the same global config
would fail `sync-agents.sh --check` — their clone would see a `docket` block that their own
`agent_harnesses` doesn't call for. Making per-repo targeting come from the repo's own committed
(or machine-local) config keeps the committed artifact deterministic across every clone.

> Note: because the block is shared, it is removed only when the **last** `AGENTS.md`-dispatch
> harness is de-listed. De-listing opencode from a repo that still targets Codex (or the reverse)
> leaves the block in place, correctly. Your own `AGENTS.md` content outside the docket markers is
> preserved untouched.

## Pinning models and effort

The generated definitions carry the model/effort resolved from the layered `agents:` config over
docket's shipped `opencode:` block. That config **overrides** the shipped pin rather than being the
only source of one — set it for any agent whose tier you want retuned.

**Models are reached through OpenRouter**, so authenticate that provider first:

```sh
opencode providers          # alias: opencode auth
```

OpenRouter model IDs are **double-prefixed** — `openrouter/<vendor>/<model>`, e.g.
`openrouter/deepseek/deepseek-v4-flash-0731`. opencode splits that into a provider id
(`openrouter`) and a model id (`deepseek/deepseek-v4-flash-0731`) itself; docket passes the whole
string through untouched and validates nothing (ADR-0015). Use exactly the IDs opencode reports:

```sh
opencode models openrouter
```

Set them per agent under `agents:` (harness-first) in whichever config layer applies — see the main
README's agent-layer section for the full precedence rules.

**Effort is a provider model option, not a first-class opencode field.** opencode has no reasoning
-effort field of its own; it forwards unrecognized agent-frontmatter keys to the provider as model
options. Docket therefore emits effort as `reasoningEffort:`, and it arrives as a real per-agent
reasoning effort. Two consequences:

- **It only applies when a model resolves.** A provider option with no provider selected has
  nothing to reach, so docket drops the effort (with a warning) when the model is unset or is the
  `model: inherit` sentinel, which has no opencode equivalent.
- **The vocabulary is model-specific.** Which effort tokens a model accepts is the provider's
  business, not opencode's or docket's. `high` is the ceiling docket ships; if your model rejects
  docket's token, pin `effort` explicitly alongside your model.

## Verifying it works

After opting a repo in and running `sync-agents.sh`:

1. `.opencode/agents/docket-*.md` exist — sixteen of them.
2. `AGENTS.md` contains the marker-bounded `docket` dispatch block.
3. Ask opencode to resolve one definition and read back what it actually applied:

```sh
opencode debug agent docket-build-economy
```

It prints the fully resolved config. The shape to expect (permissions elided):

```json
{
  "name": "docket-build-economy",
  "mode": "subagent",
  "model": { "providerID": "openrouter", "modelID": "deepseek/deepseek-v4-flash-0731" },
  "options": { "reasoningEffort": "medium" },
  "description": "…"
}
```

`model` split into `providerID` + `modelID` confirms the double-prefixed ID parsed; the effort
appearing under `options` (not as a top-level field) is exactly the passthrough described above.
Compare the values against the `opencode:` block in `agents/harness-defaults.yml`, which is the
single source of truth — a field you set in a config layer wins over the shipped value.

`sync-agents.sh --check` validates the `AGENTS.md` block's presence and currency (it is exempt from
the tracked-file leg — the block is *meant* to be committed) and flags a stale or missing block for
CI.

## Restart after (re)generating

opencode loads its config once at process start and does **not** hot-reload. After
`sync-agents.sh` writes new definitions, quit and restart opencode before invoking a docket skill —
an already-open session keeps the old definitions.
