# Upgrading from Bash docket

This guide moves a Bash docket install, and the repositories it manages, to the docket binary.

An automated test runs every marked step below against saved copies of real `v0.9.2` and `v0.9.3`
installs. When something is outside that test, the guide says so.

## 1. Who this is for

Use this guide if all of these are true:

- You installed Bash docket from the `v0.9.2` or `v0.9.3` tag.
- Your repositories keep their docket records on a separate `docket` branch.
- You use docket with Claude Code.

Repositories that keep their records on the default branch (the single-branch layout) are not
covered. The guide covers Claude Code only. Sections for Cursor and OpenCode arrive in later
pre-releases. Codex is not supported.

## 2. Before you start

- In each repository, finish or defer every change that is in progress. The test covers
  repositories with no change in progress.
- In each repository, check out the default branch and make sure `git status` shows no changes.
  The steps below commit and push to the default branch.

## 3. Install the docket binary

Run these commands outside any repository. They download the installer and the checksum file into
a new folder, check the installer against the checksum, then run it for Claude Code.

<!-- upgrade-step: install-binary -->
```sh
mkdir -p ~/docket-download && cd ~/docket-download
VERSION=v1.0.0-alpha.1
curl -fsSLO "https://github.com/danielhanold/docket/releases/download/$VERSION/install.sh"
curl -fsSLO "https://github.com/danielhanold/docket/releases/download/$VERSION/checksums.txt"
grep '  install.sh$' checksums.txt | shasum -a 256 -c -
sh install.sh --harness claude
```

The `shasum` line must print `install.sh: OK`. If it prints anything else, stop and do not run
`install.sh`.

The installer puts `docket` in `~/.local/bin`. Make sure that folder is on your `PATH`.

On a Bash install, this first run stops with `install: invalid-state` and a list of `conflict`
lines. That is expected. The run changes nothing and leaves no `docket` command behind. Section 4
clears the conflicts.

If you build docket from a source checkout instead, see [Installing docket](../install/install.md).
That route needs Go and is not part of this guide.

## 4. Take over the old Claude Code install

The installer takes over the agent files Bash docket wrote under `~/.claude/agents/` by itself, as
long as they still hold exactly what the Bash installer wrote.

It takes over nothing else. Each path it will not touch is printed as a `conflict` line that tells
you to move or delete it. For a Bash `v0.9.2` or `v0.9.3` install, that list is:

- every `docket-*` link under `~/.claude/skills/`. Each one points into your old Bash checkout.
  Deleting a link leaves the checkout alone.
- on `v0.9.3` only, `~/.claude/agents/docket-plan-writer.md`.

Delete them, then run the installer again from the same folder. The failed run left no `docket`
command, so run `sh install.sh` again, not `docket install`.

<!-- upgrade-step: takeover-remedy -->
```sh
rm ~/.claude/skills/docket-*
rm -f ~/.claude/agents/docket-plan-writer.md
sh install.sh --harness claude
```

This time the installer reports `install: applied`.

Bash docket also left a `runtime.bash` setting in your global config,
`~/.config/docket/config.yml`. docket warns about it and ignores it. Delete these lines from that
file. The path on the `bash:` line is whatever your Bash install recorded.

<!-- upgrade-step: global-config-cleanup -->
```yaml
# >>> docket (runtime.bash) >>>
runtime:
  bash: '/opt/homebrew/bin/bash'
# <<< docket (runtime.bash) <<<
```

Then confirm the install. Both commands succeed, and `docket install check` prints no warning.

<!-- upgrade-step: confirm-install -->
```sh
docket version
docket install check
```

## 5. Upgrade each repository

Do this in every repository that has a `docket` branch. Replace `<repo>` with the repository's
folder.

<!-- upgrade-step: repo-prepare -->
```sh
cd <repo>
docket repository prepare
```

It reports `healthy` and keeps using the `.docket` folder Bash docket made. That does not mean the
repository is ready yet. The next command finds what still needs fixing.

<!-- upgrade-step: repo-check -->
```sh
docket repository check
```

On a Bash repository it exits with an error and lists some of these findings:

| Finding | What it means | Fix |
|---|---|---|
| `postconditions-unmet` | A summary: at least one of the other checks failed. | Clears once the others are fixed. |
| `committed-ignore-invalid` | The docket block in `.gitignore` is the Bash version. | Replace the block, below. |
| `legacy-config-key-present` | `.docket.yml` still sets `metadata_branch`. | Remove the setting, below. |
| `test-config-missing` | No test command is set up for docket's build checks. | `docket repository configure-tests`, below. |
| `board-stale` | `BOARD.md` on the `docket` branch is in the Bash format. | `docket repository repair`, below. |
| `artifact-links-stale` | A change record's links block is in the Bash format. | `docket repository repair`, below. |

Start with `.gitignore`. Replace everything from its `# docket:start` line to its `# docket:end`
line with exactly these lines, which `docket repository check` also prints:

<!-- upgrade-step: fix-gitignore -->
```text
# docket:start (managed by docket — do not hand-edit)
.docket/
.worktrees/
.claude/settings.local.json
.docket.local.yml
.claude/agents/docket-*.md
.codex/agents/docket-*.md
.cursor/agents/docket-*.md
.opencode/agents/docket-*.md
.codex/agents/docket-*.toml
.cursor/rules/docket-dispatch.mdc
# docket:end
```

Next, remove the settings docket no longer accepts. `docket status` lists each one as an error or
a warning, with a remedy. Remove each of them from `.docket.yml`. Section 6 lists every one. Settings
that show up only as notices can stay. If the repository has no `.docket.yml`, there is nothing to
remove.

<!-- upgrade-step: config-cleanup -->
```sh
docket status
```

`docket status` reads `.docket.yml` from the pushed default branch, not from your working copy. Its
list clears only after you commit and push. Commit and push both files:

<!-- upgrade-step: commit-fixes -->
```sh
git commit -am "Update docket's .gitignore block and settings"
git push
```

Now set up the test command. `docket repository configure-tests` writes the test settings into
`.docket.yml`. Review them, then commit and push.

<!-- upgrade-step: configure-tests -->
```sh
docket repository configure-tests
```

<!-- upgrade-step: commit-config -->
```sh
git add .docket.yml
git commit -m "Configure docket's test command"
git push
```

The last fix is for the records on the `docket` branch. First look at what will change: the command
lists the files it would rewrite and asks `repair? [y/N]`. Answer `N`.

<!-- upgrade-step: repair-preview -->
```sh
docket repository repair
```

Then apply the repairs. docket commits and pushes them to the `docket` branch itself.

<!-- upgrade-step: repair-apply -->
```sh
docket repository repair --yes
```

It then tells you to run `docket repository prepare` to bring your local `.docket` folder up to
date. Until you do, `docket repository check` reports `metadata-worktree-dirty` and
`local-metadata-diverged`. Both clear after `prepare`. Run it and check again:

<!-- upgrade-step: repo-confirm -->
```sh
docket repository prepare
docket repository check
```

`docket repository check` now reports `healthy`.

## 6. Settings that changed

These are the Bash-era settings docket reports, in `.docket.yml` or in
`~/.config/docket/config.yml`. `<harness>`, `<agent>`, `<role>` and `<setting>` stand for any name.

| Setting | What docket does | Fix |
|---|---|---|
| `metadata_branch` | Warning: ignores it. `docket repository check` also reports it as an error. | Remove it. |
| `runtime.bash` | Warning: ignores it. | Remove it. |
| `terminal_publish` | Error when set to `true`. | Remove it, or set it to `false`. |
| `auto_groom` | Error when set to `true`. | Remove it, or set it to `false`. |
| `build.checkpoint` | Error when set to `true`. | Remove it, or set it to `false`. |
| `finalize.skip_results_only_delta` | Error when set to `true`. | Remove it, or set it to `false`. |
| `auto_capture.enabled` | Error when set to `true`. | Remove it, or set it to `false`. |
| `dummy_mode.enabled` | Error when set to `true`. | Remove it, or set it to `false`. |
| `finalize.gate` | Error when set to `ci`. | Remove it, or set it to `local` or `off`. |
| `board_surfaces` | Error when the list holds `github`. | Remove it, or remove `github` from the list. |
| `skills.<role>` | Error. | Remove it. |
| `agents.<harness>.<agent>.runner` | Error. | Remove it. |
| `agents.<harness>.<agent>.model`, `agents.<harness>.<agent>.effort` | Error in `.docket.yml`. | Remove it, or move it to `~/.config/docket/config.yml`. |
| `learnings.cap`, `github_project`, `delegation_observation_budget` | Notice: kept, but has no effect. | Nothing needed. You can remove it. |
| `auto_capture.types`, `dummy_mode.persona`, `dummy_mode.surfaces`, `runners.codex.<setting>`, `runners.opencode.<setting>` | Notice: read only once the setting it belongs to is available. | Nothing needed. You can remove it. |

## 7. Leftovers you can delete

The Bash installer also left these behind. docket does not use them, and `docket install check`
does not mention them. Delete them:

<!-- upgrade-step: leftovers -->
```text
~/.claude/settings.json  the "DOCKET_SCRIPTS_DIR" and "DOCKET_BASH_PATH" entries under "env"
~/.zshenv  the lines from "# >>> docket (DOCKET_SCRIPTS_DIR) >>>" to "# <<< docket (DOCKET_SCRIPTS_DIR) <<<"
```

The test covers zsh, where those lines are in `~/.zshenv`. Other shells keep them in a different
startup file, which the test does not cover.

Your old Bash checkout, usually `~/dev/docket`, is no longer used by Claude Code: nothing under
`~/.claude` points into it after the upgrade. The links Bash docket made for other tools, under
`~/.cursor`, `~/.codex` and `~/.agents`, still do. Keep the checkout while you use those tools with
Bash docket.

This guide does not cover two more things:

- Agent files under `.claude/agents/docket-*.md` and a `.claude/settings.local.json` inside a
  repository, which older Bash versions wrote. The saved installs have none, so the test does not
  cover them. The new `.gitignore` block keeps both out of commits.
- The docket block in a repository's `CLAUDE.md`, between the `docket:dispatch:start` and
  `docket:dispatch:end` lines. The upgrade leaves it as it is, and `docket repository check` does
  not report it. The guide does not cover changing it.

## 8. Restart Claude Code

Quit Claude Code and start it again. Claude Code loads agents and skills when it starts, so
clearing a conversation is not enough.

## 9. If something goes wrong

Open an issue at <https://github.com/danielhanold/docket/issues> with the command you ran and its
full output.

The Bash tags `v0.9.2` and `v0.9.3` remain available.
