<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0561 — Export an opened artifact as a clean, docket-free Markdown file](../../changes/archive/2026-10-20-0561-export-an-opened-artifact-as-a-clean-docket-free-markdown-fi.md)**
<!-- docket:backlink:end -->

# Export an Opened Artifact as Clean Markdown Implementation Plan

> **For agentic workers:** This plan is executed by `docket-build` (task-by-task, tier-routed workers under the `docket-build-task` contract, one full-suite gate at the end). Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `docket open <change|spec|plan|results> [<id>] --export` writes a clean, docket-free Markdown copy of the artifact, reports leftovers and docket mentions as warnings, and opens (or with `--print`, prints) that copy.

**Architecture:** A pure `internal/export` package holds the seven-step transform and the export filename rule. A separate pure `internal/exportcheck` package holds the self-check detector. It shares no code or patterns with the transform, and a test enforces that. `internal/document` gains one lenient line classifier that both packages' callers can use. It reuses the frontmatter locator, the fence tracking, and the marker grammar without the all-or-nothing marker validation. `internal/app/open.go` routes `--export` through the existing `localArtifact` path, then transforms, checks, writes atomically, and hands the copy to the existing `finishOpen`.

**Tech Stack:** Go 1.26 stdlib only (`regexp`, `strings`, `os`, `path/filepath`), cobra/pflag for the flags. No new modules.

**Spec:** `docs/superpowers/specs/2026-10-20-export-an-opened-artifact-as-a-clean-docket-free-markdown-fi-design.md` (on the `docket` metadata branch; the metadata checkout is `.docket/` in the primary checkout). Every rule below argues from it; read it before Task 2.

## Global Constraints

- Command surface, verbatim: `docket open <change|spec|plan|results> [<id>] --export [--output-path <path>] [--warnings all|leftovers|mentions] [--print]`.
- Refusals (usage errors, exit 2, nothing written): `--export` with `board` or `pr`; `--output-path` or `--warnings` without `--export`; a `--warnings` value other than `all`, `leftovers`, `mentions`.
- `--export` always reads the **local metadata checkout** file through `localArtifact`, in every repository and regardless of `open.artifacts`. A failed prepare adds today's staleness note and proceeds.
- Default destination: `~/Downloads` on darwin, `os.UserHomeDir()` on linux. A missing `~/Downloads` on darwin is refused with one line naming `--output-path`. An existing directory as `--output-path` means "this directory, normal name". Anything else is the exact file path. A relative path resolves against the working directory. A missing parent directory is refused (`output-path-invalid`).
- Filename `<date>-<type>-<name>.md`, type ∈ {`issue`,`spec`,`plan`,`results`}. Spec/plan/results: date = basename's leading `YYYY-MM-DD`, name = rest with a trailing `-design` (spec) / `-results` (results) removed. With no leading date: the change's `created:` plus the whole stem. Issue: `created:` + `slug`. An existing destination file is replaced, and the output says so.
- Nothing inside a fenced code block or an inline code span is ever changed.
- Bare `#N`, `PR #N`, ADR references (`ADR-0147`, `ADR-0065's`), unknown ids, and bare typed URLs stay exactly as written. None of them is ever warned about.
- Wherever the export itself writes the word for a change, it writes "issue" (`another issue`, `other issues`).
- Warnings never block: the file is written, the result is `applied`, exit 0.
- Human stdout is exactly the exported path. Notes (`note: …`) and warnings (`warning: …`) go to stderr.
- Every conversion is deterministic Go. No agent, no AI, no new dependency.
- The capability catalog stays ≤ 16,384 bytes (`TestCapabilitiesPayloadWithinByteBudget`). The spec measured about 16,264 after this change. Going over is a stop-and-report, never a ceiling raise in this change.
- Docs describe current behavior only, with no change or PR citations (`docs/reference/cli.md`, `--help`).
- Mutation probes run with `go test -count=1`, and restore via a `cp` backup, never `git checkout --`.

## Review Focus

1. **CRLF source.** A record saved with `\r\n` must come out with LF only. No stray `\r` may be left in headings, and the issue allowlist must still match `## Why\r`. Test: `TestTransformNormalizesCRLF` (Task 4).
2. **Damaged markers and frontmatter must never eat content.** A start marker with no end, an end with no start, and a `---` first line that never closes must each leave every other line in place. Never remove to EOF. Test: `TestTransformDamagedSourceKeepsContent` (Task 4).
3. **A link target with balanced parentheses** (`[Foo](https://en.wikipedia.org/wiki/Foo_(bar))`) or a title (`[a](u "t")`) must reduce to `Foo` / `a` with no `)` or quote residue. Test: cases in `TestStripLinks` (Task 2).
4. **An id two records claim (ambiguous in the snapshot)** must be treated as unknown and left as written, never given an arbitrary title. Test: `TestOpenExportAmbiguousIDLeftAsWritten` (Task 7).
5. **Heading-shaped lines inside a fence** (`## Reconcile log` in a code example) must not end an issue section or count as an empty-section heading, and the fenced bytes must come out identical. Test: `TestTransformFenceHeadingsAreContent` (Task 4).

Known limitation (record it in the results file, do not build for it): an inline code span that crosses a line break is treated as plain text. The sources contain none. Task 6 inventories and confirms this.

---

## File structure

| File | Responsibility |
|---|---|
| `internal/document/classify.go` (new) | `ClassifyLines`: lenient per-line classes (frontmatter / fence / marker start / end / malformed / text) |
| `internal/export/inline.go` (new) | inline code-span masking and step 5 (links) |
| `internal/export/refs.go` (new) | step 6 (change references) and title cleaning |
| `internal/export/export.go` (new) | `Kind`, `Titles`, `Input`, `Output`, `Transform`, steps 1–4 and 7, the step switch |
| `internal/export/filename.go` (new) | `FileName` |
| `internal/export/testdata/corpus/` (new) | frozen real artifacts + constructed code fixture + `titles.tsv` + `PROVENANCE.md` |
| `internal/exportcheck/check.go` (new) | the independent detector: leftovers, heading rules, mentions |
| `internal/app/open_export.go` (new) | the export branch of `Open`: validation, destination, write, warnings filter |
| `internal/app/open.go` (modify) | options/deps/result fields, `resolveOpenChange` also returns the snapshot, the export branch |
| `internal/cli/presenter.go` (modify) | `HumanWarnings()` → `warning: …` lines on stderr |
| `internal/cli/open.go` (modify) | flags, usage validation, help text, production `HomeDir`/`GOOS`/`Getwd` wiring |
| `internal/cli/capability_production_test.go` (modify) | the pinned `open` signature |
| `docs/reference/cli.md` (modify) | the `docket open` entry |

---

### Task 1: Lenient line classifier in `internal/document`

**Files:**
- Create: `internal/document/classify.go`
- Test: `internal/document/classify_test.go`

**Interfaces:**
- Produces:
  ```go
  type LineKind int
  const (
      LineText LineKind = iota
      LineFrontmatter     // a line of the leading ---…--- block, both fences included
      LineFence           // a code-fence delimiter line, or a line inside a fenced block
      LineMarkerStart
      LineMarkerEnd
      LineMarkerMalformed // opens like a marker (markerPrefixRE) but fails markerRE
  )
  type LineInfo struct {
      Kind LineKind
      Name string // qualified block name (dckt: prefix kept) for start/end markers; "" otherwise
  }
  // ClassifyLines returns one LineInfo per physical line, exactly as scanLines tiles src.
  // It never errors: an unclosed frontmatter opener is LineText, an unclosed fence runs to EOF,
  // and marker balance is not checked.
  func ClassifyLines(src []byte) []LineInfo
  ```

- [ ] **Step 1: Write the failing test** `TestClassifyLines`: a table over sources and the expected kinds per line. The table includes:
  - frontmatter + body + a `docket:artifacts` start/end pair → `[Frontmatter×N, Text, MarkerStart{artifacts}, Text, MarkerEnd{artifacts}]`;
  - a `<!-- dckt:backlink:start -->` line → `MarkerStart{Name:"dckt:backlink"}`;
  - a marker inside a fenced block (```` ``` ```` and `~~~`, including a longer closing run) → `LineFence`;
  - an unclosed `---` first line → every line `LineText`;
  - an end marker with no start, and a start with no end → still classified (no error);
  - `<!-- docket:Bad -->` → `LineMarkerMalformed`;
  - CRLF input → same kinds as LF.

  Also assert `len(ClassifyLines(src)) == len(scanLines(src))` for every case.
- [ ] **Step 2: Run** `go test -count=1 ./internal/document/ -run TestClassifyLines`. Expected: FAIL (undefined: ClassifyLines).
- [ ] **Step 3: Implement** `ClassifyLines`, reusing `scanLines`, `locateFrontmatter` (on its error, treat as no frontmatter), `fenceRun`/`isBareFence`, `markerPrefixRE`, and `markerRE`. Do not change `scanMarkers` or `Parse`.
- [ ] **Step 4: Run** the same command. Expected: PASS. Then run `go test -count=1 ./internal/document/`. Expected: PASS.
- [ ] **Step 5: Commit** `git add internal/document/classify.go internal/document/classify_test.go && git commit -m "feat(document): classify lines leniently for readers that must not trust marker balance"`

---

### Task 2: Code-span masking and link removal (step 5)

**Files:**
- Create: `internal/export/inline.go`, `internal/export/doc.go` (package comment only)
- Test: `internal/export/inline_test.go`

**Interfaces:**
- Produces (package `export`, unexported, used by Tasks 3–4):
  ```go
  // maskCode replaces every inline code span in line with a placeholder made of
  // private-use runes only (no digits, no ASCII), so later rules cannot match inside code.
  // restore puts the original spans back byte-for-byte.
  func maskCode(line string) (masked string, restore func(string) string)
  // linkLabels collects the normalized (lower-cased, whitespace-collapsed) labels of every
  // link-reference definition line outside fences.
  func linkLabels(lines []string, info []document.LineInfo) map[string]bool
  // isLinkDefinition reports a CommonMark link-reference definition line ("[ref]: url ...").
  func isLinkDefinition(line string) bool
  // stripLinks removes every link form from a masked line and keeps its visible text.
  func stripLinks(masked string, labels map[string]bool) string
  ```

- [ ] **Step 1: Write the failing tests.**
  - `TestMaskCode`: `` "a `x [y](z)` b" `` masks to a string with no `[`, `(`, or digits from the span, and restore of the unchanged masked string returns the input byte-identical. Cover double-backtick spans (``` `` a ` b `` ```), an escaped backtick (`` \` `` is not a span), and an unmatched backtick run (stays text).
  - `TestStripLinks`: table, input → output:
    - `[Run Modes](https://cursor.com/docs/x)` → `Run Modes`
    - `[PR 265](https://github.com/o/r/pull/265)` → `PR 265`
    - `[#559](../changes/archive/x.md)` → `#559`
    - `[text](#anchor)` → `text`; `[m](mailto:a@b.c)` → `m`
    - `[Foo](https://en.wikipedia.org/wiki/Foo_(bar)) end` → `Foo end`
    - `[a](u "t")` → `a`
    - `<https://go.dev/x>` → `https://go.dev/x`
    - `![alt](p.png)` → `alt`; `![](p.png) x` → ` x` (the image is removed)
    - `[text][ref]`, `[text][]`, `[ref]` (label present) → `text`, `text`, `ref`; `[note]` with no definition → unchanged
    - `<a href="https://x">text</a>` → `text`
    - `see https://go.dev/doc/toolchain` → unchanged (bare URL)
    - `` [`internal/app/open.go`](../x) `` handled through `maskCode` → `` `internal/app/open.go` ``
    - `<!-- docket:x:start -->` → unchanged (not an autolink)
  - `TestIsLinkDefinition`: `[ref]: https://x`, `  [a b]: <u> "t"` → true; `[ref] text`, `    [ref]: u` (4-space indent, code) → false.
- [ ] **Step 2: Run** `go test -count=1 ./internal/export/ -run 'TestMaskCode|TestStripLinks|TestIsLinkDefinition'`. Expected: FAIL (package or symbols missing).
- [ ] **Step 3: Implement** `inline.go`. Use a hand-written scanner for inline links/images: match brackets, then a balanced-paren destination with an optional quoted title. Reference-style and shortcut links resolve against `labels`. A regexp is fine for the autolink (`<scheme:…>` with a 2–32 char scheme, no spaces or `<>`) and for HTML `<a …>…</a>`.
- [ ] **Step 4: Run** the step 2 command. Expected: PASS.
- [ ] **Step 5: Commit** `git add internal/export && git commit -m "feat(export): mask inline code and remove every link form, keeping its text"`

---

### Task 3: Change references and title cleaning (step 6)

**Files:**
- Create: `internal/export/refs.go`
- Test: `internal/export/refs_test.go`

**Interfaces:**
- Consumes: nothing from Task 2 (it operates on already-masked text).
- Produces:
  ```go
  // Titles maps a known change id to its raw title. An id the snapshot cannot resolve
  // to exactly one change is absent, so it reads as unknown.
  type Titles map[int]string
  func convertRefs(masked string, titles Titles) string // step 6 on one masked line
  func cleanTitle(title string, titles Titles) string   // the one-pass title cleaning
  ```

The recognized forms are exactly these. Recognizing a form decides nothing on its own: a form converts only when its number is in `titles`.
- **Word form.** The word `change` (case-insensitive, whole word), then whitespace, then an optional `#`, then digits. The plural `changes` takes a list of such numbers separated by `,`, `and`, or `, and`. List items convert item by item. The word `changes` is dropped only when every item converts.
- **Padded form.** `#0ddd`, `(0ddd)`, or a bare `0ddd` that stands as its own word: exactly four digits with a leading `0`. It does not count when preceded by `[A-Za-z0-9_/.#-]`, unless that `#` is its own `#0ddd` form. It does not count when followed by `[A-Za-z0-9_/-]`, or by `.` + alphanumeric. So `0561-export…md`, `docs/0561`, and `0559.md` never match. `0282's` and a sentence-final `0559.` do match. A padded number immediately after `ADR` + a space or hyphen (case-insensitive) is an ADR reference and never matches.
- Bare unpadded `#N` and `PR #N` are never forms.

Readings, in priority order:
1. **References-only parenthesis.** A `( … )` whose whole content is items separated by `,` / `and` / whitespace, where each item is a form or another reference token (`#N`, `PR #N`, `ADR-N`, an unknown form). Converted items are removed with their separators. The rest re-join with `, `, and the last joiner is ` and ` when the original's last separator held `and`. When nothing is left, the parenthesis and the one space before it are removed.
2. **Own-title dash.** A converted reference followed by ` — `, ` – `, or ` - ` and then text that begins with that change's raw title: the reference and the dash are removed.
3. **Elsewhere.** The reference, `change` word included, becomes `"` + `cleanTitle(title)` + `"`. A list becomes `"A", "B" and "C"`.

`cleanTitle` makes one pass. A references-only parenthesis of convertible forms is removed. Any other convertible form becomes `another issue` (`other issues` for a plural list). `#N` and ADR stay. `"` becomes `'`. It never looks a title up from inside a title.

- [ ] **Step 1: Write the failing tests.** `TestConvertRefs` is a table with `titles = {547: "Measure whether the 1h cache TTL cut implement-next's cost", 363: "Remove main-mode compatibility from Go v1", 461: "A", 550: "B", 113: "C", 114: "D", 120: "E", 466: "Bring test_go_race back under its 60s row", 111: "Guard", 559: "Close a change to done without a PR", 282: "X", 88: "Y"}`:
  - `Change #547 holds the full measurement` → `"Measure whether the 1h cache TTL cut implement-next's cost" holds the full measurement`
  - `from change 0363 forbids it` → `from "Remove main-mode compatibility from Go v1" forbids it`
  - `changes #461 and #550` → `"A" and "B"`; `changes 0113, 0114 and 0120` → `"C", "D" and "E"`; `changes 0113, 9999 and 0120` → `changes "C", 9999 and "E"`
  - `fields (0461, 0550)` → `fields`; `(0461, ADR-0012)` → `(ADR-0012)`; `(#461, ADR-0012)` unchanged
  - `# Change 0466 — Bring test_go_race back under its 60s row budget` → `# Bring test_go_race back under its 60s row budget`
  - `the 0111 guard` → `the "Guard" guard`; `0282's` → `"X"'s`; `mirroring 0088` → `mirroring "Y"`; `#0559` → `"Close a change to done without a PR"`; `(0559)` alone in prose → removed with its leading space
  - unchanged: `#559`, `PR #559`, `ADR-0147`, `ADR 0147`, `ADR-0065's`, `change 9999`, `0561-export.md`, `docs/0561`, `0559.md`, `change 3 files` (3 not in titles)
  - `(see change 0547)` → `(see "Measure …")` (prose parenthesis, reading 3)

  `TestCleanTitle`:
  - `Clear the unfixed review findings from change 0113` → `Clear the unfixed review findings from another issue`
  - `Fix "quoted" bits (0461, 0550)` → `Fix 'quoted' bits`
  - `Split changes 0113 and 0114` → `Split other issues`
  - `Follow #559 and ADR-0012` → unchanged
  - a title that contains its own id → `another issue` (no recursion)
- [ ] **Step 2: Run** `go test -count=1 ./internal/export/ -run 'TestConvertRefs|TestCleanTitle'`. Expected: FAIL.
- [ ] **Step 3: Implement** `refs.go`. Use one finder that returns spans (start, end, ids, known flags, word/plural, inside-parenthesis context), and two renderers (`convertRefs`, `cleanTitle`).
- [ ] **Step 4: Run** the step 2 command. Expected: PASS.
- [ ] **Step 5: Commit** `git add internal/export/refs.go internal/export/refs_test.go && git commit -m "feat(export): convert explicit change references to titles and clean inserted titles"`

---

### Task 4: The transform pipeline and the export filename

**Files:**
- Create: `internal/export/export.go`, `internal/export/filename.go`
- Test: `internal/export/export_test.go`, `internal/export/filename_test.go`

**Interfaces:**
- Consumes: `document.ClassifyLines` (Task 1); `maskCode`, `linkLabels`, `isLinkDefinition`, `stripLinks` (Task 2); `Titles`, `convertRefs`, `cleanTitle` (Task 3).
- Produces:
  ```go
  type Kind string
  const (KindIssue Kind = "issue"; KindSpec Kind = "spec"; KindPlan Kind = "plan"; KindResults Kind = "results")
  type Input struct {
      Kind   Kind
      Source []byte // the artifact bytes from the metadata checkout
      Title  string // the change's raw title (the issue H1)
      Titles Titles
  }
  type Stray struct { SourceLine int; Text string } // an unpaired or malformed marker line that was removed
  type Output struct { Markdown []byte; Stray []Stray }
  func Transform(in Input) Output
  func FileName(kind Kind, sourcePath, created, slug string) string // created is "YYYY-MM-DD" or ""

  // unexported, for the Task 6 mutation test:
  type step int
  const (stepFrontmatter step = iota + 1; stepBlocks; stepIssueSections; stepStamps; stepLinks; stepRefs; stepCleanup)
  var allSteps = []step{stepFrontmatter, stepBlocks, stepIssueSections, stepStamps, stepLinks, stepRefs, stepCleanup}
  func transform(in Input, skip map[step]bool) Output // Transform == transform(in, nil)
  ```

Pipeline. Normalize `\r\n` to `\n` first. Classify the lines with `ClassifyLines`, then:

1. Drop `LineFrontmatter` lines.
2. Pair markers leniently in source order. A start opens a block that its same-name end closes, and both markers and everything between them are dropped. An end with no open start, a start never closed before EOF or before another start, and a `LineMarkerMalformed` line are each dropped **alone** and recorded in `Stray` (1-based source line). Never drop to EOF.
3. Issue only. Keep only the H2 sections `## Why`, `## What changes`, `## Out of scope`, `## Open questions` in source order. An H2 boundary is a non-fence line `## `. Everything else, the pre-H2 preamble included, is dropped. Prepend `# ` + `cleanTitle(in.Title)` and a blank line.
4. Spec only, before the first H2. Remove a stamp sentence `^Change #?N[,.] [Gg]roomed YYYY-MM-DD( \(…\))?\.`, keeping any rest of the line (left-trimmed). Remove a line that is only `Change N.` / `Change #N.`. Remove a line that is only `**Change:** N` / `**Change:** #N`. Remove a `·`-separated run of `**Key:** value` pairs that includes `**Change:**`.
5–6. On every non-fence line: collect `linkLabels` once. Drop `isLinkDefinition` lines. Then, per line: `maskCode` → `stripLinks` → `convertRefs` → restore.
7. Drop a heading (any level, non-fence) whose section holds only blank lines before the next heading of the same or higher level, or before EOF. Repeat until stable. Collapse runs of blank lines to one. No leading blank lines. Exactly one trailing `\n`.

- [ ] **Step 1: Write the failing tests** (`export_test.go`), each asserting exact output bytes:
  - `TestTransformSpec`: a spec with the backlink block, an H1, `Change #561, groomed 2026-10-20.`, a link, and a `change 0559` reference → H1 first, no stamp, no backlink, link text kept, title quoted.
  - `TestTransformSpecStampKeepsRest`: `Change #12, groomed 2026-01-02. Found while building change #13 (PR #14).` → `Found while building "<title 13>" (PR #14).`. `Change 0373. Groomed 2026-09-02 (interactive). Priority high: x` → `Priority high: x`. `**Change:** 0003` alone → removed. `**Change:** 5 · **Type:** feat · **Priority:** high` → removed. The same `Change 12.` line *after* the first H2 → kept and converted by step 6.
  - `TestTransformIssue`: a record with frontmatter, `## Artifacts` block, `## Why`, `## What changes`, `## Reconcile log`, `## Why deferred`, `## Out of scope` → `# <title>` + the three kept sections in source order, and nothing else.
  - `TestTransformNormalizesCRLF` (Review Focus 1): the issue fixture in CRLF → the same bytes as the LF output, and no `\r` anywhere.
  - `TestTransformDamagedSourceKeepsContent` (Review Focus 2): (a) a start with no end, mid-file → only that line is gone, and later prose is present; (b) an end with no start → only that line is gone; (c) unclosed `---` first line → the `---` and every following line kept. For (a)/(b), `Stray` holds the source line numbers.
  - `TestTransformFenceHeadingsAreContent` (Review Focus 5): an issue whose `## Why` holds a fence containing `## Reconcile log` and `` `change 0559` `` → the fence comes out byte-identical inside `## Why`.
  - `TestTransformCleanup`: an `## Artifacts` heading left empty → dropped; three blank lines → one; leading blanks gone; one trailing newline.

  `filename_test.go` `TestFileName` uses the spec's four #559 rows verbatim:
  - `(KindSpec, "docs/superpowers/specs/2026-10-17-close-a-change-to-done-without-a-pr-design.md", "2026-10-17", "…")` → `2026-10-17-spec-close-a-change-to-done-without-a-pr.md`
  - the plan, results, and change rows likewise
  - an undated basename `foo-design.md` with created `2026-01-02` → `2026-01-02-spec-foo.md`
  - empty created and undated → `spec-foo.md`
- [ ] **Step 2: Run** `go test -count=1 ./internal/export/`. Expected: FAIL (undefined Transform/FileName).
- [ ] **Step 3: Implement** `export.go` and `filename.go` as specified above, with `transform` honoring `skip` per step.
- [ ] **Step 4: Run** `go test -count=1 ./internal/export/`. Expected: PASS.
- [ ] **Step 5: Commit** `git add internal/export && git commit -m "feat(export): transform an artifact into clean Markdown and name the exported file"`

---

### Task 5: The independent self-check detector

**Files:**
- Create: `internal/exportcheck/check.go`
- Test: `internal/exportcheck/check_test.go`, `internal/exportcheck/independence_test.go`

**Interfaces:**
- Produces:
  ```go
  package exportcheck
  const (
      RuleMarker = "marker"; RuleFrontmatter = "frontmatter"; RuleBanner = "banner"; RuleLink = "link"
      RuleChangeRef = "change-reference"; RuleStamp = "stamp"; RuleHeading = "heading"
  )
  type Leftover struct { Line int; Text string; Rule string } // 1-based output line
  type Mentions struct { Count int; Lines []int }
  type Report struct { Leftovers []Leftover; Mentions Mentions }
  func Check(md []byte, known func(id int) bool) Report
  ```

The detector is written **from the spec's rules, not from `internal/export`**. It must not import `internal/export`, and `internal/export` must not import it. It has its own fence and code-span awareness, written fresh, and it must never import `internal/document`.

Leftovers, outside code:
- any `<!-- docket:` / `<!-- dckt:` text → marker
- a `---` first line → frontmatter
- a line containing `↩` → banner
- any link syntax (inline link or image `](`, autolink `<scheme:`, reference link or definition, `<a ` tag) → link
- a word or padded change form whose number is `known` (same exclusions as the spec: ADR, paths, bare `#N`, `PR #N`) → change-reference
- a stamp-shaped line before the first H2 → stamp
- heading rules → heading: not exactly one H1, the H1 not the first heading, or a level skipped (Line = the offending heading)

Mentions: every line, code included, that contains the whole word `docket` or `dckt`, case-insensitive.

- [ ] **Step 1: Write the failing tests.**
  - `TestCheckFindsEachLeftoverKind`: one crafted input per rule, asserting the exact `Leftover{Line, Rule}`.
  - `TestCheckKeepsWhatTheExportKeeps`: `#559`, `PR #12`, `ADR-0147`, `change 9999` (unknown), `https://go.dev/x` bare, and every leftover shape inside a fence and inside a code span → zero leftovers.
  - `TestCheckMentions`: `docket` in prose on line 2 and in a fence on line 5, plus `Docketed` (not a whole word) on line 7 → `Mentions{2, [2 5]}`.
  - `TestCheckHeadingRules`: two H1s; an H2 before the H1; H2 → H4 → each reported.
  - `independence_test.go` `TestDetectorSharesNothingWithTransform`: run `go list -deps` (via `os/exec`, `-f '{{.ImportPath}}'`) on `./internal/exportcheck` and `./internal/export`. Assert that neither dep list contains the other package, and that exportcheck's list holds no `github.com/danielhanold/docket/internal/document`. Mutation-check it: add a blank import of `internal/export` in `check.go` (from a `cp` backup) → red; restore from the backup.
- [ ] **Step 2: Run** `go test -count=1 ./internal/exportcheck/`. Expected: FAIL.
- [ ] **Step 3: Implement** `check.go`.
- [ ] **Step 4: Run** `go test -count=1 ./internal/exportcheck/`. Expected: PASS. Then run the import mutation from step 1. Expected: `TestDetectorSharesNothingWithTransform` FAIL. Restore from the backup and re-run: PASS.
- [ ] **Step 5: Commit** `git add internal/exportcheck && git commit -m "feat(exportcheck): independent self-check for export leftovers, heading rules, and mentions"`

---

### Task 6: Frozen real corpus, the corpus guard, and the per-step mutation guard

**Files:**
- Create: `internal/export/testdata/corpus/{changes,specs,plans,results,constructed}/…`, `internal/export/testdata/corpus/titles.tsv`, `internal/export/testdata/corpus/PROVENANCE.md`
- Test: `internal/export/corpus_test.go` (package `export`, internal test, so it reaches `transform`/`allSteps`. It may import `exportcheck`: a test-only import, which the Task 5 guard does not see because it reads non-test deps.)

**Interfaces:**
- Consumes: `Transform`, `transform`, `allSteps`, `Kind`, `Titles` (Task 4); `exportcheck.Check` (Task 5).

- [ ] **Step 1: Inventory before copying** (learning: *frozen-corpus-covers-what-it-contains*). From the metadata checkout (`git -C <primary> worktree list` shows its path, normally `<primary>/.docket`), pick real files and copy them verbatim. Candidates the planner found, confirm each:
  - changes: `docs/changes/archive/2026-06-02-0001-results-artifact.md` (`blob/main` docket links); `docs/changes/archive/2026-09-14-0423-certify-native-multi-agent-v2-orchestration-through-docket-i.md` (metadata-branch absolute link); one record with bookkeeping sections (Reconcile log + Build evidence block + Closeout notes; find with `grep -l`).
  - specs: `2026-09-02-harden-integration-race-test-isolation-under-parallel-load-design.md` (`Change N. Groomed … (interactive). Priority …`); `2026-07-28-board-checks-exit-swallowed-design.md` (`Change N.`); `2026-06-04-migration-tool-pwd-target-design.md` (`**Change:** N`); a spec whose H1 is `# Change N — <its own title>` (verify the remainder equals the record title); `2026-07-10-docket-skill-slimming-design.md` (autolink); a recent `Change #N, groomed …` spec.
  - results: `docs/results/2026-10-17-let-change-groom-clear-stacked-on-results.md` (dense references).
  - plans: one plan with code fences holding reference- and marker-shaped text.
  - `constructed/code.md`: hand-built. It holds reference-, link-, and marker-shaped text in ```` ``` ```` and `~~~` fences and in code spans, plus a bare URL, `#N`, `PR #N`, `ADR-0012`, and an unknown padded id.

  Write `titles.tsv` (`<id>\t<raw title>`, one per line) for every change id any corpus file references, read from each record's frontmatter `title:`. Write `PROVENANCE.md`: the source (metadata branch commit from `git -C <metadata checkout> rev-parse HEAD`), the date, and per file which rules it exercises. Name every category the corpus does **not** contain (expected: images, reference-style links, HTML `<a>`, multi-line code spans) and the unit test that covers each instead.
- [ ] **Step 2: Write the tests.**
  - `TestCorpusExportsClean`: for each corpus file (kind from its folder; issue title from the record's `title:` frontmatter), run `Transform` and `corpusProblems`. Assert no problems. `corpusProblems(kind, src, out)` returns a `[]string` of failures. It runs `exportcheck.Check` (expect zero leftovers, `known` = `titles` membership) and then makes direct assertions:
    - no line of out is a frontmatter fence at line 1;
    - no `<!-- docket:`/`<!-- dckt:` outside code;
    - an issue has only allowlisted H2s;
    - a spec has no stamp line before its first H2;
    - no `](` or `<http` outside code;
    - no two consecutive blank lines;
    - no heading immediately followed by an equal-or-higher heading;
    - the fenced and code-span regions of src appear byte-identical, in order, in out.
  - Direct survivor assertions on `constructed/code.md`: the bare URL, `#N`, `PR #N`, `ADR-0012`, and the unknown id each appear verbatim in the output.
  - `TestCorpusCodeRegionsUntouched`: every fenced block and code span of `constructed/code.md` appears byte-identical in its output.
  - `TestEveryStepIsLoadBearing`: for each `s` in `allSteps`, run `transform(in, map[step]bool{s: true})` over the whole corpus. Assert that `corpusProblems` reports at least one failure for **some** file. This is the spec's "disabling any one transform step turns the corpus test red", made a standing guard. If a step stays green, the corpus is missing the case: add the real file that exercises it (or a constructed one, noted in PROVENANCE). Never weaken the guard.
- [ ] **Step 3: Run** `go test -count=1 ./internal/export/ -run 'TestCorpus|TestEveryStepIsLoadBearing' -v`. Expected: PASS. Fix the transform, never the frozen files, for any real leftover. Keep the `-v` output lines for the results file: which step reddened which file.
- [ ] **Step 4: Confirm the repo-wide guards ignore the corpus.** Run `go test -count=1 ./internal/repoguard/`. Expected: PASS (`testdata` is skipped by `repoguard.go`). If a guard trips on the corpus, add a bounded exclusion naming `internal/export/testdata/corpus/`, plus a planted violation just outside it that still reddens (learning: *frozen-fixture-corpus-trips-repo-wide-scans*).
- [ ] **Step 5: Commit** `git add internal/export && git commit -m "test(export): frozen real-artifact corpus exports clean, and every transform step is load-bearing"`

---

### Task 7: `Open --export` in the app layer

**Files:**
- Create: `internal/app/open_export.go`
- Modify: `internal/app/open.go` (`OpenOptions`, `OpenDeps`, `OpenResult`, `Open`, `resolveOpenChange`), `internal/cli/presenter.go`
- Test: `internal/app/open_export_test.go` (reuse `newOpenFixture`/`openCorpus` from `open_test.go`; add a `writeBody(t, rel, body)` helper beside `writeFile`), `internal/cli/presenter_test.go` (or the existing presenter test file)

**Interfaces:**
- Consumes: `export.Transform`, `export.FileName`, `export.Kind*`, `export.Titles` (Task 4); `exportcheck.Check` (Task 5).
- Produces (package `app`):
  ```go
  // OpenOptions gains:
  Export     bool
  OutputPath string // "" = default destination
  Warnings   string // "" = WarningsAll
  // OpenDeps gains (production sets all three explicitly; tests inject non-defaults):
  HomeDir func() (string, error)
  Getwd   func() (string, error)
  GOOS    string
  // OpenResult gains:
  ExportedPath string          `json:"exported_path,omitempty"`
  Replaced     bool            `json:"replaced,omitempty"`
  Leftovers    []ExportLeftover `json:"leftovers,omitempty"`
  Mentions     *ExportMentions  `json:"mentions,omitempty"`
  type ExportLeftover struct { Line int `json:"line"`; Text string `json:"text"`; Rule string `json:"rule"` }
  type ExportMentions struct { Count int `json:"count"`; Lines []int `json:"lines"` }
  const (WarningsAll = "all"; WarningsLeftovers = "leftovers"; WarningsMentions = "mentions")
  const (
      ReasonOpenInvalidOption     = "invalid-option"      // usage refusals (ResultInvalidInput)
      ReasonOpenOutputPathInvalid = "output-path-invalid" // missing parent / default dir missing (ResultInvalidState)
      ReasonOpenExportFailed      = "export-failed"       // read/write error (ResultExternalFailed)
  )
  // ValidateOpenExport is the one usage check, shared by the CLI's Args and Open.
  func ValidateOpenExport(o OpenOptions) error
  func (r OpenResult) HumanWarnings() []string
  // resolveOpenChange now also returns the snapshot it built:
  func resolveOpenChange(ctx, reader, pin, id, branch) (domain.Change, domain.Snapshot, *openFailure)
  ```
  The reason constants follow `open.go`'s existing `ReasonOpen*` block. `TestNoInlineFindingCodeLiterals` already accepts that site.

Flow when `o.Export` is set:
1. After the target check, run `ValidateOpenExport` (failure → `ResultInvalidInput`, `ReasonOpenInvalidOption`).
2. Resolve the change and snapshot, then take the artifact path (unchanged refusals). Skip the GitHub-URL branch entirely: read through `localArtifact`, keeping its staleness note.
3. Build `Titles` from `snap.Changes()`, keeping only ids that occur exactly once.
4. `Transform`, then `Check` (`known` = `Titles` membership). Append each `Stray` as `ExportLeftover{Line: SourceLine, Rule: "stray-marker", Text}`.
5. Destination: `exportDestination` resolves default vs `--output-path` per Global Constraints. A relative path joins `Getwd()`. Name via `FileName` (created from `c.Created()` when `FieldPresent`: `Value.Format("2006-01-02")`).
6. Write atomically: a temp file in the destination directory, write, `os.Chmod` 0o644 explicitly (learning: *promised-file-mode-needs-explicit-chmod*), then rename. Set `Replaced` if the destination existed before; when it did, add the note `replaced <path>`.
7. Filter by `Warnings`. Set `ExportedPath` and `Target` (`TargetKindFile`) to the destination, then `finishOpen`.

Human warnings: `leftover line N (<rule>): <text>` per leftover, and one `N mentions of docket on lines a, b, …` when the count is > 0. The wording is plain-language finding style. A stray marker reads `damaged marker removed at source line N: <text>`. The presenter prints each as `warning: <line>` to stderr after the notes, in human mode only.

- [ ] **Step 1: Write the failing tests** (`open_export_test.go`, using the fake-reader fixture; `HomeDir`/`GOOS`/`Getwd` injected; temp dirs only):
  - `TestOpenExportWritesCleanCopy`: spec 7 with a body holding frontmatter-free spec text, a backlink block, and `change 0003` (expect change 3's title exactly as the `openChange` helper records it, in quotes) under `--print` and `OutputPath` = temp dir. Expect `applied`, `Launched` false, `ExportedPath == Target == <dir>/2026-01-02-spec-widget.md`, and the file content exactly the expected clean bytes with mode 0644.
  - `TestOpenExportOpensTheCopy`: without `Print`, `fakeOpener.targets == [exported path]` and `Launched` is true.
  - `TestOpenExportIgnoresGitHubRouting`: shared + GitHub web URL + default `open.artifacts` → the target is still the local exported file, and `Prepare` was called.
  - `TestOpenExportDefaultDestination`: `GOOS: "darwin"` + home with `Downloads/` → `<home>/Downloads/…`. `GOOS: "darwin"` without it → `output-path-invalid`, and the message names `--output-path`. `GOOS: "linux"` → `<home>/…`. These pin the injected values (learning: *defaulted-param-hides-caller-wiring*).
  - `TestOpenExportOutputPath`: an existing dir → normal name inside it. A file path → exactly that path. A relative `out.md` → `Getwd()/out.md`. A missing parent → `output-path-invalid` and nothing written. Exporting twice → the second result has `Replaced` and the `replaced` note.
  - `TestOpenExportIssueNameAndH1`: `change 7` → `<created>-issue-widget.md`, starting `# <title>`.
  - `TestOpenExportWarningsFilter`: a source leaving a stray marker plus `docket` mentions. `all` → both. `leftovers` → `Mentions == nil`. `mentions` → `Leftovers == nil`. Exit result is `applied` in every case.
  - `TestOpenExportAmbiguousIDLeftAsWritten` (Review Focus 4): two blobs claiming id 3 → `change 0003` stays verbatim.
  - `TestValidateOpenExport`: `board`/`pr` + Export; OutputPath without Export; Warnings without Export; Warnings `"bogus"` → each an error. Valid combos → nil.
  - `TestOpenResultExportJSON`: the marshalled keys `exported_path`, `replaced`, `leftovers[].line/text/rule`, `mentions.count/lines`; a non-export result carries none of them.
  - Presenter: `TestPresenterWritesWarningsToStderr`: stdout exactly the path; stderr `note: …` lines then `warning: …` lines; JSON mode writes nothing to stderr.
- [ ] **Step 2: Run** `go test -count=1 ./internal/app/ -run 'TestOpenExport|TestValidateOpenExport|TestOpenResultExportJSON|TestOpen' && go test -count=1 ./internal/cli/ -run TestPresenter`. Expected: FAIL on the new tests.
- [ ] **Step 3: Implement** as specified. Update the three existing callers of `resolveOpenChange` for the new return.
- [ ] **Step 4: Run** the step 2 commands. Expected: PASS. Also run `go test -count=1 ./internal/app/ -run 'Schema|EveryResultStruct'` (the schema reflects the new `OpenResult` fields) and `go test -count=1 -tags integration -run '^TestOpen' ./internal/app/`. Expected: PASS.
- [ ] **Step 5: Commit** `git add internal/app internal/cli/presenter.go internal/cli/*presenter*_test.go && git commit -m "feat(open): --export writes a clean Markdown copy, reports leftovers and mentions, and opens it"`

---

### Task 8: CLI flags, help, docs, and the catalog pin

**Files:**
- Modify: `internal/cli/open.go`, `internal/cli/open_test.go`, `internal/cli/capability_production_test.go`, `docs/reference/cli.md`

**Interfaces:**
- Consumes: `app.ValidateOpenExport`, the new `OpenOptions`/`OpenDeps` fields (Task 7).

- [ ] **Step 1: Write the failing tests** in `internal/cli/open_test.go`:
  - `TestOpenCLIExportOptions`: `open spec 541 --export --output-path /tmp/x --warnings leftovers --print --repo-dir /tmp/repo` → the stub sees `OpenOptions{RepoDir:"/tmp/repo", What:"spec", ID:541, Print:true, Export:true, OutputPath:"/tmp/x", Warnings:"leftovers"}`. Without `--warnings` → `Warnings: ""`.
  - `TestOpenCLIExportUsageErrors`: `open board --export`, `open pr 5 --export`, `open spec 5 --output-path x`, `open spec 5 --warnings all`, `open spec 5 --export --warnings bogus`, `open spec 5 --export --output-path ""` → each exit 2, one stderr line, and the stub not reached.
  - `TestOpenCLIExportDepsWiring`: call the real `openDepsFor()` (with git available) and assert `GOOS == runtime.GOOS` and `HomeDir`/`Getwd` non-nil.
  - In `TestRepresentativeSignatures`, update the `open` row to the measured signature. The expected value is `<what> [<id>] [--export] [--output-path <path>] [--print] [--repo-dir <dir>] [--warnings <which>=all]`. Pin what `capabilities --json` actually prints, after checking it against the flag registrations.
- [ ] **Step 2: Run** `go test -count=1 ./internal/cli/ -run 'TestOpenCLI|TestRepresentativeSignatures'`. Expected: FAIL.
- [ ] **Step 3: Implement.**
  - Register `--export` (bool), `--output-path` (usage with a backquoted `` `path` ``), and `--warnings` (default `all`, backquoted `` `which` ``, naming all three values).
  - In `Args`, after `ParseOpenArgs`, build the options from the flags (`Warnings` only when `Changed`) and return `app.ValidateOpenExport`'s error. Refuse an explicitly empty `--output-path`.
  - Wire `HomeDir: os.UserHomeDir`, `Getwd: os.Getwd`, `GOOS: runtime.GOOS` in `openDepsFor`.
  - Extend `Long` with the export flags, destinations, the filename shape, and what the export removes.
  - Update the `docket open` entry in `docs/reference/cli.md` to say it can also export a clean Markdown copy (`--export`), pointing to `docket open --help` for details. Current behavior only, no change citations.
- [ ] **Step 4: Run** `go test -count=1 ./internal/cli/ -run 'TestOpenCLI|TestRepresentativeSignatures|TestCapabilitiesPayloadWithinByteBudget' -v`. Expected: PASS, and the `capabilities payload: N bytes` log line ≤ 16384 (record N). If it is over, stop and report BLOCKED. Do not raise the ceiling. Then run `go test -count=1 ./internal/repoguard/` (docs-alignment guards). Expected: PASS.
- [ ] **Step 5: Commit** `git add internal/cli docs/reference/cli.md && git commit -m "feat(cli): docket open --export, --output-path, and --warnings"`

---

### Task 9: Real-repository sweep (build-time evidence, no commit)

The suite cannot see the metadata branch (learning: *metadata-branch-invisible-to-suite*), so verify against the real artifacts once and record the result.

- [ ] **Step 1:** Make a sweep directory with `out=$(mktemp -d "${TMPDIR:-/tmp}/docket-export-sweep.XXXXXX")`. From the feature worktree, for every change id in `docs/changes/active` and `docs/changes/archive` of the metadata checkout, and for each of `change spec plan results`, run `go run ./cmd/docket open <what> <id> --export --print --output-path "$out" --json --repo-dir .`. Build once with `go build -o "$out/docket" ./cmd/docket` and loop over the built binary. Skip `artifact-unset` refusals. Tally the `leftovers` per rule.
- [ ] **Step 2:** Expected: zero leftovers other than `stray-marker` on genuinely damaged sources. For any other leftover, fix the transform rule (and add the case to the corpus or a unit test), then re-run. Also spot-read three outputs (one per kind) for natural reading.
- [ ] **Step 3:** Report in the task's completion report, for the results file:
  - the files swept, the leftover counts per rule, and the mention totals;
  - the catalog byte count from Task 8;
  - the `TestEveryStepIsLoadBearing` step → file mapping from Task 6;
  - the multi-line code-span limitation.

  Remove `$out`.
