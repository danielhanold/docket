# v1.0.0-rc.2 — release evidence

Publication record for the second release candidate of the Go binary, a full release and "Latest" (change 0579). No harness test and no public-install check before publication: rc.2 is the build the human tests with, gated by the whole-suite source gate. Claude Code and Cursor supported; OpenCode untested; Codex unsupported.

## Candidate

| | |
|---|---|
| Commit | `4a64bc3bbb66f0368cc3e8d30d4031a596922f45` |
| Workflow run | https://github.com/danielhanold/docket/actions/runs/39952716960 |
| `checksums.txt` SHA-256 | `d4c636da1bce4851c923c0935267e2504e8f02bf76d2bb66235da7d4da0a9d42` |
| Tag | `v1.0.0-rc.2` (object `cae0fb4b2bca6cfc038ee8136a00a74a719443ba`) |
| Release | https://github.com/danielhanold/docket/releases/tag/v1.0.0-rc.2 (full release, Latest) |

## Gates

| Phase | Verdict | Record |
|---|---|---|
| 0 — Before the cut | pass (no loops attested; no other change in progress; no open PRs) | [decisions.md](decisions.md) |
| 1 — Cut and freeze | pass; `main` held at the candidate through publication | [decisions.md](decisions.md) |
| 2 — Package once | pass; `evidence.json` byte-exact, Bash upgrade shard passed, no STOP lines | [candidate/run.txt](candidate/run.txt) |
| 3 — Coverage statement | none by design; recorded | [decisions.md](decisions.md) |
| 4 — Publish | pass, at Daniel's explicit "publish" | [publication.md](publication.md) |
| 5 — Public install | none by design | [decisions.md](decisions.md) |

Release notes as published: [release-notes.md](release-notes.md).
