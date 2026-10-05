<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0530 — Keep plan, results, and build evidence on the metadata branch, and ship the spec with the PR](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0530-keep-plan-results-and-build-evidence-on-the-metadata-branch.md)**
<!-- docket:backlink:end -->

# Keep plan, results, and build evidence on the metadata branch, and ship the spec with the PR: design

Change #530, groomed interactively on 2026-10-05. It is the first of the private-visibility series (#529–#533), but it applies to **every** repository: the human chose one location for build artifacts in both modes over a private-only path.

## Summary

Today the spec lives on the metadata branch, while the plan and the results file ride the feature branch into the integration branch, and the build evidence lives in the PR description. After this change:

| Artifact | Today | After |
|---|---|---|
| Change record, ADRs, learnings, board | metadata branch | unchanged |
| Spec | metadata branch only | metadata branch (source) **plus a copy committed as the feature branch's first commit, which merges with the code** |
| Plan | feature branch → integration | metadata branch, same path |
| Results | feature branch → integration (one commit per checkpoint) | metadata branch, same path (one metadata commit per checkpoint) |
| Build evidence | marker block in the PR description | `## Build evidence` section of the change record |
| Links between same-branch artifacts | absolute GitHub URLs | relative paths |

A shared-mode PR then contains the spec plus the code, and nothing else.

## Evidence gathered at grooming

- **Branch census (this repository, 2026-10-05).** `docs/superpowers/specs/` has 197 files on `main`, all from before the metadata-branch migration, and 387 on `docket`. `docs/superpowers/plans/` has 374 on `main` and 0 on `docket`. `docs/results/` has 291 on `main` and 0 on `docket`.
- **What one change adds to `main` today (0507).** A `docs(plan)` commit, three `docs(results)` commits, and the post-merge `change 0507 final backlinks retargeted to archive` commit, which was pushed directly to `main` outside the PR.
- **Plan contract.** `agents/docket-plan-writer.md` says: "You own exactly one durable artifact: the plan file, committed on the feature branch … no writes outside the feature worktree." `change.attach-plan` (`internal/app/change_attach.go`) requires:
  - the plan under the hard-coded `plansPlanningRoot = "docs/superpowers/plans"` at the feature head;
  - a single-artifact commit descending from the base;
  - a `Docket-Plan-Path:` trailer (`verifyPathTrailer`);
  - a well-formed backlink.
- **Results contract.**
  - `change.attach-results` requires the file under `results_dir` at the feature head.
  - `change.mark-implemented` refuses `results-missing`, `results-identity-broken`, and `results-content-invalid`.
  - `run.verify` (`internal/app/run_verify.go`) checks the same conditions at the feature head.
  - Implement-next's evidence-sequencing rule says: "A post-gate results edit invalidates stale evidence and can require another certification run."
- **Evidence.**
  - The codec (`internal/evidence/codec.go`) renders a four-line block named `build-evidence`.
  - `pr.publish` (`assemblePRBody`) upserts the block into the PR body, and `finalize.publish` and `evidence.recertify` rewrite it there.
  - The finalize merge gate reads it: `GateSatisfied` in `internal/app/finalize_merge.go`.
  - So does the finalize rebase no-op skip (`internal/app/finalize_rebase.go`) and `run.verify`.
- **Integration-branch backlink legs.** `runCloseoutBacklinkLeg` / `backlinkLegRetarget` (`finalize_closeout.go`), `cleanupBacklinkOp` (`finalize_cleanup.go`), and `sweepAssessBacklinkLeg` / `sweepBacklinkArtifactPaths` (`maintenance_assess.go`) re-stamp plan and results backlinks on the integration branch.
- **Links.** `render.BlobURLOnBranch` (`internal/render/link.go`) and `lifecycleBranch` (`internal/render/artifacts.go`) render absolute URLs. Archived records already carry absolute links to specs and ADRs on `/blob/docket/` (for example change 0363's `## Artifacts` block).
- **Status.** `artifactChecks` (`internal/app/status.go`) checks plan and results paths against the integration revision.

## Decisions (settled with the human)

1. One location for every repository: plan, results, and evidence live on the metadata branch in shared and private mode alike.
2. Plan and results keep their paths (`docs/superpowers/plans/<date>-<slug>.md`, `<results_dir>/<date>-<slug>-results.md`); only the branch changes.
3. Evidence becomes a `## Build evidence` body section of the change record, reusing the existing codec and marker block unchanged. It is not frontmatter, because the board reads the header and evidence re-mints several times per run.
4. Each results checkpoint is its own metadata commit.
5. The spec reaches the integration branch by riding the PR as the feature branch's first commit, never by a direct push.
6. Same-branch links become relative, with a one-time conversion of existing records.
7. Changes in flight at cutover finish on the previous version. There is no compatibility read of PR-body evidence.

## Design

### 1. Plan on the metadata branch

- The plan-writer still authors the plan with `superpowers:writing-plans`. It no longer commits anything on the feature branch; it hands the plan to `change.attach-plan`.
- `change.attach-plan` becomes a metadata transaction carrying the plan Markdown, the way `change.groom` carries `spec_markdown`. In one commit it:
  - writes `docs/superpowers/plans/<date>-<slug>.md` on the metadata branch;
  - stamps the plan's backlink to the change record;
  - sets `plan:` and `updated:`;
  - re-renders `## Artifacts` and the board.
  - It is idempotent on replay, and a later call replaces the content for a re-plan.
- Refusals that no longer apply are removed: feature-head, descends-from-base, single-artifact delta, and the `Docket-Plan-Path` trailer. Path-under-plans-root and exact-revision checks stay.
- Build workers and reviewers receive the plan's absolute path in the metadata worktree, read after a re-sync (`repository.prepare`), the same way the spec is read today. The plan is never in the feature worktree.
- The plan-writer's receipt and the parent's verification move from git state on the feature branch to git state on the metadata branch: `plan:` is set and the file exists at the metadata tip.

### 2. Results on the metadata branch

- `change.attach-results` becomes a metadata transaction carrying the results Markdown. The first call writes the file at `<results_dir>/<date>-<slug>-results.md` on the metadata branch, stamps its backlink, and sets `results:`. Later checkpoints replace the content.
- Every checkpoint (the build checkpoint performed by the build controller, the review checkpoint, final consolidation, and any later one) is one metadata commit. The feature branch never moves for a results edit, so evidence never goes stale because of one. The evidence-sequencing rule simplifies accordingly.
- `ValidateResultsContent`, `mark-implemented` condition 5, and `run.verify` read the results file from the metadata tip, not the feature head.

### 3. Build evidence in the change record

- The record gains an operation-owned `## Build evidence` section holding the existing codec's block, byte-identical to today's PR-body block. It replaces itself on each re-mint and stays in the archived record.
- Every operation that writes the PR-body block today writes the record section instead, through an exact-revision metadata transaction: `pr.publish`, `finalize.publish`, `evidence.recertify`, and `mark-implemented`, where it records the evidence it verifies. Catalog effects gain `metadata-write` where needed.
- Every reader reads the record section: the finalize merge gate, the finalize rebase no-op skip, `run.verify`, and `evidence.recertify`.
- The PR description no longer carries the evidence block.
- Recovery for a change caught in flight at cutover is `evidence.recertify`, which writes the record section. No compatibility read of the PR body is added.

### 4. The spec rides the PR

- Immediately after the feature workspace is prepared, and before the plan, a typed operation commits the spec copy as the feature branch's first commit.
  - **Path:** the metadata spec's own path, `docs/superpowers/specs/<date>-<slug>-design.md`.
  - **Content:** the metadata spec without its backlink block. In shared mode one plain line follows the title, `Change NNNN — <title>`, with no URL, so it never needs updating. Change #532 omits that line in private mode, so leave the seam.
  - **Commit subject:** neutral, for example `Add design spec: <title>`.
- Because the copy precedes the build gate, it never stales evidence.
- If a spec is revised while the change is in flight (halt, re-groom, resume), resuming replaces the copy before building continues, detected by comparing bytes.
- Trivial changes (no spec) get no copy. A stacked child's copy rides the child's branch.
- If the integration branch already has a different file at that path, the operation refuses with a typed reason. The 197 legacy specs on `main` are untouched.
- After the merge is verified, close-out renders a `Spec (merged)` row in `## Artifacts` with the absolute integration-branch URL. Integration paths never move.

### 5. Links

- `## Artifacts` rows for spec, plan, results, and ADRs, plus board rows, become relative paths from the file containing the link. `docs/changes/active/` and `docs/changes/archive/` are siblings, so a record's relative links are identical before and after archiving. They resolve on GitHub (against the branch being viewed) and in a local checkout.
- Backlink blocks inside spec, plan, and results files on the metadata branch link relatively to the record. The archive transaction retargets them atomically in the same commit, as it already does for the spec. There is no integration push.
- PR row and `Spec (merged)` row: absolute. In shared mode, the PR description keeps its backlink line (absolute, and repointed at archive by #529) and gains absolute links to the plan and results on the metadata branch. Those paths never move.
- **One-time conversion.** `repository check` reports records whose `## Artifacts` block carries absolute same-branch links, and `repository repair` re-renders them in one metadata commit. Rows pointing at plan and results files already on the integration branch stay absolute, so legacy records keep working.
- `artifactChecks` looks for a record's plan and results on the metadata branch, falling back to the integration branch for records closed before cutover.

### 6. Retirements

- The integration-branch backlink legs and their pending warnings: close-out, cleanup, and sweep assessment. #529's PR-body leg is separate and stays.
- The `Docket-Plan-Path` trailer and the unused `Docket-Results-Path` constant.
- The deferred key `finalize.skip_results_only_delta`. Its premise, results-only commits after the gate, no longer exists. Retire it under the schema's retirement convention, drop the stale comment block in this repository's `.docket.yml`, and leave frozen `testdata/` corpora untouched.
- `artifact.backlink`, if a whole-repo grep shows no caller remains once attach-plan and attach-results stamp their own backlinks.

### 7. Skills, agents, embedded twins

- **docket-implement-next:** Steps 4, 6.5 and 7, the postcondition table, PR-body assembly (no evidence block), evidence sequencing, and the plan and results seams in `references/edge-paths.md`.
- **docket-plan-writer:** the agent contract.
- **docket-build:** the controller's build checkpoint and the time-limit audit line, which now go to the metadata results file.
- **docket-finalize-change:** the evidence source.
- **docket-status:** the artifact checks.
- **docket-convention:**
  - the directory layout;
  - the manifest comments for `plan:` and `results:`;
  - the frozen-artifact rule, which becomes "closed-out plans and results";
  - the branch model ("the feature branch adds the spec copy and the code");
  - the derived-views paragraph.
- The embedded twins under `internal/assets/embedded/tree/` change in step, and the prose-contract pins are updated.

### 8. Living docs

Correct every living page that would otherwise describe removed behavior. Derive the list from a whole-repo grep, not from memory. Known pages:

- `docs/guide/where-the-metadata-lives.md`, whose table says plan and results reach the integration branch through the PR;
- `docs/concepts/two-branches.md`;
- `docs/guide/landing-changes.md`;
- `docs/reference/fields.md`.

Describe current behavior only. Private visibility is **not** mentioned in any `docs/` page.

## Cutover

Changes that are in progress or implemented when this lands finish on the previous binary before the new one is installed. If one is caught mid-flight, `evidence.recertify` writes the record section, and the remaining artifacts can be re-attached through the new operations. The results file's human-action section states this install ordering.

## Acceptance criteria

1. A full shared-mode run (claim → plan → build → review → PR → mark-implemented → finalize) leaves the PR diff containing exactly the spec copy plus code. The plan, the results file, and the `## Build evidence` section exist on the metadata branch.
2. A results checkpoint after the gate does not change the feature head, and finalize does not re-run the suite because of it.
3. The finalize merge gate, rebase skip, and `run.verify` pass and fail from the record section exactly as they did from the PR body (parity tests). Removing the record read turns them red.
4. No operation pushes to the integration branch outside a PR merge (pinned by a test).
5. The spec copy is the first feature-branch commit, matches the metadata spec minus its backlink block plus the change line, and is refreshed after a mid-flight re-groom.
6. A record's relative links resolve before and after archiving. `repository repair` converts legacy absolute same-branch links in one commit, and a second run is a no-op.
7. The whole suite is green, and the living-docs guards pass with the corrected pages.

## ADRs expected

One ADR: build artifacts (plan, results, evidence) live on the metadata branch, and the spec ships with the PR. It relates to ADR-0001 (whose "publish terminal records by copy" this replaces with a copy that rides the PR) and ADR-0066 (where the evidence lives).

## Out of scope

- Private visibility: remote, naming, footprint, switching (#531–#533).
- Moving plan and results files already merged into the integration branch.
- Changing the plan or results content contract.
- A compatibility read of PR-body evidence.
