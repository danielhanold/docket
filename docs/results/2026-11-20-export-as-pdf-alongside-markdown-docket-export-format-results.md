<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0594 — Export as PDF alongside Markdown: docket export --format](../changes/archive/2026-11-21-0594-export-as-pdf-alongside-markdown-docket-export-format.md)**
<!-- docket:backlink:end -->

# Results: Export as PDF alongside Markdown

**Human action:** Important — open a few exported PDFs in a desktop viewer and check that they look right. No automated test can judge how a PDF looks.

## Outcome

`docket export` now has a `--format` flag that takes `markdown`, `pdf`, or both, given comma-separated or as a repeated flag. With no flag it writes both: a `.md` file and a `.pdf` file with the same name. When the export includes a PDF, the PDF opens; otherwise the Markdown does. The PDF is drawn from the same clean Markdown bytes by a new pure-Go package, `internal/pdfrender`. It uses goldmark to parse and fpdf to draw, with DejaVu and Noto Emoji (monochrome) fonts built into the binary.

A character the fonts cannot draw, including every character above U+FFFF, becomes `□`. The export then adds one note saying how many characters were replaced. If the PDF fails, the Markdown copy is kept, nothing opens, and the command exits with `pdf-failed`. The JSON result now has a `files` list of `{format, path, replaced}` in place of `exported_path` and `replaced`. The `docket-export` skill always asks for `--format markdown`. ADR-0164 records the choice of renderer and fonts.

## Human actions and testing

- **Important: check the PDFs by eye.** The tests check what gets drawn and where, but not how the page looks.
  - Prerequisites: build the branch (`go build -o /tmp/docket-pdf ./cmd/docket` in the feature worktree).
  - Steps: run `/tmp/docket-pdf export spec 594`, `/tmp/docket-pdf export plan 594`, and `/tmp/docket-pdf export results <id of a change whose results has a table>`. Each command should write a `.md` and a `.pdf` to `~/Downloads` and open the PDF.
  - Expected: the text is easy to read; code is in a monospaced font with its line breaks kept; tables have borders, and the header row repeats after a page break; `→ — ↩ ✅ ⚠` render as themselves; page numbers sit centred in the footer.
  - Why it matters: if you skip this, layout problems such as cramped spacing or odd column widths reach colleagues unseen.
  - Cleanup: delete the exported files from `~/Downloads`.

## Verification performed

- Full suite (`go run ./cmd/docket development test`) passed at the build head and again after the review fixes, at the final head. The budget report showed only screening notes (two BUDGET WATCH parallel-overrun streaks at 1/5) and no breaches.
- Binary size (`go build ./cmd/docket` on this machine): 18,042,338 bytes on main, 24,413,714 bytes on this branch, an increase of about 6.4 MB. The spec estimated about 5.5 MB.
- Render time: about 41 ms for a 30 KB document and about 13 ms for a tiny one, most of it spent registering the fonts (measured by the Task 2 worker). `go test -count=1` wall times: `./internal/pdfrender/` about 1.7 s, `./internal/export/` about 0.5 s (0.2 s before), `./internal/app/` about 4.9 s.
- Tests cover these cases: every file in the frozen corpus renders, and its placeholder count matches exactly; a character above U+FFFF gets a placeholder wherever it appears, and nothing panics; every Unicode scalar is screened; text that cannot be broken still wraps and the layout finishes; the table header repeats; a row taller than a page splits; parsing and refusals for `--format`; the `--output-path` extension rule; a PDF failure keeps the Markdown; which file opens for each format choice; the shape of `files` in `docket schema`. Mutation probes turned each key guard red.
- Review: a deep whole-branch review returned 1 important and 3 minor findings, and all four were fixed in-branch.

## Known issues and follow-ups

- **Characters above U+FFFF become `□`.** A document with emoji such as 🟡, 🟢, or 🗑 gets a placeholder for each, plus a note giving the count. This is confirmed and intended: fpdf v0.12.0 can only draw characters up to U+FFFF (ADR-0164). There is no workaround; the Markdown copy keeps the real characters. No action unless readers complain.
- **The capability catalog is 2 bytes under its 16 KB limit** (16,382 of 16,384). The next command or flag added to the catalog will trip `TestCapabilitiesPayloadWithinByteBudget`. That change will have to decide whether to raise the limit or trim text. Expect this when catalog content is next added.
- **A code span that wraps at a run of several spaces** starts its next line with the extra spaces. It is cosmetic, and only seen in long inline code with repeated spaces.
