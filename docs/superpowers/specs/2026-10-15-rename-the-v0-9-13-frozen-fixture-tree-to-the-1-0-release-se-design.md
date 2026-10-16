# Name test fixture folders by what they hold — design

Change 0548 — Name test fixture folders by what they hold

## Problem

The sample files that tests run against live under `testdata/repositories/`, in folders named like releases: `v0.9.2` through `v0.9.13`. The names don't tell a reader what a folder holds or whether it may change, and most of them are wrong:

- **Ten are fake versions.** `v0.9.4` through `v0.9.13` were each "the next number". Tests kept byte copies of files docket ships: `agents/harness-defaults.yml` and docket's own `.docket.yml`. The "never edit a fixture, make a new folder" rule covered those copies too, so every edit to a shipped file cut a new folder, re-pointed one test at it, and left the old folder unread. Eight of the ten are read by nothing today. `v0.9.13` was cut after the `v1.0.0-rc.1` tag, so it isn't even an old release.
- **Even the real names mislead.** The configs in `v0.9.2` are hand-written, not taken from that release. Two of the five records in `v0.9.2/documents` come from a later commit on the `docket` branch.
- **"Never edit" is the wrong rule.** It implies no sample can ever change, when the real point is narrower: a sample must not be changed to make a failing test pass.

A byte copy that must always equal its original adds no protection. A test can read the shipped file directly and still fail whenever an edit changes what it checks.

## Decision

- **`testdata/repositories/` holds only samples of data docket must keep reading:** configs, records, and a repository snapshot. Each folder is named for what it holds. Where its data came from (release, commit, date) goes in its `PROVENANCE.md`, never in a folder name. Tagging a release creates nothing here.
- **The rule for samples** (stated in `testdata/README.md`, §6):
  - A failing test here means docket broke something that already exists in real repositories, so a sample is never edited to make the test pass.
  - A sample is removed only when docket deliberately drops support for it, and the reason is recorded in its provenance file.
  - Adding a new sample is always fine.
- **Input that must keep up with docket is built by the test itself.** A test that checks a file docket ships reads it where it lives. A test that needs a complete, current list writes that input inline. No copy of a shipped file, and no fixture that must be kept up to date, lives under `testdata/`.
- **A fixture no test reads is deleted.** Git history keeps it.

After this change `testdata/repositories/` holds `configs/`, `documents/`, `status-corpus/`, and the existing `.gitignore`.

Rejected:

- **Rename `v0.9.13` to a 1.0 name, or name folders after releases.** The folder postdates the `v1.0.0-rc.1` tag. A release name is unknown when a folder is cut, and several folders can be cut between two releases.
- **Date-named folders.** Honest names, but every edit to a shipped file would still cut a folder and re-point a test.
- **A `current/` folder for fixtures that are edited in place.** That keeps two rules in one tree: one folder is edited and the rest aren't. Its only resident, the complete deferred-capability config, is better placed inside the test that requires it to be complete.
- **One `configs/` folder that is kept up to date.** None of the old configs has changed since it was written (2026-08-13). They are useful because they don't change. `deferred-pairs` still contains `finalize.skip_results_only_delta`, which docket has since retired, and its test proves that an old config containing that setting still loads. Keeping the configs up to date would quietly remove that protection.
- **Keep the byte copies and update them in place.** Every edit to a shipped file, even a comment, would still have to be copied into `testdata/`, for no extra protection.

## Design

### 1. Final layout of `testdata/repositories/`

Re-list the folders on `main` at build time rather than trusting this table. A change that edits a shipped file (for example #554) may land first and cut another numbered folder. Apply the same rule to it:

- a folder that holds only copies of shipped files is deleted;
- a newer `deferred-active` is the one whose content goes inline (§4).

| Today | After |
|---|---|
| `v0.9.2/` `sparse-defaults/`, `example-activated/`, `four-layer-collision/`, `mode-docket/`, `mode-main-custom-paths/`, `fenced-machine-keys/`, `deferred-pairs/`, `deferred-active/`, `invalid/` | `git mv` into `configs/`, keeping their names |
| `v0.9.2/documents/` | `git mv` to `documents/` |
| `v0.9.3/status-corpus/` | `git mv` to `status-corpus/` |
| `v0.9.2/PROVENANCE.md` | split into `configs/PROVENANCE.md` and `documents/PROVENANCE.md` (§2) |
| `v0.9.2/agents-harness-defaults.yml`, `v0.9.2/docket-self/`, `v0.9.3/agents-harness-defaults.yml`, `v0.9.3/PROVENANCE.md` (covers only that copy) | deleted |
| `v0.9.4/` … `v0.9.13/` | deleted. The content of `v0.9.13/deferred-active/repo/.docket.yml` moves into the test (§4). |

Every moved sample keeps its exact bytes; git shows each one as an `R100` rename. `configs/deferred-active/` is the older `v0.9.2` copy, which the command-line tests read (§4). It doesn't request the `agents.*.*.cache_ttl` pin, and nothing requires it to.

### 2. Provenance files

Each of the three folders has exactly one `PROVENANCE.md`, at its root, covering everything inside it.

- **`configs/PROVENANCE.md`** carries over `v0.9.2/PROVENANCE.md`'s header (source repo, commit `096c48de`, date 2026-08-13) and its config fixture lines.
  - It says plainly that these configs were hand-written at that commit to look like configs written for docket at the time. None was captured from a real repository.
  - It drops the `docket-self/` and `agents-harness-defaults.yml` entries.
  - It rewords `deferred-active/` to: "the deferred capabilities a repository could request at that commit, all switched on; the command-line tests use it as a configuration docket refuses to write under. The complete, current list is written inline in `TestFixtureDeferredActive`."
- **`documents/PROVENANCE.md`** is the `documents/` entry from `v0.9.2/PROVENANCE.md`, with its pins: three records from tag `v0.9.2`, and two from the `docket` branch at `6d34592c`.
- **`status-corpus/PROVENANCE.md`**: update its title path, and delete the sentence about the agent-defaults sidecar that shared its tree.

Write no notes about what was removed or moved; git history records that.

`frozenCorpus` in `internal/document/fixtures_test.go` reads every file in its folder, so a `PROVENANCE.md` placed there would be parsed as a record. It must skip `PROVENANCE.md`, the same way the status-corpus loader in `internal/app/status_corpus_test.go` already does. Its "want the 5 captured records" check stays.

### 3. Agent-defaults parity reads the shipped file

In `internal/config/defaults_test.go`:

- Remove `sidecarPath` and the byte-equality call. The parity test parses `agents/harness-defaults.yml` (`liveSidecarPath`, renamed to describe what it is) and compares it with the Go built-in table, exactly as it does today against the copy.
- Rename `TestBuiltinAgentsParityWithFrozenSidecar` to `TestBuiltinAgentsParityWithShippedDefaults`.
- Its messages stop saying "frozen sidecar". The mismatch message says that `agents/harness-defaults.yml` and the built-in table in `internal/config/defaults.go` must change together.

Update the header comment in `internal/config/defaults.go` to name `agents/harness-defaults.yml` as the parity oracle.

Update the `internal/repoguard/absence_test.go` comment that says the file is "byte-pinned twice — by TestBuiltinAgentsParityWithFrozenSidecar's frozen sidecar (currently v0.9.13) and by the embedded-tree copy". It is now checked by the parity test reading it directly, and pinned by the embedded-tree copy.

### 4. Tests that need current input build it themselves

Add a small helper in `internal/config/fixtures_test.go`. It writes a fixture directory into `t.TempDir()` from strings: `repo/.docket.yml`, plus `xdg/docket/config.yml` when one is given. The two tests below use it and resolve the result through `mustResolveFixtureFrom`, unchanged.

- **`TestFixtureDocketSelf`**
  - The repository layer is the checkout's `../../.docket.yml`, read at test time. Copy only that one file, never the checkout root, so a developer's `.docket.local.yml` cannot reach the test.
  - The global layer is today's two-line `auto_capture: enabled: true`, written from a string.
  - The expectations stay: the finalize test command, the blocker set, and each blocker's layer.
  - The byte-equality assert and its "cut a NEW versioned fixture tree" remedy are removed.
  - A failed expectation says that docket's own `.docket.yml` changed what this test checks, and that the expectations must be re-derived by running the resolver against it.
- **`TestFixtureDeferredActive`**
  - Its configuration becomes a Go raw-string constant, starting from the content of `v0.9.13/deferred-active/repo/.docket.yml`.
  - When `repoSettableDeferredPaths` finds a repository-settable deferred row that the YAML doesn't request, the failure names the missing path and says to add it to that constant. The dynamic-row `t.Fatalf` gets the same wording.
  - `fixtureAgentHarness` and `fixtureAgentName` stay; their comment now describes the inline YAML.
- **`copyFixtureRepo` in `cmd/docket/config_cli_test.go`**
  - Its root changes from `v0.9.2` to `configs`.
  - Its two callers need only some configuration docket refuses to write under, and they take their expectations from docket's own output. So the older `configs/deferred-active` serves them unchanged.

### 5. Paths, helpers, comments, and messages

- `fixtureRoot` becomes `../../testdata/repositories/configs`. Delete `assertFrozenCopyMatchesLive`, which has no callers left, and the `docketSelfRoot` and `deferredActiveRoot` constants.
- `frozenDir` in `internal/document/fixtures_test.go` points at `documents`, and `corpusRelDir` in `internal/app/status_corpus_test.go` points at `status-corpus`. Their comments name the new `PROVENANCE.md` paths.
- **Rewrite every comment or message that states the old rule**, such as "immutable input", "Do NOT edit the frozen copy", "re-cut into a newer versioned tree", "parameterized by the versioned tree root", and the history comments on `TestFixtureDocketSelf` and `TestFixtureDeferredActive`:
  - Where it is about samples, it states the §6 rule.
  - Where it is about tests writing into `testdata/` (the `copyFixtureRepo` comment, the file header in `fixtures_test.go`), it says tests never write inside `testdata/`.
  - Comments describe what the code does now, without history.
- `internal/repoguard/repoguard.go`: the comment citing `testdata/repositories/v0.9.2..v0.9.6` describes the folders as named for what they hold.
- Synthetic paths in `internal/repoguard` tests:
  - `absence_test.go`: `testdata/repositories/v0.9.6/x.sh` becomes `testdata/repositories/configs/x.sh`.
  - `repoguard_test.go`: both occurrences of `testdata/repositories/v0.9.2/x.md` become `testdata/repositories/configs/x.md`.
- `internal/repoguard/retired_vocabulary_test.go`: the negative-control string "the frozen fixture testdata/repositories/v0.9.2/fenced-machine-keys/ keeps its name" names `configs/fenced-machine-keys/`. It must still pass as a negative control.
- **Final grep:** search code, scripts, and current docs for:
  - `repositories/v0.`, `sidecarPath`, "frozen sidecar", `FrozenSidecar`, "versioned tree", "versioned fixture tree", and `assertFrozenCopyMatchesLive`.
  - It comes back empty.
  - Excluded from the search:
    - frozen build records (old plans, specs, results, archived change records) and ADRs;
    - `internal/install/`, whose "frozen sidecar" is the embedded legacy floor under `internal/install/legacydata/`, a different and still-needed copy;
    - `testdata/bash-upgrade/` and `internal/bashupgrade/`, whose `v0.9.2`/`v0.9.3` are real release tags.

### 6. `testdata/README.md`

Rewrite the repository-fixtures section. It describes current behavior only.

- **Heading:** `` ## Root `testdata/repositories/<kind>/` ``.
- **What lives here:** samples of data docket must keep reading, in folders named for what they hold: `configs/`, `documents/`, `status-corpus/`. Where a sample came from is recorded in its folder's `PROVENANCE.md`, never in a folder name. Tagging a release creates nothing here.
- **The rule, in these words:**

  > These are samples of data docket must keep reading. A failing test here means docket broke something that already exists in real repositories, so don't edit a sample to make the test pass. Remove a sample only when docket deliberately drops support for it, and record why in its provenance file. Adding new samples is always fine.

- **What doesn't live here:** input that must keep up with docket. A test that checks a file docket ships reads it where it lives. A test that needs a complete, current list builds its input itself.
- **Provenance:** one `PROVENANCE.md` at each folder's root covers everything in that folder. A test that walks a folder skips it.
- **Unread samples are deleted;** git history keeps them.
- **Rules that stay:** copy before mutation (tests never write inside `testdata/`), and expected outputs live with the test.
- **Delete** the multi-owner provenance exception and the closing "Change 0304 establishes the convention only…" paragraph.
- **`bash-upgrade` section:** "**Immutable**, exactly like the versioned trees above: a different state is a new case, never an edit." becomes "**Never edited to make a test pass**, like the samples above: a different state is a new case."

### 7. Folder-name guard

Add a test in `internal/repoguard` that lists the directories directly under `testdata/repositories/`. It fails on any name that looks like a version or a date, meaning one that matches `^v?[0-9]`.

Its message:

- names the folder;
- states the rule: name sample folders for what they hold, and record the release or date in `PROVENANCE.md`;
- points at `testdata/README.md`.

Put the name check in a small function and unit-test it directly:

- `configs`, `documents`, `status-corpus`, and `visibility-modes` pass;
- `v0.9.14`, `v1.0.0-rc.1`, `1.0.0`, and `2026-10-11` fail.

### 8. Learnings

The learnings finding `config-edit-trips-its-own-frozen-drift-guard` teaches the old protocol: edit `.docket.yml`, watch the byte guard go red, cut a new versioned tree. The build updates it through `learning.update`, on the metadata branch rather than in the PR.

- **`hook`:** "Editing docket's own `.docket.yml` can fail `TestFixtureDocketSelf`. The fix is to re-derive its expectations from the resolver's real output, and a spec exclusion naming `internal/config` means 'don't change the logic', not 'don't touch that test'."
- **`## Apply`:**
  - docket's tests read `.docket.yml` and `agents/harness-defaults.yml` directly, so an edit fails only a test whose expectations it changes;
  - re-derive those expectations by running the resolver and reading its actual output, never by guessing;
  - samples under `testdata/repositories/` follow the README rule: never edit one to make a test pass;
  - keep the spec-exclusion advice, reworded without "drift guard" or "versioned tree".
- **`changes`:** add 548.
- Keep the war story.

## Testing

- Full suite green.
- The folder-name guard's unit cases (§7) pass, and the guard passes on the new layout.
- Throwaway local checks, recorded in the results and never committed:
  1. Change one model in `agents/harness-defaults.yml` → `TestBuiltinAgentsParityWithShippedDefaults` fails with the "must change together" message. Revert.
  2. Change `finalize.test_command` in `.docket.yml` → `TestFixtureDocketSelf` fails with the re-derive message. Revert.
  3. Put a `.docket.local.yml` requesting a deferred capability at the checkout root → `TestFixtureDocketSelf` still passes. Remove it.
  4. Delete one line from the inline deferred-active YAML → `TestFixtureDeferredActive` fails, naming the missing path and the constant. Revert.
  5. Create an empty `testdata/repositories/v0.9.14/` → the folder-name guard fails. Remove it.
- Build verification, recorded in the results:
  - `ls testdata/repositories` shows exactly `configs`, `documents`, `status-corpus`, and `.gitignore`;
  - `git diff -M100% --name-status origin/main...HEAD -- testdata/repositories` shows every moved sample file as `R100`, so no sample's bytes changed;
  - the §5 grep is clean.

## Out of scope

- Changing any sample's contents, or renaming individual fixtures (`fenced-machine-keys` keeps its name).
- The legacy installer's embedded v0.9.2 floor (`internal/install/legacydata/`) and its goldens (`internal/install/testdata/legacy/`).
- `testdata/bash-upgrade/`, whose tag-named cases are real tags, and package-local `testdata/` directories.
- Editing frozen build records (old plans, specs, results, archived change records) and ADRs that name the old folders.
