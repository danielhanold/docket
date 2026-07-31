<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0174 — Reuse test git fixtures instead of rebuilding them per assertion](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-07-31-0174-reuse-test-git-fixtures.md)**
<!-- docket:backlink:end -->

# Reuse test git fixtures instead of rebuilding them per assertion

## Problem

The suite takes ~530s wall clock across 71 files, and the cost is concentrated in a handful of
files that rebuild a git repository from scratch for every assertion group.

Measured per-file (2026-07-31, `for f in tests/*.sh; do time bash "$f"; done`, sequential):

| file | wall clock | share |
|---|---|---|
| `test_sync_agents.sh` | 197.8s | 37% |
| `test_docket_config.sh` | 103.2s | 19% |
| `test_sync_agents_codex.sh` | 66.8s | 13% |
| `test_docket_status.sh` | 29.1s | 5% |
| `test_render_board.sh` | 17.8s | 3% |
| `test_board_checks.sh` | 17.3s | 3% |
| `test_closeout.sh` | 16.3s | 3% |
| `test_sync_agents_cursor.sh` | 14.3s | 3% |
| the other 63 files, combined | ~67s | 13% |

Three distinct causes hide behind that table, and they need different fixes:

- **(A) clone-per-fixture.** `test_docket_config.sh` calls `mkrepo` **122 times**;
  `test_board_checks.sh` calls `new_repo` **37 times**; `test_closeout.sh` **31 times**;
  `test_docket_status.sh` calls `git_repo_setup` **30 times**. Each call spawns ~8–10 git
  processes — `init --bare`, `clone`, two `config`, `checkout -b`, a commit, `push -u`, and
  sometimes `remote set-head` — for a baseline that is byte-identical every time. At ~0.5–0.8s per
  fixture this is the whole cost of those four files.
- **(B) script-invocation cost.** `test_sync_agents*.sh` (279s combined, 53% of the suite) is not
  git-bound; it repeatedly invokes the real `sync-agents.sh`, which costs ~5.5s per run — even
  `bash sync-agents.sh --help` pays it, because `--help` is not a recognized flag and falls
  through into a full generation pass. `test_render_board.sh` is the same class at a smaller
  scale: ~163 invocations of `render-board.sh` at ~0.15s each.
- **(C) no parallelism.** There is no suite runner; the files run sequentially in a bare loop.

**This change addresses (A) only.** (B) is tracked separately — it is a change to shipped script
behavior, not a test-only refactor, and deserves its own design and its own review argument.
(C) is deliberately out of scope: introducing a suite runner is its own design, and change 0150
already records the missing runner as an open gap.

Addressable here: `test_docket_config.sh` + `test_docket_status.sh` + `test_board_checks.sh` +
`test_closeout.sh` ≈ **165s of 530s**. `test_render_board.sh` is explicitly excluded despite
appearing in the top eight — it is cause (B), and `cp -R` would buy it almost nothing.

## Design

### The technique

Each affected file keeps its fixture-builder helper's **name and signature exactly as they are**,
and changes only the body:

1. On first call, build the baseline repository once into a per-file template directory.
2. On every call — including the first — produce the caller's fixture by **copying** the template
   (`cp -R`), then rewriting the copy's `remote.origin.url` to point at the copied origin.

A directory copy of a small repo is a fraction of the cost of `init` + `clone` + commit + `push`,
and every one of the 220 call sites across the four files stays untouched. That is the point of
holding the signature fixed: the diff is four helper bodies, and the review question is a single
one — *are the copied fixtures still independent?*

### Why the URL rewrite is load-bearing

The two fixture shapes place the bare origin differently, and both embed an **absolute** path in
the clone's `.git/config`:

- `mkrepo <dir>` (docket_config) puts the bare origin at the sibling path `<dir>.origin.git`.
- `new_repo` (board_checks, closeout) allocates `root=$(mktemp -d)` and uses
  `$root/origin.git` + `$root/work`.
- `git_repo_setup <root>` (docket_status) builds `$root/seed` then clones it bare to
  `$root/origin.git`.

So the copy must take **both** the working clone and its bare origin, and then repoint
`remote.origin.url` at the copy's origin. Skipping the rewrite would leave every fixture pushing
into the *template's* origin — which would not fail loudly; it would silently couple fixtures
that the tests assume are isolated. The rewrite is the correctness core of this change, not a
detail.

### Independence invariant

The property that must hold after this change, stated so it can be tested rather than argued:

> Mutating one fixture — committing, pushing, branching, deleting files — must have no observable
> effect on any other fixture, and none on the template.

This gets an explicit test, not just suite-green. The suite passing is necessary but not
sufficient: a fixture-coupling bug shows up as order-dependent flakiness, which a single green run
does not rule out.

### Scope

In scope — four helper bodies:

| file | helper | call sites |
|---|---|---|
| `tests/test_docket_config.sh` | `mkrepo` | 122 |
| `tests/test_board_checks.sh` | `new_repo` | 37 |
| `tests/test_closeout.sh` | `new_repo` | 31 |
| `tests/test_docket_status.sh` | `git_repo_setup` | 30 |

Cleanup rides the existing per-file `trap 'rm -rf …' EXIT`; the template directory is created
under the same temp root the file already owns, so no new cleanup path is introduced.

## Out of scope

- `sync-agents.sh`'s per-invocation cost and the three `test_sync_agents*` files — cause (B),
  tracked as its own change.
- `test_render_board.sh` — cause (B) at smaller scale.
- A parallel suite runner — cause (C); see change 0150.
- Any change to what the tests *assert*. This change makes existing assertions cheaper; it must
  not delete, weaken, or merge a single one. The assertion count per file is expected to be
  identical before and after.

## Open questions

1. **Do any assertions depend on fixtures having distinct commit SHAs?** Today every `mkrepo`
   call produces a different baseline SHA (different commit timestamps). Template reuse makes the
   baseline SHA identical across every fixture in a file. If any test compares SHAs across two
   fixtures expecting them to differ, it will start passing or failing for the wrong reason.
   This must be checked per file before the helper body is swapped — not discovered by a red suite.
2. **Does `test_board_checks.sh`'s commit-ageing interact with a fixed template?** It ages commits
   relative to a pinned `NOW_EPOCH`. The expectation is that this is unaffected, because the tests
   set dates explicitly rather than relying on the baseline commit's own date — but the baseline
   commit date does become fixed at template-build time, so it needs confirming.
3. **Is `remote set-head origin -a` still needed after a copy?** `mkrepo` runs it so `origin/HEAD`
   resolves; whether that survives `cp -R` + URL rewrite, or must be re-run per fixture, decides
   whether the copy path is two commands or three.
4. **Should the four helpers share one implementation?** They are near-identical in shape but
   differ in layout and in what they seed. A shared helper in a common test library would prevent
   the technique being re-derived a fifth time; four independent bodies keep the diff local and
   each file self-contained. Worth deciding during planning, not now.

## Verification

- Every affected file's assertion count is unchanged, checked mechanically (count `ok - ` /
  `NOT OK - ` emissions before and after), not by reading the diff.
- An explicit fixture-independence test per changed helper: build two fixtures, mutate the first,
  assert the second and the template are unaffected.
- Before/after wall clock recorded per changed file in the results doc, using the same measurement
  method as the table above, so the claimed saving is a measurement rather than an estimate.
- Full suite green.
