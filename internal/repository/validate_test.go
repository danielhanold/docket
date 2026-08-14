package repository

import (
	"fmt"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/domain"
)

// minimalChange writes a well-formed change manifest, with extra frontmatter
// lines appended verbatim so a case can add or override exactly the field it
// is about.
func minimalChange(id int, slug, status string, extra ...string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "---\nid: %d\nslug: %s\ntitle: 'A change'\nstatus: %s\n", id, slug, status)
	b.WriteString("priority: medium\ntype: feat\ncreated: 2026-08-01\nupdated: 2026-08-02\n")
	for _, line := range extra {
		b.WriteString(line + "\n")
	}
	b.WriteString("---\n\n## Why\n\nBody.\n")
	return b.String()
}

// minimalADR writes a well-formed ADR record.
func minimalADR(id int, slug, status string, extra ...string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "---\nid: %d\nslug: %s\ntitle: 'A decision'\nstatus: %s\ndate: 2026-08-01\n", id, slug, status)
	for _, line := range extra {
		b.WriteString(line + "\n")
	}
	b.WriteString("---\n\n## Decision\n\nBody.\n")
	return b.String()
}

// minimalLearning writes a well-formed learnings-ledger finding.
func minimalLearning(slug string, extra ...string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "---\nslug: %s\nhook: \"A hook.\"\ntopics: [process]\nchanges: []\n", slug)
	b.WriteString("created: 2026-08-01\nupdated: 2026-08-01\npromotion_state: retained\n")
	for _, line := range extra {
		b.WriteString(line + "\n")
	}
	b.WriteString("---\n\n## Apply\n\nBody.\n")
	return b.String()
}

// record builds one supplied document.
func record(t *testing.T, kind RecordKind, loc RecordLocation, recordPath, source string) InputDocument {
	t.Helper()
	return input(t, kind, loc, recordPath, source)
}

// buildRecords builds a snapshot over the given documents, failing on a
// call-shape error: every case here supplies a well-formed call.
func buildRecords(t *testing.T, docs ...InputDocument) BuildResult {
	t.Helper()
	result, err := BuildSnapshot(BuildInput{Config: effectiveConfig(), Documents: docs})
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	return result
}

// changeEntity is the finding subject a change record produces.
func changeEntity(id int, slug, recordPath string) domain.EntityRef {
	return domain.EntityRef{Kind: domain.EntityChange, ID: id, Slug: slug, Path: recordPath}
}

func TestValidateSingleSnapshot(t *testing.T) {
	activePath := "docs/changes/active/0042-widget.md"
	archivePath := "docs/changes/archive/2026-08-01-0042-widget.md"

	cases := []struct {
		name     string
		docs     []InputDocument
		code     string
		severity domain.Severity
		field    string
		entity   domain.EntityRef
		absent   bool // the code must NOT appear
	}{
		{
			name:     "unusable numeric id",
			docs:     []InputDocument{record(t, KindChange, LocationActive, "docs/changes/active/0000-widget.md", minimalChange(0, "widget", "proposed"))},
			code:     CodeChangeIDInvalid,
			severity: domain.SeverityError,
			field:    "id",
			entity:   changeEntity(0, "widget", "docs/changes/active/0000-widget.md"),
		},
		{
			name:     "slug outside the token grammar",
			docs:     []InputDocument{record(t, KindChange, LocationActive, activePath, minimalChange(42, "Widget Thing", "proposed"))},
			code:     CodeChangeSlugInvalid,
			severity: domain.SeverityError,
			field:    "slug",
			entity:   changeEntity(42, "Widget Thing", activePath),
		},
		{
			name:     "status outside the closed set",
			docs:     []InputDocument{record(t, KindChange, LocationActive, activePath, minimalChange(42, "widget", "shipped"))},
			code:     CodeChangeStatusUnknown,
			severity: domain.SeverityError,
			field:    "status",
			entity:   changeEntity(42, "widget", activePath),
		},
		{
			name: "priority outside the closed set is a warning",
			docs: []InputDocument{record(t, KindChange, LocationActive, activePath,
				strings.Replace(minimalChange(42, "widget", "proposed"), "priority: medium", "priority: urgent", 1))},
			code:     CodeChangePriorityUnknown,
			severity: domain.SeverityWarning,
			field:    "priority",
			entity:   changeEntity(42, "widget", activePath),
		},
		{
			name:     "filename disagrees with frontmatter identity",
			docs:     []InputDocument{record(t, KindChange, LocationActive, "docs/changes/active/0043-widget.md", minimalChange(42, "widget", "proposed"))},
			code:     CodeChangeFilenameMismatch,
			severity: domain.SeverityError,
			field:    "path",
			entity:   changeEntity(42, "widget", "docs/changes/active/0043-widget.md"),
		},
		{
			name: "filename slug truncated by the writer is not a mismatch",
			docs: []InputDocument{record(t, KindChange, LocationActive, "docs/changes/active/0042-widget-with-a-very-long.md",
				minimalChange(42, "widget-with-a-very-long-authored-slug", "proposed"))},
			code:   CodeChangeFilenameMismatch,
			absent: true,
		},
		{
			name:     "terminal status left in active",
			docs:     []InputDocument{record(t, KindChange, LocationActive, activePath, minimalChange(42, "widget", "done"))},
			code:     CodeChangePlacementInvalid,
			severity: domain.SeverityError,
			field:    "status",
			entity:   changeEntity(42, "widget", activePath),
		},
		{
			name:     "live status found in archive",
			docs:     []InputDocument{record(t, KindChange, LocationArchive, archivePath, minimalChange(42, "widget", "proposed"))},
			code:     CodeChangePlacementInvalid,
			severity: domain.SeverityError,
			field:    "status",
			entity:   changeEntity(42, "widget", archivePath),
		},
		{
			name:     "archive filename carries no usable date",
			docs:     []InputDocument{record(t, KindChange, LocationArchive, "docs/changes/archive/0042-widget.md", minimalChange(42, "widget", "done"))},
			code:     CodeChangeArchiveDateInvalid,
			severity: domain.SeverityError,
			field:    "path",
			entity:   changeEntity(42, "widget", "docs/changes/archive/0042-widget.md"),
		},
		{
			name: "archived terminal record still holding a claim stamp",
			docs: []InputDocument{record(t, KindChange, LocationArchive, archivePath,
				minimalChange(42, "widget", "done", "claimed_at: 2026-08-01T10:00:00Z"))},
			code:     CodeChangeTerminalClaimStamp,
			severity: domain.SeverityError,
			field:    "claimed_at",
			entity:   changeEntity(42, "widget", archivePath),
		},
		{
			name: "in-progress without a branch",
			docs: []InputDocument{record(t, KindChange, LocationActive, activePath,
				minimalChange(42, "widget", "in-progress", "claimed_at: 2026-08-01T10:00:00Z"))},
			code:     CodeChangeStateIncoherent,
			severity: domain.SeverityError,
			field:    "branch",
			entity:   changeEntity(42, "widget", activePath),
		},
		{
			name: "in-progress with an unreadable claim stamp",
			docs: []InputDocument{record(t, KindChange, LocationActive, activePath,
				minimalChange(42, "widget", "in-progress", "branch: feat/widget", "claimed_at: yesterday"))},
			code:     CodeChangeStateIncoherent,
			severity: domain.SeverityError,
			field:    "claimed_at",
			entity:   changeEntity(42, "widget", activePath),
		},
		{
			name:     "blocked without a reason",
			docs:     []InputDocument{record(t, KindChange, LocationActive, activePath, minimalChange(42, "widget", "blocked"))},
			code:     CodeChangeStateIncoherent,
			severity: domain.SeverityError,
			field:    "blocked_by",
			entity:   changeEntity(42, "widget", activePath),
		},
		{
			name: "implemented without a pull request",
			docs: []InputDocument{record(t, KindChange, LocationActive, activePath,
				minimalChange(42, "widget", "implemented", "branch: feat/widget",
					"claimed_at: 2026-08-01T10:00:00Z", "plan: docs/plans/p.md", "reconciled: true"))},
			code:     CodeChangeStateIncoherent,
			severity: domain.SeverityError,
			field:    "pr",
			entity:   changeEntity(42, "widget", activePath),
		},
		{
			name: "implemented without reconciliation",
			docs: []InputDocument{record(t, KindChange, LocationActive, activePath,
				minimalChange(42, "widget", "implemented", "branch: feat/widget",
					"claimed_at: 2026-08-01T10:00:00Z", "plan: docs/plans/p.md", "pr: 7"))},
			code:     CodeChangeStateIncoherent,
			severity: domain.SeverityError,
			field:    "reconciled",
			entity:   changeEntity(42, "widget", activePath),
		},
		{
			name: "stacked-merged without a stack parent",
			docs: []InputDocument{record(t, KindChange, LocationActive, activePath,
				minimalChange(42, "widget", "stacked-merged", "branch: feat/widget",
					"claimed_at: 2026-08-01T10:00:00Z", "plan: docs/plans/p.md", "pr: 7", "reconciled: true"))},
			code:     CodeChangeStateIncoherent,
			severity: domain.SeverityError,
			field:    "stacked_on",
			entity:   changeEntity(42, "widget", activePath),
		},
		{
			name:   "proposed record with no claim key at all is coherent",
			docs:   []InputDocument{record(t, KindChange, LocationActive, activePath, minimalChange(42, "widget", "proposed"))},
			code:   CodeChangeStateIncoherent,
			absent: true,
		},
		{
			name: "depends_on names no supplied change",
			docs: []InputDocument{record(t, KindChange, LocationActive, activePath,
				minimalChange(42, "widget", "proposed", "depends_on: [41]"))},
			code:     CodeChangeReferenceDangling,
			severity: domain.SeverityError,
			field:    "depends_on",
			entity:   changeEntity(42, "widget", activePath),
		},
		{
			name: "stacked_on names no supplied change",
			docs: []InputDocument{record(t, KindChange, LocationActive, activePath,
				minimalChange(42, "widget", "proposed", "stacked_on: 41"))},
			code:     CodeChangeReferenceDangling,
			severity: domain.SeverityError,
			field:    "stacked_on",
			entity:   changeEntity(42, "widget", activePath),
		},
		{
			name: "related names no supplied change — associative, so a warning",
			docs: []InputDocument{record(t, KindChange, LocationActive, activePath,
				minimalChange(42, "widget", "proposed", "related: [41]"))},
			code:     CodeChangeReferenceDangling,
			severity: domain.SeverityWarning,
			field:    "related",
			entity:   changeEntity(42, "widget", activePath),
		},
		{
			name: "discovered_from names no supplied change",
			docs: []InputDocument{record(t, KindChange, LocationActive, activePath,
				minimalChange(42, "widget", "proposed", "discovered_from: [41]"))},
			code:     CodeChangeReferenceDangling,
			severity: domain.SeverityWarning,
			field:    "discovered_from",
			entity:   changeEntity(42, "widget", activePath),
		},
		{
			name: "adrs names no supplied ADR",
			docs: []InputDocument{record(t, KindChange, LocationActive, activePath,
				minimalChange(42, "widget", "proposed", "adrs: [9]"))},
			code:     CodeChangeReferenceDangling,
			severity: domain.SeverityWarning,
			field:    "adrs",
			entity:   changeEntity(42, "widget", activePath),
		},
		{
			name: "a resolved dependency is not a finding",
			docs: []InputDocument{
				record(t, KindChange, LocationActive, activePath, minimalChange(42, "widget", "proposed", "depends_on: [41]")),
				record(t, KindChange, LocationActive, "docs/changes/active/0041-gadget.md", minimalChange(41, "gadget", "proposed")),
			},
			code:   CodeChangeReferenceDangling,
			absent: true,
		},
		{
			name: "dependency cycle names every member",
			docs: []InputDocument{
				record(t, KindChange, LocationActive, activePath, minimalChange(42, "widget", "proposed", "depends_on: [41]")),
				record(t, KindChange, LocationActive, "docs/changes/active/0041-gadget.md", minimalChange(41, "gadget", "proposed", "depends_on: [42]")),
			},
			code:     CodeChangeDependencyCycle,
			severity: domain.SeverityError,
			field:    "depends_on",
			entity:   domain.EntityRef{Kind: domain.EntityChange, ID: 41},
		},
		{
			name: "stack cycle names every member",
			docs: []InputDocument{
				record(t, KindChange, LocationActive, activePath, minimalChange(42, "widget", "proposed", "stacked_on: 41")),
				record(t, KindChange, LocationActive, "docs/changes/active/0041-gadget.md", minimalChange(41, "gadget", "proposed", "stacked_on: 42")),
			},
			code:     CodeChangeStackCycle,
			severity: domain.SeverityError,
			field:    "stacked_on",
			entity:   domain.EntityRef{Kind: domain.EntityChange, ID: 41},
		},
		{
			name: "ADR graph findings are merged in",
			docs: []InputDocument{record(t, KindADR, LocationLedger, "docs/adrs/0007-choice.md",
				minimalADR(7, "choice", "Superseded by ADR-0009", "supersedes: []", "reverses: []", "relates_to: []"))},
			code:     domain.CodeADRDanglingReference,
			severity: domain.SeverityError,
			field:    "status",
			entity:   domain.EntityRef{Kind: domain.EntityADR, ID: 7},
		},
		{
			name: "ADR filename disagrees with its identity",
			docs: []InputDocument{record(t, KindADR, LocationLedger, "docs/adrs/0008-choice.md",
				minimalADR(7, "choice", "Accepted"))},
			code:     CodeADRFilenameMismatch,
			severity: domain.SeverityError,
			field:    "path",
			entity:   domain.EntityRef{Kind: domain.EntityADR, ID: 7, Slug: "choice", Path: "docs/adrs/0008-choice.md"},
		},
		{
			name: "learning topic outside the token grammar",
			docs: []InputDocument{record(t, KindLearning, LocationLedger, "docs/changes/learnings/a-lesson.md",
				strings.Replace(minimalLearning("a-lesson"), "topics: [process]", "topics: ['Not A Topic']", 1))},
			code:     CodeLearningTopicInvalid,
			severity: domain.SeverityError,
			field:    "topics",
			entity:   domain.EntityRef{Kind: domain.EntityLearning, Slug: "a-lesson", Path: "docs/changes/learnings/a-lesson.md"},
		},
		{
			name: "promoted learning naming no destination",
			docs: []InputDocument{record(t, KindLearning, LocationLedger, "docs/changes/learnings/a-lesson.md",
				strings.Replace(minimalLearning("a-lesson"), "promotion_state: retained", "promotion_state: promoted", 1))},
			code:     CodeLearningPromotionDestination,
			severity: domain.SeverityError,
			field:    "promoted_to",
			entity:   domain.EntityRef{Kind: domain.EntityLearning, Slug: "a-lesson", Path: "docs/changes/learnings/a-lesson.md"},
		},
		{
			name: "learning filename disagrees with its slug",
			docs: []InputDocument{record(t, KindLearning, LocationLedger, "docs/changes/learnings/other.md",
				minimalLearning("a-lesson"))},
			code:     CodeLearningFilenameMismatch,
			severity: domain.SeverityError,
			field:    "path",
			entity:   domain.EntityRef{Kind: domain.EntityLearning, Slug: "a-lesson", Path: "docs/changes/learnings/other.md"},
		},
		{
			name: "learning changes reference no supplied change",
			docs: []InputDocument{record(t, KindLearning, LocationLedger, "docs/changes/learnings/a-lesson.md",
				strings.Replace(minimalLearning("a-lesson"), "changes: []", "changes: [12]", 1))},
			code:     CodeLearningReferenceDangling,
			severity: domain.SeverityWarning,
			field:    "changes",
			entity:   domain.EntityRef{Kind: domain.EntityLearning, Slug: "a-lesson", Path: "docs/changes/learnings/a-lesson.md"},
		},
		{
			name: "artifact reference supplied as a derived view",
			docs: []InputDocument{
				record(t, KindChange, LocationActive, activePath,
					minimalChange(42, "widget", "proposed", "spec: docs/changes/BOARD.md")),
				record(t, KindDerived, LocationDerived, "docs/changes/BOARD.md", "# Board\n"),
			},
			code:     CodeArtifactReferenceKindMismatch,
			severity: domain.SeverityError,
			field:    "spec",
			entity:   changeEntity(42, "widget", activePath),
		},
		{
			name: "artifact reference nobody supplied is not a finding",
			docs: []InputDocument{record(t, KindChange, LocationActive, activePath,
				minimalChange(42, "widget", "proposed", "spec: docs/specs/design.md"))},
			code:   CodeArtifactReferenceKindMismatch,
			absent: true,
		},
		{
			name: "a supplied artifact nothing references is accounted, not flagged",
			docs: []InputDocument{
				record(t, KindChange, LocationActive, activePath, minimalChange(42, "widget", "proposed")),
				record(t, KindArtifact, LocationArtifact, "docs/specs/design.md", "# Design\n"),
			},
			code:   CodeArtifactReferenceKindMismatch,
			absent: true,
		},
		{
			name: "two changes claiming one slug",
			docs: []InputDocument{
				record(t, KindChange, LocationActive, activePath, minimalChange(42, "widget", "proposed")),
				record(t, KindChange, LocationActive, "docs/changes/active/0041-widget.md", minimalChange(41, "widget", "proposed")),
			},
			code:     CodeChangeSlugDuplicate,
			severity: domain.SeverityError,
			field:    "slug",
			entity:   changeEntity(42, "widget", activePath),
		},
		{
			name: "two ADRs claiming one id",
			docs: []InputDocument{
				record(t, KindADR, LocationLedger, "docs/adrs/0007-choice.md", minimalADR(7, "choice", "Accepted")),
				record(t, KindADR, LocationLedger, "docs/adrs/0007-other.md", minimalADR(7, "other", "Accepted")),
			},
			code:     CodeADRIDDuplicate,
			severity: domain.SeverityError,
			field:    "id",
			entity:   domain.EntityRef{Kind: domain.EntityADR, ID: 7, Slug: "choice", Path: "docs/adrs/0007-choice.md"},
		},
		{
			name: "two learnings claiming one slug",
			docs: []InputDocument{
				record(t, KindLearning, LocationLedger, "docs/changes/learnings/a-lesson.md", minimalLearning("a-lesson")),
				record(t, KindLearning, LocationLedger, "docs/changes/learnings/copy.md", minimalLearning("a-lesson")),
			},
			code:     CodeLearningSlugDuplicate,
			severity: domain.SeverityError,
			field:    "slug",
			entity:   domain.EntityRef{Kind: domain.EntityLearning, Slug: "a-lesson", Path: "docs/changes/learnings/a-lesson.md"},
		},
		{
			name: "duplicate paths are reported once per claimant",
			docs: []InputDocument{
				record(t, KindChange, LocationActive, activePath, minimalChange(42, "widget", "proposed")),
				record(t, KindChange, LocationActive, activePath, minimalChange(42, "widget", "proposed")),
			},
			code:     CodeRecordPathDuplicate,
			severity: domain.SeverityError,
			field:    "path",
			entity:   domain.EntityRef{Kind: domain.EntityChange, Path: activePath},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := buildRecords(t, tc.docs...).Report.Findings()
			if tc.absent {
				for _, f := range findings {
					if f.Code == tc.code {
						t.Fatalf("unexpected %s on %+v", f.Code, f.Entity)
					}
				}
				return
			}
			for _, f := range findings {
				if f.Code == tc.code && f.Field == tc.field && f.Entity == tc.entity {
					if f.Severity != tc.severity {
						t.Fatalf("severity = %q, want %q", f.Severity, tc.severity)
					}
					return
				}
			}
			t.Fatalf("no %s/%s finding on %+v; got %v", tc.code, tc.field, tc.entity, findings)
		})
	}
}

func TestValidateCycleFindingNamesEveryMember(t *testing.T) {
	result := buildRecords(t,
		record(t, KindChange, LocationActive, "docs/changes/active/0042-widget.md",
			minimalChange(42, "widget", "proposed", "depends_on: [41]")),
		record(t, KindChange, LocationActive, "docs/changes/active/0041-gadget.md",
			minimalChange(41, "gadget", "proposed", "depends_on: [40]")),
		record(t, KindChange, LocationActive, "docs/changes/active/0040-sprocket.md",
			minimalChange(40, "sprocket", "proposed", "depends_on: [42]")),
	)
	for _, f := range result.Report.Findings() {
		if f.Code != CodeChangeDependencyCycle {
			continue
		}
		if got := f.Detail["members"]; got != "0040,0042,0041" {
			t.Errorf("members detail = %q", got)
		}
		if len(f.Related) != 3 {
			t.Errorf("Related = %v, want three members", f.Related)
		}
		return
	}
	t.Fatalf("no %s finding: %v", CodeChangeDependencyCycle, findingCodes(result.Report.Findings()))
}

func TestValidateReportIsDeterministic(t *testing.T) {
	docs := []InputDocument{
		record(t, KindChange, LocationActive, "docs/changes/active/0042-widget.md",
			minimalChange(42, "widget", "shipped", "related: [41]")),
		record(t, KindADR, LocationLedger, "docs/adrs/0007-choice.md",
			minimalADR(7, "choice", "Superseded by ADR-0009")),
	}
	first := buildRecords(t, docs...).Report.Findings()
	second := buildRecords(t, docs[1], docs[0]).Report.Findings()
	if len(first) != len(second) {
		t.Fatalf("report lengths differ: %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i].Code != second[i].Code || first[i].Entity != second[i].Entity {
			t.Fatalf("report order differs at %d: %v vs %v", i, first[i], second[i])
		}
	}
}
