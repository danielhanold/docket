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
outright. Re-run `install.sh` after editing any config layer — it delegates to docket's Go engine
(`docket development install`), which reconciles opencode's definitions and the committed `AGENTS.md`
dispatch block for you in one journaled transaction (change 0351). As a repository opt-in,
`agent_harnesses` has three states: *absent* keeps the shipped default (Claude only) and writes no
opencode surfaces, a *non-empty* list reconciles exactly the harnesses named, and an *explicit empty*
list (`agent_harnesses: []`) retires every docket-owned repository surface — including the shared
`AGENTS.md` block once the last `AGENTS.md`-dispatch harness is de-listed — that the repo previously
had. That same install run also retires the old **global** parent-facing dispatch blocks earlier
docket versions wrote into personal instruction files, proof-gated against docket's exact ownership
marker with no `--force`. Restart opencode after any run that changed a definition or the dispatch
block: it registers agents and reads `AGENTS.md` at process start, so clearing a conversation is not
enough.

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

## Delegating Claude Code agents to opencode (runner delegation)

Everything above configures opencode as the harness **hosting** your session. Runner delegation is
the other direction: your session stays in Claude Code, and individual docket agents are handed to
opencode — with its models and its bill — for their whole run. The motivating use is cost
asymmetry: through OpenRouter, opencode reaches DeepSeek-tier models at a fraction of a
frontier-model task, and because docket's build and review roles are already separate agents
(ADR-0063), you can send build work to cheap models while review stays on your Claude
subscription.

```yaml
# .docket.local.yml (this machine only) or the global ~/.config/docket/config.yml
agents:
  claude:                       # the PARENT harness: when Claude Code hosts the session…
    build-economy:  { runner: opencode, model: openrouter/deepseek/deepseek-v4-flash-0731, effort: medium }
    build-standard: { runner: opencode, model: openrouter/deepseek/deepseek-v4-flash-0731, effort: high }
    build-premium:  { runner: opencode, model: openrouter/moonshotai/kimi-k3,             effort: medium }
    build-max:      { runner: opencode, model: openrouter/moonshotai/kimi-k3,             effort: high }
    # review-lean / review-standard / review-deep: no runner: → native Claude Code
runners:
  opencode:
    permissions: auto-approve   # REQUIRED — see below. Default `ask` refuses to delegate.
```

Re-run `sync-agents.sh` after editing, and restart the parent session.

> **Where the child works.** `runner-dispatch.sh` anchors a delegated run at the repo's **main
> worktree** by default (`docket_main_worktree()`, cwd-independent by design — ADR-0034) and hands
> that path to `opencode run --dir`. That suits the metadata-scoped agents (`status`, `adr`), but a
> build worker's contract requires it to stay inside the feature worktree on its branch — so a
> delegated build worker runs in the tree named by `--worktree`, which the generated `build-*`
> shims carry, and the facade aborts a `build-*` delegation that names none rather than silently
> starting the child in the primary checkout.

**Delegate leaves, not orchestrators.** A delegated run's own sub-dispatches run child-natively, so
delegating an orchestrator drags everything beneath it into the child. Delegate
`docket-implement-next` and its review dispatch goes to opencode too. Delegating the four profile
workers rather than the `docket-build` controller is the same rule applied one level down:
delegating the controller would move the routing decision into the child as well.

**Model selection is explicit, by design.** The `opencode:` block in `agents/harness-defaults.yml`
is *not* consulted here, and that is deliberate. That block answers "if opencode ran this whole
project, what should each role cost?"; delegation asks a different question — "which rows do I want
to leave my Claude subscription, and which do I deliberately keep?" — and the build-delegated /
review-native split above is exactly that asymmetry. Cross-indexing the two would also mean
retuning the native table silently changed what your delegated Claude Code builds run on, with the
coupling invisible at the config site. So you write the models yourself, where you can see, grep,
review, and revert them.

Relatedly: **a delegated agent must carry a `model:`.** Docket never forwards its own shipped
default to another harness, so without one the run would fall through to opencode's own default —
pay-per-token, of unknown identity — and the mistake would surface on your bill rather than in the
run. `sync-agents.sh` refuses to generate a model-less delegation.

### Effort is optional — but it is not inherited either

`effort:` maps to `opencode run --variant`, opencode's provider-specific reasoning-effort knob.
Docket's vocabulary passes through **verbatim, with no mapping table**: `--variant` accepts
docket's `max` natively, unlike codex where `max` becomes `xhigh`.

The trap is that effort follows the **same provenance rule as model** — a value from docket's
shipped `agents/harness-defaults.yml` is never forwarded to a child harness — but unlike model
there is no error when it is missing. So this:

```yaml
build-max: { runner: opencode, model: openrouter/moonshotai/kimi-k3 }   # no effort
```

generates a shim with **no `--variant` flag at all**, and the run silently takes the provider's
default effort. The shipped `claude` effort for that agent is *not* used, because it is a Claude
default and means nothing to opencode; the shipped `opencode` effort is not used either, because
the runner path resolves under the parent harness. Write the effort explicitly, as the recipe above
does — or opt out deliberately with `effort: auto`, per the next section.

### Saying "no effort at all" — omit vs `auto`

There are two ways to end up with no `--variant`, and they are **not** equivalent:

| You write | Meaning | Result |
|---|---|---|
| no `effort:` key | "no opinion here" — the field still resolves from **lower config layers** | whatever a machine-local or global layer supplies; nothing only if no user layer sets one |
| `effort: auto` | "explicitly no pin" — wins the field and suppresses it | never any `--variant` |

So if your global `~/.config/docket/config.yml` pins an effort for that agent, omitting the key in
the repo does **not** hand you the model's default — the global value is user-configured, so it is
forwarded like any other. Use `auto` to actually opt out:

```yaml
build-max: { runner: opencode, model: openrouter/moonshotai/kimi-k3, effort: auto }
```

`auto` is docket's existing no-pin sentinel for effort and behaves identically on every runner.

Two smaller notes:

- **The vocabulary is model-specific.** Which tokens a model accepts is the provider's business,
  not opencode's or docket's (ADR-0015 — docket validates nothing). If a model rejects docket's
  token, change the token, not the model.
- **Effort needs a model to attach to.** `--variant` is a provider model option, so with no model
  resolved the adapter drops the effort with a warning rather than passing a dangling flag. Under
  the required-model rule that combination is unreachable through a generated shim, but it is
  reachable if you invoke the adapter by hand.

Note this is a **different mechanism from *Pinning models and effort* above**, which covers opencode
as the *hosting* harness: that path writes `reasoningEffort:` into `.opencode/agents/docket-*.md`,
read by opencode's own agent loader. Delegation bakes `--variant` into the shim's command line
instead. Same `effort:` key in your config, two different destinations.

### What `auto-approve` actually grants

opencode has no sandbox *levels*. It has a permission system that prompts before editing a file or
running a shell command, and `--auto` auto-approves everything **not explicitly denied** in
opencode's own config. Its own help text marks it `(dangerous!)`.

A delegated run cannot answer a prompt, so `runners.opencode.permissions` has two values and no
useful third:

| Value | Effect |
|---|---|
| `ask` (default) | The adapter **refuses to delegate**, with a diagnostic naming this knob. Without `--auto` the child would block on the first approval until something timed out; refusing turns a silent hang into a message. |
| `auto-approve` | Bakes `--auto`. A delegated build worker can then run any command in the repository unwatched, except what your opencode deny rules forbid. |

The default names what actually happens rather than serving as a placeholder, and nobody receives
blanket auto-approval as a side effect of typing `runner: opencode` — the risk is accepted at a
visible line in config. **Pair `auto-approve` with opencode's own deny rules**: `--auto` approves
what is not explicitly denied, so the deny list — not the flag — is *intended* to be the real
boundary. Treat that as unverified: docket read it from a single line of `opencode run --help` and
has not tested the interaction. Confirm the deny-rule spelling and behavior against your own
opencode version before relying on it to contain a delegated build worker.

### Verifying a delegated run

Model IDs and entitlement live outside this repo, so no docket test can validate them — confirm
them yourself:

```sh
opencode models openrouter        # the IDs above must appear, spelled exactly
```

Catalog presence is not entitlement: an ID can be listed and still fail under your credentials.
Certify one real dispatch end to end before trusting the setup — ask for a `build-economy` task and
confirm the work happened in opencode, not Claude Code.

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
