# Guard the README's remaining config fences against key drift — results
Change: #108 · Branch: feat/guard-the-readme-s-remaining-config-fences-against-key-drift · PR: <url> · Plan: docs/superpowers/plans/2026-07-21-readme-config-fence-key-drift-guard.md · ADRs: 53

## Verify (human)

- [ ] Read ADR-0053 and confirm the **default-in** choice is what you want: every ```` ```yaml ```` fence you add to `README.md` from now on is guarded automatically, and a *non-config* yaml fence must carry `<!-- docket:config-fence: ignore -->` or the suite reddens. The burden deliberately falls on the rarer case.
- [ ] Confirm you are happy with the one `README.md` edit — line 233's blank line became `<!-- docket:config-fence: values -->` above the `reclaim:` fence. Rendering is unchanged (verified against CommonMark: an HTML block closes the list exactly as the blank line did), but it is a visible line in a user-facing document.

## Findings

**The change's own premise reproduced twice more.** The stub that proposed this change enumerated the unguarded fences and was already wrong when filed (it omitted the `reclaim:` fence). During the build the same failure mode recurred twice in the *guard* rather than the prose, both times as a fail-open path that kept the suite fully green while a real drift went undetected:

1. **The `values` marker had no population floor** (found by the whole-branch review). Deleting the marker from `README.md`, or displacing it one non-blank line earlier, left the suite green *even with `reclaim.lease_ttl` drifted*. `f9_value` was green for a reason other than the property it claimed — it would have read identically had the `values` machinery never been wired up. The design had reasoned carefully about the *typo* direction (a malformed token is a hard fail, because a typo'd `values` fails open and silent) and then missed deletion and displacement entirely.
2. **The first fix pinned *a* fence, not *the* fence** (found by the re-review of that fix). The added floor asserted "at least one fence is values-marked", so *relocating* the marker to fence 209 — which documents shipped defaults and therefore passes value equality — absorbed it harmlessly and the suite went green again with `lease_ttl` drifted 72 → 99.

Closed by a layered set: a `seen <line> <token>` record emitted for every fence the scanner reaches (before any skip), an exact-count floor that all 9 were visited, an at-least-one-values-marked floor, a whole-file reconciliation that every `docket:config-fence` line in the README is attached to a fence, and finally a **positive control** that pins the `reclaim:` fence's value coverage semantically — it mutates a `$tmp` copy of the README and asserts the drift *is* reported, so it holds regardless of which fence carries the marker. Each was mutation-tested; the relocation scenario was re-verified independently after the fix.

**The half-widening trap was real and only the value assert catches it.** `flatten_yaml` carried its key class twice (shape test + value strip). Widening only the shape test is invisible to every count-based floor — path counts are identical either way — and shows up solely as the extracted *value* coming back as the entire raw line. Task 1 asserts the value rather than the path count for exactly this reason, and the half-fix is pinned as a mutation test.

**One AGENTS.md violation was inherited verbatim from the plan.** The plan's Task 3 code used `printf ... | grep -Fxq`, a `producer | early-exiting-consumer` pipeline under `set -o pipefail` — the first rule in `AGENTS.md`, and a pattern this repo has been bitten by four times (changes 11, 16, 46, 83). The task review caught it; the here-string form was then directed up front for Task 6, which carried the same shape plus an early-exiting `awk ... {exit}`.

**Recorded as ADR-0053** — README yaml fences are guarded by default, with an opt-out marker grammar. It captures the default-in choice, existence-only-by-default with opt-in value equality, the nearest-preceding-non-blank attachment rule, and the asymmetric-failure argument for hard-failing a malformed token.

**Residual holes, documented rather than closed** (both in the section's inline comments and in ADR-0053's consequences):
- A future fence key whose name collides with a prose-comment word in `.docket.example.yml` would be silently accepted by `is_pseudo_key`. No collision exists among today's 12 top-level fence keys. Not closed because the only tight closure is an explicit two-key allowlist — precisely the enumerated floor this change exists to avoid.
- A `yaml` fence nested inside a wider four-backtick fence is discovered as a config fence. Latent: the README has no such block, and adding one trips the count floor first, which is the right order of operations.
- `flatten_yaml` handles inline lists only, so a fence using YAML block-sequence syntax (`board_surfaces:` / `  - inline`) reports `drop raw=2 flat=1`. Documented as a constraint on how README config fences must be written.

## Follow-ups

None minted. Two candidates were considered and deliberately not filed as changes:

- **The `flatten_yaml` block-sequence limitation** is now a documented authoring constraint rather than a defect — README config fences use inline-list syntax. Filing work to teach the flattener block sequences would be speculative.
- **The lesson from findings 1 and 2** is a build-loop learning, not a change: *a hard-fail grammar guards only the tokens it is pointed at — the marker's own presence and position need a separate population floor, and "at least one" pins a population, not the specific coverage you care about.* It belongs to the learnings harvest at close-out, and is left for it rather than minted as backlog.

## Notable plan deviations

- **Here-string form substituted for the plan's piped `grep -Fxq` / `awk ... {exit}`** at every site (Tasks 3 and 6), per `AGENTS.md`. Semantics identical; the plan's three code blocks were reconciled to the shipped form and a "Corrections applied during execution" section was added to the plan so the next reader is not taught the hazard.
- **Two review-driven fix rounds beyond the plan's six tasks**, adding six asserts the plan did not specify (the `seen` population floors, whole-file marker reconciliation, the `reclaim:` value-coverage positive control, and permanent fixtures for the floor-2 `empty` and `BAD`-marker branches, which the plan left exercised only at build time).
- **Task 1 touched section `(8)`** — a comment-only edit correcting a "deliberately narrow key regex" note that the widening made false. The plan authorized this explicitly as the sole permitted touch.

## Suite

`tests/test_docket_example_yml.sh`: 138 `ok -` at baseline → **157 `ok -`, 0 `NOT OK`**. Whole suite: **53/53 files pass** (3093 `ok`, 0 `NOT OK`), verified at the branch tip.
