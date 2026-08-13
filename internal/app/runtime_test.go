package app

import (
	"encoding/json"
	"testing"

	"github.com/danielhanold/docket/internal/buildinfo"
)

func TestDiagnosticRuntimeSupportedTuple(t *testing.T) {
	r := DiagnosticRuntime(buildinfo.RuntimeFacts{GoVersion: "go1.26.5", GOOS: "darwin", GOARCH: "arm64"})
	if !r.SupportedTarget {
		t.Fatal("darwin/arm64 must be a supported target")
	}
	if got, want := r.HumanText(), "go_version: go1.26.5\ngo_os: darwin\ngo_arch: arm64\nsupported_target: true"; got != want {
		t.Fatalf("text = %q, want %q", got, want)
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"protocol_version":1,"operation":"diagnostic.runtime","result":"applied","go_version":"go1.26.5","go_os":"darwin","go_arch":"arm64","supported_target":true}`
	if string(b) != want {
		t.Fatalf("json = %s, want %s", b, want)
	}
}

func TestDiagnosticRuntimeUnsupportedTupleStillApplied(t *testing.T) {
	r := DiagnosticRuntime(buildinfo.RuntimeFacts{GoVersion: "go1.26.5", GOOS: "windows", GOARCH: "amd64"})
	if r.SupportedTarget {
		t.Fatal("windows/amd64 must not be a supported target")
	}
	if r.Result != ResultApplied {
		t.Fatalf("inspection on an unsupported tuple is still applied, got %q", r.Result)
	}
}

func TestAllFourApprovedTuples(t *testing.T) {
	for _, tuple := range [][2]string{{"darwin", "amd64"}, {"darwin", "arm64"}, {"linux", "amd64"}, {"linux", "arm64"}} {
		r := DiagnosticRuntime(buildinfo.RuntimeFacts{GoVersion: "go1.26.5", GOOS: tuple[0], GOARCH: tuple[1]})
		if !r.SupportedTarget {
			t.Fatalf("%s/%s must be supported", tuple[0], tuple[1])
		}
	}
}
