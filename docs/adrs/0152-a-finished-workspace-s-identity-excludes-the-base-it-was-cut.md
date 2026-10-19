---
id: 152
slug: 'a-finished-workspace-s-identity-excludes-the-base-it-was-cut'
title: 'A finished workspace''s identity excludes the base it was cut from'
status: 'Accepted'
date: '2026-10-19'
supersedes: []
reverses: []
relates_to: [92, 35]
change: 567
---

## Context

workspace.prepare records BaseRef/BaseCommit in the workspace manifest. Every later workspace operation rebuilds its target from the current effective base (domain.ResolveEffectiveBase), and ownership (`ownsManifest` in internal/workspace/prepare.go) required m.BaseRef == target.BaseRef. A stacked child's effective base legitimately moves from its parent's branch to the integration branch once the parent is done (ADR-0092), so its finished checkout read as foreign: cleanup blocked, the sweep reported StateForeign, and finalize of a child whose parent landed first refused at workspace inspection.

## Decision

For a manifest in the `ready` or `cleaned` phase, ownership no longer compares BaseRef. Identity is repository common dir, workspace id, feature ref, change id, slug, and path, all still compared exactly. An `allocating` manifest (an interrupted creation) still requires the base match, because prepareResume resumes from the recorded BaseCommit and must not mix it with a different base ref. BaseRef/BaseCommit stay recorded in the manifest as history. Every work-protecting proof (registration at the exact path on the feature ref, HEAD equal to the ref tip, clean tree, non-forcing removal) is unchanged.

## Consequences

Carried stacked children can be cleaned up after their root lands, and a child whose parent landed first can be rebased/published/finalized. A reader of the manifest's BaseRef may see a ref that is no longer the change's effective base (prepareExisting reports the target's BaseRef next to the manifest's old BaseCommit). Cleanup of carried children still needs its own carry proof; ownership alone authorizes no delete.

## Alternatives considered

Rewriting the manifest's BaseRef when the parent lands (needs a write path triggered by another change's closeout, racy); comparing against the full stack ancestry of possible bases (more complex, brittle when ancestors are archived); dropping the base check for all phases (unsafe for an interrupted allocation resuming from a recorded commit).
