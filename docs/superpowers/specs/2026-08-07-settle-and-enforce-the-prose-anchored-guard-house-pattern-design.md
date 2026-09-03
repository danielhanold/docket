<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0253 — Settle and enforce the prose-anchored guard house pattern](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-03-0253-settle-and-enforce-the-prose-anchored-guard-house-pattern.md)**
<!-- docket:backlink:end -->

# Design: settle and enforce the prose-anchored guard house pattern (0253)

## Problem

Two defect classes in the same idiom — prose-anchored `grep -qE` guards over skill bodies:

1. **Reflow fragility (was #0171).** Line-scoped `[^.]{0,N}` guards cannot cross a newline, so a
   cosmetic rewrap of the guarded sentence false-reddens them. The de-facto cure already exists —
   `flatten(){ tr -s '[:space:]' ' '; }` over the haystack — but is independently defined in
   `tests/test_docket_review.sh:193`, `tests/test_gate_execution_posture.sh:18`, and
   `tests/test_loop_continuation.sh:91`, and whole files (`test_docket_build.sh` densest) never
   adopted it.
2. **Stacked-gap catastrophic backtracking (was #0233).** Two *sequential* gap atoms — negated
   class (`[^x]{0,n}`) or dot (`.{0,n}`) — in one ERE branch backtrack for minutes under ugrep on
   non-matching input — the guard hangs precisely when it is catching a mutation. The stub's
   "four files" list is corrected by inspection: `test_dispatch_capability.sh`'s two-gap lines
   (:123, :156) are *mirrored alternations* (one gap per branch — safe, see rule 2), while the
   genuinely dense sequential populations are `test_finalize_disposition.sh` (~18 dot-gap stacks,
   e.g. :36, :49, :183) and `test_docket_review.sh` (~12+, including three-gap branches at :365,
   :777), alongside `test_docket_build.sh` (:110, :299-305, :344, :348) and
   `test_gate_execution_posture.sh` (:70, :120, :329). Those four files are only the *densest*:
   the clean-population bar covers all of `tests/*.sh` — ~50+ stacked pattern-strings across
   roughly a dozen files under the full atom set, unbounded stacks included (e.g.
   `test_finalize_gate.sh:121,149` stacks two-to-three unbounded `[^.]*` gaps over a large
   haystack; `test_typed_changes_docs.sh:165` stacks five `.*`). It grows with every
   proximity-shaped sentinel. Budget for the real spread; mass exemption tokens to dodge the
   scale would gut the guard and are not acceptable.

## What this change does

### 1. Hoist the house helper: `tests/lib/prose_guard.sh`

- Sourced (never executed); joins the existing `tests/lib/sync_agents_common.sh` precedent and the
  `tests/lib/` home change 0252 is standing up — coordinate at build time, do not wait on it.
- Exports exactly `flatten(){ tr -s '[:space:]' ' '; }` — the review file's canonical form; `-s`
  squeeze is load-bearing (wrapped list-item continuation indents), keep its comment.
- Its header comment states the **house pattern rule** (the single write-down 0171 asked for):
  1. Flatten the haystack (slice with awk over the *newline-bearing* source first — an awk range
     over a flattened file has one line; extractors stay unflattened, match targets get flattened).
  2. At most ONE bounded gap per alternation branch. Mirrored alternations
     (`A[^.]{0,n}B|B[^.]{0,m}A`) are safe — branches do not multiply.
  3. Every repetition bound ≤ 255 — a bound EQUAL to 255 is legal, above it is not
     (`test_grep_portability.sh`'s `MAX_BOUND=255` is the authority; the stub's "< 255" wording
     is corrected to match the enforcing guard).
  4. Scope gaps with a negated class; widen the exclusion (`[^.|]`) where a flattened table row
     could bridge two cells/rows and satisfy a deleted-rule pattern (the documented
     `test_docket_review.sh` bridging failures).
  5. Deliberate exceptions — line-anchored structure guards (`^- \*\*…` bullet-reappearance,
     `^\| C …` table-row/cell asserts) and deliberately unflattened table slices — are legitimate
     and stay; each carries a one-line comment saying why flattening would erase its signal (the
     existing files already model this).

### 2. Convert the line-scoped prose guards

- **Re-derive the site list at build time** (never from the stub's counts — 0172's rule): sweep
  `tests/*.sh` for prose-matching asserts whose haystack is an unflattened multi-line slice and
  whose pattern carries a bounded gap or a phrase that a rewrap can split. The stub's tally
  (24 `test_docket_build.sh`, 18 `test_docket_review.sh`, 20 `test_gate_execution_posture.sh`,
  1 `test_finalize_disposition.sh`) is a floor-check, not the list.
- Conversion = flatten the haystack via the sourced helper, keep the pattern's semantics; where a
  file already defines `flatten()`, replace the local definition with the `source`.
- **Mutation proof per converted assert, both directions** (0171's completion bar): deleting the
  guarded prose reddens; a width-64 rewrap of the guarded prose stays green (the re-wrap positive
  control already recorded in `test_docket_review.sh`).
- `flatten_yaml()` (`test_docket_example_yml.sh:1247`) is NOT hoisted: it is a YAML
  key-path flattener, not a whitespace normalizer — different contract. It gets a cross-reference
  comment to the house helper so nobody "unifies" them blind.
- Sequential stacked-gap patterns found during conversion are rewritten to a single gap using this
  menu, mutation-proved after rewrite: (a) split one three-term proximity assert into two
  single-gap asserts; (b) fold the middle literal into one end (`A[^.]{0,n}C` when B's presence is
  separately asserted); (c) keep mirrored-alternation shape with one gap per branch. The Problem
  section's corrected site survey (~50+ sequential pattern-strings across ~a dozen files, dot-gap
  and unbounded stacks included) sets the budget — again re-derived at build time, not copied. Benign-looking
  small-bound stacks (e.g. `at most[^.|]{0,20}three[^.|]{0,20}suite runs`) are rewritten too: the
  ban is flat because hazard scales silently with bound and haystack.

### 3. Enforce: the stacked-gap leg in `tests/test_grep_portability.sh`

- A new static source-scan leg in the existing file (per the stub), reusing its walk narrowed to
  `tests/*.sh`, its `ok`/`nok` reporting, and its one-scan-function discipline.
- **Hazard definition (what reddens):** two or more gap atoms — negated-class (`[^…]{m,n}`,
  `[^…]*`, `[^…]+`) **or dot-gap (`.{m,n}`, `.*`, `.+`)** — in one pattern with **no top-level
  `|` between them**. Dot-gaps are in the atom set because the densest stacked file
  (`test_finalize_disposition.sh`) is built on them and `.` backtracks at least as badly; a lone
  `.*`/`.{0,n}` stays legal exactly as A5's single-gap logic says.
- **Top-level `|` is computed, not grepped:** after replacing every bracket expression with a
  placeholder token (so `|` inside `[^.|]` cannot masquerade as alternation), the scanner tracks
  parenthesis depth (respecting `\(`, `\|` escapes) — a `|` inside a group like `(->|→|to)` or
  `(e|ation)` does NOT separate gaps. This is load-bearing: nearly every real sequential hazard
  (`test_docket_build.sh:299-305`, `test_gate_execution_posture.sh:70,120`) has a grouped `|`
  between its gaps, and a naive no-`|`-between check would pass them all.
- **Per pattern-string, not per source line:** compound lines (`grep … && grep …`, e.g.
  `test_docket_review.sh:746,770,777,783`) carry multiple independent patterns on one line; the
  scanner extracts each quoted pattern argument and evaluates it alone, so gaps in two different
  patterns never read as one stack. Builder notes from adversarial review: bracket tokenization
  must survive a POSIX class nested in a negated class (`[^[:alnum:]]`,
  `test_gate_execution_posture.sh:120` — a naive `[^]]*`-terminated stripper mangles it), and
  eval-template patterns with escaped quotes (`test_dispatch_capability.sh:123`) evade a naive
  `"[^"]*"` pairer — today's instances are mirrored-safe, so a miss is green-not-red, but the
  shape exists.
- Scans only lines that build grep patterns (assert strings and pattern variables), skips
  comment-only lines; a rare false positive takes a `# stacked-gap-ok: <reason>` token on the site
  line or the line above — every exempted site is printed (visible-skip house rule, 0190), never
  silently passed.
- The narrowed `tests/*.sh` walk gets its own population floor (the host file's `MIN_FILES=100`
  covers only the repo-wide walk).
- Self-handling: `test_grep_portability.sh` assembles its own hazard literals at runtime already;
  the new leg's own detection patterns follow the same discipline so the file stays
  self-membered and clean — no self-exclusion carve-out.
- **Mutation-tested at build time** (a guard is code): temp-fixture controls routed through the
  same scan function — reddening: (1) a plain sequential stack `A[^.]{0,n}B[^.]{0,m}C`, (2) a
  sequential stack with a grouped `|` between the gaps (`A[^.]{0,n}(x|y)[^.]{0,m}C` — the shape
  that defeats naive detection), (3) a dot-gap stack; staying green: (4) a mirrored-alternation
  one-gap-per-branch pattern, (5) a single-gap pattern.
- **Hang-vs-fail proof:** the demonstration that the banned shape hangs rather than fails runs
  once at build verification, wrapped in a watchdog (`timeout`/background-kill), and is recorded
  in the change's results — it is NOT committed as a perpetual runtime assert. The committed guard
  stays a static source scan, matching the file's own stated philosophy (runtime probes of local
  grep behavior are platform-dependent false signals) and burning no suite budget.

## Sequencing and batching

Single change. Order: (1) `tests/lib/prose_guard.sh` + rule comment; (2) per-file conversion
commits, each file's tests green at its commit; (3) the portability-guard leg last, landing only
when the population is clean, mutation controls included in the same commit. Full-suite gate at the
end (backgrounded — the suite sits at the 600s ceiling).

## Out of scope

- The SIGPIPE producer-pipe sweep (#0172) — same files, separate change; textual collision only.
- Guards over non-prose (code-shaped) anchors; `flatten_yaml`'s own semantics.
- Loosening any guard's bite — every conversion is mutation-proved in both directions.

## Assumptions

- **A1 — helper home is a sourced `tests/lib/prose_guard.sh`, hoisting the three identical
  `flatten()` copies; `flatten_yaml` stays local.** Chosen: `tests/lib/` is the established shared
  home (`sync_agents_common.sh` precedent; 0252 is standardizing it) and the three copies are
  byte-identical. `flatten_yaml` is a different contract (YAML key-path extraction) — merging it
  would change what its asserts test; it gets a cross-reference comment only. The stub's "4 flatten
  copies" is thus satisfied as 3 replaced + 1 documented-distinct. Rejected: hoisting all four into
  one polymorphic helper (semantics change); leaving copies with a comment (the triplication IS the
  defect).
- **A2 — the house rule permits deliberate line-scoped guards.** The stub says "convert the 63",
  but the same files contain guards that are line-anchored *on purpose* (bullet-reappearance
  detectors, unflattened table row/cell asserts, whose comments record that flattening erases their
  signal or bridges cells). Blanket conversion would weaken them — the exact opposite of the
  completion bar. Chosen: convert reflow-fragile prose guards; structure-anchored guards stay, each
  with its why-comment; the rule text names both cases. Rejected: convert everything (weakens
  bite); enumerate exceptions in the guard (ADR-0050 — no allowlists; exceptions live as
  site comments).
- **A3 — the site list is re-derived at build time; the stub's 63/4-file tally is a floor-check.**
  An independent recount already diverges (e.g. `test_docket_review.sh` has 45 bounded-gap patterns
  but most already read flattened haystacks; `test_finalize_disposition.sh` shows 2 not 1), and
  0172 set the precedent that stub examples never define the population. Rejected: treating 63 as
  the checklist (stale the day the base moves — learnings `moving-base`).
- **A4 — stacked = sequential within one alternation branch; mirrored alternations are safe and
  stay; top-level `|` requires paren-depth tracking, not bracket normalization alone.**
  Backtracking multiplies only when gaps compose in sequence; `A[gap]B|B[gap]A` tries each branch
  independently and is the house's established two-order idiom. Detection needs two normalizations:
  bracket expressions become placeholder tokens (so `[^.|]` doesn't read as alternation) AND `|`
  separates gaps only at parenthesis depth zero (respecting escapes) — because nearly every real
  sequential hazard carries a grouped `|` such as `(->|→|to)` or `(e|ation)` *between* its gaps,
  and a check keyed on any bare `|` between gaps would go blind to almost the entire hazard
  population while its plain-stack mutation control stayed green. The mutation controls therefore
  include a grouped-`|` sequential fixture. Note the sequential population is the *majority* of
  two-gap sites (~40+), mirrored pairs the minority. Rejected: ban any two gaps per pattern
  (false-reddens the safe mirrored idiom); bracket-normalization-only detection (blind, per
  above); runtime hang-detection instead of static analysis (platform-dependent, budget-hostile).
- **A5 — unbounded single gaps (`[^.]*`) remain legal.** `test_docket_review.sh:534-540` uses
  `never mint[^.]*…` deliberately — the match lies past the 255 bound ceiling and the negated class
  already scopes the gap to one sentence; a single gap does not stack. They count as gap atoms for
  the stacking check but are not themselves violations. Rejected: forcing bounds everywhere
  (re-introduces the exact BSD-ceiling defect the file documents).
- **A6 — the guard leg is static; the hang proof is a build-time watchdog demonstration, not a
  committed runtime assert.** The stub asks for "a `timeout`-shaped assert"; honored at build
  verification (recorded in results) rather than in the suite: `timeout` is Homebrew coreutils on
  this platform (not base macOS), a perpetual hang-probe costs seconds-to-minutes against
  `runtime-budgets.tsv` for zero new information, and `test_grep_portability.sh`'s own header
  argues static source truth over runtime tool probes. The committed mutation controls (seeded
  fixture reddens) prove the guard bites without ever risking a hang. Rejected: committed
  timeout-wrapped hang assert (portability + budget); no proof at all (violates "a guard is
  code: mutation-test it").
- **A7 — guard placement is a leg inside `test_grep_portability.sh`, scoped to `tests/*.sh`.** Per
  the stub; reuses walk, controls discipline, and the existing runtime-budget row rather than a new
  file. Prose-grep patterns essentially all live in `tests/` today; widening the scope later is a
  one-line walk change. Rejected: new `tests/test_stacked_gaps.sh` (duplicate walk + new budget row
  for the same concern); repo-wide scope now (skills/docs quote patterns verbatim; the sibling
  bound-guard's docs/ reasoning applies).
- **A8 — couplings: `related: [252, 172]`, `depends_on` stays empty.** 0172 (pipe-shape sweep)
  rewrites `test_docket_build.sh` and `test_docket_review.sh` — the same assert lines this change
  reflows — and its spec records `related: [253]`; 0252 shares the `tests/lib/` home (already in
  frontmatter). Both are textual/ordering collisions, orderable either way; whichever lands second
  reconciles at rebase by intent (learnings `concurrent-edits-compose-at-rebase`). Rejected:
  `depends_on` in either direction (no semantic dependency); prose-only mention (owner wants
  couplings in fields).
