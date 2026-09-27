---
id: 447
slug: 'repository-check-flags-docket-s-own-single-quoted-frontmatte'
title: 'repository check flags docket''s own single-quoted frontmatter as needing manual review'
status: 'in-progress'
priority: 'medium'
type: 'fix'
created: '2026-09-23'
updated: '2026-09-27'
depends_on: []
stacked_on:
related: [352, 191, 266]
discovered_from: [446]
adrs: [71]
spec: 'docs/superpowers/specs/2026-09-27-repository-check-flags-docket-s-own-single-quoted-frontmatte-design.md'
plan: 'docs/superpowers/plans/2026-09-27-repository-check-flags-docket-s-own-single-quoted-frontmatte.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/repository-check-flags-docket-s-own-single-quoted-frontmatte'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-09-27T11:16:57Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-27-repository-check-flags-docket-s-own-single-quoted-frontmatte-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-27-repository-check-flags-docket-s-own-single-quoted-frontmatte-design.md) |
| Plan | [2026-09-27-repository-check-flags-docket-s-own-single-quoted-frontmatte.md](https://github.com/danielhanold/docket/blob/fix/repository-check-flags-docket-s-own-single-quoted-frontmatte/docs/superpowers/plans/2026-09-27-repository-check-flags-docket-s-own-single-quoted-frontmatte.md) |
| ADRs | [ADR-0071](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0071-writer-guarantees-yaml-validity-by-construction.md) |
<!-- docket:artifacts:end -->

## Why

`docket repository check` reports 801 `frontmatter-manual-review` errors, and the count grows by about 3 with every new change record. They're false positives against docket's own writer output, and no command clears them: every finding is `repairable: false`, and the remedy says to edit the frontmatter by hand. A hand edit can't help, though, because the writer re-quotes the value by design (ADR-0071).

The cause is in `planQuote` (`internal/reposetup/repair.go`, from change 0352's repair roster). The canonical writer single-quotes string fields such as `slug: 'my-slug'`, `title: '…'` and `type: 'fix'`. The checker then:

1. Tests the raw token. Its first byte, `'`, is a YAML indicator, so `unsafeScalarShape` returns true.
2. Calls `decodesToStringLiteral`, which requires the decoded value to equal the raw token. `'fix'` decodes to `fix`, which isn't equal, so the value is classed as "ambiguous".

So any correctly quoted string field reports as needing manual review. A newly created record like 0446 gets 3 findings (`slug`, `title`, `type`) with no hand edits at all. The noise buries real findings at `error` severity, and it has already been treated as ignorable "corpus noise".

## What changes

- Treat a token that parses as exactly one single- or double-quoted YAML scalar as a well-formed string: no finding. Decide on the parsed node's style, never on the first byte.
- Keep reporting the real cases unchanged: plain (unquoted) tokens whose decoded value isn't the literal string (bare `yes`/`true`, unquoted `: `), plus malformed quoting (unterminated or trailing content).
- Guard it with an end-to-end `change.create` → zero-findings test, a writer/checker parity table over adversarial strings, and still-flagged negative cases, each mutation-checked.
- Verify on the live corpus that `frontmatter-manual-review` drops from ~909 to 0 with other finding families unchanged. The `repository migrate` preview's `[manual]` noise disappears with it.

## Out of scope

- The other large findings families (`artifact-links-stale`, `drop-terminal-claimed-at`). Those are separate issues with their own repair paths.
- Changing the writer's quoting policy (ADR-0071).
- Hand-editing existing records.

## Reconcile log

### 2026-09-27

2026-09-27 — Reconciled against origin/main e244dfd26: planQuote, unsafeScalarShape, and decodesToStringLiteral still exist in internal/reposetup/repair.go as the spec describes; no equivalent quoted-scalar helper has landed elsewhere. Scope, relations, and spec unchanged.
