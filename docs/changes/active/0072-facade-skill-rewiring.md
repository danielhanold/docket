---
id: 72
slug: facade-skill-rewiring
title: Rewire the operating skills and Step-0 to the facade — retire the eval preamble
status: proposed
priority: medium
created: 2026-07-13
updated: 2026-07-13
depends_on: [68]
related: [68, 73]
adrs: []
spec: docs/superpowers/specs/2026-07-13-facade-skill-rewiring-design.md
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
| Spec | [2026-07-13-facade-skill-rewiring-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-13-facade-skill-rewiring-design.md) |
<!-- docket:artifacts:end -->

## Why

Change 0068 gives docket a finite executable facade with read-not-eval config emission, but the
seven operating skills and the convention's Step-0 preamble still instruct agents to build the
old shapes: `eval "$(docket-config.sh --export)"`, inline worktree ensure + hook disable, inline
`fetch/pull`, and direct per-helper invocations. Until the prose moves, the facade is unused and
the permission surface is unchanged.

## What changes

- Rewrite the convention's *Step-0 preamble* to: run `docket.sh preflight` as its own Bash
  call, read the printed `KEY=value` block, and interpolate the values as literals in later
  commands — no `eval`, no `source`, no inline sync programs. `preflight` fails closed;
  `CREATE_ORPHAN` keeps `docket-config.sh --bootstrap` as the one sanctioned direct-helper
  spelling (byte-exact, convention-only — the facade deliberately doesn't expose it).
- Update every operating skill (and the terminal-close-out reference) to invoke daily helpers
  only through the facade's canonical spelling; prose that reads config values (shell
  variables like `$BOARD_SURFACES`/`$SKILL_*`) switches to literal interpolation from the
  preflight/env output — verified at groom time to cover every value skill prose reads.
- Route ALL metadata-tree sync instructions through "re-run `docket.sh preflight`" — pre-read
  syncs and the push-retry CAS loops alike; plain git plumbing (add/commit/push) stays direct.
- Wiring tests (tokenizer + unique anchors): a strip-then-scan sweep judging code spans per
  invocation — canonical spelling byte-exact then stripped, ops derived from `scripts/docket.md`'s
  inventory by grep, human-initiated tier allowed in prose position only, `eval`/fetch/pull
  shapes forbidden — plus mutation-tested presence anchors for the new Step-0 and re-sync
  instructions. Existing skill-prose test anchors are followed to the new spellings, never
  loosened.

## Out of scope

- Any facade or helper behavior change (0068 owns the facade; a `bootstrap` facade verb is a
  possible future stub, not this change).
- The Cursor guide and published permission fragment (0073).
- Changing what the skills do — only how their shell surface is expressed.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
