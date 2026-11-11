<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0592 — Layer the tickets.* config keys at every normal level, not just the repo](../../changes/archive/2026-11-11-0592-layer-the-tickets-config-keys-at-every-normal-level-not-just.md)**
<!-- docket:backlink:end -->

# Layer the tickets.* config keys at every normal level

Change #592, groomed 2026-11-10, revised 2026-11-10.

## Problem

`internal/config/schema.go` declares nine `tickets.*` keys. One, `tickets.companion_skill`, already uses `scopeAny`. The other eight (`provider`, `site`, `project`, and the five `auto.*` switches) use `scopeRepoOnly`, so a value for any of them in the global config or `.docket.local.yml` is warned and ignored. Most other keys, such as `change_types` and `auto_groom`, use `scopeAny` and layer per field: repo-local > repo-committed > global > built-in.

The schema comment gives the reason as "They belong to the repository". The repo-only set otherwise holds the repository-identity keys (`integration_branch`, `changes_dir`, `adrs_dir`, `results_dir`), where a machine-level override would make clones disagree about where records live (ADR-0019). No tickets key has that property.

## Decision

All eight keys move to `scopeAny`, the same scope `companion_skill` already has. This was decided with the human at grooming, key by key:

| Key | Why it can layer |
|---|---|
| `provider`, `site` | Usually the same ticket system for every repo a person works in, so the global config is their natural home. Only the `docket-link-ticket` skill reads them, through `diagnostic.config`, and it asks the person when one is empty. Nothing derives a record path, branch name, or other cross-clone value from them. |
| `project` | Usually set per repo in `.docket.yml`, but a person whose repos all file into one project can set it once globally. Linking and creating a ticket is always attended, so a wrong project is caught before a ticket is created. |
| `auto.change_created`, `auto.spec_written`, `auto.pr_opened`, `auto.pr_merged`, `auto.results_completed` | Same shape as `auto_groom`, which already layers. Today `ticket.context` only reports them, and nothing acts on them. A team that wants agreement commits the value in `.docket.yml`. A global value never outranks a committed one, so only the person's own `.docket.local.yml` can override the team's choice. |

## Unsetting a global value at the repo level

Every `tickets.*` key defaults to empty (`""`) or `false`, and a value declared in a higher layer wins. So a repo cancels a global value by writing the default explicitly, in `.docket.yml` (for everyone) or `.docket.local.yml` (for this machine):

```yaml
tickets:
  project: ""          # cancels a global project for this repo
  auto:
    pr_opened: false   # cancels a global true
```

`diagnostic.config` then reports the empty or `false` value with the repo layer as its source. This needs no new mechanism. It is documented in the config reference and in the `.docket.example.yml` comment.

A bare `project:` (nothing after the colon) is a YAML null, not an empty string, and stays rejected. The rejection message for a string key that accepts an empty value names the fix: write `""` to clear the value. The hint goes in the shared string validator, so every such string key gets it, not just the tickets keys. Null is not newly accepted as "unset", which would make these keys read YAML differently from every other string key.

## Trace to confirm at build

1. Re-grep every reader of `eff.Tickets.*` (today: `internal/app/config.go` for `diagnostic.config`, `internal/app/ticket_context.go` for the `auto.*` switches, the `docket-link-ticket` skill) and confirm none needs cross-clone agreement. If a new reader appeared since grooming and does, stop and report rather than moving that key.
2. Check the private-repository path: a private repository keeps its config in `.git/dckt/config.yml` and never reads `.docket.yml`. State what the global layer and `.docket.local.yml` do there, and test it.
3. Confirm `diagnostic.config` reports provenance for a value from each machine layer, and for the explicit `""` / `false` cancel.

## Build

- Change the eight scopes in the schema table to `scopeAny`, and rewrite the comment above the block: the ticket settings layer like any other key.
- Add the `""` hint to the rejection message for a null on a string key that accepts empty.
- Update the schema, decode, resolve, and defaults tests that enumerate the keys so each of the eight has global and `.docket.local.yml` cases, including precedence, provenance, and a repo-level cancel of a global value (`""` for strings, `false` for switches). Add a test for the null rejection's hint, and a private-repository case.
- `docs/reference/config-keys.md`: change the eight rows from `repo-only` to `any layer`; replace the stale paragraph under the table ("no docket command reads them yet …") with how the settings are read today, which layers they can come from, and how to cancel a global value.
- `.docket.example.yml`: rewrite the stale comment above the `tickets` block the same way, including the cancel example.
- Update any skill prose that says these values come only from the repository config (`docket-link-ticket`, `docket-update-ticket`), and regenerate embedded assets after skill edits.

## Out of scope

Repository-identity keys, new ticket settings, new providers, any new place to read ticket settings from, and accepting YAML null as "unset".
