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

Change 0068 gives docket a finite executable facade with read-not-eval config emission, but the
seven operating skills and the convention's Step-0 preamble still instruct agents to build the
old shapes: `eval "$(docket-config.sh --export)"`, inline worktree ensure + hook disable, inline
`fetch/pull`, and direct per-helper invocations. Until the prose moves, the facade is unused and
the permission surface is unchanged.

## What changes

- Rewrite the convention's *Step-0 preamble* to: run `docket.sh preflight` as its own terminal
  call, read the printed `KEY=value` block, and interpolate the values as literals in later
  commands — no `eval`, no `source`, no inline sync programs.
- Update every operating skill (and the terminal-close-out reference) to invoke daily helpers
  only through the facade's canonical spelling; `docket-groom-next`/`docket-new-change` prose
  that reads config values switches to literal interpolation from the preflight/env output.
- Wiring tests: skill and reference prose contains no inline Step-0 `eval`/`if`/worktree/
  fetch-pull programs and invokes only operations from the facade's declared inventory, with an
  explicit carve-out for prose *references* to the human-initiated tier (`install.sh`,
  `migrate-to-docket.sh`, `sync-agents.sh`) — those are never runtime invocations.
- Document mid-run re-sync as "re-run `docket.sh preflight`" wherever skills currently instruct
  an inline fetch/pull before reads.

## Out of scope

- Any facade or helper behavior change (0068 owns the facade; helpers are unchanged).
- The Cursor guide and published permission fragment (0073).
- Changing what the skills do — only how their shell surface is expressed.

## Open questions

- Exact wiring-test definition of "runtime helper invocation" vs. prose mention (tokenize per
  invocation, per LEARNINGS 2026-07-13 #64).
- Whether any skill still needs a config value the preflight block does not emit.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
