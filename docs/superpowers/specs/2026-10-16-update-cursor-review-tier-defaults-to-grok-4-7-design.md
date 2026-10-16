<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0554 — Refresh the shipped Claude and Cursor model defaults](../../changes/active/0554-update-cursor-review-tier-defaults-to-grok-4-7.md)**
<!-- docket:backlink:end -->

# Refresh the shipped Claude and Cursor model defaults — design

Change #554, groomed 2026-10-16 (revised the same day to cover the whole Claude block).

## Goal

Ship new built-in model defaults by replacing every row of the Claude block and every row of the
Cursor block (17 agents each). Codex and opencode values don't change. There are no changes to
behavior, schema, or emitters. The work is a data edit, plus the comments and the one doc that
describe the data.

## Target values

### Claude: all 17 rows

| agent | today | new |
|---|---|---|
| adr | `claude-opus-5` / low | `claude-opus-5-5` / low |
| auto-groom | `claude-opus-5` / low | `claude-opus-5-5` / medium |
| auto-groom-critic | `claude-opus-5` / medium | `claude-opus-5-5` / high |
| brainstorm-consultant | `claude-opus-5` / medium | `claude-opus-5-5` / medium |
| build-economy | `claude-sonnet-5` / low | `claude-opus-5-5` / low |
| build-standard | `claude-opus-5` / low | `claude-opus-5-5` / medium |
| build-premium | `claude-opus-5` / medium | `claude-opus-5-5` / high |
| build-max | `claude-opus-5` / high | `claude-opus-5-5` / xhigh |
| finalize-change | `claude-opus-5` / low | `claude-opus-5-5` / low |
| implement-next | `claude-opus-5` / medium, `cache_ttl: 1h` | `claude-opus-5-5` / low, `cache_ttl: 1h` |
| integration-repair | `claude-opus-5` / medium | `claude-opus-5-5` / medium |
| plan-writer | `claude-opus-5` / high | `claude-opus-5-5` / high |
| rebase-resolver | `claude-opus-5` / medium | `claude-opus-5-5` / medium |
| review-lean | `claude-sonnet-5` / high | `claude-opus-5-5` / low |
| review-standard | `claude-opus-5` / medium | `claude-opus-5-5` / medium |
| review-deep | `claude-opus-5` / high | `claude-opus-5-5` / high |
| status | `claude-haiku-4-5-20251001` / medium | `claude-haiku-5-5` / medium |

Daniel supplied 15 of these rows. Grooming settled the other two: `integration-repair` and
`rebase-resolver` move to the new model at their current effort, `medium`. `implement-next` keeps
its `cache_ttl: 1h` pin, and the comment above that row stays as it is.

### Cursor: all 17 rows (every effort stays `auto`)

| agent | today | new |
|---|---|---|
| adr | `cursor-grok-4.5-high` | `cursor-grok-4.6-high` |
| auto-groom | `cursor-grok-4.5-medium` | `cursor-grok-4.6-medium` |
| auto-groom-critic | `cursor-grok-4.5-high` | `cursor-grok-4.6-high` |
| brainstorm-consultant | `cursor-grok-4.5-high` | `cursor-grok-4.6-high` |
| build-economy | `cursor-grok-4.5-low` | `cursor-grok-4.6-low` |
| build-standard | `cursor-grok-4.5-medium` | `cursor-grok-4.6-medium` |
| build-premium | `cursor-grok-4.5-high` | `cursor-grok-4.6-high` |
| build-max | `claude-opus-5-high` | `claude-opus-5-5-high` |
| finalize-change | `cursor-grok-4.5-high-fast` | `cursor-grok-4.6-low` |
| implement-next | `cursor-grok-4.5-high` | `cursor-grok-4.6-low` |
| integration-repair | `cursor-grok-4.5-high` | `cursor-grok-4.6-high` |
| plan-writer | `cursor-grok-4.5-xhigh` | `claude-opus-5-5-medium` |
| rebase-resolver | `cursor-grok-4.5-high` | `cursor-grok-4.6-high` |
| review-lean | `cursor-grok-4.5-medium` | `grok-4.7-low` |
| review-standard | `cursor-grok-4.5-high` | `grok-4.7-medium` |
| review-deep | `claude-opus-5-high` | `grok-4.7-high` |
| status | `cursor-grok-4.5-low-fast` | `claude-haiku-5-5-thinking-medium` |

These are Daniel's IDs, copied verbatim. The review tiers' `grok-4.7-*` IDs have no `cursor-`
prefix, while the other Grok rows keep it; that is also verbatim. Docket treats model IDs as
opaque (ADR-0015), so nothing in the repository can check them. Confirming them is a human item
(see *Verification*).

In the roster Daniel supplied, review-deep was `grok-4.7-medium` with `effort: high`. Grooming
settled it as `grok-4.7-high` with `effort: auto`. Every Cursor row keeps the rule that the
variant is encoded in the ID and effort is `auto`, so no wrapper gets an `[effort=…]` bracket.

## Where the values live (edit every copy)

1. `agents/harness-defaults.yml` is the shipped table.
2. `internal/config/defaults.go` holds `builtinAgentDefaults` and `builtinAgentCacheTTL`. The Go
   tables must equal the YAML. Cursor's `auto` is written exactly the way the existing rows
   write it.
3. `.docket.example.yml` has the commented `agents:` example, with the whole `claude:` block and
   the whole `cursor:` block. Keep its column alignment.
4. `internal/assets/embedded/tree/` is regenerated with `go generate ./internal/assets/`. Never
   hand-edit it.
5. `docs/reference/harness/validation.md` currently says the only Cursor wrappers allowed to carry
   a Claude model ID are `docket-build-max` and `docket-review-deep` (`claude-opus-5-high`).
   After this change the exceptions are `docket-build-max` (`claude-opus-5-5-high`),
   `docket-plan-writer` (`claude-opus-5-5-medium`), and `docket-status`
   (`claude-haiku-5-5-thinking-medium`). `docket-review-deep` is no longer one of them.

Tests that assert a specific shipped value, such as a resolve or install test expecting
`claude-sonnet-5` for build-economy, move to the new value. Tests driven by their own fixed input
table, such as the harness renderer goldens, don't change.

Do **not** touch frozen historical data: `internal/install/legacydata/`,
`internal/install/testdata/legacy/`, any `testdata/repositories/` tree that survives #548, the
release notes under `docs/release/`, archived specs, plans, and results, and the corpus ADR copy
under `internal/repository/testdata/`. These files record past states.

## Comments in `agents/harness-defaults.yml` that must stay true

- **The review-deep-equals-build-max rule is removed altogether.** Daniel decided at grooming that
  it no longer holds as a principle. On Claude, review-deep is `high` and build-max is `xhigh`; on
  Cursor, review-deep is `grok-4.7-high` and build-max is `claude-opus-5-5-high`. Delete the
  sentence from the review-tier comment in the Claude block, and delete the matching
  "review-deep equals build-max" sentence in the opencode block's comment. Opencode's values are
  unchanged and happen to stay equal, but the comment shouldn't state it as a rule. Don't add a
  replacement rule.
- **The rest of the Claude review-tier comment.** Its sentence about the lean tier deliberately
  departing on Claude (Sonnet instead of one step above the build) no longer describes the table:
  review-lean is now `claude-opus-5-5` / low. Remove or rewrite it so that every remaining
  sentence describes the shipped rows.
- **The build-economy comment.** It explains why build-economy ships Sonnet rather than Haiku. The
  row is now Opus 5.5 at `low`. Rewrite the comment to say why build-economy doesn't use a smaller
  model: a contract fumble halts the build instead of escalating, so the risk lands on the human.
  Haiku (`claude-haiku-5-5`) is the documented cost-aggressive override, replacing the old
  `claude-haiku-4-5-20251001` mention.
- **The Cursor block header.** It describes the tiering as "cheapest for the mechanical status
  sweep, mid for design and repair work, highest for the max build tier." Rewrite it to describe
  the new table: Grok 4.6 at the level in its name for most rows, Grok 4.7 for all three review
  tiers, and Claude models selected through Cursor for `status`, `plan-writer`, and `build-max`.
  Keep the existing explanation of why every effort is `auto`.

## Sequencing

This change depends on #548. Today `internal/config/defaults_test.go` compares the Go table with a
frozen byte copy (`testdata/repositories/v0.9.13/agents-harness-defaults.yml`), so any edit to the
shipped file would require a new versioned fixture tree, which is exactly what #548 deletes. Once
#548 lands, the tests read `agents/harness-defaults.yml` directly, and this change is a plain data
edit with no fixture work. If reconcile finds #548 changed the parity tests in some other way,
follow whatever #548 shipped. Never create a versioned fixture folder for this change.

## Verification

- The full suite is green (`build.test_command`). The YAML↔Go parity test and the
  `.docket.example.yml` mirror guards are the in-repo proof that every copy agrees.
- A whole-repo grep over maintained source (excluding the frozen data listed above) finds no
  `cursor-grok-4.5`, `claude-opus-5-high`, `claude-haiku-4-5-20251001`, or shipped-default use of
  `claude-opus-5` / `claude-sonnet-5`. Illustrative examples in prose, such as the override sample
  in the agent-layer reference, are not shipped defaults and can stay.
- **Human item (Important):** model IDs are vendor facts that no in-repo test can check. After
  installing, check that the wrappers resolve. On Claude Code, `docket-status` should run on
  `claude-haiku-5-5` and the other agents on `claude-opus-5-5`. With `cursor` in
  `agent_harnesses`, open Cursor's agent picker and confirm each docket agent runs on its new
  model, especially the `grok-4.7-*` rows and `claude-haiku-5-5-thinking-medium`.

## Out of scope

- The Codex and opencode values. Only opencode's comment changes, as described above.
- The interactive skills' advisory session-model recommendation (`claude-sonnet-5` in
  `docket-new-change` and `docket-groom-next`). That belongs to #166.
- The Cursor emitter's effort handling, and the opaque-ID rule (ADR-0015).
- Users' own global-config overrides. A global pin still shadows the new built-in, which is how
  layering is meant to work.
