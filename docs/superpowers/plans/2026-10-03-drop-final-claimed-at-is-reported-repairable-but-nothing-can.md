<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0496 — Add `docket repository repair` and stop flagging empty claimed_at](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0496-drop-final-claimed-at-is-reported-repairable-but-nothing-can.md)**
<!-- docket:backlink:end -->
# `docket repository repair` and the empty-claimed_at false positive — Implementation Plan

> **For agentic workers:** this plan is executed by **docket-build**: one tier-routed worker per
> task, strictly sequential in the feature worktree, one commit per task, no per-task review, and a
> single full-suite gate at the end. Each task names its **Risk** (the routing tier). Steps use
> checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stop `repository check` flagging the empty `claimed_at:` cleared form, add
`docket repository repair` (catalog operation `repository.repair`) as the one repair path on a
migrated repository (frontmatter roster + derived views, one lease-guarded commit), make `migrate`
only migrate, and point every repair remedy at the new command.

**Architecture:** (1) `reposetup.planClaimedAt` decides on the parsed value, so an empty value
(bare key, `''`, `""`, `null`, `~`) is no stamp, the same as the decode layer's `FieldEmpty`.
(2) A new `internal/app/repository_repair.go` service reuses `migrateRoute` for topology,
`readCheckCorpus` + `corpusFindings` + `derivedViewFindings` for the exact set `check` reports, and
a pure `planRepositoryRepair` that applies frontmatter repairs first and recomputes derived views
over the repaired corpus. It publishes through the lease machinery that today's
`executeDerivedRepair` uses, moved and generalized. (3) A new cobra leaf `repository repair` with the
same two-pass confirm flow as `migrate`, plus catalog, schema, and install registration.
(4) `migrate`'s already-migrated branch becomes a plain no-op that names the new command.
(5) Go remedies, skills, glossary, the embedded asset mirror, and the capability-surface exemption
pin follow.

**Tech Stack:** Go (module `github.com/danielhanold/docket`), `go test` (with `-tags integration`
for the real-git app tests), `go generate ./internal/assets/` for the embedded skill mirror, and the
Go suite runner (`go run ./cmd/docket development test`), which docket-build runs at the gate. No
task runs it.

**Spec:** `docs/superpowers/specs/2026-10-03-drop-final-claimed-at-is-reported-repairable-but-nothing-can-design.md`
(on the `docket` metadata branch; the synchronized copy is under `.docket/` in the primary
checkout). Executors read the spec beside this plan. Change file:
`docs/changes/active/0496-drop-final-claimed-at-is-reported-repairable-but-nothing-can.md`.

## Global Constraints

- **An empty `claimed_at:` is not a claim stamp, everywhere.** Closeout keeps the uniform
  bare-null cleared form. No task changes closeout and no task rewrites the archived records.
- **The repair roster's membership and rules are unchanged.** Only `planClaimedAt`'s
  empty-value gate is new. `buildRepair`, `ApplyRepairs`, `RepairDigest`, and the roster codes are
  untouched.
- **No new blocking gate.** Frontmatter and derived-view findings stay visibility-only warnings
  (or errors for manual review) exactly as today. `CheckExit` is unchanged.
- **One publisher.** `repository repair` publishes through the `BuildTree` → `CommitTree` →
  `PushLease` (exact lease on the pinned tip) → `FetchBranch` re-read sequence that
  `executeDerivedRepair` uses today. Move and generalize it. Do not write a second publisher.
  A moved tip is `contended`, never an overwrite.
- **Repair commit subject is exactly `docket: repository repair`.**
- **Result vocabulary:** `applied` | `no-op` | `contended` | `invalid-state` | `external-failed` |
  `internal-error` (the existing `Result*` constants), plus `unsupported-config` / `invalid-input`
  only where the shared gather-failure mapping already produces them.
- **Catalog operation `repository.repair`, effects exactly `metadata-write`, signature
  `[--repo-dir <dir>] [--yes]`.** No other flag.
- **`migrate` only migrates.** On an already-migrated repository it never writes, with or without
  `--repair-frontmatter` / `--yes`, and its human output is exactly
  `repository already migrated; for mechanical repairs run docket repository repair`. The legacy
  path (`gatherMigrationRepairs`, `decideMigrateAuthorization`,
  `migrateRepairAuthorizationRequired`) is unchanged.
- **Agent-executed skill surfaces name the catalog id `repository.repair`**, never a hard-coded
  `docket repository repair` argv (ADR-0104). Never add a `docket repository repair` entry to
  `capabilityExemptions`.
- **Remedies that legitimately name `migrate` stay:** legacy, partial/half-migrated, and
  local-attachment recovery remedies are not touched.
- **ADR work is out of scope for every build task.** The new ADR ("repair on a migrated repository
  is `repository repair`; `migrate` only migrates") is recorded by the coordinator through
  docket-adr after the build. No task creates or edits anything under `docs/adrs/`.
- **Frozen records are never edited:** `docs/superpowers/plans/**` (except this plan),
  `docs/superpowers/specs/**`, `docs/results/**`, `docs/changes/**`, and Accepted ADR bodies.
- **Broader README / `docs/guide/**` alignment is out of scope** (change 0464). Only the glossary
  lines named in Task 7 change.
- **Comment anchors** name a symbol or quote a clause, never a line number (ADR-0054).
- **Every test run that observes a change in outcome defeats the cache:** `go test -count=1 …`.
- **Mutation restore uses a backup copy**, never `git checkout --`:
  `cp <file> <file>.bak` → apply the mutation → confirm it landed (`grep -c` before/after) → run
  the test (no `(cached)`) → `mv -f <file>.bak <file>` → confirm the restored run is green.
- **Integration tests keep their shard prefixes.** A `repository repair` real-git test name starts
  with `TestIntegrationRepoRepair` (new shard, Task 4). A migrate test starts with
  `TestIntegrationRepoMigration`. Run integration tests filtered, never unfiltered (the
  `internal/app` integration binary refuses an unfiltered run):
  `go test -tags integration -count=1 -run '^TestIntegrationRepoRepair' ./internal/app/`.
- **Tasks run focused tests only.** The whole suite runs once at docket-build's gate through
  `build.test_command`. Read every `BUDGET WATCH:` and `SERIAL CONFIRMED OVER BUDGET:` line there,
  and report the new `tests/test_go_integration_app_reporepair.sh` row's measured margin as a
  number.
- **Plan-supplied test code is a draft.** If a supplied fixture record trips an unexpected
  error-severity snapshot finding, fix the *fixture* (add the missing field), never weaken the
  assertion. Then mutation-test the assert's key as the step says.

## Review Focus

1. **Every empty spelling of `claimed_at` on an archived record.** A human or a past writer may
   have left `claimed_at: ''`, `claimed_at: null`, or `claimed_at: ~`, not only the bare key, and
   on a non-final archived record as well as a final one. None of these is a stamp, so none should
   produce a finding (not even manual review). Pinned in Task 1
   (`TestRepairDropClaimedAtEmptyValueIsNoStamp`, every spelling × done/killed/in-progress).
2. **A record that needs a frontmatter fix and an `## Artifacts` re-render in the same pass.** The
   written blob must be the composed bytes, not one fix overwriting the other, and a following
   `check` must report neither. Pinned in Task 2 (`TestPlanRepositoryRepairComposesFrontmatterAndArtifactLinks`)
   and end to end in Task 4 (`TestIntegrationRepoRepairAppliesOneDescendantCommit`).
3. **A record under an unbalanced managed marker.** It must be listed as manual review and left
   byte-identical, while the other repairs in the same run still apply. Pinned in Task 2
   (`TestPlanRepositoryRepairMalformedMarkerIsManualOnly`) and Task 4
   (`TestIntegrationRepoRepairMalformedMarkerIsManualReview`).
4. **A human runs `repository repair` interactively on a clean repository.** It should report the
   no-op and never prompt `repair? [y/N]` for nothing. Pinned in Task 3
   (`TestRepositoryRepairInteractiveNoPromptWithoutConfirmation`).
5. **A human runs `migrate --repair-frontmatter --yes` on a migrated repository out of habit.**
   It must write nothing and name `docket repository repair` rather than silently ignoring the
   flag. Pinned in Task 5 (`TestIntegrationRepoMigrationMigratedRepoNeverRepairs`, all four flag
   combinations).

---

## File map

| File | Task | Responsibility |
|---|---|---|
| `internal/reposetup/repair.go` | 1 | `planClaimedAt` ignores an empty value (`claimedAtHoldsValue`) |
| `internal/reposetup/repair_test.go` | 1 | empty-spelling / timestamp / malformed / non-final cases |
| `internal/app/repository_repair.go` (new) | 2, 5 | the `repository.repair` service, pure plan, publisher, result constructors; Task 5 moves the shared derived-repair helpers in |
| `internal/app/repository_repair_test.go` (new) | 2, 5 | pure plan + result-shape unit tests; Task 5 moves the helper tests in |
| `internal/app/schema_registry.go` | 3 | `repository.repair` binding |
| `internal/cli/repository.go` | 3, 5 | `repository repair` leaf + confirm flow; Task 5 rewords the `--repair-frontmatter` help |
| `internal/cli/install.go` | 3 | `"repository repair": true` known command |
| `internal/cli/repository_test.go` | 3 | registration, annotation, `--yes`, preview, confirm, no-prompt tests |
| `internal/app/repository_repair_integration_test.go` (new) | 4 | real-git repair shard + moved drift helpers |
| `tests/test_go_integration_app_reporepair.sh` (new) | 4 | shard runner, prefix `TestIntegrationRepoRepair` |
| `tests/runtime-budgets.tsv` | 4 | budget row for the new shard |
| `internal/app/repomigration_integration_test.go` | 4, 5 | helpers move out (4); healthy-repair tests replaced by the never-repairs test (5) |
| `internal/app/repository_migrate.go` | 5 | already-migrated → `migrateNoOp`; new no-op text; drop `RepairedViews`; `MigrateOptions` doc |
| `internal/app/repository_migrate_repair.go` | 5 | deleted (helpers moved to `repository_repair.go`) |
| `internal/app/repository_migrate_repair_test.go` | 5 | deleted (helper tests moved) |
| `internal/app/repository_migrate_test.go` | 5 | `migrateNoOp` names the repair command |
| `internal/reposetup/health.go`, `internal/reposetup/derived.go` | 6 | repairable remedies name `docket repository repair` |
| `internal/app/status.go`, `internal/app/status_branch_malformed_test.go` | 6 | branch-malformed hand-edit remedy |
| `internal/app/repair_remedy_family_test.go` (new) | 6 | family-keyed guard: no repairable-family remedy names `repository migrate` |
| `skills/docket-adr/SKILL.md`, `skills/docket-status/SKILL.md`, `skills/docket-convention/SKILL.md` | 7 | retarget to the `repository.repair` operation |
| `docs/reference/glossary.md` | 7 | the two repair command lines |
| `internal/assets/embedded/**` | 7 | regenerated by `go generate ./internal/assets/` |
| `internal/repoguard/capability_surface_test.go` | 7 | `docket repository migrate` exemption count → measured value |

---

### Task 1: `planClaimedAt` treats an empty `claimed_at` as no stamp

**Risk:** standard. It is a small predicate, but it decides a reported finding on every closed
change. Getting the empty/malformed split wrong either keeps the false positive or hides a real
stale lease.

**Files:**
- Modify: `internal/reposetup/repair.go` (`planClaimedAt`; add `claimedAtHoldsValue`; the roster
  comment for `drop-final-claimed-at`)
- Test: `internal/reposetup/repair_test.go`

**Interfaces:**
- Consumes: `document.Document.Field`, `document.Document.DecodeFrontmatter`,
  `document.ShapeEmpty`, `yaml.Node` (all already imported in `repair.go`).
- Produces: `func claimedAtHoldsValue(doc document.Document, f document.Field) bool` (unexported,
  package `reposetup`). `PlanRepairs` keeps its exported signature
  `PlanRepairs(path string, src []byte, archived bool) ([]RepairFinding, error)`. Task 2 relies on
  it returning **no** `claimed_at` finding for an empty value.

- [ ] **Step 1: Write the failing tests**

Append to `internal/reposetup/repair_test.go`:

```go
// TestRepairDropClaimedAtEmptyValueIsNoStamp pins change 0496: an EMPTY claimed_at
// is docket's deliberate cleared form (closeout's clearClaimedAt writes the bare
// null), and the decode layer reads every empty spelling as FieldEmpty — "no
// stamp". planClaimedAt must agree: no finding at all (neither repairable nor
// manual review), for every empty spelling, on final AND non-final archived
// records. Mutation probe: delete the claimedAtHoldsValue gate in planClaimedAt —
// every subtest here must redden.
func TestRepairDropClaimedAtEmptyValueIsNoStamp(t *testing.T) {
	spellings := []struct{ name, line string }{
		{"bare key", "claimed_at:"},
		{"bare key trailing space", "claimed_at: "},
		{"single-quoted empty", "claimed_at: ''"},
		{"double-quoted empty", `claimed_at: ""`},
		{"null keyword", "claimed_at: null"},
		{"tilde", "claimed_at: ~"},
	}
	for _, sp := range spellings {
		for _, status := range []string{"done", "killed", "in-progress"} {
			t.Run(sp.name+"/"+status, func(t *testing.T) {
				src := "---\nid: 7\nstatus: " + status + "\n" + sp.line + "\n---\nbody\n"
				fs := planOne(t, "docs/changes/archive/2026-08-01-0007-x.md", src, true)
				for _, f := range fs {
					if f.Field == "claimed_at" {
						t.Fatalf("empty claimed_at (%q) on a %s archived record must yield no finding, got %+v", sp.line, status, f)
					}
				}
			})
		}
	}
}

// TestRepairDropClaimedAtNonEmptyUnchanged proves the empty-value gate leaves every
// NON-empty value on the existing path: a timestamp and a malformed non-empty
// value on a final archived record still yield the repairable drop with the
// unchanged patch preview; a timestamp on a non-final archived record is still a
// manual-review finding.
func TestRepairDropClaimedAtNonEmptyUnchanged(t *testing.T) {
	for _, tc := range []struct{ name, value string }{
		{"timestamp", "2026-08-01T10:00:00Z"},
		{"malformed non-empty", "not-a-timestamp"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := "---\nid: 7\nstatus: done\nclaimed_at: " + tc.value + "\n---\nbody\n"
			fs := planOne(t, "docs/changes/archive/2026-08-01-0007-x.md", src, true)
			f := repairableWithCode(t, fs, RepairDropClaimedAt)
			if want := "-claimed_at: " + tc.value + "\n"; string(f.Patch) != want {
				t.Errorf("Patch = %q, want %q (preview must be unchanged)", f.Patch, want)
			}
		})
	}
	t.Run("non-final timestamp is manual review", func(t *testing.T) {
		src := "---\nid: 7\nstatus: in-progress\nclaimed_at: 2026-08-01T10:00:00Z\n---\nbody\n"
		fs := planOne(t, "docs/changes/archive/2026-08-01-0007-x.md", src, true)
		var got *RepairFinding
		for i := range fs {
			if fs[i].Field == "claimed_at" {
				got = &fs[i]
			}
		}
		if got == nil || got.Repairable {
			t.Fatalf("non-final archived timestamp must stay a manual-review finding, got %+v", fs)
		}
	})
}
```

- [ ] **Step 2: Run the tests and confirm the empty cases fail**

Run: `go test -count=1 -run 'TestRepairDropClaimedAt' ./internal/reposetup/`
Expected: FAIL. `TestRepairDropClaimedAtEmptyValueIsNoStamp` subtests report a `claimed_at`
finding (repairable for done/killed, manual review for in-progress).
`TestRepairDropClaimedAtNonEmptyUnchanged` passes already.

- [ ] **Step 3: Implement the gate**

In `internal/reposetup/repair.go`, change `planClaimedAt` and add the helper below it:

```go
// planClaimedAt decides the drop-final-claimed-at roster entry. An EMPTY
// claimed_at (the bare null closeout's clearClaimedAt writes, or any other
// spelling the decode layer reads as FieldEmpty) is not a claim stamp and is
// never a finding — the snapshot validator raises CodeChangeFinalClaimStamp only
// for FieldPresent, and this planner agrees (change 0496).
func planClaimedAt(path string, src []byte, doc document.Document, archived bool) (RepairFinding, bool) {
	f, ok := doc.Field("claimed_at")
	if !ok {
		return RepairFinding{}, false
	}
	if !claimedAtHoldsValue(doc, f) {
		return RepairFinding{}, false
	}
	if !archived {
		// An active record legitimately holds a claim lease; leave it alone.
		return RepairFinding{}, false
	}
	if !finalStatus(doc) {
		return RepairFinding{
			Path:       path,
			Field:      "claimed_at",
			Repairable: false,
			Message:    "claimed_at on a non-final archived record; needs manual review",
		}, true
	}
	candidate, preview, err := buildRepair(doc, src, RepairDropClaimedAt, "claimed_at")
	if err != nil {
		return RepairFinding{}, false
	}
	_ = candidate
	return RepairFinding{
		Path:       path,
		Field:      "claimed_at",
		Code:       RepairDropClaimedAt,
		Repairable: true,
		Message:    "claimed_at on a final archived change; remove the stale claim lease",
		Patch:      preview,
	}, true
}

// claimedAtHoldsValue reports whether the located claimed_at entry carries a
// value, deciding on the PARSED node exactly as the decode layer's
// (*decoder).state classifies a scalar for optionalTime: a valueless key
// (document.ShapeEmpty), a YAML null (`null`, `~`), and an empty string (`''`,
// `""`) are FieldEmpty — no stamp. Anything else holds a value: a well-formed
// timestamp (FieldPresent) or a malformed non-empty value, including a
// collection (FieldMalformed). An undecodable frontmatter block fails toward
// "holds a value", which keeps the pre-0496 finding rather than hiding one.
func claimedAtHoldsValue(doc document.Document, f document.Field) bool {
	if f.Shape == document.ShapeEmpty {
		return false
	}
	var m map[string]yaml.Node
	if err := doc.DecodeFrontmatter(&m); err != nil {
		return true
	}
	n, ok := m["claimed_at"]
	if !ok {
		return false
	}
	if n.Kind != yaml.ScalarNode {
		return true
	}
	return n.Tag != "!!null" && n.Value != ""
}
```

Also update the roster comment near the top of the file:

```go
//	drop-final-claimed-at      a NON-EMPTY claimed_at removed from an
//	                      already-final (done/killed) archived change; an empty
//	                      claimed_at is the cleared form and is never a finding.
```

- [ ] **Step 4: Run the tests and confirm they pass**

Run: `go test -count=1 ./internal/reposetup/`
Expected: PASS (the whole package, including `TestRepairDropClaimedAtEligible`,
`TestRepairDropClaimedAtRefused`, and `TestRepairWriterCheckerParity`).

- [ ] **Step 5: Mutation-test the gate**

Back up `internal/reposetup/repair.go`. Delete the three lines
`if !claimedAtHoldsValue(doc, f) { return RepairFinding{}, false }` from `planClaimedAt`, and
confirm with `grep -c 'claimedAtHoldsValue(doc, f)' internal/reposetup/repair.go` (1 → 0). Run
`go test -count=1 -run TestRepairDropClaimedAtEmptyValueIsNoStamp ./internal/reposetup/`. It must
FAIL in every subtest. Restore with `mv -f`, re-run, and confirm it is green.

- [ ] **Step 6: Commit**

```bash
git add internal/reposetup/repair.go internal/reposetup/repair_test.go
git commit -m "fix(0496): an empty claimed_at is not a claim stamp in the repair planner"
```

---

### Task 2: The `repository.repair` service

**Risk:** premium. This code publishes to the shared metadata branch. A composition or lease
mistake can overwrite a concurrent write or ship non-canonical bytes. It can be rolled back, but
the cost is real.

**Files:**
- Create: `internal/app/repository_repair.go`
- Test: `internal/app/repository_repair_test.go` (new)

**Interfaces:**
- Consumes (all package `app`, unchanged in this task):
  `GatherSetupFacts(ctx, d SetupDeps, forMutation bool) (reposetup.Facts, setupContext, error)`;
  `migrateRoute(ctx, git *gitcli.Client, facts reposetup.Facts, sc *setupContext) (migratePhase, *RepositoryMigrateResult)`
  and the phases `phaseAlreadyMigrated`, `phaseLegacyFull`, `phaseResumePrune`, `phaseResumeLocal`;
  `migrateGatherFailure(err) RepositoryMigrateResult`; `migrateSourceMoved(expected, fresh string) bool`;
  `readCheckCorpus(ctx, git, sc) (checkCorpus, error)`;
  `corpusFindings(cfg config.Effective, recs []corpusRecord) []reposetup.RepairFinding`;
  `derivedViewFindings(cfg config.Effective, corpus checkCorpus) []reposetup.DerivedFinding`;
  `buildCorpusSnapshot(cfg, recs) (domain.Snapshot, bool)`;
  and, still living in `repository_migrate_repair.go` until Task 5:
  `splitDerivedFindings`, `derivedRepairFiles`,
  `composeDerivedRepairBytes(sc setupContext, snap domain.Snapshot, corpus checkCorpus, recByPath map[string]corpusRecord, file string) ([]byte, error)`;
  `indentPatch(patch []byte) string`; `blobMode`; `branchRefPrefix`; `setupRemote()`;
  `reposetup.ApplyRepairs`.
- Produces (Tasks 3–5 rely on these exact names):
  - `const OperationRepositoryRepair = "repository.repair"`
  - `const repositoryRepairSubject = "docket: repository repair"`
  - `type RepairOptions struct { Authorized bool; ExpectedSource string }`
  - `type RepositoryRepairResult struct` (fields below) with methods `HumanText() string`,
    `SourceRev() string`, `ConfirmationRequired() bool`
  - `func RunRepositoryRepair(ctx context.Context, d SetupDeps, o RepairOptions) RepositoryRepairResult`
  - `type repositoryRepairPlan struct` and
    `func planRepositoryRepair(sc setupContext, corpus checkCorpus) (repositoryRepairPlan, error)`
  - constructors `repairNoOp`, `repairConfirmationRequired`, `repairApplied`, `repairContended`,
    `repairRefusal`, `repairExternalFailure`, `repairInternalFailure`, `repairFromMigrateResult`.

- [ ] **Step 1: Write the failing unit tests**

Create `internal/app/repository_repair_test.go`:

```go
package app

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/reposetup"
	"github.com/danielhanold/docket/internal/repository"
)

// repairArchivedDone is a minimal valid ARCHIVED done record (the same shape the
// migration fixtures use) carrying the given claimed_at line.
func repairArchivedDone(claimedLine string) []byte {
	return []byte("---\nid: 3\nslug: archived-change\nstatus: done\ntitle: Change archived-change\ntype: feature\n" +
		claimedLine + "\n---\n\nBody for archived-change.\n")
}

const repairArchivedPath = "docs/changes/archive/2026-01-02-0003-archived-change.md"

func repairLink() render.LinkContext {
	return render.LinkContext{MetadataBranch: reposetup.MetadataBranchName}
}

// TestPlanRepositoryRepairComposesFrontmatterAndArtifactLinks proves a record that
// needs BOTH a frontmatter repair (an unsafe `title: yes`) and an artifact-links
// re-render is written once, as the composed bytes, and that a re-check over the
// composed bytes reports neither finding (Review Focus 2).
func TestPlanRepositoryRepairComposesFrontmatterAndArtifactLinks(t *testing.T) {
	cfg := derivedTestConfig()
	path := "docs/changes/active/0001-example.md"
	fm := strings.Replace(derivedChangeFM, "title: Example change", "title: yes", 1)
	rec := corpusRecord{path: path, bytes: changeRecordBytes(fm, ""), kind: repository.KindChange, location: repository.LocationActive}
	corpus := checkCorpus{records: []corpusRecord{rec}, link: repairLink()}

	plan, err := planRepositoryRepair(setupContext{cfg: cfg}, corpus)
	if err != nil {
		t.Fatalf("planRepositoryRepair: %v", err)
	}
	if len(plan.files) != 1 || plan.files[0] != path {
		t.Fatalf("files = %v, want exactly [%s]", plan.files, path)
	}
	if len(plan.frontmatter) != 1 || plan.frontmatter[0].Code != reposetup.RepairQuoteScalar {
		t.Errorf("frontmatter = %+v, want one quote-unsafe-scalar repair", plan.frontmatter)
	}
	if findingByCode(plan.derived, reposetup.CodeArtifactLinksStale) == nil {
		t.Errorf("derived = %+v, want artifact-links-stale computed over the repaired corpus", plan.derived)
	}
	got := plan.contents[path]
	if !bytes.Contains(got, []byte("title: 'yes'")) || !bytes.Contains(got, []byte("| Spec |")) {
		t.Fatalf("composed bytes must carry BOTH repairs; got:\n%s", got)
	}
	recheck := checkCorpus{records: []corpusRecord{{path: path, bytes: got, kind: repository.KindChange, location: repository.LocationActive}}, link: repairLink()}
	for _, f := range corpusFindings(cfg, recheck.records) {
		t.Errorf("re-check frontmatter finding on composed bytes: %+v", f)
	}
	for _, f := range derivedViewFindings(cfg, recheck) {
		t.Errorf("re-check derived finding on composed bytes: %+v", f)
	}
}

// TestPlanRepositoryRepairBoardRecomputedOverRepairedCorpus proves the board is
// re-rendered from the snapshot of the FRONTMATTER-REPAIRED corpus, so the board
// blob and the record blob land canonical in one pass.
func TestPlanRepositoryRepairBoardRecomputedOverRepairedCorpus(t *testing.T) {
	cfg := derivedTestConfig()
	path, canonical := canonicalChangeRecord(t, cfg)
	stamped := corpusRecord{path: repairArchivedPath, bytes: repairArchivedDone("claimed_at: 2026-08-01T10:00:00Z"), kind: repository.KindChange, location: repository.LocationArchive}
	active := corpusRecord{path: path, bytes: canonical, kind: repository.KindChange, location: repository.LocationActive}
	corpus := checkCorpus{
		records: []corpusRecord{active, stamped},
		link:    repairLink(),
		board:   corpusFile{present: true, bytes: []byte("# Backlog\n\nstale bytes\n")},
	}
	plan, err := planRepositoryRepair(setupContext{cfg: cfg}, corpus)
	if err != nil {
		t.Fatalf("planRepositoryRepair: %v", err)
	}
	board := boardCorpusPath(cfg)
	if !planHasFile(plan.files, board) || !planHasFile(plan.files, repairArchivedPath) {
		t.Fatalf("files = %v, want the board and the stamped archived record", plan.files)
	}
	repairedRecs := []corpusRecord{active, {path: repairArchivedPath, bytes: plan.contents[repairArchivedPath], kind: repository.KindChange, location: repository.LocationArchive}}
	snap, ok := buildCorpusSnapshot(cfg, repairedRecs)
	if !ok {
		t.Fatal("snapshot of the repaired corpus failed")
	}
	want, err := renderCanonicalBoard(snap, corpusBoardUnrenderable(cfg, repairedRecs), boardPresentation(cfg))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !bytes.Equal(plan.contents[board], want) {
		t.Errorf("board bytes are not the canonical render over the repaired corpus")
	}
	if bytes.Contains(plan.contents[repairArchivedPath], []byte("claimed_at")) {
		t.Errorf("stamped archived record still carries claimed_at:\n%s", plan.contents[repairArchivedPath])
	}
}

// TestPlanRepositoryRepairEmptyClaimedAtIsClean proves the 269-record shape — a
// final archived record with the empty cleared claimed_at — plans nothing, while
// the same record with a real stamp (the control) plans the drop.
func TestPlanRepositoryRepairEmptyClaimedAtIsClean(t *testing.T) {
	cfg := derivedTestConfig()
	for _, tc := range []struct {
		name, line string
		wantFiles  int
	}{
		{"empty cleared form", "claimed_at:", 0},
		{"control: real stamp", "claimed_at: 2026-08-01T10:00:00Z", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := corpusRecord{path: repairArchivedPath, bytes: repairArchivedDone(tc.line), kind: repository.KindChange, location: repository.LocationArchive}
			plan, err := planRepositoryRepair(setupContext{cfg: cfg}, checkCorpus{records: []corpusRecord{rec}, link: repairLink()})
			if err != nil {
				t.Fatalf("planRepositoryRepair: %v", err)
			}
			if len(plan.files) != tc.wantFiles {
				t.Errorf("files = %v, want %d", plan.files, tc.wantFiles)
			}
			if len(plan.manual) != 0 {
				t.Errorf("manual = %v, want none", plan.manual)
			}
		})
	}
}

// TestPlanRepositoryRepairMalformedMarkerIsManualOnly proves a record under an
// unbalanced managed marker is listed as manual review and never written, while
// the other repairable finding in the same run (the stale board) still plans
// (Review Focus 3).
func TestPlanRepositoryRepairMalformedMarkerIsManualOnly(t *testing.T) {
	cfg := derivedTestConfig()
	path := "docs/changes/active/0001-example.md"
	src := []byte("---\n" + derivedChangeFM + "---\n\n## Artifacts\n\n<!-- docket:artifacts:start (generated — do not hand-edit) -->\n| Artifact | Link |\n\n## Why\n\nbody\n")
	corpus := checkCorpus{
		records: []corpusRecord{{path: path, bytes: src, kind: repository.KindChange, location: repository.LocationActive}},
		link:    repairLink(),
		board:   corpusFile{present: true, bytes: []byte("# Backlog\n\nstale bytes\n")},
	}
	plan, err := planRepositoryRepair(setupContext{cfg: cfg}, corpus)
	if err != nil {
		t.Fatalf("planRepositoryRepair: %v", err)
	}
	if _, written := plan.contents[path]; written {
		t.Errorf("the malformed record must never be written")
	}
	if !planHasFile(plan.files, boardCorpusPath(cfg)) {
		t.Errorf("files = %v, want the stale board still repaired", plan.files)
	}
	var sawMalformed bool
	for _, m := range plan.manual {
		if strings.HasPrefix(m, "["+reposetup.CodeArtifactLinksMalformed+"] "+path) {
			sawMalformed = true
		}
	}
	if !sawMalformed {
		t.Errorf("manual = %v, want an artifact-links-malformed line for %s", plan.manual, path)
	}
}

// TestRepairConfirmationRequiredNamesYes proves the unauthorized preview is
// invalid-state / confirmation-required, carries the pinned tip, lists both
// repair kinds, and names --yes.
func TestRepairConfirmationRequiredNamesYes(t *testing.T) {
	plan := repositoryRepairPlan{
		frontmatter: []reposetup.RepairFinding{{Path: repairArchivedPath, Field: "claimed_at", Code: reposetup.RepairDropClaimedAt, Repairable: true, Patch: []byte("-claimed_at: x\n")}},
		derived:     []reposetup.DerivedFinding{{Code: reposetup.CodeBoardStale, Path: "docs/changes/BOARD.md", Repairable: true}},
		files:       []string{"docs/changes/BOARD.md", repairArchivedPath},
		contents:    map[string][]byte{},
	}
	out := repairConfirmationRequired(setupContext{}, "abc123", plan)
	if out.Result != ResultInvalidState || !out.ConfirmationRequired() {
		t.Fatalf("preview = %q/%q, want invalid-state/confirmation-required", out.Result, out.RepositoryState)
	}
	if out.SourceRevision != "abc123" || out.SourceRev() != "abc123" {
		t.Errorf("SourceRevision = %q, want the pinned tip", out.SourceRevision)
	}
	if len(out.RepairedFiles) != 2 || len(out.Repairs) != 1 || len(out.RepairedViews) != 1 {
		t.Errorf("preview sets = files %v repairs %v views %v", out.RepairedFiles, out.Repairs, out.RepairedViews)
	}
	h := out.HumanText()
	for _, want := range []string{"--yes", "[drop-final-claimed-at] " + repairArchivedPath, "[board-stale] docs/changes/BOARD.md", "-claimed_at: x"} {
		if !strings.Contains(h, want) {
			t.Errorf("preview human lacks %q:\n%s", want, h)
		}
	}
}

// TestRepairAppliedNamesRevisionAndPending proves the applied document names the
// new and prior tips, the file set, and the `docket repository prepare` sync.
func TestRepairAppliedNamesRevisionAndPending(t *testing.T) {
	out := repairApplied("newtip", "priortip", repositoryRepairPlan{files: []string{"docs/changes/BOARD.md"}})
	if out.Result != ResultApplied || out.MetadataTip != "newtip" || out.SourceRevision != "priortip" {
		t.Fatalf("applied = %+v", out)
	}
	if !strings.Contains(strings.Join(out.PendingLocal, " "), "docket repository prepare") {
		t.Errorf("PendingLocal = %v, want the prepare sync remedy", out.PendingLocal)
	}
}

// TestRepairNoOpListsManualReview proves nothing-repairable is an idempotent
// no-op that still surfaces the manual-review list.
func TestRepairNoOpListsManualReview(t *testing.T) {
	out := repairNoOp("tip", []string{"[artifact-links-malformed] p: m"})
	if out.Result != ResultNoOp {
		t.Fatalf("Result = %q, want no-op", out.Result)
	}
	if !strings.Contains(out.HumanText(), "[artifact-links-malformed] p: m") {
		t.Errorf("no-op human must list manual review: %q", out.HumanText())
	}
}

// TestRepairContendedNamesBothRevisions proves a moved tip is contended and names
// both revisions.
func TestRepairContendedNamesBothRevisions(t *testing.T) {
	out := repairContended("fresh1", "pinned0")
	if out.Result != ResultContended {
		t.Fatalf("Result = %q, want contended", out.Result)
	}
	if h := out.HumanText(); !strings.Contains(h, "fresh1") || !strings.Contains(h, "pinned0") {
		t.Errorf("contended human must name both revisions: %q", h)
	}
}

// TestRepairFromMigrateResultRestampsOperation proves a routed refusal keeps its
// result, state, and remedy but carries the repository.repair operation key.
func TestRepairFromMigrateResultRestampsOperation(t *testing.T) {
	m := migrateRefusal(reposetup.StateConflict, "inspect and resolve it manually")
	out := repairFromMigrateResult(m)
	if out.Operation != OperationRepositoryRepair || out.Result != ResultInvalidState || out.RepositoryState != string(reposetup.StateConflict) {
		t.Fatalf("restamped = %+v", out)
	}
	if h := out.HumanText(); strings.Contains(h, OperationRepositoryMigrate) || !strings.Contains(h, "inspect and resolve it manually") {
		t.Errorf("restamped human = %q", h)
	}
}

// TestRepairResultJSONFieldNames pins the protocol-v1 keys.
func TestRepairResultJSONFieldNames(t *testing.T) {
	out := repairApplied("newtip", "priortip", repositoryRepairPlan{
		frontmatter: []reposetup.RepairFinding{{Path: repairArchivedPath, Code: reposetup.RepairDropClaimedAt, Repairable: true}},
		derived:     []reposetup.DerivedFinding{{Code: reposetup.CodeBoardStale, Path: "docs/changes/BOARD.md", Repairable: true}},
		manual:      []string{"[x] y: z"},
		files:       []string{"docs/changes/BOARD.md", repairArchivedPath},
	})
	raw, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"protocol_version", "operation", "result", "repository_state", "source_revision",
		"metadata_revision", "repairs", "repaired_views", "repaired_files", "manual_review", "pending_local"} {
		if _, ok := decoded[key]; !ok {
			t.Errorf("result JSON missing %q: %s", key, raw)
		}
	}
	if decoded["operation"] != OperationRepositoryRepair {
		t.Errorf("operation = %v", decoded["operation"])
	}
}
```

`containsPath` lives in the integration-tagged `repomigration_integration_test.go`, so a
default-tag unit test cannot see it. The tests above use this unit-visible helper instead. Add it
at the bottom of `repository_repair_test.go`:

```go
// planHasFile reports whether files contains p.
func planHasFile(files []string, p string) bool {
	for _, f := range files {
		if f == p {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Run the tests and confirm they fail to compile**

Run: `go test -count=1 -run 'TestPlanRepositoryRepair|TestRepair' ./internal/app/`
Expected: FAIL (build). `planRepositoryRepair`, `RepositoryRepairResult`, and the constructors
are undefined.

- [ ] **Step 3: Implement the service**

Create `internal/app/repository_repair.go`:

```go
package app

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/reposetup"
)

// repository_repair.go — `docket repository repair`: authorized mechanical repair
// of every repairable finding `docket repository check` reports on an
// ALREADY-MIGRATED repository — the frontmatter repair roster and the derived
// views (inline board, artifact-links blocks, ADR index). Migrate only migrates
// (change 0496); this is the one repair path on a migrated repository.
//
// It reads the SAME corpus check reads (readCheckCorpus at the pinned remote
// docket tip), applies the frontmatter repairs in memory first, recomputes the
// derived-view findings over the REPAIRED corpus, and publishes the composed
// bytes as ONE descendant of the pinned tip under an exact owned lease (learning
// cas-re-read-fresh-origin / decide-and-act-on-the-same-copy). An unauthorized run
// returns the preview; an authorized run re-proves the pinned revision — a moved
// tip is contention, never an overwrite. Non-repairable findings are listed as
// manual review and never touched; a malformed managed marker keeps its
// manual-review posture (AGENTS.md marker order/balance rule).

// OperationRepositoryRepair is the operation key `repository repair` records.
const OperationRepositoryRepair = "repository.repair"

// repositoryRepairSubject is the commit subject of the repair descendant on the
// docket metadata branch.
const repositoryRepairSubject = "docket: repository repair"

// repairConfirmationRequiredState is the repository_state an unauthorized
// preview carries.
const repairConfirmationRequiredState = "confirmation-required"

// RepairOptions carries the two-pass authorization the CLI resolves. Authorized
// is true only via --yes or an interactive confirmed preview; ExpectedSource is
// the pinned metadata tip the preview showed ("" on the first, preview, pass).
type RepairOptions struct {
	Authorized     bool
	ExpectedSource string
}

// RepositoryRepairResult is the protocol-v1 document `repository repair`
// returns. SourceRevision is the pinned metadata tip the run read and keyed on;
// MetadataTip is the published repair descendant. Repairs are the frontmatter
// roster repairs, RepairedViews the derived-view files, RepairedFiles the union
// written (or, on a preview, planned). ManualReview lists what the repair will
// not touch.
type RepositoryRepairResult struct {
	Envelope
	RepositoryState string                    `json:"repository_state"`
	SourceRevision  string                    `json:"source_revision"`
	MetadataTip     string                    `json:"metadata_revision,omitempty"`
	Repairs         []reposetup.RepairFinding `json:"repairs,omitempty"`
	RepairedViews   []string                  `json:"repaired_views,omitempty"`
	RepairedFiles   []string                  `json:"repaired_files,omitempty"`
	ManualReview    []string                  `json:"manual_review,omitempty"`
	PendingLocal    []string                  `json:"pending_local,omitempty"`
	// Findings carries a resolver diagnosis lifted from a gather failure.
	Findings []reposetup.Finding `json:"findings,omitempty"`
	human    string
}

// HumanText renders the human summary.
func (r RepositoryRepairResult) HumanText() string {
	if r.human != "" {
		return r.human
	}
	return fmt.Sprintf("%s: %s (%s)", r.Operation, r.Result, r.RepositoryState)
}

// SourceRev exposes the pinned metadata tip a preview showed, so the CLI's
// interactive confirm flow re-invokes pinned to exactly that copy.
func (r RepositoryRepairResult) SourceRev() string { return r.SourceRevision }

// ConfirmationRequired reports whether this result is a preview awaiting --yes —
// the only result the CLI's interactive flow prompts on.
func (r RepositoryRepairResult) ConfirmationRequired() bool {
	return r.RepositoryState == repairConfirmationRequiredState
}

// repositoryRepairPlan is the pure repair decision over one corpus read: the
// repairable frontmatter findings, the repairable derived-view findings computed
// over the frontmatter-REPAIRED corpus, the manual-review lines, and the final
// composed bytes per written file.
type repositoryRepairPlan struct {
	frontmatter []reposetup.RepairFinding
	derived     []reposetup.DerivedFinding
	manual      []string
	files       []string          // sorted, de-duplicated union of written paths
	contents    map[string][]byte // final composed bytes per file in files
}

// RunRepositoryRepair repairs every mechanically repairable finding `repository
// check` reports on an already-migrated repository, under the two-pass
// authorization model.
func RunRepositoryRepair(ctx context.Context, d SetupDeps, o RepairOptions) RepositoryRepairResult {
	facts, sc, err := GatherSetupFacts(ctx, d, true)
	if err != nil {
		return repairFromMigrateResult(migrateGatherFailure(err))
	}
	phase, refusal := migrateRoute(ctx, d.Git, facts, &sc)
	if refusal != nil {
		if refusal.RepositoryState == string(reposetup.StateFresh) {
			return repairRefusal(reposetup.StateFresh,
				"the repository has no docket metadata branch; run `docket repository init` to create it")
		}
		return repairFromMigrateResult(*refusal)
	}
	switch phase {
	case phaseAlreadyMigrated:
		// proceed
	case phaseLegacyFull:
		return repairRefusal(reposetup.StateLegacy,
			"the repository has a legacy single-branch planning surface; run `docket repository migrate` to convert it, then re-run `docket repository repair`")
	case phaseResumePrune:
		return repairRefusal(reposetup.StatePartial,
			"an interrupted migration still has a live planning surface on the integration branch; run `docket repository migrate` to finish it, then re-run `docket repository repair`")
	case phaseResumeLocal:
		return repairRefusal(reposetup.StateNeedsReview,
			"the local .docket metadata attachment is incomplete; run `docket repository migrate` to finish attaching it, then re-run `docket repository repair`")
	default:
		return repairInternalFailure(reposetup.StateUnknown, "routing the repository topology",
			fmt.Errorf("unexpected topology phase %d", phase))
	}

	metadataTip := sc.metadataTip
	if metadataTip == "" {
		return repairExternalFailure(reposetup.StateHealthy, "pinning the metadata revision",
			errors.New("no authoritative metadata tip is available"))
	}
	corpus, err := readCheckCorpus(ctx, d.Git, sc)
	if err != nil {
		return repairExternalFailure(reposetup.StateHealthy, "reading the metadata corpus", err)
	}
	plan, err := planRepositoryRepair(sc, corpus)
	if err != nil {
		return repairInternalFailure(reposetup.StateHealthy, "planning the repairs", err)
	}
	if len(plan.files) == 0 {
		return repairNoOp(metadataTip, plan.manual)
	}
	if !o.Authorized {
		return repairConfirmationRequired(sc, metadataTip, plan)
	}
	// Decide-and-act on the same copy: an authorized re-invocation acts only on
	// the exact metadata revision its preview showed.
	if migrateSourceMoved(o.ExpectedSource, metadataTip) {
		return repairContended(metadataTip, o.ExpectedSource)
	}
	return executeRepositoryRepair(ctx, d.Git, sc, metadataTip, plan)
}

// planRepositoryRepair is the pure repair decision. Frontmatter repairs are
// planned exactly as check reports them (corpusFindings) and applied in memory
// first (ApplyRepairs re-derives and re-proves each patch); the derived-view
// findings are then computed over the REPAIRED corpus, so a record needing both a
// frontmatter fix and an artifact-links re-render — or a frontmatter fix that
// changes what the board renders — comes out canonical in one pass. It never
// writes.
func planRepositoryRepair(sc setupContext, corpus checkCorpus) (repositoryRepairPlan, error) {
	cfg := sc.cfg
	p := repositoryRepairPlan{contents: map[string][]byte{}}

	byPath := map[string][]reposetup.RepairFinding{}
	for _, f := range corpusFindings(cfg, corpus.records) {
		if !f.Repairable {
			p.manual = append(p.manual, manualReviewLine("frontmatter-manual-review", f.Path, f.Message))
			continue
		}
		p.frontmatter = append(p.frontmatter, f)
		byPath[f.Path] = append(byPath[f.Path], f)
	}

	repaired := corpus
	repaired.records = make([]corpusRecord, len(corpus.records))
	for i, r := range corpus.records {
		if fs, ok := byPath[r.path]; ok {
			b, err := reposetup.ApplyRepairs(r.bytes, fs)
			if err != nil {
				return repositoryRepairPlan{}, fmt.Errorf("applying the frontmatter repairs to %s: %w", r.path, err)
			}
			r.bytes = b
			p.contents[r.path] = b
		}
		repaired.records[i] = r
	}

	repairable, diagnostics := splitDerivedFindings(derivedViewFindings(cfg, repaired))
	for _, d := range diagnostics {
		p.manual = append(p.manual, manualReviewLine(d.Code, d.Path, d.Message))
	}
	p.derived = repairable
	if len(repairable) > 0 {
		snap, ok := buildCorpusSnapshot(cfg, repaired.records)
		if !ok {
			return repositoryRepairPlan{}, errors.New("the repaired metadata corpus could not be built into a snapshot")
		}
		recByPath := map[string]corpusRecord{}
		for _, r := range repaired.records {
			recByPath[r.path] = r
		}
		for _, f := range derivedRepairFiles(repairable) {
			b, err := composeDerivedRepairBytes(sc, snap, repaired, recByPath, f)
			if err != nil {
				return repositoryRepairPlan{}, fmt.Errorf("composing the repaired %s: %w", f, err)
			}
			p.contents[f] = b
		}
	}

	for f := range p.contents {
		p.files = append(p.files, f)
	}
	sort.Strings(p.files)
	return p, nil
}

// manualReviewLine renders one manual-review entry.
func manualReviewLine(code, path, message string) string {
	return fmt.Sprintf("[%s] %s: %s", code, path, message)
}

// executeRepositoryRepair publishes the planned bytes as a single descendant of
// the pinned metadata tip under an exact owned lease and re-reads the
// postcondition byte-exactly. Only the planned files change; every other blob is
// carried forward from the pinned tree.
func executeRepositoryRepair(ctx context.Context, git *gitcli.Client, sc setupContext, metadataTip string, plan repositoryRepairPlan) RepositoryRepairResult {
	tipOID := gitcli.ObjectID(metadataTip)
	docketRef := gitcli.RefName(branchRefPrefix + reposetup.MetadataBranchName)

	ops := make([]gitcli.TreeOp, 0, len(plan.files))
	for _, f := range plan.files {
		ops = append(ops, gitcli.TreeOp{PutBlob: &gitcli.PutBlobOp{Path: gitcli.RepoPath(f), Content: plan.contents[f], Mode: blobMode}})
	}
	tree, err := git.BuildTree(ctx, sc.repo, tipOID, ops)
	if err != nil {
		return repairExternalFailure(reposetup.StateHealthy, "composing the repaired metadata tree", err)
	}
	commit, err := git.CommitTree(ctx, sc.repo, tree, []gitcli.ObjectID{tipOID}, repositoryRepairSubject, nil)
	if err != nil {
		return repairExternalFailure(reposetup.StateHealthy, "creating the repair commit", err)
	}
	out, err := git.PushLease(ctx, sc.repo, setupRemote(), docketRef, commit, tipOID)
	if err != nil {
		return repairExternalFailure(reposetup.StateHealthy, "publishing the repair", err)
	}
	switch out.Disposition {
	case gitcli.PushApplied:
		// proceed
	case gitcli.PushLeaseLost:
		return repairContended(string(out.Remote), metadataTip)
	default:
		return repairExternalFailure(reposetup.StateHealthy, "publishing the repair", errors.New("docket lease push failed"))
	}
	rev, err := git.FetchBranch(ctx, sc.repo, setupRemote(), docketRef)
	if err != nil {
		return repairExternalFailure(reposetup.StateHealthy, "re-reading the repaired metadata branch", err)
	}
	if rev.Commit != commit {
		return repairContended(string(rev.Commit), metadataTip)
	}
	return repairApplied(string(commit), metadataTip, plan)
}

// --- result constructors -----------------------------------------------------

// newRepairResult stamps the envelope for a repair outcome.
func newRepairResult(result Result, out RepositoryRepairResult) RepositoryRepairResult {
	out.Envelope = NewEnvelope(OperationRepositoryRepair, result)
	return out
}

// repairPreviewText renders the confirmation preview: repo, remote, exact pinned
// metadata revision, each repair as `[code] path` (frontmatter entries with their
// patch preview), and the manual-review list.
func repairPreviewText(sc setupContext, metadataTip string, plan repositoryRepairPlan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "docket repository repair — preview\n")
	fmt.Fprintf(&b, "  repository:  %s\n", sc.repo.PrimaryWorktree)
	fmt.Fprintf(&b, "  remote:      %s\n", setupRemote())
	fmt.Fprintf(&b, "  metadata:    %s @ %s\n", reposetup.MetadataBranchName, metadataTip)
	fmt.Fprintf(&b, "  repairs:\n")
	for _, f := range plan.frontmatter {
		fmt.Fprintf(&b, "    [%s] %s\n", f.Code, f.Path)
		fmt.Fprintf(&b, "%s\n", indentPatch(f.Patch))
	}
	for _, f := range plan.derived {
		fmt.Fprintf(&b, "    [%s] %s\n", f.Code, f.Path)
	}
	writeManualReview(&b, plan.manual)
	return b.String()
}

// writeManualReview appends the manual-review block when there is one.
func writeManualReview(b *strings.Builder, manual []string) {
	if len(manual) == 0 {
		return
	}
	fmt.Fprintf(b, "  manual review (not repaired):\n")
	for _, m := range manual {
		fmt.Fprintf(b, "    %s\n", m)
	}
}

// repairConfirmationRequired is the unauthorized preview: invalid-state with
// repository_state confirmation-required, naming --yes. SourceRevision is the
// pinned metadata tip so the confirm flow re-proves the exact copy shown.
func repairConfirmationRequired(sc setupContext, metadataTip string, plan repositoryRepairPlan) RepositoryRepairResult {
	out := newRepairResult(ResultInvalidState, RepositoryRepairResult{
		RepositoryState: repairConfirmationRequiredState,
		SourceRevision:  metadataTip,
		Repairs:         plan.frontmatter,
		RepairedViews:   derivedRepairFiles(plan.derived),
		RepairedFiles:   plan.files,
		ManualReview:    plan.manual,
	})
	out.human = repairPreviewText(sc, metadataTip, plan) +
		"\nconfirmation required: re-run with --yes to authorize these repairs"
	return out
}

// repairApplied is the success document: new and prior tips, the repaired file
// set, and the local sync remedy (the remote advanced; .docket fast-forwards on
// the next `docket repository prepare`).
func repairApplied(newTip, priorTip string, plan repositoryRepairPlan) RepositoryRepairResult {
	pending := []string{
		"fast-forward your local .docket metadata worktree: re-run `docket repository prepare` to sync it to the repaired metadata revision",
	}
	out := newRepairResult(ResultApplied, RepositoryRepairResult{
		RepositoryState: string(reposetup.StateNeedsReview),
		SourceRevision:  priorTip,
		MetadataTip:     newTip,
		Repairs:         plan.frontmatter,
		RepairedViews:   derivedRepairFiles(plan.derived),
		RepairedFiles:   plan.files,
		ManualReview:    plan.manual,
		PendingLocal:    pending,
	})
	var b strings.Builder
	fmt.Fprintf(&b, "repository repaired: metadata %s (%d file(s))\npending local sync: %s\n",
		newTip, len(plan.files), strings.Join(pending, "; "))
	writeManualReview(&b, plan.manual)
	out.human = strings.TrimRight(b.String(), "\n")
	return out
}

// repairNoOp is the idempotent nothing-to-repair document. Manual-review items,
// if any, are still listed — they are a human's to resolve.
func repairNoOp(metadataTip string, manual []string) RepositoryRepairResult {
	out := newRepairResult(ResultNoOp, RepositoryRepairResult{
		RepositoryState: string(reposetup.StateHealthy),
		SourceRevision:  metadataTip,
		ManualReview:    manual,
	})
	var b strings.Builder
	fmt.Fprintf(&b, "repository repair: nothing to repair at metadata %s\n", metadataTip)
	writeManualReview(&b, manual)
	out.human = strings.TrimRight(b.String(), "\n")
	return out
}

// repairContended reports that the docket metadata tip differs from the copy the
// repair decided on. Nothing was overwritten.
func repairContended(freshTip, expected string) RepositoryRepairResult {
	out := newRepairResult(ResultContended, RepositoryRepairResult{
		RepositoryState: string(reposetup.StateHealthy),
		SourceRevision:  expected,
		MetadataTip:     freshTip,
	})
	out.human = fmt.Sprintf("repair contended: the docket metadata branch moved to %s since the repair pinned %s; re-run to preview the new state",
		freshTip, expected)
	return out
}

// repairRefusal builds an invalid-state refusal naming a remedy valid in exactly
// the classified state.
func repairRefusal(state reposetup.State, remedy string) RepositoryRepairResult {
	out := newRepairResult(ResultInvalidState, RepositoryRepairResult{RepositoryState: string(state)})
	out.human = fmt.Sprintf("%s: %s (%s): %s", OperationRepositoryRepair, ResultInvalidState, state, remedy)
	return out
}

// repairExternalFailure builds an external-failed result naming the stage.
func repairExternalFailure(state reposetup.State, stage string, err error) RepositoryRepairResult {
	out := newRepairResult(ResultExternalFailed, RepositoryRepairResult{RepositoryState: string(state)})
	out.human = fmt.Sprintf("%s: %s while %s: %s", OperationRepositoryRepair, ResultExternalFailed, stage, err.Error())
	return out
}

// repairInternalFailure builds an internal-error result naming the stage.
func repairInternalFailure(state reposetup.State, stage string, err error) RepositoryRepairResult {
	out := newRepairResult(ResultInternalError, RepositoryRepairResult{RepositoryState: string(state)})
	out.human = fmt.Sprintf("%s: %s while %s: %s", OperationRepositoryRepair, ResultInternalError, stage, err.Error())
	return out
}

// repairFromMigrateResult re-stamps a shared topology/gather refusal (built by
// the migrate route or gather-failure mapping) as a repository.repair result,
// keeping its result, state, findings, and remedy text.
func repairFromMigrateResult(m RepositoryMigrateResult) RepositoryRepairResult {
	out := newRepairResult(m.Result, RepositoryRepairResult{
		RepositoryState: m.RepositoryState,
		Findings:        m.Findings,
	})
	out.human = strings.Replace(m.HumanText(), OperationRepositoryMigrate, OperationRepositoryRepair, 1)
	return out
}
```

- [ ] **Step 4: Run the tests and confirm they pass**

Run: `go test -count=1 -run 'TestPlanRepositoryRepair|TestRepair' ./internal/app/` and
`go vet ./internal/app/`.
Expected: PASS. If a plan test's fixture record raises an unexpected snapshot error finding, fix
the fixture (add the missing field) and say so in the commit message.

- [ ] **Step 5: Mutation-test the composition order**

Back up `internal/app/repository_repair.go`. In `planRepositoryRepair`, change
`derivedViewFindings(cfg, repaired)` to `derivedViewFindings(cfg, corpus)` and change
`composeDerivedRepairBytes(sc, snap, repaired, recByPath, f)` to read `recByPath` built from
`corpus.records` (the unrepaired bytes). Confirm the edit landed with `grep -c`. Run
`go test -count=1 -run TestPlanRepositoryRepairComposesFrontmatterAndArtifactLinks ./internal/app/`.
It must FAIL (the composed bytes lose `title: 'yes'`). Restore with `mv -f` and re-run green.

- [ ] **Step 6: Commit**

```bash
git add internal/app/repository_repair.go internal/app/repository_repair_test.go
git commit -m "feat(0496): repository.repair service composes frontmatter and derived-view repairs"
```

---

### Task 3: Register `docket repository repair` (CLI, catalog, schema, install)

**Risk:** standard. This is registration across established seams with an interactive flow that
mirrors `migrate`.

**Files:**
- Modify: `internal/cli/repository.go` (runner var, group registration, new
  `newRepositoryRepairCommand`, group `Short`)
- Modify: `internal/cli/install.go` (known-command set)
- Modify: `internal/app/schema_registry.go` (binding)
- Test: `internal/cli/repository_test.go`

**Interfaces:**
- Consumes: `app.RunRepositoryRepair(ctx, d app.SetupDeps, o app.RepairOptions) app.RepositoryRepairResult`,
  `app.RepairOptions{Authorized, ExpectedSource}`, and the `ConfirmationRequired() bool` and
  `SourceRev() string` methods (Task 2). Also `capability(...)`, `EffectMetadataWrite`,
  `resolveRepoDir`, `gitcli.NewClient`, `repositoryConfirmInteractive`, `repositoryReadYes`
  (existing).
- Produces: `var repositoryRepairRunner func(context.Context, app.SetupDeps, app.RepairOptions) app.OperationResult`;
  catalog entry `{"id":"repository.repair","argv":["docket","repository","repair"],"signature":"[--repo-dir <dir>] [--yes]","effects":["metadata-write"]}`.

- [ ] **Step 1: Write the failing CLI tests**

Append to `internal/cli/repository_test.go`:

```go
// fakeRepairResult is a stub repair result whose confirmation state is set by the
// test, so the CLI's prompt decision is exercised without a repository.
type fakeRepairResult struct {
	app.Envelope
	source  string
	confirm bool
}

func (r fakeRepairResult) HumanText() string          { return "repair plan @ " + r.source }
func (r fakeRepairResult) SourceRev() string          { return r.source }
func (r fakeRepairResult) ConfirmationRequired() bool { return r.confirm }

// TestRepositoryRepairRegisteredWithCapability proves `repository repair` is a
// registered leaf carrying exactly --repo-dir and --yes, the catalog id
// repository.repair, and exactly the metadata-write effect.
func TestRepositoryRepairRegisteredWithCapability(t *testing.T) {
	root := captureTree(t)
	cmd, _, err := root.Find([]string{"repository", "repair"})
	if err != nil || cmd == nil || cmd.Name() != "repair" {
		t.Fatalf("repository repair not registered: cmd=%v err=%v", cmd, err)
	}
	for _, flag := range []string{"repo-dir", "yes"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("repository repair: missing --%s flag", flag)
		}
	}
	if cmd.Flags().Lookup("repair-frontmatter") != nil {
		t.Errorf("repository repair must not carry --repair-frontmatter")
	}
	if got := cmd.Annotations[capAnnotationID]; got != "repository.repair" {
		t.Errorf("capability id = %q, want repository.repair", got)
	}
	if got := cmd.Annotations[capAnnotationEffects]; got != string(EffectMetadataWrite) {
		t.Errorf("effects = %q, want exactly %q", got, EffectMetadataWrite)
	}
}

// TestRepositoryRepairYesAuthorizesDirectly proves --yes calls the service once,
// authorized, with no preview pass.
func TestRepositoryRepairYesAuthorizesDirectly(t *testing.T) {
	var calls []app.RepairOptions
	old := repositoryRepairRunner
	repositoryRepairRunner = func(ctx context.Context, d app.SetupDeps, o app.RepairOptions) app.OperationResult {
		calls = append(calls, o)
		return fakeRepairResult{Envelope: app.NewEnvelope("repository.repair", app.ResultApplied), source: "abc123"}
	}
	defer func() { repositoryRepairRunner = old }()

	_, _, code := runCLI(t, "repository", "repair", "--yes")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if len(calls) != 1 || !calls[0].Authorized {
		t.Fatalf("--yes must call the service exactly once, authorized: %+v", calls)
	}
}

// TestRepositoryRepairNonInteractivePreview proves without --yes and without a
// terminal the service is called once, unauthorized, and never re-invoked.
func TestRepositoryRepairNonInteractivePreview(t *testing.T) {
	var calls []app.RepairOptions
	old := repositoryRepairRunner
	repositoryRepairRunner = func(ctx context.Context, d app.SetupDeps, o app.RepairOptions) app.OperationResult {
		calls = append(calls, o)
		return fakeRepairResult{Envelope: app.NewEnvelope("repository.repair", app.ResultInvalidState), source: "abc123", confirm: true}
	}
	oldI := repositoryConfirmInteractive
	repositoryConfirmInteractive = func() bool { return false }
	defer func() { repositoryRepairRunner = old; repositoryConfirmInteractive = oldI }()

	_, _, _ = runCLI(t, "repository", "repair")
	if len(calls) != 1 || calls[0].Authorized {
		t.Fatalf("want exactly one unauthorized preview call: %+v", calls)
	}
}

// TestRepositoryRepairInteractiveConfirmReinvokes proves `y` re-invokes
// authorized, pinned to the previewed revision.
func TestRepositoryRepairInteractiveConfirmReinvokes(t *testing.T) {
	var calls []app.RepairOptions
	old := repositoryRepairRunner
	repositoryRepairRunner = func(ctx context.Context, d app.SetupDeps, o app.RepairOptions) app.OperationResult {
		calls = append(calls, o)
		return fakeRepairResult{Envelope: app.NewEnvelope("repository.repair", app.ResultInvalidState), source: "pinnedtip", confirm: !o.Authorized}
	}
	oldI := repositoryConfirmInteractive
	repositoryConfirmInteractive = func() bool { return true }
	defer func() { repositoryRepairRunner = old; repositoryConfirmInteractive = oldI }()

	_, _, _ = runCLIStdin(t, "y\n", "repository", "repair")
	if len(calls) != 2 {
		t.Fatalf("runner called %d times, want preview then authorized", len(calls))
	}
	if calls[0].Authorized || !calls[1].Authorized || calls[1].ExpectedSource != "pinnedtip" {
		t.Errorf("calls = %+v, want unauthorized preview then authorized pinned to pinnedtip", calls)
	}
}

// TestRepositoryRepairInteractiveDeclineDoesNotAuthorize proves `n` never
// re-invokes authorized.
func TestRepositoryRepairInteractiveDeclineDoesNotAuthorize(t *testing.T) {
	var calls []app.RepairOptions
	old := repositoryRepairRunner
	repositoryRepairRunner = func(ctx context.Context, d app.SetupDeps, o app.RepairOptions) app.OperationResult {
		calls = append(calls, o)
		return fakeRepairResult{Envelope: app.NewEnvelope("repository.repair", app.ResultInvalidState), source: "pinnedtip", confirm: true}
	}
	oldI := repositoryConfirmInteractive
	repositoryConfirmInteractive = func() bool { return true }
	defer func() { repositoryRepairRunner = old; repositoryConfirmInteractive = oldI }()

	_, _, _ = runCLIStdin(t, "n\n", "repository", "repair")
	if len(calls) != 1 || calls[0].Authorized {
		t.Fatalf("a declined preview must never authorize: %+v", calls)
	}
}

// TestRepositoryRepairInteractiveNoPromptWithoutConfirmation proves an
// interactive run whose preview is NOT a confirmation request (a no-op or a
// refusal) is presented directly, with no prompt and no re-invoke (Review Focus 4).
func TestRepositoryRepairInteractiveNoPromptWithoutConfirmation(t *testing.T) {
	var calls []app.RepairOptions
	old := repositoryRepairRunner
	repositoryRepairRunner = func(ctx context.Context, d app.SetupDeps, o app.RepairOptions) app.OperationResult {
		calls = append(calls, o)
		return fakeRepairResult{Envelope: app.NewEnvelope("repository.repair", app.ResultNoOp), source: "tip", confirm: false}
	}
	oldI := repositoryConfirmInteractive
	repositoryConfirmInteractive = func() bool { return true }
	defer func() { repositoryRepairRunner = old; repositoryConfirmInteractive = oldI }()

	out, _, _ := runCLIStdin(t, "y\n", "repository", "repair")
	if len(calls) != 1 {
		t.Fatalf("runner called %d times, want one (no prompt, no re-invoke)", len(calls))
	}
	if strings.Contains(out, "repair? [y/N]") {
		t.Errorf("a no-op preview must not prompt; stdout:\n%s", out)
	}
}
```

Add `"repair"` to the subcommand list in `TestRepositoryCommandsRegistered`
(`[]string{"init", "check", "migrate", "prepare", "configure-tests", "repair"}`). Add `"strings"` to
the test file's imports if it is not already there.

- [ ] **Step 2: Run the tests and confirm they fail**

Run: `go test -count=1 -run 'TestRepositoryRepair|TestRepositoryCommandsRegistered' ./internal/cli/`
Expected: FAIL (build: `repositoryRepairRunner` undefined).

- [ ] **Step 3: Implement the command**

In `internal/cli/repository.go`:

1. Add to the runner `var (...)` block:

```go
	repositoryRepairRunner = func(ctx context.Context, d app.SetupDeps, o app.RepairOptions) app.OperationResult {
		return app.RunRepositoryRepair(ctx, d, o)
	}
```

2. Change the group `Short` to `"Initialize, migrate, check, and repair the docket repository topology"`.
3. In `newRepositoryCommand`, build `repairCmd := newRepositoryRepairCommand(setResult)` and add it:
   `repositoryCmd.AddCommand(initCmd, checkCmd, migrateCmd, repairCmd, prepareCmd, configureTestsCmd, syncIntegrationCmd)`.
4. Extend the file header comment's one-exception sentence so it reads "…is `migrate`'s and
   `repair`'s two-pass confirm flow…".
5. Add after `newRepositoryMigrateCommand`:

```go
// newRepositoryRepairCommand builds `docket repository repair` with its --yes
// flag and the two-pass confirm flow. --yes performs the authorized repair
// directly; without it the service returns a preview, presented as-is
// non-interactively, or — on a terminal, and only when the preview is a
// confirmation request — printed and confirmed, then re-invoked authorized and
// pinned to exactly the metadata revision the preview showed. A no-op or a
// refusal is presented without a prompt.
func newRepositoryRepairCommand(setResult func(app.OperationResult)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repair",
		Short: "Preview and apply the mechanical repairs `repository check` reports on a migrated repository",
		Args:  cobra.NoArgs,
		// metadata-write: one repair descendant published to the metadata branch
		// under an exact lease. It never touches the local .docket worktree (the
		// result names `docket repository prepare` to sync it).
		Annotations: capability("repository.repair", EffectMetadataWrite),
		RunE: func(c *cobra.Command, _ []string) error {
			repoDir, err := resolveRepoDir(c)
			if err != nil {
				return err
			}
			client, err := gitcli.NewClient()
			if err != nil {
				return err
			}
			deps := app.SetupDeps{Git: client, RepoDir: repoDir}
			yes, _ := c.Flags().GetBool("yes")
			if yes {
				setResult(repositoryRepairRunner(c.Context(), deps, app.RepairOptions{Authorized: true}))
				return nil
			}

			preview := repositoryRepairRunner(c.Context(), deps, app.RepairOptions{})
			jsonMode, _ := c.Flags().GetBool("json")
			confirmable, ok := preview.(interface{ ConfirmationRequired() bool })
			if jsonMode || !repositoryConfirmInteractive() || !ok || !confirmable.ConfirmationRequired() {
				setResult(preview)
				return nil
			}
			fmt.Fprintln(c.OutOrStdout(), preview.HumanText())
			fmt.Fprint(c.OutOrStdout(), "repair? [y/N] ")
			if !repositoryReadYes(c.InOrStdin()) {
				setResult(preview)
				return nil
			}
			expected := ""
			if p, ok := preview.(interface{ SourceRev() string }); ok {
				expected = p.SourceRev()
			}
			setResult(repositoryRepairRunner(c.Context(), deps, app.RepairOptions{Authorized: true, ExpectedSource: expected}))
			return nil
		},
	}
	cmd.Flags().String("repo-dir", "", "repository `dir` to operate on (default: current directory)")
	cmd.Flags().Bool("yes", false, "authorize the previewed repairs without an interactive confirmation")
	return cmd
}
```

In `internal/cli/install.go`, add `"repository repair":           true,` directly after
`"repository migrate"` (gofmt aligns the column).

In `internal/app/schema_registry.go`, add between `repository.prepare` and
`repository.sync-integration` (gofmt aligns the column):

```go
	{ID: "repository.repair", Request: nil, Result: RepositoryRepairResult{}},                                      // RunRepositoryRepair
```

- [ ] **Step 4: Run the tests and confirm they pass**

Run: `go test -count=1 ./internal/cli/ ./internal/app/ -run 'TestRepository|TestProductionCapability|TestProductionEffects|TestCapabilitiesPayload|TestRepresentativeSignatures|Schema|Registry'`
then `go run ./cmd/docket capabilities --json | grep -F '"repository.repair"'`.
Expected: PASS. The catalog line reads
`{"id":"repository.repair","argv":["docket","repository","repair"],"signature":"[--repo-dir <dir>] [--yes]","effects":["metadata-write"]}`.
If a schema-registry test reports a missing vocabulary or description for the new result type,
satisfy it the way `RepositoryMigrateResult` does. Do not exempt the type.

- [ ] **Step 5: Mutation-test the no-prompt guard**

Back up `internal/cli/repository.go`, delete `|| !ok || !confirmable.ConfirmationRequired()` from
the condition (and the then-unused `confirmable, ok :=` line), and confirm it landed. Run
`go test -count=1 -run TestRepositoryRepairInteractiveNoPromptWithoutConfirmation ./internal/cli/`.
It must FAIL. Restore with `mv -f` and re-run green.

- [ ] **Step 6: Commit**

```bash
git add internal/cli/repository.go internal/cli/install.go internal/app/schema_registry.go internal/cli/repository_test.go
git commit -m "feat(0496): register docket repository repair (catalog repository.repair)"
```

---

### Task 4: Real-git `repository repair` integration shard

**Risk:** standard. These are real-git integration tests over established fixtures, plus a new
shard runner and its budget row.

**Files:**
- Create: `internal/app/repository_repair_integration_test.go`
- Create: `tests/test_go_integration_app_reporepair.sh`
- Modify: `tests/runtime-budgets.tsv` (one row)
- Modify: `internal/app/repomigration_integration_test.go`. **Move out** (cut, do not copy) the
  helpers `repairAuthoredSentinel`, `staleRepairRecord`, `(*initRepo).publishHealthyDrift`,
  `currentDocketTip`, `showDocketFile`, and `containsPath` into the new file. The two
  `TestIntegrationRepoMigrationHealthyRepair*` tests stay for now (Task 5 replaces them). Same
  package, so they still compile.

**Interfaces:**
- Consumes: `RunRepositoryRepair`, `RepairOptions`, `RepositoryRepairResult` (+ `ConfirmationRequired`),
  `repositoryRepairSubject` (Task 2); fixtures `newHealthyRepo`, `newInitRepo`, `legacyDocketYML`,
  `cleanLegacyFiles`, `healthySetupYML`, `runGit`, `tryGit`, `writeRepoFile`, `newGitClient`,
  `changedPathSet`, `sameStringSet`, `keysOf`, `(*initRepo).writeDocketFileAndPush`,
  `(*initRepo).runCheck` (existing integration helpers).
- Produces: `(*initRepo).runRepair(t, o RepairOptions) RepositoryRepairResult`,
  `(*initRepo).publishFrontmatterDrift(t)`, and the moved helpers above (Task 5's migrate test
  uses `publishHealthyDrift`, `currentDocketTip`).

- [ ] **Step 1: Add the shard runner and budget row**

Create `tests/test_go_integration_app_reporepair.sh` (and `chmod +x` it like its siblings):

```bash
#!/usr/bin/env bash
# docket-suite: go
# tests/test_go_integration_app_reporepair.sh — Go integration shard (change 0496):
# the `repository repair` service on an already-migrated repository — preview
# writes nothing, one lease-guarded descendant commit carrying exactly the listed
# files, frontmatter + derived-view composition, moved-tip contention, legacy and
# fresh refusals, malformed-marker manual review, and the empty-claimed_at no-op —
# behind the `integration` build tag, prefix ^TestIntegrationRepoRepair.
# Declarations only — execution and inspection live in
# tests/lib/go-integration-shard.sh; the completeness contract is
# tests/test_go_integration_contract.sh.
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$REPO" || exit 1
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SHARD_PKG="./internal/app"
SHARD_PREFIX="TestIntegrationRepoRepair"
SHARD_MODE="normal"

. "$REPO/tests/lib/go-integration-shard.sh"
shard_inspect_maybe
run_integration_shard
exit "$fail"
```

In `tests/runtime-budgets.tsv`, add directly after the
`tests/test_go_integration_app_reporecovery.sh` row (a literal TAB between columns):

```
tests/test_go_integration_app_reporepair.sh	30	parallel
```

(30s: seven scenarios, each building a healthy real-git fixture, which is a bit more than the 20s
repomigration shard carries per scenario. Docket-build re-measures it at the gate.)

- [ ] **Step 2: Write the integration tests**

Create `internal/app/repository_repair_integration_test.go`. Line 1 is `//go:build integration`
and line 2 is blank (contract check 1). Move the six helpers named in **Files** into it verbatim,
then add:

```go
//go:build integration

package app

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/reposetup"
)

// This is the real-Git `repository repair` shard (prefix TestIntegrationRepoRepair,
// tests/test_go_integration_app_reporepair.sh). Each test builds an
// already-migrated repository (newHealthyRepo), publishes drift onto its docket
// metadata branch from the .docket worktree, drives RunRepositoryRepair, and
// inspects the remote docket branch with an independent git oracle.

// ... moved helpers: repairAuthoredSentinel, staleRepairRecord, publishHealthyDrift,
// currentDocketTip, showDocketFile, containsPath ...

// runRepair drives RunRepositoryRepair against the invocation clone.
func (r *initRepo) runRepair(t *testing.T, o RepairOptions) RepositoryRepairResult {
	t.Helper()
	client := newGitClient(t)
	return RunRepositoryRepair(context.Background(), SetupDeps{Git: client, RepoDir: r.invocation}, o)
}

const (
	repairStampPath = "docs/changes/archive/2026-01-02-0003-archived-change.md"
	repairEmptyPath = "docs/changes/archive/2026-01-03-0004-archived-empty.md"
)

// repairArchivedRecord renders a minimal valid archived done record carrying the
// given claimed_at line.
func repairArchivedRecord(id int, slug, claimedLine string) string {
	return "---\nid: " + strconv.Itoa(id) + "\nslug: " + slug + "\nstatus: done\ntitle: Change " + slug +
		"\ntype: feature\n" + claimedLine + "\n---\n\nBody for " + slug + ".\n"
}

// publishFrontmatterDrift publishes one archived done record with a REAL claim
// stamp (a repairable drop-final-claimed-at) and one with the EMPTY cleared form
// (not a finding; it must survive every repair byte-identical).
func (r *initRepo) publishFrontmatterDrift(t *testing.T) {
	t.Helper()
	dotDocket := filepath.Join(r.invocation, ".docket")
	writeRepoFile(t, dotDocket, repairStampPath, repairArchivedRecord(3, "archived-change", "claimed_at: 2026-08-01T10:00:00Z"))
	writeRepoFile(t, dotDocket, repairEmptyPath, repairArchivedRecord(4, "archived-empty", "claimed_at:"))
	runGit(t, dotDocket, "add", "--", repairStampPath, repairEmptyPath)
	runGit(t, dotDocket, "commit", "-q", "-m", "publish archived claim stamps")
	runGit(t, dotDocket, "push", "-q", "origin", string(reposetup.MetadataBranchName))
}

// TestIntegrationRepoRepairPreviewListsBothKindsAndWritesNothing proves the
// unauthorized run returns confirmation-required pinned to the docket tip,
// listing the frontmatter repair AND the derived-view repairs, never the empty
// claimed_at record — and writes nothing.
func TestIntegrationRepoRepairPreviewListsBothKindsAndWritesNothing(t *testing.T) {
	r := newHealthyRepo(t)
	r.publishHealthyDrift(t)
	r.publishFrontmatterDrift(t)
	before := currentDocketTip(t, r)

	res := r.runRepair(t, RepairOptions{})
	if res.Result != ResultInvalidState || !res.ConfirmationRequired() {
		t.Fatalf("preview = %q/%q (%s), want invalid-state/confirmation-required", res.Result, res.RepositoryState, res.HumanText())
	}
	if res.SourceRevision != before {
		t.Errorf("SourceRevision = %q, want the pinned docket tip %q", res.SourceRevision, before)
	}
	for _, p := range []string{"docs/changes/BOARD.md", "docs/changes/active/0001-example.md", repairStampPath} {
		if !containsPath(res.RepairedFiles, p) {
			t.Errorf("RepairedFiles = %v, want %s", res.RepairedFiles, p)
		}
	}
	if containsPath(res.RepairedFiles, repairEmptyPath) {
		t.Errorf("the empty claimed_at record must not be a repair: %v", res.RepairedFiles)
	}
	if len(res.Repairs) != 1 || res.Repairs[0].Code != reposetup.RepairDropClaimedAt || res.Repairs[0].Path != repairStampPath {
		t.Errorf("Repairs = %+v, want exactly the stamped record's drop", res.Repairs)
	}
	for _, want := range []string{"--yes", "[drop-final-claimed-at] " + repairStampPath, "[board-stale] docs/changes/BOARD.md"} {
		if !strings.Contains(res.HumanText(), want) {
			t.Errorf("preview human lacks %q:\n%s", want, res.HumanText())
		}
	}
	if after := currentDocketTip(t, r); after != before {
		t.Errorf("preview advanced the docket branch %q -> %q", before, after)
	}
}

// TestIntegrationRepoRepairAppliesOneDescendantCommit proves an authorized repair
// publishes exactly ONE descendant of the pinned tip, subject `docket: repository
// repair`, changing exactly the listed files; the record carries both repairs'
// canonical bytes and its untouched prose; the empty claimed_at record is
// byte-identical; a second run is a no-op; and a following check reports no
// repairable finding (Review Focus 2).
func TestIntegrationRepoRepairAppliesOneDescendantCommit(t *testing.T) {
	r := newHealthyRepo(t)
	r.publishHealthyDrift(t)
	r.publishFrontmatterDrift(t)
	before := currentDocketTip(t, r)
	emptyBefore := showDocketFile(t, r, repairEmptyPath)

	res := r.runRepair(t, RepairOptions{Authorized: true})
	if res.Result != ResultApplied {
		t.Fatalf("repair = %q (%s), want applied", res.Result, res.HumanText())
	}
	after := currentDocketTip(t, r)
	if res.MetadataTip != after || res.SourceRevision != before {
		t.Errorf("result tips = %q <- %q, want %q <- %q", res.MetadataTip, res.SourceRevision, after, before)
	}
	if parent := runGit(t, r.invocation, "rev-parse", after+"^"); parent != before {
		t.Errorf("repair commit parent = %s, want the pinned tip %s (one descendant)", parent, before)
	}
	if subject := runGit(t, r.invocation, "log", "-1", "--format=%s", after); subject != repositoryRepairSubject {
		t.Errorf("subject = %q, want %q", subject, repositoryRepairSubject)
	}
	want := map[string]bool{}
	for _, f := range res.RepairedFiles {
		want[f] = true
	}
	if got := changedPathSet(t, r, before, after); !sameStringSet(got, want) {
		t.Errorf("changed paths = %v, want exactly RepairedFiles %v", keysOf(got), keysOf(want))
	}
	record := showDocketFile(t, r, "docs/changes/active/0001-example.md")
	if !strings.Contains(record, "| Spec |") || !strings.Contains(record, repairAuthoredSentinel) {
		t.Errorf("repaired record must carry the Spec row and the untouched prose:\n%s", record)
	}
	if stamped := showDocketFile(t, r, repairStampPath); strings.Contains(stamped, "claimed_at") {
		t.Errorf("stamped archived record still carries claimed_at:\n%s", stamped)
	}
	if got := showDocketFile(t, r, repairEmptyPath); got != emptyBefore {
		t.Errorf("empty claimed_at record changed:\n%s", got)
	}
	if strings.Contains(showDocketFile(t, r, "docs/changes/BOARD.md"), "hand-written stale board") {
		t.Errorf("board was not recomputed")
	}

	second := r.runRepair(t, RepairOptions{Authorized: true})
	if second.Result != ResultNoOp {
		t.Errorf("second repair = %q (%s), want no-op", second.Result, second.HumanText())
	}
	if got := currentDocketTip(t, r); got != after {
		t.Errorf("idempotent re-run moved the docket branch %s -> %s", after, got)
	}
	for _, f := range r.runCheck(t).Findings {
		if f.Repairable != nil && *f.Repairable {
			t.Errorf("check after repair still reports a repairable finding: %+v", f)
		}
	}
}

// TestIntegrationRepoRepairMovedTipIsContended proves an authorized run pinned to
// a preview's tip, after a concurrent write moved the docket branch, is contended
// and overwrites nothing.
func TestIntegrationRepoRepairMovedTipIsContended(t *testing.T) {
	r := newHealthyRepo(t)
	r.publishHealthyDrift(t)
	preview := r.runRepair(t, RepairOptions{})
	if !preview.ConfirmationRequired() {
		t.Fatalf("preview = %q (%s), want confirmation-required", preview.Result, preview.HumanText())
	}
	r.writeDocketFileAndPush(t, "docs/changes/active/0002-concurrent.md",
		"---\nid: 2\nslug: concurrent\nstatus: proposed\ntitle: Concurrent\ntype: feature\n---\n\nBody.\n",
		"a concurrent metadata write")
	moved := currentDocketTip(t, r)

	res := r.runRepair(t, RepairOptions{Authorized: true, ExpectedSource: preview.SourceRevision})
	if res.Result != ResultContended {
		t.Fatalf("repair = %q (%s), want contended", res.Result, res.HumanText())
	}
	if got := currentDocketTip(t, r); got != moved {
		t.Errorf("a contended repair wrote: docket %s -> %s", moved, got)
	}
	if h := res.HumanText(); !strings.Contains(h, moved) || !strings.Contains(h, preview.SourceRevision) {
		t.Errorf("contended human must name both revisions: %q", h)
	}
}

// TestIntegrationRepoRepairRefusesLegacyRepository proves a legacy repository is
// refused naming `docket repository migrate` and no docket branch is created.
func TestIntegrationRepoRepairRefusesLegacyRepository(t *testing.T) {
	r := newInitRepo(t, legacyDocketYML, cleanLegacyFiles())
	res := r.runRepair(t, RepairOptions{Authorized: true})
	if res.Result != ResultInvalidState || res.RepositoryState != string(reposetup.StateLegacy) {
		t.Fatalf("legacy repair = %q/%q (%s), want invalid-state/legacy", res.Result, res.RepositoryState, res.HumanText())
	}
	if !strings.Contains(res.HumanText(), "docket repository migrate") {
		t.Errorf("legacy refusal must name `docket repository migrate`: %q", res.HumanText())
	}
	if _, err := tryGit(r.origin, "rev-parse", "--verify", "refs/heads/"+string(reposetup.MetadataBranchName)); err == nil {
		t.Errorf("a legacy refusal must not create the docket branch")
	}
}

// TestIntegrationRepoRepairRefusesFreshRepository proves a fresh repository is
// refused naming `docket repository init` (a remedy valid in that state).
func TestIntegrationRepoRepairRefusesFreshRepository(t *testing.T) {
	r := newInitRepo(t, healthySetupYML, nil)
	res := r.runRepair(t, RepairOptions{Authorized: true})
	if res.Result != ResultInvalidState || res.RepositoryState != string(reposetup.StateFresh) {
		t.Fatalf("fresh repair = %q/%q (%s), want invalid-state/fresh", res.Result, res.RepositoryState, res.HumanText())
	}
	if !strings.Contains(res.HumanText(), "docket repository init") {
		t.Errorf("fresh refusal must name `docket repository init`: %q", res.HumanText())
	}
}

// TestIntegrationRepoRepairEmptyClaimedAtIsCleanNoOp proves the 269-record shape:
// an archived done record with the empty cleared claimed_at is no finding — the
// repair is a no-op that writes nothing, and check exits 0 on an otherwise healthy
// repository.
func TestIntegrationRepoRepairEmptyClaimedAtIsCleanNoOp(t *testing.T) {
	r := newHealthyRepo(t)
	r.writeDocketFileAndPush(t, repairEmptyPath, repairArchivedRecord(4, "archived-empty", "claimed_at:"), "publish an archived record with the cleared claimed_at")
	before := currentDocketTip(t, r)

	res := r.runRepair(t, RepairOptions{Authorized: true})
	if res.Result != ResultNoOp {
		t.Fatalf("repair = %q (%s), want no-op", res.Result, res.HumanText())
	}
	if got := currentDocketTip(t, r); got != before {
		t.Errorf("a no-op repair wrote: docket %s -> %s", before, got)
	}
	check := r.runCheck(t)
	if code := check.CheckExitCode(); code != 0 {
		t.Errorf("check exit = %d (%s), want 0: an empty claimed_at is not a finding", code, check.HumanText())
	}
}

// TestIntegrationRepoRepairMalformedMarkerIsManualReview proves a record under an
// unbalanced managed marker is listed as manual review and left byte-identical,
// while the other repairs in the same run still apply (Review Focus 3).
func TestIntegrationRepoRepairMalformedMarkerIsManualReview(t *testing.T) {
	r := newHealthyRepo(t)
	r.publishHealthyDrift(t)
	malformedPath := "docs/changes/active/0005-malformed.md"
	malformed := "---\nid: 5\nslug: malformed\ntitle: Malformed\nstatus: proposed\npriority: medium\ntype: feature\n" +
		"created: 2026-08-30\nupdated: 2026-08-30\nspec: docs/superpowers/specs/2026-08-30-example-design.md\n---\n\n" +
		"## Artifacts\n\n<!-- docket:artifacts:start (generated — do not hand-edit) -->\n| Artifact | Link |\n\n## Why\n\nbody\n"
	r.writeDocketFileAndPush(t, malformedPath, malformed, "publish a record with an unbalanced managed block")

	res := r.runRepair(t, RepairOptions{Authorized: true})
	if res.Result != ResultApplied {
		t.Fatalf("repair = %q (%s), want applied (the board still repairs)", res.Result, res.HumanText())
	}
	if containsPath(res.RepairedFiles, malformedPath) {
		t.Errorf("the malformed record must never be written: %v", res.RepairedFiles)
	}
	var listed bool
	for _, m := range res.ManualReview {
		if strings.HasPrefix(m, "["+reposetup.CodeArtifactLinksMalformed+"] "+malformedPath) {
			listed = true
		}
	}
	if !listed {
		t.Errorf("ManualReview = %v, want the malformed record listed", res.ManualReview)
	}
	if got := showDocketFile(t, r, malformedPath); got != strings.TrimSpace(malformed) {
		t.Errorf("malformed record changed:\n%s", got)
	}
}
```

(`showDocketFile` returns `runGit` output, which is `TrimSpace`d. Compare it to the trimmed
fixture, as the last assertion does.)

- [ ] **Step 3: Run the shard and confirm it passes**

Run: `go test -tags integration -count=1 -run '^TestIntegrationRepoRepair' ./internal/app/`
then `go test -tags integration -count=1 -run '^TestIntegrationRepoMigration' ./internal/app/`
(the moved helpers still serve the migration shard) and `bash tests/test_go_integration_contract.sh`.
Expected: PASS for all three. The contract shows the new runner selecting 7 tests, and every
`TestIntegrationRepoRepair*` test matching exactly one runner.

- [ ] **Step 4: Mutation-test two keys**

1. Back up `internal/app/repository_repair.go`. Replace `if migrateSourceMoved(o.ExpectedSource, metadataTip) {`
   with `if false {` and confirm it landed. Run
   `go test -tags integration -count=1 -run '^TestIntegrationRepoRepairMovedTipIsContended$' ./internal/app/`.
   It must still be `contended`, because the lease catches the move. That is expected
   defense-in-depth, so record it, restore, and then do the stronger probe: also replace
   `case gitcli.PushLeaseLost:` with `case "never-a-disposition":` (and keep the `if false`). The
   test must now FAIL (the lease-lost fallthrough becomes `external-failed`, not contended).
   Restore with `mv -f` and re-run green.
2. Back up `internal/app/repository_repair.go`. Change `repositoryRepairSubject` to
   `"docket: repair derived views"` and confirm it landed. Run
   `…-run '^TestIntegrationRepoRepairAppliesOneDescendantCommit$'…`. It must FAIL. Restore and
   re-run green.

- [ ] **Step 5: Commit**

```bash
git add internal/app/repository_repair_integration_test.go internal/app/repomigration_integration_test.go tests/test_go_integration_app_reporepair.sh tests/runtime-budgets.tsv
git commit -m "test(0496): real-git repository repair shard"
```

---

### Task 5: `migrate` only migrates

**Risk:** standard. It removes a code path and moves helpers between files in one package. The
risk is a lost test or a dangling reference, and the compiler and the shard catch both.

**Files:**
- Modify: `internal/app/repository_migrate.go`
- Modify: `internal/app/repository_repair.go` (receives the moved helpers)
- Delete: `internal/app/repository_migrate_repair.go`
- Delete: `internal/app/repository_migrate_repair_test.go` (its helper tests move to
  `repository_repair_test.go`)
- Modify: `internal/app/repository_repair_test.go`
- Modify: `internal/app/repository_migrate_test.go`
- Modify: `internal/app/repomigration_integration_test.go`
- Modify: `internal/cli/repository.go` (`--repair-frontmatter` help text and the migrate
  builder comment)

**Interfaces:**
- Consumes: Task 2's `RunRepositoryRepair` users of `splitDerivedFindings`, `derivedRepairFiles`,
  `composeDerivedRepairBytes`, `snapshotChangeByPath` (signatures unchanged by the move).
- Produces: `migrateNoOp(sourceRevision string) RepositoryMigrateResult` whose human text is
  exactly `repository already migrated; for mechanical repairs run docket repository repair`.
  `RepositoryMigrateResult` loses its `RepairedViews` field.

- [ ] **Step 1: Write the failing tests**

Append to `internal/app/repository_migrate_test.go`:

```go
// TestMigrateNoOpNamesRepairCommand pins change 0496: migrate only migrates, and
// its already-migrated no-op names the command that repairs a migrated
// repository.
func TestMigrateNoOpNamesRepairCommand(t *testing.T) {
	out := migrateNoOp("tip")
	if out.Result != ResultNoOp {
		t.Fatalf("Result = %q, want no-op", out.Result)
	}
	if got, want := out.HumanText(), "repository already migrated; for mechanical repairs run docket repository repair"; got != want {
		t.Errorf("human = %q, want %q", got, want)
	}
}
```

In `internal/app/repomigration_integration_test.go`, **delete**
`TestIntegrationRepoMigrationHealthyRepairPreview`, `TestIntegrationRepoMigrationHealthyRepairApplies`,
and the section comment above them ("healthy-repository derived-view repair (change 0377)"). Their
coverage now lives in Task 4's shard. Add in their place:

```go
// TestIntegrationRepoMigrationMigratedRepoNeverRepairs pins change 0496: on an
// already-migrated repository with repairable drift, migrate writes nothing under
// every flag combination and names `docket repository repair` (Review Focus 5).
func TestIntegrationRepoMigrationMigratedRepoNeverRepairs(t *testing.T) {
	r := newHealthyRepo(t)
	r.publishHealthyDrift(t)
	before := currentDocketTip(t, r)
	for _, o := range []MigrateOptions{
		{},
		{Authorized: true},
		{RepairAuthorized: true},
		{Authorized: true, RepairAuthorized: true},
	} {
		res := r.runMigrate(t, o)
		if res.Result != ResultNoOp {
			t.Errorf("migrate %+v = %q (%s), want no-op", o, res.Result, res.HumanText())
		}
		if !strings.Contains(res.HumanText(), "docket repository repair") {
			t.Errorf("migrate %+v human must name `docket repository repair`: %q", o, res.HumanText())
		}
		if after := currentDocketTip(t, r); after != before {
			t.Fatalf("migrate %+v wrote on a migrated repository: docket %s -> %s", o, before, after)
		}
	}
}
```

- [ ] **Step 2: Run the tests and confirm they fail**

Run: `go test -count=1 -run TestMigrateNoOpNamesRepairCommand ./internal/app/` and
`go test -tags integration -count=1 -run '^TestIntegrationRepoMigrationMigratedRepoNeverRepairs$' ./internal/app/`.
Expected: both FAIL. The unit test fails on the old human text. The integration test fails because
`{Authorized: true, RepairAuthorized: true}` returns `applied` and moves the tip, and the human
text lacks the repair command.

- [ ] **Step 3: Make migrate only migrate and move the helpers**

In `internal/app/repository_migrate.go`:

```go
	if phase == phaseAlreadyMigrated {
		// Migrate only migrates (change 0496). An already-migrated repository is
		// the idempotent no-op under every flag combination; mechanical repair on a
		// migrated repository is `docket repository repair`, and the no-op names it.
		return migrateNoOp(sc.metadataTip)
	}
```

```go
// migrateNoOp is the idempotent already-migrated document, keyed on the remote
// postconditions (metadata branch present, no live surface on integration). It
// names the repair command, so a migrated-repo run never silently ignores a
// --repair-frontmatter it no longer honors.
func migrateNoOp(sourceRevision string) RepositoryMigrateResult {
	out := newMigrateResult(ResultNoOp, RepositoryMigrateResult{
		RepositoryState: string(reposetup.StateHealthy),
		SourceRevision:  sourceRevision,
		CopyPrefixes:    []string{},
		RemovedPaths:    []string{},
	})
	out.human = "repository already migrated; for mechanical repairs run docket repository repair"
	return out
}
```

- Update the `MigrateOptions` doc comment so `RepairAuthorized` reads: "is --repair-frontmatter,
  which authorizes the mechanical frontmatter repairs a LEGACY migration's plan lists; an
  already-migrated repository is a no-op that names `docket repository repair`".
- Remove the `RepairedViews` field from `RepositoryMigrateResult` (nothing sets it after this
  change).
- Move `splitDerivedFindings`, `derivedRepairFiles`, `composeDerivedRepairBytes`, and
  `snapshotChangeByPath` (with their doc comments) verbatim from `repository_migrate_repair.go`
  into `repository_repair.go`, below `executeRepositoryRepair`. Add the imports they need
  (`document`, `domain`, `render`).
- `git rm internal/app/repository_migrate_repair.go`. This deletes `migrateHealthyRepair`,
  `executeDerivedRepair`, `derivedRepairPreviewText`, `derivedRepairConfirmationRequired`,
  `derivedRepairApplied`, and `migrateRepairSubject`.
- Move `TestSplitDerivedFindings`, `TestDerivedRepairFilesSortedUnique`,
  `TestComposeDerivedRepairBoardBytes`, and `TestComposeDerivedRepairArtifactLinksBytes` verbatim
  into `repository_repair_test.go`, merging imports. Then
  `git rm internal/app/repository_migrate_repair_test.go`. The two
  `TestDerivedRepair{ConfirmationRequiredNamesYes,AppliedNamesRevisionAndPending}` tests are
  dropped: their subjects are deleted, and Task 2's `TestRepairConfirmationRequiredNamesYes` /
  `TestRepairAppliedNamesRevisionAndPending` guard the replacement (learning
  test-premise-deleted-not-regated).
- Fix comment anchors that named the deleted symbols. Run `git grep -n "migrateHealthyRepair\|executeDerivedRepair\|derivedRepairApplied\|derivedRepairConfirmationRequired\|migrateRepairSubject\|repository_migrate_repair" -- ':!docs'`
  and update every hit outside `docs/` to the new symbol (`RunRepositoryRepair` /
  `executeRepositoryRepair` / `repository_repair.go`). The result must be empty. Also replace the
  stale sentence in the header comment of `internal/reposetup/derived.go` that says
  "`repository migrate` repairs exactly the deterministic (Repairable) ones" with
  "`repository repair` repairs exactly the deterministic (Repairable) ones". Task 6 owns that
  file's remedy string, so leave the remedy for Task 6.

In `internal/cli/repository.go`, change the `--repair-frontmatter` help to
`"authorize the mechanical frontmatter repairs a legacy migration's plan lists"`, and add one
sentence to `newRepositoryMigrateCommand`'s doc comment: "On an already-migrated repository the
service is a no-op naming `docket repository repair`."

- [ ] **Step 4: Run the tests and confirm they pass**

Run: `go build ./... && go vet ./internal/app/ ./internal/cli/`,
`go test -count=1 ./internal/app/ ./internal/cli/ -run 'TestMigrate|TestRepository|TestRepair|TestPlanRepositoryRepair|TestSplitDerived|TestDerivedRepair|TestComposeDerived|Schema|Registry'`,
`go test -tags integration -count=1 -run '^TestIntegrationRepoMigration' ./internal/app/`, and
`go test -tags integration -count=1 -run '^TestIntegrationRepoRepair' ./internal/app/`.
Expected: PASS for all.

- [ ] **Step 5: Mutation-test the no-op branch**

Back up `internal/app/repository_migrate.go` and replace the body of the `phaseAlreadyMigrated`
branch with `return migrateConfirmationRequired(sc.metadataTip, reposetup.MigrationPlan{}, migrationRepairs{}, "x")`.
Confirm it landed. Run `…-run '^TestIntegrationRepoMigrationMigratedRepoNeverRepairs$'…`. It must
FAIL. Restore with `mv -f` and re-run green.

- [ ] **Step 6: Commit**

```bash
git add -u internal/app internal/cli
git add internal/app/repository_repair.go internal/app/repository_repair_test.go
git commit -m "refactor(0496): migrate only migrates; derived-view repair moves to repository repair"
```

(`git add -u` stages the two deletions and the modified tracked files under those two directories
only. Run `git status --short` first and confirm nothing outside the files listed above is staged.)

---

### Task 6: Remedies name a working command, plus the family guard

**Risk:** standard. These are string retargets plus one guard that has to be keyed on the finding
family and mutation-tested.

**Files:**
- Modify: `internal/reposetup/health.go` (`frontmatterFinding` repairable remedy)
- Modify: `internal/reposetup/derived.go` (`DerivedFinding.Finding` repairable remedy)
- Modify: `internal/app/status.go` (`branchMalformedCheck` remedy + its doc comment)
- Modify: `internal/app/status_branch_malformed_test.go`
- Create: `internal/app/repair_remedy_family_test.go`

**Interfaces:**
- Consumes: `checkCorpusOutcome(cfg, corpus, readErr) ([]reposetup.RepairFinding, []reposetup.Finding)`,
  `reposetup.EvaluateHealth(c Classification, f Facts, fm []RepairFinding) []Finding`,
  `derivedTestConfig`, `changeRecordBytes`, `derivedChangeFM` (existing).
- Produces: nothing consumed later.

- [ ] **Step 1: Write the failing guard and update the status test**

Create `internal/app/repair_remedy_family_test.go`:

```go
package app

import (
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/render"
	"github.com/danielhanold/docket/internal/reposetup"
	"github.com/danielhanold/docket/internal/repository"
)

// TestRepairableFamilyRemediesNameRepairNotMigrate pins change 0496's remedy
// rule over the whole mechanically-repairable finding FAMILY, keyed on shape, not
// on a hand-listed set of codes: every finding `repository check` emits with a
// non-nil Repairable pointer (the frontmatter roster via frontmatterFinding and
// the derived views via DerivedFinding.Finding — the only two producers of that
// pointer) must never send the reader to `docket repository migrate`, which does
// not repair a migrated repository; each REPAIRABLE one must name `docket
// repository repair`. The population is produced by the real check pipeline over
// a corpus carrying every member kind, with a floor so a dead fixture cannot pass
// vacuously.
//
// Mutation probes: restore "Run `docket repository migrate`" in derived.go's
// repairable remedy, or "Apply the previewed mechanical repair" in health.go's —
// each must redden this test.
func TestRepairableFamilyRemediesNameRepairNotMigrate(t *testing.T) {
	cfg := derivedTestConfig()
	malformedFM := strings.Replace(derivedChangeFM, "id: 1\nslug: example", "id: 4\nslug: malformed", 1)
	corpus := checkCorpus{
		records: []corpusRecord{
			// repairable frontmatter: a real claim stamp on a final archived record
			{path: "docs/changes/archive/2026-01-02-0003-archived-change.md", bytes: []byte("---\nid: 3\nslug: archived-change\nstatus: done\ntitle: Change archived-change\ntype: feature\nclaimed_at: 2026-08-01T10:00:00Z\n---\n\nBody.\n"), kind: repository.KindChange, location: repository.LocationArchive},
			// manual-review frontmatter: an unsafe scalar with an ambiguous decode
			{path: "docs/changes/active/0002-other.md", bytes: []byte("---\nid: 2\nslug: other\nstatus: proposed\ntitle: true\ntype: feature\n---\n\nBody.\n"), kind: repository.KindChange, location: repository.LocationActive},
			// repairable derived: a stale artifact-links block
			{path: "docs/changes/active/0001-example.md", bytes: changeRecordBytes(derivedChangeFM, ""), kind: repository.KindChange, location: repository.LocationActive},
			// non-repairable derived: an unbalanced managed marker
			{path: "docs/changes/active/0004-malformed.md", bytes: []byte("---\n" + malformedFM + "---\n\n## Artifacts\n\n<!-- docket:artifacts:start (generated — do not hand-edit) -->\n| Artifact | Link |\n\n## Why\n\nbody\n"), kind: repository.KindChange, location: repository.LocationActive},
		},
		link:  render.LinkContext{MetadataBranch: reposetup.MetadataBranchName},
		board: corpusFile{present: true, bytes: []byte("# Backlog\n\nstale bytes\n")},
	}
	fm, extra := checkCorpusOutcome(cfg, corpus, nil)
	cls := reposetup.Classification{State: reposetup.StateHealthy}
	findings := append(reposetup.EvaluateHealth(cls, reposetup.Facts{}, fm), extra...)

	seen := map[string]bool{}
	var repairable, manual int
	for _, f := range findings {
		if f.Repairable == nil {
			continue // not a member of the mechanically-repairable family
		}
		seen[f.Code] = true
		if strings.Contains(f.Remedy, "repository migrate") {
			t.Errorf("finding %s (%s) remedy names `repository migrate`, which does not repair a migrated repository: %q", f.Code, f.Ref, f.Remedy)
		}
		if *f.Repairable {
			repairable++
			if !strings.Contains(f.Remedy, "docket repository repair") {
				t.Errorf("repairable finding %s (%s) remedy must name `docket repository repair`: %q", f.Code, f.Ref, f.Remedy)
			}
		} else {
			manual++
		}
	}
	// Population floor (marker-scoped-guard-needs-a-population-floor): both
	// producers, both polarities.
	for _, code := range []string{
		string(reposetup.RepairDropClaimedAt), "frontmatter-manual-review",
		reposetup.CodeBoardStale, reposetup.CodeArtifactLinksStale, reposetup.CodeArtifactLinksMalformed,
	} {
		if !seen[code] {
			t.Errorf("population floor: the fixture produced no %s finding (seen: %v)", code, seen)
		}
	}
	if repairable < 3 || manual < 2 {
		t.Errorf("population floor: repairable=%d manual=%d, want >=3 and >=2", repairable, manual)
	}
}
```

In `internal/app/status_branch_malformed_test.go`, change the no-PR remedy assertion to:

```go
	for _, id := range []string{"0002", "0008"} {
		r := byIdentity[id][0].Remedy
		if strings.Contains(r, "change relink") || strings.Contains(r, "repository migrate") || !strings.Contains(r, "docket repository repair") {
			t.Errorf("change %s remedy = %q, want the hand-edit + `docket repository repair` remedy", id, r)
		}
	}
```

and in that test's doc comment, change "plus repository migrate remedy" to
"plus `docket repository repair` remedy".

- [ ] **Step 2: Run the tests and confirm they fail**

Run: `go test -count=1 -run 'TestRepairableFamilyRemediesNameRepairNotMigrate|TestStatusBranchMalformedFindings' ./internal/app/`
Expected: FAIL. The derived repairable remedies name `repository migrate`, the frontmatter
repairable remedy does not name `docket repository repair`, and the status remedy names
`repository migrate`. If a population-floor line fails instead, fix the fixture first (add the
missing field) until only the remedy assertions fail.

- [ ] **Step 3: Retarget the remedies**

`internal/reposetup/health.go`, in `frontmatterFinding`'s repairable branch:

```go
			Remedy:     "Run `docket repository repair` to preview and apply this repair, or edit the record frontmatter by hand.",
```

`internal/reposetup/derived.go`, in `DerivedFinding.Finding`'s repairable branch:

```go
			Remedy:     "Run `docket repository repair` to recompute the canonical " + string(df.View) + " output, or edit the file by hand.",
```

`internal/app/status.go`, in `branchMalformedCheck`:

```go
	remedy := "correct branch: on the change record on the docket branch (the real feature branch, or clear it if no branch was ever created), then run: docket repository repair to re-render the board"
```

and in its doc comment change "plus repository migrate to re-render the board" to
"plus `docket repository repair` to re-render the board".

Leave every legacy / partial / local-attachment remedy that names `docket repository migrate`
untouched. Confirm with
`git grep -n "repository migrate" -- internal/reposetup/health.go internal/reposetup/derived.go internal/app/status.go`.
Only the legacy, partial, and attachment remedies may remain.

- [ ] **Step 4: Run the tests and confirm they pass**

Run: `go test -count=1 ./internal/reposetup/` and
`go test -count=1 -run 'TestRepairableFamily|TestStatusBranchMalformed|TestRepositoryCheck|TestDerivedViewFindings|TestCorpusFindings' ./internal/app/`.
Expected: PASS. If a `reposetup` health test asserted the old frontmatter remedy text, update its
expected string to the new remedy. Never delete that test.

- [ ] **Step 5: Mutation-test the guard (both producers)**

For each of `internal/reposetup/derived.go` and `internal/reposetup/health.go`, one at a time: back
it up, restore the old remedy string (derived: ``"Run `docket repository migrate` to recompute the canonical " + string(df.View) + " output, or edit the file by hand."``;
health: `"Apply the previewed mechanical repair, or edit the record frontmatter manually."`), and
confirm it landed with `grep -c`. Run
`go test -count=1 -run TestRepairableFamilyRemediesNameRepairNotMigrate ./internal/app/`. It must
FAIL. Restore with `mv -f` and re-run green.

- [ ] **Step 6: Commit**

```bash
git add internal/reposetup/health.go internal/reposetup/derived.go internal/app/status.go internal/app/status_branch_malformed_test.go internal/app/repair_remedy_family_test.go
git commit -m "fix(0496): repair remedies name docket repository repair, guarded by finding family"
```

(Add any `internal/reposetup/*_test.go` you updated in Step 4 to the same commit.)

---

### Task 7: Skills, glossary, embedded mirror, and the exemption pin

**Risk:** standard. These are prose edits, but they sit under a counted laundering guard and a
generated-mirror drift guard, and both must end green on measured values.

**Files:**
- Modify: `skills/docket-adr/SKILL.md` (the ADR-index drift repair block)
- Modify: `skills/docket-status/SKILL.md` (the `artifact-links-stale` self-heal sentence)
- Modify: `skills/docket-convention/SKILL.md` (*Board refresh on status writes*)
- Modify: `docs/reference/glossary.md` (the *ADR index / `## Update` note* code block and the
  *Repository check / migrate* entry)
- Regenerate: `internal/assets/embedded/**` via `go generate ./internal/assets/`
- Modify: `internal/repoguard/capability_surface_test.go` (`capabilityExemptions` count for
  `docket repository migrate`)

**Interfaces:**
- Consumes: the catalog id `repository.repair` (Task 3).
- Produces: nothing consumed later.

- [ ] **Step 1: Derive the site list and confirm it**

Run:
`git grep -n "repository migrate\|repair-frontmatter\|previewed mechanical repair" -- skills agents cursor-rules AGENTS.md CLAUDE.md docs/reference/glossary.md`
and sort each hit into **repair-on-a-migrated-repo** (retarget) vs **legacy / partial / bootstrap**
(keep). Expected retarget set: `skills/docket-adr/SKILL.md` (the
`docket repository migrate --repair-frontmatter` block and its lead-in sentence),
`skills/docket-status/SKILL.md` (the "One drift self-heals" sentence),
`skills/docket-convention/SKILL.md` (the *Board refresh on status writes* sentence "repaired by an
authorized, human-typed `docket repository migrate`"), and `docs/reference/glossary.md` (the
`--repair-frontmatter   # human-typed: re-renders a stale index` line and the *Repository check /
migrate* entry). Every other hit is a legacy, bootstrap, or half-migrated remedy and stays. If the
grep shows a repair-on-migrated site not listed here, retarget it the same way and name it in the
commit message.

- [ ] **Step 2: Edit the skills (catalog id, never a hard-coded argv)**

`skills/docket-adr/SKILL.md`. Replace the sentence ending "regenerate the drifted index through
the authorized mechanical repair:" and the fenced block after it with:

````markdown
A deterministic `adr-index-stale` finding is `Repairable`; regenerate the drifted index through the authorized mechanical repair — the `repository.repair` operation, which previews the repair set and writes only when re-run with `--yes`:

```
repository.repair  --yes   # resolve argv from the capability catalog; run it without --yes first to preview
```
````

(Keep the paragraph that follows, "Repair re-renders only the canonical derived bytes it owns …",
unchanged.)

`skills/docket-status/SKILL.md`. Replace
"One drift self-heals: a stale `## Artifacts` block surfaces later as an `artifact-links-stale` finding that authorized `docket repository migrate` re-renders."
with
"One drift self-heals: a stale `## Artifacts` block surfaces later as an `artifact-links-stale` finding that the authorized `repository.repair` operation re-renders (resolve argv from the capability catalog)."

`skills/docket-convention/SKILL.md`, *Board refresh on status writes*. Replace
"is surfaced by `repository.check` and repaired by an authorized, human-typed `docket repository migrate` — never a hand-render."
with
"is surfaced by `repository.check` and repaired by the authorized `repository.repair` operation (resolve argv from the capability catalog; a human authorizes its `--yes` apply) — never a hand-render."

Do not touch the convention's two human-typed bootstrap remedies (`docket repository migrate` and
`docket repository init`) anywhere else.

- [ ] **Step 3: Edit the glossary**

In *ADR index / `## Update` note*, replace the line
``docket repository migrate --repair-frontmatter   # human-typed: re-renders a stale index`` with:

```sh
docket repository repair --yes   # human-typed: re-renders a stale index (run without --yes to preview)
```

In *Repository check / migrate*, keep the heading (the glossary index links to its anchor) and
replace the body and block with:

````markdown
`repository check` is the read-only topology and consistency check (including board drift).
`repository migrate` is the human-typed migration path for a legacy single-branch repository.
`repository repair` is the human-authorized repair path on a migrated one: it previews every
mechanically repairable finding `check` reports, and applies them in one commit with `--yes`.

```sh
docket repository check
docket repository migrate        # legacy repository only
docket repository repair         # preview; add --yes to apply
```
````

- [ ] **Step 4: Regenerate the embedded mirror**

Run: `go generate ./internal/assets/`, then
`git status --short internal/assets/embedded`.
Expected: the three skill mirrors and the manifest change, and nothing else.

- [ ] **Step 5: Re-pin the exemption count to the measured value**

Run: `go test -count=1 -run '^TestCapabilitySurface$' ./internal/repoguard/`. Expected: FAIL on the
`docket repository migrate` pin (9), reporting the measured count (expected 6: three skill sites
moved to the catalog id). It must report **no** violations. A violation means a
`docket repository repair` spelling landed on a skill surface, so fix the skill, never the pin.
Set `"docket repository migrate":` in `capabilityExemptions` to the reported number, then re-run.
Expected: PASS.

- [ ] **Step 6: Run the prose and asset guards**

Run: `go test -count=1 ./internal/repoguard/ ./internal/assets/`.
Expected: PASS. If a repoguard prose test pinned a sentence you replaced, repoint the assert at the
new owning text (learning restatement-accumulates-its-own-guards). Never re-add the old text.

- [ ] **Step 7: Commit**

```bash
git add skills/docket-adr/SKILL.md skills/docket-status/SKILL.md skills/docket-convention/SKILL.md docs/reference/glossary.md internal/assets/embedded internal/repoguard/capability_surface_test.go
git commit -m "docs(0496): skills and glossary name repository.repair; re-pin the migrate exemption count"
```

(Add any repoguard test you repointed in Step 6 to the same commit.)

---

## Self-review (done at authoring time)

- **Spec coverage:** §1 `planClaimedAt` → Task 1. §2 command, preconditions, repair set,
  composition order, two-pass authorization, publisher reuse, subject, and result shape → Tasks 2,
  3, and 4. §3 migrate on a migrated repository → Task 5. §4 remedies → Tasks 6 and 7, plus the
  exemption pin in Task 7. Error handling: corpus read → `external-failed`; `ApplyRepairs`
  rejection and render failure → `internal-error` before any write (both live in
  `planRepositoryRepair`, which runs before the publisher); lease lost → `contended`; legacy,
  partial, and unknown → typed refusals (Tasks 2 and 4). Testing list → Tasks 1, 2, 4, 5, and 6.
  The ADR is the coordinator's job (Global Constraints).
- **Known residual:** a hermetic test cannot easily force an `ApplyRepairs` rejection on
  roster-produced findings, because `buildRepair` is the same function on both sides. The
  `internal-error` mapping is a two-line pass-through in `RunRepositoryRepair` and is checked by
  review, not by test.
- **Names used across tasks:** `RepairOptions{Authorized, ExpectedSource}`,
  `RepositoryRepairResult{RepositoryState, SourceRevision, MetadataTip, Repairs, RepairedViews,
  RepairedFiles, ManualReview, PendingLocal, Findings}`, `ConfirmationRequired()`, `SourceRev()`,
  `planRepositoryRepair(sc, corpus)`, `repositoryRepairPlan{frontmatter, derived, manual, files,
  contents}`, `repositoryRepairSubject`, `OperationRepositoryRepair`, and `repositoryRepairRunner`
  are consistent in Tasks 2–5.
