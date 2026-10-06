<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0534 — Install an alias for the docket binary](../changes/archive/2026-10-06-0534-install-an-alias-for-the-docket-binary.md)**
<!-- docket:backlink:end -->

# Install an alias for the docket binary — Results

**Human action:** One check is recommended before merge: try both installers on a real machine and confirm `dckt` appears beside `docket`. The release smoke test that covers the downloader end to end only runs in release-candidate CI.

## Outcome

Installing docket now also places `dckt`, a short alias for the same binary, in the same bin directory. Both install paths create it:

- **Release downloader** (the published `install.sh`): after it moves the verified binary into place, it creates `dckt` as a relative symlink to `docket`. The alias is created when it is absent, left alone when it already points at this docket, and never replaced when it is anything else.
- **Development installer** (`docket development install`, behind the repository-root `install.sh`): it places the alias inside the same journaled install transaction as the binary. If the install fails and rolls back, the transaction removes an alias it created and leaves any alias that was there before.

A `dckt` that docket does not own is never overwritten, and there is no force option. In that case the install still succeeds and prints a warning that names the path and the remedy. The same applies to a `dckt` docket cannot read or resolve, such as a looping link or a link through a file. `docket install check` reports a missing or foreign alias as a warning finding (`binary-alias-missing` or `binary-alias-foreign`), never as a failure. When the docket binary itself is gone, check reports binary drift instead of a misleading alias warning. `docket uninstall` leaves both the binary and the alias in place.

One departure from the spec: the spec said the downloader's ownership record would let a later run tell its own alias from a foreign one. In practice, ownership is decided by whether the link resolves to the installed binary. The record's `alias=` line is still written, but only for information, and the downloader's contract comment now says so.

The binary behaves identically whichever name invokes it: `dckt version --json` matches `docket version --json`, and a test pins that.

## Human actions and testing

### Important — Release install produces the alias

**Why:** The full release smoke script (`scripts/release-smoke.sh`) now checks the alias, but it only runs in release-candidate CI. Locally, its edits were only syntax-checked. The downloader's own test suites run against a local fixture.

**Prerequisites:** a release bundle (the next release candidate), and a scratch bin directory such as `/tmp/dckt-try/bin` that is not on your normal PATH.

1. Run the release `install.sh` with its bin directory pointed at the scratch directory, following its usage text.
   Expected: the install succeeds, and `ls -l /tmp/dckt-try/bin/dckt` shows `dckt -> docket`.
2. Run `/tmp/dckt-try/bin/dckt version --json` and `/tmp/dckt-try/bin/docket version --json`.
   Expected: the two outputs are identical.
3. Run the installer again.
   Expected: it succeeds with no alias warning, and the link is unchanged.

**Cleanup:** `rm -rf /tmp/dckt-try`, plus the downloader's state record if you pointed `XDG_STATE_HOME` somewhere temporary.

### Optional — Foreign dckt is preserved

1. Before installing, create a file of your own: `echo mine > <bin>/dckt`.
2. Run either installer.
   Expected: the install succeeds and prints a warning naming `<bin>/dckt`. The file still contains `mine`, and `docket install check` lists a `binary-alias-foreign` warning.

## Verification performed

- Full suite (`go run ./cmd/docket development test`) at the final head: 81/81 files green, recorded as build evidence.
- The first full-suite run was red. The bash-upgrade guide integration test copies the downloader's install steps, and that copy did not create the alias, so `install check` warned. The test copy was updated to mirror the downloader, and the next run was green.
- Every new guard was mutation-tested by its build worker: alias classification, transaction placement, rollback, foreign-alias preservation, check reporting, the downloader alias step, and dckt/docket output parity. Each turned red when its code was removed.
- `scripts/release-smoke.sh` was only syntax-checked (`bash -n`). It runs only in release-candidate CI.
- Whole-branch review (deep tier) returned 4 findings: 1 blocker and 3 minor. All 4 were fixed in-branch, and the full table is in the PR body.
- The suite runner noted `SERIAL CONFIRMATION DUE` for `tests/test_go_finalize_e2e.sh` and `PARALLEL-SENSITIVE` timings. These are wall-clock screening notes for files this change does not touch, not budget breaches.
