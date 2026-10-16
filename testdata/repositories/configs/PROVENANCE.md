# Provenance — `testdata/repositories/configs/`

- **Source repo:** `danielhanold/docket`
- **Commit:** `096c48de`
- **Date:** 2026-08-13
- **Redaction:** none

This one file covers everything in `configs/`, per `testdata/README.md`. A sample added later
adds one entry below.

These configurations were hand-written at the commit above to look like configurations written
for docket at the time. None was captured from a real repository. Each holds a `repo/` directory
(the repository handed to `FSOptions.RepoDir`) and, where the sample needs a global layer, an
`xdg/docket/config.yml`. Configuration files only.

## Contents

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
- `deferred-pairs/` — every deferred setting that has an inactive spelling,
  declared inactive.
- `deferred-active/` — the deferred capabilities a repository could request at that commit, all
  switched on; the command-line tests use it as a configuration docket refuses to write under.
  The complete, current list is written inline in `TestFixtureDeferredActive`.
- `invalid/<reason>/` — nine single-fault documents, one per rejection reason:
  `malformed`, `duplicate-key`, `alias-merge`, `multi-doc`, `wrong-type`,
  `bad-enum`, `scalar-auto-capture`, `unknown-key`, `model-typo`.
