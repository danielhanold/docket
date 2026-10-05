# v1.0.0-alpha.1 — publication (Phase 4)

Each step was probe → act only if absent → verify. Freeze probe before tagging:
`git ls-remote origin refs/heads/main` = `49e4af94b838ed11beb7ee32027eb122425632d9`.

## Tag

- Probe: `refs/tags/v1.0.0-alpha.1` absent.
- Act: `git tag -a v1.0.0-alpha.1 49e4af94b838ed11beb7ee32027eb122425632d9 -m "docket v1.0.0-alpha.1"`, pushed.
- Tag object: `9c3f4c54d34d11373aec59a928886b811c60e516`
- Peeled target: `49e4af94b838ed11beb7ee32027eb122425632d9` (= candidate)

## Release

- Probe: release absent.
- Act: `gh release create v1.0.0-alpha.1 --verify-tag --draft --prerelease --title "v1.0.0-alpha.1 — Docket is a Go binary" --notes-file release-notes.md`
- Release id: `403288686`
- URL: https://github.com/danielhanold/docket/releases/tag/v1.0.0-alpha.1

## Assets

Uploaded from the read-only copy without `--clobber`. Exactly six; name, size and SHA-256 equal to the copy.

| Asset | Id | Size | SHA-256 |
|---|---|---|---|
| `checksums.txt` | 611102001 | 507 | `2ab19e3d78a64895352561bbef3d3f35dadfca325d4052ae15f2f650ec153e27` |
| `docket_v1.0.0-alpha.1_darwin_amd64.tar.gz` | 611102003 | 7403914 | `0db0efc50e70e66bab4ca5de2838c42f8200fd1946cef2547abbb77331aebf12` |
| `docket_v1.0.0-alpha.1_darwin_arm64.tar.gz` | 611102002 | 7026462 | `ceb35a19c28db67f59eabe2e928ec48203c421242d15f06ec191d68f8881721a` |
| `docket_v1.0.0-alpha.1_linux_amd64.tar.gz` | 611102000 | 7219766 | `95cddeac2e236f7949dbf164277bbece068b3b51f71273e0f1c70e9752e74afe` |
| `docket_v1.0.0-alpha.1_linux_arm64.tar.gz` | 611101999 | 6691035 | `396622c46185dedbb5fb52b8cef9d83379ecc966025353b08ae21659b9f7a196` |
| `install.sh` | 611102101 | 11963 | `26f9b77114d80712b0abc33486f1ede046623541b32e5d8711a564ef4b89b92a` |

## Publish

- Act: `gh release edit v1.0.0-alpha.1 --draft=false --prerelease --latest=false`
- Verified: `isDraft: false`, `isPrerelease: true`, `publishedAt: 2026-10-05T01:07:41Z`.
- `releases/latest` is still `v0.9.3`.
- Tag re-probed after publish: still `9c3f4c54…` → `49e4af94b838…`.
