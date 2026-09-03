<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0399 — Authoritative machine-readable request/result schema surface](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0399-authoritative-machine-readable-request-result-schema-surface.md)**
<!-- docket:backlink:end -->

# Authoritative machine-readable request/result schema surface

## Summary

Add one authoritative, read-only, versioned schema surface to the docket binary,
derived from the live Go types, that describes everything `docket capabilities`
(change 0394, ADR-0104) intentionally omits — first and foremost the **request
body** of every mutating leaf, and ideally the result/envelope shape and the
closed finding/disposition/effect vocabularies.

`capabilities` answers *which verbs exist and how to invoke them*. This surface
answers *what payload each verb takes and returns*. An agent must be able to
learn a request body from the binary itself — never from `--help`, `strings` on
the binary, or the docket source tree.

## Problem

`docket capabilities` (0394) is now the sole authority for the executable CLI
surface: per-leaf `argv`, a compact `signature`, and a closed effect class. To
stay within its measured 12 KB budget it **excludes** request/result schemas by
design, and 0394 explicitly deferred "complete request/result JSON-schema
discovery" to change 0360.

Change 0360 (the implement-next coordination-tax umbrella) records the concrete
consequences firsthand, all still reproducible on the Go tree:

- `--help` for a `--request`/`--input` command does not print the allowed keys.
- Unknown-field errors do not list the accepted keys.
- Finding *messages* name the wrong key — an empty request to a lifecycle op
  reports `change_id must be a positive change id` while the real JSON key is
  `id`; the caller then sends `change_id` and hits unknown-field.
- Decode errors say `decoding --request JSON` for a flag actually named
  `--input`.
- Skill prose lists `depends_on` as top-level while the request nests it under
  `relations`, and describes a "dated" reconcile-log entry while the CLI already
  prepends `### YYYY-MM-DD`.
- **Schema probing mutates.** A well-formed-enough JSON body to `reconcile`,
  `halt`, or `pr publish` is a live write today: probing `{}`, heading
  enumeration, and `{title,body}` applied on `origin/docket` and *created* a
  GitHub PR; the retry returned `contended`.

Firsthand this session (2026-09-03, scp-qarch-deploy, Cursor): after confirming
`capabilities` carries no body schema, the only authoritative way to build the
`change.create` and `change.groom` requests was to read
`internal/app/change_create.go` and `change_groom.go` at the installed commit —
because some builds use compacted JSON keys (e.g. `l` for `spec_markdown`) that
cannot be guessed. Reading product source to learn a payload is precisely the
failure mode 0394 ended for verbs; it persists for payloads.

## Goals

- Make the running binary the sole authority for its request/result payload
  shapes, derived from the live Go types (no second hand-maintained copy).
- One read-only bootstrap that returns a payload schema for any operation,
  callable before repository preparation, that never mutates anything.
- Preserve 0394's compact invocation catalog and its byte budget: schemas are a
  **separate, on-demand** surface, not inlined into `capabilities`.
- Version the schema contract independently and fail closed on absent /
  malformed / incompatible data, exactly like the capability contract.
- Make finding and decode messages name the real JSON keys and the real flags.
- Migrate the Step-0 / skill contract so agents resolve a request body from this
  surface instead of `--help`, `strings`, or source.

## Non-goals

- Re-opening the compact invocation catalog (0394) or raising its 12 KB ceiling
  to carry schemas inline.
- The other implement-next coordination-tax legs owned by 0360 (context after
  claim, receipt `version` chaining, evidence-from-PASSED, gate argv,
  session-scoped sync).
- An MCP server/adapter or MCP-as-primary-interface (rejected by 0394).
- New lifecycle operations unrelated to schema discovery.
- Rewriting historical records or Accepted ADR prose.

## Architecture (recommended; grooming/build may adjust)

### A separate on-demand schema operation

Add one public, read-only leaf — recommended `docket schema` — that returns
protocol-v1 JSON:

- `docket schema --json` — every operation's payload schema (may exceed the
  capabilities budget; that is acceptable because it is opt-in, not Step-0).
- `docket schema --operation <id> --json` — one operation (e.g.
  `change.create`), the common agent path: fetch only the payload you are about
  to send.

Rationale for a dedicated op rather than growing `capabilities` entries:
0394 measured that loading all schemas at startup (35 request structs, 48 result
structs, 391 JSON-tagged fields) was ~12k+ tokens and deliberately kept it out
of the always-on Step-0 payload. Schema lookup is per-operation and occasional,
so it should be paid for only when needed.

### Live-type derivation

Derive each schema by reflecting over the actual request/result Go structs the
handlers decode into (the `internal/app` `*Request`/`*Result` types), so the
surface cannot drift from the binary. Emit, per field: the **real JSON key**
(including compacted spellings), Go/JSON type, required vs optional, nesting
(e.g. `relations.depends_on`), repeatability, and any closed enum (change types,
priorities, outcomes, section intents, effect vocabulary, finding codes). The
operation id is the same stable id `capabilities` already uses, so the two
surfaces join on `id`.

Format options to settle at groom (see Alternatives): a docket-native typed
descriptor (consistent with the capabilities envelope) vs. standard JSON Schema
per operation. Recommendation: a compact docket-native descriptor mirroring the
capabilities entry shape, with an explicit `schema_version`.

### Result and vocabulary coverage

Minimum viable scope is the **request body** (that is the acute gap). Strongly
desired in the same surface: the result/envelope shape per operation and the
closed vocabularies (finding codes, dispositions, effects) so an agent can
interpret a refusal without reading source.

### Read-only and message fixes

- The schema op performs no Git discovery, config load, network, or write, and
  stays callable when repository-aware ops would refuse (same posture as
  `capabilities`).
- Independently: make decode/validation messages name the JSON key and the flag
  that was actually passed, and have unknown-field errors list accepted keys.
  This closes the "message teaches the wrong key" defect even before an agent
  fetches the schema.

### Workflow consumption

Extend the Step-0 / skill contract: an agent that must construct a `--request`
body resolves it from `docket schema --operation <id>` and never from `--help`,
binary `strings`, or the docket source. Historical records keep the spellings
that were true when written.

## Alternatives considered

- **Grow `capabilities` entries with an inline schema per leaf.** Rejected as
  the default: breaks 0394's compact-catalog invariant and 12 KB budget, and
  reloads schemas on every Step-0.
- **Per-command `--json-schema` flag** (e.g. `docket change create
  --json-schema`). Viable and discoverable, but scatters the contract across N
  commands and duplicates envelope/versioning logic; a single `docket schema`
  op centralizes it. Could be offered as sugar over the same generator.
- **Standard JSON Schema output.** Maximally interoperable; heavier and less
  consistent with docket's existing typed envelopes. Decide at groom.
- **Keep reading `--help` / source.** The status quo — rejected: prose is
  incomplete for structured bodies, `strings`/source reading is unauthoritative
  across builds (compacted keys), and probing to discover a body can mutate.

## Verification

- Round-trip: every operation that decodes a `*Request` struct appears in the
  schema surface, and every schema entry resolves to a real handler; a new
  mutating leaf without a schema entry reddens the suite (mirror 0394's
  correspondence + mutation guards).
- Field fidelity: for representative ops (`change.create`, `change.groom`,
  `change.reconcile`), assert the emitted keys, required/optional, nesting, and
  enums match the structs — including any compacted key.
- Read-only: assert the schema op is repository-, config-, network-, and
  write-independent, and callable when repo-aware ops refuse.
- Message fixes: empty/unknown-field requests report the real JSON key and list
  accepted keys; decode error names the actual flag.
- No-mutate probe: a discovery-shaped request to a mutating op cannot apply
  (covered by validate/dry-run or by the schema surface removing the need to
  probe).
- Cross-harness: in Cursor, build a `--request` body for a mutating op using
  only the schema surface — no `--help`, no `strings`, no source read.

## Open questions

1. Dedicated `docket schema` op vs. per-command `--json-schema` vs. both.
2. Output format: docket-native typed descriptor vs. standard JSON Schema.
3. Scope for v1: request-only, or request + result + vocabularies together.
4. Relationship to 0360: keep this standalone, stack it on a 0360 split, or
   fold it in as the "schema no-mutate / skill CLI-card" leg when 0360 is
   groomed into a stack.
5. Should the message-key fixes ship as a fast, independent sub-step ahead of
   the full schema surface (they are cheap and independently valuable)?

## Assumptions

- Reflection over the existing `*Request`/`*Result` types can faithfully
  represent required/optional, nesting, and enums without a second inventory.
- The stable operation id from `capabilities` is the correct join key.
- A per-operation schema fetch is an acceptable occasional cost outside the
  Step-0 budget.

## Relationship to 0360 and 0394

This change realizes the request/result schema-discovery that **0394** built the
capability catalog around but deferred, and carves out the **0360** leg its own
backlog note calls "skill CLI-card + schema no-mutate" / "split at groom into a
stack." It is intentionally standalone and narrow so it can ship at `critical`
without waiting on the rest of the 0360 umbrella; if the maintainer prefers, it
can instead become a stacked child of a 0360 split or be folded back in.
