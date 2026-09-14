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
		}{{"failed-json", `{"result":"gate-failed"}`, "diagnostic", 1}, {"invalid-json", `{`, "parse diagnostic", 2}} {
			t.Run(shell+"/"+tc.name, func(t *testing.T) {
				dir := testsupport.TempDir(t)
				gate := filepath.Join(dir, "gate")
				body := "#!/bin/sh\nprintf '%s' '" + tc.stdout + "'\nprintf '%s' '" + tc.stderr + "' >&2\nexit " + strconv.Itoa(tc.exit) + "\n"
				if err := os.WriteFile(gate, []byte(body), 0o755); err != nil {
					t.Fatal(err)
				}
				out, errFile, rc := filepath.Join(dir, "stdout"), filepath.Join(dir, "stderr"), filepath.Join(dir, "rc")
				program := "gate_argv=(\"$GATE\")\nfirst_stdout=\"$OUT\"\nfirst_stderr=\"$ERR\"\n" + string(match[1]) + "printf '%s' \"$gate_rc\" >\"$RC\"\n"
				cmd := exec.Command(shellPath, "-c", program)
				cmd.Env = append(os.Environ(), "GATE="+gate, "OUT="+out, "ERR="+errFile, "RC="+rc)
				if output, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("capture shell: %v: %s", err, output)
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
