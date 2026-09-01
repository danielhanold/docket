package cli

import (
	"strings"
	"testing"
)

func TestAgentEnterCommandRegistered(t *testing.T) {
	root := captureTree(t)
	cmd, _, err := root.Find([]string{"agent", "enter"})
	if err != nil || cmd == nil || cmd.Name() != "enter" {
		t.Fatalf("agent enter not registered: cmd=%v err=%v", cmd, err)
	}
	for _, flag := range []string{"role", "request", "cwd", "approval-policy", "sandbox"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("agent enter: missing --%s flag", flag)
		}
	}
	for _, key := range []string{"agent", "agent enter"} {
		if !assetIndependent[key] {
			t.Errorf("%q must resolve its role contract from embedded compatible assets", key)
		}
	}
}

func TestAgentEnterRequiresClosedExecutionContext(t *testing.T) {
	if _, errS, code := runCLI(t, "agent", "enter"); code != 2 || !strings.Contains(errS, "required") {
		t.Fatalf("missing flags: stderr=%q code=%d", errS, code)
	}

	dir := t.TempDir()
	base := []string{"agent", "enter", "--role", "docket-implement-next", "--request", "-", "--cwd", dir}
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"unknown approval", []string{"--approval-policy", "sometimes", "--sandbox", "workspace-write"}, "approval policy"},
		{"unknown sandbox", []string{"--approval-policy", "never", "--sandbox", "host-root"}, "sandbox mode"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := append(append([]string(nil), base...), tc.args...)
			_, errS, code := runCLI(t, args...)
			if code != 2 || !strings.Contains(errS, tc.want) {
				t.Fatalf("stderr=%q code=%d, want %q", errS, code, tc.want)
			}
		})
	}
}
