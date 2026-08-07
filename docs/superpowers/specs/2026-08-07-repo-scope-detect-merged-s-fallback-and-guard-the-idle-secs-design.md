<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0250 — Repo-scope detect-merged's fallback and guard the idle-secs duplication](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0250-repo-scope-detect-merged-s-fallback-and-guard-the-idle-secs.md)**
<!-- docket:backlink:end -->

# Repo-scope detect_merged's fallback and guard the idle-secs duplication — design

**Change:** 0250 · **Date:** 2026-08-07 · **Type:** fix · **Files:** `scripts/docket-status.sh`, `scripts/docket-status.md`, `tests/test_docket_status.sh`. No change to `scripts/board-checks.sh`.

Consolidates the 0219 close-out harvest (#0239 + #0241, both killed into this change). Two independent parts, one small test-heavy PR.

## Part 1 — `--repo` in `detect_merged`'s `gh pr list` fallback (#0239)

### Behavior change

`scripts/docket-status.sh` `detect_merged`, per-change fallback arm (currently ~line 555):

```sh
pl_json="$("$GH" pr list --head "feat/$slug" --state merged --json number,mergedAt 2>/dev/null)"
```

becomes

```sh
pl_json="$("$GH" pr list --repo "$repo" --head "feat/$slug" --state merged --json number,mergedAt 2>/dev/null)"
```

`--repo` is passed **unconditionally**: by the time the fallback executes, `$repo` is always resolved and shape-checked — the function resolves `repo="${REPO_FLAG:-}"`, falls back to `gh repo view`, and returns early (`sweep-skipped gh-unavailable` / `repo-unresolved`) if either fails. There is no live "unresolved repo reaches the fallback" path, so no conditional. This mirrors `detect_orphan_pr` (~line 728) exactly, including a short "`--repo "$repo"` is what SPENDS the resolution above" comment at the call site so the two legs read the same.

No change to the batched graphql arm (already honors the resolution), no change to sweep posture, reasons, or output tokens.

### Tests (0219 argv-recording stub idiom, already in `tests/test_docket_status.sh`)

- New argv-recording `GH` stub for `detect_merged` (modeled on the existing `gh-orphan-argv.sh` fixture at ~line 1085): logs `$*` per invocation, answers `repo view` with `x/y` and `pr list` with a valid merged-PR JSON array.
- Fixture: one `implemented` change with `pr:` **empty** so the fallback arm is taken.
- Asserts:
  1. Non-vacuous witness: a `^pr list` line exists in the argv log.
  2. Every `pr list` invocation carries `--repo x/y` (the `! grep -qvF` every-line idiom from the detect_orphan_pr tests).
  3. `REPO_FLAG` end-to-end: rerun with `REPO_FLAG=someone/elsewhere` assigned AFTER the `. "$SCRIPT"` source (the script's prologue resets it — see the warning at ~line 1122); every `pr list` carries `--repo someone/elsewhere` and no `repo view` subprocess is spent (mirrors the existing detect_orphan_pr REPO_FLAG block at ~line 1129).
  4. The run still emits the merged-candidate line (no regression in the sweep output).

## Part 2 — correspondence guard over the idle-secs values (#0241, ADR-0072)

### Mechanism: test-only textual extraction, no script changes

A new guard block in `tests/test_docket_status.sh`:

1. `grep` `scripts/docket-status.sh` for lines matching `^ORPHAN_PR_IDLE_SECS=` and `scripts/board-checks.sh` for `^ABORTED_RUN_IDLE_SECS=`.
2. Assert each pattern matches **exactly one** line (non-vacuous, and reddens loudly on rename/removal — the extraction anchor failing IS a finding, per the restatement-accumulates-its-own-guards learning).
3. Strip each line to the inner arithmetic expression — remove the `NAME=` prefix AND the `$(( ))` wrapper when present, tolerating a wrapperless `NAME=7200` form — then evaluate in the **test shell** with `val=$(( expr ))` (never by sourcing either script; never a bare `eval` of the raw file-derived line), and assert the two values are equal.
4. In-suite mutation witness (the `test_board_checks.sh:2507` arm-mutation idiom, adopted rather than merely cited): sed-copy `docket-status.sh` with `ORPHAN_PR_IDLE_SECS` retuned to a different value, re-run the extraction+compare against the mutated copy, assert the guard REDDENS, and assert by before/after grep counts that the mutation actually landed. This makes the guard's own sensitivity a suite property, not a one-off manual check.

The "minimal shared sentinel" the stub asked for is the assignment line itself — no shared file, no third component, nothing added to either script's dependency graph. ADR-0072's offline contract is untouched because neither script changes for this part.

### Location

`tests/test_docket_status.sh` — the guard protects the *copy* (`ORPHAN_PR_IDLE_SECS`), and the copy's owner is docket-status. The test reads `board-checks.sh` as data (a file path), not as a dependency.

## Frontmatter

- `adrs: [72]` — already set on the stub (repairs #0241's omission); the change touches no ADR body, so no `docket-adr` invocation.

## Out of scope (restated from the stub, confirmed)

- No shared-helper refactor of the leg C predicate (ADR-0072 stands).
- No guard over the broader predicate *shape* (base handling, ref resolution) — #0241's prose floated it; the 0250 triage consolidation narrowed the guard to the idle-secs values. The shape remains prose-mitigated; if drift there ever bites, that is a new stub, not scope creep here.
- No other `detect_*` leg, no other `gh` call sites.

## Assumptions

1. **`--repo` passed unconditionally, not "when resolved".**
   Chosen: unconditional `--repo "$repo"` in the fallback.
   Rejected: `${repo:+--repo "$repo"}` conditional — dead generality, because detect_merged's early returns guarantee `$repo` is resolved and shape-valid before the fallback runs; a conditional would imply a reachable unresolved path that does not exist.
   Rejected: threading owner/name separately — the sibling `detect_orphan_pr` already fixed this exact shape with a plain `--repo "$repo"`, and matching it keeps the two legs symmetrical.

2. **Correspondence guard = textual extraction + arithmetic eval in the test shell; zero script changes.**
   Chosen: grep each script for its single anchored assignment line, evaluate the RHS with `$(( ))` in the test, compare values.
   Rejected: sourcing either script to read the variables — docket scripts are not pure; sourcing to probe has caused real damage before, and the existing suite's `. "$SCRIPT"` idiom is confined to purpose-built fixture dirs; a pure-text guard is strictly safer and works offline.
   Rejected: a shared constants file both scripts read — that is exactly the "third component both must load" ADR-0072 rejected.
   Comparing *evaluated values* (not RHS strings) tolerates formatting differences (`7200` vs `2 * 3600`) while still reddening on any one-sided retune.

3. **Guard lives in `tests/test_docket_status.sh`, not `test_board_checks.sh` or a new file.**
   Chosen: docket-status's suite, because `ORPHAN_PR_IDLE_SECS` is the derived copy and its comment carries the sync promise.
   Rejected: a new test file — two assertions do not justify a suite entry; rejected: board-checks' suite — the original constant is the upstream, the copy is what drifts.

4. **Guard scope is the idle-secs values only.**
   Chosen: value-equality guard, per the 0250 stub's explicit narrowing at triage.
   Rejected: also asserting the base-handling shape (from #0241's prose) — no cheap non-brittle anchor exists for "shape agreement" short of executing both predicates against shared fixtures, which is a much larger change than the stub scopes; the narrowing was a deliberate triage decision, honored here.

5. **Docs touch: `docket-status.md` update is mandatory; no ADR-0072 `## Update` note.**
   Chosen: `scripts/docket-status.md` MUST be updated — it quotes the fallback command verbatim (~:159-163, `gh pr list --head feat/<slug> --state merged`, no `--repo`), so after Part 1 that quote is stale; update the quoted command and add one sentence that the fallback is repo-scoped. Plus the call-site "SPENDS the resolution" comment in the script.
   Chosen: no dated `## Update` note on ADR-0072, even though the convention provides one for non-reversing context changes and this change does soften the ADR's "nothing fails at test time" cost claim. Real ground: the note is optional context and the skip is reversible; the guard is discoverable from the test side (a reader of the ADR alone gains no pointer to it — accepted cost), and adding the note would pull a `docket-adr` invocation into an otherwise script+test change. A builder who disagrees may add the note via docket-adr; the decision here is only that it is not required.
   Rejected: skipping the `docket-status.md` edit ("only if it states otherwise") — the conditional is decidable now and resolves true.

6. **Guard sensitivity proven in-suite, not manually.**
   Chosen: the sed-mutate-a-copy witness (mechanism step 4 above), matching the house idiom at `test_board_checks.sh:2507`.
   Rejected: a one-off manual mutation check before commit — leaves the guard's ability to redden unverified for every future run, which is the exact drift-blindness this change exists to close.

7. **Part 1 test fixture: a new argv-recording GH stub rather than extending `gh-detect-ok.sh`.**
   Chosen: dedicated stub (per the `gh-orphan-argv.sh` precedent) — the existing detect_merged stubs answer content, not argv, and overloading them couples unrelated asserts.
   Rejected: extending the existing detect-case fixture — cheaper but muddies each fixture's single purpose; the suite's precedent is one stub per witness kind.

## Build notes

- TDD order: argv assertions red first (stub records no `--repo`), then the one-line fix; the guard's equality asserts are green-on-arrival by construction, and its sensitivity is carried by the in-suite mutation witness (mechanism step 4), not a manual check.
- Run tests under `/usr/bin/grep` compatibility discipline: keep the extraction patterns BSD-safe (no bounded-repetition beyond 255, plain anchors only).
