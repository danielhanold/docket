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
claimed_at: 2026-08-08T19:06:25Z
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

2026-08-08 — `docket-implement-next` halted at Step 5 (build) for the **second** time, on the
same root cause, now fully diagnosed. Steps 0-4 remain landed: the change is claimed and
reconciled, `feat/guard-the-config-suite-s-enumerated-claims-export-order-and` is cut from
`origin/main`, and the plan is committed on it at `7bd45872` with `plan:` recorded here. This
session **pushed that branch to origin** so the plan is no longer local-only. No build work was
performed; the branch still carries no test changes.

**What stopped the run.** The `docket-build-*` profile wrappers this session loaded still carry
the change-0079 cross-harness delegation body:

```
This agent is DELEGATED to the `opencode` runner (cross-harness runner delegation, change 0079).
… docket.sh runner-dispatch --runner opencode --agent build-standard \
  --model openrouter/deepseek/deepseek-v4-flash-0731 --effort high --worktree <feature worktree>
```

Verified this session by probing `docket-build-economy` and `docket-build-standard` directly:
both quoted that text back out of their own instructions. The wrapper additionally forbids the
inline fallback — "never run the skill inline on this harness as a fallback."

**Root cause (confirmed).** The wrappers are gitignored generated artifacts
(`.gitignore:9` → `.claude/agents/docket-*.md`). Those in the `docket.change-258` worktree — this
session's cwd, and therefore its project-scoped agent source — were **stale**; the primary tree's
`/Users/homer/dev/docket/.claude/agents/` copies had already been regenerated to the correct
inline form (`model: claude-opus-5`, no runner delegation). Project-scoped wrappers outrank the
user-level ones, so the stale copies won.

Current resolved config agrees the delegation is superseded: `.docket.local.yml` sets
`agent_harnesses: [claude,cursor]` (opencode is no longer a target for this repo) and pins
`agents.claude.build-*` to `claude-opus-5`. The baked `openrouter/deepseek/deepseek-v4-flash-0731`
model id appears in no config layer any more.

**Repair already applied.** This session copied the four correct `docket-build-*.md` wrappers from
the primary tree into `docket.change-258/.claude/agents/`. On disk they are now inline and correct.

**Why the run still halted.** Claude Code registers subagent definitions **only at process start**,
so the refreshed files are invisible to this session — re-probing `docket-build-standard` after the
copy still returned the opencode delegation body. And `skills.build` resolves to `docket-build`,
not `auto`, so the convention's Tier C (discipline dispatch) makes this abort-and-report, never
authorization to execute the plan inline. Routing every task to `docket-build-max` — the one
profile whose stale wrapper happened to be inline already — was rejected as a deliberate corruption
of the routing contract and of the reviewer-rung selection that keys off the build record.

Note for the record: `runners.opencode.permissions: auto-approve` **is** now set in
`~/.config/docket/config.yml` (it was absent at the first halt) and `opencode` 1.18.15 is on PATH,
so the original hard refusal is gone. Delegation was still not taken, because it would run the
build on a runner and model this repo's current configuration has removed.

**What a human must do.** Start a **fresh** session with cwd
`/Users/homer/dev/docket.change-258` and re-run `docket-implement-next 258`, telling it Steps 0-4
are landed and Step 5 has not started. The wrappers are already correct on disk; only the process
restart is missing. Optionally re-run `install.sh` / `sync-agents.sh` first so no other worktree
carries stale copies.

**State left behind (safe to resume).** `status: in-progress`, `claimed_at` refreshed. The
worktree `.worktrees/guard-the-config-suite-s-enumerated-claims-export-order-and`, its branch, and
the plan commit are preserved and now also on `origin`.
