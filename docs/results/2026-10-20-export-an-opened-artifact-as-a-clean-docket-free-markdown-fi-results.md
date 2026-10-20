<!-- docket:backlink:start (generated — do not hand-edit) -->
> ↩ **[Change 0561 — Export an opened artifact as a clean, docket-free Markdown file](../changes/active/0561-export-an-opened-artifact-as-a-clean-docket-free-markdown-fi.md)**
<!-- docket:backlink:end -->

# Export an opened artifact as a clean, docket-free Markdown file — Results

**Human action:** Assessment pending; the review fix pass is still running.

## Outcome

`docket open <change|spec|plan|results> [<id>] --export` writes a cleaned Markdown copy of the artifact (to `~/Downloads` on macOS or the home folder on Linux, or to `--output-path`) and opens it. The build is complete and the full suite is green. The whole-branch review returned 7 findings, which are being fixed in-branch.

## Known issues and follow-ups

Review findings returned (deep tier), pending fixes:

- Important: about a third of real spec stamps use older shapes (for example `Change: #0036 · …` or `**Change:** [#0104](…)`) that the stamp removal misses, and the self-check cannot see them after conversion.
- Important: the self-check flags author prose as leftovers: any `↩`, an `<a>` placeholder, `x[i][j]`.
- Minor: an inline code span that crosses a line break is not protected.
- Minor: an inserted title clashes with the author's own quotes (`""title""`, a quoted `"(0507)"` becomes `""`).
- Minor: stray-marker leftovers report a source line in a field defined as an output line.
- Minor: `--output-path` can overwrite the source artifact.
- Minor: the corpus test has no direct assertion for change-reference conversion.
