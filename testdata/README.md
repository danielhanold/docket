# testdata — fixture conventions

Two fixture tiers, one rule each:

## Package-local `testdata/`

Narrow unit fixtures and output goldens live in the owning package's own
`testdata/` directory (e.g. `internal/cli/testdata/`). They belong to that
package's tests alone; no other package reads them.

## Root `testdata/repositories/<kind>/`

Repository samples shared across packages, in folders named for what they hold: `configs/`
(configuration files), `documents/` (records), and `status-corpus/` (a repository snapshot for
`docket status`). Where a sample came from is recorded in its folder's `PROVENANCE.md`, never in a
folder name. Tagging a release creates nothing here, and `TestSampleFoldersNamedForWhatTheyHold`
(`internal/repoguard`) fails on a folder named like a version or a date.

These are samples of data docket must keep reading. A failing test here means docket broke
something that already exists in real repositories, so don't edit a sample to make the test pass.
Remove a sample only when docket deliberately drops support for it, and record why in its
provenance file. Adding new samples is always fine.

- **What doesn't live here:** input that must keep up with docket. A test that checks a file
  docket ships reads it where it lives. A test that needs a complete, current list builds its
  input itself.
- **Provenance:** one `PROVENANCE.md` at each folder's root covers everything in that folder:
  source repo, commit or tag, date, and any redaction. A test that walks a folder skips it.
- **Unread samples are deleted;** git history keeps them.
- **Copy before mutation:** a test that needs to mutate a sample copies it into its own temp
  directory first and mutates the copy. Tests never write inside `testdata/`.
- **Expected outputs live with the test:** expected transformed output belongs beside the owning
  test (its package `testdata/`), never inside `testdata/repositories/`.
