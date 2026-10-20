# Models: tuning model and effort per task

**Why pin a model per task.** Most harnesses invite one mental model: *one session, one model.* You
choose a tier when you start, and everything that hour runs at it. That is how you end up paying
top-tier prices to regenerate a **board** (the generated overview of every change and its state,
never edited by hand) — and thinking at the cheap tier while designing a build. Both are the same
mistake in opposite directions: the model was matched to the **session** instead of to the task.

docket's unit of work is the skill, so the tier is a property of the skill, not of your session.
Each docket agent ships pinned to the model and effort its job calls for, and the built-in values
are shown in [`.docket.example.yml`](../../.docket.example.yml). Different agents run in the **same
session**, minutes apart, each at its own model and effort, and you never pick one.

Each **autonomous** docket skill runs as a model/effort-pinned **agent** — a separately launched
worker with its own context, pinned to a model and effort (`docket-implement-next`,
`docket-auto-groom`, `docket-finalize-change`, `docket-status`, `docket-adr`).

The two interactive skills, `docket-new-change` and `docket-groom-next`, run at whatever model and
effort your session uses, so you choose them. Pick by preference and by how complex you expect the
change to be: a quick stub or a small revise needs little, while a net-new
design, a cross-cutting change, or a stub whose premise needs tracing through the code is worth
your strongest model at higher effort.

To change the model or effort an autonomous skill runs at:

**1. Edit your global config.** Pins live in the `agents:` block of your global
`~/.config/docket/config.yml` (see [Global config](global-config.md)), and only there: docket
installs agent wrappers for your user, never per repository. An `agents:` pin in a repository's
`.docket.yml` or `.docket.local.yml` — even one that repeats the built-in value — makes docket
refuse to change the repository until you move it to the global config. Each pin applies per agent and per field over the built-in
table compiled into docket; a field you leave out keeps its built-in value, and a `default:` block
applies to every harness that has no pin of its own for that agent.

The block's exact shape — harness, then agent, then `model` and `effort` — is shown in
[`.docket.example.yml`](../../.docket.example.yml), with the built-in values as a starting point;
copy from there rather than from a snippet, so the shape has a single source of truth.

**`model: inherit` on Claude Code.** `inherit` is a Claude Code frontmatter value, not a docket
keyword: it tells Claude Code to run the subagent on the **parent conversation's** model, which is a
different outcome from setting no model at all (Claude Code's own subagent default). docket passes it
through to a `.claude` wrapper verbatim. Cursor, Codex, and opencode have no equivalent, so on those
harnesses `inherit` is treated as "no pin" and the wrapper is generated without a model, letting the
harness apply its default.

**Changing only the model?** To override an agent's model while *dropping* its pinned effort — e.g.
pointing it at another harness's model, where Claude Code's effort tiers do not apply — set `effort:
auto`, which drops the effort line entirely so the agent inherits the model default. Omitting the
`effort:` key instead *keeps* the built-in effort, so `auto` is the explicit way to drop it.

**Prompt-cache TTL (Claude Code).** An agent entry may also set `cache_ttl`, how long Claude Code
keeps that agent's prompt cache. docket ships `cache_ttl: 1h` on Claude Code's `implement-next`
only. It waits on builds and tests longer than the 5-minute default cache lasts, so it kept paying
to write its whole context into the cache again, and the 1-hour TTL roughly halves its cost per
run. The other agents rarely wait that long, and a 1-hour cache write costs more, so they keep the
default. `cache_ttl: auto` removes a pin. Cursor, Codex, and opencode have no such setting: install
leaves the field out of their wrappers and warns. The [agent layer
reference](../../skills/docket-convention/references/agent-layer.md#prompt-cache-ttl) covers the
measurements, Claude Code's machine-wide alternatives, and which one wins.

**Finding model IDs.** A `model:` value is passed to the harness verbatim — docket never validates
it — so use exactly the IDs your harness reports:

| Harness | List available model IDs |
|---|---|
| Claude Code | `ant models list` |
| Cursor | `cursor-agent models` |
| Codex | `codex debug models \| jq -r '.models[] \| .slug'` |
| opencode | `opencode models openrouter` |

**2. Refresh the generated wrappers.** The resolved model and effort are baked into generated
wrapper *copies* (not symlinks), so after editing your global config, regenerate them:

```bash
bash ~/dev/docket/install.sh   # runs docket development install --source ~/dev/docket
```

The install rewrites the seventeen agent wrappers in every **present** harness's user-level agents
directory:

| Harness | Wrapper directory |
|---|---|
| Claude Code | `~/.claude/agents/` |
| Codex | `~/.codex/agents/` |
| Cursor | `~/.cursor/agents/` |
| opencode | `${XDG_CONFIG_HOME:-~/.config}/opencode/agents/` |

`docket install check` reports whether this machine's installation is current; it writes nothing
and checks only your machine.

### How the pin survives a direct invocation

Both Cursor and Claude Code run a *directly-invoked* skill — a human typing `/docket-status`, or the
model auto-invoking it — inline at the session model, which silently defeats the wrapper's
model/effort pin. They fix it with **two mechanisms**: Cursor uses a generated `docket-dispatch.mdc`
rule (in a repository whose `agent_harnesses` includes `cursor`) that forces a real dispatch; **Claude Code uses native `context: fork` + `agent: docket-<name>`
frontmatter** committed in each forked skill's `SKILL.md`, which forks the invocation into the same
pinned wrapper. That frontmatter is inert in every other harness (unknown keys are ignored), so one
shared `SKILL.md` serves all of them, and it degrades to inline behavior on a Claude Code too
old to know the field. **Fork-exclusion principle:** only skills that never need the human mid-run
are forked — a forked subagent has no channel to the human (Claude Code withholds `AskUserQuestion`,
`EnterPlanMode`, and similar from subagents). So the four headless-safe autonomous skills —
`docket-status`, `docket-adr`, `docket-implement-next`, `docket-auto-groom` — carry the frontmatter;
the two interactive brainstorm skills (`docket-new-change`, `docket-groom-next`) and
`docket-finalize-change` (which keeps a real prompt — the multi-candidate batch confirmation — so a
headless drive is authorized by
[naming ids](../guide/landing-changes.md) instead) do not.

**The two invocation paths.** **Dispatch** — launching a named agent to do a step and waiting for it
to return — is one way to reach a pinned wrapper; a plain skill-invoke, forked, is the other. Both
land a directly-invoked skill on the *same* pinned wrapper, so the model and effort are identical
either way. What differs is what **you** see while it runs:

| Path | How | You get | You give up |
|---|---|---|---|
| **Skill-invoke** | `/docket-status`, or the model auto-invoking the skill | The pinned run, forked — cheapest, no dispatch turn | Observability: it returns as `completed (forked execution)`, with no box to drill into in the TUI |
| **Agent-dispatch** | `@docket-status`, or a subagent dispatch naming the wrapper | The **identical** pinned run, drillable live in the TUI | One dispatch turn of overhead |

Reach for **agent-dispatch when you want to watch a long run** — a build you intend to babysit — and
**skill-invoke for everything else**.

`docket-implement-next` is the exception: start it with a plain request in the session (for
example "implement 412") or with `/loop docket-implement-next`, not a skill-invoke. In Claude Code,
where the skill runs forked, a skill-invoke, typed or invoked by the model, goes straight to the
implementer at the same pinned model and effort, but the session never runs `run.start`, so the run
tracker never sees it.
A typed `/docket-implement-next` runs untracked.
It still builds to a PR, but if it stops partway there is no automatic retry, resume, or cancel, and
its outcome can only be read unattributed (`docket run verdict --unattributed`). See
[Run tracker](../concepts/run-tracker.md).
