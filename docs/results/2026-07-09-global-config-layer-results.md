# Results — Global config layer (change 0050)

**Change:** #0050 · PR: _(opened by this build)_ · Spec: `docs/superpowers/specs/2026-07-09-global-config-layer-design.md` · Plan: `docs/superpowers/plans/2026-07-09-global-config-layer.md` · ADR: **ADR-0019**

Built autonomously via `docket-implement-next` (SDD: 6 plan tasks, per-task spec+quality review, whole-branch final review at the session tier). Build-receipt detail (files, full test output) lives in the PR description; this file records what the merge-gate reviewer should know beyond "green CI."

## Merge-gate notes

- **Backward compatibility was byte-verified, not just asserted:** the final reviewer diffed base-vs-head `docket-config.sh --export` output (stdout and stderr) and `sync-agents.sh` harness trees with **no global file present** — byte-identical in both cases. A machine without `~/.config/docket/config.yml` is untouched by this change.
- **The migration is destructive-by-design on `agents.yaml`** (renamed to `agents.yaml.migrated` after its content is rewritten under `agents:` in `config.yml`). The append-before-rename ordering makes an interrupted run recoverable (stale-twin warning, no data loss). Your real `~/.config/docket/agents.yaml` will migrate on your next `install.sh`/`sync-agents.sh` run — expected and logged loudly.
- **Optional post-merge sanity check:** drop a `config.yml` with `skills.build: auto` plus a fenced key (e.g. `metadata_branch: main`) into `~/.config/docket/`, run any docket skill's Step 0, and confirm the global-able key applies while the fenced key produces the per-repo-only warning and no behavior change. (The build's real-data smoke did exactly this against this repo — both held.)

## Findings

- **ADR-0019** recorded (on `origin/docket`; rides this change's terminal-publish onto `main` at merge): the coordination-key fence classification rule — fenced = writes shared, non-re-derivable state (shared-branch commits, committed generated files, external GitHub objects); global-able = confined to the local run (self-healing derived views, per-machine uncommitted files).
- **Suite-mutates-user-data hazard caught by the final review** (fixed pre-merge, commit `585b5ae`): `tests/test_install.sh` inherited `XDG_CONFIG_HOME`, so on machines exporting it (common on Linux) the new auto-migration would have **rewritten the developer's real global config as a test side effect**. Both install/settings suites now pin XDG hermetically. Lesson: a feature that adds a write path to a shared location upgrades every non-hermetic test that reaches it from read-leak to write hazard.
- Two mid-build behavior additions beyond the spec's letter, both reviewed and endorsed: a trailing-newline guard in the migration append (a `config.yml` without a final newline would have glued `agents:` onto its last line), and **user-level de-list pruning** — narrowing/emptying the global `agent_harnesses` list prunes docket-owned wrappers from de-listed harnesses (mirrors the per-repo de-list rule; never rmdirs the harness root; pinned by fixtures).
- Final whole-branch review verdict: **Ready to merge — with fixes**, all of which landed (`585b5ae`): the XDG pins and a prune-on-`[]` pinning assert.

## Follow-ups (candidates, none blocking)

- **#0051 filed (live-test finding, 2026-07-09):** in a repo opted into per-repo generation, the global `agents:` block is fully shadowed — the committed full set (change 0048) resolves from `.docket.yml` + built-ins only and takes harness precedence over the user-level wrappers carrying the global models. A loud causal warning was added to this PR as a stopgap; #0051 decides the real semantics (restore pre-0048 per-agent fall-through, a seed command, or docs-only).

- **Pre-existing bash-3.2 hazard** (predates this branch, confirmed at base): `prune_orphans`' `for dir in "${scan_dirs[@]}"` errors under macOS `/bin/bash` 3.2 + `set -u` when the array is empty (no harness roots on disk AND repo not opted in). Guard: `[ ${#scan_dirs[@]} -gt 0 ]`. Fine rider on the next sync-agents change.
- Migration polish: a directory at `config.yml` plus a live `agents.yaml` aborts `sync-agents.sh` with a raw redirection error (fail-loud but unprefixed — `docket-config.sh` handles the same state gracefully); `mv` would silently clobber a pre-existing `agents.yaml.migrated` (only reachable by recreating `agents.yaml` after a prior migration).
- Cosmetic: an unknown `skills:` role present in both layers warns twice; one fence-loop printf is double-quoted where siblings are single-quoted.
- `docket-config.sh`'s `SKILLS_BLK`/`GSKILLS_BLK` mktemps still rely on explicit `rm -f` rather than the EXIT trap (pre-existing 0049 pattern; no `die` path between creation and cleanup today).
