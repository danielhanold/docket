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
claimed_at: 2026-08-08T19:27:09Z
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

### 2026-08-08 — resumed at Step 5 (no scope change)

`origin/main` is still `487bfdc5`, unchanged since the pass above, so the reconcile is carried
forward as-is under the resume-safety guard. The previous run's `## Run halted` section is
removed by this commit — git history keeps it — because the run is live again.

## Run halted

2026-08-08 — `docket-implement-next` halted at Step 5 (build) for the **third** time, but on a
**new and much narrower** cause. The previous halt's root cause is RESOLVED: the `docket-build-*`
profile wrappers now route correctly to the `opencode` runner and the delegation actually
executed real work. What stopped this run is a **dispatch-duration ceiling**, not configuration.

**Routing verdict (the thing this run was re-entered to test): the delegation WORKS.**
`docket-build-standard` was dispatched by name for plan Task 1 and made its single facade call

```
docket.sh runner-dispatch --runner opencode --agent build-standard \
  --model openrouter/deepseek/deepseek-v4-flash-0731 --effort high \
  --worktree /Users/homer/dev/docket/.worktrees/guard-the-config-suite-s-enumerated-claims-export-order-and
```

The child ran inside the named feature worktree, on the feature branch, and performed genuine
work before it was killed. Evidence relayed from its stdout (unverified by the parent, since the
run did not complete):

- Baseline focused run: all `0258 L1` asserts green. The child noted a plan defect in passing —
  Step 2's prose says "nine `ok - 0258 L1 …` lines", but the verbatim Step 1 block contains
  exactly **eight** asserts.
- Step 3 (fence reorder `ADRS_DIR`/`RESULTS_DIR`): both sequence asserts NOT OK; restored.
- Step 4a (delete `LEARNINGS_CAP` from the fence): both sequence asserts plus the prose-numeral
  assert (`32/33`) NOT OK; restored.
- Step 4b (prose numeral only, `33`→`32`): only the prose assert NOT OK; restored.
- Step 5 (count-stable rename `BOARD_SURFACES`→`BOARD_SURFACE`): both sequence asserts NOT OK,
  prose assert green; restored. This is the mutation that proves the guard earns its place.
- Step 6 (resolver-side mutation) and Step 7 (clean-check + commit): **never reached**.

**What stopped the run.** The `runner-dispatch` call exited **143** — killed at the harness's
600000 ms maximum foreground timeout. The wrapper's own contract is "if the dispatch exits
non-zero, abort-and-report its stderr diagnostic — never retry silently, and never run the skill
inline on this harness as a fallback", so the worker correctly returned `BLOCKED`. A worker
`BLOCKED` is a `docket-build` halting condition ("continuation is unsafe"), and `skills.build`
resolves to `docket-build`, not `auto`, so the parent has no authorization to finish the task
inline.

**Root cause is capacity, not correctness.** Each full run of `tests/test_docket_config.sh`
(2868 lines) costs roughly 60 s. Task 1 as written serializes a baseline plus **six** whole-file
runs across its mutation proofs, and the child got through five of them in ten minutes. The task
is simply larger than one foreground dispatch window.

**State left behind (safe to resume).** No commit was made and there is **no stray commit** — the
branch is still at `7bd45872` (the plan commit). The worktree
`.worktrees/guard-the-config-suite-s-enumerated-claims-export-order-and` is dirty with exactly one
modified path, `tests/test_docket_config.sh` (+64 lines — the Task 1 Step 1 guard block, already
verified green and mutation-proved through Step 5). `scripts/docket-config.md` and
`scripts/docket-config.sh` are **clean**, so every mutation was restored byte-for-byte. The
worktree is preserved deliberately for inspection or resume.

**What a human must decide.** One of:

1. **Narrow the per-run command.** The mutation proofs re-run the whole 2868-line file each time.
   A focused command that runs only the `0258 L1` section would cut each proof from ~60 s to
   seconds and bring Task 1 inside one dispatch window. This is the cheapest fix and is a plan
   edit, not a design change.
2. **Re-cut Task 1 into two tasks** at the Step 5/Step 6 boundary, so each fits a dispatch window.
   Note that resuming mid-task is not something the worker contract supports — the second task
   would have to be written to adopt the already-present guard block.
3. **Raise the runner dispatch budget** if `runner-dispatch` can be given a longer ceiling than the
   600000 ms foreground maximum, which is the harness's limit rather than docket's.

Option 1 is recommended: it is the only one that also makes Task 2's five mutation proofs
affordable, and Task 2 will otherwise hit the identical wall.
