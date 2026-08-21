package release

import (
	"os"
	"strings"
	"testing"
)

func TestValidateVersionAccepts(t *testing.T) {
	accepted := []string{
		"v1.0.0",
		"v0.2.10-rc.1",
		"v1.0.0-candidate.base",
	}
	for _, v := range accepted {
		if err := ValidateVersion(v); err != nil {
			t.Errorf("ValidateVersion(%q) = %v, want nil", v, err)
		}
	}
}

func TestValidateVersionRejects(t *testing.T) {
	rejected := []string{
		"1.0.0",      // no leading v
		"v1.0",       // only two components
		"v01.0.0",    // leading zero
		"v1.0.0-",    // empty prerelease after dash
		"v1.0.0-a/b", // path separator in prerelease
		"v1.0.0-a b", // space in prerelease
		"v1.0.0-.rc", // prerelease may not lead with a separator
		"v1.0.0--rc", // prerelease may not lead with a separator
		"",           // empty
		" v1.0.0",    // leading space
		"v1.0.0 ",    // trailing space
	}
	for _, v := range rejected {
		if err := ValidateVersion(v); err == nil {
			t.Errorf("ValidateVersion(%q) = nil, want error", v)
		}
	}
}

func TestTuplesOrder(t *testing.T) {
	got := Tuples()
	want := []Tuple{
		{OS: "darwin", Arch: "amd64"},
		{OS: "darwin", Arch: "arm64"},
		{OS: "linux", Arch: "amd64"},
		{OS: "linux", Arch: "arm64"},
	}
	if len(got) != len(want) {
		t.Fatalf("Tuples() length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Tuples()[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestArchiveNameAllTuples(t *testing.T) {
	want := map[Tuple]string{
		{OS: "darwin", Arch: "amd64"}: "docket_v1.2.3_darwin_amd64.tar.gz",
		{OS: "darwin", Arch: "arm64"}: "docket_v1.2.3_darwin_arm64.tar.gz",
		{OS: "linux", Arch: "amd64"}:  "docket_v1.2.3_linux_amd64.tar.gz",
		{OS: "linux", Arch: "arm64"}:  "docket_v1.2.3_linux_arm64.tar.gz",
	}
	for _, tup := range Tuples() {
		got := ArchiveName("v1.2.3", tup)
		if got != want[tup] {
			t.Errorf("ArchiveName(v1.2.3, %+v) = %q, want %q", tup, got, want[tup])
		}
	}
}

// validInputs returns an Inputs value every field of which passes Validate,
// using the calling test's own temp dirs for the two path fields.
func validInputs(t *testing.T) Inputs {
	t.Helper()
	return Inputs{
		SourceRoot:  t.TempDir(),
		Version:     "v1.2.3",
		Commit:      strings.Repeat("ab", 20), // 40 lowercase hex
		SourceEpoch: 1700000000,
		OutDir:      t.TempDir(),
	}
}

func TestInputsValidateAccepts(t *testing.T) {
	if err := validInputs(t).Validate(); err != nil {
		t.Fatalf("Validate() on good inputs = %v, want nil", err)
	}
}

func TestInputsValidateRejects(t *testing.T) {
	missing := "/no/such/source/root/should/not/exist/xyzzy"

	cases := []struct {
		name   string
		mutate func(*Inputs)
	}{
		{"bad version", func(in *Inputs) { in.Version = "1.0.0" }},
		{"short commit", func(in *Inputs) { in.Commit = strings.Repeat("ab", 19) }},
		{"uppercase commit", func(in *Inputs) { in.Commit = strings.ToUpper(strings.Repeat("ab", 20)) }},
		{"non-hex commit", func(in *Inputs) { in.Commit = strings.Repeat("gz", 20) }},
		{"zero epoch", func(in *Inputs) { in.SourceEpoch = 0 }},
		{"negative epoch", func(in *Inputs) { in.SourceEpoch = -1 }},
		{"relative SourceRoot", func(in *Inputs) { in.SourceRoot = "relative/root" }},
		{"missing SourceRoot", func(in *Inputs) { in.SourceRoot = missing }},
		{"relative OutDir", func(in *Inputs) { in.OutDir = "relative/out" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := validInputs(t)
			tc.mutate(&in)
			if err := in.Validate(); err == nil {
				t.Errorf("Validate() with %s = nil, want error", tc.name)
			}
		})
	}
}

func TestInputsValidateChecksEveryFieldBeforeActing(t *testing.T) {
	// Multiple fields wrong at once: every one must be reported, so a caller
	// cannot fix them one build at a time (learning validate-the-whole-input-set-first).
	in := Inputs{
		SourceRoot:  "relative/root",
		Version:     "1.0.0",
		Commit:      "short",
		SourceEpoch: 0,
		OutDir:      "relative/out",
	}
	err := in.Validate()
	if err == nil {
		t.Fatal("Validate() with all fields wrong = nil, want error")
	}
	msg := err.Error()
	for _, want := range []string{"version", "commit", "epoch", "SourceRoot", "OutDir"} {
		if !strings.Contains(msg, want) {
			t.Errorf("Validate() error %q does not mention %q", msg, want)
		}
	}
}

func TestBuildDateEpochZero(t *testing.T) {
	in := Inputs{SourceEpoch: 0}
	if got := in.BuildDate(); got != "1970-01-01T00:00:00Z" {
		t.Errorf("BuildDate(epoch 0) = %q, want 1970-01-01T00:00:00Z", got)
	}
}

func TestBuildDateIsUTCRFC3339(t *testing.T) {
	in := Inputs{SourceEpoch: 1700000000}
	if got := in.BuildDate(); got != "2023-11-14T22:13:20Z" {
		t.Errorf("BuildDate(1700000000) = %q, want 2023-11-14T22:13:20Z", got)
	}
}

// Sanity: an absolute path that exists but is a file, not a dir, is not a
// valid SourceRoot.
func TestInputsValidateRejectsFileSourceRoot(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "notadir.*")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	in := validInputs(t)
	in.SourceRoot = f.Name()
	if err := in.Validate(); err == nil {
		t.Errorf("Validate() with a file SourceRoot = nil, want error")
	}
}
