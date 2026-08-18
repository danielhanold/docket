# Provenance — `testdata/repositories/v0.9.4/`

- **Source repo:** `danielhanold/docket`
- **Commit:** `5707bb71`
- **Date:** 2026-08-18
- **Redaction:** none

This file covers the `docket-self` fixture only — not a whole tree of new
fixtures. It is **sparse**: it carries only the single fixture below and extends
nothing in `v0.9.2/`. Every frozen reader other than `TestFixtureDocketSelf`
stays on `v0.9.2/`; only that drift guard advances to this tree.

## Contents

- `docket-self/` — a re-cut of the `v0.9.2/docket-self/` fixture for change 0326,
  which contracts this repository's committed `.docket.yml` by turning three
  deferred switches (`terminal_publish`, `finalize.skip_results_only_delta`,
  `build.checkpoint`) explicitly `false` so the Go v1 capability fence permits
  mutation. `repo/.docket.yml` is a byte-exact copy of THIS repository's
  committed `.docket.yml` at the commit above (the contracted file); the global
  layer beside it (`xdg/docket/config.yml`, which still requests auto-capture)
  is carried over from `v0.9.2/docket-self/` byte-for-byte and is not this
  change's to touch. No redaction.
