---
slug: commented-default-is-no-default
hook: "A default that ships commented out is not a default — and for a config that is deliberately sparse, decide whether sparseness means 'which entries appear' or 'how much of one entry appears'."
topics: [config, defaults, guards]
changes: [168]
created: 2026-07-31
updated: 2026-07-31
promotion_state: candidate
promoted_to:
---

## Apply
Two related traps when shipping opinionated defaults in a config file.

**A commented example is inert.** Writing the intended values into a commented block documents the
intent and delivers none of it: at runtime the key is absent, so those consumers fall through to
whatever the no-config path does. It reads as coverage in review — the values are right there in the
file — while behaving in production exactly like shipping nothing. If a value is the project's
default, it has to be live; if it is genuinely a suggestion, say so in prose rather than in
config-shaped syntax that looks like it is doing work.

**Sparseness needs a stated axis.** A config that intentionally covers only part of a space must
declare *which* part it is allowed to skip. "This block may omit entries" and "this block may cover
an entry partially" are different contracts with different failure modes, and the partial one lets a
newly added consumer land covered on one dimension and uncovered on another — inconsistently, and
without anything noticing. Pick the coarse axis (whole entries in or out), then enforce completeness
*within* each present entry with a validator that derives its expected set from the real consumer
rather than from a hand-kept list.

## War story
- 2026-07-31 (#168, PR #140 — merged) — The shipped `agents/harness-defaults.yml` cursor block
  covered only the three `docket-build-*` profile workers; docket's intended tiering for the other
  nine generated agents sat in a **commented** example block. Since a commented default is inert,
  those nine in practice had no docket default at all — the opposite of the change's own claim to
  ship Cursor support. Caught by maintainer review of the PR, not by any test.

  The fix set the axis explicitly: the cursor block now covers all twelve wrappers, and `hd_validate`
  enforces for cursor the same completeness it already enforced for claude, keyed off a new
  `HD_SHIPPED_HARNESSES`. Sparseness became a property of *which harnesses appear*, never of how much
  of one appears — so a thirteenth wrapper cannot land pinned on one shipped harness and unpinned on
  another. Completing the block immediately exposed a latent guard defect that partial coverage had
  been masking; see [[guard-keyed-on-presence-not-provenance]]. Related:
  [[config-knob-ship-end-to-end]] on shipping a knob's sample, docs, and prose together.
