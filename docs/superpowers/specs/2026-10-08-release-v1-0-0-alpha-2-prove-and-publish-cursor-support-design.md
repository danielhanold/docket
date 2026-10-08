<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0512 — Release v1.0.0-alpha.2: prove and publish Cursor support](../../changes/archive/2026-10-08-0512-release-v1-0-0-alpha-2-prove-and-publish-cursor-support.md)**
<!-- docket:backlink:end -->

# v1.0.0-alpha.2 — Cursor acceptance and publication

**Change:** 0512 · **Type:** chore · **Priority:** high · **Groomed:** 2026-10-08 · **Status:** Approved design

## Purpose and boundary

Take one reviewed commit of `main` to a verified, public `v1.0.0-alpha.2` pre-release of the Go
binary, with Cursor proven end to end, and a human present at every external-truth and
irreversible step.

This is the alpha.1 protocol (change 0366, spec
`2026-09-04-human-attended-v1-0-0-rc1-acceptance-and-publication-design.md`) with the Claude Code
row replaced by a Cursor row, trimmed to what alpha.2 needs, and with the gaps alpha.1 hit
designed in up front. Where this spec says "as 0366", the 0366 spec's wording applies with the
version, harness and file names substituted.

This change builds no product code. The Cursor section of the Bash upgrade guide and its test
are change 0543, which merges before the cut. If a source defect turns up anywhere in the
protocol, it goes to a separate reviewed change, the candidate is discarded, and packaging
restarts from the new merged commit.

**Not driven by `docket-implement-next`.** The operator runs this protocol by hand in an attended
session. The change is claimed for its branch and closed out through typed operations (Phases 1
and 6), never planned or built by an autonomous run.

## Decisions (human, 2026-10-08)

| Topic | Decision |
|---|---|
| Upgrade guide | Split out as change 0543 (guide + `TestBashUpgrade` Cursor assertions), built normally, merged before the cut. 0512 depends on it. |
| Cursor isolation | A **separate Cursor instance**: the operator quits their own Cursor, then launches Cursor's binary from the test-home shell with `--user-data-dir` and `--extensions-dir` inside the test home, and logs in fresh. Isolation is probed inside Cursor before the lifecycle, and dry-run in Phase 0 against the alpha.1 binary. A separate macOS user and the headless `cursor-agent` CLI were rejected. |
| Cursor permissions | Cursor runs **outside the sandbox with all permissions**: Run Mode **Run Everything**, and **no** `terminalAllowlist` in the test home's `~/.cursor/permissions.json` (a non-empty allowlist disables Run Everything). The documented Allowlist (with Sandbox) setup is therefore not proven by this release and is listed as a known gap. |
| Kill and resume | Kill the **whole test Cursor instance** (SIGTERM to its main process) as soon as the first build commit lands on the feature branch on origin; relaunch the same way and resume. |
| Verdict lines | The Cursor agent copies every printed `run-*` line verbatim into a scratch file in the test home right after it prints; the harness record quotes that file. |
| Claude Code | Not re-tested beyond the candidate's whole-suite source gate. |
| Latest | `v0.9.3` stays "Latest"; alpha.2 is a pre-release with `--latest=false`. |
| Rollback | No rehearsal, no promise; the notes state only that the Bash tags `v0.9.2` and `v0.9.3` remain available. |

## Current reality (2026-10-08)

| Fact | Value |
|---|---|
| Latest tags | `v1.0.0-alpha.1` (pre-release, 2026-10-05) and `v0.9.3` ("Latest"). `main` is about 230 commits past `v1.0.0-alpha.1`. |
| Fixed since alpha.1 | 0524: `evidence.json` keeps `checksums.txt`'s exact bytes. 0525: finalize merges on a repository whose plan does not answer the branch-rules API, recording a `branch-rules-unavailable` note. 0526: `docket repository configure-tests --command "<cmd>"` sets both gates to `local`. 0523: a behind-only `.docket` copy is no longer reported as a conflict. |
| Cursor install surface | `install --harness cursor` links skills under `~/.cursor/skills/`, writes agent wrappers to `~/.cursor/agents/docket-*.md`, and adds a `sessionStart` hook to `~/.cursor/hooks.json`. The dispatch rule `.cursor/rules/docket-dispatch.mdc` is written per repository, only when that repository's `.docket.yml` or `.docket.local.yml` lists `cursor` in `agent_harnesses`. |
| Cursor sandbox | docket needs the network and must run outside Cursor's sandbox (`docs/install/cursor.md`). |
| Backlog | No change `in-progress`, no open PR (2026-10-08). |
| Candidate workflow, bundle, run tracker verbs | As 0366. Bundle: `docket_v1.0.0-alpha.2_{darwin,linux}_{amd64,arm64}.tar.gz`, `checksums.txt`, `install.sh`. |

## Roles and vocabulary

As 0366: operator, candidate, window, STOP, test home. In addition:

- **Test Cursor.** The Cursor instance launched from the test-home shell with its own user-data
  and extensions directories. It is the only Cursor running during Phase 3. The operator's own
  Cursor stays quit from the start of Phase 3 to its end.

## Protocol

### Phase 0 — Before the cut

Record each item in `decisions.md`: the decision, who made it, when, and the merge SHA or "held".

- **0543** (Cursor upgrade-guide section and its test) is merged.
- No change other than 0512 is `in-progress` once the window opens; every open PR is merged or
  recorded as held until after Phase 6.
- The operator attests that no `/loop` or autonomous session is running against the Docket
  repository.
- **Isolation dry run.** Before the window opens, rehearse the Phase 3 Cursor launch with the
  published `v1.0.0-alpha.1` installed into a scratch test home (`--harness cursor`): quit the
  operator's Cursor, launch the test Cursor as Phase 3 describes, and run the isolation probe
  (Phase 3, step 6). Record the Cursor version and the probe output. A failed probe is a STOP
  here, before any freeze; the human decides how to isolate before continuing. Delete the
  scratch test home afterwards.

### Phase 1 — Cut the candidate and open the window

As 0366 (candidate SHA from `origin/main`, the verbatim `docket status --json` and
`gh pr list --state open`, the no-loops attestation, the freeze rule), with one addition to the
claim step:

- After `change.claim` and `workspace.prepare`, **reconcile** the record and **attach a pointer
  plan** at once: a short plan file that points at this spec's protocol, landed through
  `change.attach-plan` from a plan-only commit carrying the `Docket-Plan-Path:` trailer. Phase 6's
  `change.mark-implemented` refuses without both (`not-reconciled`, `plan-unlinked`).

### Phase 2 — Package once

As 0366, with version `v1.0.0-alpha.2`. The `evidence.json` check is exact: `checksums_txt` must
be byte-equal to the bundle's `checksums.txt`, trailing newline included (0524). A difference is a
STOP with no waiver. Confirm from the source-gate log that the `TestBashUpgrade` cases, including
0543's Cursor assertions, ran and passed.

### Phase 3 — The Cursor test (human-attended)

**Test home setup.**

1. Create the test home (for example `~/docket-alpha2-test/`). Open a shell with `HOME` set to it
   and every `XDG_*` variable pointing inside it.
2. Build a mirror for the unpublished release: a directory `<mirror>` whose `<mirror>/v1.0.0-alpha.2`
   is (or links to) the read-only copy.
3. Install the candidate from the read-only copy:
   `DOCKET_RELEASE_BASE_URL=file://<mirror> sh <copy>/install.sh --harness cursor --bin-dir <test home>/bin`.
   `install.sh` always downloads from its base URL, so the plain form would reach for a release
   that does not exist yet. Put `<test home>/bin` first on `PATH`.
4. Verify:
   - `docket version --json` shows `v1.0.0-alpha.2` and the candidate commit;
   - `docket install check --json` is clean, with `mode: release` and the Cursor harness only;
   - `docket diagnostic runtime --json` shows `supported_target: true`.
5. Inside this shell: `gh auth login`, then `gh auth setup-git`, so HTTPS `git push` works. The
   operator's SSH keys and credential helpers are not in the test home. Copy no other operator
   state.
6. **Launch the test Cursor and probe isolation.**
   - Quit the operator's own Cursor completely.
   - From the test-home shell, start Cursor's application binary directly (not `open -a`, which
     drops the shell environment) with `--user-data-dir <test home>/cursor/user-data` and
     `--extensions-dir <test home>/cursor/extensions`. Log in to Cursor fresh.
   - Set the Run Mode to **Run Everything**. The test home has no `~/.cursor/permissions.json`
     allowlist.
   - **Isolation probe**, run in a terminal inside the test Cursor and through the Cursor agent:
     `echo $HOME`, `command -v docket`, `docket version --json`, `docket install check --json`.
     They must show the test home, `<test home>/bin/docket`, the candidate commit, and
     `mode: release` with the Cursor harness only. Any other answer is a STOP.
   - Record the test Cursor's main-process PID, found by the test-home `--user-data-dir` in its
     command line.

**Remote and fixture.**

1. `gh repo create danielhanold/docket-accept-v1-0-0-alpha-2-cursor --private --clone` inside the
   test home. The clone starts on `master` with nothing pushed.
2. Rename the branch to `main`. Add `README.md` and an executable `test.sh` (`#!/bin/sh` /
   `exit 0`), commit, and push `main`.
3. Run `docket repository init`, then `docket repository configure-tests --command "sh ./test.sh"`.
   Confirm `build` and `finalize` both have `gate: local` and `test_command: sh ./test.sh`.
4. Set `agent_harnesses: [cursor]` in `.docket.yml`, then run
   `docket install --harness cursor --repo-dir <fixture>`. Confirm
   `.cursor/rules/docket-dispatch.mdc` exists in the fixture.
5. Commit and push. `docket repository check --json` must be clean.

**Lifecycle.** The human types every step in a chat in the test Cursor, with the fixture open:

1. Create one stub with the catalog-resolved `change.create` request (type `feat`, "Add a
   greeting line to README"). It is needs-grooming.
2. Groom it with `docket-groom-next <id>` and exit with a short spec.
3. Implement it as the fixture's dispatch rule prescribes: `run.start` with `implement-next`,
   dispatch `docket-implement-next <id>` with the run context, then `run.verdict` with the key.
   The agent appends every printed `run-*` line, verbatim, to `<test home>/run-lines.txt` as it
   appears.
4. **Kill and resume.**
   - From the test-home shell, poll the fixture's origin for the feature branch named in the
     record's `branch:` field, and for its first commit beyond `main`. That commit is the start of
     the build.
   - As soon as it appears, send SIGTERM to the test Cursor's main process and confirm it has
     exited. Record the timestamp and the commit seen.
   - Relaunch the test Cursor exactly as in setup step 6 and open a new chat.
   - Run `run.start` with `implement-next --resume <id>`, dispatch `docket-implement-next <id>`
     again, and run `run.verdict`. The `run-*` lines go to `run-lines.txt` as before.
   - The run must resume that id (never claim another change), reconcile the interrupted work
     through its own continuation or takeover path, and reach `implemented` with an open PR. Obey
     every `run-*` verdict line exactly.
5. Finalize with `docket-finalize-change <id>`. It should merge by itself. A
   `branch-rules-unavailable` note is expected on the private fixture and is not a failure.
6. Run `docket-status`, then check the terminal predicate.

**Terminal predicate.** As 0366: the record is archived `done` with a full-URL `pr:` and `plan:`
and `results:` set; the PR is `MERGED`; the default branch carries the greeting line;
`docket status --json` shows no ready changes and zero error findings;
`docket repository check --json` is clean; the remote metadata branch holds the archived record
and the re-rendered board.

**Record** `harness/cursor.md` with:

- the Cursor version, the Run Mode, and the chat model used;
- the test-home path (as `$TEST_HOME`) and the launch command (paths sanitized);
- the Phase 0 dry-run result and the Phase 3 isolation probe output;
- the candidate commit and the archive's SHA-256;
- proof that the test Cursor started after the install;
- proof the named children ran (`docket-plan-writer`, a `docket-build-*` tier, a
  `docket-review-*` tier);
- the kill timestamp, the feature-branch commit that triggered it, and the resume timestamp;
- the verbatim `run-*` lines from `run-lines.txt`;
- the terminal predicate results;
- the location of a sanitized transcript, or a summary that says it is one.

Archive or delete the disposable repository afterwards; the record keeps the PR URL.

**Gate.** As 0366. A Docket defect goes to a source change and a new candidate. A Cursor vendor
defect is diagnosed against the recorded Cursor version, and the human decides whether alpha.2
ships with it listed as a known gap.

After Phase 3, the operator may quit the test Cursor and reopen their own.

### Phase 4 — Publish (the human's irreversible step)

As 0366, with tag and release `v1.0.0-alpha.2`, title `v1.0.0-alpha.2 — Cursor support`, and the
preconditions unchanged (Phases 1–3 recorded and passing, `origin/main` still the candidate, notes
reviewed, the human's explicit "publish" in `decisions.md`). Probe → act only if absent → verify
→ record for the tag, the draft pre-release, the six assets (never `--clobber`), and the final
`gh release edit v1.0.0-alpha.2 --draft=false --prerelease --latest=false`. Verify that `v0.9.3`
is still "Latest". A tag or release that points elsewhere is a STOP. Never move or delete a
tag; a re-cut ships under a new pre-release number the human chooses.

### Phase 5 — Public install check

As 0366, in a second fresh test home with no Docket and no `DOCKET_RELEASE_BASE_URL`: download
`install.sh` and `checksums.txt` from the `v1.0.0-alpha.2` release URL, verify the script's
SHA-256 before running it, run `sh install.sh --harness cursor --bin-dir <test home>/bin`, and
verify `docket version --json`, `docket install check --json` (clean, Cursor only) and
`docket diagnostic runtime --json`. Record in `public-install.md`.

### Phase 6 — Closeout

The order matters: the final head is gated and published before the change is marked
implemented. Once the change is implemented, `pr.publish` returns `contended` and the PR body
would keep stale evidence.

1. Assemble the evidence bundle and the results record on the claimed feature branch. The results
   record's human-verify section is the gate table, one line per phase.
2. Drive the build gate on the feature worktree through the gate driver to `PASSED`.
3. Record evidence, attach results, and publish the PR. Its body carries the gate table.
4. Then mark the change implemented.
5. Run `docket-finalize-change 512`. The window closes at its merge.
6. Run `docket-status`; it should come back clean. Held PRs may merge now.

Every operation is resolved from the capability catalog.

## Evidence bundle

```text
docs/release/v1.0.0-alpha.2/
  README.md               index: candidate identity, gate table (phase → verdict → file)
  decisions.md            Phase 0 decisions and dry run; every STOP, its reason, and the resumption probe
  candidate/evidence.json verbatim workflow artifact
  candidate/checksums.txt verbatim
  candidate/run.txt       run id/URL, toolchain, suite summary, TestBashUpgrade result, copy listing
  harness/cursor.md       the Cursor row (fields above)
  harness/cursor-transcript.txt   sanitized, or a pointer/summary
  publication.md          tag object and peeled SHA, release id/URL, asset ids and digests
  public-install.md       URLs, digests, command outputs
  release-notes.md        the notes as published
```

Sanitization as 0366: no tokens or credentials; home paths and account names become
`$TEST_HOME`; only Docket-related transcript content; a transcript that cannot be sanitized is
summarized, and the summary says so. Once the closeout PR merges, `results:` and the bundle are
frozen build records.

## Release notes outline

Title: `v1.0.0-alpha.2 — Cursor support`. Sections, in order:

1. **What this release is.** The second public Go build, an alpha. Cursor is tested end to end in
   this release. Claude Code was tested end to end in alpha.1 and is covered here by the test
   suite only. OpenCode is proven in alpha.3; Codex is paused.
2. **Install.** The downloader command with `--harness cursor` (and `--harness claude`); the four
   supported platforms; verifying `install.sh` against `checksums.txt` before running it; the
   runtime dependencies (`sh`, `curl`, `tar`, one SHA-256 tool); restart Cursor after
   installing. For Cursor: docket must run outside Cursor's sandbox, with a link to
   `docs/install/cursor.md`.
3. **Upgrading.** From v0.9.x: a link to `docs/release/upgrading-from-bash.md`, which now covers
   Cursor, plus the line that the Bash tags `v0.9.2` and `v0.9.3` remain available. From
   alpha.1: re-run the installer for `v1.0.0-alpha.2`.
4. **What changed since alpha.1.** Themes, each naming its changes, drawn from
   `git log v1.0.0-alpha.1..<candidate>`.
5. **Known gaps.** Each names its tracking change where one exists:
   - the documented Allowlist (with Sandbox) Cursor setup was not exercised; the acceptance run
     used Run Everything;
   - OpenCode is installable but untested (0513);
   - Codex is paused (0433);
   - no Homebrew, Windows, or signing/notarization;
   - anything the run itself records as a human-accepted gap.
6. **Evidence.** A link to `docs/release/v1.0.0-alpha.2/` on `main` and the ADRs the release
   rests on.

## Failure and retry boundary

As 0366: STOP before publication on any failed source, candidate, Cursor lifecycle, checksum or
evidence gate (missing or ambiguous evidence fails the gate); resume only from authoritative
probes (`git ls-remote`, `gh run view`, `gh release view`) and the recorded checksums, from the
feature branch's last pushed evidence commit; never repair source inline; never automatically
compensate a published effect; stop a dispatched run deliberately only with `run.cancel` and its
key.

## Alternatives considered

- **Keep the upgrade guide inside this change.** Rejected: it is code plus a test that must run
  in the candidate's source gate, and this protocol builds no code. It is change 0543.
- **A separate macOS user account for Cursor.** Rejected as too heavy for an alpha, as in alpha.1;
  a separate Cursor instance with its own user-data directory, probed, isolates enough.
- **The headless `cursor-agent` CLI.** Rejected: it is not the IDE users run.
- **The documented Allowlist (with Sandbox) mode.** Not used: the human chose to run Cursor
  outside the sandbox with all permissions. The gap is stated in the notes.
- **Stopping the chat instead of killing Cursor.** Rejected: it does not prove that a dead
  process's run can be resumed.

## Out of scope

- Source changes of any kind, including the upgrade guide (0543).
- OpenCode (alpha.3, 0513) and Codex.
- Re-proving Claude Code beyond the candidate's source gate.
- Proving the Allowlist (with Sandbox) Cursor setup.
- Rollback rehearsal, four-platform smoke evidence, backlog or ledger audit.
- Homebrew, Windows, signing/notarization, SBOM or provenance; a publishing workflow.

## Acceptance boundary

- **Designed:** this spec is linked from the record.
- **Implemented:** the closeout PR is open with every phase's evidence in the bundle and every
  gate green (or a recorded human downgrade listed in the notes' known gaps); the tag and release
  exist at the candidate with six verified assets; the public install check passed.
- **Done:** `docket-finalize-change` archives the change and the sweep is clean.
