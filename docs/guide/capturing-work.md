# Capturing work that outlives the session

By the end of this page you can turn an idea into a tracked unit of work that survives the
session it occurred to you in — write it down once, keep working, and pick it up (or let the
autonomous loop pick it up) days or weeks later without re-explaining it. You will know what a
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
- `priority` — `low` / `medium` / `high`, a hint for ordering, never a hard gate.
- `type` — which category of work this is (see [Typing your work](#typing-your-work), below).
- `depends_on` — the ids of other changes that must merge before this one can start.
- `spec`, `plan`, `results` — the design document a change links to, written before building;
  the task-by-task breakdown a build follows, written on the feature branch; and the optional
  close-out record of what a build actually did. Filled in as the change moves through its life.
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
  enough to build (no spec, not marked `trivial`) sits in a **needs-brainstorm** state (a
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
it, the change goes back to the queue). If a run crashes before it ever pushes a branch, the
change would otherwise sit stuck at `in-progress` forever; instead the expired lease lets it
self-heal back to `proposed`. Recovering these is covered in
[Keeping the backlog honest](./keeping-the-backlog-honest.md).

The **board** (the generated overview of every change and its state, never edited by hand) is
your at-a-glance view: every change grouped by status, one Type cell per row, with a note like
"waiting on #12 — needs your merge" where a dependency is holding something up. You regenerate
it with the status skill — a **skill** being a named, reusable instruction set an agent loads for
one job — and never edit it by hand.

One honest caveat about dependencies: chains serialize on the merge gate. A change that depends
on another cannot start until that dependency's pull request is merged. Unrelated changes drain
freely in parallel around it — only the dependent one waits.

## Priorities, types, and dependencies

Three manifest fields shape *what gets built when*, and none of them is a hard scheduler:

- **`priority`** orders the board and breaks ties, but it does not jump a change ahead of a
  dependency or force a build. It is a hint to you and to the ordering, not a queue lock.
- **`type`** categorizes the work (`feat`, `fix`, `docs`, and so on) so reports and filters stay
  legible. [Typing your work](#typing-your-work) covers the vocabulary.
- **`depends_on`** is the one hard constraint. Listing `depends_on: [12]` means this change may
  not start until change 12 has *merged* — not merely been built, but landed on your integration
  branch (the branch code lands on, usually `main`). This is why the board says "waiting on
  #12": the dependency is a real gate, and the concrete consequence of ignoring it is that the
  second change would be built against code that is not there yet.

Record a dependency whenever one change genuinely needs another's merged code or decision. Leave
it empty when the two pieces of work are independent — that is what lets them drain in parallel.

## Capturing an idea: designed, rough, or discovered

There are three moments work enters the backlog, and they differ only in how finished the idea
is when you write it down.

**A fully designed change.** When you already know what you want, capture it with the new-change
skill: you describe the idea, docket brainstorms it with you into a **build-ready** change (a
proposed change that has a spec or is marked trivial and whose dependencies are all merged) — a
spec written, dependencies noted — and commits the change file to the backlog. Because the
backlog is durable, you can capture now and build later; the idea does not evaporate with the
session.

**A rough stub.** When the idea is real but not yet designed, capture it as a stub and skip the
design step — it lands at `needs-brainstorm` and waits. You groom it later, in a session at
whatever model you choose. Grooming is its own page: [Designing before
building](./designing-before-building.md).

**Small and mechanical.** When the work is so small and mechanical that a design conversation
would be ceremony — a rename, a dependency bump — mark it `trivial`. A trivial change skips
needs-brainstorm entirely and is build-ready the moment its dependencies are clear, without a
spec.

**Scan mode.** Instead of describing one idea, you can point the new-change skill at the project
and have it *scan* for candidate work — gaps, TODOs, obvious follow-ups — and mint them as
`proposed` stubs in one pass. Scan mode only ever creates stubs for you to review and groom; it
never designs or builds anything on its own.

## Typing your work

Agents constantly surface follow-up work mid-task: a design refresh notices an adjacent gap, a
build uncovers a latent bug, a close-out finding implies a next step. With a human in the room
the model asks whether to file it. In an unattended run there is nobody to ask, so that work is
**mentioned in the run's final report** rather than acted on silently.

That is the whole shape of automatic capture today. The `auto_capture` map (`enabled`, `types`)
is still a parseable configuration key and its stored values are never rewritten, but it
activates nothing: **automatic change capture is deferred — capture work deliberately with
`docket change create`**. An enabled value is answered with an unsupported diagnostic before any
step changes state, never a silent fallback — so a run *reports* the follow-up work it finds and
a human files it deliberately.

- **Parseable, inactive.** With `auto_capture` set (`enabled: true` or `false`) the config
  resolver still reads it and never rewrites your stored values; no run mints a stub from it.
- **Where discovered work goes.** The autonomous build — during its reconcile step (a check at
  build time that the change is still worth doing and its assumptions still hold, before any code
  is written) and its review — and the close-out pass surface genuine follow-up work **in the
  run's final report**. Lessons from the build loop go to [learnings](./remembering-why.md) — the
  loop's memory of lessons from past builds, curated by a human; drift inside the change currently
  being built goes to that change's own reconcile log — see
  [Building without supervision](./building-without-supervision.md).
- **The taxonomy still governs creation.** `change_types` and `auto_capture.types` remain the
  documented vocabulary a deliberately-created change draws from (below): they type the work you
  file, they do not schedule its capture.

```yaml
change_types: [chore, docs, feat, fix, refactor, perf]   # the taxonomy — see below

auto_capture:
  enabled: true
  types: [feat, fix]                                     # a subset of change_types, or `all`
```

Work you file with `docket change create` shows up on the board as ordinary `needs-brainstorm`
work and flows into the grooming queue like anything else you filed by hand.

### The taxonomy (`change_types`)

`change_types` is the vocabulary a change's `type:` may draw from. It governs what *this machine
creates*, never how shared history *reads*: a change carrying a type you have not configured
still renders on the board and still answers `--type` queries, because configuration is not
allowed to make someone else's work unreadable.

- **Default.** `[chore, docs, feat, fix, refactor, perf]` when no layer sets it.
- **Replaced, never merged.** The first configuration layer that sets `change_types` wins
  *entirely*. Merging would make a built-in value unremovable — you could only ever add types,
  never drop one — so restating the whole list is how you remove `perf`, or add something like
  `spike`.
- **Grammar.** Each entry matches `[a-z][a-z0-9-]*`. Duplicates and an empty list are
  configuration errors. `all` and `untyped` are reserved: they are query pseudo-values, never
  stored types.
- **Global-able.** Set it per-repo, in your global config, or in the machine-local layer — the
  layer model is covered in [Governing through
  configuration](./governing-through-configuration.md).

```yaml
# global config — drop a built-in, add your own; the list REPLACES the default
change_types: [chore, docs, feat, fix, spike]
```

`auto_capture.types` must be drawn from the taxonomy, but it is validated against the
`change_types` **visible to the layer that set it**. So narrowing `change_types` on one machine
never invalidates a `types` list inherited from the committed config — the two keys resolve
through independent chains, and each layer only has to be internally consistent.

Every active board row carries a **Type** cell, and the status and board tools both take
report-only `--type` / `--priority` filters: `--type untyped` finds changes carrying no type at
all, and `--type all` (the default) selects everything. These narrow the **digest only** — never
the board itself, the merge sweep, archiving, harvesting, health checks, or any write.

### Migrating to typed changes

An earlier version made `auto_capture` a map. **The old scalar form is a hard error with no
compatibility shim** — the resolver refuses to start and prints the replacement, carrying your
own value forward:

```yaml
# before — no longer valid
# auto_capture: true

# after
auto_capture:
  enabled: true
  types: all
```

Then categorize the active backlog once. Archived changes are never reclassified, and every
creation path writes a type from here on, so the untyped set can only shrink.

```bash
# 1. the exact inventory. --digest-only keeps this a WRITE-FREE read: a bare `docket status`
#    run commits and pushes the board, sweeps merged changes, archives, and harvests before it
#    prints the digest — not what you want from a command you are running only to *look*.
docket status --digest-only --type untyped

# 2. an agent proposes a complete id -> type mapping; you approve it as one decision

# 3. apply it by writing each change's `type:` frontmatter (all files or none, idempotent).
#    From here on every creation path writes a type, so the untyped set only shrinks.
```

## Where discovered work lands

To bring the thread together: the follow-up work an unattended run notices is never filed
silently. It is written into the run's final report, and you decide what becomes a change. That
keeps the backlog something you own — nothing appears in it that a human did not choose to put
there — while still making sure no genuine finding is lost between sessions. The next step for
anything you do capture is to design it: [Designing before
building](./designing-before-building.md).
