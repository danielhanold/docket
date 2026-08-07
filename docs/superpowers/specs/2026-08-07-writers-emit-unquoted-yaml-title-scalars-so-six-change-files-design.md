<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0235 — Writers emit unquoted YAML title scalars, so six change files fail to parse](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0235-writers-emit-unquoted-yaml-title-scalars-so-six-change-files.md)**
<!-- docket:backlink:end -->

# Design — Writers emit unquoted YAML title scalars, so six change files fail to parse

Change: 0235 · Type: fix · Drafted by `docket-auto-groom` (autonomous; see `## Assumptions`).

## Problem

Six change files on `docket` carry frontmatter that no YAML parser accepts, all of them an unquoted
`title:` scalar carrying a colon:

| id | where | shape |
|---|---|---|
| 0121 | active | `The manifest's elsewhere: check proves …` — colon-space |
| 0219 | active | `aborted-run's sixth signature: PR opened …` — colon-space |
| 0234 | active | `Split gate-execution.md: probe evidence …` — colon-space |
| 0173 | archive | `… a model ID containing / or :` — **trailing colon** |
| 0211 | archive | `… stops after the build: commits …` — colon-space |
| 0217 | archive | `Clear change 0202's three minor findings: dead guard …` — colon-space |

docket already knows the rule (promoted `yaml-scalar` finding in AGENTS.md, ADR-0065) and already
*detects* four of the six (`board-checks.sh`'s `scalar-form`, change 0191) — yet keeps minting new
violations, because the rule is scoped to **hand-authored** scalars while the highest-volume writer
is a script: `mint-stub.sh`'s `set_field` writes `print key ": " val` with no quoting step at all.
Its byte-for-byte ENVIRON write (the B1 property) protects the value from *shell/regex*
reinterpretation; it says nothing about *YAML syntax*.

`scalar-form` has its own hole: it tests for `': '` and for bare YAML booleans, but not for a value
**ending** in `:` — which is why 0173 was never reported.

Impact is bounded (docket's own readers are grep/awk and tolerate it) but the failure mode for any
real parser — the GitHub mirror, an external tool, a future reader — is a whole-manifest parse abort,
not one bad field.

## Design

### 1. One shared predicate, two consumers

Add a single needs-quoting predicate to `scripts/lib/docket-frontmatter.sh` — the library both the
writer and the checker already source (`mint-stub.sh:85`, `board-checks.sh:73`) — e.g.
`docket_scalar_needs_quoting VALUE`, returning 0 when emitting VALUE as a **bare** YAML scalar would
not be well-formed. It takes the **logical value** and carries only syntax legs:

1. the value contains `': '` (colon-space);
2. the value **ends** with `:` (the 0173 leg — new);
3. the value is, whole-value and case-insensitive, exactly one of `on off yes no true false`
   (YAML 1.1 boolean);
4. the value contains ` #` — a YAML comment introducer, which **silently truncates** the value
   rather than aborting (`finding #3` in a title is entirely plausible from auto-capture);
5. the value's first character is a YAML indicator: one of ``[ ] { } , & * ! | > ' " % @ ` ? :``,
   or a leading `-`/`?`/`:` followed by a space. `&anchor …` silently loses its first word;
   `[WIP] …`, `*star* …` and `@mention …` abort the parse. **Exemption:** a value that is already a
   well-formed flow collection — it opens with `[` and closes with `]`, or opens with `{` and closes
   with `}` — does not fire. `set_field` writes `discovered_from: [234]` through this same path, and
   quoting it would turn a YAML *sequence* into a *string* for any real parser: a silent,
   parser-visible regression on the very field this change cites. docket's own `list_field` would
   not notice (it unwraps quotes first), which is exactly what makes it worth an explicit leg.

   The exemption is a **shape** test, keeping A3's no-key-enumeration stance — it never asks which
   key is being written.

Legs 4 and 5 are prospective — no title in the tree hits them today — but they are the same defect
class reached by ordinary English, two `case` statements away, in a file this change already opens.

An **empty** value is explicitly exempt, not incidentally so: no leg would fire on `""` in a careful
`case` implementation, but `archive-change.sh:101` writes `claimed_at ""` through its own
`set_field`, so the day that copy adopts the predicate the empty case must already be stated.

**Domain discipline.** The predicate never inspects quoting: it answers "would this logical value
be safe bare?". The *already-quoted* skip leg (raw token opens with `"` or `'`) stays where the raw
token lives, in `board-checks.sh`'s `scalar_form_check` — applying it to a logical value would be
unsound, since a title that logically *starts* with a quote character (`"quoted" start`) must be
quoted, not skipped.

- `board-checks.sh`'s `scalar_form_check` keeps its skip leg and its per-leg messages, and delegates
  the syntax legs to the predicate instead of restating them inline. This closes the trailing-colon
  gap in the checker and makes writer and checker provably one rule rather than two copies that can
  drift (`restatement-accumulates-its-own-guards`).
- `mint-stub.sh`'s `set_field` calls the predicate before writing, and quotes when it fires.

### 2. Quote at the write boundary — silently, single-quoted, only when needed

`set_field` gains a quoting step: when `docket_scalar_needs_quoting` fires for the value, the value
is emitted as a **single-quoted** YAML scalar with every embedded `'` doubled; otherwise it is
emitted bare, exactly as today. The doubling happens **in bash, before the value is exported** to
`awk`'s `MINT_SF_VAL` — never through `awk`'s `gsub`, whose replacement string reinterprets `&` on
model-authored prose. The ENVIRON read stays the write mechanism, so B1 byte-fidelity survives with
no new metacharacter surface.

**The reader must be the exact inverse.** `_docket_unwrap_quotes` today strips one matched quote
pair and deliberately does no unescaping, so a written `'…manifest''s…'` would read back as
`manifest''s` — visible in `BOARD.md` (`render-board.sh` renders `field "$f" title` directly) and in
`mint-stub`'s `dup_of` slug comparison. Three of the six repair targets carry apostrophes, so this
is not hypothetical. `_docket_unwrap_quotes` therefore gains one leg: inside a **single-quoted**
token, `''` collapses to `'`. Double-quoted tokens are untouched — no escape interpretation is added
there, and the two existing double-quoted titles (0190, 0137) contain no escapes.

Only-when-needed is load-bearing, not cosmetics: `set_field` also writes `id`, dates, `type` and the
**list** `discovered_from: [234]`, and unconditional quoting would turn a YAML sequence into a
string. None of the predicate's legs can fire on an integer, an ISO date, or a slug, and leg 5's
flow-collection exemption keeps it off a `[…]` list — so a narrow predicate is also the safe one.
`§5`'s "`discovered_from` is still an unquoted list" assert is the guard on precisely this.

The rest of the reader contract already holds: `field()` unwraps one matched pair of surrounding
quotes (change 0138) and `field_raw()`/`fm_field_raw()` keep them for the checker; both quoting
forms stay accepted on read — the writer merely picks one.

### 3. Repair the six files — on `docket` **and** on the integration branch

One commit on `docket` quoting the six `title:` values, active and archive alike. No word of any
title changes; an archived record stays the same record, in a form a parser can read. (An archived
file is immutable as a *record*; a syntactically broken one is not a record anyone can read. Nothing
in the convention makes an archived change file immutable — that rule covers `Accepted` ADRs only.)

This repo runs `terminal_publish: true`, and all three archived targets (0173, 0211, 0217) are
already published on `origin/main` with the broken line intact. A `docket`-only repair would leave
main's copies unparseable and the two branches divergent — the stated goal half-met. The three
archived files are therefore **republished onto the integration branch** with
`terminal-publish.sh`, in the same change (`adr-update-delivery`: an edit to an already-published
record has to reach the branch it was published to).

### 4. Widen the rule's wording

The promoted `yaml-scalar` finding (AGENTS.md, and its receipt in
`docs/changes/learnings/yaml-scalar.md`) says "hand-authored". Widen it to cover **any** writer —
model or script — and name the write-boundary quoting as the script-side half. ADR-0065's decision
is unchanged and is not edited.

### 5. Guards

- Unit coverage for the predicate itself: each leg fires; each near-miss (a colon with no space, an
  interior colon-space inside an already-quoted value, `offset` vs `off`) stays silent.
- `set_field` round-trip: mint a stub whose `--title` carries each malformed shape, then assert the
  written file's `field_raw` is quoted, `field` returns the original value byte-for-byte, and
  `discovered_from` is still an unquoted list. **The apostrophe cases are mandatory fixtures**, not
  an extra — a title carrying both `'` and `': '` (all three of 0121/0217/0219 do) is the one shape
  that fails if the writer's doubling and the reader's undoubling are not exact inverses. Include a
  title that logically starts with `"`, which must be quoted rather than skipped. Mutation-test by
  stripping the quoting step, and separately the undoubling leg, and watching the asserts redden
  (`guards-are-code`, `assert-detects-removal-not-replacement`).
- `board-checks.sh`: add the trailing-colon RED fixture alongside the existing colon-space and
  boolean ones, and keep the quoted-value SILENT fixtures.
- The **live-tree** backstop stays `scalar-form` under `board-checks`, which already runs over the
  real metadata branch. No YAML-library guard is added — see the assumptions.

## Out of scope

- `field-domain`'s `title contains '|'` finding (change 0189) — a rendering corruption, with its own
  check.
- Rewording any title for style.
- A YAML library anywhere in the runtime read path. The readers stay grep/awk.

## Assumptions

Autonomous decisions taken without the human; each records the rejected alternatives.

**A1 — Quote silently rather than reject the title.** `mint-stub` is invoked by autonomous skills
(auto-capture) with no human present to reword a title, so a refusal turns a valid capture into an
abort and loses the discovery. *Rejected:* refuse and make the caller reword (keeps raw files
maximally readable, but converts a formatting nit into a run-halting error on an unattended path);
warn-and-write-anyway (leaves the malformed file, which is the status quo).

**A2 — Single-quoted output, with the reader taught its exact inverse.** Single-quoted YAML
interprets no escapes, so only `'` needs doubling and a title containing `\` or `"` survives
untouched. The cost is that `_docket_unwrap_quotes` does not undouble `''` today, so the writer's
form must be matched by one new reader leg (§2) — a producer whose escaping the reader cannot invert
would corrupt every apostrophe-bearing title in `BOARD.md` and in `dup_of`'s slug comparison, and
three of the six repair targets carry apostrophes. *Rejected:* double-quoted (needs `\` **and** `"`
escaped — two escapes to invert instead of one), even though the two already-correct files (0190,
0137) use it; both stay valid, since the checker's skip leg accepts either form and the reader's
double-quote handling is unchanged. *Rejected:* picking a form per value (single-quote unless the
value has an apostrophe, then double-quote) to avoid touching the reader — it dodges the read-path
edit only until a value carries both an apostrophe and a `"`, and a branching writer with no
inverse is harder to reason about than one rule plus its inverse.

**A3 — Quote only when needed, not always.** Unconditional quoting would quote
`discovered_from: [234]` and break `list_field`. *Rejected:* always-quote (simpler rule, breaks list
and integer fields); quote only the `title` key by name (a hand-listed field enumeration — the exact
shape `board-checks.md` already argues against, and it would miss the next free-text field added).

**A4 — One shared predicate in `lib/docket-frontmatter.sh`, scoped to the logical value.** The
writer and the checker enforcing the same rule from one definition is what makes the trailing-colon
fix land in both at once, and both scripts already source the library. The predicate takes the
**logical value** and holds only the syntax legs; the already-quoted skip leg stays in
`scalar_form_check`, which is the only site holding a raw token. Sharing the skip leg too would be
unsound in the writer's domain — a title logically starting with `"` would be skipped and written
bare, which does not parse. *Rejected:* fixing the two sites independently (the drift that produced
this change); one predicate over the raw token for both (wrong answer on the writer's side).

**A5 — No YAML-parser-based test.** The change stub proposes "a test that parses every change file's
frontmatter as YAML" as the real backstop. Rejected on two grounds: the hermetic suite cannot see
the metadata branch where every real violation lives (`metadata-branch-invisible-to-suite`), and a
guard gated on an optional `yq`/`python3-yaml` being installed goes silently vacuous wherever it is
absent, which is precisely the failure class this change exists to fix. The shape predicate, now
shared with the writer and run by `board-checks` over the live tree, covers the same six violations
deterministically and with no new dependency. *Rejected alternatives:* an optional parser leg that
skips when absent (vacuous); a hard `yq` dependency for the suite (a new install-time requirement
for a shape check three `case` statements can do).

**A6 — Repair archived files in place, and republish them.** No convention rule is violated:
immutability covers `Accepted` ADRs, not archived change files. Because `terminal_publish: true` has
already put the three archived targets on `origin/main`, the repair is only complete when it reaches
main too (§3). *Rejected:* leave archive alone as immutable history (leaves a permanently
unparseable record); repair on `docket` only (leaves main's published copies broken and the branches
divergent); re-archive with a note (churn for a syntax fix).

**A7 — Only `mint-stub`'s `set_field` is a script-side offender.** `archive-change.sh` and
`reclaim-claims.sh` carry their own `set_field` copies but only ever write generated constants
(status, dates, branch names, a results path), so their values cannot reach the predicate's legs.
They are left alone; the build should re-confirm this rather than take it on faith
(`model-authored-values-are-untrusted-input`: the precondition lives in the call sites, not the
helper).

**A8 — `blocked_by:` gets no writer-side fix.** It is written by skills (models) editing frontmatter
directly, not by a script, so the AGENTS.md rule widened in §4 is its only lever. `scalar-form`
already checks it on read.

**A9 — Dependency state.** `depends_on: []`, no unsatisfied dependency. The stub is `related:` to
nothing, though 0234 is one of the six broken files and 0189's `|`-in-title finding is its declared
out-of-scope sibling; neither gates this work.

**A10 — Scope stays at the write boundary, its reader inverse, and the checker.** The read path
takes exactly one edit — `_docket_unwrap_quotes` learns to undouble `''` inside a single-quoted
token, the inverse of the writer's one escape (A2). It is not avoidable: a quoting form the reader
cannot invert defers the corruption rather than preventing it. Everything else holds — no YAML
library on the read path, no new config knob, and no new check id, since `scalar-form` gains legs
rather than a sibling check, leaving the `docket-status` check-id vocabulary and its consumers
untouched.
