<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0546 — Per-agent prompt-cache TTL pin, shipped 1h for implement-next](../changes/active/0546-per-agent-prompt-cache-ttl-pin-shipped-1h-for-implement-next.md)**
<!-- docket:backlink:end -->

# Per-agent prompt-cache TTL pin, shipped 1h for implement-next — Results

**Human action:** Review and merge the PR as usual. After merge, one Important check confirms whether Claude Code actually honours the new setting; the change is harmless if it doesn't.

## Outcome

Docket agents can now carry a third per-agent pin, `cache_ttl`, next to `model` and `effort`. It resolves like the other pins (built-in table, then the global config, per field); a `cache_ttl` in a repository config file blocks writes like any other repository agent pin, and `cache_ttl: auto` in the global config clears a shipped pin.

- The Claude Code wrapper renders it as `experimental:` / `cacheTtl: <value>`. Wrappers without a value are byte-identical to before.
- Docket ships `cache_ttl: 1h` for the Claude `docket-implement-next` agent only. That agent sits idle past Claude Code's default 5-minute cache lifetime while builds and suites run, so most of its cost was the cache being rewritten.
- Cursor, Codex, and OpenCode have no such setting. A `cache_ttl` that reaches them is left out of their files and `docket install` prints an `agent-cache-ttl-dropped` warning. The shipped defaults trigger no warning.
- `docket diagnostic config` now shows the TTL on agent lines that carry one (for example `claude-opus-5 / medium / ttl 1h`).
- Docs: `docs/install/models-and-effort.md` and the convention's agent-layer reference describe the key, the shipped default, why it is scoped to one agent, and the broader Claude Code controls docket deliberately leaves alone.

Departures from the spec:

- The spec said to extend an existing "effort dropped" warning. No such warning exists (unsupported efforts are dropped silently in the Go adapters), so the new warning uses install's existing config-warning channel.
- User docs live in `docs/install/models-and-effort.md`; the README has no agent configuration section.
- `cache_ttl: auto` was added (not in the spec), mirroring `effort: auto`, so a user can turn the shipped pin off.
- Adding the config key required a new frozen fixture tree, `testdata/repositories/v0.9.13`, because frozen fixtures are immutable.
- The private-visibility config fold now recognises agent pins from the config registry instead of a hand-written list, so `cache_ttl` (and any future agent pin) is dropped and reported when moving a repository to private.

## Human actions and testing

### Important — confirm Claude Code honours the 1h cache TTL

The whole saving depends on Claude Code applying `experimental.cacheTtl` to subagents. An open Claude Code issue disputes that, and the setting is ignored while a subscription draws on usage credits. Change #547 holds the full measurement; this is the quick first check.

Prerequisites: the PR merged, the `docket` binary rebuilt and reinstalled per AGENTS.md, and a fresh Claude Code session (the agent registry loads at startup).

1. Run `grep -A1 '^experimental:' ~/.claude/agents/docket-implement-next.md`.
   Expected: `experimental:` followed by `  cacheTtl: 1h`.
2. Run one `docket-implement-next`, then open that subagent's transcript under `~/.claude/projects/<project>/<session>/subagents/` and look at the `usage.cache_creation` blocks.
   Expected: write tokens land in `ephemeral_1h_input_tokens`, not `ephemeral_5m_input_tokens`.

If writes stay in the 5-minute bucket, leave the pin in place (it costs nothing when ignored) and record the result on #547.

## Verification performed

- Full suite (`go run ./cmd/docket development test`) green at the final head through the build gate, after the review fixes. The run printed screening-only `BUDGET WATCH` / `PARALLEL-SENSITIVE` lines for existing integration tests and no `SERIAL CONFIRMED OVER BUDGET` line.
- Workers mutation-tested the new guards: removing the renderer's write, the round-trip check, the drop warning, the digest field, the `auto` sentinel, or the registry-keyed fold each turned a test red.
- A cross-harness test proves only the Claude implement-next wrapper changed; Cursor, Codex, and OpenCode files are byte-identical.
- Review: standard tier. One important and four minor findings, all fixed in-branch (visibility fold missing the new pin; TTL hidden in the config listing; digest counting a TTL that non-Claude harnesses never render; a stale freeze-point comment; dated cost figures removed from the agent-layer reference).

## Known issues and follow-ups

- **Sonnet 5.5 for `docket-build-standard`** (suggested, from the spec's cost research). It has the most runs, so a model switch saves the most (about $0.28 per run). Trial a few runs before changing the shipped default. No existing change fits (checked 27); a human may capture it as a new change.
- **A context diet for `docket-plan-writer`** (suggested). It has the largest context of any agent at high effort; trimming what it loads likely beats a model switch. No existing change fits (checked 27); a human may capture it as a new change.
- **Measure `docket-finalize-change` before giving it the 1h TTL** (suggested). It also waits on suites but its transcripts were not analysed. Related to #547 (same measurement method, different agent and decision); a human may capture it as a new change linking #547 under `related:`.
- **A Sonnet 5.5 medium trial for `docket-implement-next`** (suggested), only after the TTL saving is measured. Related to #547; a human may capture it as a new change linking #547 under `related:`.
- **Whether Claude Code honours the pin** (suspected limitation, see Human actions). Fits #547, which already covers it; no edit needed beyond running #547 once data exists.
