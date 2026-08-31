---
id: 120
slug: docket-finalize-change-claims-integration-branch-is-read-fro
title: docket-finalize-change claims integration_branch is read from .docket.yml, but it is an exported resolver key
status: done
priority: medium
created: 2026-07-21
updated: 2026-07-28
depends_on: []
related: []
discovered_from: [102]
adrs: [52]
spec: docs/superpowers/specs/2026-07-26-skill-config-read-channel-design.md
plan: docs/superpowers/plans/2026-07-27-config-read-channel-guard.md
results: docs/results/2026-07-27-docket-finalize-change-claims-integration-branch-is-read-fro-results.md
trivial: false
auto_groomable: true
branch: feat/docket-finalize-change-claims-integration-branch-is-read-fro
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/130
blocked_by:
reconciled: true
type: docs
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-26-skill-config-read-channel-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-26-skill-config-read-channel-design.md) |
| Plan | [2026-07-27-config-read-channel-guard.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-07-27-config-read-channel-guard.md) |
| Results | [2026-07-27-docket-finalize-change-claims-integration-branch-is-read-fro-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-07-27-docket-finalize-change-claims-integration-branch-is-read-fro-results.md) |
| ADRs | [ADR-0052](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0052-config-key-resolution-boundary.md) |
<!-- docket:artifacts:end -->

## Why

`skills/docket-finalize-change/SKILL.md` states that `<integration_branch>` is "resolved from
`.docket.yml`" — but `INTEGRATION_BRANCH` is an **exported resolver key**, emitted in the Step-0
`preflight` block like `FINALIZE_GATE` and `CHANGES_DIR`.

This is the exact bug class change 0102 just closed, one key over: a user who sets
`integration_branch` in `.docket.local.yml` gets a value that every *script* honors (they read the
export) while the *skill body* ignores it (it is told to read the committed file). The two halves
of the toolchain would disagree about where code lands — a worse blast radius than 0102's, since
`integration_branch` decides the merge target.

Note `integration_branch` IS coordination-key fenced (ADR-0019), so a machine-scoped value is
warned-and-ignored rather than silently dropped. That makes this less severe than 0102 — the user
gets a warning — but the skill's prose is still factually wrong about its own read channel, and
ADR-0052 now states the rule it violates.

Surfaced by the whole-branch review of change 0102.

## What changes

Settled by the linked spec (autonomously groomed 2026-07-26):

- **Correct the one false claim** — `skills/docket-finalize-change/SKILL.md`'s *Per-change steps*
  step 1 parenthetical, to name the exported `INTEGRATION_BRANCH` read from the Step-0 `preflight`
  block, keeping the "not hard-coded `main`" warning.
- **The sibling audit is complete** and found no second false claim: 16 `.docket.yml` occurrences
  across 5 skill files, all others being write-backs, negatives, or convention prose about the file
  itself. The implementer re-runs the grep at reconcile rather than trusting the snapshot.
- **Guard the class** with a new `tests/test_config_read_channel.sh`: an auto-discovered population
  (`skills/**/*.md` minus two declared `docket-convention` exclusions) in which every `.docket.yml`
  occurrence must carry an in-line class marker (`write-back` / `negative`) or fail. Four legitimate
  sites get markers as part of this change. Mutation-tested, with a population floor.
- **Append a dated `## Update` to ADR-0052** naming the second enforcer; delivered atomically via
  `adrs: [52]`.

**Note:** `skills/docket-finalize-change/SKILL.md` is at 4131/4200 words and 189/193 lines;
`github-board-mirror.md` has only 2 lines of budget headroom. Check both dimensions before assuming
the marker lines fit.

## Out of scope

- Changing what `integration_branch` means or how it resolves.
- Re-litigating its coordination-key fencing.

## Triage note (2026-07-26, change 0124)

Confirmed still live. The wrong claim is at `skills/docket-finalize-change/SKILL.md:92`, verbatim:
"merge it into `<integration_branch>` (resolved from `.docket.yml`; not hard-coded `main`)". The
resolver does export `INTEGRATION_BRANCH` (confirmed in the Step-0 `preflight` block), so the prose
and the mechanism disagree exactly as described.

Three other occurrences in the same file (lines 65, 104, 125) use `<integration_branch>` as a
placeholder without asserting provenance — those read correctly and need no edit. Only line 92
makes the false claim, so the minimal fix is one clause, leaving headroom for the sibling audit
against the 4200-word budget.

## Reconcile log

### 2026-07-27 — build-time reconcile (claimed for implementation)

Re-ran the spec's §2 audit against `origin/main` @ `0da1c0aa`. Every snapshot in the spec still
holds; scope is unchanged.

- **The bug is still live**, still one clause, still at `skills/docket-finalize-change/SKILL.md:92`:
  "merge it into `<integration_branch>` (resolved from `.docket.yml`; not hard-coded `main`)".
- **Audit re-run confirms the snapshot exactly**: 16 `.docket.yml` occurrences across the same 5
  skill files, in the same distribution — `docket-convention/SKILL.md` 6, `references/agent-layer.md`
  5, `docket-finalize-change/SKILL.md` 2 (the bug + the correct `negative` at line 108),
  `docket-status/SKILL.md` 2 (`write-back`, lines 61 and 78), `github-board-mirror.md` 1
  (`write-back`, line 17). No second false claim; no site the spec did not already classify.
- **§4's exclusion re-confirmed.** `docket-convention/SKILL.md:54` ("`integration_branch` is a value
  *read from* the file, so the file cannot be located *by* it") reads in context as a statement about
  where `.docket.yml` *lives* — the sentence it sits in is about the file being on the default branch,
  not the integration branch. Line 58 of the same file attributes the actual read to the resolver
  ("read `.docket.yml` authoritatively … performed deterministically by the config resolver"). It is
  not a read-channel instruction to any agent, so the exclusion stands and the spec's fallback
  rephrase is NOT triggered.
- **Both budget dimensions re-measured** on `origin/main`, and all four marker lines fit:
  `docket-finalize-change/SKILL.md` 189/193 lines · 4131/4200 words (+1 line);
  `docket-status/SKILL.md` 107/118 · 2323/2393 (+2 lines);
  `github-board-mirror.md` 17/19 · 420/462 (+1 line — the tightest, landing at 18/19).
- **ADR-0052 is still `Accepted`** and its `## Enforcement` section still names only
  `tests/test_docket_example_yml.sh`, so the dated `## Update` in scope item 4 is still required.
- `depends_on` is empty; nothing is gated. No work has been done elsewhere; no scope dropped.
