---
id: 181
slug: document-the-unquoted-space-free-rule-for-agent-model-effort
title: Document the unquoted, space-free rule for agent model/effort config values
status: proposed
priority: medium
type: docs
created: 2026-07-31
updated: 2026-07-31
depends_on: []
related: []
discovered_from: [173]
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

Change 0173 added a hard gate: a config value that `field_of` cannot consume whole — a
quoted scalar, or one with an embedded space — now **aborts** wrapper generation before any
file is written, where the pre-0173 code silently ignored it and fell through to a default.

That is the right posture (a wrong pin baked into a wrapper is worse than a loud refusal),
but the rule it enforces is documented nowhere a user would look. `README.md`'s two `agents:`
examples — one for the global `~/.config/docket/config.yml`, one for `.docket.local.yml` —
show model/effort values without stating that they must be written unquoted and space-free.
docket-convention's `.docket.yml` schema block is likewise silent.

The blast radius makes this worth its own change rather than a comment: the **global** layer is
machine-wide, so one stale quoted value there blocks `sync-agents.sh` in *every* repo on that
machine, including the invocation inside `install.sh`. The diagnostic is self-describing once
you trigger it, but a documented rule prevents the trigger.

## What changes

- One line in each of `README.md`'s two `agents:` examples stating the unquoted, space-free rule.
- The equivalent line in docket-convention's `.docket.yml` schema commentary.
- Check whether `.docket.example.yml` needs the same note.

## Out of scope

- Changing the gate's posture or its diagnostic wording (ADR-0065, change 0173).
