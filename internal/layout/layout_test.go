package layout

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestDetect(t *testing.T) {
	t.Run("empty common dir is shared", func(t *testing.T) {
		c := t.TempDir()
		m, err := Detect(c)
		if err != nil || m != Shared {
			t.Fatalf("Detect = (%q, %v), want (shared, nil)", m, err)
		}
		if got, want := StateDirOf(c), filepath.Join(c, "docket"); got != want {
			t.Fatalf("StateDirOf = %q, want %q", got, want)
		}
	})
	t.Run("dckt directory is private", func(t *testing.T) {
		c := t.TempDir()
		if err := os.Mkdir(filepath.Join(c, "dckt"), 0o755); err != nil {
			t.Fatal(err)
		}
		m, err := Detect(c)
		if err != nil || m != Private {
			t.Fatalf("Detect = (%q, %v), want (private, nil)", m, err)
		}
		if got, want := StateDirOf(c), filepath.Join(c, "dckt"); got != want {
			t.Fatalf("StateDirOf = %q, want %q", got, want)
		}
		if got, want := PrivateConfigPath(c), filepath.Join(c, "dckt", "config.yml"); got != want {
			t.Fatalf("PrivateConfigPath = %q, want %q", got, want)
		}
	})
	t.Run("dckt file fails closed", func(t *testing.T) {
		c := t.TempDir()
		if err := os.WriteFile(filepath.Join(c, "dckt"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if m, err := Detect(c); err == nil {
			t.Fatalf("Detect = (%q, nil), want an error", m)
		}
		if got := StateName(c); got != "dckt" {
			t.Fatalf("StateName = %q, want dckt", got)
		}
	})
}

func TestOwnerRepoForms(t *testing.T) {
	ok := map[string]string{
		"git@github.com:DanielHanold/Docket.git": "danielhanold-docket",
		"https://github.com/o/r":                 "o-r",
		"https://github.com/o/r.git/":            "o-r",
		"ssh://git@host:2222/Team.X/My_Repo.git": "team-x-my-repo",
		"/tmp/abc/origin.git":                    "abc-origin",
	}
	for in, want := range ok {
		got, err := OwnerRepo(in)
		if err != nil || got != want {
			t.Errorf("OwnerRepo(%q) = (%q, %v), want (%q, nil)", in, got, err, want)
		}
	}
	for _, in := range []string{"", "   ", "/", ".git", "::"} {
		if got, err := OwnerRepo(in); err == nil {
			t.Errorf("OwnerRepo(%q) = (%q, nil), want an error", in, got)
		}
	}
}

func TestCloneID(t *testing.T) {
	a := CloneID("/home/u/src/api")
	if !regexp.MustCompile(`^api-[0-9a-f]{8}$`).MatchString(a) {
		t.Fatalf("CloneID = %q, want api-<8 hex>", a)
	}
	if again := CloneID("/home/u/src/api"); again != a {
		t.Fatalf("CloneID not deterministic: %q vs %q", a, again)
	}
	if other := CloneID("/home/u/other/api"); other == a {
		t.Fatalf("CloneID collides for distinct paths: %q", other)
	}
}

func TestDataHome(t *testing.T) {
	home := func() (string, error) { return "/home/u", nil }
	env := func(v string) func(string) string {
		return func(k string) string {
			if k == "XDG_DATA_HOME" {
				return v
			}
			return ""
		}
	}
	if got, err := DataHome(env("/xdg/data"), home); err != nil || got != "/xdg/data" {
		t.Errorf("absolute XDG: (%q, %v), want /xdg/data", got, err)
	}
	want := filepath.Join("/home/u", ".local", "share")
	if got, err := DataHome(env("rel/data"), home); err != nil || got != want {
		t.Errorf("relative XDG: (%q, %v), want %q", got, err, want)
	}
	if got, err := DataHome(env(""), home); err != nil || got != want {
		t.Errorf("unset XDG: (%q, %v), want %q", got, err, want)
	}
	noHome := func() (string, error) { return "", errors.New("no home") }
	if got, err := DataHome(env(""), noHome); err == nil {
		t.Errorf("home error: (%q, nil), want an error", got)
	}
	emptyHome := func() (string, error) { return "", nil }
	if got, err := DataHome(env(""), emptyHome); err == nil {
		t.Errorf("empty home: (%q, nil), want an error", got)
	}
}

func TestLayouts(t *testing.T) {
	s := SharedLayout("/r/.git", "/r")
	wantS := Layout{Shared, "/r/.git/docket", "origin", "docket", "/r/.docket", "", "", "", ""}
	if s != wantS {
		t.Fatalf("SharedLayout = %+v, want %+v", s, wantS)
	}
	if s.MetadataRef() != "refs/heads/docket" || s.TrackingRef() != "refs/remotes/origin/docket" {
		t.Fatalf("shared refs = %q, %q", s.MetadataRef(), s.TrackingRef())
	}

	p := PrivateLayout("/r/.git", "/r", "/data", "o-r")
	wantP := Layout{Private, "/r/.git/dckt", "dckt", "dckt",
		"/data/dckt/o-r/checkouts/" + CloneID("/r"), "/r/.git/dckt/config.yml",
		"/data/dckt/o-r", "/data/dckt/o-r/remote.git", "/data/dckt/o-r/checkouts"}
	if p != wantP {
		t.Fatalf("PrivateLayout = %+v, want %+v", p, wantP)
	}
	if p.MetadataRef() != "refs/heads/dckt" || p.TrackingRef() != "refs/remotes/dckt/dckt" {
		t.Fatalf("private refs = %q, %q", p.MetadataRef(), p.TrackingRef())
	}
}

func TestCommonDirOf(t *testing.T) {
	t.Run("git directory", func(t *testing.T) {
		root := t.TempDir()
		gd := filepath.Join(root, ".git")
		if err := os.Mkdir(gd, 0o755); err != nil {
			t.Fatal(err)
		}
		c, ok, err := CommonDirOf(root)
		if err != nil || !ok || c != gd {
			t.Fatalf("CommonDirOf = (%q, %v, %v), want (%q, true, nil)", c, ok, err, gd)
		}
	})
	t.Run("linked worktree", func(t *testing.T) {
		base := t.TempDir()
		common := filepath.Join(base, "main", ".git")
		gitdir := filepath.Join(common, "worktrees", "feat")
		if err := os.MkdirAll(gitdir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(gitdir, "commondir"), []byte("../..\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		root := filepath.Join(base, "feat")
		if err := os.Mkdir(root, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: "+gitdir+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		c, ok, err := CommonDirOf(root)
		if err != nil || !ok || c != common {
			t.Fatalf("CommonDirOf = (%q, %v, %v), want (%q, true, nil)", c, ok, err, common)
		}
	})
	t.Run("relative gitdir without commondir is its own common dir", func(t *testing.T) {
		root := t.TempDir()
		sub := filepath.Join(root, "modules", "sub")
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".git"), []byte("gitdir: modules/sub\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		c, ok, err := CommonDirOf(root)
		if err != nil || !ok || c != sub {
			t.Fatalf("CommonDirOf = (%q, %v, %v), want (%q, true, nil)", c, ok, err, sub)
		}
	})
	t.Run("plain directory", func(t *testing.T) {
		c, ok, err := CommonDirOf(t.TempDir())
		if err != nil || ok {
			t.Fatalf("CommonDirOf = (%q, %v, %v), want ok=false, nil error", c, ok, err)
		}
	})
}
