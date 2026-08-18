package install

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// digestOf is the test's own hash, deliberately independent of the
// implementation's helper: a record hash is an ownership proof, so the bytes it
// covers are pinned here rather than borrowed from the code under test.
func digestOf(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func writeFileOrDie(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

func symlinkOrDie(t *testing.T, target, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", filepath.Dir(path), err)
	}
	if err := os.Symlink(target, path); err != nil {
		t.Fatalf("Symlink(%s -> %s): %v", path, target, err)
	}
}

func priorWith(records ...TargetRecord) *State {
	return &State{
		FormatVersion:  StateFormatVersion,
		ProductVersion: "0.1.0-dev",
		AssetProtocol:  1,
		AssetSetID:     "sha256:prior",
		Mode:           ModeRelease,
		Harnesses:      []string{"claude"},
		Targets:        records,
	}
}

// managedFile is a small authored file carrying a docket managed block, plus
// bytes before and after it that no install may ever disturb.
func managedFile(interior string) string {
	return "# Notes\n\nuser prose above\n\n" +
		"<!-- docket:dispatch:start (managed by docket) -->\n" +
		interior +
		"<!-- docket:dispatch:end -->\n\nuser prose below\n"
}

func TestInspectTarget(t *testing.T) {
	cases := []struct {
		name       string
		setup      func(t *testing.T, dir string) (Target, *State)
		want       Disposition
		wantReason string
		// wantRemedy are substrings the conflict's remedy must contain. Every
		// conflict is asserted to carry SOME remedy by the loop below; these
		// pin the target-specific half — the block name, the expected link
		// destination, the fact that a recorded file drifted.
		wantRemedy []string
	}{
		{
			name: "absent file is created",
			setup: func(t *testing.T, dir string) (Target, *State) {
				return Target{
					Path:    filepath.Join(dir, "agents", "docket-adr.md"),
					Kind:    KindFile,
					Content: []byte("agent\n"),
					Role:    "agent",
				}, nil
			},
			want: DispositionCreate,
		},
		{
			name: "identical file is a no-op",
			setup: func(t *testing.T, dir string) (Target, *State) {
				p := filepath.Join(dir, "agents", "docket-adr.md")
				writeFileOrDie(t, p, "agent\n")
				return Target{Path: p, Kind: KindFile, Content: []byte("agent\n"), Role: "agent"},
					priorWith(TargetRecord{Path: p, Kind: KindFile, SHA256: digestOf("agent\n"), Role: "agent"})
			},
			want: DispositionNoop,
		},
		{
			name: "owned file the plan changes is updated",
			setup: func(t *testing.T, dir string) (Target, *State) {
				p := filepath.Join(dir, "agents", "docket-adr.md")
				writeFileOrDie(t, p, "old\n")
				return Target{Path: p, Kind: KindFile, Content: []byte("new\n"), Role: "agent"},
					priorWith(TargetRecord{Path: p, Kind: KindFile, SHA256: digestOf("old\n"), Role: "agent"})
			},
			want: DispositionUpdate,
		},
		{
			name: "unknown existing file is a conflict",
			setup: func(t *testing.T, dir string) (Target, *State) {
				p := filepath.Join(dir, "agents", "docket-adr.md")
				writeFileOrDie(t, p, "hand written by the user\n")
				return Target{Path: p, Kind: KindFile, Content: []byte("new\n"), Role: "agent"}, nil
			},
			want:       DispositionConflict,
			wantReason: ReasonOwnershipConflict,
			wantRemedy: []string{"docket did not write", "move or delete", "re-run"},
		},
		{
			name: "drifted owned file is a conflict",
			setup: func(t *testing.T, dir string) (Target, *State) {
				p := filepath.Join(dir, "agents", "docket-adr.md")
				writeFileOrDie(t, p, "old\nplus a user edit\n")
				return Target{Path: p, Kind: KindFile, Content: []byte("new\n"), Role: "agent"},
					priorWith(TargetRecord{Path: p, Kind: KindFile, SHA256: digestOf("old\n"), Role: "agent"})
			},
			want:       DispositionConflict,
			wantReason: ReasonOwnershipConflict,
			wantRemedy: []string{"no longer matches the recorded install", "re-run"},
		},
		{
			name: "a directory where a file target belongs is a conflict",
			setup: func(t *testing.T, dir string) (Target, *State) {
				p := filepath.Join(dir, "agents", "docket-adr.md")
				if err := os.MkdirAll(p, 0o755); err != nil {
					t.Fatalf("MkdirAll: %v", err)
				}
				return Target{Path: p, Kind: KindFile, Content: []byte("new\n"), Role: "agent"}, nil
			},
			want:       DispositionConflict,
			wantReason: ReasonOwnershipConflict,
			wantRemedy: []string{"move or delete", "re-run"},
		},
		{
			// A symlink standing where a plain file target belongs is the
			// escape case: writing "the file" would follow the link and
			// rewrite somebody else's bytes outside every docket root. The
			// foreign file's bytes are asserted intact by the table's own
			// before/after tree comparison.
			name: "a symlink where a plain file belongs is a conflict",
			setup: func(t *testing.T, dir string) (Target, *State) {
				outside := filepath.Join(dir, "elsewhere", "someone-elses.md")
				writeFileOrDie(t, outside, "not docket's file\n")
				p := filepath.Join(dir, "agents", "docket-adr.md")
				symlinkOrDie(t, outside, p)
				return Target{Path: p, Kind: KindFile, Content: []byte("new\n"), Role: "agent"}, nil
			},
			want:       DispositionConflict,
			wantReason: ReasonOwnershipConflict,
			wantRemedy: []string{"move or delete", "re-run"},
		},
		{
			// The same escape spelled through a symlinked ANCESTOR: the
			// target's own basename does not exist, so only canonicalising
			// the parent hops reveals that the write lands outside.
			name: "a file whose parent directory is a symlink out of the root is inspected at the resolved path",
			setup: func(t *testing.T, dir string) (Target, *State) {
				outside := filepath.Join(dir, "elsewhere", "agents")
				writeFileOrDie(t, filepath.Join(outside, "docket-adr.md"), "not docket's file\n")
				symlinkOrDie(t, outside, filepath.Join(dir, "agents"))
				return Target{
					Path:    filepath.Join(dir, "agents", "docket-adr.md"),
					Kind:    KindFile,
					Content: []byte("new\n"),
					Role:    "agent",
				}, nil
			},
			want:       DispositionConflict,
			wantReason: ReasonOwnershipConflict,
			wantRemedy: []string{"move or delete", "re-run"},
		},
		{
			name: "absent symlink is created",
			setup: func(t *testing.T, dir string) (Target, *State) {
				return Target{
					Path:       filepath.Join(dir, "skills", "docket-build"),
					Kind:       KindSymlink,
					LinkTarget: filepath.Join(dir, "versions", "one", "assets", "skills", "docket-build"),
					Role:       "skill",
				}, nil
			},
			want: DispositionCreate,
		},
		{
			// The link is spelled through a symlinked ancestor, so only a
			// per-hop canonicalisation of BOTH sides can see it is already
			// exactly what the plan wants.
			name: "symlink reaching the desired target through an extra hop is a no-op",
			setup: func(t *testing.T, dir string) (Target, *State) {
				real := filepath.Join(dir, "real", "skills", "docket-build")
				if err := os.MkdirAll(real, 0o755); err != nil {
					t.Fatalf("MkdirAll: %v", err)
				}
				symlinkOrDie(t, filepath.Join(dir, "real"), filepath.Join(dir, "alias"))
				link := filepath.Join(dir, "skills", "docket-build")
				symlinkOrDie(t, filepath.Join(dir, "alias", "skills", "docket-build"), link)
				return Target{Path: link, Kind: KindSymlink, LinkTarget: real, Role: "skill"}, nil
			},
			want: DispositionNoop,
		},
		{
			name: "symlink at a different canonical target is a conflict",
			setup: func(t *testing.T, dir string) (Target, *State) {
				other := filepath.Join(dir, "elsewhere")
				if err := os.MkdirAll(other, 0o755); err != nil {
					t.Fatalf("MkdirAll: %v", err)
				}
				link := filepath.Join(dir, "skills", "docket-build")
				symlinkOrDie(t, other, link)
				return Target{
					Path:       link,
					Kind:       KindSymlink,
					LinkTarget: filepath.Join(dir, "versions", "one", "assets", "skills", "docket-build"),
					Role:       "skill",
				}, nil
			},
			want:       DispositionConflict,
			wantReason: ReasonOwnershipConflict,
			wantRemedy: []string{
				filepath.Join("versions", "one", "assets", "skills", "docket-build"),
				"re-run",
			},
		},
		{
			name: "owned symlink the plan repoints is updated",
			setup: func(t *testing.T, dir string) (Target, *State) {
				old := filepath.Join(dir, "versions", "one", "assets", "skills", "docket-build")
				if err := os.MkdirAll(old, 0o755); err != nil {
					t.Fatalf("MkdirAll: %v", err)
				}
				link := filepath.Join(dir, "skills", "docket-build")
				symlinkOrDie(t, old, link)
				return Target{
						Path:       link,
						Kind:       KindSymlink,
						LinkTarget: filepath.Join(dir, "versions", "two", "assets", "skills", "docket-build"),
						Role:       "skill",
					},
					priorWith(TargetRecord{Path: link, Kind: KindSymlink, LinkTarget: old, Role: "skill"})
			},
			want: DispositionUpdate,
		},
		{
			name: "unknown file where a symlink belongs is a conflict",
			setup: func(t *testing.T, dir string) (Target, *State) {
				link := filepath.Join(dir, "skills", "docket-build")
				writeFileOrDie(t, link, "not ours\n")
				return Target{
					Path:       link,
					Kind:       KindSymlink,
					LinkTarget: filepath.Join(dir, "versions", "one", "assets", "skills", "docket-build"),
					Role:       "skill",
				}, nil
			},
			want:       DispositionConflict,
			wantReason: ReasonOwnershipConflict,
			wantRemedy: []string{"move or delete", "re-run"},
		},
		{
			name: "absent managed-block file is created",
			setup: func(t *testing.T, dir string) (Target, *State) {
				return Target{
					Path:      filepath.Join(dir, "CLAUDE.md"),
					Kind:      KindManagedBlock,
					BlockName: "dispatch",
					Content:   []byte("dispatch body\n"),
					Role:      "dispatch",
				}, nil
			},
			want: DispositionCreate,
		},
		{
			name: "managed block already carrying the desired interior is a no-op",
			setup: func(t *testing.T, dir string) (Target, *State) {
				p := filepath.Join(dir, "CLAUDE.md")
				writeFileOrDie(t, p, managedFile("dispatch body\n"))
				return Target{
						Path:      p,
						Kind:      KindManagedBlock,
						BlockName: "dispatch",
						Content:   []byte("dispatch body\n"),
						Role:      "dispatch",
					},
					priorWith(TargetRecord{
						Path: p, Kind: KindManagedBlock, BlockName: "dispatch",
						SHA256: digestOf("dispatch body"), Role: "dispatch",
					})
			},
			want: DispositionNoop,
		},
		{
			// A rewrite keeps the block's own line ending, so a CRLF file
			// already holding the desired lines needs no write at all.
			name: "managed block differing only in line endings is a no-op",
			setup: func(t *testing.T, dir string) (Target, *State) {
				p := filepath.Join(dir, "CLAUDE.md")
				writeFileOrDie(t, p, strings.ReplaceAll(managedFile("dispatch body\n"), "\n", "\r\n"))
				return Target{
					Path:      p,
					Kind:      KindManagedBlock,
					BlockName: "dispatch",
					Content:   []byte("dispatch body\n"),
					Role:      "dispatch",
				}, nil
			},
			want: DispositionNoop,
		},
		{
			name: "owned managed block the plan changes is updated",
			setup: func(t *testing.T, dir string) (Target, *State) {
				p := filepath.Join(dir, "CLAUDE.md")
				writeFileOrDie(t, p, managedFile("old dispatch body\n"))
				return Target{
						Path:      p,
						Kind:      KindManagedBlock,
						BlockName: "dispatch",
						Content:   []byte("new dispatch body\n"),
						Role:      "dispatch",
					},
					priorWith(TargetRecord{
						Path: p, Kind: KindManagedBlock, BlockName: "dispatch",
						SHA256: digestOf("old dispatch body"), Role: "dispatch",
					})
			},
			want: DispositionUpdate,
		},
		{
			name: "hand-edited managed block is a conflict",
			setup: func(t *testing.T, dir string) (Target, *State) {
				p := filepath.Join(dir, "CLAUDE.md")
				writeFileOrDie(t, p, managedFile("old dispatch body\nand a user edit\n"))
				return Target{
						Path:      p,
						Kind:      KindManagedBlock,
						BlockName: "dispatch",
						Content:   []byte("new dispatch body\n"),
						Role:      "dispatch",
					},
					priorWith(TargetRecord{
						Path: p, Kind: KindManagedBlock, BlockName: "dispatch",
						SHA256: digestOf("old dispatch body"), Role: "dispatch",
					})
			},
			want:       DispositionConflict,
			wantReason: ReasonOwnershipConflict,
			wantRemedy: []string{"docket:dispatch", "no longer matches the recorded install", "re-run"},
		},
		{
			name: "foreign block of the same name without a prior record is a conflict",
			setup: func(t *testing.T, dir string) (Target, *State) {
				p := filepath.Join(dir, "CLAUDE.md")
				writeFileOrDie(t, p, managedFile("someone else's body\n"))
				return Target{
					Path:      p,
					Kind:      KindManagedBlock,
					BlockName: "dispatch",
					Content:   []byte("new dispatch body\n"),
					Role:      "dispatch",
				}, nil
			},
			want:       DispositionConflict,
			wantReason: ReasonOwnershipConflict,
			wantRemedy: []string{"docket:dispatch", "docket did not write", "re-run"},
		},
		{
			name: "block absent from an existing file is an update that appends",
			setup: func(t *testing.T, dir string) (Target, *State) {
				p := filepath.Join(dir, "CLAUDE.md")
				writeFileOrDie(t, p, "# Notes\n\nuser prose only\n")
				return Target{
					Path:      p,
					Kind:      KindManagedBlock,
					BlockName: "dispatch",
					Content:   []byte("dispatch body\n"),
					Role:      "dispatch",
				}, nil
			},
			want: DispositionUpdate,
		},
		{
			name: "dangling start marker is managed-block-invalid",
			setup: func(t *testing.T, dir string) (Target, *State) {
				p := filepath.Join(dir, "CLAUDE.md")
				writeFileOrDie(t, p, "# Notes\n\n<!-- docket:dispatch:start -->\nbody\n")
				return Target{
						Path:      p,
						Kind:      KindManagedBlock,
						BlockName: "dispatch",
						Content:   []byte("dispatch body\n"),
						Role:      "dispatch",
					},
					priorWith(TargetRecord{
						Path: p, Kind: KindManagedBlock, BlockName: "dispatch",
						SHA256: digestOf("body"), Role: "dispatch",
					})
			},
			want:       DispositionConflict,
			wantReason: ReasonManagedBlockInvalid,
			wantRemedy: []string{"docket:dispatch", "by hand", "re-run"},
		},
		{
			name: "malformed marker line is managed-block-invalid",
			setup: func(t *testing.T, dir string) (Target, *State) {
				p := filepath.Join(dir, "CLAUDE.md")
				writeFileOrDie(t, p, "# Notes\n\n<!-- docket:dispatch:begin -->\nbody\n")
				return Target{
					Path:      p,
					Kind:      KindManagedBlock,
					BlockName: "dispatch",
					Content:   []byte("dispatch body\n"),
					Role:      "dispatch",
				}, nil
			},
			want:       DispositionConflict,
			wantReason: ReasonManagedBlockInvalid,
			wantRemedy: []string{"docket:dispatch", "by hand", "re-run"},
		},
		{
			name: "a symlink where a managed-block file belongs is a conflict",
			setup: func(t *testing.T, dir string) (Target, *State) {
				real := filepath.Join(dir, "elsewhere", "CLAUDE.md")
				writeFileOrDie(t, real, managedFile("dispatch body\n"))
				p := filepath.Join(dir, "CLAUDE.md")
				symlinkOrDie(t, real, p)
				return Target{
					Path:      p,
					Kind:      KindManagedBlock,
					BlockName: "dispatch",
					Content:   []byte("new dispatch body\n"),
					Role:      "dispatch",
				}, nil
			},
			want:       DispositionConflict,
			wantReason: ReasonOwnershipConflict,
			wantRemedy: []string{"move or delete", "re-run"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			target, prior := tc.setup(t, dir)

			before := snapshotTree(t, dir)
			got, err := InspectTarget(target, prior, nil)
			if err != nil {
				t.Fatalf("InspectTarget: %v", err)
			}
			if got.Disposition != tc.want {
				t.Errorf("disposition = %q, want %q", got.Disposition, tc.want)
			}
			if got.Reason != tc.wantReason {
				t.Errorf("reason = %q, want %q", got.Reason, tc.wantReason)
			}
			// There is no --force, so a conflict without a remedy is a dead
			// end for the user: every conflict states a way forward, and no
			// other disposition invents one.
			if tc.want == DispositionConflict {
				if got.Remedy == "" {
					t.Errorf("conflict carries no remedy")
				}
				for _, want := range tc.wantRemedy {
					if !strings.Contains(got.Remedy, want) {
						t.Errorf("remedy = %q, want it to contain %q", got.Remedy, want)
					}
				}
				if detail := got.ConflictDetail(); detail != got.Reason+": "+got.Remedy {
					t.Errorf("conflict detail = %q, want %q", detail, got.Reason+": "+got.Remedy)
				}
			} else if got.Remedy != "" {
				t.Errorf("%s carries a remedy %q", got.Disposition, got.Remedy)
			}
			if got.Target.Path != target.Path {
				t.Errorf("inspection dropped its target: %q", got.Target.Path)
			}
			// Inspection is pure: it never repairs, creates, or rewrites.
			if after := snapshotTree(t, dir); after != before {
				t.Errorf("inspection mutated the tree:\nbefore:\n%s\nafter:\n%s", before, after)
			}
		})
	}
}

// snapshotTree renders every path under root with its bytes or link target, so
// any write a read-only inspection performs is visible.
func snapshotTree(t *testing.T, root string) string {
	t.Helper()
	var b strings.Builder
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			dest, readErr := os.Readlink(path)
			if readErr != nil {
				return readErr
			}
			b.WriteString(rel + " -> " + dest + "\n")
		case info.IsDir():
			b.WriteString(rel + "/\n")
		default:
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			b.WriteString(rel + " = " + string(data) + "\n")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	return b.String()
}

func TestInspectTargetRejectsInvalidTargets(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name   string
		target Target
	}{
		{"empty path", Target{Kind: KindFile}},
		{"relative path", Target{Path: "agents/docket-adr.md", Kind: KindFile}},
		{"unknown kind", Target{Path: filepath.Join(dir, "x"), Kind: TargetKind("directory")}},
		{"symlink without a link target", Target{Path: filepath.Join(dir, "x"), Kind: KindSymlink}},
		{"symlink with a relative link target", Target{Path: filepath.Join(dir, "x"), Kind: KindSymlink, LinkTarget: "../assets"}},
		{"managed block without a name", Target{Path: filepath.Join(dir, "x"), Kind: KindManagedBlock}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := InspectTarget(tc.target, nil, nil)
			if err == nil {
				t.Fatal("want an error, got nil")
			}
			if !errors.Is(err, ErrInvalidTarget) {
				t.Errorf("error %v does not wrap ErrInvalidTarget", err)
			}
			if _, err := RecordFor(tc.target); !errors.Is(err, ErrInvalidTarget) {
				t.Errorf("RecordFor error %v does not wrap ErrInvalidTarget", err)
			}
		})
	}
}

// The legacy seam is the third ownership proof: bytes docket's own frozen
// renderer would have produced are provably docket's, even with no record.
func TestLegacySeamReproducible(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "agents", "docket-adr.md")
	writeFileOrDie(t, p, "legacy wrapper bytes\n")

	target := Target{Path: p, Kind: KindFile, Content: []byte("new\n"), Role: "agent"}
	legacy := func(tt Target) ([]byte, bool) {
		if tt.Path != p {
			return nil, false
		}
		return []byte("legacy wrapper bytes\n"), true
	}

	got, err := InspectTarget(target, nil, legacy)
	if err != nil {
		t.Fatalf("InspectTarget: %v", err)
	}
	if got.Disposition != DispositionUpdate {
		t.Errorf("disposition = %q, want %q", got.Disposition, DispositionUpdate)
	}
	if got.Reason != "" {
		t.Errorf("reason = %q, want empty", got.Reason)
	}

	// Without the seam the very same file is unprovable and preserved.
	bare, err := InspectTarget(target, nil, nil)
	if err != nil {
		t.Fatalf("InspectTarget(no seam): %v", err)
	}
	if bare.Disposition != DispositionConflict {
		t.Errorf("without the seam: disposition = %q, want %q", bare.Disposition, DispositionConflict)
	}
}

func TestLegacySeamNonReproducible(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "agents", "docket-adr.md")
	writeFileOrDie(t, p, "legacy wrapper bytes with a user edit\n")

	target := Target{Path: p, Kind: KindFile, Content: []byte("new\n"), Role: "agent"}
	cases := map[string]LegacyReproducer{
		"bytes differ":   func(Target) ([]byte, bool) { return []byte("legacy wrapper bytes\n"), true },
		"not reproduced": func(Target) ([]byte, bool) { return nil, false },
	}
	for name, legacy := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := InspectTarget(target, nil, legacy)
			if err != nil {
				t.Fatalf("InspectTarget: %v", err)
			}
			if got.Disposition != DispositionConflict {
				t.Errorf("disposition = %q, want %q", got.Disposition, DispositionConflict)
			}
			if got.Reason != ReasonOwnershipConflict {
				t.Errorf("reason = %q, want %q", got.Reason, ReasonOwnershipConflict)
			}
		})
	}
}

// TestInspectManagedBlockLegacyAdoption is the managed-block half of ownership
// proof three: a docket:dispatch block whose interior is byte-exact to what the
// frozen legacy renderer would have produced is adopted (DispositionUpdate) even
// with no prior record, instead of being reported as a foreign-block conflict.
// The stub reproducer stands in for the real one, so the adoption seam is proven
// here without pinning a corpus.
func TestInspectManagedBlockLegacyAdoption(t *testing.T) {
	const interior = "legacy dispatch interior\n"
	// The stub reproduces the frozen interior for the dispatch block and nothing
	// else — matching the real reproducer's KindManagedBlock/BlockName gate.
	legacyStub := func(tt Target) ([]byte, bool) {
		if tt.Kind == KindManagedBlock && tt.BlockName == "dispatch" {
			return []byte(interior), true
		}
		return nil, false
	}
	dispatchTarget := func(p string) Target {
		return Target{
			Path:      p,
			Kind:      KindManagedBlock,
			BlockName: "dispatch",
			Content:   []byte("new dispatch body\n"),
			Role:      "dispatch",
		}
	}

	t.Run("exact legacy block with no prior record is adopted", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "CLAUDE.md")
		writeFileOrDie(t, p, managedFile(interior))
		got, err := InspectTarget(dispatchTarget(p), nil, legacyStub)
		if err != nil {
			t.Fatalf("InspectTarget: %v", err)
		}
		if got.Disposition != DispositionUpdate {
			t.Errorf("disposition = %q, want %q", got.Disposition, DispositionUpdate)
		}
		if got.Reason != "" {
			t.Errorf("reason = %q, want empty", got.Reason)
		}
	})

	t.Run("legacy block with one byte changed is a foreign-block conflict", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "CLAUDE.md")
		writeFileOrDie(t, p, managedFile("legacy dispatch interiorX\n"))
		got, err := InspectTarget(dispatchTarget(p), nil, legacyStub)
		if err != nil {
			t.Fatalf("InspectTarget: %v", err)
		}
		if got.Disposition != DispositionConflict {
			t.Errorf("disposition = %q, want %q", got.Disposition, DispositionConflict)
		}
		if got.Reason != ReasonOwnershipConflict {
			t.Errorf("reason = %q, want %q", got.Reason, ReasonOwnershipConflict)
		}
	})

	t.Run("malformed markers short-circuit before the legacy check", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "CLAUDE.md")
		// A dangling start marker whose body IS the exact legacy interior: were
		// the legacy check to run before marker validation, this would be wrongly
		// adopted. Marker validity is a precondition, so it must stay invalid.
		writeFileOrDie(t, p, "# Notes\n\n<!-- docket:dispatch:start (managed by docket) -->\n"+interior)
		got, err := InspectTarget(dispatchTarget(p), nil, legacyStub)
		if err != nil {
			t.Fatalf("InspectTarget: %v", err)
		}
		if got.Disposition != DispositionConflict {
			t.Errorf("disposition = %q, want %q", got.Disposition, DispositionConflict)
		}
		if got.Reason != ReasonManagedBlockInvalid {
			t.Errorf("reason = %q, want %q", got.Reason, ReasonManagedBlockInvalid)
		}
	})

	t.Run("nil legacy preserves the foreign-block conflict", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "CLAUDE.md")
		writeFileOrDie(t, p, managedFile(interior))
		got, err := InspectTarget(dispatchTarget(p), nil, nil)
		if err != nil {
			t.Fatalf("InspectTarget: %v", err)
		}
		if got.Disposition != DispositionConflict {
			t.Errorf("disposition = %q, want %q", got.Disposition, DispositionConflict)
		}
		if got.Reason != ReasonOwnershipConflict {
			t.Errorf("reason = %q, want %q", got.Reason, ReasonOwnershipConflict)
		}
	})
}

// RecordFor is the other half of every ownership proof: what the installer
// publishes after applying a target must be exactly what a later inspection
// accepts as proof it owns that target.
func TestRecordForRoundTrip(t *testing.T) {
	dir := t.TempDir()

	fileTarget := Target{
		Path:    filepath.Join(dir, "agents", "docket-adr.md"),
		Kind:    KindFile,
		Content: []byte("agent\n"),
		Role:    "agent",
	}
	writeFileOrDie(t, fileTarget.Path, string(fileTarget.Content))

	linkDest := filepath.Join(dir, "real", "skills", "docket-build")
	if err := os.MkdirAll(linkDest, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	symlinkOrDie(t, filepath.Join(dir, "real"), filepath.Join(dir, "alias"))
	linkTarget := Target{
		Path:       filepath.Join(dir, "skills", "docket-build"),
		Kind:       KindSymlink,
		LinkTarget: filepath.Join(dir, "alias", "skills", "docket-build"),
		Role:       "skill",
	}
	symlinkOrDie(t, linkTarget.LinkTarget, linkTarget.Path)

	blockTarget := Target{
		Path:      filepath.Join(dir, "CLAUDE.md"),
		Kind:      KindManagedBlock,
		BlockName: "dispatch",
		Content:   []byte("dispatch body\n"),
		Role:      "dispatch",
	}
	writeFileOrDie(t, blockTarget.Path, managedFile(string(blockTarget.Content)))

	var records []TargetRecord
	for _, target := range []Target{fileTarget, linkTarget, blockTarget} {
		rec, err := RecordFor(target)
		if err != nil {
			t.Fatalf("RecordFor(%s): %v", target.Path, err)
		}
		records = append(records, rec)
	}
	prior := priorWith(records...)

	// The record proves ownership even when the next plan wants other bytes.
	next := []Target{
		{Path: fileTarget.Path, Kind: KindFile, Content: []byte("agent v2\n"), Role: "agent"},
		{Path: linkTarget.Path, Kind: KindSymlink, LinkTarget: filepath.Join(dir, "real", "skills", "docket-build-v2"), Role: "skill"},
		{Path: blockTarget.Path, Kind: KindManagedBlock, BlockName: "dispatch", Content: []byte("dispatch v2\n"), Role: "dispatch"},
	}
	for _, target := range next {
		got, err := InspectTarget(target, prior, nil)
		if err != nil {
			t.Fatalf("InspectTarget(%s): %v", target.Path, err)
		}
		if got.Disposition != DispositionUpdate {
			t.Errorf("%s: disposition = %q, want %q (record did not prove ownership)",
				target.Path, got.Disposition, DispositionUpdate)
		}
	}

	// A symlink record stores the canonical destination, not the spelling the
	// plan happened to use.
	if want := linkDest; records[1].LinkTarget == linkTarget.LinkTarget {
		t.Errorf("symlink record kept the uncanonical spelling %q (want the canonical %q)",
			records[1].LinkTarget, want)
	}
}

func TestPruneCandidates(t *testing.T) {
	dir := t.TempDir()

	kept := filepath.Join(dir, "agents", "kept.md")
	writeFileOrDie(t, kept, "kept\n")
	removable := filepath.Join(dir, "agents", "removable.md")
	writeFileOrDie(t, removable, "removable\n")
	drifted := filepath.Join(dir, "agents", "drifted.md")
	writeFileOrDie(t, drifted, "drifted by the user\n")
	goneLink := filepath.Join(dir, "skills", "gone")
	block := filepath.Join(dir, "CLAUDE.md")
	writeFileOrDie(t, block, managedFile("dispatch body\n"))

	prior := priorWith(
		TargetRecord{Path: kept, Kind: KindFile, SHA256: digestOf("kept\n"), Role: "agent"},
		TargetRecord{Path: removable, Kind: KindFile, SHA256: digestOf("removable\n"), Role: "agent"},
		TargetRecord{Path: drifted, Kind: KindFile, SHA256: digestOf("original\n"), Role: "agent"},
		TargetRecord{Path: goneLink, Kind: KindSymlink, LinkTarget: filepath.Join(dir, "versions", "one"), Role: "skill"},
		TargetRecord{Path: block, Kind: KindManagedBlock, BlockName: "dispatch", SHA256: digestOf("dispatch body"), Role: "dispatch"},
	)
	plan := []Target{{Path: kept, Kind: KindFile, Content: []byte("kept\n"), Role: "agent"}}

	got, err := PruneCandidates(prior, plan)
	if err != nil {
		t.Fatalf("PruneCandidates: %v", err)
	}

	want := map[string]bool{
		removable: true,  // identity still equals the record
		drifted:   false, // user bytes: preserved, blocks the upgrade
		goneLink:  true,  // already absent: nothing to preserve
		block:     true,  // owned block still carries the recorded interior
	}
	if len(got) != len(want) {
		t.Fatalf("PruneCandidates returned %d candidates, want %d: %+v", len(got), len(want), got)
	}
	for _, p := range got {
		w, ok := want[p.Record.Path]
		if !ok {
			t.Errorf("unexpected prune candidate %q", p.Record.Path)
			continue
		}
		if p.Removable != w {
			t.Errorf("%s: removable = %v, want %v", p.Record.Path, p.Removable, w)
		}
	}
	for _, p := range got {
		if p.Record.Path == kept {
			t.Errorf("a target still in the plan must not be a prune candidate")
		}
	}
}

func TestPruneCandidatesNoPriorState(t *testing.T) {
	got, err := PruneCandidates(nil, []Target{{Path: "/tmp/x", Kind: KindFile}})
	if err != nil {
		t.Fatalf("PruneCandidates(nil): %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("PruneCandidates(nil) = %+v, want none", got)
	}
}
