---
id: 251
slug: retune-the-run-tests-budget-regime-for-portability-and-shard
title: 'Retune the run-tests budget regime for portability and sharding'
status: 'implemented'
priority: high
type: refactor
created: 2026-08-07
updated: '2026-08-23'
depends_on: []
related: [258, 273]
discovered_from: [229, 230]
adrs: []
spec: docs/superpowers/specs/2026-08-07-retune-the-run-tests-budget-regime-for-portability-and-shard-design.md
plan: 'docs/superpowers/plans/2026-08-22-retune-the-run-tests-budget-regime-for-portability-and-shard.md'
results:
trivial: false
auto_groomable: true
branch: 'feat/retune-the-run-tests-budget-regime-for-portability-and-shard'
pr: 'github.com/danielhanold/docket#232'
blocked_by:
reconciled: true
claimed_at: '2026-08-22T18:34:00Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | `docs/superpowers/specs/2026-08-07-retune-the-run-tests-budget-regime-for-portability-and-shard-design.md` |
| Plan | `docs/superpowers/plans/2026-08-22-retune-the-run-tests-budget-regime-for-portability-and-shard.md` |
<!-- docket:artifacts:end -->

## Why

Consolidates #0229 and #0230 (2026-08-07 triage): both discovered from 0227, both about the run-tests budget/sharding regime; a fix to one constrains the other.

Verified 2026-08-07:

- **Hardware-pinned slack factor (#0229).** `scripts/run-tests.sh:78` — `SLACK_NUM=5; SLACK_DEN=2`, consumed at `:306`: parallel wall-clock is compared against serially-measured budgets via a constant derived from one 11-core machine. Smaller hosts flake red; larger hosts make the check vacuous. The file's own comment at `:56` concedes the failure mode ("teaches people to pass `--no-budget-check`"). Note the flag set has moved since the stub: `--strict-budget` now exists (`:107-111`) — re-read the current levers before designing.
- **Self-scanning population floor pins the file (#0230).** `tests/test_docket_config.sh:2623` asserts the 0126 poison-prelude guard reached `>= 60` sites by scanning `${BASH_SOURCE[0]}` (`:2594`, cross-checks `:2615-2647`) — any file split falsifies the floor, so the guard blocks sharding the suite's biggest file (2769 lines, 55s budget in runtime-budgets.tsv, measured ~50s — ~5s headroom, closer than the stub's "not urgent" framing). It is the only file with this shape (`test_comment_anchor_style.sh:47` and `test_grep_portability.sh:34` use BASH_SOURCE for self-exclusion only).

## What changes

Settled design (2026-08-07 auto-groom; budget leg amended 2026-08-11 on human review; detail in
the linked spec):

- **Budget leg — stateful budget confirmation.** The parallel-phase `5/2` comparison is a *screen*
  producing candidate observations, tracked across executions in persistent per-test,
  per-execution-context state (`$GIT_DIR/docket/run-tests-budget-state.tsv`, advisory
  infrastructure — fail-open, locked, atomically rewritten). Five consecutive qualifying parallel
  overruns schedule a solo confirmation; the `ceiling * 3/2` solo comparison is the only
  authoritative breach (at `-j 1`, direct 3/2 comparison, no state). A confirmed-healthy test is
  classified parallel-sensitive and revalidated after every ten later qualifying overruns; a
  normal suite run performs at most ONE scheduled confirmation. `--strict-budget` bypasses the
  schedule — every current candidate is confirmed immediately (exit 4 on a confirmed breach or a
  failed confirmation, fail-closed). A failed confirmation never clears a candidate, and the
  confirmation run never changes the suite pass/fail verdict. Advisory default, 0/1/3/4 exit
  precedence, and the `--timings` five-column format are unchanged. Replaces the original
  one-unconditional-serial-rerun-per-candidate design (unbounded confirm tail; spec assumption 4
  records the reversal). Thirty deterministic tests enumerated in the spec, driven by a
  measured-duration injection seam — no real multi-second sleeps.
- **Floor leg — family-corpus guard, then the split.** `prelude_report`, the raw-grep cross-check
  extractors, and the r9 site derivation move to a glob-discovered corpus over
  `tests/test_docket_config*.sh` (computed membership, ADR-0050 shape; whole-corpus floors keep
  today's values; new `>= 2` files corpus floor; SITE lines gain file attribution). Then
  `tests/test_docket_config.sh` (2868 lines, ~50s vs a 55s budget) is split two ways at a measured
  section boundary with summed assertion-count parity, budget rows re-cut and `EXPECTED_TOTAL`
  re-seeded.
- Docs move in the same change: run-tests.sh comment block, run-tests.md budget sections,
  tests/README.md ("argued whole" paragraph + placement guidance), stale 0229/0230 references
  repointed to 0251, suite file counts corrected (88 today, 89 post-split).

## Out of scope

- Rewriting the budgets themselves (`runtime-budgets.tsv` values) beyond the mechanical re-cut of
  the split file's two rows.
- Any non-config-suite shard; no new default gating (advisory posture not revisited).

## Open questions

Resolved at grooming: the stub's "genuine forks with no house preference" read was too pessimistic —
run-tests.md and the budget learnings state the direction (contention-independent measurement so the
gate can be sharp again), and the critic passed the design with 0 needs-human verdicts across two
rounds. Coordination with #0258 (same test file) is at build time: whichever lands second rebases;
the glob corpus makes the guard indifferent to where 0258's asserts land. Residual (parked, in the
spec's assumption 2): budget table values still encode the calibration host's absolute speed —
confined to the opt-in strict path; a serial-canary rescale is the named follow-up shape if that
path proves flaky on slower hosts.

## Reconcile log

### 2026-08-22

Reconciled against current `origin/main` (f94844d7) and `origin/docket`. Design holds on both legs; scope unchanged; no fundamental invalidation. Findings folded in as build-time guidance:

- **Related #0258 has landed** (archived done, 2026-08-09) touching the same file. It introduced a `tests/test_docket_config*.sh` family glob for its OWN L2 control (test_docket_config.sh:~3015-3043), but the 0126 prelude-correspondence guard (test_docket_config.sh:~2600-2790) STILL does a whole-file `${BASH_SOURCE[0]}` scan with the `>= 60` sites floor — so Leg 2's actual target is intact and the build must move that guard, not assume 0258 already did. 0258's existing family-glob control is prior art to mirror, not duplicate.
- **Every cited count/line number in the spec and change body is stale — re-derive mechanically at build time, never copy the spec's literals** (learning: backstops compute, never re-enumerate): suite is now **121** `tests/test_*.sh` files (spec/docs say 86/88; run-tests.md:346 still says "86 files"); `test_docket_config.sh` is now **3304** lines (spec said 2868); `EXPECTED_TOTAL` is **2275** (spec said 1345); `runtime-budgets.tsv` has grown; `SLACK_NUM=5; SLACK_DEN=2` now at run-tests.sh:80 (spec said :78), advisory comparison at :346. The post-split suite file count and re-cut budget rows must come from measured `-j 1 --timings` runs at build time.
- **Doc-count updates (assumption 8) expand accordingly**: the stale suite-file counts to correct are wherever they now live (run-tests.sh header, run-tests.md, tests/README.md), derived from a fresh count — not the spec's frozen "89".
- Leg 1 (stateful budget confirmation state machine + measured-duration injection seam, 30 deterministic tests) is unaffected by codebase drift; the advisory/strict exit contract (0/1/3/4) is still byte-compatible in run-tests.sh. AUTO_CAPTURE is disabled this run; no adjacent stubs minted.
