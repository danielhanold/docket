# The change lifecycle as a state machine

## The problem it solves

Work that is "in progress" everywhere and finished nowhere is the failure mode of
an untracked backlog. Two people — or two automated build runs — pick up the same
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
      ├──────────────► needs-grooming ────(design a spec)──┐
      │  (no spec yet)                                      │
      │                                                     ▼
      └──(has a spec or trivial mark, deps done)───────► build-ready
                                                             │
                                                     (build takes a claim)
                                                             ▼
                                                        in-progress
                                                             │
                                                      (PR opened)
                                                             ▼
                                                       implemented
                                                             │
                              ┌──────────────────────────────┤
                              │ (stacked: merged into        │ (PR merged; finalize
                              │  its parent's branch)        │  closeout proves it)
                              ▼                              ▼
                      stacked-merged ──(stack root lands)──► done

   off-ramps:
      blocked   — an external blocker is recorded; unblocking returns to in-progress
      deferred  — consciously shelved; may be revived to proposed
      killed    — abandoned as obsolete; final, kept in the archive
```

- **proposed** is the raw entry. A proposed change with neither a spec nor a
  trivial mark is **needs-grooming** — it needs a design conversation first. A
  proposed change that has a **spec** (the design document a change links to,
  written before building) or is marked trivial, and whose dependencies are all
  `done`, is **build-ready**.
- **in-progress** means a build has taken a **claim** — the moment a change is
  picked up for building; it records which branch will carry the work and when it
  was taken. Grooming a change to build-ready takes no claim; only building does,
  so a groom and a build never contend for the same change. A build that halts
  leaves its change `in-progress`; a human recovers it with
  `docket change resume-halted` and `docket run start implement-next --resume <id>`.
- **implemented** means the build reached an open pull request.
- **done** is written by `docket finalize closeout`, the close-out step of
  **finalize** (rebase, retest, publish, merge, close out, clean up), only after it
  has proved the pull request merged. `docket maintenance sweep` is the safety
  net: it moves a merged change that finalize never closed out to `done`.
- **blocked**, **deferred**, and **killed** are the off-ramps. `done` and
  `killed` are the two final statuses.

A **stacked change** — a change built on another change's unmerged branch rather
than on the integration branch — can be built before its parent has merged. Its
base is its parent's branch while the parent is live, and the integration branch
once the parent is `done`. When it merges into its parent it parks at
**stacked-merged**, because its code has not reached the integration branch yet,
and it moves to `done` once the root of its stack lands.

## The invariants

- A change is in exactly one state at a time; the change file is the source of
  truth, and the board is derived from it, never hand-edited into disagreement.
- Grooming a change to build-ready records no claim — only a build does — so a
  groom and a build never fight over the same change.
- A claim records which branch will carry the work and when it was taken.
- A dependency is satisfied only when it is `done`; a change waits until every
  dependency is `done` before it is build-ready.
- A stacked change's base is its parent's branch while the parent is live, and
  the integration branch once the parent is `done`.
- A change becomes done only after its pull request has actually merged, proved
  by finalize's closeout or by the maintenance sweep — never on a human's say-so
  alone.

## Decided in

- [ADR-0004](../adrs/0004-grooming-takes-no-claim.md) — let grooming take no
  claim, since a conflict-checked final push already protects a human-attended
  session.
- [ADR-0092](../adrs/0092-a-stacked-changes-base-is-its-parents-merge-destination.md)
  — defined a stacked change's effective base as its parent's merge destination.
