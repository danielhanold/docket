---
id: 369
slug: 'migrate-maintained-consumers-to-the-direct-go-cli'
title: 'Migrate retained lifecycle consumers to typed Go operations'
status: 'in-progress'
priority: 'critical'
type: 'refactor'
created: '2026-08-29'
updated: '2026-08-30'
depends_on: [318]
stacked_on:
related: [317, 322, 326, 361, 366, 367, 370, 371, 372]
discovered_from: [318]
adrs: [36, 74, 99]
spec: 'docs/superpowers/specs/2026-08-29-migrate-maintained-consumers-to-the-direct-go-cli-design.md'
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch: 'refactor/migrate-maintained-consumers-to-the-direct-go-cli'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-08-30T01:28:49Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-29-migrate-maintained-consumers-to-the-direct-go-cli-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-29-migrate-maintained-consumers-to-the-direct-go-cli-design.md) |
| ADRs | [ADR-0036](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0036-codex-agents-md-dispatch-block-committed-machine-neutral.md), [ADR-0074](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0074-build-gate-verdict-is-tri-state-runner-defined-non-failure-exit-is-a-halt.md), [ADR-0099](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0099-one-metadata-topology-for-go-v1.md) |
<!-- docket:artifacts:end -->

## Why

The Go v1 cutover should migrate only behavior already supported by its public typed operations.
Treating intentionally deferred Bash capabilities as missing Go verbs would expand v1 and keep this
change too large to implement autonomously.

## What changes

- Derive and classify the maintained lifecycle consumer inventory by behavioral shape.
- Migrate planning, maintenance, implementation, and metadata-only finalize consumers only where
  an exact public Go operation already exists.
- Remove caller-owned follow-up work already performed atomically by those operations, including
  proven redundant ADR-index rendering.
- Update canonical executable instructions, regenerate maintained copies deterministically, and
  add a stage-local mutation-tested guard for the migrated surface.
- Leave native agent dispatch, deferred features, the final global seal, and Bash deletion to their
  explicit dependent changes.

## Out of scope

New Go verbs or shims; native dispatch (0371); automatic capture, learning automation, terminal
publication, and the final consumer seal (0372); physical Bash deletion (0370); release and
self-host acceptance (0366); post-cutover board work (0367); and historical-record rewrites.

## Design decisions

Consumer behavior is classified before editing: existing typed Go operation, native host dispatch,
intentionally deferred capability, transaction-absorbed behavior, historical/non-executable, or
unresolved. Only the first and fourth classes move here. Reconciliation halts if an in-scope caller
needs a new operation or substantial bespoke adapter. This is a sequential merged-main dependency,
not a stacked branch.

## Reconcile log

### 2026-08-30

### 2026-08-30 — reconciled against merged-0318 reality

Claimed at version `b201c2e4`; base resolves to `main`; `depends_on: [318]` is `done`. The full typed Go CLI that 0318 (Go-only source cutover) landed is present and was probed directly — no scope drift from the spec's assumptions.

**Consumer inventory (shape-derived, canonical non-historical surfaces: skills/, agents/, cursor-rules/, CLAUDE.md, AGENTS.md, README.md; embedded copies under `internal/assets/embedded/tree/` are generated mirrors of skills/).** No live command-instruction string literals exist in Go source — every `docket.sh` hit under `cmd/`/`internal/` is a "mirrors/ports" doc comment, not a consumer.

**Class A — migrate to a landed public Go verb (invocation/structured-output swap only):**
- `gate-before` → `docket run gate-before` (CLAUDE.md, AGENTS.md, cursor-rules/run-gate.md).
- `gate-verdict` → `docket run gate-verdict` (same files).
- `render-artifact-backlink` → `docket artifact backlink` (docket-new-change, docket-auto-groom, docket-groom-next, convention/references/terminal-close-out).
- `docket-status --digest-only` → `docket status --json` (emits a `ready` array + `changes`/`findings`; docket-implement-next, docket-status).
- `cleanup-feature-branch` → `docket finalize cleanup` (terminal-close-out).
- `archive-change` → `docket finalize closeout` (terminal-close-out).
- `bootstrap` → `docket repository init` (convention CREATE_ORPHAN path).

**Class D — redundant follow-up absorbed by a transaction (remove, do not replace, after atomic-ownership proof):**
- standalone `render-adr-index` after `docket adr record/reverse/supersede` (docket-adr) — the spec's named example.

**Frozen / unmapped — no landed Go verb, left unchanged and reported (not this change):**
`preflight` (~22 sites, config-resolve + metadata-worktree sync), board-refresh (`docket-status --board-only`), `render-change-links` (the `## Artifacts` writer) — no Go verb; `terminal-publish`, `mint-stub`, `mark-publish-deferred`, `render-learnings-index` — Class C, owned by 0372; `runner-dispatch` — Class B, owned by 0371; `stack-base`/`stack-children`/`stack-closeout` — no Go verb; `backfill-change-types` — one-off, no verb; `adr-checks` — no ADR-specific verb (`repository check` is not equivalent). Several of these interleave with Class A calls inside the same file (e.g. terminal-close-out mixes `archive-change`/`cleanup-feature-branch` with `terminal-publish`/`render-change-links`; docket-status mixes digest with `--board-only`/`preflight`); the spec explicitly blesses this mixed, independently-green intermediate state.

**Abort boundary — NOT tripped.** Every in-scope op maps to a pre-existing public Go verb (verified by direct probe) or is left frozen; none requires a new/expanded operation, a bespoke compatibility adapter, native-dispatch generator work, a deferred-feature retirement, or a repository-wide absence invariant. The stage-local mutation-tested guard covers only the migrated Class A/D surface and must permit the frozen Class B/C/0370 paths. The multi-family spread (planning, maintenance, implementation, finalize, plus the parent/run-gate and setup surfaces) is invocation-adaptation, which the spec sanctions — not redesign across families.

**Primary build-time risk flagged for planning (spec sequencing step 3 — "verify every mapped public command"):** confirm `docket run gate-before`/`run gate-verdict` reproduce the Bash facade's attribution key, unattributed fallback, and retry-once accounting exactly. If a mapping turns out to need a bespoke adapter rather than a straight invocation swap, that is a halt trigger surfaced during build. All other mappings are straight swaps or the single Class D removal.

**Regeneration path:** editing a canonical skill requires regenerating the embedded mirror via `go generate ./internal/assets/` (`cmd/genassets`), proven by `internal/assets/generate_test.go`; a second generation must produce no diff. Authoritative suite gate: `go run ./cmd/docket development test`.
