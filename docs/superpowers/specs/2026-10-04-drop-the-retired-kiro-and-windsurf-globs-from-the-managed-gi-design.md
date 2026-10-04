<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0506 — Drop the retired-harness globs from the managed .gitignore block](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-10-04-0506-drop-the-retired-kiro-and-windsurf-globs-from-the-managed-gi.md)**
<!-- docket:backlink:end -->

# Drop the retired-harness globs from the managed .gitignore block — design

## Problem

`docket repository init` and `docket repository migrate` write a managed block into the
repository's `.gitignore` (`internal/reposetup/gitignore.go`, `canonicalBlockBytes`). The block
still carries agent-wrapper globs for three harness tokens docket no longer accepts:

```
.agents/agents/docket-*.md
.kiro/agents/docket-*.md
.windsurf/agents/docket-*.md
```

The accepted harness vocabulary is exactly `claude`, `codex`, `cursor`, `opencode`
(`agentHarnessTokens` in `internal/config/schema.go`; `harness.Order`). ADR-0020 defines the
block as derived from the harness roster so ignored paths and generated paths cannot drift; these
three lines are that drift. Change 0464 surfaced it as a follow-up.

## Decision

### 1. Remove exactly the three retired-harness lines

The new canonical block (markers unchanged, order otherwise unchanged):

```
# docket:start (managed by docket — do not hand-edit)
.docket/
.worktrees/
.claude/settings.local.json
.docket.local.yml
.claude/agents/docket-*.md
.codex/agents/docket-*.md
.cursor/agents/docket-*.md
.opencode/agents/docket-*.md
.codex/agents/docket-*.toml
.cursor/rules/docket-dispatch.mdc
# docket:end
```

### 2. Keep the four supported-harness wrapper globs and the codex `.toml` glob

Since change 0351 docket writes no per-repository agent wrappers at all, so these five lines match
nothing docket creates today. They are kept deliberately: they quarantine leftover wrapper files
from older docket versions. This repository carries 17 such files in `.cursor/agents/`, hidden by
`.cursor/agents/docket-*.md`. Removing the globs would surface leftovers as untracked files, and
`repository init`'s clean-primary preflight (`primaryCleanPresence`, which counts untracked
non-ignored paths) would then refuse. Retiring them is a separate decision, out of scope here.

### 3. Existing repositories: no compatibility machinery

`repository check` admits `healthy` only when the committed block is byte-exact
(`ValidGitignoreBlock`, condition `committed-ignore-valid`). After this change an adopting
repository whose committed `.gitignore` still holds the previous block classifies as non-healthy
with the existing `IgnoreDefectNonCanonical` finding — the old block contains every new entry plus
the three extras, so it is non-canonical, not missing entries — whose remedy already prints the
exact new block. `repository configure-tests` refuses until the block is rewritten. Operational
commands (`repository prepare`, `status`, the change ops, implement/groom/finalize) do not consult
the block (`operationalRefusal` refuses only `legacy`), so day-to-day work is unaffected.

The rewrite path already exists: re-running `docket repository init` on an initialized repository
passes `initGuard` and calls `ensureManagedGitignore` → `EnsureGitignoreBlock`, which strips the
stale block, re-emits the canonical one, preserves every byte outside the block, and leaves the
file unstaged for review. No version-tolerant acceptance of the previous block is added (YAGNI).
The results file's human actions must tell the operator to re-run `docket repository init` in each
adopting repository and commit the rewritten `.gitignore`.

### 4. This repository's own `.gitignore`

The PR updates docket's own committed `.gitignore` to the new canonical block in the same change,
so this repository stays `healthy` once it merges (produce it with the `EnsureGitignoreBlock`
rewrite or an equivalent byte-exact edit; outside-block content untouched).

### 5. Stale header comments

The header comments of `internal/reposetup/gitignore.go` and `gitignore_test.go` describe the
block as a port of `scripts/lib/docket-gitignore-block.sh` kept byte-identical by
`TestIntegrationRepoSetupGitignoreParity`. Both the bash lib and that test are gone. Rewrite the
comments so the Go file is the single owner of the block's bytes and `TestGitignoreBlockCanonical`
is its drift assert. Also update the `canonicalBlockBytes` order comment if its wording names the
removed entries' position. Comment anchors name symbols, never line numbers (ADR-0054).

## Tests

- **`TestGitignoreBlockCanonical`** — update its frozen expected bytes to the new block (it is the
  frozen-copy drift assert; learning `frozen-copy-needs-a-drift-assert`).
- **Roster guard (new).** Every block entry of the form `<root>/agents/docket-*.<ext>` must name a
  harness in the accepted vocabulary (`.claude`, `.codex`, `.cursor`, `.opencode`). Key it on the
  entry's syntactic shape over `GitignoreEntries()`, not a hand-kept list of forbidden spellings.
  Derive the accepted set from the harness vocabulary if that import is cycle-free from the test
  package; otherwise state the four tokens once with a comment naming the source symbol.
  Mutation check: re-adding `.kiro/agents/docket-*.md` to `canonicalBlockBytes` must redden it.
- **Upgrade rewrite.** `EnsureGitignoreBlock` on a file holding the previous 13-entry block between
  user lines returns `changed == true`, output equal to the user lines + one blank separator + the
  new block, user bytes preserved; a second call returns `changed == false` (idempotent).
- **Explain on the previous block.** `ExplainGitignoreBlock` on the previous block returns
  `IgnoreDefectNonCanonical` (not `MissingEntries`), so `repository check`'s remedy names the
  rewrite.

## Out of scope

- Retiring the four supported-harness wrapper globs or the codex `.toml` glob (see Decision 2).
- Deleting leftover per-repository wrapper files (e.g. this repository's `.cursor/agents/`).
- `link-skills.sh`'s `.kiro/skills` / `.windsurf/skills` targets and the frozen legacy-install
  testdata README — different mechanisms, not the managed ignore block.
- Accepted ADRs (0020, 0060) stay unedited; nothing here reverses a decision.
