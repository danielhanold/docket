## What this release is

This is the first public build of Docket as a Go binary. It is an alpha for people who already use Docket.

- It was tested end to end on **Claude Code only**. Cursor support is proven in alpha.2 and OpenCode in alpha.3. Codex support is paused.
- It replaces the Bash implementation outright. There is no Bash fallback.

## Install

Supported platforms: macOS and Linux, on amd64 and arm64. You need `sh`, `curl`, `tar`, and a SHA-256 tool (`shasum` or `sha256sum`).

Download the installer and the checksum file, check the installer, then run it:

```sh
VERSION=v1.0.0-alpha.1
curl -fsSLO "https://github.com/danielhanold/docket/releases/download/$VERSION/install.sh"
curl -fsSLO "https://github.com/danielhanold/docket/releases/download/$VERSION/checksums.txt"
grep '  install.sh$' checksums.txt | shasum -a 256 -c -
sh install.sh --harness claude
```

The `shasum` line must print `install.sh: OK`. If it doesn't, stop and don't run the installer. Don't pipe the downloaded script into `sh`.

Restart Claude Code after installing so it loads the Docket skills and agents.

## Upgrading from v0.9.x

Follow [Upgrading from Bash docket](https://github.com/danielhanold/docket/blob/main/docs/release/upgrading-from-bash.md). It covers taking over the old install, upgrading each repository, the settings that changed, and what you can delete afterwards.

The Bash tags `v0.9.2` and `v0.9.3` are still available.

## What changed since v0.9.3

- **Go foundation:** a Git adapter, an isolated metadata transaction engine, status and health checks, planning operations, workspaces, the GitHub PR adapter, and build evidence (changes 0308, 0309, 0310, 0312, 0313).
- **Bash control plane removed:** the Go-only source cutover, the frozen Bash facade and its tests deleted, and the remaining workflow operations moved to Go commands (changes 0318, 0339, 0363, 0369, 0370, 0372, 0377).
- **CLI surface:** a capability catalog that skills read for every command, a schema command, and typed operations for creating, grooming, revising, and retitling changes (changes 0382, 0394, 0395, 0445, 0461).
- **Configuration:** separate build and finalize test settings, configurable attempt limits and repair budgets, and configurable board sections and sorting (changes 0256, 0349, 0367, 0374, 0419, 0421).
- **Native harness dispatch:** Docket agents are dispatched through the harness's own agent registry, and plan writing runs in its own pinned agent (changes 0324, 0371).
- **Run tracker:** a tracked start, a verdict, an explicit cancel, and a resume for each dispatched implement-next run, with one handle per run (changes 0471, 0477, 0489, 0490, 0491).
- **Build gate and gate driver:** a native process supervisor and local gate, durable test-run driving, and build-evidence recording and re-certification (changes 0314, 0391, 0415).
- **Finalize:** rebase, repair, merge, archive, recovery, reclaim, cleanup of killed changes, and stacked changes, with optional closeout notes (changes 0316, 0330, 0388, 0483).
- **Metadata and the board:** a single metadata layout on the `docket` branch, native repository init, migration, and health checks, and a deterministic frontmatter writer (changes 0266, 0352, 0363, 0364).
- **Install:** a release downloader, a development install, and `docket uninstall` (changes 0317, 0322, 0323).
- **Docs:** a rewritten README, a guide, concepts, and reference docs, clearer terminology, and an Apache 2.0 license (changes 0400, 0402, 0404, 0464, 0468, 0469).

## Known gaps

- **The build agent can stall during long test runs (change 0412).** A dispatched implement-next run can return before its test run finishes. Recovery: run `docket run verdict <key>` with the key `docket run start` gave you, and do exactly what its `run-*` line says. If the run needs to be restarted, use `docket run start implement-next --resume <id>` and dispatch `docket-implement-next <id>` again.
- **Finalize can't merge on some private repositories.** On a private repository whose GitHub plan doesn't return the branch-rules API, finalize stops before merging. Merge the PR on GitHub yourself, then run `docket-finalize-change <id>` again. It archives the merged change and cleans up.
- **`docket repository check` can report a clean metadata copy as diverged (change 0523).** This happens when the local `.docket` copy is only behind the remote. Run `docket repository prepare` to sync it, then check again.
- **Cursor and OpenCode** can be installed but are not tested in this release (changes 0512, 0513).
- **Codex** is paused (change 0433).
- No Homebrew, no Windows, and no code signing or notarization.

## Evidence

The acceptance evidence for this release is in [`docs/release/v1.0.0-alpha.1/`](https://github.com/danielhanold/docket/tree/main/docs/release/v1.0.0-alpha.1) on `main`. The release rests on ADR-0095, ADR-0096, ADR-0099, ADR-0100, ADR-0102, ADR-0103, and ADR-0104.
