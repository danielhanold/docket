package app

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/danielhanold/docket/internal/document"
)

// validateTitle is the one title-shape rule change.create and change.groom
// share (change 0461). A title must be non-empty after trimming (empty-title),
// valid UTF-8, a single line, and free of every rune the frontmatter writer
// refuses — control characters (tab included, though the writer tolerates it:
// a title is one board cell and one backlink line), U+2028/U+2029, and
// U+FFFE/U+FFFF (invalid-title). The rune set is document.IllegalTextRune, the
// writer's own predicate, so a title this admits always serializes. It returns
// ("", "") for a valid title.
func validateTitle(title string) (FindingCode, string) {
	if strings.TrimSpace(title) == "" {
		return FCEmptyTitle, "title must be non-empty"
	}
	if !utf8.ValidString(title) {
		return FCInvalidTitle, "title must be valid UTF-8"
	}
	if strings.ContainsAny(title, "\r\n") {
		return FCInvalidTitle, "title must be a single line (no line breaks)"
	}
	for _, r := range title {
		if document.IllegalTextRune(r) {
			return FCInvalidTitle, fmt.Sprintf("title must not contain control or line-separator characters (found %U)", r)
		}
	}
	return "", ""
}
