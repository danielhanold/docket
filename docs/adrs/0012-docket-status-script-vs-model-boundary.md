---
id: 12
slug: docket-status-script-vs-model-boundary
title: docket-status script-vs-model boundary for skill passes
status: Accepted
date: 2026-06-19
supersedes: []
reverses: []
relates_to: [7]
change: 23
---

## Context

`docket-status` (and its sibling skills) runs multiple passes over the backlog: board render, merge sweep, and health checks. As the project scripted successive passes — `render-board.sh` in change 0022, the terminal-transition close-out scripts in change 0025, and `board-checks.sh` (five mechanical health checks) in change 0023 — the team repeatedly faced the same per-pass judgment call: move it into a deterministic shell script, or keep it as agent-prose? Without a durable rule, each pass re-opened the debate.

The five mechanical health checks scripted in change 0023 are: broken-spec, broken-plan-results, dependency cycle, stale in-progress, and merge-gate stall. Two checks stayed agent-driven: `blocked_by:` re-examination (requires reading free text to infer whether an external blocker has cleared) and inline board/source drift (requires judgment to reconcile prose inconsistencies).

ADR-0007 established a narrower version of this boundary for the GitHub-mirror surface: deterministic external-write mechanics live in `github-mirror.sh`; agent-prose drives it. Change 0023 generalised that single-surface rule into a principle that covers the entire skill family.

## Decision

A `docket-status` (or sibling-skill) pass moves into a **deterministic script** when it is **mechanical and free of shared terminal-transition side effects** — a pure transformation or git/gh probe whose output the model only surfaces. It **stays agent-prose** when it requires **judgment** (reading free text to infer intent, e.g. whether a `blocked_by:` external blocker has cleared), or when it drives **terminal-publish / harvest** mechanics that are **shared with `docket-finalize-change`** and must not diverge (so the script, if any, is owned jointly — as change 0025 did — never duplicated per-caller).

The script owns deterministic plumbing and is **fail-closed / warn-only**: it exits non-zero on hard errors, emits structured output on findings, and never mutates state autonomously. The model owns judgment and authors commit messages.

This generalises ADR-0007's mirror boundary from one surface (the GitHub mirror) to the whole skill family.

**Practical corollary for new passes:** ask two questions. (1) Is the check purely mechanical — no reading of intent from free text? (2) Does it avoid terminal-transition side effects shared with other callers? If both answers are yes, script it. If either is no, keep it in the model.

## Consequences

**Enables:** cheaper, deterministic, independently testable passes — a script is mutation-tested hermetically; a model turn re-sends the whole skill-and-convention context on every step. A clear, repeatable triage means future passes can be classified without reopening first-principles debate.

**Costs:** a shared terminal-transition primitive must be extracted jointly across all its call sites (the larger blast-radius that change 0025 absorbed) rather than scripted per-caller; a genuinely judgment-bearing pass must stay in-model even when most of its siblings are scripted, so the skill is a hybrid of script invocations and model judgment, giving up the simplicity of "all one or all the other."

**Rationalises prior decisions:** this boundary was already being applied by change 0011 (`github-mirror.sh`), change 0022 (`render-board.sh`), and change 0025 (the close-out scripts). ADR-0012 makes the implicit rule explicit and citable.
