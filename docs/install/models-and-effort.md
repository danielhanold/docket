# Models: tuning model and effort per task

**Why pin a model per task.** Most harnesses invite one mental model: *one session, one model.* You
choose a tier when you start, and everything that hour runs at it. That is how you end up paying
top-tier prices to regenerate a **board** (the generated overview of every change and its state,
never edited by hand) — and thinking at the cheap tier while designing a build. Both are the same
mistake in opposite directions: the model was matched to the **session** instead of to the task.

docket's unit of work is the skill, so the tier is a property of the skill, not of your session. A
`docket-status` sweep is mechanical file bookkeeping — the cheap tier. A `docket-implement-next`
build and the autonomous design pass before it both reason hardest about the **change** (one unit of
planned work, roughly one pull request, tracked as one markdown file), so both run the top tier —
but the build spends more effort, because turning a settled design into working code demands more
deliberation than drafting the design did. Recording the decision it reached (`docket-adr`) runs
that same top-tier model at the lowest effort. They run in the **same session**, minutes apart, each
at its own model and effort, and you never pick one.

Each **autonomous** docket skill runs as a model/effort-pinned **agent** — a separately launched
worker with its own context, pinned to a model and effort (`docket-implement-next`,
`docket-auto-groom`, `docket-finalize-change`, `docket-status`, `docket-adr`; the two interactive
skills, `docket-new-change` and `docket-groom-next`, stay inline and only surface an advisory
recommendation). To change the model or effort one of them runs at:

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
