---
id: 96
slug: legacy-reproduction-uses-a-frozen-embedded-floor
title: Legacy reproduction resolves pins from a frozen embedded v0.9.2 floor, not the live defaults table
status: Accepted
date: 2026-08-18
supersedes: []
reverses: []
relates_to: []
change: 322
---

## Context

Change 0322 fills change 0311's third ownership proof (`install.LegacyReproducer`, previously wired
`nil`) so the Go installer can adopt an exact legacy Bash (`v0.9.2`) user-level install instead of
reporting an `ownership-conflict`. Adoption is byte-exact: a target is adopted only when the bytes on
disk equal what the reproducer emits (`provenByLegacy` / `provenByLegacyInterior`).

Reproducing byte-exact v0.9.2 agent-definition output requires the resolved `(model, effort)` pins,
which v0.9.2 computed from its shipped `agents/harness-defaults.yml` floor overlaid by the user's
global `agents:` config. Two facts make the *source* of those pins a real decision:

- HEAD's `agents/harness-defaults.yml` has **already drifted** from v0.9.2's — it adds a
  `plan-writer` row per harness — and will keep drifting as shipped defaults evolve.
- If the reproducer resolved pins through the live defaults table (or the live `internal/harness/*`
  renderers), then whether a machine "is an exact legacy install" would change every time docket
  ships a new default — silently converting a previously-adoptable machine into an ownership
  conflict, or vice versa.

## Decision

The legacy reproducer is **frozen and self-contained**. It resolves pins from a frozen, embedded copy
of v0.9.2's floor — `internal/install/legacydata/harness-defaults.yml`, guarded byte-identical to the
capture input by `TestLegacyHarnessDefaultsFrozenCopy` — overlaid **only** where a field's provenance
is the user's global config layer (`config.Value.Provenance.Layer == LayerGlobal`). It never reads the
live `agents/harness-defaults.yml` and never calls the live `internal/harness/*` renderers. The agent
bodies it renders are the 16 v0.9.2 agent sources embedded under `internal/install/legacydata/`, pinned
by SHA-256 in `TestLegacyReproducer_FrozenBodyDigests`; the goldens it is validated against are frozen
under `internal/install/testdata/legacy/`, captured from the `v0.9.2` tag.

## Consequences

- **Adoption is stable across shipped-default changes.** "Is this an exact legacy install?" depends
  only on frozen v0.9.2 bytes and the user's own global config, so a future `harness-defaults.yml`
  bump can never silently break or widen legacy adoption.
- **The reproducer is CI-verifiable without the external v0.9.2 checkout** — embedded floor + embedded
  bodies + digest pins + frozen goldens make drift a red test, honoring
  `frozen-corpus-covers-what-it-contains` at the executable level.
- **A second copy of `harness-defaults.yml` now lives in `legacydata/`** and must never be "synced" to
  the live one — that would defeat the entire point. The frozen-copy guard exists to keep it byte-equal
  to the *capture input*, not to the live sidecar.
- **Only the v0.9.2 closed inventory is reproduced.** A machine installed by an even-older Bash version
  is not adopted (it falls through to the ordinary ownership-conflict path); widening the frozen corpus
  is the mechanism if that is ever needed.
