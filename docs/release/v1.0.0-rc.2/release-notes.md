## What this release is

The second release candidate for `v1.0.0`, published as a full release. It replaces `v1.0.0-rc.1` as "Latest" and carries everything merged since rc.1 (267 commits, about 45 changes).

## Harness status

- **Claude Code** and **Cursor**: supported. Cursor runs docket's subagents at their pinned models.
- **OpenCode**: installs and ships, but is **untested**. Full support comes after `v1.0.0`.
- **Codex**: **unsupported**.

## Install

```sh
VERSION=v1.0.0-rc.2
curl -fsSLO "https://github.com/danielhanold/docket/releases/download/$VERSION/install.sh"
curl -fsSLO "https://github.com/danielhanold/docket/releases/download/$VERSION/checksums.txt"
grep '  install.sh$' checksums.txt | shasum -a 256 -c -
sh install.sh --version "$VERSION" --harness claude --harness cursor
```

The `shasum` line must print `install.sh: OK`; if it prints anything else, do not run `install.sh`. Drop the `--harness` you don't use. The installer puts `docket` in `~/.local/bin`. Supported platforms: macOS and Linux, on amd64 and arm64. You need `sh`, `curl`, `tar`, and a SHA-256 tool (`shasum` or `sha256sum`). Restart Claude Code and Cursor after installing. For Cursor, run docket outside the sandbox (see [Cursor setup](https://github.com/danielhanold/docket/blob/main/docs/install/cursor.md)). Install with the script, not a browser download (see Known gaps).

## Upgrading

**From rc.1 or an alpha:**

1. Let any docket run that is still going finish, or stop it with `docket run cancel`, before upgrading. The run tracker changed since rc.1, and a run that spans the upgrade would mix old and new instructions.
2. Re-run the install commands above, then restart Claude Code and Cursor.
3. In each **shared** repository that uses docket, run `docket install --repo-dir <repo>`, then commit what it changed (`AGENTS.md`, `CLAUDE.md` or `.cursor/rules/`). The dispatch instructions gained a rule for loops that carry on past a halt. Private repositories need nothing: their instructions come from the binary.
4. What you will notice:
   - `reclaim.lease_ttl` and `reclaim.auto` in a config file now produce a warning and are ignored; delete them.
   - `docket change reclaim` is gone; resume a stranded change by id.
   - An unbuilt change marked `trivial: true` with no spec now reads as needs-grooming; groom it to give it a spec.
   - In Claude Code, a typed `/docket-implement-next` starts an untracked run; start a tracked run with a plain request or `/loop`.

**From Bash `v0.9.x`:** follow [Upgrading from Bash docket](https://github.com/danielhanold/docket/blob/main/docs/release/upgrading-from-bash.md), which covers Claude Code and Cursor and has no OpenCode section yet. The Bash tags `v0.9.2` and `v0.9.3` remain available.

## What changed since rc.1

**Using docket**
- **Open and export** (0561, 0578): `docket open --export` writes a clean, docket-free Markdown copy of a change, spec, plan or results file, checks it for leftovers, and opens it; the new `docket-export` skill makes agent sessions use it.
- **Hold a change out of loops** (0560): a groomed change can be put on hold so drains skip it; naming it still builds it.
- **Loops carry on past a halt** (0570): a halt that concerns only one queue-picked change no longer ends the loop, up to `run.max_consecutive_halts` (default 2); `docket run reset-halts` sets the count back.
- **Close a change without a PR** (0559): a change can deliver a results artifact instead of a PR.
- **Finalize under auto mode** (0552): `docket install` grants the Claude Code allow rules finalize needs to publish and merge.
- **Simpler grooming** (0573): `trivial` is retired; every build-ready change has a spec.
- **Claim reclaim removed** (0575): no claim lease and no reclaim; recover a stranded run by resuming it by id.
- **Typed `/docket-implement-next`** (0556) starts an untracked run.
- **Clearer findings** (0553): repository check and status findings explain themselves in plain language.
- **Groom and stacks** (0550, 0563, 0564, 0565, 0567, 0568, 0571): groom can set priority and clear `stacked_on`; stack-parent kills and deferrals are refused when they would strand built children; a landed parent closes out without waiting on children that never merged into it.
- **Other** (0549, 0551, 0554, 0555, 0562): the local metadata checkout is refreshed after every write; a change built on another machine can be checked out into its own worktree; shipped model defaults refreshed; Claude's dispatch block stays in `AGENTS.md` with `CLAUDE.md` linking to it; sweeps retry deleting a leftover local branch.

**Fixes and internal work:** the docket branch is recognized by its shape in everyday repository commands (0574); implement-next no longer trips over its own bookkeeping (0158); prompt-cache TTL pin for implement-next (0546, 0547); test-suite housekeeping, fixture naming, and shard budgets (0548, 0557, 0558, 0569, 0577).

## Known gaps

- Cursor's sandboxed mode ("Allowlist (with Sandbox)") was not exercised.
- Quitting Cursor in the middle of a docket run can leave a `cursor-agent` worker running; check for one before resuming.
- OpenCode is untested; full support comes after `v1.0.0`. Codex is unsupported.
- The binaries are not notarized: an archive downloaded in a browser and extracted in Finder is blocked by macOS Gatekeeper. Use the install script, or run `xattr -d com.apple.quarantine docket`.
- No Homebrew or Windows.

## Evidence

The release evidence is in [`docs/release/v1.0.0-rc.2/`](https://github.com/danielhanold/docket/tree/main/docs/release/v1.0.0-rc.2) on `main`. ADRs accepted since rc.1: ADR-0149 through ADR-0160 (ADR-0159 was superseded by ADR-0160).
