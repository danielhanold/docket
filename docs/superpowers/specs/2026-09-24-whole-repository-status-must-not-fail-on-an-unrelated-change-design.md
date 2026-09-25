<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0454 — Whole-repository status must not fail on an unrelated change's invalid branch name](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-25-0454-whole-repository-status-must-not-fail-on-an-unrelated-change.md)**
<!-- docket:backlink:end -->

# Whole-repository status survives an unrelated invalid branch name — design

**Change:** 0454 · **Type:** fix · **Groomed:** 2026-09-24 (interactive, Daniel)

## Problem

Change 0449 (ADR-0127) bounded every named operation's live branch-fact probe to its own
base/stack (`stackBranchesFor`), but left the whole-corpus probe (`stackBranches`) in place for
the two reads that rank the whole backlog. When any change that is a stack ancestor records a
`branch:` value that is not a valid ref name, that probe fails and takes the entire read down with
`external-failed`. The read a human or agent uses to find and repair the broken record is the one
that breaks. 0449's own integration test (`named_isolation_integration_test.go`,
`assertUnrelatedBytesIntact`) works around this by not calling `Status`.

## Findings from tracing the existing implementation

- `Status` step 4 (`internal/app/status.go`) calls `reader.BranchFacts(ctx, pin,
  stackBranches(snap))`. `stackBranches` collects the recorded `branch:` of every stack ancestor
  of every change in the corpus.
- `gitStatusReader.BranchFacts` (`internal/app/status_git.go`) calls `gitcli.FetchBranch` per
  branch. `FetchBranch` runs `validateRefName` (`internal/gitcli/types.go`) locally before any
  network use and returns `KindInvalidRequest` for a name like `feat/a..parent`. The reader maps
  only `KindRefUnavailable` to "absent"; every other failure aborts the whole read.
- Three whole-repository reads share this exact probe:
  - `docket status`
  - `maintenance.preflight`, which runs `Status` for its post-sweep read, so implement-next's
    selection-path startup breaks too
  - `context.implementation` with no explicit id, the automatic-selection branch that calls
    `stackBranches(snap)`
- `repository check` never probes branch facts and is unaffected.
- Snapshot validation (`repository.BuildSnapshot`) does not flag a malformed `branch:` value. The
  defect is only noticed when something fetches it.
- `domain.ResolveEffectiveBase` already handles a parent branch missing from `BranchFacts`: rule 6
  returns `BaseBranchAbsent`. The child is then not build-ready (`stack-base-unresolved`, board
  reads "waiting on #A — stack base not built"). The parent's own readiness never consults its
  own branch.
- gitcli's local grammar is narrower than git's `check-ref-format`. Names containing `:`, `?`,
  `[`, `~`, `^`, other ASCII control characters, or ending in `.` pass `validateRefName`, reach
  `git fetch`, and fail as `KindCommandFailed`. That also aborts the read today.

## Design

### 1. One ref-name predicate, completed to git's grammar

Complete `gitcli.validateRefName` to git's `check-ref-format` rules by adding rejection of:

- ASCII control characters (< 0x20 and 0x7F), not only whitespace and NUL
- `~`, `^`, `:`, `?`, `[`
- a name ending in `.`

Its existing rules (leading `-`, NUL, whitespace, `@{`, `\`, `*`, the `refs/` prefix, empty,
`.`/`..`, and leading-dot components, `..` sequences, and a `.lock` component suffix) stay
unchanged.

Export the predicate so the app layer can ask the same question the probe will ask (for example
`gitcli.ValidateRefName(RefName) error`, or a `ValidBranchName(short string) bool` wrapper that
prefixes `refs/heads/`). Every gitcli operation keeps calling it. The only behavioural change for
them is that a name git would reject anyway now fails early as `KindInvalidRequest` instead of
`KindCommandFailed`.

Do not merge the other narrower copies (`recordedBranch`, `domain.malformedBranchRef`,
`transaction.validRefShape`) into this one. That is outside this change; see Out of scope.

### 2. The whole-corpus probe skips names that cannot exist

`stackBranches` excludes any recorded ancestor branch that fails the exported predicate. A string
that is not a valid ref name cannot exist on the remote, so leaving it out of the probe is an
accurate statement of absence, not a guess: `BranchFacts.HasBranch` returns false for it.

- Both whole-corpus callers (`Status` and automatic `context.implementation`) get the fix from this
  one edit. Preflight gets it through `Status`.
- `stackBranchesFor` (named operations) is left unchanged. A named operation whose own stack
  parent carries a malformed branch keeps failing closed as it does today; 0449 already keeps an
  unrelated record from reaching it.
- `gitStatusReader.BranchFacts` is unchanged. A well-formed name that still fails to probe for a
  real reason (network, auth) remains a whole-read `external-failed`. An observation failure must
  never be reported as proof that a branch is absent.

### 3. Selection needs no new logic

The domain already handles the result. A change stacked on a parent with a malformed branch
(directly, or through `stacked-merged` parents per rule 5) resolves to `BaseBranchAbsent`. It is not build-ready, drops out of
`ready` and automatic selection, and shows the existing `stack-base-unresolved` readiness. The
parent itself keeps whatever readiness it had. No new readiness token, status, or field.

### 4. Status reports a per-change finding

Extend status's app-level health checks, in the same loop and style as `artifactChecks`: for every
**displayed active change** whose `branch:` is present, non-empty, and fails the predicate, emit
one finding:

| field | value |
|---|---|
| `code` | `branch-malformed` (new `FindingCode` constant `FCBranchMalformed`, registered in `AllFindingCodes`; it reuses the token finalize's skip and `recordedBranch` already use) |
| `severity` | `error` |
| `entity_kind` | `change` |
| `entity_identity` | the change identity (`changeIdentity`) |
| `field` | `branch` |
| `message` | names the change and quotes the recorded value as not a valid git branch name |
| `remedy` | the concrete next step for this record's state; see *Remedy text* below |

- The finding sorts with the artifact findings: `assembleFindings` already orders that group by
  identity, then field.
- Human output (`status_human.go`) and the preflight envelope (which forwards status findings)
  render it with no format change.
- The finding covers every displayed active change, not only stack parents. It costs nothing
  extra, and a malformed branch anywhere already breaks finalize and named post-claim operations,
  so status is where a human should see it.

#### Remedy text

`StatusFinding.Remedy` must be valid for the exact reported state, so the text depends on whether
the record has a PR. Status stays offline: it fills in only values it already holds (the change
id, the record's blob version from the pinned corpus, and the PR number parsed from `pr:` with the
existing `parsePRRef`). It never reads GitHub.

- **The record's `pr:` parses to a PR number:** name the existing typed repair, which adopts the
  PR's own head branch as `branch:`:
  `docket change repair-identity --id <N> --expect-version <version> --adopt-pr-head --expect-pr <pr> --expect-head <the PR's head branch>`.
  Id, version, and PR number are filled in. The text tells the human to take the head branch from
  the PR itself. `repair-identity` re-checks every value and refuses on drift, so a stale remedy
  can never write.
- **No `pr:`, or one that does not parse:** no typed operation edits `branch:`. The remedy says to
  correct `branch:` on the change record on the `docket` branch (the real feature branch, or clear
  it if no branch was ever created), then run `docket repository migrate` to re-render the board,
  which a hand edit leaves stale.

`repair-identity`'s workspace gate (`repairProveWorkspaceClear`) skips the workspace check only
when `recordedBranch` calls the current branch malformed. `recordedBranch`'s own shape check is
narrower than the completed gitcli predicate: `feat/a:b` passes it, then fails later as a
workspace conflict. To make the PR-case remedy work for every name status flags, `recordedBranch`
delegates its shape check to the exported gitcli predicate. The effect is fail-closed: a name git
would reject is refused as `branch-malformed` earlier, rather than failing inside git.

The finding is deliberately **not** a snapshot-validation finding. Adding it to
`repository.BuildSnapshot`'s report would put it in the ADR-0127 transaction engine's
subject/baseline comparison, where strict whole-corpus operations would start refusing on it. That
widens the change for no gain.

### 5. No new ADR

This applies ADR-0127's existing rule (a read reports per-record defects, it does not abort on an
unrelated record) to the one read that rule left out, and it closes that ADR's recorded follow-up.
It sets no new policy.

## Testing

1. **gitcli unit:** a table test covering every rule `validateRefName` adds, plus one existing
   rule as a control. Mutation: deleting any one added rule makes its row fail.
2. **`stackBranches` unit:** a corpus with a stack parent on `feat/a..parent` and one on
   `feat/a:b`. Neither name appears in the output; valid ancestors still do.
3. **`Status` (fake reader):** over that corpus the result is `applied`, not `external-failed`.
   - The child is not in `ready` and reads `stack-base-unresolved`.
   - Each parent carries exactly one `branch-malformed` error finding with `field: branch`.
   - Unrelated changes render and rank normally.
   - A poisoned-reader variant (0449's `poisonBranchReader` idiom) proves the malformed name is
     never asked for. Mutation: removing the filter makes this red.
4. **Automatic `context.implementation` (no id):** with the malformed parent present, selection
   returns the top healthy build-ready change, not `external-failed`.
5. **`maintenance.preflight`:** returns its normal verdict with the finding forwarded.
6. **0449 integration test:** change `assertUnrelatedBytesIntact` (and its comment) to go through
   the real `Status` over the `feat/a..parent` fixture and assert the finding, instead of routing
   around it.
7. **Remedy variants:** a malformed record with a parseable `pr:` gets the `repair-identity`
   remedy with its id, version, and PR number filled in. One with no `pr:` gets the hand-edit and
   `repository migrate` remedy. Mutation: swapping the branch condition makes this red.
8. **The PR-case remedy works:** `change repair-identity --adopt-pr-head` (fake GitHub, adopted
   head present on the remote) applies on records whose branch is `feat/a..parent` and on records
   whose branch is `feat/a:b`. The `feat/a:b` row fails without the `recordedBranch` delegation.
9. **Regression:** a well-formed branch whose probe fails for a real external reason still yields
   `external-failed` for the whole read.

Run the whole suite at the build gate (`build.test_command`).

## Out of scope

- Named-operation validation scoping (done by 0449).
- Repairing, rewriting, or auto-clearing the malformed `branch:` value.
- Making a malformed `branch:` a snapshot-validation error, or refusing writes on it.
- Merging `domain.malformedBranchRef` or `transaction.validRefShape` into the gitcli predicate.
  Only `recordedBranch` delegates, because the remedy depends on it.
- A new typed operation for editing `branch:` on a record without a PR.
- `repository check`, which does not probe branch facts.
