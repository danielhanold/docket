<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0399 — Authoritative machine-readable request/result schema surface](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-03-0399-authoritative-machine-readable-request-result-schema-surface.md)**
<!-- docket:backlink:end -->

# Authoritative machine-readable request/result schema surface

## Summary

Add one authoritative, read-only, versioned schema surface to the docket binary,
derived from the live Go types, that describes everything `docket capabilities`
(change 0394, ADR-0104) intentionally omits: the **request body** of every
mutating leaf, the **result/envelope shape** of every operation, and the closed
**finding-code / disposition / effect vocabularies**. All three are required in
v1; none is optional.

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

Firsthand on 2026-09-03, in a downstream consumer repository driven from the
Cursor harness: after confirming `capabilities` carries no body schema, the only
authoritative way to build the `change.create` and `change.groom` requests was
to read `internal/app/change_create.go` and `change_groom.go` at the installed
commit, because the exact JSON key spellings (`change_id` on groom but `id` on
the lifecycle ops, `relations` nesting, `spec_markdown`) cannot be guessed from
`--help` and differ from what the finding messages teach. Reading product source
to learn a payload is precisely the failure mode 0394 ended for verbs; it
persists for payloads.

## Goals

- Make the running binary the sole authority for its request/result payload
  shapes, derived from the live Go types (no second hand-maintained copy).
- One read-only bootstrap that returns a payload schema for any operation,
  callable before repository preparation, that never mutates anything.
- Cover **request body, result/envelope shape, and the closed vocabularies**
  (finding codes, dispositions, effects) in the same surface, so an agent can
  both construct a request and interpret a refusal without reading source. This
  is a v1 requirement, not a stretch goal.
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

## Architecture

The shape below is settled (see *Resolved questions*); the build may adjust
internals but not the operation, the format family, or the coverage scope.

### A separate on-demand schema operation

Add one public, read-only leaf, `docket schema`, registered with its own
capability id and `read` effect like every other leaf, that returns protocol-v1
JSON:

- `docket schema --json` — every operation's payload schema (may exceed the
  capabilities budget; that is acceptable because it is opt-in, not Step-0).
- `docket schema --operation <id> --json` — one operation (e.g.
  `change.create`), the common agent path: fetch only the payload you are about
  to send.

Rationale for a dedicated op rather than growing `capabilities` entries: the
Go tree today has 35 `*Request` and 52 `*Result` structs in `internal/app`
(measured 2026-09-03), far more than the capabilities catalog's 12 KB budget
(ADR-0104) could carry, and 0394 deliberately kept schemas out of the always-on
Step-0 payload for that reason. Schema lookup is per-operation and occasional,
so it should be paid for only when needed.

### Live-type derivation

Derive each schema by reflecting over the actual request/result Go structs the
handlers decode into (the `internal/app` `*Request`/`*Result` types), so the
surface cannot drift from the binary. Emit, per field: the **real JSON key**,
Go/JSON type, required vs optional, nesting (e.g. `relations.depends_on`),
repeatability, and any closed enum (change types, priorities, outcomes, section
intents, effect vocabulary, finding codes). The operation id is the same stable
id `capabilities` already uses (`change.create`, `change.reconcile`, …), so the
two surfaces join on `id`.

Two facts about the current tree bound what reflection alone can see, and the
design must close both gaps in the same way 0394 closed the effect gap — with a
co-located declaration and a guard, never a second hand-maintained inventory:

- **Required-ness lives in validator code, not in struct tags.** Today a
  missing field surfaces as a finding from a hand-written validator (e.g.
  `validateLifecycleShape`), and no tag marks a field required. The request
  structs gain a co-located struct tag declaring required-ness, the schema
  reads that tag, and a test asserts for each representative op that an empty
  request's findings name exactly the fields the tag marks required, so the
  tag and the validator cannot silently disagree.
- **Finding codes are untyped string literals.** `StatusFinding.Code` is a
  plain string and codes are minted inline at roughly thirty call sites, so
  there is no type to reflect. The finding-code vocabulary is declared once as
  a typed registry in Go, all call sites mint from it, and a whole-repo
  grep-derived guard reddens on any finding-code literal outside the registry.
  Priorities, section intents, groom outcomes, and effects are already typed
  closed sets and are reflected directly; change types are brought under the
  same typed pattern if they are not already.

Format (settled): a compact docket-native typed descriptor mirroring the
capabilities entry shape, carried in the protocol-v1 envelope with its own
`schema_version` integer that consumers refuse fail-closed when unsupported,
exactly as `capability_version` works. Standard JSON Schema output is not part
of v1 (see *Resolved questions*).

### Result and vocabulary coverage (required)

The surface carries three sections, all required in v1:

- **Request** — per operation, the request-body descriptor above (absent for
  read-only leaves that take no body).
- **Result** — per operation, the result/envelope descriptor: the envelope
  fields, the operation-specific result fields, and which fields are present
  only on success or only on refusal.
- **Vocabularies** — the closed sets, emitted once and referenced by name from
  request/result fields: finding codes, dispositions, effect classes, change
  types, priorities, outcomes, and section intents.

The request body is the acute gap, but a request without its result and
vocabularies still forces an agent to read source to interpret a refusal, so
the three ship together.

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
  consistent with docket's existing typed envelopes. Rejected for v1 (see
  *Resolved questions*).
- **Keep reading `--help` / source.** The status quo — rejected: prose is
  incomplete for structured bodies, `strings`/source reading is unauthoritative
  (it shows tags, not required-ness or nesting, and not the installed build's
  validator), and probing to discover a body can mutate.

## Verification

- Round-trip: every operation that decodes a `*Request` struct appears in the
  schema surface, and every schema entry resolves to a real handler; a new
  mutating leaf without a schema entry reddens the suite (mirror 0394's
  correspondence + mutation guards).
- Field fidelity: for representative ops (`change.create`, `change.groom`,
  `change.reconcile`), assert the emitted request keys, required/optional,
  nesting, and enums match the structs.
- Result and vocabulary fidelity: for the same representative ops, assert the
  emitted result descriptor matches the `*Result` struct and the envelope, and
  that every closed vocabulary (finding codes, dispositions, effects, change
  types, priorities, outcomes, section intents) is emitted once and is
  referenced by name from the fields that use it; adding a member to a
  vocabulary without it appearing in the surface reddens the suite.
- Read-only: assert the schema op is repository-, config-, network-, and
  write-independent, and callable when repo-aware ops refuse.
- Message fixes: empty/unknown-field requests report the real JSON key and list
  accepted keys; the decode error names the actual flag (`--input`, which is the
  flag every JSON-taking command registers today).
- Required-ness and vocabulary guards: the required-tag/validator agreement
  test and the finding-code registry grep guard above are mutation-tested
  (drop a tag, mint a code inline) and redden.
- No-probe path: the schema surface plus the listed-accepted-keys error make
  discovery probes unnecessary; the cross-harness check below is the proof. A
  validate/dry-run flag is not part of this change (see *Resolved questions*).
- Cross-harness: in Cursor, build a `--request` body for a mutating op using
  only the schema surface — no `--help`, no `strings`, no source read.

## Resolved questions

Answered with the maintainer on 2026-09-03.

1. **Dedicated `docket schema` op only.** No per-command `--json-schema` flag in
   v1. One leaf keeps the envelope, versioning, and the round-trip guard in one
   place; a per-command flag would scatter N copies of that contract. It can be
   added later as sugar over the same generator if a real consumer asks for it.
2. **Docket-native typed descriptor, not standard JSON Schema.** It stays
   consistent with the protocol-v1 envelopes agents already parse, needs no
   JSON Schema draft vocabulary, and is smaller. An exporter to standard JSON
   Schema is a possible follow-on, not v1.
3. **Request + result + vocabularies together.** All three are required in v1
   (see *Result and vocabulary coverage*).
4. **Standalone.** 0360 is still an ungroomed `proposed` umbrella; this change
   ships at `critical` without waiting on it. When 0360 is groomed into a
   stack, its "skill CLI-card + schema no-mutate" leg is covered by this change
   and should be dropped there rather than rebuilt; 0360's own open question on
   schema-probe posture (validate/dry-run) is answered by point 5 below.
5. **Message-key fixes ship inside this change as its first plan tasks**, not
   as a separate change. They are cheap and independently valuable, so they
   land first on the branch, but a second change would cost more coordination
   than the fixes themselves. No validate/dry-run flag is added: with the
   schema surface and listed-accepted-keys errors an agent has no reason to
   probe, and a dry-run mode would be a second protocol surface to version and
   test. If a real need for dry-run remains after this ships, it belongs to
   0360.

## Assumptions

- Reflection over the existing `*Request`/`*Result` types, plus the co-located
  required tag and the typed finding-code registry described above, can
  faithfully represent required/optional, nesting, and enums without a second
  hand-maintained inventory.
- The stable operation id from `capabilities` is the correct join key.
- A per-operation schema fetch is an acceptable occasional cost outside the
  Step-0 budget.

## Relationship to 0360 and 0394

This change realizes the request/result schema-discovery that **0394** built the
capability catalog around but deferred, and carves out the **0360** leg its own
backlog note calls "skill CLI-card + schema no-mutate" / "split at groom into a
stack." It is intentionally standalone and narrow so it can ship at `critical`
without waiting on the rest of the 0360 umbrella. The maintainer confirmed
standalone; when 0360 is groomed, its schema leg is dropped as covered here.

**0360 is not retired by this change.** Assessed 2026-09-03: of 0360's legs,
this change subsumes exactly two — "CLI schema" and "schema probes must not
mutate" (with 0360's open question 6 on probe posture). Every other leg is
independent of schema discovery and still reproduces on the Go tree after this
change lands: context after claim, session-scoped sync, receipt `version`
chaining, reconcile no-op, lease refresh folded into long ops, the runtime card,
skipping the unconditional sweep, evidence from a PASSED drive, `.docket.local.yml`
visibility from a feature worktree, gate `FAILED` vs `exec-error`, suite argv
word-splitting (which dumps the environment into gate logs), stacked review
sizing, and the optional Go driver. 0360 therefore stays open after 399 and is
regroomed into a stack minus the two schema legs; it must not be killed on the
strength of this change alone.
