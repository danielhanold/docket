# docket.sh — the one executable docket facade

## Purpose

The single executable entry point an agent or skill invokes instead of calling any other
`scripts/*.sh` helper directly. `docket.sh` is a finite dispatch table of named operations: it
forwards args verbatim to the matching helper and passes the helper's exit code and stderr
through unmasked. There is **no** `run`/`exec`/`shell`/`eval` operation — the facade never
executes caller-supplied shell text, and it never `eval`s positional args. Introduced in change
0068.

The **Subcommand inventory** table below **is the permission inventory**: an operation exists if
and only if it has a row in that table, and (except for the three verbs `preflight`, `env`, and
`bootstrap`) the operation name equals the wrapped helper's basename (`scripts/<op>.sh`). Anything
not in the table is rejected with exit 2, whether or not a same-named script happens to exist on
disk.
`tests/test_docket_facade.sh` derives both the `docket.sh`-declared op set and the
`docket.md`-documented op set by grep and asserts they are set-equal, so this table can never
silently drift out of sync with the dispatcher — see Invariants.

## Usage

```
docket.sh <operation> [args...]
docket.sh -h | --help
```

`-h`/`--help` prints the leading usage comment from `docket.sh` itself. Called with no operation,
or with an operation not in the inventory below, `docket.sh` exits 2 and lists the supported
operations on stderr. All `[args...]` are forwarded to the wrapped helper exactly as received —
no parsing, rewriting, or quoting reinterpretation happens in the facade.

Mock seams (for tests): `SCRIPTS_DIR` (directory the wrapped helpers are resolved from; defaults
to the directory `docket.sh` itself lives in) and `GIT`. `CONFIG_EXPORT_CMD` is honored
transitively by `docket_preflight` (see `scripts/lib/docket-preflight.sh`).

## Subcommand inventory

| Operation | Wraps | Notes |
|---|---|---|
| `preflight` | shared `docket_preflight` (`scripts/lib/docket-preflight.sh`) | Step-0 / mid-run re-sync verb; side effects then prints the `env` block |
| `env` | `docket-config.sh --export --format plain` | read-only config resolver |
| `bootstrap` | `docket-config.sh --bootstrap` | guarded `CREATE_ORPHAN` orphan-`docket` create (fresh repo, once, human-attended); the facade's sole write-path verb, reached only via this verb |
| `docket-status` | `docket-status.sh` | the docket-status orchestrator |
| `board-refresh` | `board-refresh.sh` | gated, atomic `BOARD.md` writer |
| `archive-change` | `archive-change.sh` | move a change to `archive/` |
| `terminal-publish` | `terminal-publish.sh` | publish terminal records onto the integration branch |
| `cleanup-feature-branch` | `cleanup-feature-branch.sh` | delete a merged feature branch + worktree |
| `github-mirror` | `github-mirror.sh` | GitHub Issues/Projects mirror |
| `sync-integration-branch` | `sync-integration-branch.sh` | fast-forward the local integration branch |
| `render-change-links` | `render-change-links.sh` | per-change Artifacts link block (pure renderer) |
| `render-adr-index` | `render-adr-index.sh` | ADR index (pure renderer) |
| `render-learnings-index` | `render-learnings-index.sh` | learnings index (pure renderer) |
| `adr-checks` | `adr-checks.sh` | ADR consistency checks |
| `board-checks` | `board-checks.sh` | board consistency checks |
| `runner-dispatch` | `runner-dispatch.sh` | delegate one agent run to a child harness via a registered runner adapter (change 0079) |

Operation name = wrapped helper basename for every row except the three verbs `preflight`, `env`,
and `bootstrap`, whose `Wraps` column names an implementation or a flagged resolver invocation
rather than a same-named script (there is no `scripts/preflight.sh`, `scripts/env.sh`, or
`scripts/bootstrap.sh`).

## Behavior

**Dispatch.** `docket.sh <op> [args...]` looks `<op>` up in the table above. A match on one of the
14 wrapped ops execs `$SCRIPTS_DIR/<op>.sh "$@"` — args forwarded verbatim, the helper's exit code
and stderr pass through unmasked (the facade uses `exec`, so the wrapped helper's process directly
replaces `docket.sh`'s; there is no wrapper-added exit-code translation or output buffering). A
match on `preflight`, `env`, or `bootstrap` runs the verb-specific logic below instead of execing a
same-named script. No match: `docket.sh` rejects with exit 2 and a `supported operations: preflight
env bootstrap <op>...` line on stderr.

**No escape hatch.** The dispatch table has no `run`, `exec`, `shell`, or `eval` operation arm,
and `docket.sh` never `eval`s `"$@"`, `"$*"`, or any positional/caller-supplied argument. The
`env`/`preflight` verbs print raw `KEY=value` text on stdout for the model to read as literals.
`bootstrap` is the one verb whose stdout is instead the resolver's `%q`-quoted `shell` format (a
pure-routing artifact — see **`bootstrap`** below), but no caller is meant to `eval` or source
that either: the Step-0 flow re-runs `preflight` for the model-facing block.

**`bootstrap`.** `docket.sh bootstrap` execs `docket-config.sh --bootstrap "$@"` — the guarded
`CREATE_ORPHAN` orphan-`docket` create (fresh repo, once, human-attended). Args are forwarded
verbatim (so `--repo-dir` stays usable in fixtures); it is pure routing, not a composite (it does
not sync the worktree or re-run `preflight`). Outside the `CREATE_ORPHAN` cell it performs no
write and exits with the resolver's own status (`env`-like), because failing closed on a
non-`PROCEED` verdict is `preflight`'s job, not this verb's. Unlike `env`/`preflight`, this verb
does **not** pass `--format`, so its stdout is the resolver's *default* `shell`-format output
(`%q`-quoted, eval-able) — not the raw, plain, model-facing `KEY=value` block described below. A
caller that needs that model-facing block re-runs `preflight` afterward — exactly the Step-0
`CREATE_ORPHAN` flow: `bootstrap` (create the orphan) then `preflight` (sync + print the block).

## `env` output

`docket.sh env` execs `docket-config.sh --export --format plain` and prints its resolved
configuration as raw `KEY=value` lines, one per line:

- **No `export ` prefix** and **no shell quoting** (`--format plain` — contrast with
  `--format shell`'s `%q`-quoted, eval-able output used elsewhere). The consumer is the model
  reading literals off stdout, not a shell sourcing the output.
- `BOOTSTRAP` is always present in a successful `env` line set (`PROCEED`, `STOP_MIGRATE`, or
  `CREATE_ORPHAN`).
- An **aborting resolver emits nothing and exits non-zero** — e.g. run outside a git repo, or
  origin unreachable. There is no partial-line output on failure; stdout is empty and the exit
  code is non-zero.
- `METADATA_WORKTREE` is **absolutized** (an absolute path to the metadata worktree, or to the
  repo root itself in main-mode). The other path-valued keys — `CHANGES_DIR`, `ADRS_DIR`,
  `RESULTS_DIR` — stay **repo-relative subpaths**, not absolute, and are composed by the caller
  against whichever tree is the correct root for that key's consumer: the metadata worktree root
  for changes/ADRs, the feature worktree root for results. This is a deliberate, ADR-recorded
  narrowing of the general "all path-valued keys are absolute" rule — the `*_DIR` keys have no
  single correct root, since which tree they resolve against differs by consumer.
- `REPO_ROOT` (change 0075) — the absolute path of the repo's **main worktree**, emitted in the
  `plain` format only (which is what `preflight`/`env` print here; see `docket-config.md` for the
  full field reference and why the `shell` format omits it). It is the cwd-independent literal a
  skill `cd`s to (or targets with `git -C`) before any step that can remove the worktree the agent
  is currently standing in — see `skills/docket-finalize-change/SKILL.md`'s *durable root*.

## `preflight`

The sanctioned Step-0 (and mid-run re-sync) verb. Runs `docket_preflight` — the sync
implementation shared with `docket-status.sh` via `scripts/lib/docket-preflight.sh` — then prints
the same `env` block described above. `docket_preflight` resolves config, enforces the bootstrap
verdict, and (docket-mode) ensures and syncs the metadata worktree, or (main-mode) syncs the
primary tree. **Fails closed**: any bootstrap verdict other than `PROCEED` (`STOP_MIGRATE`,
`CREATE_ORPHAN`, or an unrecognized value) makes `preflight` return non-zero with a stderr
diagnostic instead of printing the `env` block.

## Not exposed

The following scripts are deliberately **not** operations in this facade — invoking their name as
`docket.sh <name>` is rejected with exit 2, and none of them has a row in the inventory table
above:

- `docket-config.sh` — reached only indirectly, through the `env`, `preflight`, and `bootstrap`
  verbs, each of which prepends a fixed resolver flag (`--export` / `--bootstrap`); `docket-config`
  is never itself a routable op.
- `disable-worktree-hooks.sh` — internal to `docket_preflight`.
- `render-board.sh` — internal to `board-refresh.sh` (see `board-refresh.md`); `board-refresh` is
  the exposed op, `render-board` is not.
- The human-initiated tier: `install.sh`, `migrate-to-docket.sh`, `sync-agents.sh`,
  `ensure-docket-env.sh`, `ensure-claude-settings.sh` — one-time or human-run setup scripts, never
  invoked by an agent through this facade.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Success. |
| 2 | Unknown or missing operation — `docket.sh` lists the supported operations on stderr. |
| *(other)* | Propagated verbatim from the wrapped helper's own exit code (for the 14 wrapped ops), or from `docket_preflight`/`docket-config.sh` failure (for `preflight`/`env`/`bootstrap`). |

## Invariants

- **The Subcommand inventory table above is the permission inventory.** An operation is callable
  through `docket.sh` if and only if it has a row here. `tests/test_docket_facade.sh` derives the
  `docket.sh`-side op set by grepping the `WRAPPED_OPS=` line plus the three verbs, derives the
  `docket.md`-side op set by grepping this table's leading `` | `op` `` cells, and asserts the two
  sets are exactly equal — this file and `docket.sh` cannot drift apart without turning the test
  suite red.
- **No `run`/`exec`/`shell`/`eval` escape hatch.** The facade never executes caller-supplied shell
  text and never `eval`s positional/caller args. The sentinel greps `docket.sh` for exactly this.
- **Args forwarded verbatim; exit code and stderr pass through unmasked.** `docket.sh` does not
  reinterpret, re-quote, or filter arguments, output, or exit codes for any wrapped op — it is a
  thin, transparent dispatch layer, not a translating proxy.
- **Operation name = helper basename**, except the `preflight`/`env`/`bootstrap` verbs (documented
  above with a non-`<op>.sh` `Wraps` value).
- **Not-exposed scripts stay not exposed.** None of `docket-config`, `disable-worktree-hooks`,
  `render-board`, `install`, `migrate-to-docket`, `sync-agents`, `ensure-docket-env`, or
  `ensure-claude-settings` may ever gain a row in this table or a token in `docket.sh`'s
  `WRAPPED_OPS` — doing so is exactly what the sentinel's not-exposed assertions catch.
