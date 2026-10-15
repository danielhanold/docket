package app

import (
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/reposetup"
	"github.com/danielhanold/docket/internal/repository"
)

// danglingDependencyRecord is an active change whose depends_on names a change
// that does not exist: a validator finding, not a frontmatter-shape one.
func danglingDependencyRecord() corpusRecord {
	return corpusRecord{
		path:     "docs/changes/active/0005-waits.md",
		bytes:    changeRecordBytes("id: 5\nslug: waits\ntitle: Waits\nstatus: proposed\npriority: medium\ntype: feature\ncreated: 2026-08-30\nupdated: 2026-08-30\ndepends_on: [172]\n", ""),
		kind:     repository.KindChange,
		location: repository.LocationActive,
	}
}

// TestCheckReportsValidatorCodeNotManualReview pins the code switch: a
// validator finding surfaced by check carries the validator's own code and the
// shared plain-language text, never the frontmatter-manual-review wrapper.
//
// Mutation probe: drop FindingCode's ReviewCode arm — this test must redden.
func TestCheckReportsValidatorCodeNotManualReview(t *testing.T) {
	rec := danglingDependencyRecord()
	fm := corpusFindings(derivedTestConfig(), []corpusRecord{rec}, reposetup.RecordLocator{Checkout: ".docket"})
	findings := reposetup.EvaluateHealth(reposetup.Classification{State: reposetup.StateHealthy}, reposetup.Facts{}, fm)
	var got *reposetup.Finding
	for i := range findings {
		if findings[i].Ref == rec.path {
			got = &findings[i]
		}
	}
	if got == nil {
		t.Fatalf("no finding for %s in %+v", rec.path, findings)
	}
	if got.Code != repository.CodeChangeReferenceDangling {
		t.Errorf("code = %q, want %q", got.Code, repository.CodeChangeReferenceDangling)
	}
	if got.Severity != reposetup.SeverityError {
		t.Errorf("severity = %q, want error", got.Severity)
	}
	if got.Repairable == nil || *got.Repairable {
		t.Errorf("Repairable = %v, want non-nil false", got.Repairable)
	}
	if got.Impact == "" {
		t.Error("impact is empty")
	}
	if want := "`.docket/docs/changes/active/0005-waits.md`"; !strings.Contains(got.Remedy, want) {
		t.Errorf("remedy %q does not name %s", got.Remedy, want)
	}
	if strings.Contains(got.Message, "metadata corpus finding") {
		t.Errorf("message still carries the wrapper text: %q", got.Message)
	}
}

// TestPlanRepositoryRepairManualListNamesValidatorCode proves repair's list of
// what it can't fix names the validator's own code. A setupContext holding only
// cfg suffices because no derived repair is planned for this corpus.
func TestPlanRepositoryRepairManualListNamesValidatorCode(t *testing.T) {
	plan, err := planRepositoryRepair(setupContext{cfg: derivedTestConfig()},
		checkCorpus{records: []corpusRecord{danglingDependencyRecord()}, link: repairLink()})
	if err != nil {
		t.Fatalf("planRepositoryRepair: %v", err)
	}
	want := "[" + repository.CodeChangeReferenceDangling + "]"
	found := false
	for _, line := range plan.manual {
		if strings.Contains(line, want) {
			found = true
		}
	}
	if !found {
		t.Errorf("manual list %q does not name %s", plan.manual, want)
	}
}
