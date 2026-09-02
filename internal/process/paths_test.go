package process

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

func TestValidateLaunchRequest(t *testing.T) {
	cwd := testsupport.TempDir(t)
	ok := LaunchRequest{Root: testsupport.TempDir(t), Cwd: cwd, Argv: []string{"/bin/true"}}
	if err := validateLaunchRequest(ok); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
	cases := map[string]LaunchRequest{
		"relative root": {Root: "rel", Cwd: cwd, Argv: []string{"x"}},
		"relative cwd":  {Root: testsupport.TempDir(t), Cwd: "rel", Argv: []string{"x"}},
		"missing cwd":   {Root: testsupport.TempDir(t), Cwd: filepath.Join(cwd, "absent"), Argv: []string{"x"}},
		"empty argv":    {Root: testsupport.TempDir(t), Cwd: cwd, Argv: nil},
		"empty argv0":   {Root: testsupport.TempDir(t), Cwd: cwd, Argv: []string{""}},
	}
	for name, req := range cases {
		err := validateLaunchRequest(req)
		if err == nil {
			t.Errorf("%s: accepted", name)
			continue
		}
		if f, ok := AsFailure(err); !ok || f.Class != FailInvalidInput {
			t.Errorf("%s: class = %v, want invalid-input", name, err)
		}
	}
}

func TestResolveRunDirContainment(t *testing.T) {
	root := testsupport.TempDir(t)
	id := "0123456789abcdef0123456789abcdef"
	real := filepath.Join(root, id)
	if err := os.Mkdir(real, 0o700); err != nil {
		t.Fatal(err)
	}
	got, gotID, err := resolveRunDir(root, real)
	if err != nil || gotID != id || !strings.HasSuffix(got, id) {
		t.Fatalf("valid run dir refused: %v %v %v", got, gotID, err)
	}
	// A symlink at the run slot is refused as blocked.
	link := filepath.Join(root, "fedcba9876543210fedcba9876543210")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if _, _, err := resolveRunDir(root, link); err == nil {
		t.Fatal("symlink run slot accepted")
	} else if f, _ := AsFailure(err); f == nil || f.Class != FailBlocked {
		t.Fatalf("symlink class = %v", err)
	}
	// A directory outside the root is refused even when its NAME looks right.
	outside := filepath.Join(testsupport.TempDir(t), id)
	if err := os.Mkdir(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := resolveRunDir(root, outside); err == nil {
		t.Fatal("escaped run dir accepted")
	}
	// A non-run-id name inside the root is invalid input.
	odd := filepath.Join(root, "not-a-run-id")
	os.Mkdir(odd, 0o700)
	if _, _, err := resolveRunDir(root, odd); err == nil {
		t.Fatal("non-hex run dir accepted")
	}
}

// TestResolveRunDirCanonicalisesRoot — on macOS /tmp is a symlink to
// /private/tmp, so a root spelled through the symlink and a run dir spelled
// physically must still match. TempDir already exercises this on darwin;
// build the two spellings explicitly so the test bites on linux too.
func TestResolveRunDirCanonicalisesRoot(t *testing.T) {
	base := testsupport.TempDir(t)
	physical := filepath.Join(base, "phys")
	os.Mkdir(physical, 0o700)
	alias := filepath.Join(base, "alias")
	if err := os.Symlink(physical, alias); err != nil {
		t.Skip("symlinks unavailable")
	}
	id := "0123456789abcdef0123456789abcdef"
	os.Mkdir(filepath.Join(physical, id), 0o700)
	if _, _, err := resolveRunDir(alias, filepath.Join(physical, id)); err != nil {
		t.Fatalf("aliased root vs physical run dir must canonicalise equal: %v", err)
	}
}

func TestBoundReason(t *testing.T) {
	if got := boundReason("a\t b\nc\x00d"); got != "a b cd" {
		t.Fatalf("flatten: %q", got)
	}
	long := strings.Repeat("é", 300)
	got := boundReason(long)
	if len([]rune(got)) != 200 {
		t.Fatalf("rune bound: %d runes", len([]rune(got)))
	}
}
