---
id: 172
slug: normalize-the-banned-producer-pipe-shape-across-tests-and-he
title: Normalize the banned producer-pipe shape across tests and helpers
status: 'killed'
priority: medium
type: chore
created: 2026-07-30
updated: '2026-09-03'
depends_on: []
related: [253, 150]
discovered_from: [167]
adrs: []
spec: docs/superpowers/specs/2026-08-07-normalize-the-banned-producer-pipe-shape-across-tests-and-he-design.md
plan:
results:
trivial: false
auto_groomable: true
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
| Artifact | Link |
|---|---|
| Spec | [2026-08-07-normalize-the-banned-producer-pipe-shape-across-tests-and-he-design.md](https://github.com/danielhanold/docket/blob/docket/docs/superpowers/specs/2026-08-07-normalize-the-banned-producer-pipe-shape-across-tests-and-he-design.md) |
<!-- docket:artifacts:end -->

## Why

`AGENTS.md` bans `producer | early-exiting-consumer` (`grep -q`, `head`, `head -n1`) under
`set -o pipefail`: the producer takes SIGPIPE and the 141 becomes an intermittent failure. Several
existing test helpers still use exactly that shape, and change 0167's reviews found three more while
declining to fix them piecemeal — each time correctly, because normalizing one arm of a family while its
siblings keep the old shape makes the inconsistency worse and invites a later "fix" that reintroduces it.

Known instances: `fmv()` in `tests/test_docket_build.sh` (`awk | sed | head -n1 | sed`), `ex_field()` in
`tests/test_docket_example_yml.sh` (the sibling it was copied from), the non-vacuity floors using
`printf … | grep -c .`, and the `echo "$out" | grep -qxF` asserts in `tests/test_docket_config.sh`'s
`BLD-a`/`BLD-b` — which match the surrounding `RCL-a`/`AC-a` family, so the whole band moves together or
not at all.

Every instance found so far is benign in practice — exit status discarded inside `$( )`, inputs far under
the 64K pipe buffer, `grep -c` does not exit early. The risk is not today's flake; it is that the banned
shape reads as sanctioned house style, so the next helper copies it into a place where the producer is
large or the status is load-bearing.

## What changes

Sweep all tracked `*.sh` under `tests/` and `scripts/` (via `git ls-files`, no executable-bit
filter — most of the population is not chmod +x) for the banned producer-pipe shape, derive the
true site list from that whole-repo grep rather than the instances listed above, and convert each
to a `pipefail`-safe canonical form: here-string for variable producers (`grep -q P <<<"$var"`),
status-preserving capture for live producers (`var="$(cmd)" && grep -q P <<<"$var"`), and a
single-`awk` collapse for first-match helper chains (`fmv()`, `fm()`). Safe look-alikes in the
same assert families (`… | grep -c .` floors) convert too so the band moves together. Where a site
is deliberately exempt, a standard `# pipefail-ok: <reason>` comment says so, and a new grep-based
guard test (`tests/test_pipefail_shape.sh`, mutation-tested, visible skips, self-excluded,
budgets-registered) keeps the sweep from decaying. Design detail, hazard taxonomy, per-form
verdict-equivalence arguments, and batching order are in the linked spec.

## Out of scope

- Changing what any assert checks. This is a mechanical shape normalization; every test's verdict before
  and after must be identical.
- The prose-anchor reflow-fragility cleanup, which is its own separate change (0253 — related; it
  touches two of the same test files, so the two changes collide textually but are orderable
  either way).

## Why killed

Backlog review 2026-09-02 (Bash→Go migration): already fixed in Go — TestPipeShapes in internal/repoguard/shellshape_test.go guards the shape; every named site was a deleted Bash test.
