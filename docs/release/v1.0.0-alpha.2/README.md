# v1.0.0-alpha.2 — release evidence

Acceptance and publication record for the second public Go build of Docket, with Cursor proven end to end (change 0512).

## Candidate

| | |
|---|---|
| Commit | `ec4c2b1841954c2d6a3dd018e1133ee762861137` |
| Workflow run | https://github.com/danielhanold/docket/actions/runs/37822474785 |
| `checksums.txt` SHA-256 | `c4f1623d92da91d379844d50ec47b305e1602a2c07654accec07286b40445ae5` |
| Tag | `v1.0.0-alpha.2` (object `b83912b0575b516563c40e600108b7c4b1e5cdf6`) |
| Release | https://github.com/danielhanold/docket/releases/tag/v1.0.0-alpha.2 (pre-release, not latest) |

## Gates

| Phase | Verdict | Record |
|---|---|---|
| 0 — Before the cut | pass (0543 merged; Cursor isolation dry run passed) | [decisions.md](decisions.md) |
| 1 — Cut and freeze | pass; `main` held at the candidate through publication | [decisions.md](decisions.md) |
| 2 — Package once | pass; `evidence.json` byte-exact, no waiver | [candidate/run.txt](candidate/run.txt) |
| 3 — Cursor lifecycle | pass, with recorded findings (trivial groom; build tier inferred; orphaned `cursor-agent` worker; subagent model efforts) | [harness/cursor.md](harness/cursor.md) |
| 4 — Publish | pass, at Daniel's explicit "publish" | [publication.md](publication.md) |
| 5 — Public install | pass | [public-install.md](public-install.md) |

Release notes as published: [release-notes.md](release-notes.md).
