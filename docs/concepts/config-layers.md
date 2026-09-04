# Config layers and the coordination fence

## The problem it solves

Docket runs on more than one machine and inside more than one person's
checkout of the same repository. Some settings have to be identical in
every one of those checkouts or the tool quietly breaks: if two clones
disagree about which git branch holds the backlog, one of them writes its
planning state to a branch the other never reads, and the shared backlog
splits in two without anyone noticing. Other settings are personal —
which model you are willing to pay for, which local bash to run — and
forcing them on a teammate by storing them in the shared repository would
be just as wrong.

One flat config file cannot serve both needs at once. Docket instead
resolves configuration from several ordered **layers**, each a file at a
known location, and draws a fence through them. A **coordination key** — a
config key whose value must be identical for every clone, so it may only
be set in the committed repo config — is honored from the shared file and
nowhere else; a personal value lives in a layer that is never committed
and never reaches anyone else.

## The moving parts

Four layers resolve per key, from lowest precedence to highest. A later
layer overrides an earlier one for the keys it sets — except where the
fence forbids it.

```
 lowest precedence ─────────────────────────────────► highest precedence

 shipped defaults     global user config    committed repo    machine-local
 (built into the      ~/.config/docket/     .docket.yml       .docket.local.yml
  installed tool)     config.yml                              (gitignored)

 every repo,          every repo,           this repo,        this repo,
 every machine        THIS machine          EVERY clone       THIS machine
 (baseline)           (personal)            (SHARED)          (personal)

           ┌───────────────── the fence ──────────────────┐
           │ a coordination key is read ONLY from           │
           │ committed .docket.yml; the same key set in     │
           │ the global or machine-local layer is ignored   │
           │ with a loud warning                            │
           └────────────────────────────────────────────────┘
```

- **Shipped defaults** are the baseline built into the installed tool —
  for example the model and effort each generated **agent** (a separately
  launched worker with its own context, pinned to a model and effort)
  falls back to, kept in a sidecar indexed by **harness** (the tool that
  runs the agent: Claude Code, Cursor, Codex, or opencode).
- **Global user config** (`~/.config/docket/config.yml`) carries your
  cross-repo personal defaults on this machine; it takes the same schema
  as the repo config, and a repo's committed `.docket.yml` wins over it
  per key.
- **Committed `.docket.yml`** is the shared, version-controlled repo
  config — the only place a coordination key may be set.
- **Machine-local `.docket.local.yml`** is a gitignored sibling of
  `.docket.yml`: the highest-precedence layer, but scoped to this one
  clone, so it can override a personal key without ever touching what the
  team sees — and it still cannot override a coordination key.

The fenced keys today are `metadata_branch`, `integration_branch`,
`changes_dir`, `adrs_dir`, `results_dir`, `github_project`,
`terminal_publish`, and the `github` token of `board_surfaces`.

## The invariants

- The four layers resolve per key, lowest to highest: shipped defaults,
  global user config, committed `.docket.yml`, machine-local
  `.docket.local.yml`; a later layer wins per key.
- A coordination key is honored only from the committed `.docket.yml`; the
  same key set in the global or machine-local layer is ignored with a
  warning, so no personal layer can split the backlog across machines or
  mint stray external objects.
- The machine-local layer is gitignored and never committed, so a personal
  override cannot leak onto a teammate.
- A malformed or unreadable personal layer falls back to built-ins for
  that layer only; the repo's other layers are still honored, so a broken
  personal file never bricks a repo.
- A documented config key is read through the config resolver, never by a
  raw read of `.docket.yml`, so the layering and the fence are always
  applied.
- The global file must be named `config.yml`; a `~/.config/docket/.docket.yml`
  is never read.

## Decided in

- [ADR-0019](../adrs/0019-global-config-fence-classification.md) — set the
  coordination-key fence classification rule that decides which keys may
  live only in the committed layer.
- [ADR-0016](../adrs/0016-harness-first-agent-config.md) — made agent
  model and effort resolve per harness with a field-level default
  fallback.
- [ADR-0020](../adrs/0020-generated-agent-artifacts-machine-local.md) —
  added `.docket.local.yml` as the machine-local layer that completes the
  four-layer config, keeping generated agent artifacts machine-local.
- [ADR-0052](../adrs/0052-config-key-resolution-boundary.md) — required a
  documented config key to resolve through the resolver, ruling a raw
  model-read of `.docket.yml` unsupported.
- [ADR-0064](../adrs/0064-shipped-agent-defaults-live-in-a-harness-indexed-sidecar.md)
  — moved the shipped model and effort defaults into a harness-indexed
  sidecar as the baseline layer (supersedes ADR-0048).
