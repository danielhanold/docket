# Governing through configuration

By the end of this page you will know how to change docket's behavior without editing its code:
the four places a setting can live and which one wins when two of them disagree, how to rebind any
workflow step to a different tool, and which settings are safe to keep on your own machine versus
the ones that must be shared with everyone who clones the repo. docket runs with zero
configuration — everything here is optional, reached for only when a default does not fit.

Throughout, the exact shape of every key — its spelling, its default value, and the layers it may
be set in — is the shipped `.docket.example.yml`, a copy of every key active at its default with
full documentation inline. This page names each key and says what it is for; for the precise shape
of any one of them, see [Config keys](../reference/config-keys.md), which points at that example
file rather than restating values that could drift.

## The four layers, and which one wins

A setting can be written in up to four places. docket resolves each key **independently** across
them, so you set only the keys you care about in whichever place fits, and everything else falls
through to the shipped default. From strongest to weakest:

1. **Repo-local** — a repository's `.docket.local.yml`. This machine only, this repo only. Wins
   over everything.
2. **Repo-committed** — that repository's committed `.docket.yml`. Applies to every clone of the
   repo, on every machine.
3. **Global** — your cross-repo file at `~/.config/docket/config.yml`. This machine, every repo on
   it.
4. **Built-in** — docket's shipped defaults, when no layer above sets the key.

The concrete consequence of "resolved independently, per key": if the global file sets one key and
the repo's committed file sets a different key, both take effect — the repo file does not replace
the whole global file, only the one key it names. Where two layers set the *same* key, the higher
one on the list wins outright.

Two keys hold a map rather than a single value: `skills:` (rebinds workflow steps, below) and
`agents:` (per-agent model and effort, covered in
[Running on your harness](./running-on-your-harness.md)). These **merge field by field** with the
same precedence, so a global default and a repo override can each set different fields of the same
map and both survive.

One historical key is worth knowing because you may still see it: `runtime.bash` named the path to
a machine's Bash and was the deliberate exception to the precedence above — a machine-identity
value resolved repo-local over global, with a committed value warned and ignored. That key is now
**obsolete** (the Bash runtime it named was retired) and is warned-and-ignored in every layer, but
the local-over-global rule it illustrated still governs any future machine-identity key.

## The per-repo file: `.docket.yml`

Add a `.docket.yml` to a repository to change docket's defaults for that repo. It is **committed**
(not gitignored) on the repository's **default branch** (`origin/HEAD`), because every clone,
agent, and device needs the same shared values, and the default branch is the one place a skill
(a named, reusable instruction set an agent loads for one job) can reliably find it before any
other configuration has been read.

Every key is optional; an unset key means the shipped default. Common per-repo keys name where
planning and code live and how the board (the generated overview of every change and its state,
never edited by hand) is rendered — `metadata_branch`, `integration_branch`, and `board_surfaces`
— all of which are explained where they matter in
[Where the metadata lives](./where-the-metadata-lives.md), plus the merge-gate switch
`finalize.gate` covered in [Proving the build](./proving-the-build.md). Set only the keys you want
to change, and copy their shape from `.docket.example.yml` rather than from any snippet — that
example file is the surface the test suite keeps honest against docket's own resolver, and is
deliberately the only place that enumerates every key.

With no `.docket.yml` at all, a repo runs in docket's default two-branch mode. What that mode is,
and how to opt out of it, is [Where the metadata lives](./where-the-metadata-lives.md).

## Workflow roles — the `skills:` map

docket is a thin lifecycle layer wrapped around a workflow engine, and each of its **five workflow
invocation points is a pluggable role**. The optional `skills:` map, in any config layer, rebinds a
role either to a different skill (its name is handed to the harness verbatim) or to the sentinel
`auto`, which means "no skill — the running agent (a separately launched worker with its own
context, pinned to a model and effort) does the step inline at its own model."

| Role | Default | Where it runs |
|---|---|---|
| `brainstorm` | `superpowers:brainstorming` | up-front design, before the spec |
| `plan` | `superpowers:writing-plans` | the task plan built from the spec |
| `build` | `docket-build` | executing the plan task by task |
| `review` | `docket-review` | whole-branch review before the pull request |
| `finish` | `superpowers:finishing-a-development-branch` | pushing the branch and opening the pull request |

Three roles default to the superpowers engine; docket owns `build` and `review` itself. To run the
superpowers engine everywhere, bind `build` and `review` explicitly to their superpowers
equivalents. If a bound skill cannot be invoked at runtime — superpowers is not installed, or a
custom name is misspelled — docket **degrades to that role's `auto` fallback with a prominent
warning**, so a repository without superpowers still works out of the box.

One binding carries a caveat worth knowing before you set it: **`build: auto` is dual-purpose.** It
authorizes inline building *and* the in-branch fix workers docket runs on review findings, which
execute the build role's own contract — there is deliberately no separate key for those fix
workers. The full shape of the map, the `auto` sentinel, and each role's fallback are documented
once in the `docket-convention` skill's *Skill layer* section; consult it there rather than copying
examples, and see [Config keys](../reference/config-keys.md) for where the shape is owned.

## Cross-repo defaults: global config

Settings you want on **every** repo on your machine go in one optional user-level file,
`~/.config/docket/config.yml` (more precisely `${XDG_CONFIG_HOME:-~/.config}/docket/config.yml`).
It accepts the **same schema as `.docket.yml`**, and a repository's committed `.docket.yml` wins
over it per key. This is where a personal preference that spans your work belongs — a default
`skills:` binding, your per-agent model and effort choices, a taxonomy of change types you use
across projects — rather than repeating it in every repo. Its full accepted key set is the
global-able subset documented in `.docket.example.yml`.

## Machine-local overrides: `.docket.local.yml`

A repository's `.docket.local.yml` is an optional, **gitignored** sibling of its committed
`.docket.yml` — an override scoped to both this machine *and* this repo that never leaves the
clone. Reach for it when the value is genuinely yours alone: a personal model preference, a local
test command, or a way to try a setting before committing it for the whole team. It accepts the
same **global-able** key set as the global file above.

Because it never leaves your clone, it deliberately **cannot** set the shared, coordination keys
(the next section): those are warned-and-ignored here just as they are in the global file, so a
machine-local value can never silently split shared state. Its own path, and every file docket's
installer generates, is kept out of git by a marker-bounded block the installer maintains in the
repo's `.gitignore`.

## The coordination fence

Some keys write **shared** state, and a value for them that lived on only one machine would
silently split the backlog across machines or mint external objects that others cannot see. These
are **coordination keys** — a coordination key being a config key whose value must be identical for
every clone, so it may only be set in the committed repo config. They are ignored, with a loud
warning, when set either globally **or** in a repo's `.docket.local.yml`; they take effect only in
the committed `.docket.yml`.

The fenced keys are `metadata_branch`, `integration_branch`, `changes_dir`, `adrs_dir`,
`results_dir`, `github_project`, `terminal_publish`, and the `github` token of `board_surfaces` —
each naming either the **metadata branch** (the `docket` git branch where the backlog, specs, and
decisions are stored, separate from the code), the **integration branch** (the branch code lands
on, usually `main`), a shared directory, or an external GitHub object. The reasoning behind the
fence — why a per-clone value here would corrupt shared state — is
[Config layers and the coordination fence](../concepts/config-layers.md).

## When a config file is misplaced or malformed

docket fails soft on a bad or misplaced file rather than bricking a repo:

- A `~/.config/docket/.docket.yml` is **never read** — the resolver warns and points you at the
  correct name, `config.yml`. (The global file is `config.yml`; `.docket.yml` is the per-repo
  name.)
- A malformed or unreadable `config.yml` (or `.docket.local.yml`) warns and falls back to the
  built-in defaults **for that layer only**. The repo's own committed file and every other layer
  are still honored, so a broken personal or machine-local file never takes a working repository
  down with it.

## Migrating from `agents.yaml`

Older docket kept per-agent settings in a separate global file, `~/.config/docket/agents.yaml`.
That file is migrated automatically: the next install run rewrites its contents under the `agents:`
key of `config.yml` and renames the original to `agents.yaml.migrated`. Nothing reads the old file
after the migration, so there is no manual step — the first install after upgrading does it for
you.
