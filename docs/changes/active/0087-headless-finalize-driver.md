---
id: 87
slug: headless-finalize-driver
title: Headless finalize — the finalize-side disposition contract, mirroring 0088
status: in-progress
priority: high
created: 2026-07-17
updated: 2026-07-18
depends_on: []
related: [8, 88, 95]
adrs: [43]
spec: docs/superpowers/specs/2026-07-18-headless-finalize-driver-design.md
plan:
results:
trivial: false
auto_groomable: false
branch: feat/headless-finalize-driver
claimed_at: 2026-07-18T20:04:32Z
pr:
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-07-18-headless-finalize-driver-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-07-18-headless-finalize-driver-design.md) |
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
  | `halted` | Any abort-and-report point, **or** an eligible set in which every member needs a human. | **stop + surface** |

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
  third.

- **Order by *mergeability*, not priority — decided 2026-07-18.** The goal is to close out as many
  changes as possible per drain, so selection maximizes each attempt's chance of success:

  1. **`depends_on` order** — a hard correctness constraint, not a preference (a dependency is
     satisfied only at `done`).
  2. **GitHub's `mergeable` field** — `CONFLICTING` is excluded from selection and marked for a
     human instead.
  3. **Smallest diff first** (`changedFiles`, then `additions + deletions`) — cheaper to re-test,
     less likely to redden the suite after rebase, and lands the most changes before any halt.
  4. **priority → age → lowest id** as the final tiebreak — priority is *demoted*, not deleted; it
     still encodes human intent and guarantees a total, reproducible order.

  Measured on the real backlog 2026-07-18: `mergeable` is directly available from `gh` (though it
  resolves lazily — the first query returns `UNKNOWN` and only triggers computation, so the probe
  must poll). The tempting elaboration — pairwise file-overlap ranking — was measured and
  **discriminates nothing here**: the four mergeable PRs have completely disjoint file sets. The
  spec says don't build it.

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
- **A dated `## Finalize blocked` body section — the one piece of genuinely new surface.** Decided
  2026-07-18: a gate failure is marked with a section mirroring the proven `## Auto-groom blocked`
  pattern, **not** an eighth status and not a reuse of `blocked`. Presence drives a distinct board
  cell (`finalize blocked — needs you`), and the reason travels in prose. Rejected: an eighth
  status would erase true information (the change really is `implemented` with an open PR) and
  flatten six distinct abort reasons into one label, while forcing changes to the lifecycle
  diagram, the board renderer, the GitHub mirror's seven-state mapping, and the health checks.

  Two consequences the spec draws out: selection **skips** changes already carrying the section
  (otherwise a re-run re-selects the same known-bad change forever), and — unlike auto-groom's
  human-only re-arm — a **successful finalize clears the section automatically**, since the
  condition is machine-verifiable.

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

_Resolved during the 2026-07-18 in-session grooming — see the linked spec §5._ Settled: single
merge per invocation (not a drain) · `contended` adopted verbatim for vocabulary parity · ordering
by mergeability with priority demoted to a tiebreak · gate failures marked with a dated
`## Finalize blocked` section rather than an eighth status · docket owns the contract, not the
driver · no new ADR anticipated. `/loop` composition is confirmed live at CC 2.1.214, so 0088's
deferred §6 spike is not a blocker.

Two items deliberately left for the build to settle (spec §5, §6): whether the six abort reasons
need distinguishing on the **board** or only in the section body; and whether the driver
re-verifies harness/classifier posture per run or simply `halted`s on denial — the CC 2.1.211 pin
lived in ADR-0042, which ADR-0043 reversed, so a retained pin needs a new home.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-07-18 — reconciled against `origin/main` @ `e0fbf89`

**Design holds in full. No scope change; three facts refreshed.**

1. **Spec §6's collision risk is RESOLVED, not pending.** All three PRs it flagged as in-flight
   have merged: **#96** (change 0093, archive-decay-digest → `scripts/render-board.sh`,
   `tests/test_render_board.sh`), **#97**, and **#98** (change 0092, orphan-detection →
   `scripts/board-checks.sh`, `scripts/docket-status.md`). The base is stable at `e0fbf89`; this
   build composes against a settled renderer rather than racing it. Only **#89** (change 0078) and
   **#69** (change 0044) remain open.

2. **The mergeability measurement is stale in its particulars; its decisions stand.** §3.5 measured
   "four mergeable PRs with completely disjoint file sets" over a five-PR backlog that no longer
   exists (two PRs open now). The conclusions it supported are unaffected and remain binding:
   `mergeable` resolves lazily so the probe must poll bounded and treat still-`UNKNOWN` as
   "attempt it," and **pairwise file-overlap ranking is not built**. Re-measuring on a two-PR
   backlog would discriminate even less, so the "revisit only on evidence" posture is retained
   as-is.

3. **Board cell lands on the `implemented` section, not `readiness()`.** Confirmed in current code:
   `readiness()` (`scripts/lib/docket-frontmatter.sh`) is by contract meaningful only for a
   `proposed` change, and `render-board.sh`'s `readiness_cell` is reached only from the `proposed`
   branch of `print_section`. The `implemented` section renders `| # | Title | Priority | PR |`.
   So §3.4's `finalize blocked — needs you` cell is a **new** render path in the `implemented`
   table — it must not be bolted onto `readiness()`. The existing `has_section` helper is the
   right primitive (it is exactly what the `auto-groom-blocked` token already uses).

**Size budget, confirmed live:** `tests/test_skill_size_budgets.sh` caps
`skills/docket-finalize-change/SKILL.md` at **160 lines / 2699 words**; it currently sits at
**132 / 2266**. Headroom exists but §3.1 + §3.2 prose will likely consume most of it — raise the
row in the same diff if it does, as change 0088 did.

**Verified absent:** the finalize skill carries zero disposition vocabulary today (0 matches for
all four words), so this is net-new prose, not an edit to an existing contract.
