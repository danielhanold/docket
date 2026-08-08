<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0255 — Complete ADR-0065's quote leg and document the unquoted rule](https://github.com/danielhanold/docket/blob/docket/docs/changes/archive/2026-08-08-0255-complete-adr-0065-s-quote-leg-and-document-the-unquoted-rule.md)**
<!-- docket:backlink:end -->

# Complete ADR-0065's quote leg and document the unquoted rule — design

Change: 0255 (consolidates killed #0180 correctness half + killed #0181 docs half; discovered from 0173's whole-branch review). ADR-0065 is Accepted and states the family rule: every `field`/`field_raw` reader pair needs an explicit quote leg beside the raw-vs-consumed comparison, because that comparison is a whitespace test only.

## Problem

1. `hd_validate` (`scripts/lib/harness-defaults.sh:150-159`) still has only the `[ "$v" != "$raw" ]` leg. A quoted but space-free value (`{model: "claude-opus-5"}`) has consumed == raw under both `hd_field` (`:88`) and `hd_field_raw` (`:104`), so the quotes ride into the emitted pin verbatim — exactly ADR-0065's documented gap, live in the shipped-defaults reader.
2. `harness_agent_line` (`sync-agents.sh:436` sed path, `:444` `${line%%#*}` path) strips `#` before either reader runs, so `{model: c#5}` truncates to `c` with raw == consumed == `c` and passes the gate silently — the same silent-truncation class in a corner the current validator structurally cannot see.
3. The unquoted/space-free rule is documented nowhere a user looks before tripping the gate: zero hits for "unquoted" in README.md, skills/docket-convention/ (including references/agent-layer.md), and .docket.example.yml. The only statements are the diagnostics themselves (`sync-agents.sh:611-612`, `harness-defaults.sh:158`).

## Design

### 1. Quote leg in `hd_validate`

Copy the sync-agents reference shape (`sync-agents.sh:606-613`) into `hd_validate`'s per-field loop: extend the existing `elif [ "$v" != "$raw" ]` to

    elif [ "$v" != "$raw" ] || case "$raw" in '"'*|"'"*) true;; *) false;; esac; then

keeping the `!=` leg byte-for-byte (ADR-0065 decision clause), covering single quotes as well as double (ADR-0065 requires both), and reusing the existing merged diagnostic string unchanged ("… is not a bare scalar — the reader consumes only '$v'; write model/effort values unquoted and space-free"). No separate quote-specific message: the reference implementation merges both legs into one diagnostic and the remedy text is identical for both.

The leg judges lexical shape only — `anthropic/claude-opus-5`, `openai:gpt-5.6-sol`, and every other bare provider-shaped ID pass unexamined (ADR-0015, no vendor allowlist).

No shared helper is extracted. `harness-defaults.sh`'s header explicitly forbids coupling the shipped-data reader to the user-config readers, and reader consolidation is its own change (out of scope here).

### 2. The `#`-strip corner: out of contract, and validated

Settled posture (default carried from #0180): a `#` inside an `agents:` entry's flow map is **out of contract** — the strip order does not change. Re-ordering the strip so readers see the raw line would break legitimate trailing comments (`status: { model: x, effort: low }  # comment`, and full-line comments) across every config layer, for a corner no real model ID exercises.

"Out of contract" is enforced, not just stated: both validators gain a `#` leg that inspects the **pre-strip** entry line. Precise rule (one formulation, no equivalents): the leg applies **only when the entry's first `{` precedes any `#` on the line** (a `#` before the first `{`, or no `{` at all, never fires). When it applies, take the substring after that first `{`; the leg **fires** iff the substring contains a `#` before its first `}`, **or** contains a `#` and no `}` at all (a commented-away closing brace is the same truncation). Consequences: a trailing comment after `}` stays legal; a full-line comment never reaches the entry matcher; a commented-out map (`status: # {model: c#5}` — including the natural workaround for this very gate) does **not** fire even when the comment itself contains `#` after a `{`. Post-strip such an entry is field-absent: legal in user config (all fields optional); in the shipped sidecar it is already a missing-field error under `hd_validate`'s existing non-empty leg, which is the correct diagnosis there — no truncation is possible.

This leg is a new mini-rule motivated by #0180's "same defect wearing different clothes" (silent truncation), not by ADR-0065's consequence clause, which mandates only the quote leg. Cost, stated as ADR-0065 stated it for the quote leg: the validator now hard-aborts generation on input the previous code silently truncated — a `#` inside a flow map in **any** layer, including the machine-wide global config read by every repo and by `install.sh`, previously generated (with a clipped value) and now refuses loudly before any wrapper is written. That is the intended trade.

- `sync-agents.sh` / `validate_user_agent_values`: `harness_agent_line` grows an optional 5th arg (`keep_comments=1`) — or a thin companion wrapper — returning the entry line without the strip, honored on both the bash-3.2 sed path and the bash-4 cache path so the two paths cannot disagree. The validator fetches the pre-strip line once per entry and applies the `#` check before the per-field loop. Diagnostic goes through `log` like the sibling diagnostics; distinct wording, because the unquoted remedy does not apply — e.g. `"$h/$a entry contains '#' inside the flow map — comments cannot appear inside {…}; docket strips them before parsing ($f)"`.
- `harness-defaults.sh` / `hd_validate`: same leg via a pre-strip view of the entry line (`_hd_entry_line` companion or raw-mode arg over an unstripped `_hd_block` variant). The sidecar is docket-shipped, but hd_validate exists precisely to stop malformed shipped data before any wrapper is written, `_hd_block` strips comments identically so the corner exists there too, and the mirror-equality test does not reliably catch a truncating `#` typo (see Assumption 3).

Wrapper emission continues to read the stripped line — the corner is rejected at validation, never silently emitted.

### 3. Documentation at the point of use

One consistent sentence, reusing the diagnostic's wording, added as a comment line at each point of use:

> Write model/effort values unquoted and space-free; `#` cannot appear inside the `{…}` flow map.

Targets:
- `README.md` global-config example (`agents:` block, ~line 395-397).
- `README.md` `.docket.local.yml` example (`agents:` block, ~line 423-425).
- `skills/docket-convention/SKILL.md` `.docket.yml` schema block (line 44's `agents:` comment gains the clause, kept to one line).
- `skills/docket-convention/references/agent-layer.md` — the example `agents:` block (~lines 30-42); this is the blocking read the convention mandates before configuring `agents:`, so the rule must appear here, not only in SKILL.md's one-liner.
- `.docket.example.yml` — one line in the `agents:` intro comment (~lines 330-352); warranted, since this is the canonical copy-from example and its block is entirely example values users uncomment and edit.

No change to the gate's posture or diagnostic wording (0181 out-of-scope carried over; the existing string stays byte-identical so the two validators keep emitting the same sentence).

### 4. Tests (value-level, per #0180's warning that "generation succeeded" passes against the bug)

- `tests/test_harness_defaults_validator.sh`: fire probes — double-quoted space-free value, single-quoted space-free value, `#` inside the flow map; ignore probes — bare provider-prefixed ID (`anthropic/claude-opus-5`) passes, trailing `# comment` after `}` passes. Assert non-zero exit and the diagnostic's remedy text, mirroring the 0173 probe style in `tests/test_sync_agents_validator.sh`.
- `tests/test_sync_agents_validator.sh`: add the `#` probes (fire: `{model: c#5}` aborts before any wrapper is written, diagnostic names the flow map; ignore: trailing comment after `}` still generates). The quote probes for the user-config validator already exist (0173) and are untouched.

## Out of scope (restated from the stub)

- Consolidating the readers (the config-reader consolidation change).
- Quoting support / widening legal values — ADR-0065 chose validation, not tolerance.
- Any vendor model allowlist (ADR-0015).

## Assumptions

1. **Quote-leg shape in `hd_validate`: inline copy of the sync-agents condition, merged into the existing diagnostic.** Rejected: a separate quote-specific diagnostic (the reference implementation merges both legs and the remedy text is identical); a shared extracted helper (the file header forbids coupling shipped-data readers to user-config code, and consolidation is another change's scope).
2. **`#` corner: keep the strip order, declare in-flow-map `#` out of contract, and add a validating leg on the pre-strip line.** Rejected: re-ordering the strip so readers see raw lines (breaks legitimate trailing/full-line comments everywhere for a corner no real ID exercises — and the stub's own default is against it); documenting without validating (the stub's default per #0180 is "state … and validate it", and silent truncation is the defect class this family exists to close).
3. **The `#` leg lands in BOTH validators, not just sync-agents'.** Rejected: sync-agents-only (the shipped sidecar is docket-controlled, but hd_validate's whole purpose is pre-write validation of shipped data, `_hd_block` strips comments identically so the corner exists there too, and the mirror-equality test parses through the same stripping readers so it is not a reliable catch for a truncating typo). Authority is #0180's same-defect-class argument, not ADR-0065, whose consequence clause mandates only the quote leg. Cost is one small pre-strip view helper per file; feasible on both sync-agents code paths because the bash-4 layer-body cache retains comments (`section_body` strips only for boundary logic, printing raw `$0`).
4. **`#` rule scope, exact formulation (per §2): the leg applies only when the entry's first `{` precedes any `#` on the line; it then fires iff, after that `{`, a `#` appears before the first `}` or a `#` appears with no `}` at all. Trailing comments after `}`, full-line comments, and commented-out maps (any `#` before the first `{`) remain legal.** The applies-only-when clause is the critic's prescribed remedy from the re-check round, adopted verbatim. Rejected: banning `#` anywhere on an entry line (gratuitously breaks the documented, legitimate trailing-comment style used throughout .docket.example.yml and agent-layer.md's own example); firing on commented-out maps (the natural workaround for this gate; field-absent post-strip — legal in user config, and in the sidecar already correctly diagnosed by the existing missing-field leg, not a truncation).
5. **Docs get one identical sentence at five points of use (README ×2, SKILL.md schema line, agent-layer.md example, .docket.example.yml), reusing the diagnostic's own wording.** Rejected: a single central statement with pointers (the whole finding is that the rule is absent where users actually look — the examples they copy); divergent per-file phrasings (one sentence keeps the docs and the diagnostic mutually recognizable). `.docket.example.yml` inclusion decided yes — its `agents:` sketch is the canonical copy-from block, so it "warrants it" under the stub's conditional.
6. **The `#` diagnostic uses distinct wording, routed through the existing `log` path.** Rejected: reusing the "unquoted and space-free" string (its remedy does not describe the `#` problem, and 0173's split-diagnostic precedent — the present-but-empty case at `sync-agents.sh:604-605` — exists precisely so a diagnostic never blames the wrong cause).
7. **Existing diagnostic strings stay byte-identical** in both files, so tests pinning the wording (0173's) and the two validators' shared sentence stay in lockstep. Rejected: rewording to mention `#` in the same string (posture/wording change was 0181's explicit out-of-scope, carried into 0255).
8. **Coupling: `related: [256]`, `depends_on:` stays empty.** Active change 0256 (config-reader consolidation — one extractor or a recorded ADR) targets these exact readers/validators; this design's no-helper-extraction choice defers extraction **to** 0256, and the two new leg sites this change adds are sites 0256 must consolidate or keep byte-identical. That is a real design-settled coupling, written into the stub's `related:` frontmatter (forward link only), not merely prose. No build-order dependency in either direction, so `depends_on:` stays empty.
9. **The `#` "out of contract" ruling is recorded in this spec, the point-of-use docs, and code comments at the strip sites — no ADR update.** #0180 asked to "decide and record"; this spec plus the documented rule at the five points of use is the record. Rejected: a new ADR or a dated update to ADR-0065 (the leg neither reverses nor supersedes ADR-0065 — it extends the same silent-truncation-becomes-loud-abort posture to a lexical corner, below ADR altitude; if the builder's whole-branch review disagrees, docket-adr remains invocable at build time per the implement-next flow).
