package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/repoguard"
)

// .docket.example.yml is docket's one-place configuration reference. It is
// documentation that nothing reads, so this guard is the only thing keeping it
// honest. The authority is the schema registry (internal/config/schema.go); the
// guard reads it directly rather than re-listing the vocabulary, and checks:
//
//	Direction A (documented -> known): every key the example documents is a
//	  schema path, exact or (a block header) a prefix of one.
//	Direction B (supported -> documented): every supported schema path is
//	  documented: a static leaf by its exact key, a dynamic per-harness family
//	  by a documented ancestor block.
//	Direction C (unsupported -> absent): no unsupported path is documented as an
//	  active or scope-tagged key.
//	D (copy safety): a verbatim copy of the example as .docket.yml resolves with
//	  no warning, no write blocker, and effective values equal to the built-in
//	  defaults.
//
// The key-presence core is a prose-contract row
// (repoguard/prose_contracts_test.go); this is the full scan.
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
// deliberately ships keys in commented form (agents/agent_harnesses) that a
// decoder would never surface. The documented-key set is therefore a lexical view
// of the file.

// activeKeyRe matches an active (uncommented) `key:` line, capturing indent and key.
// The key class admits an internal hyphen: the board.sorting.<section> leaves
// have hyphenated section segments (e.g. `in-progress`), and the extractor must
// qualify them exactly to check the correspondence.
var activeKeyRe = regexp.MustCompile(`^([ \t]*)([A-Za-z_][A-Za-z0-9_-]*)[ \t]*:`)

// scopeTagRe / commentedKeyRe drive the commented block-opener discriminator: every
// intentionally-commented top-level key in this file is the line IMMEDIATELY after
// its own "# scope: ..." tag (the same tag every active key carries). A commented
// PROSE line ending in "word:" is never preceded by a scope tag, so it is not a
// false positive.
var (
	scopeTagRe     = regexp.MustCompile(`^[ \t]*#[ \t]*scope:[ \t]*(repo-only|any layer|global-only)`)
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

// undocumentedSupported returns every supported registry path the documented
// set does not cover: a static path by its exact key, a dynamic path by any
// documented ancestor.
func undocumentedSupported(documented map[string]bool) []string {
	var out []string
	for _, spec := range registry() {
		if !dispositionSupported(spec.disp) {
			continue
		}
		segs := splitPath(spec.path)
		if strings.Contains(spec.path, "*") {
			hit := false
			for k := range documented {
				if matchAt(segs, splitPath(k)) {
					hit = true
					break
				}
			}
			if !hit {
				out = append(out, spec.path)
			}
			continue
		}
		if !documented[spec.path] {
			out = append(out, spec.path)
		}
	}
	sort.Strings(out)
	return out
}

// documentedUnsupported returns every documented key that names only
// unsupported registry paths (exactly or as a block header), such as `skills`
// or `learnings.cap`. A key that also heads a supported path (`agents`,
// `finalize`) is not flagged.
func documentedUnsupported(documented map[string]bool) []string {
	var out []string
	for k := range documented {
		ks := splitPath(k)
		matched, supported := false, false
		for _, spec := range registry() {
			if matchAt(splitPath(spec.path), ks) {
				matched = true
				if dispositionSupported(spec.disp) {
					supported = true
					break
				}
			}
		}
		if matched && !supported {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// verbatimCopyProblems resolves content as a committed .docket.yml on its own
// and reports what would make a verbatim copy unsafe: a resolve error, a
// warning or error diagnostic, a mutation-preflight blocker, or effective values
// that differ from the built-in defaults.
func verbatimCopyProblems(content string) []string {
	ctx := ResolveContext{DefaultBranch: "main"}
	snap, diags, err := Resolve([]Source{{Layer: LayerRepository, Name: ".docket.yml", Data: []byte(content)}}, ctx)
	if err != nil {
		return []string{fmt.Sprintf("resolve: %v (diagnostics %+v)", err, diags)}
	}
	var problems []string
	for _, d := range snap.Diagnostics {
		if d.Severity == SeverityWarning || d.Severity == SeverityError {
			problems = append(problems, fmt.Sprintf("diagnostic %s at %s: %s", d.Code, d.Path, d.Message))
		}
	}
	if dec := PreflightMutation(snap); !dec.Allowed {
		for _, b := range dec.Blockers {
			problems = append(problems, "mutation blocker: "+b.Path)
		}
	}
	base, _, err := Resolve(nil, ctx)
	if err != nil {
		return append(problems, fmt.Sprintf("resolve built-ins: %v", err))
	}
	if got, want := effectiveValuesOnly(snap.Effective), effectiveValuesOnly(base.Effective); !reflect.DeepEqual(got, want) {
		problems = append(problems, fmt.Sprintf("effective values differ from built-in defaults:\n got: %v\nwant: %v", got, want))
	}
	return problems
}

// effectiveValuesOnly renders an Effective through JSON and drops every
// provenance and explicit marker, leaving only resolved values.
func effectiveValuesOnly(e Effective) any {
	b, err := json.Marshal(e)
	if err != nil {
		return "marshal: " + err.Error()
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return "unmarshal: " + err.Error()
	}
	return dropProvenance(v)
}

func dropProvenance(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			if k == "provenance" || k == "explicit" {
				continue
			}
			out[k] = dropProvenance(val)
		}
		return out
	case []any:
		for i := range x {
			x[i] = dropProvenance(x[i])
		}
		return x
	}
	return v
}

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
	if len(documented) < 50 {
		t.Fatalf("population floor: only %d documented example keys extracted (expected >= 50)", len(documented))
	}
	// The commented block openers must be reached, or the discriminator silently
	// dropped a whole class of documented keys.
	for _, k := range []string{"agents", "agent_harnesses"} {
		if !documented[k] {
			t.Errorf("population: commented block opener %q was not extracted", k)
		}
	}

	// The schema registry is the authority. Split each path into segments once.
	type regPath struct {
		segs []string
	}
	var reg []regPath
	staticLeaves, dynamicPaths := 0, 0
	for _, spec := range registry() {
		reg = append(reg, regPath{segs: splitPath(spec.path)})
		if !dispositionSupported(spec.disp) {
			continue
		}
		if strings.Contains(spec.path, "*") {
			dynamicPaths++
		} else {
			staticLeaves++
		}
	}
	if staticLeaves < 35 {
		t.Fatalf("population floor: only %d supported static schema leaves (expected >= 35)", staticLeaves)
	}
	if dynamicPaths < 2 {
		t.Fatalf("population floor: only %d supported dynamic schema paths (expected >= 2)", dynamicPaths)
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

	// Direction B: every supported schema path is documented.
	if u := undocumentedSupported(documented); len(u) != 0 {
		t.Errorf("supported schema paths not documented in .docket.example.yml (settable-but-undocumented):\n%s", strings.Join(u, "\n"))
	}

	// Direction C: no unsupported path is documented.
	if u := documentedUnsupported(documented); len(u) != 0 {
		t.Errorf("unsupported keys documented in .docket.example.yml:\n%s", strings.Join(u, "\n"))
	}

	// D: a verbatim copy as .docket.yml is safe.
	if p := verbatimCopyProblems(string(b)); len(p) != 0 {
		t.Errorf("a verbatim copy of .docket.example.yml as .docket.yml is unsafe:\n%s", strings.Join(p, "\n"))
	}

	t.Run("non_vacuity", func(t *testing.T) {
		for _, k := range []string{"finalize.gate", "finalize.repair_max_attempts", "run.max_attempts", "board.sorting.in-progress.by", "agent_harnesses", "agents"} {
			if !documented[k] {
				t.Errorf("extraction missed documented key %q", k)
			}
		}
		if knownDocumented("finalize.bogus_setting") || knownDocumented("no_such_top_level_key") {
			t.Errorf("Direction A admitted a bogus key")
		}
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
		// Direction B: strip a supported key -> reported.
		shrunk := map[string]bool{}
		for k := range documented {
			if k != "finalize.gate" {
				shrunk[k] = true
			}
		}
		if u := undocumentedSupported(shrunk); len(u) != 1 || u[0] != "finalize.gate" {
			t.Errorf("Direction B missed a stripped supported key: %v", u)
		}
		// Direction C: plant refused keys -> reported, via the real extractor.
		planted := exampleDocumentedKeys(string(b) + "\n# scope: any layer\nskills:\n  build: docket-build\nlearnings:\n  cap: 300\n")
		got := strings.Join(documentedUnsupported(planted), ",")
		for _, want := range []string{"skills", "skills.build", "learnings.cap"} {
			if !strings.Contains(","+got+",", ","+want+",") {
				t.Errorf("Direction C missed planted %q (got %s)", want, got)
			}
		}
		// D: a blocking active line makes the verbatim copy unsafe.
		if p := verbatimCopyProblems(string(b) + "\nterminal_publish: true\n"); len(p) == 0 {
			t.Errorf("verbatim-copy check admitted a blocking line")
		}
		if p := verbatimCopyProblems(string(b) + "\nskills:\n  build: docket-build\n"); len(p) == 0 {
			t.Errorf("verbatim-copy check admitted a skills binding")
		}
	})
}
