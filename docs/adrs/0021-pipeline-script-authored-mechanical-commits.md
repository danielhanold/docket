---
id: 21
slug: pipeline-script-authored-mechanical-commits
title: Deterministic pipeline scripts may author formulaic commits and mutate blessed-sequence state
status: Accepted
date: 2026-07-11
supersedes: []
reverses: []
relates_to: [12]
change: 58
---

## Context

ADR-0012 drew the script-vs-model boundary for `docket-status` and its sibling
skills: a mechanical, side-effect-free pass is scripted; a judgment-bearing pass
stays in-model — and, as ADR-0012 phrased it, "the model owns judgment and authors
commit messages." That phrasing bundled two distinct things under "the model": the
*judgment* of what to write, and the *authoring of the commit message* that records
a mechanical step.

That bundling has a cost the project kept paying. A model turn re-sends the whole
skill-and-convention context on every step, so a status pipeline invoked one
script-per-turn — board render, then merge sweep, then archive, then re-render —
is expensive precisely because the model must sit between each mechanical script
only to type a formulaic commit message (`docket: board refresh`;
`docket(NNNN): done — archived (status done, <date>)`;
`docket(NNNN): re-render artifacts block`) and hand off to the next script. Change
0058's `scripts/docket-status.sh` orchestrator collapses that whole pipeline into a
single invocation — but it can only do so if the script itself may author those
commit messages and carry the state mutations forward, rather than surfacing back
to the model between every step.

`archive-change.sh` already sat on this line: it commits and pushes the archive
move itself; only the `--message` string was model-supplied. So the question was
never whether a blessed script may mutate repository state — it already does — but
whether it may also author the fixed, formulaic message that names that mutation.

## Decision

A **deterministic pipeline script** may (a) author formulaic commit messages from
fixed templates — e.g. `docket: board refresh`,
`docket(NNNN): done — archived (status done, <date>)`,
`docket(NNNN): re-render artifacts block` — and (b) mutate repository state,
**provided the mutation follows an already-blessed script sequence and carries the
failure posture of its calling skill** (fail-closed / abort-and-report; no
autonomous divergence from the sequence its caller would have run step by step).

**Judgment-bearing prose stays model-authored.** Harvest-learnings entries, kill
reasons, and PR bodies read intent from context and are not templatable; the script
never writes them. This is the same line ADR-0012 draws — it is not moved. What this
ADR does is split ADR-0012's "the model authors commit messages" clause: the
*judgment* of what a mechanical step should say collapses to a **fixed string** when
the message is formulaic, and a fixed string is data, not judgment, so the script
may hold it. A message that would vary with intent is not a fixed string and remains
the model's.

Determinism is preserved. Templated messages are constant strings, not model
output; where the determinism invariant requires it, concurrent runs of the pipeline
converge byte-identically. The script owns the mechanical sequence end to end; the
model owns only the prose that genuinely encodes a decision.

## Consequences

**Enables:** one-invocation deterministic pipelines. A status sweep that was N model
turns (one per script, each re-sending full context) becomes a single orchestrator
invocation — a direct turn-count and cost reduction. It removes the last reason the
model had to sit between mechanical steps, which is exactly what change 0058's
`scripts/docket-status.sh` relies on to run the whole status pipeline in one call.

**Makes the boundary explicit:** templated/mechanical commits and blessed-sequence
state mutation are **script-authorable**; judgment prose is **not**. This
legitimizes what `archive-change.sh` already half-did (it commits and pushes; only
`--message` came from the model) and gives future pipeline scripts a citable rule
instead of re-litigating each mechanical commit.

**Costs:** the exact commit messages a pipeline emits no longer live in the skill
body a reader is looking at — to see them, a reader must consult the script or its
contract (`scripts/<name>.md`), one layer removed from the prose. The set of
templated strings becomes part of the script's contract surface, and widening it is
a deliberate change to that contract, not an ad-hoc model choice.

**Relationship:** this RELATES TO ADR-0012 and does not supersede or reverse it. The
script/judgment boundary of ADR-0012 stands unchanged; this ADR only clarifies that
a formulaic commit message falls on the script side of that same line.
