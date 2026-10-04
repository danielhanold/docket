# opencode: running docket under opencode

opencode is a first-class docket harness. An install writes two kinds of opencode artifact:

- **User-level skills and agent definitions**, under
  `${XDG_CONFIG_HOME:-~/.config}/opencode/`: docket's skills are linked under `skills/`, and its 17
  agent definitions are written to `agents/docket-*.md` — markdown with YAML frontmatter, one per
  docket agent. The **filename is the agent identifier**, so no `name:` field is written.
  Definitions are user-level only; no repository carries its own copies. The built-in table (mirrored
  in `agents/harness-defaults.yml`) has a complete `opencode:` block, so every definition is
  generated **pinned** with no configuration at all.
- **A `docket` dispatch block in `AGENTS.md`, on opt-in only** — the same marker-bounded repo-root
  block, **shared with Codex**: opencode reads the same committed project-root `AGENTS.md`, so one
  managed block serves both. A repo targeting either harness gets it; a repo targeting both gets it
  exactly once. It is **committed and machine-neutral** (ADR-0036).

### The opt-in you need

The user-level definitions need no opt-in. The `AGENTS.md` block is written only when a
repository's own `.docket.yml` or `.docket.local.yml` lists `opencode` in `agent_harnesses`:

```yaml
agent_harnesses: [claude, opencode]
```

A global `agent_harnesses` writes nothing into a repository; the installer ignores it. The first of
local-then-committed that declares the key wins the list outright. Re-run the install after editing
it. The three opt-in states (absent / non-empty / explicit empty) and the shared-block removal rule
are exactly as described for [Codex](codex.md); because the block is shared, it is removed only when
the **last** `AGENTS.md`-dispatch harness is de-listed, and your own `AGENTS.md` content outside the
docket markers is preserved untouched.

### Pinning models and effort

The generated definitions carry the model and effort resolved from your **global** config's
`agents:` table over the built-in `opencode:` block; agent overrides are honoured from the global
file only, and your value overrides the shipped pin. **Models are reached through OpenRouter**,
so authenticate that provider first (`opencode auth login`). OpenRouter model IDs are
**double-prefixed** — `openrouter/<vendor>/<model>`, e.g.
`openrouter/deepseek/deepseek-v4-flash-0731`. opencode splits that into a provider id (`openrouter`)
and a model id itself; docket passes the whole string through untouched and validates nothing
(ADR-0015). Use exactly the IDs opencode reports:

```sh
opencode models openrouter
```

**Effort is a provider model option, not a first-class opencode field.** opencode has no
reasoning-effort field of its own; it forwards unrecognized agent-frontmatter keys to the provider as
model options. Docket therefore emits effort as `reasoningEffort:`, and it arrives as a real
per-agent reasoning effort. Two consequences: it only applies when a model resolves (docket
silently drops the effort when the model is unset or is the `model: inherit` sentinel, which has no
opencode equivalent), and the vocabulary is model-specific — `high` is the ceiling docket ships; if
your model rejects docket's token, pin `effort` explicitly alongside your model.

### Verifying it works

After opting a repo in and running the install:

1. `${XDG_CONFIG_HOME:-~/.config}/opencode/agents/docket-*.md` exist — 17 of them.
2. The opted-in repository's `AGENTS.md` contains the marker-bounded `docket` dispatch block.
3. Ask opencode to resolve one definition and read back what it actually applied:

```sh
opencode debug agent docket-build-economy
```

It prints the fully resolved config; `model` split into `providerID` + `modelID` confirms the
double-prefixed ID parsed, and the effort appearing under `options` (not as a top-level field) is
exactly the passthrough described above. Compare the values against the `opencode:` block in
`agents/harness-defaults.yml`, the shipped mirror of the built-in table — a field you set in your
global config wins over the shipped value.

### Restart after (re)generating

opencode loads its config once at process start and does **not** hot-reload. After an install writes
new definitions, quit and restart opencode before invoking a docket skill — an already-open session
keeps the old definitions.
