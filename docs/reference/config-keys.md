# Config keys

Every config key, its shipped default, its meaning, and the layers it may be set in are documented
in one place: [`../../.docket.example.yml`](../../.docket.example.yml). That file is the shipped
reference (ADR-0048 makes it canonical), and the `docket-convention` skill's configuration section,
**"Configuration — `.docket.yml` (optional, committed on the default branch)"**
([`../../skills/docket-convention/SKILL.md`](../../skills/docket-convention/SKILL.md)), states the
resolution rules. This page copies no values and no defaults — read the current shape from the
example file, where a test keeps it honest against the actual schema.

## How to read a key's shape and scope

The example file carries, inline beside each key, its default value and a **scope tag**. The four
resolution layers, highest precedence first, are: repo-local `.docket.local.yml` (this machine,
gitignored), repo-committed `.docket.yml` (every clone), global
`${XDG_CONFIG_HOME:-~/.config}/docket/config.yml` (this machine, every repo), and docket's built-in
defaults. Nested blocks merge leaf by leaf, not whole-block. The scope tags are:

- **repo-only** — a coordination key (a config key whose value must be identical for every clone,
  so it may only be set in the committed repo config); a value set in either machine-scoped layer is
  warned-and-ignored (ADR-0019).
- **any layer** — behavioral only; per-machine divergence is benign.
- **local-only** — machine-specific tooling; a committed value is warned-and-ignored.

Read the exact tag and default for any key from the example file, not from here.

## Top-level blocks

One line per top-level block, enumerated from the example file. Each names the block's purpose; open
the example file for its keys, defaults, and per-block scope.

- **`runtime`** — the machine-local Bash-4+ interpreter path docket runs its shell with (local-only).
- **`metadata_branch`** — where planning metadata lives; selects docket-mode vs single-branch mode (repo-only).
- **`integration_branch`** — the branch code lands on, usually `main` (repo-only).
- **`changes_dir`, `adrs_dir`, `results_dir`** — where change files, ADRs, and results records live (repo-only).
- **`finalize`** — the terminal-half sequencer's knobs (test command, publish behavior).
- **`learnings`** — the learnings-ledger settings.
- **`reclaim`** — the stale-claim reclamation policy (the claim lease and its threshold).
- **`build`** — the build role's settings, including `build.test_command` (the build gate suite command).
- **`review`** — the review role's settings.
- **`gate_observation_budget`** — the per-observation slice budget for supervised gate runs.
- **`delegation_observation_budget`** — the observation budget for delegated runner sub-processes.
- **`board_surfaces`** — which derived board view(s) to render; `[]` disables the board entirely.
- **`board`** — board-rendering options.
- **`github_project`** — the GitHub Projects target when the board mirrors to GitHub.
- **`terminal_publish`** — opt-in publishing of terminal records to the integration branch.
- **`auto_groom`** — opt-in autonomous grooming of the needs-brainstorm queue.
- **`change_types`** — the allowed change-type taxonomy.
- **`auto_capture`** — the discovered-work capture policy.
- **`dummy_mode`** — the persona that shapes docket's generated prose and design conversations.
- **`runners`** — the harness runner pairings for delegated dispatch.
- **`skills`** — the workflow-role → skill bindings (the `skills:` map).
- **`agents`** — the model/effort-pinned agent wrapper definitions (presence-sensitive; ships commented).
- **`agent_harnesses`** — which harnesses the per-repo agent pass generates wrapper files for.

Blocks are added and renamed as docket evolves; the example file, not this list, is the authority
for what your binary accepts.
