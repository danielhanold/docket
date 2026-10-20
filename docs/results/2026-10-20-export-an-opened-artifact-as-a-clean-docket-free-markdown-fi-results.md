<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0561 — Export an opened artifact as a clean, docket-free Markdown file](../changes/active/0561-export-an-opened-artifact-as-a-clean-docket-free-markdown-fi.md)**
<!-- docket:backlink:end -->

# Export an opened artifact as a clean, docket-free Markdown file — Results

**Human action:** No action is required before merge. One optional walkthrough is suggested: export a real spec and read the copy as an outside reader would.

## Outcome

Sharing a spec, plan, results file or change with someone who does not use docket used to mean deleting docket's machinery from a copy by hand. `docket open <change|spec|plan|results> [<id>] --export` now does that. It writes a clean Markdown copy and opens it in the default Markdown app. With `--print`, it prints the path and opens nothing.

- **Where the copy goes:** `~/Downloads` on macOS or the home folder on Linux, named `<date>-<type>-<name>.md` (`issue`, `spec`, `plan` or `results`). Use `--output-path` to choose another directory or file. Exporting over the source artifact, or into the metadata checkout, is refused (`output-path-protected`).
- **What is removed:** frontmatter, docket's generated blocks and their marker comments, the spec's change stamp (old shapes included), and every link (its text is kept). For a change, only the proposal sections are kept, under a title heading.
- **What is converted:** explicit change references (`change 0559`, `the 0111 guard`) become the change's title in quotes. A bare `#N`, a PR number or an ADR reference stays as written.
- **Nothing inside code** (fences and inline code spans, including spans that cross a line break) is touched.
- **Self-check:** the exported file is checked independently for leftovers and for mentions of "docket". The results are printed as `warning:` lines and never block the export. `--warnings all|leftovers|mentions` chooses which are shown. `--json` carries the same lists; a damaged source marker reports its `source_line`.

## Human actions and testing

### Optional — export a real spec and read it

Shows what a teammate will receive.

1. From the repository, after this change is installed, run `docket open spec 561 --export --print`.
   Expected: one path, ending in `-spec-export-an-opened-artifact-as-a-clean-docket-free-markdown-fi.md`, on stdout. Any `warning:` lines go to stderr. The mentions line lists lines that say "docket".
2. Open that file.
   Expected: one H1 title, no `<!--` markers, no `↩` banner, no links, no `Change #561, groomed …` line.
3. Run `docket open spec 561 --export --output-path .docket/docs`.
   Expected: refused with `output-path-protected`, and nothing written.

Cleanup: delete the exported file from `~/Downloads`.

## Verification performed

- The full suite (`go run ./cmd/docket development test`) is green at the final head. It ran twice: once after the build and once after the review fixes.
- Corpus guard: 18 real artifacts copied from the metadata branch plus 2 hand-built files all export with zero leftovers. A standing test proves that switching off any one of the seven transform steps turns the corpus test red. Change-reference conversion now has its own direct assertion.
- Real-repository sweep, done once during the build because the suite cannot see the metadata branch: 2,292 export attempts across all 573 change ids produced 1,075 exports.
  - It found 8 leftovers, all author prose. The review fixes removed the false-positive rules behind them (any `↩`, `<a>` placeholders, undefined `[a][b]`).
  - The sweep stubbed out repository prepare: a real `docket open` spends about 70 seconds in prepare in this repository (see follow-ups).
  - After the stamp fix, all 246 real specs with a stamp exported with zero stamp leftovers.
- Capability catalog: 16,205 of 16,384 bytes.
- Whole-branch review (deep tier, bumped for diff size): 7 findings (2 important, 5 minor), all fixed in-branch. The full table is in the PR body.

## Known issues and follow-ups

- **Some cases still read imperfectly (confirmed, in scope).**
  - A stamp line that wraps onto a second line leaves the continuation behind (for example "Approved design").
  - Metadata lines without a `Change` field (`**Status:** …`) stay.
  - The quote-awareness that stops doubled quotes looks only at the current line.
  - A reference to another repository's change, such as "Markhaus change #0004", is converted to this repository's change 4, because the export cannot tell repositories apart.

  Impact: a few odd lines in rare exports. Workaround: edit the exported copy. Next action: none unless someone reports it.
- **`docket open` is slow in this repository (confirmed, out of scope).** About 70 seconds per call, almost all of it in repository prepare (`verifyMetadataOwnership` → `verifyLegacyEquivalence`). It affects every `docket open`, not only exports, and this branch does not change it. Next action: profile prepare's legacy-equivalence check. Related to #360 (its prepare/sync cost is for implement-next runs; folding in `open`'s cost would widen what it is for) — capture as a new change linking #360 under `related:`.
