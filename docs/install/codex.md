# Codex: running docket under Codex

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

Set them per agent under `agents:` (harness-first) in whichever config layer applies — see
[Models](models-and-effort.md) for the full precedence rules. Upgrading from a docket that shipped
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
