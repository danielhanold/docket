<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0172 — Normalize the banned producer-pipe shape across tests and helpers](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-09-03-0172-normalize-the-banned-producer-pipe-shape-across-tests-and-he.md)**
<!-- docket:backlink:end -->

# Design: normalize the banned producer-pipe shape across tests and helpers (0172)

## Problem

`AGENTS.md` rule 1 bans `producer | early-exiting-consumer` (`grep -q`, `head`, `head -n1`) under
`set -o pipefail`: when the consumer exits before EOF the producer takes SIGPIPE and the pipeline's
141 becomes an intermittent failure — or, worse (learnings `pipefail.md`, change 0083), makes a
`… && die` guard fail *open*. Essentially every test file and script in this repo sets `pipefail`,
yet a whole-repo sweep finds on the order of 400+ pipeline sites matching the shape family across
~60 files in `tests/` and `scripts/`. All instances found so far are benign today (small in-memory
producers, status discarded inside `$( )`), but the shape reads as house style and keeps getting
copied — five recorded recurrences (#11, #16, #46, #83, #108) and three more declined piecemeal in
0167's reviews.

## What this change does

1. **Derive the true site list at build time** from a whole-repo grep over all tracked `*.sh`
   under `tests/` and `scripts/` — enumerate via `git ls-files 'tests/*.sh' 'scripts/*.sh'` (or
   equivalent `find`), with **no executable-bit filter**: most shell files in this repo (including
   three of the stub's four named sites and all of `scripts/lib/`) are not chmod +x. Never derive the list from the stub's examples. The scanning patterns must be
   verified under `/usr/bin/grep` (PATH `grep` is ugrep and masks BSD portability bugs — learnings
   `grep-is-ugrep…`).
2. **Convert every site to a pipefail-safe canonical form** (table below), preserving each test's
   verdict exactly.
3. **Comment deliberate exemptions** with a single standard token, `# pipefail-ok: <reason>`, on or
   immediately above the site line.
4. **Add an enforcement guard test** (`tests/test_pipefail_shape.sh`) that scans `tests/` and
   `scripts/` for the banned shape family and fails on new unexempted instances, so the sweep does
   not decay.

## Hazard definition (what the scan matches)

A pipeline stage feeding a consumer that may exit before reading EOF:

- `| grep -q…`, `| grep -m…`, `| grep -l…` (quits at first match / first file)
- `| head`, `| head -n…`, `| head -c…`
- `| sed … q` forms only — `sed -n '…p;q'`, `sed '…;q'`. A `sed -n 1p` **without** `q` reads to
  EOF and is NOT a hazard.
- `| awk '… exit …'` where the `exit` is in a per-record rule (an `END{exit …}` reads to EOF and
  is safe)
- `| read`-like single-line consumers and pipes into helper functions that `exit` early (the #46
  production case)

**Not** a hazard (reads to EOF): `| grep -c`, `| wc -l`, `| sort`, `| sed` without `q`, full-file
`awk`. These are converted anyway when they sit in the same assert family (see Assumptions A3) but
are never *required* to change by the guard.

**Guard-enforceable subset**: the plain-text guard detects the first three bullet classes plus
single-line per-record `awk … exit` (multi-line awk stages may evade a line-grep — a false
negative the guard's stated boundary covers); pipes into early-exiting *functions* are not
statically detectable by a grep and stay a review/AGENTS.md obligation. The guard's header comment states
this boundary explicitly so it never claims more than it enforces.

## Canonical conversion forms

| Site shape | Canonical replacement |
|---|---|
| `echo "$var" \| grep -q P` / `printf "%s" "$var" \| grep -q P` | `grep -q P <<<"$var"` |
| `! echo "$var" \| grep -q P` | `! grep -q P <<<"$var"` |
| `[ "$(echo "$var" \| grep -c P)" -eq N ]` | `[ "$(grep -c P <<<"$var")" -eq N ]` |
| `printf "%s\n" "$var" \| grep -c .` (non-vacuity floors) | `grep -c . <<<"$var"` |
| `command \| grep -q P` (live producer) | `var="$(command)" && grep -q P <<<"$var"` — the `&&` (not `;`) preserves the producer's failure status that `pipefail` propagates today; where the producer's status is separately asserted or deliberately ignored, `;` plus a site comment is acceptable |
| `producer \| … \| head -n1 \| …` helper chains (`fmv()`, `fm()`) | collapse into one `awk` program over the input that selects the first match and trims internally — no pipe stages at all |

Per-form equivalence caveats, verified once per form rather than per site (recorded in the guard
test's header comment):

- Trailing newline: `<<<"$var"` appends exactly one, the same normalization `printf "%s\n"`
  performs; for `grep -q`/`-c` matching this is verdict-identical (a final unterminated line is
  still a line to grep), including `grep -qxF` whole-line matches.
- Empty input: `printf "%s" "" | grep -q P` sees zero lines while `<<<""` supplies one empty
  line — divergent for patterns that can match an empty line (e.g. `^$`, bare `.*`). Sites whose
  pattern can match empty get per-site inspection, not blind conversion.
- `echo "$var"` swallows a leading `-n`/`-e`/`-E` in `$var`; `<<<"$var"` does not. Convert
  `echo`-producer sites knowing the here-string is *more* faithful — flag any assert that
  (pathologically) relied on the swallowing.
- Producer failure status: see the live-producer row — `&&` joining preserves it; a bare `;`
  silently converts today's red into green (the fail-open direction of learnings #83).

Any site where these bytes or statuses are genuinely load-bearing is an exemption candidate or a
per-site-inspected conversion, never a silent mechanical one.

`|| true` is **not** an accepted fix — it discards the very status the assert reads.

## Enforcement guard

`tests/test_pipefail_shape.sh`:

- Greps all tracked `*.sh` under `tests/` and `scripts/` (no executable-bit filter — see §1) for
  the guard-enforceable hazard subset (`grep -qE` family, POSIX-portable classes, verified against
  `/usr/bin/grep`).
- Scans **inside single-quoted `assert '…'` strings too** — those strings are eval'd by the
  harness, so they are the primary site population (e.g. `test_docket_config.sh` BLD-a/BLD-b),
  not prose. Comment-only lines are skipped; genuine prose mentions of the shape get a
  `pipefail-ok:` token with reason.
- Skips lines carrying `pipefail-ok:` (same line or the line immediately above), but prints every
  exempted site it skipped (the `test_grep_portability.sh` visible-skip precedent; silent skips
  are a house defect per the 0190 work), and structurally excludes its own file (the
  `test_comment_anchor_style.sh` self-exclusion precedent) since it embeds the hazard patterns.
- Is **mutation-tested at build time**: a seeded synthetic violation in a temp fixture must redden
  it before it counts as done ("a guard is code: mutation-test it — or it is decoration",
  AGENTS.md).
- Registers a row in `tests/runtime-budgets.tsv` (every new `tests/test_*.sh` must, or
  `test_runtime_budgets.sh` goes red).
- On failure, prints file:line list plus the two canonical fixes (here-string / capture-then-match).

## Verification protocol (verdict preservation)

- Full suite green immediately before the sweep and after each file's conversion (excluding the
  new guard, see Batching); final full-suite gate at the end with the guard included and green
  (backgrounded run — the suite sits at the 600s Bash ceiling).
- Per converted file: the assert count (`grep -c` on the assert lines) and every assert *name*
  string are byte-identical before/after — only the command under test changes shape.
- The per-form equivalence arguments above (trailing newline, empty input, `echo` flag-swallow,
  producer status) are the semantic gate; suite-green alone cannot catch an assert that got
  *weaker* on its failure path, so producer-status-bearing sites get per-site inspection.
- No assert may change what it checks; a site whose conversion would alter the verdict is an
  exemption plus a note, never a behavioral edit (that would violate the stub's out-of-scope).

## Batching

Single change, commits batched per file (or per named family — the `RCL-a`/`AC-a`/`BLD-a`/`BLD-b`
band in `tests/test_docket_config.sh` moves in one commit, as the stub requires). The guard test
lands first as the inventory ledger but is **excluded from the per-file green gate** while
conversions are in flight (it is by design red until the last file converts); every other test
stays green at every commit, and the final gate runs everything including the guard. If the
harness cannot tolerate a known-red test mid-branch, the fallback is guard-lands-last with the
inventory kept in the plan instead.

## Out of scope

- Changing what any assert checks (stub).
- The prose-anchor reflow-fragility cleanup (stub; separate change).
- `skills/` markdown code excerpts and docs prose.
- Sites in vendored/third-party content (none known under `tests/`/`scripts/`).

## Assumptions

- **A1 — scope is all tracked `*.sh` under `tests/` + `scripts/`, no pipefail-presence carve-out
  and no executable-bit filter.** Chosen: enumerate via `git ls-files` — most of the repo's shell
  files (including three of the stub's four named sites and all of `scripts/lib/`) are not
  chmod +x, so an executable-only sweep would miss most of the population. Nearly every file sets
  `pipefail`; the few that do not (some top-level tests under `set -u` only, sourced libs that
  execute under callers' `pipefail`) convert too — the shape is banned repo-wide and spreads by
  copy. Rejected: executable-bit filter (near-total false negative); only files that
  literally set `pipefail` (fragile — sourcing flips the hazard on silently); whole repo including
  skills markdown (prose excerpts are not executed; noise).
- **A2 — hazard = consumer that can exit before EOF**, per the corrected table: `sed` only in its
  `…q` forms (a `sed -n 1p` without `q` reads to EOF and is safe), `awk` only with per-record
  `exit`. `grep -c`/`wc`/full `sed`/`sort` are safe. The guard enforces the statically detectable
  subset and says so; pipes into early-exiting functions (the #46 case) remain a review/AGENTS.md
  obligation. Rejected: banning every pipe (unworkable); claiming the guard covers function
  consumers (a grep cannot see them — the guard must not overstate its contract).
- **A3 — safe look-alikes inside assert families convert too.** The `printf … | grep -c .` floors
  and `| grep -cE` counters are not hazards, but leaving them preserves the copyable shape next to
  converted siblings — the stub's "whole band moves together" point. The guard still only enforces
  true hazards. Rejected: exempt-comment each safe look-alike (hundreds of noise comments); leave
  untouched (reintroduces the inconsistency the stub exists to end).
- **A4 — here-string is the default canonical form** for variable producers; for live command
  producers, capture joined with `&&` (`var="$(cmd)" && grep … <<<"$var"`) so the producer's
  failure status that `pipefail` propagates today is preserved — a bare `;` would flip today's red
  to green, the fail-open direction learnings #83 warns about, and is allowed only where the
  producer's status is separately asserted or deliberately ignored, with a site comment.
  Single-`awk` collapse for first-match helper chains (`fmv()`, `test_docket_example_yml.sh`'s
  `fm()` — the stub's "ex_field()" — and `yaml_get`-style `sed -n … | head -n1` pickers). Matches
  AGENTS.md's own prescription and the existing normalized files
  (`tests/test_sync_agents_validator.sh`, `tests/test_run_tests.sh`, `scripts/terminal-publish.sh`).
  Rejected: `;` joining as default (drops producer status); `|| true` guards (mask status,
  learnings #46 shows it as a patch not a fix); `set +o pipefail` toggles around sites (spooky
  action, easy to orphan).
- **A5 — exemption token is `# pipefail-ok: <reason>`.** One token, reason mandatory, honored by
  the guard on the site line or the line above. Expected to be rare (streaming producers where
  capture is genuinely wrong). Rejected: no exemption path (forces bad conversions); free-form
  comments (unenforceable).
- **A6 — an enforcement guard test is in scope.** The stub's Why names recurrence — "the next
  helper copies it" — as the real risk, and the family has five recorded reintroductions; a sweep
  with no guard decays back. The guard is a small grep-based test in the repo's existing
  style-guard idiom, with the obligations the idiom carries: mutation-tested with a seeded
  violation ("a guard is code" rule), visible reporting of exempted sites (the
  `test_grep_portability.sh` precedent; silent skips are a house defect per 0190), structural
  self-exclusion (`test_comment_anchor_style.sh` precedent), a `tests/runtime-budgets.tsv` row,
  and scanning inside single-quoted `assert '…'` strings since those are eval'd — they are the
  primary site population, not prose. Rejected: sweep only (documented recurrence says it won't
  hold); shellcheck (does not flag this shape); pre-commit hook (repo enforces conventions via the
  test suite); skipping quoted strings (would blind the guard to most real sites).
- **A7 — verdict preservation is verified structurally**, not re-derived per assert: identical
  assert names/counts per file plus a green full suite before and after, with the per-form
  equivalence arguments (trailing newline, empty input, `echo` flag-swallow, producer-failure
  status) recorded once in the guard test's header. Suite-green alone cannot catch a
  failure-path weakening, so sites where the producer's exit status or empty-input behavior is
  load-bearing get per-site inspection rather than blind conversion. Rejected: per-site manual
  proofs for all ~400+ sites (the forms are uniform); suite-green as the sole gate (blind to
  fail-open regressions).
- **A8 — one coupling: `related: [253]`.** Change 0253 (settle and enforce the prose-anchored
  guard house pattern) is minted and names `tests/test_docket_build.sh` and
  `tests/test_docket_review.sh`, both of which this sweep rewrites — a real file-collision
  coupling, recorded in frontmatter (forward link only). `depends_on` stays empty: the changes
  are orderable either way, colliding only textually. `discovered_from: [167]` already records
  provenance. Rejected: prose-only mention (owner explicitly wants couplings in fields);
  `depends_on: [253]` (no semantic dependency).
