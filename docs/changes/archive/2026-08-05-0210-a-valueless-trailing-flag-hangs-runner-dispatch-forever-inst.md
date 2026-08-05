---
id: 210
slug: a-valueless-trailing-flag-hangs-runner-dispatch-forever-inst
title: A valueless trailing flag hangs runner-dispatch forever instead of aborting
status: killed
priority: medium
type: fix
created: 2026-08-05
updated: 2026-08-05
depends_on: []
related: [208]
discovered_from: [206]
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

`runner-dispatch.sh` parses every flag as `--flag) VAR="${2:-}"; shift 2 ;;`. When the flag is the
**final** argument with no value, `shift 2` with `$# = 1` returns non-zero and shifts **nothing**.
The script runs under `set -uo pipefail` with no `-e`, so `$1` stays the flag and the
`while [ $# -gt 0 ]` loop **never terminates**.

The facade's failure posture everywhere else is a loud `die`; here it is a hang. Under a foreground
generated shim invoked with `timeout 600000`, that burns ten minutes of the caller's budget and
produces no diagnostic at all.

Found by change 0206's whole-branch review, which added the fifth instance (`--worktree`); the
defect is pre-existing for the four older flags (`--runner`, `--agent`, `--model`, `--effort`).

## What changes

- Guard the value before consuming it, e.g.
  `--worktree) [ $# -ge 2 ] || die "--worktree requires a value"; WORKTREE="$2"; shift 2 ;;`
- Apply the same shape to all five flags — derive the site list from a grep of the parse loop, not
  by hand.
- Add a test leg per flag asserting a trailing valueless flag exits nonzero rather than hanging
  (bound it with a timeout so a regression fails fast instead of wedging the suite).

## Out of scope

- Any change to what the flags mean or how their values are resolved.

## Why killed

Consolidated into change 0208 (runner-dispatch hardening bundle); scope carried over verbatim, nothing dropped.
