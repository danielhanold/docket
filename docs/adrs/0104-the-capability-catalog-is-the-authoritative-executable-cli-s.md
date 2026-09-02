---
id: 104
slug: 'the-capability-catalog-is-the-authoritative-executable-cli-s'
title: 'The capability catalog is the authoritative executable CLI surface'
status: 'Accepted'
date: '2026-09-02'
supersedes: []
reverses: []
relates_to: [3, 20, 36]
change: 394
---

## Context

Changes 0369/0370/0371/0377 moved every retained Docket workflow onto the native Go CLI and deleted the Bash facade, so the running Cobra tree is now the sole owner of which verbs exist. The agent-executed surfaces that drive that tree — installed skills, agents, and harness-generated copies — still carried hand-authored executable `docket <verb>` spellings, which can silently drift from the binary they invoke.

Discovery by trial is not a safe substitute. A well-formed probe can mutate metadata or an external system before the agent learns whether the verb exists, and walking `--help` burns agent context to reconstruct, unreliably, what the binary already knows exactly. There was no authoritative machine-readable bridge from the running binary to the agent instructions executing it.

## Decision

The running binary exposes one repository-independent, read-only bootstrap — `docket capabilities --json` — that walks its live production Cobra tree and emits a compact protocol-v1 catalog of every public executable leaf: a stable dotted operation id, the exact argv, a compact invocation signature, and a closed effect classification (`read` | `local-write` | `metadata-write` | `external-write` | `process-control`) co-located on each leaf's Cobra registration, never in a second command-name-to-effect map. The catalog is deterministic and byte-identical across runs, and is bounded by a measured serialized-byte budget (12 KB for the current 65-leaf tree).

THE RULE: `docket capabilities --json` is the ONLY executable Docket CLI spelling a maintained agent-executed workflow surface (`skills/`, `agents/`, `cursor-rules/`) may hard-code. Every other agent invocation names its semantic operation by dotted catalog id and constructs the executable argv from the fetched catalog entry. The shared Step-0 preamble fetches and validates the catalog — fail-closed on an unknown verb, an unsupported version, a malformed envelope, duplicate ids, invalid effects, or missing argv — before repository preparation. A shape-based repoguard test (`internal/repoguard/capability_surface_test.go`) enforces this on the maintained surfaces, permitting only the bootstrap plus a small measured set of human-typed remedy exemptions (`docket repository migrate` / `init` / `configure-tests`).

Fail-closed at the capability boundary: an unknown `capabilities` verb means the installed binary predates this contract — stop and instruct the human to update or reinstall, never fall back to `--help` or a guessed spelling. A cataloged invocation that later returns unknown-command mid-run means the binary was replaced or is inconsistent — stop.

## Consequences

Enables: the binary is the single source of truth for its own CLI surface; skills carry no parallel capability inventories; discovery is read-only, bounded, harness-neutral, and mechanically kept consistent with the tree (correspondence plus producer-mutation guards); the effect classification makes mutating and external side-effects visible at discovery time, before anything is invoked.

Costs: one extra bootstrap fetch per agent context; a co-located capability annotation (id plus effects) is required on every new public leaf, or construction and tests fail loudly; the 12 KB budget is a design ceiling — growing past it is a deliberate design event, never a silent truncation.

Gives up: hand-authored CLI spellings in workflow prose, and discovery by trial or by `--help`.

## Alternatives considered

Keep hand-authored spellings and rely on review to catch drift — rejected: nothing mechanical ties workflow prose to the Cobra tree, and the failure mode is a wrong invocation at runtime.

Let agents discover verbs by trying commands or walking `--help` — rejected: a well-formed probe can mutate metadata or an external system, and the reconstruction is unreliable and context-expensive.

Maintain a separate generated document or a command-name-to-effect map alongside the tree — rejected: a second artifact is one more thing that can drift; the effect classification is co-located on each leaf's registration precisely so it cannot.
