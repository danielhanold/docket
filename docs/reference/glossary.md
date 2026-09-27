# Glossary

Every docket term in one place: what it is, what it is for, and — where a command exists — how you
reach it on the CLI. The first sentence of each entry is the same one-clause gloss the guide and
concept pages use on first mention, so a word means the same thing on every page.

**How to read the snippets.** They are illustrations of today's command shape, not the contract.
The flags a verb accepts are owned by `docket <noun> <verb> --help`, and the machine-readable list
of every operation is owned by `docket capabilities --json` (see [`cli.md`](cli.md)). A
`--request <file>` body is built from `docket schema --operation <id>`, never guessed. Snippets
that belong to autonomous workflows (the run gate, gate drives, finalize steps) are shown so you
can recognise them in a run log; the skills normally invoke them for you.

Terms are grouped by the layer of docket they belong to. Jump to a group:

1. [Repository and branches](#repository-and-branches)
2. [Work records](#work-records)
3. [Change lifecycle and statuses](#change-lifecycle-and-statuses)
4. [Grooming](#grooming)
5. [Building a change](#building-a-change)
6. [The run gate](#the-run-gate)
7. [Supervised gate runs](#supervised-gate-runs)
8. [Review](#review)
9. [Finalize and close-out](#finalize-and-close-out)
10. [Status, health, and maintenance](#status-health-and-maintenance)
11. [Skills, agents, and harnesses](#skills-agents-and-harnesses)
12. [Configuration](#configuration)
13. [Operations and the CLI protocol](#operations-and-the-cli-protocol)

An [alphabetical index](#alphabetical-index) closes the page.

---

## Repository and branches

### Integration branch

The branch code lands on, usually `main`.

**Used for:** every feature branch is cut from it, every PR targets it (except a stacked change's),
and finalize rebases onto it before merging. Set by the `integration_branch` coordination key
(`auto` resolves to the remote's default branch).

```sh
docket diagnostic config --repo-dir . --json   # shows the resolved integration branch
```

### Metadata branch

The `docket` git branch where the backlog, specs, and decisions are stored, separate from the code.

**Used for:** keeping planning history out of your code history. The two branches never merge into
each other. Set by the `metadata_branch` coordination key.

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

### Docket-mode / single-branch mode

Docket-mode is the default layout: metadata on the metadata branch, code on the integration
branch. A **single-branch** (legacy) layout keeps the backlog on the integration branch.

**Used for:** deciding where docket reads and writes. A single-branch repo is refused with a
migration prompt rather than half-initialised.

```sh
docket repository init      # fresh repo: create the orphan docket branch
docket repository migrate   # legacy single-branch repo: move the backlog (human-typed only)
```

### Bootstrap guard

The first-run check that probes whether the `docket` branch exists and whether the planning
surface still sits on the integration branch, and then proceeds, creates the orphan branch, or
stops with a migrate prompt.

**Used for:** making sure no skill ever writes into a half-migrated repository. It surfaces as the
disposition of `repository.prepare`.

### Feature branch

The branch a single change's code is built on, minted at claim as `<type>/<slug>` (or
`<branch_prefix>/<slug>`) and recorded in the change's `branch:` field.

**Used for:** carrying the code, plan, and results of one change. It never modifies docket
metadata. Branches are named by slug, not id — read `branch:` rather than grepping for the number.

```sh
docket status --records --json   # each change's branch/pr fields
```

### Worktree / feature workspace

An isolated working copy of the repo, on its own branch. A change's build happens in a feature
worktree under `.worktrees/`.

**Used for:** building several changes without disturbing your own checkout. Keep editors out of
`.worktrees/` while a run is active — an out-of-band edit trips the run's identity check.

```sh
docket workspace inspect --id 412
docket workspace prepare --id 412 --version <version>
```

### Terminal record / terminal publish

The terminal record is a change's archived file (plus results) once it reaches `done` or `killed`.
Terminal publish was the opt-in copying of those records onto the integration branch.

**Used for:** historical browsing. `terminal_publish` is still parsed and coordination-fenced but
is deferred from Go v1 and activates nothing; the integration branch gets code, plans, and results
through PRs alone.

---

## Work records

### Change

One unit of planned work, roughly one pull request, tracked as one markdown file.

**Used for:** everything. A change carries a frontmatter **manifest** (id, status, priority, type,
dependencies, links) and a PM-altitude body (`## Why`, `## What changes`, `## Out of scope`, …).
Active changes live in `<changes_dir>/active/NNNN-<slug>.md`; terminal ones in
`archive/<date>-NNNN-<slug>.md`.

```sh
docket schema --operation change.create        # the request fields
docket change create --request new-change.json  # usually driven by docket-new-change
```

### Stub

A change captured without a design — `proposed`, no spec, not trivial. In lifecycle terms it is
**needs-brainstorm**.

**Used for:** capturing an idea quickly and designing it later with grooming.

### Manifest

The frontmatter block at the top of a change file. Its fields (`status`, `priority`, `type`,
`depends_on`, `stacked_on`, `related`, `discovered_from`, `adrs`, `spec`, `plan`, `results`,
`trivial`, `auto_groomable`, `branch`, `claimed_at`, `pr`, `blocked_by`, `reconciled`, …) are owned
by the `docket-convention` skill (see [`fields.md`](fields.md)).

**Used for:** the machine-readable state of a change. Edit it only through typed operations; a
hand edit leaves the board stale.

### Change version (`--version`)

An opaque per-change revision token — the `version` field of each change in `docket status --json`.

**Used for:** compare-and-swap. Mutating operations take `--version <id>` and refuse if the change
moved under you, so two sessions can never silently overwrite each other.

```sh
docket status --json | jq -r '.changes[] | select(.id==412) | .version'
docket change claim --id 412 --version <that-token>
```

### Spec

The design document a change links to, written before building.

**Used for:** giving the build everything it needs to implement without guessing. Stored on the
metadata branch and linked from the `spec:` field; produced by a brainstorm or by auto-groom.

### Trivial

A change marked `trivial: true` — small and mechanical enough to need no spec.

**Used for:** skipping the design step. A trivial change with its dependencies merged is
build-ready without a spec.

### Plan

The task-by-task breakdown a build follows, written on the feature branch.

**Used for:** routing each task to a build profile. The plan file lives on the feature branch; the
`plan:` field is attached on the metadata branch. A merged plan is a frozen build record — never
hand-edited afterwards.

```sh
docket change attach-plan --id 412 --version <v> --path docs/superpowers/plans/<file>.md --commit <sha>
```

### Results

The close-out record of what a build actually did — required for every implemented change,
trivial included.

**Used for:** telling the human what to check (`**Human action:**`, `## Outcome`, verification,
known follow-ups). The file lives in `<results_dir>` on the feature branch; the `results:` field is
attached on the metadata branch.

```sh
docket change attach-results --id 412 --version <v> --path docs/results/<file>.md --commit <sha>
```

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

### Board

The generated overview of every change and its state, never edited by hand.

**Used for:** reading the backlog at a glance (`BOARD.md` on the metadata branch). Every typed
mutation re-renders it inside its own commit. `board_surfaces: [inline]` turns it on; `[]` turns it
off.

```sh
docket status              # the same information, human-readable
docket repository check    # surfaces a stale or hand-edited board
```

### Derived view / generated block / backlink

A derived view is anything rendered from the change files rather than authored: the board, each
change's `## Artifacts` link block, and the `docket:backlink` block stamped at the top of every
spec, plan, results file, and PR body. Each has exactly one writer and is never hand-edited.

**Used for:** keeping links between a change and its artifacts correct in both directions.

```sh
docket artifact backlink --artifact docs/results/<file>.md --change docs/changes/active/0412-<slug>.md
```

### Presence-encoded section

A body section whose mere presence is state: `## Run halted`, `## Finalize blocked`,
`## Auto-groom blocked`, `## Publish deferred`, `## Reclaim log`.

**Used for:** making a stop verifiable in git rather than a claim in a report. The board's
"needs you" cells are driven by these sections; each has a named operation that writes and removes
it.

---

## Change lifecycle and statuses

Statuses: `proposed` · `in-progress` · `blocked` · `deferred` · `implemented` · `stacked-merged` ·
`done` · `killed` (closed vocabulary `statuses` in `docket schema`).

| Status | Meaning | Moved there by |
|---|---|---|
| `proposed` | Drafted, awaiting work | `change create`, `change revive`, `change reclaim` |
| `in-progress` | Claimed, being built | `change claim`, `change unblock` |
| `blocked` | External blocker, recorded in `blocked_by:` | `change block` |
| `deferred` | Consciously shelved, may revive | `change defer` |
| `implemented` | Built, PR open — the human merge gate | `change mark-implemented` |
| `stacked-merged` | Merged into its stack parent, awaiting the stack root | finalize close-out |
| `done` | PR merged and archived (happy terminal) | finalize close-out / sweep |
| `killed` | Abandoned as obsolete (sad terminal) | `change kill`, reconcile |

### Readiness: build-ready / needs-brainstorm / not-proposed

**Build-ready** is a proposed change that has a spec or is marked trivial and whose dependencies
are all merged. **Needs-brainstorm** is a proposed change with neither a spec nor a trivial mark; it
needs a design conversation first. Anything not `proposed` reads `not-proposed`.

**Used for:** selection. Only build-ready changes can be implemented; only needs-brainstorm changes
can be groomed. Selection order is priority → age (`created`) → lowest id.

```sh
docket status --json | jq '.changes[] | {id, readiness, readiness_reason, unmet_dependencies}'
```

### Dependency (`depends_on`) / implicitly blocked

A change id that must reach `done` before this change is build-ready. An unsatisfied dependency
makes a change *implicitly* blocked — it is skipped, not marked `blocked` — and the board shows
**waiting on #N** (with "needs your merge" when #N is already `implemented`).

**Used for:** ordering work. Use explicit `blocked` only for blockers docket cannot infer.

### Related / discovered_from

Informational cross-links. `related:` is read by reconcile; `discovered_from:` records which change
surfaced this one. Neither gates readiness.

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

### Claim / claim lease / reclaim

A **claim** is the moment a change is picked up for building; it records which branch will carry
the work and when it was taken. The **claim lease** is a timestamp on a claim (`claimed_at:`);
when it expires with no branch behind it, the change goes back to the queue. **Reclaim** is that
return trip (`in-progress → proposed`), logged in `## Reclaim log`.

**Used for:** making sure an abandoned build never holds a change forever, while a claim that
already has a branch (real work) is flagged for a human instead. Grooming never takes a claim.

```sh
docket change claim         --id 412 --version <v>
docket change refresh-claim --id 412 --version <v>   # extend the lease at a phase boundary
docket change reclaim       --id 412 --version <v>   # refuses with lease-not-expired if still live
```

Config: `reclaim.lease_ttl` (hours) and `reclaim.auto`.

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

### Halt / resume-halted

A **halt** is an autonomous run stopping because it needs a human. It writes the bare `## Run halted`
section and commits it, so the stop is visible in git. **Resume-halted** removes the marker so a
new run can pick the change back up.

```sh
docket change halt          --id 412 --version <v> --input halt.json
docket change resume-halted --id 412 --version <v>
```

### Archive

The single physical move of a change file from `active/` to `archive/<UTC-date>-NNNN-<slug>.md` on
a terminal transition (`done` or `killed`). It is idempotent.

---

## Grooming

### Groom

Taking a stub through design to build-ready. Exits: a linked spec, a trivial verdict, a kill, a
defer, a revise of an already-groomed change, or (autonomous only) an abstain.

**Used for:** the step between capturing and building. Interactive grooming is
`docket-groom-next`; the typed write underneath is `change.groom` with an `outcome` of `spec`,
`trivial`, `revise`, `abstain`, or `rearm`.

```sh
docket schema --operation change.groom
docket change groom --request groom.json
```

### Brainstorm / consultant

A **brainstorm** is the design conversation that produces a spec. In docket's own brainstorm role
the parent holds the dialogue with you, then dispatches the **consultant**
(`docket-brainstorm-consultant`) once to author the spec or return critique.

**Used for:** turning intent into a spec during `docket-new-change` or `docket-groom-next`.

### Auto-groom / auto-groomable / autonomous-eligible

**Auto-groom** grooms stubs with no human, gated by an adversarial **critic**. A stub is
**auto-groomable** when its `auto_groomable:` override is `true`, or unset and the repo's
`auto_groom` knob is `true`. It is **autonomous-eligible** when it is needs-brainstorm *and*
auto-groomable.

**Used for:** draining design work unattended. Arm a stub by committing `auto_groomable: true`
before dispatch; an uncommitted flag fails preflight.

### Abstain / re-arm

**Abstain** is auto-groom declining to design a stub it cannot safely default: it flips
`auto_groomable: false` and writes `## Auto-groom blocked`. **Re-arm** is the human supplying the
missing context and flipping it back, which removes that section in the same commit.

```sh
# groom.json: {"change_id": 412, "version": "<v>", "outcome": "rearm", ...}
docket change groom --request groom.json
```

### Critic

`docket-auto-groom-critic` — the agent that attacks an auto-groom draft and returns exactly one
verdict. It never improves the draft; if it cannot be dispatched, auto-groom abstains rather than
self-critiquing.

---

## Building a change

### Implement-next / the drainer

`docket-implement-next` — the autonomous workflow that picks the next build-ready change (or the
id you name), claims it, reconciles, plans, builds, reviews, and stops at an open PR. It never
merges.

**Used for:** draining the backlog unattended. Always dispatch it as its named agent, bracketed by
the run gate. Pass an explicit id to resume an `in-progress` change — a bare run skips it.

```sh
# inside an agent session
/docket-implement-next 412
/loop docket-implement-next 412 413 414   # drain several back-to-back
```

### Preflight

The implementation-scope sweep that runs at the start of a selection-path implement-next
(`maintenance.preflight`): it closes out merged-but-unclosed changes and returns a compact
post-sweep read. A run that names one explicit id skips it.

```sh
docket maintenance preflight --json
```

### Implementation context

The read-only bundle implement-next assembles before claiming: the selected change, its readiness,
and what it needs to proceed.

```sh
docket context implementation --id 412 --json
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

### Workflow role

One of the five pluggable steps — `brainstorm`, `plan`, `build`, `review`, `finish` — each bound
to a skill through the `skills:` config map.

**Used for:** swapping the skill a step uses without touching the workflow. The sentinel `auto`
means "no skill; the running agent does the step itself".

```yaml
# .docket.yml
skills:
  brainstorm: docket-brainstorm
  plan: superpowers:writing-plans
  build: docket-build
```

### Plan writer

`docket-plan-writer` — the agent implement-next dispatches to invoke the plan skill, commit the
plan with its backlink on the feature branch, and return `PLAN_PATH=<path>`.

### Build profile / escalation

A build profile is one of four worker tiers (economy, standard, premium, max) a plan task is
routed to by risk. Standard is the default and the "uncertainty sink". A worker that finds its
task beyond its tier returns under-capacity and the task **escalates** one tier — at most once.

**Used for:** paying premium rates only where mistakes are expensive. Each profile is its own agent
(`docket-build-economy`, `-standard`, `-premium`, `-max`) with its own model and effort pin.

### Build gate

The full test-suite run at the end of a build that must be green before review. Its command is
`build.test_command`; `build.max_attempts` (default 4) caps the initial run plus repair-and-rerun
cycles before a red suite halts for a human.

**Used for:** catching a test a single task broke without ever running. The verdict is tri-state:
pass, fail, or halt.

### Build evidence

The committed record of that gate run, read by the reviewer. It certifies an exact tested commit.

**Used for:** letting review and finalize trust a record rather than a worker's word. Adding a
commit after the evidence was recorded makes it stale (`evidence-unverified`).

```sh
docket evidence record    --id 412 --head <sha> --run <run-dir>
docket evidence verify    --head <sha> --record <evidence-file>
docket evidence recertify --id 412
```

### Budget watch / serially confirmed breach

Wall-clock lines the suite runner prints even on a green run. `BUDGET WATCH:` and
`PARALLEL-SENSITIVE:` are screening findings (parallel timings are machine-dependent);
`SERIAL CONFIRMED OVER BUDGET:` is an authoritative breach to act on. Neither fails the run.

### Mark implemented

The transition to `implemented` once the PR is open, carrying the evidence and PR reference.

```sh
docket pr publish --id 412 --head <sha> --evidence <file> --body pr-body.md
docket change mark-implemented --id 412 --version <v> --head <sha> --pr <url> --evidence <file>
```

### Run verify

A read-only check of one change's claim-to-implemented postconditions (committed plan and results,
evidence, PR, status) that reports a closed verdict.

**Used for:** trusting git, not a completion report. Run it whenever a dispatched build says it
finished.

```sh
docket run verify --id 412
```

---

## The run gate

### Run gate

The bookkeeping around a launched build run: who launched it, whether it finished, whether it may
be retried. It keeps that state durably, outside the worker's prose.

**Used for:** deciding whether to dispatch again. A completion notification is the child's claim,
never the parent's verdict.

### Arm / gate key / run epoch / dispatch context

**Arming** (`run.gate-before`) mints three values before a dispatch: the **gate key** (ties a finish
to this launch), the **run epoch** (the id of this run, threaded into cancel and drive flags), and
the **dispatch context** (a token copied into the dispatch prompt). It prints
`gate-armed <key> <epoch> <dispatch-context>`; `gate-unarmed` still allows a keyless dispatch that
can never authorise a re-dispatch.

```sh
docket run gate-before implement-next
docket run gate-before implement-next --resume 412   # resuming an in-progress change
```

### Gate verdict

The single report line `run.gate-verdict` prints after a run returns. Obey the line, never the exit
code or the child's prose.

| Line | Meaning |
|---|---|
| `gate-retry-once …` | The only line that authorises another dispatch — once, same key, for the id and unmet conjuncts it names. Granted at most `run.max_attempts - 1` times. |
| `gate-continue <key> run-waiting <id> <continuation-id> <phase>` | Non-terminal: the same attempt still owns work. Resume it with the continuation id; spends no retry. |
| `gate-done …` | Finished (e.g. `gate-done run-complete`). |
| `gate-stop …` | Stop; no re-dispatch (e.g. `gate-stop gate-unavailable takeover-ambiguous`). |
| `gate-observe …` | Observe only; no re-dispatch (e.g. `gate-observe run-incomplete`). |
| `run-halted` | A human is needed. |

```sh
docket run gate-verdict <key>
docket run gate-verdict --unattributed 412   # no key: observe-only, can never authorise a retry
```

### Attribution / unattributed read

**Attribution** is tying a finish to the exact launch that produced it, by the gate key — never by
timing or names. With no key, an **unattributed** read reports on a named change id but cannot
authorise a re-dispatch. Attribution is conservative: when unsure, the gate declines to credit.

### Continuation

A single-use id handed out with `gate-continue`, redeemed by the resumed controller so the same
attempt carries on.

```sh
docket run gate-claim <key> <continuation-id>
```

### Cancel

The explicit stop for a dispatched run — there is no automatic Stop button. It fences the run
epoch so nothing new attaches, tears down its tasks and processes, and reports `cancelled`,
`cancellation-pending` (re-run to finish), `already-cancelled`, or `refused`. It never counts as a
failure and never earns a retry.

```sh
docket run cancel --key <key> --epoch <epoch> --reason "superseded by 413"
```

### Resume dispositions

What arming with `--resume` reports when a prior run exists: `resume-active-run` (the prior epoch
may still be live — cancel it or continue it), `cancellation-pending` (finish the cancel first), or
`resume-replacement-reserved` (exactly one replacement dispatch is reserved; dispatch that one).

---

## Supervised gate runs

### Gate run / run dir

A **gate run** is a supervised local execution of a command (usually the test suite) launched
under docket's native supervisor, with a durable **run dir** holding its record.

```sh
docket gate launch  --cwd <worktree> --root <run-root> -- ./run-tests.sh
docket gate observe <run-dir>
docket gate stop    <run-dir> --reason "wrong branch"
docket gate recover --root <run-root>
docket gate cleanup <run-dir>
```

### Gate drive / slice / owner generation / handoff / takeover

A **gate drive** runs a gate in resumable **slices** so no agent has to block for the whole suite.
Each drive has an **owner generation**; ownership moves by **handoff** (a single-use token the next
owner **claims**) or, when a child returned without handing off, by **takeover**. A **scope** binds
a drive to one parent/child dispatch boundary; the final PASSED/FAILED result is
**acknowledged** to close it.

**Used for:** the build and finalize suite gates. A forked worker drives the suite with inline,
blocking `advance` calls — it must never background the suite and yield.

```sh
docket gate drive start   --repo-dir . --owner build --run-root <dir> --run-epoch <epoch> -- <suite argv>
docket gate drive advance --drive-id <id> --owner-gen <gen>
docket gate drive handoff --drive-id <id> --owner-gen <gen>
docket gate drive claim   --drive-id <id> --handoff-id <token>
```

### Admission slot

The per-worktree slot that lets only one gate execution run at a time. A busy-slot refusal means
the slot is **occupied** by a run admission could not prove finished — not necessarily a live
process. Inspect with `gate observe`, settle with `gate stop`; history cleanup and `gate recover`
do not free it.

```sh
docket gate history cleanup --repo-dir . --dry-run   # historical drives only — not slot evidence
```

---

## Review

### Review rung

One of three pinned reviewer agents — `docket-review-lean`, `docket-review-standard`,
`docket-review-deep` — all running the same read-only whole-branch contract. The rung is chosen
deterministically one step above the build: economy → lean, standard → standard, premium/max →
deep, bumped one step for a diff over 1500 changed lines.

**Used for:** a whole-branch review before the PR opens. Reviewers never fix, dispatch, or run the
suite.

### Finding severity: blocker / important / minor

The tiers a reviewer assigns. They decide which findings the fix loop routes and at what profile.

### Fix loop

The bounded in-branch repair that runs after review and before the PR opens: findings are routed
to build profiles as fix tasks (`review.max_fix_tasks`, default 10), then one full-suite run
confirms. Anything left unfixed becomes a line in the PR body.

---

## Finalize and close-out

### Finalize

The close-out sequence: rebase onto the integration branch, retest, merge, archive. Each step gates
the next; a failed step stops the rest.

**Used for:** landing an approved or merged PR and clearing it off the backlog. Run it as the
`docket-finalize-change` agent, with or without an id (without, it auto-detects eligible changes).

```sh
docket context finalize --id 412 --json   # what finalize would act on
docket finalize rebase  --id 412 --version <v> --head <sha>
docket finalize merge   --id 412 --version <v> --head <sha>
docket finalize closeout --id 412
docket finalize cleanup  --id 412
```

### Finalize gate

How finalize validates the rebased branch before merging: `finalize.gate` is `local` (run the
suite here, default), `ci` (poll GitHub checks), `both`, or `off` (trust the PR's CI).

### Rebase resolver / integration repair

The two specialised finalize workers, split at the rebase-completion boundary.
`docket-rebase-resolver` reconciles each conflicted hunk by intent
(`finalize.resolver_max_attempts`, default 10). `docket-integration-repair` makes a red rebased
suite green with a minimal fix, never weakening a test (`finalize.repair_max_attempts`, default 6).
Neither may be substituted inline.

### Finalize blocked / reason token / clear-block

When a gate failure needs a human, finalize writes `## Finalize blocked` with a typed **reason
token** (e.g. a mismatched PR head). Auto-detect runs skip a blocked change; naming its id
overrides that. A human clears the block explicitly. The token vocabulary and remedies live in the
finalize skill's `references/gate-failure.md`.

```sh
docket finalize clear-block --id 412 --version <v> --head <sha> --pr-number 301
```

### Merge policy / branch protection

Whether a merge needs a human approval is settled before finalize runs. The single-maintainer path
is branch protection that requires a PR but zero approvals.

### Closeout / closeout notes

**Closeout** is the terminal transition that archives the change (`done-archived`,
`stacked-merged`, or `root-archived` for a stack root). **Closeout notes** is the optional final
body section it writes (`### Verification`, `### Late findings`).

### Retarget children

When a stack parent lands, finalize repoints its stacked children's PRs at the new base.

```sh
docket finalize retarget-children --id 412 --version <v> --input children.json
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

### Status

The read-only report of backlog state, readiness, selection, and repository health. It never writes
— not even the board.

```sh
docket status
docket status --priority high --type fix
docket status --records --json
```

### Health check / health code

A health check is a status-time scan for things a human should look at: stale claims, broken
links, stalled dependencies. Each result carries a **finding code** (for example `artifact-missing`,
`deferred-setting`, `publish-deferred`) and a remedy.

```sh
docket status --json | jq '.findings[] | {code, message, remedy}'
docket repository check
```

### Sweep

The pass that observes merged PRs and closes their changes out to `done`. `maintenance.sweep`
runs the full scope; preflight runs the implementation scope.

```sh
docket maintenance sweep
docket maintenance sweep --scope implementation
```

### Repository check / migrate

`repository check` is the read-only topology and consistency check (including board drift).
`repository migrate` is the human-typed repair and migration path, never run by an agent.

```sh
docket repository check
docket repository migrate --repair-frontmatter
```

---

## Skills, agents, and harnesses

### Skill

A named, reusable instruction set an agent loads for one job (a `skills/<name>/SKILL.md`).

**Used for:** holding a workflow's instructions once, independent of the tool that runs them. The
shared contract every docket skill loads first is `docket-convention`.

### Agent / wrapper

An agent is a separately launched worker with its own context, pinned to a model and effort. A
**wrapper** is the thin generated file that names the agent, pins its model and effort, and points
at the skill it loads. Wrappers are machine-local — regenerated per machine, never committed.

```sh
docket install                       # (re)generate skills, agents, and dispatch material
docket install --harness cursor
docket install check
```

### Harness

The tool that runs the agent: Claude Code, Cursor, Codex, or opencode.

**Used for:** targeting generated wrappers (`agent_harnesses:`) and picking per-harness model
defaults from the shipped sidecar `agents/harness-defaults.yml`.

### Dispatch

Launching a named agent to do a step and waiting for it to return. Foreground dispatch means the
parent actively blocks — it never backgrounds a child and yields.

**Used for:** every workflow step that runs in its own agent. When a workflow has a registered
same-name `docket-*` agent, dispatch it rather than running the workflow inline.

### Dispatch tiers (A / B / C) and the carve-out

What a workflow does when dispatch is genuinely unavailable. **Tier A** (status, ADR): run inline as
a first-class equivalent. **Tier B** (the critic): abstain. **Tier C** (plan writer, build, review):
proceed inline only if the role is explicitly `auto`, otherwise halt. The finalize resolver and
repair agents are a **carve-out**: abort-and-report, never inline.

### Fork / forked skill

On harnesses that support it, a skill can run as a fork of the current context rather than a fresh
agent. A fork's `completed` report is not proof it finished — verify git state.

### Agent enter

The Codex entry point that launches a registered docket role as a foreground root thread with the
right cwd, sandbox, approval policy, and (for feature children) the owning workflow's worktree.
Other harnesses dispatch named agents natively instead.

```sh
docket agent enter --role <role> --request req.md --cwd "$PWD" \
  --approval-policy <policy> --sandbox <mode> --run-epoch <epoch> --run-gate-key <key>
```

### Runner / delegation

**Runner delegation** hands an agent's whole run to a different harness, chosen by an explicit
`runner:` key on that agent — never inferred from a model id.

```yaml
agents:
  claude:            # the parent harness hosting the session
    build-economy: { runner: opencode, model: openrouter/<model>, effort: medium }
```

### Abort-and-report

The rule every autonomous wrapper carries: an unmet precondition or blocking ambiguity is surfaced
and stopped on, never turned into an interactive prompt.

---

## Configuration

### Config layers

Four layers resolved per key, lowest to highest: shipped defaults → global
`~/.config/docket/config.yml` → committed `.docket.yml` → gitignored `.docket.local.yml`. Nested
blocks merge leaf by leaf. Full shape and defaults: `.docket.example.yml` (see
[`config-keys.md`](config-keys.md)).

```sh
docket diagnostic config --repo-dir . --json
```

### Coordination key / scope tag / fence

A coordination key is a config key whose value must be identical for every clone, so it may only be
set in the committed repo config. The **fence** ignores (with a warning) a coordination key set in
any other layer. Each key's **scope tag** in the example file is `repo-only`, `any layer`, or
`local-only`.

### Change types

The allowed `type:` values (`change_types`, default `chore, docs, feat, fix, refactor, perf`). The
type also becomes the feature-branch prefix.

### Priority

`critical` > `high` > `medium` (default) > `low` — the first key of selection order.

### Dummy mode / persona / "In plain terms"

`dummy_mode` calibrates human-facing prose to a described reader (the **persona**). Dialogue and
reports are rewritten; results, change sections, and PR bodies get an additive
`### In plain terms` block. Agents never read that block as a decision input.

### Auto-capture / discovered work

Work an autonomous run discovers mid-run is reported in its final report, never silently minted.
`auto_capture` is parsed but deferred from Go v1 — capture deliberately with `docket change create`.

### Inert / deferred setting

A config key that is parsed but activates nothing in the current binary (for example
`terminal_publish`, `auto_capture`). Status surfaces them as `inert-setting` / `deferred-setting`
findings.

---

## Operations and the CLI protocol

### Operation / operation id

A single `docket` capability with a stable dotted id (`change.claim`, `run.gate-verdict`,
`finalize.merge`). Skills resolve the command for an id from the catalog rather than hard-coding it.

### Capability catalog

The machine-readable list of every operation the `docket` binary offers, which skills read instead
of hard-coding commands. Each entry carries its `argv`, signature, and **effects**.

```sh
docket capabilities --json | jq -r '.commands[] | "\(.id)\t\(.argv|join(" "))"'
```

### Effects

The closed set describing what an operation may touch: `read`, `local-write`, `metadata-write`,
`external-write` (GitHub, pushes), `process-control`. A workflow stops if an operation's effects
exceed what it is authorised to do.

### Schema / request file

`docket schema` emits every operation's request and result fields plus the closed vocabularies.
A **request file** (`--request` / `--input`) is a JSON body built from that schema.

```sh
docket schema --operation change.kill
docket schema --json | jq '.vocabularies | keys'
```

### Protocol-v1 envelope

The JSON shape every `--json` output shares: `protocol_version`, `operation`, `result`, then
operation-specific fields and `findings`.

### Result / disposition

The **result** is the envelope's top-level outcome (`applied`, `no-op`, `contended`,
`invalid-input`, `invalid-state`, `blocked`, `gate-failed`, …). A **disposition** is the one-word
outcome an operation reports: applied, no-op, refused, or error — plus operation-specific closed
sets (`claim_dispositions`, `merge_dispositions`, `sync_dispositions`, …) listed by `docket schema`.

### Contended

The outcome when a compare-and-swap lost a race with another writer (another session or loop). It
is not a failure of your input: re-read and retry.

### Finding / finding code / remedy

A structured diagnostic attached to a result: a `code`, a `severity`, the entity it concerns, a
`message`, and a `remedy` naming the next command. The full code list is the `finding_codes`
vocabulary.

### Version / development install

`docket version` reports the binary's build identity (full commit). `development install` rebuilds
and installs docket from a source checkout.

```sh
docket version
docket development install --source ~/dev/docket
```

---

## Alphabetical index

[Abort-and-report](#abort-and-report) ·
[Abstain / re-arm](#abstain--re-arm) ·
[Admission slot](#admission-slot) ·
[ADR](#adr) ·
[Agent / wrapper](#agent--wrapper) ·
[Agent enter](#agent-enter) ·
[Archive](#archive) ·
[Arm / gate key / run epoch / dispatch context](#arm--gate-key--run-epoch--dispatch-context) ·
[Attribution](#attribution--unattributed-read) ·
[Auto-capture](#auto-capture--discovered-work) ·
[Auto-groom](#auto-groom--auto-groomable--autonomous-eligible) ·
[Block / unblock, defer / revive, kill](#block--unblock-defer--revive-kill) ·
[Board](#board) ·
[Bootstrap guard](#bootstrap-guard) ·
[Brainstorm / consultant](#brainstorm--consultant) ·
[Budget watch](#budget-watch--serially-confirmed-breach) ·
[Build evidence](#build-evidence) ·
[Build gate](#build-gate) ·
[Build profile / escalation](#build-profile--escalation) ·
[Build-ready](#readiness-build-ready--needs-brainstorm--not-proposed) ·
[Cancel](#cancel) ·
[Capability catalog](#capability-catalog) ·
[Change](#change) ·
[Change types](#change-types) ·
[Change version](#change-version---version) ·
[Claim / claim lease / reclaim](#claim--claim-lease--reclaim) ·
[Closeout](#closeout--closeout-notes) ·
[Config layers](#config-layers) ·
[Contended](#contended) ·
[Continuation](#continuation) ·
[Coordination key](#coordination-key--scope-tag--fence) ·
[Critic](#critic) ·
[Dependency](#dependency-depends_on--implicitly-blocked) ·
[Derived view / backlink](#derived-view--generated-block--backlink) ·
[Dispatch](#dispatch) ·
[Dispatch tiers](#dispatch-tiers-a--b--c-and-the-carve-out) ·
[Disposition](#result--disposition) ·
[Docket-mode](#docket-mode--single-branch-mode) ·
[Dummy mode](#dummy-mode--persona--in-plain-terms) ·
[Effects](#effects) ·
[Feature branch](#feature-branch) ·
[Finalize](#finalize) ·
[Finalize blocked](#finalize-blocked--reason-token--clear-block) ·
[Finalize gate](#finalize-gate) ·
[Finding](#finding--finding-code--remedy) ·
[Fix loop](#fix-loop) ·
[Fork](#fork--forked-skill) ·
[Gate drive](#gate-drive--slice--owner-generation--handoff--takeover) ·
[Gate run / run dir](#gate-run--run-dir) ·
[Gate verdict](#gate-verdict) ·
[Groom](#groom) ·
[Halt / resume-halted](#halt--resume-halted) ·
[Harness](#harness) ·
[Health check](#health-check--health-code) ·
[Implement-next](#implement-next--the-drainer) ·
[Implementation context](#implementation-context) ·
[Inert / deferred setting](#inert--deferred-setting) ·
[Integration branch](#integration-branch) ·
[Learnings](#learnings--finding--promotion) ·
[Manifest](#manifest) ·
[Mark implemented](#mark-implemented) ·
[Merge policy](#merge-policy--branch-protection) ·
[Metadata branch](#metadata-branch) ·
[Metadata worktree](#metadata-worktree) ·
[Operation](#operation--operation-id) ·
[Plan](#plan) ·
[Plan writer](#plan-writer) ·
[Preflight](#preflight) ·
[Presence-encoded section](#presence-encoded-section) ·
[Priority](#priority) ·
[Protocol-v1 envelope](#protocol-v1-envelope) ·
[Rebase resolver / integration repair](#rebase-resolver--integration-repair) ·
[Reconcile](#reconcile--reconcile-log) ·
[Related / discovered_from](#related--discovered_from) ·
[Repository check / migrate](#repository-check--migrate) ·
[Result](#result--disposition) ·
[Results](#results) ·
[Resume dispositions](#resume-dispositions) ·
[Retarget children](#retarget-children) ·
[Review rung](#review-rung) ·
[Run gate](#run-gate) ·
[Run verify](#run-verify) ·
[Runner / delegation](#runner--delegation) ·
[Schema / request file](#schema--request-file) ·
[Severity](#finding-severity-blocker--important--minor) ·
[Skill](#skill) ·
[Spec](#spec) ·
[Stacked change](#stacked-change--effective-base) ·
[Status](#status) ·
[Stub](#stub) ·
[Sweep](#sweep) ·
[Sync integration](#sync-integration) ·
[Terminal record](#terminal-record--terminal-publish) ·
[Trivial](#trivial) ·
[Version / development install](#version--development-install) ·
[Workflow role](#workflow-role) ·
[Worktree](#worktree--feature-workspace)
