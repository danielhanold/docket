<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0270 — Fence runner-config locality at the main worktree (regression test + contract correction)](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0270-fence-runner-config-locality-at-the-main-worktree.md)**
<!-- docket:backlink:end -->

# Fence runner-config locality at the main worktree

**Change:** #0270 · **Type:** chore · **Date:** 2026-08-09

## Context

`docket.sh runner-dispatch` hands a task to an external AI runner (codex, cursor, opencode). Two
things it resolves look similar and are easy to conflate:

- **The run anchor** — which working tree the child process operates in. Set by `--worktree`, and
  for a `build-*` agent it is *required* to be a feature worktree (ADR-0034). Exported as
  `DOCKET_REPO_ROOT`.
- **The runner config** — the `runners.<name>:` block that supplies per-runner knobs
  (`codex.sandbox`, `opencode.permissions`, …), exported as `DOCKET_RUNNER_CFG_<KEY>`.

These are resolved from **different trees**, deliberately. The anchor is whatever the caller named;
the config is always read at the **main worktree**:

```
scripts/runner-dispatch.sh:116   REPO_ROOT="$(docket_main_worktree)"      # main worktree, always
scripts/runner-dispatch.sh:154   export DOCKET_REPO_ROOT="$ANCHOR"        # the named tree
scripts/runner-dispatch.sh:176   for f in "$REPO_ROOT/.docket.local.yml" "$REPO_ROOT/.docket.yml" "$GLOBAL_CFG"
```

The decoupling is load-bearing because `.docket.local.yml` — the machine-local layer, where a
grant like `runners.opencode.permissions: auto-approve` is written — is **gitignored**
(`.gitignore:8`). A freshly created feature worktree therefore carries no copy of it. If the config
loop were anchored at `--worktree`, every machine-local runner grant would silently vanish the
moment a `build-*` agent was dispatched, and the opencode adapter would refuse the dispatch by
design (`scripts/runners/opencode.sh:44-50`).

**The invariant holds today, and is unfenced and undocumented.** Change 0269's spec recorded the
opposite as fact in its `## Out of scope` — "the adapter resolves config from `DOCKET_REPO_ROOT` =
that worktree … Real, independently reproducible" — and that claim became stub #0270. It was never
reproducible. Two independent code reads plus an empirical probe (grant written only in a main
worktree, dispatch from a sibling feature worktree, env-dumping fake adapter) all confirm the grant
arrives; an adversarial pass over the non-reproduction found no path either. The human who ran 0269
confirms no dispatch ever actually refused — the claim came from reading the code.

That is the finding worth acting on. A competent author read this code and concluded the opposite,
because two things invited the mistake:

1. **`tests/test_runner_dispatch.sh` never crosses the two axes.** Worktree anchoring is tested at
   line 160 (change 0206's block); config-layer precedence is tested at line 220. Both run against
   a default anchor or without config. Nothing asserts that a main-worktree grant survives a
   `--worktree` dispatch, so the invariant could be broken by an ordinary refactor with the suite
   staying green.
2. **`scripts/runner-dispatch.md` step 3 writes the config paths as `<repo>/.docket.local.yml`**,
   immediately after step 2 has defined the anchor as possibly a feature worktree. `<repo>` is
   never bound, and reads naturally as "the anchor". `scripts/runners/opencode.md` compounds it: its
   env table row for `DOCKET_REPO_ROOT` correctly says "the main worktree unless the caller named a
   feature worktree", and the very next row introduces `runners.opencode.permissions` with no
   statement of where that config is read from.

So #0270 is repurposed from "fix a defect" to "fence and document the invariant the defect report
got backwards".

## Decision

Add one regression test crossing anchor × config locality, and correct the two contract documents
that invited the misreading. No production code changes.

### 1. The regression test

**Location:** `tests/test_runner_dispatch.sh`, as a new section after the change-0206 `--worktree`
block (which ends at the `rm -rf "$SBX"` preceding line 220's config-resolution block).

`tests/test_runner_opencode.sh` is **not** the right home even though the provenance is an opencode
grant: that file exercises the adapter in isolation (`ADAPTER="$REPO/scripts/runners/opencode.sh"`)
and never runs the facade. The config loop lives in the facade and is runner-agnostic — it knows no
runner's key names — so the invariant is tested once, at the facade, through the existing codex
argv-recording harness. The section comment must name the opencode provenance so a reader
searching for the permissions story lands here.

**Fixture — a real linked worktree, not a `mkdir`.** The existing block (b)/(c) fixtures create
`$SBX/.worktrees/featslug` with a bare `mkdir -p`. That is inadequate here and would make the test
vacuous. With a plain subdirectory, `docket_main_worktree "$ANCHOR"` trivially returns `$SBX`
because the subdirectory *is* part of the main worktree — the resolution being tested never
happens. A real linked worktree is the only fixture that exercises
`git worktree list --porcelain`'s "main worktree is listed first, from every worktree in the set"
behavior (`scripts/lib/docket-root.sh:27-32`), which is what actually makes the invariant true.

Create it with `git -C "$SBX" worktree add -b <branch> "$SBX/.worktrees/featslug"`.

**Setup:**

- Write the grant to `"$SBX/.docket.local.yml"` **only** — after the worktree is created, and
  never into the worktree. Use a `runners.codex.*` key the fake adapter records in argv;
  `sandbox: danger-full-access` is the established carrier (line 222's block already uses it).
- Write `.docket.local.yml` into `$SBX/.gitignore` so the fixture mirrors production. Absent this,
  the fixture still passes for the wrong reason — the file is untracked either way — but a future
  reader cannot see *why* the worktree lacks it. This line is documentation inside the fixture.
- Invoke the facade with **cwd inside the linked worktree** and `--worktree` pointed at it. The cwd
  matters: it is the production shape (a build worker dispatches from its own tree) and it is the
  condition under which a cwd-derived root would return the *wrong* tree.

**Asserts — three, and all three are required:**

| # | Assert | What it fences |
|---|---|---|
| a | The grant value (`danger-full-access`) appears in the recorded argv | The config reached the child at all |
| b | `DOCKET_REPO_ROOT` handed to the adapter **is** the linked worktree path | Anti-vacuity: the anchor did not silently fall back to the main worktree |
| c | `DOCKET_REPO_ROOT` is **not** `$SBX` | The same, stated as a negative so a prefix-match cannot satisfy (b) |

Assert (b) is what makes this a test of *decoupling* rather than of config resolution. Without it,
a regression that anchored the config loop at `$ANCHOR` **and** made the anchor fall back to the
main worktree would leave (a) green while the invariant was gone.

**Mutation test — mandatory before the change is considered done** (AGENTS.md: *"A guard is code:
mutation-test it — strip the thing it guards, watch it redden — or it is decoration"*). Two
mutations, both must redden this section and only this section:

1. In `scripts/runner-dispatch.sh:176`, change the loop's `"$REPO_ROOT/…"` prefixes to
   `"$ANCHOR/…"` → assert (a) must fail.
2. In `scripts/runner-dispatch.sh:154`, change `export DOCKET_REPO_ROOT="$ANCHOR"` to
   `="$REPO_ROOT"` → assert (b) must fail.

Record both mutation results in the results file. Revert both mutations before committing.

### 2. The contract corrections

**`scripts/runner-dispatch.md`, step 3** (line 77-79). Replace the unbound `<repo>/` prefix with an
explicit statement of the tree and the reason:

- Name the layers as resolved at the **main worktree**, and say so in the same sentence that names
  them — not in a later aside. The reader who misread this stopped at the path list.
- State that this is **independent of `--worktree`**, and give the reason in one clause: the
  machine-local layer is gitignored, so a feature worktree has no copy of it and an anchor-relative
  read would silently drop every machine-local grant on exactly the `build-*` dispatches that
  require `--worktree`.
- Point at `runner-dispatch.sh:116`'s `docket_main_worktree()` call by **symbol name**, not line
  number (AGENTS.md cross-reference rule).

**`scripts/runners/opencode.md`.** Two touches, wording only:

- The env table's `DOCKET_RUNNER_CFG_PERMISSIONS` row (line 57): note that the value is resolved by
  the facade from the **main worktree's** config layers, so a `--worktree` delegation still sees a
  machine-local grant. This is the row sitting directly under the `DOCKET_REPO_ROOT` row that
  *does* say "unless the caller named a feature worktree" — the adjacency is what produced the
  misreading, so the correction belongs adjacent too.
- The Prerequisites bullet (line 138), `runners.opencode.permissions: auto-approve` "in a config
  layer": say which tree that layer is read from, since this is the line a human follows when
  setting the grant up and getting it wrong here is the shape of the reported "defect".

Keep both edits to statements of fact about where config is read. Do not restate the facade's
precedence rules here — `runner-dispatch.md` owns those.

## What changes

- `tests/test_runner_dispatch.sh` — one new section: real linked worktree, main-worktree-only
  grant, dispatch from inside the worktree, three asserts.
- `scripts/runner-dispatch.md` — step 3 rewritten to bind the config tree explicitly.
- `scripts/runners/opencode.md` — env-table row and Prerequisites bullet state the config tree.

No production code changes. If a mutation test fails to redden the new asserts, the test is wrong —
fix the test, not the script.

## Out of scope

- **Editing change 0269's spec.** It is a merged, point-in-time build record; AGENTS.md forbids
  rewriting one, and its `## Out of scope` claim is corrected here rather than there. A reader who
  follows 0269's stale claim arrives at #0270, which is the intended path.
- **Any change to `runner-dispatch.sh` or the adapters.** The invariant is already correct. This
  change only fences and documents it.
- **The `--worktree` gates themselves** — ADR-0034 anchoring, the `build-*` requirement, and gate
  3's membership test. Change 0208 owns hardening those.
- **`runners:` config-reader duplication** — change 0256.
- **Whether `permissions: auto-approve` actually behaves as documented** against a real opencode
  binary. `scripts/runners/opencode.md` already flags that as **Unverified** and it is an external
  truth no in-repo test can be an oracle for.
- **The quoted-value trap.** `scripts/runners/opencode.sh:50` already dies loudly on
  `"auto-approve"` with a hint naming the unquoted-value rule (change 0205). It was a candidate
  cause for the reported refusal; since no refusal ever occurred, there is nothing to fix.

## Coupling and build order

Change **0208** (proposed, groomed, `depends_on: [237]`) edits the same two files — and its spec
already converts the existing (b)/(c) `mkdir` worktree fixtures to real `git worktree add` and adds
a real-worktree fixture of its own. This is a `related:` coupling, **not** a `depends_on`: #0270
stands alone and creates its own linked worktree if it lands first.

Whichever lands second rebases. If 0208 lands first, prefer its real-worktree fixture helper over
introducing a second one — the asserts in this spec are what matter, not the fixture plumbing.

## Verification

- The new section's three asserts pass.
- Both mutations redden the expected assert, and only this section; both are reverted.
- The whole suite is green at the build gate (`finalize.test_command` → `scripts/run-tests.sh`),
  with no `OVER BUDGET:` line for `tests/test_runner_dispatch.sh` — a real `git worktree add` is
  slower than a `mkdir`, so the file's wall-clock budget is a live risk in this change and must be
  checked rather than assumed.
