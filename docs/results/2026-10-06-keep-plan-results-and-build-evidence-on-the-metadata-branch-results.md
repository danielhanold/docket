<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0530 — Keep plan, results, and build evidence on the metadata branch, and ship the spec with the PR](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0530-keep-plan-results-and-build-evidence-on-the-metadata-branch.md)**
<!-- docket:backlink:end -->
# Keep plan, results, and build evidence on the metadata branch, and ship the spec with the PR — Results

**Human action:** Yes. Before installing the binary built from this merge, let every other change that is in progress or implemented finish on the current binary, then run `docket repository repair` once to convert old absolute links.

## Outcome

Build artifacts no longer ride the PR. `change.attach-plan` and `change.attach-results` now take the Markdown (`--markdown`) and write the plan and results files to the `docket` metadata branch at their existing paths, in one metadata commit that also sets `plan:` / `results:` and stamps the backlink. Build evidence lives in a `## Build evidence` section of the change record instead of the PR description. The merge gate, clear-block, the rebase no-op skip, and `run.verify` all read it there. A new `workspace.commit-spec` operation commits a copy of the spec as the feature branch's first commit, so a PR contains the spec copy plus code and nothing else.

Links between artifacts on the same branch are now relative, so archiving no longer breaks them. Old `done` and `killed` records keep their absolute plan and results rows, because those files live on `main` or on the old feature branch. Close-out no longer pushes a backlink commit to `main`, and a guard test fails if any operation pushes the integration branch outside a PR merge. Also retired: `artifact.backlink`, the `Docket-Plan-Path` trailer, and the `finalize.skip_results_only_delta` key, which now draws an "obsolete setting, remove it" warning.

Departures from the spec:

- `pr.publish` writes no evidence anywhere. `change.mark-implemented` already records the same verified evidence in a metadata transaction, and a second write at publish time would change the record revision between the two steps. `pr.publish` still refuses to publish an uncertified head.
- `finalize.publish` with skipped evidence (`build.gate: off`) now records it in the change record. Before, it refused with `body-assembly-failed`.
- The PR description gains a generated block of absolute links to the plan and results on the metadata branch, placed right after the backlink. If an author had moved the backlink lower in the description, it moves back to the top.
- `workspace.commit-spec` builds its commit in a private index and moves the branch with a compare-and-swap. A failed commit leaves the workspace untouched, and the commit date comes from git rather than the injected clock.
- `finalize.cleanup` no longer declares a metadata-write effect; it only edits the PR description and deletes refs.

## Human actions and testing

### Important — Install only after in-flight changes finish

Changes that are in progress or implemented when this merges still expect evidence in the PR description and plan/results on their feature branch. The new binary reads neither. If one is caught mid-flight, `evidence.recertify` writes the record section, and its plan and results can be re-attached with the new `--markdown` operations.

1. Run `docket status` and list changes with status `in-progress` or `implemented`.
   Expected: none, or only changes you are prepared to finish first.
2. Finish (or recertify) each one with the current binary, then install the new binary per AGENTS.md "Rebuild the binary after a merge to main".
   Expected: `docket version` reports the merged commit.

### Important — Convert legacy absolute links once

1. After installing, run `docket repository check`.
   Expected: artifact-link drift findings for records whose `## Artifacts` block still carries absolute same-branch links.
2. Run `docket repository repair` and approve.
   Expected: one metadata commit; re-running `repository check` shows no artifact-link drift, and a second repair changes nothing.

### Optional — Watch one full run end to end

Run the next build-ready change through implement-next and finalize with the new binary.
Expected: the PR diff holds only the spec copy and code; the plan, the results file, and a `## Build evidence` section in the change record exist on `docket`; `main` gains no commit after the merge.

## Verification performed

- Full suite (`go run ./cmd/docket development test`) green at the build-gate head; the final head is certified again before the PR opens. The budget report showed only `PARALLEL-SENSITIVE` screening lines (finalize e2e, app-merge integration, race), no serial-confirmed breach.
- Each task added focused tests, and the new gates (record-evidence readers, the integration-push guard, the attach replay key, the legacy link rule) were mutation-tested: removing the guarded behavior turned the test red.
- Whole-branch review (deep tier): 5 findings (2 important, 3 minor), all fixed in-branch.

## Known issues and follow-ups

### Old absolute backlinks inside metadata spec files are not converted

`repository repair` converts `## Artifacts` blocks, not the backlink block at the top of spec files already on the `docket` branch. Those keep working as links, but code that checks whether an artifact "points home" no longer recognizes the absolute form. A plan or results re-attach over such a file, or a mark-implemented on results whose backlink was never re-stamped, could refuse. Neither should happen after cutover, because attach, groom, kill, and close-out all stamp relative backlinks. Suspected, not observed. Workaround: re-groom or re-attach the artifact so its backlink is re-stamped.

### Step 6.5 checkpoint prose may pass a stale revision

The implement-next skill now re-reads the record revision after the plan and build dispatches, because those steps commit to the record. The results-checkpoint mechanics (item 4) still say `--revision <revision>` without saying to re-read it. An agent following the text literally could get `contended` and need to re-read. Suspected. Suggested next action: tighten that line in a later edit to the skill.

### Minor test and message leftovers

Some older `evidence.recertify` refusal tests still assert that no PR edit happened. Recertify can no longer edit a PR, so those assertions can never fail. A `--markdown` read error is reported as "reading --record". `gitcli.Client.CommitChangedPaths` no longer has a caller. None of these affects behavior.

### This change was built on the previous flow

As the spec's cutover rule requires, this change's own plan and results ride its feature branch, and its evidence sits in the PR description.
