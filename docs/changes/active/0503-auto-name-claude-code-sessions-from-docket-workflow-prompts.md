---
id: 503
slug: 'auto-name-claude-code-sessions-from-docket-workflow-prompts'
title: 'Auto-name Claude Code sessions from docket workflow prompts'
status: 'deferred'
priority: 'low'
type: 'feat'
created: '2026-10-04'
updated: '2026-10-04'
depends_on: []
stacked_on:
related: [345]
discovered_from: []
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

Daniel names every Claude Code session by hand after the docket workflow it runs — `<id>-groom`, `<id>-implement`, `<id>-finalize` (for example `0497-groom`) — by typing `/rename` after starting the workflow. The names make sessions findable in `claude --resume` and in the Claude mobile app's session list. This is a convenience, not a critical feature, so whatever automates it must stay light: Daniel's explicit concern is running a hook on every message just to name a session.

### Research (2026-10-04)

**Claude Code (verified live on 2.1.288).** A session's name can be set five ways:

1. `/rename` — human only; Claude cannot invoke it.
2. `claude --name <name>` at launch.
3. A `SessionStart` hook's `hookSpecificOutput.sessionTitle` — applies on startup/resume/fork only, before the first message, so it cannot know the change id.
4. A `UserPromptSubmit` hook's `hookSpecificOutput.sessionTitle` — the docs: "Use to name sessions automatically based on the prompt content."
5. The Agent SDK's `renameSession()` — an external program.

No other hook event can set a title. `PreToolUse` on the `Skill`/`Agent` tool fires exactly when a docket workflow starts (no guessing from prose), but has no title output.

Live probe: a `UserPromptSubmit` hook received the typed prompt verbatim and returned `sessionTitle: "0497-probe"`; the session transcript recorded a `custom-title` entry `0497-probe` — the same entry `/rename` writes.

Hook facts that shape the design:

- `UserPromptSubmit` fires once per submitted message — not per tool call or shell command — and also on `/loop` ticks, background-subagent report-backs, and cross-session messages. It supports no matcher, so its command runs on every message. Default timeout 30 s; a hung hook stalls the prompt.
- Its input carries `prompt` and, when the session already has a custom title, `session_title`. An AI-generated title does not count as a custom title.
- Hook locations: `~/.claude/settings.json` (all projects, this machine); `<repo>/.claude/settings.local.json`; `<repo>/.claude/settings.json` (committed); a plugin's `hooks/hooks.json`; skill frontmatter (registered once the skill is invoked; `once: true` is honored only there); subagent frontmatter (only while that subagent runs).
- A skill-frontmatter hook would exist only in docket sessions and could remove itself after one run, but only grooming runs as a skill in the main session. Implement and finalize run as dispatched agents, whose hooks cannot rename the parent session. Rejected.
- Daniel starts sessions from the terminal and from the Claude mobile app via Remote Control (sessions run on his Mac; he uses no cloud sessions), so one hook in `~/.claude/settings.json` covers both. Cloud sessions would not read that file. Remote Control picks the remote title in this order: a `--name`/`--remote-control` name, then the `/rename` title, then the last meaningful message, then an auto-generated name.
- Daniel drives docket workflows with prose prompts (`groom 497`), not `/docket-*` slash commands, which do not currently work in docket (see 0345). So `UserPromptExpansion` — slash commands only, with parsed `command_name`/`command_args` but no title output — does not apply.

**Other harnesses** (desk research of current docs and source, 2026-10-04; not probed live):

| Harness | Manual rename | Automatic option |
|---|---|---|
| Codex CLI (v0.160.0) | `/rename <name>`; `codex resume <name>` | Only app-server JSON-RPC `thread/name/set` or the Python SDK `thread.set_name()`. No launch flag; none of its 12 hook events has a title output. A hook calling `thread/name/set` is untested and may not update the live TUI. |
| Cursor IDE + CLI | sidebar/tab rename; CLI `/rename` | None: no `--name` launch flag (Cursor staff, 2026-08-20), no title output from any hook, no agent-initiated rename (open forum requests). The Cloud Agents API takes a `name` at creation only. |
| OpenCode (v1.18.34) | `/rename`, `ctrl+r`, desktop inline edit | `opencode run --title`, `PATCH /session/:id {title}`, SDK `session.update`; plugins can call `client.session.update`. Auto-titles never overwrite a set title. |
| Gemini CLI (v0.62.0) | none (named checkpoints via `/chat save <tag>`) | none |

Sources: code.claude.com/docs/en/hooks (UserPromptSubmit and SessionStart decision control, Hook locations, Hooks in skills and agents); code.claude.com/docs/en/remote-control; code.claude.com/docs/en/sessions; Codex `codex-rs/app-server-protocol` (`ThreadSetName`) and `codex-rs/hooks/src/schema.rs`; cursor.com/docs/cli/reference/slash-commands, cursor.com/docs/agent/hooks, forum.cursor.com/t/want-rename-command-with-cli/168920; OpenCode `packages/opencode/src/cli/cmd/run.ts` and `packages/plugin/src/index.ts`.

## What changes

Implementation stub — option 1 below, chosen by Daniel. A hypothesis for grooming, not a settled design.

### Options weighed

1. **Per-message `UserPromptSubmit` hook, kept minimal** — chosen. The only automatic option that also covers sessions started from the phone.
2. Terminal shortcut (`dg 497` runs `claude --name 0497-groom "groom 497"`): exact names and no hook, but does nothing for phone-started sessions, and it is a personal shell alias rather than a docket change.
3. Leave naming manual.

### Hypothesis

A user-level `UserPromptSubmit` hook in `~/.claude/settings.json` runs a small program on each submitted message:

- If the input carries `session_title` (the session already has a custom name, from `/rename`, `--name`, or this hook), exit 0 immediately without reading the prompt. A human's name always wins, and a named session costs one field check.
- Otherwise, if the prompt **starts with** `groom`, `implement`, or `finalize` (case-insensitive; a leading "let's" or "please" ignored) followed closely by a change number, return `hookSpecificOutput.sessionTitle` = `<id zero-padded to 4 digits>-<verb>`.
- Otherwise output nothing. Never block a prompt, never add context, never touch the network; any error exits 0 silently; set a short `timeout`.

Agreed examples:

| Message | Session name |
|---|---|
| `groom 497` | `0497-groom` |
| `Let's implement 497` | `0497-implement` |
| `finalize change 0497` | `0497-finalize` |
| `why did finalize fail on 497?` | none — does not start with the verb |
| `groom the next one` | none — no number |
| any message in a session that already has a name | none — the existing name wins |

The verb vocabulary is exactly groom / implement / finalize; Daniel confirmed he uses no other phrasings.

### Open questions for grooming

- **Where the logic lives:** a `docket` subcommand reading the hook JSON on stdin (tested Go; the Claude-specific payload belongs behind the Claude harness adapter per the three-boundary rule) vs a standalone script shipped with docket. Measure per-message process-start cost either way.
- **Who registers the hook:** Daniel pastes a documented snippet into `~/.claude/settings.json` once per machine (leaning: keeps docket's installer out of a user-owned file) vs the installer merging it in (the first time docket would write `settings.json`: it must preserve existing keys, stay idempotent across reinstalls, and remove cleanly). Not yet decided.
- **Renaming within one session:** an earlier note had the hook replace a name it made itself (`0497-groom` becomes `0497-implement` when moving on in the same session); the chosen option-1 shape stops once any name exists. Pick one.
- **Phone display:** confirm a hook-set title shows in the mobile app's session list, including for sessions a `claude remote-control` server spawns (a `--name` passed to the server outranks a `/rename`-equivalent title).
- **False positives:** a message like `implement 2 tests` in an unnamed session would match. Decide whether that is acceptable or whether the number must resolve to an existing change (only on a prefix match, so the per-message cost stays trivial).

## Out of scope

- Codex, Cursor, and Gemini CLI: no supported way to set a session name automatically (see the research above).
- OpenCode: possible via `--title` or a plugin calling `session.update`; a follow-up only if docket work moves to OpenCode.
- Cloud sessions (claude.ai/code on the web): they do not read `~/.claude/settings.json`, and Daniel does not use them.
- Workflows started without a change id (`groom the next one`, a bare implement-next pick, new-change, which mints its id mid-session): the id is unknown when the message is sent, so these stay unnamed and use `/rename`.
- `/docket-*` slash commands and the `UserPromptExpansion` event; revisit if 0345 makes slash commands work.
- Naming dispatched subagents or other sessions; renaming by writing transcript files directly or through the Agent SDK.
- A terminal launcher shortcut (`claude --name …`): a personal alias if wanted, not part of this change.

## Why deferred

A hook is too much overhead for the convenience this change would bring.
