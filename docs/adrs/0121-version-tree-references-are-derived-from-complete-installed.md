---
id: 121
slug: 'version-tree-references-are-derived-from-complete-installed'
title: 'Version-tree references are derived from complete installed state, and uncertainty always retains'
status: 'Accepted'
date: '2026-09-16'
supersedes: []
reverses: []
relates_to: []
change: 323
---

## Context

Version collection reclaims managed version trees under the installer's data root after an upgrade or an uninstall. Doing that safely is hard for three reasons. First, the installed population is heterogeneous: a repository may carry the tree the current AssetSetID names, plus legacy trees from earlier installs and mixed-version trees left by scoped upgrades that replaced only some targets. Second, the evidence is indirect: what a tree is referenced by lives partly in the recorded install state (the AssetSetID, each recorded target Path and LinkTarget) and partly in the live filesystem, where a still-symlink target's actual destination is the only thing that says which tree is really in use. Third, both sources can be wrong or incomplete: a state file can be missing a field, a path can be malformed or unreadable, a symlink can point through several hops or into a path whose ancestors are themselves symlinks, and two plausible readings can disagree about which version root a path descends from. Collection's action is deletion, so an error in the reference set is not a degraded result but destroyed installed state.

## Decision

The reference set is derived afresh, on every collection pass, from the COMPLETE installed ownership state rather than from any single field or cached summary: the recorded AssetSetID, every recorded target Path, every recorded LinkTarget, and — for each target that is still a symlink — its observed live destination. Each candidate reference is canonicalized before it is interpreted: existing ancestors are resolved, and every symlink hop is followed. A canonicalized path is recognized as a reference to a version tree only when it is a strict descendant of exactly one immediate non-symlink version root; a path that is a version root itself, that descends from none, or that could be read as descending from more than one, is not a recognized reference.

The load-bearing rule is how the derivation behaves when it cannot see clearly. Missing, malformed, unreadable, or ambiguous evidence resolves to RETAIN — the tree in question is never collected. And when resolution itself cannot be completed, the derivation returns an ERROR rather than an empty or partial reference set, so an unresolvable system can never present as "nothing is referenced." The destructive branch is structurally unable to fire at the moment the system is least understood.

## Consequences

Collection can safely reclaim both the current tree's predecessors and legacy or mixed-version trees left by scoped upgrades, because the reference set spans the whole ownership record rather than only the current asset set. A referenced tree is never deleted, even when the evidence for the reference is degraded.

The cost is deliberate asymmetry. Collection under-reclaims rather than over-reclaims: a tree whose evidence is damaged is kept forever until a human or a repaired state makes it resolvable, so disk can be held by trees nothing actually uses. The error-rather-than-empty rule also means a broken install surfaces collection as a failure instead of silently doing nothing — which is the intended signal, but it does mean callers must have a non-fatal posture for that failure (see the post-commit best-effort decision recorded alongside this one).

Maintainers extending the installer must preserve two invariants when adding a new kind of target or link: the new evidence has to be folded into the reference derivation (an unconsidered reference source silently makes trees collectible), and every new failure path must resolve to retain-or-error, never to "skip this candidate and continue."

## Alternatives considered

Derive references from the current AssetSetID alone. Rejected: it cannot see legacy or mixed-version trees, so a scoped upgrade that left some targets pointing at an older root would let collection delete a root that is still in live use.

Trust recorded paths without canonicalization. Rejected: a recorded path and a live symlink destination can spell the same tree differently, and unresolved symlink hops make a genuinely referenced root look unreferenced.

Treat unreadable or ambiguous evidence as "not a reference" and continue. Rejected: this makes the destructive branch most aggressive exactly when the system is least understood — the failure mode this decision exists to prevent.

Return an empty reference set when resolution fails. Rejected: an empty set is indistinguishable from a correctly-resolved "nothing is referenced," which would authorize collecting everything.

Cache a previously computed reference set. Rejected: the live filesystem is part of the evidence, so a cached set can be stale in the one direction that deletes data.
