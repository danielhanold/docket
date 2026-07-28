<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0130 — Make the finalize marker reachability guard portable to BSD grep](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0130-make-the-finalize-marker-reachability-guard-portable-to-bsd.md)**
<!-- docket:backlink:end -->

# Make the finalize marker reachability guard portable to BSD grep — results
Change: #0130 · Branch: feat/make-the-finalize-marker-reachability-guard-portable-to-bsd · PR: <url> · Plan: docs/superpowers/plans/2026-07-28-bsd-grep-interval-portability.md · ADRs: none

## Verify (human)

- [ ] Nothing interactive. The automated evidence is complete: 65 test files, 0 red, under `PATH=/usr/bin:$PATH` (BSD grep 2.6.0-FreeBSD) and under the ambient PATH (ugrep 7.5.0 / Homebrew GNU grep 3.12). If you want the one-command confirmation of the bug this closes, run `PATH=/usr/bin:$PATH bash tests/test_grep_portability.sh` — it prints which `grep` it resolved on its first line.

## Findings

**1. The design's BRE claim was wrong, and the gap was live — caught only at the whole-branch review.**

The spec (§Assumptions A3) and the plan both asserted that the ERE extraction pattern `\{[0-9]+(,[0-9]*)?\}` "also matches BRE `\{m,n\}`", and called the over-match harmless. It does not match it at all: against the source text `\{0,600\}` the pattern consumes `{`, then `600`, then requires `}` and finds `\`. The guard shipped covering roughly half the surface its own header comment claimed.

This was not theoretical. 21 tracked non-`docs/` files use BRE intervals (`\{0,1\}`, `\{4,\}`) in `sed` and plain `grep` today, and the hazard is identical to the ERE one:

```
/usr/bin/grep 'a\{0,600\}' f    -> grep: maximum repetition exceeds 255
/usr/bin/sed  's/a\{0,600\}/X/' f -> sed: RE error: maximum repetition exceeds 255
```

Fixed by making both delimiters optionally-backslashed (`'\\?\{[0-9]+(,[0-9]*)?\\?\}'`) plus a backslash strip in `offenders()`. **The strip is load-bearing and its absence is a silent fail-open** — without it `${interval#\{}` no-ops, the numeric comparison errors on `\{0`, and a real BRE violation is found by the scan but never reported. The re-reviewer verified that directly: with the strip removed and a genuine `\{0,600\}` planted in a tracked file, the main check printed `ok - no ERE repetition bound above 255`. A BRE positive control and a legal-BRE negative control now cover both halves.

Three per-task reviews passed green before this surfaced at the whole-branch review — the same shape recorded in the `backstop-must-compute-not-reenumerate` finding, where the population-level defect survived five per-task reviews.

**2. The plan's own guard source violated the invariant the guard enforces.**

The plan embedded the complete guard text for verbatim transcription, and that text wrote `{0,600}` literally inside four header comments. Since the guard is deliberately *not* self-excluded, transcribing it faithfully made the plan's own stated success criterion (`exit=0`, zero `NOT OK`) unreachable — staging the file produced a self-scan failure naming those exact four lines. The implementer diagnosed it, reworded only those four comments, and reported the deviation rather than silently diverging; the reviewer diffed the shipped file against the plan's block line-for-line and confirmed nothing else moved.

Worth generalizing: a guard that scans its own population makes the *documentation* of the pattern it forbids a live constraint on the guard's source. The plan author has to write the header without the very literal the header is about.

**3. `{,n}` (no lower bound) is deliberately not covered — measured, not assumed.**

The first reviewer flagged that `INTERVAL` cannot match `{,600}` because a digit before the comma is mandatory, and the ledger initially recorded it as a silent-escape risk. The final review measured it instead: `/usr/bin/grep -E 'a{,600}' f` exits 1 with **no error**, because BSD does not parse that form as an interval at all — it treats the braces as literals. So `{,n}` cannot produce `maximum repetition exceeds 255`, which is the entire hazard class. It is a GNU-only semantic extension whose failure mode is a red assertion on BSD, not a green one. Ruled out of scope with the measurement recorded, not with an assumption.

**4. No ADR.** Spec A8 defaulted to none unless a non-obvious trade-off surfaced while building. The one that surfaced — the BRE gap — is a defect fix, not an architecture choice, and ADR-0050 already supplies the governing rule for how this class of backstop is built (derive the predicate, mutation-test the population). The 255 floor itself is an external tool constraint rather than a docket design decision. A8 stands.

**Mutation evidence.** Every cell matched prediction: the repaired assertion reddens exactly one assert when the marker-write clause is deleted; the guard reports a re-introduced `{0,600}` in `tests/test_finalize_disposition.sh`; the self-membership assert is red until the file is staged; a `{0,600}` and a `\{0,600\}` appended to `AGENTS.md` (tracked, non-`.sh`) each redden the guard; the same literal under `docs/` leaves it green; `{0,255}` and legal `\{0,1\}` are not flagged; `{256}` is.

## Follow-ups

- **Change #0150** (minted during this build's reconcile pass) — pin or report the resolved shell toolchain across the suite. This change closes the specific grep-bound hole and adds one informational line naming the resolved `grep`, but there is still no suite runner and no coverage of the sibling GNU-vs-BSD divergences (`sed -i`, `awk`, `date`, `readlink`). Spec A4 scoped it out deliberately.
- **Deferred minor, recorded not fixed:** `offenders()` parses `scan_file` output by splitting on the first colon. `grep -no` emits `lineno:match` and a brace interval contains no colon, so the split is unambiguous today — but a future change to `scan_file`'s flags breaks it silently. Flagged in the plan's self-review and left as a comment-level risk.
