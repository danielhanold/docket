package app

import (
	"regexp"
	"strings"
	"testing"
)

// TestVisibilitySubjectsCarryNoModeVocabulary keys on the subject VALUES the
// switch can commit with, so a later edit cannot slip mode vocabulary in.
func TestVisibilitySubjectsCarryNoModeVocabulary(t *testing.T) {
	subjects := []string{visibilityRemoveSubject, visibilityAddSubject}
	if len(subjects) != 2 || subjects[0] == subjects[1] {
		t.Fatalf("want exactly two distinct subjects, got %q", subjects)
	}
	banned := regexp.MustCompile(`(?i)shared|public|private|visibility`)
	for _, s := range subjects {
		if strings.TrimSpace(s) == "" || strings.ContainsAny(s, "\r\n") {
			t.Errorf("subject %q is not one non-empty line", s)
		}
		if m := banned.FindString(s); m != "" {
			t.Errorf("subject %q carries mode vocabulary %q", s, m)
		}
		if !validVisibilitySubject(s) {
			t.Errorf("subject %q is not accepted by validVisibilitySubject", s)
		}
	}
}
