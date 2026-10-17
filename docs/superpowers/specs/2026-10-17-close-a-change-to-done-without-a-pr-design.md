# Let a change deliver a results artifact instead of a PR — design

Change 0559 — Let a change deliver a results artifact instead of a PR

Change #559, groomed 2026-10-17.

## Problem

Some changes deliver no code. Research, measurements, and preparation work for a larger epic produce findings, recorded in the change's results artifact, and nothing to merge. #547 is the current example: it measured whether the 1-hour cache TTL cut implement-next's cost, attached a results artifact confirming it, and has no PR to open.

Docket has no way to finish such a change. A change reaches a final status only by finalize after a merged PR (`done`) or by `change.kill` (`killed`, which reads as abandoned). Three earlier changes worked around this: #124 and #545 were killed although they were finished, and #423 was hand-edited to `done`. A results-artifact change also cannot be built by the loop today: implement-next always goes plan → build → review → PR → implemented, and the run tracker counts a run as successful only when the change is implemented with a PR and build evidence.

## Decisions

### 1. A `deliverable` field on every change

- New frontmatter key `deliverable` with a closed vocabulary (`deliverables` in `docket schema`): **`pr`** (default — the change ends in a merged PR) and **`results-artifact`** (the change ends when its results artifact is attached and the change is completed). The value is hyphenated like every other docket value; the key is permanent (ADR-0129: frontmatter fields are never renamed).
- An absent or empty `deliverable:` reads as `pr`. No existing record is rewritten.
- `change.create` takes an optional `deliverable` (empty means `pr`) and writes the key explicitly on every new record, the way it writes `priority:`. Place it directly after `type:`.
- An unknown value is refused with `unknown-deliverable` (create and groom); a record carrying one is a decode error finding, handled the way an unknown `priority:` is.
- `docket status --json` and `context.implementation` report each change's `deliverable`. The board does not show it.
- A `results-artifact` change's branch is never published or merged, so it takes no part in stacking: `change.create` and `change.groom` refuse `stacked_on` on a `results-artifact` change, and refuse `stacked_on` naming a `results-artifact` parent. Ordering between changes uses `depends_on`.
- Build-readiness is unchanged and applies to both deliverables: `proposed`, a spec or `trivial: true`, and every `depends_on` done. A ready `results-artifact` change is selected and claimed like any other.

### 2. `change.groom` can edit `deliverable`, `type`, and `branch_prefix`

Today `type` and `branch_prefix` can be set only at creation, and nothing could edit `deliverable`. `ChangeGroomRequest` gains three optional fields, modelled on the existing `title` and `priority` fields (#461, #550):

- `deliverable` — `enum=deliverables`; empty leaves it unchanged.
- `type` — validated against the resolved `change_types` exactly as `change.create` does (`unknown-type`); empty leaves it unchanged.
- `branch_prefix` — three states: absent leaves it unchanged, an empty string clears it, a value sets it after `domain.NormalizeBranchPrefix` (`invalid-branch-prefix` on rejection, as in create).

Rules, identical to `priority`: the spec, trivial, revise, and re-enable outcomes accept each field; abstain refuses each; any one of them alone is a valid revise (extend the `empty-revise` minimum-input rule and its message). `change.groom` already works only on `proposed` changes, so a claimed change cannot change its deliverable mid-run. Nothing is renamed: slug, record path, spec path, and branch stay put. `branch:` is not minted until claim, so the next claim builds the branch name from the new `type`/`branch_prefix`.

### 3. `docket change complete`

A new typed operation that moves a `results-artifact` change to `done` and archives it, with no PR.

- Catalog id `change.complete`, CLI `docket change complete --request <file> [--repo-dir <dir>]`, effect `metadata-write`. Request `{change_id, path, revision}`, all required (the caller pins the record from `status --json`, as for kill). Result `{id, disposition, archive_path, committed_revision, findings}` plus the envelope. `disposition` comes from a new closed vocabulary, `complete_dispositions` (`completed`, `already-completed`, `contended`, `failed`), registered in `docket schema` and the glossary like the other disposition vocabularies; refusals carry their own reasons. `already-completed` follows the `already-<what happened>` house pattern (`already-claimed`, `already-cancelled`, `already-merged`, `already-clean`), not closeout's bare `already`.
- **Preconditions** (a refusal writes nothing):
  - `deliverable` is `results-artifact`; a `pr` change is refused (it closes through finalize).
  - Status is `proposed` or `in-progress` (`illegal-source-status` otherwise; a blocked change is unblocked first, a deferred one revived).
  - A `proposed` change must be build-ready by the existing readiness rule (spec or trivial, every `depends_on` done); refuse with the readiness reason. An `in-progress` change was checked at claim and is not re-checked.
  - `pr:` is empty.
  - The results artifact is attached and valid: `results:` set, the file on the metadata tip carrying this change's backlink, and the FINAL content contract (`ValidateResultsContent`, `ResultsPhaseFinal`). Reuse the results verification `change.mark-implemented` already does (`verifyImplementedResults`), generalised so both operations share it, rather than copying it.
- **Effects**, in one metadata commit (subject `change NNNN completed (done, archived)`): status `done`; `claimed_at:` cleared (an archived record may not hold a claim); `branch:` **kept**, as a done `pr` change keeps it, because cleanup deletes a local branch only by its recorded name (a change completed straight from `proposed` was never claimed and has none); the `## Run halted` section removed if present (`run verify` reads it first and would otherwise report a finished change as halted); `updated:` set to the archive date; the record moved to `archive/<YYYY-MM-DD>-NNNN-<slug>.md`, dated by the transaction clock in UTC and re-read on each retry (the kill rule); the `## Artifacts` block re-rendered; spec/plan/results backlinks retargeted; the inline board re-rendered; pushed under the exact-lease push.
- **Scope:** validate with the scoped gate `finalize.closeout` uses (`changeScope`), not kill's whole-corpus gate, so an unrelated record's defect cannot block it.
- **Retries:** a `complete` on a change that is already `done` with `deliverable: results-artifact` returns `already-completed` and writes nothing, so a run whose response was lost can re-run it safely.
- **Metadata only:** it reads and changes no git branches, worktrees, GitHub, or run-tracker state. Removing the worktree and local branch is `finalize.cleanup`'s job (section 4), run after it, the way finalize runs close-out and then cleanup.
- The kill operation (`change_kill.go`) is the closest template: a single-target archive transaction over the same pipeline.

`change.mark-implemented` refuses a `results-artifact` change, so each deliverable has exactly one legal ending.

### 4. What a done change without a PR needs elsewhere

- **Artifacts links.** `legacyArtifactBranch` (`internal/render/artifacts.go`) treats every `done` change without a `## Build evidence` section as legacy and points its Plan/Results rows at the integration branch, where a results artifact never lives. A `results-artifact` change is never legacy: its rows link to the metadata branch. This must hold in shared repositories (relative or `docket`-branch links) and private ones (no GitHub link to `main`). It refines ADR-0141's legacy-row rule; record that in this change's ADR.
- **`finalize.cleanup` gains a no-PR path** for a `done` `results-artifact` change. It skips the merged-PR probe and the merge-destination check, then:
  - removes the feature worktree through the existing `workspace.Cleanup`, which already refuses a worktree with uncommitted, staged, or untracked files (cleanup `pending`, nothing removed);
  - deletes the local feature branch by its recorded name only when the branch's tip is contained in `origin/<integration_branch>` — that is, the branch has no commits of its own; otherwise the branch is kept and cleanup reports `pending` with the existing `tip-not-in-merge-chain` reason;
  - never touches a remote ref: this branch is never published;
  - with no recorded branch (completed from `proposed`), or with neither the worktree nor the local branch present on this machine, reports `already-clean`.
  Every existing fail-closed rule (probe errors, moved refs, the tombstone for replays) applies unchanged.
- **Sweep.** A `done` `results-artifact` change enters the full-scope cleanup worklist like any done change and is cleaned through that path, so the sweep is the safety net for a leftover worktree. The cleanup assessment (`internal/app/maintenance_assess.go`) must not report it `skipped` / `snapshot-invalid` for lacking a PR. Archived #423 has no `deliverable` field, reads as `pr`, and keeps its current entry.
- **Board archive table.** The date column's header changes from "Merged" to "Closed": it was already wrong for killed rows and would be wrong for completed ones. Rows are otherwise unchanged; a completed change looks like any other archived row.

### 5. implement-next's results-artifact ending

Selection, claim, reconcile, and workspace preparation (`workspace.prepare`) are unchanged: a `results-artifact` run gets its feature worktree exactly as a `pr` run does. After that it takes a separate ending. Put it in a new `skills/docket-implement-next/references/results-artifact.md`, reached by a one-paragraph pointer in `SKILL.md`, so the common `pr` path's prompt does not grow.

1. **No plan, no build, no PR.** Skip `workspace.commit-spec` (the branch must keep no commits of its own), Steps 4, 5, and 7, and Step 6's evidence checks and diff review. No plan file, plan-writer dispatch, build gate, publish, or PR. The spec (or a trivial change's body) is the plan.
2. **Do the work** the spec describes, in the feature worktree. Never commit to the branch, never publish it, and never modify the primary checkout. Anything worth keeping goes into the results artifact: the findings, the numbers, and the commands or short scripts that produced them, inline, so a reader can re-run them. Delete everything else from the worktree before cleanup. If the work turns out to need a code change, or something must be kept as a file the results artifact cannot carry (a long script, a large data file), halt (`change.halt`) and say so: a person decides whether it becomes a follow-up change with a PR.
3. **Human steps.** When the change or spec says a step needs a human, or the run finds that one does, the run does the work it can and halts (`change.halt`) when it reaches that step, naming the step in the halt report. Halting at the step, not earlier, leaves the work so far in the worktree for the person. In an interactive session the dispatching session then works through it with the human; a driver stops on the `halted` disposition, as today. A person finishes the change by doing the steps, attaching the results artifact, and running `docket change complete` (section 7).
4. **Write and attach the results artifact** under Step 6.5's existing rules.
5. **Review.** Dispatch `docket-review-standard` (the existing default tier when there is no build record) inside the existing `docket:feature-dispatch` block, naming the feature worktree as its worktree, so the dispatch repoguard (`internal/repoguard/feature_worktree_dispatch_test.go`, which scans every `skills/**/*.md`) and Codex's `agent.enter` worktree requirement hold unchanged. The reviewer touches no docket metadata, so the payload carries the spec body (or the trivial change's body) and the results artifact's text inline, plus the change's title and scope and the relevant learnings hooks. The `docket-review` skill gains a short results-artifact mode: no evidence verification and no branch diff; it checks that the results answer what the spec asked, that claims and numbers trace to cited sources or commands a reader could re-run, that the `**Human action:**` line is accurate, and that follow-ups are named. Same return schema and severity levels.
6. **Fix pass.** The run revises the results artifact itself for every finding at or above `review.min_fix_severity` (blockers always), capped by `review.max_fix_tasks`, and re-attaches it. No fix workers, no suite gate, no second review. Findings below the threshold go into the results artifact's known issues; distinct follow-up work goes into the final report, as today.
7. **Complete, then clean up.** Run `docket change complete`, then `finalize.cleanup` with the change id. A cleanup that reports `pending` (files left in the worktree, commits on the branch) goes into the run report; it never halts the run and never undoes the completion. The run ends with the `advanced` disposition, as a run that opens a PR does, so drivers treat both alike. Update Step postconditions and the terminal disposition table: `advanced` means a PR opened **or** a `results-artifact` change completed (done and archived, with a valid results artifact attached).

The run refreshes its claim at phase boundaries as today. Reclaim is unchanged: a run that halts or is abandoned after preparing its worktree leaves a local branch and worktree, which `change.reclaim` already refuses to reclaim (`branch-still-present`, `workspace-active`); a run that stops before the worktree exists leaves a change reclaimable once its lease expires, as for a `pr` change (ADR-0059).

### 6. Run tracker

`run verify` gains the `results-artifact` postconditions; `pr` changes are unchanged:

- `proposed` → `run-unclaimed` and `## Run halted` → `run-halted`, as today.
- `done` with a linked, present, FINAL-valid results artifact → `run-complete`. Cleanup's outcome does not enter the verdict.
- `in-progress` → `run-incomplete` with a new reason `not-completed`, plus the existing results reasons when they apply.

`run.verdict` consumes these as it does today, so a completed run reads `run-done` and drives the successful-run closeout (ADR-0148). Confirm no verdict path assumes a PR or a gate drive for this deliverable.

### 7. A person completing a change

Anyone can run `docket change complete` on a build-ready `proposed` `results-artifact` change, or on an `in-progress` one (for example after a halt), and then `finalize.cleanup` to remove its worktree (or leave it to the sweep). A person's completion has no automatic review: the person is the reviewer. "Attended" is not a field: a change that needs a human names those steps in its body or spec.

### 8. Docs, skills, and the ADR

- `docket-convention` (`SKILL.md`): the manifest gains `deliverable:`; the lifecycle diagram and status table say `done` means a merged PR **or** a completed results artifact; the archive-date sentence becomes "the merge date for a merged change, and the transaction date for a completed results artifact and for a killed change"; name `change.complete`.
- `docket-convention/references/close-out.md`: add `change.complete` as the third done driver, owning its own transaction as kill does, and `finalize.cleanup`'s no-PR path. Fix two existing errors there: kill's flag is `--request` (not `--input`), and kill is not idempotent (a retry after success is refused).
- `docket-groom-next`: one short paragraph beside Retitle and Reprioritize covering `deliverable`, `type`, and `branch_prefix`.
- `docket-new-change`: one clause — when the work ends in findings, not code, the create request carries `deliverable: results-artifact`.
- `docket-implement-next`: the pointer, the new reference file, and the `advanced` row of the terminal disposition table (section 5).
- `docket-review`: the results-artifact mode.
- `change attach-results` help text: drop "an in-progress change"; it attaches to any change.
- User docs: `docs/reference/glossary.md` (define `deliverable` and results artifact; lifecycle table: `done` is reached by finalize close-out, the sweep, or `change complete`), `docs/concepts/change-lifecycle.md`, and the guides that define `advanced` as "opened a pull request" (`docs/guide/building-without-supervision.md` and others). Find every site that equates `done` with a merged PR, or `advanced` with a PR, by a whole-repo grep, not from this list.
- Run `go generate ./internal/assets/` after skill edits.
- Record one ADR through `docket-adr`: a change's deliverable is a PR or a results artifact; `done` no longer implies a merge; a completed change's artifacts are never legacy (refines ADR-0141); and a no-op replay result names what already happened (`already-<what happened>`), never a bare `already` (section 9).

The capability catalog is about 15.9 KB against the 16 KB cap in `TestCapabilitiesPayloadWithinByteBudget`; one new entry fits. Do not raise the cap to make room.

### 9. Every "already" result names what already happened

`change complete` returns `already-completed` (section 3). Three existing operations still return a bare `already`, the value this spec first copied. Rename each to the `already-<what happened>` pattern the rest of docket uses (`already-claimed`, `already-cancelled`, `already-merged`, `already-clean`, `already-current`):

| Where | Today | New | Meaning |
|---|---|---|---|
| `finalize closeout` disposition (`closeout_dispositions`, `CloseoutDispAlready`) | `already` | `already-closed-out` | the change is already done and archived, or already stacked-merged |
| `finalize block` disposition (`block_dispositions`, `BlockDispAlready`) | `already` | `already-recorded` | the section already records this attempt; pairs with `recorded` |
| `finalize retarget-children` per-child `outcome` (`childOutcomeAlready`) | `already` | `already-retargeted` | the child's PR already targets the new base; pairs with `retargeted` |

Four internal outcome values never reach output but carry the same bare string. Rename them too, so nothing is left to copy: `githubcli.BodyAlready` → `already-edited`, `githubcli.CommentAlready` → `already-posted`, `githubcli.RetargetAlready` → `already-retargeted`, and the PR-backlink repoint's `prBacklinkAlready` → `already-repointed`. Go constant names may stay.

Only the spelling changes; behavior does not. Nothing persists these values (they are read in-process or by skills, and the run tracker stores none of them), and skills ship with the binary, so there is no compatibility alias. Update every reader: `skills/docket-finalize-change/SKILL.md` (closeout routing), `skills/docket-convention/references/stacked-changes.md` (the root-archived replay), the `block_dispositions` and `closeout_dispositions` rows in `docs/reference/glossary.md`, and the tests that assert the old value. Find them with a whole-repo grep for the bare value, not from this list. Point-in-time records (archived changes, specs, plans, results) keep the old word. The new ADR (section 8) records the naming rule.

## Tests

Follow the existing groom, kill, closeout, and cleanup test structure. Run the whole suite at the build gate.

1. **Field.** Absent/empty `deliverable` reads as `pr`; create writes it explicitly; an unknown value is refused on create and groom; `results-artifact` with `stacked_on`, and `stacked_on` naming a `results-artifact` parent, are refused.
2. **Groom edits.** Each of `deliverable`, `type`, `branch_prefix` lands on spec, trivial, revise, and re-enable; each alone is a valid revise; abstain refuses each; `branch_prefix` absent / empty / value means unchanged / cleared / set; an unconfigured type is refused; a claim after a type or prefix edit mints the new branch name.
3. **Complete — happy paths.** From a build-ready `proposed` change and from an `in-progress` change (with and without `## Run halted`): one commit, archived at the clock date, `claimed_at:` cleared, `branch:` kept, halt section gone, artifacts block and board re-rendered, backlinks retargeted, disposition `completed`; a second call returns `already-completed` and writes nothing.
4. **Complete — refusals,** each writing nothing: `pr` deliverable; blocked/deferred/implemented/final status; needs-grooming; an open dependency; `pr:` set; no results; results file missing or with a foreign backlink; results failing the final contract; revision mismatch.
5. **Mark-implemented** refuses a `results-artifact` change.
6. **Artifacts links.** A completed change's Results row points at the metadata branch in a shared and a private fixture; a legacy `pr` done record still renders as before.
7. **Cleanup and sweep.** For a completed change, `finalize.cleanup` removes a clean worktree and deletes a local branch with no commits of its own, without any PR probe; a dirty worktree is kept (`pending`); a branch with its own commit is kept (`pending`, `tip-not-in-merge-chain`); a remote ref of the same name is untouched; no recorded branch, or neither worktree nor local branch present, reports `already-clean`. The sweep cleans a completed change through this path and reports no `snapshot-invalid` for it.
8. **Board.** The archive header reads "Closed".
9. **Run verify.** Completed → `run-complete` (also when cleanup is pending); in-progress → `run-incomplete` with `not-completed`; halted → `run-halted`; `pr` changes unchanged.
10. **Mutation checks.** Removing the deliverable precondition from complete makes test 4's `pr` case pass wrongly; restoring the legacy rule for completed changes makes test 6 fail; dropping the own-commits check from the no-PR cleanup path makes test 7's own-commit case delete the branch.
11. **Private repository.** Exercise complete and cleanup end to end in a private fixture (metadata on `dckt`, nothing pushed to the shared remote).
12. **Already-values.** Pin the new spellings (`already-completed`, `already-closed-out`, `already-recorded`, `already-retargeted`, and the four internal values) in a spelling test like `TestFinalLifecycleCodeSpellings`, and assert that no vocabulary `docket schema` registers contains a bare `already`. Existing closeout, block, and retarget replay tests assert the new values.

## Out of scope

- The autonomous loop meeting a change that needs a human: a field that keeps a build-ready change away from the loop; whether a loop continues past a halt that concerns only one change, which needs a typed halt reason the driver can read (today every halt stops the driver, and #559 keeps that); and where the loop logic lives (the skill body, or a reference read only when needed). Until then, a person who runs a loop lists change ids and checks that none needs a human. Captured as #560.
- A clean, docket-free export of any artifact `docket open` can open (#561).
- How `pr` changes close, finalize, and kill, apart from renaming finalize's bare `already` results (section 9).
- Reclaim: unchanged (section 5).
- Showing `deliverable` on the board.
- Fixing archived #423, #124, or #545.
- Completing #547 itself: once this ships, a person grooms it (deliverable, plus trivial or a spec) and completes it.
