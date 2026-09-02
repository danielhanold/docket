package install

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/config"
	"github.com/danielhanold/docket/internal/testsupport"
)

// TestLegacyHarnessDefaultsFrozenCopy guards the embedded frozen floor against
// drift from the capture input it was copied from. The reproducer's pins and the
// corpus goldens both descend from testdata/legacy/_inputs/harness-defaults.yml;
// if the embedded copy under legacydata/ ever diverges from it, "is this a legacy
// install?" would silently answer against a different floor than the one the
// corpus was captured under.
func TestLegacyHarnessDefaultsFrozenCopy(t *testing.T) {
	captured, err := os.ReadFile(filepath.Join("testdata", "legacy", "_inputs", "harness-defaults.yml"))
	if err != nil {
		t.Fatalf("reading capture input: %v", err)
	}
	if !bytes.Equal(legacyHarnessDefaultsYML, captured) {
		t.Fatal("legacydata/harness-defaults.yml has drifted from testdata/legacy/_inputs/harness-defaults.yml")
	}
}

// globalAgents builds a resolved AgentsTable whose status and
// brainstorm-consultant carry GLOBAL-layer (model, effort) on every legacy
// harness — the provenance legacyInputsFor keys on to overlay the frozen floor.
// A "" model and "" effort means neither field is set (no global override).
func globalAgents(model, effort string) config.AgentsTable {
	glob := func(v string) config.Value[string] {
		return config.Value[string]{Value: v, Provenance: config.Provenance{Layer: config.LayerGlobal}, Explicit: true}
	}
	table := config.AgentsTable{}
	for _, h := range legacyHarnesses {
		row := map[string]config.AgentSetting{}
		for _, a := range []string{"status", "brainstorm-consultant"} {
			row[a] = config.AgentSetting{Model: glob(model), Effort: glob(effort)}
		}
		table[h] = row
	}
	return table
}

// agentsForShape mirrors the corpus config-<shape>.yml global overrides as a
// resolved AgentsTable, so legacyInputsFor reconstructs the exact pins the
// v0.9.2 emitters were fed for that shape. `default` has no override — pure
// frozen floor.
func agentsForShape(shape string) config.AgentsTable {
	switch shape {
	case "default":
		return nil
	case "fully-pinned":
		return globalAgents("legacy-pinned-model", "high")
	case "partially-pinned":
		return globalAgents("legacy-pinned-model", "") // effort: auto resolves to ""
	case "unpinned":
		return globalAgents("inherit", "") // model: inherit verbatim; effort: auto -> ""
	default:
		panic("unknown shape " + shape)
	}
}

// TestLegacyInputsForReproducesCorpus is the production-wiring proof: the pins
// legacyInputsFor derives (frozen v0.9.2 floor ⊕ the run's GLOBAL agent
// overrides) drive NewLegacyReproducer to reproduce every captured corpus golden
// byte-for-byte, across all four harnesses and all four pin shapes. This is the
// same assertion as TestLegacyReproducer_NativeAgents, but the inputs come from
// the production construction path rather than the hand-written pinsForShape
// table — so it catches a floor mis-parse or a wrong overlay, not just a bad
// emitter.
func TestLegacyInputsForReproducesCorpus(t *testing.T) {
	corpus := legacyCorpusDir(t)
	covered := 0
	for _, harness := range legacyHarnesses {
		for _, shape := range legacyShapes {
			rep := NewLegacyReproducer(legacyInputsFor(legacyHarnesses, agentsForShape(shape)))
			agentsDir := filepath.Join(corpus, harness, shape, "agents")
			entries, err := os.ReadDir(agentsDir)
			if err != nil {
				t.Fatalf("reading %s: %v", agentsDir, err)
			}
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				golden, err := os.ReadFile(filepath.Join(agentsDir, e.Name()))
				if err != nil {
					t.Fatalf("reading golden %s: %v", e.Name(), err)
				}
				targetPath := filepath.Join("/legacyroot", legacyAgentDirName[harness], "agents", e.Name())
				got, ok := rep(Target{Path: targetPath, Kind: KindFile, Role: roleAgent})
				if !ok {
					t.Errorf("%s/%s/%s: production inputs yield no legacy spelling", harness, shape, e.Name())
					continue
				}
				if !bytes.Equal(got, golden) {
					t.Errorf("%s/%s/%s: bytes differ:\n%s", harness, shape, e.Name(), firstDiff(got, golden))
					continue
				}
				covered++
			}
		}
	}
	if covered != 32 {
		t.Fatalf("covered %d goldens, want 32", covered)
	}
}

// TestInspectAdoptsExactLegacyViaProductionInputs proves the whole third
// ownership proof end to end through the production input construction: a target
// whose planned content DIFFERS from the on-disk bytes (so noop is impossible)
// classifies DispositionUpdate — adopt — exactly when the on-disk bytes are the
// frozen legacy reproduction, and DispositionConflict on any perturbation or when
// the seam is nil. It is deliberately independent of whether the live HEAD
// renderer happens to match v0.9.2 (which would make a real install a noop): the
// differing planned content forces the ownership proofs to be the deciding path.
func TestInspectAdoptsExactLegacyViaProductionInputs(t *testing.T) {
	corpus := legacyCorpusDir(t)
	rep := NewLegacyReproducer(legacyInputsFor(legacyHarnesses, agentsForShape("fully-pinned")))
	ext := map[string]string{"claude": "md", "codex": "toml", "cursor": "md", "opencode": "md"}

	for _, harness := range legacyHarnesses {
		for _, agent := range []string{"docket-status", "docket-brainstorm-consultant"} {
			name := agent + "." + ext[harness]
			legacyBytes, err := os.ReadFile(filepath.Join(corpus, harness, "fully-pinned", "agents", name))
			if err != nil {
				t.Fatalf("read corpus %s/%s: %v", harness, name, err)
			}
			dir := testsupport.TempDir(t)
			p := filepath.Join(dir, legacyAgentDirName[harness], "agents", name)
			// Planned content deliberately differs from the on-disk legacy bytes, so
			// the noop path is unreachable and the ONLY non-conflict verdict is proof
			// three — the frozen legacy reproduction.
			target := Target{Path: p, Kind: KindFile, Role: roleAgent, Content: []byte("docket HEAD content — differs\n")}

			writeFileOrDie(t, p, string(legacyBytes))
			got, err := InspectTarget(target, nil, rep)
			if err != nil {
				t.Fatalf("%s/%s: InspectTarget: %v", harness, agent, err)
			}
			if got.Disposition != DispositionUpdate {
				t.Errorf("%s/%s: disposition = %q, want %q (adopt)", harness, agent, got.Disposition, DispositionUpdate)
			}
			if got.Reason != "" {
				t.Errorf("%s/%s: adopted target carries reason %q, want empty", harness, agent, got.Reason)
			}

			// One byte flipped: no longer the frozen reproduction -> ownership conflict.
			mutated := append([]byte(nil), legacyBytes...)
			mutated[len(mutated)/2] ^= 0x20
			writeFileOrDie(t, p, string(mutated))
			bad, err := InspectTarget(target, nil, rep)
			if err != nil {
				t.Fatalf("%s/%s mutated: InspectTarget: %v", harness, agent, err)
			}
			if bad.Disposition != DispositionConflict || bad.Reason != ReasonOwnershipConflict {
				t.Errorf("%s/%s mutated: got (%q, %q), want (%q, %q)",
					harness, agent, bad.Disposition, bad.Reason, DispositionConflict, ReasonOwnershipConflict)
			}

			// Nil seam: the very same exact-legacy bytes are unprovable and preserved.
			writeFileOrDie(t, p, string(legacyBytes))
			bare, err := InspectTarget(target, nil, nil)
			if err != nil {
				t.Fatalf("%s/%s nil seam: InspectTarget: %v", harness, agent, err)
			}
			if bare.Disposition != DispositionConflict {
				t.Errorf("%s/%s nil seam: disposition = %q, want %q", harness, agent, bare.Disposition, DispositionConflict)
			}
		}
	}
}
