<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0113 — A suppressed hand-off can silently end an autonomous run — make step completion verifiable, not narrated](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0113-suppressed-handoff-silently-ends-autonomous-run.md)**
<!-- docket:backlink:end -->

# Verifiable step completion — the `aborted-run` check

Design doc for change #0113. Written 2026-08-02.

## Problem

Twice, a forked `docket-implement-next` run has narrated success it had not achieved.

- **2026-07-20, change 0109.** Ran Steps 0–4, ended its turn at the Step 4/5 boundary. On disk: the
  feature worktree identical to `main` (zero build commits), the plan file untracked, `plan:` never
  written, `status: in-progress`, `pr:` empty.
- **2026-08-02, change 0194.** Byte-comparable signature: zero build commits, plan untracked, `plan:`
  never written, `status: in-progress`, `pr:` empty. Closing line: "stopping here as directed (no
  execution, no execution-option question)" — asserting a caller instruction that does not exist.
- **Same 0194 run, resumed.** Did the real work (four commits, 76/76, PR #153 open and mergeable),
  then stopped short again at the *end* of Step 7: `origin/docket` still carried `status:
  in-progress`, `pr:` empty, and `results:` empty despite the results file being committed on the
  branch. Closing line again: "stopping here as directed."

ADR-0044's remedy is not what failed. The `superpowers:writing-plans` hand-off never reached the
human; suppression worked exactly as designed. What failed is that the agent treated *emitting the
suppression-log line* as *discharging the step*. The suppression log is being emitted in place of
the work.

The third stop is what makes this a class rather than an instance. The plan→build seam is not
uniquely exposed. The shape recurs wherever a step's real work and its bookkeeping are separable:
complete the visible artifact, narrate success, drop the metadata write. An invariant scoped to
"Step 4 isn't done until `plan:` is written" would have caught the first stop and sailed past the
third.

The human caught all three by reading git state. That is the detection this change mechanizes.

## Design decisions

### The oracle is external

The agent that dropped the bookkeeping write is the least reliable narrator of whether it dropped
it. Both incidents produced confident, specific, wrong reports — so a completion check keyed on
hedging or uncertainty in the report catches nothing. This is the `capability-absence-needs-a-failed-attempt`
learning applied one step over: an agent's own report about its own completion is untrusted input.

Detection therefore lives in a deterministic script whose exit-code behavior can be mutation-tested,
not in a self-check the failing faculty is asked to perform on itself. This is the ADR-0012
split — the model judges *what*, the script does the mechanical check.

Rejected: a self-check in the skill body before returning (asks the failed faculty to audit itself);
logic in the generated wrapper (wrappers are thin generated files); a script the skill is DIRECTED to
call (an agent that can skip the step can skip the check).

### It rides the existing health-check family

`aborted-run` becomes a new check-id in `board-checks.sh`, joining the fourteen already declared in
`BOARD_CHECK_IDS`. That surface already reads change frontmatter, already has a check-id registry,
already renders findings into the `docket-status` report, and already has fixture-plus-mutation test
conventions — change 0191 shipped `scalar-form` through exactly this path last week.

Detection consequently rides every `docket-status` run and every Board pass, including the one at
the top of the *next* `docket-implement-next`. A `/loop` drain trips over the finding on its
following iteration rather than banking a false success indefinitely.

Findings surface in the **`docket-status` health report on stdout**, not as `BOARD.md` cells. The
board's needs-you cells are driven by change-file markers (`## Auto-groom blocked`, `## Finalize
blocked`); `board-checks.sh` is a pure reader that writes nothing. No change to that posture.

Rejected: a standalone `verify-run-completion.sh` invoked by the run driver — in the failing
scenario the driver is the `/loop` that just banked the false success.

### Advisory only — never self-heal

A firing check surfaces a finding with a specific remedy line. It flips no status, releases no
claim, and touches no file.

The 0109 run left a real written plan. A naive claim release would have stranded it. More
generally, `board-checks.sh` is a pure reader by contract; making it a writer changes what the whole
check family is, and "the remedy here is unambiguous" is precisely the judgment that failed in the
incidents this change exists to catch.

Rejected: auto-writing the missing field for the pure-bookkeeping subset; excluding flagged changes
from `docket-implement-next` selection (couples the health surface into the selector, and creates a
new failure mode if the check itself is ever wrong).

## The predicate

Fires only on changes with `status: in-progress`. Two independent legs; either emits.

### Leg A — manifest/git incoherence (time-free)

The feature branch carries an artifact file that the manifest field does not record. Two pairs, same
shape:

| Condition | Emit |
|---|---|
| a file under `docs/superpowers/plans/` exists on `branch:` but not on `INTEGRATION_BRANCH`, and `plan:` is empty | plan committed but `plan:` unset |
| a file under `<results_dir>/` exists on `branch:` but not on `INTEGRATION_BRANCH`, and `results:` is empty | results committed but `results:` unset |

This is the **exact inverse of the existing `broken-plan-results` check**, which catches *field set,
file missing*. Leg A catches *file present, field empty*. Same two fields, same two trees, opposite
direction — together they close a square that was previously half-open, and the implementation is a
near-mirror of code already in `board-checks.sh`.

Leg A is time-free and has no false-positive window beyond the seconds between an artifact commit
and its field write — and since the finding is advisory and self-clearing, that race costs nothing.

**Two candidate pairs were cut during design:**

- *"PR open but `pr:` empty."* Detecting an open PR requires a network probe, which
  `board-checks.sh` forbids by contract (`stale-finalize-blocked` states this explicitly). Git-only
  means git-only. The cost is that the third stop's most direct signature is not detected as such —
  but that run also had a committed results file with `results:` empty, which leg A does see, and
  leg B backstops it regardless.
- *"Build commits present while `in-progress`."* That is what a healthy in-flight build looks like.
  Not incoherent.

### Leg B — run-scale stale claim (time-based)

`claimed_at` older than **12 hours** ⇒ emit. This catches the abort that leaves nothing in git —
the originating 0109 instance and the first 0194 stop, where the plan was written but never
committed.

**Separate check-id, not a widened `stale-in-progress`.** The existing check already keys on
`claimed_at` + `RECLAIM_LEASE_TTL` (72h) and on a 3-day branch-idle threshold. Those are
human-scale abandonment horizons with a distinct remedy ("this looks abandoned, reclaim it") and a
machine contract `docket-status` keys on — the trailing `[reclaimable]` marker. `aborted-run`'s
remedy is different ("this run stopped mid-step, go look"), and widening the existing predicate
would change what an already-written consumer sees. This is the
`shared-resource-keeps-first-owner-assumptions` learning: a single-owner predicate gaining a second
owner stays valid-looking and becomes wrong.

**The window is hardcoded, not configurable.** House precedent: `stale-in-progress` hardcodes its
3-day branch-idle threshold while only the lease TTL is a knob. Hardcoding also dodges the
`config-knob-ship-end-to-end` tax — sample config, README, and relaxed prose — for a value this repo
would set once and never revisit.

**Why 12h and not tighter.** The heartbeat is coarser than it looks. `docket-implement-next`
SKILL.md:135 re-stamps `claimed_at` at only two later boundaries — reconcile and `implemented` — so
the whole plan → build → review span currently carries no stamp. This change densifies it (below),
which adds a stamp at the plan→build seam, but the build span itself stays irreducible: no metadata
is written between the `plan:` write and `implemented`. 12h is six times tighter than the 72h lease
while leaving room for a marathon build; and when a genuinely long build does trip it, the finding
is free, self-clearing, and arguably worth a glance anyway.

## Riders

### Densify the claim heartbeat

`docket-implement-next` SKILL.md:135 currently reads "Each LATER phase-boundary metadata commit
(reconcile, `implemented`) also RE-STAMPS `claimed_at`". Change it so **every** metadata commit the
skill makes re-stamps. The commits are already happening; this costs one sentence and puts a
heartbeat at the `plan:` write — exactly the seam where the first stop occurs. Directly tightens
leg B's effective detection latency.

### Split the §5 sentence (lever 1)

In §5 the obligation to proceed and the obligation to stay silent are one sentence, and an agent can
satisfy the first clause while dropping the second. Both incidents misread exactly this sentence.
Split them into two, so "proceed" and "don't ask" are separately stated obligations rather than
clauses of one.

This is belt to the oracle's braces: prose alone was already shown insufficient at run 40 (ADR-0044's
own motivating incident), which is why the external check is the load-bearing half of this change.

### Dated `## Update` note on ADR-0044

ADR-0044's decision stands — pre-specification at the call site works, and the hand-off genuinely
never reached the human in any of these runs. But "the remedy's suppression-log line got emitted in
place of the work" is context worth recording against the decision. Appended as a dated `## Update`,
never an edit to the decision (an Accepted ADR is immutable except its `status:` line).

Ships atomically by listing `44` in this change's `adrs:` — already set — per the
`adr-update-delivery` learning; never a standalone push.

## Testing

Every predicate gets a fixture that trips it **and** a mutation test that breaks the predicate and
confirms the suite goes red. Three predicates: the two leg-A pairs and leg B.

This is not optional rigor. Change 0107's build hit the adjacent "guard reports ok while asserting
nothing" vacuity trap, and mutation testing was what settled it. A completion check that can never
fail is the defect this change exists to fix, wearing a badge.

Negative fixtures matter as much: a healthy in-flight build (recent `claimed_at`, no committed
artifacts) must produce no finding, and a change with `plan:` correctly set alongside its committed
plan must produce no finding.

## Ripple list

Verified against the `scalar-form` check-id shipped by change 0191.

| Surface | Change |
|---|---|
| `scripts/lib/docket-frontmatter.sh` | add `aborted-run` to `BOARD_CHECK_IDS` (alphabetically first) |
| `scripts/board-checks.sh` | the two legs; update the check-id list in the header comment |
| `scripts/board-checks.md` | contract entry for the new check |
| `scripts/docket-status.md` | report vocabulary |
| `tests/test_board_checks.sh` | fixtures, negative fixtures, mutation tests |
| `skills/docket-implement-next/SKILL.md` | §5 sentence split; line-135 heartbeat densification |
| `docs/adrs/0044-*.md` | dated `## Update` note |

## Out of scope

- Reversing or weakening ADR-0044. Pre-specification at the call site works and stays.
- Re-litigating `context: fork` (ADR-0024) or the wrapper mechanism.
- Any change to `superpowers:writing-plans` — vendored; change 0096 settled that docket adapts at
  its own call sites rather than patching the plugin.
- Making `board-checks.sh` a writer, or giving it network access.
