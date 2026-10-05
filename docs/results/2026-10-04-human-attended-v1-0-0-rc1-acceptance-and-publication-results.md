<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0366 — v1.0.0-alpha.1 acceptance and publication (Claude Code)](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0366-human-attended-v1-0-0-rc1-acceptance-and-publication.md)**
<!-- docket:backlink:end -->
# v1.0.0-alpha.1 acceptance and publication (Claude Code) — Results

**Human action:** None needed to merge. `v1.0.0-alpha.1` is already tagged and published as a pre-release, and this PR only adds the evidence bundle. Optionally, delete or archive the disposable fixture repository `danielhanold/docket-accept-v1-0-0-alpha-1-claude`; the evidence keeps its PR URL.

## Outcome

Docket's first Go binary is public as [`v1.0.0-alpha.1`](https://github.com/danielhanold/docket/releases/tag/v1.0.0-alpha.1). It is a pre-release, not marked latest, so `v0.9.3` is still "Latest".

- It was cut from `main` at `49e4af94b838ed11beb7ee32027eb122425632d9` and packaged once by the release-candidate workflow.
- It carries six assets: four platform archives, `checksums.txt` and `install.sh`. Each asset's name, size and SHA-256 equals the read-only copy that every test used.
- Claude Code was tested end to end on a disposable private repository from a throwaway home folder, with a mid-run kill and resume.
- A fresh install from the public release URL passed.
- `main` stayed at the candidate from the cut through publication.

The evidence is in `docs/release/v1.0.0-alpha.1/`. Its `README.md` holds the candidate identity and the gate table.

## Human actions and testing

This was a human-attended release; Daniel was present at every gate. The gate table is the human-verify record:

| Phase | Verdict | Record |
|---|---|---|
| 0 — Before the cut | pass | `decisions.md` |
| 1 — Cut and freeze | pass | `decisions.md` |
| 2 — Package once | pass, with one human-waived STOP | `candidate/run.txt`, `decisions.md` |
| 3 — Claude Code lifecycle | pass, with recorded downgrades | `harness/claude.md` |
| 4 — Publish | pass, at Daniel's explicit "publish" | `publication.md` |
| 5 — Public install | pass | `public-install.md` |

## Verification performed

- **Package:** workflow run 37240575898 was green on all seven jobs. The source gate ran `SUITE files=78 passed=78 failed=0` on go1.26.8, including `test_go_integration_bashupgrade` (`rc=0 ok=4`). It produced four `BUDGET WATCH` screening lines and no `SERIAL CONFIRMED OVER BUDGET` line.
- **Candidate:** `evidence.json` named the candidate commit and `v1.0.0-alpha.1`. Every bundle file's SHA-256 matched `checksums.txt`, with no unmatched line.
- **Claude Code:** create, groom, implement (`docket-plan-writer`, `docket-build-economy`, `docket-review-lean`/`-standard`), SIGTERM kill, resume of the same change, and finalize. The terminal predicate passed: the record was archived `done` with a full-URL `pr:`, the PR merged, the greeting line is on `main`, status shows 0 errors, `repository check` is healthy, and the board is re-rendered.
- **Publication:** the tag object `9c3f4c54…` peels to the candidate. The release is a pre-release that isn't marked latest, and all six asset digests equal the read-only copy.
- **Public install:** `install.sh: OK` against the published `checksums.txt` before running it. Then `docket version` reported `v1.0.0-alpha.1` at the candidate commit, `install check` was clean, and `supported_target` was true.

## Known issues and follow-ups

### Two STOPs were waived by the human

- **The `evidence.json` checksum copy lost its trailing newline.** The workflow builds it with a shell command substitution, which strips the final newline, so the copy isn't byte-equal to `checksums.txt`. The digests themselves were all correct. A one-line workflow fix would make the check exact for the next alpha.
- **Finalize couldn't merge on the private fixture.** Its branch-protection check calls the GitHub branch-rules API, which this private repository's plan does not answer. Daniel merged by hand and re-ran finalize, which closed the change out. Daniel left this out of the release notes as a GitHub-side issue.

### Evidence gaps in the Claude Code record

- The printed `run-*` verdict lines from the kill and resume were not captured. The record uses transcript timestamps and `docket run verify --id 1` = `run-complete` instead.
- The kill came during review rather than right after the plan was attached. My poller watched the record's `plan:` field, which was set later than the plan commit.

### `repository check` mislabel seen in the field

After finalize, `repository check` called the behind-only `.docket` copy a conflict. That is change 0523. `docket repository prepare` cleared it, and the release notes list it as a known gap.

### Release-notes decisions

Daniel dropped 0412 from the notes' known gaps, overriding the spec, and dropped the private-repo merge block.

### Follow-ups for later

These come from the spec's list; none was minted:
- a tag-triggered publishing workflow for a later release;
- a decision on the undocumented repo-root `link-skills.sh`.
