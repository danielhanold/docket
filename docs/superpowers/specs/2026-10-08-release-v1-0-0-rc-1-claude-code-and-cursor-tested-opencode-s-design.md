<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0544 — Release v1.0.0-rc.1: Claude Code and Cursor tested, OpenCode shipped untested](../../changes/archive/2026-10-08-0544-release-v1-0-0-rc-1-claude-code-and-cursor-tested-opencode-s.md)**
<!-- docket:backlink:end -->

# v1.0.0-rc.1 — release candidate publication

**Change:** 0544 · **Type:** chore · **Priority:** high · **Groomed:** 2026-10-08 · **Status:** Approved design

## Purpose and boundary

Take one reviewed commit of `main` to a verified, public `v1.0.0-rc.1` **full release** of the
Go binary — not a pre-release — which becomes the repository's "Latest" release. The notes call Claude Code and Cursor tested, OpenCode shipped but untested, and Codex
unsupported.

This is the alpha.2 protocol (change 0512, spec
`2026-10-08-release-v1-0-0-alpha-2-prove-and-publish-cursor-support-design.md`) **without a human
harness test**. Where this spec says "as 0512", that spec's wording applies with the version,
title and file names substituted.

This change builds no product code. Its only source edits are the install docs (see *Docs*
below), made on its own feature branch and merged at closeout. If a source defect turns up, it goes to a separate reviewed
change, the candidate is discarded, and packaging restarts from the new merged commit.

**Not driven by `docket-implement-next`.** The operator runs this protocol by hand in an attended
session. The change is claimed for its branch and closed out through typed operations (Phases 1
and 6), never planned or built by an autonomous run.

## Decisions (human, 2026-10-08)

| Topic | Decision |
|---|---|
| Human harness test | **None.** Claude Code was tested end to end in alpha.1 (0366) and Cursor in alpha.2 (0512). rc.1 is gated by the candidate's whole-suite source gate only, for both harnesses. |
| Source changes before the cut | Allowed. rc.1 ships from `origin/main` at the cut, gated by the suite only. The notes list what changed since alpha.2. A code change does not trigger a human retest. |
| OpenCode | Files ship and install as today; the notes call it **untested**, not unsupported. Full support is 0513, after `v1.0.0`. |
| Codex | The only officially **unsupported** harness. |
| Signing / notarization | Not needed for the supported install path, and not done. See *macOS Gatekeeper* below. |
| Release type | rc.1 is a **full release, not a pre-release**, and becomes "Latest", replacing `v0.9.3`. Nothing installs from "Latest" automatically: the downloader is version-stamped and no doc links `releases/latest`. |
| Docs | Edited on 0544's own feature branch and merged at closeout (see *Docs*). The tagged commit keeps the old docs; `main` gets the new ones when the closeout PR merges. |
| Rollback | As 0512: no rehearsal; the notes state that the Bash tags `v0.9.2` and `v0.9.3` remain available. |

## Current reality (2026-10-08)

| Fact | Value |
|---|---|
| Latest tags | `v1.0.0-alpha.2` (pre-release, 2026-10-08, commit `ec4c2b1`), `v1.0.0-alpha.1` (pre-release), `v0.9.3` ("Latest"). |
| Since alpha.2 | 11 commits on `main`, all under `docs/release/`; no source change. Cut today, rc.1's binary would differ from alpha.2's only in its embedded version identity. |
| Install path | The published downloader (`internal/release/downloader/install.sh`) fetches with `curl` and unpacks with `tar`, then installs. |
| darwin binaries | Cross-built with `CGO_ENABLED=0`; the Go linker ad-hoc signs darwin/arm64 output (`Signature=adhoc`, `linker-signed`). |
| Open Cursor follow-up | 0545 (whether Cursor runs docket subagents at their pinned models) — a known gap carried from alpha.2, not a precondition. |
| Docs today | `README.md` and `docs/install/install.md` describe only the clone-and-`bash install.sh` path and call Codex supported; `docs/install/keeping-current.md` suggests `git checkout v0.9.3`. None mentions the release downloader. |
| Bundle | `docket_v1.0.0-rc.1_{darwin,linux}_{amd64,arm64}.tar.gz`, `checksums.txt`, `install.sh`. |

## macOS Gatekeeper

Gatekeeper assesses only files carrying the `com.apple.quarantine` extended attribute, which
browsers, Mail and AirDrop add to downloads; `curl` and `tar` do not. The supported install path
therefore never triggers Gatekeeper, and Apple Silicon's must-be-signed rule is met by the
linker's ad-hoc signature. A user who downloads an archive in a browser and extracts it in Finder
gets a quarantined, un-notarized binary that macOS refuses to run; the remedy is the install
script (or `xattr -d com.apple.quarantine docket`). Phase 5 records proof of the first claim, and
the notes state the second as a known gap. Notarization (Apple Developer Program membership,
Developer ID signing in CI) is out of scope.

## Docs

On the feature branch, before Phase 6's gate, bring the install docs in line with rc.1:

- `README.md` (*Install and the five steps*) and `docs/install/install.md`: lead with the release
  downloader (download `install.sh` and `checksums.txt` from the release, verify, then
  `sh install.sh --version v1.0.0-rc.1 --harness <name>`); keep the clone-and-build path as the
  way to run from source.
- Harness status in `docs/install/install.md` (and each harness page's opening line where it
  claims support): Claude Code and Cursor supported and tested; OpenCode installs but is
  untested; Codex unsupported.
- `docs/install/keeping-current.md`: updating a release install means re-running the downloader
  for the new version; drop the `v0.9.3` example.
- One line in the install docs: install with the script, not a browser download, because the
  binaries are not notarized (see *macOS Gatekeeper*).

Docs describe current behavior only — no change numbers. The whole-suite gate in Phase 6 covers
any doc-guard tests these edits touch.

## Protocol

### Phase 0 — Before the cut

Record each item in `decisions.md`: the decision, who made it, when, and the merge SHA or "held".

- No change other than 0544 is `in-progress` once the window opens; every open PR is merged or
  recorded as held until after Phase 6.
- The operator attests that no `/loop` or autonomous session is running against the Docket
  repository.

No harness dry run.

### Phase 1 — Cut the candidate and open the window

As 0512 (candidate SHA from `origin/main`, the verbatim `docket status --json` and
`gh pr list --state open`, the claim, `workspace.prepare`, reconcile, pointer plan, the freeze
rule). Also record `git log --oneline v1.0.0-alpha.2..<candidate>` and
`git diff --stat v1.0.0-alpha.2 <candidate> -- . ':!docs'` in `decisions.md`: the source that
rc.1 adds over the last human-tested build.

### Phase 2 — Package once

As 0512, with version `v1.0.0-rc.1`. `evidence.json` must match exactly, including
`checksums_txt` byte-equal to the bundle's `checksums.txt`; a difference is a STOP with no
waiver. Confirm from the source-gate log that the `TestBashUpgrade` cases, including the Cursor
assertions, ran and passed.

### Phase 3 — (none)

No human harness test. `decisions.md` records the coverage statement: Claude Code proven in
alpha.1, Cursor in alpha.2, both covered at this candidate by the whole-suite source gate.

### Phase 4 — Publish (the human's irreversible step)

As 0512, with tag and release `v1.0.0-rc.1` and title `v1.0.0-rc.1 — release candidate`, except
that the release is **not** a pre-release.
Preconditions: Phases 1–2 recorded and passing, `origin/main` still the candidate, notes reviewed,
the human's explicit "publish" in `decisions.md`. Probe → act only if absent → verify → record
for the tag, the draft release, the six assets (never `--clobber`), and
`gh release edit v1.0.0-rc.1 --draft=false --prerelease=false --latest`. Verify that `v1.0.0-rc.1`
is now "Latest" and is not marked pre-release. Never move or delete a tag; a re-cut ships as `v1.0.0-rc.2`.

### Phase 5 — Public install check

As 0512, in a fresh test home with no Docket and no `DOCKET_RELEASE_BASE_URL`: download
`install.sh` and `checksums.txt` from the `v1.0.0-rc.1` release URL, verify the script's SHA-256,
run `sh install.sh --version v1.0.0-rc.1 --harness claude --harness cursor --bin-dir <test home>/bin`,
and verify `docket version --json`, `docket install check --json` (clean, harnesses
`[claude, cursor]`) and `docket diagnostic runtime --json`. On macOS also record
`xattr <test home>/bin/docket` (no `com.apple.quarantine`) and `codesign -v <test home>/bin/docket`
(valid). Record in `public-install.md`.

### Phase 6 — Closeout

As 0512, in the same order, with `docket-finalize-change 544`. The doc edits (*Docs*) are
committed on the feature branch before the build gate runs, so the gate covers them; they reach
`main` when finalize merges the PR.

## Evidence bundle

```text
docs/release/v1.0.0-rc.1/
  README.md               index: candidate identity, gate table (phase → verdict → file)
  decisions.md            Phase 0 decisions, the source diff since alpha.2, coverage statement, every STOP
  candidate/evidence.json verbatim workflow artifact
  candidate/checksums.txt verbatim
  candidate/run.txt       run id/URL, toolchain, suite summary, TestBashUpgrade result, copy listing
  publication.md          tag object and peeled SHA, release id/URL, asset ids and digests
  public-install.md       URLs, digests, command outputs, quarantine and signature probes
  release-notes.md        the notes as published
```

Sanitization as 0512. Once the closeout PR merges, `results:` and the bundle are frozen build
records.

## Release notes outline

Title: `v1.0.0-rc.1 — release candidate`. Sections, in order:

1. **What this release is.** The first release candidate for `v1.0.0`, published as a full
   release: the first Go build to be "Latest", replacing the Bash `v0.9.3`.
2. **Harness status.** Claude Code: tested end to end (alpha.1). Cursor: tested end to end
   (alpha.2). Both are covered at this build by the full test suite; neither was re-run by hand.
   OpenCode: installs and ships, **untested** (0513, after `v1.0.0`). Codex: **unsupported**.
3. **Install.** The downloader command with `--harness claude` and `--harness cursor`; the four
   platforms; verifying `install.sh` against `checksums.txt`; runtime dependencies (`sh`, `curl`,
   `tar`, one SHA-256 tool); restart the harness after installing; for Cursor, run outside the
   sandbox (link `docs/install/cursor.md`). Install with the script, not a browser download (see
   Known gaps).
4. **Upgrading.** From v0.9.x: link `docs/release/upgrading-from-bash.md` (Claude Code and
   Cursor); the guide has no OpenCode section yet; the Bash tags `v0.9.2` and `v0.9.3` remain
   available. From an alpha: re-run the installer for `v1.0.0-rc.1`.
5. **What changed since alpha.2.** Themes with their changes, from
   `git log v1.0.0-alpha.2..<candidate>`; "documentation only" if that is the truth.
6. **Known gaps.** Each names its tracking change where one exists:
   - the Cursor Allowlist (with Sandbox) setup was not exercised;
   - whether Cursor runs docket subagents at their pinned models is unconfirmed (0545);
   - OpenCode untested (0513); Codex unsupported;
   - the binaries are not notarized: a browser-downloaded archive is blocked by macOS
     Gatekeeper — use the install script;
   - no Homebrew or Windows;
   - anything the run records as a human-accepted gap.
7. **Evidence.** A link to `docs/release/v1.0.0-rc.1/` on `main` and the ADRs alpha.2 listed.

## Failure and retry boundary

As 0512: STOP before publication on any failed source, candidate, checksum or evidence gate
(missing or ambiguous evidence fails the gate); resume only from authoritative probes
(`git ls-remote`, `gh run view`, `gh release view`) and the recorded checksums; never repair
source inline; never automatically compensate a published effect.

## Alternatives considered

- **Re-run the full lifecycle on Claude Code and Cursor.** Rejected by the human: both were just
  proven, and the source gate covers the candidate.
- **Freeze rc.1 at alpha.2's source.** Rejected: fixes that land before the cut should ship; the
  notes name them instead.
- **A short manual smoke on each harness.** Rejected as ceremony; the public install check
  already proves the release installs for both.
- **Publish rc.1 as a pre-release, `v0.9.3` staying Latest.** Rejected by the human: rc.1 is an
  actual release.
- **Fix the docs in a separate change merged before the cut.** Rejected as more machinery than the
  gap warrants; the docs lag `main` only until the closeout PR merges.
- **Notarize the darwin binaries.** Deferred: the supported install path never meets Gatekeeper;
  it needs paid Apple membership and CI signing for a browser path docket does not document.

## Out of scope

- Human-testing any harness; OpenCode support and its upgrade-guide section (0513); Codex.
- Source changes of any kind inside the freeze.
- Homebrew, Windows, signing/notarization, SBOM or provenance; a publishing workflow.
- Stable `v1.0.0`.
- Docs beyond the install pages named in *Docs*.

## Acceptance boundary

- **Designed:** this spec is linked from the record.
- **Implemented:** the closeout PR is open with every phase's evidence in the bundle, the doc
  edits, and every gate green; the tag and full release exist at the candidate with six verified assets and the release is
  "Latest"; the public
  install check passed.
- **Done:** `docket-finalize-change` archives the change and the sweep is clean.
