---
id: 355
slug: build-review-roles-are-skill-invoked-that-fan-out-to-profile
title: 'Build/review roles are skill-invoked that fan out to profile agents — Step 5 ''dispatch'' vocabulary invites an agent-not-found misfire'
status: 'in-progress'
priority: medium
type: fix
created: 2026-08-26
updated: '2026-08-26'
depends_on: []
stacked_on:
related: [212, 257, 283]
discovered_from: [351]
adrs: [59, 63, 66, 69]
spec: docs/superpowers/specs/2026-08-26-role-skill-invocation-and-nested-agent-topology-design.md
plan: 'docs/superpowers/plans/2026-08-26-role-skill-invocation-and-nested-agent-topology.md'
results:
trivial: false
auto_groomable:
branch: 'fix/build-review-roles-are-skill-invoked-that-fan-out-to-profile'
pr:
blocked_by:
reconciled: true
claimed_at: '2026-08-26T21:52:18Z'
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-26-role-skill-invocation-and-nested-agent-topology-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-26-role-skill-invocation-and-nested-agent-topology-design.md) |
| Plan | [2026-08-26-role-skill-invocation-and-nested-agent-topology.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-26-role-skill-invocation-and-nested-agent-topology.md) |
| ADRs | [ADR-0059](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0059-dispatch-capability-resolved-not-inferred-from-tool-name.md), [ADR-0063](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0063-docket-owns-the-build-role-profile-routed-workers.md), [ADR-0066](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0066-docket-owns-the-review-role-suite-runs-in-the-build-gate.md), [ADR-0069](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0069-mode-conditioned-clause-discriminates-on-provenance.md) |
<!-- docket:artifacts:end -->

## Why

At the plan→build seam (Step 5 of `docket-implement-next`), the running skill tries to **dispatch
`docket-build` as a named agent** before falling back to invoking it as a skill:

```
⏺ docket-build(Build change 351 plan)
Error: Agent type 'docket-build' not found. Available agents: … docket-build-economy,
docket-build-max, docket-build-premium, docket-build-standard, …
```

It then self-corrects — "the build role is a skill I invoke inline, which then fans out to the
profile workers" — and proceeds via `Skill(docket-build)`. Observed live on the change 351 run
(discovered_from 351).

There is **deliberately no `docket-build` agent wrapper**: the build role is a skill that routes
each plan task to the profile workers (`docket-build-standard`/`-premium`/`-max`/`-economy`), which
*are* registered agents. The absence is by design, not a gap — so the fix is a wording/clarity
change, **not** adding an agent.

The misfire is invited by mixed vocabulary. `skills/docket-implement-next/SKILL.md` §
`Step 5 — Build` correctly says the build skill is **"invoked"** (contrast: plan-writer/status/review
say "dispatch … the subagent") — but the *same paragraph* calls it "this long build **dispatch**"
and frames the Tier-C posture as "cannot **dispatch**," and the always-in-context "Docket agents —
dispatch, don't run inline" rule primes agent-dispatch as the default posture. The rule's own
conditional already covers this ("if no same-name agent is registered, do not invent one") — the
model just failed to check it against the muddy Step 5 wording.

Cost today is one wasted error round-trip that self-heals. The latent trap is worse: Step 5 makes a
build role that **cannot dispatch** a Tier-C **authorized-or-halt** condition ("any other resolved
value is abort-and-report"). A model that reads `Agent 'docket-build' not found` as the
cannot-dispatch trigger could **falsely halt a healthy run** instead of invoking the skill inline.
This run dodged it; the wording leaves it open, and it recurs every implement run.

## What changes

Make the role/agent boundary explicit without expanding the parent-facing dispatch block:

- A resolved `skills.*` value is invoked as a skill; any nested named-agent dispatch belongs to
  that invoked skill's own contract.
- Step 5 treats `docket-build` as the built-in skill whose contract fans out to build-profile
  workers. A custom build binding owns its own topology.
- Step 6 dispatches a Docket reviewer rung only for the `docket-review` binding. A custom review
  binding returns its own findings without an additional Docket review.
- A rejected same-name role-agent attempt is the wrong operation, not Tier-C evidence. Tier C still
  applies when an invoked role's genuinely required nested dispatch is unavailable.
- Contract guards derive the generic role-invocation population and mutation-pin the built-in vs
  custom topology split.

## Design decisions

The linked spec settles the design at three layers: role bindings are skill invocations; built-in
skills own their profile/rung fan-out; and ADR-0059 classifies only failures at a required nested
dispatch boundary. `AGENTS.md`, its generators, the build/review skill bodies, routing, evidence,
and lifecycle behavior remain unchanged.

## Out of scope

- Adding a `docket-build` agent wrapper — the absence is intentional; build fans out to the profile
  workers. This change must not introduce one.
- The Tier-C authorized-or-halt safety invariant itself — clarify what does and does not *trigger*
  it, never relax it.
- Changing `docket-brainstorm`; the shared rule already describes its consultant fan-out.

## Reconcile log

### 2026-08-26

### 2026-08-26 — reconcile (implement-next)

Re-read the linked spec, related changes 212/257/283, and the cited ADRs (0059/0063/0066/0069) against current source. Findings:

- The observed defect still stands verbatim in the source. `skills/docket-implement-next/SKILL.md` Step 5 still calls the operation a "long build dispatch" while also saying the build skill is "invoked" and framing Tier C as "cannot dispatch"; Step 6 still dispatches the Docket reviewer rung unconditionally. `skills/docket-convention/SKILL.md`'s Skill layer still lacks the generic role-binding-is-a-skill-invocation rule.
- Change 212 is `done` (ADR-0069 inline-loaded-role-skill scoping already landed) — its precedent holds and needs no rework here.
- Related changes 257 and 283 are still `proposed` (unbuilt), so neither has touched the shared convention/build/review prose or the three target test files yet. No landed conflict to fold in; per the spec's relationships note, whichever of us lands second reconciles the shared anchors, and 355 is landing first.
- Scope, relations, and the five-file source list in the spec remain accurate. No obsolescence, no fundamental design invalidation. Proceeding to plan/build unchanged.

Auto-capture disabled (AUTO_CAPTURE_ENABLED=false); no follow-up stubs minted.

## Run halted

### 2026-08-27

2026-08-26 — Build-evidence gate cannot go green on this machine under the full parallel suite; a human is needed.

**State reached.** All build work for change 355 is complete and committed on `fix/build-review-roles-are-skill-invoked-that-fan-out-to-profile` (HEAD `bfddc28c`): the three role-invocation edits (through `2b341739`) plus the regenerated embedded asset bundle and the two skill-size-budget bumps (`bfddc28c`). `go generate ./internal/assets/` is idempotent over the committed tree (identical sha256). No PR opened; nothing marked implemented.

**Blocker.** The native evidence gate (`docket gate drive` over `scripts/run-tests.sh`) returned FAILED. Three of 128 files were red: `test_go_race`, `test_go_toolchain`, `test_sync_agents_claude_surface`. `docket evidence record` mints a durable record only from a PASSED drive at head, so no green evidence can be produced here — and without it the PR/mark-implemented path is correctly closed.

**Serial confirmation (per repo discipline) — all three are load-induced, not defects:**
- `internal/app` package alone: PASS in 410.8s (`go test ./internal/app/ -timeout 30m`, exit 0). Under the parallel suite, `test_go_race` (`-race`, ~2x) and `test_go_toolchain` (`go test ./...`) both drive the `internal/app` binary concurrently and each exceeds Go's hard 600s per-package default → killed → rc=1.
- `test_sync_agents_claude_surface` serially: ALL PASS. Its one red assert under load — "cycle: the sync terminates (the link walk is bounded)" — is a 20s wall-clock watchdog (line 152, 40×0.5s) blown under contention; the immediately following assert ("AGENTS.md still got its block") passed, proving the sync actually completed its work within the run, just past the 20s window.

**Not a regression from this change.** The branch diff touches ZERO Go files — `internal/app` is byte-identical to `origin/main`, and none of the three failing test files is modified by this branch. The failures reproduce on `origin/main` for the same machine-load reason.

**What a human must decide.** The evidence gate has no serial-confirmation escape hatch for Go's own per-package timeout, so green evidence requires one of: (a) run the gate on a faster / higher-core machine where the parallel `internal/app` load stays under 600s; (b) reduce `scripts/run-tests.sh` parallelism or raise the `internal/app` go-test `-timeout` in `test_go_race`/`test_go_toolchain` (separate, tracked work — NOT change 355's scope); or (c) accept the serial-confirmation evidence and open the PR manually. I did not modify unrelated test files to force a pass, and did not fabricate evidence.
