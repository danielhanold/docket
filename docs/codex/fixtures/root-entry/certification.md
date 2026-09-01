# Coordinator root-entry certification

Date: 2026-09-01  
Change: 0393  
Host path: VS Code Codex  
Codex CLI: 0.152.0

## Production-path probe

The build under test was entered through its public command, with the real registered
`docket-implement-next` role contract:

```text
go run ./cmd/docket agent enter \
  --role docket-implement-next \
  --request .root-entry-probe-request.txt \
  --cwd <feature-worktree> \
  --approval-policy never \
  --sandbox danger-full-access
```

The request constrained the run to a capability probe and required the root coordinator to dispatch
the registered `docket-brainstorm-consultant` leaf. The leaf returned
`CONSULTANT_SENTINEL=393-ROOT-ENTRY-7F31`; the root turn consumed it and returned
`ROOT_SENTINEL=393-ROOT-ENTRY-7F31`. The command exited 0. The temporary request file was removed.

This proves the public entry operation creates a coordinator-capable root thread, preserves its
request, reaches a real registered child through native named-agent dispatch, waits through child
completion, and returns the root role's terminal message. Unit protocol tests separately pin the
thread/start and turn/start mapping and prove that an unrelated child completion cannot terminate
the root wait.

## Exact plan-writer composition

The same public command then ran the unmodified `docket-implement-next` charter against a fully
isolated Git repository and local bare origin containing exactly one critical, build-ready change.
The normal workflow claimed and reconciled that change, created its feature worktree, and dispatched
the real registered `docket-plan-writer` role.

The child produced commit `aa8da1c531cb515c00992cd84aac929876825745`. Verification found:

- a clean one-file delta containing only
  `docs/superpowers/plans/2026-09-01-write-the-root-entry-plan-writer-sentinel.md`;
- the exact commit trailer `Docket-Plan-Path: docs/superpowers/plans/2026-09-01-write-the-root-entry-plan-writer-sentinel.md`;
- ordered, balanced `docket:backlink` markers pointing to the isolated change; and
- metadata commit `193abe28` attaching that exact plan, proving the root consumed and verified the
  child's `PLAN_PATH` receipt before continuing.

The isolated root was interrupted after attachment, before building the disposable sentinel. Its
temporary repository and origin carried every workflow mutation; no production backlog state was
used for this certification.

## Transport selection

The managed app-server proxy control socket was also spiked and failed after initialization with a
broken pipe. Direct `codex app-server --stdio` completed the same protocol. Production therefore
uses the direct, closed argv `codex app-server --stdio` and has no proxy or `codex exec` fallback.
