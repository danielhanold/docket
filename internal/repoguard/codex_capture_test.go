package repoguard

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

func TestCodexGateCaptureLiteralPreservesFirstResponse(t *testing.T) {
	reference := filepath.Join(guardRoot(t), "skills", "docket-build", "references", "codex-task-handoff.md")
	b, err := os.ReadFile(reference)
	if err != nil {
		t.Fatal(err)
	}
	match := regexp.MustCompile("(?s)```bash\\n(.*?)```").FindSubmatch(b)
	if len(match) != 2 {
		t.Fatal("missing literal bash capture block")
	}
	for _, shell := range []string{"bash", "zsh"} {
		shellPath, err := exec.LookPath(shell)
		if err != nil {
			t.Logf("optional shell %s unavailable", shell)
			continue
		}
		for _, tc := range []struct {
			name, stdout, stderr string
			exit                 int
			chunked              bool
		}{{"failed-json", `{"result":"gate-failed"}`, "diagnostic", 1, false}, {"invalid-json", `{`, "parse diagnostic", 2, false}, {"chunked-json", `{"result":"applied"}`, "first/last", 0, true}, {"empty-terminal", "", "empty", 0, true}} {
			t.Run(shell+"/"+tc.name, func(t *testing.T) {
				dir := testsupport.TempDir(t)
				gate := filepath.Join(dir, "gate")
				body := "#!/bin/sh\nprintf x >>\"$CALLS\"\nprintf '%s' '" + tc.stdout + "'\nprintf '%s' '" + tc.stderr + "' >&2\nexit " + strconv.Itoa(tc.exit) + "\n"
				if tc.chunked {
					middle := len(tc.stdout) / 2
					body = "#!/bin/sh\nprintf x >>\"$CALLS\"\nsleep 0.01\nprintf '%s' '" + tc.stdout[:middle] + "'\nprintf '%s' '" + tc.stderr[:len(tc.stderr)/2] + "' >&2\nsleep 0.01\nprintf '%s' '" + tc.stdout[middle:] + "'\nprintf '%s' '" + tc.stderr[len(tc.stderr)/2:] + "' >&2\nexit " + strconv.Itoa(tc.exit) + "\n"
				}
				if err := os.WriteFile(gate, []byte(body), 0o755); err != nil {
					t.Fatal(err)
				}
				out, errFile, rc := filepath.Join(dir, "stdout"), filepath.Join(dir, "stderr"), filepath.Join(dir, "rc")
				program := "gate_argv=(\"$GATE\")\nfirst_stdout=\"$OUT\"\nfirst_stderr=\"$ERR\"\n" + string(match[1]) + "printf '%s' \"$gate_rc\" >\"$RC\"\n"
				cmd := exec.Command(shellPath, "-c", program)
				cmd.Env = append(os.Environ(), "GATE="+gate, "OUT="+out, "ERR="+errFile, "RC="+rc, "CALLS="+filepath.Join(dir, "calls"))
				if output, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("capture shell: %v: %s", err, output)
				}
				calls, _ := os.ReadFile(filepath.Join(dir, "calls"))
				if string(calls) != "x" {
					t.Fatalf("mutation replayed: %q", calls)
				}
				gotOut, _ := os.ReadFile(out)
				gotErr, _ := os.ReadFile(errFile)
				gotRC, _ := os.ReadFile(rc)
				if string(gotOut) != tc.stdout || string(gotErr) != tc.stderr || strings.TrimSpace(string(gotRC)) != strconv.Itoa(tc.exit) {
					t.Fatalf("capture=(%q,%q,%q)", gotOut, gotErr, gotRC)
				}
			})
		}
	}
}

func TestCodexFeatureBootstrapAndPrivatePayloadContract(t *testing.T) {
	root := guardRoot(t)
	feature, err := os.ReadFile(filepath.Join(root, "skills", "docket-convention", "references", "codex-feature-binding.md"))
	if err != nil {
		t.Fatal(err)
	}
	handoff, err := os.ReadFile(filepath.Join(root, "skills", "docket-build", "references", "codex-task-handoff.md"))
	if err != nil {
		t.Fatal(err)
	}
	native, err := os.ReadFile(filepath.Join(root, "skills", "docket-convention", "references", "codex-native-dispatch.md"))
	if err != nil {
		t.Fatal(err)
	}
	planning, err := os.ReadFile(filepath.Join(root, "skills", "docket-implement-next", "references", "codex-planning-results.md"))
	if err != nil {
		t.Fatal(err)
	}
	review, err := os.ReadFile(filepath.Join(root, "skills", "docket-review", "references", "codex-review-binding.md"))
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"feature binding": string(feature), "task handoff": string(handoff)} {
		for _, clause := range []string{"schema --operation agent.check-inputs", "--payload", "--payload-sha256", "entry_argv"} {
			if !strings.Contains(body, clause) {
				t.Errorf("%s omits %q", name, clause)
			}
		}
	}
	for name, body := range map[string]string{"native dispatch": string(native), "planning": string(planning)} {
		if !strings.Contains(body, "schema --operation agent.check-inputs") {
			t.Errorf("%s does not point to the versioned input documents", name)
		}
	}
	if !strings.Contains(string(review), "declared resource") || !strings.Contains(string(review), "SHA-256") {
		t.Error("review binding omits hashed evidence construction")
	}
	f := string(feature)
	for _, forbidden := range []string{"must not run `repository.prepare`", "metadata-writing operations"} {
		if !strings.Contains(f, forbidden) {
			t.Errorf("feature binding omits child prohibition %q", forbidden)
		}
	}
	h := string(handoff)
	if !strings.Contains(h, "Missing assignment or payload files are controller work") {
		t.Error("task handoff lets a controller treat not-yet-created provenance files as unavailable")
	}
	order := []string{"repository/workspace preparation", "immutable assignment", "prepare the child scope", "private payload", "at `dispatch`"}
	last := -1
	for _, phrase := range order {
		i := strings.Index(h, phrase)
		if i < 0 || i <= last {
			t.Fatalf("task handoff construction order missing or out of order at %q", phrase)
		}
		last = i
	}
}
