---
slug: optimization-needs-a-measured-oracle
hook: "A performance change has no oracle in the suite — correctness asserts pass identically whether the optimization happened or not, so scope it and accept it on measured wall clock."
topics: [testing, performance, planning]
changes: [174, 175]
created: 2026-07-31
updated: 2026-08-01
promotion_state: candidate
promoted_to:
---

## Apply
A change whose whole purpose is to make something faster is the one change the test suite cannot
judge. Every existing assertion is about behavior, and a correct-but-inert implementation preserves
behavior perfectly — so green is the expected result of both shipping the optimization and shipping
nothing. Two consequences, at opposite ends of the change:

1. **Measure before scoping, not after.** An estimate written from a plausible per-unit cost is a
   guess with a spec's authority. Time the real unit and the real total before the estimate becomes
   the change's justification, or the change ships having captured all the available saving while
   the recorded promise was off by an order of magnitude.
2. **Accept on wall clock.** Add a per-file (or per-target) timing check to the evidence, and treat
   an unchanged or *worse* number as a red result even when every assertion is green. Where the
   optimization's mechanism is a piece of retained state, also mutation-test that the state is
   actually retained — the correctness asserts will not tell you.

Related but distinct from [[green-suite-untested-branch]]: there the fixture never reaches the
branch; here the branch is reached and the assertions are honest — there is simply no assertion
whose value depends on the property the change exists to deliver.

## War story
- 2026-07-31 (#174, PR #141 — merged) — A change replacing four per-assertion git-fixture builders
  with build-once-and-copy shipped both halves of this lesson in one build.
  **The estimate was the defect.** The spec asserted ~0.5–0.8s per fixture and ~165s of a 530s suite
  recoverable. Measured, per-fixture cost was ~0.127s and the four files went 193s → 170s (~12%).
  The implementation captured essentially all of the saving that existed; the number that justified
  the work did not. The real cost lived elsewhere entirely — `test_docket_config.sh` spends ~105s of
  ~109s in 121 real `bash scripts/docket-config.sh` invocations — so the suite is **invocation-bound,
  not fixture-bound**, and that reframing (not the spec's) is what the follow-ups inherited.
  **And the first implementation was inert.** The plan specified lazy template init
  (`[ -n "$TEMPLATE" ] || _build_template`) inside each helper; two of the four files consume that
  helper as `read -r W _ < <(new_repo)` — a **subshell** — so the assignment never reached the
  parent and every call rebuilt the template. That form ran green, with the full pre-existing
  assertion set intact and an empty `comm -23` independence guard, *while being slower than
  baseline* (18.9s vs 17.2s). Every fixture really was freshly built, so every independence
  assertion passed for the right value and the wrong reason. The only signal that caught it was the
  per-file wall-clock check. Fixed by initializing once at file scope and allocating roots via
  `mktemp -d "$TEMPLATE/fXXXXXX"`, with the reason recorded in-file so it is not "simplified" back.
  One durable technique from the same build: the ad-hoc probe that proved copied fixtures never
  mutate the shared template was **kept** as a template-integrity assertion before each file's final
  `exit` — turning a one-time investigation into a property checked across every future run.
- 2026-08-01 (#175, PR #144 — merged) — The first cache optimization cut the parser-tool count
  from 788 to 644–667, but did not meet the `<400` acceptance budget because validation still
  re-parsed each config layer. A second, Bash-4+-only validation sidecar reduced the retained
  fixture to 38 calls and the real generation median from 1.82s to 0.29s, while the Bash-3.2
  fallback retained its existing behavior. The performance guard mattered as much as the timing:
  bypassing the cache raised the same fixture to 597 calls and reddened it. Treat a mechanism
  metric as a companion to wall clock when the regression shape is retained work; measure the
  actual fixture and mutation-test that the metric still observes the optimized path.
