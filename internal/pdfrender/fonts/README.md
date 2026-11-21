# Embedded PDF fonts

`internal/pdfrender` embeds these files with `//go:embed` and draws every PDF
with them, so rendering needs no network and no installed fonts. The files are
stored uncompressed, exactly as listed below.

## DejaVu Fonts 2.37 (text and code)

- `DejaVuSans.ttf`, `DejaVuSans-Bold.ttf`, `DejaVuSans-Oblique.ttf`,
  `DejaVuSans-BoldOblique.ttf`: body text and headings.
- `DejaVuSansMono.ttf`, `DejaVuSansMono-Bold.ttf`: inline code and code blocks.
- `LICENSE-DejaVu.txt`: the archive's `LICENSE` file (Bitstream Vera Fonts
  copyright, DejaVu changes, and the Arev Fonts copyright).

Source: the `ttf/` directory of
<https://github.com/dejavu-fonts/dejavu-fonts/releases/download/version_2_37/dejavu-fonts-ttf-2.37.tar.bz2>.
The font files are unmodified.

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
| `DejaVuSans.ttf` | `7da195a74c55bef988d0d48f9508bd5d849425c1770dba5d7bfc6ce9ed848954` |
| `DejaVuSans-Bold.ttf` | `e6476c1b80502924294eed40894c5b18e06c181444ca953e5334262df9c27724` |
| `DejaVuSans-Oblique.ttf` | `4af75fa16ee6d3ad43e1ecec41862c24954af26a55c6bb1ebb27bd486a50f5f4` |
| `DejaVuSans-BoldOblique.ttf` | `eb436dca0c2594b73d8b603b892e374fdfd8d885d25ffb4f18df4c4c0b49e50f` |
| `DejaVuSansMono.ttf` | `b4a6c3e4faab8773f4ff761d56451646409f29abedd68f05d38c2df667d3c582` |
| `DejaVuSansMono-Bold.ttf` | `bce60f1b4421acd9ea51ba6623d7024ecbe6817a953e3654df62a5e6bdf8f769` |
| `NotoEmoji-Regular.ttf` | `6f2d6800665145d37410afbd0150d30a314936b8a248e952c5c3e93e2453ba5d` |
| `LICENSE-DejaVu.txt` | `7a083b136e64d064794c3419751e5c7dd10d2f64c108fe5ba161eae5e5958a93` |
| `OFL-NotoEmoji.txt` | `500bb1ccf43df7bbb522112f9133a52b16e1c35e809632f5d8609b179152de5b` |
