# v0.9.11 agent-defaults sidecar

- **Source repo:** `danielhanold/docket`
- **Commit:** aa0cdb5b38f255ac63ae2411ec02affc1a7ed14b (the tree equals `agents/harness-defaults.yml` at that commit)
- **Date:** 2026-10-04
- **Redaction:** none

Source: docket's own `agents/harness-defaults.yml` as of this change (align the human-facing docs
and example config with the docket binary), 2026-10-04.

Cut from `v0.9.9/agents-harness-defaults.yml` by this change. The ONLY file in this tree is
`agents-harness-defaults.yml`, a byte-exact copy of `agents/harness-defaults.yml` at this change.
It differs from the v0.9.9 sidecar in comments only: the header now describes the compiled
built-in agent table and global-only overrides, and the references to the deleted Bash tooling
(the shell validator, the wrapper-sync script, the bare-scalar reader rule) and the change
citations in the codex and opencode block comments are gone. No key or value changed, so the Go
built-in agent table in `internal/config/defaults.go` is unchanged. Only the agent-defaults parity
oracle's `sidecarPath` advances to this tree; every other frozen reader stays where it was. Older
versioned fixture trees (v0.9.2–v0.9.10) are immutable inputs and are never edited in place.
