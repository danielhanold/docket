<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0515 — Retire the finalize repair sign-off so a green repair merges](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0515-make-finalize-merge-honor-the-repair-sign-off-block-when-an.md)**
<!-- docket:backlink:end -->

# Retire the finalize repair sign-off: a repair that turns the suite green merges

## Problem

When finalize rebases a change and the suite goes red, `docket-integration-repair` writes a
minimal fix and finalize re-runs the suite. When the fix turns the suite green, the finalize skill
still does not merge:

- An **autonomous** run records `finalize.block --reason repair-needs-signoff`, publishes the repaired
  head, and halts. The human then has to review the repair, run `finalize.clear-block`, and run
  finalize again (0444 hit exactly this).
- An **attended** run publishes the repaired head and prompts for a go-ahead before merging.

This is ADR-0010's sign-off rule. It argues that "the human's PR approval predated the repair". In the
setup docket recommends, though, no human approves the PR at all: branch protection requires zero
approvals (ADR-0043), `finalize.require_pr_approval` defaults to `false` (ADR-0011), and the
original build can merge on green tests and the review agent's pass without anyone having to read the
code. So
a small, test-preserving repair has to wait for a human when the much larger change it repairs did
not. The rule adds a stop and buys little. Teams that do want a repair re-reviewed already have
GitHub's own mechanism for it (see *Repos that require approvals* below).

The rule is also enforced only by the skill text. The binary never refuses:

- `finalize.merge` folds the marker into `NotSuperseded` as `in.explicitID || !in.finalizeBlocked`
  (`mergeConditions`, `internal/app/finalize_merge.go`), and the CLI
  (`newFinalizeMergeSubcommand`, `internal/cli/finalize.go`) always sends `ExplicitID: true`, so
  that term is always true.
- `finalizeBlockedMap()` (`internal/app/finalize_context.go`) is a placeholder that always returns
  an empty map, so no finalize selection ever skips a marked change. Its comment invites a later task
  to "wire marker reading in one place".

The docs claim more than either: that auto-detect skips every change carrying `## Finalize blocked`,
and that a named id never overrides the sign-off.

## Goal

Finalize adds no human stop of its own beyond what the repository's approval policy asks for. A
repair that turns the rebased suite green publishes and merges like any other green change, and the
repair is recorded where a human can see it later. `## Finalize blocked` stays a visible note that
never stops selection or merge. The next run retries the change, as it does today.

## Design

### 1. The finalize skill merges a green repair

`skills/docket-finalize-change/SKILL.md`:

- **Step 6** ("Sign-off on an authored repair") is replaced. After a repair, the re-gate is `PASSED`
  and its evidence recorded, so the run continues to step 7 (publish) and step 8 (merge) on both
  autonomous and attended runs. There is no `finalize.block`, no prompt, and no `clear-block`.
- **Visibility, never a stop.** The run's final report names the repair: what broke, the claimed
  repair commits, and the attempts used. Step 9 passes the same facts to `finalize.closeout` as a
  `late_findings` entry, so the archived record's `## Closeout notes` keeps them. This extends step 9's
  "records only the context supplied at invocation" to include the run's own repair. If closeout
  refuses that notes request for any reason, the run retries closeout without notes. A lost note never
  stops the closeout.
- **Step 7:** remove the attended-repair `finalize.clear-block` sentence.
- **Step 8:** remove "or the repair sign-off" from what a named id never overrides, and remove the
  `finalize-blocked` override (see §3).
- *Selection*, *Terminal disposition*, and step 1: a named id or allowlist overrides only
  `approval-required`. Drop "a `## Finalize blocked` marker on the auto-detect path" from the
  blocked-but-non-empty paragraph.
- *Sign-off, abort, and the blocked marker* (the pointer section): drop "the sign-off rule".

`skills/docket-finalize-change/references/gate-failure.md`:

- Replace *Sign-off on auto-authored repairs* with a short section: a repair that greens the suite
  merges, recorded in the report and the closeout notes. It also states the approval rule below.
- Remove the abort-and-report item "an authored repair under autonomous finalize".
- In the two-agents section, drop "fires the sign-off rule below".
- Marker lifecycle: the section is a visible note. It never stops selection or merge, and the next
  run retries the change. Remove the auto-detect skip, the named-id override, and the
  `repair-needs-signoff` clause from the `clear-block` bullet. `clear-block` stays as a manual way to
  remove a note, and closeout still strips it.

`agents/docket-integration-repair.md`: the sequencer re-gates the repaired head and merges when it is
green. It no longer prompts or records a `repair-needs-signoff` marker. Regenerate the harness goldens
(`internal/harness/*/testdata/golden/docket-integration-repair.*`) and the embedded bundle
(`internal/assets/embedded/tree`).

### 2. Repos that require approvals

Docket keeps no re-approval of its own. Approval is the repository's policy:

- When branch protection requires approvals **and** has GitHub's "Dismiss stale pull request
  approvals when new commits are pushed" turned on, publishing the repair dismisses the PR's
  approval. The PR needs a fresh human approval before it can merge: GitHub refuses the merge
  (`halted`), and with `finalize.require_pr_approval: true` auto-detect skips the PR as
  `approval-required` until someone approves it. A human reviews the repair because the repository
  requires it, not because docket does.
- That GitHub setting is off by default. With it off, the earlier approval stands and the repair
  merges like any other green change. A team that wants repairs re-reviewed turns it on.
- The single-maintainer setup (zero required approvals, `require_pr_approval: false`) never stops for
  a repair.

The ADR (§4) and the docs (§5) say this in these terms.

### 3. Remove the binary's unused "blocked note" machinery

These lines never block anything today, but they are a ready-made hook for a future stop. Removing
them makes "a note never stops finalize" a property of the code, not just of its current wiring:

- Delete `finalizeBlockedMap()`. `domain.SelectFinalizeQueue` drops its `blocked` parameter and the
  `skipFinalizeBlocked` token. Both callers (`context.finalize`, the sweep in `maintenance.go`) stop
  passing it.
- `overridableSkip` keeps only `approval-required`.
- `mergeConditions`: `NotSuperseded` becomes just `in.revisionMatches`.
  `mergeConditionInputs.finalizeBlocked` and `changeHasFinalizeBlockedMarker` go, plus
  `headingText` if nothing else uses it. The `superseded` message stops mentioning a marker. The
  header comment's "never … the repair sign-off (gate)" goes too.
- `finalize.block`, `finalize.clear-block`, `HasFinalizeBlocked`, and the board's
  "finalize blocked — needs you" cell are unchanged. Other abort points still record notes, which
  stay visible.

### 4. ADR

Record a new ADR: *finalize adds no human gate of its own.*

- A repair that turns the rebased suite green merges. This reverses ADR-0010's sign-off rule.
  ADR-0010 stays Accepted (its resolver/repair split stands) and gets a dated `## Update` pointing to
  the new ADR. The same goes for the sign-off mention in ADR-0008's update.
- When a repository requires PR approvals and dismisses stale approvals on new commits, the repair
  push removes the approval, so the merge waits for a fresh approval. Human review of repairs is the
  repository's policy, set in GitHub, not a docket gate.
- `## Finalize blocked` is a visible note that never stops selection or merge. The documented "auto-detect
  skips marked changes" rule is dropped rather than implemented, because skipping would end the
  retries that let transient failures (flaky tests, a busy worktree, a moved base) heal on their own.
- Relates to ADR-0010, ADR-0011, ADR-0043.

### 5. Docs describe what finalize does

Remove or rewrite every sign-off passage, and every claim that notes are skipped:

- `README.md` (the human checkpoints list: "unattended repair … blocks for your sign-off").
- `docs/concepts/finalize-sequencer.md` (the repair bullet, and "always waits for a human's
  sign-off").
- `docs/guide/landing-changes.md`: *When finalize is blocked* (no skip; the next run retries; closeout
  removes the note after merge, not "a successful finalize clears it automatically"), the "One block
  always waits for you" paragraph (removed), and *Repos that require approvals*, which gains the
  dismiss-stale-approvals sentence from §2.
- `docs/guide/proving-the-build.md` ("gates the merge behind sign-off").
- `docs/reference/glossary.md`: remove the *Repair sign-off (`repair-needs-signoff`)* entry and its
  index line. Fix *Finalize blocked / reason token / clear-block* (visible only, retried) and the
  explicit-id entries that list `finalize-blocked` skips.

Docs state the current behavior only, with no change or PR citations (ADR links are fine).

## Where this change can stop a run

It removes a stop and adds none:

- **Removed:** the halt after every authored repair on autonomous runs, and the prompt on attended
  runs.
- **Binary:** only never-taken branches are deleted. No condition, skip reason, or refusal is added.
- **Closeout note:** written in the closeout's own request. A refusal falls back to a closeout without
  notes.
- **Approval-requiring repos:** they stop only where the repository's own branch rule says to (§2).
  Docket adds nothing there.
- **Work in flight:** no active change carries a `## Finalize blocked` section (checked 2026-10-04).

## Tests

- **Selection retries a noted change.** A change carrying `## Finalize blocked` (any reason, including
  `repair-needs-signoff` text) is an actionable auto-detect candidate, not skipped. Mutation:
  reintroduce a skip on `HasFinalizeBlocked`, and the test fails.
- **Merge ignores a note.** `finalize.merge` merges a change whose record carries the section, with
  every other condition holding. Mutation: reintroduce the marker term with `ExplicitID: false`
  reaching it, and the test fails.
- **End to end.** Rewrite `TestE2EConflictAndRepair` (`internal/app/finalize_e2e_test.go`): conflict →
  resolver → red suite → repair → re-gate green → evidence → publish → merge → closeout, with no
  `finalize.block` or `clear-block`. Block and clear-block keep their own focused tests.
- **Prose guards.** Replace the `align_0502_signoff` sentinels in
  `internal/repoguard/prose_contracts_test.go` with a sentinel pinning the new rule (present: the
  green-repair-merges sentence in both finalize files; absent: `repair-needs-signoff` in skills and
  agents). Mutation: restore the old step 6 text, and the test fails. Adjust the line budgets in
  `internal/repoguard/budgets_test.go`.
- Replace the leftover `repair-needs-signoff` argument in `internal/cli/revision_rename_test.go` with
  a neutral reason token, so the retired token is gone from maintained source.
- Run the whole suite at the gate and read its budget report.

## Out of scope

- `finalize.block` / `finalize.clear-block` themselves, and the other abort-and-report points.
- The integration-repair ladder and its budget.
- Approval semantics (`require_pr_approval`, explicit id, `--admin`). The allowlist override-note gap
  (the skill says an allowlist overrides `approval-required`; the binary gives allowlist members no
  override note) is separate.
- The board's "finalize blocked — needs you" wording.

## Rollout note

This edits an agent file (`docket-integration-repair`) and the finalize skill. After the install,
restart running sessions before the next finalize, so that no run mixes the old and new contracts.

## Related work

0517 (finalize re-test evidence) and 0520 (finalize request schemas) are in progress and edit
`skills/docket-finalize-change/SKILL.md` and nearby `internal/app/finalize_*` files, in different
paragraphs and functions. Expect text conflicts at rebase, not design conflicts.
