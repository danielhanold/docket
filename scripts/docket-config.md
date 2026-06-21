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
- `--bootstrap` — additionally perform the `CREATE_ORPHAN` write when the verdict warrants
  it; a no-op in every other bootstrap cell
- `--repo-dir DIR` — target a specific git repo directory (used by test fixtures)
- `-h`, `--help` — print the script's usage header

**Mock seam:** `GIT="${GIT:-git}"` — override `GIT` in tests to inject a stub.

## Behavior

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

| `.docket.yml` key | Default | Notes |
|---|---|---|
| `metadata_branch` | `docket` | must be `docket` or `main`; anything else aborts |
| `integration_branch` | `auto` | `auto` or empty → `DEFAULT_BRANCH`; explicit value used verbatim |
| `changes_dir` | `docs/changes` | |
| `adrs_dir` | `docs/adrs` | |
| `results_dir` | `docs/results` | |
| `gate` (finalize) | `local` | read from `finalize.gate` leaf key |
| `test_command` (finalize) | `` (empty) | read from `finalize.test_command` leaf key |
| `board_surfaces` | `inline` | YAML list `[a, b]` stripped of brackets/commas; `[]` → empty string |
| `auto_groom` | `false` | |

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
5. Re-reports `BOOTSTRAP=PROCEED` — the repo is now migrated; the caller may proceed.

In every other cell (`STOP_MIGRATE`, `PROCEED`, or `main`-mode), `--bootstrap` is a no-op.

### Emit

All resolved values are printed as `KEY='value'` (shell-quoted via `printf '%s=%q'`) to
stdout in this order:

```
DOCKET_MODE
DEFAULT_BRANCH
METADATA_BRANCH
INTEGRATION_BRANCH
METADATA_WORKTREE
CHANGES_DIR
ADRS_DIR
RESULTS_DIR
FINALIZE_GATE
FINALIZE_TEST_COMMAND
BOARD_SURFACES
AUTO_GROOM
BOOTSTRAP
```

13 lines total. The last line is always `BOOTSTRAP=…`.

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
| `mktree`/`commit-tree`/push failed during orphan create | 1 |
| `--repo-dir` missing its argument | 2 |
| Unknown argument | 2 |

## Invariants

- **Read-only by default.** The only write (`create_orphan`) is opt-in via `--bootstrap`
  and guarded to `¬DOCKET ∧ ¬LIVE`. Fetch and `remote set-head` are the sole side effects
  of a plain `--export` call.
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
- **13 `KEY=value` lines always emitted in the same order.** Skills may rely on the order
  for pipe consumers, but should use the variable names (via `eval`) for correctness.
