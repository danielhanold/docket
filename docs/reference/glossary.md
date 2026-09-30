# Glossary

Every docket term in one place: what it is, what it is for, and — where a command exists — how you
reach it on the CLI. The first sentence of each entry is the same one-clause gloss the guide and
concept pages use on first mention, so a word means the same thing on every page.

**How to read the snippets.** They are illustrations of today's command shape, not the contract.
The flags a verb accepts are owned by `docket <noun> <verb> --help`, and the machine-readable list
of every operation is owned by `docket capabilities --json` (see [`cli.md`](cli.md)). A
`--request <file>` body is built from `docket schema --operation <id>`, never guessed. Snippets
that belong to autonomous workflows (the run tracker, gate drives, finalize steps) are shown so you
can recognise them in a run log; the skills normally invoke them for you.

Terms are grouped by the layer of docket they belong to. Jump to a group:

1. [Repository and branches](#repository-and-branches)
2. [Work records](#work-records)
3. [Change lifecycle and statuses](#change-lifecycle-and-statuses)
4. [Grooming](#grooming)
5. [Building a change](#building-a-change)
6. [The run tracker](#the-run-tracker)
7. [Supervised gate runs](#supervised-gate-runs)
8. [Review](#review)
9. [Finalize and close-out](#finalize-and-close-out)
10. [Status, health, and maintenance](#status-health-and-maintenance)
11. [Skills, agents, and harnesses](#skills-agents-and-harnesses)
12. [Configuration](#configuration)
13. [Operations and the CLI protocol](#operations-and-the-cli-protocol)
14. [Obsolete terms](#obsolete-terms)

An [alphabetical index](#alphabetical-index) closes the page.

---

## Repository and branches

### Adopting docket in a repository

Bringing a repo under docket, which is a separate step from installing docket on your machine. An
existing single-branch repo is moved with `repository migrate`, a human-typed command that asks for
confirmation unless you pass `--yes`. A fresh repo gets the orphan `docket` branch and `.docket/`
worktree from `repository init`.

**Used for:** the one-time setup per repo. The skills never migrate for you; the bootstrap guard
stops and points at `migrate`. A local gate with no test command halts until one is configured,
and `repository configure-tests` generates that policy.

```sh
cd <target-repo>
docket repository migrate            # existing repo (human-typed)
docket repository init               # fresh repo
docket repository configure-tests    # write the pending build/finalize test policy
```

### Archived record

A change's archived file (plus its results) once it reaches a final status, `done` or `killed`. It
stays on the metadata branch; the integration branch gets code, plans, and results through PRs alone.

**Used for:** historical browsing. `archive/` on the metadata branch keeps every closed change.

### Bootstrap guard

The first-run check that probes whether the `docket` branch exists and whether the planning
surface still sits on the integration branch, and then proceeds, creates the orphan branch, or
stops with a migrate prompt.

**Used for:** making sure no skill ever writes into a half-migrated repository. It surfaces as the
disposition of `repository.prepare`; see [Bootstrap verdict](#bootstrap-verdict-proceed--stop_migrate--create_orphan).

### Bootstrap verdict: `PROCEED` / `STOP_MIGRATE` / `CREATE_ORPHAN`

The bootstrap guard's three outcomes. `PROCEED` means the repo is already migrated. `STOP_MIGRATE`
means a legacy single-branch or half-migrated layout was found. `CREATE_ORPHAN` means a fresh repo
that needs an empty `docket` branch.

**Used for:** deciding whether a skill may touch metadata at all. `repository.prepare` enforces the
verdict fail-closed and reports it as a disposition: `applied`/`no-op` when it proceeds, otherwise
`refused` with a remedy naming the human-typed `docket repository migrate` or `docket repository init`.

```sh
docket repository prepare --repo-dir . --json   # the verdict surfaces as this envelope's disposition
```

### Docket-mode / single-branch mode

Docket-mode is the default layout: metadata on the metadata branch, code on the integration
branch. A **single-branch** (legacy) layout keeps the backlog on the integration branch.

**Used for:** deciding where docket reads and writes. A single-branch repo is refused with a
migration prompt rather than half-initialised. Docket-mode is the only supported topology; the old
`metadata_branch: main` opt-out is obsolete.

```sh
docket repository init      # fresh repo: create the orphan docket branch
docket repository migrate   # legacy single-branch repo: move the backlog (human-typed only)
```

### Feature branch

The branch a single change's code is built on, minted at claim as `<type>/<slug>` (or
`<branch_prefix>/<slug>`) and recorded in the change's `branch:` field.

**Used for:** carrying the code, plan, and results of one change. It never modifies docket
metadata. Branches are named by slug, not id — read `branch:` rather than grepping for the number.

```sh
docket status --records --json   # each change's branch/pr fields
```

### Git hooks in docket worktrees (pre-commit, husky, lefthook)

Docket's bookkeeping commits skip your repo's git hooks. The `.docket/` worktree, and docket's
temporary publish and migration worktrees, point `core.hooksPath` at an empty directory.

**Used for:** stopping a shared `pre-commit`, husky, or lefthook hook from firing on claims, board
writes, or ADRs. Your code commits on feature branches still run the team's hooks. There is nothing
to configure, and the setting heals itself on every docket run.

### Integration branch

The branch code lands on, usually `main`.

**Used for:** every feature branch is cut from it, every PR targets it (except a stacked change's),
and finalize rebases onto it before merging. Set by the `integration_branch` coordination key
(`auto` resolves to the remote's default branch). A GitFlow repo whose default branch is `main` but
whose integration line is `develop` must set `integration_branch: develop` explicitly.

```sh
docket diagnostic config --repo-dir . --json   # shows the resolved integration branch
```

### Metadata branch

The `docket` git branch where the backlog, specs, and decisions are stored, separate from the code.

**Used for:** keeping planning history out of your code history. The two branches never merge into
each other. It is always the orphan `docket` branch: the old `metadata_branch` key is obsolete
(change 0363) and resolves nothing.

```sh
git log --oneline origin/docket -5   # recent backlog commits, never mixed with code
```

### Metadata worktree

A second checkout of the repo at `.docket/`, parked on the metadata branch, so backlog edits never
touch your code checkout.

**Used for:** every change, ADR, and board write. It is gitignored, shared by every docket session
on the machine, and has the repo's git hooks disabled so bookkeeping commits never trip a
code-side pre-commit hook. Because it is shared, stage by explicit path there — never `git add -A`.

```sh
docket repository prepare --repo-dir . --json   # create/sync .docket/ and print the typed context
git -C .docket log --oneline -3                  # inspect it without cd-ing into it
```

### Step-0 preamble

The fixed startup every operating skill runs before anything else: load `docket-convention`, fetch
and validate the capability catalog, run `repository.prepare`, then act on its disposition.

**Used for:** making every skill start from synced metadata and a validated command surface. A
`refused` or `error` disposition stops the skill. Every mid-run re-sync, including a push-retry, is
a fresh `repository.prepare` run.

```sh
docket capabilities --json                      # step 2: capability bootstrap, its own call
docket repository prepare --repo-dir . --json   # step 3: prepare, its own call
```

### Worktree / feature workspace

An isolated working copy of the repo, on its own branch. A change's build happens in a feature
worktree under `.worktrees/`.

**Used for:** building several changes without disturbing your own checkout. Keep editors out of
`.worktrees/` while a run is active — an out-of-band edit trips the run's identity check.

```sh
docket workspace inspect --id 412
docket workspace prepare --id 412 --revision <revision>
```

---

## Work records

### `## Artifacts` block

The first body section of every change: a generated list of links to its spec, plan, results, PR,
and ADRs, rendered from the frontmatter. It sits between marker comments, and the operation that
writes the frontmatter re-renders it in the same commit.

**Used for:** getting from a change to its artifacts in one click. Never hand-edit it. A stale
block surfaces as an `artifact-links-stale` finding.

### ADR

An architecture decision record: one file per decision, immutable once accepted.

**Used for:** keeping the *why* of a non-obvious decision. After acceptance only its `status:` line
changes; a reversal or supersession is always a new ADR. The index (`<adrs_dir>/README.md`) is
generated.

```sh
docket adr record    --request adr.json
docket adr supersede --request supersede.json
docket adr reverse   --request reverse.json
```

### ADR index / `## Update` note

The **ADR index** is the generated `<adrs_dir>/README.md` that lists every ADR with its status. Each
ADR operation re-renders it inside its own commit. An **`## Update` note** is a dated section appended
to an Accepted ADR when the surrounding context changes but the decision still stands. The
`## Decision` section is never edited.

**Used for:** browsing decisions, and recording new context without a new ADR. Index drift from a
hand edit shows up as `adr-index-stale` in `repository check`. The fix is the human-typed repair,
never a hand render.

```sh
docket repository check
docket repository migrate --repair-frontmatter   # human-typed: re-renders a stale index
```

### ADR status: Accepted / Superseded by / Reversed by / Deprecated

An ADR's `status:` line is the only field that changes after acceptance: `Accepted`,
`Superseded by ADR-NN`, `Reversed by ADR-NN`, or `Deprecated`. **Supersede** replaces a decision
with a newer one. **Reverse** undoes it. Either way the replacement is a new ADR, and the old ADR's
status flip lands in the same commit.

**Used for:** changing your mind without rewriting history. The target of a supersede or reverse
must be `Accepted`, or the operation refuses.

```sh
docket adr supersede --request supersede.json   # {request_id, target:{id,path,revision}, successor:{…}}
docket adr reverse   --request reverse.json
```

### Backlog

The set of changes docket tracks. Day to day it means the changes not yet in a final status, in
`<changes_dir>/active/`; the board and the status digest summarise it.

**Used for:** the queue that grooming and implement-next draw from. It is durable, so work captured
in one session is picked up in a later one.

```sh
docket status          # counts, readiness, and the ordered ready queue
```

### Board

The generated overview of every change and its state, never edited by hand.

**Used for:** reading the backlog at a glance (`BOARD.md` on the metadata branch). Every typed
mutation re-renders it inside its own commit. `board_surfaces: [inline]` turns it on; `[]` turns it
off.

```sh
docket status              # the same information, human-readable
docket repository check    # surfaces a stale or hand-edited board
```

### Change

One unit of planned work, roughly one pull request, tracked as one markdown file.

**Used for:** everything. A change carries a frontmatter **manifest** (id, status, priority, type,
dependencies, links) and a PM-altitude body (`## Why`, `## What changes`, `## Out of scope`, …).
Active changes live in `<changes_dir>/active/NNNN-<slug>.md`; final ones (`done`, `killed`) in
`archive/<date>-NNNN-<slug>.md`.

```sh
docket schema --operation change.create        # the request fields
docket change create --request new-change.json  # usually driven by docket-new-change
```

### Revision (`--revision`)

The exact id of a pinned state. Docket uses the word in one sense only:

- **Record revision:** the git blob object id of a record a typed operation can mutate: a change,
  an ADR, a learning finding, or a linked spec. Spelled `--revision` on a flag, `revision` in a
  request or read (`docket status --json` carries one per change), `spec_revision` for a groom's
  linked spec, `target.revision` for the ADR a supersede/reverse flips, and `change.revision` for
  an ADR's producing change.
- **PR revision:** a hash over a pull request's mutable snapshot, spelled `pr.revision` in
  `context finalize` and `pr_revision` in `finalize merge` / `finalize retarget-children`.
- **Commit revision:** the commit ids a mutation reports, spelled `committed_revision`,
  `metadata_revision` and `*_branch_revision`.

**Used for:** conflict-checked writes. A mutating operation takes the record revision you read and refuses
if the record moved under you, so two sessions can never silently overwrite each other.

```sh
docket status --json | jq -r '.changes[] | select(.id==412) | .revision'
docket change claim --id 412 --revision <that-revision>
```

### Derived view / generated block / backlink

A derived view is anything rendered from the change files rather than authored: the board, each
change's `## Artifacts` link block, and the `docket:backlink` block stamped at the top of every
spec, plan, results file, and PR body. Each has exactly one writer and is never hand-edited.

**Used for:** keeping links between a change and its artifacts correct in both directions.

```sh
docket artifact backlink --artifact docs/results/<file>.md --change docs/changes/active/0412-<slug>.md
```

### Frozen build record

A merged change's `plan:` and `results:` files. After the PR merges, nobody hand-edits them again,
not even to fix a stale line reference.

**Used for:** keeping a completed run auditable: they record what the build was told to do at the
time. Corrections go in a new change. The only writer allowed afterwards is `artifact.backlink`,
which re-stamps the generated backlink block.

### Id / slug

The **id** is a change's integer number, zero-padded to four digits in its filename. The **slug** is
its short kebab-case name, derived from the title. `change.create` allocates both, reading the next
id from fresh origin state, so no one picks an id by hand.

**Used for:** naming the change file (`NNNN-<slug>.md`) and the feature branch (`<type>/<slug>`).
Branches carry the slug, not the id.

```sh
docket status --json | jq '.changes[] | {id, slug, path}'
```

### Learnings / finding / promotion

Learnings are the loop's memory of lessons from past builds, curated by a human. Each lesson is a
**finding** file under `<changes_dir>/learnings/`. **Promotion** moves a finding into the
always-in-context rules (`AGENTS.md`) when the answer to *"will the agent know to search for
this?"* is no.

**Used for:** not repeating a mistake. Readers pull only the findings whose index line is relevant;
promotion is always a human act.

```sh
docket learning record --request finding.json
docket learning update --request finding-update.json
```

### Learnings index / read on demand

The **index** is `learnings/README.md`, a small grouped list of one line per finding (hook plus
topics). **Read on demand** is the read rule: always load the index, then open only the findings
whose index line bears on the change at hand.

**Used for:** a memory that grows without growing every run's context. Readers are implement-next
(at plan and review time), groom-next, and auto-groom, and all are gated by `learnings.enabled`.
Automated index rendering is deferred from Go v1, so the existing index bytes are kept, not refreshed.

### Learnings ledger / war story / `promotion_state`

The **ledger** is `<changes_dir>/learnings/`: one curated finding per file, on the metadata branch
only. Each finding has an `## Apply` rule and dated `## War story` entries. A **war story** records
what happened on one change. `promotion_state` is `retained` (default), `candidate`, or `promoted`.

**Used for:** keeping lessons without loading them all into every run. A record either creates a
finding or extends one (a new war-story entry, the change id added to `changes:`). It never merges
two findings. A promoted file is kept as the rule's receipt.

```sh
docket learning record --request finding.json          # {request_id, slug, hook, topics, changes, apply, war_story}
docket learning update --request finding-update.json   # {path, revision, hook, topics, changes, sections}
```

### Manifest

The frontmatter block at the top of a change file. Its fields (`status`, `priority`, `type`,
`depends_on`, `stacked_on`, `related`, `discovered_from`, `adrs`, `spec`, `plan`, `results`,
`trivial`, `auto_groomable`, `branch`, `claimed_at`, `pr`, `blocked_by`, `reconciled`, …) are owned
by the `docket-convention` skill (see [`fields.md`](fields.md)).

**Used for:** the machine-readable state of a change. Edit it only through typed operations; a
hand edit leaves the board stale.

### Plan

The task-by-task breakdown a build follows, written on the feature branch.

**Used for:** routing each task to a build tier. The plan file lives on the feature branch; the
`plan:` field is attached on the metadata branch. A merged plan is a frozen build record — never
hand-edited afterwards.

```sh
docket change attach-plan --id 412 --revision <v> --path docs/superpowers/plans/<file>.md --commit <sha>
```

### Marker section

A body section whose mere presence is state: `## Run halted`, `## Finalize blocked`,
`## Auto-groom blocked`, `## Publish deferred`, `## Reclaim log`.

**Used for:** making a stop verifiable in git rather than a claim in a report. The board's
"needs you" cells are driven by these sections; each has a named operation that writes and removes
it.

### Results

The close-out record of what a build actually did — required for every implemented change,
trivial included.

**Used for:** telling the human what to check (`**Human action:**`, `## Outcome`, verification,
known follow-ups). The file lives in `<results_dir>` on the feature branch; the `results:` field is
attached on the metadata branch.

```sh
docket change attach-results --id 412 --revision <v> --path docs/results/<file>.md --commit <sha>
```

### Results `**Human action:**` line

The required statement right after a results file's title. In one or two sentences it says whether
the human needs to do anything. It is followed by the required `## Outcome` section.

**Used for:** telling the reviewer at a glance whether to act. During implementation it may say the
assessment is pending; the final results must give a settled answer.

### Spec

The design document a change links to, written before building.

**Used for:** giving the build everything it needs to implement without guessing. Stored on the
metadata branch and linked from the `spec:` field; produced by a brainstorm or by auto-groom.

### Stub

A change captured without a design — `proposed`, no spec, not trivial. In lifecycle terms it is
**needs-grooming**.

**Used for:** capturing an idea quickly and designing it later with grooming.

### Trivial

A change marked `trivial: true` — small and mechanical enough to need no spec.

**Used for:** skipping the design step. A trivial change with its dependencies merged is
build-ready without a spec.

---

## Change lifecycle and statuses

Statuses: `proposed` · `in-progress` · `blocked` · `deferred` · `implemented` · `stacked-merged` ·
`done` · `killed` (allowed values `statuses` in `docket schema`).

`done` and `killed` are the two **final statuses**; every other status is non-final.

| Status | Meaning | Moved there by |
|---|---|---|
| `proposed` | Drafted, awaiting work | `change create`, `change revive`, `change reclaim` |
| `in-progress` | Claimed, being built | `change claim`, `change unblock` |
| `blocked` | External blocker, recorded in `blocked_by:` | `change block` |
| `deferred` | Consciously shelved, may revive | `change defer` |
| `implemented` | Built, PR open — the PR handoff | `change mark-implemented` |
| `stacked-merged` | Merged into its stack parent, awaiting the stack root | finalize close-out |
| `done` | PR merged and archived (happy final status) | finalize close-out / sweep |
| `killed` | Abandoned as obsolete (sad final status) | `change kill`, reconcile |

### `## Why deferred` / `## Why killed`

The body sections written when a change enters `deferred` or `killed`, carrying the authored reason.
`change.defer` and `change.kill` each take the body in their request (`why_deferred` / `why_killed`)
and splice the section in the same commit.

**Used for:** leaving a reason a later reader can act on: revive a deferred change, or understand a
killed one in the archive.

```sh
docket change defer --request defer.json   # {change_id, path, revision, why_deferred}
docket change kill  --request kill.json    # {change_id, path, revision, why_killed}
```

### Archive

The single physical move of a change file from `active/` to `archive/<UTC-date>-NNNN-<slug>.md` on
a final transition (`done` or `killed`). It is idempotent.

### Block / unblock, defer / revive, kill

Typed transitions for the side exits: **block** records an external blocker, **unblock** clears it
and returns to `in-progress`; **defer** shelves a change with `## Why deferred`, **revive** returns
it to `proposed`; **kill** archives it as obsolete with `## Why killed`.

```sh
docket change block   --request block.json
docket change unblock --request unblock.json
docket change defer   --request defer.json
docket change revive  --request revive.json
docket change kill    --request kill.json
```

### Claim / claim lease / reclaim

A **claim** is the moment a change is picked up for building; it records which branch will carry
the work and when it was taken. The **claim lease** is a timestamp on a claim (`claimed_at:`);
when it expires with no branch behind it, the change goes back to the queue. **Reclaim** is that
return trip (`in-progress → proposed`), logged in `## Reclaim log`.

**Used for:** making sure an abandoned build never holds a change forever, while a claim that
already has a branch (real work) is flagged for a human instead. Grooming never takes a claim.

```sh
docket change claim         --id 412 --revision <v>
docket change refresh-claim --id 412 --revision <v>   # extend the lease at a phase boundary
docket change reclaim       --id 412 --revision <v>   # refuses with lease-not-expired if still live
```

Config: `reclaim.lease_ttl` (hours) and `reclaim.auto`.

### Dependency (`depends_on`) / implicitly blocked

A change id that must reach `done` before this change is build-ready. An unsatisfied dependency
makes a change *implicitly* blocked — it is skipped, not marked `blocked` — and the board shows
**waiting on #N** (with "needs your merge" when #N is already `implemented`).

**Used for:** ordering work. Use explicit `blocked` only for blockers docket cannot infer.

### Halt / resume-halted

A **halt** is an autonomous run stopping because it needs a human. It writes the bare `## Run halted`
section and commits it, so the stop is visible in git. **Resume-halted** removes the marker so a
new run can pick the change back up.

```sh
docket change halt          --id 412 --revision <v> --input halt.json
docket change resume-halted --id 412 --revision <v> --acknowledge-quiescent
```

`--acknowledge-quiescent` is required: it is your explicit statement that the prior worker is
**quiescent** (no longer writing). Without it, on revision drift, or on a live gate lock, the
operation refuses and writes nothing.

### Owned sections / section intents (`preserve` / `replace` / `remove`)

**Owned sections** are the change-body headings that typed operations may write: `## Why`,
`## What changes`, `## Out of scope`, `## Open questions`, `## Why deferred`, `## Why killed`,
`## Auto-groom blocked`. A `change.groom` request edits them through `sections[]` entries, each with a
`heading` spelled with its `## ` marker and an `intent`. `replace` carries new Markdown; `preserve`
and `remove` must carry empty Markdown.

**Used for:** rewriting a proposal's body during grooming without hand edits. Any other heading is
refused with `invalid-section-heading`. A `revise` needs at least one `replace` or `remove`, and
`re-enable` may not name `## Auto-groom blocked` because it removes that section itself.

```sh
docket schema --operation change.groom   # the sections[] shape
```

### PR handoff

The stop at `implemented`: implement-next opens the pull request and ends there. It never merges; a human (or a
finalize run they start) takes the change the rest of the way. It is a handoff to a person, not a
[gate](#gate): no suite runs at this stop.

**Used for:** keeping merge a human decision. After review, the next step is `docket-finalize-change`.

### Readiness: build-ready / needs-grooming / not-proposed

**Build-ready** is a proposed change that has a spec or is marked trivial and whose dependencies
are all merged. **Needs-grooming** is a proposed change with neither a spec nor a trivial mark; it
needs a design conversation first. Anything not `proposed` reads `not-proposed`. Status reports
four more kinds: `auto-groom-blocked`, `waiting-dependency`, `stack-base-unresolved`, and `invalid`
(see [Readiness reason](#readiness-reason--waiting-on-n--needs-you-cells)).

**Used for:** selection. Only build-ready changes can be implemented; only needs-grooming changes
can be groomed. Selection order is priority → age (`created`) → lowest id.

```sh
docket status --json | jq '.changes[] | {id, readiness, readiness_reason, unmet_dependencies}'
```

### Readiness reason / "waiting on #N" / "needs you" cells

`readiness_reason` is the human-readable explanation beside each change's `readiness` in
`docket status --json`, for example "ready to build" or "waiting on unmet dependencies". It is prose,
not a parseable code; key on `readiness`. The board renders the same state as cells like
**⏳ waiting on #N — not yet built**, **… — needs your merge**, and the "needs you" cells
(**auto-groom blocked**, **run halted**, **finalize blocked**).

**Used for:** seeing why a change is not moving. A "needs you" cell always comes from a
marker section, so it is backed by a commit.

```sh
docket status --json | jq '.changes[] | {id, readiness, readiness_reason, unmet_dependencies}'
```

### Related / discovered_from

Informational cross-links. `related:` is read by reconcile; `discovered_from:` records which change
surfaced this one. Neither gates readiness.

### Selection order

The deterministic ranking used to pick the next change: priority (`critical` > `high` > `medium` >
`low`), then age (`created`), then lowest id. A missing or malformed `created:` sorts last within its
priority band.

**Used for:** implement-next's build-ready pick, and auto-groom's and groom-next's stub pick.
Groom-next orders its stubs in bands first (abstained, then opted-out, then auto-groomable). Finalize
uses its own ordering by mergeability.

```sh
docket status --json | jq '.ready'   # the build-ready queue, already in selection order
```

### Stacked change / effective base

A stacked change is a change built on another change's unmerged branch rather than on the
integration branch, named by the single-integer `stacked_on:` field. Its **effective base** is its
parent's merge destination, and it can be build-ready before the parent merges once that base
resolves.

**Used for:** building a chain of dependent PRs without waiting for each to merge. On the board an
unresolved base reads *waiting on #A — stack base not built*.

```sh
docket status --json | jq '.changes[] | select(.id==413) | .effective_base'
```

---

## Grooming

### Abstain / re-enable

**Abstain** is auto-groom declining to design a stub it cannot safely default: it flips
`auto_groomable: false` and writes `## Auto-groom blocked`. **Re-enable** is the human supplying the
missing context and flipping it back, which removes that section in the same commit.

```sh
# groom.json: {"change_id": 412, "revision": "<v>", "outcome": "re-enable", ...}
docket change groom --request groom.json
```

### Auto-groom / auto-groomable

**Auto-groom** grooms stubs with no human, gated by an adversarial **critic**. A stub's **effective
auto-groomable** value is its `auto_groomable:` override when set, or else the repo's `auto_groom`
knob. A stub is **auto-groomable** (selectable by auto-groom) when it is needs-grooming *and* that
effective value is `true`.

**Used for:** draining design work unattended. Arm a stub by committing `auto_groomable: true`
before dispatch; an uncommitted flag fails preflight.

### Brainstorm / consultant

A **brainstorm** is the design conversation that produces a spec. In docket's own brainstorm role
the parent holds the dialogue with you, then dispatches the **consultant**
(`docket-brainstorm-consultant`) once to author the spec or return critique.

**Used for:** turning intent into a spec during `docket-new-change` or `docket-groom-next`.

### Capture modes: designed / rough stub / trivial / scan

The ways work enters the backlog. **Designed**: brainstorm with `docket-new-change` into a spec, so
the change is build-ready. **Rough stub**: capture without a design; it waits at needs-grooming.
**Trivial**: mark it `trivial` and skip design. **Scan mode**: point `docket-new-change` at the
project to mint several stubs in one pass.

**Used for:** matching capture effort to how finished the idea is. Scan mode is opt-in and only
creates stubs; it never designs or builds. Work an unattended run discovers is reported in its final
report, never filed automatically.

### Critic

`docket-auto-groom-critic` — the agent that attacks an auto-groom draft and returns exactly one
verdict. Also called the **adversarial gate**. It never improves the draft; if it cannot be dispatched, auto-groom abstains rather than
self-critiquing.

### Groom

Taking a stub through design to build-ready. Exits: a linked spec, a trivial verdict, a kill, a
defer, a revise of an already-groomed change, or (autonomous only) an abstain.

**Used for:** the step between capturing and building. Interactive grooming is
`docket-groom-next`; the typed write underneath is `change.groom` with an `outcome` of `spec`,
`trivial`, `revise`, `abstain`, or `re-enable`.

```sh
docket schema --operation change.groom
docket change groom --request groom.json
```

### Groom outcome `revise`

The `change.groom` outcome that adjusts a change that is already groomed (`proposed` with a spec or
`trivial: true`). It can replace the linked spec's body and edit owned sections. It never sets
`spec:` or `trivial:`, so a change cannot flip between spec'd and trivial.

**Used for:** fixing a just-landed design. Reach it by naming the id to `docket-groom-next`. A spec
replace also needs `spec_revision` (the spec's blob id), so a concurrent spec edit contends
instead of being overwritten. A `title` alone is also a valid revise: it rewrites `title:`, the board row, and
the spec's backlink line, and renames nothing — the slug and every path stay put.

```sh
# groom.json: {"change_id": 412, "path": "…", "revision": "<v>", "outcome": "revise", "spec_markdown": "…", "spec_revision": "<blob>"}
docket change groom --request groom.json
```

---

## Building a change

### Budget watch / serially confirmed breach

Wall-clock lines the suite runner prints even on a green run. `BUDGET WATCH:` and
`PARALLEL-SENSITIVE:` are screening findings (parallel timings are machine-dependent);
`SERIAL CONFIRMED OVER BUDGET:` is an authoritative breach to act on. Neither fails the run.

### Build evidence

The committed record of that gate run, read by the reviewer. It certifies an exact tested commit.

**Used for:** letting review and finalize trust a record rather than a worker's word. Adding a
commit after the evidence was recorded makes it stale (`evidence-unverified`).

```sh
docket evidence record    --id 412 --head <sha> --run <run-dir>
docket evidence verify    --head <sha> --record <evidence-file>
docket evidence recertify --id 412
```

### Build gate

The full test-suite run at the end of a build that must be green before review. Its command is
`build.test_command`; `build.max_attempts` (default 4) caps the initial run plus repair-and-rerun
cycles before a red suite halts for a human.

**Used for:** catching a test a single task broke without ever running. The verdict is tri-state
(see [Tri-state verdict](#tri-state-verdict--halt-exit-code)); an empty suite command halts as a
configuration gap (see [Suite command](#suite-command-buildtest_command--finalizetest_command--configure-tests)).

### Build tier / escalation

A build tier is one of four workers (economy, standard, premium, max) a plan task is
routed to by risk. Standard is the default and the "uncertainty sink". A worker that finds its
task beyond its tier returns `NEEDS_ESCALATION` with a concrete reason, and the task **escalates** one
tier — at most once. A return without a reason is malformed and halts the build.

**Used for:** paying premium rates only where mistakes are expensive. Each tier is its own agent
(`docket-build-economy`, `-standard`, `-premium`, `-max`) with its own model and effort pin.

### Development test (`docket development test`)

The command that runs docket's own complete test suite from the current checkout, through the Go-native suite runner.
It is what this repository sets both `build.test_command` and `finalize.test_command` to.

**Used for:** the suite gate when working on docket itself. Run it from source (`go run ./cmd/docket …`) so the gate
tests the exact checkout under review, not a stale installed binary. Other repositories set their own suite command.

```sh
go run ./cmd/docket development test
```

### Focused tests / task gate

The tests a single build worker runs for its own plan task: the baseline, the failing test, the passing test, and a
focused re-run. They are never the whole suite.

**Used for:** proving one task before its single commit. A passed task gate never substitutes for the
[build gate](#build-gate), because a task that passes in isolation can still break a test it never ran.

### Gate

Docket's word for a checkpoint that must pass before the next step may run. A gate either lets the work through,
stops it for repair, or halts it for a human. It never guesses a pass from a worker's report.

**Used for:** knowing which checkpoint stopped a run. The kinds are:
- [Focused tests / task gate](#focused-tests--task-gate) — one build task's own tests.
- [Build gate](#build-gate) — the one whole-suite run after the last task.
- [Finalize gate](#finalize-gate) — re-runs the whole suite on the rebased branch before merging.
- [Run tracker](#run-tracker) — bookkeeping that decides whether a dispatched run may be retried.
- [Policy gate](#policy-gate-require_pr_approval) — `require_pr_approval`, which asks whether a merge was authorised.
- Adversarial gate — the [critic](#critic) that must pass an auto-groom draft.
- [Worktree slot](#worktree-slot) — one gate run per worktree at a time.

The build and finalize gates both run the whole suite. The page that covers them is
[Suite gate](#suite-gate). The `implemented` stop where a person merges is the
[PR handoff](#pr-handoff), not a gate.

### Implementation context

The read-only bundle implement-next assembles before claiming: the selected change, its readiness,
and what it needs to proceed.

```sh
docket context implementation --id 412 --json
```

### Implement-next / the drainer

`docket-implement-next` — the autonomous workflow that picks the next build-ready change (or the
id you name), claims it, reconciles, plans, builds, reviews, and stops at an open PR. It never
merges.

**Used for:** draining the backlog unattended. Always dispatch it as its named agent, bracketed by
the run tracker. Pass an explicit id to resume an `in-progress` change — a bare run skips it.

```sh
# inside an agent session
/docket-implement-next 412
/loop docket-implement-next 412 413 414   # drain several back-to-back
```

### Mark implemented

The transition to `implemented` once the PR is open, carrying the evidence and PR reference.

```sh
docket pr publish --id 412 --head <sha> --evidence <file> --body pr.json   # --body is a JSON request, not markdown
docket change mark-implemented --id 412 --revision <v> --head <sha> --pr <url> --evidence <file>
```

### Plan writer

`docket-plan-writer` — the agent implement-next dispatches to invoke the plan skill, commit the
plan with its backlink on the feature branch, and return `PLAN_PATH=<path>`.

### PR publish

Creates the change's pull request for a published feature head, or adopts the one that already
exists. It writes docket's backlink and build-evidence blocks into the body. Before calling GitHub
it checks that the local head, the remote head, the evidence head, and the requested head all
match.

**Used for:** opening the PR at the end of a build. It never makes a duplicate PR, and your
authored prose is kept byte-for-byte. `--body` takes a JSON request file with the title and body,
not a markdown file.

```sh
docket pr publish --id 412 --head <sha> --body pr.json --evidence evidence.json
```

### Preflight

The implementation-scope sweep that runs at the start of a selection-path implement-next
(`maintenance.preflight`): it closes out merged-but-unclosed changes and returns a compact
post-sweep read. A run that names one explicit id skips it.

```sh
docket maintenance preflight --json
```

### Re-certify (`evidence recertify`)

The supported fix when a follow-up commit (for example, one answering review feedback) is pushed to
an `implemented` change's open PR and the build evidence goes stale (`evidence-unverified`). It
re-runs `build.test_command` at the current published head and replaces only the PR's
build-evidence block.

**Used for:** refreshing the proof without re-entering implement-next. The change stays
`implemented`, and nothing is committed, pushed, or merged. It needs a clean feature worktree whose
local, remote, and PR heads all agree. It charges one attempt against `build.max_attempts`.

```sh
docket evidence recertify --id 412
```

### Reconcile / reconcile log

**Reconcile** is a check at build time that the change is still worth doing and its assumptions
still hold, before any code is written. It ends one of three ways: refresh the scope (append a
dated `## Reconcile log` entry and set `reconciled: true`), kill the change as obsolete, or halt it
for a human.

**Used for:** never building a correct implementation of a stale plan.

```sh
docket change reconcile --input reconcile.json
```

### Run verify

A read-only check of one change's claim-to-implemented postconditions (committed plan and results,
evidence, PR, status) that reports a closed verdict. The convention calls it **verify-run**. Its
verdicts are `run-complete`, `run-unclaimed`, `run-incomplete`, `run-halted`, and `run-waiting`, all
with exit code 0, so key on the verdict, never the exit code.

**Used for:** trusting git, not a completion report. Run it whenever a dispatched build says it
finished.

```sh
docket run verify --id 412
```

### Suite command (`build.test_command` / `finalize.test_command`) / configure-tests

The two config keys that name "the tests": `build.test_command` for the build gate and `finalize.test_command` for
the finalize gate. They are independent and may differ. Both default to `""`, which means unconfigured.

**Used for:** telling docket what command to run. A `local` gate with an empty command halts as a configuration gap,
not a red suite, and names `docket repository configure-tests` as the remedy. Each gate reads its key from config,
never from a second copy.

```sh
docket repository configure-tests --repo-dir .   # generate the build/finalize test policy in .docket.yml
docket diagnostic config --repo-dir . --json      # see what each key resolves to
```

### Suite gate

A full run of the configured test suite that certifies a branch, as opposed to one task's focused tests. The build gate
and finalize's local gate are both suite gates, and each reads its own command from config.

**Used for:** getting one trusted green/red answer over the whole branch. A green build gate leaves build evidence, and
finalize can skip its re-run when that evidence is still pinned to the head being merged.

### Workflow role

One of the five pluggable steps — `brainstorm`, `plan`, `build`, `review`, `finish` — each bound
to a skill. The shipped defaults are `superpowers:brainstorming`, `superpowers:writing-plans`,
`docket-build`, `docket-review`, and `superpowers:finishing-a-development-branch`.

**Used for:** knowing which skill runs each step. Rebinding is deferred in Go v1: any explicit
`skills.*` value in config — even one repeating the default, or the `auto` sentinel — blocks every
repository mutation until removed.

### Workspace publish

The implement-next step that pushes a change's ready feature head to its remote branch. The push
never forces, and the step returns the verified remote head. It refuses if the head it finds is not
the `--head` you gave it.

**Used for:** making the tested head exist on the remote before a PR is opened. A diverged remote
is `contended` and an unobservable result is `unknown`. Neither forces a push, and this step opens
no PR.

```sh
docket workspace publish --id 412 --head <sha>
```

---

## The run tracker

### Start / run key / run id / run context

**Starting** a run (`run.start`) mints three values before a dispatch: the **run key** (ties a finish
to this launch), the **run id** (the id of this run, threaded into cancel and drive flags), and
the **run context** (a token). The run context and the run id are both copied into the
implement-next dispatch prompt; a scope prepared with `--run-id` hands that run id to every scoped
start under it, so build-task workers never receive it (except the repair worker, for its
build-owned post-fix re-run). It prints
`run-started <key> <run-id> <run-context>`; `run-untracked` still allows a keyless dispatch that
can never authorise a re-dispatch.

```sh
docket run start implement-next
docket run start implement-next --resume 412   # resuming an in-progress change
```

### Attribution / unattributed read

**Attribution** is tying a finish to the exact launch that produced it, by the run key — never by
timing or names. With no key, an **unattributed** read reports on a named change id but cannot
authorise a re-dispatch. Attribution is conservative: when unsure, the run tracker declines to credit.

### Cancel

The explicit stop for a dispatched run — there is no automatic Stop button. It fences the run
so nothing new attaches, tears down its tasks and processes, and reports `cancelled`,
`cancellation-pending` (re-run to finish), `already-cancelled`, or `refused`. It never counts as a
failure and never earns a retry. Starting a run states the **owner-lifecycle caveat**: closing a tab,
interrupting the coordinator, or killing a process does not tell the run tracker the run is over — only
cancel does.

```sh
docket run cancel --key <key> --run-id <run-id> --reason "superseded by 413"
```

### Continuation

A single-use id handed out with `run-continue`, redeemed by the resumed controller so the same
attempt carries on.

```sh
docket run continue <key> <continuation-id>
```

### Run fence

The mark `run.cancel` puts on a run, located by its [run id](#start--run-key--run-id--run-context), so nothing new can
attach to it. A fenced run is never restored, and a scope prepared with `--run-id` lets the fence also revoke a later
takeover.

**Used for:** making a cancel stick while teardown finishes.

### Run verdict

The single report line `run.verdict` prints after a run returns. Obey the line, never the exit
code or the child's prose.

| Line | Meaning |
|---|---|
| `run-retry-once …` | The only line that authorises another dispatch — once, same key, for the id and unmet conditions it names. Granted at most `run.max_attempts - 1` times. |
| `run-continue <key> run-waiting <id> <continuation-id> <phase>` | Non-terminal: the same attempt still owns work. Resume it with the continuation id; spends no retry. |
| `run-done …` | Finished (e.g. `run-done run-complete`). |
| `run-stop …` | Stop; no re-dispatch (e.g. `run-stop run-tracker-unavailable takeover-ambiguous`). |
| `run-observe …` | Observe only; no re-dispatch (e.g. `run-observe run-incomplete`). |
| `run-halted` | A human is needed. |

```sh
docket run verdict <key>
docket run verdict --unattributed 412   # no key: observe-only, can never authorise a retry
```

### Run-context refusal (`run-context-invalid` / `run-context-conflict`)

The two `change.claim` outcomes when the run context token passed as `--run-context` fails validation against
the started run: the token is invalid, or it conflicts with a claim already bound to the run. Either refusal writes
nothing.

**Used for:** binding a gated dispatch to exactly one claim. Never retry a refused gated claim as an ungated one. Leave
out `--run-context` only when no run context was given.

```sh
docket change claim --id 412 --revision <v> --run-context <run-context>
docket schema --json | jq '.vocabularies.claim_dispositions'
```

### Resume dispositions

What starting with `--resume` reports when a prior run exists: `resume-active-run` (the prior run
may still be live — cancel it or continue it), `cancellation-pending` (finish the cancel first), or
`resume-replacement-reserved` (exactly one replacement dispatch is reserved; dispatch that one).

### Run tracker

The bookkeeping around a launched build run: who launched it, whether it finished, whether it may
be retried. It keeps that state durably, outside the worker's prose.

**Used for:** deciding whether to dispatch again. A completion notification is the child's claim,
never the parent's verdict. The `run.*` operations (`run start`, `run verdict`, `run continue`,
`cancel`) are the **run tracker**: it owns attribution, durable state, and retry accounting, which
are never reimplemented by hand. If the `docket` binary is missing, the install is broken.

---

## Supervised gate runs

### Worktree slot

The per-worktree slot that lets only one gate run proceed at a time. A busy-slot refusal means
the slot is **occupied** by a run admission could not prove finished — not necessarily a live
process. Inspect with `gate observe`, settle with `gate stop`; history cleanup and `gate recover`
do not free it.

```sh
docket gate history cleanup --repo-dir . --dry-run   # historical drives only — not slot evidence
```

### Drive disposition: WAITING / PASSED / FAILED / HALTED

The four outcomes of a `gate drive start` or `advance` call. `WAITING` means the drive is still live and this slice
ended. `PASSED` and `FAILED` mean the suite finished green or red. `HALTED` means it cannot continue safely: a changed
worktree, uncertain ownership, deadline expiry, bad state, or a process death.

**Used for:** keying the next step on `.outcome`, never on an exit status or log text. Only `FAILED` feeds repair. Only
`PASSED` exposes the run dir for evidence. `HALTED` stops automation and is never turned into a red suite.

### Gate drive / slice / owner generation / handoff / takeover

A **gate drive** runs a gate in resumable **slices** so no agent has to block for the whole suite.
Each drive has an **owner generation**; ownership moves by **handoff** (a single-use token the next
owner **claims**) or, when a child returned without handing off, by **takeover**. A **scope** binds
a drive to one parent/child dispatch boundary; the final PASSED/FAILED result is
**acknowledged** to close it.

**Used for:** the build and finalize suite gates. The component making these calls is the **gate
driver**. A forked worker drives the suite with inline, blocking `advance` calls — it must never
background the suite and yield. `prepare-scope` mints a recovery scope for one parent/child
dispatch boundary; `takeover` lets the parent reclaim a drive whose child returned without handing
off; `acknowledge` consumes the final PASSED/FAILED result and closes the scope (idempotent).

```sh
docket gate drive start   --repo-dir . --owner build --run-root <dir> --run-id <run-id> -- <suite argv>
docket gate drive advance --drive-id <id> --owner-gen <gen>
docket gate drive handoff --drive-id <id> --owner-gen <gen>
docket gate drive claim   --drive-id <id> --handoff-id <token>
docket gate drive prepare-scope --change-id 412 --task-id <id> --phase <name> --branch <name> --worktree <dir> --run-id <run-id>
docket gate drive takeover      --scope-id <id> --parent-cap <token>
docket gate drive acknowledge   --scope-id <id> --child-cap <token> --drive-id <id> --owner-gen <gen>
```

### Gate run / run dir

A **gate run** is a supervised local execution of a command (usually the test suite) launched
under the gate supervisor, with a durable **run dir** holding its record. It is started so it stays alive past the call
that started it. The **gate supervisor** is the `docket` binary re-run as a per-run supervisor. It starts the suite in a
new session with every stream sent to the run dir, and it writes an exact terminal record (the true exit code, or the
signal).

**Used for:** making a gate survive the harness killing the call that launched it. The supervisor holds the run's live
lock until the terminal record is written, so an observer sees either "still running" or "finished, here is how".

```sh
docket gate launch  --cwd <worktree> --root <run-root> -- ./run-tests.sh
docket gate observe <run-dir>
docket gate stop    <run-dir> --reason "wrong branch"
docket gate recover --root <run-root>
docket gate cleanup <run-dir>
```

### Worktree changed / certified input changed

The state a gate checked no longer matches the state now in front of it. A gate drive halts
`worktree-changed` when the worktree fingerprint (HEAD, index, status, live file bytes) moved
since the drive started, or when a takeover finds a drive whose recorded branch, worktree or
change is not the scope's. `evidence.recertify` refuses `certified-input-changed` when the PR
head or the build command moved after the gate passed. In finalize, a pull request whose pushed
head no longer equals the branch finalize just rebased and retested is refused `pr-head-mismatch`.

**Used for:** refusing to certify or merge something that was not verified. It is a halt, never a
red suite. Realign the pushed head (or undo the stray edit), then name the change id to run
finalize again.

### Launch-then-observe / detached run

The posture for anything that may outlast one foreground call. Launch it detached, get a key back right away, then make
short observe calls until a terminal result appears. Completion comes from a durable record, never from the launching
command returning.

**Used for:** long suite runs and delegated runs on other harnesses. A quiet run, or a stale "still running" report, is
not evidence that it crashed. The caller must never background a run and walk away.

### Liveness probe / moved to background

A **liveness probe** checks whether a recorded process is still there. Only a failed existence check proves the process
is gone. Any other non-zero answer means its liveness is *unprovable*, not that it died. A command **moved to background** is a
harness moving a still-running command into the background.

**Used for:** not declaring a run dead or finished too early. When a shell tool yields with a live task or session id,
keep that id and collect its real exit through the harness's own wait. Do not re-run the command or report completion.

```sh
docket gate launch  --cwd <worktree> --root <run-root> -- ./run-tests.sh   # primitive; workflows use gate drive
docket gate observe <run-dir>
```

### Observation budget (`gate_observation_budget` / `delegation_observation_budget`)

How long, in minutes, docket keeps watching for a terminal result. `gate_observation_budget` (default 30) covers a
suite run an agent started. `delegation_observation_budget` (default 60) covers a delegated runner child.

**Used for:** failing closed instead of waiting forever. When the gate budget runs out with no result, the build halts
for a human, and never counts it as red or as a pass. When the delegation budget runs out, docket kills the detached
process group and reports the run unavailable. `0` means observe once, then fail closed.

```yaml
# .docket.yml
gate_observation_budget: 30
delegation_observation_budget: 60
```

### Process recovery / gate history cleanup

**Process recovery** (`gate recover`) scans a run root and marks owned runs proven abandoned, keeping everything else.
**Gate history cleanup** assesses the gate-drive history recorded before worktree slots existed, and recovers the
records it safely can while keeping all evidence.

**Used for:** tidying old run records. Neither one frees a current [worktree slot](#worktree-slot). A cleanup that
reports zero blockers says nothing about whether the slot is free; inspect it with `gate observe` and settle it with
`gate stop`.

```sh
docket gate recover --root <run-root>
docket gate history cleanup --repo-dir . --dry-run    # preview; drop --dry-run to write markers
docket gate history cleanup --repo-dir . --drive-id <id>
```

### Tri-state verdict / halt exit code

A suite gate's result has three values, not two. **Green** is a finished run that exited zero. **Red** is a finished
failing run. **Halt** is a non-zero exit that the runner defines as a non-failure. "Still running" and "result
unavailable" are not verdicts; they end as budget halts.

**Used for:** never reading a halt as a pass or a fail. The run tracker reports a halt with its own exit code. That code
comes from the run's recorded state, not from how the gate found out the run stopped.

### `worktree-busy` / `launch-unconfirmed`

The two reasons a gate start is refused at a worktree's [worktree slot](#worktree-slot). `worktree-busy` means another
gate is live in that worktree. `launch-unconfirmed` means nothing proved whether an earlier launch happened.

**Used for:** recognising a blocking diagnostic, which is neither a red suite nor a retry trigger. It charges no suite
attempt. The fix is an operator act: let the incumbent finish, or stop it with `run.cancel`. A `launch-unconfirmed`
slot must be recovered or cancelled; restarting blind never clears it.

---

## Review

### Finding severity: blocker / important / minor

The severity levels a reviewer assigns. They decide which findings the fix loop routes and at what tier.

### Fix loop

The bounded in-branch repair that runs after review and before the PR opens: findings are routed
to build tiers as fix tasks (`review.max_fix_tasks`, default 10), then one full-suite run
confirms. Anything left unfixed becomes a line in the PR body.

### Review tier

One of three pinned reviewer agents — `docket-review-lean`, `docket-review-standard`,
`docket-review-deep` — all running the same read-only whole-branch contract. The tier is chosen
deterministically one step above the build: economy → lean, standard → standard, premium/max →
deep, bumped one step for a diff over 1500 changed lines.

**Used for:** a whole-branch review before the PR opens. Reviewers never fix, dispatch, or run the
suite.

---

## Finalize and close-out

### Closeout / closeout notes

**Closeout** is the final transition that archives the change (`done-archived`,
`stacked-merged`, or `root-archived` for a stack root). **Closeout notes** is the optional final
body section it writes (`### Verification`, `### Late findings`).

### Finalize

The close-out sequence: rebase onto the integration branch, retest, merge, archive. Each step gates
the next; a failed step stops the rest.

**Used for:** landing an approved or merged PR and clearing it off the backlog. Run it as the
`docket-finalize-change` agent, with or without an id (without, it auto-detects eligible changes).

```sh
docket context finalize --id 412 --json   # what finalize would act on
docket finalize rebase  --id 412 --revision <v> --head <sha>
docket finalize merge   --id 412 --revision <v> --head <sha>
docket finalize closeout --id 412
docket finalize cleanup  --id 412
```

### Finalize blocked / reason token / clear-block

When a gate failure needs a human, finalize writes `## Finalize blocked` with a typed **reason
token** (e.g. a mismatched PR head). Auto-detect runs skip a blocked change; naming its id
overrides that. A human clears the block explicitly. The token vocabulary and remedies live in the
finalize skill's `references/gate-failure.md`.

`finalize block` writes the marker (after first posting an owned PR comment); `clear-block` removes it.

```sh
docket finalize block --id 412 --revision <v> --pr-number 301 --attempt <token> \
  --reason <token> --head <sha> --input block-report.json
docket finalize clear-block --id 412 --revision <v> --head <sha> --pr-number 301
```

### Finalize drain / run outcome (`/loop docket-finalize-change`)

Every finalize run ends with one of four outcomes: `advanced` (it closed a change out), `contended`
(another writer got there first), `drained` (nothing eligible), or `halted` (a human is needed). A
driver continues on `advanced`/`contended` and stops on `drained`/`halted`, which are the same four
words implement-next uses.

**Used for:** closing out hands-free, one merge per iteration. Unlike the build drainer, this loop
**does merge**. Naming ids bounds the run and authorizes merges `require_pr_approval` would hold.

```sh
# inside an agent session
/loop docket-finalize-change             # every eligible implemented change
/loop docket-finalize-change 90,92,94    # only these ids (naming them is the authorization)
```

### Finalize gate

How finalize validates the rebased branch before merging: it rebases onto the integration branch and
re-runs the suite. `finalize.gate` is `local` (run
`finalize.test_command` here, default) or `off` (trust the PR's CI). `ci` and `both` are deferred in
Go v1 and block every repository mutation while set.

**Used for:** never merging a stale branch untested. It shares the worktree's single
[worktree slot](#worktree-slot), so it can be refused with `worktree-busy`.

### Finalize publish

The finalize step that pushes the rebased (or repaired) head to the remote feature branch, using
the receipt's exact lease. It then updates the PR's build-evidence block to match that head and
leaves the rest of the PR body byte-identical.

**Used for:** getting the head that finalize retested onto the PR before the merge. It never
creates a second PR. An `unknown` probe (`rewrite-unknown` / `pr-probe-failed`) stops the run with
no second mutation.

```sh
docket finalize publish --id 412 --attempt <token> --head <sha> --evidence evidence.json
```

### Finalize selection: auto-detect / explicit id / id allowlist

How finalize picks what to close out. **Auto-detect** (no id) takes the head of the policy-ordered
queue: merged-recovery first, then mergeable open PRs, smallest diff, then priority → age → id. An
**explicit id** or an **id allowlist** narrows that set, and naming the ids counts as the human's
authorization.

**Used for:** a named id or allowlist member overrides the `approval-required` and
`finalize-blocked` skips. It never overrides a real blocker such as `pr-closed`,
`dependency-unmerged`, or `draft`.

```sh
docket context finalize --json                     # auto-detect: the ordered candidate set
docket context finalize --id 412 --json            # exactly this change, even if skipped
docket context finalize --allowlist 90,92,94 --json
```

### Relink (`change relink`)

A human-gated fix for a change whose recorded `branch:` disagrees with its PR
(`branch-pr-head-mismatch`) or points at a branch that no longer exists (`branch-missing`). There
are exactly two options. **Trust the PR** adopts the PR's head as `branch:`. **Trust the record**
keeps `branch:` and points `pr:` at a PR the human names.

**Used for:** adopting the right PR or branch so finalize can proceed. Never guess or search for a
branch or PR. After a relink, re-read `context finalize --id` before any other finalize step.
Non-interactive callers halt instead of relinking.

```sh
docket change relink --id 412 --expect-revision <v> --adopt-pr-head --expect-pr 301 --expect-head <branch>
docket change relink --id 412 --expect-revision <v> --adopt-pr <pr-ref> --expect-branch <branch>
```

### Merge policy / branch protection

Whether a merge needs a human approval is settled before finalize runs. The single-maintainer path
is branch protection that requires a PR but zero approvals.

### Policy gate (`require_pr_approval`)

`finalize.require_pr_approval` is a policy check that a human authorised the merge. It is separate from the
[finalize gate](#finalize-gate). When `true`, auto-detect finalize refuses a PR whose review decision is not `APPROVED`.

**Used for:** requiring GitHub approval on unattended merges. Naming a change id (`docket-finalize-change 412`, or an id
list to `/loop`) counts as the authorisation and overrides it. The retest runs either way.

```yaml
# .docket.yml
finalize:
  require_pr_approval: true
```

### Rebase continue / rebase abort

These are the two ways out of a conflicted owned rebase. **Continue** feeds a resolver's report
back. It checks the reported paths against the live unmerged set, stages exactly those, and carries
on. **Abort** puts the branch back to the recorded original head and verifies that it did.

**Used for:** continue after a `resolved` resolver report. Abort after a `stuck` report, a spent
budget, or an unavailable resolver, and only once the resolver child is known to have finished.
A `receipt-write-failed` continue whose message says Git already advanced is recovered by
re-running the same continue, never by aborting.

```sh
docket finalize rebase-continue --id 412 --attempt <token> --input resolver-report.json
docket finalize rebase-abort    --id 412 --attempt <token> --input resolver-report.json
```

### Rebase receipt / attempt token (`--attempt`)

Before it changes anything in Git, `finalize rebase` writes an ownership-scoped **receipt** and
returns an opaque **attempt token** naming that owned rebase. Every later step of the same attempt
passes the token back: resolver reservation, continue or abort, publish, and block.

**Used for:** making sure only the attempt that owns a rebase can continue it, publish it, or
record a block against it. A token that does not match the receipt is refused before any push.
Never edit or delete a receipt by hand.

```sh
docket finalize rebase  --id 412 --revision <v> --head <sha>    # returns the attempt token
docket finalize publish --id 412 --attempt <token> --head <sha> --evidence evidence.json
```

### Rebase resolver / integration repair

The two specialised finalize workers, split at the rebase-completion boundary.
`docket-rebase-resolver` reconciles each conflicted hunk by intent
(`finalize.resolver_max_attempts`, default 10). `docket-integration-repair` makes a red rebased
suite green with a minimal fix, never weakening a test (`finalize.repair_max_attempts`, default 6).
Neither may be substituted inline.

### Repair sign-off (`repair-needs-signoff`)

A fix that `docket-integration-repair` wrote after the human approved the PR, so the human has not
seen it. It never merges unseen. An autonomous run records a `## Finalize blocked` marker with
reason `repair-needs-signoff` and halts. An attended run publishes the repair and asks before
merging.

**Used for:** keeping a machine-written fix out of `main` until a human has looked at it. After
reviewing the pushed repair on the PR, re-run finalize. The retry clears the block and merges.

```sh
docket finalize block --id 412 --revision <v> --pr-number 301 --attempt <token> \
  --reason repair-needs-signoff --head <repaired-sha> --input block-report.json
```

### Resolver reservation / resolver budget exhausted

Each conflict-resolver dispatch must first be admitted by a durable **reservation** under the owned
attempt. The binary enforces `finalize.resolver_max_attempts` (default 10), not the skill. Once that
budget is spent, the reservation returns `exhausted`, or a continue returns
`resolver-budget-exhausted`.

**Used for:** bounding how often finalize sends in `docket-rebase-resolver`. A `reserved` result
authorizes exactly one dispatch, and `pending` means one is already outstanding, so launch nothing.
An exhausted budget halts through the abort flow. Raising the limit applies only to the next
finalize attempt.

```sh
docket finalize resolver-reserve --id 412 --attempt <token>
```

### Retarget children

When a stack parent lands, finalize repoints its stacked children's PRs at the new base. Each child reports an outcome. `skipped-final` reports a done or killed child, and is also emitted for stacked-merged children (the child no longer needs its PR retargeted).

```sh
docket finalize retarget-children --id 412 --revision <v> --input children.json
```

### Sync integration

Fast-forwarding a clean primary checkout that is already on the integration branch to the freshly
fetched tip; runs at the end of finalize and of each maintenance sweep. Dispositions: `advanced`,
`already-current`, `skipped`, `refused`, `failed`.

```sh
docket repository sync-integration --repo-dir . --json
```

---

## Status, health, and maintenance

### Health check / health code

A health check is a status-time scan for things a human should look at: stale claims, broken
links, stalled dependencies. Each result carries a **finding code** (for example `artifact-missing`,
`deferred-setting`, `publish-deferred`) and a remedy.

```sh
docket status --json | jq '.findings[] | {code, message, remedy}'
docket repository check
```

### Repository check / migrate

`repository check` is the read-only topology and consistency check (including board drift).
`repository migrate` is the human-typed repair and migration path, never run by an agent.

```sh
docket repository check
docket repository migrate --repair-frontmatter
```

### Status

The read-only report of backlog state, readiness, selection, and repository health. It never writes
— not even the board.

```sh
docket status
docket status --priority high --type fix
docket status --records --json
```

### Status vs the merged-PR sweep

These are two different ways the backlog stays current. The **merged-PR sweep** is close-out: it
moves already-merged changes to `done`, archives them, and refreshes the board. That makes it the
safety net for when you skipped finalize. **Status** only reads and reports, including health
checks.

**Used for:** telling apart a refresh that writes from a read that never does. The `docket-status`
skill runs the sweep and then reads. The `docket status` command on its own never writes.

```sh
docket maintenance sweep --scope full   # the merged-PR sweep (writes)
docket status                           # the report (read-only)
```

### Sweep

The pass that observes merged PRs and closes their changes out to `done`. `maintenance.sweep`
runs the full scope; preflight runs the implementation scope.

```sh
docket maintenance sweep
docket maintenance sweep --scope implementation
```

---

## Skills, agents, and harnesses

### Abort-and-report

The rule every autonomous wrapper carries: an unmet precondition or blocking ambiguity is surfaced
and stopped on, never turned into an interactive prompt.

### Agent / wrapper

An agent is a separately launched worker with its own context, pinned to a model and effort. A
**wrapper** is the thin generated file that names the agent, pins its model and effort, and points
at the skill it loads. Wrappers are machine-local — regenerated per machine, never committed.

```sh
docket install                       # (re)generate skills, agents, and dispatch material
docket install --harness cursor
docket install check
```

### Agent enter

The Codex entry point that launches a registered docket role as a foreground root thread with the
right cwd, sandbox, approval policy, and (for feature children) the owning workflow's worktree.
Other harnesses dispatch named agents natively instead.

```sh
docket agent enter --role <role> --request req.md --cwd "$PWD" \
  --approval-policy <policy> --sandbox <mode> --run-id <run-id> --run-key <key>
```

### Cursor dispatch rule (`docket-dispatch.mdc`)

A generated Cursor rule that turns a directly invoked docket skill into a real dispatch of the
matching pinned subagent. Without it, Cursor runs the skill inline at the session model and the pin
is lost.

**Used for:** keeping model and effort pins on Cursor. It is machine-local and gitignored, written
user-level (`~/.cursor/rules/`) and per repo (`.cursor/rules/`) when `cursor` is in
`agent_harnesses`. Claude Code handles the same problem with `context: fork` frontmatter instead.

### `DIRECTED to:` marker

The house phrase an autonomous skill uses when it invokes a role skill. It states the outcome up
front, for example "**DIRECTED to:** write the plan file and stop there". The caller then answers any
choice the sub-skill poses from resolved config.

**Used for:** stopping an interactive prompt inside an invoked skill (such as a plan skill's
"which execution option?") from outranking the caller's autonomy. The caller logs one line only when
it actually suppressed such a prompt.

### Dispatch

Launching a named agent to do a step and waiting for it to return. Foreground dispatch means the
parent actively blocks — it never backgrounds a child and yields.

**Used for:** every workflow step that runs in its own agent. When a workflow has a registered
same-name `docket-*` agent, dispatch it rather than running the workflow inline.

### Dispatch fallbacks

What a workflow does when dispatch is genuinely unavailable; the fallback differs by kind.
**`inline`** (status, ADR): run the same work inline as a first-class equivalent. **`abstain`**
(the critic): abstain. **`auto-or-halt`** (plan writer, build, review, fix workers): proceed inline
only if the role is explicitly `auto`, otherwise halt (in Go v1 an explicit `skills.*` value blocks
mutation, so in practice it halts). **`no-fallback`** (the finalize rebase resolver and integration
repair): abort-and-report, never inline.

### docket-adr

The skill that records, supersedes, and reverses ADRs, and keeps the ADR index valid. Implement-next
dispatches it once for each non-obvious decision made during a build, and it returns the new ADR
number.

**Used for:** capturing *why* at the moment of decision. Invoke it directly for any decision nobody
has recorded yet. Its dispatch fallback is `inline`: without dispatch it runs inline with the same git-state contract.

```sh
/docket-adr        # in an agent session (forked into its pinned wrapper on Claude Code)
@docket-adr        # agent-dispatch: same pinned run, drillable in the TUI
```

### docket-auto-groom

The autonomous groomer. It drains every auto-groomable stub in one invocation. For each stub
it drafts a spec with an `## Assumptions` block, or a trivial verdict, then has the critic attack it.
Each stub exits as spec, trivial, or abstain.

**Used for:** grooming overnight or from a routine. Kill and defer are never autonomous. Arm stubs
by committing `auto_groomable: true`, or set the repo's `auto_groom: true`.

```sh
/docket-auto-groom
```

### docket-brainstorm

Docket's own brainstorm role. The parent holds
the design dialogue with you inline, then dispatches the pinned consultant once to author the spec
or return critique.

**Used for:** getting every spec authored or audited by a pinned high-tier consultant. It is invoked
from `docket-new-change` or `docket-groom-next`, never on its own. Asking for a consultant-written
spec in either session uses it for that run, (A `skills.brainstorm` binding is deferred in Go v1 and would block mutation.)


### docket-build

Docket's build role (`skills.build`, the default). It routes each plan task (`### Task N`) to one of
the four build-tier agents, allows one bounded escalation per task, skips per-task review, and
ends with a single full-suite build gate.

**Used for:** executing the plan inside implement-next's Step 5. A human does not invoke it
directly. Worker outcomes are `COMPLETE`, `WAITING`, `NEEDS_ESCALATION`, or `BLOCKED`, and a
malformed return halts the build. See *Build tier / escalation* and *Build gate*.

### docket-build-task

The per-task worker contract preloaded into the four `docket-build-*` tier agents. It owns
exactly one plan task: focused test, implementation, verification, self-review, and one commit.

**Used for:** the unit of work under docket-build. It returns `COMPLETE`, `NEEDS_ESCALATION`, or
`BLOCKED`, and it is never invoked directly by a human.

### docket-convention

The shared contract every docket skill loads first, as the blocking Step 0. It defines
configuration, directory layout, the change manifest and lifecycle, the ADR format, readiness and
selection, the bootstrap guard, and the branch model.

**Used for:** the single source of docket vocabulary. It is pure reference: it performs no reads,
writes, or git operations, and it is injected into most generated wrappers rather than run as an
agent.

### docket-finalize-change

The close-out sequencer for an approved or merged PR. It retargets stacked children, rebases,
re-tests, publishes, merges once, archives, and cleans up, then reports `advanced`, `contended`,
`drained`, or `halted`. It is the one place docket itself merges.

**Used for:** landing work now instead of waiting for the sweep. It is not forked, because it keeps
real prompts. Naming ids authorizes a headless drive and overrides the `approval-required` and
`finalize-blocked` skips. See *Finalize*.

```sh
/docket-finalize-change 90
/loop docket-finalize-change 90,92,94   # one merge per iteration, stops on drained/halted
```

### docket-groom-next

The interactive groomer. It selects the next needs-grooming stub (or the id you name), opens with a
cold-start recap, and brainstorms it with you. It exits with spec, trivial, kill, defer, revise, or
re-enable. It never takes a claim and never mints ids.

**Used for:** designing stubs with a human in the loop. Naming an already-groomed id routes to
`revise`. It runs inline at the session model, and the model it recommends is advisory.

```sh
/docket-groom-next
/docket-groom-next 412
```

### docket-new-change

The interactive producer, and the entry point a human runs to propose work. It allocates the change
through `change.create`, brainstorms the design with you, and grooms it to a spec in one step. The
trivial path skips the brainstorm; scan mode mints stubs.

**Used for:** capturing an idea as a tracked change. It writes markdown only, never branches or
code, and it stops at the spec. To adjust a spec afterwards, use `docket-groom-next <id>` rather than
re-running this.

```sh
/docket-new-change
```

### docket-review

Docket's review role (`skills.review`, the default). A bounded, read-only reviewer reads the branch
diff and the build-evidence record and returns findings ranked by severity. It never fixes,
dispatches, or runs the suite.

**Used for:** implement-next's Step 6 whole-branch review. It runs through one of the three review
tier agents and is never invoked by a human. See *Review tier* and *Fix loop*.

### docket-status

The backlog read-and-janitor skill. A see-only request runs the write-free `status` read. An explicit
refresh first runs `maintenance.sweep --scope full` (close out merged PRs, retry cleanups, run health
checks, sync the integration branch), then reads.

**Used for:** knowing what is ready, stuck, or merged, and recovering a PR merged with the GitHub
button. Health checks are warn-only: it never auto-fixes. Its dispatch fallback is `inline`: without dispatch it runs
inline.

```sh
/docket-status     # skill-invoke: forked, cheapest
@docket-status     # agent-dispatch: same pinned run, drillable live
```

### Fork / forked skill

On harnesses that support it, a skill can run as a fork of the current context rather than a fresh
agent. A fork's `completed` report is not proof it finished — verify git state.

### Fork-exclusion principle

Only skills that never need the human mid-run are forked, because a forked subagent has no channel
back to the human. The forked skills are `docket-status`, `docket-adr`, `docket-implement-next`, and
`docket-auto-groom` (`context: fork` + `agent:` frontmatter).

**Used for:** explaining why `docket-new-change`, `docket-groom-next`, and `docket-finalize-change`
run inline. A headless finalize is authorized by naming ids instead.

### Harness

The tool that runs the agent: Claude Code, Cursor, Codex, or opencode.

**Used for:** targeting generated wrappers (`agent_harnesses:`) and picking per-harness model
defaults from the shipped sidecar `agents/harness-defaults.yml`.

### Harness defaults sidecar (`agents/harness-defaults.yml`)

The file docket ships with the built-in model and effort for every agent on every harness. It is
program data, not user config. Each harness block is complete, and it never carries `runner:` or
a neutral `default:` block.

**Used for:** seeing the shipped pins. Don't edit it; override a pin from a config layer instead.
`.docket.example.yml` mirrors it value for value.

### Interactive vs autonomous skills

**Autonomous** skills have a generated, model- and effort-pinned wrapper and carry the
abort-and-report rule: implement-next, auto-groom, finalize-change, status, adr, build-task, and
review. **Interactive** skills (`docket-new-change`, `docket-groom-next`) stay inline at the session
model, because the conversation with you is the point. They only advise a model at startup.

**Used for:** knowing which skills you can dispatch and walk away from, and which need you at the
keyboard.

### Managed dispatch block (`docket:dispatch`)

A marker-bounded block (`<!-- docket:dispatch:start … -->` … `end`) that docket writes into a repo's
parent-facing instructions file, such as `CLAUDE.md` or `AGENTS.md`. It tells the parent to dispatch
the same-name `docket-*` agent and to bracket implement-next runs with the run tracker.

**Used for:** routing a plain request to the pinned agent. It is committed and machine-neutral:
agent names and prose only, never a model ID. The install writes or retires it according to
`agent_harnesses`. Never hand-edit inside the markers.

```sh
docket install    # reconcile the block for this repo's agent_harnesses
```

### Model / effort, pinned vs unpinned wrapper

A **pinned** wrapper carries a resolved `model` and `effort`, resolved field by field in this order:
`agents.<harness>.<agent>` → `agents.default.<agent>` → the shipped sidecar. When nothing resolves,
the wrapper is **unpinned** and the harness applies its own default.

**Used for:** matching the tier to the task rather than the session. `effort: auto` drops the
effort line, while leaving `effort:` out keeps the shipped effort, so the two differ. `model:
inherit` means "the parent's model" on Claude Code and "no pin" on other harnesses.

```yaml
# ~/.config/docket/config.yml
agents:
  default:
    status: { model: <id>, effort: low }
```

### Restart after regenerating wrappers

Harnesses register agents and read instruction files only when their process starts. After any
install that changed a wrapper, a skill's frontmatter, or a dispatch block, start a fresh harness
process. Clearing the conversation or opening a new chat in the same process is not enough.

**Used for:** avoiding a new fork that seems to do nothing, a healthy pin that looks broken, or an
"agent type not found" error in an old session.

### Sandbox / allowlisting the binary

Cursor decides whether a command runs through three independent gates: command approval (`terminalAllowlist`), filesystem access, and network. Docket needs the network to fetch, rebase, and push, so under a sandboxed harness it must run
**outside** the sandbox. On Cursor, only a `terminalAllowlist` entry for `docket` does that. Granting
read paths or network in `sandbox.json` leaves the command sandboxed.

**Used for:** running docket under Cursor's Allowlist (with Sandbox) mode. Allowlisting `docket`
authorizes every operation it has, including pushes and deletions. Allowlist your own build commands
separately, and never allowlist something broader like `eval` or `bash`.

```json
{ "terminalAllowlist": ["docket"] }
```

(That fragment goes in `~/.cursor/permissions.json`. Invalid JSON there silently disables the
whole allowlist.)

### Skill

A named, reusable instruction set an agent loads for one job (a `skills/<name>/SKILL.md`).

**Used for:** holding a workflow's instructions once, independent of the tool that runs them. The
shared contract every docket skill loads first is `docket-convention`.

### Two invocation paths: skill-invoke vs agent-dispatch

There are two ways to reach a pinned wrapper, and both run at the same model and effort.
**Skill-invoke** (`/docket-status`, forked on Claude Code) is cheapest but shows only
`completed (forked execution)`. **Agent-dispatch** (`@docket-status`) costs one dispatch turn but
can be drilled into live.

**Used for:** use agent-dispatch when you want to watch a long run, and skill-invoke for everything
else.

```sh
# inside an agent session
/docket-status     # skill-invoke (forked)
@docket-status     # agent-dispatch (drillable)
```

---

## Configuration

### `agent_harnesses`

The list of harnesses docket generates wrappers and dispatch material for. In a repo file,
**presence** is the opt-in. It has three states: absent (keep the shipped Claude-only default), a
non-empty list (reconcile exactly those harnesses), or `[]` (retire every docket-owned repo
surface).

**Used for:** turning on Cursor, Codex, or opencode. A **global** value only scopes the user-level
pass and never opts a repo in, so Codex's `AGENTS.md` block needs the repo's own key. Re-run the
install after changing it.

```yaml
# .docket.yml (whole team) or .docket.local.yml (this clone)
agent_harnesses: [claude, codex]
```

### Attempt budgets (`run.max_attempts` / `build.max_attempts` / finalize repair)

Separate caps on how many tries each layer gets. `run.max_attempts` (default 2) counts whole implement-next attempts
per change: the first plus retries. `build.max_attempts` (default 4) counts full-suite runs in one build phase.
`finalize.resolver_max_attempts` (10) and `finalize.repair_max_attempts` (6) cap rebase-conflict resolution and
post-rebase repair.

**Used for:** knowing which budget ran out. `1` turns off retries or repair at that layer. A new value applies to the
next run or phase you start, never one already running. Explicit halts need a human whatever the budget. A cancel
spends no attempt.

```yaml
# .docket.yml
run:
  max_attempts: 2
build:
  max_attempts: 4
```

### Auto-capture / discovered work

Work an autonomous run discovers mid-run is reported in its final report, never silently minted.
`auto_capture` is parsed but deferred from Go v1 — capture deliberately with `docket change create`.

### `board.section_order` / `board.sorting`

Presentation keys for the inline board. `section_order` must list every section exactly once
(`in-progress, built, blocked, groomed, proposed, deferred` by default). `sorting.<section>.by` is
`id`, `updated`, or `created`, and `.direction` is `asc` or `desc` (default `updated`/`desc`).

**Used for:** arranging `BOARD.md` to taste. It is presentation only: selection, the digest, and
lifecycle never read it. An invalid list is warned about and ignored as a whole, so a lower layer or
the built-in order applies.

```yaml
board:
  section_order: [in-progress, built, blocked, groomed, proposed, deferred]
  sorting:
    proposed: { by: created, direction: asc }
```

### `build.checkpoint`

Whether docket-build keeps a resume ledger. `false` (default) keeps none: a resumed run rebuilds its
progress from the plan, commits, code, and tests. `true` writes a compact ledger to the gitignored
`.superpowers/docket-build/<change-id>/progress.md`.

**Used for:** cheaper resumes of long builds. With `true`, a task is skipped on resume only when its
entry is COMPLETE, the plan hash still matches, and its commit is an ancestor of the branch. Any value
other than `true`/`false` is a config error.

### Change types

The allowed `type:` values (`change_types`, default `chore, docs, feat, fix, refactor, perf`). The
type also becomes the feature-branch prefix.

### Config layers

Four layers resolved per key, lowest to highest: shipped defaults → global
`~/.config/docket/config.yml` → committed `.docket.yml` → gitignored `.docket.local.yml`. Nested
blocks merge leaf by leaf. Full shape and defaults: `.docket.example.yml` (see
[`config-keys.md`](config-keys.md)). In Go v1 an `agents:` model/effort pin is honoured only from the
global file; the same pin in `.docket.yml` or `.docket.local.yml` blocks mutation.

```sh
docket diagnostic config --repo-dir . --json
```

### Coordination key / scope tag / shared-setting guard

A coordination key is a config key whose value must be identical for every clone, so it may only be
set in the committed repo config. The **shared-setting guard** ignores (with a warning) a coordination key set in
any other layer. The warning's code is `shared-setting-ignored`. Each key's **scope tag** in the example file is `repo-only`, `any layer`, or
`local-only`.

### Dummy mode / persona / "In plain terms"

`dummy_mode` calibrates human-facing prose to a described reader (the **persona**). Dialogue and
reports are rewritten; results, change sections, and PR bodies get an additive
`### In plain terms` block. Agents never read that block as a decision input.

### `finalize.skip_results_only_delta`

When `true`, finalize's gate accepts a near-match as proof the suite already passed: the tested
commit is an ancestor of the merge head, and every file added since then is under `results_dir`.
Default `false`.

**Used for:** skipping the redundant re-run caused by the results file committed after testing. Turn
it on only if no test reads files from `results_dir`. It is repo-only (shared-setting guarded), because
it states a fact about one repo's suite.

### GitHub board mirror / `github_project`

A mirror of the board to GitHub Issues and a Projects v2 board, requested with the `github` token
of `board_surfaces` and the `github_project` key. Neither works in Go v1. `inline` (`BOARD.md`) is
the only supported surface.

**Used for:** nothing yet. `github_project` is inert: it is read by nothing and only its
shared-setting guard runs. A `github` token in the committed `board_surfaces` blocks every mutation
until you remove it.

### Inert / deferred setting

A config key that is parsed but activates nothing in the current binary (for example
`terminal_publish`, `auto_capture`). Status surfaces them as `inert-setting` / `deferred-setting`
findings.

### `learnings.enabled` / `learnings.cap`

`learnings.enabled` (default `true`) is the read gate for the learnings subsystem. With `false`,
readers perform zero learnings reads. `learnings.cap` (default 300) is the count of active findings
(`retained` + `candidate`) past which the ledger needs human curation.

**Used for:** switching the memory off, or signalling when it needs pruning. `false` is never a
purge: files stay byte-untouched and re-enabling resumes from them. Promoted findings do not count
against the cap, and docket never auto-merges findings.

### Managed global config

The user-level `~/.config/docket/config.yml`. It uses the same schema as `.docket.yml`, and a repo's
committed file wins over it key by key. The installer writes a minimal copy on first run and fills
in the values it manages on later runs without overwriting yours.

**Used for:** machine-wide preferences, such as per-agent model and effort. A
`~/.config/docket/.docket.yml` is never read, and an old `agents.yaml` there is migrated in
automatically.

```sh
docket diagnostic config --repo-dir . --json   # shows which layer each value came from
```

### Priority

`critical` > `high` > `medium` (default) > `low` — the first key of selection order.

### `reclaim.auto` / `reclaim.lease_ttl`

`reclaim.lease_ttl` (default 72 hours) is how long a claim lease lasts. `reclaim.auto` (default
`false`) decides what happens to an expired, branchless claim. With `false`, status only flags and
recommends it; with `true`, each maintenance sweep reclaims it back to `proposed`.

**Used for:** letting crashed runs self-heal without a human. Detection is always on; only the
mutation is opt-in. A claim with a branch is never reclaimed automatically.

```sh
docket change reclaim --id 412 --revision <v>   # the explicit, one-off reclaim
```

### Reserved type tokens `all` / `untyped`, and migrating to typed changes

`all` and `untyped` match the change-type grammar but are reserved, so they may never be configured
in `change_types` or stored in a change's `type:`. A change with no stored type renders `untyped` in
the board's Type cell. Migrating an older backlog means writing a `type:` onto each untyped active
change once. Archived changes are never reclassified.

**Used for:** keeping reports legible. `change.create` refuses an empty or unknown type, so the
untyped set only shrinks. The scalar `auto_capture: true` from before the map form is a hard error.

### `review.min_fix_severity`

The lowest review-finding severity that implement-next's fix loop repairs in-branch before the PR
opens: `minor` (default; fix everything), `important` (minors go in the PR body), or `blocker`.
Blockers are always fixed.

**Used for:** trading PR polish against run cost. The implementer applies the threshold, not the
reviewer. Pair it with `review.max_fix_tasks`, which caps non-blocker fix tasks per run.

---

## Operations and the CLI protocol

### Capability catalog

The machine-readable list of every operation the `docket` binary offers, which skills read instead
of hard-coding commands. Each entry carries its `argv`, signature, and **effects**.

```sh
docket capabilities --json | jq -r '.commands[] | "\(.id)\t\(.argv|join(" "))"'
```

### Allowed values (operation dispositions)

A fixed, schema-published set of allowed tokens. Automation keys on these tokens, never on prose or
exit codes. `docket schema --json` lists them under `.vocabularies`; beside the general ones
(`statuses`, `results`, `effects`, `finding_codes`, `change_types`) sit the operation-specific sets
below.

| Vocabulary | Members | Reported / accepted by |
|---|---|---|
| `block_dispositions` | `recorded` `already` `cleared` `nothing-to-clear` `unknown` `contended` `refused` `failed` | `finalize block`, `finalize clear-block` |
| `cancel_dispositions` | `cancelled` `already-cancelled` `cancellation-pending` `refused` | `run cancel` |
| `claim_dispositions` | `applied` `already-claimed` `contended` `failed` `run-context-invalid` `run-context-conflict` | `change claim`, `change refresh-claim` |
| `cleanup_dispositions` | `cleaned` `already-clean` `pending` `retained` `children-retarget-required` `rebase-scratch-cleared` | `finalize cleanup`, `gate cleanup` |
| `closeout_dispositions` | `done-archived` `stacked-merged` `root-archived` `already` `children-retarget-required` `contended` `blocked` `unknown` `failed` | `finalize closeout` |
| `groom_outcomes` | `spec` `trivial` `revise` `abstain` `re-enable` | `change groom` (request `outcome`) |
| `halt_dispositions` | `halted` `resumed` `contended` `refused` `failed` | `change halt`, `change resume-halted` |
| `merge_dispositions` | `merged` `already-merged` `contended` `not-mergeable` `denied` `blocked` `unknown` | `finalize merge` |
| `publish_dispositions` | `published` `noop` `contended` `unknown` `blocked` | `finalize publish` |
| `reclaim_dispositions` | `reclaimed` `skipped` `contended` `failed` | `change reclaim` |
| `reconcile_dispositions` | `applied` `contended` `failed` | `change reconcile` |
| `sync_dispositions` | `advanced` `already-current` `failed` `refused` `skipped` | `repository sync-integration`, `maintenance sweep` |
| `section_intents` | `preserve` `replace` `remove` | `change groom` (request `sections[].intent`) |
| `priorities` | `critical` `high` `medium` `low` | `change create` (request `priority`), `status --priority` |

**Used for:** writing drivers that branch on outcomes safely. Read an `unknown` as "state
unobservable; retain", which authorizes nothing destructive.

```sh
docket schema --json | jq -c '.vocabularies.merge_dispositions'
```

### Conflict-checked write / push-retry

A **conflict-checked write** is a write that succeeds only if the record still matches the record
revision you read. Docket pairs it with an exact-lease push to the metadata remote. **Push-retry** is
the recovery when that race is lost: re-run `repository.prepare`, re-read the path and revision,
then retry.

**Used for:** letting several sessions and loops share one backlog without silent overwrites. A lost
race returns `contended` and writes nothing. It is also why grooming needs no claim: the conflict-checked
final push already protects it.

```sh
docket repository prepare --repo-dir . --json   # step 1 of every retry: re-sync
docket status --json | jq -r '.changes[] | select(.id==412) | .revision'   # step 2: fresh revision
```

### Contended

The outcome when a conflict-checked write lost a race with another writer (another session or loop). It
is not a failure of your input: re-read and retry.

### Diagnostic runtime

A read-only report on the binary itself: the Go toolchain it was built with, the OS/architecture it
targets, and whether that target is supported. It works without a completed install.

**Used for:** a quick sanity check when docket misbehaves on a new machine. It says nothing about
Bash, config, or harnesses; use `diagnostic config` for config.

```sh
docket diagnostic runtime --json
```

### Digest / digest-only read

The **backlog digest** is the structured `status` payload: `summary` counts, one `changes` entry per
displayed change, and the ordered `ready` queue. A **digest-only read** is a read that produces the
digest without any write. ADR-0047 introduced it as the Bash-era `docket-status --digest-only` flag;
in the Go binary, plain `docket status` is that read.

**Used for:** selecting work without side effects: a selection read must never also be a write.
Summarize backlog state from the digest, never by opening `BOARD.md`.

```sh
docket status --json | jq '{summary, ready}'
```

### Effects

The closed set describing what an operation may touch: `read`, `local-write`, `metadata-write`,
`external-write` (GitHub, pushes), `process-control`. A workflow stops if an operation's effects
exceed what it is authorised to do.

### Finding / finding code / remedy

A structured diagnostic attached to a result: a `code`, a `severity`, the entity it concerns, a
`message`, and a `remedy` naming the next command. The full code list is the `finding_codes`
vocabulary.

### Install / version tree / install collect

`docket install` puts docket's skills, agents, and dispatch material into each harness as one
journaled, all-or-nothing transaction. Each installed asset set is stored as an immutable
**version tree** (`versions/<asset-set-id>/`). After every successful or no-op install or
uninstall, a best-effort pass collects the trees nothing references any more.

**Used for:** keeping the machine current. Run `install collect` by hand only when a
`collection-pending` warning asks for it. Anything that can't be proven unreferenced is kept.
Never edit anything under `versions/` yourself.

```sh
docket install                        # or: bash ~/dev/docket/install.sh
docket install --harness cursor --repo-dir <repo>
docket install collect --dry-run
```

### Install check

A read-only check of whether this machine's installation is current. It also works as a CI gate:
it fails if the managed `.gitignore` block is missing or stale, if a generated file is tracked, or
if the committed `.docket.yml` uses the legacy bare `agents:` shape.

**Used for:** catching drift before it bites. If only the generated content has drifted, the check
still passes and just suggests re-running the install.

```sh
docket install check
```

### Keeping docket current

After every pull of a new docket version, re-run the install. Pulling alone updates only the skill
symlinks. Wrappers, new harness support, managed global config, and dispatch surfaces are updated
only by an install run.

**Used for:** upgrading safely. Run the machine install first, then any per-repo steps in the
release notes, then restart the harness. Merges to docket's own `main` use the verified rebuild in
`AGENTS.md`.

```sh
cd ~/dev/docket && git pull
bash ~/dev/docket/install.sh          # same engine as: docket development install --source ~/dev/docket
docket version                        # confirm the installed commit
```

### Operation / operation id

A single `docket` capability with a stable dotted id (`change.claim`, `run.verdict`,
`finalize.merge`). Skills resolve the command for an id from the catalog rather than hard-coding it.

### Protocol-v1 envelope

The JSON shape every `--json` output shares: `protocol_version`, `operation`, `result`, then
operation-specific fields and `findings`.

### `request_id` / `replayed` (idempotent replay)

`request_id` is a caller-chosen idempotency key on the create-style operations (`change.create`,
`adr.record`, `adr.supersede`, `adr.reverse`, `learning.record`). Retrying with the same key after a
lost response returns the original result with `replayed: true` rather than creating a duplicate.

**Used for:** safe retries when you cannot tell whether the first attempt landed. Pick one stable key
per logical create and reuse it on every retry.

```sh
docket schema --operation change.create   # request_id is required; result carries replayed
```

### Result / disposition

The **result** is the envelope's top-level outcome (`applied`, `no-op`, `contended`,
`invalid-input`, `invalid-state`, `blocked`, `gate-failed`, …). A **disposition** is the one-word
outcome an operation reports: applied, no-op, refused, or error — plus operation-specific closed
sets (`claim_dispositions`, `merge_dispositions`, `sync_dispositions`, …) listed by `docket schema`.

### Schema / request file

`docket schema` emits every operation's request and result fields plus the allowed values.
A **request file** (`--request` / `--input`) is a JSON body built from that schema.

```sh
docket schema --operation change.kill
docket schema --json | jq '.vocabularies | keys'
```

### Uninstall

Removes the harness integrations docket recorded (skill symlinks, agent wrappers, managed dispatch
blocks) and nothing else. The CLI binary, global config, and each repo's docket setup all stay.

**Used for:** retiring docket from one harness or all of them. It is ownership-safe with no
`--force`: any file or block that no longer matches what docket recorded is left alone and
reported.

```sh
docket uninstall --dry-run
docket uninstall --harness cursor
```

### Version / development install

`docket version` reports the binary's build identity (full commit). `development install` rebuilds
and installs docket from a source checkout.

```sh
docket version
docket development install --source ~/dev/docket
```

---

## Obsolete terms

Retired features. Their config keys are still recognised, so a stale file gets a warning or a refusal
instead of being silently accepted; nothing in current docket uses them.

### Runner delegation

**Runner delegation** handed an agent's whole run to a different harness, chosen by an explicit
`runner:` key on that agent. It is retired in Go v1 (change 0371): any `agents.<h>.<a>.runner` value
blocks every repository mutation until removed. See [Runner shim / `runners` block](#runner-shim--runners-block).

### Runner shim / `runners` block

These are the config for runner delegation. `runners.<name>` holds per-runner knobs:
`codex.sandbox`, `codex.network`, `opencode.permissions`, and `shim_model` / `shim_effort`, which
pin the small relay agent that runs in your own harness.

**Used for:** nothing in Go v1. Cross-harness delegation is retired (change 0371). Any
`agents.<h>.<a>.runner` value blocks mutation, and every `runners.*` key is an inert companion
(`inert-setting`).

### `runtime.bash`

A former key that named the path to Bash 4 or newer for docket's shell scripts. The Bash runtime is
gone, so the key is now warned about and ignored in every layer.

**Used for:** nothing today. Delete it if a diagnostic flags it. `.docket.example.yml` still shows
it as live, but that text is out of date.

### Terminal publish

Terminal publish (also called **selective publish on close-out**) was the opt-in copying of archived
records onto the integration branch. It is deferred from Go v1: `terminal_publish: false` is inert,
and `true` blocks every repository mutation until you remove it.

**Used for:** nothing in Go v1. The integration branch gets code, plans, and results through PRs alone.

---

## Alphabetical index

- [## Artifacts block](#-artifacts-block)
- [## Why deferred / ## Why killed](#-why-deferred---why-killed)
- [Abort-and-report](#abort-and-report)
- [Abstain / re-enable](#abstain--re-enable)
- [Adopting docket in a repository](#adopting-docket-in-a-repository)
- [ADR](#adr)
- [ADR index / ## Update note](#adr-index---update-note)
- [ADR status: Accepted / Superseded by / Reversed by / Deprecated](#adr-status-accepted--superseded-by--reversed-by--deprecated)
- [Adversarial gate](#critic) — see Critic
- [Agent / wrapper](#agent--wrapper)
- [Agent enter](#agent-enter)
- [agent_harnesses](#agent_harnesses)
- [Allowed values (operation dispositions)](#allowed-values-operation-dispositions)
- [Archive](#archive)
- [Archived record](#archived-record)
- [Attempt budgets (run.max_attempts / build.max_attempts / finalize repair)](#attempt-budgets-runmax_attempts--buildmax_attempts--finalize-repair)
- [Attribution / unattributed read](#attribution--unattributed-read)
- [Auto-capture / discovered work](#auto-capture--discovered-work)
- [Auto-groom / auto-groomable](#auto-groom--auto-groomable)
- [Backlog](#backlog)
- [Block / unblock, defer / revive, kill](#block--unblock-defer--revive-kill)
- [Board](#board)
- [board.section_order / board.sorting](#boardsection_order--boardsorting)
- [Bootstrap guard](#bootstrap-guard)
- [Bootstrap verdict: PROCEED / STOP_MIGRATE / CREATE_ORPHAN](#bootstrap-verdict-proceed--stop_migrate--create_orphan)
- [Brainstorm / consultant](#brainstorm--consultant)
- [Budget watch / serially confirmed breach](#budget-watch--serially-confirmed-breach)
- [Build evidence](#build-evidence)
- [Build gate](#build-gate)
- [Build tier / escalation](#build-tier--escalation)
- [build.checkpoint](#buildcheckpoint)
- [Cancel](#cancel)
- [Capability catalog](#capability-catalog)
- [Capture modes: designed / rough stub / trivial / scan](#capture-modes-designed--rough-stub--trivial--scan)
- [Change](#change)
- [Change types](#change-types)
- [Change version / entity version](#revision---revision) — see Revision
- [Claim / claim lease / reclaim](#claim--claim-lease--reclaim)
- [Close-out](#closeout--closeout-notes) — see Closeout / closeout notes
- [Closeout / closeout notes](#closeout--closeout-notes)
- [Config layers](#config-layers)
- [Conflict-checked write / push-retry](#conflict-checked-write--push-retry)
- [Contended](#contended)
- [Continuation](#continuation)
- [Coordination key / scope tag / shared-setting guard](#coordination-key--scope-tag--shared-setting-guard)
- [Critic](#critic)
- [Cursor dispatch rule (docket-dispatch.mdc)](#cursor-dispatch-rule-docket-dispatchmdc)
- [Cursor's three gates](#sandbox--allowlisting-the-binary) — see Sandbox / allowlisting the binary
- [Dependency (depends_on) / implicitly blocked](#dependency-depends_on--implicitly-blocked)
- [Derived view / generated block / backlink](#derived-view--generated-block--backlink)
- [Development test (docket development test)](#development-test-docket-development-test)
- [Diagnostic runtime](#diagnostic-runtime)
- [Digest / digest-only read](#digest--digest-only-read)
- [DIRECTED to: marker](#directed-to-marker)
- [Dispatch](#dispatch)
- [Dispatch fallbacks](#dispatch-fallbacks)
- [docket-adr](#docket-adr)
- [docket-auto-groom](#docket-auto-groom)
- [docket-brainstorm](#docket-brainstorm)
- [docket-build](#docket-build)
- [docket-build-task](#docket-build-task)
- [docket-convention](#docket-convention)
- [docket-finalize-change](#docket-finalize-change)
- [docket-groom-next](#docket-groom-next)
- [Docket-mode / single-branch mode](#docket-mode--single-branch-mode)
- [docket-new-change](#docket-new-change)
- [docket-review](#docket-review)
- [docket-status](#docket-status)
- [Drive disposition: WAITING / PASSED / FAILED / HALTED](#drive-disposition-waiting--passed--failed--halted)
- [Dummy mode / persona / "In plain terms"](#dummy-mode--persona--in-plain-terms)
- [Effective auto-groomable](#auto-groom--auto-groomable) — see Auto-groom / auto-groomable
- [Effects](#effects)
- [Escalation (NEEDS_ESCALATION)](#build-tier--escalation) — see Build tier / escalation
- [Feature branch](#feature-branch)
- [Final status](#change-lifecycle-and-statuses)
- [Finalize](#finalize)
- [Finalize blocked / reason token / clear-block](#finalize-blocked--reason-token--clear-block)
- [Finalize drain / run outcome (/loop docket-finalize-change)](#finalize-drain--run-outcome-loop-docket-finalize-change)
- [Finalize gate](#finalize-gate)
- [Finalize publish](#finalize-publish)
- [Finalize selection: auto-detect / explicit id / id allowlist](#finalize-selection-auto-detect--explicit-id--id-allowlist)
- [finalize.skip_results_only_delta](#finalizeskip_results_only_delta)
- [Finding / finding code / remedy](#finding--finding-code--remedy)
- [Finding severity: blocker / important / minor](#finding-severity-blocker--important--minor)
- [Fix loop](#fix-loop)
- [Focused tests / task gate](#focused-tests--task-gate)
- [Fork / forked skill](#fork--forked-skill)
- [Fork-exclusion principle](#fork-exclusion-principle)
- [Frozen build record](#frozen-build-record)
- [Gate](#gate)
- [Gate drive / slice / owner generation / handoff / takeover](#gate-drive--slice--owner-generation--handoff--takeover)
- [Gate driver](#gate-drive--slice--owner-generation--handoff--takeover) — see Gate drive / slice / owner generation / handoff / takeover
- [Gate run / run dir](#gate-run--run-dir)
- [Gate supervisor](#gate-run--run-dir) — see Gate run / run dir
- [Git hooks in docket worktrees (pre-commit, husky, lefthook)](#git-hooks-in-docket-worktrees-pre-commit-husky-lefthook)
- [GitHub board mirror / github_project](#github-board-mirror--github_project)
- [Groom](#groom)
- [Groom outcome revise](#groom-outcome-revise)
- [Halt / resume-halted](#halt--resume-halted)
- [Harness](#harness)
- [Harness defaults sidecar (agents/harness-defaults.yml)](#harness-defaults-sidecar-agentsharness-defaultsyml)
- [Health check / health code](#health-check--health-code)
- [Id / slug](#id--slug)
- [Implementation context](#implementation-context)
- [Implement-next / the drainer](#implement-next--the-drainer)
- [Inert / deferred setting](#inert--deferred-setting)
- [Install / version tree / install collect](#install--version-tree--install-collect)
- [Install check](#install-check)
- [Integration branch](#integration-branch)
- [Interactive vs autonomous skills](#interactive-vs-autonomous-skills)
- [Keeping docket current](#keeping-docket-current)
- [Launch-then-observe / detached run](#launch-then-observe--detached-run)
- [Learnings / finding / promotion](#learnings--finding--promotion)
- [Learnings index / read on demand](#learnings-index--read-on-demand)
- [Learnings ledger / war story / promotion_state](#learnings-ledger--war-story--promotion_state)
- [learnings.enabled / learnings.cap](#learningsenabled--learningscap)
- [Liveness probe / moved to background](#liveness-probe--moved-to-background)
- [Managed dispatch block (docket:dispatch)](#managed-dispatch-block-docketdispatch)
- [Managed global config](#managed-global-config)
- [Manifest](#manifest)
- [Mark implemented](#mark-implemented)
- [Marker section](#marker-section)
- [Merge policy / branch protection](#merge-policy--branch-protection)
- [Metadata branch](#metadata-branch)
- [Metadata worktree](#metadata-worktree)
- [Model / effort, pinned vs unpinned wrapper](#model--effort-pinned-vs-unpinned-wrapper)
- [Observation budget (gate_observation_budget / delegation_observation_budget)](#observation-budget-gate_observation_budget--delegation_observation_budget)
- [Operation / operation id](#operation--operation-id)
- [Owned sections / section intents (preserve / replace / remove)](#owned-sections--section-intents-preserve--replace--remove)
- [Owner-lifecycle caveat](#cancel) — see Cancel
- [Plan](#plan)
- [Plan writer](#plan-writer)
- [Policy gate (require_pr_approval)](#policy-gate-require_pr_approval)
- [PR handoff](#pr-handoff)
- [PR publish](#pr-publish)
- [Preflight](#preflight)
- [Priority](#priority)
- [Process recovery / gate history cleanup](#process-recovery--gate-history-cleanup)
- [Protocol-v1 envelope](#protocol-v1-envelope)
- [Quiescent](#halt--resume-halted) — see Halt / resume-halted
- [Readiness: build-ready / needs-grooming / not-proposed](#readiness-build-ready--needs-grooming--not-proposed)
- [Readiness reason / "waiting on #N" / "needs you" cells](#readiness-reason--waiting-on-n--needs-you-cells)
- [Rebase continue / rebase abort](#rebase-continue--rebase-abort)
- [Rebase receipt / attempt token (--attempt)](#rebase-receipt--attempt-token---attempt)
- [Rebase resolver / integration repair](#rebase-resolver--integration-repair)
- [Re-certify (evidence recertify)](#re-certify-evidence-recertify)
- [reclaim.auto / reclaim.lease_ttl](#reclaimauto--reclaimlease_ttl)
- [Reconcile / reconcile log](#reconcile--reconcile-log)
- [Related / discovered_from](#related--discovered_from)
- [Relink (change relink)](#relink-change-relink)
- [Repair sign-off (repair-needs-signoff)](#repair-sign-off-repair-needs-signoff)
- [Repository check / migrate](#repository-check--migrate)
- [request_id / replayed (idempotent replay)](#request_id--replayed-idempotent-replay)
- [Reserved type tokens all / untyped, and migrating to typed changes](#reserved-type-tokens-all--untyped-and-migrating-to-typed-changes)
- [Resolver reservation / resolver budget exhausted](#resolver-reservation--resolver-budget-exhausted)
- [Restart after regenerating wrappers](#restart-after-regenerating-wrappers)
- [Result / disposition](#result--disposition)
- [Results](#results)
- [Results **Human action:** line](#results-human-action-line)
- [Resume dispositions](#resume-dispositions)
- [Retarget children](#retarget-children)
- [Review tier](#review-tier)
- [review.min_fix_severity](#reviewmin_fix_severity)
- [Revision (--revision)](#revision---revision)
- [Run-context refusal (run-context-invalid / run-context-conflict)](#run-context-refusal-run-context-invalid--run-context-conflict)
- [Run fence](#run-fence)
- [Run tracker](#run-tracker)
- [Run verdict](#run-verdict)
- [Run verify](#run-verify)
- [Runner delegation](#runner-delegation)
- [Runner shim / runners block](#runner-shim--runners-block)
- [runtime.bash](#runtimebash)
- [Sandbox / allowlisting the binary](#sandbox--allowlisting-the-binary)
- [Schema / request file](#schema--request-file)
- [Selection order](#selection-order)
- [Selective publish](#terminal-publish) — see Terminal publish
- [Skill](#skill)
- [Spec](#spec)
- [Stacked change / effective base](#stacked-change--effective-base)
- [Start / run key / run id / run context](#start--run-key--run-id--run-context)
- [Status](#status)
- [Status vs the merged-PR sweep](#status-vs-the-merged-pr-sweep)
- [Step-0 preamble](#step-0-preamble)
- [Stub](#stub)
- [Suite command (build.test_command / finalize.test_command) / configure-tests](#suite-command-buildtest_command--finalizetest_command--configure-tests)
- [Suite gate](#suite-gate)
- [Sweep](#sweep)
- [Sync integration](#sync-integration)
- [Terminal publish](#terminal-publish)
- [Tri-state verdict / halt exit code](#tri-state-verdict--halt-exit-code)
- [Trivial](#trivial)
- [Two invocation paths: skill-invoke vs agent-dispatch](#two-invocation-paths-skill-invoke-vs-agent-dispatch)
- [Uninstall](#uninstall)
- [Verify-run](#run-verify) — see Run verify
- [Version / development install](#version--development-install)
- [Workflow role](#workflow-role)
- [Workspace publish](#workspace-publish)
- [Worktree / feature workspace](#worktree--feature-workspace)
- [Worktree changed / certified input changed](#worktree-changed--certified-input-changed)
- [Worktree slot](#worktree-slot)
- [worktree-busy / launch-unconfirmed](#worktree-busy--launch-unconfirmed)
