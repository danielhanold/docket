package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

func TestRuntimePinsRejectStaleGlobalRolesAndSkillAliases(t *testing.T) {
	for _, broken := range []string{"", "valid-symlink", ".codex/agents/docket-demo.toml", ".agents/skills/docket-demo/SKILL.md", ".codex/skills/docket-demo/SKILL.md", "missing-reference", "../other.md", "../LAUNCH.md/extra", ".agents/skills/docket-demo/../../escape", "/absolute"} {
		t.Run(broken, func(t *testing.T) {
			root := testsupport.TempDir(t)
			pins := map[string]string{"../LAUNCH.md": "fixture metadata, not a runtime pin"}
			unsafe := strings.HasPrefix(broken, "../") || strings.Contains(broken, "/../") || filepath.IsAbs(broken)
			if unsafe {
				pins[broken] = "11507a0e2f5e69d5dfa40a62a1bd7b6ee57e6bcd85c67c9b8431b36fff21c437"
			}
			for _, relative := range []string{".codex/agents/docket-demo.toml", ".agents/skills/docket-demo/SKILL.md", ".agents/skills/docket-demo/references/entry.md"} {
				pins[relative] = "11507a0e2f5e69d5dfa40a62a1bd7b6ee57e6bcd85c67c9b8431b36fff21c437" // SHA-256 of new
				if err := writeFile(filepath.Join(root, relative), []byte("new"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.MkdirAll(filepath.Join(root, ".codex/skills"), 0o755); err != nil {
				t.Fatal(err)
			}
			if broken == "valid-symlink" {
				if err := os.Symlink(filepath.Join(root, ".agents/skills/docket-demo"), filepath.Join(root, ".codex/skills/docket-demo")); err != nil {
					t.Fatal(err)
				}
			} else {
				for _, p := range []string{"SKILL.md", "references/entry.md"} {
					if err := writeFile(filepath.Join(root, ".codex/skills/docket-demo", p), []byte("new"), 0o644); err != nil {
						t.Fatal(err)
					}
				}
			}
			if broken == "missing-reference" {
				if err := os.Remove(filepath.Join(root, ".agents/skills/docket-demo/references/entry.md")); err != nil {
					t.Fatal(err)
				}
			} else if broken != "" && broken != "valid-symlink" && !unsafe {
				if err := os.WriteFile(filepath.Join(root, broken), []byte("old"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			body, err := json.Marshal(map[string]any{"files": pins, "binary": map[string]string{"path": "/candidate/docket"}})
			if err != nil {
				t.Fatal(err)
			}
			manifestPath := filepath.Join(root, "manifest.json")
			if err := os.WriteFile(manifestPath, body, 0o644); err != nil {
				t.Fatal(err)
			}
			err = verifyRuntimePins(manifestPath, root)
			if (err == nil) != (broken == "" || broken == "valid-symlink") {
				t.Fatalf("broken=%q error=%v", broken, err)
			}
			if unsafe && !strings.Contains(err.Error(), "unsafe manifest path") {
				t.Fatalf("unsafe path refused for wrong reason: %v", err)
			}
		})
	}
}

func TestRuntimePinsRejectEmptyOrUnsafeManifest(t *testing.T) {
	for _, body := range []string{`{}`, `{"files":{"AGENTS.md":"abc"}}`, `{"files":{".codex/agents/../escape":"abc"}}`, `{"files":{".codex/agents/docket-demo.toml":"not-a-hash"}}`} {
		root := testsupport.TempDir(t)
		p := filepath.Join(root, "manifest.json")
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := verifyRuntimePins(p, root); err == nil {
			t.Fatalf("accepted %s", body)
		}
	}
}
