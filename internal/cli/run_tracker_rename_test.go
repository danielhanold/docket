package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

// TestRunTrackerVocabularyHardCut pins ADR-0129 Decision 2 for family (a)
// (change 0471): the renamed verbs and flags exist and the retired spellings are
// gone. No alias, no hidden flag. It also pins change 0477's row 38e: the gate
// drive's --gate-context became --run-context, and the old flag is unknown.
func TestRunTrackerVocabularyHardCut(t *testing.T) {
	root := captureTree(t)
	find := func(path ...string) *cobra.Command {
		t.Helper()
		cmd, _, err := root.Find(path)
		if err != nil {
			t.Fatalf("find %v: %v", path, err)
		}
		return cmd
	}
	names := map[string]bool{}
	for _, c := range find("run").Commands() {
		names[c.Name()] = true
		for _, a := range c.Aliases {
			names[a] = true
		}
	}
	for _, want := range []string{"start", "verdict", "continue", "cancel", "verify"} {
		if !names[want] {
			t.Errorf("docket run %s is missing", want)
		}
	}
	for _, gone := range []string{"gate-before", "gate-verdict", "gate-claim"} {
		if names[gone] {
			t.Errorf("docket run %s survives the hard cut", gone)
		}
	}
	for _, f := range []struct {
		path       []string
		want, gone string
	}{
		{[]string{"run", "cancel"}, "run-id", "epoch"},
		{[]string{"change", "claim"}, "run-context", "gate-context"},
		{[]string{"agent", "enter"}, "run-key", "run-gate-key"},
		{[]string{"agent", "enter"}, "run-id", "run-epoch"},
		{[]string{"gate", "drive", "start"}, "run-id", "run-epoch"},
		{[]string{"gate", "drive", "prepare-scope"}, "run-id", "run-epoch"},
		// change 0477, ADR-0129 row 38e: the gate drive takes the same
		// --run-context as change claim; --gate-context is gone everywhere.
		{[]string{"gate", "drive", "start"}, "run-context", "gate-context"},
		{[]string{"gate", "drive", "prepare-scope"}, "run-context", "gate-context"},
	} {
		cmd := find(f.path...)
		if cmd.Flags().Lookup(f.want) == nil {
			t.Errorf("%v lacks --%s", f.path, f.want)
		}
		if cmd.Flags().Lookup(f.gone) != nil {
			t.Errorf("%v still registers the retired --%s", f.path, f.gone)
		}
	}
	// An old spelling is refused at the command line (exit 2), not ignored.
	if _, _, code := runCLI(t, "--json", "run", "gate-before", "implement-next"); code != 2 {
		t.Errorf("docket run gate-before exited %d, want 2 (unknown command)", code)
	}
	if _, _, code := runCLI(t, "--json", "run", "cancel", "--key", "k", "--epoch", "e", "--reason", "r"); code != 2 {
		t.Errorf("docket run cancel --epoch exited %d, want 2 (unknown flag)", code)
	}
	// change 0477: an in-flight caller still passing the gate drive's old flag
	// fails loudly as an unknown flag (exit 2), never silently ignored.
	if _, _, code := runCLI(t, "--json", "gate", "drive", "prepare-scope",
		"--change-id", "1", "--task-id", "t", "--phase", "build", "--branch", "b",
		"--worktree", "/tmp", "--gate-context", "x"); code != 2 {
		t.Errorf("docket gate drive prepare-scope --gate-context exited %d, want 2 (unknown flag)", code)
	}
}
