<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0363 — Remove main-mode compatibility from Go v1](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-29-0363-remove-main-mode-compatibility-from-go-v1.md)**
<!-- docket:backlink:end -->

# Remove main-mode compatibility from Go v1

## Purpose and boundary

Go v1 supports one steady-state repository topology: planning metadata lives on the fixed orphan
`docket` branch, while code lands on the independently resolved `integration_branch`. The
single-branch metadata topology formerly selected by `metadata_branch: main` is legacy migration
input, not a compatibility mode.

This change removes the second topology from active configuration, operational context, status,
planning mutations, finalization, link rendering, protocol output, documentation, and tests.
Change 0352 is a hard prerequisite: its repository classifier, health findings, and native
`repository migrate` operation must be landed before this contraction is implemented. Change 0318
remains the later Go-only cutover and does not absorb this work.

The change is a clean protocol-v1 cut. Go v1 has no released main-mode client contract to preserve,
so mode-shaped JSON fields disappear rather than surviving as constants.

## One supported topology

The branch roles are fixed and distinct:

- `docket` is the metadata branch and the source of active changes, archived changes, ADRs,
  learnings, specs, and their metadata-side generated links.
- `integration_branch` remains resolved configuration. It may be `main`, `develop`, or the remote
  default through `auto`; this change does not constrain trunk or GitFlow repositories.
- Plans, results, and code continue to reach the integration branch through feature-branch work.
- Terminal publishing remains disabled in Go v1. Removing main mode neither restores it nor copies
  new terminal metadata onto the integration branch.

No production API accepts a metadata-branch choice after this change. Internal code may use one
named constant for the fixed `docket` branch, owned beside repository-topology classification; it
must not thread the constant through mode-shaped fields that suggest another value is supported.

## Configuration contract and the legacy tombstone

`metadata_branch` is removed from the active Go configuration schema and from
`config.Effective`. It is absent from effective-configuration JSON, human effective-value output,
examples, capability presentation, and current configuration documentation.

The decoder retains a narrow, decode-only tombstone for the exact top-level key. The tombstone:

- recognizes the old key in any loaded layer so configuration inspection can name its source;
- emits an `obsolete-setting` diagnostic attributed to the declaring layer: repository-layer
  declarations point to `docket repository check`, while machine-local and global declarations
  tell the operator to remove the key from that named file;
- never contributes a resolved value, never selects a branch, and never appears as a capability;
- does not make an otherwise parseable document syntactically invalid; and
- is not a general alias or compatibility shim for main mode.

The repository-layer occurrence remains meaningful to change 0352's topology and migration path.
Migration uses its source-preserving editor to remove that exact top-level entry from the pinned
repository configuration bytes. Machine-local and global occurrences are non-authoritative stale
configuration: inspection names the offending layer, but they never affect topology and migration
does not claim authority to rewrite them.

`diagnostic config` remains available and reports configuration validity and capability policy. It
does not certify repository topology; operational commands additionally require the repository
gate below.

## Shared operational-repository gate

The implementation uses the existing status-pinning machinery but moves repository-validity
policy to an explicit shared owner. This is an extraction, not a second preflight or a duplicate set
of Git probes.

The shared loader performs one ordered read:

1. Discover the canonical repository and resolve the remote default branch.
2. Resolve normal configuration, including `integration_branch` and configured metadata paths.
   The obsolete tombstone is diagnostic-only and cannot influence the result.
3. Fetch and pin the configured integration revision and the fixed remote `docket` revision.
4. Probe the integration tree's live planning surface and the remaining facts required by change
   0352's repository classifier.
5. Classify the repository once and retain its stable state, reasons, findings, and exact remedies.
6. For an operational repository, return the pinned default, integration, and metadata revisions,
   resolved configuration, repository web identity, and other existing read context.

Ordinary repository-aware commands require the operational verdict before entering their own
logic. They do not inspect `metadata_branch`, guess from branch presence, or reproduce classifier
messages. `StatusReader.PinContext` becomes an adapter over this loader, preserving the existing
single-pin contract for status, planning, workspaces, gates, and finalization.

The repository command family intentionally calls the classifier below the operational gate:

| Command family | Allowed classifier states |
|---|---|
| `repository check` | every determinable or unknown state, for diagnosis |
| `repository init` | the states already authorized by change 0352's init contract |
| `repository migrate` | legacy and provably resumable migration states authorized by change 0352 |
| ordinary repository-aware commands | operational/healthy only |

Help, version, installation diagnostics, and configuration inspection remain usable without an
operational repository when their existing contracts do not require one.

## Typed refusal and failure ordering

A legacy single-branch repository is detected by change 0352's shared topology facts and
classifier, not by treating the obsolete key as an active selector. The classifier remains the
single source of these machine tokens:

- repository state: `legacy`;
- finding code: `legacy-repository`;
- severity: `error`; and
- remedy: `docket repository migrate`.

An ordinary command attempted in that state returns one protocol document whose envelope keeps the
attempted operation name and carries:

```json
{
  "result": "invalid-state",
  "reason": "legacy-repository",
  "repository_state": "legacy",
  "findings": [
    {
      "code": "legacy-repository",
      "severity": "error",
      "message": "Legacy single-branch docket layout: a live surface exists without a docket metadata branch.",
      "remedy": "Run `docket repository migrate` to convert this repository to the docket metadata topology."
    }
  ]
}
```

The message remains explanatory; clients key on the result, reason, repository state, and finding
code. The finding is the same typed value `repository check` returns, not a command-specific copy.
Human output presents the same state and remedy.

Failure ordering is fail-closed. Invalid configuration that prevents trustworthy path or
integration-branch resolution reports `invalid-config`. A recognized obsolete key does not mask
topology classification. Unknown or conflicting repository facts retain change 0352's own state
and findings and never collapse into the legacy remedy.

## Operational-context and protocol contraction

The shared operational context and its consumers lose every value whose purpose was selecting or
reporting a metadata mode:

- remove `StatusPin.Mode` and equivalent mode fields;
- remove configurable metadata-branch fields where the only valid value becomes `docket`;
- remove `config.Effective.MetadataBranch`;
- replace mode-conditioned source selection with the fixed metadata source plus the separately
  resolved integration source; and
- collapse helpers such as `metadataBranchOf` into the fixed branch owner or delete them when the
  caller no longer needs a branch parameter.

Status protocol output removes these fields completely:

- `config.effective.metadata_branch`;
- `status.context.metadata_mode`; and
- `status.context.metadata_branch`.

They are absent, not empty and not constant compatibility values. `metadata_revision` remains,
because it identifies the exact metadata snapshot the operation read. Default-branch and
integration-branch names and revisions remain unchanged. Human status output drops the `mode:`
line and may continue to render `metadata branch: docket @ <revision>` as useful explanatory
identity rather than as a configurable choice.

All mode branches in metadata transactions, planning lifecycle operations, finalization, cleanup,
backlink retargeting, and artifact/link rendering collapse to their docket-topology behavior.
Specs and metadata-side records link to `docket`; merged plans and results retain their existing
integration-branch link behavior. Removing the main-mode branch must not flatten this real
artifact-location variance.

## Test contraction without coverage loss

Implementation begins with a fresh whole-repository inventory after change 0352 reaches `done`.
The inventory derives executable mode sites from source rather than relying on a hand-maintained
file list, then separates them from maintained prose and frozen or historical records.

Tests are handled by the property they guard:

1. Generic application tests that use `mainModePin` only as convenient context switch to a
   docket-topology pin and retain their request validation, result, transaction, and finding
   assertions.
2. Genuine two-mode integration matrices collapse to their docket row. Their mode-independent
   assertions over exact-lease CAS, private transaction worktrees, ref isolation, retries,
   interruption recovery, finalization, cleanup, and link repair remain.
3. Tests whose sole subject is same-ref main-mode behavior are deleted rather than re-gated under a
   misleading name.
4. Docket-specific source separation is strengthened where the former second row accidentally
   supplied contrast: fixtures keep genuinely different integration and metadata trees so reading
   the wrong source remains observable.
5. Public protocol tests assert the three removed JSON keys are absent from serialized output.
6. Configuration tests prove the tombstone is diagnostic-only, absent from `Effective`, unable to
   select a branch, and source-attributed across layers.
7. Topology tests prove that legacy state reaches the shared typed refusal and that ordinary
   operations do not enter their bodies. Mutation probes remove the refusal and alter the fixed
   metadata source so the relevant tests visibly redden.
8. A structural guard detects production consumers that bypass the shared operational loader or
   reintroduce mode-shaped conditionals. It keys on executable shape and excludes point-in-time
   records explicitly; it is mutation-tested before being trusted.

Frozen `v0.9.2` fixture bytes are never rewritten. Existing main-mode inputs remain evidence of
what legacy repositories looked like and may feed obsolete-setting, repository-check, or migration
tests. If a current self-repository fixture must change, create a new versioned fixture tree with
provenance and re-derived expectations; never edit an older version in place. Historical specs,
plans, results, archived changes, Accepted ADR authored text, and the frozen compatibility corpus
retain their original claims.

The build gate is the configured full suite, `scripts/run-tests.sh`. Budget-watch and
parallel-sensitive output is reviewed, and any serial-confirmed budget breach is a build finding
even when the suite process exits zero.

## Documentation and decision records

Current program documentation, configuration examples, command help, and maintained agent-facing
contracts are rewritten for one topology and the explicit migration prerequisite. Repository-wide
search results are classified before editing so historical records are not falsified and copied
prose guards are relocated to the actual contract owner rather than restored as stale wording.

Implementation records a new ADR that supersedes ADR-0002. The new decision restates the surviving
default/bootstrap rules, removes the pinned main-mode opt-out, and makes native migration the only
legacy exit. ADR-0001 remains Accepted because its orphan metadata-branch architecture still
stands; it receives a dated update pointing to the new ADR and noting that the old opt-out
consequence no longer applies. The new ADR relates to ADR-0001 and ADR-0052. Its id is allocated by
the normal docket ADR workflow, then attached to this change.

## Explicit exclusions

This change does not:

- redesign or reimplement change 0352's initialization, migration, check, repair, confirmation,
  receipt, or recovery behavior;
- retain a transitional status read or another normal-command compatibility path for legacy
  repositories;
- remove or constrain `integration_branch`;
- restore terminal publishing or copy terminal metadata onto integration;
- rewrite frozen fixtures or point-in-time records to erase historical main-mode claims;
- contract unrelated configuration capabilities;
- remove Bash production code, change machine installation, prove self-hosting, publish a release,
  or perform change 0318's hard cutover; or
- create an implementation plan or modify production code during grooming.

## Acceptance boundary

The change is complete when:

- active Go configuration has no resolved `metadata_branch` setting and inspection treats the old
  key only as an attributed obsolete tombstone;
- every ordinary repository-aware command reaches one shared operational-repository gate, and a
  legacy repo receives the shared `invalid-state` / `legacy-repository` refusal and migrate remedy;
- repository check and migrate still diagnose and exit legacy state through change 0352's
  classifier and source-preserving editor;
- production code has no main-mode branch, mode selector, same-ref metadata fallback, or
  mode-shaped public field;
- status/config JSON omits the three removed fields while retaining revision and integration
  identity;
- docket-mode transaction, lifecycle, finalization, link, and failure invariants remain covered;
- frozen compatibility data and historical records remain byte-untouched;
- current documentation and examples describe one topology and the migration prerequisite;
- the superseding ADR and ADR-0001 update are recorded and linked; and
- the complete configured suite passes with no unresolved authoritative budget breach.
