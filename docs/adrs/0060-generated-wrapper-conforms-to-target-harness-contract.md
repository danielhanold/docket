---
id: 60
slug: generated-wrapper-conforms-to-target-harness-contract
title: A generated wrapper conforms to its target harness's own documented contract
status: Accepted
date: 2026-07-27
supersedes: []
reverses: []
relates_to: [8, 15, 17, 59]
change: 135
---

## Context

docket's agent layer generates model/effort-pinned subagent wrapper files per harness, from the
built-in sources in `agents/docket-*.md`. `sync-agents.sh`'s `emit_for_harness()` routed `codex`
to a named emitter and passed everything else — Cursor included — through the generic `emit()`,
which preserves the source's Claude frontmatter verbatim.

Cursor documents five custom-agent frontmatter fields (`name`, `description`, `model`,
`readonly`, `is_background`) and encodes reasoning effort inside the model value as
`<id>[effort=<e>]`. It has neither a standalone `effort:` field nor a `skills:` preload. docket
therefore emitted two fields Cursor ignores and one pin Cursor cannot read, while reporting all
three as honored.

The observed consequence was a live `docket-implement-next` run under Cursor in which plan,
build, review, and finish all silently degraded to their inline `auto` fallbacks and still
produced a plausible PR — the third instance of the `skill-fallback-degrades-discipline`
learning: complete-looking artifacts concealing an unrun discipline. The silent inheritance was
the mechanism: a harness token that reached the generic branch looked supported and was not.

## Decision

A generated wrapper conforms to its **target** harness's own documented contract, emitted by a
named per-harness emitter.

1. **The generic emitter is Claude's shape**, not a default other harnesses may silently inherit.

2. **The catch-all branch is a known gap.** A harness reaching it has no verified contract
   mapping — its wrapper is a best guess, not a supported shape — and the branch itself is
   documented as such at the site.

3. **Per-harness field mapping stays a pure translation.** docket keeps no allowlist of a
   vendor's model IDs or effort tokens (ADR-0015 passthrough; ADR-0059's rejection of
   vendor-internal tables), because a committed table of a vendor's internals goes stale
   silently, and a stale entry produces a false negative that reads as a successful degrade.

4. **A pin that cannot be expressed in the target contract is dropped LOUDLY at generation
   time** — never silently.

This decision **refines** ADR-0008 (the agent layer's generated subagents) and ADR-0015
(harness-portable model IDs) without superseding either, and **cites** ADR-0059 as governing the
dispatch/tiering question it does not reopen.

## Consequences

**Enables.** docket can honestly claim a pin on any harness it has actually mapped, so the
workflow discipline it advertises becomes reachable rather than nominal. Convention-level rules
delivered through wrapper *content* can now reach a Cursor child at all, which the `skills:`
preload never could.

**Costs.** One emitter per harness rather than one shared emitter, and a per-harness contract
that must be re-checked whenever the vendor's own docs move.

**Given up.** The convenience of adding a harness token and getting a plausible-looking wrapper
for free. Adding a token now means either writing its emitter or knowingly accepting the
documented gap.

**Residual.** `agents`, `kiro`, and `windsurf` remain accepted tokens on the catch-all today —
stated in the branch comment, but not yet enforced at runtime.
