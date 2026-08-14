---
id: 20
slug: generated-agent-artifacts-machine-local
title: Generated agent artifacts are machine-local, never committed; `.docket.local.yml` completes the four-layer config
status: Accepted
date: 2026-07-09
supersedes: [17]
reverses: []
relates_to: [15, 19]
change: 51
---

## Context

Change 0050's live test surfaced a bug in the generation model ADR-[[0017]] established:
opting a repo into per-repo generation (via `agents:` or `agent_harnesses:`) fully **shadows**
the global `agents:` block. ADR-0017's always-full-set pass commits wrapper files resolved from
`.docket.yml` + built-ins only — never the global layer, by ADR-0008's original two-layer split
(project-level = built-in ⊕ per-repo; user-level = built-in ⊕ global). Once those wrapper files
are committed, Claude Code's native project-over-user precedence (the mechanism ADR-0008 relies
on for free layering) makes the **committed** file win over the user-level wrapper that would
otherwise carry the global value. A user who set a model/effort preference globally, expecting it
to apply everywhere, silently loses it in any repo that opts into per-repo generation.

The root tension: model/effort choices are a **machine preference** (which model a given human's
agent runs — plausibly different per device, per account, per API budget), but ADR-0017 forces
every such choice through **committed** files, exactly the layer ADR-[[0019]]'s coordination-key
fence rightly forbids the global config from writing into (a global value shaping committed files
would fail `sync-agents.sh --check` on every other clone). The generation model and the fence
were both individually correct; composed, they produce a value that can be set but never actually
takes effect once a repo opts in.

Rejected alternatives (recorded in the change 0051 spec):

- **Committed override-only fall-through** — keep committed files but make missing fields
  fall through to the global layers at generation time. Rejected: file-granularity shadowing
  remains (a repo's `agents:` block still fully owns whichever fields it sets, all-or-nothing at
  the file level rather than the field), and whether a non-Claude harness applies the equivalent
  of project-over-user precedence itself remains unverified — the open follow-up ADR-0016 flagged
  and never closed.
- **A seed command** — one-time-copy the global values into the repo at opt-in time. Rejected:
  global edits made afterward stop propagating, which is a worse footgun than the bug it fixes.
- **Docs-only** — tell users to avoid depending on globals once opted in. Rejected: the bug
  persists; it only relocates the failure from silent to documented-and-silent.
- **Hybrid committed+local** — keep the committed pass for a "team defaults" mode and add a
  parallel local mode. Rejected: two generation modes to maintain and explain, for a distinction
  (team defaults vs. machine preference) that the four-layer model below expresses more simply as
  one pass with one more layer.

## Decision

1. **Generated agent artifacts become GITIGNORED and machine-local, never committed.**
   `sync-agents.sh`'s per-repo pass still writes the full built-in agent set (ADR-0017's
   always-full-set rule) plus the Cursor dispatch rule, for every harness in
   `agent_harnesses`, but now writes them as local files under a managed `.gitignore` block
   rather than as tracked, committed wrappers. Each field resolves at generation time across
   **four** layers, most-specific first:

   `local.agents.<H>` → `local.default` → `committed.<H>` → `committed.default` →
   `global.<H>` → `global.default` → built-in

   where `committed` is `.docket.yml`'s `agents:` block and `global` is
   `~/.config/docket/config.yml`'s. The harness-first fallback within each layer (ADR-[[0016]])
   is unchanged; this decision adds two more layers above it, not a new fallback shape.

2. **A new config file, `.docket.local.yml`** — machine-and-repo-scoped, lives at the repo
   root, is gitignored, and accepts exactly the same global-able key set ADR-0019 defined
   (`skills:`, `agents:`, `auto_groom`, `finalize.*`, `board_surfaces` minus `github`,
   `agent_harnesses` scoped as ADR-0019 already scoped it) — never the fenced coordination
   keys, which stay per-repo-committed-only regardless of which local/global file names them.
   Per-field precedence is **repo-local > repo-committed > global > built-in**; a fenced key
   set in `.docket.local.yml` is warned-and-ignored, identical in posture to a fenced key set
   in the global file. Per-repo generation stays opt-in exactly as ADR-0017 defined it — a
   repo opts in via `agents:`/`agent_harnesses:` in **either** the committed or the new local
   file.

3. **A managed `# docket:generated` `.gitignore` block, owned by `sync-agents.sh`.** The
   generator writes and maintains one block (bounded by a recognizable marker, the same
   generated-block discipline the codebase already uses elsewhere) listing every path it
   generates, derived from a single harness-roster table so the ignored paths and the
   generated paths can never drift apart.

4. **One-time migration + a redefined `--check`.** `sync-agents.sh` untracks (via `git rm
   --cached`, files left on disk) any 0048/0017-era committed wrapper it finds, once, as part
   of a normal generation run. `--check` — previously "do the committed files match resolved
   config" — is redefined to **three** legs: (a) the `.gitignore` block is current
   (CI-meaningful: catches a stale roster), (b) no `docket-*` generated file is tracked by git
   (CI-meaningful: catches a re-added committed wrapper), and (c) local file content is
   fresh relative to resolved config (advisory only — machine-local staleness is a self-heal,
   never a CI failure, since `--check` on CI has no access to a contributor's
   `.docket.local.yml`).

## Consequences

- The shadowing bug class disappears entirely: no committed bytes exist for a per-repo opt-in
  to shadow a global value with, because no generated agent artifact is committed at all.
  ADR-[[0019]]'s fence becomes **moot for generation specifically** — not repealed, just
  inapplicable, since there is no longer committed generated state a global value could
  corrupt. The fence still governs every other coordination key untouched by this decision.
- ADR-[[0017]]'s by-construction dispatch-target guarantee is **kept**: the full built-in
  agent set is still generated for every listed harness, so a Cursor dispatch rule's targets
  still resolve without a missing-agent fallback to inline execution. Only *where* the files
  land (local vs. committed) changes.
- **This ADR SUPERSEDES ADR-0017's committed-generation model** — the decision that generated
  wrapper files are committed — while explicitly **keeping** three of ADR-0017's
  sub-decisions unchanged: the opt-in gate (`agents:` or `agent_harnesses:` present), the
  always-full-set rationale (agents compose, so a partial set defeats dispatch), and the
  prune scoping (strictly `docket-*` names, never touching a non-docket file). Only the
  committed-vs-local storage decision is reversed.
- **Cost, consciously accepted:** the clone-identical committed-wrapper reproducibility
  guarantee ADR-0008 introduced and ADR-0017 preserved is **retired**, not replaced. Team
  defaults for agent model/effort now live in the committed `.docket.yml` `agents:` block by
  convention only — visible in review, but no longer CI-enforced-identical bytes on every
  clone, since no bytes are committed. This is a deliberate solo-first call (Daniel,
  2026-07-09): correctness of the *value* a build runs on no longer needs byte-for-byte
  file parity to be trustworthy, and the four-layer resolution is deterministic and
  re-derivable on every machine even though it is never checked in.
- `.docket.local.yml` completes the four-layer config picture started by ADR-0019's global
  file: repo-local (uncommitted, this ADR) > repo-committed (`.docket.yml`) > global
  (`config.yml`) > built-in — the same per-field, fence-respecting resolution rule extended
  by one more layer rather than redesigned.

## Update — 2026-07-10 (change 0057)

Change 0057 broadens decision 3's managed `.gitignore` block along three axes. The decision
itself — one marker-bounded, self-healing, CI-gated block derived from a single harness roster —
**stands**; this is a non-reversing context change, recorded here rather than as a new ADR:

- **Renamed.** The marker is now `# docket:start (managed by docket — do not hand-edit)` /
  `# docket:end` (was `# docket:generated:*`). The block no longer describes only *generated*
  artifacts, so "generated" in the name was misleading. `sync-agents.sh` performs a one-time
  in-place upgrade of any lingering legacy-spelling block; `--check` leg (a) evaluates the new
  spelling and treats a legacy block as stale (its remedy — a `sync-agents.sh` run — performs
  the upgrade).
- **Broadened scope.** The block is now the single home for **all** docket-owned ignores — the
  core entries `.docket/`, `.worktrees/`, `.claude/settings.local.json`, and `.docket.local.yml`
  in addition to the per-harness generated-artifact patterns. Its emitted content stays a pure
  constant (core entries + the static roster), so the "second roster" drift risk does not
  materialise.
- **Broadened ownership.** No longer sole-`sync-agents.sh`: a shared sourceable lib
  (`scripts/lib/docket-gitignore-block.sh`) owns the mechanics (roster, markers, constant
  emitter, hardened `ensure`), and **three** writers seed/heal the block —
  `migrate-to-docket.sh` (seeds it at migration, replacing the three bare lines it used to
  append), `docket-config.sh --bootstrap` (seeds it on a fresh docket-mode repo, closing the
  prior gap where a bootstrapped repo got no ignores at all), and `sync-agents.sh` (self-heals
  with a widened trigger: opted-in, `.docket.local.yml` present, a `docket` branch present, or
  the block already present). Because the emitter is a pure constant, every writer emits
  byte-identical bytes by construction.

The four-leg `--check` shape (decision 4) is unchanged. The block mechanics are additionally
hardened against dangling/out-of-order/malformed markers (refuse-and-warn, never consume to
EOF — the outside-bytes invariant now holds even for a hand-corrupted block).
