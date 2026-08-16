# Provenance — `testdata/repositories/v0.9.3/`

- **Source repo:** `danielhanold/docket`
- **Commit:** `a4d72613`
- **Date:** 2026-08-15
- **Redaction:** none

This file covers the agent-defaults sidecar only — `agents-harness-defaults.yml`
— not the whole `v0.9.3/` tree; the frozen `status` semantic corpus that also
lives in this tree keeps its own provenance at `status-corpus/PROVENANCE.md`.
The sidecar is **sparse**: it carries only the single fixture below and extends
nothing in `v0.9.2/`. Every frozen reader other than the agent-defaults parity oracle
stays on `v0.9.2/`; only that oracle's `sidecarPath` advances to this tree.

## Contents

- `agents-harness-defaults.yml` — byte-exact copy of `agents/harness-defaults.yml`
  at the commit above (change 0324, which registers the seventeenth shipped
  agent `docket-plan-writer`); no redaction. This re-cut is what pins the Go
  built-in agent table in `internal/config/defaults.go` after the roster grew
  from sixteen to seventeen agents.
