---
id: 252
slug: harden-test-fixtures-and-hermeticity-into-tests-lib
title: 'Harden test fixtures and hermeticity into tests-lib'
status: 'in-progress'
priority: high
type: chore
created: 2026-08-07
updated: '2026-08-28'
depends_on: []
related: [253, 278, 222]
discovered_from: [243, 177, 182]
adrs: []
spec: docs/superpowers/specs/2026-08-07-harden-test-fixtures-and-hermeticity-into-tests-lib-design.md
plan:
results:
trivial: false
auto_groomable: true
branch: 'chore/harden-test-fixtures-and-hermeticity-into-tests-lib'
pr:
blocked_by:
reconciled: false
claimed_at: '2026-08-28T21:05:10Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-07-harden-test-fixtures-and-hermeticity-into-tests-lib-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-07-harden-test-fixtures-and-hermeticity-into-tests-lib-design.md) |
<!-- docket:artifacts:end -->

## Why

Consolidates #0243, #0177, and #0182 (2026-08-07 triage): three test-infrastructure robustness changes that all want the same home — shared, checked helpers under `tests/lib/` (precedent exists: `tests/lib/sync_agents_common.sh`).

Verified 2026-08-07:

- **Unchecked git fixture setup flakes the gate (#0243).** `tests/test_docket_example_yml.sh:24-32` `mkrepo()` runs `git init --bare`/`clone`/`checkout -b`/`add` with no exit-status checks; `:45-50` runs `cp`/`add`/`commit`/`push` bare; the file's own comment at `:52-54` says so. A transient failure here reddened 0190's post-fix gate once and never reproduced. Three files define a `mkrepo`; no shared fixture helper exists. House discipline is fail-loudly, not retry.
- **0174's template helpers have four robustness gaps (#0177), all confirmed:** (1) sticky `MKREPO_TEMPLATE` global assigned before any `git init` (test_docket_config.sh:16-17, test_closeout.sh:81-82, test_board_checks.sh:54-55) — a partial build failure leaves a poisoned template for every later test; (2) unguarded `mktemp -d` before `cp -R` (test_closeout.sh:145-148, test_board_checks.sh:79); (3) undocumented destructive `rm -rf` pre-clean (test_docket_config.sh:33); (4) leaked template root in test_closeout.sh (:161-164 admits it; the only EXIT trap at :611 covers `$tmp` only).
- **2026-08-09 (absorbed #0278, killed pointing here): the #0243 flake struck again live**, at 0271's finalize merge gate — first full parallel run `SUITE files=100 passed=99 failed=1`, the one red being the fidelity fixture's vacuity guard (`NOT OK - fidelity fixture: example reached the fixture's origin/main`); isolated re-run and a second full run both green. The gate passed only because a human directed the re-run; an unattended finalize would have dispatched `docket-integration-repair` against a nonexistent defect. The spec's `fx` adoption at `test_docket_example_yml.sh` (`mkrepo` :24-32 and the fidelity `cp`/`add`/`commit`/`push` :45-50) is exactly the fix 0278 asked for; its "retry or abort" open question is already ruled here (hard abort, no retry). Second live occurrence — raises the urgency of building this change.
- **Facade tests read the developer's real global config (#0182).** `tests/test_runner_dispatch.sh:9` unsets `XDG_CONFIG_HOME` with no compensating pin; only the 0173-era block (:240-287) pins `DOCKET_HARNESS_ROOT`; the pre-existing sections (:131-161, :220-232 — the layer-resolution block especially) read `$HOME/.config/docket/config.yml` if present. The correct hermetic pattern is demonstrated at `test_docket_example_yml.sh:17-20` (`XDG_CONFIG_HOME="$tmp/xdg-void"` + "never read OR WRITE" comment).

## What changes

Settled design (see spec): a narrow, sourceable mechanics library `tests/lib/fixture_lib.sh` — not a prologue — plus adoption at the existing sites. Fixture builders stay per-file (their bodies genuinely differ); only the mechanics are shared.

- `fx` checked fixture-step runner (hard `exit 1`, loud marker, no retry; `|| true` expected-failure steps are exempt), `fx_mktemp_d` guarded mktemp, `fx_defer_rm` single lib-owned EXIT trap with path registration (fixes closeout's leaked template root and the trap-replacement hazard), `fx_pin_hermetic_config` pin-XDG-to-void — with a self-test file `tests/test_fixture_lib.sh` exercising the failure branches.
- Adopt `fx` at the 8 `mkrepo`/`new_repo` fixture-builder files; assign template globals only after a successful build; add a vacuity check at every substitution consumer of a builder; document `mkrepo`'s destructive pre-clean in the code.
- Hermeticity sweep, per-reader (`docket-config.sh` ignores `DOCKET_HARNESS_ROOT`; only an XDG pin is universally honored): pin-to-void at file scope in `test_runner_dispatch.sh`, then classify every `unset XDG_CONFIG_HOME` site against the reader it actually reaches; exposure is direct-invocation-only (run-tests.sh sandboxes the gate).

## Out of scope

- The prose-guard `flatten()` hoist — that is the house-pattern change (same `tests/lib/` home; coordinate at build time).
- Retry semantics for fixture setup (fail-loudly is the discipline).
