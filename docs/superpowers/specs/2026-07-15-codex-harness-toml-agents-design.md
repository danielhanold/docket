# Codex harness: TOML agent generation + AGENTS.md dispatch — design

**Change:** 0077 · **Date:** 2026-07-15 · **Status:** approved (brainstormed with Daniel)

## Context

docket's agent layer generates model/effort-pinned subagent wrappers per harness
(`agent_harnesses:`), and `codex` is already a valid harness token — but `sync-agents.sh`
emits the same markdown-frontmatter wrapper format for every harness. OpenAI Codex CLI
does not read that format: per its subagent documentation
(https://learn.chatgpt.com/docs/agent-configuration/subagents), Codex loads standalone
**TOML** agent files from `~/.codex/agents/` (personal) and `<repo>/.codex/agents/`
(project), with fields `name`, `description`, `developer_instructions`, and optional
`model` / `model_reasoning_effort` (values `minimal` … `ultra`), `sandbox_mode`,
`mcp_servers`, `skills.config`, `nickname_candidates`.

So today a repo that sets `agent_harnesses: [claude, codex]` generates
`.codex/agents/docket-*.md` files that Codex silently ignores — dead output. Codex also
has no analog of Cursor's generated `docket-dispatch.mdc`, so a directly-invoked docket
skill would run inline at the session model, defeating the pin (the same inline quirk
Cursor and Claude Code each solve their own way).

What already works and is untouched: `codex` in `DOCKET_GI_HARNESS_TOKENS`,
`link-skills.sh` linking docket skills into `~/.codex/skills` when present, and the
non-claude fallback-model warning in `sync-agents.sh`.

## Design

### 1. Per-harness emitter registry in sync-agents.sh

A small dispatch table maps harness token → (file extension, emitter function):

- `codex` → `.toml` + `emit_codex_toml()`
- every other token → `.md` + the existing `emit()` (byte-identical output to today)

The built-in `agents/docket-*.md` files remain the single source of truth. The codex
emitter transforms a built-in wrapper into TOML:

| built-in wrapper (markdown) | Codex TOML field |
|---|---|
| filename stem (`docket-<name>`) | `name` |
| frontmatter `description:` | `description` |
| body (the wrapper prose, incl. skill/convention injection text) | `developer_instructions` (multi-line TOML string) |
| resolved `model` | `model` (omitted when it resolves to empty/inherit) |
| resolved `effort` | `model_reasoning_effort` (omitted on `effort: auto` or no effort) |

Claude-specific frontmatter that has no TOML analog (`skills:`, `tools:`, `agent:`) is
not carried as fields; the `skills:` preload intent is expressed in
`developer_instructions` prose (load the named skill from the linked skills directory
before acting). Effort values pass through **verbatim** — docket's `low`/`medium`/
`high`/`xhigh`/`max` are all valid `model_reasoning_effort` values — consistent with
ADR-0015's unvalidated passthrough (docket never validates model IDs or effort values;
the harness interprets them).

Both generation passes fan out through the registry: the user-level pass writes
`~/.codex/agents/docket-*.toml`, the per-repo pass writes
`<repo>/.codex/agents/docket-*.toml`. Everything else about the layer — harness-first
`agents:` resolution, always-full-set generation, opt-in per-repo pass, the
fallback-model warning for non-claude harnesses — is unchanged.

### 2. Dispatch: a managed block in the repo's AGENTS.md

When `codex` is in the repo's `agent_harnesses`, the per-repo pass maintains a
marker-bounded docket block in `<repo>/AGENTS.md` (created if absent) instructing Codex:
when a `docket-*` skill is invoked directly, delegate to the matching `docket-<name>`
agent instead of running it inline — the Codex analog of Cursor's dispatch rule.

- Markers are markdown comments (`<!-- docket:dispatch:start (managed by docket — do not
  hand-edit) -->` / `<!-- docket:dispatch:end -->`); the writer reuses the hardened
  managed-block pattern from `scripts/lib/docket-gitignore-block.sh`: closed-block guard
  (refuse on malformed/dangling markers), idempotence (no write when current), outside
  bytes preserved verbatim.
- The block is **committed** (unlike the wrappers): its content is machine-neutral —
  agent names and delegation instructions only, no model IDs; the pins live in the
  gitignored TOML files. sync-agents.sh prints a one-time commit notice when it writes
  or repairs the block, same as the .gitignore block.
- Only the dispatch-capable autonomous agents are listed (same set as the Cursor rule);
  content is assembled from the same per-agent dispatch intent, not hand-duplicated
  per harness.
- De-listing `codex` from `agent_harnesses` removes the block (prune), mirroring how a
  de-listed cursor drops its dispatch rule.

A user-level `~/.codex/AGENTS.md` variant is deliberately **out of scope** until change
0078's live validation shows how Codex merges user-level and project instructions.

### 3. Housekeeping

- **.gitignore block:** the emitter in `docket-gitignore-block.sh` additionally emits
  `.codex/agents/docket-*.toml` (the existing `.md` patterns stay for all tokens; the
  block remains a pure constant). All three writers pick this up automatically.
- **Prune:** orphan pruning extends to `.toml` wrappers (a removed built-in drops its
  TOML wrapper; de-listing `codex` drops the wrappers and the AGENTS.md block).
- **`--check`:** the tracked-file leg covers `.codex/agents/docket-*.toml`; the advisory
  content-staleness leg regenerates and diffs TOML like it does markdown. The AGENTS.md
  dispatch block is **exempt** from the tracked-file leg (it is meant to be committed)
  but gets its own presence/currency check, CI-meaningful like the .gitignore leg.

### 4. Testing

New cases in the existing `tests/` suite (no live Codex needed):

- codex in `agent_harnesses` generates the full built-in set as valid TOML (parse-check
  with a minimal shell/awk assertion, not a TOML library): field mapping, model/effort
  passthrough, `effort: auto` dropping the key, inherit-model omission.
- Non-codex harness output is byte-identical to pre-change output (regression guard).
- AGENTS.md block: create-when-absent, idempotent re-run, outside-bytes preserved,
  malformed-marker refusal, prune on de-list.
- .gitignore block includes the TOML pattern; `--check` legs behave as specified.

## Out of scope

- Live Codex CLI behavior verification (change 0078, depends on this landing).
- User-level `~/.codex/AGENTS.md` dispatch instructions.
- `sandbox_mode`, `mcp_servers`, `skills.config`, `nickname_candidates` TOML fields —
  omitted from generated files; a future `agents:` schema extension can add them when a
  need appears.
- Any change to `link-skills.sh` (Codex skills linking already works).

## Risks / verify at plan time

The TOML field names and locations above come from a single fetch of the linked doc
page, summarized by a model. **Before coding, re-verify the exact field names, file
extension, and directory paths against the live Codex documentation** (and `codex --help`
if available). If Codex turns out to also accept markdown agents, the emitter registry
still stands — pick the format the docs mark canonical.
