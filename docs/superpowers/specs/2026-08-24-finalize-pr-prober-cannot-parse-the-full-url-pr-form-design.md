<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0344 — Finalize PR prober cannot parse the full-URL pr: form](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0344-finalize-pr-prober-cannot-parse-the-full-url-pr-form.md)**
<!-- docket:backlink:end -->

# Finalize PR prober cannot parse the full-URL `pr:` form — design (change 0344)

## Problem

`docket context finalize` reports `pr-unknown` for any change whose `pr:` frontmatter is written in
the full-URL form (`https://github.com/owner/repo/pull/N`), making that change un-finalizable
through the binary: the merge-gate selector refuses before GitHub is ever contacted. This directly
blocks finalizing change 0341 (PR #235) and every other change written with a full-URL `pr:`.

## Root cause (code-proven)

Two sibling functions in `internal/app/finalize_context.go` are the **only** two things in the tree
that resolve a PR number from the `pr:` field, and **both parse only the `owner/repo#N` shorthand**:

- `parsePRNumber(ref string) (int, bool)` — the number extractor. A whole-repo grep
  (`grep -rn parsePRNumber internal --include=*.go`) shows **five** call sites, all of which pass a
  change's `pr:` value and so all of which are broken today for a URL-form `pr:`:
  `finalize_context.go:609` (`githubFinalizeProber.ProbePR`), `finalize_cleanup.go:233`,
  `finalize_closeout.go:286`, `finalize_closeout.go:604`, and `finalize_merge.go:425`.
- `prNumberToken(ref string) string` — **one** call site (`finalize_context.go:514`), the error
  path of `probeFinalizeFacts`, carrying the canonical number into the unknown-facts record.

Both key on `strings.LastIndex(ref, "#")`. A canonical PR URL contains no `#`, so both return
"no number". For `probeFinalizeFacts` the failure folds into unknown facts and the domain selector
reads `pr-unknown`; the other four `parsePRNumber` sites (cleanup, closeout ×2, merge) likewise fail
to resolve a URL-form PR and mis-handle it — so the fix must widen `parsePRNumber` itself, not just
the probe path.

Verified against current `main` (caller sets derived by grep, not recalled): these two functions are
the whole population of `pr:`-number parsers; a full URL has no `#`; and the two already **diverge
subtly** — `parsePRNumber` requires `n > 0`, while `prNumberToken` runs only `strconv.Atoi` and would
accept `0` or a negative. That latent divergence is exactly the class the fix below closes.

This is a **pre-existing** bug introduced by change 0316 (which added both functions); it was merely
first hit live while finalizing 0341, whose `pr:` is the board-required URL form. It is **not**
introduced by 0341 and does **not** depend on it.

## The bind this sits in

The board requires `pr:` to be the **full URL** (the shorthand renders as plain text / mangles on
the board), while the finalize prober only understands the **shorthand**. The two representations
the codebase requires are mutually exclusive across subsystems. The fix reconciles them by teaching
the prober to accept both — the board keeps its required URL form, finalize stops refusing it.

## What changes

1. **One shared extractor.** Introduce a single unexported `parsePRRef(ref string) (int, bool)` in
   `finalize_context.go` that accepts **both** forms and returns the positive PR number:
   - If `ref` contains the `/pull/` path segment, read the integer immediately after it (URL form).
   - Otherwise, read the integer after the last `#` (existing shorthand form).
   - In both cases require a **positive** integer; return `(0, false)` otherwise.
2. **Both functions delegate to it.** `parsePRNumber` becomes a thin pass-through to `parsePRRef`;
   `prNumberToken` returns `strconv.Itoa(n)` when `parsePRRef` succeeds and `""` otherwise. Neither
   keeps its own parsing logic, so they can never again diverge on which forms they accept — the
   single-source-of-truth move that `validator-must-match-the-reader-it-feeds` and
   `fix-reintroduces-its-own-defect-class` both prescribe for twin parsers. Because the widening
   lands in `parsePRNumber` itself, **all five** of its call sites (probe, cleanup, closeout ×2,
   merge) accept the URL form for free — the fix is not scoped to the probe path alone.
3. **URL-shape parse rule.** After `/pull/`, take the run of characters up to the next `/`, `?`,
   `#`, or end-of-string as the number segment, and require it to be an all-digit positive integer.
   This accepts the canonical `.../pull/N`, plus a trailing slash, a `?query`, a `#fragment`, and a
   deeper sub-page (`.../pull/N/files`) — every one of which unambiguously names PR `N`. It rejects
   a non-numeric segment (`.../pull/abc`) and a missing number.
4. **Tests.** Unit-test `parsePRRef` (via the two exported-within-package callers, matching the
   existing test style) across: canonical URL, trailing slash, `?query`, `#fragment`, `/files`
   sub-page, the `owner/repo#N` shorthand, a non-positive `#0`/`#-1`, non-numeric garbage, and empty
   — asserting the shorthand still parses. Add one selector-level test proving a URL-form `pr:` no
   longer yields `pr-unknown` (extend `finalize_context_test.go`'s `prRefFor` helper with a
   URL-form variant). Because `parsePRNumber` also feeds cleanup, closeout, and merge, the plan
   should confirm (grep-derived, not recalled) that those paths inherit the widened parse and add
   coverage where a URL-form `pr:` reaching them is observable.

## Out of scope

- Changing the required `pr:` representation (it stays a full URL for the board).
- Any board-rendering work (separate concern; 0343's family).
- Routing through 0341's `githubWebURL` / `linkContextOf` helpers — see Assumption 4.
- Broader URL-parsing refactors — this stays a targeted, local parser fix.

## Assumptions

Each decision below was defaulted autonomously (no human in the loop). The chosen default, the
alternatives weighed, and the reason are recorded for later audit.

1. **Accepted URL shapes — liberal in what we accept.**
   *Chosen:* accept the canonical `.../pull/N` and tolerate a trailing slash, `?query`, `#fragment`,
   and a deeper sub-page (`.../pull/N/files`), because in every one of those the number after
   `/pull/` is unambiguous.
   *Rejected:* strict canonical-only (reject any suffix) — it re-introduces the exact failure mode
   this change exists to kill (an un-finalizable change) for benign variant URLs, with no upside,
   since the number is never ambiguous.
   *Why safe:* a permissive-but-unambiguous parse can only ever resolve to the one correct PR number;
   it cannot mis-target a different PR. Directly supported by `display-format-is-not-a-parse-format`
   (the displayed/required form is the form typed back, so the parser must accept it).

2. **Parse direction / disambiguation order — check `/pull/` before `#`.**
   *Chosen:* if `ref` contains `/pull/`, parse the URL form; otherwise fall back to the `#N`
   shorthand.
   *Rejected:* keying on `#` first — a URL with a `#fragment` (`.../pull/235#discussion`) contains a
   `#`, so a `#`-first reader would mis-read the fragment as the number. Ordering `/pull/` first is
   the only correct precedence.
   *Why safe:* the shorthand never contains `/pull/`, and a URL's number always follows `/pull/`, so
   the two forms are cleanly separable by this test.

3. **One shared extractor rather than two patched parsers.**
   *Chosen:* collapse both functions onto a single `parsePRRef`, and require a positive integer in
   the shared path.
   *Rejected:* patch each function independently to also handle URLs — rejected because the two
   already diverge (`prNumberToken` accepted non-positive) and independent patches would let them
   diverge again on the new URL forms too.
   *Caller-variance check (`consolidation-flattens-caller-variance`), over the grep-derived caller
   set:* `prNumberToken` has one caller (the error branch of `probeFinalizeFacts`) and `parsePRNumber`
   has five (probe, cleanup, closeout ×2, merge). The only real variance between the twins is
   `prNumberToken`'s historical acceptance of a non-positive integer, and its sole caller carries that
   value into unknown facts, where a `0`/negative "number" is meaningless; tightening it to
   positive-only is a strict improvement, not a behavior any caller relied on. The five `parsePRNumber`
   callers already require positive-only and only *gain* the URL form. Safe to flatten.

4. **Do not reuse change 0341's URL helpers.**
   *Chosen:* write a small local extractor in `finalize_context.go`.
   *Rejected:* routing through 0341's `githubWebURL` / `linkContextOf` (`internal/app/link_context.go`).
   *Why:* those helpers are on 0341's **unmerged** feature branch (0341 is `implemented`, not
   `done`) and 0344 carries **no** `depends_on: [341]` — building on them would manufacture a
   dependency on unmerged work and stall this fix behind 0341's merge. They also parse the **opposite
   direction** (remote → web URL), not URL → number, so there is no real code to share. A local
   extractor keeps 0344 independent and mergeable on its own.

5. **Dependency state.** `depends_on: []` is correct and retained. The bug and its fix are entirely
   within `finalize_context.go`; nothing in 0344 requires 0341, 0343, or any other change to land
   first. (0341 is the change whose finalize *surfaced* the bug, recorded as `discovered_from: [341]`
   — informational, not a readiness gate.)

## Open questions

None remain. The three open questions the stub raised (which URL shapes to accept; whether to share
one internal extractor; whether to reuse 0341's helper) are resolved in Assumptions 1, 3, and 4.
