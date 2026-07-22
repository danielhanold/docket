# Centralize shared Bash runtime configuration helpers

## Context

Change 0132 makes `runtime.bash` machine-local configuration and requires Docket's configured
runtime to be GNU Bash 4+. Its implementation introduced the same small YAML subset parser in
`install.sh`, `scripts/ensure-global-config.sh`, and `scripts/docket-config.sh`. The latter two
also duplicate the runtime-path validation contract.

The copies currently differ in caller policy: installer-owned markers are excluded while checking
for an explicit user value, the resolver rejects an ambiguous machine-local layer, and install
reads the already-normalized global value after the installer succeeds. Duplicating the scanner
and validator makes a future parsing or validation fix likely to land in only one path.

The bootstrap path is special. `install.sh` and `ensure-global-config.sh` must run before a
compatible runtime has been discovered and persisted, so their shared helpers must be compatible
with the system Bash available at bootstrap time (including macOS Bash 3.2). This does not relax
the configured-runtime requirement: the helper continues to validate and select GNU Bash 4+ for
all post-bootstrap Docket execution.

## Goals

- Make one bootstrap-compatible library the source of truth for every duplicated
  `runtime.bash` parsing, declaration-counting, path-serialization, and Bash-version validation
  helper.
- Preserve each caller's current authority, marker, precedence, and diagnostic policy.
- Keep the runtime contract unchanged: a configured value must be an absolute executable GNU Bash
  version 4 or newer.
- Prove the shared helper works both in the bootstrap path and in the resolver/install callers.

## Non-goals

- Replace Docket's broader YAML-reading strategy with `yq` or a general YAML parser (change 0018
  remains the place to evaluate that).
- Change runtime discovery order, managed-block ownership, layer precedence, or user-facing
  failure posture from change 0132.
- Permit Bash 3.2 as Docket's configured runtime after installation.
- Refactor unrelated configuration helpers.

## Design

### Bootstrap-compatible runtime helper library

Add a source-only `scripts/lib/docket-runtime.sh` library. Its implementation uses syntax and
utilities available to Bash 3.2, has no top-level side effects, and owns the reusable runtime
primitives:

- scan a top-level `runtime:` block and decode one-line, quoted or unquoted `bash:` scalars;
- count declarations and report an ambiguous declaration set distinctly from an absent value;
- optionally exclude a caller-supplied, balanced installer-managed marker block while scanning;
- validate that a value can be serialized as a one-line YAML scalar; and
- validate an absolute executable as GNU Bash 4+ without printing policy-specific diagnostics.

The library's scanner is the sole implementation of scalar decoding and runtime-block traversal.
It exposes small namespaced functions so callers can request their existing semantic shape:
unique-value lookup for the resolver, first explicit value plus a count for the installer, and a
normal global-value read for the post-bootstrap installer. A caller supplies marker strings only
when it deliberately needs to exclude the installer-owned block; the library never decides which
configuration authority wins.

### Caller boundaries

`scripts/ensure-global-config.sh` sources the library before discovery. It keeps discovery order,
managed-marker validation and rewrite logic, explicit-versus-managed authority checks, and its
current error messages. It replaces only its `explicit_runtime`, `explicit_runtime_count`,
`validate_runtime`, and serializability helper bodies with library calls.

`scripts/docket-config.sh` sources the same library through its existing `scripts/lib` resolution.
It retains repo-local/global precedence, committed-key fencing, duplicate-value failure messages,
and resolver-specific diagnostics; it delegates parsing, counting, and boolean validation.

`install.sh` sources the library through `scripts/lib` after locating its own repository root. It
uses the shared runtime read to obtain the global value written or validated by the preceding
bootstrap step, then continues to execute downstream scripts through `DOCKET_BASH_PATH`.

No caller imports discovery or managed-block mutation from the library. This prevents a shared
utility from silently changing the distinction between installer-owned data and resolver-owned
policy.

### Verification

- Add direct unit coverage for scalar decoding, duplicate counting, marker-block exclusion, blank
  values, quoted inline comments, apostrophes, and backslashes through the new library interface.
- Exercise the library from a bootstrap invocation that is valid under Bash 3.2-compatible
  syntax; the test must not require Bash-4-only language features in the helper.
- Preserve and extend the existing installer, install, and resolver tests to prove their current
  authority, precedence, diagnostics, and `DOCKET_BASH_PATH` routing behavior are unchanged.
- Mutation-test the single shared scanner/validator: removing its runtime-block constraint,
  duplicate detection, marker exclusion, or Bash-major-version check must redden a focused test.
- Run the full repository suite at the build gate.

## Decisions

- The shared library is bootstrap-compatible because it runs before the configured Bash can be
  selected; it is not a fallback runtime.
- The library centralizes reusable mechanics only. Authority, discovery, writes, and diagnostics
  remain caller-owned policy.
- Change 0132 is the provenance for this maintenance follow-up; no new ADR is needed because the
  established runtime and configuration boundaries remain unchanged.
