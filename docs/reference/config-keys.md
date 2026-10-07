# Config keys

This page lists every configuration key docket supports, with its built-in default and the layers
it may be set in. The full description of each key lives beside it in
[`../../.docket.example.yml`](../../.docket.example.yml), the shipped example file (ADR-0048 makes
it canonical, and a test keeps it in step with the binary's schema). The `docket-convention`
skill's section **"Configuration — `.docket.yml` (optional, committed on the default branch)"**
([`../../skills/docket-convention/SKILL.md`](../../skills/docket-convention/SKILL.md)) states the
resolution rules. To see the configuration your repository actually resolves to, run
`docket diagnostic config --repo-dir .`.

## Layers and scope

docket reads four layers. For each key, the highest layer that sets it wins:

1. machine-local `.docket.local.yml` (this clone; gitignored)
2. committed `.docket.yml` (every clone)
3. global `${XDG_CONFIG_HOME:-$HOME/.config}/docket/config.yml` (every repository on this machine)
4. the built-in defaults

Nested blocks merge key by key. A malformed file, an unknown key, or a bad value in any layer makes
the whole configuration invalid, and docket refuses to change the repository until it is fixed.

Each key has one scope:

- **repo-only** — settable only in the committed `.docket.yml`. A value in either machine layer is
  ignored with a warning, so one clone cannot move shared planning state (ADR-0019).
- **any layer** — settable in `.docket.yml`, `.docket.local.yml`, or the global config.
- **global-only** — settable only in the global config. Declared in `.docket.yml` or
  `.docket.local.yml`, it makes docket refuse to change the repository until you move it to the
  global config.

## Keys

| Key | Default | Scope | What it sets |
|---|---|---|---|
| `integration_branch` | `auto` (the repository's default branch) | repo-only | the branch pull requests merge into |
| `changes_dir` | `docs/changes` | repo-only | where change files and the board live on the `docket` branch |
| `adrs_dir` | `docs/adrs` | repo-only | where the ADR ledger lives on the `docket` branch |
| `results_dir` | `docs/results` | repo-only | where results records live on the `docket` branch |
| `finalize.gate` | `local` | any layer | `local` rebases and re-runs the suite before merging; `off` skips both |
| `finalize.test_command` | `""` (unconfigured) | any layer | the suite finalize runs after its rebase |
| `finalize.require_pr_approval` | `false` | any layer | whether a merge finalize picked on its own needs an approval |
| `finalize.resolver_max_attempts` | `10` | any layer | conflict-resolver attempts per rebase |
| `finalize.repair_max_attempts` | `6` | any layer | integration-repair attempts when the suite is red after the rebase |
| `build.gate` | `local` | any layer | `local` runs the build gate once after every task; `off` records `skipped` evidence |
| `build.test_command` | `""` (unconfigured) | any layer | the suite the build gate runs |
| `build.max_attempts` | `4` | any layer | full suite runs a build may spend, repairs included |
| `run.max_attempts` | `2` | any layer | attempts a tracked implement-next run may make on one change |
| `review.min_fix_severity` | `minor` | any layer | the lowest review-finding severity the fix pass repairs |
| `review.max_fix_tasks` | `10` | any layer | the most non-blocker fix tasks one fix pass dispatches |
| `reclaim.lease_ttl` | `72` | any layer | hours a claim on an in-progress change lasts |
| `reclaim.auto` | `false` | any layer | whether `docket maintenance sweep` reclaims an eligible change or reports it skipped |
| `learnings.enabled` | `true` | any layer | whether learnings are written and read |
| `gate_observation_budget` | `30` | any layer | minutes docket waits for a test-suite run it started to finish |
| `board_surfaces` | `[inline]` | any layer | which board views to render; `[]` renders none |
| `board.section_order` | `[in-progress, built, blocked, groomed, proposed, deferred]` | any layer | the order of the board's sections |
| `board.sorting.<section>.by` / `.direction` | `updated` / `desc` | any layer | how each board section is sorted |
| `change_types` | `[chore, docs, feat, fix, refactor, perf]` | any layer | the types a change can be classified as |
| `agent_harnesses` | none | any layer | the dispatch opt-in (below) |
| `agents.<harness>.<agent>.model` / `.effort` | the built-in table | global-only | model and effort pins for docket's agents |

**`agent_harnesses`** opts a repository in to docket's parent-facing dispatch surfaces: the
managed block in `CLAUDE.md` / `AGENTS.md` and, for Cursor, `.cursor/rules/docket-dispatch.mdc`.
It takes `claude`, `codex`, `cursor`, and `opencode`. While it is absent, `docket install` touches
no repository surface. Only `.docket.yml` or `.docket.local.yml` can opt a repository in; the
installer ignores a value in the global config. `docket repository init` asks for it (or takes
`--harnesses <list>|none`), `docket repository configure-harnesses` changes it later and refreshes
the surfaces in the same run, and `docket repository check` warns `harnesses-unset` while it is unset.

**`agents`** pins apply per agent and per field; anything you leave out keeps its built-in value.
The built-in table is compiled into the binary, and `agents/harness-defaults.yml` ships the same
table. Re-run `docket install` after editing.
