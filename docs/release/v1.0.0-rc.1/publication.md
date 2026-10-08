# v1.0.0-rc.1 — publication (Phase 4)

Each step: probe → act only if absent → verify → record. Daniel's explicit "publish" is in `decisions.md`.

| Step | Probe before | Action | Verified |
|---|---|---|---|
| Freeze | `git ls-remote origin refs/heads/main` = `d0f4712a53bd6a158fddada91cdf612f1755a531` | — | equal before tagging and again before publishing |
| Tag | `refs/tags/v1.0.0-rc.1` absent | `git tag -a v1.0.0-rc.1 d0f4712a5… -m "docket v1.0.0-rc.1"`, pushed | tag object `5df38f857fb0255db7b78ad86757da562327678a` peels to `d0f4712a53bd6a158fddada91cdf612f1755a531` |
| Release | `gh release view`: not found | `gh release create v1.0.0-rc.1 --verify-tag --draft --title "v1.0.0-rc.1 — release candidate" --notes-file release-notes.md` | draft, not a pre-release |
| Assets | none present | the six files from the read-only copy uploaded (no `--clobber`) | exactly six; each name and GitHub digest equal to the read-only copy |
| Publish | draft | `gh release edit v1.0.0-rc.1 --draft=false --prerelease=false --latest` | published 2026-10-08T21:36:34Z, not a pre-release; `v1.0.0-rc.1` is "Latest"; tag re-probed at the candidate |

Release: https://github.com/danielhanold/docket/releases/tag/v1.0.0-rc.1

| Asset | Asset id | Size | Digest |
|---|---|---|---|
| `checksums.txt` | RA_kwDOSsVhTs4lI1r6 | 495 | `sha256:28b9757ade9df58530fde8ac4a4bf9b0ded8cc8d4221a26593c293c8652653c5` |
| `docket_v1.0.0-rc.1_darwin_amd64.tar.gz` | RA_kwDOSsVhTs4lI1s3 | 8953698 | `sha256:6e8b3eb2c8b418ab47f76d6886a75c7170b4cf93b273930fccfc8df2f6a83081` |
| `docket_v1.0.0-rc.1_darwin_arm64.tar.gz` | RA_kwDOSsVhTs4lI1vT | 8498887 | `sha256:6ffe40c70127cc862ad494a2287f39cf688c1fe3e40d1b4c3ae7f50b04fb2637` |
| `docket_v1.0.0-rc.1_linux_amd64.tar.gz` | RA_kwDOSsVhTs4lI1xH | 8783247 | `sha256:5a25d7ae49fd1401f40b5dfd0ace3eba72c83f58e687dc15d7aa28a6942492ea` |
| `docket_v1.0.0-rc.1_linux_arm64.tar.gz` | RA_kwDOSsVhTs4lI10P | 8144677 | `sha256:911a529d2e31a390df379fad38f6d4ba9ce0d1592c291d052ac63a0798a56301` |
| `install.sh` | RA_kwDOSsVhTs4lI12t | 13663 | `sha256:52fb4dcb78648db02948632713a08120443abe15681b19e338e139bfb6321608` |
