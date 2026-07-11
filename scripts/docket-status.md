# scripts/docket-status.sh

## Purpose

Deterministic orchestrator that collapses the docket-status pipeline (config eval, bootstrap
gate, worktree sync, board render, sweep, health checks) into a single invocation. Change 0058.

## Usage

```
docket-status.sh [--board-only] [--repo OWNER/REPO] [--project OWNER/NUMBER]
                  [--auto-create-project] [--project-owner OWNER]
```

## Behavior

Evaluates `docket-config.sh --export` and `eval`s the result, then gates on `BOOTSTRAP`:
`PROCEED` continues, `STOP_MIGRATE`/`CREATE_ORPHAN` exit non-zero with a remedy on stderr.
Later tasks wire the worktree sync, board render, sweep, and health-check passes.

## Exit codes

- `0` — pass completed.
- non-zero — hard error only (config/bootstrap failure, unknown argument).

## Invariants

- Stderr carries diagnostics; stdout stays machine-parseable.
- Never truncates a committed file on a failed render.
