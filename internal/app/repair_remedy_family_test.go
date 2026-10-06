package app

import (
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/layout"
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
		link:  render.LinkContext{MetadataBranch: layout.SharedName},
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
