# ensure-claude-settings.sh — idempotent Claude Code allow-rule injector

## Purpose

Grants `docket`'s `terminal-publish` push permission by writing one `allow` rule into the
calling repo's per-user `.claude/settings.local.json` (gitignored). The rule pre-authorizes
exactly the command shape `terminal-publish` issues:

```
git -C <transient-worktree> push origin HEAD:<integration_branch>
```

Force-push and pushes to other branches remain guarded. Because `settings.local.json` is
gitignored and per-user, `migrate-to-docket.sh` cannot seed it for future cloners; this
helper lets any fresh cloner of an already-migrated repo grant themselves the rule with a
single run. Introduced in change 0027.

## Usage

```bash
# From inside the repo you want to grant:
bash scripts/ensure-claude-settings.sh
```

The script operates on the repo containing `$PWD` (resolved via `git rev-parse
--show-toplevel`). No positional arguments; no flags.

**Integration branch resolution:** the script calls `docket-config.sh --export --repo-dir
<root>` and reads `INTEGRATION_BRANCH` from the output. Override with
`DOCKET_INTEGRATION_BRANCH=<branch>` to skip the config call (required when origin is
unreachable and useful in tests).

**Mock seam:** `GIT="${GIT:-git}"` — override `GIT` in tests.

## Behavior

1. **Resolve repo root.** `git rev-parse --show-toplevel` from `$PWD`. Exits 1 with a
   diagnostic if not inside a git repo.

2. **Resolve integration branch.** Uses `DOCKET_INTEGRATION_BRANCH` if set; otherwise calls
   `docket-config.sh` and reads `INTEGRATION_BRANCH`. Exits 1 if the branch cannot be
   resolved.

3. **Construct the rule.** `Bash(git -C * push origin HEAD:<integration_branch>)`.

4. **Ensure `settings.local.json` exists.** Creates `<repo>/.claude/settings.local.json`
   (and the `.claude/` directory) if absent, seeding it with `{}`. Refuses to clobber a
   pre-existing file that is not valid JSON (exits 1 with a diagnostic instructing the user to
   fix or remove it).

5. **Idempotently merge the rule.** Uses `jq` to add the rule to
   `.permissions.allow[]` only if it is not already present. All existing keys and rules are
   preserved in their original order. A second run is always safe and always a no-op on the
   rule.

6. **Emit a one-line summary** to stdout:
   - `rule already present in <rel-path> — no change.` if the rule was already there.
   - `created <rel-path> and granted: <rule>` if the settings file was newly created.
   - `added grant to <rel-path>: <rule>` if the rule was appended to an existing file.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Rule is present (either pre-existing or just added). |
| 1 | Not inside a git repo; integration branch unresolvable; `settings.local.json` exists but contains invalid JSON; `jq` update failed. |

## Invariants

- **No git writes.** The script never touches the git index or creates commits. It only writes
  `<repo>/.claude/settings.local.json`.
- **Idempotent.** Re-running any number of times leaves exactly one copy of the rule in the
  allow list. The rule count never exceeds 1.
- **Preserves all existing content.** All keys outside `.permissions.allow` and all existing
  entries in `.permissions.allow` are carried through unchanged. Only the new rule entry is
  appended (when absent).
- **Refuses corrupt input.** If `settings.local.json` pre-exists but fails `jq empty`, the
  script exits 1 without modifying the file.
- **Per-repo, per-user.** The target file is `<repo>/.claude/settings.local.json`
  (gitignored). It is never written to a global or user-level settings location.
- **Rule mirrors the real command.** The allow pattern is derived from `$INTEGRATION_BRANCH`
  at runtime, so it tracks `.docket.yml` changes without any manual skill or convention edit.
