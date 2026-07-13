---
id: 28
slug: report-channel-is-not-a-board-surface
title: A report channel is not a board surface — the backlog digest is ungated
status: Accepted
date: 2026-07-13
supersedes: []
reverses: []
relates_to: [12, 21]
change: 69
---

## Context

docket's `board_surfaces` config lists which derived board *views* to render:
`inline` (the committed `BOARD.md`) and `github` (the Issues/Projects mirror);
`[]` disables the board entirely. Change 0059 made `board-refresh.sh` the sole
gated writer of `BOARD.md` — it, and only it, owns the write decision, so a repo
that sets `board_surfaces: []` gets no board rendered and no board committed.

That gate was correct, and it is not in question. What it exposed is that
`docket-status.sh` had **no backlog-state channel at all** independent of the
board. In a board-off repo with nothing merged and no health findings, the status
pass exited 0 having printed *nothing* — yet every prose surface still promised a
`BOARD.md`, and the skill was still *instructed* to summarize backlog state.
Opening `BOARD.md` had been its only way to comply. An agent handed that silence
therefore behaved rationally and went hunting for the file: it re-ran the pass,
read `BOARD.md`, and `bash -x`-traced the script before discovering the pass had
correctly no-opped. A silent success was indistinguishable from a broken run.

The obvious fix — let the status pass render the board anyway — would have
re-opened the very hole change 0059 closed. The question was therefore not *how*
to get backlog state into the report, but *what kind of thing* that output is.

## Decision

Backlog state reaches the status report through a **digest**:
`render-board.sh --format digest`, a second, line-oriented projection of the
dependency-resolution/readiness pass the renderer already runs, piped straight to
stdout by an **ungated** `backlog_pass()` in `docket-status.sh`.

The digest is **report output, not a board surface**. It persists nothing, commits
nothing, pushes nothing, and never touches `BOARD.md`. Because it is not a surface,
it runs **regardless of `board_surfaces`** — which is precisely what lets
`board_surfaces: []` keep meaning "no board is rendered or committed" while backlog
state still reaches the report. The resulting split, and the rule a reader needs:

> **`board-refresh.sh` gates the surface; `render-board.sh` serves the report.**

Readiness keeps exactly one owner — `readiness()` in `lib/docket-frontmatter.sh`.
The digest is a *projection* of that pass, never a reimplementation of it.

**Corollary (part of this decision).** Because the digest is now the *sole* backlog
channel — the skill is explicitly forbidden from probing `BOARD.md` — it must be
**truthful at end of pass**. On a full run it is emitted **after** the merge sweep,
so a change swept during that pass reports as `done`, not `implemented`. (Under
`--board-only`, which performs no sweep, it is emitted before the early exit.) A
pre-sweep digest would have made the report contradict itself in the very scenario
the change exists to serve — `docket-implement-next`'s Step-0 sweep — reporting a
change as awaiting merge in the same report that says it was just swept to done.

**The test for a future channel.** Does the output *persist* anything? If not, it is
a **report** and is ungated. If so, it is a **surface** and must go through the gate.

## Consequences

**Enables a self-evidencing report.** `board off` is positive evidence of a
deliberate skip, and `pass ok` closes every completed pass, so stdout is never
empty. "Thin" can no longer read as "broken," and no agent has a reason to go
hunting for a file the config told it not to write.

**Costs a sentinel.** `docket-status.sh` now invokes `render-board.sh` directly
(read-only), so change 0059's sentinel — "the orchestrator never calls
`render-board.sh`" — could not be kept as stated and had to be **narrowed** rather
than preserved: every invocation must be the read-only `--format digest`, and a
**separate** scan asserts the orchestrator never redirects the renderer's stdout
into `BOARD.md`. Two guards, because they catch different holes: the first catches a
write-capable *invocation*, the second catches a read-only invocation whose *output*
is persisted anyway. A single guard would have left one of the two open.

**Widens `render-board.sh`'s contract.** It now has two output formats, and the
digest format is part of the contract surface a future edit must not break — the
price of not reimplementing readiness a second time. That trade is deliberate: one
owner of readiness is worth one more format.

**Relationship.** This RELATES TO ADR-0012 (the script-vs-model boundary) and
ADR-0021 (deterministic pipeline scripts) and supersedes neither. Both stand: the
digest is a mechanical, side-effect-free projection the model only surfaces, which
is exactly the ADR-0012 script side of the line. What this ADR adds is a second,
orthogonal axis that ADR-0012 does not speak to — not *who* produces the output
(script vs. model), but *what the output is* (report vs. surface) — and it is that
axis, not the script/model one, that decides whether a config gate applies.
