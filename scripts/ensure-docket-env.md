# ensure-docket-env.sh — idempotent Docket runtime-environment injector

## Purpose

Exports both `DOCKET_SCRIPTS_DIR` (the absolute path to this `scripts/` directory) and the
validated installer-selected `DOCKET_BASH_PATH` into two locations:

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

No positional arguments; no flags. `DOCKET_BASH_PATH` is required and must name an absolute,
executable GNU Bash 4+ interpreter; invalid input fails before either destination is changed.

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
| `zsh` | `~/.zshenv` | `export NAME="<value>"` |
| `bash` | `~/.bashrc` | `export NAME="<value>"` |
| `fish` | `~/.config/fish/config.fish` | `set -gx NAME "<value>"` |
| other | `~/.profile` | `export NAME="<value>"` (POSIX fallback) |

The script wraps the export line in a named marker block:

```
# >>> docket (DOCKET_SCRIPTS_DIR) >>>
export DOCKET_SCRIPTS_DIR="<value>"
export DOCKET_BASH_PATH="<absolute Bash 4+ path>"
# <<< docket (DOCKET_SCRIPTS_DIR) <<<
```

**Idempotency / re-run safety:** before appending, the script strips any pre-existing docket
marker block from the profile using `awk`. Marker order, balance, and uniqueness are validated
first; malformed blocks fail without touching the profile. A fresh block is then appended. This means:
- A second run on the same path produces exactly one marker block (no duplication).
- A moved clone updates the exported path instead of duplicating or preserving a stale value.
- File permissions are preserved across the rewrite (read via `stat`, applied via `chmod`).
- The rendered file is created beside the profile and atomically renamed into place.

### Stage 2: Claude Code settings.json env block

Writes both `env.DOCKET_SCRIPTS_DIR` and `env.DOCKET_BASH_PATH` into `~/.claude/settings.json` (or
`$DOCKET_HARNESS_ROOT/.claude/settings.json`) using `jq`.

Behavior:
- Creates the directory and seeds the file with `{}` if it does not exist.
- Skips the write if `jq` is not installed (emits a warning; the profile export still
  completes).
- Leaves the file unchanged if it is not valid JSON (emits a warning).
- Preserves all existing keys in the file; only the two Docket env keys are set/overwritten.
- File permissions are preserved across the rewrite.
- Writes through a same-directory temporary file and atomic rename.

The `settings.json` write is idempotent: running again with the same path simply overwrites
the same key with the same value.

## Exit codes

Invalid runtime input, malformed profile markers, or settings permission/rename failures exit
non-zero. A missing `jq` or pre-existing invalid JSON remains a warning and does not undo the
already-completed profile update.

## Invariants

- **Profile is always written.** Even if the `settings.json` stage fails, the shell-profile
  export completes. The two stages are independent.
- **Idempotent in both stages.** Re-running any number of times produces the same end state:
  exactly one marker block in the profile (with the current path), and the correct
  `DOCKET_SCRIPTS_DIR` and `DOCKET_BASH_PATH` values in `settings.json`.
- **Stale-path safe.** A re-run after moving the docket clone replaces the old path in the
  profile block rather than appending a duplicate.
- **Real clone, no drift.** `DOCKET_SCRIPTS_DIR` is always set to the `scripts/` directory of
  the running script (`HERE`), which is the same location the skills are symlinked from.
- **Never edits consuming-repo files.** All writes go to the user's home directory (profile
  and `~/.claude/settings.json`). No consuming repo's files are touched.
