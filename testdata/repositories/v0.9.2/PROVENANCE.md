# Provenance — `testdata/repositories/v0.9.2/`

- **Source repo:** `danielhanold/docket`
- **Commit:** `096c48de`
- **Date:** 2026-08-13
- **Redaction:** none

This one file covers the whole `v0.9.2/` tree, per `testdata/README.md`. Each
fixture subdirectory added later extends it with one line below.

## Contents

- `agents-harness-defaults.yml` — byte-exact copy of `agents/harness-defaults.yml`
  (the final v0.9.2 shipped agent defaults) plus the configuration-only fixtures
  of change 0305; no redaction.

The repository fixtures below were authored for change 0305 against the v0.9.2
configuration surface — hand-written, not captured from a live repository, with
the one exception noted. Each holds a `repo/` directory (the repository handed
to `FSOptions.RepoDir`) and, where the fixture needs a global layer, an
`xdg/docket/config.yml`. Configuration files only; no redaction anywhere.

- `sparse-defaults/` — a repository with no configuration files at all (the
  `repo/.gitkeep` placeholder exists only so git tracks the empty directory).
- `example-activated/` — every supported setting declared explicitly at its
  shipped default.
- `four-layer-collision/` — the same leaves declared in the global, repository,
  and repository-local layers, with scalar, list, nested, and agent collisions.
- `mode-main-custom-paths/` — main-mode repository with an explicit integration
  branch and all three docket directories relocated.
- `mode-docket/` — docket-mode repository leaving `integration_branch: auto`.
- `fenced-machine-keys/` — every repo-fenced setting declared from a machine
  layer, split between the global and repository-local files.
- `docket-self/` — `repo/.docket.yml` is a byte-exact copy of THIS repository's
  committed `.docket.yml` at commit `096c48de`; the global layer beside it is
  authored for 0305 and requests auto-capture.
- `deferred-pairs/` — every deferred setting that has an inactive spelling,
  declared inactive.
- `deferred-active/` — every repo-settable deferred capability requested at once.
- `invalid/<reason>/` — nine single-fault documents, one per rejection reason:
  `malformed`, `duplicate-key`, `alias-merge`, `multi-doc`, `wrong-type`,
  `bad-enum`, `scalar-auto-capture`, `unknown-key`, `model-typo`.
