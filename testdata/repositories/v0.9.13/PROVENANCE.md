# Provenance — `testdata/repositories/v0.9.13/`

This one file covers the whole `v0.9.13/` tree, per `testdata/README.md`. Each fixture added to the
tree extends it with one entry below.

## Contents

- `deferred-active/` — cut from `v0.9.2/deferred-active`, 2026-10-11, by the change that adds the
  per-agent `cache_ttl` pin. Hand-written configuration, no redaction. The ONLY difference from
  the v0.9.2 copy is one added line, `cache_ttl: 1h`, under `agents.claude.adr` in
  `repo/.docket.yml`: the new `agents.*.*.cache_ttl` registry row is a repo-settable agent pin,
  so a repository layer requesting it is one more preflight blocker, and the fixture must request
  every such capability at once. Everything else is carried verbatim. Older versioned fixture
  trees (v0.9.2–v0.9.12) are immutable inputs and are never edited in place.
