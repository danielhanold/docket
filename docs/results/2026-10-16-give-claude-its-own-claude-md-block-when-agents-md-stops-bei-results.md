<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0555 — Keep Claude's dispatch block in AGENTS.md while CLAUDE.md links to it](../changes/archive/2026-10-16-0555-give-claude-its-own-claude-md-block-when-agents-md-stops-bei.md)**
<!-- docket:backlink:end -->

# Keep Claude's dispatch block in AGENTS.md while CLAUDE.md links to it: results

**Human action:** Recommended before merge. Decide whether the absolute `CLAUDE.md` symlink that install writes is acceptable for now (see Known issues). Nothing else needs a human.

## Outcome

When `CLAUDE.md` links to `AGENTS.md` and claude is opted in, Claude now co-owns the `AGENTS.md` dispatch block. The block survives dropping codex and opencode, is added for a claude-only repository with a hand-made link, and is restored in a repository that already lost it. When a claude-only repository has its own `AGENTS.md` and no `CLAUDE.md`, install links the two instead of writing a block-only `CLAUDE.md`. The block's text now follows the repository's full `agent_harnesses` list, so `docket install --harness opencode` no longer strips the Codex clause. Install no longer retires a surface in the same run that writes it. `classifyClaudeMD` also accepts the absolute link install itself writes (`<root>/AGENTS.md`), so the share rule still applies on every later install. The install docs, the glossary, and the agent-layer reference describe the new rule.

## Human actions and testing

- **Important: decide about the absolute link.** Why: a `CLAUDE.md` link docket creates holds an absolute, machine-specific path. If someone commits it, other clones and CI get a dangling `CLAUDE.md`. This change makes such links more common, because claude-only repositories with their own `AGENTS.md` now get one. When: before merging. If skipped: the behavior ships as described under Known issues.
  - Steps: in a scratch clone with `agent_harnesses: [claude]`, an `AGENTS.md` holding some text, and no `CLAUDE.md`, run `docket install`, then `readlink CLAUDE.md`.
  - Expected: it prints an absolute path ending in `/AGENTS.md`, and `AGENTS.md` now starts with the `docket:dispatch` block above your text.
  - Cleanup: delete the scratch clone.
- **Optional: check this repository's own drop.** In a scratch copy of this repository (`CLAUDE.md` is a relative link here), set `agent_harnesses: [claude]` in `.docket.local.yml` and run `docket install`. Expected: `AGENTS.md` keeps its `docket:dispatch` block, and `CLAUDE.md` still resolves to it.

## Verification performed

- The whole suite (`go run ./cmd/docket development test`) passed at the final head. The budget report had BUDGET WATCH and PARALLEL-SENSITIVE screening lines for unrelated shards (privatemode, recordops, repovisibility and others) and no SERIAL CONFIRMED OVER BUDGET line.
- The workers mutation-tested every new planner rule, the `computeRemovals` planned-path skip, the `AGENTS.md` classifier, and the absolute-link clause. Each mutation turned its target test red.
- A standard-tier review found 1 blocker and 2 important findings. The blocker (install's own absolute link read as foreign) and one important (tests rewrote that link to a relative one, which hid the blocker) were fixed in-branch. The other important finding is reported below.
- Backlog check: 21 proposed or deferred changes checked.

## Known issues and follow-ups

- **Install writes the `CLAUDE.md` link with an absolute path** (confirmed, came before this change). It happens whenever install creates the link: Claude sharing with codex or opencode, and now claude-only repositories with their own `AGENTS.md`. The link works on the machine that ran install, and later installs recognize it. A committed copy points at nothing on any other machine. Workaround: replace it by hand with `ln -sf AGENTS.md CLAUDE.md`, which is the relative form install already recognizes. Suggested next action: write the repository link as the relative `AGENTS.md`, and teach `internal/install` to validate and compare a relative `LinkTarget` for repository surfaces. No existing change fits (checked 21). A human could capture this as a new change, linking #555 under `related:`.
- **Accepted gap (by design):** after a drop to `[claude]`, a first install scoped to a harness that is no longer opted in (for example `--harness opencode`) still retires the shared block. The next run that includes claude restores it.
