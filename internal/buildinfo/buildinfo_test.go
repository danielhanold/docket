package buildinfo

import (
	"runtime"
	"testing"
)

func TestCurrentDevelopmentDefaults(t *testing.T) {
	info := Current()
	if info.Version != "development" || info.Commit != "unknown" || info.BuildDate != "unknown" {
		t.Fatalf("development defaults wrong: %+v", info)
	}
}

func TestCurrentReflectsInjectedVars(t *testing.T) {
	origV, origC, origD := Version, Commit, BuildDate
	defer func() { Version, Commit, BuildDate = origV, origC, origD }()
	Version, Commit, BuildDate = "1.2.3", "abc1234", "2026-08-13"
	info := Current()
	if info.Version != "1.2.3" || info.Commit != "abc1234" || info.BuildDate != "2026-08-13" {
		t.Fatalf("injected identity not reflected: %+v", info)
	}
}

func TestCurrentRuntimeMatchesHost(t *testing.T) {
	f := CurrentRuntime()
	if f.GoVersion != runtime.Version() || f.GOOS != runtime.GOOS || f.GOARCH != runtime.GOARCH {
		t.Fatalf("runtime facts diverge from host: %+v", f)
	}
}
