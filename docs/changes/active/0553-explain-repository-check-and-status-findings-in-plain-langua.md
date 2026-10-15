---
id: 553
slug: 'explain-repository-check-and-status-findings-in-plain-langua'
title: 'Explain repository check and status findings in plain language'
status: 'proposed'
priority: 'high'
type: 'fix'
created: '2026-10-15'
updated: '2026-10-15'
depends_on: []
stacked_on:
related: [496, 403, 352, 533, 539]
discovered_from: []
adrs: [136]
spec: 'docs/superpowers/specs/2026-10-15-explain-repository-check-and-status-findings-in-plain-langua-design.md'
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
| Spec | [2026-10-15-explain-repository-check-and-status-findings-in-plain-langua-design.md](../../superpowers/specs/2026-10-15-explain-repository-check-and-status-findings-in-plain-langua-design.md) |
| ADRs | [ADR-0136](../../adrs/0136-repair-on-a-migrated-repository-is-docket-repository-repair.md) |
<!-- docket:artifacts:end -->

## Why

`docket repository check` and `docket status` report record-level problems in internal vocabulary a human cannot act on. A real example: `[warning] drop-final-claimed-at` with the message "claimed_at on a final archived change; remove the stale claim lease" — the reader cannot tell what is wrong, whether it matters, or what to type. The worst family is record validation: `repository check` folds every one of the validator's codes into `frontmatter-manual-review` with the message `metadata corpus finding: <code>`, drops the finding's details, and attaches one generic remedy ("this shape is outside the mechanical repair roster"); `docket status` prints the same codes as a bare code plus `key=value` details and no remedy at all. The check header `repository check: no-op (healthy)` reads as a contradiction next to a listed warning, and the generated-view remedy tells the reader to hand-edit BOARD.md and the ADR index, which the convention forbids.

Daniel reads this output himself, and agents relay it; every finding should answer what is wrong, whether it matters, and what to do — without opening the source.

## What changes

- Every record-level finding `repository check` and `docket status` can show carries three plain-language parts: what is wrong (naming the specific record and value), whether it matters (explicitly "harmless" when it is), and a remedy that works as typed in the state that produced it.
- One shared app-layer table supplies that text for the validator codes that reach `status` and `check`; both commands use it. The frontmatter-repair and generated-view messages are rewritten in place under the same rules.
- `repository check` reports a validation problem under its own code (as `docket status` already does) instead of `frontmatter-manual-review`, which stays for genuine frontmatter-shape problems.
- The check header names the setup state in plain words and counts findings ("setup healthy, 1 warning (read-only, nothing was changed)").
- JSON gains an optional `impact` field; `message` keeps only the what-is-wrong sentence.
- Generated-view remedies stop suggesting hand edits of generated files.
- A coverage guard and a generated catalog file of every message make the wording reviewable in one place and keep new codes from shipping without text.

## Out of scope

- Git branch/worktree topology findings, config diagnostics, `artifact-missing`, and `branch-malformed` — already plain sentences.
- History-check (evolution) and transaction-scope codes, which never reach `status` or `check`.
- What is detected, severities, exit codes, and repair behavior.
- Any new blocking gate.
- A `--explain` flag or other verbosity switch.
- Naming the artifact-backlink codes in the docket-status skill (change 539).
