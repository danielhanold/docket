// Package export turns a docket artifact (a change record, spec, plan, or results
// file) into a clean Markdown copy an outside reader can use: docket machinery is
// removed, links are reduced to their visible text, and explicit change references
// become the referenced change's title. Every rule is deterministic, and nothing
// inside a fenced code block or an inline code span is ever changed. The package is
// pure: it reads no files and writes none.
package export
