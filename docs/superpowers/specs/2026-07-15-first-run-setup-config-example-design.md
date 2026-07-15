# First-run setup: a committed starter config + a scaffolding install step

**Change:** 0081 — first-run-setup-config-example
**Date:** 2026-07-15
**Status:** design (build-ready)

## Problem

After a first `git clone` + machine install, docket's setup is not obvious:

1. **There is no visible next step.** `install.sh` links skills, generates agent
   wrappers, and exports `DOCKET_SCRIPTS_DIR`, but a first-time user is left without a
   clear "now do this" — especially the global `~/.config/docket/config.yml` that
   selects harnesses and tunes per-skill models.
2. **The default configuration is invisible.** docket resolves per-harness, per-skill
   model/effort defaults during bootstrap, but the first user cannot *see* what those
   defaults are, nor discover the shape they'd edit to change them. The values are
   real (shipped in each `agents/docket-*.md` wrapper frontmatter) but effectively
   hidden.

This change makes first-run setup a short, explicit sequence and makes the default
configuration a concrete, editable artifact.

## Goals

- Ship a committed, copy-me starter file that documents the default configuration and
  the shape a user edits to enable harnesses and tune models.
- Have `install.sh` drop that starter into place on first run (non-destructively), so
  the global config exists and is discoverable without a manual copy.
- Restructure the README's Install section from a single "one-line install" into a
  short numbered setup sequence that names the config step.

## Non-goals

- No changes to how config is *resolved* (`docket-config.sh`, the four-layer
  precedence, the coordination-key fence) — this change only adds a starter artifact
  and a scaffolding step.
- No new config keys, and no change to `sync-agents.sh` generation behavior.
- Not a full config reference in the starter file — it points at README →
  Configuration for the complete schema.

## Deliverable 1 — `config.yml.example` (committed, repo root)

A new file at the docket repo root, `config.yml.example`, is the template for the
global `~/.config/docket/config.yml`. It has three parts:

### 1a. `agent_harnesses` — active

```yaml
agent_harnesses: [claude]   # add cursor / codex here to enable those harnesses
```

Shipped active with `[claude]` (the default). A header note explains: to enable a
non-claude harness you add it to this list **and** uncomment its `agents:` block below
(the two are orthogonal — the list decides which harness dirs get generated files; the
block decides the model/effort those files carry).

### 1b. `agents.claude` — the claude built-in defaults, active and explicit

The `claude` harness key holds the claude harness's values, populated with the
**nine** current claude built-in defaults (verified from `agents/docket-*.md`
frontmatter). We use the harness-named `claude:` key rather than the neutral
`default:` key because claude is docket's default harness — so all three blocks in
this file are harness-named (`claude` active, `codex`/`cursor` commented), which reads
consistently, and the claude harness resolves `agents.claude.<agent>` → built-in:

```yaml
agents:
  claude:
    status:                { model: claude-haiku-4-5-20251001, effort: medium }
    adr:                   { model: claude-sonnet-5,           effort: medium }
    brainstorm-consultant: { model: claude-opus-4-8,           effort: xhigh }
    auto-groom:            { model: claude-opus-4-8,           effort: xhigh }
    auto-groom-critic:     { model: claude-opus-4-8,           effort: xhigh }
    implement-next:        { model: claude-opus-4-8,           effort: xhigh }
    rebase-resolver:       { model: claude-opus-4-8,           effort: xhigh }
    integration-repair:    { model: claude-opus-4-8,           effort: xhigh }
    finalize-change:       { model: claude-sonnet-5,           effort: medium }
```

A comment states these **mirror docket's shipped built-in defaults** — they are shown
so the defaults are visible and tunable; deleting a line falls back to the same
built-in, so an unedited file behaves exactly as no file at all.

> Implementation note: these nine values are the source of a maintenance coupling — if
> a shipped `agents/docket-*.md` default changes, this file must be updated to match.
> **ADR-0039** records this decision: `agents/docket-*.md` frontmatter is the source of
> truth and `config.yml.example` is a documented mirror. The build should add a comment
> in the file stating this, and the verification includes a build-time equality check
> (below). No automated drift guard is in scope (see Out of scope).

### 1c. `codex` / `cursor` — commented-out example blocks

Both non-claude blocks ship **commented out**, carrying the user-provided example IDs
verbatim, as siblings of the `claude:` block under the same `agents:` key (same agent
order and indentation as 1b, so uncommenting drops them straight into place). The caveat
is one consolidated comment line above the blocks — not fragments trailing the data lines:

```yaml
  # To enable: verify the example IDs against your harness's current models, uncomment the block, and add the harness to `agent_harnesses` above.
  # codex:
  #   status:                { model: gpt-5.6-luna, effort: xhigh }
  #   adr:                   { model: gpt-5.6-terra, effort: xhigh }
  #   brainstorm-consultant: { model: gpt-5.6-sol, effort: medium }
  #   auto-groom:            { model: gpt-5.6-sol, effort: low }
  #   auto-groom-critic:     { model: gpt-5.6-sol, effort: medium }
  #   implement-next:        { model: gpt-5.6-sol, effort: medium }
  #   rebase-resolver:       { model: gpt-5.6-sol, effort: high }
  #   integration-repair:    { model: gpt-5.6-sol, effort: high }
  #   finalize-change:       { model: gpt-5.6-terra, effort: high }
  # cursor:
  #   status:                { model: grok-4.5-fast-medium, effort: auto }
  #   adr:                   { model: grok-4.5-xhigh, effort: auto }
  #   brainstorm-consultant: { model: grok-4.5-xhigh, effort: auto }
  #   auto-groom:            { model: grok-4.5-high, effort: auto }
  #   auto-groom-critic:     { model: grok-4.5-xhigh, effort: auto }
  #   implement-next:        { model: grok-4.5-xhigh, effort: auto }
  #   rebase-resolver:       { model: grok-4.5-xhigh, effort: auto }
  #   integration-repair:    { model: grok-4.5-xhigh, effort: auto }
  #   finalize-change:       { model: grok-4.5-fast-high, effort: auto }
```

Kept commented because a harness block present in `agents:` but **not** listed in
`agent_harnesses` is warned-and-ignored — so an out-of-the-box file with an active
`cursor:` block would emit a spurious warning. Commenting both blocks keeps a fresh
copy warning-free, and the enable instruction (uncomment + add to `agent_harnesses`)
keeps the two in step.

### 1d. Header comment

A short top-of-file comment: what the file is, that it accepts the full `.docket.yml`
schema, that only `agent_harnesses` + `agents` are shown here, and a pointer to README
→ Configuration for every other key.

## Deliverable 2 — `install.sh` scaffolds the global config (new primitive)

Add a fourth idempotent primitive, `scripts/ensure-global-config.sh`, invoked by
`install.sh`. Behavior:

- Destination: `${XDG_CONFIG_HOME:-$HOME/.config}/docket/config.yml`.
- If the destination **does not exist**: create the parent directory as needed, copy
  `config.yml.example` (from the docket repo root) to it, and log
  `docket: wrote <dest> from config.yml.example (edit to enable harnesses / tune models)`.
- If the destination **already exists**: do nothing to it, and log
  `docket: <dest> already exists — left untouched`.
- Never overwrites, never merges, never edits an existing file. Exit 0 in both cases.

It has a co-located contract `scripts/ensure-global-config.md` (Purpose / Usage /
Behavior / Exit codes / Invariants), matching the repo convention that every
`scripts/<name>.sh` has a `.md`.

**Ordering in `install.sh`:** run `ensure-global-config.sh` **first**, before
`sync-agents.sh`, so the first `sync-agents` pass reads the just-written global config.
On a first run that config is defaults-equivalent, so ordering is functionally a no-op,
but "config exists before the generator runs" is the clearer invariant. `install.sh`'s
header comment (the "runs N primitives in order" list) is updated to include the new
step.

The change data note and the existing three primitives are otherwise unchanged.

## Deliverable 3 — README Install restructure

Current: `## Install` → `### Prerequisites` → `### The one-line install` (install.sh +
three primitives + change-data/migration note).

New shape:

- Keep `### Prerequisites` unchanged.
- Rename `### The one-line install` to a step-1 framing, e.g.
  **`### 1. Install docket on your machine`**, keeping the `install.sh` one-liner and
  the primitives explanation. Update the primitives list to mention the new
  `ensure-global-config.sh` step (it drops a starter `~/.config/docket/config.yml` on
  first run).
- Add **`### 2. Set up your global config`**:
  - `install.sh` writes a starter `~/.config/docket/config.yml` from
    `config.yml.example` the first time (and leaves an existing one untouched).
  - The starter file shows docket's default per-skill models — this is where the
    otherwise-invisible defaults become visible and editable.
  - To enable a harness beyond claude: add it to `agent_harnesses` **and** uncomment
    its `agents:` block, then re-run `install.sh` so `sync-agents.sh` regenerates the
    wrappers.
  - Claude-only users can skip editing entirely — the defaults already apply.
- The existing "change data lives in each project / `migrate-to-docket.sh`" paragraph
  stays as the tail of the section.

Cross-links: the new step-2 prose links to the existing Configuration section rather
than restating the schema.

## Testing / verification

This is a docs + shell-scaffolding change; verification is behavioral:

- **`ensure-global-config.sh` — fresh:** with `XDG_CONFIG_HOME` pointed at an empty
  temp dir, run it; assert `config.yml` is created, byte-identical to
  `config.yml.example`, and the "wrote … from template" line is logged.
- **`ensure-global-config.sh` — existing:** pre-create a sentinel `config.yml` with
  distinct contents; run it; assert the file is unchanged and the "left untouched" line
  is logged.
- **Idempotency:** running `install.sh` twice leaves a user-edited config untouched on
  the second run.
- **YAML validity:** `config.yml.example` parses as valid YAML (e.g. `yq`), and with
  the `codex`/`cursor` blocks uncommented it still parses and resolves via
  `docket-config.sh`/`sync-agents.sh` without a harness-block warning once the harness
  is added to `agent_harnesses`.
- **Defaults match:** the nine `agents.claude` values equal the shipped
  `agents/docket-*.md` frontmatter (guards the documented-mirror coupling at build
  time).

## Out of scope / follow-ups

- Automated sync between `agents/docket-*.md` defaults and `config.yml.example` (a
  `--check`-style drift guard) — a possible follow-up, not built here. The sync
  *requirement* is recorded in **ADR-0039**; only the build-time equality check
  (Testing) enforces it for now.
- Interactive prompting during install (choosing harnesses at install time) — the
  starter is copy-then-edit, not a wizard.
