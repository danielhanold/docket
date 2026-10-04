package config

import (
	"strings"
	"testing"
)

// unsupportedHits returns the display name of every shape that matches s.
func unsupportedHits(shapes []UnsupportedKeyShape, s string) []string {
	var hits []string
	for _, sh := range shapes {
		if sh.Re.MatchString(s) {
			hits = append(hits, sh.Name)
		}
	}
	return hits
}

func TestUnsupportedKeyShapes(t *testing.T) {
	shapes := UnsupportedKeyShapes(SettingPaths())
	if len(shapes) == 0 {
		t.Fatalf("no unsupported-key shapes derived from the schema registry")
	}

	// Registry-derived population: every unsupported path, spelled as a
	// config key, is matched, so a new unsupported row is covered with no edit
	// here. A top-level key is also probed inside a nested comment.
	for _, p := range SettingPaths() {
		if p.Supported {
			continue
		}
		spelled := strings.ReplaceAll(p.Path, "*", "x")
		probes := []string{"set `" + spelled + "` here"}
		if !strings.Contains(p.Path, ".") {
			probes = []string{spelled + ": x\n", "#   # " + spelled + ": x\n"}
		}
		for _, probe := range probes {
			if len(unsupportedHits(shapes, probe)) == 0 {
				t.Errorf("registry path %s not matched by %q", p.Path, probe)
			}
		}
	}

	// Nested-comment probes: a key commented out inside an already-commented
	// block. Each of these escaped the single optional comment marker.
	for _, s := range []string{
		"#   # terminal_publish: true",
		"# # skills:",
		"#   #   cap: 5",
		"#   #     adr: { model: x, runner: codex }",
		"#   # - skills: x",
		"#\t#\tterminal_publish: true",
	} {
		if len(unsupportedHits(shapes, s)) == 0 {
			t.Errorf("nested-comment unsupported key not matched: %q", s)
		}
	}

	// Negatives: supported keys, prose headings, and scope tags, nested or not.
	for _, s := range []string{
		"build:", "review:", "plan:", "finalize.gate: local", "learnings.enabled",
		"auto_groomable: true", "#   # auto_groomable: true",
		"#   # gate: local", "## Reconcile: x", "### Readiness: build-ready",
		"#   # scope: any layer",
	} {
		if got := unsupportedHits(shapes, s); len(got) != 0 {
			t.Errorf("supported text matched: %q -> %v", s, got)
		}
	}

	// Degenerate registries derive no shapes and do not panic.
	if got := UnsupportedKeyShapes(nil); len(got) != 0 {
		t.Errorf("UnsupportedKeyShapes(nil) = %d shapes, want 0", len(got))
	}
	if got := UnsupportedKeyShapes([]SettingPath{{Path: "finalize.gate", Supported: true}}); len(got) != 0 {
		t.Errorf("all-supported registry derived %d shapes, want 0", len(got))
	}
}
