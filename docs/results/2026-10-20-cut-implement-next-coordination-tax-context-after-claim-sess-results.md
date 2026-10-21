<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0360 — Stop implement-next tripping over its own bookkeeping](../changes/archive/2026-10-21-0360-cut-implement-next-coordination-tax-context-after-claim-sess.md)**
<!-- docket:backlink:end -->

# Stop implement-next tripping over its own bookkeeping — Results

**Human action:** No action is required before merge. After merging, reinstall the binary and skills and restart agent sessions so new runs pick up the reworded implement-next skill and stop calling the deleted `change refresh-claim` command.

## Outcome

Implement-next runs were failing on their own bookkeeping. The skill told agents that receipts carry the record revision, so most runs' first reconcile came back `contended`. Re-reading a claimed change through `context.implementation --id` was refused. Every write was preceded by a redundant re-sync. Four claim refreshes per run protected nothing. `evidence.verify` failed in most runs.

What changed:

- `docket context implementation --id <id>` now returns the full bundle for an `in-progress` change. The bundle reports `claim_eligible: false`, the recorded feature branch, and the halt marker. Other non-proposed statuses still refuse, and selection without `--id` is unchanged (ADR-0158).
- The `docket change refresh-claim` command is deleted. Claim, reconcile, and resume-halted still stamp `claimed_at` (ADR-0159).
- `docket schema <id>` now works as a positional form of `--operation <id>`. A disagreeing pair refuses.
- `repository.prepare` no longer returns the always-empty `skills` field.
- The implement-next, convention, and docket-adr skills were changed:
  - Before each post-claim write, the run reads the record's revision from the local metadata checkout. On `contended` it re-syncs once and retries.
  - Re-syncs now happen only at startup, after dispatched writers, and after a contended write.
  - The skill gives a minimal "nothing changed" reconcile request.
  - The `--evidence` file is the `evidence.record` receipt's `block` text, and the standalone verify call is gone.
  - docket-adr receives the producing change, so the ADR number lands in `adrs:` in the same commit.
  - The review-size bump diffs against the change's own effective base.
  - PR-body assembly moved into Step 7.

There were no material departures from the spec. One sentence was added beyond the plan: the convention's dispatch-fallback `halt` row no longer says `claimed_at` is refreshed.

## Human actions and testing

### Optional — confirm the bookkeeping fixes on the next few runs

This change targets agent behaviour, so automated tests cannot show the effect on real runs.

After merging, reinstall, and restart sessions:

1. Run implement-next on a build-ready change.
   Expected: the first `change.reconcile` is applied with no `contended` result, and no `context.implementation` call is refused for the claimed change.
2. Count `repository prepare` calls in the transcript.
   Expected: about two, with no `change refresh-claim` or `evidence verify` calls.

## Verification performed

- The full suite (`go run ./cmd/docket development test`) passed through the build gate at the build head 6df90cba8 and again at the post-fix head 21d18b60f. BUDGET WATCH lines were first-strike parallel-overrun screening notes (streak 1/5), and no breaches were reported.
- Each task worker ran focused RED/GREEN tests. Mutation checks covered the in-progress context path, the schema mismatch refusal, the refresh-claim retirement test, the board-unchanged reconcile integration test, and the new skill prose-contract rows.
- Review: docket-review-standard ran over the whole branch. It returned two minor findings, and both were fixed in-branch (21d18b60f).
- The backlog check ran over 19 proposed and deferred changes.

## Known issues and follow-ups

- **`docket adr record` without `--json` refuses with `invalid-input` and prints no detail.** While recording ADR-0158, the docket-adr agent's first human-mode call refused with no explanation. The identical request with `--json` applied. Suspected, not reproduced. Impact: an agent or human sees a bare refusal and cannot tell what is wrong. Workaround: pass `--json`. Suggested next action: reproduce, and make human output name the finding. Verdict: No existing change fits (checked 19). A human can capture it as a new change.
- **Related work:** #575 (Retire claim reclaim and its lease) builds on ADR-0159. Once reclaim is gone, `claimed_at` has no reader. This is not a follow-up from this change. It is noted so #575's builder sees ADR-0159.
