package bashupgrade

import (
	"reflect"
	"testing"
)

func TestSubstituteHome(t *testing.T) {
	got := substituteHome([]byte("x=@@SANDBOX_HOME@@/dev/docket/scripts\ny=@@SANDBOX_HOME@@"), "/tmp/h")
	if string(got) != "x=/tmp/h/dev/docket/scripts\ny=/tmp/h" {
		t.Fatalf("substituteHome = %q", got)
	}
	if string(substituteHome([]byte("no token"), "/tmp/h")) != "no token" {
		t.Fatal("substituteHome changed bytes without a token")
	}
}

func TestParseCloneConfig(t *testing.T) {
	in := "# replay\nconfig core.hooksPath /dev/null\n\nworktree .docket docket\n"
	got, err := parseCloneConfig(in)
	if err != nil {
		t.Fatal(err)
	}
	want := []cloneAction{{Kind: "config", A: "core.hooksPath", B: "/dev/null"}, {Kind: "worktree", A: ".docket", B: "docket"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseCloneConfig = %#v", got)
	}
	hooks, err := parseCloneConfig("config extensions.worktreeConfig true\nhooks-off .docket\n")
	if err != nil {
		t.Fatal(err)
	}
	if want := []cloneAction{{Kind: "config", A: "extensions.worktreeConfig", B: "true"}, {Kind: "hooks-off", A: ".docket"}}; !reflect.DeepEqual(hooks, want) {
		t.Fatalf("parseCloneConfig(hooks-off) = %#v", hooks)
	}
	for _, bad := range []string{
		"config onlykey\n", "worktree .docket\n", "exec rm -rf /\n", "config a b c\n",
		"hooks-off\n", "hooks-off .docket extra\n", "hooks-off ../elsewhere\n", "worktree /abs docket\n",
	} {
		if _, err := parseCloneConfig(bad); err == nil {
			t.Errorf("parseCloneConfig(%q) accepted a malformed line", bad)
		}
	}
}

func TestFindingCodesWalksAnyShape(t *testing.T) {
	doc := `{"result":"applied","checks":[{"findings":[{"code":"board-stale","severity":"warning"}]}],"findings":[{"code":"test-config-missing","severity":"error"}]}` + "\n"
	got := findingCodes(t, doc)
	want := []findingRef{{"board-stale", "warning"}, {"test-config-missing", "error"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("findingCodes = %#v", got)
	}
}
