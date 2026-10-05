# v1.0.0-alpha.1 — decisions and STOPs

## Phase 0 — before the cut (2026-10-04)

| Item | Decision | By | Evidence |
|---|---|---|---|
| 0502 | merged (archived done) | — | `fc719ac6d change 0502 final backlinks retargeted to archive` |
| 0511 | merged (archived done) | — | `49e4af94b change 0511 final backlinks retargeted to archive` |
| 0510 | merged (archived done) | — | `c499de79a change 0510 final backlinks retargeted to archive` |
| Open PRs | none (`gh pr list --state open` empty) | — | probed 2026-10-04 |
| Other in-progress changes | none besides 0366 | — | `docket status --json` |
| 0412 | known gap for the release notes (still `proposed`) | human | spec decision |
| No loops / autonomous sessions | attested | human (Daniel) | explicit attestation in session, 2026-10-04; one other interactive `claude` (s002, "empty") was named and covered by the attestation |

## Phase 1 — candidate and window

- Candidate SHA: `49e4af94b838ed11beb7ee32027eb122425632d9` (`change 0511 final backlinks retargeted to archive`, ct 1791152258)
- `docket status --json`: 0 error findings, 0 warning findings at metadata `c4096903931d`
- 0366 claimed; branch `chore/human-attended-v1-0-0-rc1-acceptance-and-publication`

## STOPs

### STOP 1 — Phase 2, evidence byte-equality (2026-10-04)

- Gate: `evidence.json` `checksums_txt` must be byte-equal to the bundle's `checksums.txt`.
- Reason: they differ only by the final newline. The workflow builds `$checksums` with a shell command substitution (`.github/workflows/release-candidate.yml`, `--arg checksums "$checksums"`), which strips trailing newlines. All five bundle digests recomputed and match the manifest; no unmatched lines.
- Resolution: human (Daniel) accepted as benign on 2026-10-04; candidate kept. No source change.
- Resumption probe: `git ls-remote origin refs/heads/main` = 49e4af94b838ed11beb7ee32027eb122425632d9.

### STOP 2 — Phase 3, finalize merge blocked on a private repo (2026-10-04)

- Gate: finalize merges the PR itself.
- Reason: finalize stopped at the branch-protection check (GitHub branch-rules API returns an error for a private repository on a plan without it). The operator merged PR #1 by hand; re-running finalize then archived and cleaned up.
- Resolution: human (Daniel) classed it as a **known gap** for alpha.1 — a Claude Code path limitation on private repos without the branch-rules API. It goes in the release notes. No source change.

### Observation — Phase 3, change 0523

`docket repository check` reported a clean, behind-only `.docket` worktree as `conflict`. This is the already-tracked change 0523. `docket repository prepare` synced it and the check went `healthy`. Not a new defect.
