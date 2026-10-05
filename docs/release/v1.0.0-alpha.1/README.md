# v1.0.0-alpha.1 — release evidence

Acceptance and publication record for the first public Go build of Docket (change 0366).

## Candidate

| | |
|---|---|
| Commit | `49e4af94b838ed11beb7ee32027eb122425632d9` |
| Workflow run | https://github.com/danielhanold/docket/actions/runs/37240575898 |
| `checksums.txt` SHA-256 | `2ab19e3d78a64895352561bbef3d3f35dadfca325d4052ae15f2f650ec153e27` |
| Tag | `v1.0.0-alpha.1` (object `9c3f4c54d34d11373aec59a928886b811c60e516`) |
| Release | https://github.com/danielhanold/docket/releases/tag/v1.0.0-alpha.1 (pre-release, not latest) |

## Gates

| Phase | Verdict | Record |
|---|---|---|
| 0 — Before the cut | pass | [decisions.md](decisions.md) |
| 1 — Cut and freeze | pass; `main` held at the candidate through publication | [decisions.md](decisions.md) |
| 2 — Package once | pass, with one human-waived STOP (trailing-newline difference in `evidence.json`) | [candidate/run.txt](candidate/run.txt), [decisions.md](decisions.md) |
| 3 — Claude Code lifecycle | pass, with recorded downgrades (manual merge on a private repo; verdict lines not captured; 0523 mislabel synced) | [harness/claude.md](harness/claude.md) |
| 4 — Publish | pass | [publication.md](publication.md) |
| 5 — Public install | pass | [public-install.md](public-install.md) |

Release notes as published: [release-notes.md](release-notes.md).
