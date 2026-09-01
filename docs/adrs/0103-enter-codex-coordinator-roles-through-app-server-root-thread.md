---
id: 103
slug: 'enter-codex-coordinator-roles-through-app-server-root-thread'
title: 'Enter Codex coordinator roles through app-server root threads'
status: 'Accepted'
date: '2026-09-01'
supersedes: []
reverses: []
relates_to: [36, 59, 60, 94]
change: 393
---

## Context

ADR-0036 keeps parent-facing Docket routing in a committed, machine-neutral managed block. Live VS Code Codex runs showed that an ordinary registered child may lack the top-level collaboration controls required by a role that owns nested agent sequencing, while the same role contract can dispatch its registered children when seeded into an app-server root thread. Registration alone therefore cannot encode the required launch capability.

## Decision

Add a closed harness-neutral launch posture to Docket's agent inventory. Codex repository routing recognizes the generated root-coordinator marker and enters those roles through the foreground `docket agent enter` operation. That operation resolves the same typed role contract used for TOML registration and drives `codex app-server --stdio` to create a root thread and turn with the caller's request, absolute working directory, approval policy, sandbox, model, effort, instructions, and skill inputs. Ordinary roles continue through native child dispatch. There is no child-wide delegation grant, parent relay, `codex exec`, or cross-harness fallback.

## Consequences

ADR-0036's committed routing surface remains the owner, but its managed interior now has a Codex-specific launch clause when Codex is opted in. Coordinator entry becomes an explicit runtime operation with a foreground terminal receipt and fail-closed context validation. Other harness outputs and leaf-role contracts remain unchanged. Codex app-server protocol compatibility is now a maintained integration boundary and must be covered by contract tests plus live certification.

## Alternatives considered

Grant collaboration controls to every spawned role: rejected because the host need not offer that policy and leaf roles do not require it. Keep ordinary registered-child launch: rejected because it reproduces the plan-writer failure. Add a typed parent relay: empirically viable but rejected because it creates a continuation protocol and inserts the parent into every child edge. Use `codex exec` or another harness: rejected because neither preserves the selected native root-thread contract.
