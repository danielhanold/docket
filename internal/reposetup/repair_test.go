package reposetup

import (
	"bytes"
	"strings"
	"testing"
)

// planOne runs PlanRepairs and returns the findings; it fails the test on a
// non-nil error, which the roster never produces for a well-formed call.
func planOne(t *testing.T, path string, src string, archived bool) []RepairFinding {
	t.Helper()
	fs, err := PlanRepairs(path, []byte(src), archived)
	if err != nil {
		t.Fatalf("PlanRepairs(%q) unexpected error: %v", path, err)
	}
	return fs
}

// repairableWithCode returns the single repairable finding carrying code, and
// fails if there is not exactly one.
func repairableWithCode(t *testing.T, fs []RepairFinding, code RepairCode) RepairFinding {
	t.Helper()
	var hits []RepairFinding
	for _, f := range fs {
		if f.Repairable && f.Code == code {
			hits = append(hits, f)
		}
	}
	if len(hits) != 1 {
		t.Fatalf("want exactly one repairable finding with code %q, got %d (all: %+v)", code, len(hits), fs)
	}
	return hits[0]
}

// hasRepairableCode reports whether any repairable finding carries code.
func hasRepairableCode(fs []RepairFinding, code RepairCode) bool {
	for _, f := range fs {
		if f.Repairable && f.Code == code {
			return true
		}
	}
	return false
}

func TestRepairQuoteScalarEligible(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "boolean keyword",
			src:  "---\nid: 7\ntitle: yes\n---\nbody\n",
			want: "---\nid: 7\ntitle: 'yes'\n---\nbody\n",
		},
		{
			name: "leading indicator char",
			src:  "---\nid: 7\ntitle: :30\n---\nbody\n",
			want: "---\nid: 7\ntitle: ':30'\n---\nbody\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs := planOne(t, "docs/changes/active/0001-x.md", tc.src, false)
			f := repairableWithCode(t, fs, RepairQuoteScalar)
			if f.Field != "title" {
				t.Fatalf("Field = %q, want title", f.Field)
			}
			out, err := ApplyRepairs([]byte(tc.src), fs)
			if err != nil {
				t.Fatalf("ApplyRepairs: %v", err)
			}
			if string(out) != tc.want {
				t.Fatalf("ApplyRepairs =\n%q\nwant\n%q", out, tc.want)
			}
		})
	}
}

func TestRepairQuoteScalarRefused(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"ambiguous bool decode", "---\nid: 7\ntitle: true\n---\nbody\n"},
		{"multi-line block scalar", "---\nid: 7\ntitle: |\n  a\n  b\n---\nbody\n"},
		{"flow collection never quoted", "---\nid: 7\ntitle: [a, b]\n---\nbody\n"},
		{"already safe scalar", "---\nid: 7\ntitle: hello world\n---\nbody\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs := planOne(t, "docs/changes/active/0001-x.md", tc.src, false)
			if hasRepairableCode(fs, RepairQuoteScalar) {
				t.Fatalf("expected no repairable quote finding, got %+v", fs)
			}
		})
	}
}

func TestRepairScalarToListEligible(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "single scalar",
			src:  "---\nid: 7\ndepends_on: 3\n---\nbody\n",
			want: "---\nid: 7\ndepends_on: [3]\n---\nbody\n",
		},
		{
			name: "comma scalar",
			src:  "---\nid: 7\ndepends_on: 3, 7\n---\nbody\n",
			want: "---\nid: 7\ndepends_on: [3, 7]\n---\nbody\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs := planOne(t, "docs/changes/active/0001-x.md", tc.src, false)
			f := repairableWithCode(t, fs, RepairScalarToList)
			if f.Field != "depends_on" {
				t.Fatalf("Field = %q, want depends_on", f.Field)
			}
			out, err := ApplyRepairs([]byte(tc.src), fs)
			if err != nil {
				t.Fatalf("ApplyRepairs: %v", err)
			}
			if string(out) != tc.want {
				t.Fatalf("ApplyRepairs =\n%q\nwant\n%q", out, tc.want)
			}
		})
	}
}

func TestRepairScalarToListRefused(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"wrong item type", "---\nid: 7\ndepends_on: foo\n---\nbody\n"},
		{"partial sequence", "---\nid: 7\ndepends_on: 3, foo\n---\nbody\n"},
		{"already a list", "---\nid: 7\ndepends_on: [3, 7]\n---\nbody\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs := planOne(t, "docs/changes/active/0001-x.md", tc.src, false)
			if hasRepairableCode(fs, RepairScalarToList) {
				t.Fatalf("expected no repairable list finding, got %+v", fs)
			}
		})
	}
}

func TestRepairDropClaimedAtEligible(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "done record",
			src:  "---\nid: 7\nstatus: done\nclaimed_at: 2026-08-01T10:00:00Z\n---\nbody\n",
			want: "---\nid: 7\nstatus: done\n---\nbody\n",
		},
		{
			name: "killed record",
			src:  "---\nid: 7\nclaimed_at: 2026-08-01T10:00:00Z\nstatus: killed\n---\nbody\n",
			want: "---\nid: 7\nstatus: killed\n---\nbody\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs := planOne(t, "docs/changes/archive/2026-08-01-0007-x.md", tc.src, true)
			f := repairableWithCode(t, fs, RepairDropClaimedAt)
			if f.Field != "claimed_at" {
				t.Fatalf("Field = %q, want claimed_at", f.Field)
			}
			out, err := ApplyRepairs([]byte(tc.src), fs)
			if err != nil {
				t.Fatalf("ApplyRepairs: %v", err)
			}
			if string(out) != tc.want {
				t.Fatalf("ApplyRepairs =\n%q\nwant\n%q", out, tc.want)
			}
		})
	}
}

func TestRepairDropClaimedAtRefused(t *testing.T) {
	cases := []struct {
		name     string
		src      string
		archived bool
	}{
		// Mutation-probe target: the terminal-status check refuses this.
		{"non-terminal archived record", "---\nid: 7\nstatus: in-progress\nclaimed_at: 2026-08-01T10:00:00Z\n---\nbody\n", true},
		// An active (unarchived) record legitimately holds a claim lease.
		{"active unarchived record", "---\nid: 7\nstatus: done\nclaimed_at: 2026-08-01T10:00:00Z\n---\nbody\n", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fs := planOne(t, "docs/changes/x.md", tc.src, tc.archived)
			if hasRepairableCode(fs, RepairDropClaimedAt) {
				t.Fatalf("expected no repairable drop finding, got %+v", fs)
			}
		})
	}
}

func TestRepairUndecodableSingleFinding(t *testing.T) {
	// Duplicate key: document.Parse rejects it, so the whole record is named
	// once as non-repairable rather than partially repaired.
	src := "---\nid: 7\nid: 8\n---\nbody\n"
	fs := planOne(t, "docs/changes/active/0001-x.md", src, false)
	if len(fs) != 1 {
		t.Fatalf("want exactly one finding, got %d: %+v", len(fs), fs)
	}
	if fs[0].Repairable {
		t.Fatalf("undecodable finding must be non-repairable: %+v", fs[0])
	}
	if fs[0].Path != "docs/changes/active/0001-x.md" {
		t.Fatalf("Path = %q", fs[0].Path)
	}
}

func TestApplyRepairsTamperedPatchErrors(t *testing.T) {
	src := "---\nid: 7\ntitle: yes\n---\nbody\n"
	fs := planOne(t, "docs/changes/active/0001-x.md", src, false)
	f := repairableWithCode(t, fs, RepairQuoteScalar)
	// Tamper the preview so it no longer matches the canonical repair.
	tampered := f
	tampered.Patch = bytes.ReplaceAll(f.Patch, []byte("'yes'"), []byte("'no'"))
	if bytes.Equal(tampered.Patch, f.Patch) {
		t.Fatalf("tamper did not change the patch; test is vacuous")
	}
	if _, err := ApplyRepairs([]byte(src), []RepairFinding{tampered}); err == nil {
		t.Fatalf("ApplyRepairs accepted a tampered patch; want error")
	}
}

func TestRepairDigestStableAndSensitive(t *testing.T) {
	srcA := "---\nid: 7\ntitle: yes\n---\nbody\n"
	srcB := "---\nid: 7\ndepends_on: 3\n---\nbody\n"
	a := planOne(t, "a.md", srcA, false)
	b := planOne(t, "b.md", srcB, false)

	// Stable across runs.
	if RepairDigest(a) != RepairDigest(a) {
		t.Fatalf("RepairDigest not stable")
	}
	// Order-sensitive.
	ab := append(append([]RepairFinding(nil), a...), b...)
	ba := append(append([]RepairFinding(nil), b...), a...)
	if RepairDigest(ab) == RepairDigest(ba) {
		t.Fatalf("RepairDigest not order-sensitive")
	}
	// Content-sensitive: a different patch changes the digest.
	mutated := append([]RepairFinding(nil), a...)
	m := mutated[0]
	m.Patch = append(append([]byte(nil), m.Patch...), 'X')
	mutated[0] = m
	if RepairDigest(mutated) == RepairDigest(a) {
		t.Fatalf("RepairDigest not content-sensitive")
	}
	// Non-repairable findings do not contribute to the plan digest.
	withNoise := append([]RepairFinding(nil), a...)
	withNoise = append(withNoise, RepairFinding{Path: "z.md", Repairable: false, Message: "noise"})
	if RepairDigest(withNoise) != RepairDigest(a) {
		t.Fatalf("non-repairable findings must not change the digest")
	}
}

// TestRepairListFieldRosterExcludesBlockedBy pins that blocked_by is NOT treated
// as a list field: the decode layer reads it as a scalar OptionalString, so a
// scalar blocked_by must never be converted to a flow sequence.
func TestRepairListFieldRosterExcludesBlockedBy(t *testing.T) {
	src := "---\nid: 7\nblocked_by: 3\n---\nbody\n"
	fs := planOne(t, "docs/changes/active/0001-x.md", src, false)
	if hasRepairableCode(fs, RepairScalarToList) {
		t.Fatalf("blocked_by must not be list-converted: %+v", fs)
	}
	if !strings.HasPrefix("blocked_by", "blocked") { // keep the anchor name greppable
		t.Fatal("unreachable")
	}
}
