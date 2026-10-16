<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0555 — Keep Claude's dispatch block in AGENTS.md while CLAUDE.md links to it](../../changes/active/0555-give-claude-its-own-claude-md-block-when-agents-md-stops-bei.md)**
<!-- docket:backlink:end -->

# Keep Claude's dispatch block in AGENTS.md while CLAUDE.md links to it — design

Change #555, groomed 2026-10-16.

## Problem

In a shared repository, install can make `CLAUDE.md` a relative symlink to `AGENTS.md`, so Claude
reads the same file Codex and opencode read. The planner (`reposeed.Plan`,
`internal/reposeed/plan.go`) counts only codex and opencode as owners of the `AGENTS.md` dispatch
block. Claude owns only the link. Two things go wrong:

1. **Dropping codex and opencode strips Claude's rules.** With `agent_harnesses: [claude, opencode]`
   and the link in place (this repository's own setup), changing the list to `[claude]` makes
   install retire the `AGENTS.md` block, because none of its recorded owners is still opted in. The
   link is kept, so Claude keeps reading `AGENTS.md`, now without the dispatch and run-tracker
   rules. Every later install skips the link (below), so the rules never come back. Since change
   #537, an `AGENTS.md` that held only the block is deleted, and the link points at nothing.
2. **A claude-only repository with the link gets no block at all.** When claude is the only
   selected harness and `CLAUDE.md` is a relative link to `AGENTS.md`, the planner skips `CLAUDE.md`
   (`ClaudeMDLinkToAgents` falls into the `isLink` case) and plans nothing else. Linking
   `CLAUDE.md` to `AGENTS.md` by hand is a common setup, and today it leaves such a repository with no
   dispatch block. The skip came in as a quick fix (commit bc56f96fa) after a `--harness claude`
   install in this repository tried to write a block through the link and failed.
3. **A claude-only repository with its own `AGENTS.md` gets a block-only `CLAUDE.md`.** When
   `CLAUDE.md` is absent, the planner links it to `AGENTS.md` only when codex or opencode is
   selected. Otherwise it writes a regular `CLAUDE.md` holding only the block, so Claude never sees
   the repository's `AGENTS.md`.

A related bug sits in the same function. `Plan` picks the `AGENTS.md` block's text from the
harnesses the run is limited to (`--harness`), not from every harness the repository opted into. In a
repository with codex opted in, `docket install --harness opencode` rewrites the block without the
Codex clause until the next unscoped install. Private repositories got this fix on 2026-10-06
(commit 6e9f236ab, `PrivatePlanInput.OptIns`). Shared repositories did not.

## Decision

**When `CLAUDE.md` is a relative link to `AGENTS.md` and claude is selected, Claude co-owns the
`AGENTS.md` dispatch block.** The block lives in one file, and Claude reads it through the link.
**When `CLAUDE.md` is absent and the repository has its own `AGENTS.md`, install links them** rather
than writing a block-only `CLAUDE.md`.

This puts the Go planner back on ADR-0078's rules. When `CLAUDE.md` is a link, write the block once,
into the file the link points to. When `CLAUDE.md` is absent and `AGENTS.md` exists, create the
link. ADR-0078 chose the link so Claude sees everything `AGENTS.md` carries (project instructions
and promoted learnings). It rejected a block-only `CLAUDE.md` because Claude would lose that
content.

### Rejected: replace the link with Claude's own `CLAUDE.md`

The stub proposed that, once `AGENTS.md` is no longer shared, install replace the link with a regular
`CLAUDE.md` holding only Claude's block. It was dropped because:

- Claude would stop seeing the rest of `AGENTS.md`. In this repository that is 12.7 KB of project
  instructions and promoted learnings, which is the loss ADR-0078 rejected.
- It needs the install transaction to swap a symlink for a file at one path. Today the apply step
  refuses to write a block where it finds a link, and one transaction cannot remove and write the
  same path.
- It still leaves a claude-only repository with a hand-made link without a block.

## Design

### Planner rule (`reposeed.Plan`)

Two new inputs, both computed by the caller (`Plan` never stats a path):

- **The full opt-in set.** Same meaning as `PrivatePlanInput.OptIns`: the repository's whole
  `agent_harnesses` list, while `Harnesses` stays the set this run plans for. Nil means `Harnesses`
  is already the full set.
- **The `AGENTS.md` state, read with `os.Lstat`, with three values:** missing, a regular file, or
  something else (a symlink, including a circular one, a directory, or an unreadable path).

Field names, and whether the `AGENTS.md` state joins the `CLAUDE.md` state or stands alone, are the
builder's call.

**Claude shares `AGENTS.md`** when claude is selected and either:

- `CLAUDE.md` is a relative link to `AGENTS.md` (`ClaudeMDLinkToAgents`) and `AGENTS.md` is missing
  or a regular file, whatever else is opted in; or
- `CLAUDE.md` is absent, and either codex or opencode is in the full opt-in set or `AGENTS.md` is a
  regular file.

When `CLAUDE.md` links to `AGENTS.md` but `AGENTS.md` is something else, claude-only planning stays
as it is today: `CLAUDE.md` is skipped and nothing is planned for Claude. Planning a block there
would only turn a run that succeeds today into an ownership conflict, or into a filesystem error
for a circular link. When codex or opencode is selected, their `AGENTS.md` block is planned as it is
today, whatever `AGENTS.md` is.

When Claude shares, the plan holds:

- the `CLAUDE.md` link target, owned by claude (as today);
- the `AGENTS.md` dispatch block, with claude among its owners.

**The `AGENTS.md` block is planned** when codex or opencode is selected, or when Claude shares. Its
owners are the selected harnesses among codex and opencode, plus claude when Claude shares.

**The block's text follows the full opt-in set:** the Codex interior when codex is opted in anywhere,
the plain interior otherwise. This fixes the `--harness opencode` bug. The new rule needs it too:
`--harness claude` in a repository with codex opted in now plans the `AGENTS.md` block, and without
this fix it would strip the Codex clause.

These cases do not change:

- A regular `CLAUDE.md` keeps its content and gets its own block, even when `AGENTS.md` is present.
- `ClaudeMDOther` is still planned as a block, so inspection reports the conflict.
- A `CLAUDE.md` that links anywhere else is still skipped (`ClaudeMDForeignLink`).
- A claude-only repository with no `CLAUDE.md` and no `AGENTS.md` still gets a regular `CLAUDE.md`
  with the block.

### What each case does after the change

| Setup | Today | After |
|---|---|---|
| `[claude, opencode]` → `[claude]`, link in place | `AGENTS.md` block retired; link kept; Claude has no rules | block kept, owned by claude; link kept |
| `[claude, codex]` → `[claude]`, link in place | block retired | block kept, rewritten with the plain interior |
| `[claude]`, hand-made link, `AGENTS.md` has user text | nothing planned | block added to `AGENTS.md`; link recorded |
| `[claude]`, link in place, `AGENTS.md` has no block (already broken) | nothing planned | block added |
| `[claude]`, link in place, `AGENTS.md` missing (dangling link) | nothing planned | `AGENTS.md` created with the block; link resolves |
| `[claude]`, link in place, `AGENTS.md` is a symlink or a directory | nothing planned | unchanged |
| `[claude]`, no `CLAUDE.md`, user's own `AGENTS.md` | regular `CLAUDE.md` with block | link created; block added to `AGENTS.md` |
| `[claude]`, regular `CLAUDE.md`, any `AGENTS.md` | own block in `CLAUDE.md` | unchanged |
| `[claude, codex]`, `--harness claude`, nothing on disk yet | regular `CLAUDE.md` with block | link created; `AGENTS.md` created with the Codex interior |
| `[claude, codex, opencode]`, `--harness opencode` | block rewritten without the Codex clause | Codex clause kept |
| `[claude, codex]` → `[codex]`, link in place | link removed; block kept for codex | unchanged |
| `[claude]` sharing → `[]` | — | link removed; block retired, and an `AGENTS.md` left empty is deleted (#537) |

"Added" means docket's usual placement for a new block: at the top of the file, or right after
frontmatter (`renderManagedBlock`). Docket never appends it at the end.

A repository that already has a block-only `CLAUDE.md` from an earlier install keeps it. It is a
regular file now, and a regular `CLAUDE.md` is never replaced.

### Never retire a block this run also writes (`computeRemovals`)

`computeRemovals` (`internal/app/repophase.go`) retires a prior surface when none of its recorded
owners is still opted in. Records written before this change list `AGENTS.md` as owned by codex
and opencode only. This repository's live record lists `[opencode]`. After the drop to `[claude]`,
the new rule plans the `AGENTS.md` block while the old record still retires it:

- **When the planned block matches what is on disk** (opencode dropped, plain interior both
  times), the write inspects as a no-op. `planSteps` drops no-ops before its duplicate check, so
  the removal runs. The block is retired while the new record says claude owns it. The next install
  adds it back, and the state flips back and forth.
- **When the planned block differs** (codex dropped, so the Codex interior becomes the plain one),
  the write is an update. `rejectDuplicateDestinations` refuses the whole run ("the dispatch block in
  AGENTS.md is written by more than one step").

**Rule:** a prior surface whose path this run plans is never retired. The planned write replaces its
record. Ownership is still proven at inspection, before anything is written: the on-disk block must
match the prior record or the frozen legacy bytes, or the run refuses as it does today.

Today `planned` is set only in private mode, where it also changes how `composeRecordBytes` carries
surfaces forward. Keep the private-mode meaning unchanged, and add the shared-mode check without
reusing that parameter's nil-means-shared convention. How to do that is the builder's call.

`composeRecordBytes` needs no change. A prior `AGENTS.md` surface with no surviving owner is skipped,
and the planned surface carries the new owners. A scoped run still unions in an out-of-scope
co-owner.

**Scoped runs: the guard covers only what this run plans.** One gap is accepted. Take a record
that lists `AGENTS.md` as owned by `[opencode]` only. After the drop to `[claude]`, if the first run
is `docket install --harness opencode`, that run plans nothing, so the block is retired. It retires
the surface the user asked about, and the next run that includes claude adds it back (recreating
`AGENTS.md` if #537 deleted it). Closing the gap would need a second, unscoped plan to work out
what is still wanted, plus record carry-forward for a block the scoped run did not write. That
costs more than a one-run gap is worth.

### Callers that build a `PlanInput`

Each one supplies the two new inputs:

- `ResolveRepoPhase` (`internal/app/repophase.go`) passes `optIns` and the `AGENTS.md` state read
  beside `classifyClaudeMD(root)`.
- The shared-mode plan in `repository_set_visibility_commit.go` passes the same facts for
  `st.primary`. It already plans with the full list, so its opt-ins equal its harnesses.
- `reposetup.PlanInit`'s `SeedInput`. The builder confirms whether anything consumes it and fills in
  the fields to match.

`classifyClaudeMD` itself does not change. A dangling relative link to `AGENTS.md` still reads as
`ClaudeMDLinkToAgents`, which is what lets install repair a repository whose `AGENTS.md` is gone.

### Install transaction

There is no change to `internal/install` behavior. Every step the new plan produces is one the
transaction already handles:

- adding a block to a file that has none (no ownership proof needed);
- creating `AGENTS.md`;
- rewriting a block whose interior matches the prior record;
- re-planning an existing link (a no-op).

**Accepted: a new path into the existing ownership refusal (decided at grooming).** A claude-only
repository with the link now has an `AGENTS.md` block planned. If that `AGENTS.md` already holds a
`docket:dispatch` block docket cannot prove is its own (no matching record, no legacy match, or
broken markers), install refuses the whole run. The machine half refuses too, with the existing
conflict and remedy naming `AGENTS.md`. Today the same repository's install skips `CLAUDE.md` and
succeeds. This is the refusal every repository surface already gets. A regular `CLAUDE.md`, or an
`AGENTS.md` codex shares, with an unprovable block refuses today, so no special case is added to
skip it.

### Comments and docs to update

- `reposeed.Plan`: "Claude never co-owns this block — when it shares, it does so through a CLAUDE.md
  link" and the share comment above `share :=`.
- The `ClaudeMDLinkToAgents` doc comment ("a share re-plans the same link").
- `computeRemovals`'s doc comment, for the planned-path rule.
- `TestPlanClaudeSymlinkWithoutShareIsSkipped`: its "link to AGENTS.md, claude alone" case and
  comment move to the new behavior. The two foreign-link cases stay.
- `TestPlanClaudeCodexAbsentSymlinks` and its comment: `AGENTS.md` owners become `[claude, codex]`.
  Any other test asserting `AGENTS.md` owners without claude while Claude shares changes the same
  way.
- `inspectManagedBlock`'s "Adding a block only appends" comment (`internal/install/inspect.go`). The
  block is inserted at the top or after frontmatter. Comment only.
- `docs/install/claude-code.md` ("The dispatch block in `CLAUDE.md`, on opt-in only").
- `docs/reference/glossary.md`, the `agent_harnesses` entry ("with a `CLAUDE.md` → `AGENTS.md`
  symlink when Codex or opencode is also opted in").
- `skills/docket-convention/references/agent-layer.md`, the "a non-empty list" bullet. Regenerate the
  embedded copy (`go generate ./internal/assets/`).

The docs describe the rule as it now stands: Claude shares `AGENTS.md` through a `CLAUDE.md` link
when `CLAUDE.md` already links there, or when there is no `CLAUDE.md` and `AGENTS.md` already exists
or is needed by codex or opencode. Otherwise `CLAUDE.md` gets its own block.

## Verification

New and updated tests. Each new one must redden under the matching mutation listed after them.

**Planner (`internal/reposeed`):**

1. **Link, claude alone:** the `AGENTS.md` block owned by `[claude]` with the plain interior, plus the
   `CLAUDE.md` link owned by claude. This replaces the skip case.
2. **Link, claude and codex:** `AGENTS.md` owners `[claude, codex]`, Codex interior.
3. **No `CLAUDE.md`, `AGENTS.md` a regular file, claude alone:** the link plus the `AGENTS.md` block
   owned by `[claude]`.
4. **No `CLAUDE.md`, no `AGENTS.md`, claude alone:** a `CLAUDE.md` block and no `AGENTS.md` target
   (unchanged).
5. **Regular `CLAUDE.md`, `AGENTS.md` a regular file, claude alone:** a `CLAUDE.md` block and no
   `AGENTS.md` target (unchanged).
6. **Scoped interior:** `Harnesses [opencode]`, opt-ins `[codex, opencode]` gives the Codex interior.
   `Harnesses [claude]`, opt-ins `[claude, codex]`, link in place gives the Codex interior with
   owners `[claude]`.
7. **Scoped share decision:** `Harnesses [claude]`, opt-ins `[claude, codex]`, no `CLAUDE.md`, no
   `AGENTS.md` gives the link plus the `AGENTS.md` block.
8. **Foreign links:** still skipped (existing cases).
9. **Link to a non-regular `AGENTS.md`, claude alone:** nothing planned for Claude when `AGENTS.md`
   is a symlink or a directory.

**Repository phase on a real filesystem (`internal/app`):**

10. **The drop, from a pre-change record:** seed the repository with the link, an `AGENTS.md`
    carrying the block, and a record that lists `AGENTS.md` owned by `[opencode]` only and the link
    owned by `[claude]`. That is the shape installs wrote before this change, and this repository's
    live record. The new code would record claude as a co-owner, which hides the bug, so seed the
    record directly or produce it through a scoped run. Then install with `[claude]`. The `AGENTS.md`
    block is still there, the link is intact, and the record lists `AGENTS.md` owned by `[claude]`.
    - Run it twice: once with an `AGENTS.md` holding only the block, once with user text around it.
    - Add a codex variant: a record with `AGENTS.md` owned by `[codex]`, the Codex interior on disk,
      then install with `[claude]`. The run succeeds and the block is rewritten with the plain
      interior.
11. **Repair:** a record holding the link (claude), with an `AGENTS.md` that has user text but no
    block. Install adds the block and keeps every byte of the user text. A sibling case with
    `AGENTS.md` deleted: install creates it with the block, and the link resolves.
12. **Hand-made link, claude alone, no prior record:** the block lands in `AGENTS.md` and every byte
    of the user's text stays. Don't assume the block sits at the end.
13. **Scoped run keeps the Codex clause:** with `[claude, codex, opencode]` installed,
    `--harness opencode` leaves the `AGENTS.md` interior byte-identical.
14. **Drop claude:** `[claude, codex]` with the link → `[codex]` removes the link and keeps the block
    for codex. An existing test may already cover this. Keep it green.
15. **Drop everything:** `[claude]` sharing → `[]` removes the link and retires the block. An
    `AGENTS.md` that held only the block is deleted (#537).

Mutations to run:

- Restore the skip for a link when codex and opencode are not selected: tests 1, 10, 11, and 12
  redden.
- Drop the planned-path rule in `computeRemovals`: test 10 reddens. In the opencode variant the
  block is missing after the run; in the codex variant the run is refused.
- Pick the interior from the scoped set: tests 6 and 13 redden.
- Decide the no-`CLAUDE.md` share from the scoped set: test 7 reddens.
- Ignore the `AGENTS.md` state in the no-`CLAUDE.md` share: test 3 reddens.
- Share for a link whatever `AGENTS.md` is: test 9 reddens.
- Share even when `CLAUDE.md` is a regular file: test 5 reddens.

The whole suite (`build.test_command`) is green.

## ADRs expected

None. This restores ADR-0078's rule in the Go planner. The implementer's ADR step may still record one
if the build shows otherwise.

## Out of scope

- **A `CLAUDE.md` that links anywhere other than `AGENTS.md`** (another file, an absolute link, a
  chain, a link outside the repository). It stays unsupported and keeps being skipped.
- **Hand-made links being recorded as docket's.** Re-planning an existing link records it as
  docket's, so dropping claude later deletes it. A link carries no content, and install already
  behaves this way today when Claude shares with codex.
- **Turning an existing block-only `CLAUDE.md` into a link.** A regular `CLAUDE.md` belongs to the
  user and keeps its own block.
- **A scoped run for a harness no longer opted in** retiring a block Claude now shares. The next
  run that includes claude adds it back (see the scoped-runs note above).
- **Windows checkouts without symlink support.** ADR-0078 accepted this.
- **Private repositories**, which keep no surfaces in the working tree.
