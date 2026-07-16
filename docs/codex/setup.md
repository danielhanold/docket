# Codex setup — enabling docket's Codex harness

Codex is a first-class docket harness. `sync-agents.sh` generates two Codex artifacts:

- **`.codex/agents/docket-*.toml`** — the model/effort-pinned agent wrappers. These are
  **machine-local**: gitignored, regenerated per machine, never committed (they bake resolved
  model IDs — ADR-0020).
- **A `docket` dispatch block in `AGENTS.md`** — a marker-bounded block in the repo-root
  `AGENTS.md` that tells Codex to delegate a directly-invoked docket skill to its pinned
  `.toml` agent (Codex has no analog of Cursor's `.mdc` rule; it reads `AGENTS.md`). This block
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
list outright. Re-run `sync-agents.sh` (or `install.sh`) after editing any config layer.

**Why it works this way.** The `AGENTS.md` dispatch block is *committed*. If a global setting on
your machine generated that committed block, a collaborator (or CI) without the same global
config would fail `sync-agents.sh --check` — their clone would see a `docket` block that their
own `agent_harnesses` doesn't call for. Making per-repo targeting come from the repo's own
committed (or machine-local) config keeps the committed artifact deterministic across every
clone. Global `agent_harnesses` is therefore scoped to the user-level pass only.

> Note: when Codex is de-listed from an opted-in repo, `sync-agents.sh` **removes** the
> `AGENTS.md` dispatch block (and prints a one-time commit notice). Your own `AGENTS.md`
> content outside the docket markers is preserved untouched.

## Pinning models and effort

The `.toml` wrappers carry the model/effort resolved from the layered `agents:` config. Use the
model IDs Codex itself reports:

```sh
codex debug models | jq -r '.models[] | .slug'
```

Set them per agent under `agents:` (harness-first) in whichever config layer applies — see the
main README's agent-layer section for the full precedence rules.

## Verifying it works

After opting a repo in and running `sync-agents.sh`:

1. `.codex/agents/docket-*.toml` exist and carry the expected `model`/`effort`.
2. `AGENTS.md` contains the marker-bounded `docket` dispatch block.
3. In a Codex session opened in the repo, a directly-invoked docket skill is delegated to its
   pinned agent, and Codex runs it at the pinned model/effort.

`sync-agents.sh --check` validates the `AGENTS.md` block's presence and currency (it is exempt
from the tracked-file leg — the block is *meant* to be committed) and flags a stale or missing
block for CI.

## Restart after (re)generating

Codex registers its agents at process start. After `sync-agents.sh` writes new wrappers, restart
your Codex session before invoking a docket skill — an already-open session keeps the old
definitions.
