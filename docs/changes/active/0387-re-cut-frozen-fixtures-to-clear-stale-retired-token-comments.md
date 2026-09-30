---
id: 387
slug: 're-cut-frozen-fixtures-to-clear-stale-retired-token-comments'
title: 'Re-cut frozen fixtures to clear stale retired-token comments in harness-defaults and .docket.yml'
status: 'proposed'
priority: 'low'
type: 'chore'
created: '2026-08-31'
updated: '2026-08-31'
depends_on: []
stacked_on:
related: []
discovered_from: [370]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable:
branch_prefix:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

After change 0370 retired the Bash facade, comments at agents/harness-defaults.yml:7 and .docket.yml:20 still describe now-retired tokens. They cannot be truthfully corrected in isolation because both lines are byte-pinned to frozen versioned fixtures; an honest fix needs a coordinated fixture re-cut so the pinned bytes and the live comments stay in agreement.

## What changes

Coordinate a versioned re-cut of the frozen fixtures that pin agents/harness-defaults.yml and .docket.yml so the stale retired-token comments can be corrected without breaking the byte-pinned fixture assertions, then update the comments to reflect the post-0370 vocabulary.

## Out of scope

Changing the resolved default values themselves; any behavior change to config resolution.

## Open questions

- **Backlog review 2026-09-02 (Bash→Go migration)** — still valid for Docket Go; needs regrooming against the Go tree. Regroom the `.docket.yml` half: no live-vs-frozen byte compare was found for it (only the `status-corpus` fixture), so its stale `migrate-to-docket.sh` comment may be fixable directly; only the `agents/harness-defaults.yml` sidecar (byte-pinned by `internal/config/defaults_test.go` against `testdata/repositories/v0.9.3/`) needs a versioned re-cut. The record's line-number anchors should be re-anchored on quoted clauses (ADR-0054).
- **Update 2026-09-30 (from change 0473)** — three facts the regroom should start from:
  1. **The sidecar pin moved.** 0473 cut `testdata/repositories/v0.9.9/agents-harness-defaults.yml` and repointed `sidecarPath` in `internal/config/defaults_test.go` (`TestBuiltinAgentsParityWithFrozenSidecar`) to it; `v0.9.3` stays as-is (its `status-corpus` is still read). This change's re-cut is therefore a new tree after `v0.9.9`, following 0468's `v0.9.8` / 0473's `v0.9.9` precedent (new tree + `PROVENANCE.md`, repoint the path constant, never edit an existing tree).
  2. **There is a second byte pin.** `internal/assets/embedded/tree/agents/harness-defaults.yml` (plus its `manifest.json` entry) must be regenerated in the same commit; the comment in `internal/repoguard/absence_test.go` beginning "Its header comment naming the deleted scripts/lib/harness-defaults.sh" records that correcting the header reddens both pins.
  3. **More of the header is stale than `scripts/lib/harness-defaults.sh`.** The same header also cites `HD_SHIPPED_HARNESSES` (only a historical mention survives, in `internal/harness/dispatch.go`), `hd_validate`, and the Bash reader's parsing rule ("the reader consumes everything up to the next `,` or `}`"). Rewrite the whole rules block against the Go validator, not just the one path.
  - Correction to the note above: `.docket.yml` *is* now byte-pinned — `TestFixtureDocketSelf` (`internal/config/fixtures_test.go`) compares it against `testdata/repositories/v0.9.8/docket-self/repo/.docket.yml`, so its half needs a versioned re-cut too.

