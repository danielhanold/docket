# Export an opened artifact as a clean, docket-free Markdown file — design

Change 0561 — Export an opened artifact as a clean, docket-free Markdown file

Change #561, groomed 2026-10-20.

## Problem

People share docket artifacts with teammates who do not all use docket — most often a spec, sent as a Markdown file by email or Teams. Today the sender copies the file and deletes the docket parts by hand. As stored, every artifact carries machinery an outside reader cannot use: frontmatter, the ↩ backlink banner, the `## Artifacts` table, build-evidence blocks and their `<!-- docket:… -->` marker comments, links into files the reader cannot open, the spec's `Change #N, groomed …` stamp, and change references (`change #547`, `change 0559`, `the 0111 guard`) woven into sentences.

## Goal

One shell command a human runs (no agent session involved) that writes a clean Markdown copy of a change, its spec, its plan, or its results: nothing docket-specific is left outside prose and code, references read naturally, the heading structure is well formed, and the person is told about anything that slipped through.

## Non-goals and anti-goals

- **No board export.** The board is out of scope entirely (Daniel's decision).
- **No prose rewriting.** A sentence that talks about docket as a subject stays as written; the export removes docket's bookkeeping and, beyond step 6's change-reference conversions, does not edit the author's content. Mentions are reported instead (see *Self-check*).
- **No general Markdown reformatter.** The sources are already consistent (measured across 630 files: dash bullets only, no malformed headings, exactly one H1 first in every spec/plan/results file, no skipped heading levels). The export only cleans up what its own removals leave behind, adds the H1 an issue lacks, and reports heading-rule problems — it never renumbers headings or reflows tables.
- **No guessing at bare `#N`.** A `#N` with no `change` word before it stays exactly as written and is never warned about (Daniel's decision). The export cannot tell a change from a GitHub issue or PR, a step, or any other numbering: in this repository 1,639 of the 2,078 bare `#N` change references are numbers a real PR also has, and a repository that starts at change 1 collides with `#12` or `Step #3` just as easily. A word-list exclusion fails the other way (`fixes #515`, `case #559`, `rule #533` are real change references). The export is for a person to read; what they do with a `#N` afterwards is outside its job.
- **ADR references stay.** "ADR" is general vocabulary, not docket's: `ADR-0147` and `ADR-0065's` stay as written (Daniel's decision). Only a link around one is removed (step 5).
- **Never block.** A leftover is a warning, never a refusal to write (Daniel's decision; matches docket's visibility-only default for new checks).
- **No agent, no AI.** Every conversion is deterministic Go in the `docket` binary.
- No other export formats (.docx, PDF), no change to how artifacts are stored, no Windows support (docket ships only macOS and Linux; Windows is out entirely — Daniel's decision).

## Command surface

A flag on the existing command, not a new command (Daniel agreed; it reuses `docket open`'s target resolution; the longer `open` signature costs about 78 bytes of the 16 KB capability catalog — JSON escapes `<`/`>` and `--warnings` renders with its default — taking it from about 16,186 to about 16,264 of 16,384 bytes, so it fits without a ceiling raise):

```
docket open <change|spec|plan|results> [<id>] --export [--output-path <path>] [--warnings all|leftovers|mentions] [--print]
```

- `--export` writes the clean copy, then opens **that copy** with the platform opener (the default Markdown app), exactly as `open` launches a local file today. With `--print`, it writes the copy and prints its path without launching anything.
- `--export` always reads the **local metadata checkout** file — also in a shared repository and regardless of `open.artifacts` — through the existing `localArtifact` path (prepare first; a failed prepare adds today's staleness note and proceeds).
- `--output-path <path>` (Daniel's name; optional) overrides the destination. An existing directory means "put the file in here under its normal name"; anything else is the exact file path to write. A relative path resolves against the working directory. A parent directory that does not exist is refused with one line (`output-path-invalid` or similar; builder names it).
- `--warnings` (Daniel's addition; optional, default `all`) selects which warning lists are reported: `all`, `leftovers`, or `mentions`. Any other value is a usage error.
- Refusals (usage errors, nothing written): `--export` with `board` or `pr`; `--output-path` or `--warnings` without `--export`; an invalid `--warnings` value. Existing refusals (unknown change, artifact unset, file missing, path outside the repository) apply unchanged.
- No id: inferred from the checked-out branch, as today.

## Destination and filename

**Default directory** (Daniel's decision): `~/Downloads` on macOS, the home directory (`os.UserHomeDir`) on Linux. A missing `~/Downloads` on macOS is refused with one line naming `--output-path` (my call).

**Filename** (Daniel's decision on the shape): `<date>-<type>-<name>.md`, where `<type>` is `spec`, `plan`, `results`, or `issue` (for a change record). Examples from #559:

| Export | Source basename | Exported filename |
|---|---|---|
| spec | `2026-10-17-close-a-change-to-done-without-a-pr-design.md` | `2026-10-17-spec-close-a-change-to-done-without-a-pr.md` |
| plan | `2026-10-17-close-a-change-to-done-without-a-pr.md` | `2026-10-17-plan-close-a-change-to-done-without-a-pr.md` |
| results | `2026-10-17-close-a-change-to-done-without-a-pr-results.md` | `2026-10-17-results-close-a-change-to-done-without-a-pr.md` |
| change | `2026-10-17-0559-close-a-change-to-done-without-a-pr.md` | `2026-10-17-issue-close-a-change-to-done-without-a-pr.md` |

- spec/plan/results: `<date>` is the source basename's leading `YYYY-MM-DD`; `<name>` is the rest of the basename with a trailing `-design` (spec) or `-results` (results) removed (my call — the type word now carries that). A basename with no leading date takes the change's `created:` date and uses the whole stem as `<name>`.
- change: `<date>` is the record's `created:`; `<name>` is its `slug` (my call — active record names carry no date, and `created:` matches how the other artifacts are dated).
- An existing file at the destination is replaced; the human output says so (my call — re-exporting after an edit is the common case).

## The transform

Input: the artifact's bytes from the metadata checkout, plus the pinned snapshot `docket open` already builds (`resolveOpenChange` → `repository.BuildSnapshot`), whose corpus already includes every change — the source of reference titles.

**Fence and code awareness everywhere:** nothing inside a fenced code block or an inline code span is ever changed — no removal, no link rewrite, no reference substitution (the existing fence-aware scanners in `internal/document` and `internal/render/section.go` show the pattern). A link that wraps a code span (`` [`internal/app/open.go`](…) ``) is outside the code, so the link goes and the code span stays.

Steps, in order:

1. **Frontmatter.** Remove the leading `---…---` block (reuse `internal/document`'s frontmatter locator; never a bare column-0 `---` match).
2. **Managed blocks.** Remove every docket managed marker block — start marker, contents, end marker — in either namespace (`docket:` and the neutral `dckt:`; see `document.NeutralMarkerPrefix`): the backlink banner, the `## Artifacts` link block, `docket:build-evidence`, and any other block. Reuse the document package's fence-aware block scanner. A stray, unpaired marker line is removed too and reported as a leftover (it means the source is damaged).
3. **Issue only — keep the proposal.** For a change record, the body keeps **only** `## Why`, `## What changes`, `## Out of scope`, and `## Open questions`, in source order; every other H2 section is dropped (Daniel's decision: drop both docket bookkeeping — Reconcile log, Reclaim log, Build evidence, Closeout notes, Run halted, Finalize blocked, Auto-groom blocked — and the human-written reasons — Why deferred, Why killed, Why on hold). This is an **allowlist** on purpose (my call): the records carry ~30 one-off headings (e.g. "Why the run halted", "Triage note (…)") that a drop-list would leak; the cost is that rare hand-written extras like `## Design decisions` (20 old records) and `## Notes` (3) are dropped too. Then insert an H1 of the change's `title` at the top.
4. **Spec header stamps.** Docket's grooming has written a change stamp near the top of a spec in several shapes over time (my call to cover them all, since older specs get shared too). Remove, before step 6:
   - a stamp sentence of the shape `Change #N, groomed YYYY-MM-DD.` or `Change N. Groomed YYYY-MM-DD.` (with an optional parenthetical such as `(interactive)`); when the line carries more text after the stamp sentence, only the stamp sentence goes and the rest of the line stays and continues to step 6 (`Change #N, groomed …. Found while building change #M (PR #K).` → `Found while building "<title of M>" (PR #K).`);
   - a line that is only `Change N.` or `Change #N.`;
   - a line that is only `**Change:** N` / `**Change:** #N`, or a `·`-separated run of bold `**Key:** value` pairs that includes `**Change:**` (e.g. `**Change:** N · **Type:** feat · **Priority:** critical · **Date:** … · **Status:** …`).
   Any other line beginning with a change reference goes through step 6 normally.
5. **Links — every one.** Every link is removed and its visible text kept, whatever it points at (Daniel's decision: the reader cannot follow docket links, and one rule needs no way to tell docket links from others — a docket-only rule would also have missed the ~700 links in older archived records that reach docket files through `github.com/…/blob/main/docs/…` URLs from when artifacts lived on main). Measured across the artifacts outside code and frontmatter: 1,742 relative links, ~700 `blob/main` links, 15 external, 13 metadata-branch, 6 autolinks, 1 PR link, 1 in-page anchor.
   - an inline link `[text](target)` becomes `text` — relative, `https://`, `mailto:`, and in-page `#anchor` targets alike: `[Run Modes](https://cursor.com/docs/…)` → `Run Modes`, `[PR 265](https://github.com/…/pull/265)` → `PR 265`;
   - an autolink `<https://…>` becomes the URL as plain text;
   - an image `![alt](target)` becomes its alt text, and is removed when the alt text is empty; a reference-style link (`[text][ref]`, `[text][]`, or a `[text]` shortcut that matches a definition) becomes `text`, and every link-reference definition line (`[ref]: url`) is removed; an HTML `<a href="…">text</a>` becomes `text` (all my calls; none of these occur in the artifacts today);
   - a bare URL typed into a sentence (`see https://go.dev/doc/toolchain`) is not link syntax: it stays as plain text (Daniel's decision), and viewers may still make it clickable.
   The kept text then goes through step 6 like any other text: `[#559](../changes/…)` → `#559`, which stays; `[change 0154](…)` → a converted title.
6. **Change references.** Converted **only** where the text plainly names a docket change (Daniel's decision); everything else stays as written (see *Non-goals*: bare `#N`, `PR #N`, ADR references). The converted forms:
   - the word `change`/`changes` (case-insensitive) followed by a number with or without `#`: `change 559`, `Change #547`, `change 0363`, and a list after the plural word, `changes #461 and #550` or `changes 0113, 0114 and 0120` (each number in the list);
   - a zero-padded id, which is docket's own id format: `#0559`, `(0559)`, and a bare zero-padded id standing as its own word (`the 0111 guard`, `0282's`, `mirroring 0088` — my call), never when it is part of a path or filename such as `0561-export…md`.
   A form converts **only** when its number is a change id present in the snapshot; an unknown number is left exactly as written. (Checked across the artifacts: of 268 unpadded `change N` uses whose N is a real change id, none is the verb — "change 3 files" — so the word form is safe.) How a converted reference reads:
   - **Inside a parenthesis that holds only references** (with commas, `and`, whitespace): the converted references and their separators are removed, and the whole parenthetical, with the space before it, goes when nothing is left: `fields (0461, 0550)` → `fields`. A kept item stays in its parenthesis: `(0461, ADR-0012)` → `(ADR-0012)`; `(#461, ADR-0012)` is unchanged.
   - **Followed by a spaced dash (` — `, ` – `, or ` - `) and that change's own title**: the reference and the dash are removed, leaving the title once: `# Change 0466 — Bring test_go_race back under its 60s row…` → `# Bring test_go_race back under its 60s row…` (three older specs open this way; my call).
   - **Anywhere else**: the reference, including its `change`/`Change` word, becomes the change's title in straight double quotes: `Change #547 holds the full measurement` → `"Measure whether the 1h cache TTL cut implement-next's cost" holds the full measurement`; `from change 0363 forbids it` → `from "Remove main-mode compatibility from Go v1" forbids it`. A list becomes `"A", "B" and "C"`. (Daniel approved these readings.)
   - **Inserted titles are cleaned once, never recursively** (my call). Some change titles contain change references ("Clear the unfixed review findings from change 0113") and some contain double quotes. Before a title is inserted — and for the issue's own H1 from step 3 — it gets one pass: a parenthetical holding only convertible references is removed, any other convertible change reference becomes `another issue`, or `other issues` for a list (`"Clear the unfixed review findings from another issue"`; Daniel's decision: wherever the export itself writes the word, it says "issue", the outside reader's word, matching the `issue` filename type), bare `#N` and ADR references inside it stay, and its double quotes become single quotes so the outer quotes stay unambiguous. No title is ever looked up from inside a title, so the pass always terminates, and an inserted title never re-triggers the self-check.
7. **Cleanup of what removal left.** Drop a heading whose section is now empty (nothing but blank lines before the next heading of the same or higher level — e.g. a change's `## Artifacts`); collapse runs of blank lines to one; no leading blank lines; exactly one trailing newline.

## Self-check (warnings — never blocking)

After the transform, the export scans **its own output** with a detector that is **implemented separately from the transform but checks the same rules this spec defines** (my call). It must not call or share the transform's patterns or code — otherwise it agrees with the transform by construction and can never find anything — but it must not flag what the transform keeps on purpose either (bare `#N`, `PR #N`, ADR references, unknown ids, bare URLs, text in code), or the zero-leftover corpus test could never pass. Where it reaches further than the transform, it does so only with structural checks the transform can satisfy (any marker text at all, any top-of-file `---` block, heading rules). It produces two lists:

- **Leftovers** — docket machinery that should have been removed, each with its 1-based output line number and matched text. Outside code, at least: any `docket:`/`dckt:` marker text; a `---` frontmatter block at the top; a ↩ banner line; any link syntax at all (inline link, image, autolink, reference-style link or definition, HTML `<a`); any step-6 converted form whose number is a known change id; a stamp line. **Heading rules** are reported here too (my call — they are "the export did not produce clean output"): not exactly one H1, the H1 not the first heading, or a skipped level (H2 → H4).
- **Mentions** — every output line containing the word `docket` or `dckt`, prose **and** code (my call — teammates read both). Kept on purpose; listed so the person can review. Reported compactly: a count plus the line numbers.

`--warnings` filters which lists are reported (human and JSON output alike — my call). Warnings never change the result: the file is written, the result is `applied`, exit 0.

## Output

- Human mode: stdout is exactly the exported file's path (so `--print` composes in command substitution); notes and warnings go to stderr as today's `note:` lines do (`Presenter.Present`), e.g. `note: replaced <path>`, `warning: leftover line 12: <text>`, `warning: 14 mentions of docket on lines 3, 9, …`. Exact wording is the builder's, in docket's plain-language finding style (what, whether it matters, what to do).
- JSON mode: `OpenResult` gains the export fields (builder names them; e.g. `exported_path`, `replaced`, `leftovers: [{line, text, rule}]`, `mentions: {count, lines}`), filtered by `--warnings`. `docket schema` reflects the new fields.

## Test guard

- **Frozen real corpus.** Copy a set of real artifacts from this repository's metadata branch into a test fixture directory — specs, plans, results, and change records, active and archived, chosen to cover every removal and reference form above (including a record with bookkeeping sections, a spec with a stamp line, a spec whose H1 opens `Change N — <title>`, a results file dense with references, an older archived record with `blob/main` docket links, a shared-repo absolute metadata-branch link, external links, an autolink, a bare URL, and code spans/fences containing reference-, link-, and marker-shaped text). Direct assertions also cover what must survive unchanged: bare `#N`, `PR #N`, ADR references, unknown ids, and bare URLs. Record the fixture's inventory (which rule each file exercises, and any category it does not contain) next to it. A test exports every fixture and asserts: zero leftovers from the self-check detector, plus direct assertions for each removal so the test does not rest on the detector alone. Inputs need a snapshot with the referenced ids/titles — build it from the fixture.
- **Mutation-tested.** Disabling any one transform step must turn the corpus test red, and the build records that it checked this. The self-check detector gets the same treatment against crafted inputs carrying each leftover kind.
- **Code is untouched.** A fixture with reference-, link-, and marker-shaped text inside fences and code spans must come out byte-identical in those regions.
- Unit tests per rule (each link form, each converted reference form, parenthetical removal, the own-title dash case, bare `#N` and ADR references left alone, unknown ids, title cleaning, filename derivation, destination resolution with injected home/GOOS, `--output-path` directory-vs-file, flag refusals) and CLI tests for the new flags.
- The capability catalog stays within 16,384 bytes (`TestCapabilitiesPayloadWithinByteBudget`); the `open` signature pinned in `TestRepresentativeSignatures` updates.

## Docs

`docs/reference/cli.md`'s `docket open` entry and the command's `--help` text gain the export flags, destinations, and what the export removes — current behavior only.

## Calls left to the builder

- Package placement (e.g. a pure `internal/export` package for the transform and a separate one, or file, for the detector) and type/field names.
- Finding/reason codes for the new refusals, minted from `internal/app/finding_codes.go` constants.
- Exact warning wording, within the plain-language style above.

## Relations

- Discovered from #559 (results artifacts made sharing results common).
- Builds on #542 (`docket open`); #573 removed `open`'s trivial refusal from the same file (`internal/app/open.go`).
- The leak scanner from #532 (`internal/leakscan`, ADR-0144) is prior art: its change-reference rule matches only zero-padded ids, by design, to avoid PR numbers — the same reasoning behind step 6's narrow forms. It is not reused as the detector: it has no `change N` or link rules and scans for a different job (private-mode leaks).
