---
id: 224
slug: the-build-gate-contract-never-says-green-red-is-the-exit-code
title: The build gate contract never says green/red is the exit code, so an output-shape match passes as a gate
status: done
priority: high
type: docs
created: 2026-08-06
updated: 2026-08-07
depends_on: []
related: [190, 223, 227]
discovered_from: [203]
adrs: [74]
spec: docs/superpowers/specs/2026-08-07-the-build-gate-contract-never-says-green-red-is-the-exit-design.md
plan: docs/superpowers/plans/2026-08-07-the-build-gate-contract-never-says-green-red-is-the-exit-plan.md
results: docs/results/2026-08-07-the-build-gate-contract-never-says-green-red-is-the-exit-code-results.md
trivial: false
auto_groomable: true
branch: feat/the-build-gate-contract-never-says-green-red-is-the-exit-code
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/174
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-07-the-build-gate-contract-never-says-green-red-is-the-exit-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-07-the-build-gate-contract-never-says-green-red-is-the-exit-design.md) |
| Plan | [2026-08-07-the-build-gate-contract-never-says-green-red-is-the-exit-plan.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-07-the-build-gate-contract-never-says-green-red-is-the-exit-plan.md) |
| Results | [2026-08-07-the-build-gate-contract-never-says-green-red-is-the-exit-code-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-08-07-the-build-gate-contract-never-says-green-red-is-the-exit-code-results.md) |
| ADRs | [ADR-0074](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0074-build-gate-verdict-is-tri-state-runner-defined-non-failure-exit-is-a-halt.md) |
<!-- docket:artifacts:end -->

## Why

`skills/docket-build/SKILL.md` § *The build gate* defines what green and red **mean** — green mints
the build-evidence record, red enters the repair path — but never says what **determines** which one
a run is. In practice that is the test's exit code, and nothing in the contract says so.

On the change 0203 run (2026-08-06) the implementer's first gate attempt keyed pass/fail on a
`tail -1` string match against `"PASS"` instead of the exit code. It reported
`### RED rc=0` for nearly every test file in the suite — a gate that was simultaneously wrong in
both directions and visibly self-contradictory (`RED` next to `rc=0`). The agent caught it itself
and rekeyed on `rc`, but only because the output was absurd enough to notice.

The failure mode that matters is the quiet one. A shape-matching gate that happens to agree with the
exit code passes review and mints a **valid-looking build-evidence record** certifying a branch
nobody actually verified. Because the record is what lets the review step and finalize skip their own
suite runs, a false green propagates: `docket-implement-next` Step 6 validates the record's presence
and `head_sha`, not the reasoning that produced it.

This is squarely the repo's own house rule from `AGENTS.md` — key a guard on **shape, never an
enumerated list of spellings** — turned on the gate itself, and the reason unguarded prose is
treated as decoration.

There is a second hole the same clause closes: `skills/docket-build/references/gate-execution.md`
capability 5 requires the gate to distinguish *still running*, *completed successfully*, *completed
unsuccessfully*, and *result unavailable* — but nothing defines **successfully**, and § *Gate
execution posture* clause 3 forbids reading completion from the caller-visible signal of the command
that started the gate.

## What changes

- State the verdict rule normatively in `skills/docket-build/SKILL.md` § *The build gate*, in one
  named slot (after the `configured-bash-finalize` command-boundary paragraph, before `**Green** →`):
  green **iff** the resolved suite command exits zero; output text is diagnostic, never the verdict;
  the deciding status is the one recorded in the **terminal result artifact**, which is where
  *completed successfully* is settled; *still running* and *result unavailable* remain budget halts,
  never red; under a **per-file loop** the deciding status is the block's aggregate. The rule binds
  every full-suite run this role performs, including the repair worker's post-fix re-run.
- Add the guard to the **existing** `tests/test_docket_build.sh` under a change-0224 banner — a
  `/^#+ /`-terminated slice of § *The build gate*, flattened before phrase matching, with a
  non-vacuity companion through the same extractor and one independently mutation-tested assert per
  rule. Per `AGENTS.md`, an assert never seen red against its own mutation is decoration.
- **Raise the `skills/docket-build/SKILL.md` size-budget row in the same diff.** Measured 2026-08-07
  the file is 317/2938 against `325 3000` — the clause does not fit. Follow
  `tests/test_skill_size_budgets.sh`'s documented raise rule and its change-0201 obligation to name
  the `references/` candidate (`gate-execution.md`) and argue in-diff why the prose cannot live there.

The per-file-loop confirmation is not a separate deliverable: it is one sentence in the clause plus
its own assert, and it is already discharged against finalize's `configured-bash-finalize` block.

## Out of scope

- The execution posture / timeout problem — change 0223 (landed; this clause plugs into it).
- Suite runtime — change 0227 (landed; supersedes the killed 0225).
- Changing what green and red *do* (evidence record, repair ladder); only what decides them.
- Any change to the `docket:build-evidence` record schema (adjacent to change 0190).
- Any rule about what a suite *runner* should exit for a non-failure condition — the gate reads bare
  non-zero and must not learn an exit-code taxonomy.
- `docket-finalize-change`'s prose: its `configured-bash-finalize` block already *is* the
  exit-status test, so the mechanism is present there even though the wording is not.

## Open questions

Resolved by the spec (2026-08-07, autonomous groom — see its `## Assumptions` for the audit trail):

- **Existing test or a new one?** The existing `tests/test_docket_build.sh`, which already owns the
  build gate's contract prose. A new file was 0223's answer because it spanned four surfaces.
- **Does the rule bind the repair re-run and finalize?** The repair re-run yes; finalize no —
  scoped out above.
- **A cheap runtime assertion?** No. Contract prose plus the docs guard is the whole of it; an
  `exit_code:` field on the evidence record would change a schema with three consumers while 0190 is
  open, which is a human's call.

Residual, accepted rather than fixed: under `green iff zero`, `scripts/run-tests.sh`'s exit 3 (a
harness failure that certified nothing) reads as red. Fail-closed and identical to today's behavior.

## Reconcile log

### 2026-08-07 — reconciled against current `origin/main` (`4a11ddf0`)

Spec and stub both hold as written; scope unchanged, nothing dropped, nothing folded in.

- **Placement slot still exists verbatim.** `skills/docket-build/SKILL.md` § *The build gate* is
  unrestructured: the `configured-bash-finalize` command-boundary paragraph is still immediately
  followed by `**Green** →`, and `### Gate execution posture` is still the next `###` inside the
  section. Assumption 7's re-derive escape hatch is not needed.
- **The size-budget raise is still required and its numbers still hold.**
  `skills/docket-build/SKILL.md` measures **317 lines / 2938 words** against row `325 3000` in
  `tests/test_skill_size_budgets.sh` — 8 lines / 62 words of headroom, exactly as the spec measured
  on 2026-08-07. The raise stays part of this diff. The row's own most recent history (raised
  `320/2950 -> 325/3000` by change 0231) confirms the documented raise rule is the live one.
- **Dependencies confirmed satisfied.** `depends_on:` is empty. Changes 0223 and 0227 are archived,
  so `### Gate execution posture` and `scripts/run-tests.sh` are read as current tree state rather
  than as assumptions. Change 0190 remains non-terminal, which is precisely why assumption 3's
  refusal to touch the `docket:build-evidence` schema stands.
- **Guard home unchanged.** `tests/test_docket_build.sh` still owns the build gate's contract prose
  and still uses the banner-per-change discipline the spec's edit 2 follows.

Residual carried forward, not fixed here: `docket-finalize-change`'s prose stays silent on the
verdict rule (spec assumption 2). Not captured as a stub — the spec's own rationale rejects the
restatement (`restatement-accumulates-its-own-guards`), so filing it would be backlog churn against
a decision already argued.
