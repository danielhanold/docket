# runner-dispatch.sh — the runner delegation facade

## Purpose

The runner-neutral entry point of the cross-harness runner delegation framework (change 0079).
Generated shim wrappers make exactly one call to it (via `docket.sh runner-dispatch`); it
validates the request, anchors the repo root, resolves the per-runner config block, and hands
off — foreground — to the named per-runner adapter `scripts/runners/<name>.sh`, which owns
everything child-specific. Adding a future runner touches only the seams: a new adapter script
+ contract in `scripts/runners/`, and a registry token in `sync-agents.sh`'s
`REGISTERED_RUNNERS` (generation-time); the facade itself never changes — it has now absorbed
three adapters (`codex`, `cursor`, `opencode`) without a line of change.

## Usage

```
docket.sh runner-dispatch --runner <name> --agent <agent> [--model <m>] [--effort <e>] [--] [<args…>]
```

- `--runner <name>` (required) — the runner. **Registration is the adapter file's existence**:
  `scripts/runners/<name>.sh` present ⇒ registered. Unknown ⇒ loud nonzero naming the
  registered set (abort-and-report; explicit config is never silently ignored).
- `--agent <agent>` (required) — the built-in docket agent to delegate (e.g. `status`).
- `--model` / `--effort` — forwarded to the adapter verbatim (model is ADR-0015 opaque
  passthrough end-to-end). The generated shim bakes these only from **user-configured** values;
  a value that came from docket's shipped `agents/harness-defaults.yml` is never forwarded. Since
  change 0205 a `runner:`-bearing agent with **no** user-configured model is a generation-time
  error, so the model-less case reaches this facade only on a direct hand invocation.
- `-- <args…>` — forwarded to the adapter as caller task context.

Mock seams: `RUNNERS_DIR` (adapter directory), `GIT` (through `lib/docket-root.sh`).

## Behavior

1. **Validate** — both required flags present; adapter file exists.
2. **Anchor** — `DOCKET_REPO_ROOT` = `docket_main_worktree` (`scripts/lib/docket-root.sh`,
   ADR-0034): the repo's primary checkout, cwd-independent — correct even when invoked from
   `.docket/` or a `.worktrees/<slug>` feature worktree. Not in a repo ⇒ abort.
3. **Resolve `runners.<name>:`** — per **key**, first layer that has the key wins, across
   `<repo>/.docket.local.yml` > `<repo>/.docket.yml` >
   `${XDG_CONFIG_HOME:-$HOME/.config}/docket/config.yml`. Each `key: value` scalar is exported
   as `DOCKET_RUNNER_CFG_<KEY>` (uppercased; `.`/`-` → `_`). The facade knows no runner's key
   names — each adapter defines and defaults its own (see its contract). `runners:` is **not**
   coordination-fenced: it is a machine preference in the same class as `model`/`effort`
   (it writes no shared state), so all three layers are honored.

   A value is read as the **rest of the line** after `<key>:`, with a whitespace-preceded `#`
   comment stripped and surrounding whitespace trimmed — so paths and URLs (`/Users/x/p`,
   `https://host/v1`) survive intact. A value that parses to nothing is **skipped, not fatal**:
   this runs on a live dispatch path, so a cosmetic config typo must not convert into a failed
   dispatch. The key is still claimed for its layer before its value is parsed, so a malformed
   high-precedence value masks the same key in lower layers (precedence is per-key, not
   per-value).
4. **Handoff** — `exec "$DOCKET_BASH_PATH" scripts/runners/<name>.sh --agent <agent> [--model m] [--effort e]
   -- <args…>`, foreground. The facade's stdout/stderr/exit code are the adapter's.

## Exit codes

- `1` — validation failure, unknown runner, or not inside a git repository.
- otherwise — the adapter's exit code (the facade `exec`s it).

## Invariants

- Never runs a child harness itself; all child specifics live in the adapter.
- Never degrades a delegation request to a native run.
- Foreground only — the shim (and any native caller) blocks until the child exits.
- The `runners.<name>:` parse handles simple `key: value` scalars only — by design; a runner
  needing structured config gets it via its own adapter contract, not a richer facade parser.
