---
id: 526
slug: 'repository-init-writes-gate-off-and-configure-tests-then-ref'
title: 'repository configure-tests takes the test command as input'
status: 'proposed'
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
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-05-repository-init-writes-gate-off-and-configure-tests-then-ref-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-10-05-repository-init-writes-gate-off-and-configure-tests-then-ref-design.md) |
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
