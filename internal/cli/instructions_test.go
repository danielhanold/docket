package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/harness"
	"github.com/danielhanold/docket/internal/testsupport"
)

const (
	instrBlock   = "<!-- docket:dispatch:start (managed) -->\n## Rules\nroute it\n<!-- docket:dispatch:end -->\n"
	instrLesson  = "\n## Lessons\nkeep it small\n"
	instrPrivate = instrBlock + instrLesson
)

// instrPrivateRepo fabricates a private primary worktree whose instructions
// file holds content, and returns the worktree root.
func instrPrivateRepo(t *testing.T, content string) string {
	t.Helper()
	r := filepath.Join(testsupport.TempDir(t), "r")
	if err := os.MkdirAll(filepath.Join(r, ".git", "dckt"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(r, ".git", "dckt", "AGENTS.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return r
}

// instrSharedRepo fabricates a shared repository (a .git directory, no dckt).
func instrSharedRepo(t *testing.T) string {
	t.Helper()
	r := filepath.Join(testsupport.TempDir(t), "shared")
	if err := os.MkdirAll(filepath.Join(r, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return r
}

// instrOutsideGit returns a directory with no .git at or above it, skipping
// when the temp dir lives inside a git work tree.
func instrOutsideGit(t *testing.T) string {
	t.Helper()
	dir := testsupport.TempDir(t)
	for d := dir; ; d = filepath.Dir(d) {
		if _, err := os.Lstat(filepath.Join(d, ".git")); err == nil {
			t.Skipf("ancestor %s of the temp dir has a .git entry", d)
		}
		if filepath.Dir(d) == d {
			return dir
		}
	}
}

func TestInstructionsPrivatePlain(t *testing.T) {
	t.Chdir(instrPrivateRepo(t, instrPrivate))
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"instructions"}, instrPrivate},
		{[]string{"instructions", "--section", "dispatch"}, instrBlock},
		{[]string{"instructions", "--section", "lessons"}, instrLesson},
	}
	for _, c := range cases {
		out, errS, code := runCLI(t, c.args...)
		if out != c.want || errS != "" || code != 0 {
			t.Errorf("%v: out=%q err=%q code=%d, want out=%q", c.args, out, errS, code, c.want)
		}
	}
}

func TestInstructionsPrivateNoLessonsPrintsNothing(t *testing.T) {
	t.Chdir(instrPrivateRepo(t, instrBlock))
	out, errS, code := runCLI(t, "instructions", "--section", "lessons")
	if out != "" || errS != "" || code != 0 {
		t.Errorf("out=%q err=%q code=%d, want silence and 0", out, errS, code)
	}
}

func TestInstructionsSharedAndOutsideGitPrintNothing(t *testing.T) {
	for label, dir := range map[string]func(*testing.T) string{
		"shared":      instrSharedRepo,
		"outside git": instrOutsideGit,
	} {
		t.Run(label, func(t *testing.T) {
			t.Chdir(dir(t))
			for _, args := range [][]string{
				{"instructions"},
				{"instructions", "--hook", "claude"},
				{"instructions", "--hook", "claude", "--section", "dispatch"},
			} {
				out, errS, code := runCLI(t, args...)
				if out != "" || errS != "" || code != 0 {
					t.Errorf("%v: out=%q err=%q code=%d, want silence and 0", args, out, errS, code)
				}
			}
		})
	}
}

func TestInstructionsHookClaude(t *testing.T) {
	t.Chdir(instrPrivateRepo(t, instrPrivate))
	out, errS, code := runCLI(t, "instructions", "--hook", "claude", "--section", "dispatch")
	if errS != "" || code != 0 {
		t.Fatalf("err=%q code=%d", errS, code)
	}
	var doc struct {
		HookSpecificOutput struct {
			HookEventName     string `json:"hookEventName"`
			AdditionalContext string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("decode %q: %v", out, err)
	}
	if doc.HookSpecificOutput.HookEventName != "SessionStart" || doc.HookSpecificOutput.AdditionalContext != instrBlock {
		t.Errorf("decoded %+v, want SessionStart / the dispatch block", doc)
	}
}

func decodeCursor(t *testing.T, out string) string {
	t.Helper()
	var doc struct {
		AdditionalContext string `json:"additional_context"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("decode %q: %v", out, err)
	}
	return doc.AdditionalContext
}

// TestInstructionsHookCursorLocatesRepoWithoutWorkingDirectory proves the
// Cursor hook finds the repository from CURSOR_PROJECT_DIR or the stdin
// workspace_roots, never from the working directory (here an unrelated one).
func TestInstructionsHookCursorLocatesRepoWithoutWorkingDirectory(t *testing.T) {
	private := instrPrivateRepo(t, instrPrivate)
	shared := instrSharedRepo(t)
	t.Setenv("CURSOR_PROJECT_DIR", private)
	t.Chdir(testsupport.TempDir(t))

	out, errS, code := runCLI(t, "instructions", "--hook", "cursor")
	if errS != "" || code != 0 || decodeCursor(t, out) != instrPrivate {
		t.Errorf("env: out=%q err=%q code=%d", out, errS, code)
	}

	t.Setenv("CURSOR_PROJECT_DIR", "")
	out, errS, code = runCLIStdin(t, `{"workspace_roots":["`+private+`"]}`, "instructions", "--hook", "cursor")
	if errS != "" || code != 0 || decodeCursor(t, out) != instrPrivate {
		t.Errorf("stdin: out=%q err=%q code=%d", out, errS, code)
	}

	// From inside the private repository, stdin that names nothing usable must
	// still print nothing: the working directory is never the fallback.
	t.Chdir(private)
	for label, stdin := range map[string]string{
		"shared repo": `{"workspace_roots":["` + shared + `"]}`,
		"garbage":     "not json at all",
		"empty":       "",
	} {
		out, errS, code := runCLIStdin(t, stdin, "instructions", "--hook", "cursor")
		if out != "" || errS != "" || code != 0 {
			t.Errorf("%s: out=%q err=%q code=%d, want silence and 0", label, out, errS, code)
		}
	}
}

func TestInstructionsUnreadableFile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 file")
	}
	r := instrPrivateRepo(t, instrPrivate)
	path := filepath.Join(r, ".git", "dckt", "AGENTS.md")
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
	t.Chdir(r)

	out, errS, code := runCLI(t, "instructions")
	if out != "" || code != 1 || !strings.Contains(errS, path) {
		t.Errorf("plain: out=%q err=%q code=%d, want empty stdout, stderr naming %s, exit 1", out, errS, code, path)
	}
	t.Setenv("CURSOR_PROJECT_DIR", r)
	for _, hook := range []string{"claude", "cursor"} {
		out, errS, code := runCLI(t, "instructions", "--hook", hook)
		if out != "" || errS != "" || code != 0 {
			t.Errorf("--hook %s: out=%q err=%q code=%d, want silence and 0", hook, out, errS, code)
		}
	}
}

func TestInstructionsJSON(t *testing.T) {
	t.Chdir(instrPrivateRepo(t, instrPrivate))
	out, errS, code := runCLI(t, "--json", "instructions")
	if errS != "" || code != 0 {
		t.Fatalf("err=%q code=%d", errS, code)
	}
	for _, want := range []string{`"operation":"instructions"`, `"result":"applied"`, `"private":true`} {
		if !strings.Contains(out, want) {
			t.Errorf("--json output %q lacks %s", out, want)
		}
	}
	for _, args := range [][]string{
		{"--json", "instructions", "--hook", "claude"},
		{"instructions", "--hook", "foo"},
		{"instructions", "--section", "foo"},
		{"instructions", "--hook", "claude", "--section", "foo"},
	} {
		_, _, code := runCLI(t, args...)
		if code != 2 {
			t.Errorf("%v: exit %d, want 2 (invalid arguments)", args, code)
		}
	}
	out, _, _ = runCLI(t, "--json", "instructions", "--hook", "claude")
	if !strings.Contains(out, `"result":"invalid-input"`) {
		t.Errorf("--json --hook claude: %q, want an invalid-input document", out)
	}
}

// TestTriggerCommandsParse runs every installed hook command, minus its
// program name, through the real CLI in a private fixture.
func TestTriggerCommandsParse(t *testing.T) {
	r := instrPrivateRepo(t, instrPrivate)
	t.Chdir(r)
	for _, command := range []string{harness.ClaudeDispatchHookCommand, harness.ClaudeLessonsHookCommand, harness.CursorHookCommand} {
		words := strings.Fields(command)
		if len(words) < 2 || words[0] != "dckt" {
			t.Fatalf("trigger %q does not start with the dckt program name", command)
		}
		if command == harness.CursorHookCommand {
			t.Setenv("CURSOR_PROJECT_DIR", r)
		}
		out, errS, code := runCLI(t, words[1:]...)
		if code != 0 || errS != "" || strings.Count(out, "\n") != 1 || !json.Valid([]byte(out)) {
			t.Errorf("%q: out=%q err=%q code=%d, want one JSON line and exit 0", command, out, errS, code)
		}
	}
}

func TestInstructionsRunsWithoutInstallation(t *testing.T) {
	pinInstallEnv(t)
	t.Chdir(instrPrivateRepo(t, instrPrivate))
	if out, errS, code := runCLI(t, "instructions"); code != 0 || out != instrPrivate {
		t.Errorf("out=%q err=%q code=%d, want the file and 0 with no installation", out, errS, code)
	}
}
