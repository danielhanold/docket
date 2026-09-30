package cli

import (
	"strings"
	"testing"

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
			[]string{"finalize", "retarget-children", "--id", "1", "--version", rev, "--input", "-"}},
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
