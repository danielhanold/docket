<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0306 — Loss-preserving document layer](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-13-0306-loss-preserving-document-layer.md)**
<!-- docket:backlink:end -->

# Loss-Preserving Document Layer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

> **Restoration note (2026-08-13):** the first committed copy of this plan was truncated from
> Task 8's `TestInsertBlockAtDocumentStart` onward by `render-artifact-backlink.sh`, whose
> substring marker match treated a marker-shaped example literal inside this document as the real
> backlink block and consumed everything after it. This copy restores the authored content; the
> one triggering literal in Task 8 is written as a Go string concatenation so the close-out
> re-render cannot re-fire. The renderer defect is captured as a follow-up stub.

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

- [x] **Step 1: Write the failing test**

`internal/document/errors_test.go`:

```go
package document

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestErrorStringCarriesKindNameAndPosition(t *testing.T) {
	e := &Error{Kind: KindMalformedMarker, Name: "artifacts", Offset: 120, Line: 9, Column: 1,
		Msg: "start marker has no matching end"}
	got := e.Error()
	for _, want := range []string{"malformed-marker", "artifacts", "line 9"} {
		if !strings.Contains(got, want) {
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
```

- [x] **Step 2: Run to verify failure**

Run: `go test -count=1 ./internal/document/`
Expected: FAIL — package does not compile (`Error` undefined).

- [x] **Step 3: Implement `errors.go` and the `document.go` scaffold**

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

- [x] **Step 4: Run to verify pass** — `go test -count=1 ./internal/document/` → PASS; `gofmt -l internal/document` → empty; `go vet ./internal/document/` → clean.

- [x] **Step 5: Commit**

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

- [x] **Step 1: Write the failing tests**

`internal/document/frontmatter_test.go`:

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

- [x] **Step 2: Run to verify failure** — `go test -count=1 ./internal/document/` → FAIL (`Parse` undefined).

- [x] **Step 3: Implement**

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

- [x] **Step 4: Run to verify pass** — `go test -count=1 ./internal/document/` → PASS.

- [x] **Step 5: Commit**

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

- [x] **Step 1: Write the failing tests**

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

- [x] **Step 2: Run to verify failure** — `go test -count=1 ./internal/document/` → FAIL.

- [x] **Step 3: Implement**

In `Parse`, after fence discovery, when frontmatter exists:

```go
interior := src[d.fmOpen.End:d.fmClose.Start]
dec := yaml.NewDecoder(bytes.NewReader(interior))
var doc yaml.Node
err := dec.Decode(&doc)
```

- `io.EOF` → empty frontmatter: store a synthetic empty `MappingNode`.
- Other decode error → classify in one small function `classifyYAMLError`. Line/column from the yaml error when recoverable — best-effort, `0` otherwise; offset = `d.fmOpen.End` (frontmatter start) when line mapping is unavailable.
- Second `dec.Decode(&extra)` returning `nil` → `KindInvalidYAML` "frontmatter must contain exactly one YAML document" (mirror `internal/config/parse.go`'s two-decode pattern).
- Root must be `yaml.MappingNode` (after unwrapping `DocumentNode`) → else `KindInvalidYAML`.
- Duplicate mapping keys: yaml v3 does not report them when decoding into a `yaml.Node` — detect with a recursive node walk (`rejectDuplicateKeys`, the same shape as `internal/config/parse.go`'s `walkNode`), reporting the second occurrence as `KindDuplicateField`. *(Executed deviation from the original draft, which assumed the decoder errored; verified empirically.)*
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

- [x] **Step 4: Run to verify pass** — `go test -count=1 ./internal/document/` → PASS.

- [x] **Step 5: Commit**

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

- [x] **Step 1: Write the failing tests**

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
	if _, ok := d.Field("status"); ok {
		t.Fatal("status only appears in the body; it must not be indexed")
	}
}

func TestEmptyValueSpanIsInsertionPoint(t *testing.T) {
	d := mustParse(t, "---\npr:\n---\n")
	f, _ := d.Field("pr")
	if f.Value.Start != f.Value.End {
		t.Fatalf("empty value must have a zero-width span, got %+v", f.Value)
	}
	src := d.Source()
	if src[f.Value.Start-1] != ':' && src[f.Value.Start-1] != ' ' {
		t.Fatalf("insertion point misplaced: byte before is %q", src[f.Value.Start-1])
	}
}
```

- [x] **Step 2: Run to verify failure** — `go test -count=1 ./internal/document/` → FAIL.

- [x] **Step 3: Implement `locateFields`**

Algorithm (in `frontmatter.go`):

1. Iterate the source lines strictly between `fmOpen` and `fmClose`.
2. A line is a candidate mapping entry iff it matches `^([a-z][a-z0-9_]*):(\s|$)` at column zero against the line's text (package-level `var docketKeyRE`, compiled once). Lines that do not match (continuations, comments, indented block content, quoted/complex keys) never start a field.
3. `Entry` span starts at the matched line. If the *semantic* value continues onto later lines, extend `Entry.End` over the continuation lines. Continuation is keyed on **indentation** — blank, indented, or a column-zero `- ` block-sequence item — which is what YAML actually permits. *(Executed deviation: the original "until the next candidate key line" rule swallowed non-Docket-grammar keys like `worktree-scope:` as continuations; the indentation rule was validated against a 891-record corpus smoke test.)*
4. `Value` span: from the first non-space byte after the colon to the end of line text, minus any inline comment. Inline comment detection: scan the value region left-to-right tracking single-quote and double-quote state and bracket depth (`[`); a ` #` outside quotes begins the comment; trim trailing spaces before the `#` from the value span. If the value region is empty → `ShapeEmpty` with the zero-width span placed AFTER any trailing spaces.
5. Shape classification cross-checks the semantic tree: `!!null`/zero-length → `ShapeEmpty`; single-line plain/quoted scalar → `ShapeInline`; single-line flow sequence → `ShapeFlowSeq`; literal/folded scalar, block collection, multi-line flow, alias → `ShapeUnsupported`. The byte locator remains authoritative for spans. A key-set disagreement between locator and semantic mapping raises `KindInvalidYAML` ("locator/semantic mismatch"); a shape/byte disagreement fails closed to `ShapeUnsupported`.
6. Fields keep source order in `d.fields`.

- [x] **Step 4: Run to verify pass** — `go test -count=1 ./internal/document/` → PASS.

- [x] **Step 5: Commit**

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

- [x] **Step 1: Write the failing tests**

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

- [x] **Step 2: Run to verify failure** — `go test -count=1 ./internal/document/` → FAIL.

- [x] **Step 3: Implement `markers.go`**

```go
var (
	// A line that BEGINS like a docket marker; used to distinguish
	// "malformed marker" from ordinary prose. Column-zero anchored: an
	// indented marker-shaped line is prose.
	markerPrefixRE = regexp.MustCompile(`^<!-- docket:`)
	// The exact marker grammar. Name: lower-case hyphenated. Annotation only
	// on start markers, parenthesized, no closing paren inside.
	markerRE = regexp.MustCompile(
		`^<!-- docket:([a-z][a-z0-9-]*):(start|end)(?: \(([^)]*)\))? -->$`)
	codeFenceRE = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})")
)
```

Scan every line OUTSIDE the frontmatter region, maintaining code-fence state: an opening fence line (backtick or tilde run of ≥3, up to 3 leading spaces) flips into fenced mode recording the fence character and run length; a closing fence uses the same character with at least the opening's length and nothing but the run and whitespace. Inside fenced mode no marker matching happens. An unclosed fence shields to EOF. (Mirrors CommonMark loosely; say so in a comment.)

For each non-fenced line: `markerRE` match → record `{name, kind, annotation, line span}`; an end marker carrying an annotation is malformed. Else `markerPrefixRE` match → `KindMalformedMarker` with the line's offset.

Population validation over the ordered marker list:
- maintain `open`: a `start` while `open != ""` → nesting → `KindMarkerImbalance`; an `end` with `open == ""` or a different name → `KindMarkerImbalance`; a completed pair records a `Block`.
- after the walk, `open != ""` → dangling start → `KindMarkerImbalance`.
- a name in more than one completed pair → `KindMarkerImbalance`.

`Interior` = `Span{start.End, end.Start}`.

- [x] **Step 4: Run to verify pass** — `go test -count=1 ./internal/document/` → PASS.

- [x] **Step 5: Commit**

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

- [x] **Step 1: Write the failing tests**

`internal/document/value_test.go` (representative — the null-in-seq expectation was settled empirically at execution time, see Step 4 note):

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
		{Seq(String("a"), Bool(true), Null()), "['a', true, null]"},
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

- [x] **Step 2: Run to verify failure** — `go test -count=1 ./internal/document/` → FAIL.

- [x] **Step 3: Implement `value.go`**

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

- [x] **Step 4: Run to verify pass** — `go test -count=1 ./internal/document/` → PASS.

**Executed decision:** yaml v3.0.4 decodes `['a', true, ]` as a TWO-element sequence (the empty
token vanishes), so a null sequence element renders as the explicit `null` keyword — sequence
elements only; top-level null stays the `key:` form. The asymmetry is pinned by
`TestNullSequenceElementRoundTrips` and `TestTopLevelNullRendersEmpty`, and recorded in a comment
beside `serialize`.

- [x] **Step 5: Commit**

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

- [x] **Step 1: Write the failing tests**

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

- [x] **Step 2: Run to verify failure** — `go test -count=1 ./internal/document/` → FAIL.

- [x] **Step 3: Implement `patch.go`**

`Apply` pipeline, split for testability into `resolve` → `checkOverlaps` → `applyResolved(plan)` → `splice(plan)`:

```go
func (d Document) Apply(p PatchSet) ([]byte, error) {
	// Phase 1 — validate EVERY edit before constructing anything (resolve).
	// Phase 2 — overlap check: sort by span.Start; adjacent spans may touch but
	// not overlap; two zero-width insertions at one offset conflict.
	// Phase 3 — apply from the HIGHEST offset downward onto a fresh copy, then
	// reparse the candidate under the same rules (KindReparseFailed on failure).
}
```

`opSetField` validation: frontmatter must exist (`KindMissingFrontmatter`); `validKey` (`KindInvalidValue`); `v.validate()`; field must exist (`KindMissingPatchTarget`); shape must be `ShapeEmpty`/`ShapeInline`/`ShapeFlowSeq` (`KindUnsupportedPatchShape`); duplicate name (`KindDuplicateEdit`, `seen` map keyed `"field:"+name`).

Replacement construction for `opSetField`:
- Non-empty target value token: replace `f.Value` with serialized. Null (empty serialized): the spacing before the value token is swallowed only when nothing follows on the line, so `status: proposed   # c` → `status:    # c`, never `status:# c`.
- Empty target (zero-width span): leading space only when the preceding byte is `':'`; trailing space whenever non-spacing text (an inline comment) follows on the line — so `pr:   # note` receiving `Int(211)` yields `pr: 211   # note`-shaped output that reparses to the typed value with the comment intact.
- Idempotence follows: re-locating the patched field yields the same spans and serialized token.

The candidate-reparse gate gets a direct probe: `TestReparseGateRejectsACorruptingPayload` calls `applyResolved` with a hand-built resolved edit whose payload forges a duplicate key — unreachable through the public constructors, so the gate's refusal is observable.

- [x] **Step 4: Run to verify pass** — `go test -count=1 ./internal/document/` → PASS.

- [x] **Step 5: Mutation-probe the safety gates** (outcomes recorded in the commit body): missing-target return removed → `TestMissingTargetFailsWholeBatch` red; candidate reparse removed → `TestReparseGateRejectsACorruptingPayload` red; serializer unquoted → five tests red. All restored, re-verified green.

- [x] **Step 6: Commit**

```bash
git add internal/document/
git commit -m "feat(0306): PatchSet + Apply — validate-all field patches with candidate reparse"
```

---

### Task 8: Absent-field insertion and managed-block patches

**Files:**
- Modify: `internal/document/patch.go`
- Modify: `internal/document/patch_test.go`
- Modify: `internal/document/markers.go` (marker-line construction beside the grammar it must agree with)

**Interfaces:**
- Consumes: `Block` spans, `validBlockContent`, `BlockInsertionPoint`.
- Produces: `InsertField`, `ReplaceBlock`, `InsertBlock` — Reference B complete.

- [x] **Step 1: Write the failing tests**

Add to `patch_test.go` (representative set — the executed task carries 35 tests):

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

func TestReplaceBlockPreservesMarkerLines(t *testing.T) {
	src := "---\nid: 1\n---\n\n" + artifactsBlock + "\n## Why\n"
	got := applyOne(t, src, func(p *PatchSet) {
		p.ReplaceBlock("artifacts", "| new | row |")
	})
	want := "---\nid: 1\n---\n\n<!-- docket:artifacts:start (generated — do not hand-edit) -->\n| new | row |\n<!-- docket:artifacts:end -->\n\n## Why\n"
	if got != want { t.Fatalf("got:\n%q\nwant:\n%q", got, want) }
}

func TestReplaceBlockEmitsBlockLineEnding(t *testing.T) {
	src := "<!-- docket:homelink:start -->\r\nold\r\n<!-- docket:homelink:end -->\r\n"
	got := applyOne(t, src, func(p *PatchSet) {
		p.ReplaceBlock("homelink", "line one\nline two")
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
		p.InsertBlock("homelink", "generated — do not hand-edit", "> home", AtDocumentStart)
	})
	// The marker literal is concatenated so the docket backlink renderer can
	// never mistake this example for a real managed block in THIS file.
	want := "<!-- docket:homelink" + ":start (generated — do not hand-edit) -->\n> home\n<!-- docket:homelink:end -->\n\n# Spec\n\nprose\n"
	if got != want { t.Fatalf("got:\n%q", got) }
}

func TestInsertBlockAfterFrontmatter(t *testing.T) {
	got := applyOne(t, "---\nid: 1\n---\n\n## Why\n", func(p *PatchSet) {
		p.InsertBlock("artifacts", "", "| a |", AfterFrontmatter)
	})
	want := "---\nid: 1\n---\n\n<!-- docket:artifacts:start -->\n| a |\n<!-- docket:artifacts:end -->\n\n## Why\n"
	if got != want { t.Fatalf("got:\n%q", got) }
}

func TestInsertBlockExistingNameRejected(t *testing.T) {
	d := mustParse(t, artifactsBlock)
	var p PatchSet
	p.InsertBlock("artifacts", "", "x", AtDocumentStart)
	if _, err := d.Apply(p); !IsKind(err, KindDuplicateEdit) {
		t.Fatalf("got %v", err)
	}
}

func TestBadBlockContentRejected(t *testing.T) {
	d := mustParse(t, artifactsBlock)
	var p PatchSet
	p.ReplaceBlock("artifacts", "bad\x00content")
	if _, err := d.Apply(p); !IsKind(err, KindInvalidValue) {
		t.Fatalf("got %v", err)
	}
}

func TestFieldAndBlockPatchInOneBatch(t *testing.T) {
	src := "---\nid: 1\nstatus: proposed\n---\n\n" + artifactsBlock
	got := applyOne(t, src, func(p *PatchSet) {
		p.SetField("status", String("done"))
		p.ReplaceBlock("artifacts", "| merged |")
	})
	for _, want := range []string{"status: 'done'", "| merged |"} {
		if !strings.Contains(got, want) { t.Errorf("missing %q", want) }
	}
}
```

- [x] **Step 2: Run to verify failure** — `go test -count=1 ./internal/document/` → FAIL.

- [x] **Step 3: Implement the three ops**

- `opInsertField`: frontmatter must exist; `validKey`; `v.validate()`; field must be ABSENT (present → `KindDuplicateEdit`); zero-width span at `d.fmClose.Start`; payload = `name + ":" + maybeSpaceValue + d.lineEnding`.
- `opReplaceBlock`: `validBlockContent(content)`; block must exist (`KindMissingPatchTarget`); duplicate block name → `KindDuplicateEdit` (`seen` map, `"block:"+name` namespace). Span = `b.Interior`. Payload: logical-LF content emitted with the block's line ending (start-marker line's terminator; document ending fallback). Empty content → empty interior; a trailing logical LF does not double the final terminator.
- `opInsertBlock`: block-name grammar `[a-z][a-z0-9-]*` (`validBlockName`, in `markers.go` beside `markerRE` so the grammar has one spelling); annotation must not contain `)` or control chars; `validBlockContent(content)`; name must be absent (`KindDuplicateEdit`). Zero-width span at offset 0 (`AtDocumentStart`) or after `fmClose.End` (`AfterFrontmatter`). Payload = start-marker line + content lines + end-marker line + one blank separator when the insertion point is not already followed by a blank line or EOF; `AfterFrontmatter` leads with a blank line when needed. Marker-line construction via `startMarkerLine`/`endMarkerLine` in `markers.go`.

**Executed decisions (reviewer-confirmable):**
- Two field insertions in one `PatchSet` are refused as `KindOverlappingEdit` (both resolve to the
  same zero-width offset; Task 7's `checkOverlaps` treats same-offset insertions as a conflict).
  Pinned by `TestTwoFieldInsertionsRefusedAsOverlapping`; a later change needing a multi-field
  insert coalesces into one edit.
- `AtDocumentStart` is refused on a frontmatter document (`KindUnsupportedPatchShape`) — the
  corrupted result would parse cleanly as a frontmatterless document, so the reparse gate provably
  cannot catch it; mutation-proven real guard.

- [x] **Step 4: Run to verify pass** — `go test -count=1 ./internal/document/` → PASS.

- [x] **Step 5: Commit**

```bash
git add internal/document/
git commit -m "feat(0306): absent-field insertion and managed-block replace/insert patches"
```

---

### Task 9: Canonical new-document builder

**Files:**
- Create: `internal/document/builder.go`
- Create: `internal/document/builder_test.go`
- Create: `internal/document/testdata/new-doc-all-kinds.golden.md`

**Interfaces:**
- Produces:

```go
// FieldSpec is one ordered frontmatter field for a brand-new document.
type FieldSpec struct {
	Name  string
	Value Value
}

// New renders a canonical brand-new frontmatter document: LF endings, UTF-8
// without BOM, `---` fences, caller order, one blank line before the body,
// exactly one final newline. It validates every field before rendering and
// reparses its own output through Parse before returning.
func New(fields []FieldSpec, body string) ([]byte, error)
```

- [ ] **Step 1: Write the failing tests**

`internal/document/builder_test.go`:

```go
func TestNewRendersEveryValueKind(t *testing.T) {
	got, err := New([]FieldSpec{
		{"id", Int(306)},
		{"slug", String("loss-preserving-document-layer")},
		{"title", String("It's a 'title': tricky # yes")},
		{"trivial", Bool(false)},
		{"depends_on", Seq(Int(304))},
		{"adrs", Seq()},
		{"pr", Null()},
	}, "## Why\n\nBecause.\n")
	if err != nil { t.Fatal(err) }
	golden := filepath.Join("testdata", "new-doc-all-kinds.golden.md")
	want, err := os.ReadFile(golden)
	if err != nil { t.Fatal(err) }
	if !bytes.Equal(got, want) {
		t.Fatalf("golden mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestNewOutputReparsesAndDecodes(t *testing.T) {
	got, err := New([]FieldSpec{{"id", Int(1)}, {"note", String("it's")}}, "body\n")
	if err != nil { t.Fatal(err) }
	d, err := Parse(got)
	if err != nil { t.Fatalf("builder output must reparse: %v", err) }
	var out struct {
		ID   int    `yaml:"id"`
		Note string `yaml:"note"`
	}
	if err := d.DecodeFrontmatter(&out); err != nil { t.Fatal(err) }
	if out.ID != 1 || out.Note != "it's" { t.Fatalf("%+v", out) }
}

func TestNewRejectsDuplicateKeys(t *testing.T) {
	_, err := New([]FieldSpec{{"id", Int(1)}, {"id", Int(2)}}, "")
	if !IsKind(err, KindDuplicateField) { t.Fatalf("got %v", err) }
}

func TestNewRejectsBadKeyOrValueBeforeRenderingAnything(t *testing.T) {
	_, err := New([]FieldSpec{{"id", Int(1)}, {"Bad-Key", Int(2)}}, "")
	if !IsKind(err, KindInvalidValue) { t.Fatalf("got %v", err) }
	_, err = New([]FieldSpec{{"a", Int(1)}, {"b", String("x\x00")}}, "")
	if !IsKind(err, KindInvalidValue) { t.Fatalf("got %v", err) }
}

func TestNewNormalizesFinalNewline(t *testing.T) {
	for _, body := range []string{"body", "body\n", "body\n\n\n"} {
		got, err := New([]FieldSpec{{"id", Int(1)}}, body)
		if err != nil { t.Fatal(err) }
		if !bytes.HasSuffix(got, []byte("body\n")) || bytes.HasSuffix(got, []byte("\n\n")) {
			t.Fatalf("body %q → %q; want exactly one final newline", body, got)
		}
	}
}

func TestNewRequiresAtLeastOneField(t *testing.T) {
	if _, err := New(nil, "body\n"); !IsKind(err, KindInvalidValue) {
		t.Fatalf("a frontmatter builder with zero fields: got %v", err)
	}
}

func TestNewRejectsCRLFBody(t *testing.T) {
	if _, err := New([]FieldSpec{{"id", Int(1)}}, "a\r\nb\n"); !IsKind(err, KindInvalidValue) {
		t.Fatalf("canonical documents are LF-only: got %v", err)
	}
}
```

The golden `internal/document/testdata/new-doc-all-kinds.golden.md` (write it byte-exactly; note the doubled apostrophes and the quoted hash):

```
---
id: 306
slug: 'loss-preserving-document-layer'
title: 'It''s a ''title'': tricky # yes'
trivial: false
depends_on: [304]
adrs: []
pr:
---

## Why

Because.
```

(with a trailing newline after `Because.` and NO trailing blank line).

- [ ] **Step 2: Run to verify failure** — `go test -count=1 ./internal/document/` → FAIL.

- [ ] **Step 3: Implement `builder.go`**

Validate the whole field list first: at least one field; every `validKey`; every `Value.validate()`; duplicate names → `KindDuplicateField`. Body: valid UTF-8, no NUL, no `\r` (LF-only canon), other control chars limited to LF/tab → else `KindInvalidValue`. Render: `---\n`, each `name:` or `name: <serialized>\n`, `---\n`, `\n`, body trimmed to exactly one trailing `\n` (an empty body renders fences plus one final newline total after the closing fence; assert in a small extra test that `New([...], "")` reparses). Reparse own output via `Parse`; a failure is `KindReparseFailed` (should be unreachable — the fuzz target hunts for counterexamples).

- [ ] **Step 4: Run to verify pass** — `go test -count=1 ./internal/document/` → PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/document/
git commit -m "feat(0306): canonical new-document builder with all-kinds golden"
```

---

### Task 10: Frozen v0.9.2 document corpus and compatibility goldens

**Files:**
- Create: `testdata/repositories/v0.9.2/documents/` (frozen corpus — see capture list)
- Modify: `testdata/repositories/v0.9.2/PROVENANCE.md` (one contents line)
- Create: `internal/document/fixtures_test.go`
- Create: `internal/document/testdata/` adversarial fixtures (list below)

**Interfaces:**
- Consumes: the whole public API.
- Produces: the compatibility proof the acceptance boundary names.

- [ ] **Step 1: Capture the frozen corpus**

Real records, byte-exact, from two pinned sources (the archive/ADR/plan families live on tag `v0.9.2`; active changes and learnings exist only on the `docket` metadata branch — record BOTH pins):

```bash
mkdir -p testdata/repositories/v0.9.2/documents
git show v0.9.2:docs/changes/archive/2026-08-12-0298-stacked-changes-build-a-new-change-on-top-of-a-parent-change.md > testdata/repositories/v0.9.2/documents/archived-change-0298.md
git show v0.9.2:docs/adrs/0092-a-stacked-changes-base-is-its-parents-merge-destination.md > testdata/repositories/v0.9.2/documents/adr-0092.md
git show v0.9.2:docs/superpowers/plans/2026-08-12-stacked-changes.md > testdata/repositories/v0.9.2/documents/plan-with-backlink.md
git show 6d34592cf0934368ed075d5c644e9bfd0e4617e4:docs/changes/active/0158-batch-mode-for-docket-implement-next-build-several-coupled-c.md > testdata/repositories/v0.9.2/documents/active-change-0158.md
git show 6d34592cf0934368ed075d5c644e9bfd0e4617e4:docs/changes/learnings/frontmatter-anchored-read.md > testdata/repositories/v0.9.2/documents/learning-frontmatter-anchored-read.md
```

(0158 carries an `## Auto-groom blocked` body section plus artifacts block; the learning file has a double-quoted `hook:` string with apostrophes — both are discriminating real shapes. If any listed path is absent at its pin, `git show` fails loudly — pick the nearest same-family file at the same pin and record the substitution in PROVENANCE.)

Append to `testdata/repositories/v0.9.2/PROVENANCE.md` under `## Contents`:

```
- `documents/` — five byte-exact real records for change 0306's document-layer
  compatibility corpus: three from tag `v0.9.2` (archived change 0298, ADR 0092,
  the 0298 plan with its backlink block) and two from the `docket` metadata
  branch at commit `6d34592c` (active change 0158, learning finding
  `frontmatter-anchored-read`), which is where active changes and learnings
  live; no redaction.
```

These are historical snapshots pinned by commit — they must NOT track the live originals (the originals legitimately move as changes progress), which is exactly the drift-assert choice learning `frozen-copy-needs-a-drift-assert` requires writing down; the PROVENANCE line above is that statement.

- [ ] **Step 2: Write the package-local adversarial fixtures**

`internal/document/testdata/` — one small file each. Create each with the exact content its name promises:

| File | Content requirement |
|---|---|
| `crlf-full.md` | CRLF everywhere: fences, fields (incl. an empty value + inline comment), one managed block |
| `mixed-scalars.md` | bare, single-quoted, double-quoted values side by side |
| `empty-value-comment.md` | `pr:   # set when the PR is opened` |
| `flow-sequences.md` | `depends_on: [304]`, `adrs: []`, `related: [4, 6]` |
| `unknown-fields.md` | an unknown scalar field AND an unknown block-scalar (`x_notes: \|` + indented lines) |
| `body-keylike-lines.md` | body prose lines starting `status: ` and `id: ` |
| `fenced-marker-example.md` | a ```-fenced block containing a full marker pair |
| `invalid-dangling-marker.md` | start without end |
| `invalid-out-of-order.md` | end before start |
| `invalid-nested-markers.md` | a pair inside a pair |
| `invalid-duplicate-markers.md` | same name twice |
| `invalid-unclosed-fence.md` | `---` opener, fields, no closer |
| `invalid-duplicate-keys.md` | `id:` twice |
| `invalid-utf8.md` | contains bytes `0xFF 0xFE` (write from Go or `printf`; verify with `hexdump -C`) |

Naming rule: `invalid-*` files MUST fail Parse; every other file MUST parse. `fixtures_test.go` derives the expectation from the filename prefix — no hand-kept list (the correspondence stays one-loop-safe because the directory listing IS the population).

- [ ] **Step 3: Write the failing corpus tests**

`internal/document/fixtures_test.go`:

```go
package document

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const frozenDir = "../../testdata/repositories/v0.9.2/documents"

func frozenCorpus(t *testing.T) map[string][]byte {
	t.Helper()
	entries, err := os.ReadDir(frozenDir)
	if err != nil { t.Fatalf("frozen corpus missing: %v", err) }
	out := map[string][]byte{}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(frozenDir, e.Name()))
		if err != nil { t.Fatal(err) }
		out[e.Name()] = b
	}
	if len(out) < 5 { t.Fatalf("corpus has %d files, want the 5 captured records", len(out)) }
	return out
}

func TestFrozenCorpusParsesAndDecodesWithoutNormalizing(t *testing.T) {
	for name, src := range frozenCorpus(t) {
		d, err := Parse(src)
		if err != nil { t.Fatalf("%s: %v", name, err) }
		if got := d.Source(); !bytes.Equal(got, src) {
			t.Fatalf("%s: Source() normalized bytes", name)
		}
		if d.HasFrontmatter() {
			var meta struct {
				ID   int    `yaml:"id"`
				Slug string `yaml:"slug"`
			}
			if err := d.DecodeFrontmatter(&meta); err != nil {
				t.Fatalf("%s: decode: %v", name, err)
			}
		}
	}
}

func TestEmptyPatchSetByteIdenticalAcrossCorpus(t *testing.T) {
	for name, src := range frozenCorpus(t) {
		d, err := Parse(src)
		if err != nil { t.Fatalf("%s: %v", name, err) }
		out, err := d.Apply(PatchSet{})
		if err != nil { t.Fatalf("%s: %v", name, err) }
		if !bytes.Equal(out, src) {
			t.Fatalf("%s: empty PatchSet must be byte-identical", name)
		}
	}
}

func TestSingleFieldPatchChangesOnlyDeclaredSpan(t *testing.T) {
	src := frozenCorpus(t)["active-change-0158.md"]
	d, err := Parse(src)
	if err != nil { t.Fatal(err) }
	f, ok := d.Field("status")
	if !ok { t.Fatal("fixture must carry status:") }
	var p PatchSet
	p.SetField("status", String("in-progress"))
	out, err := d.Apply(p)
	if err != nil { t.Fatal(err) }
	pre, suf := commonPrefix(src, out), commonSuffix(src, out)
	// Every differing byte must sit inside the declared field spans.
	if pre < f.Value.Start || len(src)-suf > f.Entry.End {
		t.Fatalf("difference [%d, %d) escapes the declared field spans [%d, %d)",
			pre, len(out)-suf, f.Value.Start, f.Entry.End)
	}
}

func TestBatchPatchOnFrozenActiveChange(t *testing.T) {
	src := frozenCorpus(t)["active-change-0158.md"]
	d, err := Parse(src)
	if err != nil { t.Fatal(err) }
	var p PatchSet
	p.SetField("status", String("in-progress"))
	p.SetField("branch", String("feat/batch-mode"))
	out, err := d.Apply(p)
	if err != nil { t.Fatal(err) }
	rt, err := Parse(out)
	if err != nil { t.Fatalf("patched frozen record must reparse: %v", err) }
	var meta struct {
		Status string `yaml:"status"`
		Branch string `yaml:"branch"`
	}
	if err := rt.DecodeFrontmatter(&meta); err != nil { t.Fatal(err) }
	if meta.Status != "in-progress" || meta.Branch != "feat/batch-mode" {
		t.Fatalf("%+v", meta)
	}
	// Unknown-to-this-struct fields, comments, blocks, body survive.
	for _, invariant := range []string{"docket:artifacts:start", "## Why"} {
		if !strings.Contains(string(out), invariant) {
			t.Fatalf("lost %q", invariant)
		}
	}
}

func TestBlockReplaceOnFrozenPlanBacklink(t *testing.T) {
	src := frozenCorpus(t)["plan-with-backlink.md"]
	d, err := Parse(src)
	if err != nil { t.Fatal(err) }
	b, ok := d.Block("backlink")
	if !ok { t.Fatal("plan fixture must carry its backlink block") }
	var p PatchSet
	p.ReplaceBlock("backlink", "> ↩ re-rendered home link")
	out, err := d.Apply(p)
	if err != nil { t.Fatal(err) }
	// Marker lines and everything outside the interior are untouched.
	if !bytes.Equal(out[:b.Interior.Start], src[:b.Interior.Start]) {
		t.Fatal("bytes before the block interior changed")
	}
	tailSrc, tailOut := src[b.Interior.End:], out[len(out)-(len(src)-int(b.Interior.End)):]
	if !bytes.Equal(tailSrc, tailOut) {
		t.Fatal("bytes after the block interior changed")
	}
}

func TestPackageLocalFixturesParseByNamingRule(t *testing.T) {
	entries, err := os.ReadDir("testdata")
	if err != nil { t.Fatal(err) }
	for _, e := range entries {
		if e.IsDir() || strings.HasSuffix(e.Name(), ".golden.md") { continue }
		src, err := os.ReadFile(filepath.Join("testdata", e.Name()))
		if err != nil { t.Fatal(err) }
		_, perr := Parse(src)
		if strings.HasPrefix(e.Name(), "invalid-") {
			if perr == nil { t.Errorf("%s: want Parse failure", e.Name()) }
		} else if perr != nil {
			t.Errorf("%s: %v", e.Name(), perr)
		}
	}
}

func TestRefusalsReturnNoBytesAcrossCorpus(t *testing.T) {
	for name, src := range frozenCorpus(t) {
		d, err := Parse(src)
		if err != nil { t.Fatalf("%s: %v", name, err) }
		if !d.HasFrontmatter() { continue }
		var p PatchSet
		p.SetField("field_that_never_exists_anywhere", Int(1))
		out, err := d.Apply(p)
		if err == nil || out != nil {
			t.Errorf("%s: refusal must return (nil, error); got (%v, %v)", name, out, err)
		}
	}
}
```

`commonPrefix`/`commonSuffix` are five-line helpers in this test file (suffix scan bounded by `len - prefix` so the two never overlap). The prefix/suffix pair is the specified oracle for single-edit patches; multi-edit batches assert per-span with the same technique between edit spans — the spec's "every byte difference is covered by an edit span" is discharged by asserting difference-bounds against the plan's declared spans, never by a tolerated-lines list.

- [ ] **Step 4: Run to verify failure, then pass**

Run: `go test -count=1 ./internal/document/` — fixtures missing → FAIL. Complete Steps 1–2's captures, re-run → PASS. Also full `go test -count=1 ./...` → PASS.

- [ ] **Step 5: Check the gitignore interaction**

`testdata/repositories/.gitignore` exists (0305 added un-ignore rules for its fixture families). Run `git status --porcelain testdata/` after `git add`; every new corpus file must appear as staged. If any file is ignored, extend `testdata/repositories/.gitignore` with the narrowest un-ignore pattern for `v0.9.2/documents/` (mirror 0305's existing entries) — committed in this task.

- [ ] **Step 6: Commit**

```bash
git add testdata/repositories/v0.9.2/ internal/document/
git commit -m "feat(0306): frozen v0.9.2 document corpus, adversarial fixtures, compatibility goldens"
```

---

### Task 11: Fuzz targets, mutation-test sweep, suite gate

**Files:**
- Create: `internal/document/fuzz_test.go`
- Modify: `internal/document/patch_test.go` (only if a mutation probe exposes a vacuous assert)

**Interfaces:**
- Consumes: everything.
- Produces: the four seeded fuzz targets; the recorded mutation-probe results.

- [ ] **Step 1: Write the fuzz targets**

`internal/document/fuzz_test.go`:

```go
package document

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// seedAll adds every package-local fixture and every frozen corpus file.
func seedAll(f *testing.F) {
	f.Helper()
	for _, dir := range []string{"testdata", "../../testdata/repositories/v0.9.2/documents"} {
		entries, err := os.ReadDir(dir)
		if err != nil { f.Fatalf("seed dir %s: %v", dir, err) }
		for _, e := range entries {
			if e.IsDir() { continue }
			b, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil { f.Fatal(err) }
			f.Add(b)
		}
	}
}

// FuzzParse — fence + field-location discovery must never panic or loop, and
// a successful parse must round-trip its bytes and locate self-consistent spans.
func FuzzParse(f *testing.F) {
	seedAll(f)
	f.Fuzz(func(t *testing.T, src []byte) {
		d, err := Parse(src)
		if err != nil { return }
		if !bytes.Equal(d.Source(), src) {
			t.Fatal("Source() must equal input bytes")
		}
		for _, fld := range d.Fields() {
			if fld.Value.Start < fld.Entry.Start || fld.Value.End > fld.Entry.End ||
				fld.Entry.End > len(src) {
				t.Fatalf("field %s spans out of bounds: %+v", fld.Name, fld)
			}
		}
		for _, b := range d.Blocks() {
			if b.Interior.Start < b.Start.End || b.Interior.End > b.End.Start {
				t.Fatalf("block %s spans inconsistent: %+v", b.Name, b)
			}
		}
	})
}

// FuzzMarkers — marker discovery/balance over arbitrary body bytes.
func FuzzMarkers(f *testing.F) {
	seedAll(f)
	f.Fuzz(func(t *testing.T, body []byte) {
		d, err := Parse(body)
		if err != nil { return }
		names := map[string]bool{}
		for _, b := range d.Blocks() {
			if names[b.Name] { t.Fatalf("duplicate block name %q survived validation", b.Name) }
			names[b.Name] = true
		}
	})
}

// FuzzValueRoundTrip — serializer output must decode back to the same value.
func FuzzValueRoundTrip(f *testing.F) {
	f.Add("plain", int64(0), false)
	f.Add("it's # tricky: yes", int64(-7), true)
	f.Add("", int64(9223372036854775807), false)
	f.Fuzz(func(t *testing.T, s string, n int64, b bool) {
		v := String(s)
		if v.validate() != nil { return } // control chars etc. are legal refusals
		doc, err := New([]FieldSpec{{"s", v}, {"n", Int(n)}, {"b", Bool(b)}}, "x\n")
		if err != nil { t.Fatalf("builder refused validated values: %v", err) }
		d, err := Parse(doc)
		if err != nil { t.Fatalf("canonical output failed reparse: %v", err) }
		var out struct {
			S string `yaml:"s"`
			N int64  `yaml:"n"`
			B bool   `yaml:"b"`
		}
		if err := d.DecodeFrontmatter(&out); err != nil { t.Fatal(err) }
		if out.S != s || out.N != n || out.B != b {
			t.Fatalf("round trip: got %+v want (%q, %d, %v)", out, s, n, b)
		}
	})
}

// FuzzApply — batch patching over corpus documents with fuzzed values.
func FuzzApply(f *testing.F) {
	f.Add([]byte("---\nid: 1\nstatus: proposed\n---\nbody\n"), "done")
	f.Fuzz(func(t *testing.T, src []byte, val string) {
		d, err := Parse(src)
		if err != nil { return }
		var p PatchSet
		p.SetField("status", String(val))
		out, aerr := d.Apply(p)
		if aerr != nil {
			if out != nil { t.Fatal("error with non-nil bytes") }
			return
		}
		if _, err := Parse(out); err != nil {
			t.Fatalf("successful patch must reparse: %v", err)
		}
		// Idempotence: applying again yields identical bytes.
		d2, _ := Parse(out)
		out2, err := d2.Apply(p)
		if err != nil { t.Fatalf("idempotent reapply refused: %v", err) }
		if !bytes.Equal(out, out2) { t.Fatal("reapply not byte-idempotent") }
	})
}
```

- [ ] **Step 2: Run the seed corpus** — `go test -count=1 ./internal/document/` executes every fuzz target's seeds as ordinary tests → PASS.

- [ ] **Step 3: Run the bounded explicit fuzz campaigns (NOT wired into the suite)**

```bash
go test ./internal/document/ -run '^$' -fuzz FuzzParse -fuzztime 30s
go test ./internal/document/ -run '^$' -fuzz FuzzMarkers -fuzztime 30s
go test ./internal/document/ -run '^$' -fuzz FuzzValueRoundTrip -fuzztime 30s
go test ./internal/document/ -run '^$' -fuzz FuzzApply -fuzztime 30s
```

Expected: no crashers. A crasher writes a `testdata/fuzz/...` corpus entry — fix the defect, keep the minimized entry committed (it becomes a permanent regression seed), and note it in the results file. These commands are documentation-and-manual: the whole-suite gate stays `go test ./...` seed execution only.

- [ ] **Step 4: Mutation-test every named safety guard**

Each probe: apply the mutation, run `go test -count=1 ./internal/document/`, confirm the named test REDDENS, restore, re-run to green. Record each probe → outcome pair for the results file.

| Mutation | Must redden |
|---|---|
| Remove the closing-fence check (treat EOF as closer) | `TestUnclosedFrontmatterIsTyped` |
| Remove marker order/balance validation (return blocks without the walk) | `TestDanglingStartRejected` + siblings |
| Remove batch prevalidation (validate per-edit during application) | `TestMissingTargetFailsWholeBatch` (out != nil leg) |
| Remove the candidate reparse (phase 3) | `TestReparseGateRejectsACorruptingPayload` |
| Break the single-quote serializer (return raw string) | `TestSerializeEveryKind`, `TestSerializedValuesRoundTripThroughYAML` |
| Replace the field-patch path with YAML re-marshal of the tree (probe: route `Apply` through `yaml.Marshal` for the frontmatter region) | `TestEmptyPatchSetByteIdenticalAcrossCorpus`, `TestSetFieldPreservesEverythingElse` |

The last row is the spec's explicit "YAML marshal/re-encode must fail the compatibility goldens" proof; implement the probe crudely (marshal the semantic tree, splice it between the fences) — it only has to demonstrate the goldens catch it.

- [ ] **Step 5: Full suite gate**

Run the whole suite exactly as `finalize.test_command` resolves it:

```bash
scripts/run-tests.sh
```

Expected: green, and inspect the tail for any `OVER BUDGET:` line — `tests/test_go_toolchain.sh` (20s budget) now compiles and runs this package too; if it reports over budget, record the measured number in the results file as a finding (never silently edit the budget row).

- [ ] **Step 6: Commit**

```bash
git add internal/document/
git commit -m "test(0306): four seeded fuzz targets; mutation-probe sweep recorded"
```

---

## Self-review checklist (run after Task 11)

1. **Spec coverage sweep** — walk the spec section by section against the tasks: package contract names (`Parse`, `DecodeFrontmatter`, `Apply`, `PatchSet`, builder) — Tasks 2–9; fence rules — Task 2; YAML boundary (v3.0.4, no KnownFields, no Marshal) — Task 3 + Global Constraints; field map — Task 4; typed serialization — Task 6; markers — Task 5; canonical rendering — Task 9; error model — Task 1 + each op; corpus/goldens — Task 10; fuzz — Task 11; mutation tests — Tasks 7 + 11.
2. **Immutability audit** — grep the package for any return of an internal slice without copy (`d.source`, `d.fields`, `d.blocks`); each accessor copies.
3. **Boundary audit** — `go doc ./internal/document` output contains no `yaml.` type; `grep -rn 'yaml\.' internal/document/*.go` hits only unexported code paths and never `Marshal`/`Encoder`.
4. **gofmt/vet** — clean; full suite green.
