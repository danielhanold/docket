# Install a configured Bash 4+ runtime

## Context

Change 0128 exposed that Docket's local finalize gate can select macOS's legacy `/bin/bash`
(3.2) when a login shell runs a bare `bash`. Docket's validator family requires Bash 4+ features,
including associative arrays and `mapfile`, so the gate reports a false code failure before the
suite can meaningfully run. The actual compatible interpreter on this machine is
`/opt/homebrew/bin/bash`.

`finalize.test_command` is deliberately not the solution: it describes a repository's complete
test-suite command, while an interpreter path is machine-local Docket runtime state. Replacing
one with the other would bake Docket's own test layout into a generic repository setting.

## Goals

- Make the Bash interpreter Docket uses deterministic and inspectable.
- Require Bash 4+ at installation and preflight, with an actionable failure message.
- Discover and persist a compatible interpreter automatically during installation.
- Ensure Docket-owned shell scripts and auto-detected shell tests use that exact interpreter.
- Preserve arbitrary repository `finalize.test_command` values as explicit commands.

## Non-goals

- Alter a repository's test framework, test command, or scripts that explicitly hardcode
  `/bin/bash`.
- Fix the independent noninteractive board-conflict fixture stall tracked by change 0131.
- Add a shell-level privilege escalation mechanism or change user-wide PATH ordering.

## Design

### Machine-local runtime configuration

Add a `runtime.bash` setting whose resolved value is exported as `DOCKET_BASH_PATH`.

```yaml
runtime:
  bash: "/opt/homebrew/bin/bash"
```

The key is machine-local runtime state. The global user config is its normal home; a repository's
`.docket.local.yml` may override it for a special machine. A committed `.docket.yml` value is
warned-and-ignored under ADR-0019's configuration fence: an absolute interpreter path cannot be
shared safely between clones.

The config resolver reads the nested `runtime:` block rather than matching a bare `bash:` leaf,
validates an absolute executable path and Bash major version 4 or greater, and emits
`DOCKET_BASH_PATH` in the preflight/export block. A missing, non-executable, relative, or too-old
path is a fail-closed error.

### Install-time discovery and bootstrap

`install.sh` gains a POSIX-safe bootstrap that can run before any Bash-specific logic. It searches
deterministically for candidate absolute paths: Homebrew's `bash` formula prefix when available,
the standard Homebrew locations (`/opt/homebrew/bin/bash`, then `/usr/local/bin/bash`), then an
absolute `bash` resolved from PATH. Each candidate is executed only to read its version; a
candidate qualifies only when its major version is at least four.

On a fresh install, the installer writes the discovered path in a managed `runtime:` block in
`${XDG_CONFIG_HOME:-$HOME/.config}/docket/config.yml`, preserving existing user-authored config.
An existing valid explicit value is preserved. An existing invalid value stops installation rather
than being replaced silently. The installer also writes `DOCKET_BASH_PATH` to the machine-level
shell and supported harness environments, alongside `DOCKET_SCRIPTS_DIR`; this is the bootstrap
binding used before Docket can read its config. Re-running install refreshes that binding after a
user changes the configured path.

When no compatible Bash is available, install and preflight stop with the detected path/version
and a platform-appropriate remedy. On macOS the primary remedy is:

```text
docket requires Bash 4+; found only /bin/bash 3.2.
Install it with: brew install bash
Then rerun: bash install.sh
```

### Explicit execution boundary

Make the public Docket facade a POSIX bootstrap that executes its Bash implementation through
`DOCKET_BASH_PATH`. After Step-0 preflight, operating skills invoke every Docket helper through
that same explicit interpreter instead of a bare `bash` or PATH-selected shebang. Direct manual
execution under an unsupported interpreter fails immediately with the same remediation before any
Bash-4-only operation is attempted.

The finalize gate uses `DOCKET_BASH_PATH` to run every auto-detected shell test, so a
`tests/test_*.sh` suite runs each file as `"$DOCKET_BASH_PATH" "$test"`. A user-provided
`finalize.test_command` is still run verbatim, with `DOCKET_BASH_PATH` supplied in its environment;
Docket does not rewrite arbitrary user shell text.

### Verification

- Add hermetic discovery fixtures for Homebrew, PATH, no compatible interpreter, and Bash 3.2.
- Prove install writes and preserves the managed global `runtime.bash` value and environment
  binding without disturbing unrelated user config.
- Mutation-test resolver precedence and the committed-config fence; assert every preflight export
  contains a valid `DOCKET_BASH_PATH`.
- Prove the facade and auto-detected shell-test runner execute a fake configured Bash rather than
  `/bin/bash` or a PATH-selected `bash`.
- Preserve the full existing suite as the build gate; run it under the configured interpreter.

## Decisions

- Missing Bash 4+ is a hard installation and preflight failure, never a warning/fallback.
- `DOCKET_BASH_PATH` is the canonical environment/export name.
- Installation discovers and records the path automatically in user/global configuration.
- Runtime selection is distinct from `finalize.test_command` and must not prescribe a repository's
  test suite.
- The board-fixture hang is deliberately left to change 0131.
