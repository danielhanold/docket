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
auto_groomable: false
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

## Auto-groom blocked

### 2026-07-17 — docket-auto-groom abstained (critic: needs human context)

A default-biased self-brainstorm was drafted and gated by the adversarial critic
(`docket-auto-groom-critic`). Two decisions could not be safely auto-committed, so no spec was
emitted and `auto_groomable` was flipped to `false`. The stub stays needs-brainstorm and is now
first in the interactive `docket-groom-next` queue.

**Blocker 1 — the stub's premise is partly stale versus already-shipped #0069.** #0069 (merged
2026-07-13) already shipped `render-board.sh --format digest` — a single-read, stdout-only,
read-only, line-oriented backlog digest (`backlog <status> <count>` rollups + one `change <id>
<status> <readiness> <slug>` per active change) that `docket-status.sh` runs as an **ungated
backlog pass** in every mode (board on or off). So "docket's skills assemble the same picture by
hand" (Why) is materially stale for the backlog/readiness portion — that channel already exists.
The genuine net-new surface is narrow: build-ready queue in **selection order** (today's digest is
id-ascending), in-progress **claim staleness**, **ADR index titles**, and **learnings index
hooks**. *What a human must decide:* whether #0094 is (a) re-scoped to "extend the existing
`--format digest` with those four additions + adopt it in `implement-next`'s selection" (a much
smaller change), (b) built as a parallel `prime` verb anyway, or (c) largely subsumed by #0069 and
killed/merged into a #0069 follow-up. Reconciling stub intent with shipped reality is the stub
author's call, and options (a)/(c) are re-scope/kill decisions that are **never autonomous**.

**Blocker 2 — the rewire scope is a value fork with no safe default.** The stub's core value is
"skills rewire their Step-0 to consume the digest in one read." Both horns are unsafe to
auto-commit: *shipping the verb with no adopter* delivers dead infrastructure that saves zero
tokens (the change's whole reason to exist); *rewiring the operating-skill family* makes the digest
the **sole** Step-0 orientation channel — exactly the `sole-channel` learning whose war story is
**this repo's own #0069** (a digest made the sole channel; ordering versus the mutating merge sweep
and report totality both broke, caught only at whole-branch review). Auto-committing a build-ready
spec down either branch is out of bounds. *What a human must decide:* whether this change rewires
skills at all, and if so which skills first and in what order — with the `sole-channel` ordering /
totality re-proof designed in deliberately, not defaulted.

**Bounded fixes to fold in once the human resolves the above (from available context, no human
input needed):**
- The digest must be an **orchestrator** (the `docket-status.sh` shape) that composes
  `render-board.sh --format digest` (sole owner of readiness resolution, per ADR-0012's
  no-duplication invariant) with an ADR-index read and a learnings-index read — `render-board.sh`
  reads only change files and must not grow ADR/learnings ownership.
- **stdout-only, never a committed or cached surface** (binding constraint; matches `sole-channel`
  / `presence-encoded-state`). Token budget is a stated **direction**, not a gate
  (`size-target-is-direction`). Archive rendered as **counts only**, so prime is cheap **before**
  #0093's decay rules exist — record the #0093 relation (currently in prose only; consider adding
  #0093 to `related:`), do not depend on its decisions.
- prime stays a **separate verb** that consumes but does not subsume/replace the Step-0 `preflight`
  KEY=value config block; the `docket-config.sh` / `docket.sh preflight` contract is untouched
  (open question 3).

**Re-arm:** a human supplies the missing scope decisions, flips `auto_groomable` back to `true`,
and DELETES this `## Auto-groom blocked` section (git history keeps it).
