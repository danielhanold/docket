# v1.0.0 — release evidence

Publication record for the stable `v1.0.0` release of the Go binary, a full release and "Latest" (change 0580). No harness test and no public-install check before publication, by the human's decision; the candidate is gated by the whole-suite source gate. The install docs on `main` are version-free (`releases/latest/download`) and merged before the cut. Claude Code and Cursor supported; OpenCode untested; Codex unsupported.

## Candidate

| | |
|---|---|
| Commit | `0cf3ab4de2fd81c5799106830d1e28ab60ad3109` |
| Workflow run | https://github.com/danielhanold/docket/actions/runs/40609036031 |
| `checksums.txt` SHA-256 | `f81ebe84999469ea3a58eb0019c7b2fc81aa01f46f877176ca2998b3e6909842` |
| Tag | `v1.0.0` (object `020fe977d9c2f53343a37aec473e20c4dfd5960d`) |
| Release | https://github.com/danielhanold/docket/releases/tag/v1.0.0 (full release, Latest) |

## Gates

| Phase | Verdict | Record |
|---|---|---|
| 0 — Docs to `main`, before the cut | pass (version-free docs PR #446 merged and suite-gated; no loops attested; no other change in progress; no open PRs) | [decisions.md](decisions.md) |
| 1 — Cut and freeze | pass; `main` held at the candidate through publication | [decisions.md](decisions.md) |
| 2 — Package once | pass; `evidence.json` byte-exact, Bash upgrade shard passed, no STOP lines | [candidate/run.txt](candidate/run.txt) |
| 3 — Coverage statement | none by design; recorded | [decisions.md](decisions.md) |
| 4 — Publish | pass, at Daniel's explicit "publish"; `latest` links serve the release files | [publication.md](publication.md) |
| 5 — Public install | none by design | [decisions.md](decisions.md) |

Release notes as published: [release-notes.md](release-notes.md).
