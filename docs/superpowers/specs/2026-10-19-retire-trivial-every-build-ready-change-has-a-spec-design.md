<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0573 — Retire trivial: every build-ready change has a spec](../../changes/archive/2026-10-20-0573-retire-trivial-every-build-ready-change-has-a-spec.md)**
<!-- docket:backlink:end -->

# Every build-ready change has a spec — retire `trivial`

## Why

Grooming can end in a *trivial verdict*: no spec file, `trivial: true` on the record, and the
change body's owned sections (`## Why`, `## What changes`, `## Out of scope`) carry whatever the
groom decided. In practice this exit loses work. Grooming only offers it after a design
conversation has already happened — research done, alternatives weighed, decisions made — and the
trivial exit then asks the agent to squeeze that into "the tightened body as the trivial
rationale." Nothing requires the decisions or the rejected alternatives to survive, there is no
heading for them, and the result never reaches the PR or `main` (a spec is committed as the
feature branch's first commit; a trivial body stays on the metadata branch only).

The exit also saves almost nothing. The flag changes only build-readiness and whether a spec file
exists: implement-next still reconciles, writes a plan, builds, reviews, and writes a results file
for a trivial change. The one thing skipped is writing the design down.

History matches: every trivial verdict since about #325 (14 changes) came out of a groom; the
capture-time trivial path in `docket-new-change` was last used around #358. The human always
declines the trivial suggestion now, because the research has nowhere reliable to go.

The suggestion itself mostly comes from the brainstorm role: `superpowers:brainstorming` sizes
small work as "described in chat, no written design." Removing the exit without countering that
sizing would leave the agent still proposing to skip the spec, with no place to record the choice.

## Goal

One rule: **a change is build-ready only when it has a spec** (plus the unchanged conditions:
`proposed`, dependencies `done`, stack base resolved, and the hold rule). Every groom that makes a
change build-ready writes a spec, sized to the change — a rename gets a short one.

## Non-goals

- Rewriting archived records or other repositories' records. Docket is pre-1.0.0; an unbuilt
  `trivial: true` change in another repository simply drops back to needs-grooming.
- The broader trimming of superpowers brainstorming steps docket does not use (builder check,
  visual companion, spike offers, its own spec location) — that is #566. This change counters
  only the "no written design" sizing outcome, because it is the trivial suggestion's source.
- Changing what a spec must contain, how the plan-writer reads it, or the reconcile pass.

## Anti-goals

- A groom that settles decisions but leaves no written place for them.
- A new check, finding, warning, or gate about legacy `trivial:` lines. They are inert data.
- Rewriting history: archived records keep their `trivial: true` lines byte-for-byte.

## Decisions

### 1. Readiness and the domain model

`trivial` leaves the domain model entirely. Build-ready = `proposed` + non-empty `spec:` + the
existing dependency, stack-base, and hold conditions. Needs-grooming = `proposed` with no spec.
Auto-groomable = needs-grooming + `auto_groomable: true`.

Remove `Trivial` from the change entity, the readiness evaluation, the claim/action facts that
copy it, and the decoder's wire struct. The decoder already treats unknown frontmatter keys as
compatibility data ("Unknown keys are compatibility data and stay in the document; they are not
decoded here and never fail the decode"), so a legacy `trivial: true` line stays in the file,
untouched by every writer, and means nothing. A legacy record with `trivial: true` and no spec
reads as needs-grooming.

### 2. New records

`change.create` stops writing a `trivial:` line into the frontmatter template.

### 3. The groom operation

`change.groom` drops the `trivial` outcome. Remaining outcomes: `spec`, `revise`, `abstain`,
`re-enable`. A request naming `outcome: trivial` refuses through the existing invalid-outcome
finding and writes nothing — no special "retired" message. Remove what only the trivial outcome
used: the missing-rationale finding code and its `hasAuthoredRationale` helper, the
`groomed (trivial)` receipt text, the `trivial` field edit, and the trivial checks in the
revise/stub-edit gates (a stub edit now refuses only when the change has a spec). The schema's
`groom_outcomes` vocabulary and the `change groom` CLI help drop `trivial`.

### 4. The board

Every build-ready or held proposed change now has a spec, so it renders in the Groomed section.
Drop the `build-ready (trivial)` cell and the comments and branches that existed only for
trivial-without-spec rows in the Proposed section.

### 5. `docket open`

The `trivial-no-spec` reason goes. Asking for the spec of a change with no spec — a stub, or a
legacy trivial record — gets the existing `artifact-unset` answer ("change N has no spec yet").

### 6. Spec always — the brainstorm direction

`docket-groom-next` and `docket-new-change` tell the brainstorm role, at the point they invoke it,
that its artifact is always a written spec, sized to the change: a small change gets a short spec,
and the brainstorm never concludes that a change needs no written design. Phrase it by shape (what
outcome to avoid), never by citing a vendored superpowers heading. Keep it to the minimum prose that
does the job; these skills are read on every groom.

`docket-groom-next`: delete the trivial exit (Step 4 becomes five exits) and every "or trivial" /
"not trivial" / "trivial rationale" clause in selection, the revise route, retitle, reprioritize,
deliverable, hold, and stacking paragraphs.

`docket-new-change`: delete the *Trivial path* section and its *When to use* bullet. Small work is
captured through brainstorm mode (with a short spec) or as a stub groomed later.

### 7. Auto-groom

Autonomous grooming has two exits: **spec** and **abstain**. The designer pass drops the
"genuinely mechanical → trivial verdict" sentence; a mechanical stub gets a short spec with its
`## Assumptions` block like any other. The critic attacks a spec, never a verdict. Update the
drain summary (`groomed N, abstained K`), the termination paragraph, the hold paragraph, the skill
and agent descriptions, and the critic agent's description.

### 8. ADR

Record a new ADR that supersedes ADR-0006 (through `docket-adr` during the build): every
build-ready change carries a spec; autonomous grooming has two exits (spec, abstain); ADR-0006's two
bounds (an adversarial critic gates every build-ready exit; kill and defer are never autonomous)
carry forward unchanged. ADR-0153 (hold) already narrows ADR-0006's "build-ready means the
autonomous builder will build it" for held changes; the new ADR keeps that hold exception and
relates to ADR-0153, so the supersession does not silently restore the old rule (my call). The new
ADR's *Alternatives considered* records why the trivial exit was retired rather than repaired
(requiring decisions in the trivial body still keeps the design off the PR and saves nothing over a
short spec). ADR-0129 mentions `trivial` only inside a historical rename table and stays as it is.

### 9. Skill, agent, and doc prose

Docs describe only current behavior: delete `trivial` mentions outright, with no "formerly" notes.
The set below comes from a repo-wide grep. Watch the spelling "non-trivial": most uses are
ordinary English, but two refer to the flag and change too (implement-next's "(non-trivial
changes)" beside the spec copy, and new-change's "The default path for any non-trivial new
change"). Frozen point-in-time snapshots stay untouched: `docs/release/v1.0.0-alpha.2/*` and
`internal/install/legacydata/*`.

- `skills/docket-convention/SKILL.md` — manifest's `trivial:` line, build-readiness,
  needs-grooming, auto-groomable, the hold sentence, and "trivial included" in the results lines.
- `skills/docket-groom-next/SKILL.md`, `skills/docket-new-change/SKILL.md`,
  `skills/docket-auto-groom/SKILL.md` — as in decisions 6 and 7.
- `skills/docket-implement-next/SKILL.md` — reconcile's "A trivial change has no spec — patch the
  body only", commit-spec's "(a trivial change reports `no-spec`)", the "trivial included"
  results line, and the `reconciled` paragraph's readiness restatement.
- `skills/docket-implement-next/references/results-artifact.md`,
  `skills/docket-implement-next/results-template.md`, `skills/docket-review/SKILL.md`,
  `skills/docket-status/SKILL.md` — "or a trivial change's body", "trivial included", and the
  board-cell list.
- `agents/docket-auto-groom.md`, `agents/docket-auto-groom-critic.md` — descriptions; then
  regenerate the harness goldens that embed them (`internal/harness/*/testdata/golden/docket-auto-groom*`).
- `cursor-rules/dispatch/docket-auto-groom-critic.md` — hand-maintained; "draft spec / trivial
  verdict" appears twice.
- `README.md`, `docs/guide/capturing-work.md`, `docs/guide/designing-before-building.md`,
  `docs/guide/building-without-supervision.md`, `docs/concepts/change-lifecycle.md`,
  `docs/reference/glossary.md` (including the `### Trivial` entry and the "Capture modes" entry),
  `docs/install/models-and-effort.md` ("a trivial verdict"), and
  `docs/comparison/ai-native-sdlc-playbook.md` (the "Trivial path" row, "or `trivial: true`", and
  the capture-modes list).

"Required for every change, trivial included" becomes "required for every implemented change."
Regenerate embedded assets after skill edits (`go generate ./internal/assets/`).

### 10. Tests

- About 40 test setups use `trivial: true` as a cheap way to make a change build-ready. Give them
  a spec instead (a minimal spec file plus `spec:`), through a shared fixture helper where one
  fits.
- Delete tests that exist only for the trivial outcome, the trivial board cell, or the
  `trivial-no-spec` open reason.
- Update pinned prose and budgets that move: `internal/repoguard/prose_contracts_test.go` (the
  "trivial included" phrases) and `internal/repoguard/budgets_test.go` (skill line/byte budgets).
- New coverage: `outcome: trivial` refuses as an invalid outcome and writes nothing; a record
  carrying `trivial: true` and no spec reads as needs-grooming, and a groom of it keeps the
  `trivial: true` line byte-identical; a newly created record has no `trivial:` key.
- The status-corpus testdata records keep their `trivial: true` lines; they now double as proof
  that the key is inert.

### 11. `workspace.commit-spec` keeps `no-spec` (my call)

Once readiness requires a spec, a change claimed after the upgrade always has one. A change claimed
*before* it with `trivial: true` and no spec could still be in progress when the binary is
swapped, so `no-spec` stays as the quiet no-op it is today and that build finishes. Only its
"trivial" comment and implement-next's "(a trivial change reports `no-spec`)" clause change.

## Left to the builder

- The fixture helper's shape and where it lives.
- Exact wording of the brainstorm direction, within decision 6's constraints.
