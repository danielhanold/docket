<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0378 — Shared metadata-root classifier misreads any multi-commit docket branch as foreign](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0378-metadata-root-classifier-rejects-multi-commit-docket-branch.md)**
<!-- docket:backlink:end -->
# Metadata-Root Ownership Verifier Implementation Plan (change 0378)

> **For agentic workers:** REQUIRED SUB-SKILL: this repo's build role (`docket-build`) executes this plan task-by-task via profile workers under the `docket-build-task` contract. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the three faulty root-equals-tip ownership predicates with one shared metadata-root ownership verifier so a docket branch stays recognizable after ordinary metadata commits, without admitting foreign branches or letting migration replace a mature branch.

**Architecture:** A new app-layer verifier (`internal/app/metadata_ownership.go`, beside `repository_facts.go`) reads immutable Git objects through `internal/gitcli` and decides ownership at the **root** of the sole parentless lineage: native `OpInitRoot`/`OpMigrateSeed` receipt-and-tree proofs, or receiptless legacy exact-tree equivalence against historical integration snapshots. `internal/reposetup` stays pure and gitcli-free: it gains the deterministic seed-receipt verdict and keeps the `RootShape` vocabulary (`RootParentless` = verified docket seed root with permitted descendants; `RootUnknown` vs `RootForeign` preserves unreadable-vs-unproven). Check, init, and migrate consume the one verifier while keeping their own create-only, lease, and recovery guards; the seed-replacement path in `reconcileResumeSeed` gains a descendants guard so a broadened proof can never discard later commits.

**Tech Stack:** Go (module `github.com/danielhanold/docket`), `internal/gitcli` subprocess adapter, real-Git integration tests behind the `integration` build tag, Bash shard runners under `tests/` driven by `go run ./cmd/docket development test`.

**Spec:** `docs/superpowers/specs/2026-08-30-metadata-root-classifier-rejects-multi-commit-docket-branch-design.md` (on the `docket` metadata branch; readable in this repo at `.docket/docs/superpowers/specs/…` from the primary worktree). The plan argues from the spec; executors read both.

## Global Constraints

- `internal/reposetup` stays **pure and gitcli-free** — no gitcli import, no I/O. Git execution stays inside `internal/gitcli`; **no shell assembly or CLI-text parsing in `internal/app`**.
- Every probe error maps to Unknown with the error retained — **never** collapsed into absence or into "foreign" (learning `probe-error-is-not-clean-absence`). Never interpret a shallow boundary as a parentless root; never fall back from an errored probe to a weaker proof; never report foreign from a truncated search.
- A receipt claiming docket provenance but failing validation is **foreign, never downgraded** to the receiptless legacy path.
- No public command, config key, protocol version, persistent ownership cache, new marker, root allowlist, or blanket adopt override. The live OIDs in the spec (`f8b226f2…`, `1e7493a6…`) are **evidence, never production allowlists or test constants**.
- Historical comparison is content-read-only: no checkout of old commits, no metadata rewrite, no real-index edit, no compatibility receipts. (`gitcli.BuildTree` uses its own temp `GIT_INDEX_FILE` and writes only loose objects — that is within the read-only contract; ref/index/worktree mutation is not.)
- Do **not** modify change 0377 (branch, halt record, tasks, frozen plan), do not delete the frozen Bash facade (`scripts/`), do not touch 0370.
- Whole-suite gate: resolve the command from `finalize.test_command` at build time (currently `go run ./cmd/docket development test`); run it at the end of the build, never only the enumerated tests. Defeat Go's test cache in every mutation probe and manual re-verification (`-count=1`, learning `cached-runner-serves-a-mutated-tree`).
- New/changed test shards must keep `tests/runtime-budgets.tsv` consistent: a new file gets its own measured row and the `EXPECTED_TOTAL` in `tests/test_runtime_budgets.sh` moves with it (say which case in the diff). Never grow a file past its ceiling and raise the number.
- Mutation-testing restores: back up the uncommitted file (`cp file file.bak`) before mutating and restore from the backup — **never** `git checkout -- file` (learning `mutation-restore-needs-a-backup-copy`).
- Commit per task on this branch (`fix/metadata-root-classifier-rejects-multi-commit-docket-branch`); stage only your own paths, never `git add -A`.

## File Structure

- `internal/reposetup/receipt.go` (modify) + `internal/reposetup/receipt_test.go` — pure seed-receipt verdict `EvaluateSeedTrailers` over the root's raw trailer scan.
- `internal/reposetup/probe.go` (modify) — `RootShape` doc clarification only (no value changes).
- `internal/gitcli/history.go` (create) + `internal/gitcli/history_integration_test.go` (create) — narrow typed reads: `HasSharedAncestry`, `ListHistoryTrees`, `TreeEntryIDs`.
- `internal/app/metadata_ownership.go` (create) + `internal/app/repoownership_integration_test.go` (create) — the shared verifier and the legacy historical-snapshot search.
- `internal/app/repository_check.go`, `internal/app/repository_init.go`, `internal/app/repository_migrate.go`, `internal/app/repository_facts.go` (modify) — consumers; `metadataRootParentless` removed.
- `tests/test_go_integration_app_repoownership.sh` (create), `tests/test_go_integration_gitcli_history.sh` (create), `tests/runtime-budgets.tsv` + `tests/test_runtime_budgets.sh` (modify) — shard runners and budget registry.

The Go integration completeness contract (`tests/test_go_integration_contract.sh`) discovers shards structurally — a new shard needs no contract edit, but its test prefix must match **exactly one** runner and no existing prefix may be a name-prefix of the new tests (existing app prefixes: `TestIntegrationRepoSetup`, `TestIntegrationRepoCheck`, `TestIntegrationRepoMigration`, `TestIntegrationRepoRecovery`, `TestIntegrationRepoContention`, `TestRaceIntegrationRepoSetup`, plus non-repo families). `TestIntegrationRepoOwnership` and `TestIntegrationHistory` (gitcli) collide with none — verify with `grep -h "SHARD_PREFIX" tests/test_go_integration_*.sh` before committing.

---

### Task 1: Pure seed-receipt verdict in reposetup

**Files:**
- Modify: `internal/reposetup/receipt.go`
- Modify: `internal/reposetup/probe.go` (comments only)
- Test: `internal/reposetup/receipt_test.go`

**Interfaces:**
- Consumes: existing `Trailer`, `Receipt`, `ParseReceipt`, `OpInitRoot`, `OpMigrateSeed`, `OpMigratePrune`, trailer-key constants, `hasControlByte`.
- Produces: `type SeedVerdict int` with constants `SeedAbsent`, `SeedInvalid`, `SeedInit`, `SeedMigrate`; `func EvaluateSeedTrailers(trailers []Trailer) (Receipt, SeedVerdict)`. Task 3 consumes exactly this signature. `ParseReceipt` itself is **unchanged** (the prune-receipt readers keep last-wins tolerance; seed strictness lives in the new function).

- [ ] **Step 1: Write the failing tests.** Add to `receipt_test.go` a table-driven `TestEvaluateSeedTrailers` covering, at minimum:
  - no recognized `Docket-*` receipt trailer at all → `SeedAbsent` (legacy path eligible);
  - a recognized receipt trailer present but no `Docket-Operation` (e.g. only `Docket-Copy-Digest`) → `SeedInvalid` (docket-claiming, never legacy);
  - duplicated `Docket-Operation`, and separately any duplicated recognized key (e.g. two `Docket-Source-Revision`) → `SeedInvalid`;
  - `Docket-Operation: repository-migrate-prune/v1` on the root → `SeedInvalid` (a prune receipt is not a seed receipt);
  - unknown operation / unsupported version (`repository-init-root/v2`) → `SeedInvalid`;
  - control byte in any recognized value → `SeedInvalid`;
  - valid `OpInitRoot` (operation only, exactly the emitted format) → `SeedInit`; `OpInitRoot` carrying any migrate-only field (`Docket-Source-Revision`, `Docket-Copy-Digest`, `Docket-Repair-Digest`, `Docket-Metadata-Revision`) → `SeedInvalid`;
  - valid `OpMigrateSeed` with non-empty `Docket-Source-Revision`, `Docket-Copy-Digest`, `Docket-Repair-Digest` and **no** `Docket-Metadata-Revision` → `SeedMigrate` with all three fields preserved on the returned `Receipt` (the repair digest's meaning is preserved — its value is carried through untouched, not compared to anything here);
  - `OpMigrateSeed` missing any of the three required fields, or carrying `Docket-Metadata-Revision` → `SeedInvalid`;
  - unrecognized non-docket trailers (e.g. `Signed-off-by`) mixed in → ignored, do not affect the verdict.

```go
func TestEvaluateSeedTrailersMigrateValid(t *testing.T) {
	rec, v := reposetup.EvaluateSeedTrailers([]reposetup.Trailer{
		{Key: "Signed-off-by", Value: "someone"},
		{Key: reposetup.TrailerOperation, Value: reposetup.OpMigrateSeed},
		{Key: reposetup.TrailerSourceRevision, Value: "1111111111111111111111111111111111111111"},
		{Key: reposetup.TrailerCopyDigest, Value: "2222222222222222222222222222222222222222"},
		{Key: reposetup.TrailerRepairDigest, Value: "deadbeef"},
	})
	if v != reposetup.SeedMigrate {
		t.Fatalf("verdict = %v, want SeedMigrate", v)
	}
	if rec.SourceRevision == "" || rec.CopyDigest == "" || rec.RepairDigest != "deadbeef" {
		t.Fatalf("receipt fields not preserved: %+v", rec)
	}
}
```

- [ ] **Step 2: Run to verify failure.** `go test ./internal/reposetup/ -run TestEvaluateSeedTrailers -count=1` → FAIL (undefined: `EvaluateSeedTrailers`).

- [ ] **Step 3: Implement.** In `receipt.go`:

```go
// SeedVerdict is the deterministic decision over a ROOT commit's raw trailer
// scan: is this a valid native seed receipt, an invalid docket-claiming one
// (foreign — never downgraded to the receiptless legacy path), or no receipt
// at all (legacy path eligible)? It is stricter than ParseReceipt on purpose:
// duplicated recognized fields, a prune receipt on the root, an unsupported
// operation version, and operation-inappropriate fields are all invalid here
// even where a last-wins reader would tolerate them.
type SeedVerdict int

const (
	SeedAbsent  SeedVerdict = iota // no recognized receipt trailer: legacy eligible
	SeedInvalid                    // docket-claiming but not a valid seed receipt
	SeedInit                       // valid OpInitRoot receipt (operation only)
	SeedMigrate                    // valid OpMigrateSeed receipt (source+copy+repair)
)

// EvaluateSeedTrailers decides the seed verdict from the root commit's full
// trailer set. Unrecognized keys are ignored; any recognized key seen more
// than once, or carrying a control byte, is invalid.
func EvaluateSeedTrailers(trailers []Trailer) (Receipt, SeedVerdict) {
	counts := map[string]int{}
	var r Receipt
	for _, t := range trailers {
		var dst *string
		switch t.Key {
		case TrailerOperation:
			dst = &r.Operation
		case TrailerSourceRevision:
			dst = &r.SourceRevision
		case TrailerMetadataRev:
			dst = &r.MetadataRevision
		case TrailerCopyDigest:
			dst = &r.CopyDigest
		case TrailerRepairDigest:
			dst = &r.RepairDigest
		default:
			continue
		}
		counts[t.Key]++
		if counts[t.Key] > 1 || hasControlByte(t.Value) {
			return Receipt{}, SeedInvalid
		}
		*dst = t.Value
	}
	if len(counts) == 0 {
		return Receipt{}, SeedAbsent
	}
	switch r.Operation {
	case OpInitRoot:
		if r.SourceRevision != "" || r.MetadataRevision != "" || r.CopyDigest != "" || r.RepairDigest != "" {
			return Receipt{}, SeedInvalid
		}
		return r, SeedInit
	case OpMigrateSeed:
		if r.SourceRevision == "" || r.CopyDigest == "" || r.RepairDigest == "" || r.MetadataRevision != "" {
			return Receipt{}, SeedInvalid
		}
		return r, SeedMigrate
	default:
		// No operation, a prune receipt, or an unknown/unsupported version:
		// docket-claiming trailers with no valid seed operation.
		return Receipt{}, SeedInvalid
	}
}
```

- [ ] **Step 4: Clarify `RootShape` docs** in `probe.go` (values unchanged): `RootParentless` — "a verified docket seed root with permitted descendants and merges sharing that root; the root need not equal the tip"; `RootForeign` — "readable, exhausted evidence, no ownership proof"; `RootUnknown` — "incomplete or unreadable evidence — never collapsed into foreign". Remove any "expected receipt or exact legacy-equivalent tree **at the tip**" implication.

- [ ] **Step 5: Run and verify pass.** `go test ./internal/reposetup/ -count=1` → PASS.

- [ ] **Step 6: Commit.** `git add internal/reposetup/receipt.go internal/reposetup/receipt_test.go internal/reposetup/probe.go && git commit` — subject `feat(0378): pure seed-receipt verdict for root ownership (reposetup)`.

---

### Task 2: gitcli narrow history reads

**Files:**
- Create: `internal/gitcli/history.go`
- Test: `internal/gitcli/history_integration_test.go` (`//go:build integration` on line 1, blank line 2)
- Create: `tests/test_go_integration_gitcli_history.sh`
- Modify: `tests/runtime-budgets.tsv`, `tests/test_runtime_budgets.sh` (`EXPECTED_TOTAL`)

**Interfaces:**
- Consumes: `Client.run`, `runRequest`, `newFailure`, `validateObjectID`, `stdoutLines`, existing `Operation`/`Failure` machinery (follow `setuptree.go`'s style), `IsAncestor` (exists in `push.go` — do not duplicate it).
- Produces (Task 3/4 consume exactly these):
  - `func (c *Client) HasSharedAncestry(ctx context.Context, repo Repository, a, b ObjectID) (bool, error)` — `git merge-base a b`: exit 0 → true; exit 1 → false; anything else → error. Never collapses an error into false.
  - `type HistoryEntry struct { Commit, Tree ObjectID }`
  - `func (c *Client) ListHistoryTrees(ctx context.Context, repo Repository, tip ObjectID) ([]HistoryEntry, error)` — one `git rev-list --format=%H %T <tip>` walk over the **complete** reachable history (no depth window, no first-parent), newest-first, each commit paired with its root tree OID (the batched read the dedupe keys on).
  - `func (c *Client) TreeEntryIDs(ctx context.Context, repo Repository, commit ObjectID, paths []RepoPath) ([]TreeEntry, error)` — non-recursive `git ls-tree <commit> -- <paths...>` returning tree **and** blob entries with mode/type/OID/path (unlike `ObjectSource.ListTree`, which lists recursive leaves). Absent paths simply yield no entry — not an error. Reuse `parseLsTreeZ`-style `-z` parsing but accept `tree` entries.

- [ ] **Step 1: Write failing integration tests** (`TestIntegrationHistory…` prefix) in a local fixture repo (follow `setuptree_integration_test.go`'s fixture idiom): linear history → `ListHistoryTrees` returns all commits newest-first with correct `%T` values; disjoint orphan branch vs main → `HasSharedAncestry` false; same-root branches → true; a commit and itself → true; invalid/absent OID → error (and assert the error is an error, not `false`); `TreeEntryIDs` on a commit with `docs/x/f.txt` returns the `docs` (or requested prefix) subtree OID and skips absent paths.
- [ ] **Step 2: Run to verify failure.** `go test -tags integration ./internal/gitcli/ -run TestIntegrationHistory -count=1` → FAIL (undefined methods).
- [ ] **Step 3: Implement `history.go`** per the signatures above, mirroring `setuptree.go`'s failure classification (`KindInvalidRequest` for malformed OIDs, `KindCommandFailed` with stderr excerpt, `KindInvalidOutput` for unparseable output). For `HasSharedAncestry`, only exit code 1 with empty stderr-classified "no merge base" is a clean false — any other nonzero exit is an error.
- [ ] **Step 4: Run to verify pass.** Same command → PASS.
- [ ] **Step 5: Shard runner + budget row.** Create `tests/test_go_integration_gitcli_history.sh` by copying `tests/test_go_integration_gitcli_setuptree.sh` verbatim and changing only the header comment, `SHARD_PKG="./internal/gitcli"`, `SHARD_PREFIX="TestIntegrationHistory"`, `SHARD_MODE="normal"`. Measure cold three times (`fresh=$(mktemp -d "${TMPDIR:-/tmp}/gocache.XXXXXX"); GOCACHE=$fresh /usr/bin/time -p bash tests/test_go_integration_gitcli_history.sh >/dev/null`), take the worst, round up to the next multiple of 5 and add 5 (the table's house rule — read the header comments in `tests/runtime-budgets.tsv`), add the row with a header comment recording the three measurements, and bump `EXPECTED_TOTAL` in `tests/test_runtime_budgets.sh` by the new ceiling (case: "a new test file brings its own row").
- [ ] **Step 6: Verify contract + budgets.** `bash tests/test_go_integration_contract.sh` → all ok (new shard discovered, prefix matches exactly one runner); `bash tests/test_runtime_budgets.sh` → ok.
- [ ] **Step 7: Commit.** `git add internal/gitcli/history.go internal/gitcli/history_integration_test.go tests/test_go_integration_gitcli_history.sh tests/runtime-budgets.tsv tests/test_runtime_budgets.sh && git commit` — subject `feat(0378): gitcli shared-ancestry, history-tree, and tree-entry reads`.

---

### Task 3: Shared ownership verifier — topology and native receipt proofs

**Files:**
- Create: `internal/app/metadata_ownership.go`
- Test: `internal/app/repoownership_integration_test.go` (`//go:build integration`)
- Create: `tests/test_go_integration_app_repoownership.sh`
- Modify: `tests/runtime-budgets.tsv`, `tests/test_runtime_budgets.sh` (`EXPECTED_TOTAL`)

**Interfaces:**
- Consumes: Task 1's `reposetup.EvaluateSeedTrailers` / `SeedVerdict`; Task 2's `HasSharedAncestry`; existing `git.RootCommits`, `git.ScanCommitTrailers`, `git.TreeOID`, `git.EmptyTreeOID`, `git.IsAncestor`, `fromGitcliTrailers`.
- Produces (Tasks 4–7 consume exactly these):

```go
// seedProof names which ownership proof established RootParentless.
type seedProof int

const (
	proofNone             seedProof = iota // no proof (Shape says why)
	proofInitReceipt                       // OpInitRoot receipt + empty root tree
	proofMigrateReceipt                    // OpMigrateSeed receipt, source reachable, CopyDigest == root tree
	proofLegacyEmpty                       // receiptless empty-tree root (legacy bootstrap)
	proofLegacyEquivalent                  // receiptless root exactly equal to a historical source projection (Task 4)
)

// metadataOwnership is the shared verifier's result. It is never a boolean:
// Shape preserves the foreign-vs-unreadable distinction, and Err retains the
// probe error whenever Shape is RootUnknown.
type metadataOwnership struct {
	Tip            gitcli.ObjectID     // the pinned tip the proof was computed at
	Root           gitcli.ObjectID     // the sole parentless root ("" until proven sole)
	Shape          reposetup.RootShape // RootParentless / RootForeign / RootUnknown
	Proof          seedProof
	SourceRevision string // proving snapshot for proofMigrateReceipt / proofLegacyEquivalent
	Err            error  // retained diagnostics when Shape is RootUnknown
}

func verifyMetadataOwnership(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, tip, integrationTip gitcli.ObjectID, defaultBranch string) metadataOwnership
```

  (`defaultBranch` is threaded through for Task 4's historical config resolution; unused until then.)

- [ ] **Step 1: Write failing integration tests** (`TestIntegrationRepoOwnership…`) in `repoownership_integration_test.go`, driving `verifyMetadataOwnership` directly against real fixture repos. Build fixtures with plain `git` exec in a `t.TempDir()` (follow `newInitRepo` in `reposetup_integration_test.go` for the exec idiom); orphan roots via `git commit-tree` with trailers in the message, or via the app's own `RunRepositoryInit`/`RunRepositoryMigrate` where a genuine native lineage is wanted. Cover:
  - **Positive:** native init seed alone → `RootParentless`/`proofInitReceipt`; init seed + 3 content-changing metadata commits → still `RootParentless` (the defect under fix — write this one first); native migrate seed (built by `RunRepositoryMigrate` against a legacy fixture) + descendants → `RootParentless`/`proofMigrateReceipt` with `SourceRevision` preserved; a merge of two descendants sharing the verified root → `RootParentless`; receiptless **empty-tree** orphan root + descendants → `proofLegacyEmpty`.
  - **Negative:** two parentless roots (merge of two orphans) → `RootForeign`; a "metadata" branch created **from the integration branch** (shares ancestry) even with docket-looking files and trailers → `RootForeign`; valid receipt on a **descendant** only, root receiptless and nonempty with no historical match → `RootForeign`; init receipt with nonempty root tree → `RootForeign`; migrate receipt whose `CopyDigest` ≠ root tree OID → `RootForeign`; migrate receipt naming a source revision **not reachable** from the integration tip (well-formed OID of an unrelated commit) → `RootForeign`; migrate receipt with malformed (non-40-hex) source revision → `RootForeign`; duplicated/unknown-version/prune receipts on the root → `RootForeign` (never legacy-downgraded).
  - **Unknown:** tip object absent locally (probe `RootCommits` against an unfetched OID) → `RootUnknown` with non-nil `Err`; `integrationTip == ""` → `RootUnknown` (disjointness unprovable); assert `RootUnknown` is **never** `RootForeign` in these cases. Branch name, commit subject, author, timestamps, and a populated `.docket` directory appear in fixtures and prove nothing (the foreign-from-integration fixture carries all of them).
- [ ] **Step 2: Run to verify failure.** `go test -tags integration ./internal/app/ -run TestIntegrationRepoOwnership -count=1` → FAIL.
- [ ] **Step 3: Implement** `metadata_ownership.go`:

```go
func verifyMetadataOwnership(ctx context.Context, git *gitcli.Client, repo gitcli.Repository, tip, integrationTip gitcli.ObjectID, defaultBranch string) metadataOwnership {
	own := metadataOwnership{Tip: tip, Shape: reposetup.RootUnknown}
	roots, err := git.RootCommits(ctx, repo, tip)
	if err != nil {
		own.Err = err
		return own // unreadable history: unknown, never foreign
	}
	if len(roots) != 1 {
		own.Shape = reposetup.RootForeign
		return own
	}
	own.Root = roots[0]
	if integrationTip == "" {
		own.Err = errors.New("integration tip unknown; cannot prove disjoint ancestry")
		return own
	}
	shared, err := git.HasSharedAncestry(ctx, repo, tip, integrationTip)
	if err != nil {
		own.Err = err
		return own
	}
	if shared {
		own.Shape = reposetup.RootForeign
		return own
	}
	// Receipt trailers are read from the ROOT COMMIT ITSELF; a valid receipt on
	// a descendant cannot authorize an unrecognized root. The root is parentless,
	// so a scan from the root sees exactly the root.
	scans, err := git.ScanCommitTrailers(ctx, repo, own.Root, []string{
		reposetup.TrailerOperation, reposetup.TrailerSourceRevision,
		reposetup.TrailerMetadataRev, reposetup.TrailerCopyDigest,
		reposetup.TrailerRepairDigest,
	})
	if err != nil {
		own.Err = err
		return own
	}
	var rootTrailers []reposetup.Trailer
	for _, s := range scans {
		if s.Commit == own.Root {
			rootTrailers = fromGitcliTrailers(s.Trailers)
		}
	}
	rec, verdict := reposetup.EvaluateSeedTrailers(rootTrailers)
	rootTree, err := git.TreeOID(ctx, repo, own.Root)
	if err != nil {
		own.Err = err
		return own
	}
	switch verdict {
	case reposetup.SeedInit:
		emptyTree, err := git.EmptyTreeOID(ctx, repo)
		if err != nil {
			own.Err = err
			return own
		}
		if rootTree != emptyTree {
			own.Shape = reposetup.RootForeign
			return own
		}
		own.Shape, own.Proof = reposetup.RootParentless, proofInitReceipt
		return own
	case reposetup.SeedMigrate:
		if !isFullObjectID(rec.SourceRevision) || rec.CopyDigest != string(rootTree) {
			own.Shape = reposetup.RootForeign
			return own
		}
		reachable, err := git.IsAncestor(ctx, repo, gitcli.ObjectID(rec.SourceRevision), integrationTip)
		if err != nil {
			own.Err = err
			return own
		}
		if !reachable {
			own.Shape = reposetup.RootForeign
			return own
		}
		// The recorded repair digest keeps its meaning: an authorized-repairs seed
		// is valid without equality to an unmodified source projection.
		own.Shape, own.Proof, own.SourceRevision = reposetup.RootParentless, proofMigrateReceipt, rec.SourceRevision
		return own
	case reposetup.SeedInvalid:
		own.Shape = reposetup.RootForeign // never downgraded to the legacy path
		return own
	}
	// SeedAbsent: receiptless legacy proofs.
	emptyTree, err := git.EmptyTreeOID(ctx, repo)
	if err != nil {
		own.Err = err
		return own
	}
	if rootTree == emptyTree {
		own.Shape, own.Proof = reposetup.RootParentless, proofLegacyEmpty
		return own
	}
	return verifyLegacyEquivalence(ctx, git, repo, own, rootTree, integrationTip, defaultBranch)
}
```

  Add `isFullObjectID(s string) bool` (len 40, hex — small local helper; a receipt field is untrusted input, so validate before handing it to gitcli) and, for this task only, a `verifyLegacyEquivalence` stub that returns `own` with `Shape: RootUnknown` and `Err: errors.New("legacy-equivalence search not implemented")` — an unimplemented probe is Unknown, never a fabricated foreign (nothing consumes the verifier yet; Task 4 replaces the stub).
- [ ] **Step 4: Run to verify pass** for every non-legacy-equivalence test; the nonempty-receiptless-no-match negative from Step 1 stays failing until Task 4 — move that one test into Task 4's step 1 if written early, or guard it with a `t.Skip` removed in Task 4 (prefer moving it; skips rot).
- [ ] **Step 5: Shard runner + budget row.** Create `tests/test_go_integration_app_repoownership.sh` by copying `tests/test_go_integration_app_repocheck.sh`, changing the header, `SHARD_PKG="./internal/app"`, `SHARD_PREFIX="TestIntegrationRepoOwnership"`, `SHARD_MODE="normal"`. Measure cold ×3 as in Task 2, add the measured row + header comment, bump `EXPECTED_TOTAL`. Run `bash tests/test_go_integration_contract.sh` and `bash tests/test_runtime_budgets.sh` → ok.
- [ ] **Step 6: Commit.** `git add internal/app/metadata_ownership.go internal/app/repoownership_integration_test.go tests/test_go_integration_app_repoownership.sh tests/runtime-budgets.tsv tests/test_runtime_budgets.sh && git commit` — subject `feat(0378): shared metadata-root ownership verifier (topology + native receipts)`.

---

### Task 4: Receiptless legacy exact-equivalence against historical snapshots

**Files:**
- Modify: `internal/app/metadata_ownership.go` (replace the Task 3 stub)
- Test: `internal/app/repoownership_integration_test.go`

**Interfaces:**
- Consumes: Task 2's `ListHistoryTrees`, `TreeEntryIDs`; existing `git.BuildTree` (+ `gitcli.TreeOp{IncludePrefix: …}` — the same composition primitive `migrateExecute` uses), `readCommitBlob` (in `internal/app`, reads a blob at a commit — see its use for `.docket.yml` in `RunRepositoryMigrate`), `config.Resolve`, `config.Source`, `config.LayerRepository`, `reposetup.SpecsDir`, `git.OpenObjectSource` (for the live-surface eligibility read).
- Produces: the real `verifyLegacyEquivalence(ctx, git, repo, own, rootTree, integrationTip, defaultBranch) metadataOwnership` — on an exact match sets `Shape: RootParentless`, `Proof: proofLegacyEquivalent`, `SourceRevision: <matching snapshot commit>`; on exhausted readable history with no match sets `RootForeign`; on any read/enumeration error mid-search sets `RootUnknown` with `Err` (never foreign from a truncated search).

**Semantics (each maps to a test):**
1. Enumerate the **complete** reachable integration history via `ListHistoryTrees(integrationTip)` — never just today's already-pruned tip. Newest-first order is an optimization; commit messages and dates never gate eligibility.
2. Per candidate, resolve the **historical** directory configuration from the snapshot's own committed `.docket.yml` bytes with shipped defaults where absent, and the fixed `reposetup.SpecsDir`: build a single-layer source list `[]config.Source{{Layer: config.LayerRepository, Name: ".docket.yml", Data: bytes}}` (omit when the snapshot has no `.docket.yml`) and `config.Resolve(sources, config.ResolveContext{DefaultBranch: defaultBranch})`. The obsolete `metadata_branch` tombstone decodes as a warning, never an error — decoded only as historical migration input; **never** include global or repository-local machine layers (current user/machine configuration must not redefine historical evidence). A snapshot whose committed config fails to resolve is readable-but-ineligible: skip it, don't error the search.
3. Eligibility: the snapshot must contain the legacy live planning surface — a blob under `<changes>/active/` or a `<changes>/BOARD.md` in the snapshot's tree (same predicate as `liveSurfacePresence`, evaluated against the candidate's `ObjectSource`). Ineligible → skip.
4. Equivalence: compose the candidate's copy-set projection with `git.BuildTree(repo, "", ops)` where ops are `IncludePrefix{From: candidate, Prefix: p}` for each of `{changesDir, adrsDir, reposetup.SpecsDir}` that exists in the candidate (probe with `TreeEntryIDs`) — exactly how `migrateExecute` composes a seed. Match ⇔ composed tree OID == `rootTree`. Git's content-addressed tree identity gives complete path+mode+type+object-identity equality across the copied prefixes, preserves unknown files within them, and refuses any extra root path, missing copied path, changed bytes/mode, or unrelated tree — directory resemblance and matching subsets cannot pass.
5. Dedupe before composing: batch-key each candidate on `(configBlobOID, changesSubtreeOID, adrsSubtreeOID, specsSubtreeOID)` read via `TreeEntryIDs(commit, [".docket.yml", changesDir, adrsDir, SpecsDir])` — group config resolution by `configBlobOID` (resolve each distinct config once) and skip any projection tuple already composed. More than one snapshot proving the same exact tree is **not** ambiguity — the proof is content identity; return the first match.
6. Any gitcli error during enumeration, candidate reads, or composition → `RootUnknown` + `Err` immediately (do not keep searching and do not report foreign). Exhausted cleanly with no match → `RootForeign`.
7. No persistent cache, marker, allowlist, or adopt override. No checkout, no metadata rewrite, no real-index edit, no receipt writes.

- [ ] **Step 1: Write failing integration tests:**
  - nonempty receiptless orphan root whose tree exactly equals the copy-set projection of the **current** integration tip → `proofLegacyEquivalent`, `SourceRevision` = that tip;
  - the live case's shape: legacy seed matching an **older** snapshot, after the live planning surface was pruned from the current tip (advance integration past a prune) → still `proofLegacyEquivalent` with `SourceRevision` = the older snapshot; add descendants on the metadata branch → still `RootParentless`;
  - historical **nondefault** directories: a fixture whose old snapshot's committed `.docket.yml` sets `changes_dir: planning/changes` (and commits files there), while the repo's **current** committed config and a repository-local `.docket.local.yml` say something else → the match still succeeds (historical config decides; current/machine config does not);
  - a snapshot carrying the legacy `metadata_branch` key in `.docket.yml` → resolves (warning), still eligible;
  - **negative:** root tree with one extra file outside the copy set → `RootForeign`; a missing copied file → `RootForeign`; one changed byte → `RootForeign`; changed mode (100755 vs 100644 on one blob) → `RootForeign`; no eligible snapshot ever contained the surface → `RootForeign`; a plausible commit subject ("docket metadata seed") rescues nothing;
  - **unknown:** delete a needed object / corrupt reachability so a candidate read errors mid-search (e.g. point `integrationTip` at an OID whose history is not fully present locally) → `RootUnknown`, never foreign.
- [ ] **Step 2: Run to verify failure.** `go test -tags integration ./internal/app/ -run TestIntegrationRepoOwnership -count=1` → new tests FAIL against the stub.
- [ ] **Step 3: Implement** `verifyLegacyEquivalence` per the semantics above (a ~80-line function plus a small `historicalDirs(bytes []byte, defaultBranch string) (changes, adrs string, ok bool)` helper wrapping the single-layer `config.Resolve`).
- [ ] **Step 4: Run to verify pass**, including the Task 3 deferred negative (receiptless nonempty root, no historical match → `RootForeign`).
- [ ] **Step 5: Re-measure the ownership shard** cold ×3; if over its Task 3 ceiling, update the row per the table's rules (re-shaped file: adjust the row and `EXPECTED_TOTAL`, record the measurements). `bash tests/test_runtime_budgets.sh` → ok.
- [ ] **Step 6: Commit.** Subject `feat(0378): receiptless legacy exact-tree equivalence over historical snapshots`.

---

### Task 5: Check consumes the verifier (read-only; consistent fetched tip)

**Files:**
- Modify: `internal/app/repository_check.go` (`augmentCheckFacts`)
- Test: `internal/app/repoownership_integration_test.go` (check-level scenarios; keep the prefix `TestIntegrationRepoOwnership` so they land in the new shard — the RepoCheck shard's budget row has little headroom)

**Interfaces:**
- Consumes: Task 3/4's `verifyMetadataOwnership`.
- Produces: check behavior later tasks and the live verification (Task 10) rely on: a multi-commit verified branch classifies without `metadata-root-foreign`; all other findings retained.

**Behavior changes in `augmentCheckFacts`:**
1. Replace the inline `RootCommits`-and-compare block with the verifier, and make the **fetched** tip the one authority (learning `decide-and-act-on-the-same-copy`): on `FetchBranch` success, set `f.RemoteMetadata.Tip = string(rev.Commit)` (so `checkRevisions`' reported remote revision and `synchronizedPresence`'s comparison use the same tip the ownership proof used) and run `verifyMetadataOwnership(ctx, git, sc.repo, rev.Commit, gitcli.ObjectID(sc.sourceRevision), sc.defaultBranch)`; map `own.Shape` straight onto `f.MetadataRoot`. On `FetchBranch` **error**, set `f.MetadataRoot = reposetup.RootUnknown` and append a `setupDiag` — a fetch error is unknown **even if an older object happens to be available locally** (delete the current fall-back-to-ls-remote-tip probe).
2. Touch nothing else: the corpus/frontmatter gathering (`gatherFrontmatterFindings`), ignore-block, worktree, synchronization, partial-phase, and surface probes all stay, and check performs only the already-permitted object/remote-tracking fetch effects.

- [ ] **Step 1: Write failing integration tests:**
  - `TestIntegrationRepoOwnershipCheckHealthyMultiCommit`: `newHealthyRepo(t)` (from `repocheck_integration_test.go`), then commit-and-push 3 metadata commits onto the remote `docket` branch (content-changing — new change files; no seed receipt on any new commit, root tree contents not retained), sync the local `.docket` worktree; `runCheck` → `RepositoryState == "healthy"`, and no finding/reason `metadata-root-foreign` (this is the defect's headline test — write it first and watch it fail with `conflict`/`metadata-root-foreign`);
  - descendants + a deliberately broken frontmatter file in the corpus → the frontmatter finding still appears (independent tip/corpus validation runs; ownership fix silences nothing);
  - foreign single-root nonempty branch → still `metadata-root-foreign`; branch sharing ancestry with integration → `metadata-root-foreign`;
  - failed metadata fetch (point the remote at a URL whose `docket` ref cannot be fetched, or delete the remote repo's objects after ls-remote — simplest: break the remote path between gather and augment is not possible, so instead use a fixture whose remote repo is removed after cloning refs — if that is too contrived, drive `augmentCheckFacts` directly with a `setupContext` naming an unreachable remote) → state `unknown`-family conflict handling: `f.MetadataRoot` stays `RootUnknown` and the classifier does **not** report `metadata-root-foreign`;
  - read-only: reuse `readOnlySnapshot` (in `repocheck_integration_test.go`) around a check on a multi-commit repo → snapshot unchanged apart from permitted `refs/remotes/*` and fetched objects.
- [ ] **Step 2: Run to verify failure** (`-run TestIntegrationRepoOwnershipCheck`).
- [ ] **Step 3: Implement** the `augmentCheckFacts` change (delete the "Task-11 refines this" comment — it is the obsolete root-equals-tip note).
- [ ] **Step 4: Run to verify pass**; also run the whole existing check shard: `bash tests/test_go_integration_app_repocheck.sh` → ok (no regression).
- [ ] **Step 5: Commit.** Subject `fix(0378): repository check verifies ownership at the root, pinned to the fetched tip`.

---

### Task 6: Init consumes the verifier (create-only kept; race loser adopts, never resets)

**Files:**
- Modify: `internal/app/repository_init.go` (`publishOrAdoptMetadataRoot`, `expectedInitShape`)
- Test: `internal/app/repoownership_integration_test.go` (init scenarios, `TestIntegrationRepoOwnershipInit…`)

**Interfaces:**
- Consumes: `verifyMetadataOwnership`; existing `PushCreateLease`, `FetchBranch`.
- Produces: `publishOrAdoptMetadataRoot` unchanged in signature; `expectedInitShape` **replaced** by an init-equivalence check over the verifier's result:

```go
// initEquivalent reports whether a verified lineage is init-adoptable: its
// verified ROOT carries the empty tree (a native OpInitRoot seed, or the
// receiptless legacy-bootstrap empty root). It tests the root's tree, never
// the current tip's tree — descendants are permitted and preserved. A
// migration-seeded lineage is NOT init-equivalent: recognizing it never
// becomes permission to initialize.
func initEquivalent(own metadataOwnership) bool {
	return own.Shape == reposetup.RootParentless &&
		(own.Proof == proofInitReceipt || own.Proof == proofLegacyEmpty)
}
```

**Behavior:** in the `PushLeaseLost` arm, after the existing fetch, run the verifier at `outcome.Remote` (the reread remote tip; init's `setupContext` carries `sourceRevision` — thread it and `defaultBranch` into `publishOrAdoptMetadataRoot`, adjusting its call site in `RunRepositoryInit`):
- `RootUnknown` → external failure (the current `serr` path), naming the retained `own.Err`;
- `RootForeign` → the existing foreign refusal (message updated to say the branch "is not a verified docket metadata branch");
- verified + `initEquivalent` → **adopt `outcome.Remote`** — the tip, with descendants preserved; never re-push, never reset to the seed (the create-only push already never overwrote; adoption must not either);
- verified but migration-seeded (`proofMigrateReceipt`/`proofLegacyEquivalent`) → refusal: "the remote docket branch is an established migrated metadata branch; `docket repository init` cannot adopt it — run `docket repository check`". The fresh/state guards earlier in `RunRepositoryInit` (clean primary, at-tip, fresh classification) are untouched, as is the create-only `PushCreateLease` (never widened to an overwriting lease).

- [ ] **Step 1: Write failing integration tests:**
  - race adoption of a valid init lineage **with descendants**: create the remote `docket` branch as a genuine init root plus 2 pushed metadata commits *before* running `RunRepositoryInit` in a second clone → init succeeds, adopted `metadataTip` is the multi-commit tip, and afterwards the remote `docket` tip is **unchanged** (assert the remote OID before == after: no descendant lost, no reset to seed);
  - the same with a receiptless empty-root legacy bootstrap lineage + descendants → adopted;
  - migration-seeded remote (run a real migrate in fixture A, then `RunRepositoryInit` from clone B) → refusal, remote OIDs unchanged;
  - foreign remote branch (existing `TestIntegrationRepoSetupInitRefusesForeignMetadataBranch` covers the single-commit case — keep it green; add a multi-commit foreign lineage) → refusal, remote unchanged;
  - preservation asserts on every refusal/race case: remote OIDs, local branches, index, and worktree state preserved (only documented fetch effects) — reuse the `readOnlySnapshot` idiom.
- [ ] **Step 2: Run to verify failure.**
- [ ] **Step 3: Implement**; delete the old `expectedInitShape` body (its name may survive as `initEquivalent` + verifier call; ensure no `roots[0] == commit` comparison remains in this file).
- [ ] **Step 4: Run to verify pass**; then `bash tests/test_go_integration_app_reposetup.sh` and `bash tests/test_go_integration_app_reposetup_race.sh` → ok.
- [ ] **Step 5: Commit.** Subject `fix(0378): init race loser adopts a verified init-equivalent lineage at its tip`.

---

### Task 7: Migrate consumes the verifier; seed replacement refused once descendants exist

**Files:**
- Modify: `internal/app/repository_migrate.go` (`migrateRoute`, `reconcileResumeSeed`)
- Modify: `internal/app/repository_facts.go` (delete `metadataRootParentless` once the last caller is gone; see Task 8 for the sweep)
- Test: `internal/app/repoownership_integration_test.go` (`TestIntegrationRepoOwnershipMigrate…`) plus targeted additions to `reporecovery_integration_test.go` **only** if a hook seam is needed that lives there; prefer the ownership shard.

**Interfaces:**
- Consumes: `verifyMetadataOwnership`; existing hooks seam (`beforeMetadataLeasePush`), `PushLease`, `remoteTreeEquals`, `publishedSeedReceipt`.
- Produces: `migrateRoute` re-signed as `func migrateRoute(ctx context.Context, git *gitcli.Client, facts reposetup.Facts, sc *setupContext) (migratePhase, *RepositoryMigrateResult)` — it now takes `*setupContext` so the re-read tip it decided on is the tip later phases use (`sc.metadataTip = string(rev.Commit)`); update its one call site in `RunRepositoryMigrate`.

**Behavior:**
1. `migrateRoute`, `PresencePresent` arm: after the existing `FetchBranch` re-read, set `sc.metadataTip = string(rev.Commit)` and replace the `metadataRootParentless` call with `verifyMetadataOwnership(ctx, git, sc.repo, rev.Commit, gitcli.ObjectID(sc.sourceRevision), sc.defaultBranch)`:
   - `RootUnknown` → the existing external-failure refusal (retain `own.Err`);
   - `RootForeign` → the existing foreign refusal (message updated: "not a verified docket metadata branch");
   - verified: existing routing unchanged — live surface present → `phaseResumePrune`; else registered → `phaseAlreadyMigrated` (an **established migrated branch**, however many commits, is a no-op; nothing is discarded); else → `phaseResumeLocal` (local finish attaches at the **actual latest tip** — `migrateResumeLocal` reads `sc.metadataTip`, which step 1 just pinned to the re-read tip).
2. `reconcileResumeSeed` (the mutation boundary — recheck identity here, never trust the route's earlier read):
   - after its own fresh `FetchBranch`, verify ownership at the fresh `metadataTip`;
   - `RootUnknown` → external failure; `RootForeign` → conflict refusal (existing message);
   - **new descendants guard:** if `own.Tip != own.Root` — descendants exist while the live integration surface still needs pruning — refuse before replacing the seed or pruning integration:

```go
if own.Tip != own.Root {
	r := migrateRefusal(reposetup.StateConflict,
		"the published metadata branch has commits after its seed while the integration branch still carries the live planning surface; this partial migration must be reconciled by a human — inspect both branches, then run `docket repository check`")
	return "", &r
}
```

   - only past that guard (tip **is** the verified seed root) do the existing arms run unchanged: exact current-seed tree → adopt; `OpMigrateSeed` receipt naming a *different* source → recompose and replace under the exact owned lease **bound to the same fresh tip** (`PushLease(…, newSeed, metadataTip)` — already so; the `beforeMetadataLeasePush` contention seam stays, so a concurrent advance still loses the lease and the winner's history stays intact); anything else → conflict refusal. Existing exact-seed response-loss and stale-source recovery therefore keep working exactly while the branch has no descendants and their operation-specific preconditions hold.
3. The prune/lease/publication paths after `reconcileResumeSeed` are untouched (exact source-revision lease, byte-exact re-reads, hooks).

- [ ] **Step 1: Write failing integration tests:**
  - **no-op at latest tip:** fully migrated repo + 4 later metadata commits pushed; re-run `RunRepositoryMigrate` (authorized) → `ResultNoOp`/already-migrated, remote `docket` tip unchanged (previously this refused as foreign — headline migrate test, write first);
  - **local-attachment recovery at latest tip:** same remote, fresh clone with no local `.docket` → migrate performs only the local finish, attaching at the actual latest tip (assert `.docket` HEAD == the multi-commit remote tip, remote unchanged);
  - **descendants block seed replacement:** interrupted migration (use the `afterSeedPush` hook to stop after the seed lands), then push a descendant commit onto remote `docket`, then advance the legacy source so a resume would want to replace the seed, then resume → the new refusal fires; assert the remote `docket` tip (descendant intact), remote integration tip, local branches, index, and worktree are unchanged;
  - **descendants + resume with unchanged source:** same interruption + descendant, source unchanged → also refused (the guard keys on descendants-while-prune-pending, not on tree difference);
  - **no-descendants recovery still works:** re-run the existing exact-seed response-loss scenario (seed pushed, response lost, resume with unchanged source → adopts; with moved source → owned-lease replace) — these exist in `reporecovery_integration_test.go`; keep them green rather than duplicating them, and add one ownership-shard copy of the moved-source replace to pin that the lease is bound to the fresh tip;
  - **concurrent advance during the publication race:** drive `beforeMetadataLeasePush` to push a descendant onto remote `docket` mid-resume → migration contends (`migrateContended`), the winner's descendant survives (assert remote tip == the foreign advance).
- [ ] **Step 2: Run to verify failure.**
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run to verify pass**; then `bash tests/test_go_integration_app_repomigration.sh` and `bash tests/test_go_integration_app_reporecovery.sh` → ok (existing recovery behavior preserved).
- [ ] **Step 5: Commit.** Subject `fix(0378): migrate recognizes established branches and refuses seed replacement over descendants`.

---

### Task 8: Retire the duplicated predicates; whole-repo caller sweep

**Files:**
- Modify: `internal/app/repository_facts.go` (delete `metadataRootParentless` and rewrite the "partial-phase recovery probing" comment block: root shape is now owned by `verifyMetadataOwnership`; `publishedSeedReceipt`/`remoteTreeEquals` remain as `reconcileResumeSeed`'s mutation-boundary reads)
- Modify: any straggler found by the sweep

- [ ] **Step 1: Derive the full caller inventory from repo-wide searches, never the spec's named list** (learning: never hand-list gated sites): `grep -rn "RootCommits\|metadataRootParentless\|expectedInitShape\|RootParentless\|RootForeign\|root.*== *tip\|roots\[0\] == " internal/ cmd/ --include="*.go" | grep -v _test`. Sort hits into (a) the verifier itself, (b) consumers already converted (Tasks 5–7), (c) prose/comments. Any remaining executable root-enumeration or seed-adoption/recovery site comparing a root to a tip is a missed consumer — convert it in this task. Expected residue: none besides `verifyMetadataOwnership`'s own `RootCommits` call.
- [ ] **Step 2: Delete `metadataRootParentless`** and every obsolete root-equals-tip comment (including `migrateRoute`'s doc comment naming it and `augmentCheckFacts`' old comment if any survived Task 5). `go build ./... && go vet ./...` → clean.
- [ ] **Step 3: Run the two pure-unit packages** `go test ./internal/reposetup/ ./internal/app/ -count=1` → PASS (default-tag corpus).
- [ ] **Step 4: Commit.** Subject `refactor(0378): remove the duplicated root-equals-tip ownership predicates`.

---

### Task 9: Mutation-test the ownership and replacement guards

No production files change in this task; it produces recorded evidence that the guards are load-bearing (learning `guards-are-code`). For each probe: `cp <file> <file>.bak`, apply the mutation, run the named test with `-count=1` and **confirm it fails**, then restore from the `.bak` (never `git checkout --`), re-run to confirm green, and delete the `.bak`. Record each probe → reddened-test pair in the build-evidence notes.

- [ ] **Probe A — remove root identity validation:** in `verifyMetadataOwnership`, change `if len(roots) != 1` to `if false` (accept any root count). Expect the two-roots and shared-ancestry-adjacent negatives in `TestIntegrationRepoOwnership…` to fail.
- [ ] **Probe B — weaken exact legacy equality:** in `verifyLegacyEquivalence`, compare only that the composed tree's `changes` subtree matches (or: skip the extra-path-refusing whole-tree equality and return match on the first eligible snapshot). Expect the extra-file / changed-byte / changed-mode negatives to fail.
- [ ] **Probe C — collapse a probe error:** in `verifyMetadataOwnership`, make the `RootCommits` error branch fall through to `RootForeign` instead of returning `RootUnknown`. Expect the unknown-mapping tests (unreadable tip → `RootUnknown`) to fail.
- [ ] **Probe D — permit seed replacement after descendants:** delete the `own.Tip != own.Root` refusal in `reconcileResumeSeed`. Expect the descendants-block-seed-replacement test to fail (descendant discarded or lease pushed).
- [ ] **Probe E — receipt-on-descendant:** in `verifyMetadataOwnership`, collect trailers from **any** scanned commit rather than `s.Commit == own.Root`. Expect the receipt-only-on-a-descendant negative to fail.
- [ ] **Commit nothing** unless a probe exposed a vacuous assert — in that case fix the test (learning `assert-pins-outcome-not-mechanism`), re-probe, and commit the test fix with subject `test(0378): tighten <which> ownership guard assert`.

---

### Task 10: Live-history verification, negative control, and cost measurement

Build-time recorded verification against real history (learning `metadata-branch-invisible-to-suite`): the hermetic suite cannot see this repository's actual metadata branch, so prove the live defect is gone here and record it in the build evidence. Content-read-only throughout; live OIDs are recorded evidence, never turned into hermetic fixture requirements or test constants.

- [ ] **Step 1: Live check.** From the primary worktree (`/Users/homer/dev/docket`), run the **newly built source**: `go run ./cmd/docket repository check --json` (run it from this feature worktree's checkout: `cd .worktrees/metadata-root-classifier-rejects-multi-commit-docket-branch && go run ./cmd/docket repository check --json` with the primary repo as the invocation target if the command takes the cwd — check `cmd/docket` wiring; if check discovers from cwd, run the built binary from the primary worktree: `go build -o "$TMPDIR/docket-0378" ./cmd/docket` then run it at the primary). Assert in the recorded output: **no `metadata-root-foreign` finding**, while the unrelated corpus/frontmatter findings the spec notes are still present (removing the false finding promises neither a wholly healthy repository nor exit 0). Record: the metadata tip OID, root OID (`f8b226f2…` expected), integration tip OID, the proof category (expected: legacy equivalence, matching snapshot expected to be `1e7493a6…`'s projection), and the check's exit code.
- [ ] **Step 2: Negative control.** In a **disposable** repository (clone this repo's `docket` + integration branches into a `$TMPDIR` bare remote + work clone — never the real remote), tamper the legacy root's tree (rewrite the metadata branch in the disposable clone so its root gains one extra file) and run the built check → `metadata-root-foreign` **does** appear. Record the finding line. Delete the disposable repo.
- [ ] **Step 3: Cost.** Time the live check (3 runs, `/usr/bin/time -p`), and record the legacy-history search cost on this real history (~hundreds of integration commits). Report the numbers in the evidence. If the whole-suite run then reports any `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` / `SERIAL CONFIRMED OVER BUDGET:` line for the new shards, act on it per `tests/README.md` (serial-confirm; re-shard or re-measure the row — never hide an incomplete scan behind a cap and never bump a number to make it fit).
- [ ] **Step 4: Record** all of the above in the build-evidence record for the change (the build role's evidence surface — not a new results file in this plan).

---

## Verification coverage map (spec "Verification and acceptance" → tasks)

| Spec requirement | Where |
|---|---|
| Native init/migrate seeds, alone + multi-commit | Task 3 tests |
| Merge of descendants sharing the root | Task 3 |
| Receiptless empty bootstrap + nonempty legacy ancestry with descendants | Task 3 (empty), Task 4 (nonempty) |
| Legacy equality vs older snapshot after prune; nondefault dirs; changed current config | Task 4 |
| Healthy check classification with all other facts healthy | Task 5 |
| Init race adoption of valid init lineage | Task 6 |
| Migrated no-op / local-attachment recovery at actual latest tip | Task 7 |
| Existing exact-seed response-loss + stale-source recovery (no descendants) | Task 7 (kept green + one pinned copy) |
| Foreign nonempty single root; multiple roots; shared ancestry | Tasks 3, 5 |
| Receipt on descendant only; missing/malformed/duplicate/unknown-version/wrong-op fields; wrong init tree; digest mismatch; unavailable source | Tasks 1, 3 |
| Legacy extra/missing/changed-content/changed-mode; no exact match; subject rescues nothing | Task 4 |
| Failed fetch/root/tree/receipt/history reads + truncated history → unknown, nothing adopted/overwritten/pruned | Tasks 3, 4, 5 |
| Descendants after seeding / before resume / during race: none lost; unsafe partial refused before pruning | Task 7 |
| Ordinary metadata updates need no per-commit receipt, root tree not retained; tip/corpus validation still runs | Task 5 |
| Refusal/race preservation asserts (remote OIDs, local branch/index/worktree/config; only fetch effects) | Tasks 6, 7 |
| Mutation-test ownership + replacement guards | Task 9 |
| Live-history check, tampered negative control, legacy-path cost | Task 10 |
