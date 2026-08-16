# Provenance — artifact-block and spec-backlink goldens (change 0312, task 3)

These goldens are **historical snapshots** frozen once from the live Bash
renderers `scripts/render-change-links.sh` and `scripts/render-artifact-backlink.sh`.
Under docket's frozen-golden contract they must **NOT** track those scripts:
the scripts die in the 0316+ cutover, and a human decides whether the canonical
Go shape should change and updates the golden and its `internal/render`
serializer together. The byte-equality tests in `artifacts_test.go` are the
drift assert.

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

## Notable Bash behaviors these goldens pin

- In GitHub mode an ADR cell is `[ADR-NNNN](blob-url-to-the-resolved-file)`,
  comma-joined. In repo-relative mode the `ADR-NNNN` label is **dropped**: the
  cell is the backtick-quoted repo-relative path, comma-joined.
- Spec/Plan/Results link **text** is the path basename; the URL is the full
  repo-relative path.
- Rows appear in fixed order Spec, Plan, Results, ADRs; a row is omitted when
  its field is unset/empty. The `## Artifacts` block's PR row and the derived
  "Stacked children" row are **out of scope** for the v1 typed renderer (PR is a
  later slice; stacked children is a render-time directory scan) and are not
  frozen here.
