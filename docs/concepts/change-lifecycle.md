# The change lifecycle as a state machine

## The problem it solves

Work that is "in progress" everywhere and finished nowhere is the failure mode of
an untracked backlog. Two people — or two automated loops — pick up the same
item. An item designed months ago gets built against assumptions that no longer
hold. A finished item never gets its lessons written down. Without a single place
that says exactly what state each item is in and precisely what moves it forward,
coordination decays into guesswork and duplicated effort.

Docket models each **change** — one unit of planned work, roughly one pull
request, tracked as one markdown file — as a small state machine. A change is
always in exactly one state, and only specific events move it to the next one.
The state lives in the change's own markdown file and is summarized on the
**board**, the generated overview of every change and its state, never edited by
hand. So at any moment you can read off what is queued, what is being built, and
what is done, without asking anyone.

The states are deliberately coarse — enough to coordinate, not so many that a
human has to memorize a flowchart to file a piece of work.

## The moving parts

```
   proposed
      │
      ├──────────────► needs-brainstorm ──(design a spec)──┐
      │  (no spec yet)                                      │
      │                                                     ▼
      └──(has a spec or trivial mark, deps merged)────► build-ready
                                                             │
                                                     (build takes a claim)
                                                             ▼
                                                        in-progress
                                                             │
                                                      (PR opened)
                                                             ▼
                                                       implemented
                                                             │
                                              (PR merges, finalize + sweep)
                                                             ▼
                                                           done
```

- **proposed** is the raw entry. A proposed change with neither a spec nor a
  trivial mark is **needs-brainstorm** — it needs a design conversation first. A
  proposed change that has a **spec** (the design document a change links to,
  written before building) or is marked trivial, and whose dependencies are all
  merged, is **build-ready**.
- **in-progress** means a build has taken a **claim** — the moment a change is
  picked up for building; it records which branch will carry the work and when it
  was taken. Grooming a change to build-ready takes no claim; only building does,
  so a groom and a build never contend for the same change.
- **implemented** means the build reached an open pull request.
- **done** means the pull request merged, **finalize** (the close-out sequence:
  rebase onto the integration branch, retest, merge, archive) ran, and a status
  **sweep** observed the merge and archived the change.

A **stacked change** — a change built on another change's unmerged branch rather
than on the integration branch — can be build-ready before its parent has merged;
its effective base is the parent's merge destination, so it does not sit waiting
against the wrong branch.

## The invariants

- A change is in exactly one state at a time; the change file is the source of
  truth, and the board is derived from it, never hand-edited into disagreement.
- Grooming a change to build-ready records no claim — only a build does — so a
  groom and a build never fight over the same change.
- A claim records which branch will carry the work and when it was taken; a
  build-ready change has all its dependencies merged before it can be claimed.
- A stacked change's effective base is its parent's merge destination, not the
  integration branch.
- Capturing a follow-up stub while a change is being built is best-effort: a
  failed capture never aborts the change in flight.
- A change becomes done only after its pull request has actually merged, observed
  by a sweep — never on a human's say-so alone.

## Decided in

- [ADR-0004](../adrs/0004-grooming-takes-no-claim.md) — let grooming take no
  claim, since a final-push compare-and-swap already protects a human-attended
  session.
- [ADR-0005](../adrs/0005-close-out-only-harvest.md) — fixed the learnings
  harvest to a single writer at close-out, so the lifecycle has one moment that
  records lessons.
- [ADR-0045](../adrs/0045-auto-capture-is-best-effort.md) — made mid-build stub
  capture best-effort so a failed capture never aborts the change being built.
- [ADR-0092](../adrs/0092-a-stacked-changes-base-is-its-parents-merge-destination.md)
  — defined a stacked change's effective base as its parent's merge destination.
