package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/repository"
)

// corpusRelDir is the frozen v0.9.3 semantic corpus, repo-relative from this
// package directory (internal/app). Its provenance — tag v0.9.3, peeled commit
// dd742abd5e9fcdf8ffe78eb6f36a293410873bbf — is recorded in
// testdata/repositories/v0.9.3/status-corpus/PROVENANCE.md.
const corpusRelDir = "../../testdata/repositories/v0.9.3/status-corpus"

// loadCorpusBlobs walks the frozen corpus and returns one StatusBlob per record
// file, classifying kind and location from the repo-relative path prefix — the
// same prefix rule the Git-backed reader uses. The `.docket.yml` sidecar is
// returned separately (it is configuration, not a record).
func loadCorpusBlobs(t *testing.T) (blobs []StatusBlob, docketYML []byte) {
	t.Helper()
	root := corpusRelDir
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		data, readErr := os.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		switch {
		case rel == ".docket.yml":
			docketYML = data
			return nil
		case rel == "PROVENANCE.md":
			return nil // documentation, not a record
		case strings.HasPrefix(rel, "docs/changes/archive/"):
			blobs = append(blobs, StatusBlob{
				Kind:     repository.KindChange,
				Location: repository.LocationArchive,
				Path:     rel,
				Version:  rel, // deterministic, non-empty; not asserted in this fake-reader test
				Data:     data,
			})
		case strings.HasPrefix(rel, "docs/adrs/"):
			blobs = append(blobs, StatusBlob{
				Kind:     repository.KindADR,
				Location: repository.LocationLedger,
				Path:     rel,
				Version:  rel,
				Data:     data,
			})
		default:
			t.Fatalf("unclassified corpus file: %s", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk corpus: %v", err)
	}
	if len(blobs) == 0 {
		t.Fatalf("corpus walk found no record blobs under %s", root)
	}
	return blobs, docketYML
}

// corpusConfig resolves the frozen `.docket.yml` as the repository layer, the
// way PinContext would, and returns the snapshot together with the diagnostics
// the resolver raised. The frozen production config is faithful, not synthetic:
// it requests three capabilities Go v1 has not implemented (build.checkpoint,
// finalize.skip_results_only_delta, terminal_publish) plus one deferred setting
// (learnings.enabled), so `docket status` over this repo surfaces those four
// diagnostics as findings. Threading them is what makes the corpus the oracle
// for the operation's REAL output on this tree, not a substitute config.
func corpusConfig(t *testing.T, docketYML []byte) (config.Snapshot, []config.Diagnostic) {
	t.Helper()
	snap, diags, err := config.Resolve(
		[]config.Source{{Layer: config.LayerRepository, Name: ".docket.yml", Data: docketYML}},
		config.ResolveContext{DefaultBranch: "main"},
	)
	if err != nil {
		t.Fatalf("resolve frozen .docket.yml: %v", err)
	}
	return *snap, diags
}

// TestStatusCorpusFrozenSemantics drives app.Status over the frozen v0.9.3
// corpus through a fake reader and asserts outcomes DERIVED BY HAND from the
// frozen records — the corpus is the oracle, never the code.
//
// The corpus is the v0.9.3 tag's tree (docket's `main`-branch content), which
// carries only TERMINAL records: 9 archived changes and 5 Accepted ADRs, no
// active changes and no learnings. It therefore exercises the complete-corpus
// inventory and the health/validation path with an empty active projection;
// readiness and ready-queue semantics over active changes are covered by the
// fake-reader unit tests in status_test.go, because this tag's tree has none.
//
// Selection is fully self-consistent by construction so every RECORD finding is
// enumerable:
//   - changes {1,2,3,4,5,6,12,13,36}; ADRs {1,2,3,4,5} (contiguous 1..5, so ADR
//     id-gap warnings — one per unallocated id below the highest — cannot fire).
//   - every change/ADR cross-reference resolves WITHIN the slice EXCEPT change
//     36, whose depends_on:[35] and related:[35] both name change 35, which is
//     deliberately excluded. That yields exactly one error and one warning.
//
// On top of the two record findings, the frozen config contributes four
// findings (see corpusConfig): three deferred-capability ERRORS and one
// deferred-setting NOTICE. Findings are assembled config-first, so the full
// health tally is 4 errors + 1 warning + 1 notice = 6 findings.
func TestStatusCorpusFrozenSemantics(t *testing.T) {
	blobs, docketYML := loadCorpusBlobs(t)
	if docketYML == nil {
		t.Fatal("corpus is missing its .docket.yml sidecar")
	}
	cfg, cfgDiags := corpusConfig(t, docketYML)

	pin := StatusPin{
		Mode:                "docket",
		DefaultBranch:       "main",
		DefaultRevision:     "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		IntegrationBranch:   "main",
		IntegrationRevision: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		MetadataBranch:      "docket",
		MetadataRevision:    "cccccccccccccccccccccccccccccccccccccccc",
		Config:              cfg,
		ConfigDiags:         cfgDiags,
	}
	fake := &fakeReader{pin: pin, corpus: blobs, facts: domain.NewBranchFacts(nil)}

	got := Status(context.Background(), fake, StatusOptions{})
	if got.Result != ResultApplied {
		t.Fatalf("result = %q, want applied; message=%q", got.Result, got.Message)
	}

	// --- context echoes the docket-mode pin -----------------------------------
	if got.Context.MetadataMode != "docket" ||
		got.Context.MetadataRevision != pin.MetadataRevision ||
		got.Context.IntegrationRevision != pin.IntegrationRevision {
		t.Errorf("context did not echo the pin: %+v", got.Context)
	}

	// --- summary counts, derived by hand from the frozen records --------------
	// 9 archived changes captured; all terminal (status: done); 0 active.
	if got.Summary.TotalChanges != 9 {
		t.Errorf("TotalChanges = %d, want 9 (archived changes 1,2,3,4,5,6,12,13,36)", got.Summary.TotalChanges)
	}
	if got.Summary.ActiveChanges != 0 {
		t.Errorf("ActiveChanges = %d, want 0 (the tag's tree carries no active changes)", got.Summary.ActiveChanges)
	}
	if got.Summary.DisplayedChanges != 0 {
		t.Errorf("DisplayedChanges = %d, want 0 (no active changes to display)", got.Summary.DisplayedChanges)
	}
	if got.Summary.ReadyChanges != 0 {
		t.Errorf("ReadyChanges = %d, want 0 (readiness is evaluated over active changes only)", got.Summary.ReadyChanges)
	}
	if got.Summary.ADRs != 5 {
		t.Errorf("ADRs = %d, want 5 (ADRs 1..5)", got.Summary.ADRs)
	}
	if got.Summary.Learnings != 0 {
		t.Errorf("Learnings = %d, want 0 (the tag's tree carries no learnings ledger)", got.Summary.Learnings)
	}
	// Error tally: 3 deferred-capability config errors + change 36's dangling
	// depends_on:[35]. Warning tally: change 36's dangling related:[35]. (The
	// deferred-setting learnings.enabled diagnostic is a NOTICE, counted in
	// neither tally.)
	if got.Summary.ErrorFindings != 4 {
		t.Errorf("ErrorFindings = %d, want 4 (3 deferred-capability config errors + change 36 depends_on:[35])", got.Summary.ErrorFindings)
	}
	if got.Summary.WarningFindings != 1 {
		t.Errorf("WarningFindings = %d, want 1 (change 36 related:[35] dangling)", got.Summary.WarningFindings)
	}

	// --- empty active projection ----------------------------------------------
	if len(got.Ready) != 0 {
		t.Errorf("Ready = %v, want empty (no active/proposed changes)", got.Ready)
	}
	if len(got.Changes) != 0 {
		t.Errorf("Changes has %d rows, want 0 (no active changes displayed)", len(got.Changes))
	}

	// --- record inventory: changes by ascending id, then ADRs by ascending id -
	type recKey struct{ kind, identity, location string }
	wantRecords := []recKey{
		{"change", "0001", "archive"},
		{"change", "0002", "archive"},
		{"change", "0003", "archive"},
		{"change", "0004", "archive"},
		{"change", "0005", "archive"},
		{"change", "0006", "archive"},
		{"change", "0012", "archive"},
		{"change", "0013", "archive"},
		{"change", "0036", "archive"},
		{"adr", "0001", "ledger"},
		{"adr", "0002", "ledger"},
		{"adr", "0003", "ledger"},
		{"adr", "0004", "ledger"},
		{"adr", "0005", "ledger"},
	}
	if len(got.Records) != len(wantRecords) {
		t.Fatalf("Records has %d entries, want %d: %+v", len(got.Records), len(wantRecords), got.Records)
	}
	for i, want := range wantRecords {
		r := got.Records[i]
		if r.Kind != want.kind || r.Identity != want.identity || r.Location != want.location {
			t.Errorf("Records[%d] = {%s %s %s}, want {%s %s %s}",
				i, r.Kind, r.Identity, r.Location, want.kind, want.identity, want.location)
		}
	}

	// --- every finding, enumerated by hand ------------------------------------
	// Assembly order is config diagnostics, then parse findings (none), then the
	// validation report, then artifact findings (none). We assert by membership
	// (code + severity + field/identity) so the check is robust to intra-group
	// ordering while still pinning the exact set.
	type findKey struct{ code, severity, field, identity string }
	wantFindings := map[findKey]bool{
		// Config layer: three deferred capabilities (errors) + one deferred
		// setting (notice). Identity is empty — a config diagnostic names a
		// setting path, not a repository entity.
		{"deferred-capability-requested", "error", "build.checkpoint", ""}:                 false,
		{"deferred-capability-requested", "error", "finalize.skip_results_only_delta", ""}: false,
		{"deferred-capability-requested", "error", "terminal_publish", ""}:                 false,
		{"deferred-setting", "notice", "learnings.enabled", ""}:                            false,
		// Record layer: change 36's two dangling references to the excluded 35.
		{"change-reference-dangling", "error", "depends_on", "0036"}: false,
		{"change-reference-dangling", "warning", "related", "0036"}:  false,
	}
	if len(got.Findings) != len(wantFindings) {
		t.Fatalf("Findings has %d entries, want %d: %+v", len(got.Findings), len(wantFindings), got.Findings)
	}
	for _, f := range got.Findings {
		k := findKey{f.Code, f.Severity, f.Field, f.Identity}
		seen, expected := wantFindings[k]
		if !expected {
			t.Errorf("unexpected finding: %+v", f)
			continue
		}
		if seen {
			t.Errorf("duplicate finding: %+v", f)
		}
		wantFindings[k] = true
	}
	for k, seen := range wantFindings {
		if !seen {
			t.Errorf("missing expected finding: %+v", k)
		}
	}
}
