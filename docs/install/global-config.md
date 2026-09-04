# Global config: machine-wide defaults in `~/.config/docket/config.yml`

Settings you want on **every** repo on your machine go in one optional user-level file,
`~/.config/docket/config.yml` (more precisely `${XDG_CONFIG_HOME:-~/.config}/docket/config.yml`).
It accepts the **same schema as `.docket.yml`**, and a repository's committed `.docket.yml` wins
over it per key. This is where a personal preference that spans your work belongs — a default
`skills:` binding, your per-agent model and effort choices, a taxonomy of change types you use
across projects — rather than repeating it in every repo. Its full accepted key set is the
global-able subset documented in `.docket.example.yml`; how this file ranks against the repo's own
files is [Repo config](config-layers.md).

The installer writes a minimal `config.yml` the first time it runs and non-destructively maintains
its managed values. docket's ordinary defaults already apply, so a Claude-Code-only user can stop
here.

The canonical reference for every key is [`.docket.example.yml`](../../.docket.example.yml): every
config key, active at its shipped default, with full documentation and a scope tag saying which
layers may set it. Copy the keys you want to change into the layer you want them in.

- **To see docket's built-in per-skill model and effort:** they all live in
  [`agents/harness-defaults.yml`](../../agents/harness-defaults.yml) — docket's shipped,
  harness-indexed default sidecar, not a file you edit. All four of the example's commented harness
  blocks — `agents.claude`, `agents.cursor`, `agents.codex`, and `agents.opencode` — mirror it in
  full, value for value. To change one, see [Models](models-and-effort.md).
- **To enable another harness (Cursor, Codex, opencode):** add it to `agent_harnesses` and re-run
  `install.sh`; the Go engine reconciles that harness's wrappers and dispatch surfaces for you.
  Leave the harness's `agents:` block commented, since it only restates the shipped defaults and
  uncommenting it would freeze today's values into your config forever. `agent_harnesses` is the
  **explicit opt-in** for a repository's parent-facing dispatch surfaces, and it has **three
  states**: *absent* leaves the shipped default (Claude only) in force and writes no other harness's
  repository surfaces; a *non-empty* list reconciles exactly the harnesses you name; and an
  *explicit empty* list (`agent_harnesses: []`) retires every docket-owned repository surface the
  repo previously had. An absent key touches nothing — only an explicit value reconciles or retires.
  Each harness page ([Cursor](cursor.md), [Codex](codex.md), [opencode](opencode.md)) covers what
  its opt-in writes and where the repo itself must also opt in.

Two things to know if the file is not behaving: a `~/.config/docket/.docket.yml` is never read
(the global file is `config.yml`), and an older `~/.config/docket/agents.yaml` is migrated into it
automatically. Both are covered under *When a config file is misplaced or malformed* in
[Repo config](config-layers.md).
