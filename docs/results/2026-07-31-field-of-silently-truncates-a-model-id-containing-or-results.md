<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0173 — field_of() silently truncates a model ID containing / or :](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-07-31-0173-field-of-silently-truncates-a-model-id-containing-or.md)**
<!-- docket:backlink:end -->

# field_of() value-class truncation — results

Change: #0173 · Branch: `feat/field-of-silently-truncates-a-model-id-containing-or` · PR: (opened at close of this run) · Plan: `docs/superpowers/plans/2026-07-31-field-of-value-class-truncation.md` · ADRs: 0065

## Verify (human)

- [ ] **Confirm the abort posture is what you want for the global config layer.** A quoted or space-bearing value in `~/.config/docket/config.yml` now aborts `sync-agents.sh` in *every* repo on this machine, including the invocation inside `install.sh`. Previously such a value matched nothing and was silently ignored, and generation succeeded. The diagnostic is self-describing and names the offending file, but this is a real behavior break on a machine-wide surface. Change 0181 (minted below) documents the rule; it does not soften the gate.
- [ ] **Sanity-check your own layers before merging:** `bash sync-agents.sh --check` from a repo you actually use. It was `rc=0` on this repo and on a sandbox copy of the real global config (18 entries across `claude:` and `codex:`), but your `.docket.local.yml` was not exercised.

## Findings

**1. The spec asked for coverage its own design could not deliver — ADR-0065.**
The spec directed the new validator to mirror `hd_validate`'s single `consumed != raw` leg, *and* to fail a quoted value. Probing the real `hd_field`/`hd_field_raw` by execution showed those two instructions conflict: a quoted **but space-free** value (`{model: "claude-opus-5"}`) has `consumed == raw`, so the `!=` leg never fires. That comparison is precisely a test for *internal whitespace*, not a general bare-scalar test — the diagnostic's own remedy text ("write values unquoted") named a rule it could not enforce. Escalated and approved; built with an explicit quote leg beside the byte-for-byte `!=` leg, single-quote coverage included. Recorded as **ADR-0065**, whose rule generalizes to every `field`/`field_raw` validator pair in docket.

**2. `runner-dispatch.sh`'s bug was worse than the spec documented.**
The spec described truncation (`https://host/v1` → `https`). Execution showed a second, more severe shape: a value starting with `/` (`workdir: /Users/x/p`) does not match the old regex *at all*, so it yields the empty string and the key is `continue`d and dropped entirely — **after** having been claimed in `seen_keys`, so it also masks the same key in every lower-precedence layer with nothing. Silent key loss, not just truncation.

**3. Two over-broad behaviors caught at whole-branch review, both fixed (`ff9f0962`).**
- *A regression this change introduced.* The new block-mapping reader exported the **comment text** as the value for a comment-only line (`sandbox:   # TODO decide later` → `# TODO decide later`). The capture's `[[:space:]]*` is greedy and eats the space before the `#`, so the whitespace-preceded strip could never fire. `scripts/runners/codex.sh` would then have run `codex exec --sandbox '# TODO decide later'`, or `die`d outright on a commented-out `network:` — converting a cosmetic comment into a failed dispatch, the exact harm the tolerant posture exists to prevent.
- *A self-inconsistency in the new gate.* `validate_user_agent_values` exempted the pre-0046 flat shape as "already warned and dropped", but hard-failed on two other already-warned-and-dropped shapes: an `agents.<harness>` block outside `agent_harnesses` ("ignored (dead config)") and an agent key overriding no built-in ("ignored (typo?)"). Either, carrying a quoted value, blocked **all** wrapper generation. The carve-out now applies evenly, and deliberately errs toward validating — a harness block is skipped only when consumable by neither pass, since `USER_TARGETS` resolves after the gate. A regression assert pins that a **live** harness block is still a hard failure, so the carve-out cannot become a hole.

**4. Suite runtime grew ~66 s, and it is not the validator.** Measured, three runs each:

| | per `sync-agents.sh` invocation | invocations in `test_sync_agents.sh` | total |
|---|---|---|---|
| base (`origin/main`) | 3516 ms | 123 | ~432 s |
| this branch | 3563 ms (+1.3%) | 141 (+18) | ~498 s |

The validator costs ~47 ms per invocation — within noise. The growth is 18 new generator runs from fixtures the spec's coverage section required (three config layers independently, the `agents.default` vs `agents.<harness>` merge, and each distinct abort posture, every one of which structurally needs its own sandbox and its own generator run). The dominant cost is pre-existing: 3.5 s × 141. This is change **0175**'s target (`depends_on: [173]`), and 0175 should treat the +2.4 s the gate adds on a maximal 4-harness × 12-agent `--check` as part of its scope — the gate loop is O(harnesses × agent-keys × layers) `harness_agent_line` calls at ~6 forks each. `field_of_raw` also spends 3 forks plus a redundant subshell that 0175's memoization pass should carry across.

**5. Plan-supplied test code needed repair three times, once in my own edit.**
The plan shipped two knowingly-defective snippets and flagged them; the implementers repaired both and found a *third*, undocumented defect: the Task 3 `probe()` helper wrote its fixture under `runners: codex:` while dispatching `--runner probe`, so every value assert would have returned `<unset>` regardless of the fix — permanently red, not green-looking. Separately, my own review-fix assert over-escaped an apostrophe inside a double-quoted string (`typo'"'"'d`), producing an unterminated-quote syntax error that aborted the suite at line 1832 with **zero `NOT OK` and `rc=2`** — a green-looking assert count (526) that was really an early abort. Caught only because the assert total was lower than expected, not by any failure line.

## Follow-ups

Three stubs minted (auto-capture cap reached exactly; nothing suppressed, nothing deduped):

- **#0180** (`fix`) — apply ADR-0065's quote leg to `hd_validate` in `scripts/lib/harness-defaults.sh`, deliberately out of this change's scope, plus the related corner where `harness_agent_line` strips `#` comments *before* either reader runs, so `{model: c#5}` yields `raw == consumed == "c"` and passes the gate.
- **#0181** (`docs`) — document the unquoted, space-free rule in `README.md`'s two `agents:` examples and docket-convention's schema commentary. The gate enforces a rule stated nowhere a user would look before triggering it.
- **#0182** (`fix`) — `tests/test_runner_dispatch.sh`'s pre-existing facade sections read the developer's real `~/.config/docket/config.yml`: they unset `XDG_CONFIG_HOME` without pinning `DOCKET_HARNESS_ROOT`, so the global layer falls through to `$HOME`. Latent today, but it is machine state the test does not control.

Not minted, already tracked: the suite-runtime work is change **0175**, which carries `depends_on: [173]` and was sequenced behind this change precisely so it inherits the widened class and writes its equivalence tests against the fixed baseline.
