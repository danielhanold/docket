package app

import (
	"os"
	"path/filepath"
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

// TestKeepOnDiskCommittedKeepsHandFormatting proves going shared keeps a
// hand-formatted .docket.yml byte for byte when it already declares the
// split's leaves, and takes the split when a leaf differs, the file is absent,
// or the file does not parse.
func TestKeepOnDiskCommittedKeepsHandFormatting(t *testing.T) {
	split := []byte("visibility: shared\nintegration_branch: main\nbuild:\n  test_command: make\n")
	cases := []struct {
		name   string
		onDisk string // "" = absent
		keep   bool
	}{
		{"same leaves", "# mine\nbuild: {test_command: make}   # fast\nintegration_branch: 'main'\nvisibility: shared\n", true},
		{"a leaf differs", "build: {test_command: make all}\nintegration_branch: main\nvisibility: shared\n", false},
		{"absent", "", false},
		{"unparsable", "build: [unclosed\n", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			primary := testsupport.TempDir(t)
			if c.onDisk != "" {
				if err := os.WriteFile(filepath.Join(primary, docketYMLRel), []byte(c.onDisk), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got, err := keepOnDiskCommitted(primary, split)
			if err != nil {
				t.Fatal(err)
			}
			want := string(split)
			if c.keep {
				want = c.onDisk
			}
			if string(got) != want {
				t.Errorf("keepOnDiskCommitted = %q, want %q", got, want)
			}
		})
	}
}
