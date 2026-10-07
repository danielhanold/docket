package app

import (
	"regexp"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
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

// TestVisibilityJournalReplacesEmptyOtherSubject proves an open journal of the
// other subject that holds no path (an abandoned going-shared run's marker)
// gives way to a new switch's first path, while one holding a path still
// refuses.
func TestVisibilityJournalReplacesEmptyOtherSubject(t *testing.T) {
	common := testsupport.TempDir(t)
	if err := saveSwitchJournal(common, switchJournal{Subject: visibilityAddSubject, Paths: map[string]string{}}); err != nil {
		t.Fatal(err)
	}
	if err := recordSwitchDigest(common, visibilityRemoveSubject, ".docket.yml", digestDeleted); err != nil {
		t.Fatalf("recording over an empty add journal: %v", err)
	}
	j, ok, err := loadSwitchJournal(common)
	if err != nil || !ok {
		t.Fatalf("loadSwitchJournal = %v, %v", ok, err)
	}
	if j.Subject != visibilityRemoveSubject || len(j.Paths) != 1 || j.Paths[".docket.yml"] != digestDeleted {
		t.Errorf("journal = %+v, want the removal subject holding .docket.yml", j)
	}
	if err := recordSwitchDigest(common, visibilityAddSubject, ".gitignore", digestDeleted); err == nil {
		t.Error("recording over a removal journal holding a path succeeded, want a refusal")
	}
}
