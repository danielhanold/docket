<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0340 — Stamp build identity into the `development install` binary](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0340-stamp-build-identity-in-development-install.md)**
<!-- docket:backlink:end -->

# Stamp build identity into the `development install` binary — design

- **Change:** 0340
- **Date:** 2026-08-23
- **Status:** Approved

## Problem

`docket version` reports `docket development (commit unknown, built unknown)` even
immediately after a fresh `docket development install --source <checkout>`. The
`internal/buildinfo` ldflags seam (Version/Commit/BuildDate) exists on `main` and is
used by the release packager (change 0317, `internal/release/package.go`), but the
development-install build path (`buildBinary` in `internal/install/devmode.go`) runs a
bare `go build -o <staged> ./cmd/docket` with no `-ldflags`, so a locally-built binary
carries no identity. There is consequently no way to check the installed binary against
current source; the AGENTS.md "rebuild after every merge to main" rule mitigates the
drift blindly instead of making it visible.

## Design

### Identity values

Computed from the `--source` checkout's git state at build time:

- **Version** = `git describe --tags --always --dirty` output.
  - At a tag, clean: `v0.3.0`
  - Past a tag, clean: `v0.3.0-12-g84a1027`
  - Dirty: the same with a `-dirty` suffix
  - No tags in the repo (`--always` fallback): bare short SHA, e.g. `84a1027`
    (+ `-dirty` when dirty) — today's day-to-day case, since the repo has no tags.
- **Commit** = full SHA from `git rev-parse HEAD`, with a `-dirty` suffix appended when
  the working tree has uncommitted changes (dirtiness taken from the `describe` output's
  `-dirty` suffix, or an equivalent `git status --porcelain` probe — implementation's
  choice, but one probe, applied consistently to Version and Commit).
- **BuildDate** = UTC now, same format the release packager stamps.

No dev-specific marker is added when the checkout sits exactly at a tag: `git describe`
output is the understood convention, and Commit + BuildDate disambiguate a local build
from a release. Decided and accepted.

### Fallback — identity is a nicety, never a gate

If `git` is unavailable, the source is not a git checkout, or any git probe fails, the
build proceeds **without** `-ldflags` and the binary reports the compiled-in defaults
(`development` / `unknown` / `unknown`). The install never fails, and no partial
identity is stamped (all three or none — a Version with an `unknown` Commit would be a
new, misleading shape).

### Placement and seams

All work lands in `internal/install/devmode.go`:

- The git probes run through the same runner-seam style the file already uses for the
  toolchain (`GoRunner`): a small injectable runner that executes a git argument vector
  in the source directory and returns its output, so tests inject canned git state
  without a real repository. As with `GoRunner`, it is an argv, never a shell string.
- `buildBinary` assembles the ldflags — reusing the release packager's exact `-X`
  triple format against `github.com/danielhanold/docket/internal/buildinfo` — and
  appends `-ldflags <...>` to the existing `go build` invocation when identity was
  resolved.
- `internal/buildinfo` itself is untouched (no new fields), and the release path
  (0317) is untouched. If 0317's `internal/release/package.go` has merged by build
  time, the ldflags-assembly helper may be shared rather than duplicated; if not,
  duplicate the three-`-X` format string locally — it is one line, and 0317's branch
  must not be a dependency.

### Testing

- Unit tests inject the git runner: full identity (tagged/untagged, clean/dirty)
  asserts the exact `-ldflags` argv handed to `GoRunner`; each failure mode (no git,
  not a repo, probe error) asserts a bare build with no `-ldflags` and a successful
  outcome.
- The existing `cmd/docket` ldflags-injection test already proves the seam end-to-end;
  no new binary-level test is required.

## Out of scope

- The release/packaging build path (0317 stamps it).
- New `internal/buildinfo` fields.
- Any change to the AGENTS.md rebuild-on-merge convention (stays as a safety net).
