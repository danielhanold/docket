# testdata — fixture conventions

Three fixture tiers, one rule each:

## Package-local `testdata/`

Narrow unit fixtures and output goldens live in the owning package's own
`testdata/` directory (e.g. `internal/cli/testdata/`). They belong to that
package's tests alone; no other package reads them.

## Root `testdata/repositories/v0.9.2/<fixture-name>/`

Frozen cross-package repository fixtures — snapshots of real docket-managed
repository states, versioned by the docket release that produced them.

- **Provenance:** every fixture records where its content came from (source
  repo, commit, date, and any redaction applied) in a `PROVENANCE.md`, written
  under one of two conventions: **per fixture**, one `PROVENANCE.md` at each
  `<fixture-name>/` root, or **tree-wide**, a single `PROVENANCE.md` at the
  versioned tree's root (e.g. `v0.9.2/PROVENANCE.md`) covering all of that
  tree's fixtures. A versioned tree uses exactly one of the two — never a mix,
  so there is one place to look for any fixture's provenance. **Exception —
  multiple independent owners:** when a versioned tree holds fixtures owned by
  separate changes with no shared provenance, it splits per owner instead. The
  tree-root `PROVENANCE.md` is then scoped to its own fixture(s) alone — it does
  not claim the whole tree — and names each sub-fixture's own `PROVENANCE.md` so
  the split is still discoverable from one entry point (e.g. `v0.9.3/`, where the
  root file covers only the `agents-harness-defaults.yml` sidecar and points at
  `status-corpus/PROVENANCE.md` for the frozen `status` corpus that lives beside
  it). This is the one sanctioned mix, and only because each `PROVENANCE.md`
  still owns a disjoint, explicitly stated set of fixtures.
- **Immutability:** frozen fixtures are immutable source inputs. Never edit a
  file under `v0.9.2/` — a changed input silently re-bases every test that
  reads it. A new upstream state gets a new versioned tree, never an edit.
- **Copy before mutation:** a test that needs to mutate a repository fixture
  copies it into its own temp directory first (`cp -R`) and mutates the copy.
  Tests never write inside `testdata/`.
- **Expected outputs live with the test:** expected transformed output
  belongs beside the owning test (its package `testdata/`), never inside the
  frozen input tree.

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
- **Immutable**, exactly like the versioned trees above: a different state is a new case, never
  an edit. Tests restore into their own temp directories and never write here.
- **Temporary:** the cases and their test are retired when stable v1.0.0 ships.

Change 0304 establishes the convention only; the first frozen fixtures arrive
with the changes that need them (0305 configuration, 0306 documents).
