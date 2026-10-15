<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0548 — Retire the fake-version test fixture folders](../../changes/active/0548-rename-the-v0-9-13-frozen-fixture-tree-to-the-1-0-release-se.md)**
<!-- docket:backlink:end -->

# Retire the fake-version test fixture folders — design

## Problem

`testdata/repositories/` holds the sample files that tests run against, in folders named like releases: `v0.9.2` through `v0.9.13`. Only `v0.9.2` and `v0.9.3` are real releases (git tags). Each of `v0.9.4` through `v0.9.13` was simply "the next number", cut whenever a test needed a fresh copy of a file. The Bash line ended at `v0.9.3`, but the counting went on. The names mislead anyone who reads a test path or failure message: `v0.9.13` looks like a stale release now that docket ships 1.0 candidates, and `testdata/README.md` says the folders are "versioned by the docket release that produced them".

The tree mixes two different kinds of fixture:

1. **Release samples.** Real material taken from a docket release, kept so tests can prove today's code still handles what that release wrote: `v0.9.2` (configs and records the Go config and document readers must accept) and `v0.9.3/status-corpus` (the repository at the `v0.9.3` tag, which Go `status` must read the same way). These must never change, and the release name is the point.
2. **Copies of files docket ships.** A byte copy of `agents/harness-defaults.yml`, a byte copy of docket's own `.docket.yml`, and the hand-written `deferred-active` config, which must switch on every repository-settable deferred capability. The "never edit, make a new folder" rule written for kind 1 was applied to these too. Every edit to a shipped file therefore cut a new numbered folder, re-pointed one test at it, and left the old folder unread. That habit produced all ten fake versions. Three of them (`v0.9.8`, `v0.9.10`, `v0.9.12`) were cut for edits that changed nothing the test checks; `v0.9.8` was a comment rewording.

A byte copy that must always equal the original adds no protection. The test that matters can read the shipped file directly, and still fails whenever an edit changes what it checks.

## Decision

- **Release samples keep their release names** and are never edited. A release-named folder is created only when a test needs a sample taken from that release. Tagging a release creates nothing, which is why no `v1.0.0-rc.1` folder exists or should.
- **Tests read shipped files directly.** The parity test reads `agents/harness-defaults.yml`. The docket-self test copies the checkout's `.docket.yml` into a temp directory at test time. No copy of a shipped file lives under `testdata/`.
- **Hand-written fixtures that must describe docket as it is now live in `testdata/repositories/current/`.** The change that makes one out of date edits it in place; git history keeps the old versions. Today that is `deferred-active` alone.
- **A fixture no test reads is deleted.** Git history keeps it.

After this change, `testdata/repositories/` holds `v0.9.2/`, `v0.9.3/`, and `current/` (plus the existing `.gitignore`).

Rejected:

- **Rename `v0.9.13` to a 1.0 name, or name folders after releases.** `v0.9.13` was cut after the `v1.0.0-rc.1` tag, the release is unknown when a folder is cut, and several folders can be cut between two releases.
- **Date-named folders.** Honest names, but every edit to a shipped file would still cut a folder and re-point a test.
- **Keep the copies in `current/`, updated in place.** Every edit to a shipped file, even a comment, would still have to be copied into testdata, for no extra protection.

## Design

### 1. Final layout of `testdata/repositories/`

Re-list the folders on `main` at build time rather than trusting this table. A change that edits a shipped file may land first and cut another numbered folder (for example `v0.9.14`); apply the same rule to it. Copies of shipped files are deleted, and a newer `deferred-active` becomes the one that moves into `current/`.

| Today | After |
|---|---|
| `v0.9.2/` | kept, minus `agents-harness-defaults.yml`, `docket-self/`, and `deferred-active/` |
| `v0.9.3/` | kept, minus `agents-harness-defaults.yml` and the root `PROVENANCE.md` that covers only that file; `status-corpus/` and its own `PROVENANCE.md` stay |
| `v0.9.4/` … `v0.9.12/` | deleted |
| `v0.9.13/deferred-active/` | `git mv` to `current/deferred-active/`, byte-for-byte (a 100%-similarity rename) |
| rest of `v0.9.13/` | deleted |

Update `v0.9.2/PROVENANCE.md` so its contents list names only what remains. Write the entries for the removed fixtures out of the list, and reword the `agents-harness-defaults.yml` line, which also introduces "the configuration-only fixtures". Do not add notes about what was removed; git history records that.

Add `current/PROVENANCE.md`, covering the whole folder. For each fixture it says:

- that the fixture is hand-written;
- what it must cover (`deferred-active`: every repository-settable deferred capability);
- which test enforces that;
- that the change which makes it out of date edits it in place.

### 2. Agent-defaults parity reads the shipped file

In `internal/config/defaults_test.go`:

- Remove `sidecarPath` and the byte-equality call. The parity test parses `agents/harness-defaults.yml` (`liveSidecarPath`, renamed to describe what it is) and compares it with the Go built-in table, as it does today against the copy.
- Rename `TestBuiltinAgentsParityWithFrozenSidecar` to `TestBuiltinAgentsParityWithShippedDefaults`.
- Its messages stop saying "frozen sidecar". The mismatch message says that `agents/harness-defaults.yml` and the built-in table in `internal/config/defaults.go` must change together.

Update the header comment in `internal/config/defaults.go` to name `agents/harness-defaults.yml` as the parity oracle. Update the `internal/repoguard/absence_test.go` comment that says the file is "byte-pinned twice — by TestBuiltinAgentsParityWithFrozenSidecar's frozen sidecar (currently v0.9.13) and by the embedded-tree copy": it is now pinned by the parity test reading it directly, and by the embedded-tree copy.

### 3. The docket-self test reads docket's own `.docket.yml`

In `internal/config/fixtures_test.go`, `TestFixtureDocketSelf` builds its fixture at test time in a temp directory:

- `repo/.docket.yml`, a copy of the checkout's `../../.docket.yml`;
- `xdg/docket/config.yml`, written from a string in the test. It is the two-line global layer requesting `auto_capture.enabled: true`, identical to today's fixture file.

The test then resolves that directory through `loadFixtureFrom` as before.

- **Only `.docket.yml` is copied, never the checkout root.** A developer's `.docket.local.yml` must not reach the test.
- **The expectations stay as they are:** the finalize test command, the blocker set, and each blocker's layer. The byte-equality assert and its "cut a NEW versioned fixture tree" remedy are removed.
- **A failed expectation explains itself.** It says that docket's own `.docket.yml` changed what this test checks, and that the expectations must be re-derived against it.

### 4. `deferred-active` lives in `current/`

- `TestFixtureDeferredActive` reads `current/deferred-active`.
- `copyFixtureRepo` in `cmd/docket/config_cli_test.go` is used only for `deferred-active`. Its root changes from `v0.9.2` to `current`, so both readers use the one copy.
- When the registry gains a repository-settable deferred row, `TestFixtureDeferredActive` already fails because it derives the expected blockers from the registry. Its failure message names the file to edit: `testdata/repositories/current/deferred-active/repo/.docket.yml`.

### 5. Leftover helpers and comments

- `assertFrozenCopyMatchesLive` has no callers left; delete it.
- Remove the `docketSelfRoot` and `deferredActiveRoot` constants, and add one constant for the `current/` root.
- Rewrite the comments on `loadFixtureFrom` and `mustResolveFixtureFrom` ("parameterized by the versioned tree root", "re-cut into a newer versioned tree") and the history comments on `TestFixtureDocketSelf` and `TestFixtureDeferredActive`. They should describe what the code does now.
- Update the `internal/repoguard/repoguard.go` comment that cites `testdata/repositories/v0.9.2..v0.9.6`.
- In the synthetic table of `internal/repoguard/absence_test.go`, change the made-up path `testdata/repositories/v0.9.6/x.sh` to `testdata/repositories/current/x.sh`.
- **Final grep:** search code, scripts, and current docs for `v0.9.4` through `v0.9.13`, `sidecarPath`, "frozen sidecar", "versioned fixture tree", "versioned tree", and `FrozenSidecar`. It comes back empty.
  - Excluded from the search: frozen build records (old plans, specs, results, archived change records).
  - Also excluded: `internal/install/`'s legacy reproducer. Its "frozen sidecar" is the embedded v0.9.2 floor under `internal/install/legacydata/`, which is a different, still-needed copy.

### 6. `testdata/README.md`

Rewrite the repository-fixtures section to describe current behavior only:

- **Heading:** `testdata/repositories/`, with the two folder kinds described below.
- **`v<release>/`:** a sample taken from a real docket release, never edited. One is created only when a test needs a sample from that release; tagging a release creates nothing.
- **`current/`:** hand-written fixtures that must describe docket as it is now. They are edited in place by the change that makes them out of date.
- **Shipped files are read directly:** tests that check a file docket ships read it from where it lives, and no copy goes under `testdata/`.
- **Unread fixtures are deleted;** git history keeps them.
- **Provenance:** every folder has a `PROVENANCE.md`, either one for the whole folder (`v0.9.2/`, `current/`) or one per fixture (`v0.9.3/status-corpus/`), never both in one folder. Drop the multi-owner exception, which no folder uses any more.
- **Rules that stay:** copy-before-mutation, and expected outputs live with the test.
- **Delete** the closing "Change 0304 establishes the convention only…" paragraph.
- **`bash-upgrade` section:** "exactly like the versioned trees above" becomes "exactly like the release samples above".

### 7. Folder-name guard

Add a test in `internal/repoguard` that lists the directories under `testdata/repositories/`. It fails on any name that is neither `current` nor in a release-sample allowlist (`v0.9.2`, `v0.9.3`). Its message states the rule and points at `testdata/README.md`, and that stops a builder from cutting `v0.9.14` out of habit. Adding a future release sample means adding its tag to the allowlist, and the README says so.

Put the name check in a small function and unit-test it directly:

- `current`, `v0.9.2`, and `v0.9.3` pass;
- `v0.9.14`, `v1.0.0-rc.1`, `2026-10-11`, and `fixtures` fail.

### 8. Learnings

The learnings finding `config-edit-trips-its-own-frozen-drift-guard` teaches the old protocol: edit `.docket.yml`, the byte guard reddens, cut a new versioned tree. Through `learning.update`, replace its `## Apply` section with the current rule:

- docket's tests read `.docket.yml` and `agents/harness-defaults.yml` directly, so an edit fails only a test whose expectations it changes, and the fix is to re-derive those expectations;
- the "never edit, make a new folder" rule applies only to release samples.

Keep the war story. This lives on the metadata branch, not in the PR.

## Testing

- Full suite green.
- The folder-name guard's unit cases (section 7) pass, and the guard passes on the new layout.
- Throwaway local checks, recorded in the results, never committed:
  1. Change one model in `agents/harness-defaults.yml` → `TestBuiltinAgentsParityWithShippedDefaults` fails with the "must change together" message. Revert.
  2. Change `finalize.test_command` in `.docket.yml` → `TestFixtureDocketSelf` fails with the re-derive message. Revert.
  3. Put a `.docket.local.yml` requesting a deferred capability at the checkout root → `TestFixtureDocketSelf` still passes. Remove it.
- Build verification, recorded in the results:
  - `ls testdata/repositories` shows exactly `v0.9.2`, `v0.9.3`, `current`, and `.gitignore`;
  - `git diff -M100% --name-status origin/main...HEAD -- testdata/repositories` shows `deferred-active` as an `R100` rename;
  - the section-5 grep is clean.

## Out of scope

- Renaming `v0.9.2` or `v0.9.3`, or editing any fixture that stays in them. At grooming, every remaining fixture in both is read by a test.
- The legacy installer's embedded v0.9.2 floor (`internal/install/legacydata/`) and its goldens (`internal/install/testdata/legacy/`).
- `testdata/bash-upgrade/` and package-local `testdata/` directories.
- Editing frozen build records (old plans, specs, results, archived change records) that name the old folders.
