<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0592 — Layer the tickets.* config keys at every normal level, not just the repo](../../changes/active/0592-layer-the-tickets-config-keys-at-every-normal-level-not-just.md)**
<!-- docket:backlink:end -->

# Layer the tickets.* config keys at every normal level

Change #592, groomed 2026-11-10.

## Problem

`internal/config/schema.go` declares all eight `tickets.*` keys (`provider`, `site`, `project`, and the five `auto.*` switches) with `scopeRepoOnly`. A value for any of them in the global config or `.docket.local.yml` is warned and ignored. Most other keys, such as `change_types` and `auto_groom`, use `scopeAny` and layer per field: repo-local > repo-committed > global > built-in.

The schema comment gives the reason as "All belong to the repository". The repo-only set otherwise holds the repository-identity keys (`integration_branch`, `changes_dir`, `adrs_dir`, `results_dir`), where a machine-level override would make clones disagree about where records live (ADR-0019).

## The argument to test

For each key, ask whether a differing value across layers can make two clones of the same repo behave incompatibly. If not, repo-only is not protecting anything.

- `provider` and `site` name the ticket system. They are normally identical across every repo a person works in, so the global layer is the natural home. A wrong or missing value only changes which ticket system the interactive link and update skills suggest. It never changes where a record lives.
- `project` is usually per-repo, so it belongs in the committed file, but a machine-local override is harmless for the same reason.
- The `auto.*` switches decide whether docket acts on a linked ticket after an event. Two clones with different values would update tickets differently. That is a real difference, but it is also true of other behavior switches that already layer normally. The spec's job is to confirm this against how those switches are read.

## Trace to do before changing anything

1. Find every reader of `eff.Tickets.*` and confirm none uses the value to derive a record path, a branch name, or anything another clone must reproduce.
2. Check the private-repository path: a private repository keeps its config in `.git/dckt/config.yml` and never reads `.docket.yml`. State what the global layer does there and test it.
3. Check how `diagnostic.config` reports provenance for a key set in a machine layer.
4. Find the docs and skill prose that say these values come from the repository config (`docket-link-ticket`, `docket-update-ticket`, config reference) and list them.

## Decision rule

- If the trace finds no reader that needs cross-clone agreement, switch all eight keys to `scopeAny`. This follows the rule that config keys behave uniformly unless a difference is justified.
- If it finds one that does (most likely an `auto.*` switch), keep that key repo-only, switch the rest, and record the reason in the schema comment.
- If it finds none of the keys can move safely, leave the schema as it is and record why in the change's results.

## Build

- Change the scope in the schema table, and update the comment above the block to match.
- Update the schema, decode, resolve, and defaults tests that enumerate the keys so each key has a case for global and `.docket.local.yml` layers, including precedence and provenance.
- Add a case for the private-repository config path.
- Update the prose contracts and skill text that describe where the values come from, and regenerate embedded assets after skill edits.
- Fix the stale comment above the `tickets` block in `.docket.example.yml`: it says no docket command reads these settings yet, and that linking a change does not consult them. Rewrite it to describe how the settings are read today and which config layers they can come from.

## Out of scope

Repository-identity keys, new ticket settings, new providers, and any new place to read ticket settings from.
