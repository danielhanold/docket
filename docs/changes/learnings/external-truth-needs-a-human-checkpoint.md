---
slug: external-truth-needs-a-human-checkpoint
hook: "When a value's truth lives outside the repo (a vendor model ID, an external API name), no in-repo test can be its oracle — route it to a named human verification item instead of writing an assert that can only ever pass."
topics: [testing, verification, config]
changes: [184, 192, 205, 304]
created: 2026-08-01
updated: 2026-08-13
promotion_state: candidate
promoted_to:
---

## Apply
Some values a change ships are true or false only against a system the repo cannot see: a vendor
model ID, an external service's enum, a remote path. The repo's tests compare **generated output
against the sidecar that generated it** — both sides move together, so the assert is green whether
the value is correct or a typo. That is not a gap to close with a better assert; it is a structural
property of a mirror test, and writing more asserts around it manufactures the appearance of
coverage over an unverifiable claim.

Recognize the class at plan time by asking **where the oracle lives**:

- **In the repo** — write the guard, mutation-test it ([[guards-are-code]]).
- **Outside the repo** — no guard exists. Name the value in a **human verification item** on the
  change's `results:` file, say exactly what run would certify it, and state why no test can. A
  harness handed an unknown ID typically falls back to a house default **silently**, so the failure
  mode is not an error but a wrong-tier run nobody notices.

The distinguishing question is not "is this value new?" but "is this value new *to the outside
world*?" Re-pointing an ID the repo already ships elsewhere is covered by the existing corpus; a
value with no prior occurrence anywhere in history has never been exercised by anything.

**Outside-truth is not only values — it is behavior.** A flag's semantics, what a subcommand exits
with, whether an omitted option falls back or errors: each is a fact owned by the external tool, and
each is as unassertable as an ID. Read them off `--help` if you must, but record that the source was
documentation rather than an executed run — the two are different grades of evidence, and a one-line
`--help` summary is the weakest of them. Two corollaries follow. First, **a probe whose own failure
semantics are unknown is not a substitute for the human checkpoint**: adding a preflight check whose
exit code you have not established converts an unusual-but-working setup into a hard abort, which is
worse than the gap it closes — leave it as a named item and say what run would settle it. Second,
the rule works **at design time, not only at results time**: when a cheaper option exists that
introduces no new outside-truth, prefer it, and say in the results file that you did.

**Outside-truth also covers the ENVIRONMENT the suite cannot reach.** A property that only holds on
a machine state the build loop never occupies — a cold cache, a fresh clone, no credentials, no
network — has no in-repo oracle either, for the same structural reason: every gate run is warm, so
the assert can only ever pass. Route it to a named human item and say what state the certifying run
must start from. Two things follow. First, write the item as a *state to reproduce*, not a
conclusion to confirm ("run this from an empty module cache"), because the certifying run is the
first execution against that input class and may fail for a reason nobody predicted. Second, budget
for it to find something: a verify item that reddens has not gone wrong, it has done the one job no
green suite could do.

## War story
- 2026-08-01 (#184, PR #147) — The four-tier build-profile ladder introduced
  `cursor-grok-4.5-low`, the one shipped value in the change that was genuinely new: a
  boundary-anchored grep found no occurrence anywhere in the repo's history (the cursor family
  shipped only `-low-fast`, `-medium`, `-high`, `-high-fast`). Per ADR-0015 docket keeps no vendor
  allowlist by design, and every pin assert compares the generated wrapper against the sidecar —
  so the sidecar is both the input and the oracle, and **no test in this repo can ever detect a
  wrong ID**. The resolution was not a test but merge-gate item 2 on the results file, pointing at
  the one run that certifies it (`docs/cursor/validation.md` Phase 7 step 1, which requires an
  explicit `**Build profile:** economy` dispatch that reports its resolved model). Daniel confirmed
  the ID valid the same day. Sibling observation from the same change: a repo-committed rename
  cannot reach `~/.config/docket/config.yml` or a machine-local `.docket.local.yml` either — the
  outside-the-repo boundary cuts on writes as well as reads (see
  [[config-shape-change-strands-outer-layers]]).
- 2026-08-02 (#192, PR #150) — Second hit, and the first where the *whole* shipped table was
  outside-truth: registering opencode meant three brand-new OpenRouter model IDs, none with any
  prior occurrence in repo history. The rule was applied as written rather than rediscovered — the
  IDs were routed to named merge-gate items on the `results:` file ("catalog presence is not
  entitlement — confirm they resolve under your credentials"), alongside live-certification of the
  standard and premium rungs and one real end-to-end dispatch, with the waived set (max rung, the
  three review rungs, classification, escalation) stated explicitly. Worth noting for the class:
  the economy rung *was* certified during the build via `opencode debug agent`, which prints fully
  resolved config — that settles **spelling** questions (it is how `reasoningEffort:` was confirmed)
  but still is not an executed run, so resolved-config evidence and entitlement evidence are two
  different checkpoints.
- 2026-08-05 (#205, PR #156) — Third hit, and the one that widened the rule from *values* to
  *behavior*. The opencode runner adapter shipped **no new model IDs at all** — that was the point:
  the spec proposed `openrouter/x-ai/grok-4.5` for the premium/max rungs and the build deliberately
  used the Kimi ID `agents/harness-defaults.yml` already carried from #192, so the recipe pointed
  nothing new at the outside world. Applying the rule as a design constraint cost nothing and
  removed a whole checkpoint. What remained outside-truth was entirely **behavioral**, and all of it
  had been read off a single line of `opencode run --help`: whether an omitted `--variant` yields the
  provider default or an error, what `--auto` actually grants and how it interacts with a deny-list,
  and whether opencode's formatted stdout relays legibly through the shim (`opencode run` has no
  `--output-last-message` analogue, and parsing `--format json`'s unversioned event schema could
  silently truncate, so the adapter relays verbatim). Each went to the results file as a named item.
  The sharpest instance was the auth preflight: `codex.sh` probes `codex login status`, so the
  obvious parity move was an `opencode auth list` probe — but its exit code on a machine with **zero**
  credentials could not be established without destroying real ones, and a probe with unknown failure
  semantics would turn an unusual-but-working setup into a hard abort. The adapter checks the binary
  only and the question became a results item. Declining to probe was the correct call, not a gap.
- 2026-08-13 (#304, PR #204) — Fourth hit, and the one that widened the rule from *values and
  behavior* to the **environment**. The Go skeleton's suite gate caches Go modules under
  `<git-common-dir>/docket-go-cache/` so it is offline-capable after the first fetch — a property
  no assert in a warm-cache repo can test, since every gate run is by definition warm. It went to
  the results file as a named human verify item stating the state to start from (an isolated
  `GOMODCACHE`/`GOCACHE`). The certifying run **failed**, and not for the anticipated one-time-fetch
  reason: it exposed a real defect in `tests/test_go_toolchain.sh`, where `go list ./... 2>&1`
  word-split download chatter into gofmt's argument list (see
  [[captured-stderr-becomes-arguments]]). The item was written to certify offline capability and
  earned its keep by catching something else entirely — the argument for phrasing these as "run it
  from this state" rather than "confirm that X". The change's second item was the ordinary shape of
  this rule: ratifying that `docket help <unknown-topic>` exits 2 rather than Cobra's default
  exit 0, a behavioral choice the repo can assert but not *decide*.
