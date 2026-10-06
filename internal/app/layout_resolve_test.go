package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/danielhanold/docket/internal/gitcli"
	"github.com/danielhanold/docket/internal/layout"
	"github.com/danielhanold/docket/internal/testsupport"
)

// fakeURLReader scripts origin's URL read for resolveLayout. calls counts the
// reads so a shared-mode test can prove no URL read happened.
type fakeURLReader struct {
	url   string
	err   error
	calls int
}

func (f *fakeURLReader) RemoteURL(_ context.Context, _ gitcli.Repository, _ gitcli.RemoteName) (string, error) {
	f.calls++
	return f.url, f.err
}

// TestResolveLayoutSharedNeedsNoOriginURL: with no <common>/dckt directory the
// repository is shared, and resolving its layout never reads origin's URL (a
// shared repository must not depend on a readable origin URL).
func TestResolveLayoutSharedNeedsNoOriginURL(t *testing.T) {
	common := testsupport.TempDir(t)
	r := &fakeURLReader{err: errors.New("origin URL must not be read in shared mode")}
	got, err := resolveLayout(context.Background(), r, gitcli.Repository{CommonDir: common, PrimaryWorktree: "/r"})
	if err != nil {
		t.Fatalf("resolveLayout: %v", err)
	}
	if want := layout.SharedLayout(common, "/r"); got != want {
		t.Fatalf("layout = %+v, want %+v", got, want)
	}
	if r.calls != 0 {
		t.Errorf("shared resolution read origin's URL %d times, want 0", r.calls)
	}
	if metadataRemote(got) != "origin" || metadataRef(got) != "refs/heads/docket" {
		t.Errorf("shared metadata remote/ref = %q/%q, want origin/refs/heads/docket", metadataRemote(got), metadataRef(got))
	}
}

// TestResolveLayoutPrivateUsesOriginAndDataHome: a <common>/dckt directory makes
// the repository private; the store is located from origin's URL and the data
// home, and the metadata remote and branch are both dckt.
func TestResolveLayoutPrivateUsesOriginAndDataHome(t *testing.T) {
	common := testsupport.TempDir(t)
	if err := os.Mkdir(filepath.Join(common, layout.PrivateName), 0o755); err != nil {
		t.Fatal(err)
	}
	data := testsupport.TempDir(t)
	t.Setenv("XDG_DATA_HOME", data)
	r := &fakeURLReader{url: "git@github.com:O/R.git"}
	got, err := resolveLayout(context.Background(), r, gitcli.Repository{CommonDir: common, PrimaryWorktree: "/r"})
	if err != nil {
		t.Fatalf("resolveLayout: %v", err)
	}
	canonicalData, err := filepath.EvalSymlinks(data)
	if err != nil {
		t.Fatal(err)
	}
	if want := layout.PrivateLayout(common, "/r", canonicalData, "o-r"); got != want {
		t.Fatalf("layout = %+v, want %+v", got, want)
	}
	if got := metadataRemote(got); got != "dckt" {
		t.Errorf("metadataRemote = %q, want dckt", got)
	}
	if got := metadataRef(got); got != "refs/heads/dckt" {
		t.Errorf("metadataRef = %q, want refs/heads/dckt", got)
	}
}

// TestResolveLayoutPrivateWithoutOriginURLFails: a private repository whose
// origin URL cannot be read cannot locate its store — a clear external
// failure, never a guessed location.
func TestResolveLayoutPrivateWithoutOriginURLFails(t *testing.T) {
	common := testsupport.TempDir(t)
	if err := os.Mkdir(filepath.Join(common, layout.PrivateName), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_DATA_HOME", testsupport.TempDir(t))
	r := &fakeURLReader{err: errors.New("no such remote")}
	_, err := resolveLayout(context.Background(), r, gitcli.Repository{CommonDir: common, PrimaryWorktree: "/r"})
	if err == nil {
		t.Fatal("resolveLayout succeeded without a readable origin URL, want an error")
	}
	if !errors.Is(err, ErrStatusExternal) {
		t.Errorf("err = %v, want ErrStatusExternal", err)
	}
}

// TestResolveLayoutUnprobeableStateFails: a <common>/dckt that exists but is not
// a directory is neither mode; resolution errors rather than guessing shared.
func TestResolveLayoutUnprobeableStateFails(t *testing.T) {
	common := testsupport.TempDir(t)
	if err := os.WriteFile(filepath.Join(common, layout.PrivateName), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := resolveLayout(context.Background(), &fakeURLReader{url: "git@github.com:o/r.git"}, gitcli.Repository{CommonDir: common, PrimaryWorktree: "/r"})
	if !errors.Is(err, ErrStatusExternal) {
		t.Fatalf("err = %v, want ErrStatusExternal", err)
	}
}

// TestCanonicalExistingPrefix proves the data home is spelled the way git
// records worktree paths: symlinks in the existing ancestor resolved, and a
// not-yet-existing remainder kept.
func TestCanonicalExistingPrefix(t *testing.T) {
	real := testsupport.TempDir(t)
	canonicalReal, err := filepath.EvalSymlinks(real)
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(testsupport.TempDir(t), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if got := canonicalExistingPrefix(link); got != canonicalReal {
		t.Errorf("existing link = %q, want %q", got, canonicalReal)
	}
	if got, want := canonicalExistingPrefix(filepath.Join(link, "a", "b")), filepath.Join(canonicalReal, "a", "b"); got != want {
		t.Errorf("missing remainder = %q, want %q", got, want)
	}
}
