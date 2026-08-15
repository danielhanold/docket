package transaction

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"

	"github.com/danielhanold/docket/internal/domain"
	"github.com/danielhanold/docket/internal/gitcli"
)

// Disposition is the terminal outcome an Execute reports to the caller.
type Disposition string

// The closed set of dispositions.
const (
	DispositionApplied        Disposition = "applied"
	DispositionAlreadyApplied Disposition = "already-applied"
	DispositionNoOp           Disposition = "no-op"
	DispositionContended      Disposition = "contended"
	DispositionRefused        Disposition = "refused"
	DispositionFailed         Disposition = "failed"
	DispositionInterrupted    Disposition = "interrupted"
)

// Result is the outcome of one Execute call. Only fields relevant to the
// reported disposition are populated. Byte payloads are never leaked here:
// ContendedPaths carries paths only, and Receipt is the decoded, already
// validated receipt.
type Result struct {
	Disposition     Disposition
	Operation       OperationKey
	RequestID       string          // empty when no key
	BaseCommit      gitcli.ObjectID // last fetched base, when known
	RemoteCommit    gitcli.ObjectID // last observed remote target, when known
	AppliedCommit   gitcli.ObjectID // on applied / already-applied
	Attempts        int
	Receipt         []byte            // decoded validated receipt on applied/already-applied
	ContendedPaths  []gitcli.RepoPath // paths only, never bytes
	Findings        []domain.Finding  // refusal / validation diagnostics
	CleanupWarnings []string          // e.g. "cleanup-pending: <transaction-id>"
}

// Stage names the engine phase a failure was raised in.
type Stage string

// The closed set of engine stages.
const (
	StageValidateRequest Stage = "validate-request"
	StageFetch           Stage = "fetch"
	StageIdempotencyScan Stage = "idempotency-scan"
	StageAllocate        Stage = "allocate"
	StageWorktree        Stage = "worktree"
	StageLoadBefore      Stage = "load-before"
	StageExpectations    Stage = "expectations"
	StagePlan            Stage = "plan"
	StageLoadAfter       Stage = "load-after"
	StageMaterialize     Stage = "materialize"
	StageVerifyDelta     Stage = "verify-delta"
	StageCommit          Stage = "commit"
	StagePush            Stage = "push"
	StageProbe           Stage = "probe"
	StageCleanup         Stage = "cleanup"
	StageRecovery        Stage = "recovery"
)

// Kind classifies why a failure occurred.
type Kind string

// The closed set of failure kinds.
const (
	KindInvalidInput  Kind = "invalid-input"  // bad request/plan/receipt/key
	KindInvalidState  Kind = "invalid-state"  // repo/history contradicts engine invariants
	KindValidation    Kind = "validation"     // before/after/evolution error findings
	KindExternal      Kind = "external"       // git/transport/auth/identity failures
	KindCancelled     Kind = "cancelled"      // context cancellation / deadline
	KindUnknownResult Kind = "unknown-result" // push outcome not establishable
)

// Failure is the engine's typed error. It carries the stage and kind, a bounded
// redacted detail string, and an optional wrapped cause.
type Failure struct {
	Stage  Stage
	Kind   Kind
	Detail string // bounded, redacted
	Err    error  // wrapped cause, may be nil
}

// Error renders the failure without leaking anything beyond Detail and the
// wrapped cause's own message.
func (f *Failure) Error() string {
	var b strings.Builder
	b.WriteString("transaction: ")
	b.WriteString(string(f.Stage))
	b.WriteByte('/')
	b.WriteString(string(f.Kind))
	if f.Detail != "" {
		b.WriteString(": ")
		b.WriteString(f.Detail)
	}
	if f.Err != nil {
		b.WriteString(": ")
		b.WriteString(f.Err.Error())
	}
	return b.String()
}

// Unwrap exposes the wrapped cause for errors.Is / errors.As.
func (f *Failure) Unwrap() error { return f.Err }

// AsFailure reports whether err is, or wraps, a *Failure.
func AsFailure(err error) (*Failure, bool) {
	var f *Failure
	if errors.As(err, &f) {
		return f, true
	}
	return nil, false
}

// maxReceiptBytes bounds a receipt's canonical JSON size.
const maxReceiptBytes = 4096

// validateReceipt requires a compact, canonical JSON object of at most 4096
// bytes. Canonical means: the bytes are already compact (no insignificant
// whitespace) and re-marshalling the decoded value with encoding/json yields
// the exact same bytes (sorted keys, canonical scalars). The top value must be
// a JSON object.
func validateReceipt(b []byte) error {
	if len(b) == 0 {
		return errors.New("transaction: empty receipt")
	}
	if len(b) > maxReceiptBytes {
		return errors.New("transaction: receipt exceeds 4096 bytes")
	}
	if b[0] != '{' {
		return errors.New("transaction: receipt is not a JSON object")
	}
	if !json.Valid(b) {
		return errors.New("transaction: receipt is not valid JSON")
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, b); err != nil {
		return errors.New("transaction: receipt is not compactable JSON")
	}
	if !bytes.Equal(compact.Bytes(), b) {
		return errors.New("transaction: receipt carries insignificant whitespace")
	}
	var decoded any
	if err := json.Unmarshal(b, &decoded); err != nil {
		return errors.New("transaction: receipt does not decode")
	}
	remarshalled, err := json.Marshal(decoded)
	if err != nil {
		return errors.New("transaction: receipt does not re-marshal")
	}
	if !bytes.Equal(remarshalled, b) {
		return errors.New("transaction: receipt is not canonical")
	}
	return nil
}
