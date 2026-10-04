---
id: 506
slug: 'drop-the-retired-kiro-and-windsurf-globs-from-the-managed-gi'
title: 'Drop the retired-harness globs from the managed .gitignore block'
status: 'in-progress'
priority: 'low'
type: 'fix'
created: '2026-10-04'
updated: '2026-10-04'
depends_on: []
stacked_on:
related: [464]
discovered_from: [464]
adrs: [20, 60]
spec: 'docs/superpowers/specs/2026-10-04-drop-the-retired-kiro-and-windsurf-globs-from-the-managed-gi-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/drop-the-retired-kiro-and-windsurf-globs-from-the-managed-gi'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-10-04T09:14:59Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-10-04-drop-the-retired-kiro-and-windsurf-globs-from-the-managed-gi-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-10-04-drop-the-retired-kiro-and-windsurf-globs-from-the-managed-gi-design.md) |
| ADRs | [ADR-0020](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0020-generated-agent-artifacts-machine-local.md), [ADR-0060](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0060-generated-wrapper-conforms-to-target-harness-contract.md) |
<!-- docket:artifacts:end -->

## Why

`docket repository init` and `migrate` write a managed block into each repository's `.gitignore`. The block still ignores agent-wrapper files for three harnesses docket no longer supports: `.agents/agents/docket-*.md`, `.kiro/agents/docket-*.md`, and `.windsurf/agents/docket-*.md`. The accepted harness vocabulary is `claude`, `codex`, `cursor`, `opencode`, and ADR-0020 says the block follows that roster. Found while building change 0464.

## What changes

- Remove the three retired-harness lines from the canonical block in `internal/reposetup/gitignore.go`, its frozen test copy, and this repository's own `.gitignore`.
- Keep the four supported-harness wrapper globs and the codex `.toml` glob: docket no longer writes wrappers into repositories, but those lines hide leftover wrapper files from older versions, and removing them would make `repository init` see a dirty checkout.
- Add a test that every agent-wrapper line names a supported harness, plus tests that an existing repository's old block is rewritten cleanly and is reported as non-canonical by `repository check`.
- Rewrite the stale header comments in `gitignore.go` and its test, which still describe a deleted bash emitter and parity test.
- No compatibility code: existing repositories pick up the new block by re-running `docket repository init` and committing `.gitignore`; the results file says so.

## Out of scope

- Retiring the supported-harness wrapper globs or deleting leftover wrapper files.
- `link-skills.sh`'s `.kiro`/`.windsurf` skill targets and other harness support changes.
- Teaching `repository check` to accept the previous block.

## Reconcile log

### 2026-10-04

Re-checked against origin/main (ecbb32f19): `canonicalBlockBytes` in `internal/reposetup/gitignore.go` and the repository `.gitignore` still carry the three retired-harness globs, and the stale bash-port header comment is still present. The accepted harness vocabulary is unchanged. Scope and spec hold as written; no adjustment.
