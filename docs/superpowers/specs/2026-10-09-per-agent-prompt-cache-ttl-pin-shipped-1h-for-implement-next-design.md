<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0546 — Per-agent prompt-cache TTL pin, shipped 1h for implement-next](../../changes/active/0546-per-agent-prompt-cache-ttl-pin-shipped-1h-for-implement-next.md)**
<!-- docket:backlink:end -->

# Per-agent prompt-cache TTL pin — design

## Summary

Add an optional `cache_ttl` field to docket's per-agent pin table, next to `model` and `effort`.
The Claude Code adapter renders it as `experimental: { cacheTtl: <value> }` in the generated
wrapper's frontmatter. Docket ships `cache_ttl: 1h` for the claude `implement-next` entry only,
because the cost data below shows that agent losing about half its spend to cache expiry, while
every other agent would get more expensive under a 1-hour TTL.

## Research findings (2026-10-09)

Source: every subagent transcript under `~/.claude/projects/-Users-homer-dev-docket/*/subagents/`
(the `usage` block of each assistant message, deduplicated by message id). Costs count only
`claude-opus-5-5` turns, at Opus 5.5 prices: input $4/M, 5-minute cache write $5/M, 1-hour cache
write $8/M, cache read $0.20/M, output $20/M.

### Where the money goes

| Agent (pinned effort) | Runs | Turns/run | Avg peak context | Opus $/run | Total | Cache writes that are expiry rewrites |
|---|---|---|---|---|---|---|
| implement-next (low) | 95 | 65 | 173k | $5.35 | **$508** | **87%** (461 rewrites) |
| build-standard (medium) | 427 | 24 | 97k | $0.96 | $410 | 5% |
| plan-writer (high) | 67 | 56 | 245k | $3.96 | $265 | 24% |
| build-premium (high) | 103 | 43 | 155k | $2.23 | $230 | 12% |

A "rewrite" is a turn that wrote more than 50k cache tokens while reading fewer than it wrote:
the cached prefix had expired and the whole context was written again. All recorded cache writes
used the 5-minute TTL; no 1-hour writes appear in any subagent transcript.

### implement-next per run

| Token type | Tokens/run | Cost/run | Share |
|---|---|---|---|
| Cache reads | 8.19M | $1.64 | ~31% |
| Cache writes (5-minute TTL) | 0.70M, of which ~0.60M are expiry rewrites | $3.48 | ~65% |
| Output (visible text and tool input, estimated) | ~11k | ~$0.22 | ~4% |

Measurement note: transcripts record `output_tokens` while the response is still streaming, so the
logged count undercounts. The output figure above is estimated from visible text and tool-input
characters (÷4). Thinking tokens aren't recorded at all, so every output figure is a lower bound.

implement-next is an orchestrator. It runs commands and dispatches workers, and it sits idle while
a build worker or the test suite runs. Those waits regularly outlast five minutes, so the cache
expires and the next turn writes the full ~120k context again.

### Estimated per-run cost under each option

| Option | implement-next | build-standard | build-premium | plan-writer |
|---|---|---|---|---|
| Opus 5.5, as today | $5.35 | $0.96 | $2.23 | $3.96 |
| Opus 5.5, 1-hour TTL | **$2.60** | $1.18 ↑ | $2.55 ↑ | $4.27 ↑ |
| Sonnet 5.5 (same effort) | $3.49 | $0.68 | $1.71 | $2.98 |
| Sonnet 5.5, 1-hour TTL | $2.11 | $0.79 | $1.87 | $3.13 |

The 1-hour TTL only pays where expiry rewrites dominate. A 1-hour write costs 2× input; a
5-minute write costs 1.25×. The workers rarely expire, so for them the 1-hour TTL is pure surcharge.
That is why the pin is per agent and why the shipped default covers implement-next only.

### Effort and model are weaker levers for implement-next

- Effort mostly changes output and thinking tokens, which are ~4% of implement-next's cost. The
  low → medium price gap on public benchmarks comes from output-heavy tasks and doesn't carry over
  to this agent. For the workers, visible output is 11–13% of cost plus unrecorded thinking, so
  effort is a modest lever there.
- Sonnet 5.5 cache reads cost the same $0.20/M as Opus 5.5, so Sonnet saves only on writes and
  output: about 35% for implement-next and 24–29% for the workers.
- Haiku 5.5 would be cheapest even at its >100k-token rate ($0.50/M input; 79% of implement-next
  turns are over 100k). It isn't recommended for implement-next: the agent must follow a ~110 KB
  skill contract exactly over ~65 turns, and a single intelligence score doesn't measure that.

## Claude Code's cache-TTL controls

Source: Claude Code docs, `code.claude.com/docs/en/prompt-caching.md` ("Choose the TTL yourself")
and `code.claude.com/docs/en/sub-agents.md` (supported frontmatter fields). Requires Claude Code
v2.1.248+ for the frontmatter field and v2.1.242+ for the env var and setting.

**Used: per-agent frontmatter.**

```yaml
experimental:
  cacheTtl: 1h
```

It sits under the `experimental` map, never at the top level. It's the only control scoped to one
agent, which is what the data calls for.

**Documented, deliberately not used:**

| Control | Scope | Why not |
|---|---|---|
| `CLAUDE_CODE_SUBAGENT_PROMPT_CACHE_TTL=1h` (env var) | every subagent, plus workflows, teammates, forks, compaction, and session titles | Too broad: it would make the workers 10–23% more expensive. It's also process environment, which docket doesn't own. |
| `subagentPromptCacheTtl: "1h"` (settings key) | same bucket as the env var | Same breadth problem. It's a user settings file, not a generated docket surface. |
| `ENABLE_PROMPT_CACHING_1H=1` | main conversation and subagents | Broadest of all. A reported bug says it didn't reach subagents on Bedrock. |
| `FORCE_PROMPT_CACHING_5M=1` | everything | The opposite lever. It overrides all of the above, frontmatter included. |

Precedence, highest first: `FORCE_PROMPT_CACHING_5M` > subagent env var > subagent setting >
frontmatter `cacheTtl` > `ENABLE_PROMPT_CACHING_1H` > default. A user who sets the broader
controls overrides docket's pin, which is the user's call.

**Caveats the docs record:**

- On a Claude subscription, frontmatter `cacheTtl: 1h` is ignored while usage is drawn from usage
  credits. The env var and setting aren't ignored, but 1-hour writes cost more on credits. API-key
  and cloud-provider billing default to 5 minutes for subagents.
- Claude Code documents no keep-warm, ping, or background-task way to hold a cache open. The TTL
  is the only documented fix.
- An open Claude Code issue reports conflicting evidence about whether subagent writes land in the
  1-hour bucket, with a maintainer saying the effective TTL is decided downstream. Verification
  (below) therefore checks the transcript, not the file.

## Design

### Config surface

`AgentSetting` (`internal/config/config.go`) gains a third field next to `Model` and `Effort`:
`CacheTTL Value[string]`, JSON/YAML key `cache_ttl`. It follows the established machinery
unchanged:

- It's honoured from the built-in table and the global config only, per field, exactly like
  `model` and `effort`. A repository-layer agent pin still blocks writes, as today.
- Values are opaque passthrough (ADR-0015): docket keeps no vendor allowlist, so `1h`, `5m`, or a
  future token all pass through to the harness verbatim.
- Unset means no `experimental.cacheTtl` is emitted, so wrappers without it are the same as today.

### Shipped default

`agents/harness-defaults.yml` (ADR-0064) gets `cache_ttl: 1h` on the claude `implement-next`
entry only. The compiled defaults in `internal/config/defaults.go` follow, and the existing test
that keeps the sidecar and the compiled table equal covers the new field. The sidecar's shape rule
"every listed entry supplies both model and effort" stays true; `cache_ttl` is an optional third
field and the header comment says so. The global-config starter that the first install scaffolds
mirrors the shipped claude block (ADR-0039; `internal/config/example_correspondence_test.go`
guards the mirror), so it shows the new field on implement-next and the otherwise-invisible
default stays visible and tunable.

### Rendering

The Claude adapter's `renderAgent` (`internal/harness/claude/claude.go`) writes, after
`model`/`effort`:

```yaml
experimental:
  cacheTtl: 1h
```

`verifyRoundTrip` extends to read `experimental.cacheTtl` back and refuses a mismatch, as it does
for model and effort.

### Non-Claude harnesses

Cursor, Codex, and OpenCode have no cache-TTL setting. A resolved `cache_ttl` on one of their
agents is left out of the generated files, with a warning that names the harness and agent and says
the harness has no prompt-cache TTL setting. This reuses the ADR-0077 warn-and-drop posture for an
effort docket can't attribute (trace the existing effort-drop warning in the install path and
extend it rather than adding a parallel one): the configuration never blocks, and the drop is loud rather than
silent. The shipped sidecar carries no `cache_ttl` for those harnesses, so a default install prints
no warning.

### Documentation

The agent-layer reference (`docket-convention/references/agent-layer.md`) and the README's agent
configuration section document:

- the `cache_ttl` key, its layering, and the shipped implement-next default;
- why it's scoped to one agent (the per-agent cost table, condensed);
- the global Claude Code controls above as alternatives docket deliberately doesn't use, with
  their precedence over the frontmatter pin;
- the usage-credits caveat.

Docs describe current behaviour only, with no change or PR citations.

## Testing

- Config: `cache_ttl` resolves built-in → global per field; a repository-layer `cache_ttl` is
  refused like other repository agent pins; the sidecar/compiled-table equality test covers it.
- Claude render: implement-next's generated wrapper carries `experimental.cacheTtl: 1h`; an agent
  without the field renders byte-identical to today; a round-trip mismatch is refused. Mutation
  check: strip the renderer's write and watch the wrapper test fail.
- Non-Claude harnesses: a global `cache_ttl` on a cursor, codex, or opencode agent leaves it out
  of that harness's file and prints the warning; the shipped default prints none.

## Verification after merge

Rebuild and reinstall per the repo's after-merge rule, restart the Claude Code session (the agent
registry loads at process start), and run one `docket-implement-next`. In that subagent's
transcript, confirm cache writes land in `usage.cache_creation.ephemeral_1h_input_tokens`, that
large expiry rewrites (over 50k written, fewer read) mostly disappear, and that cost per run moves
toward ~$2.60. If writes still land in the 5-minute bucket, record that in the results file as a
Claude Code-side limitation (see the open-issue caveat). Leave the pin in place: it's harmless
when ignored.

## Recommendations recorded for separate changes

Not built here; listed for human triage.

1. **Sonnet 5.5 for build-standard.** It has the most runs, so the model switch saves the most:
   ~$0.28/run, about $120 over this history. Trial a few runs and compare build outcomes against
   the Opus history before changing the shipped default.
2. **A context diet for plan-writer.** It has the largest context of any agent (245k average peak,
   80% of turns over 100k) at high effort. Trimming what it loads likely beats a model switch.
3. **Measure finalize-change** the same way before giving it the 1-hour TTL. It also waits on test
   suites, but its transcripts weren't analysed.
4. **A Sonnet 5.5 medium trial for implement-next**, after this change lands and is measured. Use
   medium, not low: effort barely moves this agent's cost. Judge it by `run.verdict` outcomes and
   gate stops, since retries can wipe out the ~$0.50/run saving left once the TTL fix is in.
