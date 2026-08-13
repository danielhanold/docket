// Package app owns application results: the protocol-v1 envelope, the result
// taxonomy, exit mapping, and the read-only operations of change 0304. It has
// no dependency on Cobra or any presentation concern beyond HumanText.
package app

// ProtocolVersion is fixed at 1 for this protocol generation. JSON field
// names and types are protocol: removing, renaming, or retyping a field
// requires a later protocol version; adding operation-specific fields is
// compatible within v1.
const ProtocolVersion = 1

// Result is one spelling from the protocol-v1 result taxonomy.
type Result string

const (
	ResultApplied           Result = "applied"
	ResultNoOp              Result = "no-op"
	ResultContended         Result = "contended"
	ResultInvalidInput      Result = "invalid-input"
	ResultInvalidState      Result = "invalid-state"
	ResultBlocked           Result = "blocked"
	ResultUnsupportedConfig Result = "unsupported-config"
	ResultGateFailed        Result = "gate-failed"
	ResultExternalFailed    Result = "external-failed"
	ResultInterrupted       Result = "interrupted"
	ResultInternalError     Result = "internal-error"
)

// AllResults enumerates the complete v1 taxonomy, in documentation order.
var AllResults = []Result{
	ResultApplied, ResultNoOp, ResultContended, ResultInvalidInput,
	ResultInvalidState, ResultBlocked, ResultUnsupportedConfig,
	ResultGateFailed, ResultExternalFailed, ResultInterrupted,
	ResultInternalError,
}

// Envelope carries the three fields every protocol-v1 result begins with.
// Operation-specific result structs embed it; reserved envelope names cannot
// be shadowed by an operation's own fields.
type Envelope struct {
	ProtocolVersion int    `json:"protocol_version"`
	Operation       string `json:"operation"`
	Result          Result `json:"result"`
}

// NewEnvelope builds the envelope for one operation outcome.
func NewEnvelope(operation string, result Result) Envelope {
	return Envelope{ProtocolVersion: ProtocolVersion, Operation: operation, Result: result}
}

// Env returns the envelope; embedding gives every result struct this method.
func (e Envelope) Env() Envelope { return e }

// OperationResult is a fully-computed operation outcome the presenter can
// render as protocol JSON or as human text.
type OperationResult interface {
	Env() Envelope
	HumanText() string
}

// ExitCode maps a result to the deliberately coarse process exit status.
// JSON consumers use result, not the exit code.
func ExitCode(r Result) int {
	switch r {
	case ResultApplied, ResultNoOp:
		return 0
	case ResultInvalidInput:
		return 2
	default:
		return 1
	}
}
