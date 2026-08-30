---
id: 372
slug: 'retire-deferred-go-v1-workflow-surfaces-and-seal-the-consume'
title: 'Retire deferred Go v1 workflow surfaces and seal the consumer cutover'
status: 'proposed'
priority: 'critical'
type: 'refactor'
created: '2026-08-30'
updated: '2026-08-30'
depends_on: [371]
stacked_on:
related: [312, 316, 326, 369, 370]
discovered_from: [369]
adrs: [14, 29, 30, 33, 36, 74, 99]
spec: 'docs/superpowers/specs/2026-08-30-retire-deferred-go-v1-workflow-surfaces-and-seal-the-consumer-cutover-design.md'
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
| Spec | [2026-08-30-retire-deferred-go-v1-workflow-surfaces-and-seal-the-consumer-cutover-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-30-retire-deferred-go-v1-workflow-surfaces-and-seal-the-consumer-cutover-design.md) |
| ADRs | [ADR-0014](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0014-consuming-repo-script-resolution.md), [ADR-0029](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0029-docket-facade-routing-and-config-presentation.md), [ADR-0030](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0030-facade-wiring-guard-discriminates-on-invocation-prefix.md), [ADR-0033](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0033-cursor-auto-run-trust-at-facade.md), [ADR-0036](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0036-codex-agents-md-dispatch-block-committed-machine-neutral.md), [ADR-0074](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0074-build-gate-verdict-is-tri-state-runner-defined-non-failure-exit-is-a-halt.md), [ADR-0099](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0099-one-metadata-topology-for-go-v1.md) |
<!-- docket:artifacts:end -->

## Why

Several Bash-era workflow legs remain active even though their Go v1 capabilities were explicitly deferred: automatic stub capture, automated learnings-index maintenance, and terminal publishing. Treating them as missing Go verbs would silently expand v1 and keep the Bash facade alive indefinitely.

## What changes

- Retire maintained automatic-capture/minting, automated learning, terminal-publish, and
  publication-deferral workflow legs without implementing replacements.
- Preserve legacy configuration keys, stored values, indexes, markers, learning records, and
  historical evidence while preventing any enabled value from falling back to Bash.
- Provide capability-specific fail-closed diagnostics and supported explicit alternatives.
- Add the final shape-derived, mutation-tested no-facade/no-deferred-active-path seal.
- Individually audit and formally disposition conflicting facade-era ADRs.

## Out of scope

Retained lifecycle-operation migration owned by change 369; native host dispatch cutover owned by change 371; implementation of the deferred capabilities; physical Bash facade deletion; release, rollback, or four-host self-host acceptance.

## Design decisions

Each retired surface becomes either a supported explicit alternative, preserved inactive
configuration, or an explicitly unsupported request rejected before mutation. The final seal derives
prohibited executable shapes and structural historical/frozen exclusions instead of hand-listing
caller files, and every exclusion is protected by negative controls. The run halts rather than grow
if retirement requires a new subsystem or conflicts with a still-current ADR.
