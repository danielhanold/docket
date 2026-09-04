# Running on your harness

This page gets docket installed on your machine and running under whichever agent tool you use. By
the end you will have installed docket once, learned how to keep it current, learned how to make
each unit of work run at its own model and effort instead of paying one session-wide tier, and set
up any of the four supported harnesses — a **harness** is the tool that runs the agent: Claude Code,
Cursor, Codex, or opencode. Product names appear freely here because harness setup is exactly what
this page is about; the rest of the guide keeps them out of the way.

## Install docket

docket installs once per machine and then works in every repo you use it from.

### What you need first

- **A harness.** docket's skills — a **skill** is a named, reusable instruction set an agent loads
  for one job — run inside a harness that has its own on-disk `skills/` and `agents/` directories
  for docket to write into. **Claude Code, Cursor, Codex, and opencode** are first-class; docket
  also writes into `.agents/`, `.kiro/`, and `.windsurf/` harness roots when they are present.
- **`git` and the GitHub CLI (`gh`).** Every docket operation is a git operation, and the
  implementer opens pull requests with `gh`.
- **A GitHub remote** for the pull-request flow. docket pushes branches and opens PRs against your
  `origin`.
- **The superpowers plugin — recommended, not required.** superpowers is docket's default execution
  engine (brainstorm, plan, build, review, finish). Installing it is your responsibility; docket
  neither bundles nor fetches it. If it is absent, each workflow step **degrades to running inline
  at the agent's own model, with a prominent warning** — so docket still works out of the box with
  zero config, just without superpowers' structured execution. See
  [Governing through configuration](governing-through-configuration.md) to rebind any step.

### 1. Install docket on your machine

Place the docket repo at `~/dev/docket` (the source of truth the symlinks point back to), then run:

```bash
bash ~/dev/docket/install.sh
```

That is the whole install. `install.sh` is a thin bootstrapper: it resolves this checkout and hands
the install to docket's Go engine (`docket development install`), which does the real work as **one
journaled, all-or-nothing transaction** and is idempotent — re-run it any time (after adding a
harness, after editing `~/.config/docket/config.yml`, and after every version update). A single run:

- **Builds a fresh binary and hands the install to that binary**, so the version that plans and
  writes your machine is the one you are installing — never the older binary that happened to be
  running. The recursion-guarded dispatch wrappers therefore land on the **first** run, not the
  second.
- **Links each present harness's global `skills/`** back to `~/dev/docket/skills/<name>` (symlinks,
  so editing a skill in the repo takes effect everywhere at once) and **reconciles that harness's
  global agent wrappers** — the model/effort-pinned subagent copies, resolved from your config
  layers over docket's shipped defaults. It also points `~/.config/docket/config.yml` at
  [`.docket.example.yml`](../../.docket.example.yml), docket's canonical reference for every key and
  its default.
- **Retires the old global parent-facing dispatch blocks** that earlier docket versions wrote into
  your personal `~/.claude/CLAUDE.md` and the other harnesses' global instruction files, while
  keeping the global skills and agent wrappers. Removal is **proof-gated** — the engine deletes a
  block only while it still matches docket's exact ownership marker, byte for byte. There is **no
  `--force`**: a block you edited, or one that no longer matches, is left untouched and the run
  reports it so you can remedy it and re-run.
- **Reconciles each repository's parent-facing dispatch surfaces** from that repository's *explicit*
  `agent_harnesses` opt-in (see step 2) — automatic and Go-owned, with no separate synchronization
  script to run.

Two flags scope a run: **`--repo-dir <path>`** targets a repository other than the one containing
your current directory, and a repeatable **`--harness <name>`** limits the run to the named
harness(es) instead of every harness present on your machine.

### 2. Set up your global config

The installer writes a minimal `~/.config/docket/config.yml` the first time it runs and
non-destructively maintains its managed values. docket's ordinary defaults already apply, so a
Claude-Code-only user can stop here.

The canonical reference for every key is [`.docket.example.yml`](../../.docket.example.yml): every
config key, active at its shipped default, with full documentation and a scope tag saying which
layers may set it. Copy the keys you want to change into the layer you want them in.

- **To see docket's built-in per-skill model and effort:** they all live in
  [`agents/harness-defaults.yml`](../../agents/harness-defaults.yml) — docket's shipped,
  harness-indexed default sidecar, not a file you edit. All four of the example's commented harness
  blocks — `agents.claude`, `agents.cursor`, `agents.codex`, and `agents.opencode` — mirror it in
  full, value for value.
- **To enable another harness (Cursor, Codex, opencode):** add it to `agent_harnesses` and re-run
  `install.sh`; the Go engine reconciles that harness's wrappers and dispatch surfaces for you.
  Leave the harness's `agents:` block commented, since it only restates the shipped defaults and
  uncommenting it would freeze today's values into your config forever. `agent_harnesses` is the
  **explicit opt-in** for a repository's parent-facing dispatch surfaces, and it has **three
  states**: *absent* leaves the shipped default (Claude only) in force and writes no other harness's
  repository surfaces; a *non-empty* list reconciles exactly the harnesses you name; and an
  *explicit empty* list (`agent_harnesses: []`) retires every docket-owned repository surface the
  repo previously had. An absent key touches nothing — only an explicit value reconciles or retires.

> **Stale project-level Claude wrappers shadow the guard.** docket installs agent wrappers
> **machine-globally** (under `~/.claude/agents/`), never inside a repository. If a repo still
> carries its own `.claude/agents/docket-*.md` copies — as docket versions before the recursion
> guard left behind — Claude Code loads *those* project-level wrappers in preference to the guarded
> global ones, which re-enables recursive self-dispatch. Delete those project-level copies; docket
> will not touch them, because it never owned them.

> **Start a fresh harness process after any install that changed a wrapper or a parent surface.**
> Harnesses register their agents and read their instruction files **at process start**, so a
> changed dispatch wrapper or a retired dispatch block only takes effect in a newly started process
> — **clearing a conversation is not enough**.

The change data — `docs/changes/`, `docs/adrs/`, `docs/results/` — lives in each consuming project,
not in the docket repo itself. To adopt docket in an *existing* repo, run `docket repository
migrate` from inside that repo — a separate step from this machine install (see
[Where the metadata lives](where-the-metadata-lives.md)).

## Keep docket current

**Every time you pull a new version — a `git pull` on `main` or a checked-out release tag — re-run
`install.sh`.** It is the catch-all: it applies whatever the new version needs on your machine, and
it is idempotent, so running it when nothing changed is a no-op.

```bash
cd ~/dev/docket
git fetch --tags && git pull        # or: git checkout v0.8.0
bash ~/dev/docket/install.sh        # always — not only when something looks broken
```

Pulling alone is **not** enough. Skills are symlinks, so those update the moment you pull — but the
rest of docket's on-disk footprint is generated or persisted, and only an install run refreshes it:

- **Agent wrappers are generated copies**, not symlinks — they bake in the resolved model and
  effort. A version that adds a subagent, renames one, or changes a pin lands only when the
  installer reconciles the wrappers.
- **New harness support**, and any harness you installed since last time, gets its `skills/`
  symlinks and `agents/` wrappers only on the next install run.
- **Managed global config** in `~/.config/docket/config.yml` is back-filled non-destructively by the
  same run.
- **Retired global dispatch blocks and reconciled repository surfaces** land on this run too — which
  is why the recursion-guarded wrappers you are pulling only take effect after it, in a freshly
  started harness process.

Re-running the install is **in addition to** anything the release notes call for, never a
substitute. A release may also carry a per-repo step — a `docket repository migrate` run, a
`.docket.yml` key to add, a remedy commit to land — listed in the notes for that version. Do the
machine-level `install.sh` first, then the per-repo steps.

## Tuning models and effort per task

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

**1. Edit a config layer.** Up to three layers override the built-in default, resolved per field
(precedence: repo-local > repo-committed > global > built-in):

- **Global** — the `agents:` block in `~/.config/docket/config.yml` (applies to every repo on your
  machine).
- **Repo-committed** — the `agents:` block in a repo's committed `.docket.yml` (applies to that repo
  for every clone).
- **Repo-local** — the `agents:` block in that repo's `.docket.local.yml` (this machine only; wins
  over the committed value for this clone).

The config **shape** — the `agents:` keys and how the model and effort are written — is documented
once in the `docket-convention` skill's *Agent layer* section; consult it there rather than copying
field examples, so the shape has a single source of truth. For the layer model overall, see
[Governing through configuration](governing-through-configuration.md).

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
wrapper *copies* (not symlinks), so after editing any layer, regenerate them:

```bash
docket development install  # regenerate the wrappers; or re-run install.sh, which drives the same engine
```

- A **global** edit rewrites user-level wrappers into every **present** harness root
  (`~/.<harness>/agents/`, e.g. `~/.claude/agents/`, `~/.cursor/agents/`, `~/.codex/agents/`).
- A **repo-committed or repo-local** edit rewrites that repo's per-repo wrappers for each harness in
  its (local-then-committed) `agent_harnesses:` list (default `[claude]`).

The install always writes **both** passes in one run, and project wins over global at generation
time, per the four-layer precedence above.

**Generated per-repo agent files are machine-local — gitignored, never committed.** Unlike a repo's
committed `.docket.yml`, `<repo>/.<harness>/agents/docket-*.md` (and, for Cursor,
`docket-dispatch.mdc`) are regenerated on every machine from that machine's own resolved config;
they carry no team intent of their own — the committed `agents:` block is the artifact that does. A
single marker-bounded `# docket` block in the repo's `.gitignore` covers every docket-owned path,
and is seeded by `docket repository migrate` (fresh migration) or `docket repository prepare` (fresh
orphan-branch bootstrap), then self-healed by the install, which prints a loud one-time notice to
**commit it once**.

**3. Guard drift in CI.** `docket install check` is a gate:

- The `.gitignore` `# docket` block is present and current, **and** no per-repo generated file is
  tracked by git — both are **CI-meaningful** (`rc != 0` fails the build; the second leg also
  catches a repo whose migration commit never happened).
- A committed `.docket.yml` using the legacy bare-agent-key `agents:` shape (agent keys sitting
  directly under `agents:` instead of nested under `agents: default:`) also fails — **CI-meaningful**
  — naming the offending keys and the reshape to `agents.default.<agent>` in its message.
- Generated content drifting from the resolved config is **advisory only** (`rc` unaffected) — every
  clone regenerates its own copy at build time, so a stale local file is a nudge to re-run the
  install, not a CI failure.

**The clone-identical guarantee is retired.** Before this design, committing the generated per-repo
files meant an autonomous change built on the exact same model on every clone, by construction.
Generation is now all-local, so that guarantee is gone — a deliberate trade: never having to
reconcile a machine-generated file in a PR diff, at the cost of no CI-enforced pinning of the
generated copies. Team defaults for a repo still live in its committed `.docket.yml` `agents:`
block, by convention.

### How the pin survives a direct invocation

Both Cursor and Claude Code run a *directly-invoked* skill — a human typing `/docket-status`, or the
model auto-invoking it — inline at the session model, which silently defeats the wrapper's
model/effort pin. They fix it with **two mechanisms**: Cursor uses a generated `docket-dispatch.mdc`
rule that forces a real dispatch; **Claude Code uses native `context: fork` + `agent: docket-<name>`
frontmatter** committed in each forked skill's `SKILL.md`, which forks the invocation into the same
pinned wrapper. That frontmatter is inert in every other harness (unknown keys are ignored), so one
shared `SKILL.md` serves all of them, and it degrades to today's inline behavior on a Claude Code too
old to know the field. **Fork-exclusion principle:** only skills that never need the human mid-run
are forked — a forked subagent has no channel to the human (Claude Code withholds `AskUserQuestion`,
`EnterPlanMode`, and similar from subagents). So the four headless-safe autonomous skills —
`docket-status`, `docket-adr`, `docket-implement-next`, `docket-auto-groom` — carry the frontmatter;
the two interactive brainstorm skills (`docket-new-change`, `docket-groom-next`) and
`docket-finalize-change` (which keeps real prompts — the multi-candidate batch confirmation and
repair sign-off — so a headless drive is authorized by
[naming ids](landing-changes.md) instead) do not.

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

## Delegating an agent to another harness

docket agents normally run on the harness hosting your session. **Runner delegation** hands an
agent's *whole run* to a child harness with its own subscription, models, and skills — activated per
agent by an explicit `runner:` key, never inferred from model IDs. Three pairs ship today, all with
parent `claude` (Claude Code): children `codex` (OpenAI Codex CLI), `cursor` (Cursor CLI), and
`opencode`. The motivating use is cost asymmetry — through OpenRouter, opencode reaches
DeepSeek-tier models at a fraction of a frontier-model task, and because docket's build and review
roles are already separate agents (ADR-0063), you can send build work to cheap models while review
stays on your Claude subscription.

```yaml
# .docket.yml (or the global ~/.config/docket/config.yml — runner is a machine preference)
agents:
  claude:                       # the PARENT harness: when Claude Code hosts the session…
    status: { model: gpt-5.1-codex, effort: medium, runner: codex }   # …run docket-status on Codex
    adr:    { model: gpt-5.1, effort: high, runner: cursor }          # …or run docket-adr on Cursor
    # Delegate the four build workers to cheap OpenRouter models; leave review native.
    build-economy:  { runner: opencode, model: openrouter/deepseek/deepseek-v4-flash-0731, effort: medium }
    build-standard: { runner: opencode, model: openrouter/deepseek/deepseek-v4-flash-0731, effort: high }
    build-premium:  { runner: opencode, model: openrouter/moonshotai/kimi-k3,              effort: medium }
    build-max:      { runner: opencode, model: openrouter/moonshotai/kimi-k3,              effort: high }
    # review-lean / review-standard / review-deep: no runner: → native Claude Code
runners:
  codex:
    sandbox: workspace-write    # workspace-write (default) | danger-full-access
    network: true               # default true — git push and gh need it
  opencode:
    permissions: auto-approve   # ask (default) REFUSES to delegate — see below
```

The OpenRouter IDs above are illustrative, not validated: docket keeps no vendor allowlist
(ADR-0015), so **no test in this repo can catch a wrong or unentitled ID**. Confirm them against
`opencode models openrouter` under your own credentials before relying on them — catalog presence is
not entitlement.

A delegated run is anchored at the repo's **main worktree** by default (ADR-0034, cwd-independent by
design) — correct for the metadata-scoped `status`/`adr` agents. A **build profile** worker — one of
four worker tiers (economy, standard, premium, max) a plan task is routed to by risk — must instead
stay in the feature worktree on its branch, so a delegated build worker runs in the tree named by
`--worktree`. That requirement is keyed on a **declared** fact, not a name shape: every built-in
agent source carries `worktree-scope: feature` or `worktree-scope: metadata`, the generated shims
bake the `--worktree` slot for the feature-scoped ones, and the dispatch facade refuses a
feature-scoped delegation that names no worktree (or that names the main worktree).

How it works: the install generates that agent's wrapper with a **shim body** — a
**launch-then-observe** instruction over a single host-native dispatch seam (change 0371). The launch
call resolves the `runners.codex` knobs, starts `codex exec` (sandboxed, final-message relay via
`--output-last-message`) **detached**, and returns a dispatch key immediately; the wrapper then makes
bounded, short observe calls against that key until the run reports a terminal result. Two
invocations, still exactly one dispatch seam — no delegated run is bounded by one foreground call's
ceiling. Every invocation path (skill fork, `@docket-status`, composition from another skill)
inherits the delegation unchanged.

Rules and limits:

- **Only autonomous wrappers are delegatable** (the seventeen generated agents). Interactive skills
  stay inline — an exec primitive has no human channel.
- A delegated *orchestrator*'s own sub-dispatches run child-natively (for Codex: `spawn_agent`, via
  superpowers' Codex support). Per-agent model pins do **not** carry into those child-side dispatches
  (accepted limitation).
- `runner:` under a non-`claude` harness key is reserved and warned-and-ignored; an unregistered
  runner name fails generation loudly.
- **Generation is all-or-nothing.** Any bad `runner:` entry is detected before the first wrapper is
  written, and the run refuses rather than regenerating some wrappers and not others: one bad entry
  refreshes *zero* wrappers, and previously generated ones survive untouched. `docket install check`
  reports the same failure without writing anything.
- Delegation is never a policy bypass: do not delegate `docket-finalize-change` to sidestep
  merge-approval gates (ADR-0043).
- **The wrapper's own `model:`/`effort:` are not the child's.** A generated shim is two agents in one
  file: a relay that Claude Code runs, and the child run it dispatches. The frontmatter pins the
  relay and must therefore name a model *Claude Code* can resolve — it defaults to `inherit` and is
  retuned per runner with `runners.<name>.shim_model` / `shim_effort`. The child's identity travels
  in the baked `--model` / `--effort` arguments, from the `model:` you set under `agents:`.
- **A delegated agent must carry an explicit `model:` in your config.** docket never forwards its own
  shipped default to another harness — that ID means nothing to the child — so a model-less `runner:`
  is a loud generation-time error rather than a silent run on the child's own default, which on a
  pay-per-token backend surfaces on the bill instead of in the run (ADR-0067).
- **`effort:` is optional — and omitting it is not the same as opting out.** It follows the same
  never-forward-a-shipped-default rule, but with no matching error. Omitting the key means "no
  opinion" and still resolves from lower **user** config layers, whose value *is* forwarded; `effort:
  auto` is the explicit no-pin that suppresses the flag. With no flag baked, the child uses its own
  default for the chosen model.
- **Delegate leaves, not orchestrators.** A delegated run's own sub-dispatches run child-natively, so
  delegating `docket-implement-next` drags its review dispatch into the child too. Delegating the
  four `build-*` profile workers rather than the `docket-build` controller is the same rule:
  delegating the controller would move the routing decision into the child as well.

**Prerequisites (codex):** Codex CLI installed and authenticated (`codex login`); superpowers
installed in Codex; docket skills linked (automatic on install); and `[features] multi_agent = true`
in `~/.codex/config.toml` if you delegate an orchestrator (SDD fan-out) rather than a leaf agent.
Full adapter contract: `scripts/runners/codex.md`.

**Prerequisites (cursor):** `cursor-agent` installed and authenticated, and docket skills linked into
`~/.cursor/skills` (automatic on install). `cursor` delegates to `cursor-agent -p`. Note that
`cursor-agent` is unreliable and lags the Cursor IDE in features, so the adapter's posture is a loud
abort-and-report on any failure — it never falls back to running the agent inline. Cursor has no
effort flag: `effort:` is encoded inside the model value as `--model <id>[effort=<e>]`, and with no
model resolved it is dropped with a WARN. Full adapter contract: `scripts/runners/cursor.md`.
Validating the Cursor harness end to end (the CLI probe and the human IDE checklist):
[the Cursor validation runbook](../reference/harness/validation.md).

**Prerequisites (opencode):** `opencode` installed (verified against 1.18.11) with a provider
authenticated (`opencode auth login`), and docket skills linked into `~/.agents/skills` (automatic on
install). `opencode` delegates to `opencode run`. **You must set `runners.opencode.permissions:
auto-approve`** — opencode prompts for approval before editing a file or running a command, a
delegated run has nothing to answer with, and the adapter therefore refuses up front rather than
hanging. That grant is deliberately a visible line in config; pair it with opencode's own deny rules
(whose containing behavior docket has documented from its help text but not tested — verify before
relying on it). `effort:` maps to `--variant` and passes through unmapped, including docket's `max`
(unlike codex, where `max` becomes `xhigh`); the omit-vs-`auto` rule above applies unchanged. Full
adapter contract: `scripts/runners/opencode.md`; the hosting-harness recipe is in the opencode
section below.

## Claude Code

Claude Code is docket's reference harness, and everything in *Install* and *Tuning* above applies to
it directly. Two Claude-Code-specific details are worth calling out.

**Forking preserves the pin but hides the run.** As the *How the pin survives a direct invocation*
table shows, a directly-invoked forked skill returns as `completed (forked execution)`, with no box
to drill into in the TUI. A forked run is not lost, only unobservable there: Claude Code still writes
its full transcript to
`~/.claude/projects/<project-slug>/<session-id>/subagents/agent-<id>.jsonl`. Treat that path as an
**observed internal, not an interface** — it was accurate on Claude Code 2.1.207, it may move, and
docket depends on it for nothing. When you want to watch a long run live, dispatch it with
`@docket-<name>` instead, which routes through a real, drillable dispatch.

**Restart after changing an agent or a skill.** Skills and agents are **registered at process
start**. After an install, or after you edit a skill's frontmatter, an already-open session keeps
running the *old* definitions — so a freshly-added fork appears to do nothing, and a healthy pin
looks broken. Restart the harness process (a new session — clearing the context is not enough) and
re-invoke.

## Cursor

Cursor is a first-class harness both as a host and as a delegation target. Running docket under
Cursor's Auto-run in Sandbox needs a small, stable permission configuration, because docket must run
**outside** Cursor's sandbox.

> **Provenance.** Every classifier claim below was observed in **Cursor 3.11.19** on
> **2026-07-14** under **Allowlist (with Sandbox)**. Cursor's auto-run classifier is not a documented
> contract, so treat these as empirical claims about that version, and re-verify if your Cursor
> differs.

### The three gates, and why they are independent

Cursor decides whether an agent command runs, and how, through three independent gates:

1. **Command approval** (`permissions.json` → `terminalAllowlist`) — whether a command auto-runs at
   all, and whether it runs **outside** the sandbox.
2. **Filesystem access** (`sandbox.json` → `additionalReadonlyPaths`) — what a **sandboxed** command
   may read.
3. **Network** — whether a **sandboxed** command may reach the network.

They do not substitute for one another. Granting filesystem or network access to a sandboxed command
does **not** move it outside the sandbox; only a `terminalAllowlist` match does. Edits to
`~/.cursor/permissions.json` are picked up within a second or two without restarting Cursor (file
watcher).

**Run Modes and the allowlist lock.** When `~/.cursor/permissions.json` defines a non-empty
`terminalAllowlist` (or `mcpAllowlist`), Cursor constrains the selectable Run Modes to **Allowlist**
and **Allowlist (with Sandbox)** only. **Run Everything** is disabled (a banner says so), and
**Auto-review (with Sandbox)** — though still shown — becomes non-selectable. Do **not** try to
escape this with an `approvalMode` key — writing `approvalMode: "unrestricted"` alongside allowlists
emptied the Run Mode dropdown entirely and had to be removed. The recommended operator mode is
**Allowlist (with Sandbox)**.

### Why docket must run outside the sandbox

docket's runtime needs the **network**: `preflight` fetches and rebases, and skills push. A
sandboxed docket command fails — typically the `git fetch` to origin dies and `preflight` exits
non-zero — **even when** `sandbox.json` grants a read path and network access, because the command
is still sandboxed. The fix is not more sandbox permissions; it is a `terminalAllowlist` entry that
runs docket **outside** the sandbox.

### The fragments

docket now ships as a native binary on your `PATH`, invoked as `docket <operation>`. That single,
stable command name is all Cursor needs to allowlist — no wrapper path, no environment variable, no
per-spelling entries. Copy these into your Cursor config (ready-made copies ship as
[`permissions.example.json`](../reference/harness/permissions.example.json) and
[`sandbox.example.json`](../reference/harness/sandbox.example.json) in the harness reference; replace
`$USER` with your username and adjust the path to your actual docket clone).

**`~/.cursor/permissions.json`** — allowlist the `docket` binary. Cursor prefix-matches the literal
command string, so the one entry covers every `docket <operation>` invocation.

```json
{
  "terminalAllowlist": [
    "docket"
  ]
}
```

**`~/.cursor/sandbox.json`** — grant a read path to the docket clone (complementary; it does **not**
move docket out of the sandbox — see above).

```json
{
  "additionalReadonlyPaths": [
    "/Users/$USER/dev/docket"
  ]
}
```

### What allowlisting the binary authorizes

Allowlisting `docket` authorizes, **unprompted**, every operation the binary can run — including
destructive and external-writing ones:

- `docket-status`'s guarded sweep — archives merged changes, publishes terminal records onto the
  **integration branch** (the branch code lands on, usually `main`), and deletes merged feature
  branches and worktrees.
- terminal-publish's direct push to the integration branch.
- github-mirror's external writes to GitHub Issues and Projects.
- cleanup-feature-branch's provenance-guarded branch and worktree deletion.

These are shared-history and external writes, and they are the deal you accept for one line of
config. Each is guarded or provenance-checked, which is a mitigation — not a reason to leave the
statement out.

### Why the broader workarounds are not acceptable

It is tempting to allowlist something broader — `eval`, a blanket `bash`, or a bootstrap-command
prefix. Each erases the trust boundary the binary draws and returns the permission surface to
unbounded. The `docket` binary deliberately has **no** `run`/`exec`/`shell`/`eval` operation for
exactly this reason; do not reintroduce one at the permission layer.

### Scope — what this fragment does and does not cover

docket's binary stabilizes docket's own metadata and lifecycle operations. Your repo's **build-time**
commands — feature-branch git, the test suite, `gh` — are that repo's own permission surface. They
are not covered by docket's fragment and not silently granted by it; allowlist them separately
according to your own trust policy. (For example, an agent compound that runs `docket board-refresh`
alongside `git status` needs `git status` allowlisted on its own — the docket entry does not cover
it.)

### Troubleshooting

**A sandbox grant did not make docket work.** You added a read path and network access in
`sandbox.json`, but a docket command still fails (often `git fetch` to origin). Sandbox permissions
govern **sandboxed** commands; they do not move a command outside the sandbox. Only a
`terminalAllowlist` match runs docket unsandboxed. Add the `docket` entry to `permissions.json`.
(Observed: Cursor 3.11.19 · 2026-07-14.)

**One unmatched command in a compound sandboxes the whole program.** A compound command is demoted to
the sandbox **as a whole** if any leaf is unmatched — even a leaf that can never execute (`if false;
then eval true; fi; docket env`). Keep docket calls as standalone commands, and allowlist any other
leaf (e.g. `git status`) on its own. (Observed: Cursor 3.11.19 · 2026-07-14.)

**Invalid JSON silently disables the whole allowlist.** A malformed `permissions.json` (e.g. a
truncated trailing `}`) is silently ignored — the allowlist stops taking effect and every docket call
is demoted to the sandbox. Restoring valid JSON restores the allowlist within a second or two (file
watcher; no restart needed). Validate the file after editing. (Observed: Cursor 3.11.19 ·
2026-07-14.)

The full end-to-end Cursor validation — the CLI probe and the human IDE checklist — lives in
[the Cursor validation runbook](../reference/harness/validation.md).

## Codex

Codex is a first-class docket harness. An install generates two Codex artifacts:

- **`.codex/agents/docket-*.toml`** — the agent wrappers. These are **machine-local**: gitignored,
  regenerated per machine, never committed (they bake resolved model IDs — ADR-0020).
  `agents/harness-defaults.yml` ships a complete seventeen-agent `codex:` block, so all seventeen
  `.toml` wrappers are generated **pinned** with no configuration at all. Any field is overridable
  per agent from any config layer — your value wins over the shipped one. The pins are Codex-native:
  a Claude model ID means nothing to Codex, so an ID is never lent across harnesses.
- **A `docket` dispatch block in `AGENTS.md`** — a marker-bounded block in the repo-root `AGENTS.md`
  that tells Codex to delegate a directly-invoked docket skill to its matching `.toml` agent (Codex
  reads `AGENTS.md`; it has no analog of Cursor's `.mdc` rule). This block is **committed and
  machine-neutral**: it carries only agent names and delegation prose, never a model ID or effort
  value, so it is clone-identical across machines (ADR-0036).

### The opt-in you need

An install writes in two passes, and Codex lives in **both**:

| Pass | Writes | Governed by |
|---|---|---|
| **User-level** | `~/.codex/agents/docket-*.toml` | **global** `agent_harnesses:` in `~/.config/docket/config.yml` |
| **Per-repo** | `<repo>/.codex/agents/docket-*.toml` **and** the `AGENTS.md` dispatch block | the **repo's own** `agent_harnesses:` in `.docket.yml` or `.docket.local.yml` |

**The gotcha: a global `agent_harnesses` does NOT generate per-repo Codex artifacts.** Setting
`agent_harnesses: [claude, codex]` in `~/.config/docket/config.yml` writes `~/.codex/agents/…` but
produces **nothing** inside a repo — no `.codex/agents/*.toml`, no `AGENTS.md` block. To get the
per-repo artifacts, the **repo** must opt in:

```yaml
# in <repo>/.docket.yml  — commits the choice for the whole team
agent_harnesses: [claude, codex]
```

```yaml
# or in <repo>/.docket.local.yml  — this machine only, gitignored, never leaves your clone
agent_harnesses: [claude, codex]
```

Either file opts the repo in; the first of local-then-committed that declares the key wins the list
outright. Re-run `install.sh` after editing any config layer — it reconciles Codex's wrappers and
the committed `AGENTS.md` dispatch block for you in one journaled transaction (change 0351). As a
repository opt-in, `agent_harnesses` has three states: *absent* keeps the shipped default (Claude
only) and writes no Codex surfaces, a *non-empty* list reconciles exactly the harnesses named, and an
*explicit empty* list (`agent_harnesses: []`) retires every docket-owned repository surface —
including this `AGENTS.md` block — that the repo previously had. That same install run also retires
the old **global** parent-facing dispatch blocks earlier docket versions wrote into personal
instruction files, proof-gated against docket's exact ownership marker with no `--force`.

**Why it works this way.** The `AGENTS.md` dispatch block is *committed*. If a global setting on your
machine generated that committed block, a collaborator (or CI) without the same global config would
see a `docket` block their own `agent_harnesses` doesn't call for. Making per-repo targeting come
from the repo's own committed (or machine-local) config keeps the committed artifact deterministic
across every clone.

> Because the block is shared with opencode, it is removed only when the **last**
> `AGENTS.md`-dispatch harness is de-listed. De-listing Codex from a repo that still targets opencode
> (or the reverse) leaves the block in place, correctly; de-listing the last one removes it and
> prints a one-time commit notice. Your own `AGENTS.md` content outside the docket markers is
> preserved untouched.

### Pinning models and effort

The `.toml` wrappers carry the model/effort resolved from the layered `agents:` config over docket's
shipped `codex:` block. That config **overrides** the shipped pin rather than being the only source
of one — set it for any agent whose tier you want retuned. Use the model IDs Codex itself reports:

```sh
codex debug models | jq -r '.models[] | .slug'
```

Set them per agent under `agents:` (harness-first) in whichever config layer applies — see *Tuning
models and effort per task* above for the full precedence rules. Upgrading from a docket that shipped
no `codex:` block, resolution is field-by-field, so a codex agent for which you pinned only `model`
now keeps your model *and* inherits docket's shipped `effort` — if your model does not accept
docket's shipped reasoning-effort token, pin `effort` explicitly alongside it.

### Two invocation paths — one contract

Docket supports exactly two ways to start its work under Codex, and both are first-class — neither is
a workaround, and neither requires flipping a workflow's `skills:` binding to `auto`:

1. **Prose, routed by the dispatch block.** A plain request ("refresh the docket board") is routed by
   the repo's managed `AGENTS.md` dispatch block to the registered same-name `docket-*` agent.
2. **Direct invocation.** `@docket-status` (or any `@docket-…` agent) starts that same registered
   wrapper explicitly.

Either way, the wrapper you land in may need to dispatch further docket agents — planning, build,
review, grooming's critic, finalize's resolver and repair. Every generated Codex wrapper carries the
rule for that: **nested dispatch uses Codex's direct named-agent dispatch from the active top-level
tool surface.** A tool inventory read from *inside* another tool (a nested orchestration namespace)
intentionally omits Codex's top-level collaboration controls, so an agent must never conclude from
such an inventory that dispatch is unavailable — only a failed direct attempt or an explicit policy
denial establishes that. The harness-neutral statement of this rule lives in the `docket-convention`
skill's *Dispatch-capability resolution* section.

The proven nested-launch mechanics (codex-cli 0.151.0, `multi_agent = true`), the exact `spawn_agent`
/ `wait_agent` calls, the app-server entry path, and the rejected launch candidates are recorded in
the live runbook and its fixtures — see [the Codex live-validation
runbook](../reference/harness/validation-runbook.md), which drives skills loading, sandbox execution,
agent listing, dispatch and pin honoring, and metadata writes landing on `origin/docket`, end to end
in a fixture repo.

### Restart after (re)generating

Codex registers agent definitions **once, at process start**. After any install or sync that changed
a wrapper or the dispatch block, start a **fresh Codex application/CLI process** before relying on the
new definitions. Opening another conversation inside an already-running process is **not sufficient**
— that process is still holding the definitions it loaded at start.

## opencode

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
non-empty / explicit empty) and the shared-block removal rule are exactly as described for Codex
above; because the block is shared, it is removed only when the **last** `AGENTS.md`-dispatch harness
is de-listed, and your own `AGENTS.md` content outside the docket markers is preserved untouched.

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
live under *Delegating an agent to another harness* above; this is a **different mechanism** from the
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
