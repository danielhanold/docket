<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0460 — artifact.backlink refuses an absolute --change path with unknown-change](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0460-artifact-backlink-refuses-an-absolute-change-path-with-unkno.md)**
<!-- docket:backlink:end -->

# artifact.backlink — validate `--change` as a canonical repository-relative path

## Problem

`docket artifact backlink --change <absolute path>` returns `unknown-change`. The operation never
validated `--change` as a path. `resolveBacklinkChange` (`internal/app/artifact_backlink.go`)
compares the raw string with each corpus record's `Path()`, so an absolute path or a non-canonical
spelling looks like a missing record. The message does not say that the path's form is the problem.
In change 0458's implement-next run this cost an extra results commit.

## Decision: reject clearly, do not accept absolute paths

The stub proposed accepting an absolute `--change` path under the repository or its `.docket`
worktree. Tracing the code showed that the right fix goes the other way:

- Docket has one rule for paths passed on the command line: they are canonical
  repository-relative. `--artifact` on this same command already refuses absolute paths with the
  typed reason `absolute-path` and `..` escapes with `path-escape` (`containedArtifactPath`).
  `change.attach-plan` and `change.attach-results` enforce the same rule in `verifyAttachPath`,
  which also refuses non-canonical spellings (`./x`, `a//b`) as `path-escape`. The flag help for
  both backlink flags already says "canonical repository-relative path". `--change` is the only
  path flag that skips this validation.
- Accepting absolute paths would be a new policy. It would need two roots (the primary checkout and
  the `.docket` metadata worktree) and symlink canonicalisation (for example macOS `/tmp` →
  `/private/tmp`), which is the identity trap the `canonicalise-every-symlink-hop` learning records.
  No caller needs it: `context.implementation` hands out the change path repo-relative, and the
  plan-writer contract already says "repo-relative".
- The absolute path in 0458 came from skill prose. In `skills/docket-implement-next/SKILL.md`, the
  per-checkpoint mechanics (item 3) say `--change <change path>` without naming the form, so the
  coordinator made one up.

The `--artifact` check the stub asked for is already correct (absolute → `absolute-path`). It needs
only a regression test, not a behavior change.

## Design

### 1. Validate `--change` before the corpus read

In `ArtifactBacklink`, validate `req.ChangePath` before `resolveBacklinkChange`. The check is
purely lexical (the change record lives in the git corpus, not on the feature worktree's
filesystem) and matches `verifyAttachPath`:

| Input | Result | Reason |
|---|---|---|
| empty / whitespace | `invalid-input` | `path-escape` |
| `filepath.IsAbs` | `invalid-input` | `absolute-path` |
| `!filepath.IsLocal(filepath.FromSlash(p))` (`..` escape) | `invalid-input` | `path-escape` |
| `path.Clean(p) != p` (non-canonical spelling) | `invalid-input` | `path-escape` |
| well-formed but no matching record | `invalid-input` | `unknown-change` (unchanged) |

- Reuse the existing `ReasonBacklinkAbsolutePath` / `ReasonBacklinkPathEscape` constants. Do not
  add new reason codes; they are already in the operation's stable vocabulary.
- The messages name the flag and the expected form, for example:
  `--change path "/Users/…/docs/changes/active/0460-….md" is absolute; pass the canonical
  repository-relative change path (e.g. docs/changes/active/0460-<slug>.md)`. For `unknown-change`,
  keep the existing message. It is now reached only by a well-formed path.
- Keep the current order, so a malformed or missing artifact is still reported first: artifact
  containment and read, then the document parse, then `--change` validation, then the corpus
  read. `--change` validation may move earlier if the plan prefers, as long as every refusal still
  happens before any write.
- The implementation may factor a small shared lexical helper out of `verifyAttachPath` or copy the
  four checks inline. Either is fine, and it is a plan-time choice. It must not change the attach
  operations' observable reasons or messages.

### 2. Fix the prose that produced the absolute path

In `skills/docket-implement-next/SKILL.md`, per-checkpoint mechanics item 3, change
`--artifact <results path> --change <change path>` to
`--artifact <results repo-relative path> --change <change repo-relative path>`, which matches the
wording in `agents/docket-plan-writer.md`. If other maintained caller prose that passes
`artifact.backlink` flags leaves the path form unstated, fix it the same way. Find those callers
by a whole-repo grep for `artifact.backlink` / `artifact backlink`, not a hand-made list.

## Testing

Add tests alongside the existing `artifact_backlink` tests in `internal/app`:

- `--change` absolute → `invalid-input` / `absolute-path`; the artifact is byte-identical afterward.
- `--change` with `..` escape → `path-escape`; byte-identical.
- `--change` non-canonical (`./docs/changes/active/…`) → `path-escape`; byte-identical.
- `--change` empty → `path-escape`.
- `--change` repo-relative of an existing record → still renders (the existing happy path
  continues to pass).
- `--change` well-formed but absent → still `unknown-change`.
- Regression: `--artifact` absolute is still `absolute-path`.

Mutation-check the new guard: removing the `IsAbs` branch must turn the absolute-path test red
(it would fall back to `unknown-change`).

## Out of scope

- Accepting absolute paths on any flag.
- Path handling in other operations.
- The gate-drive `scope-closed` issue from the same 0458 run (tracked separately).
