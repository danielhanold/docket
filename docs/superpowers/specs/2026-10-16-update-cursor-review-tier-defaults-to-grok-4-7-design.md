<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0554 — Refresh the shipped Cursor model defaults and Claude's status model](../../changes/active/0554-update-cursor-review-tier-defaults-to-grok-4-7.md)**
<!-- docket:backlink:end -->

# Refresh the shipped Cursor model defaults and Claude's status model — design

Change #554, groomed 2026-10-16.

## Goal

Ship new built-in model defaults: a full replacement of the Cursor block (all 17 agents), and one
row of the Claude block (`status`). No behavior, schema, or emitter changes. This is a data edit
plus the comments and the one doc that describe the data.

## Target values

### Cursor — all 17 rows (every effort stays `auto`)

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

The IDs are Daniel's, copied verbatim. That includes the review tiers' `grok-4.7-*` IDs having no
`cursor-` prefix while the other Grok rows keep it. Docket treats model IDs as opaque (ADR-0015),
so nothing in the repository can check them. Confirming them is a human item (see *Verification*).

The roster Daniel supplied listed review-deep as `grok-4.7-medium` with `effort: high`. Grooming
settled it as `grok-4.7-high` with `effort: auto`. Every Cursor row keeps the "variant is encoded
in the ID, effort is `auto`" rule, so no wrapper gets a `[effort=…]` bracket.

### Claude — one row

| agent | today | new |
|---|---|---|
| status | `claude-haiku-4-5-20251001` / `medium` | `claude-haiku-5-5` / `medium` |

Every other Claude row is unchanged.

## Where the values live (edit all of them)

1. `agents/harness-defaults.yml`, the shipped table.
2. `internal/config/defaults.go`, `builtinAgentDefaults`. The Go table must equal the YAML. In the
   Go table Cursor's `auto` is written as `"auto"`, the same as the existing rows.
3. `.docket.example.yml`, the commented `agents:` example (both the `claude:` status row and the
   whole `cursor:` block). Keep its column alignment.
4. `internal/assets/embedded/tree/`, regenerated with `go generate ./internal/assets/`, never
   hand-edited.
5. `docs/reference/harness/validation.md`. Today it says the only Cursor wrappers allowed to
   carry a Claude model ID are `docket-build-max` and `docket-review-deep` (`claude-opus-5-high`).
   After this change the exceptions are `docket-build-max` (`claude-opus-5-5-high`),
   `docket-plan-writer` (`claude-opus-5-5-medium`) and `docket-status`
   (`claude-haiku-5-5-thinking-medium`). `docket-review-deep` is no longer one of them.

Do **not** touch the frozen historical data: `internal/install/legacydata/`,
`internal/install/testdata/legacy/`, any `testdata/repositories/` tree that survives #548, release
notes under `docs/release/`, or archived specs, plans, and results. They record past states.

## Comments in `agents/harness-defaults.yml` that must stay true

- **Cursor block header.** It says the tiering is "cheapest for the mechanical status sweep, mid
  for design and repair work, highest for the max build tier". Rewrite it to describe the new
  table: Grok 4.6 at the level in its name for most rows, Grok 4.7 for all three review tiers,
  and Claude models selected through Cursor for `status`, `plan-writer` and `build-max`. Keep the
  existing explanation of why every effort is `auto`.
- **Review-tier comment in the Claude block.** It says "On every harness review-deep equals the
  build-max pin". That is no longer true for Cursor, where review-deep is `grok-4.7-high` and
  build-max is `claude-opus-5-5-high`. Daniel chose this deliberately at grooming: Cursor's review
  tiers run on Grok 4.7 throughout. Reword the comment so it states the Cursor exception. The rule
  still holds for claude, codex and opencode. The opencode block's own "review-deep equals
  build-max" sentence stays as it is.
- **build-economy comment in the Claude block.** It names `claude-haiku-4-5-20251001` as the
  documented cost-aggressive override. Change it to `claude-haiku-5-5`, so the comment and the
  status row name the same Haiku.

## Sequencing

This change depends on #548. Today `internal/config/defaults_test.go` compares the Go table with a
frozen byte copy (`testdata/repositories/v0.9.13/agents-harness-defaults.yml`). Any edit to the
shipped file means creating a new versioned fixture tree, which #548 deletes. Once #548 lands, the
tests read `agents/harness-defaults.yml` directly, and this change stays a plain data edit with no
fixture work. If reconcile finds that #548 changed how the parity tests read the table, follow
whatever #548 shipped. Never create a versioned fixture folder for this change.

## Verification

- Full suite green (`build.test_command`). The YAML↔Go parity test and the `.docket.example.yml`
  mirror guards are the in-repo proof that every copy agrees.
- A whole-repo grep for `cursor-grok-4.5`, `claude-opus-5-high`, and `claude-haiku-4-5-20251001`
  over maintained source (excluding the frozen historical data listed above) returns nothing.
- **Human item (Important):** the model IDs are vendor facts no in-repo test can check. After
  installing with `cursor` in `agent_harnesses`, open Cursor's agent picker and confirm that each
  docket agent runs on its new model. Pay attention to the `grok-4.7-*` rows and
  `claude-haiku-5-5-thinking-medium`. On Claude Code, confirm `docket-status` resolves
  `claude-haiku-5-5`.

## Out of scope

- Any Claude row other than `status`. The Codex and opencode blocks.
- Changing how the Cursor emitter handles effort, or the opaque-ID rule (ADR-0015).
- Users' own global-config overrides. A global pin still shadows the new built-in, which is how
  layering is meant to work.
