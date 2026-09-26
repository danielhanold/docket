---
id: 460
slug: 'artifact-backlink-refuses-an-absolute-change-path-with-unkno'
title: 'artifact.backlink refuses an absolute --change path with unknown-change'
status: 'implemented'
priority: 'high'
type: 'fix'
created: '2026-09-25'
updated: '2026-09-26'
depends_on: []
stacked_on:
related: []
discovered_from: [458]
adrs: []
spec: 'docs/superpowers/specs/2026-09-25-artifact-backlink-refuses-an-absolute-change-path-with-unkno-design.md'
plan: 'docs/superpowers/plans/2026-09-26-artifact-backlink-refuses-an-absolute-change-path-with-unkno.md'
results: 'docs/results/2026-09-26-artifact-backlink-refuses-an-absolute-change-path-with-unkno-results.md'
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/artifact-backlink-refuses-an-absolute-change-path-with-unkno'
pr: 'https://github.com/danielhanold/docket/pull/336'
blocked_by:
reconciled: true
claimed_at: '2026-09-26T12:19:54Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-25-artifact-backlink-refuses-an-absolute-change-path-with-unkno-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-25-artifact-backlink-refuses-an-absolute-change-path-with-unkno-design.md) |
| Plan | [2026-09-26-artifact-backlink-refuses-an-absolute-change-path-with-unkno.md](https://github.com/danielhanold/docket/blob/fix/artifact-backlink-refuses-an-absolute-change-path-with-unkno/docs/superpowers/plans/2026-09-26-artifact-backlink-refuses-an-absolute-change-path-with-unkno.md) |
| Results | [2026-09-26-artifact-backlink-refuses-an-absolute-change-path-with-unkno-results.md](https://github.com/danielhanold/docket/blob/fix/artifact-backlink-refuses-an-absolute-change-path-with-unkno/docs/results/2026-09-26-artifact-backlink-refuses-an-absolute-change-path-with-unkno-results.md) |
<!-- docket:artifacts:end -->

## Why

During change 0458's implement-next run, `docket artifact backlink --change <absolute path>` returned `unknown-change`. It only succeeded with the repo-relative path, which cost the run an extra results commit. Agents naturally pass absolute paths, so the error is surprising and the message doesn't say the path form is the issue.

## What changes

Make `artifact.backlink` validate `--change` as a canonical repository-relative path, the rule `--artifact` and `change.attach-plan`/`attach-results` already enforce. An absolute path is refused with the existing typed `absolute-path` reason. A `..` escape, an empty value, or a non-canonical spelling is refused as `path-escape`. Each message names the flag and the expected form. Only a well-formed path that matches no record still returns `unknown-change`. Also fix the `docket-implement-next` results-checkpoint prose, which leaves the path form unstated and so led the agent to build the absolute path. Add tests for the absolute, escape, and non-canonical forms and the repo-relative happy path, plus a regression test that absolute `--artifact` stays refused.

The stub proposed accepting absolute paths. Grooming decided against it: that would be a new policy that departs from every other path flag, and it would need dual-root plus symlink canonicalisation that no caller needs (see the spec).

## Out of scope

Accepting absolute paths on any flag; path handling in other operations; the gate-drive `scope-closed` issue found in the same 0458 run (tracked separately).

## Reconcile log

### 2026-09-26

2026-09-26 — Reconciled against main d1ca501b. Traced internal/app/artifact_backlink.go: ArtifactBacklink still passes req.ChangePath raw to resolveBacklinkChange with no lexical validation; containedArtifactPath and verifyAttachPath (change_attach.go) still carry the canonical-path rule the spec mirrors. Change 0458 (discovered_from) has merged and does not touch this path. Scope unchanged.
