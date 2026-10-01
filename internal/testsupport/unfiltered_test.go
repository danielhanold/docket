package testsupport

import (
	"flag"
	"strings"
	"testing"
	"time"
)

// TestRefuseUnfilteredRunDecision pins every clause of the change-0479 decision:
// refuse only when run AND list are empty AND the timeout is go test's 10m
// default. Deleting any one clause of refuseUnfilteredRun reddens at least one
// "allow" case below.
func TestRefuseUnfilteredRunDecision(t *testing.T) {
	cases := []struct {
		name    string
		run     string
		list    string
		timeout time.Duration
		refuse  bool
	}{
		{"unfiltered at the default timeout", "", "", 10 * time.Minute, true},
		{"run set", "^TestIntegrationRunRecord", "", 10 * time.Minute, false},
		{"list set", "", "^Test", 10 * time.Minute, false},
		{"explicit 30m", "", "", 30 * time.Minute, false},
		{"timeout 0", "", "", 0, false},
		{"just under the default", "", "", 10*time.Minute - time.Second, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := refuseUnfilteredRun(tc.run, tc.list, tc.timeout); got != tc.refuse {
				t.Fatalf("refuseUnfilteredRun(%q, %q, %v) = %v, want %v", tc.run, tc.list, tc.timeout, got, tc.refuse)
			}
		})
	}
}

// testingFlagSet registers the four testing flags the guard reads or must
// ignore, under their real names and types, and parses args the way the
// generated test main would receive them from `go test`.
func testingFlagSet(t *testing.T, args ...string) *flag.FlagSet {
	t.Helper()
	fs := flag.NewFlagSet("unfiltered", flag.ContinueOnError)
	fs.String("test.run", "", "")
	fs.String("test.list", "", "")
	fs.String("test.skip", "", "")
	fs.Duration("test.timeout", 0, "")
	if err := fs.Parse(args); err != nil {
		t.Fatalf("parse %v: %v", args, err)
	}
	return fs
}

// TestCheckUnfilteredRunReadsTestingFlags drives the decision through real flag
// parsing, with the argv shapes `go test` hands the binary (verified at grooming:
// no -timeout arrives as -test.timeout=10m0s, -timeout 0 as 0s).
func TestCheckUnfilteredRunReadsTestingFlags(t *testing.T) {
	const pkg, glob = "internal/app", "tests/test_go_integration_app_*.sh"
	cases := []struct {
		name   string
		args   []string
		refuse bool
	}{
		{"default timeout, no filter", []string{"-test.timeout=10m0s"}, true},
		{"explicit -timeout 10m is the default", []string{"-test.timeout=10m"}, true},
		{"skip alone is not a filter", []string{"-test.timeout=10m0s", "-test.skip=."}, true},
		{"run filter allowed", []string{"-test.timeout=10m0s", "-test.run=^TestIntegrationRunRecord"}, false},
		{"list probe allowed", []string{"-test.timeout=10m0s", "-test.list=^Test"}, false},
		{"whole run at 30m allowed", []string{"-test.timeout=30m0s"}, false},
		{"timeout 0 allowed", []string{"-test.timeout=0s"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkUnfilteredRun(testingFlagSet(t, tc.args...), pkg, glob)
			if !tc.refuse {
				if err != nil {
					t.Fatalf("args %v: got %v, want nil", tc.args, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("args %v: got nil, want the refusal", tc.args)
			}
			if err.Error() != UnfilteredRunRemedy(pkg, glob) {
				t.Fatalf("refusal must be exactly the remedy text, got:\n%s", err)
			}
		})
	}
}

// TestCheckUnfilteredRunSetupErrors: a missing or mistyped flag, or empty
// arguments, is a setup error, never a silent allow and never the remedy text.
func TestCheckUnfilteredRunSetupErrors(t *testing.T) {
	const pkg, glob = "internal/app", "tests/test_go_integration_app_*.sh"
	for _, missing := range []string{"test.run", "test.list", "test.timeout"} {
		t.Run("missing "+missing, func(t *testing.T) {
			fs := flag.NewFlagSet("partial", flag.ContinueOnError)
			for _, n := range []string{"test.run", "test.list"} {
				if n != missing {
					fs.String(n, "", "")
				}
			}
			if missing != "test.timeout" {
				fs.Duration("test.timeout", 10*time.Minute, "")
			}
			err := checkUnfilteredRun(fs, pkg, glob)
			if err == nil || !strings.Contains(err.Error(), missing) || err.Error() == UnfilteredRunRemedy(pkg, glob) {
				t.Fatalf("missing %s: got %v, want a setup error naming it", missing, err)
			}
		})
	}
	t.Run("timeout of the wrong type", func(t *testing.T) {
		fs := flag.NewFlagSet("mistyped", flag.ContinueOnError)
		fs.String("test.run", "", "")
		fs.String("test.list", "", "")
		fs.String("test.timeout", "10m0s", "")
		err := checkUnfilteredRun(fs, pkg, glob)
		if err == nil || !strings.Contains(err.Error(), "test.timeout") || err.Error() == UnfilteredRunRemedy(pkg, glob) {
			t.Fatalf("mistyped test.timeout: got %v, want a setup error naming it", err)
		}
	})
	for _, tc := range []struct{ name, pkg, glob string }{{"empty package", "", glob}, {"empty glob", pkg, ""}} {
		t.Run(tc.name, func(t *testing.T) {
			if err := checkUnfilteredRun(testingFlagSet(t, "-test.timeout=10m0s"), tc.pkg, tc.glob); err == nil {
				t.Fatalf("%s: got nil, want a setup error", tc.name)
			}
		})
	}
}

// TestUnfilteredRunRemedyCarriesTheSupportedForms pins the spec's required
// content: the package, the shard glob AS PASSED (never a second literal), the
// -run form, and the -timeout value.
func TestUnfilteredRunRemedyCarriesTheSupportedForms(t *testing.T) {
	got := UnfilteredRunRemedy("internal/app", "tests/x_*.sh")
	for _, want := range []string{
		"internal/app:",
		"default 10m timeout",
		"bash <one of tests/x_*.sh>",
		"go test -tags integration -count=1 -run '^<Prefix>' ./internal/app/",
		"-timeout " + WholeCorpusTimeout,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("remedy missing %q:\n%s", want, got)
		}
	}
	if WholeCorpusTimeout != "30m" {
		t.Fatalf("WholeCorpusTimeout = %q, want 30m (spec; recompute from tests/runtime-budgets.tsv before changing)", WholeCorpusTimeout)
	}
}
