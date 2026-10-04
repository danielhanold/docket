//go:build integration

package bashupgrade

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// ownershipRefusals are the repository-setup finding codes that mean the binary
// refused (or could not prove) docket ownership of the Bash-made metadata branch.
// Any of them on a saved case is a binary defect per the spec's "When the binary is
// wrong" rule, never a guide step.
var ownershipRefusals = []string{
	"metadata-root-foreign", "metadata-root-unresolved", "metadata-ownership-unverified",
	"metadata-presence-unknown", "docket-dir-foreign",
}

func TestIntegrationBashUpgradeOwnership(t *testing.T) {
	for _, tag := range savedTags(t) {
		t.Run(tag, func(t *testing.T) {
			c := restoreCase(t, tag)
			if got, want := listRecords(t, c), readRecords(t, tag); !reflect.DeepEqual(got, want) {
				t.Fatalf("records.txt disagrees with the saved bundle\n got %v\nwant %v", got, want)
			}
			for _, args := range [][]string{{"repository", "prepare", "--json"}, {"repository", "check", "--json"}} {
				r := c.run(t, c.Clone, c.Docket, args...)
				if r.Stdout == "" {
					t.Fatalf("%s produced no JSON (code %d); stderr:\n%s", strings.Join(args, " "), r.Code, r.Stderr)
				}
				for _, f := range findingCodes(t, r.Stdout) {
					for _, bad := range ownershipRefusals {
						if f.Code == bad {
							t.Fatalf("BLOCKED: %s refused the Bash-made docket branch (%s)\nstdout:\n%s\nstderr:\n%s",
								strings.Join(args, " "), f.Code, r.Stdout, r.Stderr)
						}
					}
				}
				// prepare must adopt the Bash clone (applied or no-op); check exits
				// non-zero whenever it reports findings, which the guide handles later.
				if args[1] == "prepare" && r.Code != 0 {
					t.Fatalf("repository prepare did not adopt the restored Bash clone (code %d)\nstdout:\n%s\nstderr:\n%s", r.Code, r.Stdout, r.Stderr)
				}
				t.Logf("%s exit %d\nstdout:\n%s\nstderr:\n%s", strings.Join(args, " "), r.Code, r.Stdout, r.Stderr)
			}
		})
	}
}

// TestIntegrationBashUpgradeGuideShape keeps the guide and the step registry in step
// both ways, and refuses an unmarked block that runs docket commands.
func TestIntegrationBashUpgradeGuideShape(t *testing.T) {
	src := readGuide(t)
	g, err := parseGuide(src)
	if err != nil {
		t.Fatal(err)
	}
	marked := map[string]bool{}
	for _, s := range g.Steps {
		marked[s.Name] = true
		if _, ok := stepRegistry[s.Name]; !ok {
			t.Errorf("guide step %q (line %d) is neither executed nor mirrored by the test", s.Name, s.Line)
		}
	}
	for name := range stepRegistry {
		if !marked[name] {
			t.Errorf("test registry step %q has no marked block in the guide", name)
		}
	}
	for _, f := range g.Unmarked {
		if cmds := docketCommandLines(f.Body); len(cmds) > 0 {
			t.Errorf("unmarked fenced block at line %d runs docket commands %q; mark it as an upgrade step", f.Line, cmds)
		}
	}
	if len(g.Steps) < len(stepRegistry) || len(stepRegistry) == 0 {
		t.Fatalf("population floor: %d marked steps, %d registry entries", len(g.Steps), len(stepRegistry))
	}
}

// TestIntegrationBashUpgradeGuide runs the guide end to end on every saved case and
// requires a clean, writable repository with every record intact. Across the cases,
// every row of the guide's finding and settings tables must have been observed, so
// the tables carry no claim the test never saw.
func TestIntegrationBashUpgradeGuide(t *testing.T) {
	src := readGuide(t)
	seenFindings := map[string]bool{}
	seenSettings := map[string]bool{}
	dispatchRemoved := false
	for _, tag := range savedTags(t) {
		t.Run(tag, func(t *testing.T) {
			c := restoreCase(t, tag)
			before := listRecords(t, c)
			st := runGuide(t, c)

			// Both saved installs carry Bash skill links the installer will not take
			// over, so the first run always conflicts and the remedy always runs.
			if len(st.Conflicts) == 0 {
				t.Fatalf("first install reported no conflict (attempts: %+v)", st.InstallAttempts)
			}
			if len(st.InstallAttempts) < 2 {
				t.Fatalf("the remedy did not lead to a second install run")
			}
			assertCleanEndState(t, c)
			assertRecordsSurvive(t, c, before)
			if after, _ := os.ReadFile(filepath.Join(c.Clone, "CLAUDE.md")); strings.Contains(string(after), "docket:dispatch:") {
				t.Errorf("the repository CLAUDE.md still carries the Bash dispatch block after the guide")
			}
			dispatchRemoved = dispatchRemoved || st.DispatchBlockRemoved
			assertWritable(t, c)
			for _, f := range st.Observed["repo-check"] {
				seenFindings[f.Code] = true
			}
			for _, p := range st.Settings {
				seenSettings[p] = true
			}
		})
	}
	if t.Failed() {
		return
	}
	if !dispatchRemoved {
		t.Errorf("the guide's dispatch-block step removed no block on any saved case")
	}
	for code := range findingTable(t, src) {
		if !seenFindings[code] {
			t.Errorf("the guide's finding table lists %s, which no saved case reported", code)
		}
	}
	for _, row := range settingTable(t, src) {
		for _, p := range row.Patterns {
			if !seenSettings[p] {
				t.Errorf("the guide's settings table lists %s, which no saved case reported", p)
			}
		}
	}
}

// assertCleanEndState: install check, repository check and status all clean.
func assertCleanEndState(t *testing.T, c *upgradeCase) {
	t.Helper()
	for _, args := range [][]string{{"--json", "install", "check"}, {"--json", "repository", "check"}} {
		r := c.run(t, c.Clone, "docket", args...)
		if r.Code != 0 {
			t.Errorf("docket %s exited %d\nstdout:\n%s\nstderr:\n%s", strings.Join(args, " "), r.Code, r.Stdout, r.Stderr)
			continue
		}
		for _, f := range findingCodes(t, r.Stdout) {
			if f.Severity == "error" || f.Severity == "warning" {
				t.Errorf("docket %s still reports %s (%s)", strings.Join(args, " "), f.Code, f.Severity)
			}
		}
	}
	r := c.run(t, c.Clone, "docket", "--json", "status")
	for _, f := range findingCodes(t, r.Stdout) {
		if f.Severity == "error" {
			t.Errorf("docket status still reports error %s", f.Code)
		}
	}
}

// assertRecordsSurvive: every record that existed before the upgrade is still at the
// same path on the same branch at origin.
func assertRecordsSurvive(t *testing.T, c *upgradeCase, before []string) {
	t.Helper()
	if len(before) == 0 {
		t.Fatal("population floor: no records before the upgrade")
	}
	for _, rec := range before {
		branch, path, ok := strings.Cut(rec, ":")
		if !ok {
			t.Fatalf("malformed record entry %q", rec)
		}
		if r := c.run(t, c.Origin, "git", "cat-file", "-e", branch+":"+path); r.Code != 0 {
			t.Errorf("record %s did not survive the upgrade", rec)
		}
	}
}

// assertWritable: a new change can be created and shows on the re-rendered board.
func assertWritable(t *testing.T, c *upgradeCase) {
	t.Helper()
	const title = "Upgrade proof stub"
	req, err := json.Marshal(map[string]string{
		"request_id": "bash-upgrade-proof-0001", "title": title, "type": "chore", "priority": "low",
		"why": "Prove the upgraded repository is writable.", "what_changes": "Nothing; this is a probe.",
		"out_of_scope": "Everything else.",
	})
	if err != nil {
		t.Fatal(err)
	}
	reqPath := filepath.Join(c.Root, "change-create.json")
	if err := os.WriteFile(reqPath, req, 0o644); err != nil {
		t.Fatal(err)
	}
	if r := c.run(t, c.Clone, "docket", "change", "create", "--request", reqPath); r.Code != 0 {
		t.Fatalf("change create exited %d\nstdout:\n%s\nstderr:\n%s", r.Code, r.Stdout, r.Stderr)
	}
	board := c.mustGit(t, c.Origin, "show", "docket:"+changesDir(t, c)+"/BOARD.md")
	if !strings.Contains(board, title) {
		t.Fatalf("the new change %q is not on the docket branch's board:\n%s", title, board)
	}
}
