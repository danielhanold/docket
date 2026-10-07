<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0382 — ChangeCreateRequest should accept typed auto_groomable / branch_prefix scalars](../../changes/archive/2026-09-27-0382-changecreaterequest-typed-auto-groomable-branch-prefix-scalars.md)**
<!-- docket:backlink:end -->

# Typed auto_groomable / branch_prefix at create, and typed auto-groom abstain / re-arm — design

## Goal

Every write of a change's `auto_groomable` and `branch_prefix` scalars goes through a typed,
validated, board-refreshing docket operation. The plain-git frontmatter edits in the skills go away:

1. `docket-new-change` sets both scalars after `change create`, as a second, plain-git commit.
2. `docket-auto-groom`'s abstain sets `auto_groomable: false` and appends `## Auto-groom blocked`
   with plain git.
3. A human re-arm sets `auto_groomable: true` and deletes that section by hand.

Edits 2 and 3 are real defects, not just untidy. The board renders a stub that has a
`## Auto-groom blocked` section as **auto-groom blocked — needs you** (`render/board.go`,
`domain.ReadyAutoGroomBlocked`). So a plain-git abstain or re-arm leaves `BOARD.md` stale (the
`board-stale` finding). The auto-groom skill's claim that the abstain "changes no board-visible
cell" is false. Edit 1 is harmless to the board, because nothing renders either scalar. But a
malformed `branch_prefix` is caught only at claim time (`domain.Claim` →
`invalid-branch-component`), so the failure lands in an autonomous implement-next run instead of
in front of the human who typed it.

## Current state (traced 2026-09-27)

- `ChangeCreateRequest` (`internal/app/change_create.go`) has no `auto_groomable` or
  `branch_prefix` field. `render.ChangeRecord` always writes both as `null`.
- Go does not model `auto_groomable` at all. The only reference is the renderer's `null`, the
  decoder ignores the field, and selection or eligibility lives in skill prose.
- `branch_prefix` is decoded (`repository/decode.go` → `domain.OptionalString`). It is validated
  only at claim, by `domain.ValidBranchComponent`, and consumed by `domain.MintBranch`.
- Branch-prefix normalization (strip one trailing slash, refuse slash-embedded or
  `refs/`-qualified values) exists only as prose in `skills/docket-new-change/SKILL.md`.
- `change.groom` has outcomes `spec | trivial | revise`, a pinned `path` + `version` CAS, owned
  section edits (`render.ApplySectionEdits`), and an atomic board re-render.
  `render/section.go` already knows the `## Auto-groom blocked` heading.

## Design

### 1. `change.create` request shape

Add two optional fields to `ChangeCreateRequest`:

- `AutoGroomable *bool` (`json:"auto_groomable"`). Absent or `null` means unset, so the change
  inherits the repo's `auto_groom`. `true` and `false` are explicit overrides. This follows the
  existing `StackedOn *int` optional-scalar pattern.
- `BranchPrefix string` (`json:"branch_prefix"`). Empty (after normalization) means unset.

Both flow into `changeCreatePayload`, so the idempotency digest binds them. The **normalized**
prefix is what gets digested: requests carrying `Hotfix/` and `hotfix` are the same request. The
schema descriptor is reflected from the struct, so both fields appear in `docket schema` with no
hand edit.

### 2. Branch-prefix normalization moves into Go

Add `domain.NormalizeBranchPrefix(raw string) (normalized string, ok bool)` beside
`ValidBranchComponent`:

1. Trim surrounding whitespace. This is new: the skill never did it, and `" hotfix"` would
   otherwise reach claim and fail there.
2. Strip exactly one trailing `/` (moved from the skill).
3. Lowercase the result (new). Branch prefixes are lowercase-only, matching the change-type
   token grammar (`^[a-z][a-z0-9-]*$`), so `Hotfix` and `HOTFIX/` both store as `hotfix`.
4. If the result is empty → `("", true)`, meaning unset.
5. Otherwise require `ValidBranchComponent(result)`, which already refuses an embedded `/`,
   `refs`, `..`, `@{`, a leading `-`/`.`, a trailing `.`/`.lock`, whitespace, and git-illegal
   bytes.

`validateChangeCreateShape` calls it. A failure is refused with a new registered finding code,
`invalid-branch_prefix` (add it to `finding_codes.go` and the vocabulary), before any
transaction. The message names the rejected value and the reason, so a caller can relay it
verbatim. The create op stores the normalized value.

Deliberately **not** normalized:

- **`refs/heads/<x>`.** It is not rewritten to `<x>`. Guessing intent from a qualified ref is the
  silent rewrite the existing rule refuses.

Claim-time validation in `domain.Claim` is unchanged and stays strict.

### 3. Rendering

`render.NewChangeRecord` gains `AutoGroomable *bool` and `BranchPrefix string`. `ChangeRecord`
emits `document.Bool(v)` / `document.String(p)` when set and `document.Null()` otherwise. Quoting is
by construction (ADR-0071). No derived view changes on create.

### 4. Domain model: `auto_groomable`

The decoder reads `auto_groomable` into the change spec as a tri-state (absent/null, `true`,
`false`), using the decoder's existing optional-scalar handling, and the change builder can set it.
The new groom outcomes below write the field through this. Nothing else consumes it: selection,
eligibility, and the board stay as they are.

### 5. `change.groom` gains `outcome: abstain`

- **Precondition:** the change is needs-brainstorm (`proposed`, no `spec:`, not `trivial`).
  Otherwise the op refuses with the existing not-groomable refusal. It uses the same pinned
  `path` + `version` CAS, `contended` behaviour, exact-lease push, and single metadata commit as
  the other outcomes.
- **Request:** a new field `blocked_note` (markdown). It is required for `abstain` (the new
  finding `empty-blocked_note` if blank) and accepted by no other outcome. `sections`,
  `spec_markdown`, and `spec_version` are refused for `abstain`, so an autonomous caller cannot
  rewrite the proposal. Relationship fields are refused as well.
- **Effect:**
  - Sets `auto_groomable: false` and `updated:`.
  - Adds `## Auto-groom blocked` whose content is `Recorded <YYYY-MM-DD> (UTC).`, from the
    engine clock, then a blank line, then `blocked_note`.
  - If the section already exists, it appends a new dated entry, same format, to the end of the
    section instead of refusing. Earlier entries are preserved.
  - Re-renders the inline board in the same commit, so the row flips to
    `auto-groom blocked — needs you`.
- The change stays needs-brainstorm, and neither `spec:` nor `trivial:` is touched.

### 6. `change.groom` gains `outcome: rearm`

- **Precondition:** needs-brainstorm, as for abstain. The op refuses with a new finding,
  `nothing-to-rearm`, when the record has no `## Auto-groom blocked` section **and**
  `auto_groomable` is already `true`.
- **Request:** optional owned `sections` edits (the existing roster and intents), so a human's
  newly supplied context, typically an updated `## Open questions`, lands in the same commit.
  `blocked_note`, `spec_markdown`, and `spec_version` are refused.
- **Effect:**
  - Sets `auto_groomable: true` and `updated:`.
  - Removes the `## Auto-groom blocked` section if present, via the existing section-splice
    machinery.
  - Applies any section edits.
  - Re-renders the board in the same commit, so the row goes back to `needs-brainstorm`.

The outcome vocabulary, the invalid-outcome message, and the result `HumanText` all name the two
new outcomes.

### 7. Skills and convention

- `skills/docket-new-change/SKILL.md`: rewrite the "Two draft-time scalars `create` does not
  carry" paragraph. Both scalars now go in the `change.create` request, with the human's
  `branch_prefix` passed as typed. On `invalid-branch_prefix` the skill shows the finding and asks
  the human for a new value. All normalization and refusal rules leave the skill, and so does the
  post-create plain-git commit.
- `skills/docket-auto-groom/SKILL.md`, Step 4 exit 3 and Step 5: abstain becomes `change.groom`
  `outcome: abstain` with `blocked_note`, handled like the other exits on a `contended` refusal
  (re-sync, re-read, discard if no longer autonomous-eligible). Delete the false "changes no
  board-visible cell" claim and the plain-git abstain instructions. The Tier-B no-verdict posture
  routes through the same exit.
- `skills/docket-convention/SKILL.md`, *Autonomous grooming*: the abstain is now written by
  `change.groom` `outcome: abstain`. Re-arm is `change.groom` `outcome: rearm`, a human-typed or
  human-attended op, not a hand edit plus section deletion. Keep the rule that the abstain is the
  single agent write to `auto_groomable`.
- `skills/docket-groom-next/SKILL.md`: where it discusses abstained stubs, point the human at
  `outcome: rearm` to re-arm without grooming, and note that a spec or trivial groom of an
  abstained stub is unchanged.
- The `change.groom` operation docs and the README consultant/auto-groom sections are updated
  wherever they enumerate groom outcomes. Derive the sites from a whole-repo grep for the outcome
  list, never a hand list.

## Error handling

- Every refusal is typed and writes nothing: `invalid-branch_prefix`, `empty-blocked_note`,
  `nothing-to-rearm`, a disallowed field for the outcome, not-groomable, version mismatch, and
  contended.
- An abstain or rearm that loses the CAS race returns `contended`. The skill re-syncs and
  re-reads, as for spec and trivial.

## Testing

**Normalization**

- A table test for `domain.NormalizeBranchPrefix` covering: `hotfix` → `hotfix`,
  `  hotfix  ` → `hotfix`, `hotfix/` → `hotfix`, `Hotfix` → `hotfix`, `HOTFIX/` → `hotfix`, `hotfix//` → refused, `/hotfix` → refused,
  `team/hotfix` → refused, `refs` → refused, `refs/heads/x` → refused, `-x` → refused,
  `x.lock` → refused, `""` → unset, `"   "` → unset, `/` → unset.

**`change.create`**

- `auto_groomable` true, false, and absent write `true`, `false`, and `null`.
- `branch_prefix` set writes the normalized value, and a round-trip decode plus `MintBranch`
  yields `<prefix>/<slug>`.
- An invalid prefix is refused with `invalid-branch_prefix` and no commit.
- Creates with `Hotfix/` and `hotfix` under the same `request_id` replay instead of conflicting.
- Differing `auto_groomable` under the same `request_id` conflicts.
- The schema descriptor lists both fields.

**`change.groom` abstain**

- Sets the flag and the dated section, and `BOARD.md` in the same commit shows
  `auto-groom blocked — needs you`.
- A second abstain appends a second dated entry.
- It is refused on a spec'd, trivial, or non-proposed change, on an empty `blocked_note`, and on
  a request carrying `sections` or `spec_markdown`.

**`change.groom` rearm**

- Clears the section and sets `true`, and the board row returns to `needs-brainstorm`.
- With section edits, it applies both in one commit.
- `nothing-to-rearm` is refused.
- A version mismatch contends with nothing written.

**Skill-text guards**

- Any existing repoguard sentence guards over the edited skill paragraphs are updated. Mutation-test
  any new guard, per AGENTS.md.

## Out of scope

- Any change to what `auto_groomable` or `branch_prefix` *mean* (tri-state inheritance, how claim
  consumes the prefix).
- Moving auto-groom selection or eligibility into Go.
- Rewriting `refs/heads/<x>` to `<x>`.
- Normalizing (including lowercasing) prefixes already present on existing records, or loosening
  or case-folding claim-time validation. Only newly created records are normalized.
- Other frontmatter fields `change.create` does not accept today.
