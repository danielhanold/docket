---
id: 276
slug: dummy-mode
title: Dummy mode — persona-calibrated human-facing language simplification
status: proposed
priority: medium
type: feat
created: 2026-08-09
updated: 2026-08-09
depends_on: []
related: []
discovered_from: []
adrs: []
spec: docs/superpowers/specs/2026-08-09-dummy-mode-design.md
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
| Spec | [2026-08-09-dummy-mode-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-09-dummy-mode-design.md) |
<!-- docket:artifacts:end -->

## Why

Docket's generated prose regularly asks the human questions they cannot answer as written:
the vocabulary is too dense (internal docket terms, shell idioms) and assumes programming-
language expertise the reader may not have. The human's reliable workaround — "simplify
that question" — works every time, so it should be a configured behavior, not a per-turn
request. The simplification is for the human only; agent-facing artifacts must keep full
technical density or the build loop itself degrades.

## What changes

A `dummy_mode:` config key (`enabled`, `persona` — a free-text description of the reader,
the calibration mechanism — and an optional `surfaces:` narrowing list), resolved through
the normal config layers and exported by `docket-config.sh`. Five eligible surfaces:
interactive dialogue and end-of-run reports get **replaced** (written persona-calibrated);
results files, PR bodies, and the needs-you/terminal change sections get an **additive**
`### In plain terms` block authored alongside the technical content. A shared definition in
`docket-convention` owns the token table, semantics, and the agent-safety rule (the plain
block is never a decision input); eligible skill bodies carry one-line pointers; docs ship
a gallery of 3–5 worked persona examples spanning application types and languages. A blank
persona falls back to a shipped default (mid-level engineer, architecture-literate,
working-level in any given language, all project jargon glossed), and a human can enable
dummy mode ad-hoc for the session ("enable dummy mode" at groom/brainstorm time) even where
config leaves it off — config persona, session duration, no writes.

## Out of scope

Numeric intensity levels; per-surface persona overrides; simplifying script-rendered views
(BOARD.md, mirrors, index READMEs); the spec file, plans, learnings, or any agent-facing
artifact; non-English translation; any change to agent behavior (selection, grooming,
building, reviewing).

## Open questions

- Whether `DUMMY_MODE_SURFACES` exports `all` literally or pre-expanded to the token list
  (implementation's choice; state it in the script contract).
