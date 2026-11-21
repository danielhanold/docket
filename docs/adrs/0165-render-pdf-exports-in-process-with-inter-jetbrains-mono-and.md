---
id: 165
slug: 'render-pdf-exports-in-process-with-inter-jetbrains-mono-and'
title: 'Render PDF exports in-process with Inter, JetBrains Mono, and GitHub-weight headings'
status: 'Accepted'
date: '2026-11-21'
supersedes: [164]
reverses: []
relates_to: []
change: 594
---

## Context

Exports are shared with colleagues who prefer PDF over Markdown. docket must stay one self-contained, cross-compiled binary with no runtime dependencies and no network access, so PDF rendering cannot shell out to an external tool or download anything at runtime. ADR-0164 settled this with an in-process goldmark + fpdf renderer and embedded DejaVu fonts.

The human reviewed the rendered PDFs and found headings too heavy: DejaVu Sans Bold is a wide face much heavier than weight 600. They asked for the look of the GitHub theme used by another app of theirs, which renders in the system font SF Pro. Apple's licence forbids embedding or redistributing SF Pro in other software, so it cannot ship inside docket.

## Decision

Everything in ADR-0164 still holds except the embedded fonts and heading weight: docket renders PDF exports in-process with the docket-owned `internal/pdfrender` renderer on `github.com/yuin/goldmark` (GFM) and `codeberg.org/go-pdf/fpdf` v0.12.0; fonts are embedded uncompressed via go:embed alongside their licences; output is deterministic and PDF metadata never names docket.

Embedded fonts:
- Body text: Inter 4.1 (Regular, Italic, SemiBold, SemiBoldItalic; SIL OFL 1.1). Bold is drawn in Inter SemiBold (weight 600), matching GitHub's stylesheet for headings and strong text.
- Code: JetBrains Mono 2.304 (Regular, SemiBold; SIL OFL 1.1).
- DejaVu Sans 2.37 Regular only, as a symbol fallback (box-drawing, some math symbols, task-list boxes, and the placeholder glyph). The other five DejaVu files are removed.
- Noto Emoji (static regular instance), unchanged.

Font fallback is per rune: the style's primary face, then (for mono) the upright Inter face of the same weight, then DejaVu Sans, then Noto Emoji, then the visible placeholder (□, drawn in DejaVu Sans) counted in a visibility-only note. The BMP-only screening of runes above U+FFFF and Render's panic recovery are unchanged from ADR-0164.

Heading sizes follow GitHub's proportions to the body size: h1-h6 at 2, 1.5, 1.25, 1, 0.875, and 0.85 em.

## Consequences

PDFs read closer to GitHub-rendered Markdown, with lighter, narrower headings and strong text. Embedded font files shrink by about 0.4 MB relative to ADR-0164 (about 2.66 MB of DejaVu removed, about 2.22 MB of Inter and JetBrains Mono added). Symbols only DejaVu covers now render at regular weight even inside bold text. Colour emoji, CJK coverage, and non-BMP placeholders are unchanged from ADR-0164.

## Alternatives considered

- SF Pro / SF Mono: the look the human asked for, but Apple's licence forbids embedding or redistributing them in other software.
- Keep DejaVu and draw headings in its regular weight: DejaVu has no medium or semibold cut, so headings would lose their emphasis.
- Keep DejaVu Sans Bold for bold fallback symbols: about 0.7 MB for rare cases; bold symbols fall back to regular weight instead.
- The alternatives ADR-0164 rejected for the renderer itself (md2pdf, pandoc/Chrome/wkhtmltopdf, unipdf, goldmark-pdf, gopdf, font compression) stay rejected for the same reasons.
