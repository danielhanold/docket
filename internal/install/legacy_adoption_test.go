package install_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/install"
)

// fullyPinnedAgents is the resolved agent table for the `fully-pinned` corpus
// shape: a GLOBAL override pinning status and brainstorm-consultant to
// (legacy-pinned-model, high) on every harness. Global provenance is what the
// production reproducer keys on to overlay the frozen v0.9.2 floor, so an install
// carrying this table reproduces the fully-pinned corpus bytes.
func fullyPinnedAgents() config.AgentsTable {
	glob := func(v string) config.Value[string] {
		return config.Value[string]{Value: v, Provenance: config.Provenance{Layer: config.LayerGlobal}, Explicit: true}
	}
	table := config.AgentsTable{}
	for _, h := range []string{"claude", "codex", "cursor", "opencode"} {
		table[h] = map[string]config.AgentSetting{
			"status":                {Model: glob("legacy-pinned-model"), Effort: glob("high")},
			"brainstorm-consultant": {Model: glob("legacy-pinned-model"), Effort: glob("high")},
		}
	}
	return table
}

// legacyAgentDest is the real install target path a harness adapter plans for a
// user-level agent definition, matching the reproducer's inventory path shapes:
// claude/codex/cursor hang a dotted dir off HOME, opencode's root is undotted
// under the XDG config home.
func legacyAgentDest(w *world, harness, filename string) string {
	switch harness {
	case "opencode":
		return filepath.Join(w.home, ".config", "opencode", "agents", filename)
	default:
		return filepath.Join(w.home, "."+harness, "agents", filename)
	}
}

// seedFullyPinnedLegacyTree copies the two captured fully-pinned agent goldens
// for every harness onto their real target paths and returns the seeded paths.
func seedFullyPinnedLegacyTree(t *testing.T, w *world) []string {
	t.Helper()
	ext := map[string]string{"claude": "md", "codex": "toml", "cursor": "md", "opencode": "md"}
	var seeded []string
	for _, harness := range []string{"claude", "codex", "cursor", "opencode"} {
		for _, agent := range []string{"docket-status", "docket-brainstorm-consultant"} {
			name := agent + "." + ext[harness]
			src := filepath.Join("testdata", "legacy", harness, "fully-pinned", "agents", name)
			body, err := os.ReadFile(src)
			if err != nil {
				t.Fatalf("read corpus %s: %v", src, err)
			}
			dst := legacyAgentDest(w, harness, name)
			writeFile(t, dst, string(body))
			seeded = append(seeded, dst)
		}
	}
	return seeded
}

func legacyOptions(w *world) install.Options {
	agents := fullyPinnedAgents()
	o := w.options(agents)
	o.Config = &config.Snapshot{Effective: config.Effective{Agents: agents}}
	return o
}

// TestInstallAdoptsExactLegacyTree seeds a HOME with an exact fully-pinned
// v0.9.2 legacy user-level tree and NO prior install.json, then runs a release
// install. Every seeded legacy agent file must be adopted — no ownership
// conflict — and the published state must record Go ownership of each one. With
// the reproducer left nil (the pre-B5 wiring) each seeded file is a foreign-block
// conflict and the whole install refuses, so a green run here is the wiring's
// regression guard.
func TestInstallAdoptsExactLegacyTree(t *testing.T) {
	w := newWorld(t, allHarnessDirs...)
	seeded := seedFullyPinnedLegacyTree(t, w)

	out := install.Install(legacyOptions(w))
	if out.Reason != "" {
		t.Fatalf("install refused: reason=%q err=%v conflicts=%v", out.Reason, out.Err, conflictPaths(out))
	}
	if !out.Applied {
		t.Fatalf("install applied nothing; actions=%v", actionPaths(out))
	}

	// No seeded legacy path is a conflict, and the published state records each as
	// a docket-owned target (a harness attribution is Go ownership).
	state := loadState(t, w.roots)
	if state == nil {
		t.Fatal("no installed state was published")
	}
	recorded := map[string]install.TargetRecord{}
	for _, rec := range state.Targets {
		recorded[filepath.Clean(rec.Path)] = rec
	}
	for _, p := range seeded {
		if hasAction(out, install.OpConflict, p) {
			t.Errorf("legacy file reported as a conflict, not adopted: %s", p)
		}
		rec, ok := recorded[filepath.Clean(p)]
		if !ok {
			t.Errorf("state records no target for adopted legacy path %s", p)
			continue
		}
		if rec.Harness == "" {
			t.Errorf("adopted target %s recorded with no harness (Go ownership)", p)
		}
	}
}

// TestInstallExactLegacyTreeForeignFileConflictsOnlyThere adds one unknown
// foreign file at a planned agent path the frozen reproducer cannot reproduce.
// The install must refuse with an ownership conflict at exactly that path — the
// exact-legacy siblings are still adopted, so the foreign file is the only
// conflict, proving adoption is byte-exact and not a blanket take-over.
func TestInstallExactLegacyTreeForeignFileConflictsOnlyThere(t *testing.T) {
	w := newWorld(t, allHarnessDirs...)
	seedFullyPinnedLegacyTree(t, w)

	foreign := legacyAgentDest(w, "claude", "docket-adr.md")
	writeFile(t, foreign, "this is not what docket wrote\n")

	out := install.Install(legacyOptions(w))
	if out.Reason != install.ReasonOwnershipConflict {
		t.Fatalf("reason = %q, want %q (err=%v)", out.Reason, install.ReasonOwnershipConflict, out.Err)
	}
	if out.Applied {
		t.Fatal("a conflicting install must apply nothing")
	}
	conflicts := conflictPaths(out)
	if len(conflicts) != 1 || conflicts[0] != filepath.Clean(foreign) {
		t.Fatalf("conflicts = %v, want exactly [%s]", conflicts, filepath.Clean(foreign))
	}
}

func conflictPaths(out install.Outcome) []string {
	var paths []string
	for _, a := range out.Actions {
		if a.Op == install.OpConflict {
			paths = append(paths, filepath.Clean(a.Path))
		}
	}
	return paths
}
