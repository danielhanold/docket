---
id: 316
slug: finalize-recovery-reclaim-archive-and-stacks
title: 'Finalize, recovery, reclaim, archive, and stacks'
status: blocked
priority: critical
type: feat
created: 2026-08-12
updated: 2026-08-18
depends_on: [315]
stacked_on:
related: [298]
discovered_from: [303]
adrs: [10, 11, 35, 43, 59, 66, 74, 83, 86, 92, 95]
spec: docs/superpowers/specs/2026-08-18-finalize-recovery-reclaim-archive-and-stacks-design.md
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by: "human decision required: provide a sanctioned Go transaction CLI or explicitly authorize an exact from-source binary to mutate shared origin/docket"
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-18-finalize-recovery-reclaim-archive-and-stacks-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-18-finalize-recovery-reclaim-archive-and-stacks-design.md) |
| ADRs | [ADR-0010](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0010-finalize-merge-gate-split-agents.md), [ADR-0011](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0011-finalize-consent-model.md), [ADR-0035](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0035-cleanup-teardown-fail-closed.md), [ADR-0043](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0043-retire-bot-auto-approval-zero-approvals-branch-protection.md), [ADR-0059](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0059-dispatch-capability-resolved-not-inferred-from-tool-name.md), [ADR-0066](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0066-docket-owns-the-review-role-suite-runs-in-the-build-gate.md), [ADR-0074](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0074-build-gate-verdict-is-tri-state-runner-defined-non-failure-exit-is-a-halt.md), [ADR-0083](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0083-agent-worktree-scope-is-a-declared-frontmatter-fact.md), [ADR-0086](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0086-in-context-gating-dispatch-carved-out-of-the-tier-taxonomy.md), [ADR-0092](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0092-a-stacked-changes-base-is-its-parents-merge-destination.md), [ADR-0095](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0095-native-supervisor-delivers-a-real-session-and-an-exact-terminal-record.md) |
<!-- docket:artifacts:end -->

## Why

The migration is not functionally complete until merged and interrupted work converges safely to
the correct terminal state in both repository modes.

## What changes

Add authoritative finalize context and resumable Go operations for local rebase/retest, rewritten
head publication, merge verification, atomic terminal archive and stack close-out, explicit and
policy-driven reclaim, durable halted/finalize-blocked recovery, merged-PR maintenance, generated
terminal-link repair, and ownership-safe run/workspace/branch cleanup.

## Implementation launch blocker

No sanctioned runtime currently satisfies the active `docket-implement-next` transaction contract:
`install.sh` does not install the Go binary, `docket.sh` does not implement its verbs, and a clean
from-source build plus a successful read-only context is not authority to mutate shared
`origin/docket` or open the real PR. A human must either provide the sanctioned runtime or
explicitly authorize an exact from-source binary for this run. Until then, abort and report before
claim, plan, branch, workspace, mutation, or PR creation.

## Out of scope

Behavior owned by changes 0305 through 0315; release packaging and four-harness acceptance from
0317; configuration contraction, self-hosting, Bash removal, and hard cutover from 0318; and
deferred CI/combined gates, results-only skips, terminal publishing, automatic learning harvest,
capture/groom automation, cross-harness routing, skill rebinding, or Bash fallback behavior. This
change also does not itself install or sanction a Go `docket` executable for its migration host,
add Go verbs to `docket.sh`, authorize a source-built binary to mutate Docket's live metadata, or
bypass the unsupported-capability refusal caused by Docket's pre-cutover resolved configuration.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
