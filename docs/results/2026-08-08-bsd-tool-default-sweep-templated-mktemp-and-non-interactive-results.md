<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0254 — BSD tool-default sweep: templated mktemp and non-interactive mv](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-08-0254-bsd-tool-default-sweep-templated-mktemp-and-non-interactive.md)**
<!-- docket:backlink:end -->

# BSD tool-default sweep: templated mktemp and non-interactive mv — results

Change: #0254 · Branch: feat/bsd-tool-default-sweep-templated-mktemp-and-non-interactive · PR: <url> · Plan: docs/superpowers/plans/2026-08-08-bsd-tool-default-sweep-templated-mktemp-and-non-interactive.md · ADRs: none

## Verify (human)

- [ ] **The guard's markdown scope is a judgment call worth your eye.** Review finding 3 widened
  `tests/test_bsd_tool_defaults.sh` from shell-only to also cover *agent-executed* markdown —
  `scripts/*.md`, `skills/*/SKILL.md`, `skills/*/references/*.md` (51 files). `docs/` is
  deliberately excluded: archived changes, historical plans, and results files quote the defective
  forms verbatim as the subject under discussion, and the convention forbids rewriting them. Confirm
  you agree with where that line sits — it is the same `docs/`-exclusion precedent
  `tests/test_grep_portability.sh` documents, but this is the second guard to draw it.
- [ ] **One residual gap is shipped named rather than closed.** The mktemp predicate matches
  `$(mktemp` (now spacing-tolerant) but not a backticked `` `mktemp` ``. In markdown prose a
  backticked `mktemp` is byte-identical to a backticked invocation, so no shape separates them; a
  bare-word match produced 8 prose false positives across the newly-scoped surface, all of them
  legitimate text naming the defect. The guard's header comment states the gap. Confirm you accept
  the trade rather than an allowlist (which ADR-0050 forbids).

## Findings

Twelve review findings from the `docket-review-standard` rung. Per-finding disposition, with commit
SHAs, is in the PR body's table; what follows is what is worth remembering.

- **The guard shipped with its own defect class inside it (blocker).** The `git mv` carve-out was
  applied to the whole `path:lineno:content` string, and `[^|]*` spans `/` and `:` — so any file
  whose *path* contained `git` was silently exempt. `scripts/lib/docket-gitignore-block.sh` is in
  scope and matches; worse, the entire mv guard collapsed for any checkout under `~/git/…` or
  `~/github/…`. A guard against "a default that silently defeats a guard" was itself silently
  defeated by a path. This is the `fix-reintroduces-its-own-defect-class` learning firing exactly
  as written — audit a change's own additions against its own thesis.
- **The predicate was narrower than the rule it enforced (important).** Keying on `mv "` meant
  `mv -i "$t" "$f"` — the precise interactive behavior the change exists to prevent — was invisible,
  and the `mv -f` filter downstream was dead code. Now keyed on the command with an explicit
  `-i`/`-n` deny.
- **The sweep missed a live site in agent-executed markdown (important).** `docket-finalize-change`'s
  SKILL.md installs the learnings index with a literal bare `mv` — the same operation
  `scripts/docket-status.sh` had just been hardened for, duplicated as bash an agent runs verbatim.
  Fixed, and the guard's scope widened so the next one cannot hide there. **This is the finding
  worth carrying forward:** `.md` files that carry runnable bash are executable surface, and the
  repo had no guard that treated them as such.
- **cp/rm audit re-verified at build (spec §6, A8): zero sites.** No `cp -i` in scope; every `rm`
  carries `-f`/`-rf`. Exactly the predicted chaff and nothing more — two `git rm` invocations
  (one inside a constructed remedy string in `sync-agents.sh`, one in `migrate-to-docket.sh`) plus
  three comment mentions. No code change.
- **Behavioral coverage is now platform-independent.** The original TMPDIR pin lived inside the
  `chflags uchg` branch, so on Linux the mktemp half had no behavioral coverage at all — only the
  shape guard, which cannot prove `TMPDIR` is honored. A `PATH`-shimmed `mktemp` probe now asserts
  the created path resolves under a redirected `TMPDIR` on any platform; the macOS remnant assert
  is kept alongside it.
- **No ADRs.** Every design decision the build made was settled at groom time in the spec's
  A1–A10. The two judgment calls made during the fix loop (markdown scope; the named backtick gap)
  are both documented in the guard's own header comment and follow existing in-repo precedent, so
  neither mints a decision record.

## Follow-ups

- **Change #0263 (minted by auto-capture)** — *Guard the remaining AGENTS.md Shell rules across
  scripts, tests, and agent-executed markdown.* Verified 2026-08-08: `test_grep_portability.sh`
  covers every tracked path minus `docs/`, so the ERE-bound rule is enforced everywhere; but the
  producer-piped-into-early-exiting-consumer rule, the leading-`--` grep rule, and the awk
  indent-class rule are enforced by **nothing**, on any surface. This change built the markdown walk
  and proved the surface is live; #0263 is the work of applying it to the rules that remain.
- **Plan deviation, already corrected in-branch (`673c6852`).** The plan's Task 4 Step 2 wrote
  `scripts/run-tests.sh -j 1 --timings tests/test_bsd_tool_defaults.sh`. `--timings` takes a **path**
  argument, so the runner consumed the test file as its output path and truncated it to zero bytes
  mid-build. The worker restored it from HEAD (it was committed and unedited) and re-measured
  correctly. Worth knowing generally: `--timings` without an explicit destination will eat the next
  argument.
- **Observation, not acted on.** `scripts/run-tests.md` states that every sandbox root comes from
  `mktemp -d`, "which follows the per-job `TMPDIR`". That sentence describes `tests/`, which this
  change's scope deliberately excludes (test-side hygiene is owned by change #0252), and the sweep
  did not change its truth value — so it was left alone. It is nonetheless the exact belief this
  change disproves for bare `mktemp -d` on macOS. Worth a look when #0252 lands.
- **Measured cost.** The new guard runs in 1s and is budgeted at 10s per the table's rounding rule;
  `EXPECTED_TOTAL` moved 1355 → 1365 with a ledger entry. Full suite: 90 files, 6847 asserts, 124s.
