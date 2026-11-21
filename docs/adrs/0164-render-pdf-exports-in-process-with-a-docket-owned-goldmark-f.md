---
id: 164
slug: 'render-pdf-exports-in-process-with-a-docket-owned-goldmark-f'
title: 'Render PDF exports in-process with a docket-owned goldmark + fpdf renderer and embedded fonts'
status: 'Accepted'
date: '2026-11-21'
supersedes: []
reverses: []
relates_to: []
change: 594
---

## Context

Exports are shared with colleagues who prefer PDF over Markdown. docket must stay one self-contained, cross-compiled binary with no runtime dependencies and no network access, so PDF rendering cannot shell out to an external tool or download anything at runtime.

The PDFs should read like GitHub-rendered Markdown, whose stylesheet sets text in the system font (SF Pro on macOS) with headings and strong text at weight 600. Apple's licence forbids embedding or redistributing SF Pro in other software, so it cannot ship inside docket.

## Decision

docket renders PDF exports in-process with a docket-owned renderer (`internal/pdfrender`) built on `github.com/yuin/goldmark` (parse, with GFM extensions) and `codeberg.org/go-pdf/fpdf` v0.12.0 (draw). Fonts are embedded uncompressed via go:embed, alongside their licences:

- Body text: Inter 4.1 (Regular, Italic, SemiBold, SemiBoldItalic; SIL OFL 1.1). Bold is drawn in Inter SemiBold (weight 600), matching GitHub's stylesheet for headings and strong text.
- Code: JetBrains Mono 2.304 (Regular, SemiBold; SIL OFL 1.1).
- Symbol fallback: DejaVu Sans 2.37 Regular (box-drawing, some math symbols, task-list boxes, and the placeholder glyph).
- Emoji: a static regular instance of Noto Emoji (SIL OFL 1.1).

Font fallback is per rune: the style's primary face, then (for mono) the upright Inter face of the same weight, then DejaVu Sans, then Noto Emoji, then a visible placeholder (□, drawn in DejaVu Sans) counted in a visibility-only note. Because fpdf's UTF-8 support is BMP-only and panics on runes above U+FFFF, every such rune is screened to the placeholder before it reaches fpdf, and Render recovers panics as defence in depth.

Heading sizes follow GitHub's proportions to the body size: h1-h6 at 2, 1.5, 1.25, 1, 0.875, and 0.85 em.

Output is deterministic (fixed dates, sorted catalog), and PDF metadata never names docket.

## Consequences

The binary grows by about 6 MB (fonts about 3.9 MB). Symbols only DejaVu covers render at regular weight even inside bold text. There is no colour emoji and no CJK coverage; non-BMP characters become placeholders (about 11 occurrences across 1,479 metadata documents). Layout code (wrapping, tables, page breaks) is docket-owned and is tested through a recorder canvas.

## Alternatives considered

- SF Pro / SF Mono: the GitHub look, but Apple's licence forbids embedding or redistributing them in other software.
- DejaVu Sans and Sans Mono for all text: wide, and DejaVu has no medium or semibold cut, so headings were either too heavy (Bold) or lost their emphasis (Regular).
- Embedding DejaVu Sans Bold for bold fallback symbols: about 0.7 MB for rare cases; bold symbols fall back to regular weight instead.
- solworktech/md2pdf: heavy dependencies and theme patching.
- pandoc, headless Chrome, wkhtmltopdf: external runtime dependencies.
- unidoc/unipdf: commercial licence.
- stephenafamo/goldmark-pdf: downloads fonts at runtime and pulls grpc/otel dependencies.
- gopdf: handles non-BMP runes but needs hand-written wrapping, tables, and page breaks.
- Compressing the embedded fonts: about 35% saving, not worth the code now.
