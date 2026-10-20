# Export corpus provenance

Frozen inputs for `corpus_test.go`. Every file under `changes/`, `specs/`, `plans/`, and
`results/` is a verbatim copy from this repository's `docket` metadata branch at commit
`a6c555422f1fdb17577ef472309f0b3859f519ec`, copied on 2026-10-20. Never edit them: a leftover
found here is fixed in the transform. The files under `constructed/` are hand-built.

The folder sets the export kind: `changes/` exports as an issue, `specs/` as a spec, `plans/`
and `constructed/` as a plan, `results/` as results.

## titles.tsv

`<id>\t<raw title>` for every number in any corpus file that is a real change id, read from
that record's frontmatter `title:` on the same commit. No id is claimed by two records there.
`998` is deliberately absent: `constructed/code.md` uses it as the unknown id.

## Files and what they exercise

| File | Source path on the metadata branch | Exercises |
|---|---|---|
| `changes/2026-06-02-0001-results-artifact.md` | `docs/changes/archive/` | Older archived record: frontmatter with unquoted scalars, an artifacts block with no `## Artifacts` heading, `blob/main` docket links inside the block, a `## Reconcile log` dropped by the section allowlist, a word-form reference (`change #0004`) in kept prose. That reference names another repository's change, yet it converts to this repository's change 4 title: the export cannot tell repositories apart. |
| `changes/2026-09-14-0423-certify-native-multi-agent-v2-orchestration-through-docket-i.md` | `docs/changes/archive/` | Shared-repo absolute metadata-branch link (`blob/docket/...`), removed with its `## Reconcile log`; unpadded bare numbers (`425 and 424`) that are not reference forms. |
| `changes/2026-10-19-0567-cleanup-deletes-a-stacked-child-s-branches-once-its-code-rea.md` | `docs/changes/archive/` | Record with bookkeeping sections: `## Artifacts` block (heading left empty), `## Reconcile log`, `## Build evidence` with its `build-evidence` block; bare `#562` kept. |
| `changes/0257-clear-the-residual-review-findings-from-0193-and-0201.md` | `docs/changes/active/` | Active, deferred record: a title holding padded ids (title cleaning to "another issue"), dense padded and `#0ddd` references, `PR #152` kept, `## Why deferred` dropped, `## Open questions` kept. |
| `specs/2026-09-02-harden-integration-race-test-isolation-under-parallel-load-design.md` | `docs/superpowers/specs/` | Stamp `Change N. Groomed <date> (interactive). Priority …` (rest of the line kept); backlink block; references-only parenthesis `(0371, 0397)`. |
| `specs/2026-07-28-board-checks-exit-swallowed-design.md` | `docs/superpowers/specs/` | Stamp line that is only `Change N.`; empty headings left by removals. |
| `specs/2026-06-04-migration-tool-pwd-target-design.md` | `docs/superpowers/specs/` | `**Change:** N` stamp line; `change 0002` word form; `ADR-0002` kept. |
| `specs/2026-07-10-docket-skill-slimming-design.md` | `docs/superpowers/specs/` | Autolinks `<https://…>`; a `·`-separated `**Key:** value` run holding `**Change:**`; word-form references. |
| `specs/2026-09-23-named-implement-next-skips-unrelated-maintenance-preflight-design.md` | `docs/superpowers/specs/` | H1 `# Change N — <its own title>` (the remainder equals the record title), own-title dash removed; dense padded references in prose and in an arrow list. |
| `specs/2026-10-19-measure-whether-the-1h-cache-ttl-cut-implement-next-s-cost-design.md` | `docs/superpowers/specs/` | Recent `Change #N, groomed <date>.` stamp. |
| `specs/2026-07-26-cursor-agent-wrapper-contract-design.md` | `docs/superpowers/specs/` | External inline link `[text](https://cursor.com/…)` reduced to its text. |
| `plans/2026-10-16-list-the-artifact-backlink-finding-codes-in-the-docket-statu.md` | `docs/superpowers/plans/` | Code fences holding reference-shaped text (`change 0539` in Go comments, commit messages); code spans holding `(change 0539)` and `## Artifacts`; backlink block. |
| `results/2026-10-17-let-change-groom-clear-stacked-on-results.md` | `docs/results/` | Results dense with references: a padded list in a heading, a references-only parenthesis of seven ids removed whole, `0007-shaped` (not a form), bare `#565` kept. |
| `results/2026-10-08-release-v1-0-0-rc-1-claude-code-and-cursor-tested-opencode-s-results.md` | `docs/results/` | A bare URL ending a sentence, kept as written; backlink block. |
| `constructed/code.md` | hand-built | Reference-, link-, and marker-shaped text inside a backtick fence, a tilde fence, single- and double-backtick code spans; a bare URL, `#559` (a known id), `PR #433`, `ADR-0012`, and the unknown padded id `0998` in prose. |
| `constructed/frontmatter.md` | hand-built | A non-issue artifact with leading frontmatter. No spec, plan, or results file on the metadata branch has one, and an issue's frontmatter is also dropped by the section allowlist, so without this file disabling step 1 would leave the corpus green. |

## What the corpus does not contain

Checked by scanning every corpus file outside fences and code spans:

| Category | Covered instead by |
|---|---|
| Images (`![alt](p)`) | `TestStripLinks` |
| Reference-style links and link-reference definitions (`[t][r]`, `[r]: u`) | `TestStripLinks`, `TestIsLinkDefinition` |
| HTML `<a href>` links | `TestStripLinks` |
| Inline code spans that cross a line break (a known limitation: such a span is treated as plain text) | none; no corpus line outside a fence holds an unmatched backtick run |
| Damaged markers and an unclosed frontmatter opener | `TestTransformDamagedSourceKeepsContent` |
| CRLF line endings | `TestTransformNormalizesCRLF` |
| An id two records claim | the app-layer export tests |
