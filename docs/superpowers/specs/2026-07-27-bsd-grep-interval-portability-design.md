<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0130 — Make the finalize marker reachability guard portable to BSD grep](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-07-28-0130-make-the-finalize-marker-reachability-guard-portable-to-bsd.md)**
<!-- docket:backlink:end -->

# Design — make the finalize marker reachability guard portable to BSD grep (change 0130)

## Problem

`tests/test_finalize_disposition.sh:186` asserts that the `## Finalize blocked` marker write is
reachable from the abort-and-report procedure:

```sh
assert "SKILL wires the marker write into the abort-and-report surfacing step" \
  'grep -Eqi "where the reason surfaces.{0,600}appends the .{0,4}## Finalize blocked" "$FIN"'
```

BSD grep rejects an ERE repetition bound above 255 (`grep: maximum repetition exceeds 255`), so on a
stock macOS the assertion errors out before it ever inspects the skill — a portable whole-suite
green run is impossible. It passes on the maintainer's machine only because PATH `grep` there is
`ugrep 7.5.0`, which accepts the bound. **A passing PATH-`grep` run is not evidence.**

Measured facts (2026-07-27, integration branch):

- The matched text lives on **one** line — `skills/docket-finalize-change/SKILL.md:168`, 481 chars —
  and the gap between the two anchors is **301** chars, i.e. genuinely above 255. Shrinking the
  bound is not available.
- Across the maintained source surfaces (`tests/ scripts/ skills/ agents/ cursor-rules/` and the
  root `*.sh` / `*.md`), exactly **one** interval literal exceeds 255:
  `tests/test_finalize_disposition.sh:186`.
- Under `docs/` there are **four** more today
  (`docs/changes/archive/2026-07-26-0124-backlog-triage-pass.md:96`,
  `docs/superpowers/plans/2026-07-08-multi-harness-agent-generation.md:405`,
  `docs/superpowers/plans/2026-07-08-per-harness-agent-models.md:508,509`), and this change's own
  change file and spec will add more when `terminal_publish: true` copies them onto the integration
  branch at close-out. `docs/` is therefore an explicit, justified exclusion (A5).

## What changes

1. **Fix the one assertion.** Replace `.{0,600}` with an unbounded within-line `.*`:

   ```sh
   'grep -Eqi "where the reason surfaces.*appends the .{0,4}## Finalize blocked" "$FIN"'
   ```

   `grep` is line-based, so `.*` still confines the match to a single line and still enforces the
   ordering of the two anchors — the two properties the assertion actually rests on. The remaining
   `.{0,4}` (backtick/emphasis slack) is far below 255 and stays.

2. **Add a static portability guard** — a new `tests/test_grep_portability.sh` — that fails if any
   maintained source file contains an ERE interval literal (`{m,n}` / `{m,}`) with a bound above
   255.

   - **Walk is computed, not enumerated:** the population is **every tracked path** —
     `git ls-files` anchored on the repo root resolved from `BASH_SOURCE` (never cwd-relative;
     this also excludes `.worktrees/` and untracked scratch for free) — minus one exclusion prefix,
     `docs/`. **No extension filter**: an extension list is the same re-enumeration on a different
     axis, and A3 shows no false positive is possible at a >255 threshold. Binary safety comes from
     `grep -I`, not from a filename pattern. Follow
     `tests/test_comment_anchor_style.sh:72`'s `git ls-files` precedent, and carry its
     population-collapse sentinel (assert the scanned count is plausibly large) so an empty walk
     cannot read as green.
   - **Tracked-only is a known edge:** a source file added in the same in-progress commit is not
     yet in the population. Accepted — the guard runs in CI and locally after `git add`, and the
     self-membership assert in §3 makes the gap visible rather than silent.
   - **`docs/` exclusion is deliberate and documented in the file's header comment** — published
     terminal records, archived change files and historical plans legitimately quote defective
     patterns verbatim and are immutable point-in-time records; rewriting them is forbidden by
     convention. Every other tracked surface is in scope automatically, including any new top-level
     directory added later.

3. **The guard must pass its own scan.** No literal >255 interval may appear in
   `tests/test_grep_portability.sh`. Every such literal the guard needs — its own fixtures, its own
   documentation examples — is **assembled at runtime** (string concatenation or arithmetic), never
   written literally. The guard asserts explicitly that its own file is in the scanned population
   and is clean. Two build-time traps for the plan: the self-membership assert stays red until the
   new file is `git add`ed (a red for the wrong reason), and the guard must itself avoid GNU-only
   constructs (`grep -P`, `\d`, `-z`) — the §Verification prepend catches that empirically.

4. **Non-vacuity by fixture, not by environment.** The guard proves it bites against temp fixtures:
   one whose (runtime-assembled) content carries a 600 bound must be flagged; one carrying a 255
   bound must not be. Correct on GNU grep, BSD grep and ugrep alike; needs no local BSD grep to be
   meaningful. The guard additionally prints one informational, non-gating line naming the resolved
   `grep` and its version, so a reader can see which tool a run actually exercised.

5. **Mutation-proof both halves** (build-time verification, not shipped code):
   - the repaired assertion — delete the `**and appends the \`## Finalize blocked\` marker to the
     change file**` clause from `skills/docket-finalize-change/SKILL.md`, confirm ONLY that
     assertion reddens, restore;
   - the new guard — per ADR-0050's corollary, mutate its **population**, not only its suppression:
     confirm it reddens when a >255 literal is introduced into a real tracked file **whose
     extension is NOT `.sh`** (e.g. a `.md` or `.yml` under `skills/` or the repo root — picking a
     `.sh` file would leave an accidental extension-axis blind spot unexposed), and confirm it
     stays green when the same literal is introduced under `docs/` (proving the exclusion is the
     intended one and not an accident of the walk).

## Verification (mandatory shape)

Every verification of this change runs with `/usr/bin/grep` resolving first —
`PATH=/usr/bin:$PATH` (**prepend**, never replace: roughly a dozen test files call `jq`/`gh`, which
resolve from Homebrew paths, and `finalize.gate: local` runs the whole suite). A green ambient-PATH run proves
nothing about the bug being fixed. Both `tests/test_finalize_disposition.sh` and
`tests/test_grep_portability.sh` must pass under the prepended PATH and under the ambient one, and
the whole suite must be green.

## Out of scope

- Any change to finalize behavior or the `## Finalize blocked` contract.
- Rewriting other disposition assertions, or rewriting the four existing `docs/` occurrences.
- Pinning or wrapping `grep` across all 63 test files, or a suite-wide runner that reports the
  resolved toolchain. Recommended as a possible follow-up, deliberately not built here (A4).

## Assumptions

Every decision below was defaulted autonomously; this is the audit trail.

**A1 — Fix shape: unbounded within-line `.*`.**
Chosen over (a) chaining two sub-255 intervals (`.{0,255}.{0,150}`), and (b) a two-stage
`grep … | grep …` pipe. Chaining preserves the numeric bound but is opaque and re-breaks the moment
the prose grows past the new total; the pipe loses anchor ordering. The bound was never the
load-bearing constraint — `grep`'s line orientation is, and the two anchors are distinctive prose
occurring on no other line in the file. The mutation proof in §5 is what shows the assertion still
bites. Residual risk: if that 481-char line is ever merged with unrelated text, `.*` could span
further than intended; accepted, because the mutation proof is re-runnable.

**A2 — A repo-wide static guard, not a one-line fix; in its own file.**
Fixing only line 186 leaves the class open: nothing stops the next long-prose assertion from
reaching for `{0,600}`, and it will pass on the maintainer's machine. Homes weighed and rejected:
`tests/test_finalize_disposition.sh` (a change-scoped guard for change 0087 — a repo-wide invariant
does not belong inside it); `tests/test_comment_anchor_style.sh` and
`tests/test_script_contracts_coverage.sh` (existing repo-wide invariant files, but each owns a
single distinct invariant with a long explanatory header — folding an unrelated second invariant in
would blur both). A dedicated file keeps one invariant per guard.

**A3 — Static bound scan, never a runtime grep-flavor probe.**
Rejected: asserting that `/usr/bin/grep` *rejects* a >255 bound. On Linux `/usr/bin/grep` is GNU
grep and accepts it, so that assertion would be a platform-dependent false failure. A static bound
scan is the only predicate true on every platform, and it is exactly the property wanted (source
portability), not a property of the machine running the suite. The extraction pattern also matches
BRE `\{m,n\}` and non-regex braces; harmless at a >255 threshold, since the repo's entire interval
inventory tops out at 600 and no non-regex brace construct carries a number that large.

**A4 — Do not pin or report the toolchain suite-wide.**
The triage note asks whether the suite should pin or report which grep it ran under. Building that
touches all 63 tests (there is no suite runner) and is a separate design; the conservative default
is the informational line in the new guard plus the mandatory PATH-prepend verification posture, and
a recommendation recorded for a human. Not minted as a stub — `docket-auto-groom` is never a mint
site.

**A5 — Computed walk over `git ls-files`, with `docs/` explicitly excluded.**
A hand-enumerated directory list would leave any new top-level source directory silently unguarded,
which is precisely what ADR-0050 (backstop checks must compute, not re-enumerate) rules out; the
walk is therefore derived from tracked files, with **no extension filter** — an extension list
would re-introduce the same blind spot on a different axis (`.mdc`, `.py`, an extensionless hook)
and buys nothing given A3. `docs/` is the one exclusion, and it is a *decision*,
not an accident: four >255 intervals live there today (listed in §Problem) inside archived change
files and historical plans, and `terminal_publish: true` will publish this change's own file and
spec — which quote `{0,600}` verbatim — into `docs/` at close-out. Those artifacts are immutable
records the convention forbids rewriting, so the guard must not demand a repair it cannot legally
have. Prose surfaces that *are* maintained (`skills/`, `agents/`, `cursor-rules/`, root `*.md`)
stay in scope, because a pattern copied out of live prose into a test carries the defect with it.

**A6 — Dependencies.** `depends_on` stays empty. Neither open feature branch touches
`tests/test_finalize_disposition.sh`, and no active change adds a grep-portability test; the new
test file is a fresh path, so no file-collision coupling exists at design time. `discovered_from:
[116]` already records provenance. The implementer's reconcile re-validates.

**A7 — `type: fix`, `priority: medium` unchanged.** The bug blocks a portable green suite but has
no runtime effect on docket users; no escalation is warranted.

**A8 — No ADR.** The change does establish a durable repo-wide source invariant (a 255-bound ERE
portability floor). It is left un-ADR'd because the decision it encodes is an external tool
constraint, not a docket architecture choice, and ADR-0050 already supplies the governing rule for
how such a backstop is built. If the implementer finds itself arguing a non-obvious trade-off while
building the guard, `docket-adr` is available at step 6 — this assumption is a default, not a bar.
