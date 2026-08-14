package repository

import (
	"fmt"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/domain"
)

// BuildResult is one complete read of a repository: the immutable snapshot
// every policy query runs against, and the complete validation report for the
// records that produced it.
type BuildResult struct {
	Snapshot domain.Snapshot
	Report   domain.ValidationReport
}

// BuildSnapshot decodes every supplied document, translates the supported
// configuration leaves into the repository policy domain is allowed to
// consult, runs the complete single-snapshot validation pass, and returns the
// snapshot with its report.
//
// A Go error means the CALL is malformed — an unknown RecordKind token or a
// document supplied without a path — never that the repository is damaged.
// Repository defects are findings: a build over a broken repository still
// returns a snapshot and a report, so one pass can name every repair.
func BuildSnapshot(in BuildInput) (BuildResult, error) {
	for i, doc := range in.Documents {
		switch doc.Kind {
		case KindChange, KindADR, KindLearning, KindArtifact, KindDerived:
		default:
			return BuildResult{}, fmt.Errorf("repository: document %d: unknown record kind %q", i, doc.Kind)
		}
		if doc.Path == "" {
			return BuildResult{}, fmt.Errorf("repository: document %d (%s): empty path", i, doc.Kind)
		}
	}

	spec := domain.SnapshotSpec{Policy: translatePolicy(in.Config)}
	var findings []domain.Finding
	for _, doc := range in.Documents {
		switch doc.Kind {
		case KindChange:
			change, f := decodeChange(doc)
			spec.Changes = append(spec.Changes, change)
			findings = append(findings, f...)
		case KindADR:
			adr, f := decodeADR(doc)
			spec.ADRs = append(spec.ADRs, adr)
			findings = append(findings, f...)
		case KindLearning:
			learning, f := decodeLearning(doc)
			spec.Learnings = append(spec.Learnings, learning)
			findings = append(findings, f...)
		case KindArtifact:
			artifact, f := decodeArtifact(doc)
			spec.Artifacts = append(spec.Artifacts, artifact)
			findings = append(findings, f...)
		case KindDerived:
			view, f := decodeDerived(doc)
			spec.DerivedViews = append(spec.DerivedViews, view)
			findings = append(findings, f...)
		}
	}

	snapshot := domain.NewSnapshot(spec)
	findings = append(findings, validate(snapshot)...)
	for _, unaccounted := range unaccountedPaths(in, snapshot, findings) {
		findings = append(findings, domain.Finding{
			Code:     CodeRecordUnaccounted,
			Severity: domain.SeverityError,
			Entity:   domain.EntityRef{Kind: domain.EntityRepo, Path: unaccounted},
		})
	}

	return BuildResult{Snapshot: snapshot, Report: domain.NewValidationReport(findings)}, nil
}

// translatePolicy copies the supported configuration leaves domain policy may
// consult. Everything else in config.Effective stays outside domain by
// construction: the pure layer cannot reach a setting this function does not
// hand it.
func translatePolicy(cfg config.Effective) domain.RepositoryPolicy {
	return domain.RepositoryPolicy{
		IntegrationBranch: cfg.IntegrationBranch.Value,
		ChangeTypes:       cfg.ChangeTypes.Value,
		ReclaimTTLHours:   cfg.Reclaim.LeaseTTL.Value,
		LearningsEnabled:  cfg.Learnings.Enabled.Value,
	}
}

// unaccountedPaths returns every supplied path that reached neither a record
// in the snapshot nor a finding naming it. A record whose frontmatter could
// not be decoded at all is still accounted for — the decoder keeps it by path
// — so a non-empty result here is a defect in this package, not in the
// repository, and BuildSnapshot reports it rather than dropping the record in
// silence.
func unaccountedPaths(in BuildInput, snap domain.Snapshot, findings []domain.Finding) []string {
	accounted := make(map[string]bool)
	for _, entry := range snapshotPaths(snap) {
		accounted[entry.path] = true
	}
	for _, f := range findings {
		if f.Entity.Path != "" {
			accounted[f.Entity.Path] = true
		}
	}
	var missing []string
	seen := make(map[string]bool)
	for _, doc := range in.Documents {
		if !accounted[doc.Path] && !seen[doc.Path] {
			seen[doc.Path] = true
			missing = append(missing, doc.Path)
		}
	}
	return missing
}
