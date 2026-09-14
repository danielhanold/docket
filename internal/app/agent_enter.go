package app

const OperationAgentEnter = "agent.enter"

// AgentEnterResult is the foreground receipt for one coordinator root turn.
// Human mode relays the role's final message verbatim; JSON mode retains thread
// and turn identities for diagnostics.
//
// Run-gate claim proofs own attribution. The optional `--run-gate-key`/`--run-epoch`
// lifecycle-linkage flags (change 0375 Task 13) register this entry's thread as a
// run-epoch participant and — for a root coordinator — wire the signal-connected
// cancellation and the death guardian; they are lifecycle REGISTRATION only and
// confer no attribution and no authority. ThreadID/TurnID remain diagnostics, not
// authority.
type AgentEnterResult struct {
	Envelope
	Role     string `json:"role,omitempty"`
	ThreadID string `json:"thread_id,omitempty"`
	TurnID   string `json:"turn_id,omitempty"`
	Output   string `json:"output,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Message  string `json:"message,omitempty"`
}

func (r AgentEnterResult) HumanText() string {
	if r.Result == ResultApplied {
		return r.Output
	}
	if r.Message != "" {
		return "agent enter: " + r.Message
	}
	return "agent enter: " + string(r.Result)
}
