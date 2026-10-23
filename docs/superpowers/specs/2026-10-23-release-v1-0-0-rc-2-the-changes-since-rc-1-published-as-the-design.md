<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0579 — Release v1.0.0-rc.2: the changes since rc.1, published as the build to test](../../changes/active/0579-release-v1-0-0-rc-2-the-changes-since-rc-1-published-as-the.md)**
<!-- docket:backlink:end -->

# v1.0.0-rc.2 — release candidate publication

**Change:** 0579 · **Type:** chore · **Priority:** high · **Groomed:** 2026-10-23 · **Status:** Approved design

## Purpose and boundary

Take one commit of `main` to a verified, public `v1.0.0-rc.2` **full release** of the Go binary,
not a pre-release, which becomes the repository's "Latest" release in place of `v1.0.0-rc.1`.

rc.2 exists so the human can test what has landed since rc.1. They will install it from the GitHub
release on their own machines and run real work through Claude Code and Cursor. That use is the
test. Nothing is human-tested before publication, and there is no separate public-install check:
the human's own install from the release is that check.

This is rc.1's protocol (change 0544, spec
`2026-10-08-release-v1-0-0-rc-1-claude-code-and-cursor-tested-opencode-s-design.md`) without its
public-install check. The steps are written out below, so that spec is background only.

This change builds no product code. Its only source edits are the install-doc version strings (see
*Docs*), made on its own feature branch and merged at closeout. If a source defect turns up, it
goes to a separate reviewed change, the candidate is discarded, and packaging restarts from the new
merged commit. The version stays `v1.0.0-rc.2` until a `v1.0.0-rc.2` tag exists (see *Failure and
retry boundary*).

**Run by hand, not by `docket-implement-next`.** The operator runs this protocol in an attended
session with the human present, because Phase 4 needs the human's explicit "publish". The change
is claimed for its branch and closed out through typed operations (Phases 1 and 6), never planned
or built by an autonomous run. If `docket-implement-next` is dispatched on it anyway, it stops
before claiming, as it did for rc.1, and the session carries on by hand. The change is not on hold:
the human starts it right after grooming.

## Decisions (human, 2026-10-23)

| Topic | Decision |
|---|---|
| Testing before publication | **None.** No harness test, no public-install check. rc.2 is the build the human tests with, by installing it from the release. The full test suite gates the candidate. |
| Public-install check | Dropped. The human's install from the release is the same check. The downloader (`internal/release/downloader/install.sh`) is unchanged since rc.1, so rc.1's proof (no quarantine attribute, valid ad-hoc signature) still holds. |
| Release type | Full release, not a pre-release; becomes "Latest", replacing `v1.0.0-rc.1`. |
| Harness status | Unchanged from rc.1: Claude Code and Cursor **supported**; OpenCode **shipped but untested**; Codex **unsupported**. |
| Cursor model pins | The notes state that Cursor runs docket's subagents at their pinned models (change 0545 was closed after a manual test confirmed it). rc.1's pin gap is dropped, and the notes say nothing about effort pins. |
| Upgrade notes | Include the per-repository refresh step for shared repositories (see *Release notes*). |
| Known gaps | rc.1's list, minus the Cursor pin gap. No line saying the changes since rc.1 are untested by hand. |
| Hold | Not set. |
| Source changes before the cut | Allowed. rc.2 ships from `origin/main` at the cut; anything merged before then ships and is listed. |
| Rollback | No rehearsal. Earlier releases stay available on GitHub. |

## Current reality (2026-10-23)

| Fact | Value |
|---|---|
| Latest tags | `v1.0.0-rc.1` (full release, "Latest", 2026-10-08, commit `d0f4712a5`), `v1.0.0-alpha.2`, `v1.0.0-alpha.1` (pre-releases), `v0.9.3`, `v0.9.2` (Bash). |
| Since rc.1 | 267 commits on `main`, from about 45 changes; 485 files changed outside `docs/`. |
| Backlog | No change `in-progress` or `implemented`; no open PR. |
| Release tooling | `.github/workflows/release-candidate.yml` changed only in comments since rc.1; `internal/release/` is unchanged. |
| Install docs | `README.md` (*Install and the five steps*) and `docs/install/install.md` (*From a release*) each name `v1.0.0-rc.1` twice: the download base URL and the `--version` flag. |
| Repository dispatch instructions | The run-tracker rule gained a `run-next` paragraph after rc.1 (change 0570). A shared repository keeps that text in its committed `AGENTS.md`, `CLAUDE.md` or `.cursor/rules/` file, and `docket install` refreshes only the repository it runs in, or the one `--repo-dir` names. A private repository's instructions come from the binary at session start. |
| Bundle | `docket_v1.0.0-rc.2_{darwin,linux}_{amd64,arm64}.tar.gz`, `checksums.txt`, `install.sh`. |

## Docs

On the feature branch, before Phase 6's gate: replace `v1.0.0-rc.1` with `v1.0.0-rc.2` in the
install commands of `README.md` and `docs/install/install.md`. First grep the maintained docs for
`v1.0.0-rc.1` to catch any other install command. Leave `docs/release/`, results files and other
point-in-time records alone. Nothing else in the docs changes. The Phase 6 suite gate covers any
doc-guard test these edits touch.

## Protocol

Every docket operation is resolved from the capability catalog (`docket capabilities --json`);
request bodies come from `docket schema --operation <id> --json`.

### Phase 0 — Before the cut

Record each item in `decisions.md`: the decision, who made it, when, and the merge SHA or "held".

- No change other than 0579 is `in-progress` once the window opens; every open PR is merged or
  recorded as held until after Phase 6.
- The human attests that no `/loop` or autonomous session is running against the Docket
  repository.

### Phase 1 — Cut the candidate and open the window

1. `git fetch origin`. The candidate SHA is `git rev-parse origin/main`. Record
   `git log -1 --format='%H %ct %s' <sha>` and `git ls-remote origin refs/heads/main`.
2. Record verbatim in `decisions.md`: `docket status --json` (no `in-progress` change, zero error
   findings) and `gh pr list --state open`.
3. Record the source rc.2 adds over rc.1: `git log --oneline v1.0.0-rc.1..<candidate>` and
   `git diff --stat v1.0.0-rc.1 <candidate> -- . ':!docs'` (summary line and file count are
   enough if the stat is long).
4. Claim 0579 (`change.claim`, a supported attended claim with no run context), then
   `workspace.prepare`, then reconcile the record (`change.reconcile`) against the candidate, then
   attach a pointer plan through `change.attach-plan`: a short plan file that lists this spec's
   phases in order. Phase 6's `change.mark-implemented` refuses without the reconcile and the plan.
5. Commit and push each phase's evidence to the feature branch as it lands; the branch is the
   durable checkpoint.
6. **Freeze rule.** Every later phase re-probes `git ls-remote origin refs/heads/main` and requires
   the candidate SHA. A mismatch is a STOP. The human either re-cuts from the new tip (restart at
   Phase 2 with a fresh workflow run; the old candidate is discarded whole) or records each
   intervening commit as excluded from rc.2. The tag always targets the recorded candidate.

### Phase 2 — Package once

1. Dispatch the workflow:
   `gh workflow run release-candidate.yml --repo danielhanold/docket -f ref=<candidate SHA> -f version=v1.0.0-rc.2`.
   Capture the run id, require its `headSha` to equal the candidate, and watch it.
2. Require conclusion `success` with every job green.
3. Download `candidate-head` and `candidate-evidence` into a fresh directory outside the
   repository. Verify:
   - `evidence.json` has `source_commit` equal to the candidate, `head_version` equal to
     `v1.0.0-rc.2`, and `checksums_txt` byte-equal to the bundle's `checksums.txt`, trailing
     newline included;
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
workflow once at the same candidate, still as `v1.0.0-rc.2`; that run is a new candidate under
step 5. A second failure of the same kind, or any source defect, is a STOP for the human: a source
defect goes to its own change, and packaging restarts from the new tip of `main`, still as rc.2 and
still under 0579.

### Phase 3 — Coverage statement (no test)

`decisions.md` records: no harness test and no public-install check, by the human's decision of
2026-10-23; rc.2 is the build the human tests with; the candidate is gated by the full suite in
Phase 2; the downloader is unchanged since rc.1, whose public-install check proved no quarantine
attribute and a valid signature.

### Phase 4 — Publish (the human's irreversible step)

**Draft the notes first** (outline below) into `release-notes.md` on the feature branch. Before
showing them to the human, confirm each upgrade-note claim against the candidate: the reclaim
keys' warning text, the absence of `change.reclaim` from the capability catalog, the readiness a
specless `trivial: true` change gets, that `docket install --repo-dir <repo>` refreshes a shared
repository's dispatch instructions, and the install command. Correct any claim that does not hold
and point the correction out to the human.

**Preconditions:**

- Phases 1–3 recorded and passing;
- `origin/main` re-probed and still the candidate;
- the notes reviewed and approved by the human;
- the human's explicit "publish" recorded in `decisions.md`.

Each step is probe → act only if absent → verify → record, in `publication.md`:

1. **Tag.** Probe `git ls-remote --tags origin refs/tags/v1.0.0-rc.2`.
   - Absent: `git tag -a v1.0.0-rc.2 <candidate SHA> -m "docket v1.0.0-rc.2"`, push the tag, and
     verify the peeled target is the candidate.
   - Present at the candidate: continue.
   - Present elsewhere: STOP. Never move or delete a tag; a re-cut ships as `v1.0.0-rc.3`.
2. **Release.** Absent:
   `gh release create v1.0.0-rc.2 --verify-tag --draft --title "v1.0.0-rc.2 — release candidate 2" --notes-file release-notes.md`.
   Draft present: continue. Published: skip to step 4.
3. **Assets.** For each of the six files in the read-only copy: absent → upload it (never
   `--clobber`); present → download it and compare its SHA-256. A mismatch is a STOP; the human may
   delete the offending draft asset by hand and re-run this step.
4. **Verify.** Exactly six assets with the expected names, and sizes and digests equal to the
   read-only copy.
5. **Publish.** `gh release edit v1.0.0-rc.2 --draft=false --prerelease=false --latest`. Verify the
   release is published, not a pre-release, and "Latest", and that `v1.0.0-rc.1` no longer is.
   Re-probe the tag.

Once published, the notes on GitHub are the human's: they may edit them by hand. Never overwrite
them afterwards. `release-notes.md` keeps the text as published at this step.

### Phase 5 — (none)

No public-install check (see *Decisions*).

### Phase 6 — Closeout

The order matters: the final head is gated and published before the change is marked
implemented, or `pr.publish` returns `contended` and the PR body keeps stale evidence.

1. Make the *Docs* edits and assemble the evidence bundle on the feature branch. Write the results
   record; its human-actions section is the gate table, one line per phase.
2. Drive the build gate (`build.test_command`) on the feature worktree through the gate driver to
   `PASSED`.
3. Record evidence, attach results, and publish the PR. Its body carries the gate table.
4. Mark the change implemented.
5. Run `docket-finalize-change 579`. The window closes at its merge.
6. Rebuild the installed binary from source as the repository's standing post-merge rule
   (`AGENTS.md`, *Rebuild the binary after a merge to main*) requires.
7. Run `docket-status`; it should come back clean. Held PRs may merge now.

**The human's own rc.2 install comes after closeout (my call).** The release downloader and a
source install write the same `~/.local/bin/docket` and share one install record. So on the
operator's machine, the human installs rc.2 from the release only after step 6. Otherwise the
post-merge rebuild would replace the release install with a source build of `main`. That install
starts the human's testing and is outside this change.

## Evidence bundle

```text
docs/release/v1.0.0-rc.2/
  README.md               index: candidate identity, gate table (phase → verdict → file)
  decisions.md            Phase 0 decisions, the source added since rc.1, coverage statement, every STOP
  candidate/evidence.json verbatim workflow artifact
  candidate/checksums.txt verbatim
  candidate/run.txt       run id/URL, toolchain, suite summary, Bash upgrade shard result, copy listing
  publication.md          tag object and peeled SHA, release id/URL, asset ids and digests
  release-notes.md        the notes as published
```

Sanitization: no tokens or credentials; home paths become `$HOME`. Once the closeout PR merges,
`results:` and the bundle are frozen build records.

## Release notes outline

Title: `v1.0.0-rc.2 — release candidate 2`. Sections, in order:

1. **What this release is.** The second release candidate for `v1.0.0`, published as a full
   release; it replaces `v1.0.0-rc.1` as "Latest". It carries everything merged since rc.1.
2. **Harness status.**
   - Claude Code and Cursor: supported. Cursor runs docket's subagents at their pinned models.
   - OpenCode: installs and ships, **untested**; full support comes after `v1.0.0`.
   - Codex: **unsupported**.
3. **Install.** As rc.1, with `VERSION=v1.0.0-rc.2`:
   ```sh
   VERSION=v1.0.0-rc.2
   curl -fsSLO "https://github.com/danielhanold/docket/releases/download/$VERSION/install.sh"
   curl -fsSLO "https://github.com/danielhanold/docket/releases/download/$VERSION/checksums.txt"
   grep '  install.sh$' checksums.txt | shasum -a 256 -c -
   sh install.sh --version "$VERSION" --harness claude --harness cursor
   ```
   Then: the `shasum` line must print `install.sh: OK`; drop the `--harness` you don't use; the
   binary goes to `~/.local/bin`; the four platforms (macOS and Linux, amd64 and arm64); the
   runtime dependencies (`sh`, `curl`, `tar`, `shasum` or `sha256sum`); restart Claude Code and
   Cursor after installing; for Cursor, run docket outside the sandbox (link
   `docs/install/cursor.md`); install with the script, not a browser download (see Known gaps).
4. **Upgrading.**
   - **From rc.1 or an alpha:**
     1. Let any docket run that is still going finish, or stop it with `docket run cancel`, before
        upgrading. The run tracker changed since rc.1, and a run that spans the upgrade would mix
        old and new instructions.
     2. Re-run the install commands above, then restart Claude Code and Cursor.
     3. In each **shared** repository that uses docket, run `docket install --repo-dir <repo>`,
        then commit what it changed (`AGENTS.md`, `CLAUDE.md` or `.cursor/rules/`). The
        dispatch instructions gained a rule for loops that carry on past a halt. Private
        repositories need nothing: their instructions come from the binary.
     4. What you will notice:
        - `reclaim.lease_ttl` and `reclaim.auto` in a config file now produce a warning and are
          ignored; delete them.
        - `docket change reclaim` is gone; resume a stranded change by id.
        - An unbuilt change marked `trivial: true` with no spec now reads as needs-grooming;
          groom it to give it a spec.
        - In Claude Code, a typed `/docket-implement-next` starts an untracked run; start a
          tracked run with a plain request or `/loop`.
   - **From Bash `v0.9.x`:** follow [Upgrading from Bash docket](https://github.com/danielhanold/docket/blob/main/docs/release/upgrading-from-bash.md),
     which covers Claude Code and Cursor and has no OpenCode section yet. The Bash tags `v0.9.2`
     and `v0.9.3` remain available.
5. **What changed since rc.1.** Built at the cut from `git log v1.0.0-rc.1..<candidate>` and the
   archived change records merged in that range. Group by theme, each theme naming its change
   numbers. User-facing themes first; the ones known today are private repositories, `docket open`
   and export, holding a change out of loops, loops carrying on past a halt, closing a change
   without a PR, the Claude Code allow rules for finalize, simpler grooming (no more `trivial`),
   and the removal of claim reclaim. Fixes and internal work follow, condensed. A change merged
   after today but before the cut gets its theme too.
6. **Known gaps.**
   - Cursor's sandboxed mode ("Allowlist (with Sandbox)") was not exercised.
   - Quitting Cursor in the middle of a docket run can leave a `cursor-agent` worker running;
     check for one before resuming.
   - OpenCode is untested; full support comes after `v1.0.0`. Codex is unsupported.
   - The binaries are not notarized: an archive downloaded in a browser and extracted in Finder is
     blocked by macOS Gatekeeper. Use the install script, or run
     `xattr -d com.apple.quarantine docket`.
   - No Homebrew or Windows.
   - Any gap the run records with the human's acceptance.
7. **Evidence.** A link to `docs/release/v1.0.0-rc.2/` on `main`, and the ADRs accepted since rc.1
   (from the ADR index on the metadata branch).

## Failure and retry boundary

STOP before publication on any failed source, candidate, checksum or evidence gate; missing or
ambiguous evidence fails the gate. Resume only from authoritative probes (`git ls-remote`,
`gh run view`, `gh release view`) and the recorded checksums. Never repair source inline. Never
compensate a published effect automatically: probe each remote object and continue only when its
identity matches the recorded candidate.

**Version naming (my call).** The name moves to `v1.0.0-rc.3` only once a `v1.0.0-rc.2` tag exists
on origin. Before that, every re-run and every re-cut is still `v1.0.0-rc.2` under change 0579: the
workflow's `version` input, the asset names, `docs/release/v1.0.0-rc.2/` and the doc edits all keep
that name, and evidence from a discarded candidate is replaced, never mixed. Once the tag exists,
a re-cut is a new change for `v1.0.0-rc.3`.

## Alternatives considered

- **Keep rc.1's public-install check.** Rejected: the human's own install from the release is the
  same check, and the downloader has not changed since rc.1 proved it.
- **Human harness test before publishing.** Rejected: rc.2 is the build for that testing.
- **Publish as a pre-release, rc.1 staying Latest.** Rejected: rc.2 is an actual release, as rc.1
  was.
- **Put the change on hold.** Not needed: the human builds it right after grooming, with no loop
  running.

## Out of scope

- Human-testing any harness before publication; the public-install check.
- Product source changes of any kind inside the freeze.
- Full OpenCode support and its upgrade-guide section (0513); Codex.
- Homebrew, Windows, signing or notarization, SBOM or provenance; a publishing workflow.
- Stable `v1.0.0`.
- Docs beyond the install-doc version strings.

## Acceptance boundary

- **Designed:** this spec is linked from the record.
- **Implemented:** the closeout PR is open with every phase's evidence in the bundle, the doc
  edits, and every gate green; the tag and full release exist at the candidate with six verified
  assets, and the release is "Latest".
- **Done:** `docket-finalize-change` archives the change and the sweep is clean.
