<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0168 — Cursor support for profile-routed Docket builds](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0168-cursor-profile-routed-build-support.md)**
<!-- docket:backlink:end -->

# Cursor support for profile-routed Docket builds

Design for making Docket's lean, profile-routed build native on Cursor while moving every shipped
agent model/effort default into one harness-indexed source of truth.

## Context

Change 0167 replaced Superpowers SDD's repeated implementer/reviewer topology with a Docket-owned
build controller and one fresh worker per plan task. The controller routes each task to an
`economy`, `standard`, or `premium` profile, allows one bounded escalation, and runs the full suite
once before Docket's independent whole-branch review.

The three worker sources currently carry Claude-specific `model:` and `effort:` values in
`agents/docket-build-*.md`. `sync-agents.sh` translates those sources into every configured
harness. Cursor's emitter correctly converts its target wrapper shape, but without a
harness-specific override it still receives the Claude source value. That is not Cursor support:
an incompatible ID can silently select a fallback model while the generated wrapper appears
pinned.

The same structural problem applies beyond the three workers. All twelve source wrappers use their
frontmatter as both a behavioral template and the Claude default store. Adding a Cursor-only
exception table while leaving Claude defaults embedded would create two default mechanisms and
make the next harness repeat the split. This design instead establishes one canonical shipped
default layer now: complete for Claude, deliberately sparse for Cursor, and ready for Codex change
0169 to extend without another resolver redesign.

Change 0135 and ADR-0060 already established Cursor-native wrapper translation: Cursor receives
only its documented agent fields, effort is embedded in the model value when separate effort is
configured, and linked skills are delivered through the wrapper body. This change does not reopen
that contract. It supplies honest native defaults and certifies the existing dispatch path through
the Cursor IDE.

## Goals

- Make the lean `docket-build` role usable on Cursor without requiring users to copy model
  overrides into configuration.
- Ship three Cursor-native build-profile defaults with an intentional cost/capability ladder.
- Move all shipped Claude model/effort defaults out of the wrapper templates into the same
  harness-indexed source.
- Preserve every current Claude generated value exactly.
- Keep local, committed, and global user overrides field-by-field and higher precedence than
  shipped defaults.
- Leave unsupported harness/agent combinations honestly unpinned instead of inheriting a model ID
  from another harness.
- Preserve runner-delegation semantics: native parent defaults must never leak into child-runner
  model flags.
- Certify explicit routing, automatic routing, and one bounded escalation in the Cursor IDE.
- Make the README's skill catalog complete and count-free.

## Non-goals

- Shipping defaults for the other nine Cursor agents.
- Shipping any Codex defaults; change 0169 owns that block.
- Adding a runtime vendor model registry, allowlist, or availability lookup.
- Changing `docket-build`'s routing rubric, escalation graph, task outcome protocol, TDD contract,
  or full-suite gate.
- Changing the shared `docket-build-task` worker contract beyond harness-neutral wording where
  necessary.
- Making `cursor-agent` a certification surface.
- Replacing Docket's whole-branch review skill.

## Shipped harness defaults

Add `agents/harness-defaults.yml`:

```yaml
agents:
  claude:
    adr:                   { model: claude-opus-5, effort: low }
    auto-groom:            { model: claude-opus-5, effort: low }
    auto-groom-critic:     { model: claude-opus-5, effort: medium }
    brainstorm-consultant: { model: claude-opus-5, effort: medium }
    build-economy:         { model: claude-opus-5, effort: low }
    build-standard:        { model: claude-opus-5, effort: medium }
    build-premium:         { model: claude-opus-5, effort: high }
    finalize-change:       { model: claude-opus-5, effort: low }
    implement-next:        { model: claude-opus-5, effort: medium }
    integration-repair:    { model: claude-opus-5, effort: medium }
    rebase-resolver:       { model: claude-opus-5, effort: medium }
    status:                { model: claude-haiku-4-5-20251001, effort: medium }

  cursor:
    build-economy:         { model: cursor-grok-4.5-medium, effort: auto }
    build-standard:        { model: cursor-grok-4.5-high, effort: auto }
    build-premium:         { model: claude-opus-5-high, effort: auto }
```

The file is shipped program data, not a fourth user configuration file. It contains native defaults
only:

- Every entry is nested under a concrete harness. A harness-neutral `default:` block is forbidden;
  it would recreate the cross-harness leakage this change removes.
- Keys use the wrapper short name (`build-economy`, not `docket-build-economy`).
- A listed entry supplies both `model` and `effort`. The table is sparse by harness and agent, not
  by field.
- `runner` is forbidden. Delegation is user policy, never a shipped agent default.
- Model IDs and effort tokens remain opaque passthrough values under ADR-0015. Structural
  validation is not a vendor allowlist.

The Claude block is complete: its agent-key set exactly equals the set derived from
`agents/docket-*.md`. The Cursor block is intentionally partial: in this change its agent-key set
exactly equals the build-profile sources derived from `agents/docket-build-*.md`. Codex has no
block until change 0169.

## Wrapper sources become behavior-only templates

Remove `model:` and `effort:` from all twelve `agents/docket-*.md` sources. They retain:

- the agent name and description;
- any linked skill list;
- the behavioral instructions and autonomy boundary.

The source files no longer double as Claude configuration. `sync-agents.sh` must therefore render
native frontmatter fields from resolved values rather than depending on a line already existing in
the source. The Claude, Cursor, and Codex emitters all consume the same resolved native
model/effort pair:

- Claude emits `model:` and `effort:` when resolved.
- Cursor emits a bare `model:` when effort is `auto`; otherwise it emits
  `<model>[effort=<effort>]`.
- Codex emits `model` and `model_reasoning_effort` when resolved.
- Any harness/agent pair with no shipped or user value omits the corresponding field and inherits
  the harness's own default.

This keeps each per-harness emitter a pure target-contract translation under ADR-0060. No emitter
knows which concrete models Docket recommends.

## Resolution and provenance

Resolution remains field-by-field. For a target harness and agent, the order is:

1. machine-local `agents.<harness>.<agent>`;
2. machine-local `agents.default.<agent>`;
3. committed `agents.<harness>.<agent>`;
4. committed `agents.default.<agent>`;
5. global `agents.<harness>.<agent>`;
6. global `agents.default.<agent>`;
7. shipped `agents.<harness>.<agent>` from `agents/harness-defaults.yml`;
8. no pin — omit the field and inherit the harness default.

The first non-empty value for each field wins. A user may therefore override only a model or only
an effort while inheriting the other shipped field. Cursor's shipped entries deliberately use
`effort: auto`, so their complete built-in IDs are emitted verbatim without an appended effort
parameter.

The resolver must retain provenance as well as value. It needs to distinguish:

- values supplied by a user layer;
- values supplied by the shipped harness layer;
- no resolved value.

That distinction preserves two existing behaviors:

1. A non-Claude harness that receives a model only through `agents.default` still gets the existing
   incompatible-model warning.
2. A shipped native default configures only a native wrapper; it is not evidence that the same ID
   belongs to a delegated child harness.

## Runner delegation boundary

Runner delegation is selected by a user-authored `runner:` field. A generated Claude shim has two
separate model concerns:

- its native Claude wrapper frontmatter, which may use the shipped Claude default;
- the `--model` / `--effort` flags sent to the child runner.

Only a value resolved from a user configuration layer may become a child-runner flag. The shipped
Claude sidecar value must never be forwarded to a Codex or Cursor runner merely because the parent
wrapper is Claude.

This preserves the current meaning of a runner-only override: if a user sets `runner: codex` but
sets no model or effort, the child harness chooses its own default. If the user explicitly supplies
a model or effort alongside the runner, that direct value continues to pass through verbatim.

Tests must cover this boundary because moving the Claude floor into the normal resolver would
otherwise make accidental cross-harness forwarding the cheapest implementation.

## Unsupported combinations

After this change:

- Native Claude wrappers remain pinned exactly as before.
- Cursor's three build workers use the shipped Cursor mappings.
- The other nine Cursor wrappers omit model/effort unless a user configures them.
- All Codex wrappers omit model/effort unless a user configures them; change 0169 will add native
  Codex defaults.

An unpinned generated wrapper is an honest supported shape: the target harness selects its current
default model. Generation retains a warning that names the harness and agent when no
harness-specific shipped or user model exists. It must not silently reintroduce a Claude source
fallback.

No runtime command queries a vendor model list. Model availability can vary by Cursor account,
plan, region, and rollout, while `cursor-agent` can lag the IDE and return false negatives. The
shipped values are certified against the Cursor IDE release evidence below; user overrides remain
the escape hatch when availability differs.

## Sidecar validation and failure posture

`agents/harness-defaults.yml` is required program data. `sync-agents.sh` fails before writing any
wrapper when the file is missing, unreadable, or structurally invalid. Validation covers:

- one top-level `agents:` block;
- known harness keys;
- known wrapper short names;
- no `default:` harness;
- no duplicate harness or agent entry;
- exactly the allowed `model` and `effort` fields;
- both fields present and non-empty for every listed entry;
- complete Claude set equality with all source wrappers;
- complete Cursor build-profile set equality with all build-worker sources.

The checks validate syntactic shape and set correspondence, not vendor vocabulary. A concrete model
string is never rejected because Docket does not recognize it.

The generator validates before any write so a malformed sidecar cannot leave a half-regenerated
agent directory. Removing one Claude entry, removing one Cursor build entry, adding a phantom
entry, or restoring a source `model:`/`effort:` line must each turn the guard red.

## Cursor profile semantics

The three Cursor defaults implement the existing Docket profiles without changing their routing
meaning:

| Profile | Cursor model ID | Separate effort | Intent |
|---|---|---|---|
| `economy` | `cursor-grok-4.5-medium` | `auto` | Cost-conscious worker for positively established, low-risk tasks. |
| `standard` | `cursor-grok-4.5-high` | `auto` | Default worker for normal feature, integration, refactor, and debugging tasks. |
| `premium` | `claude-opus-5-high` | `auto` | Strongest worker for named risk or one bounded escalation. |

`effort: auto` is essential: each value is a complete Cursor built-in ID whose variant is already
encoded. The Cursor emitter writes that ID unchanged rather than producing a second, potentially
conflicting effort suffix.

The controller still owns classification and the single escalation allowance:

```text
economy  -> standard -> halt
standard -> premium  -> halt
premium  -> halt
```

The retry consumes the task's one allowance exactly as change 0167 and ADR-0063 specify. This
change validates the Cursor dispatch, not a new graph.

## Documentation

The README's catalog heading becomes `## Skills`, with no numeric count. Its table lists every
`skills/*/SKILL.md` package, including:

- `docket-brainstorm` — optional consultant for authorship or audit of a settled brainstorm;
- `docket-build` — profile-routed build controller;
- `docket-build-task` — one-task worker contract.

A bidirectional derived guard compares the README catalog's skill names with the live skill
directories. The heading therefore never needs another count edit, and a new skill cannot silently
remain undocumented.

The lean-build documentation becomes harness-neutral:

- profile workers are named Docket workers, not Claude workers;
- Claude and Cursor both have shipped build-profile mappings;
- the exact Cursor mappings are visible;
- Codex remains user-configured until change 0169;
- Cursor certification means the IDE checklist, not the CLI.

The canonical `.docket.example.yml`, agent-layer reference, convention, controller skill, and any
other maintained source that calls `agents/docket-*.md` the built-in default store must be updated
to name `agents/harness-defaults.yml` instead. The example's Cursor block must distinguish the
three shipped build defaults from the other unvalidated illustrative overrides. A whole-repo grep
derives the live update set; archived changes, accepted historical artifacts, specs, plans, and
results retain their point-in-time statements.

## Verification

### Tier 1 — hermetic, gating

Generator and contract tests prove:

1. `agents/harness-defaults.yml` passes the structural validator.
2. Its Claude keys equal the source-wrapper set in both directions.
3. Its Cursor keys equal the build-worker set in both directions.
4. Source wrappers contain no `model:` or `effort:` frontmatter defaults.
5. Native Claude generated wrappers remain byte-equivalent in model/effort and behavior to the
   pre-change output.
6. Cursor build wrappers emit exactly the three selected IDs with no standalone effort and no
   appended effort suffix.
7. Non-build Cursor and unconfigured Codex wrappers omit model/effort and warn rather than
   inheriting Claude IDs.
8. Local, committed, and global overrides still beat the shipped layer field-by-field.
9. A runner-only override does not forward a shipped Claude model or effort to the child.
10. An explicit runner model/effort override still passes through.
11. The README skill catalog and live skill packages have bidirectional set equality.

Mutation evidence accompanies the guards: delete a shipped entry, add an unknown entry, restore a
source default, remove a README skill row, and defeat the runner-provenance split; each mutation
must make its intended assertion fail. The whole repository suite runs at the build gate.

### Tier 2 — Cursor IDE, certifying

The Cursor IDE checklist is the only live certification surface. It records the Cursor build,
account/plan context, generated wrapper model lines, and request/model indicators for:

1. an explicit `economy` task dispatched to `cursor-grok-4.5-medium`;
2. an explicit `standard` task dispatched to `cursor-grok-4.5-high`;
3. an explicit `premium` task dispatched to `claude-opus-5-high`;
4. at least one automatically classified task whose routing line and model indicator agree;
5. one task deliberately started at `standard` whose revealed named risk causes
   `NEEDS_ESCALATION`, followed by exactly one foreground premium retry.

The fixture must be harmless and disposable, but it must exercise the real generated agents and
the real `docket-build` controller rather than a mock dispatch. The controller must observe the
worker's structured outcome before continuing; a merely completed child is not success evidence.

Until all five checks pass, the results file and PR body say **Cursor IDE certification pending**.
A negative IDE result blocks the support claim and the merge. `cursor-agent` is neither required
nor accepted as a substitute.

## Architecture decision

Implementation records a new ADR:

> Shipped agent defaults live in a sparse, harness-indexed sidecar; behavioral wrapper templates
> carry no cross-harness model floor.

The ADR extends ADR-0016's harness-first, field-level resolution and relates to:

- ADR-0015 — direct opaque model IDs, no vendor allowlist;
- ADR-0060 — generated wrappers conform to the target harness contract;
- ADR-0063 — Docket-owned profile-routed build workers.

It does not supersede those decisions. Change 0169 extends the table with a Codex block rather than
creating another resolution mechanism.

## Expected outcome

A Cursor user who enables the lean Docket build receives three honest native profile pins without
copying configuration. Claude users see no generated-value change. Unsupported mappings inherit
their own harness default instead of a foreign model ID. Future harness support adds data to one
validated sidecar, while user overrides, emitters, and runner delegation retain their existing
boundaries.

The README finally exposes every shipped skill without encoding another count that will drift.
