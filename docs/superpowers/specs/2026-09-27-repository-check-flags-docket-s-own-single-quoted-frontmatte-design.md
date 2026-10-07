<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0447 — repository check flags docket's own single-quoted frontmatter as needing manual review](../../changes/archive/2026-09-27-0447-repository-check-flags-docket-s-own-single-quoted-frontmatte.md)**
<!-- docket:backlink:end -->

# Repository check: stop flagging writer-quoted scalars as manual review — design

**Change:** #447 · **Type:** fix · **ADRs:** ADR-0071 (cited, not changed)

## Problem

`docket repository check` reports `frontmatter-manual-review` (severity `error`, `repairable: false`) for every correctly quoted string field in a change record. At grooming time (2026-09-27) the live corpus showed **909** such findings — **897** single-quoted and **12** double-quoted tokens — and a corpus-wide classification found **zero** genuine plain-scalar manual-review cases among them. Every new record adds roughly three more (`slug`, `title`, `type`).

Root cause, in `planQuote` (`internal/reposetup/repair.go`):

1. `unsafeScalarShape(token)` inspects the raw token's first byte; `'` and `"` are YAML indicator bytes, so every quoted token reads as "unsafe shape".
2. `decodesToStringLiteral(token)` then requires the decoded string to equal the raw token bytes. `'fix'` decodes to `fix` ≠ `'fix'`, so the token is classed "ambiguous decoded value" and a manual-review finding is emitted.

The same `PlanRepairs` path feeds the `repository migrate` preview (`internal/app/repository_migrate.go`, the `mr.diagnostics` loop), which prints one `[manual]` line per false positive. The existing tests (`TestRepairQuoteScalarEligible` / `TestRepairQuoteScalarRefused` in `internal/reposetup/repair_test.go`) contain no already-quoted token, which is how the gap shipped.

ADR-0071 makes the writer single-quote unsafe scalars by construction; the checker must agree with the writer, not the other way round.

## Decision: recognise a well-formed quoted scalar by its parsed YAML style

Rejected alternatives:

- **First/last-byte heuristic + clean decode** — still a byte pattern standing in for a property (learning `byte-pattern-guard-matches-a-spelling`); its behaviour on `'a' # x` or trailing content depends on how the value span is cut.
- **Expose scalar style on `document.Field`** — cleanest data model, but widens a shared package interface for a single caller (YAGNI).

### Fix

Add an unexported helper in `internal/reposetup/repair.go`:

```go
// wellFormedQuotedString reports whether token, decoded on its own, is exactly
// one single- or double-quoted YAML scalar — a well-formed string whose value
// is unambiguous regardless of its indicator-byte first character.
func wellFormedQuotedString(token string) bool
```

It unmarshals the token into a `yaml.Node` and returns true only when: the decode succeeds; the document holds exactly one node, and it is a `yaml.ScalarNode`; and its `Style` has `yaml.SingleQuotedStyle` or `yaml.DoubleQuotedStyle` set. Trailing content (`'a' b`), an unterminated quote (`'abc`), or any non-scalar yields false.

`planQuote` calls it immediately after the `ShapeInline` gate and, when true, returns no finding (`RepairFinding{}, false`) — before `unsafeScalarShape`/`decodesToStringLiteral` run. A double-quoted token with escapes (`"a\tb"`) is accepted: it is an unambiguous string even though its decoded text differs from its bytes.

Unchanged:

- `unsafeScalarShape`, `decodesToStringLiteral`, and the manual-review finding's message, severity, and remedy for genuine plain-scalar cases (bare `true`/`yes`, unquoted `: `, etc.).
- `buildRepair`'s `RepairQuoteScalar` guard — a quoted token never reaches it now.
- `frontmatterFinding` in `health.go`, the migrate preview code, and the writer.

Knock-on (no code): `repository check` drops the false-positive family; the `repository migrate` preview loses its `[manual]` noise lines.

## Tests

All in the normal suite; each positive guard is mutation-checked (revert the fix → it must go red).

1. **End-to-end writer path.** A test (at the `change.create` app layer, following the existing app-test harness) that creates a change whose title contains an apostrophe, `: `, ` #`, a leading `-`, and the word `yes`, reads the written record back, runs `reposetup.PlanRepairs` over it, and asserts **zero** findings. Red on revert.
2. **Writer/checker parity table** (`internal/reposetup`). For each adversarial string — empty, `yes`, `true`, `it's`, `a: b`, `x #y`, `-lead`, `[x]`, `:30`, a non-ASCII string, and one containing a character that forces double-quoting if the writer ever emits it — build a record whose `title` is written through the canonical writer (`document.String` via the same `applyValue`/builder path the writer uses), then assert `PlanRepairs` returns no finding. Red on revert.
3. **Still-flagged cases** added to `TestRepairQuoteScalarRefused` / a manual-review assertion: bare `true` stays a manual-review finding; unterminated `'abc` and trailing-content `'a' b` are **not** silently accepted (they still produce a finding or an undecodable-record finding, whichever the parser yields — assert non-empty findings for the field/record). These must stay green on revert and must go red if the helper is loosened to a first-byte check (mutation: replace the helper body with `token[0] == '\'' || token[0] == '"'`).
4. Existing `TestRepairQuoteScalarEligible` cases (bare `yes`, `:30`) stay repairable — unchanged behaviour.

## Verification (results artifact, not a test)

Run `docket repository check --json` against the live corpus with the built binary and record: `frontmatter-manual-review` count before (≈909+) → after (expected **0**), and the other finding families (`artifact-links-stale`, `drop-terminal-claimed-at`, …) unchanged in count.

## Out of scope

- `artifact-links-stale` and `drop-terminal-claimed-at` findings (separate repair paths).
- Changing the writer's quoting policy (ADR-0071).
- Hand-editing existing records.
- Adding a scalar-style field to `document.Field`.
