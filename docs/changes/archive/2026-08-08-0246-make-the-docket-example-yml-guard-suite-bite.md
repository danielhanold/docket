---
id: 246
slug: make-the-docket-example-yml-guard-suite-bite
title: 'Make the docket-example-yml guard suite bite'
status: done
priority: medium
type: fix
created: 2026-08-07
updated: 2026-08-08
depends_on: []
related: [150]
discovered_from: [178, 187, 121]
adrs: []
spec: docs/superpowers/specs/2026-08-07-make-the-docket-example-yml-guard-suite-bite-design.md
plan: docs/superpowers/plans/2026-08-08-make-the-docket-example-yml-guard-suite-bite.md
results: docs/results/2026-08-08-make-the-docket-example-yml-guard-suite-bite-results.md
trivial: false
auto_groomable: true
branch: feat/make-the-docket-example-yml-guard-suite-bite
claimed_at: 
pr: https://github.com/danielhanold/docket/pull/179
blocked_by:
reconciled: true
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-07-make-the-docket-example-yml-guard-suite-bite-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-07-make-the-docket-example-yml-guard-suite-bite-design.md) |
| Plan | [2026-08-08-make-the-docket-example-yml-guard-suite-bite.md](https://github.com/danielhanold/docket/blob/main/docs/superpowers/plans/2026-08-08-make-the-docket-example-yml-guard-suite-bite.md) |
| Results | [2026-08-08-make-the-docket-example-yml-guard-suite-bite-results.md](https://github.com/danielhanold/docket/blob/main/docs/results/2026-08-08-make-the-docket-example-yml-guard-suite-bite-results.md) |
| PR | [#179](https://github.com/danielhanold/docket/pull/179) |
<!-- docket:artifacts:end -->

## Why

Consolidates #0178, #0187, and #0121 (2026-08-07 triage): three changes all editing `tests/test_docket_example_yml.sh`'s guard family. They must land together and in order, because #0178's defect means 290 of the file's 393 asserts — including the whole region #0187 hardens — never execute under a system-bash-first environment.

Groomed autonomously 2026-08-07 with a live reproduction that **reframes #0178**: the truncation is not grep at all. With `PATH=/usr/bin:/bin`, `bash` resolves to `/bin/bash` 3.2.57, whose `$(...)` parser cannot see heredocs — it chokes on the backtick inside the `scope_guard_awk` heredoc (line 688), killing everything from line 684 to EOF (103/393 asserts run, exit 2, cryptic parse error). `scripts/run-tests.sh` is already protected by its Bash 4.3+ re-exec; the exposed path is direct invocation, the file's own documented run line. Current macOS `/usr/bin/grep` accepts ERE `\b` (probed live), and there is no repo-wide `\b` ban — but the three double-quoted-escaped `\\b` sites (:376, :409, :585) are converted anyway per this stub's commitment, with a portability-guard class scoped to that escaped form only (the ~26 single-quoted `\b` sites are blessed PATH-grep idiom).

#0187's three legs all verified: the mirror loop proves only `sidecar ⊆ example`; the round-trip slice terminator (cursor `finalize-change`, example :425) excludes the cursor build/review rows and, since 0192, the entire opencode block; the slice terminators are prefix-weak (`claude-opus-5` matches `claude-opus-5-high`). #0121's `elsewhere:` arm is a bare word grep; it is tightened to a code-shaped-context match with derived shapes, with `github_project` (whose only consumer mention is a bare fence-list token, documented as documentation-only) routed through an explicit one-key exemption mirroring `correspondence_exempt`.

## What changes

Per the linked spec, in binding order:

1. Bash>=4 fail-fast gate in `test_docket_example_yml.sh` (kills the truncation loudly); convert the three `\\b` sites to portable EREs; extend `test_grep_portability.sh` with a banned class for the escaped `\\b`/`\\<`/`\\>` source form.
2. Reverse mirror loop (key-set + per-harness arity, mutation-proven both directions); re-terminate the round-trip slice on the last harness block's sidecar-derived build-max row with a shipped-harness-headers assert; boundary-class the terminators so prefixes cannot match; fix the stale "thirty-nine rows" comment with derived phrasing.
3. Shape-tightened `elsewhere:` grep (non-comment line + derived code shapes; five entries green, `github_project` via an asserted one-key exemption; the historical prose-heredoc false positive reddens).

## Out of scope

- The suite-wide toolchain pin/report decision — that is #0150 (related: its groomed spec touches `test_grep_portability.sh`'s prologue; disjoint regions, reconcile-time collision only, no dependency).
- `(2c)` orphan-key nested-key extension — #0147 killed as subsumed by `(2b)`.
- De-backticking the heredoc or auditing other files for the bash-3.2 `$()`-heredoc hazard — the version gate plus run-tests.sh's `$TEST_BASH` cover both paths.

## Reconcile log

### 2026-08-08 — reconciled at claim (no scope change)

Re-validated the spec against `origin/main` @ 483c5dad. Every load-bearing claim still holds:

- **Truncation reproduces exactly.** `PATH=/usr/bin:/bin bash tests/test_docket_example_yml.sh`
  → 103 asserts run, then `line 1710: unexpected EOF while looking for matching \`` and
  `line 1717: syntax error`. Under `/opt/homebrew/bin/bash` the file runs 393 asserts, all green.
  The 103/393 split the spec measured is unchanged.
- **The three `\\b` sites are still at 376, 409, 585**, and are still the only `\\b`/`\\<`/`\\>`
  escaped-form occurrences outside `docs/` in the tracked tree.
- **The `$()`-heredoc construct is still at line 684**, with the backtick-bearing comment at 688.
- **The round-trip slice still terminates on the cursor `finalize-change` anchor** (~line 1005),
  and the "all thirty-nine rows" comment is still there and still stale.
- **The `github_project` classifier entry and its documentation-only comment** are still at
  `test_docket_example_yml.sh:205-213`, unchanged.
- **`tests/test_grep_portability.sh` is still 225 lines** with only the `INTERVAL` class; its
  prologue (:84-93) is untouched, so #0150 has NOT landed and step 3 extends the file the spec
  described. #0150 remains `proposed build-ready` — the A7 reconcile-time collision is still
  hypothetical, and whichever change builds second absorbs it. No `depends_on:` added.

No drift, no scope adjustment, no fold-in. Build proceeds on the spec's binding order (part 1 →
part 2 → part 3).

**Auto-capture:** nothing minted. The one adjacent candidate — auditing the rest of `tests/` for
the bash-3.2 `$()`-heredoc parse hazard — fails admission gate 2 (independent value): every other
test file is reached only through `run-tests.sh`'s `$TEST_BASH` re-exec, so the hazard has no
live exposure there. Reported, not filed.

### 2026-08-08 — resume re-reconcile (origin/main advanced; no drift, no scope change)

The first run was interrupted mid-Task-4. `origin/main` has since advanced 483c5dad → 760cac67
(changes 0237, 0250, 0259 landing), so the resume-safety guard re-ran the pass rather than
trusting the earlier one. **Zero intersection with this change's surface**: no commit in
`483c5dad..760cac67` touches `tests/test_docket_example_yml.sh`,
`tests/test_grep_portability.sh`, `docs/docket-example.yml`, or
`agents/harness-defaults.yml` — the new work sits in `scripts/render-board.sh`,
`scripts/docket-status.sh`, `scripts/runner-dispatch.sh`, `scripts/verify-run.sh` and their
suites. #0150 is still unbuilt, so the A7 collision on `test_grep_portability.sh`'s prologue
stays hypothetical and no `depends_on:` is added.

Tasks 1–3 are committed on the branch (ecb07a38, b5f22bcd, fa1f0d66). The interrupted run's
uncommitted partial Task 4 was **discarded, not adopted** — the build restarts Task 4 from the
plan under the normal test-first contract. Build resumes at plan Task 4.

**Auto-capture:** nothing new surfaced this pass; nothing minted.
