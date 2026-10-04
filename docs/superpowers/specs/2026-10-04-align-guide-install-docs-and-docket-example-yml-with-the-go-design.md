<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0464 — Align the human-facing docs and example config with the docket binary](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-04-0464-align-guide-install-docs-and-docket-example-yml-with-the-go.md)**
<!-- docket:backlink:end -->

# Align the human-facing docs and example config with the docket binary — design

## Goal

A person who reads docket's human-facing documentation — the README, the guide, the install pages,
the concept pages, the reference — and copies its examples ends up with a configuration the binary
honours and commands that exist. Today that is not true: copying `.docket.example.yml` verbatim
blocks every repository write, whole pages teach features the binary removed or refuses, commands
and health checks that no longer exist are named throughout, and nothing guards the docs, so the
drift accumulated silently.

The outcome: every in-scope page describes only what the binary does **today**; removed and
refused behaviour is gone without a trace; no page cites an individual change; the example config
is safe to copy; and one repo test stops the two worst kinds of drift from coming back.

## Alignment rules (apply to every in-scope page)

1. **Describe only current behaviour.** The binary and its typed operations are the oracle
   (`internal/config/schema.go` + `capability.go` for configuration, `docket capabilities --json`
   and each verb's `--help` for commands, the Go code for mechanisms).
2. **Removed or refused things never appear.** A setting the schema marks obsolete, inert, or
   refused (declaring it blocks writes), a retired feature, a deleted command, flag, script, or
   check, and the old name of a renamed term are deleted — no "this used to…", no tombstone
   paragraph, no "not supported, remove it" note, no lookup table of refused keys. The binary's own
   diagnostic names the key and the remedy when someone has an old config.
3. **No citations of individual changes or PRs** ("change 0363", "changes 0064/0084", "pre-0051",
   "since 0392", "PR #344", "#0471").
4. **ADR citations are allowed** where the ADR's decision is still how docket works. A citation of
   an ADR whose decision records removed or refused behaviour goes (see *Concepts*). A relative ADR
   link must resolve on the integration branch (`docs/adrs/` there holds ADR-0001…0096); a higher
   ADR is cited as plain text or not at all.
5. **Point-in-time records are untouched:** archived changes, results files, ADRs, specs and plans,
   `docs/comparison/` (dated comparison), `docs/codex/fixtures/` (dated certification).
6. **Plain register.** No internal jargon such as "Go v1"; no Bash-era vocabulary.

### Terminology decided at grooming

- **"The daily loop" → "the five steps".** `docs/guide/daily-loop.md` is renamed
  `docs/guide/five-steps.md`, titled **"Using docket: the five steps"** (capture, groom when
  needed, build, review and merge, close out). The README heading "Install and the daily loop"
  becomes "Install and the five steps"; every inbound link and mention is updated (README,
  `docs/README.md`, `docs/guide/README.md`). "Daily" is dropped: nothing is tied to a day.
- **"Loop" is reserved for `/loop`** (the harness command that re-runs a workflow until its queue
  drains). Every other sense is renamed in the in-scope docs:
  - the autonomous builder ("the loop", "the autonomous loop", "hand work to the loop") →
    **implement-next** / **a build run** (one run builds one change);
  - "fix loop" → **fix pass** (it runs once: findings become fix tasks, then one full-suite run);
    the glossary entry becomes *Fix pass*. The skill-side rename (the
    `docket-implement-next/references/fix-loop.md` file and its wording) belongs to the skills
    change (#502), which also retargets any doc link to that file;
  - "build loop" / "build-loop memory" → plain **builds** / **the learnings ledger**;
  - "both halves of the daily loop" (landing-changes) → **both drains** (the implement-next drain
    and the finalize drain).

## Scope

**In scope:** `README.md`; `docs/README.md`; `docs/guide/`; `docs/install/`; `docs/concepts/`;
`docs/reference/` (including the glossary and `docs/reference/harness/`);
`docs/release/four-harness-acceptance.md` (fix-in-place only); `.docket.example.yml` and its test;
the comments in `agents/harness-defaults.yml` (with the fixture re-cut that requires); the dead
`scripts/runners/` files; the `docket run` short help string; one new docs guard; every repo test
that pins wording on these files.

**Not in scope:** the skills, skill references and templates, the agent wrappers, and `AGENTS.md` —
all of that is the separate skills change (#502), which depends on this one. Changing
what the binary does — including deleting the obsolete/inert/refused rows from the config schema —
is not part of this change.

## Worklist

The audit behind this spec (six read-only passes over main @ `20bc0a36a`, 2026-10-04) produced
the findings below. **Treat each as a hypothesis to verify against the code at build time, not as
an oracle**: re-check the claim, then fix, delete, or record in the results file why it was not a
defect. Each bullet names the page section (or a quoted phrase) and the correct fact. After the
listed fixes, grep each page for further instances of the same defect — the audit quoted examples,
not every occurrence.

### Facts the rewrite relies on (verify once, use everywhere)

- **Configuration.** Supported keys: `integration_branch`, `changes_dir`, `adrs_dir`,
  `results_dir` (the four shared-setting-guarded, repository-only keys); `finalize.gate`
  (`local` | `off`), `finalize.test_command`, `finalize.require_pr_approval`,
  `finalize.resolver_max_attempts` (10), `finalize.repair_max_attempts` (6); `build.gate`
  (`local` | `off`), `build.test_command`, `build.max_attempts` (4); `run.max_attempts` (2);
  `review.min_fix_severity` (`minor`), `review.max_fix_tasks` (10); `reclaim.lease_ttl` (72),
  `reclaim.auto` (false); `learnings.enabled` (true); `gate_observation_budget` (30);
  `board_surfaces` (`[inline]`); `board.section_order`, `board.sorting.<section>.by|direction`;
  `change_types`; `agent_harnesses`; `agents.<harness>.<agent>.model|effort` **from the global
  config only**. Everything else in the schema is obsolete, inert, or refused and is not mentioned.
- **Layers.** Global `${XDG_CONFIG_HOME:-$HOME/.config}/docket/config.yml`, committed `.docket.yml`,
  machine-local `.docket.local.yml`, built-in defaults; precedence local > committed > global >
  built-in. Agent model/effort overrides are honoured from the global file only. A malformed file,
  an unknown key, or a bad value in **any** layer makes the whole configuration invalid (no
  per-layer fallback); an unreadable file is a load error. `docket diagnostic config --repo-dir .`
  shows the resolved configuration and anything that blocks writes. Nothing writes or maintains the
  global file for you; there is no `agents.yaml` migration.
- **`agent_harnesses`** has no default: absent means install touches no repository surface. Only a
  `.docket.yml` / `.docket.local.yml` declaration authorizes repository surfaces; a global value is
  ignored by the installer; an unknown token is an error. It selects only the parent-facing
  dispatch surfaces: the managed block in `CLAUDE.md` / `AGENTS.md` (with a `CLAUDE.md` →
  `AGENTS.md` symlink when Codex or OpenCode is also opted in) and `.cursor/rules/docket-dispatch.mdc`.
- **Agent wrappers** are user-level only: `~/.claude/agents/`, `~/.codex/agents/`,
  `~/.cursor/agents/`, `${XDG_CONFIG_HOME:-~/.config}/opencode/agents/` — 17 agents per harness.
  No per-repository wrapper files exist. The built-in model/effort table is compiled into the
  binary; `agents/harness-defaults.yml` is its shipped mirror.
- **Install.** `install.sh` is a bootstrapper that forwards only `--bin-dir` and `--harness` to
  `docket development install --source <dir>`. `docket install check` checks this machine's
  installation only; it is not a repository CI gate. The `.gitignore` managed block is written by
  `docket repository init` / `migrate` and checked by `docket repository check`.
- **Repository setup.** A repository that never used docket runs `docket repository init`;
  `docket repository migrate` converts only a legacy single-branch layout; `check` reports health;
  `repair` previews a fix and applies it only with a human's `--yes`; `configure-tests` sets the
  test commands. There is one metadata layout: the `docket` branch through the `.docket/` worktree.
- **Status and maintenance.** `docket status` is read-only. Sweeping merged changes to done,
  cleanup retries, and lease reclaim are `docket maintenance sweep` (the status skill runs it on an
  explicit refresh; implement-next runs `docket maintenance preflight`). With `reclaim.auto: false`
  an eligible change is reported as skipped (`reclaim-auto-disabled`); reclaim requires an expired
  lease, no feature branch, and no workspace. Status health findings are structural
  (`artifact-missing`, `change-reference-dangling`, `change-dependency-cycle`, `branch-malformed`,
  configuration and parse diagnostics); there is no stale-claim or stalled-dependency check. A
  halted change stays `in-progress`; recovery is `docket change resume-halted` and
  `docket run start implement-next --resume <id>`.
- **Lifecycle.** A dependency is satisfied only at `done`. `done` is written by
  `docket finalize closeout` after proving the merge (the maintenance sweep is the safety net). A
  stacked change's base is its parent's branch while the parent is live, the integration branch once
  the parent is `done`. Statuses include `blocked`, `deferred`, `killed`, `stacked-merged`.
  Branches are `<type>/<slug>` or `<branch_prefix>/<slug>`.
- **Finalize.** rebase onto the change's effective base (conflict → resolver) → retest (red →
  repair, capped by `finalize.repair_max_attempts`) → `finalize publish` (push the rebased head,
  update the PR's build-evidence block) → merge (first method the repository permits: rebase, merge
  commit, squash) → closeout (archive on the `docket` branch only; retarget backlinks) → cleanup.
  Stacked children are retargeted (`finalize retarget-children`). An autonomously authored repair
  halts for human sign-off (`finalize block --reason repair-needs-signoff`, cleared with
  `finalize clear-block`). `finalize.gate: off` skips rebase and retest; a no-op rebase with
  exact-head green evidence and the same command skips the suite. A failed backlink retarget is
  `final-backlink-pending`, repaired by `finalize cleanup`. Nothing is copied to the integration
  branch at close-out.
- **Build gate and evidence.** The build gate runs `build.test_command`; red runs become repair
  tasks (premium → max → halt) until `build.max_attempts` is spent. `build.gate: off` runs nothing
  and records `skipped` / `build-gate-off` evidence; an empty `build.test_command` under `local`
  halts with the remedy `docket repository configure-tests`. Build evidence is an immutable record
  minted by `docket evidence record` from a passed run and checked by `docket evidence verify`; it
  lives in the PR body's build-evidence block and is never committed. Per-test-file wall-clock
  budgets (`BUDGET WATCH`, `SERIAL CONFIRMED OVER BUDGET`) belong to docket's own contributor suite
  (`docket development test`), not to the build gate in a user's repository.
- **Review.** The review tier maps same-level from the build tier (economy → lean, standard →
  standard, premium/max → deep) with a bump for very large diffs. Out-of-branch follow-up work is
  reported in the run's final report for a human to capture; nothing is minted automatically.
- **Run tracker.** `docket run start` writes the run record and launches nothing; the parent
  dispatches. `docket run verdict` always exits 0 and prints one decision line: `run-done`,
  `run-retry-once`, `run-stop` (a halt is `run-stop … run-halted`), `run-continue` (nonterminal;
  also authorizes re-dispatch), or `run-observe` (unattributed). `run start --resume` refuses with
  `resume-active-run`, `cancellation-pending`, or returns the reserved replacement
  (`resume-replacement-reserved`); `run cancel` reports `cancelled`, `cancellation-pending`,
  `already-cancelled`, or `refused`. Gate-run exit codes are 0/1/2 and a halted gate run exits like
  a failed one.
- **Learnings.** `docket learning record` / `docket learning update` (JSON request) write findings;
  `learnings.enabled: false` refuses them and turns reads off. Nothing harvests, promotes,
  re-indexes, or counts findings automatically; the index file is never regenerated.
- **Outcomes vocabulary.** Every result envelope carries `result` (`applied`, `no-op`, `contended`,
  `invalid-input`, `invalid-state`, `blocked`, `unsupported-config`, `gate-failed`,
  `external-failed`, `interrupted`, `internal-error`); each operation has its own disposition
  vocabulary, listed by `docket schema`. `applied` / `no-op` / `refused` / `error` is
  `repository prepare`'s disposition set only.

### Front door

- `README.md`: "no CLI to install" → docket ships a `docket` binary that `install.sh` installs;
  adopting docket in a repository is `docket repository init` (not `migrate`); the *Status* section
  is rewritten to state what is supported today — no main-mode opt-out, no list of deferred
  features, no "Go v1"; heading "Install and the daily loop" → "Install and the five steps".
- `docs/README.md`: drop the *Workflow roles* and *Delegation* entries; daily-loop links and
  wording → five steps.

### docs/guide

- `daily-loop.md` → `five-steps.md` ("Using docket: the five steps"); the closing "health checks
  that flag stale claims … stalled dependencies" → the real findings; "the loop grooms, builds, and
  closes out" → "docket grooms, builds, and closes out".
- `capturing-work.md`: priorities are critical / high / medium / low; the board is grouped into
  in-progress, built, blocked, groomed, proposed, deferred (configurable with `board.section_order`
  / `board.sorting`) and is re-rendered by every write — no regenerate step; delete the
  auto-capture paragraph and YAML, the `auto_capture.types` claims, the "query pseudo-values"
  claim, the `--type untyped` / `--type all` / unconfigured-type claims (all are refused), and the
  whole *Migrating to typed changes* section; capture is the `docket-new-change` skill
  (`docket change create` needs `--request <file>`).
- `designing-before-building.md`: the consultant-written spec is a spoken per-run request only
  (delete the durable `skills:` config); "agents not synced" → installed by `docket install`; delete
  the whole *Shaping the conversation (dummy_mode)* section including the persona gallery; delete
  the line-oriented-config-reader claim.
- `building-without-supervision.md`: docket requires its own `docket` binary; build and review
  default to docket's own skills; rename "the loop" senses (Terminology); `/loop` drain wording
  stays.
- `proving-the-build.md`: red-suite handling and `build.max_attempts` (Facts); delete the
  `build.checkpoint` bullet and the `ci` / `both` gate values; delete the per-file budget prose
  (it describes docket's own contributor suite, which `tests/README.md` covers) and "in this
  repository both resolve…".
- `reviewing-before-the-human.md`: delete the `skills: review:` opt-out; tier mapping is same-level
  (Facts); delete "restores the older … escape hatch"; follow-ups go to the final report.
- `landing-changes.md`: delete *Selective publish on close-out* and the retired bot-approval
  paragraph; merge method order (Facts); add the repair sign-off block to *When finalize is
  blocked*; "both halves of the daily loop" → "both drains".
- `keeping-the-backlog-honest.md`: rewrite around read-only `docket status` +
  `docket maintenance sweep`, the real health findings, reclaim rules, and halted-run recovery
  (Facts); delete *Supported modes and the bootstrap guard*'s mode framing; a new repository runs
  `docket repository init`.
- `remembering-why.md`: learnings are written with `docket learning record` / `update`; delete
  close-out harvest, generated index, curation cap, and promotion proposals.
- `where-the-metadata-lives.md`: rewrite around the one layout and `repository init` / `migrate` /
  `check` / `repair` / `configure-tests`; delete every terminal-publish passage and table row,
  *Publishing archived records*, *Single-branch mode: the opt-out*, the allow-rule remnant, and
  *Carrying a pre-0051 repo forward*; the `.gitignore` managed block is written by init/migrate;
  migrate uses no worktree.

### docs/install

- `README.md`: drop *Workflow roles* and *Delegation* entries and the opencode "auto-approve grant"
  wording; fix the global-config bullet (it does not enable a harness).
- `install.md`: harness roots are the four supported harnesses (no `.kiro` / `.windsurf`); default
  roles (superpowers for brainstorm/plan/finish; docket's own for build/review); `install.sh` flags
  are `--bin-dir` / `--harness`; nothing writes `config.yml`; enabling a harness for a repository is
  `agent_harnesses` in `.docket.yml`; delete the stale project-level-wrapper callout; *Adopting
  docket in a repository* → `docket repository init` (legacy single-branch repositories: `migrate`).
- `keeping-current.md`: delete the managed-global-config bullet; refresh the example tag.
- `global-config.md`: what belongs there (agent model/effort pins and other global-able keys);
  delete the `skills:` default binding, the installer-writes-config claims, the global
  `agent_harnesses` instructions, and the `agents.yaml` migration.
- `config-layers.md`: layers and precedence (Facts); delete the `skills:` / repo-layer `agents:`
  merge prose, the `runtime.bash` paragraph, `metadata_branch` and the mode opt-out, and *Migrating
  from agents.yaml*; the shared-setting guard covers the four repository keys; malformed-file
  behaviour (Facts); the `.gitignore` block is written by init/migrate; point to
  `docket diagnostic config`.
- `workflow-roles.md`: **delete the page.** Its default-roles table (role → default skill) moves to
  `docs/reference/skills-and-agents.md`. Fix inbound links.
- `models-and-effort.md`: pins live in the global config only; `docket development install
  --source <dir>`; wrapper roots (Facts; opencode under `${XDG_CONFIG_HOME:-~/.config}`); delete
  per-repository wrapper generation, "Generated per-repo agent files…", the retired clone-identical
  note, and the `install check` CI-gate claims.
- `delegating-across-harnesses.md`: **delete the page.** Fix inbound links (`docs/README.md`,
  `docs/install/README.md`, `opencode.md`).
- `claude-code.md`: add what install writes for Claude (user-level skills and agents; with
  `agent_harnesses` including `claude`, the managed dispatch block in `CLAUDE.md`).
- `codex.md`: wrappers are user-level only; pins are global-only; fix the opt-in table and gotcha
  (Facts); delete the upgrade note, the `skills:` binding remark, "delegation prose", "install or
  sync". **Do not rewrap** the paragraph whose three sentences `TestCodexLaunchMatrixOperatorProse`
  pins with their line breaks. Drop the link to the deleted Codex runbook.
- `cursor.md`: delete "delegation target", the allowlist bullets about status publishing, terminal
  publish, the GitHub mirror, and `cleanup-feature-branch`; bare `preflight` →
  `docket repository prepare` / `docket maintenance preflight`; `docket board-refresh` /
  `docket env` → real commands; correct the "no run operation" remark (`docket run` is the run
  tracker, not an exec); keep the inline `{"terminalAllowlist": ["docket"]}` fragment, delete the
  pointer to the example JSON copies and the sandbox read-path fragment; add what install writes for
  Cursor.
- `opencode.md`: wrapper and skills roots under `${XDG_CONFIG_HOME:-~/.config}/opencode/`; 17
  agents; global-only pins; the effort drop is silent; delete *Delegating Claude Code agents to
  opencode* and its permissions table.

### docs/concepts

- **`## Decided in` sections stay.** Drop a bullet when its ADR's decision is removed or refused
  behaviour — at least ADR-0002 (superseded), ADR-0005 (close-out harvest), ADR-0018 (pluggable
  skill rebinding), ADR-0045 (auto-capture), ADR-0046 (destructive reset precondition), ADR-0051
  and ADR-0090 (publish-deferred marker), ADR-0080 (detached delegation) — and judge every other
  bullet the same way; reword a kept bullet that describes the ADR in retired terms. The README's
  four-section promise stays.
- `README.md`: the finalize summary includes publish and cleanup.
- `build-tiers-and-gate.md`: evidence is a PR-body record, never committed; per-file budgets are not
  the build gate's; add `build.gate: off` and the empty-command halt.
- `change-lifecycle.md`: how `done` is reached; dependencies satisfied at `done`; stacked base
  rule; delete best-effort follow-up capture; the diagram includes `blocked`, `deferred`, `killed`,
  `stacked-merged`.
- `config-layers.md`: delete the bash and `metadata_branch` rationale and the "external objects"
  invariant; model pins global-only; malformed layers invalidate; global path; built-in table is
  compiled in; add how to inspect the resolved configuration (`docket diagnostic config`).
- `finalize-sequencer.md`: redraw the sequence (Facts), including publish, retarget-children, repair
  sign-off, and `finalize.gate: off`; delete archive-to-integration-branch and `## Publish deferred`.
- `memory.md`: learnings are written by `docket learning record` / `update`; delete close-out
  harvest; scope the comment-anchor invariant to docket's own source (or drop it).
- `reconcile.md`: only the `Decided in` judgement.
- `run-tracker.md`: delete "launches, then observes" and history framing ("no longer", "as
  before"); verdict lines and exit codes (Facts); `run-continue` also re-dispatches; add resume
  refusals, cancel dispositions, `run-untracked`, and `run verify`.
- `skills-agents-dispatch.md`: skills are fixed default roles; wrappers come from the built-in
  table plus the global config; the dispatch return is not the only channel (ADR, status and
  plan-writer results arrive as git state); fix the fallback description and the precedence diagram.
- `two-branches.md`: delete archived-record copying, "Docket-mode is the default", and the
  destructive-reset invariant; `.docket/` paths follow `changes_dir` / `adrs_dir`; a fresh
  repository is refused with `docket repository init`.

### docs/reference

- `cli.md`: add `docket agent`; `docket repository` also prepares, repairs, configures tests, and
  syncs the integration branch; `docket run` is not read-only; fix the "never lists flags" intro or
  the flag mentions.
- `config-keys.md`: supported keys only (Facts), including the missing `run` block; delete the
  `local-only` scope tag and every non-supported row; `agent_harnesses` described as the dispatch
  opt-in; no change citations.
- `outcomes.md`: the result and disposition vocabulary (Facts).
- `skills-and-agents.md`: gains the default-roles table from `workflow-roles.md`.
- `glossary.md`: delete the *Obsolete terms* section, its TOC item, and its index lines; delete the
  tombstone entries (*Docket-mode / single-branch mode*, *GitHub board mirror / github_project*, the
  `launch-unconfirmed` tail of *worktree-busy*) and old-name index aliases (*Change version /
  entity version*, *Selective publish*); remove the change citations; rewrite or delete every entry
  that presents a refused or inert setting (*Auto-groom* repo knob, *Workflow role* rebinding, the
  `skills.*` mentions under *docket-build* / *docket-review* / *docket-brainstorm*, *Dispatch
  fallbacks*' explicit-`auto` path, *Observation budget*'s delegation half, *Finalize gate*'s
  `ci`/`both`, *Auto-capture*, *build.checkpoint*, *Dummy mode*, *finalize.skip_results_only_delta*,
  *learnings.cap*, the `local-only` scope tag, *Reserved type tokens*' scalar remark, *Learnings
  index*'s deferral note); correct *Feature branch* (status records carry no branch/PR),
  *Finding / finding code*, *Result / disposition*, *Install check*, *Managed global config* /
  *Keeping docket current*, *Tri-state verdict / halt exit code*, *Marker section*, *docket-status*,
  *Worktree / feature workspace* ("identity check" → worktree-changed), *Diagnostic runtime*; rename
  *Fix loop* → *Fix pass*.
- `harness/`: **delete** `validation-runbook.md` (Codex; Bash-era, and its core setup step blocks
  every write), `fixtures/nested-launch/`, `permissions.example.json`, and `sandbox.example.json`;
  update `harness/README.md` to list what remains. Fix `validation.md` (Cursor): `sync-agents.sh` →
  `docket install --harness cursor`; delete the nonexistent test path; 17 wrappers; the default
  build role is always `docket-build` (Phase 7 always applies); correct the pinned-model exception
  sentence; Phase 6 describes docket-build's tiers, not subagent-driven development.

### docs/release/four-harness-acceptance.md (fix in place; excluded from the guard)

Remove the change citations and the "change 0317 checklist" framing; `docket install claude` →
`docket install --harness claude`; delete the runner shim / runner fallback wording.

## `.docket.example.yml`

- **Supported keys only**, each with a short plain comment stating what it does and its default,
  grouped as today. Add the supported keys it lacks (`run.max_attempts`, the finalize attempt caps,
  `reclaim`, `review`, `board` ordering where missing).
- **Safe to copy:** active lines carry built-in defaults; keys without a built-in default
  (`agent_harnesses`) and global-only keys (`agents:` model/effort, tagged as global-config-only)
  are commented. A verbatim copy as `.docket.yml` passes the mutation preflight and resolves to the
  built-in defaults.
- Rewrite the header (no deleted test script names, no leaf-merge list naming non-supported
  blocks, correct repository-only count, correct scope tags — `local-only` goes, a global-only tag
  is added). Delete every change citation and every Bash-era remark (line-oriented reader, in-map
  `#` truncation, `docket install check` as a CI gate, per-repository wrapper files).
- **Example-file test** (`TestExampleSchemaCorrespondence`) changes to: every **supported** schema
  path is documented (dynamic families via an ancestor, as today); **no** non-supported path
  (obsolete, inert, inert-companion, deferred, deferred-active — derived from the registry's
  dispositions, never hand-listed) appears as an active or tagged key; a verbatim copy passes the
  mutation preflight. Update the non-vacuity and population lists (e.g. `skills.build`,
  `runners.codex.shim_model`, the `runtime` opener) and recompute the floors; mutation-test each
  new assert (plant a refused key → red; strip a supported key → red; plant a blocking active line
  → red).
- Regenerate the embedded copy (`go generate ./internal/assets/`); keep the `TestProseContracts`
  row for the example file true.

## `agents/harness-defaults.yml` comments

Rewrite the header comments: this file is the shipped model/effort table that the binary's
compiled built-in table mirrors; only the global configuration overrides it; delete every reference
to deleted Bash tooling (`scripts/lib/harness-defaults.sh`, `sync-agents.sh`,
`HD_SHIPPED_HARNESSES`, the Bash reader and validator), the `runner:` remark, and change citations
(ADR citations may stay if still accurate). Values are unchanged. `TestBuiltinAgentsParityWithFrozenSidecar`
byte-pins the file to a frozen fixture: cut a **new** versioned fixture tree and repoint the test —
never edit the frozen copy — and update the absence-test header that records the old comment as
known-stale. Regenerate the embedded copy.

## Dead scripts

Delete `scripts/runners/{codex,cursor,opencode}.{sh,md}` (nothing invokes them; delegation is
retired). Lower `TestScriptContractsCoverage`'s population floor and fix its comment premise. The
generated-output ban lists that forbid runner-era tokens keep passing.

## `docket run` help text

The short help "(read-only)" is wrong (`run start`, `run cancel`, `run continue`, `run verdict`
write local state). Correct it and anything that pins it.

## New docs guard

One repo test (in `internal/repoguard`) over an explicit list of living doc roots: `README.md`,
`docs/README.md`, `docs/guide/`, `docs/install/`, `docs/concepts/`, `docs/reference/` (excluding
fixture directories). `docs/release/` is deliberately excluded (dated release evidence lands
there).

1. **No change or PR citations.** Fails on the citation shapes observed in the audit — "change N",
   "changes N/M", "change-N", "pre-NNNN", "since NNNN", "PR #N", "#NNNN" — keyed on shape, with no
   hand-maintained exception list. ADR citations are allowed. Docs that need an example id use a
   placeholder (`<id>`) or a code span the guard does not treat as a citation; decide the exact
   matcher in the plan and state its limits in the test header.
2. **No non-supported config keys.** Fails when a key the schema marks obsolete, inert,
   inert-companion, deferred, or deferred-active (plus `agents.*.*.runner`) appears spelled as a
   config key — a dotted path (`skills.build`, `learnings.cap`) or a YAML key (`terminal_publish:`,
   `skills:`) — derived from the schema registry at test time, word-bounded (`auto_groomable` is a
   current per-change field and must pass). Refused *values* of supported keys (`finalize.gate:
   ci`, the `github` board token) are out of the guard's reach and left to review.
3. Mutation-tested: planting each citation shape and a registry-derived key reddens it; a
   population floor on scanned files keeps a shrunken scope from passing vacuously.

## Tests that pin wording on these files

Update each pinned phrase in the same commit as the prose it guards; remove rows whose target page
is deleted. Known: `TestProseContracts` rows (example file; `models-and-effort.md`
"completed (forked execution)", "Fork-exclusion principle"; `cursor.md`
"permissions.example.json" and the harness-reference link; `harness/validation.md`
"## The PR-handoff obligation"; the Codex runbook rows), `TestCodexLaunchMatrixOperatorProse`
(pins `codex.md` sentences with line breaks; its runbook row goes),
`TestUninstallCollectionDocContracts` and `TestInstallPrerequisiteDocContracts` (`install.md`
headings and clauses), `TestEmbeddedMatchesAuthored`, `TestBuiltinAgentsParityWithFrozenSidecar`,
`TestScriptContractsCoverage`. Grep `internal/repoguard` and `tests/` for every edited path before
editing — this list is not exhaustive.

## Acceptance

- Every worklist item is fixed, deleted, or recorded in the results file as not-a-defect with the
  evidence.
- The new docs guard and the reworked example-file test are green and their mutations redden.
- `.docket.example.yml` copied verbatim into a scratch repository passes
  `docket diagnostic config --for-mutation`.
- Every relative link in the in-scope docs resolves (one-off check at build time, recorded in the
  results file); no inbound link points at a deleted or renamed page.
- The whole suite (`build.test_command`) is green.
