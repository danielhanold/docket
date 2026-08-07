---
id: 142
slug: make-the-unmapped-harness-wrapper-gap-loud-at-generation-tim
title: Make the unmapped-harness wrapper gap loud at generation time
status: killed
priority: medium
type: fix
created: 2026-07-27
updated: 2026-08-07
depends_on: []
related: []
discovered_from: [135]
adrs: []
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
<!-- docket:artifacts:end -->

## Why

Change 0135 gave `sync-agents.sh`'s `emit_for_harness()` explicit `codex)`, `cursor)`, and `claude)`
branches and documented the `*)` catch-all as an unverified gap — "a harness reaching this branch
has no verified contract mapping; its wrapper is a best guess, not a supported shape." ADR-0060
raised that to a rule.

The gap is not hypothetical. `scripts/lib/docket-gitignore-block.sh` sets
`DOCKET_GI_HARNESS_TOKENS="claude codex cursor agents kiro windsurf"`, so `agents`, `kiro`, and
`windsurf` are **accepted** harness tokens today. Listing any of them in `agent_harnesses:` really
does generate a Claude-shaped wrapper into that harness's directory, with Claude's `effort:` and
`skills:` frontmatter, and docket reports the pins as honored. That is byte-for-byte the defect
0135 existed to fix, still live for three tokens.

Today the only thing standing between a user and a silently-wrong wrapper is a source comment
nobody's tooling reads. 0135 explicitly scoped these tokens out ("they remain on the documented
`*)` gap"), which was right for that change and leaves the residual here.

## What changes

Make the gap **machine-visible at the moment it bites**, rather than only stated in a comment.

The obvious shape is a generation-time WARN whenever a token resolves to the `*)` arm, naming the
harness and saying plainly that the wrapper is Claude-shaped and unverified for that harness — the
same loud-drop posture 0135 established for an unexpressible effort pin. Decide during grooming
whether a WARN is enough or whether an unmapped token should be refused outright unless explicitly
acknowledged, and whether `--check` should surface it.

Also worth settling: whether `agents` even belongs in the token list. It reads like a
directory-name artifact rather than a real harness, and if so the honest fix for that one token is
removal, not a warning.

## Out of scope

- Writing a real emitter for `kiro` or `windsurf`. That needs each vendor's documented contract and
  is its own change per harness (ADR-0060).
- Changing the `*)` arm's output for tokens that keep using it.

## Why killed

Consolidated into #0245 at the 2026-08-07 backlog triage: the loud-unmapped-token gap lands with the shared-parse factor (#0141's leg) it depends on for cheapness. WARN posture; token-vocabulary removal stays out of scope there.
