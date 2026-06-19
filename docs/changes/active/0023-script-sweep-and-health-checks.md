---
id: 23
slug: script-sweep-and-health-checks
title: Decide and apply scripting vs model-driven for the merge sweep and health checks
status: in-progress
priority: medium
created: 2026-06-18
updated: 2026-06-19
depends_on: [22]
related: [11, 18, 24, 25]
adrs: []
spec: docs/superpowers/specs/2026-06-18-script-sweep-and-health-checks-design.md
plan:
results:
trivial: false
auto_groomable:
branch: feat/script-sweep-and-health-checks
pr:
blocked_by:
reconciled: false
---

## Why

`docket-status` has three passes. Change 0022 scripts the first one (the
`inline` board render) because it is pure, judgment-free transformation. The
other two — the **merge sweep** and the **health checks** — also run as
model-driven prose today, and the same token-cost and determinism arguments
*may* apply, but they are not as clean-cut:

- The **merge sweep** is a terminal-transition driver: per merged PR it archives
  the change on `metadata_branch`, runs the **terminal-publish** copy onto the
  integration branch, removes the feature branch + worktree, and best-effort
  **harvests learnings**. Its idempotent archive add is mechanical, but the
  side effects (terminal-publish, harvest) are entangled with
  `docket-finalize-change` and may be better left agent-driven.
- The **health checks** are mostly mechanical probes (broken `spec:`/`plan:`/
  `results:` links, `depends_on` cycles, stale-branch age, human-merge-gate
  stalls, board/source drift) — but at least one (`blocked_by:` re-examination
  to see if an external blocker cleared) is genuinely judgment-bearing.

So this is a **decide-then-apply** change: with 0022's rendering script and its
shared dependency-resolution core in place, determine *per pass* whether it
should be scripted or stay model-driven, and implement that decision. Splitting
it from 0022 keeps the clean, high-value rendering extraction unblocked while we
deliberate the messier two passes.

## What changes

Designed at brainstorm 2026-06-18 — see
[the spec](../../superpowers/specs/2026-06-18-script-sweep-and-health-checks-design.md).
Decisions:

- **Shared helper (resolved):** dependency resolution and frontmatter parsing
  live in **one sourced helper script** (working name
  `scripts/lib/docket-frontmatter.sh`), introduced by 0022 and reused here:
  `field`/`list_field`/`has_section` (lifted verbatim from `github-mirror.sh`)
  plus a single `resolve_deps`. `github-mirror.sh` migrates onto it, so the
  extraction *reduces* parser count rather than adding one. Frontmatter parsing
  **stays hand-rolled** — `yq` would not simplify the flat-scalar / single-line-list
  shape these passes read (the nested-config case is 0018's, decided "keep
  as-is"); so 0023 does not depend on 0018.
- **Health checks → script the mechanical ones.** A new `scripts/board-checks.sh`
  (sourcing the helper) runs broken-link resolution, `depends_on` cycles,
  stale-`in-progress` age, and the human-merge-gate stall, printing findings;
  `docket-status` invokes it, then runs the **one** judgment-bearing check
  (`blocked_by:` re-examination) in-model. No auto-fix (unchanged).
- **Merge sweep → close-out mechanics now owned by change 0025.** The §5b blocker —
  "scripting it correctly means routing both finalize *and* the sweep through one
  shared archive helper" — is exactly what **0025** builds (`archive-change.sh` +
  `terminal-publish.sh` + `cleanup-feature-branch.sh`), *and* 0025 rewires the sweep
  call-site to invoke them. So this change no longer scripts the sweep's archive /
  terminal-publish / cleanup; it **consumes 0025's scripts** there. What remains for
  the sweep is the **merged-PR detection** (a mechanical `gh` query — script-or-keep
  is a small call settled at build) and the **harvest** (judgment — stays
  model-driven). 0023 and 0025 both touch `docket-status`'s sweep prose, so the
  implementer's reconcile pass must align against whichever lands first.
- Record the boundary ("mechanical & side-effect-free ⇒ script; judgment or
  shared terminal-transition ⇒ agent-prose") as an **ADR** (generalizes 0007).
- `tests/test_board_checks.sh`, matching `tests/test_github_mirror.sh`.

## Out of scope

- The `inline` board render — owned by change 0022 (the dependency).
- **Retiring/downgrading the inline board/source-drift check** once rendering is
  deterministic — spun out to change **0024**.
- Scripting the merge sweep's **close-out mechanics** (archive / terminal-publish /
  cleanup) — owned by change **0025** (the shared helper §5b asked for); this change
  consumes them, it does not restate them.
- Changing *what* the checks flag or the sweep's terminal-publish / harvest
  contract — this change moves work between model and script, not behavior.
- The `github` surface (already scripted) and adopting `yq` (change 0018).

## Open questions

Resolved at brainstorm — see the spec. None blocking; build-ready.

**Scope update 2026-06-19.** Change **0025** was created to extract the shared
terminal-transition close-out mechanics (the §5b helper). That resolves the spec's
"merge sweep → stays model-driven (deferred §5b)" decision: the sweep's close-out is
now scripted by 0025, which also rewires the sweep call-site. The spec (§5b) still
records the *old* deferral and wants a **light re-groom** at build time to match this
body — the change is now **health checks + boundary ADR**, consuming 0025 for the
sweep's mechanics rather than deciding them. Not blocking; flagged for the reconcile
pass.

## Reconcile log
