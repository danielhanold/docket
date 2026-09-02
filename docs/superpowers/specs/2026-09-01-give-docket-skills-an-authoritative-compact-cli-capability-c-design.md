<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0394 — Give Docket skills an authoritative compact CLI capability catalog](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-02-0394-give-docket-skills-an-authoritative-compact-cli-capability-c.md)**
<!-- docket:backlink:end -->

# Authoritative Compact CLI Capability Catalog Design

## Summary

Docket operating skills will learn the running Go binary's complete executable CLI surface through one stable, repository-independent, read-only JSON bootstrap before any repository operation. The binary will derive a compact catalog from its live Cobra tree, and maintained workflow instructions will resolve executable spellings from that catalog instead of hard-coding commands, trying alternatives, or inspecting `--help`.

The catalog is deliberately an invocation catalog, not generated documentation: all executable leaf commands, exact paths and compact signatures, stable semantic operation identities, closed effect classifications, binary identity, and a capability-contract version. It excludes request/result schemas, examples, remedies, tutorials, and group-level duplication. The current catalog must fit within a measured 12 KB serialized-byte budget. Complete request/result schema discovery remains change 0360's responsibility.

## Problem

Changes 0369, 0377, and 0370 moved retained workflows onto native typed Go operations and deleted the Bash facade. The running Cobra tree now owns which CLI verbs exist, while installed skills and their harness-specific generated copies still carry hand-authored executable spellings. Change 0360 records the resulting drift as one contributor to implement-next coordination cost, but the architectural gap affects every Docket workflow: there is no authoritative machine-readable bridge from the running binary to the agent instructions executing it.

Cursor makes the gap visible. During groom and implement runs, agents try candidate commands and inspect `--help` to reconstruct the binary's surface. Other harnesses may hide the same behavior rather than avoid it. Probing is not merely inefficient: commands that accept a sufficiently valid request can mutate metadata or external systems. A correct interface must make discovery read-only, bounded, harness-neutral, and mechanically consistent with the executable tree.

## Goals

- Make the running binary the sole authority for its executable CLI surface.
- Give every Docket workflow one stable, read-only bootstrap that works before repository preparation.
- Return the complete catalog so skills carry no parallel capability inventories or filtered requirement lists.
- Bound startup context with a compact, measured payload rather than help prose or exhaustive schemas.
- Classify command effects so discovery never obscures mutation or external side effects.
- Migrate maintained agent-executable workflow instructions away from hard-coded CLI paths.
- Fail closed on absent, malformed, incompatible, or internally inconsistent capability data.
- Prove tree correspondence, effect coverage, payload size, generated-asset fidelity, and cross-harness behavior mechanically.

## Non-goals

- An MCP server or adapter, or making MCP Docket's primary agent interface.
- Complete request/result JSON-schema discovery, validation-only modes, or schema-rich help; those remain with change 0360.
- Per-skill capability filters, requirement manifests, or binary-owned workflow-to-command mappings.
- Workflow-policy, lifecycle, topology, dispatch, or transaction redesign.
- New lifecycle operations unrelated to capability discovery.
- Rewriting historical changes, specs, plans, results, archived records, or Accepted ADR prose.

## Architecture

### Stable bootstrap

Add one public leaf command:

```text
docket capabilities --json
```

This is the only executable Docket CLI spelling maintained operating skills may hard-code. The operation:

- requires no repository and performs no Git discovery;
- loads no repository or global configuration;
- requires no compatible installed asset bundle;
- performs no network, filesystem, process-control, metadata, or external write;
- emits exactly one protocol-v1 document on stdout; and
- remains callable even when ordinary repository-aware operations would refuse.

The response carries `protocol_version`, `operation`, `result`, a separately versioned `capability_version`, running binary version/revision identity, shared global invocation metadata, and a deterministically ordered `commands` array.

A representative entry is:

```json
{
  "id": "change.reconcile",
  "argv": ["docket", "change", "reconcile"],
  "signature": "--input <file> [--repo-dir <dir>]",
  "effects": ["metadata-write"]
}
```

The exact field names may follow existing protocol naming conventions during implementation, but the semantic content and closed shapes above are acceptance requirements.

### Live-tree derivation

Build the production Cobra root completely, then walk that live tree. Include every public executable leaf exactly once. Exclude the root, command groups, hidden commands, and disabled/hidden completion machinery. Sort entries by stable operation identity so identical binaries emit byte-identical command ordering.

Command paths, positional usage, flags, defaults, and repeatability derive from the registered Cobra command and pflag data. Required flags must be represented as typed command metadata rather than inferred from prose such as `(required)`. Shared global flags are represented once at the document level rather than repeated in every entry.

Stable operation identity and effects live on the leaf command registration itself, preferably through typed Cobra annotations or an equivalent co-located structure consumed by the walker. They must not live in a second hand-maintained command-name map. A new executable leaf without complete capability metadata makes construction or the suite fail loudly.

### Effect model

Each public leaf carries one or more values from a closed vocabulary:

- `read`
- `local-write`
- `metadata-write`
- `external-write`
- `process-control`

Multiple values are allowed because an orchestrating command can cross more than one boundary. Effect metadata describes possible effects, not the disposition of one invocation. The catalog operation itself is always `read`.

### Compactness boundary

The catalog is a machine invocation surface, not help output. Include only:

- stable operation identity;
- exact argv path;
- compact positional/flag signature;
- effect set;
- binary and capability-contract identity; and
- shared global invocation metadata needed to execute entries correctly.

Exclude long and short help prose, examples, tutorials, request/result schemas, disposition explanations, remedies, configuration reference material, and group entries. Do not repeat shared definitions per command.

For the current 65-leaf production tree, compact JSON must serialize to no more than 12 KB. The build results must record the actual byte count and an explicitly labeled Fable-token estimate; bytes are the gating, tokenizer-independent oracle. Growth beyond the ceiling is a design event, not a reason to silently truncate entries or introduce per-skill filters.

### Workflow consumption

Revise the shared Step-0 preamble:

1. Load `docket-convention` as today.
2. If the current agent context does not already carry a validated catalog, invoke the stable capability bootstrap and validate the closed response.
3. Run `repository.prepare` using the catalog entry, then continue with the existing typed context and workflow posture.

A catalog may be reused by later skills in the same agent context. A separately dispatched agent has an independent context and fetches its own catalog. No persistent cache file, repository metadata, environment transport, or harness-specific state is introduced.

Skills continue to own workflow policy: when to prepare, select, claim, reconcile, sweep, halt, publish, or finalize. They refer to semantic operations in their procedure and construct the executable invocation from the fetched entry. They do not carry a separate requirements block, filtered capability request, or binary-owned skill-to-command mapping.

Migrate the canonical skills and maintained references that an agent executes. Derive the inventory from a whole-source search and classify executable instructions separately from explanatory prose and immutable history. Regenerate embedded assets and harness outputs through their existing owners. Historical records retain the command spellings that were true when written.

### Failure posture

Fail closed at the capability boundary:

- An unknown `docket capabilities` means the installed binary predates the contract. Stop and instruct the human to update or reinstall Docket; do not inspect `--help`.
- Refuse unsupported capability versions, malformed envelopes, duplicate operation IDs, invalid effect values, missing executable argv, and incomplete signatures.
- If a workflow reaches a semantic operation absent from the catalog, follow that workflow's existing hard-error posture. Never guess another spelling.
- If an invocation resolved from a validated catalog later returns `unknown-command`, treat the binary as replaced or internally inconsistent during the run and stop. Do not silently refetch and change interfaces mid-workflow.
- If an operation's cataloged effects exceed the workflow's authorized boundary, stop with a capability-mismatch diagnostic.

This change creates no new lifecycle status. Claimed workflows use their existing halt/report behavior when available; an incompatibility that prevents the existing durable halt operation is surfaced explicitly rather than worked around.

## Verification

### CLI and catalog tests

- Walk the production tree in both directions: every public executable leaf appears exactly once, and every catalog entry resolves to that same leaf.
- Prove hidden commands, command groups, root, and completion machinery are absent.
- Assert deterministic ordering and byte-identical repeated output.
- Assert exact projection of positional usage, required/optional/repeatable flags, defaults, and shared `--json` behavior.
- Assert the closed effect vocabulary and complete effect metadata on every leaf.
- Mutation-test the guards: add an unclassified leaf, remove a catalog entry, remove effect metadata, change a required flag, and break the population floor; each mutation must redden.
- Assert the current serialized payload is at most 12 KB and emit its measured size in test/build evidence.
- Assert the bootstrap is repository-, configuration-, asset-, network-, and write-independent.

### Skill and generated-surface tests

- Derive the maintained agent-executable CLI-literal inventory from the whole source tree before editing.
- Migrate canonical operating skills and directly executed references to the capability-first Step 0 and semantic operation references.
- Add a shape-based guard that permits the capability bootstrap but rejects other hard-coded Docket CLI paths on maintained workflow surfaces; do not hand-list today's sites.
- Regenerate embedded skills and harness-generated artifacts through the canonical generators, then prove byte-equivalence and idempotent regeneration.
- Preserve human-facing CLI documentation that is not an agent-executed workflow and preserve all immutable historical records.

### Cross-harness acceptance

- In Cursor, run `docket-status` and one metadata-writing groom/new-change path using the installed artifacts. Neither run may invoke `--help`, try alternative commands, inspect binary strings, or issue discovery probes.
- Repeat at least one representative workflow in another supported harness to prove the capability bridge is harness-neutral.
- Record which harness versions/modes were tested; harness observations are version- and mode-scoped.

## Alternatives considered

### Static version-matched command cards installed with skills

Rejected as authority. Generated cards can still drift when the binary and machine-local installed assets move independently. They also preserve a second versioned representation of the CLI.

### Exhaustive startup catalog with full request/result schemas

Rejected for this change. The current tree has 35 request structs, 48 result structs, and 391 JSON-tagged fields; loading them all was estimated at roughly 12,000 or more tokens before accounting for Fable's tokenizer. That context cost is disproportionate and overlaps change 0360.

### Per-skill filtered catalogs or requirement manifests

Rejected because they recreate the correspondence problem. Each skill would again define the exact feature set it expects, requiring guards to keep the declaration synchronized with both its procedure and the binary.

### Docket MCP server as the primary agent interface

Rejected for this change. A primary MCP interface requires a cross-harness installation, discovery, permission, lifecycle, and fallback design. An MCP server that merely returns shell commands adds a second call without removing shell invocation. No thin MCP adapter is included here.

### Continue using `--help`, errors, or trial invocations

Rejected. Help is prose-oriented and incomplete for structured requests; error-driven discovery wastes context; trial invocation is unsafe on mutating or externally visible operations.

## Acceptance criteria

1. The running binary emits a deterministic, complete catalog of every public executable leaf through one stable read-only bootstrap.
2. The current catalog is at most 12 KB serialized and contains no exhaustive request/result schemas or help prose.
3. Every leaf has exact invocation shape and complete closed effect metadata co-located with its registration.
4. Maintained Docket workflows fetch and validate the catalog before repository preparation and no longer hard-code other Docket CLI paths.
5. Missing or inconsistent capabilities stop the workflow without `--help`, guessed spellings, or probe invocations.
6. Correspondence and mutation tests prove new, removed, or incompletely classified commands cannot drift silently.
7. Embedded and generated harness artifacts are current and reproducible.
8. Cursor plus at least one other harness complete representative acceptance runs without command discovery probing.
9. Change 0360 remains the owner of complete request/result schema discovery.
10. No MCP server, MCP adapter, or MCP-primary-interface work lands in this change.

## Assumptions

- Every supported harness can invoke the local `docket` binary and retain a compact JSON tool result in the agent context.
- The current executable surface can be represented faithfully within the 12 KB byte ceiling without omitting invocation-critical information.
- Stable operation identity and effect metadata can be attached to Cobra leaf registration without creating a second command-name inventory.
- Existing generators remain the correct owners for embedded skills and harness-specific artifacts.
- The running binary is not replaced during one workflow; a cataloged-command mismatch is therefore exceptional and correctly fail-closed.
- Request/result schema drift is independently addressed by change 0360; this change does not need to solve it to eliminate verb discovery and guessed command paths.
