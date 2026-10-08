---
id: 512
slug: 'release-v1-0-0-alpha-2-prove-and-publish-cursor-support'
title: 'Release v1.0.0-alpha.2: prove and publish Cursor support'
status: 'in-progress'
priority: 'high'
type: 'chore'
created: '2026-10-04'
updated: '2026-10-08'
depends_on: [366, 543]
stacked_on:
related: [366, 511, 513, 523, 524, 525, 526, 543]
discovered_from: []
adrs: []
spec: 'docs/superpowers/specs/2026-10-08-release-v1-0-0-alpha-2-prove-and-publish-cursor-support-design.md'
plan:
results:
trivial: false
auto_groomable: false
branch_prefix:
branch: 'chore/release-v1-0-0-alpha-2-prove-and-publish-cursor-support'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-08T18:11:43Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-08-release-v1-0-0-alpha-2-prove-and-publish-cursor-support-design.md](../../superpowers/specs/2026-10-08-release-v1-0-0-alpha-2-prove-and-publish-cursor-support-design.md) |
<!-- docket:artifacts:end -->

## Why

The human decided to grow the Go pre-releases one harness at a time:

- `v1.0.0-alpha.1` (change 0366) is human-tested on Claude Code only.
- Cursor comes next, in alpha.2.
- OpenCode follows in alpha.3.

Until alpha.2 ships, Cursor users have no tested release and no upgrade steps for their Cursor files.

## What changes

A human-attended release protocol, run by the operator in an attended session, never by `docket-implement-next`. It reuses alpha.1's protocol (0366) with the Claude Code row replaced by a Cursor row. The spec holds the full protocol.

- **Before the cut.** 0543 (the Cursor section of the Bash upgrade guide and its test) is merged. The Cursor isolation launch is dry-run against the alpha.1 binary before `main` is frozen.
- **Package once.** The candidate workflow builds `v1.0.0-alpha.2`. The `evidence.json` byte check is exact again (0524).
- **Cursor test.** Run the full lifecycle in a separate Cursor instance launched from a test home, with its own user-data directory, running outside the sandbox (Run Everything). The whole Cursor process is killed as the build starts and the run is resumed. The private fixture finalizes by itself (0525) and gets its test gates from `configure-tests --command` (0526).
- **Publish.** A draft pre-release with six verified assets, published at the human's explicit "publish". `v0.9.3` stays Latest.
- **Public install check** with `--harness cursor`.
- **Closeout.** Reconcile and attach a pointer plan right after the claim. Gate, record evidence, attach results and publish the PR before marking the change implemented. Then finalize.
- **alpha.1's gaps** are each designed in: installing before publication, test-home Git credentials, fixture setup, the kill window, capturing verdict lines, closeout with no code, PR evidence ordering, private-fixture finalize, and evidence byte-equality.

## Out of scope

- The upgrade guide's Cursor section (change 0543).
- OpenCode (alpha.3) and Codex (paused).
- Re-proving Claude Code beyond the candidate's whole-suite source gate.
- Proving Cursor's documented Allowlist (with Sandbox) setup; the run uses Run Everything, and the notes list that as a known gap.

## Reconcile log

### 2026-10-08

Reconciled 2026-10-08 at the alpha.2 cut, as a hand-run release protocol. Dependency 0543 is merged (PR #408) and the candidate is origin/main ec4c2b1841954c2d6a3dd018e1133ee762861137. The Phase 0 Cursor isolation dry run passed on Cursor 3.23.23. Two adjustments to the protocol, both recorded in decisions.md: (1) the isolation probe also dispatches a test-home-only agent (~/.cursor/agents/zz-isolation-probe.md), because a ~/.cursor/rules marker was not loaded by Cursor; (2) change.attach-plan now writes the plan on the metadata branch itself, so no plan-only commit with a Docket-Plan-Path trailer is needed. The spec is otherwise current.
