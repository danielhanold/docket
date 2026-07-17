---
id: 94
slug: docket-prime-context-digest
title: docket-prime — a token-budgeted context digest skills load instead of walking docs/changes
status: proposed
priority: medium
created: 2026-07-17
updated: 2026-07-17
depends_on: []
related: [69, 85]
adrs: [12]
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
| Artifact | Link |
|---|---|
| ADRs | [ADR-0012](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0012-docket-status-script-vs-model-boundary.md) |
<!-- docket:artifacts:end -->

## Why

Synthesized from the beads (gastownhall/beads) competitive review (2026-07-17). Beads ships
`bd prime` — "Output AI-optimized workflow context" — a single command that injects the working
state an agent needs (active issues, workflow state, stored memories) in a compact, purpose-built
form. Its FAQ frames token economics as a core differentiator (~2k tokens for the CLI surface).

docket's skills assemble the same picture by hand: a run reads BOARD.md (which re-lists the full
archive — see #0093), lists `active/`, greps frontmatter for selection, loads the learnings
index, and consults the ADR index — several files, re-parsed per run, with cost growing as the
repo ages. The ongoing skill-slimming work (#0085 in flight, and the 0037/0053–0055 family)
attacks the *instruction* side of context cost; this change attacks the *state* side: one
deterministic, token-budgeted digest — board summary, build-ready queue in selection order,
in-flight claims, needs-brainstorm bands, relevant ADR titles, learnings-index hooks — that a
skill loads in one read.

## What changes

- A `docket.sh prime` (or `docket-status --prime`) verb: a deterministic script (per the
  ADR-0012 script-vs-model boundary) that emits a compact digest to stdout — sections and
  budget settled in brainstorm; candidates: counts per status, the build-ready queue (ordered,
  with priority/age), in-progress claims (with staleness), needs-brainstorm bands, blocked/
  waiting-on edges, ADR index titles, learnings hooks.
- Skills' Step-0/selection reads rewire to consume the digest instead of walking
  `docs/changes/` themselves; the underlying files remain authoritative for the change a skill
  actually operates on (digest for orientation, file reads for action).
- A rough token budget as an explicit design constraint (e.g. the digest stays useful at a few
  hundred lines even on a 500-change repo — decay rules from #0093 apply to its history view).
- Output is derived-view only: emitted to stdout (and/or a gitignored cache), never a new
  committed surface to keep fresh.

## Out of scope

- Replacing `BOARD.md` (the human-facing board stays; prime is the agent-facing view).
- Semantic/embedding-based relevance — selection of "relevant" ADRs/learnings stays cheap and
  lexical (or just titles/hooks), no new infrastructure.
- Changing what any skill *does* with the state — only how it acquires it.

## Open questions

- Stdout-only vs a committed/cached artifact (stdout keeps it always-fresh and un-driftable;
  a cache saves repeated cost in one session — where's the line?)
- Which skills adopt it first — implement-next's selection and status's report seem highest
  value; do the interactive skills use it too?
- Does prime subsume the Step-0 `preflight` KEY=value block, extend it, or stay a separate verb?

## Reconcile log
