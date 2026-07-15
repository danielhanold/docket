# ensure-global-config.sh

## Purpose

Scaffold the global docket config on first run: drop the committed `config.yml.example`
into place as the user's global `~/.config/docket/config.yml`, so the otherwise-invisible
per-skill defaults are discoverable and the file exists for editing — without ever
clobbering a config the user has already written.

## Usage

```
bash scripts/ensure-global-config.sh
```

Run by `install.sh` as a primitive, before `sync-agents.sh`. Standalone-safe.

Environment:
- `XDG_CONFIG_HOME` — when set, the config root (wins over `HOME`/`DOCKET_HARNESS_ROOT`).
- `DOCKET_HARNESS_ROOT` — test seam overriding `$HOME` for the config root; consulted only
  when `XDG_CONFIG_HOME` is unset. Matches `sync-agents.sh`'s resolution so both agree on
  the path.

## Behavior

- Destination: `${XDG_CONFIG_HOME:-${DOCKET_HARNESS_ROOT:-$HOME}/.config}/docket/config.yml`.
- If the destination does NOT exist: create the parent dir as needed, copy
  `config.yml.example` (from the repo root) to it, and log
  `docket: wrote <dest> from config.yml.example (edit to enable harnesses / tune models)`.
- If the destination already exists: do nothing to it, log
  `docket: <dest> already exists — left untouched`.
- If `config.yml.example` is missing: log a skip to stderr and exit 0 (never fatal).
- Never overwrites, merges, or edits an existing file.

## Exit codes

- `0` — on success (wrote, left-untouched, or source-missing skip). Idempotent. A genuine
  write failure (e.g. an unwritable config dir) propagates a non-zero exit under `set -e`.

## Invariants

- An existing global config is never modified.
- The written copy is byte-identical to `config.yml.example`.
- The destination path equals the path `sync-agents.sh` reads as the global config.
