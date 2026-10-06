package install

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

// aliasBin makes a canonical bin dir holding a docket binary and returns both.
func aliasBin(t *testing.T) (string, string) {
	t.Helper()
	dir, err := filepath.EvalSymlinks(testsupport.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(bin, "docket")
	if err := os.WriteFile(binary, []byte("binary\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin, binary
}

func TestInspectBinaryAlias(t *testing.T) {
	t.Run("absent is missing", func(t *testing.T) {
		_, binary := aliasBin(t)
		f, err := InspectBinaryAlias(binary)
		if err != nil || f == nil || f.Kind != AliasMissing || f.Path != AliasPathFor(binary) || f.Remedy == "" {
			t.Fatalf("finding = %+v, err %v; want missing with a remedy", f, err)
		}
	})
	t.Run("relative link (downloader spelling) is healthy", func(t *testing.T) {
		bin, binary := aliasBin(t)
		if err := os.Symlink("docket", filepath.Join(bin, AliasName)); err != nil {
			t.Fatal(err)
		}
		if f, err := InspectBinaryAlias(binary); err != nil || f != nil {
			t.Fatalf("finding = %+v, err %v; want healthy", f, err)
		}
	})
	t.Run("absolute link (development spelling) is healthy", func(t *testing.T) {
		bin, binary := aliasBin(t)
		if err := os.Symlink(binary, filepath.Join(bin, AliasName)); err != nil {
			t.Fatal(err)
		}
		if f, err := InspectBinaryAlias(binary); err != nil || f != nil {
			t.Fatalf("finding = %+v, err %v; want healthy", f, err)
		}
	})
	t.Run("alias reached through a symlinked directory is healthy", func(t *testing.T) {
		bin, binary := aliasBin(t)
		via := filepath.Join(filepath.Dir(bin), "via")
		if err := os.Symlink(bin, via); err != nil {
			t.Fatal(err)
		}
		// The link spells the binary through the symlinked directory; the
		// identity check must canonicalise every hop and still call it ours.
		if err := os.Symlink(filepath.Join(via, "docket"), filepath.Join(bin, AliasName)); err != nil {
			t.Fatal(err)
		}
		if f, err := InspectBinaryAlias(filepath.Join(via, "docket")); err != nil || f != nil {
			t.Fatalf("finding = %+v, err %v; want healthy", f, err)
		}
		// The same alias probed through the canonical spelling of the binary
		// differs from the link's text; only a canonical comparison calls it ours.
		if f, err := InspectBinaryAlias(binary); err != nil || f != nil {
			t.Fatalf("canonical spelling: finding = %+v, err %v; want healthy", f, err)
		}
	})
	t.Run("link elsewhere is foreign", func(t *testing.T) {
		bin, binary := aliasBin(t)
		other := filepath.Join(bin, "other")
		if err := os.WriteFile(other, []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(other, filepath.Join(bin, AliasName)); err != nil {
			t.Fatal(err)
		}
		f, err := InspectBinaryAlias(binary)
		if err != nil || f == nil || f.Kind != AliasForeign {
			t.Fatalf("finding = %+v, err %v; want foreign", f, err)
		}
	})
	t.Run("dangling link is foreign", func(t *testing.T) {
		bin, binary := aliasBin(t)
		if err := os.Symlink(filepath.Join(bin, "gone"), filepath.Join(bin, AliasName)); err != nil {
			t.Fatal(err)
		}
		if f, err := InspectBinaryAlias(binary); err != nil || f == nil || f.Kind != AliasForeign {
			t.Fatalf("finding = %+v, err %v; want foreign", f, err)
		}
	})
	t.Run("regular file is foreign", func(t *testing.T) {
		bin, binary := aliasBin(t)
		if err := os.WriteFile(filepath.Join(bin, AliasName), []byte("mine\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		if f, err := InspectBinaryAlias(binary); err != nil || f == nil || f.Kind != AliasForeign {
			t.Fatalf("finding = %+v, err %v; want foreign", f, err)
		}
	})
	t.Run("directory is foreign", func(t *testing.T) {
		bin, binary := aliasBin(t)
		if err := os.Mkdir(filepath.Join(bin, AliasName), 0o755); err != nil {
			t.Fatal(err)
		}
		if f, err := InspectBinaryAlias(binary); err != nil || f == nil || f.Kind != AliasForeign {
			t.Fatalf("finding = %+v, err %v; want foreign", f, err)
		}
	})
	t.Run("probe error is an error, not missing", func(t *testing.T) {
		if runtime.GOOS == "windows" || os.Geteuid() == 0 {
			t.Skip("permission bits do not deny root")
		}
		bin, binary := aliasBin(t)
		if err := os.Chmod(bin, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(bin, 0o755) })
		f, err := InspectBinaryAlias(binary)
		if err == nil {
			t.Fatalf("finding = %+v with nil error; an unreadable bin dir must not read as a clean absence", f)
		}
	})
	t.Run("link to a vanished binary is foreign", func(t *testing.T) {
		bin, binary := aliasBin(t)
		if err := os.Symlink("docket", filepath.Join(bin, AliasName)); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(binary); err != nil {
			t.Fatal(err)
		}
		if f, err := InspectBinaryAlias(binary); err != nil || f == nil || f.Kind != AliasForeign {
			t.Fatalf("finding = %+v, err %v; want foreign (the alias resolves to nothing)", f, err)
		}
	})
}
