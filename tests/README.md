# docket's test suite

Standalone POSIX-shell files, discovered by the `tests/test_*.sh` glob at depth 1 — so a new file
self-registers with the runner. It does **not** self-register with the budget table: every file
also needs a row in `tests/runtime-budgets.tsv`, which is a registry. Each file is hermetic:
`set -uo pipefail`, its own tmpdir fixtures, no ordering dependencies, runnable on its own as
`bash tests/test_X.sh`.

## Categories — every suite file must declare one

Discovery is **category-declared and fail-closed** (change 0370). Every `tests/test_*.sh` file must
carry, in its first **10 lines**, exactly one declaration line:

```
# docket-suite: go               # a wrapper that runs `go test` (the bulk of the suite)
# docket-suite: posix-install    # the retained install.sh POSIX product suite
# docket-suite: posix-downloader # the retained release-downloader POSIX product suite
```

The three tokens above are the whole vocabulary. The parser matches the line **exactly**
(`^# docket-suite: (go|posix-install|posix-downloader)$`): a missing, malformed, unknown,
below-line-10, or trailing-text declaration is a **discovery error naming the file** — never
skipped, never assigned a generic or legacy category. There is no dormant compatibility branch: a
file the runner cannot categorize aborts the run (exit `2`).

The Go-native runner in `internal/suiterunner` is the whole and only test channel: the docket-owned
product behaviour is proved by Go tests these `go`-category wrappers drive, and only two POSIX
product surfaces (the installer and the release downloader) keep their own shell suites.

## Running it

The whole suite runs through the Go-native runner — what `finalize.test_command` resolves to and
what the merge gate runs (change 0318):

```
go run ./cmd/docket development test    # the whole-suite, branch-faithful source gate
```

Budgets are enforced by a **screen-then-confirm** regime:
a parallel run over `ceiling * 5/2` records an advisory screening finding (`BUDGET WATCH:` /
`PARALLEL-SENSITIVE:`), and only a solo measurement over `ceiling * 3/2` — from a single-job run, or
from a scheduled serial confirmation the runner triggers after repeated overruns — is an
authoritative breach (`SERIAL CONFIRMED OVER BUDGET:`). A default run reports its findings loudly and
still exits `0`. `DOCKET_RUNTESTS_STRICT=1` confirms every current candidate immediately and opts into
the failing exit; `DOCKET_RUNTESTS_JOBS` sets the parallelism.

Exit `0` green (including a green run that breached a budget), `1` a test failed, `3` green but at
least one target produced no result at all — the run certified nothing about it, `4` a breach under
strict budget, `2` usage error / runner-internal fail-closed (unusable bash, missing or duplicate
target, **undeclared or malformed suite category**), `130`/`143` interrupted by `SIGINT`/`SIGTERM`.

Exit `5` — a source-hygiene preflight violation in the old topology — is **retired** (change 0370).
Its still-meaningful invariant (see "Backticks in test source" below) is now a build-gate Go guard,
not a per-run preflight.

## Where new tests go

The suite is parallel, so its wall-clock floor is `max(slowest single file, total work / -j)`, and
contention inflates both terms — change 0227 measured ~120s of wall clock against a ~53s slowest
file, so the total, not the tail, was binding. Either way an added file costs the suite its full
serial cost plus a scheduling tail, so placement is a real decision:

1. **Extend the topical shard your assertion belongs to.** This is almost always right. Find the
   file already covering that subsystem and add to it — if it has room in `tests/runtime-budgets.tsv`.
2. **If that shard has no room, extend a sibling shard or start a new one.** `test_sync_agents*.sh`,
   `test_harness_defaults*.sh`, and `test_docket_config*.sh` are already split this way; adding
   `_<topic>` to the family is cheap and keeps every part under its ceiling.
3. **A brand-new file is for a brand-new subsystem** — a new script, a new surface. It needs a row
   in `tests/runtime-budgets.tsv`, or the registry↔files correspondence guard
   (`repoguard.TestRuntimeBudgetsCorrespondence`) fails.

Topic is the usual guide, but not always the deciding one. In the `test_harness_defaults*.sh`
family the cost is *per `hd_validate` sweep* and near-uniform per call (change 0227, Task 4), so an
added assertion's placement there should follow **whether it calls `hd_validate`**, not which topic
it nominally belongs to: a non-validating assertion is nearly free in either shard, and a validating
one costs the same wherever it lands.

**Never grow a file past its budget and raise the number.** `repoguard.TestRuntimeBudgetsCorrespondence`
checks row completeness — every test file has a row, no row is orphaned, and no row is malformed or
duplicated. The Go runner (`internal/suiterunner`) evaluates each row's ceiling and `serial` pin at
suite time and classifies over-budget files. If a file legitimately
cannot be split, that is a decision to argue in the diff, not a number to bump. Two files were
argued that way at change 0227; both have since been split:

- `test_sync_agents_codex.sh` — argued whole at change 0227 on the grounds that it had "no internal
  section banners, so there is no mechanical boundary". That was already inaccurate then and the
  file was split at change 0242's review: its `# ---` banners mark two independent surfaces, and
  the banner "AGENTS.md dispatch block: created, machine-neutral, committed-style" is the boundary
  — nothing above it reads the `AGENTS.md` fixture, nothing below it reads a `.toml` wrapper. The
  dispatch half is now `test_sync_agents_codex_dispatch.sh`. Recorded here because "there is no
  boundary" is the claim a later reader would otherwise trust instead of re-checking.
- `test_docket_config.sh` — argued whole at change 0227 because the change-0126 prelude-correspondence
  guard scanned its own `${BASH_SOURCE[0]}` and asserted whole-file floors (**≥60 `eval` sites**, a
  derived cross-check) that any split would have halved and falsified. Change 0251 resolved exactly
  that: it moved the guard's population from `${BASH_SOURCE[0]}` to the discovered
  `tests/test_docket_config*.sh` family corpus (its floors now sum across the family), so splitting
  the file is routine and a new shard self-registers with the guard just as it does with the runner.
  The file was then split at its change-0102 section banner into `test_docket_config.sh` (head) and
  `test_docket_config_guards.sh` (tail, which carries the guard); further shards need no guard change.

## Backticks in test source

A backtick in test source is **not** inert text: it is command substitution, and it runs when the
shell reaches that line — before `assert` is ever called, and regardless of what the assertion then
does with the string. That is not hypothetical. Change 0212 carried a verbatim-quoted guard anchor
containing a backticked `git checkout .`; the shell executed it while sourcing the line, silently
reverting a worker's uncommitted edits, and the test printed `ok`. (The executing vector is source
evaluation at the call site — not the helper printing its description. A backtick already held in a
*variable's value* is inert through `printf '%s' "$1"`.)

**The rule.** Verbatim clauses and guard anchors are *data*, so carry them where the shell cannot
evaluate them:

- Single-quoted literals, or heredocs with a **quoted delimiter** (`<<'EOF'`). An unquoted-delimiter
  heredoc body is live — its backticks execute.
- Inside an assert condition — argument 2, the string that reaches `eval` — escape the backtick.
  The house idiom is:

  ```sh
  assert 'the span is present' 'grep -qF "\`span\`" "$f"'
  ```

  The backslash survives the source read, so `eval` sees a literal backtick.
- **Never put a backtick inside double quotes** — bare or backslash-escaped. Both spellings execute:
  the bare one at source evaluation, the escaped one one level later, when `eval` re-parses what the
  escape left behind.
- Where a pattern needs a literal backtick beside a `$var` and so cannot be single-quoted wholesale,
  define a file-local inert backtick and concatenate it:

  ```sh
  BT='`'
  grep -qF "${BT}${name}${BT}" "$f"
  ```

  Six files in the suite use this.

**The enforcement.** The invariant is a **build-gate Go guard**, not a per-run preflight (change
0370, which retired the old exit-5 source-hygiene preflight along with the frozen Bash runner):
`internal/repoguard`'s `TestNoExecutableBacktickInSuiteSource` scans every declared-category shell
suite file (the surviving runner's admitted population) and fails on a backtick the shell would
execute at source-read — a bare or backslash-escaped backtick in bare-code or double-quoted
position, including the multi-line double-quoted 0212 shape. The guard's doc comment states what it
deliberately does not model (single-quoted spans, command-substitution frames, quoted-delimiter
heredocs) and why that is safe over the small, house-style surviving surface.

**The limitation.** Because it runs at the build gate over the maintained tree — not as a preflight
inside each `docket development test` run — a violation is caught by the whole-suite gate and by
`go test ./internal/repoguard/`, not at the moment you run one file directly. Run
`go test ./internal/repoguard/ -run TestNoExecutableBacktickInSuiteSource` before you trust a new or
edited shell suite file.

## Parallel-safety

The Go-native runner gives every job its own `HOME`, `TMPDIR`, `XDG_CONFIG_HOME`, and git config
(with a synthetic identity), and pins git non-interactive. A test must not read the ambient `$HOME`,
write global git config, use a fixed `/tmp/<name>` path, touch this repo's own worktrees, or reach
the network. A file that genuinely must share the real tree carries `serial` in the budget table —
and that pin is counted by the guard, so it has to be justified.
