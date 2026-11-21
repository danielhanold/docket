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

## Decision

docket renders PDF exports in-process with a docket-owned renderer (`internal/pdfrender`) built on `github.com/yuin/goldmark` (parse, with GFM extensions) and `codeberg.org/go-pdf/fpdf` v0.12.0 (draw). DejaVu 2.37 (Sans x4, Sans Mono x2) and a static regular instance of Noto Emoji are embedded uncompressed via go:embed, alongside their licences.

Font fallback is per rune: the style's DejaVu face, then (for mono) DejaVu Sans, then Noto Emoji, then a visible placeholder (□) counted in a visibility-only note. Because fpdf's UTF-8 support is BMP-only and panics on runes above U+FFFF, every such rune is screened to the placeholder before it reaches fpdf, and Render recovers panics as defence in depth.

Output is deterministic (fixed dates, sorted catalog), and PDF metadata never names docket.

## Consequences

The binary grows by about 5.5 MB (fonts about 4.3 MB). There is no colour emoji and no CJK coverage; non-BMP characters become placeholders (about 11 occurrences across 1,479 metadata documents). Layout code (wrapping, tables, page breaks) is docket-owned and is tested through a recorder canvas.

## Alternatives considered

- solworktech/md2pdf: heavy dependencies and theme patching.
- pandoc, headless Chrome, wkhtmltopdf: external runtime dependencies.
- unidoc/unipdf: commercial licence.
- stephenafamo/goldmark-pdf: downloads fonts at runtime and pulls grpc/otel dependencies.
- gopdf: handles non-BMP runes but needs hand-written wrapping, tables, and page breaks.
- Compressing the embedded fonts: about 35% saving, not worth the code now.
