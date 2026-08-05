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
docket.sh runner-dispatch --runner <name> --agent <agent> [--model <m>] [--effort <e>] [--worktree <path>] [--] [<args…>]
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
- `--worktree <path>` (optional) — the run anchor. Resolved through `docket_anchor_path`, so an
  absolute path passes through and a **relative** one joins to the main worktree (never to the
  caller's cwd). Absent ⇒ the main worktree, byte-identical to pre-0206 behavior. **Required for
  `build-*` agents**: the `docket-build-task` contract requires a build worker to run inside its
  feature worktree, on its branch, so a `build-*` delegation without it is a loud abort rather
  than a silent run in the primary checkout on the integration branch.
- `-- <args…>` — forwarded to the adapter as caller task context.

Mock seams: `RUNNERS_DIR` (adapter directory), `GIT` (through `lib/docket-root.sh`).

## Behavior

1. **Validate** — both required flags present; adapter file exists.
2. **Anchor** — `DOCKET_REPO_ROOT` = `docket_anchor_path "${worktree:-.}"`
   (`scripts/lib/docket-root.sh`, ADR-0034). With no `--worktree` that is the repo's primary
   checkout, cwd-independent — correct even when invoked from `.docket/` or a `.worktrees/<slug>`
   feature worktree. With `--worktree` it is the named tree, and a relative value joins to the
   main worktree so it too resolves identically from any cwd. Not in a repo ⇒ abort. Three loud
   gates follow, all before the config read so an anchor failure never depends on config parsing:
   `--worktree` is **required** for a `build-*` agent; the resolved anchor must be a **directory**;
   and it must be a **worktree of this repository** (compared via `docket_main_worktree "$anchor"`,
   whose empty result for a non-repo path fails the same comparison).
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

- `1` — validation failure, unknown runner, not inside a git repository, or a rejected
  `--worktree` (missing for a `build-*` agent, not a directory, or not a worktree of this repo).
- otherwise — the adapter's exit code (the facade `exec`s it).

## Invariants

- The anchor is **never** resolved from the caller's CWD; absent `--worktree` it is the main
  worktree (ADR-0034 unamended). A relative `--worktree` joins to the main worktree, so the
  argument inherits that cwd-independence rather than reintroducing the hazard.
- Never runs a child harness itself; all child specifics live in the adapter.
- Never degrades a delegation request to a native run.
- Foreground only — the shim (and any native caller) blocks until the child exits.
- The `runners.<name>:` parse handles simple `key: value` scalars only — by design; a runner
  needing structured config gets it via its own adapter contract, not a richer facade parser.
