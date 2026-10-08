# v1.0.0-rc.1 — release evidence

Publication record for the first release candidate of the Go binary, a full release and "Latest" (change 0544). Claude Code and Cursor tested (alpha.1, alpha.2) and covered here by the whole-suite source gate; OpenCode untested; Codex unsupported.

## Candidate

| | |
|---|---|
| Commit | `d0f4712a53bd6a158fddada91cdf612f1755a531` |
| Workflow run | https://github.com/danielhanold/docket/actions/runs/37844907335 |
| `checksums.txt` SHA-256 | `28b9757ade9df58530fde8ac4a4bf9b0ded8cc8d4221a26593c293c8652653c5` |
| Tag | `v1.0.0-rc.1` (object `5df38f857fb0255db7b78ad86757da562327678a`) |
| Release | https://github.com/danielhanold/docket/releases/tag/v1.0.0-rc.1 (full release, Latest) |

## Gates

| Phase | Verdict | Record |
|---|---|---|
| 0 — Before the cut | pass (no loops attested; no other change in progress; no open PRs) | [decisions.md](decisions.md) |
| 1 — Cut and freeze | pass; `main` held at the candidate through publication | [decisions.md](decisions.md) |
| 2 — Package once | pass; `evidence.json` byte-exact, no waiver | [candidate/run.txt](candidate/run.txt) |
| 3 — Harness test | none by design; coverage statement recorded | [decisions.md](decisions.md) |
| 4 — Publish | pass, at Daniel's explicit "publish" | [publication.md](publication.md) |
| 5 — Public install | pass | [public-install.md](public-install.md) |

Release notes as published: [release-notes.md](release-notes.md).
