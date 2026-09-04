# Delegation: running an agent on a different harness

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
adapter contract: `scripts/runners/opencode.md`; the hosting-harness recipe is in
[the opencode page](opencode.md).
