# Guard the README's remaining config fences against key drift — design

Change: 0108 · anchored on `tests/test_docket_example_yml.sh` (change 0107 shipped `(8)`)

## Problem

Change 0107 added `(8) README SNIPPET CORRESPONDENCE` to `tests/test_docket_example_yml.sh`,
guarding **one** README fence — the `### `.docket.yml` — per-repo settings` worked example — against
`.docket.example.yml`, with **value equality**. Every other config fence in `README.md` is guarded
by nothing. A key renamed in the resolver, or a key that never existed, sits in those fences
indefinitely.

0107 could not simply extend its loop: its assert is value-equality, and the other fences
**deliberately show non-default values** to illustrate opting in, so the loop would go spuriously
RED against correct prose. The guard those fences need is a different assert.

## Why the fence set must be derived, not enumerated

The stub lists the unguarded fences by line number. Checked against `README.md` at `6cb6be6`, that
list **omits** the `reclaim:` fence (README:234–238) entirely. A hand-written fence list is an
[[enumerated-floor]] that ages directly into the gap it was written to close — it had already aged
before the change was groomed. **The guard must derive its fence set from the README.**

**Ground truth: `README.md` carries exactly 9 ```` ```yaml ```` fences** — 209, 234, 264, 289, 310,
407, 433, **576**, 594.

Fence **576 is indented two spaces** (a list-item continuation under the bullet that begins at
README:574) and carries `skills: / brainstorm: docket-brainstorm`:

```
574   - **Durable (config).** Bind the role in `.docket.yml` …
576     ```yaml
577     skills:
578       brainstorm: docket-brainstorm
579     ```
```

This is a **worked correction to this spec's own first draft**, recorded rather than quietly fixed
because it is the change's central lesson repeating itself one level up. The draft asserted "there
is no `skills:` fence" and put the count at 8, on the strength of a **column-0-anchored**
`grep -n '^```'`, which structurally cannot see an indented fence. The stub's "~README:574" was
right — off by two lines, not fabricated. The draft over-read README:283 ("consult it there rather
than copying examples here"), which scopes the `.docket.yml` configuration section only; the
**Customization** section at 567+ does copy a `skills:` example.

Two binding consequences for the builder:

1. **The discovery regex must be whitespace-tolerant** — `^[[:space:]]*```yaml[[:space:]]*$`, with
   the closing fence matched at the **same indent**. A column-0 regex silently excludes fence 576.
2. **The count floor and the regex must be fixed together.** A builder who implements §6 floor 1 as
   `count = 8` alongside a column-0 regex gets a **green** suite that permanently excludes fence 576
   — and the floor whose job is catching discovery breakage would actively resist the correction.
   The literal is **9**.

## Design

Add a new section **`(9) README CONFIG FENCE KEY CORRESPONDENCE`** to
`tests/test_docket_example_yml.sh`. Section `(8)` is left byte-untouched.

### 1. Fence discovery — derived, default-in

Scan the whole of `README.md` for ```` ```yaml ```` fences (whitespace-tolerant opener, closer
matched at the same indent) and extract each body. Every discovered fence is in scope **by
default** — a new config fence is guarded the day it is written, with no second edit anyone can
forget.

**Marker grammar.** Opt-out and value opt-in are both an HTML comment attached to a fence:

```
<!-- docket:config-fence: ignore -->     # not .docket.yml schema — skip this fence entirely
<!-- docket:config-fence: values -->     # also assert value equality against the example
```

- **Attachment.** The marker is the **nearest preceding non-blank line** to the fence opener, not
  strictly the line immediately above. Fence 576 forces this: it is a list-item continuation
  preceded by a blank line, and a column-0 HTML comment there would terminate the list. The parser
  therefore tolerates **leading whitespace** on the marker, and a marker must sit at **at least** its
  fence's own indent — never at column 0 for an indented fence, which would terminate the enclosing
  list.
- **Unknown or malformed token is a HARD FAIL**, naming the two valid tokens — never
  warned-and-ignored. The two mistake directions are asymmetric: a typo'd `ignore` fails safe (the
  fence is still checked, reddens, is noisy), but a typo'd `values`, a typo'd marker name, or a bare
  `<!-- docket:config-fence -->` fails **open and silent** — value coverage evaporates with no
  signal, which is exactly the drift class this change exists to end. A line matching
  `docket:config-fence` that does not match the exact grammar reddens.
- **At most one marker per fence**; a second reddens rather than one silently winning.

### 2. Anchor — the example, one hop

Each fence's keys are resolved against **`.docket.example.yml`**, not against the resolver's export
surface directly. `(2a)` + `(2b)` + `(2c)` already bind the example to the resolver in both
directions and prove it a faithful superset of everything the code reads; going through the example
is one hop, reuses the file `(8)` already anchors on, and keeps a single anchor per artifact rather
than two competing ones.

### 3. The assert — existence-only

For each fence key path: assert it exists in the example. **No value comparison.** This is what
makes the guard applicable to all nine fences, and it dissolves the stub's third open question
("how does a fence declare its values are deliberately non-default") — under existence-only, it
never has to.

Value equality is **not** lost where it is sound: it stays exactly where `(8)` already has it (the
per-repo fence, the one that documents shipped defaults), and a fence may opt **in** with

```
<!-- docket:config-fence: values -->
```

The `reclaim:` fence (README:234) is marked `values` in this change — its `lease_ttl: 72` /
`auto: false` are shipped defaults and should redden if the defaults move. The marker goes at
README:233, outside section `(8)`'s span (203–224, ending at the `### Reclaiming stale claims`
heading at 225), so `snippet_section()` is unperturbed. The remaining eight fences stay
existence-only, `(8)`'s fence among them — it is simply left unmarked, and since no unmarked fence
gets a value assert, no special-casing of `(8)`'s fence exists or is needed. Fence 209 is therefore
double-covered by `(8)` (existence + values) and `(9)` (existence only); that overlap is accepted
per A8.

### 4. Key resolution — query-by-key, not build-a-set

Two README fences use `agents:` and `agent_harnesses:` **actively**, but `(3) PRESENCE-SENSITIVE
KEYS SHIP COMMENTED` asserts those keys must be **commented** in the example. A naive
`path ∈ flatten_yaml(example)` therefore reddens against correct README prose.

Resolved by querying the example **per key name** rather than building a pseudo-key set:

- a path is known if it appears in `flatten_yaml < .docket.example.yml` (active keys), **or**
- its top-level segment matches `^#[[:space:]]*<key>:` in the example (commented pseudo-key).

Building a pseudo-key set by regex is rejected: `^#[[:space:]]*[A-Za-z_]+:` also matches the
example's prose comments — `# exceptions:` (line 22), `# scope: any layer …` (many), `# line: it
bypasses PRs…` (line 179, a wrapped prose line) — which would silently accept anything. Querying by
the key name the README actually carries never touches those lines.

**Residual hole, to be written into the test as a comment rather than closed.** Query-by-key narrows
the false-accept surface but does not eliminate it: a future fence key whose *name* collides with a
prose-comment word would be silently accepted (`scope:` would match `# scope: any layer` at
column 0, so anchoring is no defense). No collision exists among today's 12 top-level fence keys. It
is not closed because the only tight closure is an explicit two-key allowlist — the shape section
`(3)`'s awk uses at :206 — and that is an enumerated floor of exactly the kind A1 rejects. Accepting
a narrow, documented hole beats re-introducing the mechanism this change is built to avoid.

### 5. Nested paths — and a required widening of `flatten_yaml`

`flatten_yaml` (already in the file, indent-stack based, generic to depth) produces dotted paths.
Rule:

- top-level segment **active** in the example ⇒ the **full dotted path** must match
  (`finalize.gate`, `reclaim.lease_ttl`, `runners.codex.sandbox`, `skills.build` all resolve today);
- top-level segment only a **commented pseudo-key** ⇒ acceptance stops at the top-level segment,
  because a commented key has no nested body to match against (`agents.claude.status`,
  `agents.default.<agent>`).

**Blocking prerequisite — widen the key class to admit hyphens.** `flatten_yaml`'s key regex is
`^[[:space:]]*[A-Za-z_][A-Za-z0-9_]*:` — **no hyphen** — so it silently drops
`implement-next: { model: claude-opus-4-8, effort: xhigh }`, which appears in the global-config
fence (README:296) and the `.docket.local.yml` fence (README:320). Left alone, `(9)`'s §6 floor 3
(raw-vs-flattened, declared non-optional) **ships RED on correct README prose** — 11 raw lines vs 10
flattened, on both fences — which is precisely the outcome this design exists to avoid. Note the
*existence* assert is not what breaks: `agents` is a commented pseudo-key, so acceptance stops at
the top-level segment and passes. It is the non-vacuity floor that reddens.

Fix: widen the key class to `[A-Za-z_][A-Za-z0-9_-]*` in `flatten_yaml`. **This is a TWO-line edit,
not one** — `flatten_yaml` carries the key class **twice**:

- the **shape test** (`if (line !~ /^[[:space:]]*[A-Za-z_][A-Za-z0-9_]*:/) next`, :782 — was :447 when this spec was written), and
- the **value strip** (`sub(/^[[:space:]]*[A-Za-z_][A-Za-z0-9_]*:[[:space:]]*/, "", val)`, :785 — was :450). Locate BOTH by their code, not by line number: change 0102 grew the file to 884 lines after this spec was drafted, and it will move again.

**Widening only the shape test is a half-fix that this design's own floors cannot catch.** With the value strip left
hyphen-free the line passes the shape test, gets its path, and then fails the value strip — so
`agents.default.implement-next` comes back carrying **the entire raw line** as its value. Verified:
half-widened and fully-widened both yield `flat=11` on fences 289/310, so floor 3 passes either way,
the existence assert passes either way, and `ex_flat` is unchanged either way. Nothing in `(9)`
reddens. The builder must widen **both** occurrences and assert the extracted *value*, not only the
path count.

**Blast radius independently verified as nil**: `.docket.example.yml` contains **no** hyphenated
active key (`grep -nE '^[[:space:]]*[A-Za-z_][A-Za-z0-9_]*-[A-Za-z0-9_-]*:'` — no matches), so
`ex_flat` stays at 30 paths and is **byte-identical** before and after (`(8)`'s `>= 20` floor
holds), and `(8)`'s own fence has no hyphenated key, so `sn_count` stays exactly 5. The widening is
behavior-neutral for sections `(1)`–`(8)`; show it so by running the suite before and after.

`(8)`'s comment at :477–483 already documents this exact hazard ("a key spelled with any other
character … is silently REJECTED by `flatten_yaml` rather than flagged") — this change is where that
documented hazard finally has a live instance to fix.

### 6. Non-vacuity — the live risk

`(9)` iterates a discovered set, so its real failure mode is discovering **zero** fences and sailing
through green. Three floors, mirroring `(8)`'s:

1. **Exact fence count = 9**, with the remedy inlined in the assert message ("if you added a config
   fence, bump this literal AND ensure its keys are in `.docket.example.yml`") so it survives into
   CI output. The literal is `9` **only if** the discovery regex is whitespace-tolerant (§1); the
   two are a matched pair and must be mutation-tested together.
2. **Per-fence non-empty flatten** — a fence that flattens to zero paths reddens.
3. **Raw-vs-flattened cross-check per fence** — every non-blank, non-full-line-comment line inside a
   fence must survive `flatten_yaml` into exactly one path, so a key spelled outside the flattener's
   key-class regex (`[A-Za-z_][A-Za-z0-9_-]*:` after the §5 widening) is flagged rather than silently
   dropped.

**Mutation tests — all four are required**, because a floor over an invariant that never computes
passes vacuously:

1. Plant a phantom key in a README fence ⇒ `(9)`'s existence assert reddens.
2. Regress the fence-discovery regex to column-0-anchored ⇒ the count assert reddens (this is the
   draft's own bug, pinned as a test).
3. Regress `flatten_yaml`'s key class to the hyphen-free form ⇒ floor 3 reddens on fences 289/310.
4. **Marker-parse mutation** — plant `<!-- docket:config-fence: valeus -->` on a fence ⇒ hard fail.
   Required because all 9 fences today are config fences, so **zero** exercise the `ignore` path: it
   would otherwise ship with its only branch untested (`green-suite-untested-branch`,
   `specified-but-unreachable`). Also assert the `ignore` path positively, on a temporary fixture
   fence rather than by adding a real ignored fence to the README.

   **Builder note:** the fixture fence means the fence-scan helper must take its markdown path as an
   **argument**, not read a hardcoded `$README`. Section `(8)` already parameterizes this way
   (`snippet_section()` reads `$README`), so the shape exists — extend it to a parameter rather than
   introducing a second convention.

## Assumptions

| # | Decision | Chosen | Rejected | Why |
|---|---|---|---|---|
| A1 | Fence set | **Derived** by scanning the README, whitespace-tolerant opener, **count literal 9** | The stub's hand-written line-number list; a column-0 fence regex | The stub's list already omits the `reclaim:` fence. An enumerated floor ages into the gap it closes. The count and the regex are a matched pair — see §"Why the fence set must be derived": this spec's own first draft got both wrong the same way. |
| A2 | Anchor | **`.docket.example.yml`** | The resolver's export surface directly | `(2a)/(2b)/(2c)` already prove the example a faithful superset of the resolver's read surface. One hop, one anchor per artifact. |
| A3 | Assert | **Existence-only** by default | Value equality everywhere; existence-only everywhere | Value equality goes spuriously RED against the opt-in-illustrating fences (`auto_capture: true`, `terminal_publish: true`, `metadata_branch: main`, and the layered-config samples). Existence-only applies to all nine and dissolves the "declare non-default" question. |
| A4 | Value equality | **Opt-in** per fence via `<!-- docket:config-fence: values -->`; applied to `reclaim:` | Drop value checking entirely | `reclaim:`'s `72`/`false` are shipped defaults; losing that is real signal lost. One marker, only where the claim is true. |
| A5 | Non-config fences | **Default-in + `ignore` marker**; grammar, attachment rule, and unknown-token **hard fail** all specified in §1 | Default-out; auto-skip fences whose keys look unknown; warn-and-ignore on a bad token | Auto-skip is self-defeating — a typo'd key would make its own fence invisible, which is the drift being guarded. Warn-and-ignore fails **open and silent** on a typo'd `values`. Attachment is nearest-preceding-non-blank (not strictly the line above) because fence 576 is an indented list continuation. |
| A6 | Commented pseudo-keys | **Query-by-key** against the example | Build a pseudo-key set by regex | The set-building regex also matches the example's prose comments (`# exceptions:`, `# scope:`, `# line:`), silently accepting anything. |
| A7 | Nested resolution | Full path when the top-level key is **active**; top-level only when it is a **commented pseudo-key**. **Blocking prerequisite: widen `flatten_yaml`'s key class to `[A-Za-z_][A-Za-z0-9_-]*` at BOTH occurrences — the shape test and the value strip, located by their code (:782/:785 as of `5ed3d8c`; :447/:450 when drafted)** | Top-level-only everywhere; leaving the flattener alone; scoping floor 3's filter to exclude hyphenated keys | Top-level-only misses `finalize.gaet`-class typos. The hyphen-free key class drops `implement-next:` (README:296, :320), so floor 3 would ship RED on correct prose — widening is verified behavior-neutral for `(1)`–`(8)` (no hyphenated active key in the example), whereas narrowing the floor would hide the class of drift the floor exists to catch. |
| A8 | Placement | **New section `(9)`**; `(8)` untouched | Rewrite `(8)` into one general mechanism | `(8)` carries fence-1-specific guards earned by review (exact-count ceiling against regrowth, pointer target check, raw-vs-flattened). Rewriting risks regressing them for no coverage gain. |
| A9 | Reverse loop (example → README) | **Not written** | Assert every example key appears in the README | Restating the stub's out-of-scope: that is the fourth all-keys surface change 0101 deleted. The correspondence is a deliberate proper subset, not a mirror — the exception `correspondence-guard-runs-one-way` already records. |
| A10 | Dependency state | None — `depends_on: []`, no unsatisfied deps | — | Touches one test file, `README.md` markers, and nothing else. Builds against `main` at `5ed3d8c` as-is (re-verified at reconcile, 2026-07-21). |

## Learnings applied

- `correspondence-guard-runs-one-way` — direction named explicitly (fence → example, subset); the
  reverse loop is deliberately absent and A9 records why, in the test as a comment.
- `enumerated-floor` — A1: the fence set is derived, never listed.
- `backstop-must-compute-not-reenumerate` / `guards-are-code` — §6: mutation-test the **population**,
  not only the suppression.
- `verify-the-claim` — applied, then **violated by this spec's own first draft, then applied to the
  draft**. The stub's filename is wrong (`test_docket_yml_example.sh`; the real file is
  `test_docket_example_yml.sh`) and its fence list omits `reclaim:`. But the draft's counter-claim
  ("there is no `skills:` fence", "exactly 8 fences") was itself false, produced by a column-0 grep
  that structurally could not see fence 576 — the stub was right and the draft's own verification
  tool was the defect. Recorded in the body, not smoothed away: a claim's provenance ("we just
  verified it") is not evidence, and the check that produced it must be verified too.
- `agent-shell-noop-reads-as-success` — the same root cause as the fence miscount: a grep that
  matches nothing is indistinguishable from a grep that correctly matches nothing. §6's floors and
  mutation 2 exist for exactly this.
- `escape-ere-metacharacters-in-key` — A6's per-key query interpolates a key name into an ERE;
  escape it before matching.

## Out of scope

- Re-litigating 0107's forward-only direction, or any reverse/completeness loop over the example's
  keys.
- Auditing the README's non-config prose claims.
- Any change to `.docket.example.yml`'s own content, or to sections `(1)`–`(8)`.
