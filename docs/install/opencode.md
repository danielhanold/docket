# opencode: running docket under opencode

opencode is a first-class docket harness. An install generates two opencode artifacts:

- **`.opencode/agents/docket-*.md`** — the agent definitions: markdown with YAML frontmatter, one per
  docket agent. The **filename is the agent identifier**, so no `name:` field is written. These are
  **machine-local**: gitignored, regenerated per machine, never committed (they bake resolved model
  IDs — ADR-0020). `agents/harness-defaults.yml` ships a complete sixteen-agent `opencode:` block, so
  all sixteen definitions are generated **pinned** with no configuration at all. Any field is
  overridable per agent from any config layer — your value wins over the shipped one.
- **A `docket` dispatch block in `AGENTS.md`** — the same marker-bounded repo-root block, **shared
  with Codex**: opencode reads the same committed project-root `AGENTS.md`, so one managed block
  serves both. A repo targeting either harness gets it; a repo targeting both gets it exactly once.
  It is **committed and machine-neutral** (ADR-0036).

docket's skills themselves need no extra step: opencode resolves skills out of `~/.agents/skills/`,
which the install already populates.

### The opt-in you need

An install writes in two passes, and opencode lives in **both**:

| Pass | Writes | Governed by |
|---|---|---|
| **User-level** | `~/.opencode/agents/docket-*.md` | **global** `agent_harnesses:` in `~/.config/docket/config.yml` |
| **Per-repo** | `<repo>/.opencode/agents/docket-*.md` **and** the `AGENTS.md` dispatch block | the **repo's own** `agent_harnesses:` in `.docket.yml` or `.docket.local.yml` |

**The gotcha: a global `agent_harnesses` does NOT generate per-repo opencode artifacts.** Setting
`agent_harnesses: [claude, opencode]` in `~/.config/docket/config.yml` writes `~/.opencode/agents/…`
but produces **nothing** inside a repo. To get the per-repo artifacts, the **repo** must opt in, in
either its committed `.docket.yml` or its machine-local `.docket.local.yml`:

```yaml
agent_harnesses: [claude, opencode]
```

The first of local-then-committed that declares the key wins the list outright. Re-run `install.sh`
after editing any config layer — it reconciles opencode's definitions and the committed `AGENTS.md`
dispatch block for you in one journaled transaction (change 0351). The three opt-in states (absent /
non-empty / explicit empty) and the shared-block removal rule are exactly as described for
[Codex](codex.md); because the block is shared, it is removed only when the **last**
`AGENTS.md`-dispatch harness is de-listed, and your own `AGENTS.md` content outside the docket
markers is preserved untouched.

### Pinning models and effort (opencode as the host)

The generated definitions carry the model/effort resolved from the layered `agents:` config over
docket's shipped `opencode:` block, which **overrides** the shipped pin. **Models are reached through
OpenRouter**, so authenticate that provider first (`opencode auth login`). OpenRouter model IDs are
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
per-agent reasoning effort. Two consequences: it only applies when a model resolves (docket drops the
effort with a warning when the model is unset or is the `model: inherit` sentinel, which has no
opencode equivalent), and the vocabulary is model-specific — `high` is the ceiling docket ships; if
your model rejects docket's token, pin `effort` explicitly alongside your model.

### Delegating Claude Code agents to opencode

Everything above configures opencode as the harness **hosting** your session. Runner delegation is
the other direction — your session stays in Claude Code and individual docket agents are handed to
opencode, with its models and its bill, for their whole run. The config recipe and its full rules
live under [Delegation](delegating-across-harnesses.md); this is a **different mechanism** from the
hosting path here: hosting writes `reasoningEffort:` into `.opencode/agents/docket-*.md`, read by
opencode's own agent loader, while delegation bakes `--variant` into the shim's command line. Same
`effort:` key in your config, two different destinations.

The one grant delegation demands is `runners.opencode.permissions: auto-approve`. opencode has no
sandbox *levels*; it has a permission system that prompts before editing a file or running a command,
and `--auto` auto-approves everything **not explicitly denied** in opencode's own config (its own
help text marks it `(dangerous!)`). A delegated run cannot answer a prompt, so the knob has two
values and no useful third:

| Value | Effect |
|---|---|
| `ask` (default) | The adapter **refuses to delegate**, with a diagnostic naming this knob — turning a silent hang into a message. |
| `auto-approve` | Bakes `--auto`. A delegated build worker can then run any command in the repository unwatched, except what your opencode deny rules forbid. |

Nobody receives blanket auto-approval as a side effect of typing `runner: opencode` — the risk is
accepted at a visible line in config. **Pair `auto-approve` with opencode's own deny rules**: `--auto`
approves what is not explicitly denied, so the deny list — not the flag — is *intended* to be the
real boundary. Treat that as unverified: docket read it from a single line of `opencode run --help`
and has not tested the interaction, so confirm the deny-rule spelling and behavior against your own
opencode version before relying on it.

### Verifying it works

After opting a repo in and running the install:

1. `.opencode/agents/docket-*.md` exist — sixteen of them.
2. `AGENTS.md` contains the marker-bounded `docket` dispatch block.
3. Ask opencode to resolve one definition and read back what it actually applied:

```sh
opencode debug agent docket-build-economy
```

It prints the fully resolved config; `model` split into `providerID` + `modelID` confirms the
double-prefixed ID parsed, and the effort appearing under `options` (not as a top-level field) is
exactly the passthrough described above. Compare the values against the `opencode:` block in
`agents/harness-defaults.yml`, the single source of truth — a field you set in a config layer wins
over the shipped value.

### Restart after (re)generating

opencode loads its config once at process start and does **not** hot-reload. After an install writes
new definitions, quit and restart opencode before invoking a docket skill — an already-open session
keeps the old definitions.
