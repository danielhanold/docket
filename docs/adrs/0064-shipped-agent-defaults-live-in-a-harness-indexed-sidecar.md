---
id: 64
slug: shipped-agent-defaults-live-in-a-harness-indexed-sidecar
title: Shipped agent model/effort defaults live in a harness-indexed sidecar; wrapper templates carry no model floor
status: Accepted
date: 2026-07-31
supersedes: [48]
reverses: []
relates_to: [15, 16, 60, 63]
change: 168
---

## Context

Before change 0168, all twelve `agents/docket-*.md` wrapper SOURCE files carried `model:`/`effort:`
frontmatter, so each source file played two roles at once: a behavioral template describing what the
agent does, and the shipped store of Claude's model/effort defaults. `sync-agents.sh` translates
those sources into a wrapper for every configured harness, so the second role leaked across the
first: a non-Claude harness with no override silently received a Claude model ID. That is worse than
being unpinned — an incompatible ID can select a fallback model while the generated wrapper reads as
confidently pinned, so the failure is invisible at exactly the surface an operator would inspect.

ADR-0048 held `.docket.example.yml`'s commented `agents.claude` block to three invariants, the first
of which points the mirror at `agents/docket-*.md` wrapper frontmatter and states that "the wrappers
remain the single source of truth." Change 0168 relocates that source of truth into
`agents/harness-defaults.yml`, which falsifies the invariant's target while leaving the rule itself
sound — the same relocation pattern by which ADR-0048 superseded ADR-0039. The other two invariants
(resolver fidelity, must-update-as-new-keys-are-added) are untouched by the relocation but would be
orphaned if this ADR merely retired ADR-0048, so they are restated here verbatim in force.

## Decision

Shipped agent model/effort defaults live in a sparse, harness-indexed sidecar,
`agents/harness-defaults.yml`; behavioral wrapper templates carry no cross-harness model floor.

**The sidecar's shape.**

1. **Every entry nests under a CONCRETE harness.** A harness-neutral `default:` block is forbidden
   in the sidecar. Such a block is precisely the cross-harness leakage this change removes: a value
   authored once and inherited by harnesses nobody validated it against.
2. **Sparse by harness and by agent, never by field.** The table is complete for Claude (all twelve
   agents) and deliberately partial for Cursor (its three build-profile workers only), with no Codex
   block until change 0169. A listed entry supplies BOTH `model` and `effort`; there is no
   half-specified entry.
3. **`runner:` is forbidden in the sidecar.** Delegation is user policy, never a shipped default.
4. **Lowest layer.** Resolution stays field-by-field per ADR-0016, with the sidecar as the LOWEST
   layer — below machine-local, repo-committed, and global user config.
5. **The resolver retains PROVENANCE, not just value.** Only a value originating in a USER layer may
   become a delegated child-runner `--model`/`--effort` flag. A shipped native default configures a
   native wrapper and is never evidence that the same ID means anything to a child harness.
6. **An unsupported harness/agent pair is generated UNPINNED** — an honest supported shape in which
   the target harness picks its own default — rather than inheriting a foreign ID.
7. **Structural validation only, run before any wrapper is written.** Model IDs and effort tokens
   remain opaque passthrough values (ADR-0015): shape is checked, vendor allowlists are not.

**ADR-0048's three invariants, carried forward.** `.docket.example.yml` remains a hand-maintained
canonical config reference held trustworthy by three invariants, enforced together by
`tests/test_docket_example_yml.sh`:

1. **The mirror rule, re-pointed.** `.docket.example.yml`'s commented `agents.claude` block mirrors
   **`agents/harness-defaults.yml`** — no longer `agents/docket-*.md` wrapper frontmatter. The
   sidecar is now the single source of truth; the mirror never leads. A reader who finds the two
   disagreeing trusts the sidecar. The block still ships **commented**, not active, because
   `agents:` and `agent_harnesses:` are presence-sensitive: `sync-agents.sh` branches on whether the
   `^`-anchored key is present at all, so an active-but-empty block would change behavior rather
   than merely document a default.
2. **The example IS the resolver's defaults — test-enforced.** Every other key ships **active** at
   its shipped default, and the guard proves byte-fidelity by feeding the example file to the real
   resolver and diffing its `--export` output against the no-config export. This is what makes
   "copy this file" safe advice: an unedited full copy into a repo's `.docket.yml` is a no-op.
3. **The must-update rule.** Every new config flag lands in `.docket.example.yml` — value, plus
   documentation, plus scope tag — in the **same PR** that introduces it, backed by a completeness
   guard driven off the resolver's actual `--export` surface, an explicit allowlist for the schema
   keys the resolver does not export, and an inverse orphan-key check.

## Consequences

- A wrapper SOURCE file becomes purely behavioral. Reading `agents/docket-status.md` no longer tells
  you what model ships for it; that answer moves one file over, to `agents/harness-defaults.yml`.
  The indirection is the price of not shipping a Claude ID to a Cursor wrapper.
- A newly supported harness starts honest rather than pinned. Until someone authors its block, its
  wrappers generate unpinned and the harness picks its own default — a visible, supported shape
  instead of a silent fallback behind a confident-looking pin. Codex is exactly this case until
  change 0169.
- Provenance tracking is real machinery the resolver must now carry: value alone is no longer
  sufficient state, because the same string means "configure this native wrapper" from the sidecar
  and "pass this flag to a child harness" from a user layer. A resolver that forgot provenance would
  reintroduce cross-harness leakage through the delegation path instead of the generation path.
- Sparseness is load-bearing and must not be "completed" for tidiness. Filling in Cursor's remaining
  nine agents, or adding a `default:` block to avoid repetition, would restore precisely the
  leakage this ADR removes.
- Structural-only validation keeps ADR-0015's passthrough posture: a typo'd model ID still ships. The
  sidecar narrows the blast radius (it is one reviewable table rather than twelve frontmatter
  blocks) without claiming to catch bad IDs.
- ADR-0048's residual risk is inherited unchanged: key presence and exported values are mechanically
  enforced, the surrounding English is not.
