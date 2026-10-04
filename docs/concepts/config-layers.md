# Config layers and the shared-setting guard

## The problem it solves

Docket runs on more than one machine and inside more than one person's
checkout of the same repository. Some settings have to be identical in
every one of those checkouts or the tool quietly breaks: if two clones
disagree about which branch is the integration branch, or where change
files live, one of them writes work the other never sees, and the shared
backlog splits in two without anyone noticing. Other settings are
personal — which model you are willing to pay for, how long a gate run
may go quiet — and forcing them on a teammate by storing them in the
shared repository would be just as wrong.

One flat config file cannot serve both needs at once. Docket instead
resolves configuration from several ordered **layers**, each a file at a
known location, and draws a guard through them. A **repository-only key**
— a config key whose value must be identical for every clone, so it may
only be set in the committed repository config — is honored from the
shared file and nowhere else; a personal value lives in a layer that is
never committed and never reaches anyone else.

## The moving parts

Four layers resolve per key, from lowest precedence to highest. A later
layer overrides an earlier one for the keys it sets — except where the
guard forbids it.

```
 lowest precedence ─────────────────────────────────► highest precedence

 built-in defaults    global user config    committed repo    machine-local
 (compiled into the   ${XDG_CONFIG_HOME:-   .docket.yml       .docket.local.yml
  docket binary)      $HOME/.config}/                         (gitignored)
                      docket/config.yml

 every repo,          every repo,           this repo,        this repo,
 every machine        THIS machine          EVERY clone       THIS machine
 (baseline)           (personal)            (SHARED)          (personal)

           ┌────────── the shared-setting guard ──────────┐
           │ a repository-only key is read ONLY from        │
           │ committed .docket.yml; the same key set in     │
           │ the global or machine-local layer is ignored   │
           │ with a warning                                 │
           └────────────────────────────────────────────────┘
```

- **Built-in defaults** are the baseline compiled into the `docket`
  binary — for example the model and effort each **agent** (a separately
  launched worker with its own context, pinned to a model and effort)
  falls back to, kept in a table indexed by **harness** (the tool that
  runs the agent: Claude Code, Cursor, Codex, or OpenCode).
- **Global user config**
  (`${XDG_CONFIG_HOME:-$HOME/.config}/docket/config.yml`) carries your
  cross-repo personal defaults on this machine; it takes the same schema
  as the repository config, and a repository's committed `.docket.yml`
  wins over it per key. It is also the only layer that may override an
  agent's model and effort (`agents.<harness>.<agent>.model` /
  `effort`). Nothing writes or maintains this file for you.
- **Committed `.docket.yml`** is the shared, version-controlled repository
  config — the only place a repository-only key may be set.
- **Machine-local `.docket.local.yml`** is a gitignored sibling of
  `.docket.yml`: the highest-precedence layer, but scoped to this one
  clone, so it can override a personal key without ever touching what the
  team sees — and it still cannot override a repository-only key.

The repository-only keys are `integration_branch`, `changes_dir`,
`adrs_dir`, and `results_dir`.

To see what the layers resolved to, run:

```
docket diagnostic config --repo-dir .
```

It prints each effective value with the layer and line that won, and
anything that blocks docket from writing.

## The invariants

- The four layers resolve per key, lowest to highest: built-in defaults,
  global user config, committed `.docket.yml`, machine-local
  `.docket.local.yml`; a later layer wins per key.
- A repository-only key is honored only from the committed `.docket.yml`;
  the same key set in the global or machine-local layer is ignored with a
  warning, so no personal layer can split the backlog across machines.
- Agent model and effort overrides belong in the global file only. One set in
  the committed or machine-local layer is not ignored: it makes docket refuse
  to change the repository until it is moved to the global file.
- The machine-local layer is gitignored and never committed, so a personal
  override cannot leak onto a teammate.
- A malformed file, an unknown key, or a bad value in any layer makes the
  whole configuration invalid — there is no per-layer fallback — and an
  unreadable file is a load error. The diagnostic names the file and the
  key, so the fix is never a guess.
- A documented config key is read through docket's config resolver, never
  by a raw read of `.docket.yml`, so the layering and the guard are always
  applied.
- The global file must be named `config.yml`; no other file in that
  directory is read.

## Decided in

- [ADR-0019](../adrs/0019-global-config-fence-classification.md) — set the
  rule that decides which keys are repository-only: a key whose effect
  writes state every clone shares.
- [ADR-0016](../adrs/0016-harness-first-agent-config.md) — made agent
  model and effort resolve per harness with a field-level default
  fallback.
- [ADR-0020](../adrs/0020-generated-agent-artifacts-machine-local.md) —
  added `.docket.local.yml` as the machine-local layer that completes the
  four layers.
- [ADR-0052](../adrs/0052-config-key-resolution-boundary.md) — required a
  documented config key to resolve through the resolver, ruling a raw
  read of `.docket.yml` unsupported.
- [ADR-0064](../adrs/0064-shipped-agent-defaults-live-in-a-harness-indexed-sidecar.md)
  — indexed the built-in model and effort defaults by harness and made
  them the lowest layer.
