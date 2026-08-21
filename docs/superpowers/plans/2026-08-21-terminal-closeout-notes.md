<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **Change 0330 — Optional closeout notes preserve post-merge verification without rewriting frozen results** — `docs/changes/active/0330-post-merge-results-appending-has-no-home-in-the-go-runtime-f.md`
<!-- docket:backlink:end -->
# Terminal Closeout Notes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use docket-build to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give `docket finalize closeout` an optional structured notes request (`verification_outcomes` / `late_findings`) rendered as a `## Closeout notes` terminal section in the same transaction as closeout, idempotency-keyed, so post-merge verification context is preserved without rewriting frozen results files.

**Architecture:** A new focused renderer (`internal/render/closeout_notes.go`) owns all Markdown structure for the section; a new app-layer file (`internal/app/finalize_closeout_notes.go`) owns the payload type, validation/normalization, the rendered-bytes digest, and the splice helper built on the existing marker/fence-aware `render.ApplySectionEdits`. `FinalizeCloseout` gains a notes parameter carried through both the archive and stacked closeout operations, bound into the `closeoutReceipt`, with terminal-state replay/refusal decided by re-splicing against the terminal record's own bytes (the writer is the reader). The CLI adds an optional `--input` through the existing strict `decodeInputFlag`. The finalize skill routes invocation-supplied notes into the request file; the convention documents the section; embedded assets are regenerated mechanically.

**Tech Stack:** Go (cobra CLI, internal/render + internal/document + transaction engine), Bash contract tests, `go generate ./internal/assets`.

**Spec:** `docs/superpowers/specs/2026-08-21-terminal-closeout-notes-design.md`

## Global Constraints

- Merged `results:` files stay immutable; this change never writes a linked results file (spec: *Problem*, *Out of scope*).
- Exactly two authored categories: `verification_outcomes` and `late_findings` — no third category, no free-form Markdown (spec: *Request and command surface*).
- Go, not the caller, owns all Markdown structure; authored text cannot escape its bullet or forge a sibling `##` section (spec: *Rendering and document ownership*).
- Notes land in the SAME transaction as the closeout lifecycle mutation; invalid notes or failed preconditions produce zero mutation (spec: *Closeout transaction and lifecycle behavior*).
- No input and explicitly-empty input canonicalize identically; identical-notes retries replay; different notes against a terminal record are refused (spec: *Idempotency and recovery*).
- Notes never propagate to carried stacked descendants; one note owner across both closeout dispositions (spec: *Closeout transaction and lifecycle behavior*).
- No post-merge pause, prompt, checkpoint, or lifecycle state in the finalize skill (spec: *Finalize skill behavior*).
- Never hand-edit generated embedded-asset copies; regenerate mechanically after authored skill edits.
- Go mutation probes and manual re-verification runs use `-count=1` (learnings: `cached-runner-serves-a-mutated-tree`); Bash mutation probes mutate a temp COPY, never the working file (learnings: `mutation-restore-needs-a-backup-copy`).
- Suite gate: run the whole configured suite (`finalize.test_command` in `.docket.yml` resolves to `scripts/run-tests.sh`); investigate every trailing `OVER BUDGET:` line.

## Reality check (verified against this branch at plan time)

- `internal/cli/finalize.go` — `newFinalizeCloseoutSubcommand` carries `--id`/`--repo-dir` only. The strict decoder to reuse is `decodeInputFlag` (`internal/cli/change.go`): DisallowUnknownFields, exactly-one-document, `-` for stdin.
- `internal/app/finalize_closeout.go` — `FinalizeCloseout(ctx, deps, repoDir, id)`; ops `closeoutArchiveOp` (root + carried descendants; targets[0] is the explicit change) and `closeoutStackedOp` (in place); receipt struct `closeoutReceipt` with alphabetical JSON keys (`archive_date`, `ids`, `op`, `root`); early `StatusDone` → `CloseoutDispAlready` no-op short circuit; `closeoutStacked` has its own already-stacked-merged no-op.
- Byte-size bound for authored input: `boundAuthored` + `maxAuthoredMarkdownBytes` (`internal/app/change_reconcile.go`).
- `render.ApplySectionEdits` (`internal/render/section.go`) is H2-granular, fence-aware, appends an absent replace-target at EOF preceded by one blank line — exactly where the terminal section belongs. There is NO existing safe bullet renderer; a focused one is added (spec *Components* allows exactly this).
- `tests/test_results_artifact.sh` — the skipped Bash-era post-merge-append assertion is the comment block starting `# HARD STOP — DO NOT RETIRE (0316 plan Task 20)` plus the `printf 'skip - finalize post-merge results appending …'` line.
- `skills/docket-finalize-change/SKILL.md` `### 9. Closeout` invokes `docket finalize closeout --id <id>`; the next H3 is `### 10. Cleanup` (a named terminator for section slicing).
- Go closeout tests live in `internal/app/finalize_closeout_test.go` on a real-git fixture (`setupCloseoutFixture`, `mergeIntoBase`, `baselineMergedFake`, `originFile`, `originTip`, `planRepoModes()` covering docket and main modes; the fixture merge date is 2026-08-18, archive path `docs/changes/archive/2026-08-18-0005-widget.md`).
- Embedded assets regenerate with `go generate ./internal/assets` (the `//go:generate go run ../../cmd/genassets -repo ../..` directive).

---

### Task 1: Closeout-notes renderer

**Files:**
- Create: `internal/render/closeout_notes.go`
- Test: `internal/render/closeout_notes_test.go`

**Interfaces:**
- Produces: `render.CloseoutNotesHeading` (const `"## Closeout notes"`) and `render.CloseoutNotesBody(verification, late []string) string` — the section BODY without the heading line, exactly what `SectionEdit.Markdown` takes. Later tasks rely on both names.

- [ ] **Step 1: Write the failing test**

```go
package render

import "testing"

func TestCloseoutNotesBody(t *testing.T) {
	cases := []struct {
		name         string
		verification []string
		late         []string
		want         string
	}{
		{
			name:         "both categories",
			verification: []string{"Production health check passed after deployment"},
			late:         []string{"The upgrade guide should mention the legacy config cleanup"},
			want: "### Verification\n\n" +
				"- Production health check passed after deployment\n\n" +
				"### Late findings\n\n" +
				"- The upgrade guide should mention the legacy config cleanup",
		},
		{
			name:         "verification only omits the late subsection",
			verification: []string{"Smoke test green", "Rollback drill passed"},
			want: "### Verification\n\n" +
				"- Smoke test green\n" +
				"- Rollback drill passed",
		},
		{
			name: "late only omits the verification subsection",
			late: []string{"Docs gap"},
			want: "### Late findings\n\n- Docs gap",
		},
		{
			name: "both empty renders nothing",
			want: "",
		},
		{
			name:         "multiline continuation is indented so it cannot escape the bullet",
			verification: []string{"line one\n## not a heading\nline three"},
			want: "### Verification\n\n" +
				"- line one\n" +
				"  ## not a heading\n" +
				"  line three",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := CloseoutNotesBody(tc.verification, tc.late)
			if got != tc.want {
				t.Fatalf("CloseoutNotesBody = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestCloseoutNotesBodySpliceRoundTrip proves the rendered section survives
// ApplySectionEdits and lands as the FINAL section: the writer's own reader
// accepts what it emits (learnings: validator-must-match-the-reader-it-feeds).
func TestCloseoutNotesBodySpliceRoundTrip(t *testing.T) {
	src := []byte("---\nid: 5\n---\n\n## Why\n\nBecause.\n")
	body := CloseoutNotesBody([]string{"ok\n## fake"}, nil)
	out, err := ApplySectionEdits(src, []string{CloseoutNotesHeading},
		[]SectionEdit{{Heading: CloseoutNotesHeading, Intent: SectionReplace, Markdown: body}})
	if err != nil {
		t.Fatalf("splice: %v", err)
	}
	heads := scanH2Headings(out)
	if len(heads) != 2 || heads[1].heading != CloseoutNotesHeading {
		t.Fatalf("headings = %+v, want [## Why, %s] with notes last", heads, CloseoutNotesHeading)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/render/ -run 'TestCloseoutNotes' -count=1`
Expected: FAIL — `undefined: CloseoutNotesBody` / `undefined: CloseoutNotesHeading`.

- [ ] **Step 3: Write the implementation**

```go
package render

import "strings"

// CloseoutNotesHeading is the terminal change-body section `finalize closeout`
// owns. It is the final authored body section of a terminal record; the
// convention documents it and closeout is its only writer.
const CloseoutNotesHeading = "## Closeout notes"

// closeoutNotesSubheadings, in fixed render order.
const (
	closeoutVerificationSubheading = "### Verification"
	closeoutLateFindingsSubheading = "### Late findings"
)

// CloseoutNotesBody renders the `## Closeout notes` section BODY (no heading
// line): a `### Verification` bullet list, then a `### Late findings` bullet
// list, each omitted when its category is empty; both empty renders "". Each
// entry is one bullet; continuation lines are indented two spaces so authored
// text can never sit at column 0 and forge a sibling `##` section — Go, not
// the caller, owns all Markdown structure.
func CloseoutNotesBody(verification, late []string) string {
	var parts []string
	if s := closeoutBulletList(closeoutVerificationSubheading, verification); s != "" {
		parts = append(parts, s)
	}
	if s := closeoutBulletList(closeoutLateFindingsSubheading, late); s != "" {
		parts = append(parts, s)
	}
	return strings.Join(parts, "\n\n")
}

// closeoutBulletList renders one subheading plus its bullets, "" when empty.
func closeoutBulletList(subheading string, entries []string) string {
	if len(entries) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(subheading)
	b.WriteString("\n")
	for _, e := range entries {
		lines := strings.Split(e, "\n")
		b.WriteString("\n- ")
		b.WriteString(lines[0])
		for _, cont := range lines[1:] {
			b.WriteString("\n  ")
			b.WriteString(cont)
		}
	}
	return b.String()
}
```

Note the exact blank-line shape: `### Verification\n\n- first` and single newlines between sibling bullets — match the test's `want` strings byte for byte.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/render/ -count=1`
Expected: PASS (whole package — the round-trip test uses package internals).

- [ ] **Step 5: Commit**

```bash
git add internal/render/closeout_notes.go internal/render/closeout_notes_test.go
git commit -m "feat(0330): closeout-notes section renderer owns all Markdown structure"
```

---

### Task 2: Notes payload — type, validation, digest, splice helper

**Files:**
- Create: `internal/app/finalize_closeout_notes.go`
- Test: `internal/app/finalize_closeout_notes_test.go`

**Interfaces:**
- Consumes: `render.CloseoutNotesHeading`, `render.CloseoutNotesBody` (Task 1); `boundAuthored`, `maxAuthoredMarkdownBytes`, `lifecycleFinding`, `StatusFinding` (existing app helpers).
- Produces (later tasks rely on these exact names):
  - `type CloseoutNotes struct { VerificationOutcomes []string; LateFindings []string }`
  - `func (n CloseoutNotes) Empty() bool`
  - `func normalizeCloseoutNotes(n CloseoutNotes) (CloseoutNotes, []StatusFinding)`
  - `func closeoutNotesDigest(n CloseoutNotes) string` — "" for empty notes, else sha256 hex of the rendered section body
  - `func spliceCloseoutNotes(src []byte, n CloseoutNotes) ([]byte, error)` — src unchanged (same slice) when `n.Empty()`

- [ ] **Step 1: Write the failing test**

```go
package app

import (
	"strings"
	"testing"
)

func TestNormalizeCloseoutNotes(t *testing.T) {
	t.Run("valid entries are trimmed, order preserved, empties canonicalize", func(t *testing.T) {
		n, findings := normalizeCloseoutNotes(CloseoutNotes{
			VerificationOutcomes: []string{"  a  ", "b"},
			LateFindings:         []string{},
		})
		if len(findings) != 0 {
			t.Fatalf("findings = %+v, want none", findings)
		}
		if len(n.VerificationOutcomes) != 2 || n.VerificationOutcomes[0] != "a" || n.VerificationOutcomes[1] != "b" {
			t.Fatalf("normalized = %+v", n.VerificationOutcomes)
		}
		if n.LateFindings != nil {
			t.Fatalf("empty list must canonicalize to nil, got %+v", n.LateFindings)
		}
	})

	t.Run("no input and explicitly empty input canonicalize identically", func(t *testing.T) {
		a, _ := normalizeCloseoutNotes(CloseoutNotes{})
		b, _ := normalizeCloseoutNotes(CloseoutNotes{VerificationOutcomes: []string{}, LateFindings: []string{}})
		if !a.Empty() || !b.Empty() {
			t.Fatalf("both must be Empty: %+v %+v", a, b)
		}
		if closeoutNotesDigest(a) != "" || closeoutNotesDigest(b) != "" {
			t.Fatalf("empty notes must digest to the empty string")
		}
	})

	invalid := []struct {
		name  string
		notes CloseoutNotes
		code  string
	}{
		{"entry empty after trimming is invalid not dropped",
			CloseoutNotes{VerificationOutcomes: []string{"  "}}, "empty-note-entry"},
		{"control character rejected",
			CloseoutNotes{LateFindings: []string{"bad\x07bell"}}, "invalid-note-entry"},
		{"carriage return rejected",
			CloseoutNotes{LateFindings: []string{"bad\r\nline"}}, "invalid-note-entry"},
		{"managed-marker text rejected",
			CloseoutNotes{VerificationOutcomes: []string{"x <!-- docket:artifacts:start --> y"}}, "invalid-note-entry"},
		{"oversized entry rejected",
			CloseoutNotes{VerificationOutcomes: []string{strings.Repeat("a", maxAuthoredMarkdownBytes+1)}}, "authored-input-too-large"},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			_, findings := normalizeCloseoutNotes(tc.notes)
			found := false
			for _, f := range findings {
				if f.Code == tc.code {
					found = true
				}
			}
			if !found {
				t.Fatalf("findings = %+v, want code %q", findings, tc.code)
			}
		})
	}
}

func TestCloseoutNotesDigestKeysTheRenderedSection(t *testing.T) {
	a := CloseoutNotes{VerificationOutcomes: []string{"x"}}
	b := CloseoutNotes{VerificationOutcomes: []string{"x"}}
	c := CloseoutNotes{VerificationOutcomes: []string{"y"}}
	d := CloseoutNotes{LateFindings: []string{"x"}} // same text, other category
	if closeoutNotesDigest(a) != closeoutNotesDigest(b) {
		t.Fatalf("identical notes must digest identically")
	}
	if closeoutNotesDigest(a) == closeoutNotesDigest(c) || closeoutNotesDigest(a) == closeoutNotesDigest(d) {
		t.Fatalf("different notes must digest differently")
	}
}

func TestSpliceCloseoutNotes(t *testing.T) {
	src := []byte("---\nid: 5\n---\n\n## Why\n\nBecause.\n")
	t.Run("empty notes leave the record byte-identical", func(t *testing.T) {
		out, err := spliceCloseoutNotes(src, CloseoutNotes{})
		if err != nil || string(out) != string(src) {
			t.Fatalf("empty splice changed bytes or errored: %v", err)
		}
	})
	t.Run("notes land as the final section", func(t *testing.T) {
		out, err := spliceCloseoutNotes(src, CloseoutNotes{VerificationOutcomes: []string{"ok"}})
		if err != nil {
			t.Fatalf("splice: %v", err)
		}
		want := "## Closeout notes\n\n### Verification\n\n- ok\n"
		if !strings.HasSuffix(string(out), want) {
			t.Fatalf("record does not end with the notes section:\n%s", out)
		}
	})
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/app/ -run 'CloseoutNotes|SpliceCloseoutNotes' -count=1`
Expected: FAIL — `undefined: CloseoutNotes` etc.

- [ ] **Step 3: Write the implementation**

```go
package app

// finalize_closeout_notes.go — the optional authored closeout-notes payload:
// its normalized shape, its validation, the digest that binds it into the
// closeout receipt, and the splice that lands it as the terminal record's
// final authored body section. The renderer (render.CloseoutNotesBody) owns
// every Markdown byte; nothing here concatenates caller text into structure.

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/danielhanold/docket/internal/render"
)

// CloseoutNotes is the normalized optional payload `finalize closeout`
// accepts: exactly two ordered lists. Nil and empty lists are canonically
// identical (both normalize to nil), so "no input" and "explicitly empty
// input" key the same promise.
type CloseoutNotes struct {
	VerificationOutcomes []string
	LateFindings         []string
}

// Empty reports whether the notes carry no entries.
func (n CloseoutNotes) Empty() bool {
	return len(n.VerificationOutcomes) == 0 && len(n.LateFindings) == 0
}

// normalizeCloseoutNotes trims each entry and validates the whole input set
// before anything acts on any of it (learnings: validate-the-whole-input-set-
// first). An entry empty after trimming is invalid, never silently dropped;
// entries carrying C0 control characters other than '\n' and '\t' (or DEL),
// or reserved managed-marker text, are rejected; each entry and the rendered
// whole are bounded by the shared authored-input bound. Order is preserved.
func normalizeCloseoutNotes(n CloseoutNotes) (CloseoutNotes, []StatusFinding) {
	var findings []StatusFinding
	norm := func(label string, entries []string) []string {
		if len(entries) == 0 {
			return nil // canonical: empty and absent are the same request
		}
		out := make([]string, 0, len(entries))
		for i, e := range entries {
			t := strings.TrimSpace(e)
			if t == "" {
				findings = append(findings, lifecycleFinding("empty-note-entry",
					fmt.Sprintf("%s[%d] is empty after trimming; drop the entry or write one", label, i)))
				continue
			}
			if reason := invalidNoteText(t); reason != "" {
				findings = append(findings, lifecycleFinding("invalid-note-entry",
					fmt.Sprintf("%s[%d] %s", label, i, reason)))
				continue
			}
			boundAuthored(&findings, fmt.Sprintf("%s[%d]", label, i), t)
			out = append(out, t)
		}
		return out
	}
	res := CloseoutNotes{
		VerificationOutcomes: norm("verification_outcomes", n.VerificationOutcomes),
		LateFindings:         norm("late_findings", n.LateFindings),
	}
	boundAuthored(&findings, "closeout notes",
		render.CloseoutNotesBody(res.VerificationOutcomes, res.LateFindings))
	return res, findings
}

// invalidNoteText names why an entry cannot be rendered safely, or "" when it
// can. '\n' is legal (a multiline bullet; the renderer indents continuations)
// and '\t' is legal content; every other control character — including '\r',
// which would smuggle a CR into an LF document — corrupts the record.
// Managed-marker text is rejected so an entry can never open or close a
// generated block.
func invalidNoteText(t string) string {
	for _, r := range t {
		if (r < 0x20 && r != '\n' && r != '\t') || r == 0x7f {
			return "carries a control character that could corrupt the record"
		}
	}
	if strings.Contains(t, "<!-- docket:") {
		return "carries reserved managed-marker text"
	}
	return ""
}

// closeoutNotesDigest keys the promise being made: the exact rendered section
// body. Empty notes digest to "" so a no-notes receipt is byte-identical to a
// pre-notes-era receipt (learnings: idempotency-keying — key on the promised
// state, never a proxy).
func closeoutNotesDigest(n CloseoutNotes) string {
	if n.Empty() {
		return ""
	}
	sum := sha256.Sum256([]byte(render.CloseoutNotesBody(n.VerificationOutcomes, n.LateFindings)))
	return hex.EncodeToString(sum[:])
}

// closeoutNotesHeadingSet is the closed owned-heading set the notes splice may
// touch.
var closeoutNotesHeadingSet = []string{render.CloseoutNotesHeading}

// spliceCloseoutNotes lands the rendered section as the record's final
// authored body section via the marker/fence-aware section editor (append-at-
// EOF when absent, replace when present). Empty notes return src unchanged —
// today's closeout stays byte-for-byte identical.
func spliceCloseoutNotes(src []byte, n CloseoutNotes) ([]byte, error) {
	if n.Empty() {
		return src, nil
	}
	return render.ApplySectionEdits(src, closeoutNotesHeadingSet, []render.SectionEdit{{
		Heading:  render.CloseoutNotesHeading,
		Intent:   render.SectionReplace,
		Markdown: render.CloseoutNotesBody(n.VerificationOutcomes, n.LateFindings),
	}})
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/app/ -run 'CloseoutNotes|SpliceCloseoutNotes' -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/app/finalize_closeout_notes.go internal/app/finalize_closeout_notes_test.go
git commit -m "feat(0330): closeout-notes payload — validation, rendered-bytes digest, terminal splice"
```

---

### Task 3: Carry notes through both closeout paths, keyed into the receipt

**Files:**
- Modify: `internal/app/finalize_closeout.go`
- Modify: every existing `FinalizeCloseout(` call site (tests): `internal/app/finalize_closeout_test.go`, plus any hit from `grep -rn "FinalizeCloseout(" --include='*.go' internal/ cmd/` — derive the list from that grep, never from this enumeration.
- Test: `internal/app/finalize_closeout_test.go` (extend)

**Interfaces:**
- Consumes: `CloseoutNotes`, `normalizeCloseoutNotes`, `closeoutNotesDigest`, `spliceCloseoutNotes` (Task 2).
- Produces: `FinalizeCloseout(ctx context.Context, deps FinalizeDeps, repoDir string, id int, notes CloseoutNotes) CloseoutResult` — the new signature Task 4's CLI calls. New stable reason `ReasonCloseoutNotesFrozen = "terminal-notes-frozen"`.

**Semantics to implement (from the spec, in order of the existing flow):**

1. At the top of `FinalizeCloseout`, after the `id <= 0` check, normalize: `notes, noteFindings := normalizeCloseoutNotes(notes)`. Non-empty `noteFindings` → return `newCloseoutResult(ResultInvalidInput, CloseoutResult{ID: id, Disposition: CloseoutDispBlocked, Findings: noteFindings})` — invalid notes produce no probe and no mutation.
2. The `StatusDone` short circuit becomes notes-aware. Replace its body with:

```go
	case domain.StatusDone:
		if match, err := closeoutNotesMatchTerminal(cc.body, notes); err != nil {
			return closeoutRefusal(ResultInvalidState, CloseoutDispBlocked, ReasonCloseoutNotesFrozen, err.Error(), id)
		} else if !match {
			return closeoutRefusal(ResultInvalidState, CloseoutDispBlocked, ReasonCloseoutNotesFrozen,
				fmt.Sprintf("change %04d is already terminal; a retry carrying different notes is not a replay and cannot rewrite the terminal record", id), id)
		}
		return newCloseoutResult(ResultNoOp, CloseoutResult{
			ID: id, Disposition: CloseoutDispAlready, ArchivePath: cc.change.Path(),
			Message: fmt.Sprintf("change %04d is already done and archived", id),
		})
```

with the helper (writer-is-the-reader: replay is proven by re-splicing the request into the terminal bytes and comparing):

```go
// closeoutNotesMatchTerminal reports whether the terminal record already
// carries exactly the promise this request makes: splicing the request's notes
// into the terminal bytes is a byte-level no-op. Empty notes match any
// terminal record (the pre-notes replay). The comparison uses the same splice
// that writes, so reader and writer can never disagree.
func closeoutNotesMatchTerminal(body []byte, notes CloseoutNotes) (bool, error) {
	if notes.Empty() {
		return true, nil
	}
	respliced, err := spliceCloseoutNotes(body, notes)
	if err != nil {
		return false, err
	}
	return string(respliced) == string(body), nil
}
```

3. Thread `notes` into `closeoutIntegrationDestination(ctx, deps, cc, ghRepo, canonicalN, facts, notes)` and `closeoutStacked(ctx, deps, cc, parentBranch, facts, notes)` (add the parameter to both).
4. `closeoutStacked`: the already-stacked-merged no-op gains the same guard — `closeoutNotesMatchTerminal(cc.body, notes)` before returning `CloseoutDispAlready`; mismatch → the same `ReasonCloseoutNotesFrozen` refusal. `closeoutStackedOp` gains a `notes CloseoutNotes` field; in its `Plan`, after `clearFinalizeBlockedSection` and BEFORE `document.Parse(cleared)`, add:

```go
		cleared, err = spliceCloseoutNotes(cleared, o.notes)
		if err != nil {
			return refuseCloseout("notes-splice-failed", err.Error())
		}
```

and its receipt becomes `closeoutReceipt{IDs: []int{o.id}, Notes: closeoutNotesDigest(o.notes), Op: OperationFinalizeCloseout, Root: o.id}`.
5. `closeoutArchiveOp` gains a `notes CloseoutNotes` field (set from the threaded parameter in `closeoutIntegrationDestination` where the op literal is built). In its `Plan` Phase A loop, splice ONLY into the explicit change — the root target — after `clearFinalizeBlockedSection`:

```go
			if tg.id == o.rootID {
				cleared, err = spliceCloseoutNotes(cleared, o.notes)
				if err != nil {
					return refuseCloseout("notes-splice-failed", err.Error())
				}
			}
```

Descendants are untouched, which both (a) never propagates root notes and (b) preserves a stacked child's own already-authored `## Closeout notes` section through root archival — its bytes simply ride through.
6. `closeoutReceipt` gains the digest field, placed to keep `json.Marshal`'s output alphabetically keyed (the engine's canonical-receipt requirement — field order in the struct IS the key order):

```go
type closeoutReceipt struct {
	ArchiveDate string `json:"archive_date"`
	IDs         []int  `json:"ids"`
	Notes       string `json:"notes,omitempty"`
	Op          string `json:"op"`
	Root        int    `json:"root"`
}
```

`omitempty` keeps a no-notes receipt byte-identical to today's, so pre-change response-lost replays still adopt cleanly. The archive receipt marshal becomes `closeoutReceipt{ArchiveDate: o.archiveDate, IDs: ids, Notes: closeoutNotesDigest(o.notes), Op: OperationFinalizeCloseout, Root: o.rootID}`. `closeoutBacklinkReceipt` is unchanged — the backlink leg carries no notes.
7. Add the reason constant beside the others:

```go
	// ReasonCloseoutNotesFrozen: the change is already terminal and the request
	// carries notes that differ from the terminal record; refused — a terminal
	// record is never rewritten.
	ReasonCloseoutNotesFrozen = "terminal-notes-frozen"
```

8. Update every existing `FinalizeCloseout(...)` call site found by the grep to pass `CloseoutNotes{}` — behavior byte-identical by construction (`spliceCloseoutNotes` returns src unchanged, digest is "" and omitted).

- [ ] **Step 1: Update the signature, thread the parameter, and fix call sites so the package compiles (no behavior change yet beyond compilation)**

Run: `go build ./... && go test ./internal/app/ -run TestCloseoutOrdinary -count=1`
Expected: PASS — empty notes must leave every existing closeout test green untouched.

- [ ] **Step 2: Write the failing tests (extend `internal/app/finalize_closeout_test.go`)**

Reuse the existing fixture idiom (`setupCloseoutFixture`, `mergeIntoBase`, `baselineMergedFake`, `originFile`, `originTip`, `f.closeoutDeps`, `f.repo.invocation`); model the root-carry test on `TestCloseoutRootCarry` and the stacked test on `TestCloseoutStackedMerged` in the same file.

```go
// closeoutTestNotes is the both-category request the spec's rendering example uses.
func closeoutTestNotes() CloseoutNotes {
	return CloseoutNotes{
		VerificationOutcomes: []string{"Production health check passed after deployment"},
		LateFindings:         []string{"The upgrade guide should mention the legacy config cleanup"},
	}
}

const closeoutWantNotesSection = "## Closeout notes\n\n" +
	"### Verification\n\n" +
	"- Production health check passed after deployment\n\n" +
	"### Late findings\n\n" +
	"- The upgrade guide should mention the legacy config cleanup\n"

// TestCloseoutNotesLandWithArchive proves notes land in the SAME transaction as
// the ordinary archive, as the final section, in both repository modes — and
// that a no-notes closeout emits no section at all.
func TestCloseoutNotesLandWithArchive(t *testing.T) {
	requireRealGit(t)
	for _, m := range planRepoModes() {
		m := m
		t.Run(m.name, func(t *testing.T) {
			f := setupCloseoutFixture(t, m)
			mergeCommit := f.mergeIntoBase(t)
			gh := f.baselineMergedFake(f.head, mergeCommit)

			res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, closeoutTestNotes())
			if res.Result != ResultApplied || res.Disposition != CloseoutDispDoneArchived {
				t.Fatalf("closeout = %q disp %q (reason %q)", res.Result, res.Disposition, res.Reason)
			}
			archived, ok := originFile(t, f.repo.origin, f.branch, res.ArchivePath)
			if !ok {
				t.Fatalf("archived record absent at %q", res.ArchivePath)
			}
			if !strings.HasSuffix(archived, closeoutWantNotesSection) {
				t.Errorf("archived record does not END with the notes section:\n%s", archived)
			}
			if !strings.Contains(archived, "status: 'done'") {
				t.Errorf("notes landed without the lifecycle transition:\n%s", archived)
			}
		})
	}
}

// TestCloseoutNoNotesEmitsNoSection pins the byte-for-byte-today promise.
func TestCloseoutNoNotesEmitsNoSection(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0]
	f := setupCloseoutFixture(t, m)
	mergeCommit := f.mergeIntoBase(t)
	gh := f.baselineMergedFake(f.head, mergeCommit)
	res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, CloseoutNotes{})
	if res.Result != ResultApplied {
		t.Fatalf("closeout = %q", res.Result)
	}
	archived, _ := originFile(t, f.repo.origin, f.branch, res.ArchivePath)
	if strings.Contains(archived, "## Closeout notes") {
		t.Errorf("no-notes closeout conjured a notes section:\n%s", archived)
	}
}

// TestCloseoutNotesInvalidInputMutatesNothing: empty-after-trim, control
// characters, marker text, and an oversized entry each refuse before any
// probe or transaction — the change stays implemented and the tip unmoved.
func TestCloseoutNotesInvalidInputMutatesNothing(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0]
	f := setupCloseoutFixture(t, m)
	mergeCommit := f.mergeIntoBase(t)
	gh := f.baselineMergedFake(f.head, mergeCommit)
	tipBefore := originTip(t, f.repo.origin, f.branch)

	bad := []CloseoutNotes{
		{VerificationOutcomes: []string{"   "}},
		{LateFindings: []string{"bell\x07"}},
		{LateFindings: []string{"crlf\r\n"}},
		{VerificationOutcomes: []string{"<!-- docket:backlink:start -->"}},
		{VerificationOutcomes: []string{strings.Repeat("a", maxAuthoredMarkdownBytes+1)}},
	}
	for i, n := range bad {
		res := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, n)
		if res.Result != ResultInvalidInput {
			t.Fatalf("bad[%d] result = %q, want invalid input", i, res.Result)
		}
	}
	if tip := originTip(t, f.repo.origin, f.branch); tip != tipBefore {
		t.Errorf("an invalid-notes request moved the metadata tip: %q -> %q", tipBefore, tip)
	}
	if gh.probes != 0 { // add an int probe counter to fakeCloseoutGitHub if absent
		t.Errorf("an invalid-notes request reached the PR probe")
	}
}

// TestCloseoutNotesReplayAndFrozen: an identical-notes retry replays as a
// no-op with no second commit; a different-notes retry is refused with
// terminal-notes-frozen and moves nothing.
func TestCloseoutNotesReplayAndFrozen(t *testing.T) {
	requireRealGit(t)
	m := planRepoModes()[0]
	f := setupCloseoutFixture(t, m)
	mergeCommit := f.mergeIntoBase(t)
	gh := f.baselineMergedFake(f.head, mergeCommit)

	first := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, closeoutTestNotes())
	if first.Result != ResultApplied {
		t.Fatalf("first closeout = %q", first.Result)
	}
	tipAfterFirst := originTip(t, f.repo.origin, f.branch)

	replay := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, closeoutTestNotes())
	if replay.Disposition != CloseoutDispAlready || replay.Result == ResultApplied {
		t.Fatalf("identical-notes replay = %q disp %q, want no-op already", replay.Result, replay.Disposition)
	}

	different := closeoutTestNotes()
	different.LateFindings = []string{"a different late finding"}
	refused := FinalizeCloseout(context.Background(), f.closeoutDeps(gh), f.repo.invocation, f.id, different)
	if refused.Reason != ReasonCloseoutNotesFrozen {
		t.Fatalf("different-notes retry reason = %q, want %q (result %q disp %q)",
			refused.Reason, ReasonCloseoutNotesFrozen, refused.Result, refused.Disposition)
	}
	if tip := originTip(t, f.repo.origin, f.branch); tip != tipAfterFirst {
		t.Errorf("a refused retry produced a commit")
	}
	archived, _ := originFile(t, f.repo.origin, f.branch, first.ArchivePath)
	if !strings.HasSuffix(archived, closeoutWantNotesSection) {
		t.Errorf("terminal record's notes changed after the refused retry:\n%s", archived)
	}
}
```

Two further tests follow the SAME dispatch/assert pattern on the sibling fixtures in this file — write them out fully, modeled on the named tests:

- `TestCloseoutNotesStackedInPlace` (model: `TestCloseoutStackedMerged`): explicit closeout of the stacked child with `closeoutTestNotes()` → `CloseoutDispStackedMerged`; the IN-PLACE record at `groomPath(...)` now ends with `closeoutWantNotesSection` and carries `status: 'stacked-merged'`; an identical-notes re-run is `CloseoutDispAlready`; a different-notes re-run is `ReasonCloseoutNotesFrozen`.
- `TestCloseoutNotesRootCarryNoPropagation` (model: `TestCloseoutRootCarry`): first give the CHILD its own notes via its stacked closeout (`CloseoutNotes{LateFindings: []string{"child-owned note"}}`), then close out the ROOT with `closeoutTestNotes()` → `CloseoutDispRootArchived`; assert (a) the archived ROOT record ends with `closeoutWantNotesSection`, (b) the archived CHILD record does NOT contain "Production health check" (no propagation), and (c) the archived CHILD record still contains "child-owned note" (preservation through root archival).

- [ ] **Step 3: Run the new tests to verify they fail**

Run: `go test ./internal/app/ -run 'TestCloseoutNotes' -count=1`
Expected: FAIL (sections absent / reason not returned) — before Step 4's wiring, notes are accepted but dropped, so the section asserts redden. If any assert is green before the wiring exists, stop and find out why (learnings: `assert-detects-removal-not-replacement` — an assert that never saw red proves nothing).

- [ ] **Step 4: Implement the wiring exactly as specified in the task preamble (items 1–8)**

- [ ] **Step 5: Run the closeout suite**

Run: `go test ./internal/app/ -run 'TestCloseout' -count=1`
Expected: PASS — new tests and every pre-existing closeout test (`TestCloseoutOrdinary`, `TestCloseoutIdempotent`, `TestCloseoutRefusals`, `TestCloseoutStackedMerged`, `TestCloseoutRootCarry`, `TestCloseoutBacklinkLegDocketMode`, `TestCloseoutNeverEditsAuthoredBytes`).

- [ ] **Step 6: Mutation-prove the two load-bearing guards** (temp-copy discipline; `-count=1` on every reading)

1. In a scratch edit, remove the `tg.id == o.rootID` condition so notes splice into every target → `TestCloseoutNotesRootCarryNoPropagation` must redden. Restore from your backup copy (`cp file file.bak` first — never `git checkout --`).
2. Remove the `Notes:` field assignment from the archive receipt AND the `closeoutNotesMatchTerminal` mismatch branch (return the no-op unconditionally) → `TestCloseoutNotesReplayAndFrozen` must redden.
Confirm each mutation actually landed (grep the mutated copy) before trusting its reading.

- [ ] **Step 7: Commit**

```bash
git add internal/app/finalize_closeout.go internal/app/finalize_closeout_test.go
git commit -m "feat(0330): closeout carries optional notes through root and stacked paths, receipt-keyed"
```

(Include any other call-site file the Step-1 grep surfaced.)

---

### Task 4: CLI — optional `--input` on `finalize closeout`

**Files:**
- Modify: `internal/cli/finalize.go` (`newFinalizeCloseoutSubcommand`)
- Test: `internal/cli/finalize_test.go` (extend, following the file's existing subcommand-wiring test idiom)

**Interfaces:**
- Consumes: `app.FinalizeCloseout(ctx, deps, repoDir, id, notes)` (Task 3), `decodeInputFlag` (existing).
- Produces: `docket finalize closeout --id <id> [--input <request-file>|-]`.

- [ ] **Step 1: Write the failing test**

Extend `internal/cli/finalize_test.go` following its existing pattern for flag-wiring tests (read the file's `closeout` coverage first and place these beside it):

```go
// TestFinalizeCloseoutInputFlag proves --input is OPTIONAL (the no-input form
// is unchanged), decodes exactly the two documented arrays, and rejects
// unknown fields and malformed JSON as argument errors before any operation.
func TestFinalizeCloseoutInputFlag(t *testing.T) {
	cmd := newFinalizeCloseoutSubcommand(func(app.OperationResult) {})
	if cmd.Flags().Lookup("input") == nil {
		t.Fatalf("finalize closeout has no --input flag")
	}
	// --input must NOT be required: the no-input form is the unchanged default.
	if ann := cmd.Flags().Lookup("input").Annotations[cobra.BashCompOneRequiredFlag]; len(ann) != 0 {
		t.Fatalf("--input must be optional, found required annotation %v", ann)
	}
}

func TestFinalizeCloseoutInputDecode(t *testing.T) {
	valid := `{"verification_outcomes":["a"],"late_findings":["b"]}`
	var in closeoutInput
	if err := decodeRequest(strings.NewReader(valid), "-", &in); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
	if len(in.VerificationOutcomes) != 1 || in.VerificationOutcomes[0] != "a" ||
		len(in.LateFindings) != 1 || in.LateFindings[0] != "b" {
		t.Fatalf("decoded = %+v", in)
	}
	for name, bad := range map[string]string{
		"unknown field":       `{"verification_outcomes":[],"extra":true}`,
		"wrong element type":  `{"late_findings":[42]}`,
		"malformed":           `{"late_findings":`,
		"two documents":       `{}{}`,
	} {
		var dst closeoutInput
		if err := decodeRequest(strings.NewReader(bad), "-", &dst); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run 'TestFinalizeCloseoutInput' -count=1`
Expected: FAIL — no `--input` flag, `undefined: closeoutInput`.

- [ ] **Step 3: Implement**

In `internal/cli/finalize.go`, add beside `finalizeBlockInput`:

```go
// closeoutInput is the bounded request-file payload for `finalize closeout`:
// the two optional authored note lists, and nothing else. Omitted arrays and
// empty arrays are the same no-notes request. DisallowUnknownFields (via
// decodeInputFlag) rejects any other key; the app layer owns entry validation,
// bounds, and rendering.
type closeoutInput struct {
	VerificationOutcomes []string `json:"verification_outcomes"`
	LateFindings         []string `json:"late_findings"`
}
```

In `newFinalizeCloseoutSubcommand`'s `RunE`, between reading `id` and building deps:

```go
			var in closeoutInput
			if src, _ := c.Flags().GetString("input"); src != "" {
				if err := decodeInputFlag(c, &in); err != nil {
					return err
				}
			}
```

then pass it through:

```go
			setResult(app.FinalizeCloseout(c.Context(), deps, repoDir, id, app.CloseoutNotes{
				VerificationOutcomes: in.VerificationOutcomes,
				LateFindings:         in.LateFindings,
			}))
```

and register the flag (NOT marked required — the no-input form is the default):

```go
	cmd.Flags().String("input", "", "optional JSON request file with closeout notes (verification_outcomes, late_findings), or - for stdin")
```

Update the subcommand's doc comment: the id and target directory ride on flags; the optional authored notes ride in `--input`, never argv.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/cli/ -count=1`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cli/finalize.go internal/cli/finalize_test.go
git commit -m "feat(0330): finalize closeout gains optional --input notes request"
```

---

### Task 5: Skill and convention prose, embedded assets regenerated

**Files:**
- Modify: `skills/docket-finalize-change/SKILL.md`
- Modify: `skills/docket-convention/SKILL.md`
- Regenerate: `internal/assets/` embedded bundle (mechanical — `go generate ./internal/assets`; never hand-edit the generated copies)

**Interfaces:**
- Consumes: the CLI surface from Task 4 (`--input`, field names `verification_outcomes` / `late_findings`).
- Produces: the exact prose Task 6's contract test anchors on — keep the phrases below verbatim.

- [ ] **Step 1: Teach the finalize skill the invocation contract (before the mechanical flow)**

In `skills/docket-finalize-change/SKILL.md`, append a new paragraph to the end of the `## Overview` section:

```markdown
**Closeout notes ride the invocation, not a pause.** The caller may include already-known
verification outcomes or late findings in the finalize request; step 9 routes them into the
closeout operation's structured request. The skill never pauses after merge and never asks a new
mid-run question — it records context supplied when finalize was invoked, nothing more. Post-merge
observations belong in the terminal change record's `## Closeout notes` section, never appended to
the frozen merged `results:` file.
```

- [ ] **Step 2: Rewrite the closeout invocation in `### 9. Closeout`**

Replace the single line:

```markdown
`docket finalize closeout --id <id>`. No caller-supplied done boolean or archive date: it reloads
```

with:

```markdown
`docket finalize closeout --id <id> [--input <request-file>]`. When the finalize invocation
supplied verification outcomes or late findings, translate that prose into the two structured
lists — `verification_outcomes` and `late_findings`, each an array of strings — write them as a
bounded JSON request file, and pass it via `--input`; closeout renders them under `## Closeout
notes` in the same transaction that archives the record, and an identical-notes retry replays as
`already` while different notes against a terminal record are refused (`terminal-notes-frozen`).
When no notes were supplied, call the unchanged no-input form and archive immediately — there is
no post-merge pause or second user step. No caller-supplied done boolean or archive date: it reloads
```

(keeping the remainder of the paragraph byte-identical).

- [ ] **Step 3: Document the terminal section in the convention**

In `skills/docket-convention/SKILL.md`, in `### Change body sections`, append a new bullet after the `## Reconcile log` entry:

```markdown
- `## Closeout notes` — terminal-only, **optional**, and the **final authored body section** of a
  terminal record. Written solely by `docket finalize closeout` from its structured request
  (`verification_outcomes` / `late_findings`, rendered as `### Verification` / `### Late findings`
  bullet lists); never hand-edited, never copied to stacked descendants, and never a link-bearing
  artifact (no frontmatter field, no artifact-table row). It records what closeout learned; the
  merged `results:` file stays a frozen build record — the freeze rule above is unchanged.
```

Do not touch the *Merged plans and results are frozen build records* paragraph — `tests/test_results_artifact.sh` pins its exact wording.

- [ ] **Step 4: Regenerate the embedded assets mechanically**

Run: `go generate ./internal/assets && go build ./...`
Then: `go test ./internal/assets/ -count=1`
Expected: PASS — the bundle/manifest drift guards accept the regenerated copies. Stage whatever the generator changed (`git status --short internal/assets` tells you; never hand-edit those files).

- [ ] **Step 5: Run the skill-facing Bash suites touched by prose edits**

Run: `bash tests/test_configured_bash_finalize.sh && bash tests/test_skill_size_budgets.sh && bash tests/test_results_artifact.sh`
Expected: all exit 0 (the results-artifact skip line still prints until Task 6 replaces it). If the size-budget suite reddens on the finalize skill, trim the new prose (it is two short paragraphs; tightening wording is fine, but keep the Task 6 anchor phrases: `verification_outcomes`, `late_findings`, `--input`, `## Closeout notes`, "never pauses after merge").

- [ ] **Step 6: Commit**

```bash
git add skills/docket-finalize-change/SKILL.md skills/docket-convention/SKILL.md internal/assets
git commit -m "docs(0330): finalize skill routes invocation notes to closeout --input; convention documents the terminal section"
```

---

### Task 6: Retire the skipped Bash-era assertion; mutation-proven skill handoff guard

**Files:**
- Modify: `tests/test_results_artifact.sh` (remove the skip block)
- Create: `tests/test_finalize_closeout_notes.sh`

**Interfaces:**
- Consumes: the Task 5 prose anchors in `skills/docket-finalize-change/SKILL.md` and `skills/docket-convention/SKILL.md`.

- [ ] **Step 1: Remove the retired skip block**

In `tests/test_results_artifact.sh`, delete the entire block from the comment line starting `# HARD STOP — DO NOT RETIRE (0316 plan Task 20).` through the `printf 'skip - finalize post-merge results appending …'` line inclusive. The premise is deliberately retired, not re-gated (learnings: `test-premise-deleted-not-regated` — the block guarded a behavior that is now, by design, forbidden): merged results stay frozen, and replacement coverage lives in Task 3's Go tests plus this task's contract guard.

Run: `bash tests/test_results_artifact.sh`
Expected: exit 0, and the output no longer contains a `skip -` line.

- [ ] **Step 2: Write the contract guard**

Create `tests/test_finalize_closeout_notes.sh`:

```bash
#!/usr/bin/env bash
# tests/test_finalize_closeout_notes.sh — the closeout-notes handoff contract.
#
# Guards the producer-to-consumer shape change 0330 added: the finalize skill's
# closeout step transforms invocation-supplied verification/findings into the
# two named request fields and passes the request to `docket finalize closeout
# --input`, with no post-merge pause; and the convention documents the terminal
# `## Closeout notes` section. The handoff checker is exercised by a mutation
# probe on a temp COPY of the skill (never the working file).
set -uo pipefail
REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO"
fail=0
assert(){ if eval "$2"; then printf 'ok - %s\n' "$1"; else printf 'NOT OK - %s\n' "$1"; fail=1; fi; }

SKILL="skills/docket-finalize-change/SKILL.md"
CONV="skills/docket-convention/SKILL.md"

# --- section slice: named terminator, existence-asserted (never a bare /^### /) ---
closeout_section(){ # closeout_section FILE -> the "### 9. Closeout" section text
  awk '/^### 9\. Closeout/{f=1} /^### 10\. Cleanup/{f=0} f' "$1"
}
assert "closeout heading present in the skill" 'grep -q "^### 9\. Closeout" "$SKILL"'
assert "named terminator (### 10. Cleanup) present in the skill" 'grep -q "^### 10\. Cleanup" "$SKILL"'

# --- the handoff checker: run against the real skill AND the mutated copy ---
# The prose is hard-wrapped, so the checker reads a whitespace-collapsed slice.
check_notes_handoff(){ # check_notes_handoff FILE -> 0 when the handoff contract holds
  local sec flat
  sec="$(closeout_section "$1")"
  [ -n "$sec" ] || return 1                       # extractor non-vacuity (see below too)
  flat="$(tr '[:space:]' ' ' <<<"$sec" | tr -s ' ')"
  grep -qF -- "verification_outcomes" <<<"$flat" || return 1
  grep -qF -- "late_findings" <<<"$flat" || return 1
  # Producer->consumer: the request file is PASSED to the closeout invocation.
  grep -qE 'docket finalize closeout --id <id> \[--input <request-file>\]' <<<"$flat" || return 1
  grep -qF -- "pass it via \`--input\`" <<<"$flat" || return 1
  # No new pause: the unchanged no-input default is stated in the same step.
  grep -qF -- "no post-merge pause" <<<"$flat" || return 1
  return 0
}

# Extractor non-vacuity: the slice is non-empty and carries the invocation line.
SEC="$(closeout_section "$SKILL")"
assert "closeout section extractor returns a non-empty slice" '[ -n "$SEC" ]'
assert "closeout section carries the closeout invocation" \
  'grep -qF -- "docket finalize closeout" <<<"$SEC"'

assert "finalize skill routes invocation notes into the closeout --input handoff" \
  'check_notes_handoff "$SKILL"'

# The invocation contract is stated up front, before the mechanical flow.
OVERVIEW_FLAT="$(awk '/^## Overview/{f=1} /^## When to use/{f=0} f' "$SKILL" | tr '[:space:]' ' ' | tr -s ' ')"
assert "overview names the no-pause invocation contract" \
  'grep -qF -- "never pauses after merge" <<<"$OVERVIEW_FLAT"'

# --- convention: the terminal section is documented, bound to its claims ---
CONV_FLAT="$(tr '[:space:]' ' ' < "$CONV" | tr -s ' ')"
assert "convention documents ## Closeout notes as terminal-only and closeout-written" \
  'grep -qE -- "\`## Closeout notes\`[^|]{0,220}Written solely by \`docket finalize closeout\`" <<<"$CONV_FLAT"'
assert "convention keeps the results freeze rule beside the new section" \
  'grep -qF -- "the freeze rule above is unchanged" <<<"$CONV_FLAT"'

# --- mutation probe: prove the checker detects the handoff's removal ---
# Copy the skill, verify the handoff is present, strip it, verify the mutation
# LANDED, then require the same checker to reject the copy.
TMP="$(mktemp "${TMPDIR:-/tmp}/closeout-notes-skill.XXXXXX")"
trap 'rm -f "$TMP"' EXIT
cp "$SKILL" "$TMP"
assert "mutation baseline: checker passes on the pristine copy" 'check_notes_handoff "$TMP"'
sed -i '' -e 's/--input//g' "$TMP" 2>/dev/null || sed -i -e 's/--input//g' "$TMP"
assert "mutation landed: --input is gone from the copy" '! grep -qF -- "--input" "$TMP"'
assert "checker rejects the skill with the handoff stripped (mutation-proven)" \
  '! check_notes_handoff "$TMP"'

exit $fail
```

House rules honored (verify, do not assume): every leading-`--` pattern is declared via `grep -qF --` / `grep -qE --`; no `producer | grep -q` pipelines (heredoc `<<<` only); wrapped-prose asserts read collapsed haystacks; the section slice uses a NAMED terminator whose existence is separately asserted; the mutation runs on a temp copy.

- [ ] **Step 3: Run it — including the intentional-red rehearsal**

Run: `bash tests/test_finalize_closeout_notes.sh`
Expected: exit 0, every line `ok -`.
Then rehearse a real failure: on a temp copy of the CONVENTION file, delete the `## Closeout notes` bullet and point the two `CONV` asserts at the copy (edit locally, do not commit) — they must go red. Revert your local edit.

- [ ] **Step 4: Register the new test if the runner requires it**

Check how `scripts/run-tests.sh` discovers tests (read `tests/README.md`): if discovery is by glob, nothing to do; if there is a budget/manifest table, add `test_finalize_closeout_notes.sh` per the README's instructions.

- [ ] **Step 5: Commit**

```bash
git add tests/test_results_artifact.sh tests/test_finalize_closeout_notes.sh
git commit -m "test(0330): retire the skipped Bash-era append assertion; mutation-proven closeout-notes handoff guard"
```

---

### Task 7: Whole-suite gate

**Files:** none created; fixes only if the gate reddens.

- [ ] **Step 1: Run the configured suite**

The suite command is what `finalize.test_command` resolves to in `.docket.yml` — currently `scripts/run-tests.sh`; read it there, never from this line, before running.

Run: `scripts/run-tests.sh`
Expected: exit 0.

- [ ] **Step 2: Act on budget findings**

Inspect the tail of the run for `OVER BUDGET:` lines. They do not fail the run and nothing else will surface them: any file this change pushed over budget is a finding to fix (split the test or tighten it) or to record explicitly in the build results with its measured number — never silence.

- [ ] **Step 3: Spot re-verification with the cache defeated**

Run: `go test ./internal/app/ ./internal/cli/ ./internal/render/ -count=1`
Expected: PASS (a cached verdict certifies nothing about this tree).

- [ ] **Step 4: Commit any gate-driven fixes**

```bash
git add -u
git commit -m "fix(0330): suite-gate follow-ups"
```

(Skip the commit if the gate was green with nothing to fix.)

---

## Self-review (performed at plan time)

- **Spec coverage:** request surface + strict decode (Task 4); rendering/ownership incl. escape-proof bullets and marker rejection (Tasks 1–2); same-transaction landing, root/stacked carry, no-propagation, preservation, both repo modes (Task 3); idempotency digest in the receipt, replay vs terminal-notes-frozen refusal, no-input == empty-input (Tasks 2–3); skill behavior with no pause (Task 5); convention section + freeze rule untouched (Task 5); skip retirement + mutation-proven handoff guard with non-vacuous extractor (Task 6); embedded assets regenerated mechanically (Task 5); whole suite + OVER BUDGET + `-count=1` (Task 7).
- **Known judgment calls, argued from the spec:** (1) replay-vs-refusal on an already-terminal record is decided by re-splicing the request into the terminal bytes — the writer is the reader, so the two can never disagree (spec: "the promise being keyed is the exact terminal record"); (2) the receipt digest keys the RENDERED section body, and `omitempty` keeps pre-change no-notes receipts byte-identical; (3) `\n` and `\t` are the only permitted control characters in an entry (`\r` refused) since the renderer emits LF-only continuations.
- **Type consistency:** `CloseoutNotes` / `normalizeCloseoutNotes` / `closeoutNotesDigest` / `spliceCloseoutNotes` / `closeoutNotesMatchTerminal` / `ReasonCloseoutNotesFrozen` / `closeoutInput` / `render.CloseoutNotesHeading` / `render.CloseoutNotesBody` are used with the same spellings across Tasks 1–6.
- **Executor caution:** plan-supplied test code is unverified (learnings: `plan-supplied-test-code-is-unverified`) — prove each new assert CAN pass and saw red before trusting it; fixture helper names (`originFile`, `originTip`, `groomPath`, `fakeCloseoutGitHub`) must be re-checked against `internal/app/finalize_closeout_test.go` as it stands when the task runs, and the probe-counter assert in `TestCloseoutNotesInvalidInputMutatesNothing` requires adding a counter to the fake if it lacks one.
