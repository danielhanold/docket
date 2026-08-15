package transaction

import (
	"context"
	"time"

	"github.com/danielhanold/docket/internal/document"
	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
)

// LoadedState is the complete decoded, validated view of one tree the engine
// works from — the base before an operation runs, and the overlay after. It
// pairs the immutable snapshot and its validation report with the per-path
// document views and the exact source bytes an evolution check reads.
type LoadedState struct {
	Snapshot  domain.Snapshot
	Report    domain.ValidationReport
	Documents map[string]document.Document // keyed by repo-relative path; defensively copied views
	Sources   map[string][]byte            // exact bytes per path (evolution input)
}

// StateLoader turns a read-only Tree into a complete LoadedState and compares
// two states for illegal evolution. It is caller-supplied so the engine never
// embeds a second production composer; tests back it with document.Parse and
// repository.BuildSnapshot.
type StateLoader interface {
	// Load reads and validates the complete state visible through t. A programmer
	// or I/O failure is an error; a domain-invalid corpus is reported through the
	// returned LoadedState.Report, not an error.
	Load(ctx context.Context, t Tree) (LoadedState, error)
	// ValidateEvolution returns findings for changes between before and after
	// that violate evolution rules (e.g. rewriting a frozen record).
	ValidateEvolution(before, after LoadedState) []domain.Finding
}

// AttemptState is the per-attempt context handed to a semantic operation: the
// base revision this attempt fetched, the validated state loaded from it, and
// the read-only tree the operation may inspect.
type AttemptState struct {
	Base  gitcli.Revision
	State LoadedState
	Tree  Tree
}

// OperationResult carries a semantic operation's non-plan outcome. A refusal is
// a domain decision — not an error — and populates Findings.
type OperationResult struct {
	Refused  bool
	Findings []domain.Finding // populated on refusal
}

// SemanticOperation computes the closed MutationPlan for one attempt's state, or
// refuses. The error return is reserved for programmer or loader failures and is
// never a domain outcome; a refusal is reported through OperationResult.
type SemanticOperation interface {
	Key() OperationKey
	Plan(ctx context.Context, st AttemptState) (MutationPlan, OperationResult, error)
}

// Clock is the engine's only time source; tests pin it. Semantic operations
// never read the wall clock.
type Clock interface{ Now() time.Time }
