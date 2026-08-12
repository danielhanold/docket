# Stacked changes — building on another change's unmerged branch

> The mechanics behind `stacked_on:` and the `stacked-merged` lifecycle state: how a child's base
> branch is resolved, how its PR is opened and merged, what finalize owes a parent that still has
> open children, and what happens when a parent is killed. Read on trigger — the change at hand
> carries `stacked_on:`, or has stacked children. Loaded on demand from `docket-convention/SKILL.md`;
> sibling files are not auto-loaded with the skill.

## The governing invariant

**`done` means the change's code is reachable from the integration branch.** A change merged only
into its stack parent is not `done`, however green its PR looks. Every rule below is downstream of
that one sentence: the extra state exists because a merge happened, the promotion exists because
reachability arrived later, and the finalize gate exists because deleting a parent branch early
destroys the path by which reachability was going to arrive.

## Declaring the stack — `stacked_on:`

`stacked_on: <parent id>` on the child names exactly one parent, as a single **integer scalar**
(never a flow collection, never quoted). It is the sole source of truth for the relationship: the
parent-side **Stacked children** row is derived at render time by `render-change-links.sh`, and no
`stacked_children:` field exists. The parent id is never copied into `related:` or `depends_on:`.
That row is a **human view, not an oracle**: it is regenerated when something writes the parent, so
it can lag a child added later. Anything that decides — a gate, a report, a close-out — reads
`docket.sh stack-children` instead.

The key is **optional**, so every read of it uses the anchored `fm_field`, never `field` — in this
repo a change body discussing `stacked_on:` is ordinary content, and an unanchored read of an absent
key falls through to it.

A chain must be **acyclic and complete**: every ancestor exists and no id repeats. A chain that is
neither is a data defect, reported as the `stack-invalid` health check — never worked around.

`stacked_on` is orthogonal to `depends_on`. `depends_on` gates *readiness* on a dependency reaching
`done`; `stacked_on` says *where this change's code sits*. Stacking a change on a parent does not
make it depend on that parent, and a `depends_on` entry is never satisfied by anything short of
`done` — see the next section.

## The `stacked-merged` state

`stacked-merged` is the sixth **active**, non-terminal status: the change's PR merged into its stack
parent's branch rather than into the integration branch. The change file stays in `active/`, its
feature branch is **not** deleted, and no terminal record is published — there is no terminal
transition yet to publish.

What it satisfies:

- **`verify-run`** — an implement-next run that reached it is complete; the change is not unclaimed.
- **The board and the mirror** — it renders in its own section and keeps its issue **open**.

What it does **not** satisfy:

- **`depends_on` for anything.** A dependency is satisfied at `done` and at nothing else. A change
  depending on a `stacked-merged` change is still waiting, correctly: that code has not shipped.
- **Close-out.** Nothing is archived, published, or cleaned up until the stack root lands.

The merge sweep is the only producer of the state (`docket-status.sh`, report line
`stacked-merged <id> <parent>`); `stack-closeout.sh` is the only consumer that takes a change out of
it. Neither is restated here — `scripts/docket-status.md` and `scripts/stack-closeout.md` own them.

## Resolving the base — `stack-base.sh`

Every branch cut, PR base, and rebase target for a change goes through one resolver, invoked
**unconditionally**: an unstacked change resolves to the integration branch at exit `0`, so no caller
needs to know in advance whether a change is stacked.

```
"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh stack-base \
  --changes-dir <changes_dir> --id <id> --integration-branch <integration_branch>
```

The walk applies four rules, upward from the change:

1. **A live parent whose branch is pushed is the base** — the parent's `branch:` is the answer. The
   remote ref must actually exist: `branch:` is stamped at *claim* and the branch is pushed at the
   *PR* step, so an `in-progress` parent routinely carries a valid-looking name with nothing behind
   it.
2. **A parent that already merged resolves upward** — a `done` parent, or a `stacked-merged` parent
   whose branch is gone, contributes no branch, so the answer is whatever *its* base resolves to,
   recursively, until the walk reaches an unstacked ancestor and lands on the integration branch.
3. **A `killed` parent stops the walk.**
4. **Anything else is invalid** — a missing parent, a cycle, or a parent branch with no remote ref.

| Exit | Meaning | What you must do |
|---|---|---|
| `0` | Resolved; the branch name is the only thing on stdout. | Cut from it, open the PR against it, rebase onto it. |
| `2` | Usage error. | Fix the invocation. |
| `3` | The chain reaches a **killed** parent. | Stop and surface it. This is a **scoping decision** a human makes — see *When a parent is killed*. |
| `4` | **Invalid resolution.** | Treat as a **data repair**: fix `stacked_on:`/`branch:`, or push the parent's branch. |

**Never fall back to the integration branch on `3` or `4`.** Both print nothing on stdout precisely
so a caller cannot mistake a broken stack for a fine one; a silent fallback produces a branch nobody
designed while every surface still reports it as stacked. The two codes are separate because the
remedies are, and the board reports them as the separate `stack-parent-killed` and `stack-invalid`
health checks.

A change whose base does not resolve is **not build-ready**: the board reads
*waiting on #A — stack base not built* and the digest token is `stack-base-unresolved`.

## Building a stacked child

At the branch cut, the resolved effective base replaces `origin/<integration_branch>` — and only
there. Everything else about the feature branch is unchanged: it is cut after claim and reconcile,
carries only plan + results + code, and never modifies docket metadata.

```
git worktree add .worktrees/<slug> -b feat/<slug> origin/<effective-base>
```

The PR **targets that same base**, not the integration branch. Fetch the base ref directly before
cutting, as with any feature branch.

**The child's rebase is lazy.** A child is not rebased when the parent's branch moves; it is rebased
at the child's **own next finalize gate**, which already rebases onto its base and re-runs the suite
before merging. Rebasing children eagerly on every parent push would rewrite branches with open PRs
for no gain the gate does not already deliver.

## Finalizing a parent that has open children

Before merging a change that has stacked children, resolve every child's state. **Get that child set
from the scan, never from the parent's rendered `## Stacked children` row** — that row is a view
regenerated when something writes the *parent*, so a child stacked on an already-`implemented`
parent is simply absent from it. Run this **unconditionally**, as with `stack-base`: an unstacked
change prints nothing.

```
"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh stack-children \
  --changes-dir <changes_dir> --id <this change's id> --open-only
```

Each line is `<padded id> <status> <pr-or-dash>`, parents before children. Empty stdout at exit `0`
means no open children, so this section's gate does not fire. Exit `4` means the id names no change: a
typo, never an all-clear. Drop `--open-only` for the whole graph, which is what step 3.5's close-out
gate asks for.

- **Children still open** (any status short of `stacked-merged` or `done`, with a PR whose base is
  this parent's branch):
  - **Autonomous finalize hard-blocks.** It cannot ask, and merging would strand child PRs against a
    branch about to be deleted. Abort-and-report through the channels the gate-failure reference
    owns, naming the blocking children.
  - **Interactive finalize warns and lets the human override.** State which children are open and
    what the override costs (their PRs must be retargeted now, by this run), then proceed only on an
    explicit go-ahead.
- **Retarget every open child PR explicitly, BEFORE the parent's branch is deleted** —
  `gh pr edit <child-pr> --base <the parent's own base>` — and verify each edit landed. **Docket
  never relies on GitHub's delete-time base retargeting**: it is a platform behaviour docket does not
  control, it is silent, and it does not exist at all for a branch deleted by any route other than
  the PR merge UI.
- **A parent branch with open child PRs that cannot be retargeted is RETAINED, not deleted.** Skip
  the cleanup step for it and say so in the report. A retained branch is a tidiness cost a human
  clears in a minute; a deleted one closes its children's PRs and loses their review history.

A child already at `stacked-merged` blocks nothing — its code is in the parent's branch and rides the
merge, which is exactly what the stack close-out then promotes.

## When a parent is killed

Killing a parent invalidates its descendants' base. Every descendant was written against work that
was abandoned, so **nothing is auto-rebased and nothing is auto-promoted** — re-scoping a change off
a killed parent is a design decision, not a fallback.

- **Open descendants** (`proposed`, `in-progress`, `blocked`, `implemented`) flip to `blocked` with
  `blocked_by: stack parent #A killed — re-scope, re-parent, or kill`, naming the three exits a
  human actually has.
- **`stacked-merged` descendants flip to `blocked` too**, with the same `blocked_by:`, and **their
  feature branches are preserved**. Their code merged into a branch that will never reach the
  integration branch, so promoting them would write a `done` record that the governing invariant
  says is false — and deleting their branches would destroy the only copy of work a human may well
  want to re-parent.

`stack-base.sh` exit `3` and the `stack-parent-killed` health check are how this state is surfaced;
the flip itself is a human-directed edit, never an automatic sweep.

## The stack close-out is idempotent

When a stack **root** merges, `stack-closeout.sh` promotes each `stacked-merged` descendant through
the shared terminal close-out to `done`, archived under the **root's** merge date, and regenerates
the root's marker-bounded **Stack carried** table on the root's archived record.

**Two invokers run this one op**, and neither substitutes for the other: the `docket-status` merge
sweep, right after it sweeps a root to `done`; and `docket-finalize-change`, after its archive step
and **before** the cleanup that deletes the root's branch. The sweep cannot cover for finalize — it
only ever enumerates `active/` for a merged PR, and a root finalize has archived is never
re-enumerated while a `stacked-merged` descendant has no merged PR of its own to find — so a
close-out finalize skips is a stack stranded permanently. Gate on the root actually having
descendants, so an unstacked change pays nothing: not a subprocess, not the fetch.

```
"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh stack-closeout \
  --changes-dir <changes_dir> --root-id <root id> --date <the root's UTC merge date> \
  --integration-branch <integration_branch> --metadata-branch <metadata_branch> \
  --adrs-dir <adrs_dir> --terminal-publish <terminal_publish>
```

`--date` is the root's `mergedAt` in **UTC**, never `now()`: the pass is re-run, and a clock-derived
date makes two runs disagree about the same descendant's archive filename. Key on the report lines —
`promoted`, `promote-skipped`, `promote-failed`, `stack-carried`, `stack-carried-failed` — never on
the exit code, and relay them verbatim. The posture at a failed line is **log and continue**: the
root is already `done`, so stopping buys nothing and loses the rest of the close-out. Say so loudly
and name the by-hand re-run of this same command, because once the root leaves `active/` no sweep
picks it up again.

It is safe to re-run, and re-running it is the designed recovery from a partial pass. The
idempotency key is two-part and both parts are read from durable state: the descendant is **archived
on the metadata branch** *and* carries **no outstanding `## Publish deferred` marker**. Archiving is
the sequence's first step, so an archived-but-unpublished descendant is a *resumable* run, not a
finished one — and the working tree's cleanliness is never the probe, because a half-finished run is
exactly what leaves it dirty. A per-descendant failure never abandons its siblings. The full report
vocabulary and failure reasons live in `scripts/stack-closeout.md`.
