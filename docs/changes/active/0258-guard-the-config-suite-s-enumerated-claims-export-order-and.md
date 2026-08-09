---
id: 258
slug: guard-the-config-suite-s-enumerated-claims-export-order-and
title: 'Guard the config-suite''s enumerated claims: export order and rung pairs'
status: in-progress
priority: medium
type: chore
created: 2026-08-07
updated: 2026-08-09
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
claimed_at: 2026-08-09T18:44:04Z
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

### 2026-08-09 — resumed at Step 5 (no scope change)

`origin/main` has advanced from `487bfdc5` to `05fbb224` (changes 0242 and 0269 landed), so the
resume-safety guard's re-reconcile fired. Every premise re-verified against the new tip and all
hold — none of the three files this change reasons about moved:

- `scripts/docket-config.md`, `scripts/docket-config.sh` and `tests/test_docket_config.sh` are
  byte-identical between `487bfdc5` and `05fbb224`. The `### Emit` fence is still 34 entries with
  `REPO_ROOT` annotated plain-only and the "33 lines in `shell` format; 34 in `plain`" sentence
  intact; `config_scalar_get` still dispatches exactly the three layer arms.
- `tests/runtime-budgets.tsv` did change in 0242, but the `tests/test_docket_config.sh` row is
  unchanged at `55  parallel`, so Task 3's budget check is unaffected.
- #0251 has still not landed, so the `test_docket_config*.sh` family glob still resolves to one
  file; the corpus-indifference constraint is honored regardless.
- The previous run's `## Run halted` section is removed by this commit — git history keeps it —
  because the run is live again.
- **Auto-capture (site A).** Nothing minted. The dispatch-duration ceiling that halted the last
  two runs is already tracked as #0271, so filing it again would be a duplicate.

### 2026-08-09 — resumed at Step 3 after the rebase onto main @ 324d2268 (no scope change)

`origin/main` advanced from `05fbb224` to `324d2268` (change 0271 landed), so the resume-safety
guard's re-reconcile fired. The design is carried forward unchanged; one in-scope code delta
followed from the rebase.

- **0271 moved two of the three files this change reasons about.** `scripts/docket-config.sh` gained
  `DELEGATION_OBSERVATION_BUDGET` in the resolver's emission, and `scripts/docket-config.md` gained
  the key in its config-key table and its exit-code table — but **not** in the `### Emit` fence.
  Leg 1's whole-sequence guard reddened on the rebase. That is the guard working as designed: the
  per-key presence greps it replaces all stayed green. Commit `a0484a2f` adds the key to the fence
  and moves the count prose from 33/34 to 34/35. In scope — the branch cannot be green without it.
- **Leg 1 premises re-verified.** The fence is now 35 entries (34 in `shell`), `REPO_ROOT` still
  annotated plain-only, and the prose sentence tracks at "34 lines in `shell` format; 35 in
  `plain`". The extractor's `### Emit` heading anchor and the resolver's live emission still agree
  entry for entry.
- **Leg 2 premises re-verified.** `config_scalar_get` still dispatches exactly the three layer arms
  (`committed` / `global` / `local`) plus the `*)` die arm. n = 3, n·(n−1) = 6, matching s4–s9.
- **Rebase conflict resolution sanity-checked.** 0271's `DOB-a`..`DOB-f` block and 0258's leg-1
  block collided additively at the tail of `tests/test_docket_config.sh`. Both are present and
  intact, 0271's first, with disjoint variable prefixes (`dob_*` vs `doc_*`/`emit_*`/`l1_*`) and the
  0174 template-integrity assert still last in the file.
- **`tests/runtime-budgets.tsv`** still carries `tests/test_docket_config.sh	55	parallel`; the
  branch does not touch it.
- **#0251 still `proposed`**, so the `test_docket_config*.sh` family glob resolves to one file; the
  corpus-indifference constraint is honored regardless.
- The previous run's `## Run halted` section is removed by this commit — git history keeps it —
  because the run is live again. Its four carried-forward review findings are re-triaged in this
  run's own review pass rather than adopted as a verdict against a tree that no longer exists.
- **Auto-capture (site A).** Nothing minted. The dispatch-duration ceiling that halted the last
  three runs is #0271, which is now merged; this run is its real-world exercise.
