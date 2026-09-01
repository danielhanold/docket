# Codex setup — enabling docket's Codex harness

Codex is a first-class docket harness. `sync-agents.sh` generates two Codex artifacts:

- **`.codex/agents/docket-*.toml`** — the agent wrappers. These are **machine-local**: gitignored,
  regenerated per machine, never committed (they bake resolved model IDs — ADR-0020).
  `agents/harness-defaults.yml` ships a complete seventeen-agent `codex:` block, so all seventeen `.toml`
  wrappers are generated **pinned** with no configuration at all. Any field is overridable per agent
  from any config layer — your value wins over the shipped one. The pins are Codex-native: a Claude
  model ID means nothing to Codex, so an ID is never lent across harnesses.
- **A `docket` dispatch block in `AGENTS.md`** — a marker-bounded block in the repo-root
  `AGENTS.md` that tells Codex to delegate a directly-invoked docket skill to its matching
  `.toml` agent — pinned or not, since the agent also carries the skill's dispatch contract and
  preload (Codex has no analog of Cursor's `.mdc` rule; it reads `AGENTS.md`). This block
  is **committed and machine-neutral**: it carries only agent names and delegation prose, never
  a model ID or effort value, so it is clone-identical across machines (ADR-0036).

## Two scopes — and the opt-in you need

`sync-agents.sh` writes in two passes, and Codex lives in **both**:

| Pass | Writes | Governed by |
|---|---|---|
| **User-level** | `~/.codex/agents/docket-*.toml` | **global** `agent_harnesses:` in `~/.config/docket/config.yml` |
| **Per-repo** | `<repo>/.codex/agents/docket-*.toml` **and** the `AGENTS.md` dispatch block | the **repo's own** `agent_harnesses:` in `.docket.yml` or `.docket.local.yml` |

**The gotcha: a global `agent_harnesses` does NOT generate per-repo Codex artifacts.** Setting
`agent_harnesses: [claude, codex]` in `~/.config/docket/config.yml` writes `~/.codex/agents/…`
but produces **nothing** inside a repo — no `.codex/agents/*.toml`, no `AGENTS.md` block — and
`sync-agents.sh` prints no explanation. To get the per-repo artifacts, the **repo** must opt in:

```yaml
# in <repo>/.docket.yml  — commits the choice for the whole team
agent_harnesses: [claude, codex]
```

```yaml
# or in <repo>/.docket.local.yml  — this machine only, gitignored, never leaves your clone
agent_harnesses: [claude, codex]
```

Either file opts the repo in; the first of local-then-committed that declares the key wins the
list outright. Re-run `install.sh` after editing any config layer — it delegates to docket's Go
engine (`docket development install`), which reconciles Codex's wrappers and the committed
`AGENTS.md` dispatch block for you in one journaled transaction (change 0351). As a repository
opt-in, `agent_harnesses` has three states: *absent* keeps the shipped default (Claude only) and
writes no Codex surfaces, a *non-empty* list reconciles exactly the harnesses named, and an
*explicit empty* list (`agent_harnesses: []`) retires every docket-owned repository surface —
including this `AGENTS.md` block — that the repo previously had. That same install run also retires
the old **global** parent-facing dispatch blocks earlier docket versions wrote into personal
instruction files, proof-gated against docket's exact ownership marker with no `--force`. Restart
Codex after any run that changed a wrapper or the dispatch block: it registers agents and reads
`AGENTS.md` at process start, so clearing a conversation is not enough.

**Why it works this way.** The `AGENTS.md` dispatch block is *committed*. If a global setting on
your machine generated that committed block, a collaborator (or CI) without the same global
config would fail `sync-agents.sh --check` — their clone would see a `docket` block that their
own `agent_harnesses` doesn't call for. Making per-repo targeting come from the repo's own
committed (or machine-local) config keeps the committed artifact deterministic across every
clone. Global `agent_harnesses` is therefore scoped to the user-level pass only.

> Note: because the block is shared with opencode, it is removed only when the **last**
> `AGENTS.md`-dispatch harness is de-listed. De-listing Codex from a repo that still targets
> opencode (or the reverse) leaves the block in place, correctly; de-listing the last one removes
> it and prints a one-time commit notice. Your own `AGENTS.md` content outside the docket markers
> is preserved untouched.

## Pinning models and effort

The `.toml` wrappers carry the model/effort resolved from the layered `agents:` config over docket's
shipped `codex:` block. That config **overrides** the shipped pin rather than being the only source
of one — set it for any agent whose tier you want retuned. Use the model IDs Codex itself reports:

```sh
codex debug models | jq -r '.models[] | .slug'
```

Set them per agent under `agents:` (harness-first) in whichever config layer applies — see the
main README's agent-layer section for the full precedence rules.

**Upgrading from a docket that shipped no `codex:` block:** resolution is field-by-field, so a codex
agent for which you pinned only `model` now keeps your model *and* inherits docket's shipped
`effort` — previously it got no effort line at all. If your model does not accept docket's shipped
reasoning-effort token, pin `effort` explicitly alongside it.

## Verifying it works

After opting a repo in and running `sync-agents.sh`:

1. `.codex/agents/docket-*.toml` exist and each carries the `model` / `model_reasoning_effort`
   docket ships for that agent — compare against the `codex:` block in
   `agents/harness-defaults.yml`, which is the single source of truth. A field you set in a config
   layer wins over the shipped value. Since docket's `codex:` block covers every agent, both lines
   are always present unless you deliberately blank one with a sentinel — `model: inherit` or
   `effort: auto`, which the Codex emitter drops rather than writing — in which case Codex applies
   its own default for the dropped field.
2. `AGENTS.md` contains the marker-bounded `docket` dispatch block.
3. In a Codex session opened in the repo, a directly-invoked docket skill is delegated to its
   pinned agent, and Codex runs it at the pinned model/effort.

`sync-agents.sh --check` validates the `AGENTS.md` block's presence and currency (it is exempt
from the tracked-file leg — the block is *meant* to be committed) and flags a stale or missing
block for CI.

Those three checks are the static ones. To validate the whole loop live — skills loading, docket's
scripts running under Codex's sandbox, agents listing, dispatch and the model pin actually being
honored, and metadata writes landing on `origin/docket` — work through the
[Codex live-validation runbook](validation-runbook.md), which drives each of these end to end in a
fixture repo and records the observed outcome.

## Two invocation paths — one contract

Docket supports two parent-facing request shapes under Codex. The managed dispatch contract selects
the launch mechanism from the role's inventory posture; neither shape requires flipping a workflow's
`skills:` binding to `auto`:

1. **Prose, routed by the dispatch block.** A plain request ("refresh the docket board") is routed
   by the repo's managed `AGENTS.md` dispatch block. Ordinary roles use registered-agent dispatch;
   a role whose source declares `launch: root-coordinator` is entered with `docket agent enter`,
   preserving the caller's request and execution context.
2. **Direct invocation.** `@docket-status` (or another ordinary-child role) starts that registered
   wrapper explicitly. A `root-coordinator` marker is a hard boundary: invoke its workflow through
   the managed prose route or call `docket agent enter` directly, never `@` child launch.

Either way, the wrapper you land in may need to dispatch further docket agents — planning, build,
review, grooming's critic, finalize's resolver and repair. Every generated Codex wrapper carries
the rule for that: **nested dispatch uses Codex's direct named-agent dispatch from the active
top-level tool surface.** A tool inventory read from *inside* another tool (a nested orchestration
namespace) intentionally omits Codex's top-level collaboration controls, so an agent must never
conclude from such an inventory that dispatch is unavailable — only a failed direct attempt or an
explicit policy denial establishes that. The harness-neutral statement of this rule lives in the
docket-convention skill's *Dispatch-capability resolution* section.

### The proven nested-launch mechanics

Codex's nested launch mechanics differ by where the registered role starts. A host integration is
not required to expose collaboration controls to every spawned child, so Docket records the
required launch posture in the agent inventory instead of inferring capability from a wrapper file
or Codex version.

- **Ordinary child roles:** the machine-neutral dispatch instruction docket
  already renders — start the registered agent by name — is sufficient. In every passing run the
  session realized it as the native collaboration tool call, verbatim:

  ```
  spawn_agent {"agent_type":"probe-coordinator","fork_turns":"all","message":"…","task_name":"…"}
  wait_agent  {"timeout_ms":3600000}
  ```

  `agent_type` is the registered agent's `name` from `~/.codex/agents/<name>.toml`. The same call
  can work from a root thread and from a child when that host grants collaboration controls. The
  nested-launch fixture records that behavior for codex-cli 0.151.0; it is evidence about that
  host/version combination, not Docket's authorization contract.

- **Coordinator roles:** `docket agent enter` uses the app-server v2 protocol —
  `initialize` → `thread/start` (seeding the thread with the registered agent's
  `developerInstructions`; `ThreadStartParams` carries no agent-name field, so the seeding is the
  client-side mechanism) → `turn/start`. The resulting role is a root thread, so its required
  named-agent dispatch remains available without granting delegation to every child role.

The spawned thread runs **AS the registered definition**: its `developer_instructions` arrive
verbatim as the thread's developer message (so the wrapper's skill preload and recursion guard are
in force), and the definition's own `model` / `model_reasoning_effort` pins apply to the spawned
thread — even under `fork_turns:"all"`. Only what the fixture actually passed is documented here;
`codex exec --agent`, `codex exec run-agent`, `codex remote-control pair`, and the
`codex app-server proxy` control socket were **attempted and rejected** on this build (see
`decision.md` §"Rejected candidates") and are **not** supported launch paths.

The command is closed and foreground-only:

```
docket agent enter \
  --role docket-implement-next \
  --request request.txt \
  --cwd /absolute/repository/worktree \
  --approval-policy never \
  --sandbox workspace-write
```

It accepts only inventory roles marked `root-coordinator`, maps the same typed role contract used
by TOML registration (instructions, skill preload, model, and effort), waits for the root turn's
terminal event rather than a child's event, and prints the root role's final message.

## Restart after (re)generating

Codex registers agent definitions **once, at process start**. After any install or sync that
changed a wrapper or the dispatch block, start a **fresh Codex application/CLI process** before
relying on the new definitions. Opening another conversation inside an already-running process is
**not sufficient** — that process is still holding the definitions it loaded at start.
