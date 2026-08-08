---
id: 258
slug: guard-the-config-suite-s-enumerated-claims-export-order-and
title: 'Guard the config-suite''s enumerated claims: export order and rung pairs'
status: in-progress
priority: medium
type: chore
created: 2026-08-07
updated: 2026-08-08
depends_on: []
related: [251]
discovered_from: [123, 125]
adrs: []
spec: docs/superpowers/specs/2026-08-07-guard-the-config-suite-s-enumerated-claims-export-order-and-design.md
plan: docs/superpowers/plans/2026-08-08-guard-the-config-suite-s-enumerated-claims-export-order-and.md
results:
trivial: false
auto_groomable: true
branch: feat/guard-the-config-suite-s-enumerated-claims-export-order-and
claimed_at: 2026-08-08T19:12:00Z
pr:
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-07-guard-the-config-suite-s-enumerated-claims-export-order-and-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-07-guard-the-config-suite-s-enumerated-claims-export-order-and-design.md) |
| Plan | [2026-08-08-guard-the-config-suite-s-enumerated-claims-export-order-and.md](https://github.com/danielhanold/docket/blob/feat/guard-the-config-suite-s-enumerated-claims-export-order-and/docs/superpowers/plans/2026-08-08-guard-the-config-suite-s-enumerated-claims-export-order-and.md) |
<!-- docket:artifacts:end -->

## Why

Consolidates #0123 and #0125 (2026-08-07 triage): the same meta-question — an enumerated claim in the config-suite's contract surface is prose-only; guard the claim or delete it — asked about two adjacent claims. One posture ruling, two applications.

Verified 2026-08-07:

- **Export-list order unguarded (#0123).** The fenced export list at `scripts/docket-config.md:344-374` (32/33 entries) is pinned only by per-key *presence* greps (`test_docket_config.sh:1248,1429,1581,1954`) and two runtime-only adjacency clusters (`:1643-1650`, `:1943`). No test reads the fence block and compares its **sequence** to `--export --format plain` output — a doc-side reorder stays green while R7 pins runtime order for a few pairs.
- **Rung-pair completeness prose-only (#0125).** Section S pins all six ordered rung pairs (s4–s9), but the "six pairs" claim lives only in the section header comment; no derivation from the resolver's layer set exists, so a fourth config layer silently leaves six cells unpinned. The blocker named in the stub has resolved: 0114 landed ADR-0054 ("convert, do not close"), so source-shape anchors are a live option.

Posture ruled at grooming (2026-08-07, autonomous, critic-gated): **guard both claims** — neither is re-specified away. Design detail lives in the spec.

## What changes

- **Leg 1:** a whole-sequence equality guard — the doc's fenced export list (extracted at the `### Emit` heading anchor) vs the resolver's `--export` key emission, both formats (`plain` = fence; `shell` = fence minus `REPO_ROOT`), plus a derived check of the doc's "33/34 lines" prose numerals. Existing R7/AUTO_* adjacency asserts stay.
- **Leg 2:** the rung-pair completeness claim becomes computed — expected ordered pairs derived from `config_scalar_get`'s layer-dispatch arms (n·(n−1)); pinned pairs declared by per-fixture `RUNG_PAIR:` markers collected across the `test_docket_config*.sh` family glob; verdict is set equality. The header comment's prose enumeration stops being the claim of record.
- Both guards mutation-proved (reorder, removal, count-stable rename, and a simulated fourth layer must redden) and written corpus-indifferent to #0251's split (no `BASH_SOURCE` whole-file scans).

## Out of scope

- Adding config layers or keys; changing emission order.
- The population-floor/sharding rework of the same file — owned by #0251; coordinate at build time (same test file; whichever lands second rebases, per 0251's spec assumptions 7/9).

## Reconcile log

### 2026-08-08 — reconcile at claim (no scope change)

Re-verified every premise of the spec against `origin/main` @ `487bfdc5`; all hold, so the
design is carried forward unchanged.

- **Leg 1 doc side.** `scripts/docket-config.md` still opens `### Emit` with "printed as
  `KEY=value` lines to stdout in this order", followed by one fenced block of **34** entries
  (`DOCKET_MODE` … `BOOTSTRAP`), `REPO_ROOT` annotated `(plain format only — see below)`, and the
  prose sentence "33 lines in `shell` format; 34 in `plain` format". Counts match the spec's
  33/34; no doc edit has landed since grooming.
- **Leg 1 runtime side.** A live `docket.sh preflight` emits the fence's sequence exactly,
  `REPO_ROOT` in position 6. The claim is true today, which is what makes it worth pinning.
- **Leg 2 resolver side.** `config_scalar_get` (`scripts/docket-config.sh`) still dispatches
  exactly three layer arms — `committed` / `global` / `local`, each calling
  `config_scalar_from_lines … "${CONFIG_LINES_<LAYER>[@]}"` — plus the `*)` die arm. n = 3,
  n·(n−1) = 6, matching section (S4-S9)'s six fixtures.
- **Leg 2 pinned side.** `tests/test_docket_config.sh` section `(S4-S9)` (changes 0106 + 0112)
  carries fixtures s4–s9 and states the six-pair enumeration only in its header comment, as the
  spec describes. The six pairs are unchanged: s4 local→committed, s5 committed→global,
  s6 global→committed, s7 committed→local, s8 global→local, s9 local→global.
- **ADR-0054** ("Cross-references … anchor on symbols or quoted clauses, never line numbers") is
  `Accepted`, so the leg-2 source-shape anchor remains licensed.
- **#0251 coupling.** Still `proposed` / build-ready — the `test_docket_config*.sh` split has NOT
  landed, so the family glob currently resolves to the single file. The corpus-indifference
  constraint (no `BASH_SOURCE` whole-file scans; iterate the glob) is honored regardless, per
  spec assumption 8; whichever change lands second rebases. No `depends_on` added.
- **Auto-capture (site A).** Nothing minted: the pass surfaced no work outside this change's
  scope that clears the six admission gates.

## Run halted

2026-08-08 — `docket-implement-next` halted at Step 5 (build). Steps 0-4 completed and are
landed: the change is claimed, reconciled, `feat/guard-the-config-suite-s-enumerated-claims-export-order-and`
is cut from `origin/main`, and the plan is committed on it at `7bd45872` with `plan:` recorded
here. No build work was performed and the feature branch carries no test changes.

**What stopped the run.** Every `docket-build` profile dispatch from this session routes to the
`opencode` runner and dies before the worker starts:

```
runners/opencode: runners.opencode.permissions is 'ask' (the default) — a delegated run cannot
answer opencode's approval prompts and would hang. Set 'runners.opencode.permissions:
auto-approve' ... or drop 'runner: opencode' from this agent.
```

**Root cause.** The session's working directory is the `docket.change-258` worktree, and its
project-scoped `.claude/agents/docket-build-standard.md` is a stale cross-harness runner-delegation
wrapper (change 0079) pinning `model: openrouter/deepseek/deepseek-v4-flash-0731` and delegating to
`runner: opencode`. Project-scoped wrappers outrank the user-level ones, so the correct wrapper at
`~/.claude/agents/docket-build-standard.md` (`model: claude-opus-5`) is never reached. All four
`docket-build-*` profiles carry the same stale wrapper in that worktree and in the primary tree's
`.claude/agents/`. The first dispatch attempt surfaced the same broken model id as a bare harness
error ("openrouter/deepseek/deepseek-v4-flash-0731 ... may not exist or you may not have access").

Neither `runners:` nor an `opencode:` agents block exists in `~/.config/docket/config.yml`, though
`agent_harnesses` there lists `[claude,codex,opencode]`.

**Why the run did not build inline instead.** `skills.build` resolves to `docket-build`, not `auto`,
so per the convention's Tier C (discipline dispatch) this is abort-and-report, not authorization to
execute the plan inline. The operator also directed the run to stop rather than use runner
delegation.

**What a human must decide.**

1. Whether the `docket-build-*` profile wrappers in `<worktree>/.claude/agents/` should be
   regenerated without `runner: opencode` (re-run `install.sh` / `sync-agents.sh`, then start a
   fresh session — wrappers register only at process start), or
2. whether opencode delegation is wanted, in which case `runners.opencode.permissions:
   auto-approve` must be set, and the `openrouter/deepseek/deepseek-v4-flash-0731` model id
   verified as reachable.

Configuration changes are the human's call; this run made none.

**State left behind (safe to resume).** `status: in-progress`, `claimed_at` refreshed. The worktree
`.worktrees/guard-the-config-suite-s-enumerated-claims-export-order-and` and its branch are
preserved with the plan commit. A resume should be given the explicit id (`258`) and told that
Steps 0-4 are done and Step 5 has not started.

