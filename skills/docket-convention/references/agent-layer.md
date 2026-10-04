# Agent layer — configuring model/effort-pinned subagents

> On-demand detail for the convention's *Agent layer* — read before configuring agent model/effort pins or
> `agent_harnesses`, or debugging the agent install. The runtime contract (which skills get wrappers, dispatch
> semantics, abort-and-report) stays in `SKILL.md`'s *Agent layer* stub; this file is the configuration mechanics.

Contents: [What a wrapper is](#what-a-wrapper-is) · [Where wrappers live](#where-wrappers-live) · [Model and effort pins](#model-and-effort-pins) · [Per-harness wrapper shapes](#per-harness-wrapper-shapes) · [Repository dispatch blocks: agent_harnesses](#repository-dispatch-blocks-agent_harnesses) · [Launch posture](#launch-posture) · [Invocation paths](#invocation-paths) · [Checking the install](#checking-the-install)

## What a wrapper is

A wrapper is a thin agent definition rendered from docket's `agents/docket-*.md` sources. It carries the resolved
model and effort, when they resolve, and lists the skills it preloads in its skills list; the skill body stays the
single source of behavior. Every wrapper also opens with a self-recursion guard: a wrapper already running as
`docket-<name>` carries out its charter directly and never dispatches another `docket-<name>` to do it.

## Where wrappers live

Wrappers are **installed at user level** by the `install` operation, never per repository: Claude Code's go to
`~/.claude/agents/`, and each other harness's go to its own user-level agent directory. The install writes no
agent definitions into a repository. Without `--harness`, the install targets every harness detected on this
machine, widened by the current repository's `agent_harnesses`; a repeatable `--harness <name>` names the targets
exactly. The same transaction links the machine-global skills and reconciles the repository's dispatch blocks (see
[below](#repository-dispatch-blocks-agent_harnesses)).

A harness registers agents and skills at process start, so restart the harness process after any install that
changed a wrapper or a skill; an already-open session still runs the old definitions.

## Model and effort pins

The built-in model/effort table is compiled into the binary; `agents/harness-defaults.yml` is its shipped copy, and
a test keeps the two equal. Every shipped harness (`claude`, `codex`, `cursor`, `opencode`) carries a complete
entry for every agent, so a wrapper is pinned out of the box.

Overrides (`agents.<harness>.<agent>.model|effort`) come **from the global configuration only** —
`${XDG_CONFIG_HOME:-~/.config}/docket/config.yml`. An agent pin in `.docket.yml` or `.docket.local.yml` blocks
writes. Overrides apply per agent and per field: a model override leaves the built-in effort in place, and the
reverse. Within the global block, a harness-specific entry falls back to a harness-neutral `default:` entry before
the built-in value:

```yaml
agents:
  default:                              # harness-neutral fallback inside the global layer
    implement-next: { model: claude-opus-5, effort: medium }
  cursor:                               # per-harness override — only what differs
    implement-next: { model: gpt-5.1, effort: high }
  # effort: auto drops the effort pin and lets the harness pick; omitting the effort key keeps the built-in
  # effort — auto and omitted are not equivalent.
```

Keys are wrapper short names (`build-economy`, not `docket-build-economy`). Write model and effort values unquoted
and space-free: each must be a single token.

**Model IDs are opaque passthrough values (ADR-0015).** A `model` value is passed to the harness verbatim, with no
alias layer and no vendor allowlist; the running harness interprets it (a Claude ID under Claude Code, a Cursor
model ID under Cursor). That passthrough is what lets docket drive non-Claude harnesses.

## Per-harness wrapper shapes

Each harness gets its own wrapper shape:

| harness | file | model | effort | skills list |
|---|---|---|---|---|
| claude | `.md` | `model:` | `effort:` | frontmatter list |
| cursor | `.md` | `model: <id>[effort=<e>]` | *(inside the model value)* | body preamble |
| codex | `.toml` | `model =` | `model_reasoning_effort =` | `developer_instructions` preamble |
| opencode | `.md` | `model:` (`openrouter/<vendor>/<id>` passes through whole) | `reasoningEffort:` (a provider model option) | body preamble |

Cursor's wrapper carries `name`, `description`, and `model`, leaving `readonly` and `is_background` at Cursor's
defaults. On Cursor and opencode an effort with no resolved model has nowhere to attach and is dropped.

## Repository dispatch blocks: agent_harnesses

`agent_harnesses` is a repository's opt-in for its parent-facing dispatch blocks — the managed `docket:dispatch`
blocks that route a requested docket workflow to its wrapper. Only a value declared in `.docket.yml` or
`.docket.local.yml` authorizes the install to write repository files; the value has three states:

- **absent** — the install touches no repository surface;
- **a non-empty list** — the install reconciles the dispatch surface of each listed harness: a managed block in
  `CLAUDE.md` (claude), a managed block in `AGENTS.md` (codex, opencode), and the
  `.cursor/rules/docket-dispatch.mdc` rule (cursor). When claude is listed with codex or opencode and no regular
  `CLAUDE.md` file exists, `CLAUDE.md` becomes a link to `AGENTS.md` instead;
- **an explicit empty list** (`agent_harnesses: []`) — the install retires every docket-owned repository surface the
  repository had.

`--repo-dir <path>` targets another repository. `agent_harnesses` decides which harnesses get dispatch blocks; it
never decides which pins a wrapper carries.

The install also retires the global dispatch blocks earlier docket versions wrote into personal instruction files
(`~/.claude/CLAUDE.md` and the other harnesses' globals). The removal is proof-gated: a block is removed only while
it still matches docket's exact ownership marker; a modified or foreign block is left untouched and reported.

## Launch posture

Agent-source frontmatter may declare `launch: root-coordinator`; absence means the closed default
`child`. The posture describes a role's required entry capability, not a model setting and not a
request to broaden every child's authority. Mark a role `root-coordinator` when its charter owns
multi-agent sequencing and therefore requires native collaboration controls at entry. The inventory
parser rejects unknown posture values, and correspondence tests derive the marked set from the
role's same-name skill contract rather than maintaining a filename allowlist.

The scope matrix is harness-neutral: root coordinators use a native root-entry path; feature roles
receive `Feature worktree: <absolute canonical feature-worktree root>` in the owner's payload; metadata
children use native named-agent dispatch. Codex realizes the first two through catalog-resolved
`agent.enter [--worktree <dir>]`: it resolves the typed installed role contract, launches
`codex app-server --stdio`. Root-coordinator entry starts its root thread at the caller's absolute
cwd, retaining caller's approval policy and sandbox, and passes an unchanged request file as root turn.
Feature-child entry validates `--worktree`, then starts its root thread at the verified canonical
feature-worktree root — both the process and thread cwd — and supplies it through `--worktree`; its
request bytes are unchanged. Metadata children use native named-agent dispatch. Other harnesses retain
native worktree mechanisms. No route falls back to `codex exec`, another harness, a shell relay,
or ordinary child launch.

Before launch, root entry compares the selected installed role and preloaded skill files against the
registration planner's output and asset catalog. A missing, edited, or stale
contract is refused with `role-contract-unavailable`; it is never silently repaired during entry.

The parent includes the run tracker's run context unchanged in the request file, alongside the
user's unchanged request and any resume/continuation identity. The coordinator uses it in its claim
transaction. After foreground completion, the parent asks that keyed run tracker for the verdict;
thread/turn ids and coordinator prose are diagnostic output, never claim proof.
If Codex requests interactive approval or user input, root entry reports an explicit unsupported
interaction error: this foreground transport has no approval/input channel and cannot approve or
answer on the caller's behalf.

## Invocation paths

A harness that runs a directly-invoked skill inline at the session model (Cursor does) defeats the pin; the
repository dispatch block routes the request to the pinned wrapper instead. Claude Code also forks natively: the
four headless-safe autonomous skills (`docket-status`, `docket-adr`, `docket-implement-next`, `docket-auto-groom`)
carry `context: fork` + `agent: docket-<name>` frontmatter in their `SKILL.md`, forking a directly-invoked skill
into the same pinned wrapper — inert in every other harness. **Fork-exclusion principle:** only skills that never
need the human mid-run are forked, since a forked subagent has no channel back to the human; the two interactive
skills stay inline, and `docket-finalize-change` stays unforked — its headless merge is gated by a permission
classifier (ADR-0043).

A forked skill-invoke (`/docket-status`) and an explicit agent dispatch (`@docket-status`, or a subagent dispatch
naming the wrapper) land on the *same* wrapper at the *same* resolved model/effort; they differ only in
observability (the dispatch is drillable in the TUI, the fork is not) and cost (the dispatch spends a turn). A
wrapper whose skills list preloads the very skill that forks into it does not recurse: preload is content injection
at startup, while the fork fires on invocation.

## Checking the install

The `install.check` operation reports whether this machine's installation is current and writes nothing. It is a
machine-only report: it covers this machine's installed targets (binary, skills, wrappers), never a repository's
dispatch blocks.
Re-run the `install` operation after editing the global agent pins; the install never runs on its own at session
start.
