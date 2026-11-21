# Embedded PDF fonts

`internal/pdfrender` embeds these files with `//go:embed` and draws every PDF
with them, so rendering needs no network and no installed fonts. The files are
stored uncompressed, exactly as listed below.

## Inter 4.1 (body text and headings)

- `Inter-Regular.ttf`, `Inter-Italic.ttf`: body text.
- `Inter-SemiBold.ttf`, `Inter-SemiBoldItalic.ttf`: headings, bold text, and
  table headers. Docket draws bold in Inter's SemiBold (weight 600) cut, the
  weight GitHub's stylesheet uses for headings and `strong`.
- `OFL-Inter.txt`: the archive's `LICENSE.txt`, the SIL Open Font License 1.1.

Source: the `extras/ttf/` directory of
<https://github.com/rsms/inter/releases/download/v4.1/Inter-4.1.zip>.
The font files are unmodified.

## JetBrains Mono 2.304 (code)

- `JetBrainsMono-Regular.ttf`, `JetBrainsMono-SemiBold.ttf`: inline code and
  code blocks.
- `OFL-JetBrainsMono.txt`: the archive's `OFL.txt`, the SIL Open Font
  License 1.1.

Source: the `fonts/ttf/` directory of
<https://github.com/JetBrains/JetBrainsMono/releases/download/v2.304/JetBrainsMono-2.304.zip>.
The font files are unmodified.

## DejaVu Sans 2.37 (symbol fallback)

- `DejaVuSans.ttf`: draws the symbols Inter and JetBrains Mono lack, such as
  box-drawing characters, some math symbols, and the task-list boxes, and the
  placeholder `□`.
- `LICENSE-DejaVu.txt`: the archive's `LICENSE` file (Bitstream Vera Fonts
  copyright, DejaVu changes, and the Arev Fonts copyright).

Source: the `ttf/` directory of
<https://github.com/dejavu-fonts/dejavu-fonts/releases/download/version_2_37/dejavu-fonts-ttf-2.37.tar.bz2>.
The font file is unmodified.

## Google Noto Emoji (monochrome emoji fallback)

- `NotoEmoji-Regular.ttf`: the static regular-weight (`wght=400`) instance of
  the variable font `NotoEmoji[wght].ttf`.
- `OFL-NotoEmoji.txt`: the font's `OFL.txt`, the SIL Open Font License 1.1.

Source: <https://raw.githubusercontent.com/google/fonts/06bfe0d62a98375cb09c942fa18e68bd32cff057/ofl/notoemoji/>
(`google/fonts` commit `06bfe0d62a98375cb09c942fa18e68bd32cff057`), files
`NotoEmoji%5Bwght%5D.ttf` and `OFL.txt`.

The static instance was made with fontTools 4.68.0:

```sh
uvx --from fonttools fonttools varLib.instancer "NotoEmoji[wght].ttf" wght=400 -o NotoEmoji-Regular.ttf
```

`OFL-NotoEmoji.txt` declares no Reserved Font Name (its copyright line is
`Copyright 2013 Google LLC` with no reserved-name clause), so the instance keeps
the font's original names.

## Checksums

| file | sha256 |
| --- | --- |
| `Inter-Regular.ttf` | `40d692fce188e4471e2b3cba937be967878f631ad3ebbbdcd587687c7ebe0c82` |
| `Inter-Italic.ttf` | `bbc051dd204b5019a1aa0bc0ae2aa8a05ab13e7a3f979fa357631dc7feb6833a` |
| `Inter-SemiBold.ttf` | `78a843fade9d4612a5567302fb595b56976eb5fcebf4fea5a5912d638bafcde3` |
| `Inter-SemiBoldItalic.ttf` | `eff2930c3d3b35d3fcf5f76252b6baef4c3e907d9d2fde1d16cf5d417f8deef4` |
| `JetBrainsMono-Regular.ttf` | `a0bf60ef0f83c5ed4d7a75d45838548b1f6873372dfac88f71804491898d138f` |
| `JetBrainsMono-SemiBold.ttf` | `1b3bfa1ed5665a4ce3f9feb68d2d4e40e70bf8b4b7d9a3edd418f321b4e166a0` |
| `DejaVuSans.ttf` | `7da195a74c55bef988d0d48f9508bd5d849425c1770dba5d7bfc6ce9ed848954` |
| `NotoEmoji-Regular.ttf` | `6f2d6800665145d37410afbd0150d30a314936b8a248e952c5c3e93e2453ba5d` |
| `OFL-Inter.txt` | `262481e844521b326f5ecd053e59b98c8b2da78c8ee1bdbb6e8174305e54935a` |
| `OFL-JetBrainsMono.txt` | `30f0c136e3c88e422d0791acd97238870f9054a9729bc34cf2ff0d4ed8cac4ad` |
| `LICENSE-DejaVu.txt` | `7a083b136e64d064794c3419751e5c7dd10d2f64c108fe5ba161eae5e5958a93` |
| `OFL-NotoEmoji.txt` | `500bb1ccf43df7bbb522112f9133a52b16e1c35e809632f5d8609b179152de5b` |
