## What this release is

The first release candidate for `v1.0.0`, published as a full release. It is the first Go build to be "Latest", replacing the Bash `v0.9.3`.

- **Claude Code** was tested end to end in `v1.0.0-alpha.1`. **Cursor** was tested end to end in `v1.0.0-alpha.2`. Both are covered at this build by the full test suite; neither was re-run by hand.
- **OpenCode** installs and ships, but is **untested**. Full OpenCode support comes after `v1.0.0`.
- **Codex** is **unsupported**.
- Like the alphas, this is a hard replacement of the Bash implementation, with no Bash fallback.

## Install

Supported platforms: macOS and Linux, on amd64 and arm64. You need `sh`, `curl`, `tar`, and a SHA-256 tool (`shasum` or `sha256sum`).

Download the installer and the checksum file, check the installer, then run it for the harnesses you use:

```sh
VERSION=v1.0.0-rc.1
curl -fsSLO "https://github.com/danielhanold/docket/releases/download/$VERSION/install.sh"
curl -fsSLO "https://github.com/danielhanold/docket/releases/download/$VERSION/checksums.txt"
grep '  install.sh$' checksums.txt | shasum -a 256 -c -
sh install.sh --version "$VERSION" --harness claude --harness cursor
```

The `shasum` line must print `install.sh: OK`; if it prints anything else, do not run `install.sh`. Drop the `--harness` you don't use. The installer puts `docket` in `~/.local/bin`.

Restart Claude Code and Cursor after installing: both load agents and skills at startup.

**Cursor:** docket must run outside Cursor's sandbox, because it fetches and pushes. See [Cursor: running docket under Cursor](https://github.com/danielhanold/docket/blob/main/docs/install/cursor.md).

Install with the script, not a browser download (see Known gaps).

## Upgrading

- **From Bash docket `v0.9.x`:** follow [Upgrading from Bash docket](https://github.com/danielhanold/docket/blob/main/docs/release/upgrading-from-bash.md), which covers Claude Code and Cursor; it has no OpenCode section yet. The Bash tags `v0.9.2` and `v0.9.3` remain available.
- **From an alpha:** re-run the install commands above for `v1.0.0-rc.1`, then restart your harnesses.

## What changed since alpha.2

Documentation only. The release candidate's source is identical to `v1.0.0-alpha.2`; only the embedded version differs. The changes since alpha.2 are the alpha.2 release records under `docs/release/`.

## Known gaps

- **Cursor's sandboxed mode was not exercised.** The documented "Allowlist (with Sandbox)" setup was not part of the Cursor test; it used Run Everything (unsandboxed).
- **Cursor may run docket's subagents at a different model effort than their pins.** Unconfirmed; tracked in change 0545.
- **Quitting Cursor can leave its agent worker running.** If you stop Cursor in the middle of a docket run, check for a leftover `cursor-agent` process before resuming.
- OpenCode is untested; full OpenCode support comes after `v1.0.0`. Codex is unsupported.
- **The binaries are not notarized.** An archive downloaded in a browser and extracted in Finder is blocked by macOS Gatekeeper. Use the install script (or run `xattr -d com.apple.quarantine docket`).
- No Homebrew or Windows.

## Evidence

The release evidence is in [`docs/release/v1.0.0-rc.1/`](https://github.com/danielhanold/docket/tree/main/docs/release/v1.0.0-rc.1) on `main`. The release rests on the ADRs listed for alpha.1 (ADR-0095, ADR-0096, ADR-0099, ADR-0100, ADR-0102, ADR-0103, ADR-0104) and on ADR-0140 through ADR-0148.
