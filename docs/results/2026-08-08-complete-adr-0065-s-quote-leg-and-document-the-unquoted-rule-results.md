<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0255 — Complete ADR-0065's quote leg and document the unquoted rule](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0255-complete-adr-0065-s-quote-leg-and-document-the-unquoted-rule.md)**
<!-- docket:backlink:end -->

# Complete ADR-0065's quote leg and document the unquoted rule — results

Change: #0255 · Branch: feat/complete-adr-0065-s-quote-leg-and-document-the-unquoted-rule · PR: (see change manifest `pr:`) · Plan: docs/superpowers/plans/2026-08-08-complete-adr-0065-s-quote-leg-and-document-the-unquoted-rule-plan.md · ADRs: 76

## Verify (human)

- [ ] **Confirm the Bash 4+ / Bash 3.2 sidecar validators still agree on your own machine.** The suite pins their verdict parity, but the two paths are selected by `${BASH_VERSINFO[0]}` at runtime, and which one your shell takes is environment, not repo state. `bash --version` tells you which path `install.sh` will exercise for you; both were exercised here (the bash-3.2 path via a real `/bin/bash` 3.2.57 shim).
- [ ] **Run `install.sh` once against a real config layer that carries an `agents:` block**, ideally the machine-wide `~/.config/docket/config.yml`. This change converts three previously-silent truncations into hard aborts before any wrapper is written. If any layer on your machine currently carries a quoted model/effort value or a `#` inside a flow map, generation will now refuse loudly where it previously produced a clipped pin. That is the intended trade (spec §2), but it is the one behavior no in-repo test can observe for *your* config.

## Findings

**A blocker the spec's own site inventory missed — the third validator.** The spec enumerated exactly two validators needing ADR-0065's quote leg (`hd_validate`, `validate_user_agent_values`). Whole-branch review found a third, and it was the one that matters most: `validate_harness_defaults` in `sync-agents.sh` short-circuits to `hd_validate` only when `${BASH_VERSINFO[0]} -lt 4`, so on Bash 4+ — the default, and both call sites — an awk single-pass validator runs instead, carrying the same whitespace-only `consumed != raw` test. A quoted pin passed and shipped on the primary execution path. It was missed because it does not *look* like a `field`/`field_raw` pair: it has no such functions, just one awk program inlining both readings.

Recorded as **ADR-0076** — ADR-0065's rule binds by **role** (any code judging whether a config value is consumable), not by reader shape. Worth emphasising for the next auditor: before the fix, every test in the repo passed while a quoted pin shipped. The suite did not catch this; a whole-branch read did.

**Three copies now, not two.** The flow-map `#` predicate exists as `_hd_flow_map_has_comment` (harness-defaults.sh), `flow_map_has_comment` (sync-agents.sh), and the awk `flow_comment()` inside `validate_harness_defaults`. Duplication by value remains the accepted design — `harness-defaults.sh`'s header forbids coupling the shipped-data reader to the user-config readers, and extraction is #0256's scope. The correspondence is now pinned by a three-way table-driven parity test rather than by prose; that test is the regression net #0256's extraction should be checked against.

**Two guards that were green for the wrong reason**, both caught at review and both instances of "an assert that confirms the wording just introduced detects nothing":

- The `#`-leg fire probe used `{ model: c#5, effort: low }`, which strips to an *unterminated* map, so `effort` read as missing and `hd_validate` returned 1 *before* the change too — the assert detected nothing. The discriminating input is `{ model: claude-opus-5, effort: lo#w }`, where both fields stay readable post-strip and rc genuinely flips 0 → 1.
- All five documentation sentinels grepped only for `unquoted and space-free`, which already existed. The clause this change actually adds — `#` cannot appear inside the flow map — was unguarded at every site; deleting it from all five files left the suite green.

**Two environment hazards worth carrying forward**, both cost real build time here:

- The awk program in `sync-agents.sh` lives inside a single-quoted shell word, so **no literal apostrophe may appear in it, comments included**. One did, during a fix: the shell word truncated silently, `bash -n` still passed, and the failure surfaced only when sourcing the file. Use `\047` / `\042`.
- macOS ships BWK awk, where **`close` is a builtin and cannot be used as a parameter name**. It is a parse error that surfaces only at runtime and makes every awk-path assert fail at once, looking nothing like a logic bug.

**A mutation-test landing check that could not land.** The plan's original `grep -c "case \"$raw\" in"` used PATH `grep`, which is ugrep here; it reads the `$` as an anchor and reports `0` on an *unmutated* file — indistinguishable from a landed mutation, which would have made every mutation test in this change a false green. All landing checks now use `/usr/bin/grep -cF`.

## Plan deviations

**The plan was corrected mid-build, and the correction is recorded in the plan file itself** (commit `f03499ee`, and a dated note in its self-review section). Its first draft told Tasks 1 and 2 to add probes to `tests/test_harness_defaults_validator.sh` and *raise that file's budget row*. The first Task 1 worker returned `BLOCKED` against it, correctly: `tests/test_runtime_budgets.sh` pins the table's TOTAL precisely so a per-row raise reddens, and its remedy text refuses the raise by name. The worker also measured that file at **49.5s against a 50s row** — its margin was gone before this change added anything. Every new probe now lives in a new shard, `tests/test_harness_defaults_flow_map.sh`, which brings its own row: the guard's own first sanctioned case.

Two smaller, self-reported deviations: the README doc-slice regex was tightened (the plan's bare-path anchor also matched an earlier `change_types` example header, spanning two fenced blocks), and the `agent-layer.md` comment was first cut from three lines to two to fit a skill size budget — then restored to three once review flagged that as the budget driving the documentation rather than the reverse.

## Follow-ups

- **#0256 (config-reader consolidation)** — already `related:`, and this change materially raises its stakes: it now has **three** copies of each leg to consolidate or keep byte-identical, not two. The three-way parity test added here is the regression net for that work. No new stub minted; 0256 already owns this.
- **Skill size budgets on two `docket-convention` files were raised** (`SKILL.md` 6100 → 6150, `agent-layer.md` 2150 → 2200) with the in-diff justification that file's header requires. Both had been left at one word of headroom. Nothing to action, but worth knowing the next edit to either file has ~50 and ~40 words of room, not 1.
- **Review finding 6 (minor) was deferred, not fixed, and needs your call.** When the new quote leg fires, the diagnostic reads `value '"claude-opus-5"' is not a bare scalar — the reader consumes only '"claude-opus-5"'` — the same string on both sides of "consumes only", because on the quote leg there is no truncation. The reviewer proposed branching the message. It was deferred because fixing it contradicts spec Assumption 7 (existing diagnostic strings stay byte-identical) and 0181's explicit out-of-scope on diagnostic wording, both settled at groom time. The wording is inherited from the 0173 twin, not introduced here — but this change propagates it to two more sites, so the cost of leaving it grows.
- **Review finding 8 (minor) needs no action**, recorded for completeness: the `#` check inspects only the first raw line matching an agent key, so a *duplicated* entry whose second copy carries the in-map `#` is not examined. The duplicate is separately an error, so rc is 1 either way and no truncated pin can ship; the loss is diagnostic completeness only.
