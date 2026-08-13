# Loss-Preserving Document Layer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A standalone `internal/document` package that reads Docket Markdown records as typed YAML without surrendering their bytes, applies validate-all loss-preserving field and managed-block patches, renders canonical new documents, and proves compatibility against a frozen `v0.9.2` corpus with seeded fuzz targets.

**Architecture:** A parsed document is two coordinated views: an immutable copy of the exact source bytes with half-open byte spans for frontmatter fields and managed blocks, plus a semantic YAML tree used only for typed decoding and shape classification. Existing documents are never emitted through a YAML encoder — a patch validates the complete edit set, replaces only the spans it owns from the highest offset downward, and reparses the candidate before returning bytes. One shared typed-value serializer feeds both field patches and canonical new-document rendering.

**Tech Stack:** Go 1.26, `go.yaml.in/yaml/v3` pinned at `v3.0.4` (already a direct dependency — this change adds **no** new dependency).

**Spec:** `.docket/docs/superpowers/specs/2026-08-13-loss-preserving-document-layer-design.md` (read it before any task; this plan argues from it).

## Global Constraints

- YAML library is exactly `go.yaml.in/yaml/v3 v3.0.4` — never `gopkg.in/yaml.v3`, never a v4 RC. YAML-library node types never cross the package boundary (no exported field, parameter, or return of type `*yaml.Node`).
- Never call `yaml.Marshal`, `yaml.Encoder`, or node re-encoding for an existing document. The byte locator is authoritative for every edit; YAML node line/column data is used only for shape classification and diagnostics.
- `internal/document` performs no filesystem, Git, network, commit, or transaction write. Callers pass bytes in and receive bytes out. It imports nothing from `internal/app`, `internal/cli`, `internal/config`, or `internal/buildinfo`.
- `Document` and every span it exposes are immutable values. `Parse` copies its input; returned source and patched bytes never alias caller-owned mutable buffers (each return is a fresh slice).
- On every error `Apply` returns `nil` bytes — no best-effort partial result. A defect in a later batch item leaves the whole input unchanged (validate-all before the first byte is constructed).
- Every error is a `*document.Error` with a stable package-local `Kind` from the closed set defined in Task 1. No other kind may be minted.
- New string values are always single-quoted with interior apostrophes doubled. Keys must match `[a-z][a-z0-9_]*`. Field string values and managed-block content must be valid UTF-8 with no NUL; block content additionally allows LF and tab; inline field strings reject all control characters except tab.
- Every mutation probe and manual re-verification runs `go test -count=1` — the Go test cache serves stale verdicts otherwise (learning `cached-runner-serves-a-mutated-tree`).
- Frozen fixtures under `testdata/repositories/v0.9.2/` are immutable inputs; the tree-wide `PROVENANCE.md` gains one line per fixture added (its stated convention). Tests copy before mutating and never write inside `testdata/`.
- `gofmt`-clean, `go vet`-clean — `tests/test_go_toolchain.sh` gates both for the whole suite; no Bash production change and no new `tests/test_*.sh` file, so no `tests/runtime-budgets.tsv` row is added. Watch the existing `test_go_toolchain.sh` 20s budget row: if `--timings` shows the added Go tests pushing it over, report it in the results file rather than silently re-budgeting.
- Validate the whole input set first: `Apply` and the builder validate every element of their ordered input before acting on any (learning `validate-the-whole-input-set-first`).

## File Structure

```
internal/document/document.go        # package doc, Document/Span types, Parse orchestration, DecodeFrontmatter
internal/document/errors.go          # Error type + closed Kind set
internal/document/frontmatter.go     # source-line scan, fence discovery, field locator
internal/document/markers.go         # managed-block marker scan (code-fence-aware) + population validation
internal/document/value.go           # closed typed value model + shared serializer
internal/document/patch.go           # PatchSet, edit validation, span replacement, candidate reparse
internal/document/builder.go         # canonical new-document renderer
internal/document/*_test.go          # one test file per source file, same base name
internal/document/fuzz_test.go       # the four fuzz targets + seed wiring
internal/document/testdata/          # package-local adversarial fixtures + new-document goldens
testdata/repositories/v0.9.2/documents/   # frozen real-record corpus (Task 10)
testdata/repositories/v0.9.2/PROVENANCE.md # + one contents line (modify)
```

---

## Reference A — core types (defined in Tasks 1–2, consumed everywhere)

These exact shapes live in `internal/document/document.go` and `errors.go`. Later tasks use them verbatim.

```go
package document

// Span is a half-open byte range [Start, End) into Document.Source().
type Span struct {
	Start int
	End   int
}

// FieldShape classifies how a located field's value is written in source.
type FieldShape int

const (
	ShapeEmpty    FieldShape = iota // "key:" — a null with no value token
	ShapeInline                     // single-line plain/quoted scalar
	ShapeFlowSeq                    // single-line flow sequence, e.g. [3, 7]
	ShapeUnsupported                // block scalar, block collection, multi-line flow, anchor/alias…
)

// Field is one located column-zero frontmatter mapping entry.
type Field struct {
	Name  string
	Entry Span       // the full physical line(s) of the entry, terminator included
	Value Span       // the value token; Start==End for ShapeEmpty (insertion point)
	Shape FieldShape
}

// Block is one located managed marker block.
type Block struct {
	Name       string
	Annotation string // start-marker annotation without parentheses; "" if none
	Start      Span   // the full start-marker line, terminator included
	End        Span   // the full end-marker line, terminator included
	Interior   Span   // bytes strictly between the two marker lines
}

// Document is an immutable parsed view over an exact byte copy of one record.
type Document struct {
	source     []byte
	lineEnding string // "\n" or "\r\n" — the document-level ending (first terminated line's)
	hasFM      bool
	fmOpen     Span // opening fence line, terminator included
	fmClose    Span // closing fence line, terminator included
	fields     []Field
	blocks     []Block
	yamlRoot   *yaml.Node // private; never crosses the boundary
}

func Parse(source []byte) (Document, error)
func (d Document) Source() []byte                       // fresh copy
func (d Document) HasFrontmatter() bool
func (d Document) LineEnding() string
func (d Document) Fields() []Field                      // fresh slice, source order
func (d Document) Field(name string) (Field, bool)
func (d Document) Blocks() []Block                      // fresh slice, source order
func (d Document) Block(name string) (Block, bool)
func (d Document) DecodeFrontmatter(destination any) error
func (d Document) Apply(p PatchSet) ([]byte, error)
```

Error model (`errors.go`):

```go
package document

// Kind is a stable package-local error kind. The set is CLOSED — spec,
// "Error model and mutation safety" names each required kind.
type Kind string

const (
	KindMissingFrontmatter  Kind = "missing-frontmatter"  // DecodeFrontmatter/patch on a doc without frontmatter
	KindUnclosedFrontmatter Kind = "unclosed-frontmatter" // opener without closer
	KindInvalidUTF8         Kind = "invalid-utf8"
	KindInvalidYAML         Kind = "invalid-yaml"         // parse failure, multi-doc, non-mapping root, unresolved alias
	KindDuplicateField      Kind = "duplicate-field"      // duplicate mapping key (parse) or duplicate builder key
	KindMalformedMarker     Kind = "malformed-marker"     // grammar failure on a docket-marker-shaped line
	KindMarkerImbalance     Kind = "marker-imbalance"     // dangling / out-of-order / nested / duplicate markers
	KindMissingPatchTarget  Kind = "missing-patch-target"
	KindUnsupportedPatchShape Kind = "unsupported-patch-shape"
	KindInvalidValue        Kind = "invalid-value"        // bad key grammar, control chars, invalid UTF-8 in a value
	KindDuplicateEdit       Kind = "duplicate-edit"
	KindOverlappingEdit     Kind = "overlapping-edit"
	KindReparseFailed       Kind = "reparse-failed"       // candidate failed the same parse rules
)

// Error is the package's one error type.
type Error struct {
	Kind   Kind
	Name   string // field or block name when applicable; "" otherwise
	Offset int    // byte offset when available; -1 otherwise
	Line   int    // 1-based; 0 when unavailable
	Column int    // 1-based; 0 when unavailable
	Msg    string
}

func (e *Error) Error() string // "<kind>: <msg>" (+ " (<name>)" / " at line L col C" when set)

// IsKind reports whether err is a *Error carrying kind.
func IsKind(err error, kind Kind) bool
```

## Reference B — patch API (defined in Tasks 7–8)

```go
// BlockInsertionPoint names the only two generic insertion points this change needs.
type BlockInsertionPoint int

const (
	AtDocumentStart  BlockInsertionPoint = iota // byte offset 0
	AfterFrontmatter                            // immediately after the closing fence line
)

// PatchSet is an ordered collection of requested edits. The zero value is an
// empty set; Apply on an empty set returns byte-identical content.
type PatchSet struct {
	edits []edit
}

func (p *PatchSet) SetField(name string, v Value)    // change an EXISTING field's value token
func (p *PatchSet) InsertField(name string, v Value) // add an ABSENT field before the closing fence
func (p *PatchSet) ReplaceBlock(name string, content string)
	// content is logical LF-separated Markdown; emitted with the block's line ending
func (p *PatchSet) InsertBlock(name, annotation, content string, at BlockInsertionPoint)

type edit struct {
	op         editOp // opSetField | opInsertField | opReplaceBlock | opInsertBlock
	name       string
	value      Value  // field ops
	content    string // block ops
	annotation string // opInsertBlock
	at         BlockInsertionPoint
}
```

## Reference C — typed value model (defined in Task 6)

```go
// Value is the closed frontmatter-value model. Callers never provide raw YAML.
type Value struct {
	kind valueKind
	str  string
	num  int64
	b    bool
	seq  []Value
}

type valueKind int

const (
	kindNull valueKind = iota
	kindString
	kindInt
	kindBool
	kindSeq
)

func Null() Value
func String(s string) Value
func Int(i int64) Value
func Bool(b bool) Value
func Seq(items ...Value) Value // items must be scalars (null/string/int/bool); no nested seq

// validate returns a *Error (KindInvalidValue) or nil.
func (v Value) validate() error

// serialize renders the canonical inline YAML token. Null renders "" (the
// caller writes "key:" with no trailing space). Strings single-quote with
// each interior apostrophe doubled. Seq renders "[a, b]" / "[]".
func (v Value) serialize() string
```

---

### Task 1: Package scaffold and error model

**Files:**
- Create: `internal/document/errors.go`
- Create: `internal/document/errors_test.go`
- Create: `internal/document/document.go` (package doc + `Span` only in this task)

**Interfaces:**
- Produces: `Kind` constants, `Error`, `IsKind`, `Span` — exactly as Reference A.

- [ ] **Step 1: Write the failing test**

`internal/document/errors_test.go`:

```go
package document

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrorStringCarriesKindNameAndPosition(t *testing.T) {
	e := &Error{Kind: KindMalformedMarker, Name: "artifacts", Offset: 120, Line: 9, Column: 1,
		Msg: "start marker has no matching end"}
	got := e.Error()
	for _, want := range []string{"malformed-marker", "artifacts", "line 9"} {
		if !contains(got, want) {
			t.Fatalf("Error() = %q, want it to contain %q", got, want)
		}
	}
}

func TestIsKindMatchesWrappedErrors(t *testing.T) {
	e := &Error{Kind: KindInvalidYAML, Offset: -1, Msg: "boom"}
	wrapped := fmt.Errorf("outer: %w", e)
	if !IsKind(wrapped, KindInvalidYAML) {
		t.Fatal("IsKind should see through wrapping")
	}
	if IsKind(wrapped, KindInvalidValue) {
		t.Fatal("IsKind must not match a different kind")
	}
	if IsKind(errors.New("plain"), KindInvalidYAML) {
		t.Fatal("IsKind must not match a non-document error")
	}
}

func contains(haystack, needle string) bool {
	return len(needle) == 0 || (len(haystack) >= len(needle) && stringsIndex(haystack, needle) >= 0)
}
```

(Use `strings.Contains` directly — the helper above is illustrative; write the test with `strings.Contains`.)

- [ ] **Step 2: Run to verify failure**

Run: `go test -count=1 ./internal/document/`
Expected: FAIL — package does not compile (`Error` undefined).

- [ ] **Step 3: Implement `errors.go` and the `document.go` scaffold**

`errors.go` exactly as Reference A, with:

```go
func (e *Error) Error() string {
	s := string(e.Kind) + ": " + e.Msg
	if e.Name != "" {
		s += " (" + e.Name + ")"
	}
	if e.Line > 0 {
		s += fmt.Sprintf(" at line %d col %d", e.Line, e.Column)
	}
	return s
}

func IsKind(err error, kind Kind) bool {
	var de *Error
	return errors.As(err, &de) && de.Kind == kind
}
```

`document.go` gets the package comment (state the two-view architecture and the no-YAML-encoder rule in two sentences) and the `Span` type.

- [ ] **Step 4: Run to verify pass** — `go test -count=1 ./internal/document/` → PASS; `gofmt -l internal/document` → empty; `go vet ./internal/document/` → clean.

- [ ] **Step 5: Commit**

```bash
git add internal/document/
git commit -m "feat(0306): internal/document scaffold — Span, closed error-kind set"
```

---

### Task 2: Source copy, UTF-8 gate, line scan, frontmatter fences

**Files:**
- Create: `internal/document/frontmatter.go`
- Create: `internal/document/frontmatter_test.go`
- Modify: `internal/document/document.go` (add `Document` struct, `Parse` orchestration, accessors)

**Interfaces:**
- Consumes: `Span`, `Error`, kinds from Task 1.
- Produces: `Parse`, `Document.Source/HasFrontmatter/LineEnding`, plus internal `sourceLine` scan:

```go
// sourceLine is one physical line of the source.
type sourceLine struct {
	span    Span   // full line including its terminator
	text    Span   // line content excluding the terminator
	ending  string // "\n", "\r\n", or "" for a final unterminated line
}

func scanLines(src []byte) []sourceLine
// lineIsExactly reports whether the line's content (terminator excluded) equals s.
func lineIsExactly(src []byte, ln sourceLine, s string) bool
```

- [ ] **Step 1: Write the failing tests**

`internal/document/frontmatter_test.go` (representative set — write all of these):

```go
func TestParseCopiesSourceAndDoesNotAlias(t *testing.T) {
	in := []byte("---\nid: 3\n---\nbody\n")
	d, err := Parse(in)
	if err != nil { t.Fatal(err) }
	in[0] = 'X' // mutate caller buffer after Parse
	if got := d.Source(); string(got) != "---\nid: 3\n---\nbody\n" {
		t.Fatalf("Document captured caller mutations: %q", got)
	}
	out := d.Source()
	out[0] = 'Y'
	if d.Source()[0] == 'Y' {
		t.Fatal("Source() returned an aliasing slice")
	}
}

func TestFrontmatterOnlyWhenFirstLineIsFence(t *testing.T) {
	d, err := Parse([]byte("intro\n---\nnot: frontmatter\n---\n"))
	if err != nil { t.Fatal(err) }
	if d.HasFrontmatter() {
		t.Fatal("a body horizontal rule must not become frontmatter")
	}
}

func TestCRLFFenceDetectedAndLineEndingRecorded(t *testing.T) {
	d, err := Parse([]byte("---\r\nid: 1\r\n---\r\nbody\r\n"))
	if err != nil { t.Fatal(err) }
	if !d.HasFrontmatter() || d.LineEnding() != "\r\n" {
		t.Fatalf("HasFrontmatter=%v LineEnding=%q", d.HasFrontmatter(), d.LineEnding())
	}
}

func TestUnclosedFrontmatterIsTyped(t *testing.T) {
	_, err := Parse([]byte("---\nid: 1\nbody without closer\n"))
	if !IsKind(err, KindUnclosedFrontmatter) {
		t.Fatalf("want unclosed-frontmatter, got %v", err)
	}
}

func TestInvalidUTF8Rejected(t *testing.T) {
	_, err := Parse([]byte{'-', '-', '-', '\n', 0xff, 0xfe, '\n', '-', '-', '-', '\n'})
	if !IsKind(err, KindInvalidUTF8) {
		t.Fatalf("want invalid-utf8, got %v", err)
	}
}

func TestNoFrontmatterDocumentIsValid(t *testing.T) {
	d, err := Parse([]byte("# Just a spec\n\nprose\n"))
	if err != nil { t.Fatal(err) }
	if d.HasFrontmatter() {
		t.Fatal("plain Markdown must parse with no frontmatter")
	}
	if err := d.DecodeFrontmatter(&struct{}{}); !IsKind(err, KindMissingFrontmatter) {
		t.Fatalf("DecodeFrontmatter without frontmatter: want missing-frontmatter, got %v", err)
	}
}

func TestEmptyDocumentParses(t *testing.T) {
	d, err := Parse(nil)
	if err != nil { t.Fatal(err) }
	if d.HasFrontmatter() || len(d.Source()) != 0 {
		t.Fatal("empty input must parse to an empty, frontmatterless document")
	}
}
```

- [ ] **Step 2: Run to verify failure** — `go test -count=1 ./internal/document/` → FAIL (`Parse` undefined).

- [ ] **Step 3: Implement**

In `frontmatter.go`:

```go
func scanLines(src []byte) []sourceLine {
	var lines []sourceLine
	start := 0
	for i := 0; i < len(src); i++ {
		if src[i] == '\n' {
			end := i + 1
			textEnd, ending := i, "\n"
			if i > start && src[i-1] == '\r' {
				textEnd, ending = i-1, "\r\n"
			}
			lines = append(lines, sourceLine{
				span: Span{start, end}, text: Span{start, textEnd}, ending: ending})
			start = end
		}
	}
	if start < len(src) {
		lines = append(lines, sourceLine{
			span: Span{start, len(src)}, text: Span{start, len(src)}, ending: ""})
	}
	return lines
}
```

`Parse` orchestration in `document.go` (this task's portion):

1. `src := append([]byte(nil), source...)` — the immutable copy.
2. `utf8.Valid(src)` — else `&Error{Kind: KindInvalidUTF8, Offset: <first invalid index>, ...}` (find the offset by walking with `utf8.DecodeRune`).
3. `lines := scanLines(src)`; document line ending = first line's non-empty `ending`, default `"\n"`.
4. Fence: frontmatter exists iff `len(lines) > 0 && lineIsExactly(src, lines[0], "---")`. Closing fence = first later line exactly `---`; none → `KindUnclosedFrontmatter`. Record `fmOpen`/`fmClose` line spans.
5. Later tasks extend Parse; for now `DecodeFrontmatter` returns `KindMissingFrontmatter` when `!hasFM` and `nil` otherwise (Task 3 replaces the happy path).

`Source()` returns `append([]byte(nil), d.source...)`.

- [ ] **Step 4: Run to verify pass** — `go test -count=1 ./internal/document/` → PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/document/
git commit -m "feat(0306): Parse — source copy, UTF-8 gate, line scan, frontmatter fences"
```

---

### Task 3: Semantic YAML validation and typed decode

**Files:**
- Modify: `internal/document/document.go` (Parse step: YAML; `DecodeFrontmatter`)
- Create: `internal/document/document_test.go`

**Interfaces:**
- Consumes: fence spans from Task 2.
- Produces: validated `yamlRoot *yaml.Node` (mapping) stored on `Document`; `DecodeFrontmatter(destination any) error`.

- [ ] **Step 1: Write the failing tests**

`internal/document/document_test.go`:

```go
func TestDecodeFrontmatterIntoCallerStruct(t *testing.T) {
	src := []byte("---\nid: 306\nslug: loss-preserving-document-layer\ntrivial: false\ndepends_on: [304]\n---\nbody\n")
	d, err := Parse(src)
	if err != nil { t.Fatal(err) }
	var got struct {
		ID        int    `yaml:"id"`
		Slug      string `yaml:"slug"`
		Trivial   bool   `yaml:"trivial"`
		DependsOn []int  `yaml:"depends_on"`
	}
	if err := d.DecodeFrontmatter(&got); err != nil { t.Fatal(err) }
	if got.ID != 306 || got.Slug != "loss-preserving-document-layer" || got.Trivial || len(got.DependsOn) != 1 || got.DependsOn[0] != 304 {
		t.Fatalf("decoded %+v", got)
	}
}

func TestUnknownFieldsAreNotRejected(t *testing.T) {
	src := []byte("---\nid: 1\nfuture_key: kept\n---\n")
	d, err := Parse(src)
	if err != nil { t.Fatal(err) }
	var got struct{ ID int `yaml:"id"` }
	if err := d.DecodeFrontmatter(&got); err != nil {
		t.Fatalf("unknown fields are compatibility data, not errors: %v", err)
	}
}

func TestDuplicateMappingKeysRejected(t *testing.T) {
	_, err := Parse([]byte("---\nid: 1\nid: 2\n---\n"))
	if !IsKind(err, KindDuplicateField) {
		t.Fatalf("want duplicate-field, got %v", err)
	}
}

func TestMultipleYAMLDocumentsRejected(t *testing.T) {
	_, err := Parse([]byte("---\nid: 1\n--- extra: doc\n---\n"))
	if err == nil {
		t.Fatal("want an error for a second YAML document")
	}
}

func TestNonMappingFrontmatterRejected(t *testing.T) {
	_, err := Parse([]byte("---\n- just\n- a list\n---\n"))
	if !IsKind(err, KindInvalidYAML) {
		t.Fatalf("want invalid-yaml, got %v", err)
	}
}

func TestUnresolvedAliasRejected(t *testing.T) {
	_, err := Parse([]byte("---\nid: *nowhere\n---\n"))
	if !IsKind(err, KindInvalidYAML) {
		t.Fatalf("want invalid-yaml, got %v", err)
	}
}

func TestEmptyFrontmatterBlockIsValid(t *testing.T) {
	d, err := Parse([]byte("---\n---\nbody\n"))
	if err != nil { t.Fatal(err) }
	if !d.HasFrontmatter() {
		t.Fatal("an empty frontmatter block is still frontmatter")
	}
	var got struct{}
	if err := d.DecodeFrontmatter(&got); err != nil {
		t.Fatalf("decoding empty frontmatter into an empty struct: %v", err)
	}
}
```

Note on `TestMultipleYAMLDocumentsRejected`: the interior text is what sits between the two Docket fences; a `---` line inside it opens a second YAML document. Assert `err != nil` and additionally that `IsKind(err, KindInvalidYAML) || IsKind(err, KindUnclosedFrontmatter)` holds — the fence scanner may legitimately classify it first; pin whichever the implementation settles on and assert that one kind, with a comment naming the choice.

- [ ] **Step 2: Run to verify failure** — `go test -count=1 ./internal/document/` → FAIL.

- [ ] **Step 3: Implement**

In `Parse`, after fence discovery, when frontmatter exists:

```go
interior := src[d.fmOpen.End:d.fmClose.Start]
dec := yaml.NewDecoder(bytes.NewReader(interior))
var doc yaml.Node
err := dec.Decode(&doc)
```

- `io.EOF` → empty frontmatter: store a synthetic empty `MappingNode`.
- Other decode error → classify: message containing `already defined` → `KindDuplicateField` (yaml v3 reports duplicate mapping keys this way; keep the classifier in one small function `classifyYAMLError` with a comment quoting the matched phrase); everything else → `KindInvalidYAML`. Line/column from the yaml error when it is a `*yaml.TypeError` or parse error string — best-effort, `0` otherwise; offset = `d.fmOpen.End` (frontmatter start) when line mapping is unavailable.
- Second `dec.Decode(&extra)` returning `nil` → `KindInvalidYAML` "frontmatter must contain exactly one YAML document" (mirror `internal/config/parse.go`'s two-decode pattern).
- Root must be `yaml.MappingNode` (after unwrapping `DocumentNode`) → else `KindInvalidYAML`.
- Alias resolution: yaml v3 fails decode on an unknown anchor — surfaces via the decode error path.

`DecodeFrontmatter`:

```go
func (d Document) DecodeFrontmatter(destination any) error {
	if !d.hasFM {
		return &Error{Kind: KindMissingFrontmatter, Offset: -1,
			Msg: "document has no frontmatter"}
	}
	if err := d.yamlRoot.Decode(destination); err != nil {
		return &Error{Kind: KindInvalidYAML, Offset: -1,
			Msg: "decode frontmatter: " + err.Error()}
	}
	return nil
}
```

(Known-field rejection stays OFF — plain `Decode`, never `KnownFields(true)`.)

- [ ] **Step 4: Run to verify pass** — `go test -count=1 ./internal/document/` → PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/document/
git commit -m "feat(0306): semantic YAML validation and typed DecodeFrontmatter"
```

---

### Task 4: Field location map

**Files:**
- Modify: `internal/document/frontmatter.go` (add `locateFields`)
- Modify: `internal/document/document.go` (wire into Parse; `Fields`/`Field` accessors)
- Modify: `internal/document/frontmatter_test.go`

**Interfaces:**
- Consumes: `sourceLine` scan, fence spans, `yamlRoot`.
- Produces: `[]Field` with exact `Entry`/`Value` spans and `FieldShape`; `Document.Fields() []Field`, `Document.Field(name string) (Field, bool)`.

- [ ] **Step 1: Write the failing tests**

Add to `frontmatter_test.go`:

```go
func mustParse(t *testing.T, src string) Document {
	t.Helper()
	d, err := Parse([]byte(src))
	if err != nil { t.Fatal(err) }
	return d
}

func fieldValueText(d Document, name string) string {
	f, ok := d.Field(name)
	if !ok { return "<absent>" }
	return string(d.Source()[f.Value.Start:f.Value.End])
}

func TestFieldSpansCoverValueTokenOnly(t *testing.T) {
	d := mustParse(t, "---\nid: 306\npriority: critical   # keep\nadrs: []\nspec:\n---\n")
	cases := []struct{ name, want string; shape FieldShape }{
		{"id", "306", ShapeInline},
		{"priority", "critical", ShapeInline},
		{"adrs", "[]", ShapeFlowSeq},
		{"spec", "", ShapeEmpty},
	}
	for _, c := range cases {
		f, ok := d.Field(c.name)
		if !ok { t.Fatalf("field %s not indexed", c.name) }
		if got := fieldValueText(d, c.name); got != c.want {
			t.Errorf("%s value = %q, want %q", c.name, got, c.want)
		}
		if f.Shape != c.shape {
			t.Errorf("%s shape = %v, want %v", c.name, f.Shape, c.shape)
		}
	}
}

func TestInlineCommentExcludedFromValueSpan(t *testing.T) {
	d := mustParse(t, "---\nstatus: proposed # not part of the value\n---\n")
	if got := fieldValueText(d, "status"); got != "proposed" {
		t.Fatalf("value = %q — inline comment must stay outside the span", got)
	}
}

func TestHashInsideQuotesIsNotAComment(t *testing.T) {
	d := mustParse(t, "---\ntitle: 'a # not a comment'\n---\n")
	if got := fieldValueText(d, "title"); got != "'a # not a comment'" {
		t.Fatalf("value = %q", got)
	}
}

func TestUnknownKeysIndexedLikeKnownKeys(t *testing.T) {
	d := mustParse(t, "---\nfuture_key: kept\n---\n")
	if _, ok := d.Field("future_key"); !ok {
		t.Fatal("unknown Docket-shaped keys must be indexed")
	}
}

func TestNonDocketKeysAreNotPatchTargets(t *testing.T) {
	d := mustParse(t, "---\n\"quoted\": v\nUpper: v\n? complex\n: v\n---\n")
	for _, name := range []string{"quoted", "Upper", "complex"} {
		if _, ok := d.Field(name); ok {
			t.Errorf("%q must not be a patch target", name)
		}
	}
}

func TestBlockShapesIndexedAsUnsupported(t *testing.T) {
	d := mustParse(t, "---\nnotes: |\n  line one\n  line two\nitems:\n  - a\n  - b\n---\n")
	for _, name := range []string{"notes", "items"} {
		f, ok := d.Field(name)
		if !ok { t.Fatalf("%s must be indexed", name) }
		if f.Shape != ShapeUnsupported {
			t.Errorf("%s shape = %v, want ShapeUnsupported", name, f.Shape)
		}
	}
}

func TestBodyLinesResemblingKeysAreNotIndexed(t *testing.T) {
	d := mustParse(t, "---\nid: 1\n---\nstatus: this is body prose\n")
	f, ok := d.Field("status")
	if ok && f.Entry.Start > 20 {
		t.Fatal("a body line must never be indexed as a field")
	}
	if ok {
		t.Fatal("status only appears in the body; it must not be indexed")
	}
}

func TestEmptyValueSpanIsInsertionPoint(t *testing.T) {
	d := mustParse(t, "---\npr:\n---\n")
	f, _ := d.Field("pr")
	if f.Value.Start != f.Value.End {
		t.Fatalf("empty value must have a zero-width span, got %+v", f.Value)
	}
	// The span sits at end-of-line-content: immediately after the colon.
	src := d.Source()
	if src[f.Value.Start-1] != ':' && src[f.Value.Start-1] != ' ' {
		t.Fatalf("insertion point misplaced: byte before is %q", src[f.Value.Start-1])
	}
}
```

- [ ] **Step 2: Run to verify failure** — `go test -count=1 ./internal/document/` → FAIL.

- [ ] **Step 3: Implement `locateFields`**

Algorithm (in `frontmatter.go`):

1. Iterate the source lines strictly between `fmOpen` and `fmClose`.
2. A line is a candidate mapping entry iff it matches `^([a-z][a-z0-9_]*):(\s|$)` at column zero against the line's text (use a package-level `var docketKeyRE = regexp.MustCompile(...)` compiled once). Lines that do not match (continuations, comments, indented block content, quoted/complex keys) never start a field.
3. `Entry` span starts at the matched line. If the *semantic* value continues onto later lines (see shape classification below), extend `Entry.End` to the last continuation line's span end; continuation lines are every following line until the next candidate key line or the closing fence.
4. `Value` span: from the first non-space byte after the colon to the end of line text, minus any inline comment. Inline comment detection: scan the value region left-to-right tracking single-quote and double-quote state and bracket depth (`[`); a ` #` (space-then-hash, or hash at value start) outside quotes begins the comment; trim trailing spaces before the `#` from the value span. If the value region is empty → `ShapeEmpty` with `Value.Start = Value.End =` position after `key:` plus any spaces (place the zero-width span AFTER any existing trailing spaces so replacement never has to touch them).
5. Shape classification cross-checks the semantic tree: look up the key in `yamlRoot`'s content pairs. The value node decides:
   - `!!null` tag or zero-length → `ShapeEmpty`
   - scalar with style 0 (plain) or single/double-quoted, whose rendering sits on one line → `ShapeInline`
   - sequence with flow style entirely on the key's line → `ShapeFlowSeq`
   - literal/folded scalar, block sequence/mapping, multi-line flow, alias node → `ShapeUnsupported`
   The byte locator remains authoritative for the spans; the node supplies only the shape verdict and a consistency check (a key found by the locator but absent from the semantic mapping — or vice versa for column-zero plain keys — is a locator bug; add an internal consistency check that returns `KindInvalidYAML` with a "locator/semantic mismatch" message rather than silently mis-indexing).
6. Fields keep source order in `d.fields`.

- [ ] **Step 4: Run to verify pass** — `go test -count=1 ./internal/document/` → PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/document/
git commit -m "feat(0306): top-level field location map with shape classification"
```

---

### Task 5: Managed-block discovery and population validation

**Files:**
- Create: `internal/document/markers.go`
- Create: `internal/document/markers_test.go`
- Modify: `internal/document/document.go` (wire `scanBlocks` into Parse; `Blocks`/`Block` accessors)

**Interfaces:**
- Consumes: `sourceLine` scan, fence spans.
- Produces: `[]Block`; `Document.Blocks()`, `Document.Block(name)`. Parse fails with `KindMalformedMarker`/`KindMarkerImbalance` on an invalid population.

- [ ] **Step 1: Write the failing tests**

`internal/document/markers_test.go`:

```go
const artifactsBlock = "<!-- docket:artifacts:start (generated — do not hand-edit) -->\n| a |\n<!-- docket:artifacts:end -->\n"

func TestBlockDiscoveryWithAnnotation(t *testing.T) {
	d := mustParse(t, "---\nid: 1\n---\n\n"+artifactsBlock)
	b, ok := d.Block("artifacts")
	if !ok { t.Fatal("artifacts block not found") }
	if b.Annotation != "generated — do not hand-edit" {
		t.Fatalf("annotation = %q", b.Annotation)
	}
	if got := string(d.Source()[b.Interior.Start:b.Interior.End]); got != "| a |\n" {
		t.Fatalf("interior = %q", got)
	}
}

func TestStartMarkerWithoutAnnotationValid(t *testing.T) {
	d := mustParse(t, "<!-- docket:backlink:start -->\nx\n<!-- docket:backlink:end -->\n")
	if _, ok := d.Block("backlink"); !ok {
		t.Fatal("annotation-free start marker is valid")
	}
}

func TestMarkerInsideCodeFenceIsContent(t *testing.T) {
	src := "example:\n\n```text\n<!-- docket:example:start -->\n```\n"
	d := mustParse(t, src)
	if len(d.Blocks()) != 0 {
		t.Fatal("marker-shaped text inside a fenced code block is authored content")
	}
}

func TestTildeFenceAlsoShieldsMarkers(t *testing.T) {
	d := mustParse(t, "~~~\n<!-- docket:x:start -->\n~~~\n")
	if len(d.Blocks()) != 0 {
		t.Fatal("tilde fences shield markers too")
	}
}

func TestDanglingStartRejected(t *testing.T) {
	_, err := Parse([]byte("<!-- docket:a:start -->\nno end\n"))
	if !IsKind(err, KindMarkerImbalance) { t.Fatalf("got %v", err) }
}

func TestEndBeforeStartRejected(t *testing.T) {
	_, err := Parse([]byte("<!-- docket:a:end -->\n<!-- docket:a:start -->\n"))
	if !IsKind(err, KindMarkerImbalance) { t.Fatalf("got %v", err) }
}

func TestDuplicatePairRejected(t *testing.T) {
	_, err := Parse([]byte("<!-- docket:a:start -->\n<!-- docket:a:end -->\n<!-- docket:a:start -->\n<!-- docket:a:end -->\n"))
	if !IsKind(err, KindMarkerImbalance) { t.Fatalf("got %v", err) }
}

func TestNestedMarkersRejected(t *testing.T) {
	_, err := Parse([]byte("<!-- docket:a:start -->\n<!-- docket:b:start -->\n<!-- docket:b:end -->\n<!-- docket:a:end -->\n"))
	if !IsKind(err, KindMarkerImbalance) { t.Fatalf("got %v", err) }
}

func TestMalformedMarkerShapedLineRejected(t *testing.T) {
	// docket-marker prefix, but bad name (uppercase) — malformed, not prose.
	_, err := Parse([]byte("<!-- docket:BadName:start -->\n"))
	if !IsKind(err, KindMalformedMarker) { t.Fatalf("got %v", err) }
}

func TestOrdinaryHTMLCommentIsProse(t *testing.T) {
	d := mustParse(t, "<!-- just a comment -->\n")
	if len(d.Blocks()) != 0 { t.Fatal("plain comments are not markers") }
}

func TestMarkersInsideFrontmatterNotScanned(t *testing.T) {
	d := mustParse(t, "---\ntitle: 'has <!-- docket:x:start --> inside'\n---\n")
	if len(d.Blocks()) != 0 {
		t.Fatal("frontmatter bytes are not marker territory")
	}
}
```

- [ ] **Step 2: Run to verify failure** — `go test -count=1 ./internal/document/` → FAIL.

- [ ] **Step 3: Implement `markers.go`**

```go
var (
	// A line that BEGINS like a docket marker; used to distinguish
	// "malformed marker" from ordinary prose.
	markerPrefixRE = regexp.MustCompile(`^<!-- docket:`)
	// The exact marker grammar. Name: lower-case hyphenated. Annotation only
	// on start markers, parenthesized, no closing paren inside.
	markerRE = regexp.MustCompile(
		`^<!-- docket:([a-z][a-z0-9-]*):(start|end)(?: \(([^)]*)\))? -->$`)
	codeFenceRE = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})")
)
```

Scan every line OUTSIDE the frontmatter region (for a frontmatter document, lines after `fmClose`; otherwise all lines), maintaining code-fence state: an opening fence line (backtick or tilde run of ≥3, up to 3 leading spaces) flips into fenced mode recording the fence character and run length; a closing fence is a line whose fence run uses the same character with at least the opening's length and nothing but the run and whitespace on the line. Inside fenced mode no marker matching happens. (Backtick info strings are allowed on the opener; a closer has none — mirror CommonMark closely enough for these fixtures and say so in a comment.)

For each non-fenced line: if `markerRE` matches → record `{name, kind, annotation, line span}`; an end marker carrying an annotation is malformed. Else if `markerPrefixRE` matches → `KindMalformedMarker` with the line's offset.

Population validation over the ordered marker list:
- maintain `open` (name of the currently open block, or ""): a `start` while `open != ""` → nesting → `KindMarkerImbalance`; an `end` with `open == ""` or a different name → `KindMarkerImbalance`; a completed pair records a `Block`.
- after the walk, `open != ""` → dangling start → `KindMarkerImbalance`.
- a name seen in more than one completed pair → `KindMarkerImbalance` ("each name occurs at most once").

`Interior` = `Span{start.End, end.Start}` (start-marker line end to end-marker line start).

- [ ] **Step 4: Run to verify pass** — `go test -count=1 ./internal/document/` → PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/document/
git commit -m "feat(0306): code-fence-aware managed-marker discovery and balance validation"
```

---

### Task 6: Typed value model and shared serializer

**Files:**
- Create: `internal/document/value.go`
- Create: `internal/document/value_test.go`

**Interfaces:**
- Produces: `Value`, constructors, `validate`, `serialize` — exactly as Reference C. Also `validKey(name string) bool` (the `[a-z][a-z0-9_]*` grammar) and `validBlockContent(s string) error` (UTF-8, no NUL, control chars limited to LF and tab).

- [ ] **Step 1: Write the failing tests**

`internal/document/value_test.go`:

```go
func TestSerializeEveryKind(t *testing.T) {
	cases := []struct {
		v    Value
		want string
	}{
		{Null(), ""},
		{String("plain"), "'plain'"},
		{String("it's"), "'it''s'"},
		{String("a: b # c"), "'a: b # c'"}, // colon-space and hash are inert inside quotes
		{String(""), "''"},
		{Int(0), "0"},
		{Int(-42), "-42"},
		{Bool(true), "true"},
		{Bool(false), "false"},
		{Seq(), "[]"},
		{Seq(Int(3), Int(7)), "[3, 7]"},
		{Seq(String("a"), Bool(true), Null()), "['a', true, ]"},
	}
	for _, c := range cases {
		if err := c.v.validate(); err != nil {
			t.Fatalf("validate(%v): %v", c.v, err)
		}
		if got := c.v.serialize(); got != c.want {
			t.Errorf("serialize = %q, want %q", got, c.want)
		}
	}
}

func TestStringValueRejectsControlCharacters(t *testing.T) {
	for _, bad := range []string{"nul\x00", "bell\x07", "line\nbreak", "cr\r"} {
		if err := String(bad).validate(); !IsKind(err, KindInvalidValue) {
			t.Errorf("String(%q).validate() = %v, want invalid-value", bad, err)
		}
	}
	if err := String("tab\tok").validate(); err != nil {
		t.Errorf("tab is allowed in strings: %v", err)
	}
}

func TestStringValueRejectsInvalidUTF8(t *testing.T) {
	if err := String(string([]byte{0xff})).validate(); !IsKind(err, KindInvalidValue) {
		t.Errorf("got %v", err)
	}
}

func TestSeqRejectsNestedSeq(t *testing.T) {
	if err := Seq(Seq(Int(1))).validate(); !IsKind(err, KindInvalidValue) {
		t.Errorf("nested sequences are outside the closed model: %v", err)
	}
}

func TestValidKeyGrammar(t *testing.T) {
	for _, ok := range []string{"id", "depends_on", "a1", "claimed_at"} {
		if !validKey(ok) { t.Errorf("validKey(%q) = false", ok) }
	}
	for _, bad := range []string{"", "Id", "1a", "-x", "with-hyphen", "with space", "UPPER"} {
		if validKey(bad) { t.Errorf("validKey(%q) = true", bad) }
	}
}

func TestBlockContentAllowsLFAndTabOnly(t *testing.T) {
	if err := validBlockContent("line one\nline\ttwo\n"); err != nil {
		t.Fatalf("LF and tab are legal: %v", err)
	}
	for _, bad := range []string{"nul\x00", "esc\x1b", "cr\r\n"} {
		if err := validBlockContent(bad); !IsKind(err, KindInvalidValue) {
			t.Errorf("validBlockContent(%q) = %v, want invalid-value", bad, err)
		}
	}
}

func TestSerializedValuesRoundTripThroughYAML(t *testing.T) {
	// The serializer's output must be understood by the same semantic decoder.
	src := "---\ns: " + String("it's # tricky: yes").serialize() +
		"\nn: " + Int(-7).serialize() +
		"\nb: " + Bool(true).serialize() +
		"\nq: " + Seq(Int(3), String("x")).serialize() + "\n---\n"
	d, err := Parse([]byte(src))
	if err != nil { t.Fatal(err) }
	var got struct {
		S string `yaml:"s"`
		N int    `yaml:"n"`
		B bool   `yaml:"b"`
		Q []any  `yaml:"q"`
	}
	if err := d.DecodeFrontmatter(&got); err != nil { t.Fatal(err) }
	if got.S != "it's # tricky: yes" || got.N != -7 || !got.B || len(got.Q) != 2 {
		t.Fatalf("round trip lost data: %+v", got)
	}
}
```

Note the `Seq(String("a"), Bool(true), Null())` case: a null sequence element renders as an empty
token — `['a', true, ]`. If yaml v3 does not decode that trailing form as a 3-element sequence with
a nil tail, change the null-in-seq rendering to the explicit `null` keyword *for sequence elements
only* and update the golden — decide by running the round-trip test, and record the choice in a
comment beside the serializer. Top-level null MUST stay `""` (the `key:` form) either way.

- [ ] **Step 2: Run to verify failure** — `go test -count=1 ./internal/document/` → FAIL.

- [ ] **Step 3: Implement `value.go`**

```go
func String(s string) Value { return Value{kind: kindString, str: s} }
// … other constructors likewise.

var keyRE = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
func validKey(name string) bool { return keyRE.MatchString(name) }

func (v Value) validate() error {
	switch v.kind {
	case kindString:
		if !utf8.ValidString(v.str) { return invalidValue("string is not valid UTF-8") }
		for _, r := range v.str {
			if r == '\t' { continue }
			if r < 0x20 || r == 0x7f {
				return invalidValue(fmt.Sprintf("control character %q in string", r))
			}
		}
	case kindSeq:
		for _, item := range v.seq {
			if item.kind == kindSeq { return invalidValue("nested sequence") }
			if err := item.validate(); err != nil { return err }
		}
	}
	return nil
}

func (v Value) serialize() string {
	switch v.kind {
	case kindNull:   return ""
	case kindString: return "'" + strings.ReplaceAll(v.str, "'", "''") + "'"
	case kindInt:    return strconv.FormatInt(v.num, 10)
	case kindBool:   return strconv.FormatBool(v.b)
	case kindSeq:
		parts := make([]string, len(v.seq))
		for i, item := range v.seq { parts[i] = item.serialize() }
		return "[" + strings.Join(parts, ", ") + "]"
	}
	return ""
}
```

`validBlockContent` walks runes: reject invalid UTF-8, NUL, and any control character other than `\n` and `\t`.

- [ ] **Step 4: Run to verify pass** — `go test -count=1 ./internal/document/` → PASS. If the null-in-seq round trip fails, apply the decided fallback and re-run.

- [ ] **Step 5: Commit**

```bash
git add internal/document/
git commit -m "feat(0306): closed typed value model and shared single-quote serializer"
```

---

### Task 7: PatchSet and Apply — existing-field patches

**Files:**
- Create: `internal/document/patch.go`
- Create: `internal/document/patch_test.go`

**Interfaces:**
- Consumes: `Field` spans, `Value` serializer, error kinds.
- Produces: `PatchSet` with `SetField`; `Document.Apply(PatchSet) ([]byte, error)` implementing validate-all → high-to-low replacement → candidate reparse. (Insert/block ops arrive in Task 8; `Apply`'s pipeline is built here to handle a generic edit list.)

- [ ] **Step 1: Write the failing tests**

`internal/document/patch_test.go`:

```go
const fixtureDoc = "---\nid: 306\nstatus: proposed   # claim flips this\nadrs: []\npr:\nunknown_key: kept\n---\n\nBody stays.\n"

func applyOne(t *testing.T, src string, build func(*PatchSet)) string {
	t.Helper()
	d := mustParse(t, src)
	var p PatchSet
	build(&p)
	out, err := d.Apply(p)
	if err != nil { t.Fatal(err) }
	return string(out)
}

func TestEmptyPatchSetIsByteIdentical(t *testing.T) {
	d := mustParse(t, fixtureDoc)
	out, err := d.Apply(PatchSet{})
	if err != nil { t.Fatal(err) }
	if string(out) != fixtureDoc {
		t.Fatalf("no-op must be byte-identical:\n%q", out)
	}
}

func TestSetFieldPreservesEverythingElse(t *testing.T) {
	got := applyOne(t, fixtureDoc, func(p *PatchSet) {
		p.SetField("status", String("in-progress"))
	})
	want := "---\nid: 306\nstatus: 'in-progress'   # claim flips this\nadrs: []\npr:\nunknown_key: kept\n---\n\nBody stays.\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestSetFieldOnEmptyValue(t *testing.T) {
	got := applyOne(t, fixtureDoc, func(p *PatchSet) {
		p.SetField("pr", Int(211))
	})
	if !strings.Contains(got, "\npr: 211\n") {
		t.Fatalf("empty value must accept a value without eating the newline:\n%q", got)
	}
}

func TestSetFieldToNullDropsValueToken(t *testing.T) {
	got := applyOne(t, "---\nbranch: 'feat/x'\n---\n", func(p *PatchSet) {
		p.SetField("branch", Null())
	})
	if got != "---\nbranch:\n---\n" {
		t.Fatalf("null must render the bare key: form, got %q", got)
	}
}

func TestBatchAppliesAllEdits(t *testing.T) {
	got := applyOne(t, fixtureDoc, func(p *PatchSet) {
		p.SetField("status", String("done"))
		p.SetField("adrs", Seq(Int(93)))
		p.SetField("id", Int(306)) // same value — still a valid edit
	})
	for _, want := range []string{"status: 'done'", "adrs: [93]", "id: 306"} {
		if !strings.Contains(got, want) { t.Errorf("missing %q in:\n%q", want, got) }
	}
}

func TestPatchIsIdempotent(t *testing.T) {
	d := mustParse(t, fixtureDoc)
	var p PatchSet
	p.SetField("status", String("done"))
	once, err := d.Apply(p)
	if err != nil { t.Fatal(err) }
	d2, err := Parse(once)
	if err != nil { t.Fatal(err) }
	twice, err := d2.Apply(p)
	if err != nil { t.Fatal(err) }
	if string(once) != string(twice) {
		t.Fatal("re-applying the same semantic patch must be byte-idempotent")
	}
}

func TestMissingTargetFailsWholeBatch(t *testing.T) {
	d := mustParse(t, fixtureDoc)
	var p PatchSet
	p.SetField("status", String("done"))     // valid
	p.SetField("no_such_field", String("x")) // defect in a LATER item
	out, err := d.Apply(p)
	if !IsKind(err, KindMissingPatchTarget) { t.Fatalf("got %v", err) }
	if out != nil { t.Fatal("on error Apply must return nil bytes") }
}

func TestUnsupportedShapeRefused(t *testing.T) {
	d := mustParse(t, "---\nnotes: |\n  text\n---\n")
	var p PatchSet
	p.SetField("notes", String("flat"))
	if _, err := d.Apply(p); !IsKind(err, KindUnsupportedPatchShape) {
		t.Fatalf("got %v", err)
	}
}

func TestUnsupportedShapeOnUnrelatedFieldIsFine(t *testing.T) {
	src := "---\nnotes: |\n  text\nstatus: proposed\n---\n"
	got := applyOne(t, src, func(p *PatchSet) { p.SetField("status", String("done")) })
	if !strings.Contains(got, "notes: |\n  text\n") {
		t.Fatalf("unrelated block scalar must stay byte-identical:\n%q", got)
	}
}

func TestDuplicateEditRejected(t *testing.T) {
	d := mustParse(t, fixtureDoc)
	var p PatchSet
	p.SetField("status", String("a"))
	p.SetField("status", String("b"))
	if _, err := d.Apply(p); !IsKind(err, KindDuplicateEdit) {
		t.Fatalf("got %v", err)
	}
}

func TestInvalidValueRejectedBeforeAnyWork(t *testing.T) {
	d := mustParse(t, fixtureDoc)
	var p PatchSet
	p.SetField("status", String("bad\x00nul"))
	if _, err := d.Apply(p); !IsKind(err, KindInvalidValue) {
		t.Fatalf("got %v", err)
	}
}

func TestApplyDoesNotMutateInputDocument(t *testing.T) {
	d := mustParse(t, fixtureDoc)
	var p PatchSet
	p.SetField("status", String("done"))
	if _, err := d.Apply(p); err != nil { t.Fatal(err) }
	if string(d.Source()) != fixtureDoc {
		t.Fatal("Apply mutated the parsed document")
	}
}

func TestPatchedBytesDoNotAliasDocument(t *testing.T) {
	d := mustParse(t, fixtureDoc)
	out, err := d.Apply(PatchSet{})
	if err != nil { t.Fatal(err) }
	out[0] = 'X'
	if d.Source()[0] == 'X' { t.Fatal("returned bytes alias the document") }
}

func TestPatchOnNoFrontmatterDocRejected(t *testing.T) {
	d := mustParse(t, "just body\n")
	var p PatchSet
	p.SetField("id", Int(1))
	if _, err := d.Apply(p); !IsKind(err, KindMissingFrontmatter) {
		t.Fatalf("got %v", err)
	}
}
```

- [ ] **Step 2: Run to verify failure** — `go test -count=1 ./internal/document/` → FAIL.

- [ ] **Step 3: Implement `patch.go`**

`Apply` pipeline (generic over ops; Task 8 adds cases):

```go
func (d Document) Apply(p PatchSet) ([]byte, error) {
	// Phase 1 — validate EVERY edit before constructing anything.
	type resolved struct {
		span    Span   // bytes to replace
		payload []byte // replacement
	}
	var plan []resolved
	seen := map[string]bool{} // "field:<name>" / "block:<name>" duplicate detection
	for _, e := range p.edits {
		// per-op validation, filling plan — see per-op rules below
	}
	// Overlap check: sort plan by span.Start; adjacent spans may touch but not overlap.
	// Insertions are zero-width spans; two insertions at the SAME offset are a
	// duplicate-edit-family conflict -> KindOverlappingEdit.
	// Phase 2 — apply from the HIGHEST offset downward onto a fresh copy.
	out := append([]byte(nil), d.source...)
	for i := len(sortedPlan) - 1; i >= 0; i-- { /* splice */ }
	// Phase 3 — reparse the candidate under the same rules.
	if _, err := Parse(out); err != nil {
		return nil, &Error{Kind: KindReparseFailed, Offset: -1,
			Msg: "patched candidate failed reparse: " + err.Error()}
	}
	return out, nil
}
```

`opSetField` validation: frontmatter must exist (`KindMissingFrontmatter`); `validKey` (`KindInvalidValue`); `v.validate()`; field must exist (`KindMissingPatchTarget`); shape must be `ShapeEmpty`/`ShapeInline`/`ShapeFlowSeq` (`KindUnsupportedPatchShape`); duplicate name (`KindDuplicateEdit`).

Replacement construction for `opSetField`:
- serialized := `v.serialize()`.
- Non-empty target value token: replace `f.Value` with serialized. If serialized is empty (Null) — extend the replaced span left to also consume the run of spaces between the colon and `f.Value.Start`, so the result is the bare `key:` form; but ONLY when no inline comment follows (if bytes from `f.Value.End` to end-of-line-text are non-empty, keep the spacing: `key:   # comment` stays legal).
- Empty target (`ShapeEmpty`, zero-width span): payload = `" " + serialized` if the byte before the insertion point is `:` (no existing spacing), else serialized. Null on an already-empty field = no-op edit (empty payload, zero-width span) — legal, byte-identical.
- Idempotence follows: re-locating the patched field yields the same spans and the same serialized token.

- [ ] **Step 4: Run to verify pass** — `go test -count=1 ./internal/document/` → PASS.

- [ ] **Step 5: Mutation-probe the two safety gates this task owns**

1. Comment out the phase-1 loop's `KindMissingPatchTarget` return; run `go test -count=1 ./internal/document/` → `TestMissingTargetFailsWholeBatch` must FAIL. Restore.
2. Comment out phase 3 (candidate reparse); no current test reddens yet (the corpus goldens arrive in Task 10) — verify instead that `TestPatchIsIdempotent` still passes, then ADD the direct probe now:

```go
func TestReparseGateCatchesACorruptingEdit(t *testing.T) {
	// A value containing a raw "---" line CANNOT arise through the typed model
	// (control chars are rejected, strings are quoted) — so corrupt via a
	// hand-built edit if the internal API allows, or skip with a comment
	// pointing at the fuzz target that owns this invariant.
	d := mustParse(t, "---\nid: 1\n---\n")
	var p PatchSet
	p.SetField("id", Int(2))
	out, err := d.Apply(p)
	if err != nil || out == nil { t.Fatalf("control run failed: %v", err) }
}
```

The honest reparse-gate mutation check: temporarily make `serialize()` for `kindString` return the raw string unquoted, run `go test -count=1`, and confirm `TestSerializedValuesRoundTripThroughYAML` or an Apply test fails (the quote rule is what keeps candidates parseable). Restore. Record both probe outcomes in the task's commit message body.

- [ ] **Step 6: Commit**

```bash
git add internal/document/
git commit -m "feat(0306): PatchSet + Apply — validate-all field patches with candidate reparse"
```

---

### Task 8: Absent-field insertion and managed-block patches

**Files:**
- Modify: `internal/document/patch.go`
- Modify: `internal/document/patch_test.go`

**Interfaces:**
- Consumes: `Block` spans, `validBlockContent`, `BlockInsertionPoint`.
- Produces: `InsertField`, `ReplaceBlock`, `InsertBlock` — Reference B complete.

- [ ] **Step 1: Write the failing tests**

Add to `patch_test.go`:

```go
func TestInsertFieldLandsBeforeClosingFence(t *testing.T) {
	got := applyOne(t, "---\nid: 1\n---\nbody\n", func(p *PatchSet) {
		p.InsertField("claimed_at", String("2026-08-13T17:51:28Z"))
	})
	want := "---\nid: 1\nclaimed_at: '2026-08-13T17:51:28Z'\n---\nbody\n"
	if got != want { t.Fatalf("got %q", got) }
}

func TestInsertFieldUsesDocumentLineEnding(t *testing.T) {
	got := applyOne(t, "---\r\nid: 1\r\n---\r\n", func(p *PatchSet) {
		p.InsertField("pr", Int(7))
	})
	if !strings.Contains(got, "pr: 7\r\n") {
		t.Fatalf("inserted line must use CRLF here: %q", got)
	}
}

func TestInsertExistingFieldRejected(t *testing.T) {
	d := mustParse(t, "---\nid: 1\n---\n")
	var p PatchSet
	p.InsertField("id", Int(2))
	if _, err := d.Apply(p); !IsKind(err, KindDuplicateEdit) {
		t.Fatalf("inserting a present field: got %v", err)
	}
}

func TestSetAndInsertSameNameRejected(t *testing.T) {
	d := mustParse(t, "---\nid: 1\n---\n")
	var p PatchSet
	p.SetField("id", Int(2))
	p.InsertField("id", Int(3))
	if _, err := d.Apply(p); !IsKind(err, KindDuplicateEdit) {
		t.Fatalf("got %v", err)
	}
}

func TestReplaceBlockPreservesMarkerLines(t *testing.T) {
	src := "---\nid: 1\n---\n\n" + artifactsBlock + "\n## Why\n"
	got := applyOne(t, src, func(p *PatchSet) {
		p.ReplaceBlock("artifacts", "| new | row |")
	})
	want := "---\nid: 1\n---\n\n<!-- docket:artifacts:start (generated — do not hand-edit) -->\n| new | row |\n<!-- docket:artifacts:end -->\n\n## Why\n"
	if got != want { t.Fatalf("got:\n%q\nwant:\n%q", got, want) }
}

func TestReplaceBlockEmitsBlockLineEnding(t *testing.T) {
	src := "<!-- docket:backlink:start -->\r\nold\r\n<!-- docket:backlink:end -->\r\n"
	got := applyOne(t, src, func(p *PatchSet) {
		p.ReplaceBlock("backlink", "line one\nline two")
	})
	if !strings.Contains(got, "line one\r\nline two\r\n") {
		t.Fatalf("logical LF content must be emitted with the block's CRLF: %q", got)
	}
}

func TestReplaceMissingBlockRejected(t *testing.T) {
	d := mustParse(t, "---\nid: 1\n---\n")
	var p PatchSet
	p.ReplaceBlock("artifacts", "x")
	if _, err := d.Apply(p); !IsKind(err, KindMissingPatchTarget) {
		t.Fatalf("got %v", err)
	}
}

func TestInsertBlockAtDocumentStart(t *testing.T) {
	got := applyOne(t, "# Spec\n\nprose\n", func(p *PatchSet) {
		p.InsertBlock("backlink", "generated — do not hand-edit", "> home", AtDocumentStart)
	})
<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0306 — Loss-preserving document layer](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0306-loss-preserving-document-layer.md)**
<!-- docket:backlink:end -->
