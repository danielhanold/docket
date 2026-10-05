---
id: 526
slug: 'repository-init-writes-gate-off-and-configure-tests-then-ref'
title: 'repository configure-tests takes the test command as input'
status: 'in-progress'
priority: 'medium'
type: 'fix'
created: '2026-10-05'
updated: '2026-10-05'
depends_on: []
stacked_on:
related: [366, 352, 374, 523, 512]
discovered_from: [366]
adrs: []
spec: 'docs/superpowers/specs/2026-10-05-repository-init-writes-gate-off-and-configure-tests-then-ref-design.md'
plan: 'docs/superpowers/plans/2026-10-05-repository-init-writes-gate-off-and-configure-tests-then-ref.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/repository-init-writes-gate-off-and-configure-tests-then-ref'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-05T11:20:20Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-05-repository-init-writes-gate-off-and-configure-tests-then-ref-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-10-05-repository-init-writes-gate-off-and-configure-tests-then-ref-design.md) |
| Plan | [2026-10-05-repository-init-writes-gate-off-and-configure-tests-then-ref.md](https://github.com/danielhanold/docket/blob/fix/repository-init-writes-gate-off-and-configure-tests-then-ref/docs/superpowers/plans/2026-10-05-repository-init-writes-gate-off-and-configure-tests-then-ref.md) |
<!-- docket:artifacts:end -->

## Why

On the alpha.1 acceptance fixture (change 0366), `docket repository init` ran on a repository whose only test was an executable root `test.sh`. It wrote `.docket.yml` with `build.gate: "off"` and `finalize.gate: "off"` and no test commands. Then `repository configure-tests` returned `no-op (healthy): the test policy is already configured; nothing to write`. The operator had to edit `.docket.yml` by hand to set `gate: local` and `test_command: sh ./test.sh` for both gates.

The real cause: `configure-tests` takes no input. It only re-runs init's suite discovery, which recognizes six repository layouts and not a root `test.sh`. So it found nothing again, planned the same `off` policy, and printed its one generic "already configured" message. That message is also false when discovery finds two candidate suites. In that case init tells the operator to "run configure-tests to choose one", and configure-tests can't choose. Whenever discovery can't find the command, there's no command path, only a hand edit.

## What changes

- `docket repository configure-tests --command "<cmd>"` sets both gates to `local` with that command. It skips discovery and replaces whatever policy is there (`off`, a different command, or a half-configured pair). It still leaves a pending, unstaged `.docket.yml` edit for review and still runs only on a healthy repository. Running it again with the same command is a no-op. An empty value or the legacy `auto` is refused.
- Without `--command`, configure-tests says what discovery actually found. For "no suite found", it says the gates are off and to re-run with `--command`. For "two suites found", it names the candidates and their commands. For "already configured", it names the configured commands. The half-configured note names `--command` instead of a hand edit. init's "no suite" and "two suites" notes point at `--command` too.
- `migrate`'s refusal when two suites are found stops pointing at `configure-tests`, which refuses on a legacy repository. It says to set `finalize.test_command`, commit it, and re-run `migrate`.
- The glossary and the build/gate guide pages describe `--command`.

## Out of scope

- init still writes `gate: "off"` when it finds no supported suite. That's honest "skipped" evidence rather than a halt on every build, as decided in 0374.
- No new suite types. A root `test.sh` is still not auto-detected; `--command` covers it.
- No per-gate command flags. Different build and finalize commands stay a hand edit. No `--command` on `init` or `migrate`.
- Gate semantics, build evidence, and the `test-config-missing` check finding are unchanged.
- configure-tests refusing a repository whose `.docket` copy is only behind (change 0523).

## Reconcile log

### 2026-10-05

Reconciled against origin/main 19206a65d. Traced premises still hold: configure-tests takes only --repo-dir and emits one generic "already configured; nothing to write" message (internal/app/repository_configure_tests.go), init's ambiguous note still points at plain configure-tests (internal/app/repository_init.go testDiscoveryNote), ConfigureTestsGapNote still prescribes a hand edit, and AmbiguousTestDiscoveryError still names configure-tests. No overlapping work landed (0523/0524 do not touch the planner). Scope unchanged.
