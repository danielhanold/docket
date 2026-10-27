<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0580 — Release v1.0.0: stable release with version-free install docs](../../changes/active/0580-release-v1-0-0-stable-release-with-version-free-install-docs.md)**
<!-- docket:backlink:end -->

# v1.0.0 — stable release publication

**Change:** 0580 · **Type:** chore · **Priority:** high · **Groomed:** 2026-10-27 · **Status:** Approved design

## Purpose and boundary

Take one commit of `main` to a verified, public `v1.0.0` **full release** of the Go binary, which
becomes the repository's "Latest" release in place of `v1.0.0-rc.2`.

v1.0.0 ships what the human has been running and testing since rc.2. On `main` since the
`v1.0.0-rc.2` tag there are only rc.2's own evidence commits and the logo (`095ec9327`). Nothing is
human-tested before publication and there is no separate public-install check: the human has
tested Claude Code and Cursor enough, and their own install from the release is the check.

This is rc.2's protocol (change 0579, spec
`2026-10-23-release-v1-0-0-rc-2-the-changes-since-rc-1-published-as-the-design.md`) with two
differences: the install docs become **version-free** and land on `main` **before** the cut, and the
release notes are a short product overview instead of a changelog. The steps are written out
below, so the rc.2 spec is background only.

This change builds no product code. Its one source edit is the pre-cut docs PR (*Phase 0*). If a
source defect turns up, it goes to a separate reviewed change, the candidate is discarded, and
packaging restarts from the new tip of `main` (see *Failure and retry boundary*).

**Run by hand, not by `docket-implement-next`.** The operator runs this protocol in an attended
session with the human present, because Phase 4 needs the human's explicit "publish". The change
is claimed for its branch and closed out through typed operations (Phases 1 and 6), never planned
or built by an autonomous run. If `docket-implement-next` is dispatched on it anyway, it stops
before claiming, as it did for rc.1 and rc.2, and the session carries on by hand. The change is not
on hold: the human runs it today, right after grooming.

## Decisions (human, 2026-10-27)

| Topic | Decision |
|---|---|
| Testing before publication | **None.** No harness test, no public-install check. The full test suite gates the candidate. |
| Release type | Full release `v1.0.0`; becomes "Latest", replacing `v1.0.0-rc.2`. |
| Release notes | One overview paragraph, a link to the install guide, a one-line harness status, and a feature highlight list. Nothing else (see *Release notes*). |
| Harness status | Unchanged from rc.2: Claude Code and Cursor **supported**; OpenCode **untested**; Codex **unsupported**. One line in the notes. |
| Install docs | Switch to the version-free `releases/latest/download` form with no `--version`, merged to `main` **before** the cut, so the install guide the notes link to is right the moment the release goes live, and no later release needs an install-doc edit. |
| Bash upgrade guide | Switches to `latest` too, in the same pre-cut PR. Nobody uses docket yet, so a frozen guide serving a later version is a theoretical risk; #514's "last verified against v1.0.0" freeze line gets reworded when that change is revived. |
| Hold | Not set. |
| Rollback | No rehearsal. Earlier releases stay available on GitHub. |

## Current reality (2026-10-27)

| Fact | Value |
|---|---|
| Latest tags | `v1.0.0-rc.2` (full release, "Latest", 2026-10-23, commit `4a64bc3bb`), `v1.0.0-rc.1`, `v1.0.0-alpha.2`, `v1.0.0-alpha.1`, `v0.9.3`, `v0.9.2`. |
| Since rc.2 | 7 commits on `main`: rc.2's evidence and install-doc commits, and the logo (`095ec9327`: `README.md` and `assets/docket-lockup-light.png`). |
| Backlog | No open PR. |
| Release tooling | `.github/workflows/release-candidate.yml` and `internal/release/` unchanged since rc.2. |
| Release installer | Each bundle's `install.sh` is stamped with its own version (`DOCKET_DEFAULT_VERSION` in `internal/release/downloader/install.sh`), so `install.sh` run without `--version` installs the release it came from. |
| Pinned install commands | `README.md` (*Install and the five steps*) and `docs/install/install.md` (*From a release*) each name `v1.0.0-rc.2` twice: the download base URL and the `--version` flag. `docs/release/upgrading-from-bash.md` §3 sets `VERSION=v1.0.0-alpha.2`, and §1 says "OpenCode follows in a later pre-release". |
| Guide test | `mirrorInstallBinary` (`internal/bashupgrade/registry_test.go`) accepts a block with no `VERSION=` line and `curl` lines ending in `/install.sh"` and `/checksums.txt"`, and requires the installer line to be `sh install.sh` with one `--harness` per covered harness. |
| Bundle | `docket_v1.0.0_{darwin,linux}_{amd64,arm64}.tar.gz`, `checksums.txt`, `install.sh`. |

## Protocol

Every docket operation is resolved from the capability catalog (`docket capabilities --json`);
request bodies come from `docket schema --operation <id> --json`.

### Phase 0 — Docs to `main`, then before the cut

1. **Version-free install docs.** On a short-lived branch cut from `origin/main` (outside this
   change; this change's own PR merges only after publication), open one docs-only PR. It is a
   plain, untracked PR — no change record of its own — referenced from `decisions.md` (my call).
   No human review stop before merging: show the human the diff as it goes in (my call):
   - `README.md` and `docs/install/install.md`: the base URL becomes
     `https://github.com/danielhanold/docket/releases/latest/download` and the installer line drops
     `--version <v>`. Adjust any surrounding sentence that names a version so it says the commands
     install the latest release; keep the rest of each section as it is. To pin a version, a reader
     can still pass `--version` — say so in one clause in `install.md` only if it fits naturally.
   - `docs/release/upgrading-from-bash.md`: in §3 drop the `VERSION=` line and point both `curl`
     lines at `https://github.com/danielhanold/docket/releases/latest/download/…`, keeping the
     block's other lines; in §1 "OpenCode follows in a later pre-release" becomes "OpenCode follows
     in a later release".
   - First grep the maintained docs (`git grep -n -E 'v1\.0\.0-(rc|alpha)'`, excluding
     `docs/release/v*`, results files and other point-in-time records) for any other pinned install
     command, and convert it the same way.
   - Gate it with the full suite (`build.test_command`) before merging; `TestIntegrationBashUpgrade`
     and the `internal/repoguard` prose contracts must stay green.
   - Merge with `gh pr merge <n> --rebase`. Record the merged commit in `decisions.md` (Phase 1
     creates the file; carry the SHA until then).
2. Record in `decisions.md`: no change other than this one is `in-progress`; every open PR is
   merged or recorded as held until after Phase 6; the human attests that no `/loop` or autonomous
   session is running against the Docket repository.

### Phase 1 — Cut the candidate and open the window

1. `git fetch origin`. The candidate SHA is `git rev-parse origin/main`, which must contain the
   Phase 0 docs merge (`git merge-base --is-ancestor <docs-commit> <candidate>`). Record
   `git log -1 --format='%H %ct %s' <sha>` and `git ls-remote origin refs/heads/main`.
2. Record verbatim in `decisions.md`: `docket status --json` (no `in-progress` change other than
   this one, zero error findings) and `gh pr list --state open`.
3. Record the source v1.0.0 adds over rc.2: `git log --oneline v1.0.0-rc.2..<candidate>` and
   `git diff --stat v1.0.0-rc.2 <candidate> -- . ':!docs'`.
4. Claim this change (`change.claim`, a supported attended claim with no run context), then
   `workspace.prepare`, then reconcile the record (`change.reconcile`) against the candidate, then
   attach a pointer plan through `change.attach-plan`: a short plan file that lists this spec's
   phases in order. Phase 6's `change.mark-implemented` refuses without the reconcile and the plan.
5. Commit and push each phase's evidence to the feature branch as it lands; the branch is the
   durable checkpoint.
6. **Freeze rule.** Every later phase re-probes `git ls-remote origin refs/heads/main` and requires
   the candidate SHA. A mismatch is a STOP. The human either re-cuts from the new tip (restart at
   Phase 2 with a fresh workflow run; the old candidate is discarded whole) or records each
   intervening commit as excluded from v1.0.0. The tag always targets the recorded candidate.

### Phase 2 — Package once

1. Dispatch the workflow:
   `gh workflow run release-candidate.yml --repo danielhanold/docket -f ref=<candidate SHA> -f version=v1.0.0`.
   Capture the run id, require its `headSha` to equal the candidate, and watch it.
2. Require conclusion `success` with every job green.
3. Download `candidate-head` and `candidate-evidence` into a fresh directory outside the
   repository. Verify:
   - `evidence.json` has `source_commit` equal to the candidate, `head_version` equal to
     `v1.0.0`, and `checksums_txt` byte-equal to the bundle's `checksums.txt`, trailing newline
     included;
   - every bundle file's recomputed SHA-256 matches a line of `checksums.txt`, with no unmatched
     line.
4. Make the copy read-only (`chmod -R a-w`). Record in `candidate/run.txt`: its path (sanitized) and
   SHA-256 listing, the run URL, the source gate's `go version`, the suite summary lines including
   every budget-report line, and the `test_go_integration_bashupgrade` result (Go prefix
   `TestIntegrationBashUpgrade`), which must have run and passed.
5. Every later phase reads **this copy**. A second workflow run, for any reason, makes a new
   candidate: the old copy is discarded and Phases 2–4 restart. Bundles are never mixed.

STOP on: any job not green; any evidence mismatch; any checksum mismatch; the Bash upgrade shard
absent, skipped or failing; an `OVER BUDGET` or `SERIAL CONFIRMED OVER BUDGET` line. `BUDGET WATCH`
and `PARALLEL-SENSITIVE` lines are screening notes: record them, they stop nothing.

**After a STOP here (my call).** Record the failure in `decisions.md`. If the human judges it is not
a source defect (a flaky test, a runner outage, a budget line from a slow runner), re-run the
workflow once at the same candidate, still as `v1.0.0`; that run is a new candidate under step 5. A
second failure of the same kind, or any source defect, is a STOP for the human: a source defect goes
to its own change, and packaging restarts from the new tip of `main`, still as `v1.0.0` and still
under this change.

### Phase 3 — Coverage statement (no test)

`decisions.md` records: no harness test and no public-install check, by the human's decision of
2026-10-27 (Claude Code and Cursor tested enough through rc.2); the candidate is gated by the full
suite in Phase 2; the downloader is unchanged since rc.1, whose public-install check proved no
quarantine attribute and a valid signature.

### Phase 4 — Publish (the human's irreversible step)

**Draft the notes first** (see *Release notes*) into `release-notes.md` on the feature branch, and
show them to the human. Before showing them, confirm the install link resolves to the Phase 0 page
on `main` and that every highlight names a feature the candidate actually has.

**Preconditions:**

- Phases 0–3 recorded and passing;
- `origin/main` re-probed and still the candidate;
- the notes reviewed and approved by the human;
- the human's explicit "publish" recorded in `decisions.md`.

Each step is probe → act only if absent → verify → record, in `publication.md`:

1. **Tag.** Probe `git ls-remote --tags origin refs/tags/v1.0.0`.
   - Absent: `git tag -a v1.0.0 <candidate SHA> -m "docket v1.0.0"`, push the tag, and verify the
     peeled target is the candidate.
   - Present at the candidate: continue.
   - Present elsewhere: STOP. Never move or delete a tag.
2. **Release.** Absent:
   `gh release create v1.0.0 --verify-tag --draft --title "v1.0.0" --notes-file release-notes.md`.
   Draft present: continue. Published: skip to step 4.
3. **Assets.** For each of the six files in the read-only copy: absent → upload it (never
   `--clobber`); present → download it and compare its SHA-256. A mismatch is a STOP; the human may
   delete the offending draft asset by hand and re-run this step.
4. **Verify.** Exactly six assets with the expected names, and sizes and digests equal to the
   read-only copy.
5. **Publish.** `gh release edit v1.0.0 --draft=false --prerelease=false --latest`. Verify the
   release is published, not a pre-release, and "Latest", and that `v1.0.0-rc.2` no longer is.
   Re-probe the tag.
6. **Latest links resolve.** Download
   `https://github.com/danielhanold/docket/releases/latest/download/install.sh` and
   `…/latest/download/checksums.txt` into a fresh directory; each must be byte-equal to the
   read-only copy's file. GitHub's `latest` redirect can lag briefly after publishing, so retry up
   to 5 attempts over about 5 minutes, recording each attempt (my call). A mismatch that persists
   past the last attempt is a STOP for the human (the release is already public; record it, never
   compensate automatically). This is a link check only, not an install.

Once published, the notes on GitHub are the human's: they may edit them by hand. Never overwrite
them afterwards. `release-notes.md` keeps the text as published at step 5.

### Phase 5 — (none)

No public-install check (see *Decisions*).

### Phase 6 — Closeout

The order matters: the final head is gated and published before the change is marked
implemented, or `pr.publish` returns `contended` and the PR body keeps stale evidence.

1. Assemble the evidence bundle on the feature branch. Write the results record; its
   human-actions section is the gate table, one line per phase, and an Optional item naming the
   two deferred changes now due: #514 (retire the saved Bash upgrade test cases; its planned
   "last verified against v1.0.0" freeze line needs rewording now that the guide points at
   `latest`) and #513 (full OpenCode support). This change does not revive or edit them.
2. Drive the build gate (`build.test_command`) on the feature worktree through the gate driver to
   `PASSED`.
3. Record evidence, attach results, and publish the PR. Its body carries the gate table.
4. Mark the change implemented.
5. Run `docket-finalize-change` on this change. The window closes at its merge.
6. Rebuild the installed binary from source as the repository's standing post-merge rule
   (`AGENTS.md`, *Rebuild the binary after a merge to main*) requires.
7. Run `docket-status`; it should come back clean. Held PRs may merge now.

**The human's own v1.0.0 install comes after closeout (my call).** The release downloader and a
source install write the same `~/.local/bin/docket` and share one install record, so on the
operator's machine the human installs v1.0.0 from the release only after step 6; otherwise the
post-merge rebuild would replace it with a source build of `main`. That install is outside this
change.

## Evidence bundle

```text
docs/release/v1.0.0/
  README.md               index: candidate identity, gate table (phase → verdict → file)
  decisions.md            Phase 0 docs merge and decisions, the source added since rc.2, coverage statement, every STOP
  candidate/evidence.json verbatim workflow artifact
  candidate/checksums.txt verbatim
  candidate/run.txt       run id/URL, toolchain, suite summary, Bash upgrade shard result, copy listing
  publication.md          tag object and peeled SHA, release id/URL, asset ids and digests, latest-link check
  release-notes.md        the notes as published
```

Sanitization: no tokens or credentials; home paths become `$HOME`. Once the closeout PR merges,
`results:` and the bundle are frozen build records.

## Release notes

Title: `v1.0.0`. Four parts, in this order, and nothing else — no upgrade steps, no changelog, no
known-gaps list, no evidence link.

1. **Overview.** One paragraph on what docket is, for someone landing on the release page: it
   tracks planned work as changes kept in your repository on their own metadata branch; you
   capture and design a change with your agent; an autonomous run builds it to an open pull
   request, tested and reviewed; finalize lands it and keeps the backlog current. Draw the wording
   from `docs/README.md` and `docs/guide/five-steps.md`.
2. **Install.** One line linking
   https://github.com/danielhanold/docket/blob/main/docs/install/install.md. No commands in the
   notes.
3. **Harness status.** One line: Claude Code and Cursor supported; OpenCode untested; Codex
   unsupported.
4. **Highlights.** A short bullet list drawn from the *Guide* section of `docs/README.md` — one
   bullet per area (capturing work, grooming before building, building unattended to a PR, the
   suite gate, review before the human, finalize, keeping the backlog honest, ADRs and learnings,
   the metadata branch) — plus any user-facing capability that page links to and the operator
   judges worth a bullet, such as private repositories or per-agent model pins. Each bullet is one
   plain sentence; link the guide page where it helps. The human approves the draft before Phase 4.

## Failure and retry boundary

STOP before publication on any failed source, candidate, checksum or evidence gate; missing or
ambiguous evidence fails the gate. Resume only from authoritative probes (`git ls-remote`,
`gh run view`, `gh release view`) and the recorded checksums. Never repair source inline. Never
compensate a published effect automatically: probe each remote object and continue only when its
identity matches the recorded candidate.

**Version naming (my call).** Until a `v1.0.0` tag exists on origin, every re-run and re-cut is
still `v1.0.0` under this change: the workflow's `version` input, the asset names and
`docs/release/v1.0.0/` keep that name, and evidence from a discarded candidate is replaced, never
mixed. Once the tag exists, a fix ships as `v1.0.1` under a new change.

## Alternatives considered

- **Keep pinned install commands and bump them to `v1.0.0`.** Rejected: merged before the cut they
  404 until publication; merged at closeout (rc.2's way) the release notes would link an install
  page still showing rc.2. The version-free form has neither gap and ends per-release doc edits.
- **A changelog since rc.2 in the notes.** Rejected: nothing user-facing changed since rc.2; a stable
  release page needs an overview, not a diff.
- **Human harness test or public-install check before publishing.** Rejected: Claude Code and
  Cursor are tested enough; the downloader has not changed since rc.1 proved it.
- **Pin the Bash upgrade guide to `v1.0.0` at closeout** (so #514 freezes it on a tested version).
  Rejected by the human: nobody uses docket yet, so `latest` everywhere is simpler.
- **Revive #513/#514 inside this change.** Rejected: they are separate work the human picks up
  after the release.

## Out of scope

- Human-testing any harness before publication; the public-install check.
- Product source changes of any kind inside the freeze.
- Reviving or building #514 (Bash upgrade test retirement) or #513 (full OpenCode support); Codex.
- Homebrew, Windows, signing or notarization, SBOM or provenance; a publishing workflow.
- Docs beyond the Phase 0 install-command edits.

## Acceptance boundary

- **Designed:** this spec is linked from the record.
- **Implemented:** the Phase 0 docs PR is merged and in the candidate; the closeout PR is open with
  every phase's evidence in the bundle and every gate green; the `v1.0.0` tag and full release
  exist at the candidate with six verified assets; the release is "Latest" and the
  `releases/latest/download` links serve its files.
- **Done:** `docket-finalize-change` archives the change and the sweep is clean.
