---
id: 147
slug: extend-the-2c-orphan-key-check-past-its-column-0-anchor-to-n
title: Extend the (2c) orphan-key check past its column-0 anchor to nested keys
status: killed
priority: medium
type: fix
created: 2026-07-28
updated: 2026-08-07
depends_on: []
related: [121, 149]
discovered_from: [122]
adrs: []
spec:
plan:
results:
trivial: false
auto_groomable: false
branch:
pr:
blocked_by:
reconciled: false
---

## Artifacts

<!-- docket:artifacts:start (generated — do not hand-edit) -->
<!-- docket:artifacts:end -->

## Why

`tests/test_docket_example_yml.sh`'s `(2c)` orphan-key check enumerates `.docket.example.yml`'s
keys with the same column-0 anchor that change 0122 is removing from the scope-tag guard
(`^[A-Za-z_][A-Za-z0-9_]*:`). Every nested key — the 17 under `finalize`, `learnings`, `reclaim`,
`auto_capture`, `runners.codex`, and `skills` — is therefore invisible to the orphan-key direction
too: a documented nested key with no consumer anywhere in the codebase would never be flagged.

Change 0122 deliberately left this alone and recorded it as an observation. Its spec's assumption 8
explains why it is not a mechanical widening: `(2c)` anchors on **consumers**, and nested keys reach
their consumers through different paths than top-level keys do — `runners.*` through the runner
adapters, `skills.*` through the `SKILL_*` export names rather than the YAML key names. Deciding
what "has a consumer" means per nested-key family is a design question, not an edit to a regex.

## What changes

Decide and implement the consumer-resolution rule for nested keys in the `(2c)` orphan-key check,
then extend its enumeration past the column-0 anchor. At minimum the design has to settle how
`skills.*` keys map to their `SKILL_*` exports and how `runners.<runner>.*` keys map to their
adapter call sites, since neither is a literal key-name grep.

## Out of scope

- The scope-tag guard itself (change 0122 owns it).
- The classification manifest / `elsewhere:` check (change 0121 owns it).

## Auto-groom blocked

**2026-07-28** — autonomous grooming abstained. A full design was drafted and the critic
**falsified its central structural claim**; the corrected picture shrinks this change to something
whose continued existence is a backlog-composition call, which the drain may not make.

### The undecidable decision

Once the critic's corrections are applied, the residual content of 0147 is roughly: rename one
assert message, add one explanatory comment, and extend `(2c)` to a nested population that is
**already covered more strongly one section up**. Whether that is still worth its own change — or
whether the stub is better closed as already-solved by changes 0102/0122/0127 — is a verdict on the
backlog's composition. Kill is never autonomous, and shipping a redundant assert to retire a stub is
the wrong way to avoid saying so.

### What a human should supply

- The ruling: build the (small, honest) residual, or kill 0147 as already-solved.
- If build: confirm that deleting the six top-level headers' existing consumer anchor is
  unacceptable (this design says it is), so the widening is additive only.

### Verified findings, ready to re-use on re-arm

All of the following were checked against the running `tests/test_docket_example_yml.sh`, and
several contradict the stub's own framing.

**The stub's premise is right; the obvious fix is wrong.** `(2c)`'s private
`sed -nE 's/^([A-Za-z_][A-Za-z0-9_]*):.*/\1/p'` walks exactly 16 top-level keys and zero nested
ones. But widening it is not the fix, for three separate reasons found below.

**`(2b)` already subsumes `(2c)` for nested keys.** For every `resolved:` key `(2b)` requires
`^EXPORT=.*\bleaf\b` in the resolver; for every `elsewhere:` key it greps the **specific named**
consumer. Both are strictly stronger than `(2c)`'s any-of-five union grep. `(2c)`'s only
independent contribution today is the three `correspondence_exempt` keys plus the six top-level
headers.

**A leaf grep is not categorically vacuous — the first draft overstated this.** Against the five
declared consumers, `sandbox` and `network` resolve to `scripts/runners/codex.sh` alone,
`lease_ttl` to the resolver alone, and `test_command` / `require_pr_approval` to the resolver plus
the finalize SKILL. Five of sixteen nested leaves resolve precisely — including
`require_pr_approval`, the historical bug the whole section was built around. It is the *generic*
leaves (`enabled`, `cap`, `gate`, `types`, `auto`, `plan`, `build`, `review`, `finish`) that go
vacuously green. Any design here must split those two populations rather than ruling on all
sixteen at once.

**The `elsewhere:HEADER` exemption would be a regression, not a simplification.** `(2c)`'s `sed`
matches a bare `finalize:` line, so all six top-level headers are consumer-grepped today and all
six pass. Exempting them removes a live anchor. Scope any header exemption to **nested** headers
(`runners.codex`) only. Note also that `is_header_key` is called on the *leaf*, so `runners.codex`
is satisfied by any bare `codex:` opener anywhere in the file — it proves a block exists, not that
it is *this* block.

**A population floor over `$example_keys` would be cargo-cult, and 0122 explicitly forbids its
shape.** The file already pins the population exactly (`expected_key_count=36`,
`expected_nested_key_count=17`), and change 0122 recorded the rule in-file: a floor's count MUST
come from the guard's own pass output, never from `example_keys_raw` — otherwise the floor stays
green while the guard reaches zero nested keys, which is the exact vacuity a floor exists to catch.

**Switching `(2c)` to `$example_keys` is not behavior-neutral.** It adds three commented keys
(`runtime`, `agents`, `agent_harnesses`) the `sed` never walked. All three currently pass, but the
change is real and must be stated rather than claimed away.

**Couplings recorded.** `related: [121, 149]`. Change **0121** edits the same file's `elsewhere:`
arm — the very anchor any nested-key design leans on — and is a concurrent editor; keep both edits
additive and reconcile at rebase. Change **0149** is settling the proportional-bound shape for the
sibling guard in `tests/test_docket_config.sh`; whatever form lands there should be reused here
rather than a third form invented.

### Recommendation

Lean toward **kill**, or toward a deliberately tiny build. The defect the stub names is real, but
the coverage it asks for already exists at greater strength in `(2b)`, and the honest remaining
delta does not obviously justify a PR. A human should make that call.


## Why killed

Killed at the 2026-08-07 backlog triage: subsumed. (2b)'s per-key checks are strictly stronger than (2c)'s union grep for every classified key, so the column-0 residual is nearly empty — as this change's own auto-groom analysis concluded, leaning kill. Remaining example-yml guard hardening lives in #0246.
