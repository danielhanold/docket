# testdata — fixture conventions

Three fixture tiers, one rule each:

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

## Root `testdata/bash-upgrade/<tag>/`

Saved Bash-install upgrade cases: the machine and repository state a user had after installing
Bash docket from a release tag (`v0.9.2`, `v0.9.3`) and running a repository through its
`docket`-branch flows. The upgrade test restores each case into a sandbox and drives the upgrade
guide against it.

- **Made once, from the tag:** each case was built by hand in a throwaway sandbox (temporary
  `HOME`, local bare `origin`) by running that tag's own installer and scripts. The tag is the
  generator; no generator script is committed.
- **Provenance per tag:** every `<tag>/` directory has its own `PROVENANCE.md` recording the tag
  and commit, the installer's SHA-256, every command run, what each record exercises, and every
  gap. `bash-upgrade/PROVENANCE.md` only points at them.
- **Saved form:** `origin.bundle` (all refs of the sandbox `origin`), `home.tar` (the harness
  folders with symlinks kept and the sandbox home rewritten to `@@SANDBOX_HOME@@`),
  `clone-config.txt` (clone actions to replay), `records.txt` (the record inventory), and, where
  the tag wrote ignored files into the repository's working tree, `clone-files.tar` (those files,
  rooted at the clone, in the same tokenized form as `home.tar`).
- **Never edited to make a test pass**, like the samples above: a different state is a new case.
  Tests restore into their own temp directories and never write here.
- **Temporary:** the cases and their test are retired when stable v1.0.0 ships.
