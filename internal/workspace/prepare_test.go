package workspace

import (
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/testsupport"
)

// TestClassifyRegistrationAbsence pins the fail-closed three-outcome
// registration probe (change 0368). It needs no git: it drives the pure
// classifier over synthetic []gitcli.WorktreeInfo. Two intended-path shapes are
// exercised: an existing canonical dir (a live registration at it canonicalizes
// and matches) and a not-yet-created canonical leaf (mirroring intendedPath for
// an unallocated worktree, where an exact registration fails canonicalization
// and must match lexically as a stale registration).
func TestClassifyRegistrationAbsence(t *testing.T) {
	feature := gitcli.RefName("refs/heads/feat/absent-probe")
	other := gitcli.RefName("refs/heads/feat/other")

	existingWant, err := canonicalizePath(testsupport.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	missingParent, err := canonicalizePath(testsupport.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	missingWant := filepath.Join(missingParent, "ws")         // never created
	gone := filepath.Join(testsupport.TempDir(t), "vanished") // never created: canonicalization fails, cleans to != want

	cases := []struct {
		name  string
		infos []gitcli.WorktreeInfo
		want  string
		exp   registrationAbsence
	}{
		{"empty list is absent", nil, existingWant, regAbsent},
		{"unrelated registration is absent", []gitcli.WorktreeInfo{{Path: testsupport.TempDir(t), Branch: other}}, existingWant, regAbsent},
		{"live registration at the path is present", []gitcli.WorktreeInfo{{Path: existingWant, Branch: other}}, existingWant, regPresent},
		{"registration on the feature ref elsewhere is present", []gitcli.WorktreeInfo{{Path: testsupport.TempDir(t), Branch: feature}}, existingWant, regPresent},
		{"stale registration at the exact path is present", []gitcli.WorktreeInfo{{Path: missingWant, Branch: other}}, missingWant, regPresent},
		{"unresolvable unrelated registration is unresolved", []gitcli.WorktreeInfo{{Path: gone, Branch: other}}, existingWant, regUnresolved},
		{"present beats unresolved", []gitcli.WorktreeInfo{{Path: gone, Branch: other}, {Path: existingWant, Branch: other}}, existingWant, regPresent},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyRegistrationAbsence(tc.infos, tc.want, feature); got != tc.exp {
				t.Errorf("classifyRegistrationAbsence = %v; want %v", got, tc.exp)
			}
		})
	}
}
