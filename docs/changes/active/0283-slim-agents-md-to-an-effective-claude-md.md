---
id: 283
slug: slim-agents-md-to-an-effective-claude-md
title: 'Slim AGENTS.md to an effective, lean always-in-context file'
status: proposed
priority: medium
type: docs
created: 2026-08-09
updated: 2026-08-09
depends_on: []
related: [263, 154]
discovered_from: []
adrs: []
spec: docs/superpowers/specs/2026-08-09-slim-agents-md-to-an-effective-claude-md-design.md
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
| Artifact | Link |
|---|---|
| Spec | [2026-08-09-slim-agents-md-to-an-effective-claude-md-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-09-slim-agents-md-to-an-effective-claude-md-design.md) |
<!-- docket:artifacts:end -->

## Why

AGENTS.md is the always-in-context rules file, so every line of it taxes every session's context
window. Many of the learnings it carries have since been codified elsewhere — enforced by shell
scripts, tests (e.g. change 0263's guards over the shell rules), or docket skills — so restating
them in the always-loaded file buys nothing and dilutes the rules that genuinely must fire
unprompted. Claude Code's guidance on writing an effective CLAUDE.md
(https://code.claude.com/docs/en/best-practices#write-an-effective-claude-md) recommends keeping
the file lean: only universally applicable, non-derivable rules belong there.

## What changes

- Audit every AGENTS.md rule for a *landed* repo-wide enforcement surface (test/guard/script);
  remove exactly those, each removal citing and mutation-verifying its enforcing guard.
- Rules whose guards are designed but unbuilt (the 0263 set) stay until those guards land.
- The dispatch table stays verbatim; the run gate keeps all steps (wording may tighten).
- Removed war-story content lands in `docs/changes/learnings/` unless already covered there or
  in an ADR.

## Out of scope

- Adding new rules or new guards; this is a pruning pass only.
- Changing the promotion mechanics or the learnings ledger itself.

## Open questions

- **Backlog review 2026-09-02 (Bash→Go migration)** — still valid for Docket Go; needs regrooming against the Go tree. Re-run the audit against the Go guards (`internal/repoguard`); decide the fate of the `## Shell` rules now that only `install.sh`, release-smoke, and the POSIX downloader suites are shell. The rebuild-after-merge and run-gate blocks are new since the spec.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
