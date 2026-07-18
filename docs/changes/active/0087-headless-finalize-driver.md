---
id: 87
slug: headless-finalize-driver
title: Headless finalize — the finalize-side disposition contract, mirroring 0088
status: proposed
priority: high
created: 2026-07-17
updated: 2026-07-18
depends_on: []
related: [8, 88, 95]
adrs: [43]
spec:
plan:
results:
trivial: false
auto_groomable: false
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| ADRs | [ADR-0043](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0043-retire-bot-auto-approval-zero-approvals-branch-protection.md) |
<!-- docket:artifacts:end -->

## Why

**Nothing invokes `docket-finalize-change` hands-off today.** A human who wants "close out the
merge gate, walk away, come back to merged PRs" has no way to get it — verified 2026-07-18: no
driver surface exists anywhere in `scripts/` or `skills/`. That gap is the whole change, and it
survives every shift in the machinery underneath it.

Two things changed on 2026-07-18 that make this both **simpler** and **well-precedented**.

**The merge wall is gone (change 0095, ADR-0043).** This change was originally framed as "ship a
consumer for 0062's `auto_approve` capability" — dispatch the bot workflow, poll it, verify
`reviewDecision`, then merge. That subsystem was retired in full: the knob, the workflow, the
setup script, and the gate's step 6 are deleted, and ADR-0042 is reversed. Branch protection set
to **require a PR with zero approvals** now lets a plain `gh pr merge --rebase` land with no
`--admin`, no bot, and nothing for a permission classifier to deny. The hard part of the original
scope — an approval chain with its own failure modes — simply evaporated. What remains is
invocation.

**Change 0088 already solved the shape of this problem on the implement side.** Its answer was a
**driver-agnostic re-invocation contract** and deliberately **no loop primitive and no new entry
surface**: `docket-implement-next` ends every run declaring one of four dispositions
(`advanced` / `contended` / `drained` / `halted`), a driver keys on them (continue on the first
two, stop on the last two), and the built-in `/loop` is documented as the *recommended* driver
rather than something docket owns. 0088's own reconcile log partitions the space explicitly —
0088 serial self-continuation, #0008 concurrent fan-out, **#0087 finalize** — and its shipped
contract table names this change in the driver list. This change is the finalize-side half of
that partition, and it should mirror the design rather than invent a second vocabulary for the
same job.

**`/loop` is confirmed working on the implement side (2026-07-18, CC 2.1.214).** The maintainer
has run `/loop docket-implement-next <id-set>` against real backlog changes and reports it drives
cleanly. That retires 0088's deferred §6 spike as a blocker for this change: the load-bearing
half — does `/loop` compose with a forked docket skill and advance across iterations — is
answered empirically. Scope the claim honestly: an id-set run ends by exhausting its set rather
than on an empty-backlog `drained`, and neither `contended` (needs two agents racing) nor
`halted` (needs a failure) was exercised. Those are far smaller risks than "does this compose at
all," and per `harness-behavior-is-mode-and-version-scoped` the observation is scoped to
**CC 2.1.214**, not to `/loop` forever.

## What changes

- **A terminal disposition contract on `docket-finalize-change`**, mirroring 0088's — the **same
  four words**, so one driver keys on both skills without knowing which it is driving:

  | Disposition | Finalize meaning | Driver action |
  |---|---|---|
  | `advanced` | Merged one change → closed out. | continue |
  | `contended` | Another writer got there first (the `docket-status` sweep archived it between selection and close-out); the archive is an idempotent no-op, **nothing merged**. | continue — re-select next |
  | `drained` | No eligible `implemented` change in scope. | **stop** |
  | `halted` | Any abort-and-report point. | **stop + surface** |

  **Decision (2026-07-18): adopt `contended` verbatim** rather than folding the race into
  `advanced` or shipping only three dispositions. There is no claim CAS here, so the *mechanism*
  differs — but the driver-facing meaning is identical ("someone else got there, nothing to do,
  keep going"), and a shared vocabulary is what keeps the driver skill-agnostic. Divergence
  would force a driver to know which skill it is running, eroding the property that justified
  0088's design. Prose on the skill, no scripts — as in 0088.

- **One merge per invocation ("single") — decided 2026-07-18.** A run merges **exactly one**
  change and exits `advanced`; it never batches. Consecutive close-outs come from the driver
  re-invoking, not from an in-run loop.

  **Ordering falls out of re-selection — no sequencing machinery.** Each invocation re-derives
  "best next" against the **current** `origin/<integration_branch>`, so no precomputed order can
  go stale. This is the direct answer to the moving-base problem: every merge moves the base, and
  a plan authored before the first merge is a prediction about a base that no longer exists by the
  third. Selection order: **`depends_on` order first** (a dependency is satisfied only at `done`,
  so a dependent can never merge ahead of it), then docket's existing deterministic order —
  priority → age (`created`) → lowest id.

- **Id-set scoping** — generalize finalize's existing explicit-id argument to an allowlist
  (`docket-finalize-change 90,92,94`): the set bounds *which changes are eligible*, and the run
  still merges only the best-ordered one of them. This matters more here than it did for
  implement-next: finalize's Selection matrix currently guards a multi-change batch with an
  **interactive prompt**, and a headless run cannot answer it. Naming the ids **is** the
  authorization that prompt would have collected.
- **Map every existing abort-and-report point to a disposition.** Finalize already has a
  well-enumerated abort set (ambiguous rebase conflict, no detectable suite, repair can't reach
  green in ≤2 attempts, red/absent CI, a rejected `--force-with-lease`, and any auto-authored
  repair under an autonomous run). These are already the right stop-and-surface semantics; this
  change names them as `halted` rather than adding new behavior.
- **Wire the stop reason somewhere a human reads.** Finalize already records abort reasons as a PR
  comment; confirm that channel covers the headless case, where nobody is tailing a log.
- **Document the drain pattern** alongside 0088's README section, framed the same way —
  recommended, confirm-in-your-harness — so both halves of the loop read as one system.

## Out of scope

- **Building a loop primitive, a `docket-drain` skill, or any new entry surface** — 0088's
  precedent is explicit and this change follows it. The driver is `/loop`, cron, a scheduled
  agent, or a human re-typing the command.
- **Concurrent/parallel fan-out** — #0008 owns that, and can build on this vocabulary the same
  way it can build on 0088's.
- **The rebase-retest gate, `require_pr_approval`, terminal-publish, or the consent model
  (ADR-0011 / ADR-0043).** This change consumes them; it does not revisit them.
- **Re-opening the retired `auto_approve` mechanism.** It is deleted and its ADR reversed; there
  is no bot chain left to drive.

## Open questions

**Settled 2026-07-18** (maintainer decision, recorded above): single-merge-per-invocation, not a
drain · `contended` adopted verbatim for vocabulary parity · ordering by re-selection, not a
precomputed sequence · `/loop` composition confirmed live (see *Why*), so 0088's deferred §6 spike
needs no follow-up change for this change to proceed.

- **How is a gate-failed change marked so a human sees it?** The open question, and the one piece
  of new surface this change may need. Today a gate abort leaves the change `implemented` with the
  PR open and surfaces only in the run report + a PR comment — correct state, but **invisible on
  the board**, so a `/loop` drain that halts on #92 leaves nothing on the planning view saying so.

  **Recommended shape: a dated `## Finalize blocked` body section, NOT a new status.** It mirrors
  the proven `## Auto-groom blocked` pattern exactly — presence drives a distinct board cell
  (`auto-groom blocked — needs you`), the reason lives in prose, a human deletes it on re-arm —
  and it costs no lifecycle change, no GitHub-mirror status remap, and no health-check rework.

  Two arguments against reusing `blocked` or minting an eighth status: (1) it would **erase true
  information** — the change really is `implemented` with an open PR, while `blocked` sits in the
  `in-progress` neighborhood of the lifecycle; (2) **"needs its PR refreshed" is only one of six
  abort reasons** (ambiguous rebase conflict · no detectable suite · repair stuck after ≤2
  attempts · red/absent CI · rejected `--force-with-lease` · autonomous repair sign-off), so a
  single status flattens six situations into one label and drops the part a human needs. The
  reason must travel with the marker.

  Left to settle: the exact section name and board cell wording; whether the six abort reasons
  need distinguishing on the board or only in the section body; and whether a re-run of finalize
  clears the section automatically (unlike auto-groom's human-only re-arm) once the gate passes.

- **Does the driver re-verify classifier posture per run?** Still live, and the pin needs a **new
  home**: CC 2.1.211 was pinned in ADR-0042, which ADR-0043 reversed. Exposure is much smaller now
  — no dispatch to deny, only `gh pr merge` itself, against protection requiring zero approvals —
  but the `harness-behavior-is-mode-and-version-scoped` learning is explicit that a headless
  observation is version-scoped. Detect and degrade, or just `halted`?

- **Should the contract be docket-owned at all, or the consuming repo's job?** 0088's answer for
  the implement side was *docket owns the contract, not the driver*; almost certainly the same
  here — confirm rather than assume.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->
