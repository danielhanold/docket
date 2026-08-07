---
id: 252
slug: harden-test-fixtures-and-hermeticity-into-tests-lib
title: 'Harden test fixtures and hermeticity into tests-lib'
status: proposed
priority: medium
type: chore
created: 2026-08-07
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [243]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

Consolidates #0243, #0177, and #0182 (2026-08-07 triage): three test-infrastructure robustness changes that all want the same home — shared, checked helpers under `tests/lib/` (precedent exists: `tests/lib/sync_agents_common.sh`).

Verified 2026-08-07:

- **Unchecked git fixture setup flakes the gate (#0243).** `tests/test_docket_example_yml.sh:24-32` `mkrepo()` runs `git init --bare`/`clone`/`checkout -b`/`add` with no exit-status checks; `:45-50` runs `cp`/`add`/`commit`/`push` bare; the file's own comment at `:52-54` says so. A transient failure here reddened 0190's post-fix gate once and never reproduced. Three files define a `mkrepo`; no shared fixture helper exists. House discipline is fail-loudly, not retry.
- **0174's template helpers have four robustness gaps (#0177), all confirmed:** (1) sticky `MKREPO_TEMPLATE` global assigned before any `git init` (test_docket_config.sh:16-17, test_closeout.sh:81-82, test_board_checks.sh:54-55) — a partial build failure leaves a poisoned template for every later test; (2) unguarded `mktemp -d` before `cp -R` (test_closeout.sh:145-148, test_board_checks.sh:79); (3) undocumented destructive `rm -rf` pre-clean (test_docket_config.sh:33); (4) leaked template root in test_closeout.sh (:161-164 admits it; the only EXIT trap at :611 covers `$tmp` only).
- **Facade tests read the developer's real global config (#0182).** `tests/test_runner_dispatch.sh:9` unsets `XDG_CONFIG_HOME` with no compensating pin; only the 0173-era block (:240-287) pins `DOCKET_HARNESS_ROOT`; the pre-existing sections (:131-161, :220-232 — the layer-resolution block especially) read `$HOME/.config/docket/config.yml` if present. The correct hermetic pattern is demonstrated at `test_docket_example_yml.sh:17-20` (`XDG_CONFIG_HOME="$tmp/xdg-void"` + "never read OR WRITE" comment).

## What changes

- Stand up a checked git-fixture helper in `tests/lib/` (fail-loud on every fixture git/cp step) and adopt it at the `mkrepo` sites.
- Harden the 0174 template helpers: assign the template global only after a successful build, guard `mktemp -d`, document the pre-clean, fix the leaked root (compose with the existing trap).
- Hermeticity sweep: pin `XDG_CONFIG_HOME` (sandbox dir) and/or `DOCKET_HARNESS_ROOT` at file scope in `test_runner_dispatch.sh`, then sweep the suite for the same fall-through; standardize the pattern in the shared helper.

## Out of scope

- The prose-guard `flatten()` hoist — that is the house-pattern change (same `tests/lib/` home; coordinate at build time).
- Retry semantics for fixture setup (fail-loudly is the discipline).
