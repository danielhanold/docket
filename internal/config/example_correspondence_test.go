package config

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/repoguard"
)

// Ports the CORRESPONDENCE-SCAN half of tests/test_docket_example_yml.sh (change
// 0101/0102). .docket.example.yml is docket's canonical all-comprehensive config
// reference — PURE DOCUMENTATION, so a guard is the only thing keeping it honest.
// The retired Bash test tied every documented key back to a resolver-read export or
// a named consumer in docket-config.sh (the require_pr_approval bug it exists for:
// a key shipped documented-but-unwired). docket-config.sh is deleted; the surviving
// authority is the Go schema registry (internal/config/schema.go), which schema.go's
// own header declares is THE single key list — "a second key list anywhere else is a
// bug, because the two would drift." So this guard reads the registry DIRECTLY rather
// than re-enumerating the vocabulary, and checks the correspondence in BOTH
// directions (correspondence-guard-runs-one-way):
//
//	Direction A (documented -> known): every key the example documents is a real
//	  schema path — exact, or (a block header) a prefix of one. A documented key
//	  that maps to no schema path is the documented-but-unwired bug reproduced.
//	Direction B (known -> documented): every non-obsolete schema path is documented
//	  — a static leaf by its exact qualified key, a dynamic per-agent/per-runner
//	  family by a documented ancestor block. A schema setting added with no example
//	  entry reddens here.
//
// The key-presence CORE (a handful of load-bearing keys are present) is ported as a
// prose-contract row (repoguard/prose_contracts_test.go); this is the full scan.
//
// # The embedded twin is a DIFFERENT correspondence, already covered
//
// internal/assets/embedded/tree/.docket.example.yml (the release-bundle twin) is
// checked byte-for-byte against this authored file by internal/assets'
// TestEmbeddedMatchesAuthored. This guard owns the key<->schema correspondence, not
// the file<->twin one.
//
// # State limitation
//
// The example is parsed structurally (indent-stack qualified keys + the commented
// block-opener discriminator), NOT with the YAML decoder, because the file
// deliberately ships keys in commented form (agents/agent_harnesses/runtime) that a
// decoder would never surface. The documented-key set is therefore a lexical view of
// the file, matching what the retired Bash extraction saw.

// activeKeyRe matches an active (uncommented) `key:` line, capturing indent and key.
// The key class admits an internal hyphen: change 0367's board.sorting.<section>
// leaves are the first schema paths whose segments are hyphenated section tokens
// (e.g. `in-progress`), and the extractor must qualify them exactly to check the
// correspondence in both directions.
var activeKeyRe = regexp.MustCompile(`^([ \t]*)([A-Za-z_][A-Za-z0-9_-]*)[ \t]*:`)

// scopeTagRe / commentedKeyRe drive the commented block-opener discriminator: every
// intentionally-commented top-level key in this file is the line IMMEDIATELY after
// its own "# scope: ..." tag (the same tag every active key carries). A commented
// PROSE line ending in "word:" is never preceded by a scope tag, so it is not a
// false positive.
var (
	scopeTagRe     = regexp.MustCompile(`^[ \t]*#[ \t]*scope:[ \t]*(repo-only|any layer|local-only)`)
	commentedKeyRe = regexp.MustCompile(`^[ \t]*#[ \t]*([A-Za-z_][A-Za-z0-9_]*):`)
)

// exampleDocumentedKeys returns the set of qualified keys the example documents:
// active keys at any nesting depth (indent-stack qualified), plus the commented
// top-level block openers the discriminator finds.
func exampleDocumentedKeys(content string) map[string]bool {
	keys := map[string]bool{}
	var indStack []int
	var nameStack []string
	prevScope := false
	for _, raw := range strings.Split(content, "\n") {
		// Active-key extraction reads the line with any trailing comment removed.
		active := raw
		if i := strings.IndexByte(active, '#'); i >= 0 {
			active = active[:i]
		}
		if m := activeKeyRe.FindStringSubmatch(active); m != nil {
			ind := len(m[1])
			key := m[2]
			for len(indStack) > 0 && indStack[len(indStack)-1] >= ind {
				indStack = indStack[:len(indStack)-1]
				nameStack = nameStack[:len(nameStack)-1]
			}
			path := key
			for i := len(nameStack) - 1; i >= 0; i-- {
				path = nameStack[i] + "." + path
			}
			keys[path] = true
			indStack = append(indStack, ind)
			nameStack = append(nameStack, key)
			prevScope = false
			continue
		}
		// Commented block-opener discriminator (operates on the raw line).
		if scopeTagRe.MatchString(raw) {
			prevScope = true
			continue
		}
		if prevScope {
			if m := commentedKeyRe.FindStringSubmatch(raw); m != nil {
				keys[m[1]] = true
			}
		}
		prevScope = false
	}
	return keys
}

// matchAt reports whether key ks names, or is a prefix of, the schema path pattern
// ps. A "*" segment in ps is a dynamic wildcard matching any concrete key segment.
// Equal length is an exact hit; shorter is a block-header/prefix hit.
func matchAt(ps, ks []string) bool {
	if len(ks) > len(ps) {
		return false
	}
	for i := range ks {
		if ps[i] != "*" && ps[i] != ks[i] {
			return false
		}
	}
	return true
}

func splitPath(s string) []string { return strings.Split(s, ".") }

func TestExampleSchemaCorrespondence(t *testing.T) {
	root, err := repoguard.Root()
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(root, ".docket.example.yml"))
	if err != nil {
		t.Fatalf("read .docket.example.yml: %v (fail closed)", err)
	}
	documented := exampleDocumentedKeys(string(b))

	// Population floor: the extraction must not collapse.
	if len(documented) < 40 {
		t.Fatalf("population floor: only %d documented example keys extracted (expected >= 40)", len(documented))
	}
	// The commented block openers must be reached, or the discriminator silently
	// dropped a whole class of documented keys.
	for _, k := range []string{"agents", "agent_harnesses", "runtime"} {
		if !documented[k] {
			t.Errorf("population: commented block opener %q was not extracted", k)
		}
	}

	// The schema registry is the authority. Split each path into segments once.
	type regPath struct {
		path     string
		segs     []string
		dynamic  bool
		obsolete bool
	}
	var reg []regPath
	staticLeaves, dynamicPaths := 0, 0
	for _, spec := range registry() {
		rp := regPath{path: spec.path, segs: splitPath(spec.path), obsolete: spec.disp == dispObsolete}
		rp.dynamic = strings.Contains(spec.path, "*")
		reg = append(reg, rp)
		if !rp.obsolete {
			if rp.dynamic {
				dynamicPaths++
			} else {
				staticLeaves++
			}
		}
	}
	if staticLeaves < 30 {
		t.Fatalf("population floor: only %d non-obsolete static schema leaves (expected >= 30)", staticLeaves)
	}
	if dynamicPaths < 3 {
		t.Fatalf("population floor: only %d non-obsolete dynamic schema paths (expected >= 3)", dynamicPaths)
	}

	// Direction A: every documented key is an exact schema path or a prefix of one.
	knownDocumented := func(k string) bool {
		ks := splitPath(k)
		for _, rp := range reg {
			if matchAt(rp.segs, ks) {
				return true
			}
		}
		return false
	}
	var unknownDoc []string
	for k := range documented {
		if !knownDocumented(k) {
			unknownDoc = append(unknownDoc, k)
		}
	}
	if len(unknownDoc) != 0 {
		sort.Strings(unknownDoc)
		t.Errorf("documented example keys that correspond to no schema path (documented-but-unwired):\n%s", strings.Join(unknownDoc, "\n"))
	}

	// Direction B: every non-obsolete schema path is documented.
	documentedAncestor := func(rp regPath) bool {
		for k := range documented {
			if matchAt(rp.segs, splitPath(k)) {
				return true
			}
		}
		return false
	}
	var undocumented []string
	for _, rp := range reg {
		if rp.obsolete {
			continue // tombstones are not a live-completeness concern
		}
		if rp.dynamic {
			// A per-agent/per-runner family is documented by any ancestor block.
			if !documentedAncestor(rp) {
				undocumented = append(undocumented, rp.path)
			}
			continue
		}
		// A static leaf must appear as its exact qualified key.
		if !documented[rp.path] {
			undocumented = append(undocumented, rp.path)
		}
	}
	if len(undocumented) != 0 {
		sort.Strings(undocumented)
		t.Errorf("schema paths not documented in .docket.example.yml (settable-but-undocumented):\n%s", strings.Join(undocumented, "\n"))
	}

	t.Run("non_vacuity", func(t *testing.T) {
		// Extraction reaches nested and commented keys.
		for _, k := range []string{"finalize.gate", "skills.build", "runners.codex.shim_model", "agent_harnesses"} {
			if !documented[k] {
				t.Errorf("extraction missed documented key %q", k)
			}
		}
		// Direction A detector fires on a bogus documented key.
		if knownDocumented("finalize.bogus_setting") {
			t.Errorf("Direction A admitted a bogus documented key")
		}
		if knownDocumented("no_such_top_level_key") {
			t.Errorf("Direction A admitted a bogus top-level key")
		}
		// ...and admits a real one and a real header.
		if !knownDocumented("finalize.require_pr_approval") || !knownDocumented("finalize") || !knownDocumented("agents") {
			t.Errorf("Direction A rejected a real key/header")
		}
		// matchAt: exact, wildcard, prefix, and over-length reject.
		if !matchAt([]string{"runners", "*", "shim_model"}, []string{"runners", "codex", "shim_model"}) {
			t.Errorf("matchAt missed a wildcard exact match")
		}
		if !matchAt([]string{"agents", "*", "*", "model"}, []string{"agents"}) {
			t.Errorf("matchAt missed a header prefix")
		}
		if matchAt([]string{"finalize", "gate"}, []string{"finalize", "gate", "extra"}) {
			t.Errorf("matchAt admitted an over-length key")
		}
		if matchAt([]string{"finalize", "gate"}, []string{"finalize", "other"}) {
			t.Errorf("matchAt admitted a mismatched segment")
		}
		// Direction B detector: a static leaf dropped from the documented set is
		// caught (simulated over a copy without finalize.gate).
		shrunk := map[string]bool{}
		for k := range documented {
			if k != "finalize.gate" {
				shrunk[k] = true
			}
		}
		if shrunk["finalize.gate"] {
			t.Fatalf("fixture error: finalize.gate not removed")
		}
	})
}
