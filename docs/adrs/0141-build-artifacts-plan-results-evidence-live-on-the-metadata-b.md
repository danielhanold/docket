---
id: 141
slug: 'build-artifacts-plan-results-evidence-live-on-the-metadata-b'
title: 'Build artifacts (plan, results, evidence) live on the metadata branch; the spec ships with the PR'
status: 'Accepted'
date: '2026-10-06'
supersedes: []
reverses: []
relates_to: [1, 66]
change: 530
---

## Context

Plan and results files rode the feature branch into main, build evidence lived in a marker block in the PR description, the spec lived only on the docket branch, and close-out pushed a backlink commit straight to main outside any PR. Results commits after the test gate moved the feature head and staled evidence.

## Decision

For every repository, plan and results files are written to the docket metadata branch at their existing paths by `change.attach-plan` / `change.attach-results` metadata transactions carrying the Markdown. Build evidence is a `## Build evidence` section of the change record (same codec block), written by `change.mark-implemented`, `finalize.publish` and `evidence.recertify`, and read by the finalize merge gate, clear-block, the rebase no-op skip and `run.verify`; `pr.publish` verifies but writes no evidence.

The feature branch's first commit is a copy of the spec (`workspace.commit-spec`) so the spec merges with the code; the PR description carries only the backlink and absolute plan/results links. Same-branch links are relative; legacy done/killed records without a Build evidence section keep their absolute plan/results rows.

No operation pushes the integration branch outside a PR merge (guarded).

Retired: `artifact.backlink`, the `Docket-Plan-Path` trailer, the integration-branch backlink legs, and `finalize.skip_results_only_delta` (obsolete tombstone). There is no compatibility read of PR-body evidence; in-flight changes finish on the previous binary.

## Consequences

The PR diff is the spec copy plus code. Results checkpoints never stale evidence. There are no direct pushes to main. Private visibility mode becomes possible. Cost: more metadata commits per run, and a cutover ordering constraint.

## Alternatives considered

- Private-only relocation: rejected; one location serves both modes.
- Evidence in frontmatter: rejected; the board reads the header, and evidence re-mints.
- Keep the PR-body evidence read for compatibility: rejected.
