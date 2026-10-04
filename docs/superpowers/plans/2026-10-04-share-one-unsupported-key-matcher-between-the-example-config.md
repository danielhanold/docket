<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0505 — Share one unsupported-key matcher between the example-config test and the docs guard](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0505-share-one-unsupported-key-matcher-between-the-example-config.md)**
<!-- docket:backlink:end -->
# Share one unsupported-key matcher Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use docket-build to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the two copies of the unsupported-config-key matcher, one in `internal/config` tests and one in `internal/repoguard` tests, with a single exported `config.UnsupportedKeyShapes`. In the same change, close the gap where a key inside a nested comment (`#   # terminal_publish: true`) went unmatched.

**Architecture:** A new non-test file `internal/config/unsupported_key_shapes.go` holds the registry-derived shape builder, next to `SettingPaths()`, the registry it reads. `TestExampleSchemaCorrespondence` (package `config`) and `TestLivingDocsAlignment` (package `repoguard`, which already imports `config`) both call it and delete their private copies. There is no new package: config's own internal test cannot import a package that imports `config`, so the code has to live in `config`. The two line-start shapes change their optional comment marker from `(?:#[ \t]*)?` to `(?:#[ \t]*)*`.

**Tech Stack:** Go (stdlib `regexp`, RE2 semantics, so `(?:#[ \t]*)*` cannot backtrack catastrophically), `go test`.

**Spec:** `docs/superpowers/specs/2026-10-04-share-one-unsupported-key-matcher-between-the-example-config-design.md` (on the `docket` branch; synchronized copy at `/Users/homer/dev/docket/.docket/docs/superpowers/specs/2026-10-04-share-one-unsupported-key-matcher-between-the-example-config-design.md`)

## Global Constraints

- No new package. The shared function lives in package `config`, file `internal/config/unsupported_key_shapes.go`.
- Exported surface, exactly: `type UnsupportedKeyShape struct { Name string; Re *regexp.Regexp }` and `func UnsupportedKeyShapes(paths []SettingPath) []UnsupportedKeyShape`.
- Regexps are compiled when `UnsupportedKeyShapes` is called. Never use a package-level `var … = regexp.MustCompile` for these shapes, because they must have no init-time cost in the binary.
- The derivation body is the existing one, byte-for-byte in logic. The only exception is the comment-marker quantifier in the two line-start shapes: `(?:#[ \t]*)?` becomes `(?:#[ \t]*)*`. Dotted-path shapes are unchanged.
  - top-level key shape: ``(?m)(?:^[ \t]*(?:#[ \t]*)*(?:-[ \t]+)?|`)<top>:``
  - mixed-block leaf shape: `(?m)(?:^[ \t]*(?:#[ \t]*)*|[{,][ \t]*)<leaf>:`
- Out of scope: which keys count as unsupported, the registry, citations, `maskCode`, the structural extractor (`exampleDocumentedKeys`), the agents-table check, refused values of supported keys. Do not touch them.
- Do not edit `.docket.example.yml` or any living doc. The spec states that no current file newly fails, and the guards running against the real files in Tasks 2 and 3 verify that claim. If either guard goes red on a real file, report BLOCKED with the matched line. Do not edit the doc.
- Cross-references in comments anchor on symbol names, never line numbers (ADR-0054, `TestCommentAnchorStyle`).
- Every mutation probe and every re-verification run uses `go test -count=1` (a cached `ok` is not evidence). Restore with a `cp` backup, never `git checkout --`. Before reading a probe's result, confirm with `git diff` that the mutation landed.
- Run `gofmt -l internal/config internal/repoguard` before each commit. The output must be empty.
- The build gate runs the whole suite through `build.test_command` (`go run ./cmd/docket development test`), not just the tests named here.

## Review Focus

1. **A nested-comment line that is legitimately supported or prose** (`#   # scope: any layer` in `.docket.example.yml`, `## Reconcile: …` / `### Readiness: …` headings in living docs) must stay unflagged. Task 1 pins `#   # scope: any layer`, `#   # gate: local`, and `## Reconcile: x` as negatives. Tasks 2 and 3 run the guards against the real files.
2. **A key spelled with a hyphen-prefixed list item inside a nested comment** (`#   # - skills: x`) should be flagged, the same as the un-nested `- skills:` form the top-level shape already admits. Task 1 adds this probe.
3. **A tab-separated nested comment** (`#\t#\tterminal_publish: true`) should be flagged, since `[ \t]` admits tabs. Task 1 adds this probe.
4. **A suffix-extended supported key** (`auto_groomable: true` next to the unsupported `auto_groom`) must stay unflagged after the quantifier widening. It is pinned as a Task 1 negative, including a nested-comment form `#   # auto_groomable: true`.
5. **An empty or all-supported registry** passed to `UnsupportedKeyShapes` should return no shapes and must not panic. Task 1 pins `UnsupportedKeyShapes(nil)` and an all-supported input as returning length 0.

---

### Task 1: Export `config.UnsupportedKeyShapes` with the nested-comment fix

**Files:**
- Create: `internal/config/unsupported_key_shapes.go`
- Test: `internal/config/unsupported_key_shapes_test.go` (create)

**Interfaces:**
- Consumes: `config.SettingPath{Path string; Supported bool}` and `config.SettingPaths() []SettingPath` (existing, `internal/config/schema.go`).
- Produces: `type UnsupportedKeyShape struct { Name string; Re *regexp.Regexp }` and `func UnsupportedKeyShapes(paths []SettingPath) []UnsupportedKeyShape`. `Name` is the display name (`"skills.build"`, `"skills:"`, `"skills.<child>"`, `"cap:"`). Tasks 2 and 3 read `.Name` and `.Re`.

- [ ] **Step 1: Write the failing test**

Create `internal/config/unsupported_key_shapes_test.go`:

```go
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
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test -count=1 ./internal/config/ -run '^TestUnsupportedKeyShapes$'`
Expected: build FAIL with `undefined: UnsupportedKeyShape` / `undefined: UnsupportedKeyShapes`.

- [ ] **Step 3: Write the implementation**

Create `internal/config/unsupported_key_shapes.go`:

```go
package config

import (
	"regexp"
	"strings"
)

// UnsupportedKeyShape is one registry-derived spelling of an unsupported
// configuration key: Name is its display name ("skills.build", "skills:",
// "skills.<child>", "cap:") and Re matches the spelling in raw text.
type UnsupportedKeyShape struct {
	Name string
	Re   *regexp.Regexp
}

// unsupportedKeySegClass stands in for a dynamic "*" path segment.
const unsupportedKeySegClass = `[A-Za-z0-9_<>*-]+`

// UnsupportedKeyShapes derives, from paths (normally SettingPaths()), the
// spellings of unsupported configuration keys that documentation must not
// carry. It derives four kinds of shape:
//
//   - each unsupported dotted path;
//   - for a top-level segment with no supported path beneath it, its YAML-key
//     form at a line start (behind any number of comment markers and an
//     optional list dash) or opening a code span;
//   - for the same segment, any dotted child of it;
//   - inside a block that also holds supported keys, an unsupported leaf's
//     YAML-key form at a line start (behind any number of comment markers) or
//     inside a flow mapping, when that leaf name is no segment of any
//     supported path. This restriction keeps `build:`, `review:`, and the
//     change-frontmatter `plan:` (also leaf names under skills.*) from
//     matching.
//
// TestExampleSchemaCorrespondence (Direction C) and repoguard's
// TestLivingDocsAlignment both call it. Regexps are compiled per call, so
// the function has no init-time cost.
func UnsupportedKeyShapes(paths []SettingPath) []UnsupportedKeyShape {
	supportedTop, supportedSeg := map[string]bool{}, map[string]bool{}
	for _, p := range paths {
		if p.Supported {
			segs := strings.Split(p.Path, ".")
			supportedTop[segs[0]] = true
			for _, s := range segs {
				supportedSeg[s] = true
			}
		}
	}
	var shapes []UnsupportedKeyShape
	seenTop, seenLeaf := map[string]bool{}, map[string]bool{}
	for _, p := range paths {
		if p.Supported {
			continue
		}
		segs := strings.Split(p.Path, ".")
		if len(segs) > 1 {
			parts := make([]string, len(segs))
			for i, s := range segs {
				if s == "*" {
					parts[i] = unsupportedKeySegClass
				} else {
					parts[i] = regexp.QuoteMeta(s)
				}
			}
			shapes = append(shapes, UnsupportedKeyShape{p.Path, regexp.MustCompile(`(?:^|[^\w.-])` + strings.Join(parts, `\.`) + `(?:[^\w-]|$)`)})
		}
		top := segs[0]
		if !supportedTop[top] && !seenTop[top] {
			seenTop[top] = true
			q := regexp.QuoteMeta(top)
			shapes = append(shapes, UnsupportedKeyShape{top + ":", regexp.MustCompile("(?m)(?:^[ \\t]*(?:#[ \\t]*)*(?:-[ \\t]+)?|`)" + q + ":")})
			if len(segs) > 1 {
				shapes = append(shapes, UnsupportedKeyShape{top + ".<child>", regexp.MustCompile(`(?:^|[^\w.-])` + q + `\.` + unsupportedKeySegClass)})
			}
		}
		leaf := segs[len(segs)-1]
		if len(segs) > 1 && supportedTop[top] && leaf != "*" && !supportedSeg[leaf] && !seenLeaf[leaf] {
			seenLeaf[leaf] = true
			shapes = append(shapes, UnsupportedKeyShape{leaf + ":", regexp.MustCompile(`(?m)(?:^[ \t]*(?:#[ \t]*)*|[{,][ \t]*)` + regexp.QuoteMeta(leaf) + `:`)})
		}
	}
	return shapes
}
```

Before going further, check the new file against the existing `exampleUnsupportedKeyShapes` body in `internal/config/example_correspondence_test.go`. The only intended difference besides names is the two `)?` → `)*` changes on the comment marker:

Run: `diff <(sed -n '/^func exampleUnsupportedKeyShapes/,/^}/p' internal/config/example_correspondence_test.go | sed 's/exampleKeyShape/UnsupportedKeyShape/g; s/exampleKeySegClass/unsupportedKeySegClass/g') <(sed -n '/^func UnsupportedKeyShapes/,/^}/p' internal/config/unsupported_key_shapes.go)`
Expected: only the signature line, the `paths := SettingPaths()` line (removed; now a parameter), and the two line-start regexp lines (`(?:#[ \\t]*)?` → `(?:#[ \\t]*)*` and `(?:#[ \t]*)?` → `(?:#[ \t]*)*`) differ.

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test -count=1 ./internal/config/ -run '^TestUnsupportedKeyShapes$' -v`
Expected: PASS.

- [ ] **Step 5: Mutation-test the new asserts**

Each probe: back up, mutate, confirm the diff landed, run, restore.

(a) Revert the top-level shape's `*` to `?`. The nested probes `#   # terminal_publish: true`, `# # skills:`, `#   # - skills: x`, `#\t#\tterminal_publish: true` and the nested population probes must go red:

```bash
f=internal/config/unsupported_key_shapes.go; cp "$f" "$f.bak"
perl -pi -e 's/\(\?:\^\[ \\\\t\]\*\(\?:#\[ \\\\t\]\*\)\*\(\?:-/(?:^[ \\\\t]*(?:#[ \\\\t]*)?(?:-/' "$f"
cmp -s "$f" "$f.bak" && echo UNCHANGED   # must print nothing; if it prints UNCHANGED the mutation did not land, so fix the substitution before reading the run
go test -count=1 ./internal/config/ -run '^TestUnsupportedKeyShapes$'   # expect FAIL
mv -f "$f.bak" "$f"
```

(b) Revert the leaf shape's `*` to `?`: after taking a `cp` backup, change the `(?:^[ \t]*(?:#[ \t]*)*|[{,]` substring to `(?:^[ \t]*(?:#[ \t]*)?|[{,]` by hand. `#   #   cap: 5` must go red. Confirm with `cmp`, run, then `mv -f` the backup back.

(c) Shape-family deletion, two separate probes, each from a fresh `cp` backup:
  - Delete the dotted-path line `shapes = append(shapes, UnsupportedKeyShape{p.Path, …})`. The population probe must go red for `learnings.cap`. Its top segment `learnings` is supported, so no top or child shape covers it, and its leaf shape needs a trailing colon that the backticked probe lacks.
  - Delete the top-level line `shapes = append(shapes, UnsupportedKeyShape{top + ":", …})`. The population probe must go red for `terminal_publish` and `auto_groom`.
  Restore with `mv -f` after each.

The file is untracked during this task, so `git diff` cannot show the mutation. Instead, confirm each mutation landed by comparing the file with its backup (`cmp -s "$f" "$f.bak" && echo UNCHANGED`, which must print nothing) before running the test. After all probes, `cmp` against the last backup shows the file is restored, and the Step 4 command passes again.

- [ ] **Step 6: Commit**

```bash
gofmt -l internal/config   # expect no output
git add internal/config/unsupported_key_shapes.go internal/config/unsupported_key_shapes_test.go
git commit -m "refactor(config): export UnsupportedKeyShapes and match keys in nested comments"
```

---

### Task 2: Point `TestExampleSchemaCorrespondence` at the shared matcher

**Files:**
- Modify: `internal/config/example_correspondence_test.go` (`directionCViolations`; delete `exampleKeyShape`, `exampleKeySegClass`, `exampleUnsupportedKeyShapes`; the `non_vacuity` subtest's Direction C plants and the shape-population check)

**Interfaces:**
- Consumes: `UnsupportedKeyShapes(paths []SettingPath) []UnsupportedKeyShape` with fields `.Name`, `.Re` (Task 1).
- Produces: nothing new. `directionCViolations(content string) []string` keeps its signature.

- [ ] **Step 1: Add the failing nested-comment plant**

In `TestExampleSchemaCorrespondence`'s `non_vacuity` subtest, extend the map literal under the comment "Direction C reaches past the structural extractor" with a third entry:

```go
			"nested-comment key": content + "\n#   # terminal_publish: true\n",
```

The full map literal then reads:

```go
		for name, planted := range map[string]string{
			"runner in commented agents table": strings.Replace(content, "#   claude:\n", "#   claude:\n#     adr: { model: x, runner: codex }\n", 1),
			"commented key after a prose line": content + "\n# scope: any layer\n# An explanatory line about the next key.\n# terminal_publish: true\n",
			"nested-comment key":               content + "\n#   # terminal_publish: true\n",
		} {
```

Extend the comment above it to name the third case:

```go
		// Direction C reaches past the structural extractor: an unsupported leaf
		// inside the commented agents table, a commented unsupported key that
		// follows an explanatory line instead of its scope tag, and a key
		// commented out inside an already-commented block.
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test -count=1 ./internal/config/ -run '^TestExampleSchemaCorrespondence$'`
Expected: FAIL with `Direction C missed the plant: nested-comment key`. The private copy still has the single `?` marker, and the structural extractor's `commentedKeyRe` does not admit a second `#`.

- [ ] **Step 3: Repoint and delete the copy**

In `directionCViolations`, replace the loop header and field reads:

```go
	for _, s := range UnsupportedKeyShapes(SettingPaths()) {
		for _, m := range s.Re.FindAllStringIndex(content, -1) {
			line := strings.Count(content[:m[0]], "\n") + 1
			out = append(out, fmt.Sprintf("line %d: unsupported key %s (%q)", line, s.Name, strings.TrimSpace(content[m[0]:m[1]])))
		}
	}
```

Delete, in full: the `type exampleKeyShape struct {…}` declaration, the `const exampleKeySegClass = …` line, and the whole `exampleUnsupportedKeyShapes` function together with its doc comment, including the "It mirrors repoguard's unsupportedKeyShapes (the living docs guard, which cannot be imported here without an import cycle)" text.

In the `non_vacuity` subtest, change the population check's emptiness guard:

```go
		if len(UnsupportedKeyShapes(SettingPaths())) == 0 {
			t.Fatalf("no unsupported-key shapes derived from the schema registry")
		}
```

Leave the guard's own registry-derived population loop that follows it (the `for _, p := range SettingPaths()` probe through `directionCViolations`) unchanged.

The imports stay the same: `regexp` is still used by `activeKeyRe` and the agents-table regexps, and `fmt` is still used. If the compiler reports an unused import, remove only that one.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -count=1 ./internal/config/ -run '^(TestExampleSchemaCorrespondence|TestUnsupportedKeyShapes)$' -v`
Expected: PASS. This run also checks the spec's claim that the real `.docket.example.yml` does not newly fail. If Direction C reports a line from the real file, stop and report BLOCKED with that line. Do not edit the example.

Confirm the copy is gone:
Run: `out=$(grep -rnE -e 'exampleUnsupportedKeyShapes|exampleKeyShape|exampleKeySegClass' internal/ || true); echo "${out:-none}"`
Expected: `none`.

- [ ] **Step 5: Mutation-test the plant**

```bash
f=internal/config/unsupported_key_shapes.go; cp "$f" "$f.bak"
perl -pi -e 's/\(\?:\^\[ \\\\t\]\*\(\?:#\[ \\\\t\]\*\)\*\(\?:-/(?:^[ \\\\t]*(?:#[ \\\\t]*)?(?:-/' "$f"
git diff --stat -- "$f"   # must show 1 line changed
go test -count=1 ./internal/config/ -run '^TestExampleSchemaCorrespondence$'   # expect FAIL: Direction C missed the plant: nested-comment key
mv -f "$f.bak" "$f"
git diff --stat -- "$f"   # must be empty
```

- [ ] **Step 6: Commit**

```bash
gofmt -l internal/config   # expect no output
git add internal/config/example_correspondence_test.go
git commit -m "refactor(config): example-config guard uses the shared unsupported-key matcher"
```

---

### Task 3: Point `TestLivingDocsAlignment` at the shared matcher

**Files:**
- Modify: `internal/repoguard/docs_alignment_test.go` (`TestLivingDocsAlignment` and its `# Limits` doc comment; `scanUnsupportedKeys`; delete `keyShape`, `keySegClass`, `unsupportedKeyShapes`)

**Interfaces:**
- Consumes: `config.UnsupportedKeyShapes(paths []config.SettingPath) []config.UnsupportedKeyShape` with fields `.Name`, `.Re` (Task 1).
- Produces: `scanUnsupportedKeys(rel, content string, shapes []config.UnsupportedKeyShape) []string` (parameter type changes; same behaviour).

- [ ] **Step 1: Add the failing nested-comment probe**

In `TestLivingDocsAlignment`'s `non_vacuity` subtest, add `"#   # terminal_publish: true"` to the flagged list (the slice that starts `"bind \`skills.build\` to", …`). It becomes:

```go
		for _, s := range []string{
			"bind `skills.build` to", "raise learnings.cap", "set `skills:` to",
			"skills:\n  build: x\n", "  terminal_publish: true", "# auto_groom: false",
			"#   # terminal_publish: true",
			"agents.claude.build-max.runner", "{ model: x, runner: codex }",
			"runners.codex.shim_model", "`dummy_mode.persona`", "build:\n  checkpoint: true\n",
			"the skills.<role> map",
		} {
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test -count=1 ./internal/repoguard/ -run '^TestLivingDocsAlignment$'`
Expected: FAIL with `unsupported key not flagged: "#   # terminal_publish: true"`.

- [ ] **Step 3: Repoint and delete the copy**

In `TestLivingDocsAlignment`, replace the shape derivation:

```go
	shapes := config.UnsupportedKeyShapes(config.SettingPaths())
```

Change `scanUnsupportedKeys` to:

```go
// scanUnsupportedKeys reports every unsupported config key spelled in content
// (code included: config examples live in code).
func scanUnsupportedKeys(rel, content string, shapes []config.UnsupportedKeyShape) []string {
	var v []string
	for _, s := range shapes {
		for _, m := range s.Re.FindAllStringIndex(content, -1) {
			v = append(v, fmt.Sprintf("%s:%d: unsupported config key %s (%q) — describe only supported settings", rel, lineOf(content, m[0]), s.Name, strings.TrimSpace(content[m[0]:m[1]])))
		}
	}
	return v
}
```

Delete, in full: `type keyShape struct {…}`, `const keySegClass = …`, and the `unsupportedKeyShapes` function together with its doc comment. Leave `citationShapes` (its own anonymous struct), `codeSpanRe`, `maskCode`, `lineOf`, and `scanCitations` untouched. `regexp` stays imported (used by `citationShapes` and `codeSpanRe`).

Update the key bullet of the `# Limits` doc comment on `TestLivingDocsAlignment`. Replace the bullet that begins `//   - A key is matched as a dotted path` with:

```go
//   - A key is matched as a dotted path (`skills.build`, `learnings.cap`,
//     `agents.<h>.<a>.runner`), a dotted child of an all-unsupported block
//     (`skills.<role>`), a YAML key at a line start — commented, or commented
//     inside an already-commented block — or opening a code span (`skills:`,
//     `# terminal_publish:`, `#   # terminal_publish:`), or — inside a block
//     that also holds supported keys — an unsupported leaf key at a line start
//     (commented or nested-commented) or inside a flow mapping (`checkpoint:`,
//     `cap:`, `{ …, runner: … }`). The shapes come from
//     config.UnsupportedKeyShapes, shared with the example-config guard. A bare
//     mention without the dot or colon (`terminal_publish` in prose) is not
//     matched, and a wrapper's own `skills` frontmatter field must be described
//     without the `skills:` spelling. A markdown heading spelled
//     `## <unsupported-key>:` is matched too.
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -count=1 ./internal/repoguard/ -run '^(TestLivingDocsAlignment|TestCommentAnchorStyle)$' -v`
Expected: PASS. This run also checks that no real living doc newly fails under the widened shapes. If a real file is reported, stop and report BLOCKED with that `file:line`. Do not edit the doc.

Confirm the copy is gone:
Run: `out=$(grep -rnE -e '\bunsupportedKeyShapes\b|\bkeyShape\b|\bkeySegClass\b' internal/ || true); echo "${out:-none}"`
Expected: `none`. (`UnsupportedKeyShapes` with a capital U must not match because of `\b` plus case. If ugrep's `\b` handling makes the result ambiguous, re-run with `/usr/bin/grep -rnwE -e 'unsupportedKeyShapes|keyShape|keySegClass' internal/`.)

- [ ] **Step 5: Mutation-test the probe**

```bash
f=internal/config/unsupported_key_shapes.go; cp "$f" "$f.bak"
perl -pi -e 's/\(\?:\^\[ \\\\t\]\*\(\?:#\[ \\\\t\]\*\)\*\(\?:-/(?:^[ \\\\t]*(?:#[ \\\\t]*)?(?:-/' "$f"
git diff --stat -- "$f"   # must show 1 line changed
go test -count=1 ./internal/repoguard/ -run '^TestLivingDocsAlignment$'   # expect FAIL: unsupported key not flagged: "#   # terminal_publish: true"
mv -f "$f.bak" "$f"
git diff --stat -- "$f"   # must be empty
```

- [ ] **Step 6: Run both packages together**

Run: `go test -count=1 ./internal/config/ ./internal/repoguard/`
Expected: `ok` for both (not `(cached)`).

- [ ] **Step 7: Commit**

```bash
gofmt -l internal/repoguard   # expect no output
git add internal/repoguard/docs_alignment_test.go
git commit -m "refactor(repoguard): living-docs guard uses the shared unsupported-key matcher"
```
