# ensure-global-config.sh

## Purpose

Scaffold the global docket config on first run: write a minimal, pointer-only
`~/.config/docket/config.yml` — a header comment naming `.docket.yml.example` as the canonical
reference, and zero active keys — so the file exists for editing without ever pinning a shipped
default, and without ever clobbering a config the user has already written.

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
- If the destination does NOT exist: create the parent dir as needed, write a fixed heredoc
  (header comment + layer-precedence list + pointer to `.docket.yml.example`, zero active
  keys), and log
  `docket: wrote <dest> (empty pointer config — see .docket.yml.example for every key)`.
- If the destination already exists: do nothing to it, log
  `docket: <dest> already exists — left untouched`.
- Never overwrites, merges, or edits an existing file.

## Exit codes

- `0` — on success (wrote or left-untouched). Idempotent. A genuine write failure (e.g. an
  unwritable config dir) propagates a non-zero exit under `set -e`.

## Invariants

- An existing global config is never modified.
- The scaffolded file contains no active keys, so it can never pin a shipped default.
- The destination path equals the path `sync-agents.sh` reads as the global config.
