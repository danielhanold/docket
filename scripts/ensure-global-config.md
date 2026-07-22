# ensure-global-config.sh

## Purpose

Bootstraps Docket's machine-local Bash runtime in the global config before any installer stage
that may require Bash 4 or newer. It also retains the minimal pointer to `.docket.example.yml` on
first install. The bootstrap itself is compatible with macOS Bash 3.2.

## Usage

```bash
bash scripts/ensure-global-config.sh
```

The destination is
`${XDG_CONFIG_HOME:-${DOCKET_HARNESS_ROOT:-$HOME}/.config}/docket/config.yml`.
`DOCKET_BASH_STANDARD_ROOT` is a test-only seam that roots the two fixed macOS candidate paths in
a sandbox; production leaves it unset.

## Discovery and validation

If the user has not written an explicit `runtime.bash`, candidates are deduplicated and checked in
this order:

1. `<brew --prefix>/bin/bash`, when that command succeeds;
2. `/opt/homebrew/bin/bash`;
3. `/usr/local/bin/bash`;
4. the absolute result of resolving `bash` on `PATH`.

Each candidate must be an absolute executable path and identify itself, from its sole
`--version` probe, as GNU Bash major version 4 or newer. If none qualifies the script exits
non-zero and tells the user to run `brew install bash`.

## Config ownership and writes

A valid hand-authored `runtime.bash` is authoritative and leaves the complete file byte-untouched.
An invalid hand-authored value stops installation and is never silently replaced. Otherwise the
script writes this owned block before the user-owned bytes:

```yaml
# >>> docket (runtime.bash) >>>
runtime:
  bash: '/absolute/path/to/bash'
# <<< docket (runtime.bash) <<<
```

The managed scalar uses YAML single-quote rules: an apostrophe in the path is doubled and a
backslash remains literal. Carriage returns and newlines are rejected. A present but empty
hand-authored declaration is invalid (it is never treated as absent and replaced by discovery).

Before replacing an existing owned block it validates marker order, balance, and uniqueness.
Malformed markers cause a non-zero exit with no write. Successful changes are rendered to a
same-directory temporary file, retain the destination's permission bits, and are atomically
renamed into place. User-owned bytes remain exact even when their final line has no newline, on
both first install and re-run.

## Exit codes

- `0` — a qualifying runtime is preserved or persisted.
- non-zero — an explicit value is invalid, no qualifying runtime exists, markers are malformed,
  or an atomic write fails.
