# docket-config.sh — deterministic resolver for docket's startup config + bootstrap guard

## Purpose

Resolves `origin/HEAD`, reads `.docket.yml` authoritatively, applies all defaults, and
evaluates the bootstrap 2×2 guard. Every docket skill runs this once at Step 0, consuming
its output with `eval "$(… --export)"`. Implemented verbatim from ADR-0002 and the
convention's Configuration / Bootstrap guard sections; no skill re-derives this logic.

## Usage

```bash
# Read-only (default): emit resolved KEY=value lines and consume them
eval "$("${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket-config.sh --export)"

# Opt-in write: also create the empty orphan when BOOTSTRAP=CREATE_ORPHAN (fresh repo)
eval "$("${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/docket-config.sh --export --bootstrap)"

# Operate on a specific repo (test/mock seam)
docket-config.sh --repo-dir /path/to/repo --export
```

**Flags:**
- `--export` — emit resolved `KEY=value` lines to stdout (default mode; always pass it)
- `--format plain|shell` — output presentation (default `shell`). `shell` is the historical
  `%q`-quoted, eval-able contract (`eval "$(… --export)"`) — byte-identical, unchanged. `plain`
  is raw `KEY=value`, no quoting, no `export ` prefix, with `METADATA_WORKTREE` absolutized —
  for the docket facade's model-facing `env`/`preflight` output (never `eval`'d). The abort
  posture is identical in both formats: an aborting run emits nothing on stdout and exits
  non-zero.
- `--bootstrap` — additionally perform the `CREATE_ORPHAN` write when the verdict warrants
  it; a no-op in every other bootstrap cell
- `--repo-dir DIR` — repo to resolve against. **Default (change 0075): the MAIN worktree of
  the repo containing CWD**, not CWD itself, so a call from `.docket/`, `.worktrees/<slug>`,
  or a subdirectory resolves the same primary root. Falls back to CWD when CWD is not inside
  a git repo (the existing not-a-repo error then fires). Still overrides verbatim when passed
  explicitly (the test/mock seam).
- `-h`, `--help` — print the script's usage header

**Mock seam:** `GIT="${GIT:-git}"` — override `GIT` in tests to inject a stub.

## Behavior

### §1: the repo anchor (change 0075)

Before Stage 1 runs, the script resolves `$REPO_DIR` (unless `--repo-dir` was passed, which
overrides verbatim): it calls `docket_main_worktree` (`scripts/lib/docket-root.sh`) to find the
**main worktree** of the repo containing CWD, and uses that as `$REPO_DIR` for the rest of the
run — never CWD itself. When CWD is not inside a git repo, `docket_main_worktree` returns empty
and `$REPO_DIR` falls back to `.`, so the existing `git rev-parse --is-inside-work-tree` gate in
Stage 1 still fires its standard "not a git repo" error unchanged.

**Behavior change (pinned by change 0075):** invoked from `<repo>/sub/` (a plain
subdirectory), the resolver now reads `<repo>/.docket.local.yml` (Stage 2b') and targets
`<repo>/.docket` as the metadata worktree — previously it would have read
`<repo>/sub/.docket.local.yml` and targeted `<repo>/sub/.docket`, since `$REPO_DIR` defaulted to
CWD (`.`). The same applies to a call from the `.docket/` metadata worktree itself or a
`.worktrees/<slug>` feature worktree: every caller in the worktree set resolves the SAME primary
root. `--bootstrap`'s `.gitignore`-seeding write (below) is anchored the same way — it now always
seeds `<repo>/.gitignore`, never `<sub>/.gitignore` or a linked worktree's own `.gitignore`.

### Stage 1: repair `origin/HEAD` and resolve the default branch

1. Assert the repo is a git working tree (`git rev-parse --is-inside-work-tree`); exit
   non-zero if not.
2. `git fetch --quiet origin` — abort with a diagnostic on non-zero return code
   (unreachable origin).
3. `git remote set-head origin -a` — abort with a diagnostic on non-zero return code
   (unresolvable `origin/HEAD`). The abort keys on the `set-head`/fetch return code,
   **never on `git show`** — a cached `origin/HEAD` lets `git show` succeed with stale bytes
   even after the remote is unreachable.
4. `git symbolic-ref --quiet --short refs/remotes/origin/HEAD` — strip the `origin/` prefix
   to yield `DEFAULT_BRANCH`.

### Stage 2: read `.docket.yml` authoritatively

`.docket.yml` is read via `git show origin/HEAD:.docket.yml` (not from the working tree).
This is authoritative because the file lives on the default branch, and only the default
branch's primary checkout can be trusted to have the working-tree copy in sync. The output
is written to a temp file and cleaned up on exit.

**File-absent vs. ref-unresolvable distinction:**
- If `origin/HEAD` resolves but `.docket.yml` is genuinely absent → `git show` exits
  non-zero; the temp file is left empty; all defaults apply. This is not an error.
- If `origin/HEAD` is unresolvable or `origin` is unreachable → the script already aborted
  in Stage 1 (keyed on fetch/set-head return codes). `git show` is never the abort signal.

**YAML reader:** a minimal flat scalar reader (`yaml_get`) handles `key: value` lines.
Inline `#` comments are stripped; leading/trailing whitespace and surrounding quotes are
removed. The key is ERE-escaped before use. Nested keys (`finalize.gate`,
`finalize.test_command`) are read by their unique leaf-key name (`gate`, `test_command`).
A value may not contain a literal `#` — it is treated as the start of an inline comment.

**Resolved values and defaults:**

| `.docket.yml` key | Default | Global-able | Notes |
|---|---|---|---|
| `metadata_branch` | `docket` | no (fenced) | must be `docket` or `main`; anything else aborts |
| `integration_branch` | `auto` | no (fenced) | `auto` or empty → `DEFAULT_BRANCH`; explicit value used verbatim |
| `changes_dir` | `docs/changes` | no (fenced) | |
| `adrs_dir` | `docs/adrs` | no (fenced) | |
| `results_dir` | `docs/results` | no (fenced) | |
| `gate` (finalize) | `local` | yes | read from `finalize.gate` leaf key; resolves repo-local > repo-committed > global |
| `test_command` (finalize) | `` (empty) | yes | read from `finalize.test_command` leaf key; resolves repo-local > repo-committed > global |
| `board_surfaces` | `inline` | yes, minus `github` | YAML list `[a, b]` stripped of brackets/commas; **`[]` → the reserved token `none`** (change 0071 — an empty value is NEVER emitted; empty means "unresolved", a wiring bug); a `github` token arriving from either machine-scoped layer (repo-local or global) is dropped (Stage 2c), and a list left empty by that drop also resolves to `none` |
| `auto_groom` | `false` | yes | resolves repo-local > repo-committed > global |
| `terminal_publish` | `false` | no (fenced) | `true`/`false`; the default `false` makes `terminal-publish.sh` a no-op for BOTH shapes — archived change files, specs, and ADRs stay on the metadata branch. `true` opts in to the direct-commit publish onto the integration branch. Anything else aborts |

`github_project` and `agents:`/`agent_harnesses` are per-repo-only / not read by this script (see
Stage 2b/2b'/2c below and `sync-agents.sh`'s own contract, respectively) — every other key above
not marked "Global-able" is per-repo-only.

**`skills:` (change 0049).** Reads the optional nested `skills:` block and emits
`SKILL_BRAINSTORM`, `SKILL_PLAN`, `SKILL_BUILD`, `SKILL_REVIEW`, `SKILL_FINISH`. Each leaf
resolves **repo-local > repo-committed > global > superpowers default** — the repo-local
`.docket.local.yml`'s `skills:` block wins if the leaf is set there, else the per-repo
`.docket.yml`'s `skills:` block, else the global `config.yml`'s `skills:` block, else the
built-in superpowers skill (`superpowers:brainstorming`, `superpowers:writing-plans`,
`superpowers:subagent-driven-development`, `superpowers:requesting-code-review`,
`superpowers:finishing-a-development-branch`); a set leaf is passed through verbatim (or the
sentinel `auto`). Leaves are read *within the block* (never as bare top-level keys). An unknown
role key under `skills:`, in any of the three layers, is warned on stderr and ignored — never
fatal.

### Stage 2b: global config layer (change 0050)

`${XDG_CONFIG_HOME:-$HOME/.config}/docket/config.yml` — read from the **local filesystem**
(per-machine by definition; no authoritative-ref concern). Full `.docket.yml` schema,
resolved per-key: repo-local > repo-committed > global > built-in. Map-valued `skills:` merges
field-by-field. `agents:` and `agent_harnesses` are **not read here** — `sync-agents.sh` is their
reader.

**Guards (Stage 2c), all warn-and-ignore, never fatal:**
- `~/.config/docket/.docket.yml` present → warned ("global config is config.yml"), never read.
- `config.yml` exists but is not a readable regular file → warned; global layer ignored.
- **Coordination-key fence:** `metadata_branch`, `integration_branch`, `changes_dir`,
  `adrs_dir`, `results_dir`, `github_project`, `terminal_publish` set in the global layer OR in
  `.docket.local.yml` → each warned "per-repo-only" (naming which file) and ignored. (Block-style
  `github_project:` with an empty value line is not detected — the fence reads the scalar value;
  nothing reads a global/local `github_project` regardless.)
- `board_surfaces` **from either machine-scoped layer** (`.docket.local.yml` or the global
  `config.yml`) drops a `github` token with a warning (external objects stay repo opt-in); a
  per-repo `github` is honored as before. The machine-layer fallback happens on the RAW value, so
  a machine-scoped `[]` (disable) is distinguishable from unset (default `[inline]`).

### Stage 2b': machine-local layer (change 0051)

`<repo>/.docket.local.yml` — machine-**and**-repo-scoped overrides for exactly the global-able
key set. Read from the **working tree** (not `origin/HEAD`) since the file is inherently
per-machine and typically gitignored; the origin/HEAD-authoritative read applies only to the
committed `.docket.yml`. Because the file is machine-scoped, the ADR-0019 coordination-key fence
applies to it verbatim, same as the global layer.

Precedence per field is the four-layer chain (the `.env` pattern): **repo-local >
repo-committed > global > built-in.** This applies uniformly to `finalize.gate`,
`finalize.test_command`, `auto_groom`, `board_surfaces`, and each `skills:` leaf.

**Guards, both warn-and-ignore, never fatal:**
- `.docket.local.yml` exists but is not a readable regular file (e.g. a directory) → warned,
  naming the file, and the local layer is ignored for the rest of the run (falls through to
  repo-committed/global/built-in as if the file were absent).
- Any fenced key set in `.docket.local.yml` → warned "per-repo-only", naming both the key and
  `.docket.local.yml`, and ignored (same posture as a fenced global key).

**`metadata_branch` drives mode and worktree:**
- `docket` → `DOCKET_MODE=docket`, `METADATA_WORKTREE=.docket`
- `main` → `DOCKET_MODE=main`, `METADATA_WORKTREE=.`
- anything else → non-zero exit with a diagnostic naming `metadata_branch`

**`integration_branch` resolution:** if the value is empty or `auto`, the resolved
`DEFAULT_BRANCH` is used (fallback `main` if `DEFAULT_BRANCH` is somehow empty after
set-head). An explicit value (e.g. `develop`) is used verbatim.

### Stage 3: bootstrap 2×2 evaluation (docket-mode only)

Skipped when `DOCKET_MODE=main`; `BOOTSTRAP` is left as `PROCEED`.

When `DOCKET_MODE=docket`, two boolean flags are probed:

- **`DOCKET`** — the `docket` branch exists on origin OR locally:
  `git rev-parse --verify --quiet refs/remotes/origin/docket` OR
  `git rev-parse --verify --quiet refs/heads/docket`.

- **`LIVE`** — the live planning surface is still on the integration branch:
  `git ls-tree origin/<INTEGRATION_BRANCH> -- <CHANGES_DIR>/active <CHANGES_DIR>/README.md
  <CHANGES_DIR>/BOARD.md` yields non-empty output. Only this pruned surface is probed —
  `archive/`, `<ADRS_DIR>/`, and pre-migration specs are excluded. If `ls-tree` itself exits
  non-zero (the ref is absent/unreadable), the script aborts with a hard config error
  (not `¬LIVE`).

**2×2 verdict → `BOOTSTRAP`:**

| | `LIVE` | `¬LIVE` |
|---|---|---|
| **`¬DOCKET`** | `STOP_MIGRATE` | `CREATE_ORPHAN` |
| **`DOCKET`** | `STOP_MIGRATE` | `PROCEED` |

### `--bootstrap` orphan-create write path

Guarded strictly to the `¬DOCKET ∧ ¬LIVE` cell. When `--bootstrap` is passed and
`BOOTSTRAP=CREATE_ORPHAN`, the script:

1. Creates an empty tree object: `git mktree < /dev/null`.
2. Creates a root commit: `git commit-tree <tree> -m 'docket: initialize empty orphan
   metadata branch'` (requires `user.name`/`user.email` to be set; aborts if not).
3. Pushes the commit directly to `origin/refs/heads/docket` (no local branch created).
4. Fetches `origin docket` to populate `refs/remotes/origin/docket`.
5. Seeds the managed `# docket` `.gitignore` block in the primary tree (`ensure_docket_gitignore_block`,
   change 0057) — closing the fresh-repo gap where a repo bootstrapped straight into docket-mode
   would otherwise never get the block from `migrate-to-docket.sh`. Prints a loud
   `COMMIT THIS` notice to stderr; it does **not** auto-commit — bootstrap runs inside a skill's
   startup, and committing to the user's integration branch from a config script crosses a
   write-scope line docket holds. The `.gitignore` is left modified but unstaged.
6. Re-reports `BOOTSTRAP=PROCEED` — the repo is now migrated; the caller may proceed.

In every other cell (`STOP_MIGRATE`, `PROCEED`, or `main`-mode), `--bootstrap` is a no-op. `--export`
without `--bootstrap` remains strictly read-only in every cell — it never touches `.gitignore` or
any other file; only the `--bootstrap` orphan-create write path (above) writes.

### Emit

All resolved values are printed as `KEY=value` lines to stdout in this order (see `--format`
above: `shell`, the default, shell-quotes each value via `printf '%s=%q'`; `plain` prints the
raw value via `printf '%s=%s'` with `METADATA_WORKTREE` absolutized):

```
DOCKET_MODE
DEFAULT_BRANCH
METADATA_BRANCH
INTEGRATION_BRANCH
METADATA_WORKTREE
REPO_ROOT            (plain format only — see below)
CHANGES_DIR
ADRS_DIR
RESULTS_DIR
FINALIZE_GATE
FINALIZE_TEST_COMMAND
BOARD_SURFACES
AUTO_GROOM
TERMINAL_PUBLISH
SKILL_BRAINSTORM
SKILL_PLAN
SKILL_BUILD
SKILL_REVIEW
SKILL_FINISH
BOOTSTRAP
```

19 lines in `shell` format (unchanged); 20 in `plain` format, with `REPO_ROOT` inserted directly
after `METADATA_WORKTREE`. The last line is always `BOOTSTRAP=…`.

**`REPO_ROOT` (change 0075) — plain format only.** The absolute path of the main worktree (the
same value `$REPO_DIR` resolved to, per the §1 repo anchor above) — the literal `docket.sh
preflight`/`env` print and a skill reads for a cwd-independent `cd`. Deliberately **absent from
the `shell` format**: `scripts/ensure-claude-settings.sh:24` sets its own `REPO_ROOT` variable
and `eval`s the shell export at `:33`, reading it back at `:38` and `:74` — emitting a
shell-format `REPO_ROOT` here would silently capture that unrelated variable name and corrupt
its behavior. `plain` output is model-facing and never `eval`'d, so the same collision risk does
not apply there.

The emitted `KEY=value` set and order are
**unchanged** by the machine-local layer (change 0051) — `.docket.local.yml` only adds a
higher-precedence input to the values already resolved above; no new KEY is introduced.

## Exit codes

The script is **fail-closed**: any hard error exits non-zero with a diagnostic to stderr and
emits no `KEY=value` output.

| Condition | Exit |
|---|---|
| Normal completion | 0 |
| `origin` unreachable (`git fetch` failed) | 1 |
| `origin/HEAD` unresolvable (`git remote set-head` failed) | 1 |
| `origin/HEAD` still empty after set-head | 1 |
| `integration_branch` ref absent/unreadable (`ls-tree` non-zero) | 1 |
| `metadata_branch` is neither `docket` nor `main` | 1 |
| `terminal_publish` is neither `true` nor `false` | 1 |
| `mktree`/`commit-tree`/push failed during orphan create | 1 |
| `--repo-dir` missing its argument | 2 |
| Unknown argument | 2 |

## Invariants

- **Read-only by default.** The only writes (`create_orphan`, and seeding the managed
  `.gitignore` block) are opt-in via `--bootstrap` and guarded to `¬DOCKET ∧ ¬LIVE`. Fetch
  and `remote set-head` are the sole side effects of a plain `--export` call.
- **Abort keys on fetch/set-head return code, never on `git show`.** A cached `origin/HEAD`
  would let `git show` succeed with stale bytes even after the remote is destroyed. All
  abort decisions precede the `git show` call.
- **File absent ≠ ref unresolvable.** A missing `.docket.yml` on a reachable origin yields
  defaults; an unreachable origin aborts. These two states are never conflated.
- **No local `docket` branch is created by `--bootstrap`.** The orphan commit is pushed
  directly to `refs/heads/docket` on origin, then fetched to populate
  `refs/remotes/origin/docket`. Only the remote ref exists after the write.
- **`BOOTSTRAP=PROCEED` after a successful `--bootstrap` run.** The script re-reports the
  post-write state, so the caller's `eval` sees `PROCEED` without a second invocation.
- **`main`-mode skips the bootstrap guard entirely.** `DOCKET`/`LIVE` are not evaluated;
  `BOOTSTRAP` is always `PROCEED` in main-mode.
- **19 `KEY=value` lines always emitted in the same order in `shell` format (20 in `plain`,
  `REPO_ROOT` inserted after `METADATA_WORKTREE` — change 0075).** Skills may rely on the order
  for pipe consumers, but should use the variable names (via `eval`) for correctness.
- **The global layer never aborts a run.** Every global-file problem (misplaced, malformed,
  fenced key) is a stderr warning; exit codes are unaffected.
- **The machine-local layer never aborts a run either.** A malformed `.docket.local.yml` or a
  fenced key set within it is a stderr warning (never fatal); the run falls through to the next
  layer in the chain.
- **`BOARD_SURFACES` is never emitted empty** (change 0071). The deliberate off-state is the
  positive token `none`; an empty value is reserved for "unresolved" and is a wiring bug every
  consumer rejects with exit 2.
