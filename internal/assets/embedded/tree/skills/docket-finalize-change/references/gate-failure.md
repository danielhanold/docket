# gate-failure — the rebase/gate failure flows

The rebase and local-gate *failure* flows for `docket-finalize-change` — read when the sequence
does not pass clean (a rebase conflict, a red rebased suite, an unavailable dispatch, or any
abort-and-report condition). Loaded on demand from `docket-finalize-change/SKILL.md`; sibling files
are not auto-loaded.

## The two agents (split at rebase-completion)

`docket-rebase-resolver` resolves conflicts *during* the rebase and never runs Git rebase mechanics
or tests; `docket-integration-repair` owns the **red suite** *after* the rebase lands, regardless of
cause. Neither wraps a skill (only `docket-convention`); both are dispatched **foreground at the
model/effort its wrapper resolves** — never a literal model or effort. Either dispatch payload includes:
Feature worktree: <absolute canonical feature-worktree root>
This harness-neutral input serves the feature-scoped role; Codex enters it through its installed
contract, other harnesses through their native worktree mechanism.

Both agents return an **authored hint, never authority**: the controller feeds it to the matching
`docket` operation, which verifies every mechanical claim against live Git — the reported paths
against the live unmerged set, the claimed commits against the real branch delta — before the next
effect. Report bodies are redaction-only, never echoed into a result document.

1. **The resolver** reconciles each conflicted hunk by merge intent in the returned workspace and
   returns a versioned `ResolverReport` JSON document — no argv. Its fields:
   `change_id` (int), `attempt` (the owned rebase attempt token from the conflicted result),
   `disposition` (`resolved` | `stuck`), `summary` (bounded prose), `touched_paths` and
   `conflicted_paths` (repo-relative), `observed_head`, `observed_base`, `recommended_action`, and
   `resolver_reservation` (the reservation token from the `reserved` result, echoed back verbatim).
   `conflicted_paths` lists authored paths only; in Docket's own repository, bundle outputs under
   `internal/assets/embedded/` are omitted and controller-regenerated.
   The controller feeds a `resolved` report to the `finalize.rebase-continue` operation with `--id <id>
   --attempt <attempt> --input <report>`, which stages exactly the reported-and-verified paths and
   continues; a `stuck` report, or paths outside the live unmerged set (refused `report-not-resolved`),
   routes to the `finalize.rebase-abort` operation with `--id <id> --attempt <attempt> --input <report>` and a
   `halted` outcome. Each resolver dispatch is **admitted by a durable `finalize.resolver-reserve`
   reservation** — Go enforces the `finalize.resolver_max_attempts` budget (default 10), not a
   skill-side counter; a spent budget surfaces as reserve `exhausted` or continue `resolver-budget-exhausted`.
2. **The repair agent** root-causes the red rebased suite, authors a **bounded** minimal fix within
   the configured `finalize.repair_max_attempts` budget (default 6, the initial attempt included), commits it on the feature branch, and returns a report naming its **claimed
   commits** and `repaired` | `stuck`; it never weakens a test, runs the rebase, or merges or
   transitions metadata. The controller re-gates the repaired head through the gate driver: the
   `gate.drive.start` operation with `--repo-dir <feature worktree> --owner finalize --change-id
   <id> --run-root <dir> --json` (response captured into `gate_reply`), then the
   `gate.drive.advance` operation with `--drive-id <id> --owner-gen <gen>` one slice per call until
   a terminal disposition, under `docket-build`'s gate-run posture. The `--owner finalize` drive
   runs `finalize.test_command` and charges no build attempt. A `PASSED` drive whose head equals
   the repaired head feeds the `evidence.record` operation with `--owner finalize --id <id> --run
   <raw run dir from the PASSED document> --head <repaired head>`, which records
   `finalize.test_command`; `FAILED` returns to repair within that budget;
   `HALTED`, a `stuck` repair, or a repair that cannot reach green within that budget is `halted`.

## A green repair merges

A repair that turns the rebased suite green publishes and merges like any other green change, on
autonomous and attended runs alike — finalize adds no human stop of its own, so a repair gets no
`finalize.block`, no prompt, and no `finalize.clear-block`. The repair stays visible: the run's final
report names what broke, the claimed repair commits, and the attempts used, and closeout records the
same facts as a `late_findings` entry under `## Closeout notes`. A refused notes request is retried
once without notes; a lost note never stops closeout.

A run that publishes a repair and then halts before closeout (a denied merge, an `unknown`, a
`record-invalid`) names the repair in its `finalize.block` request. The `report` reaches only the PR
comment and the `remedy` is what the `## Finalize blocked` section records, so both carry what broke,
the claimed repair commits, and the attempts used, the remedy opening with `Authored repair:`. A later
run whose change carries that section turns each `Authored repair:` remedy into its own closeout
`late_findings` entry, because closeout strips the section and the note is all that survives.

Approval is the repository's policy, not a docket gate. When branch protection requires approvals and
has GitHub's "Dismiss stale pull request approvals when new commits are pushed" turned on, publishing
the repair dismisses the PR's approval: GitHub refuses the merge (`halted`), and with
`finalize.require_pr_approval: true` auto-detect skips the PR as `approval-required` until a human
approves it again. That setting is off by default; with it off the earlier approval stands and the
repair merges. A team that wants repairs re-reviewed turns it on.

## abort-and-report points (the full set)

Each maps to the **`halted`** disposition and leaves the **PR open** and the change **`implemented`**:

Two outcomes look abort-shaped and are not: a `waiting` (`reason: gate-waiting`) re-enters `finalize.rebase` (below), and a reservation-reconciliation write failure after Git advanced or completed the owned continuation is **not** in this set — see *The reconciliation-write exception* below.

- an **ambiguous rebase conflict** — the resolver returns `stuck`, or the resolver budget is spent
  (`finalize.resolver-reserve` returns `exhausted`, or a continue returns `resolver-budget-exhausted`);
  the owned rebase is restored via the `finalize.rebase-abort` operation;
- a **red rebased suite the repair cannot green** within the configured repair budget (`stuck`);
- an **unresolved effective base, foreign in-progress rebase, divergent (rewritten) base, or dirty workspace** —
  the `finalize.rebase` operation returns `blocked` (a merely forward-advanced base is no longer here — it
  forward-rebases the completed rewrite instead; see `SKILL.md`'s moved-base paragraph);
- a **rewrite the publish cannot certify** — the `finalize.publish` operation returns `rewrite-unknown`/
  `pr-probe-failed` (an `unknown` never authorizes a second mutation);
- a **merge condition that fails at the fresh recheck** or an authoritatively **denied** merge —
  the `finalize.merge` operation returns the condition's token or `merge-denied`; a standing denial is
  `halted`, never retried;
- an **open unauthorized child** on an autonomous run, or a `children-retarget-required` closeout;
- the **dispatch mechanism being unavailable** for either gate agent — the `no-fallback` posture,
  established only per the convention's *Dispatch-capability resolution*, never from a tool name,
  and never substituted inline;
- a **deferred capability requested by config** — any mutating operation returns `unsupported-config`
  before any effect, naming the blockers.

A `contended` from any operation is **not** in this set: it is a lost race the next `context.finalize`
read resolves, a continue-able outcome the driver re-selects past, never `halted`.

A `waiting` (`reason: gate-waiting`) is not in this set either: the suite is still running and the
owned receipt carries the drive continuation — re-run the identical `finalize.rebase` invocation
(never `gate drive advance`) until a terminal disposition.

## The reconciliation-write exception (recover, not abort)

An owned resolver continuation can succeed in Git — advancing to another conflict, or completing the rebase — and then fail only the durable write that reconciles the reservation on the receipt (`receipt-write-failed`, with a message naming this exception and the failed write). Aborting here restores the recorded original head and discards the completed local rewrite, so this window is recovered, never aborted:

1. Preserve the workspace, the receipt, and the original resolver report. Re-run `finalize.rebase-continue` with the same `--id <id> --attempt <attempt> --input <report>` (the report still carries its `resolver_reservation` token). Resolve the invocation from capabilities and the report shape from schema as usual. Recovery rechecks live state and reconciles the outstanding continuation; it charges and refunds nothing and authorizes no new resolver dispatch.
2. Do not route this persistence failure to `finalize.rebase-abort`, restart the rebase, reserve or dispatch another resolver, fabricate a replacement report, or edit or delete the receipt. A generic `blocked` disposition or the `receipt-write-failed` token alone is insufficient to diagnose this window — the operation's message says which write failed and what it proved; its ownership and live-state checks stay authoritative, so never reproduce them with handwritten Git probes.
3. This is an operator remedy, not an autonomous retry. If persistence still fails, or the original report is unavailable, halt with the work retained: report the actual diagnostic and the missing input, and record the block through the existing finalize-block path where possible — a failed block recording is reported honestly and never authorizes abort.
4. Follow the recovery's actual result. A new conflict requires normal reserve-before-dispatch admission; an exhausted budget keeps its existing abort/halt route; a completed rebase still passes the normal gate and publication checks; a `waiting` (`gate-waiting`) resumes via the original identical `finalize.rebase` invocation, never another `rebase-continue` or a direct gate-drive call. A successful reconciliation consumes the reservation — do not replay the old report afterward.
5. Everything else keeps its verified abort route: stuck or unavailable resolvers, a continuation still stopped on the same commit, foreign or unprovable state, legacy receipts, and exhausted budgets. Establish resolver-child completion before any abort, as ever; this exception never authorizes abort on an unproven state or bypasses an existing refusal. Write failures before Git ran (reserve admission, the continuation-started marker) prove nothing about completion and carry no recovery claim.

## The finalize gate shares the worktree's one lock

Finalize runs its post-rebase suite as a **scopeless** gate in the feature worktree, and that gate
takes the same worktree lock every other gate takes: one canonical worktree carries at most one
running gate at a time. So finalize's own gate can be **refused** before it launches when another
gate's supervisor holds the lock — reason `worktree-busy`. This is a **blocking diagnostic, not a
rebase conflict and not a red suite**: it is in neither the abort-and-report set above nor a
`contended`/`waiting` continuation. Do not race a second gate. The remedy is operator-side — let
the holding gate finish, or stop it (the `run.cancel` operation with `--key <key> --reason <why>`
for a tracked run, `gate.stop <run-dir>` for a raw launch) — then re-run finalize. The worktree
frees itself when the holder ends. A gate whose own supervisor dies mid-run is never relaunched:
it halts `supervisor-died` (finalize reports `gate-halted`), and the remedy is to re-run
finalize, which re-runs the suite. When the halt message says part of the suite is still running
as a process group, wait until `pgrep -lg <pgid>` prints nothing before re-running finalize; that
`tree-survives` finding is information only and never a blocker.

**Where the reason surfaces.** The subagent returns its diagnosis in-context; finalize relays it to
the human (interactive) or the dispatching caller (autonomous), and the `finalize.block` operation records
it durably — first as the owned **comment on the PR** (idempotent by the attempt marker, so a
human returning later reads exactly why the auto-merge stopped), then as the `## Finalize blocked`
marker on the change record. The comment is the narrative, the marker is the state; the operation
writes them in that order so a crash between them replays by finding the comment by its marker.

## The `## Finalize blocked` marker — write shape and lifecycle

A gate or merge failure is recorded as a `## Finalize blocked` body section on the change record — a
`finalize.block` metadata write in an exact-revision transaction, never a hand-edit. It is **not** a new
lifecycle status or a reuse of `blocked`: the change really *is* `implemented` with an open PR, and a
transient multi-cause abort encoded as a status would make every derived view say six things about one
label. `stacked-merged` earns a status on the terms this case fails — one durable position, one cause, one exit.

- The single section names **which** reason fired (the `--reason` token) and what the human must do;
  a re-mark **replaces** the interior or appends a dated attempt bullet, never a second heading. It
  validates marker order and balance before rewriting, rerendering the inline board in one transaction.
- The section is a **visible note, never a stop**: selection and merge ignore it, so the next run
  retries the change and a transient failure (a flaky test, a busy worktree, a moved base) heals on
  its own. An **already-merged PR is a merged-recovery candidate** as always.
- A **`CONFLICTING` PR is not marked at selection time** — the resolver usually resolves it.
  Marking happens only at an abort-and-report point.
- **The `finalize.clear-block` operation removes the section** on an unmerged change by hand: it
  reprobes the exact current head, valid gate evidence, the published remote ref, and the matching
  open PR before removal — each missing condition refuses and the marker stays. Closeout strips a
  stale section when it records a merged change `done` or `stacked-merged`, so a merged record
  carries no marker.
