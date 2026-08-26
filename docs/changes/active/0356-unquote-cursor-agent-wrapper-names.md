---
id: 356
slug: unquote-cursor-agent-wrapper-names
title: 'Emit unquoted name: on Cursor agent wrappers'
status: 'in-progress'
priority: high
type: fix
created: 2026-08-26
updated: '2026-08-26'
depends_on: []
stacked_on:
related: [135, 235]
discovered_from: []
adrs: [60, 71]
spec:
plan:
results:
trivial: true
auto_groomable:
branch: 'fix/unquote-cursor-agent-wrapper-names'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-08-26T21:08:45Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| ADRs | [ADR-0060](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0060-generated-wrapper-conforms-to-target-harness-contract.md), [ADR-0071](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0071-writer-guarantees-yaml-validity-by-construction.md) |
<!-- docket:artifacts:end -->

## Why

Cursor's Task `subagent_type` enum is built from each custom-agent wrapper's `name:` field. Docket's Cursor renderer (`internal/harness/cursor/cursor.go` `quoteYAML`) unconditionally single-quotes that field, so generated files look like `name: 'docket-implement-next'`. Cursor then registers the enum value **including the quotes**. Dispatching the unquoted name `docket-implement-next` is rejected (`Invalid enum value … received 'docket-implement-next'`). Dispatch only works if the caller passes the quoted token.

Observed live 2026-08-26 while dispatching implement-next from a consuming repo: first launch failed on the enum; retry with `'docket-implement-next'` launched. Every docket Cursor wrapper in `~/.cursor/agents/` has the same quoted `name:`.

`quoteYAML`'s own comment says quoting is for free text docket did not author. Wrapper **names** are docket-authored `docket-*` identifiers and are valid bare YAML scalars. Unconditional quoting here is the wrong contract for Cursor, which does not unquote the scalar before putting it in the Task enum (ADR-0060: a generated wrapper must match the target harness's contract). ADR-0071's always-quote rule still applies to free-text fields such as `description:`.

## What changes

Emit Cursor wrapper `name:` as a bare scalar (`name: docket-implement-next`), not a single-quoted string. Leave `description:` quoted. Update the Cursor golden wrappers and the `HasPrefix` assert in `internal/harness/cursor/cursor_test.go` that currently requires `name: '…'`.

## Out of scope

- Changing Claude/OpenCode/Codex wrapper quoting unless they share the same enum-raw-token bug.
- Changing `quoteYAML` for descriptions or other free-text fields.
- A Cursor product fix for not unquoting YAML scalars (workaround is still to emit what the harness consumes).
- Broader ADR-0071 revisiting for mint-stub / change-file writers.

## Reconcile log

### 2026-08-26

Reconciled against current source. `internal/harness/cursor/cursor.go` still unconditionally single-quotes the wrapper `name:` via `quoteYAML(s.Name)` on the `name:` line of `renderAgent`, exactly as described; `quoteYAML`'s comment still frames quoting as being for free text docket did not author. All 18 Cursor goldens under `internal/harness/cursor/testdata/golden/` open with `name: '<name>'`, and `cursor_test.go` asserts `HasPrefix(content, "---\nname: '"+name+"'\n")`. Scope holds unchanged: emit `name:` as a bare scalar, keep `description:` quoted, regenerate the goldens, and relax the test assert. ADR-0060 (wrapper must match the target harness's contract) and ADR-0071 (always-quote free text) remain the governing decisions. No rescope; still trivial.
