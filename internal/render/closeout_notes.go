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
