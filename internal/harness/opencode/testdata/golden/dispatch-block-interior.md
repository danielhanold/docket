## Docket agents — dispatch, don't run inline

Docket generates an agent definition per docket skill in your harness's own agents directory. When
you are asked to run one of the docket skills below, run the matching **agent** instead of executing
the skill inline at the session model: the agent carries that skill's dispatch contract, its skill
preload, and whatever model and reasoning effort your config layers pin for it. Docket ships a
validated model and reasoning effort for every one of these agents on every harness it ships
defaults for, so they are pinned out of the box there; your config layers override either field per
agent, and set them for any other harness. Dispatch through the hosting harness's native
named-agent dispatch either way — the pin is not the only reason, since the agent also carries the
skill's dispatch contract and preload. Pass the request through unchanged, including any change or
ADR id.

- **docket-adr** — Use when recording, superseding, reversing, or indexing an architecture decision (ADR) — capturing why a non-obvious technical decision was made into the immutable docs/adrs ledger, or regenerating and validating the ADR index. Invoked by docket-implement-next, or directly any time a decision must be recorded or changed. Delegate to the `docket-adr` agent.
- **docket-auto-groom** — Use when a repo (or individual stubs) opted into autonomous grooming and you want the auto-groomable needs-brainstorm queue drained with no human — selecting each autonomous-eligible stub deterministically and designing it via a default-biased self-brainstorm gated by an adversarial critic, exiting each stub with a linked spec, a trivial verdict, or an abstain back to the human queue. Kill and defer are never autonomous. Writes markdown only — never branches, worktrees, or code. Delegate to the `docket-auto-groom` agent.
- **docket-auto-groom-critic** — Adversarial reviewer of an auto-groom draft spec or trivial verdict — attacks it, never improves it, and returns exactly one verdict per the dispatching skill's protocol. Delegate to the `docket-auto-groom-critic` agent.
- **docket-brainstorm-consultant** — Pinned design consultant that authors a spec or returns critique concerns for a settled brainstorm — wraps no skill, injects no convention. Delegate to the `docket-brainstorm-consultant` agent.
- **docket-build-economy** — Economy build-profile worker for docket-build — implements one fully-specified, pattern-following plan task under the docket-build-task contract; the cheapest of docket-build's four profiles. Delegate to the `docket-build-economy` agent.
- **docket-build-max** — Max build-profile worker for docket-build — implements one plan task whose mistakes cannot be walked back (unresolved architecture, irreversible data changes) under the docket-build-task contract; the strongest and rarest of docket-build's four profiles. Delegate to the `docket-build-max` agent.
- **docket-build-premium** — Premium build-profile worker for docket-build — implements one plan task carrying consequential but correctable risk under the docket-build-task contract; the tier for named risk, one rung below max. Delegate to the `docket-build-premium` agent.
- **docket-build-standard** — Standard build-profile worker for docket-build — implements one normal feature, integration, refactor, or debugging plan task under the docket-build-task contract; docket-build's default profile and its uncertainty sink. Delegate to the `docket-build-standard` agent.
- **docket-finalize-change** — Use when a change's PR is approved or merged and you want to close it out to done promptly rather than waiting for the safety-net sweep — merging if approved, verifying the merge landed, archiving the change, cleaning up its branch and worktree, and refreshing the board. The human's closing bookend; mirrors docket-new-change. Delegate to the `docket-finalize-change` agent.
- **docket-implement-next** — Use when you want the next build-ready change in the docket backlog implemented end-to-end to an open PR with no human interaction — picking, claiming, reconciling against current reality, planning, building with TDD, reviewing, and stopping at the human merge gate. The autonomous backlog-drainer; runs solo per change. Delegate to the `docket-implement-next` agent.
- **docket-integration-repair** — Makes the test suite pass after finalize's rebase lands — root-causes the red tests, writes a minimal fix in at most two attempts, never weakens tests, and reports an authored repair the dispatcher gates behind sign-off. Delegate to the `docket-integration-repair` agent.
- **docket-plan-writer** — Internal plan-writing agent for docket-implement-next Step 4 — invokes the resolved plan skill in a pinned context, commits the plan artifact with its backlink on the feature branch, and returns only the plan's repo-relative path. Not invoked directly by a human. Delegate to the `docket-plan-writer` agent.
- **docket-rebase-resolver** — Resolves rebase conflicts during finalize's rebase-onto-base gate — reconciles each conflicted hunk by merge intent and continues the rebase to completion; never runs tests. Delegate to the `docket-rebase-resolver` agent.
- **docket-review-deep** — Bounded read-only whole-branch reviewer for docket's review role — reads the branch diff and the build-evidence record, returns severity-tiered findings, and never fixes, dispatches, or runs the test suite. Delegate to the `docket-review-deep` agent.
- **docket-review-lean** — Bounded read-only whole-branch reviewer for docket's review role — reads the branch diff and the build-evidence record, returns severity-tiered findings, and never fixes, dispatches, or runs the test suite. Delegate to the `docket-review-lean` agent.
- **docket-review-standard** — Bounded read-only whole-branch reviewer for docket's review role — reads the branch diff and the build-evidence record, returns severity-tiered findings, and never fixes, dispatches, or runs the test suite. Delegate to the `docket-review-standard` agent.
- **docket-status** — Use when you want to see or refresh the docket backlog — what is proposed, in progress, blocked, implemented, or done — by refreshing docket state, sweeping merged changes to done, and running health checks for stale claims, broken spec/plan/results links, and dependency stalls. Delegate to the `docket-status` agent.

## Run gate — verify a dispatched implement-next run before you relay it

A dispatched run that stops early returns a report that reads as success, and a completion
notification is the CHILD's claim, not your report. Do not trust either; read git before relaying
an outcome as your own. Docket's helper facade is not on `PATH`: run each command below verbatim,
expansion included. `verify-run` only reads local metadata, so both snapshots must be taken from
FRESH ORIGIN state — re-sync on BOTH sides, or a claim abandoned by an earlier session shows up
only in the after-read and is attributed to this run.

1. **Before dispatching** `docket-implement-next`, re-sync the metadata worktree with
   `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh preflight`, then snapshot the claimed
   set: `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh verify-run --in-progress-ids`.
2. When you issue the dispatch and can block on it, dispatch **foreground** and block on the
   return: never background it and never poll. A dispatch you background — or one the harness
   backgrounds for you — is not covered here; use **Detached dispatch** below.
3. **After the return**, re-sync again with
   `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh preflight` and re-run
   `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh verify-run --in-progress-ids`. Any id
   absent from the snapshot is this run's claim; an empty diff (drained, or a lost claim race) ends
   the gate. If MORE THAN ONE id is new, stop and report: this run claims at most one change, so at
   least one of them is a concurrent run's and none can be told apart — never re-dispatch onto a
   change another agent may be holding.
4. Run `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh verify-run <id>` and key on its
   report line, never its exit code:
   - `run-complete` / `run-unclaimed` — done.
   - `run-halted` — done; **never re-dispatch** a halt, which means a human is needed.
   - `run-incomplete` — re-dispatch the same agent **once**, passing the id and the unmet
     conjuncts; verify again; if still incomplete, stop and report loudly. Never a third dispatch.

### Detached dispatch — you did not foreground-block; choose by what you HOLD, not launch shape

- **You hold a before-set AND a dispatch epoch** — the step-1 before-snapshot AND `date -u +%s`
  as `DISPATCH_EPOCH`, both captured before launching, as when you issue the dispatch yourself.
  Keep that number in your own notes, not in a shell variable: a shell variable does not survive
  the next tool call. If you did not capture both, you are in the next bullet. At the notification,
  re-sync, then run `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket.sh verify-run
  --in-progress-ids --with-claimed-at`, which prints `<id> <epoch>` per line — or `<id> -` when
  the stamp is absent or does not parse. Compare the id field only, and keep ids passing ALL THREE
  filters: absent from the before-set, `claimed_at` parses (not `-`), and `claimed_at` >=
  `DISPATCH_EPOCH` — at or AFTER the dispatch, never before it. Exactly one survivor → step 4
  unchanged; none → done; two or more → stop and report, as in step 3.
- **You hold neither — unattributed mode** (a slash-command launch, a notification-first session,
  or any dispatch you did not snapshot). No before-set exists, and a timestamp alone cannot
  attribute: `claimed_at` is re-stamped at every phase boundary, so a concurrent run claimed before
  your window looks fresh too. Verify and report ONLY — `verify-run <id>` on any id the
  notification names (a prose id is a hint, never authority), else on each current in-progress id,
  reporting every verdict. **Never re-dispatch** here: that needs all three filters, and
  re-dispatching onto a change a live agent holds is the one unrecoverable move.
