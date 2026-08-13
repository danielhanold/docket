# testdata — fixture conventions

Two fixture tiers, one rule each:

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
  so there is one place to look for any fixture's provenance.
- **Immutability:** frozen fixtures are immutable source inputs. Never edit a
  file under `v0.9.2/` — a changed input silently re-bases every test that
  reads it. A new upstream state gets a new versioned tree, never an edit.
- **Copy before mutation:** a test that needs to mutate a repository fixture
  copies it into its own temp directory first (`cp -R`) and mutates the copy.
  Tests never write inside `testdata/`.
- **Expected outputs live with the test:** expected transformed output
  belongs beside the owning test (its package `testdata/`), never inside the
  frozen input tree.

Change 0304 establishes the convention only; the first frozen fixtures arrive
with the changes that need them (0305 configuration, 0306 documents).
