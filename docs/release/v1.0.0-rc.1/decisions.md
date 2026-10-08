# v1.0.0-rc.1 — decisions and STOPs

Change 0544. Every Phase 0 decision, the source diff since alpha.2, the coverage statement, every STOP.

## Phase 0 — before the cut (2026-10-08)

- Operator: Daniel, with the attended agent session.
- No-loops attestation: Daniel, 2026-10-08: "no loop or autonomous session is running" against the repository.
- Backlog: no change `in-progress` other than 0544 (after the claim); `gh pr list --state open` is empty.
- No harness dry run (spec).

## Phase 1 — cut the candidate (recorded 2026-10-08T21:10Z)

```
d0f4712a53bd6a158fddada91cdf612f1755a531 1791489251 docs(release): correct v1.0.0-alpha.2 OpenCode wording after rc.1 replan
git ls-remote origin refs/heads/main: d0f4712a53bd6a158fddada91cdf612f1755a531	refs/heads/main
```

- `docket status --json`: no change in-progress before the claim; 0 errors, 0 warnings.
- Claimed 0544, workspace prepared, record reconciled, pointer plan attached (attended protocol).
- **Freeze rule in force:** every later phase re-probes `git ls-remote origin refs/heads/main` and requires `d0f4712a53bd6a158fddada91cdf612f1755a531`.

### Source added over the last human-tested build (`v1.0.0-alpha.2`)

`git diff --stat v1.0.0-alpha.2 d0f4712a5 -- . ':!docs'`: empty — no source change.

`git log --oneline v1.0.0-alpha.2..d0f4712a5` (11 commits, all under `docs/release/`):

```
d0f4712a5 docs(release): correct v1.0.0-alpha.2 OpenCode wording after rc.1 replan
647f79053 docs(release): v1.0.0-alpha.2 evidence index
5eb4fac35 docs(release): v1.0.0-alpha.2 public install check
fca9c3cee docs(release): v1.0.0-alpha.2 publication record
e44c85c54 docs(release): v1.0.0-alpha.2 notes final; publish decision
36084714a docs(release): draft v1.0.0-alpha.2 release notes
96dfb87af docs(release): v1.0.0-alpha.2 Phase 3 Cursor record
29d362798 docs(release): v1.0.0-alpha.2 Phase 3 isolation probe and kill finding
81b721039 docs(release): v1.0.0-alpha.2 Phase 3 setup
d0f3503cc docs(release): v1.0.0-alpha.2 Phase 2 candidate evidence
b14bba595 docs(release): v1.0.0-alpha.2 Phase 0 and Phase 1 decisions
```
