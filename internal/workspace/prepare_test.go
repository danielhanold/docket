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

// TestOwnsManifestBaseRefByPhase pins the phase-scoped base rule of
// ownsManifest. A finished checkout (ready or cleaned) stays its change's own
// after its effective base moves — a stacked child whose parent landed now
// resolves to the integration branch — so its recorded BaseRef is history, not
// identity. An interrupted allocation resumes from its recorded BaseCommit, so a
// base-ref mismatch there still reads as not owned. Every other identity field
// is compared exactly in every phase.
func TestOwnsManifestBaseRefByPhase(t *testing.T) {
	repo := gitcli.Repository{PrimaryWorktree: "/repo", CommonDir: "/repo/.git"}
	tgt, err := NewTarget(7, "stacked-child", resolvedBase("main"), "feat/stacked-child")
	if err != nil {
		t.Fatalf("NewTarget: %v", err)
	}
	intended := filepath.Join(repo.PrimaryWorktree, ".worktrees", tgt.Slug)
	owned := func(phase Phase) Manifest {
		return Manifest{
			Schema:     manifestSchemaVersion,
			ID:         workspaceID(tgt.FeatureRef),
			CommonDir:  repo.CommonDir,
			ChangeID:   tgt.ChangeID,
			Slug:       tgt.Slug,
			FeatureRef: tgt.FeatureRef,
			BaseRef:    tgt.BaseRef,
			BaseCommit: gitcli.ObjectID("0123456789abcdef0123456789abcdef01234567"),
			Path:       intended,
			Phase:      phase,
		}
	}
	oldBase := gitcli.RefName("refs/heads/feat/parent")
	if oldBase == tgt.BaseRef {
		t.Fatalf("fixture: old base %q must differ from the target base", oldBase)
	}

	cases := []struct {
		name   string
		mutate func(*Manifest)
		want   bool
	}{
		{"ready-matching-base", func(m *Manifest) {}, true},
		{"ready-other-base", func(m *Manifest) { m.BaseRef = oldBase }, true},
		{"cleaned-other-base", func(m *Manifest) { m.Phase = PhaseCleaned; m.BaseRef = oldBase }, true},
		{"allocating-matching-base", func(m *Manifest) { m.Phase = PhaseAllocating }, true},
		{"allocating-other-base", func(m *Manifest) { m.Phase = PhaseAllocating; m.BaseRef = oldBase }, false},
		{"ready-other-change-id", func(m *Manifest) { m.ChangeID = 8 }, false},
		{"ready-other-slug", func(m *Manifest) { m.Slug = "other-slug" }, false},
		{"ready-other-feature-ref", func(m *Manifest) { m.FeatureRef = "refs/heads/feat/other" }, false},
		{"ready-other-id", func(m *Manifest) { m.ID = workspaceID("refs/heads/feat/other") }, false},
		{"ready-other-path", func(m *Manifest) { m.Path = filepath.Join(repo.PrimaryWorktree, ".worktrees", "other-slug") }, false},
		{"ready-other-common-dir", func(m *Manifest) { m.CommonDir = "/elsewhere/.git" }, false},
		{"cleaned-other-base-other-change-id", func(m *Manifest) { m.Phase = PhaseCleaned; m.BaseRef = oldBase; m.ChangeID = 8 }, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := owned(PhaseReady)
			tc.mutate(&m)
			if got := ownsManifest(m, repo, tgt, intended); got != tc.want {
				t.Errorf("ownsManifest = %v; want %v", got, tc.want)
			}
		})
	}
}
