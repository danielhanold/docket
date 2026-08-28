<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0363 — Remove main-mode compatibility from Go v1](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0363-remove-main-mode-compatibility-from-go-v1.md)**
<!-- docket:backlink:end -->
# Remove Main-Mode Compatibility From Go v1 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use the configured build skill (`docket-build`) to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the `main` metadata topology from Go v1 — one supported repository model (orphan `docket` metadata branch + independently resolved `integration_branch`), with `metadata_branch` reduced to a decode-only obsolete tombstone and legacy repositories routed to `docket repository migrate` through a shared operational-repository gate.

**Architecture:** Contract the config schema (mirror the existing `runtime.bash` `dispObsolete` tombstone), extract an operational-repository loader over change 0352's `reposetup.Classify` so `StatusReader.PinContext` becomes an adapter and every ordinary command inherits the legacy refusal, then delete `StatusPin.Mode` / `metadataBranchOf` and the three mode-shaped JSON keys. Test matrices collapse to their docket row; a mutation-tested structural guard keeps mode conditionals from returning.

**Tech Stack:** Go 1.x (`internal/config`, `internal/app`, `internal/reposetup`), shell test suite (`tests/*.sh`), `scripts/run-tests.sh` as the build gate.

**Spec:** `docs/superpowers/specs/2026-08-28-remove-main-mode-compatibility-from-go-v1-design.md` (synchronized on the `docket` metadata branch; the change file is `docs/changes/active/0363-remove-main-mode-compatibility-from-go-v1.md`).

## Global Constraints

- `integration_branch` stays fully configurable (`main`, `develop`, `auto`); nothing hard-codes integration to `main`.
- Frozen fixture bytes are never edited in place: `testdata/repositories/v0.9.2/**`, `v0.9.3/**`, `v0.9.4/**` stay byte-identical. A needed self-fixture change gets a NEW versioned tree (`v0.9.5`) with `PROVENANCE.md` and re-derived expectations (learning: `config-edit-trips-its-own-frozen-drift-guard`).
- Historical records are never rewritten: `docs/results/**`, `docs/changes/archive/**`, archived specs/plans, Accepted ADR authored text keep their original main-mode claims. Classify every repo-wide search hit before editing (executable source / maintained prose / point-in-time record).
- Change 0352's classifier and editor are REUSED, never reimplemented: `reposetup.Classify`, `reposetup.StateLegacy`, the `legacy-repository` finding, `reposetup.RemoveMetadataBranchKey`. Do not modify `internal/reposetup/classify.go`'s decision ladder.
- Every mutation probe: back up with `cp file file.bak`, prove the mutation landed (`git diff` shows the changed line) BEFORE reading the result, restore with `mv file.bak file`, and defeat Go's test cache with `-count=1` (learnings: `mutation-restore-needs-a-backup-copy`, `cached-runner-serves-a-mutated-tree`).
- Absence asserts get a non-vacuity companion through the same extractor, and are proven by re-adding the removed state and watching red (learning: `assert-detects-removal-not-replacement`).
- The build gate is the full configured suite via `scripts/run-tests.sh`. A `SERIAL CONFIRMED OVER BUDGET:` line is a build finding even at exit 0.
- The new ADR is NOT hand-minted in this branch. It is recorded via the `docket-adr` workflow at review time (Task 10 flags the decision); `adr-update-delivery` learning: the ADR-0001 dated `## Update` and the new ADR travel via the change's `adrs:` list, never a standalone push.

---

### Task 1: Fresh source-derived inventory of executable mode sites

**Files:**
- No repo files are created or modified. Output is the task report (the executor carries it into later tasks and the results file).

**Interfaces:**
- Produces: a three-way classification (executable / maintained prose / frozen-historical) of every `metadata_branch`, `metadataMode`, `metadataBranchOf`, `MetadataBranch`, `metadata_mode`, `StatusPin.Mode`/`pin.Mode`, and "main mode" hit, which Tasks 2–9 consume as their site list. Never hand-list sites in later tasks — this derived list is authoritative.

- [ ] **Step 1: Derive the whole-repo hit list**

Run from the worktree root:

```bash
grep -rn "metadata_branch\|metadataBranchOf\|metadataMode\|MetadataBranch\|metadata_mode\|metadata-branch" \
  --include="*.go" internal/ cmd/ | sort > /tmp/0363-go-hits.txt
grep -rn "\.Mode\b\|pin\.Mode\|Mode " --include="*.go" internal/app/status.go internal/app/status_git.go >> /tmp/0363-go-hits.txt
grep -rln "metadata_branch\|main mode\|main-mode\|metadata branch" \
  README.md docs/ tests/ scripts/ skills/ .docket.yml .docket.yml.example 2>/dev/null | sort > /tmp/0363-other-hits.txt
```

- [ ] **Step 2: Classify every hit**

Sort into three buckets, per file:
1. **Executable Go, non-test** — the removal surface (expected to include: `internal/config/{config,defaults,resolve,schema}.go`, `internal/app/{status.go,status_git.go,status_result.go,status_human.go,config.go,change_create.go,link_context.go,implementation_context.go,repository_facts.go,maintenance.go}`, the ~12 `metadataBranchOf` call sites in `change_{implemented,halt,claim,groom,attach,lifecycle,kill,reclaim}.go`, `finalize_{block,closeout}.go`).
2. **Test Go + shell tests + maintained prose** — README, `.docket.yml`, skills/scripts docs, `tests/test_docket_metadata_branch.sh` and any shell test grepping mode prose (learning: `restatement-accumulates-its-own-guards` — grep tests for the PROSE about to be deleted, not just the source).
3. **Frozen/historical** — `testdata/repositories/v0.9.*/**`, `docs/results/**`, `docs/changes/archive/**`, `docs/adrs/**` authored text, archived specs/plans. These are untouchable (except ADR-0001's dated `## Update`, delivered at review time).
4. Note which `internal/reposetup` hits are the *legacy-input* surface (`configedit.go`, `probe.go` `LegacyConfigKey`, `migrateplan.go` `ConfigEdit`) — these are KEPT: migration still recognizes and removes the old key.

- [ ] **Step 3: Record the classified inventory in the task report** (counts per bucket + the executable file list). No commit — this task changes nothing.

---

### Task 2: Config schema — `metadata_branch` becomes a decode-only obsolete tombstone

**Files:**
- Modify: `internal/config/schema.go` (the `metadata_branch` pathSpec in `buildRegistry`)
- Modify: `internal/config/config.go` (drop `Effective.MetadataBranch`)
- Modify: `internal/config/defaults.go` (drop `MetadataBranch` from `builtinEffective`)
- Modify: `internal/config/resolve.go` (drop the `assign(&eff.MetadataBranch, …)` line)
- Test: `internal/config/resolve_test.go` / `internal/config/capability_test.go` (or a new `internal/config/obsolete_metadata_branch_test.go`)

**Interfaces:**
- Consumes: the existing obsolete-row machinery — `dispObsolete`, `CodeObsoleteSetting`, and `capability.go`'s `obsoleteCapabilities()` lift (the `runtime.bash` row at `schema.go` `buildRegistry` entry 1 is the reference pattern).
- Produces: `config.Effective` with NO `MetadataBranch` field; resolving any layer carrying `metadata_branch: <anything>` yields an `obsolete-setting` diagnostic attributed to the declaring layer, no resolution error, no effective value. Tasks 3–5 rely on `Effective` no longer carrying the field.

- [ ] **Step 1: Write the failing tests**

In a new `internal/config/obsolete_metadata_branch_test.go`, following the register of the existing resolve tests (use the package's existing source-building helpers — read `resolve_test.go` first and reuse its fixture constructors rather than inventing new ones):

```go
// TestMetadataBranchIsObsoleteTombstone: the key decodes, diagnoses, and
// never resolves.
func TestMetadataBranchIsObsoleteTombstone(t *testing.T) {
	// (a) repository-layer declaration: obsolete-setting diagnostic whose
	// message directs to `docket repository check`, provenance = repository
	// layer; document still parses; no resolution error.
	// (b) machine-local declaration: obsolete-setting diagnostic telling the
	// operator to remove the key from that named file; provenance = local layer.
	// (c) global declaration: same shape, provenance = global layer.
	// (d) metadata_branch: main selects nothing — resolution succeeds and the
	// snapshot carries no metadata-branch value anywhere (assert the marshaled
	// Effective JSON does not contain the substring `"metadata_branch"`).
	// (e) the key never appears in Capabilities as a supported/deferred row.
}

// TestEffectiveHasNoMetadataBranchField: reflection/JSON absence with a
// non-vacuity companion (integration_branch still present through the same
// marshal), per assert-detects-removal-not-replacement.
func TestEffectiveHasNoMetadataBranchField(t *testing.T) {
	b, _ := json.Marshal(Effective{})
	if bytes.Contains(b, []byte(`"metadata_branch"`)) {
		t.Fatalf("Effective still serializes metadata_branch: %s", b)
	}
	if !bytes.Contains(b, []byte(`"integration_branch"`)) {
		t.Fatalf("non-vacuity companion missing — marshal shape changed: %s", b)
	}
}
```

Write (a)–(e) as real subtests with real layer fixtures — the message asserts must pin the layer-specific remedy wording chosen in Step 3 (learning: `printed-remedy-state-validity` — the repository-layer remedy points at `docket repository check` because migration owns that occurrence; machine layers get "remove `metadata_branch` from <path>" because migration claims no authority there).

- [ ] **Step 2: Run to verify failure** — `go test -count=1 ./internal/config/ -run 'MetadataBranch'` fails (field still exists, key still resolves).

- [ ] **Step 3: Implement the tombstone**

In `schema.go`, replace the supported row:

```go
// metadata_branch is an obsolete tombstone (change 0363): recognized in any
// layer so inspection can attribute it, never resolved, never a capability.
// The repository-layer occurrence is change 0352's migration input.
{path: "metadata_branch", kind: kindString, merge: mergeScalar,
	disp: dispObsolete, validate: stringLeaf(false, false, false)},
```

Match the `runtime.bash` row's mechanics exactly — read how `dispObsolete` flows through resolution and `capability.go` before writing, and note `schema.go`'s comment that `runtime.bash` is "the only row carrying this scope": adjust that comment truthfully (the new row is NOT `scopeLocalOnly` — it must be recognized in every layer including the repository layer, so pick the scope that lets all layers decode it and verify no fence warning fires for the repository layer). Give the obsolete diagnostic layer-aware message text at the site where `CodeObsoleteSetting` diagnostics are minted (find it by grepping `CodeObsoleteSetting` in non-test config source). Remove `Effective.MetadataBranch`, its `builtinValue("docket")` default, and its `assign` line. Do NOT touch `internal/reposetup/configedit.go` / `probe.go` — the migration path keeps reading raw bytes.

- [ ] **Step 4: Fix compile fallout inside `internal/config` only** — later tasks own `internal/app`. If `internal/app` breaks the module build, insert the temporary bridge in the smallest form (a local constant `"docket"` at `status_git.go`'s `eff.MetadataBranch` read) and mark it with `// 0363 Task 4 removes this` so the structural guard task can confirm it is gone. Intermediate states must build and test green (learning: `intermediate-task-state-buildable`).

- [ ] **Step 5: Run** `go test -count=1 ./internal/config/` — new tests pass; note which existing config tests redden (matrix tests asserting `metadata_branch` as supported, `TestClassifyMatrix`, fixture expectation tests). Update expectations that describe CURRENT behavior; do not touch frozen fixture bytes. Fixture tests read v0.9.2 INPUT bytes (fine — inputs stay) but their EXPECTED resolutions change: re-derive expectations by running the resolver and reading actual output, never by guessing.

- [ ] **Step 6: Mutation probe** — restore the supported row (`cp schema.go schema.go.bak`, revert the row to the old enum spec, verify via `git diff`), run `go test -count=1 ./internal/config/ -run 'MetadataBranch'`, expect RED, then `mv schema.go.bak schema.go`.

- [ ] **Step 7: Commit** — `git add internal/config/ && git commit -m "refactor(0363): metadata_branch becomes a decode-only obsolete tombstone"`.

---

### Task 3: Shared operational-repository loader over 0352's classifier

**Files:**
- Create: `internal/app/operational_context.go`
- Create: `internal/app/operational_context_test.go`
- Modify: `internal/app/status_git.go` (`PinContext` becomes an adapter)
- Modify: `internal/app/repository_facts.go` (expose fact-gathering for reuse; drop `sc.metadataBranch = eff.MetadataBranch.Value` — the metadata branch is fixed)
- Modify: `internal/reposetup/` — add the ONE named constant for the fixed branch (owned beside topology classification, per spec): `const MetadataBranchName = "docket"` in a small new file `internal/reposetup/branch.go` (or beside `classify.go`'s types if a constants home already exists — check first).

**Interfaces:**
- Consumes: `reposetup.Classify(f reposetup.Facts) reposetup.Classification`, `reposetup.StateLegacy`, the existing facts-gathering in `repository_facts.go`, and `config.Resolve`.
- Produces:
  - `reposetup.MetadataBranchName = "docket"` — the only spelling of the fixed branch later tasks may use.
  - `func loadOperationalContext(ctx context.Context, client *gitcli.Client, repoDir string) (operationalContext, error)` performing the spec's ordered read: discover repo + remote default → resolve config (tombstone cannot influence it) → fetch/pin integration and the fixed remote `docket` revision → probe classifier facts → classify ONCE → for `StateHealthy`/operational, return pinned default/integration/metadata revisions, resolved config snapshot + diags, repo web URL.
  - `type errRepositoryNotOperational struct { State reposetup.State; Classification reposetup.Classification }` (an `error`) carrying the typed refusal inputs. Ordinary commands map it to the shared `invalid-state` / `legacy-repository` protocol document.
  - `PinContext` keeps its exact signature `(ctx, repoDir) (StatusPin, error)` and becomes an adapter: call the loader, translate `operationalContext` → `StatusPin`. All ~20 `PinContext` call sites inherit the gate with zero per-command edits.

- [ ] **Step 1: Write the failing tests** in `operational_context_test.go`, driving through the existing test seams (read `reposetup_integration_test.go` / `claim_workflow_git_test.go` first to reuse the real-temp-git-repo fixtures — this package already builds bare-origin fixtures; a legacy fixture is one with a live planning surface on the integration branch and no remote `docket` branch):

```go
// TestOperationalGateRefusesLegacy: a legacy fixture repo makes PinContext
// (via any ordinary op — use the status operation) return the shared refusal:
//   result "invalid-state", reason "legacy-repository",
//   repository_state "legacy", findings[0].code "legacy-repository",
//   findings[0].severity "error",
//   findings[0].remedy mentioning `docket repository migrate`,
// and the envelope keeps the attempted operation name.
// Also assert the ordinary operation performed NO mutation (no refs moved).

// TestOperationalGateFindingIsTheClassifierValue: the finding equals the one
// `repository check` reports for the same fixture — same code, severity,
// message, remedy — not a command-specific copy.

// TestOperationalGatePassesHealthy: a healthy docket-topology fixture returns
// a pin with IntegrationBranch resolved from config, MetadataRevision pinned
// from remote docket, and no refusal.

// TestFailClosedOrdering: invalid config (unparseable .docket.yml) reports
// invalid-config, NOT the legacy remedy; an unknown classifier state keeps
// 0352's own state/findings and never collapses into legacy-repository.
```

- [ ] **Step 2: Run to verify failure** — `go test -count=1 ./internal/app/ -run 'OperationalGate|FailClosed'` fails (loader absent).

- [ ] **Step 3: Implement** `operational_context.go`. This is an EXTRACTION: move `PinContext`'s existing discovery/config/fetch body into the loader, then add the classifier facts + single `Classify` call, reusing the probes `repository_facts.go` already implements (refactor its fact-gathering into a callable shared by check/init/migrate and the loader — one set of Git probes, not two). The refusal for ordinary commands must render through the app layer's existing `invalid-state` result shape; grep how `repository_check.go` builds its findings DTO and reuse that constructor. Remove the Task 2 bridge constant in `status_git.go`; delete the `metadataModeMain` selector block (`status_git.go` "if eff.MetadataBranch.Value == metadataModeMain" arm) — the loader always pins `reposetup.MetadataBranchName`. `repository check`/`init`/`migrate` keep their own sub-gate entry (they call the classifier below the operational gate, per the spec's command-family table) — verify they do NOT route through the ordinary-command refusal.

- [ ] **Step 4: Run** `go test -count=1 ./internal/app/ -run 'OperationalGate|FailClosed'` — PASS; then `go test -count=1 ./internal/app/` and repair fallout in tests that fixture main-mode pins ONLY where the failure is the pin shape (full contraction is Task 6; keep this commit minimal — if a broad matrix reddens, prefer adjusting the shared fixture helper (`mainModePin`-style constructors) to produce docket pins).

- [ ] **Step 5: Mutation probes** — (a) make the loader skip the classify step (return operational unconditionally): `TestOperationalGateRefusesLegacy` must redden; (b) point the pinned metadata source at the integration branch: the healthy-fixture test's MetadataRevision assert must redden. `cp`-backup, verify with `git diff`, `-count=1`, restore.

- [ ] **Step 6: Commit** — `git add internal/app/ internal/reposetup/ && git commit -m "refactor(0363): shared operational-repository gate over the 0352 classifier"`.

---

### Task 4: Remove `StatusPin.Mode`, `metadataBranchOf`, and mode-conditioned source selection

**Files:**
- Modify: `internal/app/status.go` (StatusPin: drop `Mode`, drop `MetadataBranch`; keep `MetadataRevision`)
- Modify: `internal/app/status_git.go` (drop `metadataModeMain`/`metadataModeDocket` consts and the mode comment block; `metadataRevision(pin)`/`sourceRevision(pin, source)` collapse to fixed-source logic)
- Modify: `internal/app/change_create.go` (delete `metadataBranchOf`)
- Modify: the ~12 `TargetRef: gitcli.RefName(branchRefPrefix + metadataBranchOf(pin))` call sites — from the Task 1 inventory: `change_implemented.go`, `change_halt.go` (×2), `finalize_block.go` (×2), `change_reclaim.go`, `change_groom.go`, `change_lifecycle.go`, `change_attach.go`, `change_kill.go`, `change_claim.go` (×2), `change_create.go`, `finalize_closeout.go` (×2) → `TargetRef: gitcli.RefName(branchRefPrefix + reposetup.MetadataBranchName)`
- Modify: `internal/app/link_context.go` (`linkContextOf`: `MetadataBranch: reposetup.MetadataBranchName`)
- Modify: `internal/app/implementation_context.go` (`MetadataRef: reposetup.MetadataBranchName`)
- Modify: `internal/app/finalize_context.go` (`finalizePolicy(pin)` — read it; collapse any mode arm to docket behavior)
- Modify: `internal/app/maintenance.go`, `internal/app/repository_facts.go` (residual `eff.MetadataBranch` / mode reads from the Task 1 inventory)
- Test: existing `internal/app` tests (compile fallout only; contraction is Task 6)

**Interfaces:**
- Consumes: `reposetup.MetadataBranchName` (Task 3).
- Produces: a `StatusPin` with no `Mode` and no `MetadataBranch` field. `grep -rn "metadataBranchOf\|metadataModeMain\|metadataModeDocket\|pin.Mode" --include="*.go" internal/ cmd/ | grep -v _test` returns nothing. Metadata reads use the fixed source; integration reads keep the separately resolved `IntegrationBranch`. This is what Task 7's structural guard enforces going forward.

- [ ] **Step 1: Write the removal test first** — in `internal/app/status_test.go` (or a new `mode_removal_test.go`), a compile-level assert is the test: `StatusPin{}` with no `Mode`/`MetadataBranch` fields won't compile if they linger, so the real failing test is behavioral:

```go
// TestMetadataSourceIsFixed: every metadata-source read (ReadCorpus,
// ArtifactExists with the metadata source token) resolves against the pinned
// docket revision even when integration and default branches carry a
// DIFFERENT tree — fixture with deliberately divergent trees so reading the
// wrong source is observable (spec: source-separation contrast).
```

Build the fixture with genuinely different integration and metadata trees (a change record that exists ONLY on the docket branch, and a same-named path with different bytes on integration).

- [ ] **Step 2: Run to verify it fails or is vacuous against current code** — if it passes pre-change, strengthen the fixture until deleting the docket-branch copy of the record makes it red (prove the assert CAN fail before trusting it).

- [ ] **Step 3: Implement the removal** across the file list above. Rules: (a) never substitute `pin.IntegrationBranch` or `pin.DefaultBranch` where `metadataBranchOf` stood — the replacement is always the fixed constant; (b) `sourceRevision(pin, source)` keeps its integration arm untouched (merged plans/results still link to integration — the spec forbids flattening real artifact-location variance); (c) delete the `metadataModeMain`/`metadataModeDocket` const block and its comment.

- [ ] **Step 4: Run** `go test -count=1 ./internal/app/ ./internal/config/` and fix compile fallout in tests mechanically (constructor calls that set `Mode:`/`MetadataBranch:` on pins). Where a test's premise died with the field, defer deletion to Task 6 unless it blocks compilation — then delete it now and note it in the task report (learning: `test-premise-deleted-not-regated` — ask what the block guards before deleting).

- [ ] **Step 5: Verify absence** — run the grep from Produces; expect zero non-test hits.

- [ ] **Step 6: Commit** — `git add -u internal/ && git commit -m "refactor(0363): remove StatusPin.Mode and collapse metadataBranchOf to the fixed docket branch"`.

---

### Task 5: Protocol contraction — remove the three JSON keys and the human `mode:` line

**Files:**
- Modify: `internal/app/status.go` (`StatusContext`: drop `MetadataMode`, drop `MetadataBranch`; keep `MetadataRevision`, default/integration identity; update `contextFromPin`)
- Modify: `internal/app/status_result.go` (drop the `metadata_branch` tagged field noted in the inventory)
- Modify: `internal/app/status_human.go` (drop `mode: %s` line; keep/emit `metadata branch: docket @ <rev>` as explanatory identity, sourced from `reposetup.MetadataBranchName`)
- Modify: `internal/app/config.go` (drop the `leafLine("metadata_branch", …)` row from effective-config human output; the effective JSON key vanished with Task 2's `Effective` field)
- Test: `internal/app/status_test.go`, `internal/app/status_human_test.go`, `internal/app/config_test.go`

**Interfaces:**
- Consumes: Task 4's pin shape.
- Produces: serialized status/config documents with `config.effective.metadata_branch`, `status.context.metadata_mode`, `status.context.metadata_branch` ABSENT (not empty, not constant), `metadata_revision` retained.

- [ ] **Step 1: Write the failing public-protocol absence tests**

```go
// TestStatusProtocolOmitsModeFields: run the status operation against a
// healthy fixture, marshal the result, and assert over the raw bytes:
//   !bytes.Contains(b, []byte(`"metadata_mode"`))
//   !bytes.Contains(b, []byte(`"metadata_branch"`))   // context AND config.effective
// with non-vacuity companions through the same bytes:
//   bytes.Contains(b, []byte(`"metadata_revision"`))
//   bytes.Contains(b, []byte(`"integration_branch"`))
```

Assert on the SERIALIZED document end-to-end (the same bytes a client reads), not on struct fields — that is what makes re-adding a tagged field redden.

- [ ] **Step 2: Run to verify failure** — `go test -count=1 ./internal/app/ -run 'ProtocolOmits'` — RED (fields still emitted).

- [ ] **Step 3: Implement** the removals; update `HumanText` in `status_human.go` (drop the `mode:` fprintf; render `metadata branch: docket @ <short-rev>` unconditionally from the constant + `MetadataRevision`).

- [ ] **Step 4: Run** the package tests with `-count=1`; update human-output goldens for current behavior.

- [ ] **Step 5: Mutation probe** — re-add `MetadataMode string \`json:"metadata_mode"\`` to `StatusContext` (with `contextFromPin` filling it) via a `cp`-backed edit, confirm `TestStatusProtocolOmitsModeFields` reddens, restore.

- [ ] **Step 6: Commit** — `git add -u internal/app/ && git commit -m "refactor(0363): remove mode-shaped protocol fields from status and config output"`.

---

### Task 6: Test contraction — collapse matrices to the docket row without coverage loss

**Files:**
- Modify: the `internal/app/*_test.go` files from Task 1's inventory that construct main-mode pins or iterate a two-mode matrix (expected: `status_test.go`, `status_corpus_test.go`, `status_human_test.go`, `change_*_test.go`, `link_context_test.go`, `learning_ops_test.go`, `adr_ops_test.go`, `change_integration_test.go`, and integration/race suites)
- Modify: `tests/test_docket_metadata_branch.sh` and any shell test from the inventory whose asserts grep mode prose

**Interfaces:**
- Consumes: Task 1's classification; Tasks 4–5's shapes.
- Produces: a suite where mode-independent invariants (exact-lease CAS, private transaction worktrees, ref isolation, retries, interruption recovery, finalization, cleanup, link repair) remain covered on docket topology; zero surviving tests whose subject is the retired topology.

- [ ] **Step 1: Partition, per the spec's four categories, every test the inventory flagged.** For each file, record the category in the task report:
  1. Generic tests using a main-mode pin as convenience context → switch fixture to a docket-topology pin, keep every assertion.
  2. Genuine two-mode matrices → BEFORE deleting the main row, confirm (or add) the docket-row compensating assert and mutation-test it, THEN collapse (learning: `compensating-assert-must-exist-when-cited` — compensation first, relaxation second; point any "covered by" comment at the assert by name).
  3. Same-ref main-mode-only subject → DELETE, never re-gate under a misleading name (learning: `test-premise-deleted-not-regated`).
  4. Docket source-separation → strengthen: where the former main row accidentally supplied the contrast, give the docket fixture genuinely divergent integration/metadata trees (Task 4 Step 1's fixture pattern) so reading the wrong source stays observable.

- [ ] **Step 2: Execute the partition file-by-file, running `go test -count=1` per package as you go.** For shell tests: `restatement-accumulates-its-own-guards` — where a shell assert greps deleted README/skill prose, REPOINT it at the surviving canonical owner (or delete with the category-3 rule); never restore deleted prose to keep a grep green.

- [ ] **Step 3: Verify frozen fixtures untouched** — `git diff --stat e486cd47 -- testdata/ internal/config/testdata/ 2>/dev/null` shows nothing under any `v0.9.*` tree (additions of a NEW `v0.9.5` tree in Task 8 are the only permitted testdata change).

- [ ] **Step 4: Mutation probes on the strengthened separation fixtures** — alter the fixed metadata source (point a read at integration): the separation tests redden; remove the operational refusal (Task 3's probe repeated at HEAD of this task): refusal tests redden. Backup/verify/restore discipline as always.

- [ ] **Step 5: Run the full Go suite** — `go test -count=1 ./...`.

- [ ] **Step 6: Commit** — `git add -u . && git commit -m "test(0363): collapse mode matrices to docket topology and strengthen source separation"`.

---

### Task 7: Mutation-tested structural guard

**Files:**
- Create: `internal/app/mode_shape_guard_test.go` (Go-native guard — it can walk packages; a shell guard would re-implement Go parsing)

**Interfaces:**
- Consumes: the post-Task-4 tree.
- Produces: a guard that reddens when production code (a) reintroduces a mode-shaped conditional/field, or (b) reads a metadata branch from configuration instead of `reposetup.MetadataBranchName`, or (c) bypasses the shared operational loader with its own pin assembly.

- [ ] **Step 1: Write the guard, keyed on syntactic shape, not an enumerated spelling list** (AGENTS.md rule; learning: `byte-pattern-guard-matches-a-spelling`). Concretely:

```go
// TestNoModeShapedProduction walks every non-test .go file under internal/
// and cmd/ (parsing with go/ast, skipping *_test.go and any path containing
// "testdata/") and fails on:
//  (a) a string literal "main" compared against a field/selector whose name
//      contains "Mode" or "Metadata" (ast.BinaryExpr EQL/NEQ shape);
//  (b) a struct field or JSON tag matching metadata_mode|metadata_branch
//      outside internal/reposetup (the tombstone/migration owner) and
//      internal/config's diagnostics vocabulary (the obsolete-setting code);
//  (c) any call constructing StatusPin{} outside operational_context.go /
//      status_git.go (the loader and its adapter are the only assemblers).
// Each exclusion is an explicit path list asserted non-empty (population
// floor), and the guard asserts it VISITED >0 files per root so a moved
// directory cannot make it vacuously green.
```

Exclusions must be bounded (exact paths, never a wildcard that swallows future source) and the frozen corpus excluded as data (learning: `frozen-fixture-corpus-trips-repo-wide-scans`).

- [ ] **Step 2: Run against the current tree** — `go test -count=1 ./internal/app/ -run 'NoModeShaped'` — PASS (the tree is clean after Tasks 4–5); if it fails, the hit is either a missed removal (fix it) or a legitimate site (tighten the shape, never allowlist the spelling).

- [ ] **Step 3: Mutation-test the guard — all three arms, by ADDITION here** (the guard's job is detecting reintroduction): (a) plant `if pin.Mode == "main" {}`-shaped code in a production file; (b) plant a `json:"metadata_mode"` tag on a DTO; (c) plant a `StatusPin{...}` literal in `change_create.go`. Each must redden the guard individually. Then mutate the guard's own population (point it at a nonexistent root) and confirm the visited-count floor reddens — an unfalsifiable walker is decoration. `cp`-backup, `git diff` proof, `-count=1`, restore for every probe; record the 4-cell matrix in the task report.

- [ ] **Step 4: Commit** — `git add internal/app/mode_shape_guard_test.go && git commit -m "test(0363): structural guard against mode-shaped production code"`.

---

### Task 8: Current documentation, examples, and self-config for one topology

**Files:**
- Modify: `README.md` (inventory hits — at minimum the lines found at grep: the config reference block (`metadata_branch: docket …` sample), the "no `.docket.yml` at all" default paragraph, the per-repo-only fenced-keys list, the single-branch opt-out section (`metadata_branch: main` block + bootstrap-guard paragraph), and the "`main`-mode remains a simple, fully-supported opt-out" closing claim — replace opt-out prose with the one-topology statement + `docket repository migrate` prerequisite)
- Modify: `.docket.yml` (delete the `metadata_branch: docket` line — the setting no longer exists)
- Modify: any maintained skill/script/agent-contract doc the Task 1 inventory classified as maintained prose (classify each hit BEFORE editing; historical records untouched; a prose guard copied from a former owner is relocated to the actual contract owner, not restored as stale wording)
- Create: `testdata/repositories/v0.9.5/docket-self/**` — new versioned fixture tree, copied from `v0.9.4/docket-self` with only `.docket.yml` changed, plus `PROVENANCE.md`
- Modify: `internal/config/fixtures_test.go` (`TestFixtureDocketSelf` re-pointed to `v0.9.5`, expectations re-derived by RUNNING the resolver, never guessed)
- Modify: CLI help text — grep `cmd/` and `internal/cli/` for topology prose (the inventory's list; the Task 1 grep of Go sources found none, but re-verify against long-form help strings: `grep -rn "metadata" internal/cli/ cmd/ --include="*.go" | grep -v _test`)

**Interfaces:**
- Consumes: Task 1's three-way classification; Task 2's tombstone semantics (docs must describe the obsolete key as migration input, not an option).
- Produces: no maintained surface documents `metadata_branch` as a setting; the migration path is the documented legacy exit.

- [ ] **Step 1: Re-derive the maintained-prose hit list at HEAD** (Task 1's list plus anything Tasks 2–7 touched):

```bash
grep -rn "metadata_branch\|main mode\|main-mode" README.md .docket.yml docs/ skills/ scripts/ tests/ \
  | grep -v "docs/results/\|docs/changes/\|docs/adrs/\|docs/superpowers/specs/\|testdata/"
```

Classify each remaining hit (maintained vs historical) in the task report before editing anything.

- [ ] **Step 2: Edit the maintained surfaces.** README: one topology, `integration_branch` flexibility retained, and a "Migrating a legacy single-branch repository" pointer to `docket repository migrate`. `.docket.yml`: remove the key (this is the edit that trips the drift guard — proceed to Step 3, which is the guard's own remedy).

- [ ] **Step 3: Cut the new fixture tree** — `cp -R testdata/repositories/v0.9.4/docket-self testdata/repositories/v0.9.5/docket-self`, overwrite only the copied repo `.docket.yml` with the new live bytes, carry everything else verbatim, write `testdata/repositories/v0.9.5/PROVENANCE.md` (source version, date, change id 0363, exactly which file changed and why). Re-point `TestFixtureDocketSelf`'s `docketSelfRoot`, run `go test -count=1 ./internal/config/ -run 'DocketSelf'`, and paste the resolver's ACTUAL output into the expectations. Confirm `v0.9.4/**` is byte-untouched: `git diff e486cd47 -- testdata/repositories/v0.9.2 testdata/repositories/v0.9.3 testdata/repositories/v0.9.4` is empty.

- [ ] **Step 4: Re-run the shell suite files that grep README/config prose** (from Step 1's tests/ hits — at minimum `tests/test_docket_metadata_branch.sh`, `tests/test_docket_config.sh`, `tests/test_docket_example_yml.sh`, `tests/test_docket_root.sh`): repoint or delete per Task 6's partition rules.

- [ ] **Step 5: Commit** — `git add -A README.md .docket.yml testdata/repositories/v0.9.5 internal/config/ docs/ skills/ scripts/ tests/ && git commit -m "docs(0363): one-topology documentation and v0.9.5 self-fixture"` (list paths explicitly as shown — never a bare `git add -A` from the root without reviewing `git status` first; another task's scratch must not ride along).

---

### Task 9: Layer-stranding check and results-file obligations

**Files:**
- No production edits. Output feeds the results file and merge-gate notes.

**Interfaces:**
- Consumes: Task 2's tombstone behavior.
- Produces: recorded merge-gate actions for outer config layers.

- [ ] **Step 1: Check the machine layers this PR cannot reach** (learning: `config-shape-change-strands-outer-layers` — the suite is hermetic and cannot see them): `grep -n "metadata_branch" ~/.config/docket/config.yml .docket.local.yml 2>/dev/null`. The tombstone means a stale key degrades to an `obsolete-setting` warning, never a hard failure — verify that claim by running the built binary's config inspection (`go run ./cmd/docket config` or the repo's equivalent command — read `internal/cli` for the exact verb) against a temp dir carrying a global-layer `metadata_branch`.

- [ ] **Step 2: Record in the task report** (for the results file): (a) the outer-layer findings and that the tombstone is the shim; (b) that metadata-branch-side behavior (board/backlink rendering against the real `docket` branch) was verified at build time where the hermetic suite cannot see it (learning: `metadata-branch-invisible-to-suite`) — run one real `docket status` against this repository and confirm the human output shows `metadata branch: docket @ <rev>` and no `mode:` line.

---

### Task 10: Decision records — flag, do not mint

**Files:**
- No ADR files are created on this branch. This task produces the recorded flag the review step consumes.

**Interfaces:**
- Consumes: the spec's "Documentation and decision records" section.
- Produces: an entry in the task report / results notes that the review step MUST run the `docket-adr` workflow.

- [ ] **Step 1: Record the ADR obligations verbatim for the review step:**
  - New ADR superseding ADR-0002: restates the surviving default/bootstrap rules, removes the pinned main-mode opt-out, makes native `docket repository migrate` the only legacy exit; `related:` ADR-0001 and ADR-0052. Id allocated by the workflow, then attached to change 0363's `adrs:` frontmatter.
  - ADR-0001: stays Accepted; receives a dated `## Update` (never a Decision rewrite — learning: `adr-update-delivery`) pointing at the new ADR and noting the opt-out consequence no longer applies. Both deliverables travel via the change's `adrs:` list so they land atomically.
  - ADR-0069 (mode-conditioned clause discriminates on provenance) is in the change's `adrs:` — check at review whether the new ADR should note that the discriminating clause's subject was removed; do not edit ADR-0069 itself.

---

### Task 11: Full-suite build gate

**Files:** none (gate).

- [ ] **Step 1: Run the whole configured suite** — `scripts/run-tests.sh` (the command `finalize.test_command` resolves to; read it there if it has changed). Never only the tests this plan enumerated.

- [ ] **Step 2: Read the budget output** — any `BUDGET WATCH:` / `PARALLEL-SENSITIVE:` line is a screening finding; a `SERIAL CONFIRMED OVER BUDGET:` line is an authoritative breach to act on even at exit 0. Trust the `SUITE …` summary line, not a piped exit code.

- [ ] **Step 3: Final absence sweep** — re-run Task 4 Step 5's grep and Task 8 Step 1's grep at HEAD; both must be clean of maintained-surface hits. Run `go test -race -count=1 ./...` once (the race gate also must defeat the cache).

- [ ] **Step 4: Verify the branch delta against the acceptance boundary** — walk the spec's ten acceptance bullets and point each at the commit/test that satisfies it; record the mapping in the task report for the results file.

---

## Self-Review (performed at plan time)

- **Spec coverage:** purpose/boundary → Tasks 2–8; one-topology constants rule → Task 3 (`reposetup.MetadataBranchName`), Task 4; tombstone contract (5 bullets) → Task 2; shared gate + ordered read + command-family table → Task 3; typed refusal + fail-closed ordering → Task 3 Steps 1/3; operational/protocol contraction (5 removals + 3 JSON keys + human output) → Tasks 4–5; test contraction categories 1–8 → Tasks 2 (cat. 6), 3 (cat. 7), 5 (cat. 5), 6 (cats. 1–4), 7 (cat. 8); frozen fixtures / v0.9.5 protocol → Task 8; docs → Task 8; ADR → Task 10; suite gate → Task 11. Exclusions respected: no reposetup redesign (Task 3 reuses), no `integration_branch` constraint (Task 4 Step 3 rule b), no historical rewrites (Global Constraints + Task 1 bucket 3).
- **Type consistency:** `reposetup.MetadataBranchName` introduced in Task 3, consumed in Tasks 4, 5, 7. `loadOperationalContext` / `errRepositoryNotOperational` named once (Task 3) and referenced nowhere else by signature. `PinContext(ctx, repoDir) (StatusPin, error)` unchanged throughout.
- **Placeholder scan:** inventory-driven tasks (1, 6, 8) intentionally derive their file lists from the Task 1 grep rather than hand-listing — that is the AGENTS.md "never hand-list the sites" rule, not a placeholder; every code-bearing step carries the concrete shape or the exact command.
