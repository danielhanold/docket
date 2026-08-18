---
slug: migration-host-builds-through-the-frozen-prior-workflow
hook: "A change that unblocks its own tooling cannot be built by the tool it fixes — drive it through a clean checkout of the last release's frozen workflow, and reconcile against the actual code state, never the change's own framing of what its dependency left undone."
topics: [migration, tooling, reconcile, docket-workflow]
changes: [322]
created: 2026-08-18
updated: 2026-08-18
promotion_state: candidate
promoted_to:
---

## Apply

When a change's whole purpose is to make the current toolchain usable — the migration-host case —
the current toolchain is by definition unable to run it. Docket's Go transaction CLI fences off
every mutation (`change`/`workspace`/`evidence`/`pr`/`run`) while its config requests capabilities
the shipped binary does not yet ship; the change that withdraws that request (0326) and the change
that adopts the legacy artifacts (0322) are exactly the ones the fence blocks. The named
`docket-implement-next` / `docket-finalize-change` agents load the *current* skill, so they inherit
the fence and halt on the first mutation (observed: 0322's first dispatch halted `unsupported-config`
before claiming anything).

The way through is a **frozen prior workflow**:

- Add a clean checkout of the last release tag (here `v0.9.2`) as a detached worktree; it supplies
  the pre-cutover **Bash** skills and `scripts/docket.sh` facade, which mutate through git+gh, not
  the fenced binary.
- Preload that checkout's `skills/docket-implement-next/SKILL.md` and `docket-convention/SKILL.md`
  **by absolute path** and route every `docket.sh` call through its `scripts/` dir
  (`DOCKET_SCRIPTS_DIR=<tag-checkout>/scripts`). Run the workflow **inline in the session** — a
  named `docket-*` subagent can only load the harness's *current* generated skill, never the frozen
  one, so the preload cannot be delegated. This is the one legitimate reason to run a docket skill
  inline rather than through its agent.
- The transient `go run ./cmd/docket` is authorized for **`development install` only** — never a
  transaction verb. Feature branches still cut from current `origin/main`.

**Reconcile against the code, not the change's Why.** 0322's "Why" said 0311 "left its
reproduction seam unwired" — true, but 0311 had actually landed the entire `development install`
command and `internal/install` engine. The framing understated what the dependency shipped, and a
plan written from the Why alone would have rebuilt an installer that already existed. Read the
dependency's merged code before planning; the change body is a snapshot from when it was drafted.

**Frozen means frozen.** Reproducing a prior release's bytes (legacy adoption, golden corpora) must
resolve its inputs from an *embedded copy* of that release's data, never the live table — HEAD's
`harness-defaults.yml` had already drifted (ADR-0096, [[tolerance-constant-calibrated-on-one-machine]]
is the timing-side cousin of the same "a constant is context-relative" trap).

## War story
- 2026-08-18 (#322, PR #217 — merged) — Bootstrap the Go development install + adopt legacy
  user-level artifacts. First dispatch of the current `docket-implement-next` agent **halted**:
  `docket change claim` refused `unsupported-config` / `deferred-capability-requested` before any
  mutation. Re-driven through a `v0.9.2` detached worktree with its Bash skills preloaded by
  absolute path and `docket.sh` routed through its `scripts/`, run inline; 10 build tasks fanned to
  profile agents, full suite green, ADR-0096 recorded, PR opened and finalized — all metadata
  mutations via the frozen Bash facade, `go run` used only for `development install`. Reconcile
  caught that 0311 had shipped the whole install engine, so the plan narrowed to the two named seams
  (install.sh bootstrapper + the `LegacyReproducer` wiring) instead of a from-scratch installer.
  The finalize merge gate rebased onto current `origin/main` (picking up 0325's flake fix) and went
  green before merge. Siblings 0326/0316/0318 will hit the same fence and take the same route.
