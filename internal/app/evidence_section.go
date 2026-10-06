package app

import (
	"fmt"
	"strings"

	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/evidence"
	"github.com/danielhanold/docket/internal/render"
)

// This file owns the change record's operation-owned "## Build evidence"
// section: the durable home of a change's build evidence. The section body is
// exactly the evidence codec's build-evidence block (evidence.Render), so the
// bytes a reader extracts are the bytes the PR description used to carry. The
// section is never authored: change.groom's owned-heading allowlist
// (render.ChangeOwnedHeadings) does not contain it.

// buildEvidenceHeading is the exact H2 line of the section.
const buildEvidenceHeading = "## Build evidence"

// recordEvidenceSection returns the section body, whether it is present, and an
// error for a duplicated heading. Headings inside fenced code are content
// (namedSectionBody scans fence-aware).
func recordEvidenceSection(record []byte) (string, bool, error) {
	return namedSectionBody(record, buildEvidenceHeading)
}

// ReadRecordEvidence extracts the record's build evidence. An absent section is
// evidence.ErrMissing; a duplicated section or a malformed block is an error.
func ReadRecordEvidence(record []byte) (evidence.Record, error) {
	body, present, err := recordEvidenceSection(record)
	if err != nil {
		return evidence.Record{}, fmt.Errorf("record evidence: %w", err)
	}
	if !present {
		return evidence.Record{}, evidence.ErrMissing
	}
	return evidence.Extract([]byte(body))
}

// VerifyRecordEvidence is evidence.Verify scoped to the record's section: missing
// when absent, malformed when the section is duplicated or its block unreadable.
func VerifyRecordEvidence(record []byte, head string) evidence.Verdict {
	body, present, err := recordEvidenceSection(record)
	if err != nil {
		return evidence.VerdictMalformed
	}
	if !present {
		return evidence.VerdictMissing
	}
	return evidence.Verify([]byte(body), head)
}

// UpsertRecordEvidence makes rec (green or skipped) the whole body of the
// record's "## Build evidence" section, appending the section at EOF when it is
// absent and preserving every other byte. The record is re-normalized through
// the codec constructors, rendered in the record's own line ending, and the
// candidate must read back as exactly rec before any bytes are returned.
func UpsertRecordEvidence(record []byte, rec evidence.Record) ([]byte, error) {
	var norm evidence.Record
	var err error
	switch rec.Result {
	case evidence.ResultGreen:
		norm, err = evidence.NewRecord(rec.Command, rec.Head, rec.RanAt)
	case evidence.ResultSkipped:
		norm, err = evidence.NewSkippedRecord(rec.Head, rec.RanAt)
	default:
		return nil, fmt.Errorf("record evidence: result %q is neither green nor skipped", rec.Result)
	}
	if err != nil {
		return nil, err
	}
	doc, err := document.Parse(record)
	if err != nil {
		return nil, fmt.Errorf("record evidence: malformed record: %w", err)
	}
	block := evidence.Render(norm)
	if le := doc.LineEnding(); le != "" && le != "\n" {
		block = strings.ReplaceAll(block, "\n", le)
	}
	out, err := render.ApplySectionEdits(record, []string{buildEvidenceHeading},
		[]render.SectionEdit{{Heading: buildEvidenceHeading, Intent: render.SectionReplace, Markdown: block}})
	if err != nil {
		return nil, fmt.Errorf("record evidence: %w", err)
	}
	got, err := ReadRecordEvidence(out)
	if err != nil {
		return nil, fmt.Errorf("record evidence: candidate failed reparse: %w", err)
	}
	if got != norm {
		return nil, fmt.Errorf("record evidence: candidate reparsed to a different record")
	}
	return out, nil
}

// recordEvidenceFacts is the record-section reader the finalize gates share: the
// recorded head and command, and whether the record is green. Any read failure
// — absent, duplicated, malformed — reads as no green evidence.
func recordEvidenceFacts(record []byte) (head, command string, green bool) {
	rec, err := ReadRecordEvidence(record)
	if err != nil {
		return "", "", false
	}
	return rec.Head, rec.Command, rec.Result == evidence.ResultGreen
}
