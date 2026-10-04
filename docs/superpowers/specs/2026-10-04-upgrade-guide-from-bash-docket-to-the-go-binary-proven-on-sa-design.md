<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0511 — Upgrade guide from Bash docket to the Go binary, proven on saved v0.9.2 and v0.9.3 installs](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0511-upgrade-guide-from-bash-docket-to-the-go-binary-proven-on-sa.md)**
<!-- docket:backlink:end -->

# Upgrade guide from Bash docket to the Go binary — design

**Change:** 0511 · **Type:** docs · **Priority:** critical · **Date:** 2026-10-04 · **Status:**
Approved design

## Purpose

Give the few known Bash-era docket users a written, proven path from their Bash install, and their
repositories on the `docket` branch, to the Go binary. Prove the path on saved copies of real
v0.9.2 and v0.9.3 Bash state, so every step in the guide is known to work against the binary that
ships. This change blocks `v1.0.0-alpha.1` (change 0366), which depends on it.

## Decisions made with the human (2026-10-04)

| Topic | Decision |
|---|---|
| Audience | A few known users. They installed Bash docket from the **v0.9.2 or v0.9.3 tags**. Nobody needs coverage for tracking `main`. |
| Repositories | Only repositories **already on the `docket` branch**. Single-branch repositories are not a concern. |
| Harness | **Claude Code only** for alpha.1. Cursor follows in alpha.2 and OpenCode in alpha.3; each extends this guide and test. Codex is paused. |
| Proof | **Saved Bash state plus a test** that runs the guide's own commands on copies of it. |
| v0.9.3 install conflict | A **hand remedy in the guide**: delete the named generated files and re-run. No installer change. |
| Lifetime | The saved cases and the test are **temporary**: retire them when stable v1.0.0 ships. The guide then stays as a frozen document. A deferred retirement stub tracks this. |
| Release name | `v1.0.0-alpha.1` (owned by 0366's revise; noted here only for the guide's wording). |

## What the code does today (traced at `main` @ `6908393f3`)

These facts set the guide's content and the test's expectations. The build re-verifies each one
and treats any it cannot confirm as something to observe, not assume.

**Machine install.**

- Legacy takeover is the frozen v0.9.2 reproducer (`internal/install/legacy.go`,
  `internal/install/legacy_inputs.go`, ADR-0096). Its closed set `legacyBuiltinAgents` has the
  sixteen v0.9.2 agents and deliberately leaves out `plan-writer`. A file whose bytes match
  nothing in that set is an ownership conflict, and there is no `--force`.
- Two install routes exist:
  - the release downloader, `internal/release/downloader/install.sh`, which forwards `--harness`
    verbatim to `docket install`;
  - the contributor bootstrap, the repo-root `install.sh`, which uses
    `go run … development install` when no `docket` is on `PATH`. That route needs a Go
    toolchain, and `docs/install/install.md` does not list Go as a prerequisite.

**Repository.**

- `repository migrate` picks its route from what is on the remote (`migrateRoute`) and only
  converts the single-branch layout. Its no-op message for an already-migrated repository points
  to `repository repair`.
- Taking over a receipt-less `docket` branch goes through `verifyMetadataOwnership`
  (`metadata_ownership.go`). It accepts only:
  - an empty-tree root (`proofLegacyEmpty`), or
  - a root equal to a past integration snapshot (`proofLegacyEquivalent`).

  **It is unproven whether a branch made by Bash v0.9.2 or v0.9.3 passes.** This is the
  highest-risk unknown in the change.
- Findings a Bash `docket`-branch repository is expected to trip (`internal/reposetup/health.go`
  `conditionFinding`):

  | Finding | Fix |
  |---|---|
  | `legacy-config-key-present` | hand edit: remove `metadata_branch` |
  | `committed-ignore-invalid` (the old installer's markers, `IgnoreDefectLegacyOnly`) | hand edit |
  | `docket-worktree-hooks-enabled` | hand edit |
  | `test-config-missing` | `docket repository configure-tests` |

- `repository repair` (`repository_repair.go`) applies, with `--yes`, only the fixes on the
  `docket` branch: `board-stale`, `adr-index-stale`, `artifact-links-missing`/`-stale`, and the
  frontmatter repairs `quote-unsafe-scalar`, `scalar-to-list` and `drop-final-claimed-at`. A
  Bash `BOARD.md` is adopted byte for byte, so expect `board-stale`.

**Configuration.**

`internal/config` (`schema.go` `buildRegistry`, `capability.go`, `decode.go`) sorts the old
settings like this:

| Behavior | Settings |
|---|---|
| Warn and ignore | `metadata_branch`, `runtime.bash` |
| Block only when switched on | `terminal_publish`, `auto_groom`, `build.checkpoint`, `finalize.skip_results_only_delta`, `auto_capture.enabled`, `dummy_mode.enabled` |
| Block by value | `finalize.gate: ci` or `both`; `board_surfaces` containing `github` |
| Block whenever present | any `skills.<role>`; `agents.*.*.runner`; `agents.*.*.model`/`effort` in a repository layer |
| Info notice only | `learnings.cap`, `github_project`, `delegation_observation_budget`, and the companion keys |
| Invalid (commands refuse) | any other unknown key; only `docket install` downgrades it to a warning |

Mutation preflight (`PreflightMutation`) runs on claim, halt, reclaim, relink, recertify,
finalize, maintenance and install. `change create`/`groom` check only the `github` board surface.

**Leftovers.**

- Per-repository agent wrappers (`.claude/agents/docket-*` and the other harness folders) are not
  cleaned up. Their ignore globs are kept on purpose.
- Old dispatch blocks in a repository's `CLAUDE.md`/`AGENTS.md` are adopted or retired by
  `docket install` only when the repository declares `agent_harnesses` in its repository layer.

**In-flight work.** `.worktrees/<slug>` paths and `claimed_at` still decode. No guidance exists
on draining in-flight changes before upgrading.

**Guards the build must respect.**

- `TestLivingDocsAlignment` (`internal/repoguard/docs_alignment_test.go`, `livingDocRoots`)
  rejects refused setting spellings and change citations in `README.md` and
  `docs/{guide,install,concepts,reference}`. It excludes `docs/release/`.
- `TestNoRetiredBashControlPlane` (`internal/repoguard/absence_test.go`) scans the executable
  surface.
- `TestRetiredVocabularySeal` exempts `docs/` and `testdata/`.

## Design

### 1. Saved Bash state (built once, at build time)

**How it is made.** For each tag, `v0.9.2` and `v0.9.3`:

1. Check the tag out into a scratch directory **outside** the repository.
2. Set up a sandbox: a fresh temporary `HOME` with every `XDG_*` variable pointing inside it, and a
   local bare repository as `origin`. Nothing touches the real `HOME`, `~/.claude`, `~/dev/docket`,
   GitHub or the network.
3. Run that tag's own `install.sh` for every harness it supported, so the saved home folder carries
   the Claude Code, Cursor and OpenCode files later alphas need. Codex files are saved if the tag
   wrote them, but they are never asserted.
4. Create a consuming repository through that tag's own `docket`-branch bootstrap.
5. Populate it through that tag's own flows. The minimum content is:
   - one proposed change with a spec;
   - one trivial proposed change;
   - one deferred change;
   - two done changes in the archive;
   - one ADR;
   - the board as that version rendered it;
   - learnings, if the version had them.

   Where a flow needs an interactive agent step, use the tag's own scripts for the equivalent
   effect, or record the gap in the provenance note.
6. Run the Bash scripts only as subprocesses with the sandbox environment. Never source them into
   the agent's shell.

**Configs.**

- **v0.9.2** keeps the `.docket.yml` its bootstrap produced: the common path.
- **v0.9.3** sets every key that tag's own schema accepted and that Go now refuses or warns about,
  drawn from the configuration table above. That proves the clean-up path. The provenance note
  lists exactly which keys were set and which the tag did not accept.

**Saved form.** Each tag gets a directory `testdata/bash-upgrade/<tag>/` holding:

- a git bundle of the sandbox `origin` with every ref (the integration branch and the `docket`
  branch, exact history);
- the sandbox home folder's harness directories, with symlinks preserved. That includes the skills
  tree the links point at, so a restored link resolves the way it did on a user's machine whose
  Bash checkout was left untouched. Absolute sandbox paths are rewritten to one placeholder token
  that the test substitutes;
- `PROVENANCE.md`: the tag and its commit, the SHA-256 of the tag's `install.sh`, the exact
  commands run, the date, what each record exercises, and any gap.

Keep each tag's directory small, ideally under 1 MB. No generator script is committed. The tags are
the generator, and a committed Bash script would join the retired-Bash guard's population.

### 2. The proof test

- **Shape.** An `integration`-tagged Go test with a `TestBashUpgrade` name prefix, placed in the
  topical shard `tests/README.md` *Where new tests go* points to. Add a runtime budget row if the
  suite requires one.
- **Setup per tag.** Restore the bundle into a bare `origin` plus a clone, restore the home folder
  (substituting the placeholder), and use the docket binary under test.
- **The guide is the script.** The guide marks each runnable step: a marker comment such as
  `<!-- upgrade-step: <name> -->` directly before a fenced shell block. The test:
  - extracts the marked blocks in order and runs their docket commands, with the restored
    repository as the working directory and the sandbox as `HOME`;
  - substitutes only a small fixed set of placeholders, such as the repository path;
  - mirrors each hand-edit step (removing a setting, deleting old agent files, removing an old
    dispatch block) with its own edit, keyed to that step's marker.

  A guard fails the test if:
  - a marked step is neither executed nor mirrored;
  - an unmarked fenced block in the guide contains a `docket` command.
- **The install step.** No published release exists at build time, so the test runs the same
  `docket install --harness claude` that the downloader hands off to. The real download is proven
  by 0366's public-install phase.
- **v0.9.3 only.** The first install run must report an ownership conflict naming the expected
  files. That proves the guide's remedy is needed and names the right files. After the remedy, the
  re-run succeeds.
- **Required end state, per tag:**
  - `docket install check` is clean for Claude Code;
  - `docket repository check` is clean;
  - `docket status` reports zero error findings;
  - every pre-existing change, ADR and spec record survives, and the counts match the provenance
    note;
  - a `change create` of a new stub succeeds and shows on the re-rendered board, which proves the
    repository is writable.
- **Mutation checks.** Each is recorded in the results:
  - deleting a marked step from a copy of the guide turns the test red;
  - dropping the v0.9.3 remedy turns the v0.9.3 case red.
- **Scope of assertions.** Only Claude Code is asserted. Cursor and OpenCode files are restored but
  left unasserted until alpha.2 and alpha.3.

### 3. The guide — `docs/release/upgrading-from-bash.md`

**Why there.** It must name the refused settings, and `docs/release/` is the one docs root the
living-docs guard excludes. It is release-transition material, which fits that folder's purpose.
Link to it from `README.md` (the install section) and from `docs/install/install.md` (*Adopting
docket in a repository*). Neither link spells a refused setting.

**Voice.**

- Written for users, in plain words.
- Present behavior only: no change numbers and no history narration.
- Claude Code only. It says Cursor and OpenCode sections arrive in later pre-releases and that
  Codex is not supported.

**Outline.**

1. **Who this is for.** A Bash docket v0.9.2 or v0.9.3 install with repositories on the `docket`
   branch. It says plainly that single-branch repositories are not covered.
2. **Before you start.** Finish or park (defer) every in-flight change, and know how to roll back.
3. **Install the Go binary.** The release download is the upgrade route: download `install.sh`
   and `checksums.txt`, verify the checksum before running, then run with `--harness claude`.
   Confirm with `docket version` and `docket install check`. The checkout route
   (`git pull`, then `bash install.sh`, which needs Go) gets one sentence pointing to
   `docs/install/install.md` as the contributor path. It is not an upgrade step, because this
   test does not prove it over a Bash home folder.
4. **Take over the old Claude Code install.**
   - What is taken over automatically.
   - The ownership-conflict remedy: delete the `docket-*` files the installer names under
     `~/.claude/agents/`, then re-run.
   - The old dispatch block in `~/.claude/CLAUDE.md`, if the installer reports one.
5. **Upgrade each repository.** `docket repository prepare`, then `docket repository check`. Fix
   each finding using a finding-to-fix table. Then `docket repository configure-tests`, then
   `docket repository repair` (preview) and `--yes`. Commit and push config edits on the default
   branch.
6. **Settings that changed.** A table of each old `.docket.yml` setting, what Go does with it
   (blocks writes, warns, or ignores it), and the fix.
7. **Leftovers you can delete.** Per-repository agent files and anything else the test shows Go
   ignores.
8. **Restart Claude Code.** A new process; clearing a conversation is not enough.
9. **If something goes wrong.** Roll back with the v0.9.2 tag's `install.sh`.

**Truth rule.** Every fact the guide states is one the test observed, or one 0366's release
verification proves: the real download and its checksum step. A step or claim neither exercises is
either removed or explicitly marked as not covered.

### 4. When the binary is wrong

If following the guide on a saved case fails because the binary misbehaves, and the guide cannot
honestly explain it, the build **halts** with `## Run halted` naming the defect and its evidence.
The human then captures a fix change, adds it to this change's `depends_on`, and resumes. A "step"
that would ask users to hand-edit the `docket` branch, or to bypass a refusal, counts as a defect,
not a guide step. The most likely candidate is `verifyMetadataOwnership` refusing a Bash-made
`docket` branch.

### 5. Lifetime and the later alphas

- **alpha.2 (Cursor) and alpha.3 (OpenCode)** each add their harness's guide section and test
  assertions, reusing the saved cases unchanged.
- **When stable v1.0.0 ships,** a retirement change deletes `testdata/bash-upgrade/` and the test.
  The guide stays as a frozen record. A deferred stub tracks the retirement.

### 6. Relationship to 0366

- 0366 depends on this change.
- 0366's upgrade-probe phase follows this guide against the candidate's packaged bytes, including
  the real downloader. A guide step that fails there is a release STOP.
- The alpha.1 release notes link this guide instead of carrying their own upgrade steps.

## Out of scope

- Single-branch repositories.
- Cursor and OpenCode guide sections and assertions (saved only). Codex.
- Widening ADR-0096's frozen floor to recognize v0.9.3.
- Users who tracked `main` between v0.9.3 and the Go cutover.
- The alpha.1 release notes (0366).
- Product code changes. A defect gets its own change (section 4).

## Acceptance

- `testdata/bash-upgrade/v0.9.2/` and `testdata/bash-upgrade/v0.9.3/` are committed with complete
  provenance notes.
- The `TestBashUpgrade` cases are green on both tags.
- Both mutation checks were observed red and recorded in the results.
- The guide is published at `docs/release/upgrading-from-bash.md` and linked from `README.md` and
  `docs/install/install.md`.
- The whole suite is green at the build gate, with budget-report lines read and any finding noted.
