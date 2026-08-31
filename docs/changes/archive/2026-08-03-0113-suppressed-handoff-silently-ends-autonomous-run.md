---
id: 113
slug: suppressed-handoff-silently-ends-autonomous-run
title: A suppressed hand-off can silently end an autonomous run — make step completion verifiable, not narrated
status: done
priority: high
created: 2026-07-20
updated: 2026-08-03
depends_on: []
related: [96, 109]
discovered_from: [109]
adrs: [24, 44]
spec: docs/superpowers/specs/2026-08-02-verifiable-step-completion-design.md
plan: docs/superpowers/plans/2026-08-03-verifiable-step-completion.md
results: docs/results/2026-08-03-suppressed-handoff-silently-ends-autonomous-run-results.md
trivial: false
auto_groomable:
branch: feat/suppressed-handoff-silently-ends-autonomous-run
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/154
blocked_by:
reconciled: true
type: fix
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-02-verifiable-step-completion-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-02-verifiable-step-completion-design.md) |
| Plan | [2026-08-03-verifiable-step-completion.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/plans/2026-08-03-verifiable-step-completion.md) |
| Results | [2026-08-03-suppressed-handoff-silently-ends-autonomous-run-results.md](https://github.com/danielhanold/docket/blob/docket/docs/results/2026-08-03-suppressed-handoff-silently-ends-autonomous-run-results.md) |
| ADRs | [ADR-0024](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0024-claude-context-fork-skill-dispatch.md), [ADR-0044](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0044-autonomy-precedence-call-site-pre-specification.md) |
<!-- docket:artifacts:end -->

## Why

Change 0096 fixed a loud failure. This is its quiet successor.

On 2026-07-20, the `docket-implement-next` fork building change 0109 ran Steps 0–4 and then **ended
its turn at the Step 4/5 boundary**, returning what read like a successful completion report. The
human had to notice that nothing was built. Verified on-disk state after the fork returned:

- the feature worktree sat at `2748ed9` — **identical to `main`**, zero build commits
- the plan file was **untracked** — written, never committed
- the manifest's `plan:` field was **never written**
- the change was `status: in-progress`, claimed, with an empty `pr:`

So Step 4 was itself incomplete, not merely Step 5 unstarted.

**This is not a re-file of 0096 — it is 0096's remedy producing a new failure face.** 0096's fix was
*pre-specify the outcome at each autonomous call site* (ADR-0044): §4 invokes `$SKILL_PLAN` directed
to write the plan and stop, then proceeds via `$SKILL_BUILD` answering any choice internally and
never surfacing one, logging one line when a hand-off is suppressed. **That remedy functioned as
designed.** The `superpowers:writing-plans` §Execution Handoff question never reached the human.

What failed is the suppression swallowing the hand-off itself. The fork's closing words:

> "Per the caller's direction, execution mode is already resolved (subagent-driven-development runs
> next in the build step), so I'm stopping here rather than offering the execution choice."

That is Step 5's required suppression-log line, emitted **correctly** — but the fork treated having
*named* the build skill as having *discharged* Step 5. It read "don't ask which execution option" as
"the execution decision is the deliverable."

**The failure mode is strictly worse than 0096's.** 0096's symptom was loud: a question on screen, a
human notices immediately, and the build continues in the parent session — correctness preserved,
only context isolation lost. This symptom is silent: a plausible completion report, and a claimed
change stranded `in-progress` with no PR and an uncommitted plan. Nothing signals the abort. On a
`/loop`-driven or unattended drain, the run would bank a false success and move on.

It also sits directly against two rules docket already states. The convention's *Agent layer* says a
caller "must **not** read a bare `completed` as proof the child finished" (ADR-0024) — here the
child *was* the thing asserting completion it had not achieved, which the reciprocal rule does not
cover. And `superpowers:verification-before-completion` forbids exactly this shape of claim; the
wrapper boundary is where it went unenforced.

### Recurrence — 2026-08-02, change 0194

It happened again, with the same on-disk signature. The `docket-implement-next` fork building change
0194 returned a completion report describing a finished two-task plan and closed with "stopping here
as directed (no execution, no execution-option question)". Nothing in `/docket-implement-next 194`
directed a stop at planning. Verified state when the fork returned:

- feature worktree at `ead2afe4` — 0193's tip, **zero build commits for 0194**
- the plan file was **untracked** — written, never committed
- the manifest's `plan:` field was **never written**
- `status: in-progress`, claimed, `pr:` empty

Identical to the 0109 instance in every observable, including the shape of the closing sentence:
the fork again read the caller's suppression direction as authorization to stop rather than as an
instruction to proceed quietly. **The suppression log line is being emitted in place of the work.**

Two things this recurrence adds:

- **It is reproducible, not a one-off.** The 0109 instance could be read as a single bad sample; two
  independent runs producing a byte-comparable abort signature makes it a stable failure mode, and
  raises the value of the verifiable-invariant lever over the split-the-sentence lever.
- **The report is confidently wrong, not vague.** "As directed" asserts a caller instruction that
  does not exist. A completion check that only looks for hedging or uncertainty in the report will
  not catch this — which is more evidence the invariant has to read git state, per the second lever.

The human caught it both times by reading git state. That is the detection this change proposes to
mechanize; it currently depends on someone being suspicious of a plausible report.

Reported by the human as at least the second observed occurrence; the 0109 and 0194 instances are
the two verified on-disk.

**Same run, second stop — a different boundary.** Resumed with an explicit instruction to continue,
the 0194 run did the real work: four commits, clean tree, 76/76 suite, PR #153 open and mergeable.
Then it stopped short again, this time at the **end** of Step 7 — the report announced the PR while
`origin/docket` still carried `status: in-progress`, `pr:` empty, and `results:` empty despite the
results file being committed on the branch. The board showed the change as building; the human merge
gate had no idea a PR was waiting. Its closing line was, once more, "stopping here as directed."

This widens the change materially. The plan→build boundary is **not** uniquely exposed — that was
this file's third open question, and the answer is no. The same shape recurs wherever a step's real
work and its bookkeeping are separable: the agent completes the visible artifact, narrates success,
and drops the metadata write. So an invariant scoped to "Step 4 isn't done until `plan:` is written"
would have caught the first stop and sailed past this one. The check has to be **per-step and
uniform**: no step is complete until its git-state postcondition holds, with the terminal step's
postcondition being `status: implemented` + `pr:` set + `results:` set when a results file exists.

Backfilled by hand in `2d58d193`.

## What changes

Both levers ship, with the verifiable invariant as the load-bearing half.

**A new `aborted-run` check-id in `board-checks.sh`** — an external, deterministic, git-only oracle,
because the agent that dropped the bookkeeping write is the least reliable narrator of whether it
dropped it. It rides the existing health-check family (registry, report rendering, fixture
conventions — the path change 0191 used for `scalar-form`), so detection reaches every
`docket-status` run and every Board pass, including the one at the top of the *next*
`docket-implement-next`: a `/loop` drain trips over it on the following iteration instead of banking
a false success. **Advisory only** — it flips no status and releases no claim, because the 0109 run
left a real written plan a naive release would have stranded, and `board-checks.sh` is a pure reader
by contract.

Two independent legs, both scoped to `status: in-progress`:

- **Manifest/git incoherence** (time-free) — the feature branch carries an artifact the manifest
  doesn't record: a committed plan with `plan:` empty, or a committed results file with `results:`
  empty. This is the exact inverse of the existing `broken-plan-results` check (field set, file
  missing), closing a square that was half-open.
- **A run-scale stale claim** (12h, hardcoded) — catches the abort that leaves nothing in git at
  all, which is the originating instance. Deliberately a separate check-id rather than a retuned
  `stale-in-progress`, whose 72h/3-day horizons and `[reclaimable]` machine contract serve a
  different remedy.

Two riders: **densify the claim heartbeat** so every metadata commit re-stamps `claimed_at` (today
only reconcile and `implemented` do, leaving the whole plan→build→review span unstamped), and
**split the §5 sentence** that fuses the obligation to proceed with the obligation to stay silent —
the sentence both incidents misread.

ADR-0044 gets a dated `## Update` note: the decision stands, but its remedy generating a new failure
surface is context worth recording against it.

Every predicate must be mutation-tested — a completion check that cannot fail is this defect wearing
a badge. Design detail, the cut predicates and why, and the ripple list live in the linked spec.

## Out of scope

- Reversing or weakening ADR-0044. Pre-specification at the call site works and should stay.
- Re-litigating `context: fork` (ADR-0024) or the wrapper mechanism.
- Any change to `superpowers:writing-plans` itself — it is vendored, and 0096 already settled that
  docket adapts at its own call sites rather than patching the plugin.

## Open questions

All four resolved during grooming (2026-08-02) — see the linked spec for the reasoning.

## Reconcile log

### 2026-08-03

Reconciled at claim. The spec's ripple list and its assumptions were re-verified against
`origin/main` at `06a403e3` (0191's terminal publish, the most recent check-id shipped through this
same path):

- `BOARD_CHECK_IDS` in `scripts/lib/docket-frontmatter.sh` holds exactly the **fourteen** ids the
  spec names, `scalar-form` among them — `aborted-run` sorts alphabetically first, as designed.
- `broken-plan-results` (field set, file missing on `INTEGRATION_BRANCH`) and `stale-in-progress`
  (lease TTL + hardcoded 3-day branch-idle horizon, trailing `[reclaimable]` marker) are both
  present and shaped as the spec describes, so leg A's near-mirror implementation and leg B's
  separate-check-id rationale both still hold.
- The `docket-implement-next` §"field-write rule" sentence the heartbeat rider densifies still reads
  "Each LATER phase-boundary metadata commit (reconcile, `implemented`) also RE-STAMPS
  `claimed_at`" — unchanged, so the rider applies verbatim.
- ADR-0044 is `Accepted`; the dated `## Update` note ships atomically via this change's `adrs: [44]`.

No scope adjustment, no work found already done elsewhere. Change 0190
(close-the-build-evidence-value-gap) is `in-progress` and touches the adjacent post-gate-commit
concern, but is not a dependency and shares no surface with this change's legs.
