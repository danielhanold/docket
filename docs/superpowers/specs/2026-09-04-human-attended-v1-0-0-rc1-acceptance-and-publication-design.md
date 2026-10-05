<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0366 — v1.0.0-alpha.1 acceptance and publication (Claude Code)](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-05-0366-human-attended-v1-0-0-rc1-acceptance-and-publication.md)**
<!-- docket:backlink:end -->

# v1.0.0-alpha.1 — acceptance and publication

**Change:** 0366 · **Type:** chore · **Priority:** critical · **Revised:** 2026-10-04 (supersedes the
2026-09-03 beta1 design) · **Status:** Approved design

## Purpose and boundary

Take one reviewed commit of `main` to a verified, public `v1.0.0-alpha.1` pre-release of the Go
binary, with a human present at every external-truth and irreversible step.

This change builds no product code. If a source defect turns up anywhere in the protocol, it goes
to a separate reviewed change, the candidate is discarded, and packaging restarts from the new
merged commit.

## Decisions (human, 2026-10-04)

| Topic | Decision |
|---|---|
| Name | **`v1.0.0-alpha.1`**. The dotted pre-release identifier keeps later builds sorted (`alpha.2` < `alpha.10`). This replaces `v1.0.0-beta1`. The path to stable runs alphas, then betas, then `v1.0.0`. |
| Harnesses | **Claude Code is the only human-tested harness.** Cursor follows in alpha.2 (0512) and OpenCode in alpha.3 (0513). Codex is paused (0433 deferred). |
| Dropped from the 2026-09-03 design | The fresh macOS user account; the `v0.9.2` rollback rehearsal and cross-compatibility probe; recorded four-platform smoke evidence and the operator-machine smoke run; the separate Bash-install upgrade probes; the backlog/migration-ledger audit and its migration learnings. |
| Upgrade proof | Change 0511's guide and its `TestBashUpgrade` cases on the saved v0.9.2/v0.9.3 Bash state. They run inside the candidate's whole-suite source gate. The public-install check proves the guide's download step. |
| 0412 (build-agent stall on long test runs) | **Known gap.** Listed in the release notes with its recovery steps; not a blocker. |
| Not blockers | 0345, 0360, 0409, 0507, 0510. 0510 must still be finished or held before the cut (see Phase 0). |
| Rollback | No rehearsal and no rollback promise. The notes state only the fact that the Bash tags `v0.9.2` and `v0.9.3` remain available. |

## Current reality (2026-10-04)

| Fact | Value |
|---|---|
| Latest tag | `v0.9.3` (Bash era, no assets). `v0.9.2` is the frozen Bash baseline the installer can take over (ADR-0096). `main` is about 1,850 commits past `v0.9.3`. |
| Candidate workflow | `.github/workflows/release-candidate.yml`, with `permissions: contents: read` and `workflow_dispatch` inputs `ref` and `version`. Jobs: `source-gate` (macos-15, the whole suite through `docket development test`) → `package` (once) → `smoke` (four native runners) → `summary`. Artifacts: `candidate-head` (the bundle) and `candidate-evidence` (`evidence.json`). Its pull-request runs are green as of 2026-10-04. |
| Version grammar | `internal/release/version.go`, mirrored in the workflow. `v1.0.0-alpha.1` matches. |
| Bundle (six files) | `docket_v1.0.0-alpha.1_{darwin,linux}_{amd64,arm64}.tar.gz`, `checksums.txt`, and `install.sh` (the rendered downloader: flags `--version`, `--bin-dir`, `--harness`; base URL `https://github.com/danielhanold/docket/releases/download/<version>`). |
| Publishing automation | None. No workflow carries a write token; publication is human-typed `gh`. |
| Operator machine | darwin/arm64 with a **development** install. It must never leak into the Claude Code test or the public-install check. |
| Run tracker verbs | `docket run start implement-next` prints `run-started <key> <run-context>`; `docket run verdict <key>`; `docket run start implement-next --resume <id>`; `docket run cancel --key <key> --reason <why>`. Argv is always resolved from `docket capabilities --json`. |

## Roles and vocabulary

- **Operator.** The human plus the attended agent session on the operator machine. The operator
  is the only actor: no autonomous Docket loop runs against the Docket repository during the
  window.
- **Candidate.** The commit SHA, the workflow run id, and the bundle's `checksums.txt`, plus one
  read-only downloaded copy of the bundle that every later phase reads.
- **Window.** From recording the candidate commit (Phase 1) to the closeout merge (Phase 6). Inside
  it, nothing merges into `main` and no autonomous run touches the Docket repository.
- **STOP.** Halt before the next effect, write the gate and the reason to `decisions.md`, and wait
  for the human. Resumption always re-probes Git and GitHub and the recorded checksums. Local files,
  elapsed time, or a previously typed command never establish success.
- **Test home.** A throwaway folder on the operator's own account, for example
  `~/docket-alpha-test/`, used as `HOME` for a shell and every process started from it. Claude Code
  reads its skills and agents from `$HOME/.claude`, so pointing `HOME` here keeps the operator's real
  `~/.claude` and development install out of the test. No new macOS user is involved.

## Protocol

### Phase 0 — Before the cut

Record each item in `decisions.md`: the decision, who made it, when, and the merge SHA or "held".

- **0502** (skills and agent files aligned with the binary) is merged. The skills ship inside the
  binary.
- **0511** (Bash upgrade guide and its proof test) is merged.
- **0510** is finished (merged) or recorded as held. No change other than 0366 may be
  `in-progress` once the window opens.
- Every open PR is merged before the cut or recorded as held until after Phase 6.
- **0412** is recorded as a known gap for the notes.
- The operator attests that no `/loop` or autonomous session is running against the Docket
  repository.

### Phase 1 — Cut the candidate and open the window

1. `git fetch origin`. The candidate SHA is `git rev-parse origin/main`. Record
   `git log -1 --format='%H %ct %s' <sha>`.
2. Record verbatim: `docket status --json` (no `in-progress` change except 0366 once claimed, and
   zero error findings), `gh pr list --state open`, and the operator's no-loops attestation.
3. Claim 0366 (`change.claim`, then `workspace.prepare`, both resolved from the catalog) so a branch
   exists for evidence checkpoints. Refresh the lease at each phase boundary. Commit and push each
   phase's evidence to the feature branch as it lands; the branch is the durable checkpoint.
4. **Freeze rule.** Every later phase re-probes `git ls-remote origin refs/heads/main` and requires
   it to equal the candidate SHA. A mismatch is a STOP. The human either re-cuts from the new tip
   (restart at Phase 2 with a fresh workflow run; the old candidate is discarded whole) or records
   each intervening commit as excluded from alpha.1. The tag always targets the recorded candidate.

### Phase 2 — Package once

1. Dispatch the workflow:
   `gh workflow run release-candidate.yml --repo danielhanold/docket -f ref=<candidate SHA> -f version=v1.0.0-alpha.1`.
   Capture the run id, require its `headSha` to equal the candidate, and watch it.
2. Require conclusion `success` with every job green. The `smoke` job runs as part of the workflow;
   its success is part of "all green" and is not recorded separately.
3. Download `candidate-head` and `candidate-evidence` into a fresh directory. Then verify:
   - `evidence.json` has `source_commit` equal to the candidate, `head_version` equal to
     `v1.0.0-alpha.1`, and `checksums_txt` byte-equal to the bundle's `checksums.txt`;
   - recomputing every bundle file's SHA-256 matches a manifest line, with no unmatched line.
4. Make the copy read-only (`chmod -R a-w`). Record in `candidate/run.txt`:
   - its path and SHA-256 listing;
   - the run URL;
   - the source gate's `go version`;
   - the suite summary lines, including any budget-report lines.

   Confirm from the source-gate log that the `TestBashUpgrade` cases ran and passed. That is the
   upgrade proof for this candidate.
5. Every later phase reads **this copy**. A second workflow run, for any reason, makes a new
   candidate: the old copy is discarded and Phases 3–5 restart. Bundles are never mixed.

STOP on: any job not green; any evidence mismatch; any checksum mismatch; `TestBashUpgrade` absent
or skipped.

### Phase 3 — The Claude Code test (human-attended)

**Test home setup.**

1. Create the test home, then open a shell with `HOME` set to it and every `XDG_*` variable pointing
   inside it.
2. Install the candidate from the read-only copy:
   `sh <copy>/install.sh --harness claude --bin-dir <test home>/bin`, then put `<test home>/bin`
   first on `PATH`.
3. Verify:
   - `docket version --json` shows version `v1.0.0-alpha.1` and commit equal to the candidate;
   - `docket install check --json` is clean, with `mode: release` and the Claude Code harness only;
   - `docket diagnostic runtime --json` shows `supported_target: true`.
4. Log in to Claude Code and `gh` inside this shell. Copy no other operator state.

**Remote and fixture.**

1. Create one disposable private repository,
   `gh repo create danielhanold/docket-accept-v1-0-0-alpha-1-claude --private`, and clone it inside
   the test home.
2. Add `README.md` and an executable `test.sh` (`#!/bin/sh` / `exit 0`), then push.
3. Run `docket repository init`. Confirm `build.test_command` and `finalize.test_command` are both
   `sh ./test.sh` with both gates `local`; use `docket repository configure-tests` if init left the
   policy pending.
4. Commit and push. `docket repository check --json` must be clean.

**Lifecycle.** The human types every step in Claude Code started from the test-home shell:

1. Create one stub with the catalog-resolved `change.create` request (type `feat`, "Add a greeting
   line to README"). The stub is needs-grooming.
2. Groom it with `docket-groom-next <id>` and exit with a short spec.
3. Implement it: run `docket run start implement-next`, dispatch `docket-implement-next <id>` with
   the run context, then run `docket run verdict <key>`, as the fixture's dispatch instructions
   prescribe.
4. **Kill and resume.**
   - From a second test-home shell, poll `docket status --records --json` until the record is
     `in-progress` with `plan:` set.
   - Terminate the Claude Code process (SIGTERM to its PID), then start a fresh Claude Code
     process from the test-home shell.
   - Run `docket run start implement-next --resume <id>` and dispatch `docket-implement-next <id>`
     again.
   - The run must resume that id (never claim another change), reconcile the interrupted gate drive
     through the driver's own continuation or takeover path, and reach `implemented` with an open
     PR. Obey every `run-*` verdict line exactly.
5. Finalize with `docket-finalize-change <id>`: rebase gate, merge, closeout and cleanup.
6. Run `docket-status`, then check the terminal predicate.

**Terminal predicate.** All of these must hold:

- the record is under `archive/` with `status: done`, a full-URL `pr:`, and `plan:` and `results:`
  set;
- `gh pr view <pr> --json state` shows `MERGED`;
- the default branch carries the greeting line;
- `docket status --json` reports no ready changes and zero error findings;
- `docket repository check --json` is clean;
- the remote metadata branch holds the archived record and the re-rendered board.

**Record** `harness/claude.md` with:

- the Claude Code version and mode;
- the test-home path (written as `$TEST_HOME`);
- the candidate commit and archive SHA-256;
- proof that a fresh process started after install;
- proof the named children ran (`docket-plan-writer`, a `docket-build-*` tier and a
  `docket-review-*` tier);
- the kill and resume timestamps and the verdict lines;
- the terminal predicate results;
- the location of a sanitized transcript.

Archive or delete the disposable repository afterwards; the record keeps the PR URL.

**Gate.** The predicate must pass. A failure is a STOP:

- a Docket defect goes to a source change and a new candidate;
- a Claude Code vendor defect is diagnosed against the recorded version, and the human decides
  whether alpha.1 ships with it listed as a known gap.

### Phase 4 — Publish (the human's irreversible step)

**Preconditions:**

- Phases 1–3 are recorded and passing;
- `origin/main` is re-probed and still equals the candidate;
- the release notes (outline below) are authored and reviewed by the human;
- the human's explicit "publish" is recorded in `decisions.md`.

Each step is probe → act only if absent → verify → record, in `publication.md`:

1. **Tag.** Probe `git ls-remote --tags origin refs/tags/v1.0.0-alpha.1`.
   - Absent: run `git tag -a v1.0.0-alpha.1 <candidate SHA> -m "docket v1.0.0-alpha.1"` and push
     the tag; verify the peeled target equals the candidate.
   - Present at the candidate: continue.
   - Present elsewhere: STOP. Never move or delete a tag; a re-cut ships as `v1.0.0-alpha.2`.
2. **Release.**
   - Absent: run
     `gh release create v1.0.0-alpha.1 --verify-tag --draft --prerelease --title "v1.0.0-alpha.1 — Docket is a Go binary" --notes-file release-notes.md`.
   - Draft present: continue.
   - Published: skip to step 4.
3. **Assets.** For each of the six files in the read-only copy:
   - absent: upload it (never `--clobber`);
   - present: download it and compare its SHA-256. A mismatch is a STOP; the human may delete the
     offending draft asset by hand and re-run this step.
4. **Verify.** Exactly six assets, with the expected names, and sizes and digests equal to the
   read-only copy.
5. **Publish.** Run `gh release edit v1.0.0-alpha.1 --draft=false --prerelease --latest=false`.
   Verify it is published and still a pre-release, and that `v0.9.3` remains "Latest". Re-probe
   the tag.

**Partial publication.** Never compensate a published effect automatically. Probe each remote
object and continue only when its identity matches the recorded candidate. A tag or release that
points elsewhere is a STOP; the next attempt is a new pre-release number.

### Phase 5 — Public install check

1. In a second, fresh test home (no Docket present, no `DOCKET_RELEASE_BASE_URL`), download
   `https://github.com/danielhanold/docket/releases/download/v1.0.0-alpha.1/install.sh` and
   `checksums.txt`.
2. Verify the script's SHA-256 against its manifest line **before** running it. Never pipe a
   downloaded script into `sh`.
3. Run `sh install.sh --harness claude --bin-dir <test home>/bin`.
4. Verify `docket version --json`, `docket install check --json` (clean) and
   `docket diagnostic runtime --json`.

Record every URL, digest and output in `public-install.md`. This also proves the download and
checksum step of 0511's upgrade guide. A public-URL check verifies that the accepted bytes are
exposed; it never authorizes a rebuild.

### Phase 6 — Closeout

1. Assemble the evidence bundle (below) and the results record on the claimed feature branch. The
   results record's human-verify section is the gate table, one line per phase.
2. Drive the build gate on the feature worktree through the gate driver to `PASSED`. Then record
   evidence, attach results, publish the PR (its body carries the gate table), and mark the change
   implemented. Every operation is resolved from the catalog.
3. Run `docket-finalize-change 366`. The window closes at its merge.
4. Run `docket-status`; it should come back clean. Held PRs may merge now.

Do not use `docket-implement-next` for 0366: there is no code to plan or build, and the evidence
exists before the PR opens.

## Evidence bundle

```text
docs/release/v1.0.0-alpha.1/
  README.md               index: candidate identity, gate table (phase → verdict → file)
  decisions.md            Phase 0 decisions; every STOP, its reason, and the resumption probe
  candidate/evidence.json verbatim workflow artifact
  candidate/checksums.txt verbatim
  candidate/run.txt       run id/URL, toolchain, suite summary, TestBashUpgrade result, copy listing
  harness/claude.md       the Claude Code row (fields above)
  harness/claude-transcript.txt   sanitized, or a pointer to a private location if > 1 MB
  publication.md          tag object and peeled SHA, release id/URL, asset ids and digests
  public-install.md       URLs, digests, command outputs
  release-notes.md        the notes as published
```

**Sanitization.**

- No tokens or credentials.
- Home paths and account names become `$TEST_HOME`.
- Only Docket-related transcript content is kept.
- A transcript that cannot be sanitized to that standard is summarized instead, and the summary
  says so.

Once the closeout PR merges, `results:` and the bundle are frozen build records.

## Release notes outline

Title: `v1.0.0-alpha.1 — Docket is a Go binary`. Sections, in order:

1. **What this release is.**
   - The first public build of the Go binary, an alpha for existing users.
   - Tested end to end on Claude Code only. Cursor support is proven in alpha.2 and OpenCode in
     alpha.3. Codex is paused.
   - A hard replacement of the Bash implementation, with no Bash fallback.
2. **Install.**
   - The downloader command with `--harness claude`.
   - The four supported platforms (darwin/linux × amd64/arm64).
   - Verifying `install.sh` against `checksums.txt` before running it.
   - The runtime dependencies: `sh`, `curl`, `tar` and one SHA-256 tool.
   - Restart Claude Code after installing.
3. **Upgrading from v0.9.x.** A link to `docs/release/upgrading-from-bash.md` (0511), plus one
   factual line: the Bash tags `v0.9.2` and `v0.9.3` remain available.
4. **What changed since v0.9.3.** Themes, each naming its changes:
   - the Go foundation;
   - the Bash control plane removed;
   - the CLI surface (capability catalog, schema, typed operations);
   - configuration;
   - native harness dispatch;
   - the run tracker;
   - the build gate and gate driver;
   - finalize;
   - metadata and the board;
   - the docs.
5. **Known gaps.** Each names its tracking change:
   - the build agent can stall during long test runs, with its recovery steps (0412);
   - Cursor and OpenCode are installable but untested in this release (0512, 0513);
   - Codex is paused (0433);
   - no Homebrew, Windows, or signing/notarization.
6. **Evidence.** A link to `docs/release/v1.0.0-alpha.1/` on `main` and the ADRs the release
   rests on.

## Failure and retry boundary

- STOP before publication on any failed source, candidate, Claude Code lifecycle, checksum or
  evidence gate. Missing or ambiguous evidence fails the gate.
- Resume only from authoritative probes (`git ls-remote`, `gh run view`, `gh release view`) and
  the recorded checksums. A crashed operator session resumes from the feature branch's last pushed
  evidence commit and the probes.
- Never repair source inline. A source change gets its own reviewed change, invalidates the
  candidate, and restarts at Phase 2 from the new merged commit.
- Never automatically compensate a published effect.
- To stop a dispatched run deliberately, use `run cancel` with its key and a reason.

## Alternatives considered

- **`v1.0.0-beta1`** (the 2026-09-03 decision). Replaced: alpha.1 signals an early build for known
  users, proven one harness at a time.
- **A fresh macOS user account.** Dropped. A test home folder on the operator's account isolates
  Claude Code's assets just as well, because Claude Code reads them from `$HOME/.claude`.
- **Three harness rows in one release.** Replaced by one harness per alpha.
- **A rollback rehearsal, recorded smoke evidence, Bash-install upgrade probes, and a backlog
  audit.** Dropped as more ceremony than an alpha needs. The upgrade path is proven by 0511's
  suite test inside the candidate's source gate.
- **A tag-triggered publishing workflow.** Rejected. The value is a human at the irreversible step.

## Out of scope

- Source changes of any kind; changing or substituting accepted bytes.
- A Bash fallback or compatibility launcher.
- Stable `v1.0.0`; alpha.2 and alpha.3 (0512, 0513).
- Cursor, OpenCode and Codex testing or support claims.
- Rollback rehearsal; four-platform smoke evidence; backlog or ledger audit.
- Homebrew, Windows, signing/notarization, SBOM or provenance.
- A publishing workflow.
- Any redesign of storage, the JSON protocol, harness topology, or the Git/GitHub adapters.

## Acceptance boundary

- **Designed:** this spec is linked from the record.
- **Implemented:** the closeout PR is open with every phase's evidence in the bundle and every gate
  green (or a recorded human downgrade listed in the notes' known gaps); the tag and release exist
  at the candidate with six verified assets; and the public install check passed.
- **Done:** `docket-finalize-change` archives the change and the sweep is clean.

## Follow-ups to report (never minted by this change)

- A tag-triggered publishing workflow, for a later release.
- Disposition of the undocumented repo-root `link-skills.sh`.
