# Install and configure

Get docket onto your machine, decide where each setting lives, and set up the harness you run
agents under. Read the first three pages in order; the rest are reached for when a default does not
fit.

- [Installing docket](install.md) — what you need first, the one-command install, and the two
  notes that trip people up after it.
- [Keeping docket current](keeping-current.md) — why every pull is followed by a re-install, and
  what silently stays stale if it is not.
- [Global config](global-config.md) — the machine-wide file at `~/.config/docket/config.yml`: what
  belongs there, and how to enable a second harness.
- [Repo config](config-layers.md) — `.docket.yml` and `.docket.local.yml`, the four-layer
  precedence, the coordination fence, and what happens when a file is misplaced or malformed.
- [Workflow roles](workflow-roles.md) — rebind any of the five workflow steps to a different skill,
  or to none, with the `skills:` map.
- [Models](models-and-effort.md) — run each docket skill at its own model and effort instead of one
  session-wide tier, and how the pin survives a direct invocation.
- [Delegation](delegating-across-harnesses.md) — hand an agent's whole run to a different harness
  with its own subscription and models.

## Harnesses

One page per supported harness — a **harness** is the tool that runs the agent. Each covers what an
install writes for it, the opt-in it needs, and how to verify it works.

- [Claude Code](claude-code.md) — the reference harness: forked runs, and when to restart.
- [Cursor](cursor.md) — the permission and sandbox configuration docket needs under Auto-run.
- [Codex](codex.md) — the `.toml` wrappers, the `AGENTS.md` dispatch block, and the per-repo opt-in.
- [opencode](opencode.md) — the agent definitions, OpenRouter model IDs, and the `auto-approve`
  grant delegation demands.
