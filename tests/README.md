# docket's test suite

122 standalone Bash files, discovered by the `tests/test_*.sh` glob — so a new file self-registers
with the runner. It does **not** self-register with the budget table: every file also needs a row in
`tests/runtime-budgets.tsv`, which is a registry, or `tests/test_runtime_budgets.sh` fails. Each
file is hermetic: `set -uo pipefail`, its own tmpdir fixtures, no
ordering dependencies, runnable on its own as `bash tests/test_X.sh`.

## Running it

The whole suite runs through the Go-native runner — what `finalize.test_command` resolves to and
what the merge gate runs (change 0318):

```
go run ./cmd/docket development test    # the whole-suite, branch-faithful source gate
```

`scripts/run-tests.sh` remains present as the frozen parity oracle and a focused-file tool; it is
no longer the whole-suite gate:

```
scripts/run-tests.sh --verbose tests/test_docket_config.sh   # one file, full output
scripts/run-tests.sh -j 1                                    # serial reference (the oracle)
```

`scripts/run-tests.md` is the contract for the frozen oracle. Budgets are enforced by a **screen-then-confirm** regime:
a parallel run over `ceiling * 5/2` records an advisory screening finding (`BUDGET WATCH:` /
`PARALLEL-SENSITIVE:`), and only a solo measurement over `ceiling * 3/2` — from `-j 1`, or from a
scheduled serial confirmation the runner triggers after repeated overruns — is an authoritative
breach (`SERIAL CONFIRMED OVER BUDGET:`). A default run reports its findings loudly and still exits
`0`. `--strict-budget` confirms every current candidate immediately and opts into the failing exit;
`--no-budget-check` skips the comparison entirely, so nothing is measured or reported.

Exit `0` green (including a green run that breached a budget), `1` a test failed, `3` green but at
least one target produced no result at all — the run certified nothing about it, `4` a breach under
`--strict-budget`, `2` usage error, `130`/`143` interrupted by `SIGINT`/`SIGTERM`.

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
   in `tests/runtime-budgets.tsv`, or `tests/test_runtime_budgets.sh` fails.

Topic is the usual guide, but not always the deciding one. In the `test_harness_defaults*.sh`
family the cost is *per `hd_validate` sweep* and near-uniform per call (change 0227, Task 4), so an
added assertion's placement there should follow **whether it calls `hd_validate`**, not which topic
it nominally belongs to: a non-validating assertion is nearly free in either shard, and a validating
one costs the same wherever it lands.

**Never grow a file past its budget and raise the number.** `tests/test_runtime_budgets.sh` checks
row completeness (every test file has a row, and no row is orphaned), counts over-ceiling rows
separately from it, counts `serial` pins, pins the table's **total** at `EXPECTED_TOTAL`, and
asserts that the resolved `finalize.test_command` runs the runner and does not pass
`--no-budget-check`. The total is what catches the quiet edit: a row moved from 35 to 60 breaks no
ceiling and pins nothing serial, but it moves the sum, so it reddens on its own. If a file legitimately
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

**The enforcement.** `scripts/run-tests.sh` runs `scripts/check-test-source-hygiene.sh` over every
target synchronously before the first job launches. A violation aborts the run with exit **5**,
having executed **zero** test files — the point being that nothing dangerous runs before the check.
The gate is fail-closed: a missing or unreadable checker refuses the run with exit `2` rather than
skipping itself. The checker reports `file:line: CLASS`, with classes `NORMAL-BACKTICK`,
`DQ-BACKTICK`, `HEREDOC-BACKTICK`, `EVAL-BACKTICK`, and `DEFN-DRIFT` (an assert-family definition
that is not byte-exactly canonical). `tests/test_assert_hygiene.sh` is its regression test, driving
committed fixtures under `tests/fixtures/hygiene/`; see `scripts/run-tests.md` for the exit-code
contract.

**The limitation.** The preflight protects **suite runs only**. `bash tests/test_x.sh` run directly
bypasses it entirely, so a violation you introduce is live in exactly the loop where you are most
likely to run one file over and over. Run the file through `scripts/run-tests.sh` before you trust it.

## Parallel-safety

`scripts/run-tests.sh` gives every job its own `HOME`, `TMPDIR`, `XDG_CONFIG_HOME`, and git config
(with a synthetic identity), and pins git non-interactive. A test must not read the ambient `$HOME`,
write global git config, use a fixed `/tmp/<name>` path, touch this repo's own worktrees, or reach
the network. A file that genuinely must share the real tree carries `serial` in the budget table —
and that pin is counted by the guard, so it has to be justified.
