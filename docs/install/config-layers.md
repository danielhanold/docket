# Repo config: `.docket.yml`, `.docket.local.yml`, and which layer wins

By the end of this page you will know how to change docket's behavior without editing its code:
the four places a setting can live and which one wins when two of them disagree, and which settings
are safe to keep on your own machine versus the ones that must be shared with everyone who clones
the repo. docket runs with zero configuration — everything here is optional, reached for only when
a default does not fit.

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
3. **Global** — your cross-repo file at `${XDG_CONFIG_HOME:-$HOME/.config}/docket/config.yml`. This
   machine, every repo on it. See [Global config](global-config.md).
4. **Built-in** — docket's compiled-in defaults, when no layer above sets the key.

The concrete consequence of "resolved independently, per key": if the global file sets one key and
the repo's committed file sets a different key, both take effect — the repo file does not replace
the whole global file, only the one key it names. Where two layers set the *same* key, the higher
one on the list wins outright. Nested blocks such as `finalize:` or `review:` merge key by key the
same way: setting one key in a block leaves its other keys resolving from the layers below.

One block is the exception: the `agents:` model and effort pins are honoured **only** from the
global file, because docket installs agent wrappers for your user rather than per repository. See
[Models](models-and-effort.md).

To see what docket actually resolved, and anything that would block a write, run:

```bash
docket diagnostic config --repo-dir .
```

## The per-repo file: `.docket.yml`

Add a `.docket.yml` to a repository to change docket's defaults for that repo. It is **committed**
(not gitignored) on the repository's **default branch** (`origin/HEAD`), because every clone,
agent, and device needs the same shared values, and the default branch is the one place a skill
(a named, reusable instruction set an agent loads for one job) can reliably find it before any
other configuration has been read.

Every key is optional; an unset key means the built-in default. Common per-repo keys name where
code lands and how the board (the generated overview of every change and its state, never edited
by hand) is rendered — `integration_branch` and `board_surfaces` — both explained where they matter
in [Where the metadata lives](../guide/where-the-metadata-lives.md), plus the finalize-gate switch
`finalize.gate` covered in [Proving the build](../guide/proving-the-build.md), and `agent_harnesses`,
which opts the repository in to a harness's dispatch surfaces (the managed block in `CLAUDE.md` /
`AGENTS.md`, and for Cursor `.cursor/rules/docket-dispatch.mdc`). `agent_harnesses` has no default:
while it is absent, the install touches no repository surface. Set only the keys you want to
change, and copy their shape from `.docket.example.yml` rather than from any snippet — that example
file is the surface the test suite keeps honest against docket's own resolver, and is deliberately
the only place that enumerates every key.

With no `.docket.yml` at all, a repo runs on the built-in defaults. Where the backlog itself is
stored is [Where the metadata lives](../guide/where-the-metadata-lives.md).

## Machine-local overrides: `.docket.local.yml`

A repository's `.docket.local.yml` is an optional, **gitignored** sibling of its committed
`.docket.yml` — an override scoped to both this machine *and* this repo that never leaves the
clone. Reach for it when the value is genuinely yours alone: a local test command, or a way to try
a setting before committing it for the whole team. It accepts every `scope: any layer` key.

Because it never leaves your clone, it deliberately **cannot** set the shared keys (the next
section): those are ignored with a warning here just as they are in the global file, so a
machine-local value can never silently split shared state. Its own path is kept out of git by a
marker-bounded block in the repo's `.gitignore`, written by `docket repository init` or
`docket repository migrate` and checked by `docket repository check`.

## The shared-setting guard

Some keys name **shared** planning state, and a value for them that lived on only one machine would
silently split the backlog across machines. These are the four repository-only keys —
`integration_branch`, `changes_dir`, `adrs_dir`, and `results_dir` — naming the **integration
branch** (the branch code lands on, usually `main`) and the directories on the `docket` branch where
the change files, the ADR ledger, and the results records live. They take effect only in the
committed `.docket.yml`; set globally **or** in a repo's `.docket.local.yml`, they are ignored with
a warning. The reasoning behind the guard is
[Config layers and the shared-setting guard](../concepts/config-layers.md).

## When a config file is misplaced or malformed

- A `~/.config/docket/.docket.yml` is **never read**. The global file is `config.yml`;
  `.docket.yml` is the per-repo name.
- A malformed file, an unknown key, or a bad value in **any** layer makes the whole configuration
  invalid — there is no per-layer fallback — and docket refuses to change the repository until it
  is fixed. A file that exists but cannot be read is a load error.
- `docket diagnostic config --repo-dir .` names the offending file and key.
