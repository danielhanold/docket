package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/danielhanold/docket/internal/testsupport"
)

// TestRetiredRevisionRequestKeysRefused pins ADR-0129 Decision 2 for family (b)
// (change 0472): a request file still carrying a retired record-revision key is
// refused by strict decoding as an unknown field, never silently dropped. A
// top-level key's refusal also lists the accepted keys, which name its
// replacement. Each body carries ONLY the retired key, so the refusal can name
// no other field.
func TestRetiredRevisionRequestKeysRefused(t *testing.T) {
	const rev = "0123456789abcdef0123456789abcdef01234567"
	dir := testsupport.TempDir(t)
	for _, c := range []struct {
		name, body, key, replacement string
		args                         []string
	}{
		{"change.block", `{"version":"x"}`, "version", "revision", []string{"change", "block", "--request", "-"}},
		{"change.defer", `{"version":"x"}`, "version", "revision", []string{"change", "defer", "--request", "-"}},
		{"change.kill", `{"version":"x"}`, "version", "revision", []string{"change", "kill", "--request", "-"}},
		{"change.revive", `{"version":"x"}`, "version", "revision", []string{"change", "revive", "--request", "-"}},
		{"change.unblock", `{"version":"x"}`, "version", "revision", []string{"change", "unblock", "--request", "-"}},
		{"change.reconcile", `{"version":"x"}`, "version", "revision", []string{"change", "reconcile", "--input", "-"}},
		{"learning.update", `{"version":"x"}`, "version", "revision", []string{"learning", "update", "--request", "-"}},
		{"change.groom version", `{"version":"x"}`, "version", "revision", []string{"change", "groom", "--request", "-"}},
		{"change.groom spec_version", `{"spec_version":"x"}`, "spec_version", "spec_revision", []string{"change", "groom", "--request", "-"}},
		{"adr.supersede target.version", `{"target":{"version":"x"}}`, "version", "", []string{"adr", "supersede", "--request", "-"}},
		{"adr.reverse target.version", `{"target":{"version":"x"}}`, "version", "", []string{"adr", "reverse", "--request", "-"}},
		{"adr.record change.version", `{"change":{"version":"x"}}`, "version", "", []string{"adr", "record", "--request", "-"}},
		{"finalize.retarget-children pr_version", `{"children":[{"pr_version":"x"}]}`, "pr_version", "",
			[]string{"finalize", "retarget-children", "--id", "1", "--revision", rev, "--input", "-"}},
	} {
		args := append(append([]string{}, c.args...), "--repo-dir", dir, "--json")
		out, errS, code := runCLIStdin(t, c.body, args...)
		if code != 2 || errS != "" || !strings.Contains(out, `"result":"invalid-input"`) {
			t.Errorf("%s: want invalid-input exit 2, got code=%d err=%q out=%q", c.name, code, errS, out)
			continue
		}
		if !strings.Contains(out, `unknown field \"`+c.key+`\"`) {
			t.Errorf("%s: refusal does not name the retired key %q: %s", c.name, c.key, out)
		}
		if c.replacement != "" && !strings.Contains(out, c.replacement) {
			t.Errorf("%s: refusal's accepted keys do not name %q: %s", c.name, c.replacement, out)
		}
	}
}

// revisionFlagOps is ADR-0129 row 40's operation set: exactly these commands pin
// the record revision on a flag.
var revisionFlagOps = [][]string{
	{"change", "attach-plan"}, {"change", "attach-results"}, {"change", "claim"}, {"change", "halt"},
	{"change", "mark-implemented"}, {"change", "reclaim"}, {"change", "refresh-claim"}, {"change", "resume-halted"},
	{"finalize", "block"}, {"finalize", "clear-block"}, {"finalize", "merge"}, {"finalize", "rebase"},
	{"finalize", "retarget-children"}, {"workspace", "prepare"},
}

func flagRequired(f *pflag.Flag) bool {
	v := f.Annotations[cobra.BashCompOneRequiredFlag]
	return len(v) == 1 && v[0] == "true"
}

// TestRevisionFlagHardCut pins ADR-0129 Decision 2 for rows 40/40a (change
// 0472). Over the WHOLE change/finalize/workspace subtree (derived from the live
// cobra tree, not listed): no command registers --version or --expect-version,
// and the commands registering a required --revision are exactly row 40's set
// (checked in both directions). Every old spelling is refused as an unknown flag.
func TestRevisionFlagHardCut(t *testing.T) {
	root := captureTree(t)
	want := map[string]bool{}
	for _, p := range revisionFlagOps {
		want[strings.Join(p, " ")] = true
	}
	got := map[string]bool{}
	var walk func(prefix string, c *cobra.Command)
	walk = func(prefix string, c *cobra.Command) {
		key := strings.TrimSpace(prefix + " " + c.Name())
		for _, gone := range []string{"version", "expect-version"} {
			if c.Flags().Lookup(gone) != nil {
				t.Errorf("%s still registers the retired --%s (ADR-0129 rows 40/40a)", key, gone)
			}
		}
		if f := c.Flags().Lookup("revision"); f != nil {
			got[key] = true
			if !flagRequired(f) {
				t.Errorf("%s: --revision is not required", key)
			}
		}
		for _, sub := range c.Commands() {
			walk(key, sub)
		}
	}
	for _, group := range []string{"change", "finalize", "workspace"} {
		g, _, err := root.Find([]string{group})
		if err != nil {
			t.Fatalf("find %s: %v", group, err)
		}
		walk("", g)
	}
	for k := range want {
		if !got[k] {
			t.Errorf("%s lacks a required --revision", k)
		}
	}
	for k := range got {
		if !want[k] {
			t.Errorf("%s registers --revision but is not a row-40 operation", k)
		}
	}
	rep, _, err := root.Find([]string{"change", "repair-identity"})
	if err != nil {
		t.Fatal(err)
	}
	if f := rep.Flags().Lookup("expect-revision"); f == nil || !flagRequired(f) {
		t.Errorf("change repair-identity lacks a required --expect-revision")
	}
	for _, p := range append(append([][]string{}, revisionFlagOps...), []string{"change", "repair-identity"}) {
		flag := "--version"
		if p[1] == "repair-identity" {
			flag = "--expect-version"
		}
		args := append(append([]string{}, p...), flag, "x")
		_, errS, code := runCLI(t, args...)
		if code != 2 || !strings.Contains(errS, "unknown flag: "+flag) {
			t.Errorf("%v %s: want exit 2 naming the unknown flag, got code=%d err=%q", p, flag, code, errS)
		}
	}
}

// TestRevisionFlagReachesRequest (change 0472): the value passed as --revision
// reaches the request. A read left on the retired "version" name returns ""
// silently and the op refuses empty-revision. Each op first gets an explicit
// empty --revision as a control, which must reach its shape validator, so the
// non-empty assert below it cannot pass vacuously. (workspace.prepare discovers the
// repository before validating shape, so it is covered by the hard cut and by
// TestRetiredVocabularySeal (internal/repoguard), not here.)
func TestRevisionFlagReachesRequest(t *testing.T) {
	const rev = "0123456789abcdef0123456789abcdef01234567"
	head := strings.Repeat("a", 40)
	dir := testsupport.TempDir(t)
	ev := filepath.Join(dir, "evidence.json")
	if err := os.WriteFile(ev, []byte(`{"schema":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		stdin string
		args  []string
	}{
		{"", []string{"change", "claim", "--id", "1"}},
		{"", []string{"change", "refresh-claim", "--id", "1"}},
		{"", []string{"change", "reclaim", "--id", "1"}},
		{"", []string{"change", "resume-halted", "--id", "1"}},
		{"", []string{"change", "attach-plan", "--id", "1", "--path", "p.md", "--commit", head}},
		{"", []string{"change", "attach-results", "--id", "1", "--path", "r.md", "--commit", head}},
		{"", []string{"change", "mark-implemented", "--id", "1", "--head", head, "--pr", "https://github.com/o/r/pull/1", "--evidence", ev}},
		{"{}", []string{"change", "halt", "--id", "1", "--input", "-"}},
		{"{}", []string{"finalize", "block", "--id", "1", "--pr-number", "1", "--attempt", "x", "--reason", "repair-needs-signoff", "--head", head, "--input", "-"}},
		{"", []string{"finalize", "clear-block", "--id", "1", "--head", head, "--pr-number", "1"}},
		{"", []string{"finalize", "merge", "--id", "1", "--head", head}},
		{"", []string{"finalize", "rebase", "--id", "1", "--head", head}},
		{`{"children":[]}`, []string{"finalize", "retarget-children", "--id", "1", "--input", "-"}},
	} {
		name := c.args[0] + " " + c.args[1]
		run := func(value string) string {
			args := append(append([]string{}, c.args...), "--revision", value, "--repo-dir", dir, "--json")
			out, _, _ := runCLIStdin(t, c.stdin, args...)
			return out
		}
		if out := run(""); !strings.Contains(out, `"code":"empty-revision"`) {
			t.Fatalf("%s: control: an empty --revision did not reach the shape validator: %s", name, out)
		}
		if out := run(rev); strings.Contains(out, `"code":"empty-revision"`) {
			t.Errorf("%s: --revision %s never reached the request: %s", name, rev, out)
		}
	}
	// repair-identity validates its own request shape before any read.
	repair := func(value string) string {
		out, _, _ := runCLI(t, "change", "repair-identity", "--id", "1", "--expect-revision", value,
			"--adopt-pr-head", "--expect-pr", "1", "--expect-head", "b", "--repo-dir", dir, "--json")
		return out
	}
	if out := repair(""); !strings.Contains(out, "expect-revision must be") {
		t.Fatalf("repair-identity control: an empty --expect-revision did not reach validation: %s", out)
	}
	if out := repair(rev); strings.Contains(out, "expect-revision must be") {
		t.Errorf("repair-identity: --expect-revision %s never reached the request: %s", rev, out)
	}
}
