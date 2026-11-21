<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0594 — Export as PDF alongside Markdown: docket export --format](../../changes/active/0594-export-as-pdf-alongside-markdown-docket-export-format.md)**
<!-- docket:backlink:end -->

# `docket export --format`: PDF alongside Markdown

## Intent

`docket export <type> [<id>]` writes a clean Markdown copy of a change, spec, plan, or results file. Daniel shares these copies with colleagues on private-repository projects, and colleagues read a PDF more readily than a `.md` file. This change adds a `--format` flag choosing which output files an export writes — `markdown`, `pdf`, or both — with **both as the default**, and builds the PDF renderer docket does not have today.

The PDF is a plain, readable rendering of exactly the Markdown copy: no extra content, no styling ambitions beyond legible typography.

## Goals

- `--format` on `docket export`, values `markdown` and `pdf`, default both.
- A pure-Go Markdown→PDF renderer that renders every construct docket artifacts use, including the non-ASCII characters they are full of (em dashes, arrows, box drawing in code diagrams, math symbols, a few emoji).
- The `docket-export` agent skill keeps working, Markdown-only.

## Non-goals

- **No new content cleanup.** The PDF holds exactly what the Markdown copy holds — in a private repository too. The CLI export does not remove mentions of docket, bare `#N` numbers, or ADR numbers today (it only warns about mentions); the PDF inherits that unchanged. Daniel accepted this explicitly: the PDF is the Markdown, rendered.
- **No PDF from the agent skill.** The skill's private-repository rewrite pass edits the exported `.md` in place; a PDF written beside it would carry the un-rewritten text. The skill therefore always asks for Markdown only.
- No "render this arbitrary `.md` file to PDF" command.
- No configuration key for a default format; the flag is enough (YAGNI).
- No colour emoji, no syntax highlighting, no clickable links (the export already strips links to their text), no table of contents, no CJK coverage.
- No compression of the embedded fonts (measured saving ~35% of font weight; not worth the code today).

## Anti-goals

- A PDF that silently drops characters: any character the fonts cannot draw is replaced with a visible placeholder and counted in a note.
- Any runtime dependency outside the binary (no pandoc, Chrome, LaTeX, network font download). docket stays one self-contained cross-compiled binary.
- A PDF failure that loses the Markdown copy.

## Behaviour (settled)

### The flag

- `--format <formats>`: `markdown`, `pdf`, or both, given comma-separated (`--format markdown,pdf`) or as a repeated flag (`--format markdown --format pdf`). Order and duplicates do not matter. Default: both.
- An unknown or empty value is a usage error (exit 2, before the operation runs), naming the valid choices — the same posture `--warnings` has today. Direct app-layer callers get the same refusal as `invalid-input` (mirroring how `ValidateExportWarnings` is reused in `app.Export`).

### File names and destinations

- Both files share one name and differ only in extension: `<date>-<kind>-<name>.md` and `<date>-<kind>-<name>.pdf` (`export.FileName` keeps producing the `.md` name; the PDF name swaps the extension).
- Default destination (`~/Downloads` on macOS, the home directory elsewhere) and `--output-path <existing-dir>`: every requested file goes there under its name.
- `--output-path <file>` with **one** format: the path is used exactly as given, as today.
- `--output-path <file>` with **both** formats: each file takes the given path with its extension replaced by the format's own (`~/x/report.md` → `report.md` + `report.pdf`; `~/x/report` → `report.md` + `report.pdf`).
- The existing protections apply to every file written: the destination's parent must exist, and no file may be the source artifact or lie inside the metadata checkout (`protectedDestination`). Each file is written atomically (temp file beside it, rename), and replacing an existing file adds a `replaced <path>` note, per file.

### Order and failure

- The Markdown copy is produced and written first (when requested). The PDF is rendered from those same Markdown bytes, then written.
- **If PDF rendering or writing fails, the Markdown copy (when requested) is still written and kept**, and the command reports the failure as an error: a non-`applied` result with a message naming what failed, the result still listing the Markdown file that was written, and a non-zero exit. (Today's `ResultExternalFailed` with a new, specific reason — e.g. `pdf-failed` — is the expected shape; the exact reason token is the builder's, registered with the other finding codes.)
- With `--format pdf` alone, a PDF failure writes nothing.
- A Markdown write failure behaves as today and nothing further is attempted.

### Opening

- Unchanged rule for *whether* to open: only an export to the default destination without `--print` opens anything.
- *What* opens: **the PDF when the export includes PDF, otherwise the Markdown.** Exactly one file opens.
- When the PDF failed, nothing opens (builder's-call default, chosen by the coordinator: the run is an error, as today's failures open nothing); the error message names the Markdown path so it can be opened by hand.

### Warnings and notes

- The leftover/mention self-check runs on the Markdown copy exactly as today, whatever the formats; warning line numbers name lines of the Markdown copy (also with `--format pdf` alone, where they describe the Markdown the PDF was rendered from). `--warnings` filters as today.
- New note when the PDF needed placeholders: `N characters the PDF fonts cannot draw were replaced with <placeholder>` (one note per export, only when N > 0). It is a note, never a block (house rule: new checks are visibility-only).

### Output

- Human: one path per line for every file written, Markdown first, so `$(docket export spec 9 --print)` still composes for single-format use. Notes, then warnings, on stderr; the message on a failure (after the paths that were written, as `HumanText` does today for an opener failure).
- JSON (`ExportResult`): a `files` list of `{format, path, replaced}` in Markdown-then-PDF order **replaces** `exported_path` and `replaced`. `launched` stays and means "the opened file was launched". `docket schema` reflects the change (`ExportResult` is registered in `internal/app/schema_registry.go`). No back-compat alias: the only in-repo consumer (the `docket-export` skill) changes in the same change.

### The agent skill and docs

- `skills/docket-export/SKILL.md`: the export invocation always passes `--format markdown`; every read of `exported_path` becomes a read of the Markdown entry in `files` (its `path`; `replaced` from the same entry). The failure rule ("no `exported_path`") becomes "no Markdown entry in `files`". Regenerate the embedded copy (`go generate ./internal/assets/`).
- `docs/reference/cli.md` and the command's `Long` help text describe `--format`, the two file names, the `--output-path` extension rule, what opens, and the PDF-failure behaviour. Docs describe current behaviour only.

## Rendering (settled)

### Approaches considered

1. **goldmark + fpdf + a small docket-owned renderer — chosen.** Parse with `github.com/yuin/goldmark` (v1, stable; GFM table, strikethrough, and task-list extensions), walk the AST, and draw with `codeberg.org/go-pdf/fpdf` (the maintained successor of jung-kurt/gofpdf; v0.12.0, 2026-05). Both MIT, pure Go, light dependencies. Full control of the look; tests can assert on the walk.
2. `solworktech/md2pdf` (formerly mandolyte/mdtopdf) — least code, but brings gomarkdown, a syntax highlighter, SVG conversion, logrus, and opinionated themes; fixing a badly drawn docket table means patching its theme. Rejected.
3. External tools (pandoc, headless Chrome, wkhtmltopdf) — break the single self-contained binary. Rejected. Also rejected: `unidoc/unipdf` (commercial licence), `stephenafamo/goldmark-pdf` (downloads Google Fonts at runtime; pulls in grpc/otel). Go's standard library has no PDF generation.

### Fonts (measured)

A scan of all 1,479 Markdown files on the metadata branch found `—` (18.6k uses), `→` (3.2k), `…`, `§`, `–`, `·`, `↩` (480), box drawing `─ │ └ ├ ═` (in code diagrams), `⇒` (282), `≤ ≥ ≠ ≈ ∈ ⊆ ⊕ ∧ ¬`, `① ②`, `⟨ ⟩`, and ~40 emoji (`✅ ❌ ⏳ ⚠ 🟡 🟢 🗑 ⛔ ⚪ ⭐ ⏺`). The Go fonts lack `↩ ⇒ ⟨ ⟩ ∈ ⊕ ⊆ ∧ ① ②` and every emoji, so:

- **Text and code: DejaVu 2.37** — Sans, Sans-Bold, Sans-Oblique, Sans-BoldOblique (2.74 MB) and Sans Mono, Sans Mono-Bold (0.67 MB). Licence: Bitstream Vera / DejaVu (free, redistributable).
- **Emoji fallback: Google Noto Emoji (monochrome), regular weight only** — 887 KB as a static regular instance of the variable `NotoEmoji[wght].ttf` (the variable file is 2.0 MB). OFL-1.1. Has a glyph for every emoji in the corpus (but see the fpdf limit below). Colour emoji are not possible here: colour emoji fonts use bitmap/COLR/SVG tables that fpdf cannot embed and PDF does not natively support.
- Fonts are embedded uncompressed with `//go:embed` and shipped with their licence files beside them; the binary grows by about 5.5 MB in total (fonts ~4.3 MB + libraries ~1–2 MB) on today's ~18 MB.
- Per character, the renderer picks the first font that has the glyph — the style's DejaVu face, then Noto Emoji — and otherwise draws a visible placeholder (the builder picks one DejaVu can draw, e.g. `□` U+25A1) and counts it for the note. Variation selectors (U+FE0F) and similar zero-width modifiers are dropped silently, not counted.
- **fpdf limit (verified by spike, fpdf v0.12.0):** fpdf's UTF-8 font support covers only the Basic Multilingual Plane. Any character above U+FFFF (e.g. 🟡 U+1F7E1, 🟢 U+1F7E2, 🗑 U+1F5D1) makes fpdf panic at output (`index out of range [65536]` in `generateCIDFontMap`); BMP emoji (✅ ❌ ⏳ ⚠) render correctly from Noto Emoji. Daniel's decision: **every character above U+FFFF becomes the placeholder and is counted in the note** — no font remapping, no library switch (gopdf handles them, but would mean writing wrapping, page breaks, and table layout by hand; ~11 occurrences in 1,479 documents). The renderer must screen every rune before it reaches fpdf, so no input can make the export panic; a test proves a non-BMP character yields a placeholder, not a crash.

### Constructs and look

Every construct in the frozen export corpus (`internal/export/testdata/corpus/`) must render:

- Headings H1–H6 (decreasing sizes, bold); paragraphs with soft-wrap; hard line breaks.
- Emphasis, strong, strong-emphasis, strikethrough; inline code in mono on a light-grey tint.
- Fenced and indented code blocks: mono on a light-grey background, whitespace preserved, long lines wrapped (never clipped), never reflowed.
- Bullet, ordered (keeping the start number), and nested lists with increasing indent; task-list items with box glyphs (`☐` / `☑`).
- GFM tables: thin borders, bold header row, column widths measured from content and capped to the page width, cell text wrapping, header row repeated after a page break.
- Blockquotes (indent plus a left rule, nestable); thematic breaks (a horizontal line).
- Raw HTML (inline or block) is drawn as its literal text; an image is drawn as its alt text. (Links never reach the renderer — the export already strips them — but a surviving link renders its text.)
- Page: US Letter, ~1-inch margins, page number centred in the footer; the PDF's Title property is the document's first H1 (else the file name stem).

Exact sizes, colours, spacing, and the placeholder glyph are the builder's.

## Placement and tests (left to the builder, with constraints)

- The renderer lives in its own package (e.g. `internal/pdfrender`) with a narrow API such as `Render(markdown []byte) (pdf []byte, placeholders int, err error)`; it knows nothing about docket artifacts. `app.Export` / `writeCleanCopy` orchestrate formats, names, writing, opening, and the result.
- Rendering must be deterministic for a given input (fixed creation date / no timestamps or random ids in the output) so tests can compare bytes.
- Tests at minimum: every file in the frozen corpus renders without error, and its placeholder count equals exactly the number of its characters above U+FFFF or absent from both fonts (so a regression in font fallback shows up); each construct above is exercised; the `--format` parsing and refusals; the `--output-path` extension rule; the PDF-failure path keeps the Markdown, returns the error result, and opens nothing; what opens for each format set; the `files` result shape and `docket schema`.
- New module dependencies: `github.com/yuin/goldmark`, `codeberg.org/go-pdf/fpdf` (and whatever it pulls in). Whether the stack choice warrants an ADR is the builder's call under the usual ADR rules.
