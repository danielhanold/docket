---
id: 322
slug: go-installer-adopt-legacy-bash-installed-user-level-artifact
title: 'Go installer: adopt legacy Bash-installed user-level artifacts via a frozen legacy renderer'
status: proposed
priority: medium
type: feat
created: 2026-08-14
updated: 2026-08-14
depends_on: []
stacked_on:
related: []
discovered_from: [311]
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

**Trigger** — change 0311's deep review (important #3): the Go installer's third ownership proof (legacy reproduction) shipped as a nil-wired seam, so any machine that ran the Bash `sync-agents.sh` dead-ends on `ownership-conflict` at exactly the paths `docket install` plans (`~/.claude/agents/docket-*.md`, `~/.cursor/agents/docket-*.md`, `~/.cursor/rules/docket-dispatch.mdc`, `~/.codex/agents/docket-*.toml`, the `docket:dispatch` managed blocks).
**Opportunity** — a frozen legacy renderer (or an explicit, non-`--force` adoption operation gated on byte reproduction) that proves a target is the Bash installer's output and lets the Go installer take it over safely.
**Independent value** — every existing docket machine is currently un-migratable to the Go installer without hand-deleting files; adoption unblocks the whole Go v1 rollout and stands even if 0311's internals change.
**Boundary** — reproduce and adopt only known user-level Bash-installer artifact shapes; never repo-local files (0313's territory), never a broad overwrite switch; the ownership-comparison primitive from 0311 is the mechanism.
**Reason for deferral** — a byte-faithful frozen copy of the Bash emitters is its own sizable, testable deliverable; carrying it on 0311's branch would have expanded an already 21k-line change. 0311 ships the seam plus an honest human-output limitation note instead.
