# gate-failure — the rebase/gate failure flows

The rebase and local-gate *failure* flows for `docket-finalize-change` — read when the sequence
does not pass clean (a rebase conflict, a red rebased suite, an unavailable dispatch, or any
abort-and-report condition). Loaded on demand from `docket-finalize-change/SKILL.md`; sibling files
are not auto-loaded with the skill.

## The two agents (split at rebase-completion)

`docket-rebase-resolver` resolves conflicts *during* the rebase and never runs Git rebase mechanics
or tests; `docket-integration-repair` owns the **red suite** *after* the rebase lands, regardless of
cause. Neither wraps a skill (only `docket-convention`); both are dispatched **foreground at the
model/effort its wrapper resolves** — never a literal tier. Name the **feature worktree** in the
dispatch payload for either agent — both are feature-scoped, so reached through a runner delegation
each receives that worktree through the facade's `--worktree` flag, and a delegated dispatch that
names none is refused. An authored repair from `docket-integration-repair` is what fires the
sign-off rule below; pure conflict resolution does not.

Both agents return an **authored hint, never authority**: the controller feeds it to the matching
`docket` operation, which verifies every mechanical claim against live Git — the reported paths
against the live unmerged set, the claimed commits against the real branch delta — before the next
effect. Report bodies are redaction-only and are never echoed into a result document.

1. **The resolver** reconciles each conflicted hunk by merge intent in the returned workspace and
   returns a versioned `ResolverReport` JSON document — no argv. Its fields:
   `change_id` (int), `attempt` (the owned rebase attempt token from the conflicted result),
   `disposition` (`resolved` | `stuck`), `summary` (bounded prose), `touched_paths` and
   `conflicted_paths` (repo-relative), `observed_head`, `observed_base`, and `recommended_action`.
   The controller feeds a `resolved` report to the `finalize.rebase-continue` operation with `--id <id>
   --attempt <attempt> --input <report>`, which stages exactly the reported-and-verified paths and
   continues; a `stuck` report, or paths outside the live unmerged set (refused `report-not-resolved`),
   routes to the `finalize.rebase-abort` operation with `--id <id> --attempt <attempt> --input <report>` and a
   `halted` outcome. The resolver gets **at most two dispatches, enforced by the skill**.
2. **The repair agent** root-causes the red rebased suite, authors a **bounded** minimal fix in at
   most two attempts, commits it on the feature branch, and returns a report naming its **claimed
   commits** and `repaired` | `stuck`; it never weakens a test, never runs the rebase, and never
   merges or transitions metadata. The controller re-runs the gate on the repaired head through
   the `gate.launch`/`observe` operations and records the exact-head evidence through the
   `evidence.record` operation — a `stuck` repair, or a repair that cannot reach green in two attempts, is `halted`.

## Sign-off on auto-authored repairs

A repair is code the human's PR approval predated, so it never merges unseen:

- **Autonomous finalize** cannot prompt. It records the sign-off requirement durably and STOPS:
  the `finalize.block` operation with `--id <id> --version <version> --pr-number <n> --attempt <attempt>
  --reason repair-needs-signoff --head <repaired head> --input <block report>` — the disposition is
  `halted`. The human reviews the pushed repair on the PR and re-runs finalize; the retry clears the
  block (the `finalize.clear-block` operation) and merges.
- **Interactive finalize** publishes the repaired head, reports the repair diff and what broke, and
  **prompts** for go-ahead before the `finalize.merge` operation.

## abort-and-report points (the full set)

Each maps to the **`halted`** disposition and leaves the **PR open** and the change **`implemented`**:

- an **ambiguous rebase conflict** — the resolver returns `stuck`, or is still `conflicted` after
  its second dispatch; the owned rebase is restored via the `finalize.rebase-abort` operation;
- a **red rebased suite the repair cannot green** in ≤2 attempts (`stuck`);
- an **authored repair under autonomous finalize** — the sign-off rule above (`repair-needs-signoff`);
- an **unresolved effective base, foreign in-progress rebase, moved base, or dirty workspace** —
  the `finalize.rebase` operation returns `blocked`;
- a **rewrite the publish cannot certify** — the `finalize.publish` operation returns `rewrite-unknown`/
  `pr-probe-failed` (an `unknown` never authorizes a second mutation);
- a **merge conjunct that fails at the fresh recheck** or an authoritatively **denied** merge —
  the `finalize.merge` operation returns the conjunct's token or `merge-denied`; a denial that still stands
  is `halted`, never a retry loop;
- an **open unauthorized child** on an autonomous run, or a `children-retarget-required` closeout;
- the **dispatch mechanism being unavailable** for either gate agent — the carve-out below,
  established only per the convention's *Dispatch-capability resolution*, never from a tool name,
  and never substituted inline;
- a **deferred capability requested by config** — any mutating operation returns `unsupported-config`
  before any effect, naming the blockers.

A `contended` from any operation is **not** in this set: it is a lost race the next `context.finalize`
read resolves, a continue-able outcome the driver re-selects past, never `halted`.

**Where the reason surfaces.** The subagent returns its diagnosis in-context; finalize relays it to
the human (interactive) or the dispatching caller (autonomous), and the `finalize.block` operation records
it durably — first as the owned **comment on the PR** (idempotent by the attempt marker, so a
human returning later reads exactly why the auto-merge stopped), then as the `## Finalize blocked`
marker on the change record. The comment is the narrative, the marker is the state; the operation
writes them in that order so a crash between them replays by finding the comment by its marker.

## The `## Finalize blocked` marker — write shape and lifecycle

A gate or merge failure is recorded as a `## Finalize blocked` body section on the change record —
a metadata write through the `finalize.block` operation's exact-version transaction, never a hand-edit. It
is deliberately **not** a new lifecycle status and **not** a reuse of `blocked`: the change really
*is* `implemented` with an open PR, and a transient multi-cause abort encoded as a status would
force every derived view to say six different things about one label. `stacked-merged` earns a
status on exactly the terms this case fails — a durable position with one cause and one exit.

- The single section names **which** reason fired (the `--reason` token) and what the human must do;
  a re-mark **replaces** the interior or appends a dated attempt bullet — never a second heading. The
  operation validates marker order and balance before rewriting and rerenders the inline board in the
  same transaction.
- **Auto-detect selection skips** any unmerged change already carrying the section — without this a
  re-run re-selects the same known-bad change forever. A **named id or allowlist member overrides**
  the skip (naming the id is the human's "I looked at it, retry" signal); an **already-merged PR is
  a merged-recovery candidate regardless** of the marker.
- A **`CONFLICTING` PR is not marked at selection time** — the resolver usually resolves it, so
  marking up front would strand a fixable PR. Marking happens only at an abort-and-report point.
- **A successful finalize removes the section** via the `finalize.clear-block` operation, which reprobes the
  exact current head, valid gate evidence, the published remote ref, and the matching open PR before
  removal — each missing conjunct refuses. The condition is machine-verifiable, so requiring a human
  to delete it would strand stale markers on changes that are fine. Nothing strips the section at
  closeout: on an out-of-band merge it rides into the archive verbatim, where its only remaining
  reader is the human record of why the change once stalled — every automated reader is scoped to a
  change short of `done`, so archiving retires the marker's meaning whether or not the section
  survives.
