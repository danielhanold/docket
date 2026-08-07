# runner-dispatch.sh — the runner delegation facade

## Purpose

The runner-neutral entry point of the cross-harness runner delegation framework (change 0079).
Generated shim wrappers make exactly one call to it (via `docket.sh runner-dispatch`); it
validates the request, anchors the repo root, resolves the per-runner config block, and **calls
and returns** — foreground — to the named per-runner adapter `scripts/runners/<name>.sh`, which
owns everything child-specific. Adding a future runner touches only the seams: a new adapter
script + contract in `scripts/runners/`, and a registry token in `sync-agents.sh`'s
`REGISTERED_RUNNERS` (generation-time) — it absorbed three adapters (`codex`, `cursor`,
`opencode`) without a line of change.

Change 0237 replaced the original `exec` with a call-and-return so the facade regains control
after the adapter, and hung a **run gate** on that seam: for an `implement-next` delegation only,
the facade verifies in git that the delegated run actually reached its PR.

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

Mock seams: `RUNNERS_DIR` (adapter directory), `GIT` (through `lib/docket-root.sh`), and — for the
run gate (change 0237) — `VERIFY_RUN` (the disposition reader, default `scripts/verify-run.sh`) and
`DOCKET_FACADE` (the facade used for the post-return metadata re-sync, default `scripts/docket.sh`).

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
4. **Handoff** — `"$DOCKET_BASH_PATH" scripts/runners/<name>.sh --agent <agent> [--model m]
   [--effort e] -- <args…>`, foreground, **call-and-return** (change 0237 — no longer `exec`).
   The adapter's stdout/stderr pass through and its exit code is propagated **verbatim** on every
   path where the run gate takes no action.
5. **Run gate (change 0237)** — engages **only** for `--agent implement-next`. Before the handoff
   the facade records the set of `in-progress` change ids (`verify-run --in-progress-ids`); after
   the handoff it re-syncs the metadata worktree and re-reads the set. Any id **not** in the
   before-set is this run's claim, and each is checked with `verify-run <id>`:

   - `run-complete` / `run-halted` / `run-unclaimed` → nothing; exit the adapter's code.
   - `run-incomplete` → **one** bounded re-dispatch of the same adapter, with the change id and the
     unmet conjuncts as task context. If the second verdict is still `run-incomplete`, the facade
     aborts loudly with exit `1`, naming the change and the still-unmet conjuncts.

   `run-halted` never re-dispatches — a halt means a human is needed. A `build-*` delegation leaves
   its change `in-progress` by design, which is why the agent gate is load-bearing rather than an
   optimization. An unrecognised agent is a no-op, never a guess. A snapshot that cannot be read
   **disables the gate with a warning** — it never converts a healthy dispatch into a failure.

   The re-dispatched run's own exit code is not propagated: the gate's verdict is read from git,
   not from the retry's status, so the outcome is either `$rc` (the first adapter's code) or the
   two-strikes `1`.

**Signals.** The facade installs no traps. A **group**-directed signal (a terminal's Ctrl-C) is
unchanged by the loss of `exec` — the adapter is in the same process group and receives it
directly. A **pid**-directed signal to the facade now behaves differently, because the facade is a
separate process: `INT` is deferred while it waits on its child and then discarded, and `TERM`
kills the facade and orphans the adapter. Nothing in docket signals the facade by pid; forwarding
traps are a deliberate deferral, not an oversight.

## Exit codes

- `1` — validation failure, unknown runner, not inside a git repository, or a rejected
  `--worktree` (missing for a `build-*` agent, not a directory, or not a worktree of this repo).
- `1` — the run gate's two-strikes abort: a delegated `implement-next` run was still
  `run-incomplete` after one re-dispatch. The change stays `in-progress` with its claim intact.
- otherwise — the adapter's exit code, propagated verbatim.

## Invariants

- The anchor is **never** resolved from the caller's CWD; absent `--worktree` it is the main
  worktree (ADR-0034 unamended). A relative `--worktree` joins to the main worktree, so the
  argument inherits that cwd-independence rather than reintroducing the hazard.
- Never runs a child harness itself; all child specifics live in the adapter.
- The adapter's exit code is propagated verbatim whenever the run gate takes no action; the
  two-strikes abort is the only new non-zero, and only on a path that was previously silent.
- The run gate is scoped to `--agent implement-next` and never writes docket state — it acts only
  by running an agent. It re-dispatches an unfinished change **at most once**.
- Never degrades a delegation request to a native run.
- Foreground only — the shim (and any native caller) blocks until the child exits.
- The `runners.<name>:` parse handles simple `key: value` scalars only — by design; a runner
  needing structured config gets it via its own adapter contract, not a richer facade parser.
