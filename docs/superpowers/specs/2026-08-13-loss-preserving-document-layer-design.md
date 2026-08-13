<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0306 — Loss-preserving document layer](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-13-0306-loss-preserving-document-layer.md)**
<!-- docket:backlink:end -->

# Loss-preserving document layer

**Change:** 0306 · **Type:** feat · **Priority:** critical · **Date:** 2026-08-13 · **Status:**
Approved design

## Purpose and boundary

This change creates the syntax-level document foundation that every later Go repository operation
can trust. It reads Markdown records as typed YAML semantics without surrendering the original
bytes, validates Docket-managed marker blocks before any rewrite, applies narrowly owned field and
block patches, and renders canonical syntax for brand-new documents.

The approved [Go migration program map](2026-08-12-go-migration-program-map.md) and
[architecture](2026-08-12-go-migration-architecture-design.md) are fixed upstream constraints.
This spec resolves only change 0306's representation, YAML boundary, patch mechanics, canonical
syntax, compatibility corpus, and fuzzing surface. It does not define configuration semantics,
repository-wide domain types, Git reads, transactions, render content, or workflow operations.

Change 0304 is complete and supplies the Go module, Go 1.26 toolchain, test conventions, and frozen
repository-fixture convention. Change 0306 adds one inward-facing `internal/document` package and
its fixtures; it adds no CLI command or protocol result of its own.

## Chosen approach

Represent a parsed document as two coordinated views:

1. an immutable copy of the exact source bytes plus half-open byte spans for frontmatter fields and
   managed blocks; and
2. a semantic YAML tree used to decode caller-supplied typed structs.

Existing documents are never emitted through a YAML encoder. A patch validates the complete input
and requested edit set, replaces only the located spans it owns, then reparses the candidate bytes
before returning them. Callers receive bytes; this package performs no filesystem, Git, commit, or
transaction write.

Two alternatives are rejected:

- Decoding and re-encoding a YAML node tree would reorder or restyle source and cannot preserve the
  original textual representation. That directly violates the migration compatibility contract.
- Adopting a third-party concrete-syntax-tree editor would move Docket's most important persistence
  guarantee behind that editor's rewrite behavior. Docket needs only its shallow, top-level
  frontmatter shape and exact marker lines, so a small owned locator is narrower and testable
  against the frozen corpus.

This is the source-byte-sidecar architecture already selected at program level, made concrete for
one independently deliverable package; it is not a reconsideration of the program architecture.

## Package contract

`internal/document` exposes the following API concepts. Planning may split their implementations
across files, but it must retain these names, separation, and behaviors.

- `Parse(source []byte) (Document, error)` copies the source, discovers optional frontmatter,
  validates YAML when frontmatter exists, indexes patchable top-level fields, scans managed blocks,
  and records exact line endings and byte spans.
- `Document.DecodeFrontmatter(destination any) error` decodes the already-validated semantic tree
  into a caller-owned typed struct. The document package defines no change, ADR, learning, status,
  graph, or lifecycle structs.
- `Document.Apply(PatchSet) ([]byte, error)` validates every requested edit before constructing any
  output, rejects duplicate or overlapping edits, applies replacements from the highest offset
  downward, and reparses the candidate before returning a fresh byte slice.
- A `PatchSet` can set a top-level field, replace the interior of one existing managed block, or
  insert a new managed block at a document-provided insertion point. It cannot perform an arbitrary
  byte replacement.
- A new-document builder accepts an ordered list of validated keys and typed frontmatter values
  plus caller-authored body text. It owns syntax only; later operations own which fields, order,
  defaults, headings, and managed content make a change, ADR, or learning record.

`Document` and every span it exposes are immutable values. Returned source and patched bytes do not
alias caller-owned mutable buffers. YAML-library node types do not cross the package boundary, so a
later dependency update cannot spread through domain or application APIs.

## Frontmatter discovery and semantic decoding

### Fence and document shape

- Frontmatter exists only when the first physical line is exactly `---`, ignoring that line's
  `LF` or `CRLF` terminator. This retains the existing Docket record shape and prevents a body
  horizontal rule from becoming frontmatter.
- The first later exact `---` line closes the frontmatter. An opener without a closer is invalid;
  parsing does not scan to EOF and guess.
- Markdown artifacts without a leading fence remain valid documents with no frontmatter, allowing
  managed backlinks in specs, plans, and results to use the same parser.
- When frontmatter exists, its YAML document must be one top-level mapping. Multiple YAML documents,
  aliases that do not resolve, duplicate mapping keys, invalid UTF-8, or another YAML parse failure
  produce a typed parse error. No repair or fallback reader runs.

### YAML library boundary

Pin `go.yaml.in/yaml/v3` at `v3.0.4` for semantic parsing and decoding. The maintained YAML-org
repository describes v3 as an API-stable legacy line receiving security fixes, while the currently
recommended v4 line is still prerelease. The stable v3 semantics are adequate for the frozen
`v0.9.2` corpus and avoid making the document foundation depend on a release candidate.

The dependency is deliberately one-way and narrow:

- use the library to parse the extracted frontmatter once and decode into caller-owned types;
- do not enable known-field rejection, because unknown fields are valid compatibility data that
  callers may not model yet;
- do not call `Marshal`, `Encoder`, or node re-encoding for an existing document; and
- do not use YAML node line/column data as the sole patch boundary. The library itself does not
  promise source-text round trips, so Docket's byte locator remains authoritative for edits.

The package accepts Docket's existing unquoted YAML spellings according to v3 decoding semantics.
New string values are always single-quoted at the Docket write boundary, so YAML boolean keywords,
comment introducers, indicators, colon-space, trailing colons, and apostrophes cannot change the
intended type or value.

### Field location map

The source locator indexes column-zero, plain Docket keys matching `[a-z][a-z0-9_]*`. Unknown keys
of that shape are indexed and preserved just like known keys; quoted, explicit, or nested YAML keys
may be decoded semantically but are not patch targets.

For each indexed field, the locator records the mapping-entry span and the value-token span. A
patch changes the value token while retaining the key spelling, colon, pre-value spacing, inline
comment, physical line ending, adjacent comments, blank lines, field order, and every other entry.
An empty value is a valid token position and can receive a value without consuming the following
newline.

The mutable fields understood by `v0.9.2` are inline scalars, nulls, or flow sequences. A caller
that asks to patch a block scalar, block collection, complex key, or another source shape outside
that retained write surface receives `unsupported-patch-shape`; the source remains untouched.
Such a shape is still allowed on an unrelated or unknown field and remains byte-identical when
another field is patched.

Setting an absent field is a separate caller-declared mode. The document layer can insert a
validated key/value immediately before the closing fence using the document's line ending; it does
not decide whether that key is legal or absent-capable. Change 0307 owns schema and policy, and
change 0312 owns the coarse operation that requests an insertion.

## Typed value serialization

Patch callers never provide raw YAML fragments. A closed frontmatter-value model supports the
syntax the retained document records require:

- null, rendered as `key:`;
- strings, always single-quoted with each interior apostrophe doubled;
- base-10 integers;
- booleans rendered as `true` or `false`; and
- flow sequences of those scalar kinds, including the canonical empty `[]`.

Keys must match the Docket key grammar. Strings and managed-block content must be valid UTF-8 and
reject NUL plus control characters other than the logical `LF` and tab allowed in Markdown. The
package validates an entire ordered field list or `PatchSet` before it serializes the first value,
so a bad final item cannot produce a partial document.

The same serializer is shared by field patches and canonical new-document rendering. That makes
YAML validity a construction property and prevents the existing-reader path and new-record path
from drifting into different quote rules.

## Managed-block discovery and patches

The parser recognizes exact, column-zero Docket marker lines of these forms, with a lower-case
hyphenated name and an optional parenthesized annotation on the start marker:

```text
<!-- docket:<name>:start (annotation) -->
<!-- docket:<name>:end -->
```

A start marker without an annotation is also valid. A line beginning as a Docket start/end marker
but not satisfying the grammar is malformed rather than ordinary authored prose.

Marker scanning is Markdown-fence-aware: marker-shaped example text inside a matching backtick or
tilde fenced code block is authored content, not a managed block. Outside code fences, the parser
validates the entire marker population before exposing any block:

- each name occurs at most once;
- every start has exactly one later end of the same name;
- end-before-start, dangling markers, mismatched names, duplicate pairs, and nesting are invalid;
  and
- an invalid marker anywhere rejects every block patch, even if the requested block itself looks
  balanced.

Replacing a block preserves its exact start and end marker lines and changes only the bytes between
them. Replacement content is supplied as logical `LF`-separated Markdown and is emitted with the
block's existing line ending. Insertion constructs both marker lines canonically at either the
document start or immediately after frontmatter; those are the only generic insertion points this
change needs. Later renderers own the block name, annotation, content, and which insertion point is
correct.

## Canonical new-document rendering

Brand-new frontmatter documents use one Go-owned syntax:

- `LF` line endings and UTF-8 without a BOM;
- opening and closing `---` fences;
- caller-supplied field order with one field per line;
- the shared typed serializer above;
- one empty line between the closing fence and body; and
- exactly one final newline.

The builder rejects duplicate keys and malformed values before producing output, then parses its
own result through the same document parser. It does not contain the change template, ADR template,
learning template, board layout, artifact table, backlink text, or ADR-index formatting. Those are
record- and view-level content owned by change 0312. This change delivers the canonical syntax
primitive that makes those later renderers deterministic.

## Error model and mutation safety

Errors are typed with a stable package-local kind and, when available, a field/block name plus byte
offset and line/column for diagnostics. Required kinds cover missing or malformed frontmatter,
invalid YAML, duplicate fields, malformed markers, missing patch targets, unsupported patch shapes,
invalid replacement values, duplicate edits, overlapping edits, and candidate reparse failure.

Parsing and patching never modify caller input. On every error `Apply` returns no candidate bytes;
there is no best-effort partial result. Successful candidates satisfy all of these invariants:

1. the candidate reparses under the same YAML and marker rules;
2. bytes outside the union of declared replacement/insertion spans are identical to the input;
3. applying the same semantic field or block patch again is byte-idempotent; and
4. a no-op patch returns byte-identical content.

Atomic filesystem replacement, permission/mode preservation, Git staging, transaction worktrees,
entity versions, retries, and commits are intentionally absent. Change 0309 owns the transaction
that eventually persists these returned bytes.

## Compatibility corpus and goldens

Add a frozen root fixture under `testdata/repositories/v0.9.2/` following change 0304's provenance
and immutability rules. Its source is tag `v0.9.2` and it contains representative real records from
each document family change 0306 must read: active and archived changes, ADRs, learning findings,
and Markdown artifacts carrying backlinks. Tests copy any fixture before mutation.

Package-local fixtures add discriminating source shapes that a real snapshot may not conveniently
contain: `CRLF`, mixed quoted and bare scalars, empty values with inline comments, flow sequences,
unknown and multiline unknown fields, body lines beginning with frontmatter key names, code-fenced
marker examples, missing/out-of-order/nested/duplicated markers, a missing closing frontmatter
fence, duplicate YAML keys, and malformed UTF-8.

Golden tests prove:

- typed decode of the frozen records without normalizing their bytes;
- applying an empty `PatchSet` is byte-identical for every corpus file;
- single-field and batch patches alter only their declared spans while preserving unknown fields,
  comments, quoting, whitespace, field order, line endings, markers, and authored body sections;
- absent-field insertion lands immediately before the closing fence and changes nothing else;
- managed-block replacement preserves marker lines and all surrounding content;
- every marker/fence/value refusal returns no candidate bytes;
- a defect in a later batch item leaves the whole input unchanged; and
- canonical new-document goldens cover every typed value kind and remain understood by the
  semantic decoder.

The compatibility assertion compares bytes directly, not parsed structures or pretty-printed
YAML. Tests also compute the changed ranges and assert that every byte difference is covered by an
edit span; a hand-listed set of tolerated neighboring lines is not an acceptable oracle.

## Fuzz targets

Go fuzz targets seed from both the frozen corpus and package-local adversarial fixtures:

- frontmatter fence and field-location discovery;
- managed-marker discovery and balance validation;
- typed value serialization plus semantic decode; and
- batch patch application.

For arbitrary bytes the targets assert no panic or unbounded loop. For successful parses/patches
they assert reparsability, idempotence, non-aliasing, and byte identity outside reported spans. The
ordinary `go test ./...` run executes the seed corpus; the implementation plan adds bounded explicit
fuzz commands for the four parser/patch targets without turning an unbounded fuzz campaign into a
mandatory whole-suite step.

Each safety guard is mutation-tested: remove the closing-fence check, marker-order check, batch
prevalidation, candidate reparse, or single-quote serializer and the corresponding focused test
must fail. Replacing the patch path with YAML marshal/re-encode must fail the compatibility goldens.

## Out of scope

- Configuration layers, capability classification, or configuration YAML types (0305).
- Change/ADR/learning schemas, repository snapshots, lifecycle rules, graphs, readiness,
  selection, or repository-wide validation (0307).
- Git discovery/object reads, blob identities, or external-command execution (0308).
- Transaction worktrees, entity-version contention, commits, push leases, retry, or idempotency
  trailers (0309).
- Status/health composition and human or protocol presentation (0310).
- Embedded assets, installation, or harness rendering (0311).
- Change/ADR/manual-learning operations; board, artifact, backlink, and ADR-index content; or any
  skill rewiring (0312).
- Feature workspaces, GitHub pull requests, build evidence, process supervision, implementation or
  finalize workflows, recovery, release packaging, self-hosting, and cutover (0313–0318).
- A general-purpose Markdown parser, a general YAML editor, repository schema versioning, a public
  Go API, or any Bash production change.

Change 0266 remains related input because its frontmatter data-loss cases and validate-all-before-
write rule shape this primitive. Change 0306 does not implement 0266's Bash facade, migrate shell
call sites, or change current skills; the later Go planning-operation slice consumes the document
API instead.

## Acceptance boundary

Change 0306 is complete when the standalone `internal/document` package can decode the frozen
`v0.9.2` corpus into caller-supplied test structs, return byte-identical no-op results, apply typed
field and managed-block patches with exact preservation outside declared spans, render and reparse
canonical new documents, refuse malformed fences/markers/YAML without output, and expose seeded
fuzz targets whose invariants hold. No repository policy, Git access, transaction, renderer content,
CLI operation, or workflow behavior from changes 0305 or 0307–0318 is required for that proof.
