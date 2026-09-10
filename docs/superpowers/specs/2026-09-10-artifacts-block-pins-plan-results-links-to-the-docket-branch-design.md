<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0417 — Artifacts block pins plan/results links to the docket branch, where those files never live](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0417-artifacts-block-pins-plan-results-links-to-the-docket-branch.md)**
<!-- docket:backlink:end -->

# Lifecycle-pinned Plan/Results artifact links

## Problem

The `## Artifacts` block on a change file renders each row's GitHub blob URL against a single
hardcoded branch — the metadata branch `docket`:

- `internal/render/link.go` — `LinkContext.BlobURL(repoRelPath)` returns
  `RepoWebURL + "/blob/" + MetadataBranch + "/" + repoRelPath`.
- `internal/app/link_context.go` — `linkContextOf(pin)` sets `MetadataBranch:
  reposetup.MetadataBranchName` (`"docket"`), the sole `LinkContext` constructor.
- `internal/render/artifacts.go` — `pathRow` (Spec/Plan/Results) and `adcell` (ADRs) call
  `link.BlobURL(...)`, so every row resolves onto `blob/docket/<path>`.

That ref is correct for **Spec** and **ADR** rows — those records genuinely live on the `docket`
metadata branch. It is wrong for **Plan** and **Results**, whose files never live on `docket`:

- while the change is `in-progress`/`implemented`, the plan and results files live only on the
  change's **feature branch** (`branch:`);
- once the change is `done`, they have reached the **integration branch** (`main`) via the PR
  merge (terminal publication onto `docket` is deferred from Go v1, so they never land on
  `docket`).

So the Plan and Results links 404 in both states. Confirmed on change 0416: its results file is
present on `origin/fix/scoped-build-task-gate-starts-omit-prepared-scope-identity` but the change
file links `blob/docket/docs/results/…-results.md`, which does not exist.

`internal/render/artifacts.go`'s own doc comment already names this as deferred work: "the Bash
renderer's lifecycle-pinned Plan/Results branch is a later concern."

## Decision

Lifecycle-pin the blob ref **per row** for Plan and Results only. The ref points at wherever the
file actually is for the change's current status:

| Row | Ref while in-progress / implemented | Ref once done | Rationale |
|---|---|---|---|
| Spec | metadata branch (`docket`) | metadata branch | spec lives on `docket` |
| ADRs | metadata branch (`docket`) | metadata branch | ADRs live on `docket` |
| Plan | feature branch (`branch:`) | integration branch (`main`) | file is feature-branch → merges to integration |
| Results | feature branch (`branch:`) | integration branch (`main`) | same lifecycle as plan |

Selection is by the change's own `Status()` and `Branch()`, both already reachable in
`ArtifactBlockContent` (the full `domain.Change` is passed in). No new status, no new frontmatter
field.

Edge cases:

- **`branch:` unset** (a `proposed`/`trivial` change with no plan/results yet): the Plan/Results
  rows are absent anyway (nothing to link), so no ref is needed. If a plan/results path is somehow
  present with no `branch:`, fall back to the metadata branch (today's behavior) rather than emit a
  malformed URL — a defensive default, not an expected path.
- **`stacked-merged` / `killed`**: out of scope for special handling — killed changes rarely
  carry a plan/results, and stacked handling is 0405/stacked-changes territory. Use the same
  status test (`done` ⇒ integration, else feature branch); document the choice.
- **relative-link mode** (`block-*.relative.golden`): unaffected — relative rendering does not
  embed a branch ref; keep it byte-identical.

## Implementation shape

1. **Plumb the two extra refs into the render path.** `LinkContext` (or a small sibling passed to
   `ArtifactBlockContent`) gains the **integration branch name** and the **change feature branch**.
   `linkContextOf` is the sole constructor (guarded by `link_context_guard_test.go`) — extend it
   and its call sites (`change_create.go`, `change_implemented.go`, `finalize_closeout.go`,
   `repository_check.go`, …) to supply the integration branch from resolved config and the feature
   branch from the change. Prefer adding a `BlobURLOnBranch(path, branch)` (or a per-row branch
   selector) over threading status into `BlobURL`, keeping `link.go` a pure URL builder.
2. **Select the ref per row in `artifacts.go`.** `pathRow` for Plan and Results resolves its
   branch from `change.Status()`/`change.Branch()`; Spec and ADR rows keep `MetadataBranch`.
3. **Re-render at every existing frontmatter-write site.** The block already re-renders inside the
   owning metadata transaction on create, implemented, and closeout — no new call sites. The
   implemented transition is where the feature-branch ref first becomes correct; the closeout
   transition is where it flips to the integration branch. Verify the closeout path re-renders the
   block (it constructs a `LinkContext` per the exploration) so a `done` change's link is repointed
   to `main`.
4. **Tests.**
   - Extend `internal/render/artifacts_test.go` with cases: implemented change → Plan/Results on
     the feature branch, Spec/ADR on `docket`; done change → Plan/Results on the integration
     branch; missing-`branch:` fallback.
   - Regenerate the `block-spec-plan-results.github.golden` (and add a done-state golden);
     `*.relative.golden` stays byte-identical — assert that.
   - Extend `internal/app/link_context_test.go` for the new fields.
   - Add a **mutation-tested repoguard** asserting the Plan/Results rows do not hardcode the
     metadata branch: strip the per-row selection (revert to `MetadataBranch`) and the guard must
     redden. Key it on the render behavior (the produced ref for a Plan/Results row of an
     implemented change), not on a spelling.
5. **Budget re-baseline** if the new golden/guard shifts an exact-count assertion.

## Verification

- New renderer unit tests pass; guard reproduces the defect (RED on the hardcoded-`docket`
  revert) then passes.
- Manually re-render change 0416's block after the fix and confirm the Results link resolves to
  its feature branch; confirm an archived `done` change's Results link resolves to `main`.
- Full suite green at the build gate.

## Out of scope

- Adding a PR row to the Go renderer (still deferred).
- Copying plan/results onto the metadata branch / terminal publication (approach C, declined).
- Changing where plan/results files physically live.
- The reciprocal `docket:backlink` blocks.
