# Provenance — artifact-block and spec-backlink goldens (change 0312, task 3)

These goldens were **historical snapshots** frozen once from the live Bash
renderers `scripts/render-change-links.sh` and `scripts/render-artifact-backlink.sh`.
Under docket's frozen-golden contract they must **NOT** track those scripts:
the scripts die in the 0316+ cutover, and a human decides whether the canonical
Go shape should change and updates the golden and its `internal/render`
serializer together. The byte-equality tests in `artifacts_test.go` are the
drift assert.

## The artifact-block goldens are now Go renderer output (change 0530)

Change 0530 deliberately changed the canonical `## Artifacts` shape: plan,
results, spec, and ADRs all live on the metadata branch, so every row of a
non-legacy record is a link **relative to the record file** (spec acceptance 6
— the links survive the active-to-archive move unchanged and resolve both on
GitHub and in a local checkout).

The four `block-*.golden` files were therefore replaced with the Go
renderer's (`render.ArtifactBlockContent`) output for the same two fixtures,
and the Go renderer is now their source. Relative rows render identically
with or without a web URL, so each `.github.golden` / `.relative.golden` pair
is byte-identical; both are kept so the two link contexts stay pinned
separately. The fixture-to-golden mapping below is unchanged; the Bash
commands are kept as the record of the original freeze. The `backlink-*`
goldens are untouched: `render.BacklinkContent` keeps the absolute form for
PR descriptions (a PR body has no branch to be relative to), and the
metadata-branch file backlink is the separate `render.ArtifactBacklinkContent`,
pinned by explicit strings in `artifacts_test.go`.

## Generating commit

Frozen at commit `7ab36b6323871b45626ee810b5ae9c539e100aed` (change 0312, HEAD
after task 2), using the Bash renderers as they stood at that commit.

## Exact commands

A scratch tree was built with a mock `DOCKET_CONFIG` exporting
`METADATA_BRANCH=docket`, `INTEGRATION_BRANCH=main`, `CHANGES_DIR=docs/changes`,
`ADRS_DIR=docs/adrs`, plus two ADR files (`docs/adrs/0001-first-decision.md`,
`docs/adrs/0002-second-decision.md`) so ADR slug resolution succeeds, and two
change fixtures:

- `0007-alpha-change` — `spec` set, `adrs: [1, 2]`, no `plan`/`results`.
- `0008-beta-change` — `spec`/`plan`/`results` set, `branch: docket` (so the
  Bash renderer's lifecycle-pinned `build_ref` for Plan/Results resolves onto
  the same `docket` branch the Spec row uses; the v1 Go renderer carries a
  single-branch `LinkContext`, so the fixture is arranged to keep every row on
  one branch).

Artifact-block goldens (the interior bytes **between** the `docket:artifacts`
markers, exclusive — this is what `ArtifactBlockContent` returns; the caller
writes the markers via `document.ReplaceBlock`):

```
# GitHub mode
render-change-links.sh --change-file <fixture> --repo danielhanold/docket --adrs-dir docs/adrs
# repo-relative mode (no --repo, no github remote on the fixture dir)
render-change-links.sh --change-file <fixture> --adrs-dir docs/adrs
# then: extract the lines strictly between docket:artifacts:start/end
```

- `block-spec-adrs.github.golden`, `block-spec-adrs.relative.golden`
- `block-spec-plan-results.github.golden`, `block-spec-plan-results.relative.golden`

Backlink goldens (the **full** block, markers inclusive — this is what
`BacklinkContent` returns):

```
render-artifact-backlink.sh --artifact-file <spec> --change-file <change> [--repo danielhanold/docket]
# then: extract docket:backlink:start … docket:backlink:end inclusive
```

- `backlink-active.github.golden` — change at `docs/changes/active/0007-alpha-change.md`.
- `backlink-archive.github.golden` — same change relocated to
  `docs/changes/archive/2026-08-16-0007-alpha-change.md` (the kill-retarget
  shape: the link TARGET is the archive path).
- `backlink-active.relative.golden` — repo-relative (empty `RepoWebURL`).

## Notable behaviors these goldens pin

- An ADR cell is `[ADR-NNNN](relative-path-to-the-resolved-file)`,
  comma-joined, with or without a web URL. (The Bash renderer dropped the
  `ADR-NNNN` label in repo-relative mode; the relative form keeps it.)
- Spec/Plan/Results link **text** is the path basename; the link is the path
  relative to the record file.
- Rows appear in fixed order Spec, Plan, Results, ADRs; a row is omitted when
  its field is unset/empty. (The Go renderer's done-only `Spec (merged)` row
  and the legacy absolute Plan/Results rows are pinned in `artifacts_test.go`,
  not here.) The `## Artifacts` block's PR row and the derived
  "Stacked children" row are **out of scope** for the v1 typed renderer (PR is a
  later slice; stacked children is a render-time directory scan) and are not
  frozen here.
