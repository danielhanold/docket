# Learnings and ADRs as memory

## The problem it solves

A team that builds continuously accumulates two very different kinds of
knowledge, and loses both by default. There is the reasoning behind a
decision — why the backlog lives on its own branch, why one thing is refused
where another is allowed — which a code diff never captures and which a
newcomer, or the same person a year later, has to reverse-engineer from the
result. And there are the hard-won lessons of a build gone wrong — the shell
footgun, the tool that lies about its own exit code — which live in one
person's memory and die there.

Write neither down and every decision gets relitigated and every mistake gets
repeated. Write everything down with no discipline and you get a swamp nobody
reads. The two kinds of knowledge need different shapes and different curation.

Docket keeps two separate records. An **ADR** — an architecture decision
record: one file per decision, immutable once accepted — captures why a
non-obvious decision was made. **Learnings** — the loop's memory of lessons
from past builds, curated by a human — capture what a build taught. They are
kept, written, and promoted differently on purpose.

## The moving parts

```
   a decision is made              a build teaches a lesson
        │                                │
        ▼                                ▼
   ADR: one immutable file          a learnings finding
   per decision                     (a war story, pulled in by relevance)
        │                                │
   status-only edits on             human-gated promotion valve
   supersede / reverse                    │  "will an agent know
        │                                 │   to search for this?"
        ▼                                 ▼
   generated, validated             promoted → always-in-context rule;
   ADR index                        otherwise it stays a finding
```

- An ADR is written once and then immutable: after it is accepted, only its
  status line changes, and only when a later decision supersedes or reverses
  it. The record of *why* is never rewritten to match the present.
- The ADR index is generated and validated, never hand-edited, so the list of
  decisions cannot silently drift away from the files it indexes.
- A learning starts as a finding — a war story from one build, kept where it is
  pulled in by relevance rather than loaded into every context.
- Promotion is the valve between the two. A human decides whether a finding has
  graduated into a standing rule, using one test: will an **agent** — a
  separately launched worker with its own context, pinned to a model and
  effort — know to search for this on its own? If not, the lesson must fire
  unprompted, so it is promoted into the always-in-context rules; otherwise it
  stays a finding, pulled in only when relevant.
- Harvesting is confined to one moment: lessons are recorded at a **change**'s —
  one unit of planned work, roughly one pull request, tracked as one markdown
  file — close-out, one writer at one time, so the ledger has a single
  well-defined author rather than racing writers.

## The invariants

- An ADR is immutable once accepted; only its status line changes, and only on
  supersession or reversal.
- The ADR index is generated and validated, never hand-edited into disagreement
  with the ADR files it lists.
- A learning is a finding first, pulled in by relevance; only a human-gated
  promotion turns it into an always-in-context rule.
- Promotion turns on one question — will the agent know to search for this? — and
  the rule that must fire unprompted is the one that graduates.
- Learnings are harvested at close-out only: one writer, one moment, so there is
  a single author of record.
- Point-in-time records — accepted ADRs, archived changes, results — are never
  rewritten to match the present; rewriting them would falsify history.
- A cross-reference in maintained source anchors on a symbol name or a
  verbatim-quoted clause, never a line number.

## Decided in

- [ADR-0005](../adrs/0005-close-out-only-harvest.md) — fixed the learnings
  harvest to a single writer at close-out — one moment, ledger unpublished.
- [ADR-0013](../adrs/0013-adr-0012-boundary-extends-to-docket-adr-surface.md) —
  extended the script-versus-model boundary to the docket-adr surface, keeping
  the ADR index deterministically generated rather than model-authored.
- [ADR-0041](../adrs/0041-learnings-findings-directory-and-promotion-valve.md) —
  restructured the learnings ledger into a findings directory with a derived
  index and a human-gated promotion valve.
- [ADR-0054](../adrs/0054-cross-reference-anchor-style.md) — required
  cross-references in maintained source to anchor on symbols or quoted clauses,
  never line numbers.
