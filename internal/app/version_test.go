package app

import (
	"encoding/json"
	"testing"

	"github.com/danielhanold/docket/internal/buildinfo"
)

func TestVersionDevelopmentTextAndJSON(t *testing.T) {
	r := Version(buildinfo.Info{Version: "development", Commit: "unknown", BuildDate: "unknown"})
	if got, want := r.HumanText(), "docket development (commit unknown, built unknown)"; got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"protocol_version":1,"operation":"version","result":"applied","version":"development","commit":"unknown","build_date":"unknown"}`
	if string(b) != want {
		t.Fatalf("json = %s, want %s", b, want)
	}
}

func TestVersionInjectedIdentity(t *testing.T) {
	r := Version(buildinfo.Info{Version: "1.2.3", Commit: "abc1234", BuildDate: "2026-08-13"})
	if got, want := r.HumanText(), "docket 1.2.3 (commit abc1234, built 2026-08-13)"; got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
}
