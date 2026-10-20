package export

import (
	"path/filepath"
	"regexp"
	"strings"
)

// leadingDateRE splits a basename stem into its leading YYYY-MM-DD date and the rest.
var leadingDateRE = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})-(.+)$`)

// FileName returns the exported file's name, <date>-<type>-<name>.md.
//
// For a change record (KindIssue) the date is created and the name is slug. For a
// spec, plan, or results artifact the date is the source basename's leading
// YYYY-MM-DD and the name is the rest of the stem; a stem with no leading date
// takes created and keeps the whole stem. A spec's trailing "-design" and a
// results file's trailing "-results" are dropped, since the type word carries
// them. created is "YYYY-MM-DD" or ""; with no date at all the name starts with
// the type word.
func FileName(kind Kind, sourcePath, created, slug string) string {
	date, name := created, slug
	if kind != KindIssue {
		base := filepath.Base(sourcePath)
		name = strings.TrimSuffix(base, filepath.Ext(base))
		if m := leadingDateRE.FindStringSubmatch(name); m != nil {
			date, name = m[1], m[2]
		}
		switch kind {
		case KindSpec:
			name = strings.TrimSuffix(name, "-design")
		case KindResults:
			name = strings.TrimSuffix(name, "-results")
		}
	}
	out := string(kind) + "-" + name + ".md"
	if date != "" {
		out = date + "-" + out
	}
	return out
}
