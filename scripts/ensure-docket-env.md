# ensure-docket-env.sh — idempotent DOCKET_SCRIPTS_DIR injector

## Purpose

Makes docket's helper scripts reachable from any consuming repo by exporting
`DOCKET_SCRIPTS_DIR` (the absolute path to this `scripts/` directory) into two locations:

1. **Shell profile** (primary) — re-sourced on every Bash-tool call inside Claude Code, so
   dispatched subagents pick up the variable without a session restart.
2. **Claude Code user-level `settings.json` env block** (reinforcement) — read at session
   start; acts as a backup when the profile is not sourced by Claude Code's Bash invocations.

`DOCKET_SCRIPTS_DIR` points at the live docket clone the skills are symlinked from, giving
zero drift between the env variable and the scripts on disk. Every docket skill resolves its
helpers as `"${DOCKET_SCRIPTS_DIR:?run docket/install.sh}"/<name>.sh`, so a missing or
incomplete install fails loudly at the first call instead of silently degrading.

`install.sh` runs this script during initial setup. Re-running it back-fills already-migrated
clones whose clone was moved or whose profile predates the variable. Introduced in change 0034.

## Usage

```bash
bash scripts/ensure-docket-env.sh
```

No positional arguments; no flags.

**Test seams:**
- `HOME` — redirects the profile target and the default `settings.json` root.
- `DOCKET_HARNESS_ROOT` — overrides the settings.json root independently of `HOME` (default:
  `$HOME`).
- `DOCKET_TARGET_SHELL` — forces the profile flavor (default: `basename "$SHELL"`).

## Behavior

### Stage 1: shell-profile export

Selects the target profile based on `$SHELL` (or `DOCKET_TARGET_SHELL`):

| Shell | Profile file | Export syntax |
|---|---|---|
| `zsh` | `~/.zshenv` | `export DOCKET_SCRIPTS_DIR="<value>"` |
| `bash` | `~/.bashrc` | `export DOCKET_SCRIPTS_DIR="<value>"` |
| `fish` | `~/.config/fish/config.fish` | `set -gx DOCKET_SCRIPTS_DIR "<value>"` |
| other | `~/.profile` | `export DOCKET_SCRIPTS_DIR="<value>"` (POSIX fallback) |

The script wraps the export line in a named marker block:

```
# >>> docket (DOCKET_SCRIPTS_DIR) >>>
export DOCKET_SCRIPTS_DIR="<value>"
# <<< docket (DOCKET_SCRIPTS_DIR) <<<
```

**Idempotency / re-run safety:** before appending, the script strips any pre-existing docket
marker block from the profile using `awk`. A fresh block is then appended. This means:
- A second run on the same path produces exactly one marker block (no duplication).
- A moved clone updates the exported path instead of duplicating or preserving a stale value.
- File permissions are preserved across the rewrite (read via `stat`, applied via `chmod`).

### Stage 2: Claude Code settings.json env block

Writes `env.DOCKET_SCRIPTS_DIR = "<value>"` into `~/.claude/settings.json` (or
`$DOCKET_HARNESS_ROOT/.claude/settings.json`) using `jq`.

Behavior:
- Creates the directory and seeds the file with `{}` if it does not exist.
- Skips the write if `jq` is not installed (emits a warning; the profile export still
  completes).
- Leaves the file unchanged if it is not valid JSON (emits a warning).
- Preserves all existing keys in the file; only `.env.DOCKET_SCRIPTS_DIR` is set/overwritten.
- File permissions are preserved across the rewrite.

The `settings.json` write is idempotent: running again with the same path simply overwrites
the same key with the same value.

## Exit codes

This script always exits 0. Soft failures (jq absent, invalid JSON) are reported as warnings
on stdout and do not abort the run.

## Invariants

- **Profile is always written.** Even if the `settings.json` stage fails, the shell-profile
  export completes. The two stages are independent.
- **Idempotent in both stages.** Re-running any number of times produces the same end state:
  exactly one marker block in the profile (with the current path), and the correct
  `DOCKET_SCRIPTS_DIR` value in `settings.json`.
- **Stale-path safe.** A re-run after moving the docket clone replaces the old path in the
  profile block rather than appending a duplicate.
- **Real clone, no drift.** `DOCKET_SCRIPTS_DIR` is always set to the `scripts/` directory of
  the running script (`HERE`), which is the same location the skills are symlinked from.
- **Never edits consuming-repo files.** All writes go to the user's home directory (profile
  and `~/.claude/settings.json`). No consuming repo's files are touched.
