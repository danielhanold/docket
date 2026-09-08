<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0408 — Finalize publish is denied by the auto-mode classifier whenever the gate rebases](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-08-0408-finalize-publish-is-denied-by-the-auto-mode-classifier-whene.md)**
<!-- docket:backlink:end -->

# Change 0408 — Preserve a still-valid green gate across a denied finalize publish

## Decision and scope

The investigation-first framing of this change is retired. The controlled comparison of Go and direct-Git publication across Claude Code 2.1.259, 2.1.260, and current — the twelve-cell matrix and the live-classifier acceptance activity — is dropped. Direct evidence gathered on 2026-09-07 makes that experiment moot:

- The five most recent finalizes — 0379 (PR #282), 0383 (#281), 0388 (#284), 0406 (#283), and 0407 (#285), all landed 2026-09-07 on Claude Code 2.1.263 within a ~1.5h window — each performed a real non-fast-forward rebase (recorded branch head is not an ancestor of the commit that landed; the landed commit is single-parent with a tree identical to the head: a rebase-onto-moved-base then fast-forward, not a squash). Every one published its rewritten head with no denial and no human-typed command.
- Recovered Go results already recorded successful rewrite-publishes for 0403 (2.1.259), 0384 (2.1.252), and 0364 (2.1.258); 0403's pre-rebase head is not an ancestor of its published head, so that too was an actual rewrite.
- The one observed denial was change 0404 on 2.1.260 — two versions behind current. A version-specific classifier behavior that does not reproduce on the installed version does not justify resurrecting isolated historical binaries and an approved live-force-push target to characterize it.

What remains is the product objective the original spec always named and never built: honest recovery after a denied publish that preserves a still-valid green gate. This change builds exactly that. No permission grant, no split publisher, no new recovery subsystem, no change to Claude Code, branch protection, merge method, or bot approval.

## The gap this change closes

A finalize that performs a real rebase runs the local gate on the rebased head, records green evidence for that head, updates the PR body, and then publishes the rebased head with a force-with-lease push (`FinalizePublish` → `internal/workspace/rewrite.go` `PublishRewrite`). If that publish is denied — most sharply by a host permission classifier, so the Go binary never runs and emits no result — the run stops with the branch rebased and the gate green, but nothing publish-specific persisted.

On the next finalize invocation the resume path re-enters the rebase seam. With no rebase in progress and the local head descending the base, `recoverFromReceipt` (`internal/app/finalize_rebase.go:592`) reaches `noop := string(localHead) == rec.OrigHead` (`:639`). After a real rebase the local head is the rewritten head, not the receipt's `OrigHead`, so `noop` is **false**, and `composeLocalGate(..., noop=false)` re-runs the full suite even though a valid green gate for exactly this head already passed. If the base moved in the interim the resume is instead blocked as a moved base (`:602-605`) — correct, but still discarding the completed evidence.

ADR-0105 persists the gate's **live** WAITING continuation (drive id + owner generation) in the owned receipt so a multi-slice gate resumes with the identical `finalize.rebase` invocation. It does not persist **completed** evidence, so it does not cover this case. ADR-0105's own consequence note ("re-entering `composeLocalGate` after a completed real rebase may start another suite") is the residue this change removes.

This is latent on 2.1.263 only because publishes are not being denied there. Any denied publish after a real rebase wastes a passed suite; the whole point of a durable green gate is that it survives exactly this interruption.

## What changes

Persist a **completed-gate publish checkpoint** in the owned rebase receipt when the local gate reaches PASSED for the rebased head, and reuse it to skip the suite on a valid resume:

- **Record.** When `composeLocalGate` maps a PASSED terminal for a real rebase, write into the receipt a checkpoint capturing the tested head OID, the effective base head, the byte-exact resolved `finalize.test_command`, the gate policy identity, the repo/change/PR identity already in the receipt, and the green evidence (block bytes or a stable digest sufficient to re-emit or re-verify the managed PR-body block). The checkpoint is written in the same discipline ADR-0105 uses for the continuation pair: written on the terminal that establishes it, cleared on finalize closeout and whenever it is invalidated, so a stale checkpoint never wedges a receipt. The receipt stays `==`-comparable.
- **Reuse.** On a finalize resume where no rebase is in progress and the local head descends the base, if a checkpoint is present and the tested head, effective base head, resolved test command, gate policy, and repo/change/PR identity all still match current reality, skip `composeLocalGate` entirely — do not invoke the suite — reuse the recorded evidence to converge the PR body, and proceed to `FinalizePublish`.
- **Invalidate.** A moved head, a moved base, a changed resolved test command, or a changed gate policy invalidates the checkpoint; the gate re-runs exactly as today. Reuse applies only to still-valid evidence — "never rerun a green gate" governs valid evidence, never stale evidence. The receipt's identity and moved-base refusals (`:596-605`) remain in force ahead of any reuse.

Preserve unchanged: `PublishRewrite`'s exact ref-and-old-OID lease and its response-loss behavior (already at the intended head is a no-op; a changed remote is contention; an unobservable remote grants no permission to force or repeat), and `FinalizePublish`'s preservation of authored PR-body bytes outside the managed evidence block.

Host denial versus Go result stays a **skill-contract** concern, not a new Go primitive: a host permission denial prevents the docket binary from running at all, so there is no Go result to inspect — the finalize skill reports the specific denied action and uses only the permitted exact retry, and the durable checkpoint is what makes that retry cheap (no suite re-run) and safe. Persistent failure follows the existing Finalize blocked mechanism with a valid resume instruction; if recording the block is itself denied, report that separately rather than pretend the marker exists (the comment-first discipline in `internal/app/finalize_block.go` already does this).

## Tests and acceptance

Behavioral tests, each asserting observable outcomes rather than internals:

- A denied publish after a real rebase, then a resume, reuses the recorded evidence, converges the PR body, and publishes **without invoking the suite** — assert the suite is not invoked, and mutation-test that guard (remove the checkpoint-reuse branch and watch the assertion redden).
- A moved head, a moved base, and a changed resolved test command each invalidate the checkpoint and re-run the gate.
- A competing remote update surfaces as contention, not a forced overwrite; a push that succeeds but whose response is lost recovers to the promised head as a no-op.
- Authored PR-body bytes outside the managed evidence block survive the reuse path.
- The full build and finalize suites run from resolved configuration at their gates, and any authoritative runtime-budget breach is handled per AGENTS.md.

The completed-evidence publish checkpoint is a new architectural decision that extends ADR-0105 (live continuation) and refines ADR-0098 (owner-held continuation secret; the receipt is finalize's caller). Record it as a new ADR relating to 0105 and 0098.

## Related work and exclusions

Related: 0100 (a pre-Go force-with-lease denial), 0260 (denial posture), 0316 (the Go publisher), 0360 (evidence/coordination concerns), 0396 (durable running-gate continuation, ADR-0105), 0403 (successful counterexample), and 0404 (the single observed 2.1.260 denial). None is an unmet implementation dependency. ADR-0043 records the merge-side zero-approval policy and does not decide this publish-side behavior.

Out of scope: the historical-binary comparison and the live classifier acceptance activity (retired above); any change to Claude Code, branch protection, merge method, or bot approvals; any broad permission grant or user-settings change; a Go primitive that distinguishes a host denial from a Go result (a host denial means the binary never ran — the distinction lives in the skill and harness, not in Go); a split publisher or a general recovery subsystem beyond the receipt checkpoint described here. No production Git rewrite, permission edit, or implementation was performed during this rescope.
