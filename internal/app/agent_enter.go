package app

const OperationAgentEnter = "agent.enter"

// AgentEnterResult is the foreground receipt for one coordinator root turn.
// Human mode relays the role's final message verbatim; JSON mode retains the
// identities needed to attribute it.
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
