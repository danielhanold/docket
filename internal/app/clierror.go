package app

// Stable machine reasons for CLI-level failures. Message is explanatory prose
// and must not be parsed; the framework's error text may improve freely.
const (
	ReasonInvalidArguments = "invalid-arguments"
	ReasonJSONHelpConflict = "json-help-conflict"
)

// CLIErrorResult is the stable shape for CLI parsing and usage failures.
type CLIErrorResult struct {
	Envelope
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

// CLIError builds an invalid-input result under the "cli" operation name.
func CLIError(reason, message string) CLIErrorResult {
	return CLIErrorResult{
		Envelope: NewEnvelope("cli", ResultInvalidInput),
		Reason:   reason,
		Message:  message,
	}
}

// HumanText renders the human-mode diagnostic line (routed to stderr by the
// presenter; stdout stays empty on human parse failures).
func (r CLIErrorResult) HumanText() string { return "docket: " + r.Message }
