<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0560 — Put a groomed change on hold so loops skip it](../../changes/active/0560-mark-a-build-ready-change-as-off-limits-to-the-autonomous-lo.md)**
<!-- docket:backlink:end -->

# Put a groomed change on hold so loops skip it — design

Change #560, groomed 2026-10-18.

## Why

A groomed change is build-ready: it has a spec or `trivial: true`, and its dependencies are done. Every build-ready change looks alike to an autonomous loop draining the backlog. That is wrong for two kinds of change:

- A change that needs a person for some step: signing in somewhere, a manual check, a decision only the person can make. A loop claims it, works until it reaches that step, and halts. A halt stops the loop, so one such change ends the whole drain.
- A change the person has groomed but does not want built yet, for any reason.

Today the only ways to keep a groomed change away from the loop are to leave it ungroomed on purpose (#547 does this) or to defer it. Deferring shelves the change: grooming can no longer revise it, and even a run that names its id refuses it until someone revives it. #559 settled for a stop-gap: whoever starts a loop lists ids and checks that none of them needs a human.

## Goals

- A person can put a **hold** on a change. While the hold is on, no autonomous build selection picks the change: neither a drain over the whole backlog nor a drain over a list of ids, even when the list names it.
- A run that names exactly that one change builds it. Naming a single change is the deliberate act that overrides the hold.
- The hold is generic. It records no required reason. An optional reason can be added as prose when the hold is set.
- Grooming can set and lift the hold from day one, and so can capturing a new change.
- When grooming settles on a spec that needs manual work, docket suggests a hold. The auto-groomer, which has no one to ask, sets the hold itself.
- The board keeps a held change that has a spec in the Groomed table, marked `on hold`. Every groomed row states its readiness.

## Non-goals

- **Grooming is unaffected.** A hold does not keep a stub out of `docket-groom-next`'s automatic pick or `docket-auto-groom`'s queue. Grooming already has its own controls: `auto_groomable` opts a stub in or out of autonomous grooming, and groom-next runs with the person present. A hold on a stub is allowed. It takes effect once the stub is groomed.
- **Finalize is unaffected.** A held change that a named run built lands like any other.
- **Carrying on past a halt is a separate change** (#570). Every `halted` disposition still stops the driver.
- **No change to how a run behaves at a human step.** #559 settled that: the run does the work it can and halts at the step.
- The reason is not shown on the board.

## Design

### 1. The field

A new optional frontmatter field, `hold`. `hold: true` puts the change on hold; absent, empty, or `false` means no hold.

- **Writer.** `render.NewChangeRecord` gains the field, written directly after `auto_groomable` in the manifest order. Update the record golden file.
- **Decode.** Decode it with the same `optionalBool` path as `auto_groomable`, so a bad value raises the existing malformed-field warning, never an error. One difference, on purpose: a malformed `hold` value **reads as held**. The field exists to keep work away from the loop, so a garbled hand edit must fail safe (the change waits) rather than open (the loop builds it). The accessor is true when the value is `true` or malformed. Update the `optionalBool` comment, which says `auto_groomable` is the only optional boolean.
- The field is human input. Two agent writes are allowed: the auto-groomer setting it (section 5), and any operation that carries the record forward unchanged.

### 2. What a hold does

The hold is a **selection-only exclusion**. It must not make the change "not ready" in the sense the claim checks, or the named single-id run would be refused too.

- **Readiness.** `domain.EvaluateReadiness` gains one last step. A change that would otherwise be `build-ready` and is held reads as a new readiness kind, `on-hold`. Every earlier outcome wins: a held stub still reads `needs-grooming` (or `auto-groom-blocked`), a held change with an unmet dependency still reads `waiting-dependency`, and a held change with an unresolved stack base still reads `stack-base-unresolved`. Add the kind to `domain.ReadinessKind`'s closed set and to the function's documented check order.
- **Selection.** `domain.SelectQueue` is unchanged: it keeps only `build-ready`, so a held change drops out of
  - the `status` operation's top-level `ready` array,
  - `context.implementation` with no `--id`, and
  - implement-next's id-allowlist drain, which filters that same `ready` array in skill prose.
- **Claim eligibility.** `domain.ClaimEligibility` treats `on-hold` as eligible, exactly like `build-ready`. So `context.implementation --id`, `change.claim`, and `change.complete` on a `proposed` results-artifact change all accept a held change. The claim leaves the field in place. A change reclaimed back to `proposed` is still held, which is what the person asked for.
- **Status rows.** A held row reports `readiness: on-hold`, `ready: false`, and a `readiness_reason` of "on hold" (`app.readinessReason`).
- **Board.** A held change that has a spec stays in the Groomed table, marked `on hold`. Unlike a change waiting on a dependency, it can still be built by naming it, so it belongs with the groomed changes.
  - `render.boardClassify` puts a spec-backed `proposed` change in Groomed when its readiness is `build-ready` or `on-hold`.
  - The Groomed table gains a last column, Readiness. Every row states its readiness: exactly `build-ready` or exactly `on hold`. No row is left blank, because a blank cell beside rows that carry a status would leave the reader to guess a default. `on hold` names the state the change is in, not the action of holding it.
  - A held trivial change has no spec, so it stays in the Proposed table as every trivial change does today. `render.boardReadinessCell` renders its cell as `on hold` instead of `build-ready (trivial)`.
  - Update the column layouts in the `render` package comment, the board golden file, and the golden's `PROVENANCE.md`.
- **In progress table (sibling fix).** Its Readiness column shows the run's state (`run halted — needs you`), not readiness, so rename it State, as the Built table's column is named. The cell contents do not change. Rename `render.boardInProgressReadinessCell` to `boardInProgressStateCell`, matching `boardBuiltStateCell`, and update `boardSectionTableHeader`, the package comment, the header assertion in `board_test.go`, the board golden file, and `PROVENANCE.md`.
- **Everything else** that keys on readiness: dependents of a held change wait until it is done, and a change stacked on a held parent waits for the stack base as it would for any unbuilt parent. Neither needs code. Grep every switch over `ReadinessKind` (today `app.readinessReason` and `render.boardReadinessCell`, plus the equality checks in `SelectQueue`, `ClaimEligibility`, and `boardClassify`), and confirm each one handles `on-hold` deliberately.

### 3. Setting and lifting a hold

**`change.groom`** gains two request keys:

- `hold` (bool). Omitted keeps the current value, `true` sets the hold, and `false` lifts it. It is accepted on the `spec`, `trivial`, `revise`, and `re-enable` outcomes, like `deliverable`. A `hold` alone is a valid revise, so it counts toward the empty-revise check. The `abstain` outcome refuses it with a new `invalid-hold` code, as it refuses `deliverable`.
- `hold_reason` (string, optional). It is accepted only together with `hold: true` in the same request. Otherwise the request is refused with a new `hold-reason-without-hold` code and nothing is written.

The reason lives in an operation-owned body section, `## Why on hold`, modelled on `## Why deferred`:

- `hold: true` with `hold_reason` writes the section, replacing any existing one.
- `hold: true` without `hold_reason` leaves any existing section alone.
- `hold: false` removes the section, so a stale reason never outlives the hold. Git history keeps it.
- The hold keys are the section's only writer. Do not add it to `render.ChangeOwnedHeadings`. A `sections` edit naming `## Why on hold` is refused with `invalid-section-heading` on **every** groom outcome, and the same applies to a `change.reconcile` edit. Today `## Auto-groom blocked` is refused only on revise and re-enable, so this is deliberately wider.
- `hold_reason` gets the same shape checks as `blocked_note`: no column-zero `## ` heading outside fenced code, and no unterminated fence.
- Place the section where `## Why deferred` goes.

**`change.create`** gains the same `hold` and `hold_reason` keys with the same rules, so a person can capture a change already on hold.

A hold is set only on a `proposed` change. Revise already refuses other statuses (`not-revisable`), and a deferred change is revived first. Setting or lifting a hold never changes the groom state. As every groom write does, it re-renders the board and the `## Artifacts` block in the same commit. Register both keys in the `docket schema` descriptors, register the new codes in `AllFindingCodes`, and update `internal/app/schema_test.go`.

### 4. implement-next

No reference file is needed. The skip happens in the binary, so the skill only learns the new word:

- Step 1's readiness vocabulary list adds `on-hold`. The list also omits `stack-base-unresolved` and `invalid` today, so add those while editing the line.
- The id-allowlist paragraph and the final report's skip list name "on hold" as a skip reason.
- The named-invocation paragraph says a held change is built when named.
- A hold can be set between the `status` read and the `context.implementation --id` read. On the selection path (no argument, or an id set), a bundle reporting `on-hold` is dropped and reported, like any other bundle that disagrees with the digest. A hold set after that read is already caught: the claim's revision check returns `contended`.

The terminal dispositions and the driver rule are unchanged.

### 5. Suggesting a hold during grooming

- **`docket-groom-next`** (Step 3 or 4): when the settled design needs manual work, meaning a step a person must carry out, suggest a hold before the spec or trivial exit. The person decides. If they agree, send `hold: true` and an optional `hold_reason` in the same `change.groom` request. Step 4 gets a short **Hold** paragraph beside *Deliverable, type, and branch prefix*, covering setting and lifting on the spec, trivial, revise, and re-enable exits.
- **`docket-new-change`**: the same suggestion at its brainstorm and trivial exits. The hold rides in step 5's `change.groom` request, or in `change.create` for a captured stub.
- **`docket-auto-groom`**: when the spec or trivial verdict it emits names a step a person must carry out, send `hold: true` in its groom request, and say so in its run report. It adds a one-line `hold_reason` only when the stub has no `## Why on hold` section yet, so it never overwrites a person's reason. It never sends `hold: false`: lifting a hold is a person's call. The critic is unchanged.
- **`docket-convention`**:
  - the manifest template gains `hold:` with a one-line comment;
  - *Build-readiness & selection* says a held build-ready change is never selected, and that naming it builds it;
  - *Change body sections* lists `## Why on hold`;
  - *Autonomous grooming* notes the auto-groomer's hold write.

Keep each addition short. Agent-read skill prose is paid on every run. Regenerate the embedded skill copies (`go generate ./internal/assets/`) and adjust `internal/repoguard/budgets_test.go` if a budget moves.

### 6. Decision record

Record an ADR: **a hold keeps a groomed change out of autonomous build selection, and naming the change overrides it.** It should state:

- **Context.** ADR-0006's equivalence, that build-ready means the autonomous builder will build it, no longer holds for a held change.
- **Decision.** The hold is a selection-only exclusion, never a claim refusal. Any single-id act (a named run, a claim, completing the change) overrides it. It does not touch grooming or finalize. The auto-groomer sets it for specs that need manual work. A malformed value reads as held.
- **Alternatives rejected:**
  - Deferring: it blocks named runs and grooming revisions.
  - A readiness gate on the claim: it would refuse the named run.
  - Letting the hold also gate grooming picks: more places change, and it duplicates `auto_groomable`.
  - Carrying on past halts instead: that is #570, and it needs a typed halt reason.

### 7. Docs

Describe current behavior only, with no change citations:

- `docs/guide/building-without-supervision.md`: the drain section says a held change is skipped and is built when named.
- `docs/guide/capturing-work.md`: grooming can put a change on hold.
- `docs/reference/glossary.md`: a **Hold** entry.
- `docs/reference/cli.md`: the new `change groom` and `change create` request keys.
- Any board doc that lists Readiness cell values.

Grep for other pages that enumerate readiness values or groom request keys.

## Tests

Mutation-test each new guard: strip the behavior and watch the test go red.

1. **Readiness.**
   - A held build-ready change reads `on-hold`.
   - A held stub reads `needs-grooming`, and a held stub with `## Auto-groom blocked` reads `auto-groom-blocked`.
   - A held change with an unmet dependency reads `waiting-dependency`.
   - A held change with an unresolved stack base reads `stack-base-unresolved`.
   - `hold: false` and an absent field read `build-ready`.
   - A malformed value reads `on-hold` and raises the malformed-field warning.
2. **Selection and claim.**
   - `SelectQueue` excludes a held change.
   - `ClaimEligibility` accepts `on-hold`.
   - `status --json` leaves it out of `ready` and reports the row as `on-hold`, "on hold", `ready: false`.
   - `context.implementation` with no id skips it: it picks the next ready change, or returns `no-candidate` when the held change is the only one.
   - `context.implementation --id` and `change.claim` accept it, and the claim keeps the field.
   - `change.complete` accepts a held `proposed` results-artifact change.
3. **Board.**
   - A held spec-backed change renders in the Groomed table with Readiness `on hold`.
   - An unheld spec-backed build-ready change renders in the Groomed table with Readiness `build-ready`; no Groomed row has an empty Readiness cell.
   - A held trivial change renders in the Proposed table with `on hold`.
   - The In progress table's header names the last column State, and a halted row still reads `run halted — needs you`.
   - No record fault.
4. **`change.groom`.**
   - Set with and without a reason. Replace a reason. Lift, which removes the section.
   - An omitted `hold` keeps both the field and the section.
   - A `hold` alone is a valid revise.
   - Each accepting outcome (spec, trivial, revise, re-enable) writes the field.
   - Refusals that write nothing:
     - abstain with `hold` (`invalid-hold`);
     - `hold_reason` without `hold: true` (`hold-reason-without-hold`);
     - a `sections` edit naming `## Why on hold` (`invalid-section-heading`);
     - a `hold_reason` carrying a column-zero heading or an unterminated fence.
5. **`change.create`.** Hold with and without a reason; reason-without-hold refuses.
6. **Writer.** The record golden file shows `hold` directly after `auto_groomable`.
7. **Schema.** `docket schema` lists both keys on both operations, and the new codes are registered.

## Left to the builder

- The exact spellings of the readiness token (`on-hold` suggested) and the two finding codes.
- How a malformed value surfaces in `status` beyond the existing warning.
- The exact wording of the skill and doc additions, within the limits above.
