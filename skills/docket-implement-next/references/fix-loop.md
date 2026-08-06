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
resolve it by re-running the suite yourself.

## Tasks, batching, commits

Every fix runs the **`docket-build-task`** contract (focused test → implement → verify →
self-review → one commit), dispatched by profile name, **foreground and sequential** — fixes share
one worktree, so two concurrent workers would collide.

- **Blockers and importants: one task per finding**, one commit each, the message naming the
  finding and the reasoning. Per-finding tasks buy failure isolation and a bisectable narrative,
  and blockers are rare enough that the extra dispatches cost nothing.
- **Minors: route each finding first, then batch** those sharing a profile into one task per
  profile — in practice a single `economy` batch. The batch's tier is its members' shared tier, so
  it is homogeneous by construction. One commit enumerating the findings it fixed; a failed batch
  falls back to recording its members.

**Track every fix commit's SHA and whether its task addressed a blocker.** The suite gate below
cannot run without that record.

## The suite gate — revert and record

Run the **full suite once** after every fix task has landed, using the same command boundary
docket-build's gate uses, and refresh the build-evidence record from the result.

**Green** → proceed to Step 6.5 with the refreshed record.

**Red** → the loop must not leave the branch worse than the green build that entered it:

1. **Revert the non-blocker fix commits** by tracked SHA — the importants and minors. Blocker
   fixes stay: the run cannot proceed without them.
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
| **deferred** | below `REVIEW_MIN_FIX_SEVERITY`, or a max-character non-blocker; recorded for merge-time judgment |
| **reverted** | fixed, then rolled back by the suite gate; the finding stands |
| **recorded** | the fix was attempted and its escalation allowance was exhausted; name the failure |

## Auto-capture is narrower here

**A finding about this branch's own diff is never mintable** — it is fixed or it is recorded.
Minting from review survives only for genuinely distinct, beyond-the-branch work that independently
clears the materiality bar in
[`../../docket-convention/references/auto-capture.md`](../../docket-convention/references/auto-capture.md).
