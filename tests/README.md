# docket's test suite

79-plus standalone Bash files, discovered by the `tests/test_*.sh` glob — there is no registry, so
a new file self-registers. Each file is hermetic: `set -uo pipefail`, its own tmpdir fixtures, no
ordering dependencies, runnable on its own as `bash tests/test_X.sh`.

## Running it

```
scripts/run-tests.sh             # parallel, all files, budgets enforced
scripts/run-tests.sh -j 1        # serial reference
scripts/run-tests.sh --verbose tests/test_docket_config.sh   # one file, full output
```

`scripts/run-tests.md` is the contract. Exit `0` green, `1` a test failed, `4` green but a file
blew its wall-clock budget, `2` usage error.

## Where new tests go

The suite is parallel, and its wall-clock floor is its **slowest single file** — not its total. A
file that grows past its budget slows every future build, so placement is a real decision:

1. **Extend the topical shard your assertion belongs to.** This is almost always right. Find the
   file already covering that subsystem and add to it — if it has room in `tests/runtime-budgets.tsv`.
2. **If that shard has no room, extend a sibling shard or start a new one.** `test_sync_agents*.sh`
   and `test_docket_config*.sh` are already split this way; adding `_<topic>` to the family is
   cheap and keeps every part under its ceiling.
3. **A brand-new file is for a brand-new subsystem** — a new script, a new surface. It needs a row
   in `tests/runtime-budgets.tsv`, or `tests/test_runtime_budgets.sh` fails.

Topic is the usual guide, but not always the deciding one. In the `test_harness_defaults*.sh`
family the cost is *per `hd_validate` sweep* and near-uniform per call (change 0227, Task 4), so an
added assertion's placement there should follow **whether it calls `hd_validate`**, not which topic
it nominally belongs to: a non-validating assertion is nearly free in either shard, and a validating
one costs the same wherever it lands.

**Never grow a file past its budget and raise the number.** The budget guard counts over-ceiling
rows separately from row completeness, so that edit reddens on its own. If a file legitimately
cannot be split, that is a decision to argue in the diff, not a number to bump. Two files were
argued that way at change 0227 and both are still whole:

- `test_sync_agents_codex.sh` — no internal section banners, so there is no mechanical boundary.
- `test_docket_config.sh` — it carries the change-0126 prelude-correspondence guard, which scans
  its own `${BASH_SOURCE[0]}` and asserts a whole-file floor of **≥60 `eval` sites** against a
  64-site corpus, with a derived cross-check over the same file. Any split halves the corpus and
  falsifies both. Splitting it means changing that assertion — so do not re-attempt the split
  without deciding, deliberately, what the guard's population should be across several files.

## Parallel-safety

`scripts/run-tests.sh` gives every job its own `HOME`, `TMPDIR`, `XDG_CONFIG_HOME`, and git config
(with a synthetic identity), and pins git non-interactive. A test must not read the ambient `$HOME`,
write global git config, use a fixed `/tmp/<name>` path, touch this repo's own worktrees, or reach
the network. A file that genuinely must share the real tree carries `serial` in the budget table —
and that pin is counted by the guard, so it has to be justified.
