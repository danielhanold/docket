---
id: 84
slug: re-dispatch-permission-gated-on-attribution-capability-not-launch-shape
title: "Re-dispatch permission is gated on mechanical attribution capability, not launch shape"
status: Accepted
date: 2026-08-11
supersedes: []
reverses: []
relates_to: [75, 80]
change: 275
---

## Context

Docket's caller-side run gate — single-sourced at `cursor-rules/run-gate.md` and spliced into
`AGENTS.md`/`CLAUDE.md` and the Cursor rule — assumed exactly one dispatch shape: snapshot the
in-progress set, dispatch foreground, block on the return, re-snapshot, diff to attribute the
claim, then run `verify-run <id>`.

A human launching `/docket-implement-next <id>` backgrounds the run inside the same user turn, so
no before-snapshot exists and steps 1-3 of that procedure are structurally unrunnable. Observed
live on change 0271: verification was possible only because the child's own prose happened to name
the id — precisely the evidence class the gate exists to distrust.

The oracle needed to do better already existed. `verify-run --in-progress-ids --with-claimed-at`,
together with `--iso-to-epoch`, was built by change 0271 so that `runner-dispatch.sh`'s own gate
could attribute a claim by claim instant versus dispatch epoch.

## Decision

**Permission to re-dispatch is gated on mechanical attribution capability — never on launch shape,
and never on the child's own report.**

A claim may be attributed to this run only when all three filters hold together:

1. it is absent from a before-set captured **before** the launch;
2. its `claimed_at` is present and parses;
3. that `claimed_at` is at or after a dispatch epoch captured **before** the launch.

All three are required. `claimed_at` is re-stamped at every phase boundary, so a concurrent run that
claimed before the window survives the epoch filter and looks fresh; the before-set is the only
filter that excludes it.

A session holding all three may follow the verdict table in full, including its bounded single
re-dispatch. A session holding no before-set — a slash-command or notification-first launch — enters
a named **unattributed mode**: it verifies and reports every verdict, and **never re-dispatches**.
An id named in the child's prose is a hint worth verifying, never attribution authority.

This mirrors the runner facade's observe-only detached seam (ADR-0080) and extends ADR-0075's
conservative-attribution posture from the script to the parent-session prose gate.

A second, subordinate rule falls out of it: **the two paths are discriminated by what the session
holds, not by how the run was launched.** Titling them by launch shape left a session that issued a
dispatch without taking a snapshot matching one branch by title and the other by state, with no
procedure — and an agent left with no procedure improvises toward the child's prose id, the exact
failure this decision removes.

## Consequences

The observed dispatch shape becomes gated rather than ungated.

The cost is that the unattributed path can never self-heal an incomplete run — a human must act.
That is deliberate: re-dispatching onto a change a live agent is holding is the one unrecoverable
move in the whole gate.

The gate block also grew from 25 to 48 lines of always-loaded context in every harness — a real cost,
deliberately priced.

No script changed. The rule is carried by prose plus a mutation-tested guard
(`tests/test_sync_agents_run_gate.sh`). Future work that adds a dispatch path to the gate must state
which of the two attribution states it lands in.
