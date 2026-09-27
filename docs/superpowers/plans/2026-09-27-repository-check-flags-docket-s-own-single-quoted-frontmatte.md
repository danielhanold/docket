<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0447 — repository check flags docket's own single-quoted frontmatter as needing manual review](https://github.com/danielhanold/docket/blob/docket/docs/changes/active/0447-repository-check-flags-docket-s-own-single-quoted-frontmatte.md)**
<!-- docket:backlink:end -->
# Repository Check: Stop Flagging Writer-Quoted Scalars — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking. (In docket this plan is executed by the `docket-build` role.)

**Goal:** `reposetup.PlanRepairs` stops emitting a `frontmatter-manual-review` finding for a string field whose token is a well-formed single- or double-quoted YAML scalar, which is what docket's own writer emits. Genuine plain-scalar and malformed cases keep reporting as they do today.

**Architecture:** One unexported helper, `wellFormedQuotedString(token string) bool`, goes into `internal/reposetup/repair.go`. It decides on the **parsed** YAML node (kind, tag, anchor, style) and requires that nothing follows the node. `planQuote` calls it right after its `ShapeInline` gate and returns "no finding" when it is true. `unsafeScalarShape`, `decodesToStringLiteral`, `buildRepair`, `health.go`, the migrate preview and the writer do not change. Three test layers guard the fix: a direct helper table, a writer/checker parity table in `internal/reposetup`, and an end-to-end `change.create` → `PlanRepairs` zero-findings test in `internal/app`.

**Tech Stack:** Go; `go.yaml.in/yaml/v3` (already imported by `repair.go`); stdlib `errors`, `io`, `strings`.

**Spec:** `docs/superpowers/specs/2026-09-27-repository-check-flags-docket-s-own-single-quoted-frontmatte-design.md`. It lives on the `docket` metadata branch and can be read at `.docket/docs/superpowers/specs/…` from the primary tree. Change: `docs/changes/active/0447-repository-check-flags-docket-s-own-single-quoted-frontmatte.md`.

## Global Constraints

- Production code changes are confined to `internal/reposetup/repair.go`. Test changes go only in `internal/reposetup/repair_test.go` and `internal/app/change_create_test.go`. No new files, exported symbols, flags, or schema changes.
- Do NOT change `unsafeScalarShape`, `decodesToStringLiteral`, `buildRepair`, the manual-review finding's message/severity/remedy, `frontmatterFinding` in `internal/reposetup/health.go`, `internal/app/repository_migrate.go`, or the writer (`internal/document`). The writer's quoting policy is ADR-0071 and is out of scope.
- Do NOT add a scalar-style field to `document.Field`. The spec rejects that as YAGNI.
- Hand-edit no change records.
- Every `go test` invocation in this plan uses `-count=1`. Go's test cache can serve a stale pass, and a cached result is absence of evidence (learning `cached-runner-serves-a-mutated-tree`).
- Every mutation probe restores with a **backup copy** (`cp f f.bak; <mutate>; <run>; mv -f f.bak f`), never `git checkout --`, which restores to HEAD and would destroy the uncommitted edit under test (learning `mutation-restore-needs-a-backup-copy`).
- The build gate at the end runs the complete configured suite (`build.test_command`, which resolves to `go run ./cmd/docket development test`), not only these tests. That gate belongs to the executing skill, not to any task here.

## Plan-time findings (verified by probe against HEAD `e244dfd26`)

These were established by executing throwaway probe tests (via `go test -overlay`, so the worktree was left untouched). The tasks below rely on them.

1. **Root cause confirmed.** Here is `PlanRepairs` on a record with `title: 'fix'`, `"fix"`, `'it''s'`, `''`, `'yes'`, `"a\tb"` or `'a' # x`: each one currently produces exactly one non-repairable finding, `unsafe scalar shape with ambiguous decoded value; needs manual review`.
2. **The writer always single-quotes every string.** `document.String` → `serialize()` emits `'…'` with `'` doubled, for every input, including plain-safe text such as `héllo wörld`. It **never** emits a double-quoted scalar. The spec's parity row "a character that forces double-quoting if the writer ever emits it" therefore has no writer-produced instance. The double-quoted coverage comes from hand-authored tokens in Task 1: the live corpus's double-quoted tokens are hand-written.
3. **The spec's helper description is unsound if taken literally.** A single `yaml.Unmarshal` of the token into a `yaml.Node` **succeeds** on `'a' b`, `'a'b`, `'a' 'b'`, `"a" "b"` and `'a', b`. It returns one single-quoted scalar `a` and silently drops the trailing content. It also succeeds on the multi-document `'a'\n---\n'b'`. The helper must therefore use `yaml.NewDecoder` and require that a **second** `Decode` returns `io.EOF`. Probed: that second decode returns `did not find expected <document start>` for every trailing-content case, `<nil>` for the multi-document case, and `io.EOF` (matched by `errors.Is`) for every clean token.
4. **Malformed quoting never reaches `planQuote` through `PlanRepairs`.** A record with `title: 'abc` or `title: 'a' b` fails `document.Parse` and yields the single `record frontmatter is undecodable: …` finding (no `Field`). So the spec's "must go red if the helper is loosened to a first-byte check" mutation can only redden in a **direct helper test** (Task 1's `TestWellFormedQuotedString`), not through `PlanRepairs`. The `PlanRepairs`-level malformed asserts stay green under that mutation by construction, as the spec expects.
5. **Tagged / anchored quoted scalars:** `!!str 'a'` parses with `Style == TaggedStyle|SingleQuotedStyle` (5) and `&x 'a'` carries `Anchor == "x"`. `document.classifyShape` already refuses both (not `ShapeInline`), so they never reach `planQuote`. The helper still rejects them, so it stays correct on its own terms.
6. **The locator strips inline comments.** For `title: 'a' # note`, `f.Value` spans only `'a'`. A standalone `'a' # x` token also decodes cleanly (second `Decode` → `io.EOF`), so the helper accepts it either way.
7. **Live corpus baseline** (installed binary, `docket repository check --json` in `/Users/homer/dev/docket`, 2026-09-27): `frontmatter-manual-review` **914**, all with the message `unsafe scalar shape with ambiguous decoded value; needs manual review`. Other families: `artifact-links-stale` 275, `drop-terminal-claimed-at` 241, `local-metadata-diverged` 1, `metadata-worktree-dirty` 1. The last two are environmental and vary with the metadata worktree's state.

## Review Focus

The spec's silence on an input is not permission for it to break the program. Each line below names an input class the spec implies without enumerating a test for it. The pinning test lives in the named task.

1. **Trailing content after a closing quote** (`'a' b`, `'a'b`, `'a' 'b'`, `'a', b`) and **multi-document tokens**: these must be rejected by the helper even though a lone `yaml.Unmarshal` accepts them. Pinned in Task 1 `TestWellFormedQuotedString` (plan-time finding 3).
2. **Hand-authored double-quoted tokens with escapes** (`"a\tb"`, `""`): they are unambiguous strings whose decoded text differs from their bytes, so they must produce no finding. Pinned in Task 1 (helper table plus `TestRepairWellFormedQuotedScalarNoFinding`).
3. **A quoted value followed by an inline comment** (`title: 'a' # note`) must produce no finding. Pinned in Task 1 `TestRepairWellFormedQuotedScalarNoFinding`.
4. **Quoted boolean-looking and empty strings** (`'yes'`, `'true'`, `''`) must produce no finding, while bare `yes` stays **repairable** and bare `true` stays **manual-review**. Pinned in Task 1 (`TestRepairWellFormedQuotedScalarNoFinding` + `TestRepairBareScalarsKeepTheirVerdict`) and by the unchanged `TestRepairQuoteScalarEligible`.
5. **Tagged or anchored quoted scalars** (`!!str 'a'`, `&x 'a'`) and non-scalars (`['a']`, `'a': b`) must not be treated as a plain well-formed string. Pinned in Task 1 `TestWellFormedQuotedString`.

---

### Task 1: `wellFormedQuotedString` helper wired into `planQuote`

**Files:**
- Modify: `internal/reposetup/repair.go`: the import block, `planQuote`, and a new helper placed directly after `decodesToStringLiteral`
- Test: `internal/reposetup/repair_test.go`

**Interfaces:**
- Consumes: the existing `planOne(t, path, src, archived) []RepairFinding` and `hasRepairableCode` helpers in `repair_test.go`; `PlanRepairs`, `RepairQuoteScalar`.
- Produces: `func wellFormedQuotedString(token string) bool` (unexported, package `reposetup`). Tasks 2 and 3 do not call it directly. They observe it through `PlanRepairs`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/reposetup/repair_test.go`:

```go
// TestWellFormedQuotedString pins the helper's decision directly. It is the
// only layer where malformed quoting (unterminated, trailing content) reaches
// the helper: through PlanRepairs such a record is undecodable and never gets
// to planQuote. Mutation targets: a first-byte check, dropping the io.EOF
// second-decode check, and dropping the style check each redden rows here.
func TestWellFormedQuotedString(t *testing.T) {
	cases := []struct {
		token string
		want  bool
	}{
		// Well-formed quoted strings: exactly one quoted !!str scalar.
		{`'fix'`, true},
		{`"fix"`, true},
		{`''`, true},
		{`""`, true},
		{`'it''s'`, true},
		{`"a\tb"`, true}, // escape: decoded text differs from bytes, still unambiguous
		{`'yes'`, true},
		{`'a: b'`, true},
		{`'x #y'`, true},
		{`'a' # x`, true}, // trailing comment is not content; the locator strips it anyway

		// Plain scalars: not quoted, so not this helper's business.
		{``, false},
		{`fix`, false},
		{`true`, false},
		{`yes`, false},
		{`123`, false},

		// Malformed quoting.
		{`'abc`, false},
		{`"abc`, false},

		// Trailing content: a lone yaml.Unmarshal ACCEPTS every one of these.
		{`'a' b`, false},
		{`'a'b`, false},
		{`'a' 'b'`, false},
		{`"a" "b"`, false},
		{`'a', b`, false},
		{"'a'\n---\n'b'", false}, // multi-document

		// Quoted-but-not-a-plain-string shapes.
		{`!!str 'a'`, false}, // tagged
		{`&x 'a'`, false},    // anchored
		{`'a': b`, false},    // mapping
		{`['a']`, false},     // sequence
		{`'a' - b`, false},   // parse error
	}
	for _, tc := range cases {
		if got := wellFormedQuotedString(tc.token); got != tc.want {
			t.Errorf("wellFormedQuotedString(%q) = %v, want %v", tc.token, got, tc.want)
		}
	}
}

// TestRepairWellFormedQuotedScalarNoFinding is the regression for change 0447:
// a string field holding a well-formed quoted scalar (the canonical writer's
// output, or a hand-written double-quoted one) yields NO finding at all.
func TestRepairWellFormedQuotedScalarNoFinding(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"writer-shaped record", "---\nid: 7\nslug: 'my-slug'\ntitle: 'it''s: a #b'\ntype: 'fix'\n---\nbody\n"},
		{"single-quoted", "---\nid: 7\ntitle: 'fix'\n---\nbody\n"},
		{"double-quoted", "---\nid: 7\ntitle: \"fix\"\n---\nbody\n"},
		{"double-quoted escape", "---\nid: 7\ntitle: \"a\\tb\"\n---\nbody\n"},
		{"doubled apostrophe", "---\nid: 7\ntitle: 'it''s'\n---\nbody\n"},
		{"empty single-quoted", "---\nid: 7\ntitle: ''\n---\nbody\n"},
		{"empty double-quoted", "---\nid: 7\ntitle: \"\"\n---\nbody\n"},
		{"quoted boolean keyword", "---\nid: 7\ntitle: 'yes'\n---\nbody\n"},
		{"quoted true", "---\nid: 7\ntitle: 'true'\n---\nbody\n"},
		{"quoted leading indicator", "---\nid: 7\ntitle: '-lead'\n---\nbody\n"},
		{"inline comment after quote", "---\nid: 7\ntitle: 'a' # note\n---\nbody\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if fs := planOne(t, "docs/changes/active/0001-x.md", tc.src, false); len(fs) != 0 {
				t.Fatalf("want no findings for a well-formed quoted scalar, got %+v", fs)
			}
		})
	}
}

// TestRepairBareScalarsKeepTheirVerdict pins that the fix is scoped to quoted
// tokens: a bare "true" is still a manual-review finding on its field, and
// malformed quoting still yields a non-empty (undecodable-record) finding.
func TestRepairBareScalarsKeepTheirVerdict(t *testing.T) {
	t.Run("bare true stays manual review", func(t *testing.T) {
		fs := planOne(t, "docs/changes/active/0001-x.md", "---\nid: 7\ntitle: true\n---\nbody\n", false)
		if len(fs) != 1 {
			t.Fatalf("want exactly one finding, got %d: %+v", len(fs), fs)
		}
		f := fs[0]
		if f.Repairable || f.Field != "title" || !strings.Contains(f.Message, "needs manual review") {
			t.Fatalf("want a non-repairable manual-review finding on title, got %+v", f)
		}
	})
	for _, src := range []string{
		"---\nid: 7\ntitle: 'abc\n---\nbody\n",  // unterminated
		"---\nid: 7\ntitle: 'a' b\n---\nbody\n", // trailing content
	} {
		t.Run(fmt.Sprintf("malformed %q", src), func(t *testing.T) {
			fs := planOne(t, "docs/changes/active/0001-x.md", src, false)
			if len(fs) == 0 {
				t.Fatalf("malformed quoting must not be silently accepted; got no findings")
			}
			for _, f := range fs {
				if f.Repairable {
					t.Fatalf("malformed quoting must never be repairable: %+v", f)
				}
			}
		})
	}
}
```

Add `"fmt"` to the `repair_test.go` import block (it currently imports `bytes`, `strings`, `testing`).

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test -count=1 -run 'TestWellFormedQuotedString|TestRepairWellFormedQuotedScalarNoFinding|TestRepairBareScalarsKeepTheirVerdict' ./internal/reposetup/`
Expected: the build FAILS with `undefined: wellFormedQuotedString`. That is the red for the helper table. To see the behavioural red of the `PlanRepairs` table before the helper exists, temporarily comment out `TestWellFormedQuotedString`, re-run, and confirm that `TestRepairWellFormedQuotedScalarNoFinding` fails on every subtest with a `needs manual review` finding while `TestRepairBareScalarsKeepTheirVerdict` passes. Then uncomment.

- [ ] **Step 3: Write the minimal implementation**

In `internal/reposetup/repair.go`, extend the import block with `"errors"` and `"io"`. The resulting stdlib group is `crypto/sha256`, `encoding/hex`, `errors`, `fmt`, `io`, `strconv`, `strings`.

Add the helper directly after `decodesToStringLiteral`:

```go
// wellFormedQuotedString reports whether token, decoded on its own, is exactly
// one single- or double-quoted YAML string scalar and nothing else — a
// well-formed string whose value is unambiguous even though its first byte
// (' or ") is a YAML indicator. The decision is keyed on the PARSED node's
// kind, tag, anchor, and style, never on the raw first byte (learning
// byte-pattern-guard-matches-a-spelling). The second Decode must hit io.EOF:
// a lone yaml.Unmarshal stops after the first node and silently accepts
// trailing content such as "'a' b", or a second document.
func wellFormedQuotedString(token string) bool {
	dec := yaml.NewDecoder(strings.NewReader(token))
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		return false
	}
	var rest yaml.Node
	if err := dec.Decode(&rest); !errors.Is(err, io.EOF) {
		return false
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 {
		return false
	}
	n := doc.Content[0]
	if n.Kind != yaml.ScalarNode || n.Tag != "!!str" || n.Anchor != "" {
		return false
	}
	return n.Style == yaml.SingleQuotedStyle || n.Style == yaml.DoubleQuotedStyle
}
```

In `planQuote`, insert the call immediately after the `ShapeInline` gate and the `token` assignment, before `unsafeScalarShape`:

```go
	token := string(src[f.Value.Start:f.Value.End])
	if wellFormedQuotedString(token) {
		// Already a well-formed quoted string — the canonical writer's own
		// output (ADR-0071) or a hand-written quoted scalar. Its first byte is
		// an indicator, but its value is unambiguous: nothing to repair or review.
		return RepairFinding{}, false
	}
	if !unsafeScalarShape(token) {
```

Leave everything else in `planQuote` (and the rest of the file) byte-identical.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test -count=1 ./internal/reposetup/`
Expected: PASS, including the unchanged `TestRepairQuoteScalarEligible` (bare `yes` and `:30` stay repairable) and `TestRepairQuoteScalarRefused`.

- [ ] **Step 5: Mutation-test the guard (restore from backup every time)**

Run each probe separately. Each must go RED, and the named test must be the one that fails:

```bash
f=internal/reposetup/repair.go
# M1 — loosen the helper to a first-byte check. Expect TestWellFormedQuotedString RED ('abc, 'a' b, !!str … rows).
cp "$f" "$f.bak"
perl -0pi -e 's/(func wellFormedQuotedString\(token string\) bool \{\n)/$1\treturn token != "" \&\& (token[0] == \x27\\\x27\x27 || token[0] == \x27"\x27)\n/' "$f"
go test -count=1 -run TestWellFormedQuotedString ./internal/reposetup/ ; mv -f "$f.bak" "$f"

# M2 — drop the io.EOF second-decode check. Expect TestWellFormedQuotedString RED on the trailing-content rows.
cp "$f" "$f.bak"
perl -0pi -e 's/if err := dec\.Decode\(&rest\); !errors\.Is\(err, io\.EOF\) \{/if err := dec.Decode(\&rest); false \&\& !errors.Is(err, io.EOF) {/' "$f"
go test -count=1 -run TestWellFormedQuotedString ./internal/reposetup/ ; mv -f "$f.bak" "$f"

# M3 — drop the style check. Expect TestWellFormedQuotedString RED (plain !!str rows `fix`, `yes`) and the
# pre-existing TestRepairQuoteScalarEligible RED (bare yes / :30 no longer repairable). Bare `true` is NOT
# caught by M3: its tag is !!bool, so the tag check still rejects it (verified at plan time).
cp "$f" "$f.bak"
perl -0pi -e 's/return n\.Style == yaml\.SingleQuotedStyle \|\| n\.Style == yaml\.DoubleQuotedStyle/return true/' "$f"
go test -count=1 -run 'TestWellFormedQuotedString|TestRepairQuoteScalarEligible' ./internal/reposetup/ ; mv -f "$f.bak" "$f"

# M4 — remove the planQuote call (revert the fix). Expect TestRepairWellFormedQuotedScalarNoFinding RED.
cp "$f" "$f.bak"
perl -0pi -e 's/if wellFormedQuotedString\(token\) \{/if false \&\& wellFormedQuotedString(token) {/' "$f"
go test -count=1 -run TestRepairWellFormedQuotedScalarNoFinding ./internal/reposetup/ ; mv -f "$f.bak" "$f"
```

Before believing any red, prove the mutation landed. Run `grep -n` for the mutated text in each probe before the test run, or read `git diff` against the backup. After each probe, `git diff --stat` must show only this task's intended edits. If a probe stays green, stop: the guard is decoration. Investigate it and do not record it as a limitation (learning `residual-is-for-undetectable-not-unprobed`). Then run `go test -count=1 ./internal/reposetup/` once more and expect PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/reposetup/repair.go internal/reposetup/repair_test.go
git commit -m "fix(reposetup): treat well-formed quoted scalars as clean in planQuote (change 0447)"
```

---

### Task 2: Writer/checker parity table over adversarial strings

**Files:**
- Test: `internal/reposetup/repair_test.go`

**Interfaces:**
- Consumes: `planOne` (test helper), the unexported `applyValue(doc document.Document, field string, v document.Value) ([]byte, error)` in `repair.go` (the PatchSet path the repair roster itself uses to write), `document.New([]document.FieldSpec, body string) ([]byte, error)`, `document.FieldSpec{Name, Value}`, `document.String`, `document.Int`, `document.Parse`, and the `DecodeFrontmatter(any) error` method on `document.Document`.
- Produces: nothing consumed later.

- [ ] **Step 1: Write the parity test**

Append to `internal/reposetup/repair_test.go`:

```go
// parityStrings are adversarial title values: every YAML indicator a plain
// scalar may not start with, the boolean/null/number/date keywords, embedded
// ": " / " #" / trailing ":", apostrophes and double quotes, escapes-looking
// bytes, and non-ASCII. The canonical writer always single-quotes a string
// (document.String), so no row can make it emit a double-quoted scalar; the
// double-quoted shape is covered by TestRepairWellFormedQuotedScalarNoFinding.
var parityStrings = []string{
	"", "yes", "no", "true", "false", "on", "off", "null", "~", "123", "1.5", "2026-01-01",
	"it's", "''", "a' b", "\"dq\"", "a: b", "trailing:", "x #y", "#hash",
	"-lead", "?q", ":30", ",comma", "[x]", "]x", "{x}", "}x", "&anchor", "*alias",
	"!tag", "|pipe", ">fold", "%pct", "@at", "`tick`",
	"back\\slash", "tab\there", "héllo wörld — ✓",
}

// TestRepairWriterCheckerParity proves the checker agrees with the writer
// (ADR-0071): whatever string the canonical writer renders into a string
// field, PlanRepairs reports NO finding for it — through both write paths,
// document.New (a brand-new record) and PatchSet via applyValue (rewriting an
// existing field). Red on revert of change 0447's planQuote fix.
func TestRepairWriterCheckerParity(t *testing.T) {
	const path = "docs/changes/active/0007-x.md"
	for _, s := range parityStrings {
		t.Run(fmt.Sprintf("%q", s), func(t *testing.T) {
			built, err := document.New([]document.FieldSpec{
				{Name: "id", Value: document.Int(7)},
				{Name: "slug", Value: document.String("x")},
				{Name: "title", Value: document.String(s)},
				{Name: "type", Value: document.String("fix")},
			}, "body\n")
			if err != nil {
				t.Fatalf("document.New(%q): %v", s, err)
			}
			assertParity(t, path, built, s)

			doc, err := document.Parse([]byte("---\nid: 7\ntitle: placeholder\n---\nbody\n"))
			if err != nil {
				t.Fatalf("parse seed: %v", err)
			}
			patched, err := applyValue(doc, "title", document.String(s))
			if err != nil {
				t.Fatalf("applyValue(%q): %v", s, err)
			}
			assertParity(t, path, patched, s)
		})
	}
}

// assertParity checks one writer-rendered record: the title round-trips to s
// (so the row really exercised s), the token is quoted (so the row really
// exercised the quoted path), and PlanRepairs finds nothing.
func assertParity(t *testing.T, path string, rec []byte, s string) {
	t.Helper()
	doc, err := document.Parse(rec)
	if err != nil {
		t.Fatalf("writer output does not reparse: %v\n%s", err, rec)
	}
	var fm struct {
		Title string `yaml:"title"`
	}
	if err := doc.DecodeFrontmatter(&fm); err != nil || fm.Title != s {
		t.Fatalf("title round-trip = %q (err %v), want %q", fm.Title, err, s)
	}
	if !bytes.Contains(rec, []byte("\ntitle: '")) {
		t.Fatalf("writer did not single-quote the title; parity row is vacuous:\n%s", rec)
	}
	if fs := planOne(t, path, string(rec), false); len(fs) != 0 {
		t.Fatalf("checker disagrees with writer for %q: %+v\n%s", s, fs, rec)
	}
}
```

Add `"github.com/danielhanold/docket/internal/document"` to the `repair_test.go` import block, in a second import group after the stdlib group.

- [ ] **Step 2: Run it (green expected with Task 1 in place)**

Run: `go test -count=1 -run TestRepairWriterCheckerParity ./internal/reposetup/`
Expected: PASS for every row. If `document.New` or `applyValue` rejects a row (for example, a control character the writer refuses by design), drop only that row and say why in a one-line comment next to `parityStrings`. Do not weaken `assertParity`. Plan-time probing found all of `""`, `yes`, `true`, `it's`, `a: b`, `x #y`, `-lead`, `[x]`, `:30`, `héllo wörld`, `tab\there`, `back\\slash`, `"dq"`, `null`, `~`, `123` and `2026-01-01` accepted by `document.New`.

- [ ] **Step 3: Mutation-test: revert the fix and watch the table redden**

```bash
f=internal/reposetup/repair.go
cp "$f" "$f.bak"
perl -0pi -e 's/if wellFormedQuotedString\(token\) \{/if false \&\& wellFormedQuotedString(token) {/' "$f"
grep -n 'if false && wellFormedQuotedString' "$f"   # prove the mutation landed
go test -count=1 -run TestRepairWriterCheckerParity ./internal/reposetup/   # expect RED on (nearly) every row
mv -f "$f.bak" "$f"
go test -count=1 -run TestRepairWriterCheckerParity ./internal/reposetup/   # expect PASS again
```

A second probe checks that the vacuity guard is load-bearing. Temporarily change `document.String(s)` to `document.String("safe")` in the `document.New` call and expect RED from the round-trip assert. Restore it with a backup copy of `repair_test.go`, the same way.

- [ ] **Step 4: Commit**

```bash
git add internal/reposetup/repair_test.go
git commit -m "test(reposetup): writer/checker parity table for quoted string fields (change 0447)"
```

---

### Task 3: End-to-end `change.create` → zero repair findings

**Files:**
- Test: `internal/app/change_create_test.go` (package `app`, no build tag, so it runs in the default partition)

**Interfaces:**
- Consumes: the existing `planFor(t, files, op) (transaction.MutationPlan, transaction.OperationResult)`, `baseOp(surfaces []string) changeCreateOp`, `fixtureChange(id int, slug string) string` (in `internal/app/planning_test.go`), `plan.Files[i].Path` / `.Bytes` (as already used by `TestChangeCreatePlanFillsArtifactBlockForADRs`), `reposetup.PlanRepairs(path string, src []byte, archived bool) ([]RepairFinding, error)`, `document.Parse`, and `Document.DecodeFrontmatter`.
- Produces: nothing consumed later.

- [ ] **Step 1: Write the test**

Append to `internal/app/change_create_test.go`:

```go
// TestChangeCreateRecordHasNoRepairFindings is change 0447's end-to-end guard:
// a record written by change.create — with an adversarial title carrying a
// leading "-", ": ", an apostrophe, " #", and the word "yes" — is reported
// clean by the repair planner that feeds `docket repository check` and the
// `repository migrate` preview. Before the fix every writer-quoted string
// field (slug, title, type, …) produced a frontmatter-manual-review finding.
func TestChangeCreateRecordHasNoRepairFindings(t *testing.T) {
	const title = "-lead: it's a #tag, yes"
	files := map[string]string{
		"docs/changes/active/0001-first.md": fixtureChange(1, "first"),
	}
	op := baseOp([]string{})
	op.req.Title = title
	plan, opRes := planFor(t, files, op)
	if opRes.Refused {
		t.Fatalf("unexpected refusal: %v", opRes.Findings)
	}
	var recPath string
	var rec []byte
	for _, f := range plan.Files {
		if strings.HasSuffix(string(f.Path), "0002-add-a-widget.md") {
			recPath, rec = string(f.Path), f.Bytes
		}
	}
	if rec == nil {
		t.Fatal("new record not planned")
	}

	// The adversarial title really landed (the guard is not vacuous).
	doc, err := document.Parse(rec)
	if err != nil {
		t.Fatalf("written record does not parse: %v\n%s", err, rec)
	}
	var fm struct {
		Title string `yaml:"title"`
	}
	if err := doc.DecodeFrontmatter(&fm); err != nil || fm.Title != title {
		t.Fatalf("title = %q (err %v), want %q", fm.Title, err, title)
	}

	fs, err := reposetup.PlanRepairs(recPath, rec, false)
	if err != nil {
		t.Fatalf("PlanRepairs: %v", err)
	}
	if len(fs) != 0 {
		t.Fatalf("change.create output must have zero repair findings, got %+v\n%s", fs, rec)
	}
}
```

Add these to the `change_create_test.go` import block if they are absent: `"github.com/danielhanold/docket/internal/document"` and `"github.com/danielhanold/docket/internal/reposetup"`. `strings` is already imported. Check the block and keep gofmt grouping.

- [ ] **Step 2: Run it (green expected with Task 1 in place)**

Run: `go test -count=1 -run TestChangeCreateRecordHasNoRepairFindings ./internal/app/`
Expected: PASS. If `op.req.Title` is rejected by request validation (for example, a title rule this plan did not see), keep every required property (a leading `-`, `: `, an apostrophe, ` #`, `yes`) and rearrange only the surrounding text. The `planFor` path takes the slug from `op.slug` (`add-a-widget`), so slug derivation does not depend on the title.

- [ ] **Step 3: Mutation-test: revert the fix and watch it redden**

```bash
f=internal/reposetup/repair.go
cp "$f" "$f.bak"
perl -0pi -e 's/if wellFormedQuotedString\(token\) \{/if false \&\& wellFormedQuotedString(token) {/' "$f"
grep -n 'if false && wellFormedQuotedString' "$f"
go test -count=1 -run TestChangeCreateRecordHasNoRepairFindings ./internal/app/   # expect RED: slug/title/type manual-review findings
mv -f "$f.bak" "$f"
go test -count=1 -run TestChangeCreateRecordHasNoRepairFindings ./internal/app/   # expect PASS
```

`-count=1` is mandatory here: the mutated file is in a *different* package from the test (learning `cached-runner-serves-a-mutated-tree`).

- [ ] **Step 4: Commit**

```bash
git add internal/app/change_create_test.go
git commit -m "test(app): change.create output has zero repair findings (change 0447)"
```

---

## Post-build verification (results artifact, not a test, no commit)

This belongs to the build's evidence and the change's results file (written by the implement-next flow, not by these tasks). After the full-suite gate is green, build the branch binary into a scratch path and run the report-only check against the live corpus in the primary tree:

```bash
bin="$(mktemp -d "${TMPDIR:-/tmp}/docket-0447.XXXXXX")/docket"
go build -o "$bin" ./cmd/docket
out="$(cd /Users/homer/dev/docket && "$bin" repository check --json)"   # exit 1 is expected: findings exist
jq -r '.findings | group_by(.code) | map("\(.[0].code) \(length)") | .[]' <<<"$out"
```

Record the result against the plan-time baseline (finding 7): `frontmatter-manual-review` **914 → expected 0**; `artifact-links-stale` (275) and `drop-terminal-claimed-at` (241) unchanged. Newly created records may shift those two by a few; explain any delta. `local-metadata-diverged` / `metadata-worktree-dirty` are environmental. If `frontmatter-manual-review` is not 0, list the residual findings' `ref` and `message`, read each record's token, and classify it. A residual that is a genuine plain-scalar case is correct behaviour. A residual that is a quoted token is a defect in this change and must be fixed before the PR. `repository check` is report-only. Do **not** run `repository migrate` or any repair here.

## Self-review

- **Spec coverage:**
  - Helper and `planQuote` wiring: Task 1.
  - Test 1 (e2e `change.create`): Task 3.
  - Test 2 (parity table, incl. the writer-never-double-quotes note): Task 2.
  - Test 3 (bare `true` still flagged; `'abc` / `'a' b` not silently accepted; first-byte mutation reddens): Task 1. The first-byte mutation can redden only at the helper level (plan-time finding 4).
  - Test 4 (existing eligible cases unchanged): Task 1 Step 4.
  - Live-corpus verification: the post-build section.
  - Out-of-scope items: Global Constraints.
- **Deviation from the spec's letter, with its reason:** the spec says to decode with a single unmarshal and to let trailing content fail the decode. Probing shows a single `yaml.Unmarshal` does **not** fail on trailing content, so the helper uses a `yaml.Decoder` plus an `io.EOF` second decode (plan-time finding 3). The helper also requires `Tag == "!!str"`, an empty `Anchor`, and an **exact** style match rather than a style bit test. That is stricter than the spec and rejects tagged or anchored quoted scalars, which cannot reach `planQuote` anyway.
- **Supplied test code verified at plan time** (learning `plan-supplied-test-code-is-unverified`). I assembled every Go block in this plan into scratch copies of `repair.go`, `repair_test.go` and `change_create_test.go` and ran them through `go test -count=1 -overlay …`, so the worktree was left untouched. With the fix, `./internal/reposetup/` and `TestChangeCreate*` in `./internal/app/` pass and gofmt is clean. Mutation outcomes:
  - M1 (first-byte check) and M2 (no `io.EOF` check) redden `TestWellFormedQuotedString`.
  - M3 (no style check) reddens `TestWellFormedQuotedString` and `TestRepairQuoteScalarEligible`.
  - M4 (fix reverted) reddens `TestRepairWellFormedQuotedScalarNoFinding`, `TestRepairWriterCheckerParity` and `TestChangeCreateRecordHasNoRepairFindings`. The last one reports three findings: `slug`, `title`, `type`.

  The builder must still re-run these probes, because the verification was against scratch copies.
- **Type consistency:** `wellFormedQuotedString(token string) bool`, `applyValue(document.Document, string, document.Value) ([]byte, error)` and `PlanRepairs(string, []byte, bool) ([]RepairFinding, error)` match the code at HEAD `e244dfd26`.
