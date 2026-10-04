# Change: Capturing work that outlives the session

By the end of this page you can turn an idea into a tracked unit of work that survives the
session it occurred to you in — write it down once, keep working, and pick it up (or let an
autonomous build run pick it up) days or weeks later without re-explaining it. You will know what a
change file holds, how a change moves from idea to shipped, how to record what one piece of work
depends on, how to type your work so reports stay legible, and where the follow-up work an
unattended run notices ends up.

## What a change is

A **change** (one unit of planned work, roughly one pull request, tracked as one markdown file)
is the atom docket works in. Everything docket does — capturing, designing, building, merging,
closing out — happens to one change at a time.

The file has two parts: a front-matter block (the *manifest*) that machines read, and a body you
write in prose. As you meet them, the manifest fields are:

- `id` and `slug` — the change's number and its short kebab-case name; together they name its
  branch and its file. You do not pick the id; docket scans the highest existing id and hands the
  next one out, so two people capturing work at the same time never collide.
- `title` — a one-line summary.
- `status` — where the change sits in its life (the next section walks the states).
- `priority` — `critical` / `high` / `medium` / `low`, a hint for ordering, never a hard gate.
- `type` — which category of work this is (see [Typing your work](#typing-your-work), below).
- `depends_on` — the ids of other changes that must be `done` before this one can start.
- `spec`, `plan`, `results` — the design document a change links to, written before building;
  the task-by-task breakdown a build follows, written on the feature branch; and the required
  close-out record of what a build actually did (one per implemented change, trivial included).
  Filled in as the change moves through its life.
- `trivial` — a flag that says "this is small and mechanical enough to skip the design step."
- `branch`, `pr` — the feature branch and pull request, recorded once the build starts.

The exact field list and its rules are owned by the reference tier — see
[Change manifest and ADR fields](../reference/fields.md) for the authoritative version rather
than this working summary. Here we only care about the fields you meet while capturing.

## How a change moves, and the board

Each change carries a `status`, and that status walks a fixed happy path:

```
proposed  →  in-progress  →  implemented  →  done
```

- **proposed** — captured, waiting to be built. A proposed change that has not been designed
  enough to build (no spec, not marked `trivial`) sits in a **needs-grooming** state (a
  proposed change with neither a spec nor a trivial mark; it needs a design conversation first)
  until it is groomed — see [Designing before building](./designing-before-building.md).
- **in-progress** — a run has claimed it and is building.
- **implemented** — a pull request is open, waiting for your merge.
- **done** — merged and closed out.

Three off-ramps leave the happy path: `blocked` (an external blocker is recorded), `deferred`
(consciously shelved, may revive later), and `killed` (abandoned — kept in the archive as a
record, never deleted).

There is one detour on the way to `done`: `stacked-merged`. A **stacked change** (a change built
on another change's unmerged branch rather than on the integration branch) merges into that
parent rather than into your main line, so its code has not shipped yet — it parks at
`stacked-merged` and is promoted to `done` only once the root of its stack lands. The
[Building without supervision](./building-without-supervision.md) page covers stacking from the
build side.

There is also one edge running *backward*: `in-progress → proposed`. A **claim** (the moment a
change is picked up for building; it records which branch will carry the work and when it was
taken) carries a **claim lease** (a timestamp on a claim; when it expires with no branch behind
it, the change can go back to the queue). If a run crashes before it ever pushes a branch, the
change would otherwise sit stuck at `in-progress` forever; instead, once the lease has expired and
the change has no feature branch and no workspace, it can be reclaimed back to `proposed` —
by `docket maintenance sweep` when `reclaim.auto` is `true`, or by you with `docket change reclaim`
(the sweep reports such a change as skipped while `reclaim.auto` is `false`, the default).
Recovering these is covered in [Keeping the backlog honest](./keeping-the-backlog-honest.md).

The **board** (the generated overview of every change and its state, never edited by hand) is
your at-a-glance view. It groups every change into six sections — in progress, built, blocked,
groomed, proposed, and deferred — with one Type cell per row and a note like
`⏳ waiting on #<dep> — needs your merge` where a dependency is holding something up. The section
order and each section's sorting are configurable with `board.section_order` and `board.sorting`.
Every docket write re-renders the board, so there is no separate step to regenerate it.

One honest caveat about dependencies: chains serialize on the PR handoff. A change that depends
on another cannot start until that dependency is `done` — its pull request merged and the change
closed out. Unrelated changes drain freely in parallel around it — only the dependent one waits.

## Priorities, types, and dependencies

Three manifest fields shape *what gets built when*, and none of them is a hard scheduler:

- **`priority`** decides which build-ready change an autonomous run picks first — `critical`
  before `high` before `medium` before `low`, oldest first within a band — but it does not jump a
  change ahead of a dependency or force a build. It is an ordering hint, not a queue lock.
- **`type`** categorizes the work (`feat`, `fix`, `docs`, and so on) so reports and filters stay
  legible. [Typing your work](#typing-your-work) covers the vocabulary.
- **`depends_on`** is the one hard constraint. Listing `depends_on: [12]` means this change may
  not start until that dependency is `done` — not merely built, but merged into your integration
  branch (the branch code lands on, usually `main`) and closed out. This is why the board shows a
  "waiting on" note: the dependency is a real gate, and the concrete consequence of ignoring it is
  that the second change would be built against code that is not there yet.

Record a dependency whenever one change genuinely needs another's merged code or decision. Leave
it empty when the two pieces of work are independent — that is what lets them drain in parallel.

## Capturing an idea: designed, rough, or discovered

There are three moments work enters the backlog, and they differ only in how finished the idea
is when you write it down.

**A fully designed change.** When you already know what you want, capture it with the
`docket-new-change` skill — a **skill** being a named, reusable instruction set an agent loads for
one job. You describe the idea, docket brainstorms it with you into a **build-ready** change (a
proposed change that has a spec or is marked trivial and whose dependencies are all `done`) — a
spec written, dependencies noted — and commits the change file to the backlog. Because the
backlog is durable, you can capture now and build later; the idea does not evaporate with the
session.

**A rough stub.** When the idea is real but not yet designed, capture it as a stub and skip the
design step — it lands at `needs-grooming` and waits. You groom it later, in a session at
whatever model you choose. Grooming is its own page: [Designing before
building](./designing-before-building.md).

**Small and mechanical.** When the work is so small and mechanical that a design conversation
would be ceremony — a rename, a dependency bump — mark it `trivial`. A trivial change skips
needs-grooming entirely and is build-ready the moment its dependencies are clear, without a
spec.

**Scan mode.** Instead of describing one idea, you can point the new-change skill at the project
and have it *scan* for candidate work — gaps, TODOs, obvious follow-ups — and mint them as
`proposed` stubs in one pass. Scan mode only ever creates stubs for you to review and groom; it
never designs or builds anything on its own.

## Typing your work

Agents constantly surface follow-up work mid-task: a design refresh notices an adjacent gap, a
build uncovers a latent bug, a close-out finding implies a next step. With a human in the room
the model asks whether to file it. In an unattended run there is nobody to ask, so that work is
**mentioned in the run's final report** rather than acted on silently, and a human files it
deliberately.

- **Where discovered work goes.** The autonomous build — during its reconcile step (a check at
  build time that the change is still worth doing and its assumptions still hold, before any code
  is written) and its review — and the close-out pass surface genuine follow-up work **in the
  run's final report**. Lessons from builds go to the [learnings ledger](./remembering-why.md),
  curated by a human; drift inside the change currently being built goes to that change's own
  reconcile log — see [Building without supervision](./building-without-supervision.md).
- **How you file it.** Capture reported work with the `docket-new-change` skill, exactly as you
  would any other idea. The skill writes the change through `docket change create`, which takes
  the new change as a JSON request (`--request <file>`, or `-` for stdin). The new change shows up
  on the board as ordinary `needs-grooming` work and flows into the grooming queue like anything
  else you filed.
- **The taxonomy governs creation.** Every change you file draws its `type:` from `change_types`
  (below).

### The taxonomy (`change_types`)

`change_types` is the vocabulary a change's `type:` may draw from.

- **Default.** `[chore, docs, feat, fix, refactor, perf]` when no layer sets it.
- **Replaced, never merged.** The first configuration layer that sets `change_types` wins
  *entirely*. Merging would make a built-in value unremovable — you could only ever add types,
  never drop one — so restating the whole list is how you remove `perf`, or add something like
  `spike`.
- **Grammar.** Each entry matches `[a-z][a-z0-9-]*`. Duplicates and an empty list are
  configuration errors. `all` and `untyped` are reserved words and are rejected as types.
- **Any layer.** Set it per-repo, in your global config, or in the machine-local layer — the
  layer model is covered in [Repo config](../install/config-layers.md).

```yaml
# global config — drop a built-in, add your own; the list REPLACES the default
change_types: [chore, docs, feat, fix, spike]
```

Every active board row carries a **Type** cell, and `docket status` takes report-only `--type`
and `--priority` filters (each repeatable; `--type` accepts a configured change type). They
narrow only what `docket status` displays — never the board itself or any write.

## Where discovered work lands

To bring the thread together: the follow-up work an unattended run notices is never filed
silently. It is written into the run's final report, and you decide what becomes a change. That
keeps the backlog something you own — nothing appears in it that a human did not choose to put
there — while still making sure no genuine finding is lost between sessions. The next step for
anything you do capture is to design it: [Designing before
building](./designing-before-building.md).
