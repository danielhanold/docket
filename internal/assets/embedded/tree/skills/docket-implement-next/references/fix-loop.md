# fix-loop — repairing review findings in-branch

The mechanics behind `docket-implement-next` Step 6's bounded fix loop. **Read this before
dispatching the first fix task.** Loaded on demand from Step 6; sibling files are not auto-loaded
with the skill.

The loop runs **after review returns and before the PR opens**, on the branch that is already
green. The human's merge gate does not move: every auto-authored fix arrives inside the diff they
were going to read anyway. Nothing here relaxes `docket-review`'s read-only contract (ADR-0066) —
the reviewer stays a reviewer, and the fixing is the implementer's.

## Two orthogonal axes

**Character picks the profile. Severity picks only the failure posture.** Keeping these apart is
the design: a `minor` finding whose fix is genuinely subtle must not be handed to a cheap model for
being minor, and a `blocker` whose fix is a one-word typo must not burn a premium dispatch for
being a blocker.

- **Character → profile.** A finding is a very small work item with the diagnosis pre-written.
  Route it with the shared rubric in
  [`../../docket-build/references/task-routing.md`](../../docket-build/references/task-routing.md)
  — the same file `docket-build` routes plan tasks with. **Never restate that rubric here or in
  your dispatch prose.**
- **Severity → posture.** What happens when the fix does not land.

One deliberate exception to this orthogonality exists — the blocker floor, below.

## The ceiling — fix tasks stop at `premium`

**No fix task dispatches the `max` profile, at any severity.** `premium` is
"consequential but correctable" — still walk-backable inside a reviewed diff. `max` is defined by
irreversibility, and an irreversible act must never happen to a branch as an unplanned side-quest
discovered at review time. The pre-0218 blocker ladder (`standard` → `premium` → halt) also
stopped short of that top tier, so the **ceiling** matches it; the **floor** it guaranteed is
preserved separately, below.

## The floor — a blocker's fix starts no lower than `standard`

This is the one deliberate exception to the character/severity orthogonality above, and the only
place severity touches the profile: a blocker's fix task starts at `standard` even when its
character routes `economy`. A blocker is the gate that must not fail open — the run halts on it —
so its fix may not start below the uncertainty sink. Without the floor, a blocker misclassified as
mechanical would run `economy` → `standard` and halt with `premium` never tried, where the
pre-0218 ladder always reached `premium` before halting. The floor restores that guarantee;
character routing at or above `standard` is untouched, and the never-`max` ceiling still binds.

The rubric therefore doubles as the size ceiling; there is no separate knob for "too big to fix
in-branch". A **max-character blocker halts** — abort-and-report, the change stays `in-progress`
with `claimed_at` refreshed and the reason recorded. A **max-character important or minor** — rare
by construction, since unresolved architecture flagged as minor essentially does not occur —
becomes a line in the PR body for the human's merge-time judgment, **not** a follow-up change.

## The routing table

| Finding character | blocker | important | minor |
|---|---|---|---|
| `economy` | fix at `standard` — the blocker floor (→ 1 escalation) | fix (→ 1 escalation) | fix, batched (→ 1 escalation) |
| `standard` | fix (→ 1 escalation) | fix (→ 1 escalation) | fix (→ 1 escalation) |
| `premium` | fix (no retry — the next rung is `max`) | fix (no retry) | fix (no retry) |
| `max` | **halt** | PR-body record | PR-body record |

Escalation is docket-build's one-bounded-escalation rule, **truncated at `premium`**: an `economy`
fix retries once at `standard`, a `standard` fix once at `premium`, and a `premium` fix does not
retry at all. The blocker floor means a blocker's ladder is always `standard` → `premium` → halt. Failure after the allowance is exhausted follows the severity posture — a blocker
halts, an important or minor becomes a PR-body record naming the failure as the reason.

## The severity threshold

`REVIEW_MIN_FIX_SEVERITY` (from the Step-0 config export; `minor` by default) is the lowest
severity that enters this loop. `important` records minors instead of fixing them; `blocker` is the
pre-0218 record-everything behavior, kept as a compat escape hatch.

**Blockers are fixed regardless of the threshold** — a run cannot proceed past an unfixed blocker,
so the knob can never disarm the one gate that must not be disarmed. A finding below the threshold
takes the PR-body record path unchanged.

The reviewer's `unverified-build-state` blocker is the one finding you never hand to a worker: you
resolve it by re-running the suite yourself, **before any fix task dispatches**. That re-run does
**not** count against the suite gate's two-run bound below — it establishes the green baseline the
loop requires rather than verifying the loop's own work, and charging it to the gate would spend the
revert path's re-run before a single fix existed. A run that hits it therefore spends **at most
three** suite runs across Step 6; the bound below is scoped to the gate and is unchanged.

## Tasks, batching, commits

Every fix runs the **`docket-build-task`** contract (focused test → implement → verify →
self-review → one commit), dispatched by profile name, **foreground and sequential** — fixes share
one worktree, so two concurrent workers would collide.

A fix worker that returns without a schema-valid outcome may still be **running**: never discard
the worktree and dispatch a fresh worker for that finding, however dead the first one looks. Halt
instead — abort-and-report, the change staying `in-progress` with `claimed_at` refreshed and the
reason recorded, the worktree left exactly as it stands. The trigger is the malformed return you
observed, never elapsed time; a blocked foreground controller has no clock.

**If profile dispatch is unavailable** — established only per the convention's
*Dispatch-capability resolution*, **never from a tool name**; an unregistered profile wrapper is
the same condition reached by a concrete rejection — the fix dispatch is **Tier C**, on the same
authorized-or-halt terms Step 5's build role carries: an explicitly configured `skills.build: auto`
authorizes running the fix inline under this same contract, and any other resolved value is
abort-and-report. That authorizer is **borrowed on purpose** — a fix worker runs the
`docket-build-task` contract at `docket-build`'s own profiles, so the build role's switch is the
honest one and no `skills.fix` knob exists. Recording every finding instead is **not** the
fallback — that fails the loop open silently, and a blocker would ride out to the PR unfixed.

- **Order: blockers first, then importants, then minors.** Non-blocker fix commits are therefore
  the tail of the branch, and the suite gate below can lift them off without unstacking a blocker
  fix that landed on the same region.
- **Blockers and importants: one task per finding**, one commit each, the message naming the
  finding and the reasoning. Per-finding tasks buy failure isolation and a bisectable narrative,
  and blockers are rare enough that the extra dispatches cost nothing.
- **Minors: route each finding first, then batch** those sharing a profile into one task per
  profile — in practice a single `economy` batch. The batch's tier is its members' shared tier, so
  it is homogeneous by construction. One commit enumerating the findings it fixed; a failed batch
  falls back to recording its members.

**The cap — at most `REVIEW_MAX_FIX_TASKS` non-blocker fix tasks per run** (from the Step-0 config
export; `10` by default). Blockers are never counted against it: the run cannot proceed past an
unfixed blocker, so a cap that counted them would disarm the gate the floor above exists to
protect. The unit is the **task**, not the finding — a minor batch spends one slot. Fill the slots
deterministically in the dispatch order above: importants in the reviewer's returned order, then
the minor batches — so two runs over the same findings fix the same set. Overflow findings take the
disposition table's `deferred` state, with the cap named as the reason. The default of 10 sits
above auto-capture's three-mints-per-run precedent because a fix commit inside an already-reviewed
diff is far cheaper than a minted change. This bounds aggregate **count**; per-finding **size**
remains the rubric ceiling above — the "no separate knob for too big" sentence is about size, and
the two rules do not overlap.

**Track every fix commit's SHA and whether its task addressed a blocker.** The suite gate below
cannot run without that record.

## The suite gate — revert and record

Run the **full suite once** after every fix task has landed, using the same command boundary
docket-build's gate uses, and refresh the build-evidence record from the result.

**Green** → proceed to Step 6.5 with the refreshed record.

**Red** → the loop must not leave the branch worse than the green build that entered it:

1. **Revert the non-blocker fix commits** by tracked SHA — the importants and minors. Blocker
   fixes stay: the run cannot proceed without them. They are the branch's tail by the dispatch
   order above, so the revert applies cleanly. **Should one conflict anyway** → **halt**: restore
   the worktree to its pre-revert state first, then abort-and-report, the change staying
   `in-progress` with `claimed_at` refreshed and the reason recorded. A half-applied revert in a
   shared worktree is the one outcome worse than the red branch — the human must arrive at a
   coherent branch to inspect.
2. **Re-run the suite once**, refreshing the build-evidence record from this run — the record must
   always reflect the *last* suite run, or a now-green branch ships the first run's `result: red`.
3. **Green** → proceed with the refreshed record. The reverted findings are recorded unfixed in the
   PR body, which is the fallback they already had.
4. **Still red** → **halt**: the blocker fixes are implicated, and there is no second repair
   chain — abort-and-report, the change stays `in-progress` with the reason recorded.

**At most two suite runs** in this phase. **No re-review round** after fixes: remediation is
carried by each worker's own self-review, the suite gate, and the human reading every fix in the
PR diff.

## Recording — the PR-body disposition table

Findings reach the PR body as a **disposition table**, so the human sees at a glance what was done
about each one:

| State | Meaning |
|---|---|
| **fixed** | repaired in-branch; cite the commit SHA |
| **deferred** | below `REVIEW_MIN_FIX_SEVERITY`, a max-character non-blocker, or fix-task cap overflow; recorded for merge-time judgment |
| **reverted** | fixed, then rolled back by the suite gate; the finding stands |
| **recorded** | the fix was attempted and its escalation allowance was exhausted; name the failure |
| **minted** | genuinely distinct, beyond-the-branch work captured as its own change; cite the stub id |

Every finding returned by the reviewer takes exactly one of these states — the table is the complete
accounting, so a finding that reached the narrow mint path below still gets a row rather than
vanishing from the human's view.

## Auto-capture is narrower here

**A finding about this branch's own diff is never mintable** — it is fixed or it is recorded.
Minting from review survives only for genuinely distinct, beyond-the-branch work that independently
clears the materiality bar in
[`../../docket-convention/references/auto-capture.md`](../../docket-convention/references/auto-capture.md).
