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
- `agents-harness-defaults.yml` — cut from `v0.9.11/agents-harness-defaults.yml`, 2026-10-11, by
  the change that adds the per-agent `cache_ttl` pin. Source repo `danielhanold/docket`; commit:
  the commit that adds this file (it equals `agents/harness-defaults.yml` there). No redaction. A
  byte-exact copy of docket's own `agents/harness-defaults.yml` at that change. It differs from
  the v0.9.11 sidecar in one value and its comments: the claude `implement-next` row gains
  `cache_ttl: 1h` with a comment explaining why, and the header documents the optional
  `cache_ttl` field. Every model and effort is unchanged. The Go built-in table in
  `internal/config/defaults.go` gains the matching `builtinAgentCacheTTL` entry, and only the
  agent-defaults parity oracle's `sidecarPath` advances to this tree.
