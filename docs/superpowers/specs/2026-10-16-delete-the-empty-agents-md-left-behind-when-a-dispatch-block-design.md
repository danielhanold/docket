<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0537 — Delete the empty AGENTS.md left behind when a dispatch block is retired](../../changes/archive/2026-10-16-0537-delete-the-empty-agents-md-left-behind-when-a-dispatch-block.md)**
<!-- docket:backlink:end -->

# Delete the empty AGENTS.md left behind when a dispatch block is retired — design

Change #537, groomed 2026-10-16.

## Problem

When docket removes one of its managed blocks from a file, it cuts the block's lines and writes the
rest of the file back. If docket created that file to hold the block, nothing is left, and a
zero-byte file stays behind. Change #535's build found the visible case: in a repository that went
private, install retires the dispatch block an earlier install wrote into the working-tree
AGENTS.md and leaves an empty, untracked AGENTS.md. Nothing leaks, but `git add -A` would commit
it, and it sits in `git status` until someone deletes it by hand.

Every managed-block removal goes through one step, `(*Txn).removeManagedBlock` in
`internal/install/txn.go`, so the same leftover appears in each of these cases whenever docket
created the file:

- **Private-mode retirement.** Install's repository phase in a private repository retires every
  working-tree surface an earlier install wrote: the AGENTS.md and CLAUDE.md blocks (the #535 case).
- **Shared-mode harness drop.** When every owner of a repository surface leaves
  `agent_harnesses` (for example, codex and opencode dropped), install retires the AGENTS.md block.
- **Uninstall.** `docket uninstall` removes the Codex pointer block from `~/.codex/AGENTS.md`.
- **The private instructions file.** In a private repository whose `agent_harnesses` becomes empty,
  install retires the dispatch block from `.git/dckt/AGENTS.md`. Promoted lessons outside the
  block keep the file. Without lessons it is left empty, and this change deletes it, which needs
  the `set-visibility` fix below.
- **Legacy global retirement.** An old user-level dispatch block retired from a user-level file.

## Decision

After removing docket's block, **if what is left is empty or whitespace only, delete the file**
instead of writing it back.

This is the rule `repository set-visibility private --remove-shared-files` already applies (change
#533): `stripDispatchBlock` in `internal/app/repository_set_visibility_private.go` deletes AGENTS.md
or CLAUDE.md when only whitespace is left after the block is stripped
(`len(bytes.TrimSpace(out)) == 0`), and the managed `.gitignore` block is handled the same way. This
change applies the same test inside the installer's removal step, so every caller behaves alike.

### Rejected: recording whether docket created the file

The stub proposed recording, in the ownership record, whether docket created the file, and deleting
only then. It was dropped because:

- It needs a new field in two records: the machine install state (`install.TargetRecord`) and the
  repository ownership record (`reposeed.SurfaceRecord`). `install.LoadState` rejects unknown
  fields, so an older `docket` binary would refuse a state file a newer one wrote. That happens
  in practice when the binary is installed from a feature worktree and later from main.
- It must be carried forward across every reinstall, which adds logic to both record writers.
- It never helps a file created before the change ships.
- It protects only one case: a user created an empty or whitespace-only file, and docket later put
  its block into it. Deleting that file loses no content. The user's bytes were whitespace.

## Design

### The rule, in `removeManagedBlock`

Today, after `doc.Apply(patch)` cuts the block, the step publishes the result through
`writeThroughStaging`. The change: when `len(bytes.TrimSpace(out)) == 0`, remove the file with the
same primitive whole-file removals use (`removeIfPresent(t.fs, step.Path)`) instead of writing it.
Every other path through the step is unchanged:

- the `preAbsent` "already retired" no-op;
- the refusal to rewrite through a symlink;
- the parse and apply errors.

Whether to share a tiny "only whitespace left" helper with `stripDispatchBlock` is the builder's
call. The two run in different packages, on different inputs: a file on disk inside a transaction,
and a HEAD blob for a commit.

### What does not change

- **Ownership proof comes first, exactly as today.** The step only runs for a block the planner
  already proved is docket's: its interior matches the prior record or the frozen legacy bytes.
  An edited block still refuses the run with the existing conflict.
- **User text outside the block keeps the file.** Only the block's lines go, and every other byte
  stays (`TestTxnManagedBlockRemovalStripsOnlyTheBlock` and the retire/uninstall prose tests keep
  passing unchanged).
- **Directories are never removed.** A directory left empty, such as `~/.codex/`, stays.
- **Install output is unchanged.** The existing `remove <path>` action covers both "block cut" and
  "file deleted". No new action or wording.
- **Hook-entries removal is unchanged.** A hooks file docket created keeps whatever JSON is left
  after its entries are cut.

### Rollback and recovery

No journal change is needed. A managed-block removal's step already captures the file's bytes and
mode as a `preFile` pre-image at `BeginTxn`, and `restoreStep` writes those back whether the
destination now holds rewritten bytes or nothing at all. A run that deletes the file and then fails
later, or a crashed run that `Recover` finishes, puts the file back byte-for-byte with its mode.

### Cutting one block and writing another into the same file

`rejectDuplicateDestinations` allows one transaction to remove one managed block from a file and
write a different block into it, and its comment claims the steps "compose in any order". Deleting
the file keeps that true for the end state:

- **Write first, then remove:** the file holds both blocks, the cut leaves the written block, so the
  file is not empty and is rewritten.
- **Remove first, then write:** the cut leaves nothing, so the file is deleted. The write step does
  not re-verify its pre-image (only removals are re-verified in `applySteps`), so
  `renderManagedBlock` reads the path as absent and creates the file with the new block.
  `writeThroughStaging` takes the mode from the step's `preFile` pre-image, so the user's mode
  survives.

The real case is a `~/.codex/AGENTS.md` that holds only an old global dispatch block. One install
retires it and writes the Codex pointer block (`TestInstallRetiresLeftoverBlockAndWritesPointerInOneFile`
covers the version with user prose, where today's plan order is the retirement first). Update the
`rejectDuplicateDestinations` comment to say that a removal may delete the file and a later write
recreates it.

### Comments to update

Each of these describes the old "the file always stays" behaviour:

- `removeManagedBlock`'s doc comment: "rewriting it with exactly that block's marker-to-marker
  lines gone".
- The `applyStep` comment above the `removeManagedBlock` call: "it is not a whole-file deletion".
- `rejectDuplicateDestinations`, as above.
- The comment above the AGENTS.md assertion in `TestIntegrationPrivateInstructionsRetiresBuggyWorktreeSurfaces`
  (`internal/app/private_instructions_integration_test.go`): "its retirement cuts only the block
  and keeps the file".

`retireManagedBlock`'s comment in `internal/install/retire.go` ("the surrounding bytes are never
Docket's to delete") stays true. The deleted file has no surrounding bytes left.

### `set-visibility private` must not need the private file when no harness is declared

`repository set-visibility private` decides that its "instructions" phase is done (in
`planToPrivate`, `internal/app/repository_set_visibility.go`) when the private state folder exists,
no working-tree ownership record is left, and either `agent_harnesses` is not declared in the
repository configuration (`facts.SurfacesAuthorized` is false) or `.git/dckt/AGENTS.md` exists
(`privateInstructions`, which only checks that the file exists). An explicit `agent_harnesses: []`
counts as declared, so the check needs the file.

- **Today:** a private repository that empties its harness list keeps a zero-byte private file,
  and `set-visibility private` reports "nothing to do".
- **With this change and no fix:** the file is deleted, so the phase never reads as done. Every
  `set-visibility private` run on an already-private repository reports it pending and asks for
  `--yes`. Running it installs nothing, so the next run is pending again.
- **Already broken today:** a private repository whose harness list was empty from the start never
  has the file.

**Fix:** the phase also counts as done when the declared harness list is empty, because there is
nothing for the phase to install. The file is still required when at least one harness is
declared. Where the empty-list fact comes from (the resolved `agent_harnesses` value already
behind `SurfacesAuthorized`) is the builder's call.

### Known leftover this does not fix

**The dangling CLAUDE.md link (#555).** In a shared repository where docket made CLAUDE.md a link
to AGENTS.md, dropping codex and opencode while keeping claude retires the AGENTS.md block but
keeps the link. If AGENTS.md held only the block, this change deletes it, and the link now points
at nothing. Claude was already without the dispatch rules in that state (it read an AGENTS.md with
no block), so nothing gets worse. #555 owns the real fix: giving Claude its own CLAUDE.md block.

## Verification

New and updated tests. Each new one must redden under the matching mutation, listed after them.

1. **Only the block:** in `internal/install`, a managed-block removal from a file holding only
   that block deletes the file (`os.Lstat` reports not-exist).
2. **Block plus whitespace:** a file holding the block plus only blank lines, spaces, or tabs is
   deleted too.
3. **Block plus user text:** the file stays, with every byte outside the block identical. The
   existing tests cover this; they must stay green.
4. **Rollback after a deletion:** a run that deletes the file and then fails at a later step
   restores it byte-for-byte with its original mode.
5. **Cut and write on one file:** a variant of `TestInstallRetiresLeftoverBlockAndWritesPointerInOneFile`
   whose `~/.codex/AGENTS.md` holds only the leftover dispatch block. The install ends with the file
   holding exactly the pointer block, and the rollback variant (following
   `…RollsBackBothSteps`) restores the original bytes.
6. **Private retirement:** tighten `TestIntegrationPrivateInstructionsRetiresBuggyWorktreeSurfaces`
   to require AGENTS.md to be absent. Today it accepts absent or zero bytes.
7. **Shared harness drop:** a repository-phase test that retires the AGENTS.md block after its
   owners leave `agent_harnesses`. An AGENTS.md that held only the block is deleted, and one with
   user text keeps that text. The builder picks the drop scenario that fits the existing fixtures.
8. **Uninstall:** uninstall removes a managed block from a file holding only that block and deletes
   the file. A sibling of `TestUninstallSatisfiedAbsenceAndManagedBlockRemoval`, whose prose case
   stays as is.
9. **Edited block still refuses:** the existing edited-interior conflict tests stay green.
10. **Set-visibility with no harnesses:** a private repository with `agent_harnesses: []` and no
    `.git/dckt/AGENTS.md`. `repository set-visibility private` returns the no-op result ("already
    private; nothing to do"), not a pending instructions phase. A sibling case with at least one
    harness declared and the private file missing still reports the phase pending.

Mutations to run:

- Drop the whitespace check, so the step always writes back: tests 1, 2, 6, 7, and 8 redden.
- Always delete: test 3 reddens.
- Revert the set-visibility done check: test 10's no-harness case reddens. Make it count as done
  regardless of the file: its with-harness case reddens.

Tests 4 and 5 are regression guards, not mutation targets. Test 5's end state is the same whether
the emptied file is deleted or rewritten empty, and that sameness is the point.

The whole suite (`build.test_command`) is green.

## ADRs expected

None. This extends the rule #533 already applies, and the reasoning belongs in
`removeManagedBlock`'s comment. The implementer's ADR step may still record one if the build shows
otherwise.

## Out of scope

- **Empty files earlier runs already left behind.** Their surfaces are no longer in any ownership
  record, so docket cannot prove they are its own. Delete them by hand.
- **The dangling CLAUDE.md link** (#555).
- **Hook files emptied of docket's entries.** For example, a `~/.cursor/hooks.json` docket created
  is left holding `{}` after uninstall.
- **Removing directories** left empty by a deletion.
- **`set-visibility`'s own removal commit**, which already deletes whitespace-only files (#533).
- **The instructions phase with harnesses declared.** It still only checks that the private file
  exists, not that it carries the dispatch block.
