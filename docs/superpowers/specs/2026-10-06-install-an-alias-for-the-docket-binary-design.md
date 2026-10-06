<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0534 — Install an alias for the docket binary](../../changes/archive/2026-10-06-0534-install-an-alias-for-the-docket-binary.md)**
<!-- docket:backlink:end -->

# Install an alias for the docket binary: design

Change #534, groomed interactively on 2026-10-05, split out of #532 in the private-visibility series (#529–#533 plus this change and the private-instructions change). It is independent and can land first.

## Summary

Every install also places `dckt`, a symlink to the `docket` binary, in the same bin directory. The private-instructions change in this series needs it: its user-level Claude hook and Codex/OpenCode pointer must not contain the word "docket", so they invoke `dckt instructions`. Nothing else switches to the alias. Skills keep resolving `docket` from the capability catalog.

## Evidence gathered at grooming

- **Two install paths place the binary.**
  - **Public release install** (v1.0.0-alpha.1 already uses it): `internal/release/downloader/install.sh` is rendered into every release bundle by `internal/release/render.go` and published as a release asset. It:
    - downloads the archive and verifies its SHA-256 before extraction;
    - stages the binary in the bin directory (`.docket-stage.XXXXXX`) and runs `"$stage" install …`;
    - `mv -f`s it to `$bin_dir/docket`;
    - writes its own ownership record at `${XDG_STATE_HOME:-~/.local/state}/docket/release-binary.record`.

    Its contract comment pins the runtime dependencies (`/bin/sh`, `curl`, `tar`, one SHA-256 tool), the refusal to replace a binary it does not own, and "no --force path". `tests/test_release_downloader.sh`, `tests/test_release_downloader_refusals.sh`, `tests/test_release_downloader_converge.sh`, and `scripts/release-smoke.sh` depend on its spellings and PATH sandbox.
  - **Development install:** the repository-root `install.sh` is a thin bootstrapper that delegates to `docket development install` (`internal/install/devmode.go`, `binaryName = "docket"`, `resolveBinDir` defaulting to `UserRoots.BinDir` = `${XDG_BIN_HOME:-~/.local/bin}`). That command builds the binary and places it inside a journaled install transaction.
- **Uninstall never removes the binary.** `internal/install/uninstall.go`: "The installation's own binary and every unattributed target remain recorded and untouched."
- **Skills build every invocation from the capability catalog,** whose argv spells `docket`. The binary's commands do not depend on the name it was invoked by. The plan verifies this against the CLI root.

## Decisions (settled with the human)

1. The alias is `dckt`, a symlink to the installed `docket`, in the same bin directory.
2. Both install paths create it. The release downloader is the path users take from v1.0, and the development installer is the path behind the repository-root `install.sh`.
3. The alias follows the binary: same placement, same ownership posture, left in place by `uninstall`.
4. A foreign `dckt` is never overwritten, and there is no force path.
5. Sparse documentation: installer output and the downloader's usage text only, no `docs/` pages.

## Design

### Release downloader

After the verified binary is moved into `$bin_dir/docket`, the downloader ensures `$bin_dir/dckt`:

- **Absent:** create a symlink to `docket`, using a relative target so moving the bin directory keeps it valid.
- **Already a symlink resolving to `$bin_dir/docket`:** leave it (converge).
- **Anything else:** leave it untouched, print a one-line warning naming the path and the remedy, and finish the install successfully. The binary install is never failed by the alias.

The ownership record gains the alias path, so a later run can tell its own alias from a foreign one. The contract comment, usage text, and tests are updated: `ln -s` is admitted by the spelling ban and the PATH sandbox, and the tests cover create, converge, and foreign refusal. `scripts/release-smoke.sh` asserts that the alias exists after a smoke install.

### Development installer

`docket development install` applies the same three-way rule beside the binary it places, inside its journaled transaction. A rollback of a failed install removes an alias it created in that transaction and leaves any pre-existing one alone. The repository-root `install.sh` changes only its header comment, which lists what an install produces.

### Install check and uninstall

- `docket install check` reports a missing or foreign `dckt` as a finding with its remedy. It is a finding, not a refusal.
- `docket uninstall` leaves the alias in place, exactly as it leaves the binary.

## Acceptance criteria

1. A fresh release install (downloader against a local release fixture) leaves `$bin_dir/dckt` resolving to `$bin_dir/docket`. A second run converges with no change.
2. A fresh development install does the same.
3. A pre-existing foreign `dckt` is untouched by both installers, both installs succeed with a warning, and `install check` reports it.
4. `dckt version --json` and `docket version --json` report identical output.
5. `uninstall` leaves both binary and alias in place.
6. The downloader's spelling-ban and PATH-sandbox tests pass with the added `ln -s`. Removing the alias step turns the new downloader test red (mutation-tested).

## Out of scope

- Renaming the binary or the catalog spelling.
- Shell aliases, completions, and package formulas.
- The user-level surfaces that invoke `dckt` (the private-instructions change).
