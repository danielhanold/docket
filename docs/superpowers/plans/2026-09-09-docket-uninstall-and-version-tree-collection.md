<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0323 — docket uninstall and version-tree collection for the Go installer](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0323-docket-uninstall-and-version-tree-collection-for-the-go-inst.md)**
<!-- docket:backlink:end -->
# Docket Uninstall and Version-Tree Collection Implementation Plan

> **For agentic workers:** REQUIRED BUILD SKILL: Use `docket-build` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add ownership-safe user-level harness uninstall and resumable collection of unreferenced, verified installer version trees while retaining Docket's CLI, configuration, source checkouts, and repository-local setup.

**Architecture:** Keep all destructive decisions in independently testable `internal/install` planners: validate installed state, derive references from recorded and live paths, prove candidate trees against a saved or frozen-v1 manifest, and journal each quarantine/delete cycle. Uninstall reuses the existing exclusive lock and rollback transaction for harness targets and state, then invokes the same locked collector used after successful release and development installs; application results report installation/uninstall success separately from collection warnings.

**Tech Stack:** Go 1.25, Cobra CLI, `internal/install` filesystem and transaction primitives, protocol-v1 app envelopes and schema registry, Go tests with hermetic temporary homes, and the Go-native whole-suite runner.

**Spec:** `docs/superpowers/specs/2026-09-07-docket-uninstall-and-version-tree-collection-for-the-go-inst-design.md`

## Global Constraints

- `docket uninstall [--harness <name>]... [--dry-run]` removes all recorded harnesses when no filter is supplied; explicit filters are validated and deduplicated before mutation.
- `docket install collect [--dry-run]` and uninstall are asset-independent, repository-independent operations whose only authority is `<data-root>/state/install.json`.
- Preserve CLI binaries and their ownership records, global configuration, contributor checkouts, repository metadata, repository-local instruction surfaces and ownership records, user-created bytes, and parent directories; introduce no force or purge mode.
- Use the existing exclusive install lock for actual recovery, state reads, uninstall mutation, and collection. Dry-run creates nothing, performs no recovery, and refuses contention or uncertain consistency.
- Treat missing, malformed, unsupported, unreadable, or unresolved evidence as “retain”; probe errors must never become clean absence.
- Compute references afresh from the complete ownership state, including recorded paths/destinations and observed live symlink destinations. Canonicalize existing ancestors and every symlink hop.
- Collect only immediate, non-symlink children of the canonical managed versions root after proving the exact payload and directory layout. Never recursively delete the versions root or follow candidate symlinks.
- Publish a canonical manifest beside `assets/` for newly extracted trees. Reuse existing valid protocol-v1 trees without rewriting payloads, reconstructing the manifest through a frozen v1 role/layout contract.
- Journal collection separately from rollback installation journals; atomically quarantine one verified tree on the same filesystem, then delete only its verified entries and owned directories. Keep the journal until cleanup finishes.
- Automatic collection runs only after a successful/no-op release install, development-install candidate, or uninstall transaction has finalized. Its failure is a warning and does not change the primary operation's success. Explicit collection fails when the pass cannot complete.
- `install check` remains read-only and treats a valid empty-harness ownership record as installation required. A later install must repopulate that state and retain/prove a development binary record.
- ADR-0096 freezes compatibility reproduction; do not derive the legacy-v1 collector from live generator roots or roles. ADR-0110 remains install-path-only; uninstall and collection do not load configuration.
- The parent implementation workflow must use the normal `docket-adr` step to record the state-derived reference-set, post-commit collection, and retained-CLI decisions. Do not edit an Accepted ADR.
- Every destructive guard is mutation-tested with `go test -count=1`; save uncommitted files to a temporary backup before mutation and restore from that backup, never with `git checkout --`.
- The authoritative build gate is `go run ./cmd/docket development test`. Read and act on `BUDGET WATCH:`, `PARALLEL-SENSITIVE:`, and `SERIAL CONFIRMED OVER BUDGET:` lines even when the command exits zero.

## File and Responsibility Map

| File | Responsibility |
| --- | --- |
| `internal/install/state.go`, `state_test.go` | Strict installed-state validation and active-vs-empty installation semantics. |
| `internal/install/roots.go`, `roots_test.go` | Collection journal/quarantine paths and canonical managed-root helpers. |
| `internal/install/version.go`, `version_test.go` | New-tree manifest publication and exact current-manifest candidate proof. |
| `internal/install/legacy_v1_manifest.go`, `legacy_v1_manifest_test.go` | Frozen protocol-v1 manifest reconstruction independent of future generator changes. |
| `internal/install/references.go`, `references_test.go` | State-derived recorded/live reference calculation with hop-by-hop canonicalization. |
| `internal/install/collection_journal.go`, `collection_journal_test.go` | Strict durable collection-journal codec and interrupted-phase reconciliation. |
| `internal/install/collect.go`, `collect_test.go` | Candidate classification, containment revalidation, quarantine, safe deletion, and reports. |
| `internal/install/uninstall.go`, `uninstall_test.go` | All/scoped uninstall planning, preflight ownership proof, dry-run, transaction, empty state, and post-uninstall collection. |
| `internal/install/txn.go`, `txn_test.go` | Content/destination revalidation for removals immediately before apply. |
| `internal/install/service.go`, `service_test.go`, `devmode.go`, `devmode_test.go` | Shared automatic post-commit collector integration for release and development installs. |
| `internal/app/install.go`, `install_test.go` | Uninstall/collection protocol results and separation of primary outcome from cleanup warnings. |
| `internal/cli/install.go`, `root.go`, `root_test.go` | Asset-independent command wiring, flags, root resolution, and empty-state asset gate. |
| `internal/app/schema_registry.go`, `schema_registry_test.go`, `internal/cli/capability_test.go` | Operation schema/catalog correspondence for `uninstall` and `install.collect`. |
| `cmd/docket/main_test.go` | Process-level JSON/human exit semantics and byte-preserving dry-run coverage. |
| `docs/install/install.md`, `docs/install/keeping-current.md`, `docs/reference/cli.md` | Removal scope, retention, recovery, automatic cleanup warnings, and retry commands. |

---

### Task 1: Validate Installed State and Represent an Empty Installation

**Files:**
- Modify: `internal/install/state.go` (`LoadState`, new `ValidateState`, new `State.Active`)
- Modify: `internal/install/state_test.go`
- Modify: `internal/cli/install.go` (`RequireCompatibleInstallation`)
- Modify: `internal/cli/root_test.go`

**Interfaces:**
- Produces: `func ValidateState(*State) error` and `func (s *State) Active() bool`.
- `Active` is true only when at least one non-empty harness is represented by both `Harnesses` and an attributed target; binary-only/empty-harness state is valid but inactive.
- `LoadState` continues returning `(nil, nil)` only for a cleanly absent file and wraps every present-invalid document in `ErrStateInvalid`.

- [ ] **Step 1: Add failing strict-state and empty-state tests**

Add table cases that reject unknown JSON fields/trailing documents, unsupported mode/protocol, non-absolute or duplicate target paths, invalid per-kind fields, duplicate/empty harness names, attributed targets missing from `Harnesses`, harness entries with no target, and binary records carrying a harness. Add a valid empty state containing only an unattributed `roleBinary` file record and a release empty state containing no targets. Assert `Active()` is false for both and true for the existing sample state.

In `internal/cli/root_test.go`, write a valid empty state and assert an asset-dependent probe returns `installation-required`, while `install`, `install check`, and the future uninstall/collect keys remain reachable without the asset gate.

- [ ] **Step 2: Run the focused tests to verify RED**

Run: `go test -count=1 ./internal/install ./internal/cli -run 'Test(LoadStateStrict|ValidateState|StateActive|RequireCompatibleInstallationEmpty)'`

Expected: FAIL because structural validation and inactive-empty semantics do not exist.

- [ ] **Step 3: Implement strict decoding, validation, and active-state semantics**

Decode with `json.Decoder.DisallowUnknownFields`, require EOF after one document, then call `ValidateState`. Validate records by kind:

```go
func (s *State) Active() bool {
	if s == nil || len(s.Harnesses) == 0 {
		return false
	}
	have := make(map[string]bool, len(s.Harnesses))
	for _, rec := range s.Targets {
		if rec.Harness != "" {
			have[rec.Harness] = true
		}
	}
	for _, name := range s.Harnesses {
		if !have[name] { return false }
	}
	return true
}
```

Keep unknown recorded harness names structurally valid: the all-harness uninstall must be able to remove them without a current renderer. Change `RequireCompatibleInstallation` to reject `state == nil || !state.Active()` before protocol/version-tree access.

- [ ] **Step 4: Run focused tests and existing state/install checks**

Run: `go test -count=1 ./internal/install ./internal/cli -run 'Test(State|LoadState|InstallCheck|RequireCompatibleInstallation|AssetIndependent)'`

Expected: PASS; malformed present state remains distinguishable from missing state.

- [ ] **Step 5: Commit the buildable state boundary**

```bash
git add internal/install/state.go internal/install/state_test.go internal/cli/install.go internal/cli/root_test.go
git commit -m "feat(install): validate empty and active ownership state"
```

### Task 2: Publish and Prove Version-Tree Manifests, Including Frozen v1 Trees

**Files:**
- Modify: `internal/install/version.go` (`EnsureVersionTree`, replace assets-only verification with version-root proof)
- Modify: `internal/install/version_test.go`
- Create: `internal/install/legacy_v1_manifest.go`
- Create: `internal/install/legacy_v1_manifest_test.go`

**Interfaces:**
- Produces: `type ProvenVersion struct { Root string; AssetsDir string; Manifest assets.Manifest; Legacy bool }`.
- Produces: `func ProveVersionTree(root string) (ProvenVersion, error)` and `func reconstructLegacyV1Manifest(assetsDir string) (assets.Manifest, error)`.
- New version roots contain exactly `manifest.json` plus `assets/`; accepted legacy roots contain exactly `assets/` and are never rewritten merely to add the manifest.

- [ ] **Step 1: Add failing extraction-layout and proof tests**

Extend `TestEnsureVersionTreeExtractsAndVerifies` to assert canonical `manifest.json` bytes beside `assets/` and mode `0444`. Add table tests for: current manifest success; corrupt/unknown manifest; manifest ID different from directory identity; missing/changed/extra payload; extra root file; unexpected and empty directories; symlink file/directory/root; special file where supported; and an immediate candidate whose name merely resembles an asset ID.

Construct a legacy tree with no saved manifest from the exact protocol-v1 roots (`skills`, `agents`, `cursor-rules`, `.docket.example.yml`), assert reconstruction equals its computed identity, and assert unknown roots, empty directories, changed role/mode/digest, and a future live-generator root are rejected rather than incorporated.

- [ ] **Step 2: Run version tests to verify RED**

Run: `go test -count=1 ./internal/install -run 'Test(EnsureVersionTreePublishesManifest|ProveVersionTree|ReconstructLegacyV1Manifest)'`

Expected: FAIL because published roots carry only `assets/` and no standalone proof API exists.

- [ ] **Step 3: Implement atomic manifest publication and exact root proof**

During staging, write `assets.EncodeCanonical(m)` to `<staging>/manifest.json`, chmod it with the rest of the immutable files, and publish the root with the existing single rename. `ProveVersionTree` must `Lstat` the candidate root, refuse links/non-directories, validate exact root children, strictly decode/validate a saved manifest when present, require `sanitizeSegment(manifest.AssetSetID) == filepath.Base(root)`, and reuse payload verification without following links.

In `legacy_v1_manifest.go`, freeze the v1 role mapping in installer-owned constants and classify every walked path from those constants—not `assets.DefaultAllowedRoots`, `roleFor`, or generator code. Detect empty directories by recording each visited directory and requiring it to contain a known descendant. Normalize reconstructed entries to mode `0644`, compute the manifest ID, and require its sanitized ID to equal the candidate directory name.

- [ ] **Step 4: Prove compatibility and unchanged-tree reuse**

Run: `go test -count=1 ./internal/install ./internal/assets -run 'Test(EnsureVersionTree|ProveVersionTree|ReconstructLegacyV1Manifest|Manifest|Embedded)'`

Expected: PASS, including reuse of a pre-change valid legacy tree without payload writes.

- [ ] **Step 5: Mutation-test the frozen-v1 boundary**

Back up `internal/install/legacy_v1_manifest.go` to a templated temporary file. Temporarily route reconstruction through `assets.DefaultAllowedRoots()` or allow an unknown top-level directory, then run `go test -count=1 ./internal/install -run 'TestReconstructLegacyV1Manifest'`; the test must fail. Restore the backup and rerun the same command to PASS.

- [ ] **Step 6: Commit manifest proof as a complete unit**

```bash
git add internal/install/version.go internal/install/version_test.go internal/install/legacy_v1_manifest.go internal/install/legacy_v1_manifest_test.go
git commit -m "feat(install): persist and prove version manifests"
```

### Task 3: Derive the Complete Version Reference Set

**Files:**
- Create: `internal/install/references.go`
- Create: `internal/install/references_test.go`
- Modify: `internal/install/inspect.go` only if extracting the existing canonical-path helper is needed without changing its semantics

**Interfaces:**
- Produces: `type ReferenceSet map[string]struct{}` keyed by canonical version-root paths.
- Produces: `func DeriveVersionReferences(roots UserRoots, state *State) (ReferenceSet, error)`.
- Consumes: structurally validated state from Task 1 and `canonicalPath`/`linkDestination` semantics.

- [ ] **Step 1: Add failing reference fixtures**

Cover two harnesses whose target records point into different version roots after a scoped upgrade; release `AssetSetID`; development state whose source lies outside `versions/`; missing recorded targets; relative, absolute, dangling, retargeted, and multi-hop live symlinks; `/tmp` versus `/private/tmp` aliases; malformed/out-of-root reference shapes; unreadable ancestors; and a target changed from symlink to another kind. Assert both the recorded link destination and a different observed live destination are retained.

- [ ] **Step 2: Run reference tests to verify RED**

Run: `go test -count=1 ./internal/install -run 'TestDeriveVersionReferences'`

Expected: FAIL because the reference calculator does not exist.

- [ ] **Step 3: Implement canonical, fail-closed reference derivation**

Canonicalize `roots.VersionsDir()` first. For `AssetSetID`, every recorded `Path`, every recorded `LinkTarget`, and the live destination of each still-symlink target, canonicalize existing ancestors/hops and recognize a reference only when the resolved path is a strict descendant of exactly one immediate non-symlink version root. Preserve recorded references even when targets are missing. Return an error—not an empty set—on unreadable state, invalid shapes, loops, or resolution failures that leave reachability uncertain. Ignore proven external development source paths as non-candidates.

- [ ] **Step 4: Run focused tests and canonical-link regressions**

Run: `go test -count=1 ./internal/install -run 'Test(DeriveVersionReferences|CanonicalPath|LinkDestination|InstallScoped)'`

Expected: PASS.

- [ ] **Step 5: Mutation-test reference preservation**

Back up `references.go`. In three separate mutations, remove (a) recorded `LinkTarget` inclusion, (b) observed live-link inclusion, and (c) the strict containment check. Each mutation must make its named `TestDeriveVersionReferences...` case fail under `go test -count=1`. Restore the backup between mutations and finish with the focused suite green.

- [ ] **Step 6: Commit the reference calculator**

```bash
git add internal/install/references.go internal/install/references_test.go internal/install/inspect.go
git commit -m "feat(install): derive version references from ownership state"
```

### Task 4: Add a Durable, Strict Collection Journal

**Files:**
- Modify: `internal/install/roots.go`, `roots_test.go`
- Create: `internal/install/collection_journal.go`
- Create: `internal/install/collection_journal_test.go`

**Interfaces:**
- Produces root methods `CollectionDir()`, `CollectionJournalPath()`, and `CollectionQuarantineDir()`.
- Produces `collectionJournal` with format version, original asset identity, canonical manifest, source/quarantine paths, and phase (`prepared`, `quarantined`, `deleting`).
- Produces strict `loadCollectionJournal`, atomic `writeCollectionJournal`, and `reconcileCollectionJournal` helpers. These never call the installation rollback `Recover` path.

- [ ] **Step 1: Add failing journal codec and phase-recovery tests**

Test strict unknown-field/version/phase/path refusal, canonical round trips, no torn journal on rename failure, interruption before quarantine rename, response loss after rename, partial verified-entry deletion, already-absent recorded entries, changed/foreign quarantine content, and journal removal only after owned entries and owned directories are gone.

- [ ] **Step 2: Run journal tests to verify RED**

Run: `go test -count=1 ./internal/install -run 'Test(CollectionJournal|ReconcileCollectionJournal|CollectionRoots)'`

Expected: FAIL because no collection journal exists.

- [ ] **Step 3: Implement the journal state machine**

Use one same-directory staged file and rename for publication. Reconciliation rules are deterministic:

```text
prepared + source present + quarantine absent  => revalidate, rename, publish quarantined
prepared + source absent + quarantine present  => publish quarantined (rename response was lost)
quarantined/deleting                            => verify each remaining path, delete owned leaves bottom-up
any unexpected source/quarantine combination   => retain both and return a pending-cleanup error
```

Deletion enumerates manifest files and derived directories, uses `Lstat`, refuses symlinks/special files/changed bytes, accepts cleanly absent recorded entries on retry, removes directories only after proven empty, and never uses `os.RemoveAll` on a published or quarantined candidate.

- [ ] **Step 4: Run journal interruption matrix**

Run: `go test -count=1 ./internal/install -run 'Test(CollectionJournal|ReconcileCollectionJournal|CollectionRoots)'`

Expected: PASS for every injected failure phase; foreign quarantine bytes remain intact with the journal retained.

- [ ] **Step 5: Commit the resumable journal**

```bash
git add internal/install/roots.go internal/install/roots_test.go internal/install/collection_journal.go internal/install/collection_journal_test.go
git commit -m "feat(install): journal resumable version collection"
```

### Task 5: Build the Explicit Collector and Read-Only Dry-Run Lock Path

**Files:**
- Modify: `internal/install/lock.go`, `lock_test.go`
- Create: `internal/install/collect.go`
- Create: `internal/install/collect_test.go`

**Interfaces:**
- Produces `type CollectOptions struct { Roots UserRoots; FS FSOps; DryRun bool }`.
- Produces `type CollectionEntry struct { AssetSetID, Path, Status, Detail string }` with statuses `collected`, `referenced`, `unverified`, and `failed`.
- Produces `type CollectionOutcome struct { Applied bool; Entries []CollectionEntry; Pending []string; Err error }`.
- Produces `func Collect(CollectOptions) CollectionOutcome` and locked helper `collectLocked(CollectOptions, *installLock) CollectionOutcome`.
- Produces a no-create dry-run lock/consistency probe; actual collection uses `acquireInstallLock`.

- [ ] **Step 1: Add failing candidate-pass and dry-run tests**

Cover a fresh machine, missing state with existing versions, corrupt/unknown state, referenced and eligible current/legacy trees, symlink and non-directory candidates, malformed candidates, unreadable candidates, failed proof, failed quarantine rename, partial delete, lock contention, and retry completion. Snapshot the whole temp home before/after dry-run—including directory existence and modes—and assert byte identity and no lock/journal creation.

- [ ] **Step 2: Run collector tests to verify RED**

Run: `go test -count=1 ./internal/install -run 'Test(Collect|DryRunCollection|ReadOnlyInstallLock)'`

Expected: FAIL because collection and no-create lock inspection do not exist.

- [ ] **Step 3: Implement deterministic candidate classification and execution**

Under the actual lock: reconcile an existing collection journal first; require no pending rollback installation journal; load and validate state; derive references; enumerate only immediate `ReadDir` children of the canonical versions root; classify every entry before deleting the next eligible tree; and sort reports by canonical path. For each eligible candidate, call `ProveVersionTree`, recompute state/references immediately before quarantine, re-prove the tree, write the journal, rename into the private same-filesystem quarantine, then reconcile deletion.

For dry-run, open/flock only an already-existing lock file and never create a directory or file. A fresh machine is a clean no-op. If mutable state exists but no lock can be observed consistently, or a rollback/collection journal is pending, report the recovery/uncertainty without changing it.

- [ ] **Step 4: Run collector, race-sensitive lock, and version tests**

Run: `go test -count=1 ./internal/install -run 'Test(Collect|DryRunCollection|ReadOnlyInstallLock|InstallLock|ProveVersionTree)'`

Expected: PASS.

- [ ] **Step 5: Mutation-test deletion authority**

Back up `collect.go`. Mutate one guard at a time: bypass the reference-set check, bypass candidate re-proof, weaken strict containment to prefix matching, and replace an `Lstat` kind check with acceptance. Run each named destructive fixture with `go test -count=1`; each must fail and show the protected tree/content was wrongly removed. Restore from backup after each mutation and rerun all collector tests green.

- [ ] **Step 6: Commit the collector**

```bash
git add internal/install/lock.go internal/install/lock_test.go internal/install/collect.go internal/install/collect_test.go
git commit -m "feat(install): collect verified unreferenced version trees"
```

### Task 6: Implement Transactional All/Scoped Harness Uninstall

**Files:**
- Create: `internal/install/uninstall.go`
- Create: `internal/install/uninstall_test.go`
- Modify: `internal/install/txn.go` (`verifyPreImages` and removal apply checks)
- Modify: `internal/install/txn_test.go`

**Interfaces:**
- Produces `type UninstallOptions struct { Roots UserRoots; FS FSOps; Harnesses []string; SupportedHarnesses []string; DryRun bool }`.
- Produces `func Uninstall(UninstallOptions) Outcome` using the existing `Outcome.Actions` vocabulary plus collection details carried separately at the app boundary.
- Consumes: Task 1 state validation, Task 4/5 collector, and `BeginTxnWithRemovals`.

- [ ] **Step 1: Add failing uninstall planning and transaction tests**

Create four-harness state fixtures and cover: full removal; selected removal; duplicate filters; all filters validated before mutation; supported-but-unrecorded no-op; unknown explicit filter refusal; structurally valid unknown recorded harness removed by all-harness mode; unattributed binary retained; configuration/source/repository files retained; missing target/block satisfied; edited file; malformed/changed block; changed kind; canonical/dangling/repointed link; all conflicts accumulated; idempotent repeat; empty-state publication; development-binary provenance retention; and reinstall over empty state.

In `txn_test.go`, add deterministic seams that change file bytes, symlink destination, and kind after inspection/capture and immediately before apply. Assert no earlier removal occurs and the journal is discarded without rollback deleting the intruder. Add synchronous apply/commit rollback failure and abrupt interruption recovery cases.

- [ ] **Step 2: Run uninstall/transaction tests to verify RED**

Run: `go test -count=1 ./internal/install -run 'Test(Uninstall|RemovalPreImage|VerifyPreImages|Recovery.*Removal|ReinstallEmptyState)'`

Expected: FAIL; existing `verifyPreImages` explicitly exempts whole-file removals from content checks.

- [ ] **Step 3: Implement read-only uninstall preflight**

Resolve all selected harness names first. Build removals only from `State.Targets`, excluding unattributed records. For each selected record: require a structurally valid kind; treat clean absence (or an already-absent valid managed block) as satisfied; otherwise call record-level proof so file digest, canonical link destination—including dangling handling—or managed-block interior digest authorizes removal. Gather every conflict before starting a transaction.

Build the desired state by copying untouched targets and identity/provenance fields. When no harness targets remain, set `Harnesses` empty and clear `AssetSetID`; preserve binary records plus development `Mode`, `SourceRoot`, `SourceDigest`, `ProductVersion`, `AssetProtocol`, and `AgentDigest` needed to prove them.

- [ ] **Step 4: Strengthen transaction removal revalidation**

Capture enough pre-image identity in the journal (file digest, link destination, or managed-block interior/whole pre-image) to compare content as well as kind. `verifyPreImages` must validate every removal against the captured pre-image. Immediately before each destructive step, revalidate the same proof against disk; fail before mutation when it changed. Keep the documented residual limited to mutation between the last check and the supported filesystem primitive.

- [ ] **Step 5: Apply state and collect under one lock**

Actual uninstall acquires the existing lock, recovers abandoned install transactions, verifies no pending journal remains, preflights all targets, commits removals and desired state through `BeginTxnWithRemovals`, and then calls `collectLocked` before releasing the lock. Dry-run uses the no-create path from Task 5, performs the same read-only plan, reports pending recovery, and creates no journal/state/lock.

- [ ] **Step 6: Run uninstall and transaction recovery matrices**

Run: `go test -count=1 ./internal/install -run 'Test(Uninstall|RemovalPreImage|VerifyPreImages|Recovery.*Removal|ReinstallEmptyState)'`

Expected: PASS; repeat uninstall is a no-op, and collector failure does not roll a successful uninstall back.

- [ ] **Step 7: Mutation-test ownership gates**

Back up `uninstall.go` and `txn.go`. Separately disable file-digest proof, link-destination proof, managed-block digest proof, and apply-time pre-image revalidation. Each mutation must make its dedicated drift/race fixture fail under `go test -count=1`. Restore from backups and rerun the complete focused command green.

- [ ] **Step 8: Commit uninstall and strengthened removal safety**

```bash
git add internal/install/uninstall.go internal/install/uninstall_test.go internal/install/txn.go internal/install/txn_test.go
git commit -m "feat(install): uninstall recorded harness integrations safely"
```

### Task 7: Wire Protocol Results, CLI Commands, Schemas, and Catalog Entries

**Files:**
- Modify: `internal/app/install.go`, `install_test.go`
- Modify: `internal/cli/root.go`, `root_test.go`
- Modify: `internal/cli/install.go`
- Modify: `internal/app/schema_registry.go`, `schema_registry_test.go`
- Modify: `internal/cli/capability_test.go`
- Modify: `cmd/docket/main_test.go`

**Interfaces:**
- Adds operation constants `uninstall` and `install.collect`.
- Adds app entry points `RunUninstall(install.UninstallOptions)` and `RunInstallCollect(install.CollectOptions)`.
- Result JSON distinguishes the primary `result`/`applied_work` from collection entries and warnings; warnings include pending paths and exact retry command `docket install collect`.

- [ ] **Step 1: Add failing app and command-contract tests**

Assert both operations appear exactly once in capabilities and schemas with `local-write` effects and the expected flags. Assert `assetIndependent` includes command groups/leaves in both directions. Process-level cases cover human and JSON success, explicit collection nonzero on unverified/failed candidates, automatic-warning-shaped output, full/scoped harness flags, invalid filters, no record semantics, `--json` placement, and dry-run byte preservation.

- [ ] **Step 2: Run protocol and CLI tests to verify RED**

Run: `go test -count=1 ./internal/app ./internal/cli ./cmd/docket -run 'Test(Uninstall|InstallCollect|InstallResult|Capabilities|OperationBindings|InstallCommands|AssetIndependent)'`

Expected: FAIL because neither operation is registered.

- [ ] **Step 3: Extend result types without conflating primary and cleanup status**

Add optional `collection` entries and `warnings` whose collection warning shape names `pending_paths` and `retry: "docket install collect"`. For explicit collection, classify any unverified/failed/pending candidate as non-success and exit nonzero. For automatic collection, preserve the already-computed install/uninstall envelope result and append warnings. Human output must say that the CLI binary and repository setup remain installed after uninstall.

- [ ] **Step 4: Register Cobra leaves and operation schemas**

Add `collect` beneath `install` and top-level `uninstall`; both use `install.ResolveRoots(os.UserHomeDir, os.Getenv)` directly and never call `installOptions`, config loading, repository discovery, planners, renderers, or compatible-asset gating. Register repeatable `--harness` and `--dry-run` only on uninstall, `--dry-run` on collect, annotate stable operation IDs, add schema bindings, and update `assetIndependent` so parent groups and executable leaves maintain exact correspondence.

- [ ] **Step 5: Run all protocol and CLI tests**

Run: `go test -count=1 ./internal/app ./internal/cli ./cmd/docket -run 'Test(Uninstall|InstallCollect|InstallResult|Capabilities|OperationBindings|InstallCommands|AssetIndependent|Schema)'`

Expected: PASS; JSON emits one protocol-v1 document and explicit failure exits nonzero.

- [ ] **Step 6: Mutation-test command correspondence**

Back up the touched registration files. Remove each new capability annotation, schema binding, and asset-independent entry one at a time; the existing whole-tree correspondence test must fail for each under `go test -count=1 ./internal/cli ./internal/app`. Restore and rerun green.

- [ ] **Step 7: Commit the public command surface**

```bash
git add internal/app/install.go internal/app/install_test.go internal/cli/root.go internal/cli/root_test.go internal/cli/install.go internal/app/schema_registry.go internal/app/schema_registry_test.go internal/cli/capability_test.go cmd/docket/main_test.go
git commit -m "feat(cli): add uninstall and install collect commands"
```

### Task 8: Invoke Automatic Collection After Successful Installs

**Files:**
- Modify: `internal/install/service.go`, `service_test.go`
- Modify: `internal/install/devmode.go`, `devmode_test.go`
- Modify: `internal/app/install.go`, `install_test.go`

**Interfaces:**
- Adds a shared locked post-commit helper used by release `Install`, development-install candidate, and `Uninstall`.
- The helper runs after `applyPlan` has either committed or proved the state unchanged, while the caller still owns the install lock.
- `Outcome` carries collection report/warnings separately from `Err`/`Reason`, so collection cannot reclassify successful primary work.

- [ ] **Step 1: Add failing automatic-collection tests**

Cover successful changed install, otherwise-unchanged install, successful development release transition, successful development install, successful uninstall, refused install, failed install, lock contention, pending rollback journal, and injected collector failure. Assert cleanup starts only after state publication and after rollback journals are gone. Assert old mixed-version trees remain while referenced and are collected only after their last owner disappears.

- [ ] **Step 2: Run service/development tests to verify RED**

Run: `go test -count=1 ./internal/install ./internal/app -run 'Test(Install.*Collect|DevInstall.*Collect|AutomaticCollection|CollectionWarning)'`

Expected: FAIL because install returns immediately after `applyPlan`.

- [ ] **Step 3: Refactor the locked tail while keeping every intermediate state buildable**

Have release `Install` and `developmentInstallCandidate` retain the lock across `applyPlan` and a `collectLocked` call. Invoke collection when the primary outcome has no error, including `Applied == false`; never invoke it after preflight, extraction, planning, transaction, or state-publication failure. Merge collection actions/warnings into the outcome without setting primary `Err`/`Reason`. Keep the development parent a pure relay: only the candidate performs and reports collection.

- [ ] **Step 4: Run full installer package tests**

Run: `go test -count=1 ./internal/install ./internal/app ./internal/cli`

Expected: PASS, including unchanged-install cleanup and separation of successful install from collection warning.

- [ ] **Step 5: Mutation-test the post-commit ordering gate**

Back up `service.go` and `devmode.go`. Move collection before `applyPlan`, allow it after a refused plan, and skip it on the unchanged path as three separate mutations. Each dedicated ordering fixture must fail with `go test -count=1`. Restore and finish with all installer packages green.

- [ ] **Step 6: Commit automatic collection**

```bash
git add internal/install/service.go internal/install/service_test.go internal/install/devmode.go internal/install/devmode_test.go internal/app/install.go internal/app/install_test.go
git commit -m "feat(install): collect stale versions after successful installs"
```

### Task 9: Document the Lifecycle and Run End-to-End Safety Verification

**Files:**
- Modify: `docs/install/install.md`
- Modify: `docs/install/keeping-current.md`
- Modify: `docs/reference/cli.md`
- Modify only if coverage requires a process fixture extension: `cmd/docket/main_test.go`

**Interfaces:**
- Documents executable commands and exact retention/recovery semantics already implemented by Tasks 1–8.
- Supplies the final build evidence and ADR-decision inputs to the parent workflow; it does not mutate the metadata branch.

- [ ] **Step 1: Update user documentation**

Document all/scoped uninstall, deduplicated filters, dry-run, ownership refusals, valid empty state, idempotence, retained CLI/config/source/repository setup, automatic collection after successful/no-op installs and uninstall, explicit retry, report categories, unsupported direct references into `versions/`, interrupted collection recovery, and the fact that automatic cleanup warnings do not undo installation success. In `docs/reference/cli.md`, list both capability operations and flags.

- [ ] **Step 2: Add bounded prose-contract assertions where existing doc guards live**

Bind each assertion to its subject and section, not a bare phrase: retained CLI/repository setup after uninstall; explicit retry command only on collection warning; and install success separate from cleanup completion. Collapse whitespace before matching wrapped prose and assert each named section terminator exists.

- [ ] **Step 3: Run focused package and process tests**

Run: `go test -count=1 ./internal/install ./internal/app ./internal/cli ./cmd/docket ./internal/repoguard`

Expected: PASS with no cache-served mutation evidence.

- [ ] **Step 4: Perform a hermetic four-harness lifecycle smoke test**

Using a temporary `HOME`, `XDG_DATA_HOME`, `XDG_CONFIG_HOME`, and `XDG_BIN_HOME`: install all four harnesses; create a second valid version tree and retarget/record one harness to it; uninstall one owner; prove both referenced trees survive; uninstall the final owners; prove only unreferenced verified trees are collected; prove the CLI binary, config, source checkout, and repository-local files remain; reinstall and run `install check`. Record command outputs and filesystem snapshots in build evidence.

- [ ] **Step 5: Verify metadata-branch and real-history facts outside the hermetic suite**

Read `/Users/homer/dev/docket/.docket/docs/changes/active/0323-docket-uninstall-and-version-tree-collection-for-the-go-inst.md` and confirm it still links ADR-0096 and ADR-0110 and that the synchronized approved spec is unchanged. Record this named manual verification in the results file because the feature worktree suite cannot see metadata-branch artifacts. Supply the three decision summaries—state-derived references, post-commit collection, retained CLI—to the parent workflow's normal `docket-adr` step; do not create or edit metadata files from this worktree.

- [ ] **Step 6: Run the authoritative full build gate**

Run: `go run ./cmd/docket development test`

Expected: exit 0 with every suite target reporting a result. Inspect the complete output for `BUDGET WATCH:`, `PARALLEL-SENSITIVE:`, and `SERIAL CONFIRMED OVER BUDGET:`. A serial-confirmed breach must be fixed by splitting/reducing the relevant test shard rather than raising its budget; a green run with advisory lines must still record those lines in build evidence.

- [ ] **Step 7: Review the whole diff for destructive-boundary completeness**

Derive all executable deletion sites with a whole-repository `rg` over `Remove`, `RemoveAll`, rename-to-quarantine, and managed-block removal calls. Classify prose/fixtures separately, then verify every new executable site is dominated by ownership, reference, containment, and kind proof as applicable. Confirm no collector calls `os.RemoveAll` on candidates, no uninstall/collect path loads config or repository state, and no accepted ADR was rewritten.

- [ ] **Step 8: Commit documentation and any final contract tests**

```bash
git add docs/install/install.md docs/install/keeping-current.md docs/reference/cli.md cmd/docket/main_test.go
git commit -m "docs(install): explain uninstall and version collection"
```

## Self-Review Checklist

- Spec coverage: Tasks 1–9 cover the two public operations, empty state/reinstall, ownership preflight, transaction race hardening, complete references, current and frozen-v1 proof, durable quarantine cleanup, dry-run, automatic/explicit failure semantics, CLI/schema/catalog wiring, docs, mutation tests, and the full gate.
- Placeholder scan: every implementation task names exact files, symbols, tests, commands, expected outcomes, and commit boundaries; no deferred implementation step remains.
- Type consistency: `ValidateState`/`State.Active` feed all callers; `ProveVersionTree` feeds `Collect`; `DeriveVersionReferences` returns canonical version roots consumed by `Collect`; `collectLocked` is shared by install/development/uninstall; app results carry the same `CollectionEntry` vocabulary used by explicit and automatic reports.
- Buildability: every commit closes its own package tests; automatic integration occurs only after state, proof, references, journal, collector, uninstall, and public result types exist.
- Non-hermetic truth: metadata links and decision recording are explicit build-time checks owned by the parent workflow, not false unit-test oracles.
