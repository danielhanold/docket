# Codex: running docket under Codex

Codex is unsupported. An install writes three kinds of Codex artifact:

- **User-level skills and agent wrappers.** docket's skills are linked under the harness-neutral
  `~/.agents/skills/` root, which Codex reads, and its 17 agent wrappers are written to
  `~/.codex/agents/docket-*.toml`. Wrappers are user-level only; no repository carries its own
  copies. `agents/harness-defaults.yml` mirrors the built-in table, which has a complete `codex:`
  block, so every wrapper is generated **pinned** with no configuration at all. The pins are
  Codex-native: a Claude model ID means nothing to Codex, so an ID is never lent across harnesses.
- **A `docket` dispatch block in `AGENTS.md`, on opt-in only.** A marker-bounded block in the
  repo-root `AGENTS.md` tells Codex to hand a directly-invoked docket workflow to its matching
  `.toml` agent (Codex reads `AGENTS.md`; it has no analog of Cursor's `.mdc` rule). The block is
  **committed and machine-neutral**: it carries only agent names and routing instructions, never a
  model ID or effort value, so it is identical in every clone (ADR-0036).
- **A `dckt:private-instructions` pointer block in `~/.codex/AGENTS.md`.** This user-level,
  content-free block asks Codex to run `dckt instructions` once at the start of a session and to
  follow whatever it prints as the repository's own `AGENTS.md`. Outside a private repository the
  command prints nothing, so the block is inert. It needs no opt-in, and uninstall removes it only
  while it is unchanged.

### The opt-in you need

The user-level wrappers need no opt-in: the install writes them for every Codex it detects (or for
the harnesses you name with `--harness`). The `AGENTS.md` block is different. Only a repository's
own `agent_harnesses` authorizes it, declared in that repository's `.docket.yml` or
`.docket.local.yml`:

```yaml
# in <repo>/.docket.yml  — commits the choice for the whole team
agent_harnesses: [claude, codex]
```

```yaml
# or in <repo>/.docket.local.yml  — this machine only, gitignored, never leaves your clone
agent_harnesses: [claude, codex]
```

**The gotcha: a global `agent_harnesses` writes nothing into a repository.** The installer ignores
a value set in `~/.config/docket/config.yml`, so setting it there produces no `AGENTS.md` block in
any repo. An unknown harness name is an error.

Either repository file opts the repo in; the first of local-then-committed that declares the key
wins the list outright. Re-run the install after editing it, and it reconciles the `AGENTS.md`
block in one journaled transaction. `agent_harnesses` has three states: *absent* (there is no
default) touches no repository surface, a *non-empty* list reconciles exactly the harnesses named,
and an *explicit empty* list (`agent_harnesses: []`) retires every docket-owned repository surface
the repo previously had, including this `AGENTS.md` block.

**Why it works this way.** The `AGENTS.md` dispatch block is *committed*. If a global setting on your
machine generated that committed block, a collaborator without the same global config would see a
`docket` block their own configuration doesn't call for. Taking the opt-in from the repository's own
config keeps the committed file the same in every clone.

> Because the block is shared with opencode, it is removed only when the **last**
> `AGENTS.md`-dispatch harness is de-listed. De-listing Codex from a repo that still targets opencode
> (or the reverse) leaves the block in place, correctly; de-listing the last one removes it, and
> you commit that removal like any other edit. Your own `AGENTS.md` content outside the docket
> markers is preserved untouched.

### Pinning models and effort

The `.toml` wrappers carry the model and effort resolved from your **global** config's `agents:`
table over the built-in `codex:` block. Agent overrides belong in the global file only
(`~/.config/docket/config.yml`); an `agents:` pin in `.docket.yml` or `.docket.local.yml` makes
docket refuse to change the repository until you move it there. Your value overrides the
shipped pin field by field, so pinning only `model` keeps the shipped `effort`. If your model does
not accept that effort token, pin `effort` alongside it. Use the model IDs Codex itself reports:

```sh
codex debug models | jq -r '.models[] | .slug'
```

See [Models](models-and-effort.md) for the full rules.

### Two invocation paths — one contract

Docket supports exactly two ways to start its work under Codex, and both are first-class — neither is
a workaround:

1. **Prose, routed by the dispatch block.** A plain request ("refresh the docket board") is routed by
   the repo's managed `AGENTS.md` dispatch block to the registered same-name `docket-*` agent.
2. **Direct invocation.** `@docket-status` (or any `@docket-…` agent) starts that same registered
   wrapper explicitly.

Either way, the wrapper you land in may need to dispatch further docket agents — planning, build,
review, grooming's critic, finalize's resolver and repair. Every generated Codex wrapper carries the
same typed routing rule: **`[docket launch: root-coordinator]` → foreground `agent.enter` at the
caller's cwd; `[docket worktree: feature]` → foreground `agent.enter` with a verified canonical
`--worktree` and the unchanged structured payload; an unmarked metadata-scoped ordinary child →
native named-agent dispatch.** For that native named-agent leg, a tool inventory read from *inside*
another tool (a nested orchestration namespace) intentionally omits Codex's top-level collaboration
controls, so an agent must never conclude from such an inventory that dispatch is unavailable — only
a failed direct attempt or an explicit policy denial establishes that. The harness-neutral statement
of this rule lives in the `docket-convention` skill's *Dispatch-capability resolution* section.

`agent.enter` resolves the native role definition with the same precedence Codex applies to the
entered thread: a `<effective-worktree>/.codex/agents/<role>.toml` the repository itself provides
comes first, then the user-level `~/.codex/agents/<role>.toml` that docket installs. “Effective”
means the caller repository for a root coordinator and the verified `--worktree` for a feature
child. docket never writes a repository definition, but a present one that is malformed or carries
the wrong identity is refused; it never silently falls back to the user-level role.

### Restart after (re)generating

Codex registers agent definitions **once, at process start**. After any install that changed a
wrapper or the dispatch block, start a **fresh Codex application/CLI process** before relying on the
new definitions. Opening another conversation inside an already-running process is **not sufficient**
— that process is still holding the definitions it loaded at start.
