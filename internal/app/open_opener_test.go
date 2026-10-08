package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danielhanold/docket/internal/testsupport"
)

func TestSystemOpenerSelectsByPlatform(t *testing.T) {
	cases := []struct {
		goos, lookup string
		lookErr      error
		noOpener     bool
	}{
		{goos: "darwin", lookup: "open"},
		{goos: "linux", lookup: "xdg-open"},
		{goos: "linux", lookup: "xdg-open", lookErr: errors.New("not found"), noOpener: true},
		{goos: "windows", noOpener: true},
		{goos: "freebsd", noOpener: true},
	}
	for _, tc := range cases {
		var looked []string
		var ran [][2]string
		o := NewSystemOpener(tc.goos,
			func(n string) (string, error) { looked = append(looked, n); return "/bin/" + n, tc.lookErr },
			func(_ context.Context, bin, target string) error {
				ran = append(ran, [2]string{bin, target})
				return nil
			})
		err := o.Open(context.Background(), "/tmp/a b/c.md")
		if tc.noOpener {
			var no *NoOpenerError
			want := "no opener available on " + tc.goos + " — open it yourself, or use --print"
			if !errors.As(err, &no) || err.Error() != want || len(ran) != 0 {
				t.Errorf("%s: err=%v ran=%v, want %q and no launch", tc.goos, err, ran, want)
			}
			if tc.lookup == "" && len(looked) != 0 {
				t.Errorf("%s: consulted PATH %v", tc.goos, looked)
			}
			continue
		}
		if err != nil || len(looked) != 1 || looked[0] != tc.lookup || len(ran) != 1 || ran[0] != [2]string{"/bin/" + tc.lookup, "/tmp/a b/c.md"} {
			t.Errorf("%s: err=%v looked=%v ran=%v", tc.goos, err, looked, ran)
		}
	}
}

// openerScript writes a stand-in opener recording "$#|$1", chattering on
// stdout and stderr, and exiting code.
func openerScript(t *testing.T, code int, errText string) (bin, argsFile string) {
	t.Helper()
	dir := testsupport.TempDir(t)
	argsFile, bin = filepath.Join(dir, "args"), filepath.Join(dir, "fake-opener")
	body := fmt.Sprintf("#!/bin/sh\nprintf '%%s|%%s' \"$#\" \"$1\" > '%s'\necho chatter\necho '%s' >&2\nexit %d\n", argsFile, errText, code)
	if err := os.WriteFile(bin, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	return bin, argsFile
}

func TestRunOpenerProcessPassesOneArgAndReportsFailure(t *testing.T) {
	bin, args := openerScript(t, 0, "")
	if err := RunOpenerProcess(context.Background(), bin, "/tmp/a b/c d.md"); err != nil {
		t.Fatalf("RunOpenerProcess: %v", err)
	}
	if got, _ := os.ReadFile(args); string(got) != "1|/tmp/a b/c d.md" {
		t.Errorf("opener saw %q, want one argument", got)
	}
	bin, _ = openerScript(t, 3, "cannot open display")
	err := RunOpenerProcess(context.Background(), bin, "https://example.test/x")
	if err == nil || !strings.Contains(err.Error(), "exit status 3") || !strings.Contains(err.Error(), "cannot open display") {
		t.Fatalf("err = %v, want exit status and stderr", err)
	}
}
