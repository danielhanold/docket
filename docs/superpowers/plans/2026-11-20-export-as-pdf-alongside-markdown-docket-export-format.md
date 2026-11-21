<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0594 — Export as PDF alongside Markdown: docket export --format](../../changes/archive/2026-11-21-0594-export-as-pdf-alongside-markdown-docket-export-format.md)**
<!-- docket:backlink:end -->

# `docket export --format`: PDF alongside Markdown — Implementation Plan

> **For agentic workers:** executed by `docket-build` task-by-task (each task routed to a tier agent running `docket-build-task`). Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `docket export` gains `--format markdown|pdf` (default both) and a new pure-Go renderer writes a PDF of exactly the clean Markdown copy, with embedded fonts, a visible counted placeholder for undrawable characters, and a kept Markdown copy when the PDF fails.

**Architecture:** A new leaf package `internal/pdfrender` parses Markdown with goldmark, lays it out through a small `canvas` interface (so tests record what is drawn), and paints with `codeberg.org/go-pdf/fpdf` using embedded DejaVu 2.37 and Noto Emoji fonts. Every rune is screened before it reaches fpdf: runes above U+FFFF, and BMP runes no font covers, become `□` and are counted. `app.Export`/`writeCleanCopy` decide formats, names, destinations, write order, what opens, and the result's new `files` list. The CLI adds the flag. The `docket-export` skill always asks for Markdown only.

**Tech Stack:** Go 1.26, `github.com/yuin/goldmark` (v1, GFM table/strikethrough/task-list extensions), `codeberg.org/go-pdf/fpdf` v0.12.0, `golang.org/x/image/font/sfnt` (already in fpdf's module graph) for glyph coverage, Cobra/pflag. Fonts come from upstream releases. The Noto Emoji static instance is made with fontTools (`uvx`).

**Spec:** `docs/superpowers/specs/2026-11-21-export-as-pdf-alongside-markdown-docket-export-format-design.md` (metadata branch; read it through `/Users/homer/dev/docket/.docket/`). Its *Behaviour (settled)* and *Rendering (settled)* sections are normative.

## Global Constraints

- Repository visibility: shared.
- **Network is required.** Task 1 downloads the fonts and adds `golang.org/x/image`. Task 2 adds goldmark and fpdf through the Go module proxy. Treat downloaded archives as untrusted data: extract them into their own `mktemp -d "${TMPDIR:-/tmp}/docket-fonts.XXXXXX"` directory and never run anything from inside it.
- No runtime dependency outside the binary: no network at run time, no external tools. Fonts are `//go:embed`ded, uncompressed, with their licence texts embedded beside them.
- `internal/pdfrender` knows nothing about docket artifacts. It imports no `internal/...` package.
- The PDF holds exactly what the Markdown copy holds. No content is added. The PDF metadata never names docket: set no Producer, Creator, Author, Subject, or Keywords; keep fpdf's default producer. The Title is the first H1's plain text, else the file-name stem.
- Rendering is deterministic: fixed creation and modification dates (`time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)`) set per instance, never through fpdf's package-level `SetDefault*` globals; `SetCatalogSort(true)`; no timestamps or random ids. `Render` is safe for concurrent calls.
- No rune above U+FFFF ever reaches fpdf (fpdf v0.12.0 panics on them in `generateCIDFontMap`). `Render` also recovers any panic into an error as defence in depth, but tests must prove screening directly, not through that recover.
- New checks are visibility-only: the placeholder count is a note, never a block.
- Exact copy pinned by the spec: values `markdown` and `pdf`; reason token `pdf-failed`; note `N characters the PDF fonts cannot draw were replaced with □` (singular: `1 character the PDF fonts cannot draw was replaced with □`); placeholder `□` (U+25A1).
- Docs and help text describe current behaviour only. Never cite a change or PR number in `docs/`, `skills/`, help text, or the fonts README. Comments anchor on symbol names, never line numbers.
- After any edit under `skills/`, run `go generate ./internal/assets/` and commit the regenerated `internal/assets/embedded/tree/...` copy in the same commit.
- Mutation probes: back up the file into a `mktemp -d "${TMPDIR:-/tmp}/mut.XXXXXX"` directory and restore it from that backup, never with `git checkout --`. Run every probe and focused run with `go test -count=1`.
- The whole suite is `build.test_command` (`go run ./cmd/docket development test`). `docket-build`'s final gate runs it once. Each task verifies itself with the focused commands below, plus `go vet` on the packages it touched.
- Shell: never pipe into `grep -q`/`head` under pipefail; template every `mktemp`; use `mv -f`.

## Review Focus

1. **A character above U+FFFF in any context:** a heading, a table cell, a code block, link text, a list item, or the Title. Each gets the placeholder and is counted. Nothing panics. Pinned by Task 2's `TestRenderNonBMPIsPlaceholderEverywhere` and Task 4's table case.
2. **An unbreakable token wider than its box:** a long URL or hash in prose, a 400-column code line, or a long word in a narrow table column. It is split at a rune boundary and never clipped, overflows nothing, and never loops forever. Pinned by Task 2 (prose), Task 3 (code), and Task 4 (cell).
3. **`--output-path <file>` with both formats, where the file has no extension or a different one:** `report`, `report.txt`, and `report.md` each give `report.md` and `report.pdf`. With one format, the path is used exactly as given. Pinned by Task 5's `TestExportOutputPathFormats`.
4. **A PDF failure at the default destination:** the Markdown copy stays, nothing opens, the exit is non-zero, and the one-line message names the Markdown path. Pinned by Task 5's `TestExportPDFFailureKeepsMarkdown` and Task 5's CLI exit test.
5. **Re-exporting over both existing files:** each `files` entry carries its own `replaced: true`, and there is one `replaced <path>` note per file. Pinned by Task 5's re-export case.

## Results requirements (for implement-next Step 6.5, not a build task)

- **Important, human:** open the PDFs of one real spec, one plan with code fences, and one results file with a table in a desktop viewer. Confirm the text is legible, code is monospaced and not reflowed, tables are bordered with the header repeated after a page break, and `→ — ↩ ✅ ⚠` render. Visual quality has no in-repo oracle.
- Record, as numbers: the release binary's size before and after (the spec estimates about +5.5 MB on about 18 MB); `Render` time for the largest corpus spec; and `go test -count=1` wall time of `./internal/pdfrender/`, `./internal/export/`, and `./internal/app/` before and after, against `tests/test_go_toolchain.sh`'s budget row.

## Coordinator follow-ups (after the build)

- Decide through `docket-adr` whether the stack choice warrants an ADR: goldmark + fpdf + embedded DejaVu/Noto fonts, the BMP-only limit, and the binary growth. The spec leaves this to the builder, and no build task writes one.

---

### Task 1: Embed the fonts, their licences, and glyph coverage

**Build tier:** standard

**Files:**
- Create: `internal/pdfrender/doc.go`, `internal/pdfrender/fonts.go`, `internal/pdfrender/fonts_test.go`
- Create: `internal/pdfrender/fonts/DejaVuSans.ttf`, `DejaVuSans-Bold.ttf`, `DejaVuSans-Oblique.ttf`, `DejaVuSans-BoldOblique.ttf`, `DejaVuSansMono.ttf`, `DejaVuSansMono-Bold.ttf`, `NotoEmoji-Regular.ttf`, `LICENSE-DejaVu.txt`, `OFL-NotoEmoji.txt`, `README.md`
- Modify: `NOTICE` (add a third-party fonts paragraph; keep the existing copyright line, which `TestLicenseFiles` pins), `go.mod`, `go.sum`

**Interfaces:**
- Produces (package `pdfrender`):
  - `type face int` with constants `faceSans, faceSansBold, faceSansOblique, faceSansBoldOblique, faceMono, faceMonoBold, faceEmoji`, and `var allFaces = []face{...}` in that order.
  - `func (f face) family() string` and `func (f face) style() string`: fpdf family/style pairs. Families are `dejavu` (`""`, `"B"`, `"I"`, `"BI"`), `dejavumono` (`""`, `"B"`), and `notoemoji` (`""`).
  - `func (f face) bytes() []byte`: the embedded TTF.
  - `func (f face) covers(r rune) bool`: true only when `r <= 0xFFFF` and the face's cmap maps `r` to a non-zero glyph. It uses `golang.org/x/image/font/sfnt`, parses each font once (`sync.Once`), and uses a fresh `sfnt.Buffer` per call, so it is concurrency-safe.
  - `const Placeholder = '□'` (exported; the app layer quotes it in its note).

- [ ] **Step 1: Fetch the fonts.** In a fresh `d=$(mktemp -d "${TMPDIR:-/tmp}/docket-fonts.XXXXXX")`:
  - DejaVu 2.37: `curl -fsSL -o "$d/dejavu.tar.bz2" https://github.com/dejavu-fonts/dejavu-fonts/releases/download/version_2_37/dejavu-fonts-ttf-2.37.tar.bz2`, then `tar -xjf` it inside `$d`. Copy the six `ttf/DejaVuSans{,-Bold,-Oblique,-BoldOblique}.ttf` and `ttf/DejaVuSansMono{,-Bold}.ttf` files, and the archive's `LICENSE` as `LICENSE-DejaVu.txt`.
  - Noto Emoji: resolve the `google/fonts` commit (`gh api repos/google/fonts/commits/main --jq .sha`). From `https://raw.githubusercontent.com/google/fonts/<sha>/ofl/notoemoji/` fetch `NotoEmoji%5Bwght%5D.ttf` and `OFL.txt` (save the latter as `OFL-NotoEmoji.txt`). Make the static regular instance with `uvx --from fonttools fonttools varLib.instancer "$d/NotoEmoji[wght].ttf" wght=400 -o internal/pdfrender/fonts/NotoEmoji-Regular.ttf` (expect about 0.9 MB).
  - Read `OFL-NotoEmoji.txt`. If it declares a Reserved Font Name, the instance is a Modified Version: rename its name table so it does not use that name, and say so in the README. If there is none, the README says there is none.
- [ ] **Step 2: Write `fonts/README.md`:** what each file is, its source URL (with the pinned `google/fonts` commit), the exact instancer command and fontTools version, its licence, and a table `| file | sha256 |` listing all nine embedded files. Add one paragraph to `NOTICE` naming DejaVu Fonts 2.37 (Bitstream Vera / DejaVu licence) and Google Noto Emoji (SIL OFL 1.1), pointing at `internal/pdfrender/fonts/`.
- [ ] **Step 3: Inventory coverage before pinning anything.** Write a throwaway `go run` program in your scratch directory that loads each font with sfnt and reports which face covers each rune of the list in Step 4. The spec's claims about which font has which glyph come from a groomer's scan, so verify them. A listed rune that neither font has stays out of the pinned lists; name it in your task report. The spec's placeholder rule already covers it.
- [ ] **Step 4: Write the failing tests in `fonts_test.go`:**
  - `TestFontsParse`: every face in `allFaces` parses with sfnt, and `faceSans.covers('A')` is true.
  - `TestFaceCoverage`: each of `— → … § – · × “ ” ↩ ─ │ └ ├ ═ ⇒ ≤ ≥ ≠ ≈ ∈ ⊆ ⊕ ∧ ¬ ① ② ⟨ ⟩` (trimmed per Step 3) is covered by `faceSans`. Each of `✅ ❌ ⏳ ⚠ ⛔ ⚪ ⭐ ⏺` is covered by `faceEmoji`. `Placeholder`, `☐`, and `☑` are covered by `faceSans` and `faceSansBold`. `'🟡'`, `'🗑'`, and `'🟢'` are covered by no face (the BMP-only rule). `'中'` is covered by no face.
  - `TestFontProvenance`: parse the README's sha256 table. Every embedded file (`fonts/*.ttf`, `fonts/*.txt`, from the `embed.FS`) is listed with a matching SHA-256, and every listed file is embedded. Check both directions.
- [ ] **Step 5: Run** `go test -count=1 ./internal/pdfrender/`. Expect FAIL (undefined symbols).
- [ ] **Step 6: Implement `doc.go`** (the package doc: a Markdown→PDF renderer that knows nothing about docket, BMP-only, deterministic) **and `fonts.go`**: `//go:embed fonts/*.ttf fonts/*.txt` into an `embed.FS`, plus the Interfaces above. `go get golang.org/x/image@v0.40.0 && go mod tidy`.
- [ ] **Step 7: Run** `go test -count=1 ./internal/pdfrender/ ./internal/repoguard/` and `go vet ./internal/pdfrender/`. Expect PASS. The repoguard run proves the new binary and licence files trip no whole-tree guard. If one does, do not widen an exclusion silently: report BLOCKED, naming the guard and the file.
- [ ] **Step 8: Mutation probe.** Flip one hex digit of one README sha256 and confirm `TestFontProvenance` goes red. Make `covers` ignore the `> 0xFFFF` bound and confirm `TestFaceCoverage` goes red. Restore both from backup.
- [ ] **Step 9: Commit:** `feat(pdfrender): embed DejaVu and Noto Emoji fonts with their licences`.

---

### Task 2: The renderer core: API, screening, inline layout, pages

**Build tier:** premium. Named risk: a rune above U+FFFF reaching fpdf (a panic in the user's export), and a line-breaking loop that never terminates on an unbreakable token.

**Files:**
- Create: `internal/pdfrender/render.go`, `internal/pdfrender/canvas.go`, `internal/pdfrender/glyphs.go`, `internal/pdfrender/layout.go`, `internal/pdfrender/render_test.go`, `internal/pdfrender/recorder_test.go`
- Modify: `go.mod`, `go.sum` (`go get codeberg.org/go-pdf/fpdf@v0.12.0 github.com/yuin/goldmark@latest`, then `go mod tidy`; record the goldmark version in the commit message)

**Interfaces:**
- Consumes: Task 1's `face`, `covers`, `bytes`, `family`, `style`, `Placeholder`.
- Produces (package `pdfrender`):
  - `func Render(markdown []byte, fallbackTitle string) (pdf []byte, placeholders int, err error)`: the public API.
  - `type canvas interface { measure(f face, size float64, s string) float64; text(x, y float64, f face, size float64, s string); fillRect(x, y, w, h float64, gray int); line(x1, y1, x2, y2, width float64, gray int); newPage(); pageNo() int }`. Coordinates are in points, origin top-left, and `y` is the line box's top. `fpdfCanvas` implements it over `*fpdf.Fpdf` (US Letter in `pt`, all seven faces registered with `AddUTF8FontFromBytes`, auto page break off).
  - `func render(md []byte, fallbackTitle string, c canvas) (title string, placeholders int)`: the walk that `Render` and the tests share.
  - `type style struct { bold, italic, mono, strike, tint bool; size float64 }` and `type piece struct { text string; f face; st style; w float64 }`.
  - `func resolve(s string, st style) (pieces []piece, placeholders int)` in `glyphs.go`. It drops U+FE00–FE0F, U+200B–200D, U+2060, U+FEFF, U+E0020–E007F, U+E0100–E01EF, and C0/C1 controls other than tab and newline, silently and uncounted. It picks the style's primary face (mono italic falls back to mono regular), then `faceEmoji`, per rune. Anything else becomes `Placeholder` drawn in the style's sans face (`faceSans` or `faceSansBold`) and is counted. Adjacent runes on the same face merge into one piece. Invalid UTF-8 decodes to U+FFFD and is resolved like any rune.
  - The layout engine in `layout.go`: words (split on spaces, the trailing space kept) → pieces → greedy line fill within `[left, right]`. A word wider than the line is split by rune. A hard break ends the line, and a soft break becomes a space. Before each line it calls `ensure(h)`, which calls `newPage()` when `y+h` passes the bottom margin. Line height is about 1.35× the size. `tint` draws `fillRect` behind the piece, and `strike` draws a `line` through its middle. Indentation is a left offset that later block types push and pop.
- Constructs owned by this task: H1–H6 (bold, strictly decreasing sizes), paragraphs, emphasis, strong, strong-emphasis, strikethrough, inline code (mono on a light-grey tint), links (their text), autolinks (their URL as text), images (their alt text), inline raw HTML (literal text), soft and hard breaks, thematic breaks (a horizontal line across the text width), the footer page number centred on every page, and the Title. In `ast.Text` values (never in code), resolve backslash escapes and entities with goldmark's `util.UnescapePunctuations`, `util.ResolveNumericReferences`, and `util.ResolveEntityNames`. Block types that Tasks 3 and 4 own render, for now, as plain paragraphs of their text content, so every input already renders.
- The page has about 1-inch margins and a body size of about 10.5 pt. Exact sizes, greys, and spacing are yours.

- [ ] **Step 1: Write the recorder (`recorder_test.go`).** `type drawn struct { page int; x, y float64; f face; size float64; s string }`. `recorder` wraps a real `fpdfCanvas` (so `measure` is real) and records every `text`, `fillRect`, and `line` call with the page number. `func renderRec(t *testing.T, md string) (*recorder, string, int)` runs `render`. Helpers: `texts(r) string` (all drawn text concatenated in order), `find(r, s) []drawn`.
- [ ] **Step 2: Write the failing tests (`render_test.go`):**
  - `TestRenderDeterministicPDF`: `Render` of a small document twice gives byte-equal output, starting with `%PDF-` and containing `%%EOF`. Two goroutines rendering at once also give equal bytes.
  - `TestRenderNonBMPIsPlaceholderEverywhere`: `# T 🟡\n\npara 🗑 *em 🟢*\n\n`` `code 🟡` ``\n\n[link 🟡](http://x)\n` → placeholders == 5. No `drawn.s` contains a rune > 0xFFFF. `□` is drawn 5 times. The returned title is `T □`. `Render` of the same input returns `err == nil`.
  - `TestRenderDropsInvisibles`: `⚠️ ok` (U+26A0 U+FE0F) and `a‍b` → placeholders == 0. `⚠` is drawn in `faceEmoji`. No drawn text contains U+FE0F or U+200D.
  - `TestRenderMissingBMPGlyph`: `中文` → placeholders == 2.
  - `TestRenderInlineStyles`: `*i* **b** ***bi*** ~~s~~ ` + "`c`" → `i` in `faceSansOblique`, `b` in `faceSansBold`, `bi` in `faceSansBoldOblique`, `s` in `faceSans` with one `line` op at its vertical middle, and `c` in `faceMono` with a `fillRect` starting at or left of its `x`.
  - `TestRenderHeadings`: `# A` … `###### F` → all six in bold faces, with sizes strictly decreasing from A to F, each larger than the body size except F, which is at least the body size.
  - `TestRenderWrapsAndBreaks`: a 600-word paragraph produces more than one distinct `y`. Every text op satisfies `x + measure ≤ right margin + 0.01`. `a  \nb` (hard break) puts `a` and `b` on different `y`. `a\nb` (soft) puts them on the same `y`. A single 3000-character word with no spaces is split, every piece fits the width, and the pieces concatenate back to the word. All of this finishes, under the test's default timeout.
  - `TestRenderPagesAndFooter`: 300 paragraphs → `pageNo() ≥ 3`. Each page has exactly one footer op whose text is its page number, horizontally centred within 1 pt.
  - `TestRenderTitle`: `# Widget *design*\n\n# Second` → title `Widget design`. No H1 (only `## x`) → the fallback title.
  - `TestRenderLinksImagesHTML`: `[text](http://x)` draws `text` and never `http://x`. `<https://a.test>` draws `https://a.test`. `![alt text](p.png)` draws `alt text`. `a <b>x</b>` draws `<b>` and `</b>` literally.
  - `TestRenderEscapesAndEntities`: `a \*b\* &amp; c` → `texts` contains `a *b* & c`. Inside a code span, `` `\*&amp;` `` stays `\*&amp;`.
  - `TestRenderThematicBreak`: `a\n\n---\n\nb` → exactly one horizontal `line` op whose length is the text width (within 1 pt).
  - `TestRenderOddInputs`: `""`, `"\x00\x01"`, `"\t"`, `"\xff\xfe"`, `strings.Repeat(">", 200)+" deep"`, and `strings.Repeat("x", 20000)` → each returns a PDF and `err == nil`.
  - `FuzzRender`: seeded with the inputs above and the non-BMP document. Assert no error and no drawn rune above U+FFFF (through `render` with a recorder). Plain `go test` runs only the seeds.
- [ ] **Step 3: Run** `go test -count=1 ./internal/pdfrender/`. Expect FAIL (undefined `render`/`Render`).
- [ ] **Step 4: Implement** `glyphs.go`, `canvas.go`, `layout.go`, and `render.go` per the Interfaces. `Render` builds an `fpdfCanvas`, applies the Global Constraints' determinism settings, calls `render`, sets the Title (`SetTitle(title, true)`; the title is already screened), and writes `Output` into a buffer. It returns `pdf.Error()` or the output error. A deferred `recover` turns a panic into `fmt.Errorf("pdf renderer: %v", r)`.
- [ ] **Step 5: Run** `go test -count=1 ./internal/pdfrender/` and `go vet ./internal/pdfrender/`. Expect PASS. Note `Render`'s time on a 30 KB document in your report.
- [ ] **Step 6: Mutation probes**, one at a time, each restored from backup: let runes > 0xFFFF pass `resolve` unreplaced (`TestRenderNonBMPIsPlaceholderEverywhere` must go red without panicking the test binary, through the recorder assertion); drop the per-rune split of over-wide words (`TestRenderWrapsAndBreaks` red); skip the `faceEmoji` fallback (`TestRenderDropsInvisibles` red).
- [ ] **Step 7: Commit:** `feat(pdfrender): render Markdown text, headings, and inline styles to PDF`.

---

### Task 3: Lists, task lists, code blocks, blockquotes, HTML blocks

**Build tier:** standard

**Files:**
- Create: `internal/pdfrender/blocks.go`, `internal/pdfrender/blocks_test.go`
- Modify: `internal/pdfrender/render.go` (dispatch these block kinds to `blocks.go`)

**Interfaces:**
- Consumes: Task 2's `canvas`, `render`, `resolve`, the layout engine and its indent push/pop, `ensure`, and the test recorder.
- Produces: no new exported names.
- Rules: bullets `•`, with nested levels indented further. Ordered items keep `List.Start` and number up from it. Task items draw `☐`/`☑` in `faceSans` in place of the bullet and never the literal `[ ]`/`[x]`. Fenced and indented code: `faceMono` at about 9.5 pt, a light-grey `fillRect` per line, whitespace preserved, tabs expanded to the next multiple of 4 columns. A line wider than the box wraps by rune and is never reflowed or clipped. Blocks may cross pages, and the backgrounds continue. Blockquotes: indented, with a vertical rule at their left edge per nesting level. HTML blocks: their lines drawn literally, in the body face.

- [ ] **Step 1: Write the failing tests (`blocks_test.go`):**
  - `TestRenderLists`: `- a\n- b\n  - c\n` → `c.x > a.x`, two bullets at depth 0, one at depth 1. `3. x\n4. y\n` draws `3.` and `4.`. A loose list (blank lines between items) renders every item.
  - `TestRenderTaskList`: `- [ ] todo\n- [x] done\n` draws `☐` before `todo` and `☑` before `done`, and `texts` contains neither `[ ]` nor `[x]`.
  - `TestRenderCodeBlocks`: a fenced block with `  lead\tTab` draws a `faceMono` op whose text starts with `  lead` and whose tab became spaces to column 8. Each code line has a `fillRect` behind it. A 400-character line wraps into ≥ 2 ops, every one inside the box, and they concatenate to the original line. `    indented` (an indented code block) draws in `faceMono`. A 200-line block spans ≥ 2 pages, with backgrounds on both.
  - `TestRenderBlockquote`: `> a\n>\n> > b\n` → `b.x > a.x > body left`, and two vertical `line` ops at distinct x on the left.
  - `TestRenderHTMLBlock`: `<div>\nhi\n</div>\n` draws `<div>` and `</div>` literally.
- [ ] **Step 2: Run** `go test -count=1 ./internal/pdfrender/ -run 'TestRender(Lists|TaskList|CodeBlocks|Blockquote|HTMLBlock)'`. Expect FAIL.
- [ ] **Step 3: Implement** `blocks.go` per the rules above.
- [ ] **Step 4: Run** `go test -count=1 ./internal/pdfrender/` (all of Task 2 still green) and `go vet ./internal/pdfrender/`. Expect PASS.
- [ ] **Step 5: Mutation probe:** replace the code-line rune wrap with no wrap and confirm `TestRenderCodeBlocks` goes red. Restore from backup.
- [ ] **Step 6: Commit:** `feat(pdfrender): render lists, task lists, code blocks, and blockquotes`.

---

### Task 4: GFM tables, and the frozen export corpus renders

**Build tier:** premium. Named risks: the header row not repeating after a page break, or a row splitting badly at one; and a corpus test that passes vacuously.

**Files:**
- Create: `internal/pdfrender/table.go`, `internal/pdfrender/table_test.go`, `internal/export/pdf_corpus_test.go`
- Modify: `internal/pdfrender/render.go` (dispatch tables), `internal/export/testdata/corpus/PROVENANCE.md` (one paragraph: the PDF corpus test and what the corpus does **not** hold for it — see Step 5)

**Interfaces:**
- Consumes: Tasks 2–3. In the corpus test it uses `export`'s existing test helpers `loadCorpus(t)` (`corpusFile.in`, `.rel`), `Transform`, and `pdfrender.Render`.
- Table rules: thin borders, and a bold header row (`faceSansBold`). Each column's natural width is its widest cell's single-line width plus padding. When the natural widths exceed the text width, scale the columns proportionally, with a minimum width so no column vanishes. Cell text wraps inside its column, inline styles apply, and a row's height is its tallest cell. Before each row, if it does not fit on the page, start a new page and redraw the header row first. A single row taller than a page is drawn starting on a fresh page and may run past it — fine for what the corpus holds; name it in a code comment.

- [ ] **Step 1: Write the failing tests (`table_test.go`):**
  - `TestRenderTableHeaderBold`: `| Name | Note |\n|---|---|\n| a | b |\n` → `Name` and `Note` in `faceSansBold`, `a` and `b` in `faceSans`, with border `line` ops present.
  - `TestRenderTableColumnWidths`: in `| x | a much longer cell text |`, the second column is wider than the first. A table whose cells hold 400 words in total stays within the text width. A long cell wraps to ≥ 2 distinct `y`, with every op inside its column. A 300-character unbroken token in a narrow column is split, and every piece stays inside the column.
  - `TestRenderTableHeaderRepeats`: a 150-row table → pages ≥ 2, and `Name` is drawn exactly once on each page that holds rows.
  - `TestRenderTableInlineAndNonBMP`: `| **b** | ` + "`c`" + ` | 🟡 |` → `b` bold, `c` mono with a tint, and one placeholder counted.
- [ ] **Step 2: Run** `go test -count=1 ./internal/pdfrender/ -run TestRenderTable`. Expect FAIL. Then implement `table.go` and run it again. Expect PASS.
- [ ] **Step 3: Write `TestCorpusRendersToPDF` in `internal/export/pdf_corpus_test.go`:** for every file from `loadCorpus(t)`, compute `out := Transform(f.in)` and `pdf, n, err := pdfrender.Render(out.Markdown, strings.TrimSuffix(path.Base(f.rel), ".md"))`. Assert `err == nil`, that `pdf` starts with `%PDF-`, and that `n == countAboveBMP(out.Markdown)`. `countAboveBMP` counts runes > 0xFFFF that are outside the invisible ranges Task 2's `resolve` drops; write it independently in the test. Every BMP rune in the corpus is drawable by the fonts, so the count is exact: a font-fallback regression turns covered runes like `—` into placeholders and reddens it. Population floor: the sum of `n` over the corpus is ≥ 1, and `specs/2026-07-20-frontmatter-field-domain-guard-design.md` has `n == 1` (its `🟡` sits in a code span the transform keeps).
- [ ] **Step 4: Run** `go test -count=1 ./internal/export/ ./internal/pdfrender/` and `go vet ./internal/export/ ./internal/pdfrender/`. Expect PASS. Note both packages' wall time in your report.
- [ ] **Step 5: Update `PROVENANCE.md`:** the corpus feeds `TestCorpusRendersToPDF`. It holds one character above U+FFFF and none of the emoji, box-drawing, or math runes the renderer supports, so their coverage lives in `internal/pdfrender`'s own constructed tests.
- [ ] **Step 6: Mutation probes**, each restored from backup: make `faceSans.covers` always return false for `'—'` (the corpus test goes red on a count mismatch); remove the header redraw on a page break (`TestRenderTableHeaderRepeats` goes red).
- [ ] **Step 7: Commit:** `feat(pdfrender): render GFM tables; every export corpus file renders to PDF`.

---

### Task 5: Formats, files, PDF failure, and opening in `app.Export`

**Build tier:** premium. Named risk: a PDF failure that loses or hides the Markdown copy, and a result shape change that leaves a stale `exported_path` reader.

**Files:**
- Modify: `internal/app/export.go`, `internal/app/open.go` (the reason constant), `internal/app/export_test.go`, `internal/app/schema_revision_test.go` (or a new `TestSchemaExportResultFiles` beside it)
- Modify (so the `cli` package compiles in this task): `internal/cli/export_test.go` (`exportOK`), `internal/cli/presenter_test.go` (`TestPresenterWritesWarningsToStderr`)

**Interfaces:**
- Consumes: `pdfrender.Render`, `pdfrender.Placeholder`.
- Produces (package `app`):
  - `const ExportFormatMarkdown = "markdown"`, `const ExportFormatPDF = "pdf"`.
  - `type ExportFormatSet struct { Markdown, PDF bool }`.
  - `func ParseExportFormats(vals []string) (ExportFormatSet, error)`: splits each value on `,`, and accepts only the exact values `markdown` and `pdf`, in any order, duplicates allowed. An empty list or an empty element gives `--format needs a value — choose markdown, pdf, or both`. An unknown value gives `invalid --format value %q — choose markdown, pdf, or both`. Errors are one line.
  - `ExportOptions` gains `Formats string`: `""` means markdown and pdf; otherwise a comma-separated list that `ParseExportFormats` accepts. A string keeps `ExportOptions` comparable for the CLI tests.
  - `type ExportFile struct { Format string `json:"format"`; Path string `json:"path"`; Replaced bool `json:"replaced"` }`.
  - `ExportResult` drops `ExportedPath` and `Replaced`, and gains `Files []ExportFile `json:"files"`` (after `change_id`; always a list, `[]ExportFile{}` when nothing was written, in Markdown-then-PDF order). `Launched` now means the opened file was launched.
  - `func (r ExportResult) Path(format string) string`: that format's path, or `""`.
  - `HumanText`: one path per line for every file written, then `Message` on its own line when set.
  - `ReasonOpenPDFFailed = "pdf-failed" // rendering or writing the PDF failed; a requested Markdown copy is kept` in `open.go`'s reason block.
  - `var renderPDF = pdfrender.Render`: the package-level seam the tests replace.
  - `exportDestination(deps, outputPath, name string) (dest string, exact bool, f *openFailure)`: `exact` is true when the output path named a file rather than a directory or the default.
  - `exportCopy` gains `Files []ExportFile` and `Open string` (the path to open), replacing `Path`/`Replaced`.
- Flow of `writeCleanCopy(..., formats ExportFormatSet)`, in order:
  1. Read, compute the kind and `name := export.FileName(...)`. The PDF name swaps the `.md` extension for `.pdf`.
  2. Resolve **every** requested destination. With the default or a directory, each format's name goes there. With an exact file and one format, the path is used as given. With an exact file and both formats, each takes the path with its extension (`filepath.Ext` of the base, when the base has a stem before it) replaced by `.md`/`.pdf`, or appended when there is none.
  3. Run `protectedDestination` on every destination, then transform and check, before writing anything.
  4. Markdown (when requested): `writeExport`. On failure, return today's `export-failed`, with nothing further attempted.
  5. PDF (when requested): `renderPDF(out.Markdown, <name stem>)`, then `writeExport`. A render error gives `could not render the PDF: <err>`; a write error gives `could not write <pdf>: <err>`. Both are `ResultExternalFailed` + `pdf-failed`, with ` — the Markdown copy is at <md path>` appended when Markdown was written. When `n > 0`, add the placeholder note (singular/plural copy from Global Constraints).
  6. One `replaced <path>` note per replaced file. Warnings are filtered by `--warnings` exactly as today, whatever the formats.
- `Export` validates `o.Formats` (when non-empty) right after `ValidateExportWarnings`, refusing with `ResultInvalidInput` + `ReasonOpenInvalidTarget` before any read. It copies `cp.Files`, `cp.Leftovers`, and `cp.Mentions` into the result on every outcome after `writeCleanCopy` returns, so a PDF failure still lists the Markdown file and its warnings. It opens only when there was no failure, no `Print`, and no `OutputPath`: the PDF when requested, else the Markdown.

- [ ] **Step 1: Retarget the existing tests in `export_test.go`.** Replace `res.ExportedPath` with `res.Path(ExportFormatMarkdown)` and `res.Replaced` with that file's `Replaced`. Tests whose subject is Markdown-only behaviour set `Formats: "markdown"`: `TestExportIgnoresGitHubRouting`, `TestExportKeepsStalenessNote`, `TestExportTicketNameAndH1`, `TestExportWarningsFilter`, `TestExportAmbiguousIDLeftAsWritten`, and `TestExportOutputPath`'s `file` and `relative` exact-file cases. Destination, opening, protection, and opener-failure tests keep the default (both):
  - `TestExportWritesCleanCopy`: the destination holds exactly 2 entries (`…spec-widget.md`, `…spec-widget.pdf`; no temp file), both mode 0644. The `.md` bytes equal `exportSpecClean`, and the `.pdf` starts with `%PDF-`. `Files` is `[{markdown, md, false}, {pdf, pdf, false}]`, and `HumanText() == md+"\n"+pdf`.
  - `TestExportLaunchesOnlyAtTheDefaultDestination`: the default case opens exactly the `.pdf` path.
  - `TestExportOpenerFailureKeepsThePath`: `HumanText()` starts with `md+"\n"+pdf+"\n"`, and both files exist.
  - `TestExportRefusesProtectedDestination`: unchanged cases, both formats. Each refuses, and the checkout directory still holds only the source. Add a case `Formats: "pdf"` with `OutputPath` = the source itself: refused, and the source is unchanged.
- [ ] **Step 2: Write the new failing tests:**
  - `TestParseExportFormats`: ok — `{"markdown"}`→`{true,false}`, `{"pdf"}`→`{false,true}`, `{"markdown,pdf"}`, `{"pdf","markdown"}`, `{"pdf","pdf"}`→`{false,true}`. Bad (one-line, naming `markdown, pdf`) — `nil`, `{}`, `{""}`, `{"markdown,"}`, `{"html"}`, `{"PDF"}`.
  - `TestExportRefusesInvalidRequest`: add `{What: "spec", ID: 7, Formats: "html"}` and `Formats: "markdown,"` (same refusal shape, no pin read).
  - `TestExportFormatsWriteAndOpen`: table over `Formats` `""`, `"markdown"`, `"pdf"`, `"markdown,pdf"`, `"pdf,markdown"` at the linux default destination. Check exactly the requested files on disk, `Files` formats in Markdown-then-PDF order, `Launched`, and the opener target (the `.pdf` when PDF is included, else the `.md`).
  - `TestExportOutputPathFormats`: both formats with an exact `dir/report.md`, `dir/report`, or `dir/report.txt` → `dir/report.md` + `dir/report.pdf`. `Formats: "pdf"` with exact `dir/x.md` → PDF bytes (`%PDF-`) at exactly `dir/x.md`. Re-exporting both into the same directory → the second run's two entries both have `replaced: true`, with notes `replaced <md>` and `replaced <pdf>`.
  - `TestExportPDFFailureKeepsMarkdown`: with `renderPDF` stubbed to return an error, at the **default** destination: `ResultExternalFailed`, `Reason == ReasonOpenPDFFailed`, `Files == [{markdown, md, false}]`, the `.md` exists with `exportSpecClean` bytes, and there is no `.pdf` and no temp file. `Launched == false`, the opener is untouched, `Message` is one line containing the `.md` path, and `HumanText() == md+"\n"+Message`. Same stub with `Formats: "pdf"`: `Files` is empty, the destination is empty, and the message names no Markdown path. A write failure (a directory pre-created at the `.pdf` destination, real renderer) gives `pdf-failed` with the Markdown kept.
  - `TestExportMarkdownWriteFailureStops`: output directory mode 0555 (skip when `os.Geteuid() == 0`), with a counting `renderPDF` stub → `export-failed`, render calls == 0, and `Files` empty.
  - `TestExportPlaceholderNote`: a spec body holding `🟡 and 🗑` → notes contain `2 characters the PDF fonts cannot draw were replaced with □`. One such character gives `1 character the PDF fonts cannot draw was replaced with □`. `Formats: "markdown"` gives no such note.
  - `TestExportWarningsFilter`: add a `Formats: "pdf"` row whose `Leftovers`/`Mentions` equal the `markdown` row's.
  - `TestExportResultJSON`: `files` decodes to `[{"format":"markdown","path":"/d/x.md","replaced":true}]`, and neither `exported_path` nor a top-level `replaced` is present.
  - `TestSchemaExportResultFiles` (using `schemaKeyPaths`): `export RES files`, `files.format`, `files.path`, and `files.replaced` are present; `export RES exported_path` and `export RES replaced` are absent.
- [ ] **Step 3: Run** `go test -count=1 ./internal/app/ -run 'Export|Schema'`. Expect FAIL (compile).
- [ ] **Step 4: Implement** per the Interfaces and flow above.
- [ ] **Step 5: Keep `internal/cli` compiling and pinned.** `exportOK(path, notes...)` builds `Files: []app.ExportFile{{Format: app.ExportFormatMarkdown, Path: path}}`, so `TestExportCLIPrintsThePath` keeps its exact stdout. `TestPresenterWritesWarningsToStderr` gives two `Files` (md, pdf) and expects stdout `md\npdf\n`. Add `TestExportCLIPDFFailureExits`: a stubbed `pdf-failed` result with one Markdown file → non-zero exit, and stdout carries the md path then the message. Confirm the exit code against the presenter's mapping for `external-failed`.
- [ ] **Step 6: Run** `go test -count=1 ./internal/app/ ./internal/cli/ ./internal/export/` and `go vet ./internal/app/ ./internal/cli/`. Expect PASS. Note `./internal/app/`'s wall time against Task 4's note.
- [ ] **Step 7: Mutation probes**, each restored from backup: write the PDF before the Markdown and stop on its failure (`TestExportPDFFailureKeepsMarkdown` red); open the `.md` even when PDF is included (`TestExportFormatsWriteAndOpen` red); drop the extension swap (`TestExportOutputPathFormats` red).
- [ ] **Step 8: Commit:** `feat(export): write the PDF beside the Markdown copy and report every file written`.

---

### Task 6: The `--format` flag, help text, and CLI reference

**Build tier:** standard

**Files:**
- Modify: `internal/cli/export.go`, `internal/cli/export_test.go`, `internal/cli/capability_production_test.go`, `docs/reference/cli.md`

**Interfaces:**
- Consumes: `app.ParseExportFormats`, `ExportOptions.Formats`.
- Flag: `cmd.Flags().StringSlice("format", nil, "which `formats` to write: markdown, pdf, or both, comma-separated or repeated (default: markdown and pdf)")`. The default is nil, so the signature carries no `=` default. In `exportOptionsFromFlags`, only when `f.Changed("format")`: `vals, _ := f.GetStringSlice("format")`, refuse on `app.ParseExportFormats(vals)`'s error, else `o.Formats = strings.Join(vals, ",")`.
- `Short`: `Write a clean Markdown copy and a PDF of a change, spec, plan, or results file`.
- `Long` (exact copy):

  ```
  Without an id, exports the change whose recorded branch is checked out. It
  always reads the local metadata checkout. The copy drops the frontmatter,
  docket's generated blocks and markers, every link (keeping its text), and
  docket-only sections; explicit change references become quoted titles. Fenced
  code and inline code stay exactly as written.

  --format picks the files to write: markdown, pdf, or both (the default),
  comma-separated or repeated. Both share one name, <date>-<kind>-<name>.md and
  <date>-<kind>-<name>.pdf, kind one of ticket, spec, plan, or results. The PDF
  is the Markdown copy rendered, nothing more.

  By default the files land in ~/Downloads on macOS or the home directory on
  Linux, and one opens: the PDF when the export includes it, otherwise the
  Markdown. --output-path names a directory to write into or the exact file to
  write, and opens nothing; with both formats an exact file takes each format's
  extension (report.md and report.pdf). --print opens nothing wherever the files
  go. An existing file is replaced. If the PDF fails, the Markdown copy is kept,
  nothing opens, and the command exits with an error. Leftover docket machinery
  and mentions of docket are reported as warnings on stderr; --warnings picks
  which. Warnings never block.
  ```

- [ ] **Step 1: Write the failing tests.** In `TestExportCLIPassesParsedOptions`, add `--format pdf` → `Formats: "pdf"`, `--format markdown --format pdf` → `"markdown,pdf"`, and `--format markdown,pdf` → `"markdown,pdf"`; the existing no-flag rows keep `Formats: ""`. In `TestExportCLIUsageErrors`, add `{"export","spec","5","--format",""}`, `--format html`, `--format markdown,`, and `--format PDF` (exit 2, one line, runner never called). Add `TestExportCLIHelpDescribesFormats`: `export --help` contains `--format`, `markdown, pdf, or both`, `the PDF when the export includes it`, `report.md and report.pdf`, and `If the PDF fails, the Markdown copy is kept`, after collapsing whitespace. `TestExportCLIHelpNamesTicketKind` stays as is. In `TestRepresentativeSignatures`, set `"export": "<type> [<id>] [--format <formats>...] [--output-path <path>] [--print] [--repo-dir <dir>] [--warnings <which>=all]"`, confirmed against the measured catalog, since the projection owns the exact string.
- [ ] **Step 2: Run** `go test -count=1 ./internal/cli/ -run 'Export|Signatures'`. Expect FAIL.
- [ ] **Step 3: Implement** the flag, the option parsing, `Short`, `Long`, and the `--output-path` usage (`directory or exact file `path` for the files, which are then not opened (default: ~/Downloads on macOS, the home directory on Linux)`).
- [ ] **Step 4: Rewrite the `docs/reference/cli.md` bullet:** `docket export` writes a clean Markdown copy and a PDF of a change, spec, plan, or results file, free of docket's links and generated blocks, and opens the PDF; `--format` picks `markdown`, `pdf`, or both. Keep the sentences `Without an id it exports the change whose branch is checked out.` and `Agent sessions reach the export through the \`docket-export\` skill.` (a prose sentinel pins the second), and `Details: \`docket export --help\`.`
- [ ] **Step 5: Run** `go test -count=1 ./internal/cli/ ./internal/repoguard/` and `go vet ./internal/cli/`. Expect PASS.
- [ ] **Step 6: Mutation probe:** drop the `f.Changed("format")` validation and confirm `TestExportCLIUsageErrors` goes red. Restore from backup.
- [ ] **Step 7: Commit:** `feat(cli): add docket export --format`.

---

### Task 7: The `docket-export` skill asks for Markdown only

**Build tier:** economy

**Files:**
- Modify: `skills/docket-export/SKILL.md`, `internal/repoguard/prose_contracts_test.go` (the `change_0578_export_skill` entries for that file), `internal/repoguard/budgets_test.go` (the `docket-export/SKILL.md` row), `docs/reference/glossary.md` (the `### docket-export` entry)
- Regenerate: `internal/assets/embedded/tree/skills/docket-export/SKILL.md` (`go generate ./internal/assets/`)

**Interfaces:**
- Consumes: Task 5's result shape (`files` entries with `format`/`path`/`replaced`) and Task 6's flag.

- [ ] **Step 1: Update the sentinel first (red).** In the `skills/docket-export/SKILL.md` `present` list, replace the two clauses with `` Run the `export` argv as `<export argv> <what> [<id>] --format markdown --print --json --repo-dir <repo root>` `` and ``If `result` is not `applied` or `files` has no `markdown` entry, tell the person the `message` and stop``. Add `` Always `--format markdown`: the rewrite pass edits the Markdown copy, and a PDF written beside it would keep the unrewritten text. `` Add `exported_path` to that entry's `absent` list. Run `go test -count=1 ./internal/repoguard/` and expect FAIL on the `change_0578_export_skill` sentinel.
- [ ] **Step 2: Edit the skill.** §3: the argv line above, plus the `Always --format markdown` sentence right after the existing `Always --print --json` sentence. The failure rule as above. §6: `Read the file at the \`path\` of the \`markdown\` entry in \`files\``. §7: `whether an existing file was replaced (that entry's \`replaced\`)`. No `exported_path` remains. The body ships into other repositories, so write nothing that is only true here.
- [ ] **Step 3: Glossary.** In `### docket-export`, change `(\`--print --json\`)` to `(\`--format markdown --print --json\`)`.
- [ ] **Step 4: Regenerate and budget.** Run `go generate ./internal/assets/`. Set the `docket-export/SKILL.md` budget row to the file's exact new `wc -l`/`wc -w` counts and prepend `0594: the export asks for Markdown only and reads the markdown entry of files (62/1144 -> <lines>/<words>); ` to its comment.
- [ ] **Step 5: Run** `go test -count=1 ./internal/repoguard/ ./internal/assets/...`. Expect PASS (prose contracts, skill budgets, embedded-tree drift).
- [ ] **Step 6: Mutation probe:** put `exported_path` back into §6 and confirm the sentinel's `absent` clause goes red. Restore from backup.
- [ ] **Step 7: Commit:** `feat(skills): the export skill asks for Markdown only and reads files`.
