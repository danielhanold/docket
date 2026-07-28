---
id: 166
slug: retune-the-interactive-skills-advisory-session-model-recomme
title: Retune the interactive skills' advisory session-model recommendation
status: proposed
priority: medium
type: chore
created: 2026-07-28
updated: 2026-07-28
depends_on: []
related: []
discovered_from: [164]
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

Change 0164 retunes the nine `agents/docket-*.md` wrapper defaults and the `.docket.example.yml`
mirror, but deliberately leaves one surface untouched: the **advisory** recommended model pinned
in the two interactive skills. `docket-new-change` and `docket-groom-next` get no generated
wrapper (a skill cannot force the session model), so each instead surfaces an advisory
recommendation at startup — and both still name `claude-sonnet-5`, asserted by string at
`tests/test_sync_agents.sh:494-495`.

After 0164 lands, every autonomous wrapper sits on `claude-opus-5` while the two interactive
skills keep recommending a model no other docket surface names. That is the same drift class 0164
exists to fix, one layer over — a user following the advisory gets a session model inconsistent
with the agents the same repo dispatches.

It was excluded from 0164 on purpose: an advisory session-model recommendation is a different
judgment (what a human should run interactively) than a wrapper default (what a machine runs
autonomously), and folding it in would have made a values-only retune also decide that question.

## What changes

Decide what the two interactive skills should advise post-0164, and apply it — to the advisory
lines in `skills/docket-new-change/SKILL.md` and `skills/docket-groom-next/SKILL.md`, and to the
two string assertions in `tests/test_sync_agents.sh` that pin them.

Worth settling as part of that: whether the advisory should name a literal model at all, or point
at the resolved `agents:` tier so it cannot drift out of sync again.

## Out of scope

- The wrapper defaults themselves — that is change 0164.
- Any mechanism for a skill to actually set the session model. The advisory is advisory.
