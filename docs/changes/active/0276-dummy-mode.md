---
id: 276
slug: dummy-mode
title: Dummy mode — persona-calibrated human-facing language simplification
status: implemented
priority: medium
type: feat
created: 2026-08-09
updated: 2026-08-09
depends_on: []
related: []
discovered_from: []
adrs: []
spec: docs/superpowers/specs/2026-08-09-dummy-mode-design.md
plan: docs/superpowers/plans/2026-08-09-dummy-mode-plan.md
results: docs/results/2026-08-09-dummy-mode-results.md
trivial: false
auto_groomable:
branch: feat/dummy-mode
claimed_at: 2026-08-09T22:16:04Z
pr: https://github.com/danielhanold/docket/pull/190
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-09-dummy-mode-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-09-dummy-mode-design.md) |
| Plan | [2026-08-09-dummy-mode-plan.md](https://github.com/danielhanold/docket/blob/feat/dummy-mode/docs/superpowers/plans/2026-08-09-dummy-mode-plan.md) |
| Results | [2026-08-09-dummy-mode-results.md](https://github.com/danielhanold/docket/blob/feat/dummy-mode/docs/results/2026-08-09-dummy-mode-results.md) |
| PR | [#190](https://github.com/danielhanold/docket/pull/190) |
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

## Reconcile log

### 2026-08-09

Reconciled against `origin/main` @ `324d2268` and the current `docket` tip. The design holds; three
constraints from current reality are now folded into the spec, and one coupling is recorded.

- **Persona must be a single-line scalar.** `docket-config.sh`'s snapshot readers
  (`config_line_scalar_get` → `config_normalize_scalar`) parse one line and strip from the first
  `#` before unquoting. The spec's gallery shipped folded (`persona: >`) examples, which the
  resolver would read as the literal `>`. Resolution folded into the spec: a block-scalar persona
  is a hard error with a diagnostic naming the quoted form, a `#`-truncated persona is detected and
  refused rather than exported as a fragment, and the five gallery examples are rewritten as quoted
  single-line strings. Extending the shared reader for one cosmetic key was rejected — every
  skill's Step 0 runs through it.
- **`.docket.example.yml` is a guarded surface.** `tests/test_docket_example_yml.sh` enumerates
  nested keys, requires a `scope:` tag and a real consumer for each, and pins the count; adding
  three leaves means updating that count in the same commit. Added as its own implementation step.
- **Coupling — change 0258** (`in-progress`, plan committed, no implementation) touches the same
  enumerated-export assertions this change extends by three exports. Not a dependency; whichever
  lands second reconciles the enumeration.

No scope was dropped and none added. Auto-capture: nothing cleared the six admission gates — the
findings above are in-scope drift, recorded here rather than minted.
