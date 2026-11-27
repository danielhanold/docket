package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// TestLeakCheckRegistered: `docket leak check` is registered under the `leak`
// group as the read-only `leak.check` capability, carrying a repeatable --file
// (a string array, so a comma in a path is never split) and --repo-dir.
func TestLeakCheckRegistered(t *testing.T) {
	root := captureTree(t)

	cmd, _, err := root.Find([]string{"leak", "check"})
	if err != nil || cmd == nil || cmd.Name() != "check" {
		t.Fatalf("leak check not registered: cmd=%v err=%v", cmd, err)
	}
	for _, flag := range []string{"file", "repo-dir"} {
		if cmd.Flags().Lookup(flag) == nil {
			t.Errorf("leak check: missing --%s flag", flag)
		}
	}
	if f := cmd.Flags().Lookup("file"); f != nil && f.Value.Type() != "stringArray" {
		t.Errorf("leak check: --file is %q, want a repeatable stringArray", f.Value.Type())
	}
	if got := cmd.Annotations[capAnnotationID]; got != "leak.check" {
		t.Errorf("leak check capability id = %q, want leak.check", got)
	}
	if got := cmd.Annotations[capAnnotationEffects]; got != string(EffectRead) {
		t.Errorf("leak check effects = %q, want %q", got, EffectRead)
	}

	grp, _, err := root.Find([]string{"leak"})
	if err != nil || grp == nil || grp.Name() != "leak" {
		t.Fatalf("leak group not registered: grp=%v err=%v", grp, err)
	}
}

// TestLeakCheckCLIMissingFile: with no --file the command still reaches
// app.LeakCheck, which refuses invalid-input (missing-file) in exactly one
// newline-terminated document.
func TestLeakCheckCLIMissingFile(t *testing.T) {
	out, errS, code := runCLI(t, "leak", "check", "--repo-dir", testsupport.TempDir(t), "--json")
	if errS != "" {
		t.Fatalf("unexpected stderr %q (code=%d)", errS, code)
	}
	for _, want := range []string{`"operation":"leak.check"`, `"result":"invalid-input"`, `"reason":"missing-file"`} {
		if !strings.Contains(out, want) {
			t.Errorf("document lacks %s: %q", want, out)
		}
	}
	if strings.Count(out, "\n") != 1 || !strings.HasSuffix(out, "\n") {
		t.Fatalf("must be exactly one newline-terminated document, got %q", out)
	}
}

// TestLeakCheckCLIRoutesEveryFile: every --file value reaches the operation —
// the second, missing, path is the one refused as unreadable-file.
func TestLeakCheckCLIRoutesEveryFile(t *testing.T) {
	dir := testsupport.TempDir(t)
	clean := writeLeakCLIFile(t, dir, "clean.md", "nothing to see\n")
	missing := filepath.Join(dir, "absent.md")
	out, errS, code := runCLI(t, "leak", "check", "--file", clean, "--file", missing, "--repo-dir", dir, "--json")
	if errS != "" {
		t.Fatalf("unexpected stderr %q (code=%d)", errS, code)
	}
	if !strings.Contains(out, `"reason":"unreadable-file"`) {
		t.Fatalf("second --file did not reach the operation: %q", out)
	}
	if !strings.Contains(out, missing) {
		t.Fatalf("refusal does not name the missing path %q: %q", missing, out)
	}
}

// TestLeakCommandsAssetIndependent guards the install.go registration: the
// leak commands read the repository and the given files, never installed
// assets, so they must be asset-independent.
func TestLeakCommandsAssetIndependent(t *testing.T) {
	for _, key := range []string{"leak", "leak check"} {
		if !assetIndependent[key] {
			t.Errorf("%q is not registered asset-independent", key)
		}
	}
}

func writeLeakCLIFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}
