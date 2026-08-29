---
id: 369
slug: 'migrate-maintained-consumers-to-the-direct-go-cli'
title: 'Migrate maintained consumers to the direct Go CLI'
status: 'proposed'
priority: 'critical'
type: 'refactor'
created: '2026-08-29'
updated: '2026-08-29'
depends_on: [318]
stacked_on:
related: [317, 322, 326, 361, 366, 367, 370]
discovered_from: [318]
adrs: [14, 29, 30, 33, 36, 74, 99]
spec: 'docs/superpowers/specs/2026-08-29-migrate-maintained-consumers-to-the-direct-go-cli-design.md'
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
| Spec | [2026-08-29-migrate-maintained-consumers-to-the-direct-go-cli-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-29-migrate-maintained-consumers-to-the-direct-go-cli-design.md) |
| ADRs | [ADR-0014](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0014-consuming-repo-script-resolution.md), [ADR-0029](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0029-docket-facade-routing-and-config-presentation.md), [ADR-0030](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0030-facade-wiring-guard-discriminates-on-invocation-prefix.md), [ADR-0033](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0033-cursor-auto-run-trust-at-facade.md), [ADR-0036](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0036-codex-agents-md-dispatch-block-committed-machine-neutral.md), [ADR-0074](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0074-build-gate-verdict-is-tri-state-runner-defined-non-failure-exit-is-a-halt.md), [ADR-0099](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0099-one-metadata-topology-for-go-v1.md) |
<!-- docket:artifacts:end -->

## Why

The Go command surface cannot become the only supported control plane while skills, agents, generators, workflows, setup checks, and operator instructions still route through the legacy Bash facade. Migrating these maintained consumers is a repo-wide but reviewable boundary that must land before deletion.

## What changes

- Derive a whole-repository executable-site inventory and classify every active, generated,
  legacy, historical, and unknown occurrence.
- Rewrite maintained skills, agents, canonical generators, generated dispatch blocks, workflows,
  setup/health checks, validators, active instructions, and executable examples to invoke the
  PATH-resolved public Go CLI and consume JSON where machines interpret results.
- Regenerate products from canonical sources and prove deterministic, machine-neutral output and
  representative fresh-process loading.
- Leave the Bash facade/runtime/old runner and their tests behaviorally frozen and green, but prove
  that they have no maintained callers.
- Replace the facade-wiring rule with a fail-closed, shape-derived, mutation-tested no-new-callers
  guard and record the direct-Go architecture through the ADR workflow.

## Out of scope

Physical facade/runtime/configuration/test deletion (0370); a replacement forwarding shim; missing
Go capability invention; raw Git/GitHub mutation as a substitute for Docket; release publication,
rollback, or real-host acceptance (0366); post-cutover board work (0367); and rewrites of immutable
historical or frozen records.

## Design decisions

This is a sequential merged-main dependency on 0318, not a stacked branch. The intermediate state
must be independently green and usable. Process-start-loaded artifacts receive honest generator and
hermetic fresh-process evidence; live external harness reload remains human truth. Unknown inventory
or guard classification fails closed. The old implementation remains present but ceases to be a
supported integration contract.
