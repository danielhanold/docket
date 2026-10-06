---
id: 532
slug: 'private-visibility-keep-docket-out-of-prs-commits-and-shippe'
title: 'Implement private visibility for PRs, commits, and shipped files'
status: 'proposed'
priority: 'medium'
type: 'feat'
created: '2026-10-05'
updated: '2026-10-06'
depends_on: [530, 531]
stacked_on:
related: [529, 533, 334, 351]
discovered_from: []
adrs: [36, 78]
spec: 'docs/superpowers/specs/2026-10-05-private-visibility-keep-docket-out-of-prs-commits-and-shippe-design.md'
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
| Spec | [2026-10-05-private-visibility-keep-docket-out-of-prs-commits-and-shippe-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-10-05-private-visibility-keep-docket-out-of-prs-commits-and-shippe-design.md) |
| ADRs | [ADR-0036](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0036-codex-agents-md-dispatch-block-committed-machine-neutral.md), [ADR-0078](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0078-parent-facing-gate-surface-for-claude-one-physical-instructions-file.md) |
<!-- docket:artifacts:end -->

## Why

In a private-visibility repository, other people see the feature branches, PRs, commits, and code docket produces, and none of them use docket. Even with the metadata branch kept local, docket still leaves traces on shared surfaces:

- PR descriptions carry docket marker blocks and change lines.
- Finalize posts marker-bearing PR comments.
- Models habitually write change ids such as "(0507)" or "change 0612" into commit subjects and PR titles.
- Docket vocabulary can appear in the spec copy that ships with the PR.
- A repository-level instruction block or Cursor rule file names docket.

Once a leak is pushed it cannot be undone, and a rebase-merge would carry it into `main`.

## What changes

- PR descriptions in private repositories are plain prose, with no docket blocks and no change line. A finalize block is recorded on the change only, with no PR comment.
- Writing rules for grooming, the plan-writer, build and fix workers, and the PR author: no docket or `dckt` vocabulary and no change ids in specs, commits, code, or PR text.
- A leak check, in private repositories only, scans commit messages, added lines (the spec copy included), and the PR title and description for docket and `dckt` fingerprints. Matching the bare word "docket" can be switched off. The check **blocks** the feature-branch push and the PR create or edit, and the run halts with a report naming the commit or line. This is a deliberate exception to report-only checks, because a pushed leak is irreversible.
- `repository check` and `prepare` report a `docket` or `dckt` branch appearing on `origin` in a private repository. Report only.
- Repository-level dispatch blocks (AGENTS.md, CLAUDE.md) are not written in private repositories. Instead, each private repository gets a private instructions file under `.git/dckt/` holding the dispatch and run-tracker rules plus promoted lessons. A new `docket instructions` command prints it.
- Delivery is set up once per machine by `docket install`, with no rule text in either surface:
  - **Claude Code:** a user-level `SessionStart` hook loads the private instructions file automatically.
  - **Codex and OpenCode:** a static pointer block in their user-level AGENTS.md.

## Out of scope

- Content of the metadata branch, which stays private and may use docket vocabulary freely.
- Rewriting history that is already pushed.
- Shared-mode PR descriptions and comments, which are unchanged.
- Scanning for references to the host repository's own ADRs or tickets.
