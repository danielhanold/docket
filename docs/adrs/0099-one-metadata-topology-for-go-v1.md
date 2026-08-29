---
id: 99
slug: one-metadata-topology-for-go-v1
title: "One metadata topology for Go v1 (main-mode removed)"
status: Accepted
date: 2026-08-28
supersedes: [2]
reverses: []
relates_to: [1, 52]
change: 363
---

## Context

ADR-0002 made docket-mode the default while keeping a pinned `metadata_branch: main` opt-out that reproduced single-branch behavior exactly. That opt-out bought a migration runway: existing single-branch repos — docket itself included — could pin `main` instead of migrating the moment the default flipped.

The runway has been used up. Every repository-aware surface in the Go implementation has had to carry a second topology through resolution, worktree handling, publish, and close-out: two shapes of "where metadata lives", two degradation postures, two sets of tests, and a standing supply of bugs that only appear on the branch nobody runs. Change 0352 landed native `docket repository migrate`, so a legacy single-branch repository now has a first-class, in-tool exit that does not depend on a shell migration script or on staying pinned indefinitely.

With a real migration path in the tool, the compatibility mode is pure cost.

## Decision

**Go v1 supports exactly one repository metadata topology.** Planning metadata lives on the fixed orphan `docket` branch; code lands on the independently resolved `integration_branch`.

- The single-branch metadata topology formerly selected by `metadata_branch: main` is **removed as an active configuration and compatibility mode**. `metadata_branch` survives only as a **decode-only obsolete tombstone** — parsed so a legacy config is recognized rather than mis-read, and consumed as **legacy-migration input** — never as a live selector of behavior.
- **The surviving default and bootstrap rules of ADR-0002 still stand**, restated here as the operative ones: docket-mode is what a repo gets absent any configuration; `integration_branch` (`auto`→`origin/HEAD`, fallback `main`) stays the decoupled knob for *where code lands*; `.docket.yml` still lives on the repo's default branch (`origin/HEAD`), not the integration branch; and the **first-run bootstrap 2×2 guard remains in force** — no command silently restructures a repo's branches, and a half-migrated repository is still detected and refused.
- What is removed from ADR-0002 is the **pinned `metadata_branch: main` opt-out** as an escape from that guard. A repo can no longer answer the refusal by pinning.
- **Ordinary repository-aware commands refuse a legacy single-branch repository** with a typed invalid-state refusal carrying the `legacy-repository` reason and the remedy `docket repository migrate`. The refusal is typed, not prose: callers and tests key on the reason, not on wording.
- **Native `docket repository migrate` (change 0352) is the only legacy exit.** There is no pin, no flag, and no environment override that lets a single-branch repository keep operating in place.

**The rule a reader needs:** there is one topology. If a repository is not on it, the answer is always `docket repository migrate` — never a configuration value.

## Consequences

- **Enables:** one code path through resolution, worktree management, terminal publish, and close-out; tests that exercise the topology every user actually runs; a migration story with a single supported shape, so a legacy repo's state is either *migrated* or *refused with a remedy* — never a third, half-supported thing quietly limping along.
- **Costs:** the pin is gone as an escape hatch, so a legacy repository that previously kept working by configuration now hard-stops until it migrates. That stop is deliberate and loud; the remedy travels with the refusal.
- **Given up:** ADR-0002's dogfooding trade — "pin `main` and defer your own migration" — is no longer available to any repo, docket included. Deferral now means being refused, not being pinned.
- **Trade:** a one-time forced migration on the remaining legacy repositories, in exchange for deleting an entire second topology from every repository-aware surface of Go v1.
