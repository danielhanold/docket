# v1.0.0-alpha.2 — publication (Phase 4)

Each step: probe → act only if absent → verify → record. Daniel's explicit "publish" is in `decisions.md`.

| Step | Probe before | Action | Verified |
|---|---|---|---|
| Freeze | `git ls-remote origin refs/heads/main` = `ec4c2b1841954c2d6a3dd018e1133ee762861137` | — | equal before tagging and again before publishing |
| Tag | `refs/tags/v1.0.0-alpha.2` absent | `git tag -a v1.0.0-alpha.2 ec4c2b1… -m "docket v1.0.0-alpha.2"`, pushed | tag object `b83912b0575b516563c40e600108b7c4b1e5cdf6` peels to `ec4c2b1841954c2d6a3dd018e1133ee762861137` |
| Release | `gh release view`: not found | `gh release create v1.0.0-alpha.2 --verify-tag --draft --prerelease --title "v1.0.0-alpha.2 — Cursor support" --notes-file release-notes.md` | draft, pre-release, 0 assets |
| Assets | none present | the six files from the read-only copy uploaded (no `--clobber`) | exactly six; each name, size, downloaded SHA-256 and GitHub digest equal to the read-only copy |
| Publish | draft | `gh release edit v1.0.0-alpha.2 --draft=false --prerelease --latest=false` | published 2026-10-08T19:37:30Z, pre-release; `v0.9.3` still "Latest"; tag re-probed at the candidate |

Release: https://github.com/danielhanold/docket/releases/tag/v1.0.0-alpha.2 (release id 407196726)

| Asset | Asset id | Size | Digest |
|---|---|---|---|
| `checksums.txt` | 622808146 | 507 | `sha256:c4f1623d92da91d379844d50ec47b305e1602a2c07654accec07286b40445ae5` |
| `docket_v1.0.0-alpha.2_darwin_amd64.tar.gz` | 622808150 | 8953700 | `sha256:92d3a2ab3461be040bbde6faa7617fba1703c25c4edd18f1151e2ef97abb24f7` |
| `docket_v1.0.0-alpha.2_darwin_arm64.tar.gz` | 622808155 | 8498901 | `sha256:846cd193fafd827f481c1310cfd9e0d6c708b123f9e15b6d8d81912c769fd808` |
| `docket_v1.0.0-alpha.2_linux_amd64.tar.gz` | 622808147 | 8783254 | `sha256:4b64a5cf3ec8d53da5ac00c0adc22e10d3a62840f41a0348d64baf66dc5a6c25` |
| `docket_v1.0.0-alpha.2_linux_arm64.tar.gz` | 622808284 | 8144675 | `sha256:1f2e7b74b92a83ca387238f7b942ec6eb8c05162213dd10366645cd4850e16c5` |
| `install.sh` | 622808152 | 13666 | `sha256:f1fe6ddf66e0ed524d3a9d38222c88b140bef89e483d71f06d4970a70727917d` |

## Post-publication notes edit (2026-10-08T19:41:36Z)

The OpenCode lines named an alpha.3 that change 0544 had already replaced (rc.1 ships OpenCode untested; full OpenCode support moved after v1.0.0, change 0513 deferred). At Daniel's "yes", the release body was edited with `gh release edit v1.0.0-alpha.2 --notes-file release-notes.md`. Only the two OpenCode lines changed; the tag, assets and pre-release/latest flags are untouched (re-verified: published, pre-release, 6 assets, `v0.9.3` Latest).

```diff
- - OpenCode is installable but untested; it is proven in alpha.3. Codex is paused.
+ - OpenCode is installable but untested; full OpenCode support comes after v1.0.0. Codex is paused.
- - OpenCode is installable but untested (alpha.3).
+ - OpenCode is installable but untested; full OpenCode support comes after v1.0.0.
```
