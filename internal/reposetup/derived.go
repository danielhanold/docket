package reposetup

// derived.go — the closed derived-view drift vocabulary.
//
// A derived view is a file whose bytes are a pure function of the metadata
// corpus: the inline board (docs/changes/BOARD.md), the managed artifact-link
// block inside each change record, and the ADR index (docs/adrs/README.md).
// `repository check` renders each view's canonical bytes from the pinned corpus
// snapshot and byte-compares them against the stored file; a difference is a
// drift finding here. `repository migrate` repairs exactly the deterministic
// (Repairable) ones by recomputing the canonical bytes — it never edits authored
// content and never rewrites an unbalanced managed block.
//
// The split is by decidability, mirroring the frontmatter repair roster:
//   - a deterministic byte difference against a well-formed file is Repairable;
//   - a malformed managed marker (an unbalanced/dangling/out-of-order block),
//     which the automatic rewrite must never touch (AGENTS.md marker rule), and a
//     missing referenced artifact are NOT repairable — a human resolves them.

// DerivedView names one of the three canonical derived views a drift finding is
// about. It is a stable machine token carried in the finding.
type DerivedView string

const (
	DerivedViewBoard         DerivedView = "board"
	DerivedViewArtifactLinks DerivedView = "artifact-links"
	DerivedViewADRIndex      DerivedView = "adr-index"
)

// Derived-view finding codes — the closed extension to the check vocabulary.
// The *-stale / *-missing codes are deterministic byte differences and are
// mechanically repairable; the *-malformed codes name an unbalanced managed
// marker the automatic rewrite must never touch.
const (
	CodeBoardStale             = "board-stale"
	CodeBoardMalformed         = "board-malformed"
	CodeArtifactLinksStale     = "artifact-links-stale"
	CodeArtifactLinksMissing   = "artifact-links-missing"
	CodeArtifactLinksMalformed = "artifact-links-malformed"
	CodeADRIndexStale          = "adr-index-stale"
	CodeADRIndexMalformed      = "adr-index-malformed"
	// CodeCorpusUnreadable names a corpus read failure: the check could not read
	// the metadata corpus, so it reports an error rather than fabricating a clean
	// absence (learning probe-error-is-not-clean-absence).
	CodeCorpusUnreadable = "corpus-unreadable"
)

// DerivedFinding is one derived-view drift diagnosis about a single corpus file.
// A deterministic byte difference against a well-formed file is Repairable; a
// malformed managed marker or a missing referenced artifact is not.
type DerivedFinding struct {
	View       DerivedView
	Code       string
	Path       string // repo-relative corpus file the finding is about
	Repairable bool
	Message    string
}

// Finding lifts a derived-view drift diagnosis into a health Finding. A
// repairable difference is a warning naming the mechanical repair; a
// non-repairable one (a malformed marker, a missing referenced artifact) is an
// error naming manual review. The Repairable flag survives into the JSON so a
// consumer can tell a mechanical rewrite apart from a manual-review finding.
func (df DerivedFinding) Finding() Finding {
	repairable := df.Repairable
	if df.Repairable {
		return Finding{
			Code:       df.Code,
			Severity:   SeverityWarning,
			Ref:        df.Path,
			Message:    df.Message,
			Remedy:     "Run `docket repository migrate` to recompute the canonical " + string(df.View) + " output, or edit the file by hand.",
			Repairable: &repairable,
		}
	}
	return Finding{
		Code:       df.Code,
		Severity:   SeverityError,
		Ref:        df.Path,
		Message:    df.Message,
		Remedy:     "Resolve the " + string(df.View) + " condition by hand; the automatic repair will not touch it.",
		Repairable: &repairable,
	}
}
