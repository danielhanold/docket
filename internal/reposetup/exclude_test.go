package reposetup

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

const canonicalExclude = "# dckt:start\n.worktrees/\n# dckt:end\n"

func TestExcludeBlockKeepsUserLinesAndIsIdempotent(t *testing.T) {
	in := []byte("# git ls-files --others --exclude-from=.git/info/exclude\n*.swp\n")
	out, changed, err := EnsureExcludeBlock(in)
	if err != nil {
		t.Fatalf("EnsureExcludeBlock: %v", err)
	}
	if !changed {
		t.Fatal("first call reported changed=false")
	}
	if !bytes.HasPrefix(out, in) {
		t.Fatalf("user bytes not preserved as a prefix:\n%q", out)
	}
	if !bytes.Contains(out, []byte(canonicalExclude)) {
		t.Fatalf("canonical block missing:\n%q", out)
	}
	if !ValidExcludeBlock(out) {
		t.Fatalf("ValidExcludeBlock(out) = false for %q", out)
	}
	if ValidExcludeBlock(in) {
		t.Fatal("ValidExcludeBlock(in) = true for a file with no block")
	}
	again, changed2, err := EnsureExcludeBlock(out)
	if err != nil {
		t.Fatalf("second EnsureExcludeBlock: %v", err)
	}
	if changed2 || !bytes.Equal(again, out) {
		t.Fatalf("second call not idempotent: changed=%v\n%q", changed2, again)
	}
}

func TestExcludeBlockNilInput(t *testing.T) {
	out, changed, err := EnsureExcludeBlock(nil)
	if err != nil {
		t.Fatalf("EnsureExcludeBlock(nil): %v", err)
	}
	if !changed || string(out) != canonicalExclude {
		t.Fatalf("got changed=%v %q, want changed=true %q", changed, out, canonicalExclude)
	}
}

func TestExcludeBlockMalformedMarkersRefuse(t *testing.T) {
	for _, in := range []string{
		"# dckt:start\n.worktrees/\n",
		"# dckt:end\n# dckt:start\n",
		"# dckt:start\n# dckt:start\n# dckt:end\n",
	} {
		orig := []byte(in)
		buf := append([]byte(nil), orig...)
		out, changed, err := EnsureExcludeBlock(buf)
		var me *MalformedExcludeError
		if !errors.As(err, &me) {
			t.Errorf("%q: err = %v, want *MalformedExcludeError", in, err)
		}
		if out != nil || changed {
			t.Errorf("%q: out=%q changed=%v, want nil/false", in, out, changed)
		}
		if !bytes.Equal(buf, orig) {
			t.Errorf("%q: input mutated to %q", in, buf)
		}
		if ValidExcludeBlock(orig) {
			t.Errorf("%q: ValidExcludeBlock = true for malformed markers", in)
		}
	}
}

func TestExcludeBlockRewritesStaleBody(t *testing.T) {
	out, changed, err := EnsureExcludeBlock([]byte("keep\n# dckt:start\nold/\n# dckt:end\n"))
	if err != nil {
		t.Fatalf("EnsureExcludeBlock: %v", err)
	}
	want := "keep\n" + canonicalExclude
	if !changed || string(out) != want {
		t.Fatalf("got changed=%v %q, want %q", changed, out, want)
	}
}

func TestExcludeBlockNeutralSpelling(t *testing.T) {
	out, _, err := EnsureExcludeBlock([]byte("*.swp\n"))
	if err != nil {
		t.Fatalf("EnsureExcludeBlock: %v", err)
	}
	if strings.Contains(strings.ToLower(string(out)), "docket") {
		t.Fatalf("exclude output names docket: %q", out)
	}
	if strings.Contains(strings.ToLower(ExcludeStart+ExcludeEnd), "docket") {
		t.Fatal("exclude markers name docket")
	}
}
