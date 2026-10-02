package process

import "testing"

// TestStateSupervisorExited pins the shared teardown predicate: every settled
// state proves the supervisor gone, while running, an empty state, and an
// unrecognized state prove nothing (they must never read as gone).
func TestStateSupervisorExited(t *testing.T) {
	cases := map[State]bool{
		StatePassed:    true,
		StateFailed:    true,
		StateSignaled:  true,
		StateStopped:   true,
		StateVanished:  true,
		StateRunning:   false,
		"":             false,
		"establishing": false,
	}
	for st, want := range cases {
		if got := st.SupervisorExited(); got != want {
			t.Errorf("State(%q).SupervisorExited() = %v, want %v", st, got, want)
		}
	}
}
