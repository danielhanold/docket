---
id: 246
slug: make-the-docket-example-yml-guard-suite-bite
title: 'Make the docket-example-yml guard suite bite'
status: in-progress
priority: medium
type: fix
created: 2026-08-07
updated: 2026-08-08
depends_on: []
related: [150]
discovered_from: [178, 187, 121]
adrs: []
spec: docs/superpowers/specs/2026-08-07-make-the-docket-example-yml-guard-suite-bite-design.md
plan:
results:
trivial: false
auto_groomable: true
branch: feat/make-the-docket-example-yml-guard-suite-bite
claimed_at: 2026-08-08T01:32:46Z
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-07-make-the-docket-example-yml-guard-suite-bite-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-07-make-the-docket-example-yml-guard-suite-bite-design.md) |
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
