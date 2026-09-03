---
id: 399
slug: 'authoritative-machine-readable-request-result-schema-surface'
title: 'Authoritative machine-readable request/result schema surface'
status: 'done'
priority: 'critical'
type: 'fix'
created: '2026-09-03'
updated: '2026-09-03'
depends_on: []
stacked_on:
related: [360, 394]
discovered_from: [394]
adrs: [104, 109]
spec: 'docs/superpowers/specs/2026-09-03-authoritative-machine-readable-request-result-schema-surface-design.md'
plan: 'docs/superpowers/plans/2026-09-03-authoritative-machine-readable-request-result-schema-surface.md'
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'fix/authoritative-machine-readable-request-result-schema-surface'
pr: 'https://github.com/danielhanold/docket/pull/273'
blocked_by:
reconciled: true
claimed_at:
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-09-03-authoritative-machine-readable-request-result-schema-surface-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-09-03-authoritative-machine-readable-request-result-schema-surface-design.md) |
| Plan | [2026-09-03-authoritative-machine-readable-request-result-schema-surface.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-09-03-authoritative-machine-readable-request-result-schema-surface.md) |
| ADRs | [ADR-0104](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0104-the-capability-catalog-is-the-authoritative-executable-cli-s.md), [ADR-0109](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0109-docket-schema-is-a-separate-reflected-payload-schema-surface.md) |
<!-- docket:artifacts:end -->

## Why

This session hit the exact gap change 0394 deliberately left open. `docket capabilities` (change 0394, ADR-0104) is now the authoritative catalog of the executable CLI surface — per-leaf `argv`, a compact `signature`, and an effect class — but by design it EXCLUDES request/result JSON schemas to stay within its measured 12 KB budget. 0394 deferred "complete request/result JSON-schema discovery" to change 0360, and 0360 (the implement-next coordination-tax umbrella) records the same defects firsthand: `--help` does not print allowed keys, unknown-field errors do not list them, finding messages name the wrong key (`change_id` vs the real `id`), decode errors name the wrong flag (`--request` vs `--input`), and schema probing is unsafe because a well-formed-enough probe mutates `origin/docket` or opens a real GitHub PR.

With no machine-readable schema surface, an agent that needs a request body is forced to inspect `--help`, run `strings` on the binary, or read the Go request structs in the docket source. That is exactly what happened this session: after confirming `capabilities` does not carry body schemas, the only authoritative way to recover the `change.create` / `change.groom` request fields (the exact key spellings, such as `change_id` on groom but `id` on lifecycle ops, and `relations` nesting, cannot be guessed from `--help`) was to read `internal/app/change_create.go` and `change_groom.go` at the installed commit. Inspecting help text or product source to learn a request body is the failure mode 0394 set out to end for verbs; it still exists for payloads.

## What changes

PM-altitude (design belongs in the linked spec): give docket ONE authoritative, read-only, machine-readable schema surface for everything `capabilities` intentionally omits, derived from the live Go types so it can never drift from the binary.

- The request-body field schema (names, types, required/optional, nesting such as `relations`, enums, and the real JSON key spellings) for every mutating leaf.
- Required in the same v1 surface, not optional: the result/envelope shape per operation and the closed finding-code / disposition / effect vocabularies, so an agent can interpret a refusal without reading source.
- Keep it a SEPARATE on-demand call (e.g. `docket schema [--operation <id>] --json`) so `capabilities` stays compact and 0394's byte-budget invariant holds; version it like `capability_version` and fail closed the same way.
- Update the Step-0 / skill contract so an agent resolves a request body from this surface instead of `--help`, binary `strings`, or the docket source tree.
- Make schema discovery strictly read-only (no probe can mutate), and make finding/decode messages name the real JSON keys and the real flag names.

## Out of scope

- The broader implement-next coordination-tax legs owned by change 0360 (context-after-claim, receipt `version` chaining, evidence-from-PASSED, gate argv, session-scoped sync) — only the schema-discovery and schema-no-mutate legs are carved out here. 0360 stays open after this change lands; it is not retired by it.
- Re-opening 0394's compact invocation catalog or raising its 12 KB budget to carry schemas inline.
- An MCP server/adapter, or making MCP docket's primary agent interface (explicitly rejected by 0394).
- New lifecycle operations unrelated to schema discovery.
- Rewriting historical changes, specs, plans, results, or Accepted ADR prose.

## Reconcile log

### 2026-09-03

Reconciled against the current `docket` tree at claim time. Every spec claim was re-verified firsthand and still holds:

- No `docket schema` command exists yet (`docket capabilities` carries no request/result body schemas, by design).
- `internal/app` today has exactly 35 `*Request` and 52 `*Result` structs (the spec's measured counts), confirming reflection scope.
- `StatusFinding.Code` is an untyped `string`, minted inline at many call sites — no type to reflect, so the typed finding-code registry is still needed.
- The two message-fix targets both reproduce: `validateLifecycleShape` (internal/app/change_lifecycle.go) reports `change_id must be a positive change id` (code `invalid-change_id`) even for lifecycle ops whose real JSON key is `id`; and the decode error at internal/cli/change.go:575 says `decoding --request JSON` while every JSON-taking command registers the flag as `--input`.
- `change.groom` uses JSON key `change_id` while `change.create`/lifecycle ops use `id`; `relations` nesting and `spec_markdown` are non-obvious — confirming the acute source-reading gap the change closes.

Scope, relations (related: 360, 394; discovered_from: 394; adrs: 104), and out-of-scope carve-outs (0360 stays open; not retired) remain correct. No rescope needed.
