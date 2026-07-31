---
id: 169
slug: codex-profile-routed-build-support
title: Codex support for profile-routed Docket builds
status: proposed
priority: medium
type: feat
created: 2026-07-30
updated: 2026-07-31
depends_on: [167, 168]
related: [78, 79]
discovered_from: [167]
adrs: [64]
spec:
plan:
results:
trivial: false
auto_groomable:
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| ADRs | [ADR-0064](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0064-shipped-agent-defaults-live-in-a-harness-indexed-sidecar.md) |
<!-- docket:artifacts:end -->

## Why

Change 0167 intentionally supports Claude Code first. Codex uses TOML agent profiles,
`model_reasoning_effort`, and different native dispatch semantics; Claude model identifiers and
frontmatter cannot serve as a portable default.

Change 0168 then built the mechanism this change plugs into, and left Codex as the last harness
without a mapping. Docket now ships model/effort defaults from a harness-indexed sidecar,
`agents/harness-defaults.yml` (ADR-0064, which supersedes ADR-0048); Claude and Cursor each ship a
COMPLETE block covering all twelve wrappers, and Codex ships nothing. So a Codex repo today
generates twelve honestly-unpinned wrappers and Codex applies its own model to every docket agent.

That makes this change narrower than it looked when it was filed — most of the machinery now
exists — but it also means 0168 left obligations here that reading the Codex code alone will not
reveal. They are listed under *What changes*.

## What changes

Design and implement Codex-native `economy`, `standard`, and `premium` build profiles, connect them
to Codex task dispatch, and validate explicit overrides, automatic routing, and one-step
escalation end to end.

Three obligations inherited from change 0168 ride with it. Each is a place where 0168 deliberately
encoded "Codex ships nothing" as an assertion, so shipping a Codex block MUST update it — a red
test at these three points is the guard working, not a regression:

1. **Two Codex TOML asserts currently assert ABSENCE.** In `tests/test_sync_agents_codex.sh`:
   "no model key — nothing shipped for codex, so honestly unpinned", and "no
   `model_reasoning_effort` key either". Once Codex ships defaults they must assert the shipped
   VALUES instead. Absence-asserting was right while nothing shipped — it is what keeps a
   re-introduced cross-harness leak visible — so replace them with value asserts rather than
   deleting them.

2. **`HD_SHIPPED_HARNESSES` must gain `codex`.** In `scripts/lib/harness-defaults.sh` this list
   drives the completeness rule: every harness in it must cover every `agents/docket-*.md`. Adding
   Codex to the sidecar without adding it here would let Codex ship half-pinned — some agents
   carrying docket's judgment, the rest silently falling to Codex's own default. That is exactly
   the state rejected for Cursor during 0168's review.

3. **The example block's comment DEPTH is load-bearing.** In `.docket.example.yml`, a singly
   commented block means "mirrors a shipped default" (claude, cursor); a doubly commented one means
   "unvalidated illustration for a harness docket ships nothing for" — today, only codex.
   `tests/test_docket_example_yml.sh` asserts both depths EXACTLY. Shipping Codex defaults means
   stripping the codex block's second comment layer and re-pointing its rows at the sidecar values;
   until that happens the depth assertions fail, correctly.

## Out of scope

- Changing the shared task-worker contract established by change 0167.
- Cursor support (change 0168) or replacement of the whole-branch review skill.
- Revisiting ADR-0064's sidecar design. This change is a consumer of it: it adds a harness block
  and satisfies the existing completeness rule. If Codex needs a shape the sidecar cannot express,
  that is a new ADR, not an edit to that one.

## Open questions

- Which Codex models and reasoning-effort levels should ship for each profile? Note the shape
  difference from Cursor: Cursor's IDs encode their variant, so every cursor row ships
  `effort: auto`. Codex separates model from `model_reasoning_effort`, so codex rows are expected
  to carry real effort tokens — closer to the claude block than the cursor one.
- Should native Codex dispatch and Claude-parent runner delegation share one adapter contract?
- Does ADR-0064's provenance split hold for Codex? Point 5 says only a USER-layer value may become
  a delegated child-runner `--model`/`--effort` flag; a shipped native default must not. Codex is
  both a native harness and the one child runner docket ships (change 0079), so it is the only
  harness where those two roles meet. `RES_MODEL_FROM_USER` and `RES_MODEL_FROM_SIDECAR` already
  distinguish them, so this likely needs a test rather than new mechanism — but it needs
  confirming, because a shipped Codex default leaking into a delegated `--model` flag would be the
  same class of bug 0168's review found in `warn_fallback_model`.

## Reconcile log
