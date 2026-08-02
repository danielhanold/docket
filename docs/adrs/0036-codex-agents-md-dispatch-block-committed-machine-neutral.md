---
id: 36
slug: codex-agents-md-dispatch-block-committed-machine-neutral
title: Codex AGENTS.md dispatch block is committed and machine-neutral
status: Accepted
date: 2026-07-15
supersedes: []
reverses: []
relates_to: [15, 17, 20]
change: 77
---

## Context

docket's agent layer generates model/effort-pinned subagent wrappers per harness.
ADR-0020 established that generated agent artifacts — the per-harness wrappers AND the
Cursor `docket-dispatch.mdc` rule — are GITIGNORED and machine-local: they bake resolved
model IDs, which are per-machine and must not be committed.

Codex CLI needs two new things (change 0077): (1) `.codex/agents/docket-*.toml` wrapper
files, and (2) a dispatch mechanism. Codex has no analog of Cursor's `.mdc` rule, but it
reads a repo-root `AGENTS.md`. The question this decision resolves: should the Codex
dispatch instruction (the `AGENTS.md` block) follow ADR-0020's gitignored/machine-local
regime like the Cursor rule, or be committed?

## Decision

The Codex `.toml` wrappers ARE gitignored/machine-local — they bake model IDs, so ADR-0020
applies to them unchanged. But the `AGENTS.md` dispatch block is COMMITTED and
MACHINE-NEUTRAL: it carries only agent names + delegation prose, NEVER a model ID or
reasoning-effort value. The pins live exclusively in the gitignored `.toml`. The block is
therefore clone-identical across machines and belongs with the committed managed
`.gitignore` block, NOT with the machine-local wrappers.

It is maintained with the SAME hardened managed-block machinery as the `.gitignore` block
(closed-block guard, idempotence, outside-bytes-preserved), reusing the
marker-parameterized primitives in `scripts/lib/docket-gitignore-block.sh` via new generic
`ensure_managed_block` / `remove_managed_block` helpers. Its per-agent content is derived
from each built-in agent's own `description:` frontmatter (single source), assembled over
the same `agents/docket-*.md` glob as the Cursor rule (ADR-0017's full-set principle), in
`LC_ALL=C` order for cross-machine byte-determinism (it is byte-compared by
`sync-agents.sh --check`).

It is committed because `AGENTS.md` is a shared, team-visible convention file (often
hand-maintained), and a machine-neutral dispatch instruction is reproducible config, not a
machine-local artifact.

## Consequences

- Enables Codex to honor the model/effort pins — it delegates directly-invoked docket
  skills to the pinned agent — without committing any per-machine model ID. This is the
  Cursor inline-quirk problem, solved Codex's way.
- `sync-agents.sh --check` gains a presence/currency leg for the committed `AGENTS.md`
  block (CI-meaningful, symmetric with the `.gitignore` leg), while the block is EXEMPT
  from the tracked-file leg (it is meant to be committed). The `.toml` wrappers stay under
  the machine-local regime (tracked-file leg + orphan prune + advisory content-staleness).
- Accepted tradeoff: `ensure_docket_gitignore_block` was left byte-identical (proven path)
  rather than refactored onto the new generic `ensure_managed_block`, so ~6 lines of
  block-rewrite orchestration are duplicated once. Chosen deliberately to protect the
  proven gitignore path over strict DRY.
- User-level `~/.codex/AGENTS.md` dispatch (vs project-level) is deliberately deferred to
  change 0078's live-Codex validation.

## Update — 2026-08-02 (change 0192)

Scope widened from Codex-only to **any AGENTS.md-dispatch harness**. Both decisions above
stand unchanged: the project-root `AGENTS.md` dispatch block is still COMMITTED, and still
MACHINE-NEUTRAL (no model ID, no reasoning-effort value). What changed is who it serves.

opencode also reads the repo-root `AGENTS.md`, so the set of harnesses that dispatch through
that one file is now a list — `AGENTS_MD_DISPATCH_HARNESSES`, today `codex opencode`. The
block is a **single harness-neutral block shared by every harness in that list**, written
once. A repo targeting either harness gets it; a repo targeting both gets it exactly once.
The per-harness alternative (one block per harness, or a harness-scoped block) was rejected
because both harnesses read the same single file, so per-harness blocks would collide in a
repo targeting both.

Consequences of the widening:

- The block's prose names **no harness's artifact path and no harness's model vocabulary**.
  It addresses "the hosting harness's native named-agent dispatch" instead. This is a real
  expressiveness loss, accepted deliberately: the block can no longer name a generated
  wrapper file path or a harness's effort vocabulary. The cost is bounded — the block is
  committed into consumer repos and byte-checked by `--check`, so a harness-specific claim
  in it would *ship* to every consumer rather than merely display locally.
- Write-vs-strip logic, the `--check` staleness leg, and the machine-neutrality invariant
  are unchanged. Removal is now keyed to the list, not to one harness: the block is stripped
  only when the **last** AGENTS.md-dispatch harness is de-listed.
- `sync_codex_agents_md_dispatch` was renamed `sync_agents_md_dispatch` to match.
- The gitignored/machine-local regime for per-harness pinned wrappers (ADR-0020) is
  untouched; opencode's wrappers follow it exactly as Codex's `.toml` files do.
