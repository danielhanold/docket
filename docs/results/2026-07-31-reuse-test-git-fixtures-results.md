<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0174 — Reuse test git fixtures instead of rebuilding them per assertion](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0174-reuse-test-git-fixtures.md)**
<!-- docket:backlink:end -->

# Reuse test git fixtures instead of rebuilding them per assertion — results
Change: #0174 · Branch: feat/reuse-test-git-fixtures · PR: <url> · Plan: docs/superpowers/plans/2026-07-31-reuse-test-git-fixtures.md · ADRs: none

## Verify (human)

- [ ] **Decide whether a ~12% suite saving justifies this diff.** The change does exactly what it
  says and is fully verified, but its stated justification was wrong by roughly 7x — see Finding 1.
  This is the one judgement call that belongs to you rather than to the build.
- [ ] Confirm you are content that four near-identical helper bodies were kept independent rather
  than factored into a shared `tests/lib/` (rationale in Finding 4). This is the reversible half of
  the design; the other half is not.

## Findings

### 1. The change's own performance premise was wrong by ~7x (the headline finding)

The spec asserted ~0.5–0.8s per fixture and **~165s of 530s** recoverable. Both figures are wrong.
Measured on this machine, fixture construction in isolation:

| file | helper | calls | old body | new body | saved |
|---|---|---|---|---|---|
| `test_docket_config.sh` | `mkrepo` | 121 | 15.4s | 4.0s | 11.4s |
| `test_docket_status.sh` | `git_repo_setup` | 29 | 1.40s | 0.83s | 0.6s |
| `test_board_checks.sh` | `new_repo` | 34 | 6.1s | 1.4s | 4.7s |
| `test_closeout.sh` | `new_repo` | 29 | 5.3s | 1.3s | 4.0s |

Real per-fixture cost is **~0.127s**, not 0.5–0.8s. End-to-end, the four files went **193s → 170s
(~23s, ~12%)** rather than the promised ~165s. The implementation captured essentially all of the
saving that was actually available — the estimate was the defect, not the code.

Where the time really goes: `test_docket_config.sh` spends ~105s of its ~109s in 121 real
`bash scripts/docket-config.sh` invocations. That is invocation cost (the spec's cause **B**), not
fixture cost (cause **A**), and no open change covered it — now minted as **#0176**.

Consequence worth carrying forward: **the suite's cost is dominated by script invocation, not
fixture construction.** Change 0175 should inherit that framing rather than the spec's.

### 2. A green suite and an intact assertion-label set both passed while the optimization did nothing

The plan specified lazy template init (`[ -n "$TEMPLATE" ] || _build_template`) inside each helper.
For both `new_repo` files that is silently broken: they are consumed as `read -r W _ < <(new_repo)`,
a **subshell**, so the assignment never reaches the parent and every call rebuilt the template.

That defective form ran **green**, with the full pre-existing assertion set intact and an empty
`comm -23` guard — while being *slower than baseline* (18.9s vs 17.2s). Every fixture really was
freshly built, so every independence assertion passed for the wrong reason. The only signal that
caught it was the per-file **wall-clock** check.

Fixed by initializing once at file scope and allocating roots via `mktemp -d "$TEMPLATE/fXXXXXX"`.
Applied to all four files; the reason is recorded in-file so it is not "simplified" back.

### 3. The plan's scripted RED step could never have gone red

The plan claimed an undefined template global would abort each file under `set -u`. It does not:
every dereference sits inside a command substitution, so `set -u` kills only the subshell and the
parent continues with an empty string. Tasks 2–4 substituted **mutation evidence** against the
implemented helper — delete the `remote set-url` line; symlink the origin; symlink the work tree;
drop a seeded ADR — and confirmed every new assertion reddens under at least one mutation.

Two related honesty notes from review:
- In `test_docket_config.sh` the URL-rewrite mutation reddens **121** assertions and in
  `test_closeout.sh` **14** — but in `test_docket_status.sh` only **1**, and that one merely
  restates the implementation line. The "URL rewrite is the correctness core" framing is true for
  three of the four files, not all four.
- Two assertions per block ("a sibling/template worktree never sees the mutation") cannot fail given
  `cp -R` semantics. They are harmless but inflate the apparent strength of each block.

### 4. Four independent helper bodies, deliberately not a shared library

The shared part is three lines (`cp -R`, `cp -R`, `set-url`); the variance is everything else —
three directory layouts and four different seeded contents. A parameterized shared helper would
have flattened that variance, and there is no `tests/lib/` in this repo today (71 flat test files),
so introducing one is its own design decision. Recorded as a plan decision rather than an ADR:
it is a test-local convention, not an architectural contract.

`d1_fixture` in `test_closeout.sh` (6 calls) is deliberately **not** templated — it builds
`git worktree` fixtures whose administrative files record absolute paths and cannot survive a copy.

### 5. Independence was verified at the filesystem level, not just by assertion

Review probed a freshly minted fixture of each shape for any residual channel to the template:
no `objects/info/alternates`, no shared inodes (`find -links +1` empty), and the template's absolute
path appears in **zero** files of a copy — the sole absolute path being the correctly rewritten
`remote.origin.url`. An instrumented end-of-run probe confirmed no test mutates the shared template,
despite tests doing orphan branches, `git rm -rf .`, pushes, and worktree adds.

That probe then became permanent: each file now carries a **template-integrity assertion** just
before its final `exit`, so the independence property is checked across the whole run rather than
only at file scope where the original block sat. Mutation-tested — committing into a template
mid-file reddens it in all four files.

## Follow-ups

- **#0176** (minted from this build) — `docket-config.sh` costs ~0.87s per invocation and dominates
  `test_docket_config.sh` (~105s of ~109s). The largest remaining suite item after `sync-agents.sh`,
  and covered by neither #0175 (scoped to `sync-agents.sh`) nor #0150 (the missing parallel runner).
- **#0175** should be re-based on Finding 1's framing: suite cost is invocation-bound, and the
  spec-level estimate style that produced the 7x error here is worth avoiding there.
- Not fixed, deliberately, as below the bar for this change (all from review, all Minor):
  - Template-build failure is now **sticky** — each builder assigns its global on the first line, so
    a partial failure leaves a non-empty path and later calls copy a broken template instead of
    retrying. Previously each call rebuilt from scratch.
  - `new_repo`'s `root="$(mktemp -d …)"` is unguarded; an empty `$root` would `cp -R` to `/origin.git`.
  - `mkrepo` gained an unconditional `rm -rf "$dir" "$bare"`. Safe today (all 114 target paths
    checked for duplication and prefix collision) but a footgun for a future test that seeds `$dir`
    before calling it; the rationale lives in the plan, not the code.
  - `test_closeout.sh` still leaks one template root — its later `trap … EXIT` at line ~604 would
    replace an earlier one, so no trap was added. `test_board_checks.sh` (which had no trap at all)
    did get one, cutting its leak from 34 roots to zero.
