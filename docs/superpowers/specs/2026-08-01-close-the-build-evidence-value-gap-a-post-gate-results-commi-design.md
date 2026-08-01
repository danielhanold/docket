<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0190 — Close the build-evidence value gap: a post-gate results commit always defeats finalize's suite skip](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0190-close-the-build-evidence-value-gap-a-post-gate-results-commi.md)**
<!-- docket:backlink:end -->

# Close the build-evidence value gap: a post-gate results commit always defeats finalize's suite skip — design

**Change:** 0190 · **Date:** 2026-08-01 · **Groomed:** autonomous (docket-auto-groom) · **Extends:** 0170 (merged `done`; the skip stanza it extends is live on `origin/main` at tip `e108568a`, `skills/docket-finalize-change/SKILL.md` step 4). No `depends_on` gate — 0190 builds against merged main.

## Context

Change 0170 ships the suite-once evidence chain: `docket-build`'s gate mints an evidence record
(`command`, `result: green`, `head_sha`, `ran_at`); Step 7 writes it into the PR body; and
`docket-finalize-change`'s `gate: local` post-rebase suite run is skipped only when all three
hold — the rebase was a no-op, the PR body's evidence block is parseable and green, and the
block's `head_sha` equals the branch HEAD being merged.

That third condition is exact SHA equality, which is what makes the skip auditable. But
`docket-implement-next` Step 6.5 commits the `results:` file **on the feature branch** after the
gate already minted the evidence. Any such post-gate commit moves HEAD, `head_sha` no longer
matches, and the skip never fires. Step 7's own prose (as implemented in 0170) documents the
stale `head_sha` on that path as EXPECTED and the suite running. Measured against this repo's
archived history, roughly 73% of changes carry a results file — so the skip is inert on the
majority path.

This is not a safety bug: the predicate fails toward running, which is correct. It is a value gap
that 0170 deliberately left open. This change closes it by answering one design question: **can
the skip safely admit a post-gate docs-only delta, and if so on what proof?**

## Goals

- The skip fires on the common path — a no-op rebase whose only post-gate delta is a suite-invisible
  results file — without weakening the "any doubt runs the suite" posture.
- The trust boundary is explicit and verifiable per-repo, never assumed from path names.
- No new evidence-block schema, no new branch/file writes, no second suite run inside the build loop.

## Non-goals

- Weakening any other condition of the skip predicate (no-op rebase, green parseable block).
- Moving where the evidence lives (the PR body; settled by 0170/ADR-0066).
- Changing the reviewer, rung selection, or build gate (0170's other components).
- Making the skip apply to `gate: ci`, `both`'s CI leg, or `off`.

## Decision — extend the consumer-side predicate with an ancestor + path-allowlist check

Skip the post-rebase local suite run when ALL hold:

1. the rebase was a **no-op** (unchanged from 0170 — the branch was already based on the current
   `origin/<integration_branch>` tip; HEAD unchanged), and
2. the PR body carries a parseable `docket:build-evidence` block whose `result: green`
   (unchanged from 0170), and
3. **the evidence `head_sha` equals the branch HEAD being merged** — the 0170 case — **OR both of:
   (a) `head_sha` is a strict ancestor of the branch HEAD, and (b) every path changed in
   `head_sha..HEAD` lies under the allowlisted prefix(`es`)**.

Any doubt — a missing/malformed block, a non-green result, a `head_sha` that is not an ancestor,
an empty-or-unknown range, **any changed path outside the allowlist** — runs the suite exactly as
0170 ships. The posture still fails toward running; the extension only ever *adds* a skip, it
never adds a run it would not otherwise make.

### Why the consumer side, not the producer re-mint

The stub's "cheaper alternative" — have Step 7 **re-mint** the evidence after the last post-gate
commit — does not close the gap, it relocates and can enlarge the cost:

- Re-minting requires re-running the suite at Step 7 (a block certifies the *state it tested*; a
  moved HEAD is by definition untested). On the common path the re-mint run simply replaces
  finalize's run — net-neutral, zero saving.
- On the moved-base path the re-mint is **worse**: gate (#1) + Step 7 re-mint (#2) + finalize's
  post-rebase run (#3, base moved so the skip cannot fire) = three runs, versus two today.
- It also injects a second full-suite run into the implementation loop, lengthening every build
  that carries a results file.

The only way to *skip without re-testing* — which is the whole value of the skip — is to prove the
untested delta cannot affect the suite. That proof is a consumer-side, git-derived check, which is
why the extension lives in finalize's predicate and not in Step 7.

### Why not a producer-issued attestation

An alternative keeps the evidence schema grown with a `post_gate_delta: docs-only` attestation
written by Step 7. Rejected: the finalize-side git re-derivation is strictly stronger, because
finalize is the adversarial consumer — it re-computes the delta from git state, so a lying or
mistaken producer cannot make a suite-relevant delta look docs-only. Moving the judgment to the
producer would let the attacker (or the bug) write the proof. No schema change, no attestation
field.

### The allowlist — what it is, and how a path gets in

The allowlist is a closed, prefix-matched set of paths, **derived from config, not hard-coded**:
the repo's `<results_dir>/` (trailing slash, so `docs/results/` matches `docs/results/x.md` but
not a sibling `docs/results2/`). That is exactly the tree Step 6.5 commits post-gate, so the
allowlist is the minimal set the standard delta can contain, never "any docs path."

`docs/superpowers/plans/` is **not** admitted a priori: plans land on the feature branch at
plan time (pre-gate), so there is no standard post-gate plan commit; admitting a path with no
demonstrated delta only enlarges the smuggling surface. A future demonstrated need extends the
allowlist deliberately, and the fail-toward-running posture makes the narrower list safe (an
unexpected post-gate plan edit simply fails the skip and runs, by design).

### The allowlist is justified per-repo, never assumed — verified, and guarded

The stub's warning is load-bearing: "whether a docs path can ever affect a suite that greps
documentation — in this repo it demonstrably can." This repo's guards assert over `README.md`,
`skills/**/*.md`, `agents/**/*.md` — **none of which are in the allowlist**. What makes the
allowlist safe is not the word "docs" but a verified property: **no executable suite component
reads the allowlisted tree as a content source.**

Verification for this repo (done at design time, re-run at build reconcile):

- `tests/test_docket_build.sh` explicitly exempts `docs/results`, `docs/superpowers`,
  `docs/changes/archive`, `docs/adrs` from its content greps ("Exempt by design — those record…").
- `tests/test_readme_finalize_docs.sh` excludes `docs/superpowers/**`, `docs/changes/**`,
  `docs/results/**` from its doc-content `rg`.
- No other `tests/test_*.sh`, `scripts/*.sh`, or suite-reachable skill file reads `docs/results/`
  as a content source; references elsewhere (e.g. `board-checks.sh`'s `broken-plan-results`,
  `test_results_artifact.sh`) are metadata/derived-view reads or fixtures, not the test suite.
- The change file, specs, and ADRs on the metadata branch are invisible to the hermetic suite by
  construction (the learnings "metadata-branch-invisible-to-suite" rule), so `docs/changes/` is
  not a concern here at all.

**Safety does not rot silently — ship a live guard.** The build must add one guard test
(`tests/test_skip_allowlist_invisibility.sh`) enforcing the verified property. It scans the
**live committed tree** (`git grep` over HEAD — this restricts the scan to committed *tracked
blobs*, the tree the suite actually runs against; it does NOT hide the literal: `docs/results/`
still occurs ~38× in committed test bodies and config references, so those are resolved by
classification, not by being out of scope) and classifies every occurrence of the
`<results_dir>` literal across `tests/`, `scripts/`, and suite-reachable skill files into
**consumed-as-content** (a hazard — a read/grep/cat of the tree) vs **benign** (fixture paths,
config-key references, comments, the suite's own exemption constructs). The guard fails on any
new or unclassified occurrence that is consumed-as-content, and — per the marker-scoped-guard
population-floor rule — pins a floor: the *full* benign corpus (~38 committed occurrences: ~34
in `tests/` + config-key refs in `scripts`/`skills`) must classify clean or the guard reddens on
arrival — floor ≠ corpus, the cited "~10" was only a non-vacuity subset. Its positive claim —
that the suite's genuine protective mechanism survives — is asserted by **machine-recognized
shape, never the bare path**: key on the exclusion **magic token** (`test_docket_build.sh`'s
`:!docs/results` path-exclusion construct at its armed-probe and `test_readme_finalize_docs.sh`'s
`--glob "!docs/results/**"` escape) — a bare-path assert stays green because the bare literal
also sits in that file's *comment* and its armed probe, so the real exclusion can be deleted
without the bare path changing. Honest limits, stated so the builder claims no more than it
mutates: (a) the *benign* classification is necessarily curated (a pure whole-tree grep can
neither name a fixture benign nor exclude the guard's own body), while the *hazard* predicate
derives from the actual consuming code; (b) a co-located-verb detector cannot catch an *indirect*
read (`r="$ROOT/docs/results"; grep x "$r"`) — mutation (b) proves only the direct form reddens;
and (c) the guard test's own corpus classification includes this change's SKILL prose, which must
use the `<results_dir>` placeholder to avoid self-injection. The mutation tests: (a) deleting the
`:!docs/results` exclusion token in `test_docket_build.sh` reddens; (b) adding a new
content-read of `<results_dir>/` reddens.

**Fail-safe at build time.** If the reconcile-time verification cannot establish suite-invisibility
for the configured `<results_dir>`, the extension **degrades off** — finalize behaves exactly as
0170's equality-only predicate. The enforcement is mechanical, not a prose decision: the shipped
guard test goes red, which fails 0190's own whole-suite build gate, so the extension cannot land
with an unverifiable allowlist; that outcome is recorded in the results file, not papered over.

### Pinning the extension on the tracked delta

The check operates on `git diff --name-only -z <head_sha>..HEAD` (null-delimited, so no path with
spaces/newlines can break the prefix test; two-dot is tree-vs-tree, which is by construction the
post-gate delta on a linear, rebased branch). It tests **tracked paths only**, never filesystem
traversal, so a symlink planted under the allowlisted tree cannot smuggle a path that a content
guard would iterate — and in any repo the whole-tree verification below is the real protection. A
commit range that is non-empty on the graph but empty in the diff (an empty post-gate commit) is
treated as doubt and runs — over-conservative, never a hole. The change set is authoritative; the
surviving safety props are (a) the closed, prefix-matched allowlist, (b) per-repo verification,
and (c) the live guard.

### Logging

Extend the existing loud one-line skip log to name the matched permit: the exact-SHA match or the
docs-only ancestor match with the byte-identical delta summary (`head_sha → HEAD, N files, all
under <results_dir>/`), so the auditable decision records *why* it skipped.

## Safety analysis — smuggling-vector enumeration (per the stub's ask)

- **Code smuggling via an allowlisted commit:** a commit in `head_sha..HEAD` that touches a real
  code path (e.g. `scripts/`, `skills/`, `tests/`) — caught: any non-allowlisted path fails the
  skip, suite runs.
- **Content smuggling inside an allowlisted file:** a results file containing anything — harmless
  by definition, because the verified property is that no suite component reads that tree.
- **The general "docs greps docs" hazard:** a future or hypothetical guard reading
  `docs/results/` — caught by the live `test_skip_allowlist_invisibility.sh` guard introduced with
  this change, which is the mutation-tested backstop (mirrors the repo's existing exemption lines).
- **Path-shape attacks:** `docs/results2/`, `DOCS/RESULTS/` (git is case-sensitive), a bare
  `docs` — none match the trailing-slash prefix `docs/results/`; prefix match on tracked paths,
  exact semantics.
- **Range ambiguity:** if `head_sha` is neither equal nor an ancestor (a force-push rewrite), the
  "ancestor" predicate fails and the suite runs — the same fail-toward-running answer a SHA
  mismatch already gives today.
- **A lying producer:** irrelevant — finalize re-derives the delta from git state at skip time;
  the evidence block contributes only `head_sha` and `result: green`, and the range is computed
  fresh, so a wrong attestation cannot exist.

## Ripple surfaces

| Surface | Edit |
|---|---|
| `skills/docket-finalize-change/SKILL.md` | step 4's conditional-skip stanza: add the ancestor + allowlist disjunct, the loud log extension, and the degrade-off rule |
| `skills/docket-implement-next/SKILL.md` | Step 7's build-evidence prose: replace "finalize's SHA-equality condition simply fails, and the suite runs" with the extended-predicate outcome |
| `tests/test_skip_allowlist_invisibility.sh` | **new** — the live suite-invisibility guard (git grep over HEAD; hazard-vs-benign classification with a population floor; mutation-tested both ways) |
| `README.md` | the evidence-chain section: document the docs-only skip and its per-repo verification rule, **and retract/qualify the existing "the skip is the clean-path optimization, not the majority path" caveat** (README ~line 750) — 0190's thesis inverts it, so leaving the clause untouched makes the section self-contradictory |
| `docs/adrs/0066-docket-owns-the-review-role-suite-runs-in-the-build-gate.md` | a dated `## Update` note extending the skip decision with the ancestor+allowlist rule, which **also dates-and-closes the deferral sentence in 0066's Consequences** ("docs-only ancestor exemption… as separate design work" — now done, so the accepted ADR must not carry a stale open-deferral alongside the new rule) — 0066 is Accepted; the update-note form, never an edit to the Decision — dispatched via the `docket-adr` subagent at build time |
| `tests/test_docket_review.sh` | the stanza's guardian file: its existing sentinels (no-op rebase / `result: green` / `head_sha` / fails-toward-running / local-only) all survive the extension as substrings, but **none binds the new ancestor+allowlist limb** — add one shape assert for the new disjunct, so an ever-widening allowlist cannot regress unguarded |
| `docs/changes/active/0190-….md` | this change's `adrs:` gains `[66]` at spec-emission, so the ADR note is delivered atomically with the producing change (per the adr-update-delivery learning — never a standalone push) — the Field-write rule re-renders the `## Artifacts` block in the same commit |

**Budget — expect an in-diff raise.** The caps are `test_skill_size_budgets.sh` 176/178: finalize
193/4350, implement-next 147/3950 — but those are caps, and the live actuals leave thin headroom
(finalize ~191 ln/4302 w, implement-next ~143 ln/3923 w). A realistic step-4 addition (the
disjunct + log extension + degrade-off rule) is likely to redden the suite, so the change ships
its budget **raises in the same diff** (the file's own stated house rule) and re-reads the live
caps at build time. The guard test's own prose should use the `<results_dir>` placeholder, not a
literal `docs/results/`, or the guard (which scans suite-reachable skill files) would match this
change's own finalize/implement-next prose — seed the curated benign set in the same diff.

## Reconcile obligations at build time

- **0170 is already merged** (`done`; terminal-publish tip `e108568a` on `origin/main`). There is
  no feature-branch dependency; reconcile against the **live post-0170 stanza** in
  `skills/docket-finalize-change/SKILL.md` and re-verify the wording this change edits against
  `origin/main`'s current tip at build time (the base may move again). The stanza's guardian is
  `tests/test_docket_review.sh` (not `test_finalize_gate.sh`, which has no skip-anchor asserts);
  its existing sentinels survive the extension, and this change adds one shape assert for the new
  disjunct (ripple table).
- Re-run the suite-invisibility verification (whole-tree scan for **reads** of `<results_dir>/`
  as content) against the merged tree, and record it in the results file; on failure, the shipped
  guard test reddens and 0190's own build gate enforces the degrade-off rule.
- Prose discipline: cross-references in the edited SKILL files anchor on symbols or quoted
  clauses, never line numbers (`tests/test_comment_anchor_style.sh`).

## Assumptions (deferred human audit trail)

1. **Fix side — consumer (the predicate) beats producer (re-mint).** Chosen: extend finalize's
   skip predicate with an ancestor + allowlist disjunct. Rejected: Step 7 re-minting the evidence
   (net-neutral on the common path and net-negative when the base moved — gate + re-mint + finalize
   = three runs; and it adds a second full-suite run to the build loop). The deep reason: re-testing
   to earn a skip can never reduce the run count; only a *proof of suite-invisibility* can, and that
   proof is a git-side read, which lives naturally in the consumer.
2. **No attestation field.** Rejected: Step 7 writing a `post_gate_delta: docs-only` attestation.
   The finalize-side re-derivation cannot be fooled by a lying/buggy producer; an attestation moves
   the trust judgment to the party being trusted. Simpler schema, stricter safety.
3. **Allowlist = `<results_dir>/` alone, prefix-matched, config-derived.** Chosen over
   `<results_dir>/` + `docs/superpowers/plans/`: there is no standard post-gate plan commit (plans
   land pre-gate), so plans would be dead surface; the minimal set is exactly what Step 6.5 writes.
   Omitting it is safe (a post-gate plan edit fails the skip and runs). The trailing-slash prefix
   blocks sibling-path collisions.
4. **Per-repo verification is load-bearing and guarded.** Chosen: the allowlist is justified by a
   whole-tree verification that no suite component **reads** `<results_dir>/` as content (verified
   at design time: this repo's suite is hermetic to it by construction — `test_docket_build.sh`
   exempts `docs/results`/`docs/superpowers` from its content greps, `test_readme_finalize_docs.sh`
   excludes both from its doc `rg`, and every other mention is a fixture path, a config-key
   reference, or a comment), plus a **live guard test** introduced by this change, plus a
   build-time degrade-off rule enforced by the red guard (the build gate — not prose — is the
   enforcement point). Rejected: a fixed, hard-coded allowlist ("docs is safe") — the stub's own
   repo demonstrates docs paths can be suite-relevant; the name is not the property. The guard
   cannot be a pure whole-tree grep (a naive grep matches its own body and the full committed
   benign corpus); it classifies occurrences hazard-vs-benign against the live tree with a
   population floor, keys its positive claim on the machine-recognized `:!…`/`--glob "!…"`
   exclusion tokens (never the bare path — the bare literal also sits in comments and armed
   probes), and is honest that the benign classification is curated while the hazard predicate
   derives from the consuming code.
5. **No dependency on 0170 — it is already merged.** 0170 is `done` and archived; its skip stanza
   is live on `origin/main` (tip `e108568a`, finalize SKILL.md step 4). The change file's
   `depends_on` stays `[]`; 0190 builds against merged main. (Design-time verification that 0170
   was unmerged was against a stale local snapshot — the moving-base lesson: re-derive from fresh
   `origin`, always.)
6. **ADR record.** The build extends Accepted ADR-0066 with a dated `## Update` note (never an
   edit to its Decision), via the `docket-adr` subagent — the skip's trust boundary is now "state
   provably identical" *or* "delta provably suite-invisible," a real extension of the 0170
   decision that ADR-0066's own Consequences defer to separate design work; the note **also
   dates-and-closes that deferral sentence** so the accepted ADR carries no stale open-deferral.
   The producing change's `adrs:` gains `[66]` at spec-emission so the update note is delivered
   atomically with the change (adr-update-delivery rule — never a standalone push).
7. **Logging.** The skip stays loud; the log line now names the matched permit (exact-SHA or
   docs-only ancestor). The audit trail keeps working either way.
8. **`gate: ci` / `both`-CI / `off` untouched.** The extension is scoped to the local leg of
   `local`/`both`, exactly as 0170 scoped the skip itself.

## Open questions

None — the design commits every decision; the one open item at build time (whether the merged-repo
verification holds) has a defined fail-safe answer (degrade off), not a new decision.
