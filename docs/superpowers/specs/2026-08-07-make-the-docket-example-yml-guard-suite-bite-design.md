<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0246 — Make the docket-example-yml guard suite bite](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-08-0246-make-the-docket-example-yml-guard-suite-bite.md)**
<!-- docket:backlink:end -->

# Make the docket-example-yml guard suite bite — design

**Change:** 0246 · **Date:** 2026-08-07 · **Groomed:** autonomously (auto-groom)

Consolidates #0178 (truncation), #0187 (mirror guards), #0121 (`elsewhere:` word-grep) into one
ordered build over `tests/test_docket_example_yml.sh` (+ `tests/test_grep_portability.sh`).

## Live reproduction (done during grooming — reframes #0178)

Run with the system PATH only (`PATH=/usr/bin:/bin bash tests/test_docket_example_yml.sh`):

- 103 of 393 asserts run, zero failures printed, then
  `line 1710: unexpected EOF while looking for matching '` + `syntax error: unexpected end of file`,
  **exit 2**.
- Root cause is **not grep**. With that PATH, `bash` resolves to `/bin/bash` **3.2.57**, whose
  `$(...)` parser cannot see heredocs: in `scope_guard_awk="$(cat <<'SCOPE_GUARD_AWK' ...)"`
  (line 684) it scans the heredoc body as shell and chokes on the backtick in the comment
  `` (`note: |` followed by ...) `` (line 688). Minimal repro: `/bin/bash -c 'x="$(cat <<EOF ... ` ... EOF)"'`
  fails identically; bash 5.3 parses the file clean. Everything from line 684 to EOF (290 asserts,
  including the whole mirror/round-trip family) never executes.
- Severity correction vs the stub: the failure is *not* fully silent — exit code is 2, so
  `scripts/run-tests.sh` would report it red. But run-tests.sh re-execs itself under the
  configured Bash 4.3+ runtime and runs every test file with `$TEST_BASH`, so the official
  runner never hits this at all. The exposed path is direct invocation (`bash tests/...`, the
  file's own documented run line) on a system-bash-first environment, where 290 asserts
  silently don't run before the (cryptic) parse error.
- Independently verified: current macOS `/usr/bin/grep` **does** support ERE `\b` (matched in a
  live probe). The `\b` ban (test_grep_portability.sh:50) still stands as a source-portability
  rule — git-grep ERE and older BSD greps return zero silently — but `\b` is not the truncation
  mechanism and no runtime failure from the three `\b` sites was observed.

## Part 1 — kill the truncation, fence the class

1. **Bash version gate** at the top of `tests/test_docket_example_yml.sh`: if
   `BASH_VERSINFO[0] < 4`, print one loud line naming the real requirement (mirroring the
   run-tests.sh prologue's *message shape* — its own floor is 4.3 for `wait -n`; this file
   needs only 4) and `exit 2`. Bash 3.2 parses incrementally,
   so an early exit runs before the line-684 construct is ever parsed — verified by the 103
   asserts that do run today. This converts a cryptic mid-file parse error into a one-line
   diagnosis at line 1.
2. **Convert the three `\b` sites** (lines 376, 409, 585) to portable EREs. 376's embedded form
   needs an explicit class (`(^|[^[:alnum:]_])$leaf_k([^[:alnum:]_]|$)`-shaped); 409/585 may use
   the same form for uniformity. Re-assert the affected manifest checks still pass and still
   redden under their existing mutation probes.
3. **Extend `tests/test_grep_portability.sh`** with a second banned class: the escaped
   double-quoted source form `\\b` / `\\<` / `\\>` (two source backslashes, the form all three
   step-2 sites use), over the same walk, same docs/ exclusion, same single scan implementation +
   controls pattern as the interval class. Single-quoted `\b` through PATH grep is deliberately
   NOT banned — it is established, comment-blessed repo idiom (~26 sites, e.g.
   tests/test_docket_build.sh:744-747 blesses them explicitly) and converting it would be a scope
   expansion nothing in the stub supports. The class's own needed literals are assembled at
   runtime for self-membership, exactly like MAX_BOUND's. Mutation-prove: a fixture with a
   double-quoted `\\b` pattern reddens; the current tree is clean after step 2 (measured: the
   three sites in test_docket_example_yml.sh are the only `\\b`-form occurrences outside docs/).

## Part 2 — mirror + round-trip guards (#0187)

4. **Reverse mirror loop**: today's loop iterates the sidecar (`hd_agents`), proving
   sidecar ⊆ example. Add the converse per the `correspondence-guard-runs-one-way` learning:
   extract the agent keys of each example harness slice and assert each exists in the sidecar
   block (set membership + an arity assert that example-row count equals sidecar-row count per
   harness). Values need no reverse comparison — the forward loop already compares both fields.
   Mutation-prove both directions (a stale example-only row must redden).
5. **Fix the round-trip slice terminator** (line ~1005): it currently ends at the cursor
   `finalize-change` row, which in the example sits *above* the cursor build rows (425 vs 426-432)
   and above the entire opencode block (439-455) — so those rows (cursor build *and* review rows,
   plus all of opencode) never get round-trip coverage.
   Re-terminate on the example's final row: the last harness block's `build-max` row, with the
   model derived from the sidecar (`hd_field "$HD" <last-harness> build-max model`), not a
   hand-written literal. Guard the ordering assumption instead of trusting it: assert the
   uncommented slice contains every `HD_SHIPPED_HARNESSES` header, so a re-ordered example or a
   fifth harness appended after the anchor reddens rather than silently shrinking coverage.
   Extend the round-trip's existing per-harness wrapper asserts to cover a cursor build row
   (previously unexercised).
6. **Non-prefix-matchable terminators**, both sites (mirror `ex_slice` anchor and the round-trip
   anchor): follow the escaped model value with a boundary class (`[,[:space:]}]`-shaped) so
   `claude-opus-5` cannot match cursor's `claude-opus-5-high` row. Mutation-prove: deleting the
   claude terminator row must redden the terminator guard itself, not just downstream asserts.
7. **Fix the stale comment**: replace "all thirty-nine rows" with derived phrasing ("every row of
   every shipped harness block") — never a restated number.

## Part 3 — `elsewhere:` proves a code-shaped read (#0121)

8. Tighten the line-409 arm from a bare word-boundary grep to a **shape-tightened grep**
   (the stub's committed default; no consumer-header convention). Two conditions on the named
   consumer: the match line is not a comment (first non-space char not `#`), and the key occurs
   in a code-shaped context, not bare inside prose. The shape set must be DERIVED from the six
   entries' actual mentions, not guessed — e.g. `runners.codex.sandbox`'s only code mention in
   scripts/runners/codex.sh is the `--sandbox` flag (line 83), so flag-argument context is a
   required shape alongside quote/`=`/`:`/`$` adjacency. One entry cannot meet any code shape:
   `github_project`'s only mention in scripts/docket-config.sh is a bare space-delimited token in
   a `for _fkey in ...` fence list (line 400), and the classifier's own comment
   (test_docket_example_yml.sh:208-213) already calls that anchor "documentation-only, unlike
   every other elsewhere: entry". Route it through an explicit, documented per-key exemption
   mirroring the resolved arm's existing `correspondence_exempt` mechanism — never by widening
   the shape set until prose passes. Exact ERE is the builder's to settle with evidence; the
   acceptance fixtures are fixed here: the five non-exempt `elsewhere:` leaf entries (agents,
   agent_harnesses, runners.codex.sandbox, runners.codex.network, runners.opencode.permissions)
   stay green; github_project passes via its named exemption, with the exemption list asserted
   to hold exactly that one key; and a fixture reproducing the historical false positive (the key
   appearing only in English prose inside an embedded heredoc prompt — 0102's `timeout` case)
   reddens. All current
   targets are shell scripts, so shell shapes suffice; if a prose consumer (SKILL.md) is ever
   reclassified to `elsewhere:`, the shape set must be revisited (leave a comment saying so).

## Build order (binding)

Part 1 first — parts 2 and 3 edit the region that never executes under a pre-4 bash, so their
green runs are only trustworthy after the gate lands. Then part 2, then part 3 (same-file,
sequential to avoid self-conflicts). Full suite via `scripts/run-tests.sh` at the end.

## Out of scope

- Suite-wide toolchain pinning/reporting — #0150 (its spec owns the run-tests.sh seam).
- (2c) nested-key extension — #0147 killed as subsumed.
- Removing the backtick from the line-688 comment or auditing other test files for the
  heredoc-in-`$()` construct: the version gate makes bash 3.2 exit before parsing anything, and
  every other test file is reached through run-tests.sh's `$TEST_BASH`. Noted as a known
  bash-3.2 hazard in the gate's comment.

## Assumptions

- **A1 — Version gate over bash-3.2 compatibility.** Chosen: a bash>=4 fail-fast prologue in this
  test file. Rejected: (a) making the file parse under 3.2 (de-backtick the heredoc) — fragile,
  unguardable (any future backtick or 3.2-ism reintroduces it silently) and pointless given the
  repo's existing Bash 4+ runtime floor in run-tests.sh; (b) doing nothing because run-tests.sh
  already re-execs — the file's own header documents direct invocation, and the observed direct
  failure mode is 290 skipped asserts plus a misleading error.
- **A2 — The `\b` fixes stay in scope despite the reframed root cause.** The stub predicted the
  truncation was grep-side; reproduction shows it is bash-side and that current macOS BSD grep
  accepts `\b`. There is NO repo-wide `\b` ban (the stub's AGENTS.md citation is false against
  the current tree; test_grep_portability.sh:50 documents only that guard's own pattern, and ~26
  single-quoted `\b` sites are deliberate, comment-blessed PATH-grep idiom). The three-site
  conversion is kept anyway on the stub's explicit commitment plus cheapness: the sites are the
  repo's only `\\b`-escaped-form patterns, and all three are `_`-safe under an `[^[:alnum:]_]`
  class. Rejected: dropping part 1's steps 2-3 as moot; converting all 26 blessed sites (scope
  expansion nothing supports).
- **A3 — Guard bans only the escaped double-quoted `\\b`/`\\<`/`\\>` source form.** Chosen
  because the single-backslash form is blessed repo practice, not a defect: banning it would
  redden ~26 healthy sites on the guard's first run. The escaped form is exactly the three
  step-2 sites, so the tree is provably clean after conversion. Comment lines are irrelevant to
  this form (no comment spells `\\b`), keeping the scan simple and self-membership symmetric
  with the interval class.
- **A4 — Reverse mirror is key-set + arity, not a second value comparison.** Values are already
  compared row-by-row by the forward loop; duplicating them reverses nothing. Per the learnings
  ledger the reverse direction is mandatory, not optional.
- **A5 — Round-trip terminator derived from the sidecar, ordering guarded by a headers assert.**
  Rejected: another hand-written literal (the current one is exactly what went stale when 0192
  appended opencode); slicing to EOF-of-block via indentation heuristics (fragile against the
  surrounding prose the comment at ~1000 warns about).
- **A6 — Shape-tightened grep for `elsewhere:`, not a consumer-declared key header.** The stub
  committed this default; a header convention is a new repo-wide contract for a 6-entry problem
  and moves the maintenance burden onto every consumer file. Comment-region exclusion alone was
  rejected: the demonstrated false positive lived in a heredoc, on non-comment lines.
- **A7 — No dependency edges, but a real same-file coupling with #0150.** #0150's groomed spec
  rewrites `tests/test_grep_portability.sh`'s prologue (:87-93 becomes a source-and-call of its
  new `tests/lib/toolchain-report.sh`) — the same file this change's step 3 extends. The regions
  are disjoint (prologue vs a new scan class), so this is an ordinary reconcile-time collision,
  not a readiness gate: neither change needs the other's outcome, so `related: [150]` stands and
  no `depends_on:` is added. Whichever builds second reconciles against the landed file.
