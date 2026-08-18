package install

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// legacyHarnesses is the four shipped harnesses with a named v0.9.2 emitter —
// the harness dimension of the frozen corpus.
var legacyHarnesses = []string{"claude", "codex", "cursor", "opencode"}

// legacyShapes is the four captured pin shapes.
var legacyShapes = []string{"default", "fully-pinned", "partially-pinned", "unpinned"}

// legacyAgentDirName maps a harness token to the on-disk directory that holds
// its user-level agent definitions, matching the real install target paths the
// harness adapters emit: three hang a dotted dir off the home directory, while
// opencode's root is an undotted dir under the XDG config home.
var legacyAgentDirName = map[string]string{
	"claude":   ".claude",
	"codex":    ".codex",
	"cursor":   ".cursor",
	"opencode": "opencode",
}

// pinsForShape builds the resolved (model, effort) pins the v0.9.2 emitters were
// fed for each captured shape, transcribed from testdata/legacy/README.md's
// shape table. The synthetic shapes are uniform across every harness and both
// agents; only `default` varies per harness/agent (the realistic install).
func pinsForShape(shape string) map[string]AgentPin {
	switch shape {
	case "default":
		return map[string]AgentPin{
			"status": {ByHarness: map[string]HarnessPin{
				"claude":   {Model: "claude-haiku-4-5-20251001", Effort: "medium"},
				"cursor":   {Model: "cursor-grok-4.5-low-fast", Effort: "auto"},
				"codex":    {Model: "gpt-5.6-luna", Effort: "xhigh"},
				"opencode": {Model: "openrouter/deepseek/deepseek-v4-flash-0731", Effort: "low"},
			}},
			"brainstorm-consultant": {ByHarness: map[string]HarnessPin{
				"claude":   {Model: "claude-opus-5", Effort: "medium"},
				"cursor":   {Model: "cursor-grok-4.5-high", Effort: "auto"},
				"codex":    {Model: "gpt-5.6-sol", Effort: "medium"},
				"opencode": {Model: "openrouter/moonshotai/kimi-k3", Effort: "medium"},
			}},
		}
	default:
		var model, effort string
		switch shape {
		case "fully-pinned":
			model, effort = "legacy-pinned-model", "high"
		case "partially-pinned":
			model, effort = "legacy-pinned-model", "auto"
		case "unpinned":
			model, effort = "inherit", "auto"
		default:
			panic("unknown shape " + shape)
		}
		uniform := func() AgentPin {
			by := map[string]HarnessPin{}
			for _, h := range legacyHarnesses {
				by[h] = HarnessPin{Model: model, Effort: effort}
			}
			return AgentPin{ByHarness: by}
		}
		return map[string]AgentPin{
			"status":                uniform(),
			"brainstorm-consultant": uniform(),
		}
	}
}

func legacyCorpusDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join("testdata", "legacy")
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("legacy corpus dir missing: %v", err)
	}
	return dir
}

// TestLegacyReproducer_NativeAgents covers every captured (harness, shape,
// agent) golden: it builds the matching LegacyInputs, calls the reproducer with
// the corresponding install-target path, and asserts the reproduced bytes equal
// the frozen golden byte-for-byte.
func TestLegacyReproducer_NativeAgents(t *testing.T) {
	corpus := legacyCorpusDir(t)
	covered := 0
	for _, harness := range legacyHarnesses {
		for _, shape := range legacyShapes {
			agentsDir := filepath.Join(corpus, harness, shape, "agents")
			entries, err := os.ReadDir(agentsDir)
			if err != nil {
				t.Fatalf("reading %s: %v", agentsDir, err)
			}
			inputs := LegacyInputs{
				Harnesses: append([]string(nil), legacyHarnesses...),
				AgentPins: pinsForShape(shape),
			}
			rep := NewLegacyReproducer(inputs)
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
					t.Errorf("%s/%s/%s: reproducer reported no legacy spelling for %s",
						harness, shape, e.Name(), targetPath)
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
	// Four harnesses x four shapes x two agents = 32 captured goldens.
	if covered != 32 {
		t.Fatalf("expected to cover 32 goldens, covered %d", covered)
	}
}

// TestLegacyReproducer_NonInventory proves the reproducer refuses everything
// outside the closed inventory with (nil, false): a harness not in the input
// set, a non-file kind, a path that is not an agent definition, an unknown
// agent short-name, and an extension that does not match the harness.
func TestLegacyReproducer_NonInventory(t *testing.T) {
	full := LegacyInputs{Harnesses: append([]string(nil), legacyHarnesses...), AgentPins: pinsForShape("default")}

	cases := []struct {
		name   string
		inputs LegacyInputs
		target Target
	}{
		{
			name:   "harness absent from input set",
			inputs: LegacyInputs{Harnesses: []string{"codex"}, AgentPins: pinsForShape("default")},
			target: Target{Path: "/legacyroot/.claude/agents/docket-status.md", Kind: KindFile, Role: roleAgent},
		},
		{
			name:   "managed block that is not the dispatch block",
			inputs: full,
			target: Target{Path: "/legacyroot/.claude/CLAUDE.md", Kind: KindManagedBlock, BlockName: "other", Role: "dispatch"},
		},
		{
			name:   "symlink kind is not a legacy artifact",
			inputs: full,
			target: Target{Path: "/legacyroot/.claude/agents/docket-status.md", Kind: KindSymlink, Role: roleAgent},
		},
		{
			name:   "not an agent-definition path",
			inputs: full,
			target: Target{Path: "/legacyroot/.claude/skills/docket-status.md", Kind: KindFile, Role: roleAgent},
		},
		{
			name:   "cursor rules dir but not the dispatch rule",
			inputs: full,
			target: Target{Path: "/legacyroot/.cursor/rules/docket-other.mdc", Kind: KindFile, Role: "dispatch"},
		},
		{
			name:   "unknown agent short-name",
			inputs: full,
			target: Target{Path: "/legacyroot/.claude/agents/docket-nonexistent.md", Kind: KindFile, Role: roleAgent},
		},
		{
			name:   "extension does not match harness (codex must be .toml)",
			inputs: full,
			target: Target{Path: "/legacyroot/.codex/agents/docket-status.md", Kind: KindFile, Role: roleAgent},
		},
		{
			name:   "agent path but no docket- prefix",
			inputs: full,
			target: Target{Path: "/legacyroot/.claude/agents/status.md", Kind: KindFile, Role: roleAgent},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rep := NewLegacyReproducer(tc.inputs)
			got, ok := rep(tc.target)
			if ok || got != nil {
				t.Fatalf("expected (nil,false) for %q, got ok=%v len(bytes)=%d", tc.target.Path, ok, len(got))
			}
		})
	}
}

// TestLegacyReproducer_CursorDispatchRule covers the frozen Cursor
// docket-dispatch.mdc rule (Kind (b) of the closed inventory): pin-invariant,
// a regular KindFile at ~/.cursor/rules/docket-dispatch.mdc. With cursor in the
// harness set the reproducer returns the captured bytes byte-for-byte; with
// cursor absent it is outside the inventory and returns (nil,false).
func TestLegacyReproducer_CursorDispatchRule(t *testing.T) {
	corpus := legacyCorpusDir(t)
	golden, err := os.ReadFile(filepath.Join(corpus, "cursor", "docket-dispatch.mdc"))
	if err != nil {
		t.Fatalf("reading cursor dispatch golden: %v", err)
	}
	target := Target{
		Path: filepath.Join("/legacyroot", ".cursor", "rules", "docket-dispatch.mdc"),
		Kind: KindFile,
		Role: "dispatch",
	}

	// cursor present -> frozen bytes + true. The rule is pin-invariant, so no
	// AgentPins are needed.
	rep := NewLegacyReproducer(LegacyInputs{Harnesses: append([]string(nil), legacyHarnesses...)})
	got, ok := rep(target)
	if !ok {
		t.Fatalf("cursor present: reproducer reported no legacy spelling for %s", target.Path)
	}
	if !bytes.Equal(got, golden) {
		t.Fatalf("cursor dispatch bytes differ:\n%s", firstDiff(got, golden))
	}

	// cursor absent from the harness set -> outside the inventory.
	repNoCursor := NewLegacyReproducer(LegacyInputs{Harnesses: []string{"claude", "codex", "opencode"}})
	got, ok = repNoCursor(target)
	if ok || got != nil {
		t.Fatalf("cursor absent: expected (nil,false), got ok=%v len=%d", ok, len(got))
	}
}

// TestLegacyReproducer_EmbedsAllBuiltins proves every v0.9.2 built-in agent
// body is embedded, not just the two with goldens: each of the 16 short-names
// reproduces on the claude harness.
func TestLegacyReproducer_EmbedsAllBuiltins(t *testing.T) {
	want := []string{
		"adr", "auto-groom-critic", "auto-groom", "brainstorm-consultant",
		"build-economy", "build-max", "build-premium", "build-standard",
		"finalize-change", "implement-next", "integration-repair",
		"rebase-resolver", "review-deep", "review-lean", "review-standard", "status",
	}
	rep := NewLegacyReproducer(LegacyInputs{Harnesses: []string{"claude"}})
	for _, short := range want {
		path := filepath.Join("/legacyroot", ".claude", "agents", "docket-"+short+".md")
		got, ok := rep(Target{Path: path, Kind: KindFile, Role: roleAgent})
		if !ok || len(got) == 0 {
			t.Errorf("built-in %q: reproducer returned ok=%v len=%d", short, ok, len(got))
		}
	}
}

// legacyBuiltinDigests pins the SHA-256 of every frozen v0.9.2 built-in
// agent-source body embedded under legacydata/docket-*.md, keyed by agent
// short-name. There are exactly sixteen — the full set a real v0.9.2 machine
// has — and a real install refuses (no --force) unless every one byte-matches,
// so an accidental edit to ANY body must redden CI even though only two are
// pinned by the captured corpus goldens. The digests were computed from the
// current embedded bytes, which the change verified byte-match the v0.9.2
// checkout; this guard is the self-contained cross-check that needs no external
// checkout. Any drift — an edited, added, or removed body — flips a digest or
// the count and fails TestLegacyReproducer_FrozenBodyDigests below.
var legacyBuiltinDigests = map[string]string{
	"adr":                   "3064d7c638ea83dada098034e29ec760cf1c7f1edde17563a2d18eff5425b485",
	"auto-groom-critic":     "3d28f51fbf68cc9a7b0c28cb328f2f21e2778642abe0ae96847cc9887ec83590",
	"auto-groom":            "16efa7d4cdd23c7f9e42f603c406b2a5b254e2d56fcd2569aeff86bf7beec275",
	"brainstorm-consultant": "d3ddefb5afd62739254d9cc777c1de4b4cd576792fa32f1721c852556044b5a6",
	"build-economy":         "2572cbc2ad69771bdbe94bc9c19964ae9dd8031afd262aef762bec0998ed3f89",
	"build-max":             "e584b8eba10f853a4ccddd7faa1c6597bfcbe581c5d3b4c2a2e8ab60d03fd9c5",
	"build-premium":         "2c7d4560e71a966ea2a6b28391b76dbfddd4bc379a1fb3984d6fa793505e30bb",
	"build-standard":        "a1e1c99d2c0de3a222e09a8ecb5e994a44472c820488e6a92f5792a104ceab9b",
	"finalize-change":       "9d061281879fdb49383ab1d4a0339938bff062149c905ef479b35bb3ff6aafd4",
	"implement-next":        "f752983100cc034b2ad7cf79fd9ea610aad216dc888dff2f2148b3f166b269a2",
	"integration-repair":    "bb133be5d10351762c304c81eb574b1fde3140570687716b1afac7a3cfb31b44",
	"rebase-resolver":       "db68bbf03f74041eb433b2fbcc0377e0492b7d989674aa195019913b8419c32a",
	"review-deep":           "026214da8b620ab91b95019d7da8939f14d790cef52d0c05e905adaa244f3a8b",
	"review-lean":           "c59a81691760fdb1fe5a1d6bffa8e4b496516eb140190ff88d13b9c1feff5e10",
	"review-standard":       "cfc8e3bae0e8beb946378b7214c587fb0a527656ff3904b346fc77999a40c7e6",
	"status":                "dc6a323a1e2fc1de09956d80e1c653f4a427cbb953e59380d976c354c5011b11",
}

// TestLegacyReproducer_FrozenBodyDigests freezes all sixteen embedded v0.9.2
// agent-source bodies, not just the two the captured corpus goldens pin. It is
// bound on the property "the frozen bytes are unchanged": each embedded body's
// raw bytes must hash to its pinned SHA-256, the embedded set must be exactly
// the sixteen built-ins (count + every name), and no extra .md may appear. A
// body edit flips its digest; an added/removed body flips the count or a
// membership check — so any drift reddens here without needing the v0.9.2
// checkout (the only external cross-check CI lacks). Non-vacuous by
// construction: change one byte of any legacydata/docket-*.md and its SHA-256
// no longer equals the pinned hex, failing the per-body assertion.
func TestLegacyReproducer_FrozenBodyDigests(t *testing.T) {
	// The embedded set the reproducer actually parses (loadLegacyAgentSources's
	// `.md` scan) must be exactly the sixteen pinned built-ins.
	if len(legacyAgentSources) != len(legacyBuiltinDigests) {
		t.Fatalf("embedded agent-source count = %d, want %d (the frozen v0.9.2 built-ins)",
			len(legacyAgentSources), len(legacyBuiltinDigests))
	}
	for short := range legacyBuiltinDigests {
		if _, ok := legacyAgentSources[short]; !ok {
			t.Errorf("pinned built-in %q is absent from the embedded sources", short)
		}
	}
	for short := range legacyAgentSources {
		if _, ok := legacyBuiltinDigests[short]; !ok {
			t.Errorf("embedded source %q has no pinned digest (added/renamed body?)", short)
		}
	}

	// Every embedded body must hash to its pinned digest. Read the raw bytes
	// straight from the embed.FS the production loader reads, so the guard sees
	// exactly the frozen bytes.
	for short, wantHex := range legacyBuiltinDigests {
		data, err := legacydata.ReadFile("legacydata/docket-" + short + ".md")
		if err != nil {
			t.Errorf("%s: reading embedded body: %v", short, err)
			continue
		}
		sum := sha256.Sum256(data)
		gotHex := hex.EncodeToString(sum[:])
		if gotHex != wantHex {
			t.Errorf("%s: embedded body SHA-256 = %s, want pinned %s (frozen body drifted)",
				short, gotHex, wantHex)
		}
	}
}

// TestLegacyReproducer_DispatchBlock covers Kind (c): the docket-managed
// dispatch-block interior. It is harness-neutral and pin-invariant —
// sync_dispatch_surfaces wrote the same interior into every targeted instruction
// file — so the reproducer returns the frozen interior for any dispatch-block
// target regardless of harness or pins, byte-for-byte equal to the captured
// interior golden. A managed block of any other name is outside the inventory.
func TestLegacyReproducer_DispatchBlock(t *testing.T) {
	corpus := legacyCorpusDir(t)
	golden, err := os.ReadFile(filepath.Join(corpus, "dispatch-block", "interior.md"))
	if err != nil {
		t.Fatalf("reading dispatch-block interior golden: %v", err)
	}
	target := Target{
		Path:      filepath.Join("/legacyroot", ".claude", "CLAUDE.md"),
		Kind:      KindManagedBlock,
		BlockName: "dispatch",
		Role:      "dispatch",
	}

	// Pin-invariant and harness-neutral: even empty inputs reproduce it.
	rep := NewLegacyReproducer(LegacyInputs{})
	got, ok := rep(target)
	if !ok {
		t.Fatalf("reproducer reported no legacy spelling for the dispatch block")
	}
	if !bytes.Equal(got, golden) {
		t.Fatalf("dispatch block interior differs:\n%s", firstDiff(got, golden))
	}

	// A managed block of a different name is outside the closed inventory.
	other := Target{Path: target.Path, Kind: KindManagedBlock, BlockName: "other", Role: "dispatch"}
	if got, ok := rep(other); ok || got != nil {
		t.Fatalf("non-dispatch block: expected (nil,false), got ok=%v len=%d", ok, len(got))
	}
}

// --- Task B7: mutation refusal matrix ----------------------------------------
//
// The tests below prove the third ownership proof is EXACT: adoption is refused
// the moment any single dimension is perturbed away from the frozen corpus —
// a flipped byte, a changed pin (a legacy input), an out-of-inventory path, a
// broken/unbalanced marker, or a changed Target kind. Every dimension is bound
// on the PROPERTY "adoption is refused" (DispositionConflict, with the reason
// ManagedBlockInvalid where markers are the cause), never on a byte-spelling of
// what the refusal looks like (learning byte-pattern-guard-matches-a-spelling).
//
// Each row is non-vacuous: it first proves the UNMUTATED artifact IS adopted
// (DispositionUpdate) with the same reproducer, so a row that always refused —
// because of an unrelated bug in the reproducer or the harness — is caught. This
// is itself a mutation-test of the reproducer's own exactness (AGENTS.md "Guards
// and tests"): perturb the thing the proof guards and watch the adoption redden.

// replacementRender is the plan's DESIRED new bytes for an adopted target — it
// is deliberately never any legacy golden, so the on-disk legacy bytes differ
// from Content (ruling out a no-op) and the only route to DispositionUpdate is
// the legacy proof.
const replacementRender = "REPLACEMENT RENDER — never any legacy golden\n"

// flipOneByte returns a copy of b with exactly one byte changed (a single-bit
// flip of the middle byte), so the result is guaranteed to differ from b.
func flipOneByte(b []byte) []byte {
	out := append([]byte(nil), b...)
	i := len(out) / 2
	out[i] ^= 0x01
	return out
}

// legacyAgentShort recovers the agent short-name from a corpus filename
// (docket-<name>.<ext> -> <name>).
func legacyAgentShort(filename string) string {
	return strings.TrimPrefix(strings.TrimSuffix(filename, filepath.Ext(filename)), "docket-")
}

// legacyNativeCase is one captured native-agent golden plus the coordinates
// needed to rebuild its inputs and its install-target path.
type legacyNativeCase struct {
	harness, shape, filename, agent string
	golden                          []byte
}

func (c legacyNativeCase) label() string { return c.harness + "/" + c.shape + "/" + c.agent }

// loadLegacyNativeCases loads every captured native-agent golden (four harnesses
// x four shapes x two agents = 32).
func loadLegacyNativeCases(t *testing.T) []legacyNativeCase {
	t.Helper()
	corpus := legacyCorpusDir(t)
	var out []legacyNativeCase
	for _, harness := range legacyHarnesses {
		for _, shape := range legacyShapes {
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
				out = append(out, legacyNativeCase{harness, shape, e.Name(), legacyAgentShort(e.Name()), golden})
			}
		}
	}
	if len(out) != 32 {
		t.Fatalf("expected 32 native goldens, loaded %d", len(out))
	}
	return out
}

func nativeInputs(shape string) LegacyInputs {
	return LegacyInputs{
		Harnesses: append([]string(nil), legacyHarnesses...),
		AgentPins: pinsForShape(shape),
	}
}

func nativeAgentPath(root, harness, filename string) string {
	return filepath.Join(root, legacyAgentDirName[harness], "agents", filename)
}

// assertAdopted asserts an unmutated in-inventory artifact is adopted — the
// non-vacuity half of every mutation row.
func assertAdopted(t *testing.T, target Target, legacy LegacyReproducer, label string) {
	t.Helper()
	got, err := InspectTarget(target, nil, legacy)
	if err != nil {
		t.Fatalf("%s: InspectTarget: %v", label, err)
	}
	if got.Disposition != DispositionUpdate {
		t.Fatalf("%s: unmutated artifact was NOT adopted: disposition=%q reason=%q — a mutation row over this would be vacuous",
			label, got.Disposition, got.Reason)
	}
}

// assertRefused asserts a perturbed artifact is refused with the expected
// reason, never falsely adopted.
func assertRefused(t *testing.T, target Target, legacy LegacyReproducer, wantReason, label string) {
	t.Helper()
	got, err := InspectTarget(target, nil, legacy)
	if err != nil {
		t.Fatalf("%s: InspectTarget: %v", label, err)
	}
	if got.Disposition != DispositionConflict {
		t.Errorf("%s: perturbed artifact was adopted (want refusal): disposition=%q reason=%q", label, got.Disposition, got.Reason)
		return
	}
	if got.Reason != wantReason {
		t.Errorf("%s: refusal reason=%q, want %q", label, got.Reason, wantReason)
	}
}

// TestLegacyAdoption_ByteMutationRefused — dimension "byte": one flipped byte of
// a golden's on-disk content is no longer the frozen bytes, so it is a
// conflict, not an adoption.
func TestLegacyAdoption_ByteMutationRefused(t *testing.T) {
	for _, c := range loadLegacyNativeCases(t) {
		t.Run(c.label(), func(t *testing.T) {
			legacy := NewLegacyReproducer(nativeInputs(c.shape))
			root := t.TempDir()
			p := nativeAgentPath(root, c.harness, c.filename)
			target := Target{Path: p, Kind: KindFile, Content: []byte(replacementRender), Role: roleAgent}

			writeFileOrDie(t, p, string(c.golden))
			assertAdopted(t, target, legacy, "unmutated")

			mutated := flipOneByte(c.golden)
			if bytes.Equal(mutated, c.golden) {
				t.Fatal("byte flip did not change the golden")
			}
			writeFileOrDie(t, p, string(mutated))
			assertRefused(t, target, legacy, ReasonOwnershipConflict, "one flipped byte")
		})
	}
}

// TestLegacyAdoption_PinMutationRefused — dimension "input (pin)": changing a
// resolved model pin (a legacy input) makes the reproduced candidate bytes
// diverge from the byte-exact on-disk golden, so the same tree no longer adopts.
func TestLegacyAdoption_PinMutationRefused(t *testing.T) {
	for _, c := range loadLegacyNativeCases(t) {
		t.Run(c.label(), func(t *testing.T) {
			root := t.TempDir()
			p := nativeAgentPath(root, c.harness, c.filename)
			writeFileOrDie(t, p, string(c.golden))
			target := Target{Path: p, Kind: KindFile, Content: []byte(replacementRender), Role: roleAgent}

			assertAdopted(t, target, NewLegacyReproducer(nativeInputs(c.shape)), "correct pins")

			// Perturb exactly this agent's model on this harness. Appending a
			// suffix changes the emitted bytes on every harness: a concrete model
			// changes verbatim; the `inherit` sentinel stops being the drop
			// sentinel and is emitted concretely (adding a line on codex/cursor/
			// opencode, changing the value on claude).
			in := nativeInputs(c.shape)
			ap := in.AgentPins[c.agent]
			byH := make(map[string]HarnessPin, len(ap.ByHarness))
			for k, v := range ap.ByHarness {
				byH[k] = v
			}
			hp := byH[c.harness]
			hp.Model += "-MUTATED"
			byH[c.harness] = hp
			ap.ByHarness = byH
			in.AgentPins[c.agent] = ap
			mutRep := NewLegacyReproducer(in)

			// Non-vacuity: the mutated pin actually moves the reproduced bytes off
			// the golden. If it did not, refusal would be trivially guaranteed for
			// the wrong reason.
			if cand, ok := mutRep(target); !ok || bytes.Equal(cand, c.golden) {
				t.Fatalf("pin mutation did not change reproduced bytes (ok=%v, equal-to-golden=%v)", ok, bytes.Equal(cand, c.golden))
			}
			assertRefused(t, target, mutRep, ReasonOwnershipConflict, "mutated model pin")
		})
	}
}

// TestLegacyAdoption_PathMutationRefused — dimension "path": the exact golden
// bytes at a location outside the closed inventory (agents/ -> skills/) resolve
// to no legacy spelling, so they are a conflict.
func TestLegacyAdoption_PathMutationRefused(t *testing.T) {
	for _, c := range loadLegacyNativeCases(t) {
		t.Run(c.label(), func(t *testing.T) {
			legacy := NewLegacyReproducer(nativeInputs(c.shape))
			root := t.TempDir()

			good := nativeAgentPath(root, c.harness, c.filename)
			writeFileOrDie(t, good, string(c.golden))
			assertAdopted(t, Target{Path: good, Kind: KindFile, Content: []byte(replacementRender), Role: roleAgent}, legacy, "in-inventory path")

			// Right bytes, wrong location: a sibling skills/ dir is not the
			// agents/ inventory shape. The reproducer refuses the path outright...
			bad := filepath.Join(root, legacyAgentDirName[c.harness], "skills", c.filename)
			badTarget := Target{Path: bad, Kind: KindFile, Content: []byte(replacementRender), Role: roleAgent}
			if got, ok := legacy(badTarget); ok || got != nil {
				t.Fatalf("reproducer accepted an out-of-inventory path: ok=%v len=%d", ok, len(got))
			}
			// ...and the inspection over those same bytes is therefore a conflict.
			writeFileOrDie(t, bad, string(c.golden))
			assertRefused(t, badTarget, legacy, ReasonOwnershipConflict, "out-of-inventory path")
		})
	}
}

// TestLegacyAdoption_KindMutationRefused — dimension "kind": declaring a symlink
// where a file target belongs breaks the legacy spelling (the reproducer keys on
// KindFile), so the same on-disk bytes are refused.
func TestLegacyAdoption_KindMutationRefused(t *testing.T) {
	for _, c := range loadLegacyNativeCases(t) {
		t.Run(c.label(), func(t *testing.T) {
			legacy := NewLegacyReproducer(nativeInputs(c.shape))
			root := t.TempDir()
			p := nativeAgentPath(root, c.harness, c.filename)
			writeFileOrDie(t, p, string(c.golden))

			assertAdopted(t, Target{Path: p, Kind: KindFile, Content: []byte(replacementRender), Role: roleAgent}, legacy, "file kind")

			// A symlink target where a plain file belongs: the reproducer returns
			// (nil,false) for a non-file kind, so the regular file on disk is
			// unprovable and preserved.
			symTarget := Target{
				Path:       p,
				Kind:       KindSymlink,
				LinkTarget: filepath.Join(root, "elsewhere", "target"),
				Role:       roleAgent,
			}
			if got, ok := legacy(symTarget); ok || got != nil {
				t.Fatalf("reproducer accepted a symlink kind: ok=%v len=%d", ok, len(got))
			}
			assertRefused(t, symTarget, legacy, ReasonOwnershipConflict, "symlink where a file belongs")
		})
	}
}

// TestLegacyAdoption_DispatchBlockMutationsRefused covers the managed-block kind
// (c) across three dimensions: byte (an interior byte flip), marker (broken /
// unbalanced markers that short-circuit to ManagedBlockInvalid BEFORE any legacy
// check), and kind (a plain-file target where a managed block belongs). Every
// mutation is proven against the SAME frozen interior that, unmutated, adopts.
func TestLegacyAdoption_DispatchBlockMutationsRefused(t *testing.T) {
	corpus := legacyCorpusDir(t)
	interior, err := os.ReadFile(filepath.Join(corpus, "dispatch-block", "interior.md"))
	if err != nil {
		t.Fatalf("reading dispatch-block interior: %v", err)
	}
	// The dispatch interior is harness-neutral and pin-invariant, so empty inputs
	// reproduce it (matching the real production reproducer for this target).
	legacy := NewLegacyReproducer(LegacyInputs{})
	blockTarget := func(p string, kind TargetKind) Target {
		return Target{Path: p, Kind: kind, BlockName: "dispatch", Content: []byte("new dispatch interior\n"), Role: "dispatch"}
	}

	t.Run("unmutated interior adopts", func(t *testing.T) {
		root := t.TempDir()
		p := filepath.Join(root, "CLAUDE.md")
		writeFileOrDie(t, p, managedFile(string(interior)))
		assertAdopted(t, blockTarget(p, KindManagedBlock), legacy, "valid legacy block")
	})

	t.Run("byte: one interior byte changed is a conflict", func(t *testing.T) {
		root := t.TempDir()
		p := filepath.Join(root, "CLAUDE.md")
		mutated := flipOneByte(interior)
		if bytes.Equal(mutated, interior) {
			t.Fatal("byte flip did not change the interior")
		}
		writeFileOrDie(t, p, managedFile(string(mutated)))
		assertRefused(t, blockTarget(p, KindManagedBlock), legacy, ReasonOwnershipConflict, "interior byte flip")
	})

	t.Run("marker: an unbalanced (dangling start) marker is managed-block-invalid", func(t *testing.T) {
		root := t.TempDir()
		p := filepath.Join(root, "CLAUDE.md")
		// The body IS the exact legacy interior: were the legacy check to run
		// before marker validation this would be wrongly adopted. Marker validity
		// is a precondition, so it must short-circuit to ManagedBlockInvalid.
		writeFileOrDie(t, p, "# Notes\n\n<!-- docket:dispatch:start (managed by docket) -->\n"+string(interior))
		assertRefused(t, blockTarget(p, KindManagedBlock), legacy, ReasonManagedBlockInvalid, "dangling start marker over the exact legacy interior")
	})

	t.Run("marker: a malformed start keyword is managed-block-invalid", func(t *testing.T) {
		root := t.TempDir()
		p := filepath.Join(root, "CLAUDE.md")
		// `:begin` is not a valid docket marker keyword; with a lone valid `:end`
		// the block is unbalanced, so it is refused before any legacy check.
		writeFileOrDie(t, p, "# Notes\n\n<!-- docket:dispatch:begin (managed by docket) -->\n"+string(interior)+"<!-- docket:dispatch:end -->\n")
		assertRefused(t, blockTarget(p, KindManagedBlock), legacy, ReasonManagedBlockInvalid, "malformed start keyword")
	})

	t.Run("kind: a plain-file target where a managed block belongs is a conflict", func(t *testing.T) {
		root := t.TempDir()
		p := filepath.Join(root, "CLAUDE.md")
		writeFileOrDie(t, p, managedFile(string(interior)))
		// Declaring KindFile makes the whole instruction file the artifact; the
		// reproducer has no file-level legacy spelling for it, so it is refused.
		fileTarget := Target{Path: p, Kind: KindFile, Content: []byte(replacementRender), Role: "dispatch"}
		assertRefused(t, fileTarget, legacy, ReasonOwnershipConflict, "file kind where a managed block belongs")
	})
}

// TestLegacyAdoption_CursorRuleMutationsRefused covers the Cursor dispatch-rule
// kind (b) across byte, path, and input (the cursor harness token) dimensions —
// each against the same golden that, unmutated, adopts.
func TestLegacyAdoption_CursorRuleMutationsRefused(t *testing.T) {
	corpus := legacyCorpusDir(t)
	golden, err := os.ReadFile(filepath.Join(corpus, "cursor", "docket-dispatch.mdc"))
	if err != nil {
		t.Fatalf("reading cursor dispatch golden: %v", err)
	}
	legacy := NewLegacyReproducer(LegacyInputs{Harnesses: append([]string(nil), legacyHarnesses...)})
	root := t.TempDir()
	good := filepath.Join(root, ".cursor", "rules", "docket-dispatch.mdc")
	target := Target{Path: good, Kind: KindFile, Content: []byte(replacementRender), Role: "dispatch"}

	writeFileOrDie(t, good, string(golden))
	assertAdopted(t, target, legacy, "cursor rule unmutated")

	// byte: one flipped byte is no longer the frozen rule.
	writeFileOrDie(t, good, string(flipOneByte(golden)))
	assertRefused(t, target, legacy, ReasonOwnershipConflict, "cursor rule byte flip")

	// path: right bytes, wrong filename under the same rules/ dir.
	bad := filepath.Join(root, ".cursor", "rules", "docket-other.mdc")
	writeFileOrDie(t, bad, string(golden))
	assertRefused(t, Target{Path: bad, Kind: KindFile, Content: []byte(replacementRender), Role: "dispatch"},
		legacy, ReasonOwnershipConflict, "cursor rule wrong filename")

	// input: cursor absent from the targeted harness set puts the rule outside
	// the inventory even for byte-exact bytes.
	legacyNoCursor := NewLegacyReproducer(LegacyInputs{Harnesses: []string{"claude", "codex", "opencode"}})
	writeFileOrDie(t, good, string(golden))
	assertRefused(t, target, legacyNoCursor, ReasonOwnershipConflict, "cursor absent from harness set")
}

// TestLegacyReproducer_EmptyCategoryEffortRules closes the coverage the corpus
// could not: the two documented EMPTY corpus categories (README "Explicitly
// EMPTY categories") are exercised by LOGIC against the documented emitter rule,
// since no golden bytes exist for them.
//
//   - codex/opencode "effort-only" (empty model + concrete effort): codex emits
//     model_reasoning_effort with NO model line; opencode DROPS the effort.
//   - cursor "effort with no model": the effort suffix is dropped (structurally
//     impossible to emit).
func TestLegacyReproducer_EmptyCategoryEffortRules(t *testing.T) {
	const effort = "high"
	effortOnly := HarnessPin{Model: "", Effort: effort}
	in := LegacyInputs{
		Harnesses: append([]string(nil), legacyHarnesses...),
		AgentPins: map[string]AgentPin{
			"status": {ByHarness: map[string]HarnessPin{
				"claude": effortOnly, "codex": effortOnly, "cursor": effortOnly, "opencode": effortOnly,
			}},
		},
	}
	rep := NewLegacyReproducer(in)

	render := func(harness string) string {
		ext := "md"
		if harness == "codex" {
			ext = "toml"
		}
		p := filepath.Join("/legacyroot", legacyAgentDirName[harness], "agents", "docket-status."+ext)
		got, ok := rep(Target{Path: p, Kind: KindFile, Role: roleAgent})
		if !ok {
			t.Fatalf("%s: reproducer refused an in-inventory target %s", harness, p)
		}
		return string(got)
	}

	// codex effort-only: model_reasoning_effort present, model absent.
	codex := render("codex")
	if !strings.Contains(codex, `model_reasoning_effort = "`+effort+`"`) {
		t.Errorf("codex effort-only: want a model_reasoning_effort line, got:\n%s", codex)
	}
	if strings.Contains(codex, "\nmodel = ") {
		t.Errorf("codex effort-only: unexpected model line:\n%s", codex)
	}

	// opencode effort-only: effort DROPPED (guarded by model presence), no model.
	oc := render("opencode")
	if strings.Contains(oc, "reasoningEffort:") {
		t.Errorf("opencode effort-only: effort must be dropped, got:\n%s", oc)
	}
	if strings.Contains(oc, "\nmodel:") {
		t.Errorf("opencode effort-only: unexpected model line:\n%s", oc)
	}

	// cursor effort-with-no-model: the [effort=…] suffix is dropped and no model
	// line is emitted.
	cur := render("cursor")
	if strings.Contains(cur, "[effort=") {
		t.Errorf("cursor effort-no-model: effort suffix must be dropped, got:\n%s", cur)
	}
	if strings.Contains(cur, "\nmodel:") {
		t.Errorf("cursor effort-no-model: unexpected model line:\n%s", cur)
	}
}

// firstDiff renders the first differing line between two byte slices.
func firstDiff(got, want []byte) string {
	gl := strings.SplitAfter(string(got), "\n")
	wl := strings.SplitAfter(string(want), "\n")
	n := len(gl)
	if len(wl) < n {
		n = len(wl)
	}
	for i := 0; i < n; i++ {
		if gl[i] != wl[i] {
			return fmt.Sprintf("first diff at line %d:\n  got:  %q\n  want: %q", i+1, gl[i], wl[i])
		}
	}
	if len(gl) != len(wl) {
		return fmt.Sprintf("line counts differ: got %d, want %d (got len=%d want len=%d)", len(gl), len(wl), len(got), len(want))
	}
	return "no line diff (byte-length equal? got vs want lengths differ)"
}
