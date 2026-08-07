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
   `[WIP] …`, `*star* …` and `@mention …` abort the parse.

Legs 4 and 5 are prospective — no title in the tree hits them today — but they are the same defect
class reached by ordinary English, two `case` statements away, in a file this change already opens.

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
emitted bare, exactly as today. The ENVIRON mechanism is unchanged — the transformation happens on
the value inside `awk`, so B1 byte-fidelity of the *logical* value survives and no `sed`/replacement
metacharacter is ever reinterpreted.

Only-when-needed is load-bearing, not cosmetics: `set_field` also writes `id`, dates, `type` and the
**list** `discovered_from: [234]`, and unconditional quoting would turn a list into a string that
`list_field` can no longer parse. Because the predicate's three legs can never fire on an integer,
an ISO date, a slug, or a `[…]` list, a narrow predicate is also the safe one.

Reader compatibility is already in place: `field()` unwraps one matched pair of surrounding quotes
(change 0138), and `field_raw()`/`fm_field_raw()` keep them for the checker. Both quoting forms stay
accepted on read — the writer merely picks one.

### 3. Repair the six files

One commit on `docket` quoting the six `title:` values, active and archive alike. No word of any
title changes; an archived record stays the same record, in a form a parser can read. (An archived
file is immutable as a *record*; a syntactically broken one is not a record anyone can read.)

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
  `discovered_from` is still an unquoted list. Mutation-test by stripping the quoting step and
  watching the asserts redden (`guards-are-code`, `assert-detects-removal-not-replacement`).
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

**A2 — Single-quoted output.** Single-quoted YAML interprets no escapes, so only `'` needs doubling
and a title containing `\` or `"` survives untouched. *Rejected:* double-quoted (must escape `\` and
`"`; more ways to get it wrong), even though the two already-correct files (0190, 0137) use it — both
stay valid, since the checker's skip leg accepts either form.

**A3 — Quote only when needed, not always.** Unconditional quoting would quote
`discovered_from: [234]` and break `list_field`. *Rejected:* always-quote (simpler rule, breaks list
and integer fields); quote only the `title` key by name (a hand-listed field enumeration — the exact
shape `board-checks.md` already argues against, and it would miss the next free-text field added).

**A4 — One shared predicate in `lib/docket-frontmatter.sh`, not two copies.** The writer and the
checker enforcing the same rule from one definition is what makes the trailing-colon fix land in
both at once. *Rejected:* fixing the two sites independently (the drift that produced this change).

**A5 — No YAML-parser-based test.** The change stub proposes "a test that parses every change file's
frontmatter as YAML" as the real backstop. Rejected on two grounds: the hermetic suite cannot see
the metadata branch where every real violation lives (`metadata-branch-invisible-to-suite`), and a
guard gated on an optional `yq`/`python3-yaml` being installed goes silently vacuous wherever it is
absent, which is precisely the failure class this change exists to fix. The shape predicate, now
shared with the writer and run by `board-checks` over the live tree, covers the same six violations
deterministically and with no new dependency. *Rejected alternatives:* an optional parser leg that
skips when absent (vacuous); a hard `yq` dependency for the suite (a new install-time requirement
for a shape check three `case` statements can do).

**A6 — Repair archived files in place.** *Rejected:* leave archive alone as immutable history
(leaves a permanently unparseable record); re-archive with a note (churn for a syntax fix).

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

**A10 — Scope stays at the write boundary and the checker.** No change to the read path, no new
config knob, no new check id — `scalar-form` gains a leg rather than a sibling check, so the
`docket-status` check-id vocabulary and its consumers are untouched.
