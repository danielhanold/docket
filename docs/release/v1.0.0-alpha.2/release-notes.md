## What this release is

The second public build of the docket Go binary, an alpha for existing users.

- **Cursor is now tested end to end.** A full lifecycle (create, groom, implement with a mid-run kill and resume, finalize) ran in Cursor against this exact build.
- Claude Code was tested end to end in `v1.0.0-alpha.1` and is covered in this release by the test suite.
- OpenCode is installable but untested; it is proven in alpha.3. Codex is paused.
- Like alpha.1, this is a hard replacement of the Bash implementation, with no Bash fallback.

## Install

Supported platforms: macOS and Linux, on amd64 and arm64. You need `sh`, `curl`, `tar`, and a SHA-256 tool (`shasum` or `sha256sum`).

Download the installer and the checksum file, check the installer, then run it for the harnesses you use:

```sh
VERSION=v1.0.0-alpha.2
curl -fsSLO "https://github.com/danielhanold/docket/releases/download/$VERSION/install.sh"
curl -fsSLO "https://github.com/danielhanold/docket/releases/download/$VERSION/checksums.txt"
grep '  install.sh$' checksums.txt | shasum -a 256 -c -
sh install.sh --harness claude --harness cursor
```

The `shasum` line must print `install.sh: OK`; if it prints anything else, do not run `install.sh`. Drop the `--harness` you don't use. The installer puts `docket` in `~/.local/bin`.

Restart Claude Code and Cursor after installing: both load agents and skills at startup.

**Cursor:** docket must run outside Cursor's sandbox, because it fetches and pushes. See [Cursor: running docket under Cursor](https://github.com/danielhanold/docket/blob/main/docs/install/cursor.md).

## Upgrading

- **From `v1.0.0-alpha.1`:** run the install commands above with `VERSION=v1.0.0-alpha.2`, then restart your harnesses.
- **From Bash docket `v0.9.x`:** follow [Upgrading from Bash docket](https://github.com/danielhanold/docket/blob/main/docs/release/upgrading-from-bash.md), which now covers Cursor as well as Claude Code. The Bash tags `v0.9.2` and `v0.9.3` remain available.

## What changed since alpha.1

- **Private visibility.** A repository can keep all of docket's metadata off its shared remote: the metadata branch lives on a local remote under neutral `dckt` naming, nothing docket-named is committed to the repository, and a blocking leak check guards outgoing pushes and PRs. `docket repository set-visibility` switches a repository between shared and private and keeps every record. The installer places a `dckt` alias beside the binary. (#399, #400, #401, #402, #404, #406)
- **Build artifacts on the metadata branch.** Plans, results and build evidence now live on the metadata branch; the spec ships with the PR. (#398)
- **Choosing harnesses.** `docket repository init --harnesses` and `docket repository configure-harnesses` choose which coding agents docket writes instructions for, with an interactive checklist on a terminal. (#403)
- **`docket open`.** Opens the board, a change, or its artifacts from the terminal. (#407)
- **Fixes.**
  - Finalize merges on repositories whose plan does not offer GitHub branch rules, and finishes a worktree removal Git already started. (#393)
  - A `.docket` copy that is only behind its remote is reported healthy and fast-forwarded in place. (#394)
  - `docket repository configure-tests --command "<cmd>"` sets both test gates directly. (#395)
  - A merged PR's backlink is repointed when its change is archived. (#397)
  - The release candidate's `evidence.json` keeps `checksums.txt` byte-exact. (#391)
- **Docs.** The Bash upgrade guide, now with Cursor. (#389, #408)

## Known gaps

- **Cursor's sandboxed mode was not exercised.** The documented "Allowlist (with Sandbox)" setup was not part of this release's test; the acceptance run used Run Everything (unsandboxed).
- **Cursor may run docket's subagents at a different model effort than their pins.** In the acceptance run, agents pinned to grok-4.5 low or medium ran at grok-4.5 high.
- **Quitting Cursor can leave its agent worker running.** After Cursor's main process was killed, its `cursor-agent` worker stayed alive. If you stop Cursor in the middle of a docket run, check for a leftover `cursor-agent` process before resuming.
- OpenCode is installable but untested (alpha.3).
- Codex is paused.
- No Homebrew, Windows, or signing/notarization.

## Evidence

The acceptance evidence for this release is in [`docs/release/v1.0.0-alpha.2/`](https://github.com/danielhanold/docket/tree/main/docs/release/v1.0.0-alpha.2) on `main`. The release rests on the ADRs listed for alpha.1 (ADR-0095, ADR-0096, ADR-0099, ADR-0100, ADR-0102, ADR-0103, ADR-0104) and on ADR-0140 through ADR-0148.
