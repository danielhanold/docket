---
id: 109
slug: 'docket-schema-is-a-separate-reflected-payload-schema-surface'
title: '`docket schema` is a separate reflected payload-schema surface with a fail-closed fidelity boundary'
status: 'Accepted'
date: '2026-09-03'
supersedes: []
reverses: []
relates_to: [104]
change: 399
---

## Context

`docket capabilities` (ADR-0104) is the authoritative catalog of the executable CLI surface — per-leaf argv, signature, and effect — but by design EXCLUDES request/result JSON schemas so it can hold its measured byte budget as a Step-0 read. That left an agent unable to construct a `--request`/`--input` body, or interpret a refusal, from the running binary alone: it had to fall back on `--help`, `strings`, the source tree, or a mutating probe. Change 399 adds the omitted surface as a separate read-only `docket schema` operation, derived by reflecting over the live `internal/app` `*Request`/`*Result` Go structs so the published shapes cannot drift from the binary that serves them.

## Decision

1. **Separate on-demand operation, never inlined into capabilities.** `docket schema [--operation <id>] --json` is its own catalog leaf, with its own capability id and a `read` effect. It carries its own `schema_version` integer, versioned and validated fail-closed INDEPENDENTLY of `protocol_version` and `capability_version`. Schemas are never inlined into the capabilities payload — doing so would blow ADR-0104's compact-catalog budget.

2. **Reflection fidelity boundary (the rule a future extender needs).** The reflector (`reflectDescriptor`) FAILS CLOSED on any shape it cannot describe — e.g. a map that is not `map[string]string` — rather than emitting a lossy guess. It renders `[]byte` as a JSON string, and any FOREIGN-package struct as an opaque `object` instead of recursing into a deep field tree. The consequence is deliberate: docket describes ITS OWN request/result shapes faithfully, while results embedding foreign trees (`gate.drive.*` → `gatedrive.DriveDoc`, `diagnostic.config`'s effective config → `config.Effective`) schema as opaque `object`. The surface describes docket's own payload contract, not foreign internals; this boundary was chosen over loosening the fail-closed map contract.

3. **Vocabularies live in Go, guarded by shape.** Required-ness is a co-located `docket:"required"` struct tag, proven consistent with the hand-written validators by a per-operation tag/validator agreement test. Finding codes are a typed `FindingCode` registry minted everywhere, enforced by a whole-repo grep-shape guard (never an enumerated list of spellings). Result presence and enum vocabularies are likewise co-located `docket:` tags. Catalog↔schema correspondence is guarded in BOTH directions, with counts. Exactly two honest, minimal exceptions exist and are recorded as such: `development.test` emits no protocol document (it streams the suite), and `SchemaResult` is self-referential and therefore unreflectable, so it is a bound catalog entry without a reflected binding.

4. **Capabilities byte-budget step.** Adding the mandatory one-line `schema` catalog entry stepped the capabilities payload ceiling from 13KB to 14KB (13312 → 14336 bytes) as a conscious design event, exactly as changes 0397 and 0394 stepped it for their own new catalog operations. Only the operation's invocation stub was added — schemas themselves stay out — so ADR-0104's compact-catalog invariant holds.

## Consequences

**Enables.** An agent can construct any `--request`/`--input` body and interpret any refusal from the running binary alone: real JSON keys, required markers, named vocabularies, and closed finding-code / disposition / effect sets — never from `--help`, `strings`, the source tree, or a mutating probe. The skill contract (docket-convention SKILL.md) now directs agents to resolve payload shapes from the `schema` operation. Because the document is reflected over the live structs, it cannot drift from the binary.

**Costs / gives up.** Result trees that embed foreign types are opaque in the surface (the fidelity boundary above): a caller needing one of those shapes gets `object` and no further detail. A future need to describe such a tree is met by introducing a docket-owned wrapper type or an explicit descriptor — never by loosening the fail-closed reflector to guess at unsupported shapes. The schema document may exceed the capabilities byte budget; that is acceptable because it is opt-in and on-demand, not a Step-0 read.

## Alternatives considered

**Inline the schemas into `docket capabilities`.** Rejected: it would blow ADR-0104's measured byte budget for a document every skill reads at Step 0, to serve a need that is occasional and per-operation.

**Loosen the reflector to recurse into foreign packages.** Rejected: the fail-closed contract is what makes the published document trustworthy; recursing into foreign internals would publish another package's shape as docket's payload contract and make every upstream refactor a silent schema change.

**Hand-maintain the schema document (or a separate spec file).** Rejected: a hand-written surface drifts from the binary, which is the exact failure this change exists to eliminate.
