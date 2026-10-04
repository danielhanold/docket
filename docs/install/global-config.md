# Global config: machine-wide defaults in `~/.config/docket/config.yml`

Settings you want on **every** repo on your machine go in one optional user-level file,
`~/.config/docket/config.yml` (more precisely `${XDG_CONFIG_HOME:-$HOME/.config}/docket/config.yml`).
A repository's committed `.docket.yml` and its `.docket.local.yml` both win over it per key; how
the layers rank is [Repo config](config-layers.md).

Nothing creates or maintains this file for you: docket's built-in defaults apply until you write
one, so a Claude-Code-only user can skip this page entirely.

Two kinds of setting belong here:

- **Your per-agent model and effort pins.** The `agents:` block is honoured **only** from this file
  — docket installs agent wrappers for your user, not per repository, so the pins are a property of
  your machine. The built-in values are compiled into docket and mirrored, value for value, in
  [`agents/harness-defaults.yml`](../../agents/harness-defaults.yml), a shipped file you read but
  never edit. To change one, see [Models](models-and-effort.md), then re-run the install.
- **Any `scope: any layer` key you want as your personal default** across repositories — for
  example `review.min_fix_severity` or `reclaim.lease_ttl`. A repository's own files still win.

The canonical reference for every key is [`.docket.example.yml`](../../.docket.example.yml): every
supported key at its built-in default, with a short note and a scope tag saying which layers may
set it. Copy only the keys you want to change. The four `scope: repo-only` keys
(`integration_branch`, `changes_dir`, `adrs_dir`, `results_dir`) are ignored here with a warning.

`agent_harnesses` does not belong here: the installer ignores a value in the global config.
Enabling a harness for a repository is that repository's own decision, made in its `.docket.yml` or
`.docket.local.yml`; each harness page ([Cursor](cursor.md), [Codex](codex.md),
[opencode](opencode.md)) covers what that opt-in writes.

If the file is not behaving, check its name — the global file is `config.yml`, and
`~/.config/docket/.docket.yml` is never read — and run `docket diagnostic config --repo-dir .`
from any repository to see the resolved configuration. A malformed global file makes the whole
configuration invalid, as described under *When a config file is misplaced or malformed* in
[Repo config](config-layers.md).
