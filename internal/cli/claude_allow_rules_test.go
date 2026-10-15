package cli

import (
	"reflect"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/harness"
)

// TestClaudeFinalizeAllowRulesMatchCatalog pins each Claude Code allow rule to
// Bash( + its operation's space-joined catalog argv + :*), so renaming either
// command reddens here instead of leaving a rule that matches nothing, and pins
// that the Claude target carries exactly these rules in this order.
func TestClaudeFinalizeAllowRulesMatchCatalog(t *testing.T) {
	entries, err := collectCapabilities(productionRootForTest(t))
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	for _, id := range []string{"finalize.publish", "finalize.merge"} {
		e, ok := entryByID(entries, id)
		if !ok || len(e.Argv) == 0 {
			t.Fatalf("%s is not in the capability catalog with an argv", id)
		}
		want = append(want, "Bash("+strings.Join(e.Argv, " ")+":*)")
	}
	if got := []string{harness.ClaudeFinalizePublishAllowRule, harness.ClaudeFinalizeMergeAllowRule}; !reflect.DeepEqual(got, want) {
		t.Errorf("allow rules = %q, want %q", got, want)
	}
	if got := harness.ClaudeHookTarget("/h/.claude/settings.json").AllowRules; !reflect.DeepEqual(got, want) {
		t.Errorf("ClaudeHookTarget allow rules = %q, want %q", got, want)
	}
}
