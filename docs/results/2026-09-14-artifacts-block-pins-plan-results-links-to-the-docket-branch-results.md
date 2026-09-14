<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0417 — Artifacts block pins plan/results links to the docket branch, where those files never live](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0417-artifacts-block-pins-plan-results-links-to-the-docket-branch.md)**
<!-- docket:backlink:end -->
# Artifacts block pins plan/results links to the docket branch, where those files never live — Results

## Outcome

The `## Artifacts` block now lifecycle-pins its **Plan** and **Results** rows to the branch where
those files actually live, instead of the hardcoded `docket` metadata branch where they never do.
Spec and ADR rows are unchanged (those records genuinely live on `docket`).

- `render.LinkContext` gained an `IntegrationBranch` field and a `BlobURLOnBranch(repoRelPath, branch)`
  builder; `BlobURL` now delegates to it against `MetadataBranch`, so its behavior is preserved.
  An empty branch falls back to `MetadataBranch`, making a malformed `/blob//…` URL unrepresentable.
- `internal/render/artifacts.go` selects the ref per row via a new `lifecycleBranch(c, link)`:
  a `done` change resolves Plan/Results to `IntegrationBranch`; every other status — including
  `stacked-merged` and `killed`, which get no special handling by design — resolves to the change's
  feature branch (`branch:`). An unset `branch:` (or empty `IntegrationBranch` on a `done` change)
  falls back to the metadata branch.
- The app layer's sole `LinkContext` constructor, `linkContextOf`, now fills `IntegrationBranch`
  from the pinned `StatusPin.IntegrationBranch`, falling back to `StatusPin.DefaultBranch` — the
  same fallback finalize's closeout uses for git operations. All existing `ArtifactBlockContent`
  call sites pick this up with no edits; only `repository_check.go`'s hand-built partial pin needed
  the extra field so the health check renders byte-identically to the authoritative writers.

No new status, no new frontmatter field, no new re-render call site. Relative-link mode (empty
`RepoWebURL`) embeds no branch and stays byte-identical to before.

Two deliberate spec departures, both narrower/stronger than the spec text:

- The spec anticipated regenerating `block-spec-plan-results.github.golden` and adding a done-state
  golden. Instead, the frozen goldens are asserted **byte-identical** (their fixtures have no
  status and no `branch:`, so they exercise exactly the metadata-branch fallback path unchanged) and
  the new lifecycle behavior is pinned by inline exact-URL unit tests. `PROVENANCE.md` forbids
  regenerating historical snapshots, so byte-identity is the correct outcome.

## Verification performed

- Full test suite green at the build gate via the configured `build.test_command`
  (`go run ./cmd/docket development test`), driven through the native gate driver. See the
  build-evidence block in the PR body for the certified head and result.
- TDD per task: each task wrote failing tests first (confirmed RED for the intended reason), then
  went green. New renderer unit tests cover: implemented change → Plan/Results on the feature
  branch and Spec on `docket`; done change → Plan/Results on the integration branch; `stacked-merged`
  → feature branch; missing-`branch:` fallback to metadata; relative-mode byte-identity to the
  frozen golden.
- Mutation probes (each backed up, mutated, run with `-count=1`, restored): reverting the per-row
  selection to the hardcoded `MetadataBranch` reddens the implemented+done cases; deleting the
  `StatusDone` arm reddens the done case; deleting the empty-branch fallback reddens the
  fallback/`BlobURLOnBranch` tests; deleting the `IntegrationBranch` assignment in `linkContextOf`
  reddens `TestLinkContextOf*`. The 0341 `TestLinkContextSoleConstructor` guard stays green (no new
  field-carrying `render.LinkContext` literal outside `link_context.go`).
- Real-state check (renders are invisible to the hermetic suite). Change 0416 is now `done`
  (PR #295 merged). Its plan and results files were confirmed present on `origin/main` and **absent**
  on `origin/docket`, so the pre-0417 `blob/docket/…` Results link 404s. Tracing `lifecycleBranch`
  against 0416's `done` status yields the integration branch (`main`), where the file verifiably
  lives — the fix resolves the reported defect.

## Findings and limitations

### The check-corpus wiring for the integration branch is only indirectly guarded (known residual)

Dropping `IntegrationBranch: sc.integrationBranch` from `readCheckCorpus`'s corpus pin in
`repository_check.go` reddens no existing test: the hermetic check-corpus fixtures
(`repository_check_derived_test.go`) contain no implemented/done change carrying a Plan/Results row
that would render on the integration branch, so no assertion exercises `corpus.link.IntegrationBranch`.
The plumbing is correct and mirrors `linkContextOf`'s fallback, but that one call site's wiring is
proven only by inspection, not by a reddening probe. A vacuous assert was deliberately not written.
See Follow-ups.

### Expected one-time post-merge effect on existing records

Records written before this change carry docket-pinned Plan/Results rows. After merge,
`docket repository check` will surface artifact-links drift for `implemented`/`done` changes that
have `plan:`/`results:` set; the existing remedy is `docket repository migrate` (its repair path
renders through the same corpus link this change fixed). This is expected behavior, not a defect.

## Follow-ups

### Add a done-change check-corpus fixture with plan/results rows

Add a fixture to the `internal/app` check-corpus tests representing an implemented or done change
that carries `plan:`/`results:` rows, so dropping the integration-branch entry from
`readCheckCorpus`'s pin reddens a test. This closes the known residual above by turning the
inspection-only guarantee into a mutation-tested guard. Out of scope here because it requires a new
hermetic fixture rather than a change to the shipped behavior.
