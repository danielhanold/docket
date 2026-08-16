---
id: 310
slug: read-only-status-and-health-vertical-slice
title: 'Read-only status and health vertical slice'
status: done
priority: critical
type: feat
created: 2026-08-12
updated: 2026-08-16
depends_on: [307, 308]
stacked_on:
related: [261]
discovered_from: [303]
adrs: [1, 28, 34, 47, 92, 93]
spec: docs/superpowers/specs/2026-08-15-read-only-status-and-health-vertical-slice-design.md
plan: docs/superpowers/plans/2026-08-16-read-only-status-and-health-vertical-slice.md
results: docs/results/2026-08-16-read-only-status-and-health-vertical-slice-results.md
trivial: false
auto_groomable:
branch: feat/read-only-status-and-health-vertical-slice
pr: https://github.com/danielhanold/docket/pull/212
blocked_by:
claimed_at: 
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-15-read-only-status-and-health-vertical-slice-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-15-read-only-status-and-health-vertical-slice-design.md) |
| Plan | [2026-08-16-read-only-status-and-health-vertical-slice.md](https://github.com/danielhanold/docket/blob/feat/read-only-status-and-health-vertical-slice/docs/superpowers/plans/2026-08-16-read-only-status-and-health-vertical-slice.md) |
| Results | [2026-08-16-read-only-status-and-health-vertical-slice-results.md](https://github.com/danielhanold/docket/blob/feat/read-only-status-and-health-vertical-slice/docs/results/2026-08-16-read-only-status-and-health-vertical-slice-results.md) |
| PR | [#212](https://github.com/danielhanold/docket/pull/212) |
| ADRs | [ADR-0001](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0001-docket-metadata-branch-model.md), [ADR-0028](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0028-report-channel-is-not-a-board-surface.md), [ADR-0034](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0034-repo-root-anchored-to-main-worktree.md), [ADR-0047](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0047-digest-only-read-tier-skips-preflight.md), [ADR-0092](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0092-a-stacked-changes-base-is-its-parents-merge-destination.md), [ADR-0093](https://github.com/danielhanold/docket/blob/docket/docs/adrs/0093-repository-reference-severity-graded-by-structural-role.md) |
<!-- docket:artifacts:end -->

## Why

The first retained user workflow should prove that Go can open, interpret, and report an existing
repository without mutation before it is trusted to write.

## What changes

Add one `docket status` application and CLI slice that composes pinned authoritative Git objects
with the landed configuration, document, repository, and domain APIs. Report active status,
readiness, ordered selection, dependency and stack context, artifact integrity, and deterministic
health through human text and protocol-v1 JSON. Keep targeted Git fetches observational and all
metadata maintenance explicit and separate.

## Out of scope

Behavior owned by changes 0305 through 0309 or 0311 through 0318; board writes; lifecycle mutation;
maintenance sweeps; transaction worktrees; feature workspaces; pull requests; evidence capture;
supervision; release; and Bash cutover. Change 0261's unmerged board and health-check behavior also
remains separate.

## Design decisions

The focused design is approved in the linked spec. One `docket status` operation produces a shared
application result for deterministic human and JSON presenters. Repository defects remain health
data under an applied result; only failures that prevent trustworthy authoritative context fail the
operation. Filters affect the backlog projection but never reduce full-corpus health validation.

New frozen compatibility fixtures are historical snapshots derived from the refreshed `v0.9.3`
tag at peeled commit `dd742abd5e9fcdf8ffe78eb6f36a293410873bbf`. The added plan-writer feature is
fixture input only and does not expand this change into later workflow behavior.

## Reconcile log

<!-- Appended by docket-implement-next's reconcile pass: dated entries of what changed. -->

### 2026-08-16

Reconciled against current `main` (HEAD `a258c772`) before planning. Findings:

- **Dependencies satisfied.** `depends_on: [307, 308]` are both `done` (archived 2026-08-14/15);
  0309 (transaction engine, not a dependency of this read-only slice) is also archived. Build-ready
  confirmed.
- **Landed foundations all present and matching the spec.** The seven internal packages the spec
  names as clients-of are all on `main` and expose the described surfaces: `internal/gitcli`
  (`Discover`, `RemoteDefaultBranch`/`FetchBranch`/`ResolveRef`, `OpenObjectSource`→`ObjectSource`
  with `Revision()`/`ListTree()`, `ReadBlobs`), `internal/config`
  (`Resolve`→`Snapshot`/`Diagnostic`, `Effective`, capabilities, `LoadFilesystemSources`),
  `internal/document` (loss-preserving parse + typed structural errors), `internal/repository`
  (`BuildSnapshot`→`BuildResult` with deterministic `domain.Finding` validation),
  `internal/domain` (`EvaluateReadiness`, `SelectQueue`, `ResolveEffectiveBase`/stack semantics,
  `BranchFacts`), `internal/app` (protocol-v1 `Envelope`, the `Result*` taxonomy, `ExitCode`,
  `OperationResult`), `internal/cli` (Cobra `root.go` wiring, global `--json` via `DetectJSONMode`,
  `Presenter`). No parser/validator/graph/porcelain duplication is required — the slice composes.
- **`docket status` is greenfield.** No `docket status` command, no `StatusResult`, no status
  presenter exists. (`internal/gitcli/status.go` is git working-tree porcelain parsing;
  `domain`/`repository` `Status()` refer to a change's lifecycle status field — neither is this
  command.) Registration follows the existing pattern: a `*cobra.Command` in `root.go` whose `RunE`
  calls an `internal/app` operation constructor and assigns the returned `OperationResult`; the
  command key must also be added to the `assetIndependent` map in `internal/cli/install.go`
  (`TestAssetIndependentSetExact` enforces exact correspondence). The `diagnostic config` command
  (`app.ConfigInspectionResult`) is the closest template for the new result type.
- **v0.9.3 fixture provenance clarified (no scope change).** The `v0.9.3` tag object peels to commit
  `dd742abd5e9fcdf8ffe78eb6f36a293410873bbf` — exactly the commit the spec cites for the frozen
  semantic corpus; the spec's provenance is current. Note that `testdata/repositories/v0.9.3/`
  already exists on `main` but holds ONLY change 0324's agent-defaults sidecar
  (`agents-harness-defaults.yml` + a `PROVENANCE.md` naming its own source commit `a4d72613`, the
  seventeenth-agent re-cut). The new frozen semantic-corpus fixtures this change introduces are
  derived from the peeled tag commit `dd742abd` and must be added under the same `v0.9.3` corpus
  version WITHOUT relabeling or absorbing the pre-existing sidecar — the two provenance records
  (`a4d72613` sidecar vs `dd742abd` semantic corpus) coexist. This is a planning detail for the
  fixture step, not a redesign; the spec already forbids reusing/relabeling earlier corpora.
- **Scope unchanged.** No design invalidation; the acceptance boundary, delivered boundary, and
  explicit exclusions all hold against current reality. No auto-capture stub minted — no
  independently-valuable adjacent work surfaced (the v0.9.3 dual-provenance is within 0310's own
  fixture scope).
