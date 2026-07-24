---
id: 138
slug: unquote-board-change-titles
title: Board generator wraps each change title in literal double quotes
status: proposed
priority: medium
type: fix
created: 2026-07-24
updated: 2026-07-24
depends_on: []
related: []
discovered_from: [127]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

The rendered board (`BOARD.md`) shows each change's title wrapped in literal
double quotes. For change 135 the board reads:

```
"Generated Cursor wrappers violate Cursor's subagent contract, disabling skills and model effort"
```

when it should read:

```
Generated Cursor wrappers violate Cursor's subagent contract, disabling skills and model effort
```

The quotes are noise in the human-facing view and read as if they were part of
the title. Most likely introduced as part of change 127 (typed changes /
selective auto-capture), which touched board rendering.

## What changes

Fix the board generator so change titles render bare, without the surrounding
double quotes. Confirm the regression origin (change 127) and cover the fix
with a test so the quoting cannot silently return.

## Out of scope

- Any other board-column formatting or layout changes.
- The GitHub board mirror (unless it shares the same quoting path).

## Open questions

<!-- Resolve during grooming: whether the quoting is a YAML-serialization
     artifact (a title containing an apostrophe forced quoting) or an
     unconditional wrap, and whether the GitHub mirror shares the code path. -->
