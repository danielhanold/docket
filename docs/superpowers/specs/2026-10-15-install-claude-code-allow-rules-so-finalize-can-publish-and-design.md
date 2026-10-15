<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0552 — Install Claude Code allow rules so finalize can publish and merge under auto mode](../../changes/archive/2026-10-15-0552-install-claude-code-allow-rules-so-finalize-can-publish-and.md)**
<!-- docket:backlink:end -->

# Claude Code allow rules for finalize publish and merge — design

## Problem

When `docket-implement-next` has just run the full suite green and `docket-finalize-change` runs right after, finalize correctly reuses that green evidence and skips the re-run. Claude Code's auto-mode classifier reads the resulting `docket finalize publish` (a force-with-lease publish) and `docket finalize merge` as a "CI bypass" and soft-denies them, so a clean finalize stalls on a human prompt.

Adding these two rules to `permissions.allow` in the user-level `~/.claude/settings.json` cleared the soft-deny (observed on another machine, October 2026, auto mode):

```json
"allow": [
  "Bash(docket finalize publish:*)",
  "Bash(docket finalize merge:*)"
]
```

Today each user has to find and hand-add them on every machine. `docket install` should manage them.

## Decision: user-level, owned by the existing Claude settings target

**Global, not per repository.** The rules match only `docket finalize publish|merge`, which refuse outside a prepared docket repository, so a user-level grant authorizes nothing in unrelated repositories. This is the difference from change 0027, which rejected a global rule because that rule was a raw `git push … HEAD:<integration_branch>`. A per-repository grant (`.claude/settings.local.json`) would need a write in every clone, would be missing on fresh clones, and would break the private-repository rule that nothing lives in the repository root. User-level `~/.claude/settings.json` is also where `docket install` already owns entries.

**Same target, second owned list.** `docket install` already owns the two SessionStart hook commands in `~/.claude/settings.json` through one `hook-entries` target (`harness.ClaudeHookTarget`, `internal/install/hook_entries.go`). The installed state holds one record per path (`priorRecord` resolves by path) and a transaction carries at most one removal per path, so a second target on the same file is not allowed. The existing Claude target therefore grows a second owned list, its **allow rules**, next to its hook commands. Do not generalize the hook-entries machinery into a generic JSON-array target.

## Design

### Rules

- The rules are two constants beside the Claude hook commands in `internal/harness/trigger.go`: `Bash(docket finalize publish:*)` and `Bash(docket finalize merge:*)`, spelled exactly as proven. `harness.ClaudeHookTarget` sets them on the target. They use the `docket` spelling only. Skills resolve argv from the capability catalog, which spells `docket`, so the `dckt` alias is not covered.
- **Drift guard.** A test asserts that each rule equals `Bash(` + the space-joined catalog argv of `finalize.publish` / `finalize.merge` + `:*)`. A rename of either command then reddens the test instead of leaving a rule that silently matches nothing. Mutation-test the guard.

### Target and record

- `install.Target` gains `AllowRules []string`, and `TargetRecord` gains `AllowRules []string \`json:"allow_rules,omitempty"\``.
- Allow rules are valid only on a `hook-entries` target in the `claude` dialect. They must be non-empty, distinct strings. A Cursor target carrying allow rules fails validation, both on the target and on the record.
- **Record identity.** `hookEntriesDigest` covers the allow rules **only when there are any**. A record written by today's install (hooks only) keeps its existing digest and stays a valid, consistent docket record. No state-format bump.

### Editing `permissions.allow`

This reuses the existing byte-span scanner and splice helpers. Nothing is decoded and re-encoded, and every byte docket does not own is preserved.

- **Readable shape.** The file is editable for rules when its top level is an object, `permissions` is absent or an object, and `permissions.allow` is absent or an array. Elements that are not strings are ignored for matching. Any other shape is not editable. It takes the existing skip path (below), never a conflict and never "rules absent".
- **Present** means an array element equal, as a JSON string, to the rule. A rule the user typed by hand counts as present, so docket never adds a duplicate.
- **Insert** adds each missing rule, in order, as the last element of `permissions.allow`. It creates `allow` as the last member of `permissions`, and `permissions` as the last top-level member, when they are absent. It follows the file's indent unit like the hook splice does. A result that does not read back is a defect and never reaches the user's file.
- **Remove** cuts each exact rule, last rule first. An `allow` array that docket's removal empties is removed with its member. A `permissions` object that then empties is removed with its member. Insert-then-remove must return the original bytes.

### Install: converge, never just "skip if present"

Each install converges the file to the planned state, the same way it does for hook commands:

- **Retire.** Rules in the prior record that the plan no longer carries, and that the file still holds, are cut. This uses the `staleHookCommands` posture: the prior record must be a consistent claude-dialect hook-entries record, or nothing is retired.
- **Add.** Every planned rule that is not present is added. That includes a rule the user deleted by hand.
- **Disposition.** `update` when anything is retired, or when any planned hook command or rule is missing. Otherwise `noop`, and the file is byte-identical.
- One rewrite per transaction for the file: retire hooks, retire rules, insert hooks, insert rules.

### Uninstall

- Uninstall cuts the recorded rules along with the recorded hooks.
- The existing ownership check (`editable && allExact`) is unchanged, because a string rule is always exact.
- A hand-typed copy of the same string cannot be told apart from docket's, so it is removed too. The hooks behave the same way, and this is accepted.

### Files docket cannot edit

A `settings.json` that is a symlink or not editable keeps today's skip with a warning, and never blocks the install. The skip remedy now names the two rules to add by hand, along with the hooks.

### Upgrade

The first install after this change finds a hooks-only record and a file without the rules. Its disposition is `update`: it adds the rules and writes a record carrying them. The hook entries are untouched.

## Testing

- **Unit tests** in `internal/install`:
  - insert into files with no `permissions`, with `permissions` but no `allow`, with an empty `allow`, and with a populated `allow`;
  - a hand-typed rule is not duplicated;
  - unreadable shapes skip (`permissions` not an object, `allow` not an array);
  - insert/remove round trip is byte-identical;
  - removal that empties `allow` and `permissions`;
  - retire of a rule dropped from the plan, plus no retire when the prior record is inconsistent;
  - a byte-identical `noop` on an already-converged file;
  - re-add after a hand-deleted rule;
  - a hooks-only prior record digest still verifies, and the upgrade disposition is `update`;
  - a Cursor target with allow rules fails validation.
- **Install service / app level:** a full install then uninstall over a realistic `settings.json` (with user `allow`/`deny` entries, `defaultMode`, and other keys) leaves every user byte intact.
- **The catalog drift guard** above.
- The whole suite runs at the build gate.

## Docs

- `docs/install/claude-code.md` and `docs/install/install.md` list the two allow rules beside the SessionStart hooks, with one line on why: finalize publish and merge reuse a green suite run, and auto mode otherwise reads that as a CI bypass. They also note that uninstall removes the rules, and that a hand-deleted rule comes back on the next install.
- Update `skills/docket-convention/references/agent-layer.md` if it enumerates what install writes to `settings.json`.
- Record the decision as an ADR relating to ADR-0145: the user-level Claude settings target now carries a narrow permission grant, not only content-free triggers, and the ADR says why it is user-level.

## Verification

The results file carries an Optional human item, which is not a blocking gate: on this machine, in auto mode, run a finalize right after a green implement-next run and confirm that publish and merge proceed with no prompt.

## Out of scope

- Per-repository grants.
- Cursor, Codex and OpenCode.
- The `dckt` spelling.
- Any other finalize subcommand or docket command.
- A config knob to disable the rules.
- A blocking probe of classifier behavior.
- A generic JSON-array install target.
